package bootstrap

import (
	"context"
	"errors"

	"platform-go/internal/platform/config"
	"platform-go/internal/platform/session"
	"platform-go/internal/platform/storage"
)

func SessionsFromConfig(cfg config.Config) (*session.Store, error) {
	if cfg.SessionDriver != "" {
		if cfg.SessionDSN == "" {
			return nil, errors.New("session dsn is required")
		}
		db, err := storage.OpenGORM(storage.Config{Driver: cfg.SessionDriver, DSN: cfg.SessionDSN})
		if err != nil {
			return nil, err
		}
		repository, err := session.NewGORMRepository(context.Background(), db)
		if err != nil {
			if sqlDB, dbErr := db.DB(); dbErr == nil {
				_ = sqlDB.Close()
			}
			return nil, err
		}
		return session.NewRepositoryBackedStore(session.Options{}, repository)
	}
	if cfg.SessionFile == "" {
		return session.NewStore(session.Options{}), nil
	}
	return session.NewRepositoryBackedStore(session.Options{}, session.NewFileRepository(cfg.SessionFile))
}
