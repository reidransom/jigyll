package commands

import (
	"strings"

	"github.com/reidransom/jigyll/pages"
	"github.com/reidransom/jigyll/site"
	"github.com/reidransom/jigyll/utils"
)

// If path starts with /, it's a URL path. Else it's a file path relative
// to the site source directory.
func pageFromPathOrRoute(s *site.Site, path string) (pages.Document, error) {
	if path == "" {
		path = "/"
	}
	switch {
	case strings.HasPrefix(path, "/"):
		page, found := s.URLPage(path)
		if !found {
			return nil, utils.NewPathError("render", path, "the site does not include a file with this URL path")
		}
		return page, nil
	default:
		page, found := s.FilePathPage(path)
		if !found {
			return nil, utils.NewPathError("render", path, "no such file")
		}
		return page, nil
	}
}
