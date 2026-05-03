// build-bundle-placeholder writes empty placeholder bundle zips so that
// `go build ./cmd/locksmith` works on a fresh clone without running the
// real build-bundle pipeline. The real bundles are created by
// .scripts/build-bundle.
package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type entry struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}
type manifest struct {
	BundleVersion    string  `json:"bundle_version"`
	LocksmithVersion string  `json:"locksmith_version"`
	Platform         string  `json:"platform"`
	Entries          []entry `json:"entries"`
}

func main() {
	dir := filepath.Join("internal", "bundled", "assets")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, p := range []string{"darwin", "linux"} {
		path := filepath.Join(dir, "bundle-"+p+".zip")
		if _, err := os.Stat(path); err == nil {
			continue
		}
		f, err := os.Create(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		zw := zip.NewWriter(f)
		w, _ := zw.Create("manifest.json")
		json.NewEncoder(w).Encode(manifest{Platform: p, Entries: []entry{}})
		zw.Close()
		f.Close()
	}
}
