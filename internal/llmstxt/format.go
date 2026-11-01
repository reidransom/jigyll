// Package llmstxt formats discovery metadata independently of article selection
// and documentation snapshots. Callers validate metadata and choose destinations.
package llmstxt

import (
	"fmt"
	"strings"
)

// Project supplies the title and optional summary of a discovery document.
type Project struct {
	Title       string
	Description string
}

// Section is an ordered file list with an optional description.
type Section struct {
	Title       string
	Description string
	Entries     []Entry
}

// Entry describes an already selected destination; formatting never fetches it.
type Entry struct {
	Title       string
	URL         string
	Description string
}

var textEscaper = strings.NewReplacer("\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "{", "\\{", "}", "\\}", "[", "\\[", "]", "\\]", "<", "&lt;", ">", "&gt;", "#", "\\#", "!", "\\!", "|", "\\|")
var destinationEscaper = strings.NewReplacer("<", "%3C", ">", "%3E", "\n", "%0A", "\r", "%0D")

// text escapes Markdown syntax and collapses line breaks so metadata cannot
// introduce new list items, headings, links, or raw HTML.
func text(value string) string {
	return textEscaper.Replace(strings.Join(strings.Fields(value), " "))
}

func destination(value string) string {
	return "<" + destinationEscaper.Replace(value) + ">"
}

// WriteHeader writes one H1 and an optional summary blockquote. It is separate
// from WriteSections so callers can insert their own document preamble.
func WriteHeader(out *strings.Builder, project Project) {
	fmt.Fprintf(out, "# %s\n", text(project.Title))
	if project.Description != "" {
		fmt.Fprintf(out, "\n> %s\n", text(project.Description))
	}
}

// WriteSections appends H2 file lists, preserving section and entry order.
func WriteSections(out *strings.Builder, sections []Section) {
	for _, section := range sections {
		fmt.Fprintf(out, "\n## %s\n", text(section.Title))
		if section.Description != "" {
			fmt.Fprintf(out, "\n%s\n", text(section.Description))
		}
		out.WriteByte('\n')
		for _, entry := range section.Entries {
			fmt.Fprintf(out, "- [%s](%s)", text(entry.Title), destination(entry.URL))
			if entry.Description != "" {
				fmt.Fprintf(out, ": %s", text(entry.Description))
			}
			out.WriteByte('\n')
		}
	}
}
