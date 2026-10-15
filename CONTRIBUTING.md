<!--
  Contributing to Forge
  =====================
  Public contract. The contributor-facing companion to:
    - SECURITY.md                 (vulnerability reporting)
    - docs/development.md         (build, test, and release mechanics)
    - docs/documentation-guide.md (writing style and structure)
    - docs/decisions/             (the ADRs this document cites)

  Editing rules:
    1. Every command in this document MUST work against a fresh
       clone of main. A command that fails is a defect in this
       document.
    2. Every policy stated here MUST have a normative source in
       an ADR or a reference document. This document explains and
       links; it does not invent policy.
    3. A policy change MUST update this document and its normative
       source in the same PR.
-->

# Contributing to Forge

Thank you for your interest in Forge. This document explains how to
report a problem, propose a change, and get it merged. It is the
entry point; the linked documents are the contracts.

By participating in this project, you agree to abide by the
[Code of Conduct](CODE_OF_CONDUCT.md). Report unacceptable behavior
to `conduct@forge.dev`.

---

## The fastest paths

| You want to… | Do this |
|---|---|
| Report a bug | Open an issue using the bug template. Include the `forge version --format=json` output. |
| Report a security vulnerability | Follow [`SECURITY.md`](SECURITY.md). **Do not open a public issue.** |
| Propose a feature | Open a discussion or an issue using the feature template. Read §"Proposing a change" first. |
| Fix a typo or a broken link in the docs | Open a PR directly. No issue needed. |
| Fix a bug | Open an issue first if the fix is non-obvious; otherwise open a PR. |
| Add a new command or a new flag | Read §"Proposing a change" first. A new command requires an ADR. |
| Add a new dependency | Read §"Dependencies" first. A new dependency requires a documented review. |

A pull request that ignores these paths is not rejected for the
omission; it is asked to follow the path. The paths exist so that
reviewers can give useful feedback quickly.

---

## Before you start

### Toolchain

Forge pins its toolchain in `go.mod`:

```text
go 1.23.0
toolchain go1.23.4
```

Install the Go version from `go.mod`. The `toolchain` directive
makes the Go command download the exact patch version if your
system's `go` is older; a `go` that is newer than the `toolchain`
line is accepted but the release build uses the pinned version.
See [`docs/development.md`](docs/development.md) §"Toolchain" for
the rationale.

### Repository layout

| Directory | Contains |
|---|---|
| `cmd/` | The `main` packages. One binary per subdirectory. |
| `internal/` | The implementation. Nothing here is importable outside the module. |
| `docs/` | The specifications, the ADRs, and the guides. |
| `schemas/` | The JSON Schemas for the versioned output formats. |
| `scripts/` | The build, release, and dependency tooling. |
| `test/` | The cross-cutting acceptance tests. |

The `internal/` packages follow a clean-architecture layering that
is enforced by `scripts/arch/verify-layers.sh` and by
`internal/arch`. Read
[`docs/architecture/clean-architecture.md`](docs/architecture/clean-architecture.md)
before adding a package.

### Getting the code

```sh
git clone https://github.com/thapelomagqazana/forge.git
cd forge
go build ./...
go test ./...
```

The build must succeed and the tests must pass before you change
anything. A failure on a fresh clone is a defect; report it as a
bug rather than working around it.

### Installing the hooks

```sh
./scripts/install-hooks.sh
```

The hooks run the fast checks (gofmt, `go vet`, the ADR lint, the
layer check) before each commit. They are the same checks CI runs;
catching a failure locally is faster than catching it in a PR.

---

## The contract for a change

Every change to Forge follows the same four-step contract. The
steps are cumulative: a change that skips one is asked to go back.

### 1. It has a traceable requirement

Every change cites a requirement. The requirement is one of:

- A WBS item (`WBS 17.5.2`).
- An acceptance criterion (`AC 11.2.1`).
- A non-functional requirement (`NFR-SECURITY-001`).
- An ADR (`ADR-048`).
- A bug report (the issue number).

The citation goes in the commit message's trailer (see
§"Commit messages") and in the PR description. A change without a
citation is asked for one. The citation is how a future reader
answers "why does this code exist?" without reading the diff.

The requirements register is
[`docs/requirements-register.md`](docs/requirements-register.md).
The ADR index is [`docs/decisions/README.md`](docs/decisions/README.md).

### 2. It is specified before it is implemented

A change that adds a command, a flag, an output format, an error
code, or an exit code is specified in a document before it is
implemented. The specification is one of:

- An ADR, for a decision that is hard to reverse.
- A reference document, for a format or a taxonomy.
- A section in `docs/blueprint-spec.md`, `docs/cli-ux-spec.md`, or
  `docs/component-spec.md`, for a user-facing surface.

The specification and the implementation are in the same PR. A PR
that implements an unspecified surface is asked to add the
specification first. The reason is not process for its own sake:
the specification is what a reviewer reads to decide whether the
implementation is correct, and it is what a future reader reads to
decide whether a change is safe.

### 3. It is tested at the boundary it changes

Every change has a test that fails before the change and passes
after. The test is at the boundary the change affects:

| Change | Test at |
|---|---|
| A new command | `cmd/forge/binary_integration_test.go` (end-to-end) and a unit test in `internal/cli`. |
| A new flag | A unit test in `internal/cli/flags_test.go` and a golden test if the flag affects help. |
| A new error code | `internal/forgeerr/codes_test.go` and the codes golden file. |
| A new exit code | `internal/cli/exitcodes_test.go` and the exit-codes reference. |
| A new output format | A golden test under `internal/app/version/testdata/` or the relevant package's `testdata/`. |
| A new filesystem operation | `internal/filesystem/safefs/` with a property test and a fuzz target. |
| A new log field | `internal/logger/schema_test.go` and the log-record schema. |

A test that asserts the change's absence is not a test. A test
that asserts the implementation's shape (for example, "the
function has this signature") is a test only if the signature is a
frozen contract; the signature tests in `internal/cli` are the
model.

### 4. It is reviewed against the checklist

Every PR is reviewed against the checklist in §"Review checklist".
The reviewer's job is to decide whether the change is correct, not
whether it is desirable; desirability is decided in step 1's
issue or discussion.

---

## Proposing a change

### Bug reports

A bug report includes:

- The Forge version: `forge version --format=json`.
- The operating system and architecture.
- The command you ran.
- What you expected.
- What happened.
- The complete output, including the exit code.

A bug report without the version is asked for it. The version
determines which code paths are reachable; a bug in a release two
minors old may already be fixed.

### Feature requests

A feature request includes:

- The problem you are trying to solve.
- The outcome you want.
- The alternatives you considered.
- Whether the feature is in the current release channel's scope.

Forge's scope is defined in
[`docs/mvp-scope.md`](docs/mvp-scope.md) and
[`docs/phase-2-requirements.md`](docs/phase-2-requirements.md). A
feature outside the scope is not rejected; it is deferred to the
phase that owns it. The `docs/product-discovery.md` document
records the discovery process that sets the scope.

### New commands and flags

A new command or a new global flag requires an ADR. The ADR
records:

- The problem the command solves.
- The command's identity (name, short description, long
  description, usage line).
- The command's flags and their interaction with the global flags.
- The command's output format (text and JSON).
- The command's error codes and exit codes.
- The command's interaction with the stream boundary.

The template is
[`docs/decisions/template.md`](docs/decisions/template.md). The
existing command ADRs
([`ADR-004`](docs/decisions/ADR-004-root-command-identity-and-behavior-contracts.md),
[`ADR-005`](docs/decisions/ADR-005-version-command-structure-output-formats-and-stability-guarantees.md),
[`ADR-006`](docs/decisions/ADR-006-help-and-cli-ux-contracts-invocation-matrix-output-formats-stream-boundary-and-phase-2-presentation-policy.md))
are the model.

A PR that adds a command without an ADR is asked for the ADR
first. The review of the ADR is where the command's design is
decided; the review of the implementation is where its correctness
is decided. Doing both at once produces a review that cannot
separate the two.

### New dependencies

A new direct dependency requires a documented review. The review
records:

- The module's path and version.
- The module's license and its compatibility with Forge's license.
- The module's maintenance status (last release, open issues,
  maintainer count).
- The module's transitive dependency count.
- The alternative considered (a standard-library implementation, a
  smaller module, a vendored copy).
- The module's removal plan, if the dependency is temporary.

The review goes in the PR description. The `scripts/deps/` checks
assert that the module graph contains only the documented direct
and indirect dependencies; a new dependency fails the check until
it is documented. Read
[`docs/dependency-policy.md`](docs/dependency-policy.md) before
proposing a dependency.

### Changes to a frozen surface

A frozen surface is a contract that a change to it would break for
a user. The frozen surfaces are:

- The exit code ranges (`internal/cli/exitcodes.go`).
- The error code registry (`internal/forgeerr/codes.go`).
- The log record schema (`schemas/log-record.schema.json`).
- The version output schema (`schemas/version.schema.json`).
- The filesystem API (`internal/filesystem/api.go`).
- The CLI's global flags and their interactions.

A change to a frozen surface is a MAJOR change. It requires an ADR
that records the migration path and the compatibility policy. The
existing ADRs for the frozen surfaces
([`ADR-012`](docs/decisions/ADR-012-error-code-registry-and-stability-policy.md),
[`ADR-018`](docs/decisions/ADR-018-exit-code-migration-and-stability-policy.md),
[`ADR-039`](docs/decisions/ADR-039-the-filesystem-api-is-frozen-by-a-decision-table.md))
are the model.

A PR that changes a frozen surface without an ADR is closed with a
request for the ADR. This is the one case where a PR is closed
rather than asked to change: the ADR is the decision, and the
implementation cannot be reviewed until the decision is made.

---

## Development workflow

### Branching

Branch from `main`. The branch name follows the pattern:

```text
<type>/<wbs-or-issue>-<short-description>
```

Examples:

```text
feat/17.5.2-supply-chain-integrity
fix/142-broken-pipe-exit-code
docs/148-verifying-releases-guide
```

The `<type>` is one of `feat`, `fix`, `docs`, `refactor`, `test`,
`chore`. The `<wbs-or-issue>` is the WBS item, the acceptance
criterion, or the issue number. The `<short-description>` is
lowercase, hyphenated, and at most five words.

### Committing

Commit messages follow the Conventional Commits format:

```text
<type>(<scope>): <subject>

<body>

<trailers>
```

The `<type>` is the same as the branch's. The `<scope>` is the
package or the document the commit touches (`cli`, `logger`,
`forgeerr`, `release`, `docs`). The `<subject>` is imperative,
lowercase, and at most 72 characters. The `<body>` explains the
change's rationale; a commit that changes behavior without a body
is asked for one. The `<trailers>` cite the requirement:

```text
feat(release): implement the supply-chain integrity pipeline

Add three internal build tools that produce the release's
supply-chain artifacts:
  cmd/forge-internal-checksums  per-binary .sha256 files,
                                SHA256SUMS, SHA256SUMS.asc,
                                SHA256SUMS.gpg
  cmd/forge-internal-sign       cosign keyless signatures,
                                macOS notarization,
                                Windows Authenticode
  cmd/forge-internal-sbom       per-binary CycloneDX 1.5 SBOMs,
                                forge.sbom.json index

The three drivers share scripts/internal/release, which owns the
artifact-naming convention and the dist/ layout.

Traceability: SEC-REL-002 through SEC-REL-010,
NFR-SECURITY-001, ADR-048.
Linked from: Phase 2 WBS 19.0; Phase 6 WBS 22.4.
```

The `scripts/deps/check-pr-deps.sh` script reads the trailers to
verify that the cited requirement exists in the register.

### The pre-commit hook

The hook runs:

1. `gofmt -l` on the staged Go files.
2. `go vet` on the changed packages.
3. `./scripts/adr/lint.sh`.
4. `./scripts/arch/verify-layers.sh`.
5. `go test` on the packages whose files changed.

A hook failure stops the commit. Fix the failure; do not bypass
the hook with `--no-verify`. A commit made with `--no-verify` is
asked to be amended; CI runs the same checks and the PR fails
otherwise.

### Opening the PR

A PR includes:

- The title, in the commit-message format.
- The description, which cites the requirement and summarizes the
  change.
- The test plan, which names the tests that cover the change.
- The checklist from §"Review checklist", filled in.
- The `docs/` change, if the change adds or modifies a user-facing
  surface.

The PR template prompts for each. A PR that does not fill in the
template is asked to.

A PR is a draft until CI passes. Mark it ready for review when:

- CI is green.
- The description is complete.
- The change is squashed to one commit per logical change (see
  §"Squashing").

### CI

CI runs on every push and every PR. The jobs are:

| Job | Checks |
|---|---|
| `build` | `go build ./...` on Linux, macOS, and Windows. |
| `test` | `go test ./...` with the race detector on Linux, macOS, and Windows. |
| `lint` | `gofmt`, `go vet`, `staticcheck`, and the repository's own lints. |
| `arch` | `scripts/arch/verify-layers.sh` and the dependency matrix. |
| `deps` | `scripts/deps/` — the dependency policy checks. |
| `docs` | The documentation guide's checks: links, ADR index, schema sync. |
| `coverage` | The coverage floor for the changed packages. |

A red CI is the PR's responsibility, not the reviewer's. Fix the
failure; do not ask the reviewer to merge around it. A failure
that is flaky is reported as a bug; a flaky test is a defect.

### Squashing

A PR is squashed to one commit per logical change before merge.
A logical change is one that a reviewer would accept or reject as
a unit. A PR that fixes a bug and adds a feature is two logical
changes and is asked to split; a PR that adds a feature and its
test is one logical change and is squashed to one commit.

The squashed commit message is the PR's title and body, edited to
the commit-message format. The reviewer may edit the message
before merging.

### Merging

A PR merges when:

- CI is green.
- One maintainer has approved.
- A change to a frozen surface has two maintainer approvals.
- The PR's description cites a requirement.
- The branch is up to date with `main`.

The merge strategy is squash-and-merge. The `main` branch's
history is linear; a merge commit is not used. The reason is that
a linear history makes `git bisect` and `git log` usable without
`--first-parent`.

---

## Review checklist

A reviewer verifies:

### Correctness

- [ ] The change does what its description says.
- [ ] The change's tests fail before it and pass after it.
- [ ] The change's edge cases are tested: empty input, nil,
      boundary values, concurrent use.
- [ ] The change's error paths are tested.
- [ ] The change's behavior is deterministic: two runs produce
      the same output.

### Contract

- [ ] The change cites a requirement.
- [ ] The change's user-facing surface is specified in a document.
- [ ] The change's error codes and exit codes are registered.
- [ ] The change's output format matches its schema.
- [ ] The change's log records match the log-record schema.
- [ ] The change does not write to a stream the stream boundary
      forbids.

### Layering

- [ ] The change does not introduce an upward import.
- [ ] The change does not add a dependency without a review.
- [ ] The change does not put policy in a primitives package.
- [ ] The change does not put I/O in a domain package.

### Style

- [ ] The change's exported symbols have doc comments.
- [ ] The change's doc comments are complete sentences ending in
      a period.
- [ ] The change's error messages are lowercase and do not end in
      punctuation.
- [ ] The change's commit message follows the format.
- [ ] The change's documentation follows
      [`docs/documentation-guide.md`](docs/documentation-guide.md).

### Tests

- [ ] The change's tests are at the boundary the change affects.
- [ ] The change's tests do not assert the implementation's
      shape, unless the shape is a frozen contract.
- [ ] The change's tests do not sleep or poll; they use a
      deterministic synchronization primitive.
- [ ] The change's tests are parallelizable and marked
      `t.Parallel()` where safe.
- [ ] The change's tests do not depend on the host's environment
      (`$HOME`, the clock, the network) unless the test sets it.

---

## Writing documentation

The documentation's style and structure are in
[`docs/documentation-guide.md`](docs/documentation-guide.md). The
short version:

- Every document has a header comment with its normative
  references and its editing rules.
- Every command in a document is tested against a real release; a
  command that does not work is a defect.
- Every claim is linked to its source. A claim without a source is
  a defect.
- The tone is direct and second-person. "Run `forge version`", not
  "the user should run `forge version`".
- The vocabulary is the project's: a "channel" is a release
  channel, not a Go channel; a "record" is a log record, not a
  struct field.

An ADR follows the template in
[`docs/decisions/template.md`](docs/decisions/template.md). Its
status is one of `Proposed`, `Accepted`, `Superseded by ADR-NNN`,
or `Deprecated`. A new ADR is `Proposed` until a maintainer
approves it; the approval is the PR's review.

---

## The Code of Conduct

Forge follows the [Contributor Covenant](CODE_OF_CONDUCT.md). The
short version:

- Be direct about the change, not about the person.
- Assume good faith. A question is a question, not an attack.
- A reviewer's job is to make the change correct, not to make it
  the reviewer's.
- A contributor's job is to make the change reviewable, not to
  defend it.

Report unacceptable behavior to `conduct@forge.dev`. The
maintainers' response is documented in the Code of Conduct.

---

## Getting help

- **A question about the code:** open a discussion.
- **A question about a change's design:** open a discussion before
  opening the PR.
- **A question about the process:** open an issue with the
  `process` label.
- **A security question:** follow [`SECURITY.md`](SECURITY.md).
- **A question about the release pipeline:** read
  [`docs/development.md`](docs/development.md) §"Release" first;
  then open a discussion.

A question answered by an existing document is a documentation
defect. Report it: the document should have answered it. A
question answered by a maintainer is a candidate for a new
document or a new section; the maintainer will say so if it is.

---

## Related documents

| Document | Role |
|---|---|
| [`SECURITY.md`](SECURITY.md) | The vulnerability reporting process and the supply-chain policy. |
| [`docs/development.md`](docs/development.md) | The build, test, and release mechanics. |
| [`docs/documentation-guide.md`](docs/documentation-guide.md) | The documentation's style and structure. |
| [`docs/dependency-policy.md`](docs/dependency-policy.md) | The dependency review process. |
| [`docs/architecture/clean-architecture.md`](docs/architecture/clean-architecture.md) | The layering the arch checks enforce. |
| [`docs/decisions/README.md`](docs/decisions/README.md) | The ADR index and the template. |
| [`docs/requirements-register.md`](docs/requirements-register.md) | The requirements a change cites. |
| [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) | The contributor covenant. |
