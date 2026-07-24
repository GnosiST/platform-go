package integration

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/rabbitmq/amqp091-go"
)

var (
	ErrInvalidRabbitMQConfig = errors.New("invalid rabbitmq configuration")
	rabbitExchangePattern    = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,127}$`)
)

type RabbitMQConfig struct {
	URL      string
	Exchange string
}

// RabbitMQ is the single bundled MessageBus adapter. It publishes durable,
// versioned platform envelopes; consumption, acknowledgement and projection
// ownership remain with the receiving capability.
type RabbitMQ struct {
	connection *amqp091.Connection
	exchange   string
	mu         sync.Mutex
	closed     bool
}

func NewRabbitMQ(config RabbitMQConfig) (*RabbitMQ, error) {
	endpoint := strings.TrimSpace(config.URL)
	exchange := strings.TrimSpace(config.Exchange)
	parsed, err := url.Parse(endpoint)
	if err != nil || (parsed.Scheme != "amqp" && parsed.Scheme != "amqps") || parsed.Host == "" || !rabbitExchangePattern.MatchString(exchange) {
		return nil, ErrInvalidRabbitMQConfig
	}
	connection, err := amqp091.Dial(endpoint)
	if err != nil {
		return nil, fmt.Errorf("connect rabbitmq: %w", err)
	}
	adapter := &RabbitMQ{connection: connection, exchange: exchange}
	if err := adapter.declareExchange(); err != nil {
		_ = connection.Close()
		return nil, err
	}
	return adapter, nil
}

func (r *RabbitMQ) Kind() string { return "rabbitmq" }

func (r *RabbitMQ) Health(context.Context) error {
	if r == nil || r.connection == nil || r.connection.IsClosed() {
		return errors.New("rabbitmq is unavailable")
	}
	return nil
}

func (r *RabbitMQ) Publish(ctx context.Context, message Message) error {
	if err := r.Health(ctx); err != nil {
		return err
	}
	if ctx == nil || strings.TrimSpace(message.Topic) == "" || strings.TrimSpace(message.Key) == "" || len(message.Payload) == 0 {
		return errors.New("invalid rabbitmq message")
	}
	channel, err := r.connection.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()
	return channel.PublishWithContext(ctx, r.exchange, message.Topic, false, false, amqp091.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp091.Persistent,
		MessageId:    strings.TrimSpace(message.Headers["event-id"]),
		Type:         strings.TrimSpace(message.Topic),
		Headers:      rabbitHeaders(message.Headers),
		Body:         append([]byte(nil), message.Payload...),
	})
}

func (r *RabbitMQ) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.connection == nil {
		return nil
	}
	r.closed = true
	return r.connection.Close()
}

func (r *RabbitMQ) declareExchange() error {
	channel, err := r.connection.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()
	return channel.ExchangeDeclare(r.exchange, "topic", true, false, false, false, nil)
}

func rabbitHeaders(headers map[string]string) amqp091.Table {
	result := make(amqp091.Table, len(headers))
	for key, value := range headers {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		result[key] = strings.TrimSpace(value)
	}
	return result
}
