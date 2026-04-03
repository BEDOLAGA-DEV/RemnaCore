package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

// writeJSON marshals data as JSON and writes it with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError writes a JSON error response with the given HTTP status code.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// mapServiceError translates a domain-level error into an HTTP status code and
// user-facing message. This centralises the error-to-status mapping so
// individual handlers do not duplicate if/else chains.
func mapServiceError(err error) (status int, message string) {
	switch {
	// Identity domain
	case errors.Is(err, identity.ErrEmailTaken):
		return http.StatusConflict, "email already taken"
	case errors.Is(err, identity.ErrInvalidCredentials):
		return http.StatusUnauthorized, "invalid credentials"
	case errors.Is(err, identity.ErrTokenExpired):
		return http.StatusGone, "verification token expired"
	case errors.Is(err, identity.ErrSessionExpired):
		return http.StatusUnauthorized, "session expired"
	case errors.Is(err, identity.ErrNotFound):
		return http.StatusNotFound, "not found"
	case errors.Is(err, identity.ErrEmailNotVerified):
		return http.StatusForbidden, "email not verified"
	case errors.Is(err, identity.ErrPasswordResetExpired):
		return http.StatusGone, "password reset token expired"
	case errors.Is(err, identity.ErrPasswordResetNotFound):
		return http.StatusNotFound, "password reset token not found"
	case errors.Is(err, identity.ErrPasswordTooShort):
		return http.StatusBadRequest, "password must be at least 8 characters"
	case errors.Is(err, identity.ErrPasswordTooWeak):
		return http.StatusBadRequest, "password must contain uppercase, lowercase, and digit characters"

	// Billing domain -- not found
	case errors.Is(err, billing.ErrPlanNotFound):
		return http.StatusNotFound, "plan not found"
	case errors.Is(err, billing.ErrSubscriptionNotFound):
		return http.StatusNotFound, "subscription not found"
	case errors.Is(err, billing.ErrInvoiceNotFound):
		return http.StatusNotFound, "invoice not found"
	case errors.Is(err, billing.ErrFamilyGroupNotFound):
		return http.StatusNotFound, "family group not found"

	// Billing domain -- conflict / invalid state
	case errors.Is(err, aggregate.ErrInvalidTransition):
		return http.StatusConflict, "invalid subscription state transition"
	case errors.Is(err, billing.ErrInvoiceAlreadyPaid):
		return http.StatusConflict, "invoice already paid"
	case errors.Is(err, billing.ErrSubscriptionNotActive):
		return http.StatusConflict, "subscription is not active"
	case errors.Is(err, billing.ErrFamilyNotEnabled):
		return http.StatusConflict, "family sharing not enabled for this plan"
	case errors.Is(err, billing.ErrAddonNotAvailable):
		return http.StatusBadRequest, "addon not available for this plan"
	case errors.Is(err, billing.ErrCurrencyMismatch):
		return http.StatusBadRequest, "currency mismatch"
	case errors.Is(err, billing.ErrMaxBindingsExceeded):
		return http.StatusConflict, "maximum bindings exceeded"

	// MultiSub domain
	case errors.Is(err, multisub.ErrBindingNotFound):
		return http.StatusNotFound, "binding not found"
	case errors.Is(err, multisub.ErrProvisioningFailed):
		return http.StatusInternalServerError, "provisioning failed"
	case errors.Is(err, multisub.ErrRemnawaveUnavailable):
		return http.StatusServiceUnavailable, "remnawave panel unavailable"

	// Payment domain
	case errors.Is(err, payment.ErrPaymentNotFound):
		return http.StatusNotFound, "payment not found"
	case errors.Is(err, payment.ErrNoPaymentPlugin):
		return http.StatusServiceUnavailable, "no payment plugin configured"
	case errors.Is(err, payment.ErrPaymentFailed):
		return http.StatusBadGateway, "payment processing failed"
	case errors.Is(err, payment.ErrVerificationFailed):
		return http.StatusBadRequest, "webhook verification failed"
	case errors.Is(err, payment.ErrRefundFailed):
		return http.StatusBadGateway, "refund processing failed"
	case errors.Is(err, payment.ErrInvalidProvider):
		return http.StatusBadRequest, "invalid payment provider"
	case errors.Is(err, payment.ErrMissingInvoiceID):
		return http.StatusBadRequest, "invoice ID is required"
	case errors.Is(err, payment.ErrMissingAmount):
		return http.StatusBadRequest, "payment amount must be positive"
	case errors.Is(err, payment.ErrMissingCurrency):
		return http.StatusBadRequest, "currency is required"

	// Reseller domain
	case errors.Is(err, reseller.ErrTenantNotFound):
		return http.StatusNotFound, "tenant not found"
	case errors.Is(err, reseller.ErrResellerNotFound):
		return http.StatusNotFound, "reseller account not found"
	case errors.Is(err, reseller.ErrCommissionNotFound):
		return http.StatusNotFound, "commission not found"
	case errors.Is(err, reseller.ErrInvalidCommissionRate):
		return http.StatusBadRequest, "commission rate must be between 0 and 100"
	case errors.Is(err, reseller.ErrInvalidAPIKey):
		return http.StatusUnauthorized, "invalid API key"
	case errors.Is(err, reseller.ErrTenantInactive):
		return http.StatusForbidden, "tenant is inactive"
	case errors.Is(err, reseller.ErrDuplicateDomain):
		return http.StatusConflict, "domain already in use"

	// Plugin domain
	case errors.Is(err, plugin.ErrPluginNotFound):
		return http.StatusNotFound, "plugin not found"
	case errors.Is(err, plugin.ErrPluginAlreadyExists):
		return http.StatusConflict, "plugin already exists"
	case errors.Is(err, plugin.ErrInvalidManifest):
		return http.StatusBadRequest, "invalid plugin manifest"
	case errors.Is(err, plugin.ErrInvalidPluginSlug):
		return http.StatusBadRequest, "invalid plugin slug"
	case errors.Is(err, plugin.ErrPluginAlreadyEnabled):
		return http.StatusConflict, "plugin is already enabled"
	case errors.Is(err, plugin.ErrPluginNotEnabled):
		return http.StatusConflict, "plugin is not enabled"
	case errors.Is(err, plugin.ErrPermissionDenied):
		return http.StatusForbidden, "plugin permission denied"
	case errors.Is(err, plugin.ErrWASMCompilationFailed):
		return http.StatusUnprocessableEntity, "WASM compilation failed"
	case errors.Is(err, plugin.ErrSlugMismatch):
		return http.StatusBadRequest, "plugin slug mismatch during hot reload"
	case errors.Is(err, plugin.ErrPluginNotRunning):
		return http.StatusConflict, "plugin is not running"

	default:
		return http.StatusInternalServerError, "internal server error"
	}
}
