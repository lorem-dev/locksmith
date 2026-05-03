// .scripts/check-version/main_test.go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTag_GitHubRef(t *testing.T) {
	t.Setenv("GITHUB_REF", "refs/tags/v0.1.0")
	t.Setenv("CI_COMMIT_TAG", "")
	got, err := resolveTagFromEnv(envLookup)
	if err != nil {
		t.Fatalf("resolveTagFromEnv: %v", err)
	}
	if got != "v0.1.0" {
		t.Errorf("tag = %q, want v0.1.0", got)
	}
}

func TestResolveTag_GitHubRefBranch_NoTag(t *testing.T) {
	t.Setenv("GITHUB_REF", "refs/heads/main")
	t.Setenv("CI_COMMIT_TAG", "")
	got, err := resolveTagFromEnv(envLookup)
	if err != nil {
		t.Fatalf("resolveTagFromEnv: %v", err)
	}
	if got != "" {
		t.Errorf("tag = %q, want empty (branch ref)", got)
	}
}

func TestResolveTag_GitLabCommitTag(t *testing.T) {
	t.Setenv("GITHUB_REF", "")
	t.Setenv("CI_COMMIT_TAG", "v0.2.0")
	got, err := resolveTagFromEnv(envLookup)
	if err != nil {
		t.Fatalf("resolveTagFromEnv: %v", err)
	}
	if got != "v0.2.0" {
		t.Errorf("tag = %q, want v0.2.0", got)
	}
}

func TestResolveTag_NoEnv(t *testing.T) {
	t.Setenv("GITHUB_REF", "")
	t.Setenv("CI_COMMIT_TAG", "")
	got, err := resolveTagFromEnv(envLookup)
	if err != nil {
		t.Fatalf("resolveTagFromEnv: %v", err)
	}
	if got != "" {
		t.Errorf("tag = %q, want empty", got)
	}
}

func TestNormaliseTag(t *testing.T) {
	cases := map[string]string{
		"v0.1.0": "0.1.0",
		"0.1.0":  "0.1.0",
		"":       "",
	}
	for in, want := range cases {
		if got := normaliseTag(in); got != want {
			t.Errorf("normaliseTag(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVerifyVersionMatch(t *testing.T) {
	if err := verifyVersionMatch("0.1.0", "0.1.0"); err != nil {
		t.Errorf("equal: err = %v", err)
	}
	if err := verifyVersionMatch("0.1.0", "0.2.0"); err == nil {
		t.Error("mismatch: expected error, got nil")
	}
}

func TestVerifyChangelogHas_Match(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGES.md")
	body := "# Changelog\n\n## Development\n\n- in progress\n\n## Version v0.1.0 - 2026-05-04\n\n- initial release\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChangelogHas(path, "0.1.0"); err != nil {
		t.Errorf("present: err = %v", err)
	}
}

func TestVerifyChangelogHas_Missing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGES.md")
	body := "# Changelog\n\n## Development\n\n- in progress\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	err := verifyChangelogHas(path, "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "no '## Version v0.1.0'") {
		t.Errorf("missing: err = %v, want 'no ...' error", err)
	}
}

func TestReadVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "VERSION")
	if err := os.WriteFile(path, []byte("0.3.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readVersionFile(path)
	if err != nil {
		t.Fatalf("readVersionFile: %v", err)
	}
	if got != "0.3.0" {
		t.Errorf("got %q, want 0.3.0", got)
	}
}

func TestReadVersion_StripsVPrefix(t *testing.T) {
	// The file should not contain a v-prefix; if it does we strip it
	// to be tolerant.
	dir := t.TempDir()
	path := filepath.Join(dir, "VERSION")
	if err := os.WriteFile(path, []byte("v0.3.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readVersionFile(path)
	if err != nil {
		t.Fatalf("readVersionFile: %v", err)
	}
	if got != "0.3.0" {
		t.Errorf("got %q, want 0.3.0 (v stripped)", got)
	}
}

// envLookup is the production lookup function injected into
// resolveTagFromEnv. The tests use the real os.Getenv via t.Setenv.
func TestEnvLookup(t *testing.T) {
	t.Setenv("FOO_BAR_BAZ", "hello")
	if got := envLookup("FOO_BAR_BAZ"); got != "hello" {
		t.Errorf("envLookup = %q, want hello", got)
	}
	if got := envLookup("DOES_NOT_EXIST_XYZ"); got != "" {
		t.Errorf("envLookup missing = %q, want empty", got)
	}
}
