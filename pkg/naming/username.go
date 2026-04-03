// Package naming provides shared naming conventions used across the platform.
package naming

import "fmt"

const (
	// PlatformTag identifies platform-managed users in the VPN panel.
	PlatformTag = "PLATFORM"

	// UsernamePrefix is prepended to all generated Remnawave usernames.
	UsernamePrefix = "p_"

	// UsernameShortIDLen is the number of characters taken from the platform
	// user ID when building a Remnawave username.
	UsernameShortIDLen = 8
)

// BuildRemnawaveUsername constructs a deterministic Remnawave username from a
// platform user ID, a purpose label (e.g. "base"), and a numeric index.
// Format: p_{first8chars}_{purpose}_{index}
func BuildRemnawaveUsername(platformUserID, purpose string, index int) string {
	shortID := platformUserID
	if len(shortID) > UsernameShortIDLen {
		shortID = shortID[:UsernameShortIDLen]
	}
	return fmt.Sprintf("%s%s_%s_%d", UsernamePrefix, shortID, purpose, index)
}
