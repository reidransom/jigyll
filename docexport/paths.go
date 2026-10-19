package docexport

import (
	"fmt"
	"path"
	"strings"
)

// ValidateOutputPaths rejects duplicate files and file/directory ancestor
// collisions before a documentation-enabled build replaces completed output.
func ValidateOutputPaths(paths []string) error {
	files := make(map[string]struct{}, len(paths))
	for _, value := range paths {
		name := path.Clean("/" + strings.TrimPrefix(value, "/"))
		if _, exists := files[name]; exists {
			return fmt.Errorf("documentation output paths collide at %s", name)
		}
		files[name] = struct{}{}
	}
	for name := range files {
		for parent := path.Dir(name); parent != "/" && parent != "."; parent = path.Dir(parent) {
			if _, exists := files[parent]; exists {
				return fmt.Errorf("documentation output file %s conflicts with descendant %s", parent, name)
			}
		}
	}
	return nil
}
