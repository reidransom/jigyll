package site

import (
	"fmt"
	"os"
	"time"

	"github.com/reidransom/jigyll/utils"
)

// WatchRebuild watches the site directory. Each time a file changes, it
// rebuilds the site. It sends status messages (strings) and errors to its output
// channel.
//
// TODO use a logger instead of a message channel?
func (s *Site) WatchRebuild() (<-chan interface{}, error) {
	var (
		messages      = make(chan interface{})
		filesets, err = s.WatchFiles()
	)
	if err != nil {
		return nil, err
	}
	go func() {
		for fileset := range filesets {
			s = s.processFilesEvent(fileset, messages)
		}
	}()
	return messages, nil
}

// Reloaded returns the same or a new site reading the same source directory, configuration file, and load flags.
// build --incremental and site --incremental use this.
func (s *Site) Reloaded(paths []string) (*Site, error) {
	if s.RequiresFullReload(paths) {
		copy, err := FromDirectory(s.SourceDir(), s.flags)
		if err != nil {
			return nil, err
		}
		s = copy
	}
	return s, s.Read()
}

// ReloadedForEvent constructs a fresh site for topology changes so a failed
// discovery cannot mutate the site currently served to clients.
func (s *Site) ReloadedForEvent(event FilesEvent) (*Site, error) {
	if !event.Topology {
		return s.Reloaded(event.Paths)
	}
	fresh, err := FromDirectory(s.SourceDir(), s.flags)
	if err != nil {
		return nil, err
	}
	return fresh, fresh.Read()
}

func (s *Site) processFilesEvent(fileset FilesEvent, messages chan<- interface{}) *Site {
	// similar code to server.reload
	messages <- fmt.Sprintf("Regenerating: %s...", fileset)
	start := time.Now()
	r, count, err := s.rebuildEvent(fileset)
	if err != nil {
		fmt.Println()
		fmt.Fprintln(os.Stderr, err)
		return s
	}
	elapsed := time.Since(start)
	inflect := map[bool]string{true: "", false: "s"}[count == 1]
	messages <- fmt.Sprintf("wrote %d file%s in %.2fs.\n", count, inflect, elapsed.Seconds())
	return r
}

func (s *Site) rebuildEvent(event FilesEvent) (*Site, int, error) {
	if !event.Topology {
		return s.rebuild(event.Paths)
	}
	fresh, err := s.ReloadedForEvent(event)
	if err != nil {
		return s, 0, err
	}
	count, err := fresh.Write()
	if err != nil {
		return s, count, err
	}
	return fresh, count, nil
}

// reloads and rebuilds the site; returns a copy and count
func (s *Site) rebuild(paths []string) (r *Site, n int, err error) {
	if s.RequiresFullReload(paths) {
		r, err = s.Reloaded(paths)
		if err != nil {
			return
		}
		n, err = r.Write()
		return
	}
	r = s
	pathSet := utils.MakeStringSet(paths)
	for _, d := range s.docs {
		if s.invalidatesDoc(pathSet, d) {
			err = d.Reload()
			if err != nil {
				return
			}
			err = s.WriteDoc(d)
			if err != nil {
				return
			}
			n++
		}
	}
	return
}
