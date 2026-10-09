# forge.yaml Specification

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

This specification defines `forge.yaml`, the file that records a
repository's declared relationship with Forge.

`forge.yaml` is the persistent metadata that connects a repository to
its engineering foundation. It answers:

- Which Blueprint does this repository follow?
- Which version of the Blueprint does it conform to?
- Which Forge components are part of its foundation?
- Which Forge version created or last validated it?
- What is the source of truth if `forge.yaml` and repository state
  disagree?

Without `forge.yaml`, Forge has no memory of a repository's
foundation. Every subsequent command — `forge validate`,
`forge check`, `forge diff`, `forge update` — depends on this file.

---

## 2. Scope

**In scope:**

- Purpose and role of `forge.yaml`
- File schema
- Required and optional fields
- Ownership and editing rules
- Source-of-truth rules
- Migration strategy
- Corruption handling
- Compatibility with older and newer Forge versions
- Relationship to Blueprint, Template, and Component

**Out of scope:**

- Blueprint schema (see
  [`docs/blueprint-spec.md`](./blueprint-spec.md))
- Template format (see
  [`docs/template-spec.md`](./template-spec.md))
- Component format (see
  [`docs/component-spec.md`](./component-spec.md))
- Update algorithm (see
  [`docs/update-model.md`](./update-model.md))

---

## 3. Metadata Purpose

### 3.1 Why `forge.yaml` Exists

`forge.yaml` exists to record the **declared engineering foundation**
of a repository in a form that:

- Survives across Forge sessions
- Is human-readable and version-controlled
- Is machine-verifiable
- Supports validation, drift detection, and evolution

Without `forge.yaml`, Forge would:

- Have no way to know which foundation a repository was created with
- Have no baseline for detecting drift
- Have no metadata to drive `forge update`
- Be unable to distinguish "never had a foundation" from "foundation
  was removed"

### 3.2 What `forge.yaml` Records

`forge.yaml` records five kinds of information:

| Category | Example |
|----------|---------|
| **Identity** | Repository name, project type |
| **Foundation reference** | Which Blueprint, which version |
| **Components** | Which engineering components are enabled |
| **Policies** | Which foundation requirements apply |
| **Metadata** | Forge version, creation timestamp, last validation |

### 3.3 What `forge.yaml` Does Not Record

`forge.yaml` does **not** record:

- The contents of generated files
- The hash of every file (see base state in
  [`docs/update-model.md`](./update-model.md))
- Repository-specific configuration unrelated to the foundation
- Secrets, credentials, or environment-specific values
- Any information that would need to be regenerated per machine

`forge.yaml` is portable across machines and clones. Anything that
would break this property belongs elsewhere.

### 3.4 Relationship to Other Files

```text
┌──────────────────┐
│   BLUEPRINT      │  Declares what a project is.
│  (portable)      │  Not in the repository by default.
└────────┬─────────┘
         │ referenced by
         ▼
┌──────────────────┐
│   forge.yaml     │  Records the repository's relationship to
│  (in repo root)  │  the foundation. In the repository.
└────────┬─────────┘
         │ consumed by
         ▼
┌──────────────────┐
│  Forge commands  │  validate, check, diff, update, explain
└──────────────────┘
```

- The **Blueprint** is the portable declaration of intent.
- `forge.yaml` is the repository-specific record of which Blueprint
  and version apply.
- Forge commands operate on `forge.yaml`.

### 3.5 Location

`forge.yaml` lives at the **repository root**:

```text
payments-api/
├── forge.yaml          ← this file
├── README.md
├── pyproject.toml
├── src/
├── tests/
└── .github/
```

There is exactly one `forge.yaml` per repository. Nested
`forge.yaml` files (e.g., in subdirectories) are not supported in
schema version 1.

---

## 4. File Schema

### 4.1 Top-Level Structure

`forge.yaml` is a YAML document with the following top-level sections:

```yaml
forge:         # Forge metadata (required)
  version:     # Schema version
  blueprint:   # Foundation reference
  ...

project:       # Project identity (required)
  name:
  type:

language:      # Language (required)
  name:
  version:

components:    # Enabled components (optional)
  ...

policies:      # Foundation policies (optional)
  ...

metadata:      # Forge-generated metadata (optional)
  ...
```

Every section other than `forge`, `project`, and `language` is
optional.

### 4.2 Minimal Valid `forge.yaml`

The smallest valid `forge.yaml`:

```yaml
forge:
  version: 1
  blueprint: minimal

project:
  name: my-project
  type: service

language:
  name: go
```

This declares:

- Schema version 1
- Follows the `minimal` Blueprint
- A project named `my-project`
- Of type `service`
- Using Go

### 4.3 Complete `forge.yaml`

A fuller example:

```yaml
forge:
  version: 1
  blueprint: python-api
  blueprint_version: 2.4.0
  template: python-fastapi
  template_version: 1.0.0

project:
  name: payments-api
  type: service

language:
  name: python
  version: "3.13"

framework:
  name: fastapi
  version: "0.115"

database:
  name: postgres
  version: "16"

components:
  testing:
    enabled: true
    version: "1.0.0"
  docker:
    enabled: true
    version: "1.0.0"
  postgres:
    enabled: true
    version: "1.0.0"

policies:
  testing:
    required: true
    severity: error
  ci:
    required: true
    severity: error
  security:
    required: true
    severity: error
  documentation:
    required: true
    severity: warning

metadata:
  forge_version: "0.1.0"
  created_at: "2026-10-09T08:00:00Z"
  last_validated: "2026-10-09T12:00:00Z"
```

---

## 5. Required Fields

### 5.1 `forge.version`

| Attribute | Value |
|-----------|-------|
| Type | integer |
| Required | Yes |
| Description | `forge.yaml` schema version |
| Constraint | Positive integer |
| Example | `1` |

Identifies which schema this `forge.yaml` conforms to. Distinct from
the Blueprint schema version (`forge.blueprint_version`).

### 5.2 `forge.blueprint`

| Attribute | Value |
|-----------|-------|
| Type | string |
| Required | Yes |
| Description | Blueprint identifier |
| Constraint | Matches Blueprint naming rules |
| Example | `python-api`, `go-service` |

References a Blueprint by identifier. The Blueprint itself may be
bundled with Forge, installed locally, or (in future phases) fetched
from a registry.

### 5.3 `project.name`

| Attribute | Value |
|-----------|-------|
| Type | string |
| Required | Yes |
| Constraint | Lowercase, alphanumeric, hyphens, underscores |
| Example | `payments-api` |

### 5.4 `project.type`

| Attribute | Value |
|-----------|-------|
| Type | enum |
| Required | Yes |
| Allowed values | `service`, `cli`, `library`, `web-application`, `worker`, `monorepo` |

### 5.5 `language.name`

| Attribute | Value |
|-----------|-------|
| Type | enum |
| Required | Yes |
| Allowed values | `python`, `go`, `typescript`, `rust` |

---

## 6. Optional Fields

### 6.1 `forge.blueprint_version`

| Attribute | Value |
|-----------|-------|
| Type | string (semver) |
| Required | No |
| Default | Latest available in Forge's bundled Blueprints |
| Example | `2.4.0` |

Records the specific version of the referenced Blueprint. Enables
reproducibility: a repository created with `python-api@2.4.0` remains
on that version until explicitly updated.

If omitted, Forge resolves the latest available version at the time
of validation. This is convenient for development but **not
recommended for production repositories**.

### 6.2 `forge.template` and `forge.template_version`

| Attribute | Value |
|-----------|-------|
| Type | string |
| Required | No |
| Example | `python-fastapi`, `1.0.0` |

Records which template materialised the Blueprint. Used for
provenance and for future template-compatibility checks.

These fields are populated automatically by `forge new`. Developers
rarely edit them.

### 6.3 `language.version`

Records the minimum language version. Defaults to the language's
minimum supported version.

### 6.4 `framework`, `database`

Optional. Match the corresponding Blueprint sections. If present in
`forge.yaml`, they must be compatible with the Blueprint's declared
values.

### 6.5 `components`

Records which components are enabled. Each component entry has:

```yaml
components:
  <component-id>:
    enabled: <boolean>
    version: <semver, optional>
    config: <object, optional>
```

The `components` section is optional. Components are introduced in
Phase 12.

### 6.6 `policies`

Records which foundation policies apply. Each policy entry has:

```yaml
policies:
  <policy-id>:
    required: <boolean>
    severity: <enum: info | warning | error>
```

The `policies` section is optional. Policies are introduced in
Phase 8.

### 6.7 `metadata`

Records Forge-generated metadata. This section is **not edited by
developers**.

```yaml
metadata:
  forge_version: "0.1.0"
  created_at: "2026-10-09T08:00:00Z"
  last_validated: "2026-10-09T12:00:00Z"
```

| Field | Purpose |
|-------|---------|
| `forge_version` | Forge version that created or last migrated this file |
| `created_at` | When the repository was initialised |
| `last_validated` | When `forge validate` last succeeded |

---

## 7. Ownership

### 7.1 Who Edits `forge.yaml`?

`forge.yaml` has a **split ownership model**:

| Section | Owner | Editing method |
|---------|-------|----------------|
| `forge.*` | Forge | Written by `forge new` / `forge init`; edited by migrations |
| `project.*` | Developer | Can be edited directly |
| `language.*` | Developer | Can be edited directly |
| `framework`, `database` | Developer | Can be edited directly |
| `components.*` | Forge + Developer | Forge writes on `forge add`/`forge remove`; developer may edit config |
| `policies.*` | Developer | Can be edited directly |
| `metadata.*` | Forge | Never edited by hand |

### 7.2 Editing Rules

**Developers may edit:**

- `project.name` and `project.type` (with caveats — see § 7.3)
- `language.*`
- `framework.*`
- `database.*`
- `components.*.config`
- `policies.*`

**Developers must not edit:**

- `forge.version` (schema version)
- `forge.blueprint` (change via `forge update`, not by hand)
- `forge.blueprint_version` (change via `forge update`)
- `forge.template` and `forge.template_version`
- `metadata.*`

**Forge manages:**

- The entire `forge.*` section except `project.name` and
  `project.type`
- `metadata.*`
- Component entries added via `forge add` / `forge remove`

### 7.3 Changing Project Name or Type

Changing `project.name` after initial creation is **not** supported
in schema version 1. The name is embedded in generated files (package
names, import paths, configuration). Renaming requires regenerating
the project.

Changing `project.type` is supported but may invalidate the
foundation. Run `forge validate` after the change.

### 7.4 Comment Preservation

Forge preserves developer-added comments when it edits `forge.yaml`.
Specifically:

- Comments above fields Forge does not touch are preserved
- Comments inside Forge-managed sections may be lost when those
  sections are rewritten

To preserve comments in Forge-managed sections, developers should
avoid adding them, or move documentation into `project.description`
(a future field) rather than comments.

### 7.5 Formatting Preservation

Forge preserves YAML formatting (indentation, key order) when it can.
When Forge must rewrite a section, it uses the following rules:

- 2-space indentation
- Keys in canonical order
- Strings quoted only when necessary
- Trailing newline at end of file

If formatting changes on every `forge` command, that is a bug.

---

## 8. Source-of-Truth Rules

This is the most critical section of the specification.

### 8.1 The Question

When `forge.yaml` and the repository state disagree, which one wins?

**Example:** `forge.yaml` declares `testing.enabled: true`, but the
`tests/` directory has been deleted.

### 8.2 The Rule

> **`forge.yaml` is the declaration. The repository is reality.
> Forge never assumes either is authoritative; it reports the
> difference.**

Specifically:

| Command | Behaviour |
|---------|-----------|
| `forge validate` | Reports the difference as a finding |
| `forge check` | Reports the difference as drift |
| `forge diff` | Shows the difference in detail |
| `forge update` | Does **not** automatically resolve; requires explicit action |
| `forge new` | Creates a repository matching `forge.yaml` |

Forge does **not** "fix" the repository to match `forge.yaml`
automatically. It also does **not** "fix" `forge.yaml` to match the
repository automatically. It reports the difference and lets the
developer decide.

### 8.3 Why This Rule

- **Forge must not destroy developer work.** Automatically fixing
  the repository would overwrite legitimate changes.
- **Forge must not silently rewrite configuration.** Automatically
  fixing `forge.yaml` would hide drift.
- **Forge must be explainable.** A developer who deletes `tests/`
  intentionally should not have Forge silently undo the deletion.

### 8.4 Developer Actions

When `forge.yaml` and the repository disagree, the developer may:

1. **Restore the repository to match `forge.yaml`.**
   Run `forge check` to see what is missing, then restore manually or
   via a future `forge repair` command.
2. **Update `forge.yaml` to match the repository.**
   Edit `forge.yaml` directly (for developer-owned sections) or use
   `forge update` (for Forge-managed sections).
3. **Exempt the finding.**
   Add an exemption to `forge.yaml` (see
   [`docs/validation-spec.md`](./validation-spec.md) for exemptions).
4. **Leave the difference unresolved.**
   Forge will continue to report it.

### 8.5 When Forge Writes to `forge.yaml`

Forge writes to `forge.yaml` only in these cases:

| Operation | What Forge writes |
|-----------|-------------------|
| `forge new` | Entire file |
| `forge init` | Entire file |
| `forge add <component>` | Adds entry to `components.*` |
| `forge remove <component>` | Removes entry from `components.*` |
| `forge update` | Updates `forge.blueprint_version`, `forge.template_version`, `metadata.*` |
| Migration | Updates `forge.version` and field names |

In every case, Forge must:

- Back up the existing `forge.yaml` before writing
- Preserve developer-owned sections unchanged
- Preserve developer-added comments where possible
- Fail safely if the file is malformed

### 8.6 Conflict Resolution

If Forge cannot safely write to `forge.yaml` (e.g., because the file
has been malformed), it fails with a clear error and does not modify
the file.

Example:

```text
✗ Cannot update forge.yaml

Reason: forge.yaml is malformed at line 12.

The file has not been modified.

Fix the error and run the command again, or delete forge.yaml
and run `forge init` to recreate it.
```

---

## 9. Migration Strategy

### 9.1 Schema Versions

`forge.yaml` has a schema version (`forge.version`). Schema version
increments on breaking changes to the file structure.

| Version | Status | Forge support |
|---------|--------|---------------|
| 1 | Current | Forge 0.1.x and later |
| 2 | Planned | Future |

### 9.2 Migration Triggers

Migration happens when:

- Forge runs against a `forge.yaml` with a lower `forge.version`
- The migration is required to proceed with the requested command
- The user explicitly runs `forge migrate`

Forge does **not** silently upgrade schema versions. The user must
consent.

### 9.3 Migration Procedure

When migration is required, Forge:

1. Detects the schema version mismatch
2. Reports the mismatch and the required migration
3. Asks for confirmation (interactive) or requires `--yes`
   (non-interactive)
4. Backs up `forge.yaml` to `forge.yaml.backup`
5. Rewrites the file to the new schema version
6. Updates `forge.version` to the new value
7. Updates `metadata.forge_version` to the current Forge version
8. Verifies the migrated file parses correctly
9. Reports success

If any step fails, Forge restores the backup.

### 9.4 Migration Example

**Before migration (schema v1):**

```yaml
forge:
  version: 1
  blueprint: python-api

project:
  name: payments-api
  type: service

language:
  name: python
```

**After migration (schema v2, illustrative):**

```yaml
forge:
  version: 2
  blueprint: python-api
  blueprint_version: 2.4.0
  template: python-fastapi
  template_version: 1.0.0

project:
  name: payments-api
  type: service

language:
  name: python
  version: "3.12"

metadata:
  forge_version: "0.2.0"
  migrated_from: 1
  migrated_at: "2026-10-09T12:00:00Z"
```

The exact shape of schema v2 is not yet defined. This example is
illustrative.

### 9.5 Backward Compatibility

Forge supports schema version 1 for the lifetime of the 0.x series.
When schema version 2 is introduced:

- Forge 0.2.x supports both version 1 and version 2
- Forge 1.0.x supports both version 1 and version 2
- Deprecation of version 1 is announced at least one minor version
  in advance

### 9.6 Downgrade

Forge does **not** support downgrading `forge.yaml` to a lower schema
version. If a repository is used with an older Forge version after a
migration, Forge detects the newer schema version and refuses to
operate:

```text
✗ Unsupported forge.yaml schema version

Schema version: 2
This Forge version supports: 1

Upgrade Forge to a version that supports schema version 2.
```

This is deliberate: downgrading is unsafe because newer fields may
have been dropped or transformed during migration.

---

## 10. Corruption Handling

### 10.1 What Counts as Corruption

`forge.yaml` is considered corrupt if:

- It is not valid YAML
- It is not a mapping at the root
- It is missing a required section (`forge`, `project`, `language`)
- A required field is missing
- A field has the wrong type
- It contains unknown fields
- It contains a `forge.version` that is not a supported schema
  version

### 10.2 Detection

Every Forge command that reads `forge.yaml` validates it first:

1. Parse YAML
2. Validate against the schema version
3. Validate required fields
4. Validate field types
5. Validate no unknown fields
6. Validate semantic rules

If any step fails, the file is considered corrupt for that command's
purposes.

### 10.3 Behaviour on Corruption

Forge behaviour on corrupt `forge.yaml`:

| Command | Behaviour |
|---------|-----------|
| `forge validate` | Reports all validation errors |
| `forge explain` | Reports the error; does not produce an explanation |
| `forge check` | Reports the error; does not proceed with checking |
| `forge diff` | Reports the error; does not produce a diff |
| `forge update` | Refuses to proceed; suggests recovery |
| `forge add` / `forge remove` | Refuses to proceed; suggests recovery |
| `forge new` | Unaffected (does not read existing files) |
| `forge init` | Detects existing file; does not overwrite |

Forge **does not** modify a corrupt `forge.yaml` unless the user
explicitly requests it. This prevents a corrupt file from being
"silently fixed" in ways that lose data.

### 10.4 Recovery

When `forge.yaml` is corrupt, the developer has three options:

1. **Fix the file manually.**
   The validation errors identify what is wrong. Edit the file and
   re-run.

2. **Restore from backup.**
   If the corruption happened recently, restore from
   `forge.yaml.backup`.

3. **Regenerate from scratch.**
   Delete `forge.yaml` and run `forge init` to recreate it based on
   repository detection. This may lose metadata about the original
   Blueprint version.

Example error message:

```text
✗ forge.yaml is invalid

Line 8:
  project.type
  expected enum, got string

Supported values:
  service, cli, library, web-application, worker, monorepo

Fix the error and run the command again.
```

### 10.5 Automatic Repair

Forge does not automatically repair `forge.yaml`. Automatic repair
would risk silently changing the foundation. If a future phase
introduces `forge repair`, it must be explicit and require
confirmation.

---

## 11. Compatibility

### 11.1 Compatibility Matrix

Forge versions support specific `forge.yaml` schema versions and
Blueprint schema versions.

| Forge version | forge.yaml schema versions | Blueprint schema versions |
|---------------|----------------------------|---------------------------|
| 0.1.x | 1 | 1 |
| 0.2.x | 1, 2 (when introduced) | 1, 2 (when introduced) |
| 1.0.x | 1, 2 | 1, 2 |
| 2.0.x | 2, 3 (when introduced) | 2, 3 (when introduced) |

### 11.2 Old Forge on New `forge.yaml`

An older Forge version encountering a newer `forge.yaml`:

1. Parses the `forge.version` field
2. Detects the schema version is higher than supported
3. Fails with a clear error (see § 9.6)
4. Does **not** attempt to read or write the file

### 11.3 New Forge on Old `forge.yaml`

A newer Forge version encountering an older `forge.yaml`:

1. Parses the `forge.version` field
2. Detects the schema version is lower than current
3. Operates on the older schema without modification
4. Offers migration (see § 9.3) but does not require it

Forge **always** supports all older schema versions in the same
major series. Only breaking changes cross major versions.

### 11.4 Blueprint Version Compatibility

`forge.blueprint_version` is independent of `forge.version`.

- `forge.version` is the schema of the `forge.yaml` file
- `forge.blueprint_version` is the version of the referenced Blueprint

A `forge.yaml` with schema version 1 can reference a Blueprint of any
version supported by the running Forge.

If the referenced Blueprint version is not available, Forge fails:

```text
✗ Blueprint version not available

Blueprint: python-api
Requested version: 2.4.0
Available versions: 2.3.0, 2.5.0

Update forge.yaml to a supported version, or install the missing
Blueprint.
```

### 11.5 Template Version Compatibility

`forge.template_version` records the template used to materialise the
project. This is informational and used for provenance.

Forge does **not** require the original template to be present for
`validate`, `check`, `diff`, or `explain`. Template resolution is only
needed for `forge update` when the template has changed.

### 11.6 Forward Compatibility

If a future Forge version adds new optional fields to `forge.yaml`,
older Forge versions will fail with an "unknown field" error
(per § 10.1). To prevent this:

- New optional fields are only added in schema version bumps
- Schema version bumps are announced and documented

Developers who need to use both old and new Forge versions on the
same repository should keep `forge.version` at the older schema until
they upgrade all environments.

---

## 12. Examples

### 12.1 Minimal `forge.yaml`

```yaml
forge:
  version: 1
  blueprint: minimal

project:
  name: my-project
  type: service

language:
  name: go
```

### 12.2 Python API `forge.yaml`

```yaml
forge:
  version: 1
  blueprint: python-api
  blueprint_version: 2.4.0
  template: python-fastapi
  template_version: 1.0.0

project:
  name: payments-api
  type: service

language:
  name: python
  version: "3.13"

framework:
  name: fastapi
  version: "0.115"

database:
  name: postgres
  version: "16"

metadata:
  forge_version: "0.1.0"
  created_at: "2026-10-09T08:00:00Z"
  last_validated: "2026-10-09T12:00:00Z"
```

### 12.3 With Components and Policies

```yaml
forge:
  version: 1
  blueprint: python-api
  blueprint_version: 2.4.0

project:
  name: payments-api
  type: service

language:
  name: python
  version: "3.13"

components:
  testing:
    enabled: true
    version: "1.0.0"
  docker:
    enabled: true
    version: "1.0.0"
  postgres:
    enabled: true
    version: "1.0.0"

policies:
  testing:
    required: true
    severity: error
  ci:
    required: true
    severity: error
  security:
    required: true
    severity: error
  documentation:
    required: true
    severity: warning

metadata:
  forge_version: "0.1.0"
  created_at: "2026-10-09T08:00:00Z"
```

### 12.4 Adopted Repository (via `forge init`)

A repository adopted via `forge init` may not have a template:

```yaml
forge:
  version: 1
  blueprint: python-api
  blueprint_version: 2.4.0

project:
  name: legacy-service
  type: service

language:
  name: python

metadata:
  forge_version: "0.1.0"
  created_at: "2026-10-09T08:00:00Z"
  adoption_method: detection
```

The `adoption_method` field records how the repository entered Forge
management. Allowed values: `created` (via `forge new`), `detection`
(via `forge init`), `manual` (hand-authored).

---

## 13. Relationship to Other Specifications

| Specification | Relationship |
|---------------|--------------|
| [Blueprint](./blueprint-spec.md) | Referenced by `forge.yaml`; defines the foundation contract |
| [Template](./template-spec.md) | Recorded in `forge.yaml` as provenance; not required for most commands |
| [Component](./component-spec.md) | Recorded in `forge.yaml` under `components` |
| [Validation](./validation-spec.md) | Defines policies recorded in `forge.yaml` |
| [Update Model](./update-model.md) | Uses `forge.yaml` to determine target version |
| [Security Model](./security-model.md) | Governs how `forge.yaml` is written safely |
| [CLI UX Spec](./cli-ux-spec.md) | Defines the commands that read and write `forge.yaml` |
| [Architecture](./architecture.md) | Defines the module that owns `forge.yaml` parsing and writing |

---

## 14. Open Questions

The following questions remain open and should be resolved before
implementation:

- Should `forge.yaml` be named `.forge.yaml` (hidden) or
  `forge.yaml` (visible) at the repository root?
- Should `forge.yaml` support a top-level `description` field for
  human context?
- Should `forge.yaml` be signed or checksummed to prevent tampering?
- Should `forge.yaml` support environment-specific overrides (e.g.,
  `forge.dev.yaml`), or is single-file configuration sufficient?
- Should `metadata.last_validated` be updated automatically on
  successful `forge validate`, or only on explicit user action?
- What should happen if a developer manually adds a component to
  `forge.yaml` without using `forge add`?
- Should `forge.yaml` include a hash of each generated file for
  faster drift detection, or is on-demand hashing sufficient?
- Should `forge.yaml` be committed to version control by default, or
  should Forge generate a `.gitignore` entry?

These questions will be addressed in Phase 2 as implementation begins.

---

## 15. Status

**Draft.**

This specification is under active development during Phase 1. It
becomes **Approved** when:

- The schema is frozen for version 1
- All validation rules are tested
- All examples validate against the schema
- Source-of-truth rules are reviewed and confirmed
- Migration and corruption handling are tested
- Open questions have been resolved or explicitly deferred
- The specification has been reviewed for consistency with
  [`docs/blueprint-spec.md`](./blueprint-spec.md) § 13
