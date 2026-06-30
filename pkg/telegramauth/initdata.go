// Package telegramauth validates Telegram Mini App (WebApp) initData per
// https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
package telegramauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const DefaultInitDataMaxAge = 24 * time.Hour

var (
	ErrInvalidInitData = errors.New("telegramauth: invalid initData signature")
	ErrInitDataExpired = errors.New("telegramauth: initData expired")
)

type WebAppUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

func ValidateWebAppInitData(initData, botToken string, now time.Time, maxAge time.Duration) (*WebAppUser, error) {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return nil, ErrInvalidInitData
	}
	hash := values.Get("hash")
	if hash == "" {
		return nil, ErrInvalidInitData
	}
	values.Del("hash")

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(values.Get(k))
	}

	secretMAC := hmac.New(sha256.New, []byte("WebAppData"))
	secretMAC.Write([]byte(botToken))
	secret := secretMAC.Sum(nil)

	dataMAC := hmac.New(sha256.New, secret)
	dataMAC.Write([]byte(b.String()))
	expected := hex.EncodeToString(dataMAC.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(hash)) {
		return nil, ErrInvalidInitData
	}

	authDate, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil {
		return nil, ErrInvalidInitData
	}
	if now.Unix()-authDate > int64(maxAge.Seconds()) {
		return nil, ErrInitDataExpired
	}

	var u WebAppUser
	if err := json.Unmarshal([]byte(values.Get("user")), &u); err != nil || u.ID == 0 {
		return nil, ErrInvalidInitData
	}
	return &u, nil
}
