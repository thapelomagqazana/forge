# ADR-007: Configuration Foundation — Precedence, Discovery, and Error Taxonomy

**Status:** Proposed
**Date:** 2026-10-11
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge is a CLI for defining, generating, validating, and evolving software project foundations. Its behaviour depends on a small set of user-facing preferences: the log level, the output format, and (in future phases) the choice of a foundation or template. Those preferences can be supplied from several sources: a built-in default, a config file, an environment variable, or a command-line flag. A user who runs `forge version` in a fresh checkout, a user who runs `forge --log-level=debug check` on a CI runner, and a user who commits a `forge.yaml` next to their source tree all expect their preference to be honoured. The precedence between the sources is not obvious from the command line, and without a rule the CLI's behaviour would depend on the order in which the sources happen to be read.

Three problems force a decision now, before the first non-trivial command is implemented.

**First, precedence must be frozen.** The CLI reads four sources for the same field. If the rule for which source wins is not written down, the rule will be chosen implicitly by the order in which the code happens to read the sources. The implicit rule will be discovered by users through trial and error, and changing it later is a breaking change. The precedence chain must be a deliberate, documented decision.

**Second, discovery must be canonical.** A user who types `forge --config /path/to/forge.yaml` names a specific file; the CLI must load it, and must fail loudly if it does not exist. A user who types nothing names no file; the CLI must search a fixed, documented set of locations, and must fall back to the defaults if none exists. The two cases are different and must be distinguished by the discovery algorithm. Without a canonical algorithm, each command would search its own paths, and the CLI's behaviour would depend on which command ran first.

**Third, failures must be classifiable.** Configuration failures have several distinct causes: a missing file, a permission error, a malformed YAML document, an unknown field, a type mismatch, an invalid enum value, an over-sized file, an invalid encoding. A script that automates Forge needs to branch on the cause; a user who reads the diagnostic needs a message that names the cause and a suggestion that tells them what to do. A single generic "config error" satisfies neither. The taxonomy of categories, the messages, and the suggestions must be a deliberate, documented decision.

The decision is constrained by several requirements and prior specifications:

- **WBS 4.1.2** freezes the CLI's exit codes. Configuration failures must map to a specific code in that set.
- **WBS 7.3.2** freezes the error message format: a first line prefixed with `Error:`, optional context lines indented seven spaces, and an optional `Suggestion:` block. Configuration errors must render in that format.
- **WBS 7.4.1** freezes the stream boundary: results on stdout, diagnostics on stderr.
- **WBS 7.4.2** freezes the ASCII-only and no-colour policy: every byte in the CLI's output is ASCII-printable or a permitted control byte.
- **WBS 2.4.1** freezes the Cobra import rule: Cobra may be imported only under `internal/cli`. The configuration package must therefore be free of the CLI framework.
- **WBS 4.2.2** freezes the two-boundary model: process-level inputs are resolved into command-level collaborators by a single transformation.

Without a decision, the four sources would each be read by a different code path, the discovery algorithm would be undefined, and the error categories would be invented per call site. The cost of the ad-hoc design would be paid when a future contributor tried to add a fifth source or a ninth error category and found no coherent place for them.

The decision affects:

- **Users** who configure Forge and expect the configuration to be honoured.
- **Scripts and CI pipelines** that branch on Forge's exit codes and parse its diagnostics.
- **Contributors** who add a new configuration field, a new source, or a new error category.
- **Reviewers** who assess a change to the configuration package.

---

## 2. Options Considered

### Option A — A single config file, no precedence chain

The CLI reads one config file — `forge.yaml` in the current directory — and does not read the environment or the flags for configuration. The file's values are the configuration; the absence of the file means the defaults.

**Pros:**

- The simplest model. One source, one place to look.
- The precedence question does not arise: there is only one source, so there is nothing to win or lose.
- The discovery algorithm is trivial: one path.
- The error taxonomy is smaller: a missing file, a malformed file, an invalid value.

**Cons:**

- No way to override a setting for a single invocation. A user who wants `--log-level=debug` for one command must edit the file.
- No way for CI to supply a setting without writing a file. A CI runner that wants a specific log level must create a config file for every job.
- No way to have per-user and per-project settings differ. The file is either per-project or per-user, not both.
- A future addition (a second source) is a breaking change to the model, not an extension.

### Option B — A four-layer precedence chain, a canonical discovery algorithm, and an eight-category error taxonomy

The CLI reads four sources in ascending order of precedence: defaults, config file, environment variables, CLI flags. The config file is discovered by a canonical algorithm with two explicit sources (`--config`, `FORGE_CONFIG`) and four optional sources (project-local `forge.yaml`, legacy `.forge/config.yaml`, `$XDG_CONFIG_HOME/forge/config.yaml`, `$HOME/.config/forge/config.yaml`, plus `%APPDATA%\forge\config.yaml` on Windows). Configuration failures are classified into eight categories, each with a sentinel, a constructor, a message, and a suggestion.

**Pros:**

- Users can override a setting for a single invocation without editing a file.
- CI can supply settings through the environment or the flags, without writing files.
- Per-project and per-user settings can differ; the layers compose.
- The discovery algorithm is canonical and documented; a user knows exactly where Forge looks.
- The taxonomy is closed; a script can branch on the error's category.
- The precedence chain matches the convention used by `git`, `docker`, `kubectl`, and most modern CLIs.

**Cons:**

- The precedence rule is a documented contract; changing it later is a breaking change.
- The discovery algorithm searches several paths, which is more code than a single path.
- The taxonomy has eight categories, each requiring a constructor, a message, and a suggestion.
- A user who does not read the documentation may be surprised by which source won. The `forge config --show-source` command (a future WBS item) is the mitigation.

### Option C — A two-layer chain: defaults and flags only

The CLI reads only the built-in defaults and the command-line flags. There is no config file and no environment variable.

**Pros:**

- The simplest chain that allows per-invocation overrides.
- No file discovery, no file I/O, no file errors.
- The error taxonomy is trivial: an invalid flag value is a usage error.

**Cons:**

- No persistent configuration. A user who wants `log_level: debug` for every invocation must type the flag every time.
- No per-project configuration. A project's preferences cannot be committed.
- No way for CI to supply a setting without a flag on every command.
- A future addition (a config file) is a breaking change to the model.

### Option D — A four-layer chain, but the precedence is configurable

The CLI reads the four sources, but the order is configurable: a user can specify whether the environment overrides the file or the file overrides the environment.

**Pros:**

- Maximum flexibility. A user who prefers a different order can choose it.

**Cons:**

- Every user must learn the order and decide it. The decision is not the CLI's to make.
- Two invocations with the same sources can behave differently if the order is configured differently.
- The precedence rule is no longer a contract; scripts cannot rely on it.
- The configuration of the precedence is itself a configuration, which must be read from one of the sources — an infinite regress.

### Option E — No chain, no discovery: a single environment variable

The CLI reads its configuration from a single environment variable (`FORGE_CONFIG_JSON`), which contains a JSON document with all settings.

**Pros:**

- One source. No file discovery, no precedence chain.
- The environment variable can be set per invocation, per shell, or globally.
- No file I/O, no file errors.

**Cons:**

- A JSON document in an environment variable is awkward to write and to read.
- No per-project configuration without a shell wrapper.
- The environment variable's value is opaque to `git`, `cat`, and other tools.
- A future addition (a config file) is a breaking change.

---

## 3. Decision

**Option B is adopted.** The configuration chain has four layers, applied in ascending order of precedence: defaults, config file, environment variables, CLI flags. Each layer's non-empty values override the layer below. The config file is discovered by a canonical algorithm with two explicit sources (the `--config` flag and the `FORGE_CONFIG` environment variable) that error on a missing file, and four optional sources (canonical project-local `forge.yaml`, legacy `.forge/config.yaml`, `$XDG_CONFIG_HOME/forge/config.yaml`, `$HOME/.config/forge/config.yaml`, plus `%APPDATA%\forge\config.yaml` on Windows) that are silent on a missing file. Configuration failures are classified into eight categories, each with a sentinel, a constructor, a message, and a suggestion; every category is classified by the single category string `"config"` and mapped to `ExitConfig` (3).

The reasoning, in order of how much it tipped the balance:

**The four-layer chain is the CLI convention.** Every modern CLI that has more than one configuration source uses a precedence chain. Users who have used `git`, `docker`, or `kubectl` expect a flag to override an environment variable to override a file to override a default. Adopting the convention means users do not have to learn Forge's rule; they already know it. Option A (single file) and Option C (defaults and flags only) reject the convention, and their users would be surprised by the absence of the other sources.

**The precedence order is the one users expect.** The order "flag wins over env wins over file wins over default" is the order every user expects. The reverse order (default wins over file wins over env wins over flag) would make the flag useless. Option D (configurable precedence) would make the order a per-user decision, which is not a decision most users want to make, and which would break scripts that rely on the order.

**The explicit-versus-optional distinction is the right one.** A user who types `forge --config /path/to/forge.yaml` expects that file to be loaded; if it does not exist, the user has made a mistake, and silently falling back to a different file would hide the mistake. A user who types nothing has not named a file; the algorithm's optional sources are defaults, and a missing file in a default source is normal. The distinction is the algorithm's most important rule.

**The canonical project-local path is `forge.yaml`.** The file's name matches the project's convention (the spec document is `docs/forge-yaml-spec.md`) and the CLI's own convention. The alternative — `.forge/config.yaml` — is the legacy path, retained for a one-release compatibility shim. The canonical path is at the project root, where a user can see it; the hidden directory is for state, not for configuration.

**The eight categories are the minimum for a script to branch on.** A missing file is different from a malformed file; a permission error is different from a size error. A user who reads the diagnostic needs a message that names the cause; a script that automates Forge needs a category that distinguishes the causes. Option A (a single generic error) satisfies neither; Option B's eight categories satisfy both.

**The single category string is the right granularity.** The CLI's classifier reads the error's `Category()` method. All eight categories return `"config"`, so the classifier has one case for the whole subsystem. A future WBS item that splits the exit codes (for example, a distinct code for a syntax error) changes the `Category()` methods, not the classifier.

**The config package is a leaf.** The package must be testable and reusable in isolation. It must not import Cobra (WBS 2.4.1) and must not import any Forge package other than its own subpackages. The flag reader is a port in the config package and an adapter in the CLI package; the port/adapter arrangement keeps the dependency direction correct.

**The lifecycle is uniform.** The config is loaded exactly once per invocation, in the root command's `PersistentPreRunE` hook. The resolved config is stored in the command's context and read by the dispatched command's `RunE`. Option B's uniform lifecycle means every command reads the same configuration from the same place.

**Assumptions this decision depends on:**

1. A user who does not read the documentation will expect the four-layer chain. The assumption is safe because the chain is the convention.
2. The optional sources do not produce false positives. The assumption is that a user who happens to have a `forge.yaml` in their home directory does not accidentally configure Forge; the source is at `$XDG_CONFIG_HOME/forge/config.yaml` or `$HOME/.config/forge/config.yaml`, both of which name Forge explicitly.
3. The eight categories cover all current failures. The assumption is that a future failure (for example, a file whose content is valid YAML but not a mapping) is either classified as one of the eight or requires a new category via an ADR.
4. The `FORGE_CONFIG` environment variable is the only env var that names a config path. A future variable (for example, `FORGE_LOG_LEVEL`) is a new source in the env layer, not a new discovery path.

**This decision is provisional in one respect:** the taxonomy's `ErrConfigInvalid` sentinel covers E6 (an invalid enum value). A future WBS item can split E6 into two codes (one for the value, one for the file's content being valid YAML but not a mapping) without changing the CLI's exit code. The split is a category addition, not a breaking change.

---

## 4. Consequences

### 4.1 Positive

- **The precedence chain is documented and stable.** A user who reads `docs/cli-ux-spec.md` knows exactly which source wins. A script that relies on the order can do so without defensive checks.
- **The discovery algorithm is canonical.** Every command uses the same algorithm; a user knows exactly where Forge looks.
- **The error taxonomy is closed.** A script can branch on the error's category; the classifier's `"config"` case handles all eight codes. A future category addition is a sentinel and a constructor, not a change to the classifier.
- **The config package is a leaf.** It imports nothing from Forge except its own subpackages. It has no Cobra dependency. It is testable in isolation and reusable in any future tool.
- **The error messages and suggestions are uniform.** Every config error renders in the frozen format (WBS 7.3.2), with the same prefix, the same context lines, and the same "Suggestion:" block.
- **The explicit-versus-optional distinction is loud.** A user who names a config file and gets it wrong sees an error; a user who relies on discovery and has no file sees the defaults. The two cases are distinguishable by their exit codes (3 versus 0).
- **The lifecycle is uniform.** Every command reads the resolved config through the same accessor (`ConfigFromContext`). A future command's `RunE` handler reads the config the same way.

### 4.2 Negative

- **A user who does not read the documentation may be surprised by which source won.** The `forge config --show-source` command (a future WBS item) is the mitigation; until it exists, the surprise is a documented possibility.
- **The discovery algorithm searches several paths, which is more code than a single path.** The algorithm's tests cover each path; the code is more than Option A's.
- **The precedence rule is a contract; changing it later is a breaking change.** A user who relies on "flag wins over env" cannot be protected from a future change to the rule. The ADR is the record of the current rule; a change requires a superseding ADR.
- **The taxonomy has eight categories.** Each requires a sentinel, a constructor, a message, and a suggestion. The code is larger than Option A's single error.
- **The `FORGE_CONFIG` environment variable is not consulted at discovery if the `--config` flag is set.** A user who sets both and expects the flag to be ignored is surprised; the flag wins. The precedence rule says the flag is the highest-priority source; the discovery algorithm follows it.
- **The `.forge/config.yaml` legacy path is retained.** A user who has both `forge.yaml` and `.forge/config.yaml` loads the canonical one; the legacy file is silently ignored. The silent ignore is a source of confusion for the one-release window the legacy path exists.
- **A configuration error is a hard failure, not a warning.** A user who types `forge --config /typo/forge.yaml version` sees an error and exit code 3, not a warning and exit code 0. The behaviour is deliberate (see Option A's cons); the surprise is real for a user who expected the flag to be ignored when the file does not exist.

### 4.3 Neutral

- **The configuration subsystem is a new package.** Its presence is a change to the repository's structure; the change is neither positive nor negative on its own.
- **The lifecycle integration (WBS 8.4.5) adds a `PersistentPreRunE` hook to the root command.** The hook was a no-op stub before; the integration gives it behaviour. The change is the intended one; its side effects (config errors are now fatal) are documented.
- **The `ConfigFromContext` accessor is a second exported function from the CLI package.** The package's policy was "one exported function"; the accessor widens it. The widening is documented in the structural test's allowlist; the alternative (unexporting the accessor and re-exporting it when a service package needs it) is a deferred decision, not a resolved one.
- **The taxonomy's eight categories use the E1 through E8 codes.** The codes are internal; the CLI's exit code is 3 for all of them. A future change to the codes is not a change to the CLI's surface.

---

## 5. Related Requirements

- `FR-CFG-001` — The CLI reads its configuration from a precedence chain. Established by this ADR.
- `FR-CFG-002` — The configuration file is discovered by a canonical algorithm. Established by this ADR.
- `FR-CFG-003` — The CLI classifies configuration failures into distinct categories. Established by this ADR.
- `FR-CLI-004` — Configuration errors are rendered in the frozen error format. Constrained by WBS 7.3.2.
- `EXIT-002` — Configuration failures map to `ExitConfig` (3). Constrained by WBS 4.1.2.
- `ERR-002` — Configuration errors carry a category and a suggestion. Established by this ADR.
- `ERR-004` — Configuration errors are rendered with a suggestion block. Constrained by WBS 7.3.2.
- `LOG-004` — Diagnostics go to stderr, results to stdout. Constrained by WBS 7.4.1.
- `DOC-003` — The configuration subsystem is documented in `docs/cli-ux-spec.md`. Constrained by this ADR's documentation requirement.

The following requirements must be added to the register when WBS 1.4 is complete:

- `FR-CFG-004` — The explicit-versus-optional distinction for discovery sources. Established by this ADR.
- `FR-CFG-005` — The taxonomy is closed at eight categories; a ninth requires an ADR.
- `NFR-CFG-001` — The config package is a leaf with no Forge or Cobra dependencies.
- `NFR-CFG-002` — The configuration is loaded exactly once per invocation.

---

## 6. Related ADRs

- `ADR-001` — Forge is a foundation manager. The configuration subsystem is a supporting capability, not a foundation operation. This ADR does not supersede or modify ADR-001.
- `ADR-002` — Choose Cobra as the CLI framework. Cobra's `PersistentPreRunE` hook and `--config` flag are the mechanisms this ADR depends on. This ADR does not supersede ADR-002.
- `ADR-003` — CLI framework foundation package execution, injection, registry, and contract. The `Dependencies` struct is the command boundary the config package's values reach. This ADR depends on ADR-003's `Dependencies` shape; a future change to that shape affects this ADR's lifecycle.
- `ADR-004` — Root command identity and behaviour contracts. The root command's identity strings and flag inventory are not changed by this ADR. The `--config` flag is part of ADR-004's inventory; this ADR defines its semantics.
- `ADR-005` — Version command structure, output formats, and stability guarantees. The version command is the reference implementation of the handler/service pattern; the config package's lifecycle (WBS 8.4.5) uses the same pattern. This ADR is independent of ADR-005.
- `ADR-006` — Help and CLI UX contracts: invocation matrix, output formats, stream boundary, and Phase 2 presentation policy. The help contract and the stream boundary constrain this ADR's error output. This ADR does not supersede ADR-006.

A future ADR will be required if any of the following occurs:

- The precedence chain's order changes.
- The discovery algorithm's canonical path changes.
- The error taxonomy's category count changes (a ninth category is added, or one is removed).
- The config package acquires a dependency on Cobra or on another Forge package.
- The lifecycle changes (for example, the config is loaded per-command rather than once in the root hook).

---

## 7. Notes

**Open questions deferred by this ADR:**

1. **Should the `--config` flag be honoured in the presence of `--help`?** In Phase 2, Cobra's `--help` interception short-circuits the hook chain, so the `--config` flag's value is not read and the invocation succeeds. A future WBS item could change the behaviour (for example, by validating the flag before Cobra parses). The change requires a superseding ADR.

2. **Should `FORGE_CONFIG` and `--config` be allowed together?** In Phase 2, the flag wins: if `--config` is set, `FORGE_CONFIG` is not consulted. A future WBS item could reject the combination as a usage error. The change is a new pre-parse rejection, not a change to this ADR.

3. **Should the legacy `.forge/config.yaml` path be removed in a future release?** The path exists for a one-release compatibility shim. A future WBS item removes it once no project needs it. The removal is a change to the discovery algorithm's optional sources, not a change to the precedence chain or the taxonomy.

4. **Should `forge config --show-source` be added?** The command would display which source won for each field. It is the mitigation for the "surprising source" downside. A future WBS item adds the command.

5. **Should the taxonomy's `ErrConfigInvalid` sentinel be split?** The sentinel covers E6 (an invalid enum value). A future WBS item can split it into two codes (one for the value, one for a valid YAML document that is not a mapping) without changing the CLI's exit code.

**Links:**

- `docs/cli-ux-spec.md` § "Configuration Loading Lifecycle" — the reader-facing description.
- `docs/cli-ux-spec.md` § "Config Error Reference" — the eight-category table.
- `docs/forge-yaml-spec.md` — the config file's schema.
- `docs/architecture.md` — the two-boundary model and the handler/service boundary that this ADR's lifecycle depends on.
- WBS 8.1.2 — the precedence chain.
- WBS 8.4.1 — the discovery algorithm.
- WBS 8.4.2 — the loader.
- WBS 8.4.3 — the environment layer.
- WBS 8.4.4 — the flag layer.
- WBS 8.4.5 — the lifecycle integration.
- WBS 8.5.1 — the error taxonomy.
- WBS 7.3.2 — the error message format.
- WBS 7.4.1 — the stream boundary.
- WBS 2.4.1 — the Cobra import rule.
