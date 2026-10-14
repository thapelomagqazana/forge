# ADR-038: No Silent Overwrite Ever

**Status:** Proposed
**Date:** 2026-10-14
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge writes files. It writes rendered templates, generated scaffolds, updated project files, and backup copies. In every case, the target of a write may already exist: a user who runs `forge generate` twice on the same project, a project that already contains a file at the path a template wants to write, a `forge update` that rewrites a file it previously wrote. The behaviour of the write when the target exists is the entire decision. There are only two possibilities — the write succeeds, replacing the target's content; or the write fails, leaving the target untouched — and the choice between them determines whether the tool can destroy a user's work.

Before this decision, the module did not choose. `Root.Write` was a thin wrapper over `os.WriteFile`, which truncates an existing file and writes the new content. A caller who wrote to `path/to/config.yaml` and found their previous content gone had no signal: the operation succeeded, the file changed, and nothing in the return value indicated that the write had replaced rather than created. The behaviour was the standard library's default, and the standard library's default is correct for most programs; it is not correct for a foundation manager, where the target path may be a file the user authored by hand and where "the tool overwrote my config" is a bug report, not a feature.

The problem is asymmetric. A write that creates a file where none existed cannot lose anything. A write that replaces an existing file loses whatever was there. The two cases have different risk profiles, and the abstraction should not treat them the same. The standard library treats them the same because it must be general; a foundation manager can be specific. The specific behaviour is to refuse the write when the target exists, unless the caller has said explicitly that the write is meant to replace the target.

The decision had to be made before Phase 4 (render plan), Phase 12 (components), and Phase 14 (update safety) built on the abstraction, because each of those phases will produce writes whose existing-target behaviour matters. The render plan writes rendered templates into a project; a template that writes `README.md` should not silently replace the user's `README.md`. A component that writes `package.json` should not silently replace the user's `package.json`. The update mechanism that rewrites Forge-generated files has a stronger claim (it wrote the file, so it may rewrite it), but the claim is explicit in the update code, not implicit in the write primitive. Stating the primitive's policy up front lets each phase decide its own additional policy.

Related prior work: WBS 13.2.3 (the renderer output that motivated the primitive), WBS 13.5.1 (the error taxonomy that supplies `ErrExists`), WBS 13.5.2 (the atomic-write primitive whose fail-closed default this ADR ratifies).

---

## 2. Options Considered

### Option A — Mirror `os.WriteFile`: overwrite unconditionally

The write operation replaces the target's content when the target exists, and creates the file when it does not. This is the standard library's behaviour and the behaviour of most Unix utilities.

**Pros:**

- The behaviour matches the standard library, so a reader familiar with `os.WriteFile` knows what to expect.
- The operation is idempotent: writing the same content twice produces the same result, which simplifies some use cases (regenerating a file that may or may not exist).
- The caller does not have to check whether the target exists, which removes one syscall from the common path.
- The behaviour matches the pattern most CLI tools use (`cat > file`, `echo > file`), so a user's expectation is aligned.

**Cons:**

- The operation silently destroys the target's content. A user who has edited the file by hand loses their edits, and the tool reports success.
- The failure mode is silent: no log line, no warning, no return value that indicates the file existed. The user has to notice the loss after the fact.
- The abstraction has no way to distinguish "the caller intends to replace the target" from "the caller expects the target not to exist." Both call sites look identical, and the abstraction cannot tell them apart.
- A caller who wants fail-closed behaviour has to check for the target's existence before the write, which re-introduces the TOCTOU window that ADR-037's posture is designed to narrow.

### Option B — Always fail when the target exists, and provide no way to overwrite

The write operation refuses when the target exists. There is no variant that replaces. A caller who wants to overwrite must remove the target first, then write.

**Pros:**

- The failure mode is unambiguous: a write either creates a file or returns an error.
- The policy is easy to state and easy to test.
- A caller cannot accidentally overwrite, because the operation does not allow it.
- The behaviour matches `O_CREATE | O_EXCL`, which is the atomic-create primitive on every platform.

**Cons:**

- The "remove first, then write" pattern re-introduces a TOCTOU window between the two operations, which ADR-037 exists to narrow.
- The operation is not idempotent in the sense that "write the same content twice" fails the second time. A caller who wants to regenerate a file has to remove it first, which is surprising.
- The update mechanism (which legitimately wants to replace the files it wrote) has to go through remove-then-write, which is a race it should not have to accept.
- The abstraction cannot express "replace the target's content" as a single operation, which means the caller has to compose the replacement from primitives that are less safe than a single atomic operation would be.

### Option C — Two named operations: `Write` fails-closed, `WriteOverwrite` replaces

The abstraction provides two operations. `Write` refuses when the target exists, returning `ErrExists`. `WriteOverwrite` replaces the target's content atomically, using the temp-file-rename sequence. The two are named, so the caller's intent is visible at the call site. There is no boolean parameter and no configuration flag; the two behaviours are two methods.

**Pros:**

- The default is fail-closed. A caller who does not choose gets the safe behaviour.
- The intent is visible at the call site: `Write` reads as "create," `WriteOverwrite` reads as "replace."
- The overwrite is atomic (temp-file-rename), so a reader of the target sees either the old content or the new content, never a partial write. The atomicity is a property the abstraction can provide, and it does provide it.
- The update mechanism uses `WriteOverwrite`, which is the correct expression of its intent: it wrote the file, so it may rewrite it.
- The two-method design mirrors the standard library's pattern (`os.Create` vs `os.OpenFile` with `O_EXCL`), so a reader familiar with the stdlib sees the shape.
- A reviewer who reads `WriteOverwrite` at a call site knows that the caller has considered the overwrite case; a reviewer who sees `Write` knows the caller expects the target to be new.

**Cons:**

- The abstraction has two write operations, which is one more than the standard library's `os.WriteFile`. A contributor must know which one to use.
- The `WriteOverwrite` name is longer than `Write`, which is a small readability cost. (The alternative — a boolean parameter — is shorter but hides the intent.)
- A caller who wants a conditional overwrite (overwrite if the content differs, fail if it does not) has to compose the operation from `Read`, `Exists`, and `WriteOverwrite`, and the composition has its own race.
- The two operations duplicate some code: both call the pipeline, both resolve the parent, both handle the temp file. The duplication is bounded, but it exists.

### Option D — Single operation with a mode parameter

`Write(path, content, mode, overwrite bool)` — one method, one boolean. The caller passes `true` to overwrite and `false` to fail-closed.

**Pros:**

- One method, one signature, one place to look.
- The behaviour is configurable without adding a second method.
- The pattern is familiar from `os.OpenFile`, which takes flags that determine the behaviour.

**Cons:**

- The boolean is a footgun: `Write(path, content, mode, false)` and `Write(path, content, mode, true)` differ by one character, and a reviewer reading a diff cannot easily see which is which. A reader who scans the call site has to count the arguments and remember the position of the boolean.
- The boolean hides the intent. "I want to create a file" and "I want to replace a file" are two different intentions; expressing them as two methods makes the difference visible, expressing them as one method with a boolean makes it a value the reader must check.
- The boolean is not composable with future additions. A future "write if the content differs" mode would need a third value, and the boolean would become an enum, and the call sites would change. Two methods extend naturally; a boolean does not.
- The pattern has a well-known failure mode in Go: functions whose last parameter is a `bool` tend to accumulate more booleans, and the call sites become unreadable.

---

## 3. Decision

Forge adopts Option C. The abstraction provides two named write operations: `Root.Write` is fail-closed and returns an error wrapping `ErrExists` when the target exists; `Root.WriteOverwrite` replaces the target's content atomically. Both operations run the same pipeline, resolve the parent under the same symlink policy, and use the same atomic-write primitive; they differ only in whether the target's absence is checked before the write. The atomicity guarantee is the same for both: a reader of the target sees either the old content or the new content, never a partial write.

The decision rests on five assumptions:

1. **The default is the one a caller should get without thinking.** A caller who writes to a file the user may have authored should not silently replace that file. The safe behaviour is the default; the overwrite is the opt-in.
2. **The two intents are genuinely different and deserve different names.** "Create a file" and "replace a file" are two operations in a caller's mental model, and the abstraction should mirror that model. A boolean parameter collapses the two into one method and hides the difference.
3. **The atomic-write primitive is available.** `WriteOverwrite` uses the temp-file-rename-fsync sequence; without it, the overwrite would be a truncate-and-write that a concurrent reader could observe mid-write. The primitive exists (WBS 13.5.2), so the atomicity claim is honest.
4. **A caller who needs a different policy can compose one.** A caller who wants "overwrite if the content differs" reads the existing content, compares, and calls `WriteOverwrite` only if the comparison says the write is needed. The composition has a race, but the race is the caller's to accept or to close.
5. **The two-method pattern does not grow unboundedly.** The abstraction has exactly two write operations and no plan to add a third. If a third becomes necessary (for example, "write with backup"), it is added as a third named method, and the pattern remains: one intent, one name.

If assumption (3) fails on a future platform, `WriteOverwrite` degrades to truncate-and-write, and the atomicity guarantee weakens. The degradation is documented; the mitigation is to not target the platform until the primitive is available.

The decision is not provisional. The fail-closed default is the policy; a change to the default would be a new ADR superseding this one.

---

## 4. Consequences

### 4.1 Positive

- A write that does not explicitly ask to overwrite never replaces an existing file. The failure mode of "the tool overwrote my config" is eliminated for callers who use the default.
- The intent is visible at the call site. A reviewer reading `Write(path, data, mode)` sees a create; a reviewer reading `WriteOverwrite(path, data, mode)` sees a replace. The distinction is in the method name, not in an argument value.
- The overwrite is atomic. A reader of the target during a `WriteOverwrite` sees either the old content or the new content, never a partial write. This is a property that the standard library's `os.WriteFile` does not provide.
- The pattern mirrors the standard library's own (`os.Create` vs `os.OpenFile` with `O_EXCL`), so a reader familiar with the stdlib sees a familiar shape.
- The error taxonomy integrates cleanly: a `Write` that finds an existing target returns an error wrapping `ErrExists`, which is a sentinel with a stable code, a category, and a documented remediation. A caller who wants to programmatically react to the failure uses `errors.Is(err, safefs.ErrExists)`.

### 4.2 Negative

- Two methods instead of one. A contributor who is about to write a file must choose; the choice is not automatic, and a wrong choice is a bug the compiler will not catch.
- A caller who wants "overwrite only if the content differs" has to compose the check and the write themselves, and the composition has a race. The abstraction does not provide a `WriteIfChanged` operation.
- The `WriteOverwrite` name is verbose. A caller who types `WriteOverwrite` several times may be tempted to alias it, and the alias would be a second name for the same operation, which is a naming problem the abstraction does not prevent.
- A caller who has already checked for the target's existence and wants to write only when it does not exist has to do the check twice: once in their code and once inside `Write`. The duplication is the price of the fail-closed default.
- The two operations duplicate part of their implementation. The duplication is small (the pipeline call, the parent resolution, and the temp-file creation are shared through a private helper), but it exists.

### 4.3 Neutral

- The existing-file policy is documented as a decision matrix in `docs/reference/filesystem-errors.md`. The matrix covers `Write`, `WriteOverwrite`, `Mkdir`, `MkdirAll`, `Remove`, `RemoveIfExists`, `RemoveAll`, `Rename`, `RenameIfNotExists`, and `RenameOverwrite`. Every operation's behaviour for every target state is stated in one place.
- The pattern extends naturally: `Rename` is fail-closed, `RenameIfNotExists` is a synonym, and `RenameOverwrite` replaces. The three names form a family that a reader can learn as a set.
- The `Write` operation's error (`ErrExists`) is the same sentinel that `Mkdir`, `Rename`, and the other fail-closed operations use. The sentinel is the abstraction's way of saying "the target exists and the operation refused."

---

## 5. Related Requirements

- `SEC-FS-016` — the no-silent-overwrite requirement; this ADR is the decision that satisfies it.
- `FR-FS-024` — the error taxonomy requirement; the `ErrExists` sentinel is the failure the fail-closed default produces.
- `FR-FS-003` — the write operation requirement; this ADR defines the two variants the requirement covers.
- `FR-FS-004` — the atomic-write requirement; the atomicity guarantee applies to both `Write` and `WriteOverwrite`.
- `FR-FS-014` — the rename-if-not-exists requirement; the pattern for renames mirrors the pattern for writes.
- `UX-002` — the user-facing error message requirement; the `ErrExists` error names the target and the remediation.
- `NFR-API-001` — the API stability requirement; the two-method pattern is part of the frozen surface.

> The following requirements must be added to the register when WBS 1.4 is complete: `SEC-FS-027` (the fail-closed default is enforced by the API shape, not by review), `SEC-FS-028` (the overwrite variant is atomic), `UX-005` (the `ErrExists` error names the target and a remediation).

---

## 6. Related ADRs

- `ADR-035` — Two-Layer Filesystem Architecture; this ADR is a policy of the `safefs` layer.
- `ADR-037` — TOCTOU Is Narrowed, Not Closed; the atomic-write primitive that `WriteOverwrite` uses is the same one that ADR-037 names in its write-path mitigation.
- `ADR-011` — Closed Error Category Taxonomy; `ErrExists` is in the filesystem category this ADR defines.
- `ADR-013` — Error Wrapping Contract; the `ErrExists` error preserves the underlying `fs.ErrExist` while adding the sentinel.
- `ADR-018` — Exit-Code Taxonomy; a fail-closed write maps to `ExitFilesystem`, which a script can react to.

---

## 7. Notes

### Open questions

- **Should the abstraction provide `WriteIfChanged`?** A caller who wants to write only when the content differs has to compose the check and the write, with a race between them. A `WriteIfChanged` operation would close the race by comparing and writing atomically. The operation is more complex than the two current operations (it needs to read the target, compare, and write if the comparison says to), and no current consumer needs it. Deferred; tracked as a note on WBS 13.5.2.
- **Should `Mkdir` and `Rename` follow the same two-method pattern?** `Mkdir` is a no-op for an existing directory (the "already correct" case) and fails for an existing file. `Rename` is fail-closed and has a `RenameIfNotExists` synonym and a `RenameOverwrite` variant. The patterns are consistent but not identical; the differences are documented in the decision matrix. Deferred; tracked as a note on WBS 13.5.2.
- **Should `Remove` have a fail-closed default?** `Remove` currently removes the target if it exists and fails with `ErrNotFound` if it does not. The operation is "remove the file," and the caller's intent is unambiguous from the method name. A future variant (`RemoveIfExists`) could be added for the "remove if present" case, which is what `RemoveAll` provides for trees. Deferred; tracked as a note on WBS 13.5.2.

### Follow-up decisions anticipated

- **ADR-NNN — Atomic Write Semantics.** The temp-file-rename-fsync sequence and its durability guarantees are the subject of a separate ADR (see WBS 13.5.2).
- **ADR-NNN — Overwrite-With-Confirmation.** The interaction between the filesystem layer's fail-closed default and the render plan's overwrite-with-confirmation policy is a decision of its own. The filesystem layer provides the primitives; the render plan composes them with a user prompt. The follow-up ADR would state the composition explicitly.
- **ADR-NNN — Existing-File Policy Matrix.** The full matrix of operations and target states is documented in the filesystem-errors reference; the ADR would ratify the matrix as a decision.

### External references

- Go standard library: `os.WriteFile`, `os.Create`, `os.OpenFile` with `O_EXCL`
- CWE-379 — Creation of Temporary File in Directory with Insecure Permissions (the class of failure the atomic-write primitive avoids)
- `docs/reference/filesystem-errors.md` — the existing-file policy matrix this ADR ratifies
- `internal/filesystem/safefs/atomic.go` — the implementation of `WriteAtomic` and `WriteAtomicOverwrite`
- `internal/filesystem/safefs/errors.go` — the `ErrExists` sentinel
