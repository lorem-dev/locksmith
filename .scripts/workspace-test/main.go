// workspace-test runs "go test" for every module listed in the nearest go.work
// file. It supports three modes: plain (default), race, and coverage.
//
// Usage:
//
//	go run ./.scripts/workspace-test             # unit tests
//	go run ./.scripts/workspace-test race        # race detector
//	go run ./.scripts/workspace-test coverage    # coverage report per module
//
// The script resolves go.work relative to the current working directory by
// walking upward - so it works regardless of the subdirectory you run from.
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	modeTest     = "test"
	modeRace     = "race"
	modeCoverage = "coverage"
)

// color codes
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

func main() {
	mode := modeTest
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "race":
			mode = modeRace
		case "coverage":
			mode = modeCoverage
		case "test", "":
			mode = modeTest
		default:
			fmt.Fprintf(os.Stderr, "usage: workspace-test [test|race|coverage]\n")
			os.Exit(1)
		}
	}

	workFile, err := findGoWork(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	rootDir := filepath.Dir(workFile)

	modules, err := parseGoWork(workFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing go.work: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s%s=== workspace-test mode=%s modules=%d root=%s ===%s\n\n",
		colorBold, colorCyan, mode, len(modules), rootDir, colorReset)

	reportsDir := filepath.Join(rootDir, ".reports")
	if mode == modeCoverage {
		if err := os.MkdirAll(reportsDir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "error creating .reports: %v\n", err)
			os.Exit(1)
		}
	}

	type result struct {
		module string
		dir    string
		ok     bool
		out    string
		elapsed time.Duration
	}

	var results []result
	failed := 0

	for _, mod := range modules {
		dir := filepath.Join(rootDir, mod)
		label := mod
		if mod == "." {
			label = "(root)"
		}

		fmt.Printf("%s--- %s%s\n", colorGray, label, colorReset)

		var args []string
		switch mode {
		case modeRace:
			args = []string{"test", "-race", "./..."}
		case modeCoverage:
			profileName := strings.ReplaceAll(mod, "/", "-")
			if profileName == "." {
				profileName = "root"
			}
			profilePath := filepath.Join(reportsDir, "coverage-"+profileName+".out")
			args = []string{
				"test",
				"-coverprofile=" + profilePath,
				"-covermode=atomic",
				"./...",
			}
		default:
			args = []string{"test", "./..."}
		}

		start := time.Now()
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		elapsed := time.Since(start)

		ok := err == nil
		if !ok {
			failed++
		}

		results = append(results, result{
			module:  label,
			dir:     dir,
			ok:      ok,
			out:     string(out),
			elapsed: elapsed,
		})

		status := colorGreen + "PASS" + colorReset
		if !ok {
			status = colorRed + "FAIL" + colorReset
		}
		fmt.Printf("    %s  %s(%s)%s\n\n", status, colorGray, elapsed.Round(time.Millisecond), colorReset)

		if !ok {
			// Print failure output indented
			scanner := bufio.NewScanner(strings.NewReader(string(out)))
			for scanner.Scan() {
				fmt.Printf("    %s%s%s\n", colorRed, scanner.Text(), colorReset)
			}
			fmt.Println()
		}
	}

	// Coverage totals
	if mode == modeCoverage {
		fmt.Printf("%s%s=== coverage summary ===%s\n", colorBold, colorCyan, colorReset)
		for _, r := range results {
			if r.ok {
				// Extract "coverage: XX.X% of statements" from output
				cov := extractCoverage(r.out)
				covColor := colorGreen
				if cov < 90 && cov > 0 {
					covColor = colorYellow
				} else if cov == 0 {
					covColor = colorGray
				}
				fmt.Printf("    %-24s  %s%.1f%%%s\n", r.module, covColor, cov, colorReset)
			} else {
				fmt.Printf("    %-24s  %sFAIL%s\n", r.module, colorRed, colorReset)
			}
		}
		fmt.Println()

		// Open HTML reports
		for _, r := range results {
			if !r.ok {
				continue
			}
			profileName := strings.ReplaceAll(r.module, "/", "-")
			if r.module == "(root)" {
				profileName = "root"
			}
			profilePath := filepath.Join(reportsDir, "coverage-"+profileName+".out")
			htmlPath := strings.TrimSuffix(profilePath, ".out") + ".html"
			cmd := exec.Command("go", "tool", "cover", "-html="+profilePath, "-o", htmlPath)
			cmd.Dir = rootDir
			_ = cmd.Run()
		}
		fmt.Printf("%sCoverage HTML reports written to .reports/%s\n\n", colorGray, colorReset)
	}

	// Summary line
	total := len(results)
	passed := total - failed
	summaryColor := colorGreen
	if failed > 0 {
		summaryColor = colorRed
	}
	fmt.Printf("%s%s=== %d/%d modules passed ===%s\n", colorBold, summaryColor, passed, total, colorReset)

	if failed > 0 {
		os.Exit(1)
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

// parseGoWork extracts the directory paths from the use() block of a go.work file.
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
			// Strip inline comments
			if idx := strings.Index(line, "//"); idx >= 0 {
				line = strings.TrimSpace(line[:idx])
			}
			if line != "" {
				modules = append(modules, line)
			}
			continue
		}
		// Single-line use statement: use ./path
		if strings.HasPrefix(line, "use ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				modules = append(modules, parts[1])
			}
		}
	}
	return modules, scanner.Err()
}

// extractCoverage parses "coverage: XX.X% of statements" from go test output.
// Returns 0 if not found or if there are no test files.
func extractCoverage(output string) float64 {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var last float64
	for scanner.Scan() {
		line := scanner.Text()
		// Look for "coverage: XX.X% of statements"
		if idx := strings.Index(line, "coverage: "); idx < 0 {
			continue
		}
		var pct float64
		// Try to parse e.g. "coverage: 92.3% of statements"
		after := line[strings.Index(line, "coverage: ")+len("coverage: "):]
		fmt.Sscanf(after, "%f%%", &pct)
		if pct > last {
			last = pct
		}
	}
	return last
}
