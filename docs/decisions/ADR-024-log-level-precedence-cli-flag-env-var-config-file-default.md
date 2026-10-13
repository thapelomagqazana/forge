# ADR-024: Log level precedence: CLI flag > env var > config file > default

**Status:** Accepted
**Date:** 2026-10-13
**Deciders:** [@thapelomagqazana] (Architecture Owner)
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

ADR-023 fixed the four log levels (DEBUG, INFO, WARN, ERROR) and
their semantics. It named the flags and the environment variable that
select a level — `--verbose` (implies DEBUG), `--quiet` (implies
ERROR), `--log-level=<level>` (explicit), and `FORGE_LOG_LEVEL=<level>`
(env var) — but it deliberately deferred the question of *which source
wins* when more than one is present. That question is the subject of
this ADR. A user who sets `FORGE_LOG_LEVEL=debug` in their shell and
also passes `--quiet` on one invocation expects a well-defined
outcome. A CI system that exports `FORGE_LOG_LEVEL=warn` and also
has a config file with `log.level: info` expects a well-defined
outcome. Without a written precedence rule, the outcome depends on
which source the implementation happens to consult first, which is
undocumented and can change between releases. The CLI's `--log-level`
flag does not yet exist in `registerGlobalFlags`; this ADR is a
prerequisite for the WBS item that adds it.

Forge already has a precedence model for *other* kinds of
configuration. WBS 8.1.2 established a chain for the config file's
fields: flag > env > file > default. WBS 8.4.1 and 8.4.2 established
a discovery order for the config *path* (explicit flag, env var,
project-local, XDG, HOME, APPDATA). ADR-007 (the Phase 2
configuration foundation) named the general principle: environment
overrides committed defaults, and flags override the environment.
The log level is a per-invocation setting that also appears in the
config file (its `log.level` field), so it participates in the same
chain. Aligning the log level's precedence with the general
configuration chain means users do not have to learn a second set of
rules; a user who knows that `--config` beats `FORGE_CONFIG` beats
the file already knows how the log level resolves.

The forces at play are three. First, the log level is *both* a
per-invocation setting (a user types `--verbose` for one command)
and a persistent preference (a user puts `log.level: debug` in their
config file). The precedence rule must handle both without one
silently overriding the other in a way the user cannot explain.
Second, the `--verbose` and `--quiet` flags are *implicating* flags
— they set the level to a fixed value — while `--log-level` is an
*explicit* flag that sets the level by name. The precedence rule
must define how the explicit flag interacts with the implicating
ones. Third, the `FORGE_LOG_LEVEL` env var is the only way to set
the level for an entire CI session, and it must be overridable by a
single command's flags without requiring the user to unset it.

The decision relates to ADR-022 (the stream contract the level
controls), ADR-023 (the level vocabulary this ADR's values are
drawn from), ADR-007 (the general precedence chain this ADR aligns
with), ADR-030 (which will decide the `--verbose` flag's short
form), and ADR-031 (which will decide `--quiet`'s semantics). The
CLI UX spec's § 4.12 mentions flag interactions that depend on the
precedence. If no decision is made, the log level's resolution
remains a matter of implementation accident, and a future change to
the resolution order would be a silent behaviour change rather than
a documented one.

---

## 2. Options Considered

### Option A — Last-set-wins

The effective level is whatever source was consulted last. In
practice, this means the level is the result of whatever the code
happens to do: if the env var is read after the flags, the env var
wins; if the flags are read after the env var, the flags win. There
is no documented rule, and a future refactor can change the order.

**Pros:**

- No rule to document; the code is the specification.
- A change to the code's order is not a contract violation, so the
  implementation can be refactored freely.

**Cons:**

- The behaviour is not predictable from the user's perspective. A
  user who sets both an env var and a flag cannot know which will
  apply without reading the source.
- A refactor that changes the order is a silent behaviour change.
  Scripts that relied on the old order break with no test failure
  and no version bump.
- The behaviour cannot be documented, which means it cannot be
  taught, reviewed, or tested. Every new contributor must
  reverse-engineer it.
- The CLI's *other* precedence rules (config file discovery, config
  field resolution) are documented; a "no rule" answer for the log
  level would be inconsistent with the rest of the CLI.

### Option B — Flag > env > config > default

The effective level is determined by the highest-precedence source
that specifies a value. The sources, highest first:

1. `--log-level=<level>` (explicit CLI flag)
2. `--verbose` (implicating flag; implies DEBUG)
3. `--quiet` (implicating flag; implies ERROR)
4. `FORGE_LOG_LEVEL` (env var)
5. `log.level` (config file)
6. (default) (INFO)

An unparseable value at a higher-precedence source is a usage error;
the resolver does not fall through to a lower source. `--log-level`
beats the implicating flags; the implicating flags beat the env var;
the env var beats the config file; the config file beats the default.

**Pros:**

- Matches the general configuration chain (ADR-007, WBS 8.1.2).
  A user who knows how `--config` and `FORGE_CONFIG` interact
  already knows how the log level resolves.
- Matches the Twelve-Factor App § III: config in the environment
  overrides committed defaults, and the CLI's flags override the
  environment for a single invocation.
- Allows the common CI pattern: `FORGE_LOG_LEVEL=warn` set
  process-wide, and a single `--verbose` invocation to override
  it for debugging, without unsetting the env var.
- Allows the common user pattern: `log.level: debug` in the config
  file for persistent debugging, and `--quiet` for a single quiet
  invocation.
- The "no fall-through on invalid" rule makes a typo loud. A
  user who types `--log-level=trcae` gets a usage error, not a
  silent fallback to the config file's value.
- Testable at the resolution layer: a table of inputs and
  expected `(level, source)` pairs covers the chain exhaustively.

**Cons:**

- Six sources to document and to test. The resolution function is
  more complex than a single-source lookup.
- The interaction between `--log-level` and the implicating flags
  needs a rule (explicit wins); a user who writes `--quiet
  --log-level=debug` gets DEBUG, which may surprise them.
- The `--verbose` and `--quiet` flags are mutually exclusive
  (per ADR-032, expected), but their precedence relative to
  `--log-level` must still be defined for the case where one of
  them appears with the explicit flag.

### Option C — Flag > env > config > default, with no explicit-flag tier

The effective level is determined by the highest-precedence source
that specifies a value, but the sources are only `--verbose`,
`--quiet`, `FORGE_LOG_LEVEL`, and `log.level`. There is no
`--log-level` flag; the level is selected by choosing one of the
implicating flags or by setting the env var or config field.

**Pros:**

- Fewer sources to document and test. Four instead of six.
- No ambiguity between an explicit flag and the implicating flags;
  the implicating flags are the only flags.
- The env var and the config file already allow arbitrary levels;
  a user who wants `--log-level=warn` can set
  `FORGE_LOG_LEVEL=warn` instead.

**Cons:**

- A user who wants to set a level for one invocation must use an
  env var, which is awkward in a shell one-liner and impossible
  in a process that does not inherit the parent's environment.
  `forge --log-level=warn version` is clearer than
  `FORGE_LOG_LEVEL=warn forge version`.
- The env var's value must be read and parsed at process start;
  a user who wants to override the env var for one invocation has
  no way to do so without unsetting it.
- The implicating flags cannot express `warn` or `error`
  explicitly. A user who wants "log at WARN level for this
  invocation" has no flag to express it; `--verbose` and
  `--quiet` are the only options, and neither maps to WARN.
- The pattern "explicit flag overrides implicating flag" is
  common in CLIs (for example, `git --no-pager -p`); the absence
  of the explicit flag would force users to choose between the
  two implicating flags when neither matches their intent.

---

## 3. Decision

**Forge adopts Option B: the log level is resolved by a six-source
precedence chain, highest first: `--log-level` > `--verbose` >
`--quiet` > `FORGE_LOG_LEVEL` > `log.level` (config file) >
default.** An unparseable value at a higher-precedence source is a
usage error (exit 64); the resolver does not fall through to a lower
source. The resolution is performed by `logger.ResolveLevel`, which
returns a `Decision{Level, Source}` — not a bare `Level` — so that
the source that won is observable for diagnostics and tests. The
`Source` field is used by the DEBUG record that reports the resolved
level (see `docs/reference/verbose-mode.md` § 3). The chain is
documented in `docs/reference/log-levels.md` § 2, and its behaviour
is pinned by the test suite in `internal/logger/precedence_test.go`
and `internal/cli/flags_test.go`.

The reasoning that tipped the balance was consistency with the CLI's
existing configuration model. Forge already resolves its config
file's fields by the chain flag > env > file > default, and it
already discovers the config file's path by a chain that starts with
the explicit flag and the env var. A user who has learned either
chain has learned this one. Option C's reduction (dropping the
explicit flag tier) would make the log level the only configuration
value without an explicit flag, which is inconsistent and would
force users into an awkward env-var-only pattern for single
invocations. Option A's "no rule" is not a serious alternative: a
contract that cannot be documented cannot be taught, tested, or
relied on, and the CLI's other precedence rules are documented.

The reasoning that tipped the balance against a simpler two-tier
rule (flag > env, ignoring the config file) was that the config
file's `log.level` field already exists in the schema (see
`docs/forge-yaml-spec.md`), and a user who sets it expects it to
have an effect. Dropping the config file from the chain would require
deprecating the field, which is a larger change than this ADR needs
to make. Keeping the config file in the chain at the third tier
lets it serve its intended role ("persistent preference") while the
flags and env var override it for specific invocations.

The tradeoffs accepted are the ones Option B lists as cons. The
interaction between `--log-level` and the implicating flags is
resolved in favor of the explicit flag ("explicit beats
implicating"), which is documented and tested; a user who writes
`--quiet --log-level=debug` gets DEBUG and can read
`docs/reference/log-levels.md` § 2.1 to understand why. The
`--verbose`/`--quiet` mutual exclusion (per ADR-032) is a separate
decision that this ADR's chain does not depend on; if the exclusion
is lifted in a future ADR, the precedence of the two implicating
flags (verbose > quiet) is already defined by this ADR.

The decision is provisional in one respect: the `log.level` field's
presence in the config file is inherited from WBS 8.1.2 and is not
this ADR's subject. If a future ADR deprecates the field (for
example, to simplify the config schema), the chain loses one tier
and the resolution simplifies. Until then, the four-tier chain is
the correct shape, and the config file's tier is the lowest
explicitly-set tier.

---

## 4. Consequences

### 4.1 Positive

- The log level's resolution is predictable from the user's
  perspective: the chain matches the CLI's general configuration
  model, so a user who has learned one chain has learned the
  other.
- The common CI pattern works: `FORGE_LOG_LEVEL=warn` set
  process-wide, and a single `--verbose` invocation for
  debugging, without unsetting the env var.
- The common user pattern works: `log.level: debug` set in the
  config file for persistent debugging, and `--quiet` for a
  single quiet invocation.
- The `Source` field of the `Decision` makes the resolution's
  winner observable, which is what the DEBUG record
  (`config: resolving invocation` in `internal/cli/config.go`)
  and the tests need to assert the chain end to end.
- The "no fall-through on invalid" rule makes a typo loud. A
  user who types `--log-level=trcae` gets a usage error with a
  clear message, not a silent fallback.
- The chain is testable at the resolution layer as a table of
  inputs and expected `(level, source)` pairs, which is a
  natural shape for a unit test.

### 4.2 Negative

- Six sources to document and to test. The resolution function
  is more complex than a single-source lookup, and every WBS
  item that consults the level must use the resolver, not a
  direct read.
- The interaction between `--log-level` and the implicating flags
  is a rule the user must learn. A user who writes `--quiet
  --log-level=debug` gets DEBUG, which is the correct outcome
  but is not the outcome a first-time user might expect.
- The `--verbose`/`--quiet` mutual exclusion (ADR-032) means
  the two implicating flags cannot be combined, which is a
  separate constraint that this ADR's chain has to accommodate
  (the chain's tiers 2 and 3 are never both reached in a valid
  invocation).
- The config file's `log.level` field at the third tier means
  a user who sets it cannot override it with another config
  file's value (the file is discovered, not layered); a user
  who wants a per-project override uses the project-local
  config file, which is the intended pattern but is worth
  documenting.

### 4.3 Neutral

- The order in which the resolver consults the sources is an
  implementation detail; the ADR specifies the precedence, not
  the code's structure. A future refactor that consults the
  sources in a different order but produces the same result is
  compliant with this ADR.
- The `Decision.Source` type's string representations
  (`"flag(--log-level)"`, `"env(FORGE_LOG_LEVEL)"`, etc.) are
  documented in `internal/logger/precedence.go`'s docstrings
  but are not part of this ADR's contract. A future change to
  the strings is a diagnostic-surface change, not a precedence
  change.
- The env var's case sensitivity (`FORGE_LOG_LEVEL` is
  uppercase) follows the POSIX convention for environment
  variables and is not a decision this ADR needs to make.
- The config file's `log.level` field's *name* is inherited
  from WBS 8.1.2 and is not this ADR's subject.

---

## 5. Related Requirements

- `FR-CLI-LOG-002` — the log level precedence chain; the
  requirement is established by this decision (with ADR-023).
- `FR-CLI-LOG-003` — the flag mapping that participates in the
  chain (`--verbose` → DEBUG, `--quiet` → ERROR, `--log-level` →
  explicit); the requirement depends on this decision's
  precedence order.
- `FR-CLI-LOG-009` — `--verbose` elevates the level to DEBUG
  unless a higher-precedence source overrides it; the
  requirement depends on this decision's chain.
- `FR-CLI-LOG-010` — `--quiet` elevates the level to ERROR
  unless a higher-precedence source overrides it; the
  requirement depends on this decision's chain.
- `NFR-OBSERVABILITY-001` — records are attributable to a level
  and to the source that set it; the requirement is constrained
  by this decision's `Decision.Source` field.

---

## 6. Related ADRs

- `ADR-007` — Configuration Foundation: Precedence, Discovery,
  and Error Taxonomy. Establishes the general precedence chain
  this ADR's log level chain mirrors.
- `ADR-022` — Output and logging are separate architectural
  layers. Establishes the stream on which the level's records
  appear; this decision controls the level for that stream.
- `ADR-023` — Four log levels: ERROR, WARN, INFO, DEBUG.
  Establishes the vocabulary this ADR's values are drawn from.
- `ADR-025` (expected) — Log records are structured with a
  schema-versioned JSON form and an unstable human form. Will
  define the record whose `level` field carries the resolved
  value.
- `ADR-026` (expected) — The Logger interface has four level
  methods and a With combinator. Will define the interface whose
  filtering uses the resolved level.
- `ADR-030` (expected) — `--verbose` is long-only; `-v` is
  reserved for `--version`. Will define the flag whose tier in
  this chain is 2.
- `ADR-031` (expected) — `--quiet` suppresses diagnostics but
  never suppresses errors. Will define the flag whose tier in
  this chain is 3.
- `ADR-032` (expected) — `--quiet` and `--verbose` are mutually
  exclusive. Will define the constraint that prevents tiers 2
  and 3 from both being reached.

---

## 7. Notes

**Open questions deferred by this ADR:**

- Whether `--log-level` and `--verbose`/`--quiet` should be
  mutually exclusive. This ADR's chain resolves the conflict in
  favor of `--log-level` ("explicit beats implicating"), but
  the alternative — rejecting the combination as a usage error
  — is defensible and could be the subject of a future ADR. The
  current decision is the permissive one.
- Whether the config file's `log.level` field should be
  deprecated in favor of the env var and the flags. The field
  is inherited from WBS 8.1.2; a future ADR could remove it,
  which would simplify the chain to three tiers (flag > env >
  default). This ADR keeps the field because it already exists
  and serves the "persistent preference" role.

**Follow-up decisions anticipated:**

- The `--log-level` flag's registration in
  `internal/cli/flags.go` is a WBS item that depends on this
  ADR; the flag's docstring will cite
  `docs/reference/log-levels.md` § 2 for the precedence chain.
- The `--verbose` and `--quiet` flags' mutual exclusion
  (ADR-032) is a separate decision that interacts with this
  ADR's tiers 2 and 3. If the exclusion is lifted, this ADR's
  order (verbose > quiet) becomes observable in a valid
  invocation rather than only in a resolver test.
- The DEBUG record that reports the resolved level's source
  (in `internal/cli/execute.go` or a hook) is a small addition
  that this ADR's `Decision.Source` field enables; its exact
  placement is a WBS item, not an ADR.

**External references:**

- The Twelve-Factor App, § III, "Config" — the guidance that
  environment variables override committed defaults and that
  command-line flags override the environment for a single
  invocation.
- POSIX.1-2017 § 8.1, "Environment Variables" — the normative
  source for env var naming and case.
- `git`, `docker`, `kubectl` — the precedent CLIs whose flag/
  env/config precedence models Forge's chain mirrors.
- `docs/reference/log-levels.md` § 2 — the authoritative
  reference for this ADR's chain, including the resolution
  table and the "no fall-through on invalid" rule.
- `docs/forge-yaml-spec.md` — the schema that defines the
  `log.level` field whose tier in this chain is 4.

---

*End of ADR-024.*
