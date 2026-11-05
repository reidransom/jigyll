package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScaffoldNewSiteDefault(t *testing.T) {
	target := filepath.Join(t.TempDir(), "nested", "plain-site")

	require.NoError(t, scaffoldNewSite(target, ""))

	for _, path := range []string{
		"_config.yml",
		"_layouts/default.html",
		"404.html",
		"index.md",
		".gitignore",
	} {
		require.FileExists(t, filepath.Join(target, path))
	}
	for _, path := range []string{"_data", "_drafts", "_includes", "_layouts", "_posts", "_sass", "assets"} {
		info, err := os.Stat(filepath.Join(target, path))
		require.NoError(t, err)
		require.True(t, info.IsDir())
	}

	config, err := os.ReadFile(filepath.Join(target, "_config.yml"))
	require.NoError(t, err)
	require.NotContains(t, string(config), "theme:")

	require.NoError(t, ParseAndRun([]string{"build", "-s", target, "-q"}))
	home, err := os.ReadFile(filepath.Join(target, "_site", "index.html"))
	require.NoError(t, err)
	require.Contains(t, string(home), "Welcome to Jigyll")
	notFound, err := os.ReadFile(filepath.Join(target, "_site", "404.html"))
	require.NoError(t, err)
	require.Contains(t, string(notFound), "<h1>404</h1>")
}

func TestParseAndRunNew(t *testing.T) {
	target := filepath.Join(t.TempDir(), "new-site")

	require.NoError(t, ParseAndRun([]string{"new", target}))
	require.FileExists(t, filepath.Join(target, "_config.yml"))
}

func TestScaffoldNewSiteRefusesNonEmptyTarget(t *testing.T) {
	target := filepath.Join(t.TempDir(), "site")
	require.NoError(t, os.Mkdir(target, 0o755))
	original := []byte("do not change\n")
	require.NoError(t, os.WriteFile(filepath.Join(target, "keep.txt"), original, 0o644))

	err := scaffoldNewSite(target, "")
	require.Error(t, err)

	actual, readErr := os.ReadFile(filepath.Join(target, "keep.txt"))
	require.NoError(t, readErr)
	require.Equal(t, original, actual)
	assertNoStagingDirectory(t, filepath.Dir(target), filepath.Base(target))
}

func TestScaffoldNewSiteCleansUpFailedTheme(t *testing.T) {
	t.Run("invalid URL", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "invalid-theme")

		err := scaffoldNewSite(target, "https://github.com/")
		require.Error(t, err)
		require.ErrorIs(t, statError(target), os.ErrNotExist)
		assertNoStagingDirectory(t, filepath.Dir(target), filepath.Base(target))
	})

	t.Run("theme lacks default layout", func(t *testing.T) {
		repository := newThemeRepository(t, "")
		target := filepath.Join(t.TempDir(), "missing-layout")

		err := scaffoldNewSite(target, repository)
		require.Error(t, err)
		require.ErrorIs(t, statError(target), os.ErrNotExist)
		assertNoStagingDirectory(t, filepath.Dir(target), filepath.Base(target))
	})
}

func TestScaffoldNewSiteTheme(t *testing.T) {
	repository := newThemeRepository(t, "<!doctype html><main>Theme layout {{ content }}</main>\n")
	target := filepath.Join(t.TempDir(), "themed-site")

	require.NoError(t, scaffoldNewSite(target, repository))

	config, err := os.ReadFile(filepath.Join(target, "_config.yml"))
	require.NoError(t, err)
	require.Contains(t, string(config), "theme: starter-theme\n")
	_, err = os.Stat(filepath.Join(target, "_layouts", "default.html"))
	require.True(t, errors.Is(err, os.ErrNotExist))
	require.FileExists(t, filepath.Join(target, "_theme", "starter-theme", "_layouts", "default.html"))
	require.FileExists(t, filepath.Join(target, "404.html"))

	require.NoError(t, ParseAndRun([]string{"build", "-s", target, "-q"}))
	home, err := os.ReadFile(filepath.Join(target, "_site", "index.html"))
	require.NoError(t, err)
	require.Contains(t, string(home), "Theme layout")
	notFound, err := os.ReadFile(filepath.Join(target, "_site", "404.html"))
	require.NoError(t, err)
	require.Contains(t, string(notFound), "Theme layout")
}

func newThemeRepository(t *testing.T, layout string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for theme scaffolding tests")
	}

	repository := filepath.Join(t.TempDir(), "starter-theme.git")
	runGit(t, "init", repository)
	runGit(t, "-C", repository, "config", "user.email", "test@example.com")
	runGit(t, "-C", repository, "config", "user.name", "Jigyll Test")
	if layout != "" {
		require.NoError(t, os.MkdirAll(filepath.Join(repository, "_layouts"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(repository, "_layouts", "default.html"), []byte(layout), 0o644))
	} else {
		require.NoError(t, os.WriteFile(filepath.Join(repository, "README.md"), []byte("Theme without layouts\n"), 0o644))
	}
	runGit(t, "-C", repository, "add", ".")
	runGit(t, "-C", repository, "commit", "-m", "initial theme")
	return repository
}

func runGit(t *testing.T, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "git %s: %s", strings.Join(args, " "), output)
}

func statError(path string) error {
	_, err := os.Stat(path)
	return err
}

func assertNoStagingDirectory(t *testing.T, parent, targetName string) {
	t.Helper()
	staging, err := filepath.Glob(filepath.Join(parent, "."+targetName+".jigyll-new-*"))
	require.NoError(t, err)
	require.Empty(t, staging)
}

type newThemeArchiveEntry struct {
	name, content string
	kind          byte
}

func newThemeArchive(t *testing.T, entries ...newThemeArchiveEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(compressed)
	for _, entry := range entries {
		header := &tar.Header{Name: referenceCommit + "/" + entry.name, Mode: 0o644, Typeflag: entry.kind}
		if entry.kind == tar.TypeReg {
			header.Size = int64(len(entry.content))
		}
		if entry.kind == tar.TypeSymlink {
			header.Linkname = "somewhere"
		}
		require.NoError(t, archive.WriteHeader(header))
		if entry.kind == tar.TypeReg {
			_, err := io.WriteString(archive, entry.content)
			require.NoError(t, err)
		}
	}
	require.NoError(t, archive.Close())
	require.NoError(t, compressed.Close())
	return buffer.Bytes()
}

func usableNewThemeArchive(t *testing.T) []byte {
	return newThemeArchive(t,
		newThemeArchiveEntry{"_layouts/default.html", `<!doctype html><main>Archive theme {% include marker.html %} {{ content }}</main><link href="/assets/theme.css"><script src="/assets/theme.js"></script>`, tar.TypeReg},
		newThemeArchiveEntry{"_includes/marker.html", "<span>Shipped include</span>", tar.TypeReg},
		newThemeArchiveEntry{"assets/theme.css", "main { color: navy; }\n", tar.TypeReg},
		newThemeArchiveEntry{"assets/theme.js", "window.themeLoaded = true;\n", tar.TypeReg},
	)
}

func newThemeProvider(t *testing.T, archive []byte, failure string) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		const base = "/api/v1/repos/owner/docifly"
		switch r.URL.Path {
		case base + "/releases/latest":
			if failure == "resolution" {
				http.Error(w, "no published release", http.StatusNotFound)
				return
			}
			io.WriteString(w, `{"tag_name":"v1.2.3"}`)
		case base + "/tags/v1.2.3":
			fmt.Fprintf(w, `{"name":"v1.2.3","commit":{"sha":%q}}`, referenceCommit)
		case base + "/git/commits/" + referenceCommit:
			fmt.Fprintf(w, `{"sha":%q}`, referenceCommit)
		case base + "/archive/" + referenceCommit + ".tar.gz":
			if failure == "download" {
				http.Error(w, "archive unavailable", http.StatusServiceUnavailable)
				return
			}
			if failure == "disconnect" {
				w.Header().Set("Content-Length", fmt.Sprint(len(archive)+100))
				w.Write(archive[:len(archive)/2])
				return
			}
			w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

func TestScaffoldNewSiteReference(t *testing.T) {
	archive := usableNewThemeArchive(t)
	digest := sha256.Sum256(archive)
	for _, selection := range []string{"latest", "v1.2.3", referenceCommit} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/empty=%v", selection, existing), func(t *testing.T) {
				server, requests := newThemeProvider(t, archive, "")
				target := filepath.Join(t.TempDir(), "site")
				if existing {
					require.NoError(t, os.Mkdir(target, 0o755))
				}
				t.Setenv("PATH", t.TempDir())
				_, err := exec.LookPath("git")
				require.Error(t, err)
				source := server.URL + "/owner/docifly@" + selection
				require.NoError(t, ParseAndRun([]string{"new", target, "--theme", source}))
				config, err := os.ReadFile(filepath.Join(target, "_config.yml"))
				require.NoError(t, err)
				require.Contains(t, string(config), "theme: docifly\n")
				require.NotContains(t, string(config), "remote_theme")
				require.ErrorIs(t, statError(filepath.Join(target, "_layouts/default.html")), os.ErrNotExist)
				require.FileExists(t, filepath.Join(target, "_theme/docifly/_layouts/default.html"))
				require.NoDirExists(t, filepath.Join(target, "_theme/docifly", referenceCommit))
				require.NoDirExists(t, filepath.Join(target, "_theme/docifly/.git"))

				data, err := os.ReadFile(filepath.Join(target, "_theme/docifly.source.json"))
				require.NoError(t, err)
				var provenance map[string]string
				require.NoError(t, json.Unmarshal(data, &provenance))
				expected := map[string]string{
					"source": source, "repository_url": server.URL + "/owner/docifly",
					"resolved_commit": referenceCommit,
					"archive_url":     server.URL + "/api/v1/repos/owner/docifly/archive/" + referenceCommit + ".tar.gz",
					"sha256":          hex.EncodeToString(digest[:]),
				}
				if selection != referenceCommit {
					expected["resolved_tag"] = "v1.2.3"
				}
				require.Equal(t, expected, provenance)
				assertNoNewSiteTemporaryFiles(t, target)
				assertNoStagingDirectory(t, filepath.Dir(target), filepath.Base(target))

				// Builds consume the vendored theme without a provider or tools.
				server.Close()
				require.NoError(t, ParseAndRun([]string{"build", "-s", target, "-q"}))
				home, err := os.ReadFile(filepath.Join(target, "_site/index.html"))
				require.NoError(t, err)
				require.Contains(t, string(home), "Archive theme")
				require.Contains(t, string(home), "<span>Shipped include</span>")
				require.Contains(t, string(home), "Welcome to Jigyll")
				for _, asset := range []string{"theme.css", "theme.js"} {
					shipped, err := os.ReadFile(filepath.Join(target, "_theme/docifly/assets", asset))
					require.NoError(t, err)
					built, err := os.ReadFile(filepath.Join(target, "_site/assets", asset))
					require.NoError(t, err)
					require.Equal(t, shipped, built)
				}
				before := requests.Load()
				require.ErrorContains(t, scaffoldNewSite(target, source), "not empty")
				require.Equal(t, before, requests.Load())
				unchanged, err := os.ReadFile(filepath.Join(target, "_theme/docifly.source.json"))
				require.NoError(t, err)
				require.Equal(t, data, unchanged)
				assertNoStagingDirectory(t, filepath.Dir(target), filepath.Base(target))
			})
		}
	}
}

func TestScaffoldNewSiteReferenceFailures(t *testing.T) {
	good := usableNewThemeArchive(t)
	for _, tc := range []struct {
		name, providerFailure, message string
		archive                        []byte
	}{
		{"resolution", "resolution", "resolve latest", good},
		{"download", "download", "download theme", good},
		{"disconnect", "disconnect", "download theme", good},
		{"invalid gzip", "", "open compressed archive", []byte("not gzip")},
		{"truncated gzip", "", "extract theme", good[:len(good)-5]},
		{"missing layout", "", "missing _layouts/default.html", newThemeArchive(t, newThemeArchiveEntry{"assets/theme.css", "body {}", tar.TypeReg})},
		{"directory layout", "", "not a regular file", newThemeArchive(t, newThemeArchiveEntry{"_layouts/default.html/", "", tar.TypeDir})},
		{"linked layout", "", "unsupported type", newThemeArchive(t, newThemeArchiveEntry{"_layouts/default.html", "", tar.TypeSymlink})},
		{"unsafe path", "", "unsafe path", newThemeArchive(t, newThemeArchiveEntry{"../escape", "bad", tar.TypeReg})},
		{"provenance write", "", "write theme provenance", good},
		{"publication", "", "publish new site", good},
	} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/empty=%v", tc.name, existing), func(t *testing.T) {
				server, _ := newThemeProvider(t, tc.archive, tc.providerFailure)
				parent := t.TempDir()
				target := filepath.Join(parent, "site")
				var original fs.FileInfo
				if existing {
					require.NoError(t, os.Mkdir(target, 0o750))
					var err error
					original, err = os.Stat(target)
					require.NoError(t, err)
				}
				t.Setenv("PATH", t.TempDir())
				operations := newSiteOperations{rename: os.Rename, writeFile: os.WriteFile}
				if tc.name == "provenance write" {
					operations.writeFile = func(path string, _ []byte, _ fs.FileMode) error {
						require.Equal(t, "docifly.source.json", filepath.Base(path))
						return os.ErrPermission
					}
				}
				if tc.name == "publication" {
					operations.rename = func(from, to string) error {
						require.Equal(t, target, to)
						require.FileExists(t, filepath.Join(from, "_theme/docifly.source.json"))
						assertNoNewSiteTemporaryFiles(t, from)
						if existing {
							info, err := os.Stat(target)
							require.NoError(t, err)
							require.True(t, os.SameFile(original, info), "empty target must not be removed before rename")
						}
						return os.ErrPermission
					}
				}
				err := scaffoldNewSiteWithOperations(target, server.URL+"/owner/docifly@latest", operations)
				require.ErrorContains(t, err, tc.message)
				require.NotContains(t, err.Error(), "clone")
				if existing {
					info, err := os.Stat(target)
					require.NoError(t, err)
					require.True(t, os.SameFile(original, info))
					require.Equal(t, fs.FileMode(0o750), info.Mode().Perm())
					entries, err := os.ReadDir(target)
					require.NoError(t, err)
					require.Empty(t, entries)
				} else {
					_, err := os.Lstat(target)
					require.ErrorIs(t, err, os.ErrNotExist)
				}
				entries, err := os.ReadDir(parent)
				require.NoError(t, err)
				if existing {
					require.Len(t, entries, 1)
					require.Equal(t, "site", entries[0].Name())
				} else {
					require.Empty(t, entries)
				}
			})
		}
	}
}

func TestScaffoldNewSiteDestinationRefusal(t *testing.T) {
	server, requests := newThemeProvider(t, usableNewThemeArchive(t), "")
	source := server.URL + "/owner/docifly@latest"
	for _, kind := range []string{"file", "symlink", "nonempty", "root"} {
		t.Run(kind, func(t *testing.T) {
			parent := t.TempDir()
			target := filepath.Join(parent, "site")
			original := []byte("keep original bytes")
			switch kind {
			case "file":
				require.NoError(t, os.WriteFile(target, original, 0o644))
			case "symlink":
				require.NoError(t, os.Mkdir(filepath.Join(parent, "elsewhere"), 0o755))
				require.NoError(t, os.Symlink(filepath.Join(parent, "elsewhere"), target))
			case "nonempty":
				require.NoError(t, os.Mkdir(target, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(target, "keep.txt"), original, 0o644))
			case "root":
				target = string(filepath.Separator)
			}
			require.Error(t, scaffoldNewSite(target, source))
			require.Zero(t, requests.Load(), "refused destinations must not contact the provider")
			switch kind {
			case "file", "nonempty":
				path := target
				if kind == "nonempty" {
					path = filepath.Join(target, "keep.txt")
				}
				actual, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, original, actual)
			case "symlink":
				actual, err := os.Readlink(target)
				require.NoError(t, err)
				require.Equal(t, filepath.Join(parent, "elsewhere"), actual)
			}
			assertNoStagingDirectory(t, parent, "site")
		})
	}
}

func TestScaffoldNewSitePreservesConcurrentInstallation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "site")
	operations := newSiteOperations{
		writeFile: os.WriteFile,
		rename: func(from, to string) error {
			require.NoError(t, os.Mkdir(to, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(to, "keep.txt"), []byte("concurrent site"), 0o644))
			return renameNewSite(from, to)
		},
	}
	require.ErrorContains(t, scaffoldNewSiteWithOperations(target, "", operations), "publish new site")
	content, err := os.ReadFile(filepath.Join(target, "keep.txt"))
	require.NoError(t, err)
	require.Equal(t, "concurrent site", string(content))
	assertNoStagingDirectory(t, filepath.Dir(target), filepath.Base(target))
}

func TestScaffoldNewSiteExistingEmptyLegacySources(t *testing.T) {
	repository := newThemeRepository(t, "<main>Git theme {{ content }}</main>")
	for _, source := range []string{"", repository} {
		t.Run(filepath.Base(source), func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "site")
			require.NoError(t, os.Mkdir(target, 0o755))
			require.NoError(t, scaffoldNewSite(target, source))
			require.NoError(t, ParseAndRun([]string{"build", "-s", target, "-q"}))
			home, err := os.ReadFile(filepath.Join(target, "_site/index.html"))
			require.NoError(t, err)
			require.Contains(t, string(home), "Welcome to Jigyll")
			if source != "" {
				require.Contains(t, string(home), "Git theme")
			}
			assertNoStagingDirectory(t, filepath.Dir(target), filepath.Base(target))
		})
	}
}

func assertNoNewSiteTemporaryFiles(t *testing.T, root string) {
	t.Helper()
	require.NoError(t, filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		require.NoError(t, err)
		require.NotContains(t, filepath.Base(path), ".jigyll-theme-download-")
		require.NotEqual(t, "theme.tar.gz", filepath.Base(path))
		return nil
	}))
}
