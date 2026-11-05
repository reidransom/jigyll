package themearchive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Download streams an archive to an exclusively created disposable file and
// returns the observed SHA-256 of its compressed bytes. It removes only the file
// it created on failure. The client supplies transport and redirect policy; a
// request deadline of at most 30 seconds applies even with an unbounded client.
func Download(ctx context.Context, client *http.Client, url, destination string, limits Limits) (digest string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create archive request: %w", err)
	}
	// Keep archive bytes intact even when a server labels the response gzip.
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download archive: %w", err)
	}
	// Close the response before deciding whether to keep the owned file.
	var file *os.File
	defer func() {
		if closeErr := resp.Body.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		if file != nil {
			if closeErr := file.Close(); err == nil && closeErr != nil {
				err = closeErr
			}
			if err != nil {
				_ = os.Remove(destination)
			}
		}
		if err != nil {
			digest = ""
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download archive: expected HTTP 200, got %s", resp.Status)
	}
	file, err = os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(resp.Body, limits.CompressedBytes+1))
	if err != nil {
		return "", fmt.Errorf("save archive: %w", err)
	}
	if written > limits.CompressedBytes {
		return "", fmt.Errorf("compressed archive exceeds %d bytes", limits.CompressedBytes)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
