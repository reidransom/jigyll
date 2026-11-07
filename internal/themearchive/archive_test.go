package themearchive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type archiveEntry struct {
	name     string
	typeflag byte
	body     string
	linkname string
}

func TestExtract(t *testing.T) {
	for _, wrapper := range []string{"explicit", "implicit", "normalized", "late explicit"} {
		t.Run(wrapper, func(t *testing.T) {
			entries := []archiveEntry{
				{name: "theme-sha/_layouts/default.html", typeflag: tar.TypeReg, body: "theme layout"},
				{name: "theme-sha/_includes/head.html", typeflag: '\x00', body: "head"}, // Historical tar regular-file type.
				{name: "theme-sha/assets/empty/", typeflag: tar.TypeDir},
			}
			switch wrapper {
			case "explicit":
				entries = append([]archiveEntry{{name: "theme-sha/", typeflag: tar.TypeDir}}, entries...)
			case "normalized":
				entries[0].name = "./theme-sha//_layouts/./default.html"
			case "late explicit":
				entries = append(entries, archiveEntry{name: "theme-sha/", typeflag: tar.TypeDir})
			}
			destination := filepath.Join(t.TempDir(), "theme")
			require.NoError(t, Extract(writeArchive(t, archiveBytes(t, entries)), destination, DefaultLimits()))
			contents, err := os.ReadFile(filepath.Join(destination, "_layouts", "default.html"))
			require.NoError(t, err)
			require.Equal(t, "theme layout", string(contents))
			contents, err = os.ReadFile(filepath.Join(destination, "_includes", "head.html"))
			require.NoError(t, err)
			require.Equal(t, "head", string(contents))
			require.DirExists(t, filepath.Join(destination, "assets", "empty"))
			require.NoDirExists(t, filepath.Join(destination, "theme-sha"))
		})
	}
}

func TestExtractRejectsUnsafeEntries(t *testing.T) {
	valid := archiveEntry{name: "theme/_layouts/default.html", typeflag: tar.TypeReg, body: "layout"}
	for _, tc := range []struct {
		name  string
		entry archiveEntry
	}{
		{name: "absolute path", entry: archiveEntry{name: "/outside", typeflag: tar.TypeReg}},
		{name: "traversal path", entry: archiveEntry{name: "theme/../../outside", typeflag: tar.TypeReg}},
		{name: "internal traversal", entry: archiveEntry{name: "theme/a/../outside", typeflag: tar.TypeReg}},
		{name: "backslash", entry: archiveEntry{name: "theme\\outside", typeflag: tar.TypeReg}},
		{name: "drive absolute", entry: archiveEntry{name: "C:/outside", typeflag: tar.TypeReg}},
		{name: "drive relative", entry: archiveEntry{name: "C:outside", typeflag: tar.TypeReg}},
		{name: "nested drive", entry: archiveEntry{name: "theme/C:outside", typeflag: tar.TypeReg}},
		{name: "symbolic link", entry: archiveEntry{name: "theme/link", typeflag: tar.TypeSymlink, linkname: "../../outside"}},
		{name: "hard link", entry: archiveEntry{name: "theme/link", typeflag: tar.TypeLink, linkname: "theme/_layouts/default.html"}},
		{name: "character device", entry: archiveEntry{name: "theme/char", typeflag: tar.TypeChar}},
		{name: "block device", entry: archiveEntry{name: "theme/block", typeflag: tar.TypeBlock}},
		{name: "FIFO", entry: archiveEntry{name: "theme/fifo", typeflag: tar.TypeFifo}},
		{name: "socket", entry: archiveEntry{name: "theme/socket", typeflag: 's'}},
		{name: "link with directory name", entry: archiveEntry{name: "theme/link/", typeflag: tar.TypeSymlink, linkname: "../../outside"}},
		{name: "duplicate destination", entry: valid},
		{name: "normalized duplicate", entry: archiveEntry{name: "theme//_layouts/./default.html", typeflag: tar.TypeReg}},
		{name: "multiple roots", entry: archiveEntry{name: "other/file", typeflag: tar.TypeReg}},
		{name: "git directory", entry: archiveEntry{name: "theme/.git/", typeflag: tar.TypeDir}},
		{name: "git file", entry: archiveEntry{name: "theme/.git", typeflag: tar.TypeReg}},
		{name: "nested git metadata", entry: archiveEntry{name: "theme/vendor/.git/config", typeflag: tar.TypeReg}},
		{name: "case insensitive git", entry: archiveEntry{name: "theme/.GIT/config", typeflag: tar.TypeReg}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertRejectedArchive(t, archiveBytes(t, []archiveEntry{valid, tc.entry}), DefaultLimits())
		})
	}
	for _, tc := range []struct {
		name    string
		entries []archiveEntry
	}{
		{name: "empty archive"},
		{name: "missing default", entries: []archiveEntry{{name: "theme/file", typeflag: tar.TypeReg}}},
		{name: "directory default", entries: []archiveEntry{{name: "theme/_layouts/default.html/", typeflag: tar.TypeDir}}},
		{name: "file wrapper", entries: []archiveEntry{{name: "theme", typeflag: tar.TypeReg}}},
		{name: "link wrapper", entries: []archiveEntry{{name: "theme", typeflag: tar.TypeSymlink, linkname: "outside"}}},
		{name: "git wrapper", entries: []archiveEntry{{name: ".git/_layouts/default.html", typeflag: tar.TypeReg}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertRejectedArchive(t, archiveBytes(t, tc.entries), DefaultLimits())
		})
	}
}

func TestExtractBoundaries(t *testing.T) {
	archive := archiveBytes(t, []archiveEntry{
		{name: "theme/", typeflag: tar.TypeDir},
		{name: "theme/_layouts/default.html", typeflag: tar.TypeReg, body: "abc"},
		{name: "theme/_includes/head.html", typeflag: tar.TypeReg, body: "def"},
	})
	for _, dimension := range []string{"file bytes", "extracted bytes", "entries"} {
		t.Run(dimension, func(t *testing.T) {
			limits := Limits{CompressedBytes: int64(len(archive)), FileBytes: 3, ExtractedBytes: 6, Entries: 3}
			destination := filepath.Join(t.TempDir(), "at-boundary")
			require.NoError(t, Extract(writeArchive(t, archive), destination, limits))
			contents, err := os.ReadFile(filepath.Join(destination, "_includes", "head.html"))
			require.NoError(t, err)
			require.Equal(t, "def", string(contents))
			switch dimension {
			case "file bytes":
				limits.FileBytes--
			case "extracted bytes":
				limits.ExtractedBytes--
			case "entries":
				limits.Entries--
			}
			assertRejectedArchive(t, archive, limits)
		})
	}
}

func TestExtractRejectsCorruptStreams(t *testing.T) {
	raw := tarBytes(t, []archiveEntry{{name: "theme/_layouts/default.html", typeflag: tar.TypeReg, body: "layout"}})
	valid := gzipBytes(t, raw)
	badTrailer := bytes.Clone(valid)
	badTrailer[len(badTrailer)-8] ^= 0xff
	badHeader := bytes.Clone(raw)
	badHeader[0] ^= 0xff
	badMember := gzipBytes(t, nil)
	badMember[len(badMember)-8] ^= 0xff
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{name: "invalid gzip", data: []byte("not gzip")},
		{name: "truncated gzip header", data: valid[:5]},
		{name: "truncated gzip payload", data: valid[:len(valid)/2]},
		{name: "truncated gzip trailer", data: valid[:len(valid)-1]},
		{name: "bad gzip trailer after valid tar", data: badTrailer},
		{name: "bad later gzip member", data: append(bytes.Clone(valid), badMember...)},
		{name: "trailing non-gzip bytes", data: append(bytes.Clone(valid), []byte("junk")...)},
		{name: "invalid tar header", data: gzipBytes(t, badHeader)},
		{name: "truncated tar header", data: gzipBytes(t, raw[:100])},
		{name: "truncated tar file", data: gzipBytes(t, raw[:514])},
		{name: "truncated tar padding", data: gzipBytes(t, raw[:520])},
		{name: "missing tar terminator", data: gzipBytes(t, raw[:1024])},
		{name: "half tar terminator", data: gzipBytes(t, raw[:1536])},
		{name: "hidden tar payload", data: gzipBytes(t, append(bytes.Clone(raw), raw...))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertRejectedArchive(t, tc.data, DefaultLimits())
		})
	}
}

func TestExtractValidatesGzipPaddingAndGlobalMetadata(t *testing.T) {
	entries := []archiveEntry{
		{name: "pax_global_header", typeflag: tar.TypeXGlobalHeader},
		{name: "theme/_layouts/default.html", typeflag: tar.TypeReg, body: "layout"},
	}
	raw := tarBytes(t, entries)
	padded := gzipBytes(t, append(raw, make([]byte, 4096)...))
	require.NoError(t, Extract(writeArchive(t, padded), filepath.Join(t.TempDir(), "theme"), DefaultLimits()))
	limits := DefaultLimits()
	limits.ExtractedBytes = 6
	assertRejectedArchive(t, gzipBytes(t, append(raw, make([]byte, 20*512+1)...)), limits)
}

func TestExtractRefusesExistingDestinations(t *testing.T) {
	archive := writeArchive(t, []byte("invalid archive"))
	for _, kind := range []string{"empty directory", "nonempty directory", "file", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			destination := filepath.Join(root, "theme")
			outside := filepath.Join(root, "outside")
			require.NoError(t, os.Mkdir(outside, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(outside, "sentinel"), []byte("untouched"), 0o600))
			switch kind {
			case "empty directory", "nonempty directory":
				require.NoError(t, os.Mkdir(destination, 0o755))
				if kind == "nonempty directory" {
					require.NoError(t, os.WriteFile(filepath.Join(destination, "sentinel"), []byte("untouched"), 0o600))
				}
			case "file":
				require.NoError(t, os.WriteFile(destination, []byte("untouched"), 0o600))
			case "symlink":
				require.NoError(t, os.Symlink(outside, destination))
			}
			require.Error(t, Extract(archive, destination, DefaultLimits()))
			_, err := os.Lstat(destination)
			require.NoError(t, err)
			if kind == "empty directory" {
				files, err := os.ReadDir(destination)
				require.NoError(t, err)
				require.Empty(t, files)
			} else {
				file := destination
				if kind != "file" {
					file = filepath.Join(destination, "sentinel")
				}
				contents, err := os.ReadFile(file)
				require.NoError(t, err)
				require.Equal(t, "untouched", string(contents))
			}
			contents, err := os.ReadFile(filepath.Join(outside, "sentinel"))
			require.NoError(t, err)
			require.Equal(t, "untouched", string(contents))
		})
	}
}

func assertRejectedArchive(t *testing.T, archive []byte, limits Limits) {
	t.Helper()
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	require.NoError(t, os.WriteFile(outside, []byte("untouched"), 0o600))
	destination := filepath.Join(root, "theme")
	require.Error(t, Extract(writeArchive(t, archive), destination, limits))
	require.NoDirExists(t, destination)
	contents, err := os.ReadFile(outside)
	require.NoError(t, err)
	require.Equal(t, "untouched", string(contents))
	files, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Len(t, files, 1)
}

func archiveBytes(t *testing.T, entries []archiveEntry) []byte {
	t.Helper()
	return gzipBytes(t, tarBytes(t, entries))
}

func tarBytes(t *testing.T, entries []archiveEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Typeflag: entry.typeflag, Size: int64(len(entry.body)), Linkname: entry.linkname}
		require.NoError(t, writer.WriteHeader(header))
		if entry.body != "" {
			_, err := writer.Write([]byte(entry.body))
			require.NoError(t, err)
		}
	}
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}

func gzipBytes(t *testing.T, raw []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	_, err := writer.Write(raw)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}

func writeArchive(t *testing.T, data []byte) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "theme.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, data, 0o600))
	return archivePath
}
