package renderers

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
)

const (
	codeFrameAttribute = "jigyll_code_frame"
	codeTitleAttribute = "jigyll_code_title"
)

var fencedCodeOpenerRE = regexp.MustCompile("^( {0,3})(`{3,}|~{3,})(.*)$")

type codeFenceMetadata struct {
	frame string
	title string
}

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

		rewritten, recognized, err := rewriteCodeFenceInfo(string(info))
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

func rewriteCodeFenceInfo(info string) ([]byte, bool, error) {
	leadingLength := len(info) - len(strings.TrimLeft(info, " \t"))
	leading := info[:leadingLength]
	trimmed := info[leadingLength:]
	if trimmed == "" {
		return []byte(info), false, nil
	}

	languageEnd := strings.IndexAny(trimmed, " \t")
	if languageEnd == -1 {
		return []byte(info), false, nil
	}
	language := trimmed[:languageEnd]
	rest := trimmed[languageEnd:]
	metadata, unknown, recognized, err := parseCodeFenceMetadata(rest)
	if err != nil || !recognized {
		return []byte(info), recognized, err
	}

	if metadata.title != "" && metadata.frame == "" {
		metadata.frame = inferredCodeFrame(language)
	}
	if metadata.title != "" && metadata.frame == "none" {
		return nil, true, fmt.Errorf(`frame="none" cannot be combined with title`)
	}

	unknown = strings.TrimSpace(unknown)
	if metadata.frame != "none" {
		attributes := codeFrameAttribute + `="` + metadata.frame + `"`
		if metadata.title != "" {
			encodedTitle := base64.RawURLEncoding.EncodeToString([]byte(metadata.title))
			attributes += " " + codeTitleAttribute + `="` + encodedTitle + `"`
		}
		unknown = appendCodeFrameAttributes(unknown, attributes)
	}

	rewritten := leading + language
	if unknown != "" {
		rewritten += " " + unknown
	}
	return []byte(rewritten), true, nil
}

func parseCodeFenceMetadata(input string) (codeFenceMetadata, string, bool, error) {
	var metadata codeFenceMetadata
	var unknown []string
	recognized := false
	titleSeen := false
	frameSeen := false

	for position := 0; position < len(input); {
		for position < len(input) && (input[position] == ' ' || input[position] == '\t') {
			position++
		}
		if position == len(input) {
			break
		}

		name := ""
		switch {
		case hasMetadataName(input[position:], "title"):
			name = "title"
		case hasMetadataName(input[position:], "frame"):
			name = "frame"
		}
		if name == "" {
			start := position
			if input[position] == '{' {
				if end := strings.IndexByte(input[position:], '}'); end >= 0 {
					position += end + 1
				} else {
					position = len(input)
				}
			} else {
				for position < len(input) && input[position] != ' ' && input[position] != '\t' {
					position++
				}
			}
			unknown = append(unknown, input[start:position])
			continue
		}

		recognized = true
		if name == "title" {
			if titleSeen {
				return metadata, "", true, fmt.Errorf("duplicate title metadata")
			}
			titleSeen = true
		} else {
			if frameSeen {
				return metadata, "", true, fmt.Errorf("duplicate frame metadata")
			}
			frameSeen = true
		}

		position += len(name)
		if position >= len(input) || input[position] != '=' || position+1 >= len(input) || input[position+1] != '"' {
			return metadata, "", true, fmt.Errorf("%s must use a double-quoted value", name)
		}
		position += 2
		value, next, err := parseCodeMetadataValue(input, position, name)
		if err != nil {
			return metadata, "", true, err
		}
		position = next
		if position < len(input) && input[position] != ' ' && input[position] != '\t' {
			return metadata, "", true, fmt.Errorf("%s must be followed by whitespace", name)
		}

		if name == "title" {
			if value == "" {
				return metadata, "", true, fmt.Errorf("title must not be empty")
			}
			metadata.title = value
			continue
		}
		if value != "editor" && value != "terminal" && value != "none" {
			return metadata, "", true, fmt.Errorf(`frame must be one of "editor", "terminal", or "none"`)
		}
		metadata.frame = value
	}

	return metadata, strings.Join(unknown, " "), recognized, nil
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

func appendCodeFrameAttributes(unknown, attributes string) string {
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
