package version_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/lorem-dev/locksmith/sdk/version"
)

func TestCurrent_NonEmpty(t *testing.T) {
	if version.Current == "" {
		t.Fatal("Current must not be empty")
	}
}

func TestCurrent_ValidSemver(t *testing.T) {
	parts := strings.Split(version.Current, ".")
	if len(parts) != 3 {
		t.Fatalf("Current = %q, want major.minor.patch", version.Current)
	}
	for i, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			t.Errorf("part %d (%q) is not an integer: %v", i, p, err)
		}
	}
}
