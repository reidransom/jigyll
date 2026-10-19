package site

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/reidransom/jigyll/collection"
	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/jigyll/pages"
	"github.com/reidransom/jigyll/plugins"
	"github.com/reidransom/jigyll/renderers"
	"github.com/reidransom/jigyll/utils"
	"github.com/reidransom/liquid"
)

// Site is a Jekyll site.
type Site struct {
	Collections []*collection.Collection
	Routes      map[string]Document // URL path -> Document; only for output pages

	cfg             config.Config
	data            map[string]interface{} // from _data files
	flags           config.Flags           // command-line flags, override config files
	plugins         []string               // initially cfg.Plugins, but plugins can modify this
	pluginInstances map[string]plugins.Plugin
	themeDir        string // absolute path to theme directory
	remoteThemes    *remoteThemeResolver

	docs               []Document // all documents, whether or not they are output
	nonCollectionPages []Page
	documentation      *documentationInventory

	renderer   *renderers.Manager
	renderOnce sync.Once

	drop     map[string]interface{} // cached drop value
	dropOnce sync.Once

	// Localization state is set only on a prepared locale site. It stays
	// private to the site lifecycle so ordinary sites retain their existing
	// configuration and document behavior.
	localeKey           string
	localePrefix        string
	includeStatic       bool
	localizedSources    map[string]struct{}
	localizationContext *localizedSiteContext
	removedRoutes       map[string]struct{}
	outputCandidates    []Document

	// Build diagnostics
	diag BuildDiagnostics
}

// BuildDiagnostics tracks statistics about files processed during a build.
type BuildDiagnostics struct {
	FilesFound       int // total files walked
	FilesExcluded    int // excluded by config
	FilesUnderscored int // skipped due to leading underscore
	FilesStaticNoFM  int // became static files (no frontmatter, requires frontmatter)
	FilesUnpublished int // skipped due to published: false
	FilesOutput      int // files actually written
}

// DiagSummary returns a human-readable summary of the build diagnostics.
func (d BuildDiagnostics) DiagSummary() string {
	return fmt.Sprintf(
		"%d files found, %d output (%d excluded, %d no frontmatter, %d unpublished, %d underscore-prefixed)",
		d.FilesFound, d.FilesOutput,
		d.FilesExcluded, d.FilesStaticNoFM, d.FilesUnpublished, d.FilesUnderscored,
	)
}

// Diagnostics returns the build diagnostics for this site.
func (s *Site) Diagnostics() BuildDiagnostics {
	return s.diag
}

// Document is in package pages.
type Document = pages.Document

// Page is in package pages.
type Page = pages.Page

// SourceDir returns the site source directory.
func (s *Site) SourceDir() string { return s.cfg.Source }

// DestDir returns the site destination directory.
func (s *Site) DestDir() string {
	if filepath.IsAbs(s.cfg.Destination) {
		return s.cfg.Destination
	}
	return filepath.Join(s.cfg.Source, s.cfg.Destination)
}

// OutputDocs returns output pages in route order.
func (s *Site) OutputDocs() []Document {
	routes := make([]string, 0, len(s.Routes))
	for route := range s.Routes {
		routes = append(routes, route)
	}
	sort.Strings(routes)
	out := make([]Document, 0, len(routes))
	for _, route := range routes {
		out = append(out, s.Routes[route])
	}
	return out
}

// Pages returns all the pages, output or not.
func (s *Site) Pages() (out []Page) {
	for _, d := range s.docs {
		if p, ok := d.(Page); ok {
			out = append(out, p)
		}
	}
	return
}

// Posts is part of the plugins.Site interface.
func (s *Site) Posts() []Page {
	for _, c := range s.Collections {
		if c.Name == "posts" {
			return c.Pages()
		}
	}
	return nil
}

// AbsDir is in the collection.Site interface.
func (s *Site) AbsDir() string {
	d, err := filepath.Abs(s.SourceDir())
	if err != nil {
		panic(err)
	}
	return d
}

// Config is in the collection.Site interface.
func (s *Site) Config() *config.Config {
	return &s.cfg
}

// Site is in the pages.RenderingContext interface.
func (s *Site) Site() interface{} {
	return s
}

// PathPrefix is in the page.Container interface.
func (s *Site) PathPrefix() string { return s.localePrefix }

// New creates a new site record, initialized with the site defaults.
func New(flags config.Flags) *Site {
	s := &Site{cfg: config.Default(), flags: flags, remoteThemes: newRemoteThemeResolver()}
	s.cfg.ApplyFlags(flags)
	return s
}

// SetAbsoluteURL overrides the loaded configuration.
// The server uses this.
func (s *Site) SetAbsoluteURL(url string) {
	s.cfg.AbsoluteURL = url
	s.cfg.Set("url", url)
	if s.drop != nil {
		s.drop["url"] = url
	}
}

// FilenameURLs returns a map of site-relative pathnames to URL paths
func (s *Site) FilenameURLs() map[string]string {
	urls := map[string]string{}
	for _, page := range s.Pages() {
		urls[utils.MustRel(s.SourceDir(), page.Source())] = page.URL()
	}
	return urls
}

// KeepFile returns a boolean indicating that clean should leave the file in the destination directory.
func (s *Site) KeepFile(filename string) bool {
	return utils.SearchStrings(s.cfg.KeepFiles, filename)
}

// FilePathPage returns a Page, give a file path relative to site source directory.
func (s *Site) FilePathPage(rel string) (Document, bool) {
	// This looks wasteful. If it shows up as a hotspot, you know what to do.
	for _, p := range s.Routes {
		if p.Source() != "" {
			if r, err := filepath.Rel(s.SourceDir(), p.Source()); err == nil {
				if r == rel {
					return p, true
				}
			}
		}
	}
	return nil, false
}

// FilenameURLPath returns a page's URL path, give a relative file path relative to the site source directory.
func (s *Site) FilenameURLPath(relpath string) (string, bool) {
	if p, found := s.FilePathPage(relpath); found {
		return p.URL(), true
	}
	return "", false
}

// RendererManager returns the rendering manager.
func (s *Site) RendererManager() renderers.Renderers {
	if s.renderer == nil {
		panic("uninitialized rendering manager")
	}
	return s.renderer
}

// TemplateEngine is part of the plugins.Site interface.
func (s *Site) TemplateEngine() *liquid.Engine {
	return s.renderer.TemplateEngine()
}

// initializeRenderers initializes the rendering manager
func (s *Site) initializeRenderers() (err error) {
	options := renderers.Options{
		RelativeFilenameToURL: s.FilenameURLPath,
		ThemeDir:              s.themeDir,
		Localization:          s.localizationContext,
	}
	s.renderer, err = renderers.New(s.cfg, options)
	if err != nil {
		return err
	}
	engine := s.renderer.TemplateEngine()
	return s.runHooks(func(p plugins.Plugin) error {
		return p.ConfigureTemplateEngine(engine)
	})
}

// HasLayout is in the plugins.Site interface.
func (s *Site) HasLayout(ln string) bool {
	l, err := s.renderer.FindLayout(ln, nil)
	return err == nil && l != nil
}

// RelativePath is in the page.Container interface.
func (s *Site) RelativePath(path string) string {
	if s.themeDir != "" {
		if rel, err := filepath.Rel(s.themeDir, path); err == nil {
			return rel
		}
	}
	return utils.MustRel(s.cfg.Source, path)
}

// URLPage returns the page that will be served at URL
func (s *Site) URLPage(urlpath string) (p Document, found bool) {
	p, found = s.Routes[urlpath]
	if !found {
		p, found = s.Routes[filepath.Join(urlpath, "index.html")]
	}
	if !found {
		p, found = s.Routes[filepath.Join(urlpath, "index.htm")]
	}
	if !found {
		// Serve extensionless URL `/some-url` from file `/some-url.html`
		p, found = s.Routes[urlpath+".html"]
	}
	if !found && strings.HasSuffix(urlpath, "/index.html") {
		// HTML index pages are routed under their collapsed directory URL
		// (`/`, `/sub/`); resolve a literal `…/index.html` request to it.
		p, found = s.Routes[strings.TrimSuffix(urlpath, "index.html")]
	}
	return
}
