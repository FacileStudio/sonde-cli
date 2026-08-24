package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{BaseURL: baseURL, Token: token, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) do(method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		msg := strings.TrimSpace(string(data))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("%s: %s", resp.Status, msg)
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("invalid response: %w", err)
		}
	}
	return nil
}

func (c *Client) Get(path string, out any) error { return c.do(http.MethodGet, path, nil, out) }
func (c *Client) Post(path string, body, out any) error {
	return c.do(http.MethodPost, path, body, out)
}
func (c *Client) Delete(path string) error { return c.do(http.MethodDelete, path, nil, nil) }

type Monitor struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Target   string `json:"target"`
	Interval int    `json:"interval"`
	Enabled  bool   `json:"enabled"`
	Status   string `json:"status,omitempty"`
}

type Incident struct {
	ID         int        `json:"id"`
	MonitorID  int        `json:"monitor_id"`
	Cause      string     `json:"cause"`
	OpenedAt   time.Time  `json:"opened_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

func (c *Client) ListMonitors() ([]Monitor, error) {
	var res struct {
		Monitors []Monitor `json:"monitors"`
	}
	if err := c.Get("/api/monitors", &res); err != nil {
		return nil, err
	}
	return res.Monitors, nil
}

func (c *Client) AddMonitor(m Monitor) (*Monitor, error) {
	var created Monitor
	if err := c.Post("/api/monitors", m, &created); err != nil {
		return nil, err
	}
	return &created, nil
}

func (c *Client) RemoveMonitor(idOrName string) error {
	return c.Delete("/api/monitors/" + idOrName)
}

func (c *Client) Status() ([]Monitor, error) { return c.ListMonitors() }

func (c *Client) Incidents() ([]Incident, error) {
	var res struct {
		Incidents []Incident `json:"incidents"`
	}
	if err := c.Get("/api/incidents", &res); err != nil {
		return nil, err
	}
	return res.Incidents, nil
}

func (c *Client) Push(token string) error {
	return c.do(http.MethodGet, "/api/push/"+token, nil, nil)
}
