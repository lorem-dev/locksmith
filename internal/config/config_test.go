package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lorem-dev/locksmith/internal/config"
)

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
defaults:
  session_ttl: 3h
  socket_path: /tmp/locksmith.sock

logging:
  level: debug
  format: json

vaults:
  keychain:
    type: keychain
  my-gopass:
    type: gopass
    store: personal

keys:
  github-token:
    vault: keychain
    path: "github-api-token"
  anthropic-key:
    vault: my-gopass
    path: "dev/anthropic"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Defaults.SessionTTL != "3h" {
		t.Errorf("SessionTTL = %q, want %q", cfg.Defaults.SessionTTL, "3h")
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "debug")
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Logging.Format = %q, want %q", cfg.Logging.Format, "json")
	}
	if len(cfg.Vaults) != 2 {
		t.Fatalf("len(Vaults) = %d, want 2", len(cfg.Vaults))
	}
	if cfg.Vaults["my-gopass"].Store != "personal" {
		t.Errorf("gopass store = %q, want %q", cfg.Vaults["my-gopass"].Store, "personal")
	}
	if len(cfg.Keys) != 2 {
		t.Fatalf("len(Keys) = %d, want 2", len(cfg.Keys))
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := config.Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("Load() expected error for missing file")
	}
}

func TestValidate_MissingVaultRef(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "3h", SocketPath: "/tmp/test.sock"},
		Vaults:   map[string]config.Vault{"keychain": {Type: "keychain"}},
		Keys:     map[string]config.Key{"bad-key": {Vault: "nonexistent", Path: "foo"}},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected error for key referencing nonexistent vault")
	}
}

func TestValidate_Defaults(t *testing.T) {
	cfg := &config.Config{
		Vaults: map[string]config.Vault{"kc": {Type: "keychain"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
	if cfg.Defaults.SessionTTL != "3h" {
		t.Errorf("default SessionTTL = %q, want %q", cfg.Defaults.SessionTTL, "3h")
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("default Logging.Level = %q, want %q", cfg.Logging.Level, "info")
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("default Logging.Format = %q, want %q", cfg.Logging.Format, "text")
	}
}

func TestExpandPath_Tilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	result := config.ExpandPath("~/.config/locksmith/locksmith.sock")
	expected := filepath.Join(home, ".config", "locksmith", "locksmith.sock")
	if result != expected {
		t.Errorf("ExpandPath() = %q, want %q", result, expected)
	}
}

func TestExpandPath_NoTilde(t *testing.T) {
	path := "/absolute/path/to/socket"
	result := config.ExpandPath(path)
	if result != path {
		t.Errorf("ExpandPath() = %q, want %q", result, path)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	result := config.DefaultConfigPath()
	expected := filepath.Join(home, ".config", "locksmith", "config.yaml")
	if result != expected {
		t.Errorf("DefaultConfigPath() = %q, want %q", result, expected)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte("invalid: yaml: [\nbad"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, err = config.Load(cfgPath)
	if err == nil {
		t.Fatal("Load() expected error for invalid YAML")
	}
}

func TestValidate_EmptyKeyPath(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "3h", SocketPath: "/tmp/test.sock"},
		Vaults:   map[string]config.Vault{"keychain": {Type: "keychain"}},
		Keys:     map[string]config.Key{"empty-path": {Vault: "keychain", Path: ""}},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected error for key with empty path")
	}
}

func TestVault_ServiceField(t *testing.T) {
	cfg, err := config.LoadFromBytes([]byte(`
defaults:
  session_ttl: 1h
vaults:
  work:
    type: keychain
    service: com.example.work
keys:
  mykey:
    vault: work
    path: myaccount
`))
	if err != nil {
		t.Fatalf("LoadFromBytes() error: %v", err)
	}
	if cfg.Vaults["work"].Service != "com.example.work" {
		t.Errorf("Service = %q, want %q", cfg.Vaults["work"].Service, "com.example.work")
	}
}

func TestValidate_KeychainPathMultipleSlashes(t *testing.T) {
	_, err := config.LoadFromBytes([]byte(`
defaults:
  session_ttl: 1h
vaults:
  k:
    type: keychain
keys:
  bad:
    vault: k
    path: a/b/c
`))
	if err == nil {
		t.Fatal("expected error for path with multiple slashes")
	}
	if !strings.Contains(err.Error(), "a/b/c") {
		t.Errorf("error should mention path, got: %v", err)
	}
}

func TestValidate_KeychainPathSingleSlash_Valid(t *testing.T) {
	_, err := config.LoadFromBytes([]byte(`
defaults:
  session_ttl: 1h
vaults:
  k:
    type: keychain
keys:
  ok:
    vault: k
    path: github/token
`))
	if err != nil {
		t.Errorf("single-slash path should be valid, got: %v", err)
	}
}
