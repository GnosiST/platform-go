package sensitivemigration

import (
	"context"
	"errors"
)

type Mode string

const (
	ModeInventory       Mode = "inventory"
	ModeDryRun          Mode = "dry-run"
	ModePrepare         Mode = "prepare"
	ModeApply           Mode = "apply"
	ModeVerify          Mode = "verify"
	ModeRehearseRestore Mode = "rehearse-restore"
	ModeRollback        Mode = "rollback"
	StatusCompleted          = "completed"
	StatusFailed             = "failed"
	DefaultBatchSize         = 100
	MaximumBatchSize         = 1000
)

var (
	ErrInvalidOptions = errors.New("sensitive migration invalid options")
	ErrReadFailed     = errors.New("sensitive migration read failed")
)

type Cursor struct {
	TenantID string
	RecordID string
}

type Row struct {
	Resource   string
	RecordID   string
	ValuesJSON string
}

type ReadStore interface {
	TenantScopes(context.Context, ResourcePlan) ([]string, error)
	Rows(context.Context, ResourcePlan, string, string, int) ([]Row, error)
}

type Options struct {
	Mode      Mode
	BatchSize int
}

type Counts struct {
	Missing           int `json:"missing"`
	Plaintext         int `json:"plaintext"`
	TargetEnvelope    int `json:"targetEnvelope"`
	ForeignEnvelope   int `json:"foreignEnvelope"`
	MalformedEnvelope int `json:"malformedEnvelope"`
}

type Report struct {
	RunID          string `json:"runId,omitempty"`
	Mode           Mode   `json:"mode"`
	Status         string `json:"status"`
	Counts         Counts `json:"counts"`
	Checkpoints    int    `json:"checkpoints"`
	EventChainHead string `json:"eventChainHead,omitempty"`
}
