package plugin_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/lorem-dev/locksmith/internal/log"
	"github.com/lorem-dev/locksmith/internal/plugin"
)

func TestMain(m *testing.M) {
	log.Init(io.Discard, "error", "text")
	os.Exit(m.Run())
}

func TestDiscover_FindsPluginInDir(t *testing.T) {
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "locksmith-plugin-test")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	found := plugin.Discover([]string{dir})
	if len(found) != 1 {
		t.Fatalf("Discover() found %d plugins, want 1", len(found))
	}
	if found["test"] != fakeBin {
		t.Errorf("plugin path = %q, want %q", found["test"], fakeBin)
	}
}

func TestDiscover_IgnoresNonPlugin(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "other-binary"), []byte("#!/bin/sh\n"), 0o755)

	found := plugin.Discover([]string{dir})
	if len(found) != 0 {
		t.Fatalf("Discover() found %d plugins, want 0", len(found))
	}
}

func TestDiscover_IgnoresNonExecutable(t *testing.T) {
	dir := t.TempDir()
	// Write without execute bit
	os.WriteFile(filepath.Join(dir, "locksmith-plugin-nope"), []byte("#!/bin/sh\n"), 0o644)

	found := plugin.Discover([]string{dir})
	if len(found) != 0 {
		t.Fatalf("Discover() should ignore non-executable, found %d", len(found))
	}
}

func TestDiscover_MultiplePlugins(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"locksmith-plugin-keychain", "locksmith-plugin-gopass"} {
		os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755)
	}

	found := plugin.Discover([]string{dir})
	if len(found) != 2 {
		t.Fatalf("Discover() found %d plugins, want 2", len(found))
	}
	if _, ok := found["keychain"]; !ok {
		t.Error("missing keychain plugin")
	}
	if _, ok := found["gopass"]; !ok {
		t.Error("missing gopass plugin")
	}
}

func TestDiscover_FirstWins(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	bin1 := filepath.Join(dir1, "locksmith-plugin-test")
	bin2 := filepath.Join(dir2, "locksmith-plugin-test")
	os.WriteFile(bin1, []byte("#!/bin/sh\n"), 0o755)
	os.WriteFile(bin2, []byte("#!/bin/sh\n"), 0o755)

	found := plugin.Discover([]string{dir1, dir2})
	if found["test"] != bin1 {
		t.Errorf("first dir should win, got %q", found["test"])
	}
}

func TestNewManager(t *testing.T) {
	m := plugin.NewManager()
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
}

func TestManager_Get_NotFound(t *testing.T) {
	m := plugin.NewManager()
	_, err := m.Get("nonexistent")
	if err == nil {
		t.Fatal("Get() expected error for unknown vault type")
	}
}

func TestManager_Types_Empty(t *testing.T) {
	m := plugin.NewManager()
	types := m.Types()
	if len(types) != 0 {
		t.Fatalf("Types() = %v, want empty", types)
	}
}

func TestManager_Kill_Empty(t *testing.T) {
	m := plugin.NewManager()
	m.Kill() // should not panic
}

func TestDefaultSearchDirs(t *testing.T) {
	dirs := plugin.DefaultSearchDirs()
	if dirs == nil {
		t.Fatal("DefaultSearchDirs() returned nil")
	}
}

func TestDiscover_NonexistentDir(t *testing.T) {
	found := plugin.Discover([]string{"/nonexistent/path/12345"})
	if len(found) != 0 {
		t.Fatalf("expected 0 plugins from nonexistent dir, got %d", len(found))
	}
}
