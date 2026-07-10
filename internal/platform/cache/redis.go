package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisOptions struct {
	Addr     string
	Password string
	DB       int
}

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(options RedisOptions) *RedisStore {
	addr := options.Addr
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	return &RedisStore{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: options.Password,
			DB:       options.DB,
		}),
	}
}

func (s *RedisStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	value, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

func (s *RedisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *RedisStore) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return s.client.Del(ctx, keys...).Err()
}

func (s *RedisStore) DeletePrefix(ctx context.Context, prefix string) error {
	if prefix == "" {
		return nil
	}
	var cursor uint64
	for {
		keys, nextCursor, err := s.client.Scan(ctx, cursor, prefix+"*", 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := s.Delete(ctx, keys...); err != nil {
				return err
			}
		}
		if nextCursor == 0 {
			return nil
		}
		cursor = nextCursor
	}
}

func (s *RedisStore) Close() error {
	return s.client.Close()
}
