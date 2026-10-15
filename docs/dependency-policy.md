# Dependency Policy

- **Document type:** Policy
- **Status:** Draft
- **Version:** 0.4.0
- **Author:** @thapelomagqazana
- **Created:** 2026-10-09
- **Last Updated:** 2026-10-12
- **Supersedes:** 0.3.0
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

### github.com/spf13/pflag

| Attribute           | Value                                                |
|---------------------|------------------------------------------------------|
| Module path         | `github.com/spf13/pflag`                             |
| Pinned version      | `v1.0.5`                                             |
| License             | BSD-3-Clause                                         |
| Maintainer          | Steve Francia and contributors                       |
| First introduced    | WBS 2.4.1                                            |
| ADR                 | [ADR-006](decisions/ADR-006-cobra-cli-framework.md)  |
| Purpose             | POSIX/GNU-style flag parsing                         |
| Last reviewed       | 2026-10-10                                           |

**Why pflag:** pflag is a first-class part of Cobra's public API.
Cobra exposes `cobra.Command.Flags()` as a `*pflag.FlagSet`, and any
code that inspects or iterates a command's flags names pflag types
(for example, `*pflag.Flag`). The Go toolchain classifies pflag as
a direct dependency because of those references, even though Forge
did not choose it independently. See ADR-006.

**Transitive dependencies:** None. pflag depends only on the Go
standard library.

**Upgrade criteria:**

- **Patch upgrades**: permitted if the release notes contain only bug
  fixes and no API changes. No ADR required.
- **Minor upgrades**: permitted if the release notes do not indicate
  breaking changes. A brief ADR is required.
- **Major upgrades**: require a full ADR with a migration plan.

pflag has not released a major version beyond v1.x. When v2.0 is
released, its upgrade will be treated as a Phase 4-or-later concern.

**Upgrade cadence:** Reviewed quarterly.

### gopkg.in/yaml.v3

| Attribute           | Value                                                |
|---------------------|------------------------------------------------------|
| Module path         | `gopkg.in/yaml.v3`                                   |
| Pinned version      | `v3.0.1`                                             |
| License             | MIT and Apache-2.0 (dual-licensed)                   |
| Maintainer          | Canonical Ltd. and the go-yaml contributors          |
| First introduced    | WBS 8.4.2                                            |
| ADR                 | [ADR-007](decisions/ADR-007-yaml-parsing-library.md) |
| Purpose             | YAML parsing for the config file loader              |
| Last reviewed       | 2026-10-11                                           |

**Why yaml.v3:** Forge's config file (`forge.yaml`) is YAML. The
standard library has no YAML parser; the alternatives are to write
one (out of scope: the YAML specification is large and its edge
cases are subtle), to use a different format (out of scope: the
format is fixed by `docs/forge-yaml-spec.md`), or to depend on a
library. `gopkg.in/yaml.v3` is the de facto YAML library for Go,
maintained by Canonical, used by Kubernetes, Docker Compose, GitHub
Actions, and the Go toolchain's own `go test` reporting. It is a
direct dependency of the config package (WBS 8.4.2).

The library provides three features the loader depends on:

1. **Strict field matching** via `Decoder.KnownFields(true)`, which
   rejects unknown YAML keys. This is what makes a typo in
   `forge.yaml` a load-time error instead of a silently-ignored
   setting.
2. **Typed unmarshalling** into a struct, with type-mismatch errors
   that name the offending field.
3. **Line and column information** in syntax errors, which the
   loader surfaces in the E3 error message.

See ADR-007 for the alternatives considered (a hand-written parser,
`ghodss/yaml`, `sigs.k8s.io/yaml`, and a JSON config format).

**Transitive dependencies:** None. `gopkg.in/yaml.v3` depends only on
the Go standard library. The `v2` and `v3` modules share a source
tree; the `v3` module path is `gopkg.in/yaml.v3`, and the version is
`v3.0.1` (the module's tag, not a semantic version of the form
`v3.x.y`). The `v3.0.1` value is an exact tag, not a pseudo-version,
and it satisfies the V2 pin rule.

**Upgrade criteria:**

- **Patch upgrades** (e.g., `v3.0.1` → `v3.0.2`): permitted if the
  release notes contain only bug fixes and no API changes. No ADR
  required.
- **Minor upgrades** (within `v3.x`): permitted if the release notes
  do not indicate breaking changes. A brief ADR is required.
- **Major upgrades** (`v3.x` → `v4.x`): require a full ADR with a
  migration plan. yaml.v3 is the current major version; a future
  v4 would be treated as a Phase 4-or-later concern.

**Upgrade cadence:** Reviewed quarterly, aligned with the general
dependency review cycle described in § 4.5.

**CVE history:** `gopkg.in/yaml.v3` v3.0.0 fixed CVE-2022-28948, a
denial-of-service vulnerability in the decoder. The pinned version
`v3.0.1` includes the fix. The registry's "Last reviewed" field is
updated when the library's security advisories are re-checked.

### github.com/stretchr/testify

| Attribute           | Value                                                |
|---------------------|------------------------------------------------------|
| Module path         | `github.com/stretchr/testify`                        |
| Pinned version      | `v1.9.0`                                             |
| License             | MIT                                                  |
| Maintainer          | Stretchr, Inc. and contributors                      |
| First introduced    | WBS 9.5.1                                            |
| ADR                 | [ADR-011](decisions/ADR-011-testify-testing-library.md) |
| Purpose             | Test assertions (`assert`, `require`)                |
| Last reviewed       | 2026-10-12                                           |

**Why testify:** Forge's tests use testify's `assert` and `require`
packages for their assertions. The alternatives are to use the
standard library's `testing` package alone (which produces verbose,
repetitive assertion code) or to adopt a different testing library
(for example, `gotest.tools` or `is`). testify is the de facto
assertion library in the Go ecosystem, used by Kubernetes, Docker,
Hugo, Helm, and the majority of large Go projects. It provides two
assertion styles — `assert` (records a failure, continues) and
`require` (records a failure, stops) — and a `Suite` type for
class-based tests, of which Forge uses the first two.

The library is used exclusively in `_test.go` files. It is not
imported by any non-test code, and it is not linked into the
production binary. This is a property of the Go build: packages
imported only by test files do not appear in the binary.

**Transitive dependencies:**

- `github.com/davecgh/go-spew v1.1.1` — deep equality printing
  for assertion failure messages. License: ISC.
- `github.com/pmezard/go-difflib v1.0.0` — unified diff generation
  for assertion failure messages. License: BSD-3-Clause.
- `gopkg.in/yaml.v3 v3.0.0` — used by testify's `ObjectsAreEqual`
  helper for YAML-aware comparison. Testify's version constraint
  (`v3.0.0`) is satisfied by Forge's pinned `v3.0.1`, so no
  second version of yaml.v3 is added to the module graph.
- `github.com/stretchr/objx v0.5.2` — helper types for `mock` and
  `http` sub-packages. Forge does not use those sub-packages, but
  the module is part of testify's `go.mod` and is pinned by
  `go.sum`.

All four transitive dependencies are pinned automatically via
`go.sum` and require no manual maintenance.

**Upgrade criteria:**

- **Patch upgrades** (e.g., `v1.9.0` → `v1.9.1`): permitted if the
  release notes contain only bug fixes and no API changes. A short
  note in the pull request description suffices; no ADR required.
- **Minor upgrades** (e.g., `v1.9.0` → `v1.10.0`): permitted if the
  release notes do not indicate breaking changes. A brief ADR is
  required, documenting what changed and why the upgrade is safe.
- **Major upgrades** (e.g., `v1.x` → `v2.x`): require a full ADR
  describing the migration plan, the code changes required, and the
  rollback strategy.

testify has not released a major version beyond v1.x. When v2.0 is
released, its upgrade will be treated as a Phase 4-or-later concern.

**Upgrade cadence:** Reviewed quarterly, aligned with the general
dependency review cycle described in § 4.5.

**Why the dependency is acceptable despite its transitive closure:**
testify is the only Forge dependency whose transitive closure is more
than one module (cobra brings two, yaml.v3 brings none, testify
brings four). The additional closure is the accepted cost of using
the assertion library. The alternatives were evaluated and rejected:
the standard library's `testing` package produces assertion code
that is roughly three times longer for the same coverage, and the
smaller alternatives (`gotest.tools`, `is`) are used by an order of
magnitude fewer projects, which weakens the maintenance argument.
The transitive dependencies are all permissively licensed (ISC,
BSD-3-Clause, MIT), all maintained, and none is a security concern.
The dependency is test-only, so it does not affect the production
binary's size, attack surface, or startup time.

### golang.org/x/tools

| Attribute           | Value                                                |
|---------------------|------------------------------------------------------|
| Module path         | `golang.org/x/tools`                                 |
| Pinned version      | `<COMPAT>`                                           |
| License             | BSD-3-Clause                                         |
| Maintainer          | The Go team (golang.org)                             |
| First introduced    | WBS 14.1.1                                           |
| ADR                 | [ADR-041](decisions/ADR-041-arch-guardrail-dependency-golang-x-tools.md) |
| Purpose             | Loading the Go package graph for the architecture guardrail (`internal/arch`) |
| Last reviewed       | 2026-10-14                                           |

**Why golang.org/x/tools:** Forge's clean-architecture guardrail
(ADR-040) and its package-group dependency matrix (ADR-042) are
enforced by a Go test in `internal/arch` that loads every package
under `internal/...`, classifies it into a layer and a group, and
asserts the allowed-import matrix against the real import graph. The
canonical way to load a Go package graph in Go is
`golang.org/x/tools/go/packages`. It is the same library used by
`staticcheck`, `gopls`, `golangci-lint`, and every other serious Go
analysis tool.

The library provides four properties the guardrail depends on:

1. **Build-tag awareness.** `packages.Load` honors `GOOS`, `GOARCH`,
   and build tags, so the guardrail sees the same package set the
   compiler sees. A hand-written parser over the filesystem would
   miss build-tagged files or include files that are not compiled.
2. **Module resolution.** `packages.Load` resolves the module graph
   via `go list` under the hood, so module replacements and vendored
   builds are handled correctly.
3. **Test-package awareness.** `packages.Load` reports test-only
   variants (`foo [foo.test]`) and external test packages
   (`foo_test`) as distinct packages, which is what allows the
   guardrail to check imports made only from `_test.go` files.
4. **A stable, documented API.** `golang.org/x/tools/go/packages` is
   the supported replacement for the older
   `golang.org/x/tools/go/loader` package; its API is documented in
   the standard Go reference and versioned with the module.

The library is used only from `internal/arch`'s test files. It is not
imported by any non-test code, and it is not linked into the
production binary. This is a property of the Go build: packages
imported only by test files do not appear in the binary.

**Transitive dependencies:** `golang.org/x/tools` has two direct
dependencies, both of which are pulled into Forge's module graph:

- `golang.org/x/mod` — module-file parsing. Maintained by the Go
  team. License: BSD-3-Clause.
- `golang.org/x/sync` — concurrency primitives. Maintained by the Go
  team. License: BSD-3-Clause.

Both transitive dependencies are pinned automatically via `go.sum`
and require no manual maintenance. Their licenses are on the
compatible list (§5), their maintainer is the same as
`golang.org/x/tools`, and neither is a security concern.

**Version bound (the constraint that decides which version is
pinned):** `golang.org/x/tools` raises its own `go` directive as the
Go team drops support for older Go releases. The version pinned in
Forge is the newest release whose `go.mod` declares `go 1.23.0` or
lower, because that is the frozen minimum supported by Forge
(ADR-004, `docs/development.md`). Adopting a newer release would
force Forge's `go` directive above `1.23.0`, which is a project-wide
change governed by WBS 2.1.2 and blocked by the
`goversion:verify:go-directive` task.

**Upgrade criteria:**

- **Patch upgrades** (e.g., `<COMPAT>` → the next patch): permitted
  if the release notes contain only bug fixes and no API changes,
  **and** the candidate's `go` directive is still ≤ `1.23.0`. A
  short note in the pull request description suffices; no ADR
  required.
- **Minor upgrades** (e.g., `<COMPAT>` → the next minor): permitted
  if the release notes do not indicate breaking changes, **and** the
  candidate's `go` directive is still ≤ `1.23.0`. A brief ADR is
  required.
- **Major upgrades** (a hypothetical `v1.x`): require a full ADR.
  The module has not released a major version beyond v0.
- **Crossing the toolchain bound:** any upgrade whose `go` directive
  exceeds Forge's frozen minimum is blocked until WBS 2.1.2 is
  revised by a superseding ADR. The check is mechanical: verify the
  candidate's `go` directive with `go mod download -x
  golang.org/x/tools@<candidate>` and inspect the cached `.mod`
  file, or fetch `https://proxy.golang.org/golang.org/x/tools/@v/<candidate>.mod`.

**Upgrade cadence:** Reviewed quarterly, aligned with the general
dependency review cycle described in §4.5.

**CVE history:** None affecting the pinned version. Security
advisories for the `golang.org/x/` family are published via
`pkg.go.dev/vuln`; the registry's "Last reviewed" field is updated
when the advisories are re-checked.

**Why the dependency is test-only:** `golang.org/x/tools` is used by
`internal/arch`, which is itself test infrastructure. No production
package imports it. This is confirmed by `go build ./cmd/forge`
succeeding without `golang.org/x/tools` in the link step; a reviewer
can verify with `go tool nm ./forge | grep x/tools`, which must
produce no output for a correctly built binary.

**Alternatives considered and rejected:** the alternatives are the
same ones that appear in ADR-041: a hand-written parser using
`go/parser` + `go/ast`, shelling out to `go list -json`, and using
`golangci-lint`'s `depguard` rule. All three were rejected for
reasons documented in ADR-041. The short version: a hand-written
parser re-implements build-tag and module-resolution logic that the
toolchain already provides; shelling out to `go list` makes the test
depend on `go` being on `PATH` and is slower; and `depguard` expresses
allow/deny lists rather than a full matrix over dynamic package sets.

### github.com/santhosh-tekuri/jsonschema/v6

| Attribute           | Value                                                |
|---------------------|------------------------------------------------------|
| Module path         | `github.com/santhosh-tekuri/jsonschema/v6`           |
| Pinned version      | `v6.0.2`                                             |
| License             | Apache-2.0                                           |
| Maintainer          | Santhosh Kumar Tekuri                                |
| First introduced    | WBS 8.4.3                                            |
| ADR                 | [ADR-008](decisions/ADR-008-config-schema-validation.md) |
| Purpose             | JSON Schema validation for `forge.yaml`'s schema     |
| Last reviewed       | 2026-10-15                                           |

**Why jsonschema/v6:** Forge's `forge.yaml` has a published JSON
Schema (`schemas/config.schema.json`), and the config loader must
reject a config file that violates the schema with an error that
names the offending field and the schema constraint it violates. The
alternative is to hand-write the validation logic for each field,
which duplicates the schema's constraints and drifts from them: the
schema is the source of truth for the config's shape; a hand-written
validator is a second source, and the two diverge.

`github.com/santhosh-tekuri/jsonschema/v6` is the de facto JSON
Schema library for Go. It implements JSON Schema Draft 2020-12, which
is the draft Forge's `schemas/config.schema.json` declares. It
provides four properties the loader depends on:

1. **Draft 2020-12 compliance.** The schema uses `$defs`, `$ref`,
   `prefixItems`, and `unevaluatedProperties`; a library that
   implemented an older draft would reject the schema or misread
   its constraints.
2. **Descriptive validation errors.** The library's errors name the
   JSON Pointer to the offending value and the keyword that failed,
   which the loader surfaces in its error's context. A user who
   misspells a field sees the field's path and the schema's
   expectation, not a generic "invalid config".
3. **External `$ref` resolution.** The schema's `$ref`s point to
   files under `schemas/`; the library resolves them from the
   filesystem without a network call. The loader's registry is a
   local resolver; the library's API supports it.
4. **A single binary's worth of code.** The library is pure Go; it
   has no cgo dependency, and it does not use the `net/http`
   package's cgo path. The release binary's `CGO_ENABLED=0` policy
   (ADR-042) is not affected.

The library is a direct dependency of the config package (WBS 8.4.3).
It is imported by non-test code: the config loader calls the
library's `Compile` and `Validate` methods. It is not a test-only
dependency; the production binary links it.

See ADR-008 for the alternatives considered (a hand-written
validator, `santhosh-tekuri/jsonschema/v5`, `xeipuuv/gojsonschema`,
and `qri-io/jsonschema`).

**Transitive dependencies:** `github.com/santhosh-tekuri/jsonschema/v6`
has one direct dependency:

- `github.com/santhosh-tekuri/jsonschema/v6/internal/...` — the
  module's own sub-packages. Not a third-party dependency.
- `github.com/dlclark/regexp2` — an extended regular-expression
  engine that supports the ECMA-262 regex flavor JSON Schema's
  `pattern` keyword uses. License: MIT. Maintained by Damian
  Clark; the library is the one `gopls` uses for its regex
  handling.

The module's `go.mod` also lists `golang.org/x/text` and
`golang.org/x/crypto` as indirect dependencies of `regexp2` and of
the module's `cmd/` tools. Forge's build does not compile the
module's `cmd/` tools, so the two `golang.org/x/` modules are in
`go.sum` but not in the production binary's link step.

All transitive dependencies are pinned automatically via `go.sum`
and require no manual maintenance. Their licenses (MIT, BSD-3-Clause)
are on the compatible list (§5).

**Version bound:** the pinned version `v6.0.2` is the newest release
whose `go` directive is `go 1.21` or lower, which is within Forge's
frozen minimum (`go 1.23.0`, ADR-004). A future `v6.x` release whose
`go` directive exceeds Forge's minimum is blocked until Forge's
minimum is raised by a superseding ADR; the version-bound check is
the same mechanical procedure as `golang.org/x/tools`'s.

**Upgrade criteria:**

- **Patch upgrades** (e.g., `v6.0.2` → `v6.0.3`): permitted if the
  release notes contain only bug fixes and no API changes, **and**
  the candidate's `go` directive is still ≤ `1.23.0`. A short note
  in the pull request description suffices; no ADR required.
- **Minor upgrades** (within `v6.x`): permitted if the release notes
  do not indicate breaking changes, **and** the candidate's `go`
  directive is still ≤ `1.23.0`. A brief ADR is required.
- **Major upgrades** (`v6.x` → `v7.x`): require a full ADR with a
  migration plan. The library's major-version cadence tracks the
  JSON Schema drafts; a `v7` would likely implement Draft 2028 or
  a future draft, and the schema's `$schema` field would need to
  move with it.
- **Crossing the toolchain bound:** any upgrade whose `go` directive
  exceeds Forge's frozen minimum is blocked until WBS 2.1.2 is
  revised by a superseding ADR. The check is the same as
  `golang.org/x/tools`'s: fetch the candidate's `.mod` file and
  inspect its `go` directive.

**Upgrade cadence:** Reviewed quarterly, aligned with the general
dependency review cycle described in §4.5.

**CVE history:** None affecting the pinned version. Security
advisories for the library are published via `pkg.go.dev/vuln`; the
registry's "Last reviewed" field is updated when the advisories are
re-checked.

**Why the dependency is not test-only:** unlike `testify` and
`golang.org/x/tools`, this library is imported by the config package's
production code. Its presence in the release binary is intentional:
the config loader must validate the config at load time, and the
validation uses the schema. A future change that moved the
validation to a test-only path would change the binary's behavior
(a malformed config would no longer fail at load time), which is a
change to the config loader's contract, not to this dependency's
status.

**Alternatives considered and rejected:** ADR-008 documents the
alternatives. The short version: a hand-written validator duplicates
the schema's constraints and drifts; `santhosh-tekuri/jsonschema/v5`
implements Draft 7, and Forge's schema declares Draft 2020-12;
`xeipuuv/gojsonschema` is unmaintained and implements Draft 4;
`qri-io/jsonschema` is an interface-first library whose API is a
larger change to the loader than the schema-aware alternatives.

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
| 0.3.0   | 2026-10-11 | @thapelomagqazana  | Added the `gopkg.in/yaml.v3` registry entry (§ 7). The dependency was introduced by WBS 8.4.2 (the config file loader); ADR-007 documents its rationale. The entry records the module path, pinned version (`v3.0.1`), dual MIT/Apache-2.0 license, Canonical's maintenance, no transitive dependencies, and the upgrade criteria. The registry's addition closes the `verify:deps:documented` failure that reported the dependency as undocumented. |
| 0.4.0   | 2026-10-12 | @thapelomagqazana  | Added the `github.com/stretchr/testify` registry entry (§ 7). The dependency was introduced by WBS 9.5.1 (the acceptance tests) and is used throughout `internal/cli`'s test files. The entry records the module path, pinned version (`v1.9.0`), MIT license, Stretchr, Inc.'s maintenance, the four transitive dependencies (all permissively licensed), and the upgrade criteria. The registry's addition closes the `verify:deps:documented` failure that reported the dependency as undocumented. The section also records why a test-only dependency with a larger transitive closure than Forge's other dependencies is acceptable: the closure is permissively licensed, actively maintained, and does not affect the production binary. |
| 0.5.0   | 2026-10-14 | @thapelomagqazana  | Added the `golang.org/x/tools` registry entry (§7). The dependency was introduced by WBS 14.1.1 (the clean-architecture guardrail, ADR-040) and extended by WBS 14.1.2 (the package-group dependency matrix, ADR-042). The entry records the module path, pinned version (`<COMPAT>`), BSD-3-Clause license, the Go team's maintenance, the two transitive dependencies (`golang.org/x/mod` and `golang.org/x/sync`, both BSD-3-Clause), and the version-bound rationale: the pinned version is the newest release whose `go` directive is ≤ Forge's frozen minimum (`go 1.23.0`), because adopting a newer release would raise the project's minimum supported Go version for a test-only dependency. |
| 0.5.0   | 2026-10-15 | @thapelomagqazana  | Added the `github.com/santhosh-tekuri/jsonschema/v6` registry entry (§7). The dependency was introduced by WBS 8.4.3 (config schema validation); ADR-008 documents its rationale. The entry records the module path, pinned version (`v6.0.2`), Apache-2.0 license, Santhosh Kumar Tekuri's maintenance, the one transitive dependency (`github.com/dlclark/regexp2`, MIT), the two `golang.org/x/` modules that are in `go.sum` but not in the production binary, and the version-bound rationale: the pinned version's `go` directive is within Forge's frozen minimum (`go 1.23.0`). |

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
  