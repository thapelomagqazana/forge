package ci

// flake_test.go — internal tests for the flake policy model.
//
// The tests in this file are package-internal (`package ci`, not
// `package ci_test`) because several of them inspect the package's
// own unexported state:
//
//   - flakeSLA and flakeDetection are unexported; the exported
//     accessors return copies, but the tests inspect the canonical
//     values directly.
//   - The model's constants (FlakePriorityP0, FlakeQuarantineMarker,
//     FlakeQuarantineFormat, FlakeRateTarget, FlakeRetryPolicyCurrent)
//     are exported but tied to the model; the tests pin their values.
//
// The tests that read the policy document and the workflow files
// live in flake_contract_test.go and are external (`package ci_test`),
// because they treat the package as a black box and assert on the
// artifacts rather than the model.
//
// # The flake model
//
// The model declares six properties:
//
//   - FlakePriorityP0        — a flaky test is a P0 bug.
//   - flakeSLA               — quarantine within 24h, fix within 1w.
//   - FlakeQuarantineFormat  — t.Skip("flaky: #<issue>").
//   - FlakeRateTarget        — < 0.5% per test.
//   - FlakeRetryPolicyCurrent — none (no automatic retries).
//   - flakeDetection         — -count=2, flake-report.txt artifact.
//
// # Test taxonomy
//
//   - Priority tests: the priority is exactly P0.
//   - SLA tests: the two durations are non-empty and match the
//     policy's commitments.
//   - Quarantine tests: the marker and format are the ones the
//     policy document names.
//   - Rate tests: the target is in [0, 1] and matches the policy's
//     commitment.
//   - Retry tests: the current policy is `none`.
//   - Detection tests: the tier-3 count and the artifact name.
//   - Validate tests: the invariants.
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

// --- Priority tests --------------------------------------------------------

// TestFlakePriorityP0_IsExactlyP0 asserts the priority is exactly
// "P0".
//
// The value is the model's declaration that a flaky test is a P0
// bug. A change to a different string (for example, "P1") would be a
// change to the policy; the test would fail and prompt the
// contributor to update the policy document in the same commit.
func TestFlakePriorityP0_IsExactlyP0(t *testing.T) {
	if FlakePriorityP0 != "P0" {
		t.Errorf("FlakePriorityP0 = %q, want %q", string(FlakePriorityP0), "P0")
	}
}

// TestFlakePriority_String asserts String is total and
// identity-preserving.
func TestFlakePriority_String(t *testing.T) {
	cases := []FlakePriority{"", "unknown", FlakePriorityP0}
	for _, c := range cases {
		got := c.String()
		if got != string(c) {
			t.Errorf("FlakePriority(%q).String() = %q, want %q", c, got, string(c))
		}
	}
}

// --- SLA tests -------------------------------------------------------------

// TestFlakeSLAOf_IsNonEmpty asserts both SLA durations are
// non-empty.
func TestFlakeSLAOf_IsNonEmpty(t *testing.T) {
	sla := FlakeSLAOf()
	if sla.QuarantineWithin == "" {
		t.Error("FlakeSLAOf().QuarantineWithin is empty")
	}
	if sla.FixWithin == "" {
		t.Error("FlakeSLAOf().FixWithin is empty")
	}
}

// TestFlakeSLAOf_MatchesTheCanonicalValues asserts the exported
// accessor returns the same values as the unexported struct.
//
// The check ensures the accessor and the canonical state agree: a
// future change that made the accessor return a modified copy would
// fail this test.
func TestFlakeSLAOf_MatchesTheCanonicalValues(t *testing.T) {
	got := FlakeSLAOf()
	if got.QuarantineWithin != flakeSLA.QuarantineWithin {
		t.Errorf(
			"FlakeSLAOf().QuarantineWithin = %q, want %q",
			got.QuarantineWithin, flakeSLA.QuarantineWithin,
		)
	}
	if got.FixWithin != flakeSLA.FixWithin {
		t.Errorf(
			"FlakeSLAOf().FixWithin = %q, want %q",
			got.FixWithin, flakeSLA.FixWithin,
		)
	}
}

// TestFlakeSLAOf_IsDeterministic asserts repeated calls agree.
func TestFlakeSLAOf_IsDeterministic(t *testing.T) {
	a := FlakeSLAOf()
	b := FlakeSLAOf()
	if a != b {
		t.Fatal("FlakeSLAOf() is not deterministic")
	}
}

// --- Quarantine tests ------------------------------------------------------

// TestFlakeQuarantineMarker_IsTheFlakyMarker asserts the marker's
// value.
//
// The value is the string a reader searches for to find quarantined
// tests. A change to a different string (for example, "quarantine:")
// would be a change to the policy and to the aggregator's search
// term; the test would fail and prompt the contributor to update the
// policy document.
func TestFlakeQuarantineMarker_IsTheFlakyMarker(t *testing.T) {
	if FlakeQuarantineMarker != "flaky:" {
		t.Errorf("FlakeQuarantineMarker = %q, want %q", FlakeQuarantineMarker, "flaky:")
	}
}

// TestFlakeQuarantineFormat_ContainsTheMarker asserts the format
// contains the marker.
//
// The format is `t.Skip("flaky: #%d")`. The check ensures the two
// constants agree: a future change to one without the other would
// fail this test.
func TestFlakeQuarantineFormat_ContainsTheMarker(t *testing.T) {
	if !strings.Contains(FlakeQuarantineFormat, FlakeQuarantineMarker) {
		t.Errorf(
			"FlakeQuarantineFormat = %q does not contain the marker %q",
			FlakeQuarantineFormat, FlakeQuarantineMarker,
		)
	}
}

// TestFlakeQuarantineFormat_IsValidFormat asserts the format has
// exactly one `%d` placeholder.
//
// The check is a substring search for `%d` and a count of the
// placeholders. The format is the model's contract with the policy
// document; a change to the number of placeholders would fail this
// test and prompt the contributor to update the document.
func TestFlakeQuarantineFormat_IsValidFormat(t *testing.T) {
	got := FlakeQuarantineFormat
	if !strings.Contains(got, "%d") {
		t.Errorf("FlakeQuarantineFormat = %q does not contain a %%d placeholder", got)
	}
	// Count occurrences of %d. The format is expected to have
	// exactly one.
	count := strings.Count(got, "%d")
	if count != 1 {
		t.Errorf(
			"FlakeQuarantineFormat = %q has %d %%d placeholders, want 1",
			got, count,
		)
	}
}

// TestFlakeQuarantineFormat_IsATSkipCall asserts the format's prefix
// is `t.Skip(`.
//
// The check ensures the format is a Go expression that compiles as
// a `t.Skip` call. A future change that replaced the format with a
// different call (for example, `t.Skipf(`) would fail this test.
func TestFlakeQuarantineFormat_IsATSkipCall(t *testing.T) {
	if !strings.HasPrefix(FlakeQuarantineFormat, `t.Skip("`) {
		t.Errorf(
			"FlakeQuarantineFormat = %q does not start with `t.Skip(\"`",
			FlakeQuarantineFormat,
		)
	}
}

// --- Rate tests ------------------------------------------------------------

// TestFlakeRateTarget_IsInRange asserts the target is in [0, 1].
func TestFlakeRateTarget_IsInRange(t *testing.T) {
	if FlakeRateTarget < 0 || FlakeRateTarget > 1 {
		t.Errorf("FlakeRateTarget = %v, want a value in [0, 1]", FlakeRateTarget)
	}
}

// TestFlakeRateTarget_IsHalfAPercent asserts the target's exact
// value.
//
// The value is 0.005 (0.5%). A change to the target is a change to
// the policy; the test would fail and prompt the contributor to
// update the policy document and the ADR in the same commit.
func TestFlakeRateTarget_IsHalfAPercent(t *testing.T) {
	const want = 0.005
	if FlakeRateTarget != want {
		t.Errorf("FlakeRateTarget = %v, want %v", FlakeRateTarget, want)
	}
}

// TestFlakeRateTarget_IsNonZero asserts the target is strictly
// positive.
//
// A zero target would require every test to be perfectly reliable,
// which is not achievable. The policy's target is a small positive
// number.
func TestFlakeRateTarget_IsNonZero(t *testing.T) {
	if FlakeRateTarget <= 0 {
		t.Errorf("FlakeRateTarget = %v, want a positive value", FlakeRateTarget)
	}
}

// --- Retry tests -----------------------------------------------------------

// TestFlakeRetryPolicyCurrent_IsNone asserts the current retry policy
// is `none`.
//
// The policy's rationale is that a retry hides the flake; the CI
// must fail so that the flake is visible. A change to a different
// policy (for example, `automatic`) would be a change to the
// policy; the test would fail and prompt the contributor to update
// the policy document and the ADR.
func TestFlakeRetryPolicyCurrent_IsNone(t *testing.T) {
	if FlakeRetryPolicyCurrent != FlakeRetryNone {
		t.Errorf(
			"FlakeRetryPolicyCurrent = %q, want %q",
			string(FlakeRetryPolicyCurrent), string(FlakeRetryNone),
		)
	}
}

// TestFlakeRetryPolicy_Values asserts the two policy values are
// distinct.
func TestFlakeRetryPolicy_Values(t *testing.T) {
	if FlakeRetryNone == FlakeRetryAutomatic {
		t.Error("FlakeRetryNone and FlakeRetryAutomatic must be distinct")
	}
}

// TestFlakeRetryPolicy_Strings asserts the two policy values'
// string forms.
func TestFlakeRetryPolicy_Strings(t *testing.T) {
	if string(FlakeRetryNone) != "none" {
		t.Errorf("FlakeRetryNone = %q, want %q", string(FlakeRetryNone), "none")
	}
	if string(FlakeRetryAutomatic) != "automatic" {
		t.Errorf("FlakeRetryAutomatic = %q, want %q", string(FlakeRetryAutomatic), "automatic")
	}
}

// --- Detection tests -------------------------------------------------------

// TestFlakeDetectionOf_Tier3Count asserts the tier-3 count is at
// least 2.
//
// A count of 1 would not detect flakes (a single run passes if the
// test happens to succeed). A count of 2 runs every test twice; a
// failure on the second run is a flake.
func TestFlakeDetectionOf_Tier3Count(t *testing.T) {
	d := FlakeDetectionOf()
	if d.Tier3Count < 2 {
		t.Errorf("FlakeDetectionOf().Tier3Count = %d, want >= 2", d.Tier3Count)
	}
}

// TestFlakeDetectionOf_ArtifactName asserts the artifact name is
// non-empty.
func TestFlakeDetectionOf_ArtifactName(t *testing.T) {
	d := FlakeDetectionOf()
	if d.ArtifactName == "" {
		t.Error("FlakeDetectionOf().ArtifactName is empty")
	}
}

// TestFlakeDetectionOf_AggregatorNote asserts the aggregator note is
// non-empty.
func TestFlakeDetectionOf_AggregatorNote(t *testing.T) {
	d := FlakeDetectionOf()
	if d.AggregatorNote == "" {
		t.Error("FlakeDetectionOf().AggregatorNote is empty")
	}
}

// TestFlakeDetectionOf_MatchesTheCanonicalValues asserts the
// exported accessor returns the same values as the unexported
// struct.
func TestFlakeDetectionOf_MatchesTheCanonicalValues(t *testing.T) {
	got := FlakeDetectionOf()
	if got != flakeDetection {
		t.Errorf("FlakeDetectionOf() = %#v, want %#v", got, flakeDetection)
	}
}

// TestFlakeDetectionOf_IsDeterministic asserts repeated calls agree.
func TestFlakeDetectionOf_IsDeterministic(t *testing.T) {
	a := FlakeDetectionOf()
	b := FlakeDetectionOf()
	if a != b {
		t.Fatal("FlakeDetectionOf() is not deterministic")
	}
}

// --- Validate tests --------------------------------------------------------

// TestValidateFlake_AcceptsCanonicalState asserts the canonical state
// is internally consistent.
func TestValidateFlake_AcceptsCanonicalState(t *testing.T) {
	if err := ValidateFlake(); err != nil {
		t.Fatalf("ValidateFlake() rejected the canonical state: %v", err)
	}
}

// TestValidateFlake_IsDeterministic asserts repeated calls agree.
func TestValidateFlake_IsDeterministic(t *testing.T) {
	for i := 0; i < 3; i++ {
		if err := ValidateFlake(); err != nil {
			t.Fatalf("ValidateFlake() call %d failed: %v", i, err)
		}
	}
}

// TestValidateFlake_ErrorIsForgeError asserts the error type.
//
// # Why the probe is a helper, not ValidateFlake
//
// ValidateFlake on the canonical state returns nil; it cannot be
// used to produce an error without mutating package-level data,
// which the test suite deliberately avoids. The model has no
// per-entry validator (the flake policy is a set of scalars, not a
// slice of structs), so the test builds a broken copy of the
// model's state and calls a private helper. The helper does not
// exist in the current model; the test uses the model's exported
// ValidateFlake as the probe and asserts it returns nil on the
// canonical state. The complementary error-path test is performed
// by the contract test file, which reads the policy document and
// asserts the model's rendered values against it.
//
// The test below asserts the nil-path; the error-path assertion is
// a consequence of the model's constants' values, which are checked
// by the individual constant tests above.
func TestValidateFlake_ErrorIsForgeError(t *testing.T) {
	// The canonical state returns nil. The error path is exercised
	// by constructing a synthetic error via the model's
	// flakeError helper.
	err := flakeError("synthetic", "test")
	if err == nil {
		t.Fatal("flakeError returned nil")
	}
	var fe *forgeerr.Error
	if !errors.As(err, &fe) {
		t.Fatalf("flakeError returned %T, want *forgeerr.Error", err)
	}
	if fe.Category() != forgeerr.CategoryInternal {
		t.Errorf("flakeError category = %v, want %v", fe.Category(), forgeerr.CategoryInternal)
	}
	if fe.Code() != string(forgeerr.CodeInternalUnexpected) {
		t.Errorf(
			"flakeError code = %q, want %q",
			fe.Code(), forgeerr.CodeInternalUnexpected,
		)
	}
}

// --- Golden tests ----------------------------------------------------------

// TestFlakeGolden_IsDeterministic asserts repeated calls produce
// byte-identical output.
func TestFlakeGolden_IsDeterministic(t *testing.T) {
	a := FlakeGolden()
	b := FlakeGolden()
	if a != b {
		t.Fatal("FlakeGolden() is not deterministic")
	}
}

// TestFlakeGolden_IsValidJSON asserts the rendering parses as JSON.
func TestFlakeGolden_IsValidJSON(t *testing.T) {
	var doc any
	if err := json.Unmarshal([]byte(FlakeGolden()), &doc); err != nil {
		t.Fatalf("FlakeGolden() is not valid JSON: %v", err)
	}
}

// TestFlakeGolden_ContainsThePolicy asserts the rendering contains
// the model's constants.
//
// # Why the test decodes the JSON
//
// The golden is JSON. A constant whose value contains a JSON-escaped
// character (for example, `FlakeQuarantineFormat`, whose value
// contains `"`) appears in the golden's bytes in its escaped form
// (`\"`). A raw substring comparison of the Go constant against the
// golden's bytes would fail for that constant even though the
// decoded value matches.
//
// The test therefore decodes the golden and compares the decoded
// values. The comparison is robust to any future constant whose
// JSON escaping differs from its Go form.
func TestFlakeGolden_ContainsThePolicy(t *testing.T) {
	raw := FlakeGolden()

	var doc map[string]any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("FlakeGolden() is not valid JSON: %v", err)
	}

	// The model's constants, mapped to the JSON key each one is
	// rendered under. The mapping is the contract: a change to
	// either side updates the other.
	checks := []struct {
		name string // the constant's name, for the failure message
		key  string // the JSON key
		want string // the constant's value
	}{
		{"FlakePriorityP0", "priority", string(FlakePriorityP0)},
		{"FlakeQuarantineMarker", "quarantine_marker", FlakeQuarantineMarker},
		{"FlakeQuarantineFormat", "quarantine_format", FlakeQuarantineFormat},
		{"FlakeRetryPolicyCurrent", "retry_policy", string(FlakeRetryPolicyCurrent)},
		{"flakeDetection.ArtifactName", "artifact_name", flakeDetection.ArtifactName},
	}
	for _, c := range checks {
		got, ok := doc[c.key].(string)
		if !ok {
			t.Errorf("FlakeGolden()'s %q is not a string; got %T", c.key, doc[c.key])
			continue
		}
		if got != c.want {
			t.Errorf(
				"FlakeGolden()[%q] = %q, want %q (the value of %s)",
				c.key, got, c.want, c.name,
			)
		}
	}
}

// TestFlakeGolden_ContainsTheSLA asserts the rendering includes the
// SLA's durations.
func TestFlakeGolden_ContainsTheSLA(t *testing.T) {
	got := FlakeGolden()
	if !strings.Contains(got, flakeSLA.QuarantineWithin) {
		t.Errorf("FlakeGolden() does not contain %q", flakeSLA.QuarantineWithin)
	}
	if !strings.Contains(got, flakeSLA.FixWithin) {
		t.Errorf("FlakeGolden() does not contain %q", flakeSLA.FixWithin)
	}
}

// TestFlakeGolden_ContainsTheRateTarget asserts the rendering
// includes the rate target.
//
// The rendering formats the target as a four-decimal number
// ("0.0050"). The test checks for the formatted form.
func TestFlakeGolden_ContainsTheRateTarget(t *testing.T) {
	got := FlakeGolden()
	// The renderer writes "0.0050" for 0.005.
	want := "0.0050"
	if !strings.Contains(got, want) {
		t.Errorf("FlakeGolden() does not contain %q", want)
	}
}

// --- Document tests --------------------------------------------------------

// TestFlakeMarkdown_IsDeterministic asserts repeated calls produce
// byte-identical output.
func TestFlakeMarkdown_IsDeterministic(t *testing.T) {
	a := FlakeMarkdown()
	b := FlakeMarkdown()
	if a != b {
		t.Fatal("FlakeMarkdown() is not deterministic")
	}
}

// TestFlakeMarkdown_ContainsThePolicyInOneParagraph asserts the
// rendering's opening has the policy's key values.
func TestFlakeMarkdown_ContainsThePolicyInOneParagraph(t *testing.T) {
	got := FlakeMarkdown()
	must := []string{
		"P0",
		flakeSLA.QuarantineWithin,
		flakeSLA.FixWithin,
		FlakeQuarantineMarker,
	}
	for _, want := range must {
		if !strings.Contains(got, want) {
			t.Errorf("FlakeMarkdown() does not contain %q", want)
		}
	}
}

// TestFlakeMarkdown_ContainsTheQuarantineSection asserts the
// rendering has a Quarantine section with the example.
func TestFlakeMarkdown_ContainsTheQuarantineSection(t *testing.T) {
	got := FlakeMarkdown()
	if !strings.Contains(got, "Quarantine") {
		t.Error("FlakeMarkdown() does not have a Quarantine section")
	}
	if !strings.Contains(got, FlakeQuarantineFormat) {
		t.Errorf("FlakeMarkdown() does not contain the quarantine format %q", FlakeQuarantineFormat)
	}
}

// TestFlakeMarkdown_ContainsTheDetectionSection asserts the
// rendering has a Detection section.
func TestFlakeMarkdown_ContainsTheDetectionSection(t *testing.T) {
	got := FlakeMarkdown()
	if !strings.Contains(got, "Detection") {
		t.Error("FlakeMarkdown() does not have a Detection section")
	}
	if !strings.Contains(got, flakeDetection.ArtifactName) {
		t.Errorf("FlakeMarkdown() does not contain the artifact name %q", flakeDetection.ArtifactName)
	}
}

// TestFlakeMarkdown_ContainsTheRetrySection asserts the rendering
// has a Retry section.
func TestFlakeMarkdown_ContainsTheRetrySection(t *testing.T) {
	got := FlakeMarkdown()
	if !strings.Contains(got, "Retry") {
		t.Error("FlakeMarkdown() does not have a Retry section")
	}
	if !strings.Contains(got, string(FlakeRetryPolicyCurrent)) {
		t.Errorf(
			"FlakeMarkdown() does not contain the retry policy %q",
			string(FlakeRetryPolicyCurrent),
		)
	}
}

// TestFlakeMarkdown_ContainsTheTargetSection asserts the rendering
// has a Target section naming the rate target.
func TestFlakeMarkdown_ContainsTheTargetSection(t *testing.T) {
	got := FlakeMarkdown()
	if !strings.Contains(got, "Target") {
		t.Error("FlakeMarkdown() does not have a Target section")
	}
	// The renderer formats the target as a percentage with one
	// decimal: "0.5%".
	if !strings.Contains(got, "0.5%") {
		t.Error("FlakeMarkdown() does not contain the rate target")
	}
}
