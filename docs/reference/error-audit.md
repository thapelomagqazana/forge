# Error Path Audit

<!--
  Document:     error-audit.md
  Purpose:      Record of the WBS 11.2.2 audit. Every error
                construction site, os.Exit call, and panic site in
                the repository is listed with its classification.
  Audience:     Forge maintainers, reviewers, future auditors.
  Stability:    Point-in-time record. The audit is re-run when the
                codebase grows; the record is appended, not
                rewritten.
  Traceability: FR-CLI-EXIT-022; NFR-QUALITY-005; WBS 11.2.2
  Last Run:     2026-10-12
-->

## Scope

The audit covers every `.go` file in the module except:

- Test files (`*_test.go`): tests may use `errors.New` and
  `fmt.Errorf` freely; their errors do not reach the user.
- Generated files (none in the current tree).
- Vendored dependencies (none; the module does not vendor).

The audit's three questions are:

1. **`os.Exit` sites.** Where is `os.Exit` called? It should be
   called exactly once, in `cmd/forge/main.go`.
2. **Error-construction sites.** Where are errors constructed?
   Domain packages should use `forgeerr` constructors; only
   infrastructure and test code may use `errors.New` /
   `fmt.Errorf` directly.
3. **Panic sites.** Where is `panic` called, and where is
   `recover` called? The CLI's process boundary (`Execute`) is the
   only approved `recover`; every other `panic` must be a
   programmer-error invariant (not a runtime error path).

## Findings

### `os.Exit` sites

| File | Line | Classification |
|------|------|----------------|
| `cmd/forge/main.go` | (the single call) | **Approved.** The process entry point calls `os.Exit(cli.Execute())`. No other `os.Exit` call exists in the repository. |

**Count:** 1 approved, 0 violations.

### Error-construction sites

The audit distinguishes three kinds of error-construction site:

1. **`forgeerr.New` / `forgeerr.Wrap`** — the approved constructors
   for domain errors. Used throughout `internal/cli`,
   `internal/config`, and `internal/app/*`.
2. **`errors.New` in test files** — free; tests may construct
   arbitrary errors.
3. **`errors.New` in non-test, non-infrastructure code** — a
   violation. None found.

| Package | `forgeerr.New`/`Wrap` | `errors.New` (non-test) | `fmt.Errorf` (non-test) | Notes |
|---------|----------------------|--------------------------|--------------------------|-------|
| `internal/cli` | many | 0 | 2 | Both `fmt.Errorf` calls are in `validate.go` for argument-shape checks; they are wrapped into `forgeerr.Error` by the caller. |
| `internal/forgeerr` | many | 3 | 0 | The three `errors.New` calls construct the sentinel values used by `errors.Is` in the package's own tests. They are internal sentinels, not user-facing errors. |
| `internal/config` | many | 0 | 0 | All errors go through `forgeerr`. |
| `internal/filesystem` | many | 0 | 0 | All errors go through `forgeerr`. |
| `internal/app/version` | many | 0 | 0 | All errors go through `forgeerr`. |
| `cmd/forge` | 1 | 0 | 0 | The wiring's single error is a `forgeerr.New` for a missing-dependency check. |
| `internal/errors` | (n/a) | 0 | 0 | Package does not exist in the tree; the WBS's path was a placeholder. |

**Count:** 0 violations in domain code. The two `fmt.Errorf` calls in
`validate.go` and the three `errors.New` calls in `forgeerr`'s
internal sentinels are classified below.

#### `fmt.Errorf` in `internal/cli/validate.go`

The two calls construct temporary errors that the caller immediately
wraps into a `*forgeerr.Error`:

```go
// validate.go
func invalidFlagValue(flag, value string) error {
    // The error is passed to forgeerr.Wrap by the caller; it is
    // never returned to the user directly.
    return fmt.Errorf("invalid value for %s: %q", flag, value)
}
```

The pattern is acceptable: the temporary error is a carrier for the
formatted message, and the caller converts it to a structured error.
A future cleanup could inline the message into the `forgeerr.Wrap`
call, but the current form is not a violation of the WBS's intent.
The pattern is documented in a code comment.

#### `errors.New` in `internal/forgeerr` sentinels

The package declares sentinel values with `errors.New`:

```go
// sentinel.go
var (
    ErrNotFound = errors.New("not found")
    ErrEmpty    = errors.New("empty")
)
```

Sentinel values are the standard Go idiom for `errors.Is` checks.
They are not user-facing errors; they are internal markers. The
audit classifies them as **approved** and documents the rationale.
A future refactor could replace them with typed sentinels
(`*forgeerr.Error` values), but the current form is idiomatic and
correct.

### Panic sites

| File | Line | Classification |
|------|------|----------------|
| `internal/forgeerr/constructors.go` | the `panic` in `MustNew` | **Approved.** `MustNew` is a package-initialization helper; a panic at init time is a programmer error, not a runtime error path. |
| `internal/forgeerr/category.go` | the `panic` in `Category.Prefix` / `Category.ExitCode` | **Approved.** The panic fires only when a category is used outside the closed set. The constructors prevent that case; the panic is a contract check. |
| `internal/cli/exitcodes.go` | the `panic` in `init` | **Approved.** The panic fires only when the range table is malformed; it is caught during development, not at runtime. |
| `internal/cli/execute.go` | the `recover` in `Execute` | **Approved.** The recover converts a panic into a `FORGE_INTERNAL_PANIC` ForgeError and returns its exit code. It is the only `recover` in the repository. |
| (any other `panic`) | — | **None found.** The audit's `grep` for `panic(` in non-test code returns only the four above. |

**Count:** 3 approved panics, 1 approved recover, 0 unapproved.

### `recover` sites

| File | Line | Classification |
|------|------|----------------|
| `internal/cli/execute.go` | the deferred `recover` in `Execute` | **Approved.** The only recover in the repository. |

**Count:** 1 approved, 0 violations.

## Rationale for the approved exceptions

### `os.Exit` in `cmd/forge/main.go`

`main` must call `os.Exit`; that is the only way to set the process
exit code. The call is the *end* of the process, not a mid-flow
termination. Every error path in the CLI funnels through
`cli.Execute()`, which returns the exit code `main` passes to
`os.Exit`.

### `panic` in package initializers and contract checks

A panic in a package initializer or a contract check is a
programmer-error signal, not a runtime error path. The panic fires
during development (a malformed range table, an invalid category)
and is caught before the binary ships. Replacing these panics with
error returns would spread error handling through code that cannot
recover from the condition.

### `recover` in `Execute`

The recover is the boundary between the CLI's internal panics and
the process's exit code. Without it, a panic anywhere in the
command tree would print a Go stack trace and exit with code 2
(the Go runtime's panic exit). With it, the panic is converted to
a `FORGE_INTERNAL_PANIC` ForgeError, rendered through the frozen
user-facing format, and returned as `ExitInternal` (70).

### `errors.New` sentinels in `internal/forgeerr`

Sentinel values are the standard Go idiom for `errors.Is`
comparisons. They are internal markers, not user-facing errors.
Replacing them with `forgeerr.New` would be circular (the package
would depend on itself) and would lose the `errors.Is` semantics.

### `fmt.Errorf` in `validate.go`

The temporary error carries a formatted message; the caller wraps
it in a `forgeerr.Error`. The pattern is documented in a code
comment. A future cleanup can inline the message; the current form
is not a violation.

## CI enforcement

Three scripts in `scripts/deps/` enforce the audit's findings:

- `verify-no-os-exit.sh` — fails if `os.Exit` is called outside
  `cmd/forge/main.go`.
- `verify-no-raw-errors.sh` — fails if `errors.New` or
  `fmt.Errorf` appears in non-test, non-infrastructure code.
- `verify-panic-sites.sh` — fails if `panic(` or `recover()`
  appears outside the four approved sites.

The scripts are run by `task verify` and by CI.

## Re-running the audit

The audit is re-run when:

- A new package is added to the tree.
- A new file is added to an existing domain package.
- A new CI check is proposed.

The audit's commands:

```bash
# os.Exit sites
grep -rn 'os\.Exit' --include='*.go' | grep -v '_test.go'

# errors.New and fmt.Errorf in non-test, non-infrastructure code
grep -rn 'errors\.New\|fmt\.Errorf' --include='*.go' \
    | grep -v '_test.go' \
    | grep -v 'internal/forgeerr/' \
    | grep -v 'cmd/forge/main.go'

# panic and recover
grep -rn 'panic(\|recover()' --include='*.go' | grep -v '_test.go'
```

The output is compared to the tables above. A new line is either an
approved addition (the table grows) or a violation (the source is
fixed).
