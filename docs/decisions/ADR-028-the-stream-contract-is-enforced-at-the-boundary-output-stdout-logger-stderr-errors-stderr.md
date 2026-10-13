# ADR-028: The stream contract is enforced at the boundary: output→stdout, logger→stderr, errors→stderr

**Status:** Accepted
**Date:** 2026-10-13
**Deciders:** [@thapelomagqazana] (Core Runtime Owner)
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

ADR-022 established the two-layer architecture: `internal/output`
owns the product surface on stdout, `internal/logger` owns the
operational surface on stderr, and `internal/forgeerr` renders errors
to stderr. ADR-026 defined the `Logger` interface. ADR-027 forbade
direct writes from domain packages. None of these decisions defined
*where in the code the stream contract is enforced* — which function
is responsible for routing bytes to the correct stream, and which
parts of the codebase are trusted to write at all.

Today, the responsibility is distributed: the command tree writes
product output through `deps.Streams.Out`; the logger writes
diagnostics through its `Sink`; `runWithOptions` writes the error
block and the JSON error envelope. Each of these is correct
individually, but the *rule* that ties them together — "the process
boundary is the only place where streams are selected; below it,
every writer receives its destination through injection" — is not
written down. The `streams.Streams` value (WBS 12.4.1) is the
injection seam; `defaultOptions` reads the process's `os.Stdout`,
`os.Stderr`, and `os.Stdin` exactly once at process start; everything
below that point writes through the injected values. The pattern is
visible in the code but not in a decision.

The forces at play are three. First, the CLI's stream contract is
a *runtime* property: it must hold for every invocation, not just
for the invocations the tests cover. A contract that is enforced only
by convention can be silently violated by a new code path; a contract
enforced at a boundary is enforced for every path that crosses the
boundary. Second, the CLI's commands are tested in-process (per WBS
4.3.2's `runCLI` helper), which means the commands' streams are
injected `bytes.Buffer` values, not the process's `os.Stdout` and
`os.Stderr`. A command that wrote to `os.Stdout` directly would
produce test output that the test's buffer does not capture; the
test would pass even though the command's output went to the wrong
place. The injection model is what makes the tests hermetic and the
contract verifiable. Third, the `--format json` rule (ADR-021, WBS
10.0) requires that stdout carries exactly one JSON document; the
rule can only be enforced if no code path other than the JSON
envelope's writer reaches stdout. The boundary is where that
exclusivity is guaranteed.

The decision relates to ADR-020 (the single exit boundary in main,
which is the entry point that installs the injection), ADR-022 (the
stream contract), ADR-026 (the logger's interface), ADR-027 (the
domain-package prohibition), and ADR-021 (the JSON error envelope
that must be the only thing on stdout under `--format json`). The
CLI UX spec's § 4.12 mentions the CLI's stream separation; the
reference doc `docs/reference/output-streams.md` is the current
normative statement of the contract. If no decision is made, the
contract remains a property that holds by accident of the current
code's shape rather than by a boundary that the code crosses.

---

## 2. Options Considered

### Option A — Enforce the contract at every call site

Each function that writes chooses its stream inline. A command that
writes product output calls `cmd.OutOrStdout()`; a diagnostic calls
`log.Print`; an error calls `fmt.Fprintln(os.Stderr, ...)`. There
is no central writer; the contract is a convention that each call
site is expected to honor.

**Pros:**

- No central writer to define, document, or maintain.
- Each call site's stream choice is visible at the call site.
- The code is simple; a new contributor writes what they see.

**Cons:**

- The contract is not enforced. A call site that chooses the wrong
  stream produces the wrong output, and the mistake is silent
  unless a test happens to check it.
- The tests are not hermetic: a command that writes to `os.Stdout`
  directly produces test output that the test's `bytes.Buffer`
  does not capture, so the test passes while the bug ships.
- The `--format json` rule cannot be enforced: no single writer
  owns stdout, so no check can guarantee that stdout is exactly
  one JSON document.
- The redaction rules (ADR-033, ADR-034) cannot be applied
  uniformly: a `log.Print` call that bypasses the logger's `Sink`
  bypasses the redaction middleware.
- The `--verbose` and `--quiet` flags (ADR-030, ADR-031) cannot
  filter output uniformly: each call site would re-implement the
  level check.

### Option B — Enforce the contract at a single boundary function

Define a single function — the process boundary — that constructs
the process's streams, injects them into every writer below it, and
is the only place in the codebase that reads `os.Stdout`, `os.Stderr`,
or `os.Stdin`. Below the boundary, every writer receives its
destination through an injected value (`deps.Streams.Out`,
`deps.Streams.Err`, the logger's `Sink`, the error path's writer).
The boundary is `defaultOptions` (which reads the process streams)
and `runWithOptions` (which routes the command tree's output and the
error path's output through the injected streams).

**Pros:**

- The contract is enforced for every code path that crosses the
  boundary. A new command that writes through `deps.Streams.Out`
  writes to the correct stream by construction; a command that
  writes to `os.Stdout` directly is the only way to violate the
  contract, and the violation is a one-line change that a lint
  rule (WBS 12.3.2) can detect.
- The tests are hermetic: a command's streams are the injected
  `bytes.Buffer` values, and the test's assertions on the buffer's
  contents are meaningful.
- The `--format json` rule can be enforced: the boundary knows
  whether `--format json` is active and can guarantee that the
  only writer to stdout is the JSON envelope's writer (via
  `internal/output`'s `Writer`).
- The redaction rules can be applied uniformly: the logger's
  `Sink` is the only destination for diagnostics, and the
  `Sink`'s implementation applies the middleware.
- The `--verbose` and `--quiet` flags can filter output uniformly:
  the logger's level is set at the boundary, and every record
  below it is subject to the filter.
- The contract is testable at the boundary: the test suite
  constructs an `options` value with `bytes.Buffer` streams and
  asserts on the buffers; a contract test in
  `internal/cli/contract_test.go` can assert that no domain
  package writes to `os.Stdout` or `os.Stderr` by construction.

**Cons:**

- The boundary is a single function (`defaultOptions`) that must
  be the only reader of the process's streams. A future change
  that adds a second reader (for example, a package that reads
  `os.Getenv` directly to configure itself) would violate the
  boundary; the violation is a one-line change that a lint rule
  can detect, but the rule must exist.
- The boundary's responsibility is larger than a single call
  site's: it must construct the streams, inject them, and route
  every writer's destination. The function is small but is
  load-bearing; a mistake in it affects every invocation.
- The pattern "the boundary is the only reader of process state"
  requires discipline: a new package that wants a process value
  (for example, `os.Getenv` to read a variable) must either
  receive the value through injection or be added to the
  boundary's responsibility. The discipline is documented in
  `docs/architecture/output-vs-logging.md` and enforced by a
  lint rule.

### Option C — Enforce the contract at a single boundary function, with the boundary in `main`

Same as Option B, but the boundary is `cmd/forge/main.go` instead
of `internal/cli`. `main` reads the process streams, constructs an
`options` value, and passes it to `cli.Run`. `internal/cli` never
reads `os.Stdout`, `os.Stderr`, or `os.Stdin`.

**Pros:**

- The boundary is the program's entry point, which is the most
  natural place for process-state reads.
- `internal/cli` becomes a pure library: it takes an `options`
  value and does not read process state. A future embedding of
  Forge as a library (per ADR-020) is trivially supported.
- The tests never touch process state; every test constructs an
  `options` value with its own buffers.

**Cons:**

- `defaultOptions` exists precisely to construct the `options`
  value from the process. Moving the construction to `main`
  would require `main` to duplicate `defaultOptions`'s logic or
  to call it from a new package, which is a larger change than
  this ADR needs to make.
- The panic path in `Execute` (per ADR-020) writes to
  `os.Stderr` directly because a panic means the injected writers
  are not trustworthy. If the boundary were in `main`, the panic
  path's direct write would be outside `internal/cli`, which
  complicates the "boundary is the only reader" rule: `main`
  would be both the reader (for `defaultOptions`) and a writer
  (for the panic path). The current shape is cleaner.
- The `defaultOptions` function's current placement is a
  consequence of the package's API: `Run(ctx)` is the public
  entry point, and it calls `runWithOptions(ctx, defaultOptions())`.
  Moving the boundary to `main` would require changing the API
  or introducing a second constructor. The change is possible
  but is not this ADR's subject.

---

## 3. Decision

**Forge adopts Option B: the stream contract is enforced at the
process boundary, and the boundary is `defaultOptions` in
`internal/cli/execute.go`.** `defaultOptions` is the only function
in the codebase that reads `os.Stdout`, `os.Stderr`, `os.Stdin`,
`os.Args`, `os.Getenv`, or `os.Getwd`. Every function below the
boundary receives its destination through injection: commands write
product output through `deps.Streams.Out`, diagnostics go through
`internal/logger`'s `Sink`, and errors are rendered by
`internal/forgeerr` to `deps.Streams.Err`. The boundary is
documented in `internal/cli/execute.go`'s package docstring and in
`docs/architecture/output-vs-logging.md`, and it is enforced by the
Taskfile target `verify:two-boundary` (WBS 4.2.2) and by the
structural tests in `internal/cli/execute_test.go`. The single
documented exception is `Execute`'s panic recovery, which writes to
`os.Stderr` directly because a panic means the injected writers are
not trustworthy (per ADR-020).

The reasoning that tipped the balance was the tests' hermeticity. The
`runCLI` helper (WBS 4.3.2) constructs an `options` value with
`bytes.Buffer` streams and asserts on the buffers' contents. A
command that wrote to `os.Stdout` directly would produce test output
that the buffer does not capture, so the test would pass while the
command's output went to the wrong place. The only way to make the
tests meaningful is for the boundary to be the sole reader of process
state and for everything below it to write through injection. Option
A (per-call-site enforcement) makes the tests meaningless in this
sense: any call site can bypass the injection, and the test would not
know. Option C (boundary in `main`) is architecturally cleaner in
some respects but is a larger change than this ADR needs to make,
and it complicates the panic path's exception. Option B's choice of
`defaultOptions` as the boundary is the smallest change that
satisfies the requirement and is consistent with the current code's
shape.

The reasoning that tipped the balance against a per-package boundary
(each package reading its own process state) was the `--format json`
rule. The rule requires that stdout carry exactly one JSON document.
If each package read `os.Stdout` directly, no single place could
guarantee the rule; the rule would be a property that happened to
hold because the current packages did not violate it, rather than a
property enforced by the architecture. The boundary is what makes
the rule enforceable: the boundary knows whether `--format json` is
active and can guarantee that `internal/output`'s writer is the only
writer to stdout.

The tradeoffs accepted are the ones Option B lists as cons. The
boundary's load-bearing nature is accepted as a consequence of its
role; the function is small and well-documented, and its tests pin
its behavior. The discipline "the boundary is the only reader of
process state" requires a lint rule to enforce; the rule is planned
for WBS 12.3.2 and is a prerequisite for this decision's full
enforcement. Until the rule lands, the discipline is enforced by
code review and by the structural tests in
`internal/cli/execute_test.go`, which already assert that
`runWithOptions` does not reference `os.Stdout`, `os.Stderr`,
`os.Stdin`, `os.Args`, or `os.Getenv` directly.

The decision is provisional in one respect: the boundary's exact
placement (in `defaultOptions` rather than in `main`) is a
consequence of the current API's shape. A future ADR that embeds
Forge as a library (per ADR-020's note) might move the boundary to
`main` or to a new package. Until then, `defaultOptions` is the
boundary, and the structural tests pin its position.

---

## 4. Consequences

### 4.1 Positive

- The stream contract is enforced for every code path that crosses
  the boundary. A new command that writes through
  `deps.Streams.Out` writes to the correct stream by construction;
  a new diagnostic that goes through `internal/logger` writes to
  stderr by construction; a new error that goes through
  `internal/forgeerr` is rendered to stderr by construction.
- The tests are hermetic: a command's streams are the injected
  `bytes.Buffer` values, and the test's assertions on the buffers'
  contents are meaningful. A test that asserts "stdout is empty on
  failure" catches a real defect; a test that asserts "stderr
  contains the error code" catches a real defect.
- The `--format json` rule can be enforced: the boundary knows
  whether `--format json` is active and can guarantee that the
  only writer to stdout is `internal/output`'s writer. The rule
  is a property of the architecture, not a convention.
- The redaction rules (ADR-033, ADR-034) can be applied uniformly:
  the logger's `Sink` is the only destination for diagnostics, and
  the `Sink`'s implementation applies the middleware. A direct
  write would bypass the middleware; the boundary's enforcement
  prevents direct writes.
- The `--verbose` and `--quiet` flags (ADR-030, ADR-031) can
  filter output uniformly: the logger's level is set at the
  boundary, and every record below it is subject to the filter.
  A direct write would bypass the filter; the boundary prevents
  direct writes.
- The contract is testable at the boundary: the test suite
  constructs an `options` value with `bytes.Buffer` streams and
  asserts on the buffers. The structural tests in
  `internal/cli/execute_test.go` assert that `runWithOptions`
  does not reference process state directly.
- The discipline "the boundary is the only reader of process
  state" is documented and (once WBS 12.3.2's lint rule lands)
  enforced. A new contributor who wants to read `os.Getenv`
  learns that the value must flow through the boundary.

### 4.2 Negative

- The boundary is a single function (`defaultOptions`) whose
  correctness affects every invocation. A mistake in it — a typo
  in a stream name, a missing injection — is a defect that
  affects the whole CLI. The function's small size and its
  tests mitigate the risk, but the risk is real.
- The discipline "the boundary is the only reader of process
  state" requires a lint rule to enforce. Until WBS 12.3.2's
  rule lands, the discipline is enforced by code review and by
  the structural tests; a determined contributor could add a
  second reader without the tests catching it (the tests check
  `runWithOptions`, not every function).
- The boundary's placement in `defaultOptions` is a consequence
  of the current API's shape. A future refactor (for example,
  embedding Forge as a library) would need to move the boundary,
  and the move would require updating the structural tests and
  the lint rule's allow-list.
- The panic path's exception (`Execute`'s direct write to
  `os.Stderr`) is a special case that must be documented and
  recognized by the lint rule. The exception's existence is a
  small complication; the alternative — routing the panic path
  through the injected writers — is rejected because a panic
  means the injected writers are not trustworthy.
- The injection model requires every writer to accept a
  destination parameter. A package that wants to write must
  accept a `Writer` or a `Sink`; the dependency is small but is
  not zero. A package that wants to write to "the right stream"
  without accepting a parameter has no way to do so.

### 4.3 Neutral

- The `streams.Streams` value (WBS 12.4.1) is the injection
  seam for the three streams; it is a value, not an interface,
  and its fields are `io.Reader` and `io.Writer` values. The
  choice of `io.Reader`/`io.Writer` rather than concrete types
  is what makes the injection testable with `bytes.Buffer`.
- The `defaultOptions` function's return type is `options`, not
  an interface. The `options` struct is the process boundary's
  representation; its fields are the injected values. A future
  change to the struct's fields is a change to the boundary's
  API, and the change must be reflected in the tests.
- The boundary's responsibility includes the command-line
  arguments (`os.Args[1:]`), the three streams
  (`os.Stdin`/`os.Stdout`/`os.Stderr`), the environment
  (`os.Getenv`), and the working directory (`os.Getwd`). The
  five process-state reads are the boundary's full surface; a
  future addition (for example, a `--pid` flag that reads the
  process ID) would extend the surface.
- The `verify:two-boundary` Taskfile target (WBS 4.2.2) checks
  that `buildDependencies` is called exactly once and that
  `defaultOptions` is the only reader of process state. The
  target is the pre-lint-rule enforcement mechanism; the lint
  rule (WBS 12.3.2) will complement it.

---

## 5. Related Requirements

- `FR-CLI-OUTPUT-001` — product output is written to stdout and
  only stdout; the requirement is enforced by this decision's
  boundary.
- `FR-CLI-LOG-001` — diagnostics are written to stderr and only
  stderr; the requirement is enforced by this decision's
  boundary.
- `FR-CLI-OUTPUT-011` — the stream contract holds under
  `--format json`; the requirement is enforced by this
  decision's boundary.
- `FR-CLI-LOG-007` — domain packages emit diagnostics only
  through the `Logger` interface; the requirement is enforced by
  this decision's boundary (in addition to ADR-027's rule).
- `NFR-ARCHITECTURE-003` — the logger's interface is independent
  of CLI and domain packages; the requirement is compatible with
  this decision's dependency direction.
- `NFR-ARCHITECTURE-004` — no direct writes in domain packages;
  the requirement is enforced by this decision's boundary (in
  addition to ADR-027's rule).

> The following requirements must be added to the register when
> WBS 12.4.1 is complete:

- `FR-CLI-OUTPUT-011` (if not already present)
- `NFR-ARCHITECTURE-005` (if a separate requirement for the boundary's uniqueness is needed)

---

## 6. Related ADRs

- `ADR-020` — Single Exit Boundary in main. Establishes the
  process's exit boundary; this decision's stream boundary is
  the entry-side counterpart.
- `ADR-021` — JSON Error Envelope for `--format json`.
  Establishes the JSON error envelope that is the only thing on
  stdout under `--format json`; this decision's boundary enforces
  the exclusivity.
- `ADR-022` — Output and logging are separate architectural
  layers. Establishes the two layers this decision routes
  through the boundary.
- `ADR-026` — The Logger interface has four level methods and a
  With combinator. Establishes the interface that the boundary's
  `logger.SetDefault` call configures.
- `ADR-027` — Domain packages emit diagnostics only through the
  Logger interface. Establishes the prohibition that this
  decision's boundary enforces.
- `ADR-029` (expected) — Broken pipes exit cleanly with code 141
  and never print a stack trace. Will define the boundary's
  behavior when a downstream consumer closes the stream; depends
  on this decision's boundary.
- `ADR-033` (expected) — Sensitive data is redacted at three
  layers. Will define the redaction rules that this decision's
  boundary makes uniformly applicable.
- `ADR-034` (expected) — The redaction policy is closed. Will
  define the closure property that this decision's boundary
  supports.

---

## 7. Notes

**Open questions deferred by this ADR:**

- Whether the boundary should be in `defaultOptions` or in `main`
  (Option C). The ADR chooses `defaultOptions` because it is the
  smallest change from the current code and because it is
  consistent with the panic path's placement in `Execute`. A
  future ADR that embeds Forge as a library could revisit the
  choice.
- Whether the structural tests should check every function in
  `internal/cli` (not just `runWithOptions`) for process-state
  reads. The current tests check `runWithOptions`; a future
  extension could check the whole package, or a lint rule
  (WBS 12.3.2) could perform the check.
- Whether the `verify:two-boundary` Taskfile target should be
  extended to check the panic path's exception explicitly, or
  whether the exception is documented and left to code review.
  The current target checks the boundary's uniqueness; the
  exception is documented in `Execute`'s docstring.

**Follow-up decisions anticipated:**

- The lint rule (WBS 12.3.2) is a direct consequence of this
  ADR; the rule's scope includes both ADR-027's prohibition
  (domain packages) and this ADR's boundary uniqueness (only
  `defaultOptions` reads process state).
- The broken-pipe handling (ADR-029) is the boundary's behavior
  when a downstream consumer closes the stream; its design
  builds on this decision's boundary.
- The redaction rules (ADR-033, ADR-034) apply to the logger's
  `Sink`, which the boundary configures; their design is
  separate but depends on the boundary's uniformity.

**External references:**

- POSIX.1-2017 §2.5, "Standard I/O Streams" — the normative
  source for the stdout/stderr split.
- The Twelve-Factor App, §XI, "Logs" — the guidance that informs
  the logger's destination.
- Go standard library `io` — the `Reader`/`Writer` interfaces
  that the `streams.Streams` value uses as its field types.
- `docs/architecture/output-vs-logging.md` — the authoritative
  reference for the stream contract this decision enforces.
- `docs/reference/output-streams.md` — the per-exit-code stream
  contents.

---

*End of ADR-028.*
