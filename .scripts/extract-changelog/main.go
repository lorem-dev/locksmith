// extract-changelog reads CHANGES.md and emits the body of
// "## Version vX.Y.Z" for a given version. Used by the release workflow
// to assemble GitHub-release notes verbatim from the changelog.
//
// Usage:
//
//	go run ./.scripts/extract-changelog -version vX.Y.Z [-input CHANGES.md] [-o file]
//
// Or via env: VERSION=vX.Y.Z go run ./.scripts/extract-changelog
//
// Errors when the section is missing or its body is empty.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	var (
		versionFlag = flag.String("version", "", "version (e.g. v0.2.0). Falls back to $VERSION.")
		inputFlag   = flag.String("input", "CHANGES.md", "path to CHANGES.md")
		outputFlag  = flag.String("o", "", "output path (default: stdout)")
	)
	flag.Parse()

	version := *versionFlag
	if version == "" {
		version = os.Getenv("VERSION")
	}
	if version == "" {
		fmt.Fprintln(os.Stderr, "extract-changelog: -version flag or VERSION env var required")
		os.Exit(2)
	}

	body, err := extract(*inputFlag, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "extract-changelog: %v\n", err)
		os.Exit(1)
	}

	if *outputFlag == "" {
		fmt.Println(body)
		return
	}
	if err := os.WriteFile(*outputFlag, []byte(body+"\n"), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "extract-changelog: write %s: %v\n", *outputFlag, err)
		os.Exit(1)
	}
}

// extract reads the changelog at path and returns the body of the
// "## Version <version>" section, trimmed of leading/trailing whitespace.
//
// version must include the leading "v" exactly as it appears in the
// changelog heading (e.g. "v0.2.0").
func extract(path, version string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	// Match "## Version <version>" with a trailing word boundary so that
	// "v0.1.0" does not match "v0.1.10" but does match the dated form
	// "## Version v0.1.0 - 2026-05-07" produced by the changelog skill.
	heading := "## Version " + version
	pattern := regexp.MustCompile(`(?m)^## Version ` + regexp.QuoteMeta(version) + `\b`)
	lines := strings.Split(string(data), "\n")

	start := -1
	for i, line := range lines {
		if pattern.MatchString(line) {
			start = i + 1
			break
		}
	}
	if start == -1 {
		return "", fmt.Errorf("section %q not found in %s", heading, path)
	}

	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}

	body := strings.TrimSpace(strings.Join(lines[start:end], "\n"))
	if body == "" {
		return "", errors.New("section body is empty")
	}
	return body, nil
}
