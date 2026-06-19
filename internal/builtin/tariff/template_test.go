package tariff

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
