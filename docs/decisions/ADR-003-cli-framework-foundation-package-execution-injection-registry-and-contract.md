# ADR-003: CLI Framework Foundation — Package, Execution, Injection, Registry, and Contract

**Status:** Proposed
**Date:** 2026-10-10
**Deciders:** @thapelomagqazana
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge's CLI is the only surface a user or a CI system interacts with. The behaviour of that surface — how arguments are parsed, how the environment is injected, how commands are registered, how errors map to exit codes, how commands are tested — must be decided before the first user-facing command ships. If it is not, each command will invent its own shape, and the CLI's public contract will be discoverable only by reading every command file.

Two forces make this decision urgent now. First, WBS 4.1.1 through 4.4.2 introduce the CLI framework's foundation. They are the last tasks before WBS 5.x adds the substantive commands (`forge new`, `forge init`, `forge validate`, and so on). Every decision in this ADR is a prerequisite for those commands; none of them can be deferred to a later phase without leaving the first command without a pattern to follow. Second, the Forge architecture document already commits the project to a two-boundary execution model (`options` → `Dependencies` → commands) and a handler / service boundary. Those commitments are visible in WBS 4.2.2 and WBS 4.3.1. This ADR ratifies them as decisions rather than as loose descriptions.

The constraints are:

- **The CLI is a boundary, not a container.** Business logic belongs in the domain and application layers. The CLI parses, dispatches, formats, and maps to exit codes. Nothing else.
- **One dependency, one framework.** Cobra is the only permitted third-party dependency in Phase 2. The CLI framework's shape must not require a second.
- **Testability without subprocesses.** The framework must make every command's behaviour testable in-process, through the same boundary production code uses. Subprocess tests are reserved for build-level verification.
- **The registry will grow.** Phase 2's two commands become fifteen or more over the course of Phases 3–5. The registry's contract must be frozen before the growth begins, not after.

If no decision is made, the first substantive command (WBS 5.x) will set the pattern implicitly, and every subsequent command will either follow that accidental pattern or diverge from it. The cost of a retrofit grows with each command added. The CLI framework's foundation is the last moment at which the decision is free.

The parties affected are: the Forge Core Team (who will add commands), reviewers (who must enforce the pattern), and every future contributor (who will read the framework's code and infer its rules).

The requirements this decision relates to are `FR-CLI-001`, `FR-CLI-005`, `FR-CLI-009`, `FR-CLI-010`, `EXIT-001` through `EXIT-004`, `TEST-003`, `DOC-003`, and `DOC-004`.

---

## 2. Options Considered

### Option A — A minimal framework with a single exported symbol, injectable execution, and a central registry

The CLI package (`internal/cli`) exports exactly one function, `Execute() int`. It owns the command tree, the injectable environment (`options` → `Dependencies`), and the exit-code mapping. Every command is constructed by a function with a fixed signature and registered in a single slice. Commands are thin handlers that delegate to application services.

**Pros:**

- The public contract is one symbol. Downstream packages cannot depend on internals, so the framework can be refactored without breaking them.
- Injectability is uniform. Every command receives the same `Dependencies` value, which is the single point at which collaborators are wired.
- The registry is a single, reviewable, greppable slice. Adding a command is a two-file edit: the new command's file and the registry entry.
- In-process testing is the default. `executeWithOptions` accepts synthetic inputs; the `runCLI` helper exercises it. No command test spawns a subprocess.
- The contract is frozen in writing and enforced by a compiler check (the `commandConstructor` type) plus a structural test (`contract_test.go`).

**Cons:**

- Every command pays a small cost: a file, a constructor, a test file, a registry entry. A trivial command (for example, `forge version`) is more ceremony than a one-line `AddCommand` call would be.
- The `Dependencies` struct is a single point of change. Adding a collaborator means editing one struct and one constructor, and every command implicitly gains the field.
- The registry's ordering rule (alphabetical within groups) is a convention, not a compile-time property. A contributor can add a command in the wrong position without the compiler complaining; the `TestRegistry_OrderMatchesHelpOutput` test catches it, but only after the mistake is made.

### Option B — A conventional Cobra tree with no central registry and no injection struct

Each command file defines a `Command()` function that returns a `*cobra.Command`. The root command calls each `Command()` function directly. Commands read `os.Args`, `os.Stdout`, and `os.Getenv` as needed, and tests spawn the binary as a subprocess.

**Pros:**

- Familiar to anyone who has written a Cobra CLI. The pattern is documented in Cobra's README and in thousands of open-source projects.
- No new abstractions. A contributor who knows Cobra knows the pattern.
- Trivial commands are genuinely trivial: one file, one function, one `AddCommand` call.

**Cons:**

- The CLI's public contract is not one symbol; it is "whatever each command exposes." Downstream packages can depend on any command constructor, freezing the design in place.
- Testing requires a subprocess. Tests are slower (milliseconds per case), brittle (they depend on the binary being built), and unable to inspect internal state.
- No uniform way to inject collaborators. Each command reaches for `os.*` directly, and the handler / service boundary is enforced by convention only.
- Adding a command means editing the root command's file, which acquires a growing list of `AddCommand` calls. The command tree's shape is not in one place.
- The pattern cannot be frozen because there is no single shape to freeze.

### Option C — A dependency injection framework (for example, `google/wire`, `uber-go/dig`, or `facebookgo/inject`)

Use a third-party DI framework to construct the command tree. Commands declare their dependencies as fields or constructor parameters; the framework resolves them.

**Pros:**

- The wiring is declarative. Adding a collaborator means adding it to a provider, not to a struct.
- The framework handles the construction order automatically.

**Cons:**

- A second third-party dependency. The dependency policy permits one (`cobra`) in Phase 2; a DI framework would be a second, requiring an ADR for the dependency itself and a substantial review of its transitive tree.
- The construction order becomes implicit. A contributor reading a command cannot see what it depends on without reading the DI framework's configuration.
- The framework's error messages are opaque. A missing provider surfaces as a runtime error, not a compile-time one.
- The abstraction is heavier than the problem. The CLI has five collaborators, not fifty. A struct with six fields is sufficient.
- The `Dependencies` struct's type signature is a compile-time contract; a DI framework's container is a runtime one. The former is stronger.

### Option D — A hybrid: a central registry, but no injection struct; commands receive `options` directly

The registry is retained (Option A's strongest feature), but commands accept the raw `options` struct instead of a `Dependencies` struct. Each command decides for itself which fields to read.

**Pros:**

- Smaller surface. No `Dependencies` type, no `buildDependencies` function.
- Commands that need only `opts.stdout` and `opts.args` can ignore the rest.

**Cons:**

- Every command must know which `options` fields to resolve. A command that reads `opts.env` sees raw environment lookups; a command that reads `opts.rootPath` sees a raw path. The resolution logic is duplicated across commands.
- The two-boundary model collapses into one. There is no separate command boundary, and the structural test that enforces "commands do not read `os.*`" has nothing to check against.
- A future subsystem (config, logger, filesystem) must be threaded through `options` and resolved in each command, rather than resolved once in `buildDependencies`.

---

## 3. Decision

**Option A is chosen.** The CLI framework's foundation consists of: a package (`internal/cli`) that exports exactly one symbol (`Execute() int`); an injectable execution model in which `Execute` delegates to `executeWithOptions(options)`, which constructs a `Dependencies` value once and threads it through the command tree; a central registry that is the single, ordered, deterministic list of command constructors; a handler / service boundary that confines business logic to the application layer; and a frozen constructor contract that every command satisfies.

The reasoning is that the CLI's shape is a load-bearing decision that must be made before the first substantive command, and that the option that makes the shape statically checkable, reviewable, and testable is preferable to the option that makes it implicit, conventional, or framework-mediated.

The balance tipped toward Option A for four specific reasons:

1. **The compiler can enforce the shape.** The `commandConstructor` type makes the constructor signature a compile-time contract. A contributor who writes `func newFooCmd(opts options) *cobra.Command` receives a compile error, not a review comment. No other option provides this.

2. **In-process testing is the default, not a workaround.** `executeWithOptions` accepts a fully synthetic environment. Every command test can exercise the CLI end-to-end without spawning a process. This is the property WBS 4.3.2 exists to establish, and Option A is the only option that makes it the natural pattern rather than a special case.

3. **The registry is the shape.** Option B spreads the command tree across the root command's `AddCommand` calls; Option C spreads it across DI providers; Option D spreads it across each command's `options`-reading logic. Option A puts it in one slice. A reader who wants to know what commands exist opens `registry.go`. A reviewer who wants to know whether a new command is registered checks whether it appears in the slice.

4. **The public contract can be frozen.** Option A's one exported symbol is a contract the project can commit to for the life of Phase 2 and Phase 3. Options B and C expose every command constructor as a de facto public surface; freezing them would freeze the command set, and the command set is expected to grow.

The decision depends on three assumptions that could later be falsified:

- **Cobra remains the CLI framework.** If Cobra is replaced, the `commandConstructor` type's return value changes, and every command file is updated. The surrounding structure (one exported symbol, injectable execution, central registry, handler / service boundary) survives the replacement. The assumption is not that Cobra is permanent, but that the framework's shape is orthogonal to the CLI framework's identity.

- **The number of collaborators remains small.** The `Dependencies` struct has six fields. If it grows to twenty, the struct becomes a god object and the injection model should be revisited. The assumption is that the CLI's collaborators are bounded by the number of subsystems the CLI touches (config, logging, filesystem, output, environment), which is small and stable.

- **The command set is bounded by the registry's reach.** If Forge ever needs dynamically loaded commands (plugins), the static registry is insufficient. The assumption is that Phase 2 through Phase 5 use a static command set, and that plugins, if introduced later, will require their own ADR.

---

## 4. Consequences

### 4.1 Positive

- **The public surface is one symbol.** `internal/cli.Execute` is the only exported identifier. Downstream packages cannot depend on any other symbol. The framework can be refactored (renamed files, restructured functions, replaced internals) without a breaking change to any consumer except `cmd/forge/main.go`.

- **Injectability is uniform.** Every command receives the same `Dependencies` value, constructed once in `buildDependencies`. Adding a collaborator means editing one struct and one function; no command constructor changes.

- **In-process testing is the default.** The `runCLI` helper exercises the CLI through `executeWithOptions` with synthetic inputs. Every command test runs in microseconds, not milliseconds. The test suite is fast enough to run on every commit.

- **The command tree's shape is in one file.** `registry.go` is the single, greppable, reviewable declaration of which commands exist. Adding a command means appending to a slice; the diff is one line, plus the new command's file.

- **The contract is enforced, not merely documented.** The `commandConstructor` type enforces the constructor signature at compile time. `contract_test.go` enforces rules 1–6 and 8 of the frozen contract. The reviewer checklist in `docs/development.md` covers the rest.

- **The handler / service boundary is mechanical.** The structural tests in `version_test.go` assert that the handler does not call `os.Exit`, does not write to process streams, and does not import infrastructure packages. The tests fail loudly when the boundary is violated.

### 4.2 Negative

- **Every command pays a small ceremony cost.** A command that could be implemented as a single `AddCommand` call in Option B requires a file, a constructor, a test file, and a registry entry in Option A. For `forge version`, this is more code than the command's logic. The cost is accepted because it scales: fifteen commands with the ceremony cost are still fifteen commands, each following the same pattern; fifteen commands without it are fifteen different shapes.

- **The `Dependencies` struct is a single point of change.** Adding a collaborator edits the struct and the constructor. Every command implicitly gains the field, whether it uses it or not. This is a mild coupling between every command and every collaborator. It is accepted because the alternative — passing each collaborator as a separate parameter — would couple every command to every collaborator's *type* and would grow the constructor signature with each phase.

- **The registry's ordering is a convention, not a compile-time property.** A contributor can add a command in the wrong position. The `TestRegistry_OrderMatchesHelpOutput` test catches the mistake, but only after the fact. The convention is enforced by review and by the test; it is not enforced by the compiler.

- **Cobra's auto-generated commands (`completion`, `help`) complicate the help-output test.** The registry contains only Forge-authored commands; the help output contains both. The test filters by name (`completion`, `help`). If Cobra renames its auto-generated commands, the test fails and the fix is a two-line update. The complication is accepted because it is bounded and documented.

- **The command registry does not support dynamically loaded commands.** A plugin system, if introduced, will need a parallel mechanism. The assumption is that no such system is needed in Phases 2–5, and that if it is, it will be the subject of its own ADR.

### 4.3 Neutral

- The framework's shape is orthogonal to the CLI framework's identity. If Cobra is replaced, the `commandConstructor` type's return value changes, but the surrounding structure (one exported symbol, injectable execution, central registry, handler / service boundary) survives.

- The handler / service boundary requires every command to have a service package under `internal/app/<name>/`. A command whose logic is trivial (for example, `forge version`) has a service package that is nearly as simple as the command. The boundary is enforced regardless of the command's size, on the theory that a trivial command today may grow tomorrow, and the boundary's uniformity is worth more than the trivial command's brevity.

- The `Dependencies` struct's six fields are all interfaces or primitives. The struct is deliberately not a nested tree. A future collaborator that is itself a struct (for example, a `Renderer` with configuration) is added as a single field, and its internals are hidden behind the field's type. This keeps the struct's size constant while allowing collaborators to grow internally.

---

## 5. Related Requirements

- `FR-CLI-001` — the CLI must be invocable from a process entry point with a frozen signature (`Execute() int`).
- `FR-CLI-005` — the CLI must support an injectable environment for testing.
- `FR-CLI-009` — the CLI must have a central command registry.
- `FR-CLI-010` — the CLI must not contain business logic in handlers.
- `EXIT-001` — exit codes must be defined as named constants.
- `EXIT-002` — errors must map to exit codes via a single function.
- `EXIT-003` — the exit code contract must be documented.
- `EXIT-004` — failing exit codes must be testable in-process.
- `TEST-003` — CLI command behaviour must be testable without a subprocess.
- `DOC-003` — the architecture document must describe the CLI package boundary.
- `DOC-004` — the development guide must describe how to add a command.

The following requirements are anticipated and must be added to the register when their respective WBS items are complete:

- `FR-CLI-011` — the CLI must support a hidden-command escape hatch for internal and debug commands (WBS 4.4.2).
- `FR-CLI-012` — the CLI's command constructor contract must be frozen and enforced by a structural test (WBS 4.4.2).

---

## 6. Related ADRs

- `ADR-001` — Forge as a foundation manager. This ADR establishes the product context in which the CLI framework is a boundary rather than a container.
- `ADR-002` — Choose Cobra as the CLI framework. This ADR establishes the framework that `internal/cli` imports; ADR-003 assumes it and does not revisit it.
- `ADR-004` — Go version matrix. This ADR constrains the toolchain available to the CLI framework but does not interact with its shape.
- `ADR-005` — Command constructor contract. This ADR is anticipated and will be written as part of WBS 4.4.2. It supersedes the corresponding section of ADR-003 if ADR-003 is accepted before ADR-005 lands; otherwise, ADR-005 is the authoritative reference for the constructor contract and ADR-003 references it.

The relationship with ADR-005 is deliberate: ADR-003 ratifies the framework's shape as a whole, and ADR-005 ratifies the specific contract every command satisfies. The two are complementary. If ADR-005 does not exist when ADR-003 is accepted, ADR-003's § 3 should note that the constructor contract will be frozen by a follow-up ADR.

---

## 7. Notes

**On the relationship between ADR-003 and the WBS items.** The WBS items (4.1.1 through 4.4.2) describe tasks. ADR-003 describes the decision those tasks implement. The two are not redundant: the WBS items are instructions to a contributor; the ADR is a record of why the instructions are what they are. A future contributor who wants to change the framework should read the ADR first, then the WBS items, then the code.

**On the `PersistentPreRunE` hook.** The WBS 4.4.1 description shows a `PersistentPreRunE` field on the root command, with a comment that WBS 8.x will implement `loadConfigIntoDeps`. ADR-003 does not decide whether config loading happens in `PersistentPreRunE` or in `buildDependencies`. That is WBS 8.x's decision, and it will be the subject of its own ADR. The framework's shape accommodates both: `buildDependencies` can call a config loader, or the root command can register a `PersistentPreRunE` hook. The decision is deferred.

**On the placeholder config command.** The WBS 4.4.1 delivery includes a hidden placeholder for `forge config`. The placeholder exists so that the registry has two entries and the ordering convention is visible in code. It will be replaced by the real command in WBS 8.x. ADR-003 does not decide the config command's shape; it decides the framework's shape, and the placeholder is a consequence of that shape, not a commitment to the config command's behaviour.

**On the "one exported symbol" rule.** The rule is enforced for functions and methods but not for constants. The exit-code constants (`ExitSuccess`, `ExitFailure`, `ExitUsage`, and so on) are exported because downstream consumers — including tests and, in principle, shell scripts that read the constants' values — may reference them by name. The rule's intent is that no downstream package depends on the CLI's *functions*; the constants are a stable vocabulary, and their export is deliberate. This is a known nuance, and it is documented in `docs/architecture.md` § 11.

**On the deferred items.** Three items are named in the WBS cluster but not decided by ADR-003: `ForgeError` (WBS 10.0), the config loader (WBS 8.x), and the filesystem boundary enforcement (WBS 13.0). Each will be the subject of its own ADR. ADR-003 assumes they will land and provides the injection points (`Dependencies.Config`, `Dependencies.FS`, error mapping in `exitCodeFromError`) at which they will connect. The framework does not need to be revisited when they land; only the injected values change.

**On the "one exported symbol" and hidden commands.** A hidden command is a command that is registered in `registry.go` but sets `cmd.Hidden = true`. It does not appear in `forge --help` and does not appear in shell completions. The pattern is legitimate for internal and debug commands. ADR-003 does not decide which commands are hidden; it establishes that the escape hatch exists and that hidden commands satisfy the same constructor contract as visible ones.

**On the framework's permanence.** The framework's shape is intended to survive the project's lifetime. Its two boundaries (`options` and `Dependencies`), its central registry, its one exported symbol, and its handler / service separation are structural commitments, not Phase 2 conveniences. A future change to any of them is a change to the CLI's contract with its own codebase, and it should be argued for explicitly in a new ADR that supersedes this one.
