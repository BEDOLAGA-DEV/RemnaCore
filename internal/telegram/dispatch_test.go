package telegram

import (
	"context"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/cabinetbot"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
)

// botHostRegistryForTest returns an empty bothost.Registry suitable for tests
// that use fake handlers (which never call any ops).
func botHostRegistryForTest() *bothost.Registry {
	return bothost.NewRegistry()
}

// fakeHandler records the most-recent invocation for assertion in tests.
type fakeHandler struct {
	called bool
	update bothost.Update
}

func (f *fakeHandler) handle(_ context.Context, upd bothost.Update, _ bothost.Host) error {
	f.called = true
	f.update = upd
	return nil
}

// newShopBotDispatch builds a minimal shop Bot for offline dispatch tests.
// It does not call Init (no network), so b.bot is nil; dispatchUpdate is called
// directly.
func newShopBotDispatch(slug string, reg *BuiltinBotRegistry) *Bot {
	return &Bot{
		isShop:        true,
		tenantID:      "tenant-test",
		cabinetURL:    "https://shop.example.com/cabinet",
		botPluginSlug: slug,
		botRegistry:   reg,
		botOps:        botHostRegistryForTest(),
		txRunner:      nil,
		logger:        testLogger(),
	}
}

// syntheticMessageUpdate builds a minimal *models.Update carrying a text message.
func syntheticMessageUpdate(chatID int64, userID int64, firstName, text string) *models.Update {
	return &models.Update{
		Message: &models.Message{
			ID:   42,
			Chat: models.Chat{ID: chatID},
			From: &models.User{
				ID:        userID,
				FirstName: firstName,
			},
			Text: text,
		},
	}
}

// TestDispatch_MessageUpdate_RoutesToRegisteredHandler verifies that a message
// update is correctly mapped and forwarded to the registered plugin handler.
func TestDispatch_MessageUpdate_RoutesToRegisteredHandler(t *testing.T) {
	reg := NewBuiltinBotRegistry()
	fake := &fakeHandler{}
	reg.Register("my-plugin", bothost.NewPermSet(), fake.handle)

	b := newShopBotDispatch("my-plugin", reg)
	u := syntheticMessageUpdate(1001, 2002, "Alice", "/start")

	b.dispatchUpdate(context.Background(), nil, u)

	require.True(t, fake.called, "handler must be invoked")
	assert.Equal(t, int64(1001), fake.update.ChatID)
	assert.Equal(t, int64(2002), fake.update.From.ID)
	assert.Equal(t, "Alice", fake.update.From.FirstName)
	assert.Equal(t, "/start", fake.update.Text)
	assert.Equal(t, 42, fake.update.MessageID)
}

// TestDispatch_EmptySlug_ResolvesToCabinetBot verifies that an empty
// botPluginSlug resolves to the cabinet-bot default.
func TestDispatch_EmptySlug_ResolvesToCabinetBot(t *testing.T) {
	reg := NewBuiltinBotRegistry()
	fake := &fakeHandler{}
	reg.Register(cabinetbot.Slug, bothost.NewPermSet(), fake.handle)

	b := newShopBotDispatch("", reg) // empty slug → cabinet-bot
	b.dispatchUpdate(context.Background(), nil, syntheticMessageUpdate(1, 2, "Bob", "/start"))

	require.True(t, fake.called, "cabinet-bot handler must be invoked for empty slug")
}

// TestDispatch_UnknownSlug_FallsBackToCabinetBot verifies that an unknown slug
// falls back to the cabinet-bot rather than silently dropping the update.
func TestDispatch_UnknownSlug_FallsBackToCabinetBot(t *testing.T) {
	reg := NewBuiltinBotRegistry()
	fake := &fakeHandler{}
	reg.Register(cabinetbot.Slug, bothost.NewPermSet(), fake.handle)
	// "wasm-bot" is not registered → fall back to cabinet-bot

	b := newShopBotDispatch("wasm-bot", reg)
	b.dispatchUpdate(context.Background(), nil, syntheticMessageUpdate(1, 2, "Carol", "/start"))

	require.True(t, fake.called, "cabinet-bot fallback must run when slug is absent")
}

// TestDispatch_NoMessage_IsIgnored verifies that updates with neither a message
// nor a callback query are silently dropped.
func TestDispatch_NoMessage_IsIgnored(t *testing.T) {
	reg := NewBuiltinBotRegistry()
	fake := &fakeHandler{}
	reg.Register(cabinetbot.Slug, bothost.NewPermSet(), fake.handle)

	b := newShopBotDispatch("", reg)
	// An update with no message and no callback query → should not call any handler.
	b.dispatchUpdate(context.Background(), nil, &models.Update{})

	assert.False(t, fake.called, "handler must NOT be invoked for empty update")
}

// TestToBotHostUpdate_Message checks message-update mapping.
func TestToBotHostUpdate_Message(t *testing.T) {
	u := syntheticMessageUpdate(999, 888, "Dana", "hello")
	got, ok := toBotHostUpdate(u)
	require.True(t, ok)
	assert.Equal(t, int64(999), got.ChatID)
	assert.Equal(t, int64(888), got.From.ID)
	assert.Equal(t, "Dana", got.From.FirstName)
	assert.Equal(t, "hello", got.Text)
	assert.Equal(t, 42, got.MessageID)
	assert.False(t, got.IsCallback)
}

// TestToBotHostUpdate_EmptyUpdate returns false for an update with no message/callback.
func TestToBotHostUpdate_EmptyUpdate(t *testing.T) {
	_, ok := toBotHostUpdate(&models.Update{})
	assert.False(t, ok)
}
