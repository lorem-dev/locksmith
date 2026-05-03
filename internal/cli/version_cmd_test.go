package cli

import (
	"bytes"
	"testing"

	sdkversion "github.com/lorem-dev/locksmith/sdk/version"
)

func TestVersionCmd_PrintsCurrent(t *testing.T) {
	cmd := newVersionCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	want := sdkversion.Current + "\n"
	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestVersionCmd_Use(t *testing.T) {
	cmd := newVersionCmd()
	if cmd.Use != "version" {
		t.Errorf("Use = %q, want %q", cmd.Use, "version")
	}
	if cmd.Short == "" {
		t.Errorf("Short must not be empty")
	}
}
