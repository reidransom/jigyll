package site

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/reidransom/jigyll/config"
	"github.com/stretchr/testify/require"
)

func TestDebounceEventsKeepsTopology(t *testing.T) {
	input := make(chan FilesEvent, 2)
	output := debounceEvents(20*time.Millisecond, input)
	input <- FilesEvent{Topology: true}
	input <- FilesEvent{Paths: []string{"note.md"}}
	select {
	case event := <-output:
		require.True(t, event.Topology)
		require.Equal(t, []string{"note.md"}, event.Paths)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for topology event")
	}
	close(input)
}

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
	case change := <-filenames:
		require.Contains(t, change.Paths, filepath.Join("subdir", "test.txt"))
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for subdirectory file change event")
	}
}
