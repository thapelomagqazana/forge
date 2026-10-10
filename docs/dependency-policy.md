# Dependency Policy

- **Document type:** Policy
- **Status:** Draft
- **Version:** 0.2.0
- **Author:** @thapelomagqazana
- **Created:** 2026-10-09
- **Last Updated:** 2026-10-10
- **Supersedes:** 0.1.0
- **Superseded by:** —

---

## 1. Purpose

This document defines Forge's policy for third-party dependencies.
It exists to answer:

- May Forge depend on third-party code?
- If so, which code, and under what conditions?
- How is the decision to add a dependency made?
- How is the decision recorded?
- How is the decision enforced?

The policy is deliberately restrictive. Forge is a foundation manager:
its users will run it in their own repositories, often in CI, often in
environments where dependency provenance matters. Every dependency
Forge carries is a dependency its users carry. The policy's job is to
keep the list short, the versions pinned, and the record auditable.

---

## 2. Scope

**In scope:**

- Direct dependencies (the modules listed in `go.mod`'s `require`
  block without an `// indirect` comment).
- Transitive dependencies (the modules listed with an `// indirect`
  comment).
- The process for adding, upgrading, and removing a dependency.
- The record that documents each dependency.

**Out of scope:**

- The standard library. The Go standard library is not subject to this
  policy; it is part of the toolchain and is updated with the
  toolchain.
- The Go toolchain itself. The Go version matrix is governed by
  ADR-004, not by this policy.
- Build-time tools that do not appear in `go.mod` (for example, the
  `task` binary, the `markdownlint-cli2` binary). These are governed
  by `docs/development.md`, not by this policy.

---

## 3. Principles

Forge's dependency policy rests on six principles.

### 3.1 Standard library first

The Go standard library is preferred over any third-party dependency.
A dependency is added only when the standard library cannot reasonably
provide the needed functionality. "Reasonably" means: the standard
library's alternative would require more than 200 lines of new code,
or the standard library's alternative would be less safe, or the
standard library's alternative would be less maintainable.

### 3.2 Minimal footprint

The number of direct dependencies is kept as small as possible. Every
addition is a cost the project pays forever, in the form of:

- Vulnerability surface.
- Upgrade maintenance.
- License review.
- Transitive dependency growth.
- Contributor onboarding.

The cost is paid by every future contributor, not just the one who
adds the dependency. The policy's default answer to "may I add this
dependency?" is **no**, and the burden of proof is on the contributor
who wants to add one.

### 3.3 Exact pinning

Every direct dependency is pinned to an exact semantic version
(`vX.Y.Z`). Ranges, branches, pseudo-versions for direct dependencies,
and `+incompatible` markers are forbidden. The pinning is recorded in
`go.mod` and verified by `go.sum`.

### 3.4 Auditable provenance

Every direct dependency has a recorded provenance:

- Its module path.
- Its pinned version.
- Its license.
- Its maintainer or maintaining organisation.
- The ADR that introduced it.
- Its purpose in Forge.

The provenance is recorded in the Approved Dependencies registry
(§ 7 of this document).

### 3.5 Automatic transitive pinning

Transitive dependencies are pinned automatically via `go.sum`. Forge
does not hand-edit transitive dependencies and does not list them in
its policy registry. The Go toolchain's `go mod tidy` is the source
of truth for their versions.

### 3.6 Reversibility

Every dependency must be replaceable. A dependency whose removal would
require rewriting more than 20% of the code that uses it is a
dependency Forge should not have. The principle is a design
constraint, not a mechanical rule: it guides the choice of how to use
a dependency (through a narrow interface, through a wrapper) as well
as the choice of which dependency to add.

---

## 4. Decision process

Adding, upgrading, or removing a direct dependency requires a
decision. The decision's weight depends on what is being decided.

### 4.1 Adding a direct dependency

Adding a direct dependency requires an ADR. The ADR must:

1. Name the dependency (module path and pinned version).
2. State the purpose. What does Forge need the dependency for?
3. State why the standard library is insufficient. What would it cost
   to implement the same functionality without the dependency?
4. State the license. Is the license compatible with Forge's
   Apache-2.0? See § 5.
5. State the maintainer. Who maintains the dependency? Is the
   maintenance active? When was the last release?
6. Review the transitive dependency tree. What does the dependency
   bring with it? Every transitive dependency is a cost Forge pays.
7. State the upgrade criteria. What are the conditions under which the
   dependency may be patched, minor-upgraded, or major-upgraded? See
   § 4.4.

The ADR is reviewed and accepted before the dependency is added. The
`DEP_ALLOWED_DIRECT` variable in `Taskfile.yml` and the Approved
Dependencies registry (§ 7) are updated in the same commit that adds
the dependency.

### 4.2 Removing a direct dependency

Removing a direct dependency requires an ADR only if the dependency
was in the registry. The ADR must state why the dependency is being
removed and what replaces it. The `DEP_ALLOWED_DIRECT` variable and
the registry are updated in the same commit.

Removing a dependency does not require an ADR if the dependency was
never in the registry (for example, if a WBS item's work made a
dependency unused, and the dependency is being pruned). The pruning
is recorded in the commit message.

### 4.3 Upgrading a direct dependency

The weight of the upgrade decision depends on the type of upgrade.

| Upgrade type | Example | Process |
|--------------|---------|---------|
| Patch | `v1.8.1` → `v1.8.2` | A short note in the pull request description. No ADR. |
| Minor | `v1.8.1` → `v1.9.0` | A brief ADR documenting what changed and why the upgrade is safe. |
| Major | `v1.x` → `v2.x` | A full ADR with a migration plan, the required code changes, and a rollback strategy. |

The upgrade's classification (patch, minor, major) is determined by
semantic versioning. A "patch" upgrade is one whose `MAJOR` and
`MINOR` components are unchanged. A "minor" upgrade is one whose
`MAJOR` component is unchanged. A "major" upgrade is one whose
`MAJOR` component is incremented.

If a dependency's versioning does not follow semantic versioning, the
upgrade is classified by the dependency's release notes, and the ADR
records the classification's rationale.

### 4.4 Upgrade criteria

Each dependency's registry entry (§ 7) states its upgrade criteria.
The criteria are dependency-specific: a widely-used, actively
maintained library may have looser criteria than a niche library
with one maintainer.

The generic criteria are:

- **Patch upgrades** are permitted if the release notes contain only
  bug fixes and no API changes.
- **Minor upgrades** are permitted if the release notes do not
  indicate breaking changes.
- **Major upgrades** require a full ADR.

### 4.5 Quarterly review

Dependencies are reviewed quarterly. The review checks:

- Is the dependency still needed? (Has the standard library or a
  different dependency replaced it?)
- Is the dependency still maintained? (When was the last release? Are
  there open security advisories?)
- Is the dependency still correctly pinned? (Are the pinned versions
  the latest patch or minor versions?)
- Are the upgrade criteria still accurate? (Has the dependency's
  maintenance status changed?)

The review's outcome is recorded in the dependency's registry entry's
"Last reviewed" field.

---

## 5. License policy

Every direct dependency's license must be compatible with Forge's
Apache-2.0 license. Compatible licenses include:

- Apache-2.0
- MIT
- BSD-2-Clause
- BSD-3-Clause
- ISC
- MPL-2.0 (for non-modifying use)

Incompatible licenses include:

- GPL-2.0, GPL-3.0 (and their "or later" variants)
- AGPL-3.0
- SSPL
- Any license that is not on the compatible list

A dependency whose license is not on the compatible list cannot be
added without an ADR that names the license, explains why it is
acceptable, and (if necessary) states the mitigation. In practice,
Forge's default is to not add such dependencies.

Licenses are recorded in the registry (§ 7).

---

## 6. Dependency change detection

The checks below run on every pull request that modifies `go.mod` or
`go.sum`. They are implemented in `scripts/deps/check-pr-deps.sh`,
invoked by `task verify:deps:pr-check`, and are part of the composed
`task verify:deps` target.

### V1 — New direct dependency

A new direct dependency must be justified in an ADR, listed in the
Approved Dependencies registry below, and must not be present in
`go.mod` unless the ADR has been accepted.

V1 is enforced as a **warning** when the PR's base revision is
available. Without a base revision, the check is skipped (see the
script's `--base` flag). The skip is deliberate: a contributor who
runs the check locally without a base sees the other four checks and
is not blocked by V1.

### V2 — Version pin

**Every direct `require` directive in `go.mod` must be pinned to an
exact semantic version (`vX.Y.Z`).**

Indirect dependencies — those carrying the `// indirect` comment —
are out of scope for this rule. They are selected by the Go toolchain
to satisfy the direct dependencies' version constraints, and they are
recorded by `go mod tidy`. Hand-editing them would produce a
`go mod tidy` diff, which fails the integrity check
(`verify:deps:integrity`). The toolchain's choices are the source of
truth for indirect versions.

This scoping was introduced after `github.com/inconshreveable/mousetrap
v1.1.0 // indirect` (a transitive dependency of Cobra, introduced in
WBS 2.4.1) was flagged by the V2 check as a version-pin violation. The
flag was a false positive: the indirect dependency is not selected by
Forge and cannot be pinned by hand without breaking the integrity
check. The V3 and V4 checks were already scoped to direct
dependencies; V2 was not, and this change brings the three into
alignment. See § 9 (Change history) for the correction's record.

### V3 — `+incompatible` marker

No direct `require` directive in `go.mod` may contain a
`+incompatible` marker. The marker appears when a module's version
is v2 or higher and the module path does not include the major
version suffix (for example, `/v2`). Forge's policy is to use modules
whose paths include the version suffix, so the marker is not expected.

If a `+incompatible` marker is present in a direct dependency, an ADR
is required to explain why it is necessary and to document the
migration plan.

V3 is scoped to direct dependencies; indirect dependencies may carry
the marker if a transitive dependency uses one.

### V4 — Pseudo-version

No direct dependency in `go.mod` may use a pseudo-version
(`v0.0.0-YYYYMMDDHHMMSS-<hash>`). A pseudo-version indicates that the
dependency is pinned to a specific commit rather than to a released
version. Forge's policy is to depend on released versions, so a
pseudo-version in a direct dependency requires an ADR that documents
the commit hash and the reason for pinning to it.

Indirect dependencies may use pseudo-versions if the Go toolchain
selects them. They are not subject to V4.

### V5 — License

Every new or upgraded direct dependency must have a license compatible
with Forge's Apache-2.0. See § 5 for the compatible list. The license
is recorded in the registry (§ 7).

V5 is enforced by the ADR review, not by the script. The script cannot
determine a module's license without fetching the module, and the
fetch would be an external call that violates the script's hermeticity.
The ADR is the record of the license review.

---

## 7. Approved Dependencies registry

Every direct dependency is listed here with its module path, pinned
version, license, maintainer, first-introduced WBS item, ADR, purpose,
transitive dependencies, upgrade criteria, and last reviewed date.

### github.com/spf13/cobra

| Attribute           | Value                                                |
|---------------------|------------------------------------------------------|
| Module path         | `github.com/spf13/cobra`                             |
| Pinned version      | `v1.8.1`                                             |
| License             | Apache-2.0                                           |
| Maintainer          | Steve Francia and contributors                       |
| First introduced    | WBS 2.4.1                                            |
| ADR                 | [ADR-006](decisions/ADR-006-cobra-cli-framework.md)  |
| Purpose             | CLI command tree, flag parsing, help output          |
| Last reviewed       | 2026-10-10                                           |

**Why Cobra:** Forge requires a command-line framework that supports
nested subcommands, POSIX-style flags, automatic help generation, and
shell completion. Cobra provides all four and is the de facto standard
in the Go ecosystem, used by `kubectl`, `gh`, `hugo`, `helm`, and
`docker`. The alternative (`flag` from the standard library) does not
support subcommands. See ADR-006 for the full comparison.

**Transitive dependencies:**

- `github.com/spf13/pflag v1.0.5` — POSIX/GNU-style flags. Cobra's
  flag parser. Same maintainer. License: BSD-3-Clause.
- `github.com/inconshreveable/mousetrap v1.1.0` — Windows-only
  detection of double-click launches. Pulled in unconditionally but
  only compiled on Windows. License: Apache-2.0.

Both transitive dependencies are pinned automatically via `go.sum`
and require no manual maintenance.

**Upgrade criteria:**

- **Patch upgrades** (e.g., `v1.8.1` → `v1.8.2`): permitted if the
  upstream release notes contain only bug fixes and no API changes.
  A short note in the pull request description suffices; no ADR
  required.
- **Minor upgrades** (e.g., `v1.8.1` → `v1.9.0`): permitted if the
  release notes do not indicate breaking changes. A brief ADR is
  required, documenting what changed and why the upgrade is safe.
- **Major upgrades** (e.g., `v1.x` → `v2.x`): require a full ADR
  describing the migration plan, the code changes required, and the
  rollback strategy.

Cobra has not released a major version beyond v1.x. When v2.0 is
released, its upgrade will be treated as a Phase 4-or-later concern,
not Phase 2.

**Upgrade cadence:** Reviewed quarterly, aligned with the general
dependency review cycle described in § 4.5.

---

## 8. Module resolution policy

Forge's module resolution is governed by the following rules. The
rules are verified by `task verify:deps:env-policy`, which is part of
the composed `task verify:deps` target.

### 8.1 GOFLAGS

`GOFLAGS` must contain `-mod=readonly`. The flag prevents `go build`
and `go test` from modifying `go.mod` or `go.sum` silently. The flag
is documented in `docs/development.md` and is verified by
`verify:reproducible:env`.

### 8.2 GOTOOLCHAIN

`GOTOOLCHAIN` must be `auto` or `local`. The value `auto` allows Go
to honour the `toolchain` directive in `go.mod`; `local` uses the
installed toolchain. Both are acceptable; the default is `auto`.

### 8.3 GOSUMDB

`GOSUMDB` must be `sum.golang.org` (the default). The checksum
database is what verifies that a module's content matches its
declared version. Disabling it would remove a critical integrity
check.

### 8.4 GONOSUMDB and GONOSUMCHECK

Both must be empty or unset. They are legacy environment variables
that, when set, disable checksum verification for specific modules.
Forge does not use them.

### 8.5 GOPROXY

`GOPROXY` must be `https://proxy.golang.org,direct` (the default) or
a compatible proxy. The policy permits a corporate proxy if one is
required by the environment, provided that the proxy is documented in
`docs/development.md`.

### 8.6 GOMODCACHE

`GOMODCACHE` may be set to any path. Its value is informational; it
records where the module cache lives. The policy does not constrain
it.

### 8.7 Vendor directory

Forge does not use a `vendor/` directory. The absence of the directory
is verified by `verify:deps:env-policy`. If a `vendor/` directory is
ever required (for example, for an air-gapped build), an ADR is
required to justify its introduction and to document how it is kept
in sync with `go.mod`.

---

## 9. Change history

| Version | Date       | Author             | Change |
|---------|------------|--------------------|--------|
| 0.1.0   | 2026-10-09 | @thapelomagqazana  | Initial policy. Standard library first, minimal footprint, exact pinning, auditable provenance, automatic transitive pinning, reversibility. V1–V5 checks. Cobra registry entry. Module resolution policy. |
| 0.2.0   | 2026-10-10 | @thapelomagqazana  | Corrected V2's scope from "every `require` directive" to "every direct `require` directive". Indirect dependencies are selected by the Go toolchain and are out of scope for version pinning. The correction was motivated by `github.com/inconshreveable/mousetrap v1.1.0 // indirect`, a transitive dependency of Cobra, which the V2 check flagged as a false positive. The `scripts/deps/check-pr-deps.sh` script was updated to exclude `// indirect` lines, and a regression test was added to `internal/cli/structure_test.go`. V3 and V4 already scoped to direct dependencies; V2 now matches. |

---

## 10. Process summary

A contributor who wants to add, upgrade, or remove a direct dependency:

1. **Determines the change's weight.** Adding a dependency and major
   upgrades require a full ADR. Minor upgrades require a brief ADR.
   Patch upgrades require a note in the pull request description.
   Removing an unused dependency does not require an ADR.

2. **Writes the ADR if required.** The ADR follows
   [`docs/decisions/template.md`](./decisions/template.md). It names
   the dependency, its version, its license, its maintainer, its
   purpose, its transitive closure, and its upgrade criteria.

3. **Updates the registry.** The Approved Dependencies registry (§ 7)
   is updated in the same commit as the ADR.

4. **Updates `DEP_ALLOWED_DIRECT`.** The `DEP_ALLOWED_DIRECT` variable
   in `Taskfile.yml` is updated in the same commit as the ADR.

5. **Runs the checks.** `task verify:deps` runs the V1–V5 checks and
   the module resolution checks. All must pass.

6. **Opens a pull request.** The pull request's description links the
   ADR and explains the change's rationale.

A reviewer who evaluates the pull request:

1. **Reads the ADR** (if present). The ADR is the record of the
   decision; the pull request is the change that implements it.

2. **Checks the registry.** The dependency's registry entry must be
   present, complete, and accurate.

3. **Checks the license.** The license must be on the compatible list
   (§ 5). The registry's license field must be correct.

4. **Reviews the transitive closure.** The ADR's transitive
   dependency review must be complete. A new transitive dependency is
   a cost the pull request's author is proposing to add; the reviewer
   must evaluate whether the cost is justified.

5. **Runs the checks.** `task verify` runs the V1–V5 checks and the
   module resolution checks. All must pass.

6. **Approves or requests changes.** The reviewer's approval is the
   record that the ADR's reasoning has been reviewed.

---

## 11. Status

**Draft.** This policy becomes **Accepted** when:

- All Phase 2 dependencies are listed in the registry (§ 7).
- The V1–V5 checks are implemented and passing.
- The module resolution policy is verified by
  `task verify:deps:env-policy`.
- The policy has been reviewed for consistency with
  [`docs/architecture.md`](./architecture.md) § 5 (dependency
  direction) and § 13.2 (import rules).
  