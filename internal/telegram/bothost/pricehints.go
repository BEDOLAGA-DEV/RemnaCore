package bothost

import "context"

// PriceHints carries the per-request personalization inputs a bot passes for
// plans.list / plans.get so the tariff reader can run the pricing pipeline
// (geo, promo, loyalty) instead of returning list prices. All fields are
// optional; an empty hint set yields list prices unchanged.
type PriceHints struct {
	UserID    string // resolved platform user id (empty = anonymous browse)
	Country   string // ISO-3166 for geo pricing
	PromoCode string // promo code to apply, if any
}

// IsZero reports whether the hints carry no personalization signal.
func (h PriceHints) IsZero() bool {
	return h.UserID == "" && h.Country == "" && h.PromoCode == ""
}

type priceHintsKey struct{}

// WithPriceHints returns a ctx carrying the given price hints. The tariff reader
// reads them via PriceHintsFromContext when building offers.
func WithPriceHints(ctx context.Context, h PriceHints) context.Context {
	return context.WithValue(ctx, priceHintsKey{}, h)
}

// PriceHintsFromContext returns the price hints stored in ctx (zero value when
// none were set).
func PriceHintsFromContext(ctx context.Context) PriceHints {
	if h, ok := ctx.Value(priceHintsKey{}).(PriceHints); ok {
		return h
	}
	return PriceHints{}
}
