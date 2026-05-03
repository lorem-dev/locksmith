//go:build linux

package bundled

import _ "embed"

//go:embed assets/bundle-linux.zip
var bundleBytes []byte
