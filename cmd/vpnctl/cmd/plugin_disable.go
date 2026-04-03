package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var pluginDisableCmd = &cobra.Command{
	Use:   "disable <plugin-id>",
	Short: "Disable a running plugin",
	Long: `Disable a plugin by its ID. The plugin remains installed but stops
processing hooks.

Example:
  vpnctl plugin disable my-plugin`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginDisable,
}

func init() {
	pluginCmd.AddCommand(pluginDisableCmd)
}

func runPluginDisable(cmd *cobra.Command, args []string) error {
	pluginID := args[0]
	path := fmt.Sprintf("%s/%s/disable", PluginsAPIPath, pluginID)

	client := newAPIClient()
	body, status, err := client.post(path, nil)
	if err != nil {
		return err
	}

	if status >= 300 {
		return fmt.Errorf("disable failed (HTTP %d): %s", status, string(body))
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Plugin %q disabled.\n", pluginID)
	return nil
}
