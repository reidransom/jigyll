package site

import (
	"sort"

	"github.com/reidransom/jigyll/collection"
)

// render renders the site's pages.
func (s *Site) render() error {
	var errs []error
	for _, c := range s.sortedCollections() {
		if err := c.Render(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, c := range s.nonCollectionPages {
		if err := c.Render(); err != nil {
			errs = append(errs, err)
		}
	}
	return combineErrors(errs)
}

func (s *Site) ensureRendered() error {
	s.renderOnce.Do(func() {
		s.renderError = s.initializeRenderers()
		if s.renderError != nil {
			return
		}
		s.renderError = s.render()
	})
	return s.renderError
}

// returns a slice of collections, sorted by name but with _posts last.
func (s *Site) sortedCollections() []*collection.Collection {
	cols := make([]*collection.Collection, len(s.Collections))
	copy(cols, s.Collections)
	sort.Slice(cols, postsCollectionLast(cols).Less)
	return cols
}

type postsCollectionLast []*collection.Collection

func (d postsCollectionLast) Less(i, j int) bool {
	array := []*collection.Collection(d)
	a, b := array[i], array[j]
	switch {
	case a.IsPostsCollection():
		return false
	case b.IsPostsCollection():
		return true
	default:
		return a.Name < b.Name
	}
}
