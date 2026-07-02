package telegramauth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/telegramauth"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/telegramauth/telegramauthtest"
)

func TestValidateWebAppInitData(t *testing.T) {
	const token = "123456:test-bot-token"
	now := time.Unix(1_700_000_000, 0)
	userJSON := `{"id":777,"first_name":"Al","username":"al"}`
	good := telegramauthtest.BuildInitData(t, token, now.Unix()-10, userJSON)
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
	old := telegramauthtest.BuildInitData(t, token, now.Unix()-7200, userJSON)
	_, err = telegramauth.ValidateWebAppInitData(old, token, now, time.Hour)
	require.ErrorIs(t, err, telegramauth.ErrInitDataExpired)
}
