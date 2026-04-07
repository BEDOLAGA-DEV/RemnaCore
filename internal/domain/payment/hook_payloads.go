package payment

// VerifyWebhookHookRequest is the typed payload dispatched to the
// payment.verify_webhook hook. Replaces map[string]any to ensure compile-time
// field safety and consistent JSON serialization.
type VerifyWebhookHookRequest struct {
	Provider string            `json:"provider"`
	Headers  map[string]string `json:"headers"`
	Body     []byte            `json:"body"`
}

// RefundHookRequest is the typed payload dispatched to the payment.refund hook.
// Replaces map[string]any to ensure compile-time field safety and consistent
// JSON serialization.
type RefundHookRequest struct {
	Provider   string `json:"provider"`
	ExternalID string `json:"external_id"`
	Amount     int64  `json:"amount"`
	Reason     string `json:"reason,omitempty"`
}
