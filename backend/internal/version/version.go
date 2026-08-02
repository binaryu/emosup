// Package version holds the emosup release version.
//
// Set at build time via:
//
//	go build -ldflags "-X emosup/backend/internal/version.Version=v1.2.4"
package version

// Version is the emosup release version; "dev" for local builds.
var Version = "dev"
