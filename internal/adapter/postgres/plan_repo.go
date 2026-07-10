package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

// ---------------------------------------------------------------------------
// PlanRepository
// ---------------------------------------------------------------------------

// PlanRepository implements billing.PlanRepository backed by PostgreSQL.
type PlanRepository struct {
	pool  *pgxpool.Pool
	clock clock.Clock
}

// NewPlanRepository returns a new PlanRepository using the given pool.
func NewPlanRepository(pool *pgxpool.Pool, clk clock.Clock) *PlanRepository {
	return &PlanRepository{pool: pool, clock: clk}
}

// q returns a *gen.Queries backed by the active transaction (if any) or the
// pool. This ensures all methods transparently participate in RunInTx and
// respect RLS tenant scoping.
func (r *PlanRepository) q(ctx context.Context) *gen.Queries {
	return gen.New(DBFromContext(ctx, r.pool))
}

// planFields holds the common columns returned by all plan queries.
type planFields struct {
	ID                   pgtype.UUID
	Name                 string
	Description          *string
	BasePriceAmount      int64
	BasePriceCurrency    string
	BillingInterval      string
	TrafficLimitBytes    int64
	DeviceLimit          int32
	AllowedCountries     []string
	AllowedProtocols     []string
	Tier                 string
	MaxRemnawaveBindings int32
	FamilyEnabled        bool
	MaxFamilyMembers     int32
	MaxAddons            int32
	IsActive             bool
	CreatedAt            pgtype.Timestamptz
	UpdatedAt            pgtype.Timestamptz
}

// planRow is a constraint matching all sqlc-generated plan row types.
type planRow interface {
	gen.GetPlanByIDRow | gen.GetAllPlansRow | gen.GetActivePlansRow
}

// extractPlanFields extracts the common fields from any plan row type.
func extractPlanFields[T planRow](row T) planFields {
	switch r := any(row).(type) {
	case gen.GetPlanByIDRow:
		return planFields{r.ID, r.Name, r.Description, r.BasePriceAmount, r.BasePriceCurrency, r.BillingInterval, r.TrafficLimitBytes, r.DeviceLimit, r.AllowedCountries, r.AllowedProtocols, r.Tier, r.MaxRemnawaveBindings, r.FamilyEnabled, r.MaxFamilyMembers, r.MaxAddons, r.IsActive, r.CreatedAt, r.UpdatedAt}
	case gen.GetAllPlansRow:
		return planFields{r.ID, r.Name, r.Description, r.BasePriceAmount, r.BasePriceCurrency, r.BillingInterval, r.TrafficLimitBytes, r.DeviceLimit, r.AllowedCountries, r.AllowedProtocols, r.Tier, r.MaxRemnawaveBindings, r.FamilyEnabled, r.MaxFamilyMembers, r.MaxAddons, r.IsActive, r.CreatedAt, r.UpdatedAt}
	case gen.GetActivePlansRow:
		return planFields{r.ID, r.Name, r.Description, r.BasePriceAmount, r.BasePriceCurrency, r.BillingInterval, r.TrafficLimitBytes, r.DeviceLimit, r.AllowedCountries, r.AllowedProtocols, r.Tier, r.MaxRemnawaveBindings, r.FamilyEnabled, r.MaxFamilyMembers, r.MaxAddons, r.IsActive, r.CreatedAt, r.UpdatedAt}
	default:
		panic("unreachable: unhandled planRow type")
	}
}

// planRowToDomain converts any sqlc-generated plan row type to domain.
func planRowToDomain[T planRow](row T) *aggregate.Plan {
	return planFieldsToDomain(extractPlanFields(row))
}

func planFieldsToDomain(f planFields) *aggregate.Plan {
	return &aggregate.Plan{
		ID:                   pgutil.PgtypeToUUID(f.ID),
		Name:                 f.Name,
		Description:          pgutil.DerefStr(f.Description),
		BasePrice:            vo.NewMoney(f.BasePriceAmount, vo.Currency(f.BasePriceCurrency)),
		Interval:             vo.BillingInterval(f.BillingInterval),
		TrafficLimitBytes:    f.TrafficLimitBytes,
		DeviceLimit:          int(f.DeviceLimit),
		AllowedCountries:     f.AllowedCountries,
		AllowedProtocols:     f.AllowedProtocols,
		Tier:                 aggregate.PlanTier(f.Tier),
		MaxRemnawaveBindings: int(f.MaxRemnawaveBindings),
		FamilyEnabled:        f.FamilyEnabled,
		MaxFamilyMembers:     int(f.MaxFamilyMembers),
		MaxAddons:            int(f.MaxAddons),
		IsActive:             f.IsActive,
		CreatedAt:            pgutil.PgtypeToTime(f.CreatedAt),
		UpdatedAt:            pgutil.PgtypeToTime(f.UpdatedAt),
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
	addonRows, err := r.q(ctx).GetAddonsByPlanID(ctx, pgutil.UUIDToPgtype(plan.ID))
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
	row, err := r.q(ctx).GetPlanByID(ctx, pgutil.UUIDToPgtype(id))
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
	rows, err := r.q(ctx).GetAllPlans(ctx)
	if err != nil {
		return nil, pgutil.MapErr(err, "get all plans", billing.ErrPlanNotFound)
	}
	plans := make([]*aggregate.Plan, len(rows))
	for i, row := range rows {
		plans[i] = planRowToDomain(row)
	}
	if err := r.batchLoadAddons(ctx, plans); err != nil {
		return nil, err
	}
	return plans, nil
}

func (r *PlanRepository) GetActive(ctx context.Context) ([]*aggregate.Plan, error) {
	rows, err := r.q(ctx).GetActivePlans(ctx)
	if err != nil {
		return nil, pgutil.MapErr(err, "get active plans", billing.ErrPlanNotFound)
	}
	plans := make([]*aggregate.Plan, len(rows))
	for i, row := range rows {
		plans[i] = planRowToDomain(row)
	}
	if err := r.batchLoadAddons(ctx, plans); err != nil {
		return nil, err
	}
	return plans, nil
}

// batchLoadAddons loads addons for multiple plans in a single query,
// eliminating the N+1 problem from per-plan loadAddons calls.
func (r *PlanRepository) batchLoadAddons(ctx context.Context, plans []*aggregate.Plan) error {
	if len(plans) == 0 {
		return nil
	}
	ids := make([]pgtype.UUID, len(plans))
	for i, p := range plans {
		ids[i] = pgutil.UUIDToPgtype(p.ID)
	}
	addonRows, err := r.q(ctx).GetAddonsByPlanIDs(ctx, ids)
	if err != nil {
		return fmt.Errorf("batch load addons: %w", err)
	}
	// Group by plan_id
	byPlan := make(map[string][]aggregate.Addon, len(plans))
	for _, row := range addonRows {
		pid := pgutil.PgtypeToUUID(row.PlanID)
		byPlan[pid] = append(byPlan[pid], addonRowToDomain(row))
	}
	for _, p := range plans {
		p.AvailableAddons = byPlan[p.ID]
	}
	return nil
}

func (r *PlanRepository) Create(ctx context.Context, plan *aggregate.Plan) error {
	err := r.q(ctx).CreatePlan(ctx, gen.CreatePlanParams{
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
		MaxAddons:            int32(plan.MaxAddons),
		IsActive:             plan.IsActive,
		CreatedAt:            pgutil.TimeToPgtype(plan.CreatedAt),
		UpdatedAt:            pgutil.TimeToPgtype(plan.UpdatedAt),
	})
	if err != nil {
		if pgutil.IsUniqueViolation(err) {
			return fmt.Errorf("create plan: %w", billing.ErrPlanAlreadyExists)
		}
		return fmt.Errorf("create plan: %w", err)
	}

	for _, addon := range plan.AvailableAddons {
		if err := r.createAddon(ctx, plan.ID, addon); err != nil {
			return fmt.Errorf("create addon %s for plan: %w", addon.ID, err)
		}
	}
	return nil
}

func (r *PlanRepository) createAddon(ctx context.Context, planID string, addon aggregate.Addon) error {
	err := r.q(ctx).CreatePlanAddon(ctx, gen.CreatePlanAddonParams{
		ID:                pgutil.UUIDToPgtype(addon.ID),
		PlanID:            pgutil.UUIDToPgtype(planID),
		Name:              addon.Name,
		PriceAmount:       addon.Price.Amount,
		PriceCurrency:     string(addon.Price.Currency),
		AddonType:         string(addon.Type),
		ExtraTrafficBytes: addon.ExtraTrafficBytes,
		ExtraNodes:        addon.ExtraNodes,
		ExtraFeatureFlags: addon.ExtraFeatureFlags,
		CreatedAt:         pgutil.TimeToPgtype(r.clock.Now()),
	})
	if err != nil {
		if pgutil.IsUniqueViolation(err) {
			return fmt.Errorf("create plan addon: %w", billing.ErrPlanAlreadyExists)
		}
		return fmt.Errorf("create plan addon: %w", err)
	}
	return nil
}

func (r *PlanRepository) upsertAddon(ctx context.Context, planID string, addon aggregate.Addon) error {
	err := r.q(ctx).UpsertPlanAddon(ctx, gen.UpsertPlanAddonParams{
		ID:                pgutil.UUIDToPgtype(addon.ID),
		PlanID:            pgutil.UUIDToPgtype(planID),
		Name:              addon.Name,
		PriceAmount:       addon.Price.Amount,
		PriceCurrency:     string(addon.Price.Currency),
		AddonType:         string(addon.Type),
		ExtraTrafficBytes: addon.ExtraTrafficBytes,
		ExtraNodes:        addon.ExtraNodes,
		ExtraFeatureFlags: addon.ExtraFeatureFlags,
		CreatedAt:         pgutil.TimeToPgtype(r.clock.Now()),
	})
	return pgutil.MapErr(err, "upsert plan addon", billing.ErrPlanNotFound)
}

func (r *PlanRepository) Update(ctx context.Context, plan *aggregate.Plan) error {
	err := r.q(ctx).UpdatePlan(ctx, gen.UpdatePlanParams{
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
		MaxAddons:            int32(plan.MaxAddons),
		IsActive:             plan.IsActive,
	})
	if err != nil {
		return pgutil.MapErr(err, "update plan", billing.ErrPlanNotFound)
	}

	// Upsert each addon, then prune addons removed from the list.
	// This avoids DELETE ALL + RE-INSERT which breaks FK references
	// from other tables pointing to addon IDs.
	retainIDs := make([]pgtype.UUID, 0, len(plan.AvailableAddons))
	for _, addon := range plan.AvailableAddons {
		if err := r.upsertAddon(ctx, plan.ID, addon); err != nil {
			return fmt.Errorf("upsert addon %s for plan: %w", addon.ID, err)
		}
		retainIDs = append(retainIDs, pgutil.UUIDToPgtype(addon.ID))
	}

	if err := r.q(ctx).DeleteRemovedAddons(ctx, gen.DeleteRemovedAddonsParams{
		PlanID: pgutil.UUIDToPgtype(plan.ID),
		Ids:    retainIDs,
	}); err != nil {
		return pgutil.MapErr(err, "delete removed addons for plan", billing.ErrPlanNotFound)
	}
	return nil
}

var _ billing.PlanRepository = (*PlanRepository)(nil)
