# ADR-032: `--quiet` and `--verbose` are mutually exclusive

**Status:** Accepted
**Date:** 2026-10-13
**Deciders:** [@thapelomagqazana] (CLI UX Owner)
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

ADR-030 fixed `--verbose` as a long-only flag that elevates the log
level to DEBUG. ADR-031 fixed `--quiet` as a flag that elevates the
log level to ERROR, suppresses informational output, and preserves
the error block. Both flags are persistent on the root command, and
both are therefore visible on every subcommand. The two flags
express opposite intents: `--verbose` asks for *more* diagnostics;
`--quiet` asks for *fewer*. When a user passes both — because their
shell alias includes one and their one-off invocation includes the
other, because a script was assembled from two sources that each
chose a flag, or because the user simply typed both — the CLI must
resolve the conflict somehow. The decision this ADR must make is
what "resolve the conflict" means: does the CLI pick one flag's
intent and ignore the other, or does it reject the combination
outright?

The forces at play are three. First, the flags' precedences are
already defined by ADR-024, which places `--verbose` (tier 2) above
`--quiet` (tier 3) in the log-level chain. A user who reads ADR-024
and passes both flags with no other source gets DEBUG (from the
higher tier), not ERROR. The chain resolves the *level*, but it
does not resolve the *question of whether the combination should be
accepted at all*; a level can be computed while the combination is
still nonsense. Second, the flags' non-level effects differ.
`--verbose` also enables verbose error rendering (per ADR-030's
relationship to the error block); `--quiet` also suppresses
informational output (per ADR-031). A user who passes both is
asking for verbose errors *and* suppressed informational output
*and* a level that cannot be both DEBUG and ERROR. The combination
is internally inconsistent, not merely "one flag wins." Third, the
CLI's precedent for conflicting flags is to reject them. The
pre-parse validator (per WBS 4.1.2 and `internal/cli/validate.go`)
already rejects malformed invocations that Cobra would otherwise
accept silently; the validator's philosophy is that a conflicting
invocation is a user error that the CLI should surface, not a
precedence tie that the CLI should silently resolve.

The decision relates to ADR-023 (the level vocabulary the flags
select), ADR-024 (the precedence chain the flags participate in),
ADR-030 (the `--verbose` flag's shape), ADR-031 (the `--quiet`
flag's behavior), and ADR-011 (the error category taxonomy, which
classifies the rejection). The CLI UX spec's § 4.12 mentions the
flags' interaction but does not define the combined behavior. If
no decision is made, a user who passes both flags cannot predict
whether the CLI will accept the combination, reject it, or pick
one silently; and a future contributor who implements the flags
has no rule to follow.

---

## 2. Options Considered

### Option A — `--quiet` wins; `--verbose` is ignored

When both flags are present, the effective level is ERROR (from
`--quiet`); the `--verbose` flag's effects are discarded. The
invocation proceeds; no error is raised.

**Pros:**

- The invocation succeeds; a script that ends up with both flags
  does not break.
- The "quieter" flag wins, which matches the intuition that a
  user who asks for quiet wants less noise, and the extra
  verbosity is the noise they want to suppress.
- The behavior is a strict subset of `--quiet`'s behavior:
  a user who passes both gets exactly what `--quiet` alone
  would produce.

**Cons:**

- The `--verbose` flag is silently ignored. A user who passed
  it (perhaps expecting verbose errors) sees neither verbose
  errors nor an explanation of why the flag had no effect. The
  silent ignore is the exact class of behavior the pre-parse
  validator exists to prevent.
- The rule "quiet wins" is a precedence tie that the user must
  learn; it is not derivable from the flags' names. A user who
  expects "the last flag wins" or "the first flag wins" is
  surprised.
- The pre-parse validator's philosophy (reject malformed
  invocations) is contradicted. The validator rejects
  `--help <cmd>` and other malformed invocations; the
  combination `--quiet --verbose` is equally malformed and
  should be equally rejected.

### Option B — `--verbose` wins; `--quiet` is ignored

When both flags are present, the effective level is DEBUG (from
`--verbose`); the `--quiet` flag's effects are discarded. The
invocation proceeds; no error is raised.

**Pros:**

- The invocation succeeds; a script that ends up with both flags
  does not break.
- The "more diagnostic" flag wins, which matches the intuition
  that a user who asks for verbose wants more information, and
  the quiet flag's suppression would hide the information they
  asked for.
- The behavior is a strict subset of `--verbose`'s behavior:
  a user who passes both gets exactly what `--verbose` alone
  would produce.

**Cons:**

- The `--quiet` flag is silently ignored. A user who passed it
  (perhaps expecting suppressed informational output) sees
  neither the suppression nor an explanation of why the flag
  had no effect. The silent ignore is the same defect as
  Option A's.
- The rule "verbose wins" is the opposite of Option A's and is
  equally non-derivable from the flags' names. A user who
  expects "the last flag wins" is surprised.
- The pre-parse validator's philosophy is contradicted, as in
  Option A.

### Option C — The last flag on the command line wins

When both flags are present, the flag that appears later in the
argument list determines the effective behavior. `--quiet
--verbose` means verbose; `--verbose --quiet` means quiet.

**Pros:**

- The behavior matches the intuition that the last flag's
  intent is the user's current intent; a user who typed
  `--verbose --quiet` changed their mind.
- No flag is silently ignored; the user's later choice is
  honored, and the earlier choice is overridden by an
  explicit, later statement.
- The behavior is deterministic and easy to explain.

**Cons:**

- The rule "the last flag wins" is a convention, not a
  property; a user who expects "the first flag wins" or
  "the flags are independent" is surprised.
- The two flags' effects are not symmetric (per ADR-031,
  `--quiet` suppresses informational output; per ADR-030,
  `--verbose` enables verbose error rendering). A "last
  wins" rule does not resolve the non-level effects: a user
  who passes `--verbose --quiet` gets quiet's level (ERROR)
  and verbose's error rendering (the last flag's effects on
  the level, but the first flag's effects on the error
  rendering). The rule's simplicity is an illusion.
- The behavior is order-dependent, which is unusual for a
  CLI's flags; most CLIs treat flags as a set, not a
  sequence.
- The user's intent is not clarified by the combination;
  a user who wants verbose output should not pass
  `--quiet`, and a user who passes both is expressing a
  confused intent that the CLI should surface, not silently
  satisfy.

### Option D — Reject the combination with exit code 64 and a clear error

When both flags are present, the CLI rejects the invocation as a
usage error. The error message names the two flags as mutually
exclusive. The exit code is 64 (BSD EX_USAGE, per ADR-018). No
command runs; the rejection happens during the flag validation
phase (per `validateLogFlags` in `internal/cli/flags.go`).

**Pros:**

- The user's error is surfaced. A user who passed both flags
  (perhaps by accident) sees a clear message explaining the
  conflict and can correct the invocation. The alternative —
  silently ignoring one flag — hides the error, and the
  user's script continues with unintended behavior.
- The pre-parse validator's philosophy is honored: a
  malformed invocation is rejected, not silently resolved.
  The combination `--quiet --verbose` is malformed in the
  same sense as `--help <cmd>`.
- The rule is easy to learn: the two flags cannot be
  combined. A user who reads the error message knows what
  to do.
- The exit code (64, EX_USAGE) matches the convention for
  usage errors; a script that inspects the exit code
  distinguishes the conflict from other failure modes.
- The behavior is order-independent: `--quiet --verbose`
  and `--verbose --quiet` are both rejected, which matches
  the user's mental model of flags as a set.
- The mutual exclusion's enforcement is a single check in
  `validateLogFlags`, which is already the function that
  handles the flags' interaction. The check is small and
  testable.

**Cons:**

- The invocation fails. A script that ends up with both
  flags (because a wrapper added one and an environment set
  the other, or because two configuration sources were
  merged) breaks where it might have worked with a "one
  wins" rule. The break is the price of surfacing the error.
- The user must correct the invocation. A user who is in
  the middle of a debugging session and accidentally types
  both flags must retype the command; the extra step is a
  minor cost.
- The rejection is a behavior change from the pre-ADR state
  (before this decision, the flags did not exist and the
  combination was impossible). A future release that added
  the flags and later reversed the decision would need a
  deprecation period; this ADR's decision is the initial
  state, so no deprecation is required.

---

## 3. Decision

**Forge adopts Option D: `--quiet` and `--verbose` are mutually
exclusive, and passing both is a usage error (exit 64) with a
clear error message.** The check is performed by
`validateLogFlags` in `internal/cli/flags.go`, which runs during
the flag validation phase before any command logic. The error
message names the two flags as mutually exclusive and cites
`docs/reference/log-levels.md` § 4.5. The rejection is a
`*forgeerr.Error` with category `CategoryCLI` and code
`CodeCLIQuietVerboseConflict` (or the appropriate registered
code); its rendering on stderr follows the standard error block
format (per ADR-014), and its exit code is 64 (per ADR-018's
usage-error convention).

The reasoning that tipped the balance was the pre-parse
validator's philosophy. The validator rejects `--help <cmd>`,
`--version <arg>`, `--config` without a value, and other
malformed invocations because the alternative — silently
interpreting them — hides user errors. The combination
`--quiet --verbose` is malformed in the same sense: it asks for
two opposite things, and neither a "quiet wins" rule (Options A)
nor a "verbose wins" rule (Option B) nor a "last flag wins" rule
(Option C) is derivable from the flags' names. The user who
passes both flags is expressing a confused intent, and the CLI's
correct response is to surface the confusion, not to silently
pick one intent and discard the other.

The reasoning against Options A and B (the "one wins" rules) was
the silent ignore. A user who passes `--quiet --verbose` and
sees quiet's behavior has no way to know that their `--verbose`
flag was discarded; a user who passes the same combination and
sees verbose's behavior has no way to know that their `--quiet`
flag was discarded. In either case, the flag's absence from the
output is invisible, and the user's mental model of the CLI's
behavior is wrong. The silent ignore is the exact class of defect
the pre-parse validator exists to prevent; a decision that
introduces a new silent ignore in the validator's own domain is
inconsistent with the validator's purpose.

The reasoning against Option C (the "last flag wins" rule) was
the flags' non-symmetry. ADR-030 and ADR-031 define the flags'
effects beyond the log level: `--verbose` enables verbose error
rendering, and `--quiet` suppresses informational output. A "last
flag wins" rule resolves the level (the last flag's tier in
ADR-024's chain), but it does not resolve the flags' non-level
effects. A user who passes `--verbose --quiet` would get quiet's
level (ERROR) but verbose's error rendering (from the first
flag's effect on the rendering), which is neither flag's complete
behavior. The rule's simplicity is an illusion; the flags'
effects are not reducible to a single "last wins" dimension.

The tradeoffs accepted are the ones Option D lists as cons. The
invocation fails when both flags are present, which breaks a
script that ends up with both; the break is the price of
surfacing the error. The user must correct the invocation, which
is a minor cost. The rejection is a behavior change from the
pre-ADR state, but the state was "the flags did not exist," so no
deprecation is required; the decision is the initial state.

The decision is provisional in one respect: the flags' precedence
chain (ADR-024) places `--verbose` (tier 2) above `--quiet`
(tier 3), and the chain's order is defined even though the
combination is rejected. If a future ADR lifts the mutual
exclusion (for example, to allow `--verbose` to override
`--quiet` in a "verbose wins" rule), the chain's order is the
fallback. Until then, the chain's tiers 2 and 3 are never both
reached in a valid invocation, but the order is documented for
the case that the exclusion is lifted.

---

## 4. Consequences

### 4.1 Positive

- The user's error is surfaced. A user who passes both flags
  (perhaps by accident) sees a clear message explaining the
  conflict and can correct the invocation.
- The pre-parse validator's philosophy is honored: a malformed
  invocation is rejected, not silently resolved. The
  combination `--quiet --verbose` is treated like any other
  malformed invocation.
- The rule is easy to learn: the two flags cannot be
  combined. A user who reads the error message knows what
  to do.
- The exit code (64, EX_USAGE) matches the convention for
  usage errors; a script that inspects the exit code
  distinguishes the conflict from other failure modes.
- The behavior is order-independent: `--quiet --verbose` and
  `--verbose --quiet` are both rejected, which matches the
  user's mental model of flags as a set.
- The mutual exclusion's enforcement is a single check in
  `validateLogFlags`, which is already the function that
  handles the flags' interaction. The check is small,
  testable, and centralized.
- The decision is pinned by
  `TestValidateLogFlags_QuietVerboseConflict` and
  `TestResolveLogLevel_ConflictIsUsageError` in
  `internal/cli/flags_test.go`, and by the CLI-level
  acceptance test `TestVerbose_ConflictWithQuiet` in the
  binary integration tests. A future contributor who wants
  to flip the decision must update the tests, which forces
  the change to be deliberate.

### 4.2 Negative

- The invocation fails when both flags are present. A script
  that ends up with both flags (because a wrapper added one
  and an environment set the other, or because two
  configuration sources were merged) breaks where it might
  have worked with a "one wins" rule. The break is the
  price of surfacing the error.
- The user must correct the invocation. A user in the middle
  of a debugging session who accidentally types both flags
  must retype the command; the extra step is a minor cost.
- The rejection is a behavior change from the pre-ADR state.
  The state was "the flags did not exist," so no deprecation
  is required; but a future release that added the flags
  and later reversed the decision would need a deprecation
  period, and this ADR's decision is the initial state.
- The error message's exact wording is a small surface that
  must be maintained. A future change to the message (for
  example, a localization, a reformatting) must keep the
  message informative; the tests assert on the message's
  substring ("mutually exclusive") rather than its full
  text, so a cosmetic change is allowed, but the message's
  substance is pinned.

### 4.3 Neutral

- The exit code 64 (EX_USAGE) is the CLI's convention for
  usage errors (per ADR-018); the rejection uses it for
  consistency with other usage errors. The code is not new;
  it is the same code the pre-parse validator uses for
  `--help <cmd>` and other malformed invocations.
- The error's category (CategoryCLI) is the CLI's category
  for usage errors; the rejection is classified as a CLI
  error, not as an internal error. The classification
  follows from the error's nature (the user's invocation
  is wrong) and from the existing taxonomy (ADR-011).
- The mutual exclusion's interaction with the
  `FORGE_LOG_LEVEL` env var is unaffected: a user who sets
  `FORGE_LOG_LEVEL=debug` and passes `--quiet` gets ERROR
  (the flag's level); the env var is not a flag and does
  not conflict with `--quiet`. The mutual exclusion applies
  only to the two flags, not to the env var.
- The mutual exclusion's interaction with the config file's
  `log.level` field is unaffected: the config file's value
  is a tier below both flags (ADR-024), so the mutual
  exclusion does not apply to it.
- The mutual exclusion's interaction with a future
  `--log-format` flag (which does not exist yet) is
  unaffected; the `--log-format` flag's conflict rules, if
  any, are a separate decision.

---

## 5. Related Requirements

- `FR-CLI-LOG-009` — `--verbose` elevates the log level to
  DEBUG; the requirement is compatible with this decision's
  rejection of the combination with `--quiet`.
- `FR-CLI-LOG-010` — `--quiet` elevates the log level to
  ERROR and suppresses informational output; the requirement
  is compatible with this decision's rejection of the
  combination with `--verbose`.
- `UX-xxx` — the CLI's flag conventions are documented; the
  requirement (to be added when WBS 1.4 completes) is
  established by this decision.

> The following requirements must be added to the register
> when WBS 12.6.1 is complete:

- A new requirement for "`--quiet` and `--verbose` are mutually exclusive" if the existing requirements do not cover it.

---

## 6. Related ADRs

- `ADR-011` — Closed Error Category Taxonomy. Establishes the
  CategoryCLI that classifies the conflict's rejection.
- `ADR-014` — User-Facing Error Format. Establishes the
  format of the error message the rejection renders.
- `ADR-018` — Exit Code Migration and Stability Policy.
  Establishes the exit code 64 (EX_USAGE) the rejection
  returns.
- `ADR-022` — Output and logging are separate architectural
  layers. Establishes the stream the error block goes to
  (stderr).
- `ADR-023` — Four log levels: ERROR, WARN, INFO, DEBUG.
  Establishes the levels the two flags select.
- `ADR-024` — Log level precedence: CLI flag > env var >
  config file > default. Establishes the chain the two
  flags participate in; the chain's tiers 2 and 3 are
  never both reached in a valid invocation, but the order
  is documented for the case that the exclusion is lifted.
- `ADR-028` — The stream contract is enforced at the
  boundary. Establishes the boundary that renders the
  rejection's error block.
- `ADR-030` — `--verbose` is long-only; `-v` is reserved
  for `--version`. Establishes the `--verbose` flag's
  shape.
- `ADR-031` — `--quiet` suppresses diagnostics but never
  suppresses errors. Establishes the `--quiet` flag's
  behavior.

---

## 7. Notes

**Open questions deferred by this ADR:**

- Whether the mutual exclusion should be lifted in a future
  ADR to allow `--verbose` to override `--quiet` (or vice
  versa). The current decision rejects the combination;
  a future ADR could replace the rejection with a
  precedence rule (for example, "verbose wins"). The
  choice would depend on user feedback and on whether the
  rejection proves to be a source of friction in practice.
- Whether the error message should be more specific about
  the two flags' effects (for example, "verbose enables
  DEBUG records; quiet suppresses INFO records; choose
  one"). The current message names the conflict; a future
  ADR could enrich the message if user confusion emerges.
- Whether the mutual exclusion should be enforced by the
  flag parser (a Cobra-level `MarkFlagsMutuallyExclusive`
  call) or by the pre-parse validator (a Go-level check).
  The current implementation uses the validator; the
  parser-level enforcement is a possible alternative, but
  its behavior on `--help` interception is untested. The
  validator's enforcement is the current choice.

**Follow-up decisions anticipated:**

- The `--quiet` flag's per-command behavior (WBS 12.6.1)
  is defined by ADR-031 and by the WBS item's spec; the
  mutual exclusion's interaction with the per-command
  behavior is a consequence of this ADR's decision (the
  rejection happens before any command runs, so no
  command's behavior is affected).
- The `--verbose` flag's per-command behavior (WBS 12.5.1)
  is defined by ADR-030 and by the WBS item's spec; the
  mutual exclusion's interaction with the per-command
  behavior is a consequence of this ADR's decision.
- The error code that classifies the rejection
  (`CodeCLIQuietVerboseConflict` or its equivalent) is a
  member of the CLI's error code registry (ADR-012); its
  addition to the registry is part of the implementation,
  not a separate decision.

**External references:**

- `git --help`, `docker --help`, `kubectl --help` — the
  precedent CLIs whose conflicting flags are rejected with
  a usage error.
- POSIX.1-2017, "Utility Syntax Guidelines" — the guideline
  that a utility should not silently accept conflicting
  options, which favors the rejection.
- `docs/reference/log-levels.md` § 4.5 — the authoritative
  reference for this decision's mutual exclusion rule.
- `internal/cli/flags.go`'s `validateLogFlags` and
  `ErrQuietVerboseConflict` — the implementation of this
  decision.

---

*End of ADR-032.*
