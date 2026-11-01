package plugins

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/jigyll/docexport"
	"github.com/reidransom/jigyll/pages"
	"github.com/stretchr/testify/require"
)

// Keep source provenance explicit: a plugin-created page with a plausible filename
// must not become an automatic source export merely by implementing Page.
type llmsTestPage struct {
	mockPage
	source       string
	backed       bool
	static       bool
	unpublished  bool
	ext          string
	date         time.Time
	body         []byte
	articleErr   error
	captured     bool
	articleCalls int
}

func (p *llmsTestPage) Source() string      { return p.source }
func (p *llmsTestPage) SourceBacked() bool  { return p.backed }
func (p *llmsTestPage) IsStatic() bool      { return p.static }
func (p *llmsTestPage) Published() bool     { return !p.unpublished }
func (p *llmsTestPage) OutputExt() string   { return p.ext }
func (p *llmsTestPage) PostDate() time.Time { return p.date }
func (p *llmsTestPage) CaptureSource()      { p.captured = true }
func (p *llmsTestPage) Article() (pages.Article, error) {
	p.articleCalls++
	if p.articleErr != nil {
		return pages.Article{}, p.articleErr
	}
	article := pages.Article{HTML: "<p>Rendered output is not the Markdown source.</p>"}
	if p.captured {
		article.Source = p.body
	}
	return article, nil
}

// Embedding the interface deliberately does not expose SourceBacked.
type llmsPageWithoutProvenance struct{ Page }

func newLLMsTestPage(route, source, title string) *llmsTestPage {
	return &llmsTestPage{
		mockPage: mockPage{url: route, fm: pages.FrontMatter{"title": title}},
		source:   filepath.Join("/source", source),
		backed:   true,
		ext:      ".html",
		body:     []byte("# Article\n\nBody from resolved source.\n"),
	}
}

func newLLMsTestConfig() *config.Config {
	cfg := config.Default()
	cfg.Source = "/source"
	cfg.AbsoluteURL = "https://example.test"
	cfg.Set("title", "Test project")
	return &cfg
}

func newLLMsTestPlugin(t *testing.T, cfg *config.Config) ArtifactPreparer {
	t.Helper()
	factory, found := Lookup("jigyll-llms-txt")
	require.True(t, found)
	p := factory()
	require.NoError(t, p.AfterInitSite(&mockSite{cfg: cfg}))
	preparer, ok := p.(ArtifactPreparer)
	require.True(t, ok, "the llms plugin must implement the optional artifact hook")
	return preparer
}

func buildLLMsTestPlan(t *testing.T, plan *ArtifactPlan) map[string][]byte {
	t.Helper()
	require.NotNil(t, plan)
	require.NotNil(t, plan.Build)
	for _, page := range plan.Capture {
		capture, ok := page.(interface{ CaptureSource() })
		require.True(t, ok)
		capture.CaptureSource()
	}
	artifacts, err := plan.Build()
	require.NoError(t, err)
	var routes []string
	for route := range artifacts {
		routes = append(routes, route)
	}
	require.ElementsMatch(t, plan.Routes, routes, "Build must honor its declared output routes")
	return artifacts
}

func llmsTestSections(entries ...interface{}) []interface{} {
	return []interface{}{map[string]interface{}{"title": "Selected", "entries": entries}}
}

func TestLLMsTxtFreshInstancesAndTitleOnlyInventory(t *testing.T) {
	first := newLLMsTestPlugin(t, newLLMsTestConfig())
	otherConfig := newLLMsTestConfig()
	otherConfig.Set("title", "Other project")
	second := newLLMsTestPlugin(t, otherConfig)
	for _, test := range []struct {
		name   string
		plugin ArtifactPreparer
		want   string
	}{
		{"first site", first, "# Test project\n"},
		{"second site", second, "# Other project\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan, err := test.plugin.PrepareArtifacts(ArtifactContext{})
			require.NoError(t, err)
			require.Equal(t, []string{"/llms.txt"}, plan.Routes)
			require.Empty(t, plan.Capture)
			require.Equal(t, test.want, string(buildLLMsTestPlan(t, plan)["/llms.txt"]))
		})
	}
}

func TestLLMsTxtSettingsErrors(t *testing.T) {
	for _, test := range []struct {
		name  string
		key   string
		value interface{}
		want  string
	}{
		{"missing project title", "title", "", "nonempty title"},
		{"blank project title", "title", " \t\n", "nonempty title"},
		{"project title type", "title", 7, "title must be a string"},
		{"project description type", "description", false, "description must be a string"},
		{"settings scalar", "llms_txt", true, "llms_txt must be a map"},
		{"settings null", "llms_txt", nil, "llms_txt must be a map"},
		{"override title type", "llms_txt", map[string]interface{}{"title": 7}, "title must be a string"},
		{"override title blank", "llms_txt", map[string]interface{}{"title": " "}, "nonempty title"},
		{"override description type", "llms_txt", map[string]interface{}{"description": 7}, "description must be a string"},
		{"markdown type", "llms_txt", map[string]interface{}{"markdown": "false"}, "markdown must be a boolean"},
		{"sections type", "llms_txt", map[string]interface{}{"sections": "Pages"}, "sections must be a list"},
		{"section type", "llms_txt", map[string]interface{}{"sections": []interface{}{"Pages"}}, "section must be a map"},
		{"section missing title", "llms_txt", map[string]interface{}{"sections": []interface{}{map[string]interface{}{"entries": []interface{}{}}}}, "section requires title"},
		{"section blank title", "llms_txt", map[string]interface{}{"sections": []interface{}{map[string]interface{}{"title": " "}}}, "section requires title"},
		{"section title type", "llms_txt", map[string]interface{}{"sections": []interface{}{map[string]interface{}{"title": 9}}}, "title must be a string"},
		{"section description type", "llms_txt", map[string]interface{}{"sections": []interface{}{map[string]interface{}{"title": "Pages", "description": false}}}, "description must be a string"},
		{"entries missing", "llms_txt", map[string]interface{}{"sections": []interface{}{map[string]interface{}{"title": "Pages"}}}, "entries must be a list"},
		{"entries type", "llms_txt", map[string]interface{}{"sections": []interface{}{map[string]interface{}{"title": "Pages", "entries": "guide"}}}, "entries must be a list"},
		{"entry type", "llms_txt", map[string]interface{}{"sections": llmsTestSections("guide")}, "entry must be a map"},
		{"entry missing url", "llms_txt", map[string]interface{}{"sections": llmsTestSections(map[string]interface{}{})}, "entry requires url"},
		{"entry url type", "llms_txt", map[string]interface{}{"sections": llmsTestSections(map[string]interface{}{"url": 3})}, "url must be a string"},
		{"entry title type", "llms_txt", map[string]interface{}{"sections": llmsTestSections(map[string]interface{}{"url": "/guide/", "title": true})}, "title must be a string"},
		{"entry description type", "llms_txt", map[string]interface{}{"sections": llmsTestSections(map[string]interface{}{"url": "/guide/", "description": 3})}, "description must be a string"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := newLLMsTestConfig()
			cfg.Set(test.key, test.value)
			plan, err := newLLMsTestPlugin(t, cfg).PrepareArtifacts(ArtifactContext{})
			require.ErrorContains(t, err, test.want)
			require.Nil(t, plan)
		})
	}
}

func TestLLMsTxtSettingsFromYAML(t *testing.T) {
	cfg := newLLMsTestConfig()
	require.NoError(t, config.Unmarshal([]byte(`title: Site title
llms_txt:
  title: Curated project
  description: Curated summary
  markdown: false
  sections:
    - title: Start here
      description: Read these first.
      entries:
        - url: /guide/
          title: Curated guide
          description: An explicit summary.
        - url: https://external.invalid/reference
          title: External reference
`), cfg))
	guide := newLLMsTestPage("/guide/", "guide.md", "Front matter title")
	plan, err := newLLMsTestPlugin(t, cfg).PrepareArtifacts(ArtifactContext{Pages: []Page{guide}})
	require.NoError(t, err)
	require.Empty(t, plan.Capture)
	require.Equal(t, "# Curated project\n\n> Curated summary\n\n## Start here\n\nRead these first.\n\n- [Curated guide](<https://example.test/guide/>): An explicit summary.\n- [External reference](<https://external.invalid/reference>)\n", string(buildLLMsTestPlan(t, plan)["/llms.txt"]))
	require.Zero(t, guide.articleCalls)
}

func TestLLMsTxtCuratedOrderAndCanonicalNormalization(t *testing.T) {
	cfg := newLLMsTestConfig()
	cfg.BaseURL = "/manual/"
	cfg.Set("llms_txt", map[string]interface{}{
		"title": "Curated project", "description": "Project summary", "markdown": false,
		"sections": []interface{}{
			map[string]interface{}{"title": "First", "entries": []interface{}{
				map[string]interface{}{"url": "https://EXAMPLE.test/manual/z/", "title": "Chosen Z", "description": "Chosen summary"},
				map[string]interface{}{"url": "https://external.invalid/reference?q=1#part", "title": "External reference"},
			}},
			map[string]interface{}{"title": "Second", "entries": []interface{}{
				map[string]interface{}{"url": "/a/"},
				map[string]interface{}{"url": "https://example.test/outside/", "title": "Outside base path"},
			}},
		},
	})
	a := newLLMsTestPage("/a/", "a.md", "A")
	a.fm["description"] = "Front matter summary"
	z := newLLMsTestPage("/z/", "z.md", "Z")
	unused := newLLMsTestPage("/unused/", "unused.md", "Not selected")
	plan, err := newLLMsTestPlugin(t, cfg).PrepareArtifacts(ArtifactContext{Pages: []Page{a, unused, z}})
	require.NoError(t, err)
	require.Equal(t, []string{"/llms.txt"}, plan.Routes)
	require.Empty(t, plan.Capture)
	require.Equal(t, "# Curated project\n\n> Project summary\n\n## First\n\n- [Chosen Z](<https://example.test/manual/z/>): Chosen summary\n- [External reference](<https://external.invalid/reference?q=1#part>)\n\n## Second\n\n- [A](<https://example.test/manual/a/>): Front matter summary\n- [Outside base path](<https://example.test/outside/>)\n", string(buildLLMsTestPlan(t, plan)["/llms.txt"]))
	for _, page := range []*llmsTestPage{a, z, unused} {
		require.Zero(t, page.articleCalls, "HTML links and unselected pages need no source or fetching")
	}
}

func TestLLMsTxtEmptyCuratedListReplacesAutomaticInventory(t *testing.T) {
	cfg := newLLMsTestConfig()
	cfg.Set("llms_txt", map[string]interface{}{"sections": []interface{}{}})
	page := newLLMsTestPage("/guide/", "guide.md", "Guide")
	plan, err := newLLMsTestPlugin(t, cfg).PrepareArtifacts(ArtifactContext{Pages: []Page{page}})
	require.NoError(t, err)
	require.Empty(t, plan.Capture)
	require.Equal(t, "# Test project\n", string(buildLLMsTestPlan(t, plan)["/llms.txt"]))
}

func TestLLMsTxtCuratedDestinationErrors(t *testing.T) {
	for _, test := range []struct {
		name    string
		entries []interface{}
		want    string
	}{
		{"missing local", []interface{}{map[string]interface{}{"url": "/missing/"}}, "missing or excluded"},
		{"external without title", []interface{}{map[string]interface{}{"url": "https://external.invalid/"}}, "requires title"},
		{"external blank title", []interface{}{map[string]interface{}{"url": "https://external.invalid/", "title": " "}}, "requires title"},
		{"same origin missing local", []interface{}{map[string]interface{}{"url": "https://example.test/manual/missing/", "title": "Not external"}}, "missing or excluded"},
		{"duplicate normalized local", []interface{}{map[string]interface{}{"url": "/guide/"}, map[string]interface{}{"url": "https://EXAMPLE.test/manual/guide/"}}, "duplicate"},
		{"duplicate external", []interface{}{map[string]interface{}{"url": "https://external.invalid/", "title": "One"}, map[string]interface{}{"url": "https://external.invalid/", "title": "Two"}}, "duplicate"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := newLLMsTestConfig()
			cfg.BaseURL = "/manual"
			cfg.Set("llms_txt", map[string]interface{}{"sections": llmsTestSections(test.entries...)})
			plan, err := newLLMsTestPlugin(t, cfg).PrepareArtifacts(ArtifactContext{Pages: []Page{newLLMsTestPage("/guide/", "guide.md", "Guide")}})
			require.ErrorContains(t, err, test.want)
			require.Nil(t, plan)
		})
	}
	for _, destination := range []string{
		"guide/", "/a/../guide/", "/guide//", "/guide/?q=1", "/guide/#part", "/%67uide/",
		"https://example.test/manual/guide/?q=1", "https://example.test/manual/guide/#part",
		"//external.invalid/guide/", "ftp://external.invalid/file", "mailto:help@example.test",
		"javascript:alert(1)", "https://user:password@external.invalid/", "https://external.invalid/\nextra", "/guide\\other/",
	} {
		t.Run(destination, func(t *testing.T) {
			cfg := newLLMsTestConfig()
			cfg.BaseURL = "/manual"
			cfg.Set("llms_txt", map[string]interface{}{"sections": llmsTestSections(map[string]interface{}{"url": destination, "title": "Explicit title"})})
			plan, err := newLLMsTestPlugin(t, cfg).PrepareArtifacts(ArtifactContext{Pages: []Page{newLLMsTestPage("/guide/", "guide.md", "Guide")}})
			require.Error(t, err)
			require.Nil(t, plan)
		})
	}
}

func TestLLMsTxtCanonicalConfigurationErrors(t *testing.T) {
	for _, origin := range []string{"", "/relative", "ftp://example.test", "https://example.test/path", "https://user@example.test", "https://example.test?q=1", "https://example.test#part"} {
		t.Run("url="+origin, func(t *testing.T) {
			cfg := newLLMsTestConfig()
			cfg.AbsoluteURL = origin
			plan, err := newLLMsTestPlugin(t, cfg).PrepareArtifacts(ArtifactContext{})
			require.ErrorContains(t, err, "canonical site url")
			require.Nil(t, plan)
		})
	}
	for _, base := range []string{"relative", "/a/../b", "/a//b", "/a?b", "/a#b", "/a%20b", "/a\\b"} {
		t.Run("baseurl="+base, func(t *testing.T) {
			cfg := newLLMsTestConfig()
			cfg.BaseURL = base
			plan, err := newLLMsTestPlugin(t, cfg).PrepareArtifacts(ArtifactContext{})
			require.ErrorContains(t, err, "baseurl")
			require.Nil(t, plan)
		})
	}
}

func TestLLMsTxtCompanionMappingAndCapturedSource(t *testing.T) {
	for _, test := range []struct{ route, companion, resolved string }{
		{"/", "/index.md", "https://example.test/manual/next/"},
		{"/guide/", "/guide/index.md", "https://example.test/manual/guide/next/"},
		{"/guide.html", "/guide.html.md", "https://example.test/manual/next/"},
		{"/guide", "/guide.md", "https://example.test/manual/next/"},
	} {
		t.Run(test.route, func(t *testing.T) {
			cfg := newLLMsTestConfig()
			cfg.BaseURL = "/manual"
			page := newLLMsTestPage(test.route, "guide.md", "Guide")
			page.body = []byte("# Heading\n\n[Next](next/)\n\n```md\n[Example](next/)\n{{ literal }}\n```\n")
			plan, err := newLLMsTestPlugin(t, cfg).PrepareArtifacts(ArtifactContext{Pages: []Page{page}})
			require.NoError(t, err)
			require.ElementsMatch(t, []string{"/llms.txt", test.companion}, plan.Routes)
			require.Equal(t, []Page{page}, plan.Capture)
			require.False(t, page.captured, "the site, not planning, requests source capture")
			require.Zero(t, page.articleCalls, "planning must not render articles")
			artifacts := buildLLMsTestPlan(t, plan)
			require.Contains(t, string(artifacts["/llms.txt"]), "[Guide](<https://example.test/manual"+test.companion+">)")
			require.Equal(t, "# Heading\n\n[Next]("+test.resolved+")\n\n```md\n[Example](next/)\n{{ literal }}\n```\n", string(artifacts[test.companion]))
			require.Equal(t, 1, page.articleCalls)
		})
	}
}

func TestLLMsTxtCuratedHTMLUsesCanonicalHTML(t *testing.T) {
	cfg := newLLMsTestConfig()
	cfg.BaseURL = "/manual"
	cfg.Set("llms_txt", map[string]interface{}{"sections": llmsTestSections(map[string]interface{}{"url": "/landing/"})})
	page := newLLMsTestPage("/landing/", "landing.html", "Landing")
	plan, err := newLLMsTestPlugin(t, cfg).PrepareArtifacts(ArtifactContext{Pages: []Page{page}})
	require.NoError(t, err)
	require.Equal(t, []string{"/llms.txt"}, plan.Routes)
	require.Empty(t, plan.Capture)
	require.Contains(t, string(buildLLMsTestPlan(t, plan)["/llms.txt"]), "[Landing](<https://example.test/manual/landing/>)")
	require.Zero(t, page.articleCalls)
}

func TestLLMsTxtAutomaticEligibilityAndOrdering(t *testing.T) {
	cfg := newLLMsTestConfig()
	a := newLLMsTestPage("/a/", "a.markdown", "A")
	z := newLLMsTestPage("/z/", "z.md", "Z")
	all := []Page{z}
	var excluded []*llmsTestPage
	for _, test := range []struct {
		name   string
		mutate func(*llmsTestPage)
	}{
		{"private", func(p *llmsTestPage) { p.fm["llms"] = false }},
		{"draft", func(p *llmsTestPage) { p.fm["draft"] = true }},
		{"unpublished", func(p *llmsTestPage) { p.unpublished = true }},
		{"future post", func(p *llmsTestPage) { p.isPost = true; p.date = time.Now().AddDate(1, 0, 0) }},
		{"future dated page", func(p *llmsTestPage) { p.fm["date"] = "future"; p.date = time.Now().AddDate(1, 0, 0) }},
		{"robots string", func(p *llmsTestPage) { p.fm["robots"] = "follow, NOINDEX" }},
		{"robots list", func(p *llmsTestPage) { p.fm["robots"] = []interface{}{"follow", " NoIndex "} }},
		{"redirect", func(p *llmsTestPage) { p.fm["redirect_to"] = "/a/" }},
		{"clone", func(p *llmsTestPage) { p.backed = false }},
		{"empty source", func(p *llmsTestPage) { p.source = "" }},
		{"static", func(p *llmsTestPage) { p.static = true }},
		{"non html output", func(p *llmsTestPage) { p.ext = ".json" }},
		{"html source", func(p *llmsTestPage) { p.source = "/source/landing.html" }},
		{"hidden file", func(p *llmsTestPage) { p.source = "/source/.secret.md" }},
		{"hidden directory", func(p *llmsTestPage) { p.source = "/source/.private/guide.md" }},
		{"plans", func(p *llmsTestPage) { p.source = "/source/_plans/guide.md" }},
		{"drafts directory", func(p *llmsTestPage) { p.source = "/source/_drafts/guide.md" }},
		{"not found html", func(p *llmsTestPage) { p.url = "/404.html" }},
		{"not found pretty", func(p *llmsTestPage) { p.url = "/404/" }},
		{"not found extensionless", func(p *llmsTestPage) { p.url = "/404" }},
	} {
		page := newLLMsTestPage("/excluded-"+strings.ReplaceAll(test.name, " ", "-")+"/", "excluded.md", test.name)
		test.mutate(page)
		all = append(all, page)
		excluded = append(excluded, page)
	}
	withoutProvenance := newLLMsTestPage("/no-provenance/", "no-provenance.md", "No provenance")
	all = append(all, llmsPageWithoutProvenance{Page: withoutProvenance}, a)
	plan, err := newLLMsTestPlugin(t, cfg).PrepareArtifacts(ArtifactContext{Pages: all})
	require.NoError(t, err)
	require.Equal(t, []string{"/llms.txt", "/a/index.md", "/z/index.md"}, plan.Routes)
	require.Equal(t, []Page{a, z}, plan.Capture)
	require.Equal(t, "# Test project\n\n## Pages\n\n- [A](<https://example.test/a/index.md>)\n- [Z](<https://example.test/z/index.md>)\n", string(buildLLMsTestPlan(t, plan)["/llms.txt"]))
	for _, page := range append(excluded, withoutProvenance) {
		require.False(t, page.captured)
		require.Zero(t, page.articleCalls)
	}
}

func TestLLMsTxtCuratedExcludedPagesAndLocalMetadataErrors(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*llmsTestPage)
		want   string
	}{
		{"private", func(p *llmsTestPage) { p.fm["llms"] = false }, "missing or excluded"},
		{"draft", func(p *llmsTestPage) { p.fm["draft"] = true }, "missing or excluded"},
		{"generated clone", func(p *llmsTestPage) { p.backed = false }, "missing or excluded"},
		{"redirect", func(p *llmsTestPage) { p.fm["redirect_to"] = "/other/" }, "missing or excluded"},
		{"missing title", func(p *llmsTestPage) { delete(p.fm, "title") }, "requires title"},
		{"blank title", func(p *llmsTestPage) { p.fm["title"] = " " }, "requires title"},
		{"title type", func(p *llmsTestPage) { p.fm["title"] = true }, "title must be a string"},
		{"description type", func(p *llmsTestPage) { p.fm["description"] = 3 }, "description must be a string"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := newLLMsTestConfig()
			cfg.Set("llms_txt", map[string]interface{}{"sections": llmsTestSections(map[string]interface{}{"url": "/guide/"})})
			page := newLLMsTestPage("/guide/", "guide.md", "Guide")
			test.mutate(page)
			plan, err := newLLMsTestPlugin(t, cfg).PrepareArtifacts(ArtifactContext{Pages: []Page{page}})
			require.ErrorContains(t, err, test.want)
			require.Nil(t, plan)
			require.Zero(t, page.articleCalls)
		})
	}
}

func TestLLMsTxtBuildRequiresCapturedSourceAndPropagatesArticleError(t *testing.T) {
	t.Run("missing source capture", func(t *testing.T) {
		page := newLLMsTestPage("/guide/", "guide.md", "Guide")
		plan, err := newLLMsTestPlugin(t, newLLMsTestConfig()).PrepareArtifacts(ArtifactContext{Pages: []Page{page}})
		require.NoError(t, err)
		artifacts, err := plan.Build()
		require.ErrorContains(t, err, "source capture unavailable")
		require.Nil(t, artifacts)
	})
	t.Run("article error", func(t *testing.T) {
		page := newLLMsTestPage("/guide/", "guide.md", "Guide")
		failure := errors.New("article rendering failed")
		page.articleErr = failure
		plan, err := newLLMsTestPlugin(t, newLLMsTestConfig()).PrepareArtifacts(ArtifactContext{Pages: []Page{page}})
		require.NoError(t, err)
		page.CaptureSource()
		artifacts, err := plan.Build()
		require.ErrorIs(t, err, failure)
		require.Nil(t, artifacts)
	})
}

func TestLLMsTxtDocumentationInventoryReusesOwnedMarkdown(t *testing.T) {
	cfg := newLLMsTestConfig()
	cfg.BaseURL = "/manual"
	first := newLLMsTestPage("/docs/start/", "start.md", "Front matter start")
	second := newLLMsTestPage("/docs/api/", "api.md", "Front matter API")
	unselected := newLLMsTestPage("/extra/", "extra.md", "Not in documentation")
	docs := &DocumentationArtifacts{
		Project: docexport.Project{Title: "Documentation project", Description: "Documentation summary", OpenAPI: "https://api.example.test/openapi.json"},
		Groups:  []docexport.Group{{Slug: "reference", Title: "Reference"}, {Slug: "start", Title: "Start", Description: "Begin here."}},
		Articles: []docexport.Article{
			{URL: "/docs/start/", Title: "Inventory start", Description: "Start summary", NavSection: "start", CanonicalURL: "https://example.test/manual/docs/start/", MarkdownURL: "https://example.test/manual/docs/start/index.md"},
			{URL: "/docs/api/", Title: "Inventory API", NavSection: "reference", CanonicalURL: "https://example.test/manual/docs/api/", MarkdownURL: "https://example.test/manual/docs/api/index.md"},
		},
		Sources: map[string]string{"/docs/start/": first.Source(), "/docs/api/": second.Source()},
	}
	plan, err := newLLMsTestPlugin(t, cfg).PrepareArtifacts(ArtifactContext{Pages: []Page{first, unselected, second}, Documentation: docs})
	require.NoError(t, err)
	require.Equal(t, []string{"/llms.txt"}, plan.Routes)
	require.Empty(t, plan.Capture, "documentation already owns and captures these companions")
	require.Equal(t, "# Documentation project\n\n> Documentation summary\n\n## Downloads\n\n- [Article manifest](<https://example.test/manual/docs/manifest.json>): Canonical URLs, Markdown URLs, and section metadata.\n- [Section corpus](<https://example.test/manual/docs/search.json>): Downloadable source text for local search.\n\n## Reference\n\n- [Inventory API](<https://example.test/manual/docs/api/index.md>)\n\n## Start\n\nBegin here.\n\n- [Inventory start](<https://example.test/manual/docs/start/index.md>): Start summary\n\n## API specification\n\n- [OpenAPI](<https://api.example.test/openapi.json>): Canonical API specification.\n", string(buildLLMsTestPlan(t, plan)["/llms.txt"]))
	for _, page := range []*llmsTestPage{first, second, unselected} {
		require.Zero(t, page.articleCalls)
	}
}

func TestLLMsTxtDocumentationOwnershipMismatch(t *testing.T) {
	for _, test := range []struct{ name, canonical, markdown, source string }{
		{"canonical", "https://wrong.test/docs/start/", "https://example.test/docs/start/index.md", "/source/start.md"},
		{"markdown", "https://example.test/docs/start/", "https://wrong.test/docs/start/index.md", "/source/start.md"},
		{"source", "https://example.test/docs/start/", "https://example.test/docs/start/index.md", "/source/other.md"},
	} {
		t.Run(test.name, func(t *testing.T) {
			docs := &DocumentationArtifacts{
				Groups:   []docexport.Group{{Slug: "start", Title: "Start"}},
				Articles: []docexport.Article{{URL: "/docs/start/", Title: "Start", NavSection: "start", CanonicalURL: test.canonical, MarkdownURL: test.markdown}},
				Sources:  map[string]string{"/docs/start/": test.source},
			}
			plan, err := newLLMsTestPlugin(t, newLLMsTestConfig()).PrepareArtifacts(ArtifactContext{Pages: []Page{newLLMsTestPage("/docs/start/", "start.md", "Start")}, Documentation: docs})
			require.ErrorContains(t, err, "ownership mismatch")
			require.Nil(t, plan)
		})
	}
}

func TestLLMsTxtExplicitEmptyMetadataOverrides(t *testing.T) {
	for _, test := range []struct {
		name      string
		entry     map[string]interface{}
		wantError bool
	}{
		{"empty title does not inherit", map[string]interface{}{"url": "/guide/", "title": ""}, true},
		{"empty description suppresses inheritance", map[string]interface{}{"url": "/guide/", "description": ""}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := newLLMsTestConfig()
			cfg.Set("llms_txt", map[string]interface{}{"markdown": false, "sections": llmsTestSections(test.entry)})
			page := newLLMsTestPage("/guide/", "guide.md", "Guide")
			page.fm["description"] = "Inherited description"
			plan, err := newLLMsTestPlugin(t, cfg).PrepareArtifacts(ArtifactContext{Pages: []Page{page}})
			if test.wantError {
				require.ErrorContains(t, err, "requires title")
				require.Nil(t, plan)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "# Test project\n\n## Selected\n\n- [Guide](<https://example.test/guide/>)\n", string(buildLLMsTestPlan(t, plan)["/llms.txt"]))
		})
	}
}
