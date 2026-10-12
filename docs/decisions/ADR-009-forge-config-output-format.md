# ADR-009: `forge config` Output Format

**Status:** Proposed
**Date:** 2026-10-12
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

ADR-008 established that `forge config` displays the CLI's resolved
configuration and is read-only in Phase 2. That ADR fixed the
command's *subject* but not its *rendering*. A subject without a
rendering is not yet a contract: two implementations of "display the
resolved configuration" could produce two different outputs, and
neither would be wrong until something froze the format.

Two forces make the format worth freezing. First, scripts read
`forge config`'s output. A developer debugging a CI failure pipes
the output to `grep`, `jq`, or a diff tool; a script that captures
the output and compares it across invocations expects the shape to
be stable. A format that drifts between releases breaks those
scripts silently. Freezing the format is the mechanism that turns
the drift into a reviewable change.

Second, the command has two audiences with different needs. A human
reading a terminal wants a readable, aligned block with the fields
labelled and the sources annotated. A machine reading a pipe wants
a self-describing, parseable structure with no formatting that must
be stripped. The two needs are not served by one format. The CLI
already has a convention for this split: the global `--format` flag
selects a rendering, and each command that renders output defines
its own value set. The `forge version` command introduced the
convention with `text` and `json` (WBS 6.4.1, WBS 6.4.2). `forge
config` follows the same convention.

A third force is the source-attribution requirement. The config
command is the only Phase 2 command whose output includes the
*source* of each value — which precedence layer produced it. The
source is not a config field; it is metadata about the field's
resolution. The format must carry it without conflating it with the
value. This is a design constraint that the text and JSON formats
each resolve differently (a parenthetical suffix in text, a sibling
key in JSON) and that both must resolve *consistently* — a source
attribution that reads `env` in one format and `environment` in the
other would be a defect.

The problem this ADR addresses: *what byte sequence does `forge
config` produce in each of its output modes, and what does that
sequence mean?* Without a frozen format, the command's output is a
rendering decision that can change silently; with one, a change is
a breaking change that requires a new ADR, a golden-file update, and
a note in the spec. The freeze bounds the cost of future edits.

This ADR relates to ADR-008 (the command's scope), ADR-007
(configuration foundation), WBS 9.2.1 (text format freeze), WBS
9.4.1 (JSON format freeze), WBS 9.2.2 (source attribution), and the
CLI UX specification § 4.6.1, § 4.6.2, § 4.9.1, and § 4.9.2. The
audience is the Forge Core Team, script authors who consume the
output, and any future contributor who proposes a new field or a
change to the rendering.

---

## 2. Options Considered

### Option A — A single human-readable format, no JSON mode

`forge config` produces one format: a labelled, aligned text block.
The command has no `--format` flag and no JSON mode. Scripts that
want machine-readable output parse the text block or read the
configuration file directly.

**Pros:**

- One format to design, freeze, and document.
- No parser for scripts to maintain; the format is human-only.
- The command's `--format` flag is not needed; the flag surface
  stays minimal.
- The rendering logic is one function with one set of rules.

**Cons:**

- Scripts that want to consume the output must parse the text
  block, which requires knowing its layout. Any change to the
  layout breaks the scripts, and the layout is not designed for
  parsing.
- The text format's alignment (label padding, value column) is a
  presentation concern that becomes a parsing contract. Coupling
  presentation to parsing is a mistake that the CLI's other
  commands already avoid.
- A script that only wants one field (for example, the log level)
  must parse the whole block or extract a substring by position,
  which is fragile.
- The CLI's `--format` convention is broken: every other
  output-rendering command accepts `--format`, and `forge config`
  would be the exception.

### Option B — Two formats: a human-readable text block and a machine-readable JSON object

`forge config` produces one of two formats, selected by the global
`--format` flag. The text format is the default and is optimised for
a terminal. The JSON format is selected by `--format json` and is a
single-line object with fixed field names.

**Pros:**

- Each audience gets a format designed for it. The text format is
  optimised for reading; the JSON format is optimised for parsing.
- The CLI's `--format` convention is preserved. A user who has
  learned the convention from `forge version` already knows how to
  ask for JSON.
- The JSON format is a stable schema. A script that reads
  `config_path`, `log_level`, or `log_level_source` is unaffected
  by changes to the text format's alignment or ordering.
- The JSON format can be validated against a published JSON
  Schema. The text format's byte sequence cannot be validated
  without a golden file (which is a snapshot, not a specification).
- The two formats share the same underlying `ResolvedConfig`
  value. A defect in the resolution (for example, a value whose
  source is wrong) manifests in both formats identically, which
  makes the defect easier to localize.

**Cons:**

- Two formats to design, freeze, document, and test. The testing
  surface is roughly doubled: each format needs golden files,
  unit tests, and schema checks.
- The two formats must agree on the semantic content. A future
  change to one (for example, adding a field) must be mirrored in
  the other unless the ADR explicitly scopes the addition. This
  is a maintenance burden.
- The `--format` flag's value set must be documented and enforced.
  A user who passes `--format yaml` gets an error, not YAML, which
  is a small usability surprise.

### Option C — A single JSON format, no text mode

`forge config` produces one format: a JSON object. The text mode is
not offered. Users who want to read the output pipe it to `jq` or a
similar tool.

**Pros:**

- One format to design and freeze.
- The format is machine-readable by construction. Scripts consume
  it without a separate mode.
- The format is a stable schema; a published JSON Schema can
  validate it.

**Cons:**

- The output is unreadable in a terminal without a pipe to `jq`.
  The most common use case — a user glancing at the effective
  configuration — requires an additional tool.
- The command contradicts the CLI's convention that the default
  output is human-readable. Every other Phase 2 command defaults
  to a human format.
- The CLI's UX principle § 9.6 ("default output is optimised for
  humans") is violated by a JSON-only default.

### Option D — Two formats: a text block and a JSON object, but the JSON object is a free-form map

`forge config` produces text by default and JSON under `--format
json`, but the JSON is a `map[string]any` keyed by the field names,
with no fixed schema. Fields are added or removed by editing the
map.

**Pros:**

- The JSON format is easy to extend: adding a field is adding a
  key to the map.
- No struct to define; the map is the schema.
- The rendering code is short.

**Cons:**

- The JSON output has no schema. A consumer cannot rely on the
  presence of any field; every read must check for existence.
- The map's key order is unspecified by the language; a future
  version of Go's `encoding/json` could change it (though in
  practice the current implementation sorts map keys).
- A published JSON Schema cannot describe a free-form map's
  fields; the schema document would be reduced to
  `additionalProperties: true`.
- The distinction between "a field the schema defines" and "a
  field the current build happens to emit" is lost. A script that
  reads `log_level_source` cannot tell whether the field is part
  of the contract or an implementation detail.

---

## 3. Decision

The decision is **Option B**: `forge config` produces two formats,
selected by the global `--format` flag. The text format is the
default; the JSON format is selected by `--format json`. The two
formats share the same underlying `ResolvedConfig` value and differ
only in their rendering.

The text format is frozen as a labelled block: a title line, a
separator, a blank line, a `Source:` line for the resolved file
path, and two value lines for `Log level` and `Output`, each
followed by a parenthetical source attribution. The format's
byte-level rules are enumerated in § 4.6.1 of the CLI UX
specification and pinned by golden files.

The JSON format is frozen as a single-line object with five
alphabetically ordered keys: `config_path`, `log_level`,
`log_level_source`, `output_format`, `output_format_source`. The
`config_path` value is `null` when no file was resolved; every
other value is a string. The format's schema is published at
`schemas/config.schema.json` and is pinned by a test that compares
the schema's property list against the struct's JSON tags.

Option B was chosen over Option A because scripts need a format
designed for parsing, and the text format's alignment is a
presentation concern that should not become a parsing contract.
Option B was chosen over Option C because the command's primary use
case is a human reading the effective configuration in a terminal;
a JSON-only default would violate the CLI's UX principle § 9.6 and
would force every user to pipe the output through a JSON viewer.
Option B was chosen over Option D because a fixed schema is what
lets consumers rely on the output and lets the specification
describe the format precisely; a free-form map trades extensibility
for unverifiability, and the config command's schema is small
enough that the trade is not worth making.

The chosen format assumes:

1. **The CLI configuration schema remains small.** If the schema
   grows, both formats must grow with it. A schema addition is a
   format change; the ADR that adds the schema field must also
   update the format.
2. **The source vocabulary is the four frozen values** (`default`,
   `file`, `env`, `flag`). A fifth source would require a change
   to both formats and to the loader; the change is not
   anticipated but is not forbidden.
3. **The JSON format is not versioned.** No `schemaVersion` field
   is emitted. The schema is identified by its field names. A
   breaking change to a field name is an ADR-worthy event; an
   additive change (a new field) is not. The `forge version`
   JSON format uses the same unversioned convention (see § 8.1
   of the spec for the exceptions).

---

## 4. Consequences

### 4.1 Positive

- Each audience has a format designed for it. A user reads the
  text block; a script reads the JSON object.
- The CLI's `--format` convention is preserved. A user who knows
  `forge version --format json` already knows how to ask `forge
  config` for JSON.
- The JSON format is a stable schema. A consumer that reads
  `config_path` is unaffected by a change to the text format's
  alignment, and vice versa.
- The JSON format is publishable as a JSON Schema. The schema file
  at `schemas/config.schema.json` is a machine-readable contract
  that a consumer can validate against; the text format's contract
  is a golden file, which is a snapshot but not a schema.
- The two formats share the `ResolvedConfig` value. A defect in
  the resolution (for example, a value whose source is wrong)
  surfaces in both formats identically. The defect is not hidden
  by choosing a format.
- The formats are deterministic. Two invocations with the same
  inputs produce the same output, byte for byte, in both modes.
  The determinism is a precondition for golden-file testing and
  for scripting.
- The source attribution's vocabulary is uniform across the two
  formats: the same four strings appear in both. A user who learns
  the vocabulary from the text format recognizes it in the JSON
  format without translation.

### 4.2 Negative

- Two formats to design, freeze, document, and test. The testing
  surface is roughly doubled: each format needs unit tests,
  golden files, and (for JSON) a schema check. The additional
  surface is the accepted cost of the split.
- A schema addition to the CLI configuration (a future ADR) must
  update both formats. The dual update is a maintenance burden
  that the ADR accepts in exchange for serving both audiences.
- The two formats must agree on the semantic content. A change to
  one that is not mirrored in the other produces a defect. The
  defect is caught by the tests (both formats are tested against
  the same input), but the need to catch it is a cost.
- The `--format` flag's value set (`text`, `json`) is now part of
  the command's contract. A user who passes `--format yaml` gets
  an error, which is correct but is a small usability surprise
  for a user who expected YAML support. Adding a third value
  later requires an ADR.
- The JSON format's keys are alphabetical by struct field order.
  This is a consequence of Go's `encoding/json` emitting struct
  fields in declaration order; the struct's fields are declared
  in alphabetical order to make the wire order alphabetical.
  A reader who expects a "logical" order (for example, values
  before sources) sees alphabetical order instead. The order is
  deliberate and is documented; the reader must consult the
  specification.
- The text format's column widths (`configLabelWidth` = 14,
  `configValueColumn` = 6) are rendering details that a script
  parsing the text format must know. The JSON format does not
  have this problem. A script that reads the JSON format is
  unaffected by the widths; a script that reads the text format
  is coupled to them. The coupling is the accepted cost of the
  text format's readability.

### 4.3 Neutral

- The text format uses the U+2500 box-drawing character for its
  separator. The choice was made by WBS 9.2.1 and is preserved
  here. The character is in the Unicode "Box Drawing" block and
  is rendered by most terminal fonts. The character is not ASCII,
  which is a deliberate departure from the ASCII-only policy
  (§ 4.10j) that governs the CLI's identity strings. The
  separator is not an identity string; it is a rendering
  decoration. The policy's scope was clarified to permit the
  separator.
- The JSON format emits `null` for an absent config file path.
  The choice was made by WBS 9.4.1 and is preserved here. The
  alternative (`""`) would be ambiguous: an empty string could
  mean "the file's path is empty" or "no file was resolved". The
  `null` distinguishes the two. A consumer that reads the JSON
  must handle the null case; the format documents it.
- The JSON format's keys are `log_level` and `output_format`,
  following the schema's snake_case convention. The choice was
  made by WBS 6.4.2 and is preserved here. A consumer that reads
  the JSON uses the snake_case names; a Go programmer who reads
  the struct uses the PascalCase field names. The mapping is
  explicit and tested.
- The text format's labels are `Source:`, `Log level:`, and
  `Output:`. The capitalization and the trailing colons are part
  of the format. A change to any of them is a format change.
- The JSON format's source-attribution fields use the
  `<field>_source` suffix (`log_level_source`,
  `output_format_source`). The suffix is the convention
  introduced by WBS 9.4.1. A future schema addition that carries
  a source uses the same convention.

---

## 5. Related Requirements

The requirements below are drawn from
[`docs/requirements-register.md`](../requirements-register.md).
Requirement IDs that do not yet exist are marked as pending.

- `FR-CLI-006` — `forge config` displays the resolved CLI
  configuration. This ADR establishes the output format for that
  display.
- `FR-CLI-007` — `forge config --path` prints only the resolved
  config file path. The `--path` mode's format is a variant of
  this ADR's formats; its specific rules are frozen by a
  follow-on ADR.
- `FR-CFG-002` — the source of each configuration value is
  observable. Both formats carry the source; the text format
  carries it as a parenthetical suffix, the JSON format as a
  sibling field.
- `UX-006` — the command's output is one of the two frozen
  formats. This ADR is the source of that requirement.
- `UX-007` — the JSON output is a single line terminated by
  `\n`. This ADR is the source of that requirement, which
  mirrors the convention from `forge version` (WBS 6.4.2).

The following requirement is proposed but not yet registered:

> The following requirement must be added to the register when
> WBS 1.4 is complete: `UX-0NN` — the JSON output of `forge
> config` conforms to `schemas/config.schema.json`, and the schema
> is kept in sync with the struct by a test.

---

## 6. Related ADRs

- `ADR-007` — configuration foundation, precedence, discovery,
  and error taxonomy. This ADR's formats render the values the
  loader produces; the source-attribution vocabulary is defined
  by ADR-007 (and by WBS 9.2.2, which the ADR records).
- `ADR-008` — `forge config` command scope. This ADR is the
  output-format counterpart to ADR-008's scope statement. The
  two together define what the command displays and how it
  displays it. This ADR does not supersede any clause of
  ADR-008; the two are complementary.
- `ADR-006` — help and CLI UX contracts. The `forge config`
  output follows ADR-006's conventions for `--format`
  (`human`/`json` in ADR-006's vocabulary; the shipped value
  set is `text`/`json`, and the difference is documented in
  § 4.14 of the spec). ADR-006's stream boundary (§ 4.10h) is
  honoured: both formats go to stdout on success. This ADR does
  not supersede any clause of ADR-006; the two are
  complementary.
- `ADR-010` — the `--path` flag and the interaction with the
  global flags. ADR-010's `--path` mode reuses this ADR's
  formats in a restricted shape (a single-key JSON object, a
  single-line text output). The two ADRs share the JSON key
  `config_path`; the key's alignment is documented in both.

---

## 7. Notes

**Deferred decisions.** The following are anticipated but not
decided here.

- **A third output format.** A future phase may want a format
  other than text or JSON (for example, YAML for a script that
  already consumes YAML, or a Go template for a custom render).
  Each new format requires its own ADR and its own golden files.
  This ADR authorizes two formats only.
- **`schemaVersion`.** The JSON output does not include a
  version field. A future change that breaks the schema (a
  rename or a removal) may need one. The decision to add it is
  deferred; the current schema is identified by its field names.
- **Hiding fields.** If the CLI configuration schema ever gains
  a field whose value should not appear in `forge config`'s
  output (for example, a token), the current "display every
  schema field" rule must be extended. The extension is
  deferred; the schema's no-secrets rule (WBS 8.2.1) is the
  current protection.

**Anticipated schema growth.** The CLI configuration schema is
frozen at two value fields and one metadata field. A future
addition requires an ADR. That ADR must also state how the new
field appears in each of this ADR's two formats. The default is
"a line in the text format, a key in the JSON format"; a
different treatment (for example, hiding the field, or emitting
it only in one format) requires explicit justification in the
new ADR.

**On the `text` vs `human` naming.** The CLI UX specification's
earlier drafts called this format `human`. The shipped value is
`text`. The rename aligns the config command with the version
command's `text` format (WBS 6.4.1). The `human` value is not
accepted by any command in Phase 2. § 4.14 of the spec records
the change.

**On the U+2500 separator.** The separator is a Unicode
box-drawing character, not ASCII. The choice is deliberate: the
character draws a visually clean line under the title and is
rendered by the terminal fonts the CLI targets. The ASCII-only
policy in § 4.10j governs the CLI's identity strings and
diagnostics, not the config command's output decorations. If a
future phase extends the ASCII-only policy to decorations, the
separator changes to a hyphen and both formats are regenerated.

**Follow-up.** The `--path` flag and the interaction with the
global flags are the subject of ADR-010. The two ADRs are
companions: this ADR defines the full-configuration formats;
ADR-010 defines the restricted `--path` mode and how the
formats compose with `--quiet`, `--verbose`, and `--config`.

---

*End of ADR-009.*
