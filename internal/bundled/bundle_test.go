// internal/bundled/bundle_test.go
package bundled

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"
)

// makeZip builds an in-memory bundle zip. manifest is encoded as JSON and
// stored as manifest.json; entries map names to file contents.
func makeZip(t *testing.T, manifest Manifest, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	mfw, err := zw.Create("manifest.json")
	if err != nil {
		t.Fatalf("create manifest: %v", err)
	}
	if err := json.NewEncoder(mfw).Encode(manifest); err != nil {
		t.Fatalf("encode manifest: %v", err)
	}
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func TestOpenFromBytes_NonEmpty(t *testing.T) {
	mf := Manifest{
		BundleVersion:    "0.1.0",
		LocksmithVersion: "0.1.0",
		Platform:         "darwin",
		Entries: []Entry{
			{Name: "locksmith-plugin-gopass", Kind: KindPlugin, SHA256: "abc", Size: 3},
		},
	}
	data := makeZip(t, mf, map[string][]byte{
		"locksmith-plugin-gopass": []byte("aaa"),
	})
	b, err := openFromBytes(data)
	if err != nil {
		t.Fatalf("openFromBytes: %v", err)
	}
	if got := b.Manifest.Platform; got != "darwin" {
		t.Errorf("Platform = %q, want darwin", got)
	}
	if got := len(b.Manifest.Entries); got != 1 {
		t.Errorf("Entries len = %d, want 1", got)
	}
	rc, err := b.Open("locksmith-plugin-gopass")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if string(got) != "aaa" {
		t.Errorf("content = %q, want aaa", got)
	}
}

func TestOpenFromBytes_Empty(t *testing.T) {
	mf := Manifest{Platform: "darwin", Entries: []Entry{}}
	data := makeZip(t, mf, nil)
	_, err := openFromBytes(data)
	if !errors.Is(err, ErrEmptyBundle) {
		t.Errorf("err = %v, want ErrEmptyBundle", err)
	}
}

func TestOpenFromBytes_NoManifest(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("locksmith-plugin-gopass")
	w.Write([]byte("aaa"))
	zw.Close()
	_, err := openFromBytes(buf.Bytes())
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
}

func TestFindEntry(t *testing.T) {
	mf := Manifest{Entries: []Entry{
		{Name: "a", Kind: KindPlugin},
		{Name: "b", Kind: KindPinentry},
	}}
	data := makeZip(t, mf, map[string][]byte{"a": {0}, "b": {0}})
	b, err := openFromBytes(data)
	if err != nil {
		t.Fatalf("openFromBytes: %v", err)
	}
	e, ok := b.FindEntry("b")
	if !ok || e.Kind != KindPinentry {
		t.Errorf("FindEntry(b) = %v, %v; want pinentry, true", e, ok)
	}
	if _, ok := b.FindEntry("missing"); ok {
		t.Errorf("FindEntry(missing) = true, want false")
	}
}
