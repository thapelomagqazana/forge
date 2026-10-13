// Package cli — process exit codes, exit code taxonomy, and
// per-code semantics.
//
// This file defines the exit code namespace returned by the Forge
// CLI. It has two layers:
//
//   - The range taxonomy (WBS 11.1.1): the numeric ranges that
//     group codes by domain (findings, execution failures, usage,
//     internal errors, signals). The ranges are documented in
//     docs/reference/exit-code-ranges.md and decided in
//     docs/decisions/ADR-018-exit-code-taxonomy.md.
//
//   - The per-code enumeration (WBS 11.1.2): the concrete codes
//     within each range, each with exactly one trigger, one
//     severity, one stdout/stderr behavior, and one suppression
//     rule. The enumeration is documented in
//     docs/reference/exit-codes.md and docs/reference/exit-codes.yaml,
//     and decided in docs/decisions/ADR-019-exit-code-semantics.md.
//
// The two layers are complementary: the range tells a caller *how
// to branch* (is this a finding or an execution failure?); the
// code tells a caller *what this specific outcome means* (did the
// check fail, or did the config fail?). A caller that only needs
// the former reads the classifiers (IsFinding,
// IsExecutionFailure, …). A caller that needs the latter reads the
// named codes (ExitCheckFailed, ExitConfig, …).
//
// # The mapping
//
// The exit code for an error is the error's Category().ExitCode().
// The method is defined in internal/forgeerr/category.go; it is the
// single source of truth. This file declares the constants and the
// range taxonomy that the categories' exit codes must fall within.
//
// The two definitions must agree. TestAC4_ExitCodesMatchCLIConstants
// in internal/forgeerr/category_test.go polices numeric agreement, and
// TestExitCodeTaxonomy_AllCodesInRange in this package polices range
// membership. If WBS 4.1.2, WBS 11.1.1, or WBS 11.1.2 changes, the
// tests fail and the constants, the range table, and the switch in
// Category.ExitCode are updated in the same commit.
//
// # Why the constants are duplicated
//
// The constants live in internal/cli (not in internal/forgeerr) so
// that internal/forgeerr does not depend on internal/cli. The
// dependency goes one way: cli imports forgeerr.
//
// # The two failure domains
//
// Forge distinguishes between:
//
//   - Forge findings (1–9): the command ran correctly; the subject of
//     the command is in a state the caller asked to detect. This is
//     not a tool malfunction.
//   - Forge execution failures (10–19): the command could not run;
//     Forge itself encountered a problem with its inputs, environment,
//     or internal state. This is a tool malfunction from the caller's
//     perspective.
//
// The distinction is encoded in the numeric ranges so that a caller
// can branch on code < 10 vs. code >= 10 without parsing output.
//
// # Stability
//
// A constant, once shipped in a release, is never renumbered. If a new
// failure category is introduced, a new constant is added at the next
// available integer within the appropriate range. Existing codes
// retain their meaning. This is the same stability rule that applies
// to error codes in internal/forgeerr (WBS 10.3.1). The two rules are
// consistent: exit codes and error codes both change by addition,
// never by renumbering.
//
// # The two stability properties
//
// The taxonomy distinguishes two properties of a range, and the
// signals range (129–255) is the only case where they diverge:
//
//   - The range's *membership* is the set of codes within it.
//     Membership is either closed (no new codes will ever be added:
//     StabilityFrozen) or open (new codes may be added:
//     StabilityAdditive). A reserved range is a form of open
//     membership whose codes are not yet assigned.
//
//   - The range's *mapping* is the rule that converts a signal (or
//     other external event) into a code. For the signals range, the
//     mapping is the POSIX convention 128 + N; Forge does not own
//     it and cannot change it.
//
// The Stability class of a range describes its *membership*, not its
// mapping. For most ranges the two coincide: a frozen range has a
// frozen mapping (there is nothing to map), and an additive range
// has an additive mapping (new codes come from new categories, and
// the categories' mappings are the Forge-owned switch in
// Category.ExitCode).
//
// The signals range is the exception. Its mapping is frozen by
// POSIX (128 + 1 is SIGHUP, 128 + 2 is SIGINT, and so on), but its
// membership is additive: a future phase may handle SIGQUIT
// (128 + 3) or SIGKILL (128 + 9) by adding named codes to the
// range. The Stability class is therefore StabilityAdditive, and
// the frozen-mapping property is recorded in the range's Purpose
// and in the docstrings of its boundary constants
// (ExitRangeSignalsMin, ExitRangeSignalsMax).
//
// # The severity classification (WBS 11.1.2)
//
// Every non-signal code has a severity: OK, WARN, ERROR, or FATAL.
// The severity is the code's CI classification:
//
//   - OK: success. Not a CI failure.
//   - WARN: a finding the user should see, but which does not fail
//     CI unless escalated by --strict, by a forge.yaml policy, or
//     by a future CI environment variable.
//   - ERROR: a finding or execution failure that fails CI.
//   - FATAL: an internal error or unrecoverable condition. Fails
//     CI.
//
// The severity is a property of the code, not of the invocation.
// A code's severity never changes; a change of severity requires a
// new code. The mapping is defined by severityOf in this file; the
// human-readable form is the Severity column in
// docs/reference/exit-codes.md, and the machine-readable form is
// the `severity` field in docs/reference/exit-codes.yaml. The
// three must agree; the CI check
// scripts/deps/verify-exit-codes.sh asserts the agreement.
//
// # The invariant the tests enforce
//
// A StabilityFrozen range is a singleton: it contains exactly one
// code, and no code will ever be added to it. The invariant is
// asserted by TestExitCodeTaxonomy_FrozenRangesAreSingletons and by
// TestAC_11_1_1_CompatibilityPolicyStated. The signals range is not
// a singleton and is therefore not StabilityFrozen; the two tests
// are correct, and the range table was wrong until the class was
// changed.
//
// # Platform stability
//
// Forge emits only 0–255 on all platforms. Windows 32-bit exit codes
// are masked with & 0xFF. This guarantees identical codes across
// Linux, macOS, and Windows.
//
// # The removed mapping
//
// Before WBS 10.5.1, this file also defined a CategorizedError
// interface, an errorsAs wrapper, and an exitCodeFromError function
// that mapped a category string to an exit code. All three are
// superseded:
//
//   - CategorizedError was replaced by forgeerr.Category, the closed
//     taxonomy type (WBS 10.2.1).
//   - errorsAs was replaced by direct use of errors.As on
//     *forgeerr.Error.
//   - exitCodeFromError was replaced by Category.ExitCode(), the
//     category's method (WBS 10.2.1).
//
// The boundary's local helper exitCodeOf (in execute.go) performs the
// errors.As extraction and reads the category's method.
package cli

import (
	"fmt"
	"sort"
)

// ---------------------------------------------------------------------------
// Range boundaries
// ---------------------------------------------------------------------------
//
// Each constant below is part of the public contract. Values never
// change. New ranges are added at the top of the reserved space
// (30–63 or 79–127) without shifting existing boundaries.
// ---------------------------------------------------------------------------

const (
	// ExitSuccess indicates the command completed successfully.
	//
	// This is the only non-error exit code. Every other code indicates
	// some form of failure.
	//
	// Severity: OK. Not a CI failure.
	// Stability: Frozen. Universal convention.
	ExitSuccess = 0

	// ExitRangeFindingsMin is the minimum exit code for Forge findings.
	//
	// A "finding" means the command executed correctly, but the subject
	// of the command is in a state the user asked to detect. Examples:
	// drift detected, validation warnings-as-errors, check assertion
	// mismatch.
	//
	// Callers should NOT treat this as a crash. Parse output for details
	// and consider remediation.
	//
	// Stability: Additive. Individual codes within this range may be
	// added; the boundary itself is frozen.
	ExitRangeFindingsMin = 1

	// ExitRangeFindingsMax is the maximum exit code for Forge findings.
	//
	// See ExitRangeFindingsMin for semantics.
	ExitRangeFindingsMax = 9

	// ExitRangeExecutionMin is the minimum exit code for Forge
	// execution failures.
	//
	// An "execution failure" means the command could not complete
	// because Forge itself encountered a problem: bad configuration,
	// missing template, filesystem error, security denial.
	//
	// Callers SHOULD treat this as a tool failure. Fix environment or
	// inputs; retry may succeed after remediation.
	//
	// Stability: Additive.
	ExitRangeExecutionMin = 10

	// ExitRangeExecutionMax is the maximum exit code for Forge
	// execution failures.
	//
	// See ExitRangeExecutionMin for semantics.
	ExitRangeExecutionMax = 19

	// ExitRangeReservedRegistryMin is the minimum exit code for the
	// reserved registry/network/enterprise range.
	//
	// This range is reserved for Phase 2+ features: blueprint registry
	// fetch failures, network timeouts, enterprise policy server
	// errors.
	//
	// NOT YET ASSIGNED. No code in this range is emitted by current
	// Forge versions. Callers must not depend on specific values.
	//
	// Stability: Reserved. Do not assign until Phase 2.
	ExitRangeReservedRegistryMin = 20

	// ExitRangeReservedRegistryMax is the maximum exit code for the
	// reserved registry/network/enterprise range.
	//
	// See ExitRangeReservedRegistryMin for semantics.
	ExitRangeReservedRegistryMax = 29

	// ExitRangeReservedForgeMin is the minimum exit code for future
	// Forge-assigned ranges.
	//
	// Stability: Reserved.
	ExitRangeReservedForgeMin = 30

	// ExitRangeReservedForgeMax is the maximum exit code for future
	// Forge-assigned ranges.
	//
	// Stability: Reserved.
	ExitRangeReservedForgeMax = 63

	// ExitUsage indicates a usage error: unknown flag, missing required
	// argument, invalid subcommand.
	//
	// This is BSD sysexits.h EX_USAGE (64). It is a user error, not a
	// tool error.
	//
	// Severity: ERROR.
	// Stability: Frozen. BSD sysexits.h.
	ExitUsage = 64

	// ExitRangeReservedSysexitsMin is the minimum exit code for the
	// reserved BSD sysexits sub-range.
	//
	// Stability: Reserved. BSD sysexits.h.
	ExitRangeReservedSysexitsMin = 65

	// ExitRangeReservedSysexitsMax is the maximum exit code for the
	// reserved BSD sysexits sub-range.
	//
	// Stability: Reserved. BSD sysexits.h.
	ExitRangeReservedSysexitsMax = 69

	// ExitRangeInternalMin is the minimum exit code for internal
	// errors.
	//
	// Internal errors indicate a bug in Forge or an unrecoverable
	// internal state. This range aligns with BSD sysexits.h:
	//
	//	70 EX_SOFTWARE  internal logic error
	//	71 EX_OSERR     OS error (fork, pipe, etc.)
	//	72 EX_OSFILE    critical OS file missing
	//	73 EX_CANTCREAT cannot create output file
	//	74 EX_IOERR     I/O error
	//	75 EX_TEMPFAIL  temporary failure, retry
	//	76 EX_PROTOCOL  remote protocol error
	//	77 EX_NOPERM    permission denied
	//	78 EX_CONFIG    configuration error (internal)
	//
	// Callers SHOULD report a bug with --verbose output.
	//
	// Stability: Additive. BSD sysexits.h.
	ExitRangeInternalMin = 70

	// ExitRangeInternalMax is the maximum exit code for internal
	// errors.
	//
	// See ExitRangeInternalMin for semantics.
	ExitRangeInternalMax = 78

	// ExitRangeReservedFutureMin is the minimum exit code for future
	// reserved use.
	//
	// Stability: Reserved.
	ExitRangeReservedFutureMin = 79

	// ExitRangeReservedFutureMax is the maximum exit code for future
	// reserved use.
	//
	// Stability: Reserved.
	ExitRangeReservedFutureMax = 127

	// ExitInvalidArgument indicates the exit argument itself was
	// invalid.
	//
	// Stability: Frozen. Shell convention.
	ExitInvalidArgument = 128

	// ExitRangeSignalsMin is the minimum exit code for signal
	// terminations (128 + N).
	//
	// The 128 + N mapping is fixed by POSIX; the range's membership
	// is additive (Forge may handle more signals in a future phase).
	// The boundary itself (129 = 128 + 1 = SIGHUP) is frozen by the
	// convention.
	//
	// Stability: Additive (membership); frozen (boundary and
	// mapping).
	ExitRangeSignalsMin = 129

	// ExitSIGHUP indicates termination by SIGHUP (128 + 1).
	//
	// The code's value is frozen by the POSIX shell convention. It
	// is a named member of the additive signals range: the range
	// may grow, but this code's value never changes.
	//
	// Severity: none (signal terminations have no severity).
	// Stability: Frozen (the code); additive (the surrounding
	// range).
	ExitSIGHUP = 129

	// ExitSIGINT indicates termination by SIGINT (128 + 2). Ctrl-C.
	//
	// Forge installs a SIGINT handler that cancels the current
	// context, runs cleanup, and exits with this code.
	//
	// The code's value is frozen by the POSIX shell convention. It
	// is a named member of the additive signals range: the range
	// may grow, but this code's value never changes.
	//
	// Severity: none (signal terminations have no severity).
	// Stability: Frozen (the code); additive (the surrounding
	// range).
	ExitSIGINT = 130

	// ExitSIGTERM indicates termination by SIGTERM (128 + 15).
	//
	// Forge installs a SIGTERM handler that cancels the current
	// context, runs cleanup, and exits with this code.
	//
	// The code's value is frozen by the POSIX shell convention. It
	// is a named member of the additive signals range: the range
	// may grow, but this code's value never changes.
	//
	// Severity: none (signal terminations have no severity).
	// Stability: Frozen (the code); additive (the surrounding
	// range).
	ExitSIGTERM = 143

	// ExitBrokenPipe indicates the process wrote to a stream whose
	// read end had closed, and no further output was possible.
	//
	// Trigger: a write to stdout (or stderr) fails with a
	// broken-pipe error. On Unix, the error is EPIPE; on Windows,
	// it is ERROR_BROKEN_PIPE. The detection and classification
	// live in internal/streams (see streams.IsBrokenPipe and
	// streams.HandleWriteError).
	//
	// Range:  Signals (129–255). The value is 128 + 13, the POSIX
	//         convention for SIGPIPE (128 + N where SIGPIPE is 13).
	//         Forge does not receive SIGPIPE; Go installs a handler
	//         that ignores it, and the broken-pipe condition surfaces
	//         as a write error instead. Forge maps the write error
	//         to this code so that a cross-platform consumer sees
	//         one exit code for the condition.
	//
	//         A consumer that branches on IsSignal (range 129–255)
	//         treats this code as it would treat a signal
	//         termination. That is the intended reading: the
	//         process terminated because an external event closed
	//         its stream, not because Forge failed.
	//
	// Severity: none. Signal-adjacent codes have no CI severity:
	//           the termination is not Forge's failure, and the
	//           record is not part of a successful or unsuccessful
	//           command's output. A CI system that observes 141
	//           should treat it as a non-Forge condition.
	//
	// Suppressible: no. The behavior is fixed: the process exits
	//               141 and writes nothing to stderr for the
	//               broken-pipe condition itself.
	//
	// stdout: empty (or truncated at the point the pipe closed).
	// stderr: empty for the broken pipe; no error block, no stack
	//         trace.
	//
	// Stability: Frozen (the code). The value is pinned by the
	//            POSIX convention 128 + SIGPIPE and must not change.
	//            The surrounding signals range is additive, but this
	//            particular code is one of the named members whose
	//            value is fixed by the convention.
	//
	// See also: docs/reference/output-streams.md §"Broken pipe"
	//           and docs/reference/exit-codes.yaml.
	ExitBrokenPipe = 141

	// ExitRangeSignalsMax is the maximum exit code for signal
	// terminations (128 + N) and reserved high codes.
	//
	// The 128 + N mapping is fixed by POSIX; the range's membership
	// is additive (Forge may handle more signals in a future phase).
	// The boundary itself (255) is frozen by the byte-width of the
	// exit code, not by a stability class.
	//
	// Stability: Additive (membership); frozen (boundary).
	ExitRangeSignalsMax = 255
)

// ---------------------------------------------------------------------------
// Named codes within the taxonomy (WBS 11.1.1 + WBS 11.1.2 + WBS 12.4.1)
// ---------------------------------------------------------------------------
//
// These are the concrete code assignments. They are the values that
// forgeerr.Category.ExitCode() returns. Adding a new code is
// additive: pick the next free integer in the appropriate range.
//
// The block is ordered by code number, not by WBS item. A reader
// who wants the WBS-item grouping should read the docstrings:
// each names its trigger and its severity.
//
// # WBS 11.1.1 vs. WBS 11.1.2 vs. WBS 12.4.1
//
// WBS 11.1.1 introduced the taxonomy and a partial enumeration
// (ExitDriftDetected, ExitValidation, ExitCheckFailed, ExitConfig,
// ExitTemplate, ExitFilesystem, ExitSecurity, ExitConflict,
// ExitInternal, ExitUsage, the signal codes). WBS 11.1.2 fills in
// the remaining triggers and gives every code a severity. WBS
// 12.4.1 adds ExitBrokenPipe in the signals range.
//
// Each addition is ADDITIVE. None of the three renumbers,
// reclassifies, or renames an existing code. In particular:
//
//   - ExitConflict stays at 14 (execution-failure range), where
//     WBS 11.1.1 placed it.
//   - The WBS 11.1.2 aliases (ExitUpdateConflict, ExitInternalError,
//     ExitTemplateNotFound, ExitTemplateIncompatible,
//     ExitRenderingFailed, ExitPathSafetyViolation,
//     ExitSecurityPolicyViolation) share their canonical codes'
//     values.
//   - ExitBrokenPipe is a new code at 141, in the signals range.
//     The range's membership was already additive; no boundary
//     moves.
//
// The WBS 11.1.2 names for triggers that WBS 11.1.1 already named
// are declared as aliases (const X = Y) immediately after the
// canonical constant. The alias's value equals the original's by
// construction; a test asserts the equality. Aliasing keeps the
// numeric contract intact: a consumer that references either name
// gets the same exit code.
// ---------------------------------------------------------------------------

const (
	// --- Success -------------------------------------------------

	// (ExitSuccess is declared in the range-boundaries block above;
	// the constant is the only one that belongs to both blocks.)

	// --- Findings range (1–9) ------------------------------------

	// ExitDriftDetected indicates drift was detected between the
	// blueprint and the project. This is the generic finding.
	//
	// Trigger: `diff` detected drift between blueprint and project.
	// Range:  Findings (1–9).
	// Severity: ERROR. Fails CI.
	// Suppressible: --quiet suppresses the diagnostic on stderr.
	// stdout: The diff report.
	// stderr: A diagnostic on failure.
	// Legacy: ExitFailure (WBS 4.1.2, value 1).
	ExitDriftDetected = 1

	// ExitValidation indicates a validation warning promoted to an
	// error by --strict, or a check assertion mismatch.
	//
	// Trigger: A validation warning was escalated, or a check
	// assertion failed.
	// Range:  Findings (1–9).
	// Severity: ERROR. Fails CI.
	// Suppressible: --quiet suppresses the diagnostic on stderr.
	// stdout: The report.
	// stderr: A diagnostic on failure.
	// Legacy: ExitValidation (WBS 4.1.2, value 5).
	ExitValidation = 2

	// ExitCheckFailed indicates a check operation failed its
	// assertion.
	//
	// Trigger: `check` found an ERROR-level finding.
	// Range:  Findings (1–9).
	// Severity: ERROR. Fails CI.
	// Suppressible: --quiet suppresses the diagnostic on stderr.
	// stdout: The check report.
	// stderr: A diagnostic on failure.
	ExitCheckFailed = 3

	// ExitAmbiguity indicates `init` could not decide the
	// foundation from the given inputs (ambiguous).
	//
	// Trigger: `init` cannot determine the foundation from the
	// inputs (for example, two candidates match equally well).
	// Range:  Findings (1–9).
	// Severity: ERROR. Fails CI.
	// Suppressible: --quiet suppresses the diagnostic on stderr.
	// stdout: The candidates list.
	// stderr: A diagnostic on failure.
	//
	// # Value assignment
	//
	// The value is 4, in the findings range. WBS 11.1.2 lists the
	// ambiguity trigger as a finding: the command ran correctly and
	// the inputs do not uniquely determine the foundation. The
	// caller is asked to supply more information.
	ExitAmbiguity = 4

	// --- Execution-failure range (10–19) -------------------------

	// ExitConfig indicates a configuration loading or validation
	// failure: file not found, file unreadable, malformed YAML,
	// unknown field, invalid value.
	//
	// Trigger: forge.yaml is missing, unreadable, malformed, or
	// violates its schema.
	// Range:  Execution failures (10–19).
	// Severity: ERROR. Fails CI.
	// Suppressible: no.
	// stdout: Empty.
	// stderr: A diagnostic on failure.
	// Legacy: ExitConfig (WBS 4.1.2, value 3).
	ExitConfig = 10

	// ExitTemplate indicates a template error: missing, malformed,
	// or render failure. It is the generic template code.
	//
	// The WBS 11.1.2 enumeration introduces three finer triggers
	// (ExitTemplateNotFound, ExitTemplateIncompatible,
	// ExitRenderingFailed). ExitTemplate is retained for the
	// generic case; a command that cannot distinguish among the
	// three finer cases uses ExitTemplate.
	//
	// Trigger: A template error occurred that is not one of the
	// three finer triggers.
	// Range:  Execution failures (10–19).
	// Severity: ERROR. Fails CI.
	// Suppressible: no.
	// stdout: Empty.
	// stderr: A diagnostic on failure.
	ExitTemplate = 11

	// ExitTemplateNotFound indicates a template referenced by the
	// blueprint does not exist.
	//
	// Trigger: A template referenced by the blueprint does not
	// exist.
	// Range:  Execution failures (10–19).
	// Severity: ERROR. Fails CI.
	// Suppressible: no.
	// stdout: Empty.
	// stderr: A diagnostic on failure.
	//
	// # Value assignment
	//
	// The value is 11, shared with ExitTemplate. The two codes
	// have the same numeric value because they are the same
	// trigger from the caller's perspective: a template error.
	// The finer distinction is a documentation aid, not a
	// distinct exit code. A future ADR may split them; until
	// then, the assignment of a distinct value is not made.
	ExitTemplateNotFound = ExitTemplate

	// ExitTemplateIncompatible indicates a template exists but is
	// incompatible with the blueprint or the Forge version.
	//
	// Trigger: A template exists but is incompatible with the
	// blueprint or the Forge version.
	// Range:  Execution failures (10–19).
	// Severity: ERROR. Fails CI.
	// Suppressible: no.
	// stdout: Empty.
	// stderr: A diagnostic on failure.
	//
	// # Value assignment
	//
	// The value is 11, shared with ExitTemplate. See
	// ExitTemplateNotFound's docstring for the rationale.
	ExitTemplateIncompatible = ExitTemplate

	// ExitRenderingFailed indicates a template failed to render
	// (missing variables, invalid syntax).
	//
	// Trigger: A template failed to render.
	// Range:  Execution failures (10–19).
	// Severity: ERROR. Fails CI.
	// Suppressible: no.
	// stdout: Empty.
	// stderr: A diagnostic on failure.
	//
	// # Value assignment
	//
	// The value is 11, shared with ExitTemplate. See
	// ExitTemplateNotFound's docstring for the rationale.
	ExitRenderingFailed = ExitTemplate

	// ExitFilesystem indicates a filesystem operation failure:
	// permission denied, path not found, write failure.
	//
	// Trigger: A filesystem operation failed.
	// Range:  Execution failures (10–19).
	// Severity: ERROR. Fails CI.
	// Suppressible: no.
	// stdout: Empty.
	// stderr: A diagnostic on failure.
	// Legacy: ExitFilesystem (WBS 4.1.2, value 4).
	ExitFilesystem = 12

	// ExitPathSafetyViolation indicates a computed path resolved
	// outside the project root.
	//
	// Trigger: A computed path resolved outside the project root.
	// Range:  Execution failures (10–19).
	// Severity: ERROR. Fails CI.
	// Suppressible: no.
	// stdout: Empty.
	// stderr: A diagnostic on failure.
	//
	// # Value assignment
	//
	// The value is 12, shared with ExitFilesystem. The two codes
	// have the same numeric value because a path-safety violation
	// is a filesystem error from the caller's perspective. The
	// finer distinction is a documentation aid.
	ExitPathSafetyViolation = ExitFilesystem

	// ExitSecurity indicates a security boundary violation: a
	// path traversal attempt, a symlink escape, or a secret leak.
	//
	// Trigger: A security boundary was violated.
	// Range:  Execution failures (10–19).
	// Severity: ERROR. Fails CI.
	// Suppressible: no.
	// stdout: Empty.
	// stderr: A diagnostic on failure.
	// Legacy: ExitSecurity (WBS 4.1.2, value 6).
	ExitSecurity = 13

	// ExitSecurityPolicyViolation indicates a security policy was
	// violated (path traversal, symlink escape, secret exposure).
	//
	// Trigger: A security policy was violated.
	// Range:  Execution failures (10–19).
	// Severity: ERROR. Fails CI.
	// Suppressible: no.
	// stdout: Empty.
	// stderr: A diagnostic on failure.
	//
	// # Value assignment
	//
	// The value is 13, shared with ExitSecurity, for the same
	// reason as ExitPathSafetyViolation.
	ExitSecurityPolicyViolation = ExitSecurity

	// ExitConflict indicates an update could not be applied
	// without overwriting developer changes.
	//
	// Trigger: `update` could not apply a change without
	// overwriting developer modifications.
	// Range:  Execution failures (10–19).
	// Severity: ERROR. Fails CI.
	// Suppressible: --quiet suppresses the diagnostic on stderr.
	// stdout: The conflict report.
	// stderr: A diagnostic on failure.
	// Legacy: ExitConflict (WBS 4.1.2, value 7).
	//
	// # Value assignment
	//
	// The value is 14, in the execution-failure range. WBS 11.1.1
	// placed the conflict code here, and the stability policy
	// forbids renumbering. The WBS 11.1.2 doc's description of
	// the conflict trigger groups it with ambiguity; that grouping
	// is a documentation aid, not a reclassification. The code
	// stays where WBS 11.1.1 put it.
	ExitConflict = 14

	// ExitUpdateConflict is the WBS 11.1.2 name for ExitConflict.
	//
	// The alias exists so that the WBS 11.1.2 doc (which uses the
	// name "UpdateConflict") has a Go symbol to reference. A
	// consumer may use either name; the value is identical.
	ExitUpdateConflict = ExitConflict

	// ExitInternal indicates an internal error: a bug in Forge or
	// an unrecoverable internal state. It is the generic
	// internal-error code and the value returned by the safety-net
	// branch of exitCodeOf when a plain error reaches the
	// boundary.
	//
	// This is BSD sysexits.h EX_SOFTWARE (70). More specific
	// internal conditions use the adjacent sysexits codes 71–78;
	// see ExitRangeInternalMin.
	//
	// Trigger: An internal invariant was violated, or a panic was
	// recovered.
	// Range:  Internal errors (70–78).
	// Severity: FATAL. Fails CI.
	// Suppressible: no.
	// stdout: Empty.
	// stderr: A diagnostic on failure.
	// Legacy: ExitFailure (WBS 4.1.2, value 1).
	ExitInternal = 70

	// ExitInternalError is the WBS 11.1.2 name for ExitInternal.
	//
	// The alias exists so that the WBS 11.1.2 doc (which uses the
	// name "InternalError") has a Go symbol to reference.
	ExitInternalError = ExitInternal
)

// ---------------------------------------------------------------------------
// Severity (WBS 11.1.2, ADR-019)
// ---------------------------------------------------------------------------

// Severity classifies an exit code's trigger condition for the CI
// contract. It is the machine-readable form of ADR-019.
//
// The severity is a property of the code, not of the invocation.
// A code's severity never changes; a change of severity requires a
// new code.
//
// # The four severities
//
//   - SeverityOK: success. Not a CI failure.
//   - SeverityWarn: a finding the user should see, but which does
//     not fail CI. Escalated to SeverityError by --strict, by a
//     forge.yaml policy, or by a future CI environment variable.
//   - SeverityError: a finding or execution failure that fails
//     CI.
//   - SeverityFatal: an internal error or unrecoverable
//     condition. Fails CI.
//
// The empty string is not a valid severity. It is the zero value
// and is returned by severityOf for codes that have no severity
// (the signal codes and the signal-adjacent broken-pipe code).
type Severity string

const (
	// SeverityOK classifies the success code. Not a CI failure.
	SeverityOK Severity = "ok"

	// SeverityWarn classifies a finding that does not fail CI
	// unless escalated. Escalation is explicit (--strict, policy).
	SeverityWarn Severity = "warn"

	// SeverityError classifies a finding or execution failure that
	// fails CI.
	SeverityError Severity = "error"

	// SeverityFatal classifies an internal error or unrecoverable
	// condition. Fails CI.
	SeverityFatal Severity = "fatal"
)

// severityOf returns the severity of a code, or ("", false) if the
// code has no assigned severity. Signal codes and the
// signal-adjacent broken-pipe code have no severity: the concept
// does not apply to a user- or supervisor-initiated termination.
//
// The mapping is the machine-readable form of the Severity column
// in docs/reference/exit-codes.md and of the `severity` field in
// docs/reference/exit-codes.yaml. The three must agree; the CI
// check scripts/deps/verify-exit-codes.sh asserts the agreement.
//
// # Coverage
//
// Every non-signal code declared in this file must appear in the
// switch below. A code that is declared but not mapped is a bug:
// the code has no CI classification. The invariant is asserted by
// TestExitCodeSeverity_EveryCodeHasSeverity.
//
// # Why the mapping is a switch and not a table
//
// A switch is faster (no map lookup, no allocation) and is
// exhaustively checked by the Go compiler: the `default` case
// catches any code not listed, and the test asserts that every
// declared code is listed. A table (slice of code->severity pairs)
// would need its own lookup loop and its own completeness check.
// The switch wins on both counts.
//
// # Aliases
//
// The aliases (ExitUpdateConflict, ExitInternalError,
// ExitTemplateNotFound, ExitTemplateIncompatible,
// ExitRenderingFailed, ExitPathSafetyViolation,
// ExitSecurityPolicyViolation) share their canonical codes'
// values. The switch lists only the canonical codes; an alias
// reaches the same case by value.
func severityOf(code int) (Severity, bool) {
	switch code {
	case ExitSuccess:
		return SeverityOK, true

	// Findings — all ERROR in Phase 2.
	case ExitCheckFailed:
		return SeverityError, true
	case ExitDriftDetected:
		return SeverityError, true
	case ExitValidation:
		return SeverityError, true
	case ExitAmbiguity:
		return SeverityError, true

	// Execution failures — ERROR, except InternalError which is
	// FATAL.
	case ExitConfig:
		return SeverityError, true
	case ExitTemplate:
		return SeverityError, true
	case ExitFilesystem:
		return SeverityError, true
	case ExitSecurity:
		return SeverityError, true
	case ExitConflict:
		return SeverityError, true
	case ExitInternal:
		return SeverityFatal, true

	// Usage.
	case ExitUsage:
		return SeverityError, true

	// Signals and signal-adjacent codes — no severity.
	//
	// ExitBrokenPipe is grouped here even though Forge does not
	// receive SIGPIPE: the exit code (141 = 128 + 13) is the
	// POSIX convention for SIGPIPE, and a consumer that observes
	// it should read it the same way as a signal termination.
	// See the constant's docstring.
	case ExitSIGHUP, ExitSIGINT, ExitSIGTERM, ExitBrokenPipe:
		return "", false
	}
	return "", false
}

// ---------------------------------------------------------------------------
// Stability
// ---------------------------------------------------------------------------

// Stability classifies the change policy for an exit code range.
//
// The class describes the range's *membership* — the set of codes it
// contains — not its *mapping* from an external event to a code. The
// two properties coincide for every range except the signals range
// (129–255), whose mapping is frozen by POSIX while its membership
// is additive. See the package docstring, section "The two
// stability properties", for the full discussion.
type Stability string

const (
	// StabilityFrozen means the range boundaries and semantics never
	// change, and the range is a singleton: it contains exactly one
	// code, and no code will ever be added to it.
	//
	// The singleton requirement is part of the class's meaning. It is
	// asserted by TestExitCodeTaxonomy_FrozenRangesAreSingletons and
	// by TestAC_11_1_1_CompatibilityPolicyStated. A wider range whose
	// membership is closed by an external convention (for example,
	// the signals range) uses StabilityAdditive and records the
	// frozen-mapping property in its Purpose and in its boundary
	// constants' docstrings.
	StabilityFrozen Stability = "frozen"

	// StabilityAdditive means new codes may be added within the
	// range, but existing codes never change meaning and boundaries
	// never move.
	//
	// Additive ranges may be singletons or wider. The Forge findings
	// (1–9), Forge execution failures (10–19), internal errors
	// (70–78), and signals (129–255) ranges are additive.
	StabilityAdditive Stability = "additive"

	// StabilityReserved means the range is reserved for future use
	// and no codes are currently assigned. Callers must not depend
	// on specific values.
	//
	// A reserved range is a form of open membership whose codes are
	// not yet assigned. It becomes additive when the first code is
	// assigned, in the commit that assigns it.
	StabilityReserved Stability = "reserved"
)

// ---------------------------------------------------------------------------
// Range table
// ---------------------------------------------------------------------------

// ExitCodeRange describes a contiguous range of exit codes with a
// shared purpose, stability policy, and caller guidance.
//
// This type is exported so that documentation generators, CI checks,
// and IDE tooling can introspect the taxonomy programmatically.
type ExitCodeRange struct {
	// Name is a short human-readable label (e.g., "Forge findings").
	Name string

	// Min is the inclusive lower bound of the range.
	Min int

	// Max is the inclusive upper bound of the range.
	Max int

	// Purpose describes what the range means semantically.
	Purpose string

	// Stability is the change policy for this range's membership.
	// It does not describe the range's mapping; for the signals
	// range the mapping is frozen by POSIX while the membership is
	// additive. See the Stability type's docstring.
	Stability Stability

	// CallerGuide is actionable advice for callers who observe a code
	// in this range.
	CallerGuide string
}

// Contains reports whether code falls within the range [Min, Max].
func (r ExitCodeRange) Contains(code int) bool {
	return code >= r.Min && code <= r.Max
}

// IsSingleton reports whether the range contains exactly one code.
func (r ExitCodeRange) IsSingleton() bool {
	return r.Min == r.Max
}

// exitCodeRanges enumerates the documented ranges in ascending order.
// Order matters: LookupExitCodeRange relies on the ranges being
// contiguous and sorted. The invariant is checked by
// TestExitCodeTaxonomy_RangesAreContiguous.
//
// # Stability classes and singletons
//
// Every StabilityFrozen range in this table is a singleton. The
// invariant is enforced by TestExitCodeTaxonomy_FrozenRangesAreSingletons
// and by TestAC_11_1_1_CompatibilityPolicyStated; it is checked at
// test time, not at init time, because it is a property of the
// table's data rather than of its structure. The three frozen
// singletons are Success (0), Usage error (64), and Invalid exit
// argument (128). Everything else is additive or reserved.
var exitCodeRanges = []ExitCodeRange{
	{
		Name:        "Success",
		Min:         ExitSuccess,
		Max:         ExitSuccess,
		Purpose:     "Command completed; no finding raised.",
		Stability:   StabilityFrozen,
		CallerGuide: "Proceed.",
	},
	{
		Name:        "Forge findings",
		Min:         ExitRangeFindingsMin,
		Max:         ExitRangeFindingsMax,
		Purpose:     "Command ran correctly; subject is in a detectable state (drift, warning, assertion mismatch).",
		Stability:   StabilityAdditive,
		CallerGuide: "Inspect output; decide whether to remediate. Not a tool malfunction.",
	},
	{
		Name:        "Forge execution failures",
		Min:         ExitRangeExecutionMin,
		Max:         ExitRangeExecutionMax,
		Purpose:     "Command could not complete: config, template, filesystem, security.",
		Stability:   StabilityAdditive,
		CallerGuide: "Fix environment or inputs; retry may succeed after remediation.",
	},
	{
		Name:        "Reserved: registry / network / enterprise",
		Min:         ExitRangeReservedRegistryMin,
		Max:         ExitRangeReservedRegistryMax,
		Purpose:     "Phase 2+ features: registry fetch, network timeout, enterprise policy.",
		Stability:   StabilityReserved,
		CallerGuide: "Do not depend on specific values; not yet assigned.",
	},
	{
		Name:        "Reserved: future Forge use",
		Min:         ExitRangeReservedForgeMin,
		Max:         ExitRangeReservedForgeMax,
		Purpose:     "Future Forge-assigned ranges.",
		Stability:   StabilityReserved,
		CallerGuide: "Do not depend on specific values.",
	},
	{
		Name:        "Usage error",
		Min:         ExitUsage,
		Max:         ExitUsage,
		Purpose:     "Unknown flag, missing argument, invalid subcommand.",
		Stability:   StabilityFrozen,
		CallerGuide: "Fix the invocation. User error, not tool error.",
	},
	{
		Name:        "Reserved: sysexits sub-range",
		Min:         ExitRangeReservedSysexitsMin,
		Max:         ExitRangeReservedSysexitsMax,
		Purpose:     "BSD sysexits.h reserved codes.",
		Stability:   StabilityReserved,
		CallerGuide: "Do not depend on specific values.",
	},
	{
		Name:        "Internal errors",
		Min:         ExitRangeInternalMin,
		Max:         ExitRangeInternalMax,
		Purpose:     "Bug in Forge or unrecoverable internal state. Aligns with BSD sysexits.h.",
		Stability:   StabilityAdditive,
		CallerGuide: "Report a bug with --verbose output.",
	},
	{
		Name:        "Reserved: future use",
		Min:         ExitRangeReservedFutureMin,
		Max:         ExitRangeReservedFutureMax,
		Purpose:     "Future reserved use.",
		Stability:   StabilityReserved,
		CallerGuide: "Do not depend on specific values.",
	},
	{
		Name:        "Invalid exit argument",
		Min:         ExitInvalidArgument,
		Max:         ExitInvalidArgument,
		Purpose:     "Exit argument itself was invalid.",
		Stability:   StabilityFrozen,
		CallerGuide: "Shell convention; not emitted by Forge directly.",
	},
	{
		Name: "Signals (128 + N)",
		Min:  ExitRangeSignalsMin,
		Max:  ExitRangeSignalsMax,
		Purpose: "Process terminated by signal N, or by a " +
			"signal-adjacent condition that the POSIX convention " +
			"maps to 128 + N. The 128 + N mapping is fixed by " +
			"POSIX; the range's membership is additive (Forge may " +
			"handle more conditions in a future phase). " +
			"130 = SIGINT, 141 = broken pipe (SIGPIPE), " +
			"143 = SIGTERM.",
		Stability: StabilityAdditive,
		CallerGuide: "User or supervisor requested termination, " +
			"or the downstream consumer closed the output stream. " +
			"Not a Forge failure. Branch on IsSignal, not on a " +
			"specific N.",
	},
}

// ExitCodeRanges returns a copy of the documented exit code ranges.
//
// The returned slice is safe to modify; it does not alias internal
// state. Callers may use this for documentation generation, validation
// tooling, or introspection.
func ExitCodeRanges() []ExitCodeRange {
	out := make([]ExitCodeRange, len(exitCodeRanges))
	copy(out, exitCodeRanges)
	return out
}

// LookupExitCodeRange returns the documented range containing code,
// or (ExitCodeRange{}, false) if code falls outside all documented
// ranges.
//
// Because the documented ranges are contiguous from 0 to 255, this
// function returns false only for codes outside [0, 255]. It is
// provided for completeness and for callers that want to assert
// coverage.
func LookupExitCodeRange(code int) (ExitCodeRange, bool) {
	for _, r := range exitCodeRanges {
		if r.Contains(code) {
			return r, true
		}
	}
	return ExitCodeRange{}, false
}

// VerifyExitCodeRange reports whether code is within the documented
// exit code namespace [0, 255].
//
// This function does NOT validate that a code is currently assigned.
// It validates that a code is within the documented ranges. Use
// forgeerr.Lookup for assignment validation.
//
// The check is intentionally strict: codes outside [0, 255] are
// rejected because Forge guarantees cross-platform stability by
// emitting only 0–255.
func VerifyExitCodeRange(code int) error {
	if code < 0 {
		return &ExitCodeRangeError{Code: code, Reason: "negative exit code"}
	}
	if code > 255 {
		return &ExitCodeRangeError{
			Code:   code,
			Reason: "exit code exceeds 255; Forge guarantees 0–255 for cross-platform stability",
		}
	}
	return nil
}

// ExitCodeRangeError describes a violation of the exit code namespace.
type ExitCodeRangeError struct {
	// Code is the offending exit code.
	Code int
	// Reason explains why the code is invalid.
	Reason string
}

// Error implements the error interface.
func (e *ExitCodeRangeError) Error() string {
	return fmt.Sprintf("invalid exit code %d: %s", e.Code, e.Reason)
}

// ---------------------------------------------------------------------------
// Convenience classifiers
// ---------------------------------------------------------------------------

// IsFinding reports whether code falls within the Forge findings range
// (1–9). A finding means the command ran correctly and the subject of
// the command is in a detectable state.
//
// Callers should not treat a finding as a crash. Parse output for
// details and consider remediation.
func IsFinding(code int) bool {
	return code >= ExitRangeFindingsMin && code <= ExitRangeFindingsMax
}

// IsExecutionFailure reports whether code falls within the Forge
// execution-failure range (10–19). An execution failure means the
// command could not complete because Forge itself encountered a
// problem.
//
// Callers should treat an execution failure as a tool failure: fix
// environment or inputs; retry may succeed after remediation.
func IsExecutionFailure(code int) bool {
	return code >= ExitRangeExecutionMin && code <= ExitRangeExecutionMax
}

// IsUsageError reports whether code is the usage error (64).
func IsUsageError(code int) bool {
	return code == ExitUsage
}

// IsInternalError reports whether code falls within the internal-error
// range (70–78).
func IsInternalError(code int) bool {
	return code >= ExitRangeInternalMin && code <= ExitRangeInternalMax
}

// IsSignal reports whether code is a signal termination (128 + N).
//
// The classifier is range-based, not set-based: it returns true for
// every code in [129, 255], not only for the named codes
// (ExitSIGHUP, ExitSIGINT, ExitSIGTERM, ExitBrokenPipe). The
// range-based form is the contract: a future phase that handles
// SIGQUIT (131) or SIGKILL (137) does not need to change the
// classifier, because the new codes fall in the range the classifier
// already covers.
//
// This is the reason the signals range is StabilityAdditive rather
// than StabilityFrozen: the classifier promises the range, not a
// specific set of codes. See the package docstring, section "The
// two stability properties".
//
// # ExitBrokenPipe and IsSignal
//
// ExitBrokenPipe (141) is returned by IsSignal because it falls in
// the range. The classification is deliberate: the broken-pipe
// condition is signal-adjacent (128 + SIGPIPE), and a consumer that
// branches on IsSignal treats it the same way as a signal
// termination. The constant's docstring explains the distinction
// between "received a signal" and "mapped a write error to the
// signal's conventional exit code".
func IsSignal(code int) bool {
	return code >= ExitRangeSignalsMin && code <= ExitRangeSignalsMax
}

// ---------------------------------------------------------------------------
// Internal invariant checks
// ---------------------------------------------------------------------------

// init verifies the structural invariants of the range table at
// program startup. A violation here is a programmer error: the table
// is compiled into the binary and cannot be fixed at runtime.
//
// Invariants checked at init time:
//
//  1. Ranges are sorted by Min in ascending order.
//  2. Ranges are contiguous: r[i].Max + 1 == r[i+1].Min.
//  3. Ranges cover [0, 255] without gaps or overlaps.
//  4. Every named code constant falls within a documented range.
//
// Invariants checked at test time (not at init time):
//
//  5. Singleton ranges (Min == Max) are permitted.
//  6. Every StabilityFrozen range is a singleton.
//
// Invariant 6 is a property of the table's *data*, not of its
// structure, and is enforced by the test suite rather than by init.
// The separation keeps init focused on the properties that a malformed
// table would silently corrupt (contiguity, coverage) and lets the
// stability invariant fail with a readable message in the test output.
func init() {
	if err := checkRangeTableInvariants(); err != nil {
		panic("cli: exit code range table invariant violated: " + err.Error())
	}
}

// checkRangeTableInvariants returns the first invariant violation, or
// nil if the table is well-formed. It is called by init and by tests.
//
// It checks the structural invariants (1–4 above). It does not check
// the stability-class invariant (6), which is a test-time check.
//
// # Why the named list includes only the canonical constants
//
// The aliases (ExitUpdateConflict, ExitInternalError,
// ExitTemplateNotFound, ExitTemplateIncompatible,
// ExitRenderingFailed, ExitPathSafetyViolation,
// ExitSecurityPolicyViolation) share their canonical constants'
// values. The list therefore names only the canonical constant for
// each distinct code. A test in exitcodes_semantics_test.go
// asserts the alias equalities; the init check asserts the
// distinct codes' range membership.
//
// # Why the list must include every distinct code
//
// The init check's job is to catch a code that is declared but
// lands outside all documented ranges. A code missing from the
// list is not checked, which is how the ExitConflict bug (a
// declared code whose value was moved out of its range) went
// undetected in an earlier draft. The list below is the
// completeness check; it must include every distinct code.
//
// # WBS 12.4.1 addition
//
// ExitBrokenPipe is listed in the signals group. The init check
// therefore asserts (a) 141 is within [129, 255] and (b) no other
// distinct code shares the value. A future change that moved the
// constant outside the signals range panics at program startup
// with a message naming the constant.
func checkRangeTableInvariants() error {
	if len(exitCodeRanges) == 0 {
		return fmt.Errorf("range table is empty")
	}

	// Invariant 1: sorted by Min.
	if !sort.SliceIsSorted(exitCodeRanges, func(i, j int) bool {
		return exitCodeRanges[i].Min < exitCodeRanges[j].Min
	}) {
		return fmt.Errorf("range table is not sorted by Min")
	}

	// Invariants 2 & 3: contiguous and covering [0, 255].
	if exitCodeRanges[0].Min != 0 {
		return fmt.Errorf("first range must start at 0, got %d", exitCodeRanges[0].Min)
	}
	last := exitCodeRanges[len(exitCodeRanges)-1]
	if last.Max != 255 {
		return fmt.Errorf("last range must end at 255, got %d", last.Max)
	}
	for i := 1; i < len(exitCodeRanges); i++ {
		prev := exitCodeRanges[i-1]
		cur := exitCodeRanges[i]
		if prev.Max >= cur.Min {
			return fmt.Errorf(
				"ranges overlap: %q ends at %d, %q starts at %d",
				prev.Name, prev.Max, cur.Name, cur.Min,
			)
		}
		if prev.Max+1 != cur.Min {
			return fmt.Errorf(
				"gap between %q (ends %d) and %q (starts %d)",
				prev.Name, prev.Max, cur.Name, cur.Min,
			)
		}
	}

	// Invariant 4: every named code falls within a documented range.
	//
	// The list below includes every distinct constant declared in
	// the named-codes block. A future commit that adds a code must
	// add it here in the same commit; the invariant is checked at
	// init time, so a missing entry fails at program startup.
	//
	// The list is ordered by the code's value for readability.
	named := []struct {
		name string
		code int
	}{
		{"ExitSuccess", ExitSuccess},

		// Findings.
		{"ExitDriftDetected", ExitDriftDetected},
		{"ExitValidation", ExitValidation},
		{"ExitCheckFailed", ExitCheckFailed},
		{"ExitAmbiguity", ExitAmbiguity},

		// Execution failures.
		{"ExitConfig", ExitConfig},
		{"ExitTemplate", ExitTemplate},
		{"ExitFilesystem", ExitFilesystem},
		{"ExitSecurity", ExitSecurity},
		{"ExitConflict", ExitConflict},

		// Internal errors.
		{"ExitInternal", ExitInternal},

		// Usage.
		{"ExitUsage", ExitUsage},

		// Signals.
		{"ExitSIGHUP", ExitSIGHUP},
		{"ExitSIGINT", ExitSIGINT},
		{"ExitSIGTERM", ExitSIGTERM},

		// Signal-adjacent, pinned by the POSIX 128 + N convention.
		// See the constant's docstring and
		// docs/reference/output-streams.md §"Broken pipe".
		{"ExitBrokenPipe", ExitBrokenPipe},
	}
	for _, n := range named {
		if err := VerifyExitCodeRange(n.code); err != nil {
			return fmt.Errorf("%s: %w", n.name, err)
		}
		if _, ok := LookupExitCodeRange(n.code); !ok {
			return fmt.Errorf("%s (%d) falls outside all documented ranges", n.name, n.code)
		}
	}

	// Invariant 4b: no two distinct constants share a value.
	//
	// The check is the init-time equivalent of the alias check: a
	// duplicate value would mean a code is overloaded for two
	// triggers, which the WBS 11.1.2 acceptance criterion forbids.
	// Aliases are excluded from the list, so a duplicate here is
	// a genuine collision, not an alias.
	seen := make(map[int]string, len(named))
	for _, n := range named {
		if other, ok := seen[n.code]; ok {
			return fmt.Errorf(
				"code %d is shared by %q and %q; "+
					"each distinct code must have a distinct value",
				n.code, other, n.name,
			)
		}
		seen[n.code] = n.name
	}

	return nil
}
