package plugins

import "github.com/reidransom/jigyll/docexport"

// ArtifactPreparer is an optional hook for source-derived, non-HTML exports.
// Site calls it after source/plugin pages and documentation inventory are known,
// before rendering. The returned build closure runs once, before publication.
type ArtifactPreparer interface {
	PrepareArtifacts(ArtifactContext) (*ArtifactPlan, error)
}

// ArtifactContext contains actual output pages and optional documentation owners.
// Generated artifacts are deliberately absent from Pages.
type ArtifactContext struct {
	Pages         []Page
	Documentation *DocumentationArtifacts
}

// DocumentationArtifacts describes the independently owned documentation outputs.
type DocumentationArtifacts struct {
	Project  docexport.Project
	Groups   []docexport.Group
	Articles []docexport.Article
	Sources  map[string]string
}

// ArtifactPlan declares the complete output set before any rendering or cleaning.
// Build must return exactly these routes, with immutable complete bytes.
type ArtifactPlan struct {
	Routes  []string
	Capture []Page
	Build   func() (map[string][]byte, error)
}
