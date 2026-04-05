-- ============================================================================
-- Saga Instances
-- ============================================================================

-- name: CreateSagaInstance :one
INSERT INTO multisub.saga_instances (saga_type, correlation_id, status, current_step, total_steps, state_data)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, saga_type, correlation_id, status, current_step, total_steps, state_data, error_message, created_at, updated_at;

-- name: UpdateSagaProgress :exec
UPDATE multisub.saga_instances
SET current_step = $2, state_data = $3, updated_at = now()
WHERE id = $1;

-- name: CompleteSaga :exec
UPDATE multisub.saga_instances
SET status = 'completed', updated_at = now()
WHERE id = $1;

-- name: FailSaga :exec
UPDATE multisub.saga_instances
SET status = 'failed', error_message = $2, updated_at = now()
WHERE id = $1;

-- name: GetRunningSagas :many
SELECT id, saga_type, correlation_id, status, current_step, total_steps, state_data, error_message, created_at, updated_at
FROM multisub.saga_instances
WHERE status = 'running'
ORDER BY created_at
LIMIT 1000;

-- name: GetSagaByCorrelation :one
SELECT id, saga_type, correlation_id, status, current_step, total_steps, state_data, error_message, created_at, updated_at
FROM multisub.saga_instances
WHERE saga_type = $1 AND correlation_id = $2;

-- name: CleanupOldSagas :exec
DELETE FROM multisub.saga_instances
WHERE status IN ('completed', 'failed') AND updated_at < $1;
