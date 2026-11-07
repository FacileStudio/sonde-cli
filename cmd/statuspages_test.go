package cmd

import (
	"reflect"
	"testing"

	"github.com/FacileStudio/sonde-cli/internal/api"
)

func TestMatchMonitorsKeepsOrder(t *testing.T) {
	monitors := []api.Monitor{{ID: 1, Slug: "a"}, {ID: 2, Slug: "b"}, {ID: 3, Slug: "c"}}
	got, err := matchMonitors(monitors, []string{"c", " a ", "2", ""})
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{3, 1, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if _, err := matchMonitors(monitors, []string{"zzz"}); err == nil {
		t.Fatal("expected an error for an unknown monitor")
	}
}
