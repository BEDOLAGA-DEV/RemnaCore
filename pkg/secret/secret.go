// Package secret provides String, a string wrapper that masks its value in
// logs, errors, and serialization to prevent accidental leakage of sensitive
// data (tokens, keys). It lives in pkg so any layer — including domain packages
// that must not import internal/config — can hold secrets safely. Use Expose()
// to retrieve the actual value only where it is genuinely needed (auth, HMAC).
package secret

import (
	"encoding/json"
	"log/slog"
)

// String wraps a string that should not be accidentally logged or serialized.
// String(), MarshalJSON(), and MarshalText() all return a masked value.
type String struct {
	value string
}

// Mask is the masked representation returned by String/MarshalJSON/MarshalText.
const Mask = "***"

// NewString creates a String wrapping the given value.
func NewString(s string) String {
	return String{value: s}
}

// Expose returns the actual secret value. Use this only when the value is
// needed for authentication, HMAC verification, etc.
func (s String) Expose() string {
	return s.value
}

// String implements fmt.Stringer with a masked output to prevent accidental
// logging.
func (s String) String() string {
	return Mask
}

// MarshalJSON returns the mask to prevent secret leakage in JSON output.
func (s String) MarshalJSON() ([]byte, error) {
	return json.Marshal(Mask)
}

// MarshalText returns the mask for text-based serialization (e.g. YAML, slog).
func (s String) MarshalText() ([]byte, error) {
	return []byte(Mask), nil
}

// GoString implements fmt.GoStringer to prevent secret leakage via %#v.
func (s String) GoString() string {
	return "secret.String{" + Mask + "}"
}

// LogValue implements slog.LogValuer to mask the secret in structured logs.
func (s String) LogValue() slog.Value {
	return slog.StringValue(Mask)
}

// UnmarshalText implements encoding.TextUnmarshaler so config loaders (koanf)
// can populate the value from environment variables.
func (s *String) UnmarshalText(text []byte) error {
	s.value = string(text)
	return nil
}
