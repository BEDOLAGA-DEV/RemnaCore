package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

// PaymentRepository implements payment.PaymentRepository backed by PostgreSQL.
type PaymentRepository struct {
	pool *pgxpool.Pool
}

// NewPaymentRepository returns a new PaymentRepository using the given pool.
func NewPaymentRepository(pool *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{pool: pool}
}

// q returns a *gen.Queries backed by the active transaction (if any) or the
// pool. This ensures all methods transparently participate in RunInTx and
// respect RLS tenant scoping.
func (r *PaymentRepository) q(ctx context.Context) *gen.Queries {
	return gen.New(DBFromContext(ctx, r.pool))
}

func paymentRowToDomain(row gen.PaymentPaymentRecord) *payment.PaymentRecord {
	return &payment.PaymentRecord{
		ID:         pgutil.PgtypeToUUID(row.ID),
		InvoiceID:  pgutil.PgtypeToUUID(row.InvoiceID),
		Provider:   row.Provider,
		ExternalID: row.ExternalID,
		Amount:     row.Amount,
		Currency:   row.Currency,
		Status:     payment.PaymentStatus(row.Status),
		CreatedAt:  pgutil.PgtypeToTime(row.CreatedAt),
		UpdatedAt:  pgutil.PgtypeToTime(row.UpdatedAt),
	}
}

func webhookLogRowToDomain(row gen.PaymentWebhookLog) *payment.WebhookLog {
	return &payment.WebhookLog{
		ID:          pgutil.PgtypeToUUID(row.ID),
		Provider:    row.Provider,
		ExternalID:  row.ExternalID,
		RawBody:     row.RawBody,
		Status:      payment.WebhookStatus(row.Status),
		ProcessedAt: pgutil.PgtypeToOptTime(row.ProcessedAt),
		CreatedAt:   pgutil.PgtypeToTime(row.CreatedAt),
	}
}

func (r *PaymentRepository) CreatePayment(ctx context.Context, record *payment.PaymentRecord) error {
	err := r.q(ctx).CreatePaymentRecord(ctx, gen.CreatePaymentRecordParams{
		ID:         pgutil.UUIDToPgtype(record.ID),
		InvoiceID:  pgutil.UUIDToPgtype(record.InvoiceID),
		Provider:   record.Provider,
		ExternalID: record.ExternalID,
		Amount:     record.Amount,
		Currency:   record.Currency,
		Status:     string(record.Status),
		CreatedAt:  pgutil.TimeToPgtype(record.CreatedAt),
		UpdatedAt:  pgutil.TimeToPgtype(record.UpdatedAt),
	})
	return pgutil.MapErr(err, "create payment record", payment.ErrPaymentNotFound)
}

func (r *PaymentRepository) GetPaymentByID(ctx context.Context, id string) (*payment.PaymentRecord, error) {
	row, err := r.q(ctx).GetPaymentRecordByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get payment by id", payment.ErrPaymentNotFound)
	}
	return paymentRowToDomain(row), nil
}

// getPaymentRecordByIDForUpdateSQL is identical to GetPaymentRecordByID but
// acquires a FOR UPDATE row lock. Must be called within a transaction.
const getPaymentRecordByIDForUpdateSQL = `
SELECT id, invoice_id, provider, external_id, amount, currency, status, created_at, updated_at
FROM payment.payment_records WHERE id = $1 FOR UPDATE
`

func (r *PaymentRepository) GetPaymentByIDForUpdate(ctx context.Context, id string) (*payment.PaymentRecord, error) {
	db := DBFromContext(ctx, r.pool)
	row := db.QueryRow(ctx, getPaymentRecordByIDForUpdateSQL, pgutil.UUIDToPgtype(id))

	var rawRow gen.PaymentPaymentRecord
	err := row.Scan(
		&rawRow.ID, &rawRow.InvoiceID, &rawRow.Provider, &rawRow.ExternalID,
		&rawRow.Amount, &rawRow.Currency, &rawRow.Status, &rawRow.CreatedAt, &rawRow.UpdatedAt,
	)
	if err != nil {
		return nil, pgutil.MapErr(err, "get payment by id for update", payment.ErrPaymentNotFound)
	}
	return paymentRowToDomain(rawRow), nil
}

func (r *PaymentRepository) GetPaymentByExternalID(ctx context.Context, provider, externalID string) (*payment.PaymentRecord, error) {
	row, err := r.q(ctx).GetPaymentRecordByExternalID(ctx, gen.GetPaymentRecordByExternalIDParams{
		Provider:   provider,
		ExternalID: externalID,
	})
	if err != nil {
		return nil, pgutil.MapErr(err, "get payment by external id", payment.ErrPaymentNotFound)
	}
	return paymentRowToDomain(row), nil
}

// getPaymentRecordByExternalIDForUpdateSQL is identical to
// GetPaymentRecordByExternalID but acquires a FOR UPDATE row lock.
const getPaymentRecordByExternalIDForUpdateSQL = `
SELECT id, invoice_id, provider, external_id, amount, currency, status, created_at, updated_at
FROM payment.payment_records WHERE provider = $1 AND external_id = $2 FOR UPDATE
`

func (r *PaymentRepository) GetPaymentByExternalIDForUpdate(ctx context.Context, provider, externalID string) (*payment.PaymentRecord, error) {
	db := DBFromContext(ctx, r.pool)
	row := db.QueryRow(ctx, getPaymentRecordByExternalIDForUpdateSQL, provider, externalID)

	var rawRow gen.PaymentPaymentRecord
	err := row.Scan(
		&rawRow.ID, &rawRow.InvoiceID, &rawRow.Provider, &rawRow.ExternalID,
		&rawRow.Amount, &rawRow.Currency, &rawRow.Status, &rawRow.CreatedAt, &rawRow.UpdatedAt,
	)
	if err != nil {
		return nil, pgutil.MapErr(err, "get payment by external id for update", payment.ErrPaymentNotFound)
	}
	return paymentRowToDomain(rawRow), nil
}

func (r *PaymentRepository) UpdatePayment(ctx context.Context, record *payment.PaymentRecord) error {
	err := r.q(ctx).UpdatePaymentRecord(ctx, gen.UpdatePaymentRecordParams{
		ID:        pgutil.UUIDToPgtype(record.ID),
		Status:    string(record.Status),
		UpdatedAt: pgutil.TimeToPgtype(record.UpdatedAt),
	})
	return pgutil.MapErr(err, "update payment record", payment.ErrPaymentNotFound)
}

func (r *PaymentRepository) CreateWebhookLog(ctx context.Context, log *payment.WebhookLog) error {
	err := r.q(ctx).CreateWebhookLog(ctx, gen.CreateWebhookLogParams{
		ID:          pgutil.UUIDToPgtype(log.ID),
		Provider:    log.Provider,
		ExternalID:  log.ExternalID,
		RawBody:     log.RawBody,
		Status:      string(log.Status),
		ProcessedAt: pgutil.OptTimeToPgtype(log.ProcessedAt),
		CreatedAt:   pgutil.TimeToPgtype(log.CreatedAt),
	})
	if err != nil {
		// Detect unique constraint violation for idempotency.
		if isUniqueViolation(err) {
			return payment.ErrWebhookDuplicate
		}
		return pgutil.MapErr(err, "create webhook log", payment.ErrWebhookNotFound)
	}
	return nil
}

func (r *PaymentRepository) GetWebhookLog(ctx context.Context, provider, externalID string) (*payment.WebhookLog, error) {
	row, err := r.q(ctx).GetWebhookLogByProviderExternalID(ctx, gen.GetWebhookLogByProviderExternalIDParams{
		Provider:   provider,
		ExternalID: externalID,
	})
	if err != nil {
		return nil, pgutil.MapErr(err, "get webhook log", payment.ErrWebhookNotFound)
	}
	return webhookLogRowToDomain(row), nil
}

func (r *PaymentRepository) UpdateWebhookLog(ctx context.Context, log *payment.WebhookLog) error {
	err := r.q(ctx).UpdateWebhookLog(ctx, gen.UpdateWebhookLogParams{
		ID:          pgutil.UUIDToPgtype(log.ID),
		Status:      string(log.Status),
		ProcessedAt: pgutil.OptTimeToPgtype(log.ProcessedAt),
	})
	return pgutil.MapErr(err, "update webhook log", payment.ErrWebhookNotFound)
}

// pgUniqueViolationCode is the PostgreSQL error code for unique constraint violations.
const pgUniqueViolationCode = "23505"

// isUniqueViolation checks if a PostgreSQL error is a unique constraint violation
// using proper pgconn.PgError type assertion.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgUniqueViolationCode
	}
	return false
}

var _ payment.PaymentRepository = (*PaymentRepository)(nil)
