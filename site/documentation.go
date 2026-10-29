package site

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/reidransom/jigyll/docexport"
)

type documentationInventory struct {
	origin      *url.URL
	groups      []docexport.Group
	articles    []docexport.Article
	pages       []Page
	title       string
	description string
	openAPI     string
	once        sync.Once
	artifacts   map[string][]byte
	err         error
}

func (s *Site) prepareDocumentation() error {
	settings, enabled := s.cfg.Map("documentation")
	if !enabled || len(settings) == 0 || settings["enabled"] == false {
		return nil
	}
	origin, base, err := s.validateDocumentationConfig()
	if err != nil {
		return err
	}
	inventory, positions, err := s.initDocumentationInventory(origin, settings)
	if err != nil {
		return err
	}
	if err := s.collectDocumentationArticles(inventory, positions, base); err != nil {
		return err
	}
	s.documentation = inventory
	return s.addDocumentationRoutes()
}

func (s *Site) validateDocumentationConfig() (*url.URL, string, error) {
	if s.cfg.Incremental {
		return nil, "", fmt.Errorf("documentation snapshots require full builds; disable incremental")
	}
	if s.cfg.Enabled() {
		return nil, "", fmt.Errorf("documentation snapshots do not support localized builds")
	}
	origin, err := url.Parse(s.cfg.AbsoluteURL)
	if err != nil || (origin.Scheme != "https" && origin.Scheme != "http") || origin.Host == "" || origin.RawQuery != "" || origin.Fragment != "" {
		return nil, "", fmt.Errorf("documentation requires an absolute HTTP(S) site url")
	}
	base := strings.TrimSuffix(s.cfg.BaseURL, "/") + "/docs/"
	if base != "/docs/" {
		return nil, "", fmt.Errorf("documentation currently requires an empty baseurl")
	}
	return origin, base, nil
}

func (s *Site) initDocumentationInventory(origin *url.URL, settings map[string]interface{}) (*documentationInventory, map[string]int, error) {
	inventory := &documentationInventory{origin: origin, title: fmt.Sprint(s.cfg.Variables()["title"]), articles: []docexport.Article{}}
	if value, ok := settings["description"].(string); ok {
		inventory.description = value
	}
	if value, ok := settings["openapi"].(string); ok {
		inventory.openAPI = value
	}
	sectionsKey := "docs_sections"
	if value, ok := settings["sections"].(string); ok {
		sectionsKey = value
	}
	data, err := json.Marshal(s.data[sectionsKey])
	if err != nil {
		return nil, nil, err
	}
	if err = json.Unmarshal(data, &inventory.groups); err != nil || len(inventory.groups) == 0 {
		return nil, nil, fmt.Errorf("documentation requires a nonempty %s data list", sectionsKey)
	}
	positions := make(map[string]int, len(inventory.groups))
	for i, group := range inventory.groups {
		if group.Slug == "" || group.Title == "" {
			return nil, nil, fmt.Errorf("documentation groups require slug and title")
		}
		if _, exists := positions[group.Slug]; exists {
			return nil, nil, fmt.Errorf("duplicate documentation group %q", group.Slug)
		}
		positions[group.Slug] = i
	}
	return inventory, positions, nil
}

type docArticleEntry struct {
	page    Page
	article docexport.Article
}

func (s *Site) collectDocumentationArticles(inventory *documentationInventory, positions map[string]int, base string) error {
	var entries []docArticleEntry
	ids := map[string]bool{}
	for _, p := range s.Pages() {
		entry, ok, err := s.inspectDocumentationPage(p, inventory.origin, positions, base, ids)
		if err != nil {
			return err
		}
		if ok {
			entries = append(entries, entry)
			ids[entry.article.ID] = true
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i].article, entries[j].article
		if a.NavSection != b.NavSection {
			return positions[a.NavSection] < positions[b.NavSection]
		}
		if a.NavOrder != b.NavOrder {
			return a.NavOrder < b.NavOrder
		}
		return a.ID < b.ID
	})
	for _, entry := range entries {
		inventory.pages = append(inventory.pages, entry.page)
		inventory.articles = append(inventory.articles, entry.article)
	}
	return nil
}

func (s *Site) inspectDocumentationPage(p Page, origin *url.URL, positions map[string]int, base string, ids map[string]bool) (docArticleEntry, bool, error) {
	fm := p.FrontMatter()
	group := fm.String("nav_section", "")
	if fm.String("layout", "") != "docs" || !fm.Bool("published", true) || fm.Get("redirect_to", nil) != nil {
		return docArticleEntry{}, false, nil
	}
	if _, ok := positions[group]; !ok {
		return docArticleEntry{}, false, nil
	}
	rel, err := filepath.Rel(s.SourceDir(), p.Source())
	if err != nil {
		return docArticleEntry{}, false, err
	}
	if isPlanningPath(rel) {
		return docArticleEntry{}, false, nil
	}
	return s.buildDocArticleEntry(p, fm, rel, group, origin, base, ids)
}

func (s *Site) buildDocArticleEntry(p Page, fm map[string]interface{}, rel, group string, origin *url.URL, base string, ids map[string]bool) (docArticleEntry, bool, error) {
	route := p.URL()
	if !isCleanDocRoute(route, base) {
		return docArticleEntry{}, false, fmt.Errorf("documentation article %s requires a clean /docs/.../ permalink", rel)
	}
	id := strings.TrimSuffix(strings.TrimPrefix(route, base), "/")
	if id == "" || ids[id] {
		return docArticleEntry{}, false, fmt.Errorf("duplicate or empty documentation article ID %q", id)
	}
	if s.Routes[route] != p {
		return docArticleEntry{}, false, fmt.Errorf("documentation route collision at %s", route)
	}
	title, _ := fm["title"].(string)
	description, _ := fm["description"].(string)
	if title == "" || description == "" {
		return docArticleEntry{}, false, fmt.Errorf("documentation article %s requires title and description", rel)
	}
	order, ok := fm["nav_order"].(int)
	if !ok {
		return docArticleEntry{}, false, fmt.Errorf("documentation article %s requires integer nav_order", rel)
	}
	canonical := origin.ResolveReference(&url.URL{Path: route}).String()
	markdown := origin.ResolveReference(&url.URL{Path: route + "index.md"}).String()
	article := docexport.Article{ID: id, Title: title, Description: description, NavSection: group, NavOrder: order, CanonicalURL: canonical, MarkdownURL: markdown, URL: route, Sections: []docexport.Heading{}}
	return docArticleEntry{p, article}, true, nil
}

func isPlanningPath(rel string) bool {
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if strings.HasPrefix(part, ".") || part == "_plans" {
			return true
		}
	}
	return false
}

func isCleanDocRoute(route, base string) bool {
	return strings.HasPrefix(route, base) && strings.HasSuffix(route, "/") && path.Clean(route)+"/" == route && !strings.ContainsAny(route, "?#%\\")
}

func (d *documentationInventory) liquid() map[string]interface{} {
	groups := make([]map[string]interface{}, len(d.groups))
	for i, group := range d.groups {
		groups[i] = map[string]interface{}{"slug": group.Slug, "title": group.Title, "description": group.Description}
	}
	articles := make([]map[string]interface{}, len(d.articles))
	for i, article := range d.articles {
		articles[i] = map[string]interface{}{
			"id": article.ID, "title": article.Title, "description": article.Description,
			"nav_section": article.NavSection, "nav_order": article.NavOrder,
			"url": article.URL, "canonical_url": article.CanonicalURL, "markdown_url": article.MarkdownURL,
			"markdown_path": article.URL + "index.md",
		}
	}
	return map[string]interface{}{"groups": groups, "articles": articles}
}
