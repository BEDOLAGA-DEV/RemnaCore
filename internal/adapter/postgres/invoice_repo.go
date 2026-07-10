package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

// ---------------------------------------------------------------------------
// InvoiceRepository
// ---------------------------------------------------------------------------

// InvoiceRepository implements billing.InvoiceRepository backed by PostgreSQL.
type InvoiceRepository struct {
	pool *pgxpool.Pool
}

// NewInvoiceRepository returns a new InvoiceRepository using the given pool.
func NewInvoiceRepository(pool *pgxpool.Pool) *InvoiceRepository {
	return &InvoiceRepository{pool: pool}
}

// q returns a *gen.Queries backed by the active transaction (if any) or the
// pool. This ensures all methods transparently participate in RunInTx and
// respect RLS tenant scoping.
func (r *InvoiceRepository) q(ctx context.Context) *gen.Queries {
	return gen.New(DBFromContext(ctx, r.pool))
}

// invFields holds the common columns returned by all invoice queries.
type invFields struct {
	ID                  pgtype.UUID
	SubscriptionID      pgtype.UUID
	UserID              pgtype.UUID
	SubtotalAmount      int64
	TotalDiscountAmount int64
	TotalAmount         int64
	Currency            string
	PricingReason       string
	Discounts           []byte
	Status              string
	PaidAt              pgtype.Timestamptz
	CreatedAt           pgtype.Timestamptz
	UpdatedAt           pgtype.Timestamptz
}

// invRow is a constraint matching all sqlc-generated invoice row types.
type invRow interface {
	gen.GetInvoiceByIDRow | gen.GetInvoiceByIDForUpdateRow | gen.GetInvoicesBySubscriptionIDRow | gen.GetPendingInvoicesByUserIDRow | gen.GetAllInvoicesRow
}

func extractInvFields[T invRow](row T) invFields {
	switch r := any(row).(type) {
	case gen.GetInvoiceByIDRow:
		return invFields{r.ID, r.SubscriptionID, r.UserID, r.SubtotalAmount, r.TotalDiscountAmount, r.TotalAmount, r.Currency, r.PricingReason, r.Discounts, r.Status, r.PaidAt, r.CreatedAt, r.UpdatedAt}
	case gen.GetInvoiceByIDForUpdateRow:
		return invFields{r.ID, r.SubscriptionID, r.UserID, r.SubtotalAmount, r.TotalDiscountAmount, r.TotalAmount, r.Currency, r.PricingReason, r.Discounts, r.Status, r.PaidAt, r.CreatedAt, r.UpdatedAt}
	case gen.GetInvoicesBySubscriptionIDRow:
		return invFields{r.ID, r.SubscriptionID, r.UserID, r.SubtotalAmount, r.TotalDiscountAmount, r.TotalAmount, r.Currency, r.PricingReason, r.Discounts, r.Status, r.PaidAt, r.CreatedAt, r.UpdatedAt}
	case gen.GetPendingInvoicesByUserIDRow:
		return invFields{r.ID, r.SubscriptionID, r.UserID, r.SubtotalAmount, r.TotalDiscountAmount, r.TotalAmount, r.Currency, r.PricingReason, r.Discounts, r.Status, r.PaidAt, r.CreatedAt, r.UpdatedAt}
	case gen.GetAllInvoicesRow:
		return invFields{r.ID, r.SubscriptionID, r.UserID, r.SubtotalAmount, r.TotalDiscountAmount, r.TotalAmount, r.Currency, r.PricingReason, r.Discounts, r.Status, r.PaidAt, r.CreatedAt, r.UpdatedAt}
	default:
		panic("unreachable: unhandled invRow type")
	}
}

func invoiceRowToDomain[T invRow](row T) (*aggregate.Invoice, error) {
	return invFieldsToDomain(extractInvFields(row))
}

// invFieldsToDomain converts a raw invFields struct to a domain Invoice. Used
// by both the generic invoiceRowToDomain and raw pgx FOR UPDATE queries.
func invFieldsToDomain(f invFields) (*aggregate.Invoice, error) {
	var discounts []vo.Discount
	if len(f.Discounts) > 0 {
		if err := json.Unmarshal(f.Discounts, &discounts); err != nil {
			return nil, fmt.Errorf("unmarshal discounts: %w", err)
		}
	}
	return &aggregate.Invoice{
		ID:             pgutil.PgtypeToUUID(f.ID),
		SubscriptionID: pgutil.PgtypeToUUID(f.SubscriptionID),
		UserID:         pgutil.PgtypeToUUID(f.UserID),
		Discounts:      discounts,
		Subtotal:       vo.NewMoney(f.SubtotalAmount, vo.Currency(f.Currency)),
		TotalDiscount:  vo.NewMoney(f.TotalDiscountAmount, vo.Currency(f.Currency)),
		Total:          vo.NewMoney(f.TotalAmount, vo.Currency(f.Currency)),
		PricingReason:  f.PricingReason,
		Status:         aggregate.InvoiceStatus(f.Status),
		PaidAt:         pgutil.PgtypeToOptTime(f.PaidAt),
		CreatedAt:      pgutil.PgtypeToTime(f.CreatedAt),
		UpdatedAt:      pgutil.PgtypeToTime(f.UpdatedAt),
	}, nil
}

// lineItemFields holds the columns shared by every line-item row shape.
type lineItemFields struct {
	Description string
	ItemType    string
	Amount      int64
	Currency    string
	Quantity    int32
}

// lineItemRow constrains the sqlc-generated line-item row types. The
// explicit-column SELECTs each emit a distinct *Row struct (the model carries
// the extra tenant_id column added in migration 042), so the converter is
// generic over them.
type lineItemRow interface {
	gen.GetLineItemsByInvoiceIDRow | gen.GetLineItemsByInvoiceIDsRow
}

func extractLineItemFields[T lineItemRow](row T) lineItemFields {
	switch r := any(row).(type) {
	case gen.GetLineItemsByInvoiceIDRow:
		return lineItemFields{r.Description, r.ItemType, r.Amount, r.Currency, r.Quantity}
	case gen.GetLineItemsByInvoiceIDsRow:
		return lineItemFields{r.Description, r.ItemType, r.Amount, r.Currency, r.Quantity}
	default:
		panic("unreachable: unhandled lineItemRow type")
	}
}

func lineItemRowToDomain[T lineItemRow](row T) vo.LineItem {
	f := extractLineItemFields(row)
	return vo.LineItem{
		Description: f.Description,
		Type:        vo.LineItemType(f.ItemType),
		Amount:      vo.NewMoney(f.Amount, vo.Currency(f.Currency)),
		Quantity:    int(f.Quantity),
	}
}

func (r *InvoiceRepository) loadLineItems(ctx context.Context, inv *aggregate.Invoice) error {
	rows, err := r.q(ctx).GetLineItemsByInvoiceID(ctx, pgutil.UUIDToPgtype(inv.ID))
	if err != nil {
		return pgutil.MapErr(err, "get line items for invoice", billing.ErrInvoiceNotFound)
	}
	items := make([]vo.LineItem, len(rows))
	for i, row := range rows {
		items[i] = lineItemRowToDomain(row)
	}
	inv.LineItems = items
	return nil
}

// batchLoadLineItems loads line items for multiple invoices in a single query,
// eliminating the N+1 problem from per-invoice loadLineItems calls.
func (r *InvoiceRepository) batchLoadLineItems(ctx context.Context, invoices []*aggregate.Invoice) error {
	if len(invoices) == 0 {
		return nil
	}
	ids := make([]pgtype.UUID, len(invoices))
	for i, inv := range invoices {
		ids[i] = pgutil.UUIDToPgtype(inv.ID)
	}
	rows, err := r.q(ctx).GetLineItemsByInvoiceIDs(ctx, ids)
	if err != nil {
		return fmt.Errorf("batch load line items: %w", err)
	}
	// Group by invoice_id.
	byInvoice := make(map[string][]vo.LineItem, len(invoices))
	for _, row := range rows {
		iid := pgutil.PgtypeToUUID(row.InvoiceID)
		byInvoice[iid] = append(byInvoice[iid], lineItemRowToDomain(row))
	}
	for _, inv := range invoices {
		inv.LineItems = byInvoice[inv.ID]
	}
	return nil
}

func (r *InvoiceRepository) GetByID(ctx context.Context, id string) (*aggregate.Invoice, error) {
	row, err := r.q(ctx).GetInvoiceByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get invoice by id", billing.ErrInvoiceNotFound)
	}
	inv, err := invoiceRowToDomain(row)
	if err != nil {
		return nil, fmt.Errorf("get invoice by id: %w", err)
	}
	if err := r.loadLineItems(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

func (r *InvoiceRepository) GetByIDForUpdate(ctx context.Context, id string) (*aggregate.Invoice, error) {
	row, err := r.q(ctx).GetInvoiceByIDForUpdate(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get invoice by id for update", billing.ErrInvoiceNotFound)
	}
	inv, err := invoiceRowToDomain(row)
	if err != nil {
		return nil, fmt.Errorf("get invoice by id for update: %w", err)
	}
	if err := r.loadLineItems(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

func (r *InvoiceRepository) GetBySubscriptionID(ctx context.Context, subID string) ([]*aggregate.Invoice, error) {
	rows, err := r.q(ctx).GetInvoicesBySubscriptionID(ctx, pgutil.UUIDToPgtype(subID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get invoices by subscription id", billing.ErrInvoiceNotFound)
	}
	invoices := make([]*aggregate.Invoice, len(rows))
	for i, row := range rows {
		inv, err := invoiceRowToDomain(row)
		if err != nil {
			return nil, fmt.Errorf("get invoices by subscription id: %w", err)
		}
		invoices[i] = inv
		if err := r.loadLineItems(ctx, invoices[i]); err != nil {
			return nil, err
		}
	}
	return invoices, nil
}

func (r *InvoiceRepository) GetPendingByUserID(ctx context.Context, userID string) ([]*aggregate.Invoice, error) {
	rows, err := r.q(ctx).GetPendingInvoicesByUserID(ctx, pgutil.UUIDToPgtype(userID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get pending invoices by user id", billing.ErrInvoiceNotFound)
	}
	invoices := make([]*aggregate.Invoice, len(rows))
	for i, row := range rows {
		inv, err := invoiceRowToDomain(row)
		if err != nil {
			return nil, fmt.Errorf("get pending invoices by user id: %w", err)
		}
		invoices[i] = inv
		if err := r.loadLineItems(ctx, invoices[i]); err != nil {
			return nil, err
		}
	}
	return invoices, nil
}

func (r *InvoiceRepository) GetAll(ctx context.Context, limit, offset int) ([]*aggregate.Invoice, error) {
	rows, err := r.q(ctx).GetAllInvoices(ctx, gen.GetAllInvoicesParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, pgutil.MapErr(err, "get all invoices", billing.ErrInvoiceNotFound)
	}
	invoices := make([]*aggregate.Invoice, len(rows))
	for i, row := range rows {
		inv, err := invoiceRowToDomain(row)
		if err != nil {
			return nil, fmt.Errorf("get all invoices: %w", err)
		}
		invoices[i] = inv
	}
	if err := r.batchLoadLineItems(ctx, invoices); err != nil {
		return nil, err
	}
	return invoices, nil
}

func (r *InvoiceRepository) Create(ctx context.Context, inv *aggregate.Invoice) error {
	discountsJSON, err := json.Marshal(inv.Discounts)
	if err != nil {
		return fmt.Errorf("marshal invoice discounts: %w", err)
	}
	err = r.q(ctx).CreateInvoice(ctx, gen.CreateInvoiceParams{
		ID:                  pgutil.UUIDToPgtype(inv.ID),
		SubscriptionID:      pgutil.UUIDToPgtype(inv.SubscriptionID),
		UserID:              pgutil.UUIDToPgtype(inv.UserID),
		SubtotalAmount:      inv.Subtotal.Amount,
		TotalDiscountAmount: inv.TotalDiscount.Amount,
		TotalAmount:         inv.Total.Amount,
		Currency:            string(inv.Total.Currency),
		PricingReason:       inv.PricingReason,
		Discounts:           discountsJSON,
		Status:              string(inv.Status),
		PaidAt:              pgutil.OptTimeToPgtype(inv.PaidAt),
		CreatedAt:           pgutil.TimeToPgtype(inv.CreatedAt),
		UpdatedAt:           pgutil.TimeToPgtype(inv.UpdatedAt),
	})
	if err != nil {
		if pgutil.IsUniqueViolation(err) {
			return fmt.Errorf("create invoice: %w", billing.ErrInvoiceAlreadyExists)
		}
		return fmt.Errorf("create invoice: %w", err)
	}

	for _, item := range inv.LineItems {
		if err := r.createLineItem(ctx, inv.ID, item); err != nil {
			return fmt.Errorf("create line item %q for invoice: %w", item.Description, err)
		}
	}
	return nil
}

func (r *InvoiceRepository) createLineItem(ctx context.Context, invoiceID string, item vo.LineItem) error {
	err := r.q(ctx).CreateInvoiceLineItem(ctx, gen.CreateInvoiceLineItemParams{
		InvoiceID:   pgutil.UUIDToPgtype(invoiceID),
		Description: item.Description,
		ItemType:    string(item.Type),
		Amount:      item.Amount.Amount,
		Currency:    string(item.Amount.Currency),
		Quantity:    int32(item.Quantity),
	})
	if err != nil {
		if pgutil.IsUniqueViolation(err) {
			return fmt.Errorf("create invoice line item: %w", billing.ErrInvoiceAlreadyExists)
		}
		return fmt.Errorf("create invoice line item: %w", err)
	}
	return nil
}

func (r *InvoiceRepository) Update(ctx context.Context, inv *aggregate.Invoice) error {
	discountsJSON, err := json.Marshal(inv.Discounts)
	if err != nil {
		return fmt.Errorf("marshal invoice discounts: %w", err)
	}
	err = r.q(ctx).UpdateInvoice(ctx, gen.UpdateInvoiceParams{
		ID:                  pgutil.UUIDToPgtype(inv.ID),
		Status:              string(inv.Status),
		PaidAt:              pgutil.OptTimeToPgtype(inv.PaidAt),
		SubtotalAmount:      inv.Subtotal.Amount,
		TotalDiscountAmount: inv.TotalDiscount.Amount,
		TotalAmount:         inv.Total.Amount,
		PricingReason:       inv.PricingReason,
		Discounts:           discountsJSON,
	})
	return pgutil.MapErr(err, "update invoice", billing.ErrInvoiceNotFound)
}

// getAllInvoicesCursorFirstPageSQL returns the first page of invoices
// ordered by (created_at DESC, id DESC). Used when no cursor is provided.
const getAllInvoicesCursorFirstPageSQL = `
SELECT id, subscription_id, user_id, subtotal_amount, total_discount_amount,
       total_amount, currency, pricing_reason, discounts, status, paid_at, created_at, updated_at
FROM billing.invoices
ORDER BY created_at DESC, id DESC
LIMIT $1`

// getAllInvoicesCursorNextPageSQL returns subsequent pages using cursor-based
// pagination keyed on (created_at DESC, id DESC). Bypasses sqlc because the
// conditional WHERE clause for optional cursor parameters is not supported.
const getAllInvoicesCursorNextPageSQL = `
SELECT id, subscription_id, user_id, subtotal_amount, total_discount_amount,
       total_amount, currency, pricing_reason, discounts, status, paid_at, created_at, updated_at
FROM billing.invoices
WHERE (created_at, id) < ($2, $3)
ORDER BY created_at DESC, id DESC
LIMIT $1`

// GetAllCursor returns invoices using cursor-based pagination keyed on
// (created_at DESC, id DESC). Nil cursor returns the first page.
func (r *InvoiceRepository) GetAllCursor(ctx context.Context, params billing.CursorParams) ([]*aggregate.Invoice, error) {
	db := DBFromContext(ctx, r.pool)

	var invoices []*aggregate.Invoice
	if params.CreatedAt == nil || params.ID == nil {
		rows, err := db.Query(ctx, getAllInvoicesCursorFirstPageSQL, int32(params.Limit))
		if err != nil {
			return nil, pgutil.MapErr(err, "get all invoices cursor first page", billing.ErrInvoiceNotFound)
		}
		defer rows.Close()
		invoices, err = scanInvoiceRows(rows)
		if err != nil {
			return nil, err
		}
	} else {
		rows, err := db.Query(ctx, getAllInvoicesCursorNextPageSQL,
			int32(params.Limit),
			pgutil.TimeToPgtype(*params.CreatedAt),
			pgutil.UUIDToPgtype(*params.ID),
		)
		if err != nil {
			return nil, pgutil.MapErr(err, "get all invoices cursor next page", billing.ErrInvoiceNotFound)
		}
		defer rows.Close()
		invoices, err = scanInvoiceRows(rows)
		if err != nil {
			return nil, err
		}
	}
	if err := r.batchLoadLineItems(ctx, invoices); err != nil {
		return nil, err
	}
	return invoices, nil
}

// scanInvoiceRows scans pgx rows into a slice of domain invoices.
// Shared by GetAllCursor to avoid duplicating the scan loop.
func scanInvoiceRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
},
) ([]*aggregate.Invoice, error) {
	var invoices []*aggregate.Invoice
	for rows.Next() {
		var f invFields
		if err := rows.Scan(
			&f.ID, &f.SubscriptionID, &f.UserID, &f.SubtotalAmount, &f.TotalDiscountAmount,
			&f.TotalAmount, &f.Currency, &f.PricingReason, &f.Discounts, &f.Status,
			&f.PaidAt, &f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, pgutil.MapErr(err, "scan invoice cursor row", billing.ErrInvoiceNotFound)
		}
		inv, err := invFieldsToDomain(f)
		if err != nil {
			return nil, fmt.Errorf("scan invoice cursor row: %w", err)
		}
		invoices = append(invoices, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, pgutil.MapErr(err, "iterate invoice cursor rows", billing.ErrInvoiceNotFound)
	}
	return invoices, nil
}

var _ billing.InvoiceRepository = (*InvoiceRepository)(nil)
