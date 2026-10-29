package docexport

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Sections extracts disjoint search records and ordered heading metadata from an
// article's rendered body, never from its layout or unresolved Markdown source.
// A heading's fragment must be a unique, existing HTML target.
func Sections(article Article, bodyHTML string) ([]Section, []Heading, error) {
	nodes, err := html.ParseFragment(strings.NewReader(bodyHTML), &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"})
	if err != nil {
		return nil, nil, fmt.Errorf("parse article %q: %w", article.ID, err)
	}
	ids := countElementIDs(nodes)
	base, err := url.Parse(article.CanonicalURL)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid canonical article URL %q: %w", article.CanonicalURL, err)
	}
	records := []Section{}
	headings := []Heading{}
	var hierarchy []Heading
	var current *Heading
	var text sectionText
	flush := func() {
		body := text.String()
		if current == nil && strings.TrimSpace(body) == "" {
			text = sectionText{}
			return
		}
		records = append(records, buildSectionRecord(article, base, current, hierarchy, body))
		text = sectionText{}
	}
	onHeading := func(n *html.Node) error {
		heading, err := parseHeadingNode(article.ID, n, ids)
		if err != nil {
			return err
		}
		flush()
		for len(hierarchy) > 0 && hierarchy[len(hierarchy)-1].Level >= heading.Level {
			hierarchy = hierarchy[:len(hierarchy)-1]
		}
		hierarchy = append(hierarchy, heading)
		headings = append(headings, heading)
		current = &headings[len(headings)-1]
		return nil
	}
	for _, n := range nodes {
		if err := walkSectionText(n, false, &text, onHeading); err != nil {
			return nil, nil, err
		}
	}
	flush()
	return records, headings, nil
}

// Count all targets, including ignored chrome: those IDs still participate
// in browser fragment resolution and can make an otherwise valid heading ambiguous.
func countElementIDs(nodes []*html.Node) map[string]int {
	ids := make(map[string]int)
	var countIDs func(*html.Node)
	countIDs = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if id := sectionAttribute(n, "id"); id != "" {
				ids[id]++
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			countIDs(child)
		}
	}
	for _, n := range nodes {
		countIDs(n)
	}
	return ids
}

func buildSectionRecord(article Article, base *url.URL, current *Heading, hierarchy []Heading, body string) Section {
	record := Section{
		ArticleID: article.ID, Title: article.Title, Description: article.Description,
		NavSection: article.NavSection, MarkdownURL: article.MarkdownURL,
		Text: body, CanonicalURL: article.CanonicalURL, ID: article.ID, Hierarchy: []Heading{},
	}
	if current != nil {
		record.ID = article.ID + "#" + current.Fragment
		record.Heading = current.Text
		record.Fragment = current.Fragment
		record.Hierarchy = append([]Heading(nil), hierarchy...)
		target := *base
		target.Fragment = current.Fragment
		target.RawFragment = ""
		record.CanonicalURL = target.String()
	}
	return record
}

func parseHeadingNode(articleID string, n *html.Node, ids map[string]int) (Heading, error) {
	fragment := sectionAttribute(n, "id")
	if fragment == "" {
		return Heading{}, fmt.Errorf("article %q: %s heading has no fragment ID", articleID, n.Data)
	}
	if ids[fragment] != 1 {
		return Heading{}, fmt.Errorf("article %q: heading fragment %q is not unique", articleID, fragment)
	}
	var title sectionText
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if err := walkSectionText(child, false, &title, nil); err != nil {
			return Heading{}, err
		}
	}
	return Heading{Level: int(n.Data[1] - '0'), Text: title.String(), Fragment: fragment}, nil
}

func sectionAttribute(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func ignoredSectionElement(n *html.Node) bool {
	switch n.Data {
	case "script", "style", "template", "button", "input", "select", "textarea", "nav", "form":
		return true
	}
	for _, attr := range n.Attr {
		if attr.Key == "hidden" || (attr.Key == "aria-hidden" && attr.Val == "true") {
			return true
		}
		if attr.Key == "class" {
			for _, class := range strings.Fields(attr.Val) {
				// Chroma's legacy numbered blocks render gutter text as HTML.
				if class == "lnt" || class == "ln" {
					return true
				}
			}
		}
	}
	return false
}

func walkSectionText(n *html.Node, code bool, text *sectionText, heading func(*html.Node) error) error {
	if n.Type == html.TextNode {
		text.write(n.Data, code)
		return nil
	}
	if n.Type == html.ElementNode {
		if ignoredSectionElement(n) {
			return nil
		}
		if !code && isHeadingElement(n.Data) && heading != nil {
			return heading(n)
		}
	}
	separator, childCode := elementSectionMode(n, code)
	text.separate(separator)
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if err := walkSectionText(child, childCode, text, heading); err != nil {
			return err
		}
	}
	text.separate(separator)
	return nil
}

func isHeadingElement(tag string) bool {
	return len(tag) == 2 && tag[0] == 'h' && tag[1] >= '1' && tag[1] <= '6'
}

func elementSectionMode(n *html.Node, code bool) (byte, bool) {
	if n.Type != html.ElementNode {
		return 0, code
	}
	var separator byte
	switch n.Data {
	case "td", "th":
		separator = '\t'
	case "address", "article", "aside", "blockquote", "br", "dd", "div", "dl", "dt", "figcaption", "figure", "footer", "header", "hr", "li", "main", "ol", "p", "pre", "section", "table", "tr", "ul":
		separator = '\n'
	}
	if n.Data == "pre" || n.Data == "code" {
		code = true
	}
	return separator, code
}

// sectionText collapses prose whitespace without merging tokens across block or
// table boundaries. Code bytes are copied verbatim, including indentation and
// newlines across syntax-highlighter spans; pending separators are never emitted
// at the end of a record.
type sectionText struct {
	strings.Builder
	pending byte
	last    byte
}

func (t *sectionText) separate(separator byte) {
	if separator == 0 || t.Len() == 0 {
		return
	}
	if t.pending == '\n' || t.pending == '\t' && separator == ' ' {
		return
	}
	t.pending = separator
}

func (t *sectionText) emitPending() {
	if t.pending != 0 {
		if t.Len() > 0 && t.last != '\n' && (t.pending != ' ' || (t.last != ' ' && t.last != '\t')) {
			t.WriteByte(t.pending)
			t.last = t.pending
		}
		t.pending = 0
	}
}

func (t *sectionText) write(value string, code bool) {
	if code {
		if value != "" {
			t.emitPending()
			t.WriteString(value)
			t.last = value[len(value)-1]
		}
		return
	}
	start := 0
	for pos, r := range value {
		if !unicode.IsSpace(r) {
			continue
		}
		if pos > start {
			t.emitPending()
			t.WriteString(value[start:pos])
			t.last = value[pos-1]
		}
		t.separate(' ')
		start = pos + utf8.RuneLen(r)
	}
	if start < len(value) {
		t.emitPending()
		t.WriteString(value[start:])
		t.last = value[len(value)-1]
	}
}
