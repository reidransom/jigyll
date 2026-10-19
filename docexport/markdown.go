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
	destinations := map[string]bool{}
	protected := []sourceRange{}
	rawHTML := []sourceRange{}
	err = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Link:
			destinations[stdhtml.UnescapeString(string(util.UnescapePunctuations(n.Destination)))] = true
		case *ast.Image:
			destinations[stdhtml.UnescapeString(string(util.UnescapePunctuations(n.Destination)))] = true
		case *ast.HTMLBlock:
			for i := range n.Lines().Len() {
				line := n.Lines().At(i)
				rawHTML = append(rawHTML, sourceRange{line.Start, line.Stop})
			}
			if n.HasClosure() {
				rawHTML = append(rawHTML, sourceRange{n.ClosureLine.Start, n.ClosureLine.Stop})
			}
		case *ast.CodeBlock:
			for i := range n.Lines().Len() {
				line := n.Lines().At(i)
				protected = append(protected, sourceRange{line.Start, line.Stop})
			}
		case *ast.FencedCodeBlock:
			for i := range n.Lines().Len() {
				line := n.Lines().At(i)
				protected = append(protected, sourceRange{line.Start, line.Stop})
			}
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(protected, func(i, j int) bool { return protected[i].start < protected[j].start })
	sort.Slice(rawHTML, func(i, j int) bool { return rawHTML[i].start < rawHTML[j].start })
	edits := []sourceEdit{}
	protection := 0
	rawPosition := 0
	brackets := 0
	for i := 0; i < len(source); {
		for protection < len(protected) && protected[protection].end <= i {
			protection++
		}
		if protection < len(protected) && i >= protected[protection].start {
			i = protected[protection].end
			continue
		}
		for rawPosition < len(rawHTML) && rawHTML[rawPosition].end <= i {
			rawPosition++
		}
		if rawPosition < len(rawHTML) && i >= rawHTML[rawPosition].start && source[i] != '<' {
			i++
			continue
		}
		switch source[i] {
		case '\\':
			i += 2
			continue
		case '`':
			count := 1
			for i+count < len(source) && source[i+count] == '`' {
				count++
			}
			stop := i + count
			matched := false
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
					stop += run
					matched = true
					break
				}
				stop += run
			}
			if matched {
				i = stop
			} else {
				i += count
			}
			continue
		case '[':
			brackets++
		case ']':
			hadBracket := brackets > 0
			if hadBracket {
				brackets--
			}
			if hadBracket && i+1 < len(source) && (source[i+1] == '(' || source[i+1] == ':') {
				definition := source[i+1] == ':'
				var reference parser.Reference
				if definition {
					line := bytes.LastIndexByte(source[:i], '\n') + 1
					opening := bytes.IndexByte(source[line:i], '[')
					var found bool
					if opening >= 0 && definitionPrefix(source[line:line+opening]) {
						reference, found = context.Reference(util.ToLinkReference(source[line+opening+1 : i]))
					}
					if !found {
						i++
						continue
					}
				}
				start, end, ok := markdownDestination(source, i+2)
				if ok {
					raw := string(source[start:end])
					decoded := stdhtml.UnescapeString(string(util.UnescapePunctuations([]byte(raw))))
					active := destinations[decoded]
					if definition {
						active = decoded == stdhtml.UnescapeString(string(util.UnescapePunctuations(reference.Destination())))
					}
					i = end
					if active {
						if value := rebaseDestination(decoded, base, assets); value != decoded {
							edits = append(edits, sourceEdit{start, end, serializeDestination(value, start > 0 && source[start-1] == '<')})
						}
						i = markdownLinkEnd(source, end, definition)
					}
					continue
				}
			}
		case '<':
			tokenizer := html.NewTokenizer(bytes.NewReader(source[i:]))
			kind := tokenizer.Next()
			if kind == html.CommentToken {
				i += len(tokenizer.Raw())
				continue
			}
			if kind == html.StartTagToken || kind == html.SelfClosingTagToken {
				raw := tokenizer.Raw()
				token := tokenizer.Token()
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
				if changed {
					edits = append(edits, sourceEdit{i, i + len(raw), token.String()})
				}
				if token.Data == "pre" || token.Data == "code" || token.Data == "script" || token.Data == "style" || token.Data == "textarea" {
					i += rawElementLength(tokenizer, token.Data, len(raw))
					continue
				}
				i += len(raw)
				continue
			}
		}
		i++
	}
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
	for start < len(source) && (source[start] == ' ' || source[start] == '\n' || source[start] == '\r' || source[start] == '\t') {
		start++
	}
	if start >= len(source) {
		return 0, 0, false
	}
	if source[start] == '<' {
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
		marker := 0
		if strings.ContainsRune("-+*", rune(prefix[0])) {
			marker = 1
		} else {
			for marker < len(prefix) && marker < 9 && prefix[marker] >= '0' && prefix[marker] <= '9' {
				marker++
			}
			if marker == 0 || marker >= len(prefix) || (prefix[marker] != '.' && prefix[marker] != ')') {
				return false
			}
			marker++
		}
		if marker >= len(prefix) || (prefix[marker] != ' ' && prefix[marker] != '\t') {
			return false
		}
		prefix = prefix[marker+1:]
	}
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
	lines := 0
	for cursor < len(source) && strings.ContainsRune(" \t\r\n", rune(source[cursor])) {
		if source[cursor] == '\n' {
			lines++
		}
		cursor++
	}
	if lines > 1 || cursor >= len(source) {
		return end
	}
	if !definition && source[cursor] == ')' {
		return cursor + 1
	}
	if gap == cursor || !strings.ContainsRune("\"'(", rune(source[cursor])) {
		return end
	}
	closing := source[cursor]
	if closing == '(' {
		closing = ')'
	}
	for cursor++; cursor < len(source); cursor++ {
		if source[cursor] == '\\' {
			cursor++
			continue
		}
		if source[cursor] != closing {
			continue
		}
		after := cursor + 1
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
		return end
	}
	return end
}
