// Package version exposes the build-time version string.
//
// The value is intentionally a var so `go build -ldflags "-X ...Version=..."`
// can override it in release builds; dev builds fall back to "dev".
package version

var (
	// Version is the semver string, set at build time in release pipelines.
	Version = "dev"
	// Commit is the short git SHA, set at build time.
	Commit = ""
	// Date is the build timestamp in RFC3339, set at build time.
	Date = ""
)

// String returns a human-readable version line for `kennel version`.
func String() string {
	if Commit == "" {
		return Version
	}
	if Date == "" {
		return Version + " (" + Commit + ")"
	}
	return Version + " (" + Commit + ", " + Date + ")"
}
