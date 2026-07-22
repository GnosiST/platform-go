package cache

import (
	"context"
	"errors"
	"fmt"
)

type ReadinessChecker interface {
	CheckReadiness(context.Context) error
}

type Runtime struct {
	Store           Store
	InvalidationBus InvalidationBus
}

func (r Runtime) Close() error {
	var errs []error
	if closer, ok := r.InvalidationBus.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := r.Store.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	return errors.Join(errs...)
}

func (r Runtime) CheckReadiness(ctx context.Context) error {
	if checker, ok := r.Store.(ReadinessChecker); ok {
		if err := checker.CheckReadiness(ctx); err != nil {
			return fmt.Errorf("cache store readiness: %w", err)
		}
	}
	if checker, ok := r.InvalidationBus.(ReadinessChecker); ok {
		if err := checker.CheckReadiness(ctx); err != nil {
			return fmt.Errorf("cache invalidation readiness: %w", err)
		}
	}
	return nil
}
