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

// TestHWIDDevice_2_8_0Shape verifies hwid/userId identity and the delete/create
// request wire shapes match the 2.8.0 contract.
func TestHWIDDevice_2_8_0Shape(t *testing.T) {
	// Response parse: hwid + numeric userId.
	var dev HWIDDevice
	require.NoError(t, json.Unmarshal([]byte(`{"uuid":"d-1","hwid":"HW-ABC","userId":42,"createdAt":"2026-07-01T00:00:00Z"}`), &dev))
	assert.Equal(t, "HW-ABC", dev.Hwid)
	assert.Equal(t, int64(42), dev.UserID)

	// List envelope {total, devices[]}.
	var list HWIDDeviceList
	require.NoError(t, json.Unmarshal([]byte(`{"total":1,"devices":[{"uuid":"d-1","hwid":"HW-ABC","userId":42}]}`), &list))
	assert.Equal(t, 1, list.Total)
	require.Len(t, list.Devices, 1)

	// Create/delete request wire shapes.
	cb, err := json.Marshal(CreateHWIDDeviceRequest{Hwid: "HW-ABC", UserUUID: "u-1"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"hwid":"HW-ABC","userUuid":"u-1"}`, string(cb))

	db, err := json.Marshal(DeleteHWIDDeviceRequest{Hwid: "HW-ABC", UserUUID: "u-1"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"hwid":"HW-ABC","userUuid":"u-1"}`, string(db))

	dab, err := json.Marshal(DeleteAllHWIDDevicesRequest{UserUUID: "u-1"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"userUuid":"u-1"}`, string(dab))
}

// TestDropConnections_DiscriminatedUnionShape locks the 2.8.0 drop-connections
// body: dropBy{by,userUuids} + targetNodes{target}, not the old flat userUuid.
func TestDropConnections_DiscriminatedUnionShape(t *testing.T) {
	b, err := json.Marshal(NewDropUserConnections("u-1"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"dropBy":{"by":"userUuids","userUuids":["u-1"]},"targetNodes":{"target":"allNodes"}}`, string(b))
	assert.NotContains(t, string(b), `"userUuid"`)
	assert.NotContains(t, string(b), `"nodeUuid"`)
}

// TestIPFetchResult_2_8_0Shape parses the nested fetch-ips-result envelope.
func TestIPFetchResult_2_8_0Shape(t *testing.T) {
	body := `{
		"isCompleted": true, "isFailed": false,
		"progress": {"total": 2, "completed": 2, "percent": 100},
		"result": {"success": true, "userUuid": "u-1", "userId": 42,
			"nodes": [{"nodeUuid":"n-1","nodeName":"eu","countryCode":"DE",
				"ips":[{"ip":"1.2.3.4","lastSeen":"2026-07-01T00:00:00Z"}]}]}
	}`
	var res IPFetchResult
	require.NoError(t, json.Unmarshal([]byte(body), &res))
	assert.True(t, res.IsCompleted)
	assert.Equal(t, 100, res.Progress.Percent)
	require.NotNil(t, res.Result)
	assert.Equal(t, int64(42), res.Result.UserID)
	require.Len(t, res.Result.Nodes, 1)
	require.Len(t, res.Result.Nodes[0].IPs, 1)
	assert.Equal(t, "1.2.3.4", res.Result.Nodes[0].IPs[0].IP)
}

// TestSystemStats_2_8_0Shape parses the nested /system/stats envelope.
func TestSystemStats_2_8_0Shape(t *testing.T) {
	body := `{"cpu":{"cores":8},"memory":{"total":100,"free":40,"used":60},
		"uptime":123,"users":{"statusCounts":{"ACTIVE":5},"totalUsers":9},
		"onlineStats":{"onlineNow":3},"nodes":{"totalOnline":2,"totalBytesLifetime":999}}`
	var s SystemStats
	require.NoError(t, json.Unmarshal([]byte(body), &s))
	assert.Equal(t, 5, s.Users.StatusCounts["ACTIVE"])
	assert.Equal(t, 2, s.Nodes.TotalOnline)
	assert.Equal(t, int64(999), s.Nodes.TotalBytesLifetime)
}
