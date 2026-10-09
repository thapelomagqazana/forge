// Command forge is the Forge CLI.
//
// This file is the process entry point. It exists for exactly one reason:
// to bridge the operating system's process boundary to the CLI layer.
//
// # Invariant
//
// This file must remain minimal. All logic belongs in internal/cli.
//
// Specifically, this file MUST NOT:
//
//   - Parse command-line arguments.
//   - Read environment variables.
//   - Open or write files.
//   - Print to stdout or stderr.
//   - Format errors.
//   - Recover from panics.
//   - Log anything.
//   - Interpret exit codes.
//
// Each of these concerns belongs in a deeper layer where it can be tested
// in isolation. Placing any of them here would make them untestable,
// because main.go cannot be invoked from within a test process — it *is*
// the process.
//
// # Contract
//
// The file depends on exactly two packages:
//
//   - os:           for os.Exit, which is the only way to set the
//     process exit code.
//   - internal/cli: for cli.Execute, which owns the entire CLI.
//
// No other import is permitted. If a new import seems necessary, the
// logic that requires it belongs in internal/cli, not here.
//
// # Testability
//
// main.go is not tested directly — it has no logic to test. Instead,
// cli.Execute() accepts injectable inputs (args, stdout, stderr, env),
// so every observable behaviour of the Forge binary is exercised through
// tests in internal/cli. See internal/cli/execute_test.go.
//
// # Changing this file
//
// Legitimate reasons to modify this file are rare. They include:
//
//   - Changing the process entry point contract (e.g., wrapping main
//     in a function that defer-recovers, if that ever becomes the
//     chosen design).
//   - Updating the doc comment to reflect a new invariant.
//
// Illegitimate reasons include: any form of "quick fix" that adds
// logic. If you are tempted to add logic, ask: "why can this not live
// in internal/cli?" The answer is almost always "it can".
//
// See: docs/architecture.md, section "Process Entry Point".
package main

import (
	"os"

	"github.com/thapelomagqazana/forge/internal/cli"
)

// main is the process entry point.
//
// It performs exactly one action: it delegates to cli.Execute() and
// forwards the returned exit code to the operating system.
//
// The return value of cli.Execute() is treated as the *only* observable
// output of the process at this level. Everything else — help text,
// version output, error messages, logs — is the responsibility of the
// CLI layer and is written by that layer to the appropriate stream.
//
// On success, cli.Execute() returns the zero exit code, and main returns
// cleanly. On failure, cli.Execute() returns a non-zero exit code, and
// main terminates the process with that code.
//
// This function is intentionally undocumented beyond this comment, because
// it has no branches, no error paths, and no configurable behaviour.
func main() {
	os.Exit(cli.Execute())
}
