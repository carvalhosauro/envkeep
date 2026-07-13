// Package buildinfo carries build-time metadata for the envkeep CLI.
//
// Version is overridden at build time with:
//
//	go build -ldflags "-X github.com/carvalhosauro/envkeep/internal/buildinfo.Version=v1.2.3"
package buildinfo

import "runtime/debug"

// Version is the envkeep version. Precedence: an -ldflags value wins; otherwise,
// for `go install ...@version` builds, it is filled from the module version in
// the embedded build info; otherwise it stays "dev".
var Version = "dev"

func init() {
	if Version != "dev" {
		return // set via -ldflags
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			Version = v
		}
	}
}
