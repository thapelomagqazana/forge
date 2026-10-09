# Blueprint Specification

**Document type:** Specification
**Status:** Draft
**Version:** 0.1.1
**Author:** @thapelomagqazana
**Created:** 2026-10-09
**Last Updated:** 2026-10-09
**Supersedes:** —
**Superseded by:** —

---

## 1. Purpose

This specification defines what a Forge **Blueprint** is, what it
contains, how it is structured, and how it is validated.

A Blueprint is the declarative description of what a software project
should be. It is not a template. It is not a generator. It is the
source of truth that drives generation, validation, and future
evolution.

This specification exists to answer:

- What fields does a Blueprint contain?
- Which are required, which are optional?
- What types are valid?
- What defaults apply?
- How is a Blueprint versioned?
- How is a Blueprint validated?
- How is a Blueprint extended in future phases without breaking
  compatibility?

---

## 2. Scope

**In scope:**

- Blueprint schema
- Required and optional fields
- Type system
- Default values and justification
- Versioning model
- Validation rules
- Extensibility mechanism
- Example Blueprints

**Out of scope:**

- How a Blueprint is materialised into files (see
  [`docs/template-spec.md`](./template-spec.md))
- How components compose into a Blueprint (see
  [`docs/component-spec.md`](./component-spec.md))
- How policies are defined (see
  [`docs/validation-spec.md`](./validation-spec.md))
- The `forge.yaml` file that references a Blueprint (see
  [`docs/forge-yaml-spec.md`](./forge-yaml-spec.md))

---

## 3. Conceptual Model

A Blueprint describes **what a project should be**, not **how it is
materialised**.

```text
┌────────────────────────────────────────┐
│              BLUEPRINT                 │
│                                        │
│  "This is a Python API using FastAPI,  │
│   PostgreSQL, pytest, Docker, and      │
│   GitHub Actions, following the         │
│   standard security baseline."          │
│                                        │
└─────────────────┬──────────────────────┘
                  │
                  ▼
        ┌───────────────────┐
        │     TEMPLATE      │
        │                   │
        │  "Here are the    │
        │   files to create │
        │   for that."      │
        └─────────┬─────────┘
                  │
                  ▼
        ┌───────────────────┐
        │    REPOSITORY     │
        └───────────────────┘
```

The Blueprint is the declaration. The Template is the implementation.
The Repository is the result.

This separation is the central architectural decision of Forge,
recorded in
[ADR-001](./decisions/ADR-001-forge-as-foundation-manager.md).

### 3.1 What a Blueprint Is

- A declarative description of a project's engineering foundation
- A versioned artifact
- A verifiable specification
- Independent of any specific template
- Portable across languages and frameworks

### 3.2 What a Blueprint Is Not

- A template (templates materialise blueprints)
- A file manifest (blueprints declare intent, not file lists)
- A generator (generators execute against blueprints)
- A code sample (blueprints describe structure, not application code)
- A build configuration (build configuration is generated from the
  blueprint, not defined within it)

---

## 4. Blueprint Schema

### 4.1 Top-Level Structure

A Blueprint is a YAML document with the following top-level sections:

```yaml
forge:         # Forge metadata (required)
  blueprint:   # Blueprint identifier
  version:     # Blueprint schema version

project:       # Project identity (required)
  name:
  type:

language:      # Primary language (required)
  name:
  version:

framework:     # Framework (optional)
  name:
  version:

database:      # Database (optional)
  name:
  version:

testing:       # Testing strategy (optional)
  enabled:
  framework:

container:     # Containerisation (optional)
  enabled:
  runtime:

ci:            # CI provider (optional)
  provider:

security:      # Security baseline (optional)
  baseline:

documentation: # Documentation requirements (optional)
  required:
```

Every top-level section other than `forge`, `project`, and `language`
is optional. Optional sections may be omitted entirely or included
with partial content.

### 4.2 Minimal Valid Blueprint

The smallest valid Blueprint contains only required fields:

```yaml
forge:
  blueprint: minimal
  version: 1

project:
  name: my-project
  type: service

language:
  name: go
```

This Blueprint declares:

- A project named `my-project`
- Of type `service`
- Using Go
- With no other foundation requirements

### 4.3 Complete Blueprint

A fuller Blueprint declares more:

```yaml
forge:
  blueprint: python-api
  version: 1

project:
  name: payments-api
  type: service

language:
  name: python
  version: "3.13"

framework:
  name: fastapi

database:
  name: postgres
  version: "16"

testing:
  enabled: true
  framework: pytest

container:
  enabled: true
  runtime: docker

ci:
  provider: github-actions

security:
  baseline: standard

documentation:
  required:
    - README.md
    - ARCHITECTURE.md
```

---

## 5. Required Fields

Required fields must be present for a Blueprint to be valid. Missing
required fields cause validation to fail.

### 5.1 `forge.blueprint`

| Attribute | Value |
|-----------|-------|
| Type | string |
| Required | Yes |
| Description | Identifier of the Blueprint |
| Constraint | Non-empty, lowercase, alphanumeric, hyphens |
| Example | `python-api`, `go-service`, `minimal` |

The `blueprint` field identifies **which** Blueprint is being
declared. It is used for reference, provenance, and validation.

### 5.2 `forge.version`

| Attribute | Value |
|-----------|-------|
| Type | integer |
| Required | Yes |
| Description | Blueprint schema version |
| Constraint | Positive integer |
| Example | `1` |

The `version` field identifies **which schema version** this Blueprint
conforms to. It is not the version of the Blueprint itself (see § 9).
It is the version of the schema.

### 5.3 `project.name`

| Attribute | Value |
|-----------|-------|
| Type | string |
| Required | Yes |
| Description | Project name |
| Constraint | Non-empty, lowercase, alphanumeric, hyphens, underscores |
| Example | `payments-api`, `my_service` |

The project name is used as an identifier throughout generation. It
appears in generated file names, configuration, and metadata.

### 5.4 `project.type`

| Attribute | Value |
|-----------|-------|
| Type | enum |
| Required | Yes |
| Description | High-level category of the project |
| Allowed values | `service`, `cli`, `library`, `web-application`, `worker`, `monorepo` |
| Example | `service` |

The project type influences which templates are compatible, which
components are available, and which defaults apply.

### 5.5 `language.name`

| Attribute | Value |
|-----------|-------|
| Type | enum |
| Required | Yes |
| Description | Primary language |
| Allowed values | See § 5.6 |
| Example | `python` |

The language determines template compatibility and language-specific
defaults.

### 5.6 Supported Languages

The following languages are supported in the MVP. Additional languages
may be added in later phases.

| Language | Identifier | Minimum version |
|----------|-----------|-----------------|
| Python | `python` | 3.12 |
| Go | `go` | 1.22 |
| TypeScript | `typescript` | 5.0 |
| Rust | `rust` | 1.75 |

Attempting to use an unsupported language causes validation to fail.

---

## 6. Optional Fields

Optional fields may be omitted. When omitted, they either have no
effect or assume a documented default (see § 8).

### 6.1 `language.version`

| Attribute | Value |
|-----------|-------|
| Type | string (version) |
| Required | No |
| Default | Language-specific (see § 8.1) |
| Description | Minimum language version |
| Example | `"3.13"` |

### 6.2 `framework`

| Attribute | Value |
|-----------|-------|
| Type | object |
| Required | No |
| Description | Application framework |
| Subfields | `name` (string, required if `framework` is present), `version` (string, optional) |

Example:

```yaml
framework:
  name: fastapi
  version: "0.115"
```

Supported framework names are constrained per language. Using an
unsupported framework causes validation to fail.

### 6.3 `database`

| Attribute | Value |
|-----------|-------|
| Type | object |
| Required | No |
| Description | Database requirement |
| Subfields | `name` (enum), `version` (string, optional) |

Allowed database names: `postgres`, `mysql`, `sqlite`, `mongodb`,
`redis`.

Example:

```yaml
database:
  name: postgres
  version: "16"
```

### 6.4 `testing`

| Attribute | Value |
|-----------|-------|
| Type | object |
| Required | No |
| Default | `enabled: true` |
| Description | Testing strategy |
| Subfields | `enabled` (boolean), `framework` (string, optional) |

Example:

```yaml
testing:
  enabled: true
  framework: pytest
```

### 6.5 `container`

| Attribute | Value |
|-----------|-------|
| Type | object |
| Required | No |
| Default | `enabled: false` |
| Description | Containerisation requirement |
| Subfields | `enabled` (boolean), `runtime` (enum, optional) |

Allowed runtime values: `docker`, `podman`.

Example:

```yaml
container:
  enabled: true
  runtime: docker
```

### 6.6 `ci`

| Attribute | Value |
|-----------|-------|
| Type | object |
| Required | No |
| Default | No default |
| Description | Continuous integration provider |
| Subfields | `provider` (enum) |

Allowed provider values: `github-actions`, `gitlab-ci`, `none`.

Example:

```yaml
ci:
  provider: github-actions
```

### 6.7 `security`

| Attribute | Value |
|-----------|-------|
| Type | object |
| Required | No |
| Default | `baseline: standard` |
| Description | Security baseline |
| Subfields | `baseline` (enum) |

Allowed baseline values: `minimal`, `standard`, `strict`.

| Baseline | Meaning |
|----------|---------|
| `minimal` | Only environment-file exclusion |
| `standard` | Environment exclusion, dependency scanning, minimum CI permissions |
| `strict` | Standard + secret scanning, SBOM generation, signed commits |

Example:

```yaml
security:
  baseline: standard
```

### 6.8 `documentation`

| Attribute | Value |
|-----------|-------|
| Type | object |
| Required | No |
| Default | `required: [README.md]` |
| Description | Documentation requirements |
| Subfields | `required` (list of strings) |

Example:

```yaml
documentation:
  required:
    - README.md
    - ARCHITECTURE.md
    - CONTRIBUTING.md
```

Each listed file becomes a requirement that `forge check` verifies.

---

## 7. Type System

Blueprint fields use a constrained type system. Every field has exactly
one type. Types are validated before any defaults are applied.

### 7.1 Supported Types

| Type | Description | Example |
|------|-------------|---------|
| `string` | Text | `"payments-api"` |
| `integer` | Whole number | `1`, `16` |
| `boolean` | True/false | `true`, `false` |
| `version` | Version string | `"3.13"`, `"16.2"` |
| `enum` | Constrained string | `"python"`, `"fastapi"` |
| `list` | Ordered collection | `["README.md", "LICENSE"]` |
| `object` | Nested structure | `{ name: "fastapi" }` |

### 7.2 Type Rules

- **Strings** — must be quoted when numeric or boolean-like
- **Integers** — unquoted, no leading zeros
- **Booleans** — `true` or `false`, unquoted
- **Versions** — quoted strings, valid semver or major-version format
- **Enums** — must match one of the allowed values exactly
- **Lists** — YAML sequences, homogeneous types
- **Objects** — YAML mappings with documented subfields

### 7.3 Type Coercion

Forge does **not** coerce types. A value that is not the expected type
causes validation to fail:

```yaml
testing:
  enabled: "true"    # Invalid: string, not boolean
```

Correct:

```yaml
testing:
  enabled: true
```

### 7.4 Unknown Fields

Unknown fields cause validation to fail. This prevents typos from
silently creating false configuration:

```yaml
project:
  name: payments-api
  typo_field: true    # Invalid: unknown field
```

Error message:

```text
✗ Invalid Blueprint

project.typo_field
  Unknown field.

Did you mean:
  project.type
```

---

## 8. Default Values

Defaults are applied only to optional fields that are omitted. Every
default is documented and justified.

### 8.1 `language.version`

**Default:** Language-specific minimum.

| Language | Default version |
|----------|-----------------|
| Python | `"3.12"` |
| Go | `"1.22"` |
| TypeScript | `"5.0"` |
| Rust | `"1.75"` |

**Justification:** The default should be the oldest supported version,
not the newest. This ensures generated projects run in the widest
range of environments. Users who want a newer version can specify it
explicitly.

### 8.2 `testing.enabled`

**Default:** `true`.

**Justification:** Forge's premise is that engineering foundations
should include testing. A project that does not include testing is the
exception, not the rule. Making `testing.enabled` default to `true`
aligns the default with Forge's thesis and with modern engineering
practice.

A user who genuinely does not want testing sets
`testing.enabled: false` explicitly. This is deliberate.

### 8.3 `container.enabled`

**Default:** `false`.

**Justification:** Not all projects need containerisation. Defaulting
to `false` avoids imposing a container workflow on projects that do
not require one. Users who want containers enable it explicitly.

### 8.4 `security.baseline`

**Default:** `standard`.

**Justification:** Security is a core engineering concern, but different
projects have different security requirements. `standard` includes the
minimum viable security baseline (environment-file exclusion,
dependency scanning, minimum CI permissions) without imposing
enterprise-grade requirements.

A user who wants stronger security sets `baseline: strict`. A user who
wants only the bare minimum sets `baseline: minimal`.

### 8.5 `documentation.required`

**Default:** `["README.md"]`.

**Justification:** Every project should have a README. Requiring more
by default (e.g., ARCHITECTURE.md, CONTRIBUTING.md) would be
opinionated and inappropriate for small projects. Users who want more
documentation declare it explicitly.

### 8.6 Defaults That Are Not Applied

The following fields have **no** default:

| Field | Behaviour when omitted |
|-------|------------------------|
| `framework` | No framework assumed |
| `database` | No database assumed |
| `ci` | No CI provider assumed |

**Justification:** These fields depend on the project's specific
requirements. Guessing would be inappropriate. When omitted, the
corresponding capability is simply not part of the foundation.

### 8.7 Default Application Order

Defaults are applied in a fixed order:

1. Parse YAML into internal representation
2. Validate required fields
3. Apply defaults to omitted optional fields
4. Validate types and enums
5. Validate semantic rules

Defaults are **not** reapplied after validation. If a user provides a
value, it is used as-is. If a user omits a field, its default is
applied. Defaults never override explicit values, including explicit
`false` or empty strings (where those are valid).

---

## 9. Versioning

Forge has four distinct version concepts. They must not be conflated.

| Version | Purpose | Example |
|---------|---------|---------|
| **Blueprint schema version** | Which schema the Blueprint conforms to | `1` |
| **Blueprint instance version** | Which version of a specific Blueprint | `2.4.0` |
| **Forge version** | Which Forge binary produced or validated | `0.1.0` |
| **Template version** | Which implementation template was used | `1.0.0` |

### 9.1 Blueprint Schema Version

The Blueprint schema is versioned via `forge.version`. The schema
version is an integer that increments on **breaking changes** to the
schema.

- Adding optional fields does not require a version bump
- Adding required fields requires a version bump
- Removing fields requires a version bump
- Changing field types requires a version bump
- Changing default values does not require a version bump

Current schema version: **1**.

### 9.2 Blueprint Instance Version

A specific Blueprint (e.g., the `python-api` Blueprint maintained by
Forge) has its own version. This is a semver string that follows the
conventions in
[`docs/documentation-guide.md`](./documentation-guide.md).

The instance version is not part of the Blueprint file itself. It is
tracked externally (e.g., in the registry, in `forge.yaml`).

### 9.3 Forge Version

The Forge binary version is recorded in generated projects and in
metadata. It is used for compatibility checks and provenance.

The Forge version is not part of the Blueprint file.

### 9.4 Template Version

The template version indicates which implementation template was used
to materialise the Blueprint. It is a semver string.

The template version is not part of the Blueprint file. It is recorded
in the generated project's `forge.yaml`.

### 9.5 Schema Version Compatibility

| Forge version | Supported Blueprint schema versions |
|---------------|-------------------------------------|
| `0.1.x` | `1` |
| `0.2.x` | `1`, `2` (planned) |
| `1.0.x` | `1`, `2` |

When Forge encounters a Blueprint with an unsupported schema version,
it fails with a clear error:

```text
✗ Unsupported Blueprint version

Blueprint version: 2
Supported versions: 1

Upgrade Forge to a version that supports Blueprint version 2.
```

### 9.6 Schema Version Migration

When schema version 2 is introduced, Forge will provide a migration
tool (planned for a later phase) to convert version 1 Blueprints to
version 2. Until then, version 1 is the only supported schema.

---

## 10. Validation Rules

Blueprint validation occurs in two phases: **schema validation** and
**semantic validation**.

### 10.1 Schema Validation Rules

| Rule | Description |
|------|-------------|
| BP-001 | Blueprint is syntactically valid YAML |
| BP-002 | Root must be a mapping |
| BP-003 | `forge` section is present and is a mapping |
| BP-004 | `forge.blueprint` is present and is a non-empty string |
| BP-005 | `forge.version` is present and is a positive integer |
| BP-006 | `forge.version` is a supported schema version |
| BP-007 | `project` section is present and is a mapping |
| BP-008 | `project.name` is present and is a non-empty string |
| BP-009 | `project.name` matches the allowed character pattern |
| BP-010 | `project.type` is present and is a supported enum value |
| BP-011 | `language` section is present and is a mapping |
| BP-012 | `language.name` is present and is a supported language |
| BP-013 | All present fields have the correct type |
| BP-014 | All enum fields have an allowed value |
| BP-015 | No unknown fields are present |
| BP-016 | Optional nested sections (framework, database, etc.) are mappings if present |

### 10.2 Semantic Validation Rules

| Rule | Description |
|------|-------------|
| BP-101 | `language.version` is compatible with the language |
| BP-102 | `framework.name` is supported for the declared language |
| BP-103 | `framework.version`, if present, is a valid version |
| BP-104 | `database.name` is a supported database |
| BP-105 | `container.runtime` is a supported runtime |
| BP-106 | `ci.provider` is a supported provider |
| BP-107 | `security.baseline` is a supported baseline |
| BP-108 | `documentation.required` is a non-empty list of valid file names |
| BP-109 | If `framework` is present, its name is compatible with `project.type` |
| BP-110 | If `database` is present, its name is compatible with `language` |

### 10.3 Validation Result Format

Every validation failure produces a structured finding:

```json
{
  "code": "BP-008",
  "path": "project.name",
  "message": "project.name is required",
  "suggestion": "Provide a project name (e.g., 'payments-api')",
  "line": 8
}
```

Fields:

| Field | Meaning |
|-------|---------|
| `code` | The rule identifier |
| `path` | The location in the Blueprint (dot notation) |
| `message` | A human-readable description |
| `suggestion` | A suggested fix |
| `line` | The YAML line number (when available) |

### 10.4 Validation Determinism

Given the same Blueprint and the same Forge version, validation always
produces the same findings in the same order. This is required for
testing and for CI use.

---

## 11. Extensibility

Blueprints must evolve without breaking compatibility. The following
mechanisms preserve this property.

### 11.1 Optional Sections

New capabilities are added as **new optional sections**, never by
adding required fields to existing sections.

Example: A future "observability" capability is added as:

```yaml
observability:
  enabled: true
  provider: opentelemetry
```

Existing Blueprints that do not include `observability` remain valid.

### 11.2 New Optional Fields in Existing Sections

New optional fields may be added to existing sections without a schema
version bump.

Example: `framework.version` is an optional addition to the
`framework` section. Blueprints that omit it remain valid.

### 11.3 Component-Declared Configuration

Future components (Phase 12) may declare their own configuration
within a Blueprint's `components` section:

```yaml
components:
  postgres:
    enabled: true
    version: "16"
  redis:
    enabled: true
```

The `components` section is reserved for Phase 12. It is not part of
schema version 1.

### 11.4 Extension Points

New top-level sections can be added in future schema versions. The
following section names are reserved for future use:

- `observability`
- `monitoring`
- `logging`
- `secrets`
- `deployment`
- `components`
- `policies`

Using any of these section names in schema version 1 causes validation
to fail with a clear "reserved for future use" error.

### 11.5 Deprecation Policy

When a field is deprecated:

1. It remains valid in the current schema version
2. Using it produces a WARNING finding (not an ERROR)
3. The warning includes the replacement field, if any
4. The field is removed in the next major schema version

Example:

```yaml
container:
  enabled: true
  runtime: docker
  compose: true    # Deprecated in favour of components
```

Would produce:

```text
⚠ Deprecated field

container.compose
  Deprecated in favour of 'components'.

See https://forge.dev/docs/migration for details.
```

### 11.6 Reserved Keywords

The following keys are reserved and may not be used as field names in
any section:

- `forge`
- `version` (except within `forge`, `framework`, `database`, etc.)

If a user attempts to use these as custom field names, validation
fails.

---

## 12. Blueprint Examples

Every example below is valid against schema version 1.

### 12.1 Minimal Blueprint

The smallest valid Blueprint:

```yaml
forge:
  blueprint: minimal
  version: 1

project:
  name: my-project
  type: service

language:
  name: go
```

**Rationale:** Demonstrates the minimum required fields. All optional
sections are omitted. Defaults apply (testing enabled, security
baseline standard, documentation requires README.md).

### 12.2 Python API Blueprint

A Python API using FastAPI, PostgreSQL, pytest, Docker, and GitHub
Actions:

```yaml
forge:
  blueprint: python-api
  version: 1

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

testing:
  enabled: true
  framework: pytest

container:
  enabled: true
  runtime: docker

ci:
  provider: github-actions

security:
  baseline: standard

documentation:
  required:
    - README.md
    - ARCHITECTURE.md
```

**Rationale:** A realistic production-quality Python API foundation.

### 12.3 Go API Blueprint

A Go HTTP service using chi, PostgreSQL, Go testing, and Docker:

```yaml
forge:
  blueprint: go-api
  version: 1

project:
  name: inventory-api
  type: service

language:
  name: go
  version: "1.22"

framework:
  name: chi

database:
  name: postgres
  version: "16"

testing:
  enabled: true

container:
  enabled: true
  runtime: docker

ci:
  provider: github-actions

security:
  baseline: standard
```

**Rationale:** A Go service foundation aligned with Go conventions.

### 12.4 Go CLI Blueprint

A Go command-line application:

```yaml
forge:
  blueprint: go-cli
  version: 1

project:
  name: deploy-tool
  type: cli

language:
  name: go
  version: "1.22"

framework:
  name: cobra

testing:
  enabled: true

ci:
  provider: github-actions

security:
  baseline: standard
```

**Rationale:** A CLI foundation without containerisation or a
database. Demonstrates that container and database are truly optional.

### 12.5 TypeScript API Blueprint

A Node.js API using Express:

```yaml
forge:
  blueprint: typescript-node
  version: 1

project:
  name: notifications-api
  type: service

language:
  name: typescript
  version: "5.0"

framework:
  name: express

database:
  name: postgres
  version: "16"

testing:
  enabled: true
  framework: jest

container:
  enabled: true
  runtime: docker

ci:
  provider: github-actions

security:
  baseline: standard
```

**Rationale:** A TypeScript Node.js service foundation. Demonstrates
that the schema is language-agnostic.

---

## 13. Blueprint and forge.yaml Relationship

A Blueprint is a standalone declaration. `forge.yaml` is the
repository-level metadata that references a Blueprint and records
additional state.

```text
┌──────────────────┐          ┌──────────────────┐
│    BLUEPRINT     │          │    forge.yaml    │
│                  │          │                  │
│  Declares what   │◄─────────│  References a    │
│  a project is.   │          │  Blueprint and   │
│                  │          │  records state.  │
└──────────────────┘          └──────────────────┘
```

Example Blueprint (standalone):

```yaml
forge:
  blueprint: python-api
  version: 1

project:
  name: payments-api
  type: service

language:
  name: python
```

Example `forge.yaml` (in a repository):

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
```

The full `forge.yaml` specification is in
[`docs/forge-yaml-spec.md`](./forge-yaml-spec.md).

Key differences:

| Aspect | Blueprint | forge.yaml |
|--------|-----------|------------|
| Purpose | Declare intent | Record state |
| Source | Authored or referenced | Generated by `forge new` or `forge init` |
| Editing | By hand or by tooling | By Forge, rarely by hand |
| Contains metadata | No | Yes (Forge version, timestamps) |
| Portable | Yes | No (repository-specific) |

---

## 14. Examples Directory

Concrete Blueprint files are stored in `examples/blueprints/`:

```text
examples/blueprints/
├── minimal.yaml
├── python-fastapi.yaml
├── go-api.yaml
├── go-cli.yaml
├── typescript-node.yaml
└── react-app.yaml
```

Each file:

- Is valid against schema version 1
- Is used by tests in Phase 2 and Phase 3
- Is referenced by this specification

---

## 15. Open Questions

The following questions remain open and should be resolved before
implementation:

- Should `framework.version` be required when `framework.name` is
  provided, or always optional?
- Should `documentation.required` support glob patterns (e.g.,
  `docs/**/*.md`) or only literal file paths?
- Should `security.baseline` be extensible (custom baselines), or
  limited to `minimal`, `standard`, `strict`?
- Should the Blueprint include a `description` field for human
  readability?
- Should `project.type` be strictly enumerated, or should it allow
  custom types via a `custom:` prefix?
- Should Blueprint YAML files support anchors and aliases, or are
  those discouraged?
- When schema version 2 introduces reserved sections (e.g.,
  `observability`), how should schema version 1 Blueprints that
  already use those names as custom fields be migrated?
- Should the Blueprint file support comments (YAML does, but should
  Forge preserve them)?

These questions will be addressed in Phase 2 as implementation begins.

---

## 16. Status

**Draft.**

This specification is under active development during Phase 1. It
becomes **Approved** when:

- The schema is frozen for version 1
- All validation rules are tested
- All examples validate against the schema
- Open questions have been resolved or explicitly deferred
- The specification has been reviewed for consistency with
  [`docs/product-discovery.md`](./product-discovery.md) § 9.5 and
  [`docs/forge-yaml-spec.md`](./forge-yaml-spec.md)
