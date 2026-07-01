package config

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/secret"

// SecretString wraps a string that should not be accidentally logged or
// serialized. It is a type alias for secret.String so that domain packages
// (which must not import internal/config) can hold the same secret type via
// pkg/secret, while config and adapter code keep using config.SecretString.
// String(), MarshalJSON(), and MarshalText() all return a masked value; use
// Expose() to retrieve the actual secret.
type SecretString = secret.String

// NewSecretString creates a SecretString wrapping the given value.
func NewSecretString(s string) SecretString {
	return secret.NewString(s)
}
