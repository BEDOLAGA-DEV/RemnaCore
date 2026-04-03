package plugin

import (
	"testing"

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

	p, err := NewPlugin(m, nil)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.NotEmpty(t, p.ID)
	assert.Equal(t, "minimal-plugin", p.Slug)
	assert.Equal(t, "Minimal", p.Name)
	assert.Equal(t, "0.0.1", p.Version)
	assert.Equal(t, StatusInstalled, p.Status)
	assert.NotNil(t, p.Config)
	assert.Nil(t, p.EnabledAt)
	assert.False(t, p.InstalledAt.IsZero())
	assert.False(t, p.UpdatedAt.IsZero())
}

func TestNewPlugin_NilManifest(t *testing.T) {
	_, err := NewPlugin(nil, nil)
	require.ErrorIs(t, err, ErrInvalidManifest)
}

func TestPlugin_Enable(t *testing.T) {
	m := validTestManifest()
	p, err := NewPlugin(m, nil)
	require.NoError(t, err)

	err = p.Enable()
	require.NoError(t, err)
	assert.Equal(t, StatusEnabled, p.Status)
	assert.NotNil(t, p.EnabledAt)
}

func TestPlugin_EnableAlreadyEnabled(t *testing.T) {
	m := validTestManifest()
	p, err := NewPlugin(m, nil)
	require.NoError(t, err)

	require.NoError(t, p.Enable())
	err = p.Enable()
	require.ErrorIs(t, err, ErrPluginAlreadyEnabled)
}

func TestPlugin_Disable(t *testing.T) {
	m := validTestManifest()
	p, err := NewPlugin(m, nil)
	require.NoError(t, err)

	require.NoError(t, p.Enable())
	p.Disable()
	assert.Equal(t, StatusDisabled, p.Status)
}

func TestPlugin_SetError(t *testing.T) {
	m := validTestManifest()
	p, err := NewPlugin(m, nil)
	require.NoError(t, err)

	p.SetError("compilation failed")
	assert.Equal(t, StatusError, p.Status)
	assert.Equal(t, "compilation failed", p.ErrorLog)
}
