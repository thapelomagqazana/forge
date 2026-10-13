# ADR-026: The Logger interface has four level methods and a With combinator; the implementation is slog-backed and hidden

**Status:** Accepted
**Date:** 2026-10-13
**Deciders:** [@thapelomagqazana] (Architecture Owner)
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

ADR-022 placed diagnostics on stderr and named `internal/logger` as the package that produces them. ADR-023 fixed the four levels. ADR-024 fixed the precedence chain that selects a level. ADR-025 fixed the record's shape and its two renderings. None of these decisions defined the *programming interface* a caller uses to emit a record. The current `internal/logger` package has a `Logger` interface and an implementation (`slogLogger`) backed by Go's `log/slog`, but the interface's method set, the meaning of "child logger", the no-op variant, and the rules for what a caller must and must not depend on are not written down. The package works today; a future contributor who wants to add a method, swap the implementation, or introduce a second implementation (for example, a test spy) has no rule to follow.

The forces at play are three. First, the interface is a *contract* with every package in the repository. A caller in `internal/cli`, `internal/config`, or `internal/app/version` uses the interface's methods; if the interface changes, every caller changes. The interface's method set must therefore be chosen to cover the callers' needs without over-committing to a shape that a future caller cannot extend. Second, the implementation is *pluggable*. The default is `log/slog`, but a test that wants to capture records as structured data (rather than as bytes on stderr) needs a different destination. The interface must be usable with multiple implementations without every caller knowing which one is in use. Third, the context propagation path is unspecified. A caller that emits a record inside a command's `RunE` has access to the command's context; a caller that emits a record deep inside a helper function does not. The interface must let a logger travel through a call chain without being threaded through every function's signature.

The existing package's design already addresses all three forces — the interface is small, the implementation is hidden behind it, and a context-based accessor exists — but the design is documented only in the package's docstring and in the individual functions' docstrings, not as a decision with alternatives considered. A future contributor who wants to add a fifth level method (for example, a `Fatal` method per ADR-023's rejection), or to expose the concrete `slogLogger` type "for convenience", would have no ADR to consult for the reasoning that led to the current shape.

The decision relates to ADR-022 (the stream), ADR-023 (the levels), ADR-024 (the resolution that filters records), ADR-025 (the record shape), ADR-030 and ADR-031 (the flags whose semantics depend on the interface's behavior), and ADR-033 and ADR-034 (redaction, which will wrap the interface). The CLI UX spec's § 4.12 mentions the `--verbose` flag's effect but does not name the interface. If no decision is made, the interface's shape remains a matter of code review, and a future contributor's "small addition" (a method, a type assertion) can break callers in ways the ADR system is designed to prevent.

---

## 2. Options Considered

### Option A — Expose `*slog.Logger` directly

Do not define an interface. Every package imports `log/slog` and receives a `*slog.Logger` in its constructor or reads one from the context. The default logger is constructed with `slog.New(slog.NewTextHandler(os.Stderr, ...))` at process start and stored in the context.

**Pros:**

- No interface to define or maintain. `*slog.Logger` is a well-documented type with a large user base.
- Callers use `slog`'s API directly, which they may already know from other Go programs.
- No adapter layer between Forge's callers and `slog`; the call chain is shorter.
- Structured attributes are `slog.Attr`, a type that supports typed values (int, float, bool, time, duration) with dedicated constructors.

**Cons:**

- Every package in the repository depends on `log/slog`. A future change to the logger's behavior (a different sink, a different record shape, an interceptor for redaction) requires changing every package's imports, or replacing `*slog.Logger` with a different type across the codebase.
- The record's shape is `slog.Record`'s shape, which is not the shape ADR-025 defines. The `correlation_id` field has no natural home in `slog.Attr` (it is a record-level attribute, not an attribute of any single call). Adapting it requires every caller to add it explicitly, or a handler wrapper that injects it — a workaround for a missing abstraction.
- The `slog` package is a direct dependency that leaks into every caller's import list. A future decision to stop using `slog` (for example, to write a custom encoder for the JSON form's exact schema) requires touching every package.
- A test that wants to capture records as structured data must implement a `slog.Handler` (which has a `Handle(ctx, slog.Record) error` method), which is a larger surface than a one-method interface.
- The no-op logger must be implemented as a `slog.Handler` that discards records, or as a `*slog.Logger` with a nil handler and defensive checks throughout.

### Option B — Define a `Logger` interface with four level methods and a `With` combinator

Define a small interface:

```go
type Logger interface {
    Debug(msg string, args ...any)
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
    With(args ...any) Logger
}
```

Provide a default implementation backed by `log/slog`, a no-op implementation for tests and disabled paths, and context helpers (`FromContext`, `IntoContext`) so loggers travel through call chains. The concrete implementation is unexported; callers depend only on the interface.

**Pros:**

- The interface is small: four level methods plus one combinator. A caller who learns the interface learns five methods and nothing else.
- Every package depends on `internal/logger`, not on `log/slog`. A future change to the implementation touches one package; a future change to a caller's imports is a rename, not a rewrite.
- The implementation is hidden. A future replacement of `slog` (for example, a custom encoder that produces ADR-025's exact JSON schema) does not affect a single caller.
- A test that wants to capture records as structured data implements a one-method `Sink` interface (`Emit(ctx, Record)`), which is smaller than `slog.Handler`.
- The no-op logger is a simple implementation of the same interface; it satisfies the interface's contract by doing nothing and returning the receiver from `With`.
- The `correlation_id` field has a natural home: the `Record` struct that the `Sink` receives. The logger's `With` can inject fields that become part of every record's `attrs`.
- The interface is documented as a public contract; adding a method requires an ADR, which is the discipline the repository's ADR system provides for other public interfaces (the error system, the exit-code taxonomy, the flag inventory).

**Cons:**

- The interface is a new abstraction; a contributor who knows `slog` must learn the interface's five methods.
- The structured attributes are `...any` (key-value pairs) rather than `slog.Attr`; typed values are passed as their Go types and serialized by the implementation. A caller who wants `slog.Attr`'s typed constructors (for example, `slog.Duration("elapsed", d)`) does not have them; the caller passes `"elapsed", d` and the implementation formats the duration.
- The interface's `With` combinator's semantics must be defined (what a child logger shares with its parent, what it overrides); the definition is a contract that the implementation must honor and that a future replacement must honor.
- The implementation must bridge `slog`'s API and the interface's API: the default adapter converts `(msg, args)` to `slog.Record` and back. This is a small amount of code, but it is code that would not exist if the interface were `*slog.Logger` itself.

### Option C — Expose `*slog.Logger` as the interface, with a Forge-specific handler

Define no Forge interface. Every package receives a `*slog.Logger`; the logger's behavior is customized by a Forge-provided `slog.Handler` that injects the `correlation_id`, enforces the record's schema, and routes to the correct encoding. Callers use `slog`'s API directly.

**Pros:**

- No interface to define. `*slog.Logger` is a well-documented type.
- The Forge-specific behavior (correlation ID injection, schema enforcement) is encapsulated in the handler, not spread across callers.
- Structured attributes use `slog.Attr`'s typed constructors, which is the idiomatic `slog` API.
- A test that wants to capture records as structured data implements a `slog.Handler`, which is a well-understood shape.

**Cons:**

- Every package depends on `log/slog` in its imports. A future change to a custom encoder (for example, one that produces ADR-025's exact JSON schema with the `attrs` object's omitted-when-empty rule) requires either a handler wrapper or a change to every package.
- The handler's API is `Handle(ctx, slog.Record) error`, which returns an error. Every caller that wants to ignore the error (which is the correct behavior per ADR-022: a failed write to stderr is not actionable) must be adapted, or the handler must drop errors internally — a convention that is not enforced by the type system.
- The `log/slog` package's record shape is not ADR-025's shape. The `correlation_id` is a record-level attribute in `slog`'s model (it belongs to the record, not to any single call); injecting it requires the handler to know which records belong to which invocation, which requires the handler to read the context, which `slog.Handler.Handle` receives. This is workable but is a nontrivial amount of handler code.
- The interface's contract (`Logger`'s method set) is `*slog.Logger`'s method set, which is larger than the five methods Forge's callers use. A caller can call methods (`Enabled`, `Handler`, `LogAttrs`, `Log`, `WithGroup`) that Forge does not intend to support; a future change to `slog` that removes or renames one is a breaking change for callers who use it.
- The discipline "adding a method requires an ADR" is lost; the interface is `slog`'s, and `slog`'s evolution is outside Forge's control.

### Option D — Define a `Logger` interface with four level methods and a `With` combinator, with a `Sink` interface for the destination

Same as Option B, but the destination is defined by a `Sink` interface (`Emit(ctx, Record)`) that the logger is constructed with. The default `Sink` writes to stderr; a test's `Sink` captures records as structured data. The `Logger` interface remains the caller-facing contract; the `Sink` interface is the implementation-facing contract.

**Pros:**

- The caller-facing interface (Option B's) is unchanged: four level methods plus `With`.
- The implementation-facing interface is one method (`Emit`), which is smaller than `slog.Handler`'s two methods (`Enabled`, `Handle`).
- The `Record` type (ADR-025's shape) is the currency of the `Sink` interface; a `Sink` receives exactly the fields the JSON rendering will serialize, with no adaptation.
- The no-op logger is a `Logger` implementation whose methods do nothing; the no-op `Sink` is a `Sink` implementation whose `Emit` does nothing. Tests can use either, depending on whether they want to observe records.
- A test that wants to capture records as structured data implements `Sink`, a one-method interface, and inspects the `Record` values. A test that wants to observe the *call* (whether a method was invoked) implements `Logger` directly.
- The `Sink` interface's one method is easy to mock, easy to test, and easy to document.
- The `Logger` and `Sink` are separate concerns: the `Logger` decides *whether* to emit a record (level filtering); the `Sink` decides *where* the record goes. Separating them keeps each interface's responsibility small.

**Cons:**

- Two interfaces to define and maintain. The surface is slightly larger than Option B's single interface.
- A caller that wants to write a custom `Logger` must implement the four level methods plus `With`; a caller that wants to write a custom `Sink` must implement one method. The two audiences are distinct, but a new contributor must learn which interface they want.
- The `Record` type is exposed by the `Sink` interface, which means the `Record` type's shape is part of the logger package's public surface. ADR-025 already fixes the record's shape, so this is not a new commitment; the exposure is the consequence of ADR-025's decision, not this ADR's.

---

## 3. Decision

**Forge adopts Option D: a `Logger` interface with four level methods (`Debug`, `Info`, `Warn`, `Error`) and a `With(args ...any) Logger` combinator, plus a `Sink` interface with one method (`Emit(ctx context.Context, r Record)`).** The default `Logger` is backed by `log/slog` and writes to a `Sink` that defaults to a stderr writer. The `Logger` and `Sink` interfaces are the package's public contract; the concrete implementations (`slogLogger`, `StderrSink`) are unexported. Context helpers (`FromContext`, `IntoContext`) let a logger travel through a call chain without being threaded through every function's signature. A no-op implementation (`Nop`) exists for tests and for code paths where logging is disabled. The interface has zero imports from `internal/cli` or any domain package; the dependency direction is `internal/cli → internal/logger`, never the reverse.

The reasoning that tipped the balance was the separation of the *caller-facing* concern from the *implementation-facing* concern. The `Logger` interface's five methods are what a caller needs: emit a record at a level, or emit a record with prepended attributes. The `Sink` interface's one method is what a destination needs: receive a record. Splitting the two into separate interfaces means each interface's contract is small, each interface's implementers have a clear responsibility, and each interface's test surface is minimal. Option A (`*slog.Logger` directly) and Option C (`*slog.Logger` with a Forge handler) both conflate the two concerns: they make the caller depend on `slog`, which is the implementation, and they make the destination's contract (`slog.Handler`) larger than it needs to be. Option B (a single `Logger` interface with an internal `Sink` field) is close to Option D but hides the `Sink` behind the `Logger`'s constructor, which makes the destination's contract invisible to a test that wants to capture records — a `Logger` implementation is needed to observe records, which is a larger surface than a `Sink` implementation. Option D's explicit separation is the smallest pair of contracts that satisfies both audiences.

The reasoning against a fifth method (a `Fatal` or a `Trace`) is ADR-023's: adding a level requires an ADR that names the subsystem that needs it. A fifth method on the `Logger` interface would violate the "adding a method requires an ADR" rule that this ADR establishes; a future ADR that adds a level adds a method, and this ADR is superseded. The current four-method interface is the correct size for the current four levels, and the size is a consequence of ADR-023's vocabulary, not of this ADR's preference.

The reasoning against exposing `*slog.Logger`'s full method set is the discipline of a public contract. `slog.Logger` has methods (`Enabled`, `Handler`, `LogAttrs`, `Log`, `WithGroup`) that Forge's callers do not use, and each of them is a surface that a future change to `slog` could break. The five-method `Logger` interface is the smallest surface that covers the callers' needs; a caller that needs more (for example, `Enabled` to check whether a level is filtered before constructing an expensive attribute) can call `logger.Enabled(level)` on the concrete type, which is available through the package's `Default()` accessor or through a type assertion on a value the caller constructed. The interface's smallness is the point.

The tradeoffs accepted are the ones Option D lists as cons. Two interfaces instead of one is the cost of separating the caller-facing and implementation-facing concerns; the separation's benefit is a smaller surface for each audience. The `Record` type's exposure by the `Sink` interface is a commitment that ADR-025 already made; the exposure is consistent with that decision. The `...any` attribute convention (rather than `slog.Attr`'s typed constructors) is a consequence of the interface being independent of `slog`; a caller who wants typed attributes passes the Go value and the implementation serializes it. The serialization's behavior (a `time.Duration` becomes a string like `"1.5s"`; a `time.Time` becomes an RFC 3339 string) is the implementation's responsibility, and the reference doc (`docs/reference/log-record-format.md`) documents the conventions.

The decision is provisional in one respect: the `With` combinator's exact semantics ("a child shares the parent's level, format, component, and sink; it prepends its args to every record") is documented in the interface's docstring and is tested, but a future ADR could refine the semantics (for example, to allow a child to override the component). Until then, the semantics are the ones the current implementation provides, and the tests pin them.

---

## 4. Consequences

### 4.1 Positive

- The caller-facing interface is small: five methods. A contributor who learns `Logger` learns five methods and nothing else; the implementation's details are hidden.
- The implementation-facing interface is smaller: one method. A test that wants to capture records as structured data implements `Sink`, which is a smaller surface than `slog.Handler`.
- Every package depends on `internal/logger`, not on `log/slog`. A future replacement of `slog` (for example, a custom encoder that produces ADR-025's exact JSON schema) touches one package; the callers are unaffected.
- The `Record` type (ADR-025's shape) is the currency of the `Sink` interface. A `Sink` receives exactly the fields the JSON rendering will serialize; no adaptation is required.
- The no-op implementation (`Nop`) is a `Logger` whose methods do nothing; it is safe for tests, for optional logger fields, and for code paths where logging is disabled.
- Context helpers (`FromContext`, `IntoContext`) let a logger travel through a call chain without being threaded through every function's signature. A helper function deep in a call stack can call `logger.FromContext(ctx).Debug(...)` and emit a record without knowing which invocation it is part of; the context carries the correlation ID.
- The interface's docstring can state the "adding a method requires an ADR" rule explicitly, which prevents a future contributor's "small addition" from being an unnoticed contract change.
- The dependency direction (`internal/cli → internal/logger`) is enforced by the interface's lack of imports; a future change that tried to import `internal/cli` from `internal/logger` would fail to compile.

### 4.2 Negative

- Two interfaces to define and maintain. The surface is slightly larger than a single interface's would be; the benefit (smaller contracts for each audience) is worth the cost.
- A contributor who knows `slog` must learn Forge's interface's five methods and its `Record` type. The learning cost is small but is not zero.
- Structured attributes use `...any` (key-value pairs) rather than `slog.Attr`'s typed constructors. A caller who wants a typed duration passes `"elapsed", time.Since(start)` and the implementation formats it; the caller cannot use `slog.Duration`. The tradeoff is the interface's independence from `slog`; a future ADR could add typed attribute helpers to the `Logger` interface, but the current surface is the minimum.
- The `With` combinator's semantics are a contract that a future implementation must honor. A new implementation that shares fewer fields than the current one (for example, one that does not share the sink) would violate the contract; the tests pin the semantics.
- The `Record` type's exposure by the `Sink` interface means the `Record` type is part of the logger package's public surface. ADR-025 already fixed the record's shape; the exposure is consistent with that decision, but it means a future change to the `Record` type is a change to this interface's contract.
- The `Logger` interface's method set is a public contract; adding a method requires an ADR. This is the correct discipline, but it means a future contributor who wants a new method must write an ADR, which is a higher bar than a code change.

### 4.3 Neutral

- The `Logger` interface's `With` combinator returns `Logger`, not a concrete type. A caller cannot type-assert the result to `*slogLogger`; the interface hides the implementation. This is by design.
- The `Sink` interface's `Emit` method takes a `context.Context`. A `Sink` implementation that does not care about cancellation can ignore the parameter. The context is present so that a future implementation (for example, one that writes to a network destination) can honor cancellation.
- The `StderrSink` type's mutex serializes concurrent `Emit` calls. A `Sink` implementation that writes to a thread-safe destination (for example, a `bytes.Buffer` used single-threaded in a test) can omit the mutex; the interface does not require it.
- The package's `Default()` accessor returns the current package-level logger, which is a `Nop()` until `SetDefault` is called. The default is deliberately not a live logger; a package that logs through the default without the CLI having installed one writes nothing.
- The context helpers (`FromContext`, `IntoContext`) are documented in the package's docstring. Their behavior when the context does not carry a logger (`FromContext` returns `Nop()`) is a decision that a future ADR could revisit if a different default is needed.

---

## 5. Related Requirements

- `FR-CLI-LOG-006` — the `Logger` interface's method set; the requirement is established by this decision.
- `FR-CLI-LOG-007` — domain packages emit diagnostics only through the `Logger` interface; the requirement depends on this decision's interface.
- `FR-CLI-LOG-009` — `--verbose` elevates the level; the requirement's behavior depends on the interface's `Debug` method.
- `FR-CLI-LOG-010` — `--quiet` suppresses diagnostics; the requirement's behavior depends on the interface's level-filtering semantics.
- `NFR-ARCHITECTURE-003` — the logger's interface is independent of CLI and domain packages; the requirement is established by this decision's import discipline.
- `NFR-OBSERVABILITY-001` — records are attributable to a level and a source; the requirement is constrained by this decision's interface and by ADR-025's record shape.

> The following requirements must be added to the register when
> WBS 12.3.1 is complete:

- `FR-CLI-LOG-006` (if not already present)
- `NFR-ARCHITECTURE-003` (if not already present)

---

## 6. Related ADRs

- `ADR-022` — Output and logging are separate architectural layers. Establishes the stream the interface's records go to.
- `ADR-023` — Four log levels: ERROR, WARN, INFO, DEBUG. Establishes the level vocabulary the interface's four methods correspond to.
- `ADR-024` — Log level precedence: CLI flag > env var > config file > default. Establishes the resolution that determines which records the interface's methods emit (the level filter).
- `ADR-025` — Log records are structured with a schema-versioned JSON form and an unstable human form. Establishes the record shape the `Sink` interface receives.
- `ADR-033` (expected) — Sensitive data is redacted at three layers. Will define the redaction rules that wrap the `Sink` interface's output.
- `ADR-034` (expected) — The redaction policy is closed. Will define the closure property that applies to every field the `Record` type declares.

---

## 7. Notes

**Open questions deferred by this ADR:**

- Whether the `Logger` interface should have an `Enabled(level Level) bool` method. A caller that wants to check whether a level is filtered before constructing an expensive attribute (for example, a serialized request payload) would benefit from the method. The current interface does not have it; a caller can call the package-level `Enabled` function or type-assert to the concrete type. A future ADR could add the method if a use case emerges.
- Whether the `With` combinator's semantics should allow a child to override the component. The current semantics share the parent's component; a caller that wants a different component constructs a new logger. A future ADR could refine the semantics.
- Whether the `Sink` interface's `Emit` method should return an error. The current signature returns nothing; a write error is not actionable (per ADR-022's rationale for stderr). A future implementation (for example, one that writes to a network destination) might want to observe write errors; a future ADR could add an error return or a separate `Flush` method.

**Follow-up decisions anticipated:**

- The `FromContext` and `IntoContext` helpers' exact behavior (whether `FromContext` returns `Nop()` or a live logger when the context does not carry one) is documented in the package; a future ADR could revisit it if a use case for a different default emerges.
- The redaction rules (ADR-033, ADR-034) wrap the `Sink` interface's output; their design is separate from this ADR's interface but must respect the `Record` type's shape.
- A future `forge logs` command that reads the CLI's own JSON log output and renders it for humans is a plausible feature; its design would use the `Record` type (via the `Sink` interface) as its input, but the command itself is not this ADR's subject.

**External references:**

- Go standard library `log/slog` — the default implementation's backing library.
- Go standard library `context` — the mechanism for propagating the logger through call chains.
- `io.Writer` — the `Sink` interface's precedent: a small, single-method interface whose implementations include the standard library's buffered writers, network writers, and test buffers.
- `docs/reference/log-record-format.md` — the authoritative reference for the record shape the `Sink` interface receives.

---

*End of ADR-026.*
