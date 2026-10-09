# Reproducibility Scripts

This directory contains scripts that verify Forge's reproducibility
invariants. A reproducibility invariant is a property that must hold
across two or more independent environments given the same inputs.

The invariants are stated formally in
[`docs/development.md`](../../docs/development.md), section
"Reproducible Dependency Resolution". The scripts in this directory
are implementations of those invariants.

---

## Table of Contents

- [Why reproducibility is separate from integrity](#why-reproducibility-is-separate-from-integrity)
- [Scripts in this directory](#scripts-in-this-directory)
- [Running the scripts](#running-the-scripts)
- [The lib/ directory](#the-lib-directory)
- [The invariant contract](#the-invariant-contract)
- [Adding a new script](#adding-a-new-script)
- [Modifying a script](#modifying-a-script)
- [Testing the scripts](#testing-the-scripts)
- [Relationship to other check directories](#relationship-to-other-check-directories)
- [References](#references)

---

## Why reproducibility is separate from integrity

Two concepts are often conflated:

- **Integrity** is a local property. It asks: is the committed
  `go.sum` consistent with the committed `go.mod`, in *this*
  environment?

- **Reproducibility** is a global property. It asks: do two
  independent clean environments resolve the same module tree from
  the same commit?

A repository can have perfect integrity and still fail
reproducibility. This happens when the environment differs between
two machines in a way that the lockfile does not capture — for
example, different `GOPROXY` settings, different `GOFLAGS`, or a
poisoned module cache.

Keeping the checks separate allows failures to be attributed
precisely:

- An **integrity failure** means the lockfiles are stale or were
  hand-edited. The remedy is `task tidy` on a clean branch.
- A **reproducibility failure** means the environments differ. The
  remedy is to align the environment settings or investigate the
  cache state.

This directory contains the reproducibility checks. The integrity
checks live in [`scripts/deps/`](../deps/).

---

## Scripts in this directory

| Script                    | Invariant                                                       | WBS     |
|---------------------------|-----------------------------------------------------------------|---------|
| `verify-resolution.sh`    | Two clean environments resolve identical module trees.          | 3.2.1   |

Additional scripts are added here as new reproducibility invariants
are defined in later WBS items.

### `verify-resolution.sh`

Verifies the module resolution reproducibility invariant:

> Given the same commit and the same `go.mod`, `go.sum`, and pinned
> toolchain, two independent clean environments must resolve to the
> identical module tree.

The script performs five steps:

1. **Check preconditions.** Verifies that `go.mod` and `go.sum` exist,
   are committed, and have no uncommitted changes.

2. **Capture the committed tree.** Runs `go list -m all` in the
   current environment to get a reference tree.

3. **Verify the reference tree is complete.** Counts the `require`
   directives in `go.mod` and confirms that the resolved tree
   contains at least as many modules. This catches the case where
   the module cache is empty and the toolchain resolved nothing.

4. **Resolve in two clean environments.** Each environment gets its
   own temporary `GOMODCACHE` and `GOPATH`. The resolved trees are
   captured and compared.

5. **Compare against the committed tree.** Verifies that the
   resolved trees match the committed state. This catches the case
   where `go.sum` is stale relative to `go.mod`.

The script exits 0 if all checks pass, and 1 if any check fails. The
failure message identifies the specific cause.

---

## Running the scripts

Each script is invoked by a task in
[`Taskfile.yml`](../../Taskfile.yml). The task names follow the
pattern `verify:<invariant>`:

```sh
# Run the resolution reproducibility check.
task verify:resolution
```

The scripts can also be run directly for debugging:

```sh
sh scripts/reproducible/verify-resolution.sh
```

Direct invocation is useful when:

- The task wrapper is producing confusing output.
- You want to see the script's raw output without Task's
  pre-processing.
- You are debugging a change to the script itself.

The script is POSIX `sh`, so it works on Linux, macOS, and Windows
(via Git Bash or WSL) without requiring `bash`.

### Running from a subdirectory

The script locates the repository root via
`git rev-parse --show-toplevel`, so it can be invoked from any
subdirectory:

```sh
cd internal/cli
sh ../../scripts/reproducible/verify-resolution.sh
```

This is the same behaviour as the Taskfile tasks, which use
`dir: "{{.ROOT}}"` to operate from the repository root.

---

## The lib/ directory

`lib/common.sh` provides shared helpers used by every script in this
directory. It is sourced, not executed.

### What the helpers provide

- **Output formatting.** Consistent headers, `OK:` / `FAIL:` markers,
  and colour output that degrades gracefully on non-TTY streams.

- **Temporary directory management.** A `repro_mktemp` helper that
  creates a temporary directory and registers it for exception-safe
  cleanup via an `EXIT` trap. Multiple calls accumulate; all
  directories are removed when the script exits.

- **Clean-environment detection.** A `repro_has_clean_cache` helper
  that inspects `GOMODCACHE` and returns whether it is clean.

- **Go availability check.** A `repro_assert_go_available` helper
  that fails the script with an actionable message if the Go
  toolchain is not on `PATH`.

### Colour degradation

The helpers emit colour only when:

1. `stdout` is a TTY, and
2. `NO_COLOR` is not set in the environment.

See <https://no-color.org> for the `NO_COLOR` convention. This
ensures the scripts are usable in terminals, in CI logs, and when
piped to other commands.

---

## The invariant contract

Every script in this directory satisfies a common contract. The
contract exists so that scripts can be composed, tested, and
diagnosed uniformly.

### The contract

1. **Exit code semantics.** The script exits 0 when the invariant
   holds, and 1 when it fails. Other exit codes are reserved for
   unexpected errors (for example, a missing dependency).

2. **Output format.** The script prints human-readable progress
   messages. Success lines begin with `OK:` (through the
   `repro_ok` helper). Failure lines begin with `FAIL:` and are
   written to `stderr`.

3. **Preconditions.** The script verifies that the repository is in
   a state where the invariant is meaningful. If a precondition
   fails, the script exits 1 with a specific message.

4. **Idempotence.** Running the script twice produces the same
   result. The script does not leave the repository in a modified
   state.

5. **Isolation.** The script uses temporary directories for any
   state it creates. The repository is never modified.

6. **No network beyond `GOPROXY`.** The script uses the Go toolchain
   for any network access. It does not invoke `curl`, `wget`, or
   other network tools directly.

7. **Testability.** The script is accompanied by a test harness in
   `tests/scripts/reproducible/` that proves it fails under the
   conditions it is meant to catch.

### Why the contract matters

The contract allows a reader to reason about a script without
reading its implementation. When they see a script in this
directory, they know:

- What to expect from the exit code.
- Where the output goes.
- Whether the script is safe to run.
- How to test a change.

The contract is not enforced by a linter; it is enforced by
convention and by the test harness. New scripts are expected to
follow it.

---

## Adding a new script

Adding a reproducibility check follows a consistent process.

### Step 1 — Define the invariant

State the invariant as a single quotable sentence. For example:

> Given the same commit and the same `go.mod`, the resolved module
> tree must be the same in two clean environments.

The invariant must be:

- **Falsifiable.** There must be a scenario in which it is false.
- **Testable.** There must be a procedure to verify it.
- **Meaningful.** Its failure must indicate a real problem.

### Step 2 — Document the invariant

Add a section to
[`docs/development.md`](../../docs/development.md) that:

- States the invariant formally.
- Defines any preconditions (for example, "clean environment").
- Documents the verification procedure.
- Lists known limitations.

The documentation is the specification. The script is the
implementation.

### Step 3 — Implement the script

Create `scripts/reproducible/<invariant-name>.sh`. Follow the
pattern established by `verify-resolution.sh`:

- Source `lib/common.sh`.
- Follow the invariant contract above.
- Include a header comment that documents what the script verifies,
  what it does not verify, and the exit code semantics.
- Use the `repro_*` helpers for output and temporary directory
  management.

### Step 4 — Add a Taskfile task

Add a task to `Taskfile.yml` under the `verify:` namespace:

```yaml
  verify:<name>:
    desc: <short description> (WBS X.Y.Z)
    dir: "{{.ROOT}}"
    cmds:
      - sh scripts/reproducible/<invariant-name>.sh
```

Then add the task to the composed `verify` and `check` tasks so it
runs in the pre-commit hook and in CI.

### Step 5 — Add a test harness

Create `tests/scripts/reproducible/<invariant-name>_test.sh`. The
harness must:

- Construct synthetic repositories that trigger each branch of the
  script.
- Assert on the exit code and the error message for each scenario.
- Include at least: a happy path, and one scenario per failure
  branch.

### Step 6 — Register the harness

Add the harness to the `test:scripts` task in `Taskfile.yml`:

```yaml
  test:scripts:
    desc: Run shell test harnesses for the scripts/ directory
    dir: "{{.ROOT}}"
    cmds:
      - sh tests/scripts/deps/verify-integrity_test.sh
      - sh tests/scripts/reproducible/<invariant-name>_test.sh
```

### Step 7 — Update this README

Add a row to the table in [Scripts in this directory](#scripts-in-this-directory).

---

## Modifying a script

Before modifying an existing script, understand what it protects
against:

- `verify-resolution.sh` protects against environment-dependent
  resolution.

Changes should preserve the invariant contract. For example:

- Adding a check is fine if it is fast and does not duplicate
  existing checks.
- Removing a check requires justification. The check exists because
  it caught a real problem at some point.
- Changing the output format requires updating the test harness.

### Testing a modification

1. Modify the script.
2. Run `sh scripts/reproducible/<name>.sh` directly.
3. Run the test harness: `sh tests/scripts/reproducible/<name>_test.sh`.
4. Run the full `task check` to confirm no other check is affected.

Do not modify a reproducibility script without running its test
harness. The harness is what guarantees the script continues to
behave as documented.

---

## Testing the scripts

The test harnesses live in `tests/scripts/reproducible/`. Each
harness:

- Creates synthetic repositories in temporary directories.
- Runs the script under test against each repository.
- Asserts on the exit code and the error message.
- Cleans up its temporary directories on exit.

### Running the harnesses

```sh
# Run all script test harnesses.
task test:scripts

# Run a specific harness.
sh tests/scripts/reproducible/verify-resolution_test.sh
```

### Adding a scenario to a harness

Each scenario in a harness follows the same pattern:

```sh
tmpdir=$(make_synthetic_repo)
cd "$tmpdir"
assert_exit 1 "scenario description" sh "$resolution_script"
assert_message "expected substring" \
    "message description" \
    sh "$resolution_script"
cd "$repo_root"
```

The `assert_exit` and `assert_message` helpers are defined in the
harness. They report pass/fail and accumulate counts.

---

## Relationship to other check directories

The repository has three directories containing verification
scripts. Each has a distinct scope.

| Directory                    | Scope                                                       |
|------------------------------|-------------------------------------------------------------|
| `scripts/deps/`              | Integrity of the dependency lockfiles.                       |
| `scripts/reproducible/`      | Reproducibility of resolution across independent environments. |
| `scripts/adr/`               | Format and completeness of Architecture Decision Records.    |

The three directories are independent. A change to one does not
affect the others. Each is tested by its own harness in
`tests/scripts/`.

### Why the split

The split mirrors the conceptual split between the properties being
verified:

- **Integrity** is local. It concerns what is committed.
- **Reproducibility** is global. It concerns what independent
  environments do.
- **Documentation quality** is orthogonal. It concerns whether ADRs
  are well-formed.

Keeping the scripts separate ensures that each directory has a
single, clear responsibility. A contributor who is debugging a
dependency problem knows which directory to look in.

---

## References

- [`docs/development.md`](../../docs/development.md) — "Reproducible
  Dependency Resolution" section, the formal specification.
- [`docs/development.md`](../../docs/development.md) — "Reproducible
  Builds" section, the environment configuration.
- [`docs/dependency-policy.md`](../../docs/dependency-policy.md) —
  the dependency policy.
- [`Taskfile.yml`](../../Taskfile.yml) — the task definitions.
- [`scripts/deps/README.md`](../deps/README.md) — the integrity
  checks.
- [`scripts/adr/`](../adr/) — the ADR tooling.
- [`tests/scripts/`](../../tests/scripts/) — the test harnesses.
- [`scripts/reproducible/lib/common.sh`](./lib/common.sh) — the
  shared helpers.
- [`scripts/reproducible/verify-resolution.sh`](./verify-resolution.sh)
  — the resolution reproducibility check.
