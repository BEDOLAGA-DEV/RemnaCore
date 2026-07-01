package secret

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestString_Expose(t *testing.T) {
	s := NewString("super-secret")
	assert.Equal(t, "super-secret", s.Expose())
}

func TestString_String(t *testing.T) {
	s := NewString("super-secret")
	assert.Equal(t, "***", s.String())
	assert.Equal(t, "***", fmt.Sprintf("%s", s))
	assert.Equal(t, "***", fmt.Sprintf("%v", s))
}

func TestString_GoString(t *testing.T) {
	s := NewString("super-secret")
	assert.Equal(t, "secret.String{***}", fmt.Sprintf("%#v", s))
}

func TestString_MarshalJSON(t *testing.T) {
	s := NewString("super-secret")
	data, err := json.Marshal(s)
	require.NoError(t, err)
	assert.Equal(t, `"***"`, string(data))
}

func TestString_MarshalJSON_InStruct(t *testing.T) {
	type cfg struct {
		Token String `json:"token"`
	}
	c := cfg{Token: NewString("super-secret")}
	data, err := json.Marshal(c)
	require.NoError(t, err)
	assert.Equal(t, `{"token":"***"}`, string(data))
}

func TestString_MarshalText(t *testing.T) {
	s := NewString("super-secret")
	text, err := s.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "***", string(text))
}

func TestString_UnmarshalText(t *testing.T) {
	var s String
	err := s.UnmarshalText([]byte("from-env"))
	require.NoError(t, err)
	assert.Equal(t, "from-env", s.Expose())
	assert.Equal(t, "***", s.String())
}

func TestString_LogValue(t *testing.T) {
	s := NewString("super-secret")
	val := s.LogValue()
	assert.Equal(t, slog.StringValue("***"), val)
}

func TestString_ZeroValue(t *testing.T) {
	var s String
	assert.Equal(t, "", s.Expose())
	assert.Equal(t, "***", s.String())
}
