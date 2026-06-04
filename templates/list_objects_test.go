package templates

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/jigyll/site"
	"github.com/stretchr/testify/require"
)

func TestListObjectsIteration(t *testing.T) {
	// Create a test site with the example data
	cfg := config.Default()
	cfg.Source = filepath.Join("..", "example")
	
	s, err := site.FromDirectory(cfg.Source, config.Flags{})
	require.NoError(t, err)
	err = s.Read()
	require.NoError(t, err)
	
	// Template that iterates over list_objects and outputs titles
	template := `{% for item in site.data.list_objects %}{{ item.title }}{% endfor %}`
	
	// Render the template
	renderer := s.RendererManager()
	bindings := map[string]interface{}{"site": s}
	rendered, err := renderer.RenderTemplate([]byte(template), bindings, "test.liquid", 1)
	require.NoError(t, err)
	
	// Check that all object titles are rendered
	output := string(rendered)
	require.Contains(t, output, "Target - Grove Central, Miami")
	require.Contains(t, output, "Northwell Ambulatory Hospital")
	require.Contains(t, output, "Village+")
	require.Contains(t, output, "Premium Retail Buildout")
	require.Contains(t, output, "Corporate Office Renovation")
	require.Contains(t, output, "Restaurant Chain Expansion")
}

func TestListObjectsIterationWithProperties(t *testing.T) {
	// Create a test site with the example data
	cfg := config.Default()
	cfg.Source = filepath.Join("..", "example")
	
	s, err := site.FromDirectory(cfg.Source, config.Flags{})
	require.NoError(t, err)
	err = s.Read()
	require.NoError(t, err)
	
	// Template that iterates over list_objects and outputs multiple properties
	template := `{% for item in site.data.list_objects %}
<h3>{{ item.title }}</h3>
<p>{{ item.description }}</p>
<img src="{{ item.image }}" alt="{{ item.title }}">
{% endfor %}`
	
	// Render the template
	renderer := s.RendererManager()
	bindings := map[string]interface{}{"site": s}
	rendered, err := renderer.RenderTemplate([]byte(template), bindings, "test.liquid", 1)
	require.NoError(t, err)
	
	// Check that all properties are rendered correctly
	output := string(rendered)
	require.Contains(t, output, "<h3>Target - Grove Central, Miami</h3>")
	require.Contains(t, output, "Modern retail space transformation")
	require.Contains(t, output, `src="/assets/imgs/target-sm.jpg"`)
	require.Contains(t, output, `alt="Target - Grove Central, Miami"`)
}

func TestListObjectsCount(t *testing.T) {
	// Create a test site with the example data
	cfg := config.Default()
	cfg.Source = filepath.Join("..", "example")
	
	s, err := site.FromDirectory(cfg.Source, config.Flags{})
	require.NoError(t, err)
	err = s.Read()
	require.NoError(t, err)
	
	// Template that counts the objects
	template := `{% for item in site.data.list_objects %}{{ forloop.index }}{% unless forloop.last %},{% endunless %}{% endfor %}`
	
	// Render the template
	renderer := s.RendererManager()
	bindings := map[string]interface{}{"site": s}
	rendered, err := renderer.RenderTemplate([]byte(template), bindings, "test.liquid", 1)
	require.NoError(t, err)
	
	// Should have 6 items based on the YAML file
	expected := "1,2,3,4,5,6"
	require.Equal(t, expected, string(rendered))
}

func TestListObjectsWithConditional(t *testing.T) {
	// Create a test site with the example data
	cfg := config.Default()
	cfg.Source = filepath.Join("..", "example")
	
	s, err := site.FromDirectory(cfg.Source, config.Flags{})
	require.NoError(t, err)
	err = s.Read()
	require.NoError(t, err)
	
	// Template that filters objects containing "Target" in title
	template := `{% for item in site.data.list_objects %}{% if item.title contains "Target" %}{{ item.title }}{% endif %}{% endfor %}`
	
	// Render the template
	renderer := s.RendererManager()
	bindings := map[string]interface{}{"site": s}
	rendered, err := renderer.RenderTemplate([]byte(template), bindings, "test.liquid", 1)
	require.NoError(t, err)
	
	// Should only contain the one Target entry
	output := string(rendered)
	targetCount := strings.Count(output, "Target")
	require.Equal(t, 1, targetCount)
	require.Contains(t, output, "Target - Grove Central, Miami")
	require.NotContains(t, output, "Premium Retail Buildout")
	require.NotContains(t, output, "Northwell")
}