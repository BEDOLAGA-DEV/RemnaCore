package telegram

import (
	"context"

	"github.com/go-telegram/bot/models"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
)

// Compile-time assertion: *Bot must satisfy bothost.MessageSender.
var _ bothost.MessageSender = (*Bot)(nil)

// toInlineKeyboard translates the serializable bothost.Keyboard into the
// go-telegram InlineKeyboardMarkup. Button priority: WebAppURL > URL > CallbackData.
func toInlineKeyboard(kb bothost.Keyboard) *models.InlineKeyboardMarkup {
	rows := make([][]models.InlineKeyboardButton, 0, len(kb.Rows))
	for _, row := range kb.Rows {
		cols := make([]models.InlineKeyboardButton, 0, len(row))
		for _, b := range row {
			btn := models.InlineKeyboardButton{Text: b.Text}
			switch {
			case b.WebAppURL != "":
				btn.WebApp = &models.WebAppInfo{URL: b.WebAppURL}
			case b.URL != "":
				btn.URL = b.URL
			default:
				btn.CallbackData = b.CallbackData
			}
			cols = append(cols, btn)
		}
		rows = append(rows, cols)
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// SendText implements bothost.MessageSender by delegating to the fire-and-forget
// sendText helper (which logs on error). Returns nil to satisfy the interface.
func (b *Bot) SendText(ctx context.Context, chatID int64, text string) error {
	b.sendText(ctx, chatID, text)
	return nil
}

// SendKeyboard implements bothost.MessageSender by translating the bothost.Keyboard
// and delegating to sendTextWithKeyboard. Returns nil to satisfy the interface.
func (b *Bot) SendKeyboard(ctx context.Context, chatID int64, text string, kb bothost.Keyboard) error {
	b.sendTextWithKeyboard(ctx, chatID, text, toInlineKeyboard(kb))
	return nil
}

// AnswerCallback implements bothost.MessageSender by delegating to answerCallback.
// Returns nil to satisfy the interface.
func (b *Bot) AnswerCallback(ctx context.Context, callbackID, text string) error {
	b.answerCallback(ctx, callbackID, text)
	return nil
}

// EditMessage implements bothost.MessageSender by translating an optional
// bothost.Keyboard and delegating to editMessageText. A nil kb passes nil
// markup, leaving the existing keyboard unchanged. Returns nil to satisfy the
// interface.
func (b *Bot) EditMessage(ctx context.Context, chatID int64, messageID int, text string, kb *bothost.Keyboard) error {
	var markup *models.InlineKeyboardMarkup
	if kb != nil {
		markup = toInlineKeyboard(*kb)
	}
	b.editMessageText(ctx, chatID, messageID, text, markup)
	return nil
}
