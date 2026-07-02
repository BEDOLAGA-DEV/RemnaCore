package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
)

func TestToPluginResponse_ProvidesBot(t *testing.T) {
	bot := &plugin.Plugin{
		Slug:     "samplebot",
		Manifest: &plugin.Manifest{Telegram: &plugin.ManifestTelegram{ProvidesBot: true}},
	}
	assert.True(t, toPluginResponse(bot).ProvidesBot)

	nonBot := &plugin.Plugin{Slug: "tariff-manager", Manifest: &plugin.Manifest{}}
	assert.False(t, toPluginResponse(nonBot).ProvidesBot)

	nilManifest := &plugin.Plugin{Slug: "broken"}
	assert.False(t, toPluginResponse(nilManifest).ProvidesBot)
}
