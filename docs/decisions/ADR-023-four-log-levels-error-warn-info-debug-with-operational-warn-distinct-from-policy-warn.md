# ADR-023: Four log levels: ERROR, WARN, INFO, DEBUG — with operational WARN distinct from policy WARN

**Status:** Accepted
**Date:** 2026-10-13
**Deciders:** [@thapelomagqazana] (Architecture Owner)
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

The logger introduced by ADR-022 writes diagnostics to stderr and has
a notion of "level" — a filter that determines which records reach
the stream. But until now, the levels themselves have not been
defined. The current `internal/logger` package has four level
constants (`LevelDebug`, `LevelInfo`, `LevelWarn`, `LevelError`) and
a `ParseLevel` function, but no written rule about what each level
*means*, which level a given event belongs to, or how the levels
interact with the flags that select them (`--verbose`, `--quiet`,
`--log-level`) or with the environment variable that overrides them
(`FORGE_LOG_LEVEL`). The CLI UX spec's § 4 mentions "log levels" in
passing but does not enumerate them. The `--verbose` and `--quiet`
flags are planned (WBS 12.5.1 and WBS 12.6.1) and require the levels'
semantics to be fixed before their own semantics can be decided.

The forces at play are three. First, the four-level vocabulary is not
universal. Many systems have five (`TRACE`, `DEBUG`, `INFO`, `WARN`,
`ERROR`), others have six (`FATAL`), and some have three (`INFO`,
`WARN`, `ERROR`). Choosing four and naming them is a decision that
excludes alternatives, and the choice determines the vocabulary that
every record, every flag, and every future subsystem will use.
Second, the word `WARN` carries two unrelated meanings in Forge. An
*operational* `WARN` is a diagnostic about how a command ran (a
fallback was used, a deprecated form was accepted). A *policy* `WARN`
is a finding produced by a check command — a statement about the
subject of the check, printed to stdout as product output. The two
share a word but not a stream, a package, or a consumer. Without an
explicit distinction, a future contributor is likely to conflate them,
and the conflation would be invisible until a bug report about
misrouted output arrived. Third, the `FORGE_LOG_LEVEL` environment
variable and the `--log-level` flag both need a value set, and the
value set must be stable for scripts and CI configurations to depend
on it.

The decision relates to ADR-022 (the stream contract this ADR's levels
apply to), ADR-024 (which will define the precedence chain that
selects a level), ADR-025 (which will define the record shape whose
`level` field carries these values), and ADR-026 (which will define
the interface that emits records at these levels). The CLI UX spec's
§ 4.12 mentions flag semantics that depend on the level vocabulary.
If no decision is made, the level constants remain undocumented, the
`WARN` conflation remains possible, and the `--verbose` and `--quiet`
WBS items cannot specify their behaviour unambiguously.

---

## 2. Options Considered

### Option A — Three levels: INFO, WARN, ERROR

Logs at INFO, WARN, and ERROR. `DEBUG` does not exist; a user who
wants more detail passes `--verbose` and receives additional INFO
records (a convention the CLI would define). `--quiet` suppresses
INFO and WARN.

**Pros:**

- Smaller vocabulary; fewer levels to define and test.
- Matches tools that collapse `DEBUG` into a "verbose INFO" mode.
- The `--verbose` flag's semantics are already implicit: "show more
  INFO records"; no separate DEBUG concept is required.
- Removes the temptation to log at DEBUG "just in case", which is a
  common source of log noise.

**Cons:**

- A user who wants internal state for troubleshooting has no way to
  request it independently of the default INFO records; the flag
  conflates "more INFO" with "internal state", which are different
  kinds of information.
- A future subsystem that wants a diagnostic channel separate from
  the default INFO stream (for example, a trace of file resolution)
  has no level to place it at without polluting INFO.
- The convention "verbose means more INFO" is a common source of
  disagreement: some users expect `--verbose` to mean "show
  DEBUG records" (per `curl`, `ssh`, `gcc`), others expect "show
  more of the same" (per `tar`, `rsync`). Without DEBUG as a
  distinct level, the CLI cannot disambiguate.
- Adding DEBUG later is a breaking change to the value set that
  scripts and CI configurations depend on.

### Option B — Five levels: TRACE, DEBUG, INFO, WARN, ERROR

Adds `TRACE` below `DEBUG` for very fine-grained information
(per-byte, per-iteration). `--verbose` enables DEBUG; a separate
`--trace` flag (or `--log-level=trace`) enables TRACE.

**Pros:**

- Matches the vocabulary of many structured-logging systems
  (`slog`'s default handler, `zap`, `zerolog`, OpenTelemetry).
- Gives a future high-volume diagnostic channel (for example, a
  per-file trace of template rendering) a place to live without
  competing with DEBUG's troubleshooting role.
- The five-level vocabulary is familiar to users of modern Go
  logging libraries.

**Cons:**

- The distinction between TRACE and DEBUG is one that most users
  cannot articulate; in practice, the two blur, and a subsystem's
  author chooses between them by intuition.
- Five levels increase the surface of every flag (`--verbose`,
  `--quiet`, `--log-level`), every test (four levels' worth of
  filters become five), and every document (the level table grows
  by one row that most readers skip).
- No current or planned Forge subsystem has TRACE-level detail to
  emit. Adding the level now would be speculative and would
  violate the "no speculative additions" rule that governs the
  CLI's flags (see the package docstring of `internal/cli/flags.go`).
- If a future subsystem needs TRACE, it can request its addition
  with an ADR that names the subsystem and its data. That ADR is
  cheap to write when the need is real; adding the level now is
  writing it speculatively.

### Option C — Six levels: TRACE, DEBUG, INFO, WARN, ERROR, FATAL

Adds `FATAL` above `ERROR` for failures that terminate the process
(after logging, the process exits non-zero).

**Pros:**

- Matches `logrus`, `log4j`, and other logging frameworks that
  distinguish "this error is being returned" from "this error is
  ending the process".
- Gives the process-boundary panic path (WBS 10.5.1) a level at
  which to log the panic before exiting.

**Cons:**

- The FATAL level duplicates the exit-code taxonomy. Forge's exit
  codes already distinguish "the command failed" from "the process
  is terminating" (see ADR-018, ADR-020); a log level that mirrors
  the distinction would produce two ways to express the same fact,
  with no rule for keeping them consistent.
- A FATAL record's emission is coupled to process termination,
  which makes it hard to test in isolation and hard to route
  through the logger's normal `Sink` abstraction (the process is
  about to exit; the sink may be flushed asynchronously).
- In practice, FATAL is used for a vanishingly small number of
  events (panics and unrecoverable startup failures), and Forge's
  panic path already logs through `forgeerr`, not through the
  logger. The level would be defined and never emitted from
  ordinary code.
- Six levels' worth of filters, tests, and documentation for one
  level's worth of actual use.

### Option D — Four levels: DEBUG, INFO, WARN, ERROR — with operational WARN explicitly distinct from policy WARN

Logs at DEBUG, INFO, WARN, and ERROR. `WARN` is defined as an
*operational* warning: a recoverable anomaly in how Forge ran. It is
explicitly distinguished from a *policy* warning, which is a finding
produced by `forge check` (or a future `forge validate`) and printed
to stdout as product output. The two are documented as unrelated, and
the packages that produce them are different (`internal/logger` for
operational WARN, the check command's own package for policy WARN).

**Pros:**

- The four levels match the vocabulary of the tools Forge's users
  already know: `git`, `docker`, `kubectl`, and Go's `log/slog`
  default all use four or fewer levels, and INFO/WARN/ERROR is the
  common subset.
- DEBUG is the explicit "internal state for troubleshooting" level,
  which disambiguates `--verbose` (it means DEBUG, not "more INFO").
- The operational/policy WARN distinction is named in the ADR and
  documented in `docs/reference/log-levels.md`, which prevents the
  most likely source of confusion.
- No speculative levels; TRACE and FATAL can be added later if a
  real need arises, and the addition is then justified by the
  subsystem that needs it.
- The `--log-level` value set is small and stable: `debug`, `info`,
  `warn`, `error`, plus the `warning` alias. Scripts and CI
  configurations can depend on it.

**Cons:**

- A future subsystem with TRACE-level detail has no place to put
  it without an ADR; the cost is deferred, not eliminated.
- The operational/policy WARN distinction is a documentation
  discipline rather than a type-level guarantee; a contributor who
  ignores the documentation can still produce a `logger.Warn` call
  from a check command, and the two streams' separation will
  keep the bug from being immediately visible.
- The word "WARN" appears in two different contracts, and a reader
  who finds one of them first may not realize the other exists.
  Mitigated by cross-references from both documents.

---

## 3. Decision

**Forge adopts Option D: four log levels — DEBUG, INFO, WARN, ERROR
— with WARN defined as an operational warning and explicitly
distinguished from policy WARN.** The levels' semantics are:

| Level | Meaning | Audience | Emitted when |
|-------|---------|----------|--------------|
| DEBUG | Internal state for troubleshooting: values resolved, branches taken, caches hit or missed, timing measurements, retries | Developers diagnosing a bug | `--verbose`, `--log-level=debug`, or `FORGE_LOG_LEVEL=debug` |
| INFO | Progress milestones and lifecycle events: starting, finishing, loading, writing, connecting | Users who want to see what a command did | Default; suppressed by `--quiet` |
| WARN | Operational warnings: recoverable anomalies worth the user's attention (a fallback was used, a deprecated form was accepted, a value was coerced, a retry succeeded) | Operators and users who care about degraded behavior | Default; suppressed by `--log-level=error` or `FORGE_LOG_LEVEL=error` |
| ERROR | Failures: Forge could not complete an operation | Everyone | Always; cannot be suppressed by any flag, env var, or config value |

The operational WARN is distinct from the *policy* WARN that
`forge check` and `forge validate` produce. The two share a word but
not a stream, a package, or a consumer: operational WARN is a
diagnostic about how Forge ran, goes to stderr through
`internal/logger`, and is selected by `--log-level`; policy WARN is a
finding about the subject of a check, goes to stdout through the
check command's own product-output path, and is selected by the
check command's own flags. The distinction is documented in
`docs/reference/log-levels.md` § 3 with a side-by-side table, and
both `internal/logger`'s `LevelWarn` constant and the check command's
finding-severity type carry cross-references to that section.

The reasoning that tipped the balance was DEBUG's disambiguation
role. Option A's "verbose means more INFO" is a convention that
users misinterpret; `curl -v`, `ssh -v`, and `gcc -v` all mean
"show internal state", not "show more of the same". Without a
distinct DEBUG level, `--verbose` cannot match that expectation
without redefining INFO, and redefining INFO would break the
"progress milestones" role that INFO plays by default. Option B's
TRACE and Option C's FATAL are both plausible additions, but neither
has a current or planned Forge subsystem that would emit at the
level. Adding either now would be speculative; adding them later, if
a real need arises, is cheap (an ADR that names the subsystem). The
"no speculative additions" rule that governs the CLI's flags applies
equally to its log levels.

The tradeoffs accepted are the ones Option D lists as cons. The
deferral of TRACE and FATAL is accepted on the assumption that the
current subsystem set (CLI, config, version) has no data at either
level; if a future subsystem (for example, a template renderer or a
remote registry client) needs TRACE-level detail, the ADR to add the
level is expected and welcome. The operational/policy WARN
distinction being a documentation discipline rather than a
type-level guarantee is accepted because a type-level guarantee
would require the check command's finding type and the logger's
level type to be related, which would couple two packages that have
no business knowing about each other; the documentation is the
correct level of enforcement for a cross-package convention.

The decision is provisional in one respect: the `warning` alias for
`warn` is accepted today because some users will type it; if a
future release finds that the alias causes more confusion than it
avoids, removing it is a change to this ADR rather than a new one.
Similarly, the exclusion of TRACE and FATAL is a decision of
*insufficiency*, not of *prohibition*: the levels are not forbidden,
they are simply not present, and adding them is a normal extension
of this ADR rather than a violation of it.

---

## 4. Consequences

### 4.1 Positive

- The level vocabulary is small, familiar, and stable. Users of
  `git`, `docker`, `kubectl`, and `log/slog` find Forge's levels
  immediately recognizable; scripts and CI configurations can
  depend on the `--log-level` value set.
- `--verbose`'s meaning is unambiguous: it enables DEBUG records,
  which are internal state for troubleshooting. This matches the
  expectation set by `curl`, `ssh`, and `gcc`.
- The operational/policy WARN distinction is named in the ADR and
  documented in the reference doc, preventing the most likely
  source of confusion between the logger's WARN and a check
  command's WARN finding.
- The `--log-level` value set and `FORGE_LOG_LEVEL`'s accepted
  values are the same four strings (`debug`, `info`, `warn`,
  `error`) plus the `warning` alias; a script that configures one
  can configure the other.
- The `--quiet` flag's semantics follow from INFO's and WARN's
  roles: `--quiet` suppresses INFO and WARN but not ERROR, which
  is exactly the "tell me only what's wrong" behavior users
  expect from a quiet mode.
- No speculative levels. If a future subsystem needs TRACE or
  FATAL, the ADR to add it is written when the need is real, with
  the subsystem named.

### 4.2 Negative

- A future subsystem with TRACE-level detail has no place to put
  it without an ADR. The cost is deferred, not eliminated; when
  the need arises, the ADR adds a level whose role must be
  distinguished from DEBUG's, which is a design question rather
  than a mechanical one.
- The operational/policy WARN distinction is a documentation
  discipline rather than a type-level guarantee. A contributor
  who writes `logger.Warn` from inside a check command's package
  will produce an operational WARN record on stderr *and* a
  policy WARN finding on stdout, and the two will be unrelated.
  The contract test in `internal/cli/contract_test.go` cannot
  catch this because the two are in different packages; the
  prevention is documentation and code review.
- The word "WARN" appears in two contracts. A reader who finds
  only one may not realize the other exists. The mitigation is a
  cross-reference in both directions (from `internal/logger`'s
  `LevelWarn` docstring to `log-levels.md` § 3, and from the
  check command's finding type to the same section).
- The `warning` alias for `warn` is accepted but is not the
  canonical spelling; a user who uses it in a config file will
  see the value normalized to `warn` in the resolved output,
  which may be surprising. The alternative — rejecting the alias
  — would be stricter but would break users who type it.

### 4.3 Neutral

- The four levels' numeric ordering (`DEBUG < INFO < WARN <
  ERROR`) is an implementation detail of the `Level` type; the
  ADR does not specify it as part of the contract. A future
  refactor could reorder the underlying integers without changing
  the levels' names or semantics, provided the ordering rule
  ("a threshold enables itself and every higher level") is
  preserved.
- The `warning` alias's status is unspecified beyond "accepted";
  whether it appears in `--help` output, in the reference doc's
  canonical spelling, or in error messages is a matter for each
  surface to decide.
- The colors used to render levels in a terminal (if any) are
  not part of the contract. The reference doc forbids color
  codes in the current phase (see `internal/cli/config.go`'s
  rule F9 for the analogous product-output rule); a future phase
  may introduce them under a `--color` flag, which would be a
  separate ADR.

---

## 5. Related Requirements

- `FR-CLI-LOG-002` — the four levels and their semantics; the
  requirement is established by this decision.
- `FR-CLI-LOG-003` — the level precedence chain and the flag
  mapping (`--quiet` → ERROR, default → INFO, `--verbose` →
  DEBUG, `--log-level` → explicit); the requirement depends on
  this decision's vocabulary.
- `FR-CLI-LOG-004` — the log record's `level` field; the
  requirement depends on this decision's level constants.
- `FR-CLI-LOG-009` — `--verbose` elevates the level to DEBUG;
  the requirement depends on this decision's DEBUG semantics.
- `FR-CLI-LOG-010` — `--quiet` elevates the level to ERROR; the
  requirement depends on this decision's INFO, WARN, and ERROR
  semantics.
- `NFR-OBSERVABILITY-001` — records are attributable to a level;
  the requirement is constrained by this decision's vocabulary.

---

## 6. Related ADRs

- `ADR-022` — Output and logging are separate architectural
  layers. Establishes the stream on which the levels' records
  appear; this decision depends on it.
- `ADR-024` (expected) — Log level precedence: CLI flag > env
  var > config file > default. Will define how a level is
  selected; depends on this decision's vocabulary.
- `ADR-025` (expected) — Log records are structured with a
  schema-versioned JSON form and an unstable human form. Will
  define the record whose `level` field carries these values;
  depends on this decision.
- `ADR-026` (expected) — The Logger interface has four level
  methods and a With combinator. Will define the interface that
  emits records at these levels; depends on this decision.
- `ADR-030` (expected) — `--verbose` is long-only; `-v` is
  reserved for `--version`. Will define the flag whose semantics
  depend on DEBUG; depends on this decision.
- `ADR-031` (expected) — `--quiet` suppresses diagnostics but
  never suppresses errors. Will define the flag whose semantics
  depend on INFO, WARN, and ERROR; depends on this decision.

---

## 7. Notes

**Open questions deferred by this ADR:**

- Whether the `warning` alias should appear in `--help` output
  as an accepted value, or only be silently accepted. The
  current implementation accepts it silently; the question is
  whether a user who reads `--help` should learn that
  `--log-level=warning` is valid.
- Whether a future ADR should add TRACE and FATAL levels, and
  under what conditions. The current ADR's answer is "only when
  a named subsystem needs them"; a future ADR that adds either
  should name the subsystem and the data it emits.

**Follow-up decisions anticipated:**

- The `--verbose` flag's behavior (WBS 12.5.1) is fully
  determined by this ADR's DEBUG semantics; the WBS item's
  output is `docs/reference/verbose-mode.md` and integration
  tests, not an ADR.
- The `--quiet` flag's per-command behavior (WBS 12.6.1) is
  determined by this ADR's INFO and WARN semantics, but the
  per-command specifics (what "the final result" means for each
  command) may warrant an ADR if the behavior diverges across
  commands in a way that is not obvious from the levels alone.
- The `FORGE_LOG_LEVEL` environment variable's precedence
  relative to `--log-level` is decided by ADR-024, not this
  ADR; this ADR names the accepted values but not the
  resolution order.

**External references:**

- Go standard library `log/slog` — the four-level vocabulary
  (Debug, Info, Warn, Error) that Forge adopts, with the
  same ordering and the same "threshold enables itself and
  higher" rule.
- `git`, `docker`, `kubectl` — the precedent CLIs whose log
  levels Forge's four-level set matches.
- The Twelve-Factor App, §XI, "Logs" — the guidance that
  informs the levels' role as diagnostics (not product output).
- `docs/reference/log-levels.md` — the authoritative reference
  for this ADR's details, including the operational/policy
  WARN distinction's side-by-side table.

---

*End of ADR-023.*
