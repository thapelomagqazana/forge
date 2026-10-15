// Package ci — typed model of the flake policy.
//
// This file defines the repository's policy for flaky tests. A flaky
// test is a test that passes and fails intermittently on the same
// input. The policy:
//
//   - A flaky test is a P0 bug. It is not a "test problem"; it is a
//     correctness problem in the test, the code under test, or the
//     test's environment.
//
//   - Quarantine within 24 hours. The test is marked with
//     `t.Skip("flaky: #<issue>")` and the issue tracks the fix.
//
//   - Fix within 1 week. The issue is a P0; the fix is scheduled.
//
//   - No automatic retries. A retry hides the flake; the CI must
//     fail so that the flake is visible.
//
//   - The flake rate target is < 0.5% per test.
//
//   - Detection: Tier 3's `-count=2` catches many flakes; the
//     `flake-report.txt` artifact records each run's findings; a
//     future aggregator computes the per-test rate over time.
//
// # Why the policy is modelled
//
// The policy is prose, but it has invariants: the quarantine
// mechanism has a specific form; the SLA has specific durations; the
// retry policy is a closed set of behaviours. The model records the
// invariants; the contract tests assert the policy document matches
// the model.
package ci

import (
	"fmt"
	"strings"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// FlakePriority is the closed set of priorities a flaky test can
// have. The set has one member: a flaky test is a P0.
type FlakePriority string

const (
	// FlakePriorityP0 is the priority of a flaky test. The priority
	// is a P0, not a P1 or P2: the flake is a correctness problem.
	FlakePriorityP0 FlakePriority = "P0"
)

// String returns the priority as a string.
func (p FlakePriority) String() string { return string(p) }

// FlakeSLA is the typed description of the flake policy's timing
// commitments.
type FlakeSLA struct {
	// QuarantineWithin is the maximum time between the flake's
	// detection and its quarantine.
	QuarantineWithin string

	// FixWithin is the maximum time between the quarantine and the
	// fix.
	FixWithin string
}

// The SLA.
var flakeSLA = FlakeSLA{
	QuarantineWithin: "24h",
	FixWithin:        "1w",
}

// FlakeSLAOf returns the SLA.
func FlakeSLAOf() FlakeSLA {
	return flakeSLA
}

// FlakeQuarantineMarker is the marker a quarantined test carries in
// its `t.Skip` call.
//
// The form is `t.Skip("flaky: #<issue>")`. The `#<issue>` is the
// number of the issue that tracks the fix. The marker is the
// mechanism by which a reader of the test file knows the test is
// quarantined and by which the flake aggregator finds the quarantined
// tests.
const FlakeQuarantineMarker = "flaky:"

// FlakeQuarantineFormat is the Go format string a quarantined test
// uses.
//
// The format has one placeholder: the issue number. A future change
// that wanted a second placeholder (for example, a timestamp) would
// update this constant and the policy document.
const FlakeQuarantineFormat = `t.Skip("flaky: #%d")`

// FlakeRateTarget is the target flake rate per test.
//
// The value is 0.005 (0.5%). A test whose flake rate exceeds the
// target is a candidate for quarantine; a test whose rate exceeds
// the target after quarantine is a candidate for a deeper fix.
const FlakeRateTarget = 0.005

// FlakeRetryPolicy is the closed set of retry behaviours.
type FlakeRetryPolicy string

const (
	// FlakeRetryNone means no automatic retries. The CI's failure
	// is the flake's signal; a retry would hide it.
	FlakeRetryNone FlakeRetryPolicy = "none"

	// FlakeRetryAutomatic means the CI retries a failed test
	// automatically. This is forbidden by the policy.
	FlakeRetryAutomatic FlakeRetryPolicy = "automatic"
)

// FlakeRetryPolicyCurrent is the repository's retry policy.
const FlakeRetryPolicyCurrent = FlakeRetryNone

// FlakeDetection is the typed description of the detection mechanism.
type FlakeDetection struct {
	// Tier3Count is the value of the `-count` flag Tier 3 uses. The
	// value is 2: every test runs twice; a failure on the second
	// run is a flake.
	Tier3Count int

	// ArtifactName is the name of the artifact Tier 3 uploads. The
	// artifact's content is the flake report.
	ArtifactName string

	// AggregatorNote describes the future aggregator that computes
	// the per-test rate over time.
	AggregatorNote string
}

// The detection mechanism.
var flakeDetection = FlakeDetection{
	Tier3Count:   2,
	ArtifactName: "flake-report",
	AggregatorNote: "A future aggregator fetches the last N flake-report artifacts and computes " +
		"the per-test flake rate. The aggregator is out of scope for this task; the target " +
		"and the artifact's name are the interface it will consume.",
}

// FlakeDetectionOf returns the detection mechanism.
func FlakeDetectionOf() FlakeDetection {
	return flakeDetection
}

// ValidateFlake checks the flake model's internal invariants.
//
// The invariants are:
//
//  1. The priority is P0.
//  2. The SLA's two durations are non-empty.
//  3. The quarantine format contains the marker.
//  4. The rate target is in [0, 1].
//  5. The current retry policy is `none`.
//  6. The Tier 3 count is at least 2.
//  7. The artifact name is non-empty.
func ValidateFlake() error {
	if FlakePriorityP0 != "P0" {
		return flakeError(
			"priority is not P0",
			fmt.Sprintf("FlakePriorityP0 = %q, want %q", string(FlakePriorityP0), "P0"),
		)
	}
	if flakeSLA.QuarantineWithin == "" {
		return flakeError("empty quarantine SLA", "QuarantineWithin is empty")
	}
	if flakeSLA.FixWithin == "" {
		return flakeError("empty fix SLA", "FixWithin is empty")
	}
	if !strings.Contains(FlakeQuarantineFormat, FlakeQuarantineMarker) {
		return flakeError(
			"quarantine format missing the marker",
			fmt.Sprintf("FlakeQuarantineFormat = %q, want it to contain %q", FlakeQuarantineFormat, FlakeQuarantineMarker),
		)
	}
	if FlakeRateTarget < 0 || FlakeRateTarget > 1 {
		return flakeError(
			"rate target out of range",
			fmt.Sprintf("FlakeRateTarget = %v, want a value in [0, 1]", FlakeRateTarget),
		)
	}
	if FlakeRetryPolicyCurrent != FlakeRetryNone {
		return flakeError(
			"retry policy is not none",
			fmt.Sprintf("FlakeRetryPolicyCurrent = %q, want %q", string(FlakeRetryPolicyCurrent), string(FlakeRetryNone)),
		)
	}
	if flakeDetection.Tier3Count < 2 {
		return flakeError(
			"Tier 3 count is too small",
			fmt.Sprintf("flakeDetection.Tier3Count = %d, want >= 2", flakeDetection.Tier3Count),
		)
	}
	if flakeDetection.ArtifactName == "" {
		return flakeError("empty artifact name", "flakeDetection.ArtifactName is empty")
	}
	return nil
}

// FlakeGolden renders the flake model as deterministic JSON.
func FlakeGolden() string {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(`  "priority": `)
	writeJSONString(&b, string(FlakePriorityP0))
	b.WriteString(",\n")
	b.WriteString(`  "quarantine_within": `)
	writeJSONString(&b, flakeSLA.QuarantineWithin)
	b.WriteString(",\n")
	b.WriteString(`  "fix_within": `)
	writeJSONString(&b, flakeSLA.FixWithin)
	b.WriteString(",\n")
	b.WriteString(`  "quarantine_marker": `)
	writeJSONString(&b, FlakeQuarantineMarker)
	b.WriteString(",\n")
	b.WriteString(`  "quarantine_format": `)
	writeJSONString(&b, FlakeQuarantineFormat)
	b.WriteString(",\n")
	b.WriteString(`  "rate_target": `)
	b.WriteString(fmt.Sprintf("%.4f", FlakeRateTarget))
	b.WriteString(",\n")
	b.WriteString(`  "retry_policy": `)
	writeJSONString(&b, string(FlakeRetryPolicyCurrent))
	b.WriteString(",\n")
	b.WriteString(`  "tier3_count": `)
	b.WriteString(itoa(flakeDetection.Tier3Count))
	b.WriteString(",\n")
	b.WriteString(`  "artifact_name": `)
	writeJSONString(&b, flakeDetection.ArtifactName)
	b.WriteString(",\n")
	b.WriteString(`  "aggregator_note": `)
	writeJSONString(&b, flakeDetection.AggregatorNote)
	b.WriteString("\n}\n")
	return b.String()
}

// FlakeMarkdown renders the flake policy as Markdown.
func FlakeMarkdown() string {
	var b strings.Builder

	b.WriteString("# Flake Policy\n\n")
	b.WriteString("> **Status:** Accepted\n")
	b.WriteString("> **Owner:** Release Engineering\n")
	b.WriteString("> **Related:** [ADR-052](../decisions/ADR-052-pr-concurrency-and-flake-policy.md), [CI Tiers](ci-tiers.md)\n\n")
	b.WriteString("This document describes the repository's policy for flaky tests. ")
	b.WriteString("It is rendered from `internal/ci/flake.go`; a golden-file test asserts the two agree.\n\n")

	b.WriteString("## The policy in one paragraph\n\n")
	b.WriteString("A flaky test is a **P0 bug**. The test is quarantined within **")
	b.WriteString(flakeSLA.QuarantineWithin)
	b.WriteString("** of detection and fixed within **")
	b.WriteString(flakeSLA.FixWithin)
	b.WriteString("**. The quarantine mechanism is `")
	b.WriteString(FlakeQuarantineFormat)
	b.WriteString("` with a linked issue. **No automatic retries**: a retry hides the flake; ")
	b.WriteString("the CI must fail so that the flake is visible. The flake rate target is **")
	b.WriteString(fmt.Sprintf("%.1f%%", FlakeRateTarget*100))
	b.WriteString("** per test.\n\n")

	b.WriteString("## Quarantine mechanism\n\n")
	b.WriteString("A quarantined test carries a `t.Skip` call whose message starts with `")
	b.WriteString(FlakeQuarantineMarker)
	b.WriteString("` and includes the number of the issue that tracks the fix:\n\n")
	b.WriteString("```go\nfunc TestSomethingFlaky(t *testing.T) {\n")
	b.WriteString("    t.Skip(\"")
	b.WriteString(FlakeQuarantineMarker)
	b.WriteString(" #42\")\n")
	b.WriteString("    // ... the test body\n}\n```\n\n")
	b.WriteString("The `#42` is the issue number. The issue is tagged `flaky` and ")
	b.WriteString("`P0`; its title names the test and the failure mode.\n\n")

	b.WriteString("## Detection\n\n")
	b.WriteString("Tier 3's flake-check job runs `go test -count=")
	b.WriteString(itoa(flakeDetection.Tier3Count))
	b.WriteString(" ./...`. A test that passes on the first run and fails on the second is a flake. ")
	b.WriteString("The job uploads a `")
	b.WriteString(flakeDetection.ArtifactName)
	b.WriteString("` artifact with the failing tests' names.\n\n")
	b.WriteString(flakeDetection.AggregatorNote)
	b.WriteString("\n\n")

	b.WriteString("## Retry policy\n\n")
	b.WriteString("The repository's retry policy is **")
	b.WriteString(string(FlakeRetryPolicyCurrent))
	b.WriteString("**. The CI does not retry a failed test automatically. ")
	b.WriteString("A contributor who sees a failure re-runs the job manually if they believe the failure is spurious; ")
	b.WriteString("the manual re-run is a deliberate action, not a hidden retry.\n\n")

	b.WriteString("## Target\n\n")
	b.WriteString("The flake rate target is **")
	b.WriteString(fmt.Sprintf("%.1f%%", FlakeRateTarget*100))
	b.WriteString("** per test. A test whose rate exceeds the target is quarantined; ")
	b.WriteString("a test whose rate exceeds the target after a fix is a candidate for removal or redesign.\n\n")

	return b.String()
}

// flakeError builds a *forgeerr.Error for a flake-model invariant
// violation.
func flakeError(what, detail string) error {
	e := forgeerr.New(
		forgeerr.CategoryInternal,
		forgeerr.CodeInternalUnexpected,
		fmt.Sprintf("ci: flake model invariant violated: %s", what),
	)
	e = e.WithContext("detail", detail)
	return e
}

// init validates the flake model at package start.
func init() {
	if err := ValidateFlake(); err != nil {
		panic(fmt.Sprintf("ci: flake model validation failed: %v", err))
	}
}

// Compile-time assertions.
var (
	_ func() FlakeSLA       = FlakeSLAOf
	_ func() FlakeDetection = FlakeDetectionOf
	_ func() error          = ValidateFlake
	_ func() string         = FlakeGolden
	_ func() string         = FlakeMarkdown
)
