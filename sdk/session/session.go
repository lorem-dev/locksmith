// Package session provides session ID masking utilities for locksmith SDK consumers.
package session

import sdklog "github.com/lorem-dev/locksmith/sdk/log"

// HideSessionId masks a session ID for display in non-log contexts
// (RPC responses, CLI output). Always masks regardless of log level.
func HideSessionId(sessionId string) string {
	if len(sessionId) < 15 {
		return "****"
	}
	if len(sessionId) < 30 {
		return sessionId[:5] + "****" + sessionId[len(sessionId)-4:]
	}
	return sessionId[:5] + "****" + sessionId[len(sessionId)-10:]
}

// MaskSessionId masks a session ID for log output.
// Returns the full ID when debug logging is active, masked otherwise.
// Use exclusively at zerolog call sites.
func MaskSessionId(id string) string {
	if sdklog.IsDebug() {
		return id
	}
	return HideSessionId(id)
}
