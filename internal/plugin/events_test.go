package plugin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPluginInstalledEvent(t *testing.T) {
	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	e := NewPluginInstalledEvent("id-1", "my-plugin", "1.0.0", now)
	assert.Equal(t, EventPluginInstalled, e.Type)

	data := e.DataAsMap()
	require.NotNil(t, data)
	assert.Equal(t, "id-1", data["plugin_id"])
	assert.Equal(t, "my-plugin", data["slug"])
	assert.Equal(t, "1.0.0", data["version"])
	assert.Equal(t, now, e.Timestamp)
}

func TestNewPluginEnabledEvent(t *testing.T) {
	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	e := NewPluginEnabledEvent("id-1", "my-plugin", now)
	assert.Equal(t, EventPluginEnabled, e.Type)

	data := e.DataAsMap()
	require.NotNil(t, data)
	assert.Equal(t, "id-1", data["plugin_id"])
	assert.Equal(t, "my-plugin", data["slug"])
	assert.Equal(t, now, e.Timestamp)
}

func TestNewPluginDisabledEvent(t *testing.T) {
	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	e := NewPluginDisabledEvent("id-1", "my-plugin", now)
	assert.Equal(t, EventPluginDisabled, e.Type)
	assert.Equal(t, now, e.Timestamp)
}

func TestNewPluginUninstalledEvent(t *testing.T) {
	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	e := NewPluginUninstalledEvent("id-1", "my-plugin", now)
	assert.Equal(t, EventPluginUninstalled, e.Type)
	assert.Equal(t, now, e.Timestamp)
}

func TestNewPluginErrorEvent(t *testing.T) {
	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	e := NewPluginErrorEvent("id-1", "my-plugin", "out of memory", now)
	assert.Equal(t, EventPluginError, e.Type)

	data := e.DataAsMap()
	require.NotNil(t, data)
	assert.Equal(t, "out of memory", data["reason"])
	assert.Equal(t, now, e.Timestamp)
}

func TestNewHookExecutedEvent(t *testing.T) {
	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	e := NewHookExecutedEvent("id-1", "my-plugin", "invoice.created", 42, now)
	assert.Equal(t, EventHookExecuted, e.Type)

	data := e.DataAsMap()
	require.NotNil(t, data)
	assert.Equal(t, "invoice.created", data["hook_name"])
	assert.Equal(t, int64(42), data["duration_ms"])
	assert.Equal(t, now, e.Timestamp)
}

func TestNewHookFailedEvent(t *testing.T) {
	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	e := NewHookFailedEvent("id-1", "my-plugin", "invoice.created", "timed out", now)
	assert.Equal(t, EventHookFailed, e.Type)

	data := e.DataAsMap()
	require.NotNil(t, data)
	assert.Equal(t, "timed out", data["reason"])
	assert.Equal(t, now, e.Timestamp)
}
