# ADR-034: The redaction policy is closed: no log field is exempt, and every mechanism has a test

**Status:** Accepted
**Date:** 2026-10-13
**Deciders:** [@thapelomagqazana] (Security Owner)
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

ADR-033 established three redaction mechanisms — type, pattern,
and config — as the CLI's defense against leaking secrets in
diagnostics. The mechanisms' design is settled: a `Secret` type
whose `LogValue()` returns `[REDACTED]`; a pattern middleware on
the logger's `Sink`; a `sensitive: true` marker on config fields.
What the ADR did *not* establish is the policy's *closure
property*: the guarantee that the mechanisms apply uniformly to
every field the logger can emit, that every mechanism is
exercised by a test, and that the policy's behavior under
adversarial input is known. Without a closure property, the
mechanisms are a set of implementations whose coverage is
whatever the implementations happen to cover; a field the
mechanisms happen not to touch is a field the policy does not
protect, and the policy's documentation cannot claim otherwise.

The forces at play are three. First, "every field" is a
quantifiable property. ADR-025 defined the log record's fields:
`ts`, `level`, `component`, `msg`, `correlation_id`, and `attrs`.
Six fields, of which `msg` and `attrs` can carry user data; the
other four are structural (a timestamp, a level string, a
component tag, a UUID). The closure property's first half is that
the pattern middleware applies to *every* record the logger
emits, and that its scan covers `msg` and every value in `attrs`.
A mechanism that covers only some of a record's fields is not
closed; the field it misses is the leak. Second, "every mechanism
has a test" is an accountability property. A mechanism without a
test is a mechanism whose behavior is whatever the implementer
believed; the belief is not evidence, and a future change can
invalidate it silently. The closure property's second half is
that each of the three mechanisms (type, pattern, config) has a
dedicated test suite that exercises it in isolation and in
combination with the others. Third, "adversarial input" is a
robustness property. The pattern middleware runs on every record;
a malformed input (a record whose `msg` is a megabyte of
random bytes, an `attrs` map with a cyclic reference, a string
that triggers a pathological regex match) must not panic the
middleware and must not leak the input. The closure property's
third half is a fuzz test that runs the middleware on random
inputs and asserts that it never panics and never leaks.

The decision relates to ADR-016 (the error system's redaction
policy, which is a precedent for this ADR's closure shape),
ADR-025 (the record's fields, which the closure property
quantifies over), ADR-026 (the `Logger` interface and its `Sink`,
which the pattern middleware wraps), ADR-033 (the three
mechanisms the closure property closes over), and the
implementation WBS 12.7.1. The CLI UX spec's § 4.12 mentions the
`--verbose` flag's interaction with secrets but does not define
the policy's closure. If no decision is made, the mechanisms
remain a set of implementations whose coverage is whatever they
happen to cover; the policy's documentation cannot claim
uniformity, and a future field addition to the record can escape
the pattern middleware without notice.

---

## 2. Options Considered

### Option A — No closure property; the mechanisms' coverage is whatever they cover

The three mechanisms are implemented independently. The pattern
middleware is applied where its author chose to apply it; the
type-level mechanism is used where callers remember to use it;
the config-level mechanism is applied to fields the schema
marks. No policy statement asserts that every field is covered,
that every mechanism has a test, or that the middleware is
robust under adversarial input.

**Pros:**

- No policy statement to maintain. The mechanisms' coverage is
  a matter of implementation, not documentation.
- The implementations can evolve independently; a new field's
  coverage is a case-by-case decision rather than a policy
  question.
- No additional tests beyond the mechanisms' own unit tests.

**Cons:**

- The mechanisms' coverage is not knowable from the
  documentation. A user who reads the redaction policy doc
  cannot determine which fields are protected and which are
  not.
- A new field added to ADR-025's `Record` type is not
  automatically covered by the pattern middleware. The
  middleware scans the fields its author enumerated; a
  seventh field is invisible until someone updates the
  middleware, and the update is optional.
- A mechanism without a test is a mechanism whose behavior
  is whatever the implementer believed. A future change to
  the mechanism (a regex tweak, a type's method signature)
  can invalidate the belief silently; the invalidated
  behavior ships without a test failure.
- The middleware's behavior under adversarial input is
  unknown. A pathological regex can be catastrophic
  (exponential backtracking); a cyclic attribute map can
  cause infinite recursion; a megabyte-long string can
  cause a memory allocation the middleware does not
  anticipate. Without a fuzz test, these risks are
  unmeasured.
- The policy's documentation is weaker than the policy's
  intent. The intent is "secrets are redacted"; the
  documentation would have to say "secrets are redacted
  where the mechanisms happen to apply," which is not a
  useful guarantee.

### Option B — Closure by convention; the documentation states the intent, but no test enforces it

The policy's documentation asserts that every field is
covered, that every mechanism has a test, and that the
middleware is robust. The assertions are conventions that
reviewers enforce; no structural test or fuzz test verifies
them.

**Pros:**

- The documentation states the policy's intent. A reader of
  the policy doc knows what the policy is supposed to do.
- No structural tests or fuzz tests beyond the mechanisms'
  unit tests; the implementation is simpler.
- The reviewer's attention is the enforcement mechanism; the
  reviewer is expected to notice a gap.

**Cons:**

- The convention is unenforceable. A new field's coverage
  depends on a reviewer noticing the field's addition to
  ADR-025's `Record` type; a reviewer who misses the field
  passes the code, and the field escapes.
- A mechanism's test coverage depends on a reviewer noticing
  the mechanism's addition; a reviewer who misses the
  mechanism's test passes the code, and the mechanism ships
  without a test.
- The fuzz test's absence means the middleware's robustness
  is unmeasured. A pathological regex ships; the first
  user who triggers it sees a hang or a panic, and the
  policy's documentation claimed robustness.
- The convention's enforcement degrades as the codebase
  grows and the reviewers' attention is spread thinner. The
  policy's initial coverage is high; the coverage of a
  release three years from now is whatever survives the
  reviewers' attention.

### Option C — Closure by structural tests; no fuzz test

The policy's documentation asserts that every field is
covered and that every mechanism has a test. Structural tests
enforce both assertions: a test enumerates the fields of
ADR-025's `Record` type and asserts that the pattern
middleware scans each; a test enumerates the three mechanisms
(type, pattern, config) and asserts that each has a
dedicated test file. No fuzz test; the middleware's
robustness is assumed.

**Pros:**

- The structural tests enforce the field-coverage and
  mechanism-test assertions. A new field added to the record
  without updating the middleware's scan fails the
  structural test; a new mechanism added without a test file
  fails the structural test.
- The assertions are machine-checkable, so the review's
  attention is not the only enforcement mechanism.
- The structural tests are small (a few dozen lines) and
  their maintenance is proportional to the record's field
  count and the mechanism count.

**Cons:**

- The middleware's robustness under adversarial input is
  assumed. A pathological regex, a cyclic attribute map, a
  megabyte-long string — none are tested, and any of them
  could cause a hang, a panic, or a leak. The structural
  tests do not catch these.
- The middleware's behavior on edge cases (empty strings,
  very long strings, non-UTF-8 strings, strings with
  embedded nulls) is unknown. The fuzz test would explore
  these; without it, the behavior is whatever the
  implementation happens to do.
- The middleware's leak resistance on unusual inputs is
  unknown. A fuzz test would generate many inputs and
  assert that none leaks a seed; without it, the leak
  resistance is whatever the pattern set happens to catch.

### Option D — Closure by structural tests and a fuzz test

Same as Option C, plus a fuzz test that runs the middleware on
random inputs and asserts that the middleware (a) never panics
and (b) never leaks the input. The fuzz test's seeds include
known secret patterns and known-bad inputs (long strings,
cyclic-looking maps, pathological regex triggers); the fuzzing
engine mutates the seeds and explores the input space.

**Pros:**

- All of Option C's pros.
- The middleware's robustness under adversarial input is
  tested. A panic-inducing input discovered by the fuzzer
  fails the test, and the fix is applied before the input is
  seen in production.
- The middleware's behavior on edge cases is explored. The
  fuzzer generates inputs the author did not anticipate; the
  author learns about them from the test failures (or, if
  the behavior is acceptable, the test passes and the
  behavior is documented).
- The middleware's leak resistance is verified. The fuzz
  test asserts that no input leaks a seed; a leak discovered
  by the fuzzer fails the test, and the pattern set is
  extended.
- The fuzz test runs in CI with a bounded iteration count;
  the exact bound is a configuration choice (for example,
  1M iterations), and the test's duration is bounded.
  Longer runs are possible locally; the CI's run is a
  regression check, not an exhaustive exploration.

**Cons:**

- A fuzz test is a substantial implementation (a test
  driver, a corpus of seeds, a leak-detection assertion).
  The implementation is more work than Option C's
  structural tests alone.
- Fuzz tests are probabilistic; a passing run does not
  prove the absence of bugs, only the absence of bugs the
  fuzzer happened to explore. The policy's documentation
  must say so; a reader who expects a proof is misled.
- The fuzz test's runtime is a CI consideration. A 1M-
  iteration run is a few seconds of wall time; a longer
  run is longer. The bound is a tradeoff between coverage
  and CI duration.
- The fuzz test's leak-detection assertion requires the
  middleware to be callable in isolation (without the rest
  of the logger). The assertion's design is a small
  architectural constraint; the middleware's API must
  support it.

---

## 3. Decision

**Forge adopts Option D: the redaction policy is closed by
structural tests and a fuzz test.** The policy's closure
property has three parts:

1. **Field coverage.** Every field of ADR-025's `Record` type
   is covered by the pattern middleware. The middleware scans
   `msg` and every value in `attrs`; the four structural
   fields (`ts`, `level`, `component`, `correlation_id`) are
   not user data and are not scanned, but the *exclusion* is
   asserted by a structural test rather than assumed. A new
   field added to the `Record` type fails the test until the
   middleware's coverage decision for the field is recorded.

2. **Mechanism coverage.** Every mechanism (type, pattern,
   config) has a dedicated test file that exercises it in
   isolation and in combination with the others. The test
   files are `internal/logger/secret_test.go` (type),
   `internal/logger/redact_test.go` (pattern), and
   `internal/config/sensitive_test.go` (config). A new
   mechanism added to the policy fails a structural test
   until its test file exists.

3. **Robustness.** The pattern middleware is fuzz-tested:
   the fuzzer generates random inputs (records with random
   messages and attributes, seeded with known secret
   patterns and known-bad inputs) and asserts that the
   middleware (a) never panics and (b) never leaks a seed.
   The fuzz test lives in
   `internal/logger/redact_fuzz_test.go` and runs in CI with
   a bounded iteration count (a configurable constant, with
   a default of 1,000,000 iterations on the CI runner's
   budget).

The closure property is documented in
`docs/security/logging-redaction.md` § "Closure"; the
structural tests and the fuzz test are its executable form.

The reasoning that tipped the balance was the policy's
claim's strength. A policy that says "secrets are redacted"
must be able to defend the claim; a policy that says "secrets
are redacted where the mechanisms happen to apply" is a
weaker claim but an honest one. The choice between the two is
the choice between a policy that can be tested and a policy
that cannot. The closure property's three parts make the
stronger claim testable: the field coverage's assertion is
machine-checked; the mechanism coverage's assertion is
machine-checked; the robustness's assertion is fuzz-checked.
The three checks are the evidence for the claim; without
them, the claim is a belief.

The reasoning against Option A (no closure property) was the
unbounded risk. A new field added to the `Record` type can
escape the pattern middleware without notice; a mechanism
added without a test ships with unknown behavior; a
pathological regex ships without a test. The three escapes
are the class of defect the policy exists to prevent. The
closure property prevents them by making the escapes
test failures rather than review misses.

The reasoning against Option B (closure by convention) was
the convention's unenforceability. The convention's coverage
depends on the reviewers' attention, which degrades as the
codebase grows. The structural tests' enforcement is
independent of the reviewers; the tests fail when the
coverage is missing, regardless of who is reviewing.

The reasoning against Option C (structural tests without
fuzz) was the middleware's robustness. The middleware runs
on every record; a pathological input can cause a hang, a
panic, or a leak. The structural tests do not explore the
input space; they check the coverage's shape, not the
middleware's behavior. The fuzz test is the only mechanism
that explores the input space, and it is the policy's
robustness evidence.

The tradeoffs accepted are the ones Option D lists as cons.
The fuzz test is a substantial implementation; its
development is part of WBS 12.7.1's scope. The fuzz test is
probabilistic; a passing run does not prove the absence of
bugs, and the policy's documentation must say so. The fuzz
test's runtime is a CI consideration; the iteration bound is
a tradeoff between coverage and CI duration, and the bound
is configurable. The leak-detection assertion requires the
middleware to be callable in isolation, which is a small
architectural constraint on the middleware's API.

The decision is provisional in one respect: the fuzz test's
iteration bound (1,000,000) is a configuration choice, not a
contract. A future change to the CI runner's budget could
change the bound; the change is a configuration update, not
an ADR's supersession. The policy's closure property is the
ADR's subject; the bound is the implementation's choice.

---

## 4. Consequences

### 4.1 Positive

- The policy's claim ("secrets are redacted") is testable.
  The field coverage's assertion, the mechanism coverage's
  assertion, and the robustness's assertion are all
  machine-checked; a future change that invalidates any of
  them fails a test.
- A new field added to the `Record` type is caught by the
  structural test. The field's coverage decision is
  explicit (either the middleware scans it, or the field is
  documented as structural and not user data); the
  decision is recorded, not assumed.
- A new mechanism added to the policy is caught by the
  structural test. The mechanism's test file must exist;
  the mechanism's behavior is verified, not assumed.
- The middleware's behavior under adversarial input is
  explored. A panic-inducing input, a pathological regex
  trigger, a megabyte-long string — the fuzzer generates
  them, and the middleware's behavior is tested. A failure
  is a test failure, not a user-facing incident.
- The middleware's leak resistance is verified for the
  inputs the fuzzer explores. A leak discovered by the
  fuzzer fails the test; the pattern set is extended, and
  the leak is fixed before it ships.
- The policy's documentation can make the strong claim
  without hedging. The closure property's three parts are
  the claim's evidence; the documentation references the
  tests as the evidence's source.
- The closure property's shape is a precedent for future
  policies. ADR-016's error redaction policy could adopt
  a similar closure property in a future ADR; the shape is
  reusable.

### 4.2 Negative

- The fuzz test is a substantial implementation. The test
  driver, the seed corpus, and the leak-detection assertion
  are more work than the mechanisms' unit tests. The work
  is part of WBS 12.7.1's scope, and its cost is accepted.
- The fuzz test is probabilistic. A passing run does not
  prove the absence of bugs; it proves the absence of bugs
  the fuzzer happened to explore. The policy's
  documentation must say so, and a reader who expects a
  proof is misled by the policy's claim's strength.
- The fuzz test's runtime is a CI consideration. The
  iteration bound (1,000,000 by default) is a tradeoff
  between coverage and CI duration. A shorter bound
  explores less; a longer bound takes longer. The bound's
  configuration is a maintenance task.
- The leak-detection assertion requires the middleware to
  be callable in isolation. The middleware's API must
  support a single-record call (without the logger's
  surrounding machinery). The constraint is small but is
  a constraint on the middleware's design.
- The structural tests' coverage of the `Record` type's
  fields depends on the type's fields being enumerable at
  test time (via reflection or an explicit list). A
  future change to the `Record` type that makes its fields
  non-enumerable would break the structural test; the
  change would need to update the test.
- The mechanism coverage's structural test depends on the
  mechanism's test file existing at a known path. A future
  mechanism added at a different path would not be
  detected by the test; the mechanism's addition would
  need to update the test's path list.

### 4.3 Neutral

- The closure property applies to the *policy* (the three
  mechanisms and their coverage), not to the *implementation*
  (the specific regexes, the specific type signature). The
  implementation's details are the subject of WBS 12.7.1 and
  can change without superseding this ADR, provided the
  closure property's three parts continue to hold.
- The structural tests' exact form (reflection-based field
  enumeration vs. an explicit list) is a matter for WBS
  12.7.1. The tests' *assertions* are this ADR's; the tests'
  *implementation* is the WBS item's.
- The fuzz test's seed corpus is a matter for WBS 12.7.1.
  The corpus's contents (known secret patterns, known-bad
  inputs) are the WBS item's choice; the corpus's *existence*
  is this ADR's requirement.
- The fuzz test's leak-detection assertion's exact form (a
  substring search for each seed, a hash comparison, a
  set-membership check) is a matter for WBS 12.7.1. The
  assertion's *requirement* (the middleware must not leak a
  seed) is this ADR's.
- The policy's documentation
  (`docs/security/logging-redaction.md`) is WBS 12.7.1's
  output; its "Closure" section is the authoritative
  statement of this ADR's details.

---

## 5. Related Requirements

- `SEC-LOG-001` through `SEC-LOG-010` — the security
  requirements for logging; the requirements are established
  by ADR-033 and reinforced by this ADR's closure property.
- `SEC-LOG-010` — the fuzz test requirement; the requirement
  is established by this ADR (with WBS 12.7.1's
  implementation).
- `NFR-OBSERVABILITY-001` — records are attributable to a
  level and a source; the requirement is compatible with
  this ADR's closure (the structural fields are not user
  data and are not scanned).
- `NFR-ARCHITECTURE-003` — the logger's interface is
  independent of CLI and domain packages; the requirement
  is compatible with this ADR's closure (the mechanisms are
  in `internal/logger`, not in a CLI package).

> The following requirements must be added to the register
> when WBS 12.7.1 is complete:

- `SEC-LOG-010` (if not already present)
- A new requirement for "the redaction policy's closure property" if the existing requirements do not cover it.

---

## 6. Related ADRs

- `ADR-016` — Error Redaction Policy. Establishes the error
  system's redaction policy; this ADR's closure property is
  a shape that the error system's policy could adopt in a
  future ADR.
- `ADR-025` — Log records are structured with a
  schema-versioned JSON form and an unstable human form.
  Establishes the `Record` type whose fields this ADR's
  closure property quantifies over.
- `ADR-026` — The Logger interface has four level methods
  and a With combinator. Establishes the `Sink` interface
  whose implementation the pattern middleware wraps and
  which the fuzz test exercises in isolation.
- `ADR-033` — Sensitive data is redacted at three layers:
  type, pattern, and config. Establishes the three
  mechanisms this ADR's closure property closes over. The
  two ADRs are complementary: ADR-033 defines the
  mechanisms, ADR-034 defines the guarantee.

---

## 7. Notes

**Open questions deferred by this ADR:**

- The exact form of the structural tests' field
  enumeration. The tests could use reflection to enumerate
  the `Record` type's fields, or an explicit list of field
  names. The reflection-based approach is more robust to
  field additions (the test fails automatically when a
  field is added); the explicit list is simpler and easier
  to read. The choice is WBS 12.7.1's.
- The exact form of the fuzz test's leak-detection
  assertion. The assertion could search for each seed as a
  substring, or compare hashes, or check set membership.
  The choice is WBS 12.7.1's; the assertion's requirement
  (the middleware must not leak a seed) is this ADR's.
- The fuzz test's iteration bound. The default of
  1,000,000 is a starting point; the bound's final value
  depends on the CI runner's budget and the middleware's
  per-iteration cost. The choice is WBS 12.7.1's.
- Whether the closure property should apply to the *error
  system's* redaction policy as well (per ADR-016). The
  current ADR's closure property is scoped to the logger's
  policy; a future ADR could extend the shape to the
  error system's policy.

**Follow-up decisions anticipated:**

- WBS 12.7.1's implementation is the direct consequence of
  this ADR; the WBS item's outputs are the three test
  files (`secret_test.go`, `redact_test.go`,
  `sensitive_test.go`), the fuzz test
  (`redact_fuzz_test.go`), and the documentation's
  "Closure" section.
- A future ADR could extend the closure property to the
  error system's policy; the extension would be a new ADR
  that cites this one.
- A future ADR could refine the structural tests' form or
  the fuzz test's assertion; the refinements are
  implementation details that do not supersede this ADR,
  provided the closure property's three parts continue to
  hold.

**External references:**

- Go standard library `testing`'s fuzzing support
  (`testing.F`) — the mechanism the fuzz test uses.
- Go standard library `reflect` — the mechanism the
  structural test may use to enumerate the `Record` type's
  fields.
- OWASP Logging Cheat Sheet — the guidance that informs
  the policy's substance.
- `docs/security/logging-redaction.md` (expected) — the
  authoritative reference for this ADR's details,
  including the "Closure" section.
- `docs/security-model.md` — the CLI's broader security
  model, of which this ADR's closure property is a part.

---

*End of ADR-034.*
