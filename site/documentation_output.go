package site

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/reidransom/jigyll/docexport"
	"github.com/reidransom/jigyll/pages"
)

// Generated artifacts are ordinary output documents, never source pages. Both
// the HTTP preview and disk builds read the same complete, immutable snapshot.
type documentationDocument struct {
	pages.PageEmbed
	site *Site
}

func (d *documentationDocument) Write(w io.Writer) error {
	if err := d.site.ensureRendered(); err != nil {
		return err
	}
	artifacts, err := d.site.documentation.snapshot()
	if err != nil {
		return err
	}
	body, found := artifacts[d.Path]
	if !found {
		return fmt.Errorf("documentation artifact missing: %s", d.Path)
	}
	_, err = w.Write(body)
	return err
}

func (s *Site) addDocumentationRoutes() error {
	routes := []string{"/llms.txt", "/docs/manifest.json", "/docs/search.json"}
	for _, article := range s.documentation.articles {
		routes = append(routes, article.URL+"index.md")
	}
	// Check the entire namespace before registering any generated documents.
	for _, route := range routes {
		if _, exists := s.Routes[route]; exists {
			return fmt.Errorf("generated documentation route collides with authored output: %s", route)
		}
	}
	paths := make([]string, 0, len(s.Routes)+len(routes))
	for _, document := range s.Routes {
		paths = append(paths, filepath.ToSlash(destinationRelativePath(document)))
	}
	paths = append(paths, routes...)
	if err := docexport.ValidateOutputPaths(paths); err != nil {
		return err
	}
	for _, route := range routes {
		s.AddDocument(&documentationDocument{PageEmbed: pages.PageEmbed{Path: route}, site: s}, true)
	}
	return nil
}

func (d *documentationInventory) snapshot() (map[string][]byte, error) {
	d.once.Do(func() { d.artifacts, d.err = d.buildSnapshot() })
	return d.artifacts, d.err
}

func (d *documentationInventory) buildSnapshot() (map[string][]byte, error) {
	manifest := docexport.Manifest{
		SchemaVersion: docexport.SchemaVersion,
		Project:       docexport.Project{Title: d.title, Description: d.description, OpenAPI: d.openAPI},
		Groups:        d.groups, Articles: append([]docexport.Article{}, d.articles...),
	}
	corpus := docexport.Corpus{SchemaVersion: docexport.SchemaVersion, Records: []docexport.Section{}}
	markdown := make(map[string][]byte, len(d.articles))
	for i, p := range d.pages {
		body, err := p.Article()
		if err != nil {
			return nil, err
		}
		if body.Source == nil {
			return nil, fmt.Errorf("documentation source capture unavailable for %s", p.Source())
		}
		article := &manifest.Articles[i]
		payload, err := docexport.Markdown(body.Source, article.CanonicalURL, nil)
		if err != nil {
			return nil, err
		}
		records, headings, err := docexport.Sections(*article, body.HTML)
		if err != nil {
			return nil, err
		}
		article.Sections = headings
		corpus.Records = append(corpus.Records, records...)
		markdown[article.URL+"index.md"] = payload
	}
	return docexport.Assemble(manifest, corpus, markdown)
}
