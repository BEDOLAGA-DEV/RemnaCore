package nats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"

	wmnats "github.com/ThreeDotsLabs/watermill-nats/v2/pkg/nats"
	nc "github.com/nats-io/nats.go"
)

// EventPublisher wraps a Watermill NATS publisher to provide a simple
// JSON-based publish API on top of JetStream.
type EventPublisher struct {
	publisher *wmnats.Publisher
}

// NewEventPublisher creates an EventPublisher backed by the given NATS
// connection with JetStream enabled and automatic stream provisioning.
func NewEventPublisher(conn *nc.Conn) (*EventPublisher, error) {
	pub, err := wmnats.NewPublisherWithNatsConn(
		conn,
		wmnats.PublisherPublishConfig{
			Marshaler:         &wmnats.NATSMarshaler{},
			SubjectCalculator: wmnats.DefaultSubjectCalculator,
			JetStream: wmnats.JetStreamConfig{
				AutoProvision: true,
				TrackMsgId:    true,
			},
		},
		watermill.NewStdLogger(false, false),
	)
	if err != nil {
		return nil, fmt.Errorf("creating watermill publisher: %w", err)
	}

	return &EventPublisher{publisher: pub}, nil
}

// Publish serializes payload to JSON and publishes it to the given topic.
func (p *EventPublisher) Publish(_ context.Context, topic string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling event payload: %w", err)
	}

	msg := message.NewMessage(watermill.NewUUID(), data)

	if err := p.publisher.Publish(topic, msg); err != nil {
		return fmt.Errorf("publishing to %s: %w", topic, err)
	}

	return nil
}

// PublishRaw publishes a pre-serialized Watermill message to a topic.
func (p *EventPublisher) PublishRaw(topic string, msg *message.Message) error {
	return p.publisher.Publish(topic, msg)
}

// Close shuts down the underlying Watermill publisher.
func (p *EventPublisher) Close() error {
	return p.publisher.Close()
}
