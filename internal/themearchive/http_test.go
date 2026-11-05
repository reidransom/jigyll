package themearchive

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHTTPClientBoundedRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		case "/ftp":
			http.Redirect(w, r, "ftp://example.com/archive", http.StatusFound)
		case "/file":
			http.Redirect(w, r, "file:///tmp/archive", http.StatusFound)
		case "/credentials":
			http.Redirect(w, r, "http://user:secret@example.com/archive", http.StatusFound)
		case "/start":
			http.Redirect(w, r, "/done", http.StatusTemporaryRedirect)
		case "/done":
			io.WriteString(w, "provider metadata or archive")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewHTTPClient(nil)
	require.Equal(t, 30*time.Second, client.Timeout)
	for _, path := range []string{"/loop", "/ftp", "/file", "/credentials"} {
		t.Run(path, func(t *testing.T) {
			resp, err := client.Get(server.URL + path)
			if resp != nil {
				resp.Body.Close()
			}
			require.Error(t, err)
		})
	}
	resp, err := client.Get(server.URL + "/start")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "provider metadata or archive", string(body))
}

func TestHTTPClientRejectsDowngrade(t *testing.T) {
	insecureRequests := 0
	insecure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		insecureRequests++
		io.WriteString(w, "must not be requested")
	}))
	defer insecure.Close()
	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, insecure.URL, http.StatusFound)
	}))
	defer secure.Close()
	client := NewHTTPClient(secure.Client().Transport)
	resp, err := client.Get(secure.URL)
	if resp != nil {
		resp.Body.Close()
	}
	require.ErrorContains(t, err, "must not downgrade HTTPS")
	require.Equal(t, 0, insecureRequests)
}

func TestHTTPClientAllowsUpgradeAndHostRedirect(t *testing.T) {
	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "archive")
	}))
	defer secure.Close()
	insecure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, secure.URL, http.StatusFound)
	}))
	defer insecure.Close()
	client := NewHTTPClient(secure.Client().Transport)
	resp, err := client.Get(insecure.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, secure.URL, resp.Request.URL.String())
}
