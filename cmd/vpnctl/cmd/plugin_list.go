package cmd

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// pluginListEntry mirrors the JSON returned by GET /api/admin/plugins.
type pluginListEntry struct {
	ID      string `json:"id"`
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Status  string `json:"status"`
	Lang    string `json:"lang"`
}

// pluginListResponse wraps the API response.
type pluginListResponse struct {
	Data []pluginListEntry `json:"data"`
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	Long: `Display a table of all installed plugins with their status.

Example:
  vpnctl plugin list`,
	RunE: runPluginList,
}

func init() {
	pluginCmd.AddCommand(pluginListCmd)
}

func runPluginList(cmd *cobra.Command, _ []string) error {
	client := newAPIClient()
	body, status, err := client.get(PluginsAPIPath)
	if err != nil {
		return err
	}

	if status >= 300 {
		return fmt.Errorf("list failed (HTTP %d): %s", status, string(body))
	}

	var resp pluginListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSLUG\tNAME\tVERSION\tSTATUS\tLANG")
	for _, p := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", p.ID, p.Slug, p.Name, p.Version, p.Status, p.Lang)
	}
	return w.Flush()
}
