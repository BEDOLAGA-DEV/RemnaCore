package handler

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
)

func TestMapDomainError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
		wantHTTP int
	}{
		// ── Identity ────────────────────────────────────────────────────
		{"identity email taken", identity.ErrEmailTaken, "IDENTITY.EMAIL_TAKEN", http.StatusConflict},
		{"identity invalid creds", identity.ErrInvalidCredentials, "IDENTITY.INVALID_CREDENTIALS", http.StatusUnauthorized},
		{"identity token expired", identity.ErrTokenExpired, "IDENTITY.TOKEN_EXPIRED", http.StatusGone},
		{"identity session expired", identity.ErrSessionExpired, "IDENTITY.SESSION_EXPIRED", http.StatusUnauthorized},
		{"identity not found", identity.ErrNotFound, "IDENTITY.NOT_FOUND", http.StatusNotFound},
		{"identity email unverified", identity.ErrEmailNotVerified, "IDENTITY.EMAIL_NOT_VERIFIED", http.StatusForbidden},
		{"identity password short", identity.ErrPasswordTooShort, "IDENTITY.PASSWORD_TOO_SHORT", http.StatusUnprocessableEntity},
		{"identity password weak", identity.ErrPasswordTooWeak, "IDENTITY.PASSWORD_TOO_WEAK", http.StatusUnprocessableEntity},
		{"identity reset expired", identity.ErrPasswordResetExpired, "IDENTITY.RESET_EXPIRED", http.StatusGone},
		{"identity reset not found", identity.ErrPasswordResetNotFound, "IDENTITY.RESET_NOT_FOUND", http.StatusNotFound},

		// ── Billing ─────────────────────────────────────────────────────
		{"billing plan not found", billing.ErrPlanNotFound, "BILLING.PLAN_NOT_FOUND", http.StatusNotFound},
		{"billing sub not found", billing.ErrSubscriptionNotFound, "BILLING.SUBSCRIPTION_NOT_FOUND", http.StatusNotFound},
		{"billing inv not found", billing.ErrInvoiceNotFound, "BILLING.INVOICE_NOT_FOUND", http.StatusNotFound},
		{"billing family not found", billing.ErrFamilyGroupNotFound, "BILLING.FAMILY_GROUP_NOT_FOUND", http.StatusNotFound},
		{"billing inv paid", billing.ErrInvoiceAlreadyPaid, "BILLING.INVOICE_ALREADY_PAID", http.StatusConflict},
		{"billing insufficient", billing.ErrInsufficientFunds, "BILLING.INSUFFICIENT_FUNDS", http.StatusBadRequest},
		{"billing currency", billing.ErrCurrencyMismatch, "BILLING.CURRENCY_MISMATCH", http.StatusBadRequest},
		{"billing addon unavail", billing.ErrAddonNotAvailable, "BILLING.ADDON_NOT_AVAILABLE", http.StatusBadRequest},
		{"billing sub not active", billing.ErrSubscriptionNotActive, "BILLING.SUBSCRIPTION_NOT_ACTIVE", http.StatusConflict},
		{"billing not trial", billing.ErrNotTrialStatus, "BILLING.NOT_TRIAL_STATUS", http.StatusConflict},
		{"billing rate limit", billing.ErrCheckoutRateLimited, "BILLING.CHECKOUT_RATE_LIMITED", http.StatusTooManyRequests},
		{"billing addon already on", billing.ErrAddonAlreadyOnSubscription, "BILLING.ADDON_ALREADY_ON_SUBSCRIPTION", http.StatusConflict},
		{"billing addon not on", billing.ErrAddonNotOnSubscription, "BILLING.ADDON_NOT_ON_SUBSCRIPTION", http.StatusNotFound},
		{"billing plan not active", billing.ErrPlanNotActive, "BILLING.PLAN_NOT_ACTIVE", http.StatusBadRequest},
		{"billing no price", billing.ErrNoPriceConfigured, "BILLING.NO_PRICE_CONFIGURED", http.StatusBadRequest},
		{"billing family disabled", billing.ErrFamilyNotEnabled, "BILLING.FAMILY_NOT_ENABLED", http.StatusConflict},

		// Billing aggregate errors
		{"aggregate invalid transition", aggregate.ErrInvalidTransition, "BILLING.INVALID_TRANSITION", http.StatusConflict},
		{"aggregate max family", aggregate.ErrMaxFamilyExceeded, "BILLING.MAX_FAMILY_EXCEEDED", http.StatusConflict},
		{"aggregate already member", aggregate.ErrAlreadyMember, "BILLING.ALREADY_MEMBER", http.StatusConflict},
		{"aggregate cannot remove owner", aggregate.ErrCannotRemoveOwner, "BILLING.CANNOT_REMOVE_OWNER", http.StatusConflict},
		{"aggregate member not found", aggregate.ErrMemberNotFound, "BILLING.MEMBER_NOT_FOUND", http.StatusNotFound},
		{"aggregate empty plan name", aggregate.ErrEmptyPlanName, "BILLING.EMPTY_PLAN_NAME", http.StatusBadRequest},
		{"aggregate price not positive", aggregate.ErrBasePriceNotPositive, "BILLING.BASE_PRICE_NOT_POSITIVE", http.StatusBadRequest},
		{"aggregate no countries", aggregate.ErrNoCountriesAllowed, "BILLING.NO_COUNTRIES_ALLOWED", http.StatusBadRequest},
		{"aggregate addon exists", aggregate.ErrAddonAlreadyExists, "BILLING.ADDON_ALREADY_EXISTS", http.StatusConflict},
		{"aggregate addon not found", aggregate.ErrAddonNotFound, "BILLING.ADDON_NOT_FOUND", http.StatusNotFound},
		{"aggregate inv requires items", aggregate.ErrInvoiceRequiresLineItems, "BILLING.INVOICE_REQUIRES_LINE_ITEMS", http.StatusBadRequest},
		{"aggregate inv must be draft", aggregate.ErrInvoiceMustBeDraftForPending, "BILLING.INVOICE_MUST_BE_DRAFT", http.StatusConflict},
		{"aggregate inv must be pending", aggregate.ErrInvoiceMustBePendingForPaid, "BILLING.INVOICE_MUST_BE_PENDING", http.StatusConflict},
		{"aggregate inv pending failed", aggregate.ErrInvoiceMustBePendingForFailed, "BILLING.INVOICE_PENDING_FOR_FAILED", http.StatusConflict},
		{"aggregate inv must be paid", aggregate.ErrInvoiceMustBePaidForRefund, "BILLING.INVOICE_MUST_BE_PAID", http.StatusConflict},
		{"aggregate sub not active renewal", aggregate.ErrSubscriptionNotActiveForRenewal, "BILLING.SUBSCRIPTION_NOT_ACTIVE", http.StatusConflict},

		// ── MultiSub ────────────────────────────────────────────────────
		{"multisub binding not found", multisub.ErrBindingNotFound, "MULTISUB.BINDING_NOT_FOUND", http.StatusNotFound},
		{"multisub provision failed", multisub.ErrProvisioningFailed, "MULTISUB.PROVISIONING_FAILED", http.StatusInternalServerError},
		{"multisub deprovision failed", multisub.ErrDeprovisioningFailed, "MULTISUB.DEPROVISIONING_FAILED", http.StatusInternalServerError},
		{"multisub sync failed", multisub.ErrSyncFailed, "MULTISUB.SYNC_FAILED", http.StatusInternalServerError},
		{"multisub already active", multisub.ErrBindingAlreadyActive, "MULTISUB.BINDING_ALREADY_ACTIVE", http.StatusConflict},
		{"multisub unavailable", multisub.ErrRemnawaveUnavailable, "MULTISUB.REMNAWAVE_UNAVAILABLE", http.StatusServiceUnavailable},
		{"multisub max bindings", multisub.ErrMaxBindingsExceeded, "MULTISUB.MAX_BINDINGS_EXCEEDED", http.StatusConflict},
		{"multisub saga not found", multisub.ErrSagaNotFound, "MULTISUB.SAGA_NOT_FOUND", http.StatusNotFound},
		{"multisub saga exists", multisub.ErrSagaAlreadyExists, "MULTISUB.SAGA_ALREADY_EXISTS", http.StatusConflict},

		// ── Payment ─────────────────────────────────────────────────────
		{"payment not found", payment.ErrPaymentNotFound, "PAYMENT.NOT_FOUND", http.StatusNotFound},
		{"payment webhook not found", payment.ErrWebhookNotFound, "PAYMENT.WEBHOOK_NOT_FOUND", http.StatusNotFound},
		{"payment webhook dup", payment.ErrWebhookDuplicate, "PAYMENT.WEBHOOK_DUPLICATE", http.StatusConflict},
		{"payment no plugin", payment.ErrNoPaymentPlugin, "PAYMENT.NO_PLUGIN", http.StatusServiceUnavailable},
		{"payment failed", payment.ErrPaymentFailed, "PAYMENT.FAILED", http.StatusBadGateway},
		{"payment verify failed", payment.ErrVerificationFailed, "PAYMENT.VERIFICATION_FAILED", http.StatusBadRequest},
		{"payment refund failed", payment.ErrRefundFailed, "PAYMENT.REFUND_FAILED", http.StatusBadGateway},
		{"payment invalid provider", payment.ErrInvalidProvider, "PAYMENT.INVALID_PROVIDER", http.StatusBadRequest},
		{"payment missing invoice", payment.ErrMissingInvoiceID, "PAYMENT.MISSING_INVOICE_ID", http.StatusBadRequest},
		{"payment missing amount", payment.ErrMissingAmount, "PAYMENT.MISSING_AMOUNT", http.StatusBadRequest},
		{"payment missing currency", payment.ErrMissingCurrency, "PAYMENT.MISSING_CURRENCY", http.StatusBadRequest},
		{"payment missing ext id", payment.ErrMissingExternalID, "PAYMENT.MISSING_EXTERNAL_ID", http.StatusBadRequest},
		{"payment invalid state", payment.ErrInvalidPaymentState, "PAYMENT.INVALID_STATE", http.StatusConflict},

		// ── Reseller ────────────────────────────────────────────────────
		{"reseller tenant not found", reseller.ErrTenantNotFound, "RESELLER.TENANT_NOT_FOUND", http.StatusNotFound},
		{"reseller account not found", reseller.ErrResellerNotFound, "RESELLER.ACCOUNT_NOT_FOUND", http.StatusNotFound},
		{"reseller commission not found", reseller.ErrCommissionNotFound, "RESELLER.COMMISSION_NOT_FOUND", http.StatusNotFound},
		{"reseller invalid commission", reseller.ErrInvalidCommissionRate, "RESELLER.INVALID_COMMISSION_RATE", http.StatusBadRequest},
		{"reseller invalid key", reseller.ErrInvalidAPIKey, "RESELLER.INVALID_API_KEY", http.StatusUnauthorized},
		{"reseller tenant inactive", reseller.ErrTenantInactive, "RESELLER.TENANT_INACTIVE", http.StatusForbidden},
		{"reseller dup domain", reseller.ErrDuplicateDomain, "RESELLER.DUPLICATE_DOMAIN", http.StatusConflict},
		{"reseller not found", reseller.ErrNotFound, "RESELLER.NOT_FOUND", http.StatusNotFound},

		// ── Plugin ──────────────────────────────────────────────────────
		{"plugin not found", plugin.ErrPluginNotFound, "PLUGIN.NOT_FOUND", http.StatusNotFound},
		{"plugin already exists", plugin.ErrPluginAlreadyExists, "PLUGIN.ALREADY_EXISTS", http.StatusConflict},
		{"plugin invalid manifest", plugin.ErrInvalidManifest, "PLUGIN.INVALID_MANIFEST", http.StatusBadRequest},
		{"plugin invalid slug", plugin.ErrInvalidPluginSlug, "PLUGIN.INVALID_SLUG", http.StatusBadRequest},
		{"plugin not enabled", plugin.ErrPluginNotEnabled, "PLUGIN.NOT_ENABLED", http.StatusConflict},
		{"plugin already enabled", plugin.ErrPluginAlreadyEnabled, "PLUGIN.ALREADY_ENABLED", http.StatusConflict},
		{"plugin hook timeout", plugin.ErrHookTimeout, "PLUGIN.HOOK_TIMEOUT", http.StatusGatewayTimeout},
		{"plugin hook halted", plugin.ErrHookHalted, "PLUGIN.HOOK_HALTED", http.StatusBadGateway},
		{"plugin permission denied", plugin.ErrPermissionDenied, "PLUGIN.PERMISSION_DENIED", http.StatusForbidden},
		{"plugin storage quota", plugin.ErrStorageQuotaExceeded, "PLUGIN.STORAGE_QUOTA_EXCEEDED", http.StatusInsufficientStorage},
		{"plugin http rate", plugin.ErrHTTPRateLimitExceeded, "PLUGIN.HTTP_RATE_LIMIT_EXCEEDED", http.StatusTooManyRequests},
		{"plugin internal net", plugin.ErrInternalNetworkAccess, "PLUGIN.INTERNAL_NETWORK_ACCESS", http.StatusForbidden},
		{"plugin wasm fail", plugin.ErrWASMCompilationFailed, "PLUGIN.WASM_COMPILATION_FAILED", http.StatusUnprocessableEntity},
		{"plugin no handler", plugin.ErrNoHandlerForHook, "PLUGIN.NO_HANDLER_FOR_HOOK", http.StatusNotFound},
		{"plugin slug mismatch", plugin.ErrSlugMismatch, "PLUGIN.SLUG_MISMATCH", http.StatusBadRequest},
		{"plugin not running", plugin.ErrPluginNotRunning, "PLUGIN.NOT_RUNNING", http.StatusConflict},
		{"plugin incompatible sdk", plugin.ErrIncompatibleSDK, "PLUGIN.INCOMPATIBLE_SDK", http.StatusConflict},
		{"plugin draining", plugin.ErrPluginDraining, "PLUGIN.DRAINING", http.StatusServiceUnavailable},
		{"plugin wasm not found", plugin.ErrWASMNotFound, "PLUGIN.WASM_NOT_FOUND", http.StatusNotFound},
		{"plugin missing config", plugin.ErrMissingConfig, "PLUGIN.MISSING_CONFIG", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := mapDomainError(tt.err)
			assert.Equal(t, tt.wantCode, apiErr.Code)
			assert.Equal(t, tt.wantHTTP, apiErr.HTTPStatus)
			assert.NotEmpty(t, apiErr.Message)
		})
	}
}

func TestMapDomainError_UnknownError(t *testing.T) {
	unknown := errors.New("something unexpected happened")
	apiErr := mapDomainError(unknown)

	assert.Equal(t, "COMMON.INTERNAL", apiErr.Code)
	assert.Equal(t, http.StatusInternalServerError, apiErr.HTTPStatus)
	assert.Equal(t, "internal server error", apiErr.Message)
}

func TestMapDomainError_WrappedError(t *testing.T) {
	// errors.Is should traverse wrapped errors.
	wrapped := errors.Join(errors.New("context"), identity.ErrEmailTaken)
	apiErr := mapDomainError(wrapped)

	assert.Equal(t, "IDENTITY.EMAIL_TAKEN", apiErr.Code)
	assert.Equal(t, http.StatusConflict, apiErr.HTTPStatus)
}

func TestMapServiceError_BackwardCompat(t *testing.T) {
	status, message := mapServiceError(identity.ErrEmailTaken)
	assert.Equal(t, http.StatusConflict, status)
	assert.Equal(t, "email already registered", message)
}

func TestMapServiceError_Unknown(t *testing.T) {
	status, message := mapServiceError(errors.New("boom"))
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "internal server error", message)
}
