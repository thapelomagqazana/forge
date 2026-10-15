# Log Record Format

<!--
  Document:     log-record-format.md
  Purpose:      Normative schema for every record Forge emits to
                stderr. Defines the record's fields, the two
                renderings (human, json), the format selector, and
                the stability promise.
  Audience:     Operators, CI/CD integrators, log aggregators,
                monitoring authors, Forge contributors.
  Stability:    Field names are stable within a major version. The
                human rendering is NOT stable; consumers must parse
                JSON. The JSON envelope is schema-versioned.
  Traceability: FR-CLI-LOG-004, FR-CLI-LOG-005, NFR-OBSERVABILITY-001,
                ADR-022, WBS 12.2.1
  See also:     log-levels.md, output-streams.md,
                ../architecture/output-vs-logging.md
  Last Review:  2026-10-13
-->

## Overview

Every Forge log record — every diagnostic line written to stderr — has
the same shape. This document defines that shape, the two renderings
that serialize it, and the stability promise attached to each.

The record is the unit of the logger's contract with consumers. It is
distinct from the product output on stdout (governed by
[`output-streams.md`](output-streams.md)) and from the JSON error
envelope (ADR-021). A record is a **diagnostic**; product output and
errors are **results**.

## The record

| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `ts` | string (RFC3339, UTC) | yes | When the record was created. |
| `level` | enum | yes | One of `debug`, `info`, `warn`, `error`. |
| `component` | enum | yes | Subsystem tag. See §"Component tags". |
| `msg` | string | yes | Human-readable summary. |
| `correlation_id` | string (UUID v4) | yes | Per-invocation identifier; the same value on every record of one command run. |
| `attrs` | object | no | Structured key-value data. Omitted when empty. |

The canonical Go type is `logger.Record` (see
`internal/logger/record.go`).

### Timestamp (`ts`)

RFC3339 with nanosecond precision, in UTC. Format:

```text
2006-01-02T15:04:05.000000000Z
```

UTC is required. A local-time timestamp would make records
incomparable across hosts; a non-padded format would break lexical
sorting.

### Level (`level`)

The four levels defined by
[`log-levels.md`](log-levels.md). The value is the lowercase name.

### Component (`component`)

A **closed enumeration**. Adding a component requires an ADR. The
current list:

| Component | Purpose |
|-----------|---------|
| `cli` | The command dispatcher, flag parsing, argument validation. |
| `config` | Configuration discovery, loading, validation, precedence. |
| `blueprint` | Blueprint reading, parsing, validation. |
| `template` | Template engine invocation. |
| `renderer` | Output rendering to the filesystem. |
| `filesystem` | Filesystem operations (read, write, walk, stat). |
| `policy` | Policy evaluation, findings computation. |

The enumeration is intentionally small. A component tag identifies
*the subsystem that emitted the record*, not the specific function.
A record about filesystem errors is `filesystem`, whether it comes
from a load, a stat, or a walk. A record about blueprint *parsing* is
`blueprint`, not `blueprint/parser`.

### Message (`msg`)

A short, human-readable summary. It should be a complete phrase that
stands on its own ("loaded configuration", "skipped hidden file").
Message text is **not** stable; consumers must use `attrs` for
machine-readable values and `component`+`level` for classification.

Message strings do **not** end in punctuation and are **not**
capitalized, matching the conventions of `log/slog`, `zap`, and
`logrus`.

### Correlation ID (`correlation_id`)

A UUID v4 generated once per command invocation and attached to
every record the invocation emits. Two invocations of the same
command have different IDs; a single invocation's records all share
one.

The ID is generated in `PersistentPreRunE` (see
`internal/cli/hooks.go`) and stored on the command's context. It is
present on every record, including records emitted before the
subcommand's `RunE` runs and records emitted after it returns.

The ID is a **tracing aid**, not a security boundary. It is a random
v4 UUID; a consumer who wants to correlate records across processes
should extract the ID and use it as a key in their own store.

### Attrs (`attrs`)

Structured key-value data. Keys are strings; values are any
JSON-serializable value. Conventions:

- Keys are `snake_case`: `file_path`, `line_number`, `retry_count`.
- Reserved keys: the record's top-level field names are not valid
  attr keys. `logger.WithAttr` panics if a caller tries to set
  `ts`, `level`, `component`, `msg`, or `correlation_id` as an attr
  key. This prevents a silent collision between the structured
  envelope and a user field.
- Values should be JSON-serializable primitives or plain objects.
  Channels, functions, and pointers to non-serializable types are
  not permitted.
- Values are not redacted by the logger. The caller is responsible
  for not logging secrets; the error layer's redaction
  (`internal/forgeerr/redact.go`, ADR-016) is not applied to log
  records, because a record is not an error.

## Human format

The default rendering. Line-oriented, human-readable, and **not
stable** — consumers must not parse it.

### Grammar

```text
LEVEL component message key=value key=value ...
```

- `LEVEL` is the uppercase level name, padded to a fixed width
  (`DEBUG`, `INFO`, `WARN`, `ERROR`).
- `component` is the component tag in brackets: `[cli]`.
- `message` is the record's `msg`, verbatim.
- `key=value` pairs are the record's `attrs`, sorted by key, with
  values formatted by a stable rule (see below).

### Example

```text
INFO  [cli] loaded configuration path=/etc/forge/config.yaml source=discovery
WARN  [config] deprecated field name=language replacement=runtime
ERROR [filesystem] could not write file path=/tmp/out.go err="permission denied"
```

### Value formatting rules

- `string`: printed verbatim if it contains no whitespace or `"`;
  otherwise double-quoted with Go string-escaping.
- `bool`: `true` or `false`.
- `int`, `int64`, `uint64`, `float64`: `%d` or `%g`.
- `time.Time`: RFC3339.
- `error`: the error's `Error()` string, quoted if it contains
  whitespace.
- `nil`: the literal `null`.
- Everything else: `fmt.Sprintf("%v", v)`, quoted if it contains
  whitespace.

### Stability

The human format is **not stable**. Its grammar, spacing,
punctuation, and value formatting may change in any release. A
consumer that parses stderr records in human format is unsupported
and will break.

### Terminal detection

Forge does **not** detect whether stderr is a TTY. The human format
is emitted to stderr regardless of where stderr is connected. Color
is not emitted; the human format is plain ASCII (see
`ascii_policy.go` for the policy).

## JSON format

The machine-readable rendering. Selected by `FORGE_LOG_FORMAT=json`.
One JSON object per line (newline-delimited JSON, aka NDJSON).

### Example

```json
{"ts":"2026-10-13T10:00:00.123456789Z","level":"info","component":"cli","msg":"loaded configuration","correlation_id":"0192e2b4-1c3a-7f6e-9a2b-4d5e6f708192","attrs":{"path":"/etc/forge/config.yaml","source":"discovery"}}
```

### Field order

The fields are emitted in a fixed order: `ts`, `level`, `component`,
`msg`, `correlation_id`, `attrs`. JSON object key order is not
semantically significant, but the fixed order makes the raw output
easier to read and diff.

### Schema

The schema is published at
[`schemas/log-record.schema.json`](../../schemas/log-record.schema.json).
It conforms to JSON Schema 2020-12 and validates every field,
including the closed enumeration for `level` and `component`.

### Stability

Field names are **stable within a major version**. A new release may
add fields; it will not rename or remove an existing field without a
major version change and an ADR.

The `attrs` object is free-form. Its keys are determined by the
emitter; the schema constrains only that keys are strings and values
are JSON-serializable.

### Schema version

The schema is versioned via the `$id` URL and the `x-forge-schema-version`
annotation. The current version is `1`. A breaking change to the
record's shape increments the version and requires an ADR.

## Format selection

The format is selected by the `FORGE_LOG_FORMAT` environment
variable:

| Value | Rendering |
|-------|-----------|
| `human` (default) | Human format. |
| `json` | JSON format. |
| (unset, empty) | Human format. |
| Anything else | **Usage error.** The process exits 64 with a message naming the accepted values. |

The format is **independent of `--format`** (which controls product
output on stdout). A user may request JSON product output with
human-readable logs, or human product output with JSON logs. The two
selections are orthogonal.

### Interaction with `--format json`

Under `--format json`, product output goes to stdout as a single
JSON document, and log records go to stderr in the format selected
by `FORGE_LOG_FORMAT`. The two streams do not mix. See
[`../architecture/output-vs-logging.md`](../architecture/output-vs-logging.md) §4.

## Stability promise

| Aspect | Promise |
|--------|---------|
| Record field names (`ts`, `level`, `component`, `msg`, `correlation_id`, `attrs`) | Stable within a major version. |
| `level` enumeration | Stable within a major version. |
| `component` enumeration | Stable within a major version; adding a component is additive. |
| Human format grammar | **Not stable.** May change in any release. |
| Message text | **Not stable.** May change in any release. |
| `attrs` keys | Determined by emitters; stable per-emitter within a major version. |
| JSON schema | Additive within version 1; breaking changes increment the version. |

A consumer that reads JSON records and checks field names and
enumerations is supported. A consumer that parses the human format,
or that depends on message text, is not supported.

## Interaction with log levels

Level filtering (`--log-level`, `--verbose`, `--quiet`,
`FORGE_LOG_LEVEL`) applies to both formats. A record at a filtered
level is not emitted in either format. See
[`log-levels.md`](log-levels.md).

## Interaction with correlation IDs

Every record emitted during an invocation carries the same
`correlation_id`. Records emitted outside an invocation (for example,
during package initialization) carry the zero UUID
(`00000000-0000-0000-0000-000000000000`), which is a placeholder
meaning "not part of an invocation". Consumers may use the zero UUID
as a sentinel.

## Testing

The following are tested by the suite:

- **TDD:** the `Record` type's zero value; the `Component` enum's
  closed list; the correlation ID generator's UUID v4 shape.
- **ATDD:** a command invocation emits records with a single
  `correlation_id`.
- **BDD:** given `FORGE_LOG_FORMAT=json`, when a command runs, then
  stderr carries NDJSON records that validate against the schema.
- **Negative:** an invalid `FORGE_LOG_FORMAT` is a usage error.
- **Edge:** an empty `FORGE_LOG_FORMAT` selects the default.
- **Corner:** a record with no attrs omits the `attrs` field in JSON.
- **Boundary:** a record with an attr whose value contains a
  newline is correctly escaped in both formats.
- **Non-functional:** rendering is allocation-bounded; the schema
  validates 100% of sample output.

## References

- [`log-levels.md`](log-levels.md) — the four levels and precedence.
- [`output-streams.md`](output-streams.md) — the stdout/stderr
  contract.
- [`../architecture/output-vs-logging.md`](../architecture/output-vs-logging.md)
  — the two-layer architecture.
- `schemas/log-record.schema.json` — the JSON schema.
- `internal/logger/record.go` — the Go type.
- WBS 12.2.1, FR-CLI-LOG-004, FR-CLI-LOG-005, NFR-OBSERVABILITY-001.
