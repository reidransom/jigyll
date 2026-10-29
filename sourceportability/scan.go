package sourceportability

import (
	"bufio"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	attributeRE     = regexp.MustCompile(`(?i)\b(href|src|srcset|action|poster|content|data-src)\s*=\s*("(?:\{\{[^}]*\}\}|[^"])*"|'(?:\{\{[^}]*\}\}|[^'])*'|[^\s>]+)`)
	markdownRE      = regexp.MustCompile(`(!?)\[[^\]]*\]\(\s*(\{\{.*?\}\}|<?[^)\s]+>?)`)
	referenceRE     = regexp.MustCompile(`^\s*\[[^]]+\]:\s*(\{\{.*?\}\}|<?\S+>?)`)
	cssRE           = regexp.MustCompile(`(?i)\burl\(\s*("(?:\{\{[^}]*\}\}|[^"])*"|'(?:\{\{[^}]*\}\}|[^'])*'|[^)]*)\s*\)`)
	yamlKeyRE       = regexp.MustCompile(`^(\s*)(?:-\s*)?([\w-]+)\s*:\s*(.*)$`)
	jsStringRE      = regexp.MustCompile(`['"]((?:https?://|/|\{\{)[^'"\n]+)['"]`)
	liquidRE        = regexp.MustCompile(`\{\{\s*(.*?)\s*\}\}`)
	filterRE        = regexp.MustCompile(`\|\s*(relative_url|absolute_url)\s*$`)
	literalHostRE   = regexp.MustCompile(`(?i)['"]((?:https?:)?//[^'"]+)['"]`)
	liquidLiteralRE = regexp.MustCompile(`['"]([^'"]+)['"]`)
	webSchemeRE     = regexp.MustCompile(`(?i)^https?://`)
	imageExtRE      = regexp.MustCompile(`(?i)\.(?:png|jpe?g|gif|webp|avif|svg|ico)(?:[?#]|$)`)
)

// Finding names the source line and rule, not a rendered-output defect.
type Finding struct {
	File, Rule, Message string
	Line                int
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: %s: %s", f.File, f.Line, f.Rule, f.Message)
}

// Scan walks one site and returns findings in stable file/line order.
func Scan(o Options) ([]Finding, error) {
	info, err := os.Stat(o.Site)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("site root is not a directory: %s", o.Site)
	}
	s, err := loadSettings(o)
	if err != nil {
		return nil, err
	}
	var findings []Finding
	err = filepath.WalkDir(o.Site, func(name string, entry fs.DirEntry, walkErr error) error {
		return visitScanEntry(o, s, name, entry, walkErr, &findings)
	})
	return findings, err
}

func visitScanEntry(o Options, s settings, name string, entry fs.DirEntry, walkErr error, findings *[]Finding) error {
	if walkErr != nil {
		return walkErr
	}
	rel, err := filepath.Rel(o.Site, name)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)
	if entry.IsDir() {
		if shouldSkipDir(rel, entry.Name()) {
			return filepath.SkipDir
		}
		return nil
	}
	if isSkippedScanFile(o, name, rel, entry) {
		return nil
	}
	ext := strings.ToLower(filepath.Ext(name))
	if !isSupportedScanExt(ext) {
		return nil
	}
	return scanFile(name, rel, ext, s, findings)
}

func shouldSkipDir(rel, name string) bool {
	return rel != "." && (strings.HasPrefix(name, ".") || name == "_site" || name == "node_modules" || name == "vendor")
}

func isSkippedScanFile(o Options, name, rel string, entry fs.DirEntry) bool {
	return !entry.Type().IsRegular() || rel == "_config.yml" || sameFile(name, o.SharedPolicy) || sameFile(name, o.SitePolicy) || sameFile(name, o.SiteConfig)
}

func isSupportedScanExt(ext string) bool {
	switch ext {
	case ".html", ".htm", ".liquid", ".md", ".markdown", ".yml", ".yaml", ".css", ".scss", ".sass", ".js", ".mjs":
		return true
	}
	return false
}

type fileScanState struct {
	line       int
	yamlMode   bool
	yamlIndent int
	yamlURL    bool
	yamlImage  bool
	fenced     bool
}

func scanFile(name, rel, ext string, s settings, findings *[]Finding) error {
	file, err := os.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	state := fileScanState{yamlMode: ext == ".yml" || ext == ".yaml", yamlIndent: -1}
	for scanner.Scan() {
		state.line++
		scanLine(scanner.Text(), rel, ext, &state, s, findings)
	}
	return scanner.Err()
}

func scanLine(text, rel, ext string, state *fileScanState, s settings, findings *[]Finding) {
	if updateFenceAndFrontMatter(text, ext, state) {
		return
	}
	add := func(raw, context string, image bool) {
		*findings = append(*findings, s.check(rel, state.line, raw, context, image)...)
	}
	if state.yamlMode {
		scanYAMLLine(text, state, add)
	}
	if ext == ".js" || ext == ".mjs" {
		if s.enabled(JavaScriptURL, rel, state.line) {
			for _, m := range jsStringRE.FindAllStringSubmatch(text, -1) {
				add(m[1], "javascript", false)
			}
		}
		return
	}
	scanMarkupLine(text, ext, add)
}

func updateFenceAndFrontMatter(text, ext string, state *fileScanState) bool {
	if isMarkdownExt(ext) && (strings.HasPrefix(text, "```") || strings.HasPrefix(text, "~~~")) {
		state.fenced = !state.fenced
		return true
	}
	if state.fenced {
		return true
	}
	if state.line == 1 && text == "---" && (isMarkdownExt(ext) || ext == ".html") {
		state.yamlMode = true
		return true
	}
	if state.yamlMode && text == "---" {
		state.yamlMode = false
		return true
	}
	return false
}

func isMarkdownExt(ext string) bool {
	return ext == ".md" || ext == ".markdown"
}

func scanYAMLLine(text string, state *fileScanState, add func(string, string, bool)) {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "#") {
		return
	}
	if m := yamlKeyRE.FindStringSubmatch(text); m != nil {
		key := strings.ToLower(m[2])
		state.yamlURL = urlKey(key)
		state.yamlImage = imageKey(key)
		state.yamlIndent = len(m[1])
		if state.yamlURL && strings.TrimSpace(m[3]) != "" {
			add(strings.SplitN(m[3], " #", 2)[0], key, state.yamlImage)
		}
		return
	}
	indent := len(text) - len(strings.TrimLeft(text, " "))
	if state.yamlURL && indent > state.yamlIndent && strings.HasPrefix(trimmed, "- ") {
		add(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")), "yaml", state.yamlImage)
	} else if trimmed != "" && indent <= state.yamlIndent {
		state.yamlURL = false
	}
}

func scanMarkupLine(text, ext string, add func(string, string, bool)) {
	for _, m := range attributeRE.FindAllStringSubmatch(text, -1) {
		scanAttributeMatch(text, m[1], m[2], add)
	}
	for _, m := range cssRE.FindAllStringSubmatch(text, -1) {
		add(m[1], "css", imageExtRE.MatchString(trimQuotes(m[1])))
	}
	if isMarkdownExt(ext) {
		for _, m := range markdownRE.FindAllStringSubmatch(text, -1) {
			add(m[2], "markdown", m[1] == "!")
		}
		if m := referenceRE.FindStringSubmatch(text); m != nil {
			add(m[1], "markdown", false)
		}
	}
}

func scanAttributeMatch(text, rawKey, value string, add func(string, string, bool)) {
	key := strings.ToLower(rawKey)
	lowerText := strings.ToLower(text)
	if key == "content" && !strings.Contains(lowerText, "og:") && !strings.Contains(lowerText, "twitter:image") {
		return
	}
	if key == "srcset" {
		for _, part := range strings.Split(trimQuotes(value), ",") {
			if fields := strings.Fields(part); len(fields) != 0 {
				add(fields[0], key, true)
			}
		}
		return
	}
	add(value, key, (key == "src" && strings.Contains(lowerText, "<img")) || key == "poster")
}

func sameFile(a, b string) bool {
	if b == "" {
		return false
	}
	x, _ := filepath.Abs(a)
	y, _ := filepath.Abs(b)
	return x == y
}

func urlKey(key string) bool {
	switch key {
	case "url", "href", "src", "image", "images", "logo", "icon", "avatar", "poster", "action", "canonical", "redirect_to", "links", "assets", "background", "thumbnail":
		return true
	}
	return strings.HasSuffix(key, "_url") || strings.HasSuffix(key, "_image") || strings.HasSuffix(key, "_link")
}

func imageKey(key string) bool {
	return key == "image" || key == "images" || key == "logo" || key == "icon" || key == "avatar" || key == "poster" || key == "thumbnail" || strings.HasSuffix(key, "_image")
}

func trimQuotes(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 2 && (raw[0] == '\'' && raw[len(raw)-1] == '\'' || raw[0] == '"' && raw[len(raw)-1] == '"') {
		return raw[1 : len(raw)-1]
	}
	return raw
}

func webHost(raw string) (string, bool) {
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	if !webSchemeRE.MatchString(raw) {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return "", false
	}
	return strings.ToLower(u.Hostname()), true
}

func (s settings) check(file string, line int, raw, context string, image bool) []Finding {
	raw = strings.Trim(trimQuotes(raw), "<>")
	if isIgnoredTarget(raw) {
		return nil
	}
	var out []Finding
	add := func(rule, message string) {
		if context == "javascript" && rule != ExternalHost {
			rule = JavaScriptURL
			message = "heuristic JavaScript URL: " + message
		}
		if s.enabled(rule, file, line) {
			out = append(out, Finding{file, rule, message, line})
		}
	}
	if strings.Contains(raw, "{{") {
		s.checkLiquidExpression(file, line, raw, context, image, add, &out)
		return out
	}
	if strings.Contains(raw, "{%") {
		add(LiquidURL, "opaque Liquid URL; use a URL filter on the complete expression")
		return out
	}
	if host, external := webHost(raw); external {
		s.checkExternalHost(host, add)
		return out
	}
	if strings.Contains(raw, ":") || strings.HasPrefix(raw, "?") {
		return out
	}
	s.checkLocalPath(file, raw, image, add)
	return out
}

func isIgnoredTarget(raw string) bool {
	return raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "data:") || strings.HasPrefix(raw, "mailto:") || strings.HasPrefix(raw, "tel:")
}

func (s settings) checkLiquidExpression(file string, line int, raw, context string, image bool, add func(string, string), out *[]Finding) {
	m := liquidRE.FindStringSubmatch(raw)
	if m == nil || !filterRE.MatchString(m[1]) || strings.TrimSpace(strings.Replace(raw, m[0], "", 1)) != "" {
		add(LiquidURL, "opaque or unfiltered Liquid URL; use relative_url or absolute_url on the complete URL")
	}
	if m == nil {
		return
	}
	if literal := literalHostRE.FindStringSubmatch(m[1]); literal != nil {
		*out = append(*out, s.check(file, line, literal[1], context, false)...)
	}
	if image {
		if literal := liquidLiteralRE.FindStringSubmatch(m[1]); literal != nil {
			for _, finding := range s.check(file, line, literal[1], context, true) {
				if finding.Rule == ImagePlacement {
					*out = append(*out, finding)
				}
			}
		}
	}
}

func (s settings) checkExternalHost(host string, add func(string, string)) {
	if host == s.origin && s.origin != "" {
		add(FirstPartyOrigin, "hard-coded site origin; use a filtered site-relative URL")
	} else if !s.allowed[host] && host != s.origin {
		add(ExternalHost, "external host "+host+" is not in allowed_hosts")
	}
}

func (s settings) checkLocalPath(file, raw string, image bool, add func(string, string)) {
	if strings.HasPrefix(raw, "/") && s.base != "" {
		add(RootRelative, "root-relative local URL bypasses baseurl; use relative_url")
	}
	if !image || !imageExtRE.MatchString(raw) {
		return
	}
	p := strings.SplitN(strings.SplitN(raw, "?", 2)[0], "#", 2)[0]
	if strings.HasPrefix(p, "/") {
		p = strings.TrimPrefix(p, s.base+"/")
	} else {
		p = path.Join(path.Dir(file), p)
	}
	if !strings.HasPrefix(strings.TrimPrefix(path.Clean(p), "/"), "assets/") {
		add(ImagePlacement, "local image is outside assets/")
	}
}
