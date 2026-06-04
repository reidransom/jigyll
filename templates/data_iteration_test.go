package templates

import (
	"path/filepath"
	"testing"

	"github.com/reidransom/gojekyll/config"
	"github.com/reidransom/gojekyll/site"
	"github.com/stretchr/testify/require"
)

func TestDataIteration(t *testing.T) {
	// Create a test site with the example data
	cfg := config.Default()
	cfg.Source = filepath.Join("..", "example")
	
	s, err := site.FromDirectory(cfg.Source, config.Flags{})
	require.NoError(t, err)
	err = s.Read()
	require.NoError(t, err)
	
	// Template that iterates over list data
	template := `{% for item in site.data.list_data %}{{ item }}
{% endfor %}`
	
	// Render the template
	renderer := s.RendererManager()
	bindings := map[string]interface{}{"site": s}
	rendered, err := renderer.RenderTemplate([]byte(template), bindings, "test.liquid", 1)
	require.NoError(t, err)
	
	// Check that all list items are rendered
	expected := "data file value\nanother value\nlast value\n"
	require.Equal(t, expected, string(rendered))
}

func TestDataIterationWithIndex(t *testing.T) {
	// Create a test site with the example data
	cfg := config.Default()
	cfg.Source = filepath.Join("..", "example")
	
	s, err := site.FromDirectory(cfg.Source, config.Flags{})
	require.NoError(t, err)
	err = s.Read()
	require.NoError(t, err)
	
	// Template that iterates over list data with forloop index
	template := `{% for item in site.data.list_data %}{{ forloop.index }}: {{ item }}
{% endfor %}`
	
	// Render the template
	renderer := s.RendererManager()
	bindings := map[string]interface{}{"site": s}
	rendered, err := renderer.RenderTemplate([]byte(template), bindings, "test.liquid", 1)
	require.NoError(t, err)
	
	// Check that all list items are rendered with indices
	expected := "1: data file value\n2: another value\n3: last value\n"
	require.Equal(t, expected, string(rendered))
}

func TestDataIterationEmpty(t *testing.T) {
	// Create a test site
	cfg := config.Default()
	cfg.Source = filepath.Join("..", "commands", "testdata", "site")
	
	s, err := site.FromDirectory(cfg.Source, config.Flags{})
	require.NoError(t, err)
	err = s.Read()
	require.NoError(t, err)
	
	// Template that iterates over non-existent data
	template := `{% for item in site.data.nonexistent %}{{ item }}{% endfor %}`
	
	// Render the template
	renderer := s.RendererManager()
	bindings := map[string]interface{}{"site": s}
	rendered, err := renderer.RenderTemplate([]byte(template), bindings, "test.liquid", 1)
	require.NoError(t, err)
	
	// Should render empty string when data doesn't exist
	require.Equal(t, "", string(rendered))
}

func TestDataIterationWithHTML(t *testing.T) {
	// Create a test site with the example data  
	cfg := config.Default()
	cfg.Source = filepath.Join("..", "example")
	
	s, err := site.FromDirectory(cfg.Source, config.Flags{})
	require.NoError(t, err)
	err = s.Read()
	require.NoError(t, err)
	
	// Template that creates an HTML list from data
	template := `<ul>
{% for item in site.data.list_data %}<li>{{ item }}</li>
{% endfor %}</ul>`
	
	// Render the template
	renderer := s.RendererManager()
	bindings := map[string]interface{}{"site": s}
	rendered, err := renderer.RenderTemplate([]byte(template), bindings, "test.liquid", 1)
	require.NoError(t, err)
	
	// Check HTML output
	expected := "<ul>\n<li>data file value</li>\n<li>another value</li>\n<li>last value</li>\n</ul>"
	require.Equal(t, expected, string(rendered))
}