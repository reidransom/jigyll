package docexport

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"net/url"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
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
	doc := goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser().Parse(text.NewReader(source))
	destinations := map[string]bool{}
	protected := []sourceRange{}
	err = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Link:
			destinations[stdhtml.UnescapeString(string(util.UnescapePunctuations(n.Destination)))] = true
		case *ast.Image:
			destinations[stdhtml.UnescapeString(string(util.UnescapePunctuations(n.Destination)))] = true
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
	edits := []sourceEdit{}
	protection := 0
	brackets := 0
	for i := 0; i < len(source); {
		for protection < len(protected) && protected[protection].end <= i {
			protection++
		}
		if protection < len(protected) && i >= protected[protection].start {
			i = protected[protection].end
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
				if definition {
					line := bytes.LastIndexByte(source[:i], '\n') + 1
					prefix := source[line:i]
					opening := bytes.IndexByte(prefix, '[')
					if opening < 0 || opening > 3 || strings.TrimSpace(string(prefix[:opening])) != "" {
						i++
						continue
					}
				}
				start, end, ok := markdownDestination(source, i+2)
				if ok {
					raw := string(source[start:end])
					decoded := stdhtml.UnescapeString(string(util.UnescapePunctuations([]byte(raw))))
					if definition || destinations[decoded] {
						if value := rebaseDestination(decoded, base, assets); value != decoded {
							edits = append(edits, sourceEdit{start, end, value})
						}
					}
					i = end
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
				// Text inside these elements is not navigation or media markup.
				if token.Data == "pre" || token.Data == "code" || token.Data == "script" || token.Data == "style" || token.Data == "textarea" {
					closing := []byte("</" + token.Data + ">")
					if end := closingElement(source, i+len(raw), closing); end >= 0 {
						i = end
						continue
					}
				}
				changed := false
				for a := range token.Attr {
					attribute := &token.Attr[a]
					value := attribute.Val
					switch attribute.Key {
					case "href", "src", "poster", "action":
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

func closingElement(source []byte, start int, closing []byte) int {
	for start+len(closing) <= len(source) {
		offset := bytes.IndexByte(source[start:], '<')
		if offset < 0 {
			break
		}
		start += offset
		if start+len(closing) <= len(source) && bytes.EqualFold(source[start:start+len(closing)], closing) {
			return start + len(closing)
		}
		start++
	}
	return -1
}
