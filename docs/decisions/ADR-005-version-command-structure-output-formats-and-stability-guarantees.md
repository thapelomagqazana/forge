# ADR-005: Version Command Structure, Output Formats, and Stability Guarantees

**Status:** Proposed
**Date:** 2026-10-10
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

The `forge version` command has accumulated four independent requirements across Phase 1 and early Phase 2:

- **WBS 6.1.1** requires a build-metadata model — four values (`Version`, `Commit`, `BuildDate`, `Dirty`) injected at link time — that is not the CLI's concern.
- **WBS 4.3.1** requires a handler / service split for every command: the Cobra handler must not contain business logic, and the service must not depend on Cobra.
- **WBS 6.4.1** requires a byte-for-byte frozen human-readable output format.
- **WBS 6.4.2** requires a frozen JSON schema, selectable via a `--format` flag.

These four requirements interact. The metadata model's location determines where the formatters live; the formatters' location determines what "frozen" can mean; the flag's scope determines how future commands express their own format selections. Getting the answer to any one of them wrong forces rework on the others.

Three concrete problems force a decision now.

First, the metadata cannot live in `internal/cli`. The version service (`internal/app/version`) needs to read the four injected values. If they live in `internal/cli`, then `internal/app/version` must import `internal/cli` — but `internal/cli` already imports `internal/app/version` for the handler. Go rejects the cycle. Some relocation is required.

Second, the command must produce two output formats. Humans read a line-oriented block; scripts need a single-line JSON object. The two formats have different constraints (alignment, line count, field naming) and different stability contracts. Putting both in one place, or splitting them across layers, has consequences for testability and for the change process.

Third, the two formats are contracts with downstream consumers. A script that parses `forge version` today must continue to work tomorrow. The mechanism by which the contract is enforced — a test, a doc section, a review requirement — determines how expensive a future change is.

Without a decision, the four WBS items will each be implemented against a different assumption. The metadata will live somewhere; the formatters will live somewhere; the flag will have some scope; the stability contract will be some shape. The four answers may be individually correct and jointly incoherent. Fixing that after three or four commands have copied the pattern is more expensive than fixing it now.

The audience for this decision is:

- Contributors writing new commands who will copy the version command's structure.
- Reviewers who will enforce the handler / service boundary on those new commands.
- Consumers — scripts, CI systems, future tooling — who parse the version output and depend on its stability.
- The maintainers who will authorise or reject future format changes.

---

## 2. Options Considered

### Option A — Everything in `internal/cli`

Put the metadata variables, the formatters, and the handler in `internal/cli/version.go`. This is the shape a naive implementation takes.

**Pros:**

- One file. Easy to find.
- No import cycles, because there is nothing to cycle.
- No new packages. Lowest ceremony for a command whose output is four fields.

**Cons:**

- The metadata variables become part of `internal/cli`'s public surface. WBS 4.1.1 freezes that surface to one exported symbol; adding four `var`s and a `Get` function widens it.
- The service (`internal/app/version`) cannot exist, because it would need to import the variables from `internal/cli`, and `internal/cli` imports the service for the handler. Cycle.
- Formatting lives in the handler, which violates WBS 4.3.1.
- No place for a future JSON formatter to live without further widening `internal/cli`.
- The four metadata values are the future input to SBOM and SARIF emitters, which cannot import `internal/cli`. They would have to re-read the same linker variables by duplicating the `-X` prefix, which drifts.

### Option B — Metadata in a leaf package, formatters in the CLI handler

Relocate the four metadata variables to `internal/version` (a leaf package). Keep the formatters in `internal/cli/version.go`, alongside the handler.

**Pros:**

- Breaks the import cycle. Both `internal/cli` and `internal/app/version` (if it existed) could import `internal/version`.
- Metadata is reusable by future SBOM and SARIF emitters without importing the CLI.

**Cons:**

- Formatting still lives in the handler. WBS 4.3.1's boundary is violated.
- The service package (`internal/app/version`) has no reason to exist; the handler reads and formats directly. The pattern every future command should copy does not exist.
- Structural tests for the handler would have to permit formatting calls, weakening the boundary enforcement.
- A future JSON formatter would also live in the handler, growing it.

### Option C — Metadata in a leaf package, formatters in the service, handler as a dispatcher

Relocate the four metadata variables to `internal/version`. Put the formatters in a new package `internal/app/version`. Make the handler in `internal/cli/version.go` a thin dispatcher that parses the `--format` flag, calls the service, and returns the error.

Three sub-options for the format dispatch:

- **C1: The handler switches on the format name.** The handler contains a `switch` on the flag value and calls the appropriate formatter. The service exposes two functions, `Format` and `WriteJSON`.
- **C2: The service switches on the format name.** The handler passes the format value to `FormatAs(w, info, format)`, which routes to the correct formatter. The service exposes `Format`, `WriteJSON`, and `FormatAs`.
- **C3: The service exposes a `Formatter` interface.** The handler constructs a `Formatter` from the flag value via a factory and calls `Formatter.Format(w, info)`. The service exposes the interface, two implementations, and a factory.

**Pros (all three sub-options):**

- The import cycle is broken.
- WBS 4.3.1's handler / service boundary is respected: the handler parses and delegates, the service formats.
- The service is testable without Cobra.
- The pattern is available for future commands to copy.

**Cons (all three sub-options):**

- Three packages instead of one. More files, more imports.
- The formatter's location is now a separate concept from the handler's location; a contributor must know the boundary.
- The value set is owned by the service, so adding a format requires touching the service even if the handler never changes.

**Additional cons:**

- **C1:** The handler contains the value set. Adding a format requires editing the handler, which is the boundary the decision is trying to protect.
- **C2:** The service contains the value set and the dispatch. The handler is a single call to `FormatAs`. The pattern is clean, but the service owns a small `switch` that must stay in sync with the constants.
- **C3:** The `Formatter` interface adds a type for two implementations. For two formats, the interface is more ceremony than the problem warrants. The factory is a second place to update when a format is added.

### Option D — Global `--format` flag

Register `--format` as a persistent flag on the root command, alongside `--verbose`, `--quiet`, and `--config`.

**Pros:**

- One place to define the flag.
- Consistent syntax across all commands: `forge --format json version` works the same as `forge --format json check`.

**Cons:**

- Different commands have different value sets. `forge version` supports `text` and `json`; `forge new` will support `human` and `json`; `forge check` will additionally support `sarif`. A global flag cannot express three different value sets.
- Different commands have different defaults. `forge version` defaults to `text`; `forge new` will default to `human`. A global flag has one default.
- A global flag forces every command to accept every value, even if the command has no meaning for that value. `forge version --format sarif` would have to be rejected per-command, which reintroduces the per-command value set the global flag was supposed to avoid.
- The global flag inventory (WBS 5.3.1) is frozen at three flags; adding a fourth requires an ADR. This ADR would have to make the case for a global flag that no command can meaningfully interpret globally.

### Option E — Separate subcommands (`forge version` and `forge version json`)

Instead of a flag, use two subcommands. `forge version` prints text; `forge version json` prints JSON.

**Pros:**

- No flag parsing. Each subcommand has one job.
- Discovery is trivial: `forge version --help` lists the two subcommands.

**Cons:**

- Cobra's help for `forge version` would list `json` as a subcommand, which reads oddly for a command that is not a group.
- `forge version --version` semantics are unclear (does the parent accept a version flag?).
- The pattern does not generalise. `forge check` cannot reasonably have subcommands `human`, `json`, and `sarif`; it should have a `--format` flag. The version command adopting a different pattern from every other command makes the codebase less consistent, not more.
- Scripts that currently run `forge version` would continue to work, but `forge version json` is a different shape from `forge version --format json`, and future commands would have to choose which shape to follow.

### Option F — JSON schema carries a `schemaVersion` field

Add a `"schemaVersion": "1"` field to the JSON object, matching the general rule in § 8.1 of the CLI UX spec.

**Pros:**

- Consistent with every other JSON-emitting command.
- A consumer can check the version before parsing the four fields.

**Cons:**

- The version command's JSON schema is four fields. The `schemaVersion` field would be the fifth and would always be `"1"` for Phase 2. It carries no information the field names do not already carry.
- If the schema grows to more fields, or a second JSON-emitting mode is added, `schemaVersion` becomes meaningful. Until then it is ceremony.
- Adding it now means every future contributor to the schema must remember to update the field's value if and only if the change is breaking. The field itself becomes a source of errors (a contributor changes a field name but forgets to bump `schemaVersion`).

---

## 3. Decision

**Option C2 is adopted.** The version command is structured as three layers — a thin Cobra handler in `internal/cli/version.go`, an application service in `internal/app/version`, and a leaf metadata model in `internal/version`. The service owns both the value set (`FormatText`, `FormatJSON`) and the dispatch (`FormatAs`). The handler parses the per-command `--format` flag, passes the value to `FormatAs`, and returns the error unchanged (except for wrapping `ErrUnknownFormat` into a user-facing message). Both output formats are frozen: the text format byte-for-byte, the JSON schema field-by-field. A change to either requires a new ADR.

The reasoning, in order of how much it tipped the balance:

**The import cycle is not negotiable.** Option A cannot work: Go rejects the cycle, and no amount of "treat the values as internal" avoids the fact that either the import exists or it does not. The metadata must live somewhere both `internal/cli` and `internal/app/version` can import without a cycle. The only such place is a leaf package. This eliminates Option A outright.

**The handler must not format.** WBS 4.3.1 establishes the boundary and designates the version command as the reference implementation. Option B places the formatters in the handler, which is exactly what the boundary exists to prevent. If the reference implementation violates the boundary, the boundary does not exist in practice, and every future command will negotiate it anew. Option B is eliminated on this ground.

**The service should own the dispatch, not the handler.** Among the Option C sub-options, C1 puts the value set in the handler, which is the same boundary problem as Option B, just smaller. C3 introduces an interface for two implementations, which is more abstraction than the problem warrants in Phase 2. C2 keeps the handler a dispatcher and puts the value set and the switch in the service. The service is the natural owner of "what formats exist"; the handler is the natural owner of "which flags were passed". C2 is the cleanest split.

**`--format` must be per-command.** Option D would force every command to interpret one value set globally, but the value sets differ: `text` here, `human` for the creation commands, `sarif` for `forge check` later. A global flag cannot express three value sets without becoming per-command in disguise. Option D is eliminated.

**Subcommands do not generalise.** Option E is fine for a command with two modes, but `forge check` cannot have three subcommands `human`, `json`, and `sarif`. The pattern for expressing output-format selection should be consistent across the CLI. `--format <value>` is that pattern. Option E is eliminated on consistency grounds.

**The JSON schema does not need `schemaVersion`.** Option F trades a small gain (consistency with a general rule) for a small cost (a field that is always `"1"`). The tradeoff is genuinely close, and the decision could go either way. The tiebreaker is the cost of drift: a `schemaVersion` field requires discipline to bump it exactly when a breaking change occurs. The version schema is four fields whose names *are* the schema; a rename of any of them is visible in the diff, and a consumer will notice. The absence of `schemaVersion` is recorded in the CLI UX spec § 8.1 as a deliberate exception. If the schema grows beyond four fields, or a second JSON-emitting mode is added, the exception is revisited and a `schemaVersion` field is added at that point.

**The two formats are frozen.** Once the structure and the value set are settled, the formats themselves become contracts. The text format is byte-for-byte frozen by WBS 6.4.1; the JSON schema is field-name-by-field-name frozen by WBS 6.4.2. Both freezes are enforced by tests (byte-for-byte for text, field names and types for JSON) and by documentation (a section of `docs/cli-ux-spec.md` for each). A change requires all three updates in the same commit.

**Assumptions this decision depends on:**

1. No external consumer parses `forge version` in a way that would break if a new field is added to the JSON schema. Adding a field is additive and does not require an ADR; only renaming or removing does.
2. The service does not need to import anything from `internal/cli`. If a future feature requires the service to know about a CLI concept (for example, a `--verbose` flag that changes the output), the boundary must be revisited.
3. The `ErrUnknownFormat` sentinel remains the service's way of signalling "this format value is not one I support". If the error handling grows to three or four distinguishable service errors, the handler's `errors.Is` chain grows, and at some point the error-to-message mapping should move into the service. That point is not now.

**This decision is provisional in one respect:** the pattern it establishes for the version command is expected to be copied by every future command. If the pattern proves awkward for a command whose output has more structure (for example, `forge check` with its SARIF output), the pattern may be revised. The revision would be a new ADR that supersedes this one.

---

## 4. Consequences

### 4.1 Positive

- **The import cycle is broken.** `internal/version` is a leaf package that imports nothing from Forge. Both `internal/cli` and `internal/app/version` can import it.
- **The handler / service boundary is respected.** The handler is under ten lines. It parses the flag, calls the service, and returns an error. The boundary is enforceable by tests that read the handler's source.
- **The service is testable without Cobra.** `internal/app/version`'s tests construct `Info` values and `bytes.Buffer` writers. No Cobra command is constructed; no flags are parsed. The tests are fast and hermetic.
- **The pattern is copyable.** A future command's author can read `internal/cli/version.go`, `internal/app/version/service.go`, and `internal/app/version/format.go` and see the shape. The shape is the reference implementation of the handler / service boundary.
- **The metadata is reusable.** Future SBOM and SARIF emitters will import `internal/version` directly. They will not duplicate the `-X` prefix and will not depend on `internal/cli`.
- **The value set is per-command.** `forge version` accepts `text` and `json`. Future commands accept their own value sets. Each command's flags document its own vocabulary, and adding a value to one command does not force it on others.
- **The stability contract is enforceable.** The text format is pinned by a byte-for-byte test and three golden files. The JSON schema is pinned by a field-name test and three golden files. A change to either fails a test, and the failure message names the file the contributor must update.
- **The change process is documented.** `docs/cli-ux-spec.md` § 4.8.4 lists the files that must change together for a format change. A reviewer who sees one without the others rejects the PR.

### 4.2 Negative

- **Three packages where one would do.** `internal/cli`, `internal/app/version`, and `internal/version` are three import paths for a command whose total logic is about sixty lines. A contributor must know the boundary to know where to add code. The boundary is documented, but the documentation is a cost.
- **The handler's error path inspects the service.** The handler checks `errors.Is(err, ErrUnknownFormat)` to distinguish "unknown format" (a usage error) from "write failed" (a general error). This is a small coupling of the handler to the service's error taxonomy. If the service grows more distinguishable errors, the handler grows more branches, and at some point the boundary strains.
- **The value set is duplicated in two places.** `FormatText` and `FormatJSON` are constants in the service; the CLI UX spec lists them in a table. Keeping the two in sync is a manual process, checked by review. If the constants and the doc drift, the doc is wrong until a reviewer notices.
- **The JSON schema is not `schemaVersion`-tagged.** A consumer that reads the schema must infer its version from the field names. This is deliberate, but it means the version command's JSON is a special case relative to the future `forge check` JSON, which will have a `schemaVersion` field. Contributors must remember the distinction.
- **Adding a third format touches three files.** The service's constants, the service's dispatch, and the CLI UX spec's per-command table. Adding a flag value in a single-file design would touch one file. The boundary's benefit is testability; its cost is this kind of multi-file edit.
- **The `--format` flag is not the same across commands.** A user who learns `--format json` on `forge version` will be surprised that `forge new` accepts `human` as well. The values differ because the outputs differ: the version output is machine-oriented (`text`), the creation output is prose-oriented (`human`). A user who wants a single mental model must consult the help for each command.

### 4.3 Neutral

- **The version command now has two output formats, not one.** Previously it had a single text format. The JSON format is additive; no consumer is broken by its existence. But the command's surface is now larger: two formats, one flag, one dispatch, one sentinel error. Whether that counts as "the command grew" or "the command acquired the capability it needed" depends on the reader.
- **The `internal/version` package name is generic.** It holds four build-metadata values. A reader seeing `internal/version` might expect a versioning library. The package name is accurate for the domain (`version` is what the command prints), but the name collides conceptually with the more general "version" of a software project. The collision is acceptable because the package's exported symbols (`Version`, `Commit`, `BuildDate`, `Dirty`, `Get`) are unambiguous in context.
- **The stability contract is per-format, not per-command.** The text format's change process is separate from the JSON format's. A change that affects both must follow both processes. In practice, a change that affects both is rare, because the two formats present different subsets of the same data.
- **The `--version` flag is not `--format`-aware.** A user cannot run `forge --version --format json`; the flag has no format argument. This is a limitation of Cobra's built-in version flag, not a design choice. A consumer who wants the JSON version output runs `forge version --format json`. The two invocations produce different outputs for the same "how do I ask about the version?" question, which is a minor inconsistency users will encounter.

---

## 5. Related Requirements

- `FR-CLI-003` — `forge version` command exists and prints build metadata.
- `FR-CLI-007` — `--format` flag selects between text and JSON output.
- `FR-CLI-010` — stdout/stderr separation for the version command's output.
- `EXIT-004` — extra arguments to `forge version` exit with `ExitUsage`.

The following requirements are established by this decision but must be added to the register when WBS 1.4 is complete:

- `FR-CLI-014` — the version command's text format is byte-for-byte frozen; changes require an ADR.
- `FR-CLI-015` — the version command's JSON schema is field-name frozen; changes require an ADR.
- `NFR-CLI-003` — the version command's text output is exactly six lines; the JSON output is exactly one line.
- `UX-CLI-002` — the version command's `--format` value set is per-command, not global.

---

## 6. Related ADRs

- `ADR-001` — Forge is a foundation manager. Establishes the product's domain; the version command is a supporting command, not a foundation operation. This ADR does not supersede or modify ADR-001.
- `ADR-002` — Choose Cobra as the CLI framework. Establishes the command tree and flag parsing. This ADR depends on Cobra's ability to register per-command flags (`cmd.Flags().StringVar`) and to override the version template (`root.SetVersionTemplate`). Both are used.
- `ADR-003` — CLI framework foundation package execution, injection, registry, and contract. Establishes the `Execute` / `executeWithOptions` split and the `Dependencies` struct. This ADR depends on `Dependencies.Stdout` being the writer the handler uses; the handler must not write to `os.Stdout`.
- `ADR-004` — Root command identity and behaviour contracts. Establishes the four identity constants and the `--version` flag's wiring. This ADR's § 4.8 documents the format that `--version` produces; the two ADRs share the format as a joint contract.

A future ADR will be required if any of the following occurs:

- The text format changes (a field is added, reordered, or reformatted).
- The JSON schema changes in a breaking way (a field is renamed or removed).
- The handler / service boundary is revised (for example, because a future command needs a different split).
- The `--format` value set becomes global.

---

## 7. Notes

**Open questions deferred by this ADR:**

1. **Should `forge --version` accept a `--format` argument?** Cobra's built-in version flag does not accept arguments. A user who wants the JSON version output runs `forge version --format json`. If this becomes a usability problem, a future ADR could replace Cobra's version flag with a custom one that accepts `--format`.

2. **Should the version command support a `--format json-pretty`?** The single-line JSON is intended for machine consumption. A human who wants readable JSON runs `forge version --format json | jq .`. A `json-pretty` value is an additive change and would require a new constant, a new case in `FormatAs`, a new formatter, and a new section in the CLI UX spec. It is not in scope for Phase 2.

3. **Should the JSON schema carry a `schemaVersion` field?** The current decision is no, because the schema is four fields and its field names identify it. The decision should be revisited if the schema grows beyond four fields or if a second JSON-emitting mode is added (for example, a `--format json-full` that includes additional diagnostic fields).

4. **How should `forge check` (Phase 7) structure its output formats?** The pattern established here — per-command `--format`, service-owned value set, frozen schema — is expected to apply. `forge check` will additionally need SARIF, which is a substantial format with its own specification. A future ADR will address whether SARIF lives in the same dispatch or in a separate code path.

5. **Does the service's `ErrUnknownFormat` scale to more error types?** If a third or fourth distinguishable error appears in the service, the handler's `errors.Is` chain grows, and the boundary may need a revision. The point at which this becomes a problem is not yet reached; the current single error is a small coupling.

**Links:**

- WBS 6.1.1 — the build-metadata model.
- WBS 6.3.1 — the command handler and the service.
- WBS 6.4.1 — the frozen text format.
- WBS 6.4.2 — the frozen JSON schema and the `--format` flag.
- `docs/architecture.md` § "Handler / Service Boundary" — the pattern this ADR applies.
- `docs/cli-ux-spec.md` § 4.8 — the frozen formats.
- `docs/development.md` § "Build Metadata Injection" — the linker injection contract that supplies the metadata the command prints.
