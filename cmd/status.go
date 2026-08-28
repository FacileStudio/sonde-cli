package cmd

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/sonde-cli/internal/api"
	"github.com/FacileStudio/sonde-cli/internal/ui"
)

var statusWindow string

var statusCmd = &cobra.Command{
	Use:   "status [status-page-slug]",
	Args:  cobra.MaximumNArgs(1),
	Short: "Show how every monitor is doing",
	Long: `Reads the current state of the monitors on an instance.

With a status page slug it reads that page's public feed, which needs no
credential and carries the instance's own up/down verdict from the newest check.

Without one it reads your own monitors, their uptime over --window, and the
incidents still open against them. A monitor with an open incident is down; a
monitor with no checks yet is unknown, never a fabricated green.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signalContext()
		defer stop()

		if len(args) == 1 {
			return publicStatus(ctx, args[0])
		}
		return ownStatus(ctx)
	},
}

// statusRow is one monitor's line, and the shape --json emits. It is the table
// as data: a script reading it gets the verdict and the ratio the human sees,
// rather than having to reassemble them from the monitors and incidents calls.
type statusRow struct {
	ID     int64      `json:"id"`
	Slug   string     `json:"slug"`
	Name   string     `json:"name"`
	Status string     `json:"status"`
	Window string     `json:"window"`
	Uptime *float64   `json:"uptime"`
	Since  *time.Time `json:"since"`
}

// publicStatus renders a published status page. It deliberately does not
// require a token: this is the readout somebody can check from a machine that
// has never logged in.
func publicStatus(ctx context.Context, slug string) error {
	client, err := newClient(false)
	if err != nil {
		return err
	}
	page, err := client.PublicStatus(ctx, slug)
	if err != nil {
		return wrapInterrupt(ctx, err)
	}
	if flagJSON {
		return ui.JSON(page)
	}
	if len(page.Monitors) == 0 {
		ui.Warn("status page %q lists no monitors", page.Slug)
		return nil
	}

	ui.Step("%s", page.Title)
	table := ui.Table("STATUS", "SLUG", "NAME", "24H", "7D", "OPEN")
	for _, monitor := range page.Monitors {
		table.Row(
			monitor.CurrentStatus,
			monitor.Slug,
			monitor.Name,
			ratio(monitor.Uptime24h),
			ratio(monitor.Uptime7d),
			strconv.Itoa(len(monitor.OpenIncidents)),
		)
	}
	return table.Flush()
}

// ownStatus builds the readout the API has no single endpoint for: the monitors
// list carries no current state, so downness comes from the incidents that are
// still open and the percentage from each monitor's uptime window.
func ownStatus(ctx context.Context) error {
	client, err := newClient(true)
	if err != nil {
		return err
	}
	monitors, err := client.Monitors(ctx)
	if err != nil {
		return wrapInterrupt(ctx, err)
	}
	if len(monitors) == 0 && !flagJSON {
		ui.Warn("no monitors yet — add one with `sonde monitors add`")
		return nil
	}
	incidents, err := client.Incidents(ctx, 0)
	if err != nil {
		return wrapInterrupt(ctx, err)
	}

	rows := statusRows(ctx, client, monitors, openByMonitor(incidents))
	if flagJSON {
		return ui.JSON(rows)
	}
	return renderStatus(rows)
}

// statusRows assembles the readout the API has no single endpoint for: the
// monitors list carries no current state, so downness comes from the incidents
// still open and the percentage from each monitor's uptime window.
func statusRows(ctx context.Context, client *api.Client, monitors []api.Monitor,
	open map[int64]*api.Incident) []statusRow {
	rows := make([]statusRow, 0, len(monitors))
	for _, monitor := range monitors {
		state, since := verdict(monitor, open[monitor.ID])
		rows = append(rows, statusRow{
			ID:     monitor.ID,
			Slug:   monitor.Slug,
			Name:   monitor.Name,
			Status: state,
			Window: statusWindow,
			Uptime: uptimeFor(ctx, client, monitor.ID),
			Since:  since,
		})
	}
	return rows
}

// renderStatus prints the rows as a table.
func renderStatus(rows []statusRow) error {
	table := ui.Table("STATUS", "SLUG", "NAME", "UPTIME "+statusWindow, "SINCE")
	for _, row := range rows {
		table.Row(row.Status, row.Slug, row.Name, ratio(row.Uptime), since(row.Since))
	}
	return table.Flush()
}

// verdict names a monitor's state from what the authenticated API can prove. A
// disabled monitor is not down, it is off, and saying "up" for either would be
// a claim nothing here supports.
func verdict(monitor api.Monitor, incident *api.Incident) (string, *time.Time) {
	switch {
	case incident != nil:
		opened := incident.OpenedAt
		return "down", &opened
	case !monitor.Enabled:
		return "paused", nil
	default:
		return "ok", nil
	}
}

// since renders when a monitor went down, or a dash when it has not.
func since(at *time.Time) string {
	if at == nil {
		return "-"
	}
	return at.Format(time.RFC3339)
}

// openByMonitor indexes the still-running incidents by monitor, keeping the
// newest since the list arrives newest first.
func openByMonitor(incidents []api.Incident) map[int64]*api.Incident {
	open := map[int64]*api.Incident{}
	for index := range incidents {
		incident := incidents[index]
		if incident.Open() && open[incident.MonitorID] == nil {
			open[incident.MonitorID] = &incident
		}
	}
	return open
}

// uptimeFor reads one monitor's window. A failure reads as unknown rather than
// ending the listing: one monitor the server could not summarise is no reason
// to withhold the state of the others.
func uptimeFor(ctx context.Context, client *api.Client, id int64) *float64 {
	summary, err := client.MonitorUptime(ctx, id, statusWindow)
	if err != nil {
		return nil
	}
	return summary.Uptime
}

// ratio renders an uptime percentage, which is what the server already sends:
// 99 means 99%, not 9900%. A nil value means the monitor has never been
// checked, which is unknown and not zero.
func ratio(value *float64) string {
	if value == nil {
		return "unknown"
	}
	return fmt.Sprintf("%.2f%%", *value)
}

func init() {
	statusCmd.Flags().StringVar(&statusWindow, "window", "24h", "Uptime window: 24h, 7d, 30d or 90d")
	rootCmd.AddCommand(statusCmd)
}
