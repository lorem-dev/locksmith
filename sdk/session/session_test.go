package session_test

import (
	"strings"
	"testing"

	sdklog "github.com/lorem-dev/locksmith/sdk/log"
	sdksession "github.com/lorem-dev/locksmith/sdk/session"
)

func TestHideSessionId_Short(t *testing.T) {
	if got := sdksession.HideSessionId("abc"); got != "****" {
		t.Errorf("got %q, want ****", got)
	}
}

func TestHideSessionId_Medium(t *testing.T) {
	id := "abcdefghijklmnop" // 16 chars (>=15, <30)
	if want := "abcde****mnop"; sdksession.HideSessionId(id) != want {
		t.Errorf("HideSessionId(%q) = %q, want %q", id, sdksession.HideSessionId(id), want)
	}
}

func TestHideSessionId_Long(t *testing.T) {
	id := "ls_" + strings.Repeat("a", 64) // 67 chars (>=30)
	got := sdksession.HideSessionId(id)
	if got == id {
		t.Errorf("HideSessionId(%q) should mask, returned unchanged", id)
	}
	if !strings.HasPrefix(got, "ls_aa") {
		t.Errorf("masked ID should start with first 5 chars, got %q", got)
	}
	if !strings.HasSuffix(got, strings.Repeat("a", 10)) {
		t.Errorf("masked ID should end with last 10 chars, got %q", got)
	}
}

func TestMaskSessionId_NonDebug(t *testing.T) {
	_, _ = sdklog.NewLogWriter(sdklog.LogConfig{Level: "info"})
	id := "ls_" + strings.Repeat("x", 64)
	got := sdksession.MaskSessionId(id)
	if got == id {
		t.Errorf("MaskSessionId should mask at info level, got full ID %q", got)
	}
}

func TestMaskSessionId_Debug(t *testing.T) {
	_, _ = sdklog.NewLogWriter(sdklog.LogConfig{Level: "debug"})
	id := "ls_" + strings.Repeat("x", 64)
	got := sdksession.MaskSessionId(id)
	if got != id {
		t.Errorf("MaskSessionId in debug mode should return full ID, got %q", got)
	}
	// Reset
	_, _ = sdklog.NewLogWriter(sdklog.LogConfig{Level: "info"})
}

func TestHideSessionId_AlwaysMasks(t *testing.T) {
	_, _ = sdklog.NewLogWriter(sdklog.LogConfig{Level: "debug"})
	id := "ls_" + strings.Repeat("z", 64)
	got := sdksession.HideSessionId(id)
	if got == id {
		t.Errorf("HideSessionId should always mask even in debug mode, got full ID")
	}
	// Reset
	_, _ = sdklog.NewLogWriter(sdklog.LogConfig{Level: "info"})
}
