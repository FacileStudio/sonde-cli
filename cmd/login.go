package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/sonde-cli/internal/api"
	"github.com/FacileStudio/sonde-cli/internal/config"
	"github.com/FacileStudio/sonde-cli/internal/ui"
)

var (
	loginToken      string
	loginTokenStdin bool
	loginNoBrowser  bool
)

// errSSOOnly is what an instance running SSO_ONLY looks like from here: porte
// does not register the local credential routes at all, so the password login
// is not a broken endpoint but an absent one, and no retry will find it.
var errSSOOnly = errors.New(
	"this instance has no password login — it runs single sign-on only, so sign in through the browser or set " +
		config.TokenEnv)

var loginCmd = &cobra.Command{
	Use:   "login [url]",
	Short: "Authenticate with a Sonde instance (default https://sonde.facile.studio)",
	Long: `Stores the instance URL and the session token it returns, so later commands
need neither.

With no URL it signs in to https://sonde.facile.studio. By default it opens
your browser, your existing Facile session completes the login with no prompt,
and the CLI stores the session. Nothing to copy or paste.

A machine with no browser (SSH, CI) falls back to the device grant: it prints
a short code to approve from a logged-in session on any device.

An instance with no identity provider takes an address and a password instead.
Alternatives that skip all of it:

  sonde login <url> --token <token>     use a token minted elsewhere
  sonde login <url> --token-stdin       read that token from stdin`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signalContext()
		defer stop()

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		serverURL := resolveLoginURL(cfg.URL, args)

		token, err := acquireToken(ctx, serverURL)
		if err != nil {
			return err
		}

		cfg.URL = serverURL
		cfg.Token = token
		if err := config.Save(cfg); err != nil {
			return err
		}
		reportIdentity(ctx, serverURL, token)
		ui.Success("signed in to %s", serverURL)
		ui.Hint("credential stored in %s", config.Path())
		return nil
	},
}

// resolveLoginURL applies the precedence the CLI standard requires, highest
// first: the positional argument, --url, SONDE_SERVER_URL, the stored URL, then
// the hosted instance.
func resolveLoginURL(stored string, args []string) string {
	if len(args) == 1 {
		return config.NormalizeURL(args[0])
	}
	return config.ResolveURL(stored, flagURL)
}

// acquireToken produces the session token to store, either from a token the
// user already holds or by running whichever flow the instance supports.
func acquireToken(ctx context.Context, serverURL string) (string, error) {
	switch {
	case loginTokenStdin:
		return readTokenFromStdin()
	case loginToken != "":
		return loginToken, nil
	}

	client := api.New(serverURL, "")
	authConfig, err := client.AuthConfig(ctx)
	if err != nil {
		return "", wrapInterrupt(ctx, err)
	}

	switch {
	case authConfig.OIDCEnabled:
		return federatedLogin(ctx, client, serverURL)
	case authConfig.SSOOnly:
		return "", errSSOOnly
	default:
		ui.Warn("this instance has no single sign-on configured")
		return passwordLogin(ctx, client)
	}
}

// federatedLogin signs in through the browser on this machine when there is
// one, and falls back to the device grant when there is not or when the browser
// path cannot run.
func federatedLogin(ctx context.Context, client *api.Client, serverURL string) (string, error) {
	if browserAvailable() {
		return loopbackLogin(ctx, client, serverURL)
	}
	if !client.ServesDeviceExchange(ctx) {
		return loopbackLogin(ctx, client, serverURL)
	}
	token, err := deviceLogin(ctx, client)
	switch {
	case err == nil:
		return token, nil
	case errors.Is(err, errNoDeviceGrant), errors.Is(err, errNoDeviceExchange):
		ui.Hint("device approval is not available here, using the browser flow")
		return loopbackLogin(ctx, client, serverURL)
	default:
		return "", err
	}
}

// readTokenFromStdin takes a credential off a pipe, which is how a provisioning
// script hands one over without putting it in the process table.
func readTokenFromStdin() (string, error) {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("cannot read the token from stdin — %w", err)
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return "", errors.New("stdin carried no token")
	}
	return token, nil
}

// reportIdentity names who signed in. It is cosmetic, so a failure is silence
// rather than an error on an otherwise successful login.
func reportIdentity(ctx context.Context, serverURL, token string) {
	user, err := api.New(serverURL, token).Me(ctx)
	if err != nil || user.Email == "" {
		return
	}
	ui.Step("signed in as %s", user.Email)
}

func init() {
	loginCmd.Flags().StringVar(&loginToken, "token", "", "Use this token instead of running a login flow")
	loginCmd.Flags().BoolVar(&loginTokenStdin, "token-stdin", false, "Read the token from stdin")
	loginCmd.Flags().BoolVar(&loginNoBrowser, "no-browser", false, "Print URLs instead of opening a browser")
	rootCmd.AddCommand(loginCmd)
}
