# Template Specification

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

This specification defines how Forge **templates** work: what they
are, how they are structured, what they contain, and how they
materialise a Blueprint into a repository.

A template is the implementation mechanism that turns a declarative
Blueprint into actual files. It is subordinate to the Blueprint — the
Blueprint declares intent, the template materialises it.

This specification exists to answer:

- What is the structure of a template?
- What does the template manifest contain?
- How are variables resolved?
- How is conditional generation expressed?
- How is file ownership tracked?
- How is deterministic output guaranteed?
- How are templates tested?

---

## 2. Scope

**In scope:**

- Template directory structure
- Template manifest schema
- Variable model and resolution
- Conditional generation rules
- File ownership model
- Determinism requirements
- Template testing requirements
- Example templates

**Out of scope:**

- Blueprint schema (see
  [`docs/blueprint-spec.md`](./blueprint-spec.md))
- Component format (see
  [`docs/component-spec.md`](./component-spec.md))
- Registry and distribution (see
  [`docs/registry-spec.md`](./registry-spec.md), future)
- Update algorithm (see
  [`docs/update-model.md`](./update-model.md))
- Filesystem security (see
  [`docs/security-model.md`](./security-model.md))

---

## 3. Conceptual Model

A template is a **declarative, deterministic, sandboxed**
specification of how to produce the files for a Blueprint.

```text
┌──────────────────┐
│    BLUEPRINT     │  "What should this project be?"
└────────┬─────────┘
         │ consumed by
         ▼
┌──────────────────┐
│    TEMPLATE      │  "How is that materialised into files?"
└────────┬─────────┘
         │ rendered by
         ▼
┌──────────────────┐
│   RENDERER       │  Forge's template rendering engine
└────────┬─────────┘
         │ produces
         ▼
┌──────────────────┐
│   REPOSITORY     │
└──────────────────┘
```

### 3.1 What a Template Is

- A directory of files with a manifest
- A declarative specification (not a program)
- A deterministic producer of file content
- Sandboxed (no arbitrary command execution)
- Versioned

### 3.2 What a Template Is Not

- A program (templates are not scripts)
- An arbitrary code execution environment
- A plugin (templates do not extend Forge's behaviour)
- A package manager (templates do not install dependencies)
- A build system (templates do not build the generated project)
- A source of side effects (templates write only within the target
  directory)

### 3.3 The Sandbox Principle

The most important design decision about templates:

> **Templates render files. Templates do not execute code.**

Templates:

- Cannot run shell commands
- Cannot install packages
- Cannot make network requests
- Cannot read files outside the template
- Cannot write files outside the target directory

Any capability that requires execution is provided by Forge itself,
never by templates. This keeps templates safe to distribute and safe
to install from any source.

---

## 4. Template Structure

### 4.1 Directory Layout

A template is a directory with a canonical structure:

```text
<template>/
├── template.yaml       # manifest (required)
├── files/              # file templates (required)
├── tests/              # template tests (optional, but recommended)
└── README.md           # template documentation (recommended)
```

### 4.2 `template.yaml`

The template manifest. Describes the template's identity,
compatibility, variables, and configuration. Required.

### 4.3 `files/`

Contains the file tree that will be rendered. Every file inside
`files/` is a template file. Directory structure is preserved.

Example:

```text
files/
├── README.md.tmpl
├── pyproject.toml.tmpl
├── src/
│   └── {{package_name}}/
│       ├── __init__.py
│       └── main.py.tmpl
└── tests/
    └── test_health.py.tmpl
```

Directory names may also contain template expressions (e.g.,
`{{package_name}}`), allowing the target directory name to depend on
variables.

### 4.4 `tests/`

Contains fixture-based tests for the template. Used by `forge
template validate` to verify that the template renders, builds, and
tests correctly. Optional but strongly recommended.

### 4.5 `README.md`

Describes the template for template authors and for `forge template
list` / `forge template inspect`. Recommended.

### 4.6 Reserved Names

The following names are reserved and may not appear inside `files/`:

- `template.yaml`
- `template.lock`
- `forge.yaml`
- `metadata.yaml`

Attempting to render these files into the target repository causes
template validation to fail. This prevents templates from
accidentally overwriting Forge-managed files.

---

## 5. Template Manifest

### 5.1 Structure

`template.yaml` is a YAML document describing the template:

```yaml
name: python-fastapi
version: 1.0.0
description: Python FastAPI service foundation
author: Forge maintainers
license: Apache-2.0
compatibility:
  language:
    - python
  framework:
    - fastapi
  project_types:
    - service
    - web-application
variables:
  - name: package_name
    type: string
    required: true
    description: Python package name
    default: null
    pattern: "^[a-z][a-z0-9_]*$"
files:
  - path: README.md.tmpl
    target: README.md
  - path: pyproject.toml.tmpl
    target: pyproject.toml
  - path: src/{{package_name}}/main.py.tmpl
    target: src/{{package_name}}/main.py
```

### 5.2 Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Template identifier |
| `version` | semver | Template version |
| `description` | string | One-line description |
| `compatibility` | object | Languages, frameworks, project types this template supports |
| `files` | list | Files to render (see § 5.5) |

### 5.3 Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `author` | string | Template author |
| `license` | string | SPDX license identifier |
| `homepage` | string | URL to template documentation |
| `variables` | list | Declared variables (see § 6) |
| `hooks` | object | Reserved for future use; not supported in schema v1 |
| `requires` | object | Minimum Forge version (see § 5.6) |

### 5.4 Compatibility Block

```yaml
compatibility:
  language:
    - python
  framework:
    - fastapi
  project_types:
    - service
    - web-application
  blueprints:
    - python-api
```

| Field | Meaning |
|-------|---------|
| `language` | Languages the template supports |
| `framework` | Frameworks the template supports |
| `project_types` | Project types the template supports |
| `blueprints` | Blueprint IDs the template explicitly supports |

A template is compatible with a Blueprint when the Blueprint's
language, framework, and project type are all listed in the
template's compatibility. Compatibility is checked before rendering.

If `framework` is omitted from the Blueprint, the template's
`framework` list is ignored for compatibility purposes — the
template must still render successfully without a framework value.

### 5.5 Files Block

Each entry maps a template file to its rendered target path:

```yaml
files:
  - path: README.md.tmpl
    target: README.md
    condition: testing.enabled       # optional
    mode: "0644"                     # optional, defaults to 0644
  - path: pyproject.toml.tmpl
    target: pyproject.toml
  - path: src/{{package_name}}/main.py.tmpl
    target: src/{{package_name}}/main.py
```

| Field | Required | Description |
|-------|----------|-------------|
| `path` | Yes | Path to the template file relative to `files/` |
| `target` | Yes | Target path relative to the repository root |
| `condition` | No | Expression that must be true to include this file |
| `mode` | No | Unix file mode (defaults to `0644`) |

Both `path` and `target` may contain template expressions
(`{{variable}}`). The expressions in `path` are resolved against the
template's variables to locate the file. The expressions in `target`
are resolved against the Blueprint to compute the output path.

### 5.6 Requires Block

```yaml
requires:
  forge: ">=0.1.0 <1.0.0"
```

Declares the minimum (and optionally maximum) Forge version that can
render this template. If the running Forge does not satisfy the
constraint, rendering fails.

### 5.7 Hooks

Hooks are **not supported** in schema version 1. Any `hooks` block
present in a template's manifest causes template validation to fail:

```text
✗ Template validation failed

Template: python-fastapi@1.0.0

Reason: 'hooks' is not supported in schema version 1.

Templates render files. They do not execute code.
See docs/template-spec.md § 3.3 for the sandbox principle.
```

Hooks may be introduced in a future schema version with explicit
consent, sandboxing, and platform controls. They are deliberately
excluded from Phase 1.

---

## 6. Variables

### 6.1 Source of Variables

Variables come from three sources, in precedence order:

1. **Blueprint values** — values from the Blueprint's sections
2. **Template defaults** — values declared in the manifest
3. **Built-in derived values** — computed by the renderer

The renderer combines these into a single **render context**. Later
sources do not override earlier ones. If a template declares a
default for a variable that the Blueprint also provides, the
Blueprint value wins.

### 6.2 Built-In Variables

The renderer provides the following built-in variables:

| Variable | Source | Example |
|----------|--------|---------|
| `project_name` | `project.name` | `payments-api` |
| `project_type` | `project.type` | `service` |
| `language_name` | `language.name` | `python` |
| `language_version` | `language.version` | `3.13` |
| `framework_name` | `framework.name` (if present) | `fastapi` |
| `framework_version` | `framework.version` (if present) | `0.115` |
| `database_name` | `database.name` (if present) | `postgres` |
| `database_version` | `database.version` (if present) | `16` |
| `testing_enabled` | `testing.enabled` | `true` |
| `container_enabled` | `container.enabled` | `false` |
| `ci_provider` | `ci.provider` (if present) | `github-actions` |
| `security_baseline` | `security.baseline` | `standard` |

Templates reference these by their short name:

```text
# {{project_name}}
```

### 6.3 Declared Variables

Templates may declare additional variables in the manifest:

```yaml
variables:
  - name: package_name
    type: string
    required: true
    description: Python package name
    default: null
    pattern: "^[a-z][a-z0-9_]*$"

  - name: include_docs
    type: boolean
    required: false
    default: true
```

| Field | Meaning |
|-------|---------|
| `name` | Variable identifier |
| `type` | `string`, `integer`, `boolean`, `version` |
| `required` | Whether the variable must be provided |
| `default` | Value used when not provided |
| `description` | Human-readable description |
| `pattern` | Optional regex constraint (strings only) |

Declared variables are resolved in this order:

1. Blueprint value (if the variable matches a Blueprint field)
2. Default from the manifest
3. Error if `required: true` and neither applies

### 6.4 Variable Syntax

Templates use Go-style `text/template` syntax:

```text
# {{.project_name}}

This is a {{.language_name}} project using {{.framework_name}}.
```

Or, when variables are declared at the top level (recommended):

```text
# {{project_name}}

This is a {{language_name}} project using {{framework_name}}.
```

Forge supports both forms. The shorthand form (no leading `.`) is
preferred for template authors. It resolves against the render
context's top-level keys.

### 6.5 Variable Resolution Rules

- If a variable is **missing** and **not required**, its default is
  used. If no default is declared, the rendering fails.
- If a variable is **missing** and **required**, rendering fails with
  an actionable error.
- If a variable is **present but the wrong type**, rendering fails.
- If a variable is **present but violates its pattern**, rendering
  fails.

Example error:

```text
✗ Template rendering failed

Template: python-fastapi@1.0.0
File: pyproject.toml.tmpl

Variable 'package_name' is required but was not provided.

Provide it in the Blueprint:
  project:
    package_name: payments_api

Or set a default in the template manifest.
```

### 6.6 Derived Variables

The renderer computes the following derived variables automatically:

| Derived variable | Computed from |
|------------------|---------------|
| `package_name_snake` | `project_name` with `-` replaced by `_` |
| `package_name_kebab` | `project_name` with `_` replaced by `-` |
| `project_name_camel` | `project_name` in CamelCase |
| `project_name_pascal` | `project_name` in PascalCase |

These are available to all templates and can be used without
declaration. They are read-only.

### 6.7 No Arbitrary Expressions

Templates may **not** use arbitrary Go expressions beyond variable
substitution and simple conditionals. Specifically:

- No function calls
- No arithmetic beyond template's built-in `eq`, `ne`, `and`, `or`,
  `not`
- No access to the filesystem
- No environment variables
- No date/time
- No randomness

These restrictions enforce determinism (§ 8).

---

## 7. Conditional Generation

Templates may include or exclude files based on Blueprint values.

### 7.1 File-Level Conditions

A file's manifest entry may specify a condition:

```yaml
files:
  - path: Dockerfile.tmpl
    target: Dockerfile
    condition: container_enabled
  - path: tests/test_health.py.tmpl
    target: tests/test_health.py
    condition: testing_enabled
  - path: docker-compose.yaml.tmpl
    target: docker-compose.yaml
    condition: container_enabled and database_name == "postgres"
```

A file is rendered only if its condition evaluates to true.

### 7.2 Condition Syntax

Conditions are boolean expressions over variables:

| Operator | Meaning |
|----------|---------|
| `var` | True if the variable is truthy |
| `not var` | Negation |
| `var == "value"` | Equality |
| `var != "value"` | Inequality |
| `a and b` | Conjunction |
| `a or b` | Disjunction |

Parentheses are allowed for grouping.

Conditions do **not** support:

- Function calls
- Arithmetic
- Regular expressions
- Access to environment or filesystem

### 7.3 Inline Conditions

Template file content may also include inline conditionals:

```text
# {{project_name}}

{{if testing_enabled}}
## Testing

Run tests with:
    pytest
{{end}}
```

This allows a single file to conditionally include content based on
Blueprint values.

Inline conditionals use the same operators as file-level conditions.

### 7.4 Common Conditions

| Condition | Meaning |
|-----------|---------|
| `testing_enabled` | Testing is enabled |
| `container_enabled` | Containerisation is enabled |
| `ci_provider == "github-actions"` | GitHub Actions is configured |
| `database_name == "postgres"` | PostgreSQL is the database |
| `framework_name == "fastapi"` | FastAPI is the framework |
| `not testing_enabled` | Testing is disabled |
| `testing_enabled and container_enabled` | Both are enabled |

### 7.5 Determinism Under Conditions

Conditional generation is deterministic: given the same Blueprint, the
same files are included or excluded on every render. This is required
by § 8.

---

## 8. File Ownership

### 8.1 Ownership Concept

Every file that Forge generates has an **owner**. Ownership records
which template, component, or developer is responsible for the file.
This information is essential for future update operations
(§ [Update Model](./update-model.md)).

### 8.2 Ownership Values

| Owner | Meaning |
|-------|---------|
| `template:<id>@<version>` | Created by a template |
| `component:<id>@<version>` | Created by a component |
| `developer` | Created or modified by the developer |
| `shared` | Modified by both Forge and the developer |

### 8.3 Ownership Recording

Ownership is recorded in the repository's Forge state (not in
`forge.yaml` itself, to keep `forge.yaml` human-readable). The state
file lives at:

```text
.forge/
└── state.yaml
```

Example state:

```yaml
schema: 1
files:
  - path: README.md
    owner: template:python-fastapi@1.0.0
    hash: sha256:abc123...
  - path: pyproject.toml
    owner: template:python-fastapi@1.0.0
    hash: sha256:def456...
  - path: src/payments_api/main.py
    owner: template:python-fastapi@1.0.0
    hash: sha256:ghi789...
    modified: true
```

The `modified` flag indicates that the developer has changed the file
since it was generated.

### 8.4 Ownership Inheritance

When a file is generated by a template, ownership is:

1. Set to `template:<id>@<version>` on creation
2. Preserved across renders unless the file is deleted
3. Updated to `shared` if the developer modifies the file
4. Reset to `template:<id>@<version>` if `forge update` rewrites the
   file and the developer accepts the rewrite

### 8.5 Ownership and Deletion

When a template no longer produces a file (e.g., because a condition
changed), the file is **not** automatically deleted. Instead:

- `forge check` reports the file as "orphaned" (created by a
  template but no longer part of it)
- The developer may delete it, or keep it, or add it to a `keep` list

Forge never silently deletes files during ordinary operations.
Deletion requires an explicit user action.

### 8.6 Ownership and Non-Template Files

Files that were not created by a template or component have no entry
in the state file. Their ownership is implicitly `developer`.

---

## 9. Deterministic Requirements

### 9.1 The Determinism Guarantee

> **Given the same Blueprint, the same template version, and the same
> Forge version, rendering produces byte-identical output on every
> run and on every platform.**

Determinism is required for:

- Reproducible builds
- Safe updates (base state comparison)
- Cross-platform consistency
- Test suite golden files

### 9.2 Sources of Nondeterminism (Prohibited)

The following are **not permitted** in templates:

| Source | Why it breaks determinism |
|--------|---------------------------|
| Timestamps | Different on every render |
| Random values | Different on every render |
| UUIDs | Different on every render |
| Environment variables | Vary by machine |
| Filesystem iteration order | Varies by OS |
| Hostname or username | Varies by machine |
| Local file paths | Vary by machine |
| Network requests | Vary by time and network |
| Command execution | Varies by environment |

Templates that attempt any of these are rejected by template
validation.

### 9.3 Ordering

The renderer produces files in a deterministic order:

1. Files are sorted by target path (lexicographic, case-sensitive,
   `/` as separator)
2. Directories are created before their contents
3. File metadata (permissions) is applied after content is written

This ordering is stable across platforms.

### 9.4 Line Endings

Templates produce output with `\n` line endings by default.

If a template needs to produce `\r\n` (e.g., for a Windows-only
project), this must be declared explicitly in the manifest:

```yaml
settings:
  line_endings: crlf
```

Default: `lf`. On all platforms, the renderer writes the declared
line endings, not the platform default. This preserves determinism
across Windows, macOS, and Linux.

### 9.5 Encoding

All template files must be UTF-8 without BOM. Templates that are not
valid UTF-8 fail validation.

### 9.6 Binary Files

Binary files (images, compiled artifacts) are supported via the
`binary: true` flag:

```yaml
files:
  - path: assets/logo.png
    target: assets/logo.png
    binary: true
```

Binary files are copied verbatim; no rendering is applied.

### 9.7 Determinism Verification

`forge template validate` runs every template twice and compares the
outputs byte-for-byte. Any difference causes validation to fail.

---

## 10. Template Testing

Templates are first-class artifacts and must be tested. This section
defines the required test framework.

### 10.1 Test Categories

| Test | Purpose |
|------|---------|
| **Render test** | Renders the template with a fixture Blueprint and compares output to a golden file |
| **Structure test** | Verifies the generated directory structure matches expectations |
| **Build test** | Attempts to build the generated project |
| **Test execution** | Runs the generated project's test suite |

Not every test category applies to every template. A template that
generates a non-buildable artifact (e.g., documentation) may skip
build and test execution.

### 10.2 Test Fixtures

Fixtures live in `tests/fixtures/`:

```text
tests/
└── fixtures/
    ├── minimal/
    │   ├── blueprint.yaml
    │   └── expected/
    │       ├── README.md
    │       └── pyproject.toml
    ├── with-postgres/
    │   ├── blueprint.yaml
    │   └── expected/
    │       └── ...
    └── without-docker/
        ├── blueprint.yaml
        └── expected/
            └── ...
```

Each fixture is:

- A Blueprint (input)
- An `expected/` directory (golden output)

A template passes a fixture when the rendered output matches the
`expected/` directory byte-for-byte.

### 10.3 Fixture Manifest

Each fixture may declare a `fixture.yaml`:

```yaml
description: Minimal Python API with no database
commands:
  - name: build
    command: pip install -e .
  - name: test
    command: pytest
```

The `commands` are optional and used only for build and test
execution.

### 10.4 Test Execution

`forge template validate` runs all fixtures:

```text
$ forge template validate ./templates/python-fastapi

Forge Template Validation
─────────────────────────

Manifest
  ✓ name: python-fastapi
  ✓ version: 1.0.0
  ✓ description present
  ✓ compatibility valid

Variables
  ✓ All required variables declared
  ✓ Patterns valid
  ✓ Defaults valid

Rendering
  ✓ Fixture 'minimal' renders successfully
  ✓ Fixture 'with-postgres' renders successfully
  ✓ Fixture 'without-docker' renders successfully

Determinism
  ✓ Byte-identical output on second render

Path safety
  ✓ No path traversal
  ✓ No absolute paths
  ✓ No symlink escapes

Structure
  ✓ No reserved names in files/
  ✓ No duplicate target paths

Result: PASS
```

### 10.5 Golden File Testing

Golden files (the `expected/` directories) are committed to the
template repository. When a template changes intentionally, golden
files must be regenerated and committed explicitly. Changes to golden
files appear in code review.

### 10.6 Continuous Integration

Every template's tests run in CI. A template cannot be published or
certified unless all tests pass.

### 10.7 Test Coverage

Templates should cover:

- The minimal case (only required variables)
- The complete case (all optional features enabled)
- Each conditional branch (`testing_enabled`, `container_enabled`,
  each `database_name` value)
- Each supported project type
- Each supported framework

Coverage is measured by fixture count. A template without fixtures
for each conditional branch fails validation.

---

## 11. Example Templates

The following templates are planned for Phase 5 (MVP) and will be
included in Forge's bundled templates.

### 11.1 `python-fastapi`

```text
python-fastapi/
├── template.yaml
├── README.md
├── files/
│   ├── README.md.tmpl
│   ├── pyproject.toml.tmpl
│   ├── Dockerfile.tmpl
│   ├── compose.yaml.tmpl
│   ├── .gitignore
│   ├── src/
│   │   └── {{package_name}}/
│   │       ├── __init__.py
│   │       ├── main.py.tmpl
│   │       └── config.py.tmpl
│   └── tests/
│       ├── __init__.py
│       └── test_health.py.tmpl
└── tests/
    └── fixtures/
        ├── minimal/
        ├── with-postgres/
        └── without-docker/
```

### 11.2 `go-api`

```text
go-api/
├── template.yaml
├── README.md
├── files/
│   ├── README.md.tmpl
│   ├── go.mod.tmpl
│   ├── Dockerfile.tmpl
│   ├── Makefile.tmpl
│   ├── cmd/
│   │   └── server/
│   │       └── main.go.tmpl
│   └── internal/
│       └── handler/
│           └── handler.go.tmpl
└── tests/
    └── fixtures/
        └── minimal/
```

### 11.3 `go-cli`

```text
go-cli/
├── template.yaml
├── README.md
├── files/
│   ├── README.md.tmpl
│   ├── go.mod.tmpl
│   ├── Makefile.tmpl
│   └── cmd/
│       └── {{project_name}}/
│           └── main.go.tmpl
└── tests/
    └── fixtures/
        └── minimal/
```

### 11.4 `typescript-node`

```text
typescript-node/
├── template.yaml
├── README.md
├── files/
│   ├── README.md.tmpl
│   ├── package.json.tmpl
│   ├── tsconfig.json.tmpl
│   ├── Dockerfile.tmpl
│   └── src/
│       └── index.ts.tmpl
└── tests/
    └── fixtures/
        └── minimal/
```

### 11.5 `react-app`

```text
react-app/
├── template.yaml
├── README.md
├── files/
│   ├── README.md.tmpl
│   ├── package.json.tmpl
│   ├── vite.config.ts.tmpl
│   ├── index.html.tmpl
│   └── src/
│       ├── main.tsx.tmpl
│       └── App.tsx.tmpl
└── tests/
    └── fixtures/
        └── minimal/
```

---

## 12. Template Validation Rules

Every template is validated before use. The following rules apply.

### 12.1 Manifest Rules

| Rule | Description |
|------|-------------|
| TPL-001 | `template.yaml` exists and is valid YAML |
| TPL-002 | `name` is present and non-empty |
| TPL-003 | `version` is present and valid semver |
| TPL-004 | `description` is present and non-empty |
| TPL-005 | `compatibility` is present and valid |
| TPL-006 | `files` is present and non-empty |
| TPL-007 | No `hooks` block is present (schema v1) |
| TPL-008 | No unknown fields are present |

### 12.2 File Rules

| Rule | Description |
|------|-------------|
| TPL-101 | Every `files[].path` exists inside `files/` |
| TPL-102 | No `files[].path` escapes `files/` via `..` |
| TPL-103 | No `files[].target` escapes the repository root |
| TPL-104 | No duplicate `files[].target` values |
| TPL-105 | No reserved names appear as target paths |
| TPL-106 | All files inside `files/` are valid UTF-8 (unless marked binary) |
| TPL-107 | No file permissions are outside `0644`–`0755` |
| TPL-108 | Directory names inside `files/` do not collide with file targets |

### 12.3 Variable Rules

| Rule | Description |
|------|-------------|
| TPL-201 | Every variable used in templates is declared or built-in |
| TPL-202 | Every declared variable has a valid type |
| TPL-203 | Every `required: true` variable has no default |
| TPL-204 | Every `pattern` is a valid regular expression |

### 12.4 Determinism Rules

| Rule | Description |
|------|-------------|
| TPL-301 | Template renders byte-identically on repeated runs |
| TPL-302 | Template renders byte-identically across platforms |
| TPL-303 | No timestamps, randomness, or environment reads |

### 12.5 Rendering Rules

| Rule | Description |
|------|-------------|
| TPL-401 | Template renders successfully with each fixture |
| TPL-402 | Rendered output matches the fixture's `expected/` directory |
| TPL-403 | No unresolved variable placeholders remain in output |

### 12.6 Compatibility Rules

| Rule | Description |
|------|-------------|
| TPL-501 | Template declares at least one language |
| TPL-502 | Template declares at least one project type |
| TPL-503 | If `blueprints` is declared, they are valid Blueprint IDs |

### 12.7 Security Rules

| Rule | Description |
|------|-------------|
| TPL-601 | No path traversal in any target |
| TPL-602 | No absolute paths in any target |
| TPL-603 | No symlink targets |
| TPL-604 | No file writes outside the repository root |
| TPL-605 | No code execution mechanisms |

A template that fails any of these rules cannot be used.

---

## 13. Relationship to Other Specifications

| Specification | Relationship |
|---------------|--------------|
| [Blueprint](./blueprint-spec.md) | Templates materialise Blueprints |
| [forge.yaml](./forge-yaml-spec.md) | Records which template was used |
| [Component](./component-spec.md) | Components are lighter-weight templates that compose |
| [Validation](./validation-spec.md) | Defines policies that templates must satisfy |
| [Security Model](./security-model.md) | Governs template path safety and sandbox |
| [Update Model](./update-model.md) | Uses template output as base state for updates |
| [CLI UX Spec](./cli-ux-spec.md) | Defines `forge template list`, `forge new` |
| [Architecture](./architecture.md) | Defines the Template Engine and Renderer modules |

---

## 14. Open Questions

The following questions remain open and should be resolved before
implementation:

- Should templates support inheritance (a template extending
  another)?
- Should templates support partials (reusable fragments)?
- Should template files support a `.tmpl` suffix universally, or
  only when they contain template expressions?
- Should Forge provide a `forge template init` command to scaffold a
  new template from scratch?
- Should template validation run in a Docker container to isolate
  build/test commands, or rely on the host environment?
- How should templates handle files that require different content
  per platform (e.g., `.bat` vs `.sh`)?
- Should templates be able to declare that they are
  forward-compatible with future Forge versions?
- Should template output be checksummed and recorded in
  `forge.yaml` or in `.forge/state.yaml`?

These questions will be addressed in Phase 2 as implementation begins.

---

## 15. Status

**Draft.**

This specification is under active development during Phase 1. It
becomes **Approved** when:

- The manifest schema is frozen for version 1
- All validation rules are tested
- All example templates validate against the schema
- The sandbox principle is reviewed and confirmed
- Determinism guarantees are tested across platforms
- Open questions have been resolved or explicitly deferred
- The specification has been reviewed for consistency with
  [`docs/blueprint-spec.md`](./blueprint-spec.md) § 3 and
  [`docs/security-model.md`](./security-model.md)
