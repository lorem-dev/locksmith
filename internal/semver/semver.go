// Package semver implements minimal "major.minor.patch" parsing and comparison.
// It rejects pre-release suffixes and any leading "v" prefix.
package semver

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a parsed semantic version triple.
type Version struct {
	Major int
	Minor int
	Patch int
}

// Parse parses a "major.minor.patch" string with non-negative integers.
func Parse(s string) (Version, error) {
	if s == "" {
		return Version{}, fmt.Errorf("semver: empty string")
	}
	if strings.ContainsAny(s, "-+") {
		return Version{}, fmt.Errorf("semver: pre-release/build metadata not supported in %q", s)
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("semver: %q is not major.minor.patch", s)
	}
	out := Version{}
	for i, p := range parts {
		if p == "" {
			return Version{}, fmt.Errorf("semver: empty component in %q", s)
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return Version{}, fmt.Errorf("semver: component %d (%q) not an integer: %w", i, p, err)
		}
		if n < 0 {
			return Version{}, fmt.Errorf("semver: negative component %d in %q", i, s)
		}
		switch i {
		case 0:
			out.Major = n
		case 1:
			out.Minor = n
		case 2:
			out.Patch = n
		}
	}
	return out, nil
}

// Compare returns -1 if a < b, 0 if equal, 1 if a > b.
func (a Version) Compare(b Version) int {
	switch {
	case a.Major != b.Major:
		if a.Major < b.Major {
			return -1
		}
		return 1
	case a.Minor != b.Minor:
		if a.Minor < b.Minor {
			return -1
		}
		return 1
	case a.Patch != b.Patch:
		if a.Patch < b.Patch {
			return -1
		}
		return 1
	}
	return 0
}

// LessOrEqual reports whether a <= b.
func (a Version) LessOrEqual(b Version) bool { return a.Compare(b) <= 0 }

// GreaterOrEqual reports whether a >= b.
func (a Version) GreaterOrEqual(b Version) bool { return a.Compare(b) >= 0 }
