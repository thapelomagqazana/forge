# cmd/forge

This directory contains the process entry point for the `forge` binary.

## Files

| File | Purpose | Line budget |
|---|---|---|
| `main.go` | Process entry point. Delegates to `internal/cli.Execute()`. | ≤ 20 lines of code |
| `doc.go` | Package-level documentation. | Unbounded (comments only) |
| `README.md` | This file. | Unbounded (documentation only) |

## The Invariant

**`main.go` must remain minimal.**

It performs exactly one action: it calls `cli.Execute()` and passes the
result to `os.Exit()`.

Everything else — argument parsing, environment reading, I/O, error
formatting, panic recovery, logging, signal handling — belongs in
`internal/cli` or deeper, where it can be tested.

## Why

`main.go` is the only file in the repository that cannot be unit-tested.
It *is* the process. Any logic placed here is logic that cannot be
invoked from within a test binary, so any logic placed here is logic
that cannot be verified.

The solution is not to add tests to `main.go`. The solution is to add
**no logic to `main.go`**, so there is nothing to test.

Every observable behaviour of the `forge` binary is tested through
`internal/cli`, whose `Execute()` function accepts injectable inputs.

## What belongs here

Almost nothing. Legitimate changes to this directory are:

- Changing the entry-point contract (e.g., adding panic recovery if
  the team decides that is the right design — but see the ADR
  discussion first).
- Updating the documentation to reflect a new invariant.
- Adding build tags (e.g., `//go:build unix` for a Unix-specific
  entry point — not currently planned).

## What does NOT belong here

- Business logic of any kind.
- Argument parsing.
- Environment variable reading.
- File I/O.
- Network I/O.
- Error formatting.
- Logging.
- Configuration loading.
- Version printing.
- Help printing.
- Signal handling.
- Panic recovery.
- Anything that reads from or writes to stdout/stderr.

## Reviewing a change to this directory

When reviewing a PR that touches `cmd/forge/`, ask two questions:

1. **Does the change add logic?** If yes, the logic probably belongs
   in `internal/cli`. Ask the author to move it.
2. **Does the change add an import?** Only `os` and
   `internal/cli` are permitted. Any other import is a strong signal
   that the logic belongs one level deeper.

A PR that adds an import to `main.go` should be rejected by default.
The correct response is: "why can this not live in internal/cli?"

## See also

- [`docs/architecture.md`](../../docs/architecture.md) — process entry
  point section.
- [`internal/cli/doc.go`](../../internal/cli/doc.go) — the CLI layer
  that `main.go` delegates to.
- [`internal/cli/execute.go`](../../internal/cli/execute.go) — the
  `Execute()` function whose signature `main.go` depends on.
- [WBS 2.2.1](../docs/phase-2-requirements.md) — this task in the
  requirement register.
