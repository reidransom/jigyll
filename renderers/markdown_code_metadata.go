package renderers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	codeFrameAttribute   = "jigyll_code_frame"
	codeMarkersAttribute = "jigyll_code_markers"
	codeTitleAttribute   = "jigyll_code_title"
)

type codeMarkerKind string

const (
	codeMarkerNeutral  codeMarkerKind = "marked"
	codeMarkerInserted codeMarkerKind = "inserted"
	codeMarkerDeleted  codeMarkerKind = "deleted"
)

type codeLineMarker struct {
	Kind  codeMarkerKind `json:"kind"`
	Start int            `json:"start"`
	End   int            `json:"end"`
}

type codeTextMarker struct {
	Kind codeMarkerKind `json:"kind"`
	Text string         `json:"text"`
}

type codeMarkers struct {
	Lines []codeLineMarker `json:"lines,omitempty"`
	Texts []codeTextMarker `json:"texts,omitempty"`
}

func (markers codeMarkers) empty() bool {
	return len(markers.Lines) == 0 && len(markers.Texts) == 0
}

type codeFenceMetadata struct {
	frame   string
	title   string
	markers codeMarkers
}

var fencedCodeOpenerRE = regexp.MustCompile("^( {0,3})(`{3,}|~{3,})(.*)$")

// preprocessCodeFenceMetadata validates fenced-code UI metadata once at the
// Markdown seam and rewrites it into private Goldmark attributes consumed by
// the renderer. Fences without recognized metadata are returned byte-for-byte.
func preprocessCodeFenceMetadata(md []byte, firstLine int) ([]byte, error) {
	lines := bytes.Split(md, []byte("\n"))
	changed := false
	var fence byte
	var fenceLength int

	for index, line := range lines {
		if fence != 0 {
			if isCodeFenceCloser(line, fence, fenceLength) {
				fence = 0
				fenceLength = 0
			}
			continue
		}

		matches := fencedCodeOpenerRE.FindSubmatch(line)
		if matches == nil {
			continue
		}
		delimiter := matches[2]
		info := matches[3]
		if delimiter[0] == '`' && bytes.Contains(info, []byte("`")) {
			continue
		}
		fence = delimiter[0]
		fenceLength = len(delimiter)

		rewritten, metadata, recognized, err := rewriteCodeFenceInfo(string(info))
		if err == nil && !metadata.markers.empty() {
			err = validateFenceCodeMarkers(metadata.markers, lines, index, fence, fenceLength)
		}
		if err != nil {
			return nil, fmt.Errorf("code fence metadata at line %d: %w", firstLine+index, err)
		}
		if !recognized {
			continue
		}
		lines[index] = append(append(append([]byte(nil), matches[1]...), delimiter...), rewritten...)
		changed = true
	}

	if !changed {
		return md, nil
	}
	return bytes.Join(lines, []byte("\n")), nil
}
func validateFenceCodeMarkers(markers codeMarkers, lines [][]byte, opener int, fence byte, fenceLength int) error {
	contentEnd := len(lines)
	for candidate := opener + 1; candidate < len(lines); candidate++ {
		if isCodeFenceCloser(lines[candidate], fence, fenceLength) {
			contentEnd = candidate
			break
		}
	}
	if contentEnd == len(lines) && contentEnd > opener+1 && len(lines[contentEnd-1]) == 0 {
		contentEnd--
	}
	return validateCodeMarkers(markers, lines[opener+1:contentEnd])
}

func isCodeFenceCloser(line []byte, fence byte, minimumLength int) bool {
	indent := 0
	for indent < len(line) && indent < 4 && line[indent] == ' ' {
		indent++
	}
	if indent > 3 {
		return false
	}
	end := indent
	for end < len(line) && line[end] == fence {
		end++
	}
	if end-indent < minimumLength {
		return false
	}
	return len(bytes.TrimSpace(line[end:])) == 0
}

func rewriteCodeFenceInfo(info string) ([]byte, codeFenceMetadata, bool, error) {
	leadingLength := len(info) - len(strings.TrimLeft(info, " \t"))
	leading := info[:leadingLength]
	trimmed := info[leadingLength:]
	if trimmed == "" {
		return []byte(info), codeFenceMetadata{}, false, nil
	}

	languageEnd := strings.IndexAny(trimmed, " \t")
	if languageEnd == -1 {
		return []byte(info), codeFenceMetadata{}, false, nil
	}
	language := trimmed[:languageEnd]
	rest := trimmed[languageEnd:]
	metadata, unknown, recognized, err := parseCodeFenceMetadata(rest)
	if err != nil || !recognized {
		return []byte(info), metadata, recognized, err
	}

	if metadata.title != "" && metadata.frame == "" {
		metadata.frame = inferredCodeFrame(language)
	}
	if metadata.title != "" && metadata.frame == "none" {
		return nil, metadata, true, fmt.Errorf(`frame="none" cannot be combined with title`)
	}

	unknown = strings.TrimSpace(unknown)
	attributes, err := encodedCodeMetadataAttributes(metadata)
	if err != nil {
		return nil, metadata, true, err
	}
	if attributes != "" {
		unknown = appendCodeMetadataAttributes(unknown, attributes)
	}

	rewritten := leading + language
	if unknown != "" {
		rewritten += " " + unknown
	}
	return []byte(rewritten), metadata, true, nil
}
func encodedCodeMetadataAttributes(metadata codeFenceMetadata) (string, error) {
	var attributes []string
	if metadata.frame != "" && metadata.frame != "none" {
		attributes = append(attributes, codeFrameAttribute+`="`+metadata.frame+`"`)
		if metadata.title != "" {
			encodedTitle := base64.RawURLEncoding.EncodeToString([]byte(metadata.title))
			attributes = append(attributes, codeTitleAttribute+`="`+encodedTitle+`"`)
		}
	}
	if !metadata.markers.empty() {
		encodedMarkers, err := json.Marshal(metadata.markers)
		if err != nil {
			return "", fmt.Errorf("encode code markers: %w", err)
		}
		attributes = append(attributes, codeMarkersAttribute+`="`+base64.RawURLEncoding.EncodeToString(encodedMarkers)+`"`)
	}
	return strings.Join(attributes, " "), nil
}

type codeFenceMetadataParser struct {
	metadata  codeFenceMetadata
	unknown   []string
	titleSeen bool
	frameSeen bool
}

func parseCodeFenceMetadata(input string) (codeFenceMetadata, string, bool, error) {
	var parser codeFenceMetadataParser
	recognized := false
	for position := 0; position < len(input); {
		position = skipCodeMetadataWhitespace(input, position)
		if position == len(input) {
			break
		}
		next, tokenRecognized, unknown, err := parser.parseToken(input, position)
		if err != nil {
			return parser.metadata, "", recognized || tokenRecognized, err
		}
		recognized = recognized || tokenRecognized
		if unknown != "" {
			parser.unknown = append(parser.unknown, unknown)
		}
		position = next
	}
	return parser.metadata, strings.Join(parser.unknown, " "), recognized, nil
}

func (parser *codeFenceMetadataParser) parseToken(input string, position int) (int, bool, string, error) {
	if input[position] == '{' {
		markers, next, recognized, err := parseLineMarkerToken(input, position, codeMarkerNeutral, false)
		if recognized || err != nil {
			parser.metadata.markers.Lines = append(parser.metadata.markers.Lines, markers...)
			return next, true, "", err
		}
	}
	if input[position] == '"' {
		next, err := parser.parseNeutralTextMarker(input, position)
		return next, true, "", err
	}

	name := codeMetadataName(input[position:])
	if name == "" {
		token, next := unknownCodeMetadataToken(input, position)
		return next, false, token, nil
	}
	if name == "ins" || name == "del" {
		next, err := parser.parseTypedMarker(input, position, name)
		return next, true, "", err
	}
	next, err := parser.parseFrameMetadata(input, position, name)
	return next, true, "", err
}

func (parser *codeFenceMetadataParser) parseNeutralTextMarker(input string, position int) (int, error) {
	value, next, err := parseCodeMetadataValue(input, position+1, "text marker")
	if err != nil {
		return 0, err
	}
	if value == "" {
		return 0, fmt.Errorf("text marker selector must not be empty")
	}
	if err := requireCodeMetadataWhitespace(input, next, "text marker"); err != nil {
		return 0, err
	}
	parser.metadata.markers.Texts = append(parser.metadata.markers.Texts, codeTextMarker{
		Kind: codeMarkerNeutral,
		Text: value,
	})
	return next, nil
}

func (parser *codeFenceMetadataParser) parseTypedMarker(input string, position int, name string) (int, error) {
	kind := codeMarkerInserted
	if name == "del" {
		kind = codeMarkerDeleted
	}
	position += len(name)
	if position >= len(input) || input[position] != '=' || position+1 >= len(input) {
		return 0, fmt.Errorf("%s must use a line selector or double-quoted text", name)
	}
	position++
	if input[position] == '{' {
		markers, next, _, err := parseLineMarkerToken(input, position, kind, true)
		parser.metadata.markers.Lines = append(parser.metadata.markers.Lines, markers...)
		return next, err
	}
	if input[position] != '"' {
		return 0, fmt.Errorf("%s must use a line selector or double-quoted text", name)
	}

	value, next, err := parseCodeMetadataValue(input, position+1, name)
	if err != nil {
		return 0, err
	}
	if value == "" {
		return 0, fmt.Errorf("text marker selector must not be empty")
	}
	if err := requireCodeMetadataWhitespace(input, next, name); err != nil {
		return 0, err
	}
	parser.metadata.markers.Texts = append(parser.metadata.markers.Texts, codeTextMarker{Kind: kind, Text: value})
	return next, nil
}

func (parser *codeFenceMetadataParser) parseFrameMetadata(input string, position int, name string) (int, error) {
	if err := parser.recordUniqueFrameMetadata(name); err != nil {
		return 0, err
	}
	position += len(name)
	if position >= len(input) || input[position] != '=' || position+1 >= len(input) || input[position+1] != '"' {
		return 0, fmt.Errorf("%s must use a double-quoted value", name)
	}
	value, next, err := parseCodeMetadataValue(input, position+2, name)
	if err != nil {
		return 0, err
	}
	if err := requireCodeMetadataWhitespace(input, next, name); err != nil {
		return 0, err
	}
	if name == "title" {
		if value == "" {
			return 0, fmt.Errorf("title must not be empty")
		}
		parser.metadata.title = value
		return next, nil
	}
	if value != "editor" && value != "terminal" && value != "none" {
		return 0, fmt.Errorf(`frame must be one of "editor", "terminal", or "none"`)
	}
	parser.metadata.frame = value
	return next, nil
}

func (parser *codeFenceMetadataParser) recordUniqueFrameMetadata(name string) error {
	if name == "title" {
		if parser.titleSeen {
			return fmt.Errorf("duplicate title metadata")
		}
		parser.titleSeen = true
		return nil
	}
	if parser.frameSeen {
		return fmt.Errorf("duplicate frame metadata")
	}
	parser.frameSeen = true
	return nil
}

func codeMetadataName(input string) string {
	for _, name := range []string{"title", "frame", "ins", "del"} {
		if hasMetadataName(input, name) {
			return name
		}
	}
	return ""
}

func skipCodeMetadataWhitespace(input string, position int) int {
	for position < len(input) && (input[position] == ' ' || input[position] == '\t') {
		position++
	}
	return position
}

func requireCodeMetadataWhitespace(input string, position int, name string) error {
	if position < len(input) && input[position] != ' ' && input[position] != '\t' {
		return fmt.Errorf("%s must be followed by whitespace", name)
	}
	return nil
}

func unknownCodeMetadataToken(input string, position int) (string, int) {
	start := position
	if input[position] == '{' {
		if end := strings.IndexByte(input[position:], '}'); end >= 0 {
			position += end + 1
			return input[start:position], position
		}
	}
	for position < len(input) && input[position] != ' ' && input[position] != '\t' {
		position++
	}
	return input[start:position], position
}

func parseLineMarkerToken(input string, position int, kind codeMarkerKind, force bool) ([]codeLineMarker, int, bool, error) {
	relativeEnd := strings.IndexByte(input[position:], '}')
	if relativeEnd < 0 {
		token, _ := unknownCodeMetadataToken(input, position)
		body := strings.TrimPrefix(token, "{")
		if force || isLineMarkerCandidate(body) {
			return nil, 0, true, fmt.Errorf("line marker selector has an unterminated value")
		}
		return nil, position, false, nil
	}
	end := position + relativeEnd
	body := input[position+1 : end]
	if !force && !isLineMarkerCandidate(body) {
		return nil, position, false, nil
	}
	next := end + 1
	if err := requireCodeMetadataWhitespace(input, next, "line marker"); err != nil {
		return nil, 0, true, err
	}
	ranges, err := parseLineMarkerSelector(body, kind)
	if err != nil {
		return nil, 0, true, err
	}
	return ranges, next, true, nil
}

func isLineMarkerCandidate(value string) bool {
	if value == "" {
		return true
	}
	for _, character := range value {
		if (character < '0' || character > '9') && character != '-' && character != ',' {
			return false
		}
	}
	return true
}

func parseLineMarkerSelector(value string, kind codeMarkerKind) ([]codeLineMarker, error) {
	if value == "" {
		return nil, fmt.Errorf("line marker selector must not be empty")
	}
	var markers []codeLineMarker
	for _, item := range strings.Split(value, ",") {
		parts := strings.Split(item, "-")
		if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && parts[1] == "") {
			return nil, fmt.Errorf("invalid line marker selector %q", value)
		}
		start, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid line marker selector %q", value)
		}
		end := start
		if len(parts) == 2 {
			end, err = strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid line marker selector %q", value)
			}
		}
		if start < 1 || end < 1 {
			return nil, fmt.Errorf("marker lines must be positive")
		}
		if end < start {
			return nil, fmt.Errorf("marker line range %d-%d is reversed", start, end)
		}
		markers = append(markers, codeLineMarker{Kind: kind, Start: start, End: end})
	}
	return markers, nil
}

func hasMetadataName(input, name string) bool {
	if !strings.HasPrefix(input, name) {
		return false
	}
	return len(input) == len(name) || input[len(name)] == '=' || input[len(name)] == ' ' || input[len(name)] == '\t'
}

func parseCodeMetadataValue(input string, position int, name string) (string, int, error) {
	var value strings.Builder
	for position < len(input) {
		switch input[position] {
		case '"':
			return value.String(), position + 1, nil
		case '\\':
			if position+1 >= len(input) {
				return "", 0, fmt.Errorf("%s has an unterminated double-quoted value", name)
			}
			next := input[position+1]
			if next != '\\' && next != '"' {
				return "", 0, fmt.Errorf("%s contains invalid escape \\%c", name, next)
			}
			value.WriteByte(next)
			position += 2
		default:
			value.WriteByte(input[position])
			position++
		}
	}
	return "", 0, fmt.Errorf("%s has an unterminated double-quoted value", name)
}

func inferredCodeFrame(language string) string {
	switch strings.ToLower(language) {
	case "bash", "sh", "shell", "console", "powershell", "ps1":
		return "terminal"
	default:
		return "editor"
	}
}

func appendCodeMetadataAttributes(unknown, attributes string) string {
	if start := strings.IndexByte(unknown, '{'); start >= 0 {
		if relativeEnd := strings.IndexByte(unknown[start:], '}'); relativeEnd >= 0 {
			end := start + relativeEnd
			return unknown[:end] + " " + attributes + unknown[end:]
		}
	}
	if unknown == "" {
		return "{" + attributes + "}"
	}
	return unknown + " {" + attributes + "}"
}
