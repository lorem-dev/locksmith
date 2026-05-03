// internal/bundled/bundle.go
package bundled

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const manifestName = "manifest.json"

// ErrEmptyBundle is returned when the embedded bundle has no entries.
// This is the placeholder state for a fresh `make init` before `make build-all`.
var ErrEmptyBundle = errors.New("bundled: bundle has no entries")

// EntryKind classifies a bundle entry's destination directory.
type EntryKind string

const (
	// KindPlugin entries are written to PluginsDir().
	KindPlugin EntryKind = "plugin"
	// KindPinentry entries are written to PinentryPath().
	KindPinentry EntryKind = "pinentry"
)

// Entry describes one binary inside a bundle.
type Entry struct {
	Name   string    `json:"name"`
	Kind   EntryKind `json:"kind"`
	SHA256 string    `json:"sha256"`
	Size   int64     `json:"size"`
}

// Manifest is the JSON descriptor inside a bundle zip.
type Manifest struct {
	BundleVersion    string  `json:"bundle_version"`
	LocksmithVersion string  `json:"locksmith_version"`
	Platform         string  `json:"platform"`
	Entries          []Entry `json:"entries"`
}

// Bundle is an opened bundle zip with its parsed manifest.
type Bundle struct {
	zip      *zip.Reader
	Manifest Manifest
}

// OpenBundle parses the platform-specific bundle bytes embedded into the
// locksmith binary. Returns ErrEmptyBundle if the manifest is empty.
func OpenBundle() (*Bundle, error) {
	return openFromBytes(bundleBytes)
}

func openFromBytes(data []byte) (*Bundle, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("opening bundle zip: %w", err)
	}
	var mf *zip.File
	for _, f := range zr.File {
		if f.Name == manifestName {
			mf = f
			break
		}
	}
	if mf == nil {
		return nil, fmt.Errorf("bundle has no %s", manifestName)
	}
	rc, err := mf.Open()
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", manifestName, err)
	}
	defer rc.Close()
	var m Manifest
	if err := json.NewDecoder(rc).Decode(&m); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", manifestName, err)
	}
	if len(m.Entries) == 0 {
		return nil, ErrEmptyBundle
	}
	return &Bundle{zip: zr, Manifest: m}, nil
}

// FindEntry returns the manifest entry with the given name, or false.
func (b *Bundle) FindEntry(name string) (Entry, bool) {
	for _, e := range b.Manifest.Entries {
		if e.Name == name {
			return e, true
		}
	}
	return Entry{}, false
}

// Open returns a ReadCloser for the entry's content within the zip.
func (b *Bundle) Open(name string) (io.ReadCloser, error) {
	for _, f := range b.zip.File {
		if f.Name == name {
			return f.Open()
		}
	}
	return nil, fmt.Errorf("entry %q not found in bundle", name)
}
