package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCreateStatusPageSendsOrderedMonitorIDs(t *testing.T) {
	var got StatusPageInput
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/status-pages" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(StatusPage{ID: 7, Slug: got.Slug, Title: got.Title})
	}))
	defer server.Close()

	created, err := New(server.URL, "t").CreateStatusPage(context.Background(), StatusPageInput{Slug: "status", Title: "Facile", MonitorIDs: []int64{3, 1, 2}})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != 7 || !reflect.DeepEqual(got.MonitorIDs, []int64{3, 1, 2}) {
		t.Fatalf("unexpected: %+v %+v", created, got)
	}
}
