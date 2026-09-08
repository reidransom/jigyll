package site

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/stretchr/testify/require"
)

func TestLocalizedProjectRebuildRetainsGenerationUntilRequiredTranslationRecovers(t *testing.T) {
	source, english, french := requiredTranslationProjectFixture(t)

	base, err := FromDirectory(source, config.Flags{})
	require.NoError(t, err)
	project, _, err := BuildLocalizedProject(base)
	require.NoError(t, err)

	destination := filepath.Join(source, "_site", "guide", "index.html")
	initialOutput, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Contains(t, string(initialOutput), "first generation")
	require.FileExists(t, filepath.Join(source, "_site", "fr", "guide", "index.html"))
	requireLocalizedGeneration(t, project, "first")

	require.NoError(t, os.WriteFile(french, []byte(`---
lang: fr
translation_key: guide
published: false
permalink: /guide/
---
French guide
`), 0o644))

	replacement, _, err := project.Rebuild()
	require.ErrorContains(t, err, `namespace "pages" translation_key "guide" is missing required locale "fr"`)
	require.Nil(t, replacement)
	require.Equal(t, initialOutput, readLocalizedProjectOutput(t, destination))
	requireLocalizedGeneration(t, project, "first")

	require.NoError(t, os.WriteFile(english, []byte(`---
lang: en
translation_key: guide
generation: second
permalink: /guide/
---
second generation
`), 0o644))
	require.NoError(t, os.WriteFile(french, []byte(`---
lang: fr
translation_key: guide
permalink: /guide/
---
French guide
`), 0o644))

	replacement, _, err = project.Rebuild()
	require.NoError(t, err)
	require.NotNil(t, replacement)
	require.Contains(t, string(readLocalizedProjectOutput(t, destination)), "second generation")
	requireLocalizedGeneration(t, replacement, "second")
}

func TestLocalizedDevelopmentProjectMaterializesRoutesWithoutDestinationPublication(t *testing.T) {
	source, destination := localizedDevelopmentProjectFixture(t)

	base, err := FromDirectory(source, config.Flags{})
	require.NoError(t, err)
	project, _, err := BuildLocalizedDevelopmentProject(base)
	require.NoError(t, err)

	require.Contains(t, localizedServedContent(t, project, "/guide/"), "layout:start <p>English guide</p>\n layout:end")
	require.Contains(t, localizedServedContent(t, project, "/de/willkommen/"), "layout:start <p>German guide</p>\n layout:end")
	require.Equal(t, "body { color: red; }\n", localizedServedContent(t, project, "/assets/site.css"))
	require.Equal(t, "preserve this destination", string(readLocalizedProjectOutput(t, filepath.Join(destination, "sentinel.txt"))))
	require.NoFileExists(t, filepath.Join(destination, "guide", "index.html"))

	writeLocalizedProjectFile(t, source, "english.md", `---
lang: en
permalink: /guide/
layout: default
---
changed source
`)
	require.Contains(t, localizedServedContent(t, project, "/guide/"), "English guide")
	require.NotContains(t, localizedServedContent(t, project, "/guide/"), "changed source")
}

func TestLocalizedDevelopmentRebuildRetainsPriorMaterializedGenerationOnLayoutFailure(t *testing.T) {
	source, destination := localizedDevelopmentProjectFixture(t)

	base, err := FromDirectory(source, config.Flags{})
	require.NoError(t, err)
	project, _, err := BuildLocalizedDevelopmentProject(base)
	require.NoError(t, err)

	require.NoError(t, os.Remove(filepath.Join(source, "_layouts", "default.html")))
	replacement, _, err := project.RebuildDevelopment()
	require.Error(t, err)
	require.Nil(t, replacement)
	require.Contains(t, localizedServedContent(t, project, "/guide/"), "layout:start <p>English guide</p>\n layout:end")
	require.Equal(t, "preserve this destination", string(readLocalizedProjectOutput(t, filepath.Join(destination, "sentinel.txt"))))
	require.NoFileExists(t, filepath.Join(destination, "guide", "index.html"))
}

func TestLocalizedProductionProjectStillPublishesDestination(t *testing.T) {
	source, destination := localizedDevelopmentProjectFixture(t)

	base, err := FromDirectory(source, config.Flags{})
	require.NoError(t, err)
	project, _, err := BuildLocalizedProject(base)
	require.NoError(t, err)
	require.NotNil(t, project)

	require.Contains(t, string(readLocalizedProjectOutput(t, filepath.Join(destination, "guide", "index.html"))), "layout:start <p>English guide</p>\n layout:end")
	require.Contains(t, string(readLocalizedProjectOutput(t, filepath.Join(destination, "de", "willkommen", "index.html"))), "layout:start <p>German guide</p>\n layout:end")
	require.Equal(t, "body { color: red; }\n", string(readLocalizedProjectOutput(t, filepath.Join(destination, "assets", "site.css"))))
	require.NoFileExists(t, filepath.Join(destination, "sentinel.txt"))
}

func TestLocalizedProjectRequiredTranslationsFollowInclusionSettings(t *testing.T) {
	enabled := true
	tests := []struct {
		name              string
		config            string
		relativePath      string
		contents          string
		flags             config.Flags
		translationKey    string
		wantMissingTarget bool
	}{
		{
			name:         "excluded",
			config:       "exclude: [excluded.md]\n",
			relativePath: "excluded.md",
			contents: `---
lang: en
translation_key: excluded
---
Excluded
`,
		},
		{
			name:         "static",
			relativePath: "static.txt",
			contents:     "Static\n",
		},
		{
			name:         "draft-disabled",
			relativePath: "_drafts/draft.md",
			contents: `---
lang: en
translation_key: draft
---
Draft
`,
		},
		{
			name:         "draft-enabled",
			relativePath: "_drafts/draft.md",
			contents: `---
lang: en
translation_key: draft
---
Draft
`,
			flags:             config.Flags{Drafts: &enabled},
			translationKey:    "draft",
			wantMissingTarget: true,
		},
		{
			name:         "future-disabled",
			relativePath: "_posts/9999-12-31-future.md",
			contents: `---
lang: en
translation_key: future
---
Future
`,
		},
		{
			name:         "future-enabled",
			relativePath: "_posts/9999-12-31-future.md",
			contents: `---
lang: en
translation_key: future
---
Future
`,
			flags:             config.Flags{Future: &enabled},
			translationKey:    "future",
			wantMissingTarget: true,
		},
		{
			name:         "unpublished-disabled",
			relativePath: "unpublished.md",
			contents: `---
lang: en
translation_key: unpublished
published: false
---
Unpublished
`,
		},
		{
			name:         "unpublished-enabled",
			relativePath: "unpublished.md",
			contents: `---
lang: en
translation_key: unpublished
published: false
---
Unpublished
`,
			flags:             config.Flags{Unpublished: &enabled},
			translationKey:    "unpublished",
			wantMissingTarget: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source, _, _ := requiredTranslationProjectFixture(t)
			if test.config != "" {
				filename := filepath.Join(source, "_config.yml")
				contents, err := os.ReadFile(filename)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(filename, append(contents, test.config...), 0o644))
			}
			writeLocalizedProjectFile(t, source, test.relativePath, test.contents)

			base, err := FromDirectory(source, test.flags)
			require.NoError(t, err)
			_, _, err = BuildLocalizedProject(base)
			if test.wantMissingTarget {
				require.ErrorContains(t, err, `translation_key "`+test.translationKey+`" is missing required locale "fr"`)
				return
			}
			require.NoError(t, err)
		})
	}
}

func localizedDevelopmentProjectFixture(t *testing.T) (source, destination string) {
	t.Helper()
	source = t.TempDir()
	destination = filepath.Join(source, "generated")
	writeLocalizedProjectFile(t, source, "_config.yml", `destination: generated
localization:
  default_language: en
  locales:
    en: {tag: en, label: English}
    de: {tag: de, label: Deutsch}
`)
	writeLocalizedProjectFile(t, source, "_layouts/default.html", "layout:start {{ content }} layout:end")
	writeLocalizedProjectFile(t, source, "english.md", `---
lang: en
permalink: /guide/
layout: default
---
English guide
`)
	writeLocalizedProjectFile(t, source, "german.md", `---
lang: de
permalink: /willkommen/
layout: default
---
German guide
`)
	writeLocalizedProjectFile(t, source, "assets/site.css", "body { color: red; }\n")
	writeLocalizedProjectFile(t, source, "generated/sentinel.txt", "preserve this destination")
	return source, destination
}

func localizedServedContent(t *testing.T, project *LocalizedProject, route string) string {
	t.Helper()
	document, found := project.ServedDocument(route)
	require.True(t, found)
	var content bytes.Buffer
	require.NoError(t, document.RenderTo(&content))
	return content.String()
}

func requiredTranslationProjectFixture(t *testing.T) (source, english, french string) {
	t.Helper()
	source = t.TempDir()
	writeLocalizedProjectFile(t, source, "_config.yml", `destination: _site
localization:
  default_language: en
  required_translations: [fr]
  locales:
    en: {tag: en, label: English}
    fr: {tag: fr, label: Français}
`)
	english = filepath.Join(source, "english.md")
	writeLocalizedProjectFile(t, source, "english.md", `---
lang: en
translation_key: guide
generation: first
permalink: /guide/
---
first generation
`)
	french = filepath.Join(source, "french.md")
	writeLocalizedProjectFile(t, source, "french.md", `---
lang: fr
translation_key: guide
permalink: /guide/
---
French guide
`)
	return source, english, french
}

func requireLocalizedGeneration(t *testing.T, project *LocalizedProject, generation string) {
	t.Helper()
	_, document, found := project.URLPage("/guide/")
	require.True(t, found)
	page, ok := document.(Page)
	require.True(t, ok)
	require.Equal(t, generation, page.FrontMatter().String("generation", ""))
}

func readLocalizedProjectOutput(t *testing.T, filename string) []byte {
	t.Helper()
	contents, err := os.ReadFile(filename)
	require.NoError(t, err)
	return contents
}

func writeLocalizedProjectFile(t *testing.T, source, relative, contents string) {
	t.Helper()
	filename := filepath.Join(source, relative)
	require.NoError(t, os.MkdirAll(filepath.Dir(filename), 0o755))
	require.NoError(t, os.WriteFile(filename, []byte(contents), 0o644))
}
