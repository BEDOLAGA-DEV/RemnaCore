package identity

import (
	"context"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/aggregate"
)

// Read-only repository interfaces for CQRS preparation.
//
// These interfaces represent the query side of the identity persistence.
// They are subsets of the full Repository interface and exist to support a
// future CQRS split where reads go to a read replica and writes to the primary.
//
// The identity handler currently routes all reads through identity/service.Service
// (e.g., GetMe, ListUsers), which in turn calls the full Repository. When
// migrating to CQRS:
//  1. Query handlers (Me, ListUsers, GetUser) should depend on UserReader.
//  2. Command handlers (Register, Login, LinkTelegram) keep the full Repository.
//  3. Fx wiring binds UserReader to a read-replica-backed implementation.
//
// Design note: not embedded into Repository -- Go structural typing ensures
// any concrete type implementing Repository already satisfies UserReader.

// UserReader provides read-only access to platform users.
// Separated from Repository to support future CQRS split where reads go to a
// read replica and writes to the primary.
type UserReader interface {
	// GetUserByID retrieves a single user by their domain ID.
	GetUserByID(ctx context.Context, id string) (*aggregate.PlatformUser, error)

	// GetUserByEmail retrieves a user by their email address.
	GetUserByEmail(ctx context.Context, email string) (*aggregate.PlatformUser, error)

	// GetUserByTelegramID retrieves a user by their linked Telegram ID.
	GetUserByTelegramID(ctx context.Context, telegramID int64) (*aggregate.PlatformUser, error)

	// ListUsers retrieves a paginated list of all users (admin use).
	ListUsers(ctx context.Context, limit, offset int) ([]*aggregate.PlatformUser, error)
}
