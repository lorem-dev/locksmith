// Package version exposes the current locksmith version as a constant.
// At release time the value is overridden via:
//
//	-ldflags '-X github.com/lorem-dev/locksmith/sdk/version.Current=X.Y.Z'
//
// The source value is used for tests and local builds only.
package version

// Current is the locksmith version this build was produced from.
const Current = "0.1.0"
