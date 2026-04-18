package cli_test

import (
	"testing"

	"github.com/lorem-dev/locksmith/internal/cli"
)

func TestIsNoColor_NoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if !cli.IsNoColor() {
		t.Error("expected IsNoColor() = true when NO_COLOR is set")
	}
}

func TestIsNoColor_NotTTY(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	// In the test environment stderr is not a real TTY, so IsNoColor returns true.
	if !cli.IsNoColor() {
		t.Error("expected IsNoColor() = true when stderr is not a TTY")
	}
}
