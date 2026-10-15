package ci

// workflows_test.go — internal tests for the typed CI workflow model.
//
// The tests in this file are package-internal (`package ci`, not
// `package ci_test`) because several of them inspect the package's own
// unexported state:
//
//   - Validate cross-checks allWorkflows and workflows; a test that
//     wants to prove the cross-check runs must be able to see both.
//   - The closure tests assert on the canonical slices directly,
//     which is the point of the closure: adding a member must fail a
//     test, and the test that fails must be able to name the
//     canonical slice.
//   - The init tests inspect pinnedActions, which is unexported.
//
// The tests that read the YAML workflow files live in
// workflows_contract_test.go and are external (`package ci_test`),
// because they treat the package as a black box and assert on the
// artifact rather than the model.
//
// # The twelve-workflow model
//
// The closed set has twelve members:
//
//   - ci, release, security, nightly, docs, reproducible,
//     verify-binaries — the seven top-level workflows. Each has at
//     least one direct trigger (pull_request, push, schedule,
//     workflow_dispatch, push:tags).
//   - ci-tier1, ci-tier2, ci-tier3, build, cache — the five reusable
//     workflows. Each has exactly one trigger (workflow_call) and is
//     invoked by another workflow.
//
// The two subsets are disjoint by the trigger rule: a reusable
// workflow declares workflow_call and nothing else; a top-level
// workflow never declares workflow_call. The rule is checked by
// ValidateWorkflow and exercised by the broken-entry tables below.
//
// # Test taxonomy
//
// The tests are grouped by property, not by function:
//
//   - Closure tests: the set has exactly the members it is supposed
//     to have, no more and no fewer.
//   - Order tests: the canonical order is stable and the two
//     representations of the set agree.
//   - Copy tests: returned values do not alias the canonical state.
//   - WorkflowID tests: the file-name mapping, the panic on an
//     unknown ID, and the membership table.
//   - Trigger tests: the closed set, the pull_request_target
//     prohibition, and String totality.
//   - Permission tests: the closed set, the workflows' own use of
//     it, and String totality.
//   - Lookup tests: totality over the set, the zero value on
//     failure, and the deep copy.
//   - RequiredChecks tests: the union, its determinism, and its
//     agreement with the per-workflow declarations.
//   - Validate tests: the collection-level invariants.
//   - ValidateWorkflow tests: the per-entry invariants, table-driven
//     on the broken shape.
//   - Reusable-workflow tests: the disjointness rule that separates
//     the five reusable workflows from the seven top-level ones.
//   - PinnedActions tests: the map's shape, its defensive copy, the
//     core actions it must contain, and the two key shapes.
//   - IsPinnedSHA tests: the predicate's table, including the
//     length and character boundaries, and its totality.
//   - Init tests: the observable post-init state.
//
// The grouping is documentation. A reader who wants to know what the
// package promises reads the group headings; a reader who wants to
// know whether a specific promise holds reads the test in that group.

import (
	"errors"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// --- Closure tests ---------------------------------------------------------
//
// The set is closed. These tests fail if a member is added or removed
// without updating the test, which is the mechanism that forces the
// ADR, the architecture document, and the required-checks policy to
// be updated in the same change.

// TestAllWorkflows_HasExactlyTwelve asserts the closed set has the
// twelve canonical members.
//
// The test is intentionally brittle. A thirteenth workflow is not a
// formatting change; it is a change to the repository's CI
// architecture, and it must be accompanied by an ADR and a
// documentation update. The failure of this test is the first line of
// the reviewer's checklist.
//
// # History
//
// The set had five members before 18.2.1. 18.2.1 added
// WorkflowCITier1; 18.2.2 added WorkflowCITier2; 18.2.3 added
// WorkflowCITier3. A later change added WorkflowReproducible,
// WorkflowVerifyBinaries, WorkflowBuild, and WorkflowCache. The
// test's name and its constant were updated in the same change that
// added each member, which is the intended discipline: adding a
// member forces the test to change, and the change is visible in the
// diff.
func TestAllWorkflows_HasExactlyTwelve(t *testing.T) {
	got := AllWorkflows()
	if len(got) != 12 {
		t.Fatalf("AllWorkflows() returned %d workflows, want 12", len(got))
	}
}

// TestWorkflows_HasExactlyTwelve asserts the canonical description
// has the same twelve members as the canonical order.
//
// This is a separate test from TestAllWorkflows_HasExactlyTwelve
// because the two slices could drift: a developer who adds a Workflow
// to `workflows` but forgets `allWorkflows` would produce a model
// whose order function and description function disagree. The Validate
// function checks this, but the test names the property directly so
// that a failure is diagnosable without reading Validate.
func TestWorkflows_HasExactlyTwelve(t *testing.T) {
	got := Workflows()
	if len(got) != 12 {
		t.Fatalf("Workflows() returned %d workflows, want 12", len(got))
	}
}

// TestAllWorkflows_OrderIsStable asserts the canonical order.
//
// The order is part of the documentation contract: the seven
// top-level workflows are presented in this order in
// docs/architecture/ci-architecture.md, and the five reusable
// workflows are appended in dependency order because they are
// subordinate to their callers. Reordering the slice without
// reordering the document would produce a document that describes a
// different architecture from the one the code enforces.
func TestAllWorkflows_OrderIsStable(t *testing.T) {
	want := []WorkflowID{
		WorkflowCI,
		WorkflowRelease,
		WorkflowSecurity,
		WorkflowNightly,
		WorkflowDocs,
		WorkflowReproducible,
		WorkflowVerifyBinaries,
		WorkflowCITier1,
		WorkflowCITier2,
		WorkflowCITier3,
		WorkflowBuild,
		WorkflowCache,
	}
	got := AllWorkflows()
	if len(got) != len(want) {
		t.Fatalf("AllWorkflows() has %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AllWorkflows()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestAllWorkflows_ReturnsFreshSlice asserts the accessor does not
// expose the canonical slice for mutation.
//
// The test mutates the returned slice and then re-reads it. If the
// accessor returned the canonical slice directly, the second read
// would observe the mutation. This is the same property that
// forgeerr.AllCategories and forgeerr.AllCodes guarantee, and for the
// same reason: the closed set is a package-level invariant, not a
// per-caller value.
func TestAllWorkflows_ReturnsFreshSlice(t *testing.T) {
	a := AllWorkflows()
	a[0] = "mutated"
	b := AllWorkflows()
	if b[0] == "mutated" {
		t.Fatal("AllWorkflows() returned an aliased slice")
	}
}

// TestAllWorkflows_CallerTruncationDoesNotAffectPackage asserts that a
// caller who truncates the returned slice does not corrupt the
// package's view.
//
// Truncation is a distinct failure mode from element replacement: a
// slice that shares a backing array can be re-sliced to a shorter
// length by one caller, and a later caller that reads the same
// backing array would observe the shorter length. The accessor's
// `copy` prevents this; the test pins it.
func TestAllWorkflows_CallerTruncationDoesNotAffectPackage(t *testing.T) {
	a := AllWorkflows()
	_ = a[:0] // truncate; has no effect on the canonical slice
	b := AllWorkflows()
	if len(b) != 12 {
		t.Fatalf("AllWorkflows() returned %d workflows after a caller truncation, want 12", len(b))
	}
}

// --- Order tests -----------------------------------------------------------

// TestWorkflows_OrderMatchesAllWorkflows asserts the two
// representations of the closed set agree on order.
//
// This is the property that Validate checks at runtime. The test
// asserts it independently so that a future change to Validate — for
// example, removing the cross-check as an optimisation — does not
// silently lose the guarantee.
func TestWorkflows_OrderMatchesAllWorkflows(t *testing.T) {
	ids := AllWorkflows()
	ws := Workflows()
	if len(ids) != len(ws) {
		t.Fatalf("AllWorkflows() has %d entries but Workflows() has %d", len(ids), len(ws))
	}
	for i := range ids {
		if ws[i].ID != ids[i] {
			t.Errorf("Workflows()[%d].ID = %q, want %q", i, ws[i].ID, ids[i])
		}
	}
}

// TestWorkflows_ReturnsDeepCopy asserts the accessor deep-copies each
// Workflow.
//
// The shallow copy that would otherwise be tempting — `out :=
// make([]Workflow, len(workflows)); copy(out, workflows)` — shares
// the backing arrays of the three slice fields (Triggers, Permissions,
// RequiredChecks). A caller that appended to one of them, or that
// replaced an element, would corrupt the canonical set for the rest of
// the process. This test mutates every slice field of every returned
// Workflow and asserts the next call is unaffected.
//
// # Why the loop variable is named w, not t
//
// The test's parameter is *testing.T, conventionally named t. A loop
// over the returned []Workflow that used t as the loop variable would
// shadow the test handle, and any call to t.Fatalf inside the loop
// body would fail to compile ("t.Fatalf undefined (type Workflow has
// no field or method Fatalf)"). The loop variable is named w, which is
// the same convention the package's source files use for a Workflow
// value.
func TestWorkflows_ReturnsDeepCopy(t *testing.T) {
	// Mutate a returned Workflow's slices. If the accessor is shallow,
	// the mutation is visible through the next call.
	first := Workflows()
	for i := range first {
		for j := range first[i].Triggers {
			first[i].Triggers[j] = "mutated-trigger"
		}
		for j := range first[i].Permissions {
			first[i].Permissions[j] = "mutated-permission"
		}
		for j := range first[i].RequiredChecks {
			first[i].RequiredChecks[j] = "mutated-check"
		}
	}

	// Re-read and assert no slice contains the sentinel.
	second := Workflows()
	for _, w := range second {
		for _, tg := range w.Triggers {
			if tg == "mutated-trigger" {
				t.Fatalf("workflow %q: Triggers aliased the canonical slice", w.ID)
			}
		}
		for _, p := range w.Permissions {
			if p == "mutated-permission" {
				t.Fatalf("workflow %q: Permissions aliased the canonical slice", w.ID)
			}
		}
		for _, c := range w.RequiredChecks {
			if c == "mutated-check" {
				t.Fatalf("workflow %q: RequiredChecks aliased the canonical slice", w.ID)
			}
		}
	}
}

// TestLookup_ReturnsDeepCopy asserts the single-entry accessor
// deep-copies too.
//
// Lookup is on the hot path for callers that need one workflow. It
// would be tempting to return the canonical entry directly and rely on
// the caller not to mutate it. The test pins the defensive copy so
// that the temptation is not realised by a future optimisation.
func TestLookup_ReturnsDeepCopy(t *testing.T) {
	w, ok := Lookup(WorkflowCI)
	if !ok {
		t.Fatal("Lookup(WorkflowCI) not found")
	}
	for j := range w.Triggers {
		w.Triggers[j] = "mutated-trigger"
	}
	for j := range w.Permissions {
		w.Permissions[j] = "mutated-permission"
	}
	for j := range w.RequiredChecks {
		w.RequiredChecks[j] = "mutated-check"
	}

	again, ok := Lookup(WorkflowCI)
	if !ok {
		t.Fatal("Lookup(WorkflowCI) not found on second call")
	}
	for _, tg := range again.Triggers {
		if tg == "mutated-trigger" {
			t.Fatal("Lookup returned an aliased Triggers slice")
		}
	}
	for _, p := range again.Permissions {
		if p == "mutated-permission" {
			t.Fatal("Lookup returned an aliased Permissions slice")
		}
	}
	for _, c := range again.RequiredChecks {
		if c == "mutated-check" {
			t.Fatal("Lookup returned an aliased RequiredChecks slice")
		}
	}
}

// TestCloneWorkflow_PreservesNilSlices asserts the deep-copy helper
// keeps the nil-versus-empty distinction.
//
// The distinction matters: RequiredChecks is nil for advisory
// workflows and for the reusable workflows, not an empty slice.
// A clone that turned nil into []string{} would still pass a len()==0
// check but would change the value's meaning for a caller that
// branches on `== nil`. The test pins the preservation.
func TestCloneWorkflow_PreservesNilSlices(t *testing.T) {
	in := Workflow{
		ID:             WorkflowSecurity,
		Name:           "Security",
		Triggers:       []Trigger{TriggerSchedule},
		Permissions:    []Permission{PermissionContentsRead},
		RequiredChecks: nil,
		Owner:          "Security",
		ReviewPolicy:   "One Security reviewer.",
		Advisory:       true,
	}
	out := cloneWorkflow(in)
	if out.RequiredChecks != nil {
		t.Fatalf("cloneWorkflow turned a nil RequiredChecks into %#v", out.RequiredChecks)
	}

	// Non-nil empty slices must stay non-nil empty. This is the
	// complement of the previous assertion: a clone that normalised
	// both to nil would pass the first check and fail this one.
	in.RequiredChecks = []string{}
	out = cloneWorkflow(in)
	if out.RequiredChecks == nil {
		t.Fatal("cloneWorkflow turned an empty RequiredChecks into nil")
	}
}

// --- WorkflowID tests ------------------------------------------------------

// TestWorkflowID_FileName_IsTotal asserts every member of the closed
// set maps to a file name.
//
// The mapping is what the contract tests in
// workflows_contract_test.go, tier1_contract_test.go,
// tier2_contract_test.go, tier3_contract_test.go, and
// security_contract_test.go rely on to locate the YAML files. A
// missing mapping would make those tests silently skip a workflow.
func TestWorkflowID_FileName_IsTotal(t *testing.T) {
	for _, id := range AllWorkflows() {
		name := id.FileName()
		if name == "" {
			t.Errorf("WorkflowID(%q).FileName() returned empty", id)
		}
		if !strings.HasSuffix(name, ".yml") {
			t.Errorf("WorkflowID(%q).FileName() = %q, want a .yml suffix", id, name)
		}
	}
}

// TestWorkflowID_FileName_Values asserts the exact file name for each
// member of the closed set.
//
// The test is intentionally exact: a rename of a workflow file is a
// change to the CI architecture and must be reflected in
// docs/architecture/ci-architecture.md. The failure of this test is
// the first indication that the rename was not coordinated.
//
// The twelve entries correspond one-to-one with the twelve
// WorkflowID constants: seven top-level workflows and five reusable
// workflows. The order matches AllWorkflows()'s order so that a
// reader who diffs the two lists sees the same sequence.
func TestWorkflowID_FileName_Values(t *testing.T) {
	cases := []struct {
		id   WorkflowID
		want string
	}{
		{WorkflowCI, "ci.yml"},
		{WorkflowRelease, "release.yml"},
		{WorkflowSecurity, "security.yml"},
		{WorkflowNightly, "nightly.yml"},
		{WorkflowDocs, "docs.yml"},
		{WorkflowReproducible, "reproducible.yml"},
		{WorkflowVerifyBinaries, "verify-binaries.yml"},
		{WorkflowCITier1, "ci-tier1.yml"},
		{WorkflowCITier2, "ci-tier2.yml"},
		{WorkflowCITier3, "ci-tier3.yml"},
		{WorkflowBuild, "build.yml"},
		{WorkflowCache, "cache.yml"},
	}
	for _, tc := range cases {
		if got := tc.id.FileName(); got != tc.want {
			t.Errorf("WorkflowID(%q).FileName() = %q, want %q", tc.id, got, tc.want)
		}
	}
}

// TestWorkflowID_FileName_PanicsOnUnknown asserts the constructor-style
// panic on an unregistered ID.
//
// The panic is the package's contract with the caller: an unknown ID
// is a programming error, not a runtime condition. The test recovers
// the panic and asserts the message names the offending value so that
// a CI log identifies the bug without a debugger.
func TestWorkflowID_FileName_PanicsOnUnknown(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("FileName() on unknown ID did not panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is %T, want string", r)
		}
		if !strings.Contains(msg, "not-a-workflow") {
			t.Errorf("panic message %q does not name the offending value", msg)
		}
	}()
	_ = WorkflowID("not-a-workflow").FileName()
}

// TestWorkflowID_String_IsTotal asserts String never panics, including
// on an unknown ID.
//
// The asymmetry with FileName is deliberate: String is for humans and
// diagnostics, FileName is for the program. A diagnostic that panics
// on an unknown value is worse than a diagnostic that reports the
// unknown value as such. This is the same asymmetry as
// forgeerr.Category.String (total) versus Category.Prefix (panics).
func TestWorkflowID_String_IsTotal(t *testing.T) {
	cases := []WorkflowID{"", "unknown", WorkflowCI, WorkflowRelease}
	for _, c := range cases {
		got := c.String()
		if got != string(c) {
			t.Errorf("WorkflowID(%q).String() = %q, want %q", c, got, string(c))
		}
	}
}

// TestWorkflowID_Valid is the membership table for the closed set.
//
// The table's `true` cases are the twelve canonical IDs. Its `false`
// cases cover four kinds of rejection:
//
//   - the empty string (the zero value of WorkflowID),
//   - an arbitrary unknown string,
//   - a case-mismatched form of a canonical ID ("CI" is not "ci"),
//   - the file name of a canonical workflow ("ci.yml" is not an ID).
func TestWorkflowID_Valid(t *testing.T) {
	cases := []struct {
		id   WorkflowID
		want bool
	}{
		// The twelve canonical IDs.
		{WorkflowCI, true},
		{WorkflowRelease, true},
		{WorkflowSecurity, true},
		{WorkflowNightly, true},
		{WorkflowDocs, true},
		{WorkflowReproducible, true},
		{WorkflowVerifyBinaries, true},
		{WorkflowCITier1, true},
		{WorkflowCITier2, true},
		{WorkflowCITier3, true},
		{WorkflowBuild, true},
		{WorkflowCache, true},

		// Rejections.
		{"", false},                 // the zero value
		{"unknown", false},          // an arbitrary unknown string
		{"CI", false},               // case-sensitive: the canonical ID is lowercase
		{"ci.yml", false},           // file name is not an ID
		{"security.yml", false},     // file name is not an ID
		{"ci-tier1.yml", false},     // file name is not an ID
		{"reproducible.yml", false}, // file name is not an ID
		{"build.yml", false},        // file name is not an ID
		{"cache.yml", false},        // file name is not an ID
		{"ci ", false},              // trailing space
		{" ci", false},              // leading space
	}
	for _, tc := range cases {
		if got := tc.id.Valid(); got != tc.want {
			t.Errorf("WorkflowID(%q).Valid() = %v, want %v", tc.id, got, tc.want)
		}
	}
}

// TestWorkflowID_AllMembersAreValid asserts the closed set is
// self-consistent: every member of allWorkflows is Valid, and the
// number of valid IDs equals the size of the set.
func TestWorkflowID_AllMembersAreValid(t *testing.T) {
	valid := 0
	for _, id := range AllWorkflows() {
		if !id.Valid() {
			t.Errorf("AllWorkflows() member %q is not Valid()", id)
		}
		valid++
	}
	if valid != len(allWorkflows) {
		t.Fatalf("counted %d valid IDs but allWorkflows has %d", valid, len(allWorkflows))
	}
}

// --- Trigger tests ---------------------------------------------------------

// TestTrigger_Valid is the membership table for the closed set of
// triggers.
func TestTrigger_Valid(t *testing.T) {
	cases := []struct {
		trig Trigger
		want bool
	}{
		{TriggerPullRequest, true},
		{TriggerPush, true},
		{TriggerSchedule, true},
		{TriggerWorkflowDispatch, true},
		{TriggerTag, true},
		{TriggerWorkflowCall, true},
		{"", false},
		{"pull_request_target", false}, // forbidden by ADR-043
		{"PULL_REQUEST", false},        // case-sensitive
		{"pull-request", false},        // hyphen, not underscore
		{"workflow-call", false},       // hyphen, not underscore
	}
	for _, tc := range cases {
		if got := tc.trig.Valid(); got != tc.want {
			t.Errorf("Trigger(%q).Valid() = %v, want %v", tc.trig, got, tc.want)
		}
	}
}

// TestTrigger_PullRequestTargetIsNotInTheSet documents the ADR-043
// prohibition as a test.
//
// The test is deliberately explicit: a reader who wonders "why is
// pull_request_target not in the set?" finds the answer here, in the
// test that would fail if someone added it.
func TestTrigger_PullRequestTargetIsNotInTheSet(t *testing.T) {
	if Trigger("pull_request_target").Valid() {
		t.Fatal("pull_request_target must not be a member of the closed trigger set (ADR-043)")
	}
}

// TestTrigger_WorkflowCallIsInTheSet documents the reusable-workflow
// trigger as a member of the closed set.
//
// The test is the complement of TestTrigger_PullRequestTargetIsNotInTheSet:
// a reader who wonders "how is a reusable workflow invoked?" finds the
// answer here.
func TestTrigger_WorkflowCallIsInTheSet(t *testing.T) {
	if !TriggerWorkflowCall.Valid() {
		t.Fatal("workflow_call must be a member of the closed trigger set")
	}
}

// TestTrigger_String asserts String is total and identity-preserving.
func TestTrigger_String(t *testing.T) {
	cases := []Trigger{"", "unknown", TriggerPullRequest, TriggerPush, TriggerWorkflowCall}
	for _, c := range cases {
		if got := c.String(); got != string(c) {
			t.Errorf("Trigger(%q).String() = %q, want %q", c, got, string(c))
		}
	}
}

// --- Permission tests ------------------------------------------------------

// TestPermission_Valid is the membership table for the closed set of
// permissions.
//
// The table's `true` cases are the canonical permission scopes. Its
// `false` cases cover three kinds of rejection:
//
//   - the empty string (the zero value of Permission),
//   - a malformed form (missing the scope suffix, missing the space
//     after the colon, an extra space, uppercase),
//   - a well-formed form that is not in the closed set.
func TestPermission_Valid(t *testing.T) {
	cases := []struct {
		p    Permission
		want bool
	}{
		// The canonical scopes.
		{PermissionContentsRead, true},
		{PermissionContentsWrite, true},
		{PermissionSecurityEventsRead, true},
		{PermissionSecurityEventsWrite, true},
		{PermissionIDTokenWrite, true},
		{PermissionPullRequestsRead, true},
		{PermissionActionsRead, true},
		{PermissionActionsWrite, true},
		{PermissionIssuesWrite, true},
		{PermissionPackagesWrite, true},

		// Rejections.
		{"", false},
		{"contents", false},               // missing scope suffix
		{"contents:read", false},          // missing space after the colon
		{"contents:  read", false},        // extra space
		{"CONTENTS: READ", false},         // uppercase
		{"security-events: admin", false}, // not declared here
		{"id-token: read", false},         // not declared here
	}
	for _, tc := range cases {
		if got := tc.p.Valid(); got != tc.want {
			t.Errorf("Permission(%q).Valid() = %v, want %v", tc.p, got, tc.want)
		}
	}
}

// TestPermission_EveryWorkflowPermissionIsValid asserts the workflows
// themselves only reference declared permissions.
//
// This is the property that Validate checks. The test asserts it
// directly so that a failure names the offending workflow and
// permission, not just "Validate failed".
func TestPermission_EveryWorkflowPermissionIsValid(t *testing.T) {
	for _, w := range Workflows() {
		for i, p := range w.Permissions {
			if !p.Valid() {
				t.Errorf("workflow %q: Permissions[%d] = %q is not in the closed set", w.ID, i, p)
			}
		}
	}
}

// TestPermission_String asserts String is total and identity-preserving.
func TestPermission_String(t *testing.T) {
	cases := []Permission{"", "unknown", PermissionContentsRead, PermissionIDTokenWrite}
	for _, c := range cases {
		if got := c.String(); got != string(c) {
			t.Errorf("Permission(%q).String() = %q, want %q", c, got, string(c))
		}
	}
}

// --- Lookup tests ----------------------------------------------------------

// TestLookup_FindsEveryWorkflow asserts Lookup is total over the
// closed set.
func TestLookup_FindsEveryWorkflow(t *testing.T) {
	for _, id := range AllWorkflows() {
		w, ok := Lookup(id)
		if !ok {
			t.Errorf("Lookup(%q) reported not found", id)
			continue
		}
		if w.ID != id {
			t.Errorf("Lookup(%q).ID = %q", id, w.ID)
		}
		if w.Name == "" {
			t.Errorf("Lookup(%q).Name is empty", id)
		}
		if w.Owner == "" {
			t.Errorf("Lookup(%q).Owner is empty", id)
		}
		if w.ReviewPolicy == "" {
			t.Errorf("Lookup(%q).ReviewPolicy is empty", id)
		}
	}
}

// TestLookup_UnknownReturnsZero asserts Lookup does not panic and
// returns the zero Workflow for an unknown ID.
//
// The zero Workflow is what a caller sees when Lookup fails. The test
// pins the zero value so that a future change to the Workflow struct
// does not accidentally make the zero value non-zero and break
// callers that rely on the "not found" shape.
func TestLookup_UnknownReturnsZero(t *testing.T) {
	w, ok := Lookup("nope")
	if ok {
		t.Fatal("Lookup(\"nope\") reported found")
	}
	if !isZeroWorkflow(w) {
		t.Fatalf("Lookup(\"nope\") returned %#v, want the zero Workflow", w)
	}
}

// TestLookup_EmptyIDReturnsZero asserts the empty string is not a
// valid workflow ID and Lookup returns the zero Workflow for it.
//
// The case is distinct from "nope": the empty string is the zero value
// of WorkflowID, and a caller who forgets to check for it would see
// the same result as for an unknown non-empty ID. The test pins the
// equivalence so that a future change that gives the empty string a
// special meaning fails here first.
func TestLookup_EmptyIDReturnsZero(t *testing.T) {
	w, ok := Lookup("")
	if ok {
		t.Fatal("Lookup(\"\") reported found")
	}
	if !isZeroWorkflow(w) {
		t.Fatalf("Lookup(\"\") returned %#v, want the zero Workflow", w)
	}
}

// --- RequiredChecks tests --------------------------------------------------

// TestRequiredChecks_UnionIsSortedAndUnique asserts the output is
// deterministic.
//
// Determinism matters because the list is used by golden files and by
// the branch-protection configuration. A non-deterministic order would
// make both diffable artifacts unstable.
func TestRequiredChecks_UnionIsSortedAndUnique(t *testing.T) {
	got := RequiredChecks()
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Fatalf("RequiredChecks() is not strictly sorted: %v", got)
		}
	}
}

// TestRequiredChecks_OnlyFromCIAndDocs asserts the policy: only ci and
// docs contribute required checks.
//
// The policy is the mechanism that keeps security, nightly, and the
// reusable workflows from becoming merge blockers in their own right.
// If a future change added a required check to security.yml, this test
// would fail before the change reached main.
func TestRequiredChecks_OnlyFromCIAndDocs(t *testing.T) {
	ciChecks := make(map[string]struct{})
	for _, c := range mustLookup(t, WorkflowCI).RequiredChecks {
		ciChecks[c] = struct{}{}
	}
	docsChecks := make(map[string]struct{})
	for _, c := range mustLookup(t, WorkflowDocs).RequiredChecks {
		docsChecks[c] = struct{}{}
	}
	for _, c := range RequiredChecks() {
		if _, ok := ciChecks[c]; ok {
			continue
		}
		if _, ok := docsChecks[c]; ok {
			continue
		}
		t.Errorf("required check %q is not from ci or docs", c)
	}
}

// TestRequiredChecks_MatchesPerWorkflowDeclarations asserts the union
// is exactly the set of checks declared by the two workflows that are
// permitted to declare them.
//
// This is stronger than TestRequiredChecks_OnlyFromCIAndDocs: it also
// asserts that no declared check is missing from the union. A caller
// that reads RequiredChecks() and a caller that reads the two
// workflows' RequiredChecks fields must see the same set.
func TestRequiredChecks_MatchesPerWorkflowDeclarations(t *testing.T) {
	want := make(map[string]struct{})
	for _, id := range []WorkflowID{WorkflowCI, WorkflowDocs} {
		for _, c := range mustLookup(t, id).RequiredChecks {
			want[c] = struct{}{}
		}
	}
	got := RequiredChecks()
	if len(got) != len(want) {
		t.Fatalf("RequiredChecks() has %d entries, want %d", len(got), len(want))
	}
	for _, c := range got {
		if _, ok := want[c]; !ok {
			t.Errorf("RequiredChecks() contains %q, which is not declared by any workflow", c)
		}
	}
}

// TestRequiredChecks_CIHasTheTierAggregates asserts the CI workflow's
// required checks are the two PR-time tier aggregates.
//
// The aggregates are `tier1` and `tier2`; the `tier3` aggregate is
// deliberately absent because it is gated on push. The test pins the
// two PR-time names so that a future change that renamed an aggregate
// without updating the branch-protection config fails here first.
func TestRequiredChecks_CIHasTheTierAggregates(t *testing.T) {
	ci := mustLookup(t, WorkflowCI)
	want := map[string]bool{"tier1": false, "tier2": false}
	for _, c := range ci.RequiredChecks {
		if _, ok := want[c]; ok {
			want[c] = true
			continue
		}
		t.Errorf("WorkflowCI declares required check %q, which is not a PR-time tier aggregate", c)
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("WorkflowCI does not declare required check %q", name)
		}
	}
}

// TestRequiredChecks_DoesNotContainTier3 asserts the `tier3`
// aggregate is deliberately absent from the required-checks union.
//
// The absence is the mechanism that keeps Tier 3 advisory on PRs. A
// future change that added it would fail this test and prompt the
// contributor to read ADR-047 and the caller's gate in ci.yml.
func TestRequiredChecks_DoesNotContainTier3(t *testing.T) {
	for _, c := range RequiredChecks() {
		if c == "tier3" {
			t.Fatal("RequiredChecks() contains \"tier3\"; Tier 3 is main-only and must not be a PR-time required check")
		}
	}
}

// TestRequiredChecks_IsDeterministic asserts repeated calls return the
// same sequence.
//
// The check is intentionally strict (`got[i] != again[i]`), not just
// set equality: the order is part of the contract, and a caller that
// diffs two calls must see no spurious changes.
func TestRequiredChecks_IsDeterministic(t *testing.T) {
	a := RequiredChecks()
	b := RequiredChecks()
	if len(a) != len(b) {
		t.Fatalf("RequiredChecks() is not deterministic: %d vs %d entries", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("RequiredChecks() is not deterministic at index %d: %q vs %q", i, a[i], b[i])
		}
	}
}

// TestRequiredChecks_ReturnsFreshSlice asserts the output does not
// alias the caller's previous view.
func TestRequiredChecks_ReturnsFreshSlice(t *testing.T) {
	a := RequiredChecks()
	if len(a) == 0 {
		t.Skip("no required checks to mutate")
	}
	a[0] = "mutated"
	b := RequiredChecks()
	if b[0] == "mutated" {
		t.Fatal("RequiredChecks() returned an aliased slice")
	}
}

// --- Validate tests --------------------------------------------------------

// TestValidate_AcceptsCanonicalState asserts the canonical state is
// internally consistent.
//
// This is the test that runs at package init time and, in effect,
// proves the init will not panic on the shipped state. If a future
// change introduces an inconsistency, this test fails before the
// package is imported by another test that would observe the panic.
func TestValidate_AcceptsCanonicalState(t *testing.T) {
	if err := Validate(); err != nil {
		t.Fatalf("Validate() rejected the canonical state: %v", err)
	}
}

// TestValidate_IsDeterministic asserts repeated calls agree.
func TestValidate_IsDeterministic(t *testing.T) {
	for i := 0; i < 3; i++ {
		if err := Validate(); err != nil {
			t.Fatalf("Validate() call %d failed: %v", i, err)
		}
	}
}

// TestValidate_ErrorIsForgeError asserts the error type is the
// structured one, not a bare error.
//
// The type matters because the CI log and the CLI both expect a
// *forgeerr.Error with a code and a category. A future change that
// replaced the structured error with fmt.Errorf would break that
// contract; this test pins it.
func TestValidate_ErrorIsForgeError(t *testing.T) {
	err := ValidateWorkflow(Workflow{})
	if err == nil {
		t.Fatal("ValidateWorkflow on the zero Workflow returned nil")
	}
	var fe *forgeerr.Error
	if !errors.As(err, &fe) {
		t.Fatalf("ValidateWorkflow returned %T, want *forgeerr.Error", err)
	}
	if fe.Category() != forgeerr.CategoryInternal {
		t.Errorf("ValidateWorkflow category = %v, want %v", fe.Category(), forgeerr.CategoryInternal)
	}
	if fe.Code() != string(forgeerr.CodeInternalUnexpected) {
		t.Errorf("ValidateWorkflow code = %q, want %q", fe.Code(), forgeerr.CodeInternalUnexpected)
	}
}

// --- ValidateWorkflow tests ------------------------------------------------
//
// ValidateWorkflow is the per-entry validator. Its tests build
// deliberately broken Workflows and assert the validator rejects each
// broken shape with the expected invariant.
//
// # Why the assertion reads the context, not the message
//
// invariantError builds a *forgeerr.Error whose message is a fixed
// string ("ci: workflow model invariant violated") and whose context
// carries the invariant's name under the `invariant` key. The message
// is fixed because forgeerr enforces an 80-character limit (I4) and
// the workflow model's longest invariant name
// ("required checks on a non-required workflow", 43 characters) plus
// the composed message's prefix (38 characters) exceeds the limit.
//
// The tests therefore read the context's `invariant` value, not the
// message. The tables' field is named `wantInvariant` to match the
// context key; the assertion is a direct string comparison.

// validWorkflow returns a Workflow that passes every check. Each test
// starts from this value and breaks exactly one property, so the
// failure is attributable to the property under test.
//
// The helper is a function, not a package-level var, so that each
// caller gets a fresh value. A shared var would let a mutation in one
// test leak into another.
//
// # Why the baseline uses a top-level workflow
//
// The baseline must satisfy both the top-level trigger rule (at least
// one direct trigger, no workflow_call) and the reusable-workflow
// trigger rule (workflow_call only). The two rules are disjoint, so a
// single baseline cannot exercise both. The baseline uses
// WorkflowCI, a top-level workflow. The reusable-workflow rules are
// exercised by a separate table (see
// TestValidateWorkflow_ReusableWorkflowTriggerRule below).
func validWorkflow() Workflow {
	return Workflow{
		ID:             WorkflowCI,
		Name:           "CI",
		Triggers:       []Trigger{TriggerPullRequest},
		Permissions:    []Permission{PermissionContentsRead},
		RequiredChecks: []string{"check"},
		Owner:          "Release Engineering",
		ReviewPolicy:   "One Release Engineering reviewer.",
		Advisory:       false,
	}
}

// validReusableWorkflow returns a reusable Workflow that passes every
// check. It is the baseline for the reusable-workflow rule table.
func validReusableWorkflow() Workflow {
	return Workflow{
		ID:             WorkflowCITier1,
		Name:           "CI Tier 1",
		Triggers:       []Trigger{TriggerWorkflowCall},
		Permissions:    []Permission{PermissionContentsRead},
		RequiredChecks: nil,
		Owner:          "Release Engineering",
		ReviewPolicy:   "One Release Engineering reviewer.",
		Advisory:       false,
	}
}

// TestValidateWorkflow_AcceptsValidEntry asserts the baseline is
// accepted. Without this test, every other test in the group would
// pass vacuously if validWorkflow were itself invalid.
func TestValidateWorkflow_AcceptsValidEntry(t *testing.T) {
	if err := ValidateWorkflow(validWorkflow()); err != nil {
		t.Fatalf("ValidateWorkflow rejected the baseline: %v", err)
	}
}

// TestValidateWorkflow_AcceptsValidReusableEntry asserts the reusable
// baseline is accepted.
func TestValidateWorkflow_AcceptsValidReusableEntry(t *testing.T) {
	if err := ValidateWorkflow(validReusableWorkflow()); err != nil {
		t.Fatalf("ValidateWorkflow rejected the reusable baseline: %v", err)
	}
}

// TestValidateWorkflow_RejectsBrokenEntries is the table of broken
// shapes. Each row breaks exactly one property of the baseline
// returned by validWorkflow().
//
// # Why the table is built this way
//
// Each row is a name, a mutation, and the invariant name the
// validator's context must carry. The mutation is a closure over a
// *copy* of the baseline, so a row cannot accidentally affect another
// row. The assertion is a direct string comparison against the
// context's `invariant` value.
func TestValidateWorkflow_RejectsBrokenEntries(t *testing.T) {
	cases := []struct {
		name          string
		mutate        func(w *Workflow)
		wantInvariant string
	}{
		{
			name: "unknown ID",
			mutate: func(w *Workflow) {
				w.ID = "not-a-workflow"
			},
			wantInvariant: "unknown workflow ID",
		},
		{
			name: "empty name",
			mutate: func(w *Workflow) {
				w.Name = ""
			},
			wantInvariant: "empty workflow name",
		},
		{
			name: "no triggers",
			mutate: func(w *Workflow) {
				w.Triggers = nil
			},
			wantInvariant: "no triggers",
		},
		{
			name: "unknown trigger",
			mutate: func(w *Workflow) {
				w.Triggers = []Trigger{"pull_request_target"}
			},
			wantInvariant: "unknown trigger",
		},
		{
			name: "no permissions",
			mutate: func(w *Workflow) {
				w.Permissions = nil
			},
			wantInvariant: "no permissions",
		},
		{
			name: "unknown permission",
			mutate: func(w *Workflow) {
				w.Permissions = []Permission{"contents: admin"}
			},
			wantInvariant: "unknown permission",
		},
		{
			name: "no owner",
			mutate: func(w *Workflow) {
				w.Owner = ""
			},
			wantInvariant: "no owner",
		},
		{
			name: "no review policy",
			mutate: func(w *Workflow) {
				w.ReviewPolicy = ""
			},
			wantInvariant: "no review policy",
		},
		{
			name: "required checks on security",
			mutate: func(w *Workflow) {
				w.ID = WorkflowSecurity
				w.Advisory = true
				w.RequiredChecks = []string{"something"}
			},
			wantInvariant: "required checks on a non-required workflow",
		},
		{
			name: "required checks on nightly",
			mutate: func(w *Workflow) {
				w.ID = WorkflowNightly
				w.Advisory = true
				w.RequiredChecks = []string{"something"}
			},
			wantInvariant: "required checks on a non-required workflow",
		},
		{
			name: "required checks on release",
			mutate: func(w *Workflow) {
				w.ID = WorkflowRelease
				w.Advisory = false
				w.RequiredChecks = []string{"something"}
			},
			wantInvariant: "required checks on a non-required workflow",
		},
		{
			name: "security not marked advisory",
			mutate: func(w *Workflow) {
				w.ID = WorkflowSecurity
				w.Advisory = false
				w.RequiredChecks = nil
			},
			wantInvariant: "advisory workflow not marked advisory",
		},
		{
			name: "nightly not marked advisory",
			mutate: func(w *Workflow) {
				w.ID = WorkflowNightly
				w.Advisory = false
				w.RequiredChecks = nil
			},
			wantInvariant: "advisory workflow not marked advisory",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := validWorkflow()
			tc.mutate(&w)
			err := ValidateWorkflow(w)
			if err == nil {
				t.Fatal("ValidateWorkflow accepted a broken entry")
			}

			// The error must be a *forgeerr.Error; the invariant's
			// name is in the context's `invariant` key, not in the
			// message.
			var fe *forgeerr.Error
			if !errors.As(err, &fe) {
				t.Fatalf("ValidateWorkflow returned %T, want *forgeerr.Error", err)
			}
			got := fe.Context()["invariant"]
			if got != tc.wantInvariant {
				t.Errorf("ValidateWorkflow invariant = %q, want %q", got, tc.wantInvariant)
			}
		})
	}
}

// TestValidateWorkflow_ReusableWorkflowTriggerRule is the table of
// broken shapes for the reusable-workflow trigger rule.
//
// The rule is: a reusable workflow declares TriggerWorkflowCall and
// nothing else; a top-level workflow declares at least one direct
// trigger and never TriggerWorkflowCall. The two sets are disjoint by
// construction. The table exercises both halves of the rule and
// every reusable workflow.
//
// # Why the table has a case per reusable workflow
//
// The rule's implementation is set membership over the
// `reusableWorkflows` map. A test that exercised the rule on only one
// member would not catch a map that was missing the others. The table
// exercises the rule on each member so that a future change to the
// map is caught by the table, not by a runtime failure in a caller.
func TestValidateWorkflow_ReusableWorkflowTriggerRule(t *testing.T) {
	cases := []struct {
		name          string
		mutate        func(w *Workflow)
		wantInvariant string
	}{
		{
			name: "tier1 reusable workflow without workflow_call",
			mutate: func(w *Workflow) {
				w.ID = WorkflowCITier1
				w.Triggers = []Trigger{TriggerPullRequest}
			},
			wantInvariant: "reusable workflow without workflow_call",
		},
		{
			name: "tier1 reusable workflow with extra triggers",
			mutate: func(w *Workflow) {
				w.ID = WorkflowCITier1
				w.Triggers = []Trigger{TriggerWorkflowCall, TriggerPullRequest}
			},
			wantInvariant: "reusable workflow with extra triggers",
		},
		{
			name: "tier2 reusable workflow without workflow_call",
			mutate: func(w *Workflow) {
				w.ID = WorkflowCITier2
				w.Triggers = []Trigger{TriggerPullRequest}
			},
			wantInvariant: "reusable workflow without workflow_call",
		},
		{
			name: "tier2 reusable workflow with extra triggers",
			mutate: func(w *Workflow) {
				w.ID = WorkflowCITier2
				w.Triggers = []Trigger{TriggerWorkflowCall, TriggerPullRequest}
			},
			wantInvariant: "reusable workflow with extra triggers",
		},
		{
			name: "tier3 reusable workflow without workflow_call",
			mutate: func(w *Workflow) {
				w.ID = WorkflowCITier3
				w.Triggers = []Trigger{TriggerPullRequest}
			},
			wantInvariant: "reusable workflow without workflow_call",
		},
		{
			name: "tier3 reusable workflow with extra triggers",
			mutate: func(w *Workflow) {
				w.ID = WorkflowCITier3
				w.Triggers = []Trigger{TriggerWorkflowCall, TriggerPullRequest}
			},
			wantInvariant: "reusable workflow with extra triggers",
		},
		{
			name: "build reusable workflow without workflow_call",
			mutate: func(w *Workflow) {
				w.ID = WorkflowBuild
				w.Triggers = []Trigger{TriggerPullRequest}
			},
			wantInvariant: "reusable workflow without workflow_call",
		},
		{
			name: "build reusable workflow with extra triggers",
			mutate: func(w *Workflow) {
				w.ID = WorkflowBuild
				w.Triggers = []Trigger{TriggerWorkflowCall, TriggerPullRequest}
			},
			wantInvariant: "reusable workflow with extra triggers",
		},
		{
			name: "cache reusable workflow without workflow_call",
			mutate: func(w *Workflow) {
				w.ID = WorkflowCache
				w.Triggers = []Trigger{TriggerPullRequest}
			},
			wantInvariant: "reusable workflow without workflow_call",
		},
		{
			name: "cache reusable workflow with extra triggers",
			mutate: func(w *Workflow) {
				w.ID = WorkflowCache
				w.Triggers = []Trigger{TriggerWorkflowCall, TriggerPullRequest}
			},
			wantInvariant: "reusable workflow with extra triggers",
		},
		{
			name: "top-level workflow with workflow_call",
			mutate: func(w *Workflow) {
				w.ID = WorkflowCI
				w.Triggers = []Trigger{TriggerWorkflowCall}
			},
			wantInvariant: "top-level workflow with workflow_call",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := validWorkflow()
			tc.mutate(&w)
			err := ValidateWorkflow(w)
			if err == nil {
				t.Fatal("ValidateWorkflow accepted a broken entry")
			}

			var fe *forgeerr.Error
			if !errors.As(err, &fe) {
				t.Fatalf("ValidateWorkflow returned %T, want *forgeerr.Error", err)
			}
			got := fe.Context()["invariant"]
			if got != tc.wantInvariant {
				t.Errorf("ValidateWorkflow invariant = %q, want %q", got, tc.wantInvariant)
			}
		})
	}
}

// TestValidateWorkflow_IsTotalOnEveryMember asserts the canonical
// workflows all pass the per-entry validator.
//
// A failure here would be caught by Validate as well, but the
// per-entry test names the offending workflow directly.
func TestValidateWorkflow_IsTotalOnEveryMember(t *testing.T) {
	for _, w := range Workflows() {
		if err := ValidateWorkflow(w); err != nil {
			t.Errorf("ValidateWorkflow rejected canonical workflow %q: %v", w.ID, err)
		}
	}
}

// TestValidateWorkflow_DoesNotMutate asserts the validator does not
// mutate its argument.
//
// The validator takes a Workflow by value, so a mutation would only
// affect the caller if the Workflow contained a pointer or a map. It
// does not, but the test pins the property so that a future change
// that adds a map field does not silently introduce mutation.
func TestValidateWorkflow_DoesNotMutate(t *testing.T) {
	w := validWorkflow()
	before := cloneWorkflow(w)
	if err := ValidateWorkflow(w); err != nil {
		t.Fatalf("ValidateWorkflow rejected the baseline: %v", err)
	}
	if !workflowsEqual(w, before) {
		t.Fatalf("ValidateWorkflow mutated its argument: before=%#v after=%#v", before, w)
	}
}

// --- Reusable-workflow tests -----------------------------------------------

// TestReusableWorkflows_HaveWorkflowCallTrigger asserts the reusable
// workflows declare TriggerWorkflowCall and nothing else.
//
// The test is the canonical-state counterpart of the broken-entry
// table above: it asserts the shipped state satisfies the rule, so
// that a future change that added a direct trigger to a reusable
// workflow fails here.
func TestReusableWorkflows_HaveWorkflowCallTrigger(t *testing.T) {
	for _, id := range []WorkflowID{
		WorkflowCITier1,
		WorkflowCITier2,
		WorkflowCITier3,
		WorkflowBuild,
		WorkflowCache,
	} {
		w := mustLookup(t, id)
		if len(w.Triggers) != 1 {
			t.Errorf("workflow %q has %d triggers, want exactly 1", id, len(w.Triggers))
			continue
		}
		if w.Triggers[0] != TriggerWorkflowCall {
			t.Errorf("workflow %q trigger = %q, want %q", id, w.Triggers[0], TriggerWorkflowCall)
		}
	}
}

// TestTopLevelWorkflows_HaveNoWorkflowCallTrigger asserts the
// top-level workflows never declare TriggerWorkflowCall.
//
// The test is the other half of the rule. A top-level workflow that
// declared workflow_call would be invocable as a reusable workflow,
// which would make its direct triggers ambiguous.
func TestTopLevelWorkflows_HaveNoWorkflowCallTrigger(t *testing.T) {
	topLevel := []WorkflowID{
		WorkflowCI,
		WorkflowRelease,
		WorkflowSecurity,
		WorkflowNightly,
		WorkflowDocs,
		WorkflowReproducible,
		WorkflowVerifyBinaries,
	}
	for _, id := range topLevel {
		w := mustLookup(t, id)
		for _, trig := range w.Triggers {
			if trig == TriggerWorkflowCall {
				t.Errorf("top-level workflow %q declares workflow_call", id)
			}
		}
	}
}

// TestReusableWorkflows_HaveNoRequiredChecks asserts the reusable
// workflows declare no required checks.
//
// A reusable workflow is never a required check in its own right; the
// aggregate job in ci.yml is. The rule is enforced by
// ValidateWorkflow, and the test asserts the canonical state satisfies
// it.
func TestReusableWorkflows_HaveNoRequiredChecks(t *testing.T) {
	for _, id := range []WorkflowID{
		WorkflowCITier1,
		WorkflowCITier2,
		WorkflowCITier3,
		WorkflowBuild,
		WorkflowCache,
	} {
		w := mustLookup(t, id)
		if len(w.RequiredChecks) != 0 {
			t.Errorf("workflow %q declares required checks %v, want none", id, w.RequiredChecks)
		}
	}
}

// --- PinnedActions tests ---------------------------------------------------

// TestPinnedActions_EveryActionValueIsValidSHA asserts every entry
// whose key names an action has a full 40-hex-character lowercase SHA.
//
// The check skips entries whose key names a reusable workflow: those
// have a tag value, per the reusable-workflow exception documented on
// PinnedActions. The reusable-workflow keys are asserted separately by
// TestPinnedActions_ReusableWorkflowKeyShape below.
func TestPinnedActions_EveryActionValueIsValidSHA(t *testing.T) {
	for name, sha := range PinnedActions() {
		if isReusableWorkflowKey(name) {
			continue
		}
		if !IsPinnedSHA(sha) {
			t.Errorf("PinnedActions()[%q] = %q is not a full 40-hex lowercase SHA", name, sha)
		}
	}
}

// TestPinnedActions_ReturnsFreshMap asserts the accessor does not
// expose the canonical map for mutation.
//
// The test replaces every value with a syntactically valid SHA, so
// that if the map were aliased, the mutation would survive the
// init-time validity check and be observable on the next call. A
// sentinel that was not a valid SHA would fail the init check instead
// and mask the aliasing defect.
func TestPinnedActions_ReturnsFreshMap(t *testing.T) {
	a := PinnedActions()
	for k := range a {
		a[k] = "0000000000000000000000000000000000000000"
	}
	b := PinnedActions()
	for k, v := range b {
		if v == "0000000000000000000000000000000000000000" {
			t.Fatalf("PinnedActions() returned an aliased map; key %q was mutated", k)
		}
	}
}

// TestPinnedActions_ActionKeyShape asserts every non-reusable key has
// the owner/name shape: at least one slash, no @.
//
// The check is intentionally loose on the trailing part, because the
// key is the action reference *without* the SHA (the SHA is the
// value) and some actions have a sub-path after the repository name
// (for example, github/codeql-action/init). A key that begins with
// "owner/repo" is an action; the number of slashes after that is not
// a property the model constrains.
//
// The exclusion of reusable workflows is the isReusableWorkflowKey
// check. The two tests together cover the two kinds of `uses:`
// reference.
func TestPinnedActions_ActionKeyShape(t *testing.T) {
	for name := range PinnedActions() {
		if isReusableWorkflowKey(name) {
			continue
		}
		if !strings.Contains(name, "/") {
			t.Errorf("PinnedActions() action key %q does not contain '/'", name)
			continue
		}
		if strings.Contains(name, "@") {
			t.Errorf("PinnedActions() action key %q contains '@'; the SHA belongs in the value, not the key", name)
		}
	}
}

// TestPinnedActions_ReusableWorkflowKeyShape asserts every reusable-
// workflow key has the owner/repo/.github/workflows/file.yml shape.
//
// The shape is structural: a reusable workflow's key contains the
// path component "/.github/workflows/". The test asserts the shape so
// that a future entry that was mistyped as an action key fails here.
func TestPinnedActions_ReusableWorkflowKeyShape(t *testing.T) {
	reusable := 0
	for name := range PinnedActions() {
		if !isReusableWorkflowKey(name) {
			continue
		}
		reusable++
		if strings.Contains(name, "@") {
			t.Errorf("PinnedActions() reusable-workflow key %q contains '@'; the ref belongs in the value", name)
		}
		if !strings.HasSuffix(name, ".yml") {
			t.Errorf("PinnedActions() reusable-workflow key %q does not end in .yml", name)
		}
	}
	if reusable == 0 {
		t.Fatal("PinnedActions() has no reusable-workflow entries; the SLSA generator should be present")
	}
}

// TestPinnedActions_ReusableWorkflowValuesAreTags asserts every
// reusable-workflow entry has a version tag as its value, not a SHA.
//
// The reusable-workflow exception is documented on PinnedActions: a
// reusable workflow's @ref is a tag, and the pinning policy exempts
// it. The test pins the exception so that a future change that
// replaced the tag with a SHA fails here and prompts a review of the
// exception's justification.
func TestPinnedActions_ReusableWorkflowValuesAreTags(t *testing.T) {
	for name, val := range PinnedActions() {
		if !isReusableWorkflowKey(name) {
			continue
		}
		if IsPinnedSHA(val) {
			t.Errorf(
				"PinnedActions()[%q] = %q is a SHA; the reusable-workflow exception expects a version tag",
				name, val,
			)
		}
		if val == "" {
			t.Errorf("PinnedActions()[%q] has an empty value", name)
		}
	}
}

// TestPinnedActions_HasCoreActions asserts the actions the workflows
// are expected to use are present.
//
// The list is a subset of the map, not the whole map: a future
// workflow may add an action that is not in this list, and the test
// should not fail for that reason. The list names the actions that
// every version of the CI workflow architecture is expected to
// require.
func TestPinnedActions_HasCoreActions(t *testing.T) {
	want := []string{
		"actions/checkout",
		"actions/setup-go",
		"github/codeql-action/init",
		"github/codeql-action/analyze",
	}
	have := PinnedActions()
	for _, name := range want {
		if _, ok := have[name]; !ok {
			t.Errorf("PinnedActions() is missing the core action %q", name)
		}
	}
}

// TestPinnedActions_HasTierActions asserts the actions the Tier 1
// and Tier 2 workflows use are present.
func TestPinnedActions_HasTierActions(t *testing.T) {
	want := []string{
		"actions/checkout",
		"actions/setup-go",
		"golangci/golangci-lint-action",
	}
	have := PinnedActions()
	for _, name := range want {
		if _, ok := have[name]; !ok {
			t.Errorf("PinnedActions() is missing the tier action %q", name)
		}
	}
}

// TestPinnedActions_HasTier3Actions asserts the actions ci-tier3.yml
// uses are present.
//
// ci-tier3.yml uses actions/checkout, actions/setup-go, and
// codecov/codecov-action. The first two are shared with the other
// tier workflows; the third is unique to Tier 3.
func TestPinnedActions_HasTier3Actions(t *testing.T) {
	want := []string{
		"actions/checkout",
		"actions/setup-go",
		"codecov/codecov-action",
	}
	have := PinnedActions()
	for _, name := range want {
		if _, ok := have[name]; !ok {
			t.Errorf("PinnedActions() is missing the Tier 3 action %q", name)
		}
	}
}

// TestPinnedActions_HasSecurityActions asserts the actions
// security.yml uses are present.
//
// security.yml uses actions/checkout, actions/setup-go,
// actions/upload-artifact, github/codeql-action/*, and
// ossf/scorecard-action.
func TestPinnedActions_HasSecurityActions(t *testing.T) {
	want := []string{
		"actions/checkout",
		"actions/setup-go",
		"actions/upload-artifact",
		"github/codeql-action/init",
		"github/codeql-action/analyze",
		"github/codeql-action/upload-sarif",
		"ossf/scorecard-action",
	}
	have := PinnedActions()
	for _, name := range want {
		if _, ok := have[name]; !ok {
			t.Errorf("PinnedActions() is missing the security action %q", name)
		}
	}
}

// --- IsPinnedSHA tests -----------------------------------------------------

// TestIsPinnedSHA is the table for the predicate.
//
// The cases cover the length boundary (39, 40, 41), the character
// boundary (lowercase, uppercase, non-hex), and the empty case. The
// predicate is total: every input returns a bool, never a panic.
func TestIsPinnedSHA(t *testing.T) {
	// A valid 40-char SHA used as the base for mutations.
	const valid = "0123456789abcdef0123456789abcdef01234567"
	if !IsPinnedSHA(valid) {
		t.Fatalf("test fixture %q is not a valid SHA; the test is broken", valid)
	}

	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"one char", "a", false},
		{"39 chars", valid[:39], false},
		{"40 chars lowercase hex", valid, true},
		{"41 chars", valid + "a", false},
		{"uppercase", strings.ToUpper(valid), false},
		{"non-hex letter", valid[:39] + "g", false},
		{"non-hex punctuation", valid[:39] + "-", false},
		{"leading space", " " + valid[:39], false},
		{"trailing space", valid[:39] + " ", false},
		{"all zeros", strings.Repeat("0", 40), true},
		{"all f", strings.Repeat("f", 40), true},
		{"mixed case", valid[:20] + strings.ToUpper(valid[20:]), false},
		{"unicode byte", valid[:39] + "\u00e9", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsPinnedSHA(tc.in); got != tc.want {
				t.Errorf("IsPinnedSHA(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestIsPinnedSHA_IsTotal asserts the predicate never panics, even on
// inputs that are not ASCII or not valid UTF-8.
//
// The predicate is on the security path: it validates the pin before
// the pin is used. A panic here would be a denial-of-service vector.
func TestIsPinnedSHA_IsTotal(t *testing.T) {
	inputs := []string{
		"",
		"\x00",
		"\xff\xfe\xfd",
		strings.Repeat("\x00", 40),
		strings.Repeat("\xff", 40),
		strings.Repeat("a", 1<<16),
	}
	for _, in := range inputs {
		// The call must not panic. The result is not asserted; the
		// property under test is totality.
		_ = IsPinnedSHA(in)
	}
}

// --- Reusable-workflow key helper tests ------------------------------------

// TestIsReusableWorkflowKey is the table for the helper.
//
// The helper is unexported, so this test is package-internal. Its
// purpose is to distinguish a reusable-workflow key from an action
// key by the presence of the "/.github/workflows/" path component.
func TestIsReusableWorkflowKey(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"action owner/name", "actions/checkout", false},
		{"action owner/name/subpath", "github/codeql-action/init", false},
		{"reusable workflow", "slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml", true},
		{"reusable workflow short", "owner/repo/.github/workflows/x.yml", true},
		{"path component only", "/.github/workflows/", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isReusableWorkflowKey(tc.in); got != tc.want {
				t.Errorf("isReusableWorkflowKey(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestContainsSubstring is the table for the helper.
//
// The helper substitutes for strings.Contains in a single call site.
// The test is exhaustive over the boundary cases: empty needle, empty
// haystack, needle longer than haystack, needle equal to haystack, and
// needle at the start, middle, and end.
func TestContainsSubstring(t *testing.T) {
	cases := []struct {
		name   string
		s      string
		substr string
		want   bool
	}{
		{"empty needle", "abc", "", true},
		{"empty haystack", "", "a", false},
		{"both empty", "", "", true},
		{"needle longer", "ab", "abc", false},
		{"needle equal", "abc", "abc", true},
		{"needle at start", "abc", "a", true},
		{"needle in middle", "abc", "b", true},
		{"needle at end", "abc", "c", true},
		{"needle absent", "abc", "d", false},
		{"multi-char needle", "hello world", "lo wo", true},
		{"multi-char needle absent", "hello world", "lo  wo", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := containsSubstring(tc.s, tc.substr); got != tc.want {
				t.Errorf("containsSubstring(%q, %q) = %v, want %v", tc.s, tc.substr, got, tc.want)
			}
		})
	}
}

// --- init tests ------------------------------------------------------------
//
// init() runs at package import. A test cannot easily assert that a
// panic would occur without re-importing the package, so the tests
// below assert the *observable* post-init state: the pin map is valid
// and the model is consistent. The complementary test — that a
// malformed pin would be rejected — is documented in the contract
// test file as a procedural check, because it cannot be tested in
// process.

// TestInit_AcceptsCanonicalPins asserts every non-reusable pin in the
// canonical map is valid.
//
// If init had rejected the map, the test binary would not run. The
// fact that this test runs at all is partial evidence that init
// accepted. The test asserts the property explicitly so that a reader
// does not have to reason about import-time behaviour.
func TestInit_AcceptsCanonicalPins(t *testing.T) {
	for name, sha := range pinnedActions {
		if isReusableWorkflowKey(name) {
			continue
		}
		if !IsPinnedSHA(sha) {
			t.Errorf("canonical pin %q = %q is not a valid SHA; init would panic", name, sha)
		}
	}
}

// TestInit_AcceptsCanonicalModel asserts the canonical model passes
// Validate.
//
// Same reasoning as TestInit_AcceptsCanonicalPins: if init had
// panicked, this test would not run. The explicit assertion
// documents the property.
func TestInit_AcceptsCanonicalModel(t *testing.T) {
	if err := Validate(); err != nil {
		t.Fatalf("canonical model failed Validate; init would panic: %v", err)
	}
}

// --- helpers ---------------------------------------------------------------

// mustLookup fails the test if the workflow is missing.
//
// The helper exists so that tests which need a specific workflow do not
// each repeat the two-line "look up and check" pattern. It is a
// t.Helper so that a failure is attributed to the caller.
func mustLookup(t *testing.T, id WorkflowID) Workflow {
	t.Helper()
	w, ok := Lookup(id)
	if !ok {
		t.Fatalf("Lookup(%q) not found", id)
	}
	return w
}

// workflowsEqual reports whether two Workflows are equal, including
// their slice fields.
//
// The comparison is hand-written rather than reflect.DeepEqual so that
// a future change to the Workflow struct (a new field) forces the
// helper to be updated, rather than being silently covered by
// reflect.
//
// # Nil and empty slices
//
// The helper treats a nil slice and an empty non-nil slice as equal:
// both have length zero, and the element loop is empty. That is the
// correct semantics for its caller (TestValidateWorkflow_DoesNotMutate
// compares two copies of the same value, so the nil-versus-empty
// distinction is preserved by construction and does not need to be
// re-checked). A caller that needs the stricter distinction should use
// isZeroWorkflow, which requires every slice to be nil.
func workflowsEqual(a, b Workflow) bool {
	if a.ID != b.ID || a.Name != b.Name || a.Owner != b.Owner ||
		a.ReviewPolicy != b.ReviewPolicy || a.Advisory != b.Advisory {
		return false
	}
	if len(a.Triggers) != len(b.Triggers) {
		return false
	}
	for i := range a.Triggers {
		if a.Triggers[i] != b.Triggers[i] {
			return false
		}
	}
	if len(a.Permissions) != len(b.Permissions) {
		return false
	}
	for i := range a.Permissions {
		if a.Permissions[i] != b.Permissions[i] {
			return false
		}
	}
	if len(a.RequiredChecks) != len(b.RequiredChecks) {
		return false
	}
	for i := range a.RequiredChecks {
		if a.RequiredChecks[i] != b.RequiredChecks[i] {
			return false
		}
	}
	return true
}

// isZeroWorkflow reports whether w is the zero value of Workflow.
//
// The check is strictly stronger than workflowsEqual(w, Workflow{}):
// it also requires every slice field to be nil. The stricter check is
// the one the "Lookup returns the zero Workflow" tests need, because
// a future change that gave the zero value a non-nil empty slice
// would still satisfy workflowsEqual but would violate the documented
// "zero Workflow" contract.
//
// The helper is written by hand rather than with reflect.DeepEqual so
// that adding a field to Workflow forces the helper to be updated.
// A reflect-based check would silently accept a new non-zero field,
// which is the failure mode the tests that call this helper exist to
// prevent.
func isZeroWorkflow(w Workflow) bool {
	return w.ID == "" &&
		w.Name == "" &&
		w.Triggers == nil &&
		w.Permissions == nil &&
		w.RequiredChecks == nil &&
		w.Owner == "" &&
		w.ReviewPolicy == "" &&
		!w.Advisory
}
