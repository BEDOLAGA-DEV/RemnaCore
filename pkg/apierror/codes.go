package apierror

import "net/http"

// Identity error codes.
var (
	IdentityEmailTaken       = New("IDENTITY.EMAIL_TAKEN", "email already registered", http.StatusConflict)
	IdentityInvalidCreds     = New("IDENTITY.INVALID_CREDENTIALS", "invalid email or password", http.StatusUnauthorized)
	IdentityTokenExpired     = New("IDENTITY.TOKEN_EXPIRED", "token has expired", http.StatusGone)
	IdentitySessionExpired   = New("IDENTITY.SESSION_EXPIRED", "session has expired", http.StatusUnauthorized)
	IdentityNotFound         = New("IDENTITY.NOT_FOUND", "user not found", http.StatusNotFound)
	IdentityEmailNotVerified = New("IDENTITY.EMAIL_NOT_VERIFIED", "email not verified", http.StatusForbidden)
	IdentityPasswordTooShort = New("IDENTITY.PASSWORD_TOO_SHORT", "password must be at least 8 characters", http.StatusUnprocessableEntity)
	IdentityPasswordTooWeak  = New("IDENTITY.PASSWORD_TOO_WEAK", "password must contain uppercase, lowercase, and digit characters", http.StatusUnprocessableEntity)
	IdentityResetExpired     = New("IDENTITY.RESET_EXPIRED", "password reset token expired", http.StatusGone)
	IdentityResetNotFound    = New("IDENTITY.RESET_NOT_FOUND", "password reset token not found", http.StatusNotFound)
)

// Billing error codes.
var (
	BillingPlanNotFound            = New("BILLING.PLAN_NOT_FOUND", "plan not found", http.StatusNotFound)
	BillingSubscriptionNotFound    = New("BILLING.SUBSCRIPTION_NOT_FOUND", "subscription not found", http.StatusNotFound)
	BillingInvoiceNotFound         = New("BILLING.INVOICE_NOT_FOUND", "invoice not found", http.StatusNotFound)
	BillingFamilyGroupNotFound     = New("BILLING.FAMILY_GROUP_NOT_FOUND", "family group not found", http.StatusNotFound)
	BillingInvoiceAlreadyPaid      = New("BILLING.INVOICE_ALREADY_PAID", "invoice is already paid", http.StatusConflict)
	BillingInsufficientFunds       = New("BILLING.INSUFFICIENT_FUNDS", "insufficient payment amount", http.StatusBadRequest)
	BillingCurrencyMismatch        = New("BILLING.CURRENCY_MISMATCH", "currency mismatch", http.StatusBadRequest)
	BillingAddonNotAvailable       = New("BILLING.ADDON_NOT_AVAILABLE", "addon not available for this plan", http.StatusBadRequest)
	BillingSubscriptionNotActive   = New("BILLING.SUBSCRIPTION_NOT_ACTIVE", "subscription is not active", http.StatusConflict)
	BillingNotTrialStatus          = New("BILLING.NOT_TRIAL_STATUS", "subscription is not in trial status", http.StatusConflict)
	BillingCheckoutRateLimited     = New("BILLING.CHECKOUT_RATE_LIMITED", "checkout rate limit exceeded", http.StatusTooManyRequests)
	BillingAddonAlreadyOn          = New("BILLING.ADDON_ALREADY_ON_SUBSCRIPTION", "addon already on subscription", http.StatusConflict)
	BillingAddonNotOn              = New("BILLING.ADDON_NOT_ON_SUBSCRIPTION", "addon not on subscription", http.StatusNotFound)
	BillingPlanNotActive           = New("BILLING.PLAN_NOT_ACTIVE", "plan is not active", http.StatusBadRequest)
	BillingNoPriceConfigured       = New("BILLING.NO_PRICE_CONFIGURED", "plan has no price configured", http.StatusBadRequest)
	BillingFamilyNotEnabled        = New("BILLING.FAMILY_NOT_ENABLED", "family sharing not enabled for this plan", http.StatusConflict)
	BillingInvalidTransition       = New("BILLING.INVALID_TRANSITION", "invalid subscription state transition", http.StatusConflict)
	BillingMaxFamilyExceeded       = New("BILLING.MAX_FAMILY_EXCEEDED", "maximum family members exceeded", http.StatusConflict)
	BillingAlreadyMember           = New("BILLING.ALREADY_MEMBER", "user is already a member of this family group", http.StatusConflict)
	BillingCannotRemoveOwner       = New("BILLING.CANNOT_REMOVE_OWNER", "cannot remove the owner from the family group", http.StatusConflict)
	BillingMemberNotFound          = New("BILLING.MEMBER_NOT_FOUND", "member not found in family group", http.StatusNotFound)
	BillingEmptyPlanName           = New("BILLING.EMPTY_PLAN_NAME", "plan name must not be empty", http.StatusBadRequest)
	BillingBasePriceNotPositive    = New("BILLING.BASE_PRICE_NOT_POSITIVE", "base price must be positive", http.StatusBadRequest)
	BillingNoCountriesAllowed      = New("BILLING.NO_COUNTRIES_ALLOWED", "at least one country must be allowed", http.StatusBadRequest)
	BillingAddonAlreadyExists      = New("BILLING.ADDON_ALREADY_EXISTS", "addon already exists on this plan", http.StatusConflict)
	BillingAddonNotFound           = New("BILLING.ADDON_NOT_FOUND", "addon not found", http.StatusNotFound)
	BillingInvoiceRequiresItems    = New("BILLING.INVOICE_REQUIRES_LINE_ITEMS", "at least one line item is required", http.StatusBadRequest)
	BillingInvoiceMustBeDraft      = New("BILLING.INVOICE_MUST_BE_DRAFT", "invoice must be draft to mark pending", http.StatusConflict)
	BillingInvoiceMustBePending    = New("BILLING.INVOICE_MUST_BE_PENDING", "invoice must be pending to mark paid", http.StatusConflict)
	BillingInvoicePendingForFailed = New("BILLING.INVOICE_PENDING_FOR_FAILED", "invoice must be pending to mark failed", http.StatusConflict)
	BillingInvoiceMustBePaid       = New("BILLING.INVOICE_MUST_BE_PAID", "invoice must be paid to refund", http.StatusConflict)
)

// MultiSub error codes.
var (
	MultiSubBindingNotFound      = New("MULTISUB.BINDING_NOT_FOUND", "binding not found", http.StatusNotFound)
	MultiSubProvisioningFailed   = New("MULTISUB.PROVISIONING_FAILED", "provisioning failed", http.StatusInternalServerError)
	MultiSubDeprovisioningFailed = New("MULTISUB.DEPROVISIONING_FAILED", "deprovisioning failed", http.StatusInternalServerError)
	MultiSubSyncFailed           = New("MULTISUB.SYNC_FAILED", "sync failed", http.StatusInternalServerError)
	MultiSubBindingAlreadyActive = New("MULTISUB.BINDING_ALREADY_ACTIVE", "binding already active", http.StatusConflict)
	MultiSubRemnawaveUnavailable = New("MULTISUB.REMNAWAVE_UNAVAILABLE", "remnawave panel unavailable", http.StatusServiceUnavailable)
	MultiSubMaxBindingsExceeded  = New("MULTISUB.MAX_BINDINGS_EXCEEDED", "maximum bindings exceeded", http.StatusConflict)
	MultiSubSagaNotFound         = New("MULTISUB.SAGA_NOT_FOUND", "saga instance not found", http.StatusNotFound)
	MultiSubSagaAlreadyExists    = New("MULTISUB.SAGA_ALREADY_EXISTS", "saga instance already exists", http.StatusConflict)
)

// Payment error codes.
var (
	PaymentNotFound       = New("PAYMENT.NOT_FOUND", "payment not found", http.StatusNotFound)
	PaymentWebhookNotFound = New("PAYMENT.WEBHOOK_NOT_FOUND", "webhook log not found", http.StatusNotFound)
	PaymentWebhookDup     = New("PAYMENT.WEBHOOK_DUPLICATE", "duplicate webhook already processed", http.StatusConflict)
	PaymentNoPlugin       = New("PAYMENT.NO_PLUGIN", "no payment plugin configured", http.StatusServiceUnavailable)
	PaymentFailed         = New("PAYMENT.FAILED", "payment processing failed", http.StatusBadGateway)
	PaymentVerifyFailed   = New("PAYMENT.VERIFICATION_FAILED", "webhook verification failed", http.StatusBadRequest)
	PaymentRefundFailed   = New("PAYMENT.REFUND_FAILED", "refund processing failed", http.StatusBadGateway)
	PaymentInvalidProvider = New("PAYMENT.INVALID_PROVIDER", "invalid payment provider", http.StatusBadRequest)
	PaymentMissingInvoice = New("PAYMENT.MISSING_INVOICE_ID", "invoice ID is required", http.StatusBadRequest)
	PaymentMissingAmount  = New("PAYMENT.MISSING_AMOUNT", "payment amount must be positive", http.StatusBadRequest)
	PaymentMissingCurrency = New("PAYMENT.MISSING_CURRENCY", "currency is required", http.StatusBadRequest)
	PaymentMissingExtID   = New("PAYMENT.MISSING_EXTERNAL_ID", "external ID is required", http.StatusBadRequest)
	PaymentInvalidState   = New("PAYMENT.INVALID_STATE", "invalid payment state transition", http.StatusConflict)
)

// Reseller error codes.
var (
	ResellerNotFound          = New("RESELLER.NOT_FOUND", "reseller resource not found", http.StatusNotFound)
	ResellerTenantNotFound    = New("RESELLER.TENANT_NOT_FOUND", "tenant not found", http.StatusNotFound)
	ResellerAccountNotFound   = New("RESELLER.ACCOUNT_NOT_FOUND", "reseller account not found", http.StatusNotFound)
	ResellerCommissionNotFound = New("RESELLER.COMMISSION_NOT_FOUND", "commission not found", http.StatusNotFound)
	ResellerInvalidCommission = New("RESELLER.INVALID_COMMISSION_RATE", "commission rate must be between 0 and 100", http.StatusBadRequest)
	ResellerInvalidAPIKey     = New("RESELLER.INVALID_API_KEY", "invalid API key", http.StatusUnauthorized)
	ResellerTenantInactive    = New("RESELLER.TENANT_INACTIVE", "tenant is inactive", http.StatusForbidden)
	ResellerDuplicateDomain   = New("RESELLER.DUPLICATE_DOMAIN", "domain already in use", http.StatusConflict)
)

// Plugin error codes.
var (
	PluginNotFound        = New("PLUGIN.NOT_FOUND", "plugin not found", http.StatusNotFound)
	PluginAlreadyExists   = New("PLUGIN.ALREADY_EXISTS", "plugin already exists", http.StatusConflict)
	PluginInvalidManifest = New("PLUGIN.INVALID_MANIFEST", "invalid plugin manifest", http.StatusBadRequest)
	PluginInvalidSlug     = New("PLUGIN.INVALID_SLUG", "invalid plugin slug", http.StatusBadRequest)
	PluginNotEnabled      = New("PLUGIN.NOT_ENABLED", "plugin is not enabled", http.StatusConflict)
	PluginAlreadyEnabled  = New("PLUGIN.ALREADY_ENABLED", "plugin is already enabled", http.StatusConflict)
	PluginHookTimeout     = New("PLUGIN.HOOK_TIMEOUT", "hook execution timed out", http.StatusGatewayTimeout)
	PluginHookHalted      = New("PLUGIN.HOOK_HALTED", "hook execution halted by plugin", http.StatusBadGateway)
	PluginPermissionDenied = New("PLUGIN.PERMISSION_DENIED", "plugin permission denied", http.StatusForbidden)
	PluginStorageQuota    = New("PLUGIN.STORAGE_QUOTA_EXCEEDED", "plugin storage quota exceeded", http.StatusInsufficientStorage)
	PluginHTTPRateLimit   = New("PLUGIN.HTTP_RATE_LIMIT_EXCEEDED", "plugin HTTP rate limit exceeded", http.StatusTooManyRequests)
	PluginInternalNetwork = New("PLUGIN.INTERNAL_NETWORK_ACCESS", "access to internal network addresses is forbidden", http.StatusForbidden)
	PluginWASMCompileFail = New("PLUGIN.WASM_COMPILATION_FAILED", "WASM compilation failed", http.StatusUnprocessableEntity)
	PluginNoHandler       = New("PLUGIN.NO_HANDLER_FOR_HOOK", "no plugin handler for hook", http.StatusNotFound)
	PluginSlugMismatch    = New("PLUGIN.SLUG_MISMATCH", "plugin slug mismatch during hot reload", http.StatusBadRequest)
	PluginNotRunning      = New("PLUGIN.NOT_RUNNING", "plugin is not running", http.StatusConflict)
	PluginIncompatibleSDK = New("PLUGIN.INCOMPATIBLE_SDK", "incompatible plugin SDK version", http.StatusConflict)
	PluginDraining        = New("PLUGIN.DRAINING", "plugin is draining", http.StatusServiceUnavailable)
	PluginWASMNotFound    = New("PLUGIN.WASM_NOT_FOUND", "WASM binary not found in content store", http.StatusNotFound)
	PluginMissingConfig   = New("PLUGIN.MISSING_CONFIG", "plugin missing required configuration", http.StatusBadRequest)
)

// Routing error codes.
var (
	RoutingNoNodes = New("ROUTING.NO_NODES_AVAILABLE", "no suitable node available", http.StatusServiceUnavailable)
)

// Common error codes used across all domains.
var (
	ValidationFailed = New("COMMON.VALIDATION_ERROR", "invalid request body", http.StatusUnprocessableEntity)
	NotFound         = New("COMMON.NOT_FOUND", "resource not found", http.StatusNotFound)
	Internal         = New("COMMON.INTERNAL", "internal server error", http.StatusInternalServerError)
	BodyTooLarge     = New("COMMON.BODY_TOO_LARGE", "request body too large", http.StatusRequestEntityTooLarge)
	Unauthorized     = New("COMMON.UNAUTHORIZED", "authentication required", http.StatusUnauthorized)
	Forbidden        = New("COMMON.FORBIDDEN", "access denied", http.StatusForbidden)
)
