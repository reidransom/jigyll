package templates

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/jigyll/renderers"
	"github.com/reidransom/liquid"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

func TestDocsEditPageDestinations(t *testing.T) {
	cfg := config.Default()
	cfg.Source = t.TempDir()
	contents, err := os.ReadFile(filepath.Join("..", "docs", "_includes", "jigyll", "site-links.html"))
	require.NoError(t, err)
	includeDir := filepath.Join(cfg.Source, "_includes", "jigyll")
	require.NoError(t, os.MkdirAll(includeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(includeDir, "site-links.html"), contents, 0644))
	renderer, err := renderers.New(cfg, renderers.Options{})
	require.NoError(t, err)

	for _, tc := range []struct {
		name     string
		editBase string
		path     string
		want     string
	}{
		{
			name: "homepage", editBase: "https://github.com/example/manual/edit/main/docs/", path: "index.md",
			want: "https://github.com/example/manual/edit/main/docs/index.md",
		},
		{
			name: "collection article", editBase: "https://github.com/example/manual/edit/main/docs/", path: "_docs/foo.md",
			want: "https://github.com/example/manual/edit/main/docs/_docs/foo.md",
		},
		{
			name: "configured repository branch and source", editBase: "https://github.com/another/project/edit/release/website/content/", path: "_docs/nested/topic.md",
			want: "https://github.com/another/project/edit/release/website/content/_docs/nested/topic.md",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var rendered bytes.Buffer
			err := renderer.Render(&rendered, []byte(`{% include jigyll/site-links.html %}`), liquid.Bindings{
				"page": map[string]interface{}{"path": tc.path},
				"site": map[string]interface{}{
					"title": "Jigyll",
					"site_links": map[string]interface{}{
						"edit_base_url": tc.editBase,
						"attribution":   `Adapted from <a href="https://example.org/original">Original</a>.`,
					},
				},
			}, tc.path, 1)
			require.NoError(t, err)
			links := compatElements(t, rendered.Bytes(), func(n *html.Node) bool { return n.Data == "a" })
			require.Equal(t, tc.want, compatAttr(links[0], "href"))
		})
	}
}
