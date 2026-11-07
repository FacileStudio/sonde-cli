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
	pageSlug     string
	pageTitle    string
	pageMonitors string
)

var statusPagesCmd = &cobra.Command{
	Use:     "status-pages",
	Aliases: []string{"pages"},
	Short:   "List, create, update and delete public status pages",
	Long:    "Manage public status pages. Requires an admin session or a token in SONDE_TOKEN.",
}

var statusPagesListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List status pages",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signalContext()
		defer stop()

		client, err := newClient(true)
		if err != nil {
			return err
		}
		pages, err := client.StatusPages(ctx)
		if err != nil {
			return wrapInterrupt(ctx, err)
		}
		if flagJSON {
			return ui.JSON(pages)
		}
		if len(pages) == 0 {
			ui.Warn("no status pages yet — add one with `sonde status-pages create`")
			return nil
		}
		table := ui.Table("ID", "SLUG", "TITLE", "MONITORS")
		for _, page := range pages {
			table.Row(strconv.FormatInt(page.ID, 10), page.Slug, page.Title, strconv.Itoa(len(page.Monitors)))
		}
		return table.Flush()
	},
}

var statusPagesCreateCmd = &cobra.Command{
	Use:   "create",
	Args:  cobra.NoArgs,
	Short: "Create a status page",
	Long: `Creates a status page served at /status/<slug>. --monitors takes monitor slugs
or ids, comma separated, and the page lists them in that order.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signalContext()
		defer stop()

		client, err := newClient(true)
		if err != nil {
			return err
		}
		ids, err := resolveMonitorList(ctx, client, pageMonitors)
		if err != nil {
			return wrapInterrupt(ctx, err)
		}
		created, err := client.CreateStatusPage(ctx, api.StatusPageInput{Slug: pageSlug, Title: pageTitle, MonitorIDs: ids})
		if err != nil {
			return wrapInterrupt(ctx, err)
		}
		if flagJSON {
			return ui.JSON(created)
		}
		ui.Success("status page %d created (%s)", created.ID, created.Slug)
		ui.Hint("public at %s/status/%s", client.BaseURL, created.Slug)
		return nil
	},
}

var statusPagesUpdateCmd = &cobra.Command{
	Use:   "update <id|slug>",
	Args:  cobra.ExactArgs(1),
	Short: "Update a status page",
	Long:  "Changes the flags you pass and keeps the rest. --monitors replaces the whole list.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signalContext()
		defer stop()

		client, err := newClient(true)
		if err != nil {
			return err
		}
		page, err := findStatusPage(ctx, client, args[0])
		if err != nil {
			return wrapInterrupt(ctx, err)
		}
		input := api.StatusPageInput{Slug: page.Slug, Title: page.Title}
		for _, line := range page.Monitors {
			input.MonitorIDs = append(input.MonitorIDs, line.ID)
		}
		if cmd.Flags().Changed("slug") {
			input.Slug = pageSlug
		}
		if cmd.Flags().Changed("title") {
			input.Title = pageTitle
		}
		if cmd.Flags().Changed("monitors") {
			if input.MonitorIDs, err = resolveMonitorList(ctx, client, pageMonitors); err != nil {
				return wrapInterrupt(ctx, err)
			}
		}
		updated, err := client.UpdateStatusPage(ctx, page.ID, input)
		if err != nil {
			return wrapInterrupt(ctx, err)
		}
		if flagJSON {
			return ui.JSON(updated)
		}
		ui.Success("status page %d updated (%s)", updated.ID, updated.Slug)
		return nil
	},
}

var statusPagesDeleteCmd = &cobra.Command{
	Use:   "delete <id|slug>",
	Args:  cobra.ExactArgs(1),
	Short: "Delete a status page",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signalContext()
		defer stop()

		client, err := newClient(true)
		if err != nil {
			return err
		}
		page, err := findStatusPage(ctx, client, args[0])
		if err != nil {
			return wrapInterrupt(ctx, err)
		}
		if err := client.DeleteStatusPage(ctx, page.ID); err != nil {
			return wrapInterrupt(ctx, err)
		}
		if flagJSON {
			return ui.JSON(struct {
				ID      int64 `json:"id"`
				Removed bool  `json:"removed"`
			}{page.ID, true})
		}
		ui.Success("status page %d deleted", page.ID)
		return nil
	},
}

func findStatusPage(ctx context.Context, client *api.Client, reference string) (api.StatusPage, error) {
	reference = strings.TrimSpace(reference)
	pages, err := client.StatusPages(ctx)
	if err != nil {
		return api.StatusPage{}, err
	}
	for _, page := range pages {
		if page.Slug == reference || strconv.FormatInt(page.ID, 10) == reference {
			return page, nil
		}
	}
	return api.StatusPage{}, fmt.Errorf("no status page with id or slug %q — check `sonde status-pages list`", reference)
}

func resolveMonitorList(ctx context.Context, client *api.Client, list string) ([]int64, error) {
	ids := []int64{}
	if strings.TrimSpace(list) == "" {
		return ids, nil
	}
	monitors, err := client.Monitors(ctx)
	if err != nil {
		return nil, err
	}
	return matchMonitors(monitors, strings.Split(list, ","))
}

func matchMonitors(monitors []api.Monitor, references []string) ([]int64, error) {
	ids := make([]int64, 0, len(references))
	for _, reference := range references {
		reference = strings.TrimSpace(reference)
		if reference == "" {
			continue
		}
		id, ok := lookupMonitor(monitors, reference)
		if !ok {
			return nil, fmt.Errorf("no monitor with id or slug %q — check `sonde monitors list`", reference)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func lookupMonitor(monitors []api.Monitor, reference string) (int64, bool) {
	for _, monitor := range monitors {
		if monitor.Slug == reference {
			return monitor.ID, true
		}
	}
	if id, err := strconv.ParseInt(reference, 10, 64); err == nil {
		for _, monitor := range monitors {
			if monitor.ID == id {
				return id, true
			}
		}
	}
	return 0, false
}

func init() {
	for _, command := range []*cobra.Command{statusPagesCreateCmd, statusPagesUpdateCmd} {
		command.Flags().StringVar(&pageSlug, "slug", "", "Slug: lowercase letters, digits and dashes")
		command.Flags().StringVar(&pageTitle, "title", "", "Page title")
		command.Flags().StringVar(&pageMonitors, "monitors", "", "Monitor slugs or ids, comma separated, in display order")
	}
	_ = statusPagesCreateCmd.MarkFlagRequired("slug")
	_ = statusPagesCreateCmd.MarkFlagRequired("title")

	statusPagesCmd.AddCommand(statusPagesListCmd, statusPagesCreateCmd, statusPagesUpdateCmd, statusPagesDeleteCmd)
	rootCmd.AddCommand(statusPagesCmd)
}
