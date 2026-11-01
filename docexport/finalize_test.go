package docexport

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeFinalizerFixture(t *testing.T, root string) Manifest {
	t.Helper()
	manifest, corpus, markdown := snapshotFixture()
	markdown[manifest.Articles[0].URL+"index.md"] = []byte("![asset](/assets/a.png)\n\n`/assets/a.png`\n")
	artifacts, err := Assemble(manifest, corpus, markdown)
	if err != nil {
		t.Fatal(err)
	}
	artifacts[manifest.Articles[0].URL+"index.html"] = []byte("<html><body><nav>Never indexed</nav><article class='docs-article'><h1 id='final'>Final heading</h1><p>Final transformed text.</p></article></body></html>")
	for route, data := range artifacts {
		name := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(route, "/")))
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return manifest
}

func TestFinalizeUsesFinalBodyAndRevisionMapWithoutOwningDiscovery(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		index []byte
	}{
		{name: "docs-only"},
		{
			name:  "curated-plugin-index",
			index: []byte("# Curated guides\n\n> Editorial overview.\n\n## Selected\n\n- [Start](<https://example.com/docs/guide/start/index.md>)\n- [External](<https://reference.example/guide>)\n"),
		},
		{
			name:  "authored-index",
			index: []byte("Authored sentinel\r\nKeep this file byte-for-byte.\r\n/assets/a.png\r\n"),
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root := t.TempDir()
			manifest := writeFinalizerFixture(t, root)
			indexName := filepath.Join(root, "llms.txt")
			if scenario.index != nil {
				require.NoError(t, os.WriteFile(indexName, scenario.index, 0644))
			}
			unrelatedName := filepath.Join(root, "unrelated.txt")
			unrelated := []byte("Unrelated authored sentinel: /assets/a.png\n")
			require.NoError(t, os.WriteFile(unrelatedName, unrelated, 0644))

			readArtifacts := func() map[string][]byte {
				t.Helper()
				artifacts := make(map[string][]byte, 3)
				for _, route := range []string{"/docs/manifest.json", "/docs/search.json", manifest.Articles[0].URL + "index.md"} {
					data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(route, "/"))))
					require.NoError(t, err)
					artifacts[route] = data
				}
				return artifacts
			}
			beforeID := snapshotID(t, readArtifacts())
			assetMap := map[string]string{"/assets/a.png": "https://cdn.example/a-123.png"}
			require.NoError(t, Finalize(root, assetMap))
			artifacts := readArtifacts()
			require.NotEqual(t, beforeID, snapshotID(t, artifacts), "asset/body revision must update manifest and corpus identity")
			markdown := artifacts[manifest.Articles[0].URL+"index.md"]
			require.Contains(t, string(markdown), "![asset](https://cdn.example/a-123.png)")
			require.Contains(t, string(markdown), "`/assets/a.png`", "code samples must remain literal")

			var corpus Corpus
			require.NoError(t, json.Unmarshal(artifacts["/docs/search.json"], &corpus))
			require.Len(t, corpus.Records, 1)
			require.Equal(t, "Final heading", corpus.Records[0].Heading)
			require.Equal(t, "Final transformed text.", corpus.Records[0].Text)
			require.Equal(t, manifest.Articles[0].ID, corpus.Records[0].ArticleID)
			var finalized Manifest
			require.NoError(t, json.Unmarshal(artifacts["/docs/manifest.json"], &finalized))
			require.Equal(t, manifest.Articles[0].MarkdownURL, finalized.Articles[0].MarkdownURL, "advertised article routes must stay stable")
			require.Equal(t, finalized.Articles[0].MarkdownURL, corpus.Records[0].MarkdownURL)
			require.Equal(t, []Heading{{Level: 1, Text: "Final heading", Fragment: "final"}}, finalized.Articles[0].Sections)

			require.NoError(t, Finalize(root, assetMap))
			require.Equal(t, artifacts, readArtifacts(), "documentation finalization must be idempotent")
			index, err := os.ReadFile(indexName)
			if scenario.index == nil {
				require.ErrorIs(t, err, os.ErrNotExist, "docs-only finalization must not create an index")
			} else {
				require.NoError(t, err)
				require.Equal(t, scenario.index, index, "finalization must preserve separately owned discovery bytes")
			}
			data, err := os.ReadFile(unrelatedName)
			require.NoError(t, err)
			require.Equal(t, unrelated, data)
		})
	}
}

func TestFinalizeRejectsIncompleteUnsafeAndAmbiguousInputsBeforeWrites(t *testing.T) {
	for _, scenario := range []string{"missing-html", "missing-search", "duplicate-body", "traversal", "outside-symlink"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			manifest := writeFinalizerFixture(t, root)
			manifestName := filepath.Join(root, "docs/manifest.json")
			htmlName := filepath.Join(root, "docs/guide/start/index.html")
			switch scenario {
			case "missing-html":
				if err := os.Remove(htmlName); err != nil {
					t.Fatal(err)
				}
			case "missing-search":
				if err := os.Remove(filepath.Join(root, "docs/search.json")); err != nil {
					t.Fatal(err)
				}
			case "duplicate-body":
				if err := os.WriteFile(htmlName, []byte("<article class='docs-article'></article><article class='docs-article'></article>"), 0644); err != nil {
					t.Fatal(err)
				}
			case "traversal":
				manifest.Articles[0].ID = "guide/../../outside"
				data, err := json.Marshal(manifest)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(manifestName, data, 0644); err != nil {
					t.Fatal(err)
				}
			case "outside-symlink":
				outside := filepath.Join(t.TempDir(), "outside.html")
				if err := os.WriteFile(outside, []byte("<article class='docs-article'>Outside</article>"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(htmlName); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, htmlName); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(manifestName)
			if err != nil {
				t.Fatal(err)
			}
			if err := Finalize(root, map[string]string{}); err == nil {
				t.Fatal("invalid finalization unexpectedly succeeded")
			}
			after, err := os.ReadFile(manifestName)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("invalid finalization modified the manifest")
			}
		})
	}
	if err := Finalize(t.TempDir(), nil); err == nil {
		t.Fatal("missing manifest unexpectedly succeeded")
	}
}
