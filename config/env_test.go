package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromDirectory_JEKYLL_URL_Override_ConfigYML(t *testing.T) {
	// Create a temporary directory with _config.yml
	tmpDir, err := os.MkdirTemp("", "gojekyll-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	configYML := `url: https://example.com
title: Test Site
`
	configPath := filepath.Join(tmpDir, "_config.yml")
	err = os.WriteFile(configPath, []byte(configYML), 0644)
	require.NoError(t, err)

	// Set JEKYLL_URL environment variable
	origURL := os.Getenv("JEKYLL_URL")
	defer func() { _ = os.Setenv("JEKYLL_URL", origURL) }()
	err = os.Setenv("JEKYLL_URL", "https://override.com")
	require.NoError(t, err)

	// Load config
	c := Default()
	err = c.FromDirectory(tmpDir, "")
	require.NoError(t, err)

	// Check that AbsoluteURL field is overridden
	require.Equal(t, "https://override.com", c.AbsoluteURL)

	// Check that the Variables() map also reflects the override
	// This is what templates actually use
	vars := c.Variables()
	urlValue, ok := vars["url"]
	require.True(t, ok, "url should be present in variables")
	require.Equal(t, "https://override.com", urlValue, "url in variables should be overridden")
}

func TestFromDirectory_JEKYLL_URL_NotSet(t *testing.T) {
	// Create a temporary directory with _config.yml
	tmpDir, err := os.MkdirTemp("", "gojekyll-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	configYML := `url: https://example.com
title: Test Site
`
	configPath := filepath.Join(tmpDir, "_config.yml")
	err = os.WriteFile(configPath, []byte(configYML), 0644)
	require.NoError(t, err)

	// Ensure JEKYLL_URL is not set
	origURL := os.Getenv("JEKYLL_URL")
	defer func() { _ = os.Setenv("JEKYLL_URL", origURL) }()
	err = os.Unsetenv("JEKYLL_URL")
	require.NoError(t, err)

	// Load config
	c := Default()
	err = c.FromDirectory(tmpDir, "")
	require.NoError(t, err)

	// Check that AbsoluteURL field uses config value
	require.Equal(t, "https://example.com", c.AbsoluteURL)

	// Check that the Variables() map also has the original value
	vars := c.Variables()
	urlValue, ok := vars["url"]
	require.True(t, ok, "url should be present in variables")
	require.Equal(t, "https://example.com", urlValue)
}

