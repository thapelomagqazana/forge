package ci

// tiers_test.go — internal tests for the CI tier model.
//
// The tests in this file are package-internal because several of them
// inspect the package's own unexported state (allTiers, tiers) and
// because the closure tests must be able to name the canonical slices
// when they fail.
//
// # Test taxonomy
//
//   - Closure tests: the set has exactly the members it is supposed to.
//   - Order tests: the canonical order is stable and the two
//     representations agree.
//   - Number tests: the ordinal derivation.
//   - BlockingStatus tests: the closed set.
//   - TriggerSet tests: Contains.
//   - Copy tests: deep-copy semantics.
//   - Budget tests: the strictly-increasing invariant.
//   - Lookup tests: totality and the zero value.
//   - TiersForWorkflow / ChecksForWorkflow tests: the workflow-to-tier
//     projection.
//   - ValidateTiers tests: the invariants, table-driven on the broken
//     shape.
//   - formatBudget tests: the deterministic rendering.
//   - TiersTable tests: the deterministic rendering of the table.

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// --- Closure tests ---------------------------------------------------------

// TestAllTiers_HasExactlyFour asserts the closed set has the four
// canonical members. A fifth tier is a change to the CI architecture
// and must be accompanied by an ADR.
func TestAllTiers_HasExactlyFour(t *testing.T) {
	got := AllTiers()
	if len(got) != 4 {
		t.Fatalf("AllTiers() returned %d tiers, want 4", len(got))
	}
}

// TestAllTiers_OrderIsStable asserts the canonical order.
func TestAllTiers_OrderIsStable(t *testing.T) {
	want := []TierID{Tier1, Tier2, Tier3, Tier4}
	got := AllTiers()
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AllTiers()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestAllTiers_ReturnsFreshSlice asserts the accessor does not expose
// the canonical slice.
func TestAllTiers_ReturnsFreshSlice(t *testing.T) {
	a := AllTiers()
	a[0] = "mutated"
	b := AllTiers()
	if b[0] == "mutated" {
		t.Fatal("AllTiers() returned an aliased slice")
	}
}

// TestAllTierDescriptions_HasExactlyFour asserts the descriptions and
// the IDs agree in number.
func TestAllTierDescriptions_HasExactlyFour(t *testing.T) {
	got := AllTierDescriptions()
	if len(got) != 4 {
		t.Fatalf("AllTierDescriptions() returned %d tiers, want 4", len(got))
	}
}

// --- Order tests -----------------------------------------------------------

// TestAllTierDescriptions_OrderMatchesAllTiers asserts the two
// representations agree on order.
func TestAllTierDescriptions_OrderMatchesAllTiers(t *testing.T) {
	ids := AllTiers()
	ts := AllTierDescriptions()
	if len(ids) != len(ts) {
		t.Fatalf("AllTiers() has %d entries but AllTierDescriptions() has %d", len(ids), len(ts))
	}
	for i := range ids {
		if ts[i].ID != ids[i] {
			t.Errorf("AllTierDescriptions()[%d].ID = %q, want %q", i, ts[i].ID, ids[i])
		}
	}
}

// TestAllTierDescriptions_ReturnsDeepCopy asserts the accessor
// deep-copies each Tier.
//
// # Why the loop variable is named tr, not t
//
// The test's parameter is *testing.T, conventionally named t. A loop
// over the returned []Tier that used t as the loop variable would
// shadow the test handle, and any call to t.Errorf or t.Fatalf inside
// the loop body would fail to compile ("t.Fatalf undefined (type Tier
// has no field or method Fatalf)"). The loop variable is therefore
// named tr, which is the same convention the package's source files
// use for a Tier value.
func TestAllTierDescriptions_ReturnsDeepCopy(t *testing.T) {
	first := AllTierDescriptions()
	for i := range first {
		for j := range first[i].Contents {
			first[i].Contents[j].Name = "mutated-check"
		}
		for j := range first[i].BlockingOn {
			first[i].BlockingOn[j] = "mutated-trigger"
		}
		for j := range first[i].Triggers {
			first[i].Triggers[j] = "mutated-trigger"
		}
	}

	second := AllTierDescriptions()
	for _, tr := range second {
		for _, c := range tr.Contents {
			if c.Name == "mutated-check" {
				t.Fatalf("tier %q: Contents aliased the canonical slice", tr.ID)
			}
		}
		for _, b := range tr.BlockingOn {
			if b == "mutated-trigger" {
				t.Fatalf("tier %q: BlockingOn aliased the canonical slice", tr.ID)
			}
		}
		for _, tg := range tr.Triggers {
			if tg == "mutated-trigger" {
				t.Fatalf("tier %q: Triggers aliased the canonical slice", tr.ID)
			}
		}
	}
}

// --- Number tests ----------------------------------------------------------

// TestTierID_Number asserts the ordinal is derived from the tier's
// position in the canonical order, not from a parse of the string.
func TestTierID_Number(t *testing.T) {
	cases := []struct {
		id   TierID
		want int
	}{
		{Tier1, 1},
		{Tier2, 2},
		{Tier3, 3},
		{Tier4, 4},
	}
	for _, tc := range cases {
		if got := tc.id.Number(); got != tc.want {
			t.Errorf("TierID(%q).Number() = %d, want %d", tc.id, got, tc.want)
		}
	}
}

// TestTierID_Number_PanicsOnUnknown asserts the constructor-style
// panic on an unregistered ID.
func TestTierID_Number_PanicsOnUnknown(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Number() on unknown ID did not panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is %T, want string", r)
		}
		if !strings.Contains(msg, "not-a-tier") {
			t.Errorf("panic message %q does not name the offending value", msg)
		}
	}()
	_ = TierID("not-a-tier").Number()
}

// TestTierID_String asserts String is total and identity-preserving.
func TestTierID_String(t *testing.T) {
	cases := []TierID{"", "unknown", Tier1, Tier4}
	for _, c := range cases {
		if got := c.String(); got != string(c) {
			t.Errorf("TierID(%q).String() = %q, want %q", c, got, string(c))
		}
	}
}

// TestTierID_Valid is the membership table.
func TestTierID_Valid(t *testing.T) {
	cases := []struct {
		id   TierID
		want bool
	}{
		{Tier1, true},
		{Tier2, true},
		{Tier3, true},
		{Tier4, true},
		{"", false},
		{"unknown", false},
		{"TIER1", false},
		{"tier5", false},
	}
	for _, tc := range cases {
		if got := tc.id.Valid(); got != tc.want {
			t.Errorf("TierID(%q).Valid() = %v, want %v", tc.id, got, tc.want)
		}
	}
}

// --- BlockingStatus tests --------------------------------------------------

// TestBlockingStatus_Valid is the membership table.
func TestBlockingStatus_Valid(t *testing.T) {
	cases := []struct {
		b    BlockingStatus
		want bool
	}{
		{BlockingOnPR, true},
		{BlockingOnMain, true},
		{Advisory, true},
		{"", false},
		{"blocking", false},
		{"BLOCKING-ON-PR", false},
	}
	for _, tc := range cases {
		if got := tc.b.Valid(); got != tc.want {
			t.Errorf("BlockingStatus(%q).Valid() = %v, want %v", tc.b, got, tc.want)
		}
	}
}

// TestBlockingStatus_String asserts String is total and
// identity-preserving.
func TestBlockingStatus_String(t *testing.T) {
	cases := []BlockingStatus{"", "unknown", BlockingOnPR, Advisory}
	for _, c := range cases {
		if got := c.String(); got != string(c) {
			t.Errorf("BlockingStatus(%q).String() = %q, want %q", c, got, string(c))
		}
	}
}

// --- TriggerSet tests ------------------------------------------------------

// TestTriggerSet_Contains asserts the membership predicate.
func TestTriggerSet_Contains(t *testing.T) {
	ts := TriggerSet{TriggerPullRequest, TriggerPush}
	if !ts.Contains(TriggerPullRequest) {
		t.Error("TriggerSet does not contain pull_request")
	}
	if ts.Contains(TriggerSchedule) {
		t.Error("TriggerSet contains schedule")
	}
}

// TestTriggerSet_Empty asserts the empty set contains nothing.
func TestTriggerSet_Empty(t *testing.T) {
	var ts TriggerSet
	if ts.Contains(TriggerPullRequest) {
		t.Error("empty TriggerSet contains pull_request")
	}
}

// --- Budget tests ----------------------------------------------------------

// TestTiers_BudgetsAreStrictlyIncreasing asserts the invariant.
func TestTiers_BudgetsAreStrictlyIncreasing(t *testing.T) {
	ts := AllTierDescriptions()
	for i := 1; i < len(ts); i++ {
		if ts[i].Budget <= ts[i-1].Budget {
			t.Fatalf(
				"tier %q budget %v is not greater than tier %q budget %v",
				ts[i].ID, ts[i].Budget, ts[i-1].ID, ts[i-1].Budget,
			)
		}
	}
}

// TestTiers_BudgetValues asserts the four budgets match the WBS
// targets.
//
// The test is intentionally exact. A change to a budget is a change
// to the CI architecture and must be accompanied by an ADR and a
// documentation update.
func TestTiers_BudgetValues(t *testing.T) {
	cases := []struct {
		id   TierID
		want time.Duration
	}{
		{Tier1, 3 * time.Minute},
		{Tier2, 15 * time.Minute},
		{Tier3, 60 * time.Minute},
		{Tier4, 120 * time.Minute},
	}
	for _, tc := range cases {
		got, ok := LookupTier(tc.id)
		if !ok {
			t.Errorf("LookupTier(%q) not found", tc.id)
			continue
		}
		if got.Budget != tc.want {
			t.Errorf("tier %q budget = %v, want %v", tc.id, got.Budget, tc.want)
		}
	}
}

// --- Lookup tests ----------------------------------------------------------

// TestLookupTier_FindsEveryTier asserts LookupTier is total.
func TestLookupTier_FindsEveryTier(t *testing.T) {
	for _, id := range AllTiers() {
		tr, ok := LookupTier(id)
		if !ok {
			t.Errorf("LookupTier(%q) reported not found", id)
			continue
		}
		if tr.ID != id {
			t.Errorf("LookupTier(%q).ID = %q", id, tr.ID)
		}
		if tr.Name == "" {
			t.Errorf("LookupTier(%q).Name is empty", id)
		}
		if tr.Goal == "" {
			t.Errorf("LookupTier(%q).Goal is empty", id)
		}
	}
}

// TestLookupTier_UnknownReturnsZero asserts LookupTier does not
// panic and returns the zero Tier for an unknown ID.
func TestLookupTier_UnknownReturnsZero(t *testing.T) {
	tr, ok := LookupTier("nope")
	if ok {
		t.Fatal("LookupTier(\"nope\") reported found")
	}
	if tr.ID != "" || tr.Name != "" || tr.Budget != 0 || len(tr.Contents) != 0 {
		t.Fatalf("LookupTier(\"nope\") returned %#v, want the zero Tier", tr)
	}
}

// TestLookupTier_ReturnsDeepCopy asserts the accessor deep-copies.
func TestLookupTier_ReturnsDeepCopy(t *testing.T) {
	tr, ok := LookupTier(Tier1)
	if !ok {
		t.Fatal("LookupTier(Tier1) not found")
	}
	for j := range tr.Contents {
		tr.Contents[j].Name = "mutated"
	}
	for j := range tr.Triggers {
		tr.Triggers[j] = "mutated"
	}
	again, ok := LookupTier(Tier1)
	if !ok {
		t.Fatal("LookupTier(Tier1) not found on second call")
	}
	for _, c := range again.Contents {
		if c.Name == "mutated" {
			t.Fatal("LookupTier returned an aliased Contents slice")
		}
	}
	for _, tg := range again.Triggers {
		if tg == "mutated" {
			t.Fatal("LookupTier returned an aliased Triggers slice")
		}
	}
}

// --- Projection tests ------------------------------------------------------

// TestTiersForWorkflow asserts the workflow-to-tier projection.
func TestTiersForWorkflow(t *testing.T) {
	cases := []struct {
		wf   WorkflowID
		want []TierID
	}{
		{WorkflowCI, []TierID{Tier1, Tier2, Tier3}},
		{WorkflowSecurity, []TierID{Tier4}},
		{WorkflowNightly, []TierID{Tier4}},
		{WorkflowDocs, nil},
		{WorkflowRelease, nil},
	}
	for _, tc := range cases {
		got := TiersForWorkflow(tc.wf)
		if !equalTierIDs(got, tc.want) {
			t.Errorf("TiersForWorkflow(%q) = %v, want %v", tc.wf, got, tc.want)
		}
	}
}

// TestChecksForWorkflow asserts the workflow-to-check projection.
func TestChecksForWorkflow(t *testing.T) {
	ci := ChecksForWorkflow(WorkflowCI)
	if len(ci) == 0 {
		t.Fatal("ChecksForWorkflow(WorkflowCI) returned no checks")
	}
	for _, c := range ci {
		if c.Workflow != WorkflowCI {
			t.Errorf("ChecksForWorkflow(WorkflowCI) returned check %q in workflow %q", c.Name, c.Workflow)
		}
	}

	// A workflow with no checks returns an empty slice, not nil,
	// because the caller may append to the result. The empty-slice
	// contract is documented in ChecksForWorkflow's docstring.
	none := ChecksForWorkflow(WorkflowRelease)
	if none == nil {
		t.Fatal("ChecksForWorkflow(WorkflowRelease) returned nil, want an empty slice")
	}
	if len(none) != 0 {
		t.Fatalf("ChecksForWorkflow(WorkflowRelease) returned %d checks, want 0", len(none))
	}
}

// --- ValidateTiers tests ---------------------------------------------------

// TestValidateTiers_AcceptsCanonicalState asserts the canonical state
// is internally consistent.
func TestValidateTiers_AcceptsCanonicalState(t *testing.T) {
	if err := ValidateTiers(); err != nil {
		t.Fatalf("ValidateTiers() rejected the canonical state: %v", err)
	}
}

// TestValidateTiers_IsDeterministic asserts repeated calls agree.
func TestValidateTiers_IsDeterministic(t *testing.T) {
	for i := 0; i < 3; i++ {
		if err := ValidateTiers(); err != nil {
			t.Fatalf("ValidateTiers() call %d failed: %v", i, err)
		}
	}
}

// TestValidateTiers_ErrorIsForgeError asserts the error type.
//
// The probe is a synthetic broken Tier passed to a private helper
// (validateTier). ValidateTiers itself operates on the canonical
// state; a probe that does not mutate the canonical state must use
// the per-tier validator. The private helper is named validateTier
// and is unexported because it is an implementation detail.
func TestValidateTiers_ErrorIsForgeError(t *testing.T) {
	// Trigger an error by constructing a Tier with an unknown ID and
	// passing it to the unexported per-tier validator.
	err := validateTier(Tier{ID: "not-a-tier"})
	if err == nil {
		t.Fatal("validateTier on a broken tier returned nil")
	}
	var fe *forgeerr.Error
	if !errors.As(err, &fe) {
		t.Fatalf("validateTier returned %T, want *forgeerr.Error", err)
	}
	if fe.Category() != forgeerr.CategoryInternal {
		t.Errorf("validateTier category = %v, want %v", fe.Category(), forgeerr.CategoryInternal)
	}
}

// --- formatBudget tests ----------------------------------------------------

// TestFormatBudget is the table for the deterministic rendering.
func TestFormatBudget(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "0s"},
		{30 * time.Second, "30s"},
		{59 * time.Second, "59s"},
		{time.Minute, "1m"},
		{90 * time.Second, "1m 30s"},
		{3 * time.Minute, "3m"},
		{15 * time.Minute, "15m"},
		{60 * time.Minute, "1h"},
		{90 * time.Minute, "1h 30m"},
		{120 * time.Minute, "2h"},
		{-1 * time.Second, "invalid(-1s)"},
	}
	for _, tc := range cases {
		if got := formatBudget(tc.in); got != tc.want {
			t.Errorf("formatBudget(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestFormatBudget_IsDeterministic asserts repeated calls agree.
func TestFormatBudget_IsDeterministic(t *testing.T) {
	for i := 0; i < 3; i++ {
		if got := formatBudget(90 * time.Second); got != "1m 30s" {
			t.Fatalf("formatBudget(90s) call %d = %q", i, got)
		}
	}
}

// --- TiersTable tests ------------------------------------------------------

// TestTiersTable_HasHeader asserts the rendering starts with the
// expected header.
func TestTiersTable_HasHeader(t *testing.T) {
	got := TiersTable()
	if !strings.HasPrefix(got, "# CI Tier Model\n") {
		t.Errorf("TiersTable() does not start with the expected header; got %q", got[:min(len(got), 40)])
	}
}

// TestTiersTable_ListsEveryTier asserts every tier appears in the
// summary table.
func TestTiersTable_ListsEveryTier(t *testing.T) {
	got := TiersTable()
	for _, id := range AllTiers() {
		tr, _ := LookupTier(id)
		if !strings.Contains(got, "Tier "+itoa(id.Number())) {
			t.Errorf("TiersTable() does not contain 'Tier %d'", id.Number())
		}
		if !strings.Contains(got, tr.Name) {
			t.Errorf("TiersTable() does not contain tier name %q", tr.Name)
		}
	}
}

// TestTiersTable_ListsEveryCheck asserts every check appears in the
// detail sections.
func TestTiersTable_ListsEveryCheck(t *testing.T) {
	got := TiersTable()
	for _, tr := range AllTierDescriptions() {
		for _, c := range tr.Contents {
			if !strings.Contains(got, "`"+c.Name+"`") {
				t.Errorf("TiersTable() does not contain check %q", c.Name)
			}
		}
	}
}

// TestTiersTable_IsDeterministic asserts repeated calls produce
// byte-identical output.
func TestTiersTable_IsDeterministic(t *testing.T) {
	a := TiersTable()
	b := TiersTable()
	if a != b {
		t.Fatal("TiersTable() is not deterministic")
	}
}

// --- helpers ---------------------------------------------------------------

// equalTierIDs reports whether two TierID slices are equal.
func equalTierIDs(a, b []TierID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
