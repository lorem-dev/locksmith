// build-bundle reads bin/ for plugin and pinentry binaries and writes
// internal/bundled/assets/bundle-<goos>.zip plus an embedded manifest.json.
//
// Inputs (relative to workspace root):
//
//	bin/locksmith-pinentry
//	bin/locksmith-plugin-*
//
// On linux, bin/locksmith-plugin-keychain is skipped (it is built from
// provider_stub.go and would only return "platform not supported").
//
// Usage: go run ./.scripts/build-bundle
package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	pluginPrefix = "locksmith-plugin-"
	pinentryName = "locksmith-pinentry"
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
	root, err := workspaceRoot(".")
	if err != nil {
		die("finding workspace root: %v", err)
	}
	goos := runtime.GOOS
	if goos != "darwin" && goos != "linux" {
		die("unsupported GOOS %s", goos)
	}
	binDir := filepath.Join(root, "bin")
	assetsDir := filepath.Join(root, "internal", "bundled", "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		die("mkdir assets: %v", err)
	}
	out := filepath.Join(assetsDir, "bundle-"+goos+".zip")
	version := readVersion(root)

	files, err := collectFiles(binDir, goos)
	if err != nil {
		die("collecting files: %v", err)
	}
	if len(files) == 0 {
		die("no plugin or pinentry binaries found in %s", binDir)
	}

	if err := writeBundle(out, files, manifest{
		BundleVersion:    version,
		LocksmithVersion: version,
		Platform:         goos,
	}); err != nil {
		die("writing %s: %v", out, err)
	}
	fmt.Printf("wrote %s (%d entries)\n", out, len(files))
}

type binEntry struct {
	path string
	name string
	kind string
}

func collectFiles(binDir, goos string) ([]binEntry, error) {
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return nil, err
	}
	var out []binEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		switch {
		case name == pinentryName:
			out = append(out, binEntry{path: filepath.Join(binDir, name), name: name, kind: "pinentry"})
		case strings.HasPrefix(name, pluginPrefix):
			if goos == "linux" && name == "locksmith-plugin-keychain" {
				continue // stub on non-darwin; skip
			}
			out = append(out, binEntry{path: filepath.Join(binDir, name), name: name, kind: "plugin"})
		}
	}
	return out, nil
}

func writeBundle(out string, files []binEntry, mf manifest) error {
	tmp := out + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	for _, fe := range files {
		sum, size, err := hashAndCopy(zw, fe)
		if err != nil {
			zw.Close()
			f.Close()
			os.Remove(tmp)
			return err
		}
		mf.Entries = append(mf.Entries, entry{Name: fe.name, Kind: fe.kind, SHA256: sum, Size: size})
	}
	mfw, err := zw.Create("manifest.json")
	if err != nil {
		zw.Close()
		f.Close()
		os.Remove(tmp)
		return err
	}
	enc := json.NewEncoder(mfw)
	enc.SetIndent("", "  ")
	if err := enc.Encode(mf); err != nil {
		zw.Close()
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := zw.Close(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, out)
}

func hashAndCopy(zw *zip.Writer, e binEntry) (string, int64, error) {
	src, err := os.Open(e.path)
	if err != nil {
		return "", 0, err
	}
	defer src.Close()
	dst, err := zw.Create(e.name)
	if err != nil {
		return "", 0, err
	}
	h := sha256.New()
	w := io.MultiWriter(dst, h)
	n, err := io.Copy(w, src)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func readVersion(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "sdk", "version", "version.go"))
	if err != nil {
		return "0.0.0-dev"
	}
	const marker = `Current = "`
	i := strings.Index(string(data), marker)
	if i < 0 {
		return "0.0.0-dev"
	}
	rest := string(data)[i+len(marker):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return "0.0.0-dev"
	}
	return rest[:j]
}

func workspaceRoot(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, "go.work")); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("go.work not found")
		}
		abs = parent
	}
}

func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "build-bundle: "+format+"\n", a...)
	os.Exit(1)
}
