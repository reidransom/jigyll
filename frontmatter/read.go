package frontmatter

import (
	"bytes"
	"regexp"
	"time"

	"github.com/reidransom/jigyll/utils"

	yaml "gopkg.in/yaml.v2"
)

// The first four bytes of a file with front matter.
const fmMagic = "---\n"

var frontMatterMatcher = regexp.MustCompile(`(?s)^---\n(.+?\n)---\n*`)
var emptyFontMatterMatcher = regexp.MustCompile(`(?s)^---\n+---\n*`)

// FileHasFrontMatter returns a bool indicating whether the
// file looks like it has frontmatter.
func FileHasFrontMatter(filename string) (bool, error) {
	magic, err := utils.ReadFileMagic(filename)
	if err != nil {
		return false, err
	}
	return string(magic) == fmMagic, nil
}

// Read reads the frontmatter from a document. It modifies srcPtr to point to the
// content after the frontmatter, and sets firstLine to its 1-indexed line number.
func Read(sourcePtr *[]byte, firstLine *int) (fm FrontMatter, err error) {
	var (
		source = *sourcePtr
		start  = 0
	)
	// Replace Windows line feeds. This allows the following regular expressions to work.
	source = bytes.ReplaceAll(source, []byte("\r\n"), []byte("\n"))
	if match := frontMatterMatcher.FindSubmatchIndex(source); match != nil {
		start = match[1]
		if err = yaml.Unmarshal(source[match[2]:match[3]], &fm); err != nil {
			return
		}
		// Convert date strings to time.Time
		convertDates(fm)
	} else if match := emptyFontMatterMatcher.FindSubmatchIndex(source); match != nil {
		start = match[1]
	}
	if firstLine != nil {
		*firstLine = 1 + bytes.Count(source[:start], []byte("\n"))
	}
	*sourcePtr = source[start:]
	return
}

// convertDates recursively converts date strings to time.Time in front matter
func convertDates(m map[string]interface{}) {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			// Try to parse common date formats
			if t, err := parseDate(val); err == nil {
				m[k] = t
			}
		case map[string]interface{}:
			convertDates(val)
		case []interface{}:
			for i, item := range val {
				if itemMap, ok := item.(map[string]interface{}); ok {
					convertDates(itemMap)
				} else if itemStr, ok := item.(string); ok {
					if t, err := parseDate(itemStr); err == nil {
						val[i] = t
					}
				}
			}
		}
	}
}

// parseDate attempts to parse a string as a date using common formats
func parseDate(s string) (time.Time, error) {
	// Common date formats used in Jekyll
	formats := []string{
		time.RFC3339,                     // 2006-01-02T15:04:05Z07:00
		"2006-01-02 15:04:05-07:00",       // 2025-01-01 01:00:00+00:00
		"2006-01-02 15:04:05 -07:00",      // With space before timezone
		"2006-01-02T15:04:05",             // Without timezone
		"2006-01-02 15:04:05",             // Without timezone, space separator
		"2006-01-02",                      // Date only
	}
	
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	
	return time.Time{}, &time.ParseError{}
}
