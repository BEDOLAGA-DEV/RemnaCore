package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// API path for plugin management.
const (
	PluginsAPIPath = "/api/admin/plugins"
)

// installPayload is the JSON body sent to POST /api/admin/plugins.
type installPayload struct {
	Manifest string `json:"manifest"` // base64-encoded plugin.toml
	Wasm     string `json:"wasm"`     // base64-encoded .wasm binary
}

var pluginInstallCmd = &cobra.Command{
	Use:   "install <wasm-file>",
	Short: "Install a plugin from a .wasm file",
	Long: `Upload a compiled plugin to RemnaCore.

The command reads plugin.wasm and the accompanying plugin.toml from the same
directory, then uploads both to the platform admin API.

Example:
  vpnctl plugin install ./plugin.wasm`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginInstall,
}

func init() {
	pluginCmd.AddCommand(pluginInstallCmd)
}

func runPluginInstall(cmd *cobra.Command, args []string) error {
	wasmPath := args[0]

	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return fmt.Errorf("reading wasm file: %w", err)
	}

	manifestPath := filepath.Join(filepath.Dir(wasmPath), "plugin.toml")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("reading plugin.toml (expected next to %s): %w", wasmPath, err)
	}

	payload := installPayload{
		Manifest: base64.StdEncoding.EncodeToString(manifestBytes),
		Wasm:     base64.StdEncoding.EncodeToString(wasmBytes),
	}

	client := newAPIClient()
	body, status, err := client.post(PluginsAPIPath, payload)
	if err != nil {
		return err
	}

	if status >= 300 {
		return fmt.Errorf("install failed (HTTP %d): %s", status, string(body))
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Plugin installed successfully.\n%s\n", string(body))
	return nil
}
