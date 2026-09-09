package pages

import (
	"path"
	"path/filepath"

	"github.com/reidransom/jigyll/renderers"
	"github.com/reidransom/jigyll/utils"
	"github.com/reidransom/liquid"
)

// ToLiquid is part of the liquid.Drop interface.
func (d *StaticFile) ToLiquid() interface{} {
	// Merge the default front matter under the fixed metadata keys so that
	// defaults like `sitemap` or custom tags surface in `site.static_files`,
	// while the real attributes (path, name, ...) shadow any same-named default.
	return liquid.IterationKeyedMap(d.fm.Merged(FrontMatter{
		"name":          path.Base(d.relPath),
		"basename":      utils.TrimExt(path.Base(d.relPath)),
		"path":          d.URL(),
		"modified_time": d.modTime,
		"extname":       d.OutputExt(),
		"collection":    nil,
	}))
}

func (f *file) ToLiquid() interface{} {
	var (
		relpath = "/" + filepath.ToSlash(f.relPath)
		base    = path.Base(relpath)
		ext     = path.Ext(relpath)
	)
	return liquid.IterationKeyedMap(f.fm.Merged(FrontMatter{
		"path":          relpath,
		"modified_time": f.modTime,
		"name":          base,
		"basename":      utils.TrimExt(base),
		"extname":       ext,
	}))
}

// ToLiquid is in the liquid.Drop interface.
func (p *page) ToLiquid() interface{} {
	var (
		fm          = p.fm
		relpath     = p.relPath
		siteRelPath = filepath.ToSlash(p.site.RelativePath(p.filename))
		ext         = filepath.Ext(relpath)
	)
	data := map[string]interface{}{
		"categories":    p.Categories(),
		"content":       p.maybeContent(),
		"excerpt":       p.Excerpt(),
		"headings":      p.maybeHeadings(),
		"id":            utils.TrimExt(p.URL()),
		"name":          filepath.Base(siteRelPath),
		"path":          siteRelPath,
		"relative_path": siteRelPath,
		"slug":          fm.String("slug", utils.Slugify(utils.TrimExt(filepath.Base(p.relPath)))),
		"tags":          p.Tags(),
		"url":           p.URL(),

		// de facto
		"ext": ext,
	}
	if localization, ok := pageLocalization(p.site, p); ok {
		data["language"] = localeDrop(localization.Language)
		data["translation_key"] = localization.TranslationKey
		data["translations"] = localization.Translations
		data["all_translations"] = localization.AllTranslations
		alternates := make([]map[string]interface{}, len(localization.Alternates))
		for index, alternate := range localization.Alternates {
			alternates[index] = alternateDrop(alternate)
		}
		data["alternates"] = alternates
	}
	if p.IsPost() {
		if _, hasDate := fm["date"]; !hasDate {
			data["date"] = p.PostDate()
		}
	}
	for k, v := range p.fm {
		switch k {
		// doc implies these aren't present, but they appear to be present in a collection page:
		// case "layout", "published":
		case "permalink", "headings":
			// Engine-owned values cannot be overridden by front matter.
		default:
			data[k] = v
		}
	}
	return liquid.IterationKeyedMap(data)
}

func (p *page) maybeContent() interface{} {
	p.m.RLock()
	defer p.m.RUnlock()
	if p.rendered {
		return p.content
	}
	return p.raw
}

func (p *page) maybeHeadings() []renderers.Heading {
	p.m.RLock()
	defer p.m.RUnlock()
	if !p.rendered || p.headings == nil {
		return []renderers.Heading{}
	}
	return p.headings
}
