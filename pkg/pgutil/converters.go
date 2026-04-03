// Package pgutil provides reusable pgx/pgtype conversion helpers shared
// across all PostgreSQL repository implementations.
package pgutil

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// UUIDToPgtype converts a domain UUID string to pgtype.UUID.
func UUIDToPgtype(s string) pgtype.UUID {
	u, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: u, Valid: true}
}

// PgtypeToUUID converts pgtype.UUID to a domain UUID string.
func PgtypeToUUID(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

// TimeToPgtype converts time.Time to pgtype.Timestamptz.
func TimeToPgtype(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// PgtypeToTime converts pgtype.Timestamptz to time.Time.
func PgtypeToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// OptStrToPgtypeUUID converts an optional domain UUID string pointer to pgtype.UUID.
func OptStrToPgtypeUUID(s *string) pgtype.UUID {
	if s == nil {
		return pgtype.UUID{}
	}
	return UUIDToPgtype(*s)
}

// PgtypeUUIDToOptStr converts pgtype.UUID to an optional string pointer.
func PgtypeUUIDToOptStr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := uuid.UUID(u.Bytes).String()
	return &s
}

// DerefStr dereferences a *string, returning empty string for nil.
func DerefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// StrPtrOrNil returns nil if s is empty, otherwise a pointer to s.
func StrPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// OptTimeToPgtype converts an optional *time.Time to pgtype.Timestamptz.
// Returns an invalid (null) Timestamptz when t is nil.
func OptTimeToPgtype(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// PgtypeToOptTime converts pgtype.Timestamptz to an optional *time.Time.
// Returns nil when the Timestamptz is not valid (SQL NULL).
func PgtypeToOptTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

// StringsToPgtypeUUIDs converts a slice of domain UUID strings to a slice of
// pgtype.UUID suitable for PostgreSQL UUID[] columns.
func StringsToPgtypeUUIDs(ss []string) []pgtype.UUID {
	out := make([]pgtype.UUID, len(ss))
	for i, s := range ss {
		out[i] = UUIDToPgtype(s)
	}
	return out
}

// PgtypeUUIDsToStrings converts a slice of pgtype.UUID to domain UUID strings.
func PgtypeUUIDsToStrings(us []pgtype.UUID) []string {
	out := make([]string, 0, len(us))
	for _, u := range us {
		if u.Valid {
			out = append(out, uuid.UUID(u.Bytes).String())
		}
	}
	return out
}

// MapErr maps pgx.ErrNoRows to notFoundErr and wraps other errors with the
// given operation context. The caller supplies the sentinel error for their
// bounded context (e.g. identity.ErrNotFound, billing.ErrNotFound).
func MapErr(err error, op string, notFoundErr error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", op, notFoundErr)
	}
	return fmt.Errorf("%s: %w", op, err)
}
