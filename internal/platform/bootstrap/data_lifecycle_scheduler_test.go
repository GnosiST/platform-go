package bootstrap

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"platform-go/internal/platform/datalifecycle"
)

func TestScheduledDataLifecycleUsesStableBucketAndSkipsApplyWithoutPromotion(t *testing.T) {
	var output bytes.Buffer
	runtime := &fakeScheduledDataLifecycleRuntime{}
	options := DataLifecycleScheduleOptions{
		Interval: 24 * time.Hour, BatchSize: 50, MaxRetries: 2,
		Logger: log.New(&output, "", 0),
	}
	now := time.Date(2026, 7, 14, 3, 4, 5, 0, time.UTC)

	runScheduledDataLifecycleCycle(context.Background(), runtime, options, now)
	runScheduledDataLifecycleCycle(context.Background(), runtime, options, now.Add(8*time.Hour))

	if len(runtime.options) != 2 {
		t.Fatalf("Run() calls = %d, want two dry-runs", len(runtime.options))
	}
	for _, run := range runtime.options {
		if run.Mode != datalifecycle.ModeDryRun || run.RunID != "retention-20260714T000000Z-dry-run" {
			t.Fatalf("scheduled run = %+v", run)
		}
		if run.BatchSize != 50 || run.MaxRetries != 2 || run.OwnerID != retentionSchedulerOwnerID {
			t.Fatalf("scheduled options = %+v", run)
		}
	}
	if strings.Count(output.String(), `"category":"promotion-required"`) != 2 {
		t.Fatalf("log output = %q, want two classified skips", output.String())
	}
}

func TestScheduledDataLifecycleUsesOnlyPersistedExactPromotion(t *testing.T) {
	promotion := datalifecycle.Promotion{
		DatasourceID:        datalifecycle.DefaultDatasourceID,
		CurrentFingerprint:  "sha256:" + strings.Repeat("c", 64),
		PromotedFingerprint: "sha256:" + strings.Repeat("a", 64),
		ImpactReportHash:    "sha256:" + strings.Repeat("b", 64),
		ActorID:             "operator-1", Reason: "approved retention", ApprovalRef: "change-42",
	}
	runtime := &fakeScheduledDataLifecycleRuntime{promotion: promotion, promotionFound: true}
	options := DataLifecycleScheduleOptions{Interval: 24 * time.Hour, BatchSize: 100, MaxRetries: 3, Logger: log.New(&bytes.Buffer{}, "", 0)}

	runScheduledDataLifecycleCycle(context.Background(), runtime, options, time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC))

	if len(runtime.options) != 2 || runtime.options[0].Mode != datalifecycle.ModeDryRun || runtime.options[1].Mode != datalifecycle.ModeApply {
		t.Fatalf("Run() calls = %+v, want dry-run then apply", runtime.options)
	}
	apply := runtime.options[1]
	if apply.RunID != "retention-20260714T000000Z-apply" {
		t.Fatalf("apply RunID = %q", apply.RunID)
	}
	if apply.Promotion.ImpactReportHash != promotion.ImpactReportHash || apply.Promotion.ActorID != promotion.ActorID ||
		apply.Promotion.Reason != promotion.Reason || apply.Promotion.ApprovalRef != promotion.ApprovalRef ||
		apply.Promotion.PromotedFingerprint != promotion.PromotedFingerprint ||
		apply.Promotion.CurrentFingerprint != promotion.CurrentFingerprint ||
		apply.Promotion.DryRunID != "retention-20260714T000000Z-dry-run" {
		t.Fatalf("apply promotion = %+v, want exact persisted evidence", apply.Promotion)
	}
}

func TestScheduledDataLifecycleLogsOnlyClassifiedFailure(t *testing.T) {
	var output bytes.Buffer
	runtime := &fakeScheduledDataLifecycleRuntime{runErr: errors.New("secret dsn and record payload")}
	options := DataLifecycleScheduleOptions{Interval: time.Hour, BatchSize: 100, MaxRetries: 3, Logger: log.New(&output, "", 0)}

	runScheduledDataLifecycleCycle(context.Background(), runtime, options, time.Now())

	if strings.Contains(output.String(), "secret") || !strings.Contains(output.String(), `"category":"runtime"`) {
		t.Fatalf("classified log output = %q", output.String())
	}
	if len(runtime.options) != 1 || runtime.options[0].Mode != datalifecycle.ModeDryRun {
		t.Fatalf("Run() calls = %+v, want dry-run only", runtime.options)
	}
}

func TestDataLifecycleSchedulerCloseStopsInFlightRun(t *testing.T) {
	runtime := &fakeScheduledDataLifecycleRuntime{blockUntilCancelled: true, started: make(chan struct{})}
	scheduler, err := StartDataLifecycleScheduler(context.Background(), runtime, DataLifecycleScheduleOptions{
		Interval: time.Hour, BatchSize: 100, MaxRetries: 3, Logger: log.New(&bytes.Buffer{}, "", 0),
	})
	if err != nil {
		t.Fatalf("StartDataLifecycleScheduler() error = %v", err)
	}
	select {
	case <-runtime.started:
	case <-time.After(time.Second):
		t.Fatal("scheduled run did not start")
	}
	if err := scheduler.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := scheduler.Close(); err != nil {
		t.Fatalf("Close() second error = %v", err)
	}
}

type fakeScheduledDataLifecycleRuntime struct {
	mu                  sync.Mutex
	options             []datalifecycle.Options
	runErr              error
	promotion           datalifecycle.Promotion
	promotionFound      bool
	blockUntilCancelled bool
	started             chan struct{}
	startOnce           sync.Once
}

func (f *fakeScheduledDataLifecycleRuntime) Run(ctx context.Context, options datalifecycle.Options) (datalifecycle.Report, error) {
	f.mu.Lock()
	f.options = append(f.options, options)
	f.mu.Unlock()
	if f.started != nil {
		f.startOnce.Do(func() { close(f.started) })
	}
	if f.blockUntilCancelled {
		<-ctx.Done()
		return datalifecycle.Report{RunID: options.RunID, Mode: options.Mode, Status: datalifecycle.StatusFailed}, ctx.Err()
	}
	status := datalifecycle.StatusCompleted
	if f.runErr != nil {
		status = datalifecycle.StatusFailed
	}
	return datalifecycle.Report{RunID: options.RunID, Mode: options.Mode, Status: status}, f.runErr
}

func (f *fakeScheduledDataLifecycleRuntime) LoadPromotion(context.Context) (datalifecycle.Promotion, bool, error) {
	return f.promotion, f.promotionFound, nil
}
