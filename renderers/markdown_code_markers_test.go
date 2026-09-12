package renderers

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/liquid"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

func TestRenderMarkdownCodeLineMarkers(t *testing.T) {
	source := "keep\n\ninsert one\ninsert two\ndelete\nneutral wins visually\n"
	out := mustMarkdownString("```text {1,2} ins={3-4} del={5} {6} ins={6}\n" + source + "```\n")
	code := parsedCode(t, out)
	lines := elementsByClass(code, "code-line")

	require.Len(t, lines, 6)
	require.Contains(t, classList(lines[0]), "is-marked")
	require.Contains(t, classList(lines[1]), "is-marked")
	require.Equal(t, "\n", nodeText(lines[1]))
	require.Contains(t, classList(lines[2]), "is-inserted")
	require.Contains(t, classList(lines[3]), "is-inserted")
	require.Contains(t, classList(lines[4]), "is-deleted")
	require.Contains(t, classList(lines[5]), "is-inserted")
	require.NotContains(t, classList(lines[5]), "is-marked")
	for _, line := range lines {
		require.NotEmpty(t, attribute(line, "aria-description"))
	}
	require.Equal(t, source, nodeText(code))
	require.NotContains(t, nodeText(code), "+")
	require.NotContains(t, nodeText(code), "-")
	require.NotContains(t, out, "jigyll-code-markers")
}

func TestRenderMarkdownCodeTextMarkers(t *testing.T) {
	source := "added := \"added\"\nreturn value\nremoved := \"removed\"\ncall(value())\n"
	out := mustMarkdownString("```go ins=\"added\" \"return value\" del=\"removed\" ins=\"value()\"\n" + source + "```\n")
	code := parsedCode(t, out)

	require.Equal(t, source, nodeText(code))
	require.Len(t, elementsByClass(code, "code-line"), 4)
	require.Len(t, elementsByClass(code, "is-inserted"), 3)
	require.Len(t, elementsByClass(code, "is-marked"), 1)
	require.Len(t, elementsByClass(code, "is-deleted"), 2)
	for _, marker := range elementsByTag(code, "mark") {
		require.NotEmpty(t, attribute(marker, "aria-description"))
		require.Equal(t, 1, strings.Count(renderNode(marker), "aria-description="))
	}

	neutral := elementsByClass(code, "is-marked")[0]
	require.Equal(t, "return value", nodeText(neutral))
	require.Contains(t, renderNode(neutral), `<span class="k">return</span>`)
	require.Contains(t, renderNode(neutral), `<span class="nx">value</span>`)
	require.Equal(t, "Highlighted text", attribute(neutral, "aria-description"))
	require.NotContains(t, out, "jigyll-code-markers")
}

func TestRenderMarkdownCodeMarkersPreserveUnmarkedOutput(t *testing.T) {
	source := "if ready {\n\tfmt.Println(\"yes\")\n}\n"
	plain := mustMarkdownString("```go\n" + source + "```\n")
	framed := mustMarkdownString("```go title=\"main.go\"\n" + source + "```\n")

	require.Equal(t, renderedPre(plain), renderedPre(framed))
	require.NotContains(t, plain, "code-line")
	require.NotContains(t, framed, "code-line")
}
func TestRenderMarkdownCodeMarkersComposeWithFramesAndAttributes(t *testing.T) {
	framed := mustMarkdownString("```go {linenos=true} title=\"main.go\" ins={1}\nreturn ready\n```\n")
	none := mustMarkdownString("```text frame=\"none\" {1}\nplain\n```\n")

	require.Contains(t, framed, `<figure class="highlight" data-code-frame="editor">`)
	require.Contains(t, framed, `<figcaption class="code-title">main.go</figcaption>`)
	require.Contains(t, framed, `class="line code-line is-inserted"`)
	require.Contains(t, framed, `class="ln">1</span>`)
	require.NotContains(t, framed, "jigyll-code-markers")
	require.NotContains(t, none, "data-code-frame")
	require.Contains(t, none, `class="line code-line is-marked"`)
}

func TestRenderMarkdownCodeMarkerDoesNotSelectTrailingFenceNewline(t *testing.T) {
	_, err := renderMarkdown([]byte("```text {2}\none\n"))
	require.EqualError(t, err, "code fence metadata at line 1: marker line 2 is out of range (code has 1 line)")
}

func TestRenderMarkdownCodeMarkerValidation(t *testing.T) {
	tests := []struct {
		name    string
		info    string
		source  string
		message string
	}{
		{name: "empty neutral lines", info: `{}`, source: "one\n", message: "line marker selector must not be empty"},
		{name: "empty typed lines", info: `ins={}`, source: "one\n", message: "line marker selector must not be empty"},
		{name: "zero line", info: `{0}`, source: "one\n", message: "marker lines must be positive"},
		{name: "reversed range", info: `del={3-2}`, source: "one\ntwo\nthree\n", message: "marker line range 3-2 is reversed"},
		{name: "out of range", info: `ins={4}`, source: "one\ntwo\nthree\n", message: "marker line 4 is out of range (code has 3 lines)"},
		{name: "invalid range", info: `{2-}`, source: "one\ntwo\n", message: `invalid line marker selector "2-"`},
		{name: "empty neutral text", info: `""`, source: "one\n", message: "text marker selector must not be empty"},
		{name: "empty typed text", info: `del=""`, source: "one\n", message: "text marker selector must not be empty"},
		{name: "missing text", info: `"absent"`, source: "one\n", message: `marker text "absent" was not found`},
		{name: "whole line conflict", info: `ins={2} del={2}`, source: "one\ntwo\n", message: "inserted and deleted markers overlap on line 2"},
		{name: "line and text conflict", info: `ins={2} del="two"`, source: "one\ntwo\n", message: "inserted and deleted markers overlap on line 2"},
		{name: "text conflict", info: `ins="two" del="wo"`, source: "two\n", message: "inserted and deleted markers overlap on line 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := renderMarkdown([]byte("intro\n\n```text " + tt.info + "\n" + tt.source + "```\n"))
			require.EqualError(t, err, "code fence metadata at line 3: "+tt.message)
		})
	}
}

func TestRenderReportsCodeMarkerSourcePathAndLine(t *testing.T) {
	assertMarkdownRenderError(
		t,
		[]byte("intro\n\n```text ins={4}\none\ntwo\nthree\n```\n"),
		"guides/markers.md",
		5,
		"guides/markers.md: code fence metadata at line 7: marker line 4 is out of range (code has 3 lines)",
	)
}
func assertMarkdownRenderError(t *testing.T, source []byte, path string, firstLine int, expected string) {
	t.Helper()
	cfg := config.Default()
	cfg.Source = t.TempDir()
	manager, err := New(cfg, Options{})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(manager.sassTempDir))
	})

	var output bytes.Buffer
	err = manager.Render(&output, source, liquid.Bindings{}, path, firstLine)
	require.EqualError(t, err, expected)
}

func parsedCode(t *testing.T, rendered string) *html.Node {
	t.Helper()
	document, err := html.Parse(strings.NewReader(rendered))
	require.NoError(t, err)
	var code *html.Node
	walkNodes(document, func(node *html.Node) {
		if code == nil && node.Type == html.ElementNode && node.Data == "code" {
			code = node
		}
	})
	require.NotNil(t, code)
	return code
}

func elementsByClass(root *html.Node, class string) []*html.Node {
	var matches []*html.Node
	walkNodes(root, func(node *html.Node) {
		if node.Type == html.ElementNode && containsWord(attribute(node, "class"), class) {
			matches = append(matches, node)
		}
	})
	return matches
}

func elementsByTag(root *html.Node, tag string) []*html.Node {
	var matches []*html.Node
	walkNodes(root, func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == tag {
			matches = append(matches, node)
		}
	})
	return matches
}

func walkNodes(node *html.Node, visit func(*html.Node)) {
	visit(node)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkNodes(child, visit)
	}
}

func classList(node *html.Node) []string {
	return strings.Fields(attribute(node, "class"))
}

func containsWord(words, word string) bool {
	for _, candidate := range strings.Fields(words) {
		if candidate == word {
			return true
		}
	}
	return false
}

func attribute(node *html.Node, name string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == name {
			return attribute.Val
		}
	}
	return ""
}

func nodeText(node *html.Node) string {
	var text strings.Builder
	walkNodes(node, func(node *html.Node) {
		if node.Type == html.TextNode {
			text.WriteString(node.Data)
		}
	})
	return text.String()
}

func renderNode(node *html.Node) string {
	var rendered strings.Builder
	_ = html.Render(&rendered, node)
	return rendered.String()
}
