package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

// TelegramNonceRepository backs service.TelegramReplayGuard: it records consumed
// Telegram initData hashes so a signed payload cannot be replayed within its
// validity window. The table (identity.telegram_initdata_nonces) is not RLS-
// scoped — the nonce is a content hash carrying no tenant data.
type TelegramNonceRepository struct {
	pool *pgxpool.Pool
}

// NewTelegramNonceRepository returns a nonce repository bound to pool.
func NewTelegramNonceRepository(pool *pgxpool.Pool) *TelegramNonceRepository {
	return &TelegramNonceRepository{pool: pool}
}

// Consume atomically claims nonce. It returns fresh=true the first time a nonce
// is seen and fresh=false on every subsequent call for the same nonce (until the
// row is pruned after expiresAt). Expired rows sharing the nonce are cleared
// first so a nonce becomes reusable only once its window has fully passed.
func (r *TelegramNonceRepository) Consume(ctx context.Context, nonce string, expiresAt time.Time) (bool, error) {
	db := DBFromContext(ctx, r.pool)

	// Opportunistically prune this nonce's row if it has already expired, so a
	// genuinely new payload that happens to collide post-window is not blocked.
	if _, err := db.Exec(ctx,
		`DELETE FROM identity.telegram_initdata_nonces WHERE nonce = $1 AND expires_at <= now()`,
		nonce,
	); err != nil {
		return false, fmt.Errorf("prune expired nonce: %w", err)
	}

	tag, err := db.Exec(ctx,
		`INSERT INTO identity.telegram_initdata_nonces (nonce, expires_at)
		 VALUES ($1, $2) ON CONFLICT (nonce) DO NOTHING`,
		nonce, pgutil.TimeToPgtype(expiresAt),
	)
	if err != nil {
		return false, fmt.Errorf("insert telegram nonce: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
