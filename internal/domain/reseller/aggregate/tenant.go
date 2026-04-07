package aggregate

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// APIKeyLen is the number of random bytes used for API keys.
// The resulting hex-encoded string is twice this length (64 chars).
const APIKeyLen = 32

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
	BrandingConfig vo.BrandingConfig
	APIKeyHash     string // SHA-256 hash of the API key
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// BrandingConfig is an alias for the vo.BrandingConfig value object,
// re-exported for backward compatibility within the aggregate package.
type BrandingConfig = vo.BrandingConfig

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

// SetBranding updates the tenant's branding configuration and records a
// TenantUpdated event.
func (t *Tenant) SetBranding(branding BrandingConfig, now time.Time) {
	t.BrandingConfig = branding
	t.UpdatedAt = now
	t.RecordEvent(domainevent.NewTyped(TenantUpdatedPayload{
		TenantID: t.ID,
	}, now, t.ID))
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
