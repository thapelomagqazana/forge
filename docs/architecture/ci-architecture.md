# CI Workflow Architecture

> **Status:** Accepted
> **Owner:** Release Engineering
> **Reviewers:** Security, Documentation
> **Related:** [ADR-043](../decisions/ADR-043-ci-workflow-architecture.md), [Release Process](../reference/release-process.md), [Platform Tiers](../reference/platform-tiers.md)

## Purpose

This document defines the CI workflow architecture for the forge
repository. It is the authoritative reference for:

- which workflows exist and what each is for,
- which jobs are required before a merge,
- who owns each workflow file and who must review changes to it,
- how third-party actions are pinned and updated, and
- the `GITHUB_TOKEN` permissions granted to each workflow.

The architecture is enforced by tests in `internal/ci/`. A change that
violates the architecture fails CI before a human reviewer sees it.

## Workflow inventory

The repository's `.github/workflows/` directory contains twelve workflow
files. They fall into two classes: seven top-level workflows that run on
repository events, and five reusable workflows that are invoked by other
workflows via `uses:`.

### Top-level workflows

| Workflow | File | Trigger | Secrets | Required | Owner |
|---|---|---|---|---|---|
| CI | `.github/workflows/ci.yml` | `pull_request`, `push` to `main` | No | Yes | Release Engineering |
| Release | `.github/workflows/release.yml` | Tag `v*`, `workflow_dispatch` | Yes | No | Release Engineering |
| Security | `.github/workflows/security.yml` | `schedule` daily, `push` to `main`, `workflow_dispatch` | No | No (advisory) | Security |
| Nightly | `.github/workflows/nightly.yml` | `schedule` nightly, `workflow_dispatch` | No | No (advisory) | Release Engineering |
| Docs | `.github/workflows/docs.yml` | `pull_request`, `push` to `main` on docs paths | No | Yes | Documentation |
| Reproducible | `.github/workflows/reproducible.yml` | `pull_request`, tag `v*`, `schedule` weekly | No | No | Release Engineering |
| Verify Binaries | `.github/workflows/verify-binaries.yml` | Tag `v*`, `workflow_dispatch`, `schedule` weekly | No | No | Release Engineering |

### Reusable workflows

The reusable workflows are invoked by the top-level workflows. Each
declares `on: workflow_call` and nothing else.

| Workflow | File | Invoked by | Owner |
|---|---|---|---|
| CI Tier 1 | `.github/workflows/ci-tier1.yml` | `ci.yml` | Release Engineering |
| CI Tier 2 | `.github/workflows/ci-tier2.yml` | `ci.yml` | Release Engineering |
| CI Tier 3 | `.github/workflows/ci-tier3.yml` | `ci.yml` (on push) | Release Engineering |
| Build | `.github/workflows/build.yml` | `release.yml` | Release Engineering |
| Cache | `.github/workflows/cache.yml` | every tier workflow | Release Engineering |

## Required-checks policy

The following jobs must pass before a pull request may be merged into
`main`. The list is the union of the required checks declared by `ci.yml`
and `docs.yml`; no other workflow contributes required checks.

### From `ci.yml`

**Tier 1 — fast, no-network, no-secret checks (target: under 2 minutes):**

- `tier1-format`
- `tier1-vet`
- `tier1-lint`
- `tier1-test-short`
- `tier1-build`

**Tier 2 — slower, still no-secret checks (target: under 15 minutes):**

- `test-matrix`
- `cross-compile`
- `smoke`
- `release-dry-run`

### From `docs.yml`

- `docs-markdown-lint`
- `docs-link-check`
- `docs-build`

### Advisory (never blocking)

- Every job in `security.yml`.
- Every job in `nightly.yml`.
- Every job in `reproducible.yml`.
- Every job in `verify-binaries.yml`.

Security, nightly, reproducible, and verify-binaries findings are reported
as workflow results and (for security) as SARIF uploads. They do not gate
merges. The rationale is that a slow or flaky advisory workflow must not
become a single point of failure for the entire repository.

## Workflow ownership

| Workflow | Owner | Review policy |
|---|---|---|
| `ci.yml` | Release Engineering | One Release Engineering reviewer. One Security reviewer for permission changes. |
| `release.yml` | Release Engineering | Two Release Engineering reviewers. One Security reviewer. |
| `security.yml` | Security | One Security reviewer. |
| `nightly.yml` | Release Engineering | One Release Engineering reviewer. |
| `docs.yml` | Documentation | One Documentation reviewer. One Release Engineering reviewer for link-check policy changes. |
| `reproducible.yml` | Release Engineering | One Release Engineering reviewer. |
| `verify-binaries.yml` | Release Engineering | One Release Engineering reviewer. |
| `ci-tier1.yml` | Release Engineering | One Release Engineering reviewer. One Security reviewer for permission changes. |
| `ci-tier2.yml` | Release Engineering | One Release Engineering reviewer. One Security reviewer for permission changes. |
| `ci-tier3.yml` | Release Engineering | One Release Engineering reviewer. One Security reviewer for permission changes. |
| `build.yml` | Release Engineering | One Release Engineering reviewer. |
| `cache.yml` | Release Engineering | One Release Engineering reviewer. |

Ownership is recorded in `internal/ci/workflows.go` and asserted by tests.

## Pinned-action policy

Every `uses:` in every workflow references a **full 40-hex-character SHA**,
never a tag or branch. This is a direct response to the 2025 tj-actions
incident, in which a mutable tag was moved to a malicious commit.

- The set of permitted actions is closed and recorded in
  `internal/ci.PinnedActions()`.
- `.github/dependabot.yml` groups all `github-actions` updates into a single
  weekly PR, so the pins stay current without flooding the repository with
  one PR per action.
- The contract tests `TestContract_EveryUsesIsPinnedToFullSHA` and
  `TestContract_NoMutableTagReferences` fail the build if any `uses:`
  references a tag or a non-SHA ref.
- `TestContract_EveryUsesIsInTheClosedSet` fails the build if any `uses:`
  references an action the model does not declare.

### Reusable-workflow exception

A reusable workflow is referenced as
`owner/repo/.github/workflows/file.yml@ref`. GitHub does not support
pinning a reusable workflow to a SHA in the same way it supports pinning
an action: the `@ref` may be a version tag, and the SLSA generator's own
documentation recommends a tag. The pinning policy therefore exempts
references whose path contains `/.github/workflows/`. The verification
mechanism for a reusable workflow is the attestation it produces, not the
pin.

## `GITHUB_TOKEN` permissions

Every workflow declares an explicit top-level `permissions:` block. The
default is `contents: read`; individual jobs widen only what they need.

| Workflow | Top-level permissions |
|---|---|
| `ci.yml` | `contents: read`, `pull-requests: read` |
| `release.yml` | `contents: read` |
| `security.yml` | `contents: read`, `security-events: write` |
| `nightly.yml` | `contents: read` |
| `docs.yml` | `contents: read` |
| `reproducible.yml` | `contents: read` |
| `verify-binaries.yml` | `contents: read` |
| `ci-tier1.yml` | `contents: read` |
| `ci-tier2.yml` | `contents: read` |
| `ci-tier3.yml` | `contents: read` |
| `build.yml` | `contents: read` |
| `cache.yml` | `contents: read`, `actions: write` |

The top-level block is the intersection of every job's needs, not the
union. A job that needs more declares its own block:

- `release.yml`'s `publish` job declares `contents: write` and
  `packages: write`.
- `release.yml`'s `sign` and `provenance` jobs declare `id-token: write`.
- `release.yml`'s `provenance` job declares `actions: read` and
  `contents: write`.
- `security.yml`'s `security-scorecard` job declares `id-token: write`
  and `actions: read`.
- `security.yml`'s `notify` job declares `issues: write`.

`id-token: write` is required for SLSA provenance and cosign keyless
signing. It is granted only to the release workflow's `sign` and
`provenance` jobs and to the security workflow's `scorecard` job.

## `pull_request_target` policy

No workflow uses `pull_request_target`. The event grants write permissions
and access to secrets to code from a fork, which is the root cause of a
long history of CI supply-chain compromises. If a future workflow needs it,
the change must:

1. be justified in a new ADR,
2. use the safe pattern (no checkout of the PR head, no execution of
   PR-controlled code), and
3. be reviewed by the Security owner.

`ci.yml`, `docs.yml`, and `reproducible.yml` use `pull_request`, which does
not grant secrets.

## Enforcement

The architecture is enforced by:

- `internal/ci/workflows_test.go` — unit tests for the typed model.
- `internal/ci/workflows_contract_test.go` — contract tests that read the
  YAML files and assert the pinned-action, permissions, and
  `pull_request_target` rules.
- `scripts/verify-ci-architecture.sh` — a shell entry point used by
  `Taskfile.yml` and by local developers.

A change that violates the architecture fails the tier-2 check
`TestContract_*` in `ci.yml`.
