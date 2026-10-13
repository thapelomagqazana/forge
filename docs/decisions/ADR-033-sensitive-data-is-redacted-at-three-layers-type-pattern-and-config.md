# ADR-033: Sensitive data is redacted at three layers: type, pattern, and config

**Status:** Accepted
**Date:** 2026-10-13
**Deciders:** [@thapelomagqazana] (Security Owner)
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge's logger (per ADR-026) emits diagnostics to stderr; its error
system (per ADR-014) renders failures to stderr; its config loader
(per WBS 8.4.2) reads a YAML file that may contain user-provided
values. Each of these surfaces can, in principle, carry a secret —
a password, a token, an API key, a private key — from the user's
environment into a stream the user did not intend to be a
distribution channel. A user who runs `forge --verbose` in a
terminal sees their own secrets, which is not a leak. A user whose
CI system captures stderr to a log aggregator, whose log aggregator
is readable by their colleagues, whose colleagues include a
contractor with a different access level, has a leak. The threat
model is not "Forge is malicious"; it is "Forge's output goes
somewhere the user did not anticipate, and a secret in that output
is exposed."

The forces at play are three. First, the sources of secrets are
diverse. A secret can arrive as a flag value (`--token=...`), an
environment variable (`FORGE_TOKEN=...`), a config file field
(`token: ...`), a template variable (`{{ .Token }}`), an error
cause (a wrapped `net/http` error that includes the URL's query
string), or a log attribute (a caller passes `"token", tok` to a
`logger.Info` call). A single mechanism cannot cover all six
sources; each source has a different shape, and a mechanism that
covers one may miss another. Second, the mechanisms have different
*reliability* characteristics. A type-level mechanism (a `Secret`
type whose `LogValue` returns `"[REDACTED]"`) is reliable: a value
of the type is redacted by construction, and the only way to
bypass it is to convert the value to a string before passing it,
which is a code change a reviewer can see. A pattern-based
mechanism (a middleware that scans strings for `password=`,
`token=`, PEM headers) is best-effort: it catches the common cases
and misses the uncommon ones, and its false positives (a
legitimate string that happens to match a pattern) are as
problematic as its false negatives. A config-aware mechanism (a
`sensitive: true` marker on a config field) is reliable for fields
it is applied to and absent for fields it is not; the risk is a
field that should be marked and is not. Third, the cost of a leak
is asymmetric with the cost of an over-redaction. A leaked secret
is a security incident; a redacted legitimate value is an
inconvenience (a user who reads the log sees `[REDACTED]` where
they expected a value, and they re-run without `--verbose` or
read the value from its source directly). The asymmetry favors
over-redaction when the two are in tension.

The decision relates to ADR-014 (the user-facing error format),
ADR-016 (the error redaction policy, which is a precedent for this
ADR), ADR-022 (the stream contract), ADR-025 (the record shape),
ADR-026 (the `Logger` interface), and ADR-034 (which will define
the redaction policy's closure property). The CLI UX spec's § 4.12
mentions the `--verbose` flag's interaction with secrets but does
not define the redaction mechanisms. If no decision is made, the
CLI's output may carry secrets to destinations the user did not
anticipate, and the exposure is invisible to the user, the CI
system, and the log aggregator.

---

## 2. Options Considered

### Option A — Type-level redaction only

Introduce a `Secret` type (a `string` wrapper) whose `LogValue()`
method returns `"[REDACTED]"`. Every value that might be a secret
is wrapped in a `Secret` before it is passed to a logger or
rendered in an error. The type is the only mechanism; pattern-based
and config-aware redaction are not implemented.

**Pros:**

- The mechanism is reliable for values of the type. A `Secret` is
  redacted by construction; the only way to bypass the redaction
  is to convert the value to a string (`string(secret)`), which
  is a code change a reviewer can see and flag.
- The mechanism is simple: one type, one method, one rule ("wrap
  every secret in a `Secret`").
- The mechanism has no false positives. A value that is not a
  `Secret` is not redacted, even if it looks like a secret; a
  value that is a `Secret` is always redacted, even if it does
  not look like one. The type is the contract.
- The mechanism is testable at the type level: a test that
  constructs a `Secret` and passes it to a logger asserts that
  the output contains `[REDACTED]` and not the value.

**Cons:**

- The mechanism requires the caller to know which values are
  secrets. A caller that does not know (because the value
  arrived from a source the caller does not control, or because
  the value's semantic role is unclear) cannot wrap it, and the
  value is not redacted. The mechanism's reliability depends on
  the caller's discipline, which is exactly the property that a
  defense-in-depth approach exists to avoid.
- The mechanism cannot catch secrets that arrive in error
  *values* (a wrapped `net/http` error that includes the URL's
  query string). The error system renders its causes; if a
  cause contains a secret, the `Secret` type does not help,
  because the cause is not a `Secret`.
- The mechanism cannot catch secrets that arrive in the config
  file. The config loader parses the file's YAML; a `token:
  abc123` field's value is a plain string, not a `Secret`. The
  loader would have to wrap every field it thinks might be a
  secret, which is the same "caller must know" problem.
- The mechanism cannot catch secrets that arrive in log
  *attributes* if the caller forgets to wrap them. A caller
  who passes `"token", tok` where `tok` is a plain string is
  not redacted; the caller's discipline is the only defense.

### Option B — Pattern-based redaction only

A middleware wraps the logger's `Sink` and scans every record's
message and attributes for known patterns: `password=`,
`token=`, `api_key=`, `apikey=`, `authorization:`, `bearer`,
`-----BEGIN PRIVATE KEY-----`, and (optionally) high-entropy
strings. Matches are replaced with `[REDACTED]`. No `Secret` type;
no config-aware redaction.

**Pros:**

- The mechanism catches secrets the caller did not know were
  secrets. A caller who passes a plain string that contains
  `password=foo` is redacted, even if the caller did not
  recognize the value as a secret.
- The mechanism catches secrets in error *values*. The error
  system's rendered output is a string; the middleware scans
  the string and redacts matches.
- The mechanism catches secrets in the config file. The config
  loader's output is a `Config` value; when a field is rendered
  (in `forge config`'s output or in a debug record), the
  middleware scans the rendered string.
- The mechanism is a single point of control: the middleware is
  applied once, and every record goes through it.

**Cons:**

- The mechanism has false positives. A legitimate string that
  happens to match `token=` (for example, a documentation
  example that includes `token=abc`) is redacted, and the
  user's log loses information they needed. The false positive
  is the cost of the pattern's broadness.
- The mechanism has false negatives. A secret that does not
  match any pattern (for example, a base64-encoded token
  without a `token=` prefix, a secret in a URL fragment, a
  secret split across multiple lines) is not redacted. The
  false negative is the cost of the pattern's specificity.
- The mechanism is hard to test completely. The space of
  strings that could contain a secret is unbounded; the
  pattern set covers the common cases and misses the
  uncommon ones. A test that injects a secret in a format the
  pattern set does not recognize asserts nothing.
- The mechanism's pattern set is a maintenance burden. New
  secret formats (a new token prefix, a new credential
  scheme) require new patterns; the patterns' age is a
  liability.
- The mechanism's performance is a consideration for high-
  volume logging; the pattern scan runs on every record.

### Option C — Config-aware redaction only

The config file's schema gains a `sensitive: true` marker on
fields whose values are secrets (for example, `token`,
`password`, `api_key`). The config loader wraps those fields'
values in a `Secret` before they reach any logger or renderer.
Fields without the marker are not redacted.

**Pros:**

- The mechanism is reliable for marked fields. A `sensitive:
  true` field is redacted by construction; the loader's
  wrapper is the single point of control.
- The mechanism is simple for the config file's users. A user
  who marks a field as sensitive does not have to remember to
  wrap the value anywhere else.
- The mechanism has no false positives for unmarked fields. A
  legitimate value in a non-sensitive field is not redacted,
  even if it looks like a secret.

**Cons:**

- The mechanism only covers the config file. A secret that
  arrives as a flag value, an environment variable, an error
  cause, or a log attribute is not covered. The mechanism's
  scope is narrow.
- The mechanism requires the schema's author (Forge's
  maintainers) to mark every sensitive field. A field that
  should be marked and is not is a leak; the leak is
  invisible until it is exercised.
- The mechanism requires the schema's users to add the marker
  to their own sensitive fields. A user who adds a `token:`
  field to their config and forgets `sensitive: true` is not
  redacted. The user's discipline is the only defense for
  user-defined fields.
- The mechanism's coverage depends on the schema's
  completeness, which is a moving target as the config file
  gains fields.

### Option D — Layered redaction: type, pattern, and config

All three mechanisms are implemented together:

1. **Type-level** — a `Secret` type whose `LogValue()` returns
   `"[REDACTED]"`. Callers wrap values whose semantic role is a
   secret.
2. **Pattern-level** — a middleware on the logger's `Sink` that
   scans each record's message and attribute values for known
   secret patterns and redacts matches.
3. **Config-level** — a `sensitive: true` marker on config
   fields, applied by the loader, that wraps the fields' values
   in a `Secret`.

Each mechanism covers a different source and a different
reliability characteristic; the combination is defense in depth.

**Pros:**

- The three mechanisms cover the six sources of secrets. A
  flag value is wrapped by the caller (type) or caught by the
  middleware (pattern); an environment variable is wrapped by
  the caller (type) or caught by the middleware (pattern); a
  config file field is wrapped by the loader (config) or
  caught by the middleware (pattern); a template variable is
  wrapped by the renderer (type) or caught by the middleware
  (pattern); an error cause is caught by the middleware
  (pattern); a log attribute is wrapped by the caller (type)
  or caught by the middleware (pattern). No source is
  uncovered.
- The mechanisms have different reliability characteristics.
  The type-level mechanism is reliable for values it is
  applied to; the pattern-level mechanism catches values the
  type-level mechanism missed; the config-level mechanism
  catches config fields the caller might not have wrapped.
  A value that slips through one mechanism is caught by
  another, if the another's conditions are met.
- The combination is the correct response to the asymmetry
  between a leak and an over-redaction. A leak is a security
  incident; an over-redaction is an inconvenience. When the
  two are in tension, the combination favors over-redaction
  (the pattern set is broad) but provides reliability
  (the type set is exact).
- The mechanisms' combined coverage is testable: a test
  suite injects a secret into each of the six sources and
  asserts that none of the six appears in any output stream.
  The test is the security contract's executable form.
- The combination is a precedent. ADR-016's error redaction
  policy is a single mechanism (the error system's rendering
  redacts known keys); this ADR extends the policy to the
  logger and the config loader. The extended policy's
  mechanisms are the same shape as the precedent's, which
  makes the two policies teachable together.

**Cons:**

- Three mechanisms to implement, document, and test. The
  surface is larger than a single mechanism's would be.
- The mechanisms' interactions must be defined. If a value is
  wrapped in a `Secret` and the `Secret`'s rendered form is
  scanned by the pattern middleware, is the `[REDACTED]`
  token itself at risk of being re-redacted? (The answer: no,
  because `[REDACTED]` does not match any pattern; the
  interaction is benign but must be checked.) If the config
  loader wraps a field's value in a `Secret` and the pattern
  middleware scans the record, is the wrapper's rendering
  scanned twice? (The answer: the double-scan is harmless,
  but the redundancy must be documented.)
- The pattern middleware's false positives are the
  combination's worst case: a legitimate value is redacted,
  the user sees `[REDACTED]` where they expected a value, and
  the user's log is less useful. The combination's other
  mechanisms do not compensate for the pattern's false
  positives; the user's only recourse is to re-run without
  `--verbose` or to read the value from its source directly.
- The combination's coverage is not complete. A secret in a
  format that none of the mechanisms recognizes (a novel
  credential scheme, a secret split across lines, a secret
  encoded in an unusual format) is not redacted. The
  combination's coverage is "the common cases," not "all
  cases"; the documentation must say so.
- The three mechanisms' tests must cover the six sources,
  the three mechanisms' interactions, and the pattern
  middleware's false positives and false negatives. The test
  suite is a substantial investment, and its maintenance is
  ongoing as new secret formats emerge.

---

## 3. Decision

**Forge adopts Option D: sensitive data is redacted at three
layers — type, pattern, and config.** The three mechanisms are:

1. **Type-level redaction.** A `Secret` type (in
   `internal/logger/secret.go`) wraps a `string` and implements
   `LogValue() slog.Value` returning `slog.StringValue("[REDACTED]")`.
   Callers wrap values whose semantic role is a secret (a
   password, a token, an API key) before passing them to a
   logger or rendering them in an error. The type is the
   reliable mechanism; the only way to bypass it is
   `string(secret)`, which is a visible code change.

2. **Pattern-level redaction.** A middleware (in
   `internal/logger/redact.go`) wraps the logger's `Sink` and
   scans each record's message and attribute values for known
   secret patterns. Matches are replaced with `[REDACTED]`.
   The pattern set covers, at minimum: `password=`,
   `passwd=`, `token=`, `secret=`, `api_key=`, `apikey=`,
   `authorization:`, `bearer`, `-----BEGIN PRIVATE KEY-----`,
   `-----BEGIN RSA PRIVATE KEY-----`, and high-entropy strings
   that match a set of heuristics. The middleware is the
   best-effort mechanism; it catches the common cases and
   misses the uncommon ones.

3. **Config-level redaction.** The config file's schema (see
   `docs/forge-yaml-spec.md`) gains a `sensitive: true` marker
   on fields whose values are secrets. The loader
   (`internal/config/loader.go`) wraps those fields' values
   in a `Secret` before they reach any logger or renderer.
   The mechanism is the schema's completeness guarantee; a
   field that is marked is redacted, a field that is not is
   not.

The three mechanisms' combination is the security contract. A
secret that slips through one mechanism is caught by another, if
the other's conditions are met. The combination is tested by a
suite (`test/security/redaction_test.go`) that injects a secret
into each of the six sources — flag value, environment variable,
config field, template variable, error cause, log attribute —
and asserts that none appears in any output stream. The suite's
assertions are the executable form of the contract; a passing
suite is the evidence that the contract holds for the cases
tested.

The reasoning that tipped the balance was the sources'
diversity. A single mechanism cannot cover the six sources: the
type-level mechanism requires the caller to know which values
are secrets; the pattern-level mechanism requires the secret to
match a pattern; the config-level mechanism requires the field
to be marked. Each mechanism's coverage is a subset of the
sources, and the subsets are not the same. The combination's
coverage is the union, which is larger than any single
mechanism's. The combination's coverage is not complete — a
novel secret format may slip through all three — but the
combination's coverage is the best available with the mechanisms
that are practical to implement and maintain.

The reasoning against Option A (type-level only) was the
caller's discipline. The type-level mechanism's reliability
depends on the caller knowing which values are secrets; a
caller who does not know (because the value arrived from a
source they do not control, or because the value's semantic
role is unclear) cannot wrap it, and the value is not redacted.
The pattern-level mechanism does not require the caller to
know; it scans the rendered output regardless of the caller's
knowledge. The combination covers the case the type-level
mechanism misses.

The reasoning against Option B (pattern-level only) was the
false negatives. The pattern-level mechanism's coverage depends
on the pattern set; a secret in a format the set does not
recognize is not redacted. The type-level mechanism is reliable
for values it is applied to; the combination covers the case
the pattern-level mechanism misses.

The reasoning against Option C (config-level only) was the
scope. The config-level mechanism only covers the config file;
a secret that arrives as a flag value, an environment variable,
an error cause, or a log attribute is not covered. The
combination covers the sources the config-level mechanism does
not.

The tradeoffs accepted are the ones Option D lists as cons. The
three mechanisms' implementation, documentation, and testing
are a substantial investment; the investment is the cost of the
coverage. The mechanisms' interactions are benign but must be
documented; the double-scan of a `Secret`'s rendered form by
the pattern middleware is harmless (the `[REDACTED]` token does
not match any pattern) but is redundant. The pattern
middleware's false positives are the combination's worst case;
a user whose legitimate value is redacted sees `[REDACTED]`
and must re-run or read the value from its source. The
combination's coverage is not complete; a novel secret format
may slip through all three mechanisms, and the documentation
must say so.

The decision is provisional in one respect: the pattern set's
exact contents are the subject of WBS 12.7.1's implementation.
This ADR fixes the set's *shape* (the minimum patterns listed
above) and the mechanisms' relationships; the specific regexes
and their order are the WBS item's output. A future ADR could
extend the pattern set (for example, to include new credential
schemes) or refine the mechanisms' interactions; the ADR is
the policy, the WBS item is the implementation.

---

## 4. Consequences

### 4.1 Positive

- The three mechanisms cover the six sources of secrets. A
  secret that slips through one mechanism is caught by
  another, if the other's conditions are met. No source is
  entirely uncovered.
- The combination's reliability is better than any single
  mechanism's. The type-level mechanism is reliable for
  values it is applied to; the pattern-level mechanism
  catches values the type-level mechanism missed; the
  config-level mechanism catches config fields the caller
  might not have wrapped.
- The combination favors over-redaction when the two are in
  tension (a leak versus an over-redaction). The pattern
  set's broadness errs on the side of redacting; the type
  set's exactness errs on the side of reliability. The two
  are complementary.
- The combination's coverage is testable. A test suite
  injects a secret into each of the six sources and asserts
  that none appears in any output stream. A passing suite is
  the evidence that the contract holds for the cases tested.
- The combination is consistent with the precedent set by
  ADR-016's error redaction policy. The error system's
  rendering already redacts known keys; this ADR extends
  the policy's mechanisms to the logger and the config
  loader. The two policies are teachable together.
- The `Secret` type's `LogValue()` method integrates with
  `log/slog`'s `LogValuer` interface, which means the
  default `slog` handler applies the redaction automatically.
  The type-level mechanism does not require a custom
  handler; it works with the standard library's types.

### 4.2 Negative

- Three mechanisms to implement, document, and test. The
  surface is larger than a single mechanism's would be. The
  implementation is WBS 12.7.1's output; the documentation
  is `docs/security/logging-redaction.md`; the tests are
  `test/security/redaction_test.go`.
- The pattern middleware's false positives are the
  combination's worst case. A legitimate value that matches
  a pattern is redacted, and the user's log loses information
  they needed. The user's recourse is to re-run without
  `--verbose` or to read the value from its source directly.
  The false positive is the cost of the pattern's broadness.
- The combination's coverage is not complete. A secret in a
  format that none of the mechanisms recognizes is not
  redacted. The documentation must say so; a user who
  relies on the combination's completeness is misled.
- The mechanisms' interactions are benign but must be
  documented. The double-scan of a `Secret`'s rendered form
  by the pattern middleware is harmless but redundant; the
  redundancy must be documented so a future contributor does
  not "fix" it by removing one of the scans (which would
  weaken the other mechanism's coverage).
- The pattern set's maintenance is ongoing. New credential
  schemes (a new token prefix, a new authorization format)
  require new patterns; the patterns' age is a liability.
  The maintenance is a recurring cost.
- The config-level mechanism requires the schema's users
  (Forge's maintainers and the users who add their own
  fields) to mark sensitive fields. A field that should be
  marked and is not is a leak; the leak is invisible until
  it is exercised.

### 4.3 Neutral

- The `Secret` type's string representation is a valid
  `string` for callers who need the value (for example, to
  pass it to an API client). The caller's access to the
  value is not restricted; the type's redaction applies
  only to the logger's and error system's rendering. The
  distinction is deliberate: the redaction is a rendering
  concern, not a data-flow concern.
- The pattern middleware's performance is a consideration
  for high-volume logging. The scan runs on every record;
  the scan's cost is proportional to the record's size.
  The cost is not expected to be significant for Forge's
  volume (the CLI emits at most tens of records per
  invocation), but it is a consideration for a future
  high-volume subsystem.
- The config-level mechanism's `sensitive: true` marker is
  a schema change (the schema gains a new optional field).
  The change is additive; a config file without the marker
  behaves as it did before. The marker's absence on a
  field is the field's default (not sensitive), which is
  the correct default for fields whose sensitivity is not
  declared.
- The mechanisms' tests must cover the six sources, the
  three mechanisms' interactions, and the pattern
  middleware's false positives and false negatives. The
  test suite's scope is defined by WBS 12.7.1's
  acceptance criteria; the suite's maintenance is ongoing
  as new secret formats emerge.
- The redaction policy's documentation
  (`docs/security/logging-redaction.md`) is WBS 12.7.1's
  output; it is the authoritative statement of the
  mechanisms' details and of the coverage's limits. A
  future ADR could refine the policy; the current ADR is
  the policy's initial statement.

---

## 5. Related Requirements

- `SEC-LOG-001` through `SEC-LOG-010` — the security
  requirements for logging; the requirements are established
  by this decision (with WBS 12.7.1's implementation).
- `FR-CLI-LOG-006` — the logger's interface; the requirement
  is compatible with this decision's type-level mechanism
  (the `Secret` type implements `slog.LogValuer`).
- `FR-CLI-LOG-009` — `--verbose` elevates the log level;
  the requirement is compatible with this decision's
  redaction (verbose output is redacted).
- `NFR-OBSERVABILITY-001` — records are attributable to a
  level and a source; the requirement is compatible with
  this decision's redaction (the records' attribution is
  not a secret).
- `NFR-ARCHITECTURE-003` — the logger's interface is
  independent of CLI and domain packages; the requirement
  is compatible with this decision's `Secret` type (the
  type is in `internal/logger`, not in a CLI package).

> The following requirements must be added to the register
> when WBS 12.7.1 is complete:

- `SEC-LOG-001` through `SEC-LOG-010` (if not already present).

---

## 6. Related ADRs

- `ADR-014` — User-Facing Error Format. Establishes the
  error block's format; this decision's pattern middleware
  applies to the block's rendered output.
- `ADR-016` — Error Redaction Policy. Establishes the
  error system's redaction policy; this decision extends
  the policy's mechanisms to the logger and the config
  loader. The two policies are complementary.
- `ADR-022` — Output and logging are separate architectural
  layers. Establishes the stream the redacted records go to
  (stderr).
- `ADR-025` — Log records are structured with a
  schema-versioned JSON form and an unstable human form.
  Establishes the record shape the pattern middleware scans.
- `ADR-026` — The Logger interface has four level methods
  and a With combinator. Establishes the interface whose
  `Sink` is wrapped by the pattern middleware.
- `ADR-034` (expected) — The redaction policy is closed.
  Will define the closure property that this decision's
  mechanisms satisfy. The two ADRs are complementary: this
  ADR defines the mechanisms, ADR-034 defines the
  guarantee.

---

## 7. Notes

**Open questions deferred by this ADR:**

- The exact contents of the pattern set. This ADR fixes the
  set's *shape* (the minimum patterns listed above) and
  defers the specific regexes to WBS 12.7.1. The WBS item's
  output is the pattern set's implementation and its tests.
- The high-entropy detection's heuristics. The ADR
  mentions "high-entropy strings that match a set of
  heuristics" but does not specify the heuristics. The
  choice is WBS 12.7.1's; the heuristics' false-positive
  and false-negative rates are the WBS item's acceptance
  criteria.
- Whether the `Secret` type's `LogValue()` should return
  `slog.StringValue("[REDACTED]")` or a structured value
  (for example, `slog.Group("secret", slog.Bool("redacted",
  true))`). The current plan is the string; a future ADR
  could refine it if a structured value proves more useful.
- Whether the config-level mechanism should be implemented
  in the loader (`internal/config/loader.go`) or in the
  schema's parser (`internal/config/schema.go`). The
  current plan is the loader; the choice is WBS 12.7.1's.

**Follow-up decisions anticipated:**

- ADR-034 (expected) defines the redaction policy's closure
  property: every log field is subject to the policy, every
  mechanism has a test, and the fuzz test guarantees the
  middleware never panics and never leaks. This ADR's
  mechanisms are the policy's substance; ADR-034's
  guarantee is the policy's shape.
- WBS 12.7.1's implementation is the direct consequence of
  this ADR; the WBS item's outputs are
  `internal/logger/secret.go`, `internal/logger/redact.go`,
  `test/security/redaction_test.go`, and
  `docs/security/logging-redaction.md`.
- A future ADR could extend the pattern set or refine the
  mechanisms' interactions; the current ADR is the
  policy's initial statement, and the extensions are
  normal ADR territory.

**External references:**

- OWASP Logging Cheat Sheet — the guidance on what not to
  log and how to redact what is logged.
- Go standard library `log/slog`'s `LogValuer` interface —
  the mechanism the `Secret` type implements.
- The common secret patterns (password=, token=, api_key=,
  authorization:, bearer, PEM headers) — the patterns the
  middleware recognizes.
- `docs/security/logging-redaction.md` (expected) — the
  authoritative reference for this ADR's details.
- `docs/security-model.md` — the CLI's broader security
  model, of which this ADR's redaction policy is a part.

---

*End of ADR-033.*
