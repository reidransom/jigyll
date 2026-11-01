package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/jigyll/site"
	"github.com/stretchr/testify/require"
)

func llmsReloadConfig(extra string) string {
	return "title: Canonical Project\nurl: https://canonical.test\nbaseurl: /project\nplugins: [jigyll-llms-txt]\n" + extra
}

func llmsReloadArticle(title, route, extra, body string) string {
	return "---\ntitle: " + title + "\nlayout: default\npermalink: " + route + "\n" + extra + "---\n" + body + "\n\n{% include body.md %}\n\n[Related](../related/)\n"
}

func newLLMsReloadServer(t *testing.T, incremental bool) (string, *Server) {
	t.Helper()
	source := t.TempDir()
	writeLocalizedServerFile(t, source, "_config.yml", llmsReloadConfig(""))
	writeLocalizedServerFile(t, source, "_layouts/default.html", "<html><body>CHROME{{ content }}</body></html>")
	writeLocalizedServerFile(t, source, "_includes/body.md", "INCLUDE ONE {{ site.title }} {{ site.data.export.message }}\n")
	writeLocalizedServerFile(t, source, "_data/export.yml", "message: DATA ONE\n")
	writeLocalizedServerFile(t, source, "guide.md", llmsReloadArticle("Guide One", "/guide/", "", "CONTENT ONE"))
	writeLocalizedServerFile(t, source, "other.md", "---\ntitle: Other\npermalink: /other/\n---\nOTHER CONTENT\n")
	writeLocalizedServerFile(t, source, "assets/authored.txt", "AUTHORED DATA\n")
	unpublished := true
	prepared, err := site.FromDirectory(source, config.Flags{Incremental: &incremental, Unpublished: &unpublished})
	require.NoError(t, err)
	require.NoError(t, prepared.Read())
	server := &Server{Site: prepared}
	server.setServingURL()
	prepared.SetAbsoluteURL("http://preview.test:4321")
	return source, server
}

func llmsReloadResponse(t *testing.T, server *Server, route string, status int) string {
	t.Helper()
	response := httptest.NewRecorder()
	server.handler(response, httptest.NewRequest(http.MethodGet, route, nil))
	require.Equal(t, status, response.Code, "response for %s: %s", route, response.Body.String())
	return response.Body.String()
}

func requireLLMsCanonicalArtifacts(t *testing.T, server *Server, title, body string) {
	t.Helper()
	index := llmsReloadResponse(t, server, "/project/llms.txt", http.StatusOK)
	require.Contains(t, index, "["+title+"](<https://canonical.test/project/guide/index.md>)")
	require.NotContains(t, index, "preview.test")
	companion := llmsReloadResponse(t, server, "/project/guide/index.md", http.StatusOK)
	require.Contains(t, companion, body)
	require.Contains(t, companion, "https://canonical.test/project/related/")
	require.NotContains(t, companion, "preview.test")
	require.NotContains(t, companion, "CHROME")
}

func TestLLMsReloadRetainsArtifactBytesOnCandidateFailureAndRecovers(t *testing.T) {
	t.Setenv("JEKYLL_URL", "")
	for _, incremental := range []bool{false, true} {
		for _, failure := range []string{"metadata", "render", "collision"} {
			t.Run(map[bool]string{false: "full", true: "incremental"}[incremental]+"/"+failure, func(t *testing.T) {
				source, server := newLLMsReloadServer(t, incremental)
				requireLLMsCanonicalArtifacts(t, server, "Guide One", "CONTENT ONE")
				oldIndex := llmsReloadResponse(t, server, "/project/llms.txt", http.StatusOK)
				oldCompanion := llmsReloadResponse(t, server, "/project/guide/index.md", http.StatusOK)
				writeLocalizedServerFile(t, source, "guide.md", llmsReloadArticle("Unpublished Candidate", "/guide/", "", "UNACCEPTED CONTENT"))
				paths := []string{"guide.md"}
				switch failure {
				case "metadata":
					writeLocalizedServerFile(t, source, "_config.yml", llmsReloadConfig("llms_txt:\n  title: [invalid, metadata]\n"))
					paths = append(paths, "_config.yml")
				case "render":
					writeLocalizedServerFile(t, source, "_includes/body.md", "{% no_such_export_tag %}")
					paths = append(paths, "_includes/body.md")
				case "collision":
					writeLocalizedServerFile(t, source, "guide/index.md", "AUTHORED COLLISION\n")
					paths = append(paths, "guide/index.md")
				}
				require.False(t, server.reload(site.FilesEvent{Time: time.Unix(1, 0), Paths: paths}))
				require.Equal(t, oldIndex, llmsReloadResponse(t, server, "/project/llms.txt", http.StatusOK))
				require.Equal(t, oldCompanion, llmsReloadResponse(t, server, "/project/guide/index.md", http.StatusOK))

				writeLocalizedServerFile(t, source, "_config.yml", llmsReloadConfig(""))
				writeLocalizedServerFile(t, source, "_includes/body.md", "RECOVERED INCLUDE\n")
				if failure == "collision" {
					require.NoError(t, os.Remove(filepath.Join(source, "guide", "index.md")))
				}
				writeLocalizedServerFile(t, source, "guide.md", llmsReloadArticle("Recovered Guide", "/guide/", "", "RECOVERED CONTENT"))
				require.True(t, server.reload(site.FilesEvent{Time: time.Unix(2, 0), Paths: append(paths, "_config.yml", "_includes/body.md")}))
				requireLLMsCanonicalArtifacts(t, server, "Recovered Guide", "RECOVERED CONTENT")
				require.Contains(t, llmsReloadResponse(t, server, "/project/guide/index.md", http.StatusOK), "RECOVERED INCLUDE")
				require.NotEqual(t, oldIndex, llmsReloadResponse(t, server, "/project/llms.txt", http.StatusOK))
				require.NotEqual(t, oldCompanion, llmsReloadResponse(t, server, "/project/guide/index.md", http.StatusOK))

				// A second source-only reload starts from a generation whose serving URL
				// was already overridden. It must still recover the configured canonical URL.
				writeLocalizedServerFile(t, source, "_includes/body.md", "SECOND ACCEPTED INCLUDE\n")
				require.True(t, server.reload(site.FilesEvent{Time: time.Unix(3, 0), Paths: []string{"_includes/body.md"}}))
				requireLLMsCanonicalArtifacts(t, server, "Recovered Guide", "SECOND ACCEPTED INCLUDE")
			})
		}
	}
}

func TestLLMsReloadRefreshesInventoryAndCompanionsWithIncrementalFlags(t *testing.T) {
	t.Setenv("JEKYLL_URL", "")
	for _, incremental := range []bool{false, true} {
		t.Run(map[bool]string{false: "full", true: "incremental"}[incremental], func(t *testing.T) {
			source, server := newLLMsReloadServer(t, incremental)
			reload := func(paths ...string) {
				t.Helper()
				require.True(t, server.reload(site.FilesEvent{Paths: paths}))
			}
			requireLLMsCanonicalArtifacts(t, server, "Guide One", "INCLUDE ONE Canonical Project DATA ONE")

			writeLocalizedServerFile(t, source, "guide.md", llmsReloadArticle("Guide Two", "/guide/", "", "CONTENT TWO"))
			reload("guide.md")
			requireLLMsCanonicalArtifacts(t, server, "Guide Two", "CONTENT TWO")
			require.NotContains(t, llmsReloadResponse(t, server, "/project/guide/index.md", http.StatusOK), "CONTENT ONE")

			writeLocalizedServerFile(t, source, "_includes/body.md", "INCLUDE TWO {{ site.title }} {{ site.data.export.message }}\n")
			reload("_includes/body.md")
			requireLLMsCanonicalArtifacts(t, server, "Guide Two", "INCLUDE TWO Canonical Project DATA ONE")
			writeLocalizedServerFile(t, source, "_data/export.yml", "message: DATA TWO\n")
			reload("_data/export.yml")
			requireLLMsCanonicalArtifacts(t, server, "Guide Two", "INCLUDE TWO Canonical Project DATA TWO")

			writeLocalizedServerFile(t, source, "_config.yml", llmsReloadConfig("llms_txt:\n  sections:\n    - title: Selected\n      entries:\n        - url: /other/\n          title: Curated Other\n"))
			reload("_config.yml")
			index := llmsReloadResponse(t, server, "/project/llms.txt", http.StatusOK)
			require.Contains(t, index, "## Selected")
			require.Contains(t, index, "[Curated Other](<https://canonical.test/project/other/index.md>)")
			require.NotContains(t, index, "guide/index.md")
			llmsReloadResponse(t, server, "/project/guide/index.md", http.StatusNotFound)
			require.Contains(t, llmsReloadResponse(t, server, "/project/guide/", http.StatusOK), "CONTENT TWO")
			require.Contains(t, llmsReloadResponse(t, server, "/project/other/index.md", http.StatusOK), "OTHER CONTENT")

			writeLocalizedServerFile(t, source, "_config.yml", llmsReloadConfig(""))
			reload("_config.yml")
			requireLLMsCanonicalArtifacts(t, server, "Guide Two", "CONTENT TWO")
			require.NoError(t, os.Rename(filepath.Join(source, "guide.md"), filepath.Join(source, "renamed.md")))
			writeLocalizedServerFile(t, source, "renamed.md", llmsReloadArticle("Moved Guide", "/moved/", "", "MOVED CONTENT"))
			reload("guide.md", "renamed.md")
			index = llmsReloadResponse(t, server, "/project/llms.txt", http.StatusOK)
			require.Contains(t, index, "[Moved Guide](<https://canonical.test/project/moved/index.md>)")
			require.NotContains(t, index, "/guide/")
			llmsReloadResponse(t, server, "/project/guide/", http.StatusNotFound)
			llmsReloadResponse(t, server, "/project/guide/index.md", http.StatusNotFound)
			require.Contains(t, llmsReloadResponse(t, server, "/project/moved/", http.StatusOK), "MOVED CONTENT")
			require.Contains(t, llmsReloadResponse(t, server, "/project/moved/index.md", http.StatusOK), "MOVED CONTENT")

			// Preview publication flags do not make an unpublished article exportable.
			writeLocalizedServerFile(t, source, "renamed.md", llmsReloadArticle("Private Guide", "/moved/", "published: false\n", "PRIVATE CONTENT"))
			reload("renamed.md")
			index = llmsReloadResponse(t, server, "/project/llms.txt", http.StatusOK)
			require.NotContains(t, index, "Private Guide")
			require.NotContains(t, index, "/moved/")
			llmsReloadResponse(t, server, "/project/moved/index.md", http.StatusNotFound)

			writeLocalizedServerFile(t, source, "added.md", "---\ntitle: Added\npermalink: /added/\n---\nADDED CONTENT\n")
			reload("added.md")
			require.Contains(t, llmsReloadResponse(t, server, "/project/llms.txt", http.StatusOK), "https://canonical.test/project/added/index.md")
			require.Contains(t, llmsReloadResponse(t, server, "/project/added/index.md", http.StatusOK), "ADDED CONTENT")
			require.NoError(t, os.Remove(filepath.Join(source, "other.md")))
			reload("other.md")
			require.NotContains(t, llmsReloadResponse(t, server, "/project/llms.txt", http.StatusOK), "/other/")
			llmsReloadResponse(t, server, "/project/other/", http.StatusNotFound)
			llmsReloadResponse(t, server, "/project/other/index.md", http.StatusNotFound)

			writeLocalizedServerFile(t, source, "_config.yml", "title: Canonical Project\nurl: https://canonical.test\nbaseurl: /project\nllms_txt: ignored without the plugin\n")
			reload("_config.yml")
			llmsReloadResponse(t, server, "/project/llms.txt", http.StatusNotFound)
			llmsReloadResponse(t, server, "/project/added/index.md", http.StatusNotFound)
			llmsReloadResponse(t, server, "/project/moved/index.md", http.StatusNotFound)
			require.Contains(t, llmsReloadResponse(t, server, "/project/added/", http.StatusOK), "ADDED CONTENT")
			require.Equal(t, "AUTHORED DATA\n", llmsReloadResponse(t, server, "/project/assets/authored.txt", http.StatusOK))
		})
	}
}
