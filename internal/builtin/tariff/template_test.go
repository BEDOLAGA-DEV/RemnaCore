package tariff

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

func TestTariffInput_IsTemplate_RoundTrips(t *testing.T) {
	in := defaultTariffInput()
	in.Name = "Platform Starter"
	in.PriceCurrency = "USD"
	in.DurationDays = 30
	in.IsTemplate = true

	data, err := json.Marshal(in)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"is_template":true`)

	var out TariffInput
	require.NoError(t, json.Unmarshal(data, &out))
	assert.True(t, out.IsTemplate)
}

func TestTariffInput_IsTemplate_DefaultsFalse(t *testing.T) {
	assert.False(t, defaultTariffInput().IsTemplate)
}

func TestHandler_isPlatformActor(t *testing.T) {
	h := &Handler{}

	platformCtx := tenantctx.WithTenantID(context.Background(), tenantctx.PlatformScopeSentinel)
	assert.True(t, h.isPlatformActor(platformCtx))

	shopCtx := tenantctx.WithTenantID(context.Background(), "11111111-1111-1111-1111-111111111111")
	assert.False(t, h.isPlatformActor(shopCtx))

	assert.False(t, h.isPlatformActor(context.Background()))
}
