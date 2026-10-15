package ci

// cost_test.go — internal tests for the CI cost-budget model.
//
// The tests in this file are package-internal (`package ci`, not
// `package ci_test`) because several of them inspect the package's
// own unexported state:
//
//   - costBudget holds the canonical budget.
//   - costByTier holds the canonical per-tier cost estimates.
//   - optimizations holds the canonical optimization list.
//
// The exported accessors (CostBudgetOf, AllTierCosts, AllOptimizations)
// return copies; the tests assert the copies match the canonical
// state and that the canonical state's invariants hold.
//
// The tests that read the cost-budget document, the ADR, the golden
// JSON, and ci.yml live in cost_contract_test.go and are external
// (`package ci_test`), because they treat the package as a black box
// and assert on the artifacts rather than the model.
//
// # The cost model
//
// The model declares:
//
//   - CostBudget:  the monthly budget (2,000 public, 5,000 private),
//                  the alert threshold (0.80), and the two targets
//                  (50 minutes per PR, 20 minutes per main push).
//   - costByTier:  one TierCost per tier, with the runner, the job
//                  count, the per-job minutes, and the PR/main flags.
//   - optimizations: six Optimization entries, each naming the
//                    technique and the file that implements it.
//   - OSRunnerMultiplier: the closed set of multipliers (Linux 1,
//                         Windows 2, macOS 10).
//
// # Test taxonomy
//
//   - Multiplier tests: the closed set, the numeric values, the
//     panic on an unknown multiplier, and Valid.
//   - Budget tests: the accessor returns the canonical budget, the
//     alert threshold is in (0, 1), the public budget is <= the
//     private budget.
//   - Tier-cost tests: every tier has a cost, the slice is a copy.
//   - Optimization tests: the list is non-empty, the slice is a copy.
//   - Validate tests: the invariants hold, the validator is
//     deterministic and returns *forgeerr.Error.
//   - Golden tests: the JSON rendering is deterministic and valid.
//   - Document tests: the Markdown rendering is deterministic and
//     contains the budget's values and the multipliers.
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

// --- Multiplier tests ------------------------------------------------------

// TestAllOSRunnerMultipliers_HasExactlyThree asserts the closed set
// has the three canonical multipliers.
func TestAllOSRunnerMultipliers_HasExactlyThree(t *testing.T) {
	got := AllOSRunnerMultipliers()
	if len(got) != 3 {
		t.Fatalf("AllOSRunnerMultipliers() returned %d multipliers, want 3", len(got))
	}
}

// TestAllOSRunnerMultipliers_OrderIsStable asserts the canonical
// order.
//
// The order is the order in which the multipliers appear in the
// cost-budget document's table: Linux, Windows, macOS. The order is
// also the order of increasing multiplier, which is the order a
// reader expects.
func TestAllOSRunnerMultipliers_OrderIsStable(t *testing.T) {
	want := []OSRunnerMultiplier{
		OSRunnerLinux,
		OSRunnerWindows,
		OSRunnerMacOS,
	}
	got := AllOSRunnerMultipliers()
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AllOSRunnerMultipliers()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestAllOSRunnerMultipliers_ReturnsFreshSlice asserts the accessor
// does not expose the canonical slice.
func TestAllOSRunnerMultipliers_ReturnsFreshSlice(t *testing.T) {
	a := AllOSRunnerMultipliers()
	a[0] = "mutated"
	b := AllOSRunnerMultipliers()
	if b[0] == "mutated" {
		t.Fatal("AllOSRunnerMultipliers() returned an aliased slice")
	}
}

// TestOSRunnerMultiplier_Multiplier asserts the numeric value of
// each multiplier.
//
// The values are the ones the cost-budget document renders as a
// table: Linux 1, Windows 2, macOS 10. The values are a contract
// with GitHub's pricing; a change to them would be a change to the
// model and to the document in the same commit.
func TestOSRunnerMultiplier_Multiplier(t *testing.T) {
	cases := []struct {
		m    OSRunnerMultiplier
		want int
	}{
		{OSRunnerLinux, 1},
		{OSRunnerWindows, 2},
		{OSRunnerMacOS, 10},
	}
	for _, tc := range cases {
		if got := tc.m.Multiplier(); got != tc.want {
			t.Errorf("OSRunnerMultiplier(%q).Multiplier() = %d, want %d", tc.m, got, tc.want)
		}
	}
}

// TestOSRunnerMultiplier_Multiplier_PanicsOnUnknown asserts the
// constructor-style panic on an unregistered multiplier.
func TestOSRunnerMultiplier_Multiplier_PanicsOnUnknown(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Multiplier() on unknown multiplier did not panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is %T, want string", r)
		}
		if !strings.Contains(msg, "not-a-runner") {
			t.Errorf("panic message %q does not name the offending value", msg)
		}
	}()
	_ = OSRunnerMultiplier("not-a-runner").Multiplier()
}

// TestOSRunnerMultiplier_Valid is the membership table for the
// closed set.
func TestOSRunnerMultiplier_Valid(t *testing.T) {
	cases := []struct {
		m    OSRunnerMultiplier
		want bool
	}{
		{OSRunnerLinux, true},
		{OSRunnerWindows, true},
		{OSRunnerMacOS, true},
		{"", false},
		{"unknown", false},
		{"LINUX", false},
		{"darwin", false},
	}
	for _, tc := range cases {
		if got := tc.m.Valid(); got != tc.want {
			t.Errorf("OSRunnerMultiplier(%q).Valid() = %v, want %v", tc.m, got, tc.want)
		}
	}
}

// TestOSRunnerMultiplier_String_IsTotal asserts String never panics.
func TestOSRunnerMultiplier_String_IsTotal(t *testing.T) {
	cases := []OSRunnerMultiplier{"", "unknown", OSRunnerLinux, OSRunnerMacOS}
	for _, c := range cases {
		got := c.String()
		if got != string(c) {
			t.Errorf("OSRunnerMultiplier(%q).String() = %q, want %q", c, got, string(c))
		}
	}
}

// TestGitHubBillingMultiplier_ConstantsAgreeWithOSRunnerMultiplier
// asserts the integer constants and the string-based multipliers
// agree.
//
// The two representations exist for different callers: the string
// form identifies an OS; the integer form is used for arithmetic.
// The test ensures the two do not drift.
func TestGitHubBillingMultiplier_ConstantsAgreeWithOSRunnerMultiplier(t *testing.T) {
	cases := []struct {
		m        OSRunnerMultiplier
		expected GitHubBillingMultiplier
	}{
		{OSRunnerLinux, MultiplierLinux},
		{OSRunnerWindows, MultiplierWindows},
		{OSRunnerMacOS, MultiplierMacOS},
	}
	for _, tc := range cases {
		if got := GitHubBillingMultiplier(tc.m.Multiplier()); got != tc.expected {
			t.Errorf(
				"OSRunnerMultiplier(%q).Multiplier() = %d, GitHubBillingMultiplier constant = %d",
				tc.m, int(got), int(tc.expected),
			)
		}
	}
}

// --- Budget tests ----------------------------------------------------------

// TestCostBudgetOf_MatchesTheCanonicalBudget asserts the accessor
// returns the canonical budget.
func TestCostBudgetOf_MatchesTheCanonicalBudget(t *testing.T) {
	got := CostBudgetOf()
	if got != costBudget {
		t.Errorf("CostBudgetOf() = %#v, want %#v", got, costBudget)
	}
}

// TestCostBudgetOf_IsDeterministic asserts repeated calls agree.
func TestCostBudgetOf_IsDeterministic(t *testing.T) {
	a := CostBudgetOf()
	b := CostBudgetOf()
	if a != b {
		t.Fatal("CostBudgetOf() is not deterministic")
	}
}

// TestCostBudgetOf_PublicBudgetIsPositive asserts the public budget
// is positive.
func TestCostBudgetOf_PublicBudgetIsPositive(t *testing.T) {
	if CostBudgetOf().PublicMonthlyMinutes <= 0 {
		t.Errorf(
			"CostBudgetOf().PublicMonthlyMinutes = %d, want a positive value",
			CostBudgetOf().PublicMonthlyMinutes,
		)
	}
}

// TestCostBudgetOf_PrivateBudgetIsAtLeastPublic asserts the private
// budget is at least the public budget.
//
// A public repository's free tier and a private repository's free
// tier have the same 2,000-minute floor; a paid plan raises the
// private limit. The invariant is that the private budget is never
// *less* than the public budget.
func TestCostBudgetOf_PrivateBudgetIsAtLeastPublic(t *testing.T) {
	b := CostBudgetOf()
	if b.PrivateMonthlyMinutes < b.PublicMonthlyMinutes {
		t.Errorf(
			"PrivateMonthlyMinutes = %d is less than PublicMonthlyMinutes = %d",
			b.PrivateMonthlyMinutes, b.PublicMonthlyMinutes,
		)
	}
}

// TestCostBudgetOf_AlertThreshold_IsInRange asserts the alert
// threshold is in (0, 1).
func TestCostBudgetOf_AlertThreshold_IsInRange(t *testing.T) {
	th := CostBudgetOf().AlertThreshold
	if th <= 0 || th >= 1 {
		t.Errorf("AlertThreshold = %v, want a value in (0, 1)", th)
	}
}

// TestCostBudgetOf_AlertThreshold_IsEightyPercent asserts the exact
// threshold.
//
// The value is 0.80: an alert fires when 80% of the budget has been
// consumed. A change to the value is a change to the policy; the
// test would fail and prompt the contributor to update the ADR.
func TestCostBudgetOf_AlertThreshold_IsEightyPercent(t *testing.T) {
	const want = 0.80
	if got := CostBudgetOf().AlertThreshold; got != want {
		t.Errorf("AlertThreshold = %v, want %v", got, want)
	}
}

// TestCostBudgetOf_TargetMinutesPerPR_IsPositive asserts the per-PR
// target is positive.
func TestCostBudgetOf_TargetMinutesPerPR_IsPositive(t *testing.T) {
	if CostBudgetOf().TargetMinutesPerPR <= 0 {
		t.Errorf(
			"TargetMinutesPerPR = %d, want a positive value",
			CostBudgetOf().TargetMinutesPerPR,
		)
	}
}

// TestCostBudgetOf_TargetMinutesPerMainPush_IsPositive asserts the
// per-main-push target is positive.
func TestCostBudgetOf_TargetMinutesPerMainPush_IsPositive(t *testing.T) {
	if CostBudgetOf().TargetMinutesPerMainPush <= 0 {
		t.Errorf(
			"TargetMinutesPerMainPush = %d, want a positive value",
			CostBudgetOf().TargetMinutesPerMainPush,
		)
	}
}

// TestCostBudgetOf_TargetsAreOrdered asserts the per-main-push target
// is smaller than the per-PR target.
//
// A main push runs Tiers 1, 2, and 3; a PR push runs Tiers 1 and 2
// only (Tier 3 is gated on main). The per-PR target is therefore
// smaller than the per-main-push target in a naïve model, but the
// repository's targets are the reverse: a main push is a single
// event, so its total cost is amortised over fewer runs than a PR's
// series of pushes. The test pins the relationship the model
// declares; a future change that wanted a different relationship
// would change the model and the test in the same commit.
func TestCostBudgetOf_TargetsAreOrdered(t *testing.T) {
	b := CostBudgetOf()
	if b.TargetMinutesPerPR <= b.TargetMinutesPerMainPush {
		t.Errorf(
			"TargetMinutesPerPR = %d is not greater than TargetMinutesPerMainPush = %d; "+
				"the model declares the per-PR target as the larger of the two",
			b.TargetMinutesPerPR, b.TargetMinutesPerMainPush,
		)
	}
}

// --- Tier-cost tests -------------------------------------------------------

// TestAllTierCosts_CoversEveryTier asserts every tier has a cost
// estimate.
func TestAllTierCosts_CoversEveryTier(t *testing.T) {
	seen := make(map[TierID]struct{}, len(costByTier))
	for _, c := range costByTier {
		seen[c.Tier] = struct{}{}
	}
	for _, id := range AllTiers() {
		if _, ok := seen[id]; !ok {
			t.Errorf("tier %q has no cost estimate", string(id))
		}
	}
}

// TestAllTierCosts_ReturnsFreshSlice asserts the accessor returns a
// copy.
func TestAllTierCosts_ReturnsFreshSlice(t *testing.T) {
	a := AllTierCosts()
	if len(a) == 0 {
		t.Skip("no tier costs to mutate")
	}
	a[0].JobCount = -1
	b := AllTierCosts()
	if b[0].JobCount == -1 {
		t.Fatal("AllTierCosts() returned an aliased slice")
	}
}

// TestAllTierCosts_OrderMatchesAllTiers asserts the tier-cost slice's
// order matches the canonical tier order.
func TestAllTierCosts_OrderMatchesAllTiers(t *testing.T) {
	costs := AllTierCosts()
	ids := AllTiers()
	if len(costs) != len(ids) {
		t.Fatalf("AllTierCosts() has %d entries, want %d", len(costs), len(ids))
	}
	for i := range ids {
		if costs[i].Tier != ids[i] {
			t.Errorf(
				"AllTierCosts()[%d].Tier = %q, want %q",
				i, costs[i].Tier, ids[i],
			)
		}
	}
}

// TestAllTierCosts_EveryRunnerIsValid asserts every tier's runner is
// a member of the closed set.
func TestAllTierCosts_EveryRunnerIsValid(t *testing.T) {
	for _, c := range AllTierCosts() {
		if !c.Runner.Valid() {
			t.Errorf("tier %q has runner %q, which is not valid", c.Tier, c.Runner)
		}
	}
}

// TestAllTierCosts_EveryJobCountIsPositive asserts every tier's job
// count is positive.
func TestAllTierCosts_EveryJobCountIsPositive(t *testing.T) {
	for _, c := range AllTierCosts() {
		if c.JobCount <= 0 {
			t.Errorf("tier %q has JobCount = %d, want a positive value", c.Tier, c.JobCount)
		}
	}
}

// TestAllTierCosts_EveryPerJobMinutesIsPositive asserts every tier's
// per-job minutes estimate is positive.
func TestAllTierCosts_EveryPerJobMinutesIsPositive(t *testing.T) {
	for _, c := range AllTierCosts() {
		if c.PerJobMinutes <= 0 {
			t.Errorf("tier %q has PerJobMinutes = %d, want a positive value", c.Tier, c.PerJobMinutes)
		}
	}
}

// TestAllTierCosts_Tier3IsMainOnly asserts Tier 3 does not run on
// PRs.
//
// Tier 3 is gated on main pushes (18.2.3). Its cost is paid once per
// main push, not once per PR push. A future change that removed the
// gate would change the tier's cost profile; the test pins the
// current gate.
func TestAllTierCosts_Tier3IsMainOnly(t *testing.T) {
	for _, c := range AllTierCosts() {
		if c.Tier != Tier3 {
			continue
		}
		if c.RunsOnPR {
			t.Error("Tier 3's cost estimate says it runs on PRs; the tier is gated on main")
		}
		if !c.RunsOnMain {
			t.Error("Tier 3's cost estimate says it does not run on main; the tier runs on main")
		}
		return
	}
	t.Fatal("Tier 3 has no cost estimate")
}

// --- Optimization tests ----------------------------------------------------

// TestAllOptimizations_IsNonEmpty asserts the optimization list is
// non-empty.
func TestAllOptimizations_IsNonEmpty(t *testing.T) {
	if len(AllOptimizations()) == 0 {
		t.Fatal("AllOptimizations() is empty")
	}
}

// TestAllOptimizations_ReturnsFreshSlice asserts the accessor returns
// a copy.
func TestAllOptimizations_ReturnsFreshSlice(t *testing.T) {
	a := AllOptimizations()
	if len(a) == 0 {
		t.Skip("no optimizations to mutate")
	}
	a[0].Name = "mutated"
	b := AllOptimizations()
	if b[0].Name == "mutated" {
		t.Fatal("AllOptimizations() returned an aliased slice")
	}
}

// TestAllOptimizations_EveryNameIsNonEmpty asserts every
// optimization has a name.
func TestAllOptimizations_EveryNameIsNonEmpty(t *testing.T) {
	for i, o := range AllOptimizations() {
		if o.Name == "" {
			t.Errorf("AllOptimizations()[%d].Name is empty", i)
		}
	}
}

// TestAllOptimizations_EveryRationaleIsNonEmpty asserts every
// optimization has a rationale.
func TestAllOptimizations_EveryRationaleIsNonEmpty(t *testing.T) {
	for i, o := range AllOptimizations() {
		if o.Rationale == "" {
			t.Errorf("AllOptimizations()[%d].Rationale is empty", i)
		}
	}
}

// TestAllOptimizations_EveryMechanismIsNonEmpty asserts every
// optimization has a mechanism.
func TestAllOptimizations_EveryMechanismIsNonEmpty(t *testing.T) {
	for i, o := range AllOptimizations() {
		if o.Mechanism == "" {
			t.Errorf("AllOptimizations()[%d].Mechanism is empty", i)
		}
	}
}

// TestAllOptimizations_NamesAreUnique asserts the names are unique.
func TestAllOptimizations_NamesAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for i, o := range AllOptimizations() {
		if seen[o.Name] {
			t.Errorf("AllOptimizations()[%d].Name = %q appears more than once", i, o.Name)
		}
		seen[o.Name] = true
	}
}

// --- Validate tests --------------------------------------------------------

// TestValidateCost_AcceptsCanonicalState asserts the canonical state
// is internally consistent.
func TestValidateCost_AcceptsCanonicalState(t *testing.T) {
	if err := ValidateCost(); err != nil {
		t.Fatalf("ValidateCost() rejected the canonical state: %v", err)
	}
}

// TestValidateCost_IsDeterministic asserts repeated calls agree.
func TestValidateCost_IsDeterministic(t *testing.T) {
	for i := 0; i < 3; i++ {
		if err := ValidateCost(); err != nil {
			t.Fatalf("ValidateCost() call %d failed: %v", i, err)
		}
	}
}

// TestValidateCost_ErrorIsForgeError asserts the error type.
//
// # Why the probe is the costError helper, not ValidateCost
//
// ValidateCost on the canonical state returns nil; it cannot be used
// to produce an error without mutating package-level data, which the
// test suite deliberately avoids. The cost model has no per-entry
// validator (its entries are a budget struct, a slice of tier costs,
// and a slice of optimizations), so the test calls the unexported
// costError helper directly to exercise the error-construction path.
func TestValidateCost_ErrorIsForgeError(t *testing.T) {
	err := costError("synthetic", "test")
	if err == nil {
		t.Fatal("costError returned nil")
	}
	var fe *forgeerr.Error
	if !errors.As(err, &fe) {
		t.Fatalf("costError returned %T, want *forgeerr.Error", err)
	}
	if fe.Category() != forgeerr.CategoryInternal {
		t.Errorf("costError category = %v, want %v", fe.Category(), forgeerr.CategoryInternal)
	}
	if fe.Code() != string(forgeerr.CodeInternalUnexpected) {
		t.Errorf(
			"costError code = %q, want %q",
			fe.Code(), forgeerr.CodeInternalUnexpected,
		)
	}
}

// --- Golden tests ----------------------------------------------------------

// TestCostGolden_IsDeterministic asserts repeated calls produce
// byte-identical output.
func TestCostGolden_IsDeterministic(t *testing.T) {
	a := CostGolden()
	b := CostGolden()
	if a != b {
		t.Fatal("CostGolden() is not deterministic")
	}
}

// TestCostGolden_IsValidJSON asserts the rendering parses as JSON.
func TestCostGolden_IsValidJSON(t *testing.T) {
	var doc any
	if err := json.Unmarshal([]byte(CostGolden()), &doc); err != nil {
		t.Fatalf("CostGolden() is not valid JSON: %v", err)
	}
}

// TestCostGolden_ContainsTheBudget asserts the rendering includes
// the budget's two values and the threshold.
func TestCostGolden_ContainsTheBudget(t *testing.T) {
	got := CostGolden()
	must := []string{
		"public_monthly_minutes",
		"private_monthly_minutes",
		"alert_threshold",
		"target_minutes_per_pr",
		"target_minutes_per_main_push",
	}
	for _, key := range must {
		if !strings.Contains(got, key) {
			t.Errorf("CostGolden() does not contain the key %q", key)
		}
	}
}

// TestCostGolden_ContainsTheMultipliers asserts the rendering
// includes the three multipliers.
func TestCostGolden_ContainsTheMultipliers(t *testing.T) {
	got := CostGolden()
	for _, m := range AllOSRunnerMultipliers() {
		if !strings.Contains(got, string(m)) {
			t.Errorf("CostGolden() does not contain the multiplier %q", m)
		}
	}
}

// TestCostGolden_ContainsEveryTier asserts the rendering includes
// every tier.
func TestCostGolden_ContainsEveryTier(t *testing.T) {
	got := CostGolden()
	for _, id := range AllTiers() {
		if !strings.Contains(got, string(id)) {
			t.Errorf("CostGolden() does not contain the tier %q", id)
		}
	}
}

// TestCostGolden_ContainsEveryOptimization asserts the rendering
// includes every optimization's name.
func TestCostGolden_ContainsEveryOptimization(t *testing.T) {
	got := CostGolden()
	for _, o := range AllOptimizations() {
		if !strings.Contains(got, o.Name) {
			t.Errorf("CostGolden() does not contain the optimization %q", o.Name)
		}
	}
}

// --- Document tests --------------------------------------------------------

// TestCostMarkdown_IsDeterministic asserts repeated calls produce
// byte-identical output.
func TestCostMarkdown_IsDeterministic(t *testing.T) {
	a := CostMarkdown()
	b := CostMarkdown()
	if a != b {
		t.Fatal("CostMarkdown() is not deterministic")
	}
}

// TestCostMarkdown_ContainsTheBudget asserts the rendering names the
// budget's two values.
func TestCostMarkdown_ContainsTheBudget(t *testing.T) {
	got := CostMarkdown()
	b := CostBudgetOf()
	if !strings.Contains(got, itoa(b.PublicMonthlyMinutes)) {
		t.Errorf("CostMarkdown() does not contain the public budget %d", b.PublicMonthlyMinutes)
	}
	if !strings.Contains(got, itoa(b.PrivateMonthlyMinutes)) {
		t.Errorf("CostMarkdown() does not contain the private budget %d", b.PrivateMonthlyMinutes)
	}
}

// TestCostMarkdown_ContainsTheMultipliers asserts the rendering
// includes the multipliers' table.
func TestCostMarkdown_ContainsTheMultipliers(t *testing.T) {
	got := CostMarkdown()
	for _, m := range AllOSRunnerMultipliers() {
		if !strings.Contains(got, string(m)) {
			t.Errorf("CostMarkdown() does not contain the multiplier %q", m)
		}
	}
	if !strings.Contains(got, "Multiplier") {
		t.Error("CostMarkdown() does not have a Multiplier column")
	}
}

// TestCostMarkdown_ContainsEveryOptimization asserts the rendering
// names every optimization.
func TestCostMarkdown_ContainsEveryOptimization(t *testing.T) {
	got := CostMarkdown()
	for _, o := range AllOptimizations() {
		if !strings.Contains(got, o.Name) {
			t.Errorf("CostMarkdown() does not contain the optimization %q", o.Name)
		}
		if !strings.Contains(got, o.Mechanism) {
			t.Errorf(
				"CostMarkdown() does not contain the mechanism %q for optimization %q",
				o.Mechanism, o.Name,
			)
		}
	}
}

// TestCostMarkdown_ContainsTheDashboardMetrics asserts the rendering
// names the three dashboard metrics.
func TestCostMarkdown_ContainsTheDashboardMetrics(t *testing.T) {
	got := CostMarkdown()
	must := []string{
		"Monthly minutes used",
		"Cost per PR",
		"Cost per main push",
	}
	for _, want := range must {
		if !strings.Contains(got, want) {
			t.Errorf("CostMarkdown() does not contain the metric %q", want)
		}
	}
}
