package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/jigyll/site"
	"github.com/stretchr/testify/require"
)

func TestInitialLLMsPreviewServesPreparedDeploymentArtifacts(t *testing.T) {
	source := t.TempDir()
	writeLocalizedServerFile(t, source, "_config.yml", "title: Canonical Project\nurl: https://canonical.test\nbaseurl: /project\nplugins: [jigyll-llms-txt]\n")
	writeLocalizedServerFile(t, source, "_layouts/default.html", "<html>CHROME{{ content }}</html>")
	writeLocalizedServerFile(t, source, "guide.md", "---\ntitle: Guide\nlayout: default\npermalink: /guide/\n---\n# {{ site.title }}\n\n[Link](../other/)\n")
	prepared, err := site.FromDirectory(source, config.Flags{})
	require.NoError(t, err)
	require.NoError(t, prepared.Read())
	server := &Server{Site: prepared}
	prepared.SetAbsoluteURL("http://127.0.0.1:1234")
	for route, want := range map[string]string{"/project/llms.txt": "https://canonical.test/project/guide/index.md", "/project/guide/index.md": "https://canonical.test/project/other/", "/project/guide/": "CHROME"} {
		response := httptest.NewRecorder()
		server.handler(response, httptest.NewRequest(http.MethodGet, route, nil))
		require.Equal(t, http.StatusOK, response.Code)
		require.Contains(t, response.Body.String(), want)
		require.NotContains(t, response.Body.String(), "127.0.0.1")
	}
	response := httptest.NewRecorder()
	server.handler(response, httptest.NewRequest(http.MethodGet, "/projectile/llms.txt", nil))
	require.Equal(t, http.StatusNotFound, response.Code)
}
