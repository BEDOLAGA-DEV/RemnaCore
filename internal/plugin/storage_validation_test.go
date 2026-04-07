package plugin

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateStorageKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		err  error
	}{
		{"valid simple", "my-key", nil},
		{"valid with dots", "config.setting", nil},
		{"valid with dashes", "user-pref-theme", nil},
		{"valid with underscores", "cache_token_v2", nil},
		{"valid with colons", "session:abc123", nil},
		{"empty", "", ErrStorageKeyEmpty},
		{"path traversal", "../other-plugin/secret", ErrInvalidStorageKey},
		{"slash", "path/to/key", ErrInvalidStorageKey},
		{"backslash", "path\\to\\key", ErrInvalidStorageKey},
		{"null byte", "key\x00evil", ErrInvalidStorageKey},
		{"control char tab", "key\tvalue", ErrInvalidStorageKey},
		{"control char SOH", "key\x01value", ErrInvalidStorageKey},
		{"DEL char", "key\x7fvalue", ErrInvalidStorageKey},
		{"too long", strings.Repeat("a", MaxStorageKeyLen+1), ErrStorageKeyTooLong},
		{"max length", strings.Repeat("a", MaxStorageKeyLen), nil},
		{"double dots in middle", "foo..bar", ErrInvalidStorageKey},
		{"only double dots", "..", ErrInvalidStorageKey},
		{"triple dots allowed", "key...name", ErrInvalidStorageKey},
		{"single dot", "config.v1", nil},
		{"leading dot", ".hidden", nil},
		{"newline", "key\nvalue", ErrInvalidStorageKey},
		{"carriage return", "key\rvalue", ErrInvalidStorageKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStorageKey(tt.key)
			if tt.err == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tt.err)
			}
		})
	}
}

func TestStorageGet_InvalidKey(t *testing.T) {
	p := testPluginWithPerms([]PermissionScope{PermStorageRead})
	store := newTestStorageService()

	hf := newTestHostFunctions(t, p, nil)
	hf.Storage = store

	tests := []struct {
		name string
		key  string
		err  error
	}{
		{"empty key", "", ErrStorageKeyEmpty},
		{"path traversal", "../secret", ErrInvalidStorageKey},
		{"slash in key", "a/b", ErrInvalidStorageKey},
		{"null byte", "k\x00v", ErrInvalidStorageKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := hf.StorageGet(context.Background(), "test-plugin", tt.key)
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.err)
		})
	}
}

func TestStorageSet_InvalidKey(t *testing.T) {
	p := testPluginWithPerms([]PermissionScope{PermStorageWrite})
	store := newTestStorageService()

	hf := newTestHostFunctions(t, p, nil)
	hf.Storage = store

	tests := []struct {
		name string
		key  string
		err  error
	}{
		{"empty key", "", ErrStorageKeyEmpty},
		{"path traversal", "../secret", ErrInvalidStorageKey},
		{"backslash", "a\\b", ErrInvalidStorageKey},
		{"control char", "k\x01v", ErrInvalidStorageKey},
		{"too long", strings.Repeat("x", MaxStorageKeyLen+1), ErrStorageKeyTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hf.StorageSet(context.Background(), "test-plugin", tt.key, []byte("val"), 0)
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.err)
		})
	}
}

func TestStorageGet_ValidKeyPassesThrough(t *testing.T) {
	p := testPluginWithPerms([]PermissionScope{PermStorageRead})
	store := newTestStorageService()
	_ = store.Set(context.Background(), "test-plugin", "valid-key", []byte("data"), 0)

	hf := newTestHostFunctions(t, p, nil)
	hf.Storage = store

	val, err := hf.StorageGet(context.Background(), "test-plugin", "valid-key")
	require.NoError(t, err)
	assert.Equal(t, []byte("data"), val)
}

func TestStorageSet_ValidKeyPassesThrough(t *testing.T) {
	p := testPluginWithPerms([]PermissionScope{PermStorageWrite})
	store := newTestStorageService()

	hf := newTestHostFunctions(t, p, nil)
	hf.Storage = store

	err := hf.StorageSet(context.Background(), "test-plugin", "valid-key", []byte("data"), 0)
	require.NoError(t, err)

	// Verify the value was actually stored.
	val, getErr := store.Get(context.Background(), "test-plugin", "valid-key")
	require.NoError(t, getErr)
	assert.Equal(t, []byte("data"), val)
}
