package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/secret"
)

// SecretString is a type alias for secret.String; its full behavior is covered
// by pkg/secret. These smoke tests confirm the alias and constructor wire up
// correctly and that masking is preserved through the config entrypoints.

func TestSecretString_IsSecretStringAlias(t *testing.T) {
	var _ secret.String = NewSecretString("x") // compiles only if the alias holds
	assert.Equal(t, "super-secret", NewSecretString("super-secret").Expose())
}

func TestSecretString_MasksThroughConfig(t *testing.T) {
	type cfg struct {
		Token SecretString `json:"token"`
	}
	data, err := json.Marshal(cfg{Token: NewSecretString("super-secret")})
	require.NoError(t, err)
	assert.Equal(t, `{"token":"***"}`, string(data))
	assert.Equal(t, "***", NewSecretString("super-secret").String())
}
