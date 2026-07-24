package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/integration"
)

type Publisher interface {
	Publish(context.Context, integration.Message) error
}

type DispatcherOptions struct {
	MaxAttempts   int
	RetryDelay    time.Duration
	LeaseDuration time.Duration
}

type Dispatcher struct {
	store         *GORMStore
	publisher     Publisher
	maxAttempts   int
	retryDelay    time.Duration
	leaseDuration time.Duration
}

type DispatchResult struct {
	Claimed      int
	Published    int
	Retried      int
	DeadLettered int
}

func NewDispatcher(store *GORMStore, publisher Publisher, options DispatcherOptions) *Dispatcher {
	if options.MaxAttempts <= 0 {
		options.MaxAttempts = 5
	}
	if options.RetryDelay <= 0 {
		options.RetryDelay = time.Second
	}
	if options.LeaseDuration <= 0 {
		options.LeaseDuration = time.Minute
	}
	return &Dispatcher{store: store, publisher: publisher, maxAttempts: options.MaxAttempts, retryDelay: options.RetryDelay, leaseDuration: options.LeaseDuration}
}

func (d *Dispatcher) RunOnce(ctx context.Context, owner string, limit int) (DispatchResult, error) {
	if d == nil || d.store == nil || d.publisher == nil || ctx == nil || limit <= 0 {
		return DispatchResult{}, ErrInvalidLease
	}
	records, err := d.store.Claim(ctx, strings.TrimSpace(owner), limit, d.leaseDuration)
	if err != nil {
		return DispatchResult{}, err
	}
	result := DispatchResult{Claimed: len(records)}
	for _, record := range records {
		message, err := messageFromRecord(record)
		if err == nil {
			err = d.publisher.Publish(ctx, message)
		}
		if err == nil {
			if err := d.store.MarkPublished(ctx, record.ID, owner); err != nil {
				return result, err
			}
			result.Published++
			continue
		}
		status, markErr := d.store.MarkFailure(ctx, record.ID, owner, d.maxAttempts, d.retryDelay)
		if markErr != nil {
			return result, markErr
		}
		if status == StatusDeadLetter {
			result.DeadLettered++
		} else {
			result.Retried++
		}
	}
	return result, nil
}

func messageFromRecord(record Record) (integration.Message, error) {
	validated, err := ValidateEnvelope(record.Envelope)
	if err != nil {
		return integration.Message{}, err
	}
	payload, err := json.Marshal(validated)
	if err != nil {
		return integration.Message{}, fmt.Errorf("marshal outbox envelope: %w", err)
	}
	headers := map[string]string{"event-id": validated.EventID, "event-version": validated.Version, "producer": validated.Producer, "tenant-code": validated.TenantCode, "traceparent": validated.TraceParent}
	if validated.TraceState != "" {
		headers["tracestate"] = validated.TraceState
	}
	return integration.Message{Topic: validated.EventType, Key: validated.TenantCode, Payload: payload, Headers: headers}, nil
}

func IsSafePublishError(err error) bool {
	return errors.Is(err, ErrInvalidEnvelope) || errors.Is(err, ErrInvalidLease)
}
