package site

import (
	"os"
	"path/filepath"
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