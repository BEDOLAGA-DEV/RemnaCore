package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/secret"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// botTokenRe matches a valid Telegram bot token: <bot_id>:<random_suffix>.
// The random suffix must be at least 35 characters from [A-Za-z0-9_-].
// Note: getMe token-liveness validation is deferred to SP3 to keep SP1 network-free.
var botTokenRe = regexp.MustCompile(`^\d+:[A-Za-z0-9_-]{35,}$`)

// cabinetURLPrefix is the required scheme for cabinet URLs.
const cabinetURLPrefix = "https://"

// Deprecated: TenantRepository is kept for backward compatibility. The
// canonical interface is defined in the reseller domain root (repository.go).
// New code should reference reseller.TenantRepository directly.
type TenantRepository interface {
	CreateTenant(ctx context.Context, tenant *aggregate.Tenant) error
	GetTenantByID(ctx context.Context, id string) (*aggregate.Tenant, error)
	GetTenantByIDForUpdate(ctx context.Context, id string) (*aggregate.Tenant, error)
	GetTenantByDomain(ctx context.Context, domain string) (*aggregate.Tenant, error)
	GetTenantByAPIKeyHash(ctx context.Context, keyHash string) (*aggregate.Tenant, error)
	UpdateTenant(ctx context.Context, tenant *aggregate.Tenant) error
	ListTenants(ctx context.Context, limit, offset int) ([]*aggregate.Tenant, error)
	// SetTenantOwnerUserID persists an owner assignment on a pending-owner tenant.
	SetTenantOwnerUserID(ctx context.Context, tenantID, userID string, now time.Time) error
}

// Deprecated: CommissionRepository is kept for backward compatibility. The
// canonical interface is defined in the reseller domain root (repository.go).
// New code should reference reseller.CommissionRepository directly.
type CommissionRepository interface {
	CreateResellerAccount(ctx context.Context, account *aggregate.ResellerAccount) error
	GetResellerAccountByID(ctx context.Context, id string) (*aggregate.ResellerAccount, error)
	GetResellerAccountByUserAndTenant(ctx context.Context, userID, tenantID string) (*aggregate.ResellerAccount, error)

	CreateCommission(ctx context.Context, commission *aggregate.Commission) error
	GetCommissionByID(ctx context.Context, id string) (*aggregate.Commission, error)
	GetCommissionByIDForUpdate(ctx context.Context, id string) (*aggregate.Commission, error)
	GetPendingCommissions(ctx context.Context, resellerID string) ([]*aggregate.Commission, error)
	ListCommissionsByTenant(ctx context.Context, tenantID, resellerID string) ([]*aggregate.Commission, error)
	UpdateCommission(ctx context.Context, commission *aggregate.Commission) error

	UpdateResellerBalance(ctx context.Context, resellerID string, balance int64) error
}

// CustomerSummary is a tenant-scoped view of one of a shop's customers
// (an identity.platform_users row whose tenant_id is the active shop) plus
// lightweight rollups used by the reseller customers list. Canonical here;
// the reseller root re-exports it as a type alias.
type CustomerSummary struct {
	UserID          string
	Email           string
	DisplayName     string
	ActiveSubsCount int
	CreatedAt       time.Time
}

// DashboardSummary is the purpose-built tenant-scoped aggregate backing the
// reseller dashboard (NOT the platform-only /api/admin/stats view). Canonical
// here; the reseller root re-exports it as a type alias.
type DashboardSummary struct {
	ActiveCustomers     int
	ActiveSubscriptions int
	PendingCommission   int64 // cents, sum of pending commissions for the shop
	Currency            string
}

// CustomerRepository reads a shop's customers from identity.platform_users.
// All methods rely on the active app.tenant_id GUC (RLS on platform_users) and
// ALSO carry an explicit tenant_id predicate (spec §7, belt-and-suspenders).
// Canonical here; the reseller root re-exports it as a type alias.
type CustomerRepository interface {
	ListCustomersByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*CustomerSummary, error)
	CountActiveCustomersByTenant(ctx context.Context, tenantID string) (int, error)
	CountActiveSubscriptionsByTenant(ctx context.Context, tenantID string) (int, error)
	SumPendingCommissionByTenant(ctx context.Context, tenantID string) (amount int64, currency string, err error)
}

// ShopBotRepository persists per-shop Telegram bot configuration. Bot tokens
// are encrypted at rest by the adapter; callers always supply and receive
// plaintext (wrapped in secret.String). Structurally identical to
// reseller.ShopBotRepository (defined at the domain root); this copy exists
// here to avoid an import cycle — the reseller root imports this subpackage.
type ShopBotRepository interface {
	// Upsert inserts or updates the bot config for tenantID.
	Upsert(ctx context.Context, tenantID string, cfg vo.ShopBotConfig) error
	// GetByTenant returns the decrypted bot config for tenantID.
	// Returns ErrShopBotNotFound when no row exists or RLS blocks access.
	GetByTenant(ctx context.Context, tenantID string) (*vo.ShopBotConfig, error)
	// ListEnabled returns all enabled bot configs under platform-sentinel scope.
	ListEnabled(ctx context.Context) ([]vo.ShopBotWithTenant, error)
}

// BotPluginValidator reports whether slug names an installed, enabled,
// bot-capable plugin. Implemented in the wiring layer over the built-in bot
// registry (BP1) and the plugin repo (BP2); kept as a port so the reseller
// domain does not import internal/plugin or internal/telegram.
type BotPluginValidator interface {
	IsValidBotPlugin(ctx context.Context, slug string) (bool, error)
}

// AllowAllBotPlugins is a no-op BotPluginValidator that accepts every slug. It
// is a test-only default so service unit tests can construct a ResellerService
// without wiring a real validator. Production uses the telegram-backed
// implementation (telegram.NewBotPluginValidator over the BuiltinBotRegistry).
type AllowAllBotPlugins struct{}

// IsValidBotPlugin always returns true (test-only; see AllowAllBotPlugins).
func (AllowAllBotPlugins) IsValidBotPlugin(_ context.Context, _ string) (bool, error) {
	return true, nil
}

// SetShopBotInput is the input DTO for ResellerService.SetShopBot.
type SetShopBotInput struct {
	BotToken   string
	CabinetURL string
	Enabled    bool
	// BotPlugin is the slug of the bot plugin to associate with this shop's
	// bot. An empty string selects the default cabinet-bot behaviour.
	BotPlugin string
}

// ResellerService implements the core reseller and white-label use-cases:
// tenant management, reseller account creation, commission tracking, and
// API key validation.
//
// # Architectural rationale
//
// ResellerService is a "thick" domain service that combines domain logic
// (tenant construction, commission calculation, API key validation) with
// application orchestration (transaction management, event publishing,
// repository coordination). In strict hexagonal architecture these would be
// separate layers, but RemnaCore intentionally merges them:
//
//   - Every public method follows the same pattern: construct or load aggregates,
//     enforce invariants, persist changes and publish events. A separate
//     application service would duplicate this ceremony without adding meaningful
//     separation.
//
//   - Domain entities (Tenant, ResellerAccount, Commission) own their own
//     validation and state transitions. ResellerService orchestrates them but
//     does not contain business rules -- those live in the entities.
//
//   - Commission recording uses txmanager.Runner to atomically persist the
//     commission and update the reseller balance, keeping transaction boundaries
//     explicit and testable without depending on concrete database infrastructure.
//
// Rationale: in a modular monolith, the application service layer would be a
// thin pass-through that adds ceremony without adding separation. The thick
// service pattern keeps the reseller context's public API in one place, making
// it easier to audit invariants and transaction boundaries.
//
// When to split: if ResellerService exceeds ~500 lines or gains responsibilities
// beyond tenant/commission CRUD (e.g., payout orchestration, multi-currency
// settlement), extract focused services (e.g., PayoutService, TenantService)
// that each own a subset of the aggregate interactions.
type ResellerService struct {
	tenants     TenantRepository
	commissions CommissionRepository
	customers   CustomerRepository
	shopBots    ShopBotRepository
	botPlugins  BotPluginValidator
	publisher   domainevent.Publisher
	logger      *slog.Logger
	clock       clock.Clock
	txRunner    txmanager.Runner
}

// NewResellerService creates a ResellerService with the given dependencies.
func NewResellerService(
	tenants TenantRepository,
	commissions CommissionRepository,
	customers CustomerRepository,
	publisher domainevent.Publisher,
	logger *slog.Logger,
	clk clock.Clock,
	txRunner txmanager.Runner,
	shopBots ShopBotRepository,
	botPlugins BotPluginValidator,
) *ResellerService {
	return &ResellerService{
		tenants:     tenants,
		commissions: commissions,
		customers:   customers,
		shopBots:    shopBots,
		botPlugins:  botPlugins,
		publisher:   publisher,
		logger:      logger,
		clock:       clk,
		txRunner:    txRunner,
	}
}

// CreateTenant creates a new tenant, generates an API key, and returns both the
// persisted tenant and the plain-text API key (shown only once).
// ownerUserID is optional (nil for pending-owner shops created before the owner exists).
func (s *ResellerService) CreateTenant(ctx context.Context, name, domain string, ownerUserID *string) (*aggregate.Tenant, string, error) {
	now := s.clock.Now()
	tenant := aggregate.NewTenant(name, domain, ownerUserID, now)

	plainKey, err := tenant.GenerateAPIKey(now)
	if err != nil {
		return nil, "", fmt.Errorf("generating API key: %w", err)
	}

	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.tenants.CreateTenant(txCtx, tenant); err != nil {
			return fmt.Errorf("persisting tenant: %w", err)
		}
		if err := domainevent.PublishAll(txCtx, s.publisher, tenant); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, "", err
	}

	s.logger.Info("tenant created",
		slog.String("tenant_id", tenant.ID),
		slog.String("name", name),
		slog.String("owner_user_id", pgutil.DerefStr(ownerUserID)),
	)

	return tenant, plainKey, nil
}

// SetTenantOwner assigns an owner to a pending-owner tenant. Used when an
// invited shop owner accepts their invitation and the tenant already exists.
func (s *ResellerService) SetTenantOwner(ctx context.Context, tenantID, userID string) error {
	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		t, err := s.tenants.GetTenantByIDForUpdate(txCtx, tenantID)
		if err != nil {
			return fmt.Errorf("finding tenant: %w", err)
		}
		now := s.clock.Now()
		t.SetOwner(userID, now)
		return s.tenants.SetTenantOwnerUserID(txCtx, t.ID, userID, now)
	})
}

// GetTenant retrieves a tenant by ID.
func (s *ResellerService) GetTenant(ctx context.Context, tenantID string) (*aggregate.Tenant, error) {
	tenant, err := s.tenants.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("finding tenant: %w", err)
	}
	return tenant, nil
}

// GetTenantByDomain retrieves a tenant by its custom domain.
func (s *ResellerService) GetTenantByDomain(ctx context.Context, domain string) (*aggregate.Tenant, error) {
	tenant, err := s.tenants.GetTenantByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("finding tenant by domain: %w", err)
	}
	return tenant, nil
}

// ListTenants returns a paginated list of all tenants.
func (s *ResellerService) ListTenants(ctx context.Context, limit, offset int) ([]*aggregate.Tenant, error) {
	tenants, err := s.tenants.ListTenants(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing tenants: %w", err)
	}
	return tenants, nil
}

// UpdateBranding updates the branding configuration for a tenant. The read
// (with FOR UPDATE lock), mutation, update, and event publish are all inside a
// single transaction to prevent TOCTOU races.
func (s *ResellerService) UpdateBranding(ctx context.Context, tenantID string, branding vo.BrandingConfig) (*aggregate.Tenant, error) {
	var tenant *aggregate.Tenant
	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		tenant, err = s.tenants.GetTenantByIDForUpdate(txCtx, tenantID)
		if err != nil {
			return fmt.Errorf("finding tenant: %w", err)
		}

		tenant.SetBranding(branding, s.clock.Now())

		if err := s.tenants.UpdateTenant(txCtx, tenant); err != nil {
			return fmt.Errorf("updating tenant branding: %w", err)
		}
		return domainevent.PublishAll(txCtx, s.publisher, tenant)
	}); err != nil {
		return nil, err
	}

	return tenant, nil
}

// CreateResellerAccount creates a new reseller account linked to a tenant.
func (s *ResellerService) CreateResellerAccount(ctx context.Context, tenantID, userID string, rate int) (*aggregate.ResellerAccount, error) {
	account, err := aggregate.NewResellerAccount(tenantID, userID, rate, s.clock.Now())
	if err != nil {
		return nil, err
	}

	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.commissions.CreateResellerAccount(txCtx, account); err != nil {
			return fmt.Errorf("persisting reseller account: %w", err)
		}
		if err := domainevent.PublishAll(txCtx, s.publisher, account); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	s.logger.Info("reseller account created",
		slog.String("reseller_id", account.ID),
		slog.String("tenant_id", tenantID),
		slog.String("user_id", userID),
	)

	return account, nil
}

// RecordCommission creates a commission for a sale and updates the reseller's
// accumulated balance. The commission creation and balance update are wrapped
// in a database transaction to prevent race conditions on concurrent writes.
func (s *ResellerService) RecordCommission(ctx context.Context, resellerID, saleID string, saleAmount int64, rate int, currency string) (*aggregate.Commission, error) {
	commission, err := aggregate.NewCommission(resellerID, saleID, saleAmount, rate, currency, s.clock.Now())
	if err != nil {
		return nil, fmt.Errorf("creating commission: %w", err)
	}

	err = s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.commissions.CreateCommission(txCtx, commission); err != nil {
			return fmt.Errorf("persisting commission: %w", err)
		}

		account, err := s.commissions.GetResellerAccountByID(txCtx, resellerID)
		if err != nil {
			return fmt.Errorf("finding reseller account: %w", err)
		}

		newBalance := account.Balance + commission.Amount.Amount
		if err := s.commissions.UpdateResellerBalance(txCtx, resellerID, newBalance); err != nil {
			return fmt.Errorf("updating reseller balance: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("record commission tx: %w", err)
	}

	// Aggregate already recorded its own created event in NewCommission().
	if err := domainevent.PublishAll(ctx, s.publisher, commission); err != nil {
		s.logger.Warn("failed to publish aggregate events",
			slog.String("event_type", string(aggregate.EventCommissionCreated)),
			slog.Any("error", err),
		)
	}

	return commission, nil
}

// MarkCommissionPaid transitions a pending commission to paid and publishes
// the resulting domain event. The read (with FOR UPDATE lock), mutation,
// update, and event publish are all inside a single transaction to prevent
// TOCTOU races.
func (s *ResellerService) MarkCommissionPaid(ctx context.Context, commissionID string) (*aggregate.Commission, error) {
	var commission *aggregate.Commission
	err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		commission, err = s.commissions.GetCommissionByIDForUpdate(txCtx, commissionID)
		if err != nil {
			return fmt.Errorf("finding commission: %w", err)
		}

		if err := commission.MarkPaid(s.clock.Now()); err != nil {
			return err
		}

		if err := s.commissions.UpdateCommission(txCtx, commission); err != nil {
			return fmt.Errorf("updating commission: %w", err)
		}

		return domainevent.PublishAll(txCtx, s.publisher, commission)
	})
	if err != nil {
		return nil, err
	}

	s.logger.Info("commission marked as paid",
		slog.String("commission_id", commissionID),
		slog.String("reseller_id", commission.ResellerID),
	)

	return commission, nil
}

// GetPendingCommissions returns all pending commissions for a reseller.
func (s *ResellerService) GetPendingCommissions(ctx context.Context, resellerID string) ([]*aggregate.Commission, error) {
	commissions, err := s.commissions.GetPendingCommissions(ctx, resellerID)
	if err != nil {
		return nil, fmt.Errorf("listing pending commissions: %w", err)
	}
	return commissions, nil
}

// ValidateAPIKey hashes the provided plain-text key and looks up the
// corresponding tenant. Returns ErrInvalidAPIKey if no match is found,
// or ErrTenantInactive if the matched tenant is disabled.
func (s *ResellerService) ValidateAPIKey(ctx context.Context, plainKey string) (*aggregate.Tenant, error) {
	keyHash := aggregate.HashAPIKey(plainKey)

	tenant, err := s.tenants.GetTenantByAPIKeyHash(ctx, keyHash)
	if err != nil {
		return nil, ErrInvalidAPIKey
	}

	if !tenant.IsActive {
		return nil, ErrTenantInactive
	}

	return tenant, nil
}

// ListCommissions resolves the reseller account for userID under the active
// shop server-side (never from request input), then returns that reseller's
// own commissions under RunInTx so the RLS GUC is set. The query is scoped to
// the resolved account ID (not just the shop) so two reseller accounts in the
// same shop cannot see each other's commissions.
func (s *ResellerService) ListCommissions(ctx context.Context, userID, tenantID string) ([]*aggregate.Commission, error) {
	// NOTE: s.commissions.ListCommissionsByTenant resolves because the
	// service-local CommissionRepository was extended with it in C4.3.
	var commissions []*aggregate.Commission
	err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		account, err := s.commissions.GetResellerAccountByUserAndTenant(txCtx, userID, tenantID)
		if err != nil {
			return fmt.Errorf("resolving reseller account: %w", err)
		}
		list, err := s.commissions.ListCommissionsByTenant(txCtx, tenantID, account.ID)
		if err != nil {
			return fmt.Errorf("listing commissions: %w", err)
		}
		commissions = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return commissions, nil
}

// ListCustomers returns the active shop's customers (platform_users scoped to
// the tenant) under RunInTx.
func (s *ResellerService) ListCustomers(ctx context.Context, tenantID string, limit, offset int) ([]*CustomerSummary, error) {
	var customers []*CustomerSummary
	err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		list, err := s.customers.ListCustomersByTenant(txCtx, tenantID, limit, offset)
		if err != nil {
			return fmt.Errorf("listing customers: %w", err)
		}
		customers = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return customers, nil
}

// DashboardSummary builds the purpose-built tenant-scoped reseller dashboard
// aggregate (active customers, active subscriptions, pending commission) under
// RunInTx. It does NOT read the platform-only /api/admin/stats aggregates.
func (s *ResellerService) DashboardSummary(ctx context.Context, tenantID string) (DashboardSummary, error) {
	var ds DashboardSummary
	err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		customers, err := s.customers.CountActiveCustomersByTenant(txCtx, tenantID)
		if err != nil {
			return fmt.Errorf("counting customers: %w", err)
		}
		subs, err := s.customers.CountActiveSubscriptionsByTenant(txCtx, tenantID)
		if err != nil {
			return fmt.Errorf("counting subscriptions: %w", err)
		}
		pending, currency, err := s.customers.SumPendingCommissionByTenant(txCtx, tenantID)
		if err != nil {
			return fmt.Errorf("summing pending commission: %w", err)
		}
		ds = DashboardSummary{
			ActiveCustomers:     customers,
			ActiveSubscriptions: subs,
			PendingCommission:   pending,
			Currency:            currency,
		}
		return nil
	})
	if err != nil {
		return DashboardSummary{}, err
	}
	return ds, nil
}

// SetShopBot validates and upserts the Telegram bot configuration for tenantID.
// It rejects tokens that do not match the Telegram format (<id>:<suffix>) and
// cabinet URLs that are not HTTPS. A cryptographically random webhook_secret is
// generated on every call (rotated on update). Token liveness (getMe) is
// deferred to SP3 to keep SP1 network-free.
func (s *ResellerService) SetShopBot(ctx context.Context, tenantID string, in SetShopBotInput) error {
	if !botTokenRe.MatchString(in.BotToken) {
		return fmt.Errorf("set shop bot: %w", ErrShopBotInvalidToken)
	}
	if !strings.HasPrefix(in.CabinetURL, cabinetURLPrefix) {
		return fmt.Errorf("set shop bot: %w", ErrShopBotInvalidCabinetURL)
	}

	if in.BotPlugin != "" {
		ok, err := s.botPlugins.IsValidBotPlugin(ctx, in.BotPlugin)
		if err != nil {
			return fmt.Errorf("set shop bot: validate plugin: %w", err)
		}
		if !ok {
			return fmt.Errorf("set shop bot: %w", ErrShopBotInvalidPlugin)
		}
	}

	webhookSecret, err := generateWebhookSecret()
	if err != nil {
		return fmt.Errorf("set shop bot: %w", err)
	}

	cfg := vo.ShopBotConfig{
		Token:         secret.NewString(in.BotToken),
		WebhookSecret: webhookSecret,
		CabinetURL:    in.CabinetURL,
		Enabled:       in.Enabled,
		BotPluginSlug: in.BotPlugin,
	}
	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		return s.shopBots.Upsert(txCtx, tenantID, cfg)
	}); err != nil {
		return fmt.Errorf("set shop bot: %w", err)
	}
	return nil
}

// GetShopBot retrieves the Telegram bot configuration for tenantID.
// The adapter decrypts the stored token; callers receive plaintext in
// cfg.Token.Expose(). Token redaction (for API responses) is the handler's
// responsibility, not this method's.
func (s *ResellerService) GetShopBot(ctx context.Context, tenantID string) (*vo.ShopBotConfig, error) {
	var cfg *vo.ShopBotConfig
	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		cfg, err = s.shopBots.GetByTenant(txCtx, tenantID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("get shop bot: %w", err)
	}
	return cfg, nil
}

// ListEnabledShopBots returns all enabled shop bot configurations across every
// tenant. The call runs under platform-sentinel scope so the RLS policy allows
// cross-tenant visibility; this is safe because the method is only invoked by
// the internal bot manager on startup, never from a shop-scoped request path.
func (s *ResellerService) ListEnabledShopBots(ctx context.Context) ([]vo.ShopBotWithTenant, error) {
	pctx := tenantctx.WithPlatformScope(ctx)
	var result []vo.ShopBotWithTenant
	if err := s.txRunner.RunInTx(pctx, func(txCtx context.Context) error {
		var err error
		result, err = s.shopBots.ListEnabled(txCtx)
		return err
	}); err != nil {
		return nil, fmt.Errorf("list enabled shop bots: %w", err)
	}
	return result, nil
}

// generateWebhookSecret returns a 64-character lower-case hex string derived
// from 32 cryptographically random bytes. The result is used as the Telegram
// webhook X-Telegram-Bot-Api-Secret-Token header value.
// webhookSecretBytes is the number of cryptographically random bytes used to
// derive a Telegram webhook secret (hex-encoded to a 64-character token).
const webhookSecretBytes = 32

func generateWebhookSecret() (string, error) {
	b := make([]byte, webhookSecretBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating webhook secret: %w", err)
	}
	return hex.EncodeToString(b), nil
}
