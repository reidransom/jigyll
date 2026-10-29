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
	if _, err := s.scanWatchInventory(); err != nil {
		return nil, err
	}
	if !s.cfg.ForcePolling {
		changes, err := s.makeEventWatcher()
		if err == nil {
			return changes, nil
		}
		if s.cfg.Verbose {
			fmt.Fprintf(os.Stderr, "Event watcher unavailable (%v), using polling watcher\n", err)
		}
	}
	filenames, err := s.makePollingWatcher()
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

// makeEventWatcher registers each reachable physical directory once and
// translates events back to every logical path that names it.
func (s *Site) makeEventWatcher() (<-chan FilesEvent, error) {
	inventory, err := s.scanWatchInventory()
	if err != nil {
		return nil, err
	}
	dirs := inventory.registrations()
	if len(dirs) > 500 {
		return nil, watchLimitError(len(dirs))
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	for dir := range dirs {
		if err := w.Add(dir); err != nil {
			w.Close()
			return nil, err
		}
	}
	changes := make(chan FilesEvent, 100)
	go func() {
		defer w.Close()
		defer close(changes)
		for {
			select {
			case event, ok := <-w.Events:
				if !ok {
					return
				}
				if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) && !event.Has(fsnotify.Remove) && !event.Has(fsnotify.Rename) {
					continue
				}
				if s.watchProtected(event.Name) {
					continue
				}
				paths, target := inventory.event(event.Name)
				if len(paths) == 0 && !target {
					continue
				}
				topology := target || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename)
				if topology {
					next, scanErr := s.scanWatchInventory()
					if scanErr != nil {
						fmt.Fprintf(os.Stderr, "error refreshing source watches: %v\n", scanErr)
					} else {
						nextDirs := next.registrations()
						if len(nextDirs) > 500 {
							fmt.Fprintln(os.Stderr, watchLimitError(len(nextDirs)))
						} else {
							added := make([]string, 0)
							for dir := range nextDirs {
								if !dirs[dir] {
									if addErr := w.Add(dir); addErr != nil {
										scanErr = addErr
										break
									}
									added = append(added, dir)
								}
							}
							if scanErr != nil {
								for _, dir := range added {
									_ = w.Remove(dir)
								}
								fmt.Fprintf(os.Stderr, "error refreshing source watches: %v\n", scanErr)
							} else {
								for dir := range dirs {
									if !nextDirs[dir] {
										_ = w.Remove(dir)
									}
								}
								dirs, inventory = nextDirs, next
							}
						}
					}
				}
				changes <- FilesEvent{Paths: paths, Topology: topology}
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				fmt.Fprintln(os.Stderr, "error:", err)
			}
		}
	}()
	return changes, nil
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
