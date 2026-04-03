package main

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeHookInput(t *testing.T, hookName string, payload any) []byte {
	t.Helper()
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	input, err := json.Marshal(HookContext{
		HookName:  hookName,
		RequestID: "req-test-123",
		PluginID:  "stripe-payment",
		Payload:   payloadBytes,
	})
	require.NoError(t, err)
	return input
}

func TestHandleCreateCharge(t *testing.T) {
	tests := []struct {
		name           string
		payload        CreateChargePayload
		wantProvider   string
		wantStatus     string
		wantBodyParts  []string
	}{
		{
			name: "standard charge",
			payload: CreateChargePayload{
				InvoiceID: "inv-123",
				Amount:    2999,
				Currency:  "usd",
				UserID:    "user-456",
				UserEmail: "test@example.com",
				PlanName:  "Premium VPN",
				ReturnURL: "https://example.com/success",
				CancelURL: "https://example.com/cancel",
			},
			wantProvider: providerName,
			wantStatus:   statusPending,
			// url.Values.Encode URL-encodes brackets in nested keys.
			wantBodyParts: []string{
				"%5Bcurrency%5D=usd",
				"%5Bunit_amount%5D=2999",
				"%5Binvoice_id%5D=inv-123",
				"%5Buser_id%5D=user-456",
				"mode=payment",
			},
		},
		{
			name: "euro charge",
			payload: CreateChargePayload{
				InvoiceID: "inv-789",
				Amount:    1500,
				Currency:  "eur",
				UserID:    "user-abc",
				UserEmail: "eu@example.com",
				PlanName:  "Basic VPN",
				ReturnURL: "https://example.com/ok",
				CancelURL: "https://example.com/no",
			},
			wantProvider:  providerName,
			wantStatus:    statusPending,
			wantBodyParts: []string{"%5Bcurrency%5D=eur", "%5Bunit_amount%5D=1500"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := makeHookInput(t, "payment.create_charge", tc.payload)
			output, err := handleCreateCharge(input)
			require.NoError(t, err)

			var result HookResult
			require.NoError(t, json.Unmarshal(output, &result))
			assert.Equal(t, actionModify, result.Action)

			var modified CreateChargeResult
			require.NoError(t, json.Unmarshal(result.Modified, &modified))
			assert.Equal(t, tc.wantProvider, modified.Provider)
			assert.Equal(t, tc.wantStatus, modified.Status)

			for _, part := range tc.wantBodyParts {
				assert.Contains(t, modified.RequestBody, part)
			}
		})
	}
}

func TestHandleCreateCharge_InvalidInput(t *testing.T) {
	output, err := handleCreateCharge([]byte("not-json"))
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionHalt, result.Action)
	assert.Contains(t, result.Error, "invalid input")
}

func TestHandleCreateCharge_InvalidPayload(t *testing.T) {
	input, _ := json.Marshal(HookContext{
		HookName: "payment.create_charge",
		Payload:  json.RawMessage(`"not-an-object"`),
	})

	output, err := handleCreateCharge(input)
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionHalt, result.Action)
	assert.Contains(t, result.Error, "invalid payload")
}

func TestHandleVerifyWebhook(t *testing.T) {
	tests := []struct {
		name       string
		eventType  string
		wantStatus string
	}{
		{
			name:       "checkout completed",
			eventType:  eventCheckoutCompleted,
			wantStatus: statusSucceeded,
		},
		{
			name:       "checkout expired",
			eventType:  eventCheckoutExpired,
			wantStatus: statusFailed,
		},
		{
			name:       "unknown event type",
			eventType:  "payment_intent.created",
			wantStatus: statusPending,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			webhookBody, err := json.Marshal(stripeWebhookEvent{
				Type: tc.eventType,
				Data: struct {
					Object struct {
						ID       string            `json:"id"`
						Amount   int64             `json:"amount_total"`
						Currency string            `json:"currency"`
						Status   string            `json:"status"`
						Metadata map[string]string `json:"metadata"`
					} `json:"object"`
				}{
					Object: struct {
						ID       string            `json:"id"`
						Amount   int64             `json:"amount_total"`
						Currency string            `json:"currency"`
						Status   string            `json:"status"`
						Metadata map[string]string `json:"metadata"`
					}{
						ID:       "cs_123",
						Amount:   2999,
						Currency: "usd",
						Status:   "complete",
						Metadata: map[string]string{"invoice_id": "inv-123"},
					},
				},
			})
			require.NoError(t, err)

			payload := VerifyWebhookPayload{
				Provider: providerName,
				Headers:  map[string]string{stripeSignatureHeader: "t=123,v1=abc"},
				Body:     webhookBody,
			}

			input := makeHookInput(t, "payment.verify_webhook", payload)
			output, err := handleVerifyWebhook(input)
			require.NoError(t, err)

			var result HookResult
			require.NoError(t, json.Unmarshal(output, &result))
			assert.Equal(t, actionModify, result.Action)

			var modified VerifyWebhookResult
			require.NoError(t, json.Unmarshal(result.Modified, &modified))
			assert.Equal(t, tc.wantStatus, modified.Status)
			assert.Equal(t, "inv-123", modified.InvoiceID)
			assert.Equal(t, "cs_123", modified.ExternalID)
			assert.Equal(t, int64(2999), modified.Amount)
		})
	}
}

func TestHandleVerifyWebhook_WrongProvider(t *testing.T) {
	payload := VerifyWebhookPayload{Provider: "btcpay"}
	input := makeHookInput(t, "payment.verify_webhook", payload)

	output, err := handleVerifyWebhook(input)
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionContinue, result.Action)
}

func TestHandleVerifyWebhook_MissingSignature(t *testing.T) {
	payload := VerifyWebhookPayload{
		Provider: providerName,
		Headers:  map[string]string{},
		Body:     []byte(`{}`),
	}
	input := makeHookInput(t, "payment.verify_webhook", payload)

	output, err := handleVerifyWebhook(input)
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionHalt, result.Action)
	assert.Contains(t, result.Error, "missing "+stripeSignatureHeader)
}

func TestHandleVerifyWebhook_InvalidBody(t *testing.T) {
	payload := VerifyWebhookPayload{
		Provider: providerName,
		Headers:  map[string]string{stripeSignatureHeader: "t=1,v1=a"},
		Body:     []byte("not-json"),
	}
	input := makeHookInput(t, "payment.verify_webhook", payload)

	output, err := handleVerifyWebhook(input)
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionHalt, result.Action)
	assert.Contains(t, result.Error, "invalid webhook body")
}

func TestHandleRefund(t *testing.T) {
	tests := []struct {
		name       string
		payload    RefundPayload
		wantReason string
		wantAmount bool
	}{
		{
			name: "full refund customer requested",
			payload: RefundPayload{
				PaymentID:  "pay-123",
				ExternalID: "pi_abc",
				Amount:     0,
				Reason:     "customer requested",
			},
			wantReason: refundRequestedByCustomer,
			wantAmount: false,
		},
		{
			name: "partial refund duplicate",
			payload: RefundPayload{
				PaymentID:  "pay-456",
				ExternalID: "pi_def",
				Amount:     1500,
				Reason:     "duplicate charge",
			},
			wantReason: refundDuplicate,
			wantAmount: true,
		},
		{
			name: "fraud refund",
			payload: RefundPayload{
				PaymentID:  "pay-789",
				ExternalID: "pi_ghi",
				Amount:     3000,
				Reason:     "suspected fraud",
			},
			wantReason: refundFraudulent,
			wantAmount: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := makeHookInput(t, "payment.refund", tc.payload)
			output, err := handleRefund(input)
			require.NoError(t, err)

			var result HookResult
			require.NoError(t, json.Unmarshal(output, &result))
			assert.Equal(t, actionModify, result.Action)

			var modified RefundResult
			require.NoError(t, json.Unmarshal(result.Modified, &modified))
			assert.Equal(t, providerName, modified.Provider)
			assert.Contains(t, modified.RequestBody, "payment_intent="+tc.payload.ExternalID)
			assert.Contains(t, modified.RequestBody, "reason="+tc.wantReason)

			if tc.wantAmount {
				assert.Contains(t, modified.RequestBody, fmt.Sprintf("amount=%d", tc.payload.Amount))
			}
		})
	}
}

func TestMapRefundReason(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"duplicate charge", refundDuplicate},
		{"DUPLICATE", refundDuplicate},
		{"suspected fraud", refundFraudulent},
		{"Fraud detected", refundFraudulent},
		{"customer wants refund", refundRequestedByCustomer},
		{"no reason", refundRequestedByCustomer},
		{"", refundRequestedByCustomer},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := mapRefundReason(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestMapStripeEventStatus(t *testing.T) {
	tests := []struct {
		eventType string
		want      string
	}{
		{eventCheckoutCompleted, statusSucceeded},
		{eventCheckoutExpired, statusFailed},
		{"unknown.event", statusPending},
		{"", statusPending},
	}

	for _, tc := range tests {
		t.Run(tc.eventType, func(t *testing.T) {
			got := mapStripeEventStatus(tc.eventType)
			assert.Equal(t, tc.want, got)
		})
	}
}
