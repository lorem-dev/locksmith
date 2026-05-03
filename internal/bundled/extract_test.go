// internal/bundled/extract_test.go
package bundled

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type fakePrompter struct {
	calls    int
	response ConflictResolution
	err      error
}

func (p *fakePrompter) BundleExtractPrompt(name, existing, want string) (ConflictResolution, error) {
	p.calls++
	if p.err != nil {
		return 0, p.err
	}
	return p.response, nil
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TestExtract_FreshWrite(t *testing.T) {
	dir := t.TempDir()
	pluginContent := []byte("plugin-bytes")
	mf := Manifest{Entries: []Entry{
		{
			Name:   "locksmith-plugin-gopass",
			Kind:   KindPlugin,
			SHA256: sha256Hex(pluginContent),
			Size:   int64(len(pluginContent)),
		},
	}}
	data := makeZip(t, mf, map[string][]byte{"locksmith-plugin-gopass": pluginContent})
	b, openErr := openFromBytes(data)
	if openErr != nil {
		t.Fatalf("openFromBytes: %v", openErr)
	}
	if err := Extract(b, ExtractOptions{
		Names:        []string{"locksmith-plugin-gopass"},
		PluginsDir:   dir,
		PinentryPath: filepath.Join(dir, "should-not-be-used"),
	}); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "locksmith-plugin-gopass"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(pluginContent) {
		t.Errorf("content = %q, want %q", got, pluginContent)
	}
	info, _ := os.Stat(filepath.Join(dir, "locksmith-plugin-gopass"))
	if info.Mode()&0o111 == 0 {
		t.Errorf("mode = %v, want executable", info.Mode())
	}
}

func TestExtract_SHA256Match_SilentSkip(t *testing.T) {
	dir := t.TempDir()
	content := []byte("same")
	if err := os.WriteFile(filepath.Join(dir, "p"), content, 0o755); err != nil {
		t.Fatal(err)
	}
	mf := Manifest{Entries: []Entry{
		{Name: "p", Kind: KindPlugin, SHA256: sha256Hex(content)},
	}}
	data := makeZip(t, mf, map[string][]byte{"p": content})
	b, _ := openFromBytes(data)
	prompter := &fakePrompter{response: Overwrite}
	if err := Extract(b, ExtractOptions{
		Names:      []string{"p"},
		PluginsDir: dir,
		Prompter:   prompter,
	}); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if prompter.calls != 0 {
		t.Errorf("prompter called %d times, want 0 (silent skip on sha match)", prompter.calls)
	}
}

func TestExtract_Mismatch_KeepWithWarning(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "p"), []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	newContent := []byte("new")
	mf := Manifest{Entries: []Entry{
		{Name: "p", Kind: KindPlugin, SHA256: sha256Hex(newContent)},
	}}
	data := makeZip(t, mf, map[string][]byte{"p": newContent})
	b, _ := openFromBytes(data)
	prompter := &fakePrompter{response: Keep}
	var keptCalled bool
	var keptWarn bool
	if err := Extract(b, ExtractOptions{
		Names:      []string{"p"},
		PluginsDir: dir,
		Prompter:   prompter,
		OnKept: func(name string, withWarning bool) {
			if name == "p" {
				keptCalled = true
				keptWarn = withWarning
			}
		},
	}); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "p"))
	if string(got) != "old" {
		t.Errorf("file overwritten despite Keep: %q", got)
	}
	if !keptCalled || !keptWarn {
		t.Errorf("OnKept(true) not invoked: called=%v warn=%v", keptCalled, keptWarn)
	}
}

func TestExtract_Mismatch_Overwrite(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "p"), []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	newContent := []byte("new")
	mf := Manifest{Entries: []Entry{
		{Name: "p", Kind: KindPlugin, SHA256: sha256Hex(newContent)},
	}}
	data := makeZip(t, mf, map[string][]byte{"p": newContent})
	b, _ := openFromBytes(data)
	prompter := &fakePrompter{response: Overwrite}
	if err := Extract(b, ExtractOptions{Names: []string{"p"}, PluginsDir: dir, Prompter: prompter}); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "p"))
	if string(got) != "new" {
		t.Errorf("content = %q, want new", got)
	}
}

func TestExtract_StickyOverwriteAll(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a", "b"} {
		os.WriteFile(filepath.Join(dir, n), []byte("old"), 0o755)
	}
	newA, newB := []byte("newA"), []byte("newB")
	mf := Manifest{Entries: []Entry{
		{Name: "a", Kind: KindPlugin, SHA256: sha256Hex(newA)},
		{Name: "b", Kind: KindPlugin, SHA256: sha256Hex(newB)},
	}}
	data := makeZip(t, mf, map[string][]byte{"a": newA, "b": newB})
	b, _ := openFromBytes(data)
	prompter := &fakePrompter{response: OverwriteAll}
	if err := Extract(b, ExtractOptions{Names: []string{"a", "b"}, PluginsDir: dir, Prompter: prompter}); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if prompter.calls != 1 {
		t.Errorf("prompter calls = %d, want 1 (sticky)", prompter.calls)
	}
	a, _ := os.ReadFile(filepath.Join(dir, "a"))
	bb, _ := os.ReadFile(filepath.Join(dir, "b"))
	if string(a) != "newA" || string(bb) != "newB" {
		t.Errorf("files not all overwritten: a=%q b=%q", a, bb)
	}
}

func TestExtract_ForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "p"), []byte("old"), 0o755)
	newContent := []byte("new")
	mf := Manifest{Entries: []Entry{
		{Name: "p", Kind: KindPlugin, SHA256: sha256Hex(newContent)},
	}}
	data := makeZip(t, mf, map[string][]byte{"p": newContent})
	b, _ := openFromBytes(data)
	prompter := &fakePrompter{response: Keep}
	if err := Extract(b, ExtractOptions{
		Names: []string{"p"}, PluginsDir: dir, Prompter: prompter, ForceOverwrite: true,
	}); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if prompter.calls != 0 {
		t.Errorf("prompter called %d times under ForceOverwrite, want 0", prompter.calls)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "p"))
	if string(got) != "new" {
		t.Errorf("ForceOverwrite did not overwrite: %q", got)
	}
}

func TestExtract_PinentryKind(t *testing.T) {
	dir := t.TempDir()
	pin := filepath.Join(dir, "bin", "locksmith-pinentry")
	content := []byte("pinentry-bytes")
	mf := Manifest{Entries: []Entry{
		{Name: "locksmith-pinentry", Kind: KindPinentry, SHA256: sha256Hex(content)},
	}}
	data := makeZip(t, mf, map[string][]byte{"locksmith-pinentry": content})
	b, _ := openFromBytes(data)
	if err := Extract(b, ExtractOptions{Names: []string{"locksmith-pinentry"}, PinentryPath: pin}); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got, _ := os.ReadFile(pin)
	if string(got) != string(content) {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestExtract_BundleSHAFails(t *testing.T) {
	// Force a sha256 mismatch between manifest and content.
	dir := t.TempDir()
	mf := Manifest{Entries: []Entry{
		{Name: "p", Kind: KindPlugin, SHA256: sha256Hex([]byte("expected"))},
	}}
	data := makeZip(t, mf, map[string][]byte{"p": []byte("actual")})
	b, _ := openFromBytes(data)
	err := Extract(b, ExtractOptions{Names: []string{"p"}, PluginsDir: dir})
	if err == nil || !errors.Is(err, ErrSHAMismatch) {
		t.Errorf("err = %v, want ErrSHAMismatch", err)
	}
}
