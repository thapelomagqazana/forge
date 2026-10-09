// Package cli implements the Forge command-line interface.
//
// # Public surface
//
// The package exports exactly one symbol: the Execute function. Its
// signature is frozen:
//
//	func Execute() int
//
// Execute runs the Forge CLI and returns a process exit code. It is the
// only function that cmd/forge/main.go is permitted to call.
//
// Every other function, type, constant, and variable in this package is
// unexported. Downstream packages cannot depend on internals. This
// invariant is enforced by internal/cli/structure_test.go.
//
// # Design
//
// The package separates two concerns that are often conflated in Go
// CLIs:
//
//  1. Command construction. The function newRootCmd() builds a
//     *cobra.Command tree. It performs no I/O. It is a pure constructor.
//
//  2. Command execution. The function executeWithOptions() takes an
//     injectable environment (args, stdin, stdout, stderr, env) and
//     runs the command tree. It is the only function that performs
//     process-level I/O.
//
// Execute() is a thin wrapper over executeWithOptions() that supplies
// the default environment (os.Args, os.Stdin, os.Stdout, os.Stderr,
// os.Getenv). Tests call executeWithOptions() directly, with synthetic
// inputs, and assert on the captured output. This is what makes every
// command testable without spawning a subprocess.
//
// # Invariants
//
// The following invariants are enforced by tests in this package. Any
// change that violates them fails CI.
//
//   - Exactly one symbol is exported: Execute.
//   - Cobra is imported in exactly one file: root.go.
//   - No source file in this package performs direct os.Stdout,
//     os.Stderr, or os.Stdin I/O. All I/O flows through the
//     injectable options passed to executeWithOptions.
//   - The package does not import any package outside the standard
//     library, Cobra, and Forge's own module.
//
// # Adding a command
//
// See internal/cli/registry.go (added in WBS 4.4.1) for the command
// registration mechanism. Every command follows the pattern:
//
//   - A file named <command>.go contains a constructor
//     func newXxxCmd(deps Dependencies) *cobra.Command.
//
//   - The constructor is registered in registry.go.
//
//   - The command's handler is thin. Business logic lives in an
//     application service under internal/<domain>/.
//
// The full pattern is documented in the package's README (added in
// WBS 4.4.2).
//
// # Relation to cmd/forge/main.go
//
// main.go is the process entry point. It calls Execute() and passes the
// result to os.Exit. It contains no logic beyond that call. See
// cmd/forge/doc.go for the full rationale.
package cli
