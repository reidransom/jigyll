package plugins

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/reidransom/jigyll/docexport"
	"github.com/reidransom/jigyll/internal/llmstxt"
)

func init() { register("jigyll-llms-txt", func() Plugin { return &llmsTxtPlugin{} }) }

type llmsTxtPlugin struct {
	plugin
	site Site
}

func (p *llmsTxtPlugin) AfterInitSite(s Site) error { p.site = s; return nil }

type llmsSettings struct {
	project   llmstxt.Project
	markdown  bool
	sections  []llmstxt.Section
	curated   bool
	overrides map[string]llmsOverrides
}

type llmsOverrides struct{ title, description bool }

type llmsCompanion struct {
	page             Page
	route, canonical string
}

type llmsCanonical struct {
	origin *url.URL
	base   string
}

func prepareLLMsCanonical(site Site) (llmsCanonical, error) {
	cfg := site.Config()
	origin, err := url.Parse(cfg.AbsoluteURL)
	if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || (origin.Path != "" && origin.Path != "/") {
		return llmsCanonical{}, fmt.Errorf("llms_txt requires an absolute HTTP(S) canonical site url without a path")
	}
	base := strings.TrimSuffix(cfg.BaseURL, "/")
	if base != "" && (!strings.HasPrefix(base, "/") || path.Clean(base) != base || strings.ContainsAny(base, "?#%\\")) {
		return llmsCanonical{}, fmt.Errorf("llms_txt requires a clean baseurl path")
	}
	return llmsCanonical{origin: origin, base: base}, nil
}

func (c llmsCanonical) url(route string) string {
	return c.origin.ResolveReference(&url.URL{Path: c.base + route}).String()
}

type llmsRouteInventory struct {
	pages    map[string]Page
	prefixes []string
	// available is shared only during preparation. False means at least one
	// actual matching edition is excluded; nil keeps standalone missing errors.
	available map[string]bool
}

func stripLLMsPrefix(route, prefix string) string {
	if prefix == "" {
		return route
	}
	if route == "/"+prefix {
		return "/"
	}
	if strings.HasPrefix(route, "/"+prefix+"/") {
		return strings.TrimPrefix(route, "/"+prefix)
	}
	return route
}

func (r llmsRouteInventory) literal(route string) string {
	for _, prefix := range r.prefixes {
		if route == "/"+prefix || strings.HasPrefix(route, "/"+prefix+"/") {
			return stripLLMsPrefix(route, prefix)
		}
	}
	return route
}

func (p *llmsTxtPlugin) PrepareArtifacts(ctx ArtifactContext) (*ArtifactPlan, error) {
	canonical, err := prepareLLMsCanonical(p.site)
	if err != nil {
		return nil, err
	}
	settings, err := p.settings(ctx.Documentation)
	if err != nil {
		return nil, err
	}
	inventory := llmsRouteInventory{pages: make(map[string]Page, len(ctx.Pages))}
	for _, page := range ctx.Pages {
		inventory.pages[page.URL()] = page
	}
	plan, sections, err := p.prepareSelection(ctx.Documentation, settings, canonical, inventory)
	if err != nil {
		return nil, err
	}
	addLLMsIndex(plan, settings.project, sections)
	return plan, nil
}

// PrepareLocalizedLLMsArtifacts prepares one aggregate index and one companion
// plan per locale, in input order. All routes are project-relative and already
// include routing prefixes. It selects only actual supplied editions, never
// translated identities or fallback article bodies.
func PrepareLocalizedLLMsArtifacts(project Site, locales []LocalizedArtifactContext) (*ArtifactPlan, []*ArtifactPlan, error) {
	canonical, err := prepareLLMsCanonical(project)
	if err != nil {
		return nil, nil, err
	}
	projectSettings, err := (&llmsTxtPlugin{site: project}).settings(nil)
	if err != nil {
		return nil, nil, err
	}
	var prefixes []string
	for _, locale := range locales {
		if locale.Prefix != "" {
			prefixes = append(prefixes, locale.Prefix)
		}
	}
	// A nested routing prefix must win over its ancestor.
	sort.SliceStable(prefixes, func(i, j int) bool { return len(prefixes[i]) > len(prefixes[j]) })
	available := map[string]bool{}
	inventories := make([]llmsRouteInventory, len(locales))
	for i, locale := range locales {
		p := &llmsTxtPlugin{site: locale.Site}
		inventory := llmsRouteInventory{pages: make(map[string]Page, len(locale.Pages)), prefixes: prefixes, available: available}
		for _, page := range locale.Pages {
			route := stripLLMsPrefix(page.URL(), locale.Prefix)
			inventory.pages[route] = page
			eligible := p.eligible(page, route)
			if previous, exists := available[route]; !exists || previous {
				available[route] = eligible
			}
		}
		for _, route := range locale.ExcludedRoutes {
			available[stripLLMsPrefix(route, locale.Prefix)] = false
		}
		inventories[i] = inventory
	}
	plans := make([]*ArtifactPlan, len(locales))
	var sections []llmstxt.Section
	for i, locale := range locales {
		p := &llmsTxtPlugin{site: locale.Site}
		settings, err := p.settings(nil)
		if err != nil {
			return nil, nil, err
		}
		plan, selected, err := p.prepareSelection(nil, settings, canonical, inventories[i])
		if err != nil {
			return nil, nil, err
		}
		plans[i] = plan
		for _, section := range selected {
			if len(section.Entries) != 0 {
				section.Title = locale.Label + " — " + section.Title
				sections = append(sections, section)
			}
		}
	}
	aggregate := &ArtifactPlan{Build: func() (map[string][]byte, error) { return map[string][]byte{}, nil }}
	addLLMsIndex(aggregate, projectSettings.project, sections)
	return aggregate, plans, nil
}

type llmsSelection struct {
	plugin     *llmsTxtPlugin
	settings   llmsSettings
	canonical  llmsCanonical
	inventory  llmsRouteInventory
	docs       *DocumentationArtifacts
	owned      map[string]docexport.Article
	seen       map[string]bool
	plan       *ArtifactPlan
	companions []llmsCompanion
}

func (p *llmsTxtPlugin) prepareSelection(docs *DocumentationArtifacts, settings llmsSettings, canonical llmsCanonical, inventory llmsRouteInventory) (*ArtifactPlan, []llmstxt.Section, error) {
	sections := settings.sections
	if !settings.curated {
		sections = p.defaultLLMsSections(sections, docs, inventory.pages)
	}
	selection := llmsSelection{
		plugin: p, settings: settings, canonical: canonical, inventory: inventory, docs: docs,
		owned: map[string]docexport.Article{}, seen: map[string]bool{}, plan: &ArtifactPlan{},
	}
	if docs != nil {
		for _, article := range docs.Articles {
			selection.owned[article.URL] = article
		}
	}
	for i := range sections {
		selected := sections[i].Entries[:0]
		for _, entry := range sections[i].Entries {
			entry, include, err := selection.prepareEntry(entry)
			if err != nil {
				return nil, nil, err
			}
			if include {
				selected = append(selected, entry)
			}
		}
		sections[i].Entries = selected
	}
	if docs != nil {
		sections = addLLMsDocumentationSections(sections, docs, canonical)
	}
	companions := selection.companions
	selection.plan.Build = func() (map[string][]byte, error) {
		return buildLLMsCompanions(companions)
	}
	return selection.plan, sections, nil
}

func (p *llmsTxtPlugin) defaultLLMsSections(sections []llmstxt.Section, docs *DocumentationArtifacts, routes map[string]Page) []llmstxt.Section {
	if docs != nil {
		return p.documentationLLMsSections(sections, docs, routes)
	}
	cfg := p.site.Config()
	var sorted []string
	for route, page := range routes {
		if p.eligible(page, route) && cfg.IsMarkdown(page.Source()) {
			sorted = append(sorted, route)
		}
	}
	sort.Strings(sorted)
	if len(sorted) == 0 {
		return sections
	}
	section := llmstxt.Section{Title: "Pages"}
	for _, route := range sorted {
		section.Entries = append(section.Entries, llmstxt.Entry{URL: route})
	}
	return append(sections, section)
}

func (p *llmsTxtPlugin) documentationLLMsSections(sections []llmstxt.Section, docs *DocumentationArtifacts, routes map[string]Page) []llmstxt.Section {
	for _, group := range docs.Groups {
		section := llmstxt.Section{Title: group.Title, Description: group.Description}
		for _, article := range docs.Articles {
			if page, exists := routes[article.URL]; article.NavSection == group.Slug && exists && p.eligible(page, article.URL) {
				section.Entries = append(section.Entries, llmstxt.Entry{URL: article.URL, Title: article.Title, Description: article.Description})
			}
		}
		sections = append(sections, section)
	}
	return sections
}

func (s *llmsSelection) prepareEntry(entry llmstxt.Entry) (llmstxt.Entry, bool, error) {
	route, local, err := localLLMsRoute(entry.URL, s.canonical.origin, s.canonical.base)
	if err != nil {
		return entry, false, err
	}
	identity := entry.URL
	if local {
		if s.settings.curated {
			route = s.inventory.literal(route)
		}
		identity = route
	}
	if s.seen[identity] {
		return entry, false, fmt.Errorf("duplicate llms_txt entry %q", entry.URL)
	}
	s.seen[identity] = true
	if !local {
		if strings.TrimSpace(entry.Title) == "" {
			return entry, false, fmt.Errorf("external llms_txt entry %q requires title", entry.URL)
		}
		return entry, true, nil
	}
	page, err := s.localPage(route, entry.URL)
	if err != nil || page == nil {
		return entry, false, err
	}
	entry, err = inheritLLMsEntry(entry, page, route, s.settings.overrides[entry.URL])
	if err != nil {
		return entry, false, err
	}
	entry.URL, err = s.prepareCompanion(page)
	return entry, err == nil, err
}

func (s *llmsSelection) localPage(route, destination string) (Page, error) {
	if eligible, exists := s.inventory.available[route]; exists && !eligible && s.settings.curated {
		return nil, fmt.Errorf("llms_txt local route %q is missing or excluded", destination)
	}
	page, found := s.inventory.pages[route]
	if !found {
		if _, exists := s.inventory.available[route]; exists {
			return nil, nil
		}
		return nil, fmt.Errorf("llms_txt local route %q is missing or excluded", destination)
	}
	if !s.plugin.eligible(page, route) {
		return nil, fmt.Errorf("llms_txt local route %q is missing or excluded", destination)
	}
	return page, nil
}

func inheritLLMsEntry(entry llmstxt.Entry, page Page, route string, overrides llmsOverrides) (llmstxt.Entry, error) {
	var err error
	if entry.Title == "" && !overrides.title {
		entry.Title, err = optionalLLMsString(page.FrontMatter(), "title")
		if err != nil {
			return entry, err
		}
	}
	if entry.Description == "" && !overrides.description {
		entry.Description, err = optionalLLMsString(page.FrontMatter(), "description")
		if err != nil {
			return entry, err
		}
	}
	if strings.TrimSpace(entry.Title) == "" {
		return entry, fmt.Errorf("llms_txt route %s requires title", route)
	}
	return entry, nil
}

func (s *llmsSelection) prepareCompanion(page Page) (string, error) {
	actual := page.URL()
	destination := s.canonical.url(actual)
	if !s.settings.markdown || !s.plugin.site.Config().IsMarkdown(page.Source()) {
		return destination, nil
	}
	md := markdownCompanionRoute(actual)
	if article, exists := s.owned[actual]; exists {
		if s.docs.Sources[actual] != page.Source() || article.CanonicalURL != destination || article.MarkdownURL != s.canonical.url(md) || article.URL+"index.md" != md {
			return "", fmt.Errorf("llms_txt documentation ownership mismatch at %s", actual)
		}
	} else {
		s.plan.Routes = append(s.plan.Routes, md)
		s.plan.Capture = append(s.plan.Capture, page)
		s.companions = append(s.companions, llmsCompanion{page: page, route: md, canonical: destination})
	}
	return s.canonical.url(md), nil
}

func addLLMsDocumentationSections(sections []llmstxt.Section, docs *DocumentationArtifacts, canonical llmsCanonical) []llmstxt.Section {
	downloads := llmstxt.Section{Title: "Downloads", Entries: []llmstxt.Entry{
		{Title: "Article manifest", URL: canonical.url("/docs/manifest.json"), Description: "Canonical URLs, Markdown URLs, and section metadata."},
		{Title: "Section corpus", URL: canonical.url("/docs/search.json"), Description: "Downloadable source text for local search."},
	}}
	sections = append([]llmstxt.Section{downloads}, sections...)
	if api := docs.Project.OpenAPI; api != "" {
		sections = append(sections, llmstxt.Section{Title: "API specification", Entries: []llmstxt.Entry{{Title: "OpenAPI", URL: api, Description: "Canonical API specification."}}})
	}
	return sections
}

func buildLLMsCompanions(companions []llmsCompanion) (map[string][]byte, error) {
	artifacts := make(map[string][]byte, len(companions))
	for _, companion := range companions {
		body, err := companion.page.Article()
		if err != nil {
			return nil, err
		}
		if body.Source == nil {
			return nil, fmt.Errorf("llms_txt source capture unavailable for %s", companion.page.Source())
		}
		markdown, err := docexport.Markdown(body.Source, companion.canonical, nil)
		if err != nil {
			return nil, fmt.Errorf("llms_txt %s: %w", companion.route, err)
		}
		artifacts[companion.route] = markdown
	}
	return artifacts, nil
}

func addLLMsIndex(plan *ArtifactPlan, project llmstxt.Project, sections []llmstxt.Section) {
	plan.Routes = append([]string{"/llms.txt"}, plan.Routes...)
	build := plan.Build
	plan.Build = func() (map[string][]byte, error) {
		artifacts, err := build()
		if err != nil {
			return nil, err
		}
		var out strings.Builder
		llmstxt.WriteHeader(&out, project)
		llmstxt.WriteSections(&out, sections)
		artifacts["/llms.txt"] = []byte(out.String())
		return artifacts, nil
	}
}

func (p *llmsTxtPlugin) settings(docs *DocumentationArtifacts) (llmsSettings, error) {
	cfg := p.site.Config()
	result := llmsSettings{markdown: true, overrides: map[string]llmsOverrides{}}
	var err error
	result.project.Title, err = optionalLLMsString(cfg.Variables(), "title")
	if err != nil {
		return result, err
	}
	result.project.Description, err = optionalLLMsString(cfg.Variables(), "description")
	if err != nil {
		return result, err
	}
	if docs != nil {
		if docs.Project.Title != "" {
			result.project.Title = docs.Project.Title
		}
		if docs.Project.Description != "" {
			result.project.Description = docs.Project.Description
		}
	}
	_, exists := cfg.Variables()["llms_txt"]
	if exists {
		settings, ok := cfg.Map("llms_txt")
		if !ok {
			return result, fmt.Errorf("llms_txt must be a map")
		}
		if err := result.applyLLMsSettings(settings); err != nil {
			return result, err
		}
	}
	if strings.TrimSpace(result.project.Title) == "" {
		return result, fmt.Errorf("llms_txt requires a nonempty title")
	}
	return result, nil
}

func (s *llmsSettings) applyLLMsSettings(settings map[string]interface{}) error {
	for _, field := range []string{"title", "description"} {
		if _, found := settings[field]; found {
			value, err := optionalLLMsString(settings, field)
			if err != nil {
				return err
			}
			if field == "title" {
				s.project.Title = value
			} else {
				s.project.Description = value
			}
		}
	}
	if raw, found := settings["markdown"]; found {
		value, ok := raw.(bool)
		if !ok {
			return fmt.Errorf("llms_txt markdown must be a boolean")
		}
		s.markdown = value
	}
	if raw, found := settings["sections"]; found {
		s.curated = true
		list, ok := raw.([]interface{})
		if !ok {
			return fmt.Errorf("llms_txt sections must be a list")
		}
		for _, rawSection := range list {
			section, err := parseLLMsSection(rawSection, s.overrides)
			if err != nil {
				return err
			}
			s.sections = append(s.sections, section)
		}
	}
	return nil
}

func parseLLMsSection(raw interface{}, overrides map[string]llmsOverrides) (llmstxt.Section, error) {
	section, ok := llmsMap(raw)
	if !ok {
		return llmstxt.Section{}, fmt.Errorf("llms_txt section must be a map")
	}
	title, err := optionalLLMsString(section, "title")
	if err != nil {
		return llmstxt.Section{}, err
	}
	if strings.TrimSpace(title) == "" {
		return llmstxt.Section{}, fmt.Errorf("llms_txt section requires title")
	}
	description, err := optionalLLMsString(section, "description")
	if err != nil {
		return llmstxt.Section{}, err
	}
	output := llmstxt.Section{Title: title, Description: description}
	entries, ok := section["entries"].([]interface{})
	if !ok {
		return output, fmt.Errorf("llms_txt section entries must be a list")
	}
	for _, rawEntry := range entries {
		entry, override, err := parseLLMsEntry(rawEntry)
		if err != nil {
			return output, err
		}
		overrides[entry.URL] = override
		output.Entries = append(output.Entries, entry)
	}
	return output, nil
}

func parseLLMsEntry(raw interface{}) (llmstxt.Entry, llmsOverrides, error) {
	entry, ok := llmsMap(raw)
	if !ok {
		return llmstxt.Entry{}, llmsOverrides{}, fmt.Errorf("llms_txt entry must be a map")
	}
	destination, err := optionalLLMsString(entry, "url")
	if err != nil {
		return llmstxt.Entry{}, llmsOverrides{}, err
	}
	if destination == "" {
		return llmstxt.Entry{}, llmsOverrides{}, fmt.Errorf("llms_txt entry requires url")
	}
	title, err := optionalLLMsString(entry, "title")
	if err != nil {
		return llmstxt.Entry{}, llmsOverrides{}, err
	}
	description, err := optionalLLMsString(entry, "description")
	if err != nil {
		return llmstxt.Entry{}, llmsOverrides{}, err
	}
	_, titleSet := entry["title"]
	_, descriptionSet := entry["description"]
	return llmstxt.Entry{URL: destination, Title: title, Description: description}, llmsOverrides{title: titleSet, description: descriptionSet}, nil
}

func optionalLLMsString(values map[string]interface{}, name string) (string, error) {
	raw, exists := values[name]
	if !exists {
		return "", nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("llms_txt %s must be a string", name)
	}
	return value, nil
}

func llmsMap(raw interface{}) (map[string]interface{}, bool) {
	switch value := raw.(type) {
	case map[string]interface{}:
		return value, true
	case map[interface{}]interface{}:
		result := make(map[string]interface{}, len(value))
		for key, item := range value {
			name, ok := key.(string)
			if !ok {
				return nil, false
			}
			result[name] = item
		}
		return result, true
	default:
		return nil, false
	}
}

func (p *llmsTxtPlugin) eligible(page Page, route string) bool {
	if !llmsSourcePage(page) {
		return false
	}
	fm := page.FrontMatter()
	if fm.Get("llms", true) == false || fm.Get("draft", false) == true || fm.Get("redirect_to", nil) != nil {
		return false
	}
	if !llmsHTMLRoute(page, route) {
		return false
	}
	if (page.IsPost() || fm.Get("date", nil) != nil) && page.PostDate().After(time.Now()) {
		return false
	}
	if !llmsVisibleSource(p.site.Config().Source, page.Source()) {
		return false
	}
	return !llmsNoindex(fm.Get("robots", nil))
}

func llmsSourcePage(page Page) bool {
	provenance, ok := page.(interface{ SourceBacked() bool })
	return ok && provenance.SourceBacked() && page.Source() != "" && !page.IsStatic() && page.Published()
}

func llmsHTMLRoute(page Page, route string) bool {
	if page.OutputExt() != ".html" && page.OutputExt() != ".htm" && page.OutputExt() != ".xhtml" {
		return false
	}
	return route != "/404.html" && route != "/404/" && route != "/404"
}

func llmsVisibleSource(root, source string) bool {
	rel, err := filepath.Rel(root, source)
	if err != nil {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if strings.HasPrefix(part, ".") || part == "_plans" || part == "_drafts" {
			return false
		}
	}
	return true
}

func llmsNoindex(raw interface{}) bool {
	switch robots := raw.(type) {
	case string:
		for _, directive := range strings.FieldsFunc(strings.ToLower(robots), func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' }) {
			if directive == "noindex" {
				return true
			}
		}
	case []interface{}:
		for _, directive := range robots {
			if text, ok := directive.(string); ok && strings.EqualFold(strings.TrimSpace(text), "noindex") {
				return true
			}
		}
	}
	return false
}

func markdownCompanionRoute(route string) string {
	if strings.HasSuffix(route, "/") {
		return route + "index.md"
	}
	return route + ".md"
}

func localLLMsRoute(destination string, origin *url.URL, base string) (string, bool, error) {
	parsed, err := url.Parse(destination)
	if err != nil || parsed.User != nil || strings.ContainsAny(destination, "\r\n\\") {
		return "", false, fmt.Errorf("unsafe llms_txt destination %q", destination)
	}
	local := !parsed.IsAbs() && parsed.Host == ""
	if !local {
		if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return "", false, fmt.Errorf("llms_txt destination must use HTTP(S): %q", destination)
		}
		local = sameLLMsDeployment(parsed, origin, base)
		if !local {
			return "", false, nil
		}
	}
	route, clean := cleanLLMsRoute(parsed, base)
	if !clean {
		return "", false, fmt.Errorf("llms_txt local destination must be a clean canonical route: %q", destination)
	}
	return route, true, nil
}

func sameLLMsDeployment(destination, origin *url.URL, base string) bool {
	return strings.EqualFold(destination.Scheme, origin.Scheme) &&
		strings.EqualFold(destination.Hostname(), origin.Hostname()) &&
		llmsURLPort(destination) == llmsURLPort(origin) &&
		(base == "" || destination.Path == base || strings.HasPrefix(destination.Path, base+"/"))
}

func cleanLLMsRoute(parsed *url.URL, base string) (string, bool) {
	route := parsed.Path
	if parsed.IsAbs() && base != "" {
		route = strings.TrimPrefix(route, base)
	}
	if route == "" {
		route = "/"
	}
	clean := path.Clean(route)
	if strings.HasSuffix(route, "/") && clean != "/" {
		clean += "/"
	}
	return route, strings.HasPrefix(route, "/") && route == clean && parsed.RawQuery == "" && parsed.Fragment == "" && !strings.ContainsAny(route, "%?#\\") && parsed.RawPath == ""
}

func llmsURLPort(destination *url.URL) string {
	if port := destination.Port(); port != "" {
		return port
	}
	if strings.EqualFold(destination.Scheme, "https") {
		return "443"
	}
	return "80"
}
