package loopback

import "testing"

// TestOpenBrowserOnlyOpensWebURLs pins the guard on what reaches `open` and
// `xdg-open`. Both dispatch on scheme, and one caller passes a string the
// identity provider chose.
func TestOpenBrowserOnlyOpensWebURLs(t *testing.T) {
	cases := map[string]bool{
		"https://sso.example.test/device": true,
		"http://127.0.0.1:9000/device":    true,
		"file:///etc/passwd":              false,
		"javascript:alert(1)":             false,
		"/Applications/Calculator.app":    false,
		"":                                false,
	}

	for candidate, want := range cases {
		if got := openable(candidate); got != want {
			t.Errorf("openable(%q) = %v, want %v", candidate, got, want)
		}
	}
}
