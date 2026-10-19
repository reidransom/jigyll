package docexport

import (
	"net/url"
	"testing"

	"github.com/reidransom/jigyll/renderers"
	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"
)

type semanticLink struct{ destination, title string }

func parsedLinks(t *testing.T, source []byte) []semanticLink {
	t.Helper()
	document := renderers.ParseMarkdown(source, parser.NewContext())
	links := []semanticLink{}
	require.NoError(t, ast.Walk(document, func(node ast.Node, enter bool) (ast.WalkStatus, error) {
		if link, ok := node.(*ast.Link); ok && enter {
			links = append(links, semanticLink{string(util.UnescapePunctuations(link.Destination)), string(util.UnescapePunctuations(link.Title))})
		}
		return ast.WalkContinue, nil
	}))
	return links
}

func TestMarkdownPreservesParsedLinkMeaning(t *testing.T) {
	base, _ := url.Parse("https://example.com/docs/guide/")
	for _, source := range []string{
		"[link](/target/ \"a literal [example](/target/)\")\n",
		"[link][ref]\n\n[ref]: /target/ \"a literal [example](/target/)\"\n",
		"[escaped](foo\\)bar)\n",
		"[angle](<foo\\>bar>)\n",
		"> [jump][target]\n>\n> [target]: #setup\n\n## Setup\n",
		"- [jump][target]\n\n  [target]: #setup\n\n## Setup\n",
	} {
		t.Run(source, func(t *testing.T) {
			before := parsedLinks(t, []byte(source))
			require.Len(t, before, 1)
			output, err := Markdown([]byte(source), base.String(), nil)
			require.NoError(t, err)
			after := parsedLinks(t, output)
			require.Len(t, after, 1)
			destination, err := url.Parse(before[0].destination)
			require.NoError(t, err)
			require.Equal(t, base.ResolveReference(destination).String(), after[0].destination)
			require.Equal(t, before[0].title, after[0].title)
		})
	}
}

func TestMarkdownFootnotesAreContentNotLinkDefinitions(t *testing.T) {
	source := []byte("Text[^1].\n\n[^1]: Footnote text.\n")
	output, err := Markdown(source, "https://example.com/docs/guide/", nil)
	require.NoError(t, err)
	require.Equal(t, string(source), string(output))
	linked := []byte("Text[^1].\n\n[^1]: [source](#details)\n\n## Details\n")
	output, err = Markdown(linked, "https://example.com/docs/guide/", nil)
	require.NoError(t, err)
	links := parsedLinks(t, output)
	require.Len(t, links, 1)
	require.Equal(t, "https://example.com/docs/guide/#details", links[0].destination)
}

func TestMarkdownRawElementDestinationsAndBodies(t *testing.T) {
	for _, closing := range []string{"</script>", "</script >", "</script\n>"} {
		t.Run(closing, func(t *testing.T) {
			source := "[go](/target/)\n\n<div data-manifest=\"/assets/example.json\"></div>\n<script src=\"/assets/widget.js\">\nconst x = \"[go](/target/)\";\n" + closing + "\n"
			output, err := Markdown([]byte(source), "https://example.com/docs/guide/", map[string]string{"/assets/widget.js": "https://cdn.example/widget.hash.js", "/assets/example.json": "https://cdn.example/example.hash.json"})
			require.NoError(t, err)
			require.Contains(t, string(output), "src=\"https://cdn.example/widget.hash.js\"")
			require.Contains(t, string(output), "data-manifest=\"https://cdn.example/example.hash.json\"")
			require.Contains(t, string(output), "const x = \"[go](/target/)\";")
			require.Contains(t, string(output), closing)
		})
	}
	source := []byte("[real](/target/)\n\n<div>\nLiteral [example](/target/).\n</div>\n")
	output, err := Markdown(source, "https://example.com/docs/guide/", nil)
	require.NoError(t, err)
	require.Contains(t, string(output), "Literal [example](/target/).")
}
