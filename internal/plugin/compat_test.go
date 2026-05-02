package plugin

import (
	"testing"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
)

func findKind(t *testing.T, ws []CompatWarning, k WarnKind) *CompatWarning {
	t.Helper()
	for i := range ws {
		if ws[i].Kind == k {
			return &ws[i]
		}
	}
	return nil
}

func TestValidate_InfoEmpty(t *testing.T) {
	v := &CompatValidator{Platform: "linux", LocksmithVersion: "1.0.0"}
	ws := v.Validate(&vaultv1.InfoResponse{})
	if len(ws) != 1 || ws[0].Kind != WarnInfoEmpty {
		t.Fatalf("want [WarnInfoEmpty], got %+v", ws)
	}
}

func TestValidate_PlatformMismatch(t *testing.T) {
	v := &CompatValidator{Platform: "linux", LocksmithVersion: "1.0.0"}
	info := &vaultv1.InfoResponse{
		Name:                "x",
		Platforms:           []string{"darwin"},
		MinLocksmithVersion: "1.0.0",
	}
	ws := v.Validate(info)
	if findKind(t, ws, WarnPlatformMismatch) == nil {
		t.Fatalf("expected WarnPlatformMismatch in %+v", ws)
	}
}

func TestValidate_PlatformMatch_NoWarn(t *testing.T) {
	v := &CompatValidator{Platform: "linux", LocksmithVersion: "1.0.0"}
	info := &vaultv1.InfoResponse{
		Name:                "x",
		Platforms:           []string{"linux", "darwin"},
		MinLocksmithVersion: "1.0.0",
	}
	ws := v.Validate(info)
	if findKind(t, ws, WarnPlatformMismatch) != nil {
		t.Fatalf("did not expect WarnPlatformMismatch in %+v", ws)
	}
}

func TestValidate_PlatformsEmpty_SkipsCheck(t *testing.T) {
	v := &CompatValidator{Platform: "linux", LocksmithVersion: "1.0.0"}
	info := &vaultv1.InfoResponse{Name: "x", MinLocksmithVersion: "1.0.0"}
	ws := v.Validate(info)
	if findKind(t, ws, WarnPlatformMismatch) != nil {
		t.Fatalf("did not expect WarnPlatformMismatch in %+v", ws)
	}
}

func TestValidate_MinVersionMissing(t *testing.T) {
	v := &CompatValidator{Platform: "linux", LocksmithVersion: "1.0.0"}
	info := &vaultv1.InfoResponse{Name: "x"}
	ws := v.Validate(info)
	if findKind(t, ws, WarnMinVersionMissing) == nil {
		t.Fatalf("expected WarnMinVersionMissing in %+v", ws)
	}
}

func TestValidate_InvalidMinVersion(t *testing.T) {
	v := &CompatValidator{Platform: "linux", LocksmithVersion: "1.0.0"}
	info := &vaultv1.InfoResponse{Name: "x", MinLocksmithVersion: "not-semver"}
	ws := v.Validate(info)
	if findKind(t, ws, WarnInvalidMinVersion) == nil {
		t.Fatalf("expected WarnInvalidMinVersion in %+v", ws)
	}
}

func TestValidate_DaemonTooOld(t *testing.T) {
	v := &CompatValidator{Platform: "linux", LocksmithVersion: "1.0.0"}
	info := &vaultv1.InfoResponse{Name: "x", MinLocksmithVersion: "2.0.0"}
	ws := v.Validate(info)
	if findKind(t, ws, WarnDaemonTooOld) == nil {
		t.Fatalf("expected WarnDaemonTooOld in %+v", ws)
	}
}

func TestValidate_InvalidMaxVersion(t *testing.T) {
	v := &CompatValidator{Platform: "linux", LocksmithVersion: "1.0.0"}
	info := &vaultv1.InfoResponse{
		Name:                "x",
		MinLocksmithVersion: "1.0.0",
		MaxLocksmithVersion: "garbage",
	}
	ws := v.Validate(info)
	if findKind(t, ws, WarnInvalidMaxVersion) == nil {
		t.Fatalf("expected WarnInvalidMaxVersion in %+v", ws)
	}
}

func TestValidate_DaemonTooNew(t *testing.T) {
	v := &CompatValidator{Platform: "linux", LocksmithVersion: "2.0.0"}
	info := &vaultv1.InfoResponse{
		Name:                "x",
		MinLocksmithVersion: "1.0.0",
		MaxLocksmithVersion: "1.5.0",
	}
	ws := v.Validate(info)
	if findKind(t, ws, WarnDaemonTooNew) == nil {
		t.Fatalf("expected WarnDaemonTooNew in %+v", ws)
	}
}

func TestValidate_AllOK(t *testing.T) {
	v := &CompatValidator{Platform: "linux", LocksmithVersion: "1.2.3"}
	info := &vaultv1.InfoResponse{
		Name:                "x",
		Platforms:           []string{"linux"},
		MinLocksmithVersion: "1.0.0",
		MaxLocksmithVersion: "2.0.0",
	}
	ws := v.Validate(info)
	if len(ws) != 0 {
		t.Fatalf("want no warnings, got %+v", ws)
	}
}

func TestValidate_InvalidLocalSemver(t *testing.T) {
	v := &CompatValidator{Platform: "linux", LocksmithVersion: "not-a-version"}
	info := &vaultv1.InfoResponse{
		Name:                "x",
		MinLocksmithVersion: "1.0.0",
		MaxLocksmithVersion: "2.0.0",
	}
	ws := v.Validate(info)
	if findKind(t, ws, WarnInvalidSemver) == nil {
		t.Fatalf("expected WarnInvalidSemver in %+v", ws)
	}
	if findKind(t, ws, WarnDaemonTooOld) != nil || findKind(t, ws, WarnDaemonTooNew) != nil {
		t.Fatalf("version checks must be skipped when local semver is invalid: %+v", ws)
	}
}

func TestWarnKindConstants(t *testing.T) {
	for _, k := range []WarnKind{
		WarnPlatformMismatch, WarnDaemonTooNew, WarnDaemonTooOld,
		WarnInfoUnavailable, WarnInfoEmpty, WarnMinVersionMissing,
		WarnInvalidMinVersion, WarnInvalidMaxVersion, WarnInvalidSemver,
	} {
		if string(k) == "" {
			t.Errorf("WarnKind constant must not be empty")
		}
	}
}
