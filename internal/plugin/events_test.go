package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPluginInstalledEvent(t *testing.T) {
	e := NewPluginInstalledEvent("id-1", "my-plugin", "1.0.0")
	assert.Equal(t, EventPluginInstalled, e.Type)

	data := e.DataAsMap()
	require.NotNil(t, data)
	assert.Equal(t, "id-1", data["plugin_id"])
	assert.Equal(t, "my-plugin", data["slug"])
	assert.Equal(t, "1.0.0", data["version"])
	assert.False(t, e.Timestamp.IsZero())
}

func TestNewPluginEnabledEvent(t *testing.T) {
	e := NewPluginEnabledEvent("id-1", "my-plugin")
	assert.Equal(t, EventPluginEnabled, e.Type)

	data := e.DataAsMap()
	require.NotNil(t, data)
	assert.Equal(t, "id-1", data["plugin_id"])
	assert.Equal(t, "my-plugin", data["slug"])
}

func TestNewPluginDisabledEvent(t *testing.T) {
	e := NewPluginDisabledEvent("id-1", "my-plugin")
	assert.Equal(t, EventPluginDisabled, e.Type)
}

func TestNewPluginUninstalledEvent(t *testing.T) {
	e := NewPluginUninstalledEvent("id-1", "my-plugin")
	assert.Equal(t, EventPluginUninstalled, e.Type)
}

func TestNewPluginErrorEvent(t *testing.T) {
	e := NewPluginErrorEvent("id-1", "my-plugin", "out of memory")
	assert.Equal(t, EventPluginError, e.Type)

	data := e.DataAsMap()
	require.NotNil(t, data)
	assert.Equal(t, "out of memory", data["reason"])
}

func TestNewHookExecutedEvent(t *testing.T) {
	e := NewHookExecutedEvent("id-1", "my-plugin", "invoice.created", 42)
	assert.Equal(t, EventHookExecuted, e.Type)

	data := e.DataAsMap()
	require.NotNil(t, data)
	assert.Equal(t, "invoice.created", data["hook_name"])
	assert.Equal(t, int64(42), data["duration_ms"])
}

func TestNewHookFailedEvent(t *testing.T) {
	e := NewHookFailedEvent("id-1", "my-plugin", "invoice.created", "timed out")
	assert.Equal(t, EventHookFailed, e.Type)

	data := e.DataAsMap()
	require.NotNil(t, data)
	assert.Equal(t, "timed out", data["reason"])
}
