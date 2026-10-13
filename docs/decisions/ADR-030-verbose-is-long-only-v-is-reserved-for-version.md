# ADR-030: `--verbose` is long-only; `-v` is reserved for `--version`

**Status:** Accepted
**Date:** 2026-10-13
**Deciders:** [@thapelomagqazana] (CLI UX Owner)
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

The `--verbose` flag is planned by WBS 12.5.1 to elevate the log
level to DEBUG (per ADR-023) and by WBS 12.6.1's sibling flag
`--quiet` to elevate it to ERROR. When a boolean flag is added, the
question of its short form arises: many CLIs provide a single-letter
alias, and Forge's other flags (the root command's `--version`, the
`config` command's `--path`) already have short forms or are
candidates for them. The short form `-v` is the obvious candidate
for `--verbose` — its first letter matches — and every contributor
who has used `curl -v`, `ssh -v`, or `gcc -v` expects `-v` to mean
"verbose" for at least some tools.

But `-v` is already taken. Cobra (the CLI framework Forge uses, per
ADR-002) registers `-v` as the short form of `--version` when the
root command's `Version` field is non-empty. Forge's root command
sets that field (WBS 5.1.2), which means the current binary already
accepts `forge -v` and interprets it as `forge --version`. The
`--verbose` flag has not yet been registered, and the WBS item that
adds it (WBS 12.5.1) must decide what to do about the collision.
Three shapes are possible: give `-v` to `--verbose` and make
`--version` long-only; keep `-v` for `--version` and make
`--verbose` long-only; or give both flags the same short form and
resolve by position (a bad idea, not considered seriously).

The forces at play are three. First, the convention across the
CLI ecosystem is not uniform. `git -v`, `docker -v`, `kubectl -v`,
and `go version -v` all interpret `-v` as "version"; `curl -v`,
`ssh -v`, `gcc -v`, and `rsync -v` all interpret `-v` as "verbose".
The two conventions are both well-established, and the choice is a
tiebreak rather than a discovery. Second, Forge's existing
behavior — `-v` is `--version` — is already shipped in a binary
that some users may have scripted against. Flipping `-v` to mean
`--verbose` is a breaking change for any script that calls
`forge -v` and expects the version block. Third, the two flags'
audiences differ. `--version` is used rarely (mostly in scripts
that check the installed version); `--verbose` is used
interactively (a user who wants to see what a command did). The
"which flag deserves the short form" question is a question about
which audience types `-v` more often.

The decision relates to ADR-002 (Cobra as the CLI framework,
which determines `-v`'s current registration), ADR-023 (the four
log levels, which the `--verbose` flag's semantics depend on),
ADR-024 (the log level precedence chain, which determines
`--verbose`'s tier), and ADR-031 (the `--quiet` flag, which is
`--verbose`'s sibling). The CLI UX spec's § 4.12 mentions the
`--verbose` flag but does not resolve the short-form collision.
If no decision is made, the collision remains: either `--verbose`
cannot be added (because `-v` is taken), or adding it silently
shadows `--version`, or a contributor resolves the collision
ad-hoc without a written rule.

---

## 2. Options Considered

### Option A — `-v` means `--verbose`; `--version` becomes long-only

`--verbose` receives the short form `-v`. The `--version` flag
loses its short form and remains available as `--version` and as
the `version` subcommand. A script that calls `forge -v` and expects
the version block breaks; the script's author must update to
`forge --version`.

**Pros:**

- `-v` means "verbose" for a majority of the tools that Forge's
  users have used (per `curl`, `ssh`, `gcc`, `rsync`). A user
  with that habit types `forge -v` and gets verbose output, which
  matches their expectation.
- `--verbose` is used interactively, where short forms matter
  most; `--version` is used in scripts, where long forms are
  conventional (a script's `forge --version` is clearer than
  `forge -v`).
- The `version` subcommand already exists as a short alternative
  to `--version`; a user who wants the version block types
  `forge version`, which is short enough to obviate `-v`.
- `--quiet`'s short form (`-q`, planned by ADR-031) is unaffected;
  the two implicating flags have distinct short forms.

**Cons:**

- Flipping `-v`'s meaning is a breaking change for any script
  that calls `forge -v` and expects the version block. The
  binary has shipped with the current behavior, and a script
  that was written against it breaks silently (a script that
  redirects stderr to `/dev/null` and inspects stdout would
  see verbose's absence of stdout output rather than the
  version block).
- The convention "`-v` means version" is the majority for
  Forge's target tools (`git`, `docker`, `kubectl`, `go`).
  The choice inverts the majority convention in favor of the
  minority one.
- The `--version` flag's long-only status is unusual; most
  CLIs provide a short form for the version flag. A user who
  types `forge -V` (a common alternative) finds it unregistered.
- Cobra's default registration is `-v` for `--version` when
  `Version` is set; to override it, `newRootCmd` must clear the
  shorthand explicitly or register `--verbose` with a different
  letter. The override is a small amount of code but is a
  deviation from the framework's default.

### Option B — `-v` means `--version`; `--verbose` is long-only

`--version` keeps the short form `-v` that Cobra has registered.
`--verbose` is registered without a short form. The user types
`forge --verbose` for verbose output and `forge -v` or
`forge --version` for the version block. A script that calls
`forge -v` and expects the version block continues to work.

**Pros:**

- The current behavior is preserved: `forge -v` continues to
  print the version block. No script that was written against
  the existing binary breaks.
- The convention "`-v` means version" is the majority for
  Forge's target tools (`git`, `docker`, `kubectl`, `go`). The
  choice aligns with the majority, which is what most users
  will expect.
- `--verbose` is used interactively, where typing `--verbose`
  in full is a modest cost (the user is reading the output as
  it appears; the extra keystrokes are amortized over the
  time they spend reading). `--version` is used in scripts
  where `-v` is a common shorthand.
- The `--quiet` flag's short form (`-q`) is unaffected; the
  two implicating flags' short forms (or lack thereof) are
  independent decisions.
- Cobra's default registration is preserved; `newRootCmd` does
  not need to override the framework's behavior.

**Cons:**

- `--verbose` has no short form, which is unusual for a flag
  that users type frequently. A user who types `forge -v` and
  expects verbose output gets the version block instead, which
  is a minor surprise (though the version block is
  informative).
- A user with the `curl -v` habit types `forge -v` and gets
  the version block; the user must learn `forge --verbose`.
  The learning cost is small but is not zero.
- The two flags' audiences (interactive users and scripts) are
  served differently; a user who wants a short form for
  `--verbose` has no way to get one without an ADR.

### Option C — Give both flags a short form: `-v` means `--version`, `-V` means `--verbose`

`-v` remains `--version`; `--verbose` receives the short form
`-V`. The two flags are distinguished by case, matching a pattern
used by `grep -V` (version) and `grep -v` (invert), though `grep`'s
case distinction is the opposite of Forge's.

**Pros:**

- Both flags have short forms; no flag is long-only.
- The pattern "lowercase for one meaning, uppercase for another"
  is familiar from `grep`, `less`, and other tools.

**Cons:**

- `-V` for verbose is unusual; the majority of CLIs do not use
  `-V` for verbose, and a user who types it is as likely to be
  surprised as a user who types `-v` and gets version.
- The case distinction is easy to mistype and hard to
  remember; a user who types `-v` when they meant `-V` gets
  the version block, which is the same surprise Option B has,
  with no benefit.
- The pattern "lowercase for one meaning, uppercase for
  another" is not universal; `grep`'s `-v`/`-V` distinction
  is the opposite of Forge's, and the memory load is
  increased by the divergence.
- The convention `-V` for version is common in other tools
  (for example, `gcc -v` and `gcc -V` both print version
  information); assigning `-V` to verbose would break that
  convention.

### Option D — Give both flags a short form: `-v` means `--verbose`, `-V` means `--version`

`--verbose` receives `-v`; `--version` receives `-V`. A user
who types `forge -v` gets verbose output; a user who types
`forge -V` gets the version block.

**Pros:**

- `-v` means verbose for the `curl`/`ssh`/`gcc` audience, which
  is the majority of Forge's users who are accustomed to a
  short form for verbose.
- `-V` for version matches the `gcc -V`, `grep -V`, and
  `less -V` conventions, which use uppercase V for version.
- Both flags have short forms; no flag is long-only.

**Cons:**

- Flipping `-v`'s meaning is a breaking change for any script
  that calls `forge -v` and expects the version block (the
  same breaking change as Option A).
- The convention "`-v` means version" is the majority for
  Forge's target tools (`git`, `docker`, `kubectl`, `go`);
  the choice inverts the majority.
- The `-V` for version convention is not as universal as
  Option B's `-v` for version; a user who expects `-V` for
  verbose (a minority but nonzero group) is surprised.
- Cobra's default registration (`-v` for `--version`) must be
  overridden, which is a deviation from the framework's
  behavior.

---

## 3. Decision

**Forge adopts Option B: `-v` remains the short form of
`--version`; `--verbose` is registered long-only.** The user
types `forge --verbose` for verbose output and `forge -v` or
`forge --version` for the version block. The collision is
resolved in favor of the existing behavior and the majority
convention (`git`, `docker`, `kubectl`, `go`), and the cost is
borne by `--verbose`, whose absence of a short form is a minor
inconvenience for interactive users.

The reasoning that tipped the balance was the existing
behavior's preservation. The current binary accepts `forge -v`
and prints the version block; a script that was written against
the binary relies on that behavior, whether its author realized
it or not. Flipping `-v` to mean `--verbose` would break the
script silently (the script would see verbose output where it
expected the version block, and a script that redirects stderr
to `/dev/null` would see no stdout output from `-v`, since
verbose output goes to stderr). The cost of the break is
asymmetric: a script that breaks is a script that a user
deployed and trusts, and the break's repair requires the user
to notice and update the script. The cost of `--verbose` being
long-only is a few extra keystrokes for an interactive user,
which the user amortizes over the time they spend reading the
output. The choice between the two costs is not a tie; the
existing behavior's preservation is the smaller cost.

The reasoning against Option A and Option D (both flip `-v` to
verbose) was the majority convention's alignment. Forge's
target tools — the tools its users have used before — are
`git`, `docker`, `kubectl`, and `go`, all of which interpret
`-v` as version. The `curl`/`ssh`/`gcc`/`rsync` group
interprets `-v` as verbose, but Forge is a project-foundation
manager rather than a network client or a compiler, and its
audience's prior CLI experience is more likely to be with the
version-interpreting group than with the verbose-interpreting
group. The majority convention's alignment is the correct
default; Forge's divergence would require a positive reason,
and the current decision does not have one.

The reasoning against Option C (`-V` for verbose) was the
case-distinction's cost. `-V` for verbose is unusual; the
convention `-V` for version is more common (per `gcc`,
`grep`, `less`), and a user who types `-V` expecting the
version block would be surprised by verbose output. The
case distinction is easy to mistype and hard to remember; the
memory load is increased by the divergence from the more
common convention. Option C's benefit (both flags have short
forms) is small; the cost (the case distinction's confusion)
is larger.

The tradeoffs accepted are the ones Option B lists as cons.
The lack of a short form for `--verbose` is accepted because
`--verbose`'s audience is interactive and the flag is typed in
full as a matter of course; a user who types `-v` and gets the
version block is mildly surprised, but the version block is
informative and the user quickly learns the correct spelling.
The asymmetry between `--verbose`'s audience (interactive
users, who type long forms) and `--version`'s audience
(scripts, which use short forms) is not a defect; it is the
shape the flags' audiences actually have.

The decision is provisional in one respect: the choice
preserves the current behavior, which is a consequence of
Cobra's default registration and the `root.Version` field's
presence. A future ADR that changes Cobra's behavior (for
example, a switch to a different framework) could change the
`-v` registration; the change would require revisiting this
decision. Until then, the choice is stable.

---

## 4. Consequences

### 4.1 Positive

- The current behavior is preserved: `forge -v` continues to
  print the version block. No script that was written against
  the binary breaks.
- The convention "`-v` means version" is the majority for
  Forge's target tools (`git`, `docker`, `kubectl`, `go`).
  Forge's behavior aligns with the majority, which is what
  most users will expect.
- The `version` subcommand exists as a short alternative to
  `--version`; a user who wants the version block types
  `forge version`, which is short enough to obviate `-v`.
- Cobra's default registration is preserved; `newRootCmd`
  does not need to override the framework's behavior.
- The decision is documented in
  `docs/reference/verbose-mode.md` § 2, and it is pinned by
  two tests (`TestContract_VerboseHasNoShortForm` in
  `internal/cli/flags_test.go` and
  `TestVersionBehaviour_VerboseHasNoShortFlag` in
  `internal/cli/root_test.go`). A future contributor who
  wants to flip the decision must update the tests, which
  forces the change to be deliberate.

### 4.2 Negative

- `--verbose` has no short form, which is unusual for a flag
  that users type frequently. A user who types `forge -v`
  and expects verbose output gets the version block instead.
  The learning cost is small but is not zero.
- The two flags' audiences (interactive users and scripts)
  are served differently; a user who wants a short form for
  `--verbose` has no way to get one without an ADR. The
  asymmetry is documented in `verbose-mode.md` § 2, but it
  is a real limitation.
- The decision inverts the `curl`/`ssh`/`gcc`/`rsync`
  convention's expectation. A user who has used those tools
  and is accustomed to `-v` meaning verbose is surprised by
  Forge's behavior. The surprise is minor (the version block
  is informative), but it is a surprise.
- The `--quiet` flag's short form (`-q`) is unaffected by
  this decision, but the two implicating flags' short forms
  are now asymmetric: `-q` exists, `-v` does not (for
  `--verbose`). A user who wants both flags to have short
  forms is disappointed. The asymmetry is accepted; the
  `-q` short form is independent.

### 4.3 Neutral

- The `-v` short form's registration is a consequence of
  Cobra's behavior, which is a consequence of ADR-002's
  framework choice. The decision documents the consequence
  rather than re-deciding the framework choice.
- The `--verbose` flag's help text does not mention the
  absence of a short form. A user who reads `forge --help`
  sees `--verbose` listed without a short form, which is
  the correct presentation; the absence is not a defect.
- The `forge version` subcommand's existence is not part of
  this decision; the subcommand was added by WBS 5.1.2 for
  reasons independent of the short form's collision. The
  subcommand's presence makes the collision's resolution
  easier (a user who wants the version block has a short
  alternative), but the subcommand is not the reason for
  the decision.
- The `--verbose` flag's name is documented in
  `docs/reference/verbose-mode.md` § 2; the reference doc's
  `-v` reservation section is the authoritative statement
  of the decision.

---

## 5. Related Requirements

- `FR-CLI-LOG-009` — `--verbose` elevates the log level to
  DEBUG; the requirement is compatible with this decision's
  long-only registration.
- `FR-CLI-OUTPUT-001` — product output goes to stdout; the
  `--version` flag's output (the version block) is product
  output, and the `-v` short form is the version flag's, not
  the verbose flag's.
- `UX-xxx` — the CLI's flag conventions are documented; the
  requirement (to be added when WBS 1.4 completes) is
  established by this decision.

> The following requirements must be added to the register
> when WBS 12.5.1 is complete:

- `FR-CLI-LOG-009` (if not already present)
- A new requirement for "the `-v` short form is reserved for `--version`" if the existing requirements do not cover it.

---

## 6. Related ADRs

- `ADR-002` — Choose Cobra as the CLI Framework. Establishes
  the framework whose default registration this decision
  preserves.
- `ADR-005` — Version Command Structure, Output Formats, and
  Stability Guarantees. Establishes the `--version` flag and
  the `version` subcommand whose interaction this decision
  clarifies.
- `ADR-023` — Four log levels: ERROR, WARN, INFO, DEBUG.
  Establishes the levels `--verbose` elevates to.
- `ADR-024` — Log level precedence: CLI flag > env var >
  config file > default. Establishes the chain `--verbose`
  participates in.
- `ADR-031` (expected) — `--quiet` suppresses diagnostics
  but never suppresses errors. Will define the sibling flag
  whose short form (`-q`) is independent of this decision.

---

## 7. Notes

**Open questions deferred by this ADR:**

- Whether a future `--verbose` short form should be
  reconsidered if a new letter becomes available (for
  example, if `--version`'s short form is changed by a
  future ADR). The current decision does not preclude a
  future change; it records the current behavior.
- Whether the `--verbose` flag's absence of a short form
  should be mentioned in `forge --help`'s output as an
  explicit note ("long-only; `-v` is `--version`"). The
  current help text lists the flag without a short form,
  which is the standard Cobra presentation; a future
  ADR could add a note if user confusion emerges.

**Follow-up decisions anticipated:**

- The `--verbose` flag's registration (WBS 12.5.1) is a
  direct consequence of this ADR; the registration
  (`cmd.PersistentFlags().Bool("verbose", false, ...)`)
  has no short form.
- The tests that pin the decision
  (`TestContract_VerboseHasNoShortForm`,
  `TestVersionBehaviour_VerboseHasNoShortFlag`) are part
  of WBS 12.5.1's test suite.
- The `docs/reference/verbose-mode.md` § 2 section is the
  authoritative reference for the decision.

**External references:**

- `git --help`, `docker --help`, `kubectl --help`,
  `go help` — the precedent CLIs whose `-v` means version.
- `curl --help`, `ssh --help`, `gcc --help`, `rsync
  --help` — the precedent CLIs whose `-v` means verbose.
- POSIX.1-2017, "Utility Syntax Guidelines" — the
  guideline that a utility's options should be consistent
  with historical practice, which favors the majority
  convention.
- Cobra's documentation on `root.Version` and the `-v`
  shorthand — the framework behavior this decision
  preserves.
- `docs/reference/verbose-mode.md` § 2 — the authoritative
  statement of this decision.

---

*End of ADR-030.*
