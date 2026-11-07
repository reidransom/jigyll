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
	context, plans := s.artifactPlans()
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
	if err := s.validateArtifactRoutes(plans); err != nil {
		return err
	}
	for _, plan := range plans {
		if err := captureArtifactSources(plan); err != nil {
			return err
		}
	}
	if err := s.ensureRendered(); err != nil {
		return err
	}
	// Materialize every plan before registering any exports. Conversion failures
	// therefore cannot clean prior output or publish a partial generation.
	var artifacts []*artifactDocument
	for _, plan := range plans {
		bodies, err := buildArtifactBodies(plan)
		if err != nil {
			return err
		}
		for _, route := range plan.Routes {
			artifacts = append(artifacts, &artifactDocument{PageEmbed: pages.PageEmbed{Path: route}, body: bodies[route]})
		}
	}
	for _, artifact := range artifacts {
		s.Routes[artifact.URL()] = artifact
		s.outputCandidates = append(s.outputCandidates, artifact)
	}
	return nil
}

func (s *Site) artifactPlans() (plugins.ArtifactContext, []*plugins.ArtifactPlan) {
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
	return context, plans
}

func (s *Site) validateArtifactRoutes(plans []*plugins.ArtifactPlan) error {
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
	return docexport.ValidateOutputPaths(paths)
}

func captureArtifactSources(plan *plugins.ArtifactPlan) error {
	for _, page := range plan.Capture {
		capture, ok := page.(interface{ CaptureSource() })
		if !ok {
			return fmt.Errorf("source capture unavailable for %s", page.Source())
		}
		capture.CaptureSource()
	}
	return nil
}

func buildArtifactBodies(plan *plugins.ArtifactPlan) (map[string][]byte, error) {
	bodies, err := plan.Build()
	if err != nil {
		return nil, err
	}
	if len(bodies) != len(plan.Routes) {
		return nil, fmt.Errorf("generated artifact set does not match prepared routes")
	}
	for _, route := range plan.Routes {
		if _, ok := bodies[route]; !ok {
			return nil, fmt.Errorf("generated artifact missing: %s", route)
		}
	}
	return bodies, nil
}

// prepareLocalizedArtifacts declares all exports before project validation.
// Its completion barrier runs only after every route and destination is valid.
func (p *localizationProject) prepareLocalizedArtifacts() (func() error, error) {
	enabled, contexts := p.localizedArtifactContexts()
	if !enabled {
		return func() error { return nil }, nil
	}
	aggregate, locales, err := plugins.PrepareLocalizedLLMsArtifacts(p.base, contexts)
	if err != nil {
		return nil, err
	}
	type pendingArtifacts struct {
		plan      *plugins.ArtifactPlan
		documents []*artifactDocument
	}
	var pending []pendingArtifacts
	register := func(owner *Site, plan *plugins.ArtifactPlan, project bool) error {
		if err := captureArtifactSources(plan); err != nil {
			return err
		}
		item := pendingArtifacts{plan: plan}
		for _, route := range plan.Routes {
			document := &artifactDocument{PageEmbed: pages.PageEmbed{Path: route}}
			item.documents = append(item.documents, document)
			if project {
				p.aggregateDocuments = append(p.aggregateDocuments, aggregateDocument{site: owner, document: document})
				p.registerAggregateRoute(route, "jigyll-llms-txt")
			} else {
				owner.outputCandidates = append(owner.outputCandidates, document)
				owner.Routes[route] = document
			}
		}
		pending = append(pending, item)
		return nil
	}
	if err := register(p.prepared[0], aggregate, true); err != nil {
		return nil, err
	}
	for i, plan := range locales {
		if err := register(p.prepared[i], plan, false); err != nil {
			return nil, err
		}
	}
	return func() error {
		for _, localeSite := range p.prepared {
			if err := localeSite.ensureRendered(); err != nil {
				return fmt.Errorf("rendering locale %q: %w", localeSite.localeKey, err)
			}
		}
		for _, item := range pending {
			bodies, err := buildArtifactBodies(item.plan)
			if err != nil {
				return err
			}
			for _, document := range item.documents {
				document.body = bodies[document.URL()]
			}
		}
		return nil
	}, nil
}

func (p *localizationProject) localizedArtifactContexts() (bool, []plugins.LocalizedArtifactContext) {
	enabled := false
	contexts := make([]plugins.LocalizedArtifactContext, 0, len(p.prepared))
	for i, localeSite := range p.prepared {
		if _, found := localeSite.pluginInstances["jigyll-llms-txt"]; found {
			enabled = true
		}
		context := plugins.LocalizedArtifactContext{Site: localeSite, Label: p.locales[i].Label, Prefix: localeSite.localePrefix}
		output := make(map[Page]bool)
		for _, document := range localeSite.OutputDocs() {
			if page, ok := document.(Page); ok {
				context.Pages = append(context.Pages, page)
				output[page] = true
			}
		}
		for _, page := range localeSite.Pages() {
			if !output[page] {
				context.ExcludedRoutes = append(context.ExcludedRoutes, page.URL())
			}
		}
		contexts = append(contexts, context)
	}
	return enabled, contexts
}
