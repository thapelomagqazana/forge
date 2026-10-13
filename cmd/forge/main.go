// Command forge is the Forge CLI.
//
// This file is the process entry point. It exists for exactly one
// reason: to bridge the operating system's process boundary to the
// CLI layer.
//
// # Invariant
//
// This file must remain minimal. All logic belongs in internal/cli.
//
// Specifically, this file MUST NOT:
//
//   - Parse command-line arguments.
//   - Read environment variables (other than through os/signal,
//     whose use is scoped to signal registration).
//   - Open or write files.
//   - Print to stdout or stderr.
//   - Format errors.
//   - Recover from panics.
//   - Log anything.
//   - Interpret exit codes.
//
// Each of these concerns belongs in a deeper layer where it can be
// tested in isolation. Placing any of them here would make them
// untestable, because main.go cannot be invoked from within a test
// process — it *is* the process.
//
// # Contract
//
// The file depends on exactly five packages:
//
//   - context:      for context.Background(), the parent of the
//     signal-aware context.
//   - os:           for os.Exit (the only way to set the process
//     exit code) and os.Interrupt (the SIGINT signal value passed
//     to signal.NotifyContext).
//   - os/signal:    for signal.NotifyContext, which installs the
//     context that is cancelled on SIGINT or SIGTERM. This is the
//     one piece of process-level logic main owns: the signal
//     handler's lifetime must match the process's lifetime, and
//     only main can guarantee that.
//   - syscall:      for syscall.SIGTERM, the second signal value
//     passed to signal.NotifyContext. syscall.SIGTERM is declared
//     on every platform Go supports.
//   - internal/cli: for cli.Run, which owns the entire CLI.
//
// No other import is permitted. If a new import seems necessary,
// the logic that requires it belongs in internal/cli, not here.
//
// # Platform-specific signal handling
//
// The CLI's broken-pipe contract requires that a write to a pipe
// whose read end has closed returns an error instead of the
// process being terminated by the kernel. On Unix, the write
// delivers SIGPIPE unless the process has installed SIG_IGN for
// the signal. main installs the disposition by calling
// installPlatformSignalHandlers, which is provided by two
// platform-tagged files in this directory:
//
//   - signal_init_unix.go  installs SIG_IGN for SIGPIPE.
//   - signal_init_other.go is a no-op.
//
// The split exists because syscall.SIGPIPE is not declared on
// every platform Go supports (for example, js/wasm). A direct
// reference from main.go would fail to compile on those
// platforms; the tagged files confine the reference to the
// platforms where it exists.
//
// On Windows, the broken-pipe condition is ERROR_BROKEN_PIPE,
// returned by WriteFile as an error, not a signal. The CLI's write
// sites classify the error via streams.HandleWriteError, which on
// Windows matches ERROR_BROKEN_PIPE. No signal disposition is
// needed.
//
// # The exit boundary (WBS 11.3.1, ADR-020)
//
// main is the only function in the repository that calls os.Exit.
// The rule is enforced by scripts/deps/verify-no-os-exit.sh, which
// fails if os.Exit appears in any other non-test file.
//
// The split is deliberate:
//
//   - main owns the process's termination and the signal handlers
//     that influence it.
//
//   - internal/cli owns everything else: parsing arguments,
//     dispatching to commands, rendering errors, deriving the exit
//     code. cli.Run returns the code; it never calls os.Exit.
//
// The two-way split is what makes the CLI testable in-process: a
// test calls cli.Run with a synthetic context and observes the
// return value. The process-level concerns (the actual os.Exit,
// the actual signal delivery) are outside the test's scope, and
// the integration tests in cmd/forge/binary_integration_test.go
// cover them.
//
// # Signal handling
//
// signal.NotifyContext installs a context that is cancelled when
// SIGINT (Ctrl-C) or SIGTERM arrives. The context is passed to
// cli.Run, which threads it through the command tree. When the
// signal fires:
//
//  1. The context is cancelled.
//  2. Commands that respect the context return early with
//     ctx.Err() (typically context.Canceled).
//  3. cli.Run inspects the context's Cause and returns the mapped
//     code: ExitSIGINT (130) for SIGINT, ExitSIGTERM (143) for
//     SIGTERM.
//  4. main calls os.Exit with that code.
//
// The mapping (130 for SIGINT, 143 for SIGTERM) follows the POSIX
// shell convention (128 + N). A shell that observes a process
// terminated by SIGINT reports the same code Forge returns, so
// scripts cannot distinguish "Forge was interrupted" from "a
// process was interrupted", which is the intended behaviour.
//
// # Testability
//
// main.go is not tested directly — it has no logic to test beyond
// the signal registration and the single os.Exit. The signal
// registration for SIGINT and SIGTERM is exercised by the binary
// integration test in cmd/forge/binary_integration_test.go, which
// spawns the binary and sends it a signal. The SIGPIPE
// registration is exercised by the broken-pipe tests in the same
// directory, which spawn the binary, close the read end of its
// stdout, and observe the exit code. The os.Exit call is verified
// by the same tests (the process's exit status is the observable
// result).
//
// cli.Run accepts injectable inputs (args, stdout, stderr, env)
// through the boundary's options value, so every observable
// behaviour of the Forge binary is exercised through tests in
// internal/cli. See internal/cli/execute_test.go and
// internal/cli/mapping_test.go.
//
// # Changing this file
//
// Legitimate reasons to modify this file are rare. They include:
//
//   - Changing the process entry point contract (e.g., adding a
//     new signal to the NotifyContext list, or changing the
//     context's cancellation policy).
//   - Updating the doc comment to reflect a new invariant.
//
// Illegitimate reasons include: any form of "quick fix" that adds
// logic. If you are tempted to add logic, ask: "why can this not
// live in internal/cli?" The answer is almost always "it can".
//
// The signal-handling logic is the one exception, and it is
// documented above: main owns it because the signal handler's
// lifetime must match the process's, and only main can guarantee
// that.
//
// See: docs/architecture.md, section "Process Entry Point";
// docs/architecture/exit-boundary.md;
// docs/reference/output-streams.md §"Broken pipe"; and
// docs/decisions/ADR-020-exit-boundary-pattern.md.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/thapelomagqazana/forge/internal/cli"
)

// main is the process entry point.
//
// It performs exactly three actions:
//
//  1. Calls installPlatformSignalHandlers to install the
//     process's platform-specific signal dispositions. On Unix,
//     this installs SIG_IGN for SIGPIPE. On every other
//     platform, it is a no-op.
//
//  2. Installs a signal-aware context for SIGINT and SIGTERM. The
//     context is cancelled when either signal arrives; cli.Run
//     maps the cancellation to the appropriate exit code (130 for
//     SIGINT, 143 for SIGTERM).
//
//  3. Calls cli.Run with that context and forwards the returned
//     exit code to the operating system via os.Exit.
//
// The return value of cli.Run is treated as the *only* observable
// output of the process at this level. Everything else — help
// text, version output, error messages, logs — is the
// responsibility of the CLI layer and is written by that layer to
// the appropriate stream.
//
// On success, cli.Run returns (ExitSuccess, nil), and main calls
// os.Exit(0). On failure, cli.Run returns a non-zero code and a
// non-nil error (which has already been rendered to stderr); main
// calls os.Exit with the code and ignores the error.
//
// # Why the error is discarded
//
// The error returned by cli.Run is the error the boundary already
// rendered to stderr. main has no further use for it: it is not
// logged (logging belongs in the CLI layer), it is not formatted
// (formatting belongs in the CLI layer), and it is not inspected
// (the code carries the exit classification). Discarding it with
// `_` is deliberate; a future caller that embeds Forge as a
// library calls cli.Run directly and uses the error.
//
// # Why the deferred stop is important
//
// signal.NotifyContext returns a stop function that unregisters
// the signal handler. The defer ensures the handler is
// unregistered before the process exits. In a short-lived process
// the leaked handler would be harmless, but the pattern is the
// same one a long-lived process would use, and consistency
// matters. The defer also makes the cleanup visible at the call
// site: a reader sees that the handler's lifetime is scoped to
// main.
//
// # Why ctx is context.Background()
//
// The signal.NotifyContext function derives its context from a
// parent context. main has no parent context (it is the root of
// the call chain), so context.Background() is the correct choice.
// A future WBS that wants main to inherit a context from
// somewhere (for example, from an embedding process) would change
// this line.
func main() {
	// Install the process's platform-specific signal
	// dispositions. On Unix, this installs SIG_IGN for SIGPIPE;
	// on every other platform, it is a no-op. See
	// signal_init_unix.go and signal_init_other.go for the
	// platform-specific implementations.
	installPlatformSignalHandlers()

	// Install the signal-aware context. The context is cancelled
	// when SIGINT (Ctrl-C) or SIGTERM arrives. The stop function
	// unregisters the handler; defer ensures it runs before the
	// process exits.
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// Run the CLI. Run returns the process exit code and the
	// error it rendered (nil on success). The error is discarded:
	// it has already been rendered to stderr by the boundary, and
	// main has no further use for it.
	code, _ := cli.Run(ctx)

	// Exit with the code. This is the only os.Exit call in the
	// repository; scripts/deps/verify-no-os-exit.sh enforces the
	// rule.
	os.Exit(code)
}
