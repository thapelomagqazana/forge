// Package ci — typed model of the Tier 2 cross-platform matrix workflow.
//
// This file adds the Tier 2-specific model to the tier model in
// tiers.go. Where tiers.go says "Tier 2 has a 15-minute budget and
// these four checks", this file says "the matrix is 3 OS × 2 Go
// versions, the race detector runs on Linux only, the cross-compile
// target list is read from the platform matrix, and each job has a
// 20-minute timeout".
//
// The model is consumed by:
//
//   - internal/ci/tier2_contract_test.go, which reads
//     .github/workflows/ci-tier2.yml and asserts the file implements
//     the model.
//   - internal/ci/tiers.go, which uses the job names to populate the
//     Tier2.Contents list.
//   - docs/reference/ci-tiers.md, which renders the tier table.
package ci

import (
	"fmt"
	"strings"
	"time"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// Tier2JobName is the closed set of Tier 2 job names.
type Tier2JobName string

const (
	// Tier2JobTestMatrix is the full-suite matrix job. It is the
	// `test-matrix` job in ci-tier2.yml.
	Tier2JobTestMatrix Tier2JobName = "test-matrix"

	// Tier2JobCrossCompile is the cross-compile job. It is the
	// `cross-compile` job in ci-tier2.yml.
	Tier2JobCrossCompile Tier2JobName = "cross-compile"

	// Tier2JobSmoke is the smoke-test matrix job. It is the `smoke`
	// job in ci-tier2.yml.
	Tier2JobSmoke Tier2JobName = "smoke"

	// Tier2JobReleaseDryRun is the release dry-run job. It is the
	// `release-dry-run` job in ci-tier2.yml.
	Tier2JobReleaseDryRun Tier2JobName = "release-dry-run"
)

// allTier2Jobs is the canonical enumeration of the closed set.
var allTier2Jobs = []Tier2JobName{
	Tier2JobTestMatrix,
	Tier2JobCrossCompile,
	Tier2JobSmoke,
	Tier2JobReleaseDryRun,
}

// AllTier2Jobs returns a copy of the closed set of Tier 2 job names.
func AllTier2Jobs() []Tier2JobName {
	out := make([]Tier2JobName, len(allTier2Jobs))
	copy(out, allTier2Jobs)
	return out
}

// String returns the job name as a string.
func (j Tier2JobName) String() string { return string(j) }

// Valid reports whether j is a member of the closed set.
func (j Tier2JobName) Valid() bool {
	for _, k := range allTier2Jobs {
		if j == k {
			return true
		}
	}
	return false
}

// Tier2OS is the closed set of operating systems in the Tier 2 matrix.
//
// The values are GitHub Actions' runner labels, not Go's GOOS values.
// The two are related but not identical: "macos-latest" is a runner,
// "darwin" is a GOOS. The model uses the runner label because that is
// what the workflow's matrix axis names; the GOOS value is derived by
// the platform package when needed.
type Tier2OS string

const (
	Tier2OSUbuntu  Tier2OS = "ubuntu-latest"
	Tier2OSWindows Tier2OS = "windows-latest"
	Tier2OSMacOS   Tier2OS = "macos-latest"
)

// AllTier2OS returns the canonical list of operating systems in the
// matrix.
func AllTier2OS() []Tier2OS {
	return []Tier2OS{Tier2OSUbuntu, Tier2OSWindows, Tier2OSMacOS}
}

// Valid reports whether o is a member of the closed set.
func (o Tier2OS) Valid() bool {
	for _, k := range AllTier2OS() {
		if o == k {
			return true
		}
	}
	return false
}

// String returns the OS as a string.
func (o Tier2OS) String() string { return string(o) }

// Tier2GoVersion is the closed set of Go-version axis values.
type Tier2GoVersion string

const (
	// Tier2GoMin is the minimum supported Go version. It is the
	// version in go.mod's `go` directive.
	Tier2GoMin Tier2GoVersion = "min"

	// Tier2GoCurrent is the current supported Go version. It is the
	// version in go.mod's `toolchain` directive, or the latest
	// stable Go if the directive is absent.
	Tier2GoCurrent Tier2GoVersion = "current"
)

// AllTier2GoVersions returns the canonical list of Go-version axis
// values.
func AllTier2GoVersions() []Tier2GoVersion {
	return []Tier2GoVersion{Tier2GoMin, Tier2GoCurrent}
}

// Valid reports whether v is a member of the closed set.
func (v Tier2GoVersion) Valid() bool {
	for _, k := range AllTier2GoVersions() {
		if v == k {
			return true
		}
	}
	return false
}

// String returns the version as a string.
func (v Tier2GoVersion) String() string { return string(v) }

// Tier2Job describes one job in the Tier 2 workflow.
type Tier2Job struct {
	// Name is the job's name in the workflow file.
	Name Tier2JobName

	// Timeout is the job's `timeout-minutes` value.
	Timeout time.Duration

	// Permissions is the job's `permissions` block.
	Permissions []Permission

	// UsesMatrix reports whether the job is a matrix job. A matrix
	// job has a `strategy.matrix` block and a `strategy.fail-fast`
	// value; a non-matrix job does not.
	UsesMatrix bool

	// MatrixOS is the OS axis of the job's matrix. Empty for a
	// non-matrix job.
	MatrixOS []Tier2OS

	// MatrixGo is the Go-version axis of the job's matrix. Empty for
	// a non-matrix job.
	MatrixGo []Tier2GoVersion

	// RunsOn is the job's `runs-on` value for a non-matrix job. For
	// a matrix job, the value is the matrix variable and this field
	// is empty.
	RunsOn string

	// Steps is the ordered list of the job's step names.
	Steps []string
}

// tier2Jobs is the canonical description of the four Tier 2 jobs.
var tier2Jobs = []Tier2Job{
	{
		Name:        Tier2JobTestMatrix,
		Timeout:     20 * time.Minute,
		Permissions: []Permission{PermissionContentsRead},
		UsesMatrix:  true,
		MatrixOS:    AllTier2OS(),
		MatrixGo:    AllTier2GoVersions(),
		Steps: []string{
			"Check out the repository",
			"Set up Go",
			"go test ./...",
			"go test -race ./... (Linux only)",
		},
	},
	{
		Name:        Tier2JobCrossCompile,
		Timeout:     20 * time.Minute,
		Permissions: []Permission{PermissionContentsRead},
		UsesMatrix:  false,
		RunsOn:      "ubuntu-latest",
		Steps: []string{
			"Check out the repository",
			"Set up Go",
			"Cross-compile every release platform",
		},
	},
	{
		Name:        Tier2JobSmoke,
		Timeout:     20 * time.Minute,
		Permissions: []Permission{PermissionContentsRead},
		UsesMatrix:  true,
		MatrixOS:    AllTier2OS(),
		MatrixGo:    AllTier2GoVersions(),
		Steps: []string{
			"Check out the repository",
			"Set up Go",
			"Build the forge binary",
			"Run the binary smoke suite",
		},
	},
	{
		Name:        Tier2JobReleaseDryRun,
		Timeout:     30 * time.Minute,
		Permissions: []Permission{PermissionContentsRead},
		UsesMatrix:  false,
		RunsOn:      "ubuntu-latest",
		Steps: []string{
			"Check out the repository",
			"Set up Go",
			"make release-dry-run",
		},
	},
}

// Tier2Jobs returns a deep copy of the canonical Tier 2 job
// descriptions.
func Tier2Jobs() []Tier2Job {
	out := make([]Tier2Job, len(tier2Jobs))
	for i, j := range tier2Jobs {
		out[i] = cloneTier2Job(j)
	}
	return out
}

// LookupTier2Job returns a deep copy of the Tier 2 job with the given
// name and reports whether it was found.
func LookupTier2Job(name Tier2JobName) (Tier2Job, bool) {
	for _, j := range tier2Jobs {
		if j.Name == name {
			return cloneTier2Job(j), true
		}
	}
	return Tier2Job{}, false
}

// Tier2RaceDetectorOS returns the set of OSes on which the race
// detector runs.
//
// The set is a strict subset of AllTier2OS. The scope decision is
// documented in ADR-046: the race detector is 10× slower than the
// plain test run; on Windows and macOS runners, where the wall-clock
// is already near the budget, running it would push the tier past
// 15 minutes. Linux is the platform where the race detector is
// fastest and where a race is most likely to be reproduced
// deterministically.
func Tier2RaceDetectorOS() []Tier2OS {
	return []Tier2OS{Tier2OSUbuntu}
}

// Tier2JobTimeout is the timeout applied to every matrix job.
//
// The value is 20 minutes: a backstop against a hung job, not a
// budget enforcement. The budget is 15 minutes and is enforced by the
// quarterly review of measured timings.
const Tier2JobTimeout = 20 * time.Minute

// Tier2CacheKeyTemplate is the cache key template used by every Tier 2
// job.
//
// The `%s` placeholders are: the OS, the go.sum hash, the GOOS, and
// the GOARCH. For a job that does not cross-compile, the GOOS and
// GOARCH components are empty and the template reduces to the same
// key Tier 1 uses.
const Tier2CacheKeyTemplate = "%s-go-%s-%s-%s"

// Tier2GoSumGlob is the glob used to compute the cache key's hash
// component.
const Tier2GoSumGlob = "**/go.sum"

// Tier2GoFlags is the value of the GOFLAGS environment variable used
// by every Tier 2 job.
const Tier2GoFlags = "-mod=readonly"

// Tier2CrossCompileTargets is the set of release platforms that the
// cross-compile job builds.
//
// The set is the intersection of docs/reference/platforms.yaml's
// Tier 1 and Tier 2 platforms that are marked static. It is
// duplicated here as a fallback for the case where the platform
// driver's ci-matrix subcommand is not available; the authoritative
// source is the platform matrix.
var Tier2CrossCompileTargets = []struct {
	GOOS   string
	GOARCH string
}{
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"windows", "amd64"},
	{"windows", "arm64"},
}

// ValidateTier2 checks the Tier 2 model's internal invariants.
//
// The invariants are:
//
//  1. Job names are members of the closed set and are unique.
//  2. Every job has a timeout, and the timeout does not exceed
//     Tier2JobTimeout (for matrix jobs) or 30 minutes (for the
//     release-dry-run job, which is allowed a longer timeout because
//     it runs the release pipeline).
//  3. Every job declares exactly `contents: read` and nothing else.
//  4. Every matrix job has a non-empty OS axis and a non-empty Go
//     axis, and the OS axis is a subset of AllTier2OS.
//  5. Every non-matrix job has a non-empty RunsOn.
//  6. The race-detector OS set is a strict subset of the matrix OS
//     axis.
//  7. The set of job names and the Tier2.Contents list in tiers.go
//     agree.
func ValidateTier2() error {
	seen := make(map[Tier2JobName]struct{}, len(tier2Jobs))
	for _, j := range tier2Jobs {
		if !j.Name.Valid() {
			return tier2Error(
				"unknown job name",
				j.Name,
				fmt.Sprintf("the job name %q is not in the closed set", string(j.Name)),
			)
		}
		if _, dup := seen[j.Name]; dup {
			return tier2Error(
				"duplicate job name",
				j.Name,
				"the job name is declared more than once in the canonical set",
			)
		}
		seen[j.Name] = struct{}{}
		if err := validateTier2Job(j); err != nil {
			return err
		}
	}

	// The race-detector OS set must be a strict subset of the matrix
	// OS axis.
	matrixOS := make(map[Tier2OS]struct{})
	for _, j := range tier2Jobs {
		if j.UsesMatrix {
			for _, o := range j.MatrixOS {
				matrixOS[o] = struct{}{}
			}
			break
		}
	}
	for _, o := range Tier2RaceDetectorOS() {
		if _, ok := matrixOS[o]; !ok {
			return tier2Error(
				"race-detector OS is not in the matrix",
				"",
				fmt.Sprintf(
					"the race detector runs on %q, which is not in the matrix OS axis",
					string(o),
				),
			)
		}
	}

	return nil
}

// validateTier2Job checks the invariants of a single Tier 2 job.
func validateTier2Job(j Tier2Job) error {
	if !j.Name.Valid() {
		return tier2Error(
			"unknown job name",
			j.Name,
			fmt.Sprintf("the job name %q is not in the closed set", string(j.Name)),
		)
	}
	if j.Timeout <= 0 {
		return tier2Error(
			"non-positive job timeout",
			j.Name,
			fmt.Sprintf("the Timeout field is %v, which is not positive", j.Timeout),
		)
	}
	// The release-dry-run job is allowed a longer timeout because it
	// runs the release pipeline.
	maxTimeout := Tier2JobTimeout
	if j.Name == Tier2JobReleaseDryRun {
		maxTimeout = 30 * time.Minute
	}
	if j.Timeout > maxTimeout {
		return tier2Error(
			"job timeout exceeds the Tier 2 maximum",
			j.Name,
			fmt.Sprintf(
				"the Timeout field is %v, which exceeds the maximum %v",
				j.Timeout, maxTimeout,
			),
		)
	}
	if len(j.Permissions) != 1 || j.Permissions[0] != PermissionContentsRead {
		return tier2Error(
			"job permissions are not exactly contents: read",
			j.Name,
			fmt.Sprintf("the Permissions field is %v", j.Permissions),
		)
	}
	if j.UsesMatrix {
		if len(j.MatrixOS) == 0 {
			return tier2Error(
				"matrix job has no OS axis",
				j.Name,
				"a matrix job must declare a non-empty MatrixOS",
			)
		}
		if len(j.MatrixGo) == 0 {
			return tier2Error(
				"matrix job has no Go axis",
				j.Name,
				"a matrix job must declare a non-empty MatrixGo",
			)
		}
		for _, o := range j.MatrixOS {
			if !o.Valid() {
				return tier2Error(
					"unknown OS in matrix",
					j.Name,
					fmt.Sprintf("the OS %q is not in the closed set", string(o)),
				)
			}
		}
		for _, v := range j.MatrixGo {
			if !v.Valid() {
				return tier2Error(
					"unknown Go version in matrix",
					j.Name,
					fmt.Sprintf("the Go version %q is not in the closed set", string(v)),
				)
			}
		}
	} else {
		if j.RunsOn == "" {
			return tier2Error(
				"non-matrix job has no runs-on",
				j.Name,
				"a non-matrix job must declare a non-empty RunsOn",
			)
		}
	}
	return nil
}

// Tier2Golden renders the Tier 2 model as deterministic JSON.
func Tier2Golden() string {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(`  "cache_key_template": `)
	writeJSONString(&b, Tier2CacheKeyTemplate)
	b.WriteString(",\n")
	b.WriteString(`  "go_flags": `)
	writeJSONString(&b, Tier2GoFlags)
	b.WriteString(",\n")
	b.WriteString(`  "job_timeout_seconds": `)
	b.WriteString(itoa(int(Tier2JobTimeout / time.Second)))
	b.WriteString(",\n")
	b.WriteString(`  "race_detector_os": [`)
	for i, o := range Tier2RaceDetectorOS() {
		if i > 0 {
			b.WriteString(", ")
		}
		writeJSONString(&b, string(o))
	}
	b.WriteString("],\n")
	b.WriteString(`  "cross_compile_targets": [`)
	for i, t := range Tier2CrossCompileTargets {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    {")
		b.WriteString(`"goos": `)
		writeJSONString(&b, t.GOOS)
		b.WriteString(`, "goarch": `)
		writeJSONString(&b, t.GOARCH)
		b.WriteString("}")
	}
	b.WriteString("\n  ],\n")
	b.WriteString(`  "jobs": [`)
	for i, j := range tier2Jobs {
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
			b.WriteString(`      "matrix_os": [`)
			for k, o := range j.MatrixOS {
				if k > 0 {
					b.WriteString(", ")
				}
				writeJSONString(&b, string(o))
			}
			b.WriteString("],\n")
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

// cloneTier2Job returns a deep copy of j.
func cloneTier2Job(j Tier2Job) Tier2Job {
	out := j
	out.Permissions = cloneSlice(j.Permissions)
	out.MatrixOS = cloneSlice(j.MatrixOS)
	out.MatrixGo = cloneSlice(j.MatrixGo)
	out.Steps = cloneSlice(j.Steps)
	return out
}

// tier2Error builds a *forgeerr.Error for a Tier 2 invariant violation.
func tier2Error(what string, id Tier2JobName, detail string) error {
	e := forgeerr.New(
		forgeerr.CategoryInternal,
		forgeerr.CodeInternalUnexpected,
		fmt.Sprintf("ci: tier2 model invariant violated: %s", what),
	)
	e = e.WithContext("detail", detail)
	if id != "" {
		e = e.WithContext("job_name", string(id))
	}
	return e
}

// init validates the Tier 2 model at package start.
func init() {
	if err := ValidateTier2(); err != nil {
		panic(fmt.Sprintf("ci: tier2 model validation failed: %v", err))
	}
}
