package bothost

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
)

func TestRegistry_Call(t *testing.T) {
	r := NewRegistry()
	r.Register("echo", plugin.PermUsersRead, func(_ context.Context, _ *OpContext, args json.RawMessage) (json.RawMessage, error) {
		return args, nil
	})
	oc := &OpContext{TenantID: "t-1"}

	out, err := r.Call(context.Background(), oc, NewPermSet(plugin.PermUsersRead), "echo", json.RawMessage(`{"x":1}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"x":1}`, string(out))

	_, err = r.Call(context.Background(), oc, NewPermSet(), "echo", nil)
	require.ErrorIs(t, err, ErrPermissionDenied)

	_, err = r.Call(context.Background(), oc, NewPermSet(plugin.PermUsersRead), "nope", nil)
	require.ErrorIs(t, err, ErrUnknownOp)
}

func TestRegistry_Bind_NoPermRequired(t *testing.T) {
	r := NewRegistry()
	r.Register("ping", "", func(_ context.Context, _ *OpContext, _ json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`"pong"`), nil
	})
	host := r.Bind(&OpContext{TenantID: "t-1"}, NewPermSet())
	out, err := host.Call(context.Background(), "ping", nil)
	require.NoError(t, err)
	require.JSONEq(t, `"pong"`, string(out))
}

func TestPermSet_Has(t *testing.T) {
	s := NewPermSet(plugin.PermUsersRead, plugin.PermUsersWrite)
	require.True(t, s.Has(plugin.PermUsersRead))
	require.True(t, s.Has(plugin.PermUsersWrite))
	require.False(t, s.Has(plugin.PermBillingRead))
}
