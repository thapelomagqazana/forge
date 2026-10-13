# Output vs. Logging Architecture

> **Status:** Active
> **ADR:** [ADR-022](../decisions/ADR-022-output-logging-separation.md)
> **WBS:** 12.1.1
> **Owners:** Phase 6 (CLI Output), Phase 10 (JSON Output Contract)
> **Reviewers:** Architecture Owner, Phase 2 Owner (Structured Error System)
> **Last updated:** 2026-10-12

---

## 1. Purpose

This document defines the two-layer output architecture of the `forge` CLI.

Forge emits bytes on two distinct streams for two distinct purposes. This
document specifies:

1. What "output" and "logging" mean in Forge, with **no overlap**.
2. The **stream contract**: which producer writes to which stream.
3. The **interaction rule** that governs `--format json` behavior.
4. The **package boundaries** that make the separation enforceable.
5. A **decision procedure** that lets any contributor determine, from this
   document alone, which layer their new code belongs to.
6. The **testing strategy** that keeps the contract intact.

It is normative. Where code and this document disagree, the document wins
and the code must be corrected.

---

## 2. Definitions

### 2.1 Output

**Output is the product surface of the CLI.**

It is the *result* the user (human or machine) asked for. It is the CLI's
contract with the outside world — the thing a script, a pipeline, or a
downstream tool consumes.

Properties:

- **Stream:** stdout, and stdout only.
- **Formats:** `text` (for humans) and `json` (for machines), selected by
  the global `--format` flag.
- **Stability:** schema-versioned and golden-tested. Treated as a public
  API. Breaking changes require an ADR.
- **Package:** `internal/output`.
- **Vocabulary:** "prints", "renders", "emits", "produces".
- **Failure mode if misused:** downstream parsers break silently.

**Examples of output:**

- The JSON envelope produced by `forge version --format json`.
- The rendered configuration produced by `forge config`.
- The list of blueprints produced by `forge blueprint list`.
- The rendered help text for a command (Cobra convention).

### 2.2 Logger

**The logger is the operational surface of the CLI.**

It carries *diagnostics* — information about how a command ran, not what
it produced. Logs, traces, timing, debug context. It helps operators and
developers understand *how* the command behaved.

Properties:

- **Stream:** stderr, and stderr only.
- **Formats:** `text` (human-readable) and `json` (structured), selected
  by the global `--log-format` flag.
- **Levels:** `debug`, `info`, `warn`, `error`, selected by the global
  `--log-level` flag.
- **Stability:** semi-stable. The record shape is stable; the set of
  emitted events may change per release.
- **Package:** `internal/logger`.
- **Vocabulary:** "logs", "records", "emits", "traces", "warns".
- **Failure mode if misused:** noise on stderr, or — worse — corruption
  of stdout if the separation is violated.

**Examples of logger events:**

- "resolved version from build info" at `debug`.
- "cache miss, fetching remote" at `info`.
- "config file not found, using defaults" at `warn`.
- "retrying request" at `warn`.

### 2.3 Errors

**Errors are diagnostics about a failed product result.**

They are neither output nor ordinary log records. They flow through the
structured error system (`internal/forgeerr`, ADR-011..021) and are
rendered to **stderr**, never stdout, regardless of `--format`.

- **Stream:** stderr, and stderr only.
- **Formats:** rendered by the error system according to `--format`
  (text by default, JSON envelope under `--format json` per ADR-021).
- **Stability:** error codes are a public API (ADR-012). Categories and
  messages have defined stability guarantees.
- **Package:** `internal/forgeerr`.
- **Failure mode if misused:** errors appearing on stdout and breaking
  pipeline consumers, or missing exit codes.

### 2.4 No Overlap

The three categories are **disjoint**. A given byte emitted by Forge
belongs to exactly one of:

1. Output (stdout),
2. Logger (stderr),
3. Error (stderr).

There is no fourth category. There is no "both". A line that is "sort of
a result and sort of a diagnostic" is a design smell and must be resolved
into one category.

---

## 3. Stream Contract

### 3.1 Producers and Streams

```text
┌────────────────────────────────────────────────────────────────────┐
│                              forge                                 │
│                                                                    │
│   ┌──────────────────┐                                             │
│   │  command result  │                                             │
│   └────────┬─────────┘                                             │
│            │                                                       │
│            ▼                                                       │
│   ┌──────────────────┐        ┌──────────────┐                     │
│   │ internal/output  │───────►│    stdout    │  product stream     │
│   └──────────────────┘        └──────────────┘  (text | json)      │
│                                                                    │
│   ┌──────────────────┐                                             │
│   │   diagnostics    │                                             │
│   └────────┬─────────┘                                             │
│            │                                                       │
│            ▼                                                       │
│   ┌──────────────────┐        ┌──────────────┐                     │
│   │ internal/logger  │───────►│    stderr    │  operational stream │
│   └──────────────────┘        └──────────────┘  (text | json)      │
│                                                                    │
│   ┌──────────────────┐                                             │
│   │      error       │                                             │
│   └────────┬─────────┘                                             │
│            │                                                       │
│            ▼                                                       │
│   ┌──────────────────┐        ┌──────────────┐                     │
│   │ internal/forgeerr│───────►│    stderr    │  error stream       │
│   └──────────────────┘        └──────────────┘  (text | json)      │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

### 3.2 Stream Contract Table

| Producer | Stream | Format selector | JSON shape | Stability |
|----------|--------|-----------------|------------|-----------|
| **Output** | **stdout** | `--format {text,json}` | `output-envelope.schema.json` | Public API, schema-versioned |
| **Logger** | **stderr** | `--log-format {text,json}` | `log-record.schema.json` | Semi-stable, per-release |
| **Errors** | **stderr** | `--format {text,json}` | `error-envelope.schema.json` (ADR-021) | Public API, code-stable |
| **Help / usage** | **stdout** | text only | n/a | Cobra convention |
| **Version banner** | **stdout** | `--format {text,json}` | `version.schema.json` (ADR-005) | Public API, schema-versioned |

### 3.3 Flag Interactions

The three format-bearing flags are **independent**:

| Flag | Controls | Default |
|------|----------|---------|
| `--format` | Output and error rendering | `text` |
| `--log-format` | Logger rendering | `text` |
| `--log-level` | Logger verbosity | `info` |

Changing `--log-level` or `--log-format` **must not** alter stdout bytes.
Changing `--format` **must not** alter stderr logger bytes (except that
errors, which use `--format`, may change).

### 3.4 Exit Codes and Streams

The exit code and the streams are coupled by contract (ADR-018, ADR-020):

| Exit code | stdout | stderr |
|-----------|--------|--------|
| `0` (success) | Output (possibly empty) | Logger lines only |
| non-zero | **Empty** | Error envelope and/or logger lines |

On non-zero exit, stdout is empty. This lets consumers discard stdout on
failure without inspecting it.

---

## 4. Interaction Rule for `--format json`

### 4.1 The Rule

> **When `--format json` is active, stdout contains exactly one JSON
> document — the output envelope. The logger continues to emit to stderr
> unchanged. No logger line is ever injected into stdout.**

Formally:

- `len(parseJSONDocuments(stdout)) == 1` when the command succeeds.
- `stdout == ""` when the command fails.
- `stderr` may contain zero or more log records and/or one error envelope.
- `stdout ∩ stderr == ∅`.

### 4.2 Correct Behavior

```console
$ forge version --format json 2>/dev/null
{"schemaVersion":"1","command":"version","data":{"version":"1.2.3",...}}
```

```console
$ forge version --format json 1>/dev/null
{"ts":"2026-10-12T10:00:00Z","level":"debug","msg":"resolved version from build info"}
{"ts":"2026-10-12T10:00:00Z","level":"debug","msg":"rendering json envelope"}
```

### 4.3 Incorrect Behavior (Bug)

```console
$ forge version --format json
{"ts":"2026-10-12T10:00:00Z","level":"info","msg":"resolved version"}   # ← BUG
{"schemaVersion":"1","command":"version","data":{...}}                  # ← would break parsers
```

The first line is a logger line that leaked into stdout. Downstream
parsers that read a single JSON document will either fail or silently
consume the wrong document. This is the exact failure mode this
architecture prevents.

### 4.4 Enforcement

The rule is enforced at three levels:

1. **By construction:** `internal/output.Writer` has no stderr handle;
   `internal/logger.Sink` has no stdout handle. A logger line cannot be
   written to stdout because no code path can do so.
2. **By contract test:** `internal/cli/contract_test.go::TestOutputLoggingSeparation_StdoutOnlyContainsJSON`
   asserts exactly one JSON document on stdout and no output envelope on
   stderr.
3. **By lint (WBS 12.3.2):** `fmt.Println`, `fmt.Printf` to stdout, and
   `os.Stdout.Write` are forbidden outside `internal/output/`.

### 4.5 JSON Envelope Shape

Every `--format json` success emits one envelope conforming to
`schemas/output-envelope.schema.json`:

```json
{
  "schemaVersion": "1",
  "command": "version",
  "data": { }
}
```

- `schemaVersion` — bumped only on breaking changes to the envelope
  shape. Additive changes do not bump it.
- `command` — identifies the producing command for dispatch.
- `data` — command-specific payload; shape defined per command.

---

## 5. Package Boundaries

### 5.1 `internal/output`

**Responsibility:** product surface. Writes to stdout only.

**Public API:**

```go
// Format identifies the rendering format.
type Format int
const (
    FormatText Format = iota
    FormatJSON
)

// Writer renders output to a destination stream.
type Writer interface {
    WriteOutput(ctx context.Context, format Format, v any) error
}

// NewWriter returns a Writer bound to w (os.Stdout in production).
func NewWriter(w io.Writer, command string) *StdoutWriter

// Print writes v to the default writer using format.
func Print(ctx context.Context, format Format, v any) error
```

**Invariants:**

- Never writes to stderr.
- Under `FormatJSON`, writes exactly one JSON document.
- Accepts `context.Context` for future trace propagation.
- Thread-safe.

**See:** `internal/output/doc.go`.

### 5.2 `internal/logger`

**Responsibility:** operational surface. Writes to stderr only.

**Public API:**

```go
// Level is the severity of a log record.
type Level int
const (
    LevelDebug Level = iota
    LevelInfo
    LevelWarn
    LevelError
)

// Sink is the destination for log records.
type Sink interface {
    Emit(ctx context.Context, r Record)
}

// Logger emits log records to its Sink.
type Logger struct { /* ... */ }

// New creates a Logger from cfg.
func New(cfg Config) *Logger

// Package-level convenience:
func Debug(ctx context.Context, msg string, attrs ...any)
func Info(ctx context.Context, msg string, attrs ...any)
func Warn(ctx context.Context, msg string, attrs ...any)
func Error(ctx context.Context, msg string, attrs ...any)
```

**Invariants:**

- Never writes to stdout.
- Respects the configured level.
- Thread-safe.
- Accepts `context.Context` for future trace/request-ID propagation.

**See:** `internal/logger/doc.go`.

### 5.3 `internal/forgeerr`

**Responsibility:** error surface. Writes to stderr only, rendered per
`--format`.

**Invariants:**

- Never writes to stdout.
- Every error has a code from the registry (ADR-012).
- Redaction policy applied before rendering (ADR-016).

**See:** ADR-011 through ADR-021, `docs/reference/error-codes.md`.

### 5.4 Dependency Direction

```text
        ┌────────────────────┐
        │      main          │
        └─────────┬──────────┘
                  │
        ┌─────────▼──────────┐
        │   internal/cli     │
        └──┬──────┬──────┬───┘
           │      │      │
     ┌─────▼─┐ ┌──▼───┐ ┌▼──────────┐
     │output │ │logger│ │ forgeerr  │
     └───────┘ └──────┘ └───────────┘
           ▲      ▲      ▲
           └──────┴──────┘
                  │
        ┌─────────┴──────────┐
        │  domain packages   │
        │ (config, version,  │
        │  blueprint, ...)   │
        └────────────────────┘
```

Domain packages **must not** import `output`, `logger`, or `forgeerr`
directly. They return values and errors; the CLI layer decides how to
render them. This keeps the domain testable without stream concerns.

---

## 6. Decision Procedure for Contributors

When adding a new line of emitted bytes, ask:

> **"Is this the result the user asked for, or a diagnostic about how we
> produced it?"**

### 6.1 Decision Table

| The line is... | Category | Package | Stream |
|----------------|----------|---------|--------|
| The requested result | Output | `internal/output` | stdout |
| A "starting…" / "done" / "loading…" message | Logger | `internal/logger` | stderr |
| A timing measurement | Logger | `internal/logger` | stderr |
| A debug trace | Logger | `internal/logger` | stderr |
| A "cache hit/miss" note | Logger | `internal/logger` | stderr |
| A "retrying…" warning | Logger | `internal/logger` | stderr |
| A failure the user must act on | Error | `internal/forgeerr` | stderr |
| A prompt for interactive input | Output | `internal/output` | stdout |
| Help text | Output | Cobra | stdout |

### 6.2 Tie-Breaker

If unsure, **prefer the logger**. It is always safer to move a line from
output to logger than the reverse:

- Moving output → logger: harmless (diagnostic noise on stderr).
- Moving logger → output: potentially breaking (corrupts product stream).

### 6.3 Examples

**Correct:**

```go
// Command result → output.
if err := output.Print(ctx, format, versionInfo); err != nil {
    return err
}

// Diagnostic → logger.
logger.Debug(ctx, "resolved version from build info", "version", v)

// Failure → forgeerr.
return forgeerr.New(forgeerr.CodeConfigInvalid, "config is invalid",
    forgeerr.WithRemediation("run 'forge config validate'"))
```

**Incorrect:**

```go
// BAD: diagnostic on stdout.
fmt.Println("loading config...")

// BAD: result on stderr.
fmt.Fprintln(os.Stderr, versionInfo)

// BAD: log with no level control.
log.Printf("cache miss")

// BAD: output via stdout write, bypassing format selection.
os.Stdout.Write([]byte(versionInfo))
```

---

## 7. Anti-Patterns

| Anti-pattern | Why it's wrong | Fix |
|--------------|----------------|-----|
| `fmt.Println("result")` in domain code | Bypasses format selection; goes to stdout silently; not golden-testable | `output.Print(ctx, format, result)` |
| `log.Printf("debug")` | Stdlib logger; no level control; no structured fields | `logger.Debug(ctx, "debug")` |
| `fmt.Fprintln(os.Stderr, json)` for output | Output belongs on stdout | `output.Print(ctx, FormatJSON, v)` |
| `output.Print("starting...")` | "starting" is a diagnostic, not a result | `logger.Info(ctx, "starting")` |
| Injecting log lines into stdout under `--format json` | Corrupts product stream; breaks every parser | Logger is stderr-only by construction |
| Using `os.Stdout.Write` directly | Skips format handling and envelope wrapping | Use `internal/output` |
| Logging secrets | Leaks credentials into log aggregators | Redact via `internal/forgeerr/redact` |
| Emitting multiple JSON documents on stdout | Breaks single-document consumers | One envelope per invocation |
| Writing output on error paths | Leaves partial product data for failed commands | On non-zero exit, stdout is empty |

---

## 8. Testing Strategy

### 8.1 Layers of Tests

| Layer | File | What it asserts |
|-------|------|-----------------|
| Unit | `internal/output/writer_test.go` | Writer writes to its sink; JSON is one document; context cancellation; unknown format error |
| Unit | `internal/logger/logger_test.go` | Level filtering; structured attrs; JSON one-record-per-line; concurrency safety |
| Contract | `internal/cli/contract_test.go` | Stream isolation; `--format json` purity; logger never on stdout |
| Golden | `internal/cli/testdata/help/*.golden.txt` | Help text stability |
| Golden | `internal/app/version/testdata/format/*.golden` | Version output per format |
| Integration | `cmd/forge/binary_integration_test.go` | End-to-end stream capture |

### 8.2 Contract Test (Normative)

`internal/cli/contract_test.go::TestOutputLoggingSeparation_StdoutOnlyContainsJSON`
is the normative test for this document. It:

1. Runs a command with `--format json` and `--log-level debug`.
2. Asserts stdout decodes as exactly one JSON envelope.
3. Asserts `dec.More() == false` (no trailing documents).
4. Asserts stderr contains the expected log messages.
5. Asserts stderr does **not** contain the output envelope.

Any change to the architecture that breaks this test is a regression and
requires an ADR update.

### 8.3 Test Taxonomy

| Case type | Example |
|-----------|---------|
| **Positive** | JSON format emits one envelope on stdout |
| **Negative** | Unknown format returns an error and writes nothing |
| **Edge** | Empty string output produces a newline in text format |
| **Corner** | Non-string value in text format uses `fmt.Sprint` |
| **Boundary** | User data containing `<tag>` is not HTML-escaped |
| **Non-functional** | Concurrent `Info` calls emit exactly N records |

### 8.4 What Is Not Tested Here

- Format-specific golden files (owned by Phase 6 and Phase 10).
- Error envelope shape (owned by Phase 2, ADR-021).
- Log message content (owned by the emitting package).

---

## 9. Migration Notes

### 9.1 Before WBS 12.1.1

- Output was written ad-hoc via `fmt.Println` and `internal/cli/output.go`.
- Logging was via the stdlib `log` package, sometimes to stdout.
- No enforced boundary; `--format json` purity was aspirational.

### 9.2 After WBS 12.1.1

- Output lives in `internal/output`; logger lives in `internal/logger`.
- Stream separation is enforced by construction and by contract test.
- `internal/cli/output.go` becomes a thin consumer of `internal/output`.
- `internal/cli/stream_boundary.go` is superseded by this architecture.

### 9.3 Deprecations

| Symbol | Status | Replacement |
|--------|--------|-------------|
| `internal/cli.Output` | Deprecated | `internal/output.Writer` |
| `internal/cli.PrintJSON` | Deprecated | `output.Print(ctx, output.FormatJSON, v)` |
| `internal/forgeerr.Log` | Deprecated | `internal/logger` |

### 9.4 What Must Be Done in WBS 12.3.2

- Add lint rule forbidding `fmt.Println` and `os.Stdout.Write` outside
  `internal/output`.
- Migrate all domain-package output calls to the new packages.
- Delete deprecated symbols.

---

## 10. Frequently Asked Questions

**Q: Why not just use the stdlib `log` package?**
A: It writes to stderr by default but offers no levels, no structured
fields, no format selection, and no injection seam for tests. It also
makes it easy to misconfigure `log.SetOutput(os.Stdout)`, silently
violating the contract.

**Q: Why not use `log/slog` directly?**
A: We do — `internal/logger/slog_adapter.go` bridges `slog` into the
forge logger. But the package boundary matters: domain code must not
reach for `slog` directly, or the sink could be misconfigured.

**Q: Why is the logger format flag separate from `--format`?**
A: Because they serve different audiences. A user may want JSON output
for parsing but human-readable logs for debugging, or vice versa. Coupling
them would remove that choice.

**Q: What about progress bars and spinners?**
A: They are diagnostics (operational), so they belong on stderr. A
progress bar on stdout would corrupt JSON output. Future work may add a
"quiet" mode that suppresses them.

**Q: What about interactive prompts?**
A: Prompts are output — they are the result of the user having invoked a
command in interactive mode. They go to stdout. The *response* is input
from stdin. This keeps `forge <cmd> | jq` working when the command is
non-interactive.

**Q: Where do timing lines go?**
A: Logger, at `debug` level. Never output. Timing is diagnostic.

**Q: Can a command write to stdout without going through `internal/output`?**
A: Not after WBS 12.3.2. The lint rule forbids it.

---

## 11. References

### 11.1 Internal

- [ADR-022: Output/Logging Separation](../decisions/ADR-022-output-logging-separation.md)
- [ADR-005: Version Command Output Formats](../decisions/ADR-005-version-command-structure-output-formats-and-stability-guarantees.md)
- [ADR-006: Help and CLI UX Contracts](../decisions/ADR-006-help-and-cli-ux-contracts-invocation-matrix-output-formats-stream-boundary-and-phase-2-presentation-policy.md)
- [ADR-011: Closed Error Category Taxonomy](../decisions/ADR-011-closed-error-category-taxonomy.md)
- [ADR-012: Error Code Registry and Stability](../decisions/ADR-012-error-code-registry-and-stability-policy.md)
- [ADR-018: Exit Code Migration and Stability](../decisions/ADR-018-exit-code-migration-and-stability-policy.md)
- [ADR-020: stdout/stderr Contract per Exit Code](../decisions/ADR-020-stdout-stderr-contract-per-exit-code.md)
- [ADR-021: JSON Error Envelope](../decisions/ADR-021-json-error-envelope-for-format-json.md)
- [Reference: Output Streams](../reference/output-streams.md)
- [Reference: Exit Codes](../reference/exit-codes.yaml)
- [Reference: Error Codes](../reference/error-codes.md)
- [CLI UX Spec](../cli-ux-spec.md)
- [Component Spec](../component-spec.md)

### 11.2 Schemas

- `schemas/output-envelope.schema.json`
- `schemas/error-envelope.schema.json`
- `schemas/version.schema.json`
- `schemas/exit-codes.schema.json`

### 11.3 External

- POSIX.1-2017, §2.5 — Standard I/O Streams
- The Twelve-Factor App, §XI — Logs
- `kubectl`, `gh`, `docker` — precedent for stdout=result, stderr=diagnostics
- Go `log/slog` — structured logging in the standard library

---

## 12. Change Log

| Date | Change | Author |
|------|--------|--------|
| 2026-10-12 | Initial version (WBS 12.1.1) | Architecture Owner |

---

## 13. Approval

| Role | Name | Date |
|------|------|------|
| Architecture Owner | TBD | |
| Phase 6 Owner (CLI Output) | TBD | |
| Phase 10 Owner (JSON Contract) | TBD | |
| Phase 2 Owner (Error System) | TBD | |
