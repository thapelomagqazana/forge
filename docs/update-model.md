# Update / Evolution Model

- **Document type:** Specification
- **Status:** Draft
- **Version:** 0.1.0
- **Author:** @thapelomagqazana
- **Created:** 2026-10-09
- **Last Updated:** 2026-10-09
- **Supersedes:** —
- **Superseded by:** —

---

## 1. Purpose

This specification defines how Forge updates an existing repository
when its foundation changes.

`forge update` is the most technically demanding command in Forge. It
must reconcile three inputs:

- The **original** Forge output
- The **developer's** modifications
- The **new** Forge output

The result must preserve developer intent while applying foundation
changes. Getting this wrong destroys developer work.

This specification defines the update problem, the ownership model,
the change-tracking model, the update strategy, the conflict model,
dry-run semantics, rollback, and update compatibility.

This specification is written during Phase 1 even though `forge
update` is implemented in Phase 14. The design must be settled before
implementation because the ownership and change-tracking models
affect `forge new` and `forge check`.

---

## 2. Scope

**In scope:**

- The update problem statement
- Ownership model
- Change-tracking model
- Update strategy comparison
- Conflict model
- Dry-run semantics
- Rollback semantics
- Update compatibility matrix
- Safety invariants

**Out of scope:**

- The update algorithm implementation (Phase 14)
- Registry distribution (see
  [`docs/registry-spec.md`](./registry-spec.md), future)
- Blueprint schema (see
  [`docs/blueprint-spec.md`](./blueprint-spec.md))
- Template format (see
  [`docs/template-spec.md`](./template-spec.md))
- Component format (see
  [`docs/component-spec.md`](./component-spec.md))
- Filesystem boundary (see
  [`docs/security-model.md`](./security-model.md))
- CLI command surface (see
  [`docs/cli-ux-spec.md`](./cli-ux-spec.md))

---

## 3. The Update Problem

### 3.1 The Three Inputs

Every update has three inputs:

```text
┌─────────────────────────────┐
│  BASE                       │
│  What Forge originally      │
│  produced.                  │
└──────────────┬──────────────┘
               │
               │ developer changed it
               ▼
┌─────────────────────────────┐
│  CURRENT                    │
│  What the repository        │
│  contains right now.        │
└──────────────┬──────────────┘
               │
               │ Forge released a new version
               ▼
┌─────────────────────────────┐
│  TARGET                     │
│  What Forge would produce   │
│  today with the new         │
│  foundation version.        │
└─────────────────────────────┘
```

The update algorithm computes the result:

```text
RESULT = merge(BASE, CURRENT, TARGET)
```

### 3.2 Why This Is Hard

The update problem is hard because:

1. **The developer may have changed anything.**
   Between BASE and CURRENT, the developer may have edited, renamed,
   moved, or deleted any file.

2. **Forge may have changed anything.**
   Between BASE and TARGET, Forge may have edited, renamed, moved,
   or deleted any file.

3. **The changes may overlap.**
   The developer and Forge may both have changed the same file, the
   same line, or the same logical concept.

4. **The changes may not be textual.**
   A file may have been restructured (e.g., YAML keys reordered)
   without changing its meaning. Naive textual diffs would report
   spurious conflicts.

5. **The developer may have legitimate reasons for divergence.**
   The developer's changes are not necessarily errors to be
   corrected. They may reflect local requirements, exceptions, or
   intentional divergence.

### 3.3 The Update Principle

> **Forge never silently destroys developer work.**

If Forge cannot confidently merge a change, it must:

1. Stop
2. Report the conflict
3. Ask the developer to resolve it

Forge **never**:

- Overwrites a developer modification without explicit consent
- Discards a developer-added file
- Silently applies a change that conflicts

This is a design invariant. It cannot be overridden by any flag
except `--force`, which is reserved for exceptional use.

### 3.4 What Update Is Not

Update is **not**:

- A Git operation (Forge complements Git, does not replace it)
- A full re-generation (that would discard developer changes)
- A code refactoring tool (Forge operates at the file level, not the
  AST level)
- A guarantee of semantic preservation (Forge preserves text where
  possible)

---

## 4. Ownership Model

Every file in a Forge-managed repository has an **owner**. Ownership
determines how updates treat the file.

### 4.1 Ownership Values

| Owner | Meaning |
|-------|---------|
| `forge` | Forge created this file and no developer has modified it |
| `developer` | The developer created or modified this file |
| `shared` | Both Forge and the developer have modified this file |

Ownership is **per-file**, not per-line. Line-level ownership is
considered for future phases.

### 4.2 Ownership Recording

Ownership is recorded in the repository's Forge state:

```text
.forge/
└── state.yaml
```

Example:

```yaml
schema: 1
foundation:
  blueprint: python-api
  blueprint_version: 2.4.0
  template: python-fastapi
  template_version: 1.0.0

files:
  - path: README.md
    owner: forge
    generated_hash: sha256:abc123...
    current_hash: sha256:abc123...
    modified: false

  - path: src/payments_api/main.py
    owner: shared
    generated_hash: sha256:def456...
    current_hash: sha256:ghi789...
    modified: true

  - path: src/payments_api/custom.py
    owner: developer
    # No generated_hash (not generated by Forge)
```

Fields:

| Field | Meaning |
|-------|---------|
| `path` | Path relative to repository root |
| `owner` | One of `forge`, `developer`, `shared` |
| `generated_hash` | Hash of the file when Forge last generated it |
| `current_hash` | Hash of the file when Forge last observed it |
| `modified` | True if `current_hash != generated_hash` |

### 4.3 Ownership Transitions

Ownership changes as the developer interacts with the repository:

| Event | Transition |
|-------|------------|
| Forge creates a file | → `forge` |
| Developer edits a Forge-owned file | → `shared` |
| Developer creates a new file | → `developer` |
| Developer deletes a Forge-owned file | File is removed from state |
| Update rewrites a shared file with consent | → `forge` |

### 4.4 Ownership by Component

For component-managed files, ownership also records the component:

```yaml
files:
  - path: compose.postgres.yaml
    owner: forge
    component: postgres@1.0.0
    generated_hash: sha256:...
    current_hash: sha256:...
```

Component ownership is used when a component is updated or removed.

### 4.5 Ownership of Directories

Directories are not tracked as owned objects. Only files are tracked.

Directory creation is implied by file creation. Directory removal is
implied by file removal.

### 4.6 Ownership of Added Files

When a developer adds a new file that is not part of the foundation,
it is not recorded in the state. Its ownership is implicitly
`developer`.

If a Forge update would create a file with the same path, Forge
refuses to overwrite it without consent.

### 4.7 Ownership of Deleted Files

When a developer deletes a Forge-owned file:

- The file is removed from the state
- The update plan records the deletion as a developer change
- If the new foundation still produces this file, the update will
  recreate it (with confirmation)

---

## 5. Change-Tracking Model

Forge must track changes to compute safe updates.

### 5.1 What Forge Tracks

| Tracked | Purpose |
|---------|---------|
| **Base snapshot** | The exact state of files as Forge generated them |
| **Current state** | The exact state of files as they exist now |
| **Target state** | The exact state of files as the new foundation would produce them |
| **File hashes** | Fast comparison of file content |
| **Template version** | Which template produced the current state |
| **Blueprint version** | Which blueprint is currently in effect |
| **Component versions** | Which components are installed and their versions |
| **Forge version** | Which Forge version last wrote the state |

### 5.2 Base Snapshot

The base snapshot is the set of files as Forge last wrote them.

**Storage:**

Forge stores the base snapshot as a set of hashes in
`.forge/state.yaml`. The full content of generated files is not
stored; only hashes.

**Rationale:**

Storing full content would double the size of every repository and
would be redundant (the content can be regenerated from the template
version).

**Reconstruction:**

If the base content is needed (e.g., for a three-way merge), Forge
regenerates it from:

- The recorded template version
- The recorded blueprint
- The recorded component versions

This requires the template version to still be available (bundled,
cached, or from a registry).

### 5.3 Current State

The current state is the actual content of files in the repository.

**Storage:**

The current state is not stored; it is read from the filesystem when
needed.

**Hash computation:**

The current state's hashes are computed on demand.

### 5.4 Target State

The target state is what the new foundation would produce.

**Storage:**

The target state is generated in memory (or in a temporary directory)
during the update plan, but not stored.

**Hash computation:**

The target state's hashes are computed from the regenerated content.

### 5.5 Hash Algorithm

Forge uses SHA-256 for file hashes.

- Content hash: hash of the file's bytes
- Path hash: not used

Hashes are stored as strings with the format:

```text
sha256:<hex digest>
```

### 5.6 Change Categories

For each file, Forge computes three states:

| State | Meaning |
|-------|---------|
| `UNCHANGED` | BASE == CURRENT == TARGET |
| `DEVELOPER_MODIFIED` | BASE != CURRENT, CURRENT == TARGET |
| `FORGE_MODIFIED` | BASE == CURRENT, CURRENT != TARGET |
| `BOTH_MODIFIED` | BASE != CURRENT, CURRENT != TARGET, CURRENT != BASE |

Plus:

| State | Meaning |
|-------|---------|
| `ADDED_BY_DEVELOPER` | File exists in CURRENT, not in BASE |
| `ADDED_BY_FORGE` | File exists in TARGET, not in BASE, not in CURRENT |
| `DELETED_BY_DEVELOPER` | File exists in BASE, not in CURRENT |
| `DELETED_BY_FORGE` | File exists in BASE, not in TARGET |

### 5.7 Change Tracking Table

| BASE | CURRENT | TARGET | Category |
|------|---------|--------|----------|
| A | A | A | UNCHANGED |
| A | B | A | DEVELOPER_MODIFIED |
| A | A | B | FORGE_MODIFIED |
| A | B | B | DEVELOPER_MODIFIED (developer moved to new) |
| A | B | C | BOTH_MODIFIED |
| — | A | — | ADDED_BY_DEVELOPER |
| — | — | A | ADDED_BY_FORGE |
| A | — | A | DELETED_BY_DEVELOPER |
| A | A | — | DELETED_BY_FORGE |
| A | — | B | DELETED_BY_DEVELOPER + FORGE_MODIFIED |

Where A, B, C are distinct content hashes.

### 5.8 Template and Component Version Tracking

The state records which template and component versions produced the
current state:

```yaml
foundation:
  blueprint: python-api
  blueprint_version: 2.4.0
  template: python-fastapi
  template_version: 1.0.0
  components:
    - postgres@1.0.0
    - docker@2.0.0
```

This allows Forge to:

- Regenerate the base state
- Compute the target state
- Verify that the target state is compatible with the current state

### 5.9 Forge Version Tracking

The state records the Forge version that last wrote it:

```yaml
forge_version: "0.1.0"
last_updated: "2026-10-09T12:00:00Z"
```

This allows Forge to:

- Detect when the state was written by an older version
- Apply migrations if the state format changed
- Warn if the state is too old to update safely

---

## 6. Update Strategy

Forge considers four update strategies:

1. **Replace** — overwrite with the target
2. **Patch** — apply a diff
3. **Three-way merge** — combine BASE, CURRENT, TARGET
4. **Regenerate** — discard CURRENT and produce TARGET

Each is evaluated below.

### 6.1 Strategy 1: Replace

**Approach:** Replace every file with its TARGET version.

**Pros:**

- Simple
- Deterministic
- No conflict detection needed

**Cons:**

- Destroys all developer modifications
- Unacceptable for any developer workflow
- Violates the update principle (§ 3.3)

**Verdict:** **Rejected.** Not safe.

### 6.2 Strategy 2: Patch

**Approach:** Compute a textual diff between BASE and TARGET, then
apply it to CURRENT.

**Pros:**

- Well-understood (similar to `git apply`)
- Handles small changes efficiently
- Preserves developer modifications outside the diff

**Cons:**

- Fails if the developer modified a line adjacent to a change
- Does not handle file renames well
- Requires the base content to be available (or regenerated)
- Cannot detect semantic conflicts
- Silent corruption if a diff applies to the wrong location

**Verdict:** **Rejected as primary strategy.** Insufficient for
safe updates.

### 6.3 Strategy 3: Three-Way Merge

**Approach:** For each file, perform a three-way merge:

```text
BASE
 ├── CURRENT
 └── TARGET
    ↓
RESULT
```

**Pros:**

- Well-understood (same as Git's merge)
- Preserves developer modifications where possible
- Detects conflicts where changes overlap
- Handles file additions and deletions

**Cons:**

- Requires BASE content
- Line-based, so not semantically aware
- Can produce spurious conflicts on reformatting
- Cannot merge non-text files (or merges them poorly)

**Verdict:** **Adopted as primary strategy.** Best balance of safety
and correctness.

### 6.4 Strategy 4: Regenerate

**Approach:** Delete all Forge-owned files, regenerate them from the
new foundation.

**Pros:**

- Guarantees a consistent result
- No conflict detection needed

**Cons:**

- Destroys all developer modifications to Forge-owned files
- Violates the update principle (§ 3.3)

**Verdict:** **Rejected.** Only acceptable with `--force` and explicit
warning.

### 6.5 Strategy Summary

| Strategy | Preserves Dev Mods | Detects Conflicts | Requires Base | Verdict |
|----------|-------------------|-------------------|---------------|---------|
| Replace | No | N/A | No | Rejected |
| Patch | Partial | No | Yes | Rejected |
| Three-way merge | Yes | Yes | Yes | **Adopted** |
| Regenerate | No | N/A | No | Rejected |

### 6.6 Adopted Strategy Details

Forge uses a hierarchical strategy:

1. **If CURRENT == TARGET:** No change needed. Skip.
2. **If BASE == CURRENT:** Developer has not modified the file. Use
   TARGET (safe replace).
3. **If BASE == TARGET:** Forge has not modified the file. Keep
   CURRENT (preserve developer changes).
4. **Otherwise:** All three differ. Perform a three-way merge.
   - If the merge succeeds: use the merged result.
   - If the merge fails: report a conflict.

This hierarchy minimizes unnecessary merges.

### 6.7 Special Cases

**Binary files:**

Three-way merge is not meaningful for binary files. Forge:

- If BASE == CURRENT (developer didn't modify): use TARGET
- If BASE == TARGET (Forge didn't modify): keep CURRENT
- Otherwise: report conflict

**Deleted files:**

| Case | Behaviour |
|------|-----------|
| BASE == CURRENT, TARGET is absent | Forge deleted it. Delete (with confirmation). |
| BASE == TARGET, CURRENT is absent | Developer deleted it. Keep deleted. |
| BASE is present, both CURRENT and TARGET are absent | Already deleted. No action. |
| BASE is present, CURRENT is present, TARGET is absent | Forge deleted it, developer kept it. Conflict. |
| BASE is present, CURRENT is absent, TARGET is present | Developer deleted it, Forge modified it. Conflict. |

**Added files:**

| Case | Behaviour |
|------|-----------|
| TARGET adds a file, CURRENT has no such file | Add it. |
| TARGET adds a file, CURRENT has a file with the same path | Conflict. |

**Renamed files:**

Rename detection is attempted via content similarity (same hash at
different paths):

- If a file was renamed in TARGET and unchanged in CURRENT: rename.
- If a file was renamed in CURRENT and unchanged in TARGET: keep the
  rename.
- If a file was renamed in both: conflict.

Rename detection is heuristic and best-effort.

---

## 7. Conflict Model

A conflict occurs when Forge cannot determine a safe merge.

### 7.1 Conflict Types

| Type | Meaning |
|------|---------|
| **Content conflict** | BASE, CURRENT, TARGET all differ at the same location |
| **Delete-modify conflict** | One side deleted, the other modified |
| **Modify-delete conflict** | Same as above, opposite direction |
| **Add-add conflict** | Both sides added a file at the same path |
| **Rename conflict** | Renames overlapped |
| **Permission conflict** | File permissions changed on both sides |
| **Binary conflict** | Binary file modified on both sides |
| **Structure conflict** | Directory-level conflict (one side expects a dir, the other a file) |

### 7.2 Conflict Detection

For each file, Forge:

1. Computes BASE, CURRENT, TARGET hashes
2. Applies the change-tracking table (§ 5.7)
3. If the category is `BOTH_MODIFIED`, performs a three-way merge
4. If the merge reports hunks that could not be resolved, a conflict
   exists

### 7.3 Conflict Reporting

Conflicts are reported with:

- File path
- Conflict type
- The base content (for reference)
- The current content
- The target content
- The developer's changes
- The Forge changes

Example:

```text
✗ Conflict detected

File: Dockerfile
Type: Content conflict

Your changes (CURRENT):
  Line 20: RUN go build -o /app ./cmd/server

Forge's changes (TARGET):
  Line 20: RUN go build -o /app -trimpath ./cmd/server

Base (what Forge originally produced):
  Line 20: RUN go build -o /app ./cmd/server

Resolution options:
  1. Keep your version:
       RUN go build -o /app ./cmd/server
  2. Use Forge's version:
       RUN go build -o /app -trimpath ./cmd/server
  3. Merge manually
  4. Skip this file

Choose: [1/2/3/4]
```

### 7.4 Conflict Resolution

Forge does **not** resolve conflicts automatically. The developer
must choose.

In interactive mode, Forge prompts for each conflict.

In non-interactive mode:

- Forge reports all conflicts
- Exits with code 5 (conflict)
- Does not modify any files

The developer runs `forge update --conflicts` to see the conflicts
and `forge update` (interactively) to resolve them.

### 7.5 Conflict Persistence

Conflicts do not persist across runs. Each `forge update` run
recomputes conflicts from the current state.

If a developer abandons a resolution, the next run detects the same
conflicts.

### 7.6 Conflict Markers

Forge does **not** write conflict markers into files (unlike Git's
`<<<<<<<`/`=======`/`>>>>>>>` markers).

Reasons:

- Conflict markers in source files break builds and tests
- Developers frequently forget to remove them
- Forge preserves files as-is and reports conflicts separately

If a developer wants Git-style markers, they should use Git's merge
tooling after Forge reports the conflict.

### 7.7 Conflict Prevention

Conflicts can be reduced by:

- Developers keeping customizations localized
- Foundation authors avoiding unnecessary churn
- Forge providing structured extension points (see Component Spec
  § 10)

Conflict prevention is a documentation concern more than a
mechanism.

---

## 8. Dry-Run

`forge update --dry-run` shows what would happen without modifying
anything.

### 8.1 Dry-Run Guarantees

Dry-run:

- Does not write any files
- Does not modify `.forge/state.yaml`
- Does not create backups
- Does not modify Git state
- Produces the same output as a real update, minus the mutations

### 8.2 Dry-Run Output

```text
$ forge update --dry-run

Forge Update Preview
────────────────────

Foundation:
  python-api@2.4.0 → python-api@2.5.0
  python-fastapi@1.0.0 → python-fastapi@1.1.0

Changes from Forge:
  + .github/workflows/security.yml
  ~ pyproject.toml
  ~ Dockerfile

Your changes preserved:
  = src/payments_api/main.py
  = src/payments_api/custom.py
  = README.md

Potential conflicts:
  ! Dockerfile

Files added:      1
Files modified:   2
Files preserved:  3
Conflicts:        1

No files have been changed.

Run `forge update --conflicts` to inspect conflicts.
Run `forge update` to apply the update.
```

### 8.3 Dry-Run with JSON

```json
{
  "schemaVersion": "1",
  "command": "update",
  "dryRun": true,
  "foundation": {
    "from": {
      "blueprint": "python-api",
      "blueprintVersion": "2.4.0",
      "template": "python-fastapi",
      "templateVersion": "1.0.0"
    },
    "to": {
      "blueprint": "python-api",
      "blueprintVersion": "2.5.0",
      "template": "python-fastapi",
      "templateVersion": "1.1.0"
    }
  },
  "changes": {
    "added": [".github/workflows/security.yml"],
    "modified": ["pyproject.toml", "Dockerfile"],
    "preserved": [
      "src/payments_api/main.py",
      "src/payments_api/custom.py",
      "README.md"
    ],
    "deleted": []
  },
  "conflicts": [
    {
      "path": "Dockerfile",
      "type": "content"
    }
  ],
  "summary": {
    "added": 1,
    "modified": 2,
    "preserved": 3,
    "deleted": 0,
    "conflicts": 1
  }
}
```

### 8.4 Dry-Run Determinism

Dry-run produces the same result as a real update except that no
mutations occur. If a real update would fail, dry-run reports the
same failure.

---

## 9. Rollback

Every update is reversible.

### 9.1 Backup Before Update

Before modifying any file, Forge creates a backup:

```text
.forge/
└── backups/
    └── 2026-10-09T12-00-00Z/
        ├── manifest.yaml
        ├── state.yaml
        └── files/
            ├── Dockerfile
            ├── pyproject.toml
            └── ...
```

The `manifest.yaml` records what was changed:

```yaml
update:
  from:
    blueprint: python-api@2.4.0
    template: python-fastapi@1.0.0
  to:
    blueprint: python-api@2.5.0
    template: python-fastapi@1.1.0
  changes:
    added: [".github/workflows/security.yml"]
    modified: ["pyproject.toml", "Dockerfile"]
    deleted: []
```

The `state.yaml` is a copy of the state before the update.

The `files/` directory contains the previous content of every file
that was modified or deleted.

### 9.2 Atomic Update

The update is applied atomically where practical:

1. Write all new files to a staging area
2. Verify the staging area
3. Apply the staging area to the target directory
4. Update the state
5. Clean up the staging area

If any step fails, Forge rolls back.

On platforms that do not support atomic directory replacement, Forge
applies changes file-by-file with a rollback log.

### 9.3 Automatic Rollback

If the update fails partway through (e.g., a file write fails),
Forge automatically rolls back:

1. Read the backup manifest
2. Restore each modified file from the backup
3. Remove each added file
4. Restore deleted files
5. Restore the state file
6. Report the failure

### 9.4 Manual Rollback

`forge update --rollback` restores the repository to the state before
the last update.

```text
$ forge update --rollback

Rolling back to: 2026-10-09T12-00-00Z

Restored:
  pyproject.toml
  Dockerfile

Removed:
  .github/workflows/security.yml

State restored.

Result: SUCCESS
```

### 9.5 Rollback Scope

Rollback restores:

- Files modified by the update
- Files added by the update
- Files deleted by the update
- `.forge/state.yaml`

Rollback does **not** restore:

- Developer changes made after the update
- Git state
- Other backups

If the developer made changes after the update, rollback would
overwrite them. Forge warns before rolling back in this case.

### 9.6 Backup Retention

Forge retains:

- The most recent backup (always)
- Up to N previous backups (default: 5, configurable)

Older backups are deleted automatically.

### 9.7 Backup Exclusion from Git

`.forge/backups/` is added to `.gitignore` when `forge new` or `forge
init` runs. Backups are not committed.

---

## 10. Update Compatibility

Not every combination of versions is compatible. Forge refuses to
update across incompatible versions.

### 10.1 Compatibility Dimensions

Update compatibility depends on:

| Dimension | Meaning |
|-----------|---------|
| Forge version | The version of the running Forge binary |
| Blueprint schema version | The version of the Blueprint schema |
| Blueprint instance version | The version of the specific Blueprint |
| Template version | The version of the template |
| Component versions | The versions of installed components |
| State schema version | The version of `.forge/state.yaml` |

### 10.2 Compatibility Matrix

| From | To | Compatible | Notes |
|------|----|-----------| ------|
| Same version | Same version | Yes | No-op |
| Patch (1.0.0 → 1.0.1) | Higher patch | Yes | Safe |
| Minor (1.0.0 → 1.1.0) | Higher minor | Yes | Additive changes |
| Major (1.0.0 → 2.0.0) | Higher major | Yes, with caution | Breaking changes may require manual resolution |
| Lower version | Higher version | Yes | Standard forward update |
| Higher version | Lower version | No | Downgrades are not supported |

### 10.3 Cross-Version Compatibility

The following combinations are checked:

- **Forge version:** Must support the state schema version
- **Blueprint schema:** Must support both the old and new Blueprint
- **Blueprint instance:** Must be resolvable from both old and new
- **Template:** Must be resolvable from both old and new
- **Components:** All components must be resolvable in both versions

If any check fails, the update is refused.

### 10.4 Compatibility Errors

Example: downgrade attempt.

```text
✗ Update refused

Reason: Downgrades are not supported.

Current:  python-fastapi@1.1.0
Target:   python-fastapi@1.0.0

Forge cannot downgrade a foundation. If you need to revert,
use `forge update --rollback` to restore the previous update,
or manually edit forge.yaml.
```

Example: unavailable target.

```text
✗ Update refused

Reason: Target version is not available.

Template: python-fastapi
Requested: 2.0.0
Available: 1.0.0, 1.1.0, 1.2.0

Run `forge template list` to see available templates,
or install the missing version.
```

### 10.5 Partial Updates

Forge does not support partial updates within a single run:

- Either the full update succeeds (with conflicts resolved)
- Or the update is rolled back entirely

This ensures the repository is always in a consistent state.

### 10.6 Multi-Component Updates

If multiple components are being updated, they are updated together:

```text
Foundations to update:
  python-fastapi@1.0.0 → 1.1.0
  postgres@1.0.0 → 1.1.0
  docker@2.0.0 → 2.1.0
```

All updates are planned together, applied together, and rolled back
together if any fail.

---

## 11. Update Algorithm

The update algorithm is described here for completeness. The
implementation is Phase 14.

### 11.1 Algorithm Outline

```text
1. Load state from .forge/state.yaml
2. Resolve target foundation version(s)
3. Compute target file set (regenerate from target foundation)
4. Compute file-level changes:
   For each path in union(BASE, CURRENT, TARGET):
     a. Classify by change-tracking table
     b. If BOTH_MODIFIED, attempt three-way merge
     c. If merge succeeds, add to plan
     d. If merge fails, add to conflicts
5. Report plan (dry-run if requested)
6. If conflicts exist and not interactive, exit with code 5
7. If interactive, prompt for conflict resolution
8. Back up current state
9. Apply changes atomically
10. Verify result (run forge check)
11. If verification fails, roll back
12. Clean up backups older than N
13. Report success
```

### 11.2 Deterministic Ordering

The update algorithm processes files in deterministic order:

1. By category (deletes, then modifies, then adds)
2. Within each category, by path (lexicographic)

This ensures reproducible results.

### 11.3 Idempotency

Running `forge update` twice with no changes:

- First run: applies the update
- Second run: no changes detected, no-op

Running `forge update` after a partial update:

- Forge detects the current state
- Computes the correct target
- Applies only the remaining changes

### 11.4 Failure Modes

| Failure | Behaviour |
|---------|-----------|
| State file missing | Refuse; suggest `forge init` |
| State file corrupt | Refuse; suggest recovery (§ Security Model § 10) |
| Target foundation unavailable | Refuse; report which version is missing |
| Target incompatible with current | Refuse; report incompatibility |
| Conflict detected (non-interactive) | Report; exit 5 |
| Conflict resolution refused | Abort; no changes |
| File write fails | Roll back; report error |
| Verification fails | Roll back; report error |

---

## 12. Safety Invariants

The following invariants must hold for every update.

### 12.1 Never Destroy Developer Work

If the developer has modified a file and Forge's update would change
the same file, the update either:

- Merges the changes safely
- Or reports a conflict

Forge never silently overwrites.

### 12.2 Never Leave the Repository in an Inconsistent State

If the update fails partway through, Forge rolls back to the
pre-update state.

### 12.3 Never Modify Files Outside the Target Directory

All updates respect the filesystem boundary (see
[`docs/security-model.md`](./security-model.md) § 5).

### 12.4 Never Log Secrets

Update output never includes secret values (see
[`docs/security-model.md`](./security-model.md) § 8).

### 12.5 Always Show What Will Change

Every update supports `--dry-run`. `--dry-run` shows the complete
plan without making changes.

### 12.6 Never Require Network Access for Local Updates

Local updates (between local versions) do not require network access.
Registry updates (Phase 16) may require network access, but this is
explicit.

### 12.7 Never Fail Silently

If an update cannot proceed, Forge reports why and exits with a
non-zero code.

---

## 13. Examples

### 13.1 Simple Update

Scenario: Developer has not modified any files. Forge has updated
the template.

```text
$ forge update --dry-run

Forge Update Preview
────────────────────
python-fastapi@1.0.0 → python-fastapi@1.1.0

Changes from Forge:
  ~ pyproject.toml
  ~ Dockerfile

Your changes preserved: none
Conflicts: none

No files have been changed.

$ forge update

Forge Update
────────────
✓ Backup created
✓ Files updated
✓ State updated
✓ Verification passed

Updated: python-fastapi@1.0.0 → python-fastapi@1.1.0
Result: SUCCESS
```

### 13.2 Update with Developer Modifications

Scenario: Developer has modified `README.md`, which Forge also wants
to change.

```text
$ forge update --dry-run

Forge Update Preview
────────────────────
python-fastapi@1.0.0 → python-fastapi@1.1.0

Changes from Forge:
  ~ README.md
  ~ pyproject.toml

Your changes preserved:
  = README.md (will be merged)

Conflicts: none

$ forge update

Forge Update
────────────
✓ Backup created
✓ README.md merged (your changes preserved)
✓ pyproject.toml updated
✓ State updated
✓ Verification passed

Result: SUCCESS
```

### 13.3 Update with Conflict

Scenario: Developer and Forge both changed the same line in
`Dockerfile`.

```text
$ forge update --dry-run

Forge Update Preview
────────────────────
python-fastapi@1.0.0 → python-fastapi@1.1.0

Changes from Forge:
  ~ Dockerfile

Conflicts:
  ! Dockerfile

No files have been changed.

Run `forge update --conflicts` to inspect conflicts.

$ forge update --conflicts

Conflict: Dockerfile
Type: Content conflict

Your version:
  RUN go build -o /app ./cmd/server

Forge's version:
  RUN go build -o /app -trimpath ./cmd/server

Base:
  RUN go build -o /app ./cmd/server

Choose:
  [k] Keep your version
  [f] Use Forge's version
  [m] Merge manually
  [s] Skip this file
  [a] Abort update

Choice: k

$ forge update

Forge Update
────────────
✓ Backup created
✓ Dockerfile resolved (kept your version)
✓ pyproject.toml updated
✓ State updated
✓ Verification passed

Result: SUCCESS
```

### 13.4 Rollback

Scenario: Update succeeded but broke the build.

```text
$ forge update --rollback

Forge Rollback
──────────────
Restoring from: 2026-10-09T12-00-00Z

Restored:
  pyproject.toml
  Dockerfile

Removed:
  (none)

State restored to python-fastapi@1.0.0

Result: SUCCESS
```

---

## 14. Relationship to Other Specifications

| Specification | Relationship |
|---------------|--------------|
| [Blueprint](./blueprint-spec.md) | Defines the foundation that updates target |
| [forge.yaml](./forge-yaml-spec.md) | Source of truth for the current foundation |
| [Template](./template-spec.md) | Source of the target content |
| [Component](./component-spec.md) | Updates may change components |
| [Validation](./validation-spec.md) | Updates trigger validation before commit |
| [Security Model](./security-model.md) | Update respects boundary and overwrite policies |
| [CLI UX Spec](./cli-ux-spec.md) | Defines `forge update` and exit code 5 |
| [Architecture](./architecture.md) | Defines the Update Engine module |

---

## 15. Open Questions

The following questions remain open and should be resolved before
implementation:

- Should Forge support **partial updates** (a subset of files)?
- Should Forge support **selective updates** (skip a specific file)?
- Should Forge support **interactive conflict resolution in the CLI**
  (current design) or defer to an editor?
- Should Forge store the full base content (rather than just hashes)
  to avoid regeneration?
- Should Forge use a **three-way merge library** or implement one?
- Should Forge handle **renames** better (e.g., with rename
  detection, similarity thresholds)?
- Should Forge support **partial rollbacks** (restore only specific
  files)?
- Should Forge offer a **merge strategy option** (e.g., prefer
  developer, prefer Forge, ask always)?
- Should Forge integrate with **Git's merge tooling** for conflicts?
- Should Forge detect and refuse updates when the repository has
  uncommitted Git changes?
- Should Forge support **updates from local templates** (not just
  registry templates)?
- How does Forge handle updates when the developer has renamed a
  Forge-owned file?
- Should the update algorithm be **pluggable** (different strategies
  for different file types)?
- Should Forge update **`.forge/state.yaml` schema** across versions?
- Should Forge provide a **diff view** for each file before
  applying?

These questions will be addressed in Phase 2 and refined as Phase 14
approaches.

---

## 16. Status

**Draft.**

This specification is under active development during Phase 1. It
becomes **Approved** when:

- The ownership model is frozen
- The change-tracking model is tested
- The three-way merge strategy is validated with real examples
- The conflict model covers all realistic cases
- Dry-run and rollback are tested
- Compatibility rules are confirmed
- Safety invariants are documented and verifiable
- Open questions have been resolved or explicitly deferred
- The specification has been reviewed for consistency with
  [`docs/security-model.md`](./security-model.md) § 6 and
  [`docs/forge-yaml-spec.md`](./forge-yaml-spec.md) § 8
