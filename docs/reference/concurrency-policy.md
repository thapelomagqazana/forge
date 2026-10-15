# Concurrency Policy

> **Status:** Accepted
> **Owner:** Release Engineering
> **Related:** [CI Tiers](ci-tiers.md)

This document describes the concurrency policy each workflow declares. It is rendered from `internal/ci/concurrency.go`; a golden-file test asserts the two agree.

## The two policies

The repository uses two concurrency policies:

- **`on-pr`.** Cancel the superseded run if the ref is not `refs/heads/main`.   The superseded PR push is stale; the superseded main push still needs verification.
- **`never`.** Never cancel. Used by workflows triggered by tags,   schedules, or release inputs.

## Per-workflow policy

| Workflow | Group | Cancel | Policy | Rationale |
|---|---|---|---|---|
| `ci` | `ci-${{ github.workflow }}-${{ github.ref }}` | `${{ github.ref != 'refs/heads/main' }}` | `on-pr` | A superseded PR push is stale; cancelling it frees a runner for the new run. A superseded main push still needs verification. |
| `release` | `release-${{ github.workflow }}-${{ inputs.version &#124;&#124; github.ref }}` | `false` | `never` | A tag is immutable; a second run for the same tag is a retry or a bug. Serializing is the safe default. The group carries `github.workflow` so a release run cannot share a concurrency group with a tier run on the same ref, matching the shape every other workflow's group follows. |
| `security` | `security-${{ github.workflow }}-${{ github.ref }}` | `false` | `never` | Scheduled runs are the only run for their slot; cancelling one wastes its work. Push-triggered runs are rare and their inputs change infrequently. |
| `nightly` | `nightly-${{ github.workflow }}-${{ github.ref }}` | `false` | `never` | Same as security.yml. |
| `docs` | `docs-${{ github.workflow }}-${{ github.ref }}` | `${{ github.ref != 'refs/heads/main' }}` | `on-pr` | Same as ci.yml: PR pushes supersede; main pushes do not. |
| `reproducible` | `reproducible-${{ github.workflow }}-${{ github.ref }}` | `false` | `never` | The workflow is triggered by pull_request, tags, and schedule. Tags and schedules are the triggers whose runs must not cancel each other; a PR's superseded pushes are rare and their cost is bounded. The never policy is the conservative choice. |
| `verify-binaries` | `verify-binaries-${{ github.workflow }}-${{ github.ref }}` | `false` | `never` | Tag pushes and scheduled runs are the only triggers; neither is superseded by a later run in the same way a PR push is. |
| `ci-tier1` | `ci-tier1-${{ github.workflow }}-${{ github.ref }}` | `${{ github.ref != 'refs/heads/main' }}` | `on-pr` | Same as ci.yml: PR pushes supersede; main pushes do not. |
| `ci-tier2` | `ci-tier2-${{ github.workflow }}-${{ github.ref }}` | `${{ github.ref != 'refs/heads/main' }}` | `on-pr` | Same as ci.yml: PR pushes supersede; main pushes do not. |
| `ci-tier3` | `ci-tier3-${{ github.workflow }}-${{ github.ref }}` | `${{ github.ref != 'refs/heads/main' }}` | `on-pr` | Same as ci.yml: PR pushes supersede; main pushes do not. The tier is gated on push, so the policy's PR branch is rarely exercised, but the block is present for consistency with the other tiers. |
| `build` | `build-${{ github.workflow }}-${{ github.ref }}` | `false` | `never` | A reusable workflow invoked by release.yml; the caller serializes, and the build itself has no supersession semantics. |
| `cache` | `cache-${{ github.workflow }}-${{ github.ref }}` | `false` | `never` | A reusable workflow invoked by every tier; the caller serializes, and the cache itself has no supersession semantics. |

## The group template

Every group template contains a discriminator and a scope. The discriminator is either `github.workflow` (the workflow's name) or `inputs.` (a workflow input); the scope is either `github.ref` (the ref) or `inputs.` (a workflow input). The two together ensure that two runs of the same workflow on the same ref serialize, and runs of different workflows on the same ref do not collide.
