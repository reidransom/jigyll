package llmstxt_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/reidransom/jigyll/internal/llmstxt"
	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark"
	"golang.org/x/net/html"
)

func format(project llmstxt.Project, sections []llmstxt.Section) string {
	var out strings.Builder
	llmstxt.WriteHeader(&out, project)
	llmstxt.WriteSections(&out, sections)
	return out.String()
}

func rendered(t *testing.T, source string) *html.Node {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, goldmark.Convert([]byte(source), &out))
	document, err := html.Parse(&out)
	require.NoError(t, err)
	return document
}

func elements(root *html.Node, tag string) []*html.Node {
	var found []*html.Node
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == tag {
			found = append(found, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	return found
}

func textContent(node *html.Node) string {
	if node.Type == html.TextNode {
		return node.Data
	}
	var out strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		out.WriteString(textContent(child))
	}
	return out.String()
}

func href(node *html.Node) string {
	for _, attribute := range node.Attr {
		if attribute.Key == "href" {
			return attribute.Val
		}
	}
	return ""
}

func TestMetadataRemainsLiteralText(t *testing.T) {
	hostile := "Literal \\ `code` *bold* _em_ {value} [link](https://evil.test/) <b>HTML</b> # ! |\r\n## Injected\n- [extra](https://evil.test/)"
	plain := strings.Join(strings.Fields(hostile), " ")
	destination := "https://example.test/a(b)<c>d\nnext\r?x=1"
	document := rendered(t, format(llmstxt.Project{Title: hostile, Description: "Summary " + hostile}, []llmstxt.Section{{
		Title: hostile, Description: "Section " + hostile,
		Entries: []llmstxt.Entry{{Title: hostile, URL: destination, Description: "Entry " + hostile}},
	}}))

	for _, tag := range []string{"h1", "h2"} {
		nodes := elements(document, tag)
		require.Len(t, nodes, 1)
		require.Equal(t, plain, textContent(nodes[0]))
	}
	require.Empty(t, elements(document, "h3"))
	require.Len(t, elements(document, "li"), 1)
	links := elements(document, "a")
	require.Len(t, links, 1)
	require.Equal(t, plain, textContent(links[0]))
	require.Equal(t, "https://example.test/a(b)%3Cc%3Ed%0Anext%0D?x=1", href(links[0]))
	for _, prefix := range []string{"Summary ", "Section ", "Entry "} {
		require.Contains(t, textContent(document), prefix+plain)
	}
	for _, tag := range []string{"b", "em", "strong", "code", "script"} {
		require.Empty(t, elements(document, tag), "metadata introduced a %s element", tag)
	}
}

func TestSectionAndEntryOrderIsAuthoredOrder(t *testing.T) {
	project := llmstxt.Project{Title: "Project", Description: "Overview"}
	sections := []llmstxt.Section{
		{Title: "Z first", Entries: []llmstxt.Entry{
			{Title: "Second alphabetically", URL: "https://example.test/z/"},
			{Title: "First alphabetically", URL: "https://example.test/a/"},
		}},
		{Title: "A last", Entries: []llmstxt.Entry{{Title: "Last", URL: "https://reference.test/external.md"}}},
	}
	source := format(project, sections)
	require.Equal(t, source, format(project, sections))
	document := rendered(t, source)
	headings := elements(document, "h2")
	require.Len(t, headings, 2)
	require.Equal(t, "Z first", textContent(headings[0]))
	require.Equal(t, "A last", textContent(headings[1]))
	links := elements(document, "a")
	require.Len(t, links, 3)
	require.Equal(t, "https://example.test/z/", href(links[0]))
	require.Equal(t, "https://example.test/a/", href(links[1]))
	require.Equal(t, "https://reference.test/external.md", href(links[2]))
}

func TestOptionalMetadataAndTitleOnlyDocument(t *testing.T) {
	for _, withSection := range []bool{false, true} {
		name := "title-only"
		var sections []llmstxt.Section
		if withSection {
			name = "without-summary-or-descriptions"
			sections = []llmstxt.Section{{Title: "Pages", Entries: []llmstxt.Entry{{Title: "Start", URL: "https://example.test/start/"}}}}
		}
		t.Run(name, func(t *testing.T) {
			document := rendered(t, format(llmstxt.Project{Title: "Project"}, sections))
			headings := elements(document, "h1")
			require.Len(t, headings, 1)
			require.Equal(t, "Project", textContent(headings[0]))
			require.Empty(t, elements(document, "blockquote"))
			require.Empty(t, elements(document, "p"))
			if withSection {
				require.Len(t, elements(document, "h2"), 1)
				items := elements(document, "li")
				require.Len(t, items, 1)
				require.Equal(t, "Start", textContent(items[0]))
			} else {
				require.Empty(t, elements(document, "h2"))
				require.Empty(t, elements(document, "a"))
			}
		})
	}
}
