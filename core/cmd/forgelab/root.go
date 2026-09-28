package main

import (
	"os"
	"path/filepath"
)

// findRepoRoot walks up from the working directory to the directory holding
// both ROADMAP.md and core/go.mod. It returns "" when not inside the
// repository.
func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if fileExists(filepath.Join(dir, "ROADMAP.md")) && fileExists(filepath.Join(dir, "core", "go.mod")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// progressFile is where learning progress is stored: FORGELAB_PROGRESS, or
// .forgelab/progress.json at the repository root.
func progressFile(root string) string {
	if p := os.Getenv("FORGELAB_PROGRESS"); p != "" {
		return p
	}
	return filepath.Join(root, ".forgelab", "progress.json")
}
