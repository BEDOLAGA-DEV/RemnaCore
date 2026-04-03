package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPluginInstalledEvent(t *testing.T) {
	e := NewPluginInstalledEvent("id-1", "my-plugin", "1.0.0")
	assert.Equal(t, EventPluginInstalled, e.Type)
	assert.Equal(t, "id-1", e.Data["plugin_id"])
	assert.Equal(t, "my-plugin", e.Data["slug"])
	assert.Equal(t, "1.0.0", e.Data["version"])
	assert.False(t, e.Timestamp.IsZero())
}

func TestNewPluginEnabledEvent(t *testing.T) {
	e := NewPluginEnabledEvent("id-1", "my-plugin")
	assert.Equal(t, EventPluginEnabled, e.Type)
	assert.Equal(t, "id-1", e.Data["plugin_id"])
	assert.Equal(t, "my-plugin", e.Data["slug"])
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
	assert.Equal(t, "out of memory", e.Data["reason"])
}

func TestNewHookExecutedEvent(t *testing.T) {
	e := NewHookExecutedEvent("id-1", "my-plugin", "invoice.created", 42)
	assert.Equal(t, EventHookExecuted, e.Type)
	assert.Equal(t, "invoice.created", e.Data["hook_name"])
	assert.Equal(t, int64(42), e.Data["duration_ms"])
}

func TestNewHookFailedEvent(t *testing.T) {
	e := NewHookFailedEvent("id-1", "my-plugin", "invoice.created", "timed out")
	assert.Equal(t, EventHookFailed, e.Type)
	assert.Equal(t, "timed out", e.Data["reason"])
}
