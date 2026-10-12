# ADR-021: JSON Error Envelope for `--format json`

**Status:** Accepted
**Date:** 2026-10-12
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

A machine consumer of the Forge CLI — a CI runner, an IDE extension, a wrapper script — needs to read a command's failure in a form it can parse. The CLI's user-facing error format (WBS 10.4.1, rendered by `forgeerr.Format`) is designed for a human: it wraps the message, sorts context keys, and terminates with a suggestion block. A machine consumer that wants only the error's code, its message, and its structured context must parse that rendering, which is fragile (the format is stable, but parsing prose is not the consumer's job), unnecessarily verbose (the wrapping and suggestion are for humans), and not a contract (the format is frozen for humans; a consumer that depends on its exact bytes is depending on a rendering, not on a data interface).

The CLI already exposes a `--format` flag whose value set is `{text, json}` (WBS 9.4.1). For the config command, `--format json` selects a JSON rendering of the command's *success* output. Before this decision, there was no analogous rendering for the command's *failure*: a failing invocation with `--format json` wrote nothing to stdout and wrote the human-readable error block to stderr. A machine consumer that set `--format json` and observed a non-zero exit code had no JSON document to parse; it had to fall back to reading stderr, which is exactly the human-oriented rendering the flag was supposed to replace.

The decision was driven by the WBS 11.3.2 contract (`stdout` carries the command's result; in JSON mode the result is exactly one JSON document, on every exit code). The WBS's contract named the requirement; this ADR decides the document's exact shape, its stability policy, and the mechanism that produces it. The document is called the **JSON error envelope**, and its role is to be the machine-readable counterpart to the human-readable error block: the same information (code, message, category, context, remediation) rendered in a form a consumer can parse without inspecting the CLI's prose.

The decision affects the CLI's machine consumers (who parse the envelope), the CLI's error-handling code (which builds the envelope from a `*forgeerr.Error`), the CLI's tests (which assert the envelope's shape), and the CLI's documentation (which publishes the envelope's schema). It relates to WBS 10.4.1 (the human-readable error format), WBS 9.4.1 (the `--format` flag), WBS 11.3.2 (the stream contract), and WBS 10.3.1 (the error code registry, whose codes the envelope carries).

---

## 2. Options Considered

### Option A — No JSON error envelope; machine consumers parse the human-rendered error block

Leave the failure path as it was: on failure, stdout is empty and stderr carries `forgeerr.Format`'s output. A machine consumer that wants the error's structured data parses the rendered block.

**Pros:**

- No new artifact. No envelope, no schema, no additional test surface.
- The human-rendered block is already a frozen format (WBS 10.4.1); its stability is documented and tested.
- The `--format` flag's effect is unchanged: it selects the *success* output's rendering, not the failure output's.

**Cons:**

- Parsing the rendered block is fragile. The block's shape is documented (the `Error:`, `Code:`, and `Suggestion:` lines), but a consumer that depends on the exact bytes is depending on a rendering, not on a data interface. A future change to the human format (permitted by WBS 10.4.1's additive-only policy? — the format is frozen, but the "frozen" is about the human experience, not about machine parsing) risks the consumer.
- The block's content is human-oriented. The `Error:` line wraps at 80 columns; the `Suggestion:` block is prose; the context keys are formatted with a fixed indentation for readability. A machine consumer that wants only the message and the context must skip the human-oriented parts.
- The `--format json` flag's contract is incomplete. A consumer that sets the flag expects JSON on stdout in all cases; a failing invocation produces no JSON, which breaks the expectation the flag set.
- The WBS 11.3.2 contract cannot be satisfied. The contract says stdout is exactly one JSON document in JSON mode on every exit code; without an envelope, the failure path violates the contract.

### Option B — A JSON error envelope written to stdout on failure, with a versioned schema

Introduce a JSON error envelope as the stdout content in JSON mode on failure. The envelope's shape is:

```json
{
  "schemaVersion": "1",
  "exitCode": 10,
  "error": {
    "code": "FORGE_CONFIG_MALFORMED",
    "message": "invalid YAML in config.yaml",
    "category": "CONFIG",
    "context": { "path": "config.yaml", "line": 12 },
    "remediation": "Fix the YAML syntax error at the indicated line."
  }
}
```

The envelope is built from a `*forgeerr.Error` by `encodeErrorEnvelope` (in `internal/cli/error_envelope.go`), written to stdout by `writeFailure` (in `internal/cli/output.go`), and published as `schemas/error-envelope.schema.json`. The envelope's fields are additive; the `schemaVersion` field is a stability marker for the envelope's shape, incremented on a breaking change.

**Pros:**

- The envelope is a data interface, not a rendering. Its shape is published as a schema; a consumer validates against the schema without parsing prose.
- The envelope's content mirrors `forgeerr.Error`'s observable API (`Code`, `Message`, `Category`, `Context`, `Remediation`). The human rendering and the envelope carry the same information.
- The envelope's `schemaVersion` field is a stability marker. A consumer checks the version before descending; a future breaking change increments the version; an additive change (a new field) does not.
- The WBS 11.3.2 contract is satisfied. With `--format json`, stdout is exactly one JSON document on every exit code.
- The envelope is testable. The unit tests (`error_envelope_test.go`) assert its shape; the integration test (`binary_integration_test.go`) asserts the binary's stdout parses as JSON on every failure path.
- The envelope's field order is deterministic (`encoding/json` marshals struct fields in declaration order), so the bytes are predictable.

**Cons:**

- A new artifact to maintain. The envelope's type, the encoder, the schema, and the tests must be kept in sync. The CI check `scripts/deps/verify-output-streams.sh` catches a change to one without the others.
- A stability commitment. A future breaking change to the envelope requires incrementing `schemaVersion` and updating the schema and the tests. The commitment is a cost, but it is the mechanism that lets a consumer detect a breaking change.
- The `--format json` failure path now writes to stdout, which is a change from the previous behaviour. A consumer that read only stderr on failure must be updated to read stdout; the change is documented in `docs/reference/output-streams.md` and in the CLI's migration notes for the flag.
- The envelope's `context` map's keys are code-specific. A consumer that depends on a specific key must read the code's documentation; the schema does not enumerate the keys.

### Option C — A JSON error envelope written to stderr in JSON mode

Introduce the envelope but write it to stderr, keeping stdout empty on failure.

**Pros:**

- The stream assignment is unchanged: stdout for success, stderr for failure.
- The envelope is available to a consumer that reads stderr.
- A consumer that reads only stdout is unaffected by the change.

**Cons:**

- The WBS 11.3.2 contract is violated. The contract says stdout is exactly one JSON document in JSON mode; an empty stdout is not a JSON document.
- A JSON consumer that reads stdout has no document to parse. It must read stderr, which mixes with informational diagnostics (a future command's progress output) and is not the "result" stream the contract names.
- The stream assignment becomes format-dependent: stdout for text success, stderr for text failure, stdout for JSON success, stderr for JSON failure. The rule "stdout carries the result" is broken for JSON failures, where the result (the error) is on stderr.
- The consumer's code must branch on format to know which stream to read on failure; the branch is the kind of complexity the envelope exists to avoid.

### Option D — A JSON error envelope on both stdout and stderr

Write the envelope to stdout and a human-readable rendering to stderr, both in JSON mode.

**Pros:**

- Both a machine consumer (reading stdout) and a human (reading stderr) see the failure in their preferred form.
- The envelope is available on stdout, satisfying the WBS 11.3.2 contract.
- The human rendering on stderr is unchanged from text mode, so a user who set `--format json` but is reading the output in a terminal sees the familiar block.

**Cons:**

- The envelope and the human block duplicate information. A machine consumer that reads both must decide which to parse; the duplication is noise.
- The human block's presence on stderr in JSON mode is unexpected. A JSON consumer that logs stderr alongside stdout sees both the envelope and the human block; the duplication is confusing.
- The "diagnostics on stderr" rule is stretched: the human error block is not a diagnostic in the sense the rule uses; it is the failure's content in a different form.
- The choice between Option B and Option D is a policy decision about whether JSON mode is "for machines only" (Option B) or "for machines, with a human fallback on stderr" (Option D). Option B is the simpler and more consistent contract.

---

## 3. Decision

Adopt Option B: a JSON error envelope written to stdout on failure in JSON mode, with a versioned schema.

The envelope's shape is exactly:

```json
{
  "schemaVersion": "1",
  "exitCode": <int>,
  "error": {
    "code": <string>,
    "message": <string>,
    "category": <string>,
    "context": {<string>: <string>, ...},
    "remediation": <string>
  }
}
```

The top-level fields are `schemaVersion` (the envelope's shape version, currently `"1"`), `exitCode` (the process's exit code, in `[0, 255]`), and `error` (the structured error data). The `error` sub-object's fields are `code` (a registered error code, matching the pattern `^FORGE_[A-Z][A-Z0-9_]*$`), `message` (the error's short user-facing description, one line, no trailing period, at most 80 bytes), `category` (one of the closed taxonomy's seven members), `context` (an error-specific key-value map, omitted when empty), and `remediation` (the error's user-facing guidance, omitted when empty).

The envelope is produced by `encodeErrorEnvelope` from a `*forgeerr.Error`, written to stdout by `writeFailure` when `--format json` is set, and published as `schemas/error-envelope.schema.json`. The fields are additive: a future release may add new fields but never removes or renames an existing one. The `schemaVersion` field is incremented on a breaking change; a new field does not increment it.

The decision was made because three requirements pointed at the same shape:

1. **The WBS 11.3.2 contract requires a JSON document on stdout in JSON mode on every exit code.** The envelope is that document for the failure path. Option A cannot satisfy the contract; Option C violates it.
2. **A machine consumer needs a data interface, not a rendering.** The envelope's schema is the data interface; the human-rendered block is the rendering. Options A and D conflate the two; Option B separates them.
3. **The envelope's stability must be detectable.** The `schemaVersion` field lets a consumer check the shape before descending. Option B provides it; the other options do not.

The trade-offs accepted in choosing Option B:

- **A new artifact to maintain.** The envelope's type, the encoder, the schema, the reference document, and the tests must be kept in sync. The CI check `scripts/deps/verify-output-streams.sh` catches a change to one without the others; the cost of the check is the cost of the sync.
- **A stability commitment.** A future breaking change requires incrementing `schemaVersion` and updating the schema and the tests. The commitment is the mechanism that lets a consumer detect a breaking change; the cost is that a future change's shape is constrained.
- **The `--format json` failure path's stream assignment changes.** A consumer that read only stderr on failure must be updated to read stdout. The change is documented in `docs/reference/output-streams.md` and in the migration notes for the flag.
- **The envelope's `context` keys are code-specific.** The schema does not enumerate the keys; a consumer that depends on a specific key must read the code's documentation. The trade-off is that the schema is stable across codes; the cost is that a consumer's dependence on a specific key is not visible in the schema.

Two assumptions the decision depends on:

- **`encoding/json` is the encoder.** The envelope's field order is deterministic because `encoding/json` marshals struct fields in declaration order and sorts map keys. A future change to a different encoder would need to preserve the determinism. The current assumption is that `encoding/json`'s behaviour is stable; the assumption is validated by the tests that assert the envelope's byte-exact form.
- **The error's observable API is the envelope's content.** The envelope carries `Code`, `Message`, `Category`, `Context`, and `Remediation` verbatim. A future change to `forgeerr.Error`'s API that adds a field would require a corresponding change to the envelope; the change is additive (a new envelope field) and does not increment `schemaVersion`. The current assumption is that the API is stable; the assumption is validated by the tests that assert the envelope's fields.

The decision is not provisional. The envelope's shape is a public contract; a breaking change to it requires a new ADR. The `schemaVersion` field is the mechanism for detecting a breaking change; the version is incremented when the envelope's shape changes, not when a new field is added.

---

## 4. Consequences

### 4.1 Positive

- **The envelope is a data interface.** A machine consumer validates the envelope against `schemas/error-envelope.schema.json` without parsing prose. The consumer's code reads `error.code`, `error.message`, and `error.context` by name; the parsing is JSON, not text matching.
- **The WBS 11.3.2 contract is satisfied.** With `--format json`, stdout is exactly one JSON document on every exit code, including failures. The consumer parses stdout without a fallback.
- **The envelope mirrors `forgeerr.Error`'s API.** The envelope carries the same information the human rendering does: code, message, category, context, remediation. A consumer that reads the envelope and a human that reads the block see the same facts.
- **The envelope is versioned.** The `schemaVersion` field is a stability marker. A consumer checks the version before descending; a breaking change increments it; an additive change does not.
- **The envelope is testable.** The unit tests assert its shape, its determinism, and its validity for every category. The integration test asserts the binary's stdout parses as JSON on three distinct failure modes.
- **The envelope's field order is deterministic.** `encoding/json` marshals struct fields in declaration order and sorts map keys, so the envelope's bytes are predictable. A golden file can pin them.
- **The envelope's `context` and `remediation` fields are omitted when empty.** The `omitempty` tag makes the envelope smaller when a field is absent. A consumer that expects the field handles its absence; a consumer that does not need the field parses a smaller document.

### 4.2 Negative

- **A new artifact to maintain.** The envelope's type (`errorEnvelope`, `errorEnvelopeError`), the encoder (`encodeErrorEnvelope`), the writer (`writeFailure`), the schema (`schemas/error-envelope.schema.json`), the reference document (`docs/reference/output-streams.md`), and the tests (`error_envelope_test.go`, `output_test.go`, `binary_integration_test.go`) must be kept in sync. The CI check catches a change to one without the others; the cost of the check is the cost of the sync.
- **A stability commitment.** A breaking change to the envelope's shape requires incrementing `schemaVersion` and updating the schema and the tests. The commitment is the mechanism that lets a consumer detect a breaking change; the cost is that a future change's shape is constrained.
- **The `--format json` failure path's stream assignment changes.** A consumer that read only stderr on failure must be updated to read stdout. The change is documented in `docs/reference/output-streams.md`; the consumer's update is the consumer's responsibility. The CLI is pre-1.0, so the change is permissible; a post-1.0 change of the same shape would not be.
- **The envelope's `context` keys are code-specific.** The schema does not enumerate the keys; a consumer that depends on a specific key must read the code's documentation. The trade-off is that the schema is stable across codes; the cost is that a consumer's dependence on a specific key is not visible in the schema.
- **The envelope's `message` field is at most 80 bytes.** The constraint comes from `forgeerr.Error`'s message invariants. A future error whose message is longer must either be shortened or use the `context` field for the excess. The trade-off is that the envelope's messages are short; the cost is that a verbose error must be split between the message and the context.
- **The envelope's `remediation` field is prose.** A consumer that wants structured guidance must parse the prose. The trade-off is that the guidance is human-authored; the cost is that a consumer cannot depend on its structure.
- **The envelope's `exitCode` field is redundant with the process's exit status.** A consumer that reads the envelope's `exitCode` and the process's exit status sees the same value; the redundancy is a convenience (a consumer that reads only stdout knows the code). The trade-off is one redundant field; the cost is none.
- **The envelope's writer's error is unchecked.** `writeFailure` returns an error, but `runWithOptions` discards it: the exit code is the primary signal, and a failed write on stdout is not a reason to change the classification. The trade-off is that a broken pipe or a closed stream produces no envelope; the cost is that the consumer sees no envelope and must infer the failure from the exit code.

### 4.3 Neutral

- **The envelope's `schemaVersion` field is currently `"1"`.** The value is a string, not an integer. The choice is a convention; a future version could be `"2"` or `"2.0"` or `2`. The value's type is part of the envelope's shape; changing it would require incrementing the version.
- **The envelope's `error.context` is a `map[string]string`.** The map's values are strings, not arbitrary JSON values. The choice is a constraint: a context value that is not a string must be stringified before being placed in the map. The constraint is the cost of the simple schema.
- **The envelope's `error.category` is a string, not an integer.** The category is the taxonomy's string name (`"CONFIG"`, not a numeric code). The choice is a convention: the string is human-readable in a log and machine-readable in a parser. The cost is that a consumer compares strings, not integers.
- **The envelope's `error.code` is a string, not an integer.** Same rationale as the category; the code is `FORGE_CONFIG_MALFORMED`, not `10`. The string is the registry's identifier; the exit code (which is the numeric value) is the envelope's top-level `exitCode` field.
- **The envelope's `exitCode` is at the top level, not inside `error`.** The choice is a convenience: a consumer that wants only the exit code reads it without descending. The cost is one redundant field; the benefit is a shorter path for the common case.
- **The envelope's success counterpart is sketched but not implemented.** The success envelope's shape is `{"schemaVersion":"1","exitCode":0,"result":{...}}`; the `result` field's structure is deferred to Phase 10 WBS 5.0. The current state is that the failure envelope is the decision's subject; the success envelope's content is a future decision.

---

## 5. Related Requirements

- `FR-CLI-OUTPUT-010` — the JSON output contract. This ADR's primary requirement.
- `FR-CLI-EXIT-024` — the stdout/stderr contract per exit code. The envelope is the failure path's stdout content in JSON mode.
- `FR-CLI-EXIT-001` — the range taxonomy. The envelope's `exitCode` field is a value from the taxonomy.
- `FR-CLI-EXIT-021` — the error-to-exit-code mapping. The envelope's `exitCode` field is the mapping's output.
- `FR-CLI-EXIT-023` — the single exit boundary. The boundary's rendering is the mechanism that produces the envelope.
- `NFR-QUALITY-005` — the CI-enforceable policy. The envelope's tests and the CI check are the enforcement mechanisms.
- `SEC-CLI-001` — the redaction rule. The envelope carries the error's context; the redaction is applied before the envelope is built.

> The following requirements must be added to the register when
> WBS 1.4 is complete: `FR-CLI-OUTPUT-011` (the success envelope's
> shape), `FR-CLI-OUTPUT-012` (the envelope's schema version
> policy), and `NFR-QUALITY-009` (the envelope's test coverage).

---

## 6. Related ADRs

- `ADR-020` — stdout/stderr Contract per Exit Code. The contract decides that the failure path's stdout content is a JSON document in JSON mode; this ADR decides the document's shape. The two are siblings; a reader who wants the contract reads ADR-020; a reader who wants the document reads this ADR.
- `ADR-019` — Single Exit Boundary in main. The boundary is the mechanism that writes the envelope. The boundary's rendering of the envelope is the mechanism this ADR depends on.
- `ADR-018` — Exit Code Migration and Stability Policy. The envelope's `exitCode` field is a value from the taxonomy this ADR freezes. The codes' values are ADR-018's subject.
- `ADR-019` (the Exit Code Semantics ADR, if numbered separately) — Exit Code Semantics. The severity classification applies to the codes; the envelope does not carry severity. A future change could add a `severity` field; the change would be additive and would not increment `schemaVersion`.
- `ADR-014` — User-Facing Error Format. The human-readable rendering that the envelope's fields mirror. The two are documented together in `docs/reference/output-streams.md`.
- `ADR-016` — Error Redaction Policy. The redaction rules apply to the error's context; the envelope carries the redacted context verbatim.
- `ADR-012` — Error Code Registry and Stability Policy. The error codes (the strings) are the envelope's `error.code` field. The codes' stability is the subject of ADR-012.
- `ADR-011` — Closed Error Category Taxonomy. The categories are the envelope's `error.category` field. The categories' closure is the subject of ADR-011.

---

## 7. Notes

### Open questions deferred by this ADR

- **The success envelope's `result` shape.** The success envelope's outer shape is frozen (`{"schemaVersion":"1","exitCode":0,"result":{...}}`), but the `result` field's structure is not. Phase 10 WBS 5.0 will decide the structure for the config command; a future WBS will decide it for other commands. The decision is deferred; the current state is that the outer shape is a contract and the inner shape is not.
- **The schema's evolution policy.** The schema's `schemaVersion` field is incremented on a breaking change. A future ADR should record the exact criteria for "breaking" and the procedure for incrementing the version. The current state is that the version is `"1"` and a breaking change increments it; the criteria are not enumerated.
- **The envelope's interaction with `--verbose`.** The envelope carries the same fields regardless of `--verbose`. A future change could add a `verbose` field (or a nested `cause` chain) to the envelope; the change would be additive and would not increment `schemaVersion`. The decision is deferred; the current state is that `--verbose` does not alter the envelope.
- **The envelope's interaction with redaction.** The envelope carries the error's context after redaction. A future change to the redaction rules (for example, to redact a new secret key) would apply to the envelope automatically, because the envelope reads the context after redaction. The decision is deferred; the current state is that the redaction is applied before the envelope is built.
- **The envelope's size limit.** The envelope has no explicit size limit. A future error whose context is large could produce a large envelope. The decision is deferred; the current state is that no limit exists and the envelope's size is bounded by the error's context.
- **The envelope's `exitCode` field's range.** The field is an integer in `[0, 255]`. A future error whose category's code is outside the range would produce an invalid envelope; the range check is a constraint on the taxonomy, not on the envelope. The current state is that the taxonomy's codes are all in `[0, 255]`; the envelope's validation is a consequence.
- **The envelope's treatment of signal terminations.** A signal-terminated process (exit code 130 or 143) writes nothing to stdout, so no envelope is produced. A future change could produce an envelope for signal terminations; the change would be additive and would not increment `schemaVersion`. The decision is deferred; the current state is that signals bypass the envelope.

### Follow-up decisions anticipated

- **Phase 10 WBS 5.0** (JSON output contract). The success envelope's `result` shape is the WBS's subject. This ADR's contract names the failure envelope's shape; the WBS fills in the success envelope's content.
- **Phase 9 WBS 6.4.1** (config command output formats). The config command is the envelope's first application. The command's output in JSON is the subject of its own WBS; the envelope's shape is this ADR's subject.
- **A future ADR on the schema's evolution policy.** The criteria for "breaking" and the procedure for incrementing the version are deferred. The ADR would record the policy that the schema's stability depends on.

### Links to external discussion

- WBS 11.3.2 — the task that requested the envelope.
- `docs/reference/output-streams.md` — the contract's human-readable form, including the envelope's shape.
- `schemas/error-envelope.schema.json` — the envelope's machine-readable schema.
- `internal/cli/error_envelope.go` — the envelope's type and encoder.
- `internal/cli/output.go` — the format-aware writer that produces the envelope.
- `internal/cli/error_envelope_test.go` and `internal/cli/output_test.go` — the unit tests that assert the envelope's shape.
- `cmd/forge/binary_integration_test.go` — the integration test that asserts the binary's stdout parses as JSON on every failure path.
- `scripts/deps/verify-output-streams.sh` — the CI check that asserts the envelope's artifacts are present and consistent.

---

*End of ADR-021.*
