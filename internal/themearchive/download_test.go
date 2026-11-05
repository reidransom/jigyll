package themearchive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDownloadBoundaryAndDigest(t *testing.T) {
	payload := []byte("archive")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/archive" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	for _, over := range []bool{false, true} {
		t.Run(fmt.Sprintf("over=%t", over), func(t *testing.T) {
			limits := DefaultLimits()
			limits.CompressedBytes = int64(len(payload))
			if over {
				limits.CompressedBytes--
			}
			destination := filepath.Join(t.TempDir(), "archive.tgz")
			digest, err := Download(context.Background(), server.Client(), server.URL+"/archive", destination, limits)
			if over {
				require.Error(t, err)
				require.Empty(t, digest)
				require.NoFileExists(t, destination)
				return
			}
			require.NoError(t, err)
			contents, err := os.ReadFile(destination)
			require.NoError(t, err)
			require.Equal(t, payload, contents)
			expected := sha256.Sum256(contents)
			require.Equal(t, hex.EncodeToString(expected[:]), digest)
		})
	}
}

func TestDownloadPreservesCompressedBytes(t *testing.T) {
	payload := gzipBytes(t, []byte("archive bytes"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	destination := filepath.Join(t.TempDir(), "archive.tgz")
	digest, err := Download(context.Background(), server.Client(), server.URL, destination, DefaultLimits())
	require.NoError(t, err)
	contents, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, payload, contents)
	expected := sha256.Sum256(payload)
	require.Equal(t, hex.EncodeToString(expected[:]), digest)
}

func TestDownloadFailuresCleanOwnedFiles(t *testing.T) {
	for _, kind := range []string{"HTTP error", "partial response", "interrupted transfer", "timeout", "cancelled request"} {
		t.Run(kind, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch kind {
				case "HTTP error":
					http.Error(w, "failed", http.StatusBadGateway)
				case "partial response":
					w.Header().Set("Content-Length", "100")
					_, _ = w.Write([]byte("partial"))
				case "interrupted transfer":
					connection, _, err := w.(http.Hijacker).Hijack()
					if err == nil {
						_, _ = io.WriteString(connection, "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n7\r\npartial\r\n")
						_ = connection.Close()
					}
				case "timeout":
					_, _ = w.Write([]byte("partial"))
					w.(http.Flusher).Flush()
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			client := server.Client()
			ctx := context.Background()
			if kind == "timeout" {
				client.Timeout = 50 * time.Millisecond
			}
			if kind == "cancelled request" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			root := t.TempDir()
			outside := filepath.Join(root, "outside")
			require.NoError(t, os.WriteFile(outside, []byte("untouched"), 0o600))
			destination := filepath.Join(root, "archive.tgz")
			digest, err := Download(ctx, client, server.URL, destination, DefaultLimits())
			require.Error(t, err)
			require.Empty(t, digest)
			require.NoFileExists(t, destination)
			contents, err := os.ReadFile(outside)
			require.NoError(t, err)
			require.Equal(t, "untouched", string(contents))
			files, err := os.ReadDir(root)
			require.NoError(t, err)
			require.Len(t, files, 1)
		})
	}
}

func TestDownloadRefusesExistingPaths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "archive")
	}))
	defer server.Close()
	for _, kind := range []string{"file", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			outside := filepath.Join(root, "outside")
			require.NoError(t, os.WriteFile(outside, []byte("untouched"), 0o600))
			destination := filepath.Join(root, "archive.tgz")
			switch kind {
			case "file":
				require.NoError(t, os.WriteFile(destination, []byte("untouched"), 0o600))
			case "directory":
				require.NoError(t, os.Mkdir(destination, 0o755))
			case "symlink":
				require.NoError(t, os.Symlink(outside, destination))
			}
			digest, err := Download(context.Background(), server.Client(), server.URL, destination, DefaultLimits())
			require.Error(t, err)
			require.Empty(t, digest)
			_, err = os.Lstat(destination)
			require.NoError(t, err)
			if kind != "directory" {
				contents, err := os.ReadFile(destination)
				require.NoError(t, err)
				require.Equal(t, "untouched", string(contents))
			}
			contents, err := os.ReadFile(outside)
			require.NoError(t, err)
			require.Equal(t, "untouched", string(contents))
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type closeErrorBody struct{ io.Reader }

func (closeErrorBody) Close() error { return errors.New("close failed") }

func TestDownloadHonorsPolicyAndDeadline(t *testing.T) {
	policyError := errors.New("redirect refused")
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		require.True(t, ok)
		require.WithinDuration(t, time.Now().Add(30*time.Second), deadline, time.Second)
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"https://other.example/archive"}}, Body: io.NopCloser(bytes.NewReader(nil)), Request: r}, nil
	}), CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return policyError }}
	destination := filepath.Join(t.TempDir(), "archive.tgz")
	_, err := Download(context.Background(), client, "https://source.example/archive", destination, DefaultLimits())
	require.ErrorIs(t, err, policyError)
	require.NoFileExists(t, destination)
}

func TestDownloadBodyCloseFailureCleansFile(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: closeErrorBody{bytes.NewBufferString("archive")}, Header: make(http.Header), Request: r}, nil
	})}
	destination := filepath.Join(t.TempDir(), "archive.tgz")
	digest, err := Download(context.Background(), client, "https://source.example/archive", destination, DefaultLimits())
	require.Error(t, err)
	require.Empty(t, digest)
	require.NoFileExists(t, destination)
}
