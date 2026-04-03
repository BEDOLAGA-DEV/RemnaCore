package plugin

import (
	"fmt"
	"strconv"
	"strings"
)

// checkSDKCompatibility verifies the plugin's declared sdk_version is
// compatible with the current platform SDK version. Uses simple semver major
// version check: major versions must match. This allows minor/patch updates
// within the same major version while rejecting breaking changes.
func checkSDKCompatibility(pluginSDKVersion string) error {
	if pluginSDKVersion == "" {
		return fmt.Errorf("%w: plugin sdk_version is empty", ErrInvalidManifest)
	}

	pluginMajor, err := parseMajorVersion(pluginSDKVersion)
	if err != nil {
		return fmt.Errorf("%w: invalid plugin sdk_version %q: %v", ErrInvalidManifest, pluginSDKVersion, err)
	}

	currentMajor, _ := parseMajorVersion(CurrentSDKVersion)

	if pluginMajor != currentMajor {
		return fmt.Errorf("%w: plugin requires SDK v%d.x but platform provides v%s",
			ErrIncompatibleSDK, pluginMajor, CurrentSDKVersion)
	}

	return nil
}

// parseMajorVersion extracts the major version number from a semver string.
// It strips leading constraint characters (^, ~, >=, etc.) before parsing.
func parseMajorVersion(version string) (int, error) {
	// Strip leading constraint characters.
	version = strings.TrimLeft(version, "^~>=<! ")

	parts := strings.SplitN(version, ".", 3)
	if len(parts) == 0 || parts[0] == "" {
		return 0, fmt.Errorf("empty version")
	}

	return strconv.Atoi(parts[0])
}
