package outbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type SagaStatus string

const (
	SagaStatusRunning            SagaStatus = "running"
	SagaStatusCompleted          SagaStatus = "completed"
	SagaStatusCompensated        SagaStatus = "compensated"
	SagaStatusCompensationFailed SagaStatus = "compensation-failed"

	SafeSagaActionError       = "saga action failed"
	SafeSagaCompensationError = "saga compensation failed"
)

var ErrSagaCompensationFailed = errors.New("saga compensation failed")

type SagaStep struct {
	ID         string
	Execute    func(context.Context) error
	Compensate func(context.Context) error
}

type SagaCommand struct {
	Name       string
	TenantCode string
	Steps      []SagaStep
}

type SagaRun struct {
	ID         string
	Name       string
	TenantCode string
	Status     SagaStatus
	LastError  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// SagaRunner provides the compensation boundary for business composition
// roots. It records durable outcomes but deliberately has no business steps of
// its own: each caller supplies the action and its matching compensation.
type SagaRunner struct {
	db  *gorm.DB
	now func() time.Time
}

type gormSagaRun struct {
	ID         string    `gorm:"primaryKey;size:40"`
	Name       string    `gorm:"size:140;not null"`
	TenantCode string    `gorm:"index;size:64;not null"`
	Status     string    `gorm:"index;size:32;not null"`
	LastError  string    `gorm:"size:128"`
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`
}

func (gormSagaRun) TableName() string { return "platform_outbox_saga_runs" }

type gormSagaStep struct {
	SagaID    string    `gorm:"primaryKey;size:40"`
	StepID    string    `gorm:"primaryKey;size:140"`
	Position  int       `gorm:"not null"`
	Status    string    `gorm:"size:32;not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (gormSagaStep) TableName() string { return "platform_outbox_saga_steps" }

func OpenSagaRunner(ctx context.Context, db *gorm.DB, now func() time.Time) (*SagaRunner, error) {
	if ctx == nil || db == nil || now == nil || !db.WithContext(ctx).Migrator().HasTable(&gormSagaRun{}) || !db.WithContext(ctx).Migrator().HasTable(&gormSagaStep{}) {
		return nil, ErrInvalidEnvelope
	}
	return &SagaRunner{db: db, now: now}, nil
}

func (r *SagaRunner) Run(ctx context.Context, command SagaCommand) (SagaRun, error) {
	if r == nil || r.db == nil || ctx == nil || !validSagaCommand(command) {
		return SagaRun{}, ErrInvalidEnvelope
	}
	id, err := newSagaID()
	if err != nil {
		return SagaRun{}, err
	}
	now := r.now().UTC()
	row := gormSagaRun{ID: id, Name: strings.TrimSpace(command.Name), TenantCode: strings.TrimSpace(command.TenantCode), Status: string(SagaStatusRunning), CreatedAt: now, UpdatedAt: now}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return SagaRun{}, err
	}
	completed := make([]int, 0, len(command.Steps))
	for position, step := range command.Steps {
		if err := step.Execute(ctx); err != nil {
			return r.compensate(ctx, row, command.Steps, completed, err)
		}
		if err := r.recordStep(ctx, row.ID, step.ID, position, "completed"); err != nil {
			return fromGormSagaRun(row), err
		}
		completed = append(completed, position)
	}
	row.Status, row.LastError, row.UpdatedAt = string(SagaStatusCompleted), "", r.now().UTC()
	if err := r.db.WithContext(ctx).Save(&row).Error; err != nil {
		return fromGormSagaRun(row), err
	}
	return fromGormSagaRun(row), nil
}

func (r *SagaRunner) compensate(ctx context.Context, row gormSagaRun, steps []SagaStep, completed []int, actionErr error) (SagaRun, error) {
	for index := len(completed) - 1; index >= 0; index-- {
		position := completed[index]
		if err := steps[position].Compensate(ctx); err != nil {
			row.Status, row.LastError, row.UpdatedAt = string(SagaStatusCompensationFailed), SafeSagaCompensationError, r.now().UTC()
			_ = r.recordStep(ctx, row.ID, steps[position].ID, position, "compensation-failed")
			if saveErr := r.db.WithContext(ctx).Save(&row).Error; saveErr != nil {
				return fromGormSagaRun(row), saveErr
			}
			return fromGormSagaRun(row), ErrSagaCompensationFailed
		}
		if err := r.recordStep(ctx, row.ID, steps[position].ID, position, "compensated"); err != nil {
			return fromGormSagaRun(row), err
		}
	}
	row.Status, row.LastError, row.UpdatedAt = string(SagaStatusCompensated), SafeSagaActionError, r.now().UTC()
	if err := r.db.WithContext(ctx).Save(&row).Error; err != nil {
		return fromGormSagaRun(row), err
	}
	return fromGormSagaRun(row), actionErr
}

func (r *SagaRunner) recordStep(ctx context.Context, sagaID, stepID string, position int, status string) error {
	now := r.now().UTC()
	return r.db.WithContext(ctx).Save(&gormSagaStep{SagaID: sagaID, StepID: stepID, Position: position, Status: status, UpdatedAt: now}).Error
}

func validSagaCommand(command SagaCommand) bool {
	if !eventIdentifierPattern.MatchString(strings.TrimSpace(command.Name)) || !tenantCodePattern.MatchString(strings.TrimSpace(command.TenantCode)) || len(command.Steps) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(command.Steps))
	for _, step := range command.Steps {
		id := strings.TrimSpace(step.ID)
		if !eventIdentifierPattern.MatchString(id) || step.Execute == nil || step.Compensate == nil {
			return false
		}
		if _, exists := seen[id]; exists {
			return false
		}
		seen[id] = struct{}{}
	}
	return true
}

func newSagaID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "saga-" + hex.EncodeToString(value), nil
}

func fromGormSagaRun(row gormSagaRun) SagaRun {
	return SagaRun{ID: row.ID, Name: row.Name, TenantCode: row.TenantCode, Status: SagaStatus(row.Status), LastError: row.LastError, CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC()}
}
