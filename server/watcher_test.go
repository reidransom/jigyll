package server

import (
	"sync"
	"testing"
	"time"

	"github.com/reidransom/jigyll/site"
	"github.com/stretchr/testify/require"
)

func TestMergeWatchChangesRetainsTopology(t *testing.T) {
	for _, events := range [][2]site.FilesEvent{
		{{Topology: true}, {Paths: []string{"page.md"}}},
		{{Paths: []string{"page.md"}}, {Topology: true}},
	} {
		merged := mergeWatchChanges(events[0], events[1])
		require.True(t, merged.Topology)
		require.Equal(t, []string{"page.md"}, merged.Paths)
	}
}

func TestLocalizedWatchReloadsCoalescesEventsDuringAnActiveRebuild(t *testing.T) {
	changes := make(chan site.FilesEvent)
	results := make(chan bool)
	started := make(chan site.FilesEvent)
	delivered := make(chan site.FilesEvent)
	finished := make(chan struct{})
	var (
		mu         sync.Mutex
		active     int
		overlapped bool
	)

	go func() {
		runLocalizedWatchReloads(changes, func(change site.FilesEvent) bool {
			mu.Lock()
			active++
			overlapped = overlapped || active > 1
			mu.Unlock()
			started <- change
			result := <-results
			mu.Lock()
			active--
			mu.Unlock()
			return result
		}, func(change site.FilesEvent) {
			delivered <- change
		})
		close(finished)
	}()

	first := site.FilesEvent{Time: time.Unix(1, 0), Paths: []string{"index.md"}}
	changes <- first
	require.Equal(t, first, <-started)

	changes <- site.FilesEvent{Time: time.Unix(2, 0), Paths: []string{"_layouts/default.html", "index.md"}}
	changes <- site.FilesEvent{Time: time.Unix(3, 0), Paths: []string{"assets/site.css"}}

	results <- true
	require.Equal(t, first, <-delivered)

	followUp := <-started
	require.Equal(t, time.Unix(3, 0), followUp.Time)
	require.Equal(t, []string{"_layouts/default.html", "index.md", "assets/site.css"}, followUp.Paths)

	results <- true
	require.Equal(t, followUp, <-delivered)
	close(changes)
	<-finished
	mu.Lock()
	noOverlap := !overlapped
	mu.Unlock()
	require.True(t, noOverlap)
}

func TestLocalizedWatchReloadsProcessesPendingChangesAfterFailure(t *testing.T) {
	changes := make(chan site.FilesEvent)
	results := make(chan bool)
	started := make(chan site.FilesEvent)
	delivered := make(chan site.FilesEvent, 1)
	finished := make(chan struct{})

	go func() {
		runLocalizedWatchReloads(changes, func(change site.FilesEvent) bool {
			started <- change
			return <-results
		}, func(change site.FilesEvent) {
			delivered <- change
		})
		close(finished)
	}()

	first := site.FilesEvent{Time: time.Unix(1, 0), Paths: []string{"index.md"}}
	changes <- first
	require.Equal(t, first, <-started)

	pending := site.FilesEvent{Time: time.Unix(2, 0), Paths: []string{"_data/navigation.yml"}}
	changes <- pending
	results <- false

	require.Equal(t, pending, <-started)
	results <- true
	require.Equal(t, pending, <-delivered)
	close(changes)
	<-finished
}

func TestLocalizedWatchReloadsDoesNotRetryFailedRebuildWithoutAnotherEvent(t *testing.T) {
	changes := make(chan site.FilesEvent)
	started := make(chan site.FilesEvent, 1)
	delivered := make(chan site.FilesEvent, 1)
	finished := make(chan struct{})

	go func() {
		runLocalizedWatchReloads(changes, func(change site.FilesEvent) bool {
			started <- change
			return false
		}, func(change site.FilesEvent) {
			delivered <- change
		})
		close(finished)
	}()

	change := site.FilesEvent{Time: time.Unix(1, 0), Paths: []string{"index.md"}}
	changes <- change
	require.Equal(t, change, <-started)
	close(changes)
	<-finished
	require.Empty(t, delivered)
}

func TestLocalizedWatchReloadsStartsAnotherAttemptAfterFailureAndLaterEvent(t *testing.T) {
	changes := make(chan site.FilesEvent)
	results := make(chan bool)
	started := make(chan site.FilesEvent)
	delivered := make(chan site.FilesEvent)
	finished := make(chan struct{})

	go func() {
		runLocalizedWatchReloads(changes, func(change site.FilesEvent) bool {
			started <- change
			return <-results
		}, func(change site.FilesEvent) {
			delivered <- change
		})
		close(finished)
	}()

	failed := site.FilesEvent{Time: time.Unix(1, 0), Paths: []string{"index.md"}}
	changes <- failed
	require.Equal(t, failed, <-started)
	results <- false

	corrected := site.FilesEvent{Time: time.Unix(2, 0), Paths: []string{"index.md"}}
	changes <- corrected
	require.Equal(t, corrected, <-started)
	results <- true
	require.Equal(t, corrected, <-delivered)
	close(changes)
	<-finished
}
