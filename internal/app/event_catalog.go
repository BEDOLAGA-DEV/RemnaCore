package app

import (
	"reflect"

	billingaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	multisubaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra/health"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// Producer name constants used in catalog entries.
const (
	producerBilling  = "billing"
	producerMultisub = "multisub"
	producerPayment  = "payment"
	producerIdentity = "identity"
	producerReseller = "reseller"
	producerPlugin   = "plugin"
	producerInfra    = "infra"
)

// BuildEventCatalog creates the complete event catalog with all domain events
// from every bounded context. This function is called at application startup
// and its result is used for runtime validation and architecture tests.
//
// Every EventType constant in the codebase MUST be registered here. The
// architecture test TestAllEventTypesInCatalog enforces completeness.
func BuildEventCatalog() *domainevent.EventCatalog {
	c := domainevent.NewEventCatalog()

	registerBillingEvents(c)
	registerMultisubEvents(c)
	registerPaymentEvents(c)
	registerIdentityEvents(c)
	registerResellerEvents(c)
	registerPluginEvents(c)
	registerInfraEvents(c)

	return c
}

func registerBillingEvents(c *domainevent.EventCatalog) {
	// --- Subscription events ---
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventSubCreated,
		Producer:    producerBilling,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(billingaggregate.SubCreatedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventSubActivated,
		Producer:    producerBilling,
		Consumers:   []string{producerMultisub},
		PayloadType: reflect.TypeOf(billingaggregate.SubActivatedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventSubCancelled,
		Producer:    producerBilling,
		Consumers:   []string{producerMultisub},
		PayloadType: reflect.TypeOf(billingaggregate.SubCancelledPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventSubRenewed,
		Producer:    producerBilling,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(billingaggregate.SubRenewedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventSubPaused,
		Producer:    producerBilling,
		Consumers:   []string{producerMultisub},
		PayloadType: reflect.TypeOf(billingaggregate.SubPausedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventSubResumed,
		Producer:    producerBilling,
		Consumers:   []string{producerMultisub},
		PayloadType: reflect.TypeOf(billingaggregate.SubResumedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventSubExpired,
		Producer:    producerBilling,
		Consumers:   []string{}, // TODO: multisub should deprovision on expiry
		PayloadType: reflect.TypeOf(billingaggregate.SubExpiredPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventSubPastDue,
		Producer:    producerBilling,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(billingaggregate.SubPastDuePayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventSubUpgraded,
		Producer:    producerBilling,
		Consumers:   []string{}, // TODO: multisub should reprovision on upgrade
		PayloadType: reflect.TypeOf(billingaggregate.SubUpgradedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventSubDowngraded,
		Producer:    producerBilling,
		Consumers:   []string{}, // TODO: multisub should handle at next renewal
		PayloadType: reflect.TypeOf(billingaggregate.SubDowngradedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventSubTrialStarted,
		Producer:    producerBilling,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(billingaggregate.SubTrialStartedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventSubTrialEnding,
		Producer:    producerBilling,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(billingaggregate.SubTrialEndingPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventSubUpdated,
		Producer:    producerBilling,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(billingaggregate.SubUpdatedPayload{}),
		Version:     1,
	})

	// --- Invoice events ---
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventInvCreated,
		Producer:    producerBilling,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(billingaggregate.InvCreatedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventInvPaid,
		Producer:    producerBilling,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(billingaggregate.InvPaidPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventInvFailed,
		Producer:    producerBilling,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(billingaggregate.InvFailedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventInvRefunded,
		Producer:    producerBilling,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(billingaggregate.InvRefundedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventInvDiscountApplied,
		Producer:    producerBilling,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(billingaggregate.InvDiscountAppliedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventInvTotalOverridden,
		Producer:    producerBilling,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(billingaggregate.InvTotalOverriddenPayload{}),
		Version:     1,
	})

	// --- Family events ---
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventFamilyCreated,
		Producer:    producerBilling,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(billingaggregate.FamilyCreatedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventFamilyMemberAdded,
		Producer:    producerBilling,
		Consumers:   []string{}, // TODO: multisub should provision family binding
		PayloadType: reflect.TypeOf(billingaggregate.FamilyMemberAddedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventFamilyMemberRemoved,
		Producer:    producerBilling,
		Consumers:   []string{}, // TODO: multisub should deprovision family binding
		PayloadType: reflect.TypeOf(billingaggregate.FamilyMemberRemovedPayload{}),
		Version:     1,
	})

	// --- Plan events ---
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventPlanCreated,
		Producer:    producerBilling,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(billingaggregate.PlanCreatedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventPlanUpdated,
		Producer:    producerBilling,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(billingaggregate.PlanUpdatedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        billingaggregate.EventPlanDeactivated,
		Producer:    producerBilling,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(billingaggregate.PlanDeactivatedPayload{}),
		Version:     1,
	})
}

func registerMultisubEvents(c *domainevent.EventCatalog) {
	// --- Binding lifecycle events (defined in aggregate package) ---
	c.Register(domainevent.CatalogEntry{
		Type:        multisubaggregate.EventBindingCreated,
		Producer:    producerMultisub,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(multisubaggregate.BindingCreatedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        multisubaggregate.EventBindingProvisioned,
		Producer:    producerMultisub,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(multisubaggregate.BindingProvisionedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        multisubaggregate.EventBindingDeprovisioned,
		Producer:    producerMultisub,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(multisubaggregate.BindingDeprovisionedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        multisubaggregate.EventBindingFailed,
		Producer:    producerMultisub,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(multisubaggregate.BindingFailedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        multisubaggregate.EventBindingDisabled,
		Producer:    producerMultisub,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(multisubaggregate.BindingDisabledPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        multisubaggregate.EventBindingEnabled,
		Producer:    producerMultisub,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(multisubaggregate.BindingEnabledPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        multisubaggregate.EventBindingLimited,
		Producer:    producerMultisub,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(multisubaggregate.BindingLimitedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        multisubaggregate.EventBindingUnlimited,
		Producer:    producerMultisub,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(multisubaggregate.BindingUnlimitedPayload{}),
		Version:     1,
	})

	// --- Sync/orchestration events (defined in multisub package) ---
	c.Register(domainevent.CatalogEntry{
		Type:        multisubaggregate.EventBindingSyncFailed,
		Producer:    producerMultisub,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(multisub.BindingSyncFailedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        multisubaggregate.EventBindingSyncCompleted,
		Producer:    producerMultisub,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(multisub.BindingSyncCompletedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        multisubaggregate.EventBindingTrafficExceeded,
		Producer:    producerMultisub,
		Consumers:   []string{producerBilling},
		PayloadType: reflect.TypeOf(multisub.BindingTrafficExceededPayload{}),
		Version:     1,
	})
}

func registerPaymentEvents(c *domainevent.EventCatalog) {
	c.Register(domainevent.CatalogEntry{
		Type:        payment.EventChargeCreated,
		Producer:    producerPayment,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(payment.ChargeCreatedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        payment.EventChargeCompleted,
		Producer:    producerPayment,
		Consumers:   []string{producerBilling},
		PayloadType: reflect.TypeOf(payment.ChargeCompletedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        payment.EventChargeFailed,
		Producer:    producerPayment,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(payment.ChargeFailedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        payment.EventRefundCompleted,
		Producer:    producerPayment,
		Consumers:   []string{producerBilling},
		PayloadType: reflect.TypeOf(payment.RefundCompletedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        payment.EventWebhookReceived,
		Producer:    producerPayment,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(payment.WebhookReceivedPayload{}),
		Version:     1,
	})
}

func registerIdentityEvents(c *domainevent.EventCatalog) {
	c.Register(domainevent.CatalogEntry{
		Type:        identity.EventUserRegistered,
		Producer:    producerIdentity,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(identity.UserRegisteredPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        identity.EventEmailVerified,
		Producer:    producerIdentity,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(identity.EmailVerifiedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        identity.EventUserLoggedIn,
		Producer:    producerIdentity,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(identity.UserLoggedInPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        identity.EventProfileUpdated,
		Producer:    producerIdentity,
		Consumers:   []string{},
		PayloadType: nil, // reserved for future use; no typed payload yet
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        identity.EventTokenRefreshed,
		Producer:    producerIdentity,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(identity.TokenRefreshedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        identity.EventPasswordResetRequested,
		Producer:    producerIdentity,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(identity.PasswordResetRequestedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        identity.EventPasswordReset,
		Producer:    producerIdentity,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(identity.PasswordResetPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        identity.EventPasswordChanged,
		Producer:    producerIdentity,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(identity.PasswordChangedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        identity.EventTelegramLinked,
		Producer:    producerIdentity,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(identity.TelegramLinkedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        identity.EventTelegramUnlinked,
		Producer:    producerIdentity,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(identity.TelegramUnlinkedPayload{}),
		Version:     1,
	})
}

func registerResellerEvents(c *domainevent.EventCatalog) {
	c.Register(domainevent.CatalogEntry{
		Type:        reseller.EventTenantCreated,
		Producer:    producerReseller,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(reseller.TenantCreatedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        reseller.EventTenantUpdated,
		Producer:    producerReseller,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(reseller.TenantUpdatedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        reseller.EventResellerCreated,
		Producer:    producerReseller,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(reseller.ResellerCreatedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        reseller.EventCommissionCreated,
		Producer:    producerReseller,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(reseller.CommissionCreatedPayload{}),
		Version:     1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:        reseller.EventCommissionPaid,
		Producer:    producerReseller,
		Consumers:   []string{},
		PayloadType: reflect.TypeOf(reseller.CommissionPaidPayload{}),
		Version:     1,
	})
}

func registerPluginEvents(c *domainevent.EventCatalog) {
	// Plugin events use map[string]any payloads (no typed structs).
	c.Register(domainevent.CatalogEntry{
		Type:      plugin.EventPluginInstalled,
		Producer:  producerPlugin,
		Consumers: []string{},
		Version:   1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:      plugin.EventPluginEnabled,
		Producer:  producerPlugin,
		Consumers: []string{},
		Version:   1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:      plugin.EventPluginDisabled,
		Producer:  producerPlugin,
		Consumers: []string{},
		Version:   1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:      plugin.EventPluginUninstalled,
		Producer:  producerPlugin,
		Consumers: []string{},
		Version:   1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:      plugin.EventPluginHotReloaded,
		Producer:  producerPlugin,
		Consumers: []string{},
		Version:   1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:      plugin.EventPluginError,
		Producer:  producerPlugin,
		Consumers: []string{},
		Version:   1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:      plugin.EventHookExecuted,
		Producer:  producerPlugin,
		Consumers: []string{},
		Version:   1,
	})
	c.Register(domainevent.CatalogEntry{
		Type:      plugin.EventHookFailed,
		Producer:  producerPlugin,
		Consumers: []string{},
		Version:   1,
	})
}

func registerInfraEvents(c *domainevent.EventCatalog) {
	// Infra events use map[string]any payloads (no typed structs).
	c.Register(domainevent.CatalogEntry{
		Type:      health.EventNodeHealthChanged,
		Producer:  producerInfra,
		Consumers: []string{},
		Version:   1,
	})
}
