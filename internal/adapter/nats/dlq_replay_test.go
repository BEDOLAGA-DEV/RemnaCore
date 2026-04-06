package nats

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubSubscriber implements the Subscribe call by returning a caller-controlled
// channel. It avoids standing up a real NATS connection for unit tests.
type stubSubscriber struct {
	ch    chan *message.Message
	topic string
}

func (s *stubSubscriber) Subscribe(_ context.Context, topic string) (<-chan *message.Message, error) {
	s.topic = topic
	return s.ch, nil
}

// stubPublisher records every message published via PublishRaw, including the
// Watermill message UUID for deduplication assertions.
type stubPublisher struct {
	published []publishedMsg
}

type publishedMsg struct {
	topic   string
	payload []byte
	msgID   string // Watermill message UUID
}

func (p *stubPublisher) PublishRaw(topic string, msg *message.Message) error {
	p.published = append(p.published, publishedMsg{
		topic:   topic,
		payload: msg.Payload,
		msgID:   msg.UUID,
	})
	return nil
}

// makeDLQMessage creates a Watermill message containing a serialised DLQPayload.
func makeDLQMessage(t *testing.T, payload DLQPayload) *message.Message {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	return message.NewMessage(watermill.NewUUID(), data)
}

// replayDLQFromChannel is an interface-based version of ReplayDLQMessages that
// accepts test doubles. The production ReplayDLQMessages delegates to the same
// core logic (replayFromChannel) with concrete NATS types.
func replayDLQFromChannel(ctx context.Context, subscriber dlqSubscriber, publisher dlqPublisher, opts ReplayOptions) (ReplayResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = unlimitedReplayBound
	}
	return replayFromChannel(ctx, subscriber, publisher, opts, limit)
}

func TestReplayDLQMessages_ReplayesToOriginalSubject(t *testing.T) {
	ch := make(chan *message.Message, 3)

	originalPayload := `{"type":"subscription.cancelled","data":{"subscription_id":"sub-1"}}`

	dlqPayloads := []DLQPayload{
		{
			OriginalSubject: "subscription.cancelled",
			OriginalPayload: originalPayload,
			Error:           "transient error",
			MsgID:           "msg-1",
			FailedAt:        time.Now().Format(time.RFC3339),
			RetryCount:      3,
			EntityID:        "sub-1",
			EventType:       "subscription.cancelled",
		},
		{
			OriginalSubject: "subscription.paused",
			OriginalPayload: `{"type":"subscription.paused","data":{"subscription_id":"sub-2"}}`,
			Error:           "another error",
			MsgID:           "msg-2",
			FailedAt:        time.Now().Format(time.RFC3339),
			RetryCount:      3,
			EntityID:        "sub-2",
			EventType:       "subscription.paused",
		},
	}

	for _, p := range dlqPayloads {
		ch <- makeDLQMessage(t, p)
	}
	close(ch) // signal no more messages

	sub := &stubSubscriber{ch: ch}
	pub := &stubPublisher{}

	result, err := replayDLQFromChannel(context.Background(), sub, pub, ReplayOptions{})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Replayed)
	assert.Equal(t, 0, result.Skipped)
	assert.Equal(t, 0, result.Errors)

	require.Len(t, pub.published, 2)
	assert.Equal(t, "subscription.cancelled", pub.published[0].topic)
	assert.Equal(t, originalPayload, string(pub.published[0].payload))
	assert.Equal(t, "subscription.paused", pub.published[1].topic)
}

func TestReplayDLQMessages_LimitRespected(t *testing.T) {
	ch := make(chan *message.Message, 5)

	for i := 0; i < 5; i++ {
		ch <- makeDLQMessage(t, DLQPayload{
			OriginalSubject: "subscription.cancelled",
			OriginalPayload: `{"type":"test"}`,
			Error:           "error",
			MsgID:           watermill.NewUUID(),
			FailedAt:        time.Now().Format(time.RFC3339),
			RetryCount:      3,
		})
	}

	sub := &stubSubscriber{ch: ch}
	pub := &stubPublisher{}

	result, err := replayDLQFromChannel(context.Background(), sub, pub, ReplayOptions{Limit: 2})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Replayed)
	assert.Len(t, pub.published, 2)
}

func TestReplayDLQMessages_MalformedMessageSkipped(t *testing.T) {
	ch := make(chan *message.Message, 2)

	// First message: malformed (not valid JSON DLQPayload)
	ch <- message.NewMessage(watermill.NewUUID(), []byte("not json"))

	// Second message: valid
	ch <- makeDLQMessage(t, DLQPayload{
		OriginalSubject: "subscription.resumed",
		OriginalPayload: `{"type":"subscription.resumed"}`,
		Error:           "error",
		MsgID:           "msg-3",
		FailedAt:        time.Now().Format(time.RFC3339),
		RetryCount:      1,
	})
	close(ch)

	sub := &stubSubscriber{ch: ch}
	pub := &stubPublisher{}

	result, err := replayDLQFromChannel(context.Background(), sub, pub, ReplayOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Replayed)
	require.Len(t, pub.published, 1)
	assert.Equal(t, "subscription.resumed", pub.published[0].topic)
}

func TestReplayDLQMessages_ContextCancellation(t *testing.T) {
	ch := make(chan *message.Message) // unbuffered, will block

	sub := &stubSubscriber{ch: ch}
	pub := &stubPublisher{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	result, err := replayDLQFromChannel(ctx, sub, pub, ReplayOptions{Limit: 10})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 0, result.Replayed)
}

func TestReplayDLQMessages_FilterSubject(t *testing.T) {
	ch := make(chan *message.Message, 3)

	ch <- makeDLQMessage(t, DLQPayload{
		OriginalSubject: "subscription.cancelled",
		OriginalPayload: `{"type":"subscription.cancelled"}`,
		Error:           "error",
		MsgID:           "msg-1",
		FailedAt:        time.Now().Format(time.RFC3339),
		RetryCount:      3,
		EntityID:        "sub-1",
	})
	ch <- makeDLQMessage(t, DLQPayload{
		OriginalSubject: "subscription.paused",
		OriginalPayload: `{"type":"subscription.paused"}`,
		Error:           "error",
		MsgID:           "msg-2",
		FailedAt:        time.Now().Format(time.RFC3339),
		RetryCount:      1,
		EntityID:        "sub-2",
	})
	ch <- makeDLQMessage(t, DLQPayload{
		OriginalSubject: "subscription.cancelled",
		OriginalPayload: `{"type":"subscription.cancelled"}`,
		Error:           "error",
		MsgID:           "msg-3",
		FailedAt:        time.Now().Format(time.RFC3339),
		RetryCount:      2,
		EntityID:        "sub-3",
	})
	close(ch)

	sub := &stubSubscriber{ch: ch}
	pub := &stubPublisher{}

	result, err := replayDLQFromChannel(context.Background(), sub, pub, ReplayOptions{
		FilterSubject: "cancelled",
	})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Replayed)
	assert.Equal(t, 1, result.Skipped)
	require.Len(t, pub.published, 2)
	assert.Equal(t, "subscription.cancelled", pub.published[0].topic)
	assert.Equal(t, "subscription.cancelled", pub.published[1].topic)
}

func TestReplayDLQMessages_FilterEntity(t *testing.T) {
	ch := make(chan *message.Message, 3)

	ch <- makeDLQMessage(t, DLQPayload{
		OriginalSubject: "subscription.cancelled",
		OriginalPayload: `{"type":"subscription.cancelled"}`,
		Error:           "error",
		MsgID:           "msg-1",
		FailedAt:        time.Now().Format(time.RFC3339),
		RetryCount:      3,
		EntityID:        "sub-target",
	})
	ch <- makeDLQMessage(t, DLQPayload{
		OriginalSubject: "subscription.paused",
		OriginalPayload: `{"type":"subscription.paused"}`,
		Error:           "error",
		MsgID:           "msg-2",
		FailedAt:        time.Now().Format(time.RFC3339),
		RetryCount:      1,
		EntityID:        "sub-other",
	})
	ch <- makeDLQMessage(t, DLQPayload{
		OriginalSubject: "subscription.resumed",
		OriginalPayload: `{"type":"subscription.resumed"}`,
		Error:           "error",
		MsgID:           "msg-3",
		FailedAt:        time.Now().Format(time.RFC3339),
		RetryCount:      2,
		EntityID:        "sub-target",
	})
	close(ch)

	sub := &stubSubscriber{ch: ch}
	pub := &stubPublisher{}

	result, err := replayDLQFromChannel(context.Background(), sub, pub, ReplayOptions{
		FilterEntity: "sub-target",
	})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Replayed)
	assert.Equal(t, 1, result.Skipped)
	require.Len(t, pub.published, 2)
}

func TestReplayDLQMessages_DryRun(t *testing.T) {
	ch := make(chan *message.Message, 2)

	p1 := DLQPayload{
		OriginalSubject: "subscription.cancelled",
		OriginalPayload: `{"type":"subscription.cancelled"}`,
		Error:           "error",
		MsgID:           "msg-1",
		FailedAt:        time.Now().Format(time.RFC3339),
		RetryCount:      3,
		EntityID:        "sub-1",
	}
	p2 := DLQPayload{
		OriginalSubject: "subscription.paused",
		OriginalPayload: `{"type":"subscription.paused"}`,
		Error:           "error",
		MsgID:           "msg-2",
		FailedAt:        time.Now().Format(time.RFC3339),
		RetryCount:      1,
		EntityID:        "sub-2",
	}

	ch <- makeDLQMessage(t, p1)
	ch <- makeDLQMessage(t, p2)
	close(ch)

	sub := &stubSubscriber{ch: ch}
	pub := &stubPublisher{}

	result, err := replayDLQFromChannel(context.Background(), sub, pub, ReplayOptions{
		DryRun: true,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Replayed)
	assert.Equal(t, 0, result.Errors)

	// Dry-run must NOT publish anything.
	assert.Empty(t, pub.published)

	// Matched payloads must be collected.
	require.Len(t, result.Matched, 2)
	assert.Equal(t, "msg-1", result.Matched[0].MsgID)
	assert.Equal(t, "msg-2", result.Matched[1].MsgID)
}

func TestReplayDLQMessages_DryRunWithFilter(t *testing.T) {
	ch := make(chan *message.Message, 3)

	ch <- makeDLQMessage(t, DLQPayload{
		OriginalSubject: "subscription.cancelled",
		OriginalPayload: `{"type":"test"}`,
		Error:           "error",
		MsgID:           "msg-1",
		FailedAt:        time.Now().Format(time.RFC3339),
		EntityID:        "sub-1",
	})
	ch <- makeDLQMessage(t, DLQPayload{
		OriginalSubject: "subscription.paused",
		OriginalPayload: `{"type":"test"}`,
		Error:           "error",
		MsgID:           "msg-2",
		FailedAt:        time.Now().Format(time.RFC3339),
		EntityID:        "sub-2",
	})
	ch <- makeDLQMessage(t, DLQPayload{
		OriginalSubject: "subscription.cancelled",
		OriginalPayload: `{"type":"test"}`,
		Error:           "error",
		MsgID:           "msg-3",
		FailedAt:        time.Now().Format(time.RFC3339),
		EntityID:        "sub-3",
	})
	close(ch)

	sub := &stubSubscriber{ch: ch}
	pub := &stubPublisher{}

	result, err := replayDLQFromChannel(context.Background(), sub, pub, ReplayOptions{
		DryRun:        true,
		FilterSubject: "cancelled",
	})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Replayed)
	assert.Equal(t, 1, result.Skipped)
	assert.Empty(t, pub.published)
	require.Len(t, result.Matched, 2)
}

func TestReplayDLQMessages_PreservesOriginalMsgID(t *testing.T) {
	ch := make(chan *message.Message, 2)

	ch <- makeDLQMessage(t, DLQPayload{
		OriginalSubject: "subscription.cancelled",
		OriginalPayload: `{"type":"test"}`,
		Error:           "error",
		MsgID:           "deterministic-id-1",
		FailedAt:        time.Now().Format(time.RFC3339),
		RetryCount:      3,
	})
	// Empty MsgID should fall back to a generated UUID.
	ch <- makeDLQMessage(t, DLQPayload{
		OriginalSubject: "subscription.paused",
		OriginalPayload: `{"type":"test"}`,
		Error:           "error",
		MsgID:           "",
		FailedAt:        time.Now().Format(time.RFC3339),
		RetryCount:      1,
	})
	close(ch)

	sub := &stubSubscriber{ch: ch}
	pub := &stubPublisher{}

	result, err := replayDLQFromChannel(context.Background(), sub, pub, ReplayOptions{})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Replayed)

	require.Len(t, pub.published, 2)
	// First message: original MsgID preserved.
	assert.Equal(t, "deterministic-id-1", pub.published[0].msgID)
	// Second message: empty MsgID falls back to a generated UUID (non-empty).
	assert.NotEmpty(t, pub.published[1].msgID)
	assert.NotEqual(t, "deterministic-id-1", pub.published[1].msgID)
}

func TestReplayDLQMessages_DelayBetweenPublishes(t *testing.T) {
	ch := make(chan *message.Message, 3)

	for i := 0; i < 3; i++ {
		ch <- makeDLQMessage(t, DLQPayload{
			OriginalSubject: "subscription.cancelled",
			OriginalPayload: `{"type":"test"}`,
			Error:           "error",
			MsgID:           watermill.NewUUID(),
			FailedAt:        time.Now().Format(time.RFC3339),
			RetryCount:      3,
		})
	}
	close(ch)

	sub := &stubSubscriber{ch: ch}
	pub := &stubPublisher{}

	delay := 10 * time.Millisecond
	start := time.Now()

	result, err := replayDLQFromChannel(context.Background(), sub, pub, ReplayOptions{
		DelayBetween: delay,
	})
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, 3, result.Replayed)
	assert.Len(t, pub.published, 3)

	// With 3 messages and delay between publishes, there should be 2 delays
	// (delay is applied after each publish except the last when limit is reached).
	// Allow generous tolerance for CI jitter.
	expectedMinDelay := delay // at least 1 delay must have occurred
	assert.GreaterOrEqual(t, elapsed, expectedMinDelay, "expected at least one delay between publishes")
}

func TestReplayDLQMessages_DelayRespectsContextCancellation(t *testing.T) {
	ch := make(chan *message.Message, 5)

	for i := 0; i < 5; i++ {
		ch <- makeDLQMessage(t, DLQPayload{
			OriginalSubject: "subscription.cancelled",
			OriginalPayload: `{"type":"test"}`,
			Error:           "error",
			MsgID:           watermill.NewUUID(),
			FailedAt:        time.Now().Format(time.RFC3339),
			RetryCount:      3,
		})
	}

	sub := &stubSubscriber{ch: ch}
	pub := &stubPublisher{}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	result, err := replayDLQFromChannel(ctx, sub, pub, ReplayOptions{
		DelayBetween: 1 * time.Second, // long delay, ctx should cancel first
	})

	// Should have replayed at least 1 but been cancelled during the delay.
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.GreaterOrEqual(t, result.Replayed, 1)
	assert.Less(t, result.Replayed, 5)
}

func TestReplayDLQMessages_CombinedSubjectAndEntityFilter(t *testing.T) {
	ch := make(chan *message.Message, 4)

	ch <- makeDLQMessage(t, DLQPayload{
		OriginalSubject: "subscription.cancelled",
		OriginalPayload: `{"type":"test"}`,
		Error:           "error",
		MsgID:           "msg-1",
		FailedAt:        time.Now().Format(time.RFC3339),
		EntityID:        "sub-target",
	})
	ch <- makeDLQMessage(t, DLQPayload{
		OriginalSubject: "subscription.cancelled",
		OriginalPayload: `{"type":"test"}`,
		Error:           "error",
		MsgID:           "msg-2",
		FailedAt:        time.Now().Format(time.RFC3339),
		EntityID:        "sub-other",
	})
	ch <- makeDLQMessage(t, DLQPayload{
		OriginalSubject: "subscription.paused",
		OriginalPayload: `{"type":"test"}`,
		Error:           "error",
		MsgID:           "msg-3",
		FailedAt:        time.Now().Format(time.RFC3339),
		EntityID:        "sub-target",
	})
	ch <- makeDLQMessage(t, DLQPayload{
		OriginalSubject: "subscription.cancelled",
		OriginalPayload: `{"type":"test"}`,
		Error:           "error",
		MsgID:           "msg-4",
		FailedAt:        time.Now().Format(time.RFC3339),
		EntityID:        "sub-target",
	})
	close(ch)

	sub := &stubSubscriber{ch: ch}
	pub := &stubPublisher{}

	result, err := replayDLQFromChannel(context.Background(), sub, pub, ReplayOptions{
		FilterSubject: "cancelled",
		FilterEntity:  "sub-target",
	})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Replayed)
	assert.Equal(t, 2, result.Skipped)
	require.Len(t, pub.published, 2)
	assert.Equal(t, "msg-1", pub.published[0].msgID)
	assert.Equal(t, "msg-4", pub.published[1].msgID)
}

// dlqSubscriber and dlqPublisher interfaces are defined in dlq_replay.go.
