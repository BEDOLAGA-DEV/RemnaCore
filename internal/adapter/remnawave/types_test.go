package remnawave

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemnawaveUserWithTraffic_NestedUserTraffic(t *testing.T) {
	const body = `{"uuid":"u1","status":"ACTIVE","lastTrafficResetAt":"2026-01-01T00:00:00Z",
		"userTraffic":{"usedTrafficBytes":2048,"lifetimeUsedTrafficBytes":4096,"onlineAt":null,"firstConnectedAt":null,"lastConnectedNodeUuid":null}}`
	var u RemnawaveUserWithTraffic
	require.NoError(t, json.Unmarshal([]byte(body), &u))
	require.Equal(t, "u1", u.UUID)
	require.Equal(t, int64(2048), u.UsedTrafficBytesInt())
	require.Equal(t, float64(4096), u.UserTraffic.LifetimeUsedTrafficBytes)
	require.Nil(t, u.UserTraffic.OnlineAt)
	require.Nil(t, u.UserTraffic.LastConnectedNodeUuid)
}
