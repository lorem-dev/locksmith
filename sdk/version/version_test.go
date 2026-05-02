package version

import (
	"strconv"
	"strings"
	"testing"
)

func TestCurrent_NonEmpty(t *testing.T) {
	if Current == "" {
		t.Fatal("Current must not be empty")
	}
}

func TestCurrent_ValidSemver(t *testing.T) {
	parts := strings.Split(Current, ".")
	if len(parts) != 3 {
		t.Fatalf("Current = %q, want major.minor.patch", Current)
	}
	for i, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			t.Errorf("part %d (%q) is not an integer: %v", i, p, err)
		}
	}
}
