package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

func TestRabbitMQCertificationSmoke(t *testing.T) {
	endpoint := strings.TrimSpace(os.Getenv("PLATFORM_CERTIFICATION_RABBITMQ_URL"))
	if endpoint == "" {
		t.Skip("PLATFORM_CERTIFICATION_RABBITMQ_URL is not configured")
	}
	adapter, err := NewRabbitMQ(RabbitMQConfig{URL: endpoint, Exchange: "platform.outbox.certification"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = adapter.Close() })
	connection, err := amqp091.Dial(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	channel, err := connection.Channel()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = channel.Close() })
	queue, err := channel.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := channel.QueueBind(queue.Name, "platform.certification.v1", "platform.outbox.certification", false, nil); err != nil {
		t.Fatal(err)
	}
	deliveries, err := channel.Consume(queue.Name, "platform-outbox-certification", true, true, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.Publish(context.Background(), Message{Topic: "platform.certification.v1", Key: "tenant-a", Payload: []byte(`{"eventId":"evt-certification"}`), Headers: map[string]string{"event-id": "evt-certification", "event-version": "1.0", "tenant-code": "tenant-a"}}); err != nil {
		t.Fatal(err)
	}
	select {
	case delivery := <-deliveries:
		if delivery.MessageId != "evt-certification" || delivery.Headers["event-version"] != "1.0" || string(delivery.Body) != `{"eventId":"evt-certification"}` {
			t.Fatalf("delivery = %+v", delivery)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("rabbitmq did not deliver the certified outbox message")
	}
}
