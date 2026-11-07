package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// themeReference is an install-time selection, independent of remote_theme.
// A recognized reference must never fall back to Git when resolution fails.
type themeReference struct {
	Source        string
	RepositoryURL string
	Owner         string
	Repository    string
	Reference     string
	Name          string
	GitHub        bool
}

type resolvedThemeSource struct {
	Source         string `json:"source"`
	RepositoryURL  string `json:"repository_url"`
	ResolvedTag    string `json:"resolved_tag,omitempty"`
	ResolvedCommit string `json:"resolved_commit"`
	ArchiveURL     string `json:"archive_url"`
	Name           string `json:"-"`
}

var themeCommitSHA = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

// parseThemeReference distinguishes versioned repository selections from Git
// transport URLs and local paths. Existing local paths take precedence over
// shorthand; ambiguous new relative paths can be made explicit with ./.
func parseThemeReference(raw string) (themeReference, bool, error) {
	var result themeReference
	if isLocalThemeReference(raw) {
		return result, false, nil
	}
	selection, recognized, err := parseThemeRepositorySelection(raw)
	if err != nil || !recognized {
		return result, recognized, err
	}
	if strings.Count(selection.repository, "/") != 1 {
		return result, true, invalidThemeReference(raw, "repository must be owner/repository")
	}
	owner, repo, _ := strings.Cut(selection.repository, "/")
	repo = strings.TrimSuffix(repo, ".git")
	if !safeRepositoryComponent(owner) || !safeRepositoryComponent(repo) {
		return result, true, invalidThemeReference(raw, "owner and repository must be safe path components")
	}
	if !validThemeTag(selection.reference) {
		return result, true, invalidThemeReference(raw, "an exact tag, latest, or a full commit SHA is required")
	}
	result = themeReference{
		Source: raw, RepositoryURL: selection.origin + "/" + url.PathEscape(owner) + "/" + url.PathEscape(repo),
		Owner: owner, Repository: repo, Reference: selection.reference, Name: repo,
		GitHub: selection.github,
	}
	return result, true, nil
}

func isLocalThemeReference(raw string) bool {
	if raw == "" || strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") || strings.HasPrefix(raw, "~") || (strings.Contains(raw, `\`) && !strings.Contains(raw, "/")) {
		return true
	}
	_, err := os.Stat(raw)
	return err == nil
}

type themeRepositorySelection struct {
	repository string
	reference  string
	origin     string
	github     bool
}

func parseThemeRepositorySelection(raw string) (themeRepositorySelection, bool, error) {
	if strings.Contains(raw, "://") || strings.HasPrefix(raw, "file:") || strings.HasPrefix(raw, "http:") || strings.HasPrefix(raw, "https:") {
		return parseThemeURLSelection(raw)
	}
	var result themeRepositorySelection
	// scp-style Git URLs (with or without an SSH username) retain their
	// transport semantics; their @ is not a revision separator.
	if colon := strings.IndexByte(raw, ':'); colon >= 0 && !strings.Contains(raw[:colon], "/") {
		return result, false, nil
	}
	result.repository, result.reference, _ = strings.Cut(raw, "@")
	if !strings.Contains(raw, "@") && (strings.HasSuffix(raw, ".git") || strings.Count(raw, "/") != 1) {
		return result, false, nil
	}
	result.origin = "https://github.com"
	result.github = true
	return result, true, nil
}

func parseThemeURLSelection(raw string) (themeRepositorySelection, bool, error) {
	var result themeRepositorySelection
	u, err := url.Parse(raw)
	if err != nil {
		if strings.HasPrefix(raw, "http:") || strings.HasPrefix(raw, "https:") {
			return result, true, invalidThemeReference(raw, "malformed HTTP(S) URL")
		}
		return result, false, nil
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return result, false, nil
	}
	if u.Opaque != "" && strings.Contains(raw, "@") {
		return result, true, invalidThemeReference(raw, "repository URL must use HTTP(S) hierarchical syntax")
	}
	if !strings.Contains(u.Path, "@") {
		return result, false, nil
	}
	if u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return result, true, invalidThemeReference(raw, "repository URL must have a host and no credentials, query, or fragment")
	}
	result.repository, result.reference, _ = strings.Cut(strings.TrimPrefix(u.Path, "/"), "@")
	result.origin = u.Scheme + "://" + u.Host
	result.github = strings.EqualFold(u.Hostname(), "github.com")
	return result, true, nil
}

func invalidThemeReference(raw, reason string) error {
	return fmt.Errorf("invalid theme repository reference %q: %s", raw, reason)
}

func safeRepositoryComponent(value string) bool {
	return isSafeThemeDirectoryName(value) && value != ".git"
}

func validThemeTag(tag string) bool {
	if tag == "" || strings.HasPrefix(tag, "/") || strings.HasSuffix(tag, "/") || strings.HasSuffix(tag, ".") || strings.Contains(tag, "..") || strings.Contains(tag, "//") {
		return false
	}
	for _, component := range strings.Split(tag, "/") {
		if strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	for _, char := range tag {
		if unicode.IsSpace(char) || unicode.IsControl(char) || strings.ContainsRune("@~^:?*[\\", char) {
			return false
		}
	}
	return true
}

func resolveThemeReference(ctx context.Context, client *http.Client, ref themeReference) (resolvedThemeSource, error) {
	result := resolvedThemeSource{Source: ref.Source, RepositoryURL: ref.RepositoryURL, Name: ref.Name}
	// Bound the complete metadata operation, including annotated-tag chains,
	// independently of the injected client's own timeout.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	base := strings.TrimSuffix(ref.RepositoryURL, "/"+url.PathEscape(ref.Owner)+"/"+url.PathEscape(ref.Repository))
	base += "/api/v1/repos/" + url.PathEscape(ref.Owner) + "/" + url.PathEscape(ref.Repository)
	if ref.GitHub {
		base = "https://api.github.com/repos/" + url.PathEscape(ref.Owner) + "/" + url.PathEscape(ref.Repository)
	}
	selection := ref.Reference
	if selection == "latest" {
		var release struct {
			TagName    string `json:"tag_name"`
			Draft      bool   `json:"draft"`
			Prerelease bool   `json:"prerelease"`
		}
		if err := themeMetadata(ctx, client, base+"/releases/latest", &release); err != nil {
			return result, fmt.Errorf("resolve latest published theme release: %w", err)
		}
		if !validThemeTag(release.TagName) || release.Draft || release.Prerelease {
			return result, fmt.Errorf("latest theme release is not a published non-prerelease with a valid tag")
		}
		selection = release.TagName
		result.ResolvedTag = selection
	} else if !themeCommitSHA.MatchString(selection) {
		result.ResolvedTag = selection
	}

	var (
		commit string
		err    error
	)
	switch {
	case result.ResolvedTag == "":
		commit, err = resolveThemeCommit(ctx, client, base, selection)
	case ref.GitHub:
		commit, err = resolveGitHubThemeTag(ctx, client, base, selection)
	default:
		commit, err = resolveForgejoThemeTag(ctx, client, base, selection)
	}
	if err != nil {
		return result, err
	}
	if !themeCommitSHA.MatchString(commit) {
		return result, fmt.Errorf("theme provider returned an invalid full commit SHA")
	}
	result.ResolvedCommit = strings.ToLower(commit)
	result.ArchiveURL = base + "/archive/" + result.ResolvedCommit + ".tar.gz"
	if ref.GitHub {
		result.ArchiveURL = "https://codeload.github.com/" + url.PathEscape(ref.Owner) + "/" + url.PathEscape(ref.Repository) + "/tar.gz/" + result.ResolvedCommit
	}
	return result, nil
}

func resolveThemeCommit(ctx context.Context, client *http.Client, base, selection string) (string, error) {
	var object struct {
		SHA string `json:"sha"`
	}
	if err := themeMetadata(ctx, client, base+"/git/commits/"+selection, &object); err != nil {
		return "", fmt.Errorf("resolve theme commit %q: %w", selection, err)
	}
	if !strings.EqualFold(object.SHA, selection) {
		return "", fmt.Errorf("theme commit response does not match requested full SHA")
	}
	return object.SHA, nil
}

func resolveGitHubThemeTag(ctx context.Context, client *http.Client, base, selection string) (string, error) {
	var gitRef struct {
		Ref    string         `json:"ref"`
		Object themeGitObject `json:"object"`
	}
	if err := themeMetadata(ctx, client, base+"/git/ref/tags/"+url.PathEscape(selection), &gitRef); err != nil {
		return "", fmt.Errorf("resolve exact theme tag %q: %w", selection, err)
	}
	if gitRef.Ref != "refs/tags/"+selection {
		return "", fmt.Errorf("theme tag response is not the requested exact tag %q", selection)
	}
	object := gitRef.Object
	seen := make(map[string]bool)
	for object.Type == "tag" {
		if !themeCommitSHA.MatchString(object.SHA) || seen[object.SHA] || len(seen) >= 16 {
			return "", fmt.Errorf("invalid or cyclic annotated theme tag")
		}
		seen[object.SHA] = true
		var tag struct {
			Object themeGitObject `json:"object"`
		}
		if err := themeMetadata(ctx, client, base+"/git/tags/"+object.SHA, &tag); err != nil {
			return "", fmt.Errorf("resolve annotated theme tag: %w", err)
		}
		object = tag.Object
	}
	if object.Type != "commit" {
		return "", fmt.Errorf("theme tag does not resolve to a commit")
	}
	return object.SHA, nil
}

func resolveForgejoThemeTag(ctx context.Context, client *http.Client, base, selection string) (string, error) {
	// Forgejo's exact-tag endpoint returns the peeled commit for both
	// lightweight and annotated tags, not the tag object's own id.
	var tag struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := themeMetadata(ctx, client, base+"/tags/"+url.PathEscape(selection), &tag); err != nil {
		return "", fmt.Errorf("resolve exact theme tag %q: %w", selection, err)
	}
	if tag.Name != selection {
		return "", fmt.Errorf("theme tag response is not the requested exact tag %q", selection)
	}
	return tag.Commit.SHA, nil
}

type themeGitObject struct {
	Type string `json:"type"`
	SHA  string `json:"sha"`
}

func themeMetadata(ctx context.Context, client *http.Client, source string, value any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request theme metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("theme metadata %s: expected HTTP 200, got %s", source, resp.Status)
	}
	// Metadata must be small and contain exactly one complete JSON value.
	body, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return fmt.Errorf("read theme metadata: %w", err)
	}
	if len(body) > 1<<20 {
		return fmt.Errorf("theme metadata exceeds 1 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("decode theme metadata: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("theme metadata must contain a single JSON value")
	}
	return nil
}
