package docexport

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"net/url"
	"sort"
	"strings"

	"github.com/reidransom/jigyll/renderers"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"
	"golang.org/x/net/html"
)

type sourceEdit struct {
	start, end int
	value      string
}
type sourceRange struct{ start, end int }

// Markdown rebases destinations in resolved article source, preserving all other
// source bytes, especially code, tables, raw Liquid examples, and heading text.
// assets is the actual publish revision map, or nil before asset processing.
func Markdown(source []byte, canonical string, assets map[string]string) ([]byte, error) {
	base, err := url.Parse(canonical)
	if err != nil || base.Host == "" {
		return nil, fmt.Errorf("invalid canonical article URL %q", canonical)
	}
	context := parser.NewContext()
	doc := renderers.ParseMarkdown(source, context)
	destinations, protected, rawHTML, err := collectMarkdownMetadata(doc)
	if err != nil {
		return nil, err
	}
	edits := scanMarkdownEdits(source, context, destinations, protected, rawHTML, base, assets)
	return applyMarkdownEdits(source, edits)
}

func collectMarkdownMetadata(doc ast.Node) (map[string]bool, []sourceRange, []sourceRange, error) {
	destinations := map[string]bool{}
	protected := []sourceRange{}
	rawHTML := []sourceRange{}
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Link:
			destinations[unescapeDestinationBytes(n.Destination)] = true
		case *ast.Image:
			destinations[unescapeDestinationBytes(n.Destination)] = true
		case *ast.HTMLBlock:
			rawHTML = appendBlockLines(rawHTML, n)
			if n.HasClosure() {
				rawHTML = append(rawHTML, sourceRange{n.ClosureLine.Start, n.ClosureLine.Stop})
			}
		case *ast.CodeBlock, *ast.FencedCodeBlock:
			protected = appendBlockLines(protected, n)
		}
		return ast.WalkContinue, nil
	})
	sort.Slice(protected, func(i, j int) bool { return protected[i].start < protected[j].start })
	sort.Slice(rawHTML, func(i, j int) bool { return rawHTML[i].start < rawHTML[j].start })
	return destinations, protected, rawHTML, err
}

func unescapeDestinationBytes(raw []byte) string {
	return stdhtml.UnescapeString(string(util.UnescapePunctuations(raw)))
}

func appendBlockLines(ranges []sourceRange, n ast.Node) []sourceRange {
	for i := range n.Lines().Len() {
		line := n.Lines().At(i)
		ranges = append(ranges, sourceRange{line.Start, line.Stop})
	}
	return ranges
}

func scanMarkdownEdits(source []byte, context parser.Context, destinations map[string]bool, protected, rawHTML []sourceRange, base *url.URL, assets map[string]string) []sourceEdit {
	var edits []sourceEdit
	protection, rawPosition, brackets := 0, 0, 0
	for i := 0; i < len(source); {
		if next, skipped := skipProtectedPosition(i, protected, &protection); skipped {
			i = next
			continue
		}
		if skipRawTextPosition(source, i, rawHTML, &rawPosition) {
			i++
			continue
		}
		switch source[i] {
		case '\\':
			i += 2
		case '`':
			i = skipCodeSpan(source, i)
		case '[':
			brackets++
			i++
		case ']':
			i, edits = scanCloseBracket(source, i, &brackets, context, destinations, base, assets, edits)
		case '<':
			i, edits = scanHTMLToken(source, i, base, assets, edits)
		default:
			i++
		}
	}
	return edits
}

func skipProtectedPosition(i int, ranges []sourceRange, idx *int) (int, bool) {
	for *idx < len(ranges) && ranges[*idx].end <= i {
		*idx++
	}
	if *idx < len(ranges) && i >= ranges[*idx].start {
		return ranges[*idx].end, true
	}
	return i, false
}

func skipRawTextPosition(source []byte, i int, ranges []sourceRange, idx *int) bool {
	for *idx < len(ranges) && ranges[*idx].end <= i {
		*idx++
	}
	return *idx < len(ranges) && i >= ranges[*idx].start && source[i] != '<'
}

func skipCodeSpan(source []byte, i int) int {
	count := 1
	for i+count < len(source) && source[i+count] == '`' {
		count++
	}
	stop := i + count
	for stop < len(source) {
		j := bytes.IndexByte(source[stop:], '`')
		if j < 0 {
			break
		}
		stop += j
		run := 1
		for stop+run < len(source) && source[stop+run] == '`' {
			run++
		}
		if run == count {
			return stop + run
		}
		stop += run
	}
	return i + count
}

func scanCloseBracket(source []byte, i int, brackets *int, context parser.Context, destinations map[string]bool, base *url.URL, assets map[string]string, edits []sourceEdit) (int, []sourceEdit) {
	if *brackets == 0 {
		return i + 1, edits
	}
	*brackets--
	if i+1 >= len(source) || (source[i+1] != '(' && source[i+1] != ':') {
		return i + 1, edits
	}
	definition := source[i+1] == ':'
	reference, ok := lookupReferenceDefinition(source, i, definition, context)
	if !ok {
		return i + 1, edits
	}
	start, end, ok := markdownDestination(source, i+2)
	if !ok {
		return i + 1, edits
	}
	decoded := unescapeDestinationBytes(source[start:end])
	active := destinations[decoded]
	if definition {
		active = decoded == unescapeDestinationBytes(reference.Destination())
	}
	if !active {
		return end, edits
	}
	if value := rebaseDestination(decoded, base, assets); value != decoded {
		edits = append(edits, sourceEdit{start, end, serializeDestination(value, start > 0 && source[start-1] == '<')})
	}
	return markdownLinkEnd(source, end, definition), edits
}

func lookupReferenceDefinition(source []byte, i int, definition bool, context parser.Context) (parser.Reference, bool) {
	if !definition {
		return nil, true
	}
	line := bytes.LastIndexByte(source[:i], '\n') + 1
	opening := bytes.IndexByte(source[line:i], '[')
	if opening < 0 || !definitionPrefix(source[line:line+opening]) {
		return nil, false
	}
	return context.Reference(util.ToLinkReference(source[line+opening+1 : i]))
}

func scanHTMLToken(source []byte, i int, base *url.URL, assets map[string]string, edits []sourceEdit) (int, []sourceEdit) {
	tokenizer := html.NewTokenizer(bytes.NewReader(source[i:]))
	kind := tokenizer.Next()
	if kind == html.CommentToken {
		return i + len(tokenizer.Raw()), edits
	}
	if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
		return i + 1, edits
	}
	raw := tokenizer.Raw()
	token := tokenizer.Token()
	if rewriteTokenAttributes(&token, base, assets) {
		edits = append(edits, sourceEdit{i, i + len(raw), token.String()})
	}
	if isRawHTMLElement(token.Data) {
		return i + rawElementLength(tokenizer, token.Data, len(raw)), edits
	}
	return i + len(raw), edits
}

func rewriteTokenAttributes(token *html.Token, base *url.URL, assets map[string]string) bool {
	changed := false
	for a := range token.Attr {
		attribute := &token.Attr[a]
		value := attribute.Val
		switch attribute.Key {
		case "href", "src", "poster", "action", "data-manifest":
			value = rebaseDestination(value, base, assets)
		case "srcset":
			value = rebaseSrcset(value, base, assets)
		}
		if value != attribute.Val {
			attribute.Val = value
			changed = true
		}
	}
	return changed
}

func isRawHTMLElement(tag string) bool {
	return tag == "pre" || tag == "code" || tag == "script" || tag == "style" || tag == "textarea"
}

func applyMarkdownEdits(source []byte, edits []sourceEdit) ([]byte, error) {
	if len(edits) == 0 {
		return source, nil
	}
	var out bytes.Buffer
	cursor := 0
	for _, edit := range edits {
		if edit.start < cursor {
			return nil, fmt.Errorf("overlapping Markdown destinations")
		}
		out.Write(source[cursor:edit.start])
		out.WriteString(edit.value)
		cursor = edit.end
	}
	out.Write(source[cursor:])
	return out.Bytes(), nil
}

func markdownDestination(source []byte, start int) (int, int, bool) {
	start = skipMarkdownWhitespace(source, start)
	if start >= len(source) {
		return 0, 0, false
	}
	if source[start] == '<' {
		return angleDestination(source, start)
	}
	return bareDestination(source, start)
}

func skipMarkdownWhitespace(source []byte, start int) int {
	for start < len(source) && isMarkdownSpace(source[start]) {
		start++
	}
	return start
}

func isMarkdownSpace(b byte) bool {
	return b == ' ' || b == '\n' || b == '\r' || b == '\t'
}

func angleDestination(source []byte, start int) (int, int, bool) {
	for end := start + 1; end < len(source); end++ {
		if source[end] == '\\' {
			end++
			continue
		}
		if source[end] == '>' {
			return start + 1, end, true
		}
		if source[end] == '\n' {
			break
		}
	}
	return 0, 0, false
}

func bareDestination(source []byte, start int) (int, int, bool) {
	depth := 0
	for end := start; end < len(source); end++ {
		switch source[end] {
		case '\\':
			end++
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return start, end, true
			}
			depth--
		case ' ', '\t', '\n', '\r':
			if depth == 0 {
				return start, end, end > start
			}
		}
	}
	return start, len(source), len(source) > start
}

func rebaseDestination(value string, base *url.URL, assets map[string]string) string {
	ref, err := url.Parse(value)
	if err != nil {
		return value
	}
	if ref.Scheme != "" && ref.Scheme != "http" && ref.Scheme != "https" {
		return value
	}
	resolved := base.ResolveReference(ref)
	target, found := assets[value]
	if !found {
		target, found = assets[resolved.String()]
	}
	if !found && resolved.Host == base.Host && resolved.Scheme == base.Scheme {
		target, found = assets[resolved.Path]
	}
	if found {
		revised, err := url.Parse(target)
		if err == nil {
			revised.RawQuery = resolved.RawQuery
			revised.Fragment = resolved.Fragment
			return revised.String()
		}
	}
	if ref.IsAbs() || ref.Host != "" {
		return value
	}
	return resolved.String()
}

func rebaseSrcset(value string, base *url.URL, assets map[string]string) string {
	// A URL token ends at whitespace; data URLs may contain commas. Descriptors
	// end at the candidate comma. Preserve data URLs and descriptor text.
	var out strings.Builder
	for len(value) > 0 {
		spaces := len(value) - len(strings.TrimLeft(value, " \t\n\r"))
		out.WriteString(value[:spaces])
		value = value[spaces:]
		if value == "" {
			break
		}
		end := strings.IndexAny(value, " \t\n\r")
		if end < 0 {
			end = len(value)
		}
		token := value[:end]
		suffix := ""
		if strings.HasSuffix(token, ",") {
			token = strings.TrimSuffix(token, ",")
			suffix = ","
		}
		out.WriteString(rebaseDestination(token, base, assets))
		out.WriteString(suffix)
		value = value[end:]
		if suffix == "" {
			if comma := strings.IndexByte(value, ','); comma >= 0 {
				out.WriteString(value[:comma+1])
				value = value[comma+1:]
			} else {
				out.WriteString(value)
				break
			}
		}
	}
	return out.String()
}

// rawElementLength follows HTML tokenization, including valid whitespace in
// end tags, while preserving all raw-text/code bytes after the opening tag.
func rawElementLength(tokenizer *html.Tokenizer, tag string, consumed int) int {
	depth := 1
	for {
		kind := tokenizer.Next()
		consumed += len(tokenizer.Raw())
		if kind == html.ErrorToken {
			return consumed
		}
		if kind == html.StartTagToken || kind == html.EndTagToken {
			name, _ := tokenizer.TagName()
			if string(name) == tag {
				if kind == html.StartTagToken {
					depth++
				} else {
					depth--
					if depth == 0 {
						return consumed
					}
				}
			}
		}
	}
}

func definitionPrefix(prefix []byte) bool {
	for {
		prefix = bytes.TrimLeft(prefix, " \t")
		if len(prefix) == 0 {
			return true
		}
		if prefix[0] == '>' {
			prefix = prefix[1:]
			continue
		}
		rest, ok := consumeListMarker(prefix)
		if !ok {
			return false
		}
		prefix = rest
	}
}

func consumeListMarker(prefix []byte) ([]byte, bool) {
	marker := listMarkerLength(prefix)
	if marker == 0 || marker >= len(prefix) || (prefix[marker] != ' ' && prefix[marker] != '\t') {
		return nil, false
	}
	return prefix[marker+1:], true
}

func listMarkerLength(prefix []byte) int {
	if strings.ContainsRune("-+*", rune(prefix[0])) {
		return 1
	}
	marker := 0
	for marker < len(prefix) && marker < 9 && prefix[marker] >= '0' && prefix[marker] <= '9' {
		marker++
	}
	if marker == 0 || marker >= len(prefix) || (prefix[marker] != '.' && prefix[marker] != ')') {
		return 0
	}
	return marker + 1
}

func serializeDestination(value string, angle bool) string {
	if angle {
		return strings.NewReplacer("<", "%3C", ">", "%3E").Replace(value)
	}
	depth := 0
	for _, character := range value {
		switch character {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return "<" + strings.NewReplacer("<", "%3C", ">", "%3E").Replace(value) + ">"
			}
		}
	}
	if depth != 0 {
		return "<" + strings.NewReplacer("<", "%3C", ">", "%3E").Replace(value) + ">"
	}
	return value
}

// Skip link titles as syntax, not as another Markdown document to rewrite.
func markdownLinkEnd(source []byte, end int, definition bool) int {
	cursor := end
	if cursor < len(source) && source[cursor] == '>' {
		cursor++
	}
	gap := cursor
	cursor, lines := skipSpaceCountNewlines(source, cursor)
	if lines > 1 || cursor >= len(source) {
		return end
	}
	if !definition && source[cursor] == ')' {
		return cursor + 1
	}
	if gap == cursor || !strings.ContainsRune("\"'(", rune(source[cursor])) {
		return end
	}
	return scanLinkTitle(source, cursor, end, definition)
}

func skipSpaceCountNewlines(source []byte, cursor int) (int, int) {
	lines := 0
	for cursor < len(source) && isMarkdownSpace(source[cursor]) {
		if source[cursor] == '\n' {
			lines++
		}
		cursor++
	}
	return cursor, lines
}

func scanLinkTitle(source []byte, cursor, fallback int, definition bool) int {
	closing := source[cursor]
	if closing == '(' {
		closing = ')'
	}
	for cursor++; cursor < len(source); cursor++ {
		if source[cursor] == '\\' {
			cursor++
			continue
		}
		if source[cursor] == closing {
			return finishLinkTitle(source, cursor+1, fallback, definition)
		}
	}
	return fallback
}

func finishLinkTitle(source []byte, after, fallback int, definition bool) int {
	for after < len(source) && (source[after] == ' ' || source[after] == '\t' || source[after] == '\r') {
		after++
	}
	if definition {
		if after == len(source) || source[after] == '\n' {
			return after
		}
	} else if after < len(source) && source[after] == ')' {
		return after + 1
	}
	return fallback
}
