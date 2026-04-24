//go:build darwin

package pinentry

import (
	"os/exec"
	"testing"
)

func TestParseOsascriptOutput_MissingPrefix(t *testing.T) {
	pin, ok := parseOsascriptOutput("unexpected output format")
	if ok {
		t.Errorf("expected ok=false for malformed output")
	}
	if pin != "" {
		t.Errorf("expected empty pin, got %q", pin)
	}
}

func TestParseOsascriptOutput_ValidOutput(t *testing.T) {
	pin, ok := parseOsascriptOutput("button returned:OK, text returned:mysecret")
	if !ok {
		t.Errorf("expected ok=true for valid output")
	}
	if pin != "mysecret" {
		t.Errorf("got %q, want %q", pin, "mysecret")
	}
}

func TestParseOsascriptOutput_EmptyPin(t *testing.T) {
	pin, ok := parseOsascriptOutput("button returned:OK, text returned:")
	if !ok {
		t.Errorf("expected ok=true for valid output with empty pin")
	}
	if pin != "" {
		t.Errorf("got %q, want empty string", pin)
	}
}

// TestTryGUI_ExecFails exercises the err != nil path by injecting a cmd that
// returns an error.
func TestTryGUI_ExecFails(t *testing.T) {
	if _, err := exec.LookPath("osascript"); err != nil {
		t.Skip("osascript not found")
	}

	pin, err := tryGUIWithCmd("desc", "Prompt", func(script string) ([]byte, error) {
		return nil, &exec.ExitError{}
	})
	if err != errCancelled {
		t.Errorf("err = %v, want errCancelled", err)
	}
	if pin != "" {
		t.Errorf("pin = %q, want empty", pin)
	}
}

// TestTryGUI_MalformedOutput exercises the !ok path when output is unexpected.
func TestTryGUI_MalformedOutput(t *testing.T) {
	pin, err := tryGUIWithCmd("desc", "Prompt", func(script string) ([]byte, error) {
		return []byte("something unexpected\n"), nil
	})
	if err != errCancelled {
		t.Errorf("err = %v, want errCancelled", err)
	}
	if pin != "" {
		t.Errorf("pin = %q, want empty", pin)
	}
}
