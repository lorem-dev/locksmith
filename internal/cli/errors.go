package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// hints maps gRPC status codes to actionable hint messages.
var hints = map[codes.Code]string{
	codes.NotFound:         "check that the key path and service name are correct",
	codes.PermissionDenied: "access denied - check vault permissions",
	codes.Unauthenticated:  "GPG passphrase required but no UI available - see docs/configuration.md#gpg-pinentry",
	codes.Unavailable:      "vault plugin failed to start - re-run with --log-level debug",
	codes.InvalidArgument:  "invalid key configuration - check vault and path in config.yaml",
	codes.DeadlineExceeded: "vault plugin timed out - check if the vault service is reachable",
	codes.Unimplemented:    "this vault does not support the requested operation",
	codes.Internal:         "unexpected vault error - re-run with --log-level debug for full details",
	codes.Unknown:          "unexpected error - re-run with --log-level debug for full details",
}

// FormatErrorParts returns the human-readable error message and hint (may be empty).
// For gRPC status errors it returns the desc only (no "rpc error: code = X desc =" prefix).
// Exported for testing.
func FormatErrorParts(err error) (msg, hint string) {
	if s, ok := status.FromError(err); ok && s.Code() != codes.OK {
		msg = s.Message()
		hint = hints[s.Code()]
		return msg, hint
	}
	return err.Error(), ""
}

// PrintError prints a formatted error (and optional hint) to stderr.
// Uses ANSI color when stderr is a TTY and NO_COLOR is not set.
func PrintError(err error) {
	msg, hint := FormatErrorParts(err)

	fmt.Fprintf(os.Stderr, "%s %s\n", color.New(color.FgRed, color.Bold).Sprint("Error:"), msg)
	if hint != "" {
		fmt.Fprintf(
			os.Stderr,
			"%s %s\n",
			color.New(color.FgYellow, color.Bold).Sprint("Hint:"),
			color.New(color.FgHiBlack).Sprint(hint),
		)
	}
}
