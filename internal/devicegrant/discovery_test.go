package devicegrant

import "testing"

// TestIssuerRefusesAnInsecureOverride pins that only a loopback provider may be
// plaintext. Everything the grant trusts comes out of the discovery document,
// so a hostile issuer chooses the page the human is told to open and the
// endpoint the session is bought from.
func TestIssuerRefusesAnInsecureOverride(t *testing.T) {
	cases := map[string]struct {
		override string
		want     string
		wantErr  bool
	}{
		"unset":            {"", DefaultIssuer, false},
		"https":            {"https://sso.example.test", "https://sso.example.test", false},
		"plaintext":        {"http://sso.example.test", "", true},
		"loopback":         {"http://127.0.0.1:9000", "http://127.0.0.1:9000", false},
		"localhost":        {"http://localhost:9000", "http://localhost:9000", false},
		"another scheme":   {"file:///etc/passwd", "", true},
		"not a URL at all": {"://nope", "", true},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv(IssuerEnv, testCase.override)

			got, err := Issuer()
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("Issuer() = %q, want a refusal", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Issuer: %v", err)
			}
			if got != testCase.want {
				t.Errorf("Issuer() = %q, want %q", got, testCase.want)
			}
		})
	}
}
