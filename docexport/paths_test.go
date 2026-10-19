package docexport

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOutputNamespaceDistinguishesFilesFromDirectories(t *testing.T) {
	for _, paths := range [][]string{
		{"/docs/manifest.json", "/docs/manifest.json/index.md"},
		{"/docs/guide/index.md", "/docs/guide/index.md/nested/index.html"},
		{"/docs/a/index.md", "/docs/a/./index.md"},
	} {
		require.Error(t, ValidateOutputPaths(paths))
	}
}
