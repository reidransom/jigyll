package themearchive

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type archiveState struct {
	topLevel  string
	extracted int64
	entries   int64
}

// Extract strips a single enclosing directory and requires a regular
// _layouts/default.html. destination must not exist, and its parent must be a
// caller-owned working directory, not an untrusted consumer directory. Only a
// destination successfully created by this call is removed on failure.
func Extract(archivePath, destination string, limits Limits) (err error) {
	if err := os.Mkdir(destination, 0o755); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(destination)
		}
	}()

	archive, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		return fmt.Errorf("open compressed archive: %w", err)
	}
	defer gzipReader.Close()

	state := archiveState{}
	if err := state.extractEntries(gzipReader, destination, limits); err != nil {
		return err
	}
	// tar EOF precedes gzip EOF. Read through all padding and gzip members
	// to validate trailers, while rejecting hidden payloads and bounding
	// padding (including the conventional 20-block tar record).
	if err := finishGzip(gzipReader, max(limits.ExtractedBytes, 20*512)); err != nil {
		return err
	}
	if state.topLevel == "" {
		return fmt.Errorf("archive contains no entries")
	}
	info, err := os.Lstat(filepath.Join(destination, "_layouts", "default.html"))
	if err != nil {
		return fmt.Errorf("theme is missing _layouts/default.html: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("theme _layouts/default.html is not a regular file")
	}
	return nil
}

func (state *archiveState) extractEntries(source io.Reader, destination string, limits Limits) error {
	counted := &countingReader{reader: source}
	reader := tar.NewReader(counted)
	for {
		before := counted.bytes
		header, readErr := reader.Next()
		if readErr == io.EOF {
			// archive/tar also accepts EOF without the required two zero
			// blocks. A complete terminator consumes 1024 bytes, plus at
			// most 511 bytes of padding from the previous file.
			if counted.bytes-before < 1024 {
				return fmt.Errorf("archive is missing its tar terminator")
			}
			return nil
		}
		if readErr != nil {
			return fmt.Errorf("read archive: %w", readErr)
		}
		state.entries++
		if state.entries > limits.Entries {
			return fmt.Errorf("archive exceeds %d entries", limits.Entries)
		}
		// GitHub emits a global PAX header; it describes archive metadata,
		// not a destination. Local PAX headers are handled by archive/tar.
		if header.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		if err := state.extractEntry(reader, header, destination, limits); err != nil {
			return err
		}
	}
}

type countingReader struct {
	reader io.Reader
	bytes  int64
}

func (r *countingReader) Read(buffer []byte) (int, error) {
	n, err := r.reader.Read(buffer)
	r.bytes += int64(n)
	return n, err
}

func finishGzip(reader io.Reader, paddingLimit int64) error {
	var buffer [32 * 1024]byte
	var padding int64
	for {
		n, err := reader.Read(buffer[:])
		padding += int64(n)
		if padding > paddingLimit {
			return fmt.Errorf("archive has excessive trailing padding")
		}
		for _, value := range buffer[:n] {
			if value != 0 {
				return fmt.Errorf("archive has data after its tar terminator")
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("finish compressed archive: %w", err)
		}
	}
}

func (state *archiveState) extractEntry(reader *tar.Reader, header *tar.Header, destination string, limits Limits) error {
	name, err := validArchivePath(header.Name)
	if err != nil {
		return err
	}
	components := strings.Split(name, "/")
	if state.topLevel == "" {
		state.topLevel = components[0]
	}
	if components[0] != state.topLevel {
		return fmt.Errorf("archive entries must share a top-level directory")
	}
	if len(components) == 1 {
		if header.Typeflag != tar.TypeDir {
			return fmt.Errorf("archive top-level entry must be a directory")
		}
		return nil
	}
	target := filepath.Join(destination, filepath.FromSlash(strings.Join(components[1:], "/")))
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, 0o755)
	case tar.TypeReg, '\x00': // NUL is the historical tar regular-file type.
		if header.Size < 0 || header.Size > limits.FileBytes {
			return fmt.Errorf("archive file %q exceeds %d bytes", header.Name, limits.FileBytes)
		}
		if header.Size > limits.ExtractedBytes-state.extracted {
			return fmt.Errorf("archive exceeds %d extracted bytes", limits.ExtractedBytes)
		}
		state.extracted += header.Size
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := extractRegularFile(reader, target, header.Size); err != nil {
			return fmt.Errorf("extract %q: %w", header.Name, err)
		}
		return nil
	default:
		return fmt.Errorf("archive entry %q has unsupported type", header.Name)
	}
}

func validArchivePath(name string) (string, error) {
	if name == "" || strings.Contains(name, "\\") || path.IsAbs(name) {
		return "", fmt.Errorf("archive entry %q has an unsafe path", name)
	}
	for _, component := range strings.Split(name, "/") {
		if component == ".." || strings.EqualFold(component, ".git") || isWindowsDrive(component) {
			return "", fmt.Errorf("archive entry %q has an unsafe path", name)
		}
	}
	clean := path.Clean(name)
	if clean == "." {
		return "", fmt.Errorf("archive entry %q has an unsafe path", name)
	}
	return clean, nil
}

func isWindowsDrive(name string) bool {
	return len(name) >= 2 && ((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) && name[1] == ':'
}

func extractRegularFile(source io.Reader, destination string, size int64) (err error) {
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	written, err := io.Copy(file, io.LimitReader(source, size))
	if err != nil {
		return err
	}
	if written != size {
		return io.ErrUnexpectedEOF
	}
	return nil
}
