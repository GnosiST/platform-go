package capability

import (
	"context"
	"io"
)

type Runtime struct {
	MigrationExecutor MigrationExecutor
	SeedExecutor      SeedExecutor
	Closer            io.Closer
}

func (r Runtime) Close() error {
	if r.Closer == nil {
		return nil
	}
	return r.Closer.Close()
}

type MigrationExecution struct {
	CapabilityID ID
	Migration    Migration
	Runtime      Runtime
}

type SeedExecution struct {
	CapabilityID ID
	Seed         Seed
	Runtime      Runtime
}

type MigrationExecutor interface {
	RunMigration(context.Context, MigrationExecution) error
}

type SeedExecutor interface {
	RunSeed(context.Context, SeedExecution) error
}

type MigrationExecutorFunc func(context.Context, MigrationExecution) error

func (fn MigrationExecutorFunc) RunMigration(ctx context.Context, exec MigrationExecution) error {
	return fn(ctx, exec)
}

type SeedExecutorFunc func(context.Context, SeedExecution) error

func (fn SeedExecutorFunc) RunSeed(ctx context.Context, exec SeedExecution) error {
	return fn(ctx, exec)
}

func (r Runtime) RunMigration(ctx context.Context, exec MigrationExecution) error {
	exec.Runtime = r
	if r.MigrationExecutor != nil {
		return r.MigrationExecutor.RunMigration(ctx, exec)
	}
	return exec.Migration.Up(ctx, r)
}

func (r Runtime) RunSeed(ctx context.Context, exec SeedExecution) error {
	exec.Runtime = r
	if r.SeedExecutor != nil {
		return r.SeedExecutor.RunSeed(ctx, exec)
	}
	return exec.Seed.Run(ctx, r)
}
