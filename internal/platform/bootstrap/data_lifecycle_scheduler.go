package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/datalifecycle"
)

const retentionSchedulerOwnerID = "retention-scheduler"

var ErrDataLifecycleSchedule = errors.New("data lifecycle schedule invalid")

type DataLifecycleScheduleOptions struct {
	Interval   time.Duration
	BatchSize  int
	MaxRetries int
	Logger     *log.Logger
	Now        func() time.Time
}

type scheduledDataLifecycleRuntime interface {
	Run(context.Context, datalifecycle.Options) (datalifecycle.Report, error)
	LoadPromotion(context.Context) (datalifecycle.Promotion, bool, error)
}

type DataLifecycleScheduler struct {
	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once
}

type dataLifecycleScheduleEvent struct {
	Component string               `json:"component"`
	Event     string               `json:"event"`
	Category  string               `json:"category,omitempty"`
	RunID     string               `json:"runId,omitempty"`
	Mode      datalifecycle.Mode   `json:"mode,omitempty"`
	Status    string               `json:"status"`
	Counts    datalifecycle.Counts `json:"counts"`
}

func StartDataLifecycleScheduler(parent context.Context, runtime scheduledDataLifecycleRuntime, options DataLifecycleScheduleOptions) (*DataLifecycleScheduler, error) {
	if parent == nil || runtime == nil || options.Interval <= 0 || options.BatchSize < 1 || options.BatchSize > datalifecycle.MaximumBatchSize ||
		options.MaxRetries < 0 || options.MaxRetries > datalifecycle.MaximumRetries {
		return nil, ErrDataLifecycleSchedule
	}
	if options.Logger == nil {
		options.Logger = log.Default()
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	ctx, cancel := context.WithCancel(parent)
	scheduler := &DataLifecycleScheduler{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(scheduler.done)
		runScheduledDataLifecycleCycle(ctx, runtime, options, options.Now().UTC())
		ticker := time.NewTicker(options.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				runScheduledDataLifecycleCycle(ctx, runtime, options, now.UTC())
			}
		}
	}()
	return scheduler, nil
}

func (s *DataLifecycleScheduler) Close() error {
	if s == nil {
		return nil
	}
	s.stopOnce.Do(func() { s.cancel() })
	<-s.done
	return nil
}

func runScheduledDataLifecycleCycle(ctx context.Context, runtime scheduledDataLifecycleRuntime, options DataLifecycleScheduleOptions, now time.Time) {
	bucket := now.UTC().Truncate(options.Interval)
	baseRunID := "retention-" + bucket.Format("20060102T150405Z")
	dryRunID := baseRunID + "-dry-run"
	dryRunReport, err := runtime.Run(ctx, datalifecycle.Options{
		Enabled: true, Mode: datalifecycle.ModeDryRun, RunID: dryRunID,
		OwnerID: retentionSchedulerOwnerID, DatasourceID: datalifecycle.DefaultDatasourceID,
		BatchSize: options.BatchSize, MaxRetries: options.MaxRetries,
	})
	logDataLifecycleScheduleEvent(options.Logger, dataLifecycleEvent("run", dryRunReport, err))
	if err != nil || dryRunReport.Status != datalifecycle.StatusCompleted {
		return
	}

	applyRunID := baseRunID + "-apply"
	promotion, found, err := runtime.LoadPromotion(ctx)
	if err != nil {
		logDataLifecycleScheduleEvent(options.Logger, dataLifecycleScheduleEvent{
			Component: "data-lifecycle-retention", Event: "apply-skipped", Category: "repository",
			RunID: applyRunID, Mode: datalifecycle.ModeApply, Status: "skipped",
		})
		return
	}
	if !found {
		logDataLifecycleScheduleEvent(options.Logger, dataLifecycleScheduleEvent{
			Component: "data-lifecycle-retention", Event: "apply-skipped", Category: "promotion-required",
			RunID: applyRunID, Mode: datalifecycle.ModeApply, Status: "skipped",
		})
		return
	}
	applyReport, applyErr := runtime.Run(ctx, datalifecycle.Options{
		Enabled: true, Mode: datalifecycle.ModeApply, RunID: applyRunID,
		OwnerID: retentionSchedulerOwnerID, DatasourceID: datalifecycle.DefaultDatasourceID,
		BatchSize: options.BatchSize, MaxRetries: options.MaxRetries,
		Promotion: datalifecycle.PromotionApproval{
			ImpactReportHash: promotion.ImpactReportHash, ActorID: promotion.ActorID,
			Reason: promotion.Reason, ApprovalRef: promotion.ApprovalRef,
			PromotedFingerprint: promotion.PromotedFingerprint,
			CurrentFingerprint:  promotion.CurrentFingerprint, DryRunID: dryRunID,
		},
	})
	logDataLifecycleScheduleEvent(options.Logger, dataLifecycleEvent("run", applyReport, applyErr))
}

func dataLifecycleEvent(event string, report datalifecycle.Report, err error) dataLifecycleScheduleEvent {
	return dataLifecycleScheduleEvent{
		Component: "data-lifecycle-retention", Event: event, Category: dataLifecycleErrorCategory(err),
		RunID: report.RunID, Mode: report.Mode, Status: report.Status, Counts: report.Counts,
	}
}

func dataLifecycleErrorCategory(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "cancelled"
	case errors.Is(err, datalifecycle.ErrPromotionRequired):
		return "promotion-required"
	case errors.Is(err, datalifecycle.ErrLeaseHeld):
		return "lease-held"
	case errors.Is(err, datalifecycle.ErrPolicyDrift):
		return "policy-drift"
	case errors.Is(err, datalifecycle.ErrRepositoryFailed), errors.Is(err, datalifecycle.ErrPersistentRepositoryRequired):
		return "repository"
	case errors.Is(err, datalifecycle.ErrPlanningFailed):
		return "planner"
	case errors.Is(err, datalifecycle.ErrApplyFailed):
		return "applier"
	default:
		return "runtime"
	}
}

func logDataLifecycleScheduleEvent(logger *log.Logger, event dataLifecycleScheduleEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	logger.Print(string(payload))
}
