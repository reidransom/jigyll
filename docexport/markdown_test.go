package docexport

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarkdownDestinationsPreserveExamples(t *testing.T) {
	source := "Intro [guide](../other/?q=1#setup) and [local](#repeat-1), [root](/docs/start/), [mail](mailto:help@example.com).\n\n" +
		"![Alt](../../image.png) [reference][doc]\n\n[doc]: <../other/?q=2#table> \"Title\"\n\n" +
		"```html\n<a href=\"/docs/start/\">[root](/docs/start/)</a>\n```\n\n" +
		"`[root](/docs/start/)`\n\n    [root](/docs/start/)\n\n" +
		"| Field | Example |\n|---|---|\n| `_admin.yml` | `{{ literal }}` |\n\n" +
		"<img alt=\"Raw image\" src=\"/assets/photo.png?q=1\" srcset=\"/assets/photo.png 1x, /assets/photo2.png 2x\">\n"
	output, err := Markdown([]byte(source), "https://r2ware.dev/docs/site-dev/article/", map[string]string{"/assets/photo.png": "https://cdn.example/photo.hash.png"})
	require.NoError(t, err)
	result := string(output)
	for _, want := range []string{
		"[guide](https://r2ware.dev/docs/site-dev/other/?q=1#setup)",
		"[local](https://r2ware.dev/docs/site-dev/article/#repeat-1)",
		"[root](https://r2ware.dev/docs/start/)",
		"[mail](mailto:help@example.com)",
		"![Alt](https://r2ware.dev/docs/image.png)",
		"[doc]: <https://r2ware.dev/docs/site-dev/other/?q=2#table> \"Title\"",
		"```html\n<a href=\"/docs/start/\">[root](/docs/start/)</a>\n```",
		"`[root](/docs/start/)`", "    [root](/docs/start/)",
		"| `_admin.yml` | `{{ literal }}` |",
		"src=\"https://cdn.example/photo.hash.png?q=1\"",
		"srcset=\"https://cdn.example/photo.hash.png 1x, https://r2ware.dev/assets/photo2.png 2x\"",
	} {
		require.Contains(t, result, want)
	}
	require.Equal(t, 1, strings.Count(result, "[root](https://r2ware.dev/docs/start/)"))
}

func TestMarkdownUnmatchedCodeAndBalancedDestinations(t *testing.T) {
	source := "An unmatched ` then [guide](../other/) and [nested](../a(b)/#fragment).\n\n<!-- [guide](../other/) -->\n"
	output, err := Markdown([]byte(source), "https://r2ware.dev/docs/site-dev/article/", nil)
	require.NoError(t, err)
	require.Contains(t, string(output), "[guide](https://r2ware.dev/docs/site-dev/other/)")
	require.Contains(t, string(output), "[nested](https://r2ware.dev/docs/site-dev/a(b)/#fragment)")
	require.Contains(t, string(output), "<!-- [guide](../other/) -->")
}
