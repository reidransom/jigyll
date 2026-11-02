package templates

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/jigyll/renderers"
	"github.com/reidransom/liquid"
	"github.com/stretchr/testify/require"
)

type docsSearchDocument struct {
	Title    string              `json:"title"`
	URL      string              `json:"url"`
	Body     string              `json:"body"`
	Headings []renderers.Heading `json:"headings"`
}

func docsSearchRenderer(t *testing.T) *renderers.Manager {
	t.Helper()
	cfg := config.Default()
	cfg.Source = t.TempDir()
	cfg.BaseURL = "/preview"
	// The extractor is a fixed, real Docifly dependency, not an echo mock.
	// Snapshot SHA-256: c4f3a03c6bc238180572cc17746eab312cc451b7dac12895265c6037a59bf608.
	// Only that minimal dependency is vendored; neither the corpus under test nor
	// any layout is copied from the mutable sibling theme at test time.
	for name, source := range map[string]string{
		"search-corpus.html":      filepath.Join("..", "docs", "_includes", "private", "search-corpus.html"),
		"searchable-content.html": filepath.Join("testdata", "docifly", "_includes", "private", "searchable-content.html"),
	} {
		contents, err := os.ReadFile(source)
		require.NoError(t, err)
		path := filepath.Join(cfg.Source, "_includes", "private", name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, contents, 0644))
	}
	renderer, err := renderers.New(cfg, renderers.Options{})
	require.NoError(t, err)
	return renderer
}

func renderDocsSearch(t *testing.T, renderer *renderers.Manager, pages, collections []interface{}) []docsSearchDocument {
	t.Helper()
	rendered, err := renderer.RenderTemplate([]byte(`{% include private/search-corpus.html %}`), liquid.Bindings{
		"site": map[string]interface{}{"html_pages": pages, "collections": collections},
	}, "search-data.json", 1)
	require.NoError(t, err)
	var corpus struct {
		Documents []docsSearchDocument `json:"documents"`
	}
	require.NoError(t, json.Unmarshal(rendered, &corpus), "corpus must be valid JSON: %s", rendered)
	return corpus.Documents
}

func TestDocsSearchCorpusContent(t *testing.T) {
	renderer := docsSearchRenderer(t)
	headings := []renderers.Heading{{Level: 2, ID: "quoted-section", Text: "A \"quoted\" heading\ncontinued"}}
	article := map[string]interface{}{
		"layout": "docs", "url": "/docs/quoted/", "title": "A \"quoted\" title\ncontinued",
		"content": `<h2 id="quoted-section">Authored section</h2>
<p>Distinctive "quoted" text
across lines.</p>
<!-- docifly-search-exclude:start --><p>Private compatibility chrome</p><!-- docifly-search-exclude:end -->
<p>Visible tail.</p>
<!-- docifly-search-exclude:start -->Second hidden block<!-- docifly-search-exclude:end -->
<p>Final text.</p>`,
		"headings": headings,
	}
	pages := []interface{}{
		map[string]interface{}{"layout": "default", "url": "/", "title": "Home", "content": "<p>Original home content.</p>", "headings": []renderers.Heading{}},
		map[string]interface{}{"layout": "docs", "url": "/standalone/", "title": "Standalone", "content": "<p>Regular docs page.</p>"},
	}
	collections := []interface{}{map[string]interface{}{"output": true, "docs": []interface{}{
		article,
		map[string]interface{}{"layout": "default", "url": "/news/original/", "content": "<p>Original collection content.</p>"},
	}}}
	documents := renderDocsSearch(t, renderer, pages, collections)
	require.Equal(t, []docsSearchDocument{
		{Title: "Home", URL: "/preview/", Body: "Original home content.", Headings: []renderers.Heading{}},
		{Title: "Standalone", URL: "/preview/standalone/", Body: "Regular docs page."},
		{Title: "A \"quoted\" title\ncontinued", URL: "/preview/docs/quoted/", Body: "Authored section Distinctive \"quoted\" text across lines. Visible tail. Final text.", Headings: headings},
		{URL: "/preview/news/original/", Body: "Original collection content."},
	}, documents)
}

func TestDocsSearchCorpusEligibility(t *testing.T) {
	renderer := docsSearchRenderer(t)
	for _, tc := range []struct {
		name       string
		layout     string
		excluded   bool
		wantPublic bool
	}{
		{name: "default", layout: "default", wantPublic: true},
		{name: "docs", layout: "docs", wantPublic: true},
		{name: "excluded default", layout: "default", excluded: true},
		{name: "excluded docs", layout: "docs", excluded: true},
		{name: "splash", layout: "splash"},
		{name: "unknown layout", layout: "other"},
		{name: "no layout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page := map[string]interface{}{
				"layout": tc.layout, "search_exclude": tc.excluded, "url": "/candidate/", "content": "Candidate authored content.",
			}
			for _, location := range []struct {
				name        string
				pages       []interface{}
				collections []interface{}
				want        bool
			}{
				{name: "html page", pages: []interface{}{page}, want: tc.wantPublic},
				{name: "output collection", collections: []interface{}{map[string]interface{}{"output": true, "docs": []interface{}{page}}}, want: tc.wantPublic},
				{name: "non-output collection", collections: []interface{}{map[string]interface{}{"output": false, "docs": []interface{}{page}}}},
				{name: "unspecified output", collections: []interface{}{map[string]interface{}{"docs": []interface{}{page}}}},
			} {
				t.Run(location.name, func(t *testing.T) {
					documents := renderDocsSearch(t, renderer, location.pages, location.collections)
					if location.want {
						require.Equal(t, []docsSearchDocument{{URL: "/preview/candidate/", Body: "Candidate authored content."}}, documents)
					} else {
						require.Empty(t, documents)
					}
				})
			}
		})
	}
}

func TestDocsSearchCorpusExtractionAndURLs(t *testing.T) {
	renderer := docsSearchRenderer(t)
	documents := renderDocsSearch(t, renderer, []interface{}{
		map[string]interface{}{"layout": "default", "url": "/nested path/", "content": "<p>Before.</p><!-- docifly-search-exclude:start -->Unclosed marker remains searchable."},
		map[string]interface{}{"layout": "docs", "url": "https://example.org/article/", "content": "External destination."},
	}, nil)
	require.Equal(t, []docsSearchDocument{
		{URL: "/preview/nested%20path/", Body: "Before.Unclosed marker remains searchable."},
		{URL: "https://example.org/article/", Body: "External destination."},
	}, documents)
}
