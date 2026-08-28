package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/FacileStudio/sonde-cli/internal/api"
	"github.com/FacileStudio/sonde-cli/internal/devicegrant"
	"github.com/FacileStudio/sonde-cli/internal/loopback"
	"github.com/FacileStudio/sonde-cli/internal/ui"
)

// errNoDeviceGrant means the identity provider does not advertise RFC 8628. It
// is a sentinel because it is the one refusal the caller can do something
// about: falling back to a flow that works beats refusing over a discovery
// document, and it keeps the day the provider gains the grant a non-event.
var errNoDeviceGrant = errors.New("the identity provider does not offer the device grant")

// errNoDeviceExchange means this instance has not shipped porte's device
// exchange. It is a separate sentinel from errNoDeviceGrant because the two
// name different machines: one is the provider's answer and one is the
// instance's, and blaming the wrong one sends whoever is fixing it to the wrong
// server.
var errNoDeviceExchange = errors.New("this instance has not shipped the device exchange")

// deviceLogin signs in at the provider without a browser on this machine, then
// trades the provider's access token for a Sonde session at porte's device
// exchange. The provider token never reaches disk.
//
// Only the provider's own "no" falls back. A discovery that could not be made
// at all — DNS, TLS, a timeout, a 500 — is returned as itself, because the
// fallback is the loopback flow and the machine that needs the device grant is
// by definition the machine that cannot run it. Reporting an unreachable
// provider as a provider that declined would send a headless server into a
// browser flow and leave it waiting on a redirect nothing will send.
func deviceLogin(ctx context.Context, client *api.Client) (string, error) {
	issuer, err := devicegrant.Issuer()
	if err != nil {
		return "", err
	}
	provider, err := devicegrant.Discover(ctx, issuer)
	if err != nil {
		return "", wrapInterrupt(ctx, err)
	}
	if !provider.OffersDeviceGrant {
		return "", errNoDeviceGrant
	}

	authorization, err := provider.Authorize(ctx, devicegrant.DefaultClientID, devicegrant.DefaultScopes)
	if err != nil {
		return "", wrapInterrupt(ctx, err)
	}
	announceCode(authorization)

	accessToken, err := provider.AwaitToken(ctx, devicegrant.DefaultClientID, authorization)
	if err != nil {
		return "", wrapInterrupt(ctx, err)
	}

	token, err := client.DeviceExchange(ctx, accessToken)
	if err != nil {
		var apiErr *api.Error
		if errors.As(err, &apiErr) && apiErr.NotFound() {
			return "", errNoDeviceExchange
		}
		return "", wrapInterrupt(ctx, fmt.Errorf("the instance refused the provider's token — %w", err))
	}
	return token, nil
}

// announceCode is the human half of the flow, and the half that decides whether
// any of this works. The code has to survive being read off one screen and
// typed on another device, so it goes on a line of its own and verbatim: the
// provider chooses its shape and inserting a friendlier dash would offer a code
// the provider will not accept.
func announceCode(authorization devicegrant.Authorization) {
	ui.Step("open this page on any device — a phone is fine")
	ui.Hint("%s", authorization.URI)
	ui.Step("and enter this code")
	fmt.Println()
	fmt.Printf("    %s\n", authorization.UserCode)
	fmt.Println()
	if authorization.URIComplete != "" {
		ui.Hint("or open %s, which fills it in for you", authorization.URIComplete)
	}
	if !loginNoBrowser && loopback.OpenBrowser(firstNonEmpty(authorization.URIComplete, authorization.URI)) {
		ui.Hint("a browser opened here too, in case this machine has one")
	}
	ui.Step("waiting for approval — the code lasts %s", authorization.Expires.Round(time.Second))
}

// loopbackLogin runs the porte CLI flow: a listener on loopback, a browser, a
// redirect back carrying a one-time code, and an exchange for the session
// token. The state nonce is minted here and verified by the listener, so a
// callback belonging to another login is refused.
func loopbackLogin(ctx context.Context, client *api.Client, serverURL string) (string, error) {
	listener, err := loopback.Listen()
	if err != nil {
		return "", err
	}
	defer listener.Close()

	state, err := loopback.RandomState()
	if err != nil {
		return "", err
	}

	target := listener.LoginURL(serverURL, state)
	if loginNoBrowser || !loopback.OpenBrowser(target) {
		ui.Step("open this URL to sign in")
		ui.Hint("%s", target)
	} else {
		ui.Step("opening your browser to sign in")
		ui.Hint("if nothing opened, run login again with --no-browser and open the URL yourself")
	}

	ui.Step("waiting for the login to complete")
	code, err := listener.WaitForCode(ctx, state)
	if err != nil {
		return "", wrapInterrupt(ctx, err)
	}

	token, err := client.Exchange(ctx, code)
	if err != nil {
		return "", wrapInterrupt(ctx,
			fmt.Errorf("the login code was refused — run login again, a code works once and expires quickly: %w", err))
	}
	return token, nil
}

// passwordLogin exchanges an address and a password for a session token,
// prompting only for what stdin can answer. A pipe or an agent harness gets an
// error naming the alternative rather than a prompt nobody can see, because a
// CLI blocked on a hidden prompt hangs its caller until the timeout.
func passwordLogin(ctx context.Context, client *api.Client) (string, error) {
	email, err := askLine("Email")
	if err != nil {
		return "", err
	}
	password, err := askSecret("Password")
	if err != nil {
		return "", err
	}

	token, err := client.PasswordLogin(ctx, email, password)
	if err != nil {
		var apiErr *api.Error
		if errors.As(err, &apiErr) {
			switch {
			case apiErr.NotFound():
				return "", errSSOOnly
			case apiErr.Unauthenticated():
				return "", errors.New("the address or password was refused — check both and try again")
			}
		}
		return "", wrapInterrupt(ctx, err)
	}
	return token, nil
}

// askLine reads one visible answer from a terminal.
func askLine(prompt string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("%s is required and stdin is not a terminal — use --token-stdin instead",
			strings.ToLower(prompt))
	}
	fmt.Fprintf(os.Stderr, "%s: ", prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	answer := strings.TrimSpace(line)
	if answer == "" {
		return "", fmt.Errorf("%s is required", strings.ToLower(prompt))
	}
	return answer, nil
}

// askSecret reads one answer without echoing it, and prints the newline the
// terminal did not.
func askSecret(prompt string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("%s is required and stdin is not a terminal — use --token-stdin instead",
			strings.ToLower(prompt))
	}
	fmt.Fprintf(os.Stderr, "%s: ", prompt)
	entered, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if len(entered) == 0 {
		return "", fmt.Errorf("%s is required", strings.ToLower(prompt))
	}
	return string(entered), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
