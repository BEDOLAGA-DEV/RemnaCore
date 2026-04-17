package checkout

import "time"

// PluginSlug is the canonical slug for the checkout plugin.
const PluginSlug = "checkout"

// SessionStatus tracks the checkout session lifecycle.
type SessionStatus string

const (
	SessionPending    SessionStatus = "pending"
	SessionProcessing SessionStatus = "processing"
	SessionCompleted  SessionStatus = "completed"
	SessionFailed     SessionStatus = "failed"
	SessionExpired    SessionStatus = "expired"
	SessionAbandoned  SessionStatus = "abandoned"
	SessionCancelled  SessionStatus = "cancelled"
)

// PaymentMode indicates single or split payment.
type PaymentMode string

const (
	PaymentSingle PaymentMode = "single"
	PaymentSplit  PaymentMode = "split"
)

// CheckoutSession represents a checkout attempt.
type CheckoutSession struct {
	ID              string            `json:"id"`
	UserID          string            `json:"user_id"`
	TenantID        string            `json:"tenant_id,omitempty"`
	TariffID        string            `json:"tariff_id"`
	Quantity        int               `json:"quantity"`
	Currency        string            `json:"currency"`
	FinalPriceCents int64             `json:"final_price_cents"`
	PromoCode       string            `json:"promo_code,omitempty"`
	PromoDiscount   int64             `json:"promo_discount"`
	PaymentMode     PaymentMode       `json:"payment_mode"`
	Sources         []PaymentSource   `json:"sources,omitempty"`
	SubInvoices     []SubInvoice      `json:"sub_invoices,omitempty"`
	Status          SessionStatus     `json:"status"`
	PriceBreakdown  map[string]int64  `json:"price_breakdown,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	UserAgent       string            `json:"user_agent,omitempty"`
	IPAddress       string            `json:"ip_address,omitempty"`
	ReferrerURL     string            `json:"referrer_url,omitempty"`
	ExpiresAt       time.Time         `json:"expires_at"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	CompletedAt     *time.Time        `json:"completed_at,omitempty"`
	AbandonedAt     *time.Time        `json:"abandoned_at,omitempty"`
	ReminderSentAt  *time.Time        `json:"reminder_sent_at,omitempty"`
}

// PaymentSource represents a single payment source in a checkout.
type PaymentSource struct {
	Kind        string            `json:"kind"` // "balance_money" | "balance_bonus" | "stripe" | "yookassa" | "crypto" | "saved_card"
	AmountCents int64             `json:"amount_cents"`
	ProviderRef string            `json:"provider_ref,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// SubInvoice tracks a single payment within a split-payment checkout.
type SubInvoice struct {
	SubInvoiceID string     `json:"sub_invoice_id"`
	InvoiceID    string     `json:"invoice_id,omitempty"`
	SourceKind   string     `json:"source_kind"`
	AmountCents  int64      `json:"amount_cents"`
	Status       string     `json:"status"` // "pending" | "paid" | "failed"
	PaymentURL   string     `json:"payment_url,omitempty"`
	PaidAt       *time.Time `json:"paid_at,omitempty"`
}

// AvailableSource describes a payment option available to the user.
type AvailableSource struct {
	Kind           string `json:"kind"`
	Label          string `json:"label"`
	Currency       string `json:"currency,omitempty"`
	MaxAmountCents int64  `json:"max_amount_cents,omitempty"`
	Icon           string `json:"icon,omitempty"`
	Order          int    `json:"order"`
	Ref            string `json:"ref,omitempty"`
	MaxReasonHint  string `json:"max_reason_hint,omitempty"`
}

// SavedMethod represents a stored payment method (card token).
type SavedMethod struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Provider     string    `json:"provider"`
	TokenID      string    `json:"token_id"`
	Last4        string    `json:"last4"`
	Brand        string    `json:"brand"`
	ExpiresMonth int       `json:"expires_month"`
	ExpiresYear  int       `json:"expires_year"`
	IsDefault    bool      `json:"is_default"`
	CreatedAt    time.Time `json:"created_at"`
}

// --- Request DTOs ---

type CreateSessionRequest struct {
	TariffID string `json:"tariff_id"`
	Quantity int    `json:"quantity"`
}

type ConfirmSessionRequest struct {
	Sources    []PaymentSource `json:"sources"`
	PromoCode  string          `json:"promo_code,omitempty"`
	SaveMethod bool            `json:"save_method,omitempty"`
}

type ApplyPromoRequest struct {
	PromoCode string `json:"promo_code"`
}
