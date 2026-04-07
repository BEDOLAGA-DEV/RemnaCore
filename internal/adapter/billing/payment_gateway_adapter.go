// Package billing implements adapter-layer components that bridge the billing
// domain's ACL ports to other bounded contexts. These adapters live in the
// adapter layer (not the domain) because they import types from multiple
// bounded contexts that must not depend on each other directly.
package billing

import (
	"context"
	"fmt"

	billingdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
)

// PaymentGatewayAdapter translates between billing's ACL types and the payment
// domain's concrete types. It bridges two bounded contexts that must not import
// each other.
//
// This adapter currently translates only CreateCharge, which is the sole
// synchronous operation that billing initiates against payment. Other payment
// operations (webhook verification, refund, payment completion) are either
// handled directly by the gateway layer (PaymentWebhookHandler) or flow through
// domain events -- they do not cross the billing->payment ACL boundary.
//
// See billing.PaymentGateway for the full architectural rationale and
// communication flow documentation.
type PaymentGatewayAdapter struct {
	facade *payment.PaymentFacade
}

// NewPaymentGatewayAdapter creates a PaymentGatewayAdapter that implements
// billing.PaymentGateway by delegating to payment.PaymentFacade.
func NewPaymentGatewayAdapter(facade *payment.PaymentFacade) billingdomain.PaymentGateway {
	return &PaymentGatewayAdapter{facade: facade}
}

// CreateCharge translates billing's CreateChargeRequest to payment's
// CreateChargeRequest, delegates to the PaymentFacade, and translates
// the result back to billing's CreateChargeResult.
func (a *PaymentGatewayAdapter) CreateCharge(ctx context.Context, req billingdomain.CreateChargeRequest) (*billingdomain.CreateChargeResult, error) {
	result, err := a.facade.CreateCharge(ctx, payment.CreateChargeRequest{
		InvoiceID: req.InvoiceID,
		Amount:    req.Amount,
		Currency:  req.Currency,
		UserID:    req.UserID,
		UserEmail: req.UserEmail,
		PlanName:  req.PlanName,
		ReturnURL: req.ReturnURL,
		CancelURL: req.CancelURL,
	})
	if err != nil {
		return nil, fmt.Errorf("payment gateway: %w", err)
	}

	return &billingdomain.CreateChargeResult{
		Provider:    result.Provider,
		ExternalID:  result.ExternalID,
		CheckoutURL: result.CheckoutURL,
		Status:      result.Status,
	}, nil
}

// compile-time interface check
var _ billingdomain.PaymentGateway = (*PaymentGatewayAdapter)(nil)
