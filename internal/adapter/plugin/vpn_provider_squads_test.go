package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveInternalSquads(t *testing.T) {
	// Request override wins.
	require.Equal(t, []string{"req"}, resolveInternalSquads([]string{"req"}, []string{"def"}))
	// Empty request -> default.
	require.Equal(t, []string{"def"}, resolveInternalSquads(nil, []string{"def"}))
	// Both empty -> empty.
	require.Empty(t, resolveInternalSquads(nil, nil))
}
