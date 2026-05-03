package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
)

// renderHealth replicates the rendering loop in the health command for testing.
func renderHealth(w *bytes.Buffer, resp *locksmithv1.VaultHealthResponse) {
	for _, v := range resp.Vaults {
		status := "UNAVAILABLE"
		if v.Available {
			status = "OK"
		}
		fmt.Fprintf(w, "%-20s %s  %s\n", v.Name, status, v.Message)
		for _, cw := range v.CompatWarnings {
			fmt.Fprintf(w, "  ! %s\n", cw)
		}
	}
}

func TestRenderHealth_PrintsWarningsWithBangPrefix(t *testing.T) {
	resp := &locksmithv1.VaultHealthResponse{
		Vaults: []*locksmithv1.VaultHealthInfo{
			{
				Name:      "gopass",
				Available: true,
				Message:   "gopass available",
				CompatWarnings: []string{
					"platform_mismatch: plugin supports [darwin] but running on linux",
					"daemon_too_new: plugin max_locksmith_version=1.2.0, current=1.3.0",
				},
			},
		},
	}
	var buf bytes.Buffer
	renderHealth(&buf, resp)
	out := buf.String()
	if !strings.Contains(out, "  ! platform_mismatch:") {
		t.Errorf("missing first warning with `  ! ` prefix; got:\n%s", out)
	}
	if !strings.Contains(out, "  ! daemon_too_new:") {
		t.Errorf("missing second warning with `  ! ` prefix; got:\n%s", out)
	}
}

func TestRenderHealth_NoWarnings(t *testing.T) {
	resp := &locksmithv1.VaultHealthResponse{
		Vaults: []*locksmithv1.VaultHealthInfo{
			{Name: "keychain", Available: true, Message: "ok"},
		},
	}
	var buf bytes.Buffer
	renderHealth(&buf, resp)
	out := buf.String()
	if strings.Contains(out, "!") {
		t.Errorf("expected no warnings in output, got:\n%s", out)
	}
}

func TestNewVaultCmd_HasHealthSubcommand(t *testing.T) {
	cmd := newVaultCmd()
	for _, sub := range cmd.Commands() {
		if sub.Use == "health" {
			return
		}
	}
	t.Fatal("vault command missing health subcommand")
}
