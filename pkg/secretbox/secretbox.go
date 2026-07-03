// Package secretbox provides authenticated symmetric encryption (AES-256-GCM)
// for secrets stored at rest (e.g. per-shop Telegram bot tokens).
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// hmacKeyLabel is the domain-separation label used to derive an HMAC key from
// the box key, so the HMAC and the AEAD never share raw key material.
const hmacKeyLabel = "secretbox:hmac-v1"

// KeySize is the required AES-256 key length in bytes.
const KeySize = 32

// ErrInvalidKey is returned when the key is not KeySize bytes.
var ErrInvalidKey = errors.New("secretbox: key must be 32 bytes")

// Box seals and opens secrets with AES-256-GCM, and derives deterministic HMAC
// tags (for equality/uniqueness checks over encrypted-at-rest values).
type Box struct {
	aead    cipher.AEAD
	hmacKey []byte
}

// New returns a Box for the given 32-byte key.
func New(key []byte) (*Box, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secretbox: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secretbox: new gcm: %w", err)
	}
	// Derive a separate HMAC key from the box key so the AEAD and HMAC never
	// share raw key material (key separation).
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(hmacKeyLabel))
	return &Box{aead: aead, hmacKey: mac.Sum(nil)}, nil
}

// HMAC returns a deterministic hex HMAC-SHA256 tag of s. Unlike Seal (random
// nonce → different ciphertext each call), HMAC yields the same tag for the same
// input, so it can back a UNIQUE index for detecting duplicate secrets without
// storing plaintext.
func (b *Box) HMAC(s string) string {
	mac := hmac.New(sha256.New, b.hmacKey)
	mac.Write([]byte(s))
	return hex.EncodeToString(mac.Sum(nil))
}

// Seal encrypts plaintext and returns base64(nonce ‖ ciphertext).
func (b *Box) Seal(plaintext string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("secretbox: nonce: %w", err)
	}
	ct := b.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Open decrypts a value produced by Seal.
func (b *Box) Open(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("secretbox: decode: %w", err)
	}
	ns := b.aead.NonceSize()
	if len(raw) < ns {
		return "", errors.New("secretbox: ciphertext too short")
	}
	nonce, ct := raw[:ns], raw[ns:]
	pt, err := b.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("secretbox: open: %w", err)
	}
	return string(pt), nil
}
