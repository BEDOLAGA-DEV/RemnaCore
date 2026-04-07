package payment

import "context"

// Hook names dispatched by the payment facade. All payment logic is delegated
// to plugins registered for these hooks.
const (
	HookCreateCharge  = "payment.create_charge"
	HookVerifyWebhook = "payment.verify_webhook"
	HookRefund        = "payment.refund"
)

// CreateChargeHookFn dispatches the payment.create_charge hook with a typed
// request and returns a typed result. The implementation (wired in
// internal/app/wiring_payment.go) handles JSON marshaling and plugin dispatch.
// Returns (nil, error) on dispatch failure.
type CreateChargeHookFn func(ctx context.Context, req CreateChargeRequest) (*CreateChargeResult, error)

// VerifyWebhookHookFn dispatches the payment.verify_webhook hook with a typed
// request and returns verified payment details. The implementation handles JSON
// marshaling and plugin dispatch. Returns (nil, error) on dispatch failure.
type VerifyWebhookHookFn func(ctx context.Context, req VerifyWebhookHookRequest) (*VerifiedPayment, error)

// RefundHookFn dispatches the payment.refund hook with a typed request. The
// implementation handles JSON marshaling and plugin dispatch. Returns an error
// on dispatch failure.
type RefundHookFn func(ctx context.Context, req RefundHookRequest) error
