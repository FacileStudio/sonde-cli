package cmd

import (
	"fmt"
	"net/url"

	"github.com/FacileStudio/sonde-cli/internal/ui"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push <token>",
	Args:  cobra.ExactArgs(1),
	Short: "Send a heartbeat for a push monitor",
	Long: `Send a heartbeat for a push monitor.

Designed for cron. The token is the monitor's push credential, so no login is
required:

  * * * * * sonde push <token> --instance https://sonde.example.com`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if url.PathEscape(args[0]) != args[0] {
			return fmt.Errorf("invalid push token")
		}
		client, err := newClient(cmd, false)
		if err != nil {
			return err
		}
		if err := client.Push(args[0]); err != nil {
			return err
		}
		ui.Success("Heartbeat sent")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pushCmd)
}
