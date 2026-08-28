package cmd

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/sonde-cli/internal/api"
	"github.com/FacileStudio/sonde-cli/internal/ui"
)

var pushCmd = &cobra.Command{
	Use:   "push <token>",
	Args:  cobra.ExactArgs(1),
	Short: "Send a heartbeat for a push monitor",
	Long: `Sends a heartbeat for a push monitor. The token is the monitor's own
credential — the one POST /api/push/<token> takes — so no login is involved and
none is read.

This is the cron line:

  * * * * * sonde push <token> --instance https://sonde.example.com

The instance may come from SONDE_SERVER_URL instead, which is what a systemd
timer or a container tends to have. A heartbeat for an unknown token is a 404,
so a typo fails loudly rather than reporting a job alive.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signalContext()
		defer stop()

		token := strings.TrimSpace(args[0])
		if token == "" {
			return errors.New("the push token is empty")
		}

		client, err := newClient(cmd, false)
		if err != nil {
			return err
		}
		if err := client.Push(ctx, token); err != nil {
			var apiErr *api.Error
			if errors.As(err, &apiErr) && apiErr.NotFound() {
				return errors.New("the instance does not know this push token — check it against `sonde monitors list`")
			}
			return wrapInterrupt(ctx, err)
		}
		ui.Success("heartbeat sent")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pushCmd)
}
