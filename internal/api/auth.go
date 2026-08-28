package api

import (
	"context"
	"net/http"
)

// AuthConfig is what GET /api/auth/config answers. porte serves sso_only and
// oidc_enabled; allow_registration is Sonde's own addition and the login screen
// reads it, not this CLI.
type AuthConfig struct {
	SSOOnly     bool `json:"sso_only"`
	OIDCEnabled bool `json:"oidc_enabled"`
}

// User is the account behind a session, from GET /api/auth/me.
type User struct {
	ID      int64  `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	IsAdmin bool   `json:"is_admin"`
}

// AuthConfig asks the instance what it accepts, so nothing is typed before the
// CLI knows whether a password would even be looked at. It is what makes
// SSO_ONLY survivable: without it, a CLI prompts for a password on an instance
// that has none and the user concludes their password is wrong.
func (c *Client) AuthConfig(ctx context.Context) (AuthConfig, error) {
	var found AuthConfig
	err := c.do(ctx, http.MethodGet, "/api/auth/config", nil, &found)
	return found, err
}

// Exchange trades the loopback flow's one-time code for a session token. The
// code form exists so the token never travels in a query string, where it would
// land in the browser's history.
func (c *Client) Exchange(ctx context.Context, code string) (string, error) {
	var response struct {
		Token string `json:"token"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/auth/oidc/exchange",
		map[string]string{"code": code}, &response); err != nil {
		return "", err
	}
	return response.Token, nil
}

// DeviceExchange trades an access token the provider's device grant already
// issued for this instance's own session token, over porte's frozen
// /auth/oidc/device/exchange contract. What later commands present is a Sonde
// session, not the provider's token: storing the provider's would be a login
// that 401s an hour later.
func (c *Client) DeviceExchange(ctx context.Context, accessToken string) (string, error) {
	var response struct {
		Token string `json:"token"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/auth/oidc/device/exchange",
		map[string]string{"access_token": accessToken}, &response); err != nil {
		return "", err
	}
	return response.Token, nil
}

// ServesDeviceExchange reports whether this instance has shipped the device
// exchange, and it is asked before the grant runs rather than after.
//
// Order is the whole point. Discovering the instance cannot do it only once the
// human has read a code off one screen and typed it into another makes the
// device flow a ceremony that buys nothing: they land on the loopback login
// afterwards anyway, which is the flow that cannot work when the browser is on
// a different machine.
//
// The probe is a POST with an empty body. A route that exists rejects that on
// its merits, which porte does with a 400; a route that does not exist answers
// 404. Only the second is an answer about the endpoint rather than about the
// request, so only the second is read. An unreachable instance is not a 404
// either: that is the login's own problem and it should fail against the real
// flow with the real error.
func (c *Client) ServesDeviceExchange(ctx context.Context) bool {
	status, err := c.status(ctx, http.MethodPost, "/api/auth/oidc/device/exchange", map[string]string{})
	return err == nil && status != http.StatusNotFound
}

// PasswordLogin is the fallback where no identity provider is configured. Under
// SSO_ONLY porte does not register this route at all, so it answers 404 rather
// than 403 and the caller reads that as "this instance has no password login".
func (c *Client) PasswordLogin(ctx context.Context, email, password string) (string, error) {
	var response struct {
		Token string `json:"token"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/auth/login",
		map[string]string{"email": email, "password": password}, &response); err != nil {
		return "", err
	}
	return response.Token, nil
}

// Me names the account a session belongs to.
func (c *Client) Me(ctx context.Context) (User, error) {
	var response struct {
		User User `json:"user"`
	}
	err := c.do(ctx, http.MethodGet, "/api/auth/me", nil, &response)
	return response.User, err
}

// Logout revokes the session server-side. porte makes it idempotent, so a stale
// token is not an error worth stopping a logout for.
func (c *Client) Logout(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/api/auth/logout", nil, nil)
}
