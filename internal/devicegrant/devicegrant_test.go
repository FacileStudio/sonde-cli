package devicegrant

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestDiscoverRequiresTheGrantNotJustTheEndpoint pins the switch that lets this
// CLI ask for the device flow before a provider serves it. Registre advertised
// device_authorization_endpoint while refusing the grant behind it, so
// grant_types_supported is what decides.
func TestDiscoverRequiresTheGrantNotJustTheEndpoint(t *testing.T) {
	cases := map[string]struct {
		grants string
		want   bool
	}{
		"grant listed":     {fmt.Sprintf(`["authorization_code",%q]`, GrantType), true},
		"endpoint only":    {`["authorization_code"]`, false},
		"no grants at all": {`[]`, false},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/.well-known/openid-configuration" {
					t.Errorf("discovery hit %s, want the root document", r.URL.Path)
				}
				fmt.Fprintf(w, `{"device_authorization_endpoint":"%s/device",`+
					`"token_endpoint":"%s/token","grant_types_supported":%s}`,
					"http://x", "http://x", testCase.grants)
			}))
			defer server.Close()

			provider, err := Discover(context.Background(), server.URL)
			if err != nil {
				t.Fatalf("Discover: %v", err)
			}
			if provider.OffersDeviceGrant != testCase.want {
				t.Fatalf("OffersDeviceGrant = %v, want %v", provider.OffersDeviceGrant, testCase.want)
			}
		})
	}
}

// TestAwaitTokenBacksOffCumulativelyOnSlowDown pins RFC 8628 §3.5. Two
// slow_down answers mean ten seconds more, not five, and a client that resets
// its interval from the response every poll polls itself into a rate limit and
// then reports the refusal as a failed login.
func TestAwaitTokenBacksOffCumulativelyOnSlowDown(t *testing.T) {
	var gaps []time.Duration
	last := time.Now()
	polls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gaps = append(gaps, time.Since(last))
		last = time.Now()
		polls++
		w.Header().Set("Content-Type", "application/json")
		switch polls {
		case 1, 2:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"slow_down"}`))
		default:
			_, _ = w.Write([]byte(`{"access_token":"provider-token"}`))
		}
	}))
	defer server.Close()

	provider := Provider{Token: server.URL, OffersDeviceGrant: true}
	token, err := provider.await(context.Background(), DefaultClientID,
		Authorization{DeviceCode: "device"},
		&schedule{interval: 20 * time.Millisecond, step: 30 * time.Millisecond,
			deadline: time.Now().Add(5 * time.Second)})
	if err != nil {
		t.Fatalf("AwaitToken: %v", err)
	}
	if token != "provider-token" {
		t.Fatalf("token = %q, want provider-token", token)
	}
	if len(gaps) != 3 {
		t.Fatalf("polls = %d, want 3", len(gaps))
	}
	if gaps[2] <= gaps[1] || gaps[1] <= gaps[0] {
		t.Fatalf("intervals did not grow across slow_down: %v", gaps)
	}
}

// TestAwaitTokenStopsOnADenial keeps the four RFC errors distinct: telling
// somebody their code expired when they in fact refused it sends them to retry
// the thing they meant to stop.
func TestAwaitTokenStopsOnADenial(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"access_denied"}`))
	}))
	defer server.Close()

	provider := Provider{Token: server.URL}
	_, err := provider.await(context.Background(), DefaultClientID,
		Authorization{DeviceCode: "device"},
		&schedule{interval: time.Millisecond, step: slowDownStep, deadline: time.Now().Add(time.Second)})
	if err == nil {
		t.Fatal("AwaitToken accepted a denied authorization")
	}
}
