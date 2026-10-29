package site

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/reidransom/jigyll/config"
	"github.com/stretchr/testify/require"
)

func TestEventWatcherLinkedAliasesAndRetarget(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "site")
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	for _, dir := range []string{source, a, b} {
		require.NoError(t, os.MkdirAll(dir, 0755))
	}
	file := filepath.Join(a, "note.md")
	require.NoError(t, os.WriteFile(file, []byte("one"), 0644))
	require.NoError(t, os.Symlink(a, filepath.Join(source, "one")))
	require.NoError(t, os.Symlink(a, filepath.Join(source, "two")))
	s := New(config.Flags{FollowDirectorySymlinks: true})
	s.cfg.Source = source
	s.cfg.FollowDirectorySymlinks = true
	changes, err := s.makeEventWatcher()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(file, []byte("two"), 0644))
	deadline := time.After(3 * time.Second)
	found := false
	for !found {
		select {
		case e := <-changes:
			if len(e.Paths) > 0 {
				if containsPath(e.Paths, filepath.Join("one", "note.md")) && containsPath(e.Paths, filepath.Join("two", "note.md")) {
					found = true
				}
			}
		case <-deadline:
			t.Fatal("missing alias fan-out")
		}
	}
	link := filepath.Join(source, "one")
	require.NoError(t, os.Remove(link))
	require.NoError(t, os.Symlink(b, link))
	deadline = time.After(3 * time.Second)
	found = false
	for !found {
		select {
		case e := <-changes:
			if e.Topology {
				found = true
			}
		case <-deadline:
			t.Fatal("missing topology event")
		}
	}
}
func containsPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}
