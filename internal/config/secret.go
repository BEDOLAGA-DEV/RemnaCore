package config

import "encoding/json"

// SecretString wraps a string that should not be accidentally logged or
// serialized. String(), MarshalJSON(), and MarshalText() all return a
// masked value. Use Expose() to retrieve the actual secret.
type SecretString struct {
	value string
}

const secretMask = "***"

// NewSecretString creates a SecretString wrapping the given value.
func NewSecretString(s string) SecretString {
	return SecretString{value: s}
}

// Expose returns the actual secret value. Use this only when the value
// is needed for authentication, HMAC verification, etc.
func (s SecretString) Expose() string {
	return s.value
}

// String implements fmt.Stringer with a masked output to prevent
// accidental logging.
func (s SecretString) String() string {
	return secretMask
}

// MarshalJSON returns "***" to prevent secret leakage in JSON output.
func (s SecretString) MarshalJSON() ([]byte, error) {
	return json.Marshal(secretMask)
}

// MarshalText returns "***" for text-based serialization (e.g., YAML, slog).
func (s SecretString) MarshalText() ([]byte, error) {
	return []byte(secretMask), nil
}

// UnmarshalText implements encoding.TextUnmarshaler so koanf can populate
// the field from environment variables.
func (s *SecretString) UnmarshalText(text []byte) error {
	s.value = string(text)
	return nil
}
