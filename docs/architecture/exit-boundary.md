# Exit Boundary

<!--
  Document:     exit-boundary.md
  Purpose:      Define where and how the Forge CLI terminates the
                process, and document the flow from a command's
                error return to the process exit code.
  Audience:     Forge maintainers, reviewers, future auditors.
  Stability:    Design decision, frozen. Changes require an ADR.
  Traceability: FR-CLI-EXIT-023, ADR-020, WBS 11.3.1
  Last Review:  2026-10-12
-->

## Overview

The Forge CLI terminates the process in exactly one place: the
`os.Exit` call in `cmd/forge/main.go`. Every other function —
command handlers, the boundary, the error mapping — returns
values; only `main` terminates.

This is the **single exit boundary** pattern. It exists so that:

1. **Exit codes are derived, not assumed.** Every command returns
   an error; the boundary converts it to a code through the
   mapping (`cli.ExitCode`). A command cannot accidentally exit
   with the wrong code because it cannot exit at all.

2. **The error is rendered exactly once.** The boundary renders
   the error through `forgeerr.Format` and writes it to stderr.
   Commands do not write to stderr; they return errors.

3. **Tests can observe the code.** `cli.Run(ctx)` returns the
   code; a test calls it, inspects the return value, and does not
   spawn a subprocess.

4. **Signals are handled at the boundary.** `signal.NotifyContext`
   installs a context that is cancelled on SIGINT or SIGTERM; the
   boundary converts the cancellation to `ExitSIGINT` (130) or
   `ExitSIGTERM` (143) and returns it.

## The flow

The sequence for a successful invocation:

```text
┌────────┐    ┌───────────┐    ┌──────────┐    ┌──────────┐
│ shell  │───▶│  main()   │───▶│ cli.Run  │───▶│ command  │
└────────┘    └───────────┘    └──────────┘    └──────────┘
                    │                │                │
                    │                │           (runs; writes
                    │                │            stdout; returns
                    │                │            nil)
                    │                │                │
                    │                │◀───────────────┘
                    │                │
                    │       (writes to stdout via the
                    │        command's output; no error
                    │        is rendered)
                    │                │
                    │◀───────────────┘
                    │       (returns (0, nil))
                    │
              os.Exit(0)
```

The sequence for a failing invocation:

```text
┌────────┐    ┌───────────┐    ┌──────────┐    ┌──────────┐
│ shell  │───▶│  main()   │───▶│ cli.Run  │───▶│ command  │
└────────┘    └───────────┘    └──────────┘    └──────────┘
                    │                │                │
                    │                │           (fails; returns
                    │                │            *forgeerr.Error)
                    │                │                │
                    │                │◀───────────────┘
                    │                │
                    │       (Cobra returns the error;
                    │        boundary wraps/unwraps;
                    │        forgeerr.Format renders;
                    │        boundary writes to stderr;
                    │        boundary maps to exit code)
                    │                │
                    │◀───────────────┘
                    │       (returns (code, err))
                    │
              os.Exit(code)
```

The sequence for a signal:

```text
┌────────┐    ┌───────────┐    ┌──────────────┐    ┌──────────┐
│ kernel │───▶│ signal.   │───▶│ cli.Run's    │───▶│ command  │
│        │    │ Notify-   │    │ ctx is       │    │ (blocks; │
│        │    │ Context   │    │ cancelled    │    │ returns  │
│        │    │           │    │              │    │ ctx.Err) │
└────────┘    └───────────┘    └──────────────┘    └──────────┘
                    │                │                │
                    │                │◀───────────────┘
                    │                │
                    │       (boundary inspects the signal;
                    │        maps SIGINT to 130,
                    │        SIGTERM to 143;
                    │        writes a minimal line to stderr)
                    │                │
                    │◀───────────────┘
                    │       (returns (code, nil))
                    │
              os.Exit(code)
```

## The `Run` signature

```go
func Run(ctx context.Context) (int, error)
```

`Run` returns two values:

- **The exit code.** Always a documented value from the WBS 11.1.2
  table.
- **The error.** The error the boundary rendered, or `nil` if the
  invocation succeeded. The error is returned so that a caller can
  inspect it (a test asserts on the error's type; a future wrapper
  script that embeds Forge as a library can propagate it). `main`
  ignores it: it passes the code to `os.Exit`, and the error has
  already been rendered to stderr.

A nil error paired with a non-zero exit code is impossible in the
current design: the boundary maps the error to a code, and the
code is non-zero if and only if the error is non-nil. A future
WBS that adds a code-less failure mode (for example, a warning
that returns non-zero without an error) would need to relax the
invariant; the signature accommodates it.

## Why Cobra is silenced

Cobra's default behaviour is to print errors and usage to its
output stream and to return them. The printing duplicates the
boundary's rendering: the user sees the error twice, once in
Cobra's format and once in `forgeerr`'s.

`root.SilenceErrors = true` and `root.SilenceUsage = true` disable
Cobra's printing. The error reaches the boundary unprinted; the
boundary renders it exactly once.

## Signal handling

`signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)` returns
a context that is cancelled when either signal arrives. The
command tree receives that context. When the signal fires:

1. The context is cancelled.
2. Every command that respects the context returns early with
   `ctx.Err()` (typically `context.Canceled`).
3. The boundary catches the error, inspects the signal via the
   context's `Cause`, and returns the mapped code: `ExitSIGINT`
   (130) for SIGINT, `ExitSIGTERM` (143) for SIGTERM.

The distinction between the two signals is made by the `Cause` the
`NotifyContext` implementation attaches to the context. If the
cause is unavailable (an older Go version, or a context cancelled
by something other than a signal), the boundary falls back to
`ExitSIGINT` (the more common case).

## Why `main` does not call `cli.Execute`

The historical `Execute()` signature returns only the exit code.
`main` cannot distinguish "the command failed" from "the command
succeeded with a non-zero code" (the latter is impossible today,
but the two-value form is the future-proof choice). `Run`
returns both values, so `main` can log the error in a future WBS
without changing the boundary's signature.

`Execute` is preserved as a thin wrapper for backward
compatibility: `Execute()` calls `Run(context.Background())` and
returns the code, discarding the error. Callers that only need
the code continue to work; callers that want the error call
`Run`.

## References

- `cmd/forge/main.go` — the single `os.Exit` and the signal setup.
- `internal/cli/execute.go` — `Run` and `Execute`.
- `internal/cli/mapping.go` — the error-to-code mapping.
- `docs/reference/exit-codes.md` — the frozen table.
- `docs/decisions/ADR-020-exit-boundary-pattern.md` — the decision.
