package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var pluginBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Compile the current plugin to WebAssembly",
	Long: `Build the plugin in the current directory into a .wasm binary.

Runs: GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm .

Example:
  cd my-plugin && vpnctl plugin build`,
	RunE: runPluginBuild,
}

func init() {
	pluginCmd.AddCommand(pluginBuildCmd)
}

func runPluginBuild(cmd *cobra.Command, _ []string) error {
	if _, err := os.Stat("plugin.toml"); os.IsNotExist(err) {
		return fmt.Errorf("plugin.toml not found; run this command from a plugin directory")
	}

	build := exec.Command("go", "build", "-o", "plugin.wasm", ".")
	build.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	build.Stdout = cmd.OutOrStdout()
	build.Stderr = cmd.ErrOrStderr()

	fmt.Fprintln(cmd.OutOrStdout(), "Building plugin to plugin.wasm ...")
	if err := build.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Build complete: plugin.wasm")
	return nil
}
