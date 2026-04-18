package shellhook_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lorem-dev/locksmith/internal/shellhook"
)

func TestDetectShell(t *testing.T) {
	cases := []struct {
		name     string
		shellEnv string
		want     shellhook.Shell
	}{
		{"zsh_full_path", "/bin/zsh", shellhook.ShellZsh},
		{"zsh_usr_path", "/usr/bin/zsh", shellhook.ShellZsh},
		{"bash", "/bin/bash", shellhook.ShellBash},
		{"ash", "/bin/ash", shellhook.ShellAsh},
		{"fish", "/usr/bin/fish", shellhook.ShellFish},
		{"empty", "", shellhook.ShellUnknown},
		{"sh_unknown", "/bin/sh", shellhook.ShellUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("SHELL", c.shellEnv)
			t.Setenv("0", "")
			got := shellhook.DetectShell()
			if got != c.want {
				t.Errorf("DetectShell() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestDetectShell_FallbackTo0(t *testing.T) {
	t.Setenv("SHELL", "")
	t.Setenv("0", "/bin/bash")
	if got := shellhook.DetectShell(); got != shellhook.ShellBash {
		t.Errorf("got %v, want ShellBash", got)
	}
}

func TestRCFile_Known(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		s    shellhook.Shell
		want string
	}{
		{shellhook.ShellZsh, filepath.Join(home, ".zshrc")},
		{shellhook.ShellBash, filepath.Join(home, ".bashrc")},
		{shellhook.ShellFish, filepath.Join(home, ".config", "fish", "config.fish")},
	}
	for _, c := range cases {
		path, ok := shellhook.RCFile(c.s)
		if !ok {
			t.Errorf("RCFile(%v) ok=false, want true", c.s)
		}
		if path != c.want {
			t.Errorf("RCFile(%v) = %q, want %q", c.s, path, c.want)
		}
	}
}

func TestRCFile_Unknown(t *testing.T) {
	_, ok := shellhook.RCFile(shellhook.ShellUnknown)
	if ok {
		t.Error("RCFile(ShellUnknown) ok=true, want false")
	}
}

func TestRCFile_Ash_EnvVar(t *testing.T) {
	tmp := t.TempDir()
	envFile := filepath.Join(tmp, ".ashrc")
	t.Setenv("ENV", envFile)
	path, ok := shellhook.RCFile(shellhook.ShellAsh)
	if !ok {
		t.Fatal("ok=false")
	}
	if path != envFile {
		t.Errorf("got %q, want %q", path, envFile)
	}
}

func TestRCFile_Ash_Fallback(t *testing.T) {
	home, _ := os.UserHomeDir()
	t.Setenv("ENV", "")
	path, ok := shellhook.RCFile(shellhook.ShellAsh)
	if !ok {
		t.Fatal("ok=false")
	}
	want := filepath.Join(home, ".profile")
	if path != want {
		t.Errorf("got %q, want %q", path, want)
	}
}

func TestSnippet_ContainsMarker(t *testing.T) {
	for _, s := range []shellhook.Shell{shellhook.ShellBash, shellhook.ShellZsh, shellhook.ShellAsh, shellhook.ShellFish} {
		snip := shellhook.Snippet(s)
		// "# locksmith daemon autostart" is the marker constant from shellhook.go.
		// If this string is changed there, update it here too.
		if !strings.Contains(snip, "# locksmith daemon autostart") {
			t.Errorf("Snippet(%v) missing marker: %q", s, snip)
		}
		if !strings.Contains(snip, "locksmith _autostart") {
			t.Errorf("Snippet(%v) missing _autostart: %q", s, snip)
		}
	}
}

func TestSnippet_Fish_UsesEnd(t *testing.T) {
	snip := shellhook.Snippet(shellhook.ShellFish)
	if !strings.Contains(snip, "; end") {
		t.Errorf("fish snippet missing 'end': %q", snip)
	}
}

func TestSnippet_Posix_UsesFi(t *testing.T) {
	snip := shellhook.Snippet(shellhook.ShellBash)
	if !strings.Contains(snip, "; fi") {
		t.Errorf("bash snippet missing 'fi': %q", snip)
	}
}

func TestSnippet_Unknown_BareCommand(t *testing.T) {
	snip := shellhook.Snippet(shellhook.ShellUnknown)
	if strings.Contains(snip, "# locksmith daemon autostart") {
		t.Error("unknown snippet should not contain marker")
	}
	if snip != "locksmith _autostart 2>/dev/null" {
		t.Errorf("got %q", snip)
	}
}

func TestIsInstalled_False_Missing(t *testing.T) {
	ok, err := shellhook.IsInstalled(filepath.Join(t.TempDir(), "nonexistent"))
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected false for missing file")
	}
}

func TestIsInstalled_False_NoMarker(t *testing.T) {
	f := filepath.Join(t.TempDir(), ".zshrc")
	if err := os.WriteFile(f, []byte("export PATH=$PATH:/usr/local/bin\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err := shellhook.IsInstalled(f)
	if err != nil || ok {
		t.Errorf("got ok=%v err=%v", ok, err)
	}
}

func TestIsInstalled_True(t *testing.T) {
	f := filepath.Join(t.TempDir(), ".zshrc")
	if err := os.WriteFile(f, []byte("# locksmith daemon autostart\nif command -v locksmith ...\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err := shellhook.IsInstalled(f)
	if err != nil || !ok {
		t.Errorf("got ok=%v err=%v", ok, err)
	}
}

func TestInstall_AppendsSnippet(t *testing.T) {
	f := filepath.Join(t.TempDir(), ".bashrc")
	if err := os.WriteFile(f, []byte("# existing content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := shellhook.Install(f, shellhook.ShellBash); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(f)
	content := string(data)
	if !strings.Contains(content, "# locksmith daemon autostart") {
		t.Error("marker not found after Install")
	}
	if !strings.Contains(content, "# existing content") {
		t.Error("Install overwrote existing content")
	}
}

func TestInstall_CreatesFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), ".zshrc")
	if err := shellhook.Install(f, shellhook.ShellZsh); err != nil {
		t.Fatal(err)
	}
	ok, _ := shellhook.IsInstalled(f)
	if !ok {
		t.Error("IsInstalled returned false after Install")
	}
}

func TestInstall_Idempotent(t *testing.T) {
	// Install does NOT de-duplicate - this test documents that contract.
	// Callers must check IsInstalled first.
	f := filepath.Join(t.TempDir(), ".bashrc")
	if err := os.WriteFile(f, []byte("# existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := shellhook.Install(f, shellhook.ShellBash); err != nil {
		t.Fatal(err)
	}
	if err := shellhook.Install(f, shellhook.ShellBash); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(f)
	count := strings.Count(string(data), "# locksmith daemon autostart")
	if count != 2 {
		t.Errorf("expected marker to appear 2 times (Install does not de-dup), got %d", count)
	}
}
