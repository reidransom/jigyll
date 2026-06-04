package renderers

import (
	"io"
	"path/filepath"
	"sync"

	sass "github.com/bep/godartsass/v2"
	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/jigyll/filters"
	"github.com/reidransom/jigyll/internal/sasserrors"
	"github.com/reidransom/jigyll/tags"
	"github.com/reidransom/jigyll/utils"
	"github.com/reidransom/liquid"
)

// Global Sass transpiler singleton, shared across all Manager instances.
// This avoids race conditions and resource leaks when Sites are reloaded during watch mode.
// The transpiler is thread-safe and stateless (include paths are passed to Execute()),
// so a single instance can safely serve all Managers throughout the process lifetime.
// dart-sass is resolved from PATH (sass.Options{}); install it via the curl
// installer or Homebrew, or have `sass` on PATH for `go install` builds.
var (
	globalSassTranspiler     *sass.Transpiler
	globalSassTranspilerOnce sync.Once
	globalSassTranspilerErr  error
)

// Renderers applies transformations to a document.
type Renderers interface {
	ApplyLayout(string, []byte, liquid.Bindings) ([]byte, error)
	Render(io.Writer, []byte, liquid.Bindings, string, int) error
	RenderTemplate([]byte, liquid.Bindings, string, int) ([]byte, error)
}

// Manager applies a rendering transformation to a file.
type Manager struct {
	Options
	cfg          config.Config
	liquidEngine *liquid.Engine
	sassTempDir  string
	sassHash     string
}

// Options configures a rendering manager.
type Options struct {
	RelativeFilenameToURL tags.LinkTagHandler
	ThemeDir              string
}

// New makes a rendering manager.
func New(c config.Config, options Options) (*Manager, error) {
	p := Manager{Options: options, cfg: c}
	p.liquidEngine = p.makeLiquidEngine()
	if err := p.copySASSFileIncludes(); err != nil {
		return nil, err
	}
	return &p, nil
}

// sourceDir returns the site source directory. Seeing how far we can bend
// the Law of Demeter.
func (p *Manager) sourceDir() string {
	return p.cfg.Source
}

// TemplateEngine returns the Liquid engine.
func (p *Manager) TemplateEngine() *liquid.Engine {
	return p.liquidEngine
}

// Render sends content through SASS and/or Liquid -> Markdown
func (p *Manager) Render(w io.Writer, src []byte, vars liquid.Bindings, filename string, lineNo int) error {
	if p.cfg.IsSASSPath(filename) {
		return p.WriteSass(w, src)
	}
	src, err := p.RenderTemplate(src, vars, filename, lineNo)
	if err != nil {
		return err
	}
	if p.cfg.IsMarkdown(filename) {
		src, err = renderMarkdown(src)
		if err != nil {
			return err
		}
	}
	_, err = w.Write(src)
	return err
}

// RenderTemplate renders a Liquid template
func (p *Manager) RenderTemplate(src []byte, vars liquid.Bindings, filename string, lineNo int) ([]byte, error) {
	tpl, err := p.liquidEngine.ParseTemplateLocation(src, filename, lineNo)
	if err != nil {
		return nil, utils.WrapPathError(err, filename)
	}
	out, err := tpl.Render(vars)
	if err != nil {
		return nil, utils.WrapPathError(err, filename)
	}
	return out, err
}

func (p *Manager) makeLiquidEngine() *liquid.Engine {
	dirs := []string{filepath.Join(p.cfg.Source, p.cfg.IncludesDir)}
	if p.ThemeDir != "" {
		dirs = append(dirs, filepath.Join(p.ThemeDir, "_includes"))
	}
	engine := liquid.NewEngine()
	filters.AddJekyllFilters(engine, &p.cfg)
	tags.AddJekyllTags(engine, &p.cfg, dirs, p.RelativeFilenameToURL)
	return engine
}

// getSassTranspiler returns the global SASS transpiler singleton, initializing it if necessary.
// Using a global singleton avoids race conditions when Sites are reloaded during watch mode,
// and matches the godartsass recommendation to "create one and use that for all SCSS processing."
func (p *Manager) getSassTranspiler() (*sass.Transpiler, error) {
	globalSassTranspilerOnce.Do(func() {
		globalSassTranspiler, globalSassTranspilerErr = sass.Start(sass.Options{})
		globalSassTranspilerErr = sasserrors.Enhance(globalSassTranspilerErr)
	})
	return globalSassTranspiler, globalSassTranspilerErr
}
