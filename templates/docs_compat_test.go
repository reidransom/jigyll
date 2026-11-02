package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/jigyll/renderers"
	"github.com/reidransom/liquid"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

// Only consumer includes are loaded: these tests do not resolve the development
// theme symlink, use the network, or change the real documentation site.
func docsCompatRenderer(t *testing.T) *renderers.Manager {
	t.Helper()
	cfg := config.Default()
	cfg.Source = t.TempDir()
	cfg.BaseURL = "/preview"
	for _, name := range []string{"compat_callout.html", "compat_badge.html", "compat_legend.html", "navigation.html", "jigyll/assets.html"} {
		contents, err := os.ReadFile(filepath.Join("..", "docs", "_includes", name))
		require.NoError(t, err)
		path := filepath.Join(cfg.Source, "_includes", name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, contents, 0644))
	}
	renderer, err := renderers.New(cfg, renderers.Options{})
	require.NoError(t, err)
	return renderer
}

func compatElements(t *testing.T, rendered []byte, matches func(*html.Node) bool) []*html.Node {
	t.Helper()
	root, err := html.Parse(strings.NewReader(string(rendered)))
	require.NoError(t, err)
	var found []*html.Node
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.ElementNode && matches(node) {
			found = append(found, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	return found
}

func compatAttr(node *html.Node, name string) string {
	for _, attr := range node.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func TestDocsCompatibilityNotices(t *testing.T) {
	renderer := docsCompatRenderer(t)
	for _, status := range []string{"modified", "replaced", "unsupported", "jigyll-only"} {
		t.Run(status, func(t *testing.T) {
			bindings := liquid.Bindings{
				"page": map[string]interface{}{"slug": "topic", "url": "/docs/topic/"},
				"site": map[string]interface{}{"data": map[string]interface{}{"compat": map[string]interface{}{
					"topic": map[string]interface{}{"status": status, "summary": "A **semantic summary** with `code`.", "details": "/docs/differences/#" + status},
				}}},
			}
			rendered, err := renderer.RenderTemplate([]byte(`{% include compat_callout.html %}`), bindings, "notice.html", 1)
			require.NoError(t, err)
			notices := compatElements(t, rendered, func(n *html.Node) bool { return n.Data == "aside" })
			require.Len(t, notices, 1)
			require.Equal(t, status, compatAttr(notices[0], "data-compat-status"))
			label := compatAttr(notices[0], "aria-labelledby")
			require.NotEmpty(t, label)
			require.Len(t, compatElements(t, rendered, func(n *html.Node) bool { return compatAttr(n, "id") == label }), 1)
			require.Contains(t, string(rendered), "<strong>semantic summary</strong>")
			require.Contains(t, string(rendered), "<code>code</code>")
			links := compatElements(t, rendered, func(n *html.Node) bool { return n.Data == "a" })
			require.Len(t, links, 1)
			require.Equal(t, "/preview/docs/differences/#"+status, compatAttr(links[0], "href"))
		})
	}
}

func TestDocsCompatibilitySelectionAndDetails(t *testing.T) {
	renderer := docsCompatRenderer(t)
	for _, tc := range []struct {
		name       string
		key        string
		slug       string
		details    string
		wantStatus string
		wantLinks  int
	}{
		{name: "explicit key wins", key: "override", slug: "conflict", details: "/docs/other/", wantStatus: "replaced", wantLinks: 1},
		{name: "slug fallback", slug: "conflict", wantStatus: "modified"},
		{name: "missing", slug: "absent"},
		{name: "same", slug: "matching"},
		{name: "missing explicit key does not fall back", key: "absent", slug: "conflict"},
		{name: "same explicit key wins", key: "matching", slug: "conflict"},
		{name: "self details suppressed", key: "override", slug: "conflict", details: "/docs/topic/", wantStatus: "replaced"},
		{name: "different details retained", key: "override", slug: "conflict", details: "/docs/topic/#section", wantStatus: "replaced", wantLinks: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page := map[string]interface{}{"slug": tc.slug, "url": "/docs/topic/", "compat_key": tc.key}
			override := map[string]interface{}{"status": "replaced", "summary": "Chosen summary."}
			if tc.details != "" {
				override["details"] = tc.details
			}
			bindings := liquid.Bindings{
				"page": page,
				"site": map[string]interface{}{"data": map[string]interface{}{"compat": map[string]interface{}{
					"override": override,
					"conflict": map[string]interface{}{"status": "modified", "summary": "Slug summary."},
					"matching": map[string]interface{}{"status": "same", "summary": "Must not appear."},
				}}},
			}
			rendered, err := renderer.RenderTemplate([]byte(`{% include compat_callout.html %}`), bindings, "notice.html", 1)
			require.NoError(t, err)
			if tc.wantStatus == "" {
				require.Empty(t, strings.TrimSpace(string(rendered)))
				return
			}
			notices := compatElements(t, rendered, func(n *html.Node) bool { return n.Data == "aside" })
			require.Len(t, notices, 1)
			require.Equal(t, tc.wantStatus, compatAttr(notices[0], "data-compat-status"))
			links := compatElements(t, rendered, func(n *html.Node) bool { return n.Data == "a" })
			require.Len(t, links, tc.wantLinks)
			if tc.wantLinks > 0 {
				require.Equal(t, "/preview"+tc.details, compatAttr(links[0], "href"))
			}
		})
	}
}

func TestDocsCompatibilityNavigationAndLegend(t *testing.T) {
	renderer := docsCompatRenderer(t)
	var docs, entries []interface{}
	compat := map[string]interface{}{}
	for _, status := range []string{"modified", "replaced", "unsupported", "jigyll-only", "same", "missing"} {
		route := "/docs/" + status + "/"
		docs = append(docs, map[string]interface{}{"url": route, "slug": "conflict", "compat_key": status})
		entries = append(entries, map[string]interface{}{"label": status, "link": route})
		if status != "missing" {
			compat[status] = map[string]interface{}{"status": status}
		}
	}
	compat["conflict"] = map[string]interface{}{"status": "unsupported"}
	bindings := liquid.Bindings{
		"page": map[string]interface{}{"url": "/docs/modified/"},
		"site": map[string]interface{}{"docs": docs, "navigation": []interface{}{map[string]interface{}{"label": "Topics", "items": entries}}, "data": map[string]interface{}{"compat": compat}},
	}
	rendered, err := renderer.RenderTemplate([]byte(`{% include navigation.html items=site.navigation %}`), bindings, "navigation.html", 1)
	require.NoError(t, err)
	for _, status := range []string{"modified", "replaced", "unsupported", "jigyll-only", "same", "missing"} {
		links := compatElements(t, rendered, func(n *html.Node) bool { return n.Data == "a" && compatAttr(n, "href") == "/preview/docs/"+status+"/" })
		require.Len(t, links, 1)
		var badge *html.Node
		for n := links[0].FirstChild; n != nil; n = n.NextSibling {
			if compatAttr(n, "data-compat-status") != "" {
				badge = n
			}
		}
		if status == "same" || status == "missing" {
			require.Nil(t, badge)
		} else {
			require.NotNil(t, badge)
			require.Equal(t, status, compatAttr(badge, "data-compat-status"))
		}
		if status == "modified" {
			require.Equal(t, "page", compatAttr(links[0], "aria-current"))
		}
	}
	require.Len(t, compatElements(t, rendered, func(n *html.Node) bool {
		return n.Data == "aside" && compatAttr(n, "aria-label") == "Compatibility indicators"
	}), 1)
	require.Len(t, compatElements(t, rendered, func(n *html.Node) bool { return n.Data == "a" && compatAttr(n, "href") == "/preview/docs/differences/" }), 1)
	require.Len(t, compatElements(t, rendered, func(n *html.Node) bool {
		return n.Data == "link" && compatAttr(n, "href") == "/preview/assets/jigyll.css"
	}), 1)
}
