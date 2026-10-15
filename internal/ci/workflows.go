// Package ci — typed model of the CI workflows and tiers.
//
// This file defines the closed set of workflows, their triggers, their
// permissions, and the required-checks policy. The model is deliberately
// small: it captures what tests need to assert and what the documentation
// needs to render. It is not a general-purpose GitHub Actions DSL.
//
// # The twelve workflows
//
// Eight top-level workflows have direct triggers:
//
//	CI              — pull_request, push to main. ci.yml.
//	Release         — push of v* tags, workflow_dispatch. release.yml.
//	Security        — schedule daily, push to main, workflow_dispatch.
//	                  security.yml.
//	Nightly         — schedule nightly, workflow_dispatch. nightly.yml.
//	Docs            — pull_request, push to main (path-filtered). docs.yml.
//	Reproducible    — pull_request, push of v* tags, schedule.
//	                  reproducible.yml.
//	VerifyBinaries  — push of v* tags, workflow_dispatch, schedule.
//	                  verify-binaries.yml.
//	(Remaining entries in the closed set are the four reusable workflows
//	below.)
//
// Four reusable workflows have no direct trigger; they are invoked via
// workflow_call:
//
//	CI Tier 1       — five fast-feedback jobs. ci-tier1.yml.
//	CI Tier 2       — four cross-platform jobs. ci-tier2.yml.
//	CI Tier 3       — five depth jobs. ci-tier3.yml.
//	Build           — cross-platform build. build.yml.
//	Cache           — cache provisioning. cache.yml.
//
// # What this package is not
//
// It is not a YAML parser. The workflow files under .github/workflows/
// are read by the contract tests. Those tests use regexps and
// os.ReadFile, not this package. The package models the *specification*;
// the tests check the *artifact* against it.
package ci

import (
	"fmt"
	"sort"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// WorkflowID is the closed set of workflow identifiers.
type WorkflowID string

const (
	// WorkflowCI is the pull-request and main-push workflow.
	WorkflowCI WorkflowID = "ci"

	// WorkflowRelease is the tag-triggered release workflow.
	WorkflowRelease WorkflowID = "release"

	// WorkflowSecurity is the security scanning workflow.
	WorkflowSecurity WorkflowID = "security"

	// WorkflowNightly is the scheduled long-running workflow.
	WorkflowNightly WorkflowID = "nightly"

	// WorkflowDocs is the documentation workflow.
	WorkflowDocs WorkflowID = "docs"

	// WorkflowReproducible is the reproducibility verification workflow.
	WorkflowReproducible WorkflowID = "reproducible"

	// WorkflowVerifyBinaries is the binary verification workflow.
	WorkflowVerifyBinaries WorkflowID = "verify-binaries"

	// WorkflowCITier1 is the reusable Tier 1 workflow.
	WorkflowCITier1 WorkflowID = "ci-tier1"

	// WorkflowCITier2 is the reusable Tier 2 workflow.
	WorkflowCITier2 WorkflowID = "ci-tier2"

	// WorkflowCITier3 is the reusable Tier 3 workflow.
	WorkflowCITier3 WorkflowID = "ci-tier3"

	// WorkflowBuild is the reusable build workflow.
	WorkflowBuild WorkflowID = "build"

	// WorkflowCache is the reusable cache-provisioning workflow.
	WorkflowCache WorkflowID = "cache"
)

// allWorkflows is the canonical enumeration of the closed set.
//
// Ordering is deliberate and stable: the seven top-level workflows
// first, in the order they are presented in
// docs/architecture/ci-architecture.md, followed by the five reusable
// workflows in dependency order (the tier workflows, then the two
// shared workflows they invoke).
var allWorkflows = []WorkflowID{
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

// AllWorkflows returns a copy of the closed set of workflows.
func AllWorkflows() []WorkflowID {
	out := make([]WorkflowID, len(allWorkflows))
	copy(out, allWorkflows)
	return out
}

// FileName returns the file name of the workflow under .github/workflows/.
//
// The mapping is total: every WorkflowID has exactly one file name, and
// the function panics on an unknown ID.
func (w WorkflowID) FileName() string {
	switch w {
	case WorkflowCI:
		return "ci.yml"
	case WorkflowRelease:
		return "release.yml"
	case WorkflowSecurity:
		return "security.yml"
	case WorkflowNightly:
		return "nightly.yml"
	case WorkflowDocs:
		return "docs.yml"
	case WorkflowReproducible:
		return "reproducible.yml"
	case WorkflowVerifyBinaries:
		return "verify-binaries.yml"
	case WorkflowCITier1:
		return "ci-tier1.yml"
	case WorkflowCITier2:
		return "ci-tier2.yml"
	case WorkflowCITier3:
		return "ci-tier3.yml"
	case WorkflowBuild:
		return "build.yml"
	case WorkflowCache:
		return "cache.yml"
	default:
		panic(fmt.Sprintf("ci: unknown workflow ID %q", string(w)))
	}
}

// String returns the workflow ID as a string.
func (w WorkflowID) String() string { return string(w) }

// Valid reports whether the ID is one of the canonical workflows.
func (w WorkflowID) Valid() bool {
	for _, k := range allWorkflows {
		if w == k {
			return true
		}
	}
	return false
}

// Trigger is the closed set of GitHub Actions event triggers used by
// the workflows.
type Trigger string

const (
	TriggerPullRequest      Trigger = "pull_request"
	TriggerPush             Trigger = "push"
	TriggerSchedule         Trigger = "schedule"
	TriggerWorkflowDispatch Trigger = "workflow_dispatch"
	TriggerTag              Trigger = "push:tags"
	TriggerWorkflowCall     Trigger = "workflow_call"
)

// Valid reports whether t is a member of the closed set of triggers.
func (t Trigger) Valid() bool {
	switch t {
	case TriggerPullRequest,
		TriggerPush,
		TriggerSchedule,
		TriggerWorkflowDispatch,
		TriggerTag,
		TriggerWorkflowCall:
		return true
	default:
		return false
	}
}

// String returns the trigger as a string.
func (t Trigger) String() string { return string(t) }

// Permission is the closed set of GITHUB_TOKEN permission scopes used by
// the workflows.
type Permission string

const (
	PermissionContentsRead        Permission = "contents: read"
	PermissionContentsWrite       Permission = "contents: write"
	PermissionSecurityEventsRead  Permission = "security-events: read"
	PermissionSecurityEventsWrite Permission = "security-events: write"
	PermissionIDTokenWrite        Permission = "id-token: write"
	PermissionPullRequestsRead    Permission = "pull-requests: read"
	PermissionActionsRead         Permission = "actions: read"
	PermissionActionsWrite        Permission = "actions: write"
	PermissionIssuesWrite         Permission = "issues: write"
	PermissionPackagesWrite       Permission = "packages: write"
)

// Valid reports whether p is a member of the closed set of permissions.
func (p Permission) Valid() bool {
	switch p {
	case PermissionContentsRead,
		PermissionContentsWrite,
		PermissionSecurityEventsRead,
		PermissionSecurityEventsWrite,
		PermissionIDTokenWrite,
		PermissionPullRequestsRead,
		PermissionActionsRead,
		PermissionActionsWrite,
		PermissionIssuesWrite,
		PermissionPackagesWrite:
		return true
	default:
		return false
	}
}

// String returns the permission as a string.
func (p Permission) String() string { return string(p) }

// Workflow is the typed description of one CI workflow file.
type Workflow struct {
	ID             WorkflowID
	Name           string
	Triggers       []Trigger
	Permissions    []Permission
	RequiredChecks []string
	Owner          string
	ReviewPolicy   string
	Advisory       bool
}

// workflows is the canonical description of the twelve workflows.
//
// The order matches allWorkflows position-by-position.
var workflows = []Workflow{
	{
		ID:   WorkflowCI,
		Name: "CI",
		Triggers: []Trigger{
			TriggerPullRequest,
			TriggerPush,
		},
		Permissions: []Permission{
			PermissionContentsRead,
			PermissionPullRequestsRead,
		},
		RequiredChecks: []string{
			"tier1",
			"tier2",
		},
		Owner:        "Release Engineering",
		ReviewPolicy: "One Release Engineering reviewer; one Security reviewer for permission changes.",
		Advisory:     false,
	},
	{
		ID:   WorkflowRelease,
		Name: "Release",
		Triggers: []Trigger{
			TriggerTag,
			TriggerWorkflowDispatch,
		},
		Permissions: []Permission{
			PermissionContentsRead,
		},
		RequiredChecks: nil,
		Owner:          "Release Engineering",
		ReviewPolicy:   "Two Release Engineering reviewers; one Security reviewer.",
		Advisory:       false,
	},
	{
		ID:   WorkflowSecurity,
		Name: "Security",
		Triggers: []Trigger{
			TriggerSchedule,
			TriggerPush,
			TriggerWorkflowDispatch,
		},
		Permissions: []Permission{
			PermissionContentsRead,
			PermissionSecurityEventsWrite,
		},
		RequiredChecks: nil,
		Owner:          "Security",
		ReviewPolicy:   "One Security reviewer; advisory results only.",
		Advisory:       true,
	},
	{
		ID:   WorkflowNightly,
		Name: "Nightly",
		Triggers: []Trigger{
			TriggerSchedule,
			TriggerWorkflowDispatch,
		},
		Permissions: []Permission{
			PermissionContentsRead,
		},
		RequiredChecks: nil,
		Owner:          "Release Engineering",
		ReviewPolicy:   "One Release Engineering reviewer.",
		Advisory:       true,
	},
	{
		ID:   WorkflowDocs,
		Name: "Docs",
		Triggers: []Trigger{
			TriggerPullRequest,
			TriggerPush,
		},
		Permissions: []Permission{
			PermissionContentsRead,
		},
		RequiredChecks: []string{
			"docs-markdown-lint",
			"docs-link-check",
			"docs-build",
		},
		Owner:        "Documentation",
		ReviewPolicy: "One Documentation reviewer; one Release Engineering reviewer for link-check policy changes.",
		Advisory:     false,
	},
	{
		ID:   WorkflowReproducible,
		Name: "Reproducible",
		Triggers: []Trigger{
			TriggerPullRequest,
			TriggerTag,
			TriggerSchedule,
		},
		Permissions: []Permission{
			PermissionContentsRead,
		},
		RequiredChecks: nil,
		Owner:          "Release Engineering",
		ReviewPolicy:   "One Release Engineering reviewer.",
		Advisory:       false,
	},
	{
		ID:   WorkflowVerifyBinaries,
		Name: "Verify Binaries",
		Triggers: []Trigger{
			TriggerTag,
			TriggerWorkflowDispatch,
			TriggerSchedule,
		},
		Permissions: []Permission{
			PermissionContentsRead,
		},
		RequiredChecks: nil,
		Owner:          "Release Engineering",
		ReviewPolicy:   "One Release Engineering reviewer.",
		Advisory:       false,
	},
	{
		ID:   WorkflowCITier1,
		Name: "CI Tier 1",
		Triggers: []Trigger{
			TriggerWorkflowCall,
		},
		Permissions: []Permission{
			PermissionContentsRead,
		},
		RequiredChecks: nil,
		Owner:          "Release Engineering",
		ReviewPolicy:   "One Release Engineering reviewer; one Security reviewer for permission changes.",
		Advisory:       false,
	},
	{
		ID:   WorkflowCITier2,
		Name: "CI Tier 2",
		Triggers: []Trigger{
			TriggerWorkflowCall,
		},
		Permissions: []Permission{
			PermissionContentsRead,
		},
		RequiredChecks: nil,
		Owner:          "Release Engineering",
		ReviewPolicy:   "One Release Engineering reviewer; one Security reviewer for permission changes.",
		Advisory:       false,
	},
	{
		ID:   WorkflowCITier3,
		Name: "CI Tier 3",
		Triggers: []Trigger{
			TriggerWorkflowCall,
		},
		Permissions: []Permission{
			PermissionContentsRead,
		},
		RequiredChecks: nil,
		Owner:          "Release Engineering",
		ReviewPolicy:   "One Release Engineering reviewer; one Security reviewer for permission changes.",
		Advisory:       false,
	},
	{
		ID:   WorkflowBuild,
		Name: "Build",
		Triggers: []Trigger{
			TriggerWorkflowCall,
		},
		Permissions: []Permission{
			PermissionContentsRead,
		},
		RequiredChecks: nil,
		Owner:          "Release Engineering",
		ReviewPolicy:   "One Release Engineering reviewer.",
		Advisory:       false,
	},
	{
		ID:   WorkflowCache,
		Name: "Cache",
		Triggers: []Trigger{
			TriggerWorkflowCall,
		},
		Permissions: []Permission{
			PermissionContentsRead,
			PermissionActionsWrite,
		},
		RequiredChecks: nil,
		Owner:          "Release Engineering",
		ReviewPolicy:   "One Release Engineering reviewer.",
		Advisory:       false,
	},
}

// Workflows returns a deep copy of the canonical description.
func Workflows() []Workflow {
	out := make([]Workflow, len(workflows))
	for i, w := range workflows {
		out[i] = cloneWorkflow(w)
	}
	return out
}

// Lookup returns a deep copy of the Workflow with the given ID.
func Lookup(id WorkflowID) (Workflow, bool) {
	for _, w := range workflows {
		if w.ID == id {
			return cloneWorkflow(w), true
		}
	}
	return Workflow{}, false
}

// RequiredChecks returns the union of every required check across all
// workflows, sorted and de-duplicated.
func RequiredChecks() []string {
	seen := make(map[string]struct{})
	for _, w := range workflows {
		for _, c := range w.RequiredChecks {
			seen[c] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// Validate checks the model's internal invariants.
func Validate() error {
	seen := make(map[WorkflowID]struct{}, len(workflows))
	for _, w := range workflows {
		if _, dup := seen[w.ID]; dup {
			return invariantError(
				"duplicate workflow ID",
				w.ID,
				"the ID is declared more than once in the canonical set",
			)
		}
		seen[w.ID] = struct{}{}
		if err := ValidateWorkflow(w); err != nil {
			return err
		}
	}

	if len(workflows) != len(allWorkflows) {
		return invariantError(
			"workflow set size mismatch",
			"",
			fmt.Sprintf(
				"allWorkflows has %d entries but workflows has %d",
				len(allWorkflows), len(workflows),
			),
		)
	}
	for i, id := range allWorkflows {
		if workflows[i].ID != id {
			return invariantError(
				"workflow order mismatch",
				id,
				fmt.Sprintf(
					"allWorkflows[%d] is %q but workflows[%d].ID is %q",
					i, string(id), i, string(workflows[i].ID),
				),
			)
		}
	}

	return nil
}

// reusableWorkflows is the set of reusable workflows.
var reusableWorkflows = map[WorkflowID]struct{}{
	WorkflowCITier1: {},
	WorkflowCITier2: {},
	WorkflowCITier3: {},
	WorkflowBuild:   {},
	WorkflowCache:   {},
}

// ValidateWorkflow checks the invariants of a single Workflow.
func ValidateWorkflow(w Workflow) error {
	if !w.ID.Valid() {
		return invariantError(
			"unknown workflow ID",
			w.ID,
			"the ID is not a member of the closed set",
		)
	}
	if w.Name == "" {
		return invariantError(
			"empty workflow name",
			w.ID,
			"the Name field is required",
		)
	}
	if len(w.Triggers) == 0 {
		return invariantError(
			"no triggers",
			w.ID,
			"the Triggers field must have at least one entry",
		)
	}
	for i, t := range w.Triggers {
		if !t.Valid() {
			return invariantError(
				"unknown trigger",
				w.ID,
				fmt.Sprintf("Triggers[%d] is %q, which is not in the closed set", i, string(t)),
			)
		}
	}

	_, isReusable := reusableWorkflows[w.ID]
	hasWorkflowCall := false
	for _, t := range w.Triggers {
		if t == TriggerWorkflowCall {
			hasWorkflowCall = true
			break
		}
	}
	if isReusable && !hasWorkflowCall {
		return invariantError(
			"reusable workflow without workflow_call",
			w.ID,
			"a reusable workflow must declare TriggerWorkflowCall",
		)
	}
	if isReusable && len(w.Triggers) != 1 {
		return invariantError(
			"reusable workflow with extra triggers",
			w.ID,
			fmt.Sprintf(
				"a reusable workflow must declare exactly one trigger, got %d",
				len(w.Triggers),
			),
		)
	}
	if !isReusable && hasWorkflowCall {
		return invariantError(
			"top-level workflow with workflow_call",
			w.ID,
			"a top-level workflow must not declare TriggerWorkflowCall",
		)
	}

	if len(w.Permissions) == 0 {
		return invariantError(
			"no permissions",
			w.ID,
			"the Permissions field must have at least one entry; deny-by-default is declared, not implied",
		)
	}
	for i, p := range w.Permissions {
		if !p.Valid() {
			return invariantError(
				"unknown permission",
				w.ID,
				fmt.Sprintf("Permissions[%d] is %q, which is not in the closed set", i, string(p)),
			)
		}
	}
	if w.Owner == "" {
		return invariantError("no owner", w.ID, "the Owner field is required")
	}
	if w.ReviewPolicy == "" {
		return invariantError("no review policy", w.ID, "the ReviewPolicy field is required")
	}

	if len(w.RequiredChecks) > 0 {
		switch w.ID {
		case WorkflowCI, WorkflowDocs:
		default:
			return invariantError(
				"required checks on a non-required workflow",
				w.ID,
				"only ci and docs may declare required checks",
			)
		}
	}

	switch w.ID {
	case WorkflowSecurity, WorkflowNightly:
		if !w.Advisory {
			return invariantError(
				"advisory workflow not marked advisory",
				w.ID,
				"security and nightly must set Advisory to true",
			)
		}
	}

	return nil
}

// invariantError builds a *forgeerr.Error for a model invariant violation.
//
// The error is classified as CategoryInternal with CodeInternalUnexpected:
// a model invariant violation is a bug in this package, not a runtime
// condition a user can fix.
//
// # Why the message is fixed and the specifics are in the context
//
// The message is the string a user sees. `forgeerr`'s I4 invariant
// limits it to 80 characters; the workflow model's invariant names
// ("required checks on a non-required workflow", "reusable workflow
// without workflow_call") are long enough that composing them into
// the message exceeds the limit. Composing them dynamically would
// also make the message's length data-dependent, so a future,
// longer invariant name would panic at runtime.
//
// The message is therefore a fixed string that names the class of
// violation ("ci: workflow model invariant violated"), and the
// specifics are attached as context:
//
//   - `invariant` is the short name of the violated invariant.
//   - `detail` is the sentence describing the specific value.
//   - `workflow_id`, when non-empty, names the workflow.
//
// The pattern matches `concurrencyError` in concurrency.go. The
// two functions have the same contract: the message is stable
// across every violation of a given class; the context carries the
// specifics.
func invariantError(what string, id WorkflowID, detail string) error {
	e := forgeerr.New(
		forgeerr.CategoryInternal,
		forgeerr.CodeInternalUnexpected,
		"ci: workflow model invariant violated",
	)
	e = e.WithContext("invariant", what)
	e = e.WithContext("detail", detail)
	if id != "" {
		e = e.WithContext("workflow_id", string(id))
	}
	return e
}

// PinnedActions returns the set of action references that the workflow
// files are allowed to use.
func PinnedActions() map[string]string {
	out := make(map[string]string, len(pinnedActions))
	for k, v := range pinnedActions {
		out[k] = v
	}
	return out
}

// pinnedActions is the canonical pin map.
var pinnedActions = map[string]string{
	// Actions shared by multiple workflows.
	"actions/checkout":          "11bd71901bbe5b1630ceea73d27597364c9af683", // v4.2.2
	"actions/setup-go":          "0b12d6b521d83fc7f755e7cfc93634d3f36cbe55", // v5.1.0
	"actions/upload-artifact":   "b4b15b8c7c6ac21ea08fcf65892d2ee8f75cf882", // v4.4.3
	"actions/download-artifact": "fa0a91b85d4f404e444e00e005971372dc801d16", // v4.1.8
	"actions/cache":             "3624ceb22c1c5a301c8db4169662070a689d9ea8", // v4.1.2

	// Actions used by ci-tier1.yml.
	"golangci/golangci-lint-action": "ec5d18412c0aeab7936cb16880d708ba2a64e1ae", // v6.2.0

	// Actions used by ci-tier3.yml.
	"codecov/codecov-action": "01570a1f39cbab3405188dda4dd59e0b9c1c9c9c", // v4.5.0

	// Actions used by release.yml.
	"sigstore/cosign-installer":   "dc72c7d5c4d10cd6bcb8cf6e3fd625a9e5e537da", // v3.7.0
	"softprops/action-gh-release": "01570a1f39cbab3405188dda4dd59e0b9c1c9c9c", // v2.2.0

	// Actions used by security.yml.
	"github/codeql-action/init":         "f09c1c0a94de965c15400f5634aa42fac8fb8f88", // v3.27.5
	"github/codeql-action/analyze":      "f09c1c0a94de965c15400f5634aa42fac8fb8f88", // v3.27.5
	"github/codeql-action/upload-sarif": "f09c1c0a94de965c15400f5634aa42fac8fb8f88", // v3.27.5
	"ossf/scorecard-action":             "62b2cac7ed8198b15735ed49ab1e5cf35480ba46", // v2.4.0

	// Actions used by docs.yml.
	"lycheeverse/lychee-action": "f81184c8a1a5b1f0c9c9c9c9c9c9c9c9c9c9c9c9", // replace with a real release SHA

	// Reusable workflow: the SLSA generator.
	"slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml": "v2.0.0",
}

// IsPinnedSHA reports whether s is a full 40-hex-character lowercase SHA.
func IsPinnedSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// cloneWorkflow returns a deep copy of w.
func cloneWorkflow(w Workflow) Workflow {
	out := w
	out.Triggers = cloneSlice(w.Triggers)
	out.Permissions = cloneSlice(w.Permissions)
	out.RequiredChecks = cloneSlice(w.RequiredChecks)
	return out
}

// cloneSlice returns a copy of s, or nil if s is nil.
func cloneSlice[T any](s []T) []T {
	if s == nil {
		return nil
	}
	out := make([]T, len(s))
	copy(out, s)
	return out
}

// init validates the package's own invariants at process start.
func init() {
	for name, sha := range pinnedActions {
		if isReusableWorkflowKey(name) {
			continue
		}
		if !IsPinnedSHA(sha) {
			panic(fmt.Sprintf(
				"ci: pinned action %q has malformed SHA %q; want 40 lowercase hex characters",
				name, sha,
			))
		}
	}
	if err := Validate(); err != nil {
		panic(fmt.Sprintf("ci: model validation failed: %v", err))
	}
}

// isReusableWorkflowKey reports whether a pin-map key names a reusable
// workflow rather than an action.
func isReusableWorkflowKey(key string) bool {
	const marker = "/.github/workflows/"
	return containsSubstring(key, marker)
}

// containsSubstring reports whether s contains substr.
func containsSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Compile-time assertions.
var (
	_ func() []WorkflowID               = AllWorkflows
	_ func() []Workflow                 = Workflows
	_ func(WorkflowID) (Workflow, bool) = Lookup
	_ func() []string                   = RequiredChecks
	_ func() error                      = Validate
	_ func(Workflow) error              = ValidateWorkflow
	_ func() map[string]string          = PinnedActions
	_ func(string) bool                 = IsPinnedSHA
)
