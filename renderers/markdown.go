package renderers

import (
	"bytes"
	"io"
	"regexp"

	chromahtml "github.com/alecthomas/chroma/formatters/html"
	"github.com/osteele/gojekyll/utils"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
	highlighting "github.com/yuin/goldmark-highlighting"
	"golang.org/x/net/html"
)

// goldmarkEngine is a shared goldmark instance configured with extensions
// matching Jekyll's kramdown+GFM behavior.
var goldmarkEngine = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,            // tables, strikethrough, autolinks, task lists
		extension.DefinitionList, // definition lists
		extension.Footnote,       // footnotes
		highlighting.NewHighlighting(
			highlighting.WithFormatOptions(
				chromahtml.WithClasses(true),
				chromahtml.WithLineNumbers(false),
			),
			highlighting.WithWrapperRenderer(func(w util.BufWriter, c highlighting.CodeBlockContext, entering bool) {
				lang, ok := c.Language()
				if entering {
					if ok {
						_, _ = w.WriteString(`<div class="language-` + string(lang) + ` highlighter-rouge"><div class="highlight">`)
					}
				} else {
					if ok {
						_, _ = w.WriteString("</div></div>")
					}
				}
			}),
		),
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(), // auto-generate heading IDs
		parser.WithAttribute(),     // support {#id .class key="value"} on headings
	),
	goldmark.WithRendererOptions(
		gmhtml.WithXHTML(),   // self-closing tags like <br />
		gmhtml.WithUnsafe(),  // allow raw HTML passthrough
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

func renderMarkdown(md []byte) ([]byte, error) {
	// Preprocess kramdown-style IALs to Pandoc-style for goldmark
	md = preprocessIAL(md)
	out, err := goldmarkConvert(md)
	if err != nil {
		return nil, utils.WrapError(err, "markdown")
	}
	out, err = renderInnerMarkdown(out)
	if err != nil {
		return nil, utils.WrapError(err, "markdown")
	}
	return out, nil
}

func _renderMarkdown(md []byte) ([]byte, error) {
	md = preprocessIAL(md)
	return goldmarkConvert(md)
}

// search HTML for markdown=1, and process if found
func renderInnerMarkdown(b []byte) ([]byte, error) {
	z := html.NewTokenizer(bytes.NewReader(b))
	buf := new(bytes.Buffer)
outer:
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				break outer
			}
			return nil, z.Err()
		case html.StartTagToken:
			if hasMarkdownAttr(z) {
				_, err := buf.Write(stripMarkdownAttr(z.Raw()))
				if err != nil {
					return nil, err
				}
				if err := processInnerMarkdown(buf, z); err != nil {
					return nil, err
				}
				// the above leaves z set to the end token
				// fall through to render it
			}
		}
		_, err := buf.Write(z.Raw())
		if err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func hasMarkdownAttr(z *html.Tokenizer) bool {
	for {
		k, v, more := z.TagAttr()
		switch {
		case string(k) == "markdown" && string(v) == "1":
			return true
		case !more:
			return false
		}
	}
}

var markdownAttrRE = regexp.MustCompile(`\s*markdown\s*=[^\s>]*\s*`)

// return the text of a start tag, w/out the markdown attribute
func stripMarkdownAttr(tag []byte) []byte {
	tag = markdownAttrRE.ReplaceAll(tag, []byte(" "))
	tag = bytes.Replace(tag, []byte(" >"), []byte(">"), 1)
	return tag
}

// Used inside markdown=1.
// TODO Instead of this approach, only count tags that match the start
// tag. For example, if <div markdown="1"> kicked off the inner markdown,
// count the div depth.
var notATagRE = regexp.MustCompile(`@|(https?|ftp):`)

// called once markdown="1" attribute is detected.
// Collects the HTML tokens into a string, applies markdown to them,
// and writes the result
func processInnerMarkdown(w io.Writer, z *html.Tokenizer) error {
	buf := new(bytes.Buffer)
	depth := 1
loop:
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				// End of document reached before matching end tag;
				// render whatever content we collected.
				break loop
			}
			return z.Err()
		case html.StartTagToken:
			if !notATagRE.Match(z.Raw()) {
				depth++
			}
		case html.EndTagToken:
			depth--
			if depth == 0 {
				break loop
			}
		}
		_, err := buf.Write(z.Raw())
		if err != nil {
			return err
		}
	}
	html, err := _renderMarkdown(buf.Bytes())
	if err != nil {
		return err
	}
	_, err = w.Write(html)
	return err
}
