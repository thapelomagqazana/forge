# Forge Git Hooks

This directory contains the Git hooks that run automatically before
each commit and each push. They are versioned with the repository, so
every contributor gets the same checks.

The hooks are thin wrappers around tasks defined in
[`Taskfile.yml`](../Taskfile.yml). They select which tasks to run based
on what changed, then invoke the tasks and aggregate the results. The
result is a local verification loop that is fast enough to be worth
running and identical to what CI runs.

---

## Table of Contents

- [Why the hooks exist](#why-the-hooks-exist)
- [Installation](#installation)
- [The hooks](#the-hooks)
- [Bypassing the hooks](#bypassing-the-hooks)
- [Running the hooks manually](#running-the-hooks-manually)
- [How the hooks work](#how-the-hooks-work)
- [Adding a new hook](#adding-a-new-hook)
- [Modifying a hook](#modifying-a-hook)
- [Troubleshooting](#troubleshooting)
- [Uninstalling](#uninstalling)
- [References](#references)

---

## Why the hooks exist

CI is the last line of defence, not the first. Running the checks
locally, before a commit or push, provides three benefits:

1. **Faster feedback.** A failing check is reported in seconds, not
   after a CI round-trip that takes minutes.
2. **Cheaper CI.** A push that would fail CI is caught before it
   reaches the remote, so CI compute is spent on changes that are
   worth verifying.
3. **Cleaner history.** A commit that fails formatting or lint never
   enters the log. The history stays legible and every commit is
   trusted.

The hooks enforce the same rules that CI enforces. A contributor who
never bypasses the hooks will never see a CI failure that the hooks
could have caught.

---

## Installation

Run once per clone:

```sh
task hooks:install
```

Or directly:

```sh
./scripts/install-hooks.sh
```

The installer sets `core.hooksPath` to `.githooks` in the local Git
configuration. It does not modify anything in `.git/hooks/`. It is
idempotent and safe to re-run.

### Verifying the installation

```sh
task hooks:status
```

Expected output when installed:

```text
OK: hooks installed (core.hooksPath = .githooks)
```

Expected output when not installed:

```text
WARN: hooks not installed (core.hooksPath = unset)
      Run 'task hooks:install' to install them.
```

### Why installation is required

Git does not version hooks by default because they live inside `.git/`,
which is not tracked. The standard workaround is to store the hooks in
a tracked directory (`.githooks/`) and point Git at them via the
`core.hooksPath` configuration. The installer performs this one-time
configuration.

Without the installer, the hooks exist in the repository but Git does
not use them.

---

## The hooks

| Hook         | When it runs           | Runtime     | Checks                                                              |
|--------------|------------------------|-------------|---------------------------------------------------------------------|
| `pre-commit` | Before each commit     | ~5 seconds  | Formatting, Markdown lint, `go vet`, dependency policy.             |
| `pre-push`   | Before each push       | ~1 minute   | Module, version matrix, Cobra pin, reproducibility, unit tests, integration tests. |
| `commit-msg` | After message is ready | <1 second   | Conventional Commits format.                                        |

### `pre-commit`

Runs before each `git commit`. This hook performs the fast checks
from the quality gate:

- **`task fmt:check`** — Go source files are formatted per `gofmt`.
- **`task lint:md`** — Markdown files pass the rules in
  [`.markdownlint.yaml`](../.markdownlint.yaml).
- **`task vet`** — `go vet ./...` passes.
- **`task verify:deps`** — the dependency policy in
  [`docs/dependency-policy.md`](../docs/dependency-policy.md) is
  respected.

The hook is *selective*: it inspects the staged files and runs only
the checks that are relevant. A commit that changes only Markdown does
not trigger `go vet`. A commit that changes only Go does not trigger
`lint:md`. This keeps the hook under five seconds for the common case.

### `pre-push`

Runs before each `git push` contacts the remote. This hook performs
the slower checks:

- **`task module:verify`** — the Go module is configured correctly.
- **`task goversion:verify`** — the Go version matrix is documented
  and enforced.
- **`task verify:cobra`** — Cobra is pinned, imported by exactly one
  file, and documented.
- **`task verify:reproducible`** — the reproducibility invariant
  holds.
- **`task test`** — unit tests pass with the race detector.
- **`task test:integration`** — the compiled binary behaves
  correctly at the process boundary.

This hook is also selective. A push that contains only documentation
changes runs a subset of the checks. A push that changes
`go.mod` runs every relevant check.

### `commit-msg`

Runs after `git commit` has assembled the message but before the
commit is finalised. It validates the message against the
Conventional Commits format:

```text
<type>(<scope>): <subject>
```

Where `<type>` is one of:

- `feat` — a new feature
- `fix` — a bug fix
- `docs` — documentation only
- `style` — formatting, no code change
- `refactor` — restructuring without behaviour change
- `perf` — performance improvement
- `test` — adding or fixing tests
- `build` — build system or dependency changes
- `ci` — CI configuration changes
- `chore` — other changes that do not fit above
- `revert` — reverting a previous commit

The hook also enforces:

- The subject line is at most 72 characters.
- The subject line does not end with a period.
- The subject line starts with a lowercase letter.

Merge commits and revert commits are exempt because their messages are
generated by Git.

---

## Bypassing the hooks

Each hook can be bypassed for a single operation:

```sh
git commit --no-verify    # Skip pre-commit and commit-msg
git push --no-verify      # Skip pre-push
```

Bypassing is documented because forbidding it would push contributors
to disable the hooks permanently. A documented bypass is used
deliberately; an undocumented one is used habitually.

**Bypassing is discouraged.** The hooks run the same checks that CI
runs. A bypass that hides a failure will be caught by CI, at which
point the cost of fixing the issue is higher: CI has been consumed,
the push has been rejected, and the failure must be understood from
CI logs instead of local output.

When a bypass is genuinely necessary (for example, during a large
merge conflict resolution where intermediate commits are known to
fail), prefer:

1. Perform the operation with `--no-verify`.
2. Immediately run `task check` locally.
3. Fix any failures before pushing.

This preserves the spirit of the hooks while accommodating the
exception.

---

## Running the hooks manually

To run a hook without performing the corresponding Git operation:

```sh
task hooks:test          # Run the pre-commit checks
task hooks:test:push     # Run the pre-push checks
```

The tasks invoke the hook scripts directly. They do not create a
commit or push anything. They are useful for:

- Verifying a change before staging it.
- Debugging a hook that failed unexpectedly.
- Testing a modification to a hook.

To run individual checks without going through the hook:

```sh
task fmt:check
task lint:md
task vet
task verify:deps
task test
task test:integration
```

These are the same tasks the hooks invoke. Running them directly
produces the same output and the same exit codes.

---

## How the hooks work

Every hook follows the same pattern:

1. **Locate the repository root.** The hooks use
   `git rev-parse --show-toplevel` so they work from any directory
   inside the repository.

2. **Source shared helpers.** The file
   [`.githooks/lib/common.sh`](./lib/common.sh) provides consistent
   output formatting and a task runner.

3. **Determine what changed.** The hook inspects staged files (for
   `pre-commit`) or pending commits (for `pre-push`) and decides which
   checks are relevant.

4. **Run the selected checks.** Each check is a `task` target. The
   hook invokes the task and records the result.

5. **Aggregate and report.** If any check failed, the hook prints a
   summary and exits with a non-zero status. Git aborts the
   operation.

The hooks are deliberately minimal. They do not reimplement any
checks; they only select and orchestrate. This is what makes them easy
to maintain and reason about.

### The shared helper library

`.githooks/lib/common.sh` provides:

- **Colour detection.** Colour is emitted only when stdout is a TTY
  and `NO_COLOR` is not set.
- **Output helpers.** `hook_header`, `hook_ok`, `hook_fail`,
  `hook_warn`, `hook_info`.
- **Task detection.** `hook_have_task` returns whether `task` is on
  `PATH`.
- **Task runner.** `hook_run_task` invokes a task, suppresses output
  on success, and shows the full output on failure.
- **Failure summary.** `hook_summarise` prints the bypass hint.

The library is POSIX `sh`, not `bash`. This makes the hooks portable
across Linux, macOS, and Windows (via Git Bash or WSL).

### Why POSIX `sh`

The hooks use `#!/usr/bin/env sh` rather than `#!/usr/bin/env bash`.
This is deliberate:

- `sh` is available everywhere. `bash` is not installed by default on
  some minimal Linux distributions.
- Windows contributors use Git Bash or WSL. Both provide a POSIX
  shell, but Git Bash's `bash` is older than the one on most Linux
  systems.
- The hooks do not need any bash-specific features. Using `sh` keeps
  the surface small.

The consequence is that the hooks avoid `[[ ... ]]`, arrays, and
`$(...)` in favour of POSIX-compatible constructs. This is a small
constraint with a large portability payoff.

---

## Adding a new hook

Git supports several hooks beyond the three Forge uses. Adding a new
one follows a consistent process:

1. **Identify the trigger.** Which Git operation should run the hook?
   Common choices: `post-commit`, `post-merge`, `post-checkout`,
   `pre-rebase`.

2. **Create the hook file.** Create
   `.githooks/<hook-name>` with a `#!/usr/bin/env sh` shebang.

3. **Source the shared helpers.** Every hook begins with:

   ```sh
   repo_root=$(git rev-parse --show-toplevel)
   cd "$repo_root"
   . "$repo_root/.githooks/lib/common.sh"
   ```

4. **Write the hook body.** Use the helper functions for output.
   Invoke `task` targets for the actual checks.

5. **Make it executable.**

   ```sh
   chmod +x .githooks/<hook-name>
   ```

6. **Document it.** Add a row to the table in
   [The hooks](#the-hooks) section of this file.

7. **Test it.** Run the corresponding Git operation and verify the
   hook fires.

### Guidelines for new hooks

- **Keep it fast.** A hook that takes more than a few seconds will be
  bypassed. If the check is slow, put it in a less frequent hook.
- **Be selective.** Inspect the changed files and run only the checks
  that are relevant.
- **Report clearly.** Use `hook_ok`, `hook_fail`, and `hook_summarise`
  so the output is consistent with the existing hooks.
- **Support bypass.** Never make a hook impossible to bypass. Some
  legitimate workflows require it.

---

## Modifying a hook

Before modifying an existing hook, understand what it protects against:

- **`pre-commit`** protects against broken commits.
- **`pre-push`** protects against broken pushes.
- **`commit-msg`** protects against malformed commit messages.

Changes should preserve the spirit of the hook while improving its
mechanics. For example:

- Adding a check is fine if it is fast.
- Removing a check requires justification. The check exists because it
  caught a real problem at some point.
- Changing the selection logic requires understanding what files
  trigger which checks.

### Testing a modification

1. Modify the hook.
2. Run `task hooks:test` or `task hooks:test:push` to invoke it.
3. Verify the output is correct.
4. Perform a real Git operation to verify the hook integrates
   correctly.

Do not modify a hook without testing it on a real commit or push.
Hooks that look correct but behave unexpectedly are worse than no
hooks at all, because they erode trust in the system.

---

## Troubleshooting

### The hooks are not running

**Symptom:** You commit or push and no hook output appears.

**Diagnosis:** The hooks are not installed.

**Fix:**

```sh
task hooks:status
task hooks:install
```

### The hook fails with "task is not installed"

**Symptom:**

```text
✗ task is not installed
```

**Diagnosis:** The `task` binary is not on your `PATH`.

**Fix:** Install Task from <https://taskfile.dev/installation/>.

On macOS with Homebrew:

```sh
brew install go-task
```

On Linux, use the installer or your distribution's package manager.
The full list is at <https://taskfile.dev/installation/>.

### The hook fails with a check failure

**Symptom:**

```text
✗ Go formatting
```

**Diagnosis:** A check the hook runs has failed. The failure detail is
printed above the summary.

**Fix:** Run the failing check directly to see the full output:

```sh
task fmt:check
```

Then fix the issue. For formatting:

```sh
task fmt
```

For Markdown lint:

```sh
task lint:md:fix
```

The auto-fix tasks modify files in place. Review the diff before
committing.

### The commit-msg hook rejects a message

**Symptom:**

```text
commit-msg hook: subject does not match Conventional Commits format
```

**Diagnosis:** The commit message does not follow the format described
in [The hooks](#the-hooks) section.

**Fix:** Rewrite the message. Examples:

```text
feat(cli): add config command
fix(config): handle missing file gracefully
docs: add local hooks section to development guide
chore(deps): pin Cobra to v1.8.1
test(cli): add smoke test for unknown command
```

To amend the message without changing the commit:

```sh
git commit --amend
```

The commit-msg hook runs again on the amended message.

### The hook is too slow

**Symptom:** A commit or push takes longer than expected.

**Diagnosis:** A check that should not have run was triggered. This
happens when the selection logic in the hook is not precise enough.

**Fix:** Identify which check is slow by running the hook manually:

```sh
task hooks:test
```

If the slow check is running when it should not, the selection logic
needs adjustment. Open an issue or file a PR against the affected
hook.

### The hooks interfere with an automated workflow

**Symptom:** A script that performs `git commit` or `git push` is
blocked by a hook.

**Diagnosis:** Hooks run on every Git operation, including automated
ones.

**Fix:** For a one-off operation, use `--no-verify`:

```sh
git commit --no-verify -m "message"
```

For a script that performs many operations, install the hooks only in
interactive shells, not in the script's environment:

```sh
git config --local core.hooksPath .git/hooks
```

Remember to restore the hooks when finished:

```sh
git config --local core.hooksPath .githooks
```

The recommended long-term solution is to make the script tolerant of
hook failures, or to run the checks the hook would run and short-
circuit the hook with `--no-verify`.

### The hook produces no colour output

**Symptom:** All hook output is plain text, even in a terminal.

**Diagnosis:** Either `NO_COLOR` is set in the environment, or stdout
is not a TTY.

**Fix:** If you want colour, unset `NO_COLOR`:

```sh
unset NO_COLOR
```

If stdout is piped or redirected, colour is intentionally disabled.
This is the standard convention (see <https://no-color.org>).

---

## Uninstalling

To remove the hooks from the local clone:

```sh
task hooks:uninstall
```

Or directly:

```sh
git config --unset core.hooksPath
```

This removes the configuration that points Git at `.githooks`. It does
not delete the hooks from the repository. Re-installation is a single
command:

```sh
task hooks:install
```

Uninstalling is appropriate when:

- You are experimenting with a workflow that conflicts with the hooks.
- You are running an automated process that cannot tolerate the hooks.
- You are about to clone a fresh copy of the repository.

For everyday development, the hooks should remain installed.

---

## References

- [`Taskfile.yml`](../Taskfile.yml) — the tasks the hooks invoke.
- [`docs/development.md`](../docs/development.md) — the "Local Hooks"
  section, which is the entry point for new contributors.
- [`docs/dependency-policy.md`](../docs/dependency-policy.md) — the
  policy enforced by `verify:deps`.
- [`docs/decisions/`](../docs/decisions/) — the ADR directory, for the
  decisions that the hooks help enforce.
- [`.markdownlint.yaml`](../.markdownlint.yaml) — the rules enforced
  by `lint:md`.
- [`.editorconfig`](../.editorconfig) — the formatting conventions
  that `fmt:check` enforces for Go files.
- [Conventional Commits](https://www.conventionalcommits.org/) — the
  commit message format enforced by `commit-msg`.
- [Git hooks documentation](https://git-scm.com/docs/githooks) — the
  upstream reference for hook behaviour.
- [Task](https://taskfile.dev/) — the task runner the hooks use.
