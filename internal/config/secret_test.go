package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretString_Expose(t *testing.T) {
	s := NewSecretString("super-secret")
	assert.Equal(t, "super-secret", s.Expose())
}

func TestSecretString_String(t *testing.T) {
	s := NewSecretString("super-secret")
	assert.Equal(t, "***", s.String())
	assert.Equal(t, "***", fmt.Sprintf("%s", s))
	assert.Equal(t, "***", fmt.Sprintf("%v", s))
}

func TestSecretString_GoString(t *testing.T) {
	s := NewSecretString("super-secret")
	assert.Equal(t, "config.SecretString{***}", fmt.Sprintf("%#v", s))
}

func TestSecretString_MarshalJSON(t *testing.T) {
	s := NewSecretString("super-secret")
	data, err := json.Marshal(s)
	require.NoError(t, err)
	assert.Equal(t, `"***"`, string(data))
}

func TestSecretString_MarshalJSON_InStruct(t *testing.T) {
	type cfg struct {
		Token SecretString `json:"token"`
	}
	c := cfg{Token: NewSecretString("super-secret")}
	data, err := json.Marshal(c)
	require.NoError(t, err)
	assert.Equal(t, `{"token":"***"}`, string(data))
}

func TestSecretString_MarshalText(t *testing.T) {
	s := NewSecretString("super-secret")
	text, err := s.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "***", string(text))
}

func TestSecretString_UnmarshalText(t *testing.T) {
	var s SecretString
	err := s.UnmarshalText([]byte("from-env"))
	require.NoError(t, err)
	assert.Equal(t, "from-env", s.Expose())
	assert.Equal(t, "***", s.String())
}

func TestSecretString_LogValue(t *testing.T) {
	s := NewSecretString("super-secret")
	val := s.LogValue()
	assert.Equal(t, slog.StringValue("***"), val)
}

func TestSecretString_ZeroValue(t *testing.T) {
	var s SecretString
	assert.Equal(t, "", s.Expose())
	assert.Equal(t, "***", s.String())
}
