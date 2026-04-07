package payment

import (
	"errors"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// MapToAPIError maps payment domain errors to API error codes.
// Returns nil if the error is not a payment domain error.
func MapToAPIError(err error) *apierror.Error {
	switch {
	case errors.Is(err, ErrPaymentAlreadyExists):
		return apierror.PaymentAlreadyExists
	case errors.Is(err, ErrPaymentNotFound):
		return apierror.PaymentNotFound
	case errors.Is(err, ErrWebhookNotFound):
		return apierror.PaymentWebhookNotFound
	case errors.Is(err, ErrWebhookDuplicate):
		return apierror.PaymentWebhookDup
	case errors.Is(err, ErrNoPaymentPlugin):
		return apierror.PaymentNoPlugin
	case errors.Is(err, ErrPaymentFailed):
		return apierror.PaymentFailed
	case errors.Is(err, ErrVerificationFailed):
		return apierror.PaymentVerifyFailed
	case errors.Is(err, ErrRefundFailed):
		return apierror.PaymentRefundFailed
	case errors.Is(err, ErrInvalidProvider):
		return apierror.PaymentInvalidProvider
	case errors.Is(err, ErrMissingInvoiceID):
		return apierror.PaymentMissingInvoice
	case errors.Is(err, ErrMissingAmount):
		return apierror.PaymentMissingAmount
	case errors.Is(err, ErrMissingCurrency):
		return apierror.PaymentMissingCurrency
	case errors.Is(err, ErrMissingExternalID):
		return apierror.PaymentMissingExtID
	case errors.Is(err, ErrInvalidPaymentState):
		return apierror.PaymentInvalidState
	default:
		return nil
	}
}
