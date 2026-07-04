package kernel

import "context"

type UnitOfWork interface {
	Do(ctx context.Context, fn func(context.Context) error) error
}

type NoopUnitOfWork struct{}

func (NoopUnitOfWork) Do(ctx context.Context, fn func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return fn(ctx)
}
