// Package telegramauthtest provides test helpers for building signed Telegram
// WebApp initData strings. Shared by every test that exercises initData
// validation so the HMAC signing scheme is implemented exactly once.
package telegramauthtest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// BuildInitData returns a url-encoded initData string containing auth_date,
// user, and a valid hash signed with botToken per the Telegram WebApp scheme.
func BuildInitData(t *testing.T, botToken string, authDate int64, userJSON string) string {
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

	secret := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	v.Set("hash", hex.EncodeToString(hmacSHA256(secret, []byte(b.String()))))
	return v.Encode()
}

func hmacSHA256(key, msg []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(msg)
	return m.Sum(nil)
}
