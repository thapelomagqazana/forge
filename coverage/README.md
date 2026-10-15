# Coverage Baseline

This directory holds the coverage baseline file that the Tier 3 coverage
job compares against.

## Contents

- [`baseline.txt`](./baseline.txt) — the baseline file. Read by
  `.github/workflows/ci-tier3.yml`'s `coverage` job.
- `README.md` — this file.

## What the baseline is for

The Tier 3 coverage job runs the full test suite with
`go test -coverprofile`, extracts the total statement coverage, and
compares it against the value in `baseline.txt`. A decrease greater than
the threshold fails the job; a smaller decrease produces a warning but
does not fail the job.

The threshold is declared in `internal/ci/tier3.go` as
`Tier3CoverageThreshold`. The current value is 0.5 percentage points. The
Go constant is the authoritative definition; this README describes the
behaviour but does not restate the value as a contract.

The comparison is deliberately local: it does not query Codecov's API. See
"Why the baseline is in the repository" below for the rationale.

## `baseline.txt` format

The file is a plain-text file with one line per metric. Each line has the
form:

```text
key: value
```

Keys are lowercase ASCII identifiers. The only key currently read by the
Tier 3 coverage job is `total`. A future metric can be added as a second
line; the parser ignores unknown keys, so adding one does not break
existing readers.

The `total` value is a floating-point number in `[0, 1]`, representing
total statement coverage over the full test suite (not `-short`). Example:

```text
total: 0.7842
```

A value of `0.7842` means 78.42% of statements are covered. The Tier 3
job multiplies by 100 to compare against the percentage that
`go tool cover -func` prints.

Lines beginning with `#` are comments and are ignored.

## Reading the baseline in a pull request

When a contributor's change affects the coverage baseline, the diff shows:

```text
-total: 0.7842
+total: 0.7915
```

A reviewer sees the old value, the new value, and the contributor's
justification in the PR description. The baseline's presence in the
repository is what makes this possible: a Codecov-only approach would
require a reviewer to visit Codecov's website to see the same
information.

## Updating the baseline

Run:

```sh
task coverage:update-baseline
```

The task:

1. Runs `go test -count=1 -covermode=atomic -coverprofile=/tmp/coverage.out ./...`
   over the full test suite (not `-short`).
2. Runs `go tool cover -func=/tmp/coverage.out | tail -n 1` to get the
   total.
3. Parses the total and writes it to `coverage/baseline.txt`, preserving
   the file's header comment.

The task requires a clean working tree: it refuses to run if `git status
--porcelain` is non-empty. This is a safety check: the coverage of an
uncommitted change is not the coverage the baseline should record.

The task is run deliberately, not on every push to main. See "Why the
baseline is not updated automatically" below.

## Why the baseline is in the repository

The baseline is a file in the repository, not a query to Codecov's API.
Three reasons:

1. **Determinism.** The check is a float comparison between the current
   run's total and the baseline's value. It does not depend on a
   third-party service's availability, API version, or caching behaviour.
   The check either passes or fails based on the repository's own
   contents.

2. **Reviewability.** A change to the baseline is a diff in a pull
   request. A reviewer sees the old value, the new value, and the
   contributor's justification in one place. A Codecov-only approach
   would show the same information on a different website, which a
   reviewer might not visit.

3. **Testability.** The baseline's format is pinned by a schema test in
   `internal/ci/tier3_contract_test.go`. The test reads the file, parses
   it, and asserts its shape. Codecov's API is not testable without
   network access and a stable test tenant.

The coverage *profile* is still uploaded to Codecov (for the dashboard's
benefit), but the dashboard is not the source of truth for the check.

## Why the baseline is not updated automatically

An automatic update would let a small decrease become the new baseline
silently. A contributor who removes a test and does not notice would see
the coverage decrease, the job pass with a warning, and the next main
push's baseline drop by the same amount. The decrease would become the
new normal without a reviewer seeing it.

The deliberate pace forces the decrease to be visible. A contributor who
wants to lower the baseline runs `task coverage:update-baseline`, and the
resulting change is a diff in a pull request. A reviewer sees the
decrease, evaluates whether it is justified, and either approves or asks
for the test to be restored.

The same applies to increases: a contributor who adds a large test suite
runs the task, and the resulting increase is a diff. The baseline can
rise as well as fall, but only with a reviewer's acknowledgement.

## What happens if the baseline is stale

If the baseline's value is higher than the current run's total, the job
fails if the decrease exceeds the threshold and warns otherwise. A
contributor who sees a failure has two options:

1. Restore or add tests to bring the coverage back to the baseline.
2. Update the baseline to reflect a deliberate decrease, and justify the
   change in the PR description.

Both options are visible in the PR's diff. The baseline is not a rule
imposed on the contributor; it is a shared record that the contributor
updates as the code evolves.

## What happens if the baseline file is missing

The Tier 3 coverage job reads `coverage/baseline.txt`. If the file is
missing, the job fails with an error naming the path. The contract test
`TestTier3_BaselineFileExists` asserts the file's presence before the
workflow runs; a missing file is a test failure, not a runtime surprise.

If the file exists but has no `total:` line, the job fails with an error
naming the file. The contract test `TestTier3_BaselineFormatIsValid`
asserts the presence of the line.

## Related documents

- [`../internal/ci/tier3.go`](../internal/ci/tier3.go) — the Tier 3
  model, including `Tier3BaselinePath` and `Tier3CoverageThreshold`.
- [`../internal/ci/tier3_contract_test.go`](../internal/ci/tier3_contract_test.go)
  — the contract tests that read this directory.
- [`../docs/reference/ci-tiers.md`](../docs/reference/ci-tiers.md) — the
  tier model's published table.
- [`../docs/decisions/ADR-047-tier3-depth-workflow.md`](../docs/decisions/ADR-047-tier3-depth-workflow.md)
  — the ADR for Tier 3, including the decision to use a local baseline.
- [`../.github/workflows/ci-tier3.yml`](../.github/workflows/ci-tier3.yml)
  — the workflow that reads the baseline.
