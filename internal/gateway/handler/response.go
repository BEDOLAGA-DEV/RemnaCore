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
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

// writeJSON marshals data as JSON and writes it with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError writes a JSON error response with the given HTTP status code.
// Retained for backward compatibility with middleware and simple cases.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// writeAPIError writes a structured API error as JSON.
func writeAPIError(w http.ResponseWriter, apiErr *apierror.Error) {
	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	w.WriteHeader(apiErr.HTTPStatus)
	_ = json.NewEncoder(w).Encode(apiErr)
}

// writeErrorFromDomain maps a domain error to a structured API error and writes
// it as JSON. Unknown errors are mapped to COMMON.INTERNAL without leaking
// implementation details.
func writeErrorFromDomain(w http.ResponseWriter, err error) {
	writeAPIError(w, mapDomainError(err))
}

// writeValidationError writes a structured validation error, detecting
// MaxBytesError to return COMMON.BODY_TOO_LARGE instead of generic validation.
func writeValidationError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeAPIError(w, apierror.BodyTooLarge)
		return
	}
	writeAPIError(w, apierror.ValidationFailed)
}

// mapDomainError converts a domain sentinel error to a structured API error.
// Unknown errors are mapped to COMMON.INTERNAL without leaking details.
func mapDomainError(err error) *apierror.Error {
	switch {
	// ── Identity ─────────────────────────────────────────────────────────
	case errors.Is(err, identity.ErrEmailTaken):
		return apierror.IdentityEmailTaken
	case errors.Is(err, identity.ErrInvalidCredentials):
		return apierror.IdentityInvalidCreds
	case errors.Is(err, identity.ErrTokenExpired):
		return apierror.IdentityTokenExpired
	case errors.Is(err, identity.ErrSessionExpired):
		return apierror.IdentitySessionExpired
	case errors.Is(err, identity.ErrNotFound):
		return apierror.IdentityNotFound
	case errors.Is(err, identity.ErrEmailNotVerified):
		return apierror.IdentityEmailNotVerified
	case errors.Is(err, identity.ErrPasswordTooShort):
		return apierror.IdentityPasswordTooShort
	case errors.Is(err, identity.ErrPasswordTooWeak):
		return apierror.IdentityPasswordTooWeak
	case errors.Is(err, identity.ErrPasswordResetExpired):
		return apierror.IdentityResetExpired
	case errors.Is(err, identity.ErrPasswordResetNotFound):
		return apierror.IdentityResetNotFound

	// ── Billing ──────────────────────────────────────────────────────────
	case errors.Is(err, billing.ErrPlanNotFound):
		return apierror.BillingPlanNotFound
	case errors.Is(err, billing.ErrSubscriptionNotFound):
		return apierror.BillingSubscriptionNotFound
	case errors.Is(err, billing.ErrInvoiceNotFound):
		return apierror.BillingInvoiceNotFound
	case errors.Is(err, billing.ErrFamilyGroupNotFound):
		return apierror.BillingFamilyGroupNotFound
	case errors.Is(err, billing.ErrInvoiceAlreadyPaid):
		return apierror.BillingInvoiceAlreadyPaid
	case errors.Is(err, billing.ErrInsufficientFunds):
		return apierror.BillingInsufficientFunds
	case errors.Is(err, billing.ErrCurrencyMismatch):
		return apierror.BillingCurrencyMismatch
	case errors.Is(err, billing.ErrAddonNotAvailable):
		return apierror.BillingAddonNotAvailable
	case errors.Is(err, billing.ErrSubscriptionNotActive):
		return apierror.BillingSubscriptionNotActive
	case errors.Is(err, billing.ErrNotTrialStatus):
		return apierror.BillingNotTrialStatus
	case errors.Is(err, billing.ErrCheckoutRateLimited):
		return apierror.BillingCheckoutRateLimited
	case errors.Is(err, billing.ErrAddonAlreadyOnSubscription):
		return apierror.BillingAddonAlreadyOn
	case errors.Is(err, billing.ErrAddonNotOnSubscription):
		return apierror.BillingAddonNotOn
	case errors.Is(err, billing.ErrPlanNotActive):
		return apierror.BillingPlanNotActive
	case errors.Is(err, billing.ErrNoPriceConfigured):
		return apierror.BillingNoPriceConfigured
	case errors.Is(err, billing.ErrFamilyNotEnabled):
		return apierror.BillingFamilyNotEnabled

	// Billing aggregate errors (not aliased at the billing package level).
	case errors.Is(err, aggregate.ErrInvalidTransition):
		return apierror.BillingInvalidTransition
	case errors.Is(err, aggregate.ErrMaxFamilyExceeded):
		return apierror.BillingMaxFamilyExceeded
	case errors.Is(err, aggregate.ErrAlreadyMember):
		return apierror.BillingAlreadyMember
	case errors.Is(err, aggregate.ErrCannotRemoveOwner):
		return apierror.BillingCannotRemoveOwner
	case errors.Is(err, aggregate.ErrMemberNotFound):
		return apierror.BillingMemberNotFound
	case errors.Is(err, aggregate.ErrEmptyPlanName):
		return apierror.BillingEmptyPlanName
	case errors.Is(err, aggregate.ErrBasePriceNotPositive):
		return apierror.BillingBasePriceNotPositive
	case errors.Is(err, aggregate.ErrNoCountriesAllowed):
		return apierror.BillingNoCountriesAllowed
	case errors.Is(err, aggregate.ErrAddonAlreadyExists):
		return apierror.BillingAddonAlreadyExists
	case errors.Is(err, aggregate.ErrAddonNotFound):
		return apierror.BillingAddonNotFound
	case errors.Is(err, aggregate.ErrInvoiceRequiresLineItems):
		return apierror.BillingInvoiceRequiresItems
	case errors.Is(err, aggregate.ErrInvoiceMustBeDraftForPending):
		return apierror.BillingInvoiceMustBeDraft
	case errors.Is(err, aggregate.ErrInvoiceMustBePendingForPaid):
		return apierror.BillingInvoiceMustBePending
	case errors.Is(err, aggregate.ErrInvoiceMustBePendingForFailed):
		return apierror.BillingInvoicePendingForFailed
	case errors.Is(err, aggregate.ErrInvoiceMustBePaidForRefund):
		return apierror.BillingInvoiceMustBePaid
	case errors.Is(err, aggregate.ErrSubscriptionNotActiveForRenewal):
		return apierror.BillingSubscriptionNotActive

	// ── MultiSub ─────────────────────────────────────────────────────────
	case errors.Is(err, multisub.ErrBindingNotFound):
		return apierror.MultiSubBindingNotFound
	case errors.Is(err, multisub.ErrProvisioningFailed):
		return apierror.MultiSubProvisioningFailed
	case errors.Is(err, multisub.ErrDeprovisioningFailed):
		return apierror.MultiSubDeprovisioningFailed
	case errors.Is(err, multisub.ErrSyncFailed):
		return apierror.MultiSubSyncFailed
	case errors.Is(err, multisub.ErrBindingAlreadyActive):
		return apierror.MultiSubBindingAlreadyActive
	case errors.Is(err, multisub.ErrRemnawaveUnavailable):
		return apierror.MultiSubRemnawaveUnavailable
	case errors.Is(err, multisub.ErrMaxBindingsExceeded):
		return apierror.MultiSubMaxBindingsExceeded
	case errors.Is(err, multisub.ErrSagaNotFound):
		return apierror.MultiSubSagaNotFound
	case errors.Is(err, multisub.ErrSagaAlreadyExists):
		return apierror.MultiSubSagaAlreadyExists

	// ── Payment ──────────────────────────────────────────────────────────
	case errors.Is(err, payment.ErrPaymentNotFound):
		return apierror.PaymentNotFound
	case errors.Is(err, payment.ErrWebhookNotFound):
		return apierror.PaymentWebhookNotFound
	case errors.Is(err, payment.ErrWebhookDuplicate):
		return apierror.PaymentWebhookDup
	case errors.Is(err, payment.ErrNoPaymentPlugin):
		return apierror.PaymentNoPlugin
	case errors.Is(err, payment.ErrPaymentFailed):
		return apierror.PaymentFailed
	case errors.Is(err, payment.ErrVerificationFailed):
		return apierror.PaymentVerifyFailed
	case errors.Is(err, payment.ErrRefundFailed):
		return apierror.PaymentRefundFailed
	case errors.Is(err, payment.ErrInvalidProvider):
		return apierror.PaymentInvalidProvider
	case errors.Is(err, payment.ErrMissingInvoiceID):
		return apierror.PaymentMissingInvoice
	case errors.Is(err, payment.ErrMissingAmount):
		return apierror.PaymentMissingAmount
	case errors.Is(err, payment.ErrMissingCurrency):
		return apierror.PaymentMissingCurrency
	case errors.Is(err, payment.ErrMissingExternalID):
		return apierror.PaymentMissingExtID
	case errors.Is(err, payment.ErrInvalidPaymentState):
		return apierror.PaymentInvalidState

	// ── Reseller ─────────────────────────────────────────────────────────
	case errors.Is(err, reseller.ErrTenantNotFound):
		return apierror.ResellerTenantNotFound
	case errors.Is(err, reseller.ErrResellerNotFound):
		return apierror.ResellerAccountNotFound
	case errors.Is(err, reseller.ErrCommissionNotFound):
		return apierror.ResellerCommissionNotFound
	case errors.Is(err, reseller.ErrInvalidCommissionRate):
		return apierror.ResellerInvalidCommission
	case errors.Is(err, reseller.ErrInvalidAPIKey):
		return apierror.ResellerInvalidAPIKey
	case errors.Is(err, reseller.ErrTenantInactive):
		return apierror.ResellerTenantInactive
	case errors.Is(err, reseller.ErrDuplicateDomain):
		return apierror.ResellerDuplicateDomain
	case errors.Is(err, reseller.ErrNotFound):
		return apierror.ResellerNotFound

	// ── Plugin ───────────────────────────────────────────────────────────
	case errors.Is(err, plugin.ErrPluginNotFound):
		return apierror.PluginNotFound
	case errors.Is(err, plugin.ErrPluginAlreadyExists):
		return apierror.PluginAlreadyExists
	case errors.Is(err, plugin.ErrInvalidManifest):
		return apierror.PluginInvalidManifest
	case errors.Is(err, plugin.ErrInvalidPluginSlug):
		return apierror.PluginInvalidSlug
	case errors.Is(err, plugin.ErrPluginNotEnabled):
		return apierror.PluginNotEnabled
	case errors.Is(err, plugin.ErrPluginAlreadyEnabled):
		return apierror.PluginAlreadyEnabled
	case errors.Is(err, plugin.ErrHookTimeout):
		return apierror.PluginHookTimeout
	case errors.Is(err, plugin.ErrHookHalted):
		return apierror.PluginHookHalted
	case errors.Is(err, plugin.ErrPermissionDenied):
		return apierror.PluginPermissionDenied
	case errors.Is(err, plugin.ErrStorageQuotaExceeded):
		return apierror.PluginStorageQuota
	case errors.Is(err, plugin.ErrHTTPRateLimitExceeded):
		return apierror.PluginHTTPRateLimit
	case errors.Is(err, plugin.ErrInternalNetworkAccess):
		return apierror.PluginInternalNetwork
	case errors.Is(err, plugin.ErrWASMCompilationFailed):
		return apierror.PluginWASMCompileFail
	case errors.Is(err, plugin.ErrNoHandlerForHook):
		return apierror.PluginNoHandler
	case errors.Is(err, plugin.ErrSlugMismatch):
		return apierror.PluginSlugMismatch
	case errors.Is(err, plugin.ErrPluginNotRunning):
		return apierror.PluginNotRunning
	case errors.Is(err, plugin.ErrIncompatibleSDK):
		return apierror.PluginIncompatibleSDK
	case errors.Is(err, plugin.ErrPluginDraining):
		return apierror.PluginDraining
	case errors.Is(err, plugin.ErrWASMNotFound):
		return apierror.PluginWASMNotFound
	case errors.Is(err, plugin.ErrMissingConfig):
		return apierror.PluginMissingConfig

	default:
		return apierror.Internal
	}
}

// mapServiceError translates a domain-level error into an HTTP status code and
// user-facing message. Deprecated: prefer writeErrorFromDomain which returns
// structured error codes. Retained for backward compatibility.
func mapServiceError(err error) (status int, message string) {
	apiErr := mapDomainError(err)
	return apiErr.HTTPStatus, apiErr.Message
}
