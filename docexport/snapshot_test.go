package docexport

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func snapshotFixture() (Manifest, Corpus, map[string][]byte) {
	article := Article{ID: "guide/start", Title: "Start [here]", Description: "Read *this*", NavSection: "guide", URL: "/docs/guide/start/", CanonicalURL: "https://example.com/docs/guide/start/", MarkdownURL: "https://example.com/docs/guide/start/index.md"}
	return Manifest{SchemaVersion: SchemaVersion, Project: Project{Title: "Example", Description: "Overview", OpenAPI: "https://example.com/openapi.json"}, Groups: []Group{{Slug: "guide", Title: "Guide"}}, Articles: []Article{article}}, Corpus{SchemaVersion: SchemaVersion}, map[string][]byte{article.URL + "index.md": []byte("Original source\n")}
}

func snapshotID(t *testing.T, artifacts map[string][]byte) string {
	t.Helper()
	var manifest Manifest
	if err := json.Unmarshal(artifacts["/docs/manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	var corpus Corpus
	if err := json.Unmarshal(artifacts["/docs/search.json"], &corpus); err != nil {
		t.Fatal(err)
	}
	if manifest.SnapshotID == "" || manifest.SnapshotID != corpus.SnapshotID {
		t.Fatal("snapshot identity differs between manifest and corpus")
	}
	if manifest.Articles[0].Sections == nil || corpus.Records == nil {
		t.Fatal("empty sections and records must be arrays")
	}
	return manifest.SnapshotID
}

func TestAssembleIdentityTracksSemanticInputs(t *testing.T) {
	manifest, corpus, markdown := snapshotFixture()
	first, err := Assemble(manifest, corpus, markdown)
	if err != nil {
		t.Fatal(err)
	}
	id := snapshotID(t, first)
	manifest.SnapshotID, corpus.SnapshotID = "ignored previous ID", "also ignored"
	again, err := Assemble(manifest, corpus, markdown)
	if err != nil {
		t.Fatal(err)
	}
	if snapshotID(t, again) != id || !bytes.Equal(first["/docs/manifest.json"], again["/docs/manifest.json"]) {
		t.Fatal("same content is not deterministic")
	}
	if manifest.Articles[0].Sections != nil || corpus.Records != nil {
		t.Fatal("assembler mutated caller slices")
	}
	manifest.Project.Description = "Changed overview"
	changed, err := Assemble(manifest, corpus, markdown)
	if err != nil {
		t.Fatal(err)
	}
	if snapshotID(t, changed) == id {
		t.Fatal("overview was omitted from identity")
	}
	markdown[manifest.Articles[0].URL+"index.md"] = []byte("Changed source\n")
	changedSource, err := Assemble(manifest, corpus, markdown)
	if err != nil {
		t.Fatal(err)
	}
	if snapshotID(t, changedSource) == snapshotID(t, changed) {
		t.Fatal("Markdown source was omitted from identity")
	}
}

func TestAssembleEmitsOnlyDocumentationArtifacts(t *testing.T) {
	for _, openAPI := range []string{"", "https://reference.example.test/api.json"} {
		t.Run(openAPI, func(t *testing.T) {
			manifest, corpus, markdown := snapshotFixture()
			manifest.Project.OpenAPI = openAPI
			artifacts, err := Assemble(manifest, corpus, markdown)
			require.NoError(t, err)
			snapshotID(t, artifacts)
			markdownRoute := manifest.Articles[0].URL + "index.md"
			require.Len(t, artifacts, 3)
			require.Contains(t, artifacts, "/docs/manifest.json")
			require.Contains(t, artifacts, "/docs/search.json")
			require.Equal(t, markdown[markdownRoute], artifacts[markdownRoute])
			require.NotContains(t, artifacts, "/llms.txt")

			var emitted Manifest
			require.NoError(t, json.Unmarshal(artifacts["/docs/manifest.json"], &emitted))
			require.Equal(t, openAPI, emitted.Project.OpenAPI)
			require.Equal(t, manifest.Articles[0].MarkdownURL, emitted.Articles[0].MarkdownURL)
		})
	}
}
