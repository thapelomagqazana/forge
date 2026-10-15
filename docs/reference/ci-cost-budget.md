# CI Cost Budget

> **Status:** Accepted
> **Owner:** Release Engineering
> **Related:** [CI Tiers](ci-tiers.md), [CI Cache Strategy](ci-cache-strategy.md)

This document describes the repository's CI cost budget and the techniques the repository uses to stay within it. It is rendered from `internal/ci/cost.go`; a golden-file test asserts the two agree.

## The budget

The repository's monthly budget depends on its visibility:

- **Public repository.** 2000 minutes per month. GitHub Actions' free tier provides 2,000 minutes for public repositories.
- **Private repository.** 5000 minutes per month. The free tier provides 2,000; the target is set higher to accommodate a paid plan.

The alert threshold is **80%** of the budget. When the threshold is reached, an alert fires; the Release Engineering owner reviews the month's usage and identifies the driver.

## Cost drivers

The cost of a CI run is the sum, over every job, of the job's wall-clock minutes multiplied by its OS runner's multiplier. The drivers are:

### OS runner multipliers

| OS | Multiplier |
|---|---|
| linux | 1× |
| windows | 2× |
| macos | 10× |

A minute of wall-clock on a Linux runner costs 1 billing minute; on Windows, 2; on macOS, 10. The multipliers are the single largest driver of cost; a job that runs on macOS for 5 minutes costs as much as a job that runs on Linux for 50.

### Matrix size

A matrix job's cost is the sum of its cells' costs. Tier 2's matrix has six cells (three OSes × two Go versions); its cost is dominated by the macOS cells' 10× multiplier. A future change that reduced the matrix's OS coverage would reduce the cost, at the expense of cross-platform confidence.

### Cache effectiveness

The cache strategy (see [ci-cache-strategy.md](ci-cache-strategy.md)) reduces each job's wall-clock. A cold cache's first run downloads every module and compiles every object; a warm cache reuses the previous run's artifacts. The cache's hit-rate targets (90% for the module cache, 60% for the build cache) are the mechanism by which the reduction is measured.

### Concurrency control

The concurrency policy (see [concurrency-policy.md](concurrency-policy.md)) cancels superseded PR runs. A contributor who pushes five times in an hour pays for one run, not five: the first four are cancelled when the fifth supersedes them.

## Per-tier estimates

The table below estimates each tier's cost. The values are estimates, not measurements; the monthly review replaces them with actuals.

| Tier | Runner | Jobs | Per-job minutes | Runs on PR | Runs on main |
|---|---|---|---|---|---|
| Tier 1 | linux | 5 | 1 | yes | yes |
| Tier 2 | linux | 4 | 5 | yes | yes |
| Tier 3 | linux | 5 | 10 | no | yes |
| Tier 4 | linux | 8 | 8 | no | yes |

## Optimization techniques

The repository uses the following techniques to reduce its CI cost:

1. **Concurrency cancel-in-progress.** A superseded PR push's run is cancelled; the runner is freed for the new run. The technique reduces the billed minutes for a rapid series of PR pushes. See `internal/ci/concurrency.go (CancelPolicyOnPR)`.
2. **Skip CI for docs-only changes.** A change under docs/ or to a Markdown file does not trigger the CI tier. The technique eliminates the CI cost of documentation changes. See `.github/workflows/ci.yml (on.pull_request.paths-ignore)`.
3. **Cache aggressively.** The Go module cache, the Go build cache, and the linter binary cache reduce the wall-clock of every job. The technique reduces the billed minutes proportionally. See `internal/ci/cache.go`.
4. **Fail-fast off, cancel-on-fail for matrix.** A matrix job with fail-fast: false reports all cells' failures in one run. The technique avoids a second run that would only re-detect the same failure. See `internal/ci/tier2.go, internal/ci/tier3.go (the matrix jobs)`.
5. **Run expensive jobs on main only.** Tier 3's depth checks are gated on main pushes. The technique pays the depth checks' cost once per main push, not once per PR push. See `internal/ci/tiers.go (Tier3.Advisory), .github/workflows/ci.yml (the tier3 gate)`.
6. **Use Linux runners where possible.** Linux's 1× multiplier is the cheapest. The technique confines the Windows (2×) and macOS (10×) multipliers to the workflows that need them. See `internal/ci/platform_matrix.go, .github/workflows/ci-tier2.yml`.

## Cost dashboard

The monthly review reports three metrics:

- **Monthly minutes used.** The sum, over the month's runs, of each job's wall-clock minutes   multiplied by its OS runner's multiplier.
- **Cost per PR.** The month's total minutes divided by the number of PRs merged in the month.   Target: < 50 minutes per PR.
- **Cost per main push.** The month's total minutes divided by the number of main pushes in the month.   Target: < 20 minutes per main push.

The metrics' source is GitHub's billing API (`gh api /orgs/<org>/settings/billing/actions`). The review is documented in the Release Engineering calendar; a future change that wired the review into a scheduled workflow would add the workflow, not change the budget.
