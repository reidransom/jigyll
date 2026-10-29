package site

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/stretchr/testify/require"
)

func TestLocalizedDirectorySymlink(t *testing.T) {
	source, destination := localizedDevelopmentProjectFixture(t)
	external := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(external, "page.md"), []byte("---\nlang: de\npermalink: /linked/\n---\nGerman linked page\n"), 0644))
	link := filepath.Join(source, "notes")
	require.NoError(t, os.Symlink(external, link))
	require.NoError(t, os.Symlink(destination, filepath.Join(source, "output-alias")))
	base, err := FromDirectory(source, config.Flags{})
	require.NoError(t, err)
	_, _, err = BuildLocalizedDevelopmentProject(base)
	require.ErrorContains(t, err, "--follow-directory-symlinks")
	base, err = FromDirectory(source, config.Flags{FollowDirectorySymlinks: true})
	require.NoError(t, err)
	project, _, err := BuildLocalizedDevelopmentProject(base)
	require.NoError(t, err)
	require.Contains(t, localizedServedContent(t, project, "/de/linked/"), "German linked page")
	_, page, found := project.URLPage("/de/linked/")
	require.True(t, found)
	require.Equal(t, filepath.Join(link, "page.md"), page.Source())
	require.NoError(t, os.Remove(link))
	require.NoError(t, os.Symlink(filepath.Join(external, "missing"), link))
	_, _, err = project.RebuildDevelopment()
	require.Error(t, err)
	require.Contains(t, localizedServedContent(t, project, "/de/linked/"), "German linked page")
}
