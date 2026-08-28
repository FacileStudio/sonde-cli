package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Monitor is one watched thing. The field names are Sonde's wire names, which
// spell the cadence out in seconds rather than calling it "interval".
type Monitor struct {
	ID              int64     `json:"id"`
	Slug            string    `json:"slug"`
	Name            string    `json:"name"`
	Type            string    `json:"type"`
	Target          string    `json:"target"`
	IntervalSeconds int       `json:"interval_seconds"`
	TimeoutSeconds  int       `json:"timeout_seconds"`
	ExpectedStatus  int       `json:"expected_status"`
	ExpectedKeyword string    `json:"expected_keyword"`
	Enabled         bool      `json:"enabled"`
	PushToken       *string   `json:"push_token,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// MonitorInput is what POST /api/monitors takes. Slug is not optional: the
// server validates it first and refuses anything that is not lowercase letters,
// digits and dashes.
type MonitorInput struct {
	Slug            string `json:"slug"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	Target          string `json:"target"`
	IntervalSeconds int    `json:"interval_seconds"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
	ExpectedStatus  int    `json:"expected_status"`
	ExpectedKeyword string `json:"expected_keyword"`
}

// Incident is one downtime episode. ResolvedAt is nil while it is still open,
// which is the only authenticated signal that a monitor is down right now.
type Incident struct {
	ID         int64      `json:"id"`
	MonitorID  int64      `json:"monitor_id"`
	OpenedAt   time.Time  `json:"opened_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
	Cause      string     `json:"cause"`
}

// Open reports whether this incident is still running.
func (i Incident) Open() bool { return i.ResolvedAt == nil }

// Uptime is one window's summary. The ratio is a pointer because a monitor that
// has never been checked has no uptime, and reporting that as 0% would be a
// fabricated outage.
type Uptime struct {
	Window string   `json:"window"`
	Uptime *float64 `json:"uptime"`
	Total  int64    `json:"total"`
	Failed int64    `json:"failed"`
}

// PublicIncident is an open incident as a status page shows it.
type PublicIncident struct {
	ID       int64     `json:"id"`
	OpenedAt time.Time `json:"opened_at"`
	Cause    string    `json:"cause"`
}

// PublicMonitor is one line of a status page. CurrentStatus is up or down from
// the newest raw check, and unknown for a monitor that has never been checked.
type PublicMonitor struct {
	ID            int64            `json:"id"`
	Slug          string           `json:"slug"`
	Name          string           `json:"name"`
	CurrentStatus string           `json:"current_status"`
	Uptime24h     *float64         `json:"uptime_24h"`
	Uptime7d      *float64         `json:"uptime_7d"`
	OpenIncidents []PublicIncident `json:"open_incidents"`
}

// PublicStatusPage is the unauthenticated feed a status page renders from.
type PublicStatusPage struct {
	Slug     string          `json:"slug"`
	Title    string          `json:"title"`
	Monitors []PublicMonitor `json:"monitors"`
}

// Monitors lists the caller's monitors. The route answers a bare JSON array,
// not an object with a key in it.
func (c *Client) Monitors(ctx context.Context) ([]Monitor, error) {
	var found []Monitor
	err := c.do(ctx, http.MethodGet, "/api/monitors", nil, &found)
	return found, err
}

// CreateMonitor adds a monitor and returns it as stored, which is where a push
// monitor's token comes from: the server mints it and shows it here.
func (c *Client) CreateMonitor(ctx context.Context, input MonitorInput) (Monitor, error) {
	var created Monitor
	err := c.do(ctx, http.MethodPost, "/api/monitors", input, &created)
	return created, err
}

// DeleteMonitor removes a monitor by id. The route takes an id and nothing
// else, so a name or a slug has to be resolved before calling it.
func (c *Client) DeleteMonitor(ctx context.Context, id int64) error {
	return c.do(ctx, http.MethodDelete, "/api/monitors/"+strconv.FormatInt(id, 10), nil, nil)
}

// Incidents lists incidents newest first, optionally for one monitor. The
// server caps the list at 200.
func (c *Client) Incidents(ctx context.Context, monitorID int64) ([]Incident, error) {
	path := "/api/incidents"
	if monitorID > 0 {
		path += "?monitor_id=" + strconv.FormatInt(monitorID, 10)
	}
	var found []Incident
	err := c.do(ctx, http.MethodGet, path, nil, &found)
	return found, err
}

// MonitorUptime summarises one monitor over one window, which must be one of
// 24h, 7d, 30d or 90d.
func (c *Client) MonitorUptime(ctx context.Context, id int64, window string) (Uptime, error) {
	var found Uptime
	path := fmt.Sprintf("/api/monitors/%d/uptime?window=%s", id, url.QueryEscape(window))
	err := c.do(ctx, http.MethodGet, path, nil, &found)
	return found, err
}

// PublicStatus reads a published status page. It needs no credential, which is
// the point: it is the readout somebody can check from a machine that has never
// logged in.
func (c *Client) PublicStatus(ctx context.Context, slug string) (PublicStatusPage, error) {
	var page PublicStatusPage
	err := c.do(ctx, http.MethodGet, "/api/public/status/"+url.PathEscape(slug), nil, &page)
	return page, err
}

// Push sends a heartbeat for a push monitor. The route is unauthenticated by
// design — the token in the path is the whole credential — and it is a POST,
// so a link preview or a prefetch cannot mark a dead job alive.
func (c *Client) Push(ctx context.Context, token string) error {
	return c.do(ctx, http.MethodPost, "/api/push/"+url.PathEscape(token), nil, nil)
}
