# ADR-025: Log records are structured with a schema-versioned JSON form and an unstable human form

**Status:** Accepted
**Date:** 2026-10-13
**Deciders:** [@thapelomagqazana] (Architecture Owner)
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

ADR-022 separated the product surface (stdout) from the operational
surface (stderr) and named `internal/logger` as the package that owns
the operational surface. ADR-023 fixed the four log levels and their
semantics. ADR-024 fixed the precedence chain that selects a level.
None of these decisions defined the *shape* of a log record — what
fields it carries, how those fields are named, or how a record is
serialized. The current `internal/logger` package has a `Record`
struct and two writers (`writeHuman` in `format_human.go` and
`writeJSON` in `format_json.go`), but the record's contract, the two
renderings' guarantees, and the value set of a record's enumerable
fields (level, component) are not documented as a public interface.
The package works today; a future contributor who wants to add a
field, rename one, or change the JSON encoding has no rule to
follow.

Two audiences consume log records, and they have different
expectations. A human reading a terminal wants a compact line whose
fields are visible without scrolling: the level, the component, the
message, and any structured attributes on one line. A machine
consumer — a log aggregator, a CI system's log parser, a wrapper
script that greps for a specific event — wants a structured record
whose fields are addressable by name and whose encoding is stable
enough to write a parser against. The two audiences' needs are not
satisfiable by a single rendering: a human-friendly line is not
machine-parseable (its field order may vary, its message may contain
spaces), and a machine-friendly JSON object is verbose on a terminal.
The current package produces both, but does not promise anything
about either.

The forces at play are three. First, a record's *shape* is a longer-
lived contract than its *rendering*. A change to the human
rendering — say, reordering the fields for readability — is a
cosmetic change that a user notices but does not depend on. A change
to the JSON rendering — say, renaming `ts` to `timestamp` — is a
breaking change for every consumer that parses the JSON. The two
renderings need different stability promises, and the promises must
be written down. Second, `--format json` (the product-output flag
from WBS 6.4.1) and `FORGE_LOG_FORMAT` (the logger-format env var)
are independent. A user may request JSON product output and human
logs, or the reverse. Conflating the two would couple the two
surfaces' formats, which the stream separation (ADR-022) exists to
prevent. Third, the `correlation_id` field — a per-invocation
identifier that lets a consumer correlate all records from a single
`forge` invocation — is planned but not specified; without it, a
consumer of a log file cannot reconstruct which records belong to
which invocation.

The decision relates to ADR-022 (the stream the records go to),
ADR-023 (the level vocabulary a record's `level` field carries),
ADR-024 (the resolution that determines which records are emitted),
ADR-026 (the interface that produces records), and the forthcoming
ADR-033 and ADR-034 (redaction). The CLI UX spec's § 4.12 mentions
log format selection but does not define the record. If no decision
is made, the record's shape remains an implementation detail, the
JSON rendering's stability is a matter of chance, and every future
subsystem that emits a record re-derives the shape for itself.

---

## 2. Options Considered

### Option A — One rendering: a stable JSON record

Every record is emitted as a single JSON object on one line. The
human rendering is not provided; a user who wants to read the log
uses a JSON-aware viewer (`jq`, `tail -f | jq`), or the CLI provides
a separate `forge logs` command that reads the JSON and renders it
for humans. `FORGE_LOG_FORMAT` does not exist; the only format is
JSON.

**Pros:**

- One rendering to document, test, and version. The record's shape
  is the only contract; there is no second format whose stability
  must be reasoned about separately.
- The JSON form is machine-parseable by construction; a consumer
  never has to disambiguate fields from a free-form message.
- The `jq` ecosystem is well-understood and widely available; a
  user who wants to read the log can learn one tool and use it
  for every log-producing system.
- The record's shape can evolve without a second format's
  coordination; there is no risk of the two renderings drifting.

**Cons:**

- A human reading a terminal receives a JSON line that is longer
  than a human-formatted line and whose field order is determined
  by the encoder rather than by readability. The `tail -f` view
  is harder to scan.
- A user who is not familiar with `jq` (or does not have it
  installed) cannot read the log without additional tooling. This
  is a real barrier for the CLI's target users, who include
  Windows users without a POSIX shell.
- The `FORGE_LOG_FORMAT` env var's value set would be empty
  (only `json`), which is a strange configuration surface; a
  future addition of a human format would be a breaking change
  to the value set.
- The color and formatting that a terminal supports (bold level
  names, dimmed timestamps) are unavailable; the user loses the
  affordances that make terminal output scannable.
- The CLI's other surfaces (product output) provide a human
  format by default; a logger that does not is inconsistent with
  the rest of the CLI's UX.

### Option B — One rendering: an unstable human record

Every record is emitted as a human-readable line. There is no JSON
form; `FORGE_LOG_FORMAT` does not exist. A consumer that wants to
parse the log uses a pattern (regular expression, field split) that
the CLI does not promise to keep stable.

**Pros:**

- One rendering to document. The record's shape is defined only by
  the human formatter's output.
- The `tail -f` view is optimized for human reading; a user
  scanning the log sees the level, the component, the message, and
  the attributes without JSON ceremony.
- No JSON schema to maintain; no versioning decision for the
  JSON.

**Cons:**

- A machine consumer has no reliable parse. The human rendering's
  field order, spacing, and message encoding are not guaranteed;
  a future change to any of them breaks every parser that was
  written against the old form. The CLI would not be able to
  promise anything to its machine consumers.
- Log aggregation systems (Datadog, Loki, CloudWatch Logs) that
  ingest JSON by default would need per-CLI parsing rules; the
  unstructured line would require a custom grok pattern for every
  field.
- The record's fields are not addressable by name in the output;
  a consumer that wants to extract one field must parse the
  message, which is not machine-formatted.
- A future move to structured output would be a breaking change
  with no migration path; every parser written against the human
  form would break simultaneously.
- The `correlation_id` field has no place to live in a human
  rendering without a naming convention that the human formatter
  would have to invent, and the convention would be unenforced
  by any schema.

### Option C — Two renderings, both stable

Every record is emitted in one of two renderings, selected by
`FORGE_LOG_FORMAT`. Both renderings are stable: their field names,
field order, and encoding are frozen within a major version of the
CLI. A change to either rendering is a major-version change.

**Pros:**

- Both audiences are served. A human reads the human form; a
  machine parses the JSON.
- Both formats are stable, so a consumer can rely on either.
- The `FORGE_LOG_FORMAT` env var's value set is `{human, json}`,
  matching `--format`'s `{text, json}` in shape.

**Cons:**

- Two formats to version and to keep stable. A change to the
  record's shape must be applied to both renderings, and the
  change is a breaking change for consumers of either. The
  coupling of the two renderings' stability is a maintenance
  burden and a source of rigidity.
- The human rendering's stability is a promise the CLI does not
  want to keep. Human renderings are optimized for readability,
  which changes as the CLI's UX evolves: a field that is
  rendered inline today may be rendered on a second line
  tomorrow to fit a longer component name. Freezing the human
  form would prevent these improvements.
- The promise "the human form is stable" is unverifiable in
  practice: a user who greps the human form is not a "consumer"
  the CLI can identify, and their reliance on the format is
  invisible until a change breaks their grep. The JSON form, by
  contrast, is a documented interface with a schema, so a
  change to it is a recognizable contract break.
- The two renderings' stability is asymmetric in the CLI's
  ecosystem: log aggregation systems care about the JSON, not
  the human form. Freezing the human form satisfies no external
  consumer.

### Option D — Two renderings, with asymmetric stability

Every record is emitted in one of two renderings, selected by
`FORGE_LOG_FORMAT` (default `human`). The two renderings have
different stability promises:

- **JSON** — stable within a major version. Field names, field
  types, and the encoding's shape are frozen. Additive changes
  (new fields) are non-breaking; removals and renames are major-
  version changes. A JSON Schema (`schemas/log-record.schema.json`)
  publishes the shape.
- **Human** — **not stable**. Field order, spacing, formatting,
  color, and the message encoding may change between any two
  releases. A consumer must not parse it; a human may read it.

The `Record` struct is the source of truth for the fields the JSON
rendering emits; the human rendering may render any subset in any
order. A field that appears in the JSON rendering is guaranteed to
be available to a JSON consumer; the same field may be omitted or
abbreviated in the human rendering.

**Pros:**

- The two audiences' needs are met by two formats, each with the
  stability promise its audience actually needs. The JSON form
  is a documented interface; the human form is a UX surface
  that can evolve freely.
- The `FORGE_LOG_FORMAT` env var's value set is `{human, json}`,
  matching `--format`'s `{text, json}` in shape.
- A future improvement to the human rendering (reordering fields,
  adding color under a future `--color` flag, condensing a
  multi-line attribute) is not a contract break. The human form's
  evolution is decoupled from the JSON form's stability.
- The JSON Schema is the authoritative reference for the JSON
  form; a consumer can validate a line against it and can be
  sure of what the CLI promises.
- The asymmetric promise is *honest*: it tells the truth about
  what the CLI can and cannot guarantee. A user who greps the
  human form is not misled into thinking their grep is a
  supported contract.
- The `correlation_id` field has a defined home (the JSON
  rendering) and is available to machine consumers; the human
  rendering may include it or not, at the CLI's discretion.

**Cons:**

- A consumer who wanted to parse the human form has no support.
  The CLI's documentation says so explicitly, but a user who
  ignores the documentation and writes a parser against the
  human form will be broken by a future change. The
  CLI cannot prevent this, but the documentation is the honest
  position.
- Two renderings still must be maintained. The human rendering
  may change freely, but a change still requires updating its
  formatter, its golden file (if the tests use one), and its
  documentation.
- The JSON form's stability means a future removal of a field
  is a major-version change. The additive-only rule is a
  constraint on the CLI's evolution, though it is the same
  constraint that governs the error envelope (ADR-021) and the
  exit codes (ADR-018).
- The `Record` struct's fields must be chosen carefully, since
  adding a field to the JSON form is a non-breaking change but
  removing one is not; the CLI's maintainers must think before
  they add.

---

## 3. Decision

**Forge adopts Option D: two log record renderings, selected by
`FORGE_LOG_FORMAT` (default `human`), with asymmetric stability
promises.** The JSON rendering is stable within a major version:
field names, field types, and the encoding's shape are frozen;
additive changes (new fields) are non-breaking; removals and renames
are major-version changes. A JSON Schema
(`schemas/log-record.schema.json`) publishes the shape. The human
rendering is not stable: field order, spacing, color, and the
message encoding may change between any two releases; a consumer
must not parse it. Both renderings emit exactly one line per record,
terminated by `\n`; both write to stderr (per ADR-022); neither
writes to stdout.

The `Record` struct is the source of truth for the fields the JSON
rendering emits. Its fields are:

| Field | Type | Purpose |
|-------|------|---------|
| `ts` | string (RFC 3339, UTC) | The timestamp of the record. |
| `level` | string (one of `debug`, `info`, `warn`, `error`) | The record's level (per ADR-023). |
| `component` | string (one of the enumerated components) | The subsystem that emitted the record. |
| `msg` | string | A human-readable summary. |
| `correlation_id` | string (UUID) or the sentinel `"00000000-0000-0000-0000-000000000000"` | The per-invocation identifier; shared by all records from a single `forge` invocation. |
| `attrs` | object (string keys, JSON values) or omitted | Free-form structured data; omitted when empty. |

The JSON key names are short (`ts`, `msg`) rather than long
(`timestamp`, `message`) because the JSON rendering is optimized for
machine consumption, and short keys reduce log volume; the meaning
is documented in the schema. The `correlation_id` field is present
in every record; records emitted outside an invocation carry the
zero-UUID sentinel (a well-defined value, distinct from an empty
string). The `attrs` field is omitted when empty, so a record
without structured data is a shorter line.

The reasoning that tipped the balance was the asymmetry of the two
audiences' actual reliance on the formats. A machine consumer's
parser is a durable commitment: it is written once, deployed, and
runs for years, and a change to the JSON breaks it silently and
at scale. A human's reading of the log is an act of the moment:
the human reads the line, extracts the information they need, and
moves on; they do not write a parser, and if the format changes
between releases, the human adapts in seconds. Freezing the human
form would satisfy a consumer that does not exist (no one relies on
the human form durably) while preventing improvements that the
human audience benefits from. The asymmetry is the correct shape:
stability follows reliance, and the two audiences' reliance differs.

Option A (JSON-only) and Option B (human-only) were rejected because
each serves one audience at the expense of the other. Option A's
JSON-only choice would be defensible in a server-side context where
every operator has `jq`, but Forge's target users include Windows
users without a POSIX shell and users who do not know `jq`; the
CLI's other surfaces (product output) provide a human format by
default, and the logger's divergence would be inconsistent.
Option B's human-only choice would leave the CLI's machine
consumers — log aggregators, CI systems, wrapper scripts — without
a contract, which is incompatible with the CLI's promise of being
scriptable. Option C (both stable) was rejected because the human
form's stability is a promise the CLI does not want to keep and
cannot verify; the promise would constrain the CLI's UX evolution
for the benefit of a consumer the CLI cannot identify.

The tradeoffs accepted are the ones Option D lists as cons. The
JSON form's additive-only rule is a constraint on the CLI's
evolution; it is the same constraint that governs the error
envelope and the exit codes, and it is the correct constraint for a
public interface. The `Record` struct's field set must be chosen
carefully; adding a field is cheap (non-breaking), removing one is
expensive (major-version change), and the CLI's maintainers must
weigh the cost before they add. The human form's instability is a
documented limitation; a user who parses it is warned in
`docs/reference/log-record-format.md` § "Human format is not
stable" that their parser is unsupported.

The decision is provisional in one respect: the specific field
names (`ts`, `msg`, `correlation_id`) are frozen by this ADR, but
the `attrs` object's permitted value types and the `component`
enum's full value set are defined by the accompanying reference doc
and by `schemas/log-record.schema.json`, which are expected to
evolve as the CLI's subsystems grow. A future ADR that changes a
field name or removes a field is a major-version change and must
cite this ADR as superseded; a future ADR that adds a field is
non-breaking and may be recorded as a minor extension.

---

## 4. Consequences

### 4.1 Positive

- Two audiences are served by two formats, each with the stability
  promise it needs. A machine consumer can rely on the JSON form's
  schema; a human reader benefits from a human form that can be
  improved without a contract break.
- The `FORGE_LOG_FORMAT` env var's value set (`human`, `json`) is
  symmetric with `--format`'s (`text`, `json`), which makes the
  two flags' relationship easy to teach.
- A future improvement to the human rendering (reordering fields,
  adding color under a future `--color` flag, condensing a
  multi-line attribute) is not a contract break. The human form's
  evolution is decoupled from the JSON form's stability.
- The JSON Schema is the authoritative reference for the JSON
  form; a consumer can validate a line against it and can be sure
  of what the CLI promises. The schema is published at
  `schemas/log-record.schema.json` and tested by the logger's
  test suite.
- The `correlation_id` field has a defined home; a machine
  consumer can correlate all records from a single invocation,
  which is the foundation for end-to-end tracing of a CLI
  invocation.
- The record's field set is small and stable; the `Record` struct
  is the single source of truth for both renderings.
- The human form's explicit instability is *honest*. A user who
  greps the human form is warned that their parser is not a
  supported contract; the CLI does not mislead them.

### 4.2 Negative

- A user who parses the human form has no support; the CLI cannot
  prevent them from doing so, but the documentation is the
  honest position. A future change to the human form will break
  their parser, and the CLI will not treat the break as a defect.
- Two renderings still must be maintained. The human rendering
  may change freely, but a change still requires updating its
  formatter, its golden file (if the tests use one), and its
  documentation. The JSON rendering's changes require updating
  the schema, the reference doc, and the test suite.
- The JSON form's stability means a future removal of a field
  is a major-version change. The additive-only rule constrains
  the CLI's evolution; a field that a future release discovers
  is unused cannot be removed without a major version bump.
- The `Record` struct's field set must be chosen carefully. Each
  field added to the JSON form is a non-breaking change, but
  each field removed is a breaking change; the CLI's maintainers
  must weigh the cost before they add.
- The short JSON key names (`ts`, `msg`) are less self-documenting
  than long names (`timestamp`, `message`). A consumer that reads
  the schema or the reference doc learns them quickly; a
  consumer that reads only a sample log line must look them up.

### 4.3 Neutral

- The JSON rendering's key order is the struct's declaration
  order (`encoding/json` emits fields in declaration order). A
  consumer must not depend on the order; JSON objects are
  unordered by spec. The choice of declaration order is a
  readability decision, not a contract.
- The human rendering's exact format is documented as an
  *example* in `docs/reference/log-record-format.md`, not as a
  specification. A future change to the example is not a
  contract break.
- The `attrs` object's permitted value types are JSON's: string,
  number, boolean, null, array, object. The CLI does not
  constrain them beyond JSON's rules; a consumer must handle
  every JSON type.
- The `correlation_id` field's UUID format is documented but not
  enforced by the schema (the schema accepts any string); a
  future change to a different identifier format would be
  non-breaking for a consumer that treats the value as opaque.

---

## 5. Related Requirements

- `FR-CLI-LOG-004` — log record structure with all fields; the
  requirement is established by this decision.
- `FR-CLI-LOG-005` — the two formats (human, JSON) and their
  selection by `FORGE_LOG_FORMAT`; the requirement is established
  by this decision.
- `FR-CLI-LOG-006` — the JSON form is schema-versioned and stable
  within a major version; the requirement is established by this
  decision.
- `NFR-OBSERVABILITY-001` — records are structured and
  attributable; the requirement is constrained by this decision's
  field set and by the `correlation_id`'s presence in every
  record.
- `NFR-ARCHITECTURE-003` — the logger's interface is independent
  of CLI and domain packages; the requirement is compatible with
  this decision's record shape, which has no CLI-specific fields.

---

## 6. Related ADRs

- `ADR-022` — Output and logging are separate architectural
  layers. Establishes the stream the records go to; this
  decision's records are the content of that stream.
- `ADR-023` — Four log levels: ERROR, WARN, INFO, DEBUG.
  Establishes the level vocabulary this record's `level` field
  carries.
- `ADR-024` — Log level precedence: CLI flag > env var > config
  file > default. Establishes the resolution that determines
  which records are emitted; this decision's record shape is
  independent of the resolution.
- `ADR-026` (expected) — The Logger interface has four level
  methods and a With combinator. Will define the interface that
  produces records; depends on this decision's record shape.
- `ADR-021` — JSON Error Envelope for `--format json`.
  Establishes a precedent for schema-versioned JSON on stdout;
  this decision's JSON rendering is the stderr counterpart
  (different stream, different schema, same discipline).
- `ADR-033` (expected) — Sensitive data is redacted at three
  layers. Will define the redaction rules that apply to this
  record's `msg` and `attrs` fields.
- `ADR-034` (expected) — The redaction policy is closed. Will
  define the closure property that applies to every field this
  record declares.

---

## 7. Notes

**Open questions deferred by this ADR:**

- Whether the human rendering should be the default or whether
  the JSON rendering should be. The ADR chooses `human` as the
  default, on the grounds that a first-time user running
  `forge --verbose version` in a terminal wants readable output.
  A future ADR could change the default if a use case emerges
  for JSON-by-default.
- Whether the `correlation_id`'s zero-UUID sentinel should be
  omitted from the JSON rendering (like an empty `attrs`) or
  always present. The ADR chooses always present, on the grounds
  that a consumer that filters by correlation ID benefits from
  a uniform schema; a future ADR could change this.
- Whether the JSON rendering should include a `schema_version`
  field. The ADR does not include one; the schema is versioned
  by the CLI's major version, not by an inline field. A future
  ADR could add the field if the CLI's major version and the
  schema's version diverge.

**Follow-up decisions anticipated:**

- The redaction rules (ADR-033, ADR-034) apply to the `msg` and
  `attrs` fields; their design is separate from this ADR's
  record shape but must respect it.
- The component enum's full value set is defined by the reference
  doc and is expected to grow as the CLI's subsystems grow. A
  new component is a non-breaking change (the schema accepts any
  string in the `component` field).
- A future `forge logs` command that reads the CLI's own JSON
  log output and renders it for humans is a plausible feature.
  Its design would build on this ADR's JSON rendering; the
  command itself is not this ADR's subject.

**External references:**

- Go standard library `log/slog` — the record's conceptual
  shape (time, level, message, attributes) is inspired by
  `slog.Record`; the JSON rendering is Forge's own encoding.
- OpenTelemetry's log data model — the `correlation_id` field's
  role (correlating records from a single trace) is analogous
  to the trace ID in OTel's model; Forge's identifier is
  per-invocation rather than per-trace.
- JSON Schema (Draft 2020-12) — the schema language for
  `schemas/log-record.schema.json`.
- `docs/reference/log-record-format.md` — the authoritative
  reference for this ADR's record shape, the two renderings'
  examples, and the stability promises.

---

*End of ADR-025.*
