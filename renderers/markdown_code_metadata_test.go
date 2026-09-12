package renderers

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/liquid"
	"github.com/stretchr/testify/require"
)

func TestRenderMarkdownCodeFrames(t *testing.T) {
	t.Run("editor title", func(t *testing.T) {
		out := mustMarkdownString("```go title=\"Editor & \\\"quoted\\\"\"\nfmt.Println(\"hello\")\n```\n")
		require.Contains(t, out, `<figure class="highlight" data-code-frame="editor">`)
		require.Contains(t, out, `<figcaption class="code-title">Editor &amp; &#34;quoted&#34;</figcaption>`)
		require.Contains(t, out, `class="language-go highlighter-rouge"`)
		require.Contains(t, out, `<span class="nf">Println</span>`)
		require.Equal(t, 1, strings.Count(out, "Editor &amp;"))
		require.NotContains(t, codeText(out), "Editor")
	})

	t.Run("terminal title inferred", func(t *testing.T) {
		out := mustMarkdownString("```bash title=\"Install\"\nprintf 'ready\\n'\n```\n")
		require.Contains(t, out, `data-code-frame="terminal"`)
		require.Contains(t, out, `<figcaption class="code-title">Install</figcaption>`)
	})

	t.Run("explicit frame wins", func(t *testing.T) {
		out := mustMarkdownString("```bash title=\"Script\" frame=\"editor\"\necho ready\n```\n")
		require.Contains(t, out, `data-code-frame="editor"`)
		require.NotContains(t, out, `data-code-frame="terminal"`)
	})

	t.Run("untitled frame", func(t *testing.T) {
		out := mustMarkdownString("```text frame=\"terminal\"\noutput\n```\n")
		require.Contains(t, out, `<figure class="highlight" data-code-frame="terminal">`)
		require.NotContains(t, out, `<figcaption`)
	})
}

func TestRenderMarkdownCodeFrameInference(t *testing.T) {
	for _, language := range []string{"bash", "sh", "shell", "console", "powershell", "ps1"} {
		t.Run(language, func(t *testing.T) {
			out := mustMarkdownString("~~~" + language + " title=\"C:\\\\work\"\ncommand\n~~~\n")
			require.Contains(t, out, `data-code-frame="terminal"`)
			require.Contains(t, out, `<figcaption class="code-title">C:\work</figcaption>`)
		})
	}
}

func TestRenderMarkdownCodeFramePreservesHighlightedCode(t *testing.T) {
	source := "if ready {\n\tfmt.Println(\"yes\")\n}\n"
	plain := mustMarkdownString("```go\n" + source + "```\n")
	framed := mustMarkdownString("```go title=\"main.go\"\n" + source + "```\n")

	require.Equal(t, renderedPre(plain), renderedPre(framed))
	require.Contains(t, renderedPre(framed), `<span class="k">if</span>`)
	require.Contains(t, renderedPre(framed), "\n</span></span></code></pre>")
}

func TestRenderMarkdownCodeFramePreservesExistingFenceAttributes(t *testing.T) {
	out := mustMarkdownString("```go {linenos=true} title=\"main.go\"\nfmt.Println(\"hello\")\n```\n")

	require.Contains(t, out, `data-code-frame="editor"`)
	require.Contains(t, out, `<figcaption class="code-title">main.go</figcaption>`)
	require.Contains(t, out, `<span class="ln">1</span>`)
}

func TestRenderMarkdownCodeMetadataPreservesUnframedOutput(t *testing.T) {
	plain := mustMarkdownString("```go\nfmt.Println(\"hello\")\n```\n")
	unknown := mustMarkdownString("```go future=\"retained\"\nfmt.Println(\"hello\")\n```\n")
	none := mustMarkdownString("```go frame=\"none\"\nfmt.Println(\"hello\")\n```\n")

	require.Equal(t, plain, unknown)
	require.Equal(t, plain, none)
	require.NotContains(t, plain, "data-code-frame")
	require.Contains(t, plain, `<div class="language-go highlighter-rouge"><div class="highlight">`)
}

func TestRenderMarkdownCodeMetadataValidation(t *testing.T) {
	tests := []struct {
		name    string
		info    string
		message string
	}{
		{name: "unquoted title", info: `title=readme`, message: `title must use a double-quoted value`},
		{name: "single-quoted title", info: `title='readme'`, message: `title must use a double-quoted value`},
		{name: "empty title", info: `title=""`, message: `title must not be empty`},
		{name: "invalid title escape", info: `title="bad\nvalue"`, message: `title contains invalid escape \n`},
		{name: "unterminated title", info: `title="readme`, message: `title has an unterminated double-quoted value`},
		{name: "duplicate title", info: `title="one" title="two"`, message: `duplicate title metadata`},
		{name: "unquoted frame", info: `frame=editor`, message: `frame must use a double-quoted value`},
		{name: "invalid frame", info: `frame="browser"`, message: `frame must be one of "editor", "terminal", or "none"`},
		{name: "duplicate frame", info: `frame="editor" frame="terminal"`, message: `duplicate frame metadata`},
		{name: "discarded title", info: `title="readme" frame="none"`, message: `frame="none" cannot be combined with title`},
		{name: "standalone title", info: `title`, message: `title must use a double-quoted value`},
		{name: "trailing title text", info: `title="readme"x`, message: `title must be followed by whitespace`},
		{name: "trailing frame text", info: `frame="editor"x`, message: `frame must be followed by whitespace`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := renderMarkdown([]byte("intro\n\n```go " + tt.info + "\ncode\n```\n"))
			require.EqualError(t, err, "code fence metadata at line 3: "+tt.message)
		})
	}
}

func TestRenderReportsCodeMetadataSourcePathAndLine(t *testing.T) {
	cfg := config.Default()
	cfg.Source = t.TempDir()
	manager, err := New(cfg, Options{})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(manager.sassTempDir))
	})

	var output bytes.Buffer
	err = manager.Render(
		&output,
		[]byte("intro\n\n```go title=readme\ncode\n```\n"),
		liquid.Bindings{},
		"guides/example.md",
		5,
	)
	require.EqualError(t, err, "guides/example.md: code fence metadata at line 7: title must use a double-quoted value")
}

func codeText(rendered string) string {
	start := strings.Index(rendered, "<code")
	if start == -1 {
		return ""
	}
	return rendered[start:]
}

func renderedPre(rendered string) string {
	start := strings.Index(rendered, "<pre")
	end := strings.Index(rendered, "</pre>")
	if start == -1 || end == -1 {
		return ""
	}
	return rendered[start : end+len("</pre>")]
}
