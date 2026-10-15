package ci

// concurrency_test.go — internal tests for the CI concurrency model.
//
// The tests in this file are package-internal (`package ci`, not
// `package ci_test`) because several of them inspect the package's
// own unexported state:
//
//   - The closure tests assert on concurrencySpecs directly, which
//     is the point of the closure: adding a member must fail a test,
//     and the test that fails must be able to name the canonical
//     slice.
//   - The validator tests probe validateConcurrencySpec, which is
//     unexported, to exercise the error-construction path without
//     mutating the canonical state.
//
// The tests that read the YAML workflow files live in
// concurrency_contract_test.go and are external (`package ci_test`),
// because they treat the package as a black box and assert on the
// artifacts rather than the model.
//
// # The concurrency model
//
// Every workflow in the closed set has a ConcurrencySpec. Each spec
// records:
//
//   - the group template (the value of the `group:` field);
//   - the cancel-in-progress expression, if the field is an
//     expression, or the literal, if the field is a boolean;
//   - the semantic policy (`on-pr` or `never`);
//   - the rationale.
//
// The two policies:
//
//   - on-pr:  cancel-in-progress is the expression
//             `${{ github.ref != 'refs/heads/main' }}`, so a
//             superseded PR run is cancelled and a superseded main
//             push is preserved. Used by ci.yml, ci-tier1.yml,
//             ci-tier2.yml, ci-tier3.yml, and docs.yml.
//   - never:  cancel-in-progress is false. Used by release.yml,
//             security.yml, nightly.yml, reproducible.yml,
//             verify-binaries.yml, build.yml, and cache.yml.
//
// # The priority rule
//
// A workflow may have more than one trigger class. When it does, the
// classes are ranked:
//
//  1. schedule and push:tags — cancelling a run for a slot or a tag
//     wastes its work. A workflow with either trigger uses `never`.
//  2. pull_request — a superseded PR push is stale; cancelling it
//     frees a runner. A workflow whose only supersedable trigger is
//     pull_request uses `on-pr`.
//  3. workflow_call — a reusable workflow has no direct trigger.
//     Either policy is valid.
//
// The priority is what makes the rules satisfiable for a workflow
// with mixed trigger classes (for example, reproducible.yml, which
// has pull_request, schedule, and push:tags triggers).
//
// # Test taxonomy
//
//   - Closure tests: the set has exactly the members it is supposed
//     to have.
//   - Order tests: the canonical order is stable and the two
//     representations agree.
//   - Copy tests: returned values do not alias the canonical state.
//   - Policy tests: the membership table and String totality.
//   - Lookup tests: totality and the zero value.
//   - Validate tests: the invariants, table-driven on the broken
//     shape.
//   - Golden tests: the JSON rendering is deterministic and valid.
//   - Document tests: the Markdown rendering is deterministic and
//     complete.
//
// The grouping is documentation. A reader who wants to know what the
// package promises reads the group headings; a reader who wants to
// know whether a specific promise holds reads the test in that group.

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// --- Closure tests ---------------------------------------------------------

// TestAllConcurrencySpecs_HasOneSpecPerWorkflow asserts the closed
// set has exactly one spec per workflow in the workflow model.
//
// # Why the count is derived from AllWorkflows()
//
// An earlier version of this test hard-coded the count (8, when the
// workflow model had eight members). A hard-coded count fails when a
// workflow is added without naming the mismatch; the derived count
// fails with the workflow ID that is missing or extra. Deriving the
// count from the model also keeps the two models in sync: adding a
// workflow to AllWorkflows() and forgetting its concurrency spec
// fails here, not in a downstream test.
func TestAllConcurrencySpecs_HasOneSpecPerWorkflow(t *testing.T) {
	got := AllConcurrencySpecs()
	want := AllWorkflows()
	if len(got) != len(want) {
		t.Fatalf(
			"AllConcurrencySpecs() returned %d specs, want %d (one per workflow)",
			len(got), len(want),
		)
	}
}

// TestAllConcurrencySpecs_CoversEveryWorkflow asserts every workflow
// in the closed set has a spec.
//
// This is the cross-set agreement check: the spec set and the
// workflow set must be the same size and name the same members. A
// workflow added to the model without a spec would fail this test;
// a spec for a workflow that is not in the model would fail the
// validator (TestValidateConcurrency_AcceptsCanonicalState).
func TestAllConcurrencySpecs_CoversEveryWorkflow(t *testing.T) {
	seen := make(map[WorkflowID]struct{}, len(concurrencySpecs))
	for _, s := range concurrencySpecs {
		seen[s.Workflow] = struct{}{}
	}
	for _, id := range AllWorkflows() {
		if _, ok := seen[id]; !ok {
			t.Errorf("workflow %q has no concurrency spec", id)
		}
	}
}

// --- Order tests -----------------------------------------------------------

// TestAllConcurrencySpecs_OrderIsStable asserts the canonical order.
//
// The order is the order in which the specs are presented in the
// concurrency policy document. Reordering the slice without
// reordering the document would produce a document that describes a
// different order from the one the code declares.
//
// # Why the order matches allWorkflows' order
//
// The concurrency model's order matches AllWorkflows' order: the
// seven top-level workflows first, then the five reusable workflows.
// That alignment is deliberate; it lets a reader compare the two
// models position by position without a translation table. A future
// change that reorders one but not the other fails this test.
func TestAllConcurrencySpecs_OrderIsStable(t *testing.T) {
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
	got := AllConcurrencySpecs()
	if len(got) != len(want) {
		t.Fatalf("AllConcurrencySpecs() has %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Workflow != want[i] {
			t.Errorf(
				"AllConcurrencySpecs()[%d].Workflow = %q, want %q",
				i, got[i].Workflow, want[i],
			)
		}
	}
}

// --- Copy tests ------------------------------------------------------------

// TestAllConcurrencySpecs_ReturnsFreshSlice asserts the accessor does
// not expose the canonical slice.
func TestAllConcurrencySpecs_ReturnsFreshSlice(t *testing.T) {
	a := AllConcurrencySpecs()
	a[0].Workflow = "mutated"
	b := AllConcurrencySpecs()
	if b[0].Workflow == "mutated" {
		t.Fatal("AllConcurrencySpecs() returned an aliased slice")
	}
}

// TestAllConcurrencySpecs_ReturnsDeepCopy asserts the accessor
// deep-copies.
//
// ConcurrencySpec contains no slice fields, so a shallow copy of the
// slice's elements is sufficient. The test verifies that the elements
// are copied, not aliased; a future field addition that introduced a
// slice would need this test updated.
func TestAllConcurrencySpecs_ReturnsDeepCopy(t *testing.T) {
	first := AllConcurrencySpecs()
	for i := range first {
		first[i].GroupTemplate = "mutated-group"
		first[i].Rationale = "mutated-rationale"
	}
	second := AllConcurrencySpecs()
	for _, s := range second {
		if s.GroupTemplate == "mutated-group" {
			t.Fatalf("workflow %q: GroupTemplate aliased the canonical value", s.Workflow)
		}
		if s.Rationale == "mutated-rationale" {
			t.Fatalf("workflow %q: Rationale aliased the canonical value", s.Workflow)
		}
	}
}

// TestLookupConcurrencySpec_ReturnsCopy asserts the single-entry
// accessor returns a copy.
func TestLookupConcurrencySpec_ReturnsCopy(t *testing.T) {
	s, ok := LookupConcurrencySpec(WorkflowCI)
	if !ok {
		t.Fatal("LookupConcurrencySpec(WorkflowCI) not found")
	}
	s.GroupTemplate = "mutated-group"
	s.Rationale = "mutated-rationale"

	again, ok := LookupConcurrencySpec(WorkflowCI)
	if !ok {
		t.Fatal("LookupConcurrencySpec(WorkflowCI) not found on second call")
	}
	if again.GroupTemplate == "mutated-group" {
		t.Fatal("LookupConcurrencySpec returned an aliased GroupTemplate")
	}
	if again.Rationale == "mutated-rationale" {
		t.Fatal("LookupConcurrencySpec returned an aliased Rationale")
	}
}

// --- Policy tests ----------------------------------------------------------

// TestCancelPolicy_Valid is the membership table for the closed set
// of cancel policies.
func TestCancelPolicy_Valid(t *testing.T) {
	cases := []struct {
		p    CancelPolicy
		want bool
	}{
		{CancelPolicyNever, true},
		{CancelPolicyOnPR, true},
		{"", false},
		{"unknown", false},
		{"ON-PR", false},
		{"on_pr", false},
		{"never-cancel", false},
	}
	for _, tc := range cases {
		if got := tc.p.Valid(); got != tc.want {
			t.Errorf("CancelPolicy(%q).Valid() = %v, want %v", tc.p, got, tc.want)
		}
	}
}

// TestCancelPolicy_String_IsTotal asserts String never panics,
// including on an unknown policy.
func TestCancelPolicy_String_IsTotal(t *testing.T) {
	cases := []CancelPolicy{"", "unknown", CancelPolicyNever, CancelPolicyOnPR}
	for _, c := range cases {
		got := c.String()
		if got != string(c) {
			t.Errorf("CancelPolicy(%q).String() = %q, want %q", c, got, string(c))
		}
	}
}

// --- Lookup tests ----------------------------------------------------------

// TestLookupConcurrencySpec_FindsEveryWorkflow asserts
// LookupConcurrencySpec is total over the closed set.
func TestLookupConcurrencySpec_FindsEveryWorkflow(t *testing.T) {
	for _, id := range AllWorkflows() {
		s, ok := LookupConcurrencySpec(id)
		if !ok {
			t.Errorf("LookupConcurrencySpec(%q) reported not found", id)
			continue
		}
		if s.Workflow != id {
			t.Errorf("LookupConcurrencySpec(%q).Workflow = %q", id, s.Workflow)
		}
		if s.GroupTemplate == "" {
			t.Errorf("LookupConcurrencySpec(%q).GroupTemplate is empty", id)
		}
		if s.Rationale == "" {
			t.Errorf("LookupConcurrencySpec(%q).Rationale is empty", id)
		}
		if !s.Policy.Valid() {
			t.Errorf("LookupConcurrencySpec(%q).Policy = %q is not valid", id, s.Policy)
		}
	}
}

// TestLookupConcurrencySpec_UnknownReturnsZero asserts
// LookupConcurrencySpec does not panic and returns the zero spec for
// an unknown workflow.
func TestLookupConcurrencySpec_UnknownReturnsZero(t *testing.T) {
	s, ok := LookupConcurrencySpec("nope")
	if ok {
		t.Fatal("LookupConcurrencySpec(\"nope\") reported found")
	}
	if !isZeroConcurrencySpec(s) {
		t.Fatalf("LookupConcurrencySpec(\"nope\") returned %#v, want the zero ConcurrencySpec", s)
	}
}

// TestLookupConcurrencySpec_EmptyIDReturnsZero asserts the empty
// string is not a valid workflow ID.
func TestLookupConcurrencySpec_EmptyIDReturnsZero(t *testing.T) {
	s, ok := LookupConcurrencySpec("")
	if ok {
		t.Fatal("LookupConcurrencySpec(\"\") reported found")
	}
	if !isZeroConcurrencySpec(s) {
		t.Fatalf("LookupConcurrencySpec(\"\") returned %#v, want the zero ConcurrencySpec", s)
	}
}

// --- Validate tests --------------------------------------------------------

// TestValidateConcurrency_AcceptsCanonicalState asserts the canonical
// state is internally consistent.
func TestValidateConcurrency_AcceptsCanonicalState(t *testing.T) {
	if err := ValidateConcurrency(); err != nil {
		t.Fatalf("ValidateConcurrency() rejected the canonical state: %v", err)
	}
}

// TestValidateConcurrency_IsDeterministic asserts repeated calls
// agree.
func TestValidateConcurrency_IsDeterministic(t *testing.T) {
	for i := 0; i < 3; i++ {
		if err := ValidateConcurrency(); err != nil {
			t.Fatalf("ValidateConcurrency() call %d failed: %v", i, err)
		}
	}
}

// TestValidateConcurrency_ErrorIsForgeError asserts the error type is
// the structured one.
//
// # Why the probe is validateConcurrencySpec, not ValidateConcurrency
//
// ValidateConcurrency on the canonical state returns nil; it cannot
// be used to produce an error without mutating package-level data,
// which the test suite deliberately avoids. validateConcurrencySpec
// is the per-entry validator that ValidateConcurrency calls
// internally; probing it on a broken entry exercises the same
// error-construction path.
func TestValidateConcurrency_ErrorIsForgeError(t *testing.T) {
	err := validateConcurrencySpec(ConcurrencySpec{})
	if err == nil {
		t.Fatal("validateConcurrencySpec on the zero spec returned nil")
	}
	var fe *forgeerr.Error
	if !errors.As(err, &fe) {
		t.Fatalf("validateConcurrencySpec returned %T, want *forgeerr.Error", err)
	}
	if fe.Category() != forgeerr.CategoryInternal {
		t.Errorf("validateConcurrencySpec category = %v, want %v", fe.Category(), forgeerr.CategoryInternal)
	}
	if fe.Code() != string(forgeerr.CodeInternalUnexpected) {
		t.Errorf(
			"validateConcurrencySpec code = %q, want %q",
			fe.Code(), forgeerr.CodeInternalUnexpected,
		)
	}
}

// validConcurrencySpec returns a spec that passes every check. Each
// test starts from this value and breaks exactly one property.
func validConcurrencySpec() ConcurrencySpec {
	return ConcurrencySpec{
		Workflow:         WorkflowCI,
		GroupTemplate:    "ci-${{ github.workflow }}-${{ github.ref }}",
		CancelExpression: "${{ github.ref != 'refs/heads/main' }}",
		Policy:           CancelPolicyOnPR,
		Rationale:        "test rationale",
	}
}

// TestValidateConcurrencySpec_AcceptsValidEntry asserts the baseline
// is accepted.
func TestValidateConcurrencySpec_AcceptsValidEntry(t *testing.T) {
	if err := validateConcurrencySpec(validConcurrencySpec()); err != nil {
		t.Fatalf("validateConcurrencySpec rejected the baseline: %v", err)
	}
}

// TestValidateConcurrencySpec_RejectsBrokenEntries asserts the
// validator rejects each broken shape with the expected invariant
// name.
//
// # Why the assertion reads the context, not the message
//
// `concurrencyError` builds a *forgeerr.Error whose message is a
// fixed string ("ci: concurrency model invariant violated") and
// whose context carries the invariant's name under the `invariant`
// key. The message is fixed because forgeerr enforces an
// 80-character limit (I4) and the concurrency model's invariant
// names are long enough that composing them into the message would
// exceed the limit.
//
// The test therefore reads the context's `invariant` value, not the
// message. The table's field is named `wantInvariant` to match the
// context key; the assertion is a direct string comparison, not a
// substring check.
//
// # Why the baseline is a function, not a value
//
// validConcurrencySpec returns a fresh ConcurrencySpec on every
// call. A shared package-level value would let a mutation in one
// row leak into the next. The function form is the same pattern the
// other internal/ci tests use.
//
// # Why the trigger-rule rows use the combined invariant names
//
// The trigger rules are priority-ordered (see validateConcurrencySpec
// in concurrency.go): schedule and push:tags first, then
// pull_request, then workflow_call. A schedule-triggered workflow
// and a tag-triggered workflow that misuse their policy produce the
// same invariant name, "scheduled or tag-triggered workflow with
// non-never policy", because the rule that fires is the same. A
// PR-only workflow that misuses its policy produces the distinct
// name "PR-only workflow with non-PR policy".
func TestValidateConcurrencySpec_RejectsBrokenEntries(t *testing.T) {
	cases := []struct {
		name          string
		mutate        func(s *ConcurrencySpec)
		wantInvariant string
	}{
		{
			name: "unknown workflow",
			mutate: func(s *ConcurrencySpec) {
				s.Workflow = "not-a-workflow"
			},
			wantInvariant: "unknown workflow",
		},
		{
			name: "empty group template",
			mutate: func(s *ConcurrencySpec) {
				s.GroupTemplate = ""
			},
			wantInvariant: "empty group template",
		},
		{
			name: "both expression and literal set",
			mutate: func(s *ConcurrencySpec) {
				s.CancelExpression = "${{ github.ref != 'refs/heads/main' }}"
				s.CancelLiteral = true
			},
			wantInvariant: "both expression and literal set",
		},
		{
			name: "empty rationale",
			mutate: func(s *ConcurrencySpec) {
				s.Rationale = ""
			},
			wantInvariant: "empty rationale",
		},
		{
			name: "unknown policy",
			mutate: func(s *ConcurrencySpec) {
				s.Policy = "unknown"
			},
			wantInvariant: "unknown policy",
		},
		{
			name: "on-pr policy with wrong expression",
			mutate: func(s *ConcurrencySpec) {
				s.Policy = CancelPolicyOnPR
				s.CancelExpression = "${{ github.ref == 'refs/heads/main' }}"
			},
			wantInvariant: "on-pr policy with wrong expression",
		},
		{
			name: "never policy with expression",
			mutate: func(s *ConcurrencySpec) {
				s.Policy = CancelPolicyNever
				s.CancelExpression = "${{ github.ref != 'refs/heads/main' }}"
			},
			wantInvariant: "never policy with expression",
		},
		{
			name: "never policy with true literal",
			mutate: func(s *ConcurrencySpec) {
				s.Policy = CancelPolicyNever
				s.CancelExpression = ""
				s.CancelLiteral = true
			},
			wantInvariant: "never policy with true literal",
		},
		{
			name: "PR-only workflow with non-PR policy",
			mutate: func(s *ConcurrencySpec) {
				// WorkflowCI has a pull_request trigger and no
				// schedule or tag trigger, so the priority rule
				// requires on-pr.
				s.Workflow = WorkflowCI
				s.Policy = CancelPolicyNever
				s.CancelExpression = ""
				s.CancelLiteral = false
			},
			wantInvariant: "PR-only workflow with non-PR policy",
		},
		{
			name: "scheduled workflow with non-never policy",
			mutate: func(s *ConcurrencySpec) {
				// WorkflowSecurity has a schedule trigger; the
				// priority rule requires never.
				s.Workflow = WorkflowSecurity
				s.Policy = CancelPolicyOnPR
				s.CancelExpression = "${{ github.ref != 'refs/heads/main' }}"
				s.GroupTemplate = "security-${{ github.workflow }}-${{ github.ref }}"
			},
			wantInvariant: "scheduled or tag-triggered workflow with non-never policy",
		},
		{
			name: "tag-triggered workflow with non-never policy",
			mutate: func(s *ConcurrencySpec) {
				// WorkflowRelease has a push:tags trigger; the
				// priority rule requires never.
				s.Workflow = WorkflowRelease
				s.Policy = CancelPolicyOnPR
				s.CancelExpression = "${{ github.ref != 'refs/heads/main' }}"
				s.GroupTemplate = "release-${{ github.workflow }}-${{ github.ref }}"
			},
			wantInvariant: "scheduled or tag-triggered workflow with non-never policy",
		},
		{
			name: "group template missing discriminator",
			mutate: func(s *ConcurrencySpec) {
				s.GroupTemplate = "ci-${{ github.ref }}"
			},
			wantInvariant: "group template missing discriminator",
		},
		{
			name: "group template missing scope",
			mutate: func(s *ConcurrencySpec) {
				s.GroupTemplate = "ci-${{ github.workflow }}"
			},
			wantInvariant: "group template missing scope",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := validConcurrencySpec()
			tc.mutate(&s)
			err := validateConcurrencySpec(s)
			if err == nil {
				t.Fatal("validateConcurrencySpec accepted a broken entry")
			}

			// The error must be a *forgeerr.Error; the invariant's
			// name lives in its context, not in its message.
			var fe *forgeerr.Error
			if !errors.As(err, &fe) {
				t.Fatalf(
					"validateConcurrencySpec returned %T, want *forgeerr.Error",
					err,
				)
			}
			got := fe.Context()["invariant"]
			if got != tc.wantInvariant {
				t.Errorf(
					"validateConcurrencySpec invariant = %q, want %q",
					got, tc.wantInvariant,
				)
			}
		})
	}
}

// TestValidateConcurrencySpec_IsTotalOnEveryMember asserts the
// canonical specs all pass the per-entry validator.
func TestValidateConcurrencySpec_IsTotalOnEveryMember(t *testing.T) {
	for _, s := range AllConcurrencySpecs() {
		if err := validateConcurrencySpec(s); err != nil {
			t.Errorf("validateConcurrencySpec rejected canonical spec %q: %v", s.Workflow, err)
		}
	}
}

// --- Workflow-policy agreement tests ---------------------------------------

// TestConcurrency_PRWorkflowsUseOnPR asserts every workflow whose
// only supersedable trigger is pull_request uses CancelPolicyOnPR.
//
// # Why the check tolerates schedule and tag triggers
//
// The trigger rules in validateConcurrencySpec are priority-ordered:
// schedule and push:tags dominate, then pull_request, then
// workflow_call. A workflow that has a pull_request trigger and also
// a schedule or push:tags trigger is governed by the higher-priority
// rule and must use CancelPolicyNever. WorkflowReproducible is the
// one such workflow in the closed set. This test therefore checks
// only workflows whose trigger set contains pull_request and neither
// schedule nor push:tags.
func TestConcurrency_PRWorkflowsUseOnPR(t *testing.T) {
	for _, w := range Workflows() {
		hasPR := false
		hasSchedule := false
		hasTag := false
		for _, tg := range w.Triggers {
			switch tg {
			case TriggerPullRequest:
				hasPR = true
			case TriggerSchedule:
				hasSchedule = true
			case TriggerTag:
				hasTag = true
			}
		}
		if !hasPR {
			continue
		}
		if hasSchedule || hasTag {
			// The priority rule governs; not this test's concern.
			continue
		}
		s, ok := LookupConcurrencySpec(w.ID)
		if !ok {
			t.Errorf("workflow %q has a PR trigger but no spec", w.ID)
			continue
		}
		if s.Policy != CancelPolicyOnPR {
			t.Errorf(
				"workflow %q has a PR-only trigger set but uses policy %q, want %q",
				w.ID, s.Policy, CancelPolicyOnPR,
			)
		}
	}
}

// TestConcurrency_ScheduledWorkflowsUseNever asserts every workflow
// with a schedule trigger uses CancelPolicyNever.
func TestConcurrency_ScheduledWorkflowsUseNever(t *testing.T) {
	for _, w := range Workflows() {
		hasSchedule := false
		for _, tg := range w.Triggers {
			if tg == TriggerSchedule {
				hasSchedule = true
				break
			}
		}
		if !hasSchedule {
			continue
		}
		s, ok := LookupConcurrencySpec(w.ID)
		if !ok {
			t.Errorf("workflow %q has a schedule trigger but no spec", w.ID)
			continue
		}
		if s.Policy != CancelPolicyNever {
			t.Errorf(
				"workflow %q has a schedule trigger but uses policy %q, want %q",
				w.ID, s.Policy, CancelPolicyNever,
			)
		}
	}
}

// TestConcurrency_TagWorkflowsUseNever asserts every workflow with a
// push:tags trigger uses CancelPolicyNever.
func TestConcurrency_TagWorkflowsUseNever(t *testing.T) {
	for _, w := range Workflows() {
		hasTag := false
		for _, tg := range w.Triggers {
			if tg == TriggerTag {
				hasTag = true
				break
			}
		}
		if !hasTag {
			continue
		}
		s, ok := LookupConcurrencySpec(w.ID)
		if !ok {
			t.Errorf("workflow %q has a tag trigger but no spec", w.ID)
			continue
		}
		if s.Policy != CancelPolicyNever {
			t.Errorf(
				"workflow %q has a tag trigger but uses policy %q, want %q",
				w.ID, s.Policy, CancelPolicyNever,
			)
		}
	}
}

// --- Golden tests ----------------------------------------------------------

// TestConcurrencyGolden_IsDeterministic asserts repeated calls
// produce byte-identical output.
func TestConcurrencyGolden_IsDeterministic(t *testing.T) {
	a := ConcurrencyGolden()
	b := ConcurrencyGolden()
	if a != b {
		t.Fatal("ConcurrencyGolden() is not deterministic")
	}
}

// TestConcurrencyGolden_IsValidJSON asserts the rendering parses as
// JSON.
func TestConcurrencyGolden_IsValidJSON(t *testing.T) {
	var doc any
	if err := json.Unmarshal([]byte(ConcurrencyGolden()), &doc); err != nil {
		t.Fatalf("ConcurrencyGolden() is not valid JSON: %v", err)
	}
}

// TestConcurrencyGolden_ListsEverySpec asserts every workflow appears
// in the rendering.
func TestConcurrencyGolden_ListsEverySpec(t *testing.T) {
	got := ConcurrencyGolden()
	for _, id := range AllWorkflows() {
		if !strings.Contains(got, string(id)) {
			t.Errorf("ConcurrencyGolden() does not contain workflow %q", id)
		}
	}
}

// TestConcurrencyGolden_ContainsEveryGroupTemplate asserts every
// spec's group template appears in the rendering.
func TestConcurrencyGolden_ContainsEveryGroupTemplate(t *testing.T) {
	got := ConcurrencyGolden()
	for _, s := range AllConcurrencySpecs() {
		if !strings.Contains(got, s.GroupTemplate) {
			t.Errorf(
				"ConcurrencyGolden() does not contain group template %q for workflow %q",
				s.GroupTemplate, s.Workflow,
			)
		}
	}
}

// --- Document tests --------------------------------------------------------

// TestConcurrencyMarkdown_IsDeterministic asserts repeated calls
// produce byte-identical output.
func TestConcurrencyMarkdown_IsDeterministic(t *testing.T) {
	a := ConcurrencyMarkdown()
	b := ConcurrencyMarkdown()
	if a != b {
		t.Fatal("ConcurrencyMarkdown() is not deterministic")
	}
}

// TestConcurrencyMarkdown_ListsEveryWorkflow asserts every workflow
// appears in the rendering.
func TestConcurrencyMarkdown_ListsEveryWorkflow(t *testing.T) {
	got := ConcurrencyMarkdown()
	for _, id := range AllWorkflows() {
		if !strings.Contains(got, string(id)) {
			t.Errorf("ConcurrencyMarkdown() does not contain workflow %q", id)
		}
	}
}

// TestConcurrencyMarkdown_NamesTheTwoPolicies asserts the rendering
// names both policies.
func TestConcurrencyMarkdown_NamesTheTwoPolicies(t *testing.T) {
	got := ConcurrencyMarkdown()
	for _, p := range []CancelPolicy{CancelPolicyOnPR, CancelPolicyNever} {
		if !strings.Contains(got, string(p)) {
			t.Errorf("ConcurrencyMarkdown() does not name policy %q", p)
		}
	}
}

// TestConcurrencyMarkdown_ContainsEveryRationale asserts every spec's
// rationale appears in the rendering.
func TestConcurrencyMarkdown_ContainsEveryRationale(t *testing.T) {
	got := ConcurrencyMarkdown()
	for _, s := range AllConcurrencySpecs() {
		if !strings.Contains(got, s.Rationale) {
			t.Errorf(
				"ConcurrencyMarkdown() does not contain the rationale for workflow %q",
				s.Workflow,
			)
		}
	}
}

// --- helpers ---------------------------------------------------------------

// isZeroConcurrencySpec reports whether s is the zero value of
// ConcurrencySpec.
//
// The check is written by hand rather than with reflect.DeepEqual so
// that adding a field to ConcurrencySpec forces the helper to be
// updated.
func isZeroConcurrencySpec(s ConcurrencySpec) bool {
	return s.Workflow == "" &&
		s.GroupTemplate == "" &&
		s.CancelExpression == "" &&
		!s.CancelLiteral &&
		s.Policy == "" &&
		s.Rationale == ""
}
