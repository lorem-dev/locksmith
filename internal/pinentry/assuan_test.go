package pinentry

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun_Greeting(t *testing.T) {
	var out bytes.Buffer
	run(strings.NewReader("BYE\n"), &out, func(_, _ string) (string, error) {
		return "secret", nil
	})
	if !strings.HasPrefix(out.String(), "OK Pleased to meet you") {
		t.Errorf("expected greeting, got: %q", out.String())
	}
}

func TestRun_GetPin(t *testing.T) {
	var out bytes.Buffer
	input := "GETPIN\nBYE\n"
	run(strings.NewReader(input), &out, func(_, _ string) (string, error) {
		return "mysecret", nil
	})
	if !strings.Contains(out.String(), "D mysecret") {
		t.Errorf("expected D mysecret in output, got: %q", out.String())
	}
}

func TestRun_GetPin_Cancelled(t *testing.T) {
	var out bytes.Buffer
	input := "GETPIN\nBYE\n"
	run(strings.NewReader(input), &out, func(_, _ string) (string, error) {
		return "", errCancelled
	})
	if !strings.Contains(out.String(), "ERR 83886179") {
		t.Errorf("expected ERR 83886179 in output, got: %q", out.String())
	}
}

func TestRun_SetDescPrompt(t *testing.T) {
	var out bytes.Buffer
	var capturedDesc, capturedPrompt string
	input := "SETDESC Enter%20passphrase\nSETPROMPT Passphrase%3A\nGETPIN\nBYE\n"
	run(strings.NewReader(input), &out, func(desc, prompt string) (string, error) {
		capturedDesc = desc
		capturedPrompt = prompt
		return "pin", nil
	})
	if capturedDesc != "Enter passphrase" {
		t.Errorf("desc = %q, want %q", capturedDesc, "Enter passphrase")
	}
	if capturedPrompt != "Passphrase:" {
		t.Errorf("prompt = %q, want %q", capturedPrompt, "Passphrase:")
	}
}

func TestEncodeAssuan(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "plain"},
		{"pass%word", "pass%25word"},
		{"a\nb", "a%0Ab"},
		{"a\rb", "a%0Db"},
		{"a\x00b", "a%00b"},
		{"p%25q", "p%2525q"},
	}
	for _, tc := range cases {
		got := encodeAssuan(tc.in)
		if got != tc.want {
			t.Errorf("encodeAssuan(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRun_GetPin_EncodesPercent(t *testing.T) {
	var out bytes.Buffer
	input := "GETPIN\nBYE\n"
	run(strings.NewReader(input), &out, func(_, _ string) (string, error) {
		return "my%secret", nil
	})
	if !strings.Contains(out.String(), "D my%25secret") {
		t.Errorf("expected D my%%25secret in output, got: %q", out.String())
	}
}

func TestDecodePercent(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello", "hello"},
		{"hello%20world", "hello world"},
		{"foo%3Abar", "foo:bar"},
		{"a%0Ab", "a\nb"},
	}
	for _, tc := range cases {
		got := decodePercent(tc.in)
		if got != tc.want {
			t.Errorf("decodePercent(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	var out bytes.Buffer
	input := "UNKNOWNCMD\nBYE\n"
	run(strings.NewReader(input), &out, func(_, _ string) (string, error) {
		return "pin", nil
	})
	if !strings.Contains(out.String(), "ERR 536871187") {
		t.Errorf("expected ERR for unknown command, got: %q", out.String())
	}
}

func TestRun_NoopCommands(t *testing.T) {
	var out bytes.Buffer
	input := "NOP\nOPTION ttyname=/dev/tty\nSETTITLE title\nBYE\n"
	run(strings.NewReader(input), &out, func(_, _ string) (string, error) {
		return "pin", nil
	})
	// Each NOP/OPTION/SETTITLE should produce "OK"
	okCount := strings.Count(out.String(), "OK")
	// Greeting OK + NOP OK + OPTION OK + SETTITLE OK + BYE OK = 5
	if okCount < 4 {
		t.Errorf("expected at least 4 OK responses, got %d in: %q", okCount, out.String())
	}
}
