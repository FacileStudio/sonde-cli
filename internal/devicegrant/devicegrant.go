// Package devicegrant runs RFC 8628, the OAuth device authorization grant,
// against the suite's identity provider. It is the flow for a machine whose
// browser is somewhere else: nothing is redirected anywhere, so the user
// carries a short code to whatever device has a browser.
//
// The tokens it returns belong to the provider, not to Sonde. They are traded
// for a Sonde session at porte's device exchange endpoint and never written to
// disk.
package devicegrant

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// GrantType is the grant identifier from RFC 8628 §3.4, spelled exactly as the
// RFC spells it. A provider that has not been taught this string answers
// unsupported_grant_type, which is how the flow reports itself as not ready.
const GrantType = "urn:ietf:params:oauth:grant-type:device_code"

// DefaultIssuer is Registre, the suite's OIDC provider. Only the root discovery
// document is real: a per-application path answers 200 with the dashboard's
// HTML, which is a false positive with no endpoints in it.
const DefaultIssuer = "https://sso.facile.studio"

// IssuerEnv points the grant at another provider, for a Sonde deployed outside
// the suite. The instance still decides what it accepts, so an issuer it does
// not trust produces a refused exchange rather than a session.
const IssuerEnv = "SONDE_OIDC_ISSUER"

// DefaultClientID names the public client Registre registers for the suite's
// command lines. It is public because a secret compiled into a binary that
// ships to laptops and servers is readable by everyone holding a copy, so no
// secret is sent and none exists.
const DefaultClientID = "facile-cli"

// DefaultScopes are what the app needs to name the account behind the token.
// offline_access is registered for this client but not asked for here: a
// refresh token is another credential to store, and the session Sonde issues in
// exchange is the one that has to last.
const DefaultScopes = "openid profile email"

// discoveryPath is the only discovery path worth asking for.
const discoveryPath = "/.well-known/openid-configuration"

// slowDownStep is what RFC 8628 §3.5 requires: on slow_down the client adds
// five seconds to its interval, for this request and every request after it.
// Ignoring it is how a client polls itself into a rate limit and then reports
// the resulting refusal as a failed login.
const slowDownStep = 5 * time.Second

// defaultInterval is the poll cadence when the provider names none.
const defaultInterval = 5 * time.Second

// defaultExpiry bounds the wait when the provider names no expires_in.
const defaultExpiry = 10 * time.Minute

// requestTimeout bounds one call to the provider.
const requestTimeout = 15 * time.Second

// Issuer is the provider to run the grant against.
func Issuer() string {
	if override := strings.TrimSpace(os.Getenv(IssuerEnv)); override != "" {
		return override
	}
	return DefaultIssuer
}

// Provider is the part of a discovery document this package uses.
// OffersDeviceGrant is the provider's own answer to "can you do this at all":
// an advertised endpoint is not an implemented grant, so grant_types_supported
// is what decides.
type Provider struct {
	DeviceAuthorization string
	Token               string
	OffersDeviceGrant   bool
}

// Authorization is the device authorization response, RFC 8628 §3.2.
// URIComplete carries the user code in the URL, so a phone that can follow a
// link never has to type one. It is optional.
type Authorization struct {
	DeviceCode  string
	UserCode    string
	URI         string
	URIComplete string
	Interval    time.Duration
	Expires     time.Duration
}

// Discover reads the endpoints out of the provider's discovery document rather
// than assembling paths from the issuer, so a provider that moves an endpoint
// moves it for this CLI too and nobody has to ship a release for it.
func Discover(ctx context.Context, issuer string) (Provider, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimSuffix(issuer, "/")+discoveryPath, nil)
	if err != nil {
		return Provider{}, err
	}
	status, doc, err := send(request)
	if err != nil {
		return Provider{}, err
	}
	if status < 200 || status > 299 || doc == nil {
		return Provider{}, fmt.Errorf("%s served no OpenID configuration (%d)", issuer, status)
	}

	found := Provider{
		DeviceAuthorization: text(doc, "device_authorization_endpoint"),
		Token:               text(doc, "token_endpoint"),
	}
	grants, _ := doc["grant_types_supported"].([]any)
	for _, grant := range grants {
		if named, ok := grant.(string); ok && named == GrantType {
			found.OffersDeviceGrant = found.DeviceAuthorization != "" && found.Token != ""
		}
	}
	return found, nil
}

// Authorize is RFC 8628 §3.1. It sends no client secret, because this is a
// public client.
func (p Provider) Authorize(ctx context.Context, clientID, scopes string) (Authorization, error) {
	status, doc, err := p.postForm(ctx, p.DeviceAuthorization, url.Values{
		"client_id": {clientID},
		"scope":     {scopes},
	})
	if err != nil {
		return Authorization{}, err
	}
	if status < 200 || status > 299 {
		return Authorization{}, fmt.Errorf(
			"the provider would not start a device sign-in (%d: %s)", status, refusal(doc))
	}

	auth := Authorization{
		DeviceCode:  text(doc, "device_code"),
		UserCode:    text(doc, "user_code"),
		URI:         text(doc, "verification_uri"),
		URIComplete: text(doc, "verification_uri_complete"),
		Interval:    seconds(doc, "interval", defaultInterval),
		Expires:     seconds(doc, "expires_in", defaultExpiry),
	}
	if auth.DeviceCode == "" || auth.UserCode == "" || auth.URI == "" {
		return Authorization{}, fmt.Errorf("the provider's device authorization was not usable")
	}
	return auth, nil
}

// schedule is the polling clock. The interval lives here rather than being
// recomputed from each response because slow_down is cumulative: two of them
// mean ten seconds more, not five. step is a field so a test can prove that
// arithmetic without sleeping through it.
type schedule struct {
	interval time.Duration
	step     time.Duration
	deadline time.Time
}

// wait sleeps out one polling interval and reports whether there was still time
// left to poll at all.
func (s *schedule) wait(ctx context.Context) bool {
	if !time.Now().Before(s.deadline) {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(s.interval):
		return true
	}
}

// slower applies RFC 8628 §3.5's slow_down to every request from here on.
func (s *schedule) slower() { s.interval += s.step }

// AwaitToken polls the token endpoint until the user approves, refuses, or runs
// out of time, backing off by five seconds every time the provider answers
// slow_down.
func (p Provider) AwaitToken(ctx context.Context, clientID string, auth Authorization) (string, error) {
	return p.await(ctx, clientID, auth, &schedule{
		interval: auth.Interval,
		step:     slowDownStep,
		deadline: time.Now().Add(auth.Expires),
	})
}

// await is AwaitToken with the clock supplied. A transport error is retried
// rather than fatal: the deadline already bounds the loop, and a dropped packet
// on the fourth poll is no reason to make somebody go and type a new code.
func (p Provider) await(ctx context.Context, clientID string, auth Authorization, poll *schedule) (string, error) {
	for poll.wait(ctx) {
		status, doc, err := p.postForm(ctx, p.Token, url.Values{
			"grant_type":  {GrantType},
			"device_code": {auth.DeviceCode},
			"client_id":   {clientID},
		})
		if err != nil {
			continue
		}
		token, slower, err := read(status, doc)
		switch {
		case err != nil:
			return "", err
		case token != "":
			return token, nil
		case slower:
			poll.slower()
		}
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return "", fmt.Errorf("the code expired after %s without being approved — run `sonde login` again",
		auth.Expires.Round(time.Second))
}

// read turns one poll into the three things that can happen: the grant
// completed, keep waiting, or stop. RFC 8628 §3.5's errors are not
// interchangeable — telling somebody their code expired when they in fact
// refused it sends them to retry the thing they meant to stop.
func read(status int, doc map[string]any) (string, bool, error) {
	if status >= 200 && status <= 299 {
		token := text(doc, "access_token")
		if token == "" {
			return "", false, fmt.Errorf("the provider approved this machine but returned no access token")
		}
		return token, false, nil
	}

	switch text(doc, "error") {
	case "authorization_pending":
		return "", false, nil
	case "slow_down":
		return "", true, nil
	case "access_denied":
		return "", false, fmt.Errorf(
			"the sign-in was refused at the provider — run `sonde login` again if that was not deliberate")
	case "expired_token":
		return "", false, fmt.Errorf("the provider expired the code before it was approved — run `sonde login` again")
	}
	return "", false, fmt.Errorf("the provider refused the device grant (%d: %s)", status, refusal(doc))
}

// postForm sends application/x-www-form-urlencoded, which is what an OAuth
// endpoint takes. Every other request this CLI makes is JSON.
func (p Provider) postForm(ctx context.Context, target string, form url.Values) (int, map[string]any, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(form.Encode()))
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return send(request)
}

// send performs a request and decodes whatever JSON came back. A body that is
// not JSON decodes to nil rather than failing, because the status line is still
// worth reporting.
func send(request *http.Request) (int, map[string]any, error) {
	request.Header.Set("Accept", "application/json")
	response, err := (&http.Client{Timeout: requestTimeout}).Do(request)
	if err != nil {
		return 0, nil, fmt.Errorf("cannot reach the identity provider — %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return response.StatusCode, nil, err
	}
	var doc map[string]any
	_ = json.Unmarshal(body, &doc)
	return response.StatusCode, doc, nil
}

// refusal is the most useful sentence an OAuth error body carries. RFC 6749
// §5.2 makes error_description optional, so this falls through to the code
// itself rather than printing an empty reason.
func refusal(doc map[string]any) string {
	for _, key := range []string{"error_description", "error"} {
		if value := text(doc, key); value != "" {
			return value
		}
	}
	return "no reason given"
}

func text(doc map[string]any, key string) string {
	value, _ := doc[key].(string)
	return value
}

func seconds(doc map[string]any, key string, fallback time.Duration) time.Duration {
	value, _ := doc[key].(float64)
	if value <= 0 {
		return fallback
	}
	return time.Duration(value) * time.Second
}
