package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func workspace(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp(t.TempDir(), "run-")
	require.NoError(t, err)
	put(t, filepath.Join(root, ".perf-owned"), marker)
	return root
}
func put(t *testing.T, path, text string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte(text), 0600))
}
func TestValidate(t *testing.T) {
	for _, p := range []string{"default", "single", "no-cache", "single-no-cache", "all"} {
		require.NoError(t, validate("1", "0", p))
	}
	for _, a := range [][3]string{{"0", "0", "default"}, {"-1", "2", "default"}, {"1", "-1", "default"}, {"1", "0", "nope"}, {"1.5", "0", "all"}, {"1", "0x2", "all"}, {"999999999999999999999999", "0", "all"}} {
		require.Error(t, validate(a[0], a[1], a[2]))
	}
}
func TestCopyPreservesSourceAndExcludesConfiguredArtifacts(t *testing.T) {
	source := t.TempDir()
	root := workspace(t)
	config := "destination: custom-output\ncache_dir: custom-cache\nsass:\n  cache_location: custom-sass\nexclude: [private-data]\n"
	put(t, filepath.Join(source, "_config.yml"), config)
	put(t, filepath.Join(source, "index.html"), "original")
	for _, p := range []string{"custom-output/a", "custom-cache/a", "custom-sass/a", "private-data/a", ".git/config", "vendor/gems/a", "node_modules/a", ".jekyll-metadata"} {
		put(t, filepath.Join(source, p), "sentinel")
	}
	before, err := files(source)
	require.NoError(t, err)
	require.NoError(t, copySource(source, root))
	after, err := files(source)
	require.NoError(t, err)
	require.Equal(t, before, after)
	for _, p := range before {
		b, err := os.ReadFile(filepath.Join(source, p))
		require.NoError(t, err)
		switch p {
		case "_config.yml":
			require.Equal(t, config, string(b))
		case "index.html":
			require.Equal(t, "original", string(b))
		default:
			require.Equal(t, "sentinel", string(b))
		}
	}
	copied, err := files(filepath.Join(root, "source"))
	require.NoError(t, err)
	require.Equal(t, []string{"_config.yml", "index.html"}, copied)
	require.Error(t, copySource(source, root)) // no replacement/reuse
}
func TestCopyRejectsExternalAndInternalSymlinks(t *testing.T) {
	for _, external := range []bool{true, false} {
		source := t.TempDir()
		root := workspace(t)
		target := filepath.Join(source, "target")
		if external {
			target = filepath.Join(t.TempDir(), "target")
		}
		put(t, target, "untouched")
		require.NoError(t, os.Symlink(target, filepath.Join(source, "link")))
		require.ErrorContains(t, copySource(source, root), "unsupported source symlink")
		b, err := os.ReadFile(target)
		require.NoError(t, err)
		require.Equal(t, "untouched", string(b))
	}
}
func TestResetOwnershipBoundaries(t *testing.T) {
	root := workspace(t)
	put(t, filepath.Join(root, "source", "sentinel"), "source")
	for _, p := range []string{"output/a", "cache/a", "source/.jekyll-cache/a"} {
		put(t, filepath.Join(root, p), "discard")
	}
	require.NoError(t, reset(root))
	require.FileExists(t, filepath.Join(root, "source", "sentinel"))
	require.NoDirExists(t, filepath.Join(root, "source", ".jekyll-cache"))
	external := t.TempDir()
	put(t, filepath.Join(external, "sentinel"), "external")
	require.NoError(t, os.Symlink(external, filepath.Join(root, "cache")))
	require.ErrorContains(t, reset(root), "symlink cleanup target")
	require.FileExists(t, filepath.Join(external, "sentinel"))
	require.Error(t, reset(external))
	alias := filepath.Join(t.TempDir(), "run-alias")
	require.NoError(t, os.Symlink(root, alias))
	require.ErrorContains(t, reset(alias), "unsafe workspace")
	require.NoError(t, os.WriteFile(filepath.Join(root, ".perf-owned"), []byte("wrong"), 0600))
	require.Error(t, reset(root))
}
func manifestFixture(t *testing.T) (string, string) {
	source, output := t.TempDir(), t.TempDir()
	put(t, filepath.Join(source, "index.html"), "input")
	sum := sha256.Sum256([]byte("input"))
	m := manifest{Schema: 1, Revision: "2", Future: true, Timezone: "Etc/UTC", Checks: map[string][]string{"index": {"<header>Include</header>", "<main>"}}}
	m.Parameters.Includes = 10
	m.Parameters.Layouts = 4
	m.Inputs = append(m.Inputs, struct{ Path, SHA256 string }{"index.html", hex.EncodeToString(sum[:])})
	m.Outputs = append(m.Outputs, struct {
		Kind, Source, URL, Path, Title string
		Text, Links                    []string
	}{"index", "index.html", "/", "index.html", "Home", []string{"Rendered body"}, []string{"/"}})
	b, err := json.Marshal(m)
	require.NoError(t, err)
	put(t, filepath.Join(source, "_benchmark/manifest.json"), string(b))
	put(t, filepath.Join(output, "index.html"), "<html><head><title>Home</title></head><body><header>Include</header><main>Rendered body <a href=\"/\">Home</a></main></body></html>")
	return source, output
}
func TestManifestRejectsMissingURLAndBrokenRenderedInclude(t *testing.T) {
	source, output := manifestFixture(t)
	require.NoError(t, verifyManifest(source, output))
	p := filepath.Join(output, "index.html")
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	require.NoError(t, os.Remove(p))
	require.ErrorContains(t, verifyManifest(source, output), "missing expected URL")
	put(t, p, strings.ReplaceAll(string(b), "<header>Include</header>", ""))
	require.ErrorContains(t, verifyManifest(source, output), "broken layout/include")
	put(t, p, strings.ReplaceAll(string(b), "Rendered body", "wrong"))
	require.ErrorContains(t, verifyManifest(source, output), "missing rendered body")
	put(t, p, string(b))
	put(t, filepath.Join(source, "index.html"), "corrupted")
	require.ErrorContains(t, verifyManifest(source, output), "hash mismatch")
}
func TestRealComparisonRejectsIncompleteAndDifferentWork(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	put(t, filepath.Join(a, "index.html"), "<title>Home</title><p>Hello <strong>world</strong></p><a href=\"/a\">A</a>")
	put(t, filepath.Join(b, "index.html"), "<title>Home</title><p>Hello <b>world</b></p><a href=\"/a\">A</a>")
	require.NoError(t, compare(a, b))
	put(t, filepath.Join(b, "index.html"), "<title>Home</title><p>Hello wrong</p><a href=\"/a\">A</a>")
	require.ErrorContains(t, compare(a, b), "unsupported rendered content/navigation difference")
	put(t, filepath.Join(b, "extra.html"), "extra")
	require.ErrorContains(t, compare(a, b), "output path differences")
}

func TestManifestFragmentsAllowRendererAttributesNotMissingMarkup(t *testing.T) {
	want := htmlTokens([]byte("<code>code</code>"))
	require.True(t, hasFragment(htmlTokens([]byte(`<code class="language-plaintext highlighter-rouge">code</code>`)), want))
	require.False(t, hasFragment(htmlTokens([]byte("code")), want))
	require.False(t, hasFragment(htmlTokens([]byte("<code>wrong</code>")), want))
	require.False(t, hasFragment(htmlTokens([]byte(`<div class="wrong">`)), htmlTokens([]byte(`<div class="post-content">`))))
}

func TestRealCSSComparison(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	for _, root := range []string{a, b} {
		put(t, filepath.Join(root, "index.html"), "<p>Same content</p>")
	}
	put(t, filepath.Join(a, "main.css"), "body{color:#123456}body nav{padding:2px}")
	put(t, filepath.Join(b, "main.css"), "body { color: #123456; }\nbody nav { padding: 2px; }\n")
	require.NoError(t, compare(a, b))
	put(t, filepath.Join(b, "main.css"), "body { color: red; }")
	require.ErrorContains(t, compare(a, b), "rendered CSS difference")
}

func TestCopyRetainsExcludedBuildInputs(t *testing.T) {
	source, root := t.TempDir(), workspace(t)
	put(t, filepath.Join(source, "_config.yml"), "exclude: [Gemfile, Gemfile.lock, _config.yml]\n")
	put(t, filepath.Join(source, "Gemfile"), "locked dependency input")
	put(t, filepath.Join(source, "Gemfile.lock"), "locked versions")
	require.NoError(t, copySource(source, root))
	for _, name := range []string{"_config.yml", "Gemfile", "Gemfile.lock"} {
		require.FileExists(t, filepath.Join(root, "source", name))
	}
}
