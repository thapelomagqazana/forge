// Package ci — typed model of the CI concurrency policy.
//
// This file defines the concurrency policy that the workflows
// declare. Every workflow has a `concurrency:` block with two
// fields: a group and a cancel-in-progress policy. The group ensures
// that runs of the same workflow on the same ref serialize; the
// policy decides whether a superseding run cancels the run it
// supersedes.
//
// # The two policies
//
// The repository uses two concurrency policies:
//
//   - Cancel-superseded-on-PR (ci.yml, ci-tier1/2/3.yml, docs.yml).
//     A new push to a PR supersedes the previous push's run. The
//     superseded run is no longer useful: its input is stale and its
//     result is irrelevant. Cancelling it frees a runner for the new
//     run. On main pushes, the superseding run does NOT cancel:
//     every commit to main must be verified, even if a later commit
//     has already been pushed.
//
//   - Never-cancel (release.yml, security.yml, nightly.yml). These
//     workflows are triggered by tags or a schedule; their runs are
//     not superseded by a later run in the same way. A release run
//     is the only run for its tag; a scheduled run is the only run
//     for its slot. Cancelling one wastes its work.
//
// # Why the policy is modelled
//
// The concurrency block is three YAML lines. It looks like
// configuration, not like code. But it has invariants:
//
//   - Every PR-triggered workflow has a cancel-in-progress policy.
//   - No main-push-triggered workflow has `cancel-in-progress: true`.
//   - The group template contains a discriminator and a scope, so
//     that two runs of the same workflow on the same ref serialize
//     and runs of different workflows on the same ref do not
//     collide.
//
// The invariants are testable, and the test's failure names the
// workflow and the violated property.
package ci

import (
	"fmt"
	"strings"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// CancelPolicy is the closed set of cancel-in-progress policies.
type CancelPolicy string

const (
	// CancelPolicyNever means the workflow's `cancel-in-progress` is
	// always false. A superseding run waits for the run it
	// supersedes.
	CancelPolicyNever CancelPolicy = "never"

	// CancelPolicyOnPR means the workflow's `cancel-in-progress` is
	// an expression that is true for any ref that is not
	// `refs/heads/main` and false for `refs/heads/main`. A
	// superseding PR run cancels the run it supersedes; a
	// superseding main push does not.
	CancelPolicyOnPR CancelPolicy = "on-pr"
)

// Valid reports whether p is a member of the closed set.
func (p CancelPolicy) Valid() bool {
	switch p {
	case CancelPolicyNever, CancelPolicyOnPR:
		return true
	default:
		return false
	}
}

// String returns the policy as a string.
func (p CancelPolicy) String() string { return string(p) }

// ConcurrencySpec is the typed description of one workflow's
// concurrency block.
type ConcurrencySpec struct {
	// Workflow is the workflow ID the spec applies to.
	Workflow WorkflowID

	// GroupTemplate is the value of the `group:` field. It may
	// contain GitHub Actions substitution syntax. The template is a
	// contract: the workflow file's group must match it.
	GroupTemplate string

	// CancelExpression is the value of the `cancel-in-progress:`
	// field, or the empty string if the field is a literal boolean.
	//
	// When the field is a literal, CancelLiteral is set instead.
	CancelExpression string

	// CancelLiteral is the value of the `cancel-in-progress:` field
	// when the field is a literal boolean. It is used only when
	// CancelExpression is empty.
	CancelLiteral bool

	// Policy is the semantic policy the spec encodes. It is derived
	// from CancelExpression and CancelLiteral by the ValidateSpec
	// function; a contributor does not write it directly.
	Policy CancelPolicy

	// Rationale is a one-sentence explanation of why the workflow
	// uses this policy. It appears in the concurrency-policy
	// document.
	Rationale string
}

// concurrencySpecs is the canonical list of concurrency specs, one
// per workflow.
//
// The order matches AllWorkflows(): the seven top-level workflows
// first, then the five reusable workflows. The golden file and the
// rendered document depend on this order.
var concurrencySpecs = []ConcurrencySpec{
	{
		Workflow:         WorkflowCI,
		GroupTemplate:    "ci-${{ github.workflow }}-${{ github.ref }}",
		CancelExpression: "${{ github.ref != 'refs/heads/main' }}",
		Policy:           CancelPolicyOnPR,
		Rationale: "A superseded PR push is stale; cancelling it frees a runner for the new run. " +
			"A superseded main push still needs verification.",
	},
	{
		Workflow:      WorkflowRelease,
		GroupTemplate: "release-${{ github.workflow }}-${{ inputs.version || github.ref }}",
		CancelLiteral: false,
		Policy:        CancelPolicyNever,
		Rationale: "A tag is immutable; a second run for the same tag is a retry or a bug. " +
			"Serializing is the safe default. The group carries `github.workflow` so a " +
			"release run cannot share a concurrency group with a tier run on the same ref, " +
			"matching the shape every other workflow's group follows.",
	},
	{
		Workflow:      WorkflowSecurity,
		GroupTemplate: "security-${{ github.workflow }}-${{ github.ref }}",
		CancelLiteral: false,
		Policy:        CancelPolicyNever,
		Rationale: "Scheduled runs are the only run for their slot; cancelling one wastes its work. " +
			"Push-triggered runs are rare and their inputs change infrequently.",
	},
	{
		Workflow:      WorkflowNightly,
		GroupTemplate: "nightly-${{ github.workflow }}-${{ github.ref }}",
		CancelLiteral: false,
		Policy:        CancelPolicyNever,
		Rationale:     "Same as security.yml.",
	},
	{
		Workflow:         WorkflowDocs,
		GroupTemplate:    "docs-${{ github.workflow }}-${{ github.ref }}",
		CancelExpression: "${{ github.ref != 'refs/heads/main' }}",
		Policy:           CancelPolicyOnPR,
		Rationale:        "Same as ci.yml: PR pushes supersede; main pushes do not.",
	},
	{
		Workflow:      WorkflowReproducible,
		GroupTemplate: "reproducible-${{ github.workflow }}-${{ github.ref }}",
		CancelLiteral: false,
		Policy:        CancelPolicyNever,
		Rationale: "The workflow is triggered by pull_request, tags, and schedule. " +
			"Tags and schedules are the triggers whose runs must not cancel each other; " +
			"a PR's superseded pushes are rare and their cost is bounded. The " +
			"never policy is the conservative choice.",
	},
	{
		Workflow:      WorkflowVerifyBinaries,
		GroupTemplate: "verify-binaries-${{ github.workflow }}-${{ github.ref }}",
		CancelLiteral: false,
		Policy:        CancelPolicyNever,
		Rationale: "Tag pushes and scheduled runs are the only triggers; " +
			"neither is superseded by a later run in the same way a PR push is.",
	},
	{
		Workflow:         WorkflowCITier1,
		GroupTemplate:    "ci-tier1-${{ github.workflow }}-${{ github.ref }}",
		CancelExpression: "${{ github.ref != 'refs/heads/main' }}",
		Policy:           CancelPolicyOnPR,
		Rationale:        "Same as ci.yml: PR pushes supersede; main pushes do not.",
	},
	{
		Workflow:         WorkflowCITier2,
		GroupTemplate:    "ci-tier2-${{ github.workflow }}-${{ github.ref }}",
		CancelExpression: "${{ github.ref != 'refs/heads/main' }}",
		Policy:           CancelPolicyOnPR,
		Rationale:        "Same as ci.yml: PR pushes supersede; main pushes do not.",
	},
	{
		Workflow:         WorkflowCITier3,
		GroupTemplate:    "ci-tier3-${{ github.workflow }}-${{ github.ref }}",
		CancelExpression: "${{ github.ref != 'refs/heads/main' }}",
		Policy:           CancelPolicyOnPR,
		Rationale: "Same as ci.yml: PR pushes supersede; main pushes do not. " +
			"The tier is gated on push, so the policy's PR branch is rarely exercised, but the " +
			"block is present for consistency with the other tiers.",
	},
	{
		Workflow:      WorkflowBuild,
		GroupTemplate: "build-${{ github.workflow }}-${{ github.ref }}",
		CancelLiteral: false,
		Policy:        CancelPolicyNever,
		Rationale: "A reusable workflow invoked by release.yml; the caller " +
			"serializes, and the build itself has no supersession semantics.",
	},
	{
		Workflow:      WorkflowCache,
		GroupTemplate: "cache-${{ github.workflow }}-${{ github.ref }}",
		CancelLiteral: false,
		Policy:        CancelPolicyNever,
		Rationale: "A reusable workflow invoked by every tier; the caller " +
			"serializes, and the cache itself has no supersession semantics.",
	},
}

// AllConcurrencySpecs returns a deep copy of the canonical specs.
func AllConcurrencySpecs() []ConcurrencySpec {
	out := make([]ConcurrencySpec, len(concurrencySpecs))
	copy(out, concurrencySpecs)
	return out
}

// LookupConcurrencySpec returns a deep copy of the spec for the given
// workflow and reports whether it was found.
func LookupConcurrencySpec(id WorkflowID) (ConcurrencySpec, bool) {
	for _, s := range concurrencySpecs {
		if s.Workflow == id {
			return s, true
		}
	}
	return ConcurrencySpec{}, false
}

// ValidateConcurrency checks the model's internal invariants.
//
// The invariants are:
//
//  1. Every workflow in the closed set has a spec.
//  2. Every spec names a member of the closed set of workflows.
//  3. Every spec has a non-empty group template.
//  4. Every spec has either a cancel expression or a literal, not
//     both.
//  5. The cancel expression, if present, is the one the policy
//     expects:
//     - CancelPolicyOnPR expects `${{ github.ref != 'refs/heads/main' }}`.
//     - CancelPolicyNever expects an empty expression.
//  6. The literal, if set, is `false` for CancelPolicyNever.
//  7. Every spec has a non-empty rationale.
//  8. A workflow with a `pull_request` trigger uses CancelPolicyOnPR.
//  9. A workflow with a `schedule` trigger uses CancelPolicyNever.
//  10. A workflow with a `push:tags` trigger uses CancelPolicyNever.
//  11. The group template contains a discriminator (`github.workflow`
//     or `inputs.`) and a scope (`github.ref` or `inputs.`).
func ValidateConcurrency() error {
	seen := make(map[WorkflowID]struct{}, len(concurrencySpecs))
	for _, s := range concurrencySpecs {
		if !s.Workflow.Valid() {
			return concurrencyError(
				"unknown workflow",
				fmt.Sprintf("the workflow %q is not in the closed set", string(s.Workflow)),
			)
		}
		if _, dup := seen[s.Workflow]; dup {
			return concurrencyError(
				"duplicate workflow",
				fmt.Sprintf("the workflow %q has more than one spec", string(s.Workflow)),
			)
		}
		seen[s.Workflow] = struct{}{}
		if err := validateConcurrencySpec(s); err != nil {
			return err
		}
	}
	for _, id := range AllWorkflows() {
		if _, ok := seen[id]; !ok {
			return concurrencyError(
				"missing workflow",
				fmt.Sprintf("the workflow %q has no spec", string(id)),
			)
		}
	}
	return nil
}

// validateConcurrencySpec checks the invariants of a single spec.
func validateConcurrencySpec(s ConcurrencySpec) error {
	if !s.Workflow.Valid() {
		return concurrencyError(
			"unknown workflow",
			fmt.Sprintf("the workflow %q is not in the closed set", string(s.Workflow)),
		)
	}
	if s.GroupTemplate == "" {
		return concurrencyError(
			"empty group template",
			fmt.Sprintf("the workflow %q has no group template", string(s.Workflow)),
		)
	}
	if s.CancelExpression != "" && s.CancelLiteral {
		return concurrencyError(
			"both expression and literal set",
			fmt.Sprintf("the workflow %q has both CancelExpression and CancelLiteral", string(s.Workflow)),
		)
	}
	if s.Rationale == "" {
		return concurrencyError(
			"empty rationale",
			fmt.Sprintf("the workflow %q has no rationale", string(s.Workflow)),
		)
	}
	if !s.Policy.Valid() {
		return concurrencyError(
			"unknown policy",
			fmt.Sprintf("the workflow %q has policy %q", string(s.Workflow), string(s.Policy)),
		)
	}

	// The policy and the expression must agree.
	switch s.Policy {
	case CancelPolicyOnPR:
		want := "${{ github.ref != 'refs/heads/main' }}"
		if s.CancelExpression != want {
			return concurrencyError(
				"on-pr policy with wrong expression",
				fmt.Sprintf(
					"the workflow %q uses policy %q but its expression is %q; want %q",
					string(s.Workflow), string(s.Policy), s.CancelExpression, want,
				),
			)
		}
	case CancelPolicyNever:
		if s.CancelExpression != "" {
			return concurrencyError(
				"never policy with expression",
				fmt.Sprintf(
					"the workflow %q uses policy %q but has an expression %q",
					string(s.Workflow), string(s.Policy), s.CancelExpression,
				),
			)
		}
		if s.CancelLiteral {
			return concurrencyError(
				"never policy with true literal",
				fmt.Sprintf(
					"the workflow %q uses policy %q but has CancelLiteral=true",
					string(s.Workflow), string(s.Policy),
				),
			)
		}
	}

	// The policy must be consistent with the workflow's triggers.
	//
	// The rules are ordered by priority rather than applied
	// independently. A workflow may have more than one trigger class;
	// when it does, the classes are ranked:
	//
	//   1. schedule and push:tags — a run for a slot or a tag is not
	//      superseded by a later run in the same way a PR push is;
	//      cancelling one wastes its work. A workflow with either
	//      trigger uses `never`.
	//
	//   2. pull_request — a superseded PR push is stale; cancelling
	//      it frees a runner for the new run. A workflow whose only
	//      supersedable trigger is pull_request uses `on-pr`.
	//
	//   3. workflow_call — a reusable workflow has no direct trigger.
	//      Its policy is chosen by the model author to match its
	//      caller's needs. Either policy is valid.
	//
	// The priority is what makes the rules satisfiable: a workflow
	// with both pull_request and schedule triggers gets a single
	// verdict (never), not two contradictory ones.
	w, ok := Lookup(s.Workflow)
	if !ok {
		return concurrencyError(
			"workflow not found",
			fmt.Sprintf("Lookup(%q) failed", string(s.Workflow)),
		)
	}
	hasPR := false
	hasSchedule := false
	hasTag := false
	for _, t := range w.Triggers {
		switch t {
		case TriggerPullRequest:
			hasPR = true
		case TriggerSchedule:
			hasSchedule = true
		case TriggerTag:
			hasTag = true
		}
	}

	// Rule 1: any schedule or tag trigger forces `never`.
	if (hasSchedule || hasTag) && s.Policy != CancelPolicyNever {
		return concurrencyError(
			"scheduled or tag-triggered workflow with non-never policy",
			fmt.Sprintf(
				"the workflow %q triggers on schedule or tags but uses policy %q",
				string(s.Workflow), string(s.Policy),
			),
		)
	}

	// Rule 2: a workflow whose only supersedable trigger is
	// pull_request must use `on-pr`.
	if hasPR && !hasSchedule && !hasTag && s.Policy != CancelPolicyOnPR {
		return concurrencyError(
			"PR-only workflow with non-PR policy",
			fmt.Sprintf(
				"the workflow %q triggers on pull_request but uses policy %q",
				string(s.Workflow), string(s.Policy),
			),
		)
	}

	// The group template must contain a discriminator and a scope.
	// The two together ensure that two runs of the same workflow on
	// the same ref serialize, and runs of different workflows on the
	// same ref do not collide.
	//
	// The discriminator is either `github.workflow` (the workflow's
	// name, which distinguishes one workflow's runs from another's)
	// or `inputs.` (a workflow input, which distinguishes one
	// parameterised run from another). Every workflow in the closed
	// set uses `github.workflow`; the release workflow uses both
	// `github.workflow` and `inputs.version`, because its group's
	// purpose is to serialise a dry-run against a real release for
	// the same version *and* to keep release runs from colliding
	// with any other workflow's runs on the same ref.
	//
	// The scope is either `github.ref` (the ref, which scopes the
	// group to a branch or tag) or `inputs.` (a workflow input,
	// which scopes the group to the input's value). The release
	// workflow's group uses both forms in the same expression:
	//
	//	release-${{ github.workflow }}-${{ inputs.version || github.ref }}
	//
	// The expression is a group template, not a discriminator-and-
	// scope pair; the two checks below accept either form for either
	// role.
	if !strings.Contains(s.GroupTemplate, "github.workflow") &&
		!strings.Contains(s.GroupTemplate, "inputs.") {
		return concurrencyError(
			"group template missing discriminator",
			fmt.Sprintf(
				"the workflow %q has group %q; it must include `github.workflow` or `inputs.`",
				string(s.Workflow), s.GroupTemplate,
			),
		)
	}
	if !strings.Contains(s.GroupTemplate, "github.ref") &&
		!strings.Contains(s.GroupTemplate, "inputs.") {
		return concurrencyError(
			"group template missing scope",
			fmt.Sprintf(
				"the workflow %q has group %q; it must include `github.ref` or `inputs.`",
				string(s.Workflow), s.GroupTemplate,
			),
		)
	}

	return nil
}

// ConcurrencyGolden renders the concurrency model as deterministic
// JSON.
func ConcurrencyGolden() string {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(`  "specs": [`)
	for i, s := range concurrencySpecs {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    {\n")
		b.WriteString(`      "workflow": `)
		writeJSONString(&b, string(s.Workflow))
		b.WriteString(",\n")
		b.WriteString(`      "group_template": `)
		writeJSONString(&b, s.GroupTemplate)
		b.WriteString(",\n")
		b.WriteString(`      "cancel_expression": `)
		writeJSONString(&b, s.CancelExpression)
		b.WriteString(",\n")
		b.WriteString(`      "cancel_literal": `)
		if s.CancelLiteral {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		b.WriteString(",\n")
		b.WriteString(`      "policy": `)
		writeJSONString(&b, string(s.Policy))
		b.WriteString(",\n")
		b.WriteString(`      "rationale": `)
		writeJSONString(&b, s.Rationale)
		b.WriteString("\n    }")
	}
	b.WriteString("\n  ]\n}\n")
	return b.String()
}

// ConcurrencyMarkdown renders the concurrency policy as Markdown.
func ConcurrencyMarkdown() string {
	var b strings.Builder

	b.WriteString("# Concurrency Policy\n\n")
	b.WriteString("> **Status:** Accepted\n")
	b.WriteString("> **Owner:** Release Engineering\n")
	b.WriteString("> **Related:** [CI Tiers](ci-tiers.md)\n\n")
	b.WriteString("This document describes the concurrency policy each workflow declares. ")
	b.WriteString("It is rendered from `internal/ci/concurrency.go`; a golden-file test asserts the two agree.\n\n")

	b.WriteString("## The two policies\n\n")
	b.WriteString("The repository uses two concurrency policies:\n\n")
	b.WriteString("- **`on-pr`.** Cancel the superseded run if the ref is not `refs/heads/main`. ")
	b.WriteString("  The superseded PR push is stale; the superseded main push still needs verification.\n")
	b.WriteString("- **`never`.** Never cancel. Used by workflows triggered by tags, ")
	b.WriteString("  schedules, or release inputs.\n\n")

	b.WriteString("## Per-workflow policy\n\n")
	b.WriteString("| Workflow | Group | Cancel | Policy | Rationale |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, s := range concurrencySpecs {
		b.WriteString("| `")
		b.WriteString(string(s.Workflow))
		b.WriteString("` | `")
		b.WriteString(s.GroupTemplate)
		b.WriteString("` | ")
		if s.CancelExpression != "" {
			b.WriteString("`")
			b.WriteString(s.CancelExpression)
			b.WriteString("`")
		} else if s.CancelLiteral {
			b.WriteString("`true`")
		} else {
			b.WriteString("`false`")
		}
		b.WriteString(" | `")
		b.WriteString(string(s.Policy))
		b.WriteString("` | ")
		b.WriteString(s.Rationale)
		b.WriteString(" |\n")
	}
	b.WriteString("\n")

	b.WriteString("## The group template\n\n")
	b.WriteString("Every group template contains a discriminator and a scope. ")
	b.WriteString("The discriminator is either `github.workflow` (the workflow's name) ")
	b.WriteString("or `inputs.` (a workflow input); the scope is either `github.ref` ")
	b.WriteString("(the ref) or `inputs.` (a workflow input). The two together ensure ")
	b.WriteString("that two runs of the same workflow on the same ref serialize, and ")
	b.WriteString("runs of different workflows on the same ref do not collide.\n\n")

	return b.String()
}

// concurrencyError builds a *forgeerr.Error for a concurrency-model
// invariant violation.
//
// The error is classified as CategoryInternal with
// CodeInternalUnexpected: a model invariant violation is a bug in
// this package, not a runtime condition a user can fix.
//
// # Why the message is fixed and the specifics are in the context
//
// The message is the string a user sees. `forgeerr`'s I4 invariant
// limits it to 80 characters; the concurrency model's invariant
// names ("PR-triggered workflow with non-PR policy", etc.) are
// long enough that composing them into the message exceeds the
// limit.
//
// The message is therefore a fixed string that names the class of
// violation ("ci: concurrency model invariant violated"), and the
// specifics are attached as context:
//
//   - `invariant` is the short name of the violated invariant.
//   - `detail` is the sentence describing the specific value.
//
// The pattern matches the workflow model's `invariantError`; the
// difference is that the workflow model's invariant names are short
// enough to fit in the message, while the concurrency model's are
// not. A future change that shortened the invariant names could
// move them back into the message; the current form keeps the
// message stable across every violation.
func concurrencyError(what, detail string) error {
	e := forgeerr.New(
		forgeerr.CategoryInternal,
		forgeerr.CodeInternalUnexpected,
		"ci: concurrency model invariant violated",
	)
	e = e.WithContext("invariant", what)
	e = e.WithContext("detail", detail)
	return e
}

// init validates the concurrency model at package start.
func init() {
	if err := ValidateConcurrency(); err != nil {
		panic(fmt.Sprintf("ci: concurrency model validation failed: %v", err))
	}
}

// Compile-time assertions.
var (
	_ func() []ConcurrencySpec                 = AllConcurrencySpecs
	_ func(WorkflowID) (ConcurrencySpec, bool) = LookupConcurrencySpec
	_ func() error                             = ValidateConcurrency
	_ func() string                            = ConcurrencyGolden
	_ func() string                            = ConcurrencyMarkdown
)
