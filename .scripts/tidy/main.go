// tidy walks the workspace and runs "go mod tidy" in every directory
// containing a go.mod file.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	root, err := workspaceRoot(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	modules, err := findModules(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error finding modules: %v\n", err)
		os.Exit(1)
	}

	if len(modules) == 0 {
		fmt.Println("no go.mod files found")
		return
	}

	failed := 0
	for _, dir := range modules {
		rel, _ := filepath.Rel(root, dir)
		fmt.Printf("tidying %-40s\n", rel)
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  FAIL: %v\n", err)
			failed++
		}
	}

	if failed > 0 {
		fmt.Fprintf(os.Stderr, "%d module(s) failed to tidy\n", failed)
		os.Exit(1)
	}
	fmt.Printf("tidied %d module(s)\n", len(modules))
}

// findModules returns all directories under root that contain a go.mod.
// Skips .git, vendor, .scripts, and .worktrees.
func findModules(root string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" || name == ".scripts" || name == ".worktrees" {
				return filepath.SkipDir
			}
		}
		if !d.IsDir() && d.Name() == "go.mod" {
			dirs = append(dirs, filepath.Dir(path))
		}
		return nil
	})
	return dirs, err
}

// workspaceRoot walks up from dir until it finds a go.work file.
func workspaceRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, "go.work")); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("go.work not found (searched from %s)", dir)
		}
		abs = parent
	}
}
