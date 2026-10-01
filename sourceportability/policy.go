// Package sourceportability audits site source without rendering it.
package sourceportability

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v2"
)

const (
	FirstPartyOrigin = "first-party-origin"
	RootRelative     = "root-relative"
	LiquidURL        = "liquid-url"
	ExternalHost     = "external-host"
	ImagePlacement   = "image-placement"
	JavaScriptURL    = "javascript-url"
)

var defaults = map[string]bool{
	FirstPartyOrigin: true,
	RootRelative:     true,
	LiquidURL:        true,
	ExternalHost:     false,
	ImagePlacement:   false,
	JavaScriptURL:    false,
}

type override struct {
	File  string          `yaml:"file"`
	Glob  string          `yaml:"glob"`
	Line  int             `yaml:"line"`
	Rules map[string]bool `yaml:"rules"`
}

type policy struct {
	Rules        map[string]bool `yaml:"rules"`
	AllowedHosts []string        `yaml:"allowed_hosts"`
	Files        []override      `yaml:"files"`
	Lines        []override      `yaml:"lines"`
}

// Options describes one site's configuration; empty policy paths mean no policy.
type Options struct {
	Site, SharedPolicy, SitePolicy, SiteConfig string
}

type settings struct {
	shared, site policy
	origin, base string
	allowed      map[string]bool
}

func loadSettings(o Options) (settings, error) {
	var s settings
	for _, item := range []struct {
		path string
		out  *policy
	}{{o.SharedPolicy, &s.shared}, {o.SitePolicy, &s.site}} {
		if item.path == "" {
			continue
		}
		data, err := os.ReadFile(item.path)
		if err != nil {
			return s, err
		}
		if err := yaml.UnmarshalStrict(data, item.out); err != nil {
			return s, fmt.Errorf("%s: %w", item.path, err)
		}
		if err := validatePolicy(*item.out); err != nil {
			return s, fmt.Errorf("%s: %w", item.path, err)
		}
	}
	config := o.SiteConfig
	if config == "" {
		config = filepath.Join(o.Site, "_config.yml")
	}
	data, err := os.ReadFile(config)
	if err != nil && (o.SiteConfig != "" || !os.IsNotExist(err)) {
		return s, err
	}
	var site struct {
		URL     string `yaml:"url"`
		BaseURL string `yaml:"baseurl"`
	}
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &site); err != nil {
			return s, fmt.Errorf("%s: %w", config, err)
		}
	}
	if site.URL != "" {
		origin, ok := webHost(site.URL)
		if !ok {
			return s, fmt.Errorf("%s: url must be an http(s) origin", config)
		}
		s.origin = origin
	}
	s.base = "/" + strings.Trim(strings.TrimSpace(site.BaseURL), "/")
	if s.base == "/" {
		s.base = ""
	}
	hosts := s.shared.AllowedHosts
	if s.site.AllowedHosts != nil {
		hosts = s.site.AllowedHosts
	}
	s.allowed = make(map[string]bool, len(hosts))
	for _, h := range hosts {
		if h == "" || strings.ContainsAny(h, "/: ") {
			return s, fmt.Errorf("allowed_hosts must contain hostnames without schemes or ports: %q", h)
		}
		s.allowed[strings.ToLower(h)] = true
	}
	return s, nil
}

func validatePolicy(p policy) error {
	check := func(rules map[string]bool) error {
		for id := range rules {
			if _, ok := defaults[id]; !ok {
				return fmt.Errorf("unknown rule %q", id)
			}
		}
		return nil
	}
	if err := check(p.Rules); err != nil {
		return err
	}
	for _, f := range p.Files {
		if f.Glob == "" || f.File != "" || f.Line != 0 {
			return fmt.Errorf("files entries require glob and rules only")
		}
		if _, err := doublestar.Match(f.Glob, "example.md"); err != nil {
			return err
		}
		if err := check(f.Rules); err != nil {
			return err
		}
	}
	for _, l := range p.Lines {
		if l.File == "" || l.Line < 1 || l.Glob != "" {
			return fmt.Errorf("lines entries require file, positive line, and rules only")
		}
		if err := check(l.Rules); err != nil {
			return err
		}
	}
	return nil
}

func (s settings) enabled(id, file string, line int) bool {
	value := defaults[id]
	if v, ok := s.shared.Rules[id]; ok {
		value = v
	}
	if v, ok := s.site.Rules[id]; ok {
		value = v
	}
	for _, p := range []policy{s.shared, s.site} {
		for _, f := range p.Files {
			match, _ := doublestar.Match(f.Glob, file)
			if match {
				if v, ok := f.Rules[id]; ok {
					value = v
				}
			}
		}
	}
	for _, p := range []policy{s.shared, s.site} {
		for _, l := range p.Lines {
			if l.File == file && l.Line == line {
				if v, ok := l.Rules[id]; ok {
					value = v
				}
			}
		}
	}
	return value
}
