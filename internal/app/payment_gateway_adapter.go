package app

import (
	"context"
	"fmt"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
)

// paymentGatewayAdapter translates between billing's ACL types and the payment
// domain's concrete types. This adapter lives in the wiring layer because it
// bridges two bounded contexts that must not import each other.
type paymentGatewayAdapter struct {
	facade *payment.PaymentFacade
}

// newPaymentGatewayAdapter creates a paymentGatewayAdapter that implements
// billing.PaymentGateway by delegating to payment.PaymentFacade.
func newPaymentGatewayAdapter(facade *payment.PaymentFacade) billing.PaymentGateway {
	return &paymentGatewayAdapter{facade: facade}
}

// CreateCharge translates billing's CreateChargeRequest to payment's
// CreateChargeRequest, delegates to the PaymentFacade, and translates
// the result back to billing's CreateChargeResult.
func (a *paymentGatewayAdapter) CreateCharge(ctx context.Context, req billing.CreateChargeRequest) (*billing.CreateChargeResult, error) {
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

	return &billing.CreateChargeResult{
		Provider:    result.Provider,
		ExternalID:  result.ExternalID,
		CheckoutURL: result.CheckoutURL,
		Status:      result.Status,
	}, nil
}
