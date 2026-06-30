package remnawave

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/circuitbreaker"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

func TestGatewayAdapter_CreateUser_AppliesSquads(t *testing.T) {
	var sentBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sentBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"response":{"uuid":"u1","shortUuid":"s1"}}`))
	}))
	defer srv.Close()
	mk := func(def []string) *GatewayAdapter {
		return NewGatewayAdapter(NewResilientClient(NewClient(srv.URL, "t"), circuitbreaker.DefaultConfig(), slog.Default()), clock.NewReal(), def)
	}
	_, err := mk([]string{"default-sq"}).CreateUser(context.Background(), multisub.CreateRemnawaveUserRequest{Username: "u"})
	require.NoError(t, err)
	require.Contains(t, string(sentBody), `"activeInternalSquads":["default-sq"]`)

	_, err = mk([]string{"default-sq"}).CreateUser(context.Background(), multisub.CreateRemnawaveUserRequest{Username: "u", ActiveInternalSquads: []string{"override-sq"}})
	require.NoError(t, err)
	require.Contains(t, string(sentBody), `"activeInternalSquads":["override-sq"]`)
}

func TestGatewayAdapter_GetUser_StatusMapping(t *testing.T) {
	cases := []struct {
		status      string
		wantEnabled bool
		wantExpired bool
	}{
		{RemnawaveStatusActive, true, false},
		{RemnawaveStatusDisabled, false, false},
		{RemnawaveStatusExpired, false, true},
		{RemnawaveStatusLimited, false, true},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `{"response":{"uuid":"u1","status":%q,"userTraffic":{"usedTrafficBytes":1024}}}`, tc.status)
		}))
		a := NewGatewayAdapter(NewResilientClient(NewClient(srv.URL, "t"), circuitbreaker.DefaultConfig(), slog.Default()), clock.NewReal(), nil)
		got, err := a.GetUser(context.Background(), "u1")
		require.NoError(t, err)
		require.Equal(t, tc.wantEnabled, got.Enabled, "status %s enabled", tc.status)
		require.Equal(t, tc.wantExpired, got.Expired, "status %s expired", tc.status)
		require.Equal(t, int64(1024), got.UsedBytes)
		srv.Close()
	}
}
