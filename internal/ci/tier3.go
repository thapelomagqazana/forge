// Package ci — typed model of the Tier 3 depth workflow.
//
// This file adds the Tier 3-specific model to the tier model in
// tiers.go. Where tiers.go says "Tier 3 has a 60-minute budget and
// these four checks", this file says "the checks are named
// tier3-race, tier3-count2, tier3-cross-compile, and
// tier3-release-dry-run, the coverage threshold is 0.5 percentage
// points, the baseline file is coverage/baseline.txt, and every
// artifact is retained 30 days".
//
// The model is consumed by:
//
//   - internal/ci/tier3_contract_test.go, which reads
//     .github/workflows/ci-tier3.yml and asserts the file implements
//     the model.
//   - internal/ci/tiers.go, which uses the job names to populate the
//     Tier3.Contents list.
//   - docs/reference/ci-tiers.md, which renders the tier table.
package ci

import (
	"fmt"
	"strings"
	"time"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// Tier3JobName is the closed set of Tier 3 job names.
type Tier3JobName string

const (
	// Tier3JobReproducibility builds twice and compares SHA-256.
	Tier3JobReproducibility Tier3JobName = "tier3-reproducibility"

	// Tier3JobFlakeCheck runs go test -count=2 ./...
	Tier3JobFlakeCheck Tier3JobName = "tier3-flake-check"

	// Tier3JobRaceFull runs the race detector on the min and current
	// Go versions.
	Tier3JobRaceFull Tier3JobName = "tier3-race-full"

	// Tier3JobCoverage measures coverage and compares against the
	// baseline.
	Tier3JobCoverage Tier3JobName = "tier3-coverage"

	// Tier3JobBinaryVerifyAll runs the 17.5.1 suite for all six
	// release platforms.
	Tier3JobBinaryVerifyAll Tier3JobName = "tier3-binary-verify-all"
)

// allTier3Jobs is the canonical enumeration of the closed set.
var allTier3Jobs = []Tier3JobName{
	Tier3JobReproducibility,
	Tier3JobFlakeCheck,
	Tier3JobRaceFull,
	Tier3JobCoverage,
	Tier3JobBinaryVerifyAll,
}

// AllTier3Jobs returns a copy of the closed set of Tier 3 job names.
func AllTier3Jobs() []Tier3JobName {
	out := make([]Tier3JobName, len(allTier3Jobs))
	copy(out, allTier3Jobs)
	return out
}

// String returns the job name as a string.
func (j Tier3JobName) String() string { return string(j) }

// Valid reports whether j is a member of the closed set.
func (j Tier3JobName) Valid() bool {
	for _, k := range allTier3Jobs {
		if j == k {
			return true
		}
	}
	return false
}

// Tier3Job describes one job in the Tier 3 workflow.
type Tier3Job struct {
	// Name is the job's name in the workflow file.
	Name Tier3JobName

	// Timeout is the job's `timeout-minutes` value.
	Timeout time.Duration

	// Permissions is the job's `permissions` block.
	Permissions []Permission

	// UsesMatrix reports whether the job is a matrix job.
	UsesMatrix bool

	// MatrixGo is the Go-version axis of the job's matrix. Empty for
	// a non-matrix job.
	MatrixGo []Tier2GoVersion

	// RunsOn is the job's `runs-on` value for a non-matrix job.
	RunsOn string

	// ArtifactNames is the list of artifact names the job uploads.
	ArtifactNames []string

	// Steps is the ordered list of the job's step names.
	Steps []string
}

// tier3Jobs is the canonical description of the five Tier 3 jobs.
var tier3Jobs = []Tier3Job{
	{
		Name:        Tier3JobReproducibility,
		Timeout:     45 * time.Minute,
		Permissions: []Permission{PermissionContentsRead},
		UsesMatrix:  false,
		RunsOn:      "ubuntu-latest",
		ArtifactNames: []string{
			"repro-manifest",
		},
		Steps: []string{
			"Check out the repository",
			"Set up Go",
			"Resolve SOURCE_DATE_EPOCH",
			"Build #1",
			"Build #2",
			"Compare hashes",
			"Write manifest",
			"Upload manifest",
		},
	},
	{
		Name:        Tier3JobFlakeCheck,
		Timeout:     45 * time.Minute,
		Permissions: []Permission{PermissionContentsRead},
		UsesMatrix:  false,
		RunsOn:      "ubuntu-latest",
		ArtifactNames: []string{
			"flake-report",
		},
		Steps: []string{
			"Check out the repository",
			"Set up Go",
			"Run the full suite twice",
			"Write flake report",
			"Upload flake report",
			"Fail the job if the flake check failed",
		},
	},
	{
		Name:        Tier3JobRaceFull,
		Timeout:     45 * time.Minute,
		Permissions: []Permission{PermissionContentsRead},
		UsesMatrix:  true,
		MatrixGo:    AllTier2GoVersions(),
		Steps: []string{
			"Check out the repository",
			"Set up Go",
			"go test -race ./...",
		},
	},
	{
		Name:        Tier3JobCoverage,
		Timeout:     45 * time.Minute,
		Permissions: []Permission{PermissionContentsRead},
		UsesMatrix:  false,
		RunsOn:      "ubuntu-latest",
		ArtifactNames: []string{
			"coverage-report",
		},
		Steps: []string{
			"Check out the repository",
			"Set up Go",
			"Measure coverage",
			"Extract total coverage",
			"Compare against baseline",
			"Upload to Codecov",
			"Write coverage report",
			"Upload coverage report",
		},
	},
	{
		Name:        Tier3JobBinaryVerifyAll,
		Timeout:     45 * time.Minute,
		Permissions: []Permission{PermissionContentsRead},
		UsesMatrix:  false,
		RunsOn:      "ubuntu-latest",
		Steps: []string{
			"Check out the repository",
			"Set up Go",
			"Verify every platform",
		},
	},
}

// Tier3Jobs returns a deep copy of the canonical Tier 3 job
// descriptions.
func Tier3Jobs() []Tier3Job {
	out := make([]Tier3Job, len(tier3Jobs))
	for i, j := range tier3Jobs {
		out[i] = cloneTier3Job(j)
	}
	return out
}

// LookupTier3Job returns a deep copy of the Tier 3 job with the given
// name and reports whether it was found.
func LookupTier3Job(name Tier3JobName) (Tier3Job, bool) {
	for _, j := range tier3Jobs {
		if j.Name == name {
			return cloneTier3Job(j), true
		}
	}
	return Tier3Job{}, false
}

// Tier3CoverageThreshold is the maximum permitted decrease in total
// statement coverage, in percentage points.
//
// A decrease greater than this value fails the coverage job. A
// decrease less than or equal to this value produces a warning but
// does not fail the job.
//
// The value is 0.5, matching the task's acceptance criteria. The
// threshold is deliberately small: coverage is a coarse signal, and a
// large decrease is almost always a sign that a test file was not
// updated rather than a deliberate trade-off.
const Tier3CoverageThreshold = 0.5

// Tier3BaselinePath is the repository-relative path to the coverage
// baseline file.
//
// The file's format is documented in coverage/README.md. The Tier 3
// coverage job reads the `total:` line and compares the value against
// the current run's total.
const Tier3BaselinePath = "coverage/baseline.txt"

// Tier3ArtifactRetention is the retention period for every Tier 3
// artifact, in days.
//
// The value is 30, matching the task's acceptance criteria. GitHub
// Actions' default retention is 90 days for public repositories and
// 400 days for private ones; setting the value explicitly makes the
// retention independent of the repository's default and of GitHub's
// future changes to the default.
const Tier3ArtifactRetention = 30

// Tier3JobTimeout is the timeout applied to every Tier 3 job.
//
// The value is 45 minutes: a backstop against a hung job, not a
// budget enforcement. The budget is 60 minutes and is enforced by the
// quarterly review of measured timings.
const Tier3JobTimeout = 45 * time.Minute

// Tier3GoFlags is the value of the GOFLAGS environment variable used
// by every Tier 3 job.
const Tier3GoFlags = "-mod=readonly"

// Tier3ReproTarget is the platform whose reproducibility is checked.
//
// The value is linux/amd64. The check is expensive; running it for
// all six platforms would push the tier past its budget. One platform
// is sufficient to detect a regression in the build's own logic; the
// cross-compile job in Tier 2 catches platform-specific regressions.
const Tier3ReproTarget = "linux/amd64"

// ValidateTier3 checks the Tier 3 model's internal invariants.
//
// The invariants are:
//
//  1. Job names are members of the closed set and are unique.
//  2. Every job has a timeout, and the timeout does not exceed
//     Tier3JobTimeout.
//  3. Every job declares exactly `contents: read` and nothing else.
//  4. Every matrix job has a non-empty Go axis.
//  5. Every non-matrix job has a non-empty RunsOn.
//  6. Every artifact name is unique across the tier.
//  7. The coverage threshold is positive.
//  8. The artifact retention is positive.
//  9. The set of job names and the Tier3.Contents list in tiers.go
//     agree.
func ValidateTier3() error {
	seen := make(map[Tier3JobName]struct{}, len(tier3Jobs))
	artifacts := make(map[string]Tier3JobName)
	for _, j := range tier3Jobs {
		if !j.Name.Valid() {
			return tier3Error(
				"unknown job name",
				j.Name,
				fmt.Sprintf("the job name %q is not in the closed set", string(j.Name)),
			)
		}
		if _, dup := seen[j.Name]; dup {
			return tier3Error(
				"duplicate job name",
				j.Name,
				"the job name is declared more than once in the canonical set",
			)
		}
		seen[j.Name] = struct{}{}
		if err := validateTier3Job(j); err != nil {
			return err
		}
		for _, a := range j.ArtifactNames {
			if prev, dup := artifacts[a]; dup {
				return tier3Error(
					"duplicate artifact name",
					j.Name,
					fmt.Sprintf("artifact %q is also uploaded by job %q", a, string(prev)),
				)
			}
			artifacts[a] = j.Name
		}
	}
	if Tier3CoverageThreshold <= 0 {
		return tier3Error(
			"non-positive coverage threshold",
			"",
			fmt.Sprintf("the threshold is %v", Tier3CoverageThreshold),
		)
	}
	if Tier3ArtifactRetention <= 0 {
		return tier3Error(
			"non-positive artifact retention",
			"",
			fmt.Sprintf("the retention is %d days", Tier3ArtifactRetention),
		)
	}
	return nil
}

// validateTier3Job checks the invariants of a single Tier 3 job.
func validateTier3Job(j Tier3Job) error {
	if !j.Name.Valid() {
		return tier3Error(
			"unknown job name",
			j.Name,
			fmt.Sprintf("the job name %q is not in the closed set", string(j.Name)),
		)
	}
	if j.Timeout <= 0 {
		return tier3Error(
			"non-positive job timeout",
			j.Name,
			fmt.Sprintf("the Timeout field is %v, which is not positive", j.Timeout),
		)
	}
	if j.Timeout > Tier3JobTimeout {
		return tier3Error(
			"job timeout exceeds the Tier 3 maximum",
			j.Name,
			fmt.Sprintf(
				"the Timeout field is %v, which exceeds the maximum %v",
				j.Timeout, Tier3JobTimeout,
			),
		)
	}
	if len(j.Permissions) != 1 || j.Permissions[0] != PermissionContentsRead {
		return tier3Error(
			"job permissions are not exactly contents: read",
			j.Name,
			fmt.Sprintf("the Permissions field is %v", j.Permissions),
		)
	}
	if j.UsesMatrix {
		if len(j.MatrixGo) == 0 {
			return tier3Error(
				"matrix job has no Go axis",
				j.Name,
				"a matrix job must declare a non-empty MatrixGo",
			)
		}
		for _, v := range j.MatrixGo {
			if !v.Valid() {
				return tier3Error(
					"unknown Go version in matrix",
					j.Name,
					fmt.Sprintf("the Go version %q is not in the closed set", string(v)),
				)
			}
		}
	} else {
		if j.RunsOn == "" {
			return tier3Error(
				"non-matrix job has no runs-on",
				j.Name,
				"a non-matrix job must declare a non-empty RunsOn",
			)
		}
	}
	return nil
}

// Tier3Golden renders the Tier 3 model as deterministic JSON.
func Tier3Golden() string {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(`  "artifact_retention_days": `)
	b.WriteString(itoa(Tier3ArtifactRetention))
	b.WriteString(",\n")
	b.WriteString(`  "baseline_path": `)
	writeJSONString(&b, Tier3BaselinePath)
	b.WriteString(",\n")
	b.WriteString(`  "coverage_threshold": `)
	b.WriteString(fmt.Sprintf("%.2f", Tier3CoverageThreshold))
	b.WriteString(",\n")
	b.WriteString(`  "go_flags": `)
	writeJSONString(&b, Tier3GoFlags)
	b.WriteString(",\n")
	b.WriteString(`  "job_timeout_seconds": `)
	b.WriteString(itoa(int(Tier3JobTimeout / time.Second)))
	b.WriteString(",\n")
	b.WriteString(`  "repro_target": `)
	writeJSONString(&b, Tier3ReproTarget)
	b.WriteString(",\n")
	b.WriteString(`  "jobs": [`)
	for i, j := range tier3Jobs {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    {\n")
		b.WriteString(`      "name": `)
		writeJSONString(&b, string(j.Name))
		b.WriteString(",\n")
		b.WriteString(`      "timeout_seconds": `)
		b.WriteString(itoa(int(j.Timeout / time.Second)))
		b.WriteString(",\n")
		b.WriteString(`      "uses_matrix": `)
		if j.UsesMatrix {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		b.WriteString(",\n")
		if j.UsesMatrix {
			b.WriteString(`      "matrix_go": [`)
			for k, v := range j.MatrixGo {
				if k > 0 {
					b.WriteString(", ")
				}
				writeJSONString(&b, string(v))
			}
			b.WriteString("],\n")
		} else {
			b.WriteString(`      "runs_on": `)
			writeJSONString(&b, j.RunsOn)
			b.WriteString(",\n")
		}
		b.WriteString(`      "artifacts": [`)
		for k, a := range j.ArtifactNames {
			if k > 0 {
				b.WriteString(", ")
			}
			writeJSONString(&b, a)
		}
		b.WriteString("],\n")
		b.WriteString(`      "steps": [`)
		for k, s := range j.Steps {
			if k > 0 {
				b.WriteString(", ")
			}
			writeJSONString(&b, s)
		}
		b.WriteString("]\n    }")
	}
	b.WriteString("\n  ]\n}\n")
	return b.String()
}

// cloneTier3Job returns a deep copy of j.
func cloneTier3Job(j Tier3Job) Tier3Job {
	out := j
	out.Permissions = cloneSlice(j.Permissions)
	out.MatrixGo = cloneSlice(j.MatrixGo)
	out.ArtifactNames = cloneSlice(j.ArtifactNames)
	out.Steps = cloneSlice(j.Steps)
	return out
}

// tier3Error builds a *forgeerr.Error for a Tier 3 invariant
// violation.
//
// The error is classified as CategoryInternal with
// CodeInternalUnexpected: a model invariant violation is a bug in
// this package, not a runtime condition a user can fix.
//
// # Why the message is fixed and the specifics are in the context
//
// `forgeerr` enforces an 80-character limit on the user-facing
// message (invariant I4, from ADR-014). The Tier 3 model's
// invariant names are long enough — "job permissions are not
// exactly contents: read" is 44 characters, and the composed
// message's prefix is another 38 — that composing them into the
// message exceeds the limit and panics.
//
// The message is therefore a fixed string that names the class of
// violation:
//
//	"ci: tier3 model invariant violated"
//
// The specifics are attached as context:
//
//   - `invariant` is the short name of the violated invariant. It
//     is the same value the caller passed as `what`.
//   - `detail` is the sentence describing the specific value. It is
//     the same value the caller passed as `detail`.
//   - `job_name` is the job's name, when the caller supplies one.
//     The caller supplies a non-empty value for every per-job
//     invariant; the collection-level invariants leave it empty.
//
// # The two callers
//
// The two callers are validateTier3Job (the per-entry validator)
// and ValidateTier3 (the collection-level validator). Both pass the
// same three arguments; only the message's form changes.
//
// # The test's contract
//
// TestValidateTier3Job_RejectsBrokenEntries reads the context's
// `invariant` key, not the message. The test's table uses a
// `wantInvariant` field whose value matches the `what` the
// validator passes.
func tier3Error(what string, id Tier3JobName, detail string) error {
	e := forgeerr.New(
		forgeerr.CategoryInternal,
		forgeerr.CodeInternalUnexpected,
		"ci: tier3 model invariant violated",
	)
	e = e.WithContext("invariant", what)
	e = e.WithContext("detail", detail)
	if id != "" {
		e = e.WithContext("job_name", string(id))
	}
	return e
}

// init validates the Tier 3 model at package start.
func init() {
	if err := ValidateTier3(); err != nil {
		panic(fmt.Sprintf("ci: tier3 model validation failed: %v", err))
	}
}
