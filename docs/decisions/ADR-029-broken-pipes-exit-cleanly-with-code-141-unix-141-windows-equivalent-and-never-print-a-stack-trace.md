# ADR-029: Broken pipes exit cleanly with code 141 (Unix) / 141-equivalent (Windows) and never print a stack trace

**Status:** Accepted
**Date:** 2026-10-13
**Deciders:** [@thapelomagqazana] (Core Runtime Owner)
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge's commands are frequently used in shell pipelines: `forge
config --path | head -1`, `forge version | grep commit`, `forge
--verbose version 2>&1 | tail`. In each of these, the downstream
consumer may close the pipe before Forge has finished writing —
because `head -1` exits after the first line, because `grep` exits
after a match, or because the consumer crashed. When the downstream
consumer closes the pipe, the next write to stdout by Forge fails
with a "broken pipe" error (`EPIPE` on Unix, `ERROR_BROKEN_PIPE` on
Windows). A CLI that does not handle this condition will either
crash with a Go panic (producing a stack trace on stderr and a
non-Forge exit code) or exit with an unrelated error code, breaking
the pipeline's downstream logic and cluttering the terminal with
information the user did not ask for.

The current codebase does not have a specified behavior for this
condition. `internal/streams` (WBS 12.4.1) classifies a broken pipe
in its `IsBrokenPipe` function, but the classifier's result is not
yet wired to the exit boundary. The exit boundary in
`internal/cli/execute.go` maps errors to exit codes via
`forgeerr.Category().ExitCode()`, but the error taxonomy (ADR-011)
does not have a category for "the stream closed"; a broken-pipe
error would be classified as either the internal-error category
(if the error is a plain error) or a stream-classified category
(if the error is wrapped as a `forgeerr.Error`). Neither is the
right classification: the pipe closing is not a Forge failure; it
is the consumer's decision.

The forces at play are three. First, the pipeline's *exit code* is
a signal the downstream consumer's shell uses to decide whether the
upstream command succeeded. `head -1` does not care whether Forge
produced one line or a thousand; it cares whether Forge exited
cleanly. A Forge exit code of 1 (some generic failure) would make
the shell's `set -e` abort a script that was working as intended. A
Forge exit code of 0 would be misleading: the command did not
complete; it was terminated. The correct signal is the POSIX
convention for "the process was terminated by SIGPIPE" — exit code
141 (128 + 13) on Unix. Second, the *diagnostic surface* must not
emit a stack trace. A Go panic's stack trace is a developer-facing
artefact; a user who runs `forge version | head -1` should see the
version line and nothing else, not a panic trace on stderr. The
stack trace is also long and would flood the terminal or the log
aggregator. Third, the exit code and the diagnostic surface must be
*consistent across platforms*. Unix's SIGPIPE convention gives 141;
Windows does not have SIGPIPE, and the closest convention is 141 as
well (the same 128 + 13 arithmetic, though the "signal" is
simulated). A user who runs the same command on Linux and Windows
should see the same exit code and the same absence of a stack trace.

The decision relates to ADR-011 (the error category taxonomy),
ADR-018 (the exit code stability policy), ADR-020 (the single exit
boundary in main), ADR-022 (the stream contract), ADR-028 (the
boundary's enforcement of the stream contract), and ADR-025 (the
log record shape, which the diagnostic must respect). The CLI UX
spec's § 4.12 mentions the `--verbose` flag's interaction with
pipelines but does not define the broken-pipe behavior. If no
decision is made, Forge's behavior under a closed pipe is
undefined: it may crash, it may exit with an unrelated code, or it
may produce a stack trace — all of which break the pipeline's
expectations.

---

## 2. Options Considered

### Option A — Let the broken-pipe error propagate as a generic error

The broken-pipe error is not special-cased. A write to stdout that
fails with `EPIPE` returns an error to the caller; the error is
classified by the normal taxonomy (either the internal-error
category, if it is a plain error, or the CLI category, if it is
wrapped as a `forgeerr.Error`). The exit code is whatever the
taxonomy produces — 70 (internal error) for a plain error, 2
(usage error) if classified as CLI, or some other code depending
on the wrapping.

**Pros:**

- No special-case handling; the error flows through the same path
  as every other error.
- The error taxonomy is not extended with a new category; the
  existing categories suffice.
- The behavior is simple: every error is treated the same way.

**Cons:**

- The exit code is wrong. A pipeline that terminates Forge by
  closing the pipe produces a non-Forge exit code (70 or 2),
  which misleads the shell's `set -e` into thinking Forge failed
  when it was actually the pipeline's design.
- The diagnostic surface is cluttered. The error is rendered as
  a `forgeerr.Error` block on stderr; a user who runs `forge
  version | head -1` sees the version line followed by an error
  block explaining that a write to stdout failed — information
  the user did not ask for and did not need.
- The `--format json` rule (ADR-021) is violated: the error
  envelope would be written to stdout, which is already closed
  by the downstream consumer; the write would fail again, and
  the behavior would cascade.
- The behavior is platform-inconsistent: on Unix the shell may
  see the Forge process terminated by SIGPIPE (if the signal
  is not ignored) and produce 141 anyway, while on Windows the
  process exits normally with the taxonomy's code. The
  inconsistency is confusing.
- A future redaction rule (ADR-033, ADR-034) that applies to the
  error block would also apply to the broken-pipe error, but the
  broken-pipe error is not a user-facing condition; the
  redaction would be applied to a block that should not be
  written at all.

### Option B — Recover the broken pipe at the exit boundary and exit with code 141

The exit boundary (`internal/cli/execute.go`) recognizes a
broken-pipe error before rendering it. When a write to stdout fails
with `EPIPE`, the boundary returns exit code 141 (128 + SIGPIPE) on
Unix and 141 on Windows (the same arithmetic), and does not render
the error. Any in-flight output is flushed; any partial line is
truncated at the byte boundary the write reached.

**Pros:**

- The exit code matches the POSIX convention (141) that every
  shell and every pipeline expects. A pipeline that closes the
  pipe produces the same code Forge would produce if it were
  terminated by SIGPIPE directly.
- The diagnostic surface is clean: no error block, no stack
  trace. The user sees the version line and nothing else.
- The behavior is platform-consistent: Unix and Windows both
  exit with 141.
- The `--format json` rule is respected: the error envelope is
  not written to a closed stdout; the boundary recognizes the
  condition and exits without attempting a second write.
- The behavior is testable at the byte level: a test that pipes
  Forge's stdout to a process that closes the pipe after reading
  one byte asserts on the exit code and on stderr's emptiness.
- The pipeline's downstream logic works as intended: `set -e`
  does not abort, `grep` exits after its match, and the user
  sees the output they asked for.

**Cons:**

- The boundary must recognize the broken-pipe condition, which
  requires classifying the write error. The classification is
  platform-specific (`EPIPE` on Unix, `ERROR_BROKEN_PIPE` on
  Windows), which is why the `internal/streams` package's
  `IsBrokenPipe` function exists. The boundary must import and
  use it.
- The exit code 141 is not in the CLI's exit-code taxonomy
  (ADR-011, ADR-018); it is a POSIX convention, not a Forge
  category. The boundary's return of 141 bypasses the
  taxonomy's category-based mapping. The bypass is a
  documented exception that must not be confused with the
  taxonomy's codes.
- The behavior must be specified precisely: what happens if
  stderr is also closed? (The write to stderr fails; the
  boundary must not cascade.) What happens if the broken pipe
  occurs mid-line? (The line is truncated; the boundary does
  not attempt to complete it.) What happens if the broken pipe
  occurs during the error path itself? (The boundary must not
  attempt to render an error to a closed stdout.)
- The behavior is Unix-centric: the POSIX convention's exit code
  (141) is well-understood on Unix, but on Windows the
  convention is less common, and a user who sees 141 on Windows
  may not recognize it. The trade-off is between platform
  consistency (141 on both) and platform-specific clarity
  (Windows could use a different code that Windows users
  recognize).

### Option C — Recover the broken pipe and exit with code 0

The boundary recognizes the broken pipe and treats it as a
successful termination. The command completed its work; the
downstream consumer chose to stop reading; the exit code is 0.

**Pros:**

- No new exit code; the pipeline's shell sees the Forge process
  exit cleanly.
- The `set -e` behavior is preserved: a script that used `set -e`
  does not abort because Forge exited 0.
- The diagnostic surface is clean: no stack trace, no error
  block.

**Cons:**

- The exit code is misleading. The command did not complete; it
  was terminated. Exiting 0 tells the shell that Forge succeeded
  when Forge was interrupted by the consumer's decision to stop
  reading. A script that inspects the exit code cannot
  distinguish "Forge finished and wrote everything" from "Forge
  was cut off after one line."
- The POSIX convention for "the process was terminated by
  SIGPIPE" is 141, not 0. A CLI that exits 0 on a broken pipe
  is inconsistent with every Unix tool that handles the
  condition (for example, `yes | head`, `cat bigfile | head`,
  `seq 1e9 | head`). The tools' behavior is well-understood by
  shell authors; Forge's divergence would surprise them.
- The `--format json` rule (ADR-021) is respected at the byte
  level, but the rule's semantics are unclear when the pipe is
  closed mid-document: the consumer receives a partial JSON
  document, and a script that parses the output would fail with
  a parse error, not with a clean "Forge exited 0" signal.
- A future test that asserts "Forge exits non-zero when its
  output is truncated" would fail; the assertion would need to
  be specific to the broken-pipe case, which is a smaller
  target than a uniform exit code.

### Option D — Recover the broken pipe at the boundary, exit 141, and log a DEBUG record

Same as Option B, plus a DEBUG record emitted at the boundary
recording the broken-pipe event. The record is emitted through the
logger (so it goes to stderr, and is subject to `--verbose`).

**Pros:**

- All of Option B's pros.
- A user running `forge --verbose version | head -1` sees a DEBUG
  record explaining why Forge exited 141, which is useful for
  diagnosing an unexpected pipeline exit. Without the record, the
  user sees 141 and must infer its meaning.

**Cons:**

- The DEBUG record is subject to `--verbose`; a user who does not
  pass `--verbose` sees no record, which is the correct default
  behavior (the broken pipe is not a diagnostic the user asked
  for), but the record's absence is a source of "why did Forge
  exit 141?" questions from users who did not enable verbose.
- The record is emitted to stderr, which may also be closed (if
  the pipeline closes both streams). The emission must tolerate
  the closed stderr without cascading.
- The record's presence in the log stream may confuse a log
  aggregator that expects every record to be a completed event;
  the broken-pipe record is a record about the process's own
  termination, which is unusual.
- The record's presence adds a diagnostic to a condition that is
  not a failure, which blurs the boundary between "diagnostics
  about how the command ran" and "the command's termination
  reason." The distinction is one the logger's contract does not
  currently make.

---

## 3. Decision

**Forge adopts Option B: the exit boundary recognizes a
broken-pipe condition and exits with code 141 on Unix and 141 on
Windows, without rendering an error block and without emitting a
stack trace.** The recognition is performed by
`internal/streams`'s `IsBrokenPipe` function, which classifies the
platform-specific error (`EPIPE` on Unix, `ERROR_BROKEN_PIPE` on
Windows). The boundary checks the classification before rendering
the error; if the condition is a broken pipe, the boundary returns
141 without writing to stdout or stderr. Any in-flight output is
flushed; a partial line is truncated at the byte boundary the write
reached.

The exit code 141 is the POSIX convention for "terminated by
SIGPIPE" (128 + 13). It is not a Forge category (ADR-011 does not
define it), and the boundary's return of 141 bypasses the
category-based exit-code mapping. The bypass is documented in
`internal/cli/execute.go`'s docstring and in
`docs/reference/output-streams.md`; the reference doc lists 141 as
a documented exit code in the signals range (129–255), alongside
130 (SIGINT) and 143 (SIGTERM). Windows uses the same code for
consistency with Unix; a Windows user who sees 141 and does not
recognize it finds the reference doc's explanation.

The reasoning that tipped the balance was the pipeline's exit-code
semantics. A pipeline's downstream consumer (a shell script, a
Makefile, a CI runner) reads Forge's exit code to decide whether to
continue. `head -1`'s early exit is a normal, intentional pattern;
a Forge exit code of 141 matches the convention every Unix tool
follows, and a Forge exit code of 0 (Option C) would tell the shell
that Forge succeeded when it was cut off. The choice between 141
and 0 is not a matter of taste; it is the difference between a
pipeline that works as designed and one that lies to its consumer.
Option A (no special handling) fails the same test: a generic exit
code (70 or 2) makes the pipeline's shell think Forge failed, and
the failure's diagnostic clutters the terminal with information the
user did not ask for. Option D (Option B plus a DEBUG record) is
defensible but is rejected because the record's presence requires
`--verbose`, and the broken-pipe condition is one where the user
did not ask for diagnostics — the condition is the pipeline's
design, not a problem to diagnose.

The reasoning against emitting a stack trace is the same reasoning
that applies to every other error path: a user-facing CLI shows a
user-facing message, not a developer-facing stack trace. The
current codebase already suppresses stack traces for the
non-broken-pipe error paths (the `forgeerr` rendering shows the
cause chain, not a Go stack trace, unless `--verbose` is set); the
broken-pipe condition's suppression is consistent with that
behavior. A stack trace on a broken pipe would be the worst of both
worlds: a user who ran `forge version | head -1` would see the
version line on stdout and a Go stack trace on stderr, which is
exactly the outcome the option exists to prevent.

The tradeoffs accepted are the ones Option B lists as cons. The
boundary's recognition of the broken-pipe condition is a small
amount of platform-specific code (a call to `IsBrokenPipe`); the
condition's specification must be precise (what happens if stderr
is also closed; what happens if the broken pipe occurs mid-line;
what happens if the broken pipe occurs during the error path);
and the exit code 141 on Windows is a convention that Windows users
may not recognize, accepted in exchange for platform consistency.
The reference doc's signals section (129–255) is the
authoritative statement of 141's meaning; a user who sees the code
on Windows and does not recognize it finds the explanation there.

The decision is provisional in one respect: the boundary's exact
handling of the "both streams closed" case (stdout closed by the
consumer, stderr closed by the same consumer or by a redirection)
is a matter of robustness, not of contract. The current design
returns 141 without attempting a write to either stream; a future
ADR could refine the behavior (for example, attempting a write to
stderr and ignoring the failure). Until then, the behavior is
"return 141 without writing," which is the safest choice (a write
to a closed stream fails, and the failure is not actionable per
ADR-022).

---

## 4. Consequences

### 4.1 Positive

- The pipeline's exit code matches the POSIX convention (141)
  that every shell and every pipeline expects. A script that
  uses `set -e` does not abort; a script that inspects the
  exit code distinguishes "Forge was cut off" from "Forge
  failed."
- The diagnostic surface is clean: no error block, no stack
  trace. A user who runs `forge version | head -1` sees the
  version line and nothing else; the terminal is not cluttered
  with information the user did not ask for.
- The behavior is platform-consistent: Unix and Windows both
  exit with 141. A CI script that runs the same command on
  both platforms sees the same result.
- The `--format json` rule (ADR-021) is respected: the error
  envelope is not written to a closed stdout; the boundary
  recognizes the condition and exits without attempting a
  second write. A machine consumer that reads the partial
  JSON document knows the stream was cut off (the document
  is incomplete, and the exit code is 141).
- The condition is testable at the byte level: a test that
  pipes Forge's stdout to a process that closes the pipe
  after reading one byte asserts on the exit code (141) and
  on stderr's emptiness. The test is platform-specific in its
  setup but platform-independent in its assertions.
- The behavior is consistent with the tools Forge's users
  already know: `yes | head`, `cat bigfile | head`, `seq 1e9
  | head` all exit 141 (or are terminated by SIGPIPE). Forge's
  behavior is what a shell author expects.

### 4.2 Negative

- The boundary must recognize the broken-pipe condition, which
  requires importing `internal/streams` and classifying the
  write error. The classification is platform-specific
  (`EPIPE` on Unix, `ERROR_BROKEN_PIPE` on Windows); the
  `IsBrokenPipe` function exists for this purpose, but its use
  in the boundary is a small coupling.
- The exit code 141 is not in the CLI's exit-code taxonomy
  (ADR-011, ADR-018); it is a POSIX convention, and the
  boundary's return of 141 bypasses the taxonomy's category-
  based mapping. The bypass is documented, but it is an
  exception to the taxonomy's uniform mapping.
- The condition's specification must be precise: what happens
  if stderr is also closed (the boundary must not cascade);
  what happens if the broken pipe occurs mid-line (the line
  is truncated); what happens if the broken pipe occurs
  during the error path (the boundary must not attempt a
  second write). The specification is documented in
  `docs/reference/output-streams.md` and in
  `internal/cli/execute.go`'s docstring, but it must be
  maintained.
- The exit code 141 on Windows is a convention that Windows
  users may not recognize. The reference doc explains it, but
  a Windows user who sees 141 and does not read the doc may
  be confused. The alternative — a Windows-specific code —
  would break platform consistency, which is a worse trade.
- The behavior is Unix-centric: the POSIX convention (141) is
  well-understood on Unix, and the platform-specific setup
  (a pipeline whose consumer closes the pipe) is
  Unix-flavored. A Windows user who is not familiar with
  pipelines may not encounter the condition, but if they do,
  the code's meaning is less obvious.

### 4.3 Neutral

- The exit code 141 is in the signals range (129–255) of the
  exit-code taxonomy; it is the same range as 130 (SIGINT)
  and 143 (SIGTERM). The range's documentation in
  `docs/reference/exit-code-ranges.md` covers 141; the
  reference doc's signals section is the authoritative
  statement.
- The boundary's return of 141 without writing to stderr is
  the safest choice; a write to stderr might also fail (if
  stderr is closed), and the failure would not be actionable.
  The alternative — attempting a write and ignoring the
  failure — is a small addition that a future ADR could make.
- The behavior under `--format json` is defined: stdout
  contains a partial JSON document (the bytes the consumer
  read before closing the pipe), and the exit code is 141.
  A machine consumer that reads the partial document knows
  the stream was cut off; a consumer that parses the
  document fails with a parse error, which is the correct
  signal.
- The behavior under `--verbose` is defined: the boundary
  does not emit a DEBUG record for the broken-pipe
  condition; the user who wants to diagnose the condition
  reads the exit code (141) and knows its meaning from the
  reference doc. Option D's DEBUG record is not adopted;
  the condition is not a diagnostic the user asked for.
- The behavior under `--quiet` is defined: the boundary's
  return of 141 is unaffected by `--quiet`; the flag
  suppresses informational diagnostics, not the exit code's
  signal.

---

## 5. Related Requirements

- `FR-CLI-LOG-008` — the stream contract is enforced at the
  boundary; the requirement is strengthened by this decision's
  broken-pipe handling.
- `FR-CLI-OUTPUT-011` — the stream contract holds under
  `--format json`; the requirement is strengthened by this
  decision's behavior when stdout is closed.
- `NFR-ARCHITECTURE-004` — no direct writes in domain
  packages; the requirement is compatible with this
  decision's boundary-level handling.
- `NFR-OBSERVABILITY-001` — records are attributable to a
  level; the requirement is compatible with this decision's
  choice not to emit a record for the broken-pipe condition.
- `UX-xxx` — the CLI's behavior under a closed stream is
  defined; the requirement (to be added when WBS 1.4
  completes) is established by this decision.

> The following requirements must be added to the register when
> WBS 12.4.1 is complete:

- `FR-CLI-LOG-008` (if not already present)
- `FR-CLI-OUTPUT-011` (if not already present)
- A new requirement for "broken-pipe handling" if the existing requirements do not cover it.

---

## 6. Related ADRs

- `ADR-011` — Closed Error Category Taxonomy. Establishes the
  categories this decision's broken-pipe handling bypasses (the
  condition is not a category).
- `ADR-018` — Exit Code Migration and Stability Policy.
  Establishes the exit codes' stability; this decision's 141 is
  in the signals range and is compatible with the policy.
- `ADR-020` — Single Exit Boundary in main. Establishes the
  exit boundary this decision's handling extends.
- `ADR-021` — JSON Error Envelope for `--format json`.
  Establishes the error envelope this decision does not write
  when stdout is closed.
- `ADR-022` — Output and logging are separate architectural
  layers. Establishes the stream contract this decision's
  behavior respects.
- `ADR-028` — The stream contract is enforced at the boundary.
  Establishes the boundary this decision's handling extends.
- `ADR-033` (expected) — Sensitive data is redacted at three
  layers. Will define the redaction rules that do not apply to
  the broken-pipe condition (the condition is not logged).

---

## 7. Notes

**Open questions deferred by this ADR:**

- Whether the boundary should attempt a write to stderr when
  both streams are closed, or whether it should return 141
  without attempting any write. The ADR chooses the latter;
  a future ADR could revisit the choice.
- Whether the exit code on Windows should be 141 (consistent
  with Unix) or a Windows-specific code that Windows users
  recognize. The ADR chooses 141 for consistency; a future
  ADR could revisit the choice.
- Whether a DEBUG record should be emitted for the broken-pipe
  condition under `--verbose`. The ADR chooses not to emit;
  the condition is not a diagnostic the user asked for, and
  the exit code 141 is the correct signal. A future ADR
  could add the record if a use case emerges.

**Follow-up decisions anticipated:**

- The implementation of the broken-pipe handling in the
  boundary (`internal/cli/execute.go`) is a WBS item that
  depends on this ADR; the item's output is the code and the
  tests, not an ADR.
- The `internal/streams` package's `IsBrokenPipe` function is
  the classification mechanism this decision relies on; its
  platform-specific implementations (`brokenpipe_unix.go`,
  `brokenpipe_windows.go`) are already in place from WBS
  12.4.1.
- The reference doc's signals section
  (`docs/reference/output-streams.md`) is the authoritative
  statement of 141's meaning; the doc's update is part of
  this decision's implementation.

**External references:**

- POSIX.1-2017 §2.5, "Standard I/O Streams" — the normative
  source for the stdout/stderr split.
- POSIX.1-2017, "SIGPIPE" — the signal whose exit code (128 + 13 = 141) this decision adopts.
- The exit-code conventions of `bash`, `zsh`, and other
  POSIX shells — the precedent for 141's meaning.
- `docs/reference/exit-code-ranges.md` — the exit-code
  taxonomy's ranges, including the signals range (129–255).
- `docs/reference/output-streams.md` — the stream contract's
  authoritative reference, including the signals section that
  documents 141.

---

*End of ADR-029.*
