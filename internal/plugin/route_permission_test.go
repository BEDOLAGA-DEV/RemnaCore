package plugin

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateRoutePermissions(t *testing.T) {
	known := func(p string) bool { return p == "tariffs.read" }

	// Empty required_permission is valid (defaults to plugins.manage at registration).
	mEmpty := &Manifest{Routes: []ManifestRoute{{RequiredPermission: ""}}}
	require.NoError(t, validateRoutePermissions(mEmpty, known))

	// Known permission is accepted.
	mKnown := &Manifest{Routes: []ManifestRoute{{RequiredPermission: "tariffs.read"}}}
	require.NoError(t, validateRoutePermissions(mKnown, known))

	// Unknown permission is rejected with ErrInvalidManifest.
	mBad := &Manifest{Routes: []ManifestRoute{{RequiredPermission: "made.up"}}}
	err := validateRoutePermissions(mBad, known)
	require.True(t, errors.Is(err, ErrInvalidManifest))

	// Nil validator skips the check (no validator wired).
	require.NoError(t, validateRoutePermissions(mBad, nil))
}
