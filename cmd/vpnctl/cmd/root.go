package cmd

import (
	"github.com/spf13/cobra"
)

// Flag names and defaults as named constants.
const (
	FlagAPIURL   = "api-url"
	FlagAPIToken = "api-token"

	DefaultAPIURL = "http://localhost:4000"

	EnvAPIToken = "VPNCTL_API_TOKEN"
)

var (
	apiURL   string
	apiToken string
)

var rootCmd = &cobra.Command{
	Use:   "vpnctl",
	Short: "RemnaCore CLI — manage plugins and platform operations",
	Long: `vpnctl is the command-line interface for RemnaCore.

It provides commands for plugin management including scaffolding,
building, installing, enabling, disabling, listing, and uninstalling plugins.`,
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&apiURL, FlagAPIURL, DefaultAPIURL, "platform admin API base URL")
	rootCmd.PersistentFlags().StringVar(&apiToken, FlagAPIToken, "", "API authentication token (or set "+EnvAPIToken+")")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
