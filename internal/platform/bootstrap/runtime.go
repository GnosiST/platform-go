package bootstrap

import (
	"context"
	"errors"

	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/storage"
)

func RuntimeFromConfig(cfg config.Config) (capability.Runtime, error) {
	if cfg.LifecycleHistoryDriver != "" {
		if cfg.LifecycleHistoryDSN == "" {
			return capability.Runtime{}, errors.New("lifecycle history dsn is required")
		}
		db, err := storage.OpenGORM(storage.Config{Driver: cfg.LifecycleHistoryDriver, DSN: cfg.LifecycleHistoryDSN})
		if err != nil {
			return capability.Runtime{}, err
		}
		history, err := capability.NewGORMLifecycleHistory(context.Background(), db)
		if err != nil {
			if sqlDB, dbErr := db.DB(); dbErr == nil {
				_ = sqlDB.Close()
			}
			return capability.Runtime{}, err
		}
		executor := capability.NewRecordedLifecycleExecutor(history)
		return capability.Runtime{MigrationExecutor: executor, SeedExecutor: executor, Closer: history}, nil
	}
	if cfg.LifecycleHistoryFile == "" {
		return capability.Runtime{}, nil
	}
	history, err := capability.NewFileLifecycleHistory(cfg.LifecycleHistoryFile)
	if err != nil {
		return capability.Runtime{}, err
	}
	executor := capability.NewRecordedLifecycleExecutor(history)
	return capability.Runtime{MigrationExecutor: executor, SeedExecutor: executor}, nil
}
