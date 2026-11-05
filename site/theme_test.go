package site

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/stretchr/testify/require"
)

func TestFindTheme(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "jigyll-theme-test")
	require.NoError(t, err)
	defer func() { require.NoError(t, os.RemoveAll(tempDir)) }()

	t.Run("no theme specified", func(t *testing.T) {
		flags := config.Flags{}
		s := New(flags)
		s.cfg.Source = tempDir
		err := s.findTheme()
		require.NoError(t, err)
		require.Empty(t, s.themeDir)
	})

	t.Run("local and remote themes conflict", func(t *testing.T) {
		s := New(config.Flags{})
		s.cfg.Source = tempDir
		s.cfg.Theme = "local"
		s.cfg.RemoteTheme = "owner/repo@0123456789012345678901234567890123456789"

		require.EqualError(t, s.findTheme(), "_config.yml cannot specify both theme and remote_theme")
	})

	t.Run("theme found in _theme folder", func(t *testing.T) {
		// Create _theme/mytheme directory structure
		themeDir := filepath.Join(tempDir, "_theme", "mytheme")
		err := os.MkdirAll(themeDir, 0755)
		require.NoError(t, err)

		flags := config.Flags{}
		s := New(flags)
		s.cfg.Source = tempDir
		s.cfg.Theme = "mytheme"

		err = s.findTheme()
		require.NoError(t, err)
		require.Equal(t, themeDir, s.themeDir)
	})

	t.Run("theme not found anywhere", func(t *testing.T) {
		// Use a clean temp directory without the theme
		cleanTempDir, err := os.MkdirTemp("", "jigyll-theme-test-clean")
		require.NoError(t, err)
		defer func() { require.NoError(t, os.RemoveAll(cleanTempDir)) }()

		flags := config.Flags{}
		s := New(flags)
		s.cfg.Source = cleanTempDir
		s.cfg.Theme = "nonexistent-theme"

		err = s.findTheme()
		require.Error(t, err)
		// The error could be either our custom message or a bundle error
		errorMsg := err.Error()
		require.True(t,
			err.Error() != "",
			"Expected error when theme is not found, got: %s", errorMsg)
	})

	t.Run("empty theme name", func(t *testing.T) {
		flags := config.Flags{}
		s := New(flags)
		s.cfg.Source = tempDir
		s.cfg.Theme = ""

		err := s.findTheme()
		require.NoError(t, err)
		require.Empty(t, s.themeDir)
	})

	t.Run("_theme directory exists but theme subdirectory does not", func(t *testing.T) {
		// Create _theme directory but not the specific theme
		themeBaseDir := filepath.Join(tempDir, "_theme")
		err := os.MkdirAll(themeBaseDir, 0755)
		require.NoError(t, err)

		flags := config.Flags{}
		s := New(flags)
		s.cfg.Source = tempDir
		s.cfg.Theme = "missing-theme"

		err = s.findTheme()
		require.Error(t, err)
		// Just verify we get an error, the exact message may vary depending on bundle availability
		require.NotNil(t, err)
	})

	t.Run("theme found in _theme takes priority over bundle", func(t *testing.T) {
		// Create _theme/priority-theme directory structure
		themeDir := filepath.Join(tempDir, "_theme", "priority-theme")
		err := os.MkdirAll(themeDir, 0755)
		require.NoError(t, err)

		flags := config.Flags{}
		s := New(flags)
		s.cfg.Source = tempDir
		s.cfg.Theme = "priority-theme"

		err = s.findTheme()
		require.NoError(t, err)
		require.Equal(t, themeDir, s.themeDir)
		// Verify the path is what we expect from _theme folder
		require.Contains(t, s.themeDir, "_theme/priority-theme")
	})
}

func TestReadThemeAssets(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "jigyll-theme-assets-test")
	require.NoError(t, err)
	defer func() { require.NoError(t, os.RemoveAll(tempDir)) }()

	t.Run("theme has assets directory", func(t *testing.T) {
		// Create theme with assets
		themeDir := filepath.Join(tempDir, "_theme", "mytheme")
		assetsDir := filepath.Join(themeDir, "assets")
		err := os.MkdirAll(assetsDir, 0755)
		require.NoError(t, err)

		// Create a test asset file
		testAsset := filepath.Join(assetsDir, "style.css")
		err = os.WriteFile(testAsset, []byte("body { color: red; }"), 0644)
		require.NoError(t, err)

		flags := config.Flags{}
		s := New(flags)
		s.cfg.Source = tempDir
		s.themeDir = themeDir
		s.Routes = make(map[string]Document) // Initialize Routes map

		err = s.readThemeAssets()
		require.NoError(t, err)
	})

	t.Run("theme has no assets directory", func(t *testing.T) {
		// Create theme without assets
		themeDir := filepath.Join(tempDir, "_theme", "no-assets-theme")
		err := os.MkdirAll(themeDir, 0755)
		require.NoError(t, err)

		flags := config.Flags{}
		s := New(flags)
		s.cfg.Source = tempDir
		s.themeDir = themeDir
		s.Routes = make(map[string]Document) // Initialize Routes map

		err = s.readThemeAssets()
		require.NoError(t, err) // Should not error when assets dir doesn't exist
	})
}

func TestFindThemeSkipsRemoteResolutionWithoutRemoteTheme(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()

	s := New(config.Flags{})
	s.cfg.Source = t.TempDir()
	s.remoteThemes = testRemoteThemeResolver(t, server.URL)
	require.NoError(t, s.findTheme())
	require.Zero(t, requests.Load())
}

func TestFindThemeRejectsMalformedRemoteThemeBeforeDownload(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()

	s := New(config.Flags{})
	s.cfg.Source = t.TempDir()
	s.cfg.RemoteTheme = "owner/repo@main"
	s.remoteThemes = testRemoteThemeResolver(t, server.URL)

	err := s.findTheme()
	require.ErrorContains(t, err, `resolve remote theme "owner/repo@main"`)
	require.Zero(t, requests.Load())
}

func TestRemoteThemeBuildAndSiteOverrides(t *testing.T) {
	const sha = "0123456789012345678901234567890123456789"
	archive := remoteThemeArchiveWithFiles(t, map[string]string{
		"_layouts/default.html": "THEME LAYOUT {% include shared.html %}<main>{{ content }}</main>",
		"_includes/shared.html": "THEME INCLUDE",
		"_sass/_colors.scss":    "$color: red;",
		"assets/style.scss":     "---\n---\n@import \"colors\";\nbody { color: $color; }",
		"assets/theme.txt":      "THEME ASSET",
	})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	t.Run("uses remote layout include Sass and asset", func(t *testing.T) {
		root := remoteThemeTestSite(t, sha)
		s := remoteThemeTestBuild(t, root, testRemoteThemeResolver(t, server.URL))

		html := readThemeTestOutput(t, root, "index.html")
		require.Contains(t, html, "THEME LAYOUT")
		require.Contains(t, html, "THEME INCLUDE")
		require.Contains(t, readThemeTestOutput(t, root, "assets/style.css"), "red")
		require.Equal(t, "THEME ASSET", readThemeTestOutput(t, root, "assets/theme.txt"))
		require.NotEmpty(t, s.themeDir)
	})

	t.Run("site files override cached remote theme files", func(t *testing.T) {
		root := remoteThemeTestSite(t, sha)
		writeThemeTestFile(t, root, "_layouts/default.html", "SITE LAYOUT {% include shared.html %}<main>{{ content }}</main>")
		writeThemeTestFile(t, root, "_includes/shared.html", "SITE INCLUDE")
		writeThemeTestFile(t, root, "_sass/_colors.scss", "$color: blue;")
		writeThemeTestFile(t, root, "assets/style.scss", "---\n---\n@import \"colors\";\nbody { color: $color; }")
		writeThemeTestFile(t, root, "assets/theme.txt", "SITE ASSET")
		remoteThemeTestBuild(t, root, testRemoteThemeResolver(t, server.URL))

		html := readThemeTestOutput(t, root, "index.html")
		require.Contains(t, html, "SITE LAYOUT")
		require.Contains(t, html, "SITE INCLUDE")
		require.NotContains(t, html, "THEME LAYOUT")
		require.Contains(t, readThemeTestOutput(t, root, "assets/style.css"), "blue")
		require.Equal(t, "SITE ASSET", readThemeTestOutput(t, root, "assets/theme.txt"))
	})

	t.Run("missing theme resource reports the renderer error", func(t *testing.T) {
		root := remoteThemeTestSite(t, sha)
		writeThemeTestFile(t, root, "_layouts/default.html", "{% include absent.html %}{{ content }}")
		s, err := FromDirectory(root, config.Flags{})
		require.NoError(t, err)
		s.remoteThemes = testRemoteThemeResolver(t, server.URL)
		require.NoError(t, s.Read())
		_, err = s.Write()
		require.Error(t, err)
		require.Contains(t, err.Error(), "absent.html")
	})

	require.EqualValues(t, 3, requests.Load())
}

func remoteThemeTestSite(t *testing.T, sha string) string {
	t.Helper()
	root := t.TempDir()
	writeThemeTestFile(t, root, "_config.yml", "remote_theme: owner/repo@"+sha)
	writeThemeTestFile(t, root, "index.md", "---\nlayout: default\n---\nPAGE")
	return root
}

func remoteThemeTestBuild(t *testing.T, root string, resolver *remoteThemeResolver) *Site {
	t.Helper()
	s, err := FromDirectory(root, config.Flags{})
	require.NoError(t, err)
	s.remoteThemes = resolver
	require.NoError(t, s.Read())
	_, err = s.Write()
	require.NoError(t, err)
	return s
}

func writeThemeTestFile(t *testing.T, root, name, content string) {
	t.Helper()
	filename := filepath.Join(root, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(filename), 0o755))
	require.NoError(t, os.WriteFile(filename, []byte(content), 0o644))
}

func readThemeTestOutput(t *testing.T, root, name string) string {
	t.Helper()
	output, err := os.ReadFile(filepath.Join(root, "_site", name))
	require.NoError(t, err)
	return string(output)
}
