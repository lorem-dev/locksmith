package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGES.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestExtract_SectionMidFile(t *testing.T) {
	path := writeFixture(t, `# Changelog

## Development

- new dev entry

## Version v0.2.0

- BREAKING: removed flag X
- added Y

## Version v0.1.0

- initial release
`)

	got, err := extract(path, "v0.2.0")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	want := "- BREAKING: removed flag X\n- added Y"
	if strings.TrimSpace(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtract_SectionAtEndOfFile(t *testing.T) {
	path := writeFixture(t, `# Changelog

## Version v0.1.0

- only version, last in file
`)

	got, err := extract(path, "v0.1.0")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if strings.TrimSpace(got) != "- only version, last in file" {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestExtract_PreservesBreakingGrouping(t *testing.T) {
	path := writeFixture(t, `# Changelog

## Version v1.0.0

- BREAKING: removed CLI flag --foo
- BREAKING: changed proto Message.Bar
- added new feature
- fixed bug
`)

	got, err := extract(path, "v1.0.0")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}
	if !strings.HasPrefix(lines[0], "- BREAKING:") {
		t.Errorf("line 0 should start with BREAKING:, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "- BREAKING:") {
		t.Errorf("line 1 should start with BREAKING:, got %q", lines[1])
	}
	if strings.HasPrefix(lines[2], "- BREAKING:") {
		t.Errorf("line 2 should not start with BREAKING:, got %q", lines[2])
	}
}

func TestExtract_MissingSection(t *testing.T) {
	path := writeFixture(t, `# Changelog

## Version v0.1.0

- a thing
`)

	_, err := extract(path, "v9.9.9")
	if err == nil {
		t.Fatal("expected error for missing section, got nil")
	}
	if !strings.Contains(err.Error(), "v9.9.9") {
		t.Errorf("error should mention version, got %v", err)
	}
}

func TestExtract_EmptySection(t *testing.T) {
	path := writeFixture(t, `# Changelog

## Version v0.1.0

## Version v0.0.1

- first
`)

	_, err := extract(path, "v0.1.0")
	if err == nil {
		t.Fatal("expected error for empty section body, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention 'empty', got %v", err)
	}
}

func TestExtract_StopsAtNextHeading(t *testing.T) {
	path := writeFixture(t, `# Changelog

## Version v0.2.0

- second
- entry

## Version v0.1.0

- first
`)

	got, err := extract(path, "v0.2.0")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if strings.Contains(got, "first") {
		t.Errorf("output bled into next section: %q", got)
	}
}

func TestExtract_VersionRequiresVPrefix(t *testing.T) {
	// extract should accept "v0.1.0" exactly as written in the heading.
	// Without v-prefix the heading does not match.
	path := writeFixture(t, `# Changelog

## Version v0.1.0

- entry
`)

	if _, err := extract(path, "0.1.0"); err == nil {
		t.Error("expected error when version has no v-prefix")
	}
}

func TestExtract_TrailingWhitespaceTrimmed(t *testing.T) {
	path := writeFixture(t, "# Changelog\n\n## Version v0.1.0\n\n- entry\n\n\n\n## Version v0.0.1\n\n- old\n")

	got, err := extract(path, "v0.1.0")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if strings.HasSuffix(got, "\n\n") {
		t.Errorf("output should not have multiple trailing newlines: %q", got)
	}
}

func TestExtract_HeadingWithDateSuffix(t *testing.T) {
	// The 'changelog' skill writes headings as
	// '## Version vX.Y.Z - YYYY-MM-DD'. extract must accept this form.
	path := writeFixture(t, `# Changelog

## Development

- dev entry

## Version v0.1.0 - 2026-05-07

- first release

## Version v0.0.1 - 2026-04-01

- older
`)

	got, err := extract(path, "v0.1.0")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if strings.TrimSpace(got) != "- first release" {
		t.Errorf("got %q, want %q", got, "- first release")
	}
}

func TestExtract_DoesNotMatchPartialVersionPrefix(t *testing.T) {
	// v0.1.0 must NOT match a heading for v0.1.10.
	path := writeFixture(t, `# Changelog

## Version v0.1.10 - 2026-06-01

- ten release
`)

	if _, err := extract(path, "v0.1.0"); err == nil {
		t.Error("expected error: v0.1.0 must not match v0.1.10")
	}
}

func TestExtract_ChangelogSkillFormat(t *testing.T) {
	// End-to-end: exact format that the 'changelog' skill produces.
	path := writeFixture(t, "# Changelog\n\n## Version v0.1.0 - 2026-05-07\n\n- bullet\n")
	got, err := extract(path, "v0.1.0")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if strings.TrimSpace(got) != "- bullet" {
		t.Errorf("got %q, want %q", got, "- bullet")
	}
}

func TestExtract_ReadFileError(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist.md")
	_, err := extract(missing, "v0.1.0")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "read") {
		t.Errorf("error should mention 'read', got %v", err)
	}
}

// TestMain lets individual tests re-exec this binary with GO_EXEC_MAIN=1 so
// they can drive func main() through the real flag/env paths and observe its
// exit code, stdout, and stderr.
func TestMain(m *testing.M) {
	if os.Getenv("GO_EXEC_MAIN") == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}

// runMain re-execs the test binary in main-mode and returns combined output
// plus the exit code (-1 on unexpected error).
func runMain(t *testing.T, env []string, args ...string) (string, int) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "GO_EXEC_MAIN=1")
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return string(out), ee.ExitCode()
	}
	t.Fatalf("run main: %v", err)
	return string(out), -1
}

func TestMain_PrintsBodyToStdout(t *testing.T) {
	path := writeFixture(t, "# Changelog\n\n## Version v0.1.0\n\n- entry\n")
	out, code := runMain(t, nil, "-version", "v0.1.0", "-input", path)
	if code != 0 {
		t.Fatalf("exit %d, output: %s", code, out)
	}
	if !strings.Contains(out, "- entry") {
		t.Errorf("stdout should contain body, got %q", out)
	}
}

func TestMain_VersionFromEnv(t *testing.T) {
	path := writeFixture(t, "# Changelog\n\n## Version v0.2.0\n\n- env entry\n")
	out, code := runMain(t, []string{"VERSION=v0.2.0"}, "-input", path)
	if code != 0 {
		t.Fatalf("exit %d, output: %s", code, out)
	}
	if !strings.Contains(out, "- env entry") {
		t.Errorf("stdout should contain body, got %q", out)
	}
}

func TestMain_MissingVersionExits2(t *testing.T) {
	out, code := runMain(t, []string{"VERSION="}, "-input", "CHANGES.md")
	if code != 2 {
		t.Errorf("exit code = %d, want 2; output: %s", code, out)
	}
	if !strings.Contains(out, "required") {
		t.Errorf("stderr should mention 'required', got %q", out)
	}
}

func TestMain_ExtractErrorExits1(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.md")
	out, code := runMain(t, nil, "-version", "v0.1.0", "-input", missing)
	if code != 1 {
		t.Errorf("exit code = %d, want 1; output: %s", code, out)
	}
}

func TestMain_WritesToOutputFile(t *testing.T) {
	path := writeFixture(t, "# Changelog\n\n## Version v0.1.0\n\n- file out\n")
	dir := t.TempDir()
	outPath := filepath.Join(dir, "notes.md")
	_, code := runMain(t, nil, "-version", "v0.1.0", "-input", path, "-o", outPath)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(got), "- file out") {
		t.Errorf("output file should contain body, got %q", got)
	}
	if !strings.HasSuffix(string(got), "\n") {
		t.Errorf("output file should end with newline, got %q", got)
	}
}

func TestMain_WriteFileError(t *testing.T) {
	path := writeFixture(t, "# Changelog\n\n## Version v0.1.0\n\n- entry\n")
	// Use a path under a non-existent directory to force WriteFile failure.
	bogus := filepath.Join(t.TempDir(), "no-such-dir", "out.md")
	out, code := runMain(t, nil, "-version", "v0.1.0", "-input", path, "-o", bogus)
	if code != 1 {
		t.Errorf("exit code = %d, want 1; output: %s", code, out)
	}
	if !strings.Contains(out, "write") {
		t.Errorf("stderr should mention 'write', got %q", out)
	}
}
