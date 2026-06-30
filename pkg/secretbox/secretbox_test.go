package secretbox_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/secretbox"
)

func TestBox_SealOpen_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key { key[i] = byte(i) }
	b, err := secretbox.New(key)
	require.NoError(t, err)
	enc, err := b.Seal("123456:bot-token-secret")
	require.NoError(t, err)
	require.NotContains(t, enc, "bot-token-secret") // ciphertext, not plaintext
	got, err := b.Open(enc)
	require.NoError(t, err)
	require.Equal(t, "123456:bot-token-secret", got)
}

func TestBox_New_RejectsWrongKeyLen(t *testing.T) {
	_, err := secretbox.New(make([]byte, 16))
	require.Error(t, err)
}

func TestBox_Open_TamperAndWrongKey(t *testing.T) {
	k1 := make([]byte, 32); k2 := make([]byte, 32); k2[0] = 1
	b1, _ := secretbox.New(k1)
	b2, _ := secretbox.New(k2)
	enc, _ := b1.Seal("x")
	_, err := b2.Open(enc)
	require.Error(t, err) // wrong key
	_, err = b1.Open(enc + "AA")
	require.Error(t, err) // tampered
}

func TestBox_Seal_NonceUniqueness(t *testing.T) {
	b, _ := secretbox.New(make([]byte, 32))
	a, _ := b.Seal("same"); c, _ := b.Seal("same")
	require.NotEqual(t, a, c) // random nonce
}
