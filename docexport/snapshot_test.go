package docexport

import (
	"bytes"
	"encoding/json"
	"testing"
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
	if manifest.SnapshotID == "" || manifest.SnapshotID != corpus.SnapshotID || !bytes.Contains(artifacts["/llms.txt"], []byte(manifest.SnapshotID)) {
		t.Fatal("snapshot identity differs across artifacts")
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
	if !bytes.Contains(first["/llms.txt"], []byte("[Start \\[here\\]](<https://example.com/docs/guide/start/index.md>)")) || !bytes.Contains(first["/llms.txt"], []byte("[OpenAPI](<https://example.com/openapi.json>)")) {
		t.Fatalf("llms links are not escaped/canonical: %s", first["/llms.txt"])
	}
}
