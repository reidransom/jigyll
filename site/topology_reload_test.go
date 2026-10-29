package site

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/stretchr/testify/require"
)

func TestTopologyRebuildReplacesLinkedRoutesAndRetainsGoodSite(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "site")
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	for _, dir := range []string{source, a, b} {
		require.NoError(t, os.MkdirAll(dir, 0755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(a, "old.md"), []byte("---\n---\nold\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(b, "new.md"), []byte("---\n---\nnew\n"), 0644))
	link := filepath.Join(source, "notes")
	require.NoError(t, os.Symlink(a, link))
	incremental := true
	s, err := FromDirectory(source, config.Flags{FollowDirectorySymlinks: true, Incremental: &incremental})
	require.NoError(t, err)
	require.NoError(t, s.Read())
	require.Contains(t, s.Routes, "/notes/old.html")
	require.NoError(t, os.Remove(link))
	require.NoError(t, os.Symlink(b, link))
	replacement, _, err := s.rebuildEvent(FilesEvent{Topology: true})
	require.NoError(t, err)
	require.NotContains(t, replacement.Routes, "/notes/old.html")
	require.Contains(t, replacement.Routes, "/notes/new.html")
	require.NoError(t, os.Remove(link))
	require.NoError(t, os.Symlink(filepath.Join(root, "missing"), link))
	failed, _, err := replacement.rebuildEvent(FilesEvent{Topology: true})
	require.Error(t, err)
	require.Same(t, replacement, failed)
	require.Contains(t, replacement.Routes, "/notes/new.html")
	require.NoError(t, os.Remove(link))
	require.NoError(t, os.Symlink(a, link))
	repaired, _, err := replacement.rebuildEvent(FilesEvent{Topology: true})
	require.NoError(t, err)
	require.Contains(t, repaired.Routes, "/notes/old.html")
}
