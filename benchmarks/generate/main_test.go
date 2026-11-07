package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func readManifest(t *testing.T, dir string) Manifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, manifestPath))
	require.NoError(t, err)
	var manifest Manifest
	require.NoError(t, json.Unmarshal(data, &manifest))
	return manifest
}

func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	require.NoError(t, filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	}))
	return files
}

func TestGeneratedFixtureIsReproducible(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "first"), filepath.Join(root, "second")
	require.NoError(t, generate(first, 7, 3))
	require.NoError(t, generate(second, 7, 3))
	a, b := snapshot(t, first), snapshot(t, second)
	require.Equal(t, a, b, "all source bytes and manifest must be independent of location and generation time")
	require.Len(t, a, 7+3+1+1+10+4+1)
	require.NotContains(t, a["_includes/footer.html"], "site.time", "output must not depend on the build clock")

	manifest := readManifest(t, first)
	require.Equal(t, 1, manifest.SchemaVersion)
	require.Equal(t, upstreamRevision, manifest.UpstreamRevision)
	require.Equal(t, Parameters{7, 3, 10, 4}, manifest.Parameters)
	require.Len(t, manifest.Inputs, len(a)-1)
	previous := ""
	for _, input := range manifest.Inputs {
		require.Greater(t, input.Path, previous, "source hashes must be in canonical path order")
		previous = input.Path
		content, exists := a[input.Path]
		require.True(t, exists, input.Path)
		hash := sha256.Sum256([]byte(content))
		require.Equal(t, hex.EncodeToString(hash[:]), input.SHA256, input.Path)
		require.NotEqual(t, manifestPath, input.Path, "manifest must not hash itself")
	}
}

func TestExpectedURLsAndContent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "site")
	require.NoError(t, generate(dir, 3, 2))
	manifest := readManifest(t, dir)
	require.Len(t, manifest.Outputs, 6)
	want := []struct{ source, url, path, title string }{
		{"_posts/2020-01-01-post-0000.md", "/tech/2020/01/01/post-0000/", "tech/2020/01/01/post-0000/index.html", "Post Number 0: Exploring Tech Topics"},
		{"_posts/2020-01-02-post-0001.md", "/science/2020/01/02/post-0001/", "science/2020/01/02/post-0001/index.html", "Post Number 1: Exploring Science Topics"},
		{"_posts/2020-01-03-post-0002.md", "/travel/2020/01/03/post-0002/", "travel/2020/01/03/post-0002/index.html", "Post Number 2: Exploring Travel Topics"},
		{"page-000.md", "/page-000.html", "page-000.html", "Page 0"},
		{"page-001.md", "/page-001.html", "page-001.html", "Page 1"},
		{"index.html", "/", "index.html", "Home"},
	}
	files := snapshot(t, dir)
	seen := map[string]bool{}
	for i, expected := range want {
		output := manifest.Outputs[i]
		require.Equal(t, expected.source, output.Source)
		require.Equal(t, expected.url, output.URL)
		require.Equal(t, expected.path, output.Path)
		require.Equal(t, expected.title, output.Title)
		require.False(t, seen[output.Path], "output paths must be unique")
		seen[output.Path] = true
		require.Contains(t, files[output.Source], "title: ")
		require.Contains(t, files[output.Source], output.Title)
		require.NotEmpty(t, output.Text)
		for _, text := range output.Text {
			source := files[output.Source]
			if output.Kind == "index" {
				source += files["_config.yml"]
			}
			require.Contains(t, source, text)
		}
		require.Contains(t, output.Links, "/")
		require.Contains(t, output.Links, "/page-000.html")
		require.NotEmpty(t, manifest.Checks[output.Kind])
	}
	require.Contains(t, manifest.Outputs[5].Links, want[0].url)
	require.Contains(t, manifest.Outputs[5].Links, want[2].url)
	require.Contains(t, manifest.Checks["post"], `<div class="related-posts">`)
	require.Contains(t, manifest.Checks["post"], "<strong>bold</strong>")
	require.Contains(t, manifest.Checks["page"], `<nav class="toc">`)
}

func TestNestedWorkloadAndSharedConfiguration(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "site")
	require.NoError(t, generate(dir, 2, 1))
	files := snapshot(t, dir)
	var cfg struct {
		Future      bool   `yaml:"future"`
		Timezone    string `yaml:"timezone"`
		Incremental bool   `yaml:"incremental"`
		Permalink   string `yaml:"permalink"`
		Defaults    []struct {
			Scope  map[string]string `yaml:"scope"`
			Values map[string]string `yaml:"values"`
		} `yaml:"defaults"`
	}
	require.NoError(t, yaml.UnmarshalStrict([]byte(files["_config.yml"]), &map[string]any{}))
	require.NoError(t, yaml.Unmarshal([]byte(files["_config.yml"]), &cfg))
	require.True(t, cfg.Future, "future posts must be built by both engines")
	require.Equal(t, "Etc/UTC", cfg.Timezone)
	require.False(t, cfg.Incremental)
	require.Contains(t, files["_config.yml"], "incremental: false")
	require.Equal(t, "/:categories/:year/:month/:day/:title/", cfg.Permalink)
	require.Equal(t, "posts", cfg.Defaults[0].Scope["type"])
	require.Equal(t, "post", cfg.Defaults[0].Values["layout"])
	for _, edge := range []struct{ child, parent string }{{"post", "default"}, {"page", "default"}, {"default", "base"}} {
		require.Contains(t, files["_layouts/"+edge.child+".html"], "layout: "+edge.parent)
	}
	for _, name := range []string{"header", "footer", "sidebar", "breadcrumbs", "post-meta", "tag-list", "related-posts", "social-share", "pagination", "toc"} {
		require.NotEmpty(t, files["_includes/"+name+".html"])
		referenced := false
		for _, layout := range layouts {
			referenced = referenced || strings.Contains(layout, "{% include "+name+".html %}")
		}
		require.True(t, referenced, "include %s must participate in the layout graph", name)
	}
	require.Contains(t, files["_posts/2020-01-01-post-0000.md"], "{{ page.title }}")
	require.Contains(t, files["_posts/2020-01-01-post-0000.md"], "{% for tag in page.tags %}")
	require.Contains(t, files["page-000.md"], "toc:")
	require.Contains(t, files["_includes/header.html"], "{{ p.url }}")
	require.Contains(t, files["index.html"], "{% for post in site.posts %}")
	manifest := readManifest(t, dir)
	require.Equal(t, cfg.Future, manifest.Future)
	require.Equal(t, cfg.Timezone, manifest.Timezone)
	require.Equal(t, cfg.Incremental, manifest.Incremental)
}

func TestDatedPostBoundaries(t *testing.T) {
	for _, test := range []struct {
		index     int
		date, url string
	}{
		{0, "2020-01-01", "/tech/2020/01/01/post-0000/"},
		{58, "2020-02-28", "/food/2020/02/28/post-0058/"},
		{59, "2020-02-29", "/music/2020/02/29/post-0059/"},
		{60, "2020-03-01", "/tech/2020/03/01/post-0060/"},
		{365, "2020-12-31", "/tech/2020/12/31/post-0365/"},
		{366, "2021-01-01", "/science/2021/01/01/post-0366/"},
		{2556, "2026-12-31", "/science/2026/12/31/post-2556/"},
		{2557, "2027-01-01", "/travel/2027/01/01/post-2557/"},
	} {
		t.Run(test.date, func(t *testing.T) {
			require.Equal(t, test.date, postAt(test.index).Date)
			manifest := newManifest(test.index+1, 0)
			require.True(t, manifest.Future, "coverage cannot depend on today's date")
			require.Equal(t, test.url, manifest.Outputs[test.index].URL)
			require.Contains(t, manifest.Outputs[test.index].Source, test.date)
		})
	}
	manifest := newManifest(defaultPosts, defaultPages)
	require.Len(t, manifest.Outputs, 2021)
	require.Equal(t, "_posts/2025-06-22-post-1999.md", manifest.Outputs[1999].Source)
	// Exercise the four-digit slug/date boundary without allocating 10,001 files.
	large := newManifest(10001, 0)
	require.Contains(t, large.Outputs[10000].URL, "/post-10000/")
	last := int((time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC).Unix() - firstPostDate.Unix()) / 86400)
	require.Equal(t, "9999-12-31", postAt(last).Date)
}

func TestExistingPathsAreRefusedAndPreserved(t *testing.T) {
	root := t.TempDir()
	sentinel := filepath.Join(root, "sentinel")
	require.NoError(t, os.WriteFile(sentinel, []byte("user content"), 0o600))
	before := snapshot(t, root)
	require.ErrorContains(t, generate(root, 2, 1), "existing paths are never replaced")
	require.Equal(t, before, snapshot(t, root))
	require.Error(t, generate(sentinel, 2, 1), "existing regular file must not be replaced")
	empty := filepath.Join(root, "empty")
	require.NoError(t, os.Mkdir(empty, 0o700))
	require.Error(t, generate(empty, 2, 1), "even an empty existing directory is not owned")
	link := filepath.Join(root, "link")
	require.NoError(t, os.Symlink(root, link))
	require.Error(t, generate(link, 2, 1), "existing symlinks must never be followed")
	data, err := os.ReadFile(sentinel)
	require.NoError(t, err)
	require.Equal(t, "user content", string(data))
}

func TestInvalidParametersDoNotAllocate(t *testing.T) {
	for _, test := range []struct{ posts, pages int }{{-1, 0}, {0, -1}, {int(^uint(0) >> 1), 0}} {
		dir := filepath.Join(t.TempDir(), "site")
		require.Error(t, generate(dir, test.posts, test.pages))
		_, err := os.Lstat(dir)
		require.True(t, os.IsNotExist(err))
	}
	missingParent := filepath.Join(t.TempDir(), "missing", "site")
	require.Error(t, generate(missingParent, 1, 1), "the harness must allocate the parent first")
}

func TestZeroCountsAndCLI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	dir := filepath.Join(t.TempDir(), "site with spaces")
	require.NoError(t, run([]string{"-output", dir, "-posts", "0", "-pages", "0"}, &stdout, &stderr))
	manifest := readManifest(t, dir)
	require.Len(t, manifest.Outputs, 1)
	require.Equal(t, "/", manifest.Outputs[0].URL)
	require.Equal(t, []string{"/"}, manifest.Outputs[0].Links)
	require.Contains(t, stdout.String(), "future: true")
	require.Contains(t, stdout.String(), manifestPath)
	for _, args := range [][]string{nil, {"-output", dir, "unexpected"}, {"-unknown"}, {"-posts", "not-a-number"}} {
		require.Error(t, run(args, &stdout, &stderr))
	}
	require.Error(t, run([]string{"-output", dir}, &stdout, &stderr))
}
