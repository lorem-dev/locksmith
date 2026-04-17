package cli_test

import (
	"testing"

	"github.com/lorem-dev/locksmith/internal/cli"
)

func TestIsColorEnabled_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if cli.IsColorEnabled(false) {
		t.Error("expected color disabled when NO_COLOR is set")
	}
}

func TestIsColorEnabled_NotTTY(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	// isTTY will be false in test environment (no real stderr TTY)
	if cli.IsColorEnabled(false) {
		t.Error("expected color disabled when not a TTY")
	}
}

func TestBold(t *testing.T) {
	result := cli.Bold("hello", false) // color disabled
	if result != "hello" {
		t.Errorf("Bold() with color disabled = %q, want %q", result, "hello")
	}
}

func TestColorRed(t *testing.T) {
	result := cli.ColorRed("hello", false) // color disabled
	if result != "hello" {
		t.Errorf("ColorRed() with color disabled = %q, want %q", result, "hello")
	}
}

func TestIsColorEnabled_ForcedTTY(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	// isTTY=true overrides the TTY detection
	if !cli.IsColorEnabled(true) {
		t.Error("expected color enabled when isTTY=true and NO_COLOR unset")
	}
}

func TestBold_ColorEnabled(t *testing.T) {
	result := cli.Bold("hello", true)
	if result == "hello" {
		t.Error("Bold() with color enabled should wrap text with ANSI codes")
	}
}

func TestColorRed_ColorEnabled(t *testing.T) {
	result := cli.ColorRed("hello", true)
	if result == "hello" {
		t.Error("ColorRed() with color enabled should wrap text with ANSI codes")
	}
}

func TestColorYellow_ColorDisabled(t *testing.T) {
	result := cli.ColorYellow("hello", false)
	if result != "hello" {
		t.Errorf("ColorYellow() with color disabled = %q, want %q", result, "hello")
	}
}

func TestColorYellow_ColorEnabled(t *testing.T) {
	result := cli.ColorYellow("hello", true)
	if result == "hello" {
		t.Error("ColorYellow() with color enabled should wrap text with ANSI codes")
	}
}

func TestColorGray_ColorDisabled(t *testing.T) {
	result := cli.ColorGray("hello", false)
	if result != "hello" {
		t.Errorf("ColorGray() with color disabled = %q, want %q", result, "hello")
	}
}

func TestColorGray_ColorEnabled(t *testing.T) {
	result := cli.ColorGray("hello", true)
	if result == "hello" {
		t.Error("ColorGray() with color enabled should wrap text with ANSI codes")
	}
}
