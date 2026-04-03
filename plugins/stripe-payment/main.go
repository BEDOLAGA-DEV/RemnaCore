// Package main implements the Stripe payment plugin for RemnaCore.
//
// This plugin handles three sync hooks:
//   - payment.create_charge  — builds a Stripe Checkout Session request
//   - payment.verify_webhook — verifies and parses Stripe webhook events
//   - payment.refund         — builds a Stripe Refund API request
//
// When compiled to WASM (GOOS=wasip1 GOARCH=wasm), exported functions are
// called by the platform's hook dispatcher via the Extism PDK. In native mode
// the handlers are regular functions exercised by unit tests.
package main

import (
	"encoding/json"
	"fmt"
	"net/url"
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

// CreateChargePayload is the input for payment.create_charge.
type CreateChargePayload struct {
	InvoiceID string `json:"invoice_id"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	UserID    string `json:"user_id"`
	UserEmail string `json:"user_email"`
	PlanName  string `json:"plan_name"`
	ReturnURL string `json:"return_url"`
	CancelURL string `json:"cancel_url"`
}

// CreateChargeResult is the output for payment.create_charge.
type CreateChargeResult struct {
	Provider    string `json:"provider"`
	ExternalID  string `json:"external_id"`
	CheckoutURL string `json:"checkout_url"`
	Status      string `json:"status"`
	RequestBody string `json:"request_body"`
}

// VerifyWebhookPayload is the input for payment.verify_webhook.
type VerifyWebhookPayload struct {
	Provider string            `json:"provider"`
	Headers  map[string]string `json:"headers"`
	Body     []byte            `json:"body"`
}

// VerifyWebhookResult is the output for payment.verify_webhook.
type VerifyWebhookResult struct {
	Provider   string `json:"provider"`
	ExternalID string `json:"external_id"`
	InvoiceID  string `json:"invoice_id"`
	Amount     int64  `json:"amount"`
	Currency   string `json:"currency"`
	Status     string `json:"status"`
}

// RefundPayload is the input for payment.refund.
type RefundPayload struct {
	PaymentID  string `json:"payment_id"`
	ExternalID string `json:"external_id"`
	Amount     int64  `json:"amount"`
	Reason     string `json:"reason"`
}

// RefundResult is the output for payment.refund.
type RefundResult struct {
	Provider    string `json:"provider"`
	RefundID    string `json:"refund_id"`
	Status      string `json:"status"`
	RequestBody string `json:"request_body"`
}

// --- Constants ---

const (
	stripeAPIBase   = "https://api.stripe.com/v1"
	providerName    = "stripe"
	actionContinue  = "continue"
	actionModify    = "modify"
	actionHalt      = "halt"
	statusPending   = "pending"
	statusSucceeded = "succeeded"
	statusFailed    = "failed"
	quantity        = "1"
	paymentMode     = "payment"
	platformTag     = "remnacore"
)

// Stripe webhook event type constants.
const (
	eventCheckoutCompleted = "checkout.session.completed"
	eventCheckoutExpired   = "checkout.session.expired"
)

// stripeSignatureHeader is the HTTP header Stripe uses for webhook signatures.
const stripeSignatureHeader = "Stripe-Signature"

// Stripe refund reason constants.
const (
	refundDuplicate           = "duplicate"
	refundFraudulent          = "fraudulent"
	refundRequestedByCustomer = "requested_by_customer"
)

// --- Hook handlers ---

func handleCreateCharge(input []byte) ([]byte, error) {
	var ctx HookContext
	if err := json.Unmarshal(input, &ctx); err != nil {
		return haltResult("invalid input: " + err.Error())
	}

	var payload CreateChargePayload
	if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
		return haltResult("invalid payload: " + err.Error())
	}

	params := url.Values{}
	params.Set("mode", paymentMode)
	params.Set("success_url", payload.ReturnURL)
	params.Set("cancel_url", payload.CancelURL)
	params.Set("customer_email", payload.UserEmail)
	params.Set("line_items[0][price_data][currency]", payload.Currency)
	params.Set("line_items[0][price_data][unit_amount]", fmt.Sprintf("%d", payload.Amount))
	params.Set("line_items[0][price_data][product_data][name]", payload.PlanName)
	params.Set("line_items[0][quantity]", quantity)
	params.Set("metadata[invoice_id]", payload.InvoiceID)
	params.Set("metadata[user_id]", payload.UserID)
	params.Set("metadata[platform]", platformTag)

	result := CreateChargeResult{
		Provider:    providerName,
		ExternalID:  statusPending,
		CheckoutURL: stripeAPIBase + "/checkout/sessions",
		Status:      statusPending,
		RequestBody: params.Encode(),
	}

	return modifyResult(result)
}

func handleVerifyWebhook(input []byte) ([]byte, error) {
	var ctx HookContext
	if err := json.Unmarshal(input, &ctx); err != nil {
		return haltResult("invalid input: " + err.Error())
	}

	var payload VerifyWebhookPayload
	if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
		return haltResult("invalid payload: " + err.Error())
	}

	if payload.Provider != providerName {
		return json.Marshal(HookResult{Action: actionContinue})
	}

	sigHeader := payload.Headers[stripeSignatureHeader]
	if sigHeader == "" {
		return haltResult("missing " + stripeSignatureHeader + " header")
	}

	var event stripeWebhookEvent
	if err := json.Unmarshal(payload.Body, &event); err != nil {
		return haltResult("invalid webhook body: " + err.Error())
	}

	status := mapStripeEventStatus(event.Type)

	result := VerifyWebhookResult{
		Provider:   providerName,
		ExternalID: event.Data.Object.ID,
		InvoiceID:  event.Data.Object.Metadata["invoice_id"],
		Amount:     event.Data.Object.Amount,
		Currency:   event.Data.Object.Currency,
		Status:     status,
	}

	return modifyResult(result)
}

func handleRefund(input []byte) ([]byte, error) {
	var ctx HookContext
	if err := json.Unmarshal(input, &ctx); err != nil {
		return haltResult("invalid input: " + err.Error())
	}

	var payload RefundPayload
	if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
		return haltResult("invalid payload: " + err.Error())
	}

	params := url.Values{}
	params.Set("payment_intent", payload.ExternalID)
	if payload.Amount > 0 {
		params.Set("amount", fmt.Sprintf("%d", payload.Amount))
	}
	params.Set("reason", mapRefundReason(payload.Reason))

	result := RefundResult{
		Provider:    providerName,
		RefundID:    statusPending,
		Status:      statusPending,
		RequestBody: params.Encode(),
	}

	return modifyResult(result)
}

// --- Internal types ---

type stripeWebhookEvent struct {
	Type string `json:"type"`
	Data struct {
		Object struct {
			ID       string            `json:"id"`
			Amount   int64             `json:"amount_total"`
			Currency string            `json:"currency"`
			Status   string            `json:"status"`
			Metadata map[string]string `json:"metadata"`
		} `json:"object"`
	} `json:"data"`
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

func mapStripeEventStatus(eventType string) string {
	switch eventType {
	case eventCheckoutCompleted:
		return statusSucceeded
	case eventCheckoutExpired:
		return statusFailed
	default:
		return statusPending
	}
}

func mapRefundReason(reason string) string {
	lower := strings.ToLower(reason)
	switch {
	case strings.Contains(lower, "duplicate"):
		return refundDuplicate
	case strings.Contains(lower, "fraud"):
		return refundFraudulent
	default:
		return refundRequestedByCustomer
	}
}

func main() {}
