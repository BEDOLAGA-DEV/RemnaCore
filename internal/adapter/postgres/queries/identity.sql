-- name: CreateUser :exec
INSERT INTO identity.platform_users (id, email, password_hash, display_name, email_verified, telegram_id, role, tenant_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: GetUserByID :one
SELECT id, email, password_hash, display_name, email_verified, telegram_id, role, tenant_id, created_at, updated_at
FROM identity.platform_users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, display_name, email_verified, telegram_id, role, tenant_id, created_at, updated_at
FROM identity.platform_users WHERE lower(email) = lower($1);

-- name: UpdateUser :exec
UPDATE identity.platform_users
SET email = $2, password_hash = $3, display_name = $4, email_verified = $5, telegram_id = $6, role = $7, tenant_id = $8
WHERE id = $1;

-- name: CreateSession :exec
INSERT INTO identity.sessions (id, user_id, refresh_token, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: GetSessionByRefreshToken :one
SELECT id, user_id, refresh_token, expires_at, created_at
FROM identity.sessions WHERE refresh_token = $1;

-- name: DeleteSession :exec
DELETE FROM identity.sessions WHERE id = $1;

-- name: DeleteUserSessions :exec
DELETE FROM identity.sessions WHERE user_id = $1;

-- name: CreateEmailVerification :exec
INSERT INTO identity.email_verifications (id, user_id, email, token, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetEmailVerification :one
SELECT id, user_id, email, token, expires_at, created_at
FROM identity.email_verifications WHERE token = $1;

-- name: DeleteEmailVerification :exec
DELETE FROM identity.email_verifications WHERE id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM identity.sessions WHERE expires_at < now();

-- name: DeleteExpiredVerifications :exec
DELETE FROM identity.email_verifications WHERE expires_at < now();

-- name: GetUserByTelegramID :one
SELECT id, email, password_hash, display_name, email_verified, telegram_id, role, tenant_id, created_at, updated_at
FROM identity.platform_users WHERE telegram_id = $1;

-- name: ListUsers :many
SELECT id, email, password_hash, display_name, email_verified, telegram_id, role, tenant_id, created_at, updated_at
FROM identity.platform_users
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CreatePasswordReset :exec
INSERT INTO identity.password_resets (id, user_id, email, token, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetPasswordResetByToken :one
SELECT id, user_id, email, token, expires_at, created_at
FROM identity.password_resets WHERE token = $1;

-- name: DeletePasswordReset :exec
DELETE FROM identity.password_resets WHERE id = $1;

-- name: DeleteUserPasswordResets :exec
DELETE FROM identity.password_resets WHERE user_id = $1;
