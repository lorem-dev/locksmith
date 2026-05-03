package version

import (
	"os"
	"strings"
	"testing"
)

func TestCurrent_MatchesVERSIONFile(t *testing.T) {
	data, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatalf("read VERSION: %v", err)
	}
	want := strings.TrimSpace(string(data))
	if Current != want {
		t.Errorf("Current = %q, want %q", Current, want)
	}
}

func TestCurrent_NotEmpty(t *testing.T) {
	if Current == "" {
		t.Fatal("Current must not be empty")
	}
}

func TestCurrent_NoVPrefix(t *testing.T) {
	if strings.HasPrefix(Current, "v") {
		t.Errorf("Current = %q must not have v-prefix; tags carry the v, the file does not", Current)
	}
}
