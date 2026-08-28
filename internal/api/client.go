// Package api is the HTTP client for a Sonde instance. Every path here is
// transcribed from apps/api: monitor and incident routes come from
// internal/httpapi/router.go, the heartbeat from internal/checker/push.go, and
// the auth routes from the porte kits main.go mounts under /api.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// timeout bounds one call. A probe of somebody else's uptime monitor has no
// business hanging a cron job.
const timeout = 30 * time.Second

// Client talks to one instance with one credential. An empty token is valid:
// the heartbeat route is deliberately unauthenticated.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// New builds a client for an instance root such as https://sonde.example.com.
// The /api prefix belongs to the paths, not to the base, so a bare origin is
// what callers store.
func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTP:    &http.Client{Timeout: timeout},
	}
}

// Error is a refusal from the instance. Sonde answers errors through tronc,
// which writes {"error":{"code","message"}}, so the message is worth showing
// and the status is worth branching on.
type Error struct {
	Status  int
	Code    string
	Message string
}

// Error renders the instance's own words, falling back to the status line when
// the body carried none.
func (e *Error) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("the server refused the request (%d)", e.Status)
	}
	return e.Message
}

// NotFound reports a 404. On the device exchange route it means the instance
// has not shipped the endpoint; on the password login it means SSO_ONLY, which
// unregisters the credential routes rather than rejecting them.
func (e *Error) NotFound() bool { return e.Status == http.StatusNotFound }

// Unauthenticated reports a credential the instance would not accept.
func (e *Error) Unauthenticated() bool {
	return e.Status == http.StatusUnauthorized || e.Status == http.StatusForbidden
}

// do performs one request and decodes the response into out, which may be nil
// for a route that answers 204.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		request.Header.Set("Authorization", "Bearer "+c.Token)
	}

	response, err := c.HTTP.Do(request)
	if err != nil {
		return fmt.Errorf("cannot reach %s — %w", c.BaseURL, err)
	}
	defer response.Body.Close()

	data, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return err
	}
	if response.StatusCode >= 400 {
		return decodeError(response.StatusCode, data)
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("the server answered %s with something that is not JSON", path)
	}
	return nil
}

// decodeError reads tronc's error envelope, keeping the status when the body is
// empty or is something else entirely — an HTML error page from a proxy in
// front of the instance, most often.
func decodeError(status int, data []byte) error {
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(data, &envelope)
	return &Error{Status: status, Code: envelope.Error.Code, Message: envelope.Error.Message}
}

// status performs a request and reports only the status code, for the one
// caller that has to tell "this route is absent" from "this route refused me".
func (c *Client) status(ctx context.Context, method, path string, body any) (int, error) {
	err := c.do(ctx, method, path, body, nil)
	if err == nil {
		return http.StatusOK, nil
	}
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr.Status, nil
	}
	return 0, err
}
