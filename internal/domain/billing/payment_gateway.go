package billing

import "context"

// PaymentGateway is the billing context's Anti-Corruption Layer port for payment
// operations. The billing bounded context uses this interface instead of
// importing the payment domain directly, ensuring that changes in the payment
// context do not propagate into billing.
//
// Translation from billing's ACL types to payment domain types happens at the
// adapter boundary (wired via Fx in internal/app/app.go).
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
