// Package ci — typed model of the CI cost budget.
//
// This file defines the cost budget for the repository's CI usage. It
// captures:
//
//   - The monthly budget, expressed for both public and private
//     repositories. GitHub Actions' free tier is 2,000 minutes per
//     month for public repositories and 3,000 for private; a paid
//     plan raises the private limit. The model records the two
//     targets the task names: 2,000 for public, 5,000 for private.
//
//   - The OS cost multipliers: Linux 1×, Windows 2×, macOS 10×. The
//     multipliers are GitHub Actions' published pricing; the model
//     records them as constants.
//
//   - The per-tier cost estimate: the model's estimate of each
//     tier's minutes per run, and the total minutes per PR and per
//     main push.
//
//   - The alert threshold: 80% of the monthly budget.
//
//   - The optimization techniques the repository uses. Each
//     technique is a named entry with a rationale and a pointer to
//     the mechanism that implements it.
//
// # Why the budget is modelled
//
// The budget has structure: two values, a multiplier table, five
// optimization techniques, and a threshold. The model captures the
// structure; the contract tests assert the workflows' structure
// against the model's per-tier estimates.
//
// # What the model is not
//
// It is not a measurement. The actual minutes a run consumes are
// recorded by GitHub's billing API; the model records the budget
// and the estimates. A future change that wired the billing API
// into the repository would add a separate reader; the model would
// remain the budget's source of truth.
package ci

import (
	"fmt"
	"strings"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// OSRunnerMultiplier is the closed set of OS cost multipliers.
//
// The values are GitHub Actions' published pricing: a minute on a
// Linux runner costs one billing minute; a minute on a Windows
// runner costs two; a minute on a macOS runner costs ten. The
// multipliers apply to the billed minutes, not to the wall-clock.
type OSRunnerMultiplier string

const (
	// OSRunnerLinux is the Linux runner's multiplier. It is 1: a
	// minute of wall-clock is a minute of billing.
	OSRunnerLinux OSRunnerMultiplier = "linux"

	// OSRunnerWindows is the Windows runner's multiplier. It is 2.
	OSRunnerWindows OSRunnerMultiplier = "windows"

	// OSRunnerMacOS is the macOS runner's multiplier. It is 10.
	OSRunnerMacOS OSRunnerMultiplier = "macos"
)

// AllOSRunnerMultipliers returns the closed set of multipliers, in the
// order they are presented in the cost-budget document: Linux,
// Windows, macOS.
func AllOSRunnerMultipliers() []OSRunnerMultiplier {
	return []OSRunnerMultiplier{
		OSRunnerLinux,
		OSRunnerWindows,
		OSRunnerMacOS,
	}
}

// String returns the multiplier's identifier as a string.
func (m OSRunnerMultiplier) String() string { return string(m) }

// Multiplier returns the numeric multiplier for the OS.
//
// The values are the ones the cost-budget document renders as a
// table. A future change to GitHub's pricing would update these
// values; the document's table would regenerate.
func (m OSRunnerMultiplier) Multiplier() int {
	switch m {
	case OSRunnerLinux:
		return 1
	case OSRunnerWindows:
		return 2
	case OSRunnerMacOS:
		return 10
	default:
		panic(fmt.Sprintf("ci: unknown OS runner multiplier %q", string(m)))
	}
}

// Valid reports whether m is a member of the closed set.
func (m OSRunnerMultiplier) Valid() bool {
	switch m {
	case OSRunnerLinux, OSRunnerWindows, OSRunnerMacOS:
		return true
	default:
		return false
	}
}

// GitHubBillingMultiplier is the closed set of OS multipliers as
// integers. It is provided for callers that compute costs; the
// string-based OSRunnerMultiplier is used for identification.
type GitHubBillingMultiplier int

// The multipliers.
const (
	MultiplierLinux   GitHubBillingMultiplier = 1
	MultiplierWindows GitHubBillingMultiplier = 2
	MultiplierMacOS   GitHubBillingMultiplier = 10
)

// CostBudget is the typed description of the monthly cost budget.
type CostBudget struct {
	// PublicMonthlyMinutes is the monthly budget for a public
	// repository, in minutes. GitHub Actions' free tier provides
	// 2,000 minutes per month for public repositories.
	PublicMonthlyMinutes int

	// PrivateMonthlyMinutes is the monthly budget for a private
	// repository, in minutes. GitHub Actions' free tier provides
	// 2,000 minutes for private repositories; paid plans raise the
	// limit. The task names 5,000 as the private budget.
	PrivateMonthlyMinutes int

	// AlertThreshold is the fraction of the budget at which an
	// alert is raised. The value is 0.80: an alert fires when 80%
	// of the budget has been consumed.
	AlertThreshold float64

	// TargetMinutesPerPR is the target average minutes consumed by
	// a pull request's CI. The value is a target, not a hard cap;
	// the monthly review reports the actual value.
	TargetMinutesPerPR int

	// TargetMinutesPerMainPush is the target average minutes
	// consumed by a main push's CI. The value is a target, not a
	// hard cap.
	TargetMinutesPerMainPush int
}

// CostBudgetOf returns the canonical budget.
func CostBudgetOf() CostBudget {
	return costBudget
}

// costBudget is the canonical budget.
var costBudget = CostBudget{
	PublicMonthlyMinutes:     2000,
	PrivateMonthlyMinutes:    5000,
	AlertThreshold:           0.80,
	TargetMinutesPerPR:       50,
	TargetMinutesPerMainPush: 20,
}

// TierCost is the typed description of one tier's cost estimate.
type TierCost struct {
	// Tier is the tier's identifier.
	Tier TierID

	// Runner is the OS multiplier the tier's runners use. For a
	// tier with multiple OSes (Tier 2), the value is the tier's
	// dominant runner; the tier's estimate accounts for the others
	// in the JobCount and PerJobMinutes fields.
	Runner OSRunnerMultiplier

	// JobCount is the tier's job count.
	JobCount int

	// PerJobMinutes is the tier's estimate of each job's wall-clock
	// minutes. It is an estimate, not a measurement.
	PerJobMinutes int

	// RunsOnPR is true if the tier runs on every pull request.
	RunsOnPR bool

	// RunsOnMain is true if the tier runs on every main push.
	RunsOnMain bool

	// Notes is a short explanation of the tier's cost drivers.
	Notes string
}

// costByTier is the canonical per-tier cost estimate.
var costByTier = []TierCost{
	{
		Tier:          Tier1,
		Runner:        OSRunnerLinux,
		JobCount:      5,
		PerJobMinutes: 1,
		RunsOnPR:      true,
		RunsOnMain:    true,
		Notes:         "Five fast jobs on Ubuntu. With a warm cache, ~1 minute per job wall-clock.",
	},
	{
		Tier:          Tier2,
		Runner:        OSRunnerLinux,
		JobCount:      4,
		PerJobMinutes: 5,
		RunsOnPR:      true,
		RunsOnMain:    true,
		Notes: "Four jobs; the matrix job runs six cells across three OSes. " +
			"The macOS cells' 10× multiplier dominates the tier's cost.",
	},
	{
		Tier:          Tier3,
		Runner:        OSRunnerLinux,
		JobCount:      5,
		PerJobMinutes: 10,
		RunsOnPR:      false,
		RunsOnMain:    true,
		Notes: "Five depth jobs on Ubuntu. The tier is gated on main, so its cost " +
			"is paid once per main push, not per PR push.",
	},
	{
		Tier:          Tier4,
		Runner:        OSRunnerLinux,
		JobCount:      8,
		PerJobMinutes: 8,
		RunsOnPR:      false,
		RunsOnMain:    true,
		Notes: "Eight security jobs on Ubuntu. The tier runs on the daily schedule " +
			"and on main pushes; the scheduled runs dominate the tier's cost.",
	},
}

// AllTierCosts returns a copy of the canonical per-tier costs.
func AllTierCosts() []TierCost {
	out := make([]TierCost, len(costByTier))
	copy(out, costByTier)
	return out
}

// Optimization is the typed description of one cost-optimization
// technique.
type Optimization struct {
	// Name is the technique's human-readable name.
	Name string

	// Rationale is a one-sentence explanation of the technique's
	// effect on cost.
	Rationale string

	// Mechanism is the name of the model or file that implements
	// the technique. It appears in the cost-budget document as a
	// pointer.
	Mechanism string
}

// optimizations is the canonical list of optimization techniques.
var optimizations = []Optimization{
	{
		Name: "Concurrency cancel-in-progress",
		Rationale: "A superseded PR push's run is cancelled; the runner is freed for the new run. " +
			"The technique reduces the billed minutes for a rapid series of PR pushes.",
		Mechanism: "internal/ci/concurrency.go (CancelPolicyOnPR)",
	},
	{
		Name: "Skip CI for docs-only changes",
		Rationale: "A change under docs/ or to a Markdown file does not trigger the CI tier. " +
			"The technique eliminates the CI cost of documentation changes.",
		Mechanism: ".github/workflows/ci.yml (on.pull_request.paths-ignore)",
	},
	{
		Name: "Cache aggressively",
		Rationale: "The Go module cache, the Go build cache, and the linter binary cache reduce " +
			"the wall-clock of every job. The technique reduces the billed minutes proportionally.",
		Mechanism: "internal/ci/cache.go",
	},
	{
		Name: "Fail-fast off, cancel-on-fail for matrix",
		Rationale: "A matrix job with fail-fast: false reports all cells' failures in one run. " +
			"The technique avoids a second run that would only re-detect the same failure.",
		Mechanism: "internal/ci/tier2.go, internal/ci/tier3.go (the matrix jobs)",
	},
	{
		Name: "Run expensive jobs on main only",
		Rationale: "Tier 3's depth checks are gated on main pushes. The technique pays the " +
			"depth checks' cost once per main push, not once per PR push.",
		Mechanism: "internal/ci/tiers.go (Tier3.Advisory), .github/workflows/ci.yml (the tier3 gate)",
	},
	{
		Name: "Use Linux runners where possible",
		Rationale: "Linux's 1× multiplier is the cheapest. The technique confines the " +
			"Windows (2×) and macOS (10×) multipliers to the workflows that need them.",
		Mechanism: "internal/ci/platform_matrix.go, .github/workflows/ci-tier2.yml",
	},
}

// AllOptimizations returns a copy of the canonical optimizations.
func AllOptimizations() []Optimization {
	out := make([]Optimization, len(optimizations))
	copy(out, optimizations)
	return out
}

// ValidateCost checks the cost model's internal invariants.
//
// The invariants are:
//
//  1. The public budget is positive and less than or equal to the
//     private budget.
//  2. The alert threshold is in (0, 1).
//  3. The per-PR and per-main-push targets are positive.
//  4. Every tier has a cost estimate.
//  5. Every cost estimate's runner is a member of the closed set.
//  6. Every cost estimate has a positive job count and per-job
//     minutes.
//  7. The optimizations list is non-empty.
//  8. The OS multipliers' numeric values are the ones the document
//     names (Linux 1, Windows 2, macOS 10).
func ValidateCost() error {
	if costBudget.PublicMonthlyMinutes <= 0 {
		return costError(
			"non-positive public budget",
			fmt.Sprintf("PublicMonthlyMinutes = %d", costBudget.PublicMonthlyMinutes),
		)
	}
	if costBudget.PrivateMonthlyMinutes <= 0 {
		return costError(
			"non-positive private budget",
			fmt.Sprintf("PrivateMonthlyMinutes = %d", costBudget.PrivateMonthlyMinutes),
		)
	}
	if costBudget.PublicMonthlyMinutes > costBudget.PrivateMonthlyMinutes {
		return costError(
			"public budget exceeds private budget",
			fmt.Sprintf(
				"PublicMonthlyMinutes = %d, PrivateMonthlyMinutes = %d",
				costBudget.PublicMonthlyMinutes, costBudget.PrivateMonthlyMinutes,
			),
		)
	}
	if costBudget.AlertThreshold <= 0 || costBudget.AlertThreshold >= 1 {
		return costError(
			"alert threshold out of range",
			fmt.Sprintf("AlertThreshold = %v, want a value in (0, 1)", costBudget.AlertThreshold),
		)
	}
	if costBudget.TargetMinutesPerPR <= 0 {
		return costError(
			"non-positive per-PR target",
			fmt.Sprintf("TargetMinutesPerPR = %d", costBudget.TargetMinutesPerPR),
		)
	}
	if costBudget.TargetMinutesPerMainPush <= 0 {
		return costError(
			"non-positive per-main-push target",
			fmt.Sprintf("TargetMinutesPerMainPush = %d", costBudget.TargetMinutesPerMainPush),
		)
	}

	seen := make(map[TierID]struct{}, len(costByTier))
	for _, c := range costByTier {
		if !c.Tier.Valid() {
			return costError(
				"unknown tier",
				fmt.Sprintf("tier %q is not in the closed set", string(c.Tier)),
			)
		}
		if _, dup := seen[c.Tier]; dup {
			return costError(
				"duplicate tier",
				fmt.Sprintf("tier %q has more than one cost estimate", string(c.Tier)),
			)
		}
		seen[c.Tier] = struct{}{}
		if !c.Runner.Valid() {
			return costError(
				"unknown runner",
				fmt.Sprintf("tier %q has runner %q", string(c.Tier), string(c.Runner)),
			)
		}
		if c.JobCount <= 0 {
			return costError(
				"non-positive job count",
				fmt.Sprintf("tier %q has JobCount = %d", string(c.Tier), c.JobCount),
			)
		}
		if c.PerJobMinutes <= 0 {
			return costError(
				"non-positive per-job minutes",
				fmt.Sprintf("tier %q has PerJobMinutes = %d", string(c.Tier), c.PerJobMinutes),
			)
		}
	}
	for _, id := range AllTiers() {
		if _, ok := seen[id]; !ok {
			return costError(
				"missing tier",
				fmt.Sprintf("tier %q has no cost estimate", string(id)),
			)
		}
	}

	if len(optimizations) == 0 {
		return costError("empty optimizations", "no optimization techniques declared")
	}

	// The multipliers' numeric values must match the document.
	if OSRunnerLinux.Multiplier() != int(MultiplierLinux) {
		return costError(
			"linux multiplier mismatch",
			fmt.Sprintf(
				"OSRunnerLinux.Multiplier() = %d, MultiplierLinux = %d",
				OSRunnerLinux.Multiplier(), int(MultiplierLinux),
			),
		)
	}
	if OSRunnerWindows.Multiplier() != int(MultiplierWindows) {
		return costError(
			"windows multiplier mismatch",
			fmt.Sprintf(
				"OSRunnerWindows.Multiplier() = %d, MultiplierWindows = %d",
				OSRunnerWindows.Multiplier(), int(MultiplierWindows),
			),
		)
	}
	if OSRunnerMacOS.Multiplier() != int(MultiplierMacOS) {
		return costError(
			"macos multiplier mismatch",
			fmt.Sprintf(
				"OSRunnerMacOS.Multiplier() = %d, MultiplierMacOS = %d",
				OSRunnerMacOS.Multiplier(), int(MultiplierMacOS),
			),
		)
	}

	return nil
}

// CostGolden renders the cost model as deterministic JSON.
func CostGolden() string {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(`  "public_monthly_minutes": `)
	b.WriteString(itoa(costBudget.PublicMonthlyMinutes))
	b.WriteString(",\n")
	b.WriteString(`  "private_monthly_minutes": `)
	b.WriteString(itoa(costBudget.PrivateMonthlyMinutes))
	b.WriteString(",\n")
	b.WriteString(`  "alert_threshold": `)
	b.WriteString(fmt.Sprintf("%.2f", costBudget.AlertThreshold))
	b.WriteString(",\n")
	b.WriteString(`  "target_minutes_per_pr": `)
	b.WriteString(itoa(costBudget.TargetMinutesPerPR))
	b.WriteString(",\n")
	b.WriteString(`  "target_minutes_per_main_push": `)
	b.WriteString(itoa(costBudget.TargetMinutesPerMainPush))
	b.WriteString(",\n")
	b.WriteString(`  "multipliers": {`)
	for i, m := range AllOSRunnerMultipliers() {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    ")
		writeJSONString(&b, string(m))
		b.WriteString(": ")
		b.WriteString(itoa(m.Multiplier()))
	}
	b.WriteString("\n  },\n")
	b.WriteString(`  "tiers": [`)
	for i, c := range costByTier {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    {\n")
		b.WriteString(`      "tier": `)
		writeJSONString(&b, string(c.Tier))
		b.WriteString(",\n")
		b.WriteString(`      "runner": `)
		writeJSONString(&b, string(c.Runner))
		b.WriteString(",\n")
		b.WriteString(`      "job_count": `)
		b.WriteString(itoa(c.JobCount))
		b.WriteString(",\n")
		b.WriteString(`      "per_job_minutes": `)
		b.WriteString(itoa(c.PerJobMinutes))
		b.WriteString(",\n")
		b.WriteString(`      "runs_on_pr": `)
		if c.RunsOnPR {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		b.WriteString(",\n")
		b.WriteString(`      "runs_on_main": `)
		if c.RunsOnMain {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		b.WriteString(",\n")
		b.WriteString(`      "notes": `)
		writeJSONString(&b, c.Notes)
		b.WriteString("\n    }")
	}
	b.WriteString("\n  ],\n")
	b.WriteString(`  "optimizations": [`)
	for i, o := range optimizations {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    {\n")
		b.WriteString(`      "name": `)
		writeJSONString(&b, o.Name)
		b.WriteString(",\n")
		b.WriteString(`      "rationale": `)
		writeJSONString(&b, o.Rationale)
		b.WriteString(",\n")
		b.WriteString(`      "mechanism": `)
		writeJSONString(&b, o.Mechanism)
		b.WriteString("\n    }")
	}
	b.WriteString("\n  ]\n}\n")
	return b.String()
}

// CostMarkdown renders the cost-budget document as Markdown.
func CostMarkdown() string {
	var b strings.Builder

	b.WriteString("# CI Cost Budget\n\n")
	b.WriteString("> **Status:** Accepted\n")
	b.WriteString("> **Owner:** Release Engineering\n")
	b.WriteString("> **Related:** [ADR-053](../decisions/ADR-053-ci-cost-budget.md), [CI Tiers](ci-tiers.md), [CI Cache Strategy](ci-cache-strategy.md)\n\n")
	b.WriteString("This document describes the repository's CI cost budget and the ")
	b.WriteString("techniques the repository uses to stay within it. It is rendered from ")
	b.WriteString("`internal/ci/cost.go`; a golden-file test asserts the two agree.\n\n")

	// The budget.
	b.WriteString("## The budget\n\n")
	b.WriteString("The repository's monthly budget depends on its visibility:\n\n")
	b.WriteString("- **Public repository.** ")
	b.WriteString(itoa(costBudget.PublicMonthlyMinutes))
	b.WriteString(" minutes per month. GitHub Actions' free tier provides 2,000 minutes for public repositories.\n")
	b.WriteString("- **Private repository.** ")
	b.WriteString(itoa(costBudget.PrivateMonthlyMinutes))
	b.WriteString(" minutes per month. The free tier provides 2,000; the target is set higher to accommodate ")
	b.WriteString("a paid plan.\n\n")
	b.WriteString("The alert threshold is **")
	b.WriteString(fmt.Sprintf("%.0f%%", costBudget.AlertThreshold*100))
	b.WriteString("** of the budget. When the threshold is reached, an alert fires; the ")
	b.WriteString("Release Engineering owner reviews the month's usage and identifies the driver.\n\n")

	b.WriteString("## Cost drivers\n\n")
	b.WriteString("The cost of a CI run is the sum, over every job, of the job's wall-clock minutes ")
	b.WriteString("multiplied by its OS runner's multiplier. The drivers are:\n\n")
	b.WriteString("### OS runner multipliers\n\n")
	b.WriteString("| OS | Multiplier |\n")
	b.WriteString("|---|---|\n")
	for _, m := range AllOSRunnerMultipliers() {
		b.WriteString("| ")
		b.WriteString(string(m))
		b.WriteString(" | ")
		b.WriteString(itoa(m.Multiplier()))
		b.WriteString("× |\n")
	}
	b.WriteString("\nA minute of wall-clock on a Linux runner costs 1 billing minute; on Windows, 2; on macOS, 10. ")
	b.WriteString("The multipliers are the single largest driver of cost; a job that runs on macOS for 5 minutes ")
	b.WriteString("costs as much as a job that runs on Linux for 50.\n\n")

	b.WriteString("### Matrix size\n\n")
	b.WriteString("A matrix job's cost is the sum of its cells' costs. Tier 2's matrix has six cells ")
	b.WriteString("(three OSes × two Go versions); its cost is dominated by the macOS cells' 10× multiplier. ")
	b.WriteString("A future change that reduced the matrix's OS coverage would reduce the cost, at the ")
	b.WriteString("expense of cross-platform confidence.\n\n")

	b.WriteString("### Cache effectiveness\n\n")
	b.WriteString("The cache strategy (see [ci-cache-strategy.md](ci-cache-strategy.md)) reduces each job's ")
	b.WriteString("wall-clock. A cold cache's first run downloads every module and compiles every object; ")
	b.WriteString("a warm cache reuses the previous run's artifacts. The cache's hit-rate targets (90% for ")
	b.WriteString("the module cache, 60% for the build cache) are the mechanism by which the reduction is ")
	b.WriteString("measured.\n\n")

	b.WriteString("### Concurrency control\n\n")
	b.WriteString("The concurrency policy (see [concurrency-policy.md](concurrency-policy.md)) cancels ")
	b.WriteString("superseded PR runs. A contributor who pushes five times in an hour pays for one run, ")
	b.WriteString("not five: the first four are cancelled when the fifth supersedes them.\n\n")

	// The per-tier estimates.
	b.WriteString("## Per-tier estimates\n\n")
	b.WriteString("The table below estimates each tier's cost. The values are estimates, not measurements; ")
	b.WriteString("the monthly review replaces them with actuals.\n\n")
	b.WriteString("| Tier | Runner | Jobs | Per-job minutes | Runs on PR | Runs on main |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	for _, c := range costByTier {
		b.WriteString("| Tier ")
		b.WriteString(itoa(c.Tier.Number()))
		b.WriteString(" | ")
		b.WriteString(string(c.Runner))
		b.WriteString(" | ")
		b.WriteString(itoa(c.JobCount))
		b.WriteString(" | ")
		b.WriteString(itoa(c.PerJobMinutes))
		b.WriteString(" | ")
		if c.RunsOnPR {
			b.WriteString("yes")
		} else {
			b.WriteString("no")
		}
		b.WriteString(" | ")
		if c.RunsOnMain {
			b.WriteString("yes")
		} else {
			b.WriteString("no")
		}
		b.WriteString(" |\n")
	}
	b.WriteString("\n")

	// The optimization techniques.
	b.WriteString("## Optimization techniques\n\n")
	b.WriteString("The repository uses the following techniques to reduce its CI cost:\n\n")
	for i, o := range optimizations {
		b.WriteString(itoa(i + 1))
		b.WriteString(". **")
		b.WriteString(o.Name)
		b.WriteString(".** ")
		b.WriteString(o.Rationale)
		b.WriteString(" See `")
		b.WriteString(o.Mechanism)
		b.WriteString("`.\n")
	}
	b.WriteString("\n")

	// The dashboard.
	b.WriteString("## Cost dashboard\n\n")
	b.WriteString("The monthly review reports three metrics:\n\n")
	b.WriteString("- **Monthly minutes used.** The sum, over the month's runs, of each job's wall-clock minutes ")
	b.WriteString("  multiplied by its OS runner's multiplier.\n")
	b.WriteString("- **Cost per PR.** The month's total minutes divided by the number of PRs merged in the month. ")
	b.WriteString("  Target: < ")
	b.WriteString(itoa(costBudget.TargetMinutesPerPR))
	b.WriteString(" minutes per PR.\n")
	b.WriteString("- **Cost per main push.** The month's total minutes divided by the number of main pushes in the month. ")
	b.WriteString("  Target: < ")
	b.WriteString(itoa(costBudget.TargetMinutesPerMainPush))
	b.WriteString(" minutes per main push.\n\n")
	b.WriteString("The metrics' source is GitHub's billing API (`gh api /orgs/<org>/settings/billing/actions`). ")
	b.WriteString("The review is documented in the Release Engineering calendar; a future change that wired ")
	b.WriteString("the review into a scheduled workflow would add the workflow, not change the budget.\n\n")

	return b.String()
}

// costError builds a *forgeerr.Error for a cost-model invariant
// violation.
func costError(what, detail string) error {
	e := forgeerr.New(
		forgeerr.CategoryInternal,
		forgeerr.CodeInternalUnexpected,
		fmt.Sprintf("ci: cost model invariant violated: %s", what),
	)
	e = e.WithContext("detail", detail)
	return e
}

// init validates the cost model at package start.
func init() {
	if err := ValidateCost(); err != nil {
		panic(fmt.Sprintf("ci: cost model validation failed: %v", err))
	}
}

// Compile-time assertions.
var (
	_ func() []OSRunnerMultiplier = AllOSRunnerMultipliers
	_ func() CostBudget           = CostBudgetOf
	_ func() []TierCost           = AllTierCosts
	_ func() []Optimization       = AllOptimizations
	_ func() error                = ValidateCost
	_ func() string               = CostGolden
	_ func() string               = CostMarkdown
)
