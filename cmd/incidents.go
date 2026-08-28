package cmd

import (
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/sonde-cli/internal/api"
	"github.com/FacileStudio/sonde-cli/internal/ui"
)

var incidentsMonitor string

var incidentsCmd = &cobra.Command{
	Use:   "incidents",
	Args:  cobra.NoArgs,
	Short: "List incidents, newest first",
	Long: `Lists downtime episodes across every monitor, or one monitor with --monitor.

The instance caps the list at 200. An incident with no resolved time is still
running.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signalContext()
		defer stop()

		client, err := newClient(cmd, true)
		if err != nil {
			return err
		}

		var monitorID int64
		if incidentsMonitor != "" {
			monitorID, err = resolveMonitor(ctx, client, incidentsMonitor)
			if err != nil {
				return wrapInterrupt(ctx, err)
			}
		}

		incidents, err := client.Incidents(ctx, monitorID)
		if err != nil {
			return wrapInterrupt(ctx, err)
		}
		if len(incidents) == 0 {
			ui.Success("no incidents")
			return nil
		}

		table := ui.Table("ID", "MONITOR", "OPENED", "RESOLVED", "CAUSE")
		for _, incident := range incidents {
			table.Row(
				strconv.FormatInt(incident.ID, 10),
				strconv.FormatInt(incident.MonitorID, 10),
				incident.OpenedAt.Format(time.RFC3339),
				resolvedAt(incident),
				incident.Cause,
			)
		}
		return table.Flush()
	},
}

// resolvedAt renders when an episode ended, or says it has not.
func resolvedAt(incident api.Incident) string {
	if incident.Open() {
		return "still open"
	}
	return incident.ResolvedAt.Format(time.RFC3339)
}

func init() {
	incidentsCmd.Flags().StringVar(&incidentsMonitor, "monitor", "", "Limit to one monitor, by id or slug")
	rootCmd.AddCommand(incidentsCmd)
}
