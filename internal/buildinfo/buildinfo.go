// Package buildinfo carries build-time metadata for the envkeep CLI.
//
// Version is overridden at build time with:
//
//	go build -ldflags "-X github.com/carvalhosauro/envkeep/internal/buildinfo.Version=v1.2.3"
package buildinfo

import "runtime/debug"

// devVersion is the fallback version when none is injected or embedded.
const devVersion = "dev"

// goDevelPlaceholder is the version the Go toolchain records for a plain
// `go build` with no module version; envkeep treats it as devVersion.
const goDevelPlaceholder = "(devel)"

// Version is the envkeep version. Precedence: an -ldflags value wins; otherwise,
// for `go install ...@version` builds, it is filled from the module version in
// the embedded build info; otherwise it stays devVersion.
var Version = devVersion

func init() {
	if Version != devVersion {
		return // set via -ldflags
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v := bi.Main.Version; v != "" && v != goDevelPlaceholder {
			Version = v
		}
	}
}
