# ADR-008: `forge config` Command Scope

**Status:** Proposed
**Date:** 2026-10-12
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

The Forge CLI is a foundation manager. Its own behaviour is
configurable through a small set of settings: a log level, an output
format, and the path to the configuration file that supplies them. The
settings are resolved from four precedence layers (defaults, file,
environment, and flags) and are consumed by the commands that render
output.

Until this ADR, the CLI offered no way for a user to observe which
settings took effect for a given invocation. A user who set
`FORGE_LOG_LEVEL=debug` had to infer from log output that the variable
had been read. A user who wanted to know which config file Forge had
discovered had to reason about the discovery algorithm. A user who
wanted to debug why a setting had the value it did had to inspect the
environment, the file, and the flags separately. The CLI's
configuration was resolved but not exposed.

A second force is the boundary between CLI configuration and project
configuration. Forge already has a project-level configuration file
(`forge.yaml`, produced by `forge init` and consumed by `forge
validate`). It describes the project the CLI operates on: its
foundation, its policies, its required components. The CLI's own
configuration is a different concern. Mixing the two in one command
would conflate two kinds of data — a project-scoped document and a
user-scoped or environment-scoped setting — with different lifetimes,
different owners, and different visibility rules.

A third force is the risk of leaking secrets. Config files are often
committed to repositories. A CLI command that dumps its configuration
must not include fields that could contain credentials or tokens. The
configuration schema (WBS 8.2.1) is deliberately minimal for this
reason: two value fields and one metadata field. Any command that
displays configuration must preserve this boundary, and must be
future-proof against later additions to the schema (which require an
ADR and must themselves satisfy the no-secrets rule).

The problem this ADR addresses: *what does `forge config` exist to do,
and what is it forbidden from doing?* Without a frozen scope, the
command's surface could grow in later phases in ways that violate the
CLI-scoped / project-scoped boundary, that display secrets, or that
duplicate what other commands already do. The scope must be stated
before the command's identity and output format are frozen.

This ADR relates to WBS 8.1.1 (CLI configuration scope), WBS 8.2.1
(CLI configuration schema), WBS 9.1.1 (config command identity), and
the CLI UX specification § 4.6 and § 4.13.1. The audience is the Forge
Core Team, CLI users who inspect their configuration, and any future
contributor who proposes new fields or subcommands.

---

## 2. Options Considered

### Option A — No `forge config` command; rely on documentation

The CLI's configuration is documented in the CLI UX specification
(`docs/cli-ux-spec.md` § 4.13). A user who wants to know the effective
settings reads the documentation and reasons about the precedence
chain and the discovery algorithm. No command is added.

**Pros:**

- No new command to maintain, no new test surface, no new output
  format to freeze.
- The configuration's behaviour is documented in one place.
- Zero risk of leaking a field into a display; there is no display.

**Cons:**

- The user cannot observe the *effective* configuration. They can
  learn the rules but not apply them to their environment without
  doing the resolution by hand.
- The rules are resolved by the loader in ways that are not always
  obvious from the documentation (for example, which of several
  candidate config files was discovered, or which precedence layer
  set a given value).
- Debugging a configuration issue requires reading the CLI's source
  or reproducing the resolution manually.
- The CLI's own behaviour becomes opaque precisely when
  transparency matters most: when the user's environment or file
  has produced an unexpected result.

### Option B — One combined command that displays both CLI and project configuration

A single command, `forge config`, displays the CLI's configuration
*and* the project's configuration (`forge.yaml`'s contents). The user
gets one view of "everything Forge knows".

**Pros:**

- One command to learn, one output to read.
- The user sees the full picture of Forge's state in one place.
- Reduces the number of commands a user must remember.

**Cons:**

- Conflates two kinds of data with different lifetimes and different
  ownership. CLI configuration is user-scoped or environment-scoped;
  project configuration is repository-scoped and committed.
- Violates the boundary that WBS 8.1.1 drew between
  `internal/config` and `internal/blueprint`. The two packages do
  not import each other; a combined command would either import
  both or duplicate the reading logic.
- A project configuration file can contain fields a user may not
  want echoed in a terminal (for example, in future phases, a
  registry URL with credentials embedded, or a signing key
  reference). Displaying it by default widens the surface for
  accidental disclosure.
- The command's output becomes dependent on which directory the
  user is in, which is surprising: the same invocation produces
  different output in different directories.
- The command's scope becomes harder to freeze: any future addition
  to either configuration type changes the command's output.

### Option C — A dedicated `forge config` command for CLI configuration only

A dedicated command, `forge config`, displays the CLI's resolved
configuration for the current invocation. It shows each field's
effective value, the source of each value (which precedence layer
set it), and the resolved config file path. It is read-only in
Phase 2. It does not display project configuration; that is the
responsibility of `forge explain`.

**Pros:**

- Draws a clear boundary: `forge config` is about the CLI,
  `forge explain` is about the project.
- Preserves the WBS 8.1.1 package boundary. `forge config` reads
  from `internal/config` only; it does not import
  `internal/blueprint`.
- Output is deterministic and directory-independent for a given
  invocation; the same environment and file produce the same output
  regardless of where the user runs the command (except for the
  discovered file path, which is a resolution result and is
  documented as such).
- The scope is freezable: the command displays exactly the schema
  fields, plus the resolved path, plus each field's source. A future
  addition to the schema is a change to the command's output and
  requires an ADR — the boundary is explicit.
- The read-only property in Phase 2 matches the schema's
  no-secrets rule: the command cannot be used to write a value
  into the config file, so it cannot introduce secrets.

**Cons:**

- A user who wants to see both CLI and project configuration
  runs two commands.
- The command's name (`config`) is shorter than its scope
  (`CLI configuration`); the help text and the `Long` description
  must disambiguate.
- A future phase may want to add subcommands (`get`, `set`,
  `unset`). The read-only Phase 2 scope defers that decision.

### Option D — Expose configuration via flags on each command

Every command that reads configuration gains a `--show-config` flag
that displays the effective configuration before running. There is
no dedicated `forge config` command.

**Pros:**

- Configuration is visible in context: a user running `forge
  validate` can see what `forge validate` resolved.
- No new top-level command.

**Cons:**

- Every command's flag surface grows by one flag.
- The display logic is duplicated or shared across commands; the
  shared case is effectively `forge config` with an additional
  invocation.
- The user must remember to add the flag; it is opt-in, so the
  opaque-behaviour problem (Option A's con) persists by default.
- A user who wants to see the configuration without invoking a
  command has no entry point.

---

## 3. Decision

The decision is **Option C**: a dedicated, read-only `forge config`
command that displays the CLI's resolved configuration.

The command's scope is:

- **In scope:** the effective value of each field in the CLI
  configuration schema (`LogLevel`, `OutputFormat`), the source of
  each value (which of the four precedence layers set it), and the
  resolved configuration file path (`SourcePath`, shown as metadata,
  not as a configurable value).
- **Out of scope:** project configuration (`forge.yaml`), any
  project-scoped field, any value from `internal/blueprint`, and
  any writable operation on the CLI configuration.

Option C was chosen over Option A because the documentation alone
does not let a user observe the effective configuration; the CLI's
resolution is stateful and environmental, and the documentation
describes the rules rather than the outcome. Option C was chosen
over Option B because the two configuration types belong to
different packages, have different lifetimes, and carry different
disclosure risks; combining them would violate the WBS 8.1.1
boundary and widen the surface for accidental disclosure. Option C
was chosen over Option D because the flag-per-command approach
duplicates the display logic and makes the configuration
opt-in, which reintroduces the opacity that motivated the
command in the first place.

The chosen scope assumes:

1. **The CLI configuration schema remains small.** If the schema
   grows to include fields whose display in a terminal is
   undesirable (for example, an auth token for a future registry),
   the schema change itself requires an ADR, and that ADR must
   address the display question. The scope of `forge config` is
   therefore bounded by the schema's own boundary.
2. **Project configuration is exposed by a different command.**
   `forge explain` already exists in the specification for that
   purpose. If `forge explain` were removed or renamed, a
   replacement would be needed; `forge config` would not absorb
   its scope.
3. **Read-only is the Phase 2 contract.** A future phase may add
   write operations (`set`, `unset`) under a new ADR. The
   read-only decision is provisional, not permanent.

---

## 4. Consequences

### 4.1 Positive

- Users can observe the effective CLI configuration for the current
  invocation without reasoning about the precedence chain and the
  discovery algorithm by hand.
- The source of each value is visible. A user who wonders why a
  setting has a particular value sees whether it came from a
  default, a file, an environment variable, or a flag.
- The command is a natural diagnostic tool. When a setting does not
  have the expected value, the command's output names the layer
  that produced the value, which localizes the investigation.
- The package boundary from WBS 8.1.1 is preserved. `forge config`
  reads from `internal/config` only. The command does not need to
  know about `internal/blueprint`, and a future change to
  `internal/blueprint` cannot affect `forge config`'s output.
- The read-only property in Phase 2 means the command cannot write
  a value into the configuration file. Combined with the
  schema's no-secrets rule, the command cannot be used to
  introduce a secret into a committed file.
- The command is deterministic given its inputs. Two invocations
  with the same environment, file, and flags produce the same
  output. This makes the command safe to script and safe to
  snapshot in tests.
- The command's scope is small enough to freeze. The identity,
  output format, and flag interactions fit in three follow-on
  ADRs (one per concern) without the scope statement needing to be
  re-litigated each time.

### 4.2 Negative

- A user who wants to see both CLI and project configuration must
  run two commands (`forge config` and `forge explain`). The
  combined view is not available from a single invocation. This is
  the accepted tradeoff for keeping the two concerns separate.
- The command's name (`config`) is shorter than its scope ("CLI
  configuration"). A user who reads only the name may expect the
  command to display the project's `forge.yaml`. The command's
  `Short` and `Long` descriptions must disambiguate, and the
  help output must name the CLI scope explicitly. This is a
  documentation burden the ADR accepts knowingly.
- The command's output cannot include future schema fields that
  are intentionally not displayed (for example, a hypothetical
  token field). A future ADR that adds such a field must also
  decide whether `forge config` should hide it, and if so, how.
  The command's current scope does not have a hiding
  mechanism — it displays every schema field — so a future
  schema change that requires hiding would need to extend the
  scope. This is a deferred decision, not a defect, but it is a
  real constraint on what the schema may add.
- The command is read-only in Phase 2. A user who wants to
  change a value must edit the file, set the environment
  variable, or pass a flag. The convenience of `forge config set
  log_level debug` is deferred. Accepting read-only preserves the
  no-secrets rule trivially; a write operation would require a
  separate ADR and a review of the file's permissions and
  commit-safety.
- The command adds a public surface: its output format is a
  contract with scripts. The contract's freeze (in a follow-on
  ADR) imposes the same review overhead as any other frozen
  format.

### 4.3 Neutral

- The command's name is `config`, not `configuration`. The
  shorter form follows the CLI's naming convention (imperative,
  lowercase, no abbreviation that reduces clarity). The name is
  not changed by this ADR; it was set by the command's identity
  WBS item (WBS 9.1.1).
- The command is registered in the user-facing group in
  `internal/cli/registry.go`. Its position is alphabetical among
  the user-facing commands. This is a registry concern, not a
  scope concern, and is covered by the registry's own
  documentation.
- The command's `Args` is `cobra.NoArgs`. The command takes no
  positional arguments in Phase 2. A future phase that adds
  subcommands (for example, `forge config get <key>`) would
  change this; the change is deferred, not decided.
- The command's `--path` flag is a Phase 2 addition. It is
  documented in a follow-on ADR; the scope statement here does
  not fix the flag's behaviour, only the command's subject.

---

## 5. Related Requirements

The requirements below are drawn from
[`docs/requirements-register.md`](../requirements-register.md).
Requirement IDs that do not yet exist are marked as pending.

- `FR-CLI-006` — `forge config` displays the resolved CLI
  configuration. This ADR establishes the requirement's scope.
- `FR-CLI-007` — `forge config --path` prints only the resolved
  config file path. This ADR's scope statement is the parent
  requirement; the flag's semantics are frozen by a follow-on
  ADR.
- `FR-CFG-002` — the source attribution of each configuration
  value is observable. This ADR requires the command to display
  the attribution.
- `UX-006` — the command's output is one of the two frozen
  formats (`text`, `json`). The specific formats are frozen by a
  follow-on ADR.

The following requirement is proposed but not yet registered:

> The following requirement must be added to the register when
> WBS 1.4 is complete: `FR-CLI-0NN` — `forge config` is read-only
> in Phase 2; write operations require a new ADR.

---

## 6. Related ADRs

- `ADR-004` — root command identity and behavior contracts. The
  `forge config` command inherits the CLI's identity conventions
  (lowercase `Use`, one-line `Short`, multi-paragraph `Long`,
  `Args: cobra.NoArgs` where no args are taken). This ADR does
  not supersede any clause of ADR-004; the two are complementary.
- `ADR-006` — help and CLI UX contracts. The `forge config`
  command's help output follows ADR-006's contract for
  per-command help. This ADR does not supersede any clause of
  ADR-006; the two are complementary.
- `ADR-007` — configuration foundation, precedence, discovery,
  and error taxonomy. This ADR's scope statement is the
  consumer-side counterpart to ADR-007's loader-side definition.
  The two ADRs together define what the CLI's configuration is
  and what a user can observe about it.

---

## 7. Notes

**Deferred decisions.** The following decisions are anticipated
but not made here. Each requires its own ADR.

- **Output format.** The command's text and JSON formats, their
  byte-level rules, and the schema of the JSON output are frozen
  by a follow-on ADR (ADR-009 in the numbering sequence if the
  next available number is 9).
- **Flag interaction.** The `--path` flag's scope and the
  command's interaction with the global `--quiet`, `--verbose`,
  `--config`, and `--format` flags are frozen by a follow-on ADR
  (ADR-010).
- **Write operations.** If a future phase adds `forge config
  set`, `forge config unset`, or similar, the write surface
  requires a new ADR addressing the file's permissions, the
  commit-safety rule, and the schema's no-secrets rule. This ADR
  does not pre-authorize any write operation.
- **Subcommands.** A future `forge config <subcommand>` shape
  (for example, `forge config get <key>`) is anticipated but not
  decided. The command is a leaf in Phase 2.

**Anticipated schema growth.** The CLI configuration schema is
frozen at three fields by WBS 8.1.1 and WBS 8.2.1. A future
addition requires an ADR. That ADR must also state whether the
new field appears in `forge config`'s output, and if not, what
hides it. The command's current scope is "every schema field is
displayed", which is the correct default but is not the only
possible one.

**On the boundary with `forge explain`.** `forge explain` displays
the project's foundation: the blueprint, the template, the
components, the policies, and the provenance. `forge config`
displays the CLI's configuration. The two commands answer
different questions and are documented as such in
`docs/cli-ux-spec.md` § 4.4 and § 4.6. A user who is unsure which
command to run can consult the `Short` line of each in
`forge --help`: "Display resolved CLI configuration" versus
"Explain the current repository's foundation".

**On the read-only property.** Read-only in Phase 2 is not a
permanent property of the command. It is the correct Phase 2
scope because the write surface would introduce questions (file
permissions, commit-safety, secret storage) that are not yet
addressed. The property is provisional; a future ADR may change
it.

**Follow-up.** The three follow-on ADRs (output format, flag
interaction, and any future write operation) are the natural
next steps. This ADR's scope statement is the parent that they
cite.

---

*End of ADR-008.*
