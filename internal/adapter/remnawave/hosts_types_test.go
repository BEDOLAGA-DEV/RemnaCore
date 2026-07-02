package remnawave

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strptr(s string) *string { return &s }

// TestCreateHostRequest_WireShape locks the 2.8.0 create payload: a nested
// inbound object, no legacy inboundUuid/allowInsecure/requestHeader keys.
func TestCreateHostRequest_WireShape(t *testing.T) {
	req := CreateHostRequest{
		Remark:  "eu-1",
		Address: "1.2.3.4",
		Port:    443,
		Inbound: HostInbound{
			ConfigProfileUuid:        strptr("cp-1"),
			ConfigProfileInboundUuid: strptr("cpi-1"),
		},
		Tags: []string{"EU", "PREMIUM"},
	}

	b, err := json.Marshal(req)
	require.NoError(t, err)
	s := string(b)

	assert.Contains(t, s, `"inbound":{"configProfileUuid":"cp-1","configProfileInboundUuid":"cpi-1"}`)
	assert.Contains(t, s, `"tags":["EU","PREMIUM"]`)
	assert.NotContains(t, s, "inboundUuid")
	assert.NotContains(t, s, "allowInsecure")
	assert.NotContains(t, s, "requestHeader")
	assert.NotContains(t, s, "externalConfig")
}

// TestRemnawaveHost_Parse2_8_0Response verifies a 2.8.0 host response body with
// the nested inbound object and tags[] parses into the struct.
func TestRemnawaveHost_Parse2_8_0Response(t *testing.T) {
	body := `{
		"uuid":"h-1","remark":"eu-1","address":"1.2.3.4","port":443,
		"inbound":{"configProfileUuid":"cp-1","configProfileInboundUuid":"cpi-1"},
		"isDisabled":false,"tags":["EU"],
		"createdAt":"2026-07-01T00:00:00Z","updatedAt":"2026-07-01T00:00:00Z"
	}`

	var h RemnawaveHost
	require.NoError(t, json.Unmarshal([]byte(body), &h))

	assert.Equal(t, "h-1", h.UUID)
	require.NotNil(t, h.Inbound.ConfigProfileUuid)
	assert.Equal(t, "cp-1", *h.Inbound.ConfigProfileUuid)
	require.NotNil(t, h.Inbound.ConfigProfileInboundUuid)
	assert.Equal(t, "cpi-1", *h.Inbound.ConfigProfileInboundUuid)
	assert.Equal(t, []string{"EU"}, h.Tags)
}

// TestUpdateHostRequest_InboundOptional verifies inbound is omitted when nil and
// emitted when set (partial-update semantics).
func TestUpdateHostRequest_InboundOptional(t *testing.T) {
	noInbound, err := json.Marshal(UpdateHostRequest{UUID: "h-1", Remark: "renamed"})
	require.NoError(t, err)
	assert.NotContains(t, string(noInbound), "inbound")

	withInbound, err := json.Marshal(UpdateHostRequest{
		UUID:    "h-1",
		Inbound: &HostInbound{ConfigProfileUuid: strptr("cp-2")},
	})
	require.NoError(t, err)
	assert.Contains(t, string(withInbound), `"inbound":{"configProfileUuid":"cp-2","configProfileInboundUuid":null}`)
}

// TestNodeRestartRequest_AlwaysSendsForceRestart locks the 2.8.0 restart body:
// forceRestart is required and must always be present (no omitempty).
func TestNodeRestartRequest_AlwaysSendsForceRestart(t *testing.T) {
	b, err := json.Marshal(NodeRestartRequest{ForceRestart: false})
	require.NoError(t, err)
	assert.JSONEq(t, `{"forceRestart":false}`, string(b))

	b, err = json.Marshal(NodeRestartRequest{ForceRestart: true})
	require.NoError(t, err)
	assert.JSONEq(t, `{"forceRestart":true}`, string(b))
}

// TestCreateNodeRequest_2_8_0Fields verifies the node create payload uses
// consumptionMultiplier and configProfile, and drops the removed keys.
func TestCreateNodeRequest_2_8_0Fields(t *testing.T) {
	req := CreateNodeRequest{
		Name:                  "n-1",
		Address:               "1.2.3.4",
		Port:                  8443,
		ConsumptionMultiplier: 1.5,
		ConfigProfile: &NodeConfigProfile{
			ActiveConfigProfileUuid: "cp-1",
			ActiveInbounds:          []string{"in-1", "in-2"},
		},
	}
	b, err := json.Marshal(req)
	require.NoError(t, err)
	s := string(b)

	assert.Contains(t, s, `"consumptionMultiplier":1.5`)
	assert.Contains(t, s, `"configProfile":{"activeConfigProfileUuid":"cp-1","activeInbounds":["in-1","in-2"]}`)
	assert.NotContains(t, s, "consumptionFactor")
	assert.NotContains(t, s, "excludedInbounds")
	assert.NotContains(t, s, "externalRawConfig")
}
