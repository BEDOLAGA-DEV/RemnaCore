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
		PluginID:  "telegram-notification",
		Payload:   payloadBytes,
	})
	require.NoError(t, err)
	return input
}

func TestHandleNotificationSend_Telegram(t *testing.T) {
	tests := []struct {
		name    string
		payload SendPayload
		wantMsg string
	}{
		{
			name: "html body with subject",
			payload: SendPayload{
				Channel:   channelTG,
				Recipient: "123456789",
				Subject:   "Alert",
				BodyHTML:  "Something happened",
			},
			wantMsg: "Alert",
		},
		{
			name: "plain body no subject",
			payload: SendPayload{
				Channel:   channelTG,
				Recipient: "987654321",
				Subject:   "",
				Body:      "Plain message",
			},
			wantMsg: "Plain message",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := makeHookInput(t, "notification.send", tc.payload)
			output, err := handleNotificationSend(input)
			require.NoError(t, err)

			var result HookResult
			require.NoError(t, json.Unmarshal(output, &result))
			assert.Equal(t, actionModify, result.Action)

			var modified SendResult
			require.NoError(t, json.Unmarshal(result.Modified, &modified))
			assert.Equal(t, providerName, modified.Provider)
			assert.Equal(t, statusQueued, modified.Status)
			assert.Contains(t, modified.Endpoint, botAPIBase)

			var msgReq TelegramSendMessageRequest
			require.NoError(t, json.Unmarshal([]byte(modified.RequestBody), &msgReq))
			assert.Equal(t, tc.payload.Recipient, msgReq.ChatID)
			assert.Equal(t, defaultParseMode, msgReq.ParseMode)
			assert.Contains(t, msgReq.Text, tc.wantMsg)
		})
	}
}

func TestHandleNotificationSend_NonTelegramChannel(t *testing.T) {
	payload := SendPayload{
		Channel:   "email",
		Recipient: "user@example.com",
		Subject:   "Test",
		Body:      "Hello",
	}
	input := makeHookInput(t, "notification.send", payload)

	output, err := handleNotificationSend(input)
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionContinue, result.Action)
}

func TestHandleNotificationSend_InvalidInput(t *testing.T) {
	output, err := handleNotificationSend([]byte("not-json"))
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionHalt, result.Action)
	assert.Contains(t, result.Error, "invalid input")
}

func TestHandleSubscriptionActivated(t *testing.T) {
	payload := SubscriptionActivatedPayload{
		UserID:         "user-1",
		ChatID:         "123456789",
		PlanName:       "Premium VPN",
		SubscriptionID: "sub-abc",
		ExpiresAt:      "2027-01-15",
	}
	input := makeHookInput(t, "subscription.activated", payload)

	output, err := handleSubscriptionActivated(input)
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionModify, result.Action)

	var notification SendPayload
	require.NoError(t, json.Unmarshal(result.Modified, &notification))
	assert.Equal(t, channelTG, notification.Channel)
	assert.Equal(t, payload.ChatID, notification.Recipient)
	assert.Contains(t, notification.BodyHTML, "Premium VPN")
	assert.Contains(t, notification.BodyHTML, "sub-abc")
	assert.Contains(t, notification.BodyHTML, "2027-01-15")
}

func TestHandlePaymentProcessed(t *testing.T) {
	payload := PaymentProcessedPayload{
		UserID:    "user-1",
		ChatID:    "123456789",
		InvoiceID: "inv-123",
		Amount:    2999,
		Currency:  "usd",
		PlanName:  "Premium VPN",
	}
	input := makeHookInput(t, "payment.processed", payload)

	output, err := handlePaymentProcessed(input)
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionModify, result.Action)

	var notification SendPayload
	require.NoError(t, json.Unmarshal(result.Modified, &notification))
	assert.Equal(t, channelTG, notification.Channel)
	assert.Equal(t, payload.ChatID, notification.Recipient)
	assert.Contains(t, notification.BodyHTML, "29.99 USD")
	assert.Contains(t, notification.BodyHTML, "inv-123")
}

func TestHandleBindingProvisioned(t *testing.T) {
	payload := BindingProvisionedPayload{
		UserID:         "user-1",
		ChatID:         "123456789",
		BindingID:      "bind-xyz",
		Purpose:        "base",
		SubscriptionID: "sub-abc",
	}
	input := makeHookInput(t, "binding.provisioned", payload)

	output, err := handleBindingProvisioned(input)
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionModify, result.Action)

	var notification SendPayload
	require.NoError(t, json.Unmarshal(result.Modified, &notification))
	assert.Equal(t, channelTG, notification.Channel)
	assert.Equal(t, payload.ChatID, notification.Recipient)
	assert.Contains(t, notification.BodyHTML, "base")
	assert.Contains(t, notification.BodyHTML, "bind-xyz")
	assert.Contains(t, notification.BodyHTML, "sub-abc")
}

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		amount   int64
		currency string
		want     string
	}{
		{2999, "usd", "29.99 USD"},
		{1000, "eur", "10.00 EUR"},
		{50, "gbp", "0.50 GBP"},
		{0, "rub", "0.00 RUB"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := formatAmount(tc.amount, tc.currency)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"a & b", "a &amp; b"},
		{"<b>bold</b>", "&lt;b&gt;bold&lt;/b&gt;"},
		{"a > b", "a &gt; b"},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%q", tc.input), func(t *testing.T) {
			got := escapeHTML(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}
