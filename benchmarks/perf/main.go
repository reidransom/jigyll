// Command perf supports the native comparison harness; it is not a benchmarked engine.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/tdewolff/minify"
	"github.com/tdewolff/minify/css"
	"golang.org/x/net/html"
	"gopkg.in/yaml.v2"
)

const marker = "native-comparison-workspace-v1\n"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "perf:", err)
		os.Exit(1)
	}
}

func run(a []string) error {
	if len(a) == 4 && a[0] == "validate" {
		return validate(a[1], a[2], a[3])
	}
	if len(a) == 3 && a[0] == "copy" {
		return copySource(a[1], a[2])
	}
	if len(a) == 2 && a[0] == "reset" {
		return reset(a[1])
	}
	if len(a) == 3 && a[0] == "manifest" {
		return verifyManifest(a[1], a[2])
	}
	if len(a) == 3 && a[0] == "compare" {
		return compare(a[1], a[2])
	}
	if len(a) == 3 && a[0] == "capture" {
		return captureMetadata(a[1], a[2])
	}
	if len(a) == 2 && a[0] == "report" {
		return writeReport(a[1])
	}
	return fmt.Errorf("invalid helper invocation")
}

func validate(runs, warmup, profile string) error {
	for i, s := range []string{runs, warmup} {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 || (i == 0 && n == 0) || strings.Trim(s, "0123456789") != "" {
			return fmt.Errorf("runs must be positive and warmup nonnegative decimal integers")
		}
	}
	switch profile {
	case "default", "single", "no-cache", "single-no-cache", "all":
		return nil
	}
	return fmt.Errorf("invalid profile %q; choose default, single, no-cache, single-no-cache, all", profile)
}

// Only the fixed output/cache children of an owned, physical workspace can be
// reset. Neither sources nor arbitrary caller-supplied paths are cleanup targets.
func owned(root string) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	physical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return err
	}
	if physical != abs || !strings.HasPrefix(filepath.Base(abs), "run-") {
		return fmt.Errorf("unsafe workspace %s", root)
	}
	b, err := os.ReadFile(filepath.Join(root, ".perf-owned"))
	if err != nil {
		return err
	}
	if string(b) != marker {
		return fmt.Errorf("workspace ownership marker mismatch")
	}
	return nil
}

func reset(root string) error {
	if err := owned(root); err != nil {
		return err
	}
	// Jekyll restricts disk caches to its source tree. This is the private
	// copied source, never the original; do not traverse a replaced parent.
	if info, err := os.Lstat(filepath.Join(root, "source")); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink source parent")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, name := range []string{"output", "cache", "source/.jekyll-cache"} {
		p := filepath.Join(root, name)
		if info, err := os.Lstat(p); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink cleanup target %s", p)
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	return nil
}

type siteConfig struct {
	Destination string   `yaml:"destination"`
	CacheDir    string   `yaml:"cache_dir"`
	Exclude     []string `yaml:"exclude"`
	Sass        struct {
		CacheLocation string `yaml:"cache_location"`
	} `yaml:"sass"`
}

func relativePath(p string) bool {
	return p != "" && p != "." && !filepath.IsAbs(p) && filepath.Clean(p) == p && p != ".." && !strings.HasPrefix(p, ".."+string(filepath.Separator))
}

func copySource(source, root string) error {
	if err := owned(root); err != nil {
		return err
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	physical, err := filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	source = physical
	if rel, _ := filepath.Rel(source, root); rel == "." || relativePath(rel) {
		return fmt.Errorf("workspace must not be inside site source")
	}
	excludes, err := sourceExcludes(source)
	if err != nil {
		return err
	}
	dest := filepath.Join(root, "source")
	if err := os.Mkdir(dest, 0700); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if sourcePathExcluded(rel, d.Name(), excludes) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		return copySourceEntry(path, filepath.Join(dest, rel), rel, d)
	})
}

func sourceExcludes(source string) ([]string, error) {
	var cfg siteConfig
	config, err := os.ReadFile(filepath.Join(source, "_config.yml"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err := yaml.Unmarshal(config, &cfg); err != nil {
		return nil, fmt.Errorf("source config: %w", err)
	}
	excludes := []string{".git", ".hg", ".svn", ".bundle", "vendor", "node_modules", "_site", ".jekyll-cache", ".sass-cache", ".jekyll-metadata", ".scratch"}
	for _, p := range append(cfg.Exclude, cfg.Destination, cfg.CacheDir, cfg.Sass.CacheLocation) {
		if p == "" {
			continue
		}
		if filepath.IsAbs(p) {
			p, err = filepath.Rel(source, p)
			if err != nil {
				return nil, err
			}
		}
		p = filepath.Clean(p)
		if p == "." {
			return nil, fmt.Errorf("configured output/cache/exclude refers to source root")
		}
		if relativePath(p) {
			excludes = append(excludes, p)
		}
	}
	return excludes, nil
}

func sourcePathExcluded(rel, name string, excludes []string) bool {
	// These are build inputs even when excluded from published output.
	if rel == "Gemfile" || rel == "Gemfile.lock" || rel == "_config.yml" {
		return false
	}
	for _, x := range excludes {
		// Basename defaults apply at every depth; configured paths apply to subtrees.
		match, _ := filepath.Match(x, rel)
		if match || rel == x || strings.HasPrefix(rel, x+string(filepath.Separator)) || (!strings.ContainsRune(x, filepath.Separator) && name == x) {
			return true
		}
	}
	return false
}

func copySourceEntry(path, target, rel string, d fs.DirEntry) error {
	if d.Type()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsupported source symlink %s (not followed; materialize inputs explicitly)", rel)
	}
	info, err := d.Info()
	if err != nil {
		return err
	}
	if d.IsDir() {
		return os.Mkdir(target, 0700)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsupported special source file %s", rel)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(target, b, info.Mode().Perm()&0755); err != nil {
		return err
	}
	return os.Chtimes(target, info.ModTime(), info.ModTime())
}

type manifest struct {
	Schema      int                                           `json:"schema_version"`
	Revision    string                                        `json:"generator_revision"`
	Parameters  struct{ Posts, Pages, Includes, Layouts int } `json:"parameters"`
	Future      bool                                          `json:"future"`
	Timezone    string                                        `json:"timezone"`
	Incremental bool                                          `json:"incremental"`
	Inputs      []struct{ Path, SHA256 string }               `json:"inputs"`
	Outputs     []struct {
		Kind, Source, URL, Path, Title string
		Text, Links                    []string
	} `json:"outputs"`
	Checks map[string][]string `json:"checks"`
}

func verifyManifest(source, output string) error {
	b, err := os.ReadFile(filepath.Join(source, "_benchmark", "manifest.json"))
	if err != nil {
		return err
	}
	var m manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	if err := validateManifestPolicy(&m); err != nil {
		return err
	}
	inputs, err := verifyManifestInputs(source, &m)
	if err != nil {
		return err
	}
	counts, err := verifyManifestOutputs(output, &m, inputs)
	if err != nil {
		return err
	}
	fmt.Printf("Verified generated manifest: %d posts, %d pages, index; every URL/title/body/layout/include/navigation check passed\n", counts["post"], counts["page"])
	return nil
}

func validateManifestPolicy(m *manifest) error {
	if m.Schema != 1 || m.Revision != "2" || !m.Future || m.Incremental || m.Timezone != "Etc/UTC" || m.Parameters.Posts < 0 || m.Parameters.Pages < 0 || m.Parameters.Includes != 10 || m.Parameters.Layouts != 4 {
		return fmt.Errorf("unsupported or incomplete generated manifest policy/parameters; generate a new fixture with the current generator (for the default, explicitly move the old fixture aside then run just perf-setup)")
	}
	return nil
}

func verifyManifestInputs(source string, m *manifest) (map[string]bool, error) {
	inputs := map[string]bool{}
	for _, in := range m.Inputs {
		if !relativePath(in.Path) || inputs[in.Path] {
			return nil, fmt.Errorf("invalid/duplicate manifest input %q", in.Path)
		}
		inputs[in.Path] = true
		b, err := os.ReadFile(filepath.Join(source, in.Path))
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != in.SHA256 {
			return nil, fmt.Errorf("manifest input hash mismatch: %s", in.Path)
		}
	}
	actual, err := files(source)
	if err != nil {
		return nil, err
	}
	for _, p := range actual {
		if p != "_benchmark/manifest.json" && !strings.HasPrefix(p, ".jekyll-cache/") && !inputs[p] {
			return nil, fmt.Errorf("source absent from manifest: %s", p)
		}
	}
	return inputs, nil
}

func verifyManifestOutputs(output string, m *manifest, inputs map[string]bool) (map[string]int, error) {
	counts := map[string]int{}
	paths := map[string]bool{}
	urls := map[string]bool{}
	for i := range m.Outputs {
		out := &m.Outputs[i]
		if !validManifestOutput(m, i, inputs, paths, urls) {
			return nil, fmt.Errorf("incomplete/duplicate manifest output %q", out.Path)
		}
		paths[out.Path] = true
		urls[out.URL] = true
		counts[out.Kind]++
		if err := verifyManifestRenderedOutput(output, m, i); err != nil {
			return nil, err
		}
	}
	if counts["post"] != m.Parameters.Posts || counts["page"] != m.Parameters.Pages || counts["index"] != 1 || len(m.Outputs) != m.Parameters.Posts+m.Parameters.Pages+1 {
		return nil, fmt.Errorf("manifest output counts do not match parameters")
	}
	return counts, nil
}

func validManifestOutput(m *manifest, index int, inputs, paths, urls map[string]bool) bool {
	out := &m.Outputs[index]
	return relativePath(out.Path) && !paths[out.Path] && !urls[out.URL] && inputs[out.Source] && out.Title != "" && len(out.Text) != 0 && len(out.Links) != 0 && len(m.Checks[out.Kind]) != 0 && strings.HasPrefix(out.URL, "/")
}

func verifyManifestRenderedOutput(output string, m *manifest, index int) error {
	out := &m.Outputs[index]
	b, err := os.ReadFile(filepath.Join(output, out.Path))
	if err != nil {
		return fmt.Errorf("missing expected URL %s: %w", out.URL, err)
	}
	text, links, title, err := htmlContent(b)
	if err != nil {
		return err
	}
	if !strings.Contains(title, out.Title) {
		return fmt.Errorf("%s: missing rendered title %q", out.URL, out.Title)
	}
	for _, fragment := range out.Text {
		if !strings.Contains(text, normalize(fragment)) {
			return fmt.Errorf("%s: missing rendered body %q", out.URL, fragment)
		}
	}
	for _, link := range out.Links {
		if !contains(links, link) {
			return fmt.Errorf("%s: missing navigation href %q", out.URL, link)
		}
	}
	tokens := htmlTokens(b)
	for _, fragment := range m.Checks[out.Kind] {
		if !hasFragment(tokens, htmlTokens([]byte(fragment))) {
			return fmt.Errorf("%s: broken layout/include: missing %q", out.URL, fragment)
		}
	}
	return nil
}

// Renderers may add attributes (e.g. Rouge's inline-code classes). Require the
// manifest's complete tag/text sequence and every specified attribute, without
// demanding byte-identical HTML or accepting plain text in place of markup.
func htmlTokens(b []byte) []html.Token {
	z := html.NewTokenizer(bytes.NewReader(b))
	var tokens []html.Token
	for z.Next() != html.ErrorToken {
		token := z.Token()
		if token.Type == html.TextToken {
			token.Data = normalize(token.Data)
			if token.Data == "" {
				continue
			}
		}
		tokens = append(tokens, token)
	}
	return tokens
}

func hasFragment(tokens, want []html.Token) bool {
	if len(want) == 0 {
		return false
	}
	for i := 0; i+len(want) <= len(tokens); i++ {
		match := true
		for j, w := range want {
			got := tokens[i+j]
			if got.Type != w.Type || got.Data != w.Data {
				match = false
				break
			}
			for _, attr := range w.Attr {
				found := false
				for _, a := range got.Attr {
					if a == attr {
						found = true
						break
					}
				}
				if !found {
					match = false
					break
				}
			}
			if !match {
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func contains(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}
func normalize(s string) string { return strings.Join(strings.Fields(s), " ") }

// Compare visible HTML text and link targets, not Markdown renderer formatting.
// CSS is normalized with the existing minifier; other assets must agree exactly.
func htmlContent(b []byte) (string, []string, string, error) {
	doc, err := html.Parse(bytes.NewReader(b))
	if err != nil {
		return "", nil, "", err
	}
	var text, title strings.Builder
	links := []string{}
	var walk func(*html.Node, bool, bool)
	walk = func(n *html.Node, hidden, inTitle bool) {
		if n.Type == html.ElementNode {
			if n.Data == "script" || n.Data == "style" {
				hidden = true
			}
			if n.Data == "title" {
				inTitle = true
			}
			if n.Data == "a" {
				for _, a := range n.Attr {
					if a.Key == "href" {
						links = append(links, a.Val)
					}
				}
			}
		}
		if n.Type == html.TextNode && !hidden {
			text.WriteString(n.Data)
			text.WriteByte(' ')
			if inTitle {
				title.WriteString(n.Data)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, hidden, inTitle)
		}
	}
	walk(doc, false, false)
	sort.Strings(links)
	return normalize(text.String()), links, normalize(title.String()), nil
}

func files(root string) ([]string, error) {
	out := []string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("unsupported nonregular output/input %s", p)
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	return out, err
}

func compare(left, right string) error {
	a, err := files(left)
	if err != nil {
		return err
	}
	b, err := files(right)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(a, b) {
		return fmt.Errorf("unsupported output path differences: only Jigyll=%v; only Jekyll=%v", difference(a, b), difference(b, a))
	}
	htmlCount := 0
	cssNormalizer := minify.New()
	cssNormalizer.AddFunc("text/css", css.Minify)
	for _, p := range a {
		x, err := os.ReadFile(filepath.Join(left, p))
		if err != nil {
			return err
		}
		y, err := os.ReadFile(filepath.Join(right, p))
		if err != nil {
			return err
		}
		switch {
		case strings.EqualFold(filepath.Ext(p), ".html"):
			htmlCount++
			if err := compareRenderedHTML(p, x, y); err != nil {
				return err
			}
		case strings.EqualFold(filepath.Ext(p), ".css"):
			if err := compareRenderedCSS(p, x, y, cssNormalizer); err != nil {
				return err
			}
		default:
			if !bytes.Equal(x, y) {
				return fmt.Errorf("unsupported static/generated asset difference: %s", p)
			}
		}
	}
	if htmlCount == 0 {
		return fmt.Errorf("no rendered HTML pages; cannot establish comparable workload")
	}
	fmt.Printf("Verified real site: identical %d output paths, normalized text and hrefs in all %d HTML files, normalized CSS, byte-equal other assets (not byte-identical HTML/CSS)\n", len(a), htmlCount)
	return nil
}

func compareRenderedHTML(path string, x, y []byte) error {
	tx, lx, _, err := htmlContent(x)
	if err != nil {
		return err
	}
	ty, ly, _, err := htmlContent(y)
	if err != nil {
		return err
	}
	if tx != ty || !reflect.DeepEqual(lx, ly) {
		return fmt.Errorf("unsupported rendered content/navigation difference: %s\nJigyll text: %.500s\nJekyll text: %.500s", path, tx, ty)
	}
	return nil
}

func compareRenderedCSS(path string, x, y []byte, normalizer *minify.M) error {
	nx, err := normalizer.Bytes("text/css", x)
	if err != nil {
		return fmt.Errorf("%s: normalize Jigyll CSS: %w", path, err)
	}
	ny, err := normalizer.Bytes("text/css", y)
	if err != nil {
		return fmt.Errorf("%s: normalize Jekyll CSS: %w", path, err)
	}
	if !bytes.Equal(nx, ny) {
		return fmt.Errorf("unsupported rendered CSS difference: %s", path)
	}
	return nil
}

func difference(a, b []string) []string {
	out := []string{}
	for _, s := range a {
		if !contains(b, s) {
			out = append(out, s)
		}
	}
	return out
}
