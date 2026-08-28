package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPushPostsTheHeartbeatRoute pins the flagship call. Sonde registers the
// heartbeat as POST /api/push/{token}; a GET, or a missing /api prefix, is a
// cron line that reports success while the monitor goes down.
func TestPushPostsTheHeartbeatRoute(t *testing.T) {
	var method, path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	if err := New(server.URL, "").Push(context.Background(), "tok-123"); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if method != http.MethodPost {
		t.Errorf("method = %s, want POST", method)
	}
	if path != "/api/push/tok-123" {
		t.Errorf("path = %s, want /api/push/tok-123", path)
	}
}

// TestPushReportsAnUnknownTokenAsNotFound keeps the 404 distinguishable, since
// that is how a typo in a cron line has to surface.
func TestPushReportsAnUnknownTokenAsNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"unknown push token"}}`))
	}))
	defer server.Close()

	err := New(server.URL, "").Push(context.Background(), "nope")
	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("error type = %T, want *api.Error", err)
	}
	if !apiErr.NotFound() || apiErr.Message != "unknown push token" {
		t.Fatalf("error = %+v, want a 404 carrying the server's message", apiErr)
	}
}

// TestServesDeviceExchangeReadsOnlyTheAbsence pins the probe rule porte froze:
// a 404 means the instance has not shipped the endpoint, and anything else —
// including the 400 an empty body earns — means it has.
func TestServesDeviceExchangeReadsOnlyTheAbsence(t *testing.T) {
	cases := map[int]bool{
		http.StatusNotFound:     false,
		http.StatusBadRequest:   true,
		http.StatusUnauthorized: true,
	}

	for status, want := range cases {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/auth/oidc/device/exchange" || r.Method != http.MethodPost {
				t.Errorf("probe hit %s %s", r.Method, r.URL.Path)
			}
			w.WriteHeader(status)
		}))

		if got := New(server.URL, "").ServesDeviceExchange(context.Background()); got != want {
			t.Errorf("ServesDeviceExchange on %d = %v, want %v", status, got, want)
		}
		server.Close()
	}
}

// TestMonitorsDecodesABareArray pins the list shape. The routes answer a JSON
// array, not an object with a "monitors" key, so a wrapper struct silently
// yields an empty list on a healthy instance.
func TestMonitorsDecodesABareArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":7,"slug":"web","name":"Web","interval_seconds":60,"enabled":true}]`))
	}))
	defer server.Close()

	monitors, err := New(server.URL, "token").Monitors(context.Background())
	if err != nil {
		t.Fatalf("Monitors: %v", err)
	}
	if len(monitors) != 1 || monitors[0].ID != 7 || monitors[0].IntervalSeconds != 60 {
		t.Fatalf("monitors = %+v, want one monitor id 7 at 60s", monitors)
	}
}
