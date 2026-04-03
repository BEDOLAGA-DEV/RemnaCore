package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var pluginEnableCmd = &cobra.Command{
	Use:   "enable <plugin-id>",
	Short: "Enable an installed plugin",
	Long: `Enable a plugin by its ID so it starts processing hooks.

Example:
  vpnctl plugin enable my-plugin`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginEnable,
}

func init() {
	pluginCmd.AddCommand(pluginEnableCmd)
}

func runPluginEnable(cmd *cobra.Command, args []string) error {
	pluginID := args[0]
	path := fmt.Sprintf("%s/%s/enable", PluginsAPIPath, pluginID)

	client := newAPIClient()
	body, status, err := client.post(path, nil)
	if err != nil {
		return err
	}

	if status >= 300 {
		return fmt.Errorf("enable failed (HTTP %d): %s", status, string(body))
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Plugin %q enabled.\n", pluginID)
	return nil
}
