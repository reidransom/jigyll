package site

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/reidransom/jigyll/docexport"
	"github.com/reidransom/jigyll/pages"
	"github.com/reidransom/jigyll/plugins"
)

// artifactDocument contains a complete generation, never a lazy renderer or page.
type artifactDocument struct {
	pages.PageEmbed
	body []byte
}

func (d *artifactDocument) Write(w io.Writer) error { _, err := w.Write(d.body); return err }

func (s *Site) prepareArtifacts() error {
	context := plugins.ArtifactContext{}
	for _, document := range s.OutputDocs() {
		if page, ok := document.(Page); ok {
			context.Pages = append(context.Pages, page)
		}
	}
	var plans []*plugins.ArtifactPlan
	if d := s.documentation; d != nil {
		context.Documentation = &plugins.DocumentationArtifacts{Project: docexport.Project{Title: d.title, Description: d.description, OpenAPI: d.openAPI}, Groups: d.groups, Articles: d.articles}
		context.Documentation.Sources = make(map[string]string, len(d.pages))
		for _, page := range d.pages {
			context.Documentation.Sources[page.URL()] = page.Source()
		}
		plan := &plugins.ArtifactPlan{Routes: []string{"/docs/manifest.json", "/docs/search.json"}, Capture: d.pages, Build: d.buildSnapshot}
		for _, article := range d.articles {
			plan.Routes = append(plan.Routes, article.URL+"index.md")
		}
		plans = append(plans, plan)
	}
	if err := s.runHooks(func(plugin plugins.Plugin) error {
		if preparer, ok := plugin.(plugins.ArtifactPreparer); ok {
			plan, err := preparer.PrepareArtifacts(context)
			if err != nil {
				return err
			}
			if plan != nil {
				plans = append(plans, plan)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if len(plans) == 0 {
		return nil
	}
	paths := make([]string, 0, len(s.outputCandidates))
	for _, document := range s.outputCandidates {
		if _, removed := s.removedRoutes[document.URL()]; !removed {
			paths = append(paths, filepath.ToSlash(destinationRelativePath(document)))
		}
	}
	aliases := map[string]string{}
	for _, document := range s.outputCandidates {
		if _, removed := s.removedRoutes[document.URL()]; !removed {
			for _, alias := range localizedRouteAliases(document.URL()) {
				aliases[alias] = document.URL()
			}
		}
	}
	for _, plan := range plans {
		for _, route := range plan.Routes {
			if _, exists := s.Routes[route]; exists {
				return fmt.Errorf("generated artifact route collides with authored output: %s", route)
			}
			for _, alias := range localizedRouteAliases(route) {
				if owner, exists := aliases[alias]; exists {
					return fmt.Errorf("generated artifact alias %s collides with output %s", alias, owner)
				}
				aliases[alias] = route
			}
			paths = append(paths, route)
		}
	}
	if err := docexport.ValidateOutputPaths(paths); err != nil {
		return err
	}
	for _, plan := range plans {
		for _, page := range plan.Capture {
			capture, ok := page.(interface{ CaptureSource() })
			if !ok {
				return fmt.Errorf("source capture unavailable for %s", page.Source())
			}
			capture.CaptureSource()
		}
	}
	if err := s.ensureRendered(); err != nil {
		return err
	}
	// Materialize every plan before registering any exports. Conversion failures
	// therefore cannot clean prior output or publish a partial generation.
	var artifacts []*artifactDocument
	for _, plan := range plans {
		bodies, err := plan.Build()
		if err != nil {
			return err
		}
		if len(bodies) != len(plan.Routes) {
			return fmt.Errorf("generated artifact set does not match prepared routes")
		}
		for _, route := range plan.Routes {
			body, ok := bodies[route]
			if !ok {
				return fmt.Errorf("generated artifact missing: %s", route)
			}
			artifacts = append(artifacts, &artifactDocument{PageEmbed: pages.PageEmbed{Path: route}, body: body})
		}
	}
	for _, artifact := range artifacts {
		s.Routes[artifact.URL()] = artifact
		s.outputCandidates = append(s.outputCandidates, artifact)
	}
	return nil
}
