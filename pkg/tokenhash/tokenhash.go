// Package tokenhash derives a deterministic hash for bearer tokens (refresh
// tokens, password-reset / email-verification / invitation tokens) so they are
// stored and looked up by hash instead of plaintext. A database read (leaked
// backup, SQL injection, mis-scoped query) then yields only hashes, which are
// not replayable — the raw high-entropy token is never persisted.
package tokenhash

import (
	"crypto/sha256"
	"encoding/hex"
)

// Hash returns hex(SHA-256(token)). It matches Postgres
// encode(sha256(token::bytea), 'hex'), so the in-place migration that hashes
// existing rows stays consistent with application lookups.
func Hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
