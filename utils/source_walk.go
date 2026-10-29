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
	protected := make([]string, 0, len(opts.Protected))
	for _, p := range opts.Protected {
		canonical, err := CanonicalPath(p)
		if err != nil {
			return fmt.Errorf("resolving protected path %q: %w", p, err)
		}
		if pathWithin(rootPhysical, canonical) {
			return fmt.Errorf("source %q is inside protected output %q (same directory or ancestor)", root, p)
		}
		protected = append(protected, canonical)
	}
	var walk func(string, []os.FileInfo) error
	walk = func(path string, ancestors []os.FileInfo) error {
		info, err := os.Lstat(path)
		if err != nil {
			return visit(path, nil, err)
		}
		if opts.Prune != nil && opts.Prune(path, info) {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, linkErr := os.Readlink(path)
			if linkErr != nil {
				return fmt.Errorf("reading source symlink %q: %w", path, linkErr)
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(path), target)
			}
			if canonical, resolveErr := CanonicalPath(target); resolveErr == nil {
				for _, p := range protected {
					if pathWithin(canonical, p) {
						return nil
					}
				}
			}
			info, err = os.Stat(path)
			if err != nil {
				return fmt.Errorf("resolving source symlink %q: %w", path, err)
			}
			if info.IsDir() {
				canonical, err := CanonicalPath(path)
				if err != nil {
					return fmt.Errorf("resolving source symlink %q: %w", path, err)
				}
				for _, p := range protected {
					if pathWithin(canonical, p) {
						return nil
					}
				}
				if !opts.FollowDirectorySymlinks {
					return fmt.Errorf("directory symlink %q requires --follow-directory-symlinks", path)
				}
			}
		}
		if info.IsDir() {
			canonical, err := CanonicalPath(path)
			if err != nil {
				return fmt.Errorf("resolving source directory %q: %w", path, err)
			}
			for _, p := range protected {
				if pathWithin(canonical, p) {
					return nil
				}
			}
			for _, ancestor := range ancestors {
				if os.SameFile(info, ancestor) {
					return fmt.Errorf("directory symlink cycle at %q", path)
				}
			}
		}
		if err := visit(path, info, nil); err != nil {
			if err == filepath.SkipDir && info.IsDir() {
				return nil
			}
			return err
		}
		if !info.IsDir() {
			return nil
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return visit(path, info, err)
		}
		ancestors = append(ancestors, info)
		for _, entry := range entries {
			if err := walk(filepath.Join(path, entry.Name()), ancestors); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(root, nil)
}
