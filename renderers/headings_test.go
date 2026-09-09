package renderers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractHeadingsPreservesHTMLValues(t *testing.T) {
	headings, err := ExtractHeadings(`<h1 id="a&amp;b">  Title &amp; <em>emphasis</em>  </h1>
<h2 id="a&amp;b" class="no_toc">Duplicate <code>code</code></h2>
<h3>  Multiple
  spaces  </h3>
<h6 id="last">Last</h6>`)
	require.NoError(t, err)
	require.Equal(t, []Heading{
		{Level: 1, ID: "a&b", Text: "Title & emphasis"},
		{Level: 2, ID: "a&b", Text: "Duplicate code"},
		{Level: 3, ID: "", Text: "Multiple\n  spaces"},
		{Level: 6, ID: "last", Text: "Last"},
	}, headings)
}

func TestExtractHeadingsExcludesCodeAndTemplates(t *testing.T) {
	headings, err := ExtractHeadings(`<pre><h2 id="sample">Sample</h2></pre>
<code><h3 id="code">Code</h3></code>
<template><h2 id="template">Template</h2></template>
<script>"<h2 id='script'>Script</h2>"</script>
<style>/* <h2 id='style'>Style</h2> */</style>
<!-- <h2 id="comment">Comment</h2> -->
<h7 id="not-heading">Not a heading</h7>
<h2 id="article">Article <code>code</code></h2>`)
	require.NoError(t, err)
	require.Equal(t, []Heading{{Level: 2, ID: "article", Text: "Article code"}}, headings)
}
