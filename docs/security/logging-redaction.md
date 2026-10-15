# Logging Redaction Policy

<!--
  Document:     logging-redaction.md
  Purpose:      The four-layer redaction model, the guarantees of
                each layer, and the discipline that keeps a
                credential out of Forge's stdout and stderr.
  Audience:     Contributors, security reviewers, operators.
  Stability:    Public contract for the guarantees. The pattern
                set is semi-stable (may change per release).
  Traceability: SEC-LOG-001 through SEC-LOG-010, ADR-023
  See also:     ../reference/verbose-mode.md, ../reference/quiet-mode.md,
                ../architecture/output-vs-logging.md
  Last Review:  2026-10-12
-->

## 1. The guarantee

Forge never writes a credential to stdout or stderr. A credential
is any value whose disclosure would grant access to a resource,
authenticate a caller, or reveal internal state that is not meant
for public consumption.

The guarantee is **end-to-end**: it covers the CLI's own output
(product), the CLI's diagnostics (log records), and the CLI's
errors (the human error block and the JSON error envelope).

## 2. The threat model

The policy defends against four classes of accidental leak:

1. **A contributor writes `log.Println(password)`.** The
   contributor holds a credential in a plain string and passes
   it to a logger. The leak is accidental and obvious in review
   if the reviewer knows what to look for.

2. **A credential appears in an error's context.** An error is
   returned with the credential in its context map; the error's
   rendering includes the context.

3. **A credential appears in a config field.** The config file
   contains a credential; a debug record reports the parsed
   config's fields.

4. **A credential appears in a payload.** A credential is part
   of an HTTP request, a response body, or a template variable;
   a trace logs the payload.

The policy does **not** defend against an attacker who has read
access to the process's memory, to the config file's contents, or
to the environment in which the process runs. It defends against
accidental disclosure through the CLI's own output streams.

## 3. The four-layer model

The policy uses four independent layers. Each layer catches leaks
the others miss; a leak must bypass all four to reach a stream.

### 3.1 Layer 1 — type-level redaction

**Mechanism.** A `Secret` type (`internal/logger/secret.go`)
wraps a credential-like string and implements four serialization
hooks:

- `fmt.Stringer` (used by `fmt.Sprintf`, `fmt.Println`).
- `fmt.GoStringer` (used by `%#v`).
- `slog.LogValuer` (used by `log/slog`).
- `encoding/json.Marshaler` (used by `json.Marshal`).

Every hook returns the literal `[REDACTED]`.

**Guarantee.** A value wrapped in `Secret` never appears in any
text its holder emits, through any of the four serialization
paths.

**Discipline.** Every credential-shaped value should be wrapped
in `Secret` at the boundary where it enters the program: the
environment read, the config load, the flag parse, the payload
extract.

**Limitation.** The layer requires the caller to remember to wrap
the value. A caller that holds a raw string is not protected by
this layer.

### 3.2 Layer 2 — pattern-based redaction

**Mechanism.** A middleware (`internal/logger/redact.go`) scans
every log record's `Message` and string `Attrs` for credential-
shaped substrings and replaces them with `[REDACTED]`. The
pattern set covers:

- `password`, `passwd`, `pwd`, `token`, `secret`, `api_key`,
  `apikey`, `api-key`, `auth_token`, `access_token`,
  `refresh_token`, `client_secret` — assigned with `=` or `:`.
- `Authorization: <scheme> <value>` — HTTP Authorization header.
- `Bearer <value>` — bare Bearer scheme.
- `-----BEGIN <TYPE> PRIVATE KEY-----` — PEM block.
- High-entropy tokens (Shannon entropy ≥ 3.5 bits per
  character, length ≥ 16).

**Guarantee.** A record emitted through the logger contains no
substring matching the pattern set.

**Discipline.** None required of the caller; the middleware runs
inside the logger's emission path.

**Limitation.** The pattern set is closed. A credential whose
shape is not in the set and whose entropy is below the threshold
is not redacted by this layer. Layer 1 is the mechanism for
such values.

### 3.3 Layer 3 — config-aware redaction

**Mechanism.** A loader hook (`internal/config/redact.go`)
identifies fields declared `sensitive: true` in the config
schema and wraps them in `Secret` at config-load time.

**Guarantee.** A config field declared sensitive is never logged
or serialized in its plain form. Every consumer that wants the
value calls `Secret.Reveal`.

**Discipline.** The schema declares the sensitive fields; the
loader applies the wrapping. The contributor who adds a
sensitive field to the schema is responsible for the
declaration.

**Limitation.** The layer covers the config file. Credentials
from other sources (environment variables, flags, payloads) are
covered by layers 1 and 2.

### 3.4 Layer 4 — test suite

**Mechanism.** An end-to-end test suite
(`test/security/redaction_test.go`) injects synthetic secrets
into every code path that can produce output and asserts the
secret never appears on stdout or stderr. A fuzz test
(`internal/logger/redact_fuzz_test.go`) asserts the middleware
never panics and never leaks under random input.

**Guarantee.** A regression that leaks a credential through any
covered path fails CI.

**Discipline.** Every new code path that can produce output
adds a case to the test suite.

**Limitation.** The suite covers the paths it knows about. A
future code path not in the suite is not covered until a case
is added.

## 4. The pattern set

The pattern set is the middleware's definition of
"credential-shaped". It is closed: a future addition is a
deliberate change to the policy and requires an ADR.

| Pattern | Matches | False-positive policy |
|---------|---------|----------------------|
| `password=<value>` | Key-value assignment | Acceptable; the string is credential-shaped. |
| `token=<value>` | Key-value assignment | Acceptable. |
| `secret=<value>` | Key-value assignment | Acceptable. |
| `api_key=<value>`, `apikey=<value>` | Key-value assignment | Acceptable. |
| `Authorization: <scheme> <value>` | HTTP header | Precise; the header is unambiguous. |
| `Bearer <value>` | Bare scheme | Acceptable. |
| `-----BEGIN ... PRIVATE KEY-----` | PEM block | Precise; the block is unambiguous. |
| High-entropy token | Shannon entropy ≥ 3.5, length ≥ 16 | Acceptable; over-redaction of UUIDs and hashes is the documented trade. |

## 5. What is not redacted

The policy does **not** redact:

- Public identifiers: commit hashes, version strings, public
  keys, endpoint URLs.
- The names of credentials (the key `password` is not redacted;
  only its value).
- The redaction marker itself (it is idempotent).
- The shape of a redacted field (a `password=` key remains
  visible; only the value is replaced).

The distinction is deliberate: redacting the key would make logs
unreadable. The reader sees "a credential was here"; the value
is what they must not see.

## 6. The discipline

A contributor who handles a credential follows this procedure:

1. **Wrap the value at the boundary.** When reading from the
   environment, the config file, a flag, or a payload, wrap
   the value in `logger.Secret` immediately.

2. **Do not call `Reveal` for logging.** `Reveal` is for
   using the credential (authenticating a request, comparing a
   supplied value). Calling it in a log path is a bug.

3. **Declare the field sensitive in the schema** if the
   credential lives in `forge.yaml`.

4. **Add a case to the redaction test suite** if the new code
   path can produce output.

A reviewer who sees `Reveal()` in a log path, a credential in a
plain string that flows into a logger, or a sensitive field that
is not declared in the schema, blocks the PR.

## 7. Testing

The redaction test suite covers five paths:

| Path | Test | What it asserts |
|------|------|-----------------|
| Config load | `TestRedaction_ConfigLoad` | A synthetic secret in a sensitive field does not leak through any fmt verb. |
| Template variables | `TestRedaction_TemplateVars` | A `Secret` in a template var renders as the marker. |
| Error paths | `TestRedaction_ErrorPaths` | A `Secret` in an error's context renders as the marker. |
| Verbose mode | `TestRedaction_VerboseMode` | A DEBUG record with a `Secret` attr does not leak. |
| JSON mode | `TestRedaction_JSONMode` | A `Secret` in a JSON envelope renders as the marker. |

The fuzz test (`FuzzRedactString`) asserts two invariants under
random input: no panic, no leak of a credential-shaped token
that the input contained. The WBS's KPI is ≥ 1M iterations
without a panic; the Go fuzzer's `-fuzztime=1m` reaches that
count on a typical developer machine.

## 8. Stability

| Aspect | Stability |
|--------|-----------|
| The guarantee (no credential on stdout/stderr) | **Stable.** Public contract. |
| The marker (`[REDACTED]`) | **Stable.** Public contract; a log aggregator's rules depend on it. |
| The four-layer model | **Stable.** A future layer is an addition, not a replacement. |
| The pattern set | **Semi-stable.** May grow per release; each addition is documented. |
| The `Secret` type's method set | **Stable.** Adding a method is an addition; removing or renaming is breaking. |

## 9. References

- [ADR-022](../decisions/ADR-022-output-logging-separation.md) —
  output/logging separation.
- [`../architecture/output-vs-logging.md`](../architecture/output-vs-logging.md)
  — the two-layer output architecture.
- [`../reference/verbose-mode.md`](../reference/verbose-mode.md)
  — the verbose-mode contract.
- [`../reference/quiet-mode.md`](../reference/quiet-mode.md) —
  the quiet-mode contract.
- OWASP Logging Cheat Sheet.
- SEC-LOG-001 through SEC-LOG-010, WBS 12.7.1.
