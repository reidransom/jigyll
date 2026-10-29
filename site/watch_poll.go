package site

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/reidransom/jigyll/utils"
)

type pollEntry struct {
	mode     os.FileMode
	size     int64
	modified time.Time
	identity string
	link     string
}
type pollSnapshot map[string]pollEntry

func (s *Site) pollSnapshot() (pollSnapshot, error) {
	snapshot := make(pollSnapshot)
	options := utils.SourceWalkOptions{FollowDirectorySymlinks: s.cfg.FollowDirectorySymlinks, Protected: append([]string{s.DestDir()}, s.cfg.ProtectedSourcePaths...), Prune: func(path string, info os.FileInfo) bool {
		rel := utils.MustRel(s.SourceDir(), path)
		return rel != "." && s.Exclude(rel)
	}}
	err := utils.WalkSource(s.SourceDir(), options, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel := utils.MustRel(s.SourceDir(), path)
		physical, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		physical, err = filepath.Abs(physical)
		if err != nil {
			return err
		}
		entry := pollEntry{mode: info.Mode(), size: info.Size(), modified: info.ModTime(), identity: physical}
		if info.IsDir() {
			entry.size = 0
			entry.modified = time.Time{}
		}
		lstat, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if lstat.Mode()&os.ModeSymlink != 0 {
			entry.link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		snapshot[rel] = entry
		return nil
	})
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func diffPollSnapshots(old, next pollSnapshot) FilesEvent {
	event := FilesEvent{}
	for path, before := range old {
		after, found := next[path]
		if !found {
			event.Paths = append(event.Paths, path)
			event.Topology = true
			continue
		}
		if before != after {
			if before.mode.IsDir() || after.mode.IsDir() || before.identity != after.identity || before.link != after.link {
				event.Topology = true
			}
			if path != "." {
				event.Paths = append(event.Paths, path)
			}
		}
	}
	for path := range next {
		if _, found := old[path]; !found {
			event.Paths = append(event.Paths, path)
			event.Topology = true
		}
	}
	return event
}

func (s *Site) makePollingWatcher() (<-chan FilesEvent, error) {
	previous, err := s.pollSnapshot()
	if err != nil {
		return nil, err
	}
	changes := make(chan FilesEvent, 100)
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		defer close(changes)
		failed := false
		for range ticker.C {
			next, err := s.pollSnapshot()
			if err != nil {
				if !failed {
					fmt.Fprintf(os.Stderr, "error refreshing source watches: %v\n", err)
					changes <- FilesEvent{Topology: true}
					failed = true
				}
				continue
			}
			event := diffPollSnapshots(previous, next)
			if failed {
				event.Topology = true
				failed = false
			}
			previous = next
			if event.Topology || len(event.Paths) > 0 {
				changes <- event
			}
		}
	}()
	return changes, nil
}
