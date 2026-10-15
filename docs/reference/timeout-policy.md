# Timeout Policy

> **Status:** Accepted
> **Owner:** Release Engineering
> **Related:** [CI Tiers](ci-tiers.md)

Every CI job declares an explicit `timeout-minutes`. The value is a
backstop against a hung job, not a budget enforcement: the budget is
enforced by the quarterly review of measured timings.

GitHub Actions' default timeout is 360 minutes (6 hours). A job
without an explicit `timeout-minutes` inherits that default; a hung
job would block a runner for 6 hours. The repository's policy is
that no job relies on the default.

The repository's CI is organized into four tiers. Tier 1 is the
fast-feedback tier; Tier 2 is the cross-platform tier; Tier 3 is the
depth tier; Tier 4 is the scheduled security and nightly tier. Each
tier's jobs and their timeouts are:

| Tier | Job | Timeout |
|---|---|---|
| Tier 1 | `tier1-format` | 15m |
| Tier 1 | `tier1-vet` | 15m |
| Tier 1 | `tier1-lint` | 15m |
| Tier 1 | `tier1-test-short` | 15m |
| Tier 1 | `tier1-build` | 15m |
| Tier 2 | `test-matrix` | 20m |
| Tier 2 | `cross-compile` | 20m |
| Tier 2 | `smoke` | 20m |
| Tier 2 | `release-dry-run` | 30m |
| Tier 3 | `reproducibility` | 45m |
| Tier 3 | `flake-check` | 45m |
| Tier 3 | `race-full` | 45m |
| Tier 3 | `coverage` | 45m |
| Tier 3 | `binary-verify-all` | 45m |
| Tier 4 | each security job | see security.yml |
| Tier 4 | each nightly job | see nightly.yml |

A job whose timeout is reached is killed by GitHub Actions; the job's
result is a failure. A future change that needed a longer timeout
would change the job's `timeout-minutes` and the corresponding model
entry in the same commit.
