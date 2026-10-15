# Flake Policy

> **Status:** Accepted
> **Owner:** Release Engineering
> **Related:** [CI Tiers](ci-tiers.md)

This document describes the repository's policy for flaky tests. It is rendered from `internal/ci/flake.go`; a golden-file test asserts the two agree.

## The policy in one paragraph

A flaky test is a **P0 bug**. The test is quarantined within **24h** of detection and fixed within **1w**. The quarantine mechanism is `t.Skip("flaky: #%d")` with a linked issue. **No automatic retries**: a retry hides the flake; the CI must fail so that the flake is visible. The flake rate target is **0.5%** per test.

## Quarantine mechanism

A quarantined test carries a `t.Skip` call whose message starts with `flaky:` and includes the number of the issue that tracks the fix:

```go
func TestSomethingFlaky(t *testing.T) {
    t.Skip("flaky: #42")
    // ... the test body
}
```

The `#42` is the issue number. The issue is tagged `flaky` and `P0`; its title names the test and the failure mode.

## Detection

Tier 3's flake-check job runs `go test -count=2 ./...`. A test that passes on the first run and fails on the second is a flake. The job uploads a `flake-report` artifact with the failing tests' names.

A future aggregator fetches the last N flake-report artifacts and computes the per-test flake rate. The aggregator is out of scope for this task; the target and the artifact's name are the interface it will consume.

## Retry policy

The repository's retry policy is **none**. The CI does not retry a failed test automatically. A contributor who sees a failure re-runs the job manually if they believe the failure is spurious; the manual re-run is a deliberate action, not a hidden retry.

## Target

The flake rate target is **0.5%** per test. A test whose rate exceeds the target is quarantined; a test whose rate exceeds the target after a fix is a candidate for removal or redesign.
