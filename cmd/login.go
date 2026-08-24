package cmd

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/FacileStudio/sonde-cli/internal/config"
	"github.com/FacileStudio/sonde-cli/internal/ui"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	loginToken      string
	loginTokenStdin bool
	loginNoBrowser  bool
)

var loginCmd = &cobra.Command{
	Use:   "login [url]",
	Short: "Authenticate with a Sonde instance",
	Long: `Authenticate with a Sonde instance.

By default this signs you in through your browser against the instance's
identity provider (porte), so a session already open with another Facile tool
completes the login without a second prompt. A machine with no browser falls
back to the device authorization flow. Alternatives:

  sonde login <url> --token <token>     use a token from the dashboard
  sonde login <url> --token-stdin       read the token from stdin

The URL may be omitted once SONDE_SERVER_URL (or SONDE_INSTANCE) or a previous
login has set one.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		serverURL := cfg.ServerURL()
		if len(args) == 1 {
			serverURL = strings.TrimRight(args[0], "/")
		}
		if flag := instanceFlag(cmd); flag != "" {
			serverURL = flag
		}
		if serverURL == "" {
			return fmt.Errorf("no instance known — run 'sonde login <url>' or set %s", config.URLEnv)
		}

		var token string
		switch {
		case loginTokenStdin:
			raw, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read token: %w", err)
			}
			token = strings.TrimSpace(string(raw))
			if token == "" {
				return fmt.Errorf("empty token")
			}
		case loginToken != "":
			token = loginToken
		default:
			if browserAvailable() && serverOffersSSO(serverURL) {
				token, err = ssoLogin(serverURL)
			} else {
				token, err = deviceLogin(serverURL)
			}
			if err != nil {
				return err
			}
		}

		cfg.URL = serverURL
		cfg.Token = token
		if err := config.Save(cfg); err != nil {
			return err
		}
		color.Green("Logged in to %s", serverURL)
		return nil
	},
}

const ssoWait = 3 * time.Minute

var errCallbackMismatch = errors.New("the sign-in callback did not match this login attempt — run 'sonde login' again")

func browserAvailable() bool {
	if loginNoBrowser || !term.IsTerminal(int(os.Stdout.Fd())) {
		return false
	}
	switch runtime.GOOS {
	case "darwin", "windows":
		return true
	}
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

type discovery struct {
	OIDCEnabled   bool `json:"oidc_enabled"`
	DeviceEnabled bool `json:"device_enabled"`
}

func fetchDiscovery(serverURL string) (*discovery, bool) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(serverURL + "/auth/config")
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	var d discovery
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, false
	}
	return &d, true
}

func serverOffersSSO(serverURL string) bool {
	d, ok := fetchDiscovery(serverURL)
	return ok && d.OIDCEnabled
}

func ssoLogin(serverURL string) (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("cannot open a loopback port to receive the login: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	nonce, err := loginNonce()
	if err != nil {
		listener.Close()
		return "", err
	}

	authURL := fmt.Sprintf("%s/auth/oidc?flow=cli&port=%d&cli_state=%s", serverURL, port, nonce)
	fmt.Println()
	fmt.Println("To sign in, open this URL in your browser:")
	color.Cyan("  %s", authURL)
	fmt.Println()
	openBrowser(authURL)
	fmt.Print("Waiting for the browser")

	code, err := awaitLoginCode(listener, nonce)
	fmt.Println()
	if err != nil {
		return "", err
	}

	status, body, err := postJSON(serverURL+"/auth/oidc/exchange", map[string]string{"code": code})
	if err != nil {
		return "", fmt.Errorf("connection failed: %w", err)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("sign-in failed: %s", strings.TrimSpace(string(body)))
	}
	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("invalid response: %w", err)
	}
	if result.Token == "" {
		return "", fmt.Errorf("the server returned no token")
	}
	return result.Token, nil
}

func awaitLoginCode(listener net.Listener, nonce string) (string, error) {
	type outcome struct {
		code string
		err  error
	}
	done := make(chan outcome, 1)

	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Not the login redirect.", http.StatusNotFound)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(nonce)) != 1 {
			http.Error(w, "The callback did not match this login attempt. Run `sonde login` again.", http.StatusBadRequest)
			select {
			case done <- outcome{err: errCallbackMismatch}:
			default:
			}
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "Signed in. You can close this tab and go back to your terminal.")
		select {
		case done <- outcome{code: code}:
		default:
		}
	})}
	go server.Serve(listener)

	var result outcome
	select {
	case result = <-done:
	case <-time.After(ssoWait):
		result = outcome{err: fmt.Errorf("timed out waiting for the browser — run 'sonde login' again")}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	server.Shutdown(ctx)
	return result.code, result.err
}

func deviceLogin(serverURL string) (string, error) {
	status, body, err := postJSON(serverURL+"/auth/device/start", map[string]string{})
	if err != nil {
		return "", fmt.Errorf("connection failed: %w", err)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("could not start authorization: %s", strings.TrimSpace(string(body)))
	}

	var start struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
		VerifyURL  string `json:"verification_uri_complete"`
		Interval   int    `json:"interval"`
		ExpiresIn  int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &start); err != nil {
		return "", fmt.Errorf("invalid response: %w", err)
	}
	if start.Interval <= 0 {
		start.Interval = 5
	}

	fmt.Println()
	fmt.Println("To authorize this machine, open this URL in your browser:")
	color.Cyan("  %s", start.VerifyURL)
	fmt.Printf("\n  and confirm the code: ")
	color.New(color.Bold).Printf("%s\n\n", start.UserCode)
	if !loginNoBrowser && term.IsTerminal(int(os.Stdout.Fd())) {
		openBrowser(start.VerifyURL)
	}
	fmt.Print("Waiting for approval")

	deadline := time.Now().Add(time.Duration(start.ExpiresIn) * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(time.Duration(start.Interval) * time.Second)
		fmt.Print(".")

		status, body, err := postJSON(serverURL+"/auth/device/poll", map[string]string{"device_code": start.DeviceCode})
		if err != nil {
			continue
		}
		switch status {
		case http.StatusOK:
			var res struct {
				Token string `json:"token"`
			}
			if err := json.Unmarshal(body, &res); err != nil {
				return "", fmt.Errorf("invalid response: %w", err)
			}
			fmt.Println()
			return res.Token, nil
		case http.StatusBadRequest, http.StatusForbidden:
			fmt.Println()
			return "", fmt.Errorf("authorization failed: %s", strings.TrimSpace(string(body)))
		default:
			continue
		}
	}
	fmt.Println()
	return "", fmt.Errorf("authorization timed out — run `sonde login` again")
}

func loginNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("cannot generate a login nonce: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func postJSON(url string, body any) (int, []byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err
	}
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	return resp.StatusCode, raw, err
}

func openBrowser(url string) {
	var cmdExec *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmdExec = exec.Command("open", url)
	case "windows":
		cmdExec = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmdExec = exec.Command("xdg-open", url)
	}
	if err := cmdExec.Start(); err != nil {
		ui.Warn("Could not open a browser — visit the URL above manually")
	}
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove the stored credential",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		cfg.Token = ""
		if err := config.Save(cfg); err != nil {
			return err
		}
		ui.Success("Logged out")
		return nil
	},
}

func init() {
	loginCmd.Flags().StringVar(&loginToken, "token", "", "Use this token instead of the browser flow")
	loginCmd.Flags().BoolVar(&loginTokenStdin, "token-stdin", false, "Read the token from stdin")
	loginCmd.Flags().BoolVar(&loginNoBrowser, "no-browser", false, "Skip opening the browser automatically")
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
}
