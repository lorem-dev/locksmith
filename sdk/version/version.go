// Package version exposes the locksmith version, embedded at build
// time from sdk/version/VERSION via //go:embed. The embedded value is
// available regardless of how the binary was produced (`go install`,
// `go build`, `make build-all`).
package version

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var versionFile string

// Current is the locksmith version this build was produced from.
var Current = strings.TrimSpace(versionFile)
