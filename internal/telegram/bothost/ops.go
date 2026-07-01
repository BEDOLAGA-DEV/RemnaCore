package bothost

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
)

// Op name constants — the string keys used by bot plugins when calling host ops.
const (
	opTelegramSendText       = "telegram.send_text"
	opTelegramSendKeyboard   = "telegram.send_keyboard"
	opTelegramAnswerCallback = "telegram.answer_callback"
	opTelegramEditMessage    = "telegram.edit_message"
	opCabinetOpen            = "cabinet.open"
	opUserRegister           = "user.register"
)

// Cabinet UI copy constants. Defined here rather than imported from the parent
// internal/telegram package to avoid an import cycle (bothost ← telegram).
const (
	cabinetButtonLabel = "Open Cabinet"
	defaultCabinetText = "Access your personal cabinet"
)

// --- Argument structs ---

type sendTextArgs struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

type sendKeyboardArgs struct {
	ChatID   int64    `json:"chat_id"`
	Text     string   `json:"text"`
	Keyboard Keyboard `json:"keyboard"`
}

type answerCallbackArgs struct {
	CallbackID string `json:"callback_id"`
	Text       string `json:"text"`
}

type editMessageArgs struct {
	ChatID    int64     `json:"chat_id"`
	MessageID int       `json:"message_id"`
	Text      string    `json:"text"`
	Keyboard  *Keyboard `json:"keyboard"`
}

type cabinetOpenArgs struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

type registerArgs struct {
	TelegramID  int64  `json:"telegram_id"`
	DisplayName string `json:"display_name"`
}

// --- Result structs ---

type registerResult struct {
	UserID string `json:"user_id"`
}

// RegisterStdOps installs the six standard host operations into r. Call once at
// startup before any bot plugin dispatches an update.
func RegisterStdOps(r *Registry) {
	r.Register(opTelegramSendText, plugin.PermTelegramSend, handleSendText)
	r.Register(opTelegramSendKeyboard, plugin.PermTelegramSend, handleSendKeyboard)
	r.Register(opTelegramAnswerCallback, plugin.PermTelegramSend, handleAnswerCallback)
	r.Register(opTelegramEditMessage, plugin.PermTelegramSend, handleEditMessage)
	r.Register(opCabinetOpen, plugin.PermTelegramSend, handleCabinetOpen)
	r.Register(opUserRegister, plugin.PermUsersWrite, handleUserRegister)
}

func handleSendText(ctx context.Context, oc *OpContext, args json.RawMessage) (json.RawMessage, error) {
	var a sendTextArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("telegram.send_text: decode args: %w", err)
	}
	return nil, oc.Sender.SendText(ctx, a.ChatID, a.Text)
}

func handleSendKeyboard(ctx context.Context, oc *OpContext, args json.RawMessage) (json.RawMessage, error) {
	var a sendKeyboardArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("telegram.send_keyboard: decode args: %w", err)
	}
	return nil, oc.Sender.SendKeyboard(ctx, a.ChatID, a.Text, a.Keyboard)
}

func handleAnswerCallback(ctx context.Context, oc *OpContext, args json.RawMessage) (json.RawMessage, error) {
	var a answerCallbackArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("telegram.answer_callback: decode args: %w", err)
	}
	return nil, oc.Sender.AnswerCallback(ctx, a.CallbackID, a.Text)
}

func handleEditMessage(ctx context.Context, oc *OpContext, args json.RawMessage) (json.RawMessage, error) {
	var a editMessageArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("telegram.edit_message: decode args: %w", err)
	}
	return nil, oc.Sender.EditMessage(ctx, a.ChatID, a.MessageID, a.Text, a.Keyboard)
}

// handleCabinetOpen builds a one-button inline keyboard pointing to the shop
// cabinet. Telegram Mini App (WebApp) buttons require an https:// URL; plain
// URL buttons are used as a fallback for non-HTTPS origins.
func handleCabinetOpen(ctx context.Context, oc *OpContext, args json.RawMessage) (json.RawMessage, error) {
	var a cabinetOpenArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("cabinet.open: decode args: %w", err)
	}

	var btn Button
	if strings.HasPrefix(oc.CabinetURL, "https://") {
		btn = Button{Text: cabinetButtonLabel, WebAppURL: oc.CabinetURL}
	} else {
		btn = Button{Text: cabinetButtonLabel, URL: oc.CabinetURL}
	}
	kb := Keyboard{Rows: [][]Button{{btn}}}

	text := a.Text
	if text == "" {
		text = defaultCabinetText
	}

	return nil, oc.Sender.SendKeyboard(ctx, a.ChatID, text, kb)
}

// handleUserRegister finds or creates a Telegram-native customer for this shop.
// RegisterViaTelegram wraps its own RunInTx+GUC internally — do NOT wrap here.
func handleUserRegister(ctx context.Context, oc *OpContext, args json.RawMessage) (json.RawMessage, error) {
	var a registerArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("user.register: decode args: %w", err)
	}

	user, err := oc.Identity.RegisterViaTelegram(ctx, a.TelegramID, oc.TenantID, a.DisplayName)
	if err != nil {
		return nil, fmt.Errorf("user.register: %w", err)
	}

	return json.Marshal(registerResult{UserID: user.ID})
}
