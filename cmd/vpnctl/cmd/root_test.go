package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootCmd_HasPluginSubcommand(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "plugin" {
			found = true
			break
		}
	}
	assert.True(t, found, "root command should have a 'plugin' subcommand")
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "api-url", FlagAPIURL)
	assert.Equal(t, "api-token", FlagAPIToken)
	assert.Equal(t, "http://localhost:4000", DefaultAPIURL)
	assert.Equal(t, "VPNCTL_API_TOKEN", EnvAPIToken)
}
