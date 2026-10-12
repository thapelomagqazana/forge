# ADR-015: Programmer-facing error format

**Status:** Accepted
**Date:** 2026-10-12
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Programmers and log consumers see errors through logs. The user-facing format (ADR-014) is designed for a human reader at a terminal: it has a decorated first line, a Code line, an indented context block, and an optional `Suggestion:` block. The shape is readable but is not machine-parseable without work: the indentation carries meaning, the fields are interleaved with prose, and the context keys appear as an unordered set whose positions vary with the error's content.

A log consumer — a log aggregator, a metrics system, a debugger, a `grep`-based diagnostic — wants the same information as data: a field for the Code, a field for the Category, a field for the cause, and one field per context entry. The fields should be addressable by name, not by position. A count of errors by Code should be a `group by` on a named field, not a regular expression over a formatted string.

Go's standard library provides `log/slog` for structured logging. A `slog` handler renders a record as text or JSON, depending on the handler; the choice is the handler's, not the caller's. The `*forgeerr.Error` type can participate in this system by implementing `slog.LogValuer`, an interface with a single method, `LogValue() slog.Value`. When a `*Error` is passed to a log call as `slog.Any("error", err)`, `slog` calls `LogValue` and renders the returned value using the configured handler. Without `LogValue`, `slog` calls `err.Error()` and records the resulting string, which loses the structure: the Code, the Category, the cause, and the context are all collapsed into one string field.

The WBS 10.4.2 task is the freeze. It defines the value that `LogValue` returns: a `slog` group named `"error"` containing the Code, the Category, the cause (as a string), and one attribute per context entry (with keys sorted alphabetically). The task's inputs are WBS 10.4.1 (the user-facing format, which establishes the precedent that the error type has two renderings) and WBS 12.0 (the logging subsystem, which is the consumer). The affected parties are every developer who reads a Forge log, every log aggregator that indexes Forge's output, and every future logging configuration that chooses a `slog` handler.

The consequence of not deciding is that `slog.Any("error", err)` records a single string. The Code is buried in the string; the Category is absent; the cause is invisible; the context is unaddressable. A log aggregator cannot index by Code; a dashboard cannot group by Category; a diagnostic cannot extract the cause without parsing the message. The freeze is the mechanism by which the log output is structured.

---

## 2. Options Considered

### Option A — No `LogValue`; let `slog` record `err.Error()`

Do not implement `slog.LogValuer`. When a `*Error` is passed to `slog.Any`, the handler calls `err.Error()` and records the result as a single string field.

**Pros:**

- No new method to implement or document.
- The `Error()` method already exists; its output (`<code>: <message>`) is a known shape.
- The log output is consistent with the `Error()` string, which is familiar.

**Cons:**

- The structure is lost. The Code, the Category, the cause, and the context are all collapsed into one string.
- A log aggregator cannot index by Code; a `grep` for the Code matches a substring of the string field, not a named field.
- The `Error()` string is a programmer-facing representation, not a log representation; using it as the log value conflates two concerns.
- The `Error()` string does not include the Category, the cause, or the context; a log consumer that wants them cannot get them.

### Option B — `LogValue` returns a formatted string

Implement `LogValue` to return `slog.StringValue(err.Error())`. The value is a string, but it is produced by the type rather than by the standard library's default.

**Pros:**

- `LogValue` exists; the type participates in the `slog` system.
- The string can be produced with a specific format, distinct from `Error()`.

**Cons:**

- The value is still a string; a log consumer cannot address the fields.
- The format is a string, not a group; the aggregator's grouping and indexing are no better than with Option A.
- The method exists but does nothing that the default would not do, aside from changing the string's format.

### Option C — `LogValue` returns a `slog` group with structured fields

Implement `LogValue` to return a `slog` group named `"error"` containing the Code, the Category, the cause (as a string), and one attribute per context entry. The context keys are sorted alphabetically.

**Pros:**

- The structure is preserved. Each field is addressable by name.
- A log aggregator can index by `error.code` and group by `error.category`.
- The cause is a separate field; a diagnostic can extract it without parsing the message.
- The context entries are separate fields; a caller can query `error.file` or `error.line` directly.
- The shape is stable and documented; a future change is a contract change.
- The same `LogValue` works with the text handler and the JSON handler; the handler chooses the rendering, the type provides the data.
- The user-facing format (ADR-014) and the log format are complementary; the type has two renderings, one per audience.

**Cons:**

- The group's shape is a contract; a change requires an ADR.
- The `error` group name is a stable identifier; a rename is a breaking change for consumers that filter on it.
- The cause is rendered as a string, not as a nested group; a `*Error` cause is flattened to its `Error()` string, which loses the cause's own structure. A future extension could recurse, but the recursion would complicate the common case.
- The context keys' alphabetical sort is a determinism measure; without it, the attribute order would vary across runs, which would break golden-file tests and confuse readers.

### Option D — `LogValue` returns a JSON-encoded string

Implement `LogValue` to return `slog.StringValue(json.Marshal(err))`. The value is a JSON string.

**Pros:**

- The string is machine-parseable.
- The shape is a standard (JSON).

**Cons:**

- The value is a string; a log aggregator that expects structured output sees a string with an embedded JSON document, not a structured field.
- The `slog` handlers already produce JSON when configured to; double-encoding the error as a JSON string inside a JSON log entry is redundant.
- The type depends on `encoding/json`, which the package currently avoids.
- The string cannot be queried with the aggregator's field syntax; a consumer has to parse the string first.

**This option was rejected before the ADR was written; it is listed here for completeness.**

---

## 3. Decision

The log format is **Option C**: `*Error` implements `slog.LogValuer`, and `LogValue` returns a `slog` group named `"error"` containing the Code, the Category, the cause (as a string), and one attribute per context entry. The context keys are sorted alphabetically. The format is the programmer-facing counterpart to the user-facing format from ADR-014; the two renderings are complementary, not interchangeable.

The balance tipped toward Option C for three reasons. First, the structure is the point. A log consumer wants to group, index, and filter by named fields; a string field forces the consumer to parse, and parsing is what structured logging exists to avoid. Second, the two renderings are complementary. The user-facing format (ADR-014) renders the fields as prose; the log format renders them as data. A type that supports both gives each audience what it needs. Third, the `slog.LogValuer` interface is the standard mechanism. The interface's design (a single method returning a `slog.Value`) is exactly what the type needs; implementing it is a small, well-understood change.

The trade-offs accepted are: the group's shape is a contract (a change requires an ADR); the `error` group name is stable (a rename breaks consumers); the cause is rendered as a string rather than recursed (a `*Error` cause loses its own structure); and the context keys' alphabetical sort is a determinism measure that costs O(N log N) with N typically 0–3. The last is negligible; the first three are deliberate.

The decision assumes that the log consumer wants the fields at one level, not nested. If a future phase finds a need for a nested cause (for example, a cause that is itself a `*Error` with its own fields), the ADR for that phase would extend `LogValue` to recurse; the extension would be an additive change, not a breaking one, because the current shape's fields are a subset of the extension's.

---

## 4. Consequences

### 4.1 Positive

- The structure is preserved. A log entry for a `*Error` contains `error.code`, `error.category`, `error.cause`, and one attribute per context entry.
- A log aggregator can index by Code and group by Category without parsing the message.
- The same `LogValue` works with both `slog.NewTextHandler` and `slog.NewJSONHandler`; the handler chooses the rendering.
- The two renderings (user-facing and log-facing) are complementary. A reader who wants prose reads the user-facing format; a consumer that wants data reads the log format. The type provides both.
- The method is small and pure. It reads the receiver's fields, sorts the context keys, and returns a group. It does not touch the environment, does not allocate beyond the group, and is safe for concurrent use.
- The `error` group name is stable. A consumer that filters on `error.code` knows the field exists in every release.

### 4.2 Negative

- The group's shape is a contract. A change to an attribute's name, the group's name, or the set of attributes requires an ADR. The friction is deliberate but is real.
- The cause is rendered as a string, not as a nested group. A `*Error` cause loses its own structure; a consumer that wants the cause's Code or Category must extract it from the string or traverse the chain separately.
- The context keys' alphabetical sort is required for determinism. Without it, the attribute order would vary across runs (Go's map iteration is randomized), which would break golden-file tests and produce non-reproducible log entries.
- A secret-typed context value is redacted by the redaction layer (ADR-016), but the cause's `Error()` string is not redacted. The two behaviours are consistent with the redaction policy (the cause is a programmer concern), but a caller who places a secret in a cause's message leaks it into the log.
- The group name `"error"` is a stable identifier. A future rename would break consumers that filter on it; the rename would be a breaking change.

### 4.3 Neutral

- The `LogValue` method is defined on `*Error`, not on `Error`. The pointer receiver is required because `slog.LogValuer` is checked on the value passed to `slog.Any`, which is a pointer when the caller passes `err`.
- The `error` group name is a constant (`logGroupName`). The constant exists so that a future change is a one-line edit; the change itself is a contract change.
- The cause's `Error()` method is called by `LogValue`. The cause is an arbitrary error; its `Error()` method may not be safe for concurrent use. The package does not add a lock, because doing so would impose a cost on every `*Error` regardless of whether its cause needs it. The caller is responsible for the cause's concurrency safety.
- The attribute order within the group is fixed: Code, Category, cause, context. The order is not part of the contract in the sense that a consumer should not depend on it; the `slog` handlers render the attributes in the order they appear, and the order is stable.
- The `LogValue` method does not redact. Redaction is the redaction layer's concern; the layer is applied by the user-facing format (ADR-014) and, in a future phase, may be applied by the log format as well. In Phase 2, the log format renders context values as given; the caller is responsible for not placing secrets in context with non-secret-typed keys.

---

## 5. Related Requirements

- `LOG-002` — The programmer-facing error format. This ADR is the format.
- `ERR-001` — The canonical error type. The format renders the type's fields; the type defines what can be rendered.
- `ERR-002` — The error code taxonomy and stability policy. The format's `error.code` attribute renders the Codes that the registry defines.
- `SEC-003` — The redaction policy. The format's context attributes are subject to the policy in a future phase; the current phase renders them as given.
- `FR-CORE-010` *(pending)* — The `*forgeerr.Error` type implements `slog.LogValuer`. Must be added to the register when WBS 1.4 is complete.
- `NFR-TEST-006` *(pending)* — The log format has a dedicated test file (`log_test.go`) covering both the text handler and the JSON handler. Must be added to the register when WBS 1.4 is complete.

---

## 6. Related ADRs

- `ADR-011` — Closed error category taxonomy. The log format renders the Category as `error.category`; the taxonomy defines the categories.
- `ADR-012` — Error code registry and stability policy. The log format renders the Code as `error.code`; the registry defines the Codes.
- `ADR-013` — Error wrapping contract. The log format renders the cause as `error.cause`; the wrapping contract defines what the cause is.
- `ADR-014` — User-facing error format. The two formats are complementary; the user-facing format is for humans, the log-facing format is for machine consumers.
- `ADR-016` — Error redaction policy. The redaction policy applies to the log format's context attributes; the policy defines what is redacted and how.
- `ADR-017` — Error verbose modes. The verbose mode is a rendering concern of the user-facing format; the log format is the same in both modes.

---

## 7. Notes

### Open questions deferred

- **Should the cause be a nested group when it is a `*Error`?** A nested group would preserve the cause's own fields. The current design renders the cause as a string. A future ADR could recurse; the recursion would be an additive change because the string field is a subset of the nested group's data.

- **Should the Category be a field or be derived from the Code?** The Category is derivable from the Code's prefix. The current design renders it as a separate field (`error.category`), which is redundant but is convenient for consumers that want to group without parsing. A future ADR could remove the redundancy if a consumer's cost becomes a concern.

- **Should the context keys be sorted?** They are sorted alphabetically in the current design, which makes the attribute order deterministic. A future change to the order (for example, to preserve insertion order) would require the context map to be an ordered structure, which the `*Error` type does not have. The sort is the pragmatic choice for a map-backed context.

- **Should the log format redact context values?** In Phase 2, the log format renders context values as given. The redaction layer (ADR-016) is applied by the user-facing format only. A future phase may apply the layer to the log format as well; the change would be additive (the redacted value is a subset of the unredacted one) and would require an ADR.

### Follow-up decisions anticipated

- **WBS 10.4.3** (redaction) defines the redaction policy. The log format's interaction with the policy is deferred to that task; the current design renders the values as given.
- **WBS 12.0** (logging) consumes the log format. The logging subsystem's handler configuration (text vs JSON) is the handler's choice; the `LogValue` method provides the data.
- **WBS 10.5.1** (boundary integration) routes every error through the user-facing format, not the log format. The two formats are separate consumers; the boundary uses the user-facing one. A future integration may route through the log format for diagnostic purposes.

### External references

- The Go blog post "Structured Logging with `log/slog`" describes the design of `slog` and the `LogValuer` interface. The interface's single-method shape and its role in the handler chain are the primary influence on this ADR.
- The `slog` documentation's treatment of `GroupValue` and `Attr` is the reference for the group's construction. The group's name (`"error"`) follows the convention of naming the group after the entity it describes.
- The Go 1.21 release notes introduce `log/slog`; the standard library's design for structured logging is the platform the log format targets.
