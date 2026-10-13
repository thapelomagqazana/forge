# ADR-027: Domain packages emit diagnostics only through the Logger interface

**Status:** Accepted
**Date:** 2026-10-13
**Deciders:** [@thapelomagqazana] (Core Runtime Owner)
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

ADR-022 established that product output goes to stdout and diagnostics go to stderr, and that `internal/output` and `internal/logger` are the two packages that own those surfaces. ADR-026 defined the `Logger` interface that callers use to emit diagnostics. But neither decision said anything about the *code that calls* those packages. A domain package — `internal/config`, `internal/app/version`, or any future package that is not `internal/cli` — can still call `fmt.Println`, `fmt.Fprintf(os.Stderr, ...)`, `log.Print`, or `os.Stderr.Write` directly, and those calls will compile and run without violating any type-level rule. The result is a stream contract that holds for the packages that respect it and silently breaks for the packages that do not. The current codebase has no such violations; the risk is that a future contributor, unaware of the two-layer model, adds one.

The forces at play are three. First, the stream contract is a *value-level* property, not a *type-level* one. Nothing in the Go type system prevents a domain package from importing `fmt` and writing to stdout; the contract is enforced by convention, code review, and (once it exists) a lint rule. Second, the codebase is small enough today that a review miss is not catastrophic — the number of packages is manageable and the number of contributors is small — but the codebase will grow, and the review's effectiveness scales with the number of reviewers, not the number of lines. Third, the `Logger` interface is a *dependency*: a package that uses it must accept a `Logger` parameter or read one from a context. This acceptance is a small but real cost; a package that only needs to print one line for a diagnostic has to decide whether the line is worth the dependency, and the wrong decision is to reach for `fmt.Println`.

The current pattern in the codebase is mixed by necessity. `internal/cli` legitimately writes to stdout via `deps.Streams.Out` (the injection seam) and to stderr via the logger; its writes are the CLI's product output and diagnostics, both of which it owns. `internal/config` and `internal/app/version` legitimately have no direct writes at all — they return values and errors, and the CLI's handlers render them. The pattern is that domain packages *compute*, and the CLI *renders*. A diagnostic that a domain package wants to emit is not a domain concern; it is an operational event that the domain package observes, and the observation belongs in the logger.

The decision relates to ADR-022 (the stream contract), ADR-026 (the `Logger` interface), ADR-025 (the record shape), and ADR-033 and ADR-034 (redaction, which will apply to every record emitted through the interface). The CLI UX spec's § 4.12 mentions the CLI's stream separation but does not name the packages. If no decision is made, the stream contract remains a convention that any code change can silently break, and the future WBS item that adds a lint rule (WBS 12.3.2) has no ADR to cite for the rule's scope.

---

## 2. Options Considered

### Option A — Allow domain packages to write directly, with code review as the enforcement

Domain packages may call `fmt.Println`, `fmt.Fprintf(os.Stderr, ...)`, `log.Print`, or any other direct write. The stream contract is enforced by code review; a reviewer who sees a direct write from a domain package flags it. No lint rule; no interface requirement.

**Pros:**

- No dependency on `internal/logger` for a package that wants to emit one diagnostic. A two-line change can add a diagnostic without threading a logger through the call chain.
- No lint rule to write, test, or maintain.
- The codebase's existing style (domain packages compute, CLI renders) is unchanged; a contributor who follows the existing pattern will not write directly.

**Cons:**

- The contract is unenforceable. A direct write that a reviewer misses is a defect that ships, and the defect class is silent: the code compiles, the tests pass, and the user sees a stray line on the wrong stream.
- The contract's violations are not detectable by the test suite. A test that asserts "stdout is empty on failure" catches one specific violation; it does not catch a diagnostic written to stdout by a command that does not have that test.
- A future contributor who is unfamiliar with the two-layer model has no mechanism to learn it from the code. The pattern is documented in `docs/architecture/output-vs-logging.md`, but a contributor writing a new package may not have read the doc.
- The stream contract's promises (the `--format json` rule, the `--quiet` rule) are only as strong as the weakest package. A single violation in a domain package breaks the promise for every command that package participates in.

### Option B — Require every domain package to accept a `Logger` and forbid all other diagnostics

Every domain package that emits a diagnostic accepts a `Logger` as a constructor parameter or reads one from a context. Direct writes (`fmt.Print*`, `os.Stdout.Write`, `os.Stderr.Write`, `log.Print*`) are forbidden by a lint rule (WBS 12.3.2). A package that has no diagnostic to emit accepts no `Logger`; a package that has one accepts a `Logger` and uses it.

**Pros:**

- The stream contract is enforced by the lint rule, which is testable (it fails on an injected violation) and stable (it is a property of the source, not of a code review).
- Every diagnostic a domain package emits goes through the logger, which means every diagnostic is subject to the level filter (ADR-023), the precedence chain (ADR-024), the record shape (ADR-025), and the redaction rules (ADR-033, ADR-034).
- The pattern "domain packages compute, CLI renders" is reinforced: a domain package that wants a diagnostic accepts a `Logger`, which is a small but visible cost that discourages gratuitous diagnostics.
- A new contributor who reads the lint rule's error message learns the contract. The rule's error message names the doc and the interface; the contributor cannot miss it.
- The stream contract's promises are uniform across the codebase: no package can silently break them.
- The dependency direction (`internal/cli → internal/logger` and `internal/config → internal/logger`, etc.) is one-way; a domain package that imported `internal/cli` would be a layering violation that the package's structure test (or a future lint rule) could catch.

**Cons:**

- Every domain package that emits a diagnostic must accept a `Logger`. This is a small dependency that a package might not want; a package that emits one line at startup has to thread a `Logger` through its constructor.
- The lint rule must be written, tested, and maintained. It is a small amount of code (a `go/analysis` analyzer or a Taskfile-driven grep), but it is code.
- The rule's scope must be defined precisely: `internal/cli` legitimately writes to `deps.Streams.Out` and to the logger, and `internal/output` and `internal/logger` legitimately write to their destinations. The rule's allow-list must name these packages and must not grow by accident.
- A package that legitimately needs to write (for example, a future `internal/progress` package that renders a progress bar to stderr) must either use the logger or be added to the allow-list with an ADR. The allow-list's growth is a design decision that must be governed.

### Option C — Require every domain package to accept a `Logger`, but allow direct writes to `os.Stderr` for "operational" diagnostics

Same as Option B, but a domain package may write directly to `os.Stderr` for a diagnostic that is not part of the logger's level-filtered surface — for example, a panic-trace write or an "I am about to exit" line. The logger is for the level-filtered diagnostics; direct `os.Stderr` writes are for the escape hatches.

**Pros:**

- A package that needs an unfiltered write (a panic trace, a process-exit message) has a way to do it without going through the logger.
- The logger's level filter remains the mechanism for filtered diagnostics; the escape hatch is reserved for the cases that cannot be filtered.

**Cons:**

- "Escape hatch" is a category that grows. A contributor who wants their diagnostic to bypass the level filter — because the filter is inconvenient — will argue that their case is "operational" and use the escape hatch. The hatch's boundaries are subjective and unenforceable.
- Direct writes to `os.Stderr` bypass the record shape (ADR-025), the redaction rules (ADR-033, ADR-034), and the format selection (`FORGE_LOG_FORMAT`). A user who requested JSON logs will see a non-JSON line from the escape hatch, which breaks the JSON log's parseability.
- The `internal/cli` package already has the legitimate direct write it needs: `Execute`'s panic recovery (per ADR-020 and the package docstring). That is one exception, and it is documented. Adding a general-purpose hatch for domain packages expands the exception from one documented case to an unspecified many.
- The escape hatch's existence weakens the lint rule: the rule cannot distinguish an "operational" write from any other write, so it must either allow all `os.Stderr` writes (defeating the rule) or require an annotation (which is additional surface and an additional source of friction).

### Option D — Require every domain package to accept a `Logger`; forbid all direct writes; allow the documented panic path only

Same as Option B, plus an explicit, documented, single exception: `internal/cli`'s `Execute` function's panic recovery writes to `os.Stderr` directly (per ADR-020), because a panic means the injected writers are not trustworthy. No other exception. The lint rule's allow-list is the two surface packages (`internal/output`, `internal/logger`) plus the single `Execute` function, and the lint rule's message names the exception's ADR.

**Pros:**

- The stream contract is enforced uniformly. The only exception is a single function whose behavior is documented in the ADR that established it (ADR-020) and in the package's docstring.
- The lint rule's allow-list is small and stable: two packages and one function. A future contributor who wants to add a third package to the list must write an ADR, which is the correct level of friction.
- The panic path's exception is *justified*: a panic is the failure of the process environment, and the injected writers are not trustworthy in that state. The exception is not a convenience; it is a necessary response to a category of failure the injection model cannot handle.
- Every other diagnostic in the codebase goes through the logger, which means every diagnostic is subject to the level filter, the record shape, and the redaction rules.
- A new contributor who reads the lint rule's error message learns both the rule and the exception. The message names the ADR that authorizes the exception, so the contributor can read the reasoning if they disagree with the rule.

**Cons:**

- The exception is a special case that must be documented, tested, and maintained. The lint rule must recognize the exception by function name or by a comment marker; the recognition is a small amount of code.
- The exception's existence invites a future contributor to argue that their case is "like the panic path" and should also be excepted. The answer is that the panic path is exceptional by definition (it runs when the process is already failing), and a case that runs in normal operation is not the same. The distinction is documented in the ADR; a contributor who disagrees must write an ADR.
- The rule's enforcement depends on the lint rule existing. Until WBS 12.3.2 lands, the rule is enforced by convention and code review alone, which is Option A's weakness. The ADR is the commitment; the lint rule is the mechanism, and its absence is a temporary state.

---

## 3. Decision

**Forge adopts Option D: every domain package emits diagnostics only
through the `Logger` interface; direct writes (`fmt.Print*`,
`fmt.Fprint*` to `os.Stdout` or `os.Stderr`, `os.Stdout.Write`,
`os.Stderr.Write`, `log.Print*`) are forbidden in every package
except `internal/output`, `internal/logger`, and the single
documented panic-recovery function in `internal/cli/execute.go`.** The
rule is enforced by a lint rule (WBS 12.3.2) whose allow-list is the
two surface packages plus the panic path. The rule's error message
names this ADR and `docs/architecture/output-vs-logging.md` for the
rationale, so a contributor who encounters a violation learns the
rule and can read the reasoning behind it.

A domain package that emits a diagnostic accepts a `Logger` — as a
constructor parameter, as a field on an injected struct, or by
reading one from a context — and calls the logger's methods. A
package that does not emit a diagnostic accepts no `Logger`; the
absence is the correct state for a package whose work is computation
rather than operation. The choice is per package, and the lint rule
enforces that the choice is not "both".

The reasoning that tipped the balance was the uniformity of the
stream contract's promises. ADR-022 promises that under
`--format json`, stdout carries exactly one JSON document; ADR-025
promises that the JSON log rendering is stable within a major
version; ADR-033 (expected) will promise that secrets are redacted
from every diagnostic. Each of these promises is only as strong as
the weakest package. A single direct write in `internal/config`
could produce a non-JSON line on stdout under `--format json`, or a
log line that bypasses the redaction middleware, or a record with
the wrong shape. The uniformity is not a stylistic preference; it is
what makes the promises true. Option A (review-only enforcement) is
insufficient because a review miss is silent and a lint rule is
not; Option C (an "operational" escape hatch) is insufficient
because the hatch's boundaries are subjective and would be
misused; Option B is close to Option D but does not name the panic
path's exception, which would leave the exception undocumented and
therefore open to expansion. Option D names the exception and only
the exception.

The reasoning against giving domain packages a way to write
directly (Option C's escape hatch) is that a diagnostic that bypasses
the logger is a diagnostic that bypasses every promise the logger
makes. A redaction rule that covers the logger's records does not
cover a direct write; a format selection that applies to the
logger's output does not apply to a direct write; a level filter
that applies to the logger's records does not filter a direct
write. The escape hatch's existence is the promise's absence. The
`internal/cli` package's panic recovery is the only case where the
tradeoff favors the direct write, and it is the only case because a
panic means the injected writers cannot be used. Every other case
has an alternative: the logger.

The tradeoffs accepted are the ones Option D lists as cons. The
exception's documentation and the lint rule's recognition of it are
small costs; the exception's existence is a policy that a future
contributor must respect, and the ADR is the policy's authoritative
statement. The rule's enforcement depends on WBS 12.3.2's lint rule,
which does not yet exist; until it lands, the rule is enforced by
convention and code review. The ADR is the commitment; the lint rule
is the mechanism, and the interim state is a known limitation rather
than an oversight.

The decision is provisional in one respect: the allow-list's
membership is fixed today, but a future ADR may add a package to the
list if a real need arises. The bar for adding is high: the package
must be a surface (a producer of output that no other package can
produce), and the package's role must not be satisfiable by the
logger. A package that merely wants a shortcut does not meet the
bar. The current list — `internal/output`, `internal/logger`, and
the panic path — is the correct starting set, and each member's role
is documented in the package's own docstring and in
`docs/architecture/output-vs-logging.md`.

---

## 4. Consequences

### 4.1 Positive

- The stream contract's promises are uniform across the codebase.
  Under `--format json`, stdout carries exactly one JSON document
  because no package other than `internal/output` and the panic
  path writes to stdout. Under `--verbose`, the logger's records
  are the only diagnostics on stderr because no package writes to
  stderr directly.
- Every diagnostic is subject to the level filter (ADR-023), the
  precedence chain (ADR-024), the record shape (ADR-025), and the
  redaction rules (ADR-033, ADR-034). A future redaction
  middleware wraps the `Sink` interface and therefore covers every
  diagnostic; a direct write would not be covered.
- The pattern "domain packages compute, CLI renders" is
  reinforced. A domain package that wants a diagnostic accepts a
  `Logger`, which is a small, visible cost that discourages
  gratuitous diagnostics. A package that emits no diagnostics
  accepts no `Logger`, which is the correct state for a package
  whose work is pure computation.
- A new contributor who reads the lint rule's error message learns
  both the rule and the exception. The message names this ADR and
  `docs/architecture/output-vs-logging.md`, so the contributor
  can read the reasoning if they disagree with the rule.
- The exception is *one function*, documented in ADR-020 and in
  `internal/cli/execute.go`'s docstring. A future contributor who
  wants to add a second exception must write an ADR, which is the
  correct level of friction for a change to the stream contract.
- The dependency direction is one-way: `internal/cli →
  internal/logger`, `internal/config → internal/logger`, etc. A
  domain package that imported `internal/cli` (which would be a
  layering violation) is detectable by a future structural test
  or lint rule.

### 4.2 Negative

- Every domain package that emits a diagnostic must accept a
  `Logger`. A package that emits one line at startup has to
  thread a `Logger` through its constructor. The cost is small
  but is not zero; a package that wanted to avoid the dependency
  cannot.
- The lint rule must be written, tested, and maintained. It is a
  small amount of code (a `go/analysis` analyzer, a `grep`-based
  script, or a Taskfile target), but it is code that must not
  have false positives (which would frustrate contributors) or
  false negatives (which would defeat the rule's purpose).
- The rule's scope must be defined precisely: `internal/cli`
  legitimately writes to `deps.Streams.Out` (via
  `internal/output`), `internal/output` and `internal/logger`
  legitimately write to their destinations, and `Execute`'s
  panic path writes to `os.Stderr` directly. The allow-list is
  small but its boundaries are load-bearing; a future contributor
  who wants to add a package to the list must write an ADR.
- The exception's existence invites a future contributor to argue
  that their case is "like the panic path". The answer is that
  the panic path is exceptional by definition (it runs when the
  process is already failing), and a case that runs in normal
  operation is not the same. The distinction is documented in
  the ADR; a contributor who disagrees must write an ADR.
- The rule's enforcement depends on WBS 12.3.2's lint rule, which
  does not yet exist. Until it lands, the rule is enforced by
  convention and code review, which is the weakness the ADR
  exists to eliminate.

### 4.3 Neutral

- The `internal/output` package is exempted because it produces
  product output; the `internal/logger` package is exempted
  because it produces diagnostics. The two exemptions are the
  stream contract's two producers; the exemption is not a
  loophole but the contract's definition.
- The panic path's exemption is in `internal/cli/execute.go`'s
  `Execute` function. A future refactor that moved the panic
  path to a different function would need to update the lint
  rule's allow-list; the update is a one-line change, but it is
  a change that must be made.
- The lint rule's mechanism (a `go/analysis` analyzer vs. a
  Taskfile-driven `grep`) is not decided by this ADR. Either
  mechanism enforces the rule; the choice is WBS 12.3.2's.
- The rule's allow-list's exact syntax (package paths, function
  names, or a marker comment) is not decided by this ADR. The
  list's *membership* is decided; the syntax is a matter for
  WBS 12.3.2.

---

## 5. Related Requirements

- `FR-CLI-LOG-007` — domain packages emit diagnostics only
  through the `Logger` interface; the requirement is established
  by this decision.
- `FR-CLI-LOG-008` — the stream contract is enforced at the
  boundary; the requirement is strengthened by this decision's
  uniformity.
- `FR-CLI-OUTPUT-001` — product output is written to stdout and
  only stdout; the requirement's enforcement is strengthened by
  this decision.
- `FR-CLI-LOG-001` — diagnostics are written to stderr and only
  stderr; the requirement's enforcement is strengthened by this
  decision.
- `NFR-ARCHITECTURE-004` — no direct writes in domain packages;
  the requirement is established by this decision.
- `NFR-ARCHITECTURE-003` — the logger's interface is independent
  of CLI and domain packages; the requirement is compatible with
  this decision's dependency direction.

> The following requirements must be added to the register when
> WBS 12.3.2 is complete:

- `FR-CLI-LOG-007` (if not already present)
- `NFR-ARCHITECTURE-004` (if not already present)

---

## 6. Related ADRs

- `ADR-020` — Single Exit Boundary in main. Establishes the panic
  path's direct write to `os.Stderr` as the CLI's only
  exception to the injection model; this decision's allow-list
  names the exception.
- `ADR-022` — Output and logging are separate architectural
  layers. Establishes the two surface packages this decision's
  allow-list names.
- `ADR-023` — Four log levels: ERROR, WARN, INFO, DEBUG.
  Establishes the level filter that applies to every diagnostic
  this decision routes through the logger.
- `ADR-024` — Log level precedence: CLI flag > env var > config
  file > default. Establishes the resolution that determines
  which records the logger emits.
- `ADR-025` — Log records are structured with a schema-versioned
  JSON form and an unstable human form. Establishes the record
  shape that this decision's routing produces.
- `ADR-026` — The Logger interface has four level methods and a
  With combinator. Establishes the interface this decision
  requires domain packages to use.
- `ADR-033` (expected) — Sensitive data is redacted at three
  layers. Will define the redaction rules that this decision's
  routing makes uniformly applicable.
- `ADR-034` (expected) — The redaction policy is closed. Will
  define the closure property that this decision's uniformity
  supports.

---

## 7. Notes

**Open questions deferred by this ADR:**

- Whether the lint rule should be a `go/analysis` analyzer (which
  integrates with `go vet` and IDEs) or a Taskfile-driven `grep`
  (which is simpler to write and easier to run in CI). The ADR
  does not decide; the choice is WBS 12.3.2's.
- Whether the allow-list should be expressed as an inline
  annotation (a `//nolint:directprint` comment on an allowed
  write) or as a package path pattern. The annotation is more
  precise but requires a comment on every allowed write; the
  path pattern is coarser but requires no per-write change. The
  ADR does not decide; the choice is WBS 12.3.2's.
- Whether a future package might legitimately need to write
  directly. The current answer is "no, unless an ADR says so";
  a future ADR could add a package if a real need arises. The
  bar for adding is high: the package must be a surface, not a
  shortcut.

**Follow-up decisions anticipated:**

- The lint rule's implementation (WBS 12.3.2) is a direct
  consequence of this ADR; the rule's error message will cite
  this ADR and `docs/architecture/output-vs-logging.md`.
- The audit table (WBS 12.3.2's `docs/architecture/output-audit.md`)
  will classify every existing direct write in the codebase and
  note whether it was migrated or is covered by the allow-list.
  The audit's output is a supporting document for this ADR, not
  a separate decision.
- The redaction rules (ADR-033, ADR-034) apply to every diagnostic
  this decision routes through the logger; their design is
  separate from this ADR but depends on the uniformity this
  decision establishes.

**External references:**

- POSIX.1-2017 §2.5, "Standard I/O Streams" — the normative
  source for the stdout/stderr split.
- The Twelve-Factor App, §XI, "Logs" — the guidance that informs
  the logger's destination.
- `go/analysis` — the framework for a lint rule that integrates
  with `go vet`.
- `docs/architecture/output-vs-logging.md` — the authoritative
  reference for the stream contract this decision enforces.
- `docs/architecture/output-audit.md` (expected) — the audit
  table WBS 12.3.2 will produce.

---

*End of ADR-027.*
