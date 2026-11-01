package plugins

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func newLLMsLocalizedTestContext(label, prefix string, pages ...Page) LocalizedArtifactContext {
	cfg := newLLMsTestConfig()
	cfg.BaseURL = "/project"
	if prefix != "" {
		cfg.BaseURL += "/" + prefix
	}
	return LocalizedArtifactContext{Site: &mockSite{cfg: cfg}, Label: label, Prefix: prefix, Pages: pages}
}

func newLLMsLocalizedTestProject() Site {
	cfg := newLLMsTestConfig()
	cfg.BaseURL = "/project"
	return &mockSite{cfg: cfg}
}

func TestLocalizedLLMsAutomaticActualEditionsAndOrder(t *testing.T) {
	project := newLLMsLocalizedTestProject()
	project.Config().Set("llms_txt", map[string]interface{}{"title": "Project discovery", "description": "Project summary"})
	english := newLLMsTestPage("/getting-started/", "start.en.md", "English start")
	english.body = []byte("English body.\n\n[Next](next/)\n")
	german := newLLMsTestPage("/de/erste-schritte/", "start.de.md", "Deutscher Einstieg")
	german.body = []byte("Deutscher Inhalt.\n\n[Weiter](weiter/)\n")
	germanOnly := newLLMsTestPage("/de/zusaetzlich/", "extra.de.md", "Nur Deutsch")
	locales := []LocalizedArtifactContext{
		newLLMsLocalizedTestContext("Deutsch", "de", germanOnly, german),
		newLLMsLocalizedTestContext("English", "", english),
	}
	// Locale serving settings must not supply the project's canonical origin or
	// append their already-applied base path to actual prefixed page URLs.
	locales[0].Site.Config().AbsoluteURL = "http://preview.invalid:4321"
	locales[0].Site.Config().Set("title", "Locale title is not the aggregate H1")
	aggregate, plans, err := PrepareLocalizedLLMsArtifacts(project, locales)
	require.NoError(t, err)
	require.Len(t, plans, 2)
	require.Equal(t, []string{"/llms.txt"}, aggregate.Routes)
	require.Empty(t, aggregate.Capture)
	require.Equal(t, []string{"/de/erste-schritte/index.md", "/de/zusaetzlich/index.md"}, plans[0].Routes)
	require.Equal(t, []Page{german, germanOnly}, plans[0].Capture)
	require.Equal(t, []string{"/getting-started/index.md"}, plans[1].Routes)
	require.Equal(t, []Page{english}, plans[1].Capture)
	index := string(buildLLMsTestPlan(t, aggregate)["/llms.txt"])
	require.Equal(t, "# Project discovery\n\n> Project summary\n\n## Deutsch — Pages\n\n- [Deutscher Einstieg](<https://example.test/project/de/erste-schritte/index.md>)\n- [Nur Deutsch](<https://example.test/project/de/zusaetzlich/index.md>)\n\n## English — Pages\n\n- [English start](<https://example.test/project/getting-started/index.md>)\n", index)
	de := string(buildLLMsTestPlan(t, plans[0])["/de/erste-schritte/index.md"])
	en := string(buildLLMsTestPlan(t, plans[1])["/getting-started/index.md"])
	require.Contains(t, de, "Deutscher Inhalt.")
	require.Contains(t, de, "https://example.test/project/de/erste-schritte/weiter/")
	require.NotContains(t, de, "English body")
	require.Contains(t, en, "English body.")
	require.Contains(t, en, "https://example.test/project/getting-started/next/")
	require.NotContains(t, en, "Deutscher Inhalt")
	// Config changes after preparation cannot rewrite this generation's index.
	project.Config().AbsoluteURL = "http://changed.invalid"
	project.Config().BaseURL = "/changed"
	project.Config().Set("title", "Changed title")
	require.Equal(t, index, string(buildLLMsTestPlan(t, aggregate)["/llms.txt"]))
}

func TestLocalizedLLMsCuratedLiteralRoutes(t *testing.T) {
	for _, test := range []struct {
		name         string
		destinations []string
		wantEnglish  bool
		wantGerman   bool
	}{
		{"English slug does not select translation", []string{"/getting-started/"}, true, false},
		{"German slug does not select translation", []string{"/erste-schritte/"}, false, true},
		{"both literal slugs", []string{"/getting-started/", "/erste-schritte/"}, true, true},
		{"routing prefix only is stripped", []string{"/de/erste-schritte/"}, false, true},
		{"absolute other locale is local", []string{"https://example.test/project/de/erste-schritte/"}, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			english := newLLMsTestPage("/getting-started/", "start.en.md", "English start")
			german := newLLMsTestPage("/de/erste-schritte/", "start.de.md", "Deutscher Einstieg")
			locales := []LocalizedArtifactContext{
				newLLMsLocalizedTestContext("English", "", english),
				newLLMsLocalizedTestContext("Deutsch", "de", german),
			}
			var entries []interface{}
			for _, destination := range test.destinations {
				entries = append(entries, map[string]interface{}{"url": destination})
			}
			for _, locale := range locales {
				locale.Site.Config().Set("llms_txt", map[string]interface{}{"sections": llmsTestSections(entries...)})
			}
			aggregate, plans, err := PrepareLocalizedLLMsArtifacts(newLLMsLocalizedTestProject(), locales)
			require.NoError(t, err)
			index := string(buildLLMsTestPlan(t, aggregate)["/llms.txt"])
			require.Equal(t, test.wantEnglish, strings.Contains(index, "[English start]"))
			require.Equal(t, test.wantGerman, strings.Contains(index, "[Deutscher Einstieg]"))
			for i, want := range []bool{test.wantEnglish, test.wantGerman} {
				artifacts := buildLLMsTestPlan(t, plans[i])
				if want {
					require.Len(t, artifacts, 1)
					require.Len(t, plans[i].Capture, 1)
				} else {
					require.Empty(t, artifacts)
					require.Empty(t, plans[i].Capture)
				}
			}
		})
	}
}

func TestLocalizedLLMsLocaleMetadataSectionsAndExternalDestinations(t *testing.T) {
	english := newLLMsTestPage("/guide/", "guide.en.md", "English inherited title")
	english.fm["description"] = "English inherited description"
	german := newLLMsTestPage("/de/guide/", "guide.de.md", "Deutscher Titel")
	german.fm["description"] = "Deutsche Beschreibung"
	locales := []LocalizedArtifactContext{
		newLLMsLocalizedTestContext("English", "", english),
		newLLMsLocalizedTestContext("Deutsch", "de", german),
	}
	external := "https://reference.invalid/guide.md?version=2#example"
	outsideBase := "https://example.test/outside/reference?version=1#part"
	locales[0].Site.Config().Set("llms_txt", map[string]interface{}{
		"markdown": false,
		"sections": []interface{}{
			map[string]interface{}{"title": "References", "entries": []interface{}{map[string]interface{}{"url": external, "title": "External English"}}},
			map[string]interface{}{"title": "Guides", "description": "English section description", "entries": []interface{}{map[string]interface{}{"url": "/guide/", "title": "English override", "description": ""}}},
		},
	})
	locales[1].Site.Config().Set("llms_txt", map[string]interface{}{
		"sections": []interface{}{
			map[string]interface{}{"title": "Anleitungen", "entries": []interface{}{map[string]interface{}{"url": "/guide/"}}},
			map[string]interface{}{"title": "Referenzen", "entries": []interface{}{
				map[string]interface{}{"url": external, "title": "Externe Referenz"},
				map[string]interface{}{"url": outsideBase, "title": "Outside deployed base"},
			}},
		},
	})
	aggregate, plans, err := PrepareLocalizedLLMsArtifacts(newLLMsLocalizedTestProject(), locales)
	require.NoError(t, err)
	index := string(buildLLMsTestPlan(t, aggregate)["/llms.txt"])
	require.Equal(t, "# Test project\n\n## English — References\n\n- [External English](<"+external+">)\n\n## English — Guides\n\nEnglish section description\n\n- [English override](<https://example.test/project/guide/>)\n\n## Deutsch — Anleitungen\n\n- [Deutscher Titel](<https://example.test/project/de/guide/index.md>): Deutsche Beschreibung\n\n## Deutsch — Referenzen\n\n- [Externe Referenz](<"+external+">)\n- [Outside deployed base](<"+outsideBase+">)\n", index)
	require.Empty(t, plans[0].Routes)
	require.Empty(t, plans[0].Capture)
	require.Empty(t, buildLLMsTestPlan(t, plans[0]))
	require.Equal(t, []string{"/de/guide/index.md"}, plans[1].Routes)
	require.Zero(t, english.articleCalls)
	buildLLMsTestPlan(t, plans[1])
}

func TestLocalizedLLMsCuratedErrors(t *testing.T) {
	for _, test := range []struct {
		name     string
		entries  []interface{}
		excluded string
		want     string
	}{
		{"everywhere missing", []interface{}{map[string]interface{}{"url": "/missing/"}}, "", "missing or excluded"},
		{"excluded other edition", []interface{}{map[string]interface{}{"url": "/guide/"}}, "llms", "missing or excluded"},
		{"excluded absolute other locale", []interface{}{map[string]interface{}{"url": "https://example.test/project/de/guide/", "title": "Not external"}}, "llms", "missing or excluded"},
		{"noindex other edition", []interface{}{map[string]interface{}{"url": "/guide/"}}, "robots", "missing or excluded"},
		{"duplicate local prefixes", []interface{}{map[string]interface{}{"url": "/guide/"}, map[string]interface{}{"url": "/de/guide/"}}, "", "duplicate"},
		{"duplicate local absolute", []interface{}{map[string]interface{}{"url": "/guide/"}, map[string]interface{}{"url": "https://example.test/project/de/guide/"}}, "", "duplicate"},
		{"duplicate in absent locale", []interface{}{map[string]interface{}{"url": "/only-german/"}, map[string]interface{}{"url": "/de/only-german/"}}, "", "duplicate"},
		{"duplicate external", []interface{}{map[string]interface{}{"url": "https://external.invalid/", "title": "One"}, map[string]interface{}{"url": "https://external.invalid/", "title": "Two"}}, "", "duplicate"},
		{"external title required", []interface{}{map[string]interface{}{"url": "https://external.invalid/"}}, "", "requires title"},
	} {
		t.Run(test.name, func(t *testing.T) {
			english := newLLMsTestPage("/guide/", "guide.en.md", "English")
			german := newLLMsTestPage("/de/guide/", "guide.de.md", "Deutsch")
			if test.excluded == "llms" {
				german.fm["llms"] = false
			} else if test.excluded == "robots" {
				german.fm["robots"] = "noindex"
			}
			locales := []LocalizedArtifactContext{
				newLLMsLocalizedTestContext("English", "", english),
				newLLMsLocalizedTestContext("Deutsch", "de", german, newLLMsTestPage("/de/only-german/", "only.de.md", "Nur Deutsch")),
			}
			for _, locale := range locales {
				locale.Site.Config().Set("llms_txt", map[string]interface{}{"sections": llmsTestSections(test.entries...)})
			}
			aggregate, plans, err := PrepareLocalizedLLMsArtifacts(newLLMsLocalizedTestProject(), locales)
			require.ErrorContains(t, err, test.want)
			require.Nil(t, aggregate)
			require.Nil(t, plans)
			require.Zero(t, english.articleCalls)
			require.Zero(t, german.articleCalls)
		})
	}
}

func TestLocalizedLLMsCanonicalPrefixAndCompanionForms(t *testing.T) {
	for _, prefix := range []string{"", "en"} {
		for _, base := range []string{"", "/project"} {
			for _, markdown := range []bool{true, false} {
				t.Run(fmt.Sprintf("prefix=%s/base=%s/markdown=%t", prefix, base, markdown), func(t *testing.T) {
					project := newLLMsLocalizedTestProject()
					project.Config().BaseURL = base
					var pages []Page
					var expected []string
					for _, route := range []string{"/", "/guide/", "/guide.html", "/plain", "/de/literal-slug/"} {
						actual := route
						if prefix != "" {
							actual = "/" + prefix + route
						}
						pages = append(pages, newLLMsTestPage(actual, fmt.Sprintf("page%d.md", len(pages)), "Page "+route))
						expected = append(expected, markdownCompanionRoute(actual))
					}
					locale := newLLMsLocalizedTestContext("English", prefix, pages...)
					locale.Site.Config().Set("llms_txt", map[string]interface{}{"markdown": markdown})
					// Keep another configured prefix even with no editions; automatic
					// routes containing that slug must not have it stripped twice.
					other := newLLMsLocalizedTestContext("Deutsch", "de")
					aggregate, plans, err := PrepareLocalizedLLMsArtifacts(project, []LocalizedArtifactContext{locale, other})
					require.NoError(t, err)
					index := string(buildLLMsTestPlan(t, aggregate)["/llms.txt"])
					if markdown {
						require.ElementsMatch(t, expected, plans[0].Routes)
						buildLLMsTestPlan(t, plans[0])
					} else {
						require.Empty(t, plans[0].Routes)
						require.Empty(t, plans[0].Capture)
						require.Empty(t, buildLLMsTestPlan(t, plans[0]))
					}
					for _, page := range pages {
						destination := page.URL()
						if markdown {
							destination = markdownCompanionRoute(destination)
						}
						require.Contains(t, index, "<https://example.test"+base+destination+">")
					}
					require.Empty(t, buildLLMsTestPlan(t, plans[1]))
				})
			}
		}
	}
}

func TestLocalizedLLMsCompanionRequiresActualSourceCapture(t *testing.T) {
	german := newLLMsTestPage("/de/guide/", "guide.de.md", "Deutsch")
	_, plans, err := PrepareLocalizedLLMsArtifacts(newLLMsLocalizedTestProject(), []LocalizedArtifactContext{newLLMsLocalizedTestContext("Deutsch", "de", german)})
	require.NoError(t, err)
	artifacts, err := plans[0].Build()
	require.ErrorContains(t, err, "source capture unavailable")
	require.Nil(t, artifacts)
}

func TestLocalizedLLMsKnownNonOutputRouteCannotSelectOtherEdition(t *testing.T) {
	english := newLLMsTestPage("/guide/", "guide.en.md", "English")
	locales := []LocalizedArtifactContext{
		newLLMsLocalizedTestContext("English", "", english),
		newLLMsLocalizedTestContext("Deutsch", "de"),
	}
	locales[1].ExcludedRoutes = []string{"/de/guide/"}
	for _, locale := range locales {
		locale.Site.Config().Set("llms_txt", map[string]interface{}{
			"sections": llmsTestSections(map[string]interface{}{"url": "/guide/"}),
		})
	}
	aggregate, plans, err := PrepareLocalizedLLMsArtifacts(newLLMsLocalizedTestProject(), locales)
	require.ErrorContains(t, err, "missing or excluded")
	require.Nil(t, aggregate)
	require.Nil(t, plans)
	require.Zero(t, english.articleCalls)
}

func TestLocalizedLLMsLocalizedNotFoundRoutesStayExcluded(t *testing.T) {
	for _, route := range []string{"/de/404.html", "/de/404/", "/de/404"} {
		t.Run(route, func(t *testing.T) {
			notFound := newLLMsTestPage(route, "404.de.md", "Not found")
			guide := newLLMsTestPage("/de/guide/", "guide.de.md", "Guide")
			locale := newLLMsLocalizedTestContext("Deutsch", "de", notFound, guide)
			aggregate, plans, err := PrepareLocalizedLLMsArtifacts(newLLMsLocalizedTestProject(), []LocalizedArtifactContext{locale})
			require.NoError(t, err)
			index := string(buildLLMsTestPlan(t, aggregate)["/llms.txt"])
			require.NotContains(t, index, "Not found")
			require.Equal(t, []string{"/de/guide/index.md"}, plans[0].Routes)
			require.Equal(t, []Page{guide}, plans[0].Capture)
			locale.Site.Config().Set("llms_txt", map[string]interface{}{
				"sections": llmsTestSections(map[string]interface{}{"url": route}),
			})
			aggregate, plans, err = PrepareLocalizedLLMsArtifacts(newLLMsLocalizedTestProject(), []LocalizedArtifactContext{locale})
			require.ErrorContains(t, err, "missing or excluded")
			require.Nil(t, aggregate)
			require.Nil(t, plans)
			require.Zero(t, notFound.articleCalls)
		})
	}
}
