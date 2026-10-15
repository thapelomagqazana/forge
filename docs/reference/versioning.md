<!--
  Forge Versioning Scheme
  =======================
  Public contract. Versioned with the Forge release. Additive changes
  only. Removing or changing the meaning of a documented rule requires a
  new ADR.

  Machine-readable form: docs/reference/version-metadata.yaml
  Schema:               schemas/version.schema.json
  ADR:                  docs/decisions/ADR-043-versioning-and-release-channels.md
  Channels:             docs/reference/release-channels.md
  Release notes:        docs/templates/release-notes.md

  This document is normative. If it and any other document disagree on
  version semantics, this document wins. If this document and
  docs/reference/release-channels.md disagree on channel semantics,
  release-channels.md wins.

  Editing rules:
    1. Change this file and version-metadata.yaml in the same commit.
       `task version:verify` fails if they diverge.
    2. Every rule that a script can check MUST also be expressed in
       version-metadata.yaml. Prose alone is not a contract.
    3. Every example MUST be a real tag that a maintainer could create.
-->

# Forge Versioning Scheme

**Scheme version:** `1.0.0`
**Applies to:** Forge `>= 0.1.0`
**Status:** Frozen (additive policy)
**Owner:** Release Engineering
**Reviewers:** Product Owner, Architecture

Forge versions are [Semantic Versioning 2.0.0][semver] strings with a
mandatory `v` prefix. Examples:

- `v0.1.0` — first public preview
- `v1.4.2` — a stable patch release
- `v1.5.0-rc.1` — a release candidate for `v1.5.0`
- `v1.5.0-nightly.20260601` — a nightly build of the eventual `v1.5.0`

[semver]: https://semver.org/spec/v2.0.0.html

---

## 1. SemVer adoption

Forge adopts [Semantic Versioning 2.0.0][semver] without modification to
the grammar. A version string has the form:

```text
v MAJOR . MINOR . PATCH [ - PRERELEASE ] [ + BUILD ]
```

- `MAJOR`, `MINOR`, `PATCH` are non-negative integers.
- `PRERELEASE` is one or more dot-separated identifiers, each
  alphanumeric plus hyphen. Forge uses a closed set (see §5).
- `BUILD` is permitted by SemVer but **not used by Forge tags** (see §7).

The `v` prefix is **mandatory**. Go modules require it for tags; Forge
enforces it in the release pipeline. A tag without the prefix is
rejected before any artifact is produced.

The three channel tag patterns are documented in
[`release-channels.md`](./release-channels.md) and enforced by
`internal/version.ChannelOf`.

---

## 2. Meaning of each segment for Forge

SemVer defines "breaking change" in terms of a public API. Forge's
public API is not a Go package: it is a command-line surface, a
configuration schema, an exit-code table, and a set of output formats.
This section maps each SemVer segment to the Forge surfaces it governs.

### 2.1 MAJOR — a breaking change

Bump MAJOR when any of the following changes in a way that can break an
existing user. A single qualifying change is sufficient; there is no
"small enough to be a MINOR" exception.

| Surface | Breaking change | Example |
|---|---|---|
| **CLI surface** | Remove a command, rename a command, rename a flag, change a flag's short form, change a flag's default value in a way that alters observable behaviour | `forge blueprint init` → `forge init` |
| **CLI surface** | Remove a positional argument, change its meaning, change its order | `forge check <dir>` → `forge check --dir=<dir>` |
| **CLI surface** | Change a command's output when no format flag is given | Help text that scripts parse |
| **`forge.yaml` schema** | Remove a field, rename a field, change a field's type, reject a previously-accepted value | `logLevel: info` → `log.level: info` |
| **`forge.yaml` schema** | Change a field's default in a way that alters observable behaviour | default `outputFormat: text` → `json` |
| **Exit codes** | Change the numeric value of a code in the stable range (0–127), remove a code | `64` (usage) → `2` |
| **Exit codes** | Introduce a code in the reserved-for-signals range (128–255) | Not permitted; signals are owned by the OS |
| **Output format** | Change the JSON shape of `--format=json`, rename a field, remove a field | `{"version": …}` → `{"v": …}` |
| **Output format** | Change the exit-code-bearing envelope's shape | See `docs/reference/output-streams.md` |
| **Public Go API** | Any change forbidden by the [Go 1 compatibility promise][go1] | Only relevant if `internal/` is ever promoted to a public module |

[go1]: https://go.dev/doc/go1compat

A MAJOR bump resets MINOR and PATCH to `0`. Prerelease identifiers may
appear during the lead-up to the MAJOR release (`v2.0.0-rc.1`).

### 2.2 MINOR — an additive feature

Bump MINOR when adding functionality that does not break existing
users. The change must be observable; a purely internal refactor with
no user-visible effect is not a MINOR bump (see §2.3).

Additive changes:

- A new command.
- A new flag with a default that preserves existing behaviour.
- A new optional field in `forge.yaml`.
- A new exit code in the reserved-for-findings range (see
  `docs/reference/exit-codes.yaml`).
- A new output format.
- A new platform in the supported-platform matrix
  (`docs/reference/platforms.yaml`).
- A new deprecated-but-still-working alias for an existing flag or
  command, provided the old form continues to work identically.

A MINOR bump resets PATCH to `0`. Existing behaviour is preserved
verbatim unless the release notes explicitly document a deprecation.

### 2.3 PATCH — a bug fix

Bump PATCH when fixing a defect without changing the documented
surface. The change must be unobservable to a user who was relying on
the documented behaviour; only users who were relying on the defect
notice a difference.

Patch-level changes:

- A bug in output formatting (e.g. column alignment).
- A bug in error classification (e.g. a missing file was reported as a
  usage error instead of a config error).
- A bug in exit-code derivation for an error path that was already
  documented.
- A dependency upgrade that does not change behaviour.
- A documentation correction that changes generated text (e.g. help
  text).
- A security fix that does not change the documented surface.

A PATCH bump leaves MAJOR and MINOR unchanged.

**If you are unsure whether a change is PATCH or MINOR, it is MINOR.**
The cost of an unnecessary MINOR bump is a slightly faster version
cadence. The cost of an incorrect PATCH bump is a user whose scripts
break during a "bug fix only" upgrade.

---

## 3. Pre-1.0 policy (`v0.x.y`)

For `v0.x.y`, the MAJOR segment is permanently `0` and the SemVer
guarantee is relaxed. Forge adopts the standard SemVer reading of
`0.y.z` **explicitly**, so users can rely on it:

> For `v0.x.y`, any MINOR bump may contain breaking changes. PATCH
> bumps must not. Every `v0.x.0` release that contains a breaking
> change MUST list the change in its release notes under a
> `### Breaking` heading.

Concretely:

| Transition | Breaking allowed? | Release notes requirement |
|---|---|---|
| `v0.1.0` → `v0.1.1` | No | Standard sections only |
| `v0.1.0` → `v0.2.0` | **Yes** | `### Breaking` heading required if any change is breaking |
| `v0.2.0` → `v0.2.1` | No | Standard sections only |
| `v0.2.0` → `v0.3.0` | **Yes** | `### Breaking` heading required if any change is breaking |
| `v0.9.0` → `v1.0.0` | Yes | `### Breaking` heading required; also the stability declaration |

The project targets `v1.0.0` once the CLI surface, `forge.yaml` schema,
and exit-code table are declared stable. Until then, a user on `v0.x.y`
should pin to a patch version (`v0.1.1`), not a minor version.

The `### Breaking` heading is defined in
[`docs/templates/release-notes.md`](../templates/release-notes.md).
A release that contains breaking changes but omits the heading fails
the release pipeline's notes check.

---

## 4. When to bump what — decision table

The following table is the one maintainers consult. It is exhaustive
for the surfaces Forge currently exposes. If a change does not fit a
row, treat it as a MINOR bump until a new row is added by an ADR.

| Change | Bump |
|---|---|
| Add a command | MINOR |
| Remove a command | MAJOR |
| Rename a command (old name kept as alias) | MINOR |
| Rename a command (old name removed) | MAJOR |
| Add a flag with a safe default | MINOR |
| Remove a flag | MAJOR |
| Rename a flag | MAJOR |
| Change a flag's short form | MAJOR |
| Change a flag's default to a value that alters observable behaviour | MAJOR |
| Add an optional field to `forge.yaml` | MINOR |
| Remove or rename a `forge.yaml` field | MAJOR |
| Change a `forge.yaml` field's type | MAJOR |
| Add an allowed value to a closed enum in `forge.yaml` | MINOR |
| Remove an allowed value from a closed enum | MAJOR |
| Add a new exit code in the findings range | MINOR |
| Change an existing exit code's value | MAJOR |
| Add a field to the `--format=json` envelope | MINOR |
| Remove or rename a field in the `--format=json` envelope | MAJOR |
| Fix a formatting bug | PATCH |
| Fix an error-classification bug | PATCH |
| Upgrade a dependency with no behaviour change | PATCH |
| Upgrade a dependency that changes an output format | MINOR |
| Add a platform to `platforms.yaml` | MINOR |
| Remove a platform from `platforms.yaml` | MAJOR |
| Change a platform's tier | MINOR |
| Raise a platform's minimum OS | MINOR |
| Lower a platform's minimum OS | PATCH |
| Change help text to be clearer | PATCH |
| Change help text in a way scripts parse | MAJOR |

---

## 5. Pre-release identifiers

Pre-release identifiers are lowercase ASCII alphanumerics and hyphens,
dot-separated, per SemVer 2.0.0. Forge **reserves** three identifiers:

| Identifier | Intent | Feature freeze? | API stable? | Example |
|---|---|---|---|---|
| `alpha.N` | Early preview; gather direction | No | No | `v1.0.0-alpha.1` |
| `beta.N` | Feature-complete; stabilize | Yes | Mostly | `v1.0.0-beta.3` |
| `rc.N` | Release candidate | Yes | Yes (bug fixes only) | `v1.0.0-rc.2` |

`N` is a positive integer starting at `1`. Prerelease numbering is
monotonic within a single `vX.Y.Z` and a single identifier: after
`v1.0.0-alpha.1` comes `v1.0.0-alpha.2`, then `v1.0.0-beta.1`, then
`v1.0.0-rc.1`, then `v1.0.0`.

An identifier not in this table is rejected by `ChannelOf`. Adding a
new identifier is a **MINOR** change to this document and requires an
ADR. Changing the meaning of an existing identifier is a **MAJOR**
change.

### Precedence

Pre-release versions have **lower** precedence than the corresponding
release, and are ordered among themselves by the identifier's natural
order:

```text
v1.0.0-alpha.1  <  v1.0.0-alpha.2
v1.0.0-alpha.2  <  v1.0.0-beta.1
v1.0.0-beta.1   <  v1.0.0-rc.1
v1.0.0-rc.1     <  v1.0.0
v1.0.0          <  v1.0.1
v1.0.1          <  v1.1.0
v1.1.0          <  v2.0.0
```

The `nightly` identifier (see [`release-channels.md`](./release-channels.md))
is not listed here because it does not participate in SemVer
precedence for package managers: nightlies are not published to package
managers and are never compared to `alpha`, `beta`, or `rc`.

---

## 6. Version embedding

Every Forge binary embeds four strings at link time. The values are
injected by `Taskfile.yml` via `-ldflags -X` and read by
`internal/version`.

| Variable | Source | Example | Constraint |
|---|---|---|---|
| `version` | `git describe --tags --always --dirty` | `v1.4.2` | Non-empty. May contain `-N-gSHA` and `-dirty` suffixes. Not normalized. |
| `commit` | `git rev-parse --short HEAD` | `abc1234` | Non-empty. Lowercase hex, 7–12 chars. |
| `buildDate` | `date -u +%Y-%m-%dT%H:%M:%SZ` | `2026-06-01T12:00:00Z` | RFC 3339, UTC, trailing `Z`. |
| `buildType` | The build environment | `release` | One of `release`, `ci`, `local`. Unknown values are normalized to `local`. |

The four values are exposed by `forge version` and by
`forge version --format=json`. The JSON shape is:

```json
{"version":"v1.4.2","commit":"abc1234","buildDate":"2026-06-01T12:00:00Z","buildType":"release"}
```

Field order is part of the contract: a user's script may depend on it.
Any change to the field names, order, or types is a MAJOR bump.

`version` is **not normalized** by the CLI. A binary built from a dirty
working tree reports `v1.4.2-dirty`; a binary built from a commit three
ahead of the tag reports `v1.4.2-3-gabc1234`. Both are meaningful and
both are preserved verbatim.

---

## 7. Build metadata (`+BUILD`)

SemVer permits a `+BUILD` suffix. Forge **does not use it in tags.**

Rationale:

1. **Reproducibility.** Two builds of the same commit must produce the
   same version string. Embedding the commit SHA in the tag would
   violate this.
2. **Package-manager consistency.** Homebrew, Scoop, winget, and apt
   sort `+BUILD` inconsistently. A version string that sorts the same
   everywhere is easier to reason about.
3. **Redundancy.** The commit SHA is already embedded at link time and
   reported by `forge version`. It does not need to be in the tag.

A tag containing `+` is rejected by the release pipeline.

---

## 8. Additive policy

This document is a contract. Changes to it follow the rules below.

| Change | Class | Requirements |
|---|---|---|
| Add a row to §2 (a new surface that a segment governs) | Minor | Update `version-metadata.yaml` in the same commit |
| Add a row to §4 (a new decision-table case) | Minor | Update `version-metadata.yaml` in the same commit |
| Add a pre-release identifier to §5 | Minor | ADR; update `version-metadata.yaml` |
| Change the meaning of a pre-release identifier | Major | ADR; deprecation notice in a prior minor release |
| Remove a pre-release identifier | Major | ADR; deprecation notice in a prior minor release |
| Change the meaning of a SemVer segment | Major | ADR; treat as a re-versioning of the project |
| Change a variable in §6 | Major | ADR; update `version-metadata.yaml` and `internal/version` |
| Begin using `+BUILD` in tags | Major | ADR; this reverses §7 |
| Change the pre-1.0 policy in §3 | Major | ADR |

Every change to this document MUST update
[`docs/reference/version-metadata.yaml`](./version-metadata.yaml) in the
same commit. `task version:verify` fails if the two diverge.

---

## 9. Machine-readable form

The rules above are mirrored in
[`docs/reference/version-metadata.yaml`](./version-metadata.yaml). The
YAML is the source of truth for build scripts, CI tag routing, and
release tooling. It is validated against
[`schemas/version.schema.json`](../../schemas/version.schema.json) in CI
by `task version:verify`.

The two forms are asserted equal by `internal/version` tests: a rule
that appears in the prose but not in the YAML, or vice versa, fails CI.
When the two disagree, **the YAML wins**, and this document is
corrected.

---

## 10. Related documents

| Document | Role |
|---|---|
| [`release-channels.md`](./release-channels.md) | Channel definitions, tag patterns, tag-to-release routing |
| [`version-metadata.yaml`](./version-metadata.yaml) | Machine-readable form of this document and `release-channels.md` |
| [`../templates/release-notes.md`](../templates/release-notes.md) | Human template every release fills in |
| [`../../schemas/version.schema.json`](../../schemas/version.schema.json) | JSON Schema for `version-metadata.yaml` |
| [`../decisions/ADR-043-versioning-and-release-channels.md`](../decisions/ADR-043-versioning-and-release-channels.md) | Architectural decision record |
| [`../reference/exit-codes.yaml`](./exit-codes.yaml) | The exit-code table that MAJOR bumps may change |
| [`../reference/platforms.yaml`](./platforms.yaml) | The platform matrix that MINOR bumps may extend |
| [`../../internal/version/`](../../internal/version/) | Implementation: `ChannelOf`, `Metadata`, `Validate` |
