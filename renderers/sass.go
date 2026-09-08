package renderers

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/reidransom/jigyll/cache"
	"github.com/reidransom/jigyll/utils"

	sass "github.com/bep/godartsass/v2"
)

// Bump this namespace whenever compiled CSS output changes so persistent cache
// entries from older Jigyll versions cannot survive the change.
const sassCacheNamespace = "sass:v3"
const sassDirName = "_sass"

// copySASSFileIncludes copies sass partials into a temporary directory,
// removing initial underscores.
func (p *Manager) copySASSFileIncludes() error {
	// TODO delete the temp directory when done?
	// TODO use libsass.ImportsOption instead?
	// Clean up any existing temp directory to remove stale files
	if p.sassTempDir != "" {
		if err := os.RemoveAll(p.sassTempDir); err != nil {
			return err
		}
		p.sassTempDir = ""
	}
	if err := p.makeSASSTempDir(); err != nil {
		return err
	}
	h := md5.New()
	if p.ThemeDir != "" {
		if err := p.copySASSFiles(filepath.Join(p.ThemeDir, sassDirName), p.sassTempDir, h); err != nil {
			return err
		}
	}
	if err := p.copySASSFiles(filepath.Join(p.sourceDir(), p.cfg.Sass.Dir), p.sassTempDir, h); err != nil {
		return err
	}
	p.sassHash = fmt.Sprintf("%x", h.Sum(nil))
	return nil
}

func (p *Manager) makeSASSTempDir() error {
	if p.sassTempDir == "" {
		dir, err := os.MkdirTemp(os.TempDir(), "_sass")
		if p.cfg.Verbose {
			fmt.Println("create", dir)
		}
		if err != nil {
			return err
		}
		p.sassTempDir = dir
	}
	return nil
}

func (p *Manager) copySASSFiles(src, dst string, h io.Writer) error {
	if p.cfg.Verbose {
		fmt.Printf("copy sass directory %s to %s\n", src, dst)
	}
	err := filepath.Walk(src, func(from string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel := utils.MustRel(src, from)
		to := filepath.Join(dst, strings.TrimPrefix(rel, "_"))
		if p.cfg.Verbose {
			fmt.Printf("copy sass file %s to %s\n", src, to)
		}
		in, err := os.Open(from)
		if err != nil {
			return err
		}
		defer in.Close() // nolint: errcheck
		if _, err = fmt.Fprintf(h, "--- sass file: %s ---\n", rel); err != nil {
			return err
		}
		if _, err = io.Copy(h, in); err != nil {
			return err
		}
		return utils.CopyFileContents(to, from, 0644)
	})
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// SassIncludePaths returns an array of sass include directories.
func (p *Manager) SassIncludePaths() []string {
	if p.sassTempDir == "" {
		return nil
	}
	return []string{p.sassTempDir}
}

// WriteSass converts a SASS file and writes it to w.
func (p *Manager) WriteSass(w io.Writer, b []byte) error {
	s, err := cache.WithFile(fmt.Sprintf("%s: %s", sassCacheNamespace, p.sassHash), string(b), func() (s string, err error) {
		comp, err := p.getSassTranspiler()
		if err != nil {
			return "", err
		}
		res, err := comp.Execute(sass.Args{
			Source:       string(b),
			IncludePaths: p.SassIncludePaths(),
			OutputStyle:  sass.OutputStyleCompressed,
		})
		if err != nil {
			return "", err
		}
		return res.CSS, nil
	})
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, s)
	return err
}
