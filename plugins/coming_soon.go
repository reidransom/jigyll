package plugins

func init() {
	register("jekyll-coming-soon", &comingSoonPlugin{})
}

type comingSoonPlugin struct {
	plugin
	site Site
}

func (p *comingSoonPlugin) AfterInitSite(s Site) error {
	p.site = s
	return nil
}

func (p *comingSoonPlugin) isEnabled() bool {
	if p.site == nil {
		return false
	}
	enabled, _ := p.site.Config().Variables()["coming_soon"].(bool)
	return enabled
}

func (p *comingSoonPlugin) ModifyPluginList(names []string) []string {
	if !p.isEnabled() {
		return names
	}
	suppress := map[string]bool{
		"jekyll-feed":    true,
		"jekyll-sitemap": true,
	}
	var filtered []string
	for _, name := range names {
		if !suppress[name] {
			filtered = append(filtered, name)
		}
	}
	return filtered
}

func (p *comingSoonPlugin) PostReadSite(s Site) error {
	if !p.isEnabled() {
		return nil
	}
	cfg := s.Config()
	title, _ := cfg.String("title")

	for _, pg := range s.Pages() {
		if pg.URL() == "/" {
			// Replace homepage with coming soon content
			if s.HasLayout("coming_soon") {
				pg.FrontMatter()["layout"] = "coming_soon"
			} else {
				pg.SetContent(buildComingSoonHTML(title))
				pg.FrontMatter()["layout"] = ""
			}
		} else {
			// Unpublish all other pages
			pg.FrontMatter()["published"] = false
			s.RemoveRoute(pg.URL())
		}
	}
	return nil
}

func buildComingSoonHTML(title string) string {
	if title == "" {
		title = "Coming Soon"
	}
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="robots" content="noindex">
  <title>` + title + ` — Coming Soon</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      display: flex; align-items: center; justify-content: center;
      min-height: 100vh; background: #fafafa; color: #333;
      text-align: center; padding: 2rem;
    }
    h1 { font-size: 2.5rem; margin-bottom: 0.5rem; }
    p { font-size: 1.125rem; color: #666; margin-top: 0.75rem; }
    a { color: #0066cc; }
  </style>
</head>
<body>
  <div>
    <h1>` + title + `</h1>
    <p>Coming Soon</p>
  </div>
</body>
</html>`
}
