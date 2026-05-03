// internal/bundled/extract.go
package bundled

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ConflictResolution is the prompter's reply to a sha256 mismatch.
type ConflictResolution int

const (
	// Overwrite replaces the existing file with the bundled version.
	Overwrite ConflictResolution = iota
	// Keep leaves the existing file untouched and emits a warning.
	Keep
	// OverwriteAll behaves like Overwrite and applies to all subsequent
	// conflicting entries within the same Extract call.
	OverwriteAll
	// KeepAll behaves like Keep and applies to all subsequent conflicting
	// entries within the same Extract call.
	KeepAll
)

// ErrSHAMismatch is returned when the on-disk sha256 after writing does not
// match the manifest's expected sha256, indicating bundle corruption.
var ErrSHAMismatch = errors.New("bundled: sha256 mismatch after extract")

// ShortSHALen is the number of hex characters used when displaying a truncated
// SHA256 digest in user-facing output (e.g. conflict prompts).
const ShortSHALen = 8

// ShortSHA returns the first ShortSHALen characters of a hex SHA256 string,
// or the full string if it is shorter than ShortSHALen.
func ShortSHA(s string) string {
	if len(s) > ShortSHALen {
		return s[:ShortSHALen]
	}
	return s
}

// ExtractPrompter resolves sha256 mismatches during Extract.
type ExtractPrompter interface {
	// BundleExtractPrompt is called when an existing file has a different
	// sha256 from the manifest entry. existingSHA and newSHA are hex strings.
	BundleExtractPrompt(name, existingSHA, newSHA string) (ConflictResolution, error)
}

// ExtractOptions controls Extract behaviour.
type ExtractOptions struct {
	// Names lists entries to extract. Order is preserved.
	Names []string
	// PluginsDir is the destination directory for entries with Kind=plugin.
	PluginsDir string
	// PinentryPath is the destination FILE path for the entry with
	// Kind=pinentry (not a directory).
	PinentryPath string
	// Prompter resolves sha256 mismatches. Nil defaults to Keep with warning.
	Prompter ExtractPrompter
	// ForceOverwrite ignores Prompter and overwrites all mismatches.
	ForceOverwrite bool
	// OnKept is called for each entry that was NOT written. withWarning is
	// true when the user (or default policy) chose Keep on a mismatch;
	// false when the existing file already matches and was silently skipped.
	OnKept func(name string, withWarning bool)
	// OnExtracted is called for each entry that WAS written.
	OnExtracted func(name string)
}

// Extract writes selected entries from b to disk per opts.
//
//nolint:gocognit // inherent linear scan: each entry has lookup, hash-check, conflict, write, callback branches
func Extract(b *Bundle, opts ExtractOptions) error {
	const noSticky ConflictResolution = -1
	sticky := noSticky
	for _, name := range opts.Names {
		entry, ok := b.FindEntry(name)
		if !ok {
			return fmt.Errorf("bundle has no entry %q", name)
		}
		dest, err := destPathFor(entry, opts)
		if err != nil {
			return err
		}
		existingSHA, exists, err := FileSHA256(dest)
		if err != nil {
			return err
		}
		if exists && existingSHA == entry.SHA256 {
			if opts.OnKept != nil {
				opts.OnKept(name, false)
			}
			continue
		}
		if exists && !opts.ForceOverwrite {
			keep, newSticky, resolveErr := opts.resolveConflict(name, existingSHA, entry.SHA256, sticky)
			if resolveErr != nil {
				return resolveErr
			}
			if newSticky != noSticky {
				sticky = newSticky
			}
			if keep {
				if opts.OnKept != nil {
					opts.OnKept(name, true)
				}
				continue
			}
		}
		if err := writeEntry(b, entry, dest); err != nil {
			return err
		}
		if opts.OnExtracted != nil {
			opts.OnExtracted(name)
		}
	}
	return nil
}

// resolveConflict decides what to do when an existing file has a different
// sha256 from the manifest entry. It returns keep=true when the existing file
// should be left untouched, and newSticky with the updated sticky resolution
// (-1 means no change). The caller must update sticky when newSticky != -1.
func (opts ExtractOptions) resolveConflict(
	name, existingSHA, newSHA string,
	sticky ConflictResolution,
) (keep bool, newSticky ConflictResolution, err error) {
	const noSticky ConflictResolution = -1
	res := sticky
	if res == noSticky {
		if opts.Prompter == nil {
			res = Keep
		} else {
			r, perr := opts.Prompter.BundleExtractPrompt(name, existingSHA, newSHA)
			if perr != nil {
				return false, noSticky, fmt.Errorf("prompt for %q: %w", name, perr)
			}
			res = r
			if r == OverwriteAll || r == KeepAll {
				return res == Keep || res == KeepAll, r, nil
			}
		}
	}
	return res == Keep || res == KeepAll, noSticky, nil
}

func destPathFor(e Entry, opts ExtractOptions) (string, error) {
	switch e.Kind {
	case KindPlugin:
		return filepath.Join(opts.PluginsDir, e.Name), nil
	case KindPinentry:
		return opts.PinentryPath, nil
	default:
		return "", fmt.Errorf("unknown entry kind %q", e.Kind)
	}
}

// FileSHA256 returns the hex sha256 of the file at path, plus whether the
// file exists. Exported so callers can implement dry-run / diff logic
// without duplicating the hash code.
func FileSHA256(path string) (string, bool, error) {
	f, err := os.Open(path) //nolint:gosec // G304: path is internal, not user-controlled
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck // read-only open; close error not actionable here
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", true, fmt.Errorf("hashing %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), true, nil
}

func writeEntry(b *Bundle, e Entry, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dest), err)
	}
	rc, err := b.Open(e.Name)
	if err != nil {
		return err
	}
	defer rc.Close() //nolint:errcheck // zip entry reader; close error not actionable in defer
	tmp := dest + ".tmp"
	//nolint:gosec // G302: extracted binaries must be executable (0o755)
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("creating %s: %w", tmp, err)
	}
	h := sha256.New()
	w := io.MultiWriter(f, h)
	if _, err := io.Copy(w, rc); err != nil {
		f.Close()      //nolint:errcheck // closing after write failure; original error takes precedence
		os.Remove(tmp) //nolint:errcheck // best-effort cleanup of temp file
		return fmt.Errorf("writing %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp) //nolint:errcheck // best-effort cleanup of temp file
		return fmt.Errorf("closing %s: %w", tmp, err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != e.SHA256 {
		os.Remove(tmp) //nolint:errcheck // best-effort cleanup of temp file after integrity failure
		return fmt.Errorf("%w: %s: got %s want %s", ErrSHAMismatch, e.Name, got, e.SHA256)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("renaming %s -> %s: %w", tmp, dest, err)
	}
	return nil
}
