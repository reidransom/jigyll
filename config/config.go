package config

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/reidransom/jigyll/utils"
	yaml "gopkg.in/yaml.v2"
)

// Config is the Jekyll site configuration, typically read from _config.yml.
// See https://jekyllrb.com/docs/configuration/#default-configuration
type Config struct {
	// Where things are:
	Source       string
	Destination  string
	LayoutsDir   string                            `yaml:"layouts_dir"`
	DataDir      string                            `yaml:"data_dir"`
	IncludesDir  string                            `yaml:"includes_dir"`
	Collections  map[string]map[string]interface{} `yaml:"-"`
	Theme        string
	RemoteTheme  string              `yaml:"remote_theme"`
	Localization *LocalizationConfig `yaml:"localization"`

	// Handling Reading
	Include     []string
	Exclude     []string
	KeepFiles   []string `yaml:"keep_files"`
	MarkdownExt string   `yaml:"markdown_ext"`

	// Filtering Content
	Drafts      bool `yaml:"show_drafts"`
	Future      bool
	Unpublished bool

	// Plugins
	Plugins []string

	// Conversion
	ExcerptSeparator string `yaml:"excerpt_separator"`
	Incremental      bool
	Sass             struct {
		Dir string `yaml:"sass_dir"`
		// TODO Style string // compressed
	}

	// Liquid rendering
	Liquid struct {
		StrictFilters bool `yaml:"strict_filters"`
	}

	// Serving
	Host        string
	Port        int
	AbsoluteURL string `yaml:"url"`
	BaseURL     string

	// Outputting
	Permalink    string
	Paginate     int    `yaml:"paginate"`
	PaginatePath string `yaml:"paginate_path"`
	Timezone     string
	Verbose      bool
	Defaults     []struct {
		Scope struct {
			Path string
			Type string
		}
		Values map[string]interface{}
	}

	// CLI-only
	DryRun                  bool     `yaml:"-"`
	ForcePolling            bool     `yaml:"-"`
	Watch                   bool     `yaml:"-"`
	FollowDirectorySymlinks bool     `yaml:"-"`
	ProtectedSourcePaths    []string `yaml:"-"`

	// Meta
	ConfigFile string                 `yaml:"-"`
	m          map[string]interface{} `yaml:"-"` // config file, as map
	ms         yaml.MapSlice          `yaml:"-"` // config file, as MapSlice

	// Plugins
	RequireFrontMatter        bool            `yaml:"-"`
	RequireFrontMatterExclude map[string]bool `yaml:"-"`
}

// FromDirectory updates the config from the config file in
// the directory, if such a file exists.
func (c *Config) FromDirectory(dir string, configFiles string) error {
	// Check JEKYLL_CONFIG environment variable if --config flag not provided
	if configFiles == "" {
		if jekyllConfig := os.Getenv("JEKYLL_CONFIG"); jekyllConfig != "" {
			configFiles = jekyllConfig
		}
	}

	// If explicit config files are specified, use those
	if configFiles != "" {
		return c.loadConfigFiles(dir, configFiles)
	}

	// Default: read _config.yml
	path := filepath.Join(dir, "_config.yml")
	bytes, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		// break
	case err != nil:
		return err
	default:
		if err = Unmarshal(bytes, c); err != nil {
			return utils.WrapPathError(err, path)
		}
		c.ConfigFile = path
	}
	c.Source = dir

	// Override URL from JEKYLL_URL environment variable if set
	if jekyllURL := os.Getenv("JEKYLL_URL"); jekyllURL != "" {
		c.AbsoluteURL = jekyllURL
		c.Set("url", jekyllURL)
	}

	return nil
}

// loadConfigFiles loads one or more config files separated by commas.
// Later files override earlier ones.
func (c *Config) loadConfigFiles(dir string, configFiles string) error {
	// Split by comma and trim whitespace
	files := strings.Split(configFiles, ",")
	for i, f := range files {
		files[i] = strings.TrimSpace(f)
	}

	if len(files) == 0 {
		return nil
	}

	// Track config file names for display
	configFileNames := []string{}

	// Merged YAML data
	var mergedData map[string]interface{}

	// Load and merge config files in order
	for _, configFile := range files {
		if configFile == "" {
			continue
		}

		// Determine full path
		var configPath string
		if filepath.IsAbs(configFile) {
			configPath = configFile
		} else {
			configPath = filepath.Join(dir, configFile)
		}

		// Read config file
		bytes, err := os.ReadFile(configPath)
		if err != nil {
			return utils.WrapPathError(err, configPath)
		}

		// Parse YAML into a map
		var fileData map[string]interface{}
		if err := yaml.Unmarshal(bytes, &fileData); err != nil {
			return utils.WrapPathError(err, configPath)
		}

		// Merge into accumulated data
		if mergedData == nil {
			mergedData = fileData
		} else {
			// Override with values from this file
			for k, v := range fileData {
				mergedData[k] = v
			}
		}

		configFileNames = append(configFileNames, configPath)
	}

	// Convert merged data back to YAML and unmarshal into config
	mergedBytes, err := yaml.Marshal(mergedData)
	if err != nil {
		return err
	}

	if err = Unmarshal(mergedBytes, c); err != nil {
		return err
	}

	// Set the config file display string
	if len(configFileNames) == 1 {
		c.ConfigFile = configFileNames[0]
	} else {
		c.ConfigFile = strings.Join(configFileNames, ", ")
	}
	c.Source = dir

	// Override URL from JEKYLL_URL environment variable if set
	if jekyllURL := os.Getenv("JEKYLL_URL"); jekyllURL != "" {
		c.AbsoluteURL = jekyllURL
		c.Set("url", jekyllURL)
	}

	return nil
}

type configCompat struct {
	Gems []string
}

type collectionsList struct {
	Collections []string
}

type collectionsMap struct {
	Collections map[string]map[string]interface{}
}

// IsConfigPath returns true if its arguments is a site configuration file.
func (c *Config) IsConfigPath(rel string) bool {
	return rel == "_config.yml"
}

// SassDir returns the relative path of the SASS directory.
func (c *Config) SassDir() string {
	return "_sass"
}

// SourceDir returns the source directory as an absolute path.
func (c *Config) SourceDir() string {
	return utils.MustAbs(c.Source)
}

// defaultScope is the anonymous scope struct embedded in Config.Defaults;
// the type alias lets helper functions take a pointer to it.
type defaultScope = struct {
	Path string
	Type string
}

// GetFrontMatterDefaults implements https://jekyllrb.com/docs/configuration/#front-matter-defaults
//
// It mirrors Jekyll's frontmatter_defaults algorithm: a scope matches when
// its type is absent or equal to typename AND its path matches rel; conflicts
// between matching scopes are resolved by precedence (longer sanitized path
// wins, then a type-bearing scope beats a typeless one, then later config
// entries win). typename == "" represents an untyped document (a static file
// outside any collection), which only matches typeless scopes.
func (c *Config) GetFrontMatterDefaults(typename, rel string) map[string]interface{} {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "/")
	var m map[string]interface{}
	var prev *defaultScope
	for i := range c.Defaults {
		entry := &c.Defaults[i]
		if !scopeMatches(&entry.Scope, typename, rel) {
			continue
		}
		if hasPrecedence(prev, &entry.Scope) {
			m = utils.MergeStringMaps(m, entry.Values) // new entry wins
			prev = &entry.Scope
		} else {
			// old winner keeps overriding; MergeStringMaps returns a fresh map,
			// so entry.Values is never mutated.
			m = utils.MergeStringMaps(entry.Values, m)
		}
	}
	return m
}

// scopeMatches reports whether a front-matter-defaults scope applies to a
// document of the given type and rel path, following Jekyll's rules:
// an empty type matches any document (including untyped ones), an empty path
// matches every path, a path containing "*" is treated as a glob, and any
// other path is a raw string-prefix test.
func scopeMatches(scope *defaultScope, typename, rel string) bool {
	if scope.Type != "" && scope.Type != typename {
		return false
	}
	switch {
	case scope.Path == "":
		return true
	case strings.Contains(scope.Path, "*"):
		return globMatches(scope.Path, rel)
	default:
		return strings.HasPrefix(rel, scope.Path)
	}
}

// globMatches reproduces Jekyll's glob-scope behavior without disk I/O:
// Jekyll runs Dir.glob on the scope path and string-prefix-tests the doc path
// against each result, so a glob that matches an ancestor directory of the
// document also applies to the document. We approximate this by matching the
// pattern against the rel path and each of its ancestor directory paths.
func globMatches(pattern, rel string) bool {
	p := rel
	for {
		if ok, err := doublestar.Match(pattern, p); err == nil && ok {
			return true
		}
		next := path.Dir(p)
		if next == p || next == "." {
			return false
		}
		p = next
	}
}

// hasPrecedence mirrors Jekyll's FrontMatterDefaults#has_precedence?: the
// scope with the longer sanitized path wins regardless of config order; on
// equal length a type-bearing scope beats a typeless one; on a final tie the
// later (new) entry wins.
func hasPrecedence(old, new *defaultScope) bool {
	if old == nil {
		return true
	}
	newLen, oldLen := len(sanitizeScopePath(new.Path)), len(sanitizeScopePath(old.Path))
	if newLen != oldLen {
		return newLen > oldLen
	}
	switch {
	case new.Type != "" && old.Type == "":
		return true
	case new.Type == "" && old.Type != "":
		return false
	default:
		return true // equal ties: later entry wins
	}
}

// sanitizeScopePath strips a single leading "/" from a scope path, matching
// Jekyll's sanitized_path helper.
func sanitizeScopePath(p string) string {
	return strings.TrimPrefix(p, "/")
}

// RequiresFrontMatter returns a bool indicating whether the file requires front matter in order to recognize as a page.
func (c *Config) RequiresFrontMatter(rel string) bool {
	switch {
	case c.RequireFrontMatter:
		return true
	case !c.IsMarkdown(rel):
		return true
	case utils.StringArrayContains(c.Include, rel):
		return false
	case c.RequireFrontMatterExclude[strings.ToUpper(utils.TrimExt(filepath.Base(rel)))]:
		return true
	default:
		return false
	}
}

// Unmarshal updates site from a YAML configuration file.
func Unmarshal(bytes []byte, c *Config) error {
	var (
		compat configCompat
		cList  collectionsList
	)
	if err := yaml.Unmarshal(bytes, &c); err != nil {
		return err
	}
	if err := yaml.Unmarshal(bytes, &c.ms); err != nil {
		return err
	}
	if err := yaml.Unmarshal(bytes, &c.m); err != nil {
		return err
	}
	if err := yaml.Unmarshal(bytes, &cList); err == nil {
		if len(c.Collections) == 0 {
			c.Collections = make(map[string]map[string]interface{})
		}
		for _, name := range cList.Collections {
			c.Collections[name] = map[string]interface{}{}
		}
	}
	cMap := collectionsMap{c.Collections}
	if err := yaml.Unmarshal(bytes, &cMap); err == nil {
		c.Collections = cMap.Collections
	}
	if err := yaml.Unmarshal(bytes, &compat); err != nil {
		return err
	}
	if len(c.Plugins) == 0 {
		c.Plugins = compat.Gems
	}
	if err := c.Localization.Validate(); err != nil {
		return err
	}
	return nil
}

// Variables returns the configuration as a Liquid variable map.
func (c *Config) Variables() map[string]interface{} {
	m := map[string]interface{}{}
	for _, item := range c.ms {
		if s, ok := item.Key.(string); ok {
			m[s] = item.Value
		}
	}
	return m
}

// Set sets a value in the Liquid variable map.
// This does not update the corresponding value in the Config struct.
func (c *Config) Set(key string, val interface{}) {
	c.m[key] = val
	for i := range c.ms {
		if c.ms[i].Key == key {
			c.ms[i].Value = val
			return
		}
	}
	c.ms = append(c.ms, yaml.MapItem{Key: key, Value: val})
}

// Map returns the config indexed by key, if it's a map.
func (c *Config) Map(key string) (map[string]interface{}, bool) {
	switch m := c.m[key].(type) {
	case map[string]interface{}:
		return m, true
	case map[interface{}]interface{}:
		// yaml.v2 unmarshals nested mappings with interface{} keys
		result := make(map[string]interface{}, len(m))
		for k, v := range m {
			if ks, ok := k.(string); ok {
				result[ks] = v
			}
		}
		return result, true
	}
	return nil, false
}

// String returns the config indexed by key, if it's a string.
func (c *Config) String(key string) (string, bool) {
	if m, ok := c.m[key]; ok {
		if m, ok := m.(string); ok {
			return m, ok
		}
	}
	return "", false
}
