package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const repoTemplate = "../install-template/install.sh.tmpl"

func render(t *testing.T) (script, outPath string) {
	t.Helper()
	dir := t.TempDir()
	outPath = filepath.Join(dir, "install.sh")
	tmpl, err := os.ReadFile(repoTemplate)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	tmplPath := filepath.Join(dir, "install.sh.tmpl")
	if err := os.WriteFile(tmplPath, tmpl, 0o644); err != nil {
		t.Fatalf("copy template: %v", err)
	}

	if err := renderTemplate(tmplPath, outPath, defaultData()); err != nil {
		t.Fatalf("render: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	return string(data), outPath
}

func TestRender_Shebang(t *testing.T) {
	script, _ := render(t)
	if !strings.HasPrefix(script, "#!/bin/sh\n") {
		t.Errorf("script should start with #!/bin/sh, got first line: %q",
			strings.SplitN(script, "\n", 2)[0])
	}
}

func TestRender_StrictMode(t *testing.T) {
	script, _ := render(t)
	if !strings.Contains(script, "set -eu") {
		t.Error("script should contain 'set -eu'")
	}
}

func TestRender_OwnerAndRepoSubstituted(t *testing.T) {
	script, _ := render(t)
	d := defaultData()
	if !strings.Contains(script, `OWNER="`+d.Owner+`"`) {
		t.Errorf("OWNER not substituted: did not find %q", d.Owner)
	}
	if !strings.Contains(script, `REPO="`+d.Repo+`"`) {
		t.Errorf("REPO not substituted: did not find %q", d.Repo)
	}
}

func TestRender_NoUnresolvedPlaceholders(t *testing.T) {
	script, _ := render(t)
	if strings.Contains(script, "{{") || strings.Contains(script, "}}") {
		t.Error("rendered script contains unresolved Go-template delimiters")
	}
}

func TestRender_SupportedPlatformsSpaceSeparated(t *testing.T) {
	script, _ := render(t)
	// e.g. SUPPORTED_PLATFORMS="linux-amd64 linux-arm64 darwin-amd64 darwin-arm64"
	if !strings.Contains(script, `SUPPORTED_PLATFORMS="linux-amd64 linux-arm64 darwin-amd64 darwin-arm64"`) {
		t.Errorf("SUPPORTED_PLATFORMS not formatted as space-separated string")
	}
}

func TestRender_OutputIsExecutable(t *testing.T) {
	_, outPath := render(t)
	st, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if st.Mode().Perm()&0o111 == 0 {
		t.Errorf("rendered file not executable: mode=%v", st.Mode())
	}
}

func TestRender_ShellcheckClean(t *testing.T) {
	if _, err := exec.LookPath("shellcheck"); err != nil {
		t.Skip("shellcheck not installed; skipping")
	}
	_, outPath := render(t)
	cmd := exec.Command("shellcheck", "-S", "error", outPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if out, err := cmd.Output(); err != nil {
		t.Errorf("shellcheck -S error failed: %v\nstdout:\n%s\nstderr:\n%s", err, out, stderr.String())
	}
}

func TestRenderTemplate_MissingTemplate(t *testing.T) {
	dir := t.TempDir()
	err := renderTemplate(filepath.Join(dir, "nope.tmpl"), filepath.Join(dir, "out.sh"), defaultData())
	if err == nil {
		t.Fatal("expected error for missing template, got nil")
	}
	if !strings.Contains(err.Error(), "read template") {
		t.Errorf("expected 'read template' error, got: %v", err)
	}
}

func TestRenderTemplate_ParseError(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "bad.tmpl")
	if err := os.WriteFile(tmplPath, []byte("{{ .Owner "), 0o644); err != nil {
		t.Fatalf("write bad template: %v", err)
	}
	err := renderTemplate(tmplPath, filepath.Join(dir, "out.sh"), defaultData())
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "parse template") {
		t.Errorf("expected 'parse template' error, got: %v", err)
	}
}

func TestRenderTemplate_ExecuteError(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "exec.tmpl")
	// References a field that does not exist on templateData; with
	// missingkey=error this is only triggered by map keys, but a
	// nonexistent struct field surfaces as an execute error.
	if err := os.WriteFile(tmplPath, []byte("{{ .Nonexistent }}"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	err := renderTemplate(tmplPath, filepath.Join(dir, "out.sh"), defaultData())
	if err == nil {
		t.Fatal("expected execute error, got nil")
	}
	if !strings.Contains(err.Error(), "execute template") {
		t.Errorf("expected 'execute template' error, got: %v", err)
	}
}

func TestRenderTemplate_UnwritableOutput(t *testing.T) {
	dir := t.TempDir()
	// Create a regular file and try to write into it as if it were a
	// directory; MkdirAll on a path that already exists as a file
	// returns an error.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	tmplPath := filepath.Join(dir, "ok.tmpl")
	if err := os.WriteFile(tmplPath, []byte("hi"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	outPath := filepath.Join(blocker, "subdir", "install.sh")
	err := renderTemplate(tmplPath, outPath, defaultData())
	if err == nil {
		t.Fatal("expected mkdir error, got nil")
	}
	if !strings.Contains(err.Error(), "mkdir") {
		t.Errorf("expected 'mkdir' error, got: %v", err)
	}
}

func TestRenderTemplate_OpenFileError(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "ok.tmpl")
	if err := os.WriteFile(tmplPath, []byte("hi"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	// Use the temp dir itself as the output path; OpenFile on an
	// existing directory fails.
	err := renderTemplate(tmplPath, dir, defaultData())
	if err == nil {
		t.Fatal("expected open error, got nil")
	}
	if !strings.Contains(err.Error(), "create") {
		t.Errorf("expected 'create' error, got: %v", err)
	}
}

func TestMain(m *testing.M) {
	if os.Getenv("GO_EXEC_MAIN") == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}

// runMain re-execs the test binary in main-mode and returns combined output
// plus the exit code (-1 on unexpected error).
func runMain(t *testing.T, args ...string) (string, int) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "GO_EXEC_MAIN=1")
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

func TestMain_RendersTemplate(t *testing.T) {
	dir := t.TempDir()
	tmpl, err := os.ReadFile(repoTemplate)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	tmplPath := filepath.Join(dir, "install.sh.tmpl")
	if err := os.WriteFile(tmplPath, tmpl, 0o644); err != nil {
		t.Fatalf("copy template: %v", err)
	}
	outPath := filepath.Join(dir, "out", "install.sh")

	out, code := runMain(t, "-template", tmplPath, "-o", outPath)
	if code != 0 {
		t.Fatalf("exit %d, output: %s", code, out)
	}
	if !strings.Contains(out, "rendered "+outPath) {
		t.Errorf("expected success message, got: %s", out)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("expected output file at %s: %v", outPath, err)
	}
}

func TestMain_FailureExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	out, code := runMain(t,
		"-template", filepath.Join(dir, "does-not-exist.tmpl"),
		"-o", filepath.Join(dir, "out.sh"))
	if code == 0 {
		t.Fatalf("expected non-zero exit, got 0; output: %s", out)
	}
	if !strings.Contains(out, "render-install:") {
		t.Errorf("expected 'render-install:' prefix, got: %s", out)
	}
}
