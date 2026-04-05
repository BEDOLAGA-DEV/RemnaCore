-- ============================================================================
-- Migration 013: Persistent saga state for crash-safe orchestration
-- ============================================================================
-- Saga instances track multi-step workflows (provisioning, deprovisioning,
-- sync) so they can be resumed after a process restart. Each instance records
-- which step it reached and what compensation data is needed for rollback.
-- ============================================================================

CREATE TABLE IF NOT EXISTS multisub.saga_instances (
    id               UUID          NOT NULL DEFAULT gen_random_uuid(),
    saga_type        TEXT          NOT NULL,
    correlation_id   TEXT          NOT NULL,
    status           TEXT          NOT NULL DEFAULT 'running',
    current_step     INT           NOT NULL DEFAULT 0,
    total_steps      INT           NOT NULL DEFAULT 0,
    state_data       JSONB         NOT NULL DEFAULT '{}',
    error_message    TEXT,
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    UNIQUE (saga_type, correlation_id)
);

-- Index for finding running sagas on startup (resume after crash).
CREATE INDEX idx_saga_instances_running
    ON multisub.saga_instances (saga_type)
    WHERE status = 'running';

-- Index for cleanup of completed/failed sagas.
CREATE INDEX idx_saga_instances_completed_at
    ON multisub.saga_instances (updated_at)
    WHERE status IN ('completed', 'failed');

-- Reuse the identity.set_updated_at trigger function for automatic updated_at.
CREATE TRIGGER trigger_saga_instances_updated
    BEFORE UPDATE ON multisub.saga_instances
    FOR EACH ROW EXECUTE FUNCTION identity.set_updated_at();
