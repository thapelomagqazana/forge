# ADR-022: Output and logging are separate architectural layers

**Status:** Accepted
**Date:** 2026-10-13
**Deciders:** [@thapelomagqazana] (Architecture Owner)
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge writes to two standard streams, and until now it has not had a
written rule about which of its outputs belongs on which stream. The
CLI has a `forge version` command that prints a report to stdout, a
`forge config` command that prints a resolved configuration, error
messages that appear on stderr, and — as the CLI grows — a small but
increasing number of diagnostic lines that report what the CLI did
(config file discovery, version resolution, format selection). A
single global `--format json` flag has existed since WBS 9.4.1 and is
already promised by the CLI UX spec to produce "exactly one JSON
document on stdout" for the commands that use it. There is no
corresponding contract for stderr, no package boundary between the
code that produces product results and the code that produces
diagnostics, and no rule that prevents the two from mixing.

This matters because the CLI is embedded in shell pipelines, in CI
runners, and in scripts that parse its output. A single stray
`fmt.Println("loading...")` in a code path that runs under
`--format json` produces invalid JSON and breaks every downstream
parser silently — a defect class that is difficult to catch in tests
and expensive to debug in the field. The same defect class exists for
stderr: a diagnostic line that appears on stdout corrupts the product
output; a product line that appears on stderr is invisible to a
consumer that reads stdout. The streams are the CLI's public
interface, and the interface has not been specified.

The forces at play are three. First, the CLI is used by humans in a
terminal, by scripts, and by CI systems with different expectations
of the two streams; the contract must satisfy all three. Second, the
`--format json` flag's promise ("exactly one JSON document on
stdout") is a public contract that other WBS items depend on; any
rule this decision establishes must be compatible with it. Third,
the diagnostic surface is growing: `--verbose` and `--quiet` are
planned, and their behaviour depends on the stream contract. Without
a decision now, each command would invent its own rules, and the
inconsistencies would accumulate until the CLI could no longer
promise anything to its consumers.

The decision relates to WBS 12.1.2 (log level semantics), WBS 12.3.1
(the logger interface), WBS 12.4.1 (stream contract enforcement), and
WBS 12.5.1 (`--verbose`). It also interacts with the existing CLI UX
specification (`docs/cli-ux-spec.md` § 4), which describes the
`--format` flag's behaviour but does not name the stream a
diagnostic goes to. A second decision — the log level vocabulary and
its semantics — is expected to be taken separately as ADR-023 and
will rely on this one. If no decision is made, the CLI continues to
lack a public contract for its streams, the `--format json` promise
remains untested at the stream boundary, and future WBS items that
emit diagnostics would each need to re-derive the rule.

---

## 2. Options Considered

### Option A — One stream, `--quiet` and `--verbose` filter it

All output — product results and diagnostics — goes to stdout.
`--quiet` suppresses diagnostic lines; `--verbose` enables more of
them; `--format json` produces JSON on stdout that includes both the
result and (optionally) diagnostic fields. Consumers that want only
the product result pass `--quiet`. Consumers that want diagnostics
pass `--verbose`. The stream is uniform; the flags determine what
appears on it.

**Pros:**

- One stream to document and to test.
- A consumer sees a single ordered sequence of events; no need to
  multiplex stdout and stderr in the consumer's code.
- `--quiet` and `--verbose` are natural filters on a single stream.
- Log aggregation systems that already consume stdout need no
  configuration change to receive diagnostics.

**Cons:**

- Breaks the POSIX convention that stdout carries the product and
  stderr carries diagnostics; consumers that follow the convention
  would mis-parse the output.
- Breaks the existing `--format json` promise: JSON consumers would
  receive a stream that contains non-JSON lines when `--verbose` is
  set, requiring the consumer to filter them — exactly the class of
  defect the CLI's JSON contract was created to prevent.
- Makes `--quiet` a semantic requirement on every command: a
  command that produces a product result must not suppress it under
  `--quiet`, which forces the command to distinguish "the result"
  from "diagnostics about the result" — the very distinction this
  option claims to avoid.
- Couples diagnostic visibility to the product stream's content,
  which prevents a user from asking for full diagnostics on a
  command whose output is piped to `jq`.
- The choice is unusual: `git`, `docker`, `kubectl`, and `go` all
  follow the POSIX convention. A CLI that does not would surprise
  every user with prior experience.

### Option B — Two streams with a fixed contract: output → stdout, logger → stderr, errors → stderr

Product output goes to stdout; diagnostics go to stderr; errors go
to stderr. The two streams are produced by two distinct packages —
`internal/output` for the product surface and `internal/logger` for
the operational surface — and the packages enforce the contract by
construction (the output writer has no stderr handle; the logger has
no stdout handle). Under `--format json`, stdout carries exactly one
JSON document (the success envelope or the error envelope); logger
records continue on stderr unchanged. Errors are rendered by a third
package, `internal/forgeerr`, which also writes to stderr.

**Pros:**

- Follows the POSIX convention and the Twelve-Factor App's §XI
  guidance; matches the behaviour of the tools Forge's users
  already use.
- Preserves the `--format json` promise: a consumer that reads
  stdout receives exactly one JSON document, regardless of
  `--verbose`.
- Allows `--verbose` and `--quiet` to tune diagnostics without
  affecting the product stream; the two axes are independent.
- Enforceable by construction: the type system prevents a logger
  record from reaching stdout and an output value from reaching
  stderr.
- Testable at the byte level: the contract is a property of the
  bytes on each stream, which the test suite can capture and assert
  on without special-casing commands.
- Scalable: a new command that emits a diagnostic has one obvious
  place to send it, and the contract holds without the command's
  author needing to re-derive it.

**Cons:**

- Two packages to write and maintain where one would have sufficed.
- Two streams to document; the documentation must explain the
  interaction rules (what goes where under `--format json`, under
  `--quiet`, under `--verbose`).
- A consumer that wants a single ordered view of the invocation
  must multiplex the two streams itself; POSIX provides no standard
  tool for this, and the consumer's code must handle the interleave.
- Error messages that a user sees in a terminal appear on stderr,
  which some users expect to be reserved for failures; the CLI's
  own success messages on stdout and its diagnostics on stderr can
  surprise users who are not accustomed to the convention.

### Option C — Two streams with the contract as an internal convention

Product output goes to stdout; diagnostics go to stderr; errors go to
stderr. The contract is a team convention, documented in the CLI UX
spec but not enforced by the type system. Each command's author is
responsible for sending each line to the correct stream; the review
process catches violations. No dedicated packages; the code uses
`fmt.Fprintln(os.Stdout, ...)` and `fmt.Fprintln(os.Stderr, ...)`
directly.

**Pros:**

- Lowest friction: no new packages, no interface to learn.
- A command's author writes the stream choice inline, which is
  visible at the call site.
- The contract is easy to change if a future need arises.

**Cons:**

- The contract is not enforceable; a violation is a code-review
  miss away from shipping.
- The `--format json` promise cannot be tested at the byte level
  because there is no single writer whose output can be inspected.
- A future refactor that moves a diagnostic from one code path to
  another may move it across the stream boundary without anyone
  noticing; the code continues to compile and the tests continue to
  pass, because the violation is a value-level property rather than
  a type-level one.
- The convention cannot be extended to `--verbose` and `--quiet`
  without a central place to read the flags; each command would
  re-implement the level check.
- Doesn't scale: as the number of commands and diagnostics grows,
  the convention accumulates silent drift.

---

## 3. Decision

**Forge adopts Option B: two streams with a fixed, type-enforced
contract.** Product output goes to stdout and is produced by
`internal/output`. Diagnostics go to stderr and are produced by
`internal/logger`. Errors go to stderr and are rendered by
`internal/forgeerr`. Under `--format json`, stdout carries exactly
one JSON document; logger records continue on stderr unchanged. The
contract is documented in `docs/architecture/output-vs-logging.md`
and enforced by the two packages' type signatures, by a contract test
in `internal/cli/contract_test.go`, and (in a subsequent change) by a
lint rule that forbids `fmt.Print*` and `os.Stdout.Write` outside the
two approved packages.

The reasoning that tipped the balance was the `--format json`
promise. Options A and C both fail to keep the promise enforceable.
Under Option A, a consumer that reads stdout can receive diagnostic
lines, and the promise is a documentation claim rather than a
guarantee. Under Option C, the promise is a convention that any code
change can silently break. Option B is the only option under which
the promise is a property of the bytes on the stream, which is what
the consumer actually observes. The POSIX convention is a secondary
factor: the decision would be defensible on the JSON promise alone,
but the convention's alignment with `git`, `docker`, and `kubectl`
means users with prior CLI experience find Forge's behaviour
predictable.

The tradeoffs accepted are the ones Option B lists as cons: two
packages instead of one, two streams to document, and the requirement
that a consumer who wants a single ordered view must multiplex the
two streams. The first two are costs of making the contract
enforceable; the third is a property of POSIX that the CLI cannot
change. All three are accepted knowingly.

The decision is provisional in one respect: the enforcement
mechanism is a lint rule that does not yet exist (WBS 12.3.2 will add
it), and until that change lands, the contract is enforced by the
type system alone — a `fmt.Fprintln(os.Stdout, ...)` in a domain
package would compile and would not be caught by the current test
suite. The lint rule is expected within the same release; if the lint
rule is not implemented within a reasonable window, the decision
should be revisited rather than left half-enforced.

---

## 4. Consequences

### 4.1 Positive

- The `--format json` promise becomes testable at the byte level.
  A consumer that reads stdout receives exactly one JSON document,
  regardless of `--verbose`, `--quiet`, or the number of
  diagnostics the invocation emits.
- `--verbose` and `--quiet` become independent of the product
  stream. A user who wants full diagnostics on a command whose
  output is piped to `jq` can have both; under Option A that
  combination would have required filtering the stream.
- The contract is enforceable by construction. The output writer
  has no stderr handle; the logger has no stdout handle; a code
  change that tried to mix them would not compile.
- New commands have one obvious place to send each kind of output.
  A command's author does not need to re-derive the rule; the rule
  is the package the code imports.
- The contract scales to future diagnostics (WBS 12.7.1's
  redaction, WBS 12.6.1's `--quiet`, WBS 12.5.1's `--verbose`)
  because those features are implemented inside the packages rather
  than across the codebase.
- The stream contract matches POSIX and the Twelve-Factor App, so
  users with prior CLI experience find Forge's behaviour
  predictable.

### 4.2 Negative

- Two packages must be written, documented, and tested, where a
  single package would have sufficed for the current feature set.
- Two streams must be documented, and the documentation must
  explain the interaction rules under `--format json`, `--quiet`,
  and `--verbose`. The documentation's surface is larger than
  Option A's would have been.
- A consumer that wants a single ordered view of the invocation
  must multiplex the two streams itself. POSIX provides no
  standard tool for this; the consumer's code must handle the
  interleave and the partial reads that interleaving produces.
- The CLI's success messages go to stdout and its diagnostics go
  to stderr; a user who is accustomed to stderr being reserved for
  failures may be surprised to find success diagnostics there. The
  behaviour is correct under POSIX but is not universal in
  user-facing CLIs.
- The decision's enforcement is incomplete until WBS 12.3.2's lint
  rule lands. During the interim, a violation is a code-review miss
  away from shipping.

### 4.3 Neutral

- The CLI's exit codes are unaffected. The stream contract is
  orthogonal to the exit-code taxonomy; the two interact only in
  the sense that a non-zero exit code is accompanied by an error
  block on stderr, which was already true before this decision.
- The CLI's `--format` flag is unaffected. It continues to select
  the product output's encoding; the logger's encoding is selected
  separately by `FORGE_LOG_FORMAT` (and, in a future WBS item, by a
  `--log-format` flag).
- The CLI's help output continues to go to stdout, as Cobra
  produces it. The help output is product output (the user asked
  for it), not a diagnostic.
- The version banner produced by `forge --version` continues to go
  to stdout, as Cobra produces it. It is product output.
- The `internal/logger` package's concrete implementation
  (slog-backed) is unaffected by this decision; the decision is
  about the package's stream, not its encoder.

---

## 5. Related Requirements

- `FR-CLI-OUTPUT-001` — product output is written to stdout and
  only stdout; the requirement is established by this decision.
- `FR-CLI-LOG-001` — diagnostics are written to stderr and only
  stderr; the requirement is established by this decision.
- `FR-CLI-OUTPUT-010` — under `--format json`, stdout carries
  exactly one JSON document; the requirement is established by
  WBS 10.0 and reinforced by this decision.
- `FR-CLI-LOG-002` — log level semantics and precedence; the
  requirement depends on this decision's separation between
  diagnostic levels and product output.
- `FR-CLI-LOG-009` — `--verbose` elevates the logger level to
  DEBUG and never affects stdout; the requirement depends on this
  decision's stream contract.

> The following requirements must be added to the register when
> WBS 12.1.1 is complete:

- `FR-CLI-OUTPUT-001` (if not already present)
- `FR-CLI-LOG-001` (if not already present)

---

## 6. Related ADRs

- `ADR-011` — Closed Error Category Taxonomy. Establishes that
  errors are classified by category; this decision's placement of
  errors on stderr is compatible and is the stream-level
  counterpart.
- `ADR-012` — Error Code Registry and Stability Policy. Establishes
  that error codes are stable; this decision places the rendered
  error on stderr, where the code's stability is observable.
- `ADR-020` — stdout/stderr Contract per Exit Code. Establishes the
  per-exit-code stream contents; this decision is the architectural
  reason those contents are what they are.
- `ADR-021` — JSON Error Envelope for `--format json`. Establishes
  that the error envelope is a JSON document on stdout; this
  decision's `--format json` rule is what makes the envelope's
  placement consistent with the success path's placement.
- `ADR-023` (expected) — Log Level Semantics and Precedence. Will
  define the four levels whose records this decision places on
  stderr; depends on this decision.
- `ADR-025` (expected) — Log Record Schema and Format. Will define
  the record shape that `internal/logger` produces; depends on this
  decision.

---

## 7. Notes

**Open questions deferred by this ADR:**

- Whether `internal/output` should expose a schema-versioned
  envelope for success (`{"schemaVersion": "1", "result": ...}`)
  in addition to the error envelope defined by ADR-021. The
  current WBS items assume a symmetric envelope; the exact shape
  is a separate decision.
- Whether the lint rule that enforces the contract (WBS 12.3.2)
  should be a Go analyzer (`go/analysis`) or a simpler script
  driven by the `Taskfile`. The decision is deferred to the WBS
  item, but the choice affects how violations are reported and
  how easy the rule is to extend.

**Follow-up decisions anticipated:**

- `--quiet`'s per-command behaviour (WBS 12.6.1) depends on this
  decision's stream contract; the WBS item will produce
  `docs/reference/quiet-mode.md` and an ADR if the per-command
  behaviour warrants one.
- Redaction (WBS 12.7.1) depends on this decision's placement of
  diagnostics on stderr; the redaction policy will apply to
  `internal/logger`'s output, not to `internal/output`'s.

**External references:**

- POSIX.1-2017 §2.5, "Standard I/O Streams" — the normative source
  for the stdout/stderr split.
- The Twelve-Factor App, §XI, "Logs" — the guidance that informs
  the logger's destination.
- `git`, `docker`, `kubectl`, and `go` — the precedent CLIs whose
  behaviour this decision matches.

**Implementation status:**

- `internal/output` and `internal/logger` package skeletons exist
  (WBS 12.1.1's outputs).
- `docs/architecture/output-vs-logging.md` is the authoritative
  reference for this decision's details.
- The contract test lives in `internal/cli/contract_test.go`.
- The lint rule is pending WBS 12.3.2.

---

*End of ADR-022.*
