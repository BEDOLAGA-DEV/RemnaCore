package telegram

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/cabinetbot"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
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

// newShopBotWASM builds a minimal shop Bot with a WASMBotDispatcher for offline
// dispatch tests. The botRegistry does NOT contain slug so Lookup fails, forcing
// the WASM branch.
func newShopBotWASM(slug string, reg *BuiltinBotRegistry, wasmBots WASMBotDispatcher) *Bot {
	return &Bot{
		isShop:        true,
		tenantID:      "tenant-wasm",
		cabinetURL:    "https://shop.example.com/cabinet",
		botPluginSlug: slug,
		botRegistry:   reg,
		botOps:        botHostRegistryForTest(),
		wasmBots:      wasmBots,
		txRunner:      nil,
		logger:        testLogger(),
	}
}

// TestDispatch_WASMPlugin_InvokesDispatcherAndSkipsCabinetBot verifies that when
// the builtin registry has no handler for slug but WASMBotDispatcher.Resolve
// returns ok=true, dispatchUpdate calls Invoke with the marshalled update and
// does NOT fall back to the cabinet-bot handler.
func TestDispatch_WASMPlugin_InvokesDispatcherAndSkipsCabinetBot(t *testing.T) {
	reg := NewBuiltinBotRegistry()

	// Register a recording cabinet-bot handler so we can assert it is NOT called.
	cabinetCalled := false
	reg.Register(cabinetbot.Slug, bothost.NewPermSet(), func(_ context.Context, _ bothost.Update, _ bothost.Host) error {
		cabinetCalled = true
		return nil
	})
	// "wasmbot" is intentionally NOT registered in reg → Lookup will fail.

	wasm := &fakeWASMDispatcher{
		resolvePerms: bothost.NewPermSet(plugin.PermTelegramSend),
		resolveEntry: plugin.DefaultBotEntry,
		resolveOK:    true,
	}

	b := newShopBotWASM("wasmbot", reg, wasm)
	u := syntheticMessageUpdate(9001, 4002, "Eve", "/start")

	b.dispatchUpdate(context.Background(), nil, u)

	require.True(t, wasm.invokeCalled, "WASM Invoke must be called")
	assert.Equal(t, "wasmbot", wasm.invokeSlug)
	assert.Equal(t, plugin.DefaultBotEntry, wasm.invokeEntry)
	assert.False(t, cabinetCalled, "cabinet-bot fallback must NOT be called when WASM handles the update")

	// Verify envelope round-trips back to the expected bothost.Update fields.
	var got bothost.Update
	require.NoError(t, json.Unmarshal(wasm.invokeEnv, &got))
	assert.Equal(t, int64(9001), got.ChatID)
	assert.Equal(t, int64(4002), got.From.ID)
	assert.Equal(t, "Eve", got.From.FirstName)
	assert.Equal(t, "/start", got.Text)
}

// TestDispatch_WASMPlugin_ResolveFalse_FallsBackToCabinetBot verifies that when
// WASMBotDispatcher.Resolve returns ok=false, dispatch continues to the
// cabinet-bot fallback handler.
func TestDispatch_WASMPlugin_ResolveFalse_FallsBackToCabinetBot(t *testing.T) {
	reg := NewBuiltinBotRegistry()

	cabinetFake := &fakeHandler{}
	reg.Register(cabinetbot.Slug, bothost.NewPermSet(), cabinetFake.handle)

	wasm := &fakeWASMDispatcher{resolveOK: false}
	b := newShopBotWASM("wasmbot", reg, wasm)

	b.dispatchUpdate(context.Background(), nil, syntheticMessageUpdate(1, 2, "Frank", "/start"))

	assert.False(t, wasm.invokeCalled, "WASM Invoke must NOT be called when Resolve is false")
	assert.True(t, cabinetFake.called, "cabinet-bot fallback must run when WASM Resolve returns false")
}

// ── Domain-readers threading ──────────────────────────────────────────────────

// stubTariffReader is a minimal bothost.TariffReader stub for threading tests.
type stubTariffReader struct{}

func (stubTariffReader) ListVisible(_ context.Context, _ string) ([]bothost.TariffOffer, error) {
	return nil, nil
}
func (stubTariffReader) Get(_ context.Context, _ string) (*bothost.TariffOffer, error) {
	return nil, nil
}

// TestNewOpContext_DomainReadersThreaded verifies that a shop bot constructed
// with a non-nil DomainReaders.Tariffs surfaces that reader in OpContext.Tariffs,
// while unset reader fields remain nil.
func TestNewOpContext_DomainReadersThreaded(t *testing.T) {
	stub := stubTariffReader{}
	b := &Bot{
		isShop:     true,
		tenantID:   "tenant-readers-test",
		cabinetURL: "https://example.com/cabinet",
		identity:   nil,
		txRunner:   nil,
		botOps:     botHostRegistryForTest(),
		logger:     testLogger(),
		readers: DomainReaders{
			Tariffs: stub,
		},
	}

	oc := b.newOpContext()

	assert.Equal(t, stub, oc.Tariffs, "Tariffs must be threaded from DomainReaders into OpContext")
	assert.Nil(t, oc.Subs, "Subs must be nil when not set in DomainReaders")
	assert.Nil(t, oc.Invoices, "Invoices must be nil when not set in DomainReaders")
	assert.Nil(t, oc.Balance, "Balance must be nil when not set in DomainReaders")
	assert.Nil(t, oc.Checkout, "Checkout must be nil when not set in DomainReaders")
}
