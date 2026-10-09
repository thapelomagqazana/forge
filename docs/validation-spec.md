# Validation Specification

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

This specification defines what "valid" means for Forge artifacts and
repositories. It exists to answer:

- What does Forge validate?
- What are the categories of validation?
- What severity levels exist and what do they mean?
- What is the shape of a validation result?
- What rules exist?
- How is validation extended?
- How are results made machine-readable?
- What does a good remediation message look like?

Validation is the mechanism by which Forge turns declarative
foundations into verifiable properties. It is the backbone of
`forge validate`, `forge check`, and (later) `forge diff`.

---

## 2. Scope

**In scope:**

- Validation categories
- Severity levels
- Validation result model
- Validation rule catalogue
- Extensibility model
- Machine-readable output
- Remediation model
- Determinism requirements

**Out of scope:**

- Drift detection algorithm (see
  [`docs/update-model.md`](./update-model.md))
- Blueprint schema (see
  [`docs/blueprint-spec.md`](./blueprint-spec.md))
- Template format (see
  [`docs/template-spec.md`](./template-spec.md))
- Component format (see
  [`docs/component-spec.md`](./component-spec.md))
- Security policy definitions (see
  [`docs/security-model.md`](./security-model.md))
- CLI command surface for validation (see
  [`docs/cli-ux-spec.md`](./cli-ux-spec.md))

---

## 3. Conceptual Model

Validation in Forge is the process of comparing a **declared
expectation** against **observed reality**.

```text
┌────────────────────┐          ┌────────────────────┐
│    EXPECTATION     │          │     REALITY        │
│                    │          │                    │
│  Blueprint         │          │  Repository state  │
│  + forge.yaml      │  ────►   │  File existence    │
│  + Component       │  compare │  File content      │
│    manifests       │          │  File structure    │
│  + Policies        │          │  Command presence  │
│                    │          │                    │
└─────────┬──────────┘          └─────────┬──────────┘
          │                               │
          └───────────────┬───────────────┘
                          ▼
                ┌────────────────────┐
                │    VALIDATOR       │
                └─────────┬──────────┘
                          │ produces
                          ▼
                ┌────────────────────┐
                │  VALIDATION RESULT │
                │                    │
                │  List of findings  │
                │  Each with a rule  │
                │  ID, status,       │
                │  severity, message │
                │                    │
                └────────────────────┘
```

### 3.1 What Validation Is

- A **deterministic** comparison of expectation vs. reality
- A **per-rule** evaluation (each rule is independent)
- A **reporting** mechanism (validation does not modify the
  repository)
- A **contract** between Forge and the developer

### 3.2 What Validation Is Not

- A **fixer** (validation reports, it does not repair)
- A **linter** (validation evaluates foundation state, not code
  quality)
- A **scanner** (validation checks declared rules, not heuristics)
- A **security audit** (security is one category among several)

### 3.3 Validation Determinism

Given the same repository state and the same rules, validation
produces the same findings in the same order. This is required for:

- Reproducibility
- Testability
- CI stability
- Diffing between runs

Nondeterminism (e.g., unordered iteration, timestamps in findings) is
a bug.

---

## 4. Validation Categories

Validation rules are grouped into categories. Each category
represents a distinct domain of concern.

### 4.1 Category List

| Category | Concern | Introduced in |
|----------|---------|---------------|
| **Schema** | Structural correctness of configuration files | Phase 3 |
| **Structure** | Repository directory and file layout | Phase 7 |
| **Configuration** | Configuration file content | Phase 7 |
| **Template** | Template conformance and rendering | Phase 4 |
| **Component** | Component conformance and composition | Phase 12 |
| **Repository** | Repository-level properties | Phase 7 |
| **Policy** | Declared foundation policies | Phase 8 |
| **Security** | Security baseline conformance | Phase 7 |

### 4.2 Schema Validation

Validates the structural correctness of Forge configuration files.

**Examples:**

- `forge.yaml` is valid YAML
- `forge.yaml` conforms to the `forge.yaml` schema
- Blueprint conforms to the Blueprint schema
- Component manifest conforms to the Component schema

**Not in scope:** semantic correctness (that is policy).

### 4.3 Structure Validation

Validates the repository's directory and file layout against the
declared foundation.

**Examples:**

- `tests/` directory exists (if testing is required)
- `src/` directory exists (if the language convention requires it)
- `.github/workflows/` exists (if CI is configured)
- `README.md` exists
- No forbidden files present

### 4.4 Configuration Validation

Validates the content of configuration files.

**Examples:**

- `.gitignore` includes required patterns
- `pyproject.toml` includes required dependencies
- `package.json` includes required scripts
- `Dockerfile` includes required directives

Configuration validation is format-aware: YAML, TOML, JSON, and INI
are parsed structurally, not by regex.

### 4.5 Template Validation

Validates templates before and after rendering.

**Examples:**

- Template manifest is valid
- Template variables resolve
- Rendered output matches expected structure
- No unresolved placeholders remain
- No file escapes the target directory

Template validation occurs at two points:

1. When a template is installed or published (author-time)
2. When a repository is generated or checked (consumer-time)

### 4.6 Component Validation

Validates components and their composition.

**Examples:**

- Component manifest is valid
- All dependencies are satisfied
- No conflicts with installed components
- All owned files exist
- All contributions are present
- Ownership has no overlaps

### 4.7 Repository Validation

Validates repository-level properties that do not fit other
categories.

**Examples:**

- Repository has a `.git` directory (if Git is required)
- Repository has a `LICENSE` file
- Repository has a `.gitignore`
- Repository size is within a reasonable bound

### 4.8 Policy Validation

Validates against declared foundation policies.

**Examples:**

- Testing is required → `tests/` must exist
- CI is required → CI workflow must exist
- Documentation is required → required docs must exist
- Docker is required → `Dockerfile` must exist

Policy validation is the mechanism for expressing foundation
requirements. It is introduced in Phase 8.

### 4.9 Security Validation

Validates security baseline conformance.

**Examples:**

- `.env` is in `.gitignore`
- `.env.example` exists (if environment config is required)
- No obvious secret patterns are committed
- CI workflow declares minimum permissions
- Dependency scanning is configured

Security validation is **evidence-based**: it reports the presence or
absence of controls, not the security of the repository. Forge does
**not** claim "this repository is secure."

### 4.10 Category Independence

Categories are independent. A rule belongs to exactly one category.
A validation failure in one category does not prevent evaluation of
other categories.

Example:

```text
Structure: FAIL (tests/ missing)
Configuration: PASS
Security: PASS
Policy: FAIL (testing policy unsatisfied)
```

All four categories are evaluated and reported.

---

## 5. Severity Levels

Every finding has a severity level. Severity reflects the
consequence of the finding, not its category.

### 5.1 The Three Levels

| Severity | Meaning | Affects exit code |
|----------|---------|-------------------|
| `INFO` | Informational; no action required | No |
| `WARNING` | Action recommended; not blocking | No (by default) |
| `ERROR` | Required condition violated; blocking | Yes |

### 5.2 INFO

**Meaning:** A noteworthy observation that does not indicate a
problem.

**Examples:**

- "Docker is not configured (optional for this foundation)"
- "Test coverage report not found (not required)"
- "Repository has no license file (no policy requires one)"

**Behaviour:**

- Displayed in normal output
- Does not affect exit code
- May be suppressed with `--quiet`
- Included in JSON output

### 5.3 WARNING

**Meaning:** A condition worth attention that does not violate a
required rule.

**Examples:**

- "README exists but is empty"
- ".env is not ignored (security policy is advisory)"
- "CI workflow is present but does not run tests (policy allows)"

**Behaviour:**

- Displayed in normal output
- Does not affect exit code (by default)
- May be configured to affect exit code in CI mode
- Included in JSON output

### 5.4 ERROR

**Meaning:** A required condition is violated. The repository does
not conform to its declared foundation.

**Examples:**

- "tests/ is required by policy but does not exist"
- "CI workflow is required but is missing"
- "forge.yaml is malformed"

**Behaviour:**

- Displayed in normal output
- Always affects exit code
- Cannot be suppressed
- Included in JSON output
- Fails CI

### 5.5 Severity Assignment

Severity is assigned by the **rule**, not inferred at runtime.
Policies may override a rule's default severity (§ 8.4).

### 5.6 Severity Summary

The overall validation status is the **maximum severity** of any
finding:

| Highest finding severity | Overall status |
|--------------------------|----------------|
| (no findings) | `PASS` |
| `INFO` | `PASS` |
| `WARNING` | `WARNING` |
| `ERROR` | `FAIL` |

---

## 6. Validation Result Model

### 6.1 Result Shape

A validation run produces a **result**:

```json
{
  "status": "warning",
  "summary": {
    "errors": 0,
    "warnings": 2,
    "info": 3,
    "passed": 42
  },
  "findings": [
    {
      "rule": "VAL-101",
      "status": "failed",
      "severity": "warning",
      "category": "documentation",
      "message": "README.md is empty",
      "location": {
        "path": "README.md",
        "line": null,
        "column": null
      },
      "remediation": "Add content to README.md."
    }
  ]
}
```

### 6.2 Fields

| Field | Type | Description |
|-------|------|-------------|
| `status` | enum | Overall status: `pass`, `warning`, `fail` |
| `summary` | object | Counts of findings by severity |
| `findings` | list | Individual findings (see § 6.3) |

### 6.3 Finding Fields

| Field | Type | Description |
|-------|------|-------------|
| `rule` | string | Rule ID (e.g., `VAL-101`) |
| `status` | enum | `passed`, `failed`, `skipped`, `error` |
| `severity` | enum | `info`, `warning`, `error` (only present when `status` is `failed` or `error`) |
| `category` | string | One of the categories in § 4.1 |
| `message` | string | Human-readable description |
| `location` | object | Where the finding applies (path, line, column) |
| `evidence` | list | Optional supporting data |
| `remediation` | string | Suggested action |

### 6.4 The `passed` Status

Rules that pass produce a `passed` finding. This is included in the
result by default to enable complete reporting. It may be omitted
with `--no-passed` (a future flag).

Example:

```json
{
  "rule": "VAL-001",
  "status": "passed",
  "category": "structure",
  "message": "README.md exists"
}
```

### 6.5 The `skipped` Status

Rules that do not apply produce a `skipped` finding. For example, a
rule about `Dockerfile` is skipped if the foundation does not require
Docker.

Example:

```json
{
  "rule": "VAL-201",
  "status": "skipped",
  "category": "configuration",
  "message": "Docker not configured; Dockerfile rule skipped"
}
```

### 6.6 The `error` Status

Rules that fail to evaluate produce an `error` finding. This is
distinct from a rule finding — it means the validator could not
determine whether the rule passed.

Example:

```json
{
  "rule": "VAL-301",
  "status": "error",
  "severity": "error",
  "category": "configuration",
  "message": "Could not parse pyproject.toml: unexpected EOF at line 42"
}
```

An `error` status affects the overall status as if it were an `ERROR`.

---

## 7. Validation Rules

### 7.1 Rule ID Format

Rules use the format:

```text
<CATEGORY-PREFIX>-<NNN>
```

| Category | Prefix |
|----------|--------|
| Schema | `VAL-SCH` |
| Structure | `VAL-STR` |
| Configuration | `VAL-CFG` |
| Template | `VAL-TPL` |
| Component | `VAL-CMP` |
| Repository | `VAL-REP` |
| Policy | `VAL-POL` |
| Security | `VAL-SEC` |

Example: `VAL-STR-001` is the first structure rule.

For simplicity in early phases, the prefix may be shortened to
`VAL-NNN` with the category recorded as a field on the finding. The
full prefix format is used when rules are numerous enough that
categorisation matters.

### 7.2 Core Rules (Phase 7)

The following rules are required in Phase 7 (Foundation Validation).

#### 7.2.1 Structure Rules

| Rule | Severity | Description |
|------|----------|-------------|
| `VAL-STR-001` | ERROR | `README.md` exists at repository root |
| `VAL-STR-002` | ERROR | `.gitignore` exists at repository root |
| `VAL-STR-003` | ERROR | `tests/` directory exists (if testing policy requires) |
| `VAL-STR-004` | ERROR | `src/` or equivalent source directory exists (if policy requires) |
| `VAL-STR-005` | ERROR | `.github/workflows/` exists (if CI policy requires) |

#### 7.2.2 Configuration Rules

| Rule | Severity | Description |
|------|----------|-------------|
| `VAL-CFG-001` | ERROR | `forge.yaml` parses as valid YAML |
| `VAL-CFG-002` | ERROR | `forge.yaml` conforms to schema |
| `VAL-CFG-003` | WARNING | `pyproject.toml` exists (if Python project) |
| `VAL-CFG-004` | WARNING | `package.json` exists (if Node project) |
| `VAL-CFG-005` | WARNING | `go.mod` exists (if Go project) |

#### 7.2.3 Policy Rules

| Rule | Severity | Description |
|------|----------|-------------|
| `VAL-POL-001` | ERROR | Testing policy: tests exist and are discoverable |
| `VAL-POL-002` | ERROR | CI policy: workflow exists and is valid |
| `VAL-POL-003` | WARNING | Documentation policy: required docs exist |
| `VAL-POL-004` | ERROR | Security policy: baseline controls present |

#### 7.2.4 Security Rules

| Rule | Severity | Description |
|------|----------|-------------|
| `VAL-SEC-001` | WARNING | `.env` is listed in `.gitignore` |
| `VAL-SEC-002` | WARNING | `.env.example` exists (if env config required) |
| `VAL-SEC-003` | WARNING | No `.env` file is committed |
| `VAL-SEC-004` | WARNING | CI workflow declares minimum permissions |

#### 7.2.5 Repository Rules

| Rule | Severity | Description |
|------|----------|-------------|
| `VAL-REP-001` | ERROR | Repository is a Git repository |
| `VAL-REP-002` | WARNING | `LICENSE` file exists |

### 7.3 Rule Immutability

Once assigned, a rule ID is never reused or renamed. Retired rules
remain in the catalogue with a `retired` status.

### 7.4 Rule Sources

Rules are defined by one of three sources:

| Source | Meaning |
|--------|---------|
| **Built-in** | Provided by Forge |
| **Blueprint** | Declared by the referenced Blueprint |
| **Policy** | Declared in `forge.yaml` under `policies` |

The full rule set for a validation run is the union of:

1. Built-in rules (always evaluated)
2. Blueprint rules (evaluated for the referenced Blueprint)
3. Policy rules (evaluated for each enabled policy)

### 7.5 Rule Precedence

If a policy overrides the severity of a built-in rule, the policy's
severity wins. This is documented in § 8.4.

---

## 8. Extensibility

Validation must be extensible without breaking existing rules or
introducing unsafe execution.

### 8.1 The Extensibility Boundary

Forge's validation is **not** a general-purpose rules engine. It is a
curated set of rules that reflect Forge's opinionated view of
engineering foundations.

Extensibility is provided at three levels:

| Level | Mechanism | Introduced in |
|-------|-----------|---------------|
| **1** | Built-in rules | Phase 7 |
| **2** | Policy configuration | Phase 8 |
| **3** | Custom rules via plugins | Future phase |

Levels 1 and 2 are sufficient for the MVP and Phase 8. Level 3 is
defined here but not implemented.

### 8.2 Built-In Rules

Built-in rules are compiled into Forge. They are:

- Written in Go
- Versioned with Forge
- Consistent across all repositories
- Not user-modifiable

Adding a built-in rule requires a Forge release.

### 8.3 Policy Configuration

Policies are declared in `forge.yaml`:

```yaml
policies:
  testing:
    required: true
    severity: error
  ci:
    required: true
    severity: error
  security:
    required: true
    severity: warning
  documentation:
    required: false
```

Policies control:

- Which rules apply (via `required`)
- The severity of rules when they fail (via `severity`)
- Exemptions (§ 8.5)

Policy configuration does **not** introduce new rules. It only
configures the behaviour of existing rules.

### 8.4 Severity Overrides

A policy may override the default severity of a rule:

```yaml
policies:
  security:
    required: true
    severity: warning    # Overrides default severity
```

Default severities are documented in § 7.2. Overrides are:

- Explicit (documented in `forge.yaml`)
- Deterministic (no runtime inference)
- Logged in validation output

Example finding with override:

```json
{
  "rule": "VAL-SEC-001",
  "severity": "warning",
  "message": ".env is not listed in .gitignore",
  "severity_source": "policy:security"
}
```

The `severity_source` field records that the severity was overridden.

### 8.5 Exemptions

Policies may declare exemptions:

```yaml
policies:
  documentation:
    required: true
    severity: warning
    exemptions:
      - path: docs/generated/
        reason: "Auto-generated documentation"
```

Exemptions apply to specific paths or rules.

**Rules for exemptions:**

- Every exemption must declare a `reason`
- Every exemption is visible in validation output
- Every exemption is recorded in the state file
- Exemptions never suppress ERRORs, only downgrade them to
  `skipped`

Exempted findings are reported as `skipped`, not `passed`. This
keeps the exemption visible.

### 8.6 Custom Rules (Future)

Custom rules via plugins are **not** supported in the MVP or Phase 8.

When custom rules are introduced, they must:

- Declare their rule IDs in a reserved range
- Be sandboxed (no arbitrary code execution)
- Be versioned
- Be visible in validation output with a `source: plugin:<id>` field

The boundary is defined here so that plugin support does not require
retrofitting the validation result model.

### 8.7 Why Not a Full Rules Engine

Forge deliberately avoids a general-purpose rules engine (e.g.,
Rego, CEL) for the MVP and Phase 8. The reasons:

- **Determinism**: A curated rule set is easier to keep deterministic
- **Explainability**: Every rule is documented and traceable
- **Security**: Rules cannot execute arbitrary code
- **Performance**: A compiled rule set is faster than an interpreted
  one

A full rules engine may be introduced in a later phase if evidence
supports it.

---

## 9. Machine-Readable Results

### 9.1 JSON Output

`forge validate`, `forge check`, and `forge diff` all support
`--format json`. The JSON output conforms to a versioned schema.

### 9.2 Schema Version

Every JSON result includes a `schemaVersion` field:

```json
{
  "schemaVersion": "1",
  "forgeVersion": "0.1.0",
  "status": "fail",
  "summary": { ... },
  "findings": [ ... ]
}
```

The `schemaVersion` increments on breaking changes.

### 9.3 Complete Example

```json
{
  "schemaVersion": "1",
  "forgeVersion": "0.1.0",
  "command": "validate",
  "status": "fail",
  "repository": {
    "path": ".",
    "name": "payments-api"
  },
  "summary": {
    "errors": 1,
    "warnings": 2,
    "info": 1,
    "passed": 18,
    "skipped": 3,
    "total": 25
  },
  "findings": [
    {
      "rule": "VAL-STR-003",
      "status": "failed",
      "severity": "error",
      "category": "structure",
      "message": "tests/ directory is missing",
      "location": {
        "path": "tests/",
        "line": null,
        "column": null
      },
      "remediation": "Create tests/ or disable the testing policy."
    },
    {
      "rule": "VAL-SEC-001",
      "status": "failed",
      "severity": "warning",
      "category": "security",
      "message": ".env is not listed in .gitignore",
      "location": {
        "path": ".gitignore",
        "line": null,
        "column": null
      },
      "remediation": "Add '.env' to .gitignore."
    },
    {
      "rule": "VAL-REP-001",
      "status": "passed",
      "category": "repository",
      "message": "Repository is a Git repository"
    }
  ]
}
```

### 9.4 Determinism

JSON output is deterministic:

- Findings are ordered by rule ID
- Within the same rule, findings are ordered by location
- No timestamps in the core result (except `last_validated`, if
  present)
- No machine-specific data (hostname, username, absolute paths)

### 9.5 Compatibility

The JSON schema is versioned. Adding fields does not require a
version bump. Removing or renaming fields does.

Consumers of the JSON output must check `schemaVersion` before
parsing.

### 9.6 SARIF Output (Future)

`forge check` will support `--format sarif` in Phase 10. The SARIF
output maps Forge findings to SARIF results:

| Forge field | SARIF field |
|-------------|-------------|
| `rule` | `ruleId` |
| `severity` | `level` |
| `location.path` | `artifactLocation.uri` |
| `location.line` | `region.startLine` |
| `message` | `message.text` |
| `remediation` | `fixes[].description.text` |

SARIF is used for integration with code-scanning tools.

### 9.7 Human-Readable Output

The default output is human-readable. See
[`docs/cli-ux-spec.md`](./cli-ux-spec.md) § 4.3 for the exact format.

---

## 10. Remediation Model

### 10.1 Every Failure Answers Four Questions

Every finding must answer:

1. **What failed?** — The rule and its description
2. **Why?** — The reason the rule failed
3. **Where?** — The location (file, line if applicable)
4. **How can it be fixed?** — A specific remediation

### 10.2 Good vs. Bad Remediation

**Bad:**

```text
✗ Structure validation failed
```

**Better:**

```text
✗ tests/ directory is required
```

**Best:**

```text
✗ tests/ directory is required

The testing policy is enabled but no tests directory exists.

Create one of:
  tests/
  test/
  src/tests/

Or disable the policy:
  policies:
    testing:
      required: false
```

### 10.3 Remediation Format

The `remediation` field is a string containing:

- A clear statement of the action to take
- Where applicable, example commands
- Where applicable, the alternative (disable the policy)

Remediation strings are:

- **Actionable** (the developer knows what to do)
- **Specific** (not "fix the issue")
- **Safe** (does not suggest dangerous actions)

### 10.4 Remediation vs. Automation

Forge **does not** apply remediation automatically. Remediation is
guidance, not action.

The reasons:

- Automation can silently change foundations
- Developer intent may differ from Forge's default
- Remediation often requires context that Forge lacks

Future commands (`forge repair`, `forge fix`) may automate specific
remediations, but these must be explicit.

### 10.5 Remediation and CI

In CI, remediation is included in the output but not applied. The
developer must fix the issue in a subsequent commit.

For CI-specific workflows, see
[`docs/ci-spec.md`](./ci-spec.md) (Phase 10, future).

---

## 11. Validation in Different Contexts

### 11.1 `forge validate`

Validates `forge.yaml` and the current repository against the
declared foundation. All rules apply.

### 11.2 `forge check`

Validates foundation state continuously. This is the CI-oriented
command. The rule set is identical to `forge validate` for Phase 7.

The difference: `forge check` may be extended in Phase 9 with drift
detection rules that compare current state to a recorded baseline.

### 11.3 Template Validation

Validates a template in isolation (see
[`docs/template-spec.md`](./template-spec.md) § 12).

### 11.4 Component Validation

Validates a component in isolation (see
[`docs/component-spec.md`](./component-spec.md) § 12).

### 11.5 Validation on `forge new`

After `forge new` generates a repository, Forge runs validation on
the result. This ensures that the generated repository satisfies its
own foundation.

### 11.6 Validation on `forge update`

After `forge update` modifies a repository, Forge runs validation to
ensure the update did not break the foundation. If validation fails,
the update is rolled back.

---

## 12. Exit Codes

Validation affects the process exit code:

| Overall status | Exit code |
|----------------|-----------|
| `pass` | 0 |
| `warning` | 0 (by default) or 1 (with `--strict`) |
| `fail` | 1 |

`--strict` mode (future) treats `WARNING` as `ERROR` for exit code
purposes. This is useful in CI pipelines that require zero warnings.

Exit codes are consistent with
[`docs/cli-ux-spec.md`](./cli-ux-spec.md) § 7.

---

## 13. Example Validations

### 13.1 Passing Validation

```text
$ forge validate

Forge Validation
────────────────

Config
  ✓ forge.yaml is valid
  ✓ Schema conformance

Structure
  ✓ README.md exists
  ✓ .gitignore exists
  ✓ tests/ exists
  ✓ .github/workflows/ exists

Policy
  ✓ Testing policy satisfied
  ✓ CI policy satisfied
  ✓ Security policy satisfied

Result: PASS

20 passed, 0 warnings, 0 errors
```

### 13.2 Failing Validation

```text
$ forge validate

Forge Validation
────────────────

Config
  ✓ forge.yaml is valid

Structure
  ✓ README.md exists
  ✗ tests/ is missing
  ✓ .github/workflows/ exists

Policy
  ✗ Testing policy unsatisfied

Security
  ! .env is not listed in .gitignore

Result: FAIL

18 passed, 1 warning, 1 error

Fix these issues:
  1. Create tests/ or disable the testing policy.
  2. Add '.env' to .gitignore.
```

### 13.3 JSON Output

```json
{
  "schemaVersion": "1",
  "command": "validate",
  "status": "fail",
  "summary": {
    "errors": 1,
    "warnings": 1,
    "passed": 18
  },
  "findings": [
    {
      "rule": "VAL-STR-003",
      "status": "failed",
      "severity": "error",
      "category": "structure",
      "message": "tests/ is missing",
      "remediation": "Create tests/ or disable the testing policy."
    },
    {
      "rule": "VAL-SEC-001",
      "status": "failed",
      "severity": "warning",
      "category": "security",
      "message": ".env is not listed in .gitignore",
      "remediation": "Add '.env' to .gitignore."
    }
  ]
}
```

---

## 14. Validation Rule Catalogue

The following table lists all rules defined in Phase 7. Rules are
grouped by category.

### 14.1 Structure Rules

| Rule | Severity | Description |
|------|----------|-------------|
| `VAL-STR-001` | ERROR | `README.md` exists |
| `VAL-STR-002` | ERROR | `.gitignore` exists |
| `VAL-STR-003` | ERROR | `tests/` directory exists (if testing policy requires) |
| `VAL-STR-004` | ERROR | Source directory exists (if policy requires) |
| `VAL-STR-005` | ERROR | CI workflow directory exists (if CI policy requires) |

### 14.2 Configuration Rules

| Rule | Severity | Description |
|------|----------|-------------|
| `VAL-CFG-001` | ERROR | `forge.yaml` parses as valid YAML |
| `VAL-CFG-002` | ERROR | `forge.yaml` conforms to schema |
| `VAL-CFG-003` | WARNING | `pyproject.toml` exists (if Python) |
| `VAL-CFG-004` | WARNING | `package.json` exists (if Node) |
| `VAL-CFG-005` | WARNING | `go.mod` exists (if Go) |

### 14.3 Policy Rules

| Rule | Severity | Description |
|------|----------|-------------|
| `VAL-POL-001` | ERROR | Testing policy satisfied |
| `VAL-POL-002` | ERROR | CI policy satisfied |
| `VAL-POL-003` | WARNING | Documentation policy satisfied |
| `VAL-POL-004` | ERROR | Security policy satisfied |

### 14.4 Security Rules

| Rule | Severity | Description |
|------|----------|-------------|
| `VAL-SEC-001` | WARNING | `.env` is listed in `.gitignore` |
| `VAL-SEC-002` | WARNING | `.env.example` exists |
| `VAL-SEC-003` | WARNING | No `.env` file is committed |
| `VAL-SEC-004` | WARNING | CI workflow declares minimum permissions |

### 14.5 Repository Rules

| Rule | Severity | Description |
|------|----------|-------------|
| `VAL-REP-001` | ERROR | Repository is a Git repository |
| `VAL-REP-002` | WARNING | `LICENSE` file exists |

### 14.6 Rule Summary

| Category | Rules |
|----------|-------|
| Structure | 5 |
| Configuration | 5 |
| Policy | 4 |
| Security | 4 |
| Repository | 2 |
| **Total** | **20** |

More rules will be added in later phases (Template, Component).

---

## 15. Relationship to Other Specifications

| Specification | Relationship |
|---------------|--------------|
| [Blueprint](./blueprint-spec.md) | Blueprint declares which policies apply |
| [forge.yaml](./forge-yaml-spec.md) | Configuration that validation reads |
| [Template](./template-spec.md) | Templates are validated separately |
| [Component](./component-spec.md) | Components are validated separately |
| [Security Model](./security-model.md) | Security validation rules defined there |
| [Update Model](./update-model.md) | Updates trigger validation as a safety check |
| [CLI UX Spec](./cli-ux-spec.md) | Defines `forge validate` and `forge check` |
| [Architecture](./architecture.md) | Defines the Validator module |

---

## 16. Open Questions

The following questions remain open and should be resolved before
implementation:

- Should `forge validate` and `forge check` remain separate commands,
  or should `check` be an alias for `validate` with drift detection
  added?
- Should validation output include a **timeline** (when was this
  rule last passing)?
- Should validation support **rule-level exemptions** (a specific rule
  is disabled for a specific path), or only policy-level exemptions?
- Should validation be **incremental** (only re-evaluate rules whose
  inputs changed since the last run)?
- Should rules support **parameters** (e.g., a "required files" rule
  parameterised by a list)?
- Should validation results be **cached** between runs?
- How should validation handle **symlinks** in the repository?
- Should `--strict` mode treat `WARNING` as `ERROR` for exit codes, or
  should it be a distinct severity?
- Should findings include a **confidence** level (for rules that
  involve heuristics)?
- Should validation results be **stored** in the repository (e.g., as
  `.forge/validation.json`)?

These questions will be addressed in Phase 2 as implementation begins.

---

## 17. Status

**Draft.**

This specification is under active development during Phase 1. It
becomes **Approved** when:

- All validation categories are documented
- Severity levels are frozen
- The result model is stable
- The rule catalogue is complete for Phase 7
- Extensibility boundaries are confirmed
- Machine-readable output schema is stable
- The remediation model is tested with real examples
- Open questions have been resolved or explicitly deferred
- The specification has been reviewed for consistency with
  [`docs/cli-ux-spec.md`](./cli-ux-spec.md) and
  [`docs/blueprint-spec.md`](./blueprint-spec.md)
