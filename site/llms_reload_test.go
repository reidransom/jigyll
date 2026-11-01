package site

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/stretchr/testify/require"
)

func TestLLMsReloadedPreparesFreshCanonicalGeneration(t *testing.T) {
	changes := []struct {
		name, path, body, want string
	}{
		{"content", "guide.md", "---\ntitle: Revised Guide\npermalink: /guide/\n---\nRevised article\n\n{% include body.md %}\n\n{% include theme.md %}\n\n{{ site.data.metadata.label }}\n\n[Target](../target/)\n", "Revised article"},
		{"include", "_includes/body.md", "Revised include", "Revised include"},
		{"data", "_data/metadata.yml", "label: Revised data\n", "Revised data"},
		{"theme include", "_theme/test-theme/_includes/theme.md", "Revised theme include", "Revised theme include"},
	}
	for _, incremental := range []bool{false, true} {
		for _, change := range changes {
			t.Run(fmt.Sprintf("incremental=%t/%s", incremental, change.name), func(t *testing.T) {
				root := llmsSite(t, "baseurl: /project\ntheme: test-theme\n")
				writeIncludeCacheSiteFile(t, root, "_includes/body.md", "Original include")
				writeIncludeCacheSiteFile(t, root, "_theme/test-theme/_includes/theme.md", "Original theme include")
				writeIncludeCacheSiteFile(t, root, "_data/metadata.yml", "label: Original data\n")
				writeIncludeCacheSiteFile(t, root, "guide.md", "---\ntitle: Guide\npermalink: /guide/\n---\nOriginal article\n\n{% include body.md %}\n\n{% include theme.md %}\n\n{{ site.data.metadata.label }}\n\n[Target](../target/)\n")
				s := buildIncludeCacheSite(t, root, config.Flags{Incremental: &incremental})
				s.SetAbsoluteURL("http://127.0.0.1:4321")
				oldIndex := renderRoute(t, s, "/llms.txt")
				oldBody := renderRoute(t, s, "/guide/index.md")
				oldArticle, err := s.Routes["/guide/"].(Page).Article()
				require.NoError(t, err)
				oldSource := string(oldArticle.Source)

				writeIncludeCacheSiteFile(t, root, change.path, change.body)
				require.True(t, s.RequiresFullReload([]string{change.path}))
				fresh, err := s.ReloadedForEvent(FilesEvent{Paths: []string{change.path}})
				require.NoError(t, err)
				require.NotSame(t, s, fresh)
				fresh.SetAbsoluteURL("http://127.0.0.1:4321")
				index := renderRoute(t, fresh, "/llms.txt")
				body := renderRoute(t, fresh, "/guide/index.md")
				require.Contains(t, body, change.want)
				require.Contains(t, index, "https://example.test/project/guide/index.md")
				require.Contains(t, body, "https://example.test/project/target/")
				require.NotContains(t, index+body, "127.0.0.1")
				if change.name == "content" {
					require.Contains(t, index, "[Revised Guide]")
				}
				require.Equal(t, oldIndex, renderRoute(t, s, "/llms.txt"))
				require.Equal(t, oldBody, renderRoute(t, s, "/guide/index.md"))
				require.Equal(t, oldSource, string(oldArticle.Source))
				// A second refresh must not inherit either serving URL override.
				second, err := fresh.Reloaded([]string{change.path})
				require.NoError(t, err)
				require.NotSame(t, fresh, second)
				require.Equal(t, index, renderRoute(t, second, "/llms.txt"))
				require.Equal(t, body, renderRoute(t, second, "/guide/index.md"))
			})
		}
	}
}

func TestLLMsIncrementalRebuildReplacesInventoryAndObsoleteFiles(t *testing.T) {
	root := llmsSite(t, "incremental: true\nkeep_files: [preserved.txt]\ncollections:\n  guides:\n    output: true\n")
	writeIncludeCacheSiteFile(t, root, "guide.md", "---\ntitle: Original\npermalink: /original/\n---\nOriginal body")
	writeIncludeCacheSiteFile(t, root, "_guides/collection.md", "---\ntitle: Collection\npermalink: /collection/\n---\nCollection body")
	writeIncludeCacheSiteFile(t, root, "authored.txt", "Authored source")
	writeIncludeCacheSiteFile(t, root, "_site/preserved.txt", "Kept destination")
	s := buildIncludeCacheSite(t, root, config.Flags{})
	original := s
	oldIndex := renderRoute(t, s, "/llms.txt")
	oldBody := renderRoute(t, s, "/original/index.md")

	rebuild := func(event FilesEvent) {
		t.Helper()
		previous := s
		fresh, count, err := s.rebuildEvent(event)
		require.NoError(t, err)
		require.Positive(t, count)
		require.NotSame(t, previous, fresh)
		s = fresh
		require.Equal(t, "Authored source", readIncludeCacheSiteOutput(t, root, "authored.txt"))
		require.Equal(t, "Kept destination", readIncludeCacheSiteOutput(t, root, "preserved.txt"))
	}
	absent := func(routes ...string) {
		t.Helper()
		for _, route := range routes {
			_, found := s.URLPage(route)
			require.False(t, found, "obsolete route %s", route)
			require.NoFileExists(t, filepath.Join(s.DestDir(), route))
		}
	}

	writeIncludeCacheSiteFile(t, root, "added.md", "---\ntitle: Added\npermalink: /added/\n---\nAdded body")
	rebuild(FilesEvent{Paths: []string{"added.md"}, Topology: true})
	require.Contains(t, renderRoute(t, s, "/llms.txt"), "[Added]")
	require.Contains(t, readIncludeCacheSiteOutput(t, root, "added/index.md"), "Added body")

	require.NoError(t, os.Rename(filepath.Join(root, "guide.md"), filepath.Join(root, "renamed.md")))
	writeIncludeCacheSiteFile(t, root, "renamed.md", "---\ntitle: Renamed\npermalink: /renamed/\n---\nRenamed body")
	rebuild(FilesEvent{Paths: []string{"guide.md", "renamed.md"}, Topology: true})
	absent("/original/index.md", "/original/index.html")
	require.Contains(t, renderRoute(t, s, "/llms.txt"), "https://example.test/renamed/index.md")

	writeIncludeCacheSiteFile(t, root, "renamed.md", "---\ntitle: Moved\npermalink: /moved/\n---\nMoved body")
	rebuild(FilesEvent{Paths: []string{"renamed.md"}})
	absent("/renamed/index.md", "/renamed/index.html")
	require.Contains(t, readIncludeCacheSiteOutput(t, root, "moved/index.md"), "Moved body")

	writeIncludeCacheSiteFile(t, root, "renamed.md", "---\ntitle: Moved\npermalink: /moved/\npublished: false\n---\nPrivate body")
	rebuild(FilesEvent{Paths: []string{"renamed.md"}})
	absent("/moved/index.md", "/moved/index.html")
	require.NotContains(t, renderRoute(t, s, "/llms.txt"), "[Moved]")

	require.NoError(t, os.Remove(filepath.Join(root, "added.md")))
	rebuild(FilesEvent{Paths: []string{"added.md"}, Topology: true})
	absent("/added/index.md", "/added/index.html")
	require.NotContains(t, renderRoute(t, s, "/llms.txt"), "[Added]")

	configuration, err := os.ReadFile(filepath.Join(root, "_config.yml"))
	require.NoError(t, err)
	writeIncludeCacheSiteFile(t, root, "_config.yml", strings.Replace(string(configuration), "output: true", "output: false", 1))
	rebuild(FilesEvent{Paths: []string{"_config.yml"}})
	absent("/collection/index.md", "/collection/index.html")
	require.NotContains(t, renderRoute(t, s, "/llms.txt"), "[Collection]")
	require.Equal(t, oldIndex, renderRoute(t, original, "/llms.txt"))
	require.Equal(t, oldBody, renderRoute(t, original, "/original/index.md"))
}

func TestLLMsCurationAndDisablementCleanOnlyGeneratedOutputs(t *testing.T) {
	root := llmsSite(t, "incremental: true\nkeep_files: [preserved.txt]\nllms_txt:\n  sections:\n    - title: Chosen\n      entries:\n        - url: /first/\n")
	for _, name := range []string{"first", "second"} {
		writeIncludeCacheSiteFile(t, root, name+".md", "---\ntitle: "+name+"\npermalink: /"+name+"/\n---\n"+name+" body")
	}
	writeIncludeCacheSiteFile(t, root, "authored.txt", "Authored source")
	writeIncludeCacheSiteFile(t, root, "_site/preserved.txt", "Kept destination")
	s := buildIncludeCacheSite(t, root, config.Flags{})
	original := s
	oldIndex := renderRoute(t, s, "/llms.txt")
	oldBody := renderRoute(t, s, "/first/index.md")
	configuration, err := os.ReadFile(filepath.Join(root, "_config.yml"))
	require.NoError(t, err)
	curated := strings.Replace(string(configuration), "url: /first/", "url: /second/\n          title: Curated second", 1)
	writeIncludeCacheSiteFile(t, root, "_config.yml", curated)
	s, _, err = s.rebuildEvent(FilesEvent{Paths: []string{"_config.yml"}})
	require.NoError(t, err)
	require.Contains(t, renderRoute(t, s, "/llms.txt"), "[Curated second]")
	require.NotContains(t, renderRoute(t, s, "/llms.txt"), "[first]")
	require.NoFileExists(t, filepath.Join(root, "_site/first/index.md"))
	require.Contains(t, readIncludeCacheSiteOutput(t, root, "second/index.md"), "second body")

	writeIncludeCacheSiteFile(t, root, "_config.yml", strings.Replace(curated, "llms_txt:\n", "llms_txt:\n  markdown: false\n", 1))
	s, _, err = s.rebuildEvent(FilesEvent{Paths: []string{"_config.yml"}})
	require.NoError(t, err)
	require.Contains(t, renderRoute(t, s, "/llms.txt"), "https://example.test/second/")
	require.NoFileExists(t, filepath.Join(root, "_site/second/index.md"))

	// Disabling removes the old generated index; authored files and keep_files survive.
	disabled := strings.Replace(curated, "plugins: [jigyll-llms-txt]", "plugins: []", 1)
	writeIncludeCacheSiteFile(t, root, "_config.yml", disabled)
	s, _, err = s.rebuildEvent(FilesEvent{Paths: []string{"_config.yml"}})
	require.NoError(t, err)
	require.NotContains(t, s.Routes, "/llms.txt")
	require.NoFileExists(t, filepath.Join(root, "_site/llms.txt"))
	require.Equal(t, "Authored source", readIncludeCacheSiteOutput(t, root, "authored.txt"))
	require.Equal(t, "Kept destination", readIncludeCacheSiteOutput(t, root, "preserved.txt"))
	require.Equal(t, oldIndex, renderRoute(t, original, "/llms.txt"))
	require.Equal(t, oldBody, renderRoute(t, original, "/first/index.md"))

	writeIncludeCacheSiteFile(t, root, "llms.txt", "Authored discovery")
	fresh, _, err := s.rebuildEvent(FilesEvent{Paths: []string{"llms.txt"}, Topology: true})
	require.NoError(t, err)
	require.True(t, fresh.Routes["/llms.txt"].IsStatic())
	require.Equal(t, "Authored discovery", readIncludeCacheSiteOutput(t, root, "llms.txt"))

	// Membership, not ignored llms_txt settings, enables complete export refresh.
	require.NoError(t, os.Remove(filepath.Join(root, "llms.txt")))
	writeIncludeCacheSiteFile(t, root, "_config.yml", curated)
	enabled, _, err := fresh.rebuildEvent(FilesEvent{Paths: []string{"_config.yml", "llms.txt"}, Topology: true})
	require.NoError(t, err)
	require.Contains(t, renderRoute(t, enabled, "/llms.txt"), "[Curated second]")
	require.Contains(t, readIncludeCacheSiteOutput(t, root, "second/index.md"), "second body")
}

func TestLLMsFailedCandidateLeavesPreparedGenerationAndDiskUntouched(t *testing.T) {
	for _, failure := range []struct{ name, path, body string }{
		{"metadata", "guide.md", "---\ntitle: [invalid]\npermalink: /guide/\n---\nInvalid metadata body"},
		{"render", "_includes/body.md", "{% include missing.md %}"},
		{"collision", "guide/index.md", "Authored collision"},
	} {
		t.Run(failure.name, func(t *testing.T) {
			root := llmsSite(t, "incremental: true\n")
			writeIncludeCacheSiteFile(t, root, "_includes/body.md", "Successful include")
			writeIncludeCacheSiteFile(t, root, "guide.md", "---\ntitle: Guide\npermalink: /guide/\n---\nSuccessful body\n\n{% include body.md %}")
			s := buildIncludeCacheSite(t, root, config.Flags{})
			index := renderRoute(t, s, "/llms.txt")
			body := renderRoute(t, s, "/guide/index.md")
			writeIncludeCacheSiteFile(t, root, "_site/sentinel", "Last successful destination")
			writeIncludeCacheSiteFile(t, root, failure.path, failure.body)
			_, _, err := s.rebuildEvent(FilesEvent{Paths: []string{failure.path}})
			require.Error(t, err)
			require.Equal(t, index, renderRoute(t, s, "/llms.txt"))
			require.Equal(t, body, renderRoute(t, s, "/guide/index.md"))
			require.Equal(t, index, readIncludeCacheSiteOutput(t, root, "llms.txt"))
			require.Equal(t, body, readIncludeCacheSiteOutput(t, root, "guide/index.md"))
			require.Equal(t, "Last successful destination", readIncludeCacheSiteOutput(t, root, "sentinel"))
		})
	}
}

func TestSiteRenderFailureRemainsCachedBeforeCleaning(t *testing.T) {
	root := t.TempDir()
	writeIncludeCacheSiteFile(t, root, "guide.md", "---\ntitle: Guide\n---\n{% include missing.md %}")
	writeIncludeCacheSiteFile(t, root, "_site/sentinel", "Last successful destination")
	s, err := FromDirectory(root, config.Flags{})
	require.NoError(t, err)
	require.NoError(t, s.Read())
	for range 2 {
		_, err := s.Write()
		require.ErrorContains(t, err, "missing.md")
		require.Equal(t, "Last successful destination", readIncludeCacheSiteOutput(t, root, "sentinel"))
	}
}

func TestLLMsConcurrentWritesPreservePreparedSourceAndArtifacts(t *testing.T) {
	root := llmsSite(t, "incremental: true\n")
	writeIncludeCacheSiteFile(t, root, "_includes/body.md", "Prepared include")
	writeIncludeCacheSiteFile(t, root, "guide.md", "---\ntitle: Guide\nlayout: default\npermalink: /guide/\n---\n{% include body.md %}\n\n[Escaped](../target\\(one\\)/)\n\n[Reference][target]\n\n[target]: ../reference\\(two\\)/\n\n`[Literal](../unchanged)`\n")
	s := buildIncludeCacheSite(t, root, config.Flags{})
	article, err := s.Routes["/guide/"].(Page).Article()
	require.NoError(t, err)
	source := string(article.Source)
	routes := []string{"/guide/", "/llms.txt", "/guide/index.md"}
	expected := make(map[string]string, len(routes))
	for _, route := range routes {
		expected[route] = renderRoute(t, s, route)
	}
	require.Contains(t, expected["/guide/index.md"], "https://example.test/target(one)/")
	start := make(chan struct{})
	var writers sync.WaitGroup
	failures := make(chan error, 16)
	for range cap(failures) {
		writers.Add(1)
		go func() {
			defer writers.Done()
			<-start
			for range 8 {
				for _, route := range routes {
					var output bytes.Buffer
					if err := s.WriteDocument(&output, s.Routes[route]); err != nil {
						failures <- err
						return
					}
					if output.String() != expected[route] {
						failures <- fmt.Errorf("concurrent output changed for %s", route)
						return
					}
				}
			}
		}()
	}
	close(start)
	writeIncludeCacheSiteFile(t, root, "_includes/body.md", "Next include")
	fresh, err := s.ReloadedForEvent(FilesEvent{Paths: []string{"_includes/body.md"}})
	require.NoError(t, err)
	require.Contains(t, renderRoute(t, fresh, "/guide/index.md"), "Next include")
	writers.Wait()
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
	require.Equal(t, source, string(article.Source))
	require.Contains(t, source, `../target\(one\)/`)
	for _, route := range routes {
		require.Equal(t, expected[route], renderRoute(t, s, route))
	}
}

func TestLLMsThemeIncludeUsesExistingWatchInventories(t *testing.T) {
	// Start without the plugin: its later enablement must not require a second
	// watcher in order to observe an already configured theme's templates.
	root := llmsSite(t, "incremental: true\ntheme: test-theme\n")
	configuration, err := os.ReadFile(filepath.Join(root, "_config.yml"))
	require.NoError(t, err)
	writeIncludeCacheSiteFile(t, root, "_config.yml", strings.Replace(string(configuration), "plugins: [jigyll-llms-txt]", "plugins: []", 1))
	themePath := filepath.Join("_theme", "test-theme", "_includes", "body.md")
	writeIncludeCacheSiteFile(t, root, themePath, "Theme before")
	writeIncludeCacheSiteFile(t, root, "guide.md", "---\ntitle: Guide\npermalink: /guide/\n---\n{% include body.md %}")
	watched := buildIncludeCacheSite(t, root, config.Flags{})
	inventory, err := watched.scanWatchInventory()
	require.NoError(t, err)
	paths, topology := inventory.event(filepath.Join(root, themePath))
	require.False(t, topology)
	require.Equal(t, []string{themePath}, watched.affectsBuildFilter(paths))
	// Ordinary incremental invalidation is deliberately unchanged.
	require.False(t, watched.RequiresFullReload(paths))
	before, err := watched.pollSnapshot()
	require.NoError(t, err)
	require.Contains(t, before, themePath)

	writeIncludeCacheSiteFile(t, root, "_config.yml", string(configuration))
	enabled, err := watched.ReloadedForEvent(FilesEvent{Paths: []string{"_config.yml"}})
	require.NoError(t, err)
	require.Contains(t, renderRoute(t, enabled, "/guide/index.md"), "Theme before")
	writeIncludeCacheSiteFile(t, root, themePath, "Theme after with distinguishable length")
	after, err := watched.pollSnapshot()
	require.NoError(t, err)
	event := diffPollSnapshots(before, after)
	require.Contains(t, event.Paths, themePath)
	require.True(t, enabled.RequiresFullReload(paths))
	fresh, _, err := enabled.rebuildEvent(FilesEvent{Paths: paths})
	require.NoError(t, err)
	require.Contains(t, renderRoute(t, fresh, "/guide/index.md"), "Theme after with distinguishable length")
	require.Contains(t, readIncludeCacheSiteOutput(t, root, "guide/index.md"), "Theme after with distinguishable length")
	require.Contains(t, renderRoute(t, enabled, "/guide/index.md"), "Theme before")

	// The exception is scoped to the configured theme, not every nested
	// underscore directory in the source tree.
	writeIncludeCacheSiteFile(t, root, "unrelated/_private/hidden.md", "Not watched")
	snapshot, err := watched.pollSnapshot()
	require.NoError(t, err)
	require.NotContains(t, snapshot, filepath.Join("unrelated", "_private", "hidden.md"))
}

func TestLLMsDisablementHonorsKeptExportsAndAuthoredReplacement(t *testing.T) {
	root := llmsSite(t, "incremental: true\nkeep_files: [guide/index.md]\n")
	writeIncludeCacheSiteFile(t, root, "guide.md", "---\ntitle: Guide\npermalink: /guide/\n---\nPrepared guide body")
	s := buildIncludeCacheSite(t, root, config.Flags{})
	body := renderRoute(t, s, "/guide/index.md")
	configuration, err := os.ReadFile(filepath.Join(root, "_config.yml"))
	require.NoError(t, err)
	writeIncludeCacheSiteFile(t, root, "_config.yml", strings.Replace(string(configuration), "plugins: [jigyll-llms-txt]", "plugins: []", 1))
	writeIncludeCacheSiteFile(t, root, "llms.txt", "Authored replacement")
	disabled, _, err := s.rebuildEvent(FilesEvent{Paths: []string{"_config.yml", "llms.txt"}, Topology: true})
	require.NoError(t, err)
	require.NotContains(t, disabled.Routes, "/guide/index.md")
	require.True(t, disabled.Routes["/llms.txt"].IsStatic())
	require.Equal(t, "Authored replacement", readIncludeCacheSiteOutput(t, root, "llms.txt"))
	require.Equal(t, body, readIncludeCacheSiteOutput(t, root, "guide/index.md"))
	require.FileExists(t, filepath.Join(root, "guide.md"))
	require.Equal(t, body, renderRoute(t, s, "/guide/index.md"))
}

func TestLLMsReloadedUsesChangedCanonicalConfiguration(t *testing.T) {
	root := llmsSite(t, "incremental: true\nbaseurl: /before\n")
	writeIncludeCacheSiteFile(t, root, "guide.md", "---\ntitle: Guide\npermalink: /guide/\n---\n[Target](../target/)")
	s := buildIncludeCacheSite(t, root, config.Flags{})
	s.SetAbsoluteURL("http://127.0.0.1:4321")
	oldIndex := renderRoute(t, s, "/llms.txt")
	oldBody := renderRoute(t, s, "/guide/index.md")
	configuration, err := os.ReadFile(filepath.Join(root, "_config.yml"))
	require.NoError(t, err)
	changed := strings.Replace(string(configuration), "https://example.test", "https://reconfigured.test", 1)
	changed = strings.Replace(changed, "baseurl: /before", "baseurl: /after", 1)
	writeIncludeCacheSiteFile(t, root, "_config.yml", changed)
	fresh, err := s.ReloadedForEvent(FilesEvent{Paths: []string{"_config.yml"}})
	require.NoError(t, err)
	fresh.SetAbsoluteURL("http://127.0.0.1:4321")
	require.Contains(t, renderRoute(t, fresh, "/llms.txt"), "https://reconfigured.test/after/guide/index.md")
	require.Contains(t, renderRoute(t, fresh, "/guide/index.md"), "https://reconfigured.test/after/target/")
	require.Equal(t, oldIndex, renderRoute(t, s, "/llms.txt"))
	require.Equal(t, oldBody, renderRoute(t, s, "/guide/index.md"))

	// Canonical validation still applies after the serving override.
	writeIncludeCacheSiteFile(t, root, "_config.yml", strings.Replace(changed, "https://reconfigured.test", "not-an-absolute-origin", 1))
	_, err = fresh.ReloadedForEvent(FilesEvent{Paths: []string{"_config.yml"}})
	require.Error(t, err)
	require.Contains(t, renderRoute(t, fresh, "/llms.txt"), "https://reconfigured.test/after/guide/index.md")
}

func TestLLMsDoesNotRelaxDocumentationIncrementalRestriction(t *testing.T) {
	root := llmsSite(t, "incremental: true\ndocumentation:\n  sections: docs_sections\n")
	writeLLMsDocs(t, root)
	s, err := FromDirectory(root, config.Flags{})
	require.NoError(t, err)
	require.ErrorContains(t, s.Read(), "documentation snapshots require full builds; disable incremental")
}
