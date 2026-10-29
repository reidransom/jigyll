package site

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/stretchr/testify/require"
)

func TestDirectorySymlinkRequiresOptIn(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	external := filepath.Join(root, "external")
	require.NoError(t, os.MkdirAll(source, 0755))
	require.NoError(t, os.MkdirAll(external, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(external, "page.md"), []byte("---\n---\n# External page\n"), 0644))
	require.NoError(t, os.Symlink(external, filepath.Join(source, "notes")))
	s, err := FromDirectory(source, config.Flags{})
	require.NoError(t, err)
	err = s.Read()
	require.ErrorContains(t, err, "--follow-directory-symlinks")
	s, err = FromDirectory(source, config.Flags{FollowDirectorySymlinks: true})
	require.NoError(t, err)
	require.NoError(t, s.Read())
	require.Contains(t, s.Routes, "/notes/page.html")
}
