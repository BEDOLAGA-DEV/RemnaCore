package billing

import "context"

// PricingModification represents the result of a plugin-based pricing
// calculation. A nil *PricingModification from PricingModifier.ModifyPricing
// means no modification was applied (no pricing plugin registered, or the
// plugin returned no change).
type PricingModification struct {
	// Subtotal overrides the invoice subtotal when non-nil.
	Subtotal *int64
	// Discount is the discount amount in the smallest currency unit when non-nil.
	Discount *int64
	// Reason is an optional human-readable explanation (e.g., "loyalty_10pct").
	Reason string
}

// PricingModifier is an optional domain port for plugin-based pricing
// modifications. Implementations translate between the domain model and
// the plugin transport protocol (WASM JSON, gRPC, etc.).
//
// When no pricing plugin is registered, implementations return (nil, nil)
// to indicate no modification. When the plugin returns an error,
// implementations return (nil, err) and the caller decides whether to
// proceed with the original price (best-effort) or fail.
type PricingModifier interface {
	// BeginFlow pins plugin versions for the duration of a checkout flow so
	// that all hook calls (pricing, payment, etc.) use consistent plugin
	// versions even if a plugin is hot-reloaded mid-flow. Returns a context
	// that should be used for all subsequent operations within the flow.
	// If no plugin system is available, returns ctx unchanged.
	BeginFlow(ctx context.Context) context.Context

	// ModifyPricing dispatches a pricing calculation to the registered pricing
	// plugin and returns the modification result.
	ModifyPricing(ctx context.Context, invoiceID, userID, planID string, subtotal int64, currency string) (*PricingModification, error)
}
