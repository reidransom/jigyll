package docexport

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"
)

// Assemble creates a complete immutable snapshot. Its SHA-256 input is, in order,
// compact encoding/json manifest and corpus (with empty snapshot IDs), then each
// article's Markdown bytes in manifest order. Every input is prefixed by its
// binary.BigEndian uint64 byte length. Routes are already covered by the manifest.
// Caller metadata is not mutated. Returned Markdown payloads are shared,
// immutable input data; callers must not modify them after assembly.
func Assemble(manifest Manifest, corpus Corpus, markdown map[string][]byte) (map[string][]byte, error) {
	manifest.Articles = append([]Article{}, manifest.Articles...)
	if manifest.Groups == nil {
		manifest.Groups = []Group{}
	}
	corpus.Records = append([]Section{}, corpus.Records...)
	for i := range manifest.Articles {
		if manifest.Articles[i].Sections == nil {
			manifest.Articles[i].Sections = []Heading{}
		}
	}
	for i := range corpus.Records {
		if corpus.Records[i].Hierarchy == nil {
			corpus.Records[i].Hierarchy = []Heading{}
		}
	}
	manifest.SnapshotID, corpus.SnapshotID = "", ""
	if err := validateSnapshot(manifest, corpus, markdown); err != nil {
		return nil, err
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	corpusJSON, err := json.Marshal(corpus)
	if err != nil {
		return nil, err
	}
	hash := sha256.New()
	writePart := func(data []byte) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(data)))
		hash.Write(size[:])
		hash.Write(data)
	}
	writePart(manifestJSON)
	writePart(corpusJSON)
	for _, article := range manifest.Articles {
		writePart(markdown[article.URL+"index.md"])
	}
	id := hex.EncodeToString(hash.Sum(nil))
	manifest.SnapshotID, corpus.SnapshotID = id, id
	manifestJSON, err = json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	corpusJSON, err = json.Marshal(corpus)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(markdown)+3)
	out["/docs/manifest.json"] = append(manifestJSON, '\n')
	out["/docs/search.json"] = append(corpusJSON, '\n')
	out["/llms.txt"] = renderLLMs(manifest)
	for route, data := range markdown {
		out[route] = data
	}
	return out, nil
}

func absoluteHTTP(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Fragment != "" {
		return nil, fmt.Errorf("expected absolute HTTP(S) URL, got %q", raw)
	}
	return u, nil
}

func validateArticle(article Article) error {
	if article.ID == "" || strings.ContainsAny(article.ID, "\\%?#\x00") {
		return fmt.Errorf("unsafe article ID %q", article.ID)
	}
	for _, segment := range strings.Split(article.ID, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("unsafe article ID %q", article.ID)
		}
	}
	route := "/docs/" + article.ID + "/"
	if article.URL != route || path.Clean(route)+"/" != route {
		return fmt.Errorf("article %q: URL must be %q", article.ID, route)
	}
	canonical, err := absoluteHTTP(article.CanonicalURL)
	if err != nil {
		return fmt.Errorf("article %q canonical URL: %w", article.ID, err)
	}
	if canonical.Path != route || canonical.RawPath != "" || canonical.RawQuery != "" {
		return fmt.Errorf("article %q: canonical URL must use route %q without query or encoding", article.ID, route)
	}
	if article.MarkdownURL != article.CanonicalURL+"index.md" {
		return fmt.Errorf("article %q: Markdown URL must be canonical URL plus index.md", article.ID)
	}
	return nil
}

func validateSnapshot(manifest Manifest, corpus Corpus, markdown map[string][]byte) error {
	if err := validateSnapshotMetadata(manifest, corpus); err != nil {
		return err
	}
	groups, err := indexSnapshotGroups(manifest.Groups)
	if err != nil {
		return err
	}
	articles, paths, err := indexSnapshotArticles(manifest.Articles, groups, markdown)
	if err != nil {
		return err
	}
	if err := ValidateOutputPaths(paths); err != nil {
		return err
	}
	if len(markdown) != len(articles) {
		return fmt.Errorf("exported Markdown set does not match manifest articles")
	}
	return validateCorpusRecords(corpus.Records, articles)
}

func validateSnapshotMetadata(manifest Manifest, corpus Corpus) error {
	if manifest.SchemaVersion != SchemaVersion || corpus.SchemaVersion != SchemaVersion {
		return fmt.Errorf("documentation schema must be %d", SchemaVersion)
	}
	if strings.TrimSpace(manifest.Project.Title) == "" {
		return fmt.Errorf("documentation project title is required")
	}
	if manifest.Project.OpenAPI != "" {
		if _, err := absoluteHTTP(manifest.Project.OpenAPI); err != nil {
			return fmt.Errorf("project OpenAPI: %w", err)
		}
	}
	return nil
}

func indexSnapshotGroups(list []Group) (map[string]bool, error) {
	groups := make(map[string]bool, len(list))
	for _, group := range list {
		if group.Slug == "" || groups[group.Slug] {
			return nil, fmt.Errorf("empty or duplicate documentation group %q", group.Slug)
		}
		groups[group.Slug] = true
	}
	return groups, nil
}

func indexSnapshotArticles(list []Article, groups map[string]bool, markdown map[string][]byte) (map[string]Article, []string, error) {
	articles := make(map[string]Article, len(list))
	paths := []string{"/llms.txt", "/docs/manifest.json", "/docs/search.json"}
	for _, article := range list {
		if err := validateArticle(article); err != nil {
			return nil, nil, err
		}
		if _, exists := articles[article.ID]; exists {
			return nil, nil, fmt.Errorf("duplicate article %q", article.ID)
		}
		if !groups[article.NavSection] {
			return nil, nil, fmt.Errorf("article %q: unknown group %q", article.ID, article.NavSection)
		}
		articles[article.ID] = article
		paths = append(paths, article.URL+"index.html", article.URL+"index.md")
		if _, ok := markdown[article.URL+"index.md"]; !ok {
			return nil, nil, fmt.Errorf("article %q: missing exported Markdown", article.ID)
		}
	}
	return articles, paths, nil
}

func validateCorpusRecords(list []Section, articles map[string]Article) error {
	records := make(map[string]bool, len(list))
	for _, record := range list {
		article, ok := articles[record.ArticleID]
		if !ok || records[record.ID] {
			return fmt.Errorf("unknown article or duplicate search record %q", record.ID)
		}
		expectedID, expectedURL := article.ID, article.CanonicalURL
		if record.Fragment != "" {
			expectedID += "#" + record.Fragment
			u, _ := url.Parse(expectedURL)
			u.Fragment = record.Fragment
			expectedURL = u.String()
		}
		if record.ID != expectedID || record.CanonicalURL != expectedURL || record.MarkdownURL != article.MarkdownURL {
			return fmt.Errorf("search record %q: article identity or URL mismatch", record.ID)
		}
		records[record.ID] = true
	}
	return nil
}

// Escape Markdown syntax and collapse line breaks so metadata cannot introduce
// new list items, headings, links, or raw HTML into the llms document.
func llmsText(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	return strings.NewReplacer("\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "{", "\\{", "}", "\\}", "[", "\\[", "]", "\\]", "<", "&lt;", ">", "&gt;", "#", "\\#", "!", "\\!", "|", "\\|").Replace(value)
}

func llmsDestination(value string) string {
	return "<" + strings.NewReplacer("<", "%3C", ">", "%3E", "\n", "%0A", "\r", "%0D").Replace(value) + ">"
}

func renderLLMs(manifest Manifest) []byte {
	var out strings.Builder
	fmt.Fprintf(&out, "# %s\n\n> %s\n\nSnapshot: `%s`\n", llmsText(manifest.Project.Title), llmsText(manifest.Project.Description), manifest.SnapshotID)
	out.WriteString("\n## Downloads\n\n- [Article manifest](/docs/manifest.json): Canonical URLs, Markdown URLs, and section metadata.\n- [Section corpus](/docs/search.json): Downloadable source text for local search.\n")
	for _, group := range manifest.Groups {
		fmt.Fprintf(&out, "\n## %s\n", llmsText(group.Title))
		if group.Description != "" {
			fmt.Fprintf(&out, "\n%s\n", llmsText(group.Description))
		}
		out.WriteByte('\n')
		for _, article := range manifest.Articles {
			if article.NavSection == group.Slug {
				fmt.Fprintf(&out, "- [%s](%s): %s\n", llmsText(article.Title), llmsDestination(article.MarkdownURL), llmsText(article.Description))
			}
		}
	}
	if manifest.Project.OpenAPI != "" {
		fmt.Fprintf(&out, "\n## API specification\n\n- [OpenAPI](%s): Canonical API specification.\n", llmsDestination(manifest.Project.OpenAPI))
	}
	return []byte(out.String())
}
