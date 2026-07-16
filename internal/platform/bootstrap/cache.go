package bootstrap

import (
	"errors"

	"github.com/GnosiST/platform-go/internal/platform/cache"
	"github.com/GnosiST/platform-go/internal/platform/config"
)

func CacheFromConfig(cfg config.Config) (cache.Store, error) {
	switch cfg.CacheDriver {
	case "":
		return cache.NewMeteredStore("noop", cache.NewNoopStore()), nil
	case "memory":
		return cache.NewMeteredStore("memory", cache.NewMemoryStore(cache.MemoryStoreOptions{})), nil
	case "redis":
		return cache.NewMeteredStore("redis", cache.NewRedisStore(redisOptionsFromConfig(cfg))), nil
	default:
		return nil, errors.New("unsupported cache driver")
	}
}

func CacheInvalidationBusFromConfig(cfg config.Config) cache.InvalidationBus {
	if cfg.CacheDriver != "redis" {
		return cache.NewNoopInvalidationBus()
	}
	return cache.NewRedisInvalidationBus(redisOptionsFromConfig(cfg))
}

func redisOptionsFromConfig(cfg config.Config) cache.RedisOptions {
	return cache.RedisOptions{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}
}
