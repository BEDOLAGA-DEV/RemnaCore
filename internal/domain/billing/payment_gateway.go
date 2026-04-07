package billing

import "context"

// PaymentGateway is the billing context's Anti-Corruption Layer port for payment
// operations. The billing bounded context uses this interface instead of
// importing the payment domain directly, ensuring that changes in the payment
// context do not propagate into billing.
//
// Translation from billing's ACL types to payment domain types happens at the
// adapter boundary (internal/adapter/billing/payment_gateway_adapter.go, wired
// via Fx in internal/app/wiring_billing.go).
//
// # Scope: charge creation only
//
// This interface intentionally covers only the checkout flow — creating a
// payment charge when a user starts a subscription. It is the single
// synchronous cross-context call that billing initiates against payment.
//
// # Operations NOT covered by this interface
//
// The following payment operations are deliberately excluded because billing
// does not initiate them:
//
//   - Webhook verification (payment.verify_webhook): handled by the gateway
//     layer's PaymentWebhookHandler, which calls payment.PaymentFacade directly.
//     Billing is not involved in signature verification.
//
//   - Payment completion (payment.charge_completed): the webhook handler calls
//     payment.PaymentFacade.CompletePayment to mark the payment record as
//     completed, then calls billing's CheckoutService.CompleteCheckout to mark
//     the invoice as paid and activate the subscription. This is a synchronous
//     call from the gateway layer into billing — not a cross-context domain call.
//
//   - Refund (payment.refund): the payment facade owns refund initiation and
//     publishes a payment.refund_completed domain event. The billing domain has
//     Invoice.Refund() as a state transition, but no billing service currently
//     triggers refund through the payment gateway. When admin-initiated refunds
//     are needed, either:
//     (a) Add a RefundCharge method to this interface if billing must
//     synchronously initiate a refund (e.g., automatic refund on cancellation),
//     or (b) Handle it through an event consumer that listens to
//     payment.refund_completed and transitions the invoice to refunded.
//
// # Communication flow
//
// Checkout (synchronous, billing → payment):
//
//	CheckoutService.StartCheckout
//	  → PaymentGateway.CreateCharge (this interface)
//	    → PaymentGatewayAdapter (internal/adapter/billing/)
//	      → payment.PaymentFacade.CreateCharge
//	        → plugin: payment.create_charge hook
//
// Webhook completion (synchronous, gateway → payment → billing):
//
//	PaymentWebhookHandler.HandlePaymentWebhook (gateway layer)
//	  → payment.PaymentFacade.VerifyWebhook → plugin: payment.verify_webhook
//	  → payment.PaymentFacade.CompletePayment → publishes payment.charge_completed
//	  → billing.CheckoutService.CompleteCheckout
//	    → BillingService.PayInvoice → publishes invoice.paid, subscription.activated
//
// Subscription lifecycle (event-driven, billing → multisub):
//
//	billing events (subscription.activated, subscription.cancelled, etc.)
//	  → NATS JetStream
//	    → BillingEventConsumer
//	      → MultiSubOrchestrator (Remnawave provisioning/deprovisioning)
//
// # When to extend this interface
//
// Add methods here only when the billing domain needs to synchronously call
// payment as part of a billing workflow (e.g., auto-refund on cancellation,
// partial refund on plan downgrade). If the communication is event-driven
// (payment publishes, billing consumes), add an event consumer instead.
type PaymentGateway interface {
	CreateCharge(ctx context.Context, req CreateChargeRequest) (*CreateChargeResult, error)
}

// CreateChargeRequest holds the parameters for creating a payment charge. This
// is billing's own copy of the payment request DTO -- an Anti-Corruption Layer
// type that shields billing from payment domain internals.
type CreateChargeRequest struct {
	InvoiceID string
	Amount    int64
	Currency  string
	UserID    string
	UserEmail string
	PlanName  string
	ReturnURL string
	CancelURL string
}

// CreateChargeResult holds the response from the payment system. This is
// billing's own copy of the payment result DTO.
type CreateChargeResult struct {
	Provider    string
	ExternalID  string
	CheckoutURL string
	Status      string
}
