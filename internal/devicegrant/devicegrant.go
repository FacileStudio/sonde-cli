// Package devicegrant runs RFC 8628, the OAuth device authorization grant,
// against the suite's identity provider. It is the flow for a machine whose
// browser is somewhere else: nothing is redirected anywhere, so the user
// carries a short code to whatever device has a browser.
//
// The tokens it returns belong to the provider, not to Sonde. They are traded
// for a Sonde session at porte's device exchange endpoint and never written to
// disk.
//
// The package is three files: this one holds the constants and the transport,
// discovery.go asks the provider what it offers, and poll.go waits for the
// human.
package devicegrant

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// defaultInterval is the poll cadence when the provider names none.
const defaultInterval = 5 * time.Second

// defaultExpiry bounds the wait when the provider names no expires_in.
const defaultExpiry = 10 * time.Minute

// requestTimeout bounds one call to the provider.
const requestTimeout = 15 * time.Second

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

// Authorize is RFC 8628 §3.1. It sends no client secret, because this is a
// public client.
//
// The verification URIs are checked before they are returned. They are the one
// part of this flow that ends up in front of the operating system rather than
// in front of the parser: the caller prints them and offers to open them, so a
// provider that answered with a javascript: or file: URL would be choosing what
// this machine executes.
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
	if err := checkVerificationURIs(auth); err != nil {
		return Authorization{}, err
	}
	return auth, nil
}

// checkVerificationURIs refuses a verification address this CLI should not put
// in front of a human or hand to a browser.
func checkVerificationURIs(auth Authorization) error {
	for _, candidate := range []string{auth.URI, auth.URIComplete} {
		if candidate == "" {
			continue
		}
		if err := requireSecureURL(candidate); err != nil {
			return fmt.Errorf("the provider's verification address is not usable — %w", err)
		}
	}
	return nil
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
