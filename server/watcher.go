package server

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/reidransom/jigyll/site"
)

// watchReload observes one source watcher. Localized projects use a
// single-flight consumer so watch events cannot start concurrent development
// snapshot rebuilds.
func (s *Server) watchReload() error {
	s.m.RLock()
	project := s.project
	base := s.Site
	s.m.RUnlock()
	var (
		changes <-chan site.FilesEvent
		err     error
	)
	if project != nil {
		changes, err = project.WatchFiles()
	} else {
		changes, err = base.WatchFiles()
	}
	if err != nil {
		return err
	}
	if project != nil {
		go s.watchLocalizedReloads(changes)
		return nil
	}
	go func() {
		for change := range changes {
			batch := s.liveReloadBatch(change)
			if s.reload(change) {
				batch.Deliver()
			}
		}
	}()
	return nil
}

func (s *Server) watchLocalizedReloads(changes <-chan site.FilesEvent) {
	runLocalizedWatchReloads(changes, s.reload, func(change site.FilesEvent) {
		s.liveReloadBatch(change).Deliver()
	})
}

type localizedReloadResult struct {
	change  site.FilesEvent
	success bool
}

// runLocalizedWatchReloads serializes complete localized development rebuilds.
// Events observed while an attempt runs are unioned into one follow-up attempt.
func runLocalizedWatchReloads(
	changes <-chan site.FilesEvent,
	reload func(site.FilesEvent) bool,
	deliver func(site.FilesEvent),
) {
	var (
		pending *site.FilesEvent
		done    <-chan localizedReloadResult
	)
	for {
		if done == nil {
			change, open := <-changes
			if !open {
				return
			}
			done = startLocalizedReload(change, reload)
			continue
		}

		select {
		case change, open := <-changes:
			if !open {
				changes = nil
				continue
			}
			if pending == nil {
				pending = &change
			} else {
				merged := mergeWatchChanges(*pending, change)
				pending = &merged
			}
		case result := <-done:
			if result.success {
				deliver(result.change)
			}
			switch {
			case pending != nil:
				done = startLocalizedReload(*pending, reload)
				pending = nil
			case changes == nil:
				return
			default:
				done = nil
			}
		}
	}
}

func startLocalizedReload(change site.FilesEvent, reload func(site.FilesEvent) bool) <-chan localizedReloadResult {
	done := make(chan localizedReloadResult, 1)
	go func() {
		done <- localizedReloadResult{change: change, success: reload(change)}
	}()
	return done
}

func mergeWatchChanges(first, second site.FilesEvent) site.FilesEvent {
	paths := make([]string, 0, len(first.Paths)+len(second.Paths))
	seen := make(map[string]struct{}, cap(paths))
	for _, path := range first.Paths {
		if _, duplicate := seen[path]; duplicate {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	for _, path := range second.Paths {
		if _, duplicate := seen[path]; duplicate {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return site.FilesEvent{Time: second.Time, Paths: paths}
}

// liveReloadIntent describes the minimum work a client must perform for one
// watch batch. A page reload always takes precedence over resource updates.
type liveReloadIntent struct {
	pageReload    bool
	resourcePaths []string
}

func newLiveReloadIntent(s *site.Site, paths []string) liveReloadIntent {
	if s.RequiresFullReload(paths) {
		return liveReloadIntent{pageReload: true}
	}

	var (
		intent liveReloadIntent
		seen   map[string]struct{}
	)
	for _, path := range paths {
		resourcePath, found := s.FilenameURLPath(path)
		if !found {
			continue
		}
		if !isLiveReloadResource(resourcePath) {
			return liveReloadIntent{pageReload: true}
		}
		if seen == nil {
			seen = make(map[string]struct{}, len(paths))
		}
		if _, duplicate := seen[resourcePath]; duplicate {
			continue
		}
		seen[resourcePath] = struct{}{}
		intent.resourcePaths = append(intent.resourcePaths, resourcePath)
	}
	return intent
}

// liveReloadBatch captures the intent for one watch event while its source
// paths still resolve against the snapshot that observed the event.
func (s *Server) liveReloadBatch(change site.FilesEvent) liveReloadBatch {
	s.m.RLock()
	defer s.m.RUnlock()
	if s.project != nil {
		return liveReloadBatch{
			transport: s.liveReload,
			intent:    liveReloadIntent{pageReload: true},
		}
	}
	return liveReloadBatch{
		transport: s.liveReload,
		intent:    newLiveReloadIntent(s.Site, change.Paths),
	}
}

type liveReloadBatch struct {
	transport *liveReloadTransport
	intent    liveReloadIntent
}

func (b liveReloadBatch) Deliver() {
	if b.transport != nil {
		b.transport.Deliver(b.intent)
	}
}

func isLiveReloadResource(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".css" || strings.HasPrefix(mime.TypeByExtension(extension), "image/")
}

func (s *Server) reload(change site.FilesEvent) bool {
	s.m.RLock()
	project := s.project
	current := s.Site
	liveReload := s.liveReload
	s.m.RUnlock()

	fmt.Printf("Re-reading: %v %v...\n", change, change.Paths)
	start := time.Now()
	if project != nil {
		replacement, _, err := project.RebuildDevelopment()
		if err != nil {
			s.reportReloadError(liveReload, err)
			return false
		}
		s.m.Lock()
		s.project = replacement
		s.m.Unlock()
		fmt.Printf("done (%.2fs)\n", time.Since(start).Seconds())
		return true
	}

	replacement, err := current.Reloaded(change.Paths)
	if err != nil {
		s.reportReloadError(liveReload, err)
		return false
	}
	s.m.Lock()
	s.Site = replacement
	s.m.Unlock()
	// Only clear URL if JEKYLL_URL is not set
	if jekyllURL := os.Getenv("JEKYLL_URL"); jekyllURL != "" {
		replacement.SetAbsoluteURL(jekyllURL)
	} else {
		replacement.SetAbsoluteURL("")
	}
	fmt.Printf("done (%.2fs)\n", time.Since(start).Seconds())
	return true
}

func (s *Server) reportReloadError(liveReload *liveReloadTransport, err error) {
	fmt.Println()
	fmt.Fprintln(os.Stderr, err.Error())
	if liveReload != nil {
		liveReload.Alert(fmt.Sprintf("Error reading site configuration: %s", err))
	}
}
