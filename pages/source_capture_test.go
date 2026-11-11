package pages

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/jigyll/renderers"
	"github.com/stretchr/testify/require"
)

func TestSelectedSourceCaptureWithoutDocumentationLayout(t *testing.T) {
	cfg := config.Default()
	cfg.Source = t.TempDir()
	manager, err := renderers.New(cfg, renderers.Options{})
	require.NoError(t, err)
	s := articleSite{siteFake{t, cfg}, manager}
	filename := filepath.Join(cfg.Source, "article.md")
	require.NoError(t, os.WriteFile(filename, []byte("---\nlayout: default\ntitle: Selected\n---\n# {{ page.title }}\n"), 0600))
	makeSource := func() *page {
		document, err := NewFile(s, filename, "article.md", func(bool) FrontMatter { return FrontMatter{} })
		require.NoError(t, err)
		return document.(*page)
	}
	unselected := makeSource()
	body, err := unselected.Article()
	require.NoError(t, err)
	require.Nil(t, body.Source)
	selected := makeSource()
	selected.CaptureSource()
	body, err = selected.Article()
	require.NoError(t, err)
	require.Equal(t, "# Selected\n", string(body.Source))
	require.True(t, selected.SourceBacked())
	clone := selected.Clone("/copy/").(*page)
	require.False(t, clone.SourceBacked())
	body, err = clone.Article()
	require.NoError(t, err)
	require.Nil(t, body.Source)
	selected.SetContent("replacement")
	body, err = selected.Article()
	require.NoError(t, err)
	require.Nil(t, body.Source)
}

func TestRawContentPreservesAuthoredBody(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		name := "liquid disabled"
		if enabled {
			name = "liquid enabled"
		}
		t.Run(name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Source = t.TempDir()
			manager, err := renderers.New(cfg, renderers.Options{})
			require.NoError(t, err)
			s := articleSite{siteFake{t, cfg}, manager}
			filename := filepath.Join(cfg.Source, "article.md")
			body := "# {{ page.title }}\n\n{% raw %}{{ literal }}{% endraw %}\n\n`</script>` — café\n"
			require.NoError(t, os.WriteFile(filename, []byte("---\ntitle: Resolved\nraw_content: overridden\n---\n"+body), 0600))
			document, err := NewFile(s, filename, "article.md", func(bool) FrontMatter {
				return FrontMatter{"render_with_liquid": enabled}
			})
			require.NoError(t, err)
			p := document.(*page)
			assertRaw := func(p *page, expected string) {
				t.Helper()
				output, err := manager.RenderTemplate([]byte("{{ page.raw_content | jsonify }}"), p.TemplateContext(), filename, 1)
				require.NoError(t, err)
				var decoded string
				require.NoError(t, json.Unmarshal(output, &decoded))
				require.Equal(t, expected, decoded)
			}
			assertRaw(p, body)
			article, err := p.Article()
			require.NoError(t, err)
			if enabled {
				require.Contains(t, article.HTML, ">Resolved</h1>")
			} else {
				require.Contains(t, article.HTML, ">{{ page.title }}</h1>")
			}
			assertRaw(p, body)
			assertRaw(p.Clone("/copy/").(*page), body)
			p.SetContent("<p>Generated replacement</p>")
			assertRaw(p, body)

			body = "# Reloaded\n\n{{ page.title }}\n"
			require.NoError(t, os.WriteFile(filename, []byte("---\n---\n"+body), 0600))
			require.NoError(t, p.Reload())
			assertRaw(p, body)
			require.NoError(t, p.Render())
			assertRaw(p, body)

			require.NoError(t, os.WriteFile(filename, []byte("---\n---\n"), 0600))
			require.NoError(t, p.Reload())
			assertRaw(p, "")
		})
	}
}
