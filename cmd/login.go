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
	Short: "Authenticate with a Sonde instance",
	Long: `Stores the instance URL and the session token it returns, so later commands
need neither.

The instance is asked what it accepts before anything is typed. Where it offers
single sign-on there are two browser paths and the CLI picks between them:

  * the device grant at sso.facile.studio, for a machine whose browser is
    somewhere else. It prints a short code to carry to a phone, and trades the
    resulting provider token for a Sonde session.
  * the loopback flow, when the instance has not shipped the device exchange or
    the provider does not offer the grant. The browser must be on this machine.

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
		serverURL, err := resolveLoginURL(cmd, cfg.URL, args)
		if err != nil {
			return err
		}

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
// first: the positional argument, --instance, SONDE_SERVER_URL, then the stored
// URL. Sonde is self-hosted and has no built-in default to fall back on.
func resolveLoginURL(cmd *cobra.Command, stored string, args []string) (string, error) {
	resolved := config.ResolveURL(stored, instanceFlag(cmd))
	if len(args) == 1 {
		resolved = config.NormalizeURL(args[0])
	}
	if resolved == "" {
		return "", fmt.Errorf("no instance given — run `sonde login <url>` or set %s", config.URLEnv)
	}
	return resolved, nil
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

// federatedLogin prefers the device grant and falls back to the loopback flow.
//
// Both halves of the question are asked before any code is printed. The
// provider offering RFC 8628 says nothing about whether this instance can trade
// the resulting token for a Sonde session, and instances gain that endpoint one
// deployment at a time. Asking afterwards would make the human read a code off
// one screen, type it into another, wait for the poll to clear, and then land on
// the loopback login anyway — which is the flow that cannot work when the
// browser is elsewhere.
func federatedLogin(ctx context.Context, client *api.Client, serverURL string) (string, error) {
	if client.ServesDeviceExchange(ctx) {
		token, err := deviceLogin(ctx, client)
		if err == nil {
			return token, nil
		}
		if !errors.Is(err, errNoDeviceGrant) {
			return "", err
		}
		ui.Hint("the identity provider does not offer the device grant — using the browser on this machine")
	}
	return loopbackLogin(ctx, client, serverURL)
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
