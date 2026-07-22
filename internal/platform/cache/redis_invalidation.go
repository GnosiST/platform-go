package cache

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

const defaultInvalidationChannel = "platform:cache:invalidations"

type RedisInvalidationBus struct {
	client  *redis.Client
	channel string
}

func NewRedisInvalidationBus(options RedisOptions) *RedisInvalidationBus {
	addr := options.Addr
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	return &RedisInvalidationBus{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: options.Password,
			DB:       options.DB,
		}),
		channel: defaultInvalidationChannel,
	}
}

func (b *RedisInvalidationBus) PublishInvalidation(ctx context.Context, event InvalidationEvent) error {
	if event.Resource == "" {
		return nil
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return b.client.Publish(ctx, b.channel, encoded).Err()
}

func (b *RedisInvalidationBus) SubscribeInvalidations(ctx context.Context, handler InvalidationHandler) error {
	if handler == nil {
		return nil
	}
	pubsub := b.client.Subscribe(ctx, b.channel)
	go func() {
		defer pubsub.Close()
		consumeRedisInvalidations(ctx, pubsub.Channel(), handler)
	}()
	return nil
}

func consumeRedisInvalidations(ctx context.Context, messages <-chan *redis.Message, handler InvalidationHandler) {
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-messages:
			if !ok {
				return
			}
			var event InvalidationEvent
			if err := json.Unmarshal([]byte(message.Payload), &event); err == nil {
				handler(ctx, event)
			}
		}
	}
}

func (b *RedisInvalidationBus) Close() error {
	return b.client.Close()
}

func (b *RedisInvalidationBus) CheckReadiness(ctx context.Context) error {
	return b.client.Ping(ctx).Err()
}
