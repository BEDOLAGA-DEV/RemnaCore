package remnawave

import (
	"context"
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
