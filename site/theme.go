package site

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func (s *Site) findTheme() error {
	if s.cfg.Theme == "" {
		return nil
	}

	// First, try to find theme in _theme folder
	themeDir := filepath.Join(s.AbsDir(), "_theme", s.cfg.Theme)
	if _, err := os.Stat(themeDir); err == nil {
		s.themeDir = themeDir
		return nil
	}

	// Fallback to using bundle
	exe, err := exec.LookPath("bundle")
	if err != nil {
		return fmt.Errorf("the %s theme could not be found in _theme folder and bundle is not available", s.cfg.Theme)
	}
	cmd := exec.Command(exe, "show", s.cfg.Theme)
	cmd.Dir = s.AbsDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("the %s theme could not be found", s.cfg.Theme)
		}
		return err
	}
	s.themeDir = string(bytes.TrimSpace(out))
	return nil
}

func (s *Site) readThemeAssets() error {
	if s.themeDir == "" {
		return nil
	}
	err := s.readFiles(filepath.Join(s.themeDir, "assets"), s.themeDir)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
