package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalBotManifest returns a valid bot manifest struct with no hooks.
// entry is assigned directly to Telegram.Entry before Validate is called.
func minimalBotManifest(entry string) *Manifest {
	return &Manifest{
		Plugin: ManifestPlugin{
			ID: "sample-bot", Name: "Sample Bot", Version: "1.0.0", SDKVersion: "1.0.0",
		},
		Telegram: &ManifestTelegram{ProvidesBot: true, Entry: entry},
	}
}

// TestManifestTelegram groups all [telegram] section and permission tests.
func TestManifestTelegram(t *testing.T) {
	// (a) Bot manifest with provides_bot=true and zero hooks must pass Validate,
	//     and empty Entry must be defaulted to DefaultBotEntry.
	t.Run("bot_exempt_from_hook_requirement", func(t *testing.T) {
		m := minimalBotManifest("")
		err := m.Validate()
		require.NoError(t, err, "bot manifest with no hooks must pass validation")
		assert.Equal(t, DefaultBotEntry, m.Telegram.Entry,
			"empty Entry must be normalised to DefaultBotEntry after Validate")
	})

	// (b) Entry validation: valid identifier passes; invalid identifier fails.
	t.Run("valid_custom_entry_accepted", func(t *testing.T) {
		m := minimalBotManifest("my_entry")
		require.NoError(t, m.Validate())
		assert.Equal(t, "my_entry", m.Telegram.Entry)
	})

	t.Run("invalid_entry_rejected", func(t *testing.T) {
		m := minimalBotManifest("Bad Entry!")
		err := m.Validate()
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidManifest)
		assert.Contains(t, err.Error(), "Bad Entry!")
	})

	// (c) Non-bot manifest (Telegram nil) with zero hooks must still fail.
	t.Run("non_bot_still_requires_hooks", func(t *testing.T) {
		m := &Manifest{
			Plugin: ManifestPlugin{
				ID: "hook-test", Name: "Hook Test", Version: "1.0.0", SDKVersion: "1.0.0",
			},
			// Telegram is nil — not a bot
		}
		err := m.Validate()
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidManifest)
		assert.Contains(t, err.Error(), "hook")
	})

	// (d) ParsePermissions: telegram="send" yields PermTelegramSend;
	//     users="write" still yields PermUsersWrite (regression guard).
	t.Run("parse_permissions_telegram_send", func(t *testing.T) {
		m := &Manifest{
			Permissions: ManifestPermissions{Telegram: PermValueSend},
		}
		perms := m.ParsePermissions()
		assert.Contains(t, perms, PermTelegramSend)
	})

	t.Run("parse_permissions_users_write_regression", func(t *testing.T) {
		m := &Manifest{
			Permissions: ManifestPermissions{Users: PermValueWrite},
		}
		perms := m.ParsePermissions()
		assert.Contains(t, perms, PermUsersRead)
		assert.Contains(t, perms, PermUsersWrite)
	})

	// Round-trip: parse a TOML bot manifest, confirm Telegram fields and
	// ParsePermissions both work end-to-end.
	t.Run("parse_manifest_bot_toml", func(t *testing.T) {
		const botTOML = `
[plugin]
id          = "sample-bot"
name        = "Sample Bot"
version     = "1.0.0"
sdk_version = "1.0.0"

[permissions]
telegram = "send"

[telegram]
provides_bot = true
`
		m, err := ParseManifest([]byte(botTOML))
		require.NoError(t, err)
		require.NotNil(t, m.Telegram)
		assert.True(t, m.Telegram.ProvidesBot)
		assert.Equal(t, DefaultBotEntry, m.Telegram.Entry,
			"absent entry in TOML must default to DefaultBotEntry")
		perms := m.ParsePermissions()
		assert.Contains(t, perms, PermTelegramSend)
	})
}
