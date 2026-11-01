package pages

import (
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
