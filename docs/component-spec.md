# Component Specification

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

This specification defines what a Forge **component** is, how it is
structured, how components compose, and how conflicts between
components are resolved.

A component is a **reusable engineering capability** that can be added
to or removed from a project's foundation. Where a template is a
complete materialisation of a Blueprint, a component is a smaller,
independently versioned capability that can be composed into many
foundations.

This specification exists to answer:

- What is a component?
- What does a component manifest contain?
- How are component dependencies declared and resolved?
- How are conflicts between components detected?
- Which component owns which generated files?
- What is the component lifecycle?
- How do two components safely modify the same file?

---

## 2. Scope

**In scope:**

- Component concept and identity
- Component manifest schema
- Component dependencies
- Component conflicts
- File ownership model
- Component lifecycle (install, validate, update, remove)
- Composition rules
- Conflict resolution
- Example components

**Out of scope:**

- Blueprint schema (see
  [`docs/blueprint-spec.md`](./blueprint-spec.md))
- Template format (see
  [`docs/template-spec.md`](./template-spec.md))
- Registry and distribution (see
  [`docs/registry-spec.md`](./registry-spec.md), future)
- Update algorithm (see
  [`docs/update-model.md`](./update-model.md))
- Filesystem security (see
  [`docs/security-model.md`](./security-model.md))
- CLI commands for component management (see
  [`docs/cli-ux-spec.md`](./cli-ux-spec.md) § 5.4)

---

## 3. Conceptual Model

A component is a smaller, composable alternative to a monolithic
template. Where a template produces a whole project, a component
produces one capability.

```text
┌──────────────────┐
│    BLUEPRINT     │  "Python API with FastAPI, PostgreSQL,
│                  │   Docker, and pytest."
└────────┬─────────┘
         │ composed of
         ▼
┌──────────────────────────────────────────────────────┐
│                    COMPONENTS                        │
│                                                      │
│   ┌────────┐ ┌──────────┐ ┌────────┐ ┌──────────┐    │
│   │ python │ │ fastapi  │ │postgres│ │  docker  │    │
│   └────────┘ └──────────┘ └────────┘ └──────────┘    │
│                                                      │
│   ┌────────┐ ┌──────────────┐                        │
│   │ pytest │ │ github-actions│                       │
│   └────────┘ └──────────────┘                        │
│                                                      │
└───────────────────────┬──────────────────────────────┘
                        │ materialised by
                        ▼
┌──────────────────────────────────────────────────────┐
│                    REPOSITORY                        │
└──────────────────────────────────────────────────────┘
```

### 3.1 What a Component Is

- A versioned, composable engineering capability
- A declarative specification of files, configuration, and
  dependencies
- Independent of any specific Blueprint
- Reusable across many foundations
- Sandboxed (no arbitrary code execution)

### 3.2 What a Component Is Not

- A template (components are smaller and compose; templates
  materialise a whole Blueprint)
- A package manager (components do not install system dependencies)
- A plugin (components do not extend Forge itself)
- A build system (components do not build the project)
- A runtime dependency (components do not run at application
  runtime)

### 3.3 Examples

Common components:

| Component | Capability |
|-----------|------------|
| `python` | Python runtime setup |
| `go` | Go runtime setup |
| `fastapi` | FastAPI framework setup |
| `postgres` | PostgreSQL database configuration |
| `sqlite` | SQLite database configuration |
| `docker` | Docker containerisation |
| `github-actions` | GitHub Actions CI |
| `pytest` | Pytest testing framework |
| `jest` | Jest testing framework |
| `observability` | OpenTelemetry instrumentation |
| `security-baseline` | Standard security controls |

Each component is a small, focused, reusable capability. Components
compose into a foundation.

---

## 4. Component Structure

### 4.1 Directory Layout

A component is a directory with a canonical structure:

```text
<component>/
├── component.yaml      # manifest (required)
├── files/              # file templates (required)
├── config/             # default configuration (optional)
├── tests/              # component tests (optional, but recommended)
└── README.md           # component documentation (recommended)
```

### 4.2 `component.yaml`

The component manifest. Describes identity, dependencies, conflicts,
and file contributions. Required.

### 4.3 `files/`

Contains the file templates that this component contributes. Structure
mirrors templates but scoped to the component's capability.

Example for `postgres`:

```text
files/
├── compose.postgres.yaml.tmpl
├── src/
│   └── {{package_name}}/
│       └── database.py.tmpl
└── migrations/
    └── .gitkeep
```

### 4.4 `config/`

Optional. Contains default configuration values for the component.
Merged into the Blueprint's component configuration at install time.

### 4.5 `tests/`

Optional. Contains fixture-based tests verifying that the component
renders and applies correctly.

### 4.6 `README.md`

Optional but recommended. Human-readable documentation describing
what the component does and how it is configured.

---

## 5. Component Manifest

### 5.1 Structure

`component.yaml` is a YAML document describing the component:

```yaml
name: postgres
version: 1.0.0
description: PostgreSQL database support
author: Forge maintainers
license: Apache-2.0

requires:
  - name: python
    version: ">=1.0.0"

conflicts:
  - name: sqlite
    reason: "A project cannot have both PostgreSQL and SQLite as its primary database."

compatibility:
  language:
    - python
    - go
    - typescript
  project_types:
    - service
    - web-application

files:
  - path: compose.postgres.yaml.tmpl
    target: compose.postgres.yaml
  - path: src/{{package_name}}/database.py.tmpl
    target: src/{{package_name}}/database.py
    condition: language_name == "python"
  - path: migrations/.gitkeep
    target: migrations/.gitkeep

owns:
  - compose.postgres.yaml
  - src/{{package_name}}/database.py
  - migrations/

provides:
  database: postgres

metadata:
  keywords:
    - database
    - sql
    - postgresql
```

### 5.2 Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Component identifier |
| `version` | semver | Component version |
| `description` | string | One-line description |
| `files` | list | Files contributed by this component |

### 5.3 Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `requires` | list | Component dependencies |
| `conflicts` | list | Components this one cannot coexist with |
| `compatibility` | object | Languages and project types this supports |
| `owns` | list | Files/directories exclusively owned |
| `provides` | object | Capabilities this component provides |
| `config` | object | Default configuration values |
| `metadata` | object | Keywords, homepage, etc. |

---

## 6. Component Identity

### 6.1 Identity Model

Every component has a stable identity:

```text
<name>@<version>
```

Examples:

- `postgres@1.0.0`
- `docker@1.2.0`
- `github-actions@1.0.0`

The name is a lowercase alphanumeric string with hyphens. The version
is a valid semver string.

### 6.2 Version Semantics

Component versions follow semantic versioning:

| Change | Bump |
|--------|------|
| Bug fix in rendering | Patch (`1.0.0 → 1.0.1`) |
| New optional file, new optional config | Minor (`1.0.0 → 1.1.0`) |
| Breaking change (renamed file, changed API) | Major (`1.0.0 → 2.0.0`) |

A breaking change invalidates the base state used by updates. Major
version bumps require explicit user action (they do not happen
automatically).

### 6.3 Reserved Names

The following names are reserved and may not be used as component
names:

- `template`
- `blueprint`
- `forge`
- `metadata`
- `component`

Attempting to use a reserved name causes component validation to
fail.

---

## 7. Component Dependencies

### 7.1 Dependency Declaration

Components may require other components:

```yaml
requires:
  - name: python
    version: ">=1.0.0"
  - name: docker
    version: "^2.0.0"
```

Each dependency declares:

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Dependency component name |
| `version` | No | Version constraint (semver range) |

If `version` is omitted, any version of the dependency is accepted.

### 7.2 Version Constraints

Version constraints use standard semver syntax:

| Constraint | Meaning |
|------------|---------|
| `1.0.0` | Exactly 1.0.0 |
| `>=1.0.0` | At least 1.0.0 |
| `^1.0.0` | Compatible with 1.0.0 (`>=1.0.0 <2.0.0`) |
| `~1.0.0` | Patch-compatible with 1.0.0 (`>=1.0.0 <1.1.0`) |
| `>=1.0.0 <2.0.0` | Range |

### 7.3 Dependency Resolution

When a component is added to a foundation, Forge resolves the full
dependency graph:

1. Load the requested component's manifest
2. Load each dependency's manifest
3. Recursively resolve transitive dependencies
4. Detect cycles (fail if found)
5. Verify all version constraints are satisfiable
6. Verify all components are compatible with the Blueprint

Example resolution for `postgres` (which requires `python` and
`docker`):

```text
postgres@1.0.0
 ├── python@1.0.0
 └── docker@2.0.0
     └── (no further dependencies)
```

### 7.4 Cycle Detection

If a dependency graph contains a cycle:

```text
A → B → C → A
```

Forge fails with:

```text
✗ Component dependency cycle detected

Cycle:
  A → B → C → A

Break the cycle by removing one of the dependencies.
```

### 7.5 Missing Dependencies

If a required dependency is not available:

```text
✗ Missing component dependency

Component: postgres@1.0.0
Requires: python >=1.0.0

Available: (none)

Install 'python' first, or add it to the foundation.
```

### 7.6 Dependency Conflicts

If two components require incompatible versions of a shared
dependency:

```text
✗ Component version conflict

Shared dependency: docker
  postgres@1.0.0 requires: docker >=2.0.0
  observability@1.0.0 requires: docker <2.0.0

No version satisfies both constraints.
```

Forge does **not** attempt to resolve this automatically. The
developer must choose or update the conflicting components.

---

## 8. Component Conflicts

### 8.1 Conflict Declaration

Components may declare that they conflict with other components:

```yaml
conflicts:
  - name: sqlite
    reason: "A project cannot have both PostgreSQL and SQLite as its primary database."
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Conflicting component name |
| `reason` | No | Human-readable explanation |

### 8.2 Conflict Semantics

A conflict is **symmetric**: if `postgres` conflicts with `sqlite`,
then `sqlite` also conflicts with `postgres`, even if only one declares
the conflict.

Conflicts are based on **name**, not version. If any version of
`sqlite` is present, `postgres` cannot be added (and vice versa).

### 8.3 Conflict Detection

Before adding a component, Forge checks:

1. Is any conflicting component already in the foundation?
2. Does any other component declare a conflict with the one being
   added?

If either is true, the operation fails:

```text
✗ Component conflict

Cannot add: postgres@1.0.0
Already present: sqlite@1.0.0

Reason:
  A project cannot have both PostgreSQL and SQLite as its primary
  database.

Remove 'sqlite' first, or choose a different database component.
```

### 8.4 File-Level Conflicts

Two components may attempt to write to the same file. This is
distinct from a declared conflict and is handled by the ownership
model (§ 10).

### 8.5 Conflict Resolution

Forge does **not** resolve conflicts automatically. The developer
must:

1. Remove the conflicting component
2. Choose a different component
3. Add an explicit resolution to the Blueprint (if supported in a
   future version)

Automatic resolution is dangerous because it can silently discard
one component's contribution.

---

## 9. File Ownership

### 9.1 Ownership Model

Every file that a component generates has a **declared owner**. When
multiple components could write to the same file, ownership
determines how conflicts are resolved.

### 9.2 Ownership Declaration

A component declares which files it owns:

```yaml
owns:
  - compose.postgres.yaml
  - src/{{package_name}}/database.py
  - migrations/
```

The `owns` list contains:

- **Files**: exact target paths
- **Directories**: target paths ending with `/`

Patterns are not supported in schema version 1. Ownership is exact.

### 9.3 Ownership Rules

- A component owns exactly the files and directories it declares
- Two components may **not** own the same file
- A component may own a file that is inside another component's
  directory (as long as the file itself is not owned by the parent)
- Ownership is checked at install time

Example of a valid ownership split:

```text
Component A owns:
  - Dockerfile
  - compose.yaml

Component B owns:
  - src/main.py
  - tests/
```

Example of an invalid ownership split:

```text
Component A owns:
  - Dockerfile

Component B owns:
  - Dockerfile    ← conflict
```

### 9.4 Shared Files

Some files legitimately need contributions from multiple components.
For example, `pyproject.toml` may be modified by both the `python`
component (base structure) and the `pytest` component (test
configuration).

Forge handles this via **structured modification**:

- One component **owns** the file
- Other components declare **contributions** to it
- The owner component defines **extension points** where
  contributions can be merged

This is a **major architectural decision**. Full details are in
§ 10.

### 9.5 Ownership Recording

Ownership is recorded in the repository's Forge state:

```text
.forge/
└── state.yaml
```

Example state entry:

```yaml
files:
  - path: compose.postgres.yaml
    owner: component:postgres@1.0.0
    hash: sha256:abc123...
  - path: src/payments_api/database.py
    owner: component:postgres@1.0.0
    hash: sha256:def456...
```

The state file is the source of truth for ownership. It is never
edited by hand.

### 9.6 Ownership and Updates

When a component is updated:

- Files it owns are updated according to the update model
- Files it contributes to (but does not own) are re-merged
- Files it no longer owns are marked as orphaned

Ownership is preserved across updates.

---

## 10. Composition Rules

This section defines the rules for composing components and
resolving file-level conflicts.

### 10.1 The Composition Problem

Suppose two components both need to modify `pyproject.toml`:

```text
python component wants:
  [project]
  name = "{{project_name}}"
  requires-python = ">=3.12"

pytest component wants:
  [tool.pytest.ini_options]
  testpaths = ["tests"]
```

Both modifications are legitimate. Neither should overwrite the
other. The composition rules must define how this works.

### 10.2 Ownership + Contribution Model

Forge uses a **two-tier composition model**:

1. **Ownership**: one component owns each file
2. **Contributions**: other components may contribute fragments to
   owned files via declared extension points

```text
┌──────────────────────────────────────────────────┐
│  File: pyproject.toml                            │
│                                                  │
│  Owner: python@1.0.0                             │
│                                                  │
│  Extension points:                               │
│    - tool.pytest                                 │
│    - tool.coverage                               │
│    - tool.ruff                                   │
│                                                  │
└──────────────────────────────────────────────────┘
         ▲                    ▲
         │ contributes to     │ contributes to
         │ tool.pytest        │ tool.ruff
         │                    │
  ┌──────┴──────┐      ┌──────┴──────┐
  │  pytest     │      │   ruff      │
  │  @1.0.0     │      │   @1.0.0    │
  └─────────────┘      └─────────────┘
```

### 10.3 Extension Point Declaration

The owning component declares extension points in its manifest:

```yaml
# Component: python
files:
  - path: pyproject.toml.tmpl
    target: pyproject.toml

owns:
  - pyproject.toml

extension_points:
  pyproject.toml:
    - path: "tool.pytest"
      type: object
      merge: deep
    - path: "tool.coverage"
      type: object
      merge: deep
    - path: "tool.ruff"
      type: object
      merge: deep
    - path: "project.dependencies"
      type: array
      merge: append
```

Each extension point declares:

| Field | Meaning |
|-------|---------|
| `path` | Location in the file (dot notation for structured formats) |
| `type` | Data type at that location (`object`, `array`, `string`) |
| `merge` | Merge strategy (`deep`, `append`, `replace`, `unique`) |

### 10.4 Contribution Declaration

Contributing components declare their contributions:

```yaml
# Component: pytest
contributes:
  pyproject.toml:
    tool.pytest:
      testpaths: ["tests"]
      addopts: "-v"
```

Contributions are:

- Declarative (no code)
- Applied at install time
- Merged according to the extension point's strategy

### 10.5 Merge Strategies

| Strategy | Semantics |
|----------|-----------|
| `deep` | Recursive merge of nested objects |
| `append` | Concatenate arrays |
| `unique` | Concatenate arrays, remove duplicates |
| `replace` | Replace the value entirely |
| `error` | Refuse the contribution (used for single-owner values) |

### 10.6 Unstructured Files

Some files (e.g., `Dockerfile`, shell scripts) are not easily
structured. For these, the composition model degrades:

- The owning component owns the entire file
- Contributing components must not modify it
- If a contribution is required, the owning component provides a
  designated block (e.g., a `# BEGIN pytest` comment section)

Designated blocks are declared in the extension points:

```yaml
extension_points:
  Dockerfile:
    - path: "# BEGIN pytest"
      type: block
      merge: replace_block
```

The block between `# BEGIN pytest` and `# END pytest` is replaced by
the contributing component's content.

### 10.7 Conflict Handling

If two components attempt to contribute to the same extension point
with incompatible values:

```text
✗ Contribution conflict

File: pyproject.toml
Extension point: project.dependencies

Contributor A: python@1.0.0
  adds: ["fastapi>=0.115"]

Contributor B: fastapi@1.0.0
  adds: ["fastapi>=0.100"]

Values are incompatible.
```

Forge fails and requires the developer to resolve the conflict.

### 10.8 Deterministic Composition

Given the same set of components and versions, composition produces
the same output every time. The order in which components are added
does not affect the result.

This is verified by comparing output across multiple addition orders
in the test suite.

### 10.9 Composition Limits

Composition works for:

- Structured formats: YAML, TOML, JSON, INI
- Block-delimited sections in unstructured files
- File additions (component A adds `file1`, component B adds `file2`)

Composition does **not** work for:

- Two components modifying the same line in an unstructured file
- Two components claiming ownership of the same file
- Two components adding conflicting top-level keys to structured files

These cases are conflicts and must be resolved by the developer.

---

## 11. Component Lifecycle

### 11.1 Lifecycle Stages

Every component goes through the following lifecycle within a
project:

```text
Install → Validate → Update → Remove
```

### 11.2 Install

Adding a component to a project:

1. Resolve the component's manifest
2. Resolve dependencies (recursively)
3. Detect conflicts with existing components
4. Verify version constraints
5. Verify compatibility with the Blueprint
6. Compute the file plan (files to add, files to modify)
7. Verify ownership (no conflicts)
8. Merge contributions into shared files
9. Back up existing state
10. Apply the file plan
11. Update the state file
12. Run validation

Example:

```text
$ forge add postgres

Forge Add Component
───────────────────

Component: postgres@1.0.0

Dependencies resolved:
  ✓ python@1.0.0
  ✓ docker@2.0.0

Conflicts checked:
  ✓ No conflicts detected

Compatibility:
  ✓ Language: python
  ✓ Project type: service

File plan:
  + compose.postgres.yaml
  + src/payments_api/database.py
  + migrations/.gitkeep
  ~ pyproject.toml (contribution merged)

State updated.
Validation: PASS
```

### 11.3 Validate

After install, Forge validates that the component is correctly
applied:

1. All owned files exist
2. All owned files match the expected content (after merge)
3. All contributions are present in the shared files
4. The state file records the component
5. No conflicts were left unresolved

`forge check` also verifies components on every run.

### 11.4 Update

Updating a component to a new version:

1. Resolve the target version
2. Verify the update is compatible with the current state
3. Compute the update plan (files to add, modify, delete)
4. Detect conflicts with developer modifications
5. Back up state
6. Apply the update
7. Verify the result

Updates are covered in detail in
[`docs/update-model.md`](./update-model.md).

### 11.5 Remove

Removing a component from a project:

1. Verify no other component depends on it
2. Compute the removal plan (files to delete, files to modify)
3. Detect developer modifications to owned files
4. Back up state
5. Apply the removal (with confirmation if developer-modified files
   exist)
6. Update the state file
7. Verify the result

Example:

```text
$ forge remove postgres

Forge Remove Component
──────────────────────

Component: postgres@1.0.0

Dependencies:
  ✓ No other components depend on postgres

Files to remove:
  - compose.postgres.yaml
  - src/payments_api/database.py
  - migrations/.gitkeep

Files to modify:
  ~ pyproject.toml (remove contribution)

⚠ Warning: 1 file has been modified by the developer
  src/payments_api/database.py

Removing this component will delete this file.
Backup is created before removal.

Continue? [y/N]
```

### 11.6 Lifecycle Events

Each lifecycle stage emits events that are recorded:

| Event | Recorded in |
|-------|-------------|
| Component installed | `.forge/state.yaml` |
| Component updated | `.forge/state.yaml` |
| Component removed | `.forge/state.yaml` |
| Component validation failed | `.forge/state.yaml` |

State records preserve the history of component changes.

---

## 12. Component Validation Rules

Every component is validated before use.

### 12.1 Manifest Rules

| Rule | Description |
|------|-------------|
| CMP-001 | `component.yaml` exists and is valid YAML |
| CMP-002 | `name` is present, non-empty, and not reserved |
| CMP-003 | `version` is present and valid semver |
| CMP-004 | `description` is present and non-empty |
| CMP-005 | `files` is present and non-empty |
| CMP-006 | No unknown fields are present |

### 12.2 Dependency Rules

| Rule | Description |
|------|-------------|
| CMP-101 | All `requires[].name` are valid component names |
| CMP-102 | All `requires[].version` are valid semver ranges |
| CMP-103 | No dependency cycles |
| CMP-104 | No self-dependencies |

### 12.3 Conflict Rules

| Rule | Description |
|------|-------------|
| CMP-201 | All `conflicts[].name` are valid component names |
| CMP-202 | No self-conflicts |
| CMP-203 | No symmetric conflicts declared only one way (warning) |

### 12.4 File Rules

| Rule | Description |
|------|-------------|
| CMP-301 | Every `files[].path` exists inside `files/` |
| CMP-302 | No `files[].path` escapes `files/` via `..` |
| CMP-303 | No `files[].target` escapes the repository root |
| CMP-304 | No duplicate `files[].target` values |
| CMP-305 | No reserved names appear as target paths |

### 12.5 Ownership Rules

| Rule | Description |
|------|-------------|
| CMP-401 | Every `owns[]` entry is declared in `files[]` or is a directory |
| CMP-402 | No `owns[]` entry overlaps with another component's ownership |
| CMP-403 | No ownership conflicts within the same component |
| CMP-404 | Directories owned must not contain files owned by other components unless explicitly allowed |

### 12.6 Extension Point Rules

| Rule | Description |
|------|-------------|
| CMP-501 | Extension point paths are valid for the target file format |
| CMP-502 | Extension point types are valid |
| CMP-503 | Extension point merge strategies are valid |
| CMP-504 | Contribution declarations reference valid extension points |

### 12.7 Compatibility Rules

| Rule | Description |
|------|-------------|
| CMP-601 | Component declares at least one language (unless language-agnostic) |
| CMP-602 | Component declares at least one project type |
| CMP-603 | If `provides` is declared, values are non-empty |

A component that fails any of these rules cannot be used.

---

## 13. Example Components

### 13.1 `postgres`

```yaml
name: postgres
version: 1.0.0
description: PostgreSQL database support
author: Forge maintainers
license: Apache-2.0

requires:
  - name: python
    version: ">=1.0.0"
  - name: docker
    version: ">=2.0.0"

conflicts:
  - name: sqlite
    reason: "A project cannot have both PostgreSQL and SQLite."

compatibility:
  language:
    - python
    - go
    - typescript
  project_types:
    - service
    - web-application

files:
  - path: compose.postgres.yaml.tmpl
    target: compose.postgres.yaml
  - path: src/{{package_name}}/database.py.tmpl
    target: src/{{package_name}}/database.py
    condition: language_name == "python"
  - path: migrations/.gitkeep
    target: migrations/.gitkeep

owns:
  - compose.postgres.yaml
  - src/{{package_name}}/database.py
  - migrations/

provides:
  database: postgres
```

### 13.2 `docker`

```yaml
name: docker
version: 2.0.0
description: Docker containerisation
author: Forge maintainers
license: Apache-2.0

compatibility:
  language:
    - python
    - go
    - typescript
    - rust
  project_types:
    - service
    - cli
    - worker

files:
  - path: Dockerfile.tmpl
    target: Dockerfile
  - path: .dockerignore
    target: .dockerignore

owns:
  - Dockerfile
  - .dockerignore

extension_points:
  Dockerfile:
    - path: "# BEGIN RUN"
      type: block
      merge: replace_block
    - path: "# BEGIN COPY"
      type: block
      merge: replace_block

provides:
  container: docker
```

### 13.3 `github-actions`

```yaml
name: github-actions
version: 1.0.0
description: GitHub Actions continuous integration
author: Forge maintainers
license: Apache-2.0

compatibility:
  language:
    - python
    - go
    - typescript
    - rust
  project_types:
    - service
    - cli
    - library
    - web-application
    - worker

files:
  - path: workflows/ci.yml.tmpl
    target: .github/workflows/ci.yml
  - path: workflows/release.yml.tmpl
    target: .github/workflows/release.yml
    condition: project_type == "library"

owns:
  - .github/workflows/ci.yml
  - .github/workflows/release.yml

provides:
  ci: github-actions
```

### 13.4 `pytest`

```yaml
name: pytest
version: 1.0.0
description: Pytest testing framework
author: Forge maintainers
license: Apache-2.0

requires:
  - name: python
    version: ">=1.0.0"

compatibility:
  language:
    - python
  project_types:
    - service
    - cli
    - library
    - web-application
    - worker

files:
  - path: tests/__init__.py
    target: tests/__init__.py
  - path: tests/conftest.py.tmpl
    target: tests/conftest.py

owns:
  - tests/__init__.py
  - tests/conftest.py

contributes:
  pyproject.toml:
    tool.pytest:
      testpaths: ["tests"]
      addopts: "-v"

provides:
  testing: pytest
```

### 13.5 `security-baseline`

```yaml
name: security-baseline
version: 1.0.0
description: Standard security controls
author: Forge maintainers
license: Apache-2.0

compatibility:
  language:
    - python
    - go
    - typescript
    - rust
  project_types:
    - service
    - cli
    - library
    - web-application
    - worker

files:
  - path: .gitignore.security
    target: .gitignore.security
  - path: SECURITY.md.tmpl
    target: SECURITY.md

owns:
  - .gitignore.security
  - SECURITY.md

contributes:
  .gitignore:
    content:
      - ".env"
      - ".env.local"
      - "*.pem"
      - "*.key"

provides:
  security: baseline
```

---

## 14. Relationship to Other Specifications

| Specification | Relationship |
|---------------|--------------|
| [Blueprint](./blueprint-spec.md) | Components compose into a Blueprint |
| [forge.yaml](./forge-yaml-spec.md) | Records which components are installed |
| [Template](./template-spec.md) | Components are lighter-weight alternatives to templates |
| [Validation](./validation-spec.md) | Defines policies that components must satisfy |
| [Security Model](./security-model.md) | Governs component sandbox and path safety |
| [Update Model](./update-model.md) | Applies updates to components |
| [CLI UX Spec](./cli-ux-spec.md) | Defines `forge add` and `forge remove` |
| [Architecture](./architecture.md) | Defines the Component Engine module |

---

## 15. Open Questions

The following questions remain open and should be resolved before
implementation:

- Should components support **optional dependencies** (a dependency
  that is used if present but not required)?
- Should components support **weak conflicts** (a warning if present
  together, but not a failure)?
- Should the composition model support **transforms** (e.g., a
  component that modifies another component's files rather than just
  contributing)?
- How should a component declare that it requires a specific
  **Blueprint extension point** rather than a specific file?
- Should `owns` support **globs** (e.g., `src/**/*.py`), or is exact
  matching sufficient?
- Should components be able to **provide the same capability at
  different levels** (e.g., `database: postgres` and `database:
  postgres-embedded`)?
- How should version constraints interact with `provides` (e.g., two
  components that both provide `database: postgres`)?
- Should components be able to declare **post-install validation
  scripts** that run in the sandbox, or is validation always
  declarative?

These questions will be addressed in Phase 2 as implementation
begins.

---

## 16. Status

**Draft.**

This specification is under active development during Phase 1. It
becomes **Approved** when:

- The manifest schema is frozen for version 1
- All validation rules are tested
- The composition model is validated with at least 3 non-trivial
  examples
- All example components validate against the schema
- Ownership and conflict resolution are tested
- Open questions have been resolved or explicitly deferred
- The specification has been reviewed for consistency with
  [`docs/blueprint-spec.md`](./blueprint-spec.md) § 11 and
  [`docs/template-spec.md`](./template-spec.md) § 8
