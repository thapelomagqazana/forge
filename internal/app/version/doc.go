// Package version implements the application service for the
// `forge version` command.
//
// # What this package is
//
// This package contains the logic behind `forge version`: the data
// that describes the running Forge build, and the formatting that
// renders that data for a human reader.
//
// It is an application service in the sense defined by
// docs/architecture.md § 4.2.2: it orchestrates a use case on behalf
// of a CLI handler. It does not parse flags, does not write to
// os.Stdout, and does not construct Cobra commands. The CLI handler
// in internal/cli/version.go does those things and delegates the rest
// here.
//
// # What this package is not
//
// It is not a domain package. The domain layer (see
// docs/architecture.md § 4.2.3) contains pure business rules that
// describe how Forge thinks about the world. This package has no
// business rules; it renders four strings.
//
// It is not a library. The package has no exported API beyond what
// the version command needs. Its symbols are stable only within the
// lifetime of WBS 4.3.1.
//
// # Import direction
//
// This package imports only the Go standard library. It does not
// import internal/cli, internal/config, internal/filesystem, or any
// other Forge package.
//
// When WBS 4.3.2 relocates the build variables (Version, Commit,
// BuildDate) from internal/cli to internal/version, this package will
// import that new package. The public API of this package will not
// change; only the source of the values will.
//
// # Testability
//
// The Get function is pure: it reads package-level variables and
// returns a value. The Format function takes an io.Writer, so tests
// pass a bytes.Buffer. Neither function touches the filesystem, the
// network, or the process environment.
//
// See service_test.go for the unit tests.
package version
