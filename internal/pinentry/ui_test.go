package pinentry

import (
	"errors"
	"os"
	"testing"
)

// TestHexVal exercises all branches of the hexVal helper.
func TestHexVal(t *testing.T) {
	cases := []struct {
		in   byte
		want int
	}{
		{'0', 0},
		{'9', 9},
		{'a', 10},
		{'f', 15},
		{'A', 10},
		{'F', 15},
		{'g', -1},
		{'z', -1},
		{'G', -1},
		{'!', -1},
	}
	for _, tc := range cases {
		got := hexVal(tc.in)
		if got != tc.want {
			t.Errorf("hexVal(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// noGUI is a GUI function stub that always fails.
func noGUI(_, _ string) (string, error) { return "", errCancelled }

// fakeTTY returns an openTTY function that provides a writable temp file as the
// TTY. This lets tests exercise the TTY path without a real controlling terminal.
func fakeTTY(t *testing.T) func() (*os.File, error) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "fake-tty-*.txt")
	if err != nil {
		t.Fatalf("create fake tty: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return func() (*os.File, error) { return f, nil }
}

// brokenTTY returns an openTTY function that always fails.
func brokenTTY() func() (*os.File, error) {
	return func() (*os.File, error) { return nil, errors.New("no tty") }
}

// TestGetPassword_GUISuccess verifies that a successful GUI response is returned immediately.
func TestGetPassword_GUISuccess(t *testing.T) {
	mockGUI := func(_, _ string) (string, error) { return "guipin", nil }
	pin, err := getPassword("desc", "Prompt", mockGUI, brokenTTY(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pin != "guipin" {
		t.Errorf("pin = %q, want %q", pin, "guipin")
	}
}

// TestGetPassword_DefaultPrompt verifies that an empty prompt is replaced with "Passphrase".
func TestGetPassword_DefaultPrompt(t *testing.T) {
	var capturedPrompt string
	mockGUI := func(_, prompt string) (string, error) {
		capturedPrompt = prompt
		return "pin", nil
	}
	_, _ = getPassword("desc", "", mockGUI, brokenTTY(), nil)
	if capturedPrompt != "Passphrase" {
		t.Errorf("prompt = %q, want %q", capturedPrompt, "Passphrase")
	}
}

// TestGetPassword_TTYSuccess verifies TTY fallback when GUI fails and readFn succeeds.
func TestGetPassword_TTYSuccess(t *testing.T) {
	mockRead := func(_ int) ([]byte, error) { return []byte("ttypin"), nil }
	pin, err := getPassword("desc", "Prompt", noGUI, fakeTTY(t), mockRead)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pin != "ttypin" {
		t.Errorf("pin = %q, want %q", pin, "ttypin")
	}
}

// TestGetPassword_TTYReadError verifies that a readFn failure returns errCancelled.
func TestGetPassword_TTYReadError(t *testing.T) {
	mockRead := func(_ int) ([]byte, error) { return nil, errors.New("read error") }
	_, err := getPassword("desc", "Prompt", noGUI, fakeTTY(t), mockRead)
	if !errors.Is(err, errCancelled) {
		t.Errorf("err = %v, want errCancelled", err)
	}
}

// TestGetPassword_NoTTY verifies that errCancelled is returned when the TTY
// cannot be opened (both GUI and TTY unavailable).
func TestGetPassword_NoTTY(t *testing.T) {
	_, err := getPassword("desc", "Prompt", noGUI, brokenTTY(), nil)
	if !errors.Is(err, errCancelled) {
		t.Errorf("err = %v, want errCancelled", err)
	}
}
