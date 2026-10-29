package site

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/radovskyb/watcher"
	"github.com/reidransom/jigyll/utils"
)

// FilesEvent is a list of changed or added site source files, with a single
// timestamp that approximates when they were changed.
type FilesEvent struct {
	Time     time.Time // A single time is used for all the changes
	Paths    []string  // relative to site source
	Topology bool      // source tree structure changed; require complete rediscovery
}

func (e FilesEvent) String() string {
	count := len(e.Paths)
	inflect := map[bool]string{true: "", false: "s"}[count == 1]
	return fmt.Sprintf("%d file%s changed at %s", count, inflect, e.Time.Format("3:04:05PM"))
}

// WatchFiles returns a channel that receives FilesEvent on changes within the site directory.
func (s *Site) WatchFiles() (<-chan FilesEvent, error) {
	changes, err := s.makeFileWatcher()
	if err != nil {
		return nil, err
	}
	var (
		debounced = debounceEvents(time.Second/2, changes)
		filesets  = make(chan FilesEvent)
	)
	go func() {
		for change := range debounced {
			change.Paths = s.affectsBuildFilter(change.Paths)
			if len(change.Paths) > 0 || change.Topology {
				change.Time = time.Now()
				filesets <- change
			}
		}
	}()
	return filesets, nil
}

func (s *Site) makeFileWatcher() (<-chan FilesEvent, error) {
	var filenames <-chan string
	var err error
	if s.cfg.ForcePolling {
		filenames, err = s.makePollingWatcher()
	} else {
		filenames, err = s.tryEventWatcher()
		if err != nil {
			if s.cfg.Verbose {
				fmt.Fprintf(os.Stderr, "Event watcher unavailable (%v), using polling watcher\n", err)
			}
			filenames, err = s.makePollingWatcher()
		}
	}
	if err != nil {
		return nil, err
	}
	changes := make(chan FilesEvent)
	go func() {
		for filename := range filenames {
			changes <- FilesEvent{Paths: []string{filename}}
		}
		close(changes)
	}()
	return changes, nil
}

// tryEventWatcher attempts to create an fsnotify watcher, returning an error
// if there are too many directories (to avoid exhausting file descriptors)
func (s *Site) tryEventWatcher() (<-chan string, error) {
	sourceDir := s.SourceDir()
	dirCount := 0
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		rel := utils.MustRel(sourceDir, path)
		switch {
		case rel != "." && s.Exclude(rel):
			return filepath.SkipDir
		case path == s.DestDir():
			return filepath.SkipDir
		}
		dirCount++
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Conservative limit to avoid exhausting inotify watches
	// (default limit is often 8192, but other programs use them too)
	const maxDirs = 500
	if dirCount > maxDirs {
		return nil, fmt.Errorf("directory count %d exceeds safe limit %d for fsnotify", dirCount, maxDirs)
	}
	return s.makeEventWatcher()
}

//nolint:gocyclo
func (s *Site) makeEventWatcher() (<-chan string, error) {
	var (
		sourceDir = s.SourceDir()
		filenames = make(chan string, 100)
		w, err    = fsnotify.NewWatcher()
	)
	if err != nil {
		return nil, err
	}
	// Watch the source directory and all subdirectories recursively.
	// fsnotify.Watcher.Add only watches a single directory, so we walk
	// the tree and add each directory individually.
	addRecursive := func(dir string) error {
		return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				rel := utils.MustRel(sourceDir, path)
				switch {
				case rel != "." && s.Exclude(rel):
					return filepath.SkipDir
				case path == s.DestDir():
					return filepath.SkipDir
				}
				return w.Add(path)
			}
			return nil
		})
	}
	if err := addRecursive(sourceDir); err != nil {
		return nil, err
	}
	go func() {
		for {
			select {
			case event := <-w.Events:
				// Ignore attribute-only changes (Chmod). On macOS, reading a
				// file can update extended attributes (e.g.
				// com.apple.lastuseddate#PS), which fires a Chmod event and
				// causes an infinite rebuild loop.
				if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) &&
					!event.Has(fsnotify.Remove) && !event.Has(fsnotify.Rename) {
					continue
				}
				// If a new directory is created, watch it and any
				// subdirectories created along with it (e.g. mkdir -p)
				if event.Has(fsnotify.Create) {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						if err := addRecursive(event.Name); err != nil {
							fmt.Fprintf(os.Stderr, "error adding directory to watcher: %v\n", err)
						}
					}
				}
				filenames <- utils.MustRel(sourceDir, event.Name)
			case err := <-w.Errors:
				fmt.Fprintln(os.Stderr, "error:", err)
			}
		}
	}()
	return filenames, nil
}

func (s *Site) makePollingWatcher() (<-chan string, error) {
	var (
		sourceDir = utils.MustAbs(s.SourceDir())
		filenames = make(chan string, 100)
		w         = watcher.New()
	)
	if err := w.AddRecursive(sourceDir); err != nil {
		return nil, err
	}
	for _, path := range s.cfg.Exclude {
		if err := w.Ignore(filepath.Join(sourceDir, path)); err != nil {
			return nil, err
		}
	}
	if err := w.Ignore(s.DestDir()); err != nil {
		return nil, err
	}
	go func() {
		for {
			select {
			case event := <-w.Event:
				filenames <- utils.MustRel(sourceDir, event.Path)
			case err := <-w.Error:
				fmt.Fprintln(os.Stderr, "error:", err)
			case <-w.Closed:
				return
			}
		}
	}()
	go func() {
		if err := w.Start(time.Millisecond * 250); err != nil {
			log.Fatal(err)
		}
	}()
	return filenames, nil
}

// debounceEvents preserves topology changes even when no file path survives.
func debounceEvents(interval time.Duration, input <-chan FilesEvent) <-chan FilesEvent {
	output := make(chan FilesEvent)
	go func() {
		defer close(output)
		var pending FilesEvent
		var timer <-chan time.Time
		for {
			select {
			case event, ok := <-input:
				if !ok {
					if len(pending.Paths) != 0 || pending.Topology {
						output <- pending
					}
					return
				}
				for _, path := range event.Paths {
					if path != "." {
						pending.Paths = append(pending.Paths, path)
					}
				}
				pending.Topology = pending.Topology || event.Topology
				timer = time.After(interval)
			case <-timer:
				timer = nil
				if len(pending.Paths) != 0 || pending.Topology {
					output <- pending
					pending = FilesEvent{}
				}
			}
		}
	}()
	return output
}

// debounce relays values from input to output, merging successive values so long as they keep changing
// faster than interval
// TODO consider https://github.com/ReactiveX/RxGo
func debounce(interval time.Duration, input <-chan string) <-chan []string {
	var (
		pending = []string{}
		output  = make(chan []string)
		ticker  <-chan time.Time
	)
	go func() {
		for {
			select {
			case value := <-input:
				if value == "." {
					continue
				}
				pending = append(pending, value)
				ticker = time.After(interval) // replaces the previous ticker
			case <-ticker:
				ticker = nil
				if len(pending) > 0 {
					output <- pending
					pending = []string{}
				}
			}
		}
	}()
	return output
}
