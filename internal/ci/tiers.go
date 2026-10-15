// Package ci — typed model of the four CI tiers.
//
// This file defines the tier model that 18.1.1's workflow architecture
// is organised around. A tier is a class of checks with a shared
// goal, a shared timing budget, and a shared blocking status per
// trigger event. The four tiers are:
//
//	Tier 1 — Fast feedback.     Budget: 3 min.    Blocking on PR.
//	Tier 2 — Cross-platform.    Budget: 15 min.   Blocking on PR.
//	Tier 3 — Depth.             Budget: 60 min.   Advisory on PR,
//	                                              blocking on main.
//	Tier 4 — Scheduled hygiene. Budget: 120 min.  Advisory.
//
// The model is the single source of truth for the tier table that is
// published in docs/reference/ci-tiers.md. The table is rendered from
// the model by TiersTable, and a golden-file test asserts the
// published document matches the rendering. This is the same
// discipline the repository applies to the architecture matrix
// (internal/arch/matrix.go renders docs/architecture/dependency-matrix.md)
// and to the error-code registry (internal/forgeerr/codes.go renders
// docs/reference/error-codes.md).
//
// # Why the model is in Go and not in YAML
//
// A YAML file would be data, not code. The tier model has invariants
// (Tier 1 < Tier 2 < Tier 3 < Tier 4; a tier's blocking status is
// consistent with its trigger events; every check in a tier's content
// is declared in a workflow file). Invariants are testable only if
// they are expressed in a language that has a test framework. Go has
// one; YAML does not.
//
// A YAML file would also require a schema (schemas/ci-tiers.schema.json)
// and a validator, duplicating work that Go's type system already
// does. The Go model is the schema.
//
// # Why the model is not a generated artifact
//
// The alternative — generating the Go model from a YAML file at build
// time — was considered and rejected. The generator would be a second
// source of truth (the YAML) behind the first (the Go), and a change
// to either would require a regeneration step that is easy to forget.
// The repository's discipline is that models are hand-written, small,
// and testable; generation is reserved for artifacts that are too
// large to review (the SBOM, the checksum files).
//
// # Why the timings are time.Duration
//
// The budget for a tier is a duration, not a string. Storing it as a
// string would make the comparison against observed CI times a string
// parse, which is both slower and less safe than a Duration
// comparison. Storing it as a Duration makes "Tier 1 completes in
// less than 3 minutes" a machine-checkable assertion:
//
//	observed := 2*time.Minute + 47*time.Second
//	if observed > Tier1.Budget {
//	    // budget exceeded
//	}
//
// The rendering to Markdown converts the Duration to a human-readable
// form (3m, 15m, 60m, 120m). The conversion is done by formatBudget,
// not by fmt.Sprintf, so that the rendering is deterministic and can
// be pinned by a golden file.
package ci

import (
	"fmt"
	"strings"
	"time"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// TierID is the closed set of tier identifiers.
//
// The set is closed for the same reason as WorkflowID and
// forgeerr.Category: a new tier is a change to the repository's CI
// architecture and deserves a review, not a YAML edit. Adding a tier
// requires:
//
//  1. A new constant below.
//  2. A new entry in allTiers and in tiers.
//  3. An update to the ADR (docs/decisions/ADR-044-ci-tier-model.md).
//  4. An update to the golden file (testdata/ci-tiers.golden.md).
//
// The test suite in tiers_test.go asserts that allTiers, tiers, and
// the golden file agree; a constant declared but missing from tiers
// fails the build.
type TierID string

const (
	// Tier1 is fast feedback: the checks that run on every PR push and
	// complete in under three minutes on a warm cache. Its purpose is
	// to catch the majority of mistakes (formatting, vet, lint,
	// compile errors, unit-test failures) before a reviewer is
	// notified.
	//
	// Contents:
	//   - gofmt / goimports check
	//   - go vet
	//   - golangci-lint
	//   - go test -short ./...
	//   - go build ./...
	//
	// Budget: 3 minutes.
	// Blocking: yes, on every PR push.
	// Trigger: pull_request, push (to main).
	Tier1 TierID = "tier1"

	// Tier2 is cross-platform confidence: the checks that prove the
	// code works on every supported operating system and every
	// supported Go version. It runs the full unit and integration
	// suites on a matrix of {ubuntu, windows, macos} × {min Go,
	// current Go}, cross-compiles all six release platforms, and runs
	// the binary smoke suite (17.5.1) on ubuntu.
	//
	// Contents:
	//   - full unit tests
	//   - integration tests
	//   - cross-compile of all six release platforms
	//   - binary smoke tests (17.5.1) on ubuntu
	//
	// Budget: 15 minutes.
	// Blocking: yes, on every PR push and every push to main.
	// Trigger: pull_request, push (to main).
	Tier2 TierID = "tier2"

	// Tier3 is depth: the checks that are too slow to run on every PR
	// push but are cheap enough to run on every push to main. It runs
	// the race detector, a two-count test run (which catches flaky
	// tests), a cross-compile of all six release platforms, and the
	// release dry-run.
	//
	// Contents:
	//   - go test -race ./...
	//   - go test -count=2 ./...
	//   - cross-compile of all six release platforms
	//   - make release-dry-run
	//
	// Budget: 60 minutes.
	// Blocking: advisory on pull_request, blocking on push (to main).
	// Trigger: pull_request, push (to main).
	Tier3 TierID = "tier3"

	// Tier4 is scheduled hygiene: the checks that are too slow or too
	// noisy to run on every PR. It runs CodeQL, govulncheck, a secret
	// scan, a license scan, a one-hour fuzz run, and a coverage
	// regression check.
	//
	// Contents:
	//   - CodeQL analysis
	//   - govulncheck
	//   - secret scan (gitleaks)
	//   - license scan (go-licenses)
	//   - fuzzing (1 hour)
	//   - coverage regression check
	//
	// Budget: 120 minutes.
	// Blocking: advisory.
	// Trigger: schedule (nightly), workflow_dispatch.
	Tier4 TierID = "tier4"
)

// allTiers is the canonical enumeration of the closed set.
//
// It is unexported so that callers cannot mutate it; callers use
// AllTiers, which returns a copy. This mirrors forgeerr.allCategories
// and ci.allWorkflows.
//
// Ordering is deliberate and stable: Tier1, Tier2, Tier3, Tier4. The
// order is the order in which the tiers are presented in the tier
// table and in the ADR. Reordering would break the golden file.
var allTiers = []TierID{
	Tier1,
	Tier2,
	Tier3,
	Tier4,
}

// AllTiers returns a copy of the closed set of tiers.
//
// The copy protects the internal enumeration from mutation. The order
// is stable and is part of the contract: Tier1, Tier2, Tier3, Tier4.
func AllTiers() []TierID {
	out := make([]TierID, len(allTiers))
	copy(out, allTiers)
	return out
}

// String returns the tier ID as a string.
func (t TierID) String() string { return string(t) }

// Valid reports whether t is a member of the closed set.
//
// The check is a linear scan over allTiers. The set has four members,
// so a scan is faster than a map lookup and has zero allocation.
func (t TierID) Valid() bool {
	for _, k := range allTiers {
		if t == k {
			return true
		}
	}
	return false
}

// Number returns the tier's ordinal number (1, 2, 3, or 4).
//
// The number is derived from the tier's position in allTiers, not
// from a parse of the ID string. This is deliberate: the ID's string
// value ("tier1") happens to end in the number, but the string is not
// the source of truth. If a future tier is named "tier-fast" and
// inserted at position 1, Number returns 1 regardless of the string.
//
// Number panics on an unknown ID. The panic is the same
// programming-error signal as Category.Prefix and WorkflowID.FileName.
func (t TierID) Number() int {
	for i, k := range allTiers {
		if t == k {
			return i + 1
		}
	}
	panic(fmt.Sprintf("ci: TierID.Number called on invalid tier %q", string(t)))
}

// BlockingStatus describes whether a tier's failures block a merge
// for a given trigger event.
//
// The type is a closed set of three values. It is modelled as a
// distinct type rather than a bool because a tier can be blocking on
// one trigger and advisory on another: Tier3 is advisory on
// pull_request and blocking on push. A single bool would force the
// caller to know which trigger it is asking about, and would make the
// model's Validate function unable to express the rule.
type BlockingStatus string

const (
	// BlockingOnPR means the tier's failures block a merge when they
	// occur on a pull_request event.
	BlockingOnPR BlockingStatus = "blocking-on-pr"

	// BlockingOnMain means the tier's failures block a merge when they
	// occur on a push to main, but not when they occur on a
	// pull_request.
	BlockingOnMain BlockingStatus = "blocking-on-main"

	// Advisory means the tier's failures never block a merge.
	Advisory BlockingStatus = "advisory"
)

// Valid reports whether b is a member of the closed set.
func (b BlockingStatus) Valid() bool {
	switch b {
	case BlockingOnPR, BlockingOnMain, Advisory:
		return true
	default:
		return false
	}
}

// String returns the blocking status as a string.
func (b BlockingStatus) String() string { return string(b) }

// TriggerSet is the set of trigger events that cause a tier to run.
//
// The type is a slice, not a map, because the order is part of the
// contract: the tier table presents the triggers in the order they
// are declared, and a golden file pins the rendering.
type TriggerSet []Trigger

// Contains reports whether the set contains t.
func (ts TriggerSet) Contains(t Trigger) bool {
	for _, k := range ts {
		if k == t {
			return true
		}
	}
	return false
}

// Check is one check in a tier's content.
//
// A check is a named unit of work with a target duration and a
// classification. The target duration is the check's own budget, not
// the tier's: the sum of the checks' targets is bounded by the tier's
// budget, and validateTier asserts the bound.
type Check struct {
	// Name is the check's identifier. It matches the job name in the
	// workflow file that implements it. For example, "tier1-format"
	// matches the job name in ci.yml.
	Name string

	// Target is the check's own time budget. It is not the tier's
	// budget: several checks run in parallel, so the sum of the
	// targets may exceed the tier's budget. The tier's budget is the
	// wall-clock ceiling for the whole tier.
	Target time.Duration

	// Workflow is the workflow that implements the check. The value
	// is the WorkflowID of the workflow file.
	Workflow WorkflowID

	// Notes is an optional human-readable note. It appears in the
	// tier table as a parenthetical after the check's name. An empty
	// Notes produces no parenthetical.
	Notes string
}

// Tier is the typed description of one CI tier.
type Tier struct {
	// ID is the tier identifier.
	ID TierID

	// Name is the human-readable name ("Fast feedback", "Cross-platform
	// confidence", "Depth", "Scheduled hygiene").
	Name string

	// Goal is a one-sentence description of what the tier is for. It
	// is the tier's raison d'être, not a summary of its contents.
	Goal string

	// Budget is the tier's wall-clock ceiling. The contract tests
	// assert Tier1.Budget < Tier2.Budget < Tier3.Budget < Tier4.Budget.
	Budget time.Duration

	// Contents is the ordered list of checks in the tier. The order
	// is the order in which the checks are presented in the tier
	// table. It is not the order in which they run: the workflow
	// engine determines the run order, and some checks run in
	// parallel.
	Contents []Check

	// BlockingOn is the set of trigger events on which the tier's
	// failures block a merge.
	//
	// The set is not the same as the tier's trigger set. A tier may
	// be triggered by an event without being blocking on that event
	// (Tier3 on pull_request). Conversely, a tier's blocking status
	// is defined only for events that trigger it; a tier that is not
	// triggered by an event has no blocking status for that event.
	BlockingOn []Trigger

	// Triggers is the set of trigger events that cause the tier to
	// run. The set is non-empty for every tier.
	Triggers TriggerSet

	// Advisory is true if the tier is never blocking. It is a
	// derived convenience field: Advisory is true if and only if
	// BlockingOn is empty. The field exists so that callers who
	// want a boolean do not have to test len(BlockingOn) == 0.
	Advisory bool
}

// tiers is the canonical description of the four tiers.
//
// It is unexported so that callers cannot mutate it; callers use
// AllTierDescriptions, which returns a deep copy.
//
// The order matches allTiers. The two must not drift; the test suite
// asserts that AllTierDescriptions and AllTiers agree on order.
var tiers = []Tier{
	{
		ID:     Tier1,
		Name:   "Fast feedback",
		Goal:   "Catch the majority of mistakes before a reviewer is notified.",
		Budget: 3 * time.Minute,
		Contents: []Check{
			{Name: "tier1-format", Target: 30 * time.Second, Workflow: WorkflowCI, Notes: "gofmt and goimports"},
			{Name: "tier1-vet", Target: 30 * time.Second, Workflow: WorkflowCI, Notes: "go vet"},
			{Name: "tier1-lint", Target: 90 * time.Second, Workflow: WorkflowCI, Notes: "golangci-lint"},
			{Name: "tier1-test-short", Target: 60 * time.Second, Workflow: WorkflowCI, Notes: "go test -short ./..."},
			{Name: "tier1-build", Target: 30 * time.Second, Workflow: WorkflowCI, Notes: "go build ./..."},
		},
		BlockingOn: []Trigger{TriggerPullRequest, TriggerPush},
		Triggers:   TriggerSet{TriggerPullRequest, TriggerPush},
		Advisory:   false,
	},
	{
		ID:     Tier2,
		Name:   "Cross-platform confidence",
		Goal:   "Prove the code works on every supported OS and Go version.",
		Budget: 15 * time.Minute,
		Contents: []Check{
			{Name: "tier2-test-full", Target: 5 * time.Minute, Workflow: WorkflowCI, Notes: "full unit and integration tests"},
			{Name: "tier2-cross-compile", Target: 3 * time.Minute, Workflow: WorkflowCI, Notes: "all six release platforms"},
			{Name: "tier2-smoke", Target: 2 * time.Minute, Workflow: WorkflowCI, Notes: "binary smoke tests on ubuntu"},
			{Name: "tier2-matrix", Target: 15 * time.Minute, Workflow: WorkflowCI, Notes: "OS and Go version matrix"},
		},
		BlockingOn: []Trigger{TriggerPullRequest, TriggerPush},
		Triggers:   TriggerSet{TriggerPullRequest, TriggerPush},
		Advisory:   false,
	},
	{
		ID:     Tier3,
		Name:   "Depth",
		Goal:   "Catch races, flakes, and integration failures that the fast tiers cannot.",
		Budget: 60 * time.Minute,
		Contents: []Check{
			{Name: "tier3-race", Target: 20 * time.Minute, Workflow: WorkflowCI, Notes: "go test -race ./..."},
			{Name: "tier3-count2", Target: 20 * time.Minute, Workflow: WorkflowCI, Notes: "go test -count=2 ./... for flake detection"},
			{Name: "tier3-cross-compile", Target: 5 * time.Minute, Workflow: WorkflowCI, Notes: "all six release platforms"},
			{Name: "tier3-release-dry-run", Target: 20 * time.Minute, Workflow: WorkflowCI, Notes: "make release-dry-run"},
		},
		BlockingOn: []Trigger{TriggerPush},
		Triggers:   TriggerSet{TriggerPullRequest, TriggerPush},
		Advisory:   false,
	},
	{
		ID:     Tier4,
		Name:   "Scheduled hygiene",
		Goal:   "Detect drift that is invisible to per-commit checks: vulnerabilities, secrets, licenses, fuzz-discovered bugs, and coverage regressions.",
		Budget: 120 * time.Minute,
		Contents: []Check{
			{Name: "tier4-codeql", Target: 30 * time.Minute, Workflow: WorkflowSecurity, Notes: "CodeQL analysis"},
			{Name: "tier4-govulncheck", Target: 10 * time.Minute, Workflow: WorkflowSecurity, Notes: "govulncheck"},
			{Name: "tier4-secret-scan", Target: 5 * time.Minute, Workflow: WorkflowSecurity, Notes: "gitleaks"},
			{Name: "tier4-license-scan", Target: 5 * time.Minute, Workflow: WorkflowSecurity, Notes: "go-licenses"},
			{Name: "tier4-fuzz", Target: 60 * time.Minute, Workflow: WorkflowNightly, Notes: "go test -fuzz=Fuzz -fuzztime=1h"},
			{Name: "tier4-coverage", Target: 10 * time.Minute, Workflow: WorkflowNightly, Notes: "coverage regression check"},
		},
		BlockingOn: nil,
		Triggers:   TriggerSet{TriggerSchedule, TriggerWorkflowDispatch},
		Advisory:   true,
	},
}

// AllTierDescriptions returns a deep copy of the canonical tier
// descriptions.
//
// "Deep" matters: a Tier contains two slices (Contents and
// BlockingOn) and a TriggerSet (which is a slice). A shallow copy
// would share the backing arrays, and a caller that appended to one
// of them would corrupt the canonical set. The deep copy closes that
// hole. This is the same discipline as Workflows.
func AllTierDescriptions() []Tier {
	out := make([]Tier, len(tiers))
	for i, t := range tiers {
		out[i] = cloneTier(t)
	}
	return out
}

// LookupTier returns a deep copy of the Tier with the given ID and
// reports whether it was found. The function is total: it never
// panics, even on an unknown ID.
func LookupTier(id TierID) (Tier, bool) {
	for _, t := range tiers {
		if t.ID == id {
			return cloneTier(t), true
		}
	}
	return Tier{}, false
}

// TiersForWorkflow returns every tier that has at least one check
// implemented in the given workflow, in canonical tier order.
//
// The function is total: it returns an empty slice for a workflow
// that contributes to no tier. This is a valid configuration: a
// future workflow (for example, a documentation-only workflow) may
// not contribute to any tier.
//
// The return value is a fresh slice on every call. Callers that
// mutate it do not affect the model.
func TiersForWorkflow(id WorkflowID) []TierID {
	var out []TierID
	for _, t := range tiers {
		for _, c := range t.Contents {
			if c.Workflow == id {
				out = append(out, t.ID)
				break
			}
		}
	}
	if out == nil {
		// Return an empty non-nil slice rather than nil. A caller
		// that appends to the result does not have to special-case
		// nil, and a caller that ranges over the result does not
		// distinguish "no tiers" from "uninitialised". This is the
		// same contract as ChecksForWorkflow.
		return []TierID{}
	}
	return out
}

// ChecksForWorkflow returns every check that is implemented in the
// given workflow, in the order they appear in their tiers. A check
// that appears in multiple tiers (none currently does, but the model
// does not forbid it) is returned once per appearance.
//
// The return value is a fresh slice on every call. Callers that
// mutate it do not affect the model. A workflow that contributes no
// checks returns an empty non-nil slice, not nil.
func ChecksForWorkflow(id WorkflowID) []Check {
	out := []Check{}
	for _, t := range tiers {
		for _, c := range t.Contents {
			if c.Workflow == id {
				out = append(out, c)
			}
		}
	}
	return out
}

// ValidateTiers checks the tier model's internal invariants. It
// returns a structured *forgeerr.Error on the first violation.
//
// The function is total and never panics.
//
// # The two layers of validation
//
// The invariants divide into two groups:
//
//  1. Per-tier invariants. These hold for every Tier in isolation:
//     the ID is a member of the closed set, the Name and Goal are
//     non-empty, the Budget is positive, the Contents are non-empty
//     with unique names and positive targets, the triggers are
//     members of the closed set, BlockingOn is a subset of Triggers,
//     and Advisory agrees with BlockingOn. These are checked by
//     validateTier.
//
//  2. Collection-level invariants. These hold only for the set as a
//     whole: the IDs are unique, the budgets are strictly increasing
//     in canonical order, the canonical set (tiers) and the canonical
//     order (allTiers) agree on size and element-wise identity. These
//     are checked here.
//
// The split exists because the two groups have different consumers.
// A future caller (for example, a task that adds a new tier
// definition) can validate its own Tier with validateTier without
// re-validating the set; the set-level checks are meaningful only
// when the caller has the whole set.
//
// # Why the budget-ordering check is collection-level
//
// The strictly-increasing-budget invariant compares a tier to its
// predecessor. A per-tier validator that took only a Tier would not
// have a predecessor to compare against, and a version that took a
// (Tier, previousBudget) pair would push the loop's bookkeeping into
// every caller. The comparison belongs in the loop that already has
// the predecessor in scope.
//
// # Why the size-and-order check is collection-level
//
// The canonical set and the canonical order are two representations
// of the same data. A per-tier validator sees neither the slice nor
// the set; it sees one element. The agreement between the two
// representations can only be checked by inspecting both slices.
//
// # Why the ID check is duplicated
//
// ValidateTiers checks t.ID.Valid() in the loop, before calling
// validateTier, so that the failure message names the index
// ("tiers[0] is ..."). validateTier checks the same property again,
// so that a direct caller of validateTier (the test suite, a future
// task) gets a total validator. The two messages are complementary:
// ValidateTiers's names the position in the canonical slice;
// validateTier's names only the ID, because it does not have the
// position. The duplication is one boolean and one error
// construction; the cost is negligible, and the benefit is that
// neither caller has to know about the other's needs.
func ValidateTiers() error {
	seen := make(map[TierID]struct{}, len(tiers))
	var prevBudget time.Duration

	for i, t := range tiers {
		// The duplicate-ID check is collection-level: it needs the
		// map of already-seen IDs, which only the loop has.
		if _, dup := seen[t.ID]; dup {
			return tierError(
				"duplicate tier ID",
				t.ID,
				"the ID is declared more than once in the canonical set",
			)
		}
		seen[t.ID] = struct{}{}

		// The indexed unknown-ID check runs before validateTier so
		// that the failure message names the position. See the
		// docstring's "Why the ID check is duplicated" section.
		if !t.ID.Valid() {
			return tierError(
				"unknown tier ID",
				t.ID,
				fmt.Sprintf("tiers[%d] is %q, which is not in the closed set", i, string(t.ID)),
			)
		}

		// The per-tier invariants are checked by validateTier. The
		// loop does not duplicate them.
		if err := validateTier(t); err != nil {
			return err
		}

		// The budget-ordering check is collection-level: it compares
		// the current tier's budget to the previous tier's. The
		// first iteration's prevBudget is the zero Duration, and
		// validateTier has already guaranteed the current budget is
		// positive, so the first iteration always passes.
		if t.Budget <= prevBudget {
			return tierError(
				"non-increasing tier budget",
				t.ID,
				fmt.Sprintf(
					"the Budget field is %v, which is not greater than the previous tier's %v",
					t.Budget, prevBudget,
				),
			)
		}
		prevBudget = t.Budget
	}

	// The canonical set and the canonical order must agree.
	if len(tiers) != len(allTiers) {
		return tierError(
			"tier set size mismatch",
			"",
			fmt.Sprintf("allTiers has %d entries but tiers has %d", len(allTiers), len(tiers)),
		)
	}
	for i, id := range allTiers {
		if tiers[i].ID != id {
			return tierError(
				"tier order mismatch",
				"",
				fmt.Sprintf(
					"allTiers[%d] is %q but tiers[%d].ID is %q",
					i, string(id), i, string(tiers[i].ID),
				),
			)
		}
	}

	return nil
}

// validateTier checks the invariants of a single Tier.
//
// The function is the per-entry validator that ValidateTiers calls in
// its loop. It is unexported because callers outside the package
// should not need to ask "is this tier valid?" — they should be
// constructing tiers through the model, which ValidateTiers
// validates. Internal callers (ValidateTiers, the test suite) use
// validateTier directly.
//
// # The invariants checked
//
//  1. ID is a member of the closed set.
//  2. Name is non-empty.
//  3. Goal is non-empty.
//  4. Budget is positive.
//  5. Contents is non-empty.
//  6. Every check has a non-empty Name, and the names are unique
//     within the tier.
//  7. Every check has a positive Target.
//  8. Every check names a Workflow that is a member of the closed
//     set.
//  9. The sum of the checks' targets is bounded by a reasonable
//     multiple of the tier's budget.
//  10. Triggers is non-empty and every trigger is a member of the
//     closed set.
//  11. BlockingOn is a subset of Triggers.
//  12. Advisory agrees with BlockingOn.
//
// The function does NOT check:
//
//   - That the tier's budget is greater than its predecessor's. That
//     is a collection-level invariant; see ValidateTiers.
//   - That the tier's ID is unique among the canonical set. That is
//     also collection-level.
//
// # Why the checks are ordered this way
//
// The order is the order in which a reader builds a tier: first the
// identity (ID, Name, Goal), then the budget, then the contents, then
// the triggers, then the blocking rule. The order is not load-bearing
// — every failure is equally diagnosable — but it makes the failure
// message a reader sees in CI correspond to the field they are most
// likely to be editing when the failure occurs.
//
// # Why the sum-of-targets bound is a heuristic
//
// The sum of the checks' targets may exceed the tier's budget because
// checks run in parallel. The bound at four times the budget is a
// sanity check: a tier whose checks sum to more than four times its
// budget is almost certainly a modelling error, not a genuinely
// parallel workflow. The bound is a heuristic, not a scheduler; it
// catches the common mistake without pretending to model the
// workflow engine's actual concurrency.
//
// # Totality
//
// The function is total: it never panics. It returns a
// *forgeerr.Error for every violation, including violations that a
// caller might expect to be impossible (for example, a Tier with a
// zero value). The test suite exercises the zero value.
func validateTier(t Tier) error {
	if !t.ID.Valid() {
		return tierError(
			"unknown tier ID",
			t.ID,
			fmt.Sprintf("the ID %q is not in the closed set", string(t.ID)),
		)
	}
	if t.Name == "" {
		return tierError("empty tier name", t.ID, "the Name field is required")
	}
	if t.Goal == "" {
		return tierError("empty tier goal", t.ID, "the Goal field is required")
	}
	if t.Budget <= 0 {
		return tierError(
			"non-positive tier budget",
			t.ID,
			fmt.Sprintf("the Budget field is %v, which is not positive", t.Budget),
		)
	}
	if len(t.Contents) == 0 {
		return tierError(
			"no contents",
			t.ID,
			"the Contents field must have at least one check",
		)
	}

	var sum time.Duration
	names := make(map[string]struct{}, len(t.Contents))
	for _, c := range t.Contents {
		if c.Name == "" {
			return tierError(
				"check with empty name",
				t.ID,
				"every check must have a non-empty Name",
			)
		}
		if _, dup := names[c.Name]; dup {
			return tierError(
				"duplicate check name",
				t.ID,
				fmt.Sprintf("check %q is declared more than once", c.Name),
			)
		}
		names[c.Name] = struct{}{}

		if c.Target <= 0 {
			return tierError(
				"check with non-positive target",
				t.ID,
				fmt.Sprintf("check %q has Target %v, which is not positive", c.Name, c.Target),
			)
		}
		if !c.Workflow.Valid() {
			return tierError(
				"check in unknown workflow",
				t.ID,
				fmt.Sprintf(
					"check %q names workflow %q, which is not in the closed set",
					c.Name, string(c.Workflow),
				),
			)
		}
		sum += c.Target
	}

	// The parallelism heuristic. See the docstring above for the
	// rationale. The constant is named so that a future change to the
	// heuristic is a one-line edit with a visible name.
	const maxParallelism = 4
	if sum > time.Duration(maxParallelism)*t.Budget {
		return tierError(
			"checks' targets exceed four times the tier budget",
			t.ID,
			fmt.Sprintf(
				"the checks sum to %v but the budget is %v; with parallelism of at most %d, the tier is impossible",
				sum, t.Budget, maxParallelism,
			),
		)
	}

	if len(t.Triggers) == 0 {
		return tierError(
			"no triggers",
			t.ID,
			"the Triggers field must have at least one entry",
		)
	}
	for _, tr := range t.Triggers {
		if !tr.Valid() {
			return tierError(
				"unknown trigger",
				t.ID,
				fmt.Sprintf("Triggers contains %q, which is not in the closed set", string(tr)),
			)
		}
	}

	// BlockingOn must be a subset of Triggers.
	for _, b := range t.BlockingOn {
		if !t.Triggers.Contains(b) {
			return tierError(
				"blocking on a non-triggering event",
				t.ID,
				fmt.Sprintf(
					"BlockingOn contains %q but Triggers does not; a tier cannot block on an event it does not run on",
					string(b),
				),
			)
		}
	}

	// Advisory must be true if and only if BlockingOn is empty.
	wantAdvisory := len(t.BlockingOn) == 0
	if t.Advisory != wantAdvisory {
		return tierError(
			"Advisory field disagrees with BlockingOn",
			t.ID,
			fmt.Sprintf("Advisory is %v but len(BlockingOn) == %d", t.Advisory, len(t.BlockingOn)),
		)
	}

	return nil
}

// TiersTable renders the tier table as Markdown.
//
// The rendering is deterministic: the same model always produces the
// same bytes. This is the property the golden-file test relies on.
//
// The table has one row per tier, with columns:
//
//	Tier | Name | Budget | Blocking | Triggers
//
// followed by a detail section per tier. The detail section names the
// tier's goal, budget, blocking rule, triggers, and contents. Each
// check's line names the check, its target, its workflow, and its
// optional notes.
//
// The rendering is deliberately simple: no colours, no HTML, no
// anchors beyond a heading per tier. A reader who wants the model's
// exact shape reads the Go source; a reader who wants the published
// table reads docs/reference/ci-tiers.md, which is this rendering
// checked in.
func TiersTable() string {
	var b strings.Builder

	b.WriteString("# CI Tier Model\n\n")
	b.WriteString("> **Status:** Accepted\n")
	b.WriteString("> **Owner:** Release Engineering\n")
	b.WriteString("> **Related:** [ADR-044](../decisions/ADR-044-ci-tier-model.md), [CI Architecture](../architecture/ci-architecture.md)\n\n")
	b.WriteString("The four tiers below are the contract between the CI architecture and the\n")
	b.WriteString("contributors who depend on it. A tier is a class of checks with a shared\n")
	b.WriteString("goal, a shared timing budget, and a shared blocking status per trigger\n")
	b.WriteString("event. The table is rendered from `internal/ci/tiers.go`; a golden-file\n")
	b.WriteString("test asserts that this document matches the model.\n\n")

	// The summary table.
	b.WriteString("## Summary\n\n")
	b.WriteString("| Tier | Name | Budget | Blocking | Triggers |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, t := range tiers {
		b.WriteString("| ")
		b.WriteString(fmt.Sprintf("Tier %d", t.ID.Number()))
		b.WriteString(" | ")
		b.WriteString(t.Name)
		b.WriteString(" | ")
		b.WriteString(formatBudget(t.Budget))
		b.WriteString(" | ")
		b.WriteString(formatBlocking(t))
		b.WriteString(" | ")
		b.WriteString(formatTriggers(t.Triggers))
		b.WriteString(" |\n")
	}
	b.WriteString("\n")

	// The detail sections.
	b.WriteString("## Detail\n\n")
	for _, t := range tiers {
		b.WriteString("### Tier ")
		b.WriteString(fmt.Sprintf("%d — %s", t.ID.Number(), t.Name))
		b.WriteString("\n\n")
		b.WriteString("**Goal.** ")
		b.WriteString(t.Goal)
		b.WriteString("\n\n")
		b.WriteString("**Budget.** ")
		b.WriteString(formatBudget(t.Budget))
		b.WriteString("\n\n")
		b.WriteString("**Blocking.** ")
		b.WriteString(formatBlocking(t))
		b.WriteString("\n\n")
		b.WriteString("**Triggers.** ")
		b.WriteString(formatTriggers(t.Triggers))
		b.WriteString("\n\n")
		b.WriteString("**Contents.**\n\n")
		for _, c := range t.Contents {
			b.WriteString("- `")
			b.WriteString(c.Name)
			b.WriteString("` (target ")
			b.WriteString(formatBudget(c.Target))
			b.WriteString(", in ")
			b.WriteString(c.Workflow.FileName())
			if c.Notes != "" {
				b.WriteString("; ")
				b.WriteString(c.Notes)
			}
			b.WriteString(")\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

// --- helpers ---------------------------------------------------------------

// cloneTier returns a deep copy of t.
//
// The three slice fields (Contents, BlockingOn, Triggers) are copied
// into fresh backing arrays. A nil slice stays nil; an empty non-nil
// slice stays empty non-nil. The nil-versus-empty distinction is
// preserved because ValidateTiers relies on it (BlockingOn is nil for
// an advisory tier, not an empty slice).
func cloneTier(t Tier) Tier {
	out := t
	out.Contents = make([]Check, len(t.Contents))
	copy(out.Contents, t.Contents)
	out.BlockingOn = cloneSlice(t.BlockingOn)
	out.Triggers = cloneSlice(t.Triggers)
	return out
}

// formatBudget renders a duration as a human-readable budget.
//
// The rendering is deterministic:
//
//   - Durations under a minute render as "Ns".
//   - Durations under an hour render as "Nm" if they are a whole
//     number of minutes, or "Nm Ss" otherwise.
//   - Durations of an hour or more render as "Hh" if they are a whole
//     number of hours, or "Hh Mm" otherwise.
//
// The function is deterministic so that the golden file is stable.
// It uses integer arithmetic, not floating point, so that a duration
// of 90 seconds renders as "1m 30s" and never as "1.5m".
func formatBudget(d time.Duration) string {
	if d < 0 {
		// A negative duration is a programming error; validateTier
		// rejects it. The renderer is total: it produces a string
		// that names the problem rather than panicking. A panic here
		// would take down the rendering of the entire table for one
		// bad field, which is worse than rendering the bad field as
		// invalid.
		return fmt.Sprintf("invalid(%v)", d)
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		m := int(d / time.Minute)
		s := int((d % time.Minute) / time.Second)
		if s == 0 {
			return fmt.Sprintf("%dm", m)
		}
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

// formatBlocking renders a tier's blocking status.
//
// The rendering is "advisory" for an advisory tier, and a
// comma-separated list of the blocking triggers otherwise. The order
// is the order of BlockingOn, which is the order in the canonical
// model. This is part of the contract: a golden file pins the
// rendering.
func formatBlocking(t Tier) string {
	if t.Advisory {
		return "advisory"
	}
	parts := make([]string, 0, len(t.BlockingOn))
	for _, b := range t.BlockingOn {
		parts = append(parts, string(b))
	}
	return strings.Join(parts, ", ")
}

// formatTriggers renders a tier's triggers as a comma-separated list.
func formatTriggers(ts TriggerSet) string {
	parts := make([]string, 0, len(ts))
	for _, tr := range ts {
		parts = append(parts, string(tr))
	}
	return strings.Join(parts, ", ")
}

// tierError builds a *forgeerr.Error for a tier invariant violation.
//
// The error is classified as CategoryInternal with
// CodeInternalUnexpected: a tier-model invariant violation is a bug
// in this package, not a runtime condition a user can fix. The
// alternative code, CodeInternalPanic, is reserved for recovered
// panics at the process boundary and would be misleading here.
//
// The tier ID (when non-empty) and the specific explanation are
// attached as context so that a CI log names the offending tier and
// the exact property that failed. Context keys follow the snake_case
// convention enforced by forgeerr.WithContext: lowercase ASCII
// letters, digits, and underscores, starting with a letter.
//
// The function is shared by ValidateTiers and validateTier. The
// "what" parameter is a short noun phrase ("duplicate tier ID"); the
// "detail" parameter is a full sentence that names the offending
// value. The two together produce the error message the reviewer
// sees in a CI log:
//
//	ci: tier model invariant violated: duplicate tier ID
//	  code: FORGE_INTERNAL_UNEXPECTED
//	  context:
//	    tier_id = "tier1"
//	    detail = "the ID is declared more than once in the canonical set"
func tierError(what string, id TierID, detail string) error {
	e := forgeerr.New(
		forgeerr.CategoryInternal,
		forgeerr.CodeInternalUnexpected,
		fmt.Sprintf("ci: tier model invariant violated: %s", what),
	)
	e = e.WithContext("detail", detail)
	if id != "" {
		e = e.WithContext("tier_id", string(id))
	}
	return e
}

// init validates the tier model at package start.
//
// The check fails the process before any test runs, which is the
// earliest point at which a tier-model invariant violation can be
// caught. The same pattern is used by workflows.go's init for the
// workflow model and the pin map.
//
// A panic from init is the correct failure mode. It is unrecoverable
// by design: the process cannot function with an inconsistent tier
// model, and a silent fallback would hide the bug until a test that
// depends on the model's consistency happened to run.
func init() {
	if err := ValidateTiers(); err != nil {
		panic(fmt.Sprintf("ci: tier model validation failed: %v", err))
	}
}

// Compile-time assertions that the package's public surface does not
// drift. The assertions are deliberately trivial; their value is that
// a future change that alters a signature breaks the build at this
// line, with a message that names the function, rather than at a
// distant call site.
var (
	_ func() []TierID           = AllTiers
	_ func() []Tier             = AllTierDescriptions
	_ func(TierID) (Tier, bool) = LookupTier
	_ func(WorkflowID) []TierID = TiersForWorkflow
	_ func(WorkflowID) []Check  = ChecksForWorkflow
	_ func() error              = ValidateTiers
	_ func() string             = TiersTable
)
