# Output Streams

<!--
  Document:     output-streams.md
  Purpose:      The stdout/stderr contract for every exit code and
                every output layer (output, logger, errors).
                Normative for all consumers.
  Audience:     CLI users, script authors, CI/CD integrators, IDE
                extension authors, wrapper-script authors, log
                aggregators, monitoring systems.
  Stability:    Public contract. Additive only.
  Traceability: FR-CLI-EXIT-024, FR-CLI-OUTPUT-001, FR-CLI-OUTPUT-010,
                FR-CLI-LOG-001, ADR-018, ADR-020, ADR-021, ADR-022,
                WBS 11.3.2, WBS 12.1.1
  Last Review:  2026-10-12
  See also:     ../architecture/output-vs-logging.md
-->

## Overview

Every Forge invocation writes to its two standard streams according to a
**fixed, per-invocation contract**. The contract is a function of the
exit code, not of the command name, and it is the same regardless of
which command produced the result.

The two streams have distinct, non-overlapping roles:

- **stdout** carries the command's **product output** — the result the
  user (human or machine) asked for. In text mode, the result is
  human-readable. In JSON mode (`--format json`), the result is exactly
  one JSON document (a success envelope or an error envelope, depending
  on whether the command succeeded).
- **stderr** carries **diagnostics** — errors and operational log
  records that accompany a failure or qualify a success. Diagnostics
  never appear on stdout.

This two-layer split (product surface vs. operational surface) is defined
in detail [`docs/architecture/output-vs-logging.md`](../architecture/output-vs-logging.md).
This document is the **per-exit-code projection** of that architecture —
the table a consumer can rely on.

The contract is enforced by:

- The rendering path in `internal/cli/execute.go` (which writes the
  error envelope or the human error block, depending on `--format`).
- The output layer in `internal/output/` (which writes product output to
  stdout only).
- The logger layer in `internal/logger/` (which writes diagnostics to
  stderr only).
- The test suite in `internal/cli/contract_test.go`,
  `internal/cli/error_envelope_test.go`, `internal/cli/output_test.go`,
  `internal/output/writer_test.go`, and `internal/logger/logger_test.go`.
- The integration test in `cmd/forge/binary_integration_test.go` (which
  asserts the binary's observable behaviour).
- The CI check `scripts/deps/verify-output-streams.sh` (which asserts
  this document and the schemas are present and consistent).

## The three layers

Before the per-exit-code table, it helps to name the three layers that
write to the streams. This is the same taxonomy used in
[`docs/architecture/output-vs-logging.md`](../architecture/output-vs-logging.md).

| Layer | Package | Stream | Purpose |
|-------|---------|--------|---------|
| **Output** | `internal/output` | stdout | Product result. |
| **Logger** | `internal/logger` | stderr | Operational diagnostics. |
| **Errors** | `internal/forgeerr` | stderr | Error envelope / human error block. |

The layers are **disjoint**: a byte on stdout is always output, and a
byte on stderr is always a diagnostic (logger record or error). No
byte is ever both.

## The contract per exit code

The table below is the normative contract. Every row states what a
consumer may expect on each stream.

| Exit code | Category | stdout (text) | stdout (JSON) | stderr |
|-----------|----------|---------------|----------------|--------|
| `0` Success | OK | Command result; empty when the command has nothing to print. | `{"schemaVersion":"1","exitCode":0,"result":{...}}` | Logger records only. |
| `1` Findings (drift, check failed) | ERROR | Finding report. | `{"schemaVersion":"1","exitCode":1,"error":{...}}` | Logger records and/or the human error block (unless `--quiet`). |
| `2` Validation warning (escalated) | ERROR | Finding report. | `{"schemaVersion":"1","exitCode":2,"error":{...}}` | Logger records and/or the human error block. |
| `3` Check failed (alias) | ERROR | Finding report. | `{"schemaVersion":"1","exitCode":3,"error":{...}}` | Logger records and/or the human error block. |
| `4` Ambiguity (`init` cannot decide) | ERROR | Candidates list. | `{"schemaVersion":"1","exitCode":4,"error":{...}}` | Logger records and/or the human error block. |
| `10–17` Execution failures (config, template, fs, security, internal) | ERROR or FATAL | Empty. | `{"schemaVersion":"1","exitCode":N,"error":{...}}` | Error block (human) or logger records; never output. |
| `64` Usage error | ERROR | Empty. | `{"schemaVersion":"1","exitCode":64,"error":{...}}` | Error block (human) or logger records. |
| `70` Internal error | FATAL | Empty. | `{"schemaVersion":"1","exitCode":70,"error":{...}}` | Error block (human) or logger records. |
| `130` SIGINT | — | Empty. | (No output; signal termination bypasses the JSON contract.) | Empty or cleanup trace. |
| `143` SIGTERM | — | Empty. | (No output.) | Empty or cleanup trace. |

Four invariants hold across every row:

1. **stdout is either the result or empty.** Never partial output, never
   partial JSON, never diagnostics.
2. **stderr is never product output.** It may carry logger records
   and/or an error block, but never the command's result.
3. **A non-zero exit code with `--format json` produces a valid JSON
   envelope on stdout.** The envelope's `exitCode` field equals the
   process's exit code.
4. **Logger records continue on stderr, unchanged, regardless of
   `--format`.** Under `--format json`, logger records are never injected
   into stdout.

> **Note on "at most one stream."** Earlier versions of this document
> stated that every invocation writes to at most one stream for its
> primary output. That remains true for *product output*: the result
> lives on stdout, or stdout is empty. It is not true for *diagnostics*:
> logger records may accompany a success on stderr. The per-exit-code
> table above is the authoritative statement.

## The JSON error envelope

When `--format json` is set and the command fails, stdout carries exactly
one JSON object:

```json
{
  "schemaVersion": "1",
  "exitCode": 10,
  "error": {
    "code": "FORGE_CONFIG_MALFORMED",
    "message": "invalid YAML in config.yaml",
    "category": "CONFIG",
    "context": {
      "path": "config.yaml",
      "line": 12
    },
    "remediation": "Fix the YAML syntax error at the indicated line."
  }
}
```

The envelope's schema is published at
[`../../schemas/error-envelope.schema.json`](../../schemas/error-envelope.schema.json).

### Why the envelope is a wrapper, not a flat object

The envelope wraps the error's data in an outer object so that:

- **`schemaVersion` is at the top.** A consumer that parses the envelope
  can check the version before inspecting the error. A future breaking
  change to the envelope increments the version.
- **`exitCode` is at the top.** The exit code is the consumer's primary
  classification; putting it at the top makes the envelope usable without
  descending into `error`.
- **The `error` sub-object mirrors `forgeerr.Error`.** The sub-object's
  fields (`code`, `message`, `category`, `context`, `remediation`) match
  the error's API. A consumer that reads the human rendering and a
  consumer that reads the JSON see the same information.

### Success envelope

For symmetry, when `--format json` is set and the command succeeds,
stdout carries exactly one JSON object:

```json
{
  "schemaVersion": "1",
  "exitCode": 0,
  "result": { }
}
```

The `result` field's shape is command-specific. The envelope's outer
shape is stable.

### Field stability

Every field in both envelopes is **additive**: a future release may add
new fields, but never removes or renames an existing one. A consumer
that ignores unknown fields continues to work.

The `schemaVersion` field is a stability marker for the envelope's
**shape**. A new field does not increment the version; a change to an
existing field's meaning or type does. The policy mirrors the one for
exit codes ([ADR-018](../decisions/ADR-018-exit-code-migration-and-stability-policy.md))
and for error codes ([ADR-012](../decisions/ADR-012-error-code-registry-and-stability-policy.md)).

### `context` is a free-form map

The `context` field's keys and values are error-specific. Every code
documents its own context keys in [`error-codes.md`](error-codes.md). A
consumer should not depend on specific keys unless the code's
documentation names them.

### Exactly one JSON document

Under `--format json`, stdout contains **exactly one** JSON document,
followed by a single newline. There is no leading banner, no trailing
log line, and no second document. This is enforced by
`internal/output/writer.go` (which uses a single `json.Encoder.Encode`
call) and asserted by
`internal/cli/contract_test.go::TestOutputLoggingSeparation_StdoutOnlyContainsJSON`.

## The logger stream

Logger records are the third category of bytes on the streams. They are
not part of the per-exit-code result contract, but they are part of the
stream contract: a consumer that reads stderr must be prepared for them.

### When logger records are emitted

Logger records are emitted on stderr at the level selected by
`--log-level` (default `info`). Emitting a logger record never changes
the exit code, never changes stdout, and never changes the error
envelope.

### Logger formats

| `--log-format` | Record shape |
|----------------|--------------|
| `text` (default) | `TIMESTAMP LEVEL MESSAGE key=value ...` |
| `json` | `{"ts":"...","level":"...","msg":"...","attrs":{...}}` |

### Logger and JSON output

Under `--format json`, logger records continue on stderr in the format
selected by `--log-format`, which is independent of `--format`. This is
the interaction rule:

> When `--format json` is active, stdout contains exactly one JSON
> document — the output envelope. The logger continues to emit to stderr
> unchanged. No logger line is ever injected into stdout.

### Logger suppression

`--quiet` suppresses logger records at `info` and `debug` levels. It does
not suppress `warn` or `error` records, and it does not suppress the
JSON envelope on stdout.

## Interaction with `--quiet`

The `--quiet` flag suppresses diagnostics on stderr. It does **not**
suppress the JSON envelope on stdout, because the envelope is the
command's result (in JSON mode) and a machine consumer must be able to
parse it regardless of the diagnostic verbosity.

| Exit code | `--quiet` effect on stdout | `--quiet` effect on stderr |
|-----------|----------------------------|----------------------------|
| `0` | None (the result is printed). | Logger records at `info`/`debug` are suppressed. |
| Findings (`1–9`) | In text mode, the finding report is **suppressed**; the exit code still signals the finding. In JSON mode, the envelope is printed. | Logger records at `info`/`debug` are suppressed; the error block is suppressed. |
| Execution failures (`10–19`, `64`, `70`) | None (stdout is already empty in text mode; the envelope is printed in JSON mode). | Logger records at `info`/`debug` are suppressed; the error block is suppressed. |

The rule's rationale: `--quiet` means "do not print diagnostics", not
"do not print the result". In JSON mode the envelope is the result, so
it is printed. In text mode a finding's report is the result, so
`--quiet` suppressing it means "the exit code is the only signal"; that
is a deliberate choice, documented here.

## Interaction with `--verbose`

`--verbose` adds fields to the human-rendered error block on stderr: the
cause chain, the stack trace, and the `internal_`-prefixed context keys.
It **does not** alter the JSON envelope, which carries the same fields
regardless of the flag. The rationale: machine consumers should not need
to set `--verbose` to receive complete error data; the envelope is
complete by construction.

`--verbose` also raises the effective log level to `debug` if the user
has not explicitly set `--log-level`. It never causes logger records to
appear on stdout.

## Signals

A signal-terminated process (exit code 130 or 143) writes nothing to
stdout and, in most cases, nothing to stderr. The signal is the user's
or supervisor's decision; there is no error to report. A future WBS may
add a cleanup trace to stderr; the contract reserves stderr for the
trace if it is added.

The JSON envelope is **not** produced for signal terminations. A machine
consumer that observes exit code 130 or 143 knows the termination was a
signal; there is no error object to parse.

## Why the contract is per-code, not per-command

A command's output shape is a function of its exit code, not of its
name. The reason: the exit code is the consumer's primary
classification, and the contract must be a function of that
classification alone.

A hypothetical future command that produces a "report" output uses the
same contract as `check` and `diff`: exit 0 means the report is on
stdout; exit non-zero means the finding is on stdout (text) or the error
envelope is on stdout (JSON) and the diagnostic is on stderr.

## Consumer checklist

A consumer that reads Forge's streams should:

1. **Read stdout as the result.** Parse it as JSON when `--format json`
   is set; otherwise treat it as human-readable text.
2. **Read stderr as diagnostics.** Never parse it as product output.
   Logger records and error blocks may both appear.
3. **Use the exit code as the primary classification.** Do not infer
   success from stdout contents.
4. **Tolerate unknown JSON fields.** The contract is additive.
5. **Check `schemaVersion` before interpreting the envelope.** A future
   breaking change increments it.
6. **Expect exactly one JSON document on stdout under `--format json`.**
   If parsing fails or trailing data is present, that is a bug in Forge;
   please report it.
7. **Expect logger records on stderr under any `--format`.** Do not
   treat their presence as an error.

## References

- [`exit-code-ranges.md`](exit-code-ranges.md) — the range taxonomy.
- [`error-codes.md`](error-codes.md) — the error code registry.
- [`../architecture/output-vs-logging.md`](../architecture/output-vs-logging.md)
  — the two-layer output/logging architecture.
- [`../decisions/ADR-018-exit-code-migration-and-stability-policy.md`](../decisions/ADR-018-exit-code-migration-and-stability-policy.md)
  — exit-code stability.
- [`../decisions/ADR-020-stdout-stderr-contract-per-exit-code.md`](../decisions/ADR-020-stdout-stderr-contract-per-exit-code.md)
  — the stdout/stderr contract.
- [`../decisions/ADR-021-json-error-envelope-for-format-json.md`](../decisions/ADR-021-json-error-envelope-for-format-json.md)
  — the JSON error envelope.
- `schemas/output-envelope.schema.json` — the success envelope's schema.
- `schemas/error-envelope.schema.json` — the error envelope's schema.
- `docs/cli-ux-spec.md` § 4.12 — flag semantics.
- WBS 11.3.2, WBS 12.1.1, FR-CLI-EXIT-024, FR-CLI-OUTPUT-001,
  FR-CLI-OUTPUT-010, FR-CLI-LOG-001.
  