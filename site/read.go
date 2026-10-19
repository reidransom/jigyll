package site

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/reidransom/jigyll/collection"
	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/jigyll/pages"
	"github.com/reidransom/jigyll/plugins"
	"github.com/reidransom/jigyll/utils"
)

// FromDirectory reads the configuration file, if it exists.
func FromDirectory(dir string, flags config.Flags) (*Site, error) {
	s := New(flags)
	if err := s.cfg.FromDirectory(dir, flags.ConfigFiles); err != nil {
		return nil, utils.WrapError(err, "reading site")
	}
	s.cfg.ApplyFlags(s.flags)
	return s, nil
}

// Read loads the site data and files.
func (s *Site) Read() error {
	if err := s.installPlugins(); err != nil {
		return utils.WrapError(err, "initializing plugins")
	}
	s.Routes = make(map[string]Document)
	if err := s.findTheme(); err != nil {
		return utils.WrapError(err, "finding theme")
	}
	if s.data == nil {
		if err := s.readDataFiles(); err != nil {
			return utils.WrapError(err, "reading data files")
		}
	}
	if err := s.readThemeAssets(); err != nil {
		return utils.WrapError(err, "reading theme assets")
	}
	// Exclude the destination directory from source reading, matching Ruby Jekyll behavior.
	// Without this, rebuilds can read prior output files as source documents.
	if destRel, err := filepath.Rel(s.SourceDir(), s.DestDir()); err == nil && destRel != "." && !strings.HasPrefix(destRel, "..") {
		s.cfg.Exclude = append(s.cfg.Exclude, destRel)
	}
	if err := s.readFiles(s.SourceDir(), s.SourceDir()); err != nil {
		return utils.WrapError(err, "reading files")
	}
	if err := s.ReadCollections(); err != nil {
		return utils.WrapError(err, "reading collections")
	}
	if err := s.initializeRenderers(); err != nil {
		return utils.WrapError(err, "initializing renderers")
	}
	for _, p := range s.Pages() {
		err := s.runHooks(func(h plugins.Plugin) error {
			return h.PostInitPage(s, p)
		})
		if err != nil {
			return err
		}
	}
	if err := s.runHooks(func(p plugins.Plugin) error { return p.PostReadSite(s) }); err != nil {
		return err
	}
	return s.prepareDocumentation()
}

// isIncludedPath checks if a path or its parent directory is explicitly in the include list
func (s *Site) isIncludedPath(siteRel string) bool {
	for siteRel != "." && siteRel != "" {
		if utils.MatchList(s.cfg.Include, siteRel) {
			return true
		}
		siteRel = filepath.Dir(siteRel)
	}
	return false
}

// isConfiguredCollectionDirectory reports whether siteRel is the root of a
// configured collection. Localized ordinary-page discovery must leave these
// directories to ReadCollections even when an include pattern admits them.
func (s *Site) isConfiguredCollectionDirectory(siteRel string) bool {
	if filepath.Dir(siteRel) != "." || !strings.HasPrefix(siteRel, "_") {
		return false
	}
	_, configured := s.cfg.Collections[strings.TrimPrefix(siteRel, "_")]
	return configured
}

// readFiles scans the source directory and creates pages and collection.
//
//nolint:gocyclo
func (s *Site) readFiles(dir, base string) error {
	return filepath.Walk(dir, func(filename string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel := utils.MustRel(base, filename)
		switch {
		case info.IsDir() && s.Exclude(rel):
			return filepath.SkipDir
		case info.IsDir() && s.localizedSources != nil && s.isConfiguredCollectionDirectory(rel):
			return filepath.SkipDir
		case info.IsDir() && strings.HasPrefix(rel, "_") && !s.isIncludedPath(rel):
			// Top-level underscore directories (_layouts, _includes, _sass,
			// _data, collections) are handled by other code paths. Skip them
			// here unless they are explicitly listed in `include:`.
			return filepath.SkipDir
		case info.IsDir():
			return nil
		case s.Exclude(rel):
			s.diag.FilesFound++
			s.diag.FilesExcluded++
			if s.cfg.Verbose {
				fmt.Printf("skip (excluded): %s\n", rel)
			}
			return nil
		case strings.HasPrefix(rel, "_") && !s.isIncludedPath(rel):
			s.diag.FilesFound++
			s.diag.FilesUnderscored++
			if s.cfg.Verbose {
				fmt.Printf("skip (underscore): %s\n", rel)
			}
			return nil
		}
		s.diag.FilesFound++
		// Non-collection documents use Jekyll's defaults vocabulary: pages get
		// type "pages", static files (no front matter) are untyped and so only
		// match typeless scopes — mirroring Jekyll's nil-typed static files.
		defaultFrontmatter := func(isStatic bool) pages.FrontMatter {
			typename := "pages"
			if isStatic {
				typename = ""
			}
			return s.cfg.GetFrontMatterDefaults(typename, rel)
		}
		d, err := pages.NewFile(s, filename, filepath.ToSlash(rel), defaultFrontmatter)
		if err != nil {
			return utils.WrapPathError(err, filename)
		}
		if d.IsStatic() && s.localeKey != "" && !s.includeStatic {
			return nil
		}
		if p, ok := d.(Page); ok && !s.IncludesPage(p) {
			return nil
		}
		if d.IsStatic() {
			s.diag.FilesStaticNoFM++
			if s.cfg.Verbose {
				fmt.Printf("skip (no frontmatter → static file): %s\n", rel)
			}
		}
		accepted := s.registerDocument(d, true)
		if p, ok := d.(Page); ok && accepted {
			// Only add pages that don't belong to any collection
			// Collection pages are in directories starting with '_' (like _posts, _coll1, etc.)
			// However, explicitly included directories (via include config) should be added
			dir := filepath.Dir(rel)
			if dir == "." || !strings.HasPrefix(filepath.Base(dir), "_") || s.isIncludedPath(rel) {
				s.nonCollectionPages = append(s.nonCollectionPages, p)
			}
		}
		return nil
	})
}

// IncludesPage reports whether a page belongs to this prepared locale site.
// It is deliberately available to collection construction so sorting and post
// navigation are computed after locale filtering.
func (s *Site) IncludesPage(page Page) bool {
	if s.localeKey == "" {
		return true
	}
	if s.localizedSources != nil {
		_, included := s.localizedSources[page.Source()]
		return included
	}
	return s.includesLocale(page.FrontMatter())
}

// includesLocale reports whether effective front matter belongs in this site.
// An empty language remains ordinary-site behavior unless the site is prepared
// for localization, where it belongs to the configured default locale.
func (s *Site) includesLocale(fm pages.FrontMatter) bool {
	if s.localeKey == "" {
		return true
	}
	lang, found := fm["lang"]
	if !found {
		return s.localeKey == s.cfg.Localization.DefaultLanguage
	}
	key, ok := lang.(string)
	return ok && key == s.localeKey
}

// registerDocument adds a document to the site's fields and reports whether
// publication policy accepted it.
func (s *Site) registerDocument(d Document, output bool) bool {
	if d.Published() || s.cfg.Unpublished {
		s.docs = append(s.docs, d)
		if output {
			s.outputCandidates = append(s.outputCandidates, d)
			if s.removedRoutes != nil {
				delete(s.removedRoutes, d.URL())
			}
			s.Routes[d.URL()] = d
		}
		return true
	}

	s.diag.FilesUnpublished++
	if s.cfg.Verbose {
		fmt.Printf("skip (unpublished): %s\n", d.Source())
	}
	return false
}

// AddDocument adds a document to the site's fields.
// It ignores unpublished documents unless config.Unpublished is true.
func (s *Site) AddDocument(d Document, output bool) {
	s.registerDocument(d, output)
}

// ReadCollections reads the pages of the collections named in the site configuration.
// It adds each collection's pages to the site map, and creates a template site variable for each collection.
func (s *Site) ReadCollections() (err error) {
	var cols []*collection.Collection
	for name, data := range s.cfg.Collections {
		c := collection.New(s, name, data)
		cols = append(cols, c)
		err = c.ReadPages()
		if err != nil {
			break
		}
		for _, p := range c.Pages() {
			s.AddDocument(p, c.Output())
		}
	}
	sort.Slice(cols, func(i, j int) bool {
		return cols[i].Name < cols[j].Name
	})
	s.Collections = cols
	return nil
}
