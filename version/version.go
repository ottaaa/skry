// Package version holds the release version and build metadata for skry.
// Version is bumped by tagpr on each release PR; Revision is injected at
// build time via -ldflags "-X .../version.Revision=<sha>".
package version

// Version is the semantic version of skry. tagpr rewrites this constant as
// part of the release PR, so any change here should be made through tagpr
// rather than by hand.
const Version = "0.1.0"

// Revision is the short git SHA of the build. Populated via -ldflags at
// build/release time; left empty for `go run` / unreleased checkouts.
var Revision = ""

// String returns a human-readable "Version (Revision)" or just Version when
// Revision is empty.
func String() string {
	if Revision == "" {
		return Version
	}
	return Version + " (" + Revision + ")"
}
