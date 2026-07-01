package telegram

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
)

func TestBuiltinBotRegistry_RegisterLookup(t *testing.T) {
	r := NewBuiltinBotRegistry()
	called := false
	h := func(_ context.Context, _ bothost.Update, _ bothost.Host) error {
		called = true
		return nil
	}
	perms := bothost.NewPermSet(plugin.PermTelegramSend)
	r.Register("cabinet-bot", perms, h)

	got, gotPerms, ok := r.Lookup("cabinet-bot")
	require.True(t, ok)
	require.True(t, gotPerms.Has(plugin.PermTelegramSend))
	require.NoError(t, got(context.Background(), bothost.Update{}, nil))
	require.True(t, called, "the looked-up handler must be the registered one")
}

func TestBuiltinBotRegistry_LookupUnknown(t *testing.T) {
	r := NewBuiltinBotRegistry()
	_, _, ok := r.Lookup("nope")
	require.False(t, ok)
}
