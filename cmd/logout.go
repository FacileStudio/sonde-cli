package cmd

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/sonde-cli/internal/api"
	"github.com/FacileStudio/sonde-cli/internal/config"
	"github.com/FacileStudio/sonde-cli/internal/ui"
)

// logoutCmd revokes the session server-side and then forgets it locally.
//
// The order matters and so does the tolerance: the local credential is cleared
// even when the server refuses the revocation, because somebody logging out of
// a borrowed machine cares far more about the token leaving the disk than about
// a clean round trip.
//
// Only the stored session is touched. A token supplied through SONDE_TOKEN is
// left alone: it is not this command's to revoke, and it may be shared.
var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Revoke the stored session",
	Long: `Revokes the session token on the instance and forgets it locally, keeping the
instance URL so the next login does not need it again.

Running it with no session stored is not an error.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signalContext()
		defer stop()

		cfg, err := config.Load()
		unreadable := err != nil
		if unreadable {
			ui.Warn("the stored configuration could not be read — clearing it anyway")
			cfg = config.Config{}
		}

		serverURL := config.ResolveURL(cfg.URL, flagURL)
		if cfg.Token != "" && serverURL != "" {
			if err := api.New(serverURL, cfg.Token).Logout(ctx); err != nil {
				var apiErr *api.Error
				if !errors.As(err, &apiErr) || !apiErr.Unauthenticated() {
					ui.Warn("the instance could not revoke the session — %s", err)
				}
			}
		}
		if cfg.Token == "" && !unreadable {
			ui.Warn("no session stored — nothing to revoke")
		}

		if err := config.Clear(); err != nil {
			return err
		}
		if os.Getenv(config.TokenEnv) != "" {
			ui.Warn("%s is still set and outranks the stored session — unset it to finish signing out", config.TokenEnv)
		}
		ui.Success("signed out")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}
