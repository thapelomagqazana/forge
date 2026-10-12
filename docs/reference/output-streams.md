# Output Streams

<!--
  Document:     output-streams.md
  Purpose:      The stdout/stderr contract for every exit code.
                Normative for all consumers.
  Audience:     CLI users, script authors, CI/CD integrators, IDE
                extension authors, wrapper-script authors.
  Stability:    Public contract. Additive only.
  Traceability: FR-CLI-EXIT-024, FR-CLI-OUTPUT-010, WBS 11.3.2
  Last Review:  2026-10-12
-->

## Overview

Every Forge invocation writes to at most one of its two output
streams — stdout or stderr — for its **primary** output. The two
streams have fixed roles:

- **stdout** carries the command's result. In text mode, the
  result is human-readable. In JSON mode (`--format json`), the
  result is exactly one JSON document (a success object or an
  error envelope, depending on whether the command succeeded).
- **stderr** carries diagnostics: errors, warnings, and other
  messages that accompany a failure or qualify a success.

The contract is a property of every invocation, not of individual
commands. It is enforced by:

- The rendering path in `internal/cli/execute.go` (which writes
  the error envelope or the human error block, depending on
  `--format`).
- The test suite in `internal/cli/error_envelope_test.go` and
  `internal/cli/output_test.go`.
- The integration test in `cmd/forge/binary_integration_test.go`
  (which asserts the binary's observable behaviour).
- The CI check `scripts/deps/verify-output-streams.sh` (which
  asserts the reference documentation and the schema are present
  and consistent).

## The contract per exit code

| Exit code | Category | stdout (text) | stdout (JSON) | stderr |
|-----------|----------|---------------|----------------|--------|
| `0` Success | OK | Command result; empty when the command has nothing to print. | `{"schemaVersion":"1","exitCode":0,"result":{...}}` | Empty. |
| `1` Findings (drift, check failed) | ERROR | Finding report. | `{"schemaVersion":"1","exitCode":1,"error":{...}}` | Diagnostic (unless `--quiet` suppresses stderr; see below). |
| `2` Validation warning (escalated) | ERROR | Finding report. | `{"schemaVersion":"1","exitCode":2,"error":{...}}` | Diagnostic. |
| `3` Check failed (alias) | ERROR | Finding report. | `{"schemaVersion":"1","exitCode":3,"error":{...}}` | Diagnostic. |
| `4` Ambiguity (`init` cannot decide) | ERROR | Candidates list. | `{"schemaVersion":"1","exitCode":4,"error":{...}}` | Diagnostic. |
| `10–17` Execution failures (config, template, fs, security, internal) | ERROR or FATAL | Empty. | `{"schemaVersion":"1","exitCode":N,"error":{...}}` | Error block. |
| `64` Usage error | ERROR | Empty. | `{"schemaVersion":"1","exitCode":64,"error":{...}}` | Error block. |
| `70` Internal error | FATAL | Empty. | `{"schemaVersion":"1","exitCode":70,"error":{...}}` | Error block. |
| `130` SIGINT | — | Empty. | (No output; signal termination bypasses the JSON contract.) | Empty or cleanup trace. |
| `143` SIGTERM | — | Empty. | (No output.) | Empty or cleanup trace. |

Two invariants hold across every row:

1. **stdout is either the result or empty.** Never partial output,
   never partial JSON.
2. **A non-zero exit code with `--format json` produces a valid
   JSON envelope on stdout.** The envelope's `exitCode` field
   equals the process's exit code.

## The JSON error envelope

When `--format json` is set and the command fails, stdout carries
exactly one JSON object:

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

- **`schemaVersion` is at the top.** A consumer that parses the
  envelope can check the version before inspecting the error.
  A future breaking change to the envelope increments the
  version.
- **`exitCode` is at the top.** The exit code is the consumer's
  primary classification; putting it at the top makes the envelope
  usable without descending into `error`.
- **The `error` sub-object mirrors `forgeerr.Error`.** The
  sub-object's fields (`code`, `message`, `category`, `context`,
  `remediation`) match the error's API. A consumer that reads the
  human rendering and a consumer that reads the JSON see the same
  information.

### Field stability

Every field in the envelope is **additive**: a future release may
add new fields, but never removes or renames an existing one. A
consumer that ignores unknown fields continues to work.

The `schemaVersion` field is a stability marker for the envelope's
**shape**. A new field does not increment the version; a change to
an existing field's meaning or type does. The policy mirrors the
one for exit codes (ADR-018) and for error codes (ADR-012).

### `context` is a free-form map

The `context` field's keys and values are error-specific. Every
code documents its own context keys in
[`error-codes.md`](error-codes.md). A consumer should not depend on
specific keys unless the code's documentation names them.

## Interaction with `--quiet`

The `--quiet` flag suppresses diagnostics on stderr. It does **not**
suppress the JSON envelope on stdout, because the envelope is the
command's result (in JSON mode) and a machine consumer must be able
to parse it regardless of the diagnostic verbosity.

| Exit code | `--quiet` effect on stdout | `--quiet` effect on stderr |
|-----------|----------------------------|----------------------------|
| `0` | None (the result is printed). | None (stderr is already empty). |
| Findings (`1–9`) | In text mode, the finding report is **suppressed**; the exit code still signals the finding. In JSON mode, the envelope is printed. | The diagnostic is suppressed. |
| Execution failures (`10–19`, `64`, `70`) | None (stdout is already empty in text mode; the envelope is printed in JSON mode). | The diagnostic is suppressed. |

The rule's rationale: `--quiet` means "do not print diagnostics",
not "do not print the result". In JSON mode the envelope is the
result, so it is printed. In text mode a finding's report is the
result, so `--quiet` suppressing it means "the exit code is the
only signal"; that is a deliberate choice, documented here.

## Interaction with `--verbose`

`--verbose` adds fields to the human-rendered error block on
stderr: the cause chain, the stack trace, and the
`internal_`-prefixed context keys. It does **not** alter the JSON
envelope, which carries the same fields regardless of the flag.
The rationale: machine consumers should not need to set
`--verbose` to receive complete error data; the envelope is
complete by construction.

## Signals

A signal-terminated process (exit code 130 or 143) writes nothing
to stdout and, in most cases, nothing to stderr. The signal is
the user's or supervisor's decision; there is no error to report.
A future WBS may add a cleanup trace to stderr; the contract
reserves stderr for the trace if it is added.

The JSON envelope is **not** produced for signal terminations.
A machine consumer that observes exit code 130 or 143 knows the
termination was a signal; there is no error object to parse.

## Why the contract is per-code, not per-command

A command's output shape is a function of its exit code, not of
its name. The reason: the exit code is the consumer's primary
classification, and the contract must be a function of that
classification alone.

A hypothetical future command that produces a "report" output
uses the same contract as `check` and `diff`: exit 0 means the
report is on stdout; exit non-zero means the finding is on
stdout (text) or the error envelope is on stdout (JSON) and the
diagnostic is on stderr.

## References

- [`exit-codes.md`](exit-codes.md) — the frozen per-code table.
- [`exit-code-ranges.md`](exit-code-ranges.md) — the range taxonomy.
- [`error-codes.md`](error-codes.md) — the error code registry.
- `schemas/error-envelope.schema.json` — the envelope's schema.
- `docs/cli-ux-spec.md` § 4.12 — flag semantics.
- WBS 11.3.2, FR-CLI-EXIT-024, FR-CLI-OUTPUT-010.
