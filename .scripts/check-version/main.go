// check-version verifies that the git tag a CI build is running on
// matches the version recorded in sdk/version/VERSION, and that
// CHANGES.md has a corresponding "## Version v<VERSION>" section.
//
// Tag resolution priority:
//  1. $GITHUB_REF (if it begins with "refs/tags/")
//  2. $CI_COMMIT_TAG (GitLab CI)
//  3. `git tag --points-at HEAD` (local fallback)
//
// On non-tag builds (no tag resolved) the script prints
// "not a tag build, skipping" and exits 0, so it is safe to wire into
// every pipeline stage.
//
// Usage: go run ./.scripts/check-version
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	versionFile = "sdk/version/VERSION"
	changesFile = "CHANGES.md"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "check-version: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := workspaceRoot(".")
	if err != nil {
		return fmt.Errorf("finding workspace root: %w", err)
	}

	tag, err := resolveTagFromEnv(envLookup)
	if err != nil {
		return err
	}
	if tag == "" {
		// Local fallback.
		tag, err = resolveTagFromGit(root)
		if err != nil {
			return err
		}
	}
	if tag == "" {
		fmt.Println("not a tag build, skipping")
		return nil
	}

	tagVersion := normaliseTag(tag)
	fileVersion, err := readVersionFile(filepath.Join(root, versionFile))
	if err != nil {
		return err
	}
	if err := verifyVersionMatch(fileVersion, tagVersion); err != nil {
		return err
	}
	if err := verifyChangelogHas(filepath.Join(root, changesFile), tagVersion); err != nil {
		return err
	}

	fmt.Printf("version v%s verified (VERSION matches tag, CHANGES.md has section)\n", tagVersion)
	return nil
}

// envLookup is the default environment lookup; injected for testing.
func envLookup(key string) string {
	return os.Getenv(key)
}

// resolveTagFromEnv returns the tag from CI environment variables, or ""
// when none of the recognised variables carry a tag. Errors are reserved
// for malformed inputs (none currently).
func resolveTagFromEnv(getenv func(string) string) (string, error) {
	if ref := getenv("GITHUB_REF"); strings.HasPrefix(ref, "refs/tags/") {
		return strings.TrimPrefix(ref, "refs/tags/"), nil
	}
	if tag := getenv("CI_COMMIT_TAG"); tag != "" {
		return tag, nil
	}
	return "", nil
}

// resolveTagFromGit asks git for any tag pointing at HEAD. Returns the
// first tag whose name begins with "v"; otherwise the first tag in the
// list; or "" if none.
func resolveTagFromGit(root string) (string, error) {
	cmd := exec.Command("git", "tag", "--points-at", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		// `git tag` succeeds with empty output on no tags. An error here
		// indicates git is missing or the working dir is not a repo;
		// treat as "no tag" rather than fail the build.
		return "", nil
	}
	var first string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if first == "" {
			first = line
		}
		if strings.HasPrefix(line, "v") {
			return line, nil
		}
	}
	return first, nil
}

// normaliseTag strips a leading "v" from a tag.
func normaliseTag(tag string) string {
	return strings.TrimPrefix(tag, "v")
}

// readVersionFile reads VERSION, trims whitespace, and strips a leading
// "v" if present (defensive - the file should not have one).
func readVersionFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	v := strings.TrimSpace(string(data))
	v = strings.TrimPrefix(v, "v")
	return v, nil
}

// verifyVersionMatch returns an error when fileVersion and tagVersion
// disagree.
func verifyVersionMatch(fileVersion, tagVersion string) error {
	if fileVersion != tagVersion {
		return fmt.Errorf("tag v%s does not match VERSION %s", tagVersion, fileVersion)
	}
	return nil
}

// verifyChangelogHas returns an error when CHANGES.md has no
// "## Version v<version>" heading.
func verifyChangelogHas(path, version string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	pattern := regexp.MustCompile(`(?m)^## Version v` + regexp.QuoteMeta(version) + `\b`)
	if !pattern.Match(data) {
		return fmt.Errorf("CHANGES.md has no '## Version v%s' section", version)
	}
	return nil
}

// workspaceRoot walks up from start until it finds go.work.
func workspaceRoot(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("abs path: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, "go.work")); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", errors.New("go.work not found - run from inside the locksmith workspace")
		}
		abs = parent
	}
}
