package domainevent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testUpcaster is a configurable Upcaster for testing.
type testUpcaster struct {
	fromVersion int
	transform   func(map[string]any) map[string]any
}

func (u testUpcaster) FromVersion() int                      { return u.fromVersion }
func (u testUpcaster) Upcast(data map[string]any) map[string]any { return u.transform(data) }

func TestSchemaRegistry_NoUpcasters(t *testing.T) {
	registry := NewSchemaRegistry()

	event := New("user.registered", map[string]any{"user_id": "u-1"})

	got := registry.Upcast(event)

	assert.Equal(t, event.Type, got.Type)
	assert.Equal(t, event.Version, got.Version)
	assert.Equal(t, event.Data, got.Data)
}

func TestSchemaRegistry_SingleUpcast(t *testing.T) {
	registry := NewSchemaRegistry()

	// v1->v2: rename "name" to "full_name"
	registry.Register("user.registered", testUpcaster{
		fromVersion: 1,
		transform: func(data map[string]any) map[string]any {
			out := make(map[string]any, len(data))
			for k, v := range data {
				out[k] = v
			}
			out["full_name"] = out["name"]
			delete(out, "name")
			return out
		},
	})

	event := Event{
		Type:    "user.registered",
		Version: 1,
		Data:    map[string]any{"name": "Alice", "email": "alice@example.com"},
	}

	got := registry.Upcast(event)

	assert.Equal(t, 2, got.Version)
	data := got.Data.(map[string]any)
	assert.Equal(t, "Alice", data["full_name"])
	assert.NotContains(t, data, "name")
	assert.Equal(t, "alice@example.com", data["email"])
}

func TestSchemaRegistry_ChainedUpcast(t *testing.T) {
	registry := NewSchemaRegistry()

	// v1->v2: rename "name" to "full_name"
	registry.Register("user.registered", testUpcaster{
		fromVersion: 1,
		transform: func(data map[string]any) map[string]any {
			out := make(map[string]any, len(data))
			for k, v := range data {
				out[k] = v
			}
			out["full_name"] = out["name"]
			delete(out, "name")
			return out
		},
	})

	// v2->v3: add default "role" field
	registry.Register("user.registered", testUpcaster{
		fromVersion: 2,
		transform: func(data map[string]any) map[string]any {
			out := make(map[string]any, len(data)+1)
			for k, v := range data {
				out[k] = v
			}
			out["role"] = "member"
			return out
		},
	})

	event := Event{
		Type:    "user.registered",
		Version: 1,
		Data:    map[string]any{"name": "Alice", "email": "alice@example.com"},
	}

	got := registry.Upcast(event)

	assert.Equal(t, 3, got.Version)
	data := got.Data.(map[string]any)
	assert.Equal(t, "Alice", data["full_name"])
	assert.NotContains(t, data, "name")
	assert.Equal(t, "alice@example.com", data["email"])
	assert.Equal(t, "member", data["role"])
}

func TestSchemaRegistry_AlreadyLatest(t *testing.T) {
	registry := NewSchemaRegistry()

	// Register v1->v2 and v2->v3 upcasters.
	registry.Register("user.registered", testUpcaster{
		fromVersion: 1,
		transform: func(data map[string]any) map[string]any {
			data["upgraded_v1"] = true
			return data
		},
	})
	registry.Register("user.registered", testUpcaster{
		fromVersion: 2,
		transform: func(data map[string]any) map[string]any {
			data["upgraded_v2"] = true
			return data
		},
	})

	// Event already at v3 (latest) — should pass through unchanged.
	event := Event{
		Type:    "user.registered",
		Version: 3,
		Data:    map[string]any{"original": true},
	}

	got := registry.Upcast(event)

	assert.Equal(t, 3, got.Version)
	data := got.Data.(map[string]any)
	assert.True(t, data["original"].(bool))
	assert.NotContains(t, data, "upgraded_v1")
	assert.NotContains(t, data, "upgraded_v2")
}

func TestSchemaRegistry_NonMapData(t *testing.T) {
	registry := NewSchemaRegistry()

	registry.Register("user.registered", testUpcaster{
		fromVersion: 1,
		transform: func(data map[string]any) map[string]any {
			data["should_not_appear"] = true
			return data
		},
	})

	// Event with typed struct data (not map[string]any) — should pass through.
	event := Event{
		Type:    "user.registered",
		Version: 1,
		Data:    testPayload{UserID: "u-1", Email: "test@example.com"},
	}

	got := registry.Upcast(event)

	// Version unchanged, data unchanged.
	assert.Equal(t, 1, got.Version)
	typed, ok := got.Data.(testPayload)
	require.True(t, ok)
	assert.Equal(t, "u-1", typed.UserID)
}

func TestSchemaRegistry_LatestVersion(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*SchemaRegistry)
		eventType  EventType
		wantLatest int
	}{
		{
			name:       "no upcasters returns default version",
			setup:      func(_ *SchemaRegistry) {},
			eventType:  "user.registered",
			wantLatest: DefaultEventVersion,
		},
		{
			name: "single upcaster v1->v2",
			setup: func(r *SchemaRegistry) {
				r.Register("user.registered", testUpcaster{
					fromVersion: 1,
					transform:   func(d map[string]any) map[string]any { return d },
				})
			},
			eventType:  "user.registered",
			wantLatest: 2,
		},
		{
			name: "chain v1->v2->v3->v4",
			setup: func(r *SchemaRegistry) {
				noop := func(d map[string]any) map[string]any { return d }
				r.Register("sub.activated", testUpcaster{fromVersion: 1, transform: noop})
				r.Register("sub.activated", testUpcaster{fromVersion: 2, transform: noop})
				r.Register("sub.activated", testUpcaster{fromVersion: 3, transform: noop})
			},
			eventType:  "sub.activated",
			wantLatest: 4,
		},
		{
			name: "different event type returns default",
			setup: func(r *SchemaRegistry) {
				r.Register("user.registered", testUpcaster{
					fromVersion: 1,
					transform:   func(d map[string]any) map[string]any { return d },
				})
			},
			eventType:  "invoice.paid",
			wantLatest: DefaultEventVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewSchemaRegistry()
			tt.setup(registry)

			got := registry.LatestVersion(tt.eventType)
			assert.Equal(t, tt.wantLatest, got)
		})
	}
}

func TestSchemaRegistry_DuplicatePanics(t *testing.T) {
	registry := NewSchemaRegistry()

	noop := func(d map[string]any) map[string]any { return d }
	registry.Register("user.registered", testUpcaster{fromVersion: 1, transform: noop})

	assert.Panics(t, func() {
		registry.Register("user.registered", testUpcaster{fromVersion: 1, transform: noop})
	})
}

func TestSchemaRegistry_PartialUpcast_FromV2(t *testing.T) {
	registry := NewSchemaRegistry()

	// Register v1->v2 and v2->v3.
	registry.Register("user.registered", testUpcaster{
		fromVersion: 1,
		transform: func(data map[string]any) map[string]any {
			data["v1_applied"] = true
			return data
		},
	})
	registry.Register("user.registered", testUpcaster{
		fromVersion: 2,
		transform: func(data map[string]any) map[string]any {
			data["v2_applied"] = true
			return data
		},
	})

	// Event at v2 — only the v2->v3 upcaster should run.
	event := Event{
		Type:    "user.registered",
		Version: 2,
		Data:    map[string]any{"original": true},
	}

	got := registry.Upcast(event)

	assert.Equal(t, 3, got.Version)
	data := got.Data.(map[string]any)
	assert.True(t, data["original"].(bool))
	assert.NotContains(t, data, "v1_applied")
	assert.True(t, data["v2_applied"].(bool))
}

func TestSchemaRegistry_MultiStepUpcast_EndToEnd(t *testing.T) {
	// Simulates a realistic evolution: subscription.activated evolves through
	// three schema versions:
	//   v1: {subscription_id, user_id}
	//   v2: {subscription_id, user_id, plan_tier}        (added plan_tier)
	//   v3: {subscription_id, user_id, plan_tier, region} (added region)
	tests := []struct {
		name        string
		startVer    int
		data        map[string]any
		wantVersion int
		wantFields  map[string]any
	}{
		{
			name:     "v1 event upcasts through v2 and v3",
			startVer: 1,
			data: map[string]any{
				"subscription_id": "sub-1",
				"user_id":         "u-1",
			},
			wantVersion: 3,
			wantFields: map[string]any{
				"subscription_id": "sub-1",
				"user_id":         "u-1",
				"plan_tier":       "standard",
				"region":          "auto",
			},
		},
		{
			name:     "v2 event upcasts only through v3",
			startVer: 2,
			data: map[string]any{
				"subscription_id": "sub-2",
				"user_id":         "u-2",
				"plan_tier":       "premium",
			},
			wantVersion: 3,
			wantFields: map[string]any{
				"subscription_id": "sub-2",
				"user_id":         "u-2",
				"plan_tier":       "premium",
				"region":          "auto",
			},
		},
		{
			name:     "v3 event passes through unchanged",
			startVer: 3,
			data: map[string]any{
				"subscription_id": "sub-3",
				"user_id":         "u-3",
				"plan_tier":       "enterprise",
				"region":          "eu-west",
			},
			wantVersion: 3,
			wantFields: map[string]any{
				"subscription_id": "sub-3",
				"user_id":         "u-3",
				"plan_tier":       "enterprise",
				"region":          "eu-west",
			},
		},
	}

	registry := NewSchemaRegistry()

	// v1->v2: add plan_tier with default
	registry.Register("subscription.activated", testUpcaster{
		fromVersion: 1,
		transform: func(data map[string]any) map[string]any {
			if _, ok := data["plan_tier"]; !ok {
				data["plan_tier"] = "standard"
			}
			return data
		},
	})

	// v2->v3: add region with default
	registry.Register("subscription.activated", testUpcaster{
		fromVersion: 2,
		transform: func(data map[string]any) map[string]any {
			if _, ok := data["region"]; !ok {
				data["region"] = "auto"
			}
			return data
		},
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := Event{
				Type:    "subscription.activated",
				Version: tt.startVer,
				Data:    tt.data,
			}

			got := registry.Upcast(event)

			assert.Equal(t, tt.wantVersion, got.Version)
			data := got.Data.(map[string]any)
			for k, v := range tt.wantFields {
				assert.Equal(t, v, data[k], "field %s mismatch", k)
			}
		})
	}

	// Verify LatestVersion reflects the full chain.
	assert.Equal(t, 3, registry.LatestVersion("subscription.activated"))
}

func TestSchemaRegistry_Validate(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*SchemaRegistry)
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "empty registry is valid",
			setup:   func(_ *SchemaRegistry) {},
			wantErr: false,
		},
		{
			name: "continuous chain v1->v2->v3 is valid",
			setup: func(r *SchemaRegistry) {
				noop := func(d map[string]any) map[string]any { return d }
				r.Register("sub.activated", testUpcaster{fromVersion: 1, transform: noop})
				r.Register("sub.activated", testUpcaster{fromVersion: 2, transform: noop})
			},
			wantErr: false,
		},
		{
			name: "single upcaster v1->v2 is valid",
			setup: func(r *SchemaRegistry) {
				noop := func(d map[string]any) map[string]any { return d }
				r.Register("user.registered", testUpcaster{fromVersion: 1, transform: noop})
			},
			wantErr: false,
		},
		{
			name: "gap: missing v1->v2, only v2->v3 registered",
			setup: func(r *SchemaRegistry) {
				noop := func(d map[string]any) map[string]any { return d }
				r.Register("sub.activated", testUpcaster{fromVersion: 2, transform: noop})
			},
			wantErr:   true,
			errSubstr: "upcaster chain gap for sub.activated",
		},
		{
			name: "gap: v1->v2 and v3->v4, missing v2->v3",
			setup: func(r *SchemaRegistry) {
				noop := func(d map[string]any) map[string]any { return d }
				r.Register("sub.activated", testUpcaster{fromVersion: 1, transform: noop})
				r.Register("sub.activated", testUpcaster{fromVersion: 3, transform: noop})
			},
			wantErr:   true,
			errSubstr: "upcaster chain gap for sub.activated",
		},
		{
			name: "multiple event types independently valid",
			setup: func(r *SchemaRegistry) {
				noop := func(d map[string]any) map[string]any { return d }
				r.Register("sub.activated", testUpcaster{fromVersion: 1, transform: noop})
				r.Register("user.registered", testUpcaster{fromVersion: 1, transform: noop})
				r.Register("user.registered", testUpcaster{fromVersion: 2, transform: noop})
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewSchemaRegistry()
			tt.setup(registry)

			err := registry.Validate()

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSchemaRegistry_OutOfOrderRegistration(t *testing.T) {
	registry := NewSchemaRegistry()

	// Register v2->v3 first, then v1->v2. The registry should sort them.
	registry.Register("user.registered", testUpcaster{
		fromVersion: 2,
		transform: func(data map[string]any) map[string]any {
			out := make(map[string]any, len(data)+1)
			for k, v := range data {
				out[k] = v
			}
			out["step2"] = true
			return out
		},
	})
	registry.Register("user.registered", testUpcaster{
		fromVersion: 1,
		transform: func(data map[string]any) map[string]any {
			out := make(map[string]any, len(data)+1)
			for k, v := range data {
				out[k] = v
			}
			out["step1"] = true
			return out
		},
	})

	event := Event{
		Type:    "user.registered",
		Version: 1,
		Data:    map[string]any{"original": true},
	}

	got := registry.Upcast(event)

	assert.Equal(t, 3, got.Version)
	data := got.Data.(map[string]any)
	assert.True(t, data["step1"].(bool))
	assert.True(t, data["step2"].(bool))
}
