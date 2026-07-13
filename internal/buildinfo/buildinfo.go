// Package buildinfo carries build-time metadata for the envkeep CLI.
//
// Version is overridden at build time with:
//
//	go build -ldflags "-X github.com/carvalhosauro/envkeep/internal/buildinfo.Version=v1.2.3"
package buildinfo

// Version is the envkeep version. It defaults to "dev" for local builds and is
// stamped with the release tag by the build pipeline.
var Version = "dev"
