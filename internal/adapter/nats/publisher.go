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
				AutoProvision: false, // Streams are pre-created by EnsureStreams on startup.
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
//
// Context handling: ctx is accepted for domainevent.Publisher interface
// compatibility but is not used to cancel the underlying NATS publish.
// PubAck timeout is managed by NATS client options configured in
// NewConnection (default 5s). The outbox relay checks ctx.Done() between
// batches, providing cancellation at the batch boundary.
//
// A goroutine-based ctx.Done() wrapper was considered and rejected: if the
// context is cancelled mid-publish, the wrapper returns immediately but the
// background goroutine continues until PubAck timeout, leaking a goroutine
// per cancelled publish. The NATS client timeout is the correct cancellation
// mechanism for individual publishes.
func (p *EventPublisher) Publish(ctx context.Context, topic string, payload any) error {
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

// PublishWithID serializes payload to JSON and publishes it with a
// deterministic message ID. When TrackMsgId is enabled on the publisher,
// Watermill uses the message UUID as the JetStream Nats-Msg-Id header,
// enabling server-side deduplication of retransmissions.
//
// Context handling: see Publish godoc for rationale. ctx is accepted for
// interface compatibility; PubAck timeout is controlled by NATS client options.
func (p *EventPublisher) PublishWithID(ctx context.Context, id string, topic string, payload []byte) error {
	msg := message.NewMessage(id, payload)

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
