package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeBridge is a test double for BotHostBridge.
type fakeBridge struct {
	calledOp   string
	calledArgs json.RawMessage
	result     json.RawMessage
	err        error
}

func (f *fakeBridge) Call(_ context.Context, op string, args json.RawMessage) (json.RawMessage, error) {
	f.calledOp = op
	f.calledArgs = args
	return f.result, f.err
}

func TestWithBotHost_RoundTrip(t *testing.T) {
	bridge := &fakeBridge{}
	ctx := WithBotHost(context.Background(), bridge)

	got, ok := botHostFromContext(ctx)
	require.True(t, ok, "botHostFromContext should return true when bridge is present")
	assert.Same(t, bridge, got.(*fakeBridge), "botHostFromContext should return the same bridge")
}

func TestBotHostFromContext_MissingBridge(t *testing.T) {
	_, ok := botHostFromContext(context.Background())
	assert.False(t, ok, "botHostFromContext should return false on a plain context")
}

func TestHandleHostCall_ForwardsTobridge(t *testing.T) {
	canned := json.RawMessage(`{"result":"ok"}`)
	bridge := &fakeBridge{result: canned}
	ctx := WithBotHost(context.Background(), bridge)

	input := []byte(`{"op":"x","args":{"a":1}}`)
	out, err := handleHostCall(ctx, input)

	require.NoError(t, err)
	assert.Equal(t, canned, json.RawMessage(out))
	assert.Equal(t, "x", bridge.calledOp)
	assert.JSONEq(t, `{"a":1}`, string(bridge.calledArgs))
}

func TestHandleHostCall_NoBridgeInContext(t *testing.T) {
	input := []byte(`{"op":"x","args":{}}`)
	_, err := handleHostCall(context.Background(), input)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoBotHost), "expected ErrNoBotHost, got: %v", err)
}

func TestHandleHostCall_BridgeReturnsError(t *testing.T) {
	sentinel := errors.New("bridge failure")
	bridge := &fakeBridge{err: sentinel}
	ctx := WithBotHost(context.Background(), bridge)

	input := []byte(`{"op":"fail","args":{}}`)
	_, err := handleHostCall(ctx, input)

	require.Error(t, err)
	assert.True(t, errors.Is(err, sentinel), "expected sentinel error, got: %v", err)
}

func TestHandleHostCall_MalformedInput(t *testing.T) {
	bridge := &fakeBridge{}
	ctx := WithBotHost(context.Background(), bridge)

	_, err := handleHostCall(ctx, []byte("not json"))
	require.Error(t, err, "malformed JSON must produce a non-nil error")
	assert.ErrorContains(t, err, "host_call: decode request")
}

func TestHandleHostCall_NilResult(t *testing.T) {
	// Bridge returning nil result should propagate nil without panic.
	bridge := &fakeBridge{result: nil}
	ctx := WithBotHost(context.Background(), bridge)

	input := []byte(`{"op":"noop","args":{}}`)
	out, err := handleHostCall(ctx, input)

	require.NoError(t, err)
	assert.Nil(t, out, "nil bridge result should be returned as nil")
}
