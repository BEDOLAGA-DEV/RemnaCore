package nats

import (
	"context"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"

	wmnats "github.com/ThreeDotsLabs/watermill-nats/v2/pkg/nats"
	nc "github.com/nats-io/nats.go"
)

// EventSubscriber wraps a Watermill NATS subscriber to provide JetStream-backed
// subscriptions with durable consumer groups.
type EventSubscriber struct {
	subscriber *wmnats.Subscriber
}

// NewEventSubscriber creates an EventSubscriber using the given NATS
// connection. consumerGroup identifies the durable consumer group so that
// multiple instances of the same service share work.
func NewEventSubscriber(conn *nc.Conn, consumerGroup string) (*EventSubscriber, error) {
	sub, err := wmnats.NewSubscriberWithNatsConn(
		conn,
		wmnats.SubscriberSubscriptionConfig{
			Unmarshaler:       &wmnats.NATSMarshaler{},
			SubjectCalculator: wmnats.DefaultSubjectCalculator,
			QueueGroupPrefix:  consumerGroup,
			JetStream: wmnats.JetStreamConfig{
				AutoProvision: false, // streams pre-created by EnsureStreams
				DurablePrefix: consumerGroup,
			},
		},
		watermill.NewStdLogger(false, false),
	)
	if err != nil {
		return nil, fmt.Errorf("creating watermill subscriber: %w", err)
	}

	return &EventSubscriber{subscriber: sub}, nil
}

// Subscribe returns a channel of Watermill messages for the given topic.
func (s *EventSubscriber) Subscribe(ctx context.Context, topic string) (<-chan *message.Message, error) {
	ch, err := s.subscriber.Subscribe(ctx, topic)
	if err != nil {
		return nil, fmt.Errorf("subscribing to %s: %w", topic, err)
	}

	return ch, nil
}

// Close shuts down the underlying Watermill subscriber.
func (s *EventSubscriber) Close() error {
	return s.subscriber.Close()
}
