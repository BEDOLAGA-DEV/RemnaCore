package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgconn"
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
	// Deterministic keyed hash for the cross-tenant uniqueness guard (the random
	// nonce in enc makes ciphertext-level dedup impossible). A duplicate hash
	// trips the partial UNIQUE index idx_shop_bots_token_hash (migration 048).
	tokenHash := r.box.HMAC(cfg.Token.Expose())

	// Raw pgx (not sqlc) so we can carry bot_token_hash and map the token-hash
	// unique violation to a domain error. ON CONFLICT (tenant_id) handles the
	// same-tenant update; a DIFFERENT tenant with the same token collides on the
	// token-hash index instead.
	_, err = DBFromContext(ctx, r.pool).Exec(ctx,
		`INSERT INTO reseller.shop_bots
		    (tenant_id, bot_token_enc, bot_token_hash, webhook_secret, cabinet_url, bot_username, enabled, bot_plugin_slug)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (tenant_id) DO UPDATE SET
		    bot_token_enc   = EXCLUDED.bot_token_enc,
		    bot_token_hash  = EXCLUDED.bot_token_hash,
		    webhook_secret  = EXCLUDED.webhook_secret,
		    cabinet_url     = EXCLUDED.cabinet_url,
		    bot_username    = EXCLUDED.bot_username,
		    enabled         = EXCLUDED.enabled,
		    bot_plugin_slug = EXCLUDED.bot_plugin_slug`,
		pgutil.UUIDToPgtype(tenantID), enc, tokenHash, cfg.WebhookSecret,
		cfg.CabinetURL, pgutil.StrPtrOrNil(cfg.BotUsername), cfg.Enabled, pgutil.StrPtrOrNil(cfg.BotPluginSlug),
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idx_shop_bots_token_hash" {
			return fmt.Errorf("upsert shop bot: %w", reseller.ErrShopBotTokenInUse)
		}
		return fmt.Errorf("upsert shop bot: %w", err)
	}
	return nil
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
