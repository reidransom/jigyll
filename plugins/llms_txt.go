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

func (p *llmsTxtPlugin) PrepareArtifacts(ctx ArtifactContext) (*ArtifactPlan, error) {
	cfg := p.site.Config()
	origin, err := url.Parse(cfg.AbsoluteURL)
	if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || (origin.Path != "" && origin.Path != "/") {
		return nil, fmt.Errorf("llms_txt requires an absolute HTTP(S) canonical site url without a path")
	}
	base := strings.TrimSuffix(cfg.BaseURL, "/")
	if base != "" && (!strings.HasPrefix(base, "/") || path.Clean(base) != base || strings.ContainsAny(base, "?#%\\")) {
		return nil, fmt.Errorf("llms_txt requires a clean baseurl path")
	}
	settings, err := p.settings(ctx.Documentation)
	if err != nil {
		return nil, err
	}
	canonical := func(route string) string { return origin.ResolveReference(&url.URL{Path: base + route}).String() }
	routes := make(map[string]Page, len(ctx.Pages))
	for _, page := range ctx.Pages {
		routes[page.URL()] = page
	}
	sections := settings.sections
	if !settings.curated {
		if ctx.Documentation != nil {
			for _, group := range ctx.Documentation.Groups {
				section := llmstxt.Section{Title: group.Title, Description: group.Description}
				for _, article := range ctx.Documentation.Articles {
					if page, exists := routes[article.URL]; article.NavSection == group.Slug && exists && p.eligible(page) {
						section.Entries = append(section.Entries, llmstxt.Entry{URL: article.URL, Title: article.Title, Description: article.Description})
					}
				}
				sections = append(sections, section)
			}
		} else {
			var sorted []string
			for route, page := range routes {
				if p.eligible(page) && cfg.IsMarkdown(page.Source()) {
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
	if ctx.Documentation != nil {
		for _, article := range ctx.Documentation.Articles {
			owned[article.URL] = article
		}
	}
	plan := &ArtifactPlan{Routes: []string{"/llms.txt"}}
	var companions []llmsCompanion
	seen := map[string]bool{}
	for i := range sections {
		for j := range sections[i].Entries {
			entry := &sections[i].Entries[j]
			route, local, err := localLLMsRoute(entry.URL, origin, base)
			if err != nil {
				return nil, err
			}
			identity := entry.URL
			if local {
				identity = route
			}
			if seen[identity] {
				return nil, fmt.Errorf("duplicate llms_txt entry %q", entry.URL)
			}
			seen[identity] = true
			if !local {
				if strings.TrimSpace(entry.Title) == "" {
					return nil, fmt.Errorf("external llms_txt entry %q requires title", entry.URL)
				}
				continue
			}
			page, found := routes[route]
			if !found || !p.eligible(page) {
				return nil, fmt.Errorf("llms_txt local route %q is missing or excluded", entry.URL)
			}
			overrides := settings.overrides[entry.URL]
			if entry.Title == "" && !overrides.title {
				entry.Title, err = optionalLLMsString(page.FrontMatter(), "title")
				if err != nil {
					return nil, err
				}
			}
			if entry.Description == "" && !overrides.description {
				entry.Description, err = optionalLLMsString(page.FrontMatter(), "description")
				if err != nil {
					return nil, err
				}
			}
			if strings.TrimSpace(entry.Title) == "" {
				return nil, fmt.Errorf("llms_txt route %s requires title", route)
			}
			entry.URL = canonical(route)
			if settings.markdown && cfg.IsMarkdown(page.Source()) {
				md := markdownCompanionRoute(route)
				if article, exists := owned[route]; exists {
					if ctx.Documentation.Sources[route] != page.Source() || article.CanonicalURL != canonical(route) || article.MarkdownURL != canonical(md) || article.URL+"index.md" != md {
						return nil, fmt.Errorf("llms_txt documentation ownership mismatch at %s", route)
					}
				} else {
					plan.Routes = append(plan.Routes, md)
					plan.Capture = append(plan.Capture, page)
					companions = append(companions, llmsCompanion{page: page, route: md, canonical: canonical(route)})
				}
				entry.URL = canonical(md)
			}
		}
	}
	if ctx.Documentation != nil {
		downloads := llmstxt.Section{Title: "Downloads", Entries: []llmstxt.Entry{
			{Title: "Article manifest", URL: canonical("/docs/manifest.json"), Description: "Canonical URLs, Markdown URLs, and section metadata."},
			{Title: "Section corpus", URL: canonical("/docs/search.json"), Description: "Downloadable source text for local search."},
		}}
		sections = append([]llmstxt.Section{downloads}, sections...)
		if api := ctx.Documentation.Project.OpenAPI; api != "" {
			sections = append(sections, llmstxt.Section{Title: "API specification", Entries: []llmstxt.Entry{{Title: "OpenAPI", URL: api, Description: "Canonical API specification."}}})
		}
	}
	plan.Build = func() (map[string][]byte, error) {
		artifacts := make(map[string][]byte, len(plan.Routes))
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
		var out strings.Builder
		llmstxt.WriteHeader(&out, settings.project)
		llmstxt.WriteSections(&out, sections)
		artifacts["/llms.txt"] = []byte(out.String())
		return artifacts, nil
	}
	return plan, nil
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

func (p *llmsTxtPlugin) eligible(page Page) bool {
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
	if page.URL() == "/404.html" || page.URL() == "/404/" || page.URL() == "/404" {
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
