// Package cmd implements the sonde command tree.
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

var (
	flagURL     string
	flagJSON    bool
	flagNoColor bool
)

var rootCmd = &cobra.Command{
	Use:   "sonde",
	Short: "Uptime monitoring for the Facile suite",
	Long: `Companion CLI for a Sonde uptime-monitoring instance. It pushes heartbeats
from cron jobs and reads monitors, status and incidents from the shell.

The instance is resolved highest first: --url, then SONDE_SERVER_URL, then what
login stored. The credential follows the same ladder without the flag:
SONDE_TOKEN, then the stored session.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	// Set once the command's own body starts. Cobra validates flags and args
	// before this runs, so an error arriving with it still false is a usage
	// error rather than a failure of the work — and those exit 2, not 1.
	PersistentPreRun: func(cmd *cobra.Command, args []string) { commandStarted = true },
}

var commandStarted bool

// ErrInterrupted is Ctrl-C. It exits 130 rather than 1, so a script can tell a
// cancelled run from a failed one.
var ErrInterrupted = errors.New("interrupted")

func init() {
	rootCmd.Version = version
	// cobra's default is `<bin> version <v>`, which the installer cannot parse.
	rootCmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")

	rootCmd.PersistentFlags().StringVar(&flagURL, "url", "", "Sonde instance URL, overriding the stored one")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Print one JSON document and nothing else")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "Disable colored output")

	cobra.OnInitialize(func() {
		// Structured output forces colour off: a caller piping JSON into jq
		// must not receive escape codes.
		if flagNoColor || flagJSON {
			ui.DisableColor()
		}
	})
}

// Execute runs the command tree and maps the outcome onto an exit code:
// 0 success, 1 error, 2 usage error, 130 on SIGINT. Cobra's own usage dump is
// silenced, so an error prints as one line on stderr in the vocabulary every
// other message uses.
func Execute() {
	err := rootCmd.Execute()
	switch {
	case err == nil:
		return
	case !commandStarted:
		ui.Error("%s", err)
		ui.Hint("run `sonde <command> --help` for usage")
		os.Exit(2)
	case errors.Is(err, ErrInterrupted):
		// 128 + SIGINT, which is what a shell and every `while` loop expect
		// from a process the user stopped.
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

// resolveURL applies the instance precedence ladder to a loaded configuration.
func resolveURL(stored string) (string, error) {
	resolved := config.ResolveURL(stored, flagURL)
	if resolved == "" {
		return "", fmt.Errorf("no Sonde instance known — run `sonde login <url>` or set %s", config.URLEnv)
	}
	return resolved, nil
}

// newClient builds a client for the resolved instance. requireToken is false
// for the heartbeat, which is the one route that authenticates with the token
// in its own path rather than with a session.
func newClient(requireToken bool) (*api.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	serverURL, err := resolveURL(cfg.URL)
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
