//go:build darwin

package bundled

import _ "embed"

//go:embed assets/bundle-darwin.zip
var bundleBytes []byte
