# ADR-004: Root Command Identity and Behavior Contracts

**Status:** Proposed
**Date:** 2026-10-10
**Deciders:** @thapelomagqazana
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

The root command is the first thing every user of Forge encounters. It is the entry point of the CLI: the command that parses global flags, prints help when invoked with no arguments, dispatches to every subcommand, and defines the identity of the tool as a whole. Unlike a subcommand, whose behavior is confined to a single use case, the root command's behavior is the CLI's contract with its users at the most general level. Every invocation flows through it.

By the end of Phase 1, Forge's CLI had grown organically. The root command accumulated the identity strings that identify the tool (its name, its usage line, its short description, its long description), the version flag that exposes the build metadata, the help system that answers a user's question of "what can I do?", the no-args behavior that answers "what should I do first?", the unknown-command behavior that rejects malformed invocations, and the three global flags (`--verbose`, `--quiet`, `--config`) that initialize cross-cutting concerns like logging and configuration. Each of these was introduced by a distinct WBS item, and each was documented and tested in isolation. But there was no single document that stated what the root command, as a whole, guarantees to its users, and no single decision that froze the guarantees against future drift.

The absence of a single decision creates two concrete risks. First, a future contributor who wants to change one aspect of the root command — say, add a fourth global flag, or reword the short description, or extend the help output — cannot tell which changes are permitted and which require a deliberate discussion. The WBS items and the specifications each describe a facet, but the facets are documented in different places, and a reader has to assemble the whole picture from parts. Second, the identity strings and the behavioral contracts are coupled: the help contract's tests assert on the identity strings, the version contract's tests assert on the name, and a change to an identity string that does not update the corresponding behavioral tests would silently break the CLI's user-facing guarantees. The coupling is invisible from any single WBS item.

Three constraints shape the decision. First, the identity strings are public: they appear in `forge --help`, in documentation, in shell scripts that capture the help output, and in user-facing messages. Changing them is cheap in code and expensive in every artefact that quotes them. Second, the behaviors are contracts with scripts: a script that invokes `forge` with no arguments expects exit 0 and help on stdout; a script that invokes `forge --version` expects the version block; a script that invokes an unknown command expects a non-zero exit and a diagnostic on stderr. These expectations are external to the code and cannot be changed by editing it. Third, the CLI is early in its life: Phase 2 has just delivered the root command and its first subcommand, and later phases will add more commands, more flags, and more behaviors. The decision made now sets the pattern for everything that follows.

The parties affected are: users (who rely on the identity strings and the behaviors), script authors (who branch on the exit codes and parse the output), the Forge Core Team (who will extend the root command in later phases), and reviewers (who must evaluate changes to the root command against a stable reference). The relevant specifications are `docs/cli-ux-spec.md` § 4.7 (Root Command Identity), § 4.8 (Version Output Contract), § 4.9 (Help Behaviour), § 4.10 (Version Behaviour), § 4.11 (Global Flags), § 7 (Exit Codes), and § 9 (CLI UX Principles); `docs/architecture.md` § 11 (The Two-Boundary Execution Model), § 11.12 (Handler / Service Boundary), § 11.13 (Command Registration), § 11.14 (Global Flags), § 11.15 (Pre-Parse Argument Validation), and § 11.16 (Command Lifecycle Hooks); and the WBS items 5.1.1 through 5.4.1, which delivered the root command's behavior. The relevant requirement IDs are `FR-CLI-001` through `FR-CLI-010`, `EXIT-001` through `EXIT-004`, `TEST-003`, `DOC-003`, and `DOC-004`.

---

## 2. Options Considered

### Option A — Freeze the identity strings and the behavioral contracts as a single set of contracts

The root command's identity strings (`RootName`, `RootUsage`, `RootShortDesc`, `RootLongDesc`) and its behavioral contracts (no-args, help, version, unknown-command, global flags) are declared frozen in a single ADR. The identity strings live as exported constants in `internal/cli/root.go`; the behavioral contracts live as documented contracts in `docs/cli-ux-spec.md` § 4.7 through § 4.11 and are enforced by tests. Any change to any of them requires a new ADR.

**Pros:**

- The decision has a single home. A reader who wants to know what the root command guarantees reads one ADR and knows.
- The coupling between identity strings and behavioral contracts is visible. The ADR explains why the two are decided together: the behavioral tests reference the identity constants, and a change to one affects the other.
- The frozen status is enforced by tests, not merely documented. Each identity string has a test that asserts its invariants; each behavioral contract has a test that asserts its properties. A change to either fails a test and forces the contributor to update the ADR, the specification, and the test in the same commit.
- The rule "adding a new global flag requires an ADR" is stated once and inherited by the flag inventory, the flag semantics, and the flag precedence.
- The identity strings can be extended in a later phase without invalidating the decision. Adding a fifth identity string (say, a homepage URL) is a new decision, but it does not require revisiting the decision that froze the first four.

**Cons:**

- A single ADR covering both identity and behavior is broader than a strictly minimal decision. A reader who wants to change only the short description must read the whole ADR to understand what else is frozen.
- The ADR's scope spans seven WBS items, which makes it longer than a typical ADR.
- The frozen status is a constraint. A future product decision to rename the CLI or change the short description requires the full ADR process, which is slower than a direct edit. The slowdown is deliberate, but it is a real cost.

### Option B — Freeze the identity strings only; leave the behavioral contracts to the specification

The identity strings are declared frozen in the ADR. The behavioral contracts remain documented in `docs/cli-ux-spec.md` and are treated as ordinary specification content, changeable by the usual specification-review process.

**Pros:**

- The ADR is narrower. It decides the identity strings and nothing else.
- The specification's change process is lighter than the ADR's. Small behavioral refinements can be made without a new ADR.

**Cons:**

- The coupling between identity and behavior is invisible. A contributor who changes a behavioral contract that references an identity string might not realize that the identity string's tests also need updating.
- The "frozen" status of the identity strings is weakened by the fact that the behavioral contracts that assert on them are not frozen. A behavioral change that indirectly forces an identity change could bypass the ADR process.
- The `--version` flag's wiring, which is a behavioral decision that touches an identity constant (`RootName`), is decided in two places: the identity in the ADR, the wiring in the specification. A reader who wants to understand the version output must read both.
- The "adding a new global flag requires an ADR" rule is not stated in the ADR, because the ADR does not cover global flags.

### Option C — Do nothing; leave the root command's contracts as they are

The identity strings and the behavioral contracts remain as they were delivered by the WBS items. No new decision is recorded.

**Pros:**

- No work. The current state is preserved.
- The WBS items already document each facet; a reader can assemble the whole picture from them.

**Cons:**

- The two risks identified in the Context remain: no single document states what the root command guarantees, and the coupling between identity and behavior is invisible.
- A future contributor who wants to change one facet has no reference to check against. They must read every relevant WBS item and specification section to determine whether their change is permitted.
- The KPI for the root command milestone — "zero drift in root identity strings after freeze" — cannot be verified without a decision that declares the strings frozen.
- The pattern for future commands is unset. When later phases add subcommands, they will each have their own identity strings and behavioral contracts; without a decision about the root command, they have no template to follow.

### Option D — Move the identity strings into a dedicated `internal/identity` package

The identity strings are extracted from `internal/cli/root.go` and placed in a new package `internal/identity`. The package exports `Name`, `Usage`, `ShortDesc`, and `LongDesc`, which the CLI imports.

**Pros:**

- The identity strings have a single home that is not coupled to the CLI's implementation.
- A future tool that nests Forge as a subcommand (for example, a build orchestrator) can import `internal/identity` without importing `internal/cli`.
- The package's existence is a declaration that the strings are public.

**Cons:**

- The package would have exactly four constants and no functions. It is a package for four strings, which is heavier than a set of constants in the existing package.
- The strings are only meaningful to the CLI. No other consumer exists, and none is anticipated. A dedicated package for a hypothetical consumer is speculative.
- The identity strings are coupled to the CLI's behavior: the help contract asserts on `LongDesc`, the version contract asserts on `Name`, and the flag inventory is registered on the root command. Moving the strings out of the CLI package would require every assertion to import a second package.
- The move adds an `internal/identity` package to the structural test's allowlist, which requires updating that test. The move's cost exceeds its benefit.

---

## 3. Decision

**Option A is chosen.** The root command's identity strings and its behavioral contracts are frozen as a single set of contracts, recorded in this ADR, enforced by tests in `internal/cli/root_test.go`, and documented in `docs/cli-ux-spec.md` § 4.7 through § 4.11. Any change to any of them requires a new ADR that supersedes this one in whole or in part.

The decision is stated as follows:

1. **The identity strings are frozen.** `RootName`, `RootUsage`, `RootShortDesc`, and `RootLongDesc` are declared as exported constants in `internal/cli/root.go` and are the single source of truth for the CLI's identity. Their invariants (lowercase name, single-line short description of 80 runes or fewer, no emojis, no colour codes, no tabs, presence of the core loop in the long description, presence of the `<command> --help` pointer) are asserted by tests and cannot change without an ADR.

2. **The behavioral contracts are frozen.** The no-args contract (help on stdout, exit 0), the help contract (eight invocations with fixed exit codes and output streams), the version contract (five invocations with byte-identical output between `forge --version`, `forge -v`, and `forge version`), and the unknown-command contract (non-zero exit, diagnostic on stderr) are documented in `docs/cli-ux-spec.md` and enforced by tests. Any change to a contract's observable behavior requires an ADR.

3. **The global flag inventory is frozen.** Forge has exactly three global flags in Phase 2: `--verbose`, `--quiet`, and `--config`. Adding a fourth flag requires an ADR that names the consumer WBS item, defines the flag's semantics, and defines its precedence relative to the existing flags.

4. **The coupling between identity and behavior is acknowledged.** The behavioral contracts reference the identity strings; a change to an identity string requires updating the corresponding behavioral tests and the specification in the same commit. This ADR is the place where the coupling is stated.

5. **The contracts are enforced by tests, not merely documented.** Every invariant listed above has at least one test in `internal/cli/root_test.go` (and, for the version service's format, in `internal/app/version/service_test.go`). A change to any invariant fails a test and forces the contributor to update the ADR, the specification, and the test together.

The balance tipped toward Option A for four reasons. First, the coupling between identity and behavior is real and would be invisible if the two were decided separately. The help contract's tests assert on `RootLongDesc` and `RootUsage`; the version contract's tests assert on `RootName`; the flag tests assert on the flag-name constants. A change to an identity string would silently break the corresponding behavioral tests unless the coupling were acknowledged. Option A acknowledges it.

Second, the frozen status is enforceable. Option B would leave the behavioral contracts changeable by the specification's lighter process, which would let a behavioral change slip through that indirectly forces an identity change. Option A closes that path: any change to any of the contracts requires an ADR.

Third, the ADR is a template for later phases. When Phase 5 adds subcommands with their own identity strings and behaviors, each subcommand will need a decision analogous to this one. Having a single ADR that establishes the pattern for the root command gives those later decisions a reference.

Fourth, the cost of freezing is small. The identity strings are four constants; the behavioral contracts are five invocations. The tests that enforce them are already written (WBS 5.1.1 through 5.4.1). The ADR adds the declaration that the two are coupled and frozen; the tests do the enforcement. The cost of a future change is a new ADR, which is the correct price for a change to a public contract.

The decision depends on three assumptions that could later be falsified. First, that Cobra remains the CLI framework. If Cobra were replaced, the help contract's eight invocations and the version contract's rendering would change, and this ADR would need to be superseded. The identity strings would survive; the behavioral contracts would not. Second, that the CLI's user-facing surface remains small enough that a single ADR can describe it. If the root command accumulates twenty flags and fifty behaviors, this ADR's scope becomes unwieldy and would need to be split. Third, that the CLI's identity strings are stable enough to freeze. If a future product decision renames Forge — say, to "Forge CLI" or to a different product name — the identity strings change and the ADR is superseded. The assumption is that no such decision is anticipated in Phases 2 through 5.

The decision is not time-bounded. It is intended to hold for the life of the root command's current shape.

---

## 4. Consequences

### 4.1 Positive

- **The root command's contract has a single home.** A reader who wants to know what the root command guarantees reads one ADR. The ADR names every frozen element and points at the tests that enforce it. The reader does not have to assemble the picture from seven WBS items and six specification sections.

- **The coupling between identity and behavior is visible.** The ADR states that the behavioral contracts reference the identity strings, and that a change to one requires updating the other. A contributor who is about to change an identity string will read the ADR and see the coupling before making the change.

- **The frozen status is enforceable.** Every invariant has a test. A change to an identity string fails its invariant test; a change to a behavioral contract fails its contract test; a change to the flag inventory fails the inventory test. The tests are the mechanism by which the ADR's declarations are enforced. The ADR is not merely documentation; it is the reason the tests exist.

- **The ADR is a template for later phases.** When Phase 5 adds subcommands, each will have its own identity strings and behaviors. This ADR establishes the pattern: freeze the strings as constants, document the behaviors as a contract, enforce both with tests, and require an ADR to change either. The subcommands' decisions will follow the same shape.

- **The rule "adding a new global flag requires an ADR" is stated once and applies everywhere.** The rule is in the ADR, not scattered across the flag inventory's comments and the Taskfile target's docstring. A contributor who wants to add a flag reads the ADR and knows what to do.

### 4.2 Negative

- **The ADR is long.** It covers seven WBS items and six specification sections. A reader who wants to change only the short description must read the whole ADR to understand what else is frozen. The length is the price of the coupling: deciding identity and behavior separately would produce two shorter ADRs, but at the cost of losing the connection between them.

- **The frozen status is a constraint.** A future product decision to rename the CLI or to change the short description requires an ADR. The ADR process is slower than a direct edit. The slowdown is deliberate — a public contract should not change without deliberation — but it is a real cost that future contributors will feel.

- **The `RootShortDesc` constant is frozen but is not rendered by `forge --help`.** The constant is part of the CLI's identity, but Cobra renders a command's `Short` field only in a parent's command list, and the root command has no parent. The constant therefore does not appear in the help output. The freeze protects the constant for future uses (for example, a future tool that nests Forge as a subcommand), but a reader who expects the constant to appear in `forge --help` will be surprised. The specification § 4.7 and the test `TestRootIdentity_HelpOutputContainsConstants` document the fact; the ADR records it.

- **The identity strings are coupled to the CLI package.** Option D proposed extracting the strings to a dedicated `internal/identity` package. That option was rejected, but the rejection means the strings live in `internal/cli`, coupled to the CLI's implementation. A future consumer outside the CLI package cannot import the strings without importing the CLI package, which would create an import it may not want.

### 4.3 Neutral

- The ADR's scope covers both identity and behavior. A reader who was expecting a narrower decision may find the scope surprising. The ADR's Context section explains why the two are decided together.

- The tests that enforce the ADR are spread across multiple test files (`root_test.go`, `flags_test.go`, `version_test.go`, `deps_test.go`, and `internal/app/version/service_test.go`). A future contributor who wants to see all the enforcement in one place must read the ADR's cross-references. The ADR lists the test files in its Notes section.

- The identity strings are exported from `internal/cli` because the tests in `package cli_test` reference them. If the tests were rewritten to reference them via an unexported accessor, the strings could become unexported. The current design keeps them exported, which is a small concession to testability.

---

## 5. Related Requirements

- `FR-CLI-001` — the CLI must be invocable from a process entry point with a frozen signature. This ADR freezes the entry point's identity and behavior.
- `FR-CLI-002` — the CLI must support `--help` and `-h` with identical output. This ADR ratifies the requirement as a frozen contract.
- `FR-CLI-003` — the CLI must support `--version` with output identical to the `version` subcommand. This ADR ratifies the requirement as a frozen contract.
- `FR-CLI-008` — the CLI must support global flags (`--verbose`, `--quiet`, `--config`). This ADR freezes the inventory.
- `EXIT-001` — exit codes must be defined as named constants. This ADR asserts the behavioral contracts' exit codes.
- `EXIT-003` — the exit code contract must be documented. This ADR points at `docs/cli-ux-spec.md` § 7.
- `TEST-003` — CLI command behavior must be testable without a subprocess. This ADR's enforcement relies on the in-process test pattern established by WBS 4.3.2.
- `DOC-003` — the architecture document must describe the CLI package boundary. This ADR cross-references `docs/architecture.md` § 11.
- `DOC-004` — the development guide must describe how to add a command. This ADR cross-references `docs/development.md` § "Adding a Command" and § "Handler / Service Boundary Review".

The following requirements are anticipated and must be added to the register when their respective WBS items are complete:

- `FR-CLI-011` — the CLI must freeze the root command's identity strings and behavioral contracts (this ADR).
- `FR-CLI-012` — adding a global flag must require an ADR (this ADR).

---

## 6. Related ADRs

- `ADR-001` — Forge as a foundation manager. This ADR establishes the product context in which the root command exists; it is the ancestor decision.
- `ADR-002` — Choose Cobra as the CLI framework. This ADR establishes the framework whose rendering rules shape the help contract and the version contract; this ADR depends on it.
- `ADR-003` — CLI Framework Foundation: Package, Execution, Injection, Registry, and Contract. This ADR establishes the shape of the CLI package; this ADR refines one part of that shape for the root command.
- `ADR-005` — Command Constructor Contract. This ADR is anticipated by WBS 4.4.2 and will be written as part of that WBS item. It is the sibling decision to this one: ADR-005 freezes the shape of a command's constructor; this ADR freezes the shape of the root command's identity and behavior.

If ADR-005 is not yet written when this ADR is accepted, this ADR's Notes section will note the pending decision.

---

## 7. Notes

**On the tests that enforce this ADR.** The enforcement is spread across several test files. A reader who wants to see the enforcement in one place should read:

| Test file | What it enforces |
|-----------|------------------|
| `internal/cli/root_test.go` | The identity strings, the help contract, the version flag's behavior, the unknown-command behavior, and the global flag inventory and precedence. |
| `internal/cli/flags_test.go` | The global flag readers and the log-level resolver. |
| `internal/cli/version_test.go` | The version handler's thinness and its delegation to the service. |
| `internal/cli/deps_test.go` | The `Dependencies` struct's construction and field contracts. |
| `internal/app/version/service_test.go` | The version service's `Get`, `Format`, and `Raw` functions, including the format contract. |

**On the identity strings that are frozen but not rendered.** `RootShortDesc` is frozen but is not rendered by `forge --help` because Cobra renders a command's `Short` field only in a parent's command list, and the root command has no parent. The constant is part of the CLI's identity and will be rendered if Forge is ever nested as a subcommand of another tool. A future contributor who wants to render the constant in `forge --help` would need to override Cobra's help template, which would be a change to the help contract and would require a new ADR.

**On the `--version` flag's wiring.** The `--version` flag's output is the string returned by `version.Raw()`, assigned to `root.Version`, rendered by Cobra using the template `{{.Version}}`. The template override is what makes `forge --version` and `forge version` byte-identical. This wiring is part of the version contract and is frozen by this ADR.

**On the pre-parse validator's role.** The pre-parse validator (`validateArgs`) rejects four malformed invocations: `forge --help <cmd>`, `forge --version <arg>`, `forge --config` with no value, and `forge help <unknown>`. The validator is the mechanism by which the help contract's and version contract's rejections are implemented. Its behavior is part of the behavioral contracts and is frozen by this ADR. The validator's implementation is in `internal/cli/validate.go`.

**On the `PersistentPreRunE` hook's reservation.** The root command installs a `PersistentPreRunE` hook that is reserved for configuration loading (WBS 8.0) and logger initialization (WBS 12.0). The hook is a no-op in Phase 2. The reservation is part of the root command's contract: no subcommand may install its own `PersistentPreRunE`, because that would override the root's hook. This reservation is documented in `docs/architecture.md` § 11.16 and is enforced by tests in `internal/cli/hooks_test.go`. It is mentioned here because it is part of the root command's shape and interacts with the behavioral contracts.

**On the pending ADR-005.** If ADR-005 (Command Constructor Contract) is not written when this ADR is accepted, the two decisions are complementary: ADR-005 freezes the shape of a command's constructor; this ADR freezes the shape of the root command's identity and behavior. The two should be read together by a contributor who is adding a command.

**On the `RootShortDesc` assertion in the acceptance test.** An earlier version of `TestAcceptance_NoArgs` asserted that `forge --help` contains `RootShortDesc`. That assertion was incorrect: the constant is not rendered by the command's own help. The assertion was corrected to reference `RootLongDesc`, which is rendered. The correction is documented in the test's docstring and in `docs/cli-ux-spec.md` § 4.7.

**On the possibility of splitting this ADR.** If a future phase changes the root command's contract substantially — for example, by adding a new global flag, or by rewriting the help output — the change is recorded in a new ADR that supersedes this one in whole or in part. If the change is narrow (for example, adding a fifth identity string), it is recorded in a new ADR that supersedes only the relevant clause. If the changes accumulate to the point where the root command's contract is unrecognizable, this ADR is superseded in full by a new one that reflects the new shape.

**On external discussion.** None. This ADR is drafted in isolation and reviewed via the repository's pull request process.

---

## Appendix A: How to Use This Template

This appendix is a guide for the author. It is **not** included when creating an actual ADR — delete this entire appendix after copying the template.

### Step 1 — Copy the template

```bash
cp docs/decisions/template.md \
   docs/decisions/ADR-004-root-command-identity-and-behavior-contracts.md
```

Replace `NNN` with the next available number. See
[`README.md`](./README.md) § 6 for numbering rules.

### Step 2 — Fill in the metadata block

Set:

- `Status` — leave as `Proposed`
- `Date` — the date you begin drafting
- `Deciders` — the person or role making the decision
- `Supersedes` — the previous ADR, if any
- `Superseded by` — leave as `—` until superseded

### Step 3 — Write each section

Follow the guidance in each section above. Do not skip sections.
Use `—` for empty ones, except in Options Considered where at least
two options are required.

### Step 4 — Reference requirements

Every requirement ID you mention must exist in
[`docs/requirements-register.md`](../requirements-register.md), or
be listed as pending in the register.

### Step 5 — Review

Open a pull request. Request review from at least one person (yourself,
if working solo). Confirm the review checklist from
[`README.md`](./README.md) § 7.

### Step 6 — Accept

After review, change `Status` to `Accepted`. Update the index in
[`README.md`](./README.md) § 10.

### Step 7 — Commit

Commit the ADR and the README index update together as a single
change. This ensures the index never lags behind the directory.

---

## Appendix B: Worked Example

See [`ADR-001-forge-as-foundation-manager.md`](./ADR-001-forge-as-foundation-manager.md)
for a complete, accepted ADR produced from this template.

---

*End of template.*
