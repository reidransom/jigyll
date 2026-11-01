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

func (p *llmsTxtPlugin) prepareSelection(docs *DocumentationArtifacts, settings llmsSettings, canonical llmsCanonical, inventory llmsRouteInventory) (*ArtifactPlan, []llmstxt.Section, error) {
	cfg := p.site.Config()
	routes := inventory.pages
	sections := settings.sections
	if !settings.curated {
		if docs != nil {
			for _, group := range docs.Groups {
				section := llmstxt.Section{Title: group.Title, Description: group.Description}
				for _, article := range docs.Articles {
					if page, exists := routes[article.URL]; article.NavSection == group.Slug && exists && p.eligible(page, article.URL) {
						section.Entries = append(section.Entries, llmstxt.Entry{URL: article.URL, Title: article.Title, Description: article.Description})
					}
				}
				sections = append(sections, section)
			}
		} else {
			var sorted []string
			for route, page := range routes {
				if p.eligible(page, route) && cfg.IsMarkdown(page.Source()) {
					sorted = append(sorted, route)
				}
			}
			sort.Strings(sorted)
			if len(sorted) > 0 {
				section := llmstxt.Section{Title: "Pages"}
				for _, route := range sorted {
					section.Entries = append(section.Entries, llmstxt.Entry{URL: route})
				}
				sections = append(sections, section)
			}
		}
	}
	owned := map[string]docexport.Article{}
	if docs != nil {
		for _, article := range docs.Articles {
			owned[article.URL] = article
		}
	}
	plan := &ArtifactPlan{}
	var companions []llmsCompanion
	seen := map[string]bool{}
	for i := range sections {
		selected := sections[i].Entries[:0]
		for _, entry := range sections[i].Entries {
			route, local, err := localLLMsRoute(entry.URL, canonical.origin, canonical.base)
			if err != nil {
				return nil, nil, err
			}
			identity := entry.URL
			if local {
				if settings.curated {
					route = inventory.literal(route)
				}
				identity = route
			}
			if seen[identity] {
				return nil, nil, fmt.Errorf("duplicate llms_txt entry %q", entry.URL)
			}
			seen[identity] = true
			if !local {
				if strings.TrimSpace(entry.Title) == "" {
					return nil, nil, fmt.Errorf("external llms_txt entry %q requires title", entry.URL)
				}
				selected = append(selected, entry)
				continue
			}
			if eligible, exists := inventory.available[route]; exists && !eligible && settings.curated {
				return nil, nil, fmt.Errorf("llms_txt local route %q is missing or excluded", entry.URL)
			}
			page, found := routes[route]
			if !found {
				if _, exists := inventory.available[route]; exists {
					continue
				}
				return nil, nil, fmt.Errorf("llms_txt local route %q is missing or excluded", entry.URL)
			}
			if !p.eligible(page, route) {
				return nil, nil, fmt.Errorf("llms_txt local route %q is missing or excluded", entry.URL)
			}
			overrides := settings.overrides[entry.URL]
			if entry.Title == "" && !overrides.title {
				entry.Title, err = optionalLLMsString(page.FrontMatter(), "title")
				if err != nil {
					return nil, nil, err
				}
			}
			if entry.Description == "" && !overrides.description {
				entry.Description, err = optionalLLMsString(page.FrontMatter(), "description")
				if err != nil {
					return nil, nil, err
				}
			}
			if strings.TrimSpace(entry.Title) == "" {
				return nil, nil, fmt.Errorf("llms_txt route %s requires title", route)
			}
			actual := page.URL()
			entry.URL = canonical.url(actual)
			if settings.markdown && cfg.IsMarkdown(page.Source()) {
				md := markdownCompanionRoute(actual)
				if article, exists := owned[actual]; exists {
					if docs.Sources[actual] != page.Source() || article.CanonicalURL != canonical.url(actual) || article.MarkdownURL != canonical.url(md) || article.URL+"index.md" != md {
						return nil, nil, fmt.Errorf("llms_txt documentation ownership mismatch at %s", actual)
					}
				} else {
					plan.Routes = append(plan.Routes, md)
					plan.Capture = append(plan.Capture, page)
					companions = append(companions, llmsCompanion{page: page, route: md, canonical: canonical.url(actual)})
				}
				entry.URL = canonical.url(md)
			}
			selected = append(selected, entry)
		}
		sections[i].Entries = selected
	}
	if docs != nil {
		downloads := llmstxt.Section{Title: "Downloads", Entries: []llmstxt.Entry{
			{Title: "Article manifest", URL: canonical.url("/docs/manifest.json"), Description: "Canonical URLs, Markdown URLs, and section metadata."},
			{Title: "Section corpus", URL: canonical.url("/docs/search.json"), Description: "Downloadable source text for local search."},
		}}
		sections = append([]llmstxt.Section{downloads}, sections...)
		if api := docs.Project.OpenAPI; api != "" {
			sections = append(sections, llmstxt.Section{Title: "API specification", Entries: []llmstxt.Entry{{Title: "OpenAPI", URL: api, Description: "Canonical API specification."}}})
		}
	}
	plan.Build = func() (map[string][]byte, error) {
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
	return plan, sections, nil
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
		for _, field := range []string{"title", "description"} {
			if _, found := settings[field]; found {
				value, err := optionalLLMsString(settings, field)
				if err != nil {
					return result, err
				}
				if field == "title" {
					result.project.Title = value
				} else {
					result.project.Description = value
				}
			}
		}
		if raw, found := settings["markdown"]; found {
			value, ok := raw.(bool)
			if !ok {
				return result, fmt.Errorf("llms_txt markdown must be a boolean")
			}
			result.markdown = value
		}
		if raw, found := settings["sections"]; found {
			result.curated = true
			list, ok := raw.([]interface{})
			if !ok {
				return result, fmt.Errorf("llms_txt sections must be a list")
			}
			for _, rawSection := range list {
				section, ok := llmsMap(rawSection)
				if !ok {
					return result, fmt.Errorf("llms_txt section must be a map")
				}
				title, err := optionalLLMsString(section, "title")
				if err != nil {
					return result, err
				}
				if strings.TrimSpace(title) == "" {
					return result, fmt.Errorf("llms_txt section requires title")
				}
				description, err := optionalLLMsString(section, "description")
				if err != nil {
					return result, err
				}
				output := llmstxt.Section{Title: title, Description: description}
				entries, ok := section["entries"].([]interface{})
				if !ok {
					return result, fmt.Errorf("llms_txt section entries must be a list")
				}
				for _, rawEntry := range entries {
					entry, ok := llmsMap(rawEntry)
					if !ok {
						return result, fmt.Errorf("llms_txt entry must be a map")
					}
					destination, err := optionalLLMsString(entry, "url")
					if err != nil {
						return result, err
					}
					if destination == "" {
						return result, fmt.Errorf("llms_txt entry requires url")
					}
					title, err := optionalLLMsString(entry, "title")
					if err != nil {
						return result, err
					}
					description, err := optionalLLMsString(entry, "description")
					if err != nil {
						return result, err
					}
					_, titleSet := entry["title"]
					_, descriptionSet := entry["description"]
					result.overrides[destination] = llmsOverrides{title: titleSet, description: descriptionSet}
					output.Entries = append(output.Entries, llmstxt.Entry{URL: destination, Title: title, Description: description})
				}
				result.sections = append(result.sections, output)
			}
		}
	}
	if strings.TrimSpace(result.project.Title) == "" {
		return result, fmt.Errorf("llms_txt requires a nonempty title")
	}
	return result, nil
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
	provenance, ok := page.(interface{ SourceBacked() bool })
	if !ok || !provenance.SourceBacked() || page.Source() == "" || page.IsStatic() || !page.Published() {
		return false
	}
	fm := page.FrontMatter()
	if fm.Get("llms", true) == false || fm.Get("draft", false) == true || fm.Get("redirect_to", nil) != nil {
		return false
	}
	if page.OutputExt() != ".html" && page.OutputExt() != ".htm" && page.OutputExt() != ".xhtml" {
		return false
	}
	if route == "/404.html" || route == "/404/" || route == "/404" {
		return false
	}
	if (page.IsPost() || fm.Get("date", nil) != nil) && page.PostDate().After(time.Now()) {
		return false
	}
	rel, err := filepath.Rel(p.site.Config().Source, page.Source())
	if err != nil {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if strings.HasPrefix(part, ".") || part == "_plans" || part == "_drafts" {
			return false
		}
	}
	switch robots := fm.Get("robots", nil).(type) {
	case string:
		for _, directive := range strings.FieldsFunc(strings.ToLower(robots), func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' }) {
			if directive == "noindex" {
				return false
			}
		}
	case []interface{}:
		for _, directive := range robots {
			if text, ok := directive.(string); ok && strings.EqualFold(strings.TrimSpace(text), "noindex") {
				return false
			}
		}
	}
	return true
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
		local = strings.EqualFold(parsed.Scheme, origin.Scheme) && strings.EqualFold(parsed.Hostname(), origin.Hostname()) && llmsURLPort(parsed) == llmsURLPort(origin) && (base == "" || parsed.Path == base || strings.HasPrefix(parsed.Path, base+"/"))
		if !local {
			return "", false, nil
		}
	}
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
	if !strings.HasPrefix(route, "/") || route != clean || parsed.RawQuery != "" || parsed.Fragment != "" || strings.ContainsAny(route, "%?#\\") || parsed.RawPath != "" {
		return "", false, fmt.Errorf("llms_txt local destination must be a clean canonical route: %q", destination)
	}
	return route, true, nil
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
