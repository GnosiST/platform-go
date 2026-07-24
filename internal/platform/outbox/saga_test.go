package outbox

import (
	"context"
	"errors"
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/storage"
)

func TestSagaCompensatesCompletedStepsInReverseOrderAndAuditsOutcome(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: "file:outbox-saga?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareGORMStore(ctx, db); err != nil {
		t.Fatal(err)
	}
	runner, err := OpenSagaRunner(ctx, db, fixedOutboxClock)
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	failed := errors.New("remote action failed")
	run, err := runner.Run(ctx, SagaCommand{
		Name: "provision-resource", TenantCode: "tenant-a",
		Steps: []SagaStep{
			{ID: "reserve", Execute: func(context.Context) error { calls = append(calls, "reserve"); return nil }, Compensate: func(context.Context) error { calls = append(calls, "release"); return nil }},
			{ID: "activate", Execute: func(context.Context) error { calls = append(calls, "activate"); return failed }, Compensate: func(context.Context) error { calls = append(calls, "deactivate"); return nil }},
		},
	})
	if !errors.Is(err, failed) {
		t.Fatalf("Run() error = %v, want action failure", err)
	}
	if run.Status != SagaStatusCompensated || len(calls) != 3 || calls[0] != "reserve" || calls[1] != "activate" || calls[2] != "release" {
		t.Fatalf("run = %+v, calls = %#v", run, calls)
	}
	var persisted gormSagaRun
	if err := db.Where("id = ?", run.ID).Take(&persisted).Error; err != nil {
		t.Fatalf("load saga run: %v", err)
	}
	if persisted.Status != string(SagaStatusCompensated) || persisted.LastError != SafeSagaActionError {
		t.Fatalf("persisted saga = %+v", persisted)
	}
}

func TestSagaMarksCompensationFailureWithoutLeakingCause(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: "file:outbox-saga-compensation-failure?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareGORMStore(ctx, db); err != nil {
		t.Fatal(err)
	}
	runner, err := OpenSagaRunner(ctx, db, fixedOutboxClock)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runner.Run(ctx, SagaCommand{Name: "provision-resource", TenantCode: "tenant-a", Steps: []SagaStep{
		{ID: "reserve", Execute: func(context.Context) error { return nil }, Compensate: func(context.Context) error { return errors.New("private compensation failure") }},
		{ID: "activate", Execute: func(context.Context) error { return errors.New("private action failure") }, Compensate: func(context.Context) error { return nil }},
	}})
	if !errors.Is(err, ErrSagaCompensationFailed) {
		t.Fatalf("Run() error = %v, want ErrSagaCompensationFailed", err)
	}
	var persisted gormSagaRun
	if err := db.Order("created_at DESC").Take(&persisted).Error; err != nil {
		t.Fatalf("load saga run: %v", err)
	}
	if persisted.Status != string(SagaStatusCompensationFailed) || persisted.LastError != SafeSagaCompensationError {
		t.Fatalf("persisted saga = %+v", persisted)
	}
}
