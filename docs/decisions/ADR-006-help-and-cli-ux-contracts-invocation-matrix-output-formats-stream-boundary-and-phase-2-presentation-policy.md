# ADR-006: Help and CLI UX Contracts — Invocation Matrix, Output Formats, Stream Boundary, and Phase 2 Presentation Policy

**Status:** Proposed
**Date:** 2026-10-11
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge's command-line interface is its primary product surface. Its
users are heterogeneous: humans typing `forge version` at a
terminal, shell scripts that pipe `forge --help` into a documentation
generator, CI runners that branch on the exit code of a future
`forge check`, and tooling that will one day consume machine-readable
output (SBOM, SARIF). Each audience has different expectations. A
human wants an error message they can read; a script wants an error
message on the correct stream so that `2>/dev/null` silences it; a
CI runner wants an exit code that distinguishes "the invocation was
wrong" from "the invocation was right but the command failed".

The CLI's presentation layer — what a user sees and where they see it
— is the contract with those audiences. Until WBS 7.x, that contract
was implicit. The help text was written by Cobra's defaults; the error
format was whatever the error's `Error()` method returned; the stream
boundary was respected by convention; the colour policy was unstated.
Each contract existed as a shared understanding rather than as a
documented rule, and each was enforced, if at all, by a single test
that happened to assert a related property.

Three forces make the current state untenable. First, the number of
contracts has grown: WBS 5.x produced the root identity strings, the
version output contract, the help behaviour contract, and the global
flag inventory; WBS 6.x produced the version model, the linker
injection contract, and the two version output formats; WBS 7.x has
produced eleven more contracts, one per requirement. Second, the
contracts are coupled: the ASCII-only policy affects the root identity
strings, which affect every help invocation; the stream boundary
affects help, errors, and the global flags' interaction with both.
Third, the contracts are visible to consumers who will not read the
code: a script that parses `forge version` today is depending on a
frozen format, whether or not the format is documented as frozen.

If no decision is made, the contracts continue to exist as folklore.
A future contributor changes the error format to add colour and does
not know that a script depends on the plain form. A future contributor
adds a Unicode arrow to the root help output and does not know that
the ASCII-only policy forbids it. A future contributor adds a fourth
global flag and does not know that the inventory is closed. Each
change is individually reasonable and jointly breaks the contract
that the CLI's consumers rely on.

This ADR records the four decisions that the eleven WBS 7.x contracts
implement. It is the record of the *why*; the CLI UX specification
(`docs/cli-ux-spec.md`) is the record of the *what*. The ADR does not
restate the contracts' rules; it names them, states the reasoning,
and records the alternatives that were rejected.

---

## 2. Options Considered

### Option A — Adopt the contracts as the CLI's public presentation contract, freeze them, and enforce each with a test

Each contract is documented in a section of `docs/cli-ux-spec.md`,
enforced by a dedicated test file in `internal/cli/`, and frozen:
changing any contract requires an ADR. The eleven contracts are the
help invocation matrix (WBS 7.1.1), the root help content contract
(WBS 7.1.2), the per-command help content contract (WBS 7.1.3), the
command metadata contract (WBS 7.2.1), the Examples convention
(WBS 7.2.2), the invalid-command contract (WBS 7.3.1), the error
message format (WBS 7.3.2), the output stream boundary (WBS 7.4.1),
the colour and ASCII policy (WBS 7.4.2), the global flag interaction
matrix (WBS 7.4.3), and the root identity strings (WBS 5.1.1).

**Pros:**

- **Each contract has one home.** A reader who wants to know the
  error format reads § 4.9g; a reader who wants to know the help
  matrix reads § 4.9b. The sections are independent.
- **Each contract has one enforcement.** The test file names the
  contract and cites the WBS item. A change that violates a contract
  fails the specific test that names the rule.
- **The change process is uniform.** A change to any contract
  requires updating the formatter, the golden files, the test, and
  the corresponding spec section, all in one commit. A reviewer
  sees the four updates together.
- **The contracts are visible to consumers.** A script that parses
  `forge version` can cite § 4.8.1 as the frozen format. A CI runner
  can cite § 4.9f as the frozen invalid-command behaviour.
- **Future contributors have a starting point.** A contributor who
  wants to add colour sees § 4.9j and the ASCII-only policy; the
  contribution is scoped by the policy rather than unbounded.

**Cons:**

- **Eleven contracts is a lot of documentation.** The CLI UX spec's
  § 4.9 now has eleven subsections; a reader who wants to know the
  whole presentation layer reads eleven sections.
- **The frozen contracts constrain future design.** A future
  contributor who wants to improve the help output (for example,
  by adding examples to the root command) must open an ADR. The
  friction is deliberate but it is friction.
- **The golden files are a maintenance burden.** Every change to the
  help output requires regenerating the affected golden files. The
  regeneration is mechanical but must be done correctly (the file
  must end in a newline; the editor must not strip trailing
  whitespace).
- **The ASCII-only policy's scope is wide.** It forbids every
  non-ASCII character, including typographic punctuation that would
  be more readable than the ASCII equivalents. The em dash and the
  right arrow were removed from the root identity strings; a
  future contributor who wants them back must open an ADR.

### Option B — Document the contracts in the CLI UX spec but do not freeze them or enforce them with dedicated tests

Each contract is documented in `docs/cli-ux-spec.md`; the tests that
happen to assert related properties remain, but no dedicated test is
added per contract.

**Pros:**

- **Less documentation to maintain.** The spec's § 4.9 has fewer
  subsections; the contracts live in the sections that already
  exist (the help section covers help; the error section covers
  errors).
- **Less test surface.** No dedicated test file per contract; the
  existing tests cover the properties they happen to cover.
- **More flexibility.** A future contributor changes the help
  output without opening an ADR. The change is reviewed by the
  normal process.

**Cons:**

- **The contracts are aspirational.** A contract that is not
  enforced by a test will drift. The current `TestRootHelp_Golden`
  catches a change to the root help output only because the golden
  file exists; without a dedicated contract test, a future
  contributor who deletes the golden file also deletes the
  contract.
- **No change process.** A future contributor changes the error
  format without knowing that a consumer depends on it. The spec
  documents the format but does not signal that it is frozen.
- **No reason for the rules.** The spec documents what the rules
  are; the ADR documents why they exist. Without the ADR, a future
  contributor who questions a rule (why no colour? why ASCII-only?)
  must infer the reasoning from the rule itself.

### Option C — Adopt only the contracts that have external consumers; defer the internal ones

Freeze the contracts whose rules affect consumers outside the
project: the version output formats (WBS 6.4.1, WBS 6.4.2), the
exit codes (WBS 4.1.2), the stream boundary (WBS 7.4.1), and the
error message format (WBS 7.3.2). Defer the internal contracts — the
help matrix, the per-command help content contract, the command
metadata contract, the Examples convention, the ASCII-only policy —
to later phases, when their consumers are clearer.

**Pros:**

- **The contracts most likely to break consumers are the ones
  frozen first.** The version output and the exit codes are the
  contracts that scripts depend on today. The help output and the
  Examples convention are for humans and for review.
- **The internal contracts can evolve.** A future contributor who
  wants to improve the help output does not need an ADR. The
  help output is for humans; a human who wants a different layout
  writes a PR.
- **Less upfront documentation.** The CLI UX spec's § 4.9 has
  fewer subsections.

**Cons:**

- **The internal contracts still exist.** The help output is
  rendered by Cobra from the command's `Use`, `Short`, and `Long`
  fields. The fields have rules (the `Short` field is 60 runes or
  fewer; the `Long` field ends with a period). Those rules are
  enforced whether they are documented or not. Not documenting them
  means a future contributor discovers them by failing a review.
- **The line between internal and external contracts is porous.**
  A script that captures `forge --help` for documentation
  generation is an external consumer of the help output. A script
  that runs `forge help unknown` and parses the stderr is an
  external consumer of the invalid-command behaviour. Almost every
  contract has an external consumer if the project's tooling is
  taken into account.
- **The deferred contracts will be frozen later anyway.** If the
  help output is a contract in Phase 6, freezing it in Phase 6 is
  cheaper than freezing it in Phase 2 only if nothing has changed
  in the meantime. Since Phase 2's implementations have already
  frozen the shape, the "later" freeze is a formality.

### Option D — Adopt the contracts as guidance, not as rules

Document the contracts as recommended patterns; enforce the ones
that the current tests happen to enforce; leave the rest to code
review.

**Pros:**

- **The lightest documentation.** The contracts live as a
  recommended-shape section in the CLI UX spec.
- **The lightest test surface.** No new test files.

**Cons:**

- **The contracts are not contracts.** A rule that is not enforced
  is a suggestion. The eleven contracts have specific rules
  (fourteen invocations, seven invalid-command cases, four error
  format rules, ten stream boundary entries). Enforcing them
  requires tests that assert the specific rules. Without the tests,
  the rules drift.
- **The reasoning is lost.** The reason the ASCII-only policy
  exists is that colour and Unicode interact with shell scripting.
  A future contributor who sees an ASCII-only rule without the
  reasoning will question it; a future contributor who sees the
  reasoning will understand it.

---

## 3. Decision

**Option A is adopted.** The eleven contracts are documented in
`docs/cli-ux-spec.md` § 4.9, enforced by dedicated test files in
`internal/cli/`, and frozen: changing any contract requires an ADR.

The reasoning, in order of how much it tipped the balance:

**The contracts exist whether or not they are documented.** Cobra
renders the help output from the command fields; the fields have
rules; the rules are enforced by Cobra's rendering and by the
tests that assert the output. The rules can be documented and
enforced, or they can be implicit and discovered by failing a
review. The former is cheaper for everyone: a contributor reads a
rule and complies; a contributor discovers a rule by failing a
review and is annoyed.

**The contracts are visible to external consumers.** A script that
parses `forge version` depends on the format; a CI runner that
branches on the exit code depends on the mapping; a documentation
generator that captures `forge --help` depends on the help output.
Each of those is an external consumer with a legitimate expectation
that the contract is stable. Documenting the contracts makes the
expectations explicit and gives consumers a section to cite.

**The contracts are coupled.** A change to the stream boundary
affects how help is written, how errors are written, and how the
ASCII policy is enforced. Documenting them together (in § 4.9)
matches the coupling. Freezing them together (in this ADR) records
the decisions that make the coupling coherent.

**Each contract has a natural enforcement.** The help matrix is a
table-driven test; the root help content contract is a golden-file
comparison; the error message format is a unit test on the format
builder; the ASCII policy is a predicate applied to real output.
The enforcement mechanism follows from the contract's shape; a
contract whose shape suggests a test should have the test.

**The alternative's cost is invisible until it is expensive.**
Option B's cost — the contracts drift — is not visible at the
moment the decision is made. It becomes visible six months later
when a script breaks because the error format changed. Option A's
cost — the eleven sections and the golden files — is visible
immediately. The visible cost is the one that can be managed.

**Assumptions this decision depends on:**

1. **The help output is stable across Cobra versions.** The golden
   files pin Cobra's rendering; a future Cobra version that changes
   the layout breaks the golden files. The golden files would be
   regenerated at that point, with the change reviewed. The
   assumption is that Cobra's rendering changes rarely; the cost
   of the assumption is a periodic regeneration.
2. **The contracts' audiences do not conflict.** A human wants a
   readable error message; a script wants a machine-readable error
   on stderr. The two audiences want different things from the same
   output. The contracts reconcile them by putting the message on
   stderr as plain text and the future JSON form on stdout. The
   assumption is that the two channels suffice; a future command
   that needs both in one stream opens an ADR.
3. **The ASCII-only policy is the correct reading of the
   requirement.** The policy forbids every non-ASCII character. A
   future contributor might argue that typographic punctuation is
   harmless and that only control characters and ANSI escapes need
   forbidding. The policy takes the stricter position; the
   reasoning is in § 4.9j. A future revision of the policy opens
   an ADR.

**This decision is provisional in one respect:** the contracts are
frozen for Phase 2. A future phase that introduces colour, TTY
detection, or richer errors extends the contracts rather than
replacing them. The extension is documented in § 4.9j's
future-proofing rules and in the "Phase 2 scope" notes in each
section. The contracts' *shape* (a first line, an optional
suggestion, a stream boundary) is stable; the *content* of each
contract grows.

---

## 4. Consequences

### 4.1 Positive

- **Every contract has a home.** A reader who wants to know the
  error format reads § 4.9g; a reader who wants to know the help
  matrix reads § 4.9b. The CLI UX spec's § 4.9 is the reference
  for the presentation layer's rules.
- **Every contract has an enforcement.** A change that violates a
  contract fails a specific test in `internal/cli/`. The failure's
  message names the contract and cites the WBS item.
- **The change process is uniform.** A change to any contract
  requires updating the formatter, the golden files, the test, and
  the spec section, all in one commit. A reviewer sees the four
  updates together.
- **The contracts are citable.** A consumer can cite the section
  and the ADR. A reviewer can cite the section when evaluating a
  change. A contributor can cite the section when asking whether a
  proposed change is allowed.
- **Future contributors have a starting point.** A contributor who
  wants to add colour reads § 4.9j and knows the policy; a
  contributor who wants to add a new help invocation reads § 4.9b
  and knows the process.
- **The reasoning is recorded.** The ADR's Decision section
  records the alternatives and the tradeoffs. A future contributor
  who questions a rule finds the reasoning.

### 4.2 Negative

- **The documentation surface is large.** The CLI UX spec's § 4.9
  has eleven subsections; a reader who wants to know the whole
  presentation layer reads eleven sections. The summary table in
  § 4.9m mitigates this by listing each contract, its section, and
  its enforcement.
- **The frozen contracts constrain future design.** A future
  contributor who wants to improve the help output must open an
  ADR. The friction is deliberate; a change to a frozen contract
  affects consumers, and the ADR is the mechanism that makes the
  change deliberate.
- **The golden files are a maintenance burden.** Every change to
  the help output requires regenerating the affected golden files.
  The regeneration must be done correctly: the file must end in a
  newline, the editor must not strip trailing whitespace, and the
  shell redirect must capture the correct stream. The four
  conventions are documented; a contributor who violates one sees
  a test failure and reads the convention.
- **The ASCII-only policy is stricter than necessary.** The policy
  forbids typographic punctuation (em dash, curly quotes) and
  symbol glyphs (right arrow, checkmark). The em dash and the
  right arrow were removed from the root identity strings. The
  removal is a visible change to the CLI's identity; the policy
  chose universal parseability over typographic polish.
- **The contracts' audiences conflict.** The error message is for
  humans; the error's exit code is for scripts. The two audiences
  want different things from the same invocation. The contracts
  reconcile them by putting the message on stderr as plain text
  and relying on the exit code for the machine-readable signal.
  A future command that needs both in one stream opens an ADR.

### 4.3 Neutral

- **The CLI UX spec is now the largest document in `docs/`.** Its
  § 4.9 is a substantial part of its length. The size is a
  consequence of the number of contracts; a reader who wants a
  specific contract reads one section, not the whole document.
- **The `help_content.go`, `metadata.go`, `examples.go`,
  `errors.go`, `stream_boundary.go`, `ascii_policy.go`, and
  `flag_interaction.go` files exist solely to hold the contracts'
  predicates and constants.** Each is small; each exists so that
  the contract has a home in the code. The files are a
  consequence of the freeze; a codebase without the freeze would
  not have them.
- **The `--no-color` flag is reserved but not implemented.** The
  flag is documented in § 4.9j's future-proofing rules; it is not
  registered in Phase 2. A reader who sees the documentation might
  expect the flag to exist; the documentation says it does not.
- **The global flag inventory has three flags and no more.** The
  inventory is frozen; adding a fourth requires an ADR. The
  freeze's reasoning is in § 4.11; a future contributor who wants
  a fourth flag opens an ADR and states the consumer WBS item.

---

## 5. Related Requirements

The following requirements are established, affected, or constrained
by this decision:

- `FR-CLI-002` — Help behaviour contract. Established by WBS 5.2.2
  and extended by WBS 7.1.1, WBS 7.1.2, and WBS 7.1.3.
- `FR-CLI-004` — Error message contract. Established by WBS 7.3.1
  and WBS 7.3.2.
- `FR-CLI-005` — Output stream boundary. Established by WBS 7.4.1.
- `FR-CLI-007` — `--format` flag semantics. Established by
  WBS 6.4.2 and documented in § 4.12.
- `FR-CLI-010` — Stream separation for the version command.
  Established by WBS 6.3.1.
- `FR-CLI-018` — Command metadata contract. Established by
  WBS 7.2.1 and extended by WBS 7.2.2.
- `FR-CLI-022` — Help invocation matrix. Established by WBS 7.1.1.
- `EXIT-004` — Usage exit codes for invalid invocations.
  Established by WBS 7.3.1.
- `LOG-004` — Errors are never suppressed by `--quiet`.
  Established by WBS 7.4.3.
- `LOG-005` — Global flag precedence. Established by WBS 5.3.1 and
  extended by WBS 7.4.3.
- `DOC-003` — Colour and terminal policy. Established by WBS 7.4.2.

The following requirements must be added to the register when WBS 1.4
is complete:

- `FR-CLI-023` — Root help content contract.
- `FR-CLI-024` — Per-command help content contract.
- `FR-CLI-025` — Examples convention.
- `NFR-CLI-004` — ASCII-only output policy.
- `NFR-CLI-005` — No colour output in Phase 2.
- `NFR-CLI-006` — No TTY detection in Phase 2.
- `UX-CLI-003` — Global flag interaction matrix.
- `UX-CLI-004` — Output stream boundary as a decision rule.

---

## 6. Related ADRs

- `ADR-002` — Choose Cobra as the CLI framework. The help matrix,
  the root help content contract, and the per-command help content
  contract depend on Cobra's command tree and flag parsing. The
  `--format` flag's per-command scope depends on Cobra's per-command
  flag registration.
- `ADR-003` — CLI framework foundation package execution,
  injection, registry, and contract. The two-boundary model and the
  `Dependencies` struct are the substrate for the stream boundary.
  Commands receive `deps.Stdout` and `deps.Stderr`; the contract
  forbids direct `os.Stdout` and `os.Stderr` usage.
- `ADR-004` — Root command identity and behaviour contracts. The
  root identity strings are part of the presentation layer; the
  ASCII-only policy changed them. The `RootShortDesc`'s em dash and
  the `RootLongDesc`'s right arrows were replaced with ASCII
  equivalents.
- `ADR-005` — Version command structure, output formats, and
  stability guarantees. The version output contract (§ 4.8) and the
  `--format` flag's per-command semantics (§ 4.12) are documented
  in the version command's ADR; this ADR's § 4.9m references them.

A future ADR will be required if any of the following occurs:

- A contract's shape changes (a new stream is added; a new error
  block is added; the help matrix gains a row that introduces a new
  equivalence class).
- A future phase introduces colour, TTY detection, or emoji. The
  future-proofing rules are in § 4.9j; the implementing ADR
  supersedes or amends this one.
- A future phase adds a fourth global flag. The flag inventory is
  frozen in § 4.11; the addition requires an ADR that names the
  consumer WBS item.
- A future phase adds a new help invocation whose behaviour is not
  covered by the existing matrix's rules.

---

## 7. Notes

**Open questions deferred by this ADR:**

1. **Should the CLI emit colour in Phase 6+?** The policy defers
   colour to Phase 6; the implementing ADR decides whether to
   introduce it. The future-proofing rules (NO_COLOR, TTY
   detection, `--no-color`) are documented; the implementing ADR
   applies them.

2. **Should the ASCII-only policy extend to source files?** The
   policy's scope is the CLI's output. The source files may contain
   non-ASCII characters in comments and docstrings. A future
   project-wide convention that requires ASCII-only source files
   would be a separate decision; this ADR's policy does not require
   it.

3. **Should the help output list hidden commands?** The matrix's
   rule 8 says hidden commands do not appear in `forge --help`.
   They do appear in `forge help <hidden-command>` and
   `forge <hidden-command> --help`. The contract does not forbid
   a future change that lists them behind `--verbose`; the change
   would extend the matrix and require an ADR.

4. **Should the error message format gain a `code` line?** The
   format is `Error: <message>` plus an optional context and
   suggestion. WBS 10.0 introduces structured errors with a code
   field. A future ADR decides whether the code appears in the
   human-readable output or only in the JSON output.

5. **Should the golden files be regenerated by a tool?** The
   regeneration is currently a shell command per file. A future
   WBS item could add a `task update-golden` target that
   regenerates every golden file in one command. The target would
   need to preserve the four conventions (trailing newline,
   no trailing-whitespace stripping, correct stream, ASCII-only
   content).

**Links:**

- `docs/cli-ux-spec.md` § 4.9 — the eleven contract subsections.
- `docs/cli-ux-spec.md` § 4.9m — the summary of contracts.
- `docs/cli-ux-spec.md` § 4.7 — the root identity strings (updated
  for the ASCII-only policy).
- `docs/development.md` "Adding a command" — the procedure that a
  contributor follows when adding a command; it references the
  command metadata contract and the Examples convention.
- `docs/architecture.md` § 11.12 (Handler / Service Boundary) — the
  boundary that the help contracts assume.
- WBS 7.1.1, WBS 7.1.2, WBS 7.1.3, WBS 7.2.1, WBS 7.2.2,
  WBS 7.3.1, WBS 7.3.2, WBS 7.4.1, WBS 7.4.2, WBS 7.4.3 — the
  eleven WBS items that produced the contracts.
- WBS 5.1.1, WBS 5.1.2, WBS 5.2.1, WBS 5.2.2, WBS 5.2.3,
  WBS 5.3.1 — the earlier WBS items whose contracts this ADR
  references.
  