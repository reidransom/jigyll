//go:build !windows

package commands

import (
	"os"

	"golang.org/x/sys/unix"
)

func renameNewSite(staging, target string) error {
	// os.Rename deliberately refuses existing directories even on Unix.
	// The native rename atomically replaces only an empty directory and leaves
	// it untouched on failure; a non-empty installation cannot be replaced.
	if err := unix.Rename(staging, target); err != nil {
		return &os.LinkError{Op: "rename", Old: staging, New: target, Err: err}
	}
	return nil
}
