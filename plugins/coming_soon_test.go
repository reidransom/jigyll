package plugins

import (
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/jigyll/pages"
	"github.com/reidransom/liquid"
	"github.com/stretchr/testify/require"
)

// comingSoonMockSite tracks RemoveRoute calls for assertions.
type comingSoonMockSite struct {
	cfg           *config.Config
	pagesVal      []Page
	removedRoutes []string
	hasLayout     map[string]bool
}

func (s *comingSoonMockSite) AddHTMLPage(string, string, pages.FrontMatter) {}
func (s *comingSoonMockSite) RemoveRoute(url string)                        { s.removedRoutes = append(s.removedRoutes, url) }
func (s *comingSoonMockSite) Config() *config.Config                        { return s.cfg }
func (s *comingSoonMockSite) TemplateEngine() *liquid.Engine                { return nil }
func (s *comingSoonMockSite) Pages() []Page                                 { return s.pagesVal }
func (s *comingSoonMockSite) Posts() []Page                                 { return nil }
func (s *comingSoonMockSite) HasLayout(name string) bool                    { return s.hasLayout[name] }
func (s *comingSoonMockSite) HasRoute(string) bool                           { return false }

func newComingSoonSite(enabled bool, hasComingSoonLayout bool, pgs []Page) *comingSoonMockSite {
	cfg := &config.Config{}
	if enabled {
		_ = config.Unmarshal([]byte("coming_soon: true\ntitle: Test Site"), cfg)
	} else {
		_ = config.Unmarshal([]byte("title: Test Site"), cfg)
	}
	layouts := map[string]bool{}
	if hasComingSoonLayout {
		layouts["coming_soon"] = true
	}
	return &comingSoonMockSite{
		cfg:       cfg,
		pagesVal:  pgs,
		hasLayout: layouts,
	}
}

func TestComingSoon_Enabled_UnpublishesNonHomePages(t *testing.T) {
	homePage := &mockPage{fm: pages.FrontMatter{"layout": "default"}, urlVal: "/"}
	blogPage := &mockPage{fm: pages.FrontMatter{"layout": "default"}, urlVal: "/blog/"}
	aboutPage := &mockPage{fm: pages.FrontMatter{"layout": "page"}, urlVal: "/about/"}

	site := newComingSoonSite(true, false, []Page{homePage, blogPage, aboutPage})

	p := &comingSoonPlugin{}
	require.NoError(t, p.AfterInitSite(site))
	require.NoError(t, p.PostReadSite(site))

	// Non-home pages should be unpublished
	require.Equal(t, false, blogPage.fm["published"])
	require.Equal(t, false, aboutPage.fm["published"])

	// Routes should be removed
	require.Contains(t, site.removedRoutes, "/blog/")
	require.Contains(t, site.removedRoutes, "/about/")

	// Homepage should NOT be unpublished
	_, hasPublished := homePage.fm["published"]
	require.False(t, hasPublished)
}

func TestComingSoon_Enabled_FallbackHTML(t *testing.T) {
	homePage := &mockPage{fm: pages.FrontMatter{"layout": "default"}, urlVal: "/"}

	site := newComingSoonSite(true, false, []Page{homePage})

	p := &comingSoonPlugin{}
	require.NoError(t, p.AfterInitSite(site))
	require.NoError(t, p.PostReadSite(site))

	// Layout should be cleared for built-in fallback
	require.Equal(t, "", homePage.fm["layout"])

	// Content should be set to built-in HTML
	require.Contains(t, homePage.contentVal, "Coming Soon")
	require.Contains(t, homePage.contentVal, "Test Site")
	require.Contains(t, homePage.contentVal, "noindex")
}

func TestComingSoon_Enabled_UserLayout(t *testing.T) {
	homePage := &mockPage{fm: pages.FrontMatter{"layout": "default"}, urlVal: "/"}

	site := newComingSoonSite(true, true, []Page{homePage})

	p := &comingSoonPlugin{}
	require.NoError(t, p.AfterInitSite(site))
	require.NoError(t, p.PostReadSite(site))

	// Layout should be set to coming_soon
	require.Equal(t, "coming_soon", homePage.fm["layout"])

	// Content should NOT be overwritten
	require.Empty(t, homePage.contentVal)
}

func TestComingSoon_Disabled_NoChanges(t *testing.T) {
	homePage := &mockPage{fm: pages.FrontMatter{"layout": "default"}, urlVal: "/"}
	blogPage := &mockPage{fm: pages.FrontMatter{"layout": "default"}, urlVal: "/blog/"}

	site := newComingSoonSite(false, false, []Page{homePage, blogPage})

	p := &comingSoonPlugin{}
	require.NoError(t, p.AfterInitSite(site))
	require.NoError(t, p.PostReadSite(site))

	// Nothing should change
	require.Equal(t, "default", homePage.fm["layout"])
	require.Equal(t, "default", blogPage.fm["layout"])
	require.Empty(t, site.removedRoutes)
}

func TestComingSoon_ModifyPluginList_Enabled(t *testing.T) {
	site := newComingSoonSite(true, false, nil)

	p := &comingSoonPlugin{}
	require.NoError(t, p.AfterInitSite(site))

	names := p.ModifyPluginList([]string{
		"jekyll-coming-soon", "jekyll-feed", "jekyll-sitemap", "jekyll-seo-tag",
	})

	require.Contains(t, names, "jekyll-coming-soon")
	require.Contains(t, names, "jekyll-seo-tag")
	require.NotContains(t, names, "jekyll-feed")
	require.NotContains(t, names, "jekyll-sitemap")
}

func TestComingSoon_ModifyPluginList_Disabled(t *testing.T) {
	site := newComingSoonSite(false, false, nil)

	p := &comingSoonPlugin{}
	require.NoError(t, p.AfterInitSite(site))

	names := p.ModifyPluginList([]string{
		"jekyll-coming-soon", "jekyll-feed", "jekyll-sitemap",
	})

	require.Equal(t, []string{"jekyll-coming-soon", "jekyll-feed", "jekyll-sitemap"}, names)
}
