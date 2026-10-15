# Security Workflow Reference

> **Status:** Accepted
> **Owner:** Security
> **Related:** [CI Tiers](ci-tiers.md), [Platform Tiers](platform-tiers.md)

This document describes the security scanning workflow
(`.github/workflows/security.yml`). The workflow is rendered
from `internal/ci/security.go`; a golden-file test asserts the
two agree.

## Trigger

The workflow runs on:

- **Schedule.** `0 2 * * *` (02:00 UTC daily).
- **Push to main.** Every push to the `main` branch.
- **Manual.** `workflow_dispatch` for a manual run.

## Jobs

The workflow has eight jobs, running in parallel:

| Job | Timeout | SARIF | Retention | Note |
|---|---|---|---|---|
| `security-codeql` | 60m | yes | — | Go CodeQL analysis; results appear in the Security tab. |
| `security-govulncheck` | 30m | no | — | Go vulnerability database scan; reachable vulnerabilities fail the job. |
| `security-gitleaks` | 30m | yes | 90d | Secret scan on the full git history. |
| `security-grype` | 30m | yes | — | Vulnerability scan of the release artifacts with grype. |
| `security-scorecard` | 30m | yes | — | OpenSSF Scorecard; fails below the threshold. |
| `security-license-scan` | 30m | no | — | License compatibility check; fails on a forbidden license. |
| `security-sbom` | 30m | no | 90d | SBOM generation with syft; retained 90 days. |
| `security-dependabot` | 5m | no | — | Verify .github/dependabot.yml is configured for gomod and github-actions. |

## Thresholds

The workflow's pass/fail thresholds are:

- **Scorecard.** Score ≥ 7.0 (below the threshold fails the job).
- **Forbidden licenses.** A dependency whose license is one
  of the following fails the license scan: GPL-2.0, GPL-3.0, AGPL-3.0, LGPL-2.1, LGPL-3.0, SSPL-1.0, BUSL-1.1.

## Tool versions

The tools the workflow uses are pinned to specific versions:

| Tool | Version |
|---|---|
| `gitleaks` | `8.21.2` |
| `go-licenses` | `v1.6.0` |
| `govulncheck` | `v1.1.4` |
| `grype` | `v0.87.0` |
| `syft` | `v1.18.1` |

## Notifications

A workflow failure triggers the `notify` job, which opens an
issue tagged `security` and `ci-failure`. The issue links to
the failing run and lists the eight jobs so the reviewer knows
which failed.
