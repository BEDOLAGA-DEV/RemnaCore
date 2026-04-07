package plugin

import (
	"fmt"
	"strings"
)

// MaxStorageKeyLen is the maximum allowed length for a plugin storage key.
const MaxStorageKeyLen = 256

var (
	// ErrInvalidStorageKey indicates that a storage key contains forbidden
	// characters such as path traversal sequences, path separators, null bytes,
	// or control characters.
	ErrInvalidStorageKey = fmt.Errorf("storage key contains invalid characters")

	// ErrStorageKeyTooLong indicates that a storage key exceeds MaxStorageKeyLen.
	ErrStorageKeyTooLong = fmt.Errorf("storage key exceeds maximum length of %d", MaxStorageKeyLen)

	// ErrStorageKeyEmpty indicates that an empty string was provided as a
	// storage key.
	ErrStorageKeyEmpty = fmt.Errorf("storage key must not be empty")
)

// minPrintableASCII is the lowest allowed rune value in a storage key.
// All characters below this (0x00-0x1F) are ASCII control characters.
const minPrintableASCII = 0x20

// deleteControlChar is the DEL control character (0x7F), which is also
// rejected in storage keys.
const deleteControlChar = 0x7f

// ValidateStorageKey checks that a plugin storage key is safe and well-formed.
// Keys must not contain path traversal sequences (..), path separators (/ \),
// null bytes, or control characters. Keys are limited to MaxStorageKeyLen bytes.
func ValidateStorageKey(key string) error {
	if key == "" {
		return ErrStorageKeyEmpty
	}
	if len(key) > MaxStorageKeyLen {
		return ErrStorageKeyTooLong
	}
	if strings.Contains(key, "..") {
		return ErrInvalidStorageKey
	}
	if strings.ContainsAny(key, "/\\") {
		return ErrInvalidStorageKey
	}
	for _, r := range key {
		if r < minPrintableASCII || r == deleteControlChar {
			return ErrInvalidStorageKey
		}
	}
	return nil
}
