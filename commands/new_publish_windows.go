package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func renameNewSite(staging, target string) error {
	_, exists, err := resolveNewSiteTarget(target)
	if err != nil {
		return err
	}
	if !exists {
		return os.Rename(staging, target)
	}

	// Windows cannot atomically rename over an existing directory. Keep the
	// original directory in an owned sibling until publication succeeds, rather
	// than deleting it and trying to recreate it after a failed rename.
	backup, err := os.MkdirTemp(filepath.Dir(target), "."+filepath.Base(target)+".jigyll-new-backup-*")
	if err != nil {
		return err
	}
	defer os.Remove(backup)
	original := filepath.Join(backup, "original")
	if err := os.Rename(target, original); err != nil {
		return err
	}
	restore := func(cause error) error {
		if err := os.Rename(original, target); err != nil {
			// Do not delete the preserved directory if another process prevents
			// rollback. Report its exact location instead of losing it.
			return errors.Join(cause, fmt.Errorf("restore empty target (preserved at %q): %w", original, err))
		}
		return cause
	}
	entries, err := os.ReadDir(original)
	if err != nil {
		return restore(err)
	}
	if len(entries) != 0 {
		return restore(fmt.Errorf("target directory %q is not empty", target))
	}
	if err := os.Rename(staging, target); err != nil {
		return restore(err)
	}
	return os.Remove(original)
}
