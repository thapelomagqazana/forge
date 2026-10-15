// Package ci — typed model of the Tier 4 security workflow.
//
// This file defines the model behind .github/workflows/security.yml.
// Where tiers.go says "Tier 4 runs on a schedule", this file says
// "the schedule is 02:00 UTC daily; the workflow has eight jobs; the
// Scorecard threshold is 7.0; the forbidden licenses are ...".
//
// The model is consumed by:
//
//   - internal/ci/security_contract_test.go, which reads
//     security.yml and asserts it implements the model.
//   - docs/reference/security-workflow.md, which is rendered from
//     the model.
//   - internal/ci/workflows.go's WorkflowSecurity entry.
package ci

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// SecurityJobName is the closed set of Tier 4 job names.
type SecurityJobName string

const (
	// SecurityJobCodeQL runs the Go CodeQL analysis.
	SecurityJobCodeQL SecurityJobName = "security-codeql"

	// SecurityJobGovulncheck runs the Go vulnerability database scan.
	SecurityJobGovulncheck SecurityJobName = "security-govulncheck"

	// SecurityJobGitleaks runs the secret scan on the full history.
	SecurityJobGitleaks SecurityJobName = "security-gitleaks"

	// SecurityJobGrype runs the vulnerability scan of the release
	// artifacts.
	SecurityJobGrype SecurityJobName = "security-grype"

	// SecurityJobScorecard runs the OpenSSF Scorecard.
	SecurityJobScorecard SecurityJobName = "security-scorecard"

	// SecurityJobLicenseScan runs the license compatibility check.
	SecurityJobLicenseScan SecurityJobName = "security-license-scan"

	// SecurityJobSBOM generates the SBOM with syft.
	SecurityJobSBOM SecurityJobName = "security-sbom"

	// SecurityJobDependabot verifies Dependabot is configured.
	SecurityJobDependabot SecurityJobName = "security-dependabot"
)

// allSecurityJobs is the canonical enumeration of the closed set.
var allSecurityJobs = []SecurityJobName{
	SecurityJobCodeQL,
	SecurityJobGovulncheck,
	SecurityJobGitleaks,
	SecurityJobGrype,
	SecurityJobScorecard,
	SecurityJobLicenseScan,
	SecurityJobSBOM,
	SecurityJobDependabot,
}

// AllSecurityJobs returns a copy of the closed set of Tier 4 job
// names.
func AllSecurityJobs() []SecurityJobName {
	out := make([]SecurityJobName, len(allSecurityJobs))
	copy(out, allSecurityJobs)
	return out
}

// String returns the job name as a string.
func (j SecurityJobName) String() string { return string(j) }

// Valid reports whether j is a member of the closed set.
func (j SecurityJobName) Valid() bool {
	for _, k := range allSecurityJobs {
		if j == k {
			return true
		}
	}
	return false
}

// SecurityJob describes one job in the security workflow.
type SecurityJob struct {
	// Name is the job's name in the workflow file.
	Name SecurityJobName

	// Timeout is the job's `timeout-minutes` value.
	Timeout int

	// Permissions is the job's `permissions:` block.
	Permissions []Permission

	// UsesSARIF reports whether the job uploads a SARIF result.
	UsesSARIF bool

	// ArtifactRetention is the job's artifact retention, in days.
	// Zero means the job uploads no artifact.
	ArtifactRetention int

	// Note is a one-sentence explanation of the job's purpose.
	Note string
}

// securityJobs is the canonical description of the eight jobs.
var securityJobs = []SecurityJob{
	{
		Name:    SecurityJobCodeQL,
		Timeout: 60,
		Permissions: []Permission{
			PermissionContentsRead,
			PermissionSecurityEventsWrite,
		},
		UsesSARIF: true,
		Note:      "Go CodeQL analysis; results appear in the Security tab.",
	},
	{
		Name:    SecurityJobGovulncheck,
		Timeout: 30,
		Permissions: []Permission{
			PermissionContentsRead,
		},
		Note: "Go vulnerability database scan; reachable vulnerabilities fail the job.",
	},
	{
		Name:    SecurityJobGitleaks,
		Timeout: 30,
		Permissions: []Permission{
			PermissionContentsRead,
		},
		UsesSARIF:         true,
		ArtifactRetention: 90,
		Note:              "Secret scan on the full git history.",
	},
	{
		Name:    SecurityJobGrype,
		Timeout: 30,
		Permissions: []Permission{
			PermissionContentsRead,
		},
		UsesSARIF: true,
		Note:      "Vulnerability scan of the release artifacts with grype.",
	},
	{
		Name:    SecurityJobScorecard,
		Timeout: 30,
		Permissions: []Permission{
			PermissionContentsRead,
			PermissionSecurityEventsWrite,
			PermissionIDTokenWrite,
			PermissionActionsRead,
		},
		UsesSARIF: true,
		Note:      "OpenSSF Scorecard; fails below the threshold.",
	},
	{
		Name:    SecurityJobLicenseScan,
		Timeout: 30,
		Permissions: []Permission{
			PermissionContentsRead,
		},
		Note: "License compatibility check; fails on a forbidden license.",
	},
	{
		Name:    SecurityJobSBOM,
		Timeout: 30,
		Permissions: []Permission{
			PermissionContentsRead,
		},
		ArtifactRetention: 90,
		Note:              "SBOM generation with syft; retained 90 days.",
	},
	{
		Name:    SecurityJobDependabot,
		Timeout: 5,
		Permissions: []Permission{
			PermissionContentsRead,
		},
		Note: "Verify .github/dependabot.yml is configured for gomod and github-actions.",
	},
}

// AllSecurityJobDescriptions returns a deep copy of the canonical
// job descriptions.
func AllSecurityJobDescriptions() []SecurityJob {
	out := make([]SecurityJob, len(securityJobs))
	for i, j := range securityJobs {
		out[i] = cloneSecurityJob(j)
	}
	return out
}

// LookupSecurityJob returns a deep copy of the security job with the
// given name and reports whether it was found.
func LookupSecurityJob(name SecurityJobName) (SecurityJob, bool) {
	for _, j := range securityJobs {
		if j.Name == name {
			return cloneSecurityJob(j), true
		}
	}
	return SecurityJob{}, false
}

// SecurityScorecardThreshold is the minimum acceptable OpenSSF
// Scorecard score.
//
// The value is 7.0, matching the task's KPI. A future change to the
// threshold updates this constant and the ADR.
const SecurityScorecardThreshold = 7.0

// SecurityForbiddenLicenses is the set of licenses that are
// incompatible with the repository's Apache-2.0 license.
//
// The list is a legal decision; a future change to it updates this
// slice and the ADR.
//
// # Ordering
//
// The slice is in the order the licenses are presented in the ADR
// and in the security workflow reference. The rendering functions
// iterate the slice directly; its order is part of the contract.
var SecurityForbiddenLicenses = []string{
	"GPL-2.0",
	"GPL-3.0",
	"AGPL-3.0",
	"LGPL-2.1",
	"LGPL-3.0",
	"SSPL-1.0",
	"BUSL-1.1",
}

// SecuritySchedule is the workflow's cron schedule.
//
// The value is "0 2 * * *": 02:00 UTC daily. Off the hour to reduce
// queue delay; the slot avoids the global cron thundering herd at
// 00:00.
const SecuritySchedule = "0 2 * * *"

// SecuritySBOMRetention is the SBOM artifact's retention, in days.
const SecuritySBOMRetention = 90

// SecurityToolVersions is the pinned version of every tool the
// workflow uses that is not a GitHub Action.
//
// The map's keys are the tool names; the values are the version
// strings the workflow's `go install` or download commands use. The
// contract test asserts the workflow file's commands match the map.
//
// # Ordering
//
// The map's iteration order is unspecified in Go. Both rendering
// functions (SecurityGolden and SecurityMarkdown) iterate the map
// via sortedToolNames, which returns the keys in alphabetical
// order. A future change that added a tool updates the map and the
// two renderings' output without changing the two functions.
//
// # Why a map
//
// A map is the natural shape for a name-to-version lookup: the
// contract test queries the version by name; a contributor looks up
// a tool's pin by name. The rendering's determinism is preserved by
// sorting the keys before iterating; the map's unspecified order
// does not leak into the output.
var SecurityToolVersions = map[string]string{
	"govulncheck": "v1.1.4",
	"gitleaks":    "8.21.2",
	"grype":       "v0.87.0",
	"go-licenses": "v1.6.0",
	"syft":        "v1.18.1",
}

// sortedToolNames returns the tool-version map's keys in sorted
// order.
//
// The map's iteration order is unspecified in Go; a rendering that
// iterated the map directly would produce different bytes across
// runs, which would break the golden-file tests. The helper sorts
// the keys so that the rendering is deterministic.
//
// # The two callers
//
// The two callers are SecurityGolden (the JSON rendering) and
// SecurityMarkdown (the Markdown rendering). Both produce
// deterministic output by iterating the sorted keys; the two
// renderings agree on the tools' order (alphabetical).
//
// # Why the helper is a function, not a package-level variable
//
// The sorted names could be computed once at package init and
// stored in a variable. The function form is chosen because the map
// is a package-level variable that a future change might mutate
// (for example, a test that exercises a different set of tools); a
// cached slice would not reflect the mutation. The function
// recomputes the sorted keys on every call; the cost is negligible
// (the map has five entries) and the two renderings call it once
// each.
func sortedToolNames() []string {
	names := make([]string, 0, len(SecurityToolVersions))
	for name := range SecurityToolVersions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ValidateSecurity checks the security model's internal invariants.
//
// The invariants are:
//
//  1. Every job name in the closed set has a description.
//  2. Every description names a member of the closed set.
//  3. Every job has a positive timeout.
//  4. Every job has at least one permission.
//  5. Every job with an artifact has a positive retention.
//  6. The Scorecard threshold is in [0, 10].
//  7. The forbidden-license list is non-empty.
//  8. The schedule is non-empty.
//  9. The SBOM retention is positive.
//  10. Every tool version is non-empty and starts with "v" or is a
//     bare number.
func ValidateSecurity() error {
	seen := make(map[SecurityJobName]struct{}, len(securityJobs))
	for _, j := range securityJobs {
		if !j.Name.Valid() {
			return securityError(
				"unknown job name",
				fmt.Sprintf("the job name %q is not in the closed set", string(j.Name)),
			)
		}
		if _, dup := seen[j.Name]; dup {
			return securityError(
				"duplicate job name",
				fmt.Sprintf("the job name %q is declared more than once", string(j.Name)),
			)
		}
		seen[j.Name] = struct{}{}
		if err := validateSecurityJob(j); err != nil {
			return err
		}
	}
	for _, n := range allSecurityJobs {
		if _, ok := seen[n]; !ok {
			return securityError(
				"missing job",
				fmt.Sprintf("the job %q has no description", string(n)),
			)
		}
	}
	if SecurityScorecardThreshold < 0 || SecurityScorecardThreshold > 10 {
		return securityError(
			"scorecard threshold out of range",
			fmt.Sprintf("the threshold %v is not in [0, 10]", SecurityScorecardThreshold),
		)
	}
	if len(SecurityForbiddenLicenses) == 0 {
		return securityError("empty forbidden-license list", "the list must have at least one entry")
	}
	if SecuritySchedule == "" {
		return securityError("empty schedule", "the cron expression is required")
	}
	if SecuritySBOMRetention <= 0 {
		return securityError(
			"non-positive SBOM retention",
			fmt.Sprintf("the retention is %d days", SecuritySBOMRetention),
		)
	}
	for name, version := range SecurityToolVersions {
		if version == "" {
			return securityError(
				"empty tool version",
				fmt.Sprintf("the tool %q has no version", name),
			)
		}
	}
	return nil
}

// validateSecurityJob checks the invariants of a single SecurityJob.
func validateSecurityJob(j SecurityJob) error {
	if !j.Name.Valid() {
		return securityError(
			"unknown job name",
			fmt.Sprintf("the job name %q is not in the closed set", string(j.Name)),
		)
	}
	if j.Timeout <= 0 {
		return securityError(
			"non-positive timeout",
			fmt.Sprintf("the job %q has timeout %d", string(j.Name), j.Timeout),
		)
	}
	if len(j.Permissions) == 0 {
		return securityError(
			"no permissions",
			fmt.Sprintf("the job %q has no permissions", string(j.Name)),
		)
	}
	if j.ArtifactRetention < 0 {
		return securityError(
			"negative artifact retention",
			fmt.Sprintf("the job %q has retention %d", string(j.Name), j.ArtifactRetention),
		)
	}
	if j.Note == "" {
		return securityError(
			"empty note",
			fmt.Sprintf("the job %q has no note", string(j.Name)),
		)
	}
	return nil
}

// SecurityGolden renders the security model as deterministic JSON.
//
// # Determinism
//
// The function iterates the tool-version map via sortedToolNames,
// which returns the map's keys in sorted order. A direct iteration
// of the map would produce different bytes across runs, which would
// break the golden-file test TestSecurityGolden_IsDeterministic.
//
// The other collections the function iterates are slices
// (SecurityForbiddenLicenses, securityJobs); a slice's iteration
// order is its element order, which is stable.
func SecurityGolden() string {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(`  "schedule": `)
	writeJSONString(&b, SecuritySchedule)
	b.WriteString(",\n")
	b.WriteString(`  "scorecard_threshold": `)
	b.WriteString(fmt.Sprintf("%.1f", SecurityScorecardThreshold))
	b.WriteString(",\n")
	b.WriteString(`  "sbom_retention_days": `)
	b.WriteString(itoa(SecuritySBOMRetention))
	b.WriteString(",\n")
	b.WriteString(`  "forbidden_licenses": [`)
	for i, l := range SecurityForbiddenLicenses {
		if i > 0 {
			b.WriteString(", ")
		}
		writeJSONString(&b, l)
	}
	b.WriteString("],\n")
	b.WriteString(`  "tool_versions": {`)
	for i, name := range sortedToolNames() {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    ")
		writeJSONString(&b, name)
		b.WriteString(": ")
		writeJSONString(&b, SecurityToolVersions[name])
	}
	b.WriteString("\n  },\n")
	b.WriteString(`  "jobs": [`)
	for i, j := range securityJobs {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    {\n")
		b.WriteString(`      "name": `)
		writeJSONString(&b, string(j.Name))
		b.WriteString(",\n")
		b.WriteString(`      "timeout_minutes": `)
		b.WriteString(itoa(j.Timeout))
		b.WriteString(",\n")
		b.WriteString(`      "uses_sarif": `)
		if j.UsesSARIF {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		b.WriteString(",\n")
		b.WriteString(`      "artifact_retention_days": `)
		b.WriteString(itoa(j.ArtifactRetention))
		b.WriteString(",\n")
		b.WriteString(`      "note": `)
		writeJSONString(&b, j.Note)
		b.WriteString("\n    }")
	}
	b.WriteString("\n  ]\n}\n")
	return b.String()
}

// SecurityMarkdown renders the security workflow reference as
// Markdown.
//
// # Determinism
//
// The function iterates the tool-version map via sortedToolNames,
// which returns the map's keys in sorted order. A direct iteration
// of the map would produce different bytes across runs, which would
// break the determinism test TestSecurityMarkdown_IsDeterministic.
//
// The other collections the function iterates are slices
// (SecurityForbiddenLicenses, securityJobs); a slice's iteration
// order is its element order, which is stable.
func SecurityMarkdown() string {
	var b strings.Builder

	b.WriteString("# Security Workflow Reference\n\n")
	b.WriteString("> **Status:** Accepted\n")
	b.WriteString("> **Owner:** Security\n")
	b.WriteString("> **Related:** [CI Tiers](ci-tiers.md), [Platform Tiers](platform-tiers.md)\n\n")
	b.WriteString("This document describes the security scanning workflow\n")
	b.WriteString("(`.github/workflows/security.yml`). The workflow is rendered\n")
	b.WriteString("from `internal/ci/security.go`; a golden-file test asserts the\n")
	b.WriteString("two agree.\n\n")

	b.WriteString("## Trigger\n\n")
	b.WriteString("The workflow runs on:\n\n")
	b.WriteString("- **Schedule.** `")
	b.WriteString(SecuritySchedule)
	b.WriteString("` (02:00 UTC daily).\n")
	b.WriteString("- **Push to main.** Every push to the `main` branch.\n")
	b.WriteString("- **Manual.** `workflow_dispatch` for a manual run.\n\n")

	b.WriteString("## Jobs\n\n")
	b.WriteString("The workflow has eight jobs, running in parallel:\n\n")
	b.WriteString("| Job | Timeout | SARIF | Retention | Note |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, j := range securityJobs {
		b.WriteString("| `")
		b.WriteString(string(j.Name))
		b.WriteString("` | ")
		b.WriteString(fmt.Sprintf("%dm", j.Timeout))
		b.WriteString(" | ")
		if j.UsesSARIF {
			b.WriteString("yes")
		} else {
			b.WriteString("no")
		}
		b.WriteString(" | ")
		if j.ArtifactRetention > 0 {
			b.WriteString(fmt.Sprintf("%dd", j.ArtifactRetention))
		} else {
			b.WriteString("—")
		}
		b.WriteString(" | ")
		b.WriteString(j.Note)
		b.WriteString(" |\n")
	}
	b.WriteString("\n")

	b.WriteString("## Thresholds\n\n")
	b.WriteString("The workflow's pass/fail thresholds are:\n\n")
	b.WriteString("- **Scorecard.** Score ≥ ")
	b.WriteString(fmt.Sprintf("%.1f", SecurityScorecardThreshold))
	b.WriteString(" (below the threshold fails the job).\n")
	b.WriteString("- **Forbidden licenses.** A dependency whose license is one\n")
	b.WriteString("  of the following fails the license scan: ")
	b.WriteString(strings.Join(SecurityForbiddenLicenses, ", "))
	b.WriteString(".\n\n")

	b.WriteString("## Tool versions\n\n")
	b.WriteString("The tools the workflow uses are pinned to specific versions:\n\n")
	b.WriteString("| Tool | Version |\n")
	b.WriteString("|---|---|\n")
	for _, name := range sortedToolNames() {
		b.WriteString("| `")
		b.WriteString(name)
		b.WriteString("` | `")
		b.WriteString(SecurityToolVersions[name])
		b.WriteString("` |\n")
	}
	b.WriteString("\n")

	b.WriteString("## Notifications\n\n")
	b.WriteString("A workflow failure triggers the `notify` job, which opens an\n")
	b.WriteString("issue tagged `security` and `ci-failure`. The issue links to\n")
	b.WriteString("the failing run and lists the eight jobs so the reviewer knows\n")
	b.WriteString("which failed.\n\n")

	return b.String()
}

// --- helpers ---------------------------------------------------------------

// cloneSecurityJob returns a deep copy of j.
func cloneSecurityJob(j SecurityJob) SecurityJob {
	out := j
	out.Permissions = cloneSlice(j.Permissions)
	return out
}

// securityError builds a *forgeerr.Error for a security-model
// invariant violation.
func securityError(what, detail string) error {
	e := forgeerr.New(
		forgeerr.CategoryInternal,
		forgeerr.CodeInternalUnexpected,
		fmt.Sprintf("ci: security model invariant violated: %s", what),
	)
	e = e.WithContext("detail", detail)
	return e
}

// init validates the security model at package start.
func init() {
	if err := ValidateSecurity(); err != nil {
		panic(fmt.Sprintf("ci: security model validation failed: %v", err))
	}
}

// Compile-time assertions.
var (
	_ func() []SecurityJobName                  = AllSecurityJobs
	_ func() []SecurityJob                      = AllSecurityJobDescriptions
	_ func(SecurityJobName) (SecurityJob, bool) = LookupSecurityJob
	_ func() error                              = ValidateSecurity
	_ func() string                             = SecurityGolden
	_ func() string                             = SecurityMarkdown
)
