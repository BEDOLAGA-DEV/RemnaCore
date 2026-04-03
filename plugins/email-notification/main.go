// Package main implements the email notification plugin for RemnaCore.
//
// This plugin handles one sync hook and three async hooks:
//   - notification.send (sync) — builds a Resend API request to send an email
//   - user.registered (async) — formats a welcome email
//   - subscription.activated (async) — formats a subscription confirmation email
//   - payment.processed (async) — formats a payment receipt email
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

// UserRegisteredPayload carries data from the user.registered async event.
type UserRegisteredPayload struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

// SubscriptionActivatedPayload carries data from the subscription.activated async event.
type SubscriptionActivatedPayload struct {
	UserID         string `json:"user_id"`
	Email          string `json:"email"`
	PlanName       string `json:"plan_name"`
	SubscriptionID string `json:"subscription_id"`
	ExpiresAt      string `json:"expires_at"`
}

// PaymentProcessedPayload carries data from the payment.processed async event.
type PaymentProcessedPayload struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	InvoiceID string `json:"invoice_id"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	PlanName  string `json:"plan_name"`
}

// --- Constants ---

const (
	providerName   = "resend"
	channelEmail   = "email"
	resendEndpoint = "https://api.resend.com/emails"

	actionContinue = "continue"
	actionModify   = "modify"
	actionHalt     = "halt"

	statusQueued = "queued"

	defaultFromEmail = "noreply@remnacore.com"
	defaultFromName  = "RemnaCore"

	// Cents-to-major-unit divisor for currency formatting.
	centsDivisor = 100
)

// ResendEmailRequest is the Resend API email request body.
type ResendEmailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html,omitempty"`
	Text    string   `json:"text,omitempty"`
}

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

	// Only handle email channel; pass through other channels.
	if payload.Channel != channelEmail {
		return json.Marshal(HookResult{Action: actionContinue})
	}

	emailReq := ResendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", defaultFromName, defaultFromEmail),
		To:      []string{payload.Recipient},
		Subject: payload.Subject,
	}

	if payload.BodyHTML != "" {
		emailReq.HTML = payload.BodyHTML
	} else {
		emailReq.Text = payload.Body
	}

	reqBody, err := json.Marshal(emailReq)
	if err != nil {
		return haltResult("marshal email request: " + err.Error())
	}

	result := SendResult{
		Provider:    providerName,
		Status:      statusQueued,
		RequestBody: string(reqBody),
		Endpoint:    resendEndpoint,
	}

	return modifyResult(result)
}

func handleUserRegistered(input []byte) ([]byte, error) {
	var ctx HookContext
	if err := json.Unmarshal(input, &ctx); err != nil {
		return haltResult("invalid input: " + err.Error())
	}

	var payload UserRegisteredPayload
	if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
		return haltResult("invalid payload: " + err.Error())
	}

	name := payload.Name
	if name == "" {
		name = "there"
	}

	subject := "Welcome to RemnaCore!"
	body := fmt.Sprintf(
		"<h1>Welcome, %s!</h1><p>Your account has been created successfully. "+
			"You can now browse plans and activate your VPN subscription.</p>",
		escapeHTML(name),
	)

	notification := SendPayload{
		Channel:   channelEmail,
		Recipient: payload.Email,
		Subject:   subject,
		BodyHTML:  body,
	}

	return modifyResult(notification)
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

	subject := fmt.Sprintf("Subscription Activated: %s", payload.PlanName)
	body := fmt.Sprintf(
		"<h1>Subscription Confirmed</h1>"+
			"<p>Your <strong>%s</strong> plan is now active.</p>"+
			"<p>Subscription ID: %s</p>"+
			"<p>Valid until: %s</p>"+
			"<p>You can manage your subscription from the dashboard.</p>",
		escapeHTML(payload.PlanName),
		escapeHTML(payload.SubscriptionID),
		escapeHTML(payload.ExpiresAt),
	)

	notification := SendPayload{
		Channel:   channelEmail,
		Recipient: payload.Email,
		Subject:   subject,
		BodyHTML:  body,
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
	subject := fmt.Sprintf("Payment Receipt - %s", amountStr)
	body := fmt.Sprintf(
		"<h1>Payment Received</h1>"+
			"<p>Thank you for your payment.</p>"+
			"<table>"+
			"<tr><td>Invoice</td><td>%s</td></tr>"+
			"<tr><td>Amount</td><td>%s</td></tr>"+
			"<tr><td>Plan</td><td>%s</td></tr>"+
			"</table>",
		escapeHTML(payload.InvoiceID),
		escapeHTML(amountStr),
		escapeHTML(payload.PlanName),
	)

	notification := SendPayload{
		Channel:   channelEmail,
		Recipient: payload.Email,
		Subject:   subject,
		BodyHTML:  body,
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

// escapeHTML performs minimal HTML escaping for template insertion safety.
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
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
