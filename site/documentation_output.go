package site

import (
	"fmt"

	"github.com/reidransom/jigyll/docexport"
)

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
