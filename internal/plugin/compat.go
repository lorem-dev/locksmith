package plugin

import (
	"fmt"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
	"github.com/lorem-dev/locksmith/internal/semver"
)

// WarnKind classifies a compatibility warning.
type WarnKind string

// Warning kinds emitted by CompatValidator and Manager.Launch.
const (
	WarnPlatformMismatch  WarnKind = "platform_mismatch"
	WarnDaemonTooNew      WarnKind = "daemon_too_new"
	WarnDaemonTooOld      WarnKind = "daemon_too_old"
	WarnInfoUnavailable   WarnKind = "info_unavailable"
	WarnInfoEmpty         WarnKind = "info_empty"
	WarnMinVersionMissing WarnKind = "min_version_missing"
	WarnInvalidMinVersion WarnKind = "invalid_min_version"
	WarnInvalidMaxVersion WarnKind = "invalid_max_version"
	WarnInvalidSemver     WarnKind = "invalid_semver"
)

// CompatWarning describes one compatibility issue detected at plugin launch.
type CompatWarning struct {
	Kind    WarnKind
	Message string
}

// CompatValidator checks an InfoResponse against the running daemon's platform
// and locksmith version. It is a pure function over data: no I/O, no logging.
type CompatValidator struct {
	Platform         string // runtime.GOOS, e.g. "darwin"
	LocksmithVersion string // sdk/version.Current
}

// Validate returns a (possibly empty) list of warnings. Never returns an error.
func (v *CompatValidator) Validate(info *vaultv1.InfoResponse) []CompatWarning {
	var ws []CompatWarning

	// Step 0: empty Info check.
	if info == nil || info.Name == "" {
		return append(ws, CompatWarning{
			Kind:    WarnInfoEmpty,
			Message: "plugin Info() returned an empty response",
		})
	}

	// Step 1: platform check.
	if len(info.Platforms) > 0 {
		matched := false
		for _, p := range info.Platforms {
			if p == v.Platform {
				matched = true
				break
			}
		}
		if !matched {
			ws = append(ws, CompatWarning{
				Kind:    WarnPlatformMismatch,
				Message: fmt.Sprintf("plugin supports %v but running on %s", info.Platforms, v.Platform),
			})
		}
	}

	// Parse local version. If invalid, skip steps 2 and 3.
	current, err := semver.Parse(v.LocksmithVersion)
	if err != nil {
		ws = append(ws, CompatWarning{
			Kind:    WarnInvalidSemver,
			Message: fmt.Sprintf("local locksmith version %q is not valid semver: %v", v.LocksmithVersion, err),
		})
		return ws
	}

	// Step 2: min version check.
	if info.MinLocksmithVersion == "" {
		ws = append(ws, CompatWarning{
			Kind:    WarnMinVersionMissing,
			Message: "plugin does not declare min_locksmith_version",
		})
	} else {
		minV, perr := semver.Parse(info.MinLocksmithVersion)
		if perr != nil {
			ws = append(ws, CompatWarning{
				Kind:    WarnInvalidMinVersion,
				Message: fmt.Sprintf("min_locksmith_version %q is not valid semver: %v", info.MinLocksmithVersion, perr),
			})
		} else if current.Compare(minV) < 0 {
			ws = append(ws, CompatWarning{
				Kind:    WarnDaemonTooOld,
				Message: fmt.Sprintf("plugin min_locksmith_version=%s, current=%s", info.MinLocksmithVersion, v.LocksmithVersion),
			})
		}
	}

	// Step 3: max version check.
	if info.MaxLocksmithVersion != "" {
		maxV, perr := semver.Parse(info.MaxLocksmithVersion)
		if perr != nil {
			ws = append(ws, CompatWarning{
				Kind:    WarnInvalidMaxVersion,
				Message: fmt.Sprintf("max_locksmith_version %q is not valid semver: %v", info.MaxLocksmithVersion, perr),
			})
		} else if current.Compare(maxV) > 0 {
			ws = append(ws, CompatWarning{
				Kind:    WarnDaemonTooNew,
				Message: fmt.Sprintf("plugin max_locksmith_version=%s, current=%s", info.MaxLocksmithVersion, v.LocksmithVersion),
			})
		}
	}

	return ws
}
