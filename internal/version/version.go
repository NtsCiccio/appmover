// Package version holds AppMover's build version.
package version

// Version is the running build's version, e.g. "1.3.0". It's set at build
// time via -ldflags "-X appmover/internal/version.Version=1.3.0"
// (.github/workflows/release.yml does this for tagged releases); a plain
// `go build` leaves it at "dev", which internal/update treats as "never
// check for updates" since there's no meaningful version to compare.
var Version = "dev"
