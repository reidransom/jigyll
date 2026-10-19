package docexport

import (
	"net/url"
	"slices"
	"strings"
	"testing"
)

func sectionTestArticle() Article {
	return Article{
		ID: "guide", Title: "Guide", Description: "A guide", NavSection: "Reference",
		CanonicalURL: "https://example.com/docs/guide/?view=full",
		MarkdownURL:  "https://example.com/docs/guide/index.md",
	}
}

func TestSectionsIntroHierarchyAndNonoverlap(t *testing.T) {
	article := sectionTestArticle()
	records, headings, err := Sections(article, `<p>Intro <strong>only</strong>.</p>
<h2 id="parent">Parent <code>API</code></h2><p>Parent body.</p>
<h4 id="child">Child</h4><p>Child body.</p>
<h3 id="sibling">Sibling</h3><p>Sibling body.</p>
<h1 id="reset">Reset</h1><h6 id="empty">Empty</h6>`)
	if err != nil {
		t.Fatal(err)
	}
	wantHeadings := []Heading{
		{Level: 2, Text: "Parent API", Fragment: "parent"},
		{Level: 4, Text: "Child", Fragment: "child"},
		{Level: 3, Text: "Sibling", Fragment: "sibling"},
		{Level: 1, Text: "Reset", Fragment: "reset"},
		{Level: 6, Text: "Empty", Fragment: "empty"},
	}
	if !slices.Equal(headings, wantHeadings) {
		t.Fatalf("headings = %#v, want %#v", headings, wantHeadings)
	}
	if len(records) != 6 {
		t.Fatalf("got %d records, want 6", len(records))
	}
	wantText := []string{"Intro only.", "Parent body.", "Child body.", "Sibling body.", "", ""}
	wantHierarchy := [][]Heading{nil, {headings[0]}, {headings[0], headings[1]}, {headings[0], headings[2]}, {headings[3]}, {headings[3], headings[4]}}
	for i, record := range records {
		if record.Text != wantText[i] || !slices.Equal(record.Hierarchy, wantHierarchy[i]) {
			t.Errorf("record %d: text %q hierarchy %#v", i, record.Text, record.Hierarchy)
		}
		if i == 0 {
			if record.Fragment != "" || record.Heading != "" || record.CanonicalURL != article.CanonicalURL {
				t.Errorf("intro metadata = %#v", record)
			}
			continue
		}
		heading := headings[i-1]
		if record.ID != "guide#"+heading.Fragment || record.Fragment != heading.Fragment || record.Heading != heading.Text || record.CanonicalURL != article.CanonicalURL+"#"+heading.Fragment {
			t.Errorf("heading metadata = %#v", record)
		}
	}
}

func TestSectionsRejectMissingAndAmbiguousFragments(t *testing.T) {
	for _, tc := range []struct {
		name, body string
	}{
		{"missing", `<h2>Missing</h2>`},
		{"empty", `<h2 id="">Empty</h2>`},
		{"duplicate headings", `<h2 id="same">First</h2><h3 id="same">Second</h3>`},
		{"earlier element", `<div id="same"></div><h2 id="same">Heading</h2>`},
		{"later element", `<h2 id="same">Heading</h2><span id="same"></span>`},
		{"ignored element", `<h2 id="same">Heading</h2><button id="same">Copy</button>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			records, headings, err := Sections(sectionTestArticle(), tc.body)
			if err == nil || len(records) != 0 || len(headings) != 0 {
				t.Fatalf("got records %#v, headings %#v, error %v; want atomic failure", records, headings, err)
			}
		})
	}
}

func TestSectionsKeepExactEscapedFragments(t *testing.T) {
	fragment := "a b#%雪?&"
	records, headings, err := Sections(sectionTestArticle(), `<h2 id="a b#%雪?&amp;">Exact</h2>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || len(headings) != 1 {
		t.Fatalf("records %#v headings %#v", records, headings)
	}
	if records[0].Fragment != fragment || headings[0].Fragment != fragment || records[0].ID != "guide#"+fragment {
		t.Fatalf("fragment was rewritten: %#v %#v", records[0], headings[0])
	}
	target, err := url.Parse(records[0].CanonicalURL)
	if err != nil || target.Fragment != fragment || target.RawQuery != "view=full" {
		t.Fatalf("invalid fragment URL %q: %#v, %v", records[0].CanonicalURL, target, err)
	}
	if !strings.Contains(records[0].CanonicalURL, "a%20b%23%25") {
		t.Errorf("unsafe fragment URL %q", records[0].CanonicalURL)
	}
}

func TestSectionsCodeTablesAndIgnoredChrome(t *testing.T) {
	body := `<h2 id="actual">Actual<button>Copy</button></h2>
<p>Use <code>get_value()</code> now.</p><p>Next&nbsp;paragraph.</p>
<pre><code><span>if</span> ready {
	<span>get_value</span>()
}
</code></pre>
<table><tr><th>Name</th><th>Value</th></tr><tr><td>alpha</td><td><code>one_two</code></td></tr></table>
<pre><code><h3>Not a heading</h3></code></pre>
<script><h4>Script</h4></script><style>Style</style><template><h4>Template</h4></template>
<nav><h4>Navigation</h4></nav><button>Copy</button><input value="Input"><select><option>Option</option></select>
<span hidden>Hidden</span><span aria-hidden="true">Decoration</span>`
	records, headings, err := Sections(sectionTestArticle(), body)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || len(headings) != 1 || headings[0].Text != "Actual" {
		t.Fatalf("records %#v, headings %#v", records, headings)
	}
	want := "Use get_value() now.\nNext paragraph.\nif ready {\n\tget_value()\n}\nName\tValue\nalpha\tone_two\nNot a heading"
	if records[0].Text != want {
		t.Fatalf("text = %q, want %q", records[0].Text, want)
	}
}

func TestSectionsNoHeadingAndEmpty(t *testing.T) {
	for _, tc := range []struct {
		name, body, text string
		count            int
	}{
		{name: "prose", body: `<p>First</p><p>Second <em>part</em>.</p>`, text: "First\nSecond part.", count: 1},
		{name: "code indentation", body: "<pre><code>\tfirst()\n  second()</code></pre>", text: "\tfirst()\n  second()", count: 1},
		{name: "empty"},
		{name: "whitespace", body: " \n\t <p> </p> "},
		{name: "chrome only", body: `<script>ignored</script><button>Copy</button>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			records, headings, err := Sections(sectionTestArticle(), tc.body)
			if err != nil {
				t.Fatal(err)
			}
			if len(records) != tc.count || len(headings) != 0 {
				t.Fatalf("records %#v, headings %#v", records, headings)
			}
			if tc.count == 1 && (records[0].Text != tc.text || len(records[0].Hierarchy) != 0 || records[0].Fragment != "") {
				t.Fatalf("intro = %#v, want text %q", records[0], tc.text)
			}
		})
	}
}

func TestSectionsIntroIdentityDoesNotReserveAHeadingFragment(t *testing.T) {
	records, _, err := Sections(Article{ID: "guide", CanonicalURL: "https://example.com/docs/guide/"}, `Intro.<h2 id="@intro">Details</h2>Body.`)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].ID == records[1].ID {
		t.Fatalf("intro and heading records must have distinct identities: %#v", records)
	}
}
