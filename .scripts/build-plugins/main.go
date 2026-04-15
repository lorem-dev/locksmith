// build-plugins discovers all Go modules under plugins/ and builds each one.
// The output binary name is derived from the module name in go.mod:
// the last path segment is used (e.g. "locksmith-plugin-gopass" from
// "github.com/lorem-dev/locksmith-plugin-gopass").
//
// Binaries are written to bin/ relative to the workspace root.
//
// Usage:
//
//	go run ./.scripts/build-plugins
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	root, err := workspaceRoot(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	plugins, err := discoverPlugins(filepath.Join(root, "plugins"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error discovering plugins: %v\n", err)
		os.Exit(1)
	}

	if len(plugins) == 0 {
		fmt.Println("no plugins found under plugins/")
		return
	}

	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating bin/: %v\n", err)
		os.Exit(1)
	}

	failed := 0
	for _, p := range plugins {
		outPath := filepath.Join(binDir, p.binaryName)
		fmt.Printf("building %-40s -> bin/%s\n", p.moduleName, p.binaryName)

		cmd := exec.Command("go", "build", "-o", outPath, ".")
		cmd.Dir = p.dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  FAIL: %v\n", err)
			failed++
		}
	}

	if failed > 0 {
		fmt.Fprintf(os.Stderr, "%d plugin(s) failed to build\n", failed)
		os.Exit(1)
	}
	fmt.Printf("built %d plugin(s)\n", len(plugins))
}

type plugin struct {
	dir        string
	moduleName string
	binaryName string
}

// discoverPlugins scans pluginsDir for subdirectories containing go.mod and
// returns a plugin descriptor for each one.
func discoverPlugins(pluginsDir string) ([]plugin, error) {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var plugins []plugin
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(pluginsDir, e.Name())
		modFile := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(modFile); err != nil {
			continue // no go.mod - skip
		}

		moduleName, err := readModuleName(modFile)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", modFile, err)
		}

		// Binary name = last path segment of the module path.
		parts := strings.Split(moduleName, "/")
		binaryName := parts[len(parts)-1]

		plugins = append(plugins, plugin{
			dir:        dir,
			moduleName: moduleName,
			binaryName: binaryName,
		})
	}
	return plugins, nil
}

// readModuleName extracts the module path from a go.mod file.
func readModuleName(modFile string) (string, error) {
	f, err := os.Open(modFile)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("module directive not found in %s", modFile)
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
