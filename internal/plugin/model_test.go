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
	require.ErrorIs(t, err, ErrPluginAlreadyEnabled)
}

func TestPlugin_Disable(t *testing.T) {
	m := validTestManifest()
	p, err := NewPlugin(m, nil, time.Now())
	require.NoError(t, err)

	require.NoError(t, p.Enable(time.Now()))
	now := time.Now()
	p.Disable(now)
	assert.Equal(t, StatusDisabled, p.Status)
	assert.Equal(t, now, p.UpdatedAt)
}

func TestPlugin_SetError(t *testing.T) {
	m := validTestManifest()
	p, err := NewPlugin(m, nil, time.Now())
	require.NoError(t, err)

	now := time.Now()
	p.SetError("compilation failed", now)
	assert.Equal(t, StatusError, p.Status)
	assert.Equal(t, "compilation failed", p.ErrorLog)
	assert.Equal(t, now, p.UpdatedAt)
}
