// Package version holds the running build's version string.
package version

// Version is set at build time via
// -ldflags "-X github.com/deranjer/cookira/internal/version.Version=...".
// Left as "dev" for local/non-release builds.
var Version = "dev"
