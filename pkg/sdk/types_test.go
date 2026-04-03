package sdk

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookContext_JSONRoundTrip(t *testing.T) {
	ctx := HookContext{
		HookName:  "invoice.created",
		RequestID: "req-123",
		Timestamp: 1700000000,
		PluginID:  "plugin-abc",
		Payload:   json.RawMessage(`{"amount":1000}`),
	}

	data, err := json.Marshal(ctx)
	require.NoError(t, err)

	var decoded HookContext
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, ctx.HookName, decoded.HookName)
	assert.Equal(t, ctx.RequestID, decoded.RequestID)
	assert.Equal(t, ctx.Timestamp, decoded.Timestamp)
	assert.Equal(t, ctx.PluginID, decoded.PluginID)
	assert.JSONEq(t, `{"amount":1000}`, string(decoded.Payload))
}

func TestHookResult_JSONRoundTrip(t *testing.T) {
	result := HookResult{
		Action:   ActionModify,
		Modified: json.RawMessage(`{"amount":2000}`),
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded HookResult
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, ActionModify, decoded.Action)
	assert.JSONEq(t, `{"amount":2000}`, string(decoded.Modified))
	assert.Empty(t, decoded.Error)
}

func TestHookResult_HaltWithError(t *testing.T) {
	result := HookResult{
		Action: ActionHalt,
		Error:  "insufficient balance",
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded HookResult
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, ActionHalt, decoded.Action)
	assert.Equal(t, "insufficient balance", decoded.Error)
	assert.Nil(t, decoded.Modified) // omitempty
}

func TestHookAction_Values(t *testing.T) {
	assert.Equal(t, HookAction("continue"), ActionContinue)
	assert.Equal(t, HookAction("modify"), ActionModify)
	assert.Equal(t, HookAction("halt"), ActionHalt)
}

func TestHTTPRequest_JSON(t *testing.T) {
	req := HTTPRequest{
		Method:  "POST",
		URL:     "https://api.example.com/webhook",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{"event":"test"}`),
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded HTTPRequest
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "POST", decoded.Method)
	assert.Equal(t, "https://api.example.com/webhook", decoded.URL)
	assert.Equal(t, "application/json", decoded.Headers["Content-Type"])
}

func TestStorageEntry_JSON(t *testing.T) {
	entry := StorageEntry{
		Key:   "user:points:123",
		Value: []byte("500"),
	}

	data, err := json.Marshal(entry)
	require.NoError(t, err)

	var decoded StorageEntry
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "user:points:123", decoded.Key)
	assert.Equal(t, []byte("500"), decoded.Value)
}
