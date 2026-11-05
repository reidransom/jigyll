package commands

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/reidransom/jigyll/internal/themearchive"
)

var (
	new      = app.Command("new", "Create a new Jigyll site")
	newPath  = new.Arg("PATH", "Directory for the new site").Required().String()
	newTheme = new.Flag("theme", "Theme repository reference (owner/repository@latest, URL@tag, or @full-commit-SHA) or Git URL/local repository").String()
)

//go:embed all:starter
var starterFiles embed.FS

func newCommand() error {
	return scaffoldNewSite(*newPath, *newTheme)
}

// newSiteOperations keeps filesystem failure injection local to the scaffold
// transaction; production always uses the ordinary filesystem operations.
type newSiteOperations struct {
	rename    func(string, string) error
	writeFile func(string, []byte, fs.FileMode) error
}

func scaffoldNewSite(path, themeURL string) error {
	return scaffoldNewSiteWithOperations(path, themeURL, newSiteOperations{
		rename: renameNewSite, writeFile: os.WriteFile,
	})
}

func scaffoldNewSiteWithOperations(path, themeURL string, operations newSiteOperations) error {
	target, _, err := resolveNewSiteTarget(path)
	if err != nil {
		return err
	}

	reference, isReference, err := parseThemeReference(themeURL)
	if err != nil {
		return err
	}
	themeName := reference.Name
	if themeURL != "" && !isReference {
		themeName, err = themeDirectoryName(themeURL)
		if err != nil {
			return err
		}
	}

	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create parent directory for %q: %w", target, err)
	}

	staging, err := os.MkdirTemp(parent, "."+filepath.Base(target)+".jigyll-new-*")
	if err != nil {
		return fmt.Errorf("create staging directory for %q: %w", target, err)
	}
	defer os.RemoveAll(staging)

	if err := writeStarterSite(staging, themeName); err != nil {
		return err
	}
	if isReference {
		if err := installThemeReference(staging, reference, operations.writeFile); err != nil {
			return err
		}
	} else if themeURL != "" {
		if err := cloneTheme(staging, themeURL, themeName); err != nil {
			return err
		}
	}

	// Recheck after network/filesystem work. Publication must not remove an
	// originally empty target before it can safely replace or restore it.
	// A non-empty directory (including a concurrent installation) is refused.
	if _, _, err := resolveNewSiteTarget(target); err != nil {
		return err
	}
	if err := operations.rename(staging, target); err != nil {
		return fmt.Errorf("publish new site at %q: %w", target, err)
	}
	return nil
}

func installThemeReference(root string, reference themeReference, writeFile func(string, []byte, fs.FileMode) error) error {
	client := themearchive.NewHTTPClient(nil)
	resolved, err := resolveThemeReference(context.Background(), client, reference)
	if err != nil {
		return err
	}
	downloadDir, err := os.MkdirTemp(root, ".jigyll-theme-download-*")
	if err != nil {
		return fmt.Errorf("create theme download directory: %w", err)
	}
	defer os.RemoveAll(downloadDir)
	archive := filepath.Join(downloadDir, "theme.tar.gz")
	digest, err := themearchive.Download(context.Background(), client, resolved.ArchiveURL, archive, themearchive.DefaultLimits())
	if err != nil {
		return fmt.Errorf("download theme %q: %w", reference.Source, err)
	}
	themeRoot := filepath.Join(root, "_theme")
	if err := os.MkdirAll(themeRoot, 0o755); err != nil {
		return fmt.Errorf("create theme directory: %w", err)
	}
	if err := themearchive.Extract(archive, filepath.Join(themeRoot, resolved.Name), themearchive.DefaultLimits()); err != nil {
		return fmt.Errorf("extract theme %q: %w", reference.Source, err)
	}
	// This is an observed digest of the downloaded bytes, not independent
	// publisher verification or an expected-checksum comparison.
	provenance := struct {
		resolvedThemeSource
		SHA256 string `json:"sha256"`
	}{resolved, digest}
	content, err := json.MarshalIndent(provenance, "", "  ")
	if err != nil {
		return fmt.Errorf("encode theme provenance: %w", err)
	}
	content = append(content, '\n')
	if err := writeFile(filepath.Join(themeRoot, resolved.Name+".source.json"), content, 0o644); err != nil {
		return fmt.Errorf("write theme provenance: %w", err)
	}
	if err := os.RemoveAll(downloadDir); err != nil {
		return fmt.Errorf("remove theme download: %w", err)
	}
	return nil
}

func resolveNewSiteTarget(path string) (string, bool, error) {
	target, err := filepath.Abs(path)
	if err != nil {
		return "", false, fmt.Errorf("resolve target path %q: %w", path, err)
	}
	if target == filepath.Dir(target) {
		return "", false, errors.New("target directory cannot be the filesystem root")
	}

	info, err := os.Lstat(target)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return target, false, nil
	case err != nil:
		return "", false, fmt.Errorf("inspect target directory %q: %w", target, err)
	case info.Mode()&os.ModeSymlink != 0 || !info.IsDir():
		return "", false, fmt.Errorf("target path %q is not a directory", target)
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return "", false, fmt.Errorf("read target directory %q: %w", target, err)
	}
	if len(entries) != 0 {
		return "", false, fmt.Errorf("target directory %q is not empty", target)
	}
	return target, true, nil
}

func writeStarterSite(root, themeName string) error {
	for _, dir := range []string{"_data", "_drafts", "_includes", "_layouts", "_posts", "_sass", "assets"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			return fmt.Errorf("create starter directory %q: %w", dir, err)
		}
	}

	return fs.WalkDir(starterFiles, "starter", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk embedded starter path %q: %w", path, walkErr)
		}

		if path == "starter" {
			return nil
		}
		relativePath := strings.TrimPrefix(path, "starter/")
		if entry.IsDir() {
			if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(relativePath)), 0o755); err != nil {
				return fmt.Errorf("create starter directory %q: %w", relativePath, err)
			}
			return nil
		}
		if themeName != "" && relativePath == "_layouts/default.html" {
			return nil
		}

		content, err := starterFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded starter file %q: %w", relativePath, err)
		}
		if themeName != "" && relativePath == "_config.yml" {
			content = append(content, "theme: "+themeName+"\n"...)
		}
		return writeStarterFile(root, filepath.FromSlash(relativePath), content)
	})
}

func writeStarterFile(root, path string, content []byte) error {
	if err := os.WriteFile(filepath.Join(root, path), content, 0o644); err != nil {
		return fmt.Errorf("write starter file %q: %w", path, err)
	}
	return nil
}

func cloneTheme(root, themeURL, themeName string) error {
	themeDir := filepath.Join(root, "_theme", themeName)
	if err := os.MkdirAll(filepath.Dir(themeDir), 0o755); err != nil {
		return fmt.Errorf("create theme directory: %w", err)
	}

	command := exec.Command("git", "clone", "--depth", "1", themeURL, themeDir)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("clone theme %q: %w: %s", themeURL, err, strings.TrimSpace(string(output)))
	}

	layout := filepath.Join(themeDir, "_layouts", "default.html")
	info, err := os.Stat(layout)
	if err != nil {
		return fmt.Errorf("theme %q does not provide _layouts/default.html: %w", themeName, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("theme %q has a non-file _layouts/default.html", themeName)
	}
	return nil
}

func themeDirectoryName(gitURL string) (string, error) {
	path := gitURL
	if strings.Contains(gitURL, "://") || strings.HasPrefix(gitURL, "file:") {
		parsed, err := url.Parse(gitURL)
		if err != nil {
			return "", fmt.Errorf("parse theme URL %q: %w", gitURL, err)
		}
		path = parsed.Path
	}
	path = strings.TrimRight(path, "/\\")
	name := path[strings.LastIndexAny(path, "/\\:")+1:]
	name = strings.TrimSuffix(name, ".git")
	if !isSafeThemeDirectoryName(name) {
		return "", fmt.Errorf("theme URL %q does not yield a safe directory name", gitURL)
	}
	return name, nil
}

func isSafeThemeDirectoryName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, char := range name {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '-' || char == '_' || char == '.' {
			continue
		}
		return false
	}
	return true
}
