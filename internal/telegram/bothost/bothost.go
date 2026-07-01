// Package bothost is the host side of the pluggable per-shop Telegram bot
// system. A bot plugin (built-in Go or, later, WASM) handles an inbound Update
// and produces effects only through the Host op interface; the host owns the
// bot token, tenant scoping, and outbound sends. See the BP1 design spec.
package bothost

import (
	"context"
	"encoding/json"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// User is the Telegram user that produced an update. Serializable so it can
// cross the WASM boundary (BP2) unchanged.
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

// Update is the serializable inbound Telegram update a bot plugin handles.
type Update struct {
	ChatID       int64  `json:"chat_id"`
	From         User   `json:"from"`
	Text         string `json:"text"`
	IsCallback   bool   `json:"is_callback"`
	CallbackID   string `json:"callback_id"`
	CallbackData string `json:"callback_data"`
	MessageID    int    `json:"message_id"`
}

// Button is one inline-keyboard button. Exactly one of URL / WebAppURL /
// CallbackData carries the action; the MessageSender translates it to the
// go-telegram library button type.
type Button struct {
	Text         string `json:"text"`
	URL          string `json:"url,omitempty"`
	WebAppURL    string `json:"web_app_url,omitempty"`
	CallbackData string `json:"callback_data,omitempty"`
}

// Keyboard is a serializable inline keyboard (rows of buttons).
type Keyboard struct {
	Rows [][]Button `json:"rows"`
}

// Host is the effect surface a bot handler uses; every effect (a send or a
// domain operation) goes through Call.
type Host interface {
	Call(ctx context.Context, op string, args json.RawMessage) (json.RawMessage, error)
}

// MessageSender is the outbound Telegram surface, implemented by *telegram.Bot.
// Keeping the registry behind this interface avoids a dependency on the
// concrete go-telegram bot type (and a package cycle with internal/telegram).
type MessageSender interface {
	SendText(ctx context.Context, chatID int64, text string) error
	SendKeyboard(ctx context.Context, chatID int64, text string, kb Keyboard) error
	AnswerCallback(ctx context.Context, callbackID, text string) error
	EditMessage(ctx context.Context, chatID int64, messageID int, text string, kb *Keyboard) error
}

// OpContext carries the per-dispatch dependencies an op handler needs. Later
// sub-projects (BP3) extend it with billing/subscription/balance readers.
type OpContext struct {
	TenantID   string
	CabinetURL string
	Sender     MessageSender
	Identity   *identity.Service
	TxRunner   txmanager.Runner
}

// OpHandler executes one op against the OpContext and returns its JSON result
// (or nil for effect-only ops).
type OpHandler func(ctx context.Context, oc *OpContext, args json.RawMessage) (json.RawMessage, error)
