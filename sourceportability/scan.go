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
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(o.Site, name)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if rel != "." && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "_site" || entry.Name() == "node_modules" || entry.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() || rel == "_config.yml" || sameFile(name, o.SharedPolicy) || sameFile(name, o.SitePolicy) || sameFile(name, o.SiteConfig) {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".html" && ext != ".htm" && ext != ".liquid" && ext != ".md" && ext != ".markdown" && ext != ".yml" && ext != ".yaml" && ext != ".css" && ext != ".scss" && ext != ".sass" && ext != ".js" && ext != ".mjs" {
			return nil
		}
		return scanFile(name, rel, ext, s, &findings)
	})
	return findings, err
}

func scanFile(name, rel, ext string, s settings, findings *[]Finding) error {
	file, err := os.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	line, yamlMode, yamlIndent, yamlURL, yamlImage, fenced := 0, ext == ".yml" || ext == ".yaml", -1, false, false, false
	for scanner.Scan() {
		line++
		text := scanner.Text()
		if (ext == ".md" || ext == ".markdown") && (strings.HasPrefix(text, "```") || strings.HasPrefix(text, "~~~")) {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if line == 1 && text == "---" && (ext == ".md" || ext == ".markdown" || ext == ".html") {
			yamlMode = true
			continue
		}
		if yamlMode && text == "---" {
			yamlMode = false
			continue
		}
		add := func(raw, context string, image bool) {
			*findings = append(*findings, s.check(rel, line, raw, context, image)...)
		}
		if yamlMode {
			if strings.HasPrefix(strings.TrimSpace(text), "#") {
				continue
			}
			if m := yamlKeyRE.FindStringSubmatch(text); m != nil {
				indent := len(m[1])
				key := strings.ToLower(m[2])
				yamlURL = urlKey(key)
				yamlImage = imageKey(key)
				yamlIndent = indent
				if yamlURL && strings.TrimSpace(m[3]) != "" {
					add(strings.SplitN(m[3], " #", 2)[0], key, yamlImage)
				}
			} else if yamlURL && len(text)-len(strings.TrimLeft(text, " ")) > yamlIndent && strings.HasPrefix(strings.TrimSpace(text), "- ") {
				add(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), "- ")), "yaml", yamlImage)
			} else if strings.TrimSpace(text) != "" && len(text)-len(strings.TrimLeft(text, " ")) <= yamlIndent {
				yamlURL = false
			}
		}
		if ext == ".js" || ext == ".mjs" {
			if s.enabled(JavaScriptURL, rel, line) {
				for _, m := range jsStringRE.FindAllStringSubmatch(text, -1) {
					add(m[1], "javascript", false)
				}
			}
			continue
		}
		for _, m := range attributeRE.FindAllStringSubmatch(text, -1) {
			value := m[2]
			key := strings.ToLower(m[1])
			if key == "content" && !strings.Contains(strings.ToLower(text), "og:") && !strings.Contains(strings.ToLower(text), "twitter:image") {
				continue
			}
			if key == "srcset" {
				for _, part := range strings.Split(trimQuotes(value), ",") {
					if fields := strings.Fields(part); len(fields) != 0 {
						add(fields[0], key, true)
					}
				}
			} else {
				add(value, key, key == "src" && strings.Contains(strings.ToLower(text), "<img") || key == "poster")
			}
		}
		for _, m := range cssRE.FindAllStringSubmatch(text, -1) {
			add(m[1], "css", imageExtRE.MatchString(trimQuotes(m[1])))
		}
		if ext == ".md" || ext == ".markdown" {
			for _, m := range markdownRE.FindAllStringSubmatch(text, -1) {
				add(m[2], "markdown", m[1] == "!")
			}
			if m := referenceRE.FindStringSubmatch(text); m != nil {
				add(m[1], "markdown", false)
			}
		}
	}
	return scanner.Err()
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
	raw = trimQuotes(raw)
	raw = strings.Trim(raw, "<>")
	if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "data:") || strings.HasPrefix(raw, "mailto:") || strings.HasPrefix(raw, "tel:") {
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
		m := liquidRE.FindStringSubmatch(raw)
		if m == nil || !filterRE.MatchString(m[1]) || strings.TrimSpace(strings.Replace(raw, m[0], "", 1)) != "" {
			add(LiquidURL, "opaque or unfiltered Liquid URL; use relative_url or absolute_url on the complete URL")
		}
		if m != nil {
			if literal := literalHostRE.FindStringSubmatch(m[1]); literal != nil {
				out = append(out, s.check(file, line, literal[1], context, false)...)
			}
		}
		if image && m != nil {
			if literal := liquidLiteralRE.FindStringSubmatch(m[1]); literal != nil {
				for _, finding := range s.check(file, line, literal[1], context, true) {
					if finding.Rule == ImagePlacement {
						out = append(out, finding)
					}
				}
			}
		}
		return out
	}
	if strings.Contains(raw, "{%") {
		add(LiquidURL, "opaque Liquid URL; use a URL filter on the complete expression")
		return out
	}
	host, external := webHost(raw)
	if external {
		if host == s.origin && s.origin != "" {
			add(FirstPartyOrigin, "hard-coded site origin; use a filtered site-relative URL")
		} else if !s.allowed[host] && host != s.origin {
			add(ExternalHost, "external host "+host+" is not in allowed_hosts")
		}
		return out
	}
	if strings.Contains(raw, ":") || strings.HasPrefix(raw, "?") {
		return out
	}
	if strings.HasPrefix(raw, "/") && s.base != "" {
		add(RootRelative, "root-relative local URL bypasses baseurl; use relative_url")
	}
	if image && imageExtRE.MatchString(raw) {
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
	return out
}
