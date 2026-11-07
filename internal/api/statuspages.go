package api

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// StatusPageMonitor is one line of a status page, in display order.
type StatusPageMonitor struct {
	ID       int64  `json:"id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Position int    `json:"position"`
}

// StatusPage is a public page and the monitors it lists.
type StatusPage struct {
	ID        int64               `json:"id"`
	Slug      string              `json:"slug"`
	Title     string              `json:"title"`
	Monitors  []StatusPageMonitor `json:"monitors"`
	CreatedAt time.Time           `json:"created_at"`
}

// StatusPageInput is what POST and PATCH take. PATCH replaces the page, so a
// caller changing one field sends the others unchanged.
type StatusPageInput struct {
	Slug       string  `json:"slug"`
	Title      string  `json:"title"`
	MonitorIDs []int64 `json:"monitor_ids"`
}

// StatusPages lists every page. It needs an admin session.
func (c *Client) StatusPages(ctx context.Context) ([]StatusPage, error) {
	var found []StatusPage
	err := c.do(ctx, http.MethodGet, "/api/status-pages", nil, &found)
	return found, err
}

// CreateStatusPage adds a page.
func (c *Client) CreateStatusPage(ctx context.Context, input StatusPageInput) (StatusPage, error) {
	var created StatusPage
	err := c.do(ctx, http.MethodPost, "/api/status-pages", input, &created)
	return created, err
}

// UpdateStatusPage replaces a page's slug, title and monitor list.
func (c *Client) UpdateStatusPage(ctx context.Context, id int64, input StatusPageInput) (StatusPage, error) {
	var updated StatusPage
	err := c.do(ctx, http.MethodPatch, "/api/status-pages/"+strconv.FormatInt(id, 10), input, &updated)
	return updated, err
}

// DeleteStatusPage removes a page.
func (c *Client) DeleteStatusPage(ctx context.Context, id int64) error {
	return c.do(ctx, http.MethodDelete, "/api/status-pages/"+strconv.FormatInt(id, 10), nil, nil)
}
