package bothost

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/identitytest"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager/txmanagertest"
)

// --- Stub MessageSender ---

// stubSender records every outbound Telegram call for assertion in tests.
type stubSender struct {
	sendTextCalls []struct {
		ChatID int64
		Text   string
	}
	sendKeyboardCalls []struct {
		ChatID int64
		Text   string
		KB     Keyboard
	}
	answerCallbackCalls []struct {
		CallbackID string
		Text       string
	}
	editMessageCalls []struct {
		ChatID    int64
		MessageID int
		Text      string
		KB        *Keyboard
	}
}

func (s *stubSender) SendText(_ context.Context, chatID int64, text string) error {
	s.sendTextCalls = append(s.sendTextCalls, struct {
		ChatID int64
		Text   string
	}{chatID, text})
	return nil
}

func (s *stubSender) SendKeyboard(_ context.Context, chatID int64, text string, kb Keyboard) error {
	s.sendKeyboardCalls = append(s.sendKeyboardCalls, struct {
		ChatID int64
		Text   string
		KB     Keyboard
	}{chatID, text, kb})
	return nil
}

func (s *stubSender) AnswerCallback(_ context.Context, callbackID, text string) error {
	s.answerCallbackCalls = append(s.answerCallbackCalls, struct {
		CallbackID string
		Text       string
	}{callbackID, text})
	return nil
}

func (s *stubSender) EditMessage(_ context.Context, chatID int64, messageID int, text string, kb *Keyboard) error {
	s.editMessageCalls = append(s.editMessageCalls, struct {
		ChatID    int64
		MessageID int
		Text      string
		KB        *Keyboard
	}{chatID, messageID, text, kb})
	return nil
}

// --- noopEventPublisher ---

// noopEventPublisher discards domain events — used so RegisterViaTelegram's
// PublishAll does not need a real NATS connection in unit tests.
type noopEventPublisher struct{}

func (noopEventPublisher) Publish(_ context.Context, _ domainevent.Event) error { return nil }
func (noopEventPublisher) PublishBatch(_ context.Context, _ []domainevent.Event) error {
	return nil
}

// --- Helpers ---

// newTestIdentityService wires a real identity.Service backed by the given
// MockRepository and a NoopTxRunner, mirroring the pattern used in
// internal/gateway/handler tests.
func newTestIdentityService(t *testing.T, repo *identitytest.MockRepository) *identity.Service {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	jwtIssuer := authutil.NewJWTIssuer(key, &key.PublicKey)
	pub := &noopEventPublisher{}
	sessions := identity.NewSessionIssuer(repo, pub, jwtIssuer, 15*time.Minute, 7*24*time.Hour)
	return identity.NewService(
		repo, pub, txmanagertest.NoopTxRunner{}, jwtIssuer,
		clock.NewReal(), 15*time.Minute, 7*24*time.Hour, sessions,
	)
}

// callStdOp registers all standard ops and invokes op with the given
// permission and JSON-serialised args. It fails the test on marshal or Call error.
func callStdOp(t *testing.T, oc *OpContext, perm plugin.PermissionScope, op string, args any) json.RawMessage {
	t.Helper()
	r := NewRegistry()
	RegisterStdOps(r)
	raw, err := json.Marshal(args)
	require.NoError(t, err)
	out, err := r.Call(context.Background(), oc, NewPermSet(perm), op, raw)
	require.NoError(t, err)
	return out
}

// --- Telegram send-op tests ---

func TestOp_SendText(t *testing.T) {
	sender := &stubSender{}
	oc := &OpContext{TenantID: "t1", CabinetURL: "https://cab.example.com", Sender: sender}

	callStdOp(t, oc, plugin.PermTelegramSend, opTelegramSendText, sendTextArgs{
		ChatID: 42,
		Text:   "hello world",
	})

	require.Len(t, sender.sendTextCalls, 1)
	require.Equal(t, int64(42), sender.sendTextCalls[0].ChatID)
	require.Equal(t, "hello world", sender.sendTextCalls[0].Text)
}

func TestOp_SendKeyboard(t *testing.T) {
	sender := &stubSender{}
	oc := &OpContext{TenantID: "t1", Sender: sender}
	kb := Keyboard{Rows: [][]Button{{{Text: "Go", CallbackData: "go:1"}}}}

	callStdOp(t, oc, plugin.PermTelegramSend, opTelegramSendKeyboard, sendKeyboardArgs{
		ChatID:   99,
		Text:     "choose",
		Keyboard: kb,
	})

	require.Len(t, sender.sendKeyboardCalls, 1)
	c := sender.sendKeyboardCalls[0]
	require.Equal(t, int64(99), c.ChatID)
	require.Equal(t, "choose", c.Text)
	require.Equal(t, kb, c.KB)
}

func TestOp_AnswerCallback(t *testing.T) {
	sender := &stubSender{}
	oc := &OpContext{TenantID: "t1", Sender: sender}

	callStdOp(t, oc, plugin.PermTelegramSend, opTelegramAnswerCallback, answerCallbackArgs{
		CallbackID: "cq-123",
		Text:       "acknowledged",
	})

	require.Len(t, sender.answerCallbackCalls, 1)
	require.Equal(t, "cq-123", sender.answerCallbackCalls[0].CallbackID)
	require.Equal(t, "acknowledged", sender.answerCallbackCalls[0].Text)
}

func TestOp_EditMessage(t *testing.T) {
	sender := &stubSender{}
	oc := &OpContext{TenantID: "t1", Sender: sender}
	kb := &Keyboard{Rows: [][]Button{{{Text: "X", URL: "https://x.com"}}}}

	callStdOp(t, oc, plugin.PermTelegramSend, opTelegramEditMessage, editMessageArgs{
		ChatID:    7,
		MessageID: 100,
		Text:      "updated text",
		Keyboard:  kb,
	})

	require.Len(t, sender.editMessageCalls, 1)
	c := sender.editMessageCalls[0]
	require.Equal(t, int64(7), c.ChatID)
	require.Equal(t, 100, c.MessageID)
	require.Equal(t, "updated text", c.Text)
	require.Equal(t, kb, c.KB)
}

// --- cabinet.open tests ---

// TestOp_CabinetOpen_HTTPS_WebAppButton verifies that an https:// CabinetURL
// produces a single-button keyboard whose button uses WebAppURL (required by
// Telegram for Mini App buttons) and that the default text is used when args.Text is empty.
func TestOp_CabinetOpen_HTTPS_WebAppButton(t *testing.T) {
	sender := &stubSender{}
	oc := &OpContext{
		TenantID:   "t1",
		CabinetURL: "https://cabinet.example.com/shop",
		Sender:     sender,
	}

	// Omit text → defaultCabinetText must be used.
	callStdOp(t, oc, plugin.PermTelegramSend, opCabinetOpen, cabinetOpenArgs{ChatID: 55})

	require.Len(t, sender.sendKeyboardCalls, 1)
	c := sender.sendKeyboardCalls[0]
	require.Equal(t, int64(55), c.ChatID)
	require.Equal(t, defaultCabinetText, c.Text)
	require.Len(t, c.KB.Rows, 1)
	require.Len(t, c.KB.Rows[0], 1)
	btn := c.KB.Rows[0][0]
	require.Equal(t, cabinetButtonLabel, btn.Text)
	require.Equal(t, "https://cabinet.example.com/shop", btn.WebAppURL)
	require.Empty(t, btn.URL)
}

// TestOp_CabinetOpen_HTTP_URLButton verifies that a non-https CabinetURL
// produces a plain URL button (not WebApp) and that custom text passes through.
func TestOp_CabinetOpen_HTTP_URLButton(t *testing.T) {
	sender := &stubSender{}
	oc := &OpContext{
		TenantID:   "t1",
		CabinetURL: "http://cabinet.local/shop",
		Sender:     sender,
	}

	callStdOp(t, oc, plugin.PermTelegramSend, opCabinetOpen, cabinetOpenArgs{
		ChatID: 55,
		Text:   "Custom message",
	})

	require.Len(t, sender.sendKeyboardCalls, 1)
	c := sender.sendKeyboardCalls[0]
	require.Equal(t, "Custom message", c.Text)
	btn := c.KB.Rows[0][0]
	require.Equal(t, cabinetButtonLabel, btn.Text)
	require.Equal(t, "http://cabinet.local/shop", btn.URL)
	require.Empty(t, btn.WebAppURL)
}

// --- user.register test ---

// TestOp_UserRegister verifies that the op calls RegisterViaTelegram with the
// correct telegramID and tenantID, and that the returned JSON contains the
// newly-created user's ID.
func TestOp_UserRegister(t *testing.T) {
	const (
		tgID     = int64(98765)
		tenant   = "shop-xyz"
		dispName = "Bob"
	)

	repo := new(identitytest.MockRepository)
	// First call: user not yet in the store.
	repo.On("GetUserByTelegramID", mock.Anything, tgID).Return(nil, identity.ErrNotFound)

	// Capture the created user to verify the returned user_id.
	var capturedUserID string
	repo.On("CreateUser", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			u := args.Get(1).(*identity.PlatformUser)
			capturedUserID = u.ID
		}).
		Return(nil)

	svc := newTestIdentityService(t, repo)
	oc := &OpContext{TenantID: tenant, Identity: svc}

	r := NewRegistry()
	RegisterStdOps(r)
	raw, err := json.Marshal(registerArgs{TelegramID: tgID, DisplayName: dispName})
	require.NoError(t, err)

	out, err := r.Call(
		context.Background(), oc,
		NewPermSet(plugin.PermUsersWrite),
		opUserRegister, raw,
	)
	require.NoError(t, err)
	require.NotEmpty(t, capturedUserID, "CreateUser must have been called")

	var res registerResult
	require.NoError(t, json.Unmarshal(out, &res))
	require.Equal(t, capturedUserID, res.UserID)

	repo.AssertExpectations(t)
}
