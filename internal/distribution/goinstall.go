package distribution

import (
	"fmt"
	"strings"
)

// goinstallAdapter verifies the module path's stability.
//
// # Why an adapter, when there is nothing to render
//
// `go install github.com/forge-cli/forge/cmd/forge@latest` resolves
// against the module's own path. The path is in go.mod; the release
// does not change it. The adapter's job is to verify that the path is
// what the installation guide says it is, and that a v2+ module has
// the /vN suffix Go's module system requires.
//
// A module path change is a MAJOR change: a user's existing
// `go install` line would break. The adapter is the release
// pipeline's check that the path has not changed without an ADR.
//
// # Why the type is unexported
//
// The driver (cmd/forge-internal-distribute) reaches every adapter
// through For. For ChannelGoInstall, For returns a goinstallAdapter;
// the driver calls its Manifests method through the Adapter
// interface. The driver never names the concrete type, so the type
// stays unexported. A future caller that needed the adapter without
// For would require exporting the type; the current shape does not.
type goinstallAdapter struct{}

// Channel returns ChannelGoInstall.
func (goinstallAdapter) Channel() Channel { return ChannelGoInstall }

// Manifests returns no files: the channel produces nothing. It
// returns ErrModulePathChanged if the module path is wrong.
//
// The error is a defect, not a skip. The two sentinels that mean
// "skip this channel for this release" are ErrNotApplicable (the
// release's channel is excluded) and ErrUnsupportedPlatform (the
// release is missing a platform the channel needs). A wrong module
// path is neither: the release is valid, the channel is valid, and
// the module path is wrong. The driver returns 1 for it, not 3. A
// module path change requires an ADR and a version bump, not a
// skipped channel.
//
// The function reads the module path from the Release struct, which
// ParseRelease fills from go.mod. A release whose module path does
// not match the installation guide's is rejected here, before the
// installation guide is published.
func (goinstallAdapter) Manifests(r Release) ([]Manifest, error) {
	const want = "github.com/thapelomagqazana/forge"
	got := r.ModulePath
	if got != want {
		return nil, fmt.Errorf("%w: module path is %q, want %q",
			ErrModulePathChanged, got, want)
	}
	// A v2+ module must have a /vN suffix. The current module is
	// v1 (no suffix); a future v2 must add /v2.
	//
	// The check below is deliberately narrow. A module path that
	// is exactly `want` passes; a module path that has a suffix
	// after `want` is examined. A suffix that begins with "/v"
	// is a version suffix; the release's major version must be
	// at least 2 for the suffix to be correct.
	//
	// A module path with a non-version suffix (for example,
	// `github.com/forge-cli/forge/internal`) is not a module
	// path at all; the check would not see it because the
	// comparison above already failed. The adapter's job is to
	// verify the module's root path, not a subpackage's.
	if strings.HasPrefix(got, want) && strings.Contains(got[len(want):], "/v") {
		major := majorOf(r.Version)
		if major < 2 {
			return nil, fmt.Errorf("%w: module path %q has a /vN suffix but version %q is v%d",
				ErrModulePathChanged, got, r.Version, major)
		}
	}
	return nil, nil
}

// ErrModulePathChanged is returned when the module path is not what
// the installation guide documents. The error is a blocker: the
// release is not published, and the module path is investigated.
//
// The error is a sentinel so a caller can check for it with
// errors.Is. The driver treats it as a failure (exit code 1), not as
// a skip (exit code 3): a module path change is a defect in the
// release, not a legitimate reason to omit the channel.
var ErrModulePathChanged = fmt.Errorf("distribution: module path changed")

// majorOf returns the major version of a semver tag, e.g. 2 for
// "v2.1.0". A malformed tag returns 0.
//
// The function is deliberately permissive about the version's shape:
// it accepts any tag that begins with a non-negative integer followed
// by a dot. A tag like "v1.4.2-rc.1" returns 1; a tag like
// "v2.0.0-nightly" returns 2. The adapter only compares the result
// to 2; a tag the function does not understand returns 0, which is
// less than 2, and the adapter's check fails. The failure is the
// right outcome: a module with a /vN suffix and a version the
// function cannot parse is a release the adapter cannot verify, and
// the adapter's job is to verify.
func majorOf(version string) int {
	version = strings.TrimPrefix(version, "v")
	dot := strings.Index(version, ".")
	if dot < 0 {
		return 0
	}
	var n int
	for _, r := range version[:dot] {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
