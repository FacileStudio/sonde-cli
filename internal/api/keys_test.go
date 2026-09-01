package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestListKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/apikeys" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("app") == "otherapp" {
			_ = json.NewEncoder(w).Encode(ListKeysResponse{Keys: []Key{}})
			return
		}
		createdAt, _ := time.Parse(time.RFC3339, "2026-09-01T15:00:00Z")
		_ = json.NewEncoder(w).Encode(ListKeysResponse{
			Keys: []Key{
				{
					ID:             1,
					App:            "studio",
					Kind:           "secret",
					Prefix:         "sonde_studio_abc",
					AllowedOrigins: []string{},
					DailyQuota:     0,
					UsedToday:      12,
					CreatedAt:      createdAt,
				},
			},
		})
	}))
	defer server.Close()

	c := New(server.URL, "test-token")
	keys, err := c.ListKeys(context.Background(), "")
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
	if keys[0].App != "studio" || keys[0].UsedToday != 12 {
		t.Fatalf("unexpected key content: %+v", keys[0])
	}

	filteredKeys, err := c.ListKeys(context.Background(), "otherapp")
	if err != nil {
		t.Fatalf("ListKeys filtered: %v", err)
	}
	if len(filteredKeys) != 0 {
		t.Fatalf("expected 0 keys, got %d", len(filteredKeys))
	}
}

func TestCreateKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/apikeys" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var req CreateKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		createdAt, _ := time.Parse(time.RFC3339, "2026-09-01T15:00:00Z")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(CreateKeyResponse{
			Key: Key{
				ID:             2,
				App:            req.App,
				Kind:           req.Kind,
				Prefix:         "sonde_pub_studio_xyz",
				AllowedOrigins: req.AllowedOrigins,
				DailyQuota:     req.DailyQuota,
				CreatedAt:      createdAt,
			},
			Token: "sonde_pub_studio_xyz123456",
		})
	}))
	defer server.Close()

	c := New(server.URL, "test-token")
	resp, err := c.CreateKey(context.Background(), CreateKeyRequest{
		App:            "studio",
		Kind:           "public",
		AllowedOrigins: []string{"https://studio.facile.test"},
		DailyQuota:     500,
	})
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	if resp.Token != "sonde_pub_studio_xyz123456" {
		t.Fatalf("expected token, got %s", resp.Token)
	}
	if resp.Key.App != "studio" || resp.Key.DailyQuota != 500 {
		t.Fatalf("unexpected key payload: %+v", resp.Key)
	}
}

func TestRevokeKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/apikeys/42" || r.Method != http.MethodDelete {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := New(server.URL, "test-token")
	err := c.RevokeKey(context.Background(), 42)
	if err != nil {
		t.Fatalf("RevokeKey: %v", err)
	}
}
