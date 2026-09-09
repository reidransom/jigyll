package site

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/jigyll/pages"
	"github.com/reidransom/jigyll/renderers"
	"github.com/stretchr/testify/require"
)

func TestHeadingMetadataInThemeLayouts(t *testing.T) {
	root := t.TempDir()
	writeHeadingMetadataTheme(t, root)
	writeIncludeCacheSiteFile(t, root, "_includes/heading.md", "### Included `code` & label\n")
	writeIncludeCacheSiteFile(t, root, "index.md", "---\nlayout: default\nheadings: forged\n---\n"+`During={{ page.headings | jsonify }}

# Title

## Repeat

## Repeat

## Explicit *label* {: #chosen}

## Café 日本語

{% include heading.md %}

<div markdown="1">
## Repeat
</div>

## Hidden {.no_toc}

<h2>No anchor</h2>

`+"```markdown\n## Not a heading\n```\n")
	writeIncludeCacheSiteFile(t, root, "other.md", "---\nlayout: default\n---\n#### Other\n")
	writeIncludeCacheSiteFile(t, root, "empty.md", "---\nlayout: default\n---\n")
	writeIncludeCacheSiteFile(t, root, "toc.md", "---\nlayout: default\n---\n* TOC\n{:toc}\n\n## Hidden {.no_toc}\n\n### Visible\n")

	buildIncludeCacheSite(t, root, config.Flags{})
	output := readIncludeCacheSiteOutput(t, root, "index.html")
	require.Equal(t, []renderers.Heading{
		{Level: 1, ID: "title", Text: "Title"},
		{Level: 2, ID: "repeat", Text: "Repeat"},
		{Level: 2, ID: "repeat-1", Text: "Repeat"},
		{Level: 2, ID: "chosen", Text: "Explicit label"},
		{Level: 2, ID: "caf-", Text: "Café 日本語"},
		{Level: 3, ID: "included-code--label", Text: "Included code & label"},
		{Level: 2, ID: "repeat", Text: "Repeat"},
		{Level: 2, ID: "hidden", Text: "Hidden"},
		{Level: 2, ID: "", Text: "No anchor"},
	}, readHeadingMetadata(t, output))
	require.Contains(t, output, "During=[]")
	require.Contains(t, output, `<a href="#chosen">Explicit label</a>`)
	require.Contains(t, output, `<a href="#included-code--label">Included code &amp; label</a>`)
	require.NotContains(t, output, `<a href="#">`)
	require.Contains(t, output, `<h2 id="chosen">Explicit <em>label</em></h2>`)
	require.Equal(t, 2, strings.Count(output, `<h2 id="repeat">Repeat</h2>`))
	require.Equal(t, []renderers.Heading{{Level: 4, ID: "other", Text: "Other"}}, readHeadingMetadata(t, readIncludeCacheSiteOutput(t, root, "other.html")))
	require.Equal(t, []renderers.Heading{}, readHeadingMetadata(t, readIncludeCacheSiteOutput(t, root, "empty.html")))

	toc := readIncludeCacheSiteOutput(t, root, "toc.html")
	require.Equal(t, []renderers.Heading{
		{Level: 2, ID: "hidden", Text: "Hidden"},
		{Level: 3, ID: "visible", Text: "Visible"},
	}, readHeadingMetadata(t, toc))
	require.Contains(t, toc, `<ul id="markdown-toc"><li><a href="#visible">Visible</a></li></ul>`)
}

func TestHeadingMetadataRebuild(t *testing.T) {
	root := t.TempDir()
	writeHeadingMetadataTheme(t, root)
	writeIncludeCacheSiteFile(t, root, "_includes/heading.md", "## Before include\n")
	writeIncludeCacheSiteFile(t, root, "index.md", "---\nlayout: default\n---\nDuring={{ page.headings | jsonify }}\n\n## Before page\n\n{% include heading.md %}\n")
	incremental := true
	s := buildIncludeCacheSite(t, root, config.Flags{Incremental: &incremental})
	require.Equal(t, []renderers.Heading{
		{Level: 2, ID: "before-page", Text: "Before page"},
		{Level: 2, ID: "before-include", Text: "Before include"},
	}, readHeadingMetadata(t, readIncludeCacheSiteOutput(t, root, "index.html")))

	writeIncludeCacheSiteFile(t, root, "index.md", "---\nlayout: default\n---\nDuring={{ page.headings | jsonify }}\n\n### After page\n\n{% include heading.md %}\n")
	s, _, err := s.rebuild([]string{"index.md"})
	require.NoError(t, err)
	output := readIncludeCacheSiteOutput(t, root, "index.html")
	require.Contains(t, output, "During=[]")
	require.Equal(t, []renderers.Heading{
		{Level: 3, ID: "after-page", Text: "After page"},
		{Level: 2, ID: "before-include", Text: "Before include"},
	}, readHeadingMetadata(t, output))

	writeIncludeCacheSiteFile(t, root, "_includes/heading.md", "#### After include\n")
	s, _, err = s.rebuild([]string{"_includes/heading.md"})
	require.NoError(t, err)
	require.Equal(t, []renderers.Heading{
		{Level: 3, ID: "after-page", Text: "After page"},
		{Level: 4, ID: "after-include", Text: "After include"},
	}, readHeadingMetadata(t, readIncludeCacheSiteOutput(t, root, "index.html")))

	writeIncludeCacheSiteFile(t, root, "index.md", "---\nlayout: default\n---\nDuring={{ page.headings | jsonify }}\n")
	_, _, err = s.rebuild([]string{"index.md"})
	require.NoError(t, err)
	output = readIncludeCacheSiteOutput(t, root, "index.html")
	require.Contains(t, output, "During=[]")
	require.Equal(t, []renderers.Heading{}, readHeadingMetadata(t, output))
}

func TestHeadingMetadataReplacementAndClone(t *testing.T) {
	root := t.TempDir()
	writeHeadingMetadataTheme(t, root)
	writeIncludeCacheSiteFile(t, root, "index.html", "---\nlayout: default\n---\n"+`<h2 id="source">Source {{ page.url }}</h2>`)
	writeIncludeCacheSiteFile(t, root, "raw.json", "---\nlayout: default\n---\n"+`{"example":"<h2 id='not-html'>Example</h2>"}`)
	writeIncludeCacheSiteFile(t, root, "legacy.htm", "---\nlayout: default\n---\n<h3 id='legacy'>Legacy</h3>")
	s := buildIncludeCacheSite(t, root, config.Flags{})
	p := s.Routes["/"].(Page)
	p.SetContent(`<h3 id="replacement">Replacement</h3>`)
	require.Equal(t, []renderers.Heading{{Level: 3, ID: "replacement", Text: "Replacement"}}, readHeadingMetadata(t, renderRoute(t, s, "/")))

	clone := p.(interface{ Clone(string) pages.Page }).Clone("/copy/")
	var output strings.Builder
	require.NoError(t, clone.Write(&output))
	require.Equal(t, []renderers.Heading{{Level: 2, ID: "source", Text: "Source /copy/"}}, readHeadingMetadata(t, output.String()))

	p.SetContent("No headings left")
	require.Equal(t, []renderers.Heading{}, readHeadingMetadata(t, renderRoute(t, s, "/")))
	require.Equal(t, []renderers.Heading{}, readHeadingMetadata(t, readIncludeCacheSiteOutput(t, root, "raw.json")))
	require.Equal(t, []renderers.Heading{{Level: 3, ID: "legacy", Text: "Legacy"}}, readHeadingMetadata(t, readIncludeCacheSiteOutput(t, root, "legacy.htm")))
}

func writeHeadingMetadataTheme(t *testing.T, root string) {
	t.Helper()
	writeIncludeCacheSiteFile(t, root, "_config.yml", "theme: headings\nkramdown:\n  toc_levels: 3..3\ndefaults:\n  - scope:\n      path: ''\n    values:\n      headings: forged-default\n")
	writeIncludeCacheSiteFile(t, root, "_theme/headings/_layouts/default.html", "---\nlayout: shell\n---\n"+`{% include toc.html %}<article>{{ content }}</article>`)
	writeIncludeCacheSiteFile(t, root, "_theme/headings/_layouts/shell.html", `<h1 id="chrome">Chrome</h1>{{ content }}<script id="headings" type="application/json">{{ page.headings | jsonify }}</script>`)
	writeIncludeCacheSiteFile(t, root, "_theme/headings/_includes/toc.html", `<nav>{% for heading in page.headings %}{% if heading.id != '' %}{% if heading.level == 2 or heading.level == 3 %}<a href="#{{ heading.id | escape }}">{{ heading.text | escape }}</a>{% endif %}{% endif %}{% endfor %}</nav>`)
}

func readHeadingMetadata(t *testing.T, output string) []renderers.Heading {
	t.Helper()
	_, data, found := strings.Cut(output, `<script id="headings" type="application/json">`)
	require.True(t, found, "missing layout heading metadata")
	data, _, found = strings.Cut(data, "</script>")
	require.True(t, found)
	var headings []renderers.Heading
	require.NoError(t, json.Unmarshal([]byte(data), &headings))
	return headings
}
