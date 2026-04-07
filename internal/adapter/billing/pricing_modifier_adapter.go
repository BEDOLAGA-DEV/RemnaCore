package billing

import (
	"context"
	"encoding/json"
	"fmt"

	billingdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
)

// HookPricingCalculate is the plugin hook dispatched to modify invoice pricing.
const HookPricingCalculate = "pricing.calculate"

// pricingHookResult is the expected JSON response from the pricing.calculate
// hook. Pointer fields distinguish "not set" from "zero".
type pricingHookResult struct {
	Subtotal *int64 `json:"subtotal,omitempty"`
	Discount *int64 `json:"discount,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// PluginPricingModifier implements billing.PricingModifier by dispatching to
// the WASM plugin system via hookdispatch.Dispatcher. It handles JSON
// marshalling/unmarshalling of the plugin wire protocol, keeping the billing
// domain free of encoding concerns.
type PluginPricingModifier struct {
	dispatcher hookdispatch.Dispatcher
}

// NewPluginPricingModifier creates a PluginPricingModifier. If dispatcher is
// nil (no plugin system available), ModifyPricing always returns (nil, nil).
func NewPluginPricingModifier(dispatcher hookdispatch.Dispatcher) billingdomain.PricingModifier {
	return &PluginPricingModifier{dispatcher: dispatcher}
}

// BeginFlow pins plugin versions for the duration of a checkout flow. If no
// dispatcher is available, returns ctx unchanged.
func (m *PluginPricingModifier) BeginFlow(ctx context.Context) context.Context {
	if m.dispatcher == nil {
		return ctx
	}
	return m.dispatcher.BeginFlow(ctx)
}

// ModifyPricing dispatches the pricing.calculate hook to the plugin system and
// translates the JSON response into a domain PricingModification. Returns
// (nil, nil) when no dispatcher is available or no plugin handles the hook.
func (m *PluginPricingModifier) ModifyPricing(
	ctx context.Context,
	invoiceID, userID, planID string,
	subtotal int64,
	currency string,
) (*billingdomain.PricingModification, error) {
	if m.dispatcher == nil {
		return nil, nil
	}

	payload, err := json.Marshal(map[string]any{
		"invoice_id": invoiceID,
		"user_id":    userID,
		"plan_id":    planID,
		"subtotal":   subtotal,
		"currency":   currency,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal pricing request: %w", err)
	}

	output, err := m.dispatcher.DispatchSync(ctx, HookPricingCalculate, payload)
	if err != nil {
		return nil, fmt.Errorf("pricing hook dispatch: %w", err)
	}

	if output == nil {
		return nil, nil
	}

	var result pricingHookResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("unmarshal pricing result: %w", err)
	}

	return &billingdomain.PricingModification{
		Subtotal: result.Subtotal,
		Discount: result.Discount,
		Reason:   result.Reason,
	}, nil
}

// compile-time interface check
var _ billingdomain.PricingModifier = (*PluginPricingModifier)(nil)
