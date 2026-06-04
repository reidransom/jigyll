package site

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/reidransom/gojekyll/config"
	"github.com/stretchr/testify/require"
)

func TestEventWatcher_SubdirectoryChanges(t *testing.T) {
	// Create a temp site directory with a subdirectory
	dir := t.TempDir()
	subdir := filepath.Join(dir, "subdir")
	require.NoError(t, os.Mkdir(subdir, 0755))

	// Create initial file so the watcher has something to watch
	initial := filepath.Join(subdir, "test.txt")
	require.NoError(t, os.WriteFile(initial, []byte("initial"), 0644))

	s := New(config.Flags{})
	s.cfg.Source = dir

	filenames, err := s.makeEventWatcher()
	require.NoError(t, err)

	// Give the watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Modify a file in the subdirectory
	require.NoError(t, os.WriteFile(initial, []byte("changed"), 0644))

	// We should receive a notification for the subdirectory file
	select {
	case rel := <-filenames:
		require.Equal(t, filepath.Join("subdir", "test.txt"), rel)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for subdirectory file change event")
	}
}

func TestDebounce(t *testing.T) {
	input := make(chan string, 10)
	output := debounce(50*time.Millisecond, input)

	// Send several values in quick succession
	input <- "a"
	input <- "b"
	input <- "c"

	select {
	case batch := <-output:
		require.Equal(t, []string{"a", "b", "c"}, batch)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for debounced output")
	}
}

func TestDebounce_SkipsDot(t *testing.T) {
	input := make(chan string, 10)
	output := debounce(50*time.Millisecond, input)

	input <- "."
	input <- "a"

	select {
	case batch := <-output:
		require.Equal(t, []string{"a"}, batch)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for debounced output")
	}
}
