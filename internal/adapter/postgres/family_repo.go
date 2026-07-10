package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

// ---------------------------------------------------------------------------
// FamilyRepository
// ---------------------------------------------------------------------------

// FamilyRepository implements billing.FamilyRepository backed by PostgreSQL.
type FamilyRepository struct {
	pool *pgxpool.Pool
}

// NewFamilyRepository returns a new FamilyRepository using the given pool.
func NewFamilyRepository(pool *pgxpool.Pool) *FamilyRepository {
	return &FamilyRepository{pool: pool}
}

// q returns a *gen.Queries backed by the active transaction (if any) or the
// pool. This ensures all methods transparently participate in RunInTx and
// respect RLS tenant scoping.
func (r *FamilyRepository) q(ctx context.Context) *gen.Queries {
	return gen.New(DBFromContext(ctx, r.pool))
}

// familyGroupFields holds the columns shared by every family-group row shape.
type familyGroupFields struct {
	ID         pgtype.UUID
	OwnerID    pgtype.UUID
	MaxMembers int32
	CreatedAt  pgtype.Timestamptz
	UpdatedAt  pgtype.Timestamptz
}

// familyGroupRow constrains the sqlc-generated family-group row types. The
// explicit-column SELECTs each emit a distinct *Row struct (the model carries
// the extra tenant_id column added in migration 042), so the converter is
// generic over them.
type familyGroupRow interface {
	gen.GetFamilyGroupByIDRow | gen.GetFamilyGroupByOwnerIDRow | gen.GetFamilyGroupByOwnerIDForUpdateRow
}

func extractFamilyGroupFields[T familyGroupRow](row T) familyGroupFields {
	switch r := any(row).(type) {
	case gen.GetFamilyGroupByIDRow:
		return familyGroupFields{r.ID, r.OwnerID, r.MaxMembers, r.CreatedAt, r.UpdatedAt}
	case gen.GetFamilyGroupByOwnerIDRow:
		return familyGroupFields{r.ID, r.OwnerID, r.MaxMembers, r.CreatedAt, r.UpdatedAt}
	case gen.GetFamilyGroupByOwnerIDForUpdateRow:
		return familyGroupFields{r.ID, r.OwnerID, r.MaxMembers, r.CreatedAt, r.UpdatedAt}
	default:
		panic("unreachable: unhandled familyGroupRow type")
	}
}

func familyGroupRowToDomain[T familyGroupRow](row T) *aggregate.FamilyGroup {
	f := extractFamilyGroupFields(row)
	return &aggregate.FamilyGroup{
		ID:         pgutil.PgtypeToUUID(f.ID),
		OwnerID:    pgutil.PgtypeToUUID(f.OwnerID),
		MaxMembers: int(f.MaxMembers),
		CreatedAt:  pgutil.PgtypeToTime(f.CreatedAt),
		UpdatedAt:  pgutil.PgtypeToTime(f.UpdatedAt),
	}
}

func familyMemberRowToDomain(row gen.GetFamilyMembersByGroupIDRow) aggregate.FamilyMember {
	return aggregate.FamilyMember{
		UserID:   pgutil.PgtypeToUUID(row.UserID),
		Role:     aggregate.MemberRole(row.Role),
		Nickname: pgutil.DerefStr(row.Nickname),
		JoinedAt: pgutil.PgtypeToTime(row.JoinedAt),
	}
}

func (r *FamilyRepository) loadMembers(ctx context.Context, fg *aggregate.FamilyGroup) error {
	rows, err := r.q(ctx).GetFamilyMembersByGroupID(ctx, pgutil.UUIDToPgtype(fg.ID))
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
	row, err := r.q(ctx).GetFamilyGroupByID(ctx, pgutil.UUIDToPgtype(id))
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
	row, err := r.q(ctx).GetFamilyGroupByOwnerID(ctx, pgutil.UUIDToPgtype(ownerID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get family group by owner id", billing.ErrFamilyGroupNotFound)
	}
	fg := familyGroupRowToDomain(row)
	if err := r.loadMembers(ctx, fg); err != nil {
		return nil, err
	}
	return fg, nil
}

func (r *FamilyRepository) GetByOwnerIDForUpdate(ctx context.Context, ownerID string) (*aggregate.FamilyGroup, error) {
	row, err := r.q(ctx).GetFamilyGroupByOwnerIDForUpdate(ctx, pgutil.UUIDToPgtype(ownerID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get family group by owner id for update", billing.ErrFamilyGroupNotFound)
	}
	fg := familyGroupRowToDomain(row)
	if err := r.loadMembers(ctx, fg); err != nil {
		return nil, err
	}
	return fg, nil
}

func (r *FamilyRepository) Create(ctx context.Context, fg *aggregate.FamilyGroup) error {
	err := r.q(ctx).CreateFamilyGroup(ctx, gen.CreateFamilyGroupParams{
		ID:         pgutil.UUIDToPgtype(fg.ID),
		OwnerID:    pgutil.UUIDToPgtype(fg.OwnerID),
		MaxMembers: int32(fg.MaxMembers),
		CreatedAt:  pgutil.TimeToPgtype(fg.CreatedAt),
		UpdatedAt:  pgutil.TimeToPgtype(fg.UpdatedAt),
	})
	if err != nil {
		if pgutil.IsUniqueViolation(err) {
			return fmt.Errorf("create family group: %w", billing.ErrFamilyGroupAlreadyExists)
		}
		return fmt.Errorf("create family group: %w", err)
	}

	for _, member := range fg.Members {
		if err := r.createMember(ctx, fg.ID, member); err != nil {
			return fmt.Errorf("create member %s for family group: %w", member.UserID, err)
		}
	}
	return nil
}

func (r *FamilyRepository) createMember(ctx context.Context, groupID string, member aggregate.FamilyMember) error {
	err := r.q(ctx).CreateFamilyMember(ctx, gen.CreateFamilyMemberParams{
		FamilyGroupID: pgutil.UUIDToPgtype(groupID),
		UserID:        pgutil.UUIDToPgtype(member.UserID),
		Role:          string(member.Role),
		Nickname:      pgutil.StrPtrOrNil(member.Nickname),
		JoinedAt:      pgutil.TimeToPgtype(member.JoinedAt),
	})
	if err != nil {
		if pgutil.IsUniqueViolation(err) {
			return fmt.Errorf("create family member: %w", billing.ErrFamilyGroupAlreadyExists)
		}
		return fmt.Errorf("create family member: %w", err)
	}
	return nil
}

func (r *FamilyRepository) upsertMember(ctx context.Context, groupID string, member aggregate.FamilyMember) error {
	err := r.q(ctx).UpsertFamilyMember(ctx, gen.UpsertFamilyMemberParams{
		FamilyGroupID: pgutil.UUIDToPgtype(groupID),
		UserID:        pgutil.UUIDToPgtype(member.UserID),
		Role:          string(member.Role),
		Nickname:      pgutil.StrPtrOrNil(member.Nickname),
		JoinedAt:      pgutil.TimeToPgtype(member.JoinedAt),
	})
	return pgutil.MapErr(err, "upsert family member", billing.ErrFamilyGroupNotFound)
}

func (r *FamilyRepository) Update(ctx context.Context, fg *aggregate.FamilyGroup) error {
	err := r.q(ctx).UpdateFamilyGroup(ctx, gen.UpdateFamilyGroupParams{
		ID:         pgutil.UUIDToPgtype(fg.ID),
		MaxMembers: int32(fg.MaxMembers),
	})
	if err != nil {
		return pgutil.MapErr(err, "update family group", billing.ErrFamilyGroupNotFound)
	}

	// Upsert each member, then prune members removed from the list.
	// This avoids DELETE ALL + RE-INSERT which breaks FK references
	// and loses joined_at timestamps for unchanged members.
	retainUserIDs := make([]pgtype.UUID, 0, len(fg.Members))
	for _, member := range fg.Members {
		if err := r.upsertMember(ctx, fg.ID, member); err != nil {
			return fmt.Errorf("upsert member %s for family group: %w", member.UserID, err)
		}
		retainUserIDs = append(retainUserIDs, pgutil.UUIDToPgtype(member.UserID))
	}

	if err := r.q(ctx).DeleteRemovedFamilyMembers(ctx, gen.DeleteRemovedFamilyMembersParams{
		FamilyGroupID: pgutil.UUIDToPgtype(fg.ID),
		UserIds:       retainUserIDs,
	}); err != nil {
		return pgutil.MapErr(err, "delete removed family members", billing.ErrFamilyGroupNotFound)
	}
	return nil
}

func (r *FamilyRepository) Delete(ctx context.Context, id string) error {
	err := r.q(ctx).DeleteFamilyGroup(ctx, pgutil.UUIDToPgtype(id))
	return pgutil.MapErr(err, "delete family group", billing.ErrFamilyGroupNotFound)
}

var _ billing.FamilyRepository = (*FamilyRepository)(nil)
