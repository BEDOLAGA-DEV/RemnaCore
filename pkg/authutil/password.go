package authutil

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	ArgonTime     = 1          // number of iterations
	ArgonMemoryKB = 64 * 1024  // 64 MB in KB
	ArgonThreads  = 4          // parallelism
	ArgonKeyLen   = 32         // output key length in bytes
	ArgonSaltLen  = 16         // salt length in bytes
	argonVersion  = 19         // argon2 version identifier
	argonPrefix   = "$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s"
)

// HashPassword derives an argon2id hash and returns the encoded string:
// $argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>
func HashPassword(password string) (string, error) {
	salt := make([]byte, ArgonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, ArgonTime, ArgonMemoryKB, ArgonThreads, ArgonKeyLen)

	return encodeHash(salt, hash), nil
}

// VerifyPassword parses the encoded hash, re-derives the key, and performs a
// timing-safe comparison.
func VerifyPassword(password, encoded string) (bool, error) {
	salt, storedHash, params, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}

	derived := argon2.IDKey(
		[]byte(password),
		salt,
		params.time,
		params.memory,
		params.threads,
		uint32(len(storedHash)),
	)

	return subtle.ConstantTimeCompare(derived, storedHash) == 1, nil
}

// argonParams holds the extracted parameters from an encoded hash string.
type argonParams struct {
	memory  uint32
	time    uint32
	threads uint8
}

// encodeHash produces the canonical argon2id encoded string.
func encodeHash(salt, hash []byte) string {
	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	hashB64 := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf(argonPrefix, argonVersion, ArgonMemoryKB, ArgonTime, ArgonThreads, saltB64, hashB64)
}

// decodeHash parses the canonical argon2id encoded string, extracting all
// parameters from the string itself (no hardcoded defaults).
func decodeHash(encoded string) (salt, hash []byte, params argonParams, err error) {
	parts := strings.Split(encoded, "$")
	// Expected: ["", "argon2id", "v=19", "m=...,t=...,p=...", "<salt>", "<hash>"]
	if len(parts) != 6 {
		return nil, nil, params, errors.New("invalid encoded hash format")
	}

	if parts[1] != "argon2id" {
		return nil, nil, params, fmt.Errorf("unsupported algorithm: %s", parts[1])
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return nil, nil, params, fmt.Errorf("parsing version: %w", err)
	}

	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return nil, nil, params, fmt.Errorf("parsing parameters: %w", err)
	}
	params = argonParams{memory: m, time: t, threads: p}

	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, params, fmt.Errorf("decoding salt: %w", err)
	}

	hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, params, fmt.Errorf("decoding hash: %w", err)
	}

	return salt, hash, params, nil
}
