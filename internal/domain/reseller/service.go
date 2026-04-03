package reseller

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// ResellerService implements the core reseller and white-label use-cases:
// tenant management, reseller account creation, commission tracking, and
// API key validation.
type ResellerService struct {
	tenants     TenantRepository
	commissions CommissionRepository
	publisher   domainevent.Publisher
	logger      *slog.Logger
}

// NewResellerService creates a ResellerService with the given dependencies.
func NewResellerService(
	tenants TenantRepository,
	commissions CommissionRepository,
	publisher domainevent.Publisher,
	logger *slog.Logger,
) *ResellerService {
	return &ResellerService{
		tenants:     tenants,
		commissions: commissions,
		publisher:   publisher,
		logger:      logger,
	}
}

// CreateTenant creates a new tenant, generates an API key, and returns both the
// persisted tenant and the plain-text API key (shown only once).
func (s *ResellerService) CreateTenant(ctx context.Context, name, domain, ownerUserID string) (*Tenant, string, error) {
	now := time.Now()
	tenant := NewTenant(name, domain, ownerUserID, now)

	plainKey, err := tenant.GenerateAPIKey(now)
	if err != nil {
		return nil, "", fmt.Errorf("generating API key: %w", err)
	}

	if err := s.tenants.CreateTenant(ctx, tenant); err != nil {
		return nil, "", fmt.Errorf("persisting tenant: %w", err)
	}

	if err := s.publisher.Publish(ctx, NewTenantCreatedEvent(tenant.ID, ownerUserID)); err != nil {
		s.logger.Warn("failed to publish event",
			slog.String("event_type", string(EventTenantCreated)),
			slog.String("error", err.Error()),
		)
	}

	s.logger.Info("tenant created",
		slog.String("tenant_id", tenant.ID),
		slog.String("name", name),
		slog.String("owner_user_id", ownerUserID),
	)

	return tenant, plainKey, nil
}

// GetTenant retrieves a tenant by ID.
func (s *ResellerService) GetTenant(ctx context.Context, tenantID string) (*Tenant, error) {
	tenant, err := s.tenants.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("finding tenant: %w", err)
	}
	return tenant, nil
}

// GetTenantByDomain retrieves a tenant by its custom domain.
func (s *ResellerService) GetTenantByDomain(ctx context.Context, domain string) (*Tenant, error) {
	tenant, err := s.tenants.GetTenantByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("finding tenant by domain: %w", err)
	}
	return tenant, nil
}

// ListTenants returns a paginated list of all tenants.
func (s *ResellerService) ListTenants(ctx context.Context, limit, offset int) ([]*Tenant, error) {
	tenants, err := s.tenants.ListTenants(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing tenants: %w", err)
	}
	return tenants, nil
}

// UpdateBranding updates the branding configuration for a tenant.
func (s *ResellerService) UpdateBranding(ctx context.Context, tenantID string, branding BrandingConfig) (*Tenant, error) {
	tenant, err := s.tenants.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("finding tenant: %w", err)
	}

	tenant.BrandingConfig = branding
	if err := s.tenants.UpdateTenant(ctx, tenant); err != nil {
		return nil, fmt.Errorf("updating tenant branding: %w", err)
	}

	return tenant, nil
}

// CreateResellerAccount creates a new reseller account linked to a tenant.
func (s *ResellerService) CreateResellerAccount(ctx context.Context, tenantID, userID string, rate int) (*ResellerAccount, error) {
	account, err := NewResellerAccount(tenantID, userID, rate, time.Now())
	if err != nil {
		return nil, err
	}

	if err := s.commissions.CreateResellerAccount(ctx, account); err != nil {
		return nil, fmt.Errorf("persisting reseller account: %w", err)
	}

	if err := s.publisher.Publish(ctx, NewResellerCreatedEvent(account.ID, tenantID, userID)); err != nil {
		s.logger.Warn("failed to publish event",
			slog.String("event_type", string(EventResellerCreated)),
			slog.String("error", err.Error()),
		)
	}

	s.logger.Info("reseller account created",
		slog.String("reseller_id", account.ID),
		slog.String("tenant_id", tenantID),
		slog.String("user_id", userID),
	)

	return account, nil
}

// RecordCommission creates a commission for a sale and updates the reseller's
// accumulated balance.
func (s *ResellerService) RecordCommission(ctx context.Context, resellerID, saleID string, saleAmount int64, rate int, currency string) (*Commission, error) {
	commission := NewCommission(resellerID, saleID, saleAmount, rate, currency, time.Now())

	if err := s.commissions.CreateCommission(ctx, commission); err != nil {
		return nil, fmt.Errorf("persisting commission: %w", err)
	}

	// Update accumulated balance.
	account, err := s.commissions.GetResellerAccountByID(ctx, resellerID)
	if err != nil {
		return nil, fmt.Errorf("finding reseller account: %w", err)
	}

	newBalance := account.Balance + commission.Amount
	if err := s.commissions.UpdateResellerBalance(ctx, resellerID, newBalance); err != nil {
		return nil, fmt.Errorf("updating reseller balance: %w", err)
	}

	if err := s.publisher.Publish(ctx, NewCommissionCreatedEvent(commission.ID, resellerID, commission.Amount)); err != nil {
		s.logger.Warn("failed to publish event",
			slog.String("event_type", string(EventCommissionCreated)),
			slog.String("error", err.Error()),
		)
	}

	return commission, nil
}

// GetPendingCommissions returns all pending commissions for a reseller.
func (s *ResellerService) GetPendingCommissions(ctx context.Context, resellerID string) ([]*Commission, error) {
	commissions, err := s.commissions.GetPendingCommissions(ctx, resellerID)
	if err != nil {
		return nil, fmt.Errorf("listing pending commissions: %w", err)
	}
	return commissions, nil
}

// ValidateAPIKey hashes the provided plain-text key and looks up the
// corresponding tenant. Returns ErrInvalidAPIKey if no match is found,
// or ErrTenantInactive if the matched tenant is disabled.
func (s *ResellerService) ValidateAPIKey(ctx context.Context, plainKey string) (*Tenant, error) {
	keyHash := HashAPIKey(plainKey)

	tenant, err := s.tenants.GetTenantByAPIKeyHash(ctx, keyHash)
	if err != nil {
		return nil, ErrInvalidAPIKey
	}

	if !tenant.IsActive {
		return nil, ErrTenantInactive
	}

	return tenant, nil
}
