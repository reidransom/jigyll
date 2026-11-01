package site

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/stretchr/testify/require"
)

func llmsLocalizedFixture(t *testing.T, extra string, subdir bool) string {
	t.Helper()
	source := t.TempDir()
	llmsLocalizedConfig(t, source, extra, subdir)
	writeLocalizedProjectFile(t, source, "_layouts/default.html", "LAYOUT CHROME {{ content }}")
	writeLocalizedProjectFile(t, source, "_includes/edition.md", "Included {{ page.lang }} edition\n")
	writeLocalizedProjectFile(t, source, "english.md", "---\ntitle: English Guide\nlang: en\ntranslation_key: start\npermalink: /getting-started/\nlayout: default\n---\nEnglish original body\n\n{% include edition.md %}\n\n[Self](./)\n")
	writeLocalizedProjectFile(t, source, "german.md", "---\ntitle: Deutscher Leitfaden\nlang: de\ntranslation_key: start\npermalink: /erste-schritte/\nlayout: default\n---\nDeutscher Originaltext\n\n{% include edition.md %}\n\n[Self](./)\n")
	return source
}

func llmsLocalizedConfig(t *testing.T, source, extra string, subdir bool) {
	t.Helper()
	subdirValue := "false"
	if subdir {
		subdirValue = "true"
	}
	writeLocalizedProjectFile(t, source, "_config.yml", "title: Localized Project\ndescription: Project summary\nurl: https://example.test\nbaseurl: /project\ndestination: _site\nplugins: [jigyll-llms-txt]\nlocalization:\n  default_language: en\n  default_language_in_subdir: "+subdirValue+"\n  locales:\n    en: {tag: en, label: English, weight: 2}\n    de: {tag: de, label: Deutsch, weight: 1, fallbacks: [en]}\n"+extra)
}

func llmsLocalizedBuild(t *testing.T, source string, development bool) *LocalizedProject {
	t.Helper()
	base, err := FromDirectory(source, config.Flags{})
	require.NoError(t, err)
	var project *LocalizedProject
	if development {
		project, _, err = BuildLocalizedDevelopmentProject(base)
	} else {
		project, _, err = BuildLocalizedProject(base)
	}
	require.NoError(t, err)
	return project
}

func llmsLocalizedContent(t *testing.T, project *LocalizedProject, route string) string {
	t.Helper()
	var content bytes.Buffer
	if document, found := project.ServedDocument(route); found {
		require.NoError(t, document.RenderTo(&content))
	} else {
		_, document, found := project.URLPage(route)
		require.True(t, found, "missing route %s", route)
		require.NoError(t, document.Write(&content))
	}
	return content.String()
}

func llmsLocalizedAbsent(t *testing.T, project *LocalizedProject, source, route string, disk bool) {
	t.Helper()
	_, found := project.ServedDocument(route)
	require.False(t, found, "unexpected served route %s", route)
	_, _, found = project.URLPage(route)
	require.False(t, found, "unexpected project route %s", route)
	if disk {
		require.NoFileExists(t, filepath.Join(source, "_site", filepath.FromSlash(strings.TrimPrefix(route, "/"))))
	}
}

func TestLLMsLocalizedActualEditionsAndConfiguredOrder(t *testing.T) {
	for _, development := range []bool{false, true} {
		name := "build"
		if development {
			name = "development"
		}
		t.Run(name, func(t *testing.T) {
			source := llmsLocalizedFixture(t, "llms_txt:\n  sections:\n    - title: Guides\n      entries:\n        - url: /getting-started/\n        - url: /erste-schritte/\n    - title: References\n      entries:\n        - url: https://outside.test/reference.md\n          title: External Reference\n", false)
			project := llmsLocalizedBuild(t, source, development)
			index := llmsLocalizedContent(t, project, "/llms.txt")
			require.Contains(t, index, "# Localized Project\n")
			require.Contains(t, index, "> Project summary")
			previous := -1
			for _, heading := range []string{"## Deutsch — Guides", "## Deutsch — References", "## English — Guides", "## English — References"} {
				position := strings.Index(index, heading)
				require.Greater(t, position, previous, "missing or unordered heading %s", heading)
				previous = position
			}
			require.NotContains(t, index, "### ")
			require.Equal(t, 2, strings.Count(index, "https://outside.test/reference.md"))
			for _, edition := range []struct{ route, body, other, canonical string }{
				{"/getting-started/index.md", "English original body", "Deutscher Originaltext", "https://example.test/project/getting-started/"},
				{"/de/erste-schritte/index.md", "Deutscher Originaltext", "English original body", "https://example.test/project/de/erste-schritte/"},
			} {
				require.Contains(t, index, edition.canonical+"index.md")
				body := llmsLocalizedContent(t, project, edition.route)
				require.Contains(t, body, edition.body)
				require.NotContains(t, body, edition.other)
				require.Contains(t, body, edition.canonical)
				require.NotContains(t, body, "LAYOUT CHROME")
				require.NotContains(t, body, "translation_key:")
				require.NotContains(t, body, "/project/project/")
				if !development {
					require.Equal(t, body, string(readLocalizedProjectOutput(t, filepath.Join(source, "_site", edition.route))))
				}
			}
			require.Contains(t, llmsLocalizedContent(t, project, "/getting-started/index.md"), "Included en edition")
			require.Contains(t, llmsLocalizedContent(t, project, "/de/erste-schritte/index.md"), "Included de edition")
			if !development {
				require.Equal(t, index, string(readLocalizedProjectOutput(t, filepath.Join(source, "_site", "llms.txt"))))
			} else {
				require.NoDirExists(t, filepath.Join(source, "_site"))
			}
			for _, route := range []string{"/de/llms.txt", "/en/llms.txt", "/de/getting-started/index.md", "/erste-schritte/index.md"} {
				llmsLocalizedAbsent(t, project, source, route, !development)
			}
		})
	}
}

func TestLLMsLocalizedLiteralCuration(t *testing.T) {
	for _, test := range []struct {
		name, entries   string
		english, german bool
	}{
		{"both-slugs", "        - url: /getting-started/\n        - url: /erste-schritte/\n", true, true},
		{"english-only", "        - url: /getting-started/\n", true, false},
		{"german-only", "        - url: /erste-schritte/\n", false, true},
		{"prefixed-german", "        - url: /de/erste-schritte/\n", false, true},
		{"absolute-other-locale", "        - url: https://example.test/project/de/erste-schritte/\n", false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := llmsLocalizedFixture(t, "llms_txt:\n  sections:\n    - title: Chosen\n      entries:\n"+test.entries, false)
			project := llmsLocalizedBuild(t, source, false)
			index := llmsLocalizedContent(t, project, "/llms.txt")
			for _, edition := range []struct {
				selected           bool
				title, route, body string
			}{
				{test.english, "English Guide", "/getting-started/index.md", "English original body"},
				{test.german, "Deutscher Leitfaden", "/de/erste-schritte/index.md", "Deutscher Originaltext"},
			} {
				if edition.selected {
					require.Contains(t, index, "["+edition.title+"]")
					require.Contains(t, llmsLocalizedContent(t, project, edition.route), edition.body)
					require.FileExists(t, filepath.Join(source, "_site", edition.route))
				} else {
					require.NotContains(t, index, "["+edition.title+"]")
					llmsLocalizedAbsent(t, project, source, edition.route, true)
				}
			}
		})
	}
}

func TestLLMsLocalizedInvalidCurationRetainsDiskBytes(t *testing.T) {
	for _, test := range []struct{ name, url, exclusion string }{
		{"missing-everywhere", "/missing/", ""},
		{"excluded", "/erste-schritte/", "llms: false\n"},
		{"absolute-excluded", "https://example.test/project/de/erste-schritte/", "llms: false\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := llmsLocalizedFixture(t, "", false)
			project := llmsLocalizedBuild(t, source, false)
			before := llmsLocalizedSnapshot(t, source)
			llmsLocalizedConfig(t, source, "llms_txt:\n  sections:\n    - title: Chosen\n      entries:\n        - url: "+test.url+"\n", false)
			if test.exclusion != "" {
				writeLocalizedProjectFile(t, source, "german.md", "---\ntitle: Rejected Guide\nlang: de\ntranslation_key: start\npermalink: /erste-schritte/\n"+test.exclusion+"---\nRejected new body\n")
			}
			replacement, _, err := project.Rebuild()
			require.Error(t, err)
			require.Nil(t, replacement)
			llmsLocalizedRequireSnapshot(t, source, before)
			require.Contains(t, llmsLocalizedContent(t, project, "/de/erste-schritte/index.md"), "Deutscher Originaltext")
		})
	}
}

func TestLLMsLocalizedMissingEditionUsesCatalogPolicy(t *testing.T) {
	source := llmsLocalizedFixture(t, "", false)
	require.NoError(t, os.Remove(filepath.Join(source, "german.md")))
	project := llmsLocalizedBuild(t, source, false)
	index := llmsLocalizedContent(t, project, "/llms.txt")
	require.Contains(t, index, "English Guide")
	require.NotContains(t, index, "Deutscher Leitfaden")
	require.NotContains(t, index, "/de/getting-started/")
	llmsLocalizedAbsent(t, project, source, "/de/getting-started/index.md", true)
	llmsLocalizedAbsent(t, project, source, "/de/erste-schritte/index.md", true)
	before := llmsLocalizedSnapshot(t, source)
	filename := filepath.Join(source, "_config.yml")
	contents, err := os.ReadFile(filename)
	require.NoError(t, err)
	writeLocalizedProjectFile(t, source, "_config.yml", strings.Replace(string(contents), "  default_language: en\n", "  default_language: en\n  required_translations: [de]\n", 1))
	replacement, _, err := project.Rebuild()
	require.ErrorContains(t, err, `namespace "pages" translation_key "start" is missing required locale "de"`)
	require.Nil(t, replacement)
	llmsLocalizedRequireSnapshot(t, source, before)
}

func TestLLMsLocalizedRootFileAndExtensionlessCompanions(t *testing.T) {
	for _, subdir := range []bool{false, true} {
		for _, markdown := range []bool{false, true} {
			name := "root-html"
			if subdir {
				name = "subdir-html"
			}
			if markdown {
				name += "-markdown"
			}
			t.Run(name, func(t *testing.T) {
				setting := "false"
				if markdown {
					setting = "true"
				}
				source := llmsLocalizedFixture(t, "llms_txt:\n  markdown: "+setting+"\n", subdir)
				for _, locale := range []string{"en", "de"} {
					for _, page := range []struct{ name, route string }{{"home", "/"}, {"file", "/file.html"}, {"extensionless", "/extensionless"}} {
						writeLocalizedProjectFile(t, source, locale+"-"+page.name+".md", "---\ntitle: "+locale+" "+page.name+"\nlang: "+locale+"\ntranslation_key: "+page.name+"\npermalink: "+page.route+"\n---\nDistinct "+locale+" "+page.name+" body\n")
					}
				}
				project := llmsLocalizedBuild(t, source, false)
				index := llmsLocalizedContent(t, project, "/llms.txt")
				for _, locale := range []string{"en", "de"} {
					prefix := ""
					if locale == "de" || subdir {
						prefix = "/" + locale
					}
					for _, page := range []struct{ name, html, md string }{{"home", "/", "/index.md"}, {"file", "/file.html", "/file.html.md"}, {"extensionless", "/extensionless", "/extensionless.md"}} {
						route := prefix + page.html
						if markdown {
							route = prefix + page.md
							require.Contains(t, llmsLocalizedContent(t, project, route), "Distinct "+locale+" "+page.name+" body")
							require.FileExists(t, filepath.Join(source, "_site", route))
						} else {
							llmsLocalizedAbsent(t, project, source, prefix+page.md, true)
						}
						require.Contains(t, index, "https://example.test/project"+route+">")
					}
				}
				require.NotContains(t, index, "/project/project/")
				require.NotContains(t, index, "/de/de/")
				require.NotContains(t, index, "/en/en/")
				llmsLocalizedAbsent(t, project, source, "/de/llms.txt", true)
				llmsLocalizedAbsent(t, project, source, "/en/llms.txt", true)
				if !markdown {
					llmsLocalizedAbsent(t, project, source, "/de/erste-schritte/index.md", true)
					prefix := ""
					if subdir {
						prefix = "/en"
					}
					llmsLocalizedAbsent(t, project, source, prefix+"/getting-started/index.md", true)
				}
			})
		}
	}
}

func llmsLocalizedSnapshot(t *testing.T, source string) map[string][]byte {
	t.Helper()
	contents := map[string][]byte{}
	root := filepath.Join(source, "_site")
	require.NoError(t, filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		contents[relative], err = os.ReadFile(path)
		return err
	}))
	return contents
}

func llmsLocalizedRequireSnapshot(t *testing.T, source string, before map[string][]byte) {
	t.Helper()
	require.Equal(t, before, llmsLocalizedSnapshot(t, source), "failed candidate changed last-good disk generation")
}

func TestLLMsLocalizedCollisionsRetainEntireDiskGeneration(t *testing.T) {
	for _, collision := range []struct{ name, file, contents string }{
		{"shared-aggregate", "llms.txt", "authored aggregate\n"},
		{"shared-companion", "de/erste-schritte/index.md", "authored companion\n"},
		{"locale-companion", "collision.html", "---\ntitle: Collision\nlang: de\npermalink: /erste-schritte/index.md\n---\nLocale HTML collision\n"},
		{"index-alias", "collision.html", "---\ntitle: Alias\nlang: de\npermalink: /erste-schritte/index.md.html\n---\nAlias collision\n"},
		{"aggregate-index-alias", "collision.html", "---\ntitle: Alias\nlang: en\npermalink: /llms.txt.html\n---\nAggregate alias collision\n"},
		{"physical-ancestor", "de", "ancestor file\n"},
		{"physical-descendant", "llms.txt/child.txt", "descendant file\n"},
	} {
		t.Run(collision.name, func(t *testing.T) {
			source := llmsLocalizedFixture(t, "", false)
			project := llmsLocalizedBuild(t, source, false)
			before := llmsLocalizedSnapshot(t, source)
			require.Contains(t, before, "llms.txt")
			require.Contains(t, before, filepath.FromSlash("de/erste-schritte/index.md"))
			// Route/destination validation must reject the complete candidate
			// inventory before any rendering could surface this separate error.
			writeLocalizedProjectFile(t, source, "_includes/edition.md", "{% invalid_collision_candidate_tag %}\n")
			writeLocalizedProjectFile(t, source, collision.file, collision.contents)
			replacement, _, err := project.Rebuild()
			require.ErrorContains(t, err, "invalid localized routes")
			require.Nil(t, replacement)
			llmsLocalizedRequireSnapshot(t, source, before)
			require.Equal(t, string(before["llms.txt"]), llmsLocalizedContent(t, project, "/llms.txt"))
			require.Equal(t, string(before[filepath.FromSlash("de/erste-schritte/index.md")]), llmsLocalizedContent(t, project, "/de/erste-schritte/index.md"))
		})
	}
}

func TestLLMsLocalizedDocumentationRemainsExplicitlyUnsupported(t *testing.T) {
	for _, development := range []bool{false, true} {
		source := llmsLocalizedFixture(t, "documentation:\n  sections: docs_sections\n", false)
		writeLocalizedProjectFile(t, source, "_data/docs_sections.yml", "- slug: guides\n  title: Guides\n")
		writeLocalizedProjectFile(t, source, "_site/sentinel.txt", "last-good documentation sentinel\n")
		base, err := FromDirectory(source, config.Flags{})
		require.NoError(t, err)
		if development {
			_, _, err = BuildLocalizedDevelopmentProject(base)
		} else {
			_, _, err = BuildLocalizedProject(base)
		}
		require.ErrorContains(t, err, "documentation snapshots do not support localized builds")
		require.Equal(t, "last-good documentation sentinel\n", string(readLocalizedProjectOutput(t, filepath.Join(source, "_site", "sentinel.txt"))))
		require.NoFileExists(t, filepath.Join(source, "_site", "llms.txt"))
	}
}

func TestLLMsLocalizedDevelopmentRebuildRefreshesEditionIncludeAndCuration(t *testing.T) {
	source := llmsLocalizedFixture(t, "", false)
	project := llmsLocalizedBuild(t, source, true)
	oldIndex := llmsLocalizedContent(t, project, "/llms.txt")
	oldGerman := llmsLocalizedContent(t, project, "/de/erste-schritte/index.md")
	oldEnglish := llmsLocalizedContent(t, project, "/getting-started/index.md")
	writeLocalizedProjectFile(t, source, "_includes/edition.md", "Updated include {{ page.lang }}\n")
	writeLocalizedProjectFile(t, source, "german.md", "---\ntitle: Neuer Leitfaden\nlang: de\ntranslation_key: start\npermalink: /neuer-start/\nlayout: default\n---\nNeue deutsche Ausgabe\n\n{% include edition.md %}\n")
	llmsLocalizedConfig(t, source, "llms_txt:\n  title: Updated Project\n  sections:\n    - title: Updated Selection\n      entries:\n        - url: /neuer-start/\n", false)
	replacement, _, err := project.RebuildDevelopment()
	require.NoError(t, err)
	require.NotNil(t, replacement)
	index := llmsLocalizedContent(t, replacement, "/llms.txt")
	require.Contains(t, index, "# Updated Project")
	require.Contains(t, index, "## Deutsch — Updated Selection")
	require.Contains(t, index, "https://example.test/project/de/neuer-start/index.md")
	require.NotContains(t, index, "English Guide")
	require.NotContains(t, index, "/erste-schritte/")
	german := llmsLocalizedContent(t, replacement, "/de/neuer-start/index.md")
	require.Contains(t, german, "Neue deutsche Ausgabe")
	require.Contains(t, german, "Updated include de")
	require.NotContains(t, german, "English original body")
	llmsLocalizedAbsent(t, replacement, source, "/de/erste-schritte/index.md", false)
	llmsLocalizedAbsent(t, replacement, source, "/getting-started/index.md", false)
	require.Equal(t, oldIndex, llmsLocalizedContent(t, project, "/llms.txt"))
	require.Equal(t, oldGerman, llmsLocalizedContent(t, project, "/de/erste-schritte/index.md"))
	require.Equal(t, oldEnglish, llmsLocalizedContent(t, project, "/getting-started/index.md"))
	require.NoDirExists(t, filepath.Join(source, "_site"))
}

func TestLLMsLocalizedExcludedMatchCannotHideBehindEligibleOtherLocale(t *testing.T) {
	source := llmsLocalizedFixture(t, "", false)
	project := llmsLocalizedBuild(t, source, false)
	before := llmsLocalizedSnapshot(t, source)
	writeLocalizedProjectFile(t, source, "german.md", "---\ntitle: Excluded German\nlang: de\ntranslation_key: start\npermalink: /getting-started/\nllms: false\n---\nExcluded German body\n")
	llmsLocalizedConfig(t, source, "llms_txt:\n  sections:\n    - title: Chosen\n      entries:\n        - url: /getting-started/\n", false)
	replacement, _, err := project.Rebuild()
	require.Error(t, err)
	require.Nil(t, replacement)
	llmsLocalizedRequireSnapshot(t, source, before)
	require.Contains(t, llmsLocalizedContent(t, project, "/getting-started/index.md"), "English original body")
}

func TestLLMsLocalizedNonOutputMatchCannotHideBehindOtherLocale(t *testing.T) {
	source := llmsLocalizedFixture(t, "collections:\n  notes:\n    output: false\n", false)
	project := llmsLocalizedBuild(t, source, false)
	before := llmsLocalizedSnapshot(t, source)
	writeLocalizedProjectFile(t, source, "_notes/german.md", "---\ntitle: Non-output German\nlang: de\npermalink: /getting-started/\n---\nPrivate collection body\n")
	llmsLocalizedConfig(t, source, "collections:\n  notes:\n    output: false\nllms_txt:\n  sections:\n    - title: Chosen\n      entries:\n        - url: /getting-started/\n", false)
	replacement, _, err := project.Rebuild()
	require.Error(t, err)
	require.Nil(t, replacement)
	llmsLocalizedRequireSnapshot(t, source, before)
}

func TestLLMsLocalizedRelativeSourcePublishesActualStageContents(t *testing.T) {
	source := llmsLocalizedFixture(t, "", false)
	workingDirectory, err := os.Getwd()
	require.NoError(t, err)
	relative, err := filepath.Rel(workingDirectory, source)
	require.NoError(t, err)
	project := llmsLocalizedBuild(t, relative, false)
	index := string(readLocalizedProjectOutput(t, filepath.Join(source, "_site", "llms.txt")))
	require.Contains(t, index, "https://example.test/project/de/erste-schritte/index.md")
	require.Contains(t, string(readLocalizedProjectOutput(t, filepath.Join(source, "_site", "de/erste-schritte/index.md"))), "Deutscher Originaltext")
	require.Equal(t, index, llmsLocalizedContent(t, project, "/llms.txt"))
}

func TestLLMsLocalizedPublicationDoesNotBecomeWatchedSource(t *testing.T) {
	source := llmsLocalizedFixture(t, "include: [.jigyll-localized-authored, other/_site.jigyll-localized-backup]\n", false)
	writeLocalizedProjectFile(t, source, ".jigyll-localized-authored/note.txt", "Authored hidden include\n")
	writeLocalizedProjectFile(t, source, "other/_site.jigyll-localized-backup/note.txt", "Authored unrelated backup-shaped directory\n")
	project := llmsLocalizedBuild(t, source, false)
	watched := project.project.base
	before, err := watched.pollSnapshot()
	require.NoError(t, err)
	stage, cleanup, err := project.project.localizedBuildStage()
	require.NoError(t, err)
	defer cleanup()
	writeLocalizedProjectFile(t, stage, "candidate.txt", "Unpublished staged bytes\n")
	backup := watched.DestDir() + ".jigyll-localized-backup"
	writeLocalizedProjectFile(t, backup, "previous.txt", "Promotion backup bytes\n")
	for _, filename := range []string{stage, filepath.Join(stage, "candidate.txt"), backup, filepath.Join(backup, "previous.txt")} {
		require.True(t, watched.watchProtected(filename), "publication event must not trigger another generation: %s", filename)
	}
	inventory, err := watched.scanWatchInventory()
	require.NoError(t, err)
	require.NotContains(t, inventory.dirs, stage)
	require.NotContains(t, inventory.dirs, backup)
	require.Contains(t, inventory.dirs, filepath.Join(source, ".jigyll-localized-authored"))
	require.Contains(t, inventory.dirs, filepath.Join(source, "other/_site.jigyll-localized-backup"))
	after, err := watched.pollSnapshot()
	require.NoError(t, err)
	require.Equal(t, before, after, "publication staging/backup must not become polling source input")
	require.False(t, watched.watchProtected(filepath.Join(source, ".jigyll-localized-authored/note.txt")))
	require.False(t, watched.watchProtected(filepath.Join(source, "other/_site.jigyll-localized-backup/note.txt")))
	require.NoError(t, os.RemoveAll(backup))
	replacement, _, err := project.Rebuild()
	require.NoError(t, err)
	afterRebuild, err := replacement.project.base.pollSnapshot()
	require.NoError(t, err)
	require.Equal(t, before, afterRebuild, "successful promotion must not trigger a new polling generation")
}
