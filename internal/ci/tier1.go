// Package ci — typed model of the Tier 1 fast-feedback workflow.
//
// This file adds the Tier 1-specific model to the tier model in
// tiers.go. Where tiers.go says "Tier 1 has a 3-minute budget and
// these five checks", this file says "the five checks are named
// tier1-format, tier1-vet, ..., they run in parallel, each has a
// 15-minute job timeout, each restores a Go module cache keyed on
// the go.sum hash, and each sets GOFLAGS=-mod=readonly".
//
// The model is consumed by:
//
//   - internal/ci/tier1_contract_test.go, which reads
//     .github/workflows/ci-tier1.yml and asserts the file implements
//     the model.
//   - internal/ci/tiers.go, which uses the job names to populate the
//     Tier1.Contents list.
//   - docs/reference/ci-tiers.md, which renders the tier table.
//
// # Why a separate file from tiers.go
//
// tiers.go is the cross-tier model. It names the four tiers, their
// budgets, their blocking rules, and their contents. tier1.go is the
// Tier 1-specific model. The split keeps tiers.go small enough to
// review in one sitting and keeps the Tier 1 details (cache key,
// job timeout, pinned action set) in a file whose name says what it
// is about.
//
// The same split will be used for Tiers 2, 3, and 4 when 18.3.1,
// 18.4.1, and the security/nightly WBS items land.
package ci

import (
	"fmt"
	"strings"
	"time"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// Tier1JobName is the closed set of Tier 1 job names.
//
// The set is closed. Every name appears as a job in
// .github/workflows/ci-tier1.yml, as a name in the Tier1.Contents
// list in tiers.go, and as a string in this file. A change to the
// set requires a change to all three, and the contract test
// TestTier1_EveryJobIsImplementedInTheWorkflow asserts the first
// two agree.
//
// The type is string-backed for the same reasons as TierID and
// WorkflowID: it survives serialization into the golden file and
// into CI logs, and it produces readable diffs.
type Tier1JobName string

const (
	// Tier1JobFormat is the format-check job. It runs gofmt -l and
	// goimports -l and fails if either lists a file.
	Tier1JobFormat Tier1JobName = "tier1-format"

	// Tier1JobVet is the vet job. It runs go vet ./...
	Tier1JobVet Tier1JobName = "tier1-vet"

	// Tier1JobLint is the lint job. It runs golangci-lint with
	// .golangci.yml.
	Tier1JobLint Tier1JobName = "tier1-lint"

	// Tier1JobTestShort is the test-short job. It runs
	// go test -short -race -count=1 ./...
	Tier1JobTestShort Tier1JobName = "tier1-test-short"

	// Tier1JobBuild is the build-host job. It runs go build ./...
	// and builds the forge binary for the host platform.
	Tier1JobBuild Tier1JobName = "tier1-build"
)

// allTier1Jobs is the canonical enumeration of the closed set.
//
// It is unexported so that callers cannot mutate it; callers use
// AllTier1Jobs, which returns a copy. This mirrors allTiers and
// allWorkflows.
//
// Ordering is the order in which the jobs are presented in the
// workflow file and in the tier table: format, vet, lint, test,
// build. The order is not the run order (the jobs run in parallel)
// and is not load-bearing beyond the rendering; it is kept stable so
// that the golden file does not churn.
var allTier1Jobs = []Tier1JobName{
	Tier1JobFormat,
	Tier1JobVet,
	Tier1JobLint,
	Tier1JobTestShort,
	Tier1JobBuild,
}

// AllTier1Jobs returns a copy of the closed set of Tier 1 job names.
func AllTier1Jobs() []Tier1JobName {
	out := make([]Tier1JobName, len(allTier1Jobs))
	copy(out, allTier1Jobs)
	return out
}

// String returns the job name as a string.
func (j Tier1JobName) String() string { return string(j) }

// Valid reports whether j is a member of the closed set.
func (j Tier1JobName) Valid() bool {
	for _, k := range allTier1Jobs {
		if j == k {
			return true
		}
	}
	return false
}

// Tier1Job describes one job in the Tier 1 workflow.
//
// The fields are the properties that the contract test asserts
// against the YAML file. A field that is not represented here is a
// property the model does not constrain; the model's job is to
// constrain the properties that matter, not to duplicate the YAML.
type Tier1Job struct {
	// Name is the job's name in the workflow file.
	Name Tier1JobName

	// Timeout is the job's `timeout-minutes` value. The model
	// constrains it to a maximum (see validateTier1Job), not to an
	// exact value: the workflow file may use a shorter timeout, and
	// a future change that shortens it should not fail the model.
	Timeout time.Duration

	// Permissions is the job's `permissions` block. Every Tier 1
	// job declares exactly `contents: read`.
	Permissions []Permission

	// UsesCache reports whether the job restores the Go module
	// cache. Every Tier 1 job does, because every Tier 1 job needs
	// the module cache to meet the 3-minute budget.
	UsesCache bool

	// Steps is the ordered list of the job's step names. The model
	// does not constrain step contents; it constrains the presence
	// of the two steps every job needs (checkout and setup-go) and
	// the job-specific step (a single run step or a single uses
	// step).
	Steps []string
}

// tier1Jobs is the canonical description of the five Tier 1 jobs.
//
// It is unexported so that callers cannot mutate it; callers use
// Tier1Jobs, which returns a deep copy.
//
// The order matches allTier1Jobs. The two must not drift; the test
// suite asserts the agreement.
var tier1Jobs = []Tier1Job{
	{
		Name:        Tier1JobFormat,
		Timeout:     15 * time.Minute,
		Permissions: []Permission{PermissionContentsRead},
		UsesCache:   true,
		Steps: []string{
			"Check out the repository",
			"Set up Go",
			"gofmt",
			"goimports",
		},
	},
	{
		Name:        Tier1JobVet,
		Timeout:     15 * time.Minute,
		Permissions: []Permission{PermissionContentsRead},
		UsesCache:   true,
		Steps: []string{
			"Check out the repository",
			"Set up Go",
			"go vet",
		},
	},
	{
		Name:        Tier1JobLint,
		Timeout:     15 * time.Minute,
		Permissions: []Permission{PermissionContentsRead},
		UsesCache:   true,
		Steps: []string{
			"Check out the repository",
			"Set up Go",
			"golangci-lint",
		},
	},
	{
		Name:        Tier1JobTestShort,
		Timeout:     15 * time.Minute,
		Permissions: []Permission{PermissionContentsRead},
		UsesCache:   true,
		Steps: []string{
			"Check out the repository",
			"Set up Go",
			"go test -short -race -count=1",
		},
	},
	{
		Name:        Tier1JobBuild,
		Timeout:     15 * time.Minute,
		Permissions: []Permission{PermissionContentsRead},
		UsesCache:   true,
		Steps: []string{
			"Check out the repository",
			"Set up Go",
			"go build ./...",
			"Build the forge binary",
		},
	},
}

// Tier1Jobs returns a deep copy of the canonical Tier 1 job
// descriptions.
func Tier1Jobs() []Tier1Job {
	out := make([]Tier1Job, len(tier1Jobs))
	for i, j := range tier1Jobs {
		out[i] = cloneTier1Job(j)
	}
	return out
}

// LookupTier1Job returns a deep copy of the Tier 1 job with the
// given name and reports whether it was found.
func LookupTier1Job(name Tier1JobName) (Tier1Job, bool) {
	for _, j := range tier1Jobs {
		if j.Name == name {
			return cloneTier1Job(j), true
		}
	}
	return Tier1Job{}, false
}

// Tier1CacheKeyTemplate is the cache key template used by every
// Tier 1 job.
//
// The `%s` placeholder is the OS. The `%s` is substituted by the
// caller; the model does not commit to a particular substitution
// because a future change might use a different runner.
//
// The template is a contract: the contract test asserts that the
// workflow file's cache key matches the template's shape. A future
// change to the cache key is a change to this constant, the workflow
// file, and the golden file, in the same commit.
const Tier1CacheKeyTemplate = "%s-go-%s"

// Tier1CacheRestoreKeyTemplate is the cache restore-key template
// used by every Tier 1 job.
//
// The restore key is a prefix of the cache key, so that a change to
// the go.sum hash restores the previous cache rather than missing
// entirely. This is what makes the warm-cache target achievable
// across a go.sum change.
const Tier1CacheRestoreKeyTemplate = "%s-go-"

// Tier1GoSumGlob is the glob used to compute the cache key's hash
// component.
//
// The glob covers every go.sum in the repository, not just the root
// one. A future multi-module layout (for example, a tools/go.mod
// for the release scripts) would be covered by the same glob.
const Tier1GoSumGlob = "**/go.sum"

// Tier1GoFlags is the value of the GOFLAGS environment variable used
// by every Tier 1 job.
//
// -mod=readonly makes `go build`, `go test`, and `go vet` fail if
// go.mod or go.sum would need to change. The value is the same
// across every job, so it is a workflow-level env rather than a
// per-job env.
const Tier1GoFlags = "-mod=readonly"

// Tier1JobTimeout is the timeout applied to every Tier 1 job.
//
// The value is 15 minutes: a backstop against a hung job, not a
// budget enforcement. The budget is 3 minutes and is enforced by the
// quarterly review of measured timings, not by the job timeout.
const Tier1JobTimeout = 15 * time.Minute

// ValidateTier1 checks the Tier 1 model's internal invariants. It
// returns a structured *forgeerr.Error on the first violation.
//
// The invariants are:
//
//  1. Job names are members of the closed set and are unique.
//  2. Every job has a timeout, and the timeout does not exceed
//     Tier1JobTimeout.
//  3. Every job declares exactly `contents: read` and nothing else.
//  4. Every job restores the Go module cache.
//  5. Every job's steps include "Check out the repository" and
//     "Set up Go".
//  6. The set of job names and the Tier1.Contents list in tiers.go
//     agree: every Tier1JobName appears exactly once in
//     Tier1.Contents, and every Tier1.Contents entry names a
//     Tier1JobName.
//
// The function is total and never panics.
func ValidateTier1() error {
	seen := make(map[Tier1JobName]struct{}, len(tier1Jobs))
	for _, j := range tier1Jobs {
		if !j.Name.Valid() {
			return tier1Error(
				"unknown job name",
				j.Name,
				fmt.Sprintf("the job name %q is not in the closed set", string(j.Name)),
			)
		}
		if _, dup := seen[j.Name]; dup {
			return tier1Error(
				"duplicate job name",
				j.Name,
				"the job name is declared more than once in the canonical set",
			)
		}
		seen[j.Name] = struct{}{}
		if err := validateTier1Job(j); err != nil {
			return err
		}
	}

	// The model and the Tier1.Contents list must agree.
	tier, ok := LookupTier(Tier1)
	if !ok {
		return tier1Error(
			"Tier 1 is missing from tiers.go",
			"",
			"the tier model does not declare Tier 1",
		)
	}
	if len(tier.Contents) != len(tier1Jobs) {
		return tier1Error(
			"Tier 1 contents and job set disagree on size",
			"",
			fmt.Sprintf(
				"Tier1.Contents has %d entries but tier1Jobs has %d",
				len(tier.Contents), len(tier1Jobs),
			),
		)
	}
	contents := make(map[string]struct{}, len(tier.Contents))
	for _, c := range tier.Contents {
		contents[c.Name] = struct{}{}
	}
	for _, j := range tier1Jobs {
		if _, ok := contents[string(j.Name)]; !ok {
			return tier1Error(
				"job is missing from Tier1.Contents",
				j.Name,
				fmt.Sprintf("the job %q is not in tiers.go's Tier1.Contents", string(j.Name)),
			)
		}
	}

	return nil
}

// validateTier1Job checks the invariants of a single Tier 1 job.
func validateTier1Job(j Tier1Job) error {
	if !j.Name.Valid() {
		return tier1Error(
			"unknown job name",
			j.Name,
			fmt.Sprintf("the job name %q is not in the closed set", string(j.Name)),
		)
	}
	if j.Timeout <= 0 {
		return tier1Error(
			"non-positive job timeout",
			j.Name,
			fmt.Sprintf("the Timeout field is %v, which is not positive", j.Timeout),
		)
	}
	if j.Timeout > Tier1JobTimeout {
		return tier1Error(
			"job timeout exceeds the Tier 1 maximum",
			j.Name,
			fmt.Sprintf(
				"the Timeout field is %v, which exceeds the maximum %v",
				j.Timeout, Tier1JobTimeout,
			),
		)
	}
	if len(j.Permissions) != 1 || j.Permissions[0] != PermissionContentsRead {
		return tier1Error(
			"job permissions are not exactly contents: read",
			j.Name,
			fmt.Sprintf("the Permissions field is %v; every Tier 1 job must declare exactly contents: read", j.Permissions),
		)
	}
	if !j.UsesCache {
		return tier1Error(
			"job does not use the Go module cache",
			j.Name,
			"every Tier 1 job must restore the Go module cache to meet the 3-minute budget",
		)
	}
	// Every job must check out the repository and set up Go.
	hasCheckout := false
	hasSetupGo := false
	for _, s := range j.Steps {
		switch s {
		case "Check out the repository":
			hasCheckout = true
		case "Set up Go":
			hasSetupGo = true
		}
	}
	if !hasCheckout {
		return tier1Error(
			"job is missing the checkout step",
			j.Name,
			"every Tier 1 job must check out the repository first",
		)
	}
	if !hasSetupGo {
		return tier1Error(
			"job is missing the setup-go step",
			j.Name,
			"every Tier 1 job must set up Go",
		)
	}
	return nil
}

// Tier1Golden renders the Tier 1 model as a deterministic JSON
// document.
//
// The JSON is the golden file for the contract test
// TestTier1_GoldenMatchesModel. It is regenerated by
// `task ci:render-tier1`.
//
// The rendering is stable: the same model always produces the same
// bytes. This is what makes the golden file useful: a change to the
// model produces a diff, and a reviewer can see exactly what
// changed.
func Tier1Golden() string {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(`  "cache_key_template": `)
	writeJSONString(&b, Tier1CacheKeyTemplate)
	b.WriteString(",\n")
	b.WriteString(`  "cache_restore_key_template": `)
	writeJSONString(&b, Tier1CacheRestoreKeyTemplate)
	b.WriteString(",\n")
	b.WriteString(`  "go_flags": `)
	writeJSONString(&b, Tier1GoFlags)
	b.WriteString(",\n")
	b.WriteString(`  "go_sum_glob": `)
	writeJSONString(&b, Tier1GoSumGlob)
	b.WriteString(",\n")
	b.WriteString(`  "job_timeout_seconds": `)
	b.WriteString(itoa(int(Tier1JobTimeout / time.Second)))
	b.WriteString(",\n")
	b.WriteString(`  "jobs": [`)
	for i, j := range tier1Jobs {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    {\n")
		b.WriteString(`      "name": `)
		writeJSONString(&b, string(j.Name))
		b.WriteString(",\n")
		b.WriteString(`      "permissions": [`)
		for k, p := range j.Permissions {
			if k > 0 {
				b.WriteString(", ")
			}
			writeJSONString(&b, string(p))
		}
		b.WriteString("],\n")
		b.WriteString(`      "steps": [`)
		for k, s := range j.Steps {
			if k > 0 {
				b.WriteString(", ")
			}
			writeJSONString(&b, s)
		}
		b.WriteString("],\n")
		b.WriteString(`      "timeout_seconds": `)
		b.WriteString(itoa(int(j.Timeout / time.Second)))
		b.WriteString(",\n")
		b.WriteString(`      "uses_cache": `)
		if j.UsesCache {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		b.WriteString("\n    }")
	}
	b.WriteString("\n  ]\n}\n")
	return b.String()
}

// writeJSONString writes s as a JSON string literal to b.
//
// The function is a minimal JSON string encoder. It escapes the
// characters required by RFC 8259: quotation mark, reverse solidus,
// and the C0 control characters. It does not escape non-ASCII
// characters, which the JSON specification allows to appear raw in
// a UTF-8 document.
//
// The function exists because the package does not import
// encoding/json. The package's output is a small, fixed-shape
// document; a general-purpose encoder would be more code than the
// document needs, and it would introduce a marshalling step whose
// ordering is not guaranteed by the standard library.
func writeJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// itoa converts a small non-negative int to a string. The helper
// exists so that the file does not import strconv for one call site.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		panic(fmt.Sprintf("ci: itoa called with negative int %d", n))
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// cloneTier1Job returns a deep copy of j.
func cloneTier1Job(j Tier1Job) Tier1Job {
	out := j
	out.Permissions = cloneSlice(j.Permissions)
	out.Steps = cloneSlice(j.Steps)
	return out
}

// tier1Error builds a *forgeerr.Error for a Tier 1 invariant
// violation.
func tier1Error(what string, id Tier1JobName, detail string) error {
	e := forgeerr.New(
		forgeerr.CategoryInternal,
		forgeerr.CodeInternalUnexpected,
		fmt.Sprintf("ci: tier1 model invariant violated: %s", what),
	)
	e = e.WithContext("detail", detail)
	if id != "" {
		e = e.WithContext("job_name", string(id))
	}
	return e
}

// init validates the Tier 1 model at package start.
func init() {
	if err := ValidateTier1(); err != nil {
		panic(fmt.Sprintf("ci: tier1 model validation failed: %v", err))
	}
}

// Compile-time assertions that the package's public surface does not
// drift.
var (
	_ func() []Tier1JobName               = AllTier1Jobs
	_ func() []Tier1Job                   = Tier1Jobs
	_ func(Tier1JobName) (Tier1Job, bool) = LookupTier1Job
	_ func() error                        = ValidateTier1
	_ func() string                       = Tier1Golden
)
