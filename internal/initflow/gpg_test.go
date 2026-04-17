package initflow_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lorem-dev/locksmith/internal/initflow"
)

func TestApplyGPGPinentry_NoExistingFile(t *testing.T) {
	dir := t.TempDir()
	gnupgDir := filepath.Join(dir, ".gnupg")

	replaced, err := initflow.ApplyGPGPinentry(gnupgDir, "/usr/local/bin/locksmith-pinentry")
	if err != nil {
		t.Fatalf("ApplyGPGPinentry() error: %v", err)
	}
	if replaced != "" {
		t.Errorf("replaced = %q, want empty (no existing line)", replaced)
	}

	data, _ := os.ReadFile(filepath.Join(gnupgDir, "gpg-agent.conf"))
	if !strings.Contains(string(data), "pinentry-program /usr/local/bin/locksmith-pinentry") {
		t.Errorf("expected new line in conf:\n%s", data)
	}
}

func TestApplyGPGPinentry_NoExistingPinentry(t *testing.T) {
	dir := t.TempDir()
	gnupgDir := filepath.Join(dir, ".gnupg")
	os.MkdirAll(gnupgDir, 0o700)
	os.WriteFile(filepath.Join(gnupgDir, "gpg-agent.conf"),
		[]byte("default-cache-ttl 600\nmax-cache-ttl 7200\n"), 0o600)

	replaced, err := initflow.ApplyGPGPinentry(gnupgDir, "/bin/locksmith-pinentry")
	if err != nil {
		t.Fatalf("ApplyGPGPinentry() error: %v", err)
	}
	if replaced != "" {
		t.Errorf("replaced = %q, want empty", replaced)
	}

	data, _ := os.ReadFile(filepath.Join(gnupgDir, "gpg-agent.conf"))
	content := string(data)
	if !strings.Contains(content, "pinentry-program /bin/locksmith-pinentry") {
		t.Errorf("new line missing:\n%s", content)
	}
	if !strings.Contains(content, "default-cache-ttl 600") {
		t.Errorf("existing lines should be preserved:\n%s", content)
	}
}

func TestApplyGPGPinentry_CommentsOutExisting(t *testing.T) {
	dir := t.TempDir()
	gnupgDir := filepath.Join(dir, ".gnupg")
	os.MkdirAll(gnupgDir, 0o700)
	os.WriteFile(filepath.Join(gnupgDir, "gpg-agent.conf"),
		[]byte("default-cache-ttl 600\npinentry-program /opt/homebrew/bin/pinentry-mac\n"), 0o600)

	replaced, err := initflow.ApplyGPGPinentry(gnupgDir, "/bin/locksmith-pinentry")
	if err != nil {
		t.Fatalf("ApplyGPGPinentry() error: %v", err)
	}
	if replaced != "/opt/homebrew/bin/pinentry-mac" {
		t.Errorf("replaced = %q, want %q", replaced, "/opt/homebrew/bin/pinentry-mac")
	}

	data, _ := os.ReadFile(filepath.Join(gnupgDir, "gpg-agent.conf"))
	content := string(data)

	if !strings.Contains(content, "#pinentry-program /opt/homebrew/bin/pinentry-mac") {
		t.Errorf("old line should be commented out:\n%s", content)
	}
	if !strings.Contains(content, "pinentry-program /bin/locksmith-pinentry") {
		t.Errorf("new line missing:\n%s", content)
	}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "pinentry-program /opt/homebrew/bin/pinentry-mac" {
			t.Errorf("old line should be commented, found uncommented:\n%s", content)
		}
	}
}

func TestReadExistingPinentry_None(t *testing.T) {
	dir := t.TempDir()
	gnupgDir := filepath.Join(dir, ".gnupg")
	os.MkdirAll(gnupgDir, 0o700)
	os.WriteFile(filepath.Join(gnupgDir, "gpg-agent.conf"),
		[]byte("default-cache-ttl 600\n"), 0o600)

	got := initflow.ReadExistingPinentry(gnupgDir)
	if got != "" {
		t.Errorf("ReadExistingPinentry() = %q, want empty", got)
	}
}

func TestReadExistingPinentry_Found(t *testing.T) {
	dir := t.TempDir()
	gnupgDir := filepath.Join(dir, ".gnupg")
	os.MkdirAll(gnupgDir, 0o700)
	os.WriteFile(filepath.Join(gnupgDir, "gpg-agent.conf"),
		[]byte("pinentry-program /opt/homebrew/bin/pinentry-mac\n"), 0o600)

	got := initflow.ReadExistingPinentry(gnupgDir)
	if got != "/opt/homebrew/bin/pinentry-mac" {
		t.Errorf("ReadExistingPinentry() = %q, want %q", got, "/opt/homebrew/bin/pinentry-mac")
	}
}

func TestReadExistingPinentry_NoFile(t *testing.T) {
	dir := t.TempDir()
	got := initflow.ReadExistingPinentry(dir)
	if got != "" {
		t.Errorf("ReadExistingPinentry() on nonexistent file = %q, want empty", got)
	}
}
