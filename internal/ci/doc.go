// Package ci models the five-workflow CI architecture for the forge
// repository.
//
// The package exists because CI YAML is code: it has triggers, permissions,
// secrets, and a contract with the repository's required-checks policy. Like
// any other contract in this repository, it is expressed as a typed Go model
// and pinned by tests. The YAML files under .github/workflows/ are the
// executable artifact; this package is the specification they must satisfy.
//
// # The five workflows
//
//  1. ci.yml       — PR + main push. Fast feedback. No secrets.
//  2. release.yml  — Tag-triggered. Secrets. Owned by 17.5.3.
//  3. security.yml — Scheduled daily. CodeQL, govulncheck, secret scan,
//     license scan.
//  4. nightly.yml  — Scheduled nightly. Fuzz (1h), race detector (full
//     suite), long-running tests, matrix expansion.
//  5. docs.yml     — PR + push. Markdown lint, link check, docs build.
//
// # Invariants enforced by tests
//
//   - Every workflow declares an explicit top-level `permissions:` block.
//   - Every `uses:` pins a full 40-hex-character SHA, never a tag.
//   - No workflow uses `pull_request_target`.
//   - Required checks are drawn only from ci.yml (Tier 1 + Tier 2).
//   - Security findings are advisory, never blocking.
//   - Dependabot groups all github-actions updates into a single weekly PR.
//
// # Ownership
//
// Each workflow has a named owner and a review policy. Ownership is part of
// the model so it can be checked into the repository and audited.
package ci
