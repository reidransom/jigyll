// Package docexport implements static documentation snapshots, independent of layouts.
package docexport

// SchemaVersion changes when the public record contract changes incompatibly.
const SchemaVersion = 1

type Group struct {
	Slug        string `json:"slug" liquid:"slug"`
	Title       string `json:"title" liquid:"title"`
	Description string `json:"description" liquid:"description"`
}

type Heading struct {
	Level    int    `json:"level" liquid:"level"`
	Text     string `json:"text" liquid:"text"`
	Fragment string `json:"fragment" liquid:"fragment"`
}

type Section struct {
	ID           string    `json:"id"`
	ArticleID    string    `json:"article_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	NavSection   string    `json:"nav_section"`
	Heading      string    `json:"heading"`
	Hierarchy    []Heading `json:"hierarchy"`
	Fragment     string    `json:"fragment"`
	Text         string    `json:"text"`
	CanonicalURL string    `json:"canonical_url"`
	MarkdownURL  string    `json:"markdown_url"`
}

type Article struct {
	ID           string    `json:"id" liquid:"id"`
	Title        string    `json:"title" liquid:"title"`
	Description  string    `json:"description" liquid:"description"`
	NavSection   string    `json:"nav_section" liquid:"nav_section"`
	NavOrder     int       `json:"nav_order" liquid:"nav_order"`
	CanonicalURL string    `json:"canonical_url" liquid:"canonical_url"`
	MarkdownURL  string    `json:"markdown_url" liquid:"markdown_url"`
	URL          string    `json:"url" liquid:"url"`
	Sections     []Heading `json:"sections" liquid:"sections"`
}

type Manifest struct {
	SchemaVersion int       `json:"schema_version"`
	SnapshotID    string    `json:"snapshot_id"`
	Groups        []Group   `json:"groups"`
	Articles      []Article `json:"articles"`
}

type Corpus struct {
	SchemaVersion int       `json:"schema_version"`
	SnapshotID    string    `json:"snapshot_id"`
	Records       []Section `json:"records"`
}
