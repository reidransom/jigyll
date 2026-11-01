package server

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/jigyll/site"
	"github.com/stretchr/testify/require"
)

func localizedLLMsConfig(extra string) string {
	return `title: Localized Project
url: https://canonical.test
baseurl: /project
watch: true
plugins: [jigyll-llms-txt]
localization:
  default_language: en
  required_translations: [de]
  locales:
    en: {tag: en, label: English, weight: 0}
    de: {tag: de, label: Deutsch, weight: 1, fallbacks: [en]}
` + extra
}

func localizedLLMsCuration(entries string) string {
	return "llms_txt:\n  sections:\n    - title: Guides\n      entries:\n" + entries
}

func localizedLLMsArticle(language, title, route, body string) string {
	return "---\nlang: " + language + "\ntranslation_key: getting-started\ntitle: " + title + "\nlayout: default\npermalink: " + route + "\n---\n" + body + "\n\n{% include export.md %}\n"
}

func newLocalizedLLMsServer(t *testing.T) (string, *Server, *httptest.Server) {
	t.Helper()
	source := t.TempDir()
	writeLocalizedServerFile(t, source, "_config.yml", localizedLLMsConfig(localizedLLMsCuration("        - url: /getting-started/\n        - url: /erste-schritte/\n")))
	writeLocalizedServerFile(t, source, "_layouts/default.html", "<html><body>LAYOUT CHROME{{ content }}</body></html>")
	writeLocalizedServerFile(t, source, "_includes/export.md", "INCLUDE ONE {{ site.language.key }} {{ \"export.note\" | translate }}\n")
	writeLocalizedServerFile(t, source, "_data/locales/en/messages.yml", "export:\n  note: ENGLISH MESSAGE ONE\n")
	writeLocalizedServerFile(t, source, "_data/locales/de/messages.yml", "export:\n  note: DEUTSCHE NACHRICHT EINS\n")
	writeLocalizedServerFile(t, source, "guide.en.md", localizedLLMsArticle("en", "Getting Started", "/getting-started/", "ENGLISH BODY ONE"))
	writeLocalizedServerFile(t, source, "guide.de.md", localizedLLMsArticle("de", "Erste Schritte", "/erste-schritte/", "DEUTSCHER TEXT EINS"))
	base, err := site.FromDirectory(source, config.Flags{Watch: true})
	require.NoError(t, err)
	server := &Server{Site: base}
	require.NoError(t, server.ensureLocalizedProject())
	// Apply the preview override only after loading canonical configuration,
	// as production does. Rebuilds must not reinterpret it as editorial input.
	server.project.SetAbsoluteURL("http://preview.test:4321")
	server.startLiveReloader()
	t.Cleanup(server.stopLiveReloader)
	httpServer := httptest.NewServer(server.routes())
	httpServer.Client().Timeout = 5 * time.Second
	t.Cleanup(httpServer.Close)
	return source, server, httpServer
}

func localizedLLMsHTTP(t *testing.T, server *httptest.Server, route string, status int) string {
	t.Helper()
	response, err := server.Client().Get(server.URL + route)
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, status, response.StatusCode, "%s: %s", route, body)
	return string(body)
}

func localizedLLMsAcceptedBytes(t *testing.T, server *httptest.Server) map[string]string {
	t.Helper()
	artifacts := make(map[string]string, 3)
	for _, route := range []string{"/project/llms.txt", "/project/getting-started/index.md", "/project/de/erste-schritte/index.md"} {
		artifacts[route] = localizedLLMsHTTP(t, server, route, http.StatusOK)
	}
	return artifacts
}

func requireLocalizedLLMsBytes(t *testing.T, server *httptest.Server, expected map[string]string) {
	t.Helper()
	for route, body := range expected {
		require.Equal(t, body, localizedLLMsHTTP(t, server, route, http.StatusOK), "immutable accepted artifact %s", route)
	}
}

func TestLLMsLocalizedHTTPRefreshesActualEditionsAndLiteralCuration(t *testing.T) {
	t.Setenv("JEKYLL_URL", "")
	source, server, httpServer := newLocalizedLLMsServer(t)
	initial := localizedLLMsAcceptedBytes(t, httpServer)
	index := initial["/project/llms.txt"]
	require.Equal(t, 1, strings.Count(index, "# Localized Project\n"))
	require.Contains(t, index, "## English — Guides")
	require.Contains(t, index, "## Deutsch — Guides")
	require.Less(t, strings.Index(index, "## English — Guides"), strings.Index(index, "## Deutsch — Guides"))
	require.Contains(t, index, "[Getting Started](<https://canonical.test/project/getting-started/index.md>)")
	require.Contains(t, index, "[Erste Schritte](<https://canonical.test/project/de/erste-schritte/index.md>)")
	require.Contains(t, initial["/project/getting-started/index.md"], "ENGLISH BODY ONE")
	require.Contains(t, initial["/project/getting-started/index.md"], "INCLUDE ONE en ENGLISH MESSAGE ONE")
	require.Contains(t, initial["/project/de/erste-schritte/index.md"], "DEUTSCHER TEXT EINS")
	require.Contains(t, initial["/project/de/erste-schritte/index.md"], "INCLUDE ONE de DEUTSCHE NACHRICHT EINS")
	require.NotContains(t, initial["/project/de/erste-schritte/index.md"], "ENGLISH BODY ONE")
	for _, body := range initial {
		require.NotContains(t, body, "preview.test")
		require.NotContains(t, body, "LAYOUT CHROME")
		require.NotContains(t, body, "/project/de/de/")
	}
	localizedLLMsHTTP(t, httpServer, "/project/de/llms.txt", http.StatusNotFound)
	// Preview retains its existing deployment-relative alias, while advertised
	// canonical destinations always include the configured project base path.
	require.Equal(t, index, localizedLLMsHTTP(t, httpServer, "/llms.txt", http.StatusOK))

	// Drive the existing localized watch consumer with explicit event barriers;
	// changing source must not mutate bytes in the serving generation.
	changes := make(chan site.FilesEvent)
	started := make(chan site.FilesEvent, 1)
	releases := make(chan struct{})
	accepted := make(chan site.FilesEvent, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		runLocalizedWatchReloads(changes, func(change site.FilesEvent) bool {
			started <- change
			<-releases
			return server.reload(change)
		}, func(change site.FilesEvent) {
			server.liveReloadBatch(change).Deliver()
			accepted <- change
		})
	}()
	t.Cleanup(func() {
		close(releases)
		close(changes)
		select {
		case <-finished:
		case <-time.After(10 * time.Second):
			t.Error("localized watch consumer did not stop")
		}
	})
	waitEvent := func(events <-chan site.FilesEvent) site.FilesEvent {
		t.Helper()
		select {
		case event := <-events:
			return event
		case <-time.After(10 * time.Second):
			t.Fatal("localized reload did not reach its event barrier")
			return site.FilesEvent{}
		}
	}
	reload := func(paths ...string) {
		t.Helper()
		change := site.FilesEvent{Paths: paths}
		changes <- change
		require.Equal(t, change, waitEvent(started))
		releases <- struct{}{}
		require.Equal(t, change, waitEvent(accepted))
	}

	writeLocalizedServerFile(t, source, "guide.de.md", localizedLLMsArticle("de", "Erste Schritte Zwei", "/erste-schritte/", "DEUTSCHER TEXT ZWEI"))
	writeLocalizedServerFile(t, source, "_includes/export.md", "INCLUDE TWO {{ site.language.key }} {{ \"export.note\" | translate }}\n")
	change := site.FilesEvent{Paths: []string{"guide.de.md", "_includes/export.md"}}
	changes <- change
	require.Equal(t, change, waitEvent(started))
	requireLocalizedLLMsBytes(t, httpServer, initial)
	releases <- struct{}{}
	require.Equal(t, change, waitEvent(accepted))
	refreshed := localizedLLMsAcceptedBytes(t, httpServer)
	require.Contains(t, refreshed["/project/llms.txt"], "[Erste Schritte Zwei](<https://canonical.test/project/de/erste-schritte/index.md>)")
	require.Contains(t, refreshed["/project/de/erste-schritte/index.md"], "DEUTSCHER TEXT ZWEI")
	require.Contains(t, refreshed["/project/de/erste-schritte/index.md"], "INCLUDE TWO de DEUTSCHE NACHRICHT EINS")
	require.Contains(t, refreshed["/project/getting-started/index.md"], "INCLUDE TWO en ENGLISH MESSAGE ONE")

	writeLocalizedServerFile(t, source, "_data/locales/de/messages.yml", "export:\n  note: DEUTSCHE NACHRICHT ZWEI\n")
	reload("_data/locales/de/messages.yml")
	require.Contains(t, localizedLLMsHTTP(t, httpServer, "/project/de/erste-schritte/index.md", http.StatusOK), "INCLUDE TWO de DEUTSCHE NACHRICHT ZWEI")
	require.Equal(t, refreshed["/project/getting-started/index.md"], localizedLLMsHTTP(t, httpServer, "/project/getting-started/index.md", http.StatusOK))

	// One literal slug selects only its actual English edition, not the German
	// translation with the same translation_key. HTML remains independently public.
	writeLocalizedServerFile(t, source, "_config.yml", localizedLLMsConfig(localizedLLMsCuration("        - url: /getting-started/\n")))
	reload("_config.yml")
	index = localizedLLMsHTTP(t, httpServer, "/project/llms.txt", http.StatusOK)
	require.Contains(t, index, "https://canonical.test/project/getting-started/index.md")
	require.NotContains(t, index, "erste-schritte")
	localizedLLMsHTTP(t, httpServer, "/project/de/erste-schritte/index.md", http.StatusNotFound)
	require.Contains(t, localizedLLMsHTTP(t, httpServer, "/project/de/erste-schritte/", http.StatusOK), "DEUTSCHER TEXT ZWEI")

	// A canonical absolute URL into another locale is local, while a genuine
	// external URL remains authored and never acquires a companion route.
	writeLocalizedServerFile(t, source, "_config.yml", localizedLLMsConfig(localizedLLMsCuration("        - url: /getting-started/\n        - url: https://canonical.test/project/de/erste-schritte/\n        - url: https://reference.test/manual.md\n          title: External Manual\n")))
	reload("_config.yml")
	index = localizedLLMsHTTP(t, httpServer, "/project/llms.txt", http.StatusOK)
	require.Contains(t, index, "[Erste Schritte Zwei](<https://canonical.test/project/de/erste-schritte/index.md>)")
	require.Contains(t, index, "[External Manual](<https://reference.test/manual.md>)")
	require.NotContains(t, index, "preview.test")
	require.Contains(t, localizedLLMsHTTP(t, httpServer, "/project/de/erste-schritte/index.md", http.StatusOK), "DEUTSCHE NACHRICHT ZWEI")
}

func TestLLMsLocalizedHTTPRetainsExactArtifactsOnFailureAndRecovers(t *testing.T) {
	t.Setenv("JEKYLL_URL", "")
	for _, failure := range []string{"invalid-reference", "aggregate-collision", "companion-collision", "render", "required-edition"} {
		t.Run(failure, func(t *testing.T) {
			source, server, httpServer := newLocalizedLLMsServer(t)
			old := localizedLLMsAcceptedBytes(t, httpServer)
			connection := connectLiveReload(t, httpServer.URL)
			defer connection.Close()
			waitForLiveReloadConnection(t, server)

			writeLocalizedServerFile(t, source, "guide.en.md", localizedLLMsArticle("en", "Rejected English", "/getting-started/", "REJECTED ENGLISH BODY"))
			writeLocalizedServerFile(t, source, "guide.de.md", localizedLLMsArticle("de", "Abgelehnt", "/erste-schritte/", "ABGELEHNTER DEUTSCHER TEXT"))
			paths := []string{"guide.en.md", "guide.de.md"}
			switch failure {
			case "invalid-reference":
				writeLocalizedServerFile(t, source, "_config.yml", localizedLLMsConfig(localizedLLMsCuration("        - url: /absent-everywhere/\n")))
				paths = append(paths, "_config.yml")
			case "aggregate-collision":
				writeLocalizedServerFile(t, source, "llms.txt", "AUTHORED AGGREGATE COLLISION\n")
				paths = append(paths, "llms.txt")
			case "companion-collision":
				writeLocalizedServerFile(t, source, "de/erste-schritte/index.md", "AUTHORED COMPANION COLLISION\n")
				paths = append(paths, "de/erste-schritte/index.md")
			case "render":
				writeLocalizedServerFile(t, source, "_layouts/default.html", "{% no_such_localized_layout_tag %}")
				paths = append(paths, "_layouts/default.html")
			case "required-edition":
				require.NoError(t, os.Remove(filepath.Join(source, "guide.de.md")))
			}
			require.False(t, server.reload(site.FilesEvent{Paths: paths}))
			require.NoError(t, connection.SetReadDeadline(time.Now().Add(5*time.Second)))
			var alert liveReloadAlert
			require.NoError(t, connection.ReadJSON(&alert))
			require.Equal(t, "alert", alert.Command)
			require.Contains(t, alert.Message, "Error reading site configuration:")
			if failure == "required-edition" {
				require.Contains(t, alert.Message, `missing required locale "de"`)
			}
			requireLocalizedLLMsBytes(t, httpServer, old)
			localizedLLMsHTTP(t, httpServer, "/project/de/neue-schritte/index.md", http.StatusNotFound)

			writeLocalizedServerFile(t, source, "_config.yml", localizedLLMsConfig(localizedLLMsCuration("        - url: /getting-started/\n        - url: /neue-schritte/\n")))
			writeLocalizedServerFile(t, source, "_layouts/default.html", "<html><body>RECOVERED LAYOUT{{ content }}</body></html>")
			writeLocalizedServerFile(t, source, "_includes/export.md", "RECOVERED INCLUDE {{ site.language.key }}\n")
			writeLocalizedServerFile(t, source, "guide.en.md", localizedLLMsArticle("en", "Recovered English", "/getting-started/", "RECOVERED ENGLISH BODY"))
			writeLocalizedServerFile(t, source, "guide.de.md", localizedLLMsArticle("de", "Neue Schritte", "/neue-schritte/", "WIEDERHERGESTELLTER DEUTSCHER TEXT"))
			if failure == "aggregate-collision" {
				require.NoError(t, os.Remove(filepath.Join(source, "llms.txt")))
			}
			if failure == "companion-collision" {
				require.NoError(t, os.Remove(filepath.Join(source, "de", "erste-schritte", "index.md")))
			}
			change := site.FilesEvent{Paths: append(paths, "_config.yml", "_layouts/default.html", "_includes/export.md")}
			require.True(t, server.reload(change))
			server.liveReloadBatch(change).Deliver()
			requireAcceptedLiveReload(t, connection)
			index := localizedLLMsHTTP(t, httpServer, "/project/llms.txt", http.StatusOK)
			require.Contains(t, index, "[Recovered English](<https://canonical.test/project/getting-started/index.md>)")
			require.Contains(t, index, "[Neue Schritte](<https://canonical.test/project/de/neue-schritte/index.md>)")
			require.NotContains(t, index, "erste-schritte")
			require.NotContains(t, index, "preview.test")
			require.NotEqual(t, old["/project/llms.txt"], index)
			english := localizedLLMsHTTP(t, httpServer, "/project/getting-started/index.md", http.StatusOK)
			require.Contains(t, english, "RECOVERED ENGLISH BODY")
			require.Contains(t, english, "RECOVERED INCLUDE en")
			require.NotEqual(t, old["/project/getting-started/index.md"], english)
			german := localizedLLMsHTTP(t, httpServer, "/project/de/neue-schritte/index.md", http.StatusOK)
			require.Contains(t, german, "WIEDERHERGESTELLTER DEUTSCHER TEXT")
			require.Contains(t, german, "RECOVERED INCLUDE de")
			require.NotContains(t, german, "RECOVERED ENGLISH BODY")
			require.NotContains(t, german, "RECOVERED LAYOUT")
			localizedLLMsHTTP(t, httpServer, "/project/de/erste-schritte/index.md", http.StatusNotFound)
			localizedLLMsHTTP(t, httpServer, "/project/de/erste-schritte/", http.StatusNotFound)
			localizedLLMsHTTP(t, httpServer, "/project/de/llms.txt", http.StatusNotFound)
		})
	}
}

func TestLLMsLocalizedRunPreparesCanonicalExportsBeforeServingOverride(t *testing.T) {
	t.Setenv("JEKYLL_URL", "")
	source := t.TempDir()
	writeLocalizedServerFile(t, source, "_config.yml", localizedLLMsConfig(""))
	writeLocalizedServerFile(t, source, "english.md", "---\ntitle: English\nlang: en\ntranslation_key: start\npermalink: /getting-started/\n---\nEnglish startup body\n")
	writeLocalizedServerFile(t, source, "german.md", "---\ntitle: Deutsch\nlang: de\ntranslation_key: start\npermalink: /erste-schritte/\n---\nDeutscher Starttext\n")
	base, err := site.FromDirectory(source, config.Flags{})
	require.NoError(t, err)
	// Occupy the address so Run returns after real startup preparation, without
	// leaving a server goroutine or adding a production lifecycle hook.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	base.Config().Host = "127.0.0.1"
	base.Config().Port = listener.Addr().(*net.TCPAddr).Port
	server := &Server{Site: base}
	require.ErrorContains(t, server.Run(false, func(string, string) {}), "bind:")
	require.NotNil(t, server.project)
	index, found := server.project.ServedDocument("/llms.txt")
	require.True(t, found)
	var body strings.Builder
	require.NoError(t, index.RenderTo(&body))
	require.Contains(t, body.String(), "https://canonical.test/project/getting-started/index.md")
	require.Contains(t, body.String(), "https://canonical.test/project/de/erste-schritte/index.md")
}
