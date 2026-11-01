package site

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/jigyll/docexport"
	"github.com/stretchr/testify/require"
)

func llmsSite(t *testing.T, extra string) string {
	t.Helper()
	root := t.TempDir()
	writeIncludeCacheSiteFile(t, root, "_config.yml", "title: Example\ndescription: Site summary\nurl: https://example.test\nplugins: [jigyll-llms-txt]\n"+extra)
	writeIncludeCacheSiteFile(t, root, "_layouts/default.html", "<html><body>LAYOUT{{ content }}</body></html>")
	return root
}

func TestLLMsStandaloneImmutableSourceExports(t *testing.T) {
	root := llmsSite(t, "baseurl: /project\n")
	writeIncludeCacheSiteFile(t, root, "_includes/body.md", "Included {{ site.title }}\n\n[Fragment](#part) ![Image](../media/logo.svg)\n")
	writeIncludeCacheSiteFile(t, root, "start.md", "---\ntitle: Start\nlayout: default\npermalink: /getting-started/\n---\n# {{ page.title }}\n\n{% include body.md %}\n\n| A | B |\n| - | - |\n| one | two |\n\n{% raw %}`[literal](../unchanged)`\n```markdown\n![literal](../unchanged)\n```{% endraw %}\n")
	writeIncludeCacheSiteFile(t, root, "other.md", "---\ntitle: Other\nllms: false\n---\nUnselected\n")
	s, err := FromDirectory(root, config.Flags{})
	require.NoError(t, err)
	require.NoError(t, s.Read())
	// Prepared before a preview changes the serving URL; no writes or lazy export rendering.
	s.SetAbsoluteURL("http://127.0.0.1:9876")
	index := renderRoute(t, s, "/llms.txt")
	require.Contains(t, index, "https://example.test/project/getting-started/index.md")
	require.NotContains(t, index, "127.0.0.1")
	require.NotContains(t, index, "manifest.json")
	body := renderRoute(t, s, "/getting-started/index.md")
	require.Contains(t, body, "# Start")
	require.Contains(t, body, "Included Example")
	require.Contains(t, body, "https://example.test/project/getting-started/#part")
	require.Contains(t, body, "https://example.test/project/media/logo.svg")
	require.Contains(t, body, "| one | two |")
	require.Contains(t, body, "`[literal](../unchanged)`")
	require.NotContains(t, body, "LAYOUT")
	require.NotContains(t, body, "permalink:")
	require.NotContains(t, index, "Other")
	selected, err := s.Routes["/getting-started/"].(Page).Article()
	require.NoError(t, err)
	require.NotNil(t, selected.Source)
	unselected, err := s.Routes["/other.html"].(Page).Article()
	require.NoError(t, err)
	require.Nil(t, unselected.Source)
	for _, page := range s.Pages() {
		require.NotEqual(t, "/llms.txt", page.URL())
		require.NotEqual(t, "/getting-started/index.md", page.URL())
	}
	writeIncludeCacheSiteFile(t, root, "_includes/body.md", "Changed after preparation")
	require.Equal(t, body, renderRoute(t, s, "/getting-started/index.md"))
	_, err = s.Write()
	require.NoError(t, err)
	require.Equal(t, index, readIncludeCacheSiteOutput(t, root, "llms.txt"))
	require.Equal(t, body, readIncludeCacheSiteOutput(t, root, "getting-started/index.md"))
}

func TestLLMsAutomaticPublicationEligibility(t *testing.T) {
	root := llmsSite(t, "show_drafts: true\nfuture: true\nunpublished: true\ninclude: [.hidden, _plans]\npaginate: 1\ncollections:\n  guides:\n    output: true\n  private:\n    output: false\n")
	// Paginate clones of a source-backed template must never produce exports.
	configBody, err := os.ReadFile(filepath.Join(root, "_config.yml"))
	require.NoError(t, err)
	writeIncludeCacheSiteFile(t, root, "_config.yml", strings.Replace(string(configBody), "plugins: [jigyll-llms-txt]", "plugins: [jigyll-llms-txt, jekyll-paginate]", 1))
	writeIncludeCacheSiteFile(t, root, "index.html", "---\ntitle: Pagination\n---\nPagination {{ paginator.page }}")
	writeIncludeCacheSiteFile(t, root, "home.md", "---\ntitle: Home\npermalink: /home/\n---\nHome")
	for filename, fm := range map[string]string{
		"a.md": "title: Alpha\npermalink: /a/", "_posts/2020-01-01-public.md": "title: Public Post", "_posts/2020-01-02-second.md": "title: Second Post", "_guides/start.md": "title: Collection", "_private/secret.md": "title: Private Collection", "_drafts/draft.md": "title: Draft", "_posts/2999-01-01-future.md": "title: Future", "unpublished.md": "title: Unpublished\npublished: false", "redirect.md": "title: Redirect\nredirect_to: /a/", "robot.md": "title: Robot\nrobots: NOINDEX, follow", "404.md": "title: Missing\npermalink: /404.html", ".hidden/secret.md": "title: Hidden", "_plans/idea.md": "title: Planning", "excluded.md": "title: Excluded\nllms: false", "raw.html": "title: HTML", "data.json": "title: JSON"} {
		writeIncludeCacheSiteFile(t, root, filename, "---\n"+fm+"\n---\nBody")
	}
	writeIncludeCacheSiteFile(t, root, "static.md", "No front matter")
	s := buildIncludeCacheSite(t, root, config.Flags{})
	index := renderRoute(t, s, "/llms.txt")
	for _, title := range []string{"Alpha", "Public Post", "Second Post", "Collection", "Home"} {
		require.Contains(t, index, "["+title+"]")
	}
	for _, title := range []string{"Draft", "Future", "Unpublished", "Redirect", "Robot", "Missing", "Hidden", "Planning", "Excluded", "HTML", "JSON", "Private Collection"} {
		require.NotContains(t, index, "["+title+"]")
	}
	require.NotContains(t, index, "page2")
	require.Contains(t, s.Routes, "/page2/")
	require.Less(t, strings.Index(index, "https://example.test/a/index.md"), strings.Index(index, "https://example.test/home/index.md"))
}

func TestLLMsCompanionRoutesAndHTMLCuration(t *testing.T) {
	for _, markdown := range []bool{true, false} {
		t.Run(map[bool]string{true: "markdown", false: "html"}[markdown], func(t *testing.T) {
			root := llmsSite(t, "baseurl: /project\nllms_txt:\n  markdown: "+map[bool]string{true: "true", false: "false"}[markdown]+"\n  sections:\n    - title: Curated\n      entries:\n        - url: https://example.test/project/file.html\n        - url: /\n        - url: /directory/\n        - url: /extensionless\n        - url: /template/\n        - url: https://outside.test/reference.md\n          title: External\n")
			for name, route := range map[string]string{"root.md": "/", "dir.md": "/directory/", "file.md": "/file.html", "ext.md": "/extensionless", "template.html": "/template/"} {
				writeIncludeCacheSiteFile(t, root, name, "---\ntitle: "+name+"\npermalink: "+route+"\n---\nBody")
			}
			s := buildIncludeCacheSite(t, root, config.Flags{})
			index := renderRoute(t, s, "/llms.txt")
			require.Contains(t, index, "https://example.test/project/template/")
			require.NotContains(t, index, "/template/index.md")
			require.Contains(t, index, "https://outside.test/reference.md")
			expected := []string{"/file.html", "/", "/directory/", "/extensionless"}
			mdRoutes := []string{"/file.html.md", "/index.md", "/directory/index.md", "/extensionless.md"}
			previous := -1
			for i, route := range expected {
				if markdown {
					route = mdRoutes[i]
					_, ok := s.Routes[route]
					require.True(t, ok)
				} else {
					_, ok := s.Routes[mdRoutes[i]]
					require.False(t, ok)
				}
				position := strings.Index(index, "https://example.test/project"+route+">")
				require.Greater(t, position, previous)
				previous = position
			}
		})
	}
}

func TestLLMsCollisionBeforeCleaning(t *testing.T) {
	for _, collision := range []struct{ name, file, front string }{
		{"index", "llms.txt", ""}, {"companion", "guide/index.md", ""},
		{"alias", "alias.html", "---\npermalink: /guide/index.md.html\ntitle: Alias\n---\nAlias"},
		{"ancestor", "guide", "static ancestor"}, {"descendant", "llms.txt/nested.txt", "child"},
	} {
		t.Run(collision.name, func(t *testing.T) {
			root := llmsSite(t, "")
			writeIncludeCacheSiteFile(t, root, "guide.md", "---\ntitle: Guide\npermalink: /guide/\n---\nBody")
			writeIncludeCacheSiteFile(t, root, collision.file, collision.front)
			writeIncludeCacheSiteFile(t, root, "_site/sentinel", "prior generation")
			s, err := FromDirectory(root, config.Flags{})
			require.NoError(t, err)
			err = s.Read()
			if err == nil {
				_, err = s.Write()
			}
			require.Error(t, err)
			require.Equal(t, "prior generation", readIncludeCacheSiteOutput(t, root, "sentinel"))
		})
	}
}

func writeLLMsDocs(t *testing.T, root string) {
	writeIncludeCacheSiteFile(t, root, "_data/docs_sections.yml", "- slug: guides\n  title: Documentation Guides\n")
	writeIncludeCacheSiteFile(t, root, "_layouts/docs.html", "<html>DOC CHROME<article class=\"docs-article\">{{ content }}</article></html>")
	writeIncludeCacheSiteFile(t, root, "article.md", "---\nlayout: docs\ntitle: Article\ndescription: Article summary\nnav_section: guides\nnav_order: 1\npermalink: /docs/start/\n---\n# Article\n\n![Logo](/assets/logo.svg)\n")
	writeIncludeCacheSiteFile(t, root, "general.md", "---\ntitle: General\npermalink: /general/\n---\nGeneral body")
}

func TestLLMsDocumentationOwnershipAndFinalization(t *testing.T) {
	for _, mode := range []string{"docs-only", "plugin-only", "combined-default", "combined-curated", "combined-html"} {
		t.Run(mode, func(t *testing.T) {
			extra := "documentation:\n  sections: docs_sections\n  description: Documentation summary\n  openapi: https://api.example.test/openapi.json\n"
			if mode == "plugin-only" {
				extra = ""
			}
			if mode == "combined-curated" {
				extra += "llms_txt:\n  title: Curated Project\n  description: Plugin summary\n  sections:\n    - title: Chosen\n      entries:\n        - url: /general/\n        - url: /docs/start/\n          title: Chosen Article\n"
			}
			if mode == "combined-html" {
				extra += "llms_txt:\n  markdown: false\n"
			}
			root := llmsSite(t, extra)
			writeLLMsDocs(t, root)
			if mode == "docs-only" {
				data, err := os.ReadFile(filepath.Join(root, "_config.yml"))
				require.NoError(t, err)
				writeIncludeCacheSiteFile(t, root, "_config.yml", strings.Replace(string(data), "plugins: [jigyll-llms-txt]", "llms_txt: {title: Ignored}", 1))
			}
			s := buildIncludeCacheSite(t, root, config.Flags{})
			if mode == "docs-only" {
				_, ok := s.Routes["/llms.txt"]
				require.False(t, ok)
				_, err := os.Stat(filepath.Join(root, "_site/llms.txt"))
				require.True(t, os.IsNotExist(err))
				require.FileExists(t, filepath.Join(root, "_site/docs/start/index.md"))
				return
			}
			index := renderRoute(t, s, "/llms.txt")
			if mode == "plugin-only" {
				require.Contains(t, index, "[General]")
				require.NotContains(t, index, "manifest.json")
				return
			}
			require.Contains(t, index, "https://example.test/docs/manifest.json")
			require.Contains(t, index, "https://api.example.test/openapi.json")
			if mode == "combined-curated" {
				require.Contains(t, index, "# Curated Project")
				require.Contains(t, index, "> Plugin summary")
				require.Less(t, strings.Index(index, "[General]"), strings.Index(index, "[Chosen Article]"))
				require.FileExists(t, filepath.Join(root, "_site/general/index.md"))
			} else {
				require.NotContains(t, index, "[General]")
				require.Contains(t, index, "## Documentation Guides")
				require.Contains(t, index, "> Documentation summary")
			}
			if mode == "combined-html" {
				require.Contains(t, index, "[Article](<https://example.test/docs/start/>)")
			} else {
				require.Contains(t, index, "https://example.test/docs/start/index.md")
			}
			require.FileExists(t, filepath.Join(root, "_site/docs/start/index.md"))
			before := []byte(index)
			manifestBefore := readIncludeCacheSiteOutput(t, root, "docs/manifest.json")
			require.NoError(t, docexport.Finalize(filepath.Join(root, "_site"), map[string]string{"/assets/logo.svg": "https://cdn.example.test/logo.123.svg"}))
			require.Equal(t, before, []byte(readIncludeCacheSiteOutput(t, root, "llms.txt")))
			require.Contains(t, readIncludeCacheSiteOutput(t, root, "docs/start/index.md"), "https://cdn.example.test/logo.123.svg")
			manifestAfter := readIncludeCacheSiteOutput(t, root, "docs/manifest.json")
			require.NotEqual(t, manifestBefore, manifestAfter)
			var manifest docexport.Manifest
			var corpus docexport.Corpus
			require.NoError(t, json.Unmarshal([]byte(manifestAfter), &manifest))
			require.NoError(t, json.Unmarshal([]byte(readIncludeCacheSiteOutput(t, root, "docs/search.json")), &corpus))
			require.Equal(t, manifest.SnapshotID, corpus.SnapshotID)
			require.False(t, bytes.Contains(before, []byte(manifest.SnapshotID)))
			require.Len(t, manifest.Articles, 1)
		})
	}
}

func TestLLMsAbsentPluginPreservesAuthoredIndex(t *testing.T) {
	root := t.TempDir()
	writeIncludeCacheSiteFile(t, root, "_config.yml", "llms_txt: invalid and ignored\n")
	writeIncludeCacheSiteFile(t, root, "llms.txt", "authored discovery\n")
	buildIncludeCacheSite(t, root, config.Flags{})
	require.Equal(t, "authored discovery\n", readIncludeCacheSiteOutput(t, root, "llms.txt"))
}
