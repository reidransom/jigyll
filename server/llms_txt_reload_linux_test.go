package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/reidransom/jigyll/site"
	"github.com/stretchr/testify/require"
)

func TestLLMsReloadPublishesWholeArtifactsDuringConcurrentPreparation(t *testing.T) {
	t.Setenv("JEKYLL_URL", "")
	source, server := newLLMsReloadServer(t, true)
	old := map[string]string{
		"/project/llms.txt":       llmsReloadResponse(t, server, "/project/llms.txt", http.StatusOK),
		"/project/guide/index.md": llmsReloadResponse(t, server, "/project/guide/index.md", http.StatusOK),
	}
	httpServer := httptest.NewServer(server.routes())
	defer httpServer.Close()
	client := &http.Client{Timeout: 5 * time.Second}

	writeLocalizedServerFile(t, source, "guide.md", llmsReloadArticle("Concurrent Guide", "/guide/", "", "CONCURRENT CONTENT TWO"))
	include := filepath.Join(source, "_includes", "body.md")
	require.NoError(t, os.Remove(include))
	// A FIFO blocks the real include read inside candidate rendering, without a
	// production test hook or a timing-dependent assumption about preparation.
	require.NoError(t, syscall.Mkfifo(include, 0600))
	opened := make(chan error, 1)
	writerResult := make(chan error, 1)
	writerFinished := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseCandidate := func() { releaseOnce.Do(func() { close(release) }) }
	reloadResult := make(chan bool, 1)
	reloadFinished := make(chan struct{})
	continueRequests := make(chan struct{})
	var continueOnce sync.Once
	resumeRequests := func() { continueOnce.Do(func() { close(continueRequests) }) }

	t.Cleanup(func() {
		releaseCandidate()
		resumeRequests()
		// Also unblock a writer if candidate preparation failed before opening
		// the include, so failed assertions cannot strand an open goroutine.
		unblock, err := os.OpenFile(include, os.O_RDWR|syscall.O_NONBLOCK, 0600)
		if err == nil {
			defer unblock.Close()
		}
		for _, finished := range []<-chan struct{}{writerFinished, reloadFinished} {
			select {
			case <-finished:
			case <-time.After(10 * time.Second):
				t.Error("candidate preparation did not finish after releasing the include")
			}
		}
	})

	go func() {
		defer close(writerFinished)
		writer, err := os.OpenFile(include, os.O_WRONLY, 0600)
		opened <- err
		if err != nil {
			writerResult <- err
			return
		}
		<-release
		_, err = io.WriteString(writer, "CONCURRENT INCLUDE TWO {{ site.title }}\n")
		closeErr := writer.Close()
		if err == nil {
			err = closeErr
		}
		writerResult <- err
	}()
	go func() {
		defer close(reloadFinished)
		reloadResult <- server.reload(site.FilesEvent{Paths: []string{"guide.md", "_includes/body.md"}})
	}()
	select {
	case err := <-opened:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("candidate rendering did not open its included fragment")
	}

	type fetchedArtifact struct {
		route  string
		status int
		body   string
		err    error
	}
	fetch := func(route string) fetchedArtifact {
		result := fetchedArtifact{route: route}
		response, err := client.Get(httpServer.URL + route)
		if err != nil {
			result.err = err
			return result
		}
		defer response.Body.Close()
		result.status = response.StatusCode
		body, err := io.ReadAll(response.Body)
		result.body, result.err = string(body), err
		return result
	}
	const readers = 8
	first := make(chan fetchedArtifact, readers)
	results := make(chan fetchedArtifact, readers*32)
	var requests sync.WaitGroup
	for reader := range readers {
		requests.Add(1)
		go func(reader int) {
			defer requests.Done()
			routes := []string{"/project/llms.txt", "/project/guide/index.md"}
			first <- fetch(routes[reader%len(routes)])
			<-continueRequests
			for request := range 32 {
				results <- fetch(routes[(reader+request)%len(routes)])
			}
		}(reader)
	}
	requestsFinished := make(chan struct{})
	go func() {
		requests.Wait()
		close(results)
		close(requestsFinished)
	}()
	t.Cleanup(func() {
		resumeRequests()
		select {
		case <-requestsFinished:
		case <-time.After(10 * time.Second):
			t.Error("concurrent artifact requests did not finish")
		}
	})

	for range readers {
		result := <-first
		require.NoError(t, result.err)
		require.Equal(t, http.StatusOK, result.status)
		require.Equal(t, old[result.route], result.body, "candidate include is still blocked: %s", result.route)
	}
	resumeRequests()
	releaseCandidate()
	select {
	case <-reloadFinished:
		require.True(t, <-reloadResult)
	case <-time.After(10 * time.Second):
		t.Fatal("reload did not publish after the include was released")
	}
	require.NoError(t, <-writerResult)
	newArtifacts := map[string]string{
		"/project/llms.txt":       llmsReloadResponse(t, server, "/project/llms.txt", http.StatusOK),
		"/project/guide/index.md": llmsReloadResponse(t, server, "/project/guide/index.md", http.StatusOK),
	}
	require.Contains(t, newArtifacts["/project/llms.txt"], "[Concurrent Guide](<https://canonical.test/project/guide/index.md>)")
	require.Contains(t, newArtifacts["/project/guide/index.md"], "CONCURRENT CONTENT TWO")
	require.Contains(t, newArtifacts["/project/guide/index.md"], "CONCURRENT INCLUDE TWO Canonical Project")
	for route, body := range newArtifacts {
		require.NotEqual(t, old[route], body)
		require.NotContains(t, body, "preview.test")
	}
	// Each request independently chooses a generation. Requests on opposite
	// sides of publication are not required to belong to the same generation.
	for result := range results {
		require.NoError(t, result.err)
		require.Equal(t, http.StatusOK, result.status)
		require.True(t, result.body == old[result.route] || result.body == newArtifacts[result.route],
			"partial or mixed generation for %s: %q", result.route, result.body)
	}
}
