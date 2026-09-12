package renderers

import (
	"bytes"
	"encoding/base64"
	"html"
	"regexp"

	chromahtml "github.com/alecthomas/chroma/formatters/html"
	"github.com/gohugoio/hugo-goldmark-extensions/passthrough"
	"github.com/reidransom/jigyll/utils"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// goldmarkEngine is a shared goldmark instance configured with extensions
// matching Jekyll's kramdown+GFM behavior.
var goldmarkEngine = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,            // tables, strikethrough, autolinks, task lists
		extension.DefinitionList, // definition lists
		extension.Footnote,       // footnotes
		passthrough.New(passthrough.Config{ // math delimiters preserved for client-side MathJax/KaTeX
			InlineDelimiters: []passthrough.Delimiters{
				{Open: "$$", Close: "$$"},
			},
			BlockDelimiters: []passthrough.Delimiters{
				{Open: "$$", Close: "$$"},
			},
		}),
		highlighting.NewHighlighting(
			highlighting.WithFormatOptions(
				chromahtml.WithClasses(true),
				chromahtml.WithLineNumbers(false),
			),
			highlighting.WithWrapperRenderer(func(w util.BufWriter, c highlighting.CodeBlockContext, entering bool) {
				lang, hasLanguage := c.Language()
				frame, title := codeFrameDetails(c)
				if entering {
					if hasLanguage {
						_, _ = w.WriteString(`<div class="language-` + string(lang) + ` highlighter-rouge">`)
					}
					if frame != "" {
						_, _ = w.WriteString(`<figure class="highlight" data-code-frame="` + frame + `">`)
						if title != "" {
							_, _ = w.WriteString(`<figcaption class="code-title">` + html.EscapeString(title) + `</figcaption>`)
						}
					} else if hasLanguage {
						_, _ = w.WriteString(`<div class="highlight">`)
					}
					// When chroma has no lexer for the fence language (e.g. `liquid`)
					// or the fence is unlabeled, goldmark-highlighting takes its
					// fallback path and writes the raw code with no <pre><code> of
					// its own — so emit one here to match rouge's output for
					// unknown languages. Without this the code renders as
					// whitespace-collapsed inline text.
					if !c.Highlighted() {
						_, _ = w.WriteString(`<pre class="highlight"><code>`)
					}
				} else {
					if !c.Highlighted() {
						_, _ = w.WriteString("</code></pre>")
					}
					if frame != "" {
						_, _ = w.WriteString("</figure>")
					} else if hasLanguage {
						_, _ = w.WriteString("</div>")
					}
					if hasLanguage {
						_, _ = w.WriteString("</div>")
					}
				}
			}),
		),
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(), // auto-generate heading IDs
		parser.WithAttribute(),     // support {#id .class key="value"} on headings
		parser.WithASTTransformers(
			util.Prioritized(kramdownIALTransformer{}, 100),
		),
	),
	goldmark.WithRendererOptions(
		gmhtml.WithXHTML(),  // self-closing tags like <br />
		gmhtml.WithUnsafe(), // allow raw HTML passthrough
	),
)

// goldmarkConvert renders markdown to HTML using the shared goldmark engine.
func goldmarkConvert(md []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := goldmarkEngine.Convert(md, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// kramdown allows a same-line IAL on ATX headings: `## Heading {: #id .class}`.
// goldmark's attribute parser expects Pandoc-style `{#id .class}` (no colon),
// so rewrite the IAL on heading lines only. The colon must be followed by
// whitespace — that keeps kramdown's TOC markers (`{:toc}`, `{:.no_toc}`)
// literal so the TOC pass can still see them in the rendered HTML.
var (
	headingIALLineRE = regexp.MustCompile(`^(#{1,6}\s.*)\{:\s+([^}]+)\}(\s*)$`)
	codeFenceRE      = regexp.MustCompile("^ {0,3}(```|~~~)")
)

// mapLinesOutsideFences applies fn to each line of md that is not part of a
// fenced code block (``` or ~~~), leaving fenced content untouched.
func mapLinesOutsideFences(md []byte, fn func(line []byte) []byte) []byte {
	lines := bytes.Split(md, []byte("\n"))
	inFence := false
	for i, line := range lines {
		if codeFenceRE.Match(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		lines[i] = fn(line)
	}
	return bytes.Join(lines, []byte("\n"))
}

func preprocessHeadingIALs(md []byte) []byte {
	return mapLinesOutsideFences(md, func(line []byte) []byte {
		return headingIALLineRE.ReplaceAll(line, []byte("$1{$2}$3"))
	})
}

// deIndentHTMLBlocks removes leading indentation from lines inside HTML blocks.
// Kramdown doesn't treat 4-space indented content inside HTML blocks as code,
// but CommonMark/Goldmark does. This preprocessor strips the indentation so
// Goldmark renders the HTML correctly.
//
// An HTML block starts with a line beginning with an HTML block-level tag
// (optionally preceded by up to 3 spaces) and ends at a blank line. Fenced
// code blocks are left untouched — upstream's version lacks that guard and
// strips the indentation out of HTML-looking code samples.
var htmlBlockStartRE = regexp.MustCompile(`(?i)^\s{0,3}</?(?:address|article|aside|blockquote|details|dialog|dd|div|dl|dt|fieldset|figcaption|figure|footer|form|h[1-6]|header|hgroup|hr|li|main|nav|ol|p|pre|section|summary|table|ul)\b`)

func deIndentHTMLBlocks(md []byte) []byte {
	inHTMLBlock := false
	return mapLinesOutsideFences(md, func(line []byte) []byte {
		if !inHTMLBlock && htmlBlockStartRE.Match(line) {
			inHTMLBlock = true
		}
		if !inHTMLBlock {
			return line
		}
		if len(bytes.TrimSpace(line)) == 0 {
			inHTMLBlock = false
			return line
		}
		// Remove up to 4 leading spaces from lines inside HTML blocks
		trimmed := line
		for i := 0; i < 4; i++ {
			if len(trimmed) > 0 && trimmed[0] == ' ' {
				trimmed = trimmed[1:]
			} else {
				break
			}
		}
		return trimmed
	})
}

func renderMarkdown(md []byte) ([]byte, error) {
	return renderMarkdownWithOptionsAtLine(md, nil, 1)
}

func renderMarkdownWithOptions(md []byte, opts *TOCOptions) ([]byte, error) {
	return renderMarkdownWithOptionsAtLine(md, opts, 1)
}

func renderMarkdownWithOptionsAtLine(md []byte, opts *TOCOptions, firstLine int) ([]byte, error) {
	// Set default options if not provided
	// Jekyll's default toc_levels is "2..6" to exclude H1 headings
	if opts == nil {
		opts = &TOCOptions{
			MinLevel:      2,
			MaxLevel:      6,
			UseJekyllHTML: true, // Use Jekyll-compatible HTML structure by default
		}
	}
	// Ensure valid level ranges
	if opts.MinLevel < 1 {
		opts.MinLevel = 1
	}
	if opts.MaxLevel > 6 {
		opts.MaxLevel = 6
	}
	if opts.MinLevel > opts.MaxLevel {
		opts.MinLevel = 1
		opts.MaxLevel = 6
	}

	// Parse fenced-code UI metadata before other Markdown preprocessing. The
	// renderer consumes private attributes from this pass; callers see only
	// the stable semantic HTML seam.
	var err error
	md, err = preprocessCodeFenceMetadata(md, firstLine)
	if err != nil {
		return nil, err
	}
	md = preprocessHeadingIALs(md)
	md = deIndentHTMLBlocks(md)

	html, err := goldmarkConvert(md)
	if err != nil {
		return nil, utils.WrapError(err, "markdown conversion")
	}

	// Process inner markdown (for nested markdown rendering)
	html, err = renderInnerMarkdown(html)
	if err != nil {
		return nil, utils.WrapError(err, "markdown")
	}

	// Process TOC markers if they exist
	// Note: Only {:toc} is valid kramdown syntax; {::toc} is not processed
	// Jekyll only processes {:toc} in unordered lists, leaving literals elsewhere
	if tocPatternInline.Match(html) && shouldProcessTOC(html) {
		html, err = processTOC(html, opts)
		if err != nil {
			return nil, utils.WrapError(err, "toc generation")
		}
	}
	return html, nil
}

func codeFrameDetails(c highlighting.CodeBlockContext) (string, string) {
	attrs := c.Attributes()
	if attrs == nil {
		return "", ""
	}
	frame := codeMetadataAttribute(attrs, codeFrameAttribute)
	if frame != "editor" && frame != "terminal" {
		return "", ""
	}
	encodedTitle := codeMetadataAttribute(attrs, codeTitleAttribute)
	if encodedTitle == "" {
		return frame, ""
	}
	title, err := base64.RawURLEncoding.DecodeString(encodedTitle)
	if err != nil {
		return frame, ""
	}
	return frame, string(title)
}

func codeMetadataAttribute(attrs highlighting.ImmutableAttributes, name string) string {
	value, ok := attrs.GetString(name)
	if !ok {
		return ""
	}
	switch value := value.(type) {
	case []byte:
		return string(value)
	case string:
		return value
	default:
		return ""
	}
}

func _renderMarkdown(md []byte) ([]byte, error) {
	return goldmarkConvert(preprocessHeadingIALs(md))
}
