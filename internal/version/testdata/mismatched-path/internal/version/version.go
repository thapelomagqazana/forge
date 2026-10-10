// Package version is a minimal stand-in for the real
// internal/version package. It exists only to demonstrate the
// linker-prefix mismatch failure mode.
package version

// These variables mirror the real package's linker targets.
var (
	Version   string
	Commit    string
	BuildDate string
	Dirty     string
)

// Get mirrors the real package's accessor.
func Get() (string, string, string, string) {
	return Version, Commit, BuildDate, Dirty
}
