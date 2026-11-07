package site

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/reidransom/jigyll/utils"
)

// watchInventory maps physical directories onto every logical site path by
// which they are reachable. Parent watches observe mount targets themselves.
type watchInventory struct {
	dirs    map[string][]string
	targets map[string]bool
}

func (s *Site) scanWatchInventory() (*watchInventory, error) {
	inv := &watchInventory{dirs: make(map[string][]string), targets: make(map[string]bool)}
	inv.addTarget(s.SourceDir())
	opts := utils.SourceWalkOptions{FollowDirectorySymlinks: s.cfg.FollowDirectorySymlinks, Protected: append([]string{s.DestDir()}, s.cfg.ProtectedSourcePaths...), Prune: func(path string, info os.FileInfo) bool {
		rel := utils.MustRel(s.SourceDir(), path)
		return rel != "." && s.excludeWatchPath(rel)
	}}
	err := utils.WalkSource(s.SourceDir(), opts, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		physical, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		physical, err = filepath.Abs(physical)
		if err != nil {
			return err
		}
		rel := utils.MustRel(s.SourceDir(), path)
		inv.dirs[physical] = append(inv.dirs[physical], rel)
		linkInfo, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if linkInfo.Mode()&os.ModeSymlink != 0 {
			return inv.addLinkTargets(path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return inv, nil
}

func (inv *watchInventory) addTarget(path string) {
	path, err := filepath.Abs(path)
	if err != nil {
		return
	}
	for {
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		if resolved, err := filepath.EvalSymlinks(parent); err == nil {
			inv.targets[filepath.Join(resolved, filepath.Base(path))] = true
		}
		path = parent
	}
}

func (inv *watchInventory) addLinkTargets(path string) error {
	link, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	inv.addTarget(link)
	// Follow each link in the chain and watch the parent of each target.
	for {
		info, err := os.Lstat(link)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			inv.addTarget(link)
			break
		}
		target, err := os.Readlink(link)
		if err != nil {
			return err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(link), target)
		}
		link = filepath.Clean(target)
		inv.addTarget(link)
	}
	return nil
}

// Theme templates are read outside ordinary page discovery. Keep their existing
// source tree observable even when the plugin is enabled after watching starts.
func (s *Site) excludeWatchPath(rel string) bool {
	if s.watchProtected(filepath.Join(s.SourceDir(), rel)) {
		return true
	}
	if s.themeDir != "" {
		themeRel, err := filepath.Rel(s.themeDir, filepath.Join(s.SourceDir(), rel))
		if err == nil && themeRel != ".." && !filepath.IsAbs(themeRel) && !strings.HasPrefix(themeRel, ".."+string(filepath.Separator)) {
			return !s.fileAffectsBuild(rel)
		}
	}
	return s.Exclude(rel)
}

func (s *Site) watchProtected(path string) bool {
	canonical, err := utils.CanonicalPath(path)
	if err != nil {
		return false
	}
	for _, destination := range append([]string{s.DestDir()}, s.cfg.ProtectedSourcePaths...) {
		root, err := utils.CanonicalPath(destination)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(root, canonical)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			return true
		}
	}
	if s.cfg.Enabled() {
		destination, err := utils.CanonicalPath(s.DestDir())
		if err == nil {
			relative, err := filepath.Rel(filepath.Dir(destination), canonical)
			if err == nil {
				first := strings.SplitN(relative, string(filepath.Separator), 2)[0]
				// Stages and the promotion backup are siblings of the exact
				// configured destination, not authored hidden/include trees.
				if first == filepath.Base(destination)+".jigyll-localized-backup" || strings.HasPrefix(first, localizedStagePrefix(destination)) {
					return true
				}
			}
		}
	}
	return false
}

func (inv *watchInventory) registrations() map[string]bool {
	dirs := make(map[string]bool, len(inv.dirs))
	for dir := range inv.dirs {
		dirs[dir] = true
	}
	for target := range inv.targets {
		parent, err := filepath.EvalSymlinks(filepath.Dir(target))
		if err == nil {
			if abs, e := filepath.Abs(parent); e == nil {
				dirs[abs] = true
			}
		}
	}
	return dirs
}

func (inv *watchInventory) event(eventPath string) ([]string, bool) {
	path, err := filepath.Abs(eventPath)
	if err != nil {
		return nil, false
	}
	path = filepath.Clean(path)
	topology := inv.targets[path]
	var paths []string
	for _, rel := range inv.dirs[filepath.Dir(path)] {
		if rel == "." {
			paths = append(paths, filepath.Base(path))
		} else {
			paths = append(paths, filepath.Join(rel, filepath.Base(path)))
		}
	}
	return paths, topology
}

func watchLimitError(count int) error {
	return fmt.Errorf("directory count %d exceeds safe limit 500 for fsnotify", count)
}
