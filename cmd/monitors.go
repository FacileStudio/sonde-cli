package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/sonde-cli/internal/api"
	"github.com/FacileStudio/sonde-cli/internal/ui"
)

var (
	addSlug     string
	addName     string
	addType     string
	addTarget   string
	addInterval int
	addTimeout  int
	addStatus   int
	addKeyword  string
)

var monitorsCmd = &cobra.Command{
	Use:   "monitors",
	Short: "List, add and remove monitors",
}

var monitorsListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List monitors",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signalContext()
		defer stop()

		client, err := newClient(true)
		if err != nil {
			return err
		}
		monitors, err := client.Monitors(ctx)
		if err != nil {
			return wrapInterrupt(ctx, err)
		}
		if flagJSON {
			return ui.JSON(monitors)
		}
		if len(monitors) == 0 {
			ui.Warn("no monitors yet — add one with `sonde monitors add`")
			return nil
		}

		table := ui.Table("ID", "SLUG", "NAME", "TYPE", "TARGET", "EVERY", "ENABLED")
		for _, monitor := range monitors {
			table.Row(
				strconv.FormatInt(monitor.ID, 10),
				monitor.Slug,
				monitor.Name,
				monitor.Type,
				pushTarget(monitor),
				fmt.Sprintf("%ds", monitor.IntervalSeconds),
				yesNo(monitor.Enabled),
			)
		}
		return table.Flush()
	},
}

var monitorsAddCmd = &cobra.Command{
	Use:   "add",
	Args:  cobra.NoArgs,
	Short: "Add a monitor",
	Long: `Adds a monitor. The slug is required and is what a status page and the export
key on: lowercase letters, digits and dashes.

An http monitor takes a URL, a tcp monitor takes host:port, and a push monitor
takes no target at all — its push token is the endpoint, and it is printed once
here.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signalContext()
		defer stop()

		client, err := newClient(true)
		if err != nil {
			return err
		}
		created, err := client.CreateMonitor(ctx, api.MonitorInput{
			Slug:            addSlug,
			Name:            addName,
			Type:            addType,
			Target:          addTarget,
			IntervalSeconds: addInterval,
			TimeoutSeconds:  addTimeout,
			ExpectedStatus:  addStatus,
			ExpectedKeyword: addKeyword,
		})
		if err != nil {
			return wrapInterrupt(ctx, err)
		}
		if flagJSON {
			return ui.JSON(created)
		}

		ui.Success("monitor %d created (%s)", created.ID, created.Slug)
		if created.PushToken != nil {
			ui.Step("push token: %s", *created.PushToken)
			ui.Hint("cron line: * * * * * sonde push %s --url %s", *created.PushToken, client.BaseURL)
		}
		return nil
	},
}

var monitorsRemoveCmd = &cobra.Command{
	Use:   "remove <id|slug>",
	Args:  cobra.ExactArgs(1),
	Short: "Remove a monitor",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signalContext()
		defer stop()

		client, err := newClient(true)
		if err != nil {
			return err
		}
		id, err := resolveMonitor(ctx, client, args[0])
		if err != nil {
			return wrapInterrupt(ctx, err)
		}
		if err := client.DeleteMonitor(ctx, id); err != nil {
			return wrapInterrupt(ctx, err)
		}
		if flagJSON {
			return ui.JSON(struct {
				ID      int64 `json:"id"`
				Removed bool  `json:"removed"`
			}{id, true})
		}
		ui.Success("monitor %d removed", id)
		return nil
	},
}

// resolveMonitor turns what a human typed into the id the API takes. The delete
// route accepts a positive integer and nothing else, so a slug has to be looked
// up rather than passed through and 400'd.
func resolveMonitor(ctx context.Context, client *api.Client, reference string) (int64, error) {
	reference = strings.TrimSpace(reference)
	if id, err := strconv.ParseInt(reference, 10, 64); err == nil && id > 0 {
		return id, nil
	}

	monitors, err := client.Monitors(ctx)
	if err != nil {
		return 0, err
	}
	for _, monitor := range monitors {
		if monitor.Slug == reference {
			return monitor.ID, nil
		}
	}
	return 0, fmt.Errorf("no monitor with id or slug %q — check `sonde monitors list`", reference)
}

// pushTarget shows what a push monitor is reached at, since it stores no target
// and an empty column reads as a broken monitor.
func pushTarget(monitor api.Monitor) string {
	if monitor.Target != "" {
		return monitor.Target
	}
	if monitor.PushToken != nil {
		return "push token " + *monitor.PushToken
	}
	return "-"
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func init() {
	monitorsAddCmd.Flags().StringVar(&addSlug, "slug", "", "Slug: lowercase letters, digits and dashes")
	monitorsAddCmd.Flags().StringVar(&addName, "name", "", "Display name")
	monitorsAddCmd.Flags().StringVar(&addType, "type", "http", "Monitor type: http, tcp or push")
	monitorsAddCmd.Flags().StringVar(&addTarget, "target", "", "URL for http, host:port for tcp, empty for push")
	monitorsAddCmd.Flags().IntVar(&addInterval, "interval", 0, "Seconds between checks (default 60, minimum 20)")
	monitorsAddCmd.Flags().IntVar(&addTimeout, "timeout", 0, "Probe timeout in seconds (default 10)")
	monitorsAddCmd.Flags().IntVar(&addStatus, "expect-status", 0, "HTTP status that counts as up (default 200)")
	monitorsAddCmd.Flags().StringVar(&addKeyword, "expect-keyword", "", "Text the response body must contain")
	_ = monitorsAddCmd.MarkFlagRequired("slug")
	_ = monitorsAddCmd.MarkFlagRequired("name")

	monitorsCmd.AddCommand(monitorsListCmd, monitorsAddCmd, monitorsRemoveCmd)
	rootCmd.AddCommand(monitorsCmd)
}
