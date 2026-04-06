package reseller

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

const (
	// APIKeyLen is the number of random bytes used for API keys.
	// The resulting hex-encoded string is twice this length (64 chars).
	APIKeyLen = 32

	// PercentBase is the divisor used when converting a percentage integer
	// (0-100) to a fractional multiplier for commission calculations.
	PercentBase = 100

	// MinCommissionRate is the minimum allowed commission percentage.
	MinCommissionRate = 0

	// MaxCommissionRate is the maximum allowed commission percentage.
	MaxCommissionRate = PercentBase

	// CommissionPending indicates a commission that has not yet been paid out.
	CommissionPending CommissionStatus = "pending"

	// CommissionPaid indicates a commission that has been paid to the reseller.
	CommissionPaid CommissionStatus = "paid"
)

// Tenant represents a white-label tenant on the platform.
// It embeds EventRecorder to accumulate domain events during mutations.
// Services must call DomainEvents() after persisting the aggregate to
// retrieve and publish all pending events.
type Tenant struct {
	domainevent.EventRecorder

	ID             string
	Name           string
	Domain         string // custom domain
	OwnerUserID    string
	BrandingConfig BrandingConfig
	APIKeyHash     string // SHA-256 hash of the API key
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// BrandingConfig holds the white-label customisation for a tenant.
type BrandingConfig struct {
	Logo         string `json:"logo"`
	PrimaryColor string `json:"primary_color"`
	AppName      string `json:"app_name"`
	SupportEmail string `json:"support_email"`
	SupportURL   string `json:"support_url"`
}

// ResellerAccount represents a reseller's account linked to a specific tenant.
// It embeds EventRecorder to accumulate domain events during mutations.
// Services must call DomainEvents() after persisting the aggregate to
// retrieve and publish all pending events.
type ResellerAccount struct {
	domainevent.EventRecorder

	ID             string
	TenantID       string
	UserID         string
	CommissionRate int   // percent (0-100)
	Balance        int64 // cents, accumulated commission
	CreatedAt      time.Time
}

// CommissionStatus represents the current state of a commission.
type CommissionStatus string

// commissionTransitions defines the valid state transitions for a commission.
var commissionTransitions = map[CommissionStatus][]CommissionStatus{
	CommissionPending: {CommissionPaid},
	CommissionPaid:    {},
}

// Commission records a commission earned by a reseller for a sale.
// It embeds EventRecorder to accumulate domain events during mutations.
// Services must call DomainEvents() after persisting the aggregate to
// retrieve and publish all pending events.
type Commission struct {
	domainevent.EventRecorder

	ID         string
	ResellerID string
	SaleID     string // subscription or invoice ID
	Amount     int64  // cents
	Currency   string
	Status     CommissionStatus
	CreatedAt  time.Time
	PaidAt     *time.Time
}

// CanTransitionTo reports whether the commission can move to the target status.
func (c *Commission) CanTransitionTo(target CommissionStatus) bool {
	allowed, ok := commissionTransitions[c.Status]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == target {
			return true
		}
	}
	return false
}

// MarkPaid transitions the commission from pending to paid.
func (c *Commission) MarkPaid(now time.Time) error {
	if !c.CanTransitionTo(CommissionPaid) {
		return ErrCommissionAlreadyPaid
	}
	c.Status = CommissionPaid
	c.PaidAt = &now
	c.RecordEvent(domainevent.NewTyped(CommissionPaidPayload{
		CommissionID: c.ID,
		ResellerID:   c.ResellerID,
		Amount:       c.Amount,
	}, now, c.ID))
	return nil
}

// NewTenant creates a new Tenant with a generated UUID and default settings.
// The creation event is recorded on the aggregate; callers must flush
// via DomainEvents() after persisting.
func NewTenant(name, domain, ownerUserID string, now time.Time) *Tenant {
	t := &Tenant{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Name:        name,
		Domain:      domain,
		OwnerUserID: ownerUserID,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	t.RecordEvent(domainevent.NewTyped(TenantCreatedPayload{
		TenantID:    t.ID,
		OwnerUserID: ownerUserID,
	}, now, t.ID))
	return t
}

// Deactivate marks the tenant as inactive.
func (t *Tenant) Deactivate(now time.Time) error {
	if !t.IsActive {
		return ErrTenantAlreadyInactive
	}
	t.IsActive = false
	t.UpdatedAt = now
	return nil
}

// Activate marks the tenant as active.
func (t *Tenant) Activate(now time.Time) error {
	if t.IsActive {
		return ErrTenantAlreadyActive
	}
	t.IsActive = true
	t.UpdatedAt = now
	return nil
}

// GenerateAPIKey creates a cryptographically random API key, stores its SHA-256
// hash on the tenant, and returns the plain-text key. The plain-text key is
// only available at generation time; it is NEVER persisted.
func (t *Tenant) GenerateAPIKey(now time.Time) (string, error) {
	keyBytes := make([]byte, APIKeyLen)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}

	plainKey := hex.EncodeToString(keyBytes)
	t.APIKeyHash = HashAPIKey(plainKey)
	t.UpdatedAt = now

	return plainKey, nil
}

// HashAPIKey computes the SHA-256 hex digest of a plain-text API key.
func HashAPIKey(plainKey string) string {
	h := sha256.Sum256([]byte(plainKey))
	return hex.EncodeToString(h[:])
}

// NewResellerAccount creates a new ResellerAccount after validating the
// commission rate is within the allowed range.
func NewResellerAccount(tenantID, userID string, commissionRate int, now time.Time) (*ResellerAccount, error) {
	if commissionRate < MinCommissionRate || commissionRate > MaxCommissionRate {
		return nil, ErrInvalidCommissionRate
	}

	return &ResellerAccount{
		ID:             uuid.Must(uuid.NewV7()).String(),
		TenantID:       tenantID,
		UserID:         userID,
		CommissionRate: commissionRate,
		Balance:        0,
		CreatedAt:      now,
	}, nil
}

// safeCommissionAmount calculates the commission amount with overflow protection.
// It returns ErrCommissionOverflow if the multiplication would exceed int64 range.
func safeCommissionAmount(saleAmount int64, rate int) (int64, error) {
	if saleAmount == 0 || rate == 0 {
		return 0, nil
	}
	r := int64(rate)
	if r > 0 && saleAmount > math.MaxInt64/r {
		return 0, ErrCommissionOverflow
	}
	if r < 0 && saleAmount < math.MaxInt64/r {
		return 0, ErrCommissionOverflow
	}
	return saleAmount * r / PercentBase, nil
}

// NewCommission calculates and creates a commission record for a sale.
// The creation event is recorded on the aggregate; callers must flush
// via DomainEvents() after persisting. Returns ErrCommissionOverflow if
// the sale amount and rate would cause integer overflow.
func NewCommission(resellerID, saleID string, saleAmount int64, commissionRate int, currency string, now time.Time) (*Commission, error) {
	amount, err := safeCommissionAmount(saleAmount, commissionRate)
	if err != nil {
		return nil, fmt.Errorf("commission for sale %s: %w", saleID, err)
	}

	c := &Commission{
		ID:         uuid.Must(uuid.NewV7()).String(),
		ResellerID: resellerID,
		SaleID:     saleID,
		Amount:     amount,
		Currency:   currency,
		Status:     CommissionPending,
		CreatedAt:  now,
	}
	c.RecordEvent(domainevent.NewTyped(CommissionCreatedPayload{
		CommissionID: c.ID,
		ResellerID:   resellerID,
		Amount:       amount,
	}, now, c.ID))
	return c, nil
}
