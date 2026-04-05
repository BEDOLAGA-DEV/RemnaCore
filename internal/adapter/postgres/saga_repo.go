package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

// SagaRepository implements multisub.SagaRepository backed by PostgreSQL.
type SagaRepository struct {
	pool *pgxpool.Pool
}

// NewSagaRepository returns a new SagaRepository using the given pool.
func NewSagaRepository(pool *pgxpool.Pool) *SagaRepository {
	return &SagaRepository{pool: pool}
}

func (r *SagaRepository) queries(ctx context.Context) *gen.Queries {
	return gen.New(DBFromContext(ctx, r.pool))
}

func sagaRowToDomain(row gen.MultisubSagaInstance) *multisub.SagaInstance {
	return &multisub.SagaInstance{
		ID:            pgutil.PgtypeToUUID(row.ID),
		SagaType:      multisub.SagaType(row.SagaType),
		CorrelationID: row.CorrelationID,
		Status:        multisub.SagaStatus(row.Status),
		CurrentStep:   int(row.CurrentStep),
		TotalSteps:    int(row.TotalSteps),
		StateData:     row.StateData,
		ErrorMessage:  pgutil.DerefStr(row.ErrorMessage),
		CreatedAt:     pgutil.PgtypeToTime(row.CreatedAt),
		UpdatedAt:     pgutil.PgtypeToTime(row.UpdatedAt),
	}
}

// Create persists a new saga instance, returning it with the generated ID.
func (r *SagaRepository) Create(ctx context.Context, saga *multisub.SagaInstance) (*multisub.SagaInstance, error) {
	row, err := r.queries(ctx).CreateSagaInstance(ctx, gen.CreateSagaInstanceParams{
		SagaType:      string(saga.SagaType),
		CorrelationID: saga.CorrelationID,
		Status:        string(saga.Status),
		CurrentStep:   int32(saga.CurrentStep),
		TotalSteps:    int32(saga.TotalSteps),
		StateData:     saga.StateData,
	})
	if err != nil {
		return nil, fmt.Errorf("create saga instance: %w", err)
	}
	return sagaRowToDomain(row), nil
}

// UpdateProgress checkpoints the saga's current step and state data.
func (r *SagaRepository) UpdateProgress(ctx context.Context, id string, step int, stateData []byte) error {
	err := r.queries(ctx).UpdateSagaProgress(ctx, gen.UpdateSagaProgressParams{
		ID:          pgutil.UUIDToPgtype(id),
		CurrentStep: int32(step),
		StateData:   stateData,
	})
	if err != nil {
		return fmt.Errorf("update saga progress: %w", err)
	}
	return nil
}

// Complete marks a saga as successfully completed.
func (r *SagaRepository) Complete(ctx context.Context, id string) error {
	err := r.queries(ctx).CompleteSaga(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return fmt.Errorf("complete saga: %w", err)
	}
	return nil
}

// Fail marks a saga as failed with an error message.
func (r *SagaRepository) Fail(ctx context.Context, id string, errMsg string) error {
	err := r.queries(ctx).FailSaga(ctx, gen.FailSagaParams{
		ID:           pgutil.UUIDToPgtype(id),
		ErrorMessage: pgutil.StrPtrOrNil(errMsg),
	})
	if err != nil {
		return fmt.Errorf("fail saga: %w", err)
	}
	return nil
}

// GetRunning returns all sagas in 'running' status for resume on startup.
func (r *SagaRepository) GetRunning(ctx context.Context) ([]*multisub.SagaInstance, error) {
	rows, err := r.queries(ctx).GetRunningSagas(ctx)
	if err != nil {
		return nil, fmt.Errorf("get running sagas: %w", err)
	}
	result := make([]*multisub.SagaInstance, len(rows))
	for i, row := range rows {
		result[i] = sagaRowToDomain(row)
	}
	return result, nil
}

// GetByCorrelation looks up a saga by type and correlation ID.
func (r *SagaRepository) GetByCorrelation(ctx context.Context, sagaType multisub.SagaType, correlationID string) (*multisub.SagaInstance, error) {
	row, err := r.queries(ctx).GetSagaByCorrelation(ctx, gen.GetSagaByCorrelationParams{
		SagaType:      string(sagaType),
		CorrelationID: correlationID,
	})
	if err != nil {
		return nil, pgutil.MapErr(err, "get saga by correlation", multisub.ErrSagaNotFound)
	}
	return sagaRowToDomain(row), nil
}

// Cleanup removes completed and failed sagas older than the given duration.
func (r *SagaRepository) Cleanup(ctx context.Context, olderThan time.Duration) error {
	cutoff := pgutil.TimeToPgtype(time.Now().Add(-olderThan))
	err := r.queries(ctx).CleanupOldSagas(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("cleanup old sagas: %w", err)
	}
	return nil
}

// compile-time interface check
var _ multisub.SagaRepository = (*SagaRepository)(nil)
