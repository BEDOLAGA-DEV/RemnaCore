package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var pluginUninstallCmd = &cobra.Command{
	Use:   "uninstall <plugin-id>",
	Short: "Uninstall a plugin",
	Long: `Remove a plugin from the platform entirely.

Example:
  vpnctl plugin uninstall my-plugin`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginUninstall,
}

func init() {
	pluginCmd.AddCommand(pluginUninstallCmd)
}

func runPluginUninstall(cmd *cobra.Command, args []string) error {
	pluginID := args[0]
	path := fmt.Sprintf("%s/%s", PluginsAPIPath, pluginID)

	client := newAPIClient()
	body, status, err := client.delete(path)
	if err != nil {
		return err
	}

	if status >= 300 {
		return fmt.Errorf("uninstall failed (HTTP %d): %s", status, string(body))
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Plugin %q uninstalled.\n", pluginID)
	return nil
}
