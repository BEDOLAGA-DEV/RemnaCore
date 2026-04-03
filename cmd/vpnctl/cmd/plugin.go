package cmd

import "github.com/spf13/cobra"

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage RemnaCore plugins",
	Long:  "Commands for scaffolding, building, installing, and managing plugins.",
}

func init() {
	rootCmd.AddCommand(pluginCmd)
}
