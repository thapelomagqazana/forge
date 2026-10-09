# ADR-002: Choose Cobra as the CLI framework

**Status:** Accepted
**Date:** 2026-10-09
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge is a cross-platform command-line tool whose entire user surface is the terminal. The command tree will eventually contain more than a dozen commands (`new`, `init`, `check`, `diff`, `update`, `explain`, `add`, `remove`, `validate`, `template`, `config`, `doctor`, `version`), several with nested subcommands and consistent flag semantics.

Every one of these commands must support, at minimum:

- a help system (`--help`, `forge help <command>`, `forge help <command> <subcommand>`);
- a POSIX-style flag parser that accepts both `--flag value` and `--flag=value` forms, with short and long variants;
- shell completion for Bash, Zsh, Fish, and PowerShell;
- structured positional argument validation;
- a stable behaviour contract for exit codes and error output that Forge itself controls, not the framework.

The Go standard library provides the `flag` package, but `flag` is a flat flag parser. It does not model subcommands, does not generate help output, does not provide completion, and does not validate positional arguments. Building these features on top of `flag` would mean reimplementing a well-solved problem — and reimplementing it worse, because the resulting code would have no ecosystem, no community, and no battle-testing at scale.

A CLI framework is therefore required. The framework choice is not reversible without substantial rework: every command and subcommand depends on the framework's command model, its flag API, and its help and completion output. The decision must be made before WBS 4.0 (CLI Framework Foundation) begins, because that WBS builds the command tree on top of whatever framework is chosen.

The decision is constrained by:

- **License compatibility.** Forge is Apache-2.0. The framework's license must be compatible.
- **Go version compatibility.** Forge's minimum supported Go version is 1.23.0 (see ADR-004). The framework must build on Go 1.23.0 or later.
- **Dependency weight.** Every third-party dependency increases supply-chain risk and audit surface. The framework's transitive dependency tree must be small and well maintained.
- **Reproducibility.** The framework must be pinnable to an exact version, per the reproducibility contract in ADR-005.
- **Testability.** The framework must support in-process execution so that Forge's command tests do not require spawning subprocesses.

If no decision is made, WBS 4.0 cannot begin, and every later CLI-facing WBS item is blocked. The task budget for this decision is small (0.5 day); the cost of delaying it is unbounded.

Affected parties: every future contributor to `internal/cli`, every external contributor adding a command, and every downstream tool that parses Forge's help output, exit codes, or shell completion.

---

## 2. Options Considered

### Option A — Go standard library `flag` (with custom extensions)

Build the CLI on the standard library's `flag` package, adding subcommand dispatch, help generation, and completion manually.

**Pros:**

- Zero third-party dependencies. Fully satisfies the standard-library-first principle.
- Complete control over behaviour; no framework quirks to work around.
- No supply-chain risk from a new dependency.
- No license compatibility questions.

**Cons:**

- No subcommand support. Every command tree would be built manually.
- No `--help` generation. Help output would be hand-written per command.
- No shell completion. Completion scripts would need to be hand-written and maintained.
- No positional argument validation.
- POSIX-style flags (`--flag=value`, `-f value`, combined short flags) are not supported. Forge's CLI UX spec assumes POSIX conventions.
- Every contributor would need to learn Forge's custom CLI conventions instead of a widely-known framework.

### Option B — `github.com/urfave/cli`

Use the `urfave/cli` framework, a long-standing alternative to Cobra.

**Pros:**

- Mature and widely used.
- Supports subcommands, flags, and help generation.
- Smaller dependency tree than Cobra in some configurations.
- MIT license.

**Cons:**

- Smaller ecosystem of downstream CLIs than Cobra. Fewer reference implementations, less community documentation.
- Subcommand nesting (three or more levels) is less ergonomic than Cobra's.
- Shell completion is less mature than Cobra's.
- The framework's design couples command metadata and command logic more tightly than Cobra, which makes the "thin handler / application service" split that Forge requires harder to enforce.

### Option C — `github.com/alecthomas/kong`

Use the `kong` framework, a struct-tag-driven CLI parser.

**Pros:**

- Elegant, declarative design. Commands are Go structs with tags.
- Very small code footprint per command.
- Struct-tag approach integrates naturally with Go's type system.
- MIT license.

**Cons:**

- Struct-tag-driven design is less familiar to the majority of Go developers. Most Go developers who have built a CLI have used Cobra.
- Positional argument handling and dynamic command registration are less ergonomic than Cobra's.
- Smaller community and ecosystem.
- Less battle-tested at scale for the class of tools Forge aspires to resemble (`kubectl`, `gh`, `hugo`).

### Option D — `github.com/spf13/cobra`

Use the Cobra framework, the de facto standard for Go CLIs.

**Pros:**

- Used by `kubectl`, `gh` (GitHub CLI), `hugo`, `helm`, `docker` CLI, `etcd`, `delve`, and hundreds of production tools. The pattern is well understood at scale.
- Explicit `*cobra.Command` type with `Use`, `Short`, `Long`, `RunE` fields. This cleanly separates command metadata from command logic, which supports Forge's "thin handler / application service" split.
- Subcommand nesting is first-class. Arbitrary depth.
- POSIX-style flag parsing via `pflag` (transitive dependency). Both `--flag value` and `--flag=value` and combined short flags are supported.
- Help generation is automatic and consistent.
- Shell completion is generated for Bash, Zsh, Fish, and PowerShell.
- `cobra.NoArgs`, `cobra.ExactArgs(n)`, and `cobra.MatchAll` provide structured positional argument validation.
- Testable in-process: `*cobra.Command` can be executed with injected `args`, `stdin`, `stdout`, and `stderr`.
- Apache-2.0 license, fully compatible with Forge.
- Small, well-audited transitive dependency tree: `spf13/pflag` (same maintainer) and `inconshreveable/mousetrap` (Windows only).

**Cons:**

- Adds a third-party dependency. This is the first violation of the pure standard-library principle in Forge.
- Cobra's defaults (print usage and error on failure) conflict with Forge's UX contract. Both defaults must be disabled at the root command.
- The API surface is large; contributors must learn the subset Forge actually uses.
- Cobra is stable but not frozen. Upgrades require deliberate review.

### Option E — Roll our own CLI framework

Build a bespoke CLI framework in an internal package.

**Pros:**

- Complete control over behaviour.
- Zero third-party dependencies.

**Cons:**

- Duplicates well-solved problems: subcommand dispatch, flag parsing, help formatting, completion script generation, positional validation.
- The resulting framework would need to be maintained indefinitely.
- Contributors would need to learn a bespoke framework instead of a standard one.
- Time cost: weeks, not hours.
- Ecosystem cost: no community, no shared knowledge, no external documentation.
- Correctness cost: Cobra has years of bug fixes; a bespoke framework would rediscover the same bugs.

---

## 3. Decision

Forge uses **`github.com/spf13/cobra`** (Option D) as its CLI framework, pinned to the exact version `v1.8.1` at the time of this decision.

The decision is driven by four factors.

First, **Cobra is the de facto standard for Go CLIs.** Every Go developer who has built a CLI has used or seen Cobra. The framework's idioms (`Use`, `Short`, `Long`, `RunE`, `PersistentFlags`) are widely understood. Choosing Cobra means contributors spend their time on Forge's domain, not on learning a framework.

Second, **Cobra's command model directly supports Forge's architecture.** The distinction between `*cobra.Command` (metadata and handler) and the application services behind the handler is exactly the split Forge requires (see WBS 4.3.1). Frameworks that couple the two (Option B) or that hide commands behind tags (Option C) make this split harder to enforce.

Third, **Cobra's transitive dependency tree is small and auditable.** Two dependencies: `spf13/pflag` and `inconshreveable/mousetrap`. Both are maintained by the same author as Cobra, both have permissive licenses, both have no significant attack surface. This is materially smaller than the dependency trees of most alternatives.

Fourth, **Cobra is Apache-2.0 licensed**, fully compatible with Forge's license, with no copyleft implications.

The decision is made **despite** two accepted tradeoffs. First, Forge accepts a third-party dependency where a pure standard-library approach would have been possible (Option A). Second, Forge accepts that Cobra's defaults — printing usage and errors on failure — conflict with its own UX contract, and must be disabled at the root command. Both tradeoffs are documented in Section 4.2 below.

The decision is bounded by the following assumptions, each of which could later be falsified:

- Cobra v1.x remains actively maintained. If Cobra is abandoned, a migration ADR would be required.
- The transitive dependency tree does not grow substantially. If `pflag` or `mousetrap` gain new dependencies, the pin and review would be revisited.
- Go's `flag` package does not gain subcommand support. If it does, the decision could be revisited.

The decision does not depend on any time-sensitive factor and is expected to hold for the lifetime of the project.

---

## 4. Consequences

### 4.1 Positive

- The command tree is expressible declaratively. Adding a new command is a matter of adding a new file with a `newXxxCmd()` constructor, registering it in `registry.go`, and writing tests. The framework handles dispatch, help, completion, and flag parsing.
- Help output is consistent across all commands. Every command gets a `--help` flag and a `forge help <command>` entry with no additional code.
- Shell completion is available for Bash, Zsh, Fish, and PowerShell. Users get tab-completion without Forge writing a line of completion code.
- POSIX-style flags are supported. `--flag value`, `--flag=value`, `-f value`, `-fvalue`, and combined short flags (`-abc`) all work as users expect.
- Positional argument validation is provided by `cobra.Args` validators (`NoArgs`, `ExactArgs`, `MinimumNArgs`). Commands do not hand-roll argument validation.
- Commands are testable in-process. `*cobra.Command` can be executed with a custom `io.Writer` for stdout and stderr, and custom `[]string` args. No subprocess is required for the common test case.
- The framework is familiar to contributors. Reviews of CLI-related PRs are faster because reviewers already understand the idioms.

### 4.2 Negative

- **Forge acquires its first third-party dependency.** This is a deliberate departure from the pure standard-library-first principle. The departure is bounded: Cobra is the only direct dependency in Phase 2, and adding another requires an ADR (see `docs/dependency-policy.md`).
- **Cobra's defaults conflict with Forge's UX contract.** By default, Cobra prints usage on error and writes errors to stdout. Both violate the contract defined in WBS 7.4 (`stdout` for results, `stderr` for diagnostics). Both defaults are disabled at the root command via `SilenceUsage: true` and `SilenceErrors: true`, and this is documented in `internal/cli/root.go`. Every future command inherits the correct behaviour because it is set at the root.
- **The API surface is large.** Contributors must learn the subset Forge uses. The package documentation in `internal/cli` narrows this subset explicitly.
- **Cobra is not frozen.** Upgrades follow the policy in
  `docs/dependency-policy.md`. Patch upgrades require a note in the PR;
  minor upgrades require a brief ADR; major upgrades require a full
  migration ADR. This is a small ongoing cost that most third-party
  dependencies impose.
- **The dependency tree includes `inconshreveable/mousetrap`,** a Windows-only package. It is compiled only on Windows but is present in the module graph on all platforms. The cost is negligible in practice: the package is ~200 lines and has no transitive dependencies.

### 4.3 Neutral

- Forge's CLI behaviour is now partly determined by Cobra's design decisions. This is not positive or negative in itself; it is a consequence of using any framework. The design decisions that matter for Forge (help format, flag semantics, exit code handling) are either disabled (usage on error), overridden (error output), or inherited from Cobra's conventions (help layout).
- Future frameworks will inevitably emerge. If one becomes materially better than Cobra for Forge's needs, a migration ADR could be considered. No such framework exists today.

---

## 5. Related Requirements

- `FR-CLI-002` — `forge help` displays command information. Cobra provides the help system this requirement depends on.
- `FR-CLI-009` — Command registration is centralised. Cobra's command tree model shapes how the registry is built.
- `FR-CLI-010` — Commands are testable without spawning a process. Cobra's in-process execution model enables this requirement.

> The following requirement should be added to the register when WBS 1.4 is complete: `FR-CLI-011` — CLI framework is pinned to an exact version and documented in `docs/dependency-policy.md`.

---

## 6. Related ADRs

- `ADR-001` — Forge as a foundation manager. This ADR establishes the product context in which the CLI framework is chosen. Not a dependency, but a framing reference.
- `ADR-003` — Module path is `github.com/thapelomagqazana/forge`. Cobra is imported as a module path; the decision here is unaffected by the module path, but the two are related because every import of Cobra lives under Forge's module path.
- `ADR-004` — Go version matrix. Cobra's minimum Go version (1.15) is well within Forge's minimum (1.23.0), so this ADR's constraint is satisfied. A future Cobra release that required a newer Go would interact with ADR-004.
- `ADR-005` — Reproducible module and toolchain configuration. Cobra is the first dependency subject to the reproducibility contract defined in ADR-005.

---

## 7. Notes

- **Open question:** Should Cobra's shell completion be enabled by default in Phase 2, or deferred to Phase 6 (Developer Experience Refinement)? The decision is deferred to WBS 6.x. Phase 2 adds Cobra as a dependency but does not ship user-facing completion; the completion feature is wired but hidden behind a future WBS item.

- **Follow-up decision:** The exact version policy for Cobra upgrades is documented in `docs/dependency-policy.md`. That policy is a living document and may be refined as Forge gains experience with the dependency.

- **Follow-up decision:** When Cobra v2 is released (no timeline), a superseding ADR will be needed to migrate. This ADR remains in force until then.

- **External discussion:** The comparison between Cobra, urfave/cli, and kong is documented in numerous community writeups. Forge's decision is not novel; it follows the pattern of the majority of large Go CLIs. The relevant precedent is the Kubernetes CLI (`kubectl`), which is the closest analogue to Forge in command-tree complexity.

---

*End of ADR-002.*
