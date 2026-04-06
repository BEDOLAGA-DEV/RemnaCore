package plugin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validTestManifest() *Manifest {
	m, _ := ParseManifest([]byte(minimalManifestTOML))
	return m
}

func TestNewPlugin(t *testing.T) {
	m := validTestManifest()
	require.NotNil(t, m)

	now := time.Now()
	p, err := NewPlugin(m, nil, now)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.NotEmpty(t, p.ID)
	assert.Equal(t, "minimal-plugin", p.Slug)
	assert.Equal(t, "Minimal", p.Name)
	assert.Equal(t, "0.0.1", p.Version)
	assert.Equal(t, StatusInstalled, p.Status)
	assert.NotNil(t, p.Config)
	assert.Nil(t, p.EnabledAt)
	assert.Equal(t, now, p.InstalledAt)
	assert.Equal(t, now, p.UpdatedAt)
}

func TestNewPlugin_NilManifest(t *testing.T) {
	_, err := NewPlugin(nil, nil, time.Now())
	require.ErrorIs(t, err, ErrInvalidManifest)
}

func TestPlugin_Enable(t *testing.T) {
	m := validTestManifest()
	p, err := NewPlugin(m, nil, time.Now())
	require.NoError(t, err)

	now := time.Now()
	err = p.Enable(now)
	require.NoError(t, err)
	assert.Equal(t, StatusEnabled, p.Status)
	assert.NotNil(t, p.EnabledAt)
	assert.Equal(t, now, *p.EnabledAt)
}

func TestPlugin_EnableAlreadyEnabled(t *testing.T) {
	m := validTestManifest()
	p, err := NewPlugin(m, nil, time.Now())
	require.NoError(t, err)

	require.NoError(t, p.Enable(time.Now()))
	err = p.Enable(time.Now())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestPlugin_Disable(t *testing.T) {
	m := validTestManifest()
	p, err := NewPlugin(m, nil, time.Now())
	require.NoError(t, err)

	require.NoError(t, p.Enable(time.Now()))
	now := time.Now()
	err = p.Disable(now)
	require.NoError(t, err)
	assert.Equal(t, StatusDisabled, p.Status)
	assert.Equal(t, now, p.UpdatedAt)
}

func TestPlugin_SetError(t *testing.T) {
	m := validTestManifest()
	p, err := NewPlugin(m, nil, time.Now())
	require.NoError(t, err)

	now := time.Now()
	err = p.SetError("compilation failed", now)
	require.NoError(t, err)
	assert.Equal(t, StatusError, p.Status)
	assert.Equal(t, "compilation failed", p.ErrorLog)
	assert.Equal(t, now, p.UpdatedAt)
}

func TestPlugin_Transitions(t *testing.T) {
	tests := []struct {
		name      string
		from      PluginStatus
		action    string
		wantErr   bool
		wantState PluginStatus
	}{
		{
			name:      "installed to enabled",
			from:      StatusInstalled,
			action:    "enable",
			wantErr:   false,
			wantState: StatusEnabled,
		},
		{
			name:    "installed to disabled is invalid",
			from:    StatusInstalled,
			action:  "disable",
			wantErr: true,
		},
		{
			name:      "enabled to error",
			from:      StatusEnabled,
			action:    "error",
			wantErr:   false,
			wantState: StatusError,
		},
		{
			name:      "enabled to disabled",
			from:      StatusEnabled,
			action:    "disable",
			wantErr:   false,
			wantState: StatusDisabled,
		},
		{
			name:      "disabled to enabled (re-enable)",
			from:      StatusDisabled,
			action:    "enable",
			wantErr:   false,
			wantState: StatusEnabled,
		},
		{
			name:      "error to enabled (recovery)",
			from:      StatusError,
			action:    "enable",
			wantErr:   false,
			wantState: StatusEnabled,
		},
		{
			name:      "error to disabled",
			from:      StatusError,
			action:    "disable",
			wantErr:   false,
			wantState: StatusDisabled,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := validTestManifest()
			p, err := NewPlugin(m, nil, time.Now())
			require.NoError(t, err)

			// Force the plugin into the desired starting state.
			p.Status = tc.from

			now := time.Now()
			switch tc.action {
			case "enable":
				err = p.Enable(now)
			case "disable":
				err = p.Disable(now)
			case "error":
				err = p.SetError("test error", now)
			}

			if tc.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidTransition)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantState, p.Status)
			}
		})
	}
}

func TestPlugin_EnableClearsErrorLog(t *testing.T) {
	m := validTestManifest()
	p, err := NewPlugin(m, nil, time.Now())
	require.NoError(t, err)

	require.NoError(t, p.SetError("something broke", time.Now()))
	assert.Equal(t, "something broke", p.ErrorLog)

	require.NoError(t, p.Enable(time.Now()))
	assert.Equal(t, StatusEnabled, p.Status)
	assert.Empty(t, p.ErrorLog, "ErrorLog should be cleared on recovery")
}
