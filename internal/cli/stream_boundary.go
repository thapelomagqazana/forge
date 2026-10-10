// Package cli defines the Forge command-line interface.
//
// This file holds the output stream boundary contract (WBS 7.4.1).
// The contract specifies which streams receive which categories of
// output. It is documented in docs/cli-ux-spec.md "Output Stream
// Boundary" and enforced by the tests in stream_boundary_test.go.
//
// # The decision rule
//
// Every line of CLI output belongs on exactly one stream. The
// author of a command decides which stream by asking a single
// question:
//
//	If the user pipes this command's output to another program,
//	should that program receive this line?
//
// If the answer is yes, the line belongs on stdout. If the answer
// is no, the line belongs on stderr.
//
// The rule is binary. There is no "sometimes" and no "it depends".
// A line that would sometimes be wanted by a downstream program and
// sometimes not is a line that belongs on stderr, and the program
// that wants it can redirect stderr to stdout if it chooses.
//
// # The table
//
//	Category                Stream   Rationale
//	Command success output  stdout   Shell scripts pipe successful output
//	Command help (positive) stdout   Help is a requested result
//	Command version         stdout   Version is a requested result
//	Command JSON output     stdout   Machine-readable is the primary output
//	Progress indicators     stderr   Do not pollute piped output
//	Warnings                stderr   Diagnostics, not results
//	Errors                  stderr   Errors are never results
//	Usage after error       stderr   Cobra's SilenceUsage: true prevents this
//	Verbose / debug logs    stderr   Diagnostics
//	Suggestions             stderr   Part of the error block
//
// # Where the rule is enforced
//
// The rule is enforced by the tests in stream_boundary_test.go.
// Each test invokes a command through runCLI, captures the output,
// and asserts that one stream received the expected output and the
// other received nothing. A future command that writes to the
// wrong stream fails the test that names the command.
//
// The helpers assertOnlyStdout and assertOnlyStderr (defined in
// testhelper_test.go) are the mechanical implementation of the
// assertions. This file's constants and doc comment are the rule
// the helpers enforce.
//
// # No production code changes
//
// The boundary is already implemented: every command writes to
// deps.Stdout or deps.Stderr; Cobra is configured with
// SilenceUsage: true and SilenceErrors: true so it does not write
// to either stream directly; the error path in executeWithOptions
// writes to opts.stderr. This WBS item adds the documentation and
// the tests; it does not change the behaviour.
//
// # No package-level mutable state
//
// The CLI package's contract test forbids package-level
// variables. This file declares only constants.
package cli

// boundaryFixtureVersion is the command name used by the
// version-command boundary tests.
//
// # Why a constant
//
// The tests reference the command by name in several places. A
// future rename of the command is a single-line change here; the
// tests read the constant. The constant is not exported because
// no code outside the package needs it.
const boundaryFixtureVersion = "version"

// boundaryFixtureUnknownCommand is the command name used by the
// unknown-command boundary tests.
//
// # Why a constant
//
// The tests reference the unknown command's name in several
// places. The name is chosen to be a string that is definitely
// not a registered command; the constant makes that choice
// visible.
const boundaryFixtureUnknownCommand = "no-such-command"

// boundaryFixtureUnknownFormat is the format value used by the
// unknown-format boundary tests.
//
// # Why a constant
//
// The tests reference the unknown format's name in several
// places. The value is chosen to be a string that is definitely
// not a supported format; the constant makes that choice
// visible.
const boundaryFixtureUnknownFormat = "yaml"
