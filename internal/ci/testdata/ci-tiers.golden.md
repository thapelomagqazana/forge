# CI Tier Model

> **Status:** Accepted
> **Owner:** Release Engineering
> **Related:** [ADR-044](../decisions/ADR-044-ci-tier-model.md), [CI Architecture](../architecture/ci-architecture.md)

The four tiers below are the contract between the CI architecture and the
contributors who depend on it. A tier is a class of checks with a shared
goal, a shared timing budget, and a shared blocking status per trigger
event. The table is rendered from `internal/ci/tiers.go`; a golden-file
test asserts that this document matches the model.

## Summary

| Tier | Name | Budget | Blocking | Triggers |
|---|---|---|---|---|
| Tier 1 | Fast feedback | 3m | pull_request, push | pull_request, push |
| Tier 2 | Cross-platform confidence | 15m | pull_request, push | pull_request, push |
| Tier 3 | Depth | 1h | push | pull_request, push |
| Tier 4 | Scheduled hygiene | 2h | advisory | schedule, workflow_dispatch |

## Detail

### Tier 1 — Fast feedback

**Goal.** Catch the majority of mistakes before a reviewer is notified.

**Budget.** 3m

**Blocking.** pull_request, push

**Triggers.** pull_request, push

**Contents.**

- `tier1-format` (target 30s, in ci.yml; gofmt and goimports)
- `tier1-vet` (target 30s, in ci.yml; go vet)
- `tier1-lint` (target 1m 30s, in ci.yml; golangci-lint)
- `tier1-test-short` (target 1m, in ci.yml; go test -short ./...)
- `tier1-build` (target 30s, in ci.yml; go build ./...)

### Tier 2 — Cross-platform confidence

**Goal.** Prove the code works on every supported OS and Go version.

**Budget.** 15m

**Blocking.** pull_request, push

**Triggers.** pull_request, push

**Contents.**

- `tier2-test-full` (target 5m, in ci.yml; full unit and integration tests)
- `tier2-cross-compile` (target 3m, in ci.yml; all six release platforms)
- `tier2-smoke` (target 2m, in ci.yml; binary smoke tests on ubuntu)
- `tier2-matrix` (target 15m, in ci.yml; OS and Go version matrix)

### Tier 3 — Depth

**Goal.** Catch races, flakes, and integration failures that the fast tiers cannot.

**Budget.** 1h

**Blocking.** push

**Triggers.** pull_request, push

**Contents.**

- `tier3-race` (target 20m, in ci.yml; go test -race ./...)
- `tier3-count2` (target 20m, in ci.yml; go test -count=2 ./... for flake detection)
- `tier3-cross-compile` (target 5m, in ci.yml; all six release platforms)
- `tier3-release-dry-run` (target 20m, in ci.yml; make release-dry-run)

### Tier 4 — Scheduled hygiene

**Goal.** Detect drift that is invisible to per-commit checks: vulnerabilities, secrets, licenses, fuzz-discovered bugs, and coverage regressions.

**Budget.** 2h

**Blocking.** advisory

**Triggers.** schedule, workflow_dispatch

**Contents.**

- `tier4-codeql` (target 30m, in security.yml; CodeQL analysis)
- `tier4-govulncheck` (target 10m, in security.yml; govulncheck)
- `tier4-secret-scan` (target 5m, in security.yml; gitleaks)
- `tier4-license-scan` (target 5m, in security.yml; go-licenses)
- `tier4-fuzz` (target 1h, in nightly.yml; go test -fuzz=Fuzz -fuzztime=1h)
- `tier4-coverage` (target 10m, in nightly.yml; coverage regression check)
