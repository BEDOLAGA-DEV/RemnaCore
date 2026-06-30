package telegramauth_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/telegramauth"
)

func buildInitData(t *testing.T, token string, authDate int64, userJSON string) string {
	t.Helper()
	v := url.Values{}
	v.Set("auth_date", strconv.FormatInt(authDate, 10))
	v.Set("user", userJSON)
	// data_check_string = sorted "k=v" joined by \n
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(k + "=" + v.Get(k))
	}
	secret := hmacSHA256([]byte("WebAppData"), []byte(token))
	h := hex.EncodeToString(hmacSHA256(secret, []byte(b.String())))
	v.Set("hash", h)
	return v.Encode()
}

func hmacSHA256(key, msg []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(msg)
	return m.Sum(nil)
}

func TestValidateWebAppInitData(t *testing.T) {
	const token = "123456:test-bot-token"
	now := time.Unix(1_700_000_000, 0)
	userJSON := `{"id":777,"first_name":"Al","username":"al"}`
	good := buildInitData(t, token, now.Unix()-10, userJSON)
	u, err := telegramauth.ValidateWebAppInitData(good, token, now, time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(777), u.ID)
	require.Equal(t, "al", u.Username)

	// tampered hash
	_, err = telegramauth.ValidateWebAppInitData(good+"x", token, now, time.Hour)
	require.ErrorIs(t, err, telegramauth.ErrInvalidInitData)

	// wrong token
	_, err = telegramauth.ValidateWebAppInitData(good, "999:other", now, time.Hour)
	require.ErrorIs(t, err, telegramauth.ErrInvalidInitData)

	// expired
	old := buildInitData(t, token, now.Unix()-7200, userJSON)
	_, err = telegramauth.ValidateWebAppInitData(old, token, now, time.Hour)
	require.ErrorIs(t, err, telegramauth.ErrInitDataExpired)
}
