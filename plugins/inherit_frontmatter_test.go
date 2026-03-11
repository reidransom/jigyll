package plugins

import (
	"io"
	"testing"
	"time"

	"github.com/osteele/gojekyll/config"
	"github.com/osteele/gojekyll/pages"
	"github.com/osteele/liquid"
	"github.com/stretchr/testify/require"
)

type mockSite struct {
	cfg   *config.Config
	posts []Page
}

func (s *mockSite) AddHTMLPage(url string, tpl string, fm pages.FrontMatter)     {}
func (s *mockSite) RemoveRoute(url string)                                        {}
func (s *mockSite) Config() *config.Config                                        { return s.cfg }
func (s *mockSite) TemplateEngine() *liquid.Engine                                { return nil }
func (s *mockSite) Pages() []Page                                                 { return nil }
func (s *mockSite) Posts() []Page                                                 { return s.posts }
func (s *mockSite) HasLayout(string) bool                                         { return false }

type mockPage struct {
	fm   pages.FrontMatter
	date time.Time
}

func (p *mockPage) URL() string                         { return "" }
func (p *mockPage) Source() string                      { return "" }
func (p *mockPage) OutputExt() string                   { return "" }
func (p *mockPage) Published() bool                     { return true }
func (p *mockPage) IsStatic() bool                      { return false }
func (p *mockPage) Write(w io.Writer) error             { return nil }
func (p *mockPage) Reload() error                       { return nil }
func (p *mockPage) Render() error                       { return nil }
func (p *mockPage) SetContent(string)                   {}
func (p *mockPage) FrontMatter() pages.FrontMatter      { return p.fm }
func (p *mockPage) PostDate() time.Time                 { return p.date }
func (p *mockPage) IsPost() bool                        { return true }
func (p *mockPage) Categories() []string                { return nil }
func (p *mockPage) Tags() []string                      { return nil }

func TestInheritFrontmatterPlugin(t *testing.T) {
	testDate := time.Date(2025, 11, 16, 0, 0, 0, 0, time.UTC)
	
	// Create base English post
	enPost := &mockPage{
		fm: pages.FrontMatter{
			"lang":           "en",
			"title":          "Hello World",
			"layout":         "post",
			"author":         "Reid Ransom",
			"author_url":     "https://x.com/reidransom",
			"featured_image": "/assets/hello.jpg",
			"tags":           []string{"jekyll", "i18n"},
			"reading_time":   "5 min",
		},
		date: testDate,
	}
	
	// Create Spanish translated post (without inherited fields)
	esPost := &mockPage{
		fm: pages.FrontMatter{
			"lang":  "es",
			"title": "¡Hola Mundo!",
		},
		date: testDate,
	}
	
	// Create site with config
	cfg := &config.Config{}
	site := &mockSite{
		cfg:   cfg,
		posts: []Page{enPost, esPost},
	}
	
	// Run the plugin
	plugin := inheritFrontmatterPlugin{}
	err := plugin.PostReadSite(site)
	require.NoError(t, err)
	
	// Verify that ALL fields were copied (except lang and title)
	require.Equal(t, "post", esPost.fm["layout"])
	require.Equal(t, "Reid Ransom", esPost.fm["author"])
	require.Equal(t, "https://x.com/reidransom", esPost.fm["author_url"])
	require.Equal(t, "/assets/hello.jpg", esPost.fm["featured_image"])
	require.Equal(t, []string{"jekyll", "i18n"}, esPost.fm["tags"])
	require.Equal(t, "5 min", esPost.fm["reading_time"])
	
	// Verify that title was NOT copied (already existed)
	require.Equal(t, "¡Hola Mundo!", esPost.fm["title"])
	
	// Verify that lang was NOT copied (excluded by default)
	require.Equal(t, "es", esPost.fm["lang"])
}

func TestInheritFrontmatterPlugin_WithOverride(t *testing.T) {
	testDate := time.Date(2025, 11, 16, 0, 0, 0, 0, time.UTC)
	
	// Create base English post
	enPost := &mockPage{
		fm: pages.FrontMatter{
			"lang":   "en",
			"author": "Reid Ransom",
			"tags":   []string{"jekyll", "i18n"},
		},
		date: testDate,
	}
	
	// Create Spanish post with override for author
	esPost := &mockPage{
		fm: pages.FrontMatter{
			"lang":   "es",
			"author": "Juan Pérez", // Override
		},
		date: testDate,
	}
	
	// Create site
	cfg := &config.Config{}
	site := &mockSite{
		cfg:   cfg,
		posts: []Page{enPost, esPost},
	}
	
	// Run the plugin
	plugin := inheritFrontmatterPlugin{}
	err := plugin.PostReadSite(site)
	require.NoError(t, err)
	
	// Verify that author was NOT overwritten (explicit override)
	require.Equal(t, "Juan Pérez", esPost.fm["author"])
	
	// Verify that tags were copied
	require.Equal(t, []string{"jekyll", "i18n"}, esPost.fm["tags"])
}

func TestInheritFrontmatterPlugin_CustomDefaultLang(t *testing.T) {
	testDate := time.Date(2025, 11, 16, 0, 0, 0, 0, time.UTC)
	
	// Create base French post
	frPost := &mockPage{
		fm: pages.FrontMatter{
			"lang":   "fr",
			"author": "Jean Dupont",
		},
		date: testDate,
	}
	
	// Create English translated post
	enPost := &mockPage{
		fm: pages.FrontMatter{
			"lang": "en",
		},
		date: testDate,
	}
	
	// Create site with custom default_lang
	cfg := &config.Config{}
	err := config.Unmarshal([]byte("default_lang: fr"), cfg)
	require.NoError(t, err)
	
	site := &mockSite{
		cfg:   cfg,
		posts: []Page{frPost, enPost},
	}
	
	// Run the plugin
	plugin := inheritFrontmatterPlugin{}
	err = plugin.PostReadSite(site)
	require.NoError(t, err)
	
	// Verify that author was copied from French to English
	require.Equal(t, "Jean Dupont", enPost.fm["author"])
}

func TestInheritFrontmatterPlugin_ExplicitFields(t *testing.T) {
	testDate := time.Date(2025, 11, 16, 0, 0, 0, 0, time.UTC)
	
	// Create base English post
	enPost := &mockPage{
		fm: pages.FrontMatter{
			"lang":   "en",
			"author": "Reid Ransom",
			"layout": "post",
			"tags":   []string{"jekyll"},
		},
		date: testDate,
	}
	
	// Create Spanish post
	esPost := &mockPage{
		fm: pages.FrontMatter{
			"lang": "es",
		},
		date: testDate,
	}
	
	// Create site with explicit field list (only inherit author and tags)
	cfg := &config.Config{}
	var err error
	err = config.Unmarshal([]byte(`inherit_frontmatter:
  fields:
    - author
    - tags
`), cfg)
	require.NoError(t, err)
	
	site := &mockSite{
		cfg:   cfg,
		posts: []Page{enPost, esPost},
	}
	
	// Run the plugin
	plugin := inheritFrontmatterPlugin{}
	err = plugin.PostReadSite(site)
	require.NoError(t, err)
	
	// Verify that only specified fields were copied
	require.Equal(t, "Reid Ransom", esPost.fm["author"])
	require.Equal(t, []string{"jekyll"}, esPost.fm["tags"])
	
	// Verify that layout was NOT copied (not in explicit list)
	require.Nil(t, esPost.fm["layout"])
}

func TestInheritFrontmatterPlugin_ExcludeFields(t *testing.T) {
	testDate := time.Date(2025, 11, 16, 0, 0, 0, 0, time.UTC)
	
	// Create base English post
	enPost := &mockPage{
		fm: pages.FrontMatter{
			"lang":   "en",
			"author": "Reid Ransom",
			"layout": "post",
			"tags":   []string{"jekyll"},
		},
		date: testDate,
	}
	
	// Create Spanish post
	esPost := &mockPage{
		fm: pages.FrontMatter{
			"lang": "es",
		},
		date: testDate,
	}
	
	// Create site with exclude list (inherit all except layout)
	cfg := &config.Config{}
	var err error
	err = config.Unmarshal([]byte(`
inherit_frontmatter:
  exclude:
    - layout
`), cfg)
	require.NoError(t, err)
	
	site := &mockSite{
		cfg:   cfg,
		posts: []Page{enPost, esPost},
	}
	
	// Run the plugin
	plugin := inheritFrontmatterPlugin{}
	err = plugin.PostReadSite(site)
	require.NoError(t, err)
	
	// Verify that author and tags were copied
	require.Equal(t, "Reid Ransom", esPost.fm["author"])
	require.Equal(t, []string{"jekyll"}, esPost.fm["tags"])
	
	// Verify that layout was NOT copied (in exclude list)
	require.Nil(t, esPost.fm["layout"])
	
	// Verify that lang was NOT copied (always excluded)
	require.Equal(t, "es", esPost.fm["lang"])
}
