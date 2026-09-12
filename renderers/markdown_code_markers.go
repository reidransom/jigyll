package renderers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	codeAnnotationsElementOpen  = `<jigyll-code-annotations value="`
	codeAnnotationsElementClose = `</jigyll-code-annotations>`
)

type codeMarkerSpan struct {
	Kind  codeMarkerKind
	Start int
	End   int
}

type codeMarkerLinePlan struct {
	Kind  codeMarkerKind
	Spans []codeMarkerSpan
}

func validateCodeMarkers(markers codeMarkers, lines [][]byte) error {
	_, err := resolveCodeMarkers(markers, lines)
	return err
}

func resolveCodeMarkers(markers codeMarkers, lines [][]byte) ([]codeMarkerLinePlan, error) {
	wholeKinds, err := expandCodeLineMarkers(markers.Lines, len(lines))
	if err != nil {
		return nil, err
	}
	textSpans, err := findCodeTextMarkers(markers.Texts, lines)
	if err != nil {
		return nil, err
	}

	plans := make([]codeMarkerLinePlan, len(lines))
	for line := range lines {
		plan, err := resolveCodeMarkerLine(wholeKinds[line], textSpans[line], line)
		if err != nil {
			return nil, err
		}
		plans[line] = plan
	}
	return plans, nil
}

func expandCodeLineMarkers(markers []codeLineMarker, lineCount int) ([][]codeMarkerKind, error) {
	kinds := make([][]codeMarkerKind, lineCount)
	for _, marker := range markers {
		if marker.End > lineCount {
			line := marker.End
			if marker.Start > lineCount {
				line = marker.Start
			}
			noun := "lines"
			if lineCount == 1 {
				noun = "line"
			}
			return nil, fmt.Errorf("marker line %d is out of range (code has %d %s)", line, lineCount, noun)
		}
		for line := marker.Start; line <= marker.End; line++ {
			kinds[line-1] = append(kinds[line-1], marker.Kind)
		}
	}
	return kinds, nil
}

func findCodeTextMarkers(markers []codeTextMarker, lines [][]byte) ([][]codeMarkerSpan, error) {
	spans := make([][]codeMarkerSpan, len(lines))
	for _, marker := range markers {
		found := false
		for line, source := range lines {
			matches := exactTextOffsets(source, []byte(marker.Text))
			for _, start := range matches {
				spans[line] = append(spans[line], codeMarkerSpan{
					Kind:  marker.Kind,
					Start: start,
					End:   start + len(marker.Text),
				})
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("marker text %q was not found", marker.Text)
		}
	}
	return spans, nil
}

func exactTextOffsets(source, text []byte) []int {
	var offsets []int
	for offset := 0; offset <= len(source)-len(text); {
		relative := bytes.Index(source[offset:], text)
		if relative < 0 {
			break
		}
		start := offset + relative
		offsets = append(offsets, start)
		offset = start + 1
	}
	return offsets
}

func resolveCodeMarkerLine(wholeKinds []codeMarkerKind, textSpans []codeMarkerSpan, line int) (codeMarkerLinePlan, error) {
	insertedWhole := containsMarkerKind(wholeKinds, codeMarkerInserted)
	deletedWhole := containsMarkerKind(wholeKinds, codeMarkerDeleted)
	inserted, deleted := typedCodeMarkerSpans(textSpans)
	if insertedWhole && deletedWhole ||
		insertedWhole && len(deleted) > 0 ||
		deletedWhole && len(inserted) > 0 ||
		codeMarkerSpansOverlap(inserted, deleted) {
		return codeMarkerLinePlan{}, markerOverlapError(line)
	}

	whole := winningWholeMarkerKind(wholeKinds)
	return codeMarkerLinePlan{
		Kind:  whole,
		Spans: resolveInlineMarkerSpans(textSpans, whole),
	}, nil
}

func typedCodeMarkerSpans(spans []codeMarkerSpan) ([]codeMarkerSpan, []codeMarkerSpan) {
	var inserted, deleted []codeMarkerSpan
	for _, span := range spans {
		switch span.Kind {
		case codeMarkerInserted:
			inserted = append(inserted, span)
		case codeMarkerDeleted:
			deleted = append(deleted, span)
		}
	}
	return inserted, deleted
}

func codeMarkerSpansOverlap(inserted, deleted []codeMarkerSpan) bool {
	for _, insertion := range inserted {
		for _, deletion := range deleted {
			if insertion.Start < deletion.End && deletion.Start < insertion.End {
				return true
			}
		}
	}
	return false
}

func markerOverlapError(line int) error {
	return fmt.Errorf("inserted and deleted markers overlap on line %d", line+1)
}

func containsMarkerKind(kinds []codeMarkerKind, expected codeMarkerKind) bool {
	for _, kind := range kinds {
		if kind == expected {
			return true
		}
	}
	return false
}

func winningWholeMarkerKind(kinds []codeMarkerKind) codeMarkerKind {
	if containsMarkerKind(kinds, codeMarkerInserted) {
		return codeMarkerInserted
	}
	if containsMarkerKind(kinds, codeMarkerDeleted) {
		return codeMarkerDeleted
	}
	if containsMarkerKind(kinds, codeMarkerNeutral) {
		return codeMarkerNeutral
	}
	return ""
}

func resolveInlineMarkerSpans(spans []codeMarkerSpan, whole codeMarkerKind) []codeMarkerSpan {
	if whole == codeMarkerInserted || whole == codeMarkerDeleted {
		return nil
	}
	boundaries := inlineMarkerBoundaries(spans, whole)
	var resolved []codeMarkerSpan
	for index := 0; index+1 < len(boundaries); index++ {
		start, end := boundaries[index], boundaries[index+1]
		kind := winningInlineMarkerKind(spans, whole, start, end)
		if kind != "" {
			resolved = appendInlineMarkerSpan(resolved, codeMarkerSpan{Kind: kind, Start: start, End: end})
		}
	}
	return resolved
}

func inlineMarkerBoundaries(spans []codeMarkerSpan, whole codeMarkerKind) []int {
	var boundaries []int
	for _, span := range spans {
		if whole != codeMarkerNeutral || span.Kind != codeMarkerNeutral {
			boundaries = append(boundaries, span.Start, span.End)
		}
	}
	sort.Ints(boundaries)
	return compactSortedInts(boundaries)
}

func winningInlineMarkerKind(spans []codeMarkerSpan, whole codeMarkerKind, start, end int) codeMarkerKind {
	kind := codeMarkerKind("")
	for _, span := range spans {
		suppressedNeutral := whole == codeMarkerNeutral && span.Kind == codeMarkerNeutral
		if span.Start <= start && span.End >= end && !suppressedNeutral &&
			markerKindPriority(span.Kind) > markerKindPriority(kind) {
			kind = span.Kind
		}
	}
	return kind
}

func appendInlineMarkerSpan(spans []codeMarkerSpan, next codeMarkerSpan) []codeMarkerSpan {
	if len(spans) > 0 && spans[len(spans)-1].Kind == next.Kind && spans[len(spans)-1].End == next.Start {
		spans[len(spans)-1].End = next.End
		return spans
	}
	return append(spans, next)
}

func markerKindPriority(kind codeMarkerKind) int {
	switch kind {
	case codeMarkerInserted, codeMarkerDeleted:
		return 2
	case codeMarkerNeutral:
		return 1
	default:
		return 0
	}
}

func compactSortedInts(values []int) []int {
	compacted := values[:0]
	for _, value := range values {
		if len(compacted) == 0 || compacted[len(compacted)-1] != value {
			compacted = append(compacted, value)
		}
	}
	return compacted
}

func applyCodeAnnotations(rendered []byte) ([]byte, error) {
	for {
		open := bytes.Index(rendered, []byte(codeAnnotationsElementOpen))
		if open < 0 {
			return rendered, nil
		}
		valueStart := open + len(codeAnnotationsElementOpen)
		valueEnd := bytes.Index(rendered[valueStart:], []byte(`">`))
		if valueEnd < 0 {
			return nil, fmt.Errorf("render code annotations: malformed private annotation element")
		}
		valueEnd += valueStart
		contentStart := valueEnd + len(`">`)
		contentEnd := bytes.Index(rendered[contentStart:], []byte(codeAnnotationsElementClose))
		if contentEnd < 0 {
			return nil, fmt.Errorf("render code annotations: unclosed private annotation element")
		}
		contentEnd += contentStart

		payload, err := base64.RawURLEncoding.DecodeString(string(rendered[valueStart:valueEnd]))
		if err != nil {
			return nil, fmt.Errorf("render code annotations: decode private metadata: %w", err)
		}
		var annotations codeAnnotations
		if err := json.Unmarshal(payload, &annotations); err != nil {
			return nil, fmt.Errorf("render code annotations: parse private metadata: %w", err)
		}
		annotated, err := renderCodeAnnotationFragment(rendered[contentStart:contentEnd], annotations)
		if err != nil {
			return nil, err
		}

		var next bytes.Buffer
		next.Grow(len(rendered) + len(annotated))
		next.Write(rendered[:open])
		next.Write(annotated)
		next.Write(rendered[contentEnd+len(codeAnnotationsElementClose):])
		rendered = next.Bytes()
	}
}

func renderCodeAnnotationFragment(fragment []byte, annotations codeAnnotations) ([]byte, error) {
	context := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := html.ParseFragment(bytes.NewReader(fragment), context)
	if err != nil {
		return nil, fmt.Errorf("render code annotations: parse highlighted code: %w", err)
	}
	code := firstElement(nodes, "code")
	if code == nil {
		return nil, fmt.Errorf("render code annotations: highlighted code has no code element")
	}
	lines := elementsWithClass(code, "line")
	if len(lines) == 0 {
		lines = wrapFallbackCodeLines(code)
	}

	sourceLines := make([][]byte, len(lines))
	for index, line := range lines {
		text := nodeTextContent(line)
		text = strings.TrimSuffix(text, "\n")
		sourceLines[index] = []byte(text)
	}
	plans, err := resolveCodeMarkers(annotations.Markers, sourceLines)
	if err != nil {
		return nil, fmt.Errorf("render code annotations: highlighted source differs from authored source: %w", err)
	}
	if annotations.LineNumberStart > 0 && len(lines) > 0 {
		last := annotations.LineNumberStart + len(lines) - 1
		setHTMLAttribute(code, "data-line-number-width", strconv.Itoa(len(strconv.Itoa(last))))
	}
	for index, line := range lines {
		applyCodeMarkerPlan(line, plans[index], len(nodeTextContent(line)))
		if annotations.LineNumberStart > 0 {
			setHTMLAttribute(line, "data-line-number", strconv.Itoa(annotations.LineNumberStart+index))
		}
	}

	var rendered bytes.Buffer
	for _, node := range nodes {
		if err := html.Render(&rendered, node); err != nil {
			return nil, fmt.Errorf("render code annotations: serialize highlighted code: %w", err)
		}
	}
	return rendered.Bytes(), nil
}

func firstElement(nodes []*html.Node, tag string) *html.Node {
	var found *html.Node
	for _, root := range nodes {
		walkHTMLNodes(root, func(node *html.Node) {
			if found == nil && node.Type == html.ElementNode && node.Data == tag {
				found = node
			}
		})
	}
	return found
}

func elementsWithClass(root *html.Node, class string) []*html.Node {
	var elements []*html.Node
	walkHTMLNodes(root, func(node *html.Node) {
		if node.Type == html.ElementNode && markerHasHTMLClass(node, class) {
			elements = append(elements, node)
		}
	})
	return elements
}

func walkHTMLNodes(node *html.Node, visit func(*html.Node)) {
	visit(node)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkHTMLNodes(child, visit)
	}
}

func markerHasHTMLClass(node *html.Node, expected string) bool {
	for _, attribute := range node.Attr {
		if attribute.Key == "class" {
			for _, class := range strings.Fields(attribute.Val) {
				if class == expected {
					return true
				}
			}
		}
	}
	return false
}

func appendHTMLClass(node *html.Node, class string) {
	for index := range node.Attr {
		if node.Attr[index].Key == "class" {
			if !markerStringSliceContains(strings.Fields(node.Attr[index].Val), class) {
				node.Attr[index].Val += " " + class
			}
			return
		}
	}
	node.Attr = append(node.Attr, html.Attribute{Key: "class", Val: class})
}

func markerStringSliceContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func setHTMLAttribute(node *html.Node, name, value string) {
	for index := range node.Attr {
		if node.Attr[index].Key == name {
			node.Attr[index].Val = value
			return
		}
	}
	node.Attr = append(node.Attr, html.Attribute{Key: name, Val: value})
}

func wrapFallbackCodeLines(code *html.Node) []*html.Node {
	text := nodeTextContent(code)
	for child := code.FirstChild; child != nil; {
		next := child.NextSibling
		code.RemoveChild(child)
		child = next
	}
	parts := strings.SplitAfter(text, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	lines := make([]*html.Node, 0, len(parts))
	for _, part := range parts {
		line := &html.Node{Type: html.ElementNode, Data: "span", DataAtom: atom.Span}
		line.Attr = []html.Attribute{{Key: "class", Val: "line"}}
		line.AppendChild(&html.Node{Type: html.TextNode, Data: part})
		code.AppendChild(line)
		lines = append(lines, line)
	}
	return lines
}

func applyCodeMarkerPlan(line *html.Node, plan codeMarkerLinePlan, contentLength int) {
	appendHTMLClass(line, "code-line")
	if plan.Kind != "" {
		appendHTMLClass(line, "is-"+string(plan.Kind))
		setHTMLAttribute(line, "aria-description", markerAriaDescription(plan.Kind, "line"))
	}
	if len(plan.Spans) == 0 {
		return
	}

	var replacements []*html.Node
	position := 0
	for _, span := range plan.Spans {
		replacements = append(replacements, cloneChildrenInRange(line, position, span.Start)...)
		marker := &html.Node{Type: html.ElementNode, Data: "mark", DataAtom: atom.Mark}
		marker.Attr = []html.Attribute{{Key: "class", Val: "is-" + string(span.Kind)}}
		for _, child := range cloneChildrenInRange(line, span.Start, span.End) {
			marker.AppendChild(child)
		}
		setHTMLAttribute(marker, "aria-description", markerAriaDescription(span.Kind, "text"))
		replacements = append(replacements, marker)
		position = span.End
	}
	replacements = append(replacements, cloneChildrenInRange(line, position, contentLength)...)

	for child := line.FirstChild; child != nil; {
		next := child.NextSibling
		line.RemoveChild(child)
		child = next
	}
	for _, child := range replacements {
		line.AppendChild(child)
	}
}

func markerAriaDescription(kind codeMarkerKind, noun string) string {
	action := "Highlighted"
	switch kind {
	case codeMarkerInserted:
		action = "Inserted"
	case codeMarkerDeleted:
		action = "Deleted"
	}
	return action + " " + noun
}

func cloneChildrenInRange(parent *html.Node, start, end int) []*html.Node {
	offset := 0
	var clones []*html.Node
	for child := parent.FirstChild; child != nil; child = child.NextSibling {
		if clone := cloneNodeInRange(child, start, end, &offset); clone != nil {
			clones = append(clones, clone)
		}
	}
	return clones
}

func cloneNodeInRange(node *html.Node, start, end int, offset *int) *html.Node {
	if node.Type == html.TextNode {
		nodeStart := *offset
		nodeEnd := nodeStart + len(node.Data)
		*offset = nodeEnd
		overlapStart := max(start, nodeStart)
		overlapEnd := min(end, nodeEnd)
		if overlapStart >= overlapEnd {
			return nil
		}
		return &html.Node{Type: html.TextNode, Data: node.Data[overlapStart-nodeStart : overlapEnd-nodeStart]}
	}

	clone := &html.Node{
		Type:      node.Type,
		DataAtom:  node.DataAtom,
		Data:      node.Data,
		Namespace: node.Namespace,
		Attr:      append([]html.Attribute(nil), node.Attr...),
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if childClone := cloneNodeInRange(child, start, end, offset); childClone != nil {
			clone.AppendChild(childClone)
		}
	}
	if clone.FirstChild == nil {
		return nil
	}
	return clone
}

func nodeTextContent(node *html.Node) string {
	var text strings.Builder
	walkHTMLNodes(node, func(node *html.Node) {
		if node.Type == html.TextNode {
			text.WriteString(node.Data)
		}
	})
	return text.String()
}
