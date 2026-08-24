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
		return NewGatewayAdapter(NewResilientClient(NewClient(srv.URL, "t"), circuitbreaker.DefaultConfig(), slog.Default()), clock.NewReal(), def, slog.Default())
	}
	_, err := mk([]string{"default-sq"}).CreateUser(context.Background(), multisub.CreateRemnawaveUserRequest{Username: "u"})
	require.NoError(t, err)
	require.Contains(t, string(sentBody), `"activeInternalSquads":["default-sq"]`)

	_, err = mk([]string{"default-sq"}).CreateUser(context.Background(), multisub.CreateRemnawaveUserRequest{Username: "u", ActiveInternalSquads: []string{"override-sq"}})
	require.NoError(t, err)
	require.Contains(t, string(sentBody), `"activeInternalSquads":["override-sq"]`)
	require.NotContains(t, string(sentBody), `"default-sq"`)
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
			fmt.Fprintf(w, `{"response":{"id":1,"status":%q,"userTraffic":{"usedTrafficBytes":1024}}}`, tc.status)
		}))
		a := NewGatewayAdapter(NewResilientClient(NewClient(srv.URL, "t"), circuitbreaker.DefaultConfig(), slog.Default()), clock.NewReal(), nil, slog.Default())
		got, err := a.GetUser(context.Background(), "1")
		require.NoError(t, err)
		require.Equal(t, tc.wantEnabled, got.Enabled, "status %s enabled", tc.status)
		require.Equal(t, tc.wantExpired, got.Expired, "status %s expired", tc.status)
		require.Equal(t, int64(1024), got.UsedBytes)
		srv.Close()
	}
}

func TestGatewayAdapter_AssignToSquad_HitsManyUsersEndpoint(t *testing.T) {
	var gotPath string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"response":{}}`))
	}))
	defer srv.Close()

	a := NewGatewayAdapter(NewResilientClient(NewClient(srv.URL, "t"), circuitbreaker.DefaultConfig(), slog.Default()), clock.NewReal(), nil, slog.Default())
	err := a.AssignToSquad(context.Background(), "42", "squad-uuid-9")
	require.NoError(t, err)
	// The bare add-users endpoint would enrol EVERY user in the squad, so the
	// per-user variant is the only correct target.
	require.Equal(t, "/api/internal-squads/squad-uuid-9/bulk-actions/add-many-users", gotPath)
	require.Contains(t, string(gotBody), `"userIds":[42]`)
}

// A binding created before the Remnawave 3 migration holds a UUID, which the
// v3 API cannot address. That must surface as an error, not a silent no-op.
func TestGatewayAdapter_AssignToSquad_RejectsPreV3Ref(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no request expected, got %s", r.URL.Path)
	}))
	defer srv.Close()

	a := NewGatewayAdapter(NewResilientClient(NewClient(srv.URL, "t"), circuitbreaker.DefaultConfig(), slog.Default()), clock.NewReal(), nil, slog.Default())
	err := a.AssignToSquad(context.Background(), "user-uuid-1", "squad-uuid-9")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a numeric id")
}
