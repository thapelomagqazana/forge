# CLI UX Specification

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

This specification defines the Forge command-line interface before any
implementation begins. It exists to answer:

- What commands exist?
- What does each command do?
- What arguments and flags does each command accept?
- What does each command output?
- What does each command return as an exit code?
- What UX principles govern the interface?

The CLI is Forge's primary product surface. Getting it right before
coding prevents churn in Phase 2 and Phase 5.

---

## 2. Scope

**In scope:**

- Command hierarchy
- Per-command syntax, arguments, flags, and behaviour
- Interactive and non-interactive modes
- Dry-run semantics
- Exit code contract
- Machine-readable output contract
- CLI UX principles
- Human-readable output vocabulary

**Out of scope:**

- Implementation details (belongs in `docs/architecture.md`)
- Blueprint schema (belongs in `docs/blueprint-spec.md`)
- Template format (belongs in `docs/template-spec.md`)
- Update/merge algorithm (belongs in `docs/update-model.md`)
- Security model (belongs in `docs/security-model.md`)

---

## 3. Command Hierarchy

Forge's commands are organised into three tiers:

| Tier | Meaning | Availability |
|------|---------|--------------|
| **MVP** | Command is implemented in Phase 5 | Ships publicly |
| **Future** | Command is planned but not yet specified for Phase 5 | Documented here, implemented in named phase |
| **Experimental** | Command is under consideration; may be removed | Not shipped; subject to change |

### 3.1 Command Tree

```text
forge
├── new            (MVP)
├── init           (MVP)
├── validate       (MVP)
├── explain        (MVP)
├── template       (MVP, list only)
│   ├── list       (MVP)
│   ├── inspect    (Future — Phase 16)
│   ├── search     (Future — Phase 16)
│   ├── install    (Future — Phase 16)
│   └── publish    (Future — Phase 16)
├── check          (Future — Phase 7)
├── diff           (Future — Phase 9)
├── update         (Future — Phase 14)
├── add            (Future — Phase 12)
├── remove         (Future — Phase 12)
├── blueprint      (Experimental)
│   └── validate   (Experimental)
├── doctor         (Future — Phase 6)
└── version        (MVP)
```

### 3.2 Tier Assignments

| Command | Tier | First phase | Notes |
|---------|------|-------------|-------|
| `forge new` | MVP | Phase 5 | Core CREATE command |
| `forge init` | MVP | Phase 5 | Adopt existing repository |
| `forge validate` | MVP | Phase 5 | Foundation conformance |
| `forge explain` | MVP | Phase 5 | Explain foundation |
| `forge template list` | MVP | Phase 5 | List available templates |
| `forge version` | MVP | Phase 5 | Version information |
| `forge check` | Future | Phase 7 | Continuous verification |
| `forge diff` | Future | Phase 9 | Drift detection |
| `forge update` | Future | Phase 14 | Safe evolution |
| `forge add` | Future | Phase 12 | Component composition |
| `forge remove` | Future | Phase 12 | Component removal |
| `forge template inspect` | Future | Phase 16 | Registry inspection |
| `forge template search` | Future | Phase 16 | Registry search |
| `forge template install` | Future | Phase 16 | Registry installation |
| `forge template publish` | Future | Phase 16 | Registry publishing |
| `forge doctor` | Future | Phase 6 | Environment diagnostics |
| `forge blueprint validate` | Experimental | TBD | Blueprint validation |

---

## 4. Command Reference — MVP Commands

### 4.1 `forge new`

Creates a new repository from a foundation.

**Syntax:**

```text
forge new <project-name> [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `<project-name>` | Yes (or prompted) | Name of the project to create |

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--template <id>` | string | Interactive prompt | Template to use |
| `--output <path>` | string | `./<project-name>` | Destination directory |
| `--blueprint <path>` | string | — | Path to a blueprint file |
| `--non-interactive` | bool | `false` | Disable prompts; require all flags |
| `--dry-run` | bool | `false` | Show what would be created; write nothing |
| `--format <format>` | enum | `human` | `human` or `json` |
| `--quiet` | bool | `false` | Suppress non-error output |
| `--verbose` | bool | `false` | Show diagnostic output |

**Interactive mode:**

When `<project-name>` is omitted, or `--template` is not provided,
Forge enters an interactive wizard:

```text
Forge — New Project
────────────────────

Project name
> payments-api

What are you building?
❯ API
  CLI
  Web application
  Library
  Worker
  Service

Language
❯ Python
  Go
  TypeScript
  Rust

Framework
❯ FastAPI
  Flask
  None

Testing
❯ Pytest
  None

Containerisation
❯ Docker
  None

CI
❯ GitHub Actions
  None

Foundation preview
  Project: payments-api
  Template: python-fastapi@1.0.0
  Files to create: 12
  Destination: ./payments-api

Create project? [Y/n]
```

**Non-interactive mode:**

```bash
forge new payments-api \
  --template python-fastapi \
  --output ./projects \
  --non-interactive
```

All flags that would otherwise be prompted must be provided. If any
required input is missing, Forge fails with a structured error and
does not create anything.

**Defaults:**

When `<project-name>` is provided and `--template` is not, Forge may:

- Recommend a template based on the project name pattern (rare)
- Prompt interactively (default behaviour)
- Fail if `--non-interactive` is set

**Errors:**

- Invalid project name (empty, contains invalid characters)
- Template not found
- Destination directory already exists
- Invalid blueprint
- Blueprint incompatible with template
- Filesystem permission error

**Exit codes:**

| Condition | Code |
|-----------|------|
| Project created successfully | 0 |
| Validation failure | 1 |
| Invalid usage/input | 2 |
| Filesystem failure | 3 |
| Security violation | 4 |

**Output (human):**

```text
✓ Project created

payments-api/
Template: python-fastapi@1.0.0
Files: 12

Next steps:
  cd payments-api
  forge validate
```

**Output (JSON):**

```json
{
  "status": "success",
  "project": {
    "name": "payments-api",
    "path": "./payments-api",
    "template": "python-fastapi",
    "template_version": "1.0.0",
    "files_created": 12
  }
}
```

### 4.2 `forge init`

Adopts an existing repository into Forge's foundation model.

**Syntax:**

```text
forge init [flags]
```

**Arguments:**

None.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--foundation <id>` | string | Detected | Explicitly set the foundation |
| `--non-interactive` | bool | `false` | Disable prompts |
| `--dry-run` | bool | `false` | Show what would be written |
| `--format <format>` | enum | `human` | `human` or `json` |
| `--quiet` | bool | `false` | Suppress non-error output |
| `--verbose` | bool | `false` | Show diagnostic output |

**Detection behaviour:**

Forge scans the current directory for technology signals:

| Signal | Detects |
|--------|---------|
| `package.json` | Node.js, JavaScript, TypeScript |
| `pyproject.toml`, `setup.py` | Python |
| `go.mod` | Go |
| `Cargo.toml` | Rust |
| `pom.xml`, `build.gradle` | Java |
| `Dockerfile`, `compose.yaml` | Containerisation |
| `.github/workflows/` | GitHub Actions |
| `tests/`, `test/` | Testing |
| `README.md` | Documentation |

Detection produces a technology fingerprint with confidence levels
(HIGH / MEDIUM / LOW). Forge then resolves matching foundation
candidates.

**Prompts:**

```text
Forge — Repository Adoption
────────────────────────────

Scanning repository...

Detected:
  Language:   Python         (HIGH)
  Framework:  FastAPI        (HIGH)
  Testing:    Pytest         (HIGH)
  Container:  Docker         (MEDIUM)
  CI:         GitHub Actions (HIGH)

Recommended foundation:
  python-fastapi
  Match: HIGH

Why:
  ✓ pyproject.toml detected
  ✓ FastAPI dependency detected
  ✓ pytest configuration detected
  ✓ Dockerfile detected
  ✓ GitHub Actions workflow detected

Adopt this foundation? [Y/n]
```

**Generated metadata:**

`forge init` writes `forge.yaml` to the repository root:

```yaml
forge:
  version: 1
  foundation: python-fastapi
  foundation_version: 1.0.0

policies:
  testing:
    required: true
  ci:
    required: true
```

`forge init` does **not** modify source files, configuration files, or
any file other than `forge.yaml`.

**Unsupported repositories:**

If Forge cannot detect a foundation:

```text
✗ Could not detect a foundation

Detected technologies: none

Forge requires a foundation to adopt this repository.

Options:
  forge init --foundation <id>
  forge template list
```

**Failure behaviour:**

- Existing `forge.yaml` → fail with actionable message (do not
  overwrite)
- Ambiguous detection (multiple equally plausible foundations) → fail
  in non-interactive mode; prompt in interactive mode
- No `forge.yaml` can be written → filesystem error

**Exit codes:**

| Condition | Code |
|-----------|------|
| Initialized successfully | 0 |
| Initialization failure | 1 |
| Invalid usage/input | 2 |
| Filesystem failure | 3 |
| Ambiguous foundation (non-interactive) | 2 |
| Repository already initialized | 1 |

**Output (human):**

```text
✓ Repository initialized

forge.yaml created
Foundation: python-fastapi@1.0.0

Next steps:
  forge validate
  forge explain
```

### 4.3 `forge validate`

Validates `forge.yaml` and the current repository against the declared
foundation.

**Syntax:**

```text
forge validate [flags]
```

**Arguments:**

None.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--file <path>` | string | `./forge.yaml` | Path to the configuration file |
| `--format <format>` | enum | `human` | `human` or `json` |
| `--quiet` | bool | `false` | Suppress non-error output |
| `--verbose` | bool | `false` | Show diagnostic output |

**What is validated:**

| Category | Check |
|----------|-------|
| Configuration | `forge.yaml` is syntactically valid |
| Configuration | `forge.yaml` matches the Blueprint schema |
| Configuration | Required fields are present |
| Configuration | Field types are correct |
| Foundation | Referenced foundation exists |
| Foundation | Foundation version is supported |
| Foundation | Foundation is compatible with the repository |
| Structure | Required directories exist |
| Structure | Required files exist |
| Structure | No forbidden files present |
| Testing | Test infrastructure exists (if required) |
| CI | CI configuration exists (if required) |
| Documentation | README exists (if required) |
| Security | Security baseline exists (if required) |

**Errors:**

Validation returns a list of findings. Each finding has:

- Rule ID
- Severity (INFO, WARNING, ERROR)
- Category
- Message
- Location (file path, line if applicable)
- Remediation (suggested action)

**Exit codes:**

| Condition | Code |
|-----------|------|
| Validation passed | 0 |
| Validation failed (ERROR findings) | 1 |
| Invalid usage/input | 2 |
| Configuration file not found | 1 |
| Configuration file malformed | 1 |

**Output (human):**

```text
✓ forge.yaml is valid

Blueprint: payments-api
Foundation: python-fastapi@1.0.0
Template: python-fastapi@1.0.0

Checks
  ✓ Configuration
  ✓ Foundation reference
  ✓ Structure
  ✓ Testing
  ✓ CI
  ✓ Documentation
  ! Security — .env is not ignored

1 warning, 0 errors

Foundation status: WARNING
```

**Output (JSON):**

```json
{
  "status": "warning",
  "blueprint": "payments-api",
  "foundation": {
    "name": "python-fastapi",
    "version": "1.0.0"
  },
  "findings": [
    {
      "rule": "SEC-001",
      "severity": "warning",
      "category": "security",
      "message": ".env is not ignored",
      "location": ".gitignore",
      "remediation": "Add .env to .gitignore"
    }
  ],
  "summary": {
    "errors": 0,
    "warnings": 1,
    "info": 0
  }
}
```

### 4.4 `forge explain`

Explains the current repository's foundation.

**Syntax:**

```text
forge explain [flags]
```

**Arguments:**

None.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format <format>` | enum | `human` | `human` or `json` |
| `--verbose` | bool | `false` | Show extended detail |

**Output (human):**

```text
Project Foundation
──────────────────

Project
  payments-api

Blueprint
  python-api@2

Template
  python-fastapi@1.0.0

Components
  Python 3.13
  FastAPI
  PostgreSQL
  Pytest
  Docker
  GitHub Actions

Policies
  Testing .......... PASS
  CI ............... PASS
  Security ......... PASS
  Documentation .... WARN

Provenance
  Foundation source: forge.yaml
  Template source:   bundled
  Last validated:    2026-10-09T08:00:00Z
```

**Output (JSON):**

```json
{
  "schemaVersion": "1",
  "forgeVersion": "0.1.0",
  "project": {
    "name": "payments-api"
  },
  "blueprint": {
    "name": "python-api",
    "version": "2"
  },
  "template": {
    "name": "python-fastapi",
    "version": "1.0.0"
  },
  "components": [
    { "name": "Python", "version": "3.13", "role": "runtime" },
    { "name": "FastAPI", "role": "framework" },
    { "name": "PostgreSQL", "role": "database" },
    { "name": "Pytest", "role": "testing" },
    { "name": "Docker", "role": "container" },
    { "name": "GitHub Actions", "role": "ci" }
  ],
  "policies": [
    { "name": "testing", "status": "pass" },
    { "name": "ci", "status": "pass" },
    { "name": "security", "status": "pass" },
    { "name": "documentation", "status": "warning" }
  ],
  "provenance": {
    "foundation_source": "forge.yaml",
    "template_source": "bundled",
    "last_validated": "2026-10-09T08:00:00Z"
  }
}
```

### 4.5 `forge template list`

Lists locally available templates.

**Syntax:**

```text
forge template list [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format <format>` | enum | `human` | `human` or `json` |

**Output (human):**

```text
Available templates

  python-fastapi@1.0.0
    Python FastAPI foundation

  go-api@1.0.0
    Go HTTP service foundation

  go-cli@1.0.0
    Go command-line foundation

  typescript-node@1.0.0
    TypeScript Node.js API foundation

  react-app@1.0.0
    React application foundation
```

### 4.6 `forge version`

Displays version information.

**Syntax:**

```text
forge version [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format <format>` | enum | `human` | `human` or `json` |

**Output (human):**

```text
forge 0.1.0
  commit: abc123
  built:  2026-10-09T08:00:00Z
  go:     1.23.0
```

**Output (JSON):**

```json
{
  "version": "0.1.0",
  "commit": "abc123",
  "buildDate": "2026-10-09T08:00:00Z",
  "goVersion": "1.23.0"
}
```

---

## 5. Command Reference — Future Commands

Future commands are documented here for planning purposes. They are
**not** implemented in the MVP. Each is summarised with its intended
purpose and phase.

### 5.1 `forge check` (Phase 7)

Continuous verification that a repository satisfies its declared
foundation.

```text
forge check [flags]

--format human|json|sarif
--config <path>
--quiet
--verbose
```

Returns non-zero exit code on required-policy failure. Designed for CI
execution.

### 5.2 `forge diff` (Phase 9)

Shows drift between expected and actual foundation state.

```text
forge diff [flags]

--format human|json
--category <category>
--severity <severity>
--quiet
--verbose
```

Returns non-zero exit code when actionable drift is detected.

### 5.3 `forge update` (Phase 14)

Applies foundation changes to an existing repository.

```text
forge update [flags]

--dry-run
--conflicts
--rollback
--format human|json
--yes
```

Never overwrites developer changes without explicit confirmation.

### 5.4 `forge add` / `forge remove` (Phase 12)

Adds or removes a component from a foundation.

```text
forge add <component> [flags]
forge remove <component> [flags]

--dry-run
--yes
--format human|json
```

### 5.5 `forge template inspect|search|install|publish` (Phase 16)

Registry operations.

```text
forge template inspect <id> [flags]
forge template search <query> [flags]
forge template install <id>[@version] [flags]
forge template publish [flags]
```

### 5.6 `forge doctor` (Phase 6)

Diagnoses the Forge environment.

```text
forge doctor [flags]

--format human|json
```

Reports on Forge installation, configuration, templates, filesystem,
Git availability, and other dependencies.

### 5.7 `forge blueprint validate` (Experimental)

Validates a blueprint file independently of a repository.

```text
forge blueprint validate [<path>] [flags]

--format human|json
```

May be merged into `forge validate` or removed. Kept experimental
until Phase 3 clarifies the requirement.

---

## 6. Dry-Run Behaviour

Every mutating command must support `--dry-run`. Dry-run means:

- **No filesystem writes** — not even temporary files
- **No state changes** — `forge.yaml` is not modified
- **No external calls** — no registry requests, no Git operations

Dry-run output shows what *would* happen:

```text
$ forge new payments-api --dry-run

Would create:
  payments-api/
  ├── README.md
  ├── pyproject.toml
  ├── Dockerfile
  ├── compose.yaml
  ├── src/
  │   └── payments_api/
  │       └── main.py
  ├── tests/
  │   └── test_health.py
  └── .github/
      └── workflows/
          └── ci.yml

Template: python-fastapi@1.0.0
Files: 8

No files were created.
```

Dry-run exit codes match the corresponding non-dry-run command, except
that:

- Successful dry-run returns 0
- Dry-run that would fail validation returns 1
- Dry-run that would fail on filesystem conflict returns 3

Dry-run is **mandatory** for:

- `forge new`
- `forge init`
- `forge update`
- `forge add`
- `forge remove`

Dry-run is **optional** for:

- `forge validate` (already non-mutating)
- `forge check` (already non-mutating)
- `forge diff` (already non-mutating)
- `forge explain` (already non-mutating)
- `forge template list` (already non-mutating)

---

## 7. Exit Codes

Forge uses stable exit codes so that scripts and CI systems can
interpret results.

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Validation failure or operation failure |
| 2 | Invalid usage or invalid input |
| 3 | Filesystem failure |
| 4 | Security violation |
| 5 | Update conflict (Phase 14+) |

### 7.1 Exit Code Semantics

**0 — Success**
Command completed successfully. No findings of severity ERROR.

**1 — Validation Failure or Operation Failure**
The command ran but a required condition was not satisfied:

- Validation found ERROR-level findings
- A required file was missing
- A required command was not configured
- A policy required by the foundation was violated

**2 — Invalid Usage or Invalid Input**
Forge could not interpret the command:

- Unknown command or subcommand
- Missing required argument
- Invalid flag value
- Malformed `forge.yaml`
- Ambiguous foundation selection in non-interactive mode

**3 — Filesystem Failure**
Forge was unable to read or write required files:

- Permission denied
- Disk full
- Target directory already exists
- Path traversal attempted (in some contexts)

**4 — Security Violation**
Forge refused to perform an operation due to a security rule:

- Template attempts path traversal
- Template attempts to escape target directory
- Template requests hooks without opt-in
- Symlink escape attempted

**5 — Update Conflict (Phase 14+)**
An update could not be applied without overwriting developer changes:

- Conflicting modification
- Both Forge and developer changed the same region
- Manual resolution required

### 7.2 Exit Code Stability

These codes are part of Forge's public contract. Changing them requires
an ADR.

---

## 8. Machine-Readable Output

Every command that produces structured output supports
`--format json`. JSON output is governed by schema versions to enable
safe automation.

### 8.1 Schema Versioning

JSON output includes a `schemaVersion` field:

```json
{
  "schemaVersion": "1",
  "forgeVersion": "0.1.0",
  ...
}
```

Rules:

- `schemaVersion` increments on breaking changes
- Consumers must check `schemaVersion` before parsing
- Adding fields does not require a version bump
- Removing or renaming fields requires a version bump

### 8.2 Output Streams

- **stdout** — successful command output (human or JSON)
- **stderr** — errors, warnings, diagnostics

JSON output is always written to stdout. Errors during JSON-emitting
commands are also written to stdout, embedded in the JSON document:

```json
{
  "status": "error",
  "error": {
    "code": "FORGE_TEMPLATE_NOT_FOUND",
    "message": "Template 'python-django' not found",
    "suggestion": "Run 'forge template list' to see available templates"
  }
}
```

### 8.3 JSON Stability Contract

The following fields are contractually stable and will not be renamed
or removed within a schema version:

- `schemaVersion`
- `forgeVersion`
- `status`
- `error.code`
- `error.message`

All other fields are stable within a schema version but may evolve in
a new schema version.

### 8.4 Format Values

Supported `--format` values:

| Value | Meaning |
|-------|---------|
| `human` | Default human-readable output |
| `json` | Machine-readable JSON |
| `sarif` | SARIF (Phase 10+, for `forge check` only) |

Unsupported format values exit with code 2.

---

## 9. CLI UX Principles

The following principles govern every command, flag, and output.

### 9.1 Predictable Commands

Commands follow a consistent pattern:

- Verb-noun structure: `forge <verb> [<noun>]`
- Flags use `--long-form` and inherit from parent commands
- Output follows a standard vocabulary (see § 9.8)
- Exit codes follow the contract in § 7

### 9.2 Safe Defaults

Default behaviour is the safest reasonable behaviour:

- Dry-run is never required to avoid damage (real commands are safe)
- Overwrite is never the default
- Confirmation is required for destructive operations
- Non-interactive mode fails rather than guesses

### 9.3 Actionable Errors

Every error explains:

1. **What happened** — the observable failure
2. **Why** — the cause, when known
3. **What to do** — a suggested action

Example:

```text
✗ Template not found

Template: python-django

Available templates:
  python-fastapi
  go-api
  go-cli
  typescript-node
  react-app

Try:
  forge template list
```

Bad (never acceptable in Forge):

```text
error: template resolution failed
```

### 9.4 No Destructive Operations Without Confirmation

Any operation that modifies filesystem state beyond its own temporary
files must either:

- Support `--dry-run` (so the user can preview)
- Prompt for confirmation (in interactive mode)
- Require an explicit flag (in non-interactive mode)

No command silently deletes or overwrites files.

### 9.5 Scriptability

Commands are designed to work in shell scripts and CI:

- Non-interactive execution is possible for every command
- Exit codes are stable and documented
- JSON output is available
- Output is deterministic where required
- No command requires a TTY

### 9.6 Human-Readable Output

Default output is optimised for humans:

- Uses colour and symbols (`✓`, `⚠`, `✗`) where supported
- Never relies on colour to convey meaning
- Uses whitespace and structure to aid readability
- Keeps output brief in normal mode; verbose mode adds detail

### 9.7 Machine-Readable Output

JSON output is a first-class interface:

- Schema-versioned
- Stable across releases within a schema version
- Free of terminal formatting codes
- Deterministic ordering where practical

### 9.8 Output Vocabulary

Forge uses a consistent vocabulary across commands.

| Symbol | Meaning | Usage |
|--------|---------|-------|
| `✓` | PASS / success | Validation, checks |
| `⚠` | WARNING | Non-blocking issues |
| `✗` | ERROR | Blocking issues |
| `○` | SKIP / EXEMPT | Deliberately not evaluated |
| `!` | ATTENTION | Notable but not a check result |

Text formatting:

- Headings are separated by blank lines and use no trailing colons
- Field labels are aligned within a block
- Paths are wrapped in backticks when embedded in prose
- Commands are wrapped in backticks
- Symbols precede field labels where they apply

### 9.9 Progressive Disclosure

The default output is minimal. Detail is available on demand:

- `--verbose` adds diagnostic context
- `--format json` provides complete structured data
- `forge explain` provides detailed foundation information
- `forge doctor` provides environment diagnostics

### 9.10 No Hidden Magic

Forge does not:

- Modify files without showing what will change
- Change configuration based on environment inference
- Install or download anything without explicit user action
- Contact external services unless required by an explicit command
- Require network access for local commands

### 9.11 Colour Philosophy

- Colour is a visual aid, never a requirement
- Meaning is always encoded in the symbol or text
- `--no-color` disables colour
- When stdout is not a TTY, colour is disabled by default
- Colour choice respects `NO_COLOR` environment variable

### 9.12 Accessibility

- Output is readable in monochrome terminals
- Symbols are ASCII-safe alternatives when Unicode is unavailable
- Long lines wrap at 80 columns where practical
- No information is conveyed by position alone

### 9.13 Performance

- `forge version` and `forge --help` return in under 100ms
- Local commands do not perform network requests
- Interactive prompts appear within 100ms
- Generation completes in under 5 seconds for typical projects

### 9.14 Cancellation

- Ctrl+C cancels the current operation cleanly
- No partial files are left behind
- Exit code is non-zero (128 + signal)
- Interactive prompts respond to Ctrl+C by cancelling the command

---

## 10. Interactive vs Non-Interactive

Forge commands support both modes. The behaviour differs as follows:

| Aspect | Interactive | Non-interactive |
|--------|-------------|-----------------|
| Prompts | Shown | Never shown |
| Missing input | Prompted | Error (exit 2) |
| Ambiguity | Offered choice | Error (exit 2) |
| Confirmation | Prompted | Requires `--yes` |
| Output | Coloured, formatted | Plain, JSON if `--format json` |
| TTY required | Yes | No |

Non-interactive mode is triggered by:

- Explicit `--non-interactive` flag
- Absence of TTY (e.g., running in CI)
- Presence of `--format json` (implies non-interactive)

---

## 11. Error Catalogue

Every error returned by Forge has a stable code. Codes follow the
pattern:

```text
FORGE_<CATEGORY>_<SPECIFIC>
```

Categories:

| Category | Meaning |
|----------|---------|
| `USAGE` | Invalid command or flags |
| `INPUT` | Invalid user input |
| `CONFIG` | Configuration file issue |
| `FOUNDATION` | Foundation resolution issue |
| `TEMPLATE` | Template issue |
| `COMPONENT` | Component issue |
| `VALIDATION` | Validation failure |
| `FILESYSTEM` | Filesystem failure |
| `SECURITY` | Security violation |
| `NETWORK` | Network failure (future) |
| `INTERNAL` | Internal error |

Example codes:

- `FORGE_USAGE_UNKNOWN_COMMAND`
- `FORGE_INPUT_MISSING_ARGUMENT`
- `FORGE_CONFIG_NOT_FOUND`
- `FORGE_CONFIG_INVALID`
- `FORGE_FOUNDATION_NOT_FOUND`
- `FORGE_TEMPLATE_NOT_FOUND`
- `FORGE_TEMPLATE_INCOMPATIBLE`
- `FORGE_VALIDATION_FAILED`
- `FORGE_FILESYSTEM_PERMISSION_DENIED`
- `FORGE_FILESYSTEM_PATH_EXISTS`
- `FORGE_SECURITY_PATH_TRAVERSAL`
- `FORGE_SECURITY_UNSAFE_TEMPLATE`

Each code maps to an exit code (§ 7) and is included in JSON output
(§ 8).

---

## 12. Open Questions

The following questions remain open and should be resolved before
implementation:

- Should `forge validate` and `forge check` be merged, or remain
  separate commands?
- Should `forge explain` support subcommands (e.g., `forge explain
  policy testing`) in the MVP, or defer to a later phase?
- Should `forge new` write the `forge.yaml` file before or after
  generating the project files?
- Should the default output for successful operations include a
  `Next steps` block, or should that be deferred to `--verbose`?
- Should `forge version` include template versions in its output?
- What is the exact behaviour when `forge new` is given a project name
  with a `/` in it (nested path)?
- Should `forge init` detect and refuse to run if the repository has
  any uncommitted Git changes?
- Should `forge template list` distinguish between local and remote
  templates once a registry exists?

These questions will be addressed in Phase 2 as implementation begins
and the interaction details become concrete.

---

## 13. Status

**Draft.**

This specification is under active development during Phase 1. It
becomes **Approved** when:

- All MVP command behaviours are documented
- All open questions have been resolved or explicitly deferred
- The exit code contract is reviewed
- The JSON schema for each command is specified
- The specification has been reviewed for consistency with
  [`docs/product-discovery.md`](./product-discovery.md) § 9 and
  [`docs/mvp-scope.md`](./mvp-scope.md)
