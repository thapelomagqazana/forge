package ci

// security_test.go — internal tests for the Tier 4 security model.
//
// The tests in this file are package-internal (`package ci`, not
// `package ci_test`) because several of them inspect the package's
// own unexported state:
//
//   - The closure tests assert on securityJobs directly, which is
//     the point of the closure: adding a member must fail a test,
//     and the test that fails must be able to name the canonical
//     slice.
//   - The constant tests inspect the model's exported constants
//     (SecurityScorecardThreshold, SecurityForbiddenLicenses,
//     SecuritySchedule, SecuritySBOMRetention,
//     SecurityToolVersions), which are tied to the model.
//
// The tests that read the YAML workflow file live in
// security_contract_test.go and are external (`package ci_test`),
// because they treat the package as a black box and assert on the
// artifact rather than the model.
//
// # The security model
//
// The model declares eight jobs:
//
//   - security-codeql       — Go CodeQL analysis; SARIF upload.
//   - security-govulncheck  — Go vulnerability database scan.
//   - security-gitleaks     — secret scan on the full history.
//   - security-grype        — vulnerability scan of the release
//                             artifacts.
//   - security-scorecard    — OpenSSF Scorecard; fails below the
//                             threshold.
//   - security-license-scan — license compatibility check.
//   - security-sbom         — SBOM generation with syft.
//   - security-dependabot   — verify Dependabot is configured.
//
// The model pins the job names, the job timeouts, the Scorecard
// threshold, the forbidden-license list, the schedule, the SBOM
// retention, and the tool versions.
//
// # Test taxonomy
//
//   - Closure tests: the set has exactly the members it is supposed
//     to have.
//   - Order tests: the canonical order is stable and the two
//     representations agree.
//   - Copy tests: returned values do not alias the canonical state.
//   - Job-name tests: the membership table and String totality.
//   - Lookup tests: totality and the zero value.
//   - Validate tests: the invariants, table-driven on the broken
//     shape.
//   - Constant tests: the model's pinned values.
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

// TestAllSecurityJobs_HasExactlyEight asserts the closed set has the
// eight canonical job names.
//
// The test is intentionally brittle. A ninth job is a change to the
// security strategy, and it must be accompanied by an update to the
// ADR and to the workflow file.
func TestAllSecurityJobs_HasExactlyEight(t *testing.T) {
	got := AllSecurityJobs()
	if len(got) != 8 {
		t.Fatalf("AllSecurityJobs() returned %d job names, want 8", len(got))
	}
}

// TestAllSecurityJobDescriptions_HasExactlyEight asserts the
// canonical description has the same eight members as the canonical
// order.
func TestAllSecurityJobDescriptions_HasExactlyEight(t *testing.T) {
	got := AllSecurityJobDescriptions()
	if len(got) != 8 {
		t.Fatalf("AllSecurityJobDescriptions() returned %d jobs, want 8", len(got))
	}
}

// --- Order tests -----------------------------------------------------------

// TestAllSecurityJobs_OrderIsStable asserts the canonical order.
//
// The order is the order in which the jobs are presented in the
// security workflow and in the reference document: CodeQL,
// govulncheck, gitleaks, grype, scorecard, license-scan, sbom,
// dependabot. Reordering the slice without reordering the workflow
// and the document would produce artifacts that describe a different
// strategy from the one the code declares.
func TestAllSecurityJobs_OrderIsStable(t *testing.T) {
	want := []SecurityJobName{
		SecurityJobCodeQL,
		SecurityJobGovulncheck,
		SecurityJobGitleaks,
		SecurityJobGrype,
		SecurityJobScorecard,
		SecurityJobLicenseScan,
		SecurityJobSBOM,
		SecurityJobDependabot,
	}
	got := AllSecurityJobs()
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AllSecurityJobs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestAllSecurityJobDescriptions_OrderMatchesAllSecurityJobs asserts
// the two representations of the closed set agree on order.
func TestAllSecurityJobDescriptions_OrderMatchesAllSecurityJobs(t *testing.T) {
	names := AllSecurityJobs()
	jobs := AllSecurityJobDescriptions()
	if len(names) != len(jobs) {
		t.Fatalf(
			"AllSecurityJobs() has %d entries but AllSecurityJobDescriptions() has %d",
			len(names), len(jobs),
		)
	}
	for i := range names {
		if jobs[i].Name != names[i] {
			t.Errorf(
				"AllSecurityJobDescriptions()[%d].Name = %q, want %q",
				i, jobs[i].Name, names[i],
			)
		}
	}
}

// --- Copy tests ------------------------------------------------------------

// TestAllSecurityJobs_ReturnsFreshSlice asserts the accessor does not
// expose the canonical slice.
func TestAllSecurityJobs_ReturnsFreshSlice(t *testing.T) {
	a := AllSecurityJobs()
	a[0] = "mutated"
	b := AllSecurityJobs()
	if b[0] == "mutated" {
		t.Fatal("AllSecurityJobs() returned an aliased slice")
	}
}

// TestAllSecurityJobDescriptions_ReturnsDeepCopy asserts the
// accessor deep-copies each SecurityJob.
//
// The SecurityJob struct contains one slice field (Permissions). A
// shallow copy would share the backing array; a caller that appended
// to or truncated it would corrupt the canonical set for the rest of
// the process.
//
// # Why the loop variable is named j, not t
//
// The test's parameter is *testing.T, conventionally named t. A loop
// over the returned []SecurityJob that used t as the loop variable
// would shadow the test handle, and any call to t.Fatalf inside the
// loop body would fail to compile ("t.Fatalf undefined (type
// SecurityJob has no field or method Fatalf)"). The loop variable is
// named j, which is the same convention the package's source files
// use for a SecurityJob value.
func TestAllSecurityJobDescriptions_ReturnsDeepCopy(t *testing.T) {
	first := AllSecurityJobDescriptions()
	for i := range first {
		for k := range first[i].Permissions {
			first[i].Permissions[k] = "mutated-permission"
		}
		first[i].Note = "mutated-note"
	}

	second := AllSecurityJobDescriptions()
	for _, j := range second {
		for _, p := range j.Permissions {
			if p == "mutated-permission" {
				t.Fatalf("job %q: Permissions aliased the canonical slice", j.Name)
			}
		}
		if j.Note == "mutated-note" {
			t.Fatalf("job %q: Note aliased the canonical value", j.Name)
		}
	}
}

// TestLookupSecurityJob_ReturnsDeepCopy asserts the single-entry
// accessor deep-copies too.
func TestLookupSecurityJob_ReturnsDeepCopy(t *testing.T) {
	j, ok := LookupSecurityJob(SecurityJobCodeQL)
	if !ok {
		t.Fatal("LookupSecurityJob(SecurityJobCodeQL) not found")
	}
	for k := range j.Permissions {
		j.Permissions[k] = "mutated-permission"
	}
	j.Note = "mutated-note"

	again, ok := LookupSecurityJob(SecurityJobCodeQL)
	if !ok {
		t.Fatal("LookupSecurityJob(SecurityJobCodeQL) not found on second call")
	}
	for _, p := range again.Permissions {
		if p == "mutated-permission" {
			t.Fatal("LookupSecurityJob returned an aliased Permissions slice")
		}
	}
	if again.Note == "mutated-note" {
		t.Fatal("LookupSecurityJob returned an aliased Note")
	}
}

// --- Job-name tests --------------------------------------------------------

// TestSecurityJobName_Valid is the membership table for the closed
// set.
func TestSecurityJobName_Valid(t *testing.T) {
	cases := []struct {
		name SecurityJobName
		want bool
	}{
		{SecurityJobCodeQL, true},
		{SecurityJobGovulncheck, true},
		{SecurityJobGitleaks, true},
		{SecurityJobGrype, true},
		{SecurityJobScorecard, true},
		{SecurityJobLicenseScan, true},
		{SecurityJobSBOM, true},
		{SecurityJobDependabot, true},
		{"", false},
		{"unknown", false},
		{"SECURITY-CODEQL", false},
		{"security_codeql", false},
	}
	for _, tc := range cases {
		if got := tc.name.Valid(); got != tc.want {
			t.Errorf("SecurityJobName(%q).Valid() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestSecurityJobName_AllMembersAreValid asserts the closed set is
// self-consistent.
func TestSecurityJobName_AllMembersAreValid(t *testing.T) {
	valid := 0
	for _, name := range AllSecurityJobs() {
		if !name.Valid() {
			t.Errorf("AllSecurityJobs() member %q is not Valid()", name)
		}
		valid++
	}
	if valid != len(securityJobs) {
		t.Fatalf("counted %d valid names but securityJobs has %d", valid, len(securityJobs))
	}
}

// TestSecurityJobName_String_IsTotal asserts String never panics,
// including on an unknown name.
func TestSecurityJobName_String_IsTotal(t *testing.T) {
	cases := []SecurityJobName{"", "unknown", SecurityJobCodeQL, SecurityJobDependabot}
	for _, c := range cases {
		got := c.String()
		if got != string(c) {
			t.Errorf("SecurityJobName(%q).String() = %q, want %q", c, got, string(c))
		}
	}
}

// --- Lookup tests ----------------------------------------------------------

// TestLookupSecurityJob_FindsEveryJob asserts LookupSecurityJob is
// total over the closed set.
func TestLookupSecurityJob_FindsEveryJob(t *testing.T) {
	for _, name := range AllSecurityJobs() {
		j, ok := LookupSecurityJob(name)
		if !ok {
			t.Errorf("LookupSecurityJob(%q) reported not found", name)
			continue
		}
		if j.Name != name {
			t.Errorf("LookupSecurityJob(%q).Name = %q", name, j.Name)
		}
		if j.Timeout <= 0 {
			t.Errorf("LookupSecurityJob(%q).Timeout = %d, want positive", name, j.Timeout)
		}
		if len(j.Permissions) == 0 {
			t.Errorf("LookupSecurityJob(%q).Permissions is empty", name)
		}
		if j.Note == "" {
			t.Errorf("LookupSecurityJob(%q).Note is empty", name)
		}
	}
}

// TestLookupSecurityJob_UnknownReturnsZero asserts LookupSecurityJob
// does not panic and returns the zero SecurityJob for an unknown
// name.
func TestLookupSecurityJob_UnknownReturnsZero(t *testing.T) {
	j, ok := LookupSecurityJob("nope")
	if ok {
		t.Fatal("LookupSecurityJob(\"nope\") reported found")
	}
	if !isZeroSecurityJob(j) {
		t.Fatalf("LookupSecurityJob(\"nope\") returned %#v, want the zero SecurityJob", j)
	}
}

// TestLookupSecurityJob_EmptyNameReturnsZero asserts the empty
// string is not a valid job name.
func TestLookupSecurityJob_EmptyNameReturnsZero(t *testing.T) {
	j, ok := LookupSecurityJob("")
	if ok {
		t.Fatal("LookupSecurityJob(\"\") reported found")
	}
	if !isZeroSecurityJob(j) {
		t.Fatalf("LookupSecurityJob(\"\") returned %#v, want the zero SecurityJob", j)
	}
}

// --- Validate tests --------------------------------------------------------

// TestValidateSecurity_AcceptsCanonicalState asserts the canonical
// state is internally consistent.
func TestValidateSecurity_AcceptsCanonicalState(t *testing.T) {
	if err := ValidateSecurity(); err != nil {
		t.Fatalf("ValidateSecurity() rejected the canonical state: %v", err)
	}
}

// TestValidateSecurity_IsDeterministic asserts repeated calls agree.
func TestValidateSecurity_IsDeterministic(t *testing.T) {
	for i := 0; i < 3; i++ {
		if err := ValidateSecurity(); err != nil {
			t.Fatalf("ValidateSecurity() call %d failed: %v", i, err)
		}
	}
}

// TestValidateSecurity_ErrorIsForgeError asserts the error type is
// the structured one.
//
// # Why the probe is validateSecurityJob, not ValidateSecurity
//
// ValidateSecurity on the canonical state returns nil; it cannot be
// used to produce an error without mutating package-level data,
// which the test suite deliberately avoids. validateSecurityJob is
// the per-entry validator that ValidateSecurity calls internally;
// probing it on a broken entry exercises the same error-construction
// path.
func TestValidateSecurity_ErrorIsForgeError(t *testing.T) {
	err := validateSecurityJob(SecurityJob{})
	if err == nil {
		t.Fatal("validateSecurityJob on the zero SecurityJob returned nil")
	}
	var fe *forgeerr.Error
	if !errors.As(err, &fe) {
		t.Fatalf("validateSecurityJob returned %T, want *forgeerr.Error", err)
	}
	if fe.Category() != forgeerr.CategoryInternal {
		t.Errorf("validateSecurityJob category = %v, want %v", fe.Category(), forgeerr.CategoryInternal)
	}
	if fe.Code() != string(forgeerr.CodeInternalUnexpected) {
		t.Errorf("validateSecurityJob code = %q, want %q", fe.Code(), forgeerr.CodeInternalUnexpected)
	}
}

// validSecurityJob returns a SecurityJob that passes every check.
// Each test starts from this value and breaks exactly one property.
func validSecurityJob() SecurityJob {
	return SecurityJob{
		Name:    SecurityJobCodeQL,
		Timeout: 60,
		Permissions: []Permission{
			PermissionContentsRead,
			PermissionSecurityEventsWrite,
		},
		UsesSARIF:         true,
		ArtifactRetention: 0,
		Note:              "test note",
	}
}

// TestValidateSecurityJob_AcceptsValidEntry asserts the baseline is
// accepted.
func TestValidateSecurityJob_AcceptsValidEntry(t *testing.T) {
	if err := validateSecurityJob(validSecurityJob()); err != nil {
		t.Fatalf("validateSecurityJob rejected the baseline: %v", err)
	}
}

// TestValidateSecurityJob_RejectsBrokenEntries is the table of
// broken shapes.
func TestValidateSecurityJob_RejectsBrokenEntries(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(j *SecurityJob)
		wantSub string
	}{
		{
			name: "unknown name",
			mutate: func(j *SecurityJob) {
				j.Name = "not-a-security-job"
			},
			wantSub: "unknown job name",
		},
		{
			name: "zero timeout",
			mutate: func(j *SecurityJob) {
				j.Timeout = 0
			},
			wantSub: "non-positive timeout",
		},
		{
			name: "negative timeout",
			mutate: func(j *SecurityJob) {
				j.Timeout = -1
			},
			wantSub: "non-positive timeout",
		},
		{
			name: "no permissions",
			mutate: func(j *SecurityJob) {
				j.Permissions = nil
			},
			wantSub: "no permissions",
		},
		{
			name: "negative retention",
			mutate: func(j *SecurityJob) {
				j.ArtifactRetention = -1
			},
			wantSub: "negative artifact retention",
		},
		{
			name: "empty note",
			mutate: func(j *SecurityJob) {
				j.Note = ""
			},
			wantSub: "empty note",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			j := validSecurityJob()
			tc.mutate(&j)
			err := validateSecurityJob(j)
			if err == nil {
				t.Fatal("validateSecurityJob accepted a broken entry")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("validateSecurityJob error = %q, want it to contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// TestValidateSecurityJob_IsTotalOnEveryMember asserts the canonical
// jobs all pass the per-entry validator.
func TestValidateSecurityJob_IsTotalOnEveryMember(t *testing.T) {
	for _, j := range AllSecurityJobDescriptions() {
		if err := validateSecurityJob(j); err != nil {
			t.Errorf("validateSecurityJob rejected canonical job %q: %v", j.Name, err)
		}
	}
}

// --- Constant tests --------------------------------------------------------

// TestSecurityScorecardThreshold_IsInRange asserts the threshold is
// in [0, 10].
func TestSecurityScorecardThreshold_IsInRange(t *testing.T) {
	if SecurityScorecardThreshold < 0 || SecurityScorecardThreshold > 10 {
		t.Errorf(
			"SecurityScorecardThreshold = %v, want a value in [0, 10]",
			SecurityScorecardThreshold,
		)
	}
}

// TestSecurityScorecardThreshold_IsSeventeenTenths asserts the exact
// threshold.
//
// The value is 7.0 per the task's KPI. A change to the threshold is a
// change to the policy and must be accompanied by an update to the
// ADR.
func TestSecurityScorecardThreshold_IsSeven(t *testing.T) {
	const want = 7.0
	if SecurityScorecardThreshold != want {
		t.Errorf("SecurityScorecardThreshold = %v, want %v", SecurityScorecardThreshold, want)
	}
}

// TestSecurityForbiddenLicenses_IsNonEmpty asserts the list has at
// least one entry.
func TestSecurityForbiddenLicenses_IsNonEmpty(t *testing.T) {
	if len(SecurityForbiddenLicenses) == 0 {
		t.Error("SecurityForbiddenLicenses is empty")
	}
}

// TestSecurityForbiddenLicenses_ContainsCopyleft asserts the list
// contains the licenses the task names.
func TestSecurityForbiddenLicenses_ContainsCopyleft(t *testing.T) {
	must := []string{"GPL-2.0", "GPL-3.0", "AGPL-3.0"}
	for _, m := range must {
		found := false
		for _, l := range SecurityForbiddenLicenses {
			if l == m {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SecurityForbiddenLicenses does not contain %q", m)
		}
	}
}

// TestSecurityForbiddenLicenses_NoDuplicates asserts the list has no
// duplicate entries.
func TestSecurityForbiddenLicenses_NoDuplicates(t *testing.T) {
	seen := make(map[string]bool, len(SecurityForbiddenLicenses))
	for _, l := range SecurityForbiddenLicenses {
		if seen[l] {
			t.Errorf("SecurityForbiddenLicenses contains %q more than once", l)
			continue
		}
		seen[l] = true
	}
}

// TestSecuritySchedule_IsDailyAtTwo asserts the exact cron schedule.
func TestSecuritySchedule_IsDailyAtTwo(t *testing.T) {
	const want = "0 2 * * *"
	if SecuritySchedule != want {
		t.Errorf("SecuritySchedule = %q, want %q", SecuritySchedule, want)
	}
}

// TestSecuritySBOMRetention_Is90Days asserts the exact retention.
func TestSecuritySBOMRetention_Is90Days(t *testing.T) {
	if SecuritySBOMRetention != 90 {
		t.Errorf("SecuritySBOMRetention = %d, want 90", SecuritySBOMRetention)
	}
}

// TestSecurityToolVersions_AllNonEmpty asserts every tool has a
// non-empty version pin.
func TestSecurityToolVersions_AllNonEmpty(t *testing.T) {
	if len(SecurityToolVersions) == 0 {
		t.Fatal("SecurityToolVersions is empty")
	}
	for name, version := range SecurityToolVersions {
		if version == "" {
			t.Errorf("SecurityToolVersions[%q] is empty", name)
		}
	}
}

// TestSecurityToolVersions_ContainsEveryTool asserts the map names
// the tools the workflow uses.
func TestSecurityToolVersions_ContainsEveryTool(t *testing.T) {
	must := []string{"govulncheck", "gitleaks", "grype", "go-licenses", "syft"}
	for _, name := range must {
		if _, ok := SecurityToolVersions[name]; !ok {
			t.Errorf("SecurityToolVersions does not name %q", name)
		}
	}
}

// --- Golden tests ----------------------------------------------------------

// TestSecurityGolden_IsDeterministic asserts repeated calls produce
// byte-identical output.
func TestSecurityGolden_IsDeterministic(t *testing.T) {
	a := SecurityGolden()
	b := SecurityGolden()
	if a != b {
		t.Fatal("SecurityGolden() is not deterministic")
	}
}

// TestSecurityGolden_IsValidJSON asserts the rendering parses as
// JSON.
func TestSecurityGolden_IsValidJSON(t *testing.T) {
	var doc any
	if err := json.Unmarshal([]byte(SecurityGolden()), &doc); err != nil {
		t.Fatalf("SecurityGolden() is not valid JSON: %v", err)
	}
}

// TestSecurityGolden_ListsEveryJob asserts every job name appears in
// the rendering.
func TestSecurityGolden_ListsEveryJob(t *testing.T) {
	got := SecurityGolden()
	for _, name := range AllSecurityJobs() {
		if !strings.Contains(got, string(name)) {
			t.Errorf("SecurityGolden() does not contain job name %q", name)
		}
	}
}

// TestSecurityGolden_ContainsThresholds asserts the rendering
// includes the threshold, the schedule, and the retention.
func TestSecurityGolden_ContainsThresholds(t *testing.T) {
	got := SecurityGolden()
	if !strings.Contains(got, SecuritySchedule) {
		t.Errorf("SecurityGolden() does not contain the schedule %q", SecuritySchedule)
	}
	if !strings.Contains(got, "scorecard_threshold") {
		t.Error("SecurityGolden() does not contain the scorecard_threshold key")
	}
	if !strings.Contains(got, "sbom_retention_days") {
		t.Error("SecurityGolden() does not contain the sbom_retention_days key")
	}
}

// --- Document tests --------------------------------------------------------

// TestSecurityMarkdown_IsDeterministic asserts repeated calls produce
// byte-identical output.
func TestSecurityMarkdown_IsDeterministic(t *testing.T) {
	a := SecurityMarkdown()
	b := SecurityMarkdown()
	if a != b {
		t.Fatal("SecurityMarkdown() is not deterministic")
	}
}

// TestSecurityMarkdown_ListsEveryJob asserts every job name appears
// in the rendering.
func TestSecurityMarkdown_ListsEveryJob(t *testing.T) {
	got := SecurityMarkdown()
	for _, name := range AllSecurityJobs() {
		if !strings.Contains(got, string(name)) {
			t.Errorf("SecurityMarkdown() does not contain job name %q", name)
		}
	}
}

// TestSecurityMarkdown_ContainsTheSchedule asserts the rendering
// names the cron schedule.
func TestSecurityMarkdown_ContainsTheSchedule(t *testing.T) {
	got := SecurityMarkdown()
	if !strings.Contains(got, SecuritySchedule) {
		t.Errorf("SecurityMarkdown() does not contain the schedule %q", SecuritySchedule)
	}
}

// TestSecurityMarkdown_ContainsTheThresholds asserts the rendering
// names the Scorecard threshold and the forbidden licenses.
func TestSecurityMarkdown_ContainsTheThresholds(t *testing.T) {
	got := SecurityMarkdown()
	if !strings.Contains(got, "Scorecard") {
		t.Error("SecurityMarkdown() does not have a Scorecard section")
	}
	if !strings.Contains(got, "Thresholds") {
		t.Error("SecurityMarkdown() does not have a Thresholds section")
	}
	for _, l := range SecurityForbiddenLicenses {
		if !strings.Contains(got, l) {
			t.Errorf("SecurityMarkdown() does not contain license %q", l)
		}
	}
}

// TestSecurityMarkdown_ContainsEveryToolVersion asserts the rendering
// names every tool version.
func TestSecurityMarkdown_ContainsEveryToolVersion(t *testing.T) {
	got := SecurityMarkdown()
	for name, version := range SecurityToolVersions {
		if !strings.Contains(got, name) {
			t.Errorf("SecurityMarkdown() does not contain tool %q", name)
		}
		if !strings.Contains(got, version) {
			t.Errorf("SecurityMarkdown() does not contain version %q for tool %q", version, name)
		}
	}
}

// --- helpers ---------------------------------------------------------------

// isZeroSecurityJob reports whether j is the zero value of
// SecurityJob.
//
// The check is written by hand rather than with reflect.DeepEqual so
// that adding a field to SecurityJob forces the helper to be
// updated.
func isZeroSecurityJob(j SecurityJob) bool {
	return j.Name == "" &&
		j.Timeout == 0 &&
		j.Permissions == nil &&
		!j.UsesSARIF &&
		j.ArtifactRetention == 0 &&
		j.Note == ""
}
