package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWalkSourceDirectoryLinks(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "site")
	external := filepath.Join(root, "external")
	require.NoError(t, os.MkdirAll(source, 0755))
	require.NoError(t, os.MkdirAll(external, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(external, "note.md"), []byte("note"), 0644))
	require.NoError(t, os.Symlink("../external", filepath.Join(source, "a")))
	require.NoError(t, os.Symlink(external, filepath.Join(source, "b")))
	require.NoError(t, os.Symlink(filepath.Join(external, "note.md"), filepath.Join(source, "file.md")))
	visit := func(path string, info os.FileInfo, err error) error { return err }
	err := WalkSource(source, SourceWalkOptions{}, visit)
	require.ErrorContains(t, err, "--follow-directory-symlinks")
	var paths []string
	err = WalkSource(source, SourceWalkOptions{FollowDirectorySymlinks: true}, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		require.NoError(t, err)
		paths = append(paths, rel)
		return nil
	})
	require.NoError(t, err)
	require.Contains(t, paths, filepath.Join("a", "note.md"))
	require.Contains(t, paths, filepath.Join("b", "note.md"))
	require.Contains(t, paths, "file.md")
	require.NoError(t, os.Symlink(source, filepath.Join(external, "cycle")))
	err = WalkSource(source, SourceWalkOptions{FollowDirectorySymlinks: true}, visit)
	require.ErrorContains(t, err, "cycle")
	err = WalkSource(source, SourceWalkOptions{FollowDirectorySymlinks: true, Prune: func(path string, info os.FileInfo) bool { return filepath.Base(path) == "cycle" }}, visit)
	require.NoError(t, err)
}

func TestWalkSourceProtectsOutput(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "site")
	external := filepath.Join(root, "external")
	require.NoError(t, os.MkdirAll(source, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(external, "out"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(external, "outside"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(external, "out", "generated"), []byte("x"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(external, "outside", "keep"), []byte("x"), 0644))
	require.NoError(t, os.Symlink(external, filepath.Join(source, "mount")))
	var paths []string
	err := WalkSource(source, SourceWalkOptions{FollowDirectorySymlinks: true, Protected: []string{filepath.Join(external, "out")}}, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, path)
		return nil
	})
	require.NoError(t, err)
	require.NotContains(t, strings.Join(paths, "\n"), "generated")
	require.Contains(t, strings.Join(paths, "\n"), "keep")
	err = WalkSource(source, SourceWalkOptions{Protected: []string{source}}, func(_ string, _ os.FileInfo, err error) error { return err })
	require.ErrorContains(t, err, "same directory")
	canonical, err := CanonicalPath(filepath.Join(source, "mount", "missing", "output"))
	require.NoError(t, err)
	require.Equal(t, filepath.Join(external, "missing", "output"), canonical)
}
