# ADR-020: stdout/stderr Contract per Exit Code

**Status:** Accepted
**Date:** 2026-10-12
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

The Forge CLI writes to two streams: stdout and stderr. Every consumer of the CLI — a shell pipeline, a CI runner, an IDE extension, a wrapper script — observes one or both, and each consumer has its own reason for observing. A script that parses the CLI's output reads stdout; a script that logs the CLI's diagnostics reads stderr; a wrapper that shows the user what went wrong reads stderr; a CI runner that decides whether to continue reads the process's exit code. The three observations — stdout, stderr, exit code — are the CLI's entire observable surface, and their meanings must be predictable for the consumers' code to be correct.

Before WBS 11.3.2, the CLI's stream assignment was correct in practice but under-specified in contract. Commands wrote their successful output to stdout; the boundary wrote the error block to stderr; a successful invocation wrote nothing to stderr. The behaviour was consistent across commands, but nothing named the rule or forbade a future command from violating it. A future contributor could write a command that wrote its diagnostics to stdout, or that wrote its result to stderr, and no test would fail until a consumer's script broke.

The situation became acute with the introduction of `--format json` (Phase 9 WBS 9.4.1). A JSON consumer expects to parse stdout as exactly one JSON document. If a command wrote a diagnostic line to stdout before its JSON, the document would be unparseable; if the CLI wrote its JSON error to stderr instead of stdout, the consumer would see nothing on stdout and would have to parse stderr as a fallback. Neither fallback is acceptable: the JSON consumer's contract is that stdout is parseable, and the contract must hold on every exit code, including failures.

The decision affects the CLI's consumers (scripts, CI, IDE extensions), the CLI's command authors (who need to know which stream to write to), and the CLI's UX designers (who set the rules that command authors follow). It relates to WBS 11.1.2 (the exit code table, which names each code's semantic), WBS 10.4.1 (the frozen user-facing error format, which is the content of the stderr write), and WBS 9.4.1 (the `--format` flag, which selects between text and JSON rendering).

---

## 2. Options Considered

### Option A — No formal contract; command authors follow convention

The CLI documents a "convention" in its internal contributing guide and relies on command authors to follow it. The convention is: successful output on stdout, diagnostics on stderr, no mixing. No test enforces the convention; a reviewer catches a violation by reading the command's code.

**Pros:**

- Zero new artifacts. No reference document, no schema, no test.
- The convention is already followed by every Phase 2 command; documenting it adds no new behaviour.
- A future command with an unusual need (for example, a command that writes a progress meter to stderr while producing output on stdout) is not constrained by a rule it might have to work around.

**Cons:**

- The convention is not a contract. A command that violates it is not caught by any mechanism except a reviewer's attention, and a reviewer who does not know the convention will not catch it.
- The JSON consumer's expectation — "stdout is exactly one JSON document" — is not enforceable. A future command that writes a stray newline to stdout before its JSON breaks the consumer without any test failing.
- Documentation drift: the convention lives in a contributing guide that may or may not be read; the source of truth for the convention is not the code's own behavior.
- The failure modes are silent. A consumer that breaks because of a contract violation has no way to distinguish "the CLI's behaviour changed" from "the CLI always did this and my assumption was wrong".

### Option B — Documented contract per exit code, enforced by tests and schema

Publish the contract as `docs/reference/output-streams.md`, a reference document that names every exit code's stdout/stderr behavior in a table, and publish the JSON error envelope's schema as `schemas/error-envelope.schema.json`. Enforce the contract with tests: a unit test that asserts the envelope's shape and validity, an integration test that asserts the binary's stdout parses as JSON on every failure path, and a CI check that asserts the reference document and the schema are present and consistent with each other.

**Pros:**

- The contract is explicit. A consumer that reads the reference document knows exactly what to expect for every exit code.
- The contract is enforceable. The tests catch a violation at the point it is introduced; the schema gives a machine-readable form for tooling that wants to validate the envelope.
- The contract is stable. It is a public artifact, cited from the code and from the tests; a change to it is a change to a documented contract.
- The contract's scope is clear. It covers stdout, stderr, and the interaction with `--quiet`, `--verbose`, and `--format`. A consumer that reads the document once knows the rules.
- The contract is a function of the exit code, not of the command. A new command uses the same contract as every other command; the command's author does not need to invent a stream assignment.

**Cons:**

- The contract is a new artifact to maintain. A change to the CLI's stream behavior requires updating the document, the schema, and the tests in the same commit.
- The contract constrains a future command with an unusual need. A command that wants to write a progress meter to stderr while producing output on stdout must fit within the contract's rules; the contract is silent on progress meters, but a future ADR would need to extend it.
- The JSON envelope's schema is a stability commitment. A future change to the envelope's shape requires incrementing the `schemaVersion` and updating the schema and the tests in the same commit.
- The contract's enforcement is asymmetric. The unit tests catch a violation in the boundary's rendering; the integration tests catch a violation in the process's observable behavior. A violation in a command's own output path is not caught until the command's tests exercise the path.

### Option C — Documented contract per command, not per exit code

Publish the contract in each command's own documentation (`forge check --help`, `forge diff --help`, and so on), and let each command define its own stream behavior.

**Pros:**

- Each command's documentation is self-contained. A user who reads `forge check --help` finds the stream behavior for `check`.
- A command with an unusual stream need can document its behavior without affecting other commands.
- The documentation lives with the command's code, so a change to the command's behavior and a change to its documentation happen in the same commit.

**Cons:**

- The contract is not uniform. A consumer that branches on the exit code must read N commands' documentation to know what each exit code produces. The consumer's code must handle every command's stream behavior, which is the same as "no contract".
- The contract is not enforceable at the boundary. The boundary renders the error block; a per-command contract would need each command to render its own error block, which duplicates the boundary's logic and defeats its purpose.
- The JSON consumer's expectation — "stdout is parseable on every exit code" — is not uniformly satisfiable. A per-command contract can declare that one command emits valid JSON and another does not; the consumer's parser must handle both.
- The documentation is not a single source of truth. A user who wants to know the CLI's stream behavior must read every command's help text, or the CLI's top-level help, which is the reference document under a different name.

### Option D — Contract per stream, with the exit code as a secondary classification

Publish a contract that names each stream's purpose (stdout for results, stderr for diagnostics) and lets the command decide what constitutes a "result" and what constitutes a "diagnostic".

**Pros:**

- The contract is simple: two sentences, no table.
- The command has latitude to decide what is a result and what is a diagnostic, which accommodates commands whose output is not easily classified.
- The contract's spirit is clear; a reviewer can evaluate a command's behavior against it.

**Cons:**

- The contract is ambiguous where it matters most. A JSON consumer needs to know that stdout is parseable on *every* exit code; the two-sentence contract does not say whether a diagnostic can appear on stdout.
- The exit code is the consumer's primary classification; a contract that names streams without naming exit codes leaves the consumer to infer the mapping.
- A command that classifies its output differently from another command breaks the uniformity the contract exists to provide.
- The `--quiet` interaction is unspecified. The contract says "diagnostics on stderr"; whether `--quiet` suppresses the error block is a separate question that the two-sentence contract does not answer.

---

## 3. Decision

Adopt Option B: the documented contract per exit code, enforced by tests and by a published schema.

The contract is: **stdout carries the command's result; stderr carries diagnostics, errors, and warnings.** In text mode, the result is human-readable; on failure, stdout is empty and the diagnostic appears on stderr. In JSON mode (`--format json`), stdout carries exactly one JSON document: a success envelope on success, an error envelope on failure. The error envelope's shape is `{"schemaVersion":"1","exitCode":N,"error":{"code":"...","message":"...","category":"...","context":{...},"remediation":"..."}}`; the envelope is documented in `docs/reference/output-streams.md` and its schema is published at `schemas/error-envelope.schema.json`.

The `--quiet` flag suppresses informational diagnostics on stderr but never suppresses the error block and never suppresses the JSON envelope on stdout. The rationale: `--quiet` means "do not print informational output", and the error block is not informational — it is the user-facing explanation of a failure. Hiding it behind `--quiet` would make the CLI's failure modes invisible in the scenarios where the flag is most likely to be used (unattended CI, scripted invocations).

The `--verbose` flag adds fields to the human-rendered error block on stderr (the cause chain, the stack trace, the `internal_`-prefixed context keys) but does not alter the JSON envelope. The rationale: a machine consumer should not need to set `--verbose` to receive complete error data; the envelope is complete by construction.

The contract is enforced by four mechanisms: `docs/reference/output-streams.md` (the human-readable reference), `schemas/error-envelope.schema.json` (the machine-readable schema), `internal/cli/error_envelope_test.go` and `internal/cli/output_test.go` (the unit tests), and `cmd/forge/binary_integration_test.go` (the integration test that asserts the binary's stdout parses as JSON on every failure path).

The trade-offs accepted in choosing Option B:

- **The contract is a constraint on future commands.** A future command with an unusual stream need must fit within the contract or propose an ADR to extend it. The constraint is the point: the contract exists to make the CLI's stream behavior uniform across commands, and uniformity requires constraint.
- **The JSON envelope's schema is a stability commitment.** A future change to the envelope's shape requires incrementing the `schemaVersion` and updating the schema and the tests in the same commit. The commitment is the point: a machine consumer that parses the envelope needs to be able to detect a breaking change.
- **The contract is a new artifact to maintain.** The reference document, the schema, and the tests must be kept in sync. The CI check `scripts/deps/verify-output-streams.sh` catches a change to one without the others; the cost of the check is the cost of the sync.

Two assumptions the decision depends on:

- **The exit code is the consumer's primary classification.** The contract is a function of the exit code, not of the command. If a future consumer needs a different classification (for example, a per-command stream behavior), the contract must be extended. The current assumption is that the exit code's classification is sufficient for the CLI's consumers; the assumption is validated by Phase 9's config command, whose JSON output is the contract's first application.
- **The JSON envelope is the only machine-readable error output.** The contract does not provide a second machine-readable form (for example, a TOML error envelope). A future consumer that wants a different form must propose an ADR. The current assumption is that JSON is sufficient; the assumption is validated by Phase 10's JSON output contract (WBS 5.0).

The decision is not provisional. The contract is a public artifact; a change to it is a change to a documented contract. The schema's `schemaVersion` field is the mechanism for detecting a breaking change; the version is incremented when the envelope's shape changes, not when a new field is added.

---

## 4. Consequences

### 4.1 Positive

- **The contract is explicit.** A consumer that reads `docs/reference/output-streams.md` knows exactly what to expect for every exit code. The table names stdout's and stderr's content for each code, including the `--quiet`, `--verbose`, and `--format` interactions.
- **The contract is enforceable.** The unit tests assert the envelope's shape; the integration test asserts the binary's stdout parses as JSON on every failure path. A violation of the contract is caught at the point it is introduced.
- **The contract is machine-readable.** The envelope's schema is published at `schemas/error-envelope.schema.json`. A tool that wants to validate a Forge error envelope against the schema can do so without reading the Go source.
- **The JSON consumer's expectation is satisfied.** With `--format json`, stdout is exactly one JSON document on every exit code, including failures. A machine consumer parses stdout without a fallback; a failure's details are in the envelope's `error` sub-object.
- **The exit code is the primary classification.** The contract is a function of the exit code, not of the command. A new command uses the same contract as every other command; the command's author does not need to invent a stream assignment.
- **The `--quiet` rule is explicit.** The flag suppresses informational diagnostics but never the error block or the JSON envelope. The rule is documented in the reference document and enforced by the tests in `flag_interaction_test.go`.
- **The `--verbose` rule is explicit.** The flag adds fields to the human-rendered error block but does not alter the JSON envelope. The rule is documented and enforced by the envelope's tests.
- **The success envelope is defined but minimal.** The shape `{"schemaVersion":"1","exitCode":0,"result":{...}}` is frozen now so that a consumer can write a version check before the result's shape is decided. Phase 10 WBS 5.0 will fill in the result's structure.

### 4.2 Negative

- **The contract is a new artifact to maintain.** The reference document, the schema, and the tests must be kept in sync. The CI check `scripts/deps/verify-output-streams.sh` catches a change to one without the others; the sync is the cost of the contract's existence.
- **The schema's stability is a commitment.** A future change to the envelope's shape requires incrementing `schemaVersion` and updating the schema and the tests. The commitment is a cost, but it is the mechanism that lets a consumer detect a breaking change.
- **The contract constrains future commands.** A command with an unusual stream need (for example, a command that wants to write a progress meter to stderr while producing output on stdout) must fit within the contract's rules. The contract is silent on progress meters; a future ADR would need to extend it.
- **The contract's enforcement is asymmetric.** The unit tests catch a violation in the boundary's rendering; the integration tests catch a violation in the process's observable behavior. A violation in a command's own output path is not caught until the command's tests exercise the path. The asymmetry is the cost of separating the boundary's tests from the commands' tests.
- **The `--quiet` rule is a policy, not a mechanic.** The rule that `--quiet` suppresses informational diagnostics but not the error block is a policy decision. A future change to the rule requires a new ADR. The rule is small enough that it does not need its own ADR today, but it is a decision the contract encodes.
- **The `--verbose` rule is a policy.** The rule that `--verbose` does not alter the JSON envelope is a policy decision. The rule is the mechanism that lets a machine consumer receive complete error data without the flag; a future change to the rule requires a new ADR.
- **The success envelope's shape is frozen before its content is decided.** The shape is `{"schemaVersion":"1","exitCode":0,"result":{...}}`, but the `result` field's structure is not yet decided. A future change to the `result`'s structure requires incrementing `schemaVersion` only if the change is breaking; an additive change to `result`'s fields does not. The current state is that the envelope's outer shape is frozen and its content is deferred.

### 4.3 Neutral

- **The contract's table is duplicated in this ADR and in `docs/reference/output-streams.md`.** The two must agree. The CI check asserts the agreement by reading both; the cost of the duplication is the cost of the check, and the benefit is that a reader finds the table in whichever document they reach first.
- **The envelope's field order is deterministic.** `encoding/json` marshals struct fields in declaration order, so the envelope's bytes are predictable. The determinism is a property of the encoder, not a policy; a future change to the encoder would need to preserve it.
- **The envelope's `context` map's keys are sorted.** Same rationale as the field order: a property of `encoding/json`'s map handling, not a policy.
- **The envelope's `context` and `remediation` fields are omitted when empty.** The `omitempty` tag makes the envelope smaller when a field is absent. The omission is a property of the encoder; a consumer that expects the field to be present must handle its absence.
- **The contract's enforcement does not extend to third-party commands.** A future command from a plugin (if Forge supports plugins) would need to follow the contract; the enforcement mechanisms do not cover the plugin's code. The current assumption is that every command is a Forge-authored command; the assumption would be revisited if plugins are supported.

---

## 5. Related Requirements

- `FR-CLI-EXIT-024` — the stdout/stderr contract per exit code. This ADR's primary requirement.
- `FR-CLI-OUTPUT-010` — the JSON output contract. The envelope's shape and stability are this ADR's subject.
- `FR-CLI-EXIT-001` — the range taxonomy. The codes the contract is a function of.
- `FR-CLI-EXIT-021` — the error-to-exit-code mapping. The mapping's outputs are the codes the contract names.
- `FR-CLI-EXIT-023` — the single exit boundary. The boundary's rendering is the mechanism the contract depends on.
- `NFR-QUALITY-005` — the CI-enforceable policy. The contract's enforcement mechanisms are named in § 3 and § 4.
- `UX-CLI-006` — the `--quiet` and `--verbose` interaction with the error block. This ADR's contract names the rules.

> The following requirements must be added to the register when
> WBS 1.4 is complete: `FR-CLI-OUTPUT-011` (the success envelope's
> shape), `FR-CLI-OUTPUT-012` (the envelope's schema version
> policy), and `NFR-QUALITY-009` (the contract's test coverage).

---

## 6. Related ADRs

- `ADR-019` — Single Exit Boundary in main. The boundary is the mechanism that produces the output the contract names. The contract is a function of the exit code; the boundary is where the code and the output are produced together.
- `ADR-018` — Exit Code Migration and Stability Policy. The codes the contract is a function of are the ones this ADR freezes. The contract's table names each code's stream behavior; the codes' values are the subject of ADR-018.
- `ADR-019` (the Exit Code Semantics ADR, if numbered separately) — Exit Code Semantics. The severity classification applies to the codes; the contract does not change severity, only reports it.
- `ADR-020` — JSON Error Envelope for `--format json`. Companion decision: the envelope's shape, its schema, and its stability policy are the subject of the JSON envelope ADR. This ADR decides that the envelope exists and what its role is in the contract; the JSON envelope ADR decides its exact shape.
- `ADR-016` — Error Redaction Policy. The redaction rules apply to the error's context. The envelope carries the context verbatim; the redaction happens before the envelope is built.
- `ADR-014` — User-Facing Error Format. The format is the content of the stderr write in text mode. The JSON envelope is the machine-readable counterpart; the two are documented together in the reference document.
- `ADR-012` — Error Code Registry and Stability Policy. The error codes (the strings) are the envelope's `error.code` field. The codes' stability is the subject of ADR-012; the envelope carries them verbatim.
- `ADR-011` — Closed Error Category Taxonomy. The categories are the envelope's `error.category` field. The categories' closure is the subject of ADR-011; the envelope carries them verbatim.

---

## 7. Notes

### Open questions deferred by this ADR

- **The success envelope's `result` shape.** The success envelope's outer shape is frozen (`{"schemaVersion":"1","exitCode":0,"result":{...}}`), but the `result` field's structure is not. Phase 10 WBS 5.0 will decide the structure for the config command; a future WBS will decide it for other commands. The decision is deferred; the current state is that the outer shape is a contract and the inner shape is not.
- **The schema's evolution policy.** The schema's `schemaVersion` field is incremented on a breaking change. A future ADR should record the exact criteria for "breaking" and the procedure for incrementing the version. The current state is that the version is "1" and a breaking change increments it; the criteria are not enumerated.
- **The contract's extension for plugins.** If Forge supports plugins (a future WBS), the plugins' commands must follow the contract. The enforcement mechanisms (the CI check, the tests) do not cover plugin code. A future ADR would need to record how the contract applies to plugins and how it is enforced.
- **The contract's interaction with `--format` values beyond text and JSON.** The `--format` flag's value set is `{text, json}` today. A future value (for example, `yaml`) would need a contract for stdout's content in that format. The decision is deferred; the current state is that text and JSON are the two formats, and each has a stream contract.
- **The `--quiet` interaction with finding reports.** The contract says `--quiet` suppresses informational diagnostics on stderr. It also says (per the WBS 11.3.2 text) that findings are suppressed from stdout in text mode. The rule for findings in JSON mode is that the envelope is printed; the rule for findings in text mode is that the report is suppressed but the exit code still signals the finding. The rule is documented; a future change to it requires a new ADR.
- **The contract's interaction with progress output.** A future command that produces a long-running operation may want to write progress output to stderr while producing its result on stdout. The contract is silent on progress output; a future ADR would need to extend it. The current assumption is that no Phase 2 command produces progress output; the assumption would be revisited if a command does.

### Follow-up decisions anticipated

- **Phase 10 WBS 5.0** (JSON output contract). The success envelope's `result` shape is the WBS's subject. This ADR's contract names the envelope's outer shape; the WBS fills in the inner shape.
- **Phase 9 WBS 6.4.1** (config command output formats). The config command is the contract's first application. The command's output in text and JSON is the subject of its own WBS; the contract's rules are this ADR's subject.
- **A future ADR on the schema's evolution policy.** The criteria for "breaking" and the procedure for incrementing the version are deferred. The ADR would record the policy that the schema's stability depends on.

### Links to external discussion

- WBS 11.3.2 — the task that requested the contract.
- `docs/reference/output-streams.md` — the contract's human-readable form.
- `schemas/error-envelope.schema.json` — the envelope's machine-readable schema.
- `internal/cli/error_envelope.go` and `internal/cli/output.go` — the boundary's rendering of the envelope and the format-aware writers.
- `internal/cli/error_envelope_test.go` and `internal/cli/output_test.go` — the unit tests that assert the envelope's shape.
- `cmd/forge/binary_integration_test.go` — the integration test that asserts the binary's stdout parses as JSON on every failure path.
- `scripts/deps/verify-output-streams.sh` — the CI check that asserts the contract's artifacts are present and consistent.

---

*End of ADR-020.*
