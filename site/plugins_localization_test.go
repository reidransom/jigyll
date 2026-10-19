package site

import (
	"io"
	"testing"
	"time"

	"github.com/reidransom/jigyll/pages"
	"github.com/stretchr/testify/require"
)

func TestAddPageLocalizesGeneratorRoute(t *testing.T) {
	s := &Site{localePrefix: "de", Routes: make(map[string]Document)}
	s.AddPage(&generatedPluginPage{PageEmbed: pages.PageEmbed{Path: "/generated/"}})

	page, found := s.Routes["/de/generated/"]
	require.True(t, found)
	require.Equal(t, "/de/generated/", page.URL())
	require.Len(t, s.Pages(), 1)
	require.Equal(t, "/de/generated/", s.Pages()[0].URL())
}

type generatedPluginPage struct {
	pages.PageEmbed
}

func (p *generatedPluginPage) Render() error                   { return nil }
func (p *generatedPluginPage) Article() (pages.Article, error) { return pages.Article{}, nil }
func (p *generatedPluginPage) SetContent(string)               {}
func (p *generatedPluginPage) FrontMatter() pages.FrontMatter  { return nil }
func (p *generatedPluginPage) PostDate() time.Time             { return time.Time{} }
func (p *generatedPluginPage) IsPost() bool                    { return false }
func (p *generatedPluginPage) Categories() []string            { return nil }
func (p *generatedPluginPage) Write(io.Writer) error           { return nil }
func (p *generatedPluginPage) Tags() []string                  { return nil }
