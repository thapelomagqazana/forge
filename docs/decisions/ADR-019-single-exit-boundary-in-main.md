# ADR-019: Single Exit Boundary in main

**Status:** Accepted
**Date:** 2026-10-12
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

The Forge CLI's process exit is a process-level concern: the value a caller's shell observes as `$?` and that a CI runner uses to decide whether a job failed. Every command's behaviour — its output, its side effects, its error message — funnels into exactly one observable result at the process boundary: the integer passed to `os.Exit`. Before WBS 11.3.1, that funnel was under-specified. The CLI's `Execute` function returned an `int` and `main.go` called `os.Exit(cli.Execute())`, but the split between "who computes the code" and "who terminates the process" was not enforced. A future command that wanted to terminate early could have called `os.Exit` directly; a future wrapper that wanted to inspect the error before exiting had no way to do so; a signal-handling change had no obvious place to install its handler. The arrangement was correct by accident, not by design.

WBS 11.3.1 asked for a single, well-defined exit boundary. The requirements were: exactly one `os.Exit` call in the repository; Cobra configured to not print or exit on its own; a public entry point that returns the exit code and the error rather than terminating; signal handling at the boundary that maps SIGINT to 130 and SIGTERM to 143; a documented flow.

The decision is architecturally significant for four reasons. First, it is the point at which the CLI's internal error taxonomy becomes a value the operating system observes; the mapping from `Category.ExitCode()` to `os.Exit` happens here and nowhere else. Second, it is the point at which signals enter the CLI: the handler's lifetime must match the process's, and only `main` can guarantee that. Third, it is the seam for embedding Forge as a library: a future caller that wants to run the CLI in-process reads the returned error instead of the process's exit status. Fourth, it is the point at which the CI check `scripts/deps/verify-no-os-exit.sh` anchors its rule: any new `os.Exit` outside `main.go` fails CI.

The users of this decision are the CLI's consumers (scripts that branch on `$?`), the CLI's maintainers (who need to know where to add signal handling or how to embed the CLI), and the CLI's reviewers (who need a single file to inspect to know how the process terminates).

---

## 2. Options Considered

### Option A — Leave the exit boundary implicit (status quo)

Keep `Execute() int` as the public entry point and `os.Exit(cli.Execute())` in `main.go`. Do not add a second entry point, do not install a signal handler, do not enforce the "one `os.Exit`" rule.

**Pros:**

- Zero migration. No public API changes.
- The current `Execute` signature (frozen by WBS 4.1.1) is preserved without a wrapper.
- No new mechanism to test or document.

**Cons:**

- The boundary is not enforceable. A future contributor can add `os.Exit` inside a command without violating any test; the current tree does not forbid it.
- Signal handling has no home. The handler must be installed somewhere, and every candidate (main, Execute, a command's RunE) has a lifetime problem: the signal must survive the command tree's execution, and a command's lifetime is too short.
- There is no seam for embedding. A caller that wants the error before the process exits cannot get it: `Execute` discards the error after deriving the code.
- The "one `os.Exit`" invariant cannot be stated as a rule because the tree has no explicit boundary. The CI check would have to guess which file is the boundary.

### Option B — Single exit boundary with `Run(ctx) (int, error)` and signal handling in `main`

Introduce `Run(ctx context.Context) (int, error)` as the process boundary. `Run` parses the arguments, dispatches to the matching command, renders any error, and returns the process exit code and the error. `main` installs a signal-aware context via `signal.NotifyContext`, calls `Run`, and calls `os.Exit` exactly once with the returned code. `Execute() int` is preserved as a thin wrapper that calls `Run(context.Background())` and discards the error.

Cobra is configured with `SilenceErrors: true` and `SilenceUsage: true` so that it does not print or exit on its own; the boundary renders the error exactly once through `forgeerr.Format`. The "one `os.Exit`" rule is enforced by the CI check `scripts/deps/verify-no-os-exit.sh`, which fails if `os.Exit` appears outside `cmd/forge/main.go`.

**Pros:**

- The boundary is explicit and enforceable. One file is named as the boundary; the CI check anchors its rule there.
- The two-value return is the seam for embedding. A future library caller reads the error; `main` reads only the code.
- Signal handling has a home. `main` installs the handler; the context is threaded through the command tree; the boundary inspects the context's `Cause` to decide which signal fired.
- Cobra's `SilenceErrors` and `SilenceUsage` give the boundary sole ownership of the error rendering. The user sees one error block, not two.
- `Execute` is preserved, so existing callers (and tests) are unaffected.

**Cons:**

- Two public entry points (`Execute` and `Run`) for a package that historically had one. The extra surface is small (one function, one type parameter, one extra return), but it is a surface addition.
- The signal-handling code in `main` requires importing `context`, `os/signal`, and `syscall`, which grows `main.go`'s import set from two packages to five.
- The signal-to-code mapping requires inspecting the context's `Cause`, which is a subtlety: the standard library wraps the signal in an unexported type, so a direct `cause == syscall.SIGTERM` comparison fails and a `String()` comparison is needed. The subtlety is documented in `signalExitCode`'s docstring and pinned by tests, but it is a non-obvious piece of code.
- The compatibility wrapper `Execute` accumulates as a permanent deprecation candidate. A future ADR could remove it, but the removal would break any caller that uses it today (the tree has one: the tests).

### Option C — Single exit boundary with `Run(ctx) (int, error)`, but signals handled in the boundary

Same as Option B, but move the `signal.NotifyContext` install from `main` into `Run`. `main` becomes `os.Exit(cli.Run(context.Background()))` or similar; the boundary installs the handler.

**Pros:**

- `main` stays small (two imports, one call).
- The signal handling is testable in-process: a test can call `Run` and simulate a signal via a context that the boundary installs and the test cancels.

**Cons:**

- The signal handler's lifetime becomes the boundary's lifetime. When `Run` returns, the handler is unregistered. For a CLI whose process is the boundary's lifetime, the two are equivalent; for a library caller that embeds `Run`, the handler's lifetime would be the library call's, which is correct for the library use case but wrong for the process use case.
- The testability benefit is real but partial: a test that calls `Run` and wants to simulate a signal must either cancel the context the boundary installed (which requires the boundary to expose it) or send a real signal (which is what the integration test does). Neither is as clean as the boundary taking the caller's context.
- The pattern "the caller owns the context" is the Go convention: functions that accept a `context.Context` treat the context as the caller's, not as something they construct. `Run` accepting a context and installing its own handler violates the convention.

### Option D — Single exit boundary with `Run(ctx) (int, error)`, signals handled in each command

Same as Option B, but each command installs its own signal handler rather than `main` installing a shared one.

**Pros:**

- Each command has full control over its signal response.
- A command that wants a different response for SIGINT (for example, to save partial state) can implement it without affecting other commands.

**Cons:**

- Signal handling is a process-level concern, not a command-level one. The code the process returns on a signal is a property of the process, not of whichever command happens to be running.
- Every command's implementation duplicates the signal setup. A command that forgets to install the handler runs unresponsive to Ctrl-C; the failure is silent.
- The mapping from signal to code (SIGINT → 130, SIGTERM → 143) is duplicated in every command, which is exactly the kind of policy that should have one home.

---

## 3. Decision

Adopt Option B: the single exit boundary with `Run(ctx context.Context) (int, error)` as the process boundary, `main` as the only `os.Exit` caller, and signal handling installed in `main` via `signal.NotifyContext`.

The decision was made because three requirements pointed at the same shape:

1. **The boundary must be enforceable.** Option A leaves the "one `os.Exit`" rule unenforceable. Option B's CI check needs a named file; `cmd/forge/main.go` is that file.
2. **The error must be inspectable before termination.** Option A's `Execute` discards the error. Option B's two-value return preserves it for any caller that wants it.
3. **The signal handler's lifetime must match the process's.** Option C's boundary-scoped handler is correct for a library caller but wrong for the process use case; Option D duplicates the handler per command. Only Option B's `main`-scoped handler satisfies both the process use case and the library use case (a library caller passes its own context; the library does not install a handler).

The trade-offs accepted in choosing Option B over the alternatives:

- **Two public entry points.** Option B adds `Run` alongside `Execute`. Option A has one; Option B has two. The addition is small and the compatibility wrapper's value (existing callers unaffected) is real, but the surface is larger than it was.
- **The signal handler's shape.** The standard library's `signal.NotifyContext` wraps the signal in an unexported type. The comparison against `syscall.SIGTERM` must go through the signal's `String()` representation rather than the concrete type. This subtlety is documented and tested but is the kind of thing a future reader might remove without realizing why it is there.
- **The signal-handling code in `main`.** `main.go` grows from two imports to five and from three lines of code to about fifteen. The growth is the minimum the process-level concern requires, but `main` is no longer a strictly trivial file.

Two assumptions the decision depends on:

- **The `os.Exit` in `main` is the process's only termination.** If a future WBS introduces an `os.Exit` outside the boundary (for example, in a `panic` recovery path that bypasses the normal error handling), the CI check catches it, but the boundary's exclusivity is a property the tree maintains, not a property the language enforces.
- **The signal handler's shape is stable.** A future Go release that changes how `signal.NotifyContext` wraps the signal could break `signalExitCode`'s `String()` comparison. The test `TestRun_ContextCancelled_ReturnsSIGTERM` exercises the wrapper shape; its failure is the signal that the comparison needs updating.

The decision is not provisional. The boundary's shape is a public contract (the `Run` signature) and a CI-enforced rule (the "one `os.Exit`" invariant). A change to either requires a new ADR.

---

## 4. Consequences

### 4.1 Positive

- **The boundary is one file.** A reader who wants to know how Forge terminates reads `cmd/forge/main.go`. The file is about fifteen lines of code plus its docstring. The concern is localized.
- **The "one `os.Exit`" rule is enforceable.** The CI check `scripts/deps/verify-no-os-exit.sh` fails if `os.Exit` appears outside `cmd/forge/main.go`. The rule is not aspirational; it is a check.
- **The error is inspectable.** `Run` returns `(int, error)`. A future caller that embeds Forge reads the error instead of the process's exit status. The embedding seam exists today, even though no caller uses it.
- **`Execute` is preserved.** The WBS 4.1.1 signature (`func Execute() int`) is unchanged. Existing callers and tests are unaffected. The compatibility wrapper is two lines.
- **Signal handling is a single install.** The `signal.NotifyContext` install is in `main`, its lifetime matches the process's, and its cancellation propagates through the command tree via the context. Each command that respects the context returns `ctx.Err()`; the boundary inspects the context's `Cause` and returns the correct code.
- **Cobra's printing is silenced.** `SilenceErrors: true` and `SilenceUsage: true` make the boundary the sole renderer of errors. The user sees one error block, not two. The error block's format is the one `forgeerr.Format` produces, which is the frozen user-facing format.
- **The tests can exercise the boundary in-process.** The helper `runWithOptions(ctx, opts)` is callable from tests with synthetic options and a synthetic context. The context-cancellation tests (`TestRun_ContextCancelled_ReturnsSIGINT`, `TestRun_ContextCancelled_ReturnsSIGTERM`) exercise the signal mapping without spawning a subprocess.

### 4.2 Negative

- **Two public entry points.** `Run` and `Execute` coexist. A caller who wants the error calls `Run`; a caller who wants only the code calls `Execute`. The duplication is small but real. A future ADR could remove `Execute` (a breaking change for its callers), but until then the surface includes both.
- **The signal handler's comparison is subtle.** The `signalExitCode` function inspects `context.Cause(ctx)` and must compare the cause against `syscall.SIGTERM` through the cause's `String()` representation rather than through a direct `==`. The reason is that `signal.NotifyContext` wraps the signal in an unexported type; the wrapper's dynamic type is not `syscall.Signal`. A future reader who "simplifies" the comparison to a direct `==` breaks SIGTERM handling; the test `TestRun_ContextCancelled_ReturnsSIGTERM` catches the break, but the break is easy to introduce.
- **`main.go` grows.** The file's imports grow from `os` and `internal/cli` to `context`, `os`, `os/signal`, `syscall`, and `internal/cli`. The file's code grows from one line to about fifteen. The growth is the minimum the process-level concern requires, but the file is no longer trivially inspectable.
- **The `Run` signature is frozen.** The signature `func Run(ctx context.Context) (int, error)` is now a public contract. A future change that wants a different return shape (for example, a struct that carries more than the code and the error) requires a new ADR and a new entry point.
- **The signal-to-code mapping is a policy.** `SIGINT → 130` and `SIGTERM → 143` are policies, not mechanics. The values follow POSIX (`128 + N`), but a future change to the mapping (for example, to distinguish Ctrl-C from a programmatic SIGINT) requires a new ADR. The mapping is small enough that it does not need its own ADR today, but it is a decision the boundary encodes.
- **The panic path writes directly to `os.Stderr`.** `Execute`'s deferred recovery writes the panic's rendered error to `os.Stderr` directly, bypassing the injected stderr. The exception is documented; it exists because a panic means the injected writers are not trustworthy. A future WBS that wants to route the panic path through the injection layer must construct the `options` value before the `defer`, which is possible but adds complexity for a path that is, by definition, exceptional.

### 4.3 Neutral

- **The named return in `Execute`.** `Execute`'s implementation uses a named return (`code int`) because the deferred recovery must set the return value from the defer. The named return is invisible to `Execute`'s callers (the signature reads `func Execute() int`), but it is a syntactic detail a reader notices.
- **`Execute` calls `Run(context.Background())`.** The wrapper uses a background context, which is correct for a caller that only needs the code (it does not need signal handling). A future caller that wants signal handling calls `Run` with its own context.
- **The tests exercise `Run` through `runWithOptions`.** `Run` itself is a thin wrapper (`return runWithOptions(ctx, defaultOptions())`); the tests call `runWithOptions` directly with synthetic options. The difference is that `Run` reads the process's `os.Args` and writers, which the tests do not want. The seam for tests is `runWithOptions`; the seam for callers is `Run`.
- **The compatibility wrapper's eventual removal is deferred.** `Execute` is preserved for backward compatibility. Its removal is a future ADR's decision; the wrapper's presence now is a neutral consequence of the migration.

---

## 5. Related Requirements

- `FR-CLI-EXIT-023` — the single exit boundary's establishment. This ADR's primary requirement.
- `FR-CLI-EXIT-021` — the error-to-exit-code mapping. The mapping's outputs are the values `Run` returns; the boundary is where the mapping meets `os.Exit`.
- `FR-CLI-EXIT-024` — the stdout/stderr contract per exit code. The boundary's rendering of the error block and the JSON envelope is the mechanism that satisfies the contract on the failure path.
- `FR-CLI-EXIT-001` — the range taxonomy. The values `Run` returns are members of the taxonomy's ranges.
- `FR-CLI-EXIT-022` — the audit of error paths. The audit's finding that `os.Exit` appears in exactly one file is a consequence of this ADR.
- `NFR-QUALITY-005` — the CI-enforceable policy. The "one `os.Exit`" rule is enforced by `scripts/deps/verify-no-os-exit.sh`.
- `UX-CLI-005` — the signal-handling contract (Ctrl-C and SIGTERM produce the POSIX-coded exit codes). This ADR's signal handling is the mechanism.

> The following requirements must be added to the register when
> WBS 1.4 is complete: `FR-CLI-EXIT-026` (the `Run`
> signature's stability), `NFR-QUALITY-008` (the boundary's
> signal-handling test coverage), and `UX-CLI-006` (the
> `--help` interaction with signal handling).

---

## 6. Related ADRs

- `ADR-018` — Exit Code Migration and Stability Policy. The codes `Run` returns are the ones this ADR freezes. The boundary's job is to return the correct code; the codes' values are the subject of ADR-018.
- `ADR-019` — Exit Code Semantics. The severity classification applies to the codes; the boundary does not change severity, only reports it. The two decisions are independent.
- `ADR-011` — Closed Error Category Taxonomy. The categories are the layer between the error codes and the exit codes; the boundary's `exitCodeOf` helper reads the error's category to derive the code.
- `ADR-020` — JSON Error Envelope for `--format json`. The envelope is written by the boundary on the failure path when `--format json` is set. The boundary's write is the mechanism the envelope's contract depends on.
- `ADR-021` — stdout/stderr Contract per Exit Code. The boundary's rendering is the mechanism that satisfies the contract. The contract is a function of the exit code; the boundary is where the code and the output are produced together.
- `ADR-022` — Signal Handling and Exit Codes 130/143. A companion decision: the signal-handling mechanism's details (the wrapper shape, the `String()` comparison, the `Cause` inspection) are the subject of ADR-022. ADR-019 (this one) decides that signal handling *belongs* at the boundary; ADR-022 decides *how* it works.
- `ADR-001` — Forge as Foundation Manager. The product framing that motivates the CLI's exit-code contract; the boundary is the mechanism that delivers it.

---

## 7. Notes

### Open questions deferred by this ADR

- **The compatibility wrapper's removal.** `Execute() int` is preserved for backward compatibility. Its removal is a future ADR's decision. The removal's preconditions: no caller in the tree uses `Execute` (currently, the tests do), and no downstream consumer is known to use it.
- **The panic path's direct write to `os.Stderr`.** `Execute`'s deferred recovery writes the rendered panic to `os.Stderr` directly, bypassing the injected stderr. A future WBS could route the panic path through the injection layer by constructing the `options` value before the `defer`. The change is possible but adds complexity; the decision is deferred.
- **The signal set.** The boundary installs handlers for `os.Interrupt` (SIGINT) and `syscall.SIGTERM`. A future feature might want to handle SIGHUP (for a terminal's hangup) or SIGQUIT (for a core dump). The set is a policy; a change to it is a change to the boundary's contract. The current set is the minimum the CLI needs.
- **The context's propagation through commands.** The boundary passes the context to `root.ExecuteContext(ctx)`. Commands that respect the context return early on cancellation. The commands currently in the tree do not block, so the context's propagation is exercised only by the tests. A future command that performs long-running work must respect the context; the rule is documented in the command's docstring, but not enforced by a test.

### Follow-up decisions anticipated

- **WBS 11.3.2** (stdout/stderr contract per exit code). The boundary's rendering is the mechanism the contract depends on. The contract's details are the subject of the companion ADR.
- **WBS 11.4.1** (CI integration contract). The boundary's codes and streams are the inputs to the CI integration. The integration's contract is a future ADR's subject.
- **A future ADR on the `Run` signature's stability.** The `Run` signature is frozen by this ADR's decision; a change to it requires a new ADR. The future ADR would document the signature's stability class and the conditions under which it could change.

### Links to external discussion

- WBS 11.3.1 — the task that requested the single exit boundary.
- `cmd/forge/main.go` — the boundary's only `os.Exit` call.
- `internal/cli/execute.go` — `Run`, `Execute`, `runWithOptions`, `signalExitCode`, and the boundary's helpers.
- `docs/architecture/exit-boundary.md` — the sequence diagrams and the rationale.
- `scripts/deps/verify-no-os-exit.sh` — the CI check that enforces the "one `os.Exit`" rule.
- `cmd/forge/binary_integration_test.go` — the end-to-end tests that assert the process boundary's behaviour.

---

*End of ADR-019.*
