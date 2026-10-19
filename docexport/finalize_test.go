package docexport

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestFinalizeUsesFinalBodyAndRevisionMap(t *testing.T) {
	root := t.TempDir()
	manifest := writeFinalizerFixture(t, root)
	if err := Finalize(root, map[string]string{"/assets/a.png": "https://cdn.example/a-123.png"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "docs/guide/start/index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("![asset](https://cdn.example/a-123.png)")) || !bytes.Contains(data, []byte("`/assets/a.png`")) {
		t.Fatalf("lost destination revision or code fidelity: %s", data)
	}
	search, err := os.ReadFile(filepath.Join(root, "docs/search.json"))
	if err != nil {
		t.Fatal(err)
	}
	var corpus Corpus
	if err := json.Unmarshal(search, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Records) != 1 || corpus.Records[0].Heading != "Final heading" || corpus.Records[0].Text != "Final transformed text." || corpus.Records[0].ArticleID != manifest.Articles[0].ID {
		t.Fatalf("search is not extracted from final article body: %+v", corpus.Records)
	}
	before := append([]byte(nil), search...)
	if err := Finalize(root, map[string]string{"/assets/a.png": "https://cdn.example/a-123.png"}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, "docs/search.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("finalization is not idempotent")
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
