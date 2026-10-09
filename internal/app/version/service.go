package version

// Info is the result of the version application service.
//
// It is a plain struct with no methods, no embedded types, and no
// dependencies. The CLI handler formats it; nothing else does.
//
// # Why a struct and not four strings
//
// A handler could receive the four fields as separate return values.
// A struct is preferable because:
//
//   - It has a name. "Info" is more legible at the call site than
//     four positional string returns.
//
//   - It is extensible. Adding a fifth field (for example, "GoVersion"
//     or "OS") is a one-line change that does not alter the signature
//     of Get.
//
//   - It is testable. A test can construct an Info literal and pass
//     it to Format without going through Get.
//
// # Zero value
//
// The zero value of Info is valid. It represents "no build
// information is available", which renders as empty strings. The
// formatter is responsible for deciding whether to omit empty fields
// or render them as "(unknown)". In Phase 2, the formatter renders
// them as empty strings.
type Info struct {
	// Version is the semantic version of the running Forge build,
	// for example "0.1.0". It is empty when the binary was built
	// without version information (for example, during local
	// development without ldflags).
	Version string

	// Commit is the short Git commit hash of the running Forge
	// build, for example "a1b2c3d". It is empty when the binary was
	// built outside a Git working tree.
	Commit string

	// BuildDate is the RFC 3339 timestamp of when the binary was
	// built, for example "2026-10-09T12:00:00Z". It is empty when
	// the binary was built without build-date information.
	//
	// The format is not enforced by this struct. The formatter
	// renders the string as-is; the build system is responsible for
	// producing a valid RFC 3339 value.
	BuildDate string

	// Dirty reports whether the running Forge build was produced from a
	// working tree with uncommitted changes. It is the string "true"
	// or "false". It is empty when the binary was built without
	// dirty-state information.
	//
	// The value is not validated by this struct. The formatter
	// renders the string as-is; the build system is responsible for
	// producing a valid value.
	Dirty string
}

// Get returns the build information for the running Forge binary.
//
// # Purity
//
// Get is pure. It reads package-level variables, constructs an Info
// value, and returns it. It does not touch the filesystem, the
// network, the environment, or any other global state. Two calls with
// no intervening mutation return equal values.
//
// The purity is what makes the version command testable without a
// subprocess. A test can stub the build variables (by assigning to
// them in the test package), call Get, and assert on the result. See
// service_test.go for the pattern.
//
// # Why build variables live in internal/cli (for now)
//
// The build variables (Version, Commit, BuildDate) are injected at
// link time via ldflags:
//
//	go build -ldflags "\
//	  -X github.com/thapelomagqazana/forge/internal/cli.Version=0.1.0 \
//	  -X github.com/thapelomagqazana/forge/internal/cli.Commit=$(git rev-parse --short HEAD) \
//	  -X github.com/thapelomagqazana/forge/internal/cli.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
//
// The variables currently live in internal/cli because that is where
// the first WBS item that needed them put them. WBS 4.3.2 will
// relocate them to a new internal/version package, which is a more
// natural home and breaks the (currently benign) dependency from
// internal/app/version back to internal/cli.
//
// Until WBS 4.3.2 lands, Get reads the variables through an internal
// accessor. The accessor is unexported and exists only so that the
// relocation in WBS 4.3.2 touches one file, not two.
func Get() Info {
	version, commit, buildDate, dirty := buildInfo()
	return Info{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
		Dirty:     dirty,
	}
}
