package pages

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/jigyll/renderers"
	"github.com/stretchr/testify/require"
)

type articleSite struct {
	siteFake
	manager *renderers.Manager
}

func (s articleSite) RendererManager() renderers.Renderers { return s.manager }

func TestArticleResolvedSourceAndLifecycle(t *testing.T) {
	cfg := config.Default()
	cfg.Source = t.TempDir()
	require.NoError(t, config.Unmarshal([]byte("documentation:\n  layout: docs\n"), &cfg))
	manager, err := renderers.New(cfg, renderers.Options{})
	require.NoError(t, err)
	s := articleSite{siteFake{t, cfg}, manager}
	filename := filepath.Join(cfg.Source, "article.md")
	require.NoError(t, os.WriteFile(filename, []byte("---\nlayout: docs\ntitle: Resolved\n---\n# {{ page.title }}\n\n{% raw %}`{{ literal }}`{% endraw %}\n"), 0600))
	doc, err := NewFile(s, filename, "article.md", func(bool) FrontMatter { return FrontMatter{} })
	require.NoError(t, err)
	p := doc.(Page)
	article, err := p.Article()
	require.NoError(t, err)
	require.Equal(t, "# Resolved\n\n`{{ literal }}`\n", string(article.Source))
	require.Contains(t, article.HTML, "<code>{{ literal }}</code>")
	require.Len(t, article.Headings, 1)

	require.NoError(t, os.WriteFile(filename, []byte("---\nlayout: docs\n---\n# Reloaded\n"), 0600))
	require.NoError(t, p.Reload())
	article, err = p.Article()
	require.NoError(t, err)
	require.Equal(t, "# Reloaded\n", string(article.Source))
	require.NotContains(t, article.HTML, "Resolved")

	p.SetContent("<h2 id=\"replacement\">Replacement</h2>")
	article, err = p.Article()
	require.NoError(t, err)
	require.Nil(t, article.Source)
	require.Contains(t, article.HTML, "Replacement")
	require.Len(t, article.Headings, 1)
}
