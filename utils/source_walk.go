package utils

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SourceWalkOptions controls source traversal. Prune is called with the logical
// path and lstat metadata before a symbolic link is resolved.
type SourceWalkOptions struct {
	FollowDirectorySymlinks bool
	Prune                   func(string, os.FileInfo) bool
	Protected               []string
}

// CanonicalPath resolves existing ancestors even if the final path is absent.
func CanonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(abs)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		// A dangling symlink is not a missing path component.
		if _, statErr := os.Lstat(abs); statErr == nil {
			return "", err
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return "", statErr
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", err
		}
		suffix = append(suffix, filepath.Base(abs))
		abs = parent
	}
}

func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// WalkSource visits logical source paths, optionally following directory links.
// The callback may return filepath.SkipDir to prune a directory.
func WalkSource(root string, opts SourceWalkOptions, visit filepath.WalkFunc) error {
	if opts.Prune != nil {
		if info, err := os.Lstat(root); err == nil && opts.Prune(root, info) {
			return nil
		}
	}
	rootPhysical, err := CanonicalPath(root)
	if err != nil {
		if info, lstatErr := os.Lstat(root); lstatErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("resolving source symlink %q: %w", root, err)
		}
		return visit(root, nil, err)
	}
	protected, err := sourceWalkProtected(root, rootPhysical, opts.Protected)
	if err != nil {
		return err
	}
	walker := sourceWalker{opts: opts, visit: visit, protected: protected}
	return walker.walk(root, nil)
}

func sourceWalkProtected(root, rootPhysical string, paths []string) ([]string, error) {
	protected := make([]string, 0, len(paths))
	for _, p := range paths {
		canonical, err := CanonicalPath(p)
		if err != nil {
			return nil, fmt.Errorf("resolving protected path %q: %w", p, err)
		}
		if pathWithin(rootPhysical, canonical) {
			return nil, fmt.Errorf("source %q is inside protected output %q (same directory or ancestor)", root, p)
		}
		protected = append(protected, canonical)
	}
	return protected, nil
}

type sourceWalker struct {
	opts      SourceWalkOptions
	visit     filepath.WalkFunc
	protected []string
}

func (w *sourceWalker) walk(path string, ancestors []os.FileInfo) error {
	info, err := os.Lstat(path)
	if err != nil {
		return w.visit(path, nil, err)
	}
	if w.opts.Prune != nil && w.opts.Prune(path, info) {
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		info, err = w.resolveSymlink(path)
		if err != nil || info == nil {
			return err
		}
	}
	skip, err := w.validateDirectory(path, info, ancestors)
	if err != nil || skip {
		return err
	}
	if err := w.visit(path, info, nil); err != nil {
		if err == filepath.SkipDir && info.IsDir() {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return w.walkChildren(path, info, ancestors)
}

func (w *sourceWalker) protectedPath(path string) bool {
	for _, p := range w.protected {
		if pathWithin(path, p) {
			return true
		}
	}
	return false
}

func (w *sourceWalker) resolveSymlink(path string) (os.FileInfo, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return nil, fmt.Errorf("reading source symlink %q: %w", path, err)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	if canonical, resolveErr := CanonicalPath(target); resolveErr == nil {
		if w.protectedPath(canonical) {
			return nil, nil
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("resolving source symlink %q: %w", path, err)
	}
	if info.IsDir() {
		canonical, err := CanonicalPath(path)
		if err != nil {
			return nil, fmt.Errorf("resolving source symlink %q: %w", path, err)
		}
		if w.protectedPath(canonical) {
			return nil, nil
		}
		if !w.opts.FollowDirectorySymlinks {
			return nil, fmt.Errorf("directory symlink %q requires --follow-directory-symlinks", path)
		}
	}
	return info, nil
}

func (w *sourceWalker) validateDirectory(path string, info os.FileInfo, ancestors []os.FileInfo) (bool, error) {
	if !info.IsDir() {
		return false, nil
	}
	canonical, err := CanonicalPath(path)
	if err != nil {
		return false, fmt.Errorf("resolving source directory %q: %w", path, err)
	}
	if w.protectedPath(canonical) {
		return true, nil
	}
	for _, ancestor := range ancestors {
		if os.SameFile(info, ancestor) {
			return false, fmt.Errorf("directory symlink cycle at %q", path)
		}
	}
	return false, nil
}

func (w *sourceWalker) walkChildren(path string, info os.FileInfo, ancestors []os.FileInfo) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return w.visit(path, info, err)
	}
	ancestors = append(ancestors, info)
	for _, entry := range entries {
		if err := w.walk(filepath.Join(path, entry.Name()), ancestors); err != nil {
			return err
		}
	}
	return nil
}
