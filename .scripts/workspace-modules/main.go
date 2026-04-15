// workspace-modules prints the directory path of every module listed in the
// nearest go.work file, one path per line. Paths are relative to the go.work
// directory (e.g. ".", "./sdk", "./plugins/gopass").
//
// Usage:
//
//	go run ./.scripts/workspace-modules
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	workFile, err := findGoWork(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	modules, err := parseGoWork(workFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing go.work: %v\n", err)
		os.Exit(1)
	}

	for _, m := range modules {
		fmt.Println(m)
	}
}

// findGoWork walks up from dir until it finds a go.work file.
func findGoWork(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(abs, "go.work")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("go.work not found (searched from %s)", dir)
		}
		abs = parent
	}
}

// parseGoWork extracts directory paths from the use() block of a go.work file.
func parseGoWork(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var modules []string
	inUse := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "use (") || line == "use (" {
			inUse = true
			continue
		}
		if inUse && line == ")" {
			inUse = false
			continue
		}
		if inUse {
			if idx := strings.Index(line, "//"); idx >= 0 {
				line = strings.TrimSpace(line[:idx])
			}
			if line != "" {
				modules = append(modules, line)
			}
			continue
		}
		// Single-line: use ./path
		if strings.HasPrefix(line, "use ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				modules = append(modules, parts[1])
			}
		}
	}
	return modules, scanner.Err()
}
