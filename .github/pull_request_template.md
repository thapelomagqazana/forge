<!--
===============================================================================
Pull Request Template
===============================================================================

This template is applied automatically to every pull request. Fill in
each section. Delete sections that do not apply, but do not delete the
section headers without a reason.

The template has three parts:

  1. Summary — what the PR does and why.
  2. Verification — what the contributor did to verify it works.
  3. Checklists — the mechanical rules that apply to every PR.

The dependency checklist is mandatory for any PR that modifies
go.mod or go.sum. See docs/dependency-policy.md, section
"Dependency change detection" for the policy it enforces.

If a section does not apply, write "N/A" rather than deleting it.
Reviewers use the section structure to navigate the PR.
===============================================================================
-->

# Summary

<!--
A one-paragraph description of what this PR changes and why. The
"what" is visible in the diff; the "why" is not. Focus on the why.

If this PR closes an issue, mention it:

  Closes #123
  Fixes #123
  Resolves #123

GitHub links the PR to the issue automatically.
-->

## Motivation

<!--
Why is this change necessary? What problem does it solve? What would
happen if this PR were not merged?

If the change was proposed in an ADR, link it:

  See docs/decisions/ADR-NNN-title.md

If the change is part of a WBS item, mention it:

  Refs: WBS X.Y.Z
-->

## Approach

<!--
How does the change solve the problem? What design decisions were made
and why? Were alternatives considered?

This section is short for small changes and detailed for large ones.
Skip it for typo fixes and trivial documentation updates.
-->

## Verification

<!--
How did you verify the change works? The reviewer will check that the
verification covers the change's scope.

Examples:

  - `task check` passes
  - `task test:integration` passes
  - Manually verified `forge version` prints the expected output on
    macOS, Linux, and Windows
  - Added a new test for the edge case in `internal/cli/config_test.go`
  - Updated the golden file in `internal/cli/testdata/...`

The reviewer will read this section first. Make it easy to verify the
change without reverse-engineering the diff.
-->

## Screenshots

<!--
If this PR changes any user-facing output (CLI, documentation, error
messages), include before/after screenshots or terminal output.

If not, write "N/A".
-->

## Type of change

<!--
Check the box that applies. Multiple boxes may be checked.
-->

- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to change)
- [ ] Documentation update
- [ ] Refactor (no functional change)
- [ ] Performance improvement
- [ ] Build or CI change
- [ ] Dependency change
- [ ] Other (describe in the Summary section)

## Checklist — General

<!--
Every PR must satisfy these before it can be merged.
-->

- [ ] My code follows the style conventions of this project.
- [ ] I have run `task check` locally and it passes.
- [ ] I have run `task hooks:test` locally if hooks are installed.
- [ ] I have added tests that prove my fix is effective or my feature works.
- [ ] I have updated the documentation to reflect the change.
- [ ] I have read `docs/development.md` and `docs/dependency-policy.md`.
- [ ] My commit messages follow the Conventional Commits format.
- [ ] I have signed off my commits if required by the project.

## Checklist — Dependency changes

<!--
This section applies only if the PR modifies go.mod or go.sum.

If the PR does NOT change dependencies, mark the first box and skip
the rest.

The checklist mirrors the violation types (V1–V5) defined in
docs/dependency-policy.md, section "Dependency change detection".
Every box that applies must be checked. If a box does not apply,
explain why in the "Additional notes" field below.
-->

- [ ] This PR does **not** modify `go.mod` or `go.sum` — skip the rest of this section.

If the PR **does** modify dependencies, confirm each of the following:

### V1 — New direct dependency

- [ ] If this PR adds a new direct dependency, I have created an ADR documenting the addition.
- [ ] The new dependency has an entry in Section 7 (the registry) of `docs/dependency-policy.md`.
- [ ] I have run `task verify:deps` and it passes.

### V2 — Version pin

- [ ] Every `require` directive in `go.mod` is pinned to an exact semantic version (`vX.Y.Z`).
- [ ] No `require` directive uses a range (`>= vX.Y.Z`, `^vX.Y`, or a branch name).

### V3 — `+incompatible` marker

- [ ] No `require` directive contains a `+incompatible` marker.
- [ ] If a `+incompatible` marker is present, I have created an ADR explaining why it is necessary and documenting the migration plan.

### V4 — Pseudo-version

- [ ] No direct dependency in `go.mod` uses a pseudo-version (`v0.0.0-YYYYMMDDHHMMSS-<hash>`).
- [ ] If a pseudo-version is present, I have documented the commit hash and the reason for pinning to it.

### V5 — License

- [ ] I have verified that every new or upgraded dependency has a license compatible with Apache-2.0.
- [ ] The licenses are recorded in Section 7 (the registry) of `docs/dependency-policy.md`.

### General

- [ ] The transitive dependency tree is reviewed in the ADR (for new dependencies).
- [ ] The dependency policy in `docs/dependency-policy.md` is respected.

## Additional notes

<!--
Any other information the reviewer needs. Open questions, follow-up
work, concerns about the approach, requests for specific feedback.

If there is nothing to add, write "None".
-->

---

<!--
===============================================================================
Footer
===============================================================================

By submitting this pull request, I confirm that my contribution is
made under the terms of the Apache-2.0 license that governs this
project.

The reviewer will:
  1. Read the Summary, Motivation, and Approach sections.
  2. Verify the Verification section.
  3. Check the checklists.
  4. Review the diff.
  5. Leave feedback as inline comments or a summary review.

Reviews typically happen within a few days. If a PR is urgent,
mention it in the "Additional notes" section.
===============================================================================
-->