package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromDirectory_ConfigFlag_SingleFile(t *testing.T) {
	// Create a temporary directory with custom config file
	tmpDir, err := os.MkdirTemp("", "gojekyll-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create custom config file
	configYML := `title: Single Config Title
url: https://single.example.com
`
	configPath := filepath.Join(tmpDir, "my_config.yml")
	err = os.WriteFile(configPath, []byte(configYML), 0644)
	require.NoError(t, err)

	// Load config with explicit config file
	c := Default()
	err = c.FromDirectory(tmpDir, "my_config.yml")
	require.NoError(t, err)

	// Should use specified config file
	title, ok := c.String("title")
	require.True(t, ok)
	require.Equal(t, "Single Config Title", title)

	url, ok := c.String("url")
	require.True(t, ok)
	require.Equal(t, "https://single.example.com", url)
}

func TestFromDirectory_ConfigFlag_MultipleFiles(t *testing.T) {
	// Create a temporary directory with multiple config files
	tmpDir, err := os.MkdirTemp("", "gojekyll-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create base config file
	baseConfigYML := `title: Base Title
url: https://base.example.com
description: Base description
port: 4000
`
	baseConfigPath := filepath.Join(tmpDir, "_config.yml")
	err = os.WriteFile(baseConfigPath, []byte(baseConfigYML), 0644)
	require.NoError(t, err)

	// Create local override config file
	localConfigYML := `title: Local Title
url: https://local.example.com
custom_key: custom_value
`
	localConfigPath := filepath.Join(tmpDir, "_config.local.yml")
	err = os.WriteFile(localConfigPath, []byte(localConfigYML), 0644)
	require.NoError(t, err)

	// Load config with multiple files (comma-separated)
	c := Default()
	err = c.FromDirectory(tmpDir, "_config.yml,_config.local.yml")
	require.NoError(t, err)

	// Later file should override earlier file
	title, ok := c.String("title")
	require.True(t, ok)
	require.Equal(t, "Local Title", title)

	url, ok := c.String("url")
	require.True(t, ok)
	require.Equal(t, "https://local.example.com", url)

	// Values only in first file should be preserved
	description, ok := c.String("description")
	require.True(t, ok)
	require.Equal(t, "Base description", description)

	port := c.Port
	require.Equal(t, 4000, port)

	// Values from second file should be added
	customKey, ok := c.String("custom_key")
	require.True(t, ok)
	require.Equal(t, "custom_value", customKey)
}

func TestFromDirectory_ConfigFlag_TripleFiles(t *testing.T) {
	// Create a temporary directory with three config files
	tmpDir, err := os.MkdirTemp("", "gojekyll-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create base config file
	file1YML := `title: File1 Title
url: https://file1.example.com
key1: value1
`
	file1Path := filepath.Join(tmpDir, "config1.yml")
	err = os.WriteFile(file1Path, []byte(file1YML), 0644)
	require.NoError(t, err)

	// Create second config file
	file2YML := `title: File2 Title
key2: value2
`
	file2Path := filepath.Join(tmpDir, "config2.yml")
	err = os.WriteFile(file2Path, []byte(file2YML), 0644)
	require.NoError(t, err)

	// Create third config file
	file3YML := `url: https://file3.example.com
key3: value3
`
	file3Path := filepath.Join(tmpDir, "config3.yml")
	err = os.WriteFile(file3Path, []byte(file3YML), 0644)
	require.NoError(t, err)

	// Load config with three files
	c := Default()
	err = c.FromDirectory(tmpDir, "config1.yml,config2.yml,config3.yml")
	require.NoError(t, err)

	// Title from file2 (overrides file1)
	title, ok := c.String("title")
	require.True(t, ok)
	require.Equal(t, "File2 Title", title)

	// URL from file3 (overrides file1)
	url, ok := c.String("url")
	require.True(t, ok)
	require.Equal(t, "https://file3.example.com", url)

	// All keys should be present
	key1, ok := c.String("key1")
	require.True(t, ok)
	require.Equal(t, "value1", key1)

	key2, ok := c.String("key2")
	require.True(t, ok)
	require.Equal(t, "value2", key2)

	key3, ok := c.String("key3")
	require.True(t, ok)
	require.Equal(t, "value3", key3)
}

func TestFromDirectory_ConfigFlag_WithSpaces(t *testing.T) {
	// Test that spaces around commas are handled correctly
	tmpDir, err := os.MkdirTemp("", "gojekyll-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create config files
	file1YML := `title: File1`
	file1Path := filepath.Join(tmpDir, "config1.yml")
	err = os.WriteFile(file1Path, []byte(file1YML), 0644)
	require.NoError(t, err)

	file2YML := `url: https://file2.example.com`
	file2Path := filepath.Join(tmpDir, "config2.yml")
	err = os.WriteFile(file2Path, []byte(file2YML), 0644)
	require.NoError(t, err)

	// Load config with spaces around commas
	c := Default()
	err = c.FromDirectory(tmpDir, "config1.yml , config2.yml")
	require.NoError(t, err)

	// Should work despite spaces
	title, ok := c.String("title")
	require.True(t, ok)
	require.Equal(t, "File1", title)

	url, ok := c.String("url")
	require.True(t, ok)
	require.Equal(t, "https://file2.example.com", url)
}

func TestFromDirectory_ConfigFlag_FileNotFound(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "gojekyll-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Try to load with non-existent config file
	c := Default()
	err = c.FromDirectory(tmpDir, "nonexistent.yml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "nonexistent.yml")
}

func TestFromDirectory_ConfigFlag_SecondFileNotFound(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "gojekyll-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create first config file
	file1YML := `title: File1`
	file1Path := filepath.Join(tmpDir, "config1.yml")
	err = os.WriteFile(file1Path, []byte(file1YML), 0644)
	require.NoError(t, err)

	// Try to load with second file that doesn't exist
	c := Default()
	err = c.FromDirectory(tmpDir, "config1.yml,nonexistent.yml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "nonexistent.yml")
}

func TestFromDirectory_ConfigFlag_AbsolutePath(t *testing.T) {
	// Test that absolute paths work
	tmpDir, err := os.MkdirTemp("", "gojekyll-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create config file
	configYML := `title: Absolute Path Config`
	configPath := filepath.Join(tmpDir, "config.yml")
	err = os.WriteFile(configPath, []byte(configYML), 0644)
	require.NoError(t, err)

	// Load with absolute path
	c := Default()
	err = c.FromDirectory(tmpDir, configPath)
	require.NoError(t, err)

	title, ok := c.String("title")
	require.True(t, ok)
	require.Equal(t, "Absolute Path Config", title)
}

func TestFromDirectory_JEKYLL_CONFIG_EnvVar(t *testing.T) {
	// Test JEKYLL_CONFIG environment variable
	tmpDir, err := os.MkdirTemp("", "gojekyll-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create config files
	baseYML := `title: Base
port: 4000
`
	baseConfigPath := filepath.Join(tmpDir, "_config.yml")
	err = os.WriteFile(baseConfigPath, []byte(baseYML), 0644)
	require.NoError(t, err)

	localYML := `title: Local Override
custom: value
`
	localConfigPath := filepath.Join(tmpDir, "_config.local.yml")
	err = os.WriteFile(localConfigPath, []byte(localYML), 0644)
	require.NoError(t, err)

	// Set JEKYLL_CONFIG environment variable
	origConfig := os.Getenv("JEKYLL_CONFIG")
	defer func() { _ = os.Setenv("JEKYLL_CONFIG", origConfig) }()
	err = os.Setenv("JEKYLL_CONFIG", "_config.yml,_config.local.yml")
	require.NoError(t, err)

	// Load config without --config flag (should use JEKYLL_CONFIG)
	c := Default()
	err = c.FromDirectory(tmpDir, "")
	require.NoError(t, err)

	// Should merge both files
	title, ok := c.String("title")
	require.True(t, ok)
	require.Equal(t, "Local Override", title)

	port := c.Port
	require.Equal(t, 4000, port)

	custom, ok := c.String("custom")
	require.True(t, ok)
	require.Equal(t, "value", custom)
}

func TestFromDirectory_ConfigFlag_OverridesJEKYLL_CONFIG(t *testing.T) {
	// Test that --config flag takes precedence over JEKYLL_CONFIG
	tmpDir, err := os.MkdirTemp("", "gojekyll-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create config files
	envYML := `title: From ENV`
	envConfigPath := filepath.Join(tmpDir, "env_config.yml")
	err = os.WriteFile(envConfigPath, []byte(envYML), 0644)
	require.NoError(t, err)

	flagYML := `title: From Flag`
	flagConfigPath := filepath.Join(tmpDir, "flag_config.yml")
	err = os.WriteFile(flagConfigPath, []byte(flagYML), 0644)
	require.NoError(t, err)

	// Set JEKYLL_CONFIG environment variable
	origConfig := os.Getenv("JEKYLL_CONFIG")
	defer func() { _ = os.Setenv("JEKYLL_CONFIG", origConfig) }()
	err = os.Setenv("JEKYLL_CONFIG", "env_config.yml")
	require.NoError(t, err)

	// Load with --config flag (should override JEKYLL_CONFIG)
	c := Default()
	err = c.FromDirectory(tmpDir, "flag_config.yml")
	require.NoError(t, err)

	// Should use flag, not env var
	title, ok := c.String("title")
	require.True(t, ok)
	require.Equal(t, "From Flag", title)
}
