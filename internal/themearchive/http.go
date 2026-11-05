package themearchive

import (
	"fmt"
	"net/http"
	"time"
)

// NewHTTPClient supplies the install-time metadata and archive request policy.
// A nil transport uses Go's default transport. Remote themes deliberately keep
// their separate, stricter codeload-only redirect policy.
func NewHTTPClient(transport http.RoundTripper) *http.Client {
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many theme source redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("theme source redirect must use HTTP(S)")
			}
			if req.URL.User != nil {
				return fmt.Errorf("theme source redirect must not contain credentials")
			}
			if len(via) != 0 && via[len(via)-1].URL.Scheme == "https" && req.URL.Scheme != "https" {
				return fmt.Errorf("theme source redirect must not downgrade HTTPS")
			}
			return nil
		},
	}
}
