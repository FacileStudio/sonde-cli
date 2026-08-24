package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/FacileStudio/sonde-cli/internal/api"
	"github.com/FacileStudio/sonde-cli/internal/config"
	"github.com/FacileStudio/sonde-cli/internal/ui"
	"github.com/spf13/cobra"
)

func instanceFlag(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("instance")
	return strings.TrimRight(v, "/")
}

func newClient(cmd *cobra.Command, requireToken bool) (*api.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	serverURL := cfg.ServerURL()
	if flag := instanceFlag(cmd); flag != "" {
		serverURL = flag
	}
	if serverURL == "" {
		return nil, fmt.Errorf("no Sonde instance known — run 'sonde login <url>' or set %s", config.URLEnv)
	}
	token := cfg.AuthToken()
	if requireToken && token == "" && !strings.Contains(serverURL, "/api/push/") {
		return nil, fmt.Errorf("not logged in — run `sonde login` or set %s", config.TokenEnv)
	}
	return api.New(serverURL, token), nil
}

var monitorsCmd = &cobra.Command{
	Use:   "monitors",
	Short: "Manage monitors",
}

var monitorsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List monitors",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient(cmd, true)
		if err != nil {
			return err
		}
		monitors, err := client.ListMonitors()
		if err != nil {
			return err
		}
		if len(monitors) == 0 {
			fmt.Println("No monitors")
			return nil
		}
		for _, m := range monitors {
			state := m.Status
			if state == "" {
				if m.Enabled {
					state = "enabled"
				} else {
					state = "disabled"
				}
			}
			fmt.Printf("%d\t%s\t%s\t%s\t%s\n", m.ID, m.Name, m.Type, m.Target, state)
		}
		return nil
	},
}

var (
	addName     string
	addType     string
	addTarget   string
	addInterval int
)

var monitorsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a monitor",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient(cmd, true)
		if err != nil {
			return err
		}
		created, err := client.AddMonitor(api.Monitor{
			Name: addName, Type: addType, Target: addTarget, Interval: addInterval, Enabled: true,
		})
		if err != nil {
			return err
		}
		ui.Success("Monitor %d created (%s)", created.ID, created.Name)
		return nil
	},
}

var monitorsRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Args:  cobra.ExactArgs(1),
	Short: "Remove a monitor",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient(cmd, true)
		if err != nil {
			return err
		}
		if err := client.RemoveMonitor(args[0]); err != nil {
			return err
		}
		ui.Success("Monitor %s removed", args[0])
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current state of every monitor",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient(cmd, true)
		if err != nil {
			return err
		}
		monitors, err := client.Status()
		if err != nil {
			return err
		}
		if len(monitors) == 0 {
			fmt.Println("No monitors")
			return nil
		}
		for _, m := range monitors {
			fmt.Printf("%s\t%s\t%s\n", m.Status, m.Name, m.Target)
		}
		return nil
	},
}

var incidentsCmd = &cobra.Command{
	Use:   "incidents",
	Short: "List incidents",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient(cmd, true)
		if err != nil {
			return err
		}
		incidents, err := client.Incidents()
		if err != nil {
			return err
		}
		if len(incidents) == 0 {
			fmt.Println("No incidents")
			return nil
		}
		for _, inc := range incidents {
			resolved := "open"
			if inc.ResolvedAt != nil {
				resolved = "resolved " + inc.ResolvedAt.Format(time.RFC3339)
			}
			fmt.Printf("#%d\tmonitor %d\t%s\topened %s\t%s\n",
				inc.ID, inc.MonitorID, inc.Cause, inc.OpenedAt.Format(time.RFC3339), resolved)
		}
		return nil
	},
}

func init() {
	monitorsAddCmd.Flags().StringVar(&addName, "name", "", "Monitor name")
	monitorsAddCmd.Flags().StringVar(&addType, "type", "http", "Monitor type (http, tcp, push)")
	monitorsAddCmd.Flags().StringVar(&addTarget, "target", "", "URL or host to watch")
	monitorsAddCmd.Flags().IntVar(&addInterval, "interval", 60, "Check interval in seconds")
	monitorsAddCmd.MarkFlagRequired("name")
	monitorsAddCmd.MarkFlagRequired("target")

	monitorsCmd.AddCommand(monitorsListCmd, monitorsAddCmd, monitorsRemoveCmd)
	rootCmd.AddCommand(monitorsCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(incidentsCmd)
}
