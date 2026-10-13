# ADR-031: `--quiet` suppresses diagnostics but never suppresses errors

**Status:** Accepted
**Date:** 2026-10-13
**Deciders:** [@thapelomagqazana] (CLI UX Owner)
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

The `--quiet` flag is planned by WBS 12.6.1 as a persistent global
flag that suppresses informational output. Its purpose is the
opposite of `--verbose`'s (per ADR-030, `--verbose` is long-only and
`-v` is reserved for `--version`): `--verbose` elevates the log
level to DEBUG (per ADR-023), and `--quiet` elevates it to ERROR.
The two flags share a family and an intent — they tune the volume of
diagnostics — but their shapes are not symmetric. `--verbose` adds
diagnostics; `--quiet` removes them. The decision this ADR must make
is narrower than "what does `--quiet` do in general" (that is
ADR-023's territory): it must decide *what `--quiet` does to the
error block*.

The error block is the user-facing explanation of a failure. It is
produced by `internal/forgeerr` (per ADR-014) and written to stderr
by the exit boundary (per ADR-022 and ADR-028). It has a frozen
format (ADR-016), a code from the registry (ADR-012), an optional
context map, and an optional remediation. It is the only surface on
which Forge tells a user *why* a command failed. When a command exits
non-zero, the error block is what a user reads to decide what to do
next; when a command exits non-zero in a script, the error block is
what the script's author reads in the CI log to diagnose the failure.
Suppressing the error block is a larger intervention than suppressing
diagnostics: it removes the user's explanation of the failure, not
just the failure's contextual noise.

The forces at play are three. First, the flag's name is misleading.
A user who types `forge --quiet version` may expect "no output at
all"; a user who types `forge --quiet check` (per a future WBS)
may expect "no per-check lines, just the exit code"; a user who
types `forge --quiet` on a failing command may or may not expect
the error block. The flag's semantics must be defined precisely
enough that the user knows what to expect. Second, the error block's
suppression interacts with the stream contract. ADR-021 requires
that under `--format json`, the error envelope goes to stdout (as
the command's result in JSON mode). Suppressing the human error
block on stderr does not suppress the envelope on stdout; a script
that reads the envelope expects it regardless of `--quiet`. The
interaction is a rule that this ADR must state. Third, the exit
code is unaffected by `--quiet`. The flag suppresses output, not
the process's exit status; a failing command exits non-zero with
or without `--quiet`. The exit code is the machine consumer's
primary signal, and it must not be suppressed by a flag whose
purpose is to reduce noise.

The decision relates to ADR-014 (the user-facing error format),
ADR-021 (the JSON error envelope), ADR-022 (the stream contract),
ADR-023 (the level vocabulary), ADR-024 (the precedence chain),
ADR-028 (the boundary that writes the error block), and ADR-030
(the `--verbose` flag's shape). The CLI UX spec's § 4.12 mentions
`--quiet`'s interaction with other flags but does not define its
effect on the error block. If no decision is made, a user who runs
`forge --quiet` on a failing command cannot predict whether they
will see the error block, and a future contributor who implements
the flag's suppression has no rule to follow.

---

## 2. Options Considered

### Option A — `--quiet` suppresses everything, including the error block

`--quiet` suppresses all output to stderr and stdout except the
exit code. On a failing command, the error block is suppressed;
the user sees no explanation of the failure. The exit code is the
only signal.

**Pros:**

- The flag's name ("quiet") matches its behavior: no output.
- A CI script that redirects stderr to `/dev/null` and inspects
  the exit code sees no difference between `--quiet` and no flag;
  the flag is a no-op when stderr is already discarded, which is
  consistent.
- The behavior is simple: `--quiet` suppresses everything, and
  there is no exception to remember.

**Cons:**

- The user loses the error block on a failure. A user who runs
  `forge --quiet` on a command that fails sees a non-zero exit
  code and no explanation; the user must re-run without
  `--quiet` to diagnose. The re-run costs time and attention,
  and the failure's context may be non-reproducible (a race
  condition, a transient network error, a state that changed).
- The error block is not "diagnostic noise"; it is the user's
  explanation of the failure. Suppressing it treats the
  explanation as if it were the noise, which is a category
  error. The flag's purpose is to reduce the volume of *informational*
  output, not to hide the *failure's* explanation.
- A CI script that runs `forge --quiet check` and sees a
  non-zero exit code has no way to log the failure's message in
  its output; the CI log's only signal is the exit code, and a
  future debugger of the failure cannot see what Forge said.
- The `--format json` envelope on stdout is suppressed or not
  depending on the implementation; if suppressed, a machine
  consumer that reads the envelope loses the failure's data;
  if not suppressed, the "no output at all" promise is
  violated. Either choice is inconsistent with the flag's
  name.
- A user who wants the error block but no other output has no
  way to express the preference; the flag is all-or-nothing.

### Option B — `--quiet` suppresses diagnostics (INFO and DEBUG records) and informational output, but never suppresses the error block

`--quiet` elevates the log level to ERROR (per ADR-023 and
ADR-024), suppresses the log records at INFO and DEBUG levels, and
suppresses per-command informational output (for example,
`forge check`'s per-check lines, per a future WBS). The error block
is **not** suppressed: when a command fails, the error block is
written to stderr and the JSON envelope (under `--format json`) is
written to stdout. The exit code is unaffected.

**Pros:**

- The user retains the failure's explanation. A user who runs
  `forge --quiet` on a failing command sees the error block on
  stderr and knows what failed and what to do.
- The error block is treated as the failure's signal, not as
  diagnostic noise. The flag's purpose (suppress informational
  output) is honored; the failure's explanation is not
  collateral damage.
- A CI script that runs `forge --quiet check` and captures
  stderr sees the error block in its log; a future debugger of
  the failure reads the same message the user would have seen.
- The `--format json` envelope on stdout is preserved. A
  machine consumer that reads the envelope receives the
  failure's data; the flag's effect on stdout is "no
  informational output," not "no envelope."
- The behavior matches the `--quiet` semantics of `git`
  (which suppresses informational output but not errors),
  `docker` (same), and `kubectl` (same). The precedent is the
  majority convention.
- The flag's interaction with `--verbose` (ADR-030) is
  symmetric: `--verbose` adds DEBUG and INFO records;
  `--quiet` removes INFO and DEBUG records. Both flags leave
  the error block untouched.

**Cons:**

- The flag's name ("quiet") is not a perfect match for its
  behavior: a user who expects "no output at all" is
  surprised by the error block. The surprise is minor (the
  error block is informative), but it exists.
- The flag's behavior is more complex than Option A's:
  the user must remember that "quiet" means "quiet except for
  errors," not "silent." The rule is documented in
  `docs/reference/quiet-mode.md` (planned by WBS 12.6.1),
  but the documentation is additional surface.
- A user who wants a fully silent run (no output at all,
  including errors) has no flag for it. The user's
  alternative is to redirect stderr to `/dev/null`; the
  redirection is a shell feature, not a Forge feature, and
  the user's intent ("I know it failed; I do not care why")
  is expressed at the shell level rather than at the CLI
  level. This is a defensible position (Forge should not
  provide a flag for a preference that the shell already
  expresses), but it is a limitation.

### Option C — `--quiet` suppresses diagnostics and the error block, but not the JSON envelope

Same as Option A, except that the JSON error envelope (under
`--format json`) is preserved. The human error block is
suppressed; the envelope is not.

**Pros:**

- A machine consumer that reads the envelope is unaffected by
  `--quiet`; the envelope is the command's result in JSON
  mode, and the result is not "diagnostic noise."
- The behavior is consistent with the rule "the envelope is
  the result" (ADR-021), which is the same rule that governs
  the success path's envelope under `--quiet`.

**Cons:**

- A human user who runs `forge --quiet` on a failing command
  sees no error block (the block is suppressed) and no
  envelope (the human user is not reading JSON). The user's
  only signal is the exit code, which is Option A's defect:
  the user must re-run without `--quiet` to diagnose.
- The asymmetry between the human mode and the JSON mode is
  confusing: a user who toggles `--format json` on and off
  sees different output with `--quiet`, and the difference
  is not explained by the flag's name.
- The rule "`--quiet` suppresses everything except the
  envelope" is a special case that the user must learn;
  the flag's name does not prepare them for it.

### Option D — `--quiet` suppresses diagnostics, informational output, and the error block; the exit code is the only signal

Same as Option A, with the exit code as the only signal. No
exception for the JSON envelope.

**Pros:**

- The flag's name is a perfect match: no output at all.
- The behavior is simple: the exit code is the only signal.
- The JSON envelope's suppression is consistent with the
  human error block's suppression; a machine consumer that
  wants the envelope does not pass `--quiet`.

**Cons:**

- All of Option A's cons, plus the loss of the JSON envelope.
  A machine consumer that passes `--quiet` and expects the
  envelope is surprised; the flag's effect on stdout is
  stronger than its effect on stderr under the other
  options.
- The "result" semantics of the JSON envelope (ADR-021) are
  violated: the envelope is the command's result in JSON
  mode, and a flag that suppresses informational output
  should not suppress the result.
- A script that runs `forge --quiet --format json check` and
  reads the envelope finds it missing; the script's author
  must know that `--quiet` and `--format json` are
  incompatible for this purpose, which is a rule that is not
  in the flag's name.

---

## 3. Decision

**Forge adopts Option B: `--quiet` elevates the log level to ERROR
(per ADR-023 and ADR-024), suppresses INFO and DEBUG log records,
and suppresses per-command informational output, but never
suppresses the error block and never suppresses the JSON error
envelope.** When a command fails under `--quiet`, the error block
is written to stderr (in text mode) and the JSON envelope is
written to stdout (in JSON mode), exactly as it would be without
`--quiet`. The exit code is unaffected by the flag: a failing
command exits non-zero with or without `--quiet`.

The per-command informational output that `--quiet` suppresses is
defined by each command's own semantics, and its details are the
subject of WBS 12.6.1's per-command quiet behavior spec. For
`forge check` (a future command), `--quiet` suppresses the
per-check lines and leaves the exit code as the signal; for
`forge version`, `--quiet` suppresses nothing (the version block
is the command's result, not informational output); for a future
`forge new`, `--quiet` suppresses progress messages and leaves the
created path. The rule that ties these together is: `--quiet`
suppresses output whose absence does not prevent the user from
acting on the result; it does not suppress the result itself or the
explanation of a failure.

The reasoning that tipped the balance was the error block's role.
The block is not "output"; it is the user's explanation of the
failure. A user who runs `forge --quiet` on a failing command has
asked Forge to be quieter about its *success*; the user has not
asked Forge to hide its *failure*. The two are different requests,
and the flag honors the first without honoring the second. Option
A's all-or-nothing behavior conflates the two; Option C's
JSON-only exception is a special case that the flag's name does
not prepare the user for; Option D's loss of the JSON envelope
violates the envelope's role as the command's result. Option B's
asymmetric suppression is the correct shape: the flag removes
informational output, not the failure's signal.

The reasoning against Option A's simplicity argument was the cost
of the re-run. A user who suppresses the error block and sees only
an exit code must re-run the command without `--quiet` to diagnose
the failure. The re-run is not free: the command's side effects
may have already occurred (a file was created, a network request
was made, a state was modified), and re-running may not reproduce
the failure (a transient condition may have passed). The error
block's presence avoids the re-run; its absence forces it. The
simplicity of "no output at all" is not worth the user's time.

The reasoning against Option C's JSON exception was the flag's
name. A user who toggles `--format json` on and off sees different
output under `--quiet`, and the difference is not explained by the
flag's name. The user's mental model of `--quiet` is "quieter
output"; the JSON envelope's presence in JSON mode but not in
text mode is a rule that the user must learn from the
documentation. Option B's uniform suppression of informational
output (across both modes) and uniform preservation of the error
signal (across both modes) is the simpler mental model: `--quiet`
does not change *what* is preserved, only *how much of the
informational output* is suppressed.

The tradeoffs accepted are the ones Option B lists as cons. The
flag's name is not a perfect match for its behavior; the user
learns from the documentation that `--quiet` means "quiet except
for errors." A user who wants a fully silent run uses a shell
redirection; Forge does not provide a flag for a preference that
the shell already expresses. The per-command quiet behavior is a
per-command concern; the rule that ties the commands together is
documented in `docs/reference/quiet-mode.md` (planned by WBS
12.6.1), and the commands' individual behaviors are documented in
their own help output.

The decision is provisional in one respect: the per-command quiet
behavior spec (WBS 12.6.1) will define the exact informational
output that each command suppresses. The general rule
("informational output that does not prevent the user from acting
on the result") is this ADR's; the per-command specifics are the
WBS item's. If a future command's behavior diverges from the rule
in a way that requires an exception, the exception is documented
in the WBS item's spec and, if it warrants an ADR, recorded as a
new ADR that cites this one.

---

## 4. Consequences

### 4.1 Positive

- The user retains the failure's explanation. A user who runs
  `forge --quiet` on a failing command sees the error block on
  stderr and knows what failed and what to do.
- The error block is treated as the failure's signal, not as
  diagnostic noise. The flag's purpose (suppress informational
  output) is honored; the failure's explanation is not
  collateral damage.
- A CI script that runs `forge --quiet check` and captures
  stderr sees the error block in its log; a future debugger of
  the failure reads the same message the user would have seen.
  The CI log is complete, not truncated by `--quiet`.
- The `--format json` envelope on stdout is preserved. A
  machine consumer that reads the envelope receives the
  failure's data; the flag's effect on stdout is "no
  informational output," not "no envelope."
- The behavior matches the `--quiet` semantics of `git`,
  `docker`, and `kubectl`. The precedent is the majority
  convention, and a user with prior CLI experience finds
  Forge's behavior predictable.
- The flag's interaction with `--verbose` (ADR-030) is
  symmetric: `--verbose` adds DEBUG and INFO records;
  `--quiet` removes INFO and DEBUG records. Both flags leave
  the error block untouched, which means the block's
  presence is a property of the failure, not of the flag's
  combination.
- The exit code is unaffected by `--quiet`. A failing command
  exits non-zero; a successful command exits zero. The exit
  code's stability (ADR-018) is preserved.

### 4.2 Negative

- The flag's name ("quiet") is not a perfect match for its
  behavior: a user who expects "no output at all" is
  surprised by the error block. The surprise is minor (the
  error block is informative), but it exists. The
  documentation must explain the exception; the user who
  reads the documentation learns the rule.
- The flag's behavior is more complex than Option A's. The
  user must remember that "quiet" means "quiet except for
  errors," not "silent." The rule is documented in
  `docs/reference/quiet-mode.md`, but the documentation is
  additional surface.
- A user who wants a fully silent run (no output at all,
  including errors) has no flag for it. The user's
  alternative is to redirect stderr to `/dev/null`; the
  redirection is a shell feature, and the user's intent ("I
  know it failed; I do not care why") is expressed at the
  shell level. The limitation is accepted; Forge does not
  provide a flag for a preference that the shell expresses.
- The per-command quiet behavior is a per-command concern;
  the general rule's application to each command is
  documented in the WBS item's spec, and a user who wants to
  know what a specific command suppresses must read that
  command's help output or the reference doc. The
  documentation's surface is larger than a single rule's
  would be.
- The `--format json` envelope's preservation under
  `--quiet` is a rule that a user who toggles `--format
  json` on and off must learn. The rule is the correct one
  (the envelope is the result), but it is a rule; the
  flag's name does not prepare the user for it.

### 4.3 Neutral

- The `--quiet` flag's short form (`-q`, planned by WBS
  12.6.1) is independent of this decision. The short form's
  presence is a UX choice; the flag's behavior is this
  ADR's subject.
- The `--quiet` flag's interaction with the `FORGE_LOG_LEVEL`
  env var (ADR-024) is defined by the precedence chain:
  `--quiet` (a flag) outranks the env var, so a user who sets
  `FORGE_LOG_LEVEL=debug` and passes `--quiet` gets ERROR
  (the flag's level). The interaction is a consequence of
  ADR-024's chain, not of this ADR's decision.
- The `--quiet` flag's interaction with the config file's
  `log.level` field (ADR-024's tier 5) is defined by the
  chain: `--quiet` outranks the config file. A user who sets
  `log.level: debug` in the config file and passes `--quiet`
  gets ERROR.
- The `--quiet` flag's interaction with `--verbose` is the
  subject of ADR-032 (expected), which will define the two
  flags' mutual exclusion. This ADR's decision (the error
  block's preservation) is independent of the mutual
  exclusion's decision (whether the two flags can be
  combined).
- The per-command quiet behavior spec (WBS 12.6.1) is the
  WBS item's output; its exact content is not this ADR's
  subject, but its general rule ("suppress informational
  output that does not prevent the user from acting on the
  result") is.

---

## 5. Related Requirements

- `FR-CLI-LOG-010` — `--quiet` elevates the log level to ERROR
  and suppresses diagnostics; the requirement is established
  by this decision (with ADR-023 and ADR-024).
- `FR-CLI-OUTPUT-001` — product output goes to stdout; the
  requirement is compatible with this decision's preservation
  of the JSON envelope.
- `FR-CLI-LOG-001` — diagnostics go to stderr; the error
  block's presence on stderr under `--quiet` is a consequence
  of this decision.
- `UX-xxx` — the CLI's flag conventions are documented; the
  requirement (to be added when WBS 1.4 completes) is
  established by this decision.

> The following requirements must be added to the register
> when WBS 12.6.1 is complete:

- `FR-CLI-LOG-010` (if not already present)
- A new requirement for "the error block is not suppressed by `--quiet`" if the existing requirements do not cover it.

---

## 6. Related ADRs

- `ADR-014` — User-Facing Error Format. Establishes the error
  block's format; this decision preserves the block under
  `--quiet`.
- `ADR-016` — Error Redaction Policy. Establishes the
  redaction rules that apply to the error block; this
  decision does not affect them.
- `ADR-018` — Exit Code Migration and Stability Policy.
  Establishes the exit codes' stability; this decision
  preserves the codes under `--quiet`.
- `ADR-021` — JSON Error Envelope for `--format json`.
  Establishes the envelope's role as the command's result;
  this decision preserves the envelope under `--quiet`.
- `ADR-022` — Output and logging are separate architectural
  layers. Establishes the stream contract this decision
  respects.
- `ADR-023` — Four log levels: ERROR, WARN, INFO, DEBUG.
  Establishes the levels `--quiet` filters.
- `ADR-024` — Log level precedence: CLI flag > env var >
  config file > default. Establishes the chain `--quiet`
  participates in.
- `ADR-028` — The stream contract is enforced at the
  boundary. Establishes the boundary that writes the error
  block under `--quiet`.
- `ADR-030` — `--verbose` is long-only; `-v` is reserved
  for `--version`. Establishes the sibling flag whose
  behavior is symmetric with `--quiet`'s.
- `ADR-032` (expected) — `--quiet` and `--verbose` are
  mutually exclusive. Will define the two flags'
  combination; this ADR's decision (the error block's
  preservation) is independent of it.

---

## 7. Notes

**Open questions deferred by this ADR:**

- Whether `--quiet` should have a short form (`-q`). The
  short form's presence is a UX choice (WBS 12.6.1's
  subject); the flag's behavior is this ADR's subject. The
  two are independent.
- Whether the `--quiet` flag's per-command behavior should
  be documented in each command's `--help` output, in the
  reference doc, or both. The current plan (WBS 12.6.1) is
  to document the general rule in
  `docs/reference/quiet-mode.md` and the per-command
  specifics in the commands' help output. A future ADR
  could refine the documentation's placement.
- Whether the `--quiet` flag's suppression of informational
  output should extend to the `forge config` command's
  output. The `forge config` command's output is the
  command's result, not informational output; `--quiet`
  should not suppress it. The rule is consistent with
  this ADR's general rule ("suppress informational output
  that does not prevent the user from acting on the
  result"), but the specific command's behavior is WBS
  12.6.1's subject.

**Follow-up decisions anticipated:**

- The per-command quiet behavior spec (WBS 12.6.1) is the
  direct consequence of this ADR; the spec's content defines
  which informational output each command suppresses.
- The `docs/reference/quiet-mode.md` reference doc is WBS
  12.6.1's output; it is the authoritative statement of
  this ADR's details and of the per-command behavior.
- The `--quiet` flag's interaction with `--verbose` is
  ADR-032's subject; this ADR's decision is a prerequisite
  for ADR-032 (the error block's preservation is part of
  the flags' behavior, and the mutual exclusion's rules
  must account for it).
- The `--quiet` flag's interaction with the JSON envelope
  is a consequence of ADR-021 and this ADR; no separate
  decision is required.

**External references:**

- `git --help`, `docker --help`, `kubectl --help` — the
  precedent CLIs whose `--quiet` suppresses informational
  output but not errors.
- POSIX.1-2017 §2.5, "Standard I/O Streams" — the
  normative source for the stdout/stderr split, which this
  decision respects.
- The Twelve-Factor App, §XI, "Logs" — the guidance that
  informs the logger's destination and the diagnostics'
  role.
- `docs/reference/quiet-mode.md` (expected) — the
  authoritative reference for this ADR's details.

---

*End of ADR-031.*
