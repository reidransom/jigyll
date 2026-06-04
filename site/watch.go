package site

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/reidransom/jigyll/utils"
	"github.com/radovskyb/watcher"
)

// FilesEvent is a list of changed or added site source files, with a single
// timestamp that approximates when they were changed.
type FilesEvent struct {
	Time  time.Time // A single time is used for all the changes
	Paths []string  // relative to site source
}

func (e FilesEvent) String() string {
	count := len(e.Paths)
	inflect := map[bool]string{true: "", false: "s"}[count == 1]
	return fmt.Sprintf("%d file%s changed at %s", count, inflect, e.Time.Format("3:04:05PM"))
}

// WatchFiles returns a channel that receives FilesEvent on changes within the site directory.
func (s *Site) WatchFiles() (<-chan FilesEvent, error) {
	filenames, err := s.makeFileWatcher()
	if err != nil {
		return nil, err
	}
	var (
		debounced = debounce(time.Second/2, filenames)
		filesets  = make(chan FilesEvent)
	)
	go func() {
		for {
			paths := s.affectsBuildFilter(<-debounced)
			if len(paths) > 0 {
				// Create a new timestamp. Except under pathological
				// circumstances, it will be close enough.
				filesets <- FilesEvent{time.Now(), paths}
			}
		}
	}()
	return filesets, nil
}

func (s *Site) makeFileWatcher() (<-chan string, error) {
	switch {
	case s.cfg.ForcePolling:
		return s.makePollingWatcher()
	default:
		return s.makeEventWatcher()
	}
}

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
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
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
	if err != nil {
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
				// If a new directory is created, start watching it too
				if event.Has(fsnotify.Create) {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						_ = w.Add(event.Name)
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
