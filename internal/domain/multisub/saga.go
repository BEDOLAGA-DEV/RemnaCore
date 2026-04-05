package multisub

import (
	"context"
	"time"
)

// SagaType identifies the kind of multi-step workflow being tracked.
type SagaType string

const (
	SagaTypeProvisioning   SagaType = "provisioning"
	SagaTypeDeprovisioning SagaType = "deprovisioning"
	SagaTypeSync           SagaType = "sync"
)

// SagaStatus represents the lifecycle state of a persistent saga instance.
type SagaStatus string

const (
	SagaStatusRunning      SagaStatus = "running"
	SagaStatusCompleted    SagaStatus = "completed"
	SagaStatusFailed       SagaStatus = "failed"
	SagaStatusCompensating SagaStatus = "compensating"
)

// SagaInstance represents a persistent saga execution state. It records which
// step a multi-step workflow has reached so it can be resumed after a crash.
type SagaInstance struct {
	ID            string
	SagaType      SagaType
	CorrelationID string
	Status        SagaStatus
	CurrentStep   int
	TotalSteps    int
	StateData     []byte // JSON-encoded step compensation data
	ErrorMessage  string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// SagaRepository defines persistence operations for saga state. The adapter
// layer (postgres) implements this interface.
type SagaRepository interface {
	// Create persists a new saga instance, returning it with the generated ID.
	Create(ctx context.Context, saga *SagaInstance) (*SagaInstance, error)

	// UpdateProgress checkpoints the saga's current step and state data.
	UpdateProgress(ctx context.Context, id string, step int, stateData []byte) error

	// Complete marks a saga as successfully completed.
	Complete(ctx context.Context, id string) error

	// Fail marks a saga as failed with an error message.
	Fail(ctx context.Context, id string, errMsg string) error

	// GetRunning returns all sagas in 'running' status for resume on startup.
	GetRunning(ctx context.Context) ([]*SagaInstance, error)

	// GetByCorrelation looks up a saga by type and correlation ID.
	GetByCorrelation(ctx context.Context, sagaType SagaType, correlationID string) (*SagaInstance, error)

	// Cleanup removes completed and failed sagas older than the given duration.
	Cleanup(ctx context.Context, olderThan time.Duration) error
}
