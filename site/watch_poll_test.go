package site

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/reidransom/jigyll/config"
	"github.com/stretchr/testify/require"
)

func TestPollingLinkedContentAndRecovery(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "site")
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	for _, dir := range []string{source, a, b} {
		require.NoError(t, os.MkdirAll(dir, 0755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(a, "note.md"), []byte("old"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(b, "note.md"), []byte("new"), 0644))
	link := filepath.Join(source, "notes")
	require.NoError(t, os.Symlink(a, link))
	s := New(config.Flags{})
	s.cfg.Source = source
	s.cfg.FollowDirectorySymlinks = true
	changes, err := s.makePollingWatcher()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(a, "note.md"), []byte("updated old"), 0644))
	waitPoll(t, changes, func(e FilesEvent) bool { return containsPath(e.Paths, filepath.Join("notes", "note.md")) })
	require.NoError(t, os.Remove(link))
	require.NoError(t, os.Symlink(b, link))
	waitPoll(t, changes, func(e FilesEvent) bool { return e.Topology })
	require.NoError(t, os.Remove(link))
	require.NoError(t, os.Symlink(filepath.Join(root, "missing"), link))
	waitPoll(t, changes, func(e FilesEvent) bool { return e.Topology })
	require.NoError(t, os.Remove(link))
	require.NoError(t, os.Symlink(a, link))
	waitPoll(t, changes, func(e FilesEvent) bool { return e.Topology })
}
func waitPoll(t *testing.T, changes <-chan FilesEvent, match func(FilesEvent) bool) {
	t.Helper()
	timeout := time.After(3 * time.Second)
	for {
		select {
		case e := <-changes:
			if match(e) {
				return
			}
		case <-timeout:
			t.Fatal("timed out waiting for polling change")
		}
	}
}
