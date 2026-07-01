package cabinetbot

import (
	"context"
	"encoding/json"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
)

// msgRegistrationFailed is sent to the user when registration fails.
const msgRegistrationFailed = "Не удалось завершить регистрацию. Попробуйте позже."

// RequiredPerms is the op-permission set the cabinet-bot handler is granted.
func RequiredPerms() bothost.PermSet {
	return bothost.NewPermSet(plugin.PermTelegramSend, plugin.PermUsersWrite)
}

// Handler is the cabinet-bot BotHandler: register the user as a customer of the
// shop, then reply with the cabinet button. On a registration failure it tells
// the user and returns the error. It acts only through the bothost.Host op
// interface (never the bot token or a DB handle).
func Handler(ctx context.Context, update bothost.Update, host bothost.Host) error {
	name := bothost.DisplayName(update.From)

	regArgs, err := json.Marshal(map[string]any{
		"telegram_id":  update.From.ID,
		"display_name": name,
	})
	if err != nil {
		return err
	}
	if _, err := host.Call(ctx, bothost.OpUserRegister, regArgs); err != nil {
		failArgs, mErr := json.Marshal(map[string]any{
			"chat_id": update.ChatID,
			"text":    msgRegistrationFailed,
		})
		if mErr == nil {
			// Best-effort notice; the registration error is what we return.
			_, _ = host.Call(ctx, bothost.OpTelegramSendText, failArgs)
		}
		return err
	}

	openArgs, err := json.Marshal(map[string]any{"chat_id": update.ChatID})
	if err != nil {
		return err
	}
	_, err = host.Call(ctx, bothost.OpCabinetOpen, openArgs)
	return err
}
