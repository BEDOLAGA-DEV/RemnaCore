package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

// ---------------------------------------------------------------------------
// PlanRepository
// ---------------------------------------------------------------------------

// PlanRepository implements billing.PlanRepository backed by PostgreSQL.
type PlanRepository struct {
	pool    *pgxpool.Pool
	queries *gen.Queries
}

// NewPlanRepository returns a new PlanRepository using the given pool.
func NewPlanRepository(pool *pgxpool.Pool) *PlanRepository {
	return &PlanRepository{
		pool:    pool,
		queries: gen.New(pool),
	}
}

func planRowToDomain(row gen.BillingPlan) *aggregate.Plan {
	return &aggregate.Plan{
		ID:                   pgutil.PgtypeToUUID(row.ID),
		Name:                 row.Name,
		Description:          pgutil.DerefStr(row.Description),
		BasePrice:            vo.NewMoney(row.BasePriceAmount, vo.Currency(row.BasePriceCurrency)),
		Interval:             vo.BillingInterval(row.BillingInterval),
		TrafficLimitBytes:    row.TrafficLimitBytes,
		DeviceLimit:          int(row.DeviceLimit),
		AllowedCountries:     row.AllowedCountries,
		AllowedProtocols:     row.AllowedProtocols,
		Tier:                 aggregate.PlanTier(row.Tier),
		MaxRemnawaveBindings: int(row.MaxRemnawaveBindings),
		FamilyEnabled:        row.FamilyEnabled,
		MaxFamilyMembers:     int(row.MaxFamilyMembers),
		IsActive:             row.IsActive,
		CreatedAt:            pgutil.PgtypeToTime(row.CreatedAt),
		UpdatedAt:            pgutil.PgtypeToTime(row.UpdatedAt),
	}
}

func addonRowToDomain(row gen.BillingPlanAddon) aggregate.Addon {
	return aggregate.Addon{
		ID:                pgutil.PgtypeToUUID(row.ID),
		Name:              row.Name,
		Price:             vo.NewMoney(row.PriceAmount, vo.Currency(row.PriceCurrency)),
		Type:              aggregate.AddonType(row.AddonType),
		ExtraTrafficBytes: row.ExtraTrafficBytes,
		ExtraNodes:        row.ExtraNodes,
		ExtraFeatureFlags: row.ExtraFeatureFlags,
	}
}

func (r *PlanRepository) loadAddons(ctx context.Context, plan *aggregate.Plan) error {
	addonRows, err := r.queries.GetAddonsByPlanID(ctx, pgutil.UUIDToPgtype(plan.ID))
	if err != nil {
		return pgutil.MapErr(err, "get addons for plan", billing.ErrPlanNotFound)
	}
	addons := make([]aggregate.Addon, len(addonRows))
	for i, row := range addonRows {
		addons[i] = addonRowToDomain(row)
	}
	plan.AvailableAddons = addons
	return nil
}

func (r *PlanRepository) GetByID(ctx context.Context, id string) (*aggregate.Plan, error) {
	row, err := r.queries.GetPlanByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get plan by id", billing.ErrPlanNotFound)
	}
	plan := planRowToDomain(row)
	if err := r.loadAddons(ctx, plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func (r *PlanRepository) GetAll(ctx context.Context) ([]*aggregate.Plan, error) {
	rows, err := r.queries.GetAllPlans(ctx)
	if err != nil {
		return nil, pgutil.MapErr(err, "get all plans", billing.ErrPlanNotFound)
	}
	plans := make([]*aggregate.Plan, len(rows))
	for i, row := range rows {
		plans[i] = planRowToDomain(row)
		if err := r.loadAddons(ctx, plans[i]); err != nil {
			return nil, err
		}
	}
	return plans, nil
}

func (r *PlanRepository) GetActive(ctx context.Context) ([]*aggregate.Plan, error) {
	rows, err := r.queries.GetActivePlans(ctx)
	if err != nil {
		return nil, pgutil.MapErr(err, "get active plans", billing.ErrPlanNotFound)
	}
	plans := make([]*aggregate.Plan, len(rows))
	for i, row := range rows {
		plans[i] = planRowToDomain(row)
		if err := r.loadAddons(ctx, plans[i]); err != nil {
			return nil, err
		}
	}
	return plans, nil
}

func (r *PlanRepository) Create(ctx context.Context, plan *aggregate.Plan) error {
	err := r.queries.CreatePlan(ctx, gen.CreatePlanParams{
		ID:                   pgutil.UUIDToPgtype(plan.ID),
		Name:                 plan.Name,
		Description:          pgutil.StrPtrOrNil(plan.Description),
		BasePriceAmount:      plan.BasePrice.Amount,
		BasePriceCurrency:    string(plan.BasePrice.Currency),
		BillingInterval:      string(plan.Interval),
		TrafficLimitBytes:    plan.TrafficLimitBytes,
		DeviceLimit:          int32(plan.DeviceLimit),
		AllowedCountries:     plan.AllowedCountries,
		AllowedProtocols:     plan.AllowedProtocols,
		Tier:                 string(plan.Tier),
		MaxRemnawaveBindings: int32(plan.MaxRemnawaveBindings),
		FamilyEnabled:        plan.FamilyEnabled,
		MaxFamilyMembers:     int32(plan.MaxFamilyMembers),
		IsActive:             plan.IsActive,
		CreatedAt:            pgutil.TimeToPgtype(plan.CreatedAt),
		UpdatedAt:            pgutil.TimeToPgtype(plan.UpdatedAt),
	})
	if err != nil {
		return pgutil.MapErr(err, "create plan", billing.ErrPlanNotFound)
	}

	for _, addon := range plan.AvailableAddons {
		if err := r.createAddon(ctx, plan.ID, addon); err != nil {
			return fmt.Errorf("create addon %s for plan: %w", addon.ID, err)
		}
	}
	return nil
}

func (r *PlanRepository) createAddon(ctx context.Context, planID string, addon aggregate.Addon) error {
	err := r.queries.CreatePlanAddon(ctx, gen.CreatePlanAddonParams{
		ID:                pgutil.UUIDToPgtype(addon.ID),
		PlanID:            pgutil.UUIDToPgtype(planID),
		Name:              addon.Name,
		PriceAmount:       addon.Price.Amount,
		PriceCurrency:     string(addon.Price.Currency),
		AddonType:         string(addon.Type),
		ExtraTrafficBytes: addon.ExtraTrafficBytes,
		ExtraNodes:        addon.ExtraNodes,
		ExtraFeatureFlags: addon.ExtraFeatureFlags,
		CreatedAt:         pgutil.TimeToPgtype(time.Now()),
	})
	return pgutil.MapErr(err, "create plan addon", billing.ErrPlanNotFound)
}

func (r *PlanRepository) Update(ctx context.Context, plan *aggregate.Plan) error {
	err := r.queries.UpdatePlan(ctx, gen.UpdatePlanParams{
		ID:                   pgutil.UUIDToPgtype(plan.ID),
		Name:                 plan.Name,
		Description:          pgutil.StrPtrOrNil(plan.Description),
		BasePriceAmount:      plan.BasePrice.Amount,
		BasePriceCurrency:    string(plan.BasePrice.Currency),
		BillingInterval:      string(plan.Interval),
		TrafficLimitBytes:    plan.TrafficLimitBytes,
		DeviceLimit:          int32(plan.DeviceLimit),
		AllowedCountries:     plan.AllowedCountries,
		AllowedProtocols:     plan.AllowedProtocols,
		Tier:                 string(plan.Tier),
		MaxRemnawaveBindings: int32(plan.MaxRemnawaveBindings),
		FamilyEnabled:        plan.FamilyEnabled,
		MaxFamilyMembers:     int32(plan.MaxFamilyMembers),
		IsActive:             plan.IsActive,
	})
	if err != nil {
		return pgutil.MapErr(err, "update plan", billing.ErrPlanNotFound)
	}

	// Replace addons: delete all, re-insert.
	if err := r.queries.DeleteAddonsByPlanID(ctx, pgutil.UUIDToPgtype(plan.ID)); err != nil {
		return pgutil.MapErr(err, "delete addons for plan", billing.ErrPlanNotFound)
	}
	for _, addon := range plan.AvailableAddons {
		if err := r.createAddon(ctx, plan.ID, addon); err != nil {
			return fmt.Errorf("recreate addon %s for plan: %w", addon.ID, err)
		}
	}
	return nil
}

var _ billing.PlanRepository = (*PlanRepository)(nil)

// ---------------------------------------------------------------------------
// SubscriptionRepository
// ---------------------------------------------------------------------------

// SubscriptionRepository implements billing.SubscriptionRepository backed by PostgreSQL.
type SubscriptionRepository struct {
	pool    *pgxpool.Pool
	queries *gen.Queries
}

// NewSubscriptionRepository returns a new SubscriptionRepository using the given pool.
func NewSubscriptionRepository(pool *pgxpool.Pool) *SubscriptionRepository {
	return &SubscriptionRepository{
		pool:    pool,
		queries: gen.New(pool),
	}
}

func subRowToDomain(row gen.BillingSubscription) *aggregate.Subscription {
	return &aggregate.Subscription{
		ID:     pgutil.PgtypeToUUID(row.ID),
		UserID: pgutil.PgtypeToUUID(row.UserID),
		PlanID: pgutil.PgtypeToUUID(row.PlanID),
		Status: aggregate.SubscriptionStatus(row.Status),
		Period: vo.BillingPeriod{
			Start:    pgutil.PgtypeToTime(row.PeriodStart),
			End:      pgutil.PgtypeToTime(row.PeriodEnd),
			Interval: vo.BillingInterval(row.PeriodInterval),
		},
		AddonIDs:    pgutil.PgtypeUUIDsToStrings(row.AddonIds),
		AssignedTo:  pgutil.DerefStr(row.AssignedTo),
		CancelledAt: pgutil.PgtypeToOptTime(row.CancelledAt),
		PausedAt:    pgutil.PgtypeToOptTime(row.PausedAt),
		CreatedAt:   pgutil.PgtypeToTime(row.CreatedAt),
		UpdatedAt:   pgutil.PgtypeToTime(row.UpdatedAt),
	}
}

func (r *SubscriptionRepository) GetByID(ctx context.Context, id string) (*aggregate.Subscription, error) {
	row, err := r.queries.GetSubscriptionByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get subscription by id", billing.ErrSubscriptionNotFound)
	}
	return subRowToDomain(row), nil
}

func (r *SubscriptionRepository) GetByUserID(ctx context.Context, userID string) ([]*aggregate.Subscription, error) {
	rows, err := r.queries.GetSubscriptionsByUserID(ctx, pgutil.UUIDToPgtype(userID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get subscriptions by user id", billing.ErrSubscriptionNotFound)
	}
	subs := make([]*aggregate.Subscription, len(rows))
	for i, row := range rows {
		subs[i] = subRowToDomain(row)
	}
	return subs, nil
}

func (r *SubscriptionRepository) GetActiveByUserID(ctx context.Context, userID string) ([]*aggregate.Subscription, error) {
	rows, err := r.queries.GetActiveSubscriptionsByUserID(ctx, pgutil.UUIDToPgtype(userID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get active subscriptions by user id", billing.ErrSubscriptionNotFound)
	}
	subs := make([]*aggregate.Subscription, len(rows))
	for i, row := range rows {
		subs[i] = subRowToDomain(row)
	}
	return subs, nil
}

func (r *SubscriptionRepository) GetAll(ctx context.Context, limit, offset int) ([]*aggregate.Subscription, error) {
	rows, err := r.queries.GetAllSubscriptions(ctx, gen.GetAllSubscriptionsParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, pgutil.MapErr(err, "get all subscriptions", billing.ErrSubscriptionNotFound)
	}
	subs := make([]*aggregate.Subscription, len(rows))
	for i, row := range rows {
		subs[i] = subRowToDomain(row)
	}
	return subs, nil
}

func (r *SubscriptionRepository) Create(ctx context.Context, sub *aggregate.Subscription) error {
	err := r.queries.CreateSubscription(ctx, gen.CreateSubscriptionParams{
		ID:             pgutil.UUIDToPgtype(sub.ID),
		UserID:         pgutil.UUIDToPgtype(sub.UserID),
		PlanID:         pgutil.UUIDToPgtype(sub.PlanID),
		Status:         string(sub.Status),
		PeriodStart:    pgutil.TimeToPgtype(sub.Period.Start),
		PeriodEnd:      pgutil.TimeToPgtype(sub.Period.End),
		PeriodInterval: string(sub.Period.Interval),
		AddonIds:       pgutil.StringsToPgtypeUUIDs(sub.AddonIDs),
		AssignedTo:     pgutil.StrPtrOrNil(sub.AssignedTo),
		CancelledAt:    pgutil.OptTimeToPgtype(sub.CancelledAt),
		PausedAt:       pgutil.OptTimeToPgtype(sub.PausedAt),
		CreatedAt:      pgutil.TimeToPgtype(sub.CreatedAt),
		UpdatedAt:      pgutil.TimeToPgtype(sub.UpdatedAt),
	})
	return pgutil.MapErr(err, "create subscription", billing.ErrSubscriptionNotFound)
}

func (r *SubscriptionRepository) Update(ctx context.Context, sub *aggregate.Subscription) error {
	err := r.queries.UpdateSubscription(ctx, gen.UpdateSubscriptionParams{
		ID:             pgutil.UUIDToPgtype(sub.ID),
		Status:         string(sub.Status),
		PeriodStart:    pgutil.TimeToPgtype(sub.Period.Start),
		PeriodEnd:      pgutil.TimeToPgtype(sub.Period.End),
		PeriodInterval: string(sub.Period.Interval),
		AddonIds:       pgutil.StringsToPgtypeUUIDs(sub.AddonIDs),
		AssignedTo:     pgutil.StrPtrOrNil(sub.AssignedTo),
		CancelledAt:    pgutil.OptTimeToPgtype(sub.CancelledAt),
		PausedAt:       pgutil.OptTimeToPgtype(sub.PausedAt),
	})
	return pgutil.MapErr(err, "update subscription", billing.ErrSubscriptionNotFound)
}

var _ billing.SubscriptionRepository = (*SubscriptionRepository)(nil)

// ---------------------------------------------------------------------------
// InvoiceRepository
// ---------------------------------------------------------------------------

// InvoiceRepository implements billing.InvoiceRepository backed by PostgreSQL.
type InvoiceRepository struct {
	pool    *pgxpool.Pool
	queries *gen.Queries
}

// NewInvoiceRepository returns a new InvoiceRepository using the given pool.
func NewInvoiceRepository(pool *pgxpool.Pool) *InvoiceRepository {
	return &InvoiceRepository{
		pool:    pool,
		queries: gen.New(pool),
	}
}

func invoiceRowToDomain(row gen.BillingInvoice) *aggregate.Invoice {
	return &aggregate.Invoice{
		ID:             pgutil.PgtypeToUUID(row.ID),
		SubscriptionID: pgutil.PgtypeToUUID(row.SubscriptionID),
		UserID:         pgutil.PgtypeToUUID(row.UserID),
		Subtotal:       vo.NewMoney(row.SubtotalAmount, vo.Currency(row.Currency)),
		TotalDiscount:  vo.NewMoney(row.TotalDiscountAmount, vo.Currency(row.Currency)),
		Total:          vo.NewMoney(row.TotalAmount, vo.Currency(row.Currency)),
		Status:         aggregate.InvoiceStatus(row.Status),
		PaidAt:         pgutil.PgtypeToOptTime(row.PaidAt),
		CreatedAt:      pgutil.PgtypeToTime(row.CreatedAt),
		UpdatedAt:      pgutil.PgtypeToTime(row.UpdatedAt),
	}
}

func lineItemRowToDomain(row gen.BillingInvoiceLineItem) vo.LineItem {
	return vo.LineItem{
		Description: row.Description,
		Type:        vo.LineItemType(row.ItemType),
		Amount:      vo.NewMoney(row.Amount, vo.Currency(row.Currency)),
		Quantity:    int(row.Quantity),
	}
}

func (r *InvoiceRepository) loadLineItems(ctx context.Context, inv *aggregate.Invoice) error {
	rows, err := r.queries.GetLineItemsByInvoiceID(ctx, pgutil.UUIDToPgtype(inv.ID))
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

func (r *InvoiceRepository) GetByID(ctx context.Context, id string) (*aggregate.Invoice, error) {
	row, err := r.queries.GetInvoiceByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get invoice by id", billing.ErrInvoiceNotFound)
	}
	inv := invoiceRowToDomain(row)
	if err := r.loadLineItems(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

func (r *InvoiceRepository) GetBySubscriptionID(ctx context.Context, subID string) ([]*aggregate.Invoice, error) {
	rows, err := r.queries.GetInvoicesBySubscriptionID(ctx, pgutil.UUIDToPgtype(subID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get invoices by subscription id", billing.ErrInvoiceNotFound)
	}
	invoices := make([]*aggregate.Invoice, len(rows))
	for i, row := range rows {
		invoices[i] = invoiceRowToDomain(row)
		if err := r.loadLineItems(ctx, invoices[i]); err != nil {
			return nil, err
		}
	}
	return invoices, nil
}

func (r *InvoiceRepository) GetPendingByUserID(ctx context.Context, userID string) ([]*aggregate.Invoice, error) {
	rows, err := r.queries.GetPendingInvoicesByUserID(ctx, pgutil.UUIDToPgtype(userID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get pending invoices by user id", billing.ErrInvoiceNotFound)
	}
	invoices := make([]*aggregate.Invoice, len(rows))
	for i, row := range rows {
		invoices[i] = invoiceRowToDomain(row)
		if err := r.loadLineItems(ctx, invoices[i]); err != nil {
			return nil, err
		}
	}
	return invoices, nil
}

func (r *InvoiceRepository) GetAll(ctx context.Context, limit, offset int) ([]*aggregate.Invoice, error) {
	rows, err := r.queries.GetAllInvoices(ctx, gen.GetAllInvoicesParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, pgutil.MapErr(err, "get all invoices", billing.ErrInvoiceNotFound)
	}
	invoices := make([]*aggregate.Invoice, len(rows))
	for i, row := range rows {
		invoices[i] = invoiceRowToDomain(row)
	}
	return invoices, nil
}

func (r *InvoiceRepository) Create(ctx context.Context, inv *aggregate.Invoice) error {
	err := r.queries.CreateInvoice(ctx, gen.CreateInvoiceParams{
		ID:                  pgutil.UUIDToPgtype(inv.ID),
		SubscriptionID:      pgutil.UUIDToPgtype(inv.SubscriptionID),
		UserID:              pgutil.UUIDToPgtype(inv.UserID),
		SubtotalAmount:      inv.Subtotal.Amount,
		TotalDiscountAmount: inv.TotalDiscount.Amount,
		TotalAmount:         inv.Total.Amount,
		Currency:            string(inv.Total.Currency),
		Status:              string(inv.Status),
		PaidAt:              pgutil.OptTimeToPgtype(inv.PaidAt),
		CreatedAt:           pgutil.TimeToPgtype(inv.CreatedAt),
		UpdatedAt:           pgutil.TimeToPgtype(inv.UpdatedAt),
	})
	if err != nil {
		return pgutil.MapErr(err, "create invoice", billing.ErrInvoiceNotFound)
	}

	for _, item := range inv.LineItems {
		if err := r.createLineItem(ctx, inv.ID, item); err != nil {
			return fmt.Errorf("create line item %q for invoice: %w", item.Description, err)
		}
	}
	return nil
}

func (r *InvoiceRepository) createLineItem(ctx context.Context, invoiceID string, item vo.LineItem) error {
	err := r.queries.CreateInvoiceLineItem(ctx, gen.CreateInvoiceLineItemParams{
		InvoiceID:   pgutil.UUIDToPgtype(invoiceID),
		Description: item.Description,
		ItemType:    string(item.Type),
		Amount:      item.Amount.Amount,
		Currency:    string(item.Amount.Currency),
		Quantity:    int32(item.Quantity),
	})
	return pgutil.MapErr(err, "create invoice line item", billing.ErrInvoiceNotFound)
}

func (r *InvoiceRepository) Update(ctx context.Context, inv *aggregate.Invoice) error {
	err := r.queries.UpdateInvoice(ctx, gen.UpdateInvoiceParams{
		ID:                  pgutil.UUIDToPgtype(inv.ID),
		Status:              string(inv.Status),
		PaidAt:              pgutil.OptTimeToPgtype(inv.PaidAt),
		SubtotalAmount:      inv.Subtotal.Amount,
		TotalDiscountAmount: inv.TotalDiscount.Amount,
		TotalAmount:         inv.Total.Amount,
	})
	return pgutil.MapErr(err, "update invoice", billing.ErrInvoiceNotFound)
}

var _ billing.InvoiceRepository = (*InvoiceRepository)(nil)

// ---------------------------------------------------------------------------
// FamilyRepository
// ---------------------------------------------------------------------------

// FamilyRepository implements billing.FamilyRepository backed by PostgreSQL.
type FamilyRepository struct {
	pool    *pgxpool.Pool
	queries *gen.Queries
}

// NewFamilyRepository returns a new FamilyRepository using the given pool.
func NewFamilyRepository(pool *pgxpool.Pool) *FamilyRepository {
	return &FamilyRepository{
		pool:    pool,
		queries: gen.New(pool),
	}
}

func familyGroupRowToDomain(row gen.BillingFamilyGroup) *aggregate.FamilyGroup {
	return &aggregate.FamilyGroup{
		ID:         pgutil.PgtypeToUUID(row.ID),
		OwnerID:    pgutil.PgtypeToUUID(row.OwnerID),
		MaxMembers: int(row.MaxMembers),
		CreatedAt:  pgutil.PgtypeToTime(row.CreatedAt),
		UpdatedAt:  pgutil.PgtypeToTime(row.UpdatedAt),
	}
}

func familyMemberRowToDomain(row gen.BillingFamilyMember) aggregate.FamilyMember {
	return aggregate.FamilyMember{
		UserID:   pgutil.PgtypeToUUID(row.UserID),
		Role:     aggregate.MemberRole(row.Role),
		Nickname: pgutil.DerefStr(row.Nickname),
		JoinedAt: pgutil.PgtypeToTime(row.JoinedAt),
	}
}

func (r *FamilyRepository) loadMembers(ctx context.Context, fg *aggregate.FamilyGroup) error {
	rows, err := r.queries.GetFamilyMembersByGroupID(ctx, pgutil.UUIDToPgtype(fg.ID))
	if err != nil {
		return pgutil.MapErr(err, "get family members", billing.ErrFamilyGroupNotFound)
	}
	members := make([]aggregate.FamilyMember, len(rows))
	for i, row := range rows {
		members[i] = familyMemberRowToDomain(row)
	}
	fg.Members = members
	return nil
}

func (r *FamilyRepository) GetByID(ctx context.Context, id string) (*aggregate.FamilyGroup, error) {
	row, err := r.queries.GetFamilyGroupByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get family group by id", billing.ErrFamilyGroupNotFound)
	}
	fg := familyGroupRowToDomain(row)
	if err := r.loadMembers(ctx, fg); err != nil {
		return nil, err
	}
	return fg, nil
}

func (r *FamilyRepository) GetByOwnerID(ctx context.Context, ownerID string) (*aggregate.FamilyGroup, error) {
	row, err := r.queries.GetFamilyGroupByOwnerID(ctx, pgutil.UUIDToPgtype(ownerID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get family group by owner id", billing.ErrFamilyGroupNotFound)
	}
	fg := familyGroupRowToDomain(row)
	if err := r.loadMembers(ctx, fg); err != nil {
		return nil, err
	}
	return fg, nil
}

func (r *FamilyRepository) Create(ctx context.Context, fg *aggregate.FamilyGroup) error {
	err := r.queries.CreateFamilyGroup(ctx, gen.CreateFamilyGroupParams{
		ID:         pgutil.UUIDToPgtype(fg.ID),
		OwnerID:    pgutil.UUIDToPgtype(fg.OwnerID),
		MaxMembers: int32(fg.MaxMembers),
		CreatedAt:  pgutil.TimeToPgtype(fg.CreatedAt),
		UpdatedAt:  pgutil.TimeToPgtype(fg.UpdatedAt),
	})
	if err != nil {
		return pgutil.MapErr(err, "create family group", billing.ErrFamilyGroupNotFound)
	}

	for _, member := range fg.Members {
		if err := r.createMember(ctx, fg.ID, member); err != nil {
			return fmt.Errorf("create member %s for family group: %w", member.UserID, err)
		}
	}
	return nil
}

func (r *FamilyRepository) createMember(ctx context.Context, groupID string, member aggregate.FamilyMember) error {
	err := r.queries.CreateFamilyMember(ctx, gen.CreateFamilyMemberParams{
		FamilyGroupID: pgutil.UUIDToPgtype(groupID),
		UserID:        pgutil.UUIDToPgtype(member.UserID),
		Role:          string(member.Role),
		Nickname:      pgutil.StrPtrOrNil(member.Nickname),
		JoinedAt:      pgutil.TimeToPgtype(member.JoinedAt),
	})
	return pgutil.MapErr(err, "create family member", billing.ErrFamilyGroupNotFound)
}

func (r *FamilyRepository) Update(ctx context.Context, fg *aggregate.FamilyGroup) error {
	err := r.queries.UpdateFamilyGroup(ctx, gen.UpdateFamilyGroupParams{
		ID:         pgutil.UUIDToPgtype(fg.ID),
		MaxMembers: int32(fg.MaxMembers),
	})
	if err != nil {
		return pgutil.MapErr(err, "update family group", billing.ErrFamilyGroupNotFound)
	}

	// Replace members: delete all, re-insert.
	if err := r.queries.DeleteFamilyMembersByGroupID(ctx, pgutil.UUIDToPgtype(fg.ID)); err != nil {
		return pgutil.MapErr(err, "delete family members", billing.ErrFamilyGroupNotFound)
	}
	for _, member := range fg.Members {
		if err := r.createMember(ctx, fg.ID, member); err != nil {
			return fmt.Errorf("recreate member %s for family group: %w", member.UserID, err)
		}
	}
	return nil
}

func (r *FamilyRepository) Delete(ctx context.Context, id string) error {
	err := r.queries.DeleteFamilyGroup(ctx, pgutil.UUIDToPgtype(id))
	return pgutil.MapErr(err, "delete family group", billing.ErrFamilyGroupNotFound)
}

var _ billing.FamilyRepository = (*FamilyRepository)(nil)
