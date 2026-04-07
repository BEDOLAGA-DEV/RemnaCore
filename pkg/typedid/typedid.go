// Package typedid provides typed entity IDs for compile-time safety against
// accidental ID swaps between different entity types.
//
// Currently defined as type aliases for gradual migration; will be promoted
// to distinct types once all call sites are updated.
package typedid

// SubscriptionID identifies a billing subscription.
type SubscriptionID = string

// UserID identifies a platform user.
type UserID = string

// PlanID identifies a billing plan.
type PlanID = string

// InvoiceID identifies a billing invoice.
type InvoiceID = string

// BindingID identifies a Remnawave binding.
type BindingID = string

// FamilyGroupID identifies a family sharing group.
type FamilyGroupID = string

// PaymentID identifies a payment record.
type PaymentID = string

// PluginID identifies a WASM plugin.
type PluginID = string

// TenantID identifies a reseller tenant.
type TenantID = string
