package renderers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

func TestRenderMarkdownCodeLineNumbers(t *testing.T) {
	source := "one\n\nthree\n"
	plain := mustMarkdownString("```text\n" + source + "```\n")
	numbered := mustMarkdownString("```text showLineNumbers\n" + source + "```\n")
	started := mustMarkdownString("```text startLineNumber=8\n" + source + "```\n")
	before := mustMarkdownString("```text showLineNumbers startLineNumber=8\none\n```\n")
	after := mustMarkdownString("```text startLineNumber=8 showLineNumbers\none\n```\n")
	maximum := mustMarkdownString("```text startLineNumber=999999\none\n```\n")

	require.NotContains(t, plain, "data-line-number")
	require.NotContains(t, plain, "code-line")
	require.Equal(t, source, nodeText(parsedCode(t, numbered)))
	require.Equal(t, []string{"1", "2", "3"}, lineNumberValues(parsedCode(t, numbered)))
	require.Equal(t, []string{"8", "9", "10"}, lineNumberValues(parsedCode(t, started)))
	require.Equal(t, []string{"8"}, lineNumberValues(parsedCode(t, before)))
	require.Equal(t, []string{"8"}, lineNumberValues(parsedCode(t, after)))
	require.Equal(t, []string{"999999"}, lineNumberValues(parsedCode(t, maximum)))
	require.Equal(t, "\n", nodeText(elementsByClass(parsedCode(t, numbered), "code-line")[1]))
	require.NotContains(t, nodeText(parsedCode(t, started)), "8")
	require.NotContains(t, numbered, "jigyll-code-annotations")
}

func TestRenderMarkdownCodeLineNumbersPreserveChromaAndSource(t *testing.T) {
	source := "if ready {\n\tfmt.Println(\"yes\")\n}\n"
	plain := mustMarkdownString("```go\n" + source + "```\n")
	numbered := mustMarkdownString("```go showLineNumbers\n" + source + "```\n")

	require.Equal(t, source, nodeText(parsedCode(t, numbered)))
	require.Contains(t, numbered, `<span class="k">if</span>`)
	require.Contains(t, numbered, `<span class="nf">Println</span>`)
	require.Equal(t, 3, len(lineNumberValues(parsedCode(t, numbered))))
	require.Equal(t, countClassOccurrences(renderedPre(plain), "k"), countClassOccurrences(renderedPre(numbered), "k"))
	require.Equal(t, countClassOccurrences(renderedPre(plain), "nf"), countClassOccurrences(renderedPre(numbered), "nf"))
}

func TestRenderMarkdownCodeLineNumbersComposeWithFramesAndMarkers(t *testing.T) {
	source := "keep\nadded\nremove value\n"
	out := mustMarkdownString("```text title=\"changes.txt\" startLineNumber=8 {1} ins={2} del=\"remove\"\n" + source + "```\n")
	code := parsedCode(t, out)
	lines := elementsByClass(code, "code-line")

	require.Contains(t, out, `data-code-frame="editor"`)
	require.Equal(t, []string{"8", "9", "10"}, lineNumberValues(code))
	require.Contains(t, classList(lines[0]), "is-marked")
	require.Contains(t, classList(lines[1]), "is-inserted")
	require.Equal(t, "remove", nodeText(elementsByClass(code, "is-deleted")[0]))
	require.Equal(t, source, nodeText(code))
}

func TestRenderMarkdownCodeLineNumberValidation(t *testing.T) {
	tests := []struct {
		name    string
		info    string
		message string
	}{
		{name: "assigned bare flag", info: `showLineNumbers=true`, message: "showLineNumbers must be a bare flag"},
		{name: "duplicate bare flag", info: `showLineNumbers showLineNumbers`, message: "duplicate showLineNumbers metadata"},
		{name: "missing start", info: `startLineNumber`, message: "startLineNumber must use a decimal value"},
		{name: "empty start", info: `startLineNumber=`, message: "startLineNumber must use a decimal value"},
		{name: "nondecimal start", info: `startLineNumber=8x`, message: "startLineNumber must use a decimal value"},
		{name: "negative start", info: `startLineNumber=-1`, message: "startLineNumber must use a decimal value"},
		{name: "zero start", info: `startLineNumber=0`, message: "startLineNumber must be between 1 and 999999"},
		{name: "large start", info: `startLineNumber=1000000`, message: "startLineNumber must be between 1 and 999999"},
		{name: "duplicate start", info: `startLineNumber=8 startLineNumber=9`, message: "duplicate startLineNumber metadata"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := renderMarkdown([]byte("intro\n\n```text " + tt.info + "\none\n```\n"))
			require.EqualError(t, err, "code fence metadata at line 3: "+tt.message)
		})
	}
}

func TestRenderReportsCodeLineNumberSourcePathAndLine(t *testing.T) {
	assertMarkdownRenderError(
		t,
		[]byte("intro\n\n```text startLineNumber=0\none\n```\n"),
		"guides/numbers.md",
		5,
		"guides/numbers.md: code fence metadata at line 7: startLineNumber must be between 1 and 999999",
	)
}

func lineNumberValues(code *html.Node) []string {
	lines := elementsByClass(code, "code-line")
	values := make([]string, len(lines))
	for index, line := range lines {
		values[index] = attribute(line, "data-line-number")
	}
	return values
}

func countClassOccurrences(rendered, class string) int {
	return strings.Count(rendered, `class="`+class+`"`)
}
