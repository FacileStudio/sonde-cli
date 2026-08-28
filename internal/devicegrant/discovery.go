package devicegrant

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// discoveryPath is the only discovery path worth asking for.
const discoveryPath = "/.well-known/openid-configuration"

// Issuer is the provider to run the grant against.
//
// An override is required to be https, because everything the grant trusts
// comes out of the document it serves: the endpoint the user code is minted at,
// the page the human is told to open, and the token endpoint the session is
// eventually bought from. Over plaintext, anyone on the path chooses all three.
// A loopback address is the exception, since that is a developer running a
// provider on their own machine and there is no path to be on.
func Issuer() (string, error) {
	override := strings.TrimSpace(os.Getenv(IssuerEnv))
	if override == "" {
		return DefaultIssuer, nil
	}
	if err := requireSecureURL(override); err != nil {
		return "", fmt.Errorf("%s is not usable — %w", IssuerEnv, err)
	}
	return override, nil
}

// requireSecureURL refuses anything the grant should not be told to trust or to
// open: a scheme that is not http or https, and plaintext http anywhere but
// loopback.
func requireSecureURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%q is not a URL", raw)
	}
	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		if isLoopback(parsed.Hostname()) {
			return nil
		}
		return fmt.Errorf("%q is plaintext http, which only loopback may be", raw)
	default:
		return fmt.Errorf("%q is not an http or https URL", raw)
	}
}

// isLoopback reports whether a host names this machine.
func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// Provider is the part of a discovery document this package uses.
// OffersDeviceGrant is the provider's own answer to "can you do this at all":
// an advertised endpoint is not an implemented grant, so grant_types_supported
// is what decides.
type Provider struct {
	DeviceAuthorization string
	Token               string
	OffersDeviceGrant   bool
}

// Discover reads the endpoints out of the provider's discovery document rather
// than assembling paths from the issuer, so a provider that moves an endpoint
// moves it for this CLI too and nobody has to ship a release for it.
//
// A provider that answers and declines the grant returns OffersDeviceGrant
// false with no error. Every other outcome — unreachable, unparseable, a 500 —
// is an error, because "could not ask" and "was told no" are different answers
// and only the second one means the fallback flow is the right next step.
func Discover(ctx context.Context, issuer string) (Provider, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimSuffix(issuer, "/")+discoveryPath, nil)
	if err != nil {
		return Provider{}, err
	}
	status, doc, err := send(request)
	if err != nil {
		return Provider{}, err
	}
	if status < 200 || status > 299 || doc == nil {
		return Provider{}, fmt.Errorf("%s served no OpenID configuration (%d)", issuer, status)
	}

	found := Provider{
		DeviceAuthorization: text(doc, "device_authorization_endpoint"),
		Token:               text(doc, "token_endpoint"),
	}
	grants, _ := doc["grant_types_supported"].([]any)
	for _, grant := range grants {
		if named, ok := grant.(string); ok && named == GrantType {
			found.OffersDeviceGrant = found.DeviceAuthorization != "" && found.Token != ""
		}
	}
	return found, nil
}
