// Package main implements the Telegram notification plugin for RemnaCore.
//
// This plugin handles one sync hook and three async hooks:
//   - notification.send (sync) — sends a message via Telegram Bot API if channel == "telegram"
//   - subscription.activated (async) — formats a subscription confirmation message
//   - payment.processed (async) — formats a payment receipt message
//   - binding.provisioned (async) — formats a VPN binding provisioned message
//
// When compiled to WASM (GOOS=wasip1 GOARCH=wasm), exported functions are
// called by the platform's hook dispatcher via the Extism PDK.
package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// --- SDK types (mirrored from pkg/sdk for standalone WASM compilation) ---

// HookContext is the JSON envelope passed as input to every plugin hook.
type HookContext struct {
	HookName  string          `json:"hook_name"`
	RequestID string          `json:"request_id"`
	Timestamp int64           `json:"timestamp"`
	PluginID  string          `json:"plugin_id"`
	Payload   json.RawMessage `json:"payload"`
}

// HookResult is the JSON envelope returned by a plugin hook.
type HookResult struct {
	Action   string          `json:"action"`
	Modified json.RawMessage `json:"modified,omitempty"`
	Error    string          `json:"error,omitempty"`
}

// --- Payload types ---

// SendPayload is the input for notification.send.
type SendPayload struct {
	Channel   string `json:"channel"`
	Recipient string `json:"recipient"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	BodyHTML  string `json:"body_html"`
}

// SendResult is the output for notification.send.
type SendResult struct {
	Provider    string `json:"provider"`
	Status      string `json:"status"`
	RequestBody string `json:"request_body"`
	Endpoint    string `json:"endpoint"`
}

// TelegramSendMessageRequest is the Telegram Bot API sendMessage request body.
type TelegramSendMessageRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// SubscriptionActivatedPayload carries data from the subscription.activated async event.
type SubscriptionActivatedPayload struct {
	UserID         string `json:"user_id"`
	ChatID         string `json:"chat_id"`
	PlanName       string `json:"plan_name"`
	SubscriptionID string `json:"subscription_id"`
	ExpiresAt      string `json:"expires_at"`
}

// PaymentProcessedPayload carries data from the payment.processed async event.
type PaymentProcessedPayload struct {
	UserID    string `json:"user_id"`
	ChatID    string `json:"chat_id"`
	InvoiceID string `json:"invoice_id"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	PlanName  string `json:"plan_name"`
}

// BindingProvisionedPayload carries data from the binding.provisioned async event.
type BindingProvisionedPayload struct {
	UserID         string `json:"user_id"`
	ChatID         string `json:"chat_id"`
	BindingID      string `json:"binding_id"`
	Purpose        string `json:"purpose"`
	SubscriptionID string `json:"subscription_id"`
}

// --- Constants ---

const (
	providerName   = "telegram"
	channelTG      = "telegram"
	botAPIBase     = "https://api.telegram.org"
	defaultParseMode = "HTML"

	actionContinue = "continue"
	actionModify   = "modify"
	actionHalt     = "halt"

	statusQueued = "queued"

	// Cents-to-major-unit divisor for currency formatting.
	centsDivisor = 100
)

// --- Hook handlers ---

func handleNotificationSend(input []byte) ([]byte, error) {
	var ctx HookContext
	if err := json.Unmarshal(input, &ctx); err != nil {
		return haltResult("invalid input: " + err.Error())
	}

	var payload SendPayload
	if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
		return haltResult("invalid payload: " + err.Error())
	}

	// Only handle telegram channel; pass through other channels.
	if payload.Channel != channelTG {
		return json.Marshal(HookResult{Action: actionContinue})
	}

	text := payload.Body
	if payload.BodyHTML != "" {
		text = payload.BodyHTML
	}

	// Prepend subject as bold header if present.
	if payload.Subject != "" {
		text = fmt.Sprintf("<b>%s</b>\n\n%s", escapeHTML(payload.Subject), text)
	}

	msgReq := TelegramSendMessageRequest{
		ChatID:    payload.Recipient,
		Text:      text,
		ParseMode: defaultParseMode,
	}

	reqBody, err := json.Marshal(msgReq)
	if err != nil {
		return haltResult("marshal telegram request: " + err.Error())
	}

	// Bot token is placeholder; in WASM the plugin retrieves it via pdk.ConfigGet("bot_token").
	endpoint := fmt.Sprintf("%s/bot{TOKEN}/sendMessage", botAPIBase)

	result := SendResult{
		Provider:    providerName,
		Status:      statusQueued,
		RequestBody: string(reqBody),
		Endpoint:    endpoint,
	}

	return modifyResult(result)
}

func handleSubscriptionActivated(input []byte) ([]byte, error) {
	var ctx HookContext
	if err := json.Unmarshal(input, &ctx); err != nil {
		return haltResult("invalid input: " + err.Error())
	}

	var payload SubscriptionActivatedPayload
	if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
		return haltResult("invalid payload: " + err.Error())
	}

	text := fmt.Sprintf(
		"<b>Subscription Activated</b>\n\n"+
			"Plan: <b>%s</b>\n"+
			"Subscription: <code>%s</code>\n"+
			"Valid until: %s",
		escapeHTML(payload.PlanName),
		escapeHTML(payload.SubscriptionID),
		escapeHTML(payload.ExpiresAt),
	)

	notification := SendPayload{
		Channel:   channelTG,
		Recipient: payload.ChatID,
		Subject:   "",
		BodyHTML:  text,
	}

	return modifyResult(notification)
}

func handlePaymentProcessed(input []byte) ([]byte, error) {
	var ctx HookContext
	if err := json.Unmarshal(input, &ctx); err != nil {
		return haltResult("invalid input: " + err.Error())
	}

	var payload PaymentProcessedPayload
	if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
		return haltResult("invalid payload: " + err.Error())
	}

	amountStr := formatAmount(payload.Amount, payload.Currency)
	text := fmt.Sprintf(
		"<b>Payment Received</b>\n\n"+
			"Invoice: <code>%s</code>\n"+
			"Amount: <b>%s</b>\n"+
			"Plan: %s",
		escapeHTML(payload.InvoiceID),
		escapeHTML(amountStr),
		escapeHTML(payload.PlanName),
	)

	notification := SendPayload{
		Channel:   channelTG,
		Recipient: payload.ChatID,
		Subject:   "",
		BodyHTML:  text,
	}

	return modifyResult(notification)
}

func handleBindingProvisioned(input []byte) ([]byte, error) {
	var ctx HookContext
	if err := json.Unmarshal(input, &ctx); err != nil {
		return haltResult("invalid input: " + err.Error())
	}

	var payload BindingProvisionedPayload
	if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
		return haltResult("invalid payload: " + err.Error())
	}

	text := fmt.Sprintf(
		"<b>VPN Connection Ready</b>\n\n"+
			"Your <b>%s</b> connection has been provisioned.\n"+
			"Binding: <code>%s</code>\n"+
			"Subscription: <code>%s</code>",
		escapeHTML(payload.Purpose),
		escapeHTML(payload.BindingID),
		escapeHTML(payload.SubscriptionID),
	)

	notification := SendPayload{
		Channel:   channelTG,
		Recipient: payload.ChatID,
		Subject:   "",
		BodyHTML:  text,
	}

	return modifyResult(notification)
}

// --- Helpers ---

func modifyResult(data any) ([]byte, error) {
	modified, err := json.Marshal(data)
	if err != nil {
		return haltResult("marshal error: " + err.Error())
	}
	return json.Marshal(HookResult{
		Action:   actionModify,
		Modified: modified,
	})
}

func haltResult(errMsg string) ([]byte, error) {
	return json.Marshal(HookResult{
		Action: actionHalt,
		Error:  errMsg,
	})
}

// escapeHTML performs minimal HTML escaping for Telegram HTML parse mode.
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// formatAmount converts a minor-unit amount (e.g. cents) into a display string
// with currency code (e.g. "29.99 USD").
func formatAmount(amount int64, currency string) string {
	major := amount / centsDivisor
	minor := amount % centsDivisor
	return fmt.Sprintf("%d.%02d %s", major, minor, strings.ToUpper(currency))
}

func main() {}
