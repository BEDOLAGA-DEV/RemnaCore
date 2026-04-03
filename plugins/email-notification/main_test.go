package main

import (
	"encoding/json"
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
		PluginID:  "email-notification",
		Payload:   payloadBytes,
	})
	require.NoError(t, err)
	return input
}

func TestHandleNotificationSend_Email(t *testing.T) {
	tests := []struct {
		name        string
		payload     SendPayload
		wantSubject string
		wantHTML    bool
	}{
		{
			name: "html email",
			payload: SendPayload{
				Channel:   channelEmail,
				Recipient: "user@example.com",
				Subject:   "Test Subject",
				BodyHTML:  "<h1>Hello</h1>",
			},
			wantSubject: "Test Subject",
			wantHTML:    true,
		},
		{
			name: "plain text email",
			payload: SendPayload{
				Channel:   channelEmail,
				Recipient: "user@example.com",
				Subject:   "Plain Text",
				Body:      "Hello, world!",
			},
			wantSubject: "Plain Text",
			wantHTML:    false,
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
			assert.Equal(t, resendEndpoint, modified.Endpoint)

			// Verify request body contains correct email request
			var emailReq ResendEmailRequest
			require.NoError(t, json.Unmarshal([]byte(modified.RequestBody), &emailReq))
			assert.Equal(t, []string{tc.payload.Recipient}, emailReq.To)
			assert.Equal(t, tc.wantSubject, emailReq.Subject)

			if tc.wantHTML {
				assert.NotEmpty(t, emailReq.HTML)
				assert.Empty(t, emailReq.Text)
			} else {
				assert.NotEmpty(t, emailReq.Text)
				assert.Empty(t, emailReq.HTML)
			}
		})
	}
}

func TestHandleNotificationSend_NonEmailChannel(t *testing.T) {
	payload := SendPayload{
		Channel:   "telegram",
		Recipient: "123456",
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

func TestHandleUserRegistered(t *testing.T) {
	tests := []struct {
		name     string
		payload  UserRegisteredPayload
		wantName string
	}{
		{
			name: "with name",
			payload: UserRegisteredPayload{
				UserID: "user-1",
				Email:  "alice@example.com",
				Name:   "Alice",
			},
			wantName: "Alice",
		},
		{
			name: "without name",
			payload: UserRegisteredPayload{
				UserID: "user-2",
				Email:  "anon@example.com",
				Name:   "",
			},
			wantName: "there",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := makeHookInput(t, "user.registered", tc.payload)
			output, err := handleUserRegistered(input)
			require.NoError(t, err)

			var result HookResult
			require.NoError(t, json.Unmarshal(output, &result))
			assert.Equal(t, actionModify, result.Action)

			var notification SendPayload
			require.NoError(t, json.Unmarshal(result.Modified, &notification))
			assert.Equal(t, channelEmail, notification.Channel)
			assert.Equal(t, tc.payload.Email, notification.Recipient)
			assert.Equal(t, "Welcome to RemnaCore!", notification.Subject)
			assert.Contains(t, notification.BodyHTML, tc.wantName)
		})
	}
}

func TestHandleSubscriptionActivated(t *testing.T) {
	payload := SubscriptionActivatedPayload{
		UserID:         "user-1",
		Email:          "alice@example.com",
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
	assert.Equal(t, channelEmail, notification.Channel)
	assert.Equal(t, payload.Email, notification.Recipient)
	assert.Contains(t, notification.Subject, "Premium VPN")
	assert.Contains(t, notification.BodyHTML, "sub-abc")
	assert.Contains(t, notification.BodyHTML, "2027-01-15")
}

func TestHandlePaymentProcessed(t *testing.T) {
	payload := PaymentProcessedPayload{
		UserID:    "user-1",
		Email:     "alice@example.com",
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
	assert.Equal(t, channelEmail, notification.Channel)
	assert.Equal(t, payload.Email, notification.Recipient)
	assert.Contains(t, notification.Subject, "29.99 USD")
	assert.Contains(t, notification.BodyHTML, "inv-123")
	assert.Contains(t, notification.BodyHTML, "Premium VPN")
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
		{123456, "usd", "1234.56 USD"},
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
		{"<script>alert('xss')</script>", "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;"},
		{"a & b", "a &amp; b"},
		{`"quoted"`, "&quot;quoted&quot;"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := escapeHTML(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}
