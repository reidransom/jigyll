package pages

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/jigyll/renderers"
	"github.com/stretchr/testify/require"
)

func TestPageRenderWithLiquid(t *testing.T) {
	for _, tc := range []struct {
		name     string
		defaults string
		explicit string
		disabled bool
	}{
		{name: "absent"},
		{name: "explicit true", explicit: "render_with_liquid: true\n"},
		{name: "defaults false", defaults: "false", disabled: true},
		{name: "document false", explicit: "render_with_liquid: false\n", disabled: true},
		{name: "document true overrides false", defaults: "false", explicit: "render_with_liquid: true\n"},
		{name: "document false overrides true", defaults: "true", explicit: "render_with_liquid: false\n", disabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Source = t.TempDir()
			if tc.defaults != "" {
				require.NoError(t, config.Unmarshal([]byte("defaults:\n  - scope: {path: skills}\n    values: {render_with_liquid: "+tc.defaults+"}\n"), &cfg))
			}
			require.NoError(t, os.MkdirAll(filepath.Join(cfg.Source, "_layouts"), 0700))
			require.NoError(t, os.WriteFile(filepath.Join(cfg.Source, "_layouts", "default.html"), []byte("<header>{{ page.title }}</header>{{ content }}"), 0600))
			manager, err := renderers.New(cfg, renderers.Options{})
			require.NoError(t, err)
			s := articleSite{siteFake{t, cfg}, manager}
			body := "# Heading\n\n**{{ page.title }}**\n"
			expected := "**Resolved**"
			if tc.disabled {
				body += "\n`{{justfile()}}`\n"
				expected = "**{{ page.title }}**"
			}
			filename := filepath.Join(cfg.Source, "article.md")
			require.NoError(t, os.WriteFile(filename, []byte("---\nlayout: default\ntitle: Resolved\n"+tc.explicit+"---\n"+body), 0600))
			doc, err := NewFile(s, filename, "skills/article.md", func(bool) FrontMatter {
				return cfg.GetFrontMatterDefaults("pages", "skills/article.md")
			})
			require.NoError(t, err)
			p := doc.(*page)
			p.CaptureSource()
			var output bytes.Buffer
			require.NoError(t, p.Write(&output))
			require.Contains(t, output.String(), "<header>Resolved</header>")
			if tc.disabled {
				require.Contains(t, output.String(), "<strong>{{ page.title }}</strong>")
				require.Contains(t, output.String(), "<code>{{justfile()}}</code>")
			} else {
				require.Contains(t, output.String(), "<strong>Resolved</strong>")
			}
			article, err := p.Article()
			require.NoError(t, err)
			require.Equal(t, bytes.ReplaceAll([]byte(body), []byte("**{{ page.title }}**"), []byte(expected)), article.Source)
			require.Len(t, article.Headings, 1)
			require.Contains(t, p.Excerpt(), "Heading")
		})
	}
}
