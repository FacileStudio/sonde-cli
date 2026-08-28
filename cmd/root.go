package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/sonde-cli/internal/api"
	"github.com/FacileStudio/sonde-cli/internal/config"
	"github.com/FacileStudio/sonde-cli/internal/ui"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "sonde",
	Short: "Uptime monitoring for the Facile suite",
	Long: `Companion CLI for a Sonde uptime-monitoring instance. It pushes heartbeats
from cron jobs and reads monitors, status and incidents from the shell.

The instance is resolved highest first: --instance, then SONDE_SERVER_URL, then
what login stored. The credential follows the same ladder without the flag:
SONDE_TOKEN, then the stored session.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// ErrInterrupted is Ctrl-C. It exits 130 rather than 1, so a script can tell a
// cancelled run from a failed one.
var ErrInterrupted = errors.New("interrupted")

func init() {
	rootCmd.Version = version
	rootCmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().String("instance", "", "Sonde instance URL")
	cobra.OnInitialize(func() {
		if disabled, _ := rootCmd.PersistentFlags().GetBool("no-color"); disabled {
			ui.DisableColor()
		}
	})
}

// Execute runs the command tree. Cobra's own usage dump is silenced, so an
// error prints as one line on stderr in the vocabulary every other message
// uses. Ctrl-C exits 130 so a script can tell a cancelled run from a failed one.
func Execute() {
	err := rootCmd.Execute()
	switch {
	case err == nil:
		return
	case errors.Is(err, ErrInterrupted):
		os.Exit(130)
	default:
		ui.Error("%s", err)
		os.Exit(1)
	}
}

// signalContext cancels on Ctrl-C or SIGTERM so a login waiting on a browser,
// or a poll waiting on an approval, stops when asked.
func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// instanceFlag reads the --instance flag from any command in the tree.
func instanceFlag(cmd *cobra.Command) string {
	value, _ := cmd.Flags().GetString("instance")
	return value
}

// resolveURL applies the instance precedence ladder to a loaded configuration.
func resolveURL(cmd *cobra.Command, stored string) (string, error) {
	resolved := config.ResolveURL(stored, instanceFlag(cmd))
	if resolved == "" {
		return "", fmt.Errorf("no Sonde instance known — run `sonde login <url>` or set %s", config.URLEnv)
	}
	return resolved, nil
}

// newClient builds a client for the resolved instance. requireToken is false
// for the heartbeat, which is the one route that authenticates with the token
// in its own path rather than with a session.
func newClient(cmd *cobra.Command, requireToken bool) (*api.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	serverURL, err := resolveURL(cmd, cfg.URL)
	if err != nil {
		return nil, err
	}
	token := config.ResolveToken(cfg.Token)
	if requireToken && token == "" {
		return nil, fmt.Errorf("not signed in — run `sonde login` or set %s", config.TokenEnv)
	}
	return api.New(serverURL, token), nil
}

// wrapInterrupt reports a cancelled context as the interrupt it was, so Ctrl-C
// does not print as a transport failure and does not exit like one either.
func wrapInterrupt(ctx context.Context, err error) error {
	if err != nil && errors.Is(ctx.Err(), context.Canceled) {
		return ErrInterrupted
	}
	return err
}
