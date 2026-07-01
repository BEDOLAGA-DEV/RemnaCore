package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/secretbox"
)

// ShopBotRepository implements reseller.ShopBotRepository backed by PostgreSQL
// via sqlc-generated queries. Bot tokens are encrypted with AES-256-GCM on
// write and decrypted on read; the box may be nil if SECURITY_ENCRYPTION_KEY is
// unset, in which case every method returns ErrEncryptionNotConfigured.
//
// All queries resolve the underlying connection via DBFromContext so they
// participate in any active transaction and respect the app.tenant_id GUC that
// enforces row-level security.
type ShopBotRepository struct {
	pool   *pgxpool.Pool
	box    *secretbox.Box
	logger *slog.Logger
}

// NewShopBotRepository returns a ShopBotRepository. box may be nil; each method
// that needs to seal or open will return ErrEncryptionNotConfigured rather than
// panicking.
func NewShopBotRepository(pool *pgxpool.Pool, box *secretbox.Box, logger *slog.Logger) *ShopBotRepository {
	return &ShopBotRepository{pool: pool, box: box, logger: logger}
}

// q returns the sqlc Queries bound to the transaction from ctx (if any),
// otherwise falls back to the pool so RLS session variables are visible.
func (r *ShopBotRepository) q(ctx context.Context) *gen.Queries {
	return gen.New(DBFromContext(ctx, r.pool))
}

// Upsert encrypts cfg.Token and writes (or updates) the shop bot row for
// tenantID. Returns ErrEncryptionNotConfigured when the box is nil.
func (r *ShopBotRepository) Upsert(ctx context.Context, tenantID string, cfg vo.ShopBotConfig) error {
	if r.box == nil {
		return fmt.Errorf("upsert shop bot: %w", reseller.ErrEncryptionNotConfigured)
	}
	enc, err := r.box.Seal(cfg.Token.Expose())
	if err != nil {
		return fmt.Errorf("encrypt bot token: %w", err)
	}
	return r.q(ctx).UpsertShopBot(ctx, gen.UpsertShopBotParams{
		TenantID:      pgutil.UUIDToPgtype(tenantID),
		BotTokenEnc:   enc,
		WebhookSecret: cfg.WebhookSecret,
		CabinetUrl:    cfg.CabinetURL,
		BotUsername:   pgutil.StrPtrOrNil(cfg.BotUsername),
		Enabled:       cfg.Enabled,
		BotPluginSlug: pgutil.StrPtrOrNil(cfg.BotPluginSlug),
	})
}

// GetByTenant retrieves and decrypts the bot config for tenantID.
// Returns ErrEncryptionNotConfigured when the box is nil.
// Returns ErrShopBotNotFound when no row exists or RLS blocks the read.
func (r *ShopBotRepository) GetByTenant(ctx context.Context, tenantID string) (*vo.ShopBotConfig, error) {
	if r.box == nil {
		return nil, fmt.Errorf("get shop bot: %w", reseller.ErrEncryptionNotConfigured)
	}
	row, err := r.q(ctx).GetShopBotByTenant(ctx, pgutil.UUIDToPgtype(tenantID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get shop bot by tenant", reseller.ErrShopBotNotFound)
	}
	plain, err := r.box.Open(row.BotTokenEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt bot token: %w", err)
	}
	return &vo.ShopBotConfig{
		Token:         config.NewSecretString(plain),
		WebhookSecret: row.WebhookSecret,
		CabinetURL:    row.CabinetUrl,
		BotUsername:   pgutil.DerefStr(row.BotUsername),
		Enabled:       row.Enabled,
		BotPluginSlug: pgutil.DerefStr(row.BotPluginSlug),
	}, nil
}

// ListEnabled returns all enabled bot configs, decrypting each token.
// Intended to run under the platform sentinel GUC so the bot manager can
// discover every active shop bot on startup.
// Returns ErrEncryptionNotConfigured when the box is nil.
func (r *ShopBotRepository) ListEnabled(ctx context.Context) ([]vo.ShopBotWithTenant, error) {
	if r.box == nil {
		return nil, fmt.Errorf("list enabled shop bots: %w", reseller.ErrEncryptionNotConfigured)
	}
	rows, err := r.q(ctx).ListEnabledShopBots(ctx)
	if err != nil {
		return nil, fmt.Errorf("list enabled shop bots: %w", err)
	}
	out := make([]vo.ShopBotWithTenant, 0, len(rows))
	for _, row := range rows {
		tenantID := pgutil.PgtypeToUUID(row.TenantID)
		plain, openErr := r.box.Open(row.BotTokenEnc)
		if openErr != nil {
			// One shop's token can fail to decrypt (key rotation mismatch,
			// corruption) without taking down every other shop bot: skip and
			// log this row, and continue building the set. The token value is
			// never logged.
			r.logger.Warn("skipping shop bot with undecryptable token",
				slog.String("tenant", tenantID),
				slog.Any("error", openErr),
			)
			continue
		}
		out = append(out, vo.ShopBotWithTenant{
			TenantID: tenantID,
			Config: vo.ShopBotConfig{
				Token:         config.NewSecretString(plain),
				WebhookSecret: row.WebhookSecret,
				CabinetURL:    row.CabinetUrl,
				BotUsername:   pgutil.DerefStr(row.BotUsername),
				Enabled:       row.Enabled,
				BotPluginSlug: pgutil.DerefStr(row.BotPluginSlug),
			},
		})
	}
	return out, nil
}

// compile-time interface check
var _ reseller.ShopBotRepository = (*ShopBotRepository)(nil)
