# CLI UX Specification

- **Document type:** Specification
- **Status:** Draft
- **Version:** 0.10.0
- **Author:** @thapelomagqazana
- **Created:** 2026-10-09
- **Last Updated:** 2026-10-12
- **Supersedes:** 0.9.0
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
- Root command identity strings
- Version output contract
- Help behaviour contract
- Help invocation matrix
- Root help content contract
- Per-command help content contract
- Command metadata contract
- Examples convention
- Invalid-command contract
- Error message format
- Output stream boundary
- Colour, TTY, and ASCII policy
- Global flag semantics and precedence
- Global flag interaction matrix
- CLI configuration scope
- CLI configuration schema
- Configuration field naming
- Configuration defaults
- Configuration precedence
- Configuration file discovery
- Configuration file loading
- Environment variable resolution
- Configuration source attribution
- `forge config` text and JSON output formats
- `forge config --path` mode

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
├── config         (MVP, Phase 8)
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
| `forge config` | MVP | Phase 8 | Inspect and manage CLI configuration |
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
Forge - New Project
--------------------

Project name
> payments-api

What are you building?
> API
  CLI
  Web application
  Library
  Worker
  Service

Language
> Python
  Go
  TypeScript
  Rust

Framework
> FastAPI
  Flask
  None

Testing
> Pytest
  None

Containerisation
> Docker
  None

CI
> GitHub Actions
  None

Foundation preview
  Project: payments-api
  Template: python-fastapi@1.0.0
  Files to create: 12
  Destination: ./payments-api

Create project? [Y/n]
```

The prompts use ASCII characters only (see § 4.10j). The interactive
selection marker is `>`; the checked bullet is `[x]`; the unchecked
bullet is `[ ]`. Colour and Unicode glyphs are deferred to Phase 6.

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
[OK] Project created

payments-api/
Template: python-fastapi@1.0.0
Files: 12

Next steps:
  cd payments-api
  forge validate
```

The `[OK]` marker is an ASCII-safe replacement for the checkmark
glyph. See § 4.10j for the ASCII-only policy.

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
Forge - Repository Adoption
----------------------------

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
  [x] pyproject.toml detected
  [x] FastAPI dependency detected
  [x] pytest configuration detected
  [x] Dockerfile detected
  [x] GitHub Actions workflow detected

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
[ERROR] Could not detect a foundation

Detected technologies: none

Forge requires a foundation to adopt this repository.

Options:
  forge init --foundation <id>
  forge template list
```

**Failure behaviour:**

- Existing `forge.yaml` -> fail with actionable message (do not
  overwrite)
- Ambiguous detection (multiple equally plausible foundations) -> fail
  in non-interactive mode; prompt in interactive mode
- No `forge.yaml` can be written -> filesystem error

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
[OK] Repository initialized

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
[OK] forge.yaml is valid

Blueprint: payments-api
Foundation: python-fastapi@1.0.0
Template: python-fastapi@1.0.0

Checks
  [OK]  Configuration
  [OK]  Foundation reference
  [OK]  Structure
  [OK]  Testing
  [OK]  CI
  [OK]  Documentation
  [WARN] Security - .env is not ignored

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
------------------

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

### 4.6 `forge config`

Displays Forge's resolved CLI configuration. The command's scope is
documented in § 4.13.1; its schema is documented in § 4.13.2; its
source-attribution semantics are documented in § 4.13.10; its
output format is documented below.

**Syntax:**

```text
forge config [flags]
```

**Arguments:**

None. Subcommands are reserved for a future phase.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--path` | bool | `false` | Print only the resolved config file path |
| `--format <format>` | enum | `text` | `text` or `json` (global flag; see § 4.12) |

The `--format` flag is global (§ 4.12). The command inherits it and
dispatches on its value. The `--path` flag is command-local.

**Behaviour:**

With no subcommand, `forge config` displays the resolved
configuration for the current invocation. The resolution applies the
four precedence layers in order (see § 4.13.5) and records the source
of each value.

The command has two output modes, selected by `--format`:

- **text** (default) — the human-readable format documented in § 4.6.1.
- **json** — the machine-readable format documented in § 4.6.2.

The `--path` flag suppresses the full output and prints only the
resolved config file path (§ 4.6.3).

The command's interaction with the global flags is frozen in § 4.6.4.

#### 4.6.1 Text output (frozen by WBS 9.2.1)

The text format is the default. It is produced by `printConfig` in
`internal/cli/config.go`.

**The format:**

```text
Forge CLI Configuration
───────────────────────

  Source:       /home/user/.config/forge/config.yaml
  Log level:    info        (source: default)
  Output:       text        (source: default)
```

**Byte-level rules:**

| # | Rule | Rationale |
|---|------|-----------|
| F1 | Line 1: `Forge CLI Configuration` (title) | Identifies output |
| F2 | Line 2: separator of U+2500 repeated to title width | Visual clarity |
| F3 | Line 3: blank | Whitespace |
| F4 | Every value line: two spaces, label padded to 14 chars, value | Alignment |
| F5 | Source attribution in parentheses: `(source: <source>)` | Debuggability |
| F6 | Source values: `default`, `file`, `env`, `flag` | One of four |
| F7 | `Source:` line shows `<path>` or `<none>` when no file found | Explicit |
| F8 | No trailing whitespace on any line | git cleanliness |
| F9 | No colour, no ANSI escapes | Scripting |
| F10 | Every line ends with `\n` including the last | POSIX |

The value column is 6 characters wide (the `configValueColumn`
constant in `internal/cli/config.go`). Values shorter than the column
are padded; longer values push the attribution rightward. The column
is a rendering detail pinned by the golden files.

The `Source:` line has no attribution: the path *is* the source.

**Regeneration:** the text format is pinned by golden files under
`internal/cli/testdata/config/`. Regenerate with
`go test ./internal/cli -run TestConfigGolden -update` after
confirming the change is intentional.

#### 4.6.2 JSON output (frozen by WBS 9.4.1)

The JSON format is selected by `--format json`. It is produced by
`writeConfigJSON` in `internal/cli/config.go`.

**The schema:**

```json
{
  "config_path": "/home/user/.config/forge/config.yaml",
  "log_level": "info",
  "log_level_source": "default",
  "output_format": "text",
  "output_format_source": "default"
}
```

**Schema properties:**

| Field | Type | Notes |
|-------|------|-------|
| `config_path` | string or null | `null` when no file was resolved. |
| `log_level` | string | One of `debug`, `info`, `warn`, `error`. |
| `log_level_source` | string | One of `default`, `file`, `env`, `flag`. |
| `output_format` | string | One of `text`, `json`. |
| `output_format_source` | string | One of `default`, `file`, `env`, `flag`. |

**Contract properties:**

- Field names use snake_case, per § 4.13.3.
- Source fields use the `<field>_source` suffix.
- Keys are emitted in alphabetical order.
- Output is a single line terminated by `\n`.
- `config_path` is `null` when no file was resolved, not the empty
  string.
- No ANSI escapes.
- The output contains no `schemaVersion` field. The schema is
  identified by its field names. See § 8.1.

**The published schema** is `schemas/config.schema.json`. The runtime
contract is the `configJSON` struct in `internal/cli/config.go`; the
schema file is its published counterpart. The two are kept in sync by
`TestJSONSchema_MatchesStruct`.

#### 4.6.3 `--path` mode (frozen by WBS 9.3.1)

When `--path` is set, the command prints only the resolved config file
path. The full output is suppressed.

**The frozen table:**

| Invocation | Behaviour | Exit | stdout |
|------------|-----------|------|--------|
| `forge config --path` (file resolved) | Print the path | 0 | `<path>\n` |
| `forge config --path` (no file) | Print `<none>` | 0 | `<none>\n` |
| `forge config --path --config <existing>` | Print the path | 0 | `<path>\n` |
| `forge config --path --config <missing>` | Print an error | 3 | (empty) |
| `forge config --path --format json` (file resolved) | Print JSON | 0 | `{"config_path":"<path>"}\n` |
| `forge config --path --format json` (no file) | Print JSON | 0 | `{"config_path":null}\n` |

**Rules:**

- `--path` is mutually exclusive with the default full output.
- Output is a single line ending in `\n`.
- No label, no prefix, no trailing punctuation beyond the newline.
- A missing explicit `--config` is an error (exit 3), produced by the
  loader; the `--path` handler is never reached.
- A missing discovered config is `<none>`, not an error.

**Regeneration:** `--path` output is pinned by golden files under
`internal/cli/testdata/config/`. Regenerate with
`go test ./internal/cli -run 'TestConfigGolden' -update`.

#### 4.6.4 Interaction with the global flags (frozen by WBS 9.4.2)

The command composes with the global flags as follows.

| Invocation | Behaviour |
|------------|-----------|
| `forge config` | Print full config with sources. |
| `forge config --config <path>` | Load `<path>`; the `Source:` line (text) / `config_path` (JSON) reflects the resolved path. |
| `forge config --quiet` | Print full config. `--quiet` sets the log level; the command's output is not suppressed. |
| `forge config --verbose` | Print full config. In Phase 2, no debug output is emitted. |
| `forge config --quiet --verbose` | Print full config. `--quiet` wins for the log level. |
| `forge config --config <missing>` | Error; exit 3. The loader produces the error before the command runs. |
| `forge config --path --quiet` | Print path. The path is the result; it is not suppressed. |
| `forge config --path --verbose` | Print path. In Phase 2, no debug output is emitted. |

**Rules:**

- `--quiet` never suppresses `forge config` output.
- `--verbose` sets the log level to debug. In Phase 2, no command
  emits debug output; WBS 12.0 will add the discovery debug output.
  Debug output, when it lands, goes to stderr, never to stdout.
- `--config <path>` determines the file the loader reads. The resolved
  path appears in the `Source:` line (text) and in `config_path`
  (JSON).
- A missing explicit `--config` is a fatal error. The loader produces
  it; the CLI maps it to `ExitConfig` (3). Nothing is written to
  stdout.
- `--path` composes with `--quiet` and `--verbose` without conflict.

#### 4.6.5 Exit codes

| Condition | Code |
|-----------|------|
| Configuration displayed | 0 |
| Invalid usage/input | 2 |
| Configuration file unreadable or invalid | 3 |

The command does not modify the configuration; write operations are
reserved for a future phase.

### 4.7 `forge version`

Displays version information. The command supports two output formats:
`text` (the default) and `json`. The text format is frozen by WBS 6.4.1
and documented in § 4.8.1. The JSON format is frozen by WBS 6.4.2 and
documented in § 4.8.2.

**Syntax:**

```text
forge version [flags]
```

**Arguments:**

None. Extra arguments are rejected with exit code `ExitUsage` (2).

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format <format>` | enum | `text` | `text` or `json` |

`--format text` and `--format=text` are equivalent. `--format json` and
`--format=json` are equivalent. Any other value is rejected with exit
code `ExitUsage` (2) and a diagnostic on stderr that names the
offending value.

**Output (text, default):**

```text
forge 0.1.0
  commit:     a1b2c3d
  built:      2026-10-09T08:00:00Z
  dirty:      false
  go version: go1.23.0
  platform:   linux/amd64
```

**Output (json, `--format json`):**

```json
{"version":"0.1.0","commit":"a1b2c3d","build_date":"2026-10-09T08:00:00Z","dirty":false}
```

The JSON output is a single line terminated by `\n`. It contains no
pretty-printing.

### 4.8 Root Command Identity

The root command's identity strings are frozen. They are defined as
package-level constants in
[`internal/cli/root.go`](../internal/cli/root.go) and consumed by
tests, documentation, and help output.

#### The four constants

| Constant | Value |
|----------|-------|
| `RootName` | `forge` |
| `RootUsage` | `forge [command]` |
| `RootShortDesc` | `Forge - Engineering Foundations as Code` |
| `RootLongDesc` | see below |

`RootLongDesc` is a raw string literal:

```text
Forge is a cross-platform CLI for defining, generating,
validating, and evolving software project foundations as code.

The core loop:

    CREATE  ->  forge new
    VERIFY  ->  forge check
    EXPLAIN ->  forge explain
    EVOLVE  ->  forge update

Run 'forge <command> --help' for details on any command.
```

#### Where each string appears

| String | Where |
|--------|-------|
| `RootName` | The command name in `forge --help` and in the composed `Use` field. |
| `RootUsage` | The "Usage:" line in `forge --help`. |
| `RootShortDesc` | The one-line description shown in a parent's "Available Commands:" table. For the root command, this field is **not** rendered by `forge --help` (the root has no parent). It is consumed by any future tool that nests `forge` as a subcommand. |
| `RootLongDesc` | The body of `forge --help`, above the "Usage:" section. |

Cobra's help rendering uses the `Long` field as the body of the help
text and the `Short` field only in a parent's command list. Because
the root command has no parent, its `Short` field is not rendered by
`forge --help`. This is documented behaviour and is pinned by the test
`TestRootIdentity_HelpOutputContainsConstants`.

#### Rules

1. `RootName` is lowercase. It is never "Forge" in the CLI's name
   field. The command a user types is `forge`, not `Forge`.
2. `RootShortDesc` is a single line of 80 characters or fewer. The
   limit is measured in runes, not bytes.
3. `RootLongDesc` is a raw string literal. Its line breaks and
   indentation are preserved verbatim in the help output.
4. `RootLongDesc` contains the CREATE / VERIFY / EXPLAIN / EVOLVE
   loop. The four keywords are the product's core loop; removing any
   of them is a product decision, not a copy edit.
5. `RootLongDesc` ends with a pointer to `<command> --help`. The
   pointer tells a user how to learn more about a specific subcommand.
6. None of the four contains emojis, ANSI colour codes, or tabs.
7. All four are ASCII-only. The em dash and right arrow used in an
   earlier draft were replaced with the ASCII equivalents `-` and `->`
   when the ASCII-only policy (WBS 7.4.2) was frozen. See § 4.10j
   "Colour and Terminal Policy" for the policy.

#### Changing a string

Changing any of the four constants requires:

1. Updating the constant in `internal/cli/root.go`.
2. Updating the corresponding assertion in `internal/cli/root_test.go`.
3. Updating this section of `docs/cli-ux-spec.md` to quote the new
   value.
4. Regenerating the golden files under `internal/cli/testdata/help/`
   that contain the rendered value.
5. Running `task check` and confirming the full suite passes.

The updates are in the same commit. A reviewer who sees a constant
change without the corresponding test and documentation changes
rejects the PR.

#### Why these strings are frozen

The root command's identity appears in every invocation of the CLI, in
every documentation page that describes the CLI, and in every shell
script that captures `forge --help` output. Changing the strings is
cheap in code and expensive in every artefact that quotes them.
Freezing the strings is the mechanism by which the cost is bounded: a
change requires a deliberate update to several files, which a reviewer
sees.

### 4.9 Version Output Contract

The `--version` flag and the `version` subcommand produce output
derived from a single formatter in `internal/app/version`. The
`version` subcommand supports two output formats: `text` (the default)
and `json`. The `--version` flag always produces the text format,
because a flag cannot take a format argument.

#### 4.9.1 Text Format (frozen by WBS 6.4.1)

The text format is the default. It is produced by `Format(w io.Writer,
info Info) error` in
[`internal/app/version/format.go`](../internal/app/version/format.go).

**The format:**

```text
forge <version>
  commit:     <commit>
  built:      <build-date>
  dirty:      <dirty>
  go version: <go-version>
  platform:   <os>/<arch>
```

**Contract properties:**

- The output is exactly six lines.
- The header line is `forge` when `<version>` is empty and
  `forge <version>` otherwise. There is no trailing space on the
  header in either case.
- Every detail line has two spaces of indent, the key followed by a
  colon, padding to a field width of twelve characters, a single
  space, and the value.
- The value column is fixed at position 15 (1-indexed) on every  detail line.
- The output ends with a trailing newline.
- The output contains no ANSI escape sequences, regardless of whether
  stdout is a TTY.

**Field values:**

| Field | Source | Example |
|-------|--------|---------|
| `<version>` | `internal/version.Version` (injected via `-X` at build time) | `0.1.0` |
| `<commit>` | `internal/version.Commit` (injected via `-X`) | `a1b2c3d` |
| `<build-date>` | `internal/version.BuildDate` (injected via `-X`) | `2026-10-09T08:00:00Z` |
| `<dirty>` | `internal/version.Dirty` (injected via `-X`) | `false` |
| `<go-version>` | `runtime.Version()` | `go1.23.4` |
| `<os>/<arch>` | `runtime.GOOS + "/" + runtime.GOARCH` | `linux/amd64` |

**The `dirty` rule:** Only the literal `"true"` renders as `true`. Every
other value - including `"1"`, `"yes"`, `"TRUE"`, and the empty string

- renders as `false`. The rule is documented on
`internal/version.Dirty` (WBS 6.1.1) and enforced by the linker
injection contract (WBS 6.1.2).

**Empty values:** When the binary is built without `-X` flags, the
first four values are empty. The output then reads, for example:

```text
forge
  commit:
  built:
  dirty:      false
  go version: go1.23.4
  platform:   linux/amd64
```

The `commit:` and `built:` lines have trailing spaces from the
field-width padding; the values themselves are empty. The `dirty:`
line renders `false` because the formatter parses the empty string
strictly. The empty values are not a defect; they signal that the
binary carries no build metadata.

**Why the format is frozen:** Scripts that parse the output depend on
its shape. The format is a contract with those scripts, in the same
way that the exit codes are a contract with the shells that branch on
them. Freezing the format bounds the cost of a change: a change
requires updating the formatter, the tests, and this document in one
commit, which a reviewer sees.

#### 4.9.2 JSON Format (frozen by WBS 6.4.2)

The JSON format is selected by `--format json`. It is produced by
`WriteJSON(w io.Writer, info Info) error` in
[`internal/app/version/format_json.go`](../internal/app/version/format_json.go).

**The schema:**

```json
{
  "version":    "0.1.0",
  "commit":     "a1b2c3d",
  "build_date": "2026-10-09T08:00:00Z",
  "dirty":      false
}
```

**Schema properties:**

| Field | Type | Notes |
|-------|------|-------|
| `version` | string | Semantic version, or empty for an uninstrumented build. |
| `commit` | string | Short git SHA, or empty. |
| `build_date` | string | RFC 3339 UTC timestamp, or empty. |
| `dirty` | boolean | `true` only if the build was from a tree with uncommitted changes; `false` otherwise (including for empty values). |

**Contract properties:**

- Field names are snake_case.
- `dirty` is a JSON boolean, not a string.
- `build_date` is an RFC 3339 UTC string, not a Unix timestamp.
- Output is a single line terminated by `\n`. No pretty-printing.
- All four keys are always present, even when their values are empty.
- Field order is `version`, `commit`, `build_date`, `dirty`.
- The output contains no ANSI escape sequences.

**Example output:** The line below is the exact output for a populated
build:

```json
{"version":"0.1.0","commit":"a1b2c3d","build_date":"2026-10-09T08:00:00Z","dirty":false}
```

**Uninstrumented example:** The line below is the exact output for a
build without `-X` flags:

```json
{"version":"","commit":"","build_date":"","dirty":false}
```

**Schema versioning:** The JSON schema is versioned by its field names.
A breaking change to any field name requires an ADR. Adding a new field
is additive and requires a new WBS item but not an ADR.

**Why the schema is frozen:** Machine consumers parse the output. The
schema is a contract with those consumers, in the same way the text
format is a contract with human-facing scripts. Freezing the schema
bounds the cost of a change: a change requires updating the encoder,
the tests, the JSON Schema document, and this section in one commit,
which a reviewer sees.

#### 4.9.3 Where the format is implemented

The text format is implemented in
[`internal/app/version/format.go`](../internal/app/version/format.go), in
the function `Format`. The function writes to an `io.Writer`. The
function `Raw` in
[`internal/app/version/service.go`](../internal/app/version/service.go)
returns the same string as a `string`, for callers that need the value
rather than a stream.

The JSON format is implemented in
[`internal/app/version/format_json.go`](../internal/app/version/format_json.go),
in the function `WriteJSON`. The function is symmetric with the text
formatter: it takes an `io.Writer` and the same `Info` value, and it
writes the JSON object followed by a newline.

The dispatcher is `FormatAs(w io.Writer, info Info, format string)`,
also in `format_json.go`. It routes to `WriteText` for `FormatText` and
to `WriteJSON` for `FormatJSON`, and returns an error wrapping
`ErrUnknownFormat` for any other value.

The three CLI invocations consume the formatters:

- `forge version` (the subcommand, default format) calls
  `FormatAs(deps.Stdout, version.Get(), FormatText)`, which routes to
  the text formatter.
- `forge version --format json` calls
  `FormatAs(deps.Stdout, version.Get(), FormatJSON)`, which routes to
  the JSON formatter.
- `forge --version` (the flag) reads `version.Raw()` in `newRootCmd`
  and assigns the result to `root.Version`. Cobra renders the flag's
  output using a version template that prints the value verbatim:

  ```go
  root.SetVersionTemplate("{{.Version}}")
  ```

  Cobra's default template prepends `"forge version "` to the value
  and appends its own trailing newline. Forge overrides the default so
  that the flag's output is exactly the value produced by
  `version.Raw`, with no prefix and no extra newline.

Because the `--version` flag and the default `forge version` both
derive from the same text formatter, they cannot diverge without a
change to the formatter. The JSON formatter is used only by
`forge version --format json`.

#### 4.9.4 Changing a format

Changing either format requires:

1. Updating the corresponding formatter in `internal/app/version/`
   (`format.go` for text, `format_json.go` for JSON).
2. Updating the golden files under
   `internal/app/version/testdata/format/`.
3. Updating the byte-for-byte test for the format
   (`TestFormat_FullOutput` for text, `TestWriteJSON_FullOutput` for
   JSON).
4. Updating the corresponding subsection of § 4.9 above.
5. For a JSON change that renames or removes a field, opening an ADR.
   Adding a field is additive and does not require an ADR.
6. Running `task check` and confirming the full suite passes.

The updates are in the same commit. A reviewer who sees a change to a
formatter without the corresponding documentation update rejects the
PR.

#### 4.9.5 Why the format is frozen

Scripts that parse `forge --version`, `forge version`, or
`forge version --format json` depend on the format. The format is a
contract with those scripts, in the same way that the exit codes are a
contract with the shells that branch on them. Freezing the format is
the mechanism by which the cost of a change is bounded: a change
requires the updates listed in § 4.9.4, and a reviewer sees all of them
in one diff.

### 4.10 Help and Error Contracts

This subsection collects the frozen contracts that govern help output,
per-command help, invalid-command behaviour, error messages, output
streams, colour and ASCII policy, and global flag interaction. Each
contract has a subsection below, identified by a letter suffix.

#### 4.10a Help Behaviour

Forge's help system has eight entry points. The table below is the
contract: every release must handle every row consistently.

##### The contract

| Invocation | Behaviour | Exit | Stream |
|------------|-----------|------|--------|
| `forge` | Print root help | 0 | stdout |
| `forge --help` | Print root help | 0 | stdout |
| `forge -h` | Print root help | 0 | stdout |
| `forge help` | Print root help | 0 | stdout |
| `forge help version` | Print version command help | 0 | stdout |
| `forge version --help` | Print version command help | 0 | stdout |
| `forge --help version` | Rejected (help has no args) | 2 | stderr |
| `forge help unknown` | Print "unknown help topic" | 2 | stderr |

##### Rules

1. **All help goes to stdout, never stderr.** The help text is a
   successful result, not a diagnostic.

2. **All help exits 0.** The user's request was fulfilled.

3. **Errors about unknown help topics go to stderr and exit 2.**
   `forge help unknown` is a usage error: the user asked for help on a
   topic that does not exist.

4. **`--help` and `-h` are equivalent.** Both produce the same output
   and the same exit code.

5. **`forge help <cmd>` and `forge <cmd> --help` are equivalent.** The
   two forms produce the same output and the same exit code.

6. **`forge --help <cmd>` is rejected.** `--help` is a flag that takes
   no argument. Passing a positional argument after it is a usage
   error.

7. **The help output contains the frozen identity strings.** The
   output includes `RootName`, the composed usage line built from
   `RootName` and `RootUsage`, and the body of the help text
   (`RootLongDesc`). `RootShortDesc` is not rendered by `forge --help`
   for the root command; see § 4.8.

8. **The help output lists every visible registered command.** Every
   visible constructor in `internal/cli/registry.go` produces a
   command that appears in the "Available Commands:" section. Hidden
   commands do not appear; they are documented separately.

##### Where the behaviour is implemented

Six of the eight invocations are handled by Cobra's defaults. The two
rejections are handled by `validateArgs` (validate.go), which runs in
`executeWithOptions` before Cobra parses the arguments. The rejection
cannot live in a Cobra hook: Cobra's `--help` interception
short-circuits the hook chain, so a `PersistentPreRunE` or `RunE` never
sees the malformed invocation.

##### Why the contract is frozen

Every CLI user encounters the help system. Every script that captures
help output for documentation or for error reporting depends on help
going to stdout and the exit code being 0. Freezing the contract is the
mechanism by which the cost of a change is bounded.

#### 4.10b Help Invocation Matrix

WBS 7.1.1 defines the complete inventory of every way to reach help.
The matrix is the contract; each row has a test in
`internal/cli/help_matrix_test.go`.

##### The matrix

| # | Invocation | Resolves to | Exit | Stream | Notes |
|---|------------|-------------|------|--------|-------|
| H1 | `forge` | Root help | 0 | stdout | No-args behaviour (WBS 5.2.1) |
| H2 | `forge --help` | Root help | 0 | stdout | Long-form flag |
| H3 | `forge -h` | Root help | 0 | stdout | Short-form flag |
| H4 | `forge help` | Root help | 0 | stdout | Help as subcommand |
| H5 | `forge help version` | Version help | 0 | stdout | Nested help |
| H6 | `forge help config` | Config help | 0 | stdout | Nested help |
| H7 | `forge version --help` | Version help | 0 | stdout | Identical to H5 |
| H8 | `forge version -h` | Version help | 0 | stdout | Identical to H5 |
| H9 | `forge config --help` | Config help | 0 | stdout | Identical to H6 |
| H10 | `forge help help` | Help command's help | 0 | stdout | Self-referential |
| H11 | `forge help unknown` | Error: unknown help topic | 2 | stderr | Negative case |
| H12 | `forge --help version` | Error: help takes no args | 2 | stderr | Negative case |
| H13 | `forge -h --version` | Root help (help wins) | 0 | stdout | Flag precedence |
| H14 | `forge help --help` | Help for help | 0 | stdout | Reserved |

##### Rules

1. Every positive help path writes only to stdout.
2. Every positive help path exits 0.
3. Every error path writes only to stderr.
4. Every error path exits 2 (`ExitUsage`).
5. H5 == H7 == H8 (nested help equivalence) - byte-identical output.
6. H6 == H9 (config help equivalence).
7. `--help` beats `--version` when both are supplied (H13).
8. Help output contains the frozen root identity strings from WBS 5.1.1.

##### Adding a new help path

Adding a new help path requires adding a row to the matrix, adding a
case to `internal/cli/help_matrix_test.go`, and (if the row is a new
equivalence class) adding a dedicated test. The three updates are in
the same commit.

#### 4.10c Root Help Output Contract

WBS 7.1.2 freezes the exact structure and content of the root command's
help output. The output is compared byte-for-byte against
`internal/cli/testdata/help/root.golden.txt`.

##### The frozen structure

```text
<RootLongDesc>

Usage:
  forge [command] [flags]
  forge [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  config      Display resolved CLI configuration
  help        Help about any command
  version     Print Forge version information

Flags:
      --config string   path to the configuration file (overrides discovery)
      --format string   output format: text|json (default "text")
  -h, --help            help for forge
      --quiet           suppress all output except errors (log level ERROR)
      --verbose         enable verbose output (log level DEBUG)
  -v, --version         version for forge

Use "forge [command] --help" for more information about a command.
```

##### Contract properties

1. **No blank line at the start of the output.** The first line is the
   first line of `RootLongDesc`.

2. **One blank line between `RootLongDesc` and `Usage:`.**

3. **Commands are listed alphabetically by the `Use` field.**

4. **Command short descriptions are aligned with padding to the longest
   command name plus two spaces.**

5. **Flags are listed with their descriptions aligned similarly.**

6. **The trailing `Use ...` line is mandatory.**

7. **The output contains no ANSI escape sequences.**

8. **The output ends with a trailing newline.**

9. **The output is identical on all supported platforms.**

##### Where the behaviour is implemented

The root help output is rendered by Cobra from the root command's
`Use`, `Short`, and `Long` fields, plus the subcommand list (from the
registry) and the global flags (from `flags.go`). The golden file
`internal/cli/testdata/help/root.golden.txt` is the record.

#### 4.10d Per-Command Help Content Contract

WBS 7.1.3 freezes the structure and content of every subcommand's help
output. The contract is enforced structurally (by tests that read each
command's fields) and by golden files (one per command).

##### The frozen structure

```text
<LongDesc>

Usage:
  forge <command> [flags]

Examples:
  <example-1>
  <example-2>

Flags:
  <flags-specific-to-this-command>

Global Flags:
      --config string   path to the configuration file (overrides discovery)
      --format string   output format: text|json (default "text")
      --quiet           suppress all output except errors (log level ERROR)
      --verbose         enable verbose output (log level DEBUG)

Use "forge <command> --help" for more information about a command.
```

The `Aliases:`, `Examples:`, `Available Commands:`, and `Use ...`
blocks are conditional.

##### Contract rules

1. **`Short` is a one-liner, 60 runes or fewer.**
2. **`Long` is optional.** If present, it is 500 runes or fewer and
   ends with a period.
3. **`Use` is authored.**
4. **`Args` is set to a validator.**
5. **`Example` is present when required.**
6. **Global flags appear in every command's help output.**
7. **No colour, no ANSI escapes.**
8. **A command's `Flags:` block lists its local flags; a command's
   `Global Flags:` block lists the persistent flags inherited from the
   root.** The config command's own help therefore lists `--path`
   under `Flags:` and the four global flags (`--config`, `--format`,
   `--quiet`, `--verbose`) under `Global Flags:`. The two blocks are
   rendered by Cobra and are pinned by the command's help golden file.

#### 4.10e Examples Convention

WBS 7.2.2 defines when a command must declare an Example and what the
Example's content must look like.

##### When Examples are required

A command requires an Example if any of the following holds:

- It takes positional arguments.
- It has command-specific flags.
- It has non-obvious behaviour.

##### The format

1. Each line starts with two spaces.
2. Each line contains exactly one command.
3. No line starts with the `$` prefix.
4. Values are realistic.
5. The primary use case comes first.
6. Between 2 and 5 lines.

##### Example

For a hypothetical `forge new`:

```text
Examples:
  forge new payments-api --template python-fastapi
  forge new payments-api --template python-fastapi --dry-run
```

##### Where Examples are stored

Examples are stored in the `Example` field of the Cobra command, not in
the `Long` field.

#### 4.10f Invalid Command Behaviour

WBS 7.3.1 freezes the behaviour of seven invalid invocations.

WBS 10.5.1 refines the error's shape: the CLI wraps Cobra's parser
and dispatcher errors as `*forgeerr.Error` values, so the error's
rendered form is the frozen format (§ 4.10g), not a flat message.

##### The contract

| Aspect | Frozen behaviour |
|--------|------------------|
| Exit code | `ExitUsage` (2) |
| Output stream | stderr (only) |
| stdout | Empty |
| Error format | the frozen format (§ 4.10g) |
| Trailing newline | Yes (exactly one) |
| Colour | None |
| Timestamp | None |
| Stack trace | Never (Phase 2) |

##### The cases

The substrings below are what the *rendered* output contains, per the
frozen format. The code is the stable identifier; the message and the
context data are the fields the CLI fills in.

| # | Input | Exit | Rendered output contains |
|---|-------|------|--------------------------|
| I1 | `forge foobar` | 2 | `Error: unknown command`, `Code:  FORGE_CLI_UNKNOWN_COMMAND`, `command: foobar` |
| I2 | `forge verison` | 2 | `Error: unknown command`, `Code:  FORGE_CLI_UNKNOWN_COMMAND`, `command: verison`, `Suggestion:`, `Did you mean`, `version` |
| I3 | `forge version extra` | 2 | `Error: unknown command`, `Code:  FORGE_CLI_UNKNOWN_COMMAND`, `command: extra` |
| I4 | `forge --unknown-flag` | 2 | `Error: unknown flag`, `Code:  FORGE_CLI_UNKNOWN_FLAG`, `flag: --unknown-flag` |
| I5 | `forge version --unknown-flag` | 2 | `Error: unknown flag`, `Code:  FORGE_CLI_UNKNOWN_FLAG`, `flag: --unknown-flag` |
| I6 | `forge help unknown` | 2 | `Error: unknown help topic`, `Code:  FORGE_CLI_UNKNOWN_COMMAND`, `topic: unknown` |
| I7 | `forge --help unknown` | 2 | `Error: the --help flag takes no argument`, `Code:  FORGE_CLI_INVALID_ARGUMENT`, `argument: unknown` |

##### Error wrapping

Cobra's parser and dispatcher produce plain `errors.errorString`
values. The CLI's `wrapCobraError` helper (in `internal/cli/execute.go`)
recognizes the three message shapes (unknown flag, unknown shorthand
flag, unknown command) and constructs the corresponding
`*forgeerr.Error`. The recognition is textual because Cobra does not
export the error types.

The wrapped error's message is a fixed string; the varying data (the
flag name, the command name, the parent command) is in the error's
context map. The user-facing rendering (§ 4.10g) therefore shows the
data as context lines, not as part of the message.

##### Suggestions

Cobra's suggestion mechanism fires when an unknown command's name is
within edit distance 2 of a known command. The value is configured by
`SuggestionsMinimumDistance` on the root command. The suggestion is
carried as the `*forgeerr.Error`'s remediation and rendered as the
`Suggestion:` block. Rule F6 of the frozen format (remediation ends
with a period) governs the rendered text: `Did you mean "version".`

#### 4.10g Error Message Format

WBS 10.4.1 freezes the user-facing error format. The format is
implemented by `forgeerr.Format` (in
`internal/forgeerr/format.go`) and is enforced byte-for-byte by the
golden files under `internal/forgeerr/testdata/format/`. The
format is the same for every error the CLI prints to stderr.

##### The frozen format

```text
Error: <message>
Code:  <code>
       <context-key-1>: <value-1>
       <context-key-2>: <value-2>

Suggestion:
  <remediation>
```

##### Rules

| # | Rule |
|---|---|
| F1 | First line: `Error:` + message. |
| F2 | Second line: `Code:` + code (two spaces after the colon). |
| F3 | Context lines: 7 spaces + `<key>: <value>`. |
| F4 | Blank line before the Suggestion block. |
| F5 | `Suggestion:` label on its own line. |
| F6 | Suggestion content: 2-space indent, ends with a period. |
| F7 | No trailing whitespace on any line. |
| F8 | No colour, no ANSI escapes. |
| F9 | Every line ends with `\n`. |
| F10 | Output ends with exactly one `\n`. |
| F11 | Context keys are sorted alphabetically. |
| F12 | The Code prefix identifies the category. |

##### Variations

| Case | Lines |
|---|---|
| No context, no remediation | F1, F2 |
| Context, no remediation | F1, F2, F3 (per key) |
| Remediation, no context | F1, F2, blank, F5, F6 |
| Both | F1, F2, F3 (per key), blank, F5, F6 |

##### Examples

**Minimal (no context, no remediation):**

```text
Error: unknown command
Code:  FORGE_CLI_UNKNOWN_COMMAND
```

**With context:**

```text
Error: invalid YAML
Code:  FORGE_CONFIG_MALFORMED
       line: 12
       path: config.yaml
```

**With remediation:**

```text
Error: unknown command
Code:  FORGE_CLI_UNKNOWN_COMMAND
       command: verison

Suggestion:
  Did you mean "version".
```

**Full (context and remediation):**

```text
Error: invalid YAML
Code:  FORGE_CONFIG_MALFORMED
       line: 12
       path: config.yaml

Suggestion:
  Fix the YAML syntax at line 12.
```

##### Relationship to § 4.10f

§ 4.10f (Invalid Command Behaviour) describes the CLI's behaviour
for seven invalid invocations: which exit code they produce, which
stream they write to, and which substrings appear in the output.
§ 4.10g describes the byte-exact format that every such output
follows. The two contracts are complementary: § 4.10f pins the
behaviour, § 4.10g pins the rendering.

##### Relationship to § 4.10h

§ 4.10h (Output Stream Boundary) pins which stream each category
of output goes to. Errors go to stderr; the format in this
subsection applies only to the errors that reach stderr.

##### Design notes

- The format is byte-frozen. It is the exact string a user sees;
  a change to any character is a change to the contract. A change
  requires an ADR (ADR-016) and a regenerated set of golden files.
- `Code:` is always shown. Even when the code does not help the
  user directly, it helps them search the documentation and file
  bug reports.
- Context keys are snake_case, as enforced by `WithContext`
  (WBS 10.1.2).
- Context values are redacted per WBS 10.4.3. Redaction happens at
  the CLI boundary, before `Format` is called.
- The remediation is optional but recommended for every error.
- The remediation must end with a period (rule F6). A suggestion
  that would naturally end with a question mark ("Did you mean
  'version'?") is rendered with a period instead ("Did you mean
  'version'.") to satisfy the rule. If the team wants a question
  mark to satisfy the rule, that requires an ADR amending
  WBS 10.4.1.
- There is no `Cause:` line. The cause is a programmer concern,
  reachable via `errors.Is` and `errors.As`; it is not shown to
  the user.
- The format is applied by `forgeerr.Format(err, opts)`.
  `executeWithOptions` (in `internal/cli/execute.go`) is the
  single caller; the CLI does not implement its own formatter.
- A non-`*forgeerr.Error` input (an error that is neither a
  `*forgeerr.Error` nor wraps one) is rendered by a minimal
  fallback: `Error: <msg>\n`. The fallback preserves the shape
  of the format without inventing a Code line. In practice,
  every CLI error path constructs a `*forgeerr.Error`:
  commands produce them, the validator produces them, Cobra's
  errors are wrapped by `wrapCobraError`, and the boundary wraps
  any leftover as `FORGE_INTERNAL_UNEXPECTED`. The fallback is
  reached only by a genuine contract violation.

##### The Phase 2 supersession

An earlier draft of this specification (version 0.7.0 and earlier)
described a local format with no `Code:` line and no code
prefix. WBS 10.4.1 supersedes that description. The current format
is the one in this subsection; the earlier format's examples
appear only in the version history's changelog, not in the
contract.

#### 4.10h Output Stream Boundary

WBS 7.4.1 freezes the boundary between the two output streams.

##### The boundary

| Category | Stream | Rationale |
|----------|--------|-----------|
| Command success output | stdout | Shell scripts pipe successful output |
| Command help (positive) | stdout | Help is a requested result |
| Command version (positive) | stdout | Version is a requested result |
| Command JSON output | stdout | Machine-readable is the primary output |
| Progress indicators | stderr | Do not pollute piped output |
| Warnings | stderr | Diagnostics, not results |
| Errors | stderr | Errors are never results |
| Usage after error | stderr | `SilenceUsage: true` prevents this |
| Verbose / debug logs | stderr | Diagnostics |
| Suggestions | stderr | Part of the error block |

##### The decision rule

Every author of a command decides which stream a line belongs on by
asking a single question:

> If the user pipes this command's output to another program, should
> that program receive this line?

Yes -> stdout. No -> stderr.

The rule is binary. There is no "sometimes" and no "it depends".

##### Rules

1. The decision rule is the single criterion.
2. `Dependencies.Stdout` is the only valid target for stdout writes.
3. `Dependencies.Stderr` is the only valid target for stderr writes.
4. No direct `os.Stdout` or `os.Stderr` usage in any command handler.
5. `--quiet` affects stderr only.
6. `--verbose` affects stderr only.
7. The boundary is verified by tests that redirect each stream
   separately.

#### 4.10i Global Flags Interaction Matrix

WBS 7.4.3 freezes how `--quiet`, `--verbose`, and `--config` interact
with help output, version output, and error output. WBS 9.4.2 extends
the matrix with the `forge config` rows.

##### The matrix

| Invocation | Behaviour |
|------------|-----------|
| `forge --quiet --help` | Help still printed to stdout |
| `forge --verbose --help` | Help printed to stdout; verbose adds nothing |
| `forge --quiet version` | Version printed to stdout |
| `forge --quiet unknown-cmd` | Error still printed to stderr |
| `forge --verbose unknown-cmd` | Error printed to stderr; stack trace in Phase 6+ |
| `forge --quiet --verbose version` | `--quiet` wins; version still prints |
| `forge --config nonexistent.yml version` | Error; exit 3 (config loading fails) |
| `forge --config nonexistent.yml check` | Error; exit 3 |
| `forge --quiet config` | Config printed to stdout; `--quiet` does not suppress it |
| `forge --verbose config` | Config printed to stdout; no debug output in Phase 2 |
| `forge --quiet --verbose config` | `--quiet` wins; config printed to stdout |
| `forge --config nonexistent.yml config` | Error; exit 3 |

##### The rules

1. `--quiet` suppresses warnings and info logs; it never suppresses
   errors.
2. `--quiet` never suppresses stdout results.
3. `--verbose` never adds to stdout.
4. Errors are always printed to stderr.
5. `--quiet` wins over `--verbose` when both are set.
6. Help and version are always printed to stdout.
7. An explicit `--config <path>` that names a missing file is an
   error; the loader produces it before any command runs. The CLI
   maps it to `ExitConfig` (3).
8. `--quiet` never suppresses `forge config`'s output. The command's
   output is its result; the flag's contract is not to suppress
   results. This is the same rule as item 2, applied to a specific
   command. See § 4.6.4.

##### Phase 2 scope

Phase 2 implements the log-level effects of `--quiet` and `--verbose`.
Phase 2 does not implement a stack-trace mode for `--verbose`; the row
is aspirational. Phase 2 implements the config subsystem (WBS 8.x); the
config row's behaviour is now testable.

#### 4.10j Colour and Terminal Policy

WBS 7.4.2 freezes the Phase 2 policy on colour, TTY detection, and
environment variables.

##### Phase 2 decisions

| Decision | Rationale |
|----------|-----------|
| No colour output | No UX requirement yet; colour interacts with shell scripting |
| No TTY detection | Without colour, TTY detection is unnecessary |
| `NO_COLOR` and `CLICOLOR` ignored | No colour is emitted; the standards are trivially satisfied |
| No emoji or Unicode symbols | Cross-platform consistency |
| All output uses ASCII printable characters | Maximum terminal compatibility |

##### The ASCII-only rule

Every byte the CLI writes to stdout or stderr is in the ASCII printable
range (0x20 through 0x7e) or is one of two permitted control bytes:
newline (0x0a) and tab (0x09).

Bytes outside this set are forbidden.

##### Where the rule is enforced

`internal/cli/ascii_policy_test.go` iterates the registry's commands,
invokes each in a set of representative configurations, and asserts the
policy on the captured stdout and stderr.

##### Future-proofing: when colour is introduced

When colour is introduced (Phase 6+), the following rules apply:

1. Colour is disabled when `NO_COLOR` is set (per no-color.org).
2. Colour is disabled when stdout is not a TTY.
3. Colour is disabled when `--no-color` is passed.
4. Colour never conveys semantic information alone.

#### 4.10k Root Help Contains No Non-ASCII

The root help output contains no non-ASCII characters. The
`RootLongDesc` and `RootShortDesc` constants are ASCII-only; the
subcommand help output that inherits from them is ASCII-only. The
ASCII-only policy is documented in § 4.10j.

#### 4.10l Colour and Terminal Policy (see § 4.10j)

See § 4.10j for the policy. This subsection is a stub for
cross-references.

#### 4.10m Summary of Contracts

| Contract | Source WBS | Section | Enforcement |
|----------|-----------|---------|-------------|
| Help behaviour | WBS 5.2.2 | § 4.10a | `errors_test.go` |
| Help invocation matrix | WBS 7.1.1 | § 4.10b | `help_matrix_test.go` |
| Root help content | WBS 7.1.2 | § 4.10c | `help_golden_test.go` |
| Per-command help content | WBS 7.1.3 | § 4.10d | `help_content_test.go`, `help_golden_test.go` |
| Examples convention | WBS 7.2.2 | § 4.10e | `examples_test.go` |
| Invalid command | WBS 7.3.1, WBS 10.5.1 | § 4.10f | `errors_test.go` |
| Error message format | WBS 10.4.1 | § 4.10g | `internal/forgeerr/format_test.go` |
| Output stream boundary | WBS 7.4.1 | § 4.10h | `stream_boundary_test.go` |
| Global flag interaction | WBS 7.4.3, WBS 9.4.2 | § 4.10i | `flag_interaction_test.go`, `config_interaction_test.go` |
| Colour / ASCII policy | WBS 7.4.2 | § 4.10j | `ascii_policy_test.go` |

### 4.11 Version Behaviour

The `--version` flag and the `version` subcommand have six invocations.
The table below is the contract.

#### The contract

| Invocation | Behaviour | Exit | Stream |
|------------|-----------|------|--------|
| `forge --version` | Print text version block | 0 | stdout |
| `forge -v` | Print text version block | 0 | stdout |
| `forge version` | Print text version block (identical output) | 0 | stdout |
| `forge version --format json` | Print JSON version object | 0 | stdout |
| `forge version --help` | Print version command help | 0 | stdout |
| `forge --version extra` | Rejected (version takes no args) | 2 | stderr |

#### Rules

1. **`forge --version` and `forge version` produce byte-identical text
   output.**
2. **`-v` is short for `--version`.**
3. **`-v` is not short for `--verbose`.**
4. **The `--version` flag always produces the text format.**
5. **Version output goes to stdout and exits 0.**
6. **Extra arguments to a version flag are rejected.**
7. **The version formats are frozen in § 4.9.**

### 4.12 Global Flags

Forge has four global flags.

#### The inventory

| Flag | Short | Type | Persistent | Consumer | Semantics |
|------|-------|------|------------|----------|-----------|
| `--verbose` | - | bool | yes | WBS 12.0 | Set log level to DEBUG |
| `--quiet` | - | bool | yes | WBS 12.0 | Set log level to ERROR |
| `--config` | - | string | yes | WBS 8.0 | Path to config file |
| `--format` | - | string | yes | Commands that render output | Output format (`text` or `json`) |

The four flags are persistent. Every subcommand inherits them.
`forge --verbose config` and `forge config --verbose` are equivalent;
the same is true of the other three.

**Inventory history.** Phase 2 registered the first three flags. WBS
9.4.1 (JSON output for `forge config`) added `--format` as the fourth.
The rule that a new global flag requires an ADR is unchanged; only the
inventory's count changed. ADR-010 documents the addition.

#### Precedence

When `--verbose` and `--quiet` are both set, `--quiet` wins. The
effective log level is ERROR. No warning is emitted.

`--format` has no precedence interaction with the other three: it
selects a rendering, not a level.

#### Persistence

All four flags are persistent. See the inventory table.

#### The `--config` rejection

`forge --config` with no following value is a usage error.

#### The `--format` value set

`--format` accepts `text` and `json`. Any other value is rejected with
`ExitUsage` (2) and a diagnostic on stderr that names the offending
value. The value set is frozen by WBS 6.4.1.

#### The `--format` flag is global, not per-command

An earlier version of this specification described `--format` as a
per-command flag. WBS 9.4.1 supersedes that description: the flag is
registered once on the root command and inherited by every subcommand
that renders output. The per-command aspects that remain are the value
set each command accepts (see § 4.14) and the default each command
applies when the flag is absent.

The reason the flag is global rather than per-command is that its
semantics are uniform across commands: a user who types `--format json`
expects JSON from any command that renders output. Registering it once
on the root makes the uniformity mechanical; a per-command flag would
proliferate declarations and risk drift between commands.

#### Adding a global flag

Adding a global flag requires an ADR.

### 4.13 Configuration

This subsection collects the frozen contracts that govern Forge's CLI
configuration. Each contract has a subsection below, identified by a
numeric suffix.

#### 4.13.1 CLI Configuration Scope

WBS 8.1.1 freezes the boundary between the CLI's configuration and the
project's configuration.

##### In scope (Phase 2)

The CLI configuration has exactly three conceptual fields:

| Field | Purpose | Consumer |
|-------|---------|----------|
| `log_level` | Diagnostic verbosity | WBS 12.0 |
| `output_format` | Default output format (`text` or `json`) | Commands that accept `--format` |
| `source_path` | Metadata: where the config file was loaded from | Diagnostics, `forge config` |

The first two fields are read from the config file. The third is not:
it is a resolution result, written by the config loader after it
resolves the file's path.

##### Out of scope (Phase 3 or later)

| Field | Belongs to |
|-------|------------|
| Project name, type, language | Phase 3 (Blueprint) |
| Testing, CI, database, container | Phase 3 (Blueprint) |
| Policies | Phase 8 |
| Components | Phase 12 |
| Templates | Phase 4 |

The boundary between the two is the boundary between `internal/config`
and `internal/blueprint`. The two packages are separate; the first
does not import the second.

##### Explicitly rejected (ever)

| Field | Rationale |
|-------|-----------|
| Command-specific flags | Flags are invocation-scoped, not persistent |
| Secrets, tokens, credentials | Config files are often committed; a secret in a config file is a leak |
| Repository paths | Repository-scoped, not CLI-scoped |

##### Rules

1. The `Config` type has exactly three fields. Adding a fourth requires
   an ADR.
2. `source_path` is not a config value. It is a resolution result.
3. No `forge.yaml` schema is defined in Phase 2. The `internal/config`
   package handles CLI configuration only.
4. No secrets, tokens, or credentials may ever appear in CLI
   configuration.
5. Adding a field to CLI configuration requires an ADR.

#### 4.13.2 CLI Configuration Schema

WBS 8.2.1 freezes the exact schema of CLI configuration.

##### The schema

| Field | Type | Allowed values | Default | YAML key | Env var |
|-------|------|----------------|---------|----------|---------|
| `LogLevel` | string | `debug`, `info`, `warn`, `error` | `info` | `log_level` | `FORGE_LOG_LEVEL` |
| `OutputFormat` | string | `text`, `json` | `text` | `output_format` | `FORGE_OUTPUT_FORMAT` |
| `SourcePath` | string | (metadata) | `""` | (not in file) | (not in env) |

##### The schema rules

1. Two configuration fields only. Adding a field requires an ADR.
2. Both fields are strings with enumerated values. No numeric, boolean,
   or freeform fields.
3. Enumerations are case-insensitive on input, normalised to lowercase
   on output.
4. Unknown values produce a validation error.
5. Unknown YAML keys produce a validation error (strict parsing).
6. Unknown env vars in the `FORGE_` prefix are ignored silently.
7. `SourcePath` is set by the loader, not read from the file.
8. Field names in Go use PascalCase; YAML keys use snake_case; env vars
   use `UPPER_SNAKE_CASE`. The mapping is explicit and tested.

##### The `LogLevel` values

| Value | Meaning |
|-------|---------|
| `debug` | Keep everything |
| `info` | Keep INFO, WARN, ERROR (the default) |
| `warn` | Keep WARN, ERROR |
| `error` | Keep ERROR only |

##### The `OutputFormat` values

| Value | Meaning |
|-------|---------|
| `text` | The machine-friendly line-oriented format |
| `json` | The machine-readable JSON format |

The schema has two output formats; `human` was removed by WBS 8.2.1.

#### 4.13.3 Configuration Field Naming

WBS 8.2.2 freezes the mapping between a configuration field's four
representations.

##### The mapping

| Go field | YAML key | Env var | CLI flag |
|----------|----------|---------|----------|
| `LogLevel` | `log_level` | `FORGE_LOG_LEVEL` | `--log-level` |
| `OutputFormat` | `output_format` | `FORGE_OUTPUT_FORMAT` | `--output-format` |
| `SourcePath` | (metadata) | (metadata) | (metadata) |

##### The conventions

| Representation | Convention | Example |
|----------------|------------|---------|
| Go identifier | PascalCase | `LogLevel` |
| YAML key | snake_case | `log_level` |
| Environment variable | `FORGE_` + `UPPER_SNAKE_CASE` | `FORGE_LOG_LEVEL` |
| CLI flag | kebab-case with `--` prefix | `--log-level` |

##### The derivation rules

The mapping is one-way deterministic: given a Go field name, each non-Go
representation is a pure function of it.

- **`GoFieldToYAML`** inserts an underscore before each uppercase
  letter (except the first) and lowercases the result.
- **`GoFieldToEnv`** applies `GoFieldToYAML` and uppercases the result,
  prefixed with `FORGE_`.
- **`GoFieldToFlag`** applies `GoFieldToYAML`, replaces each underscore
  with a hyphen, and prefixes with `--`.

#### 4.13.4 Configuration Defaults

WBS 8.3.1 freezes the built-in default values.

##### The defaults

| Field | Default | Rationale |
|-------|---------|-----------|
| `LogLevel` | `info` | Balanced output; not silent, not noisy |
| `OutputFormat` | `text` | Human-readable is the assumed default |

The metadata field (`SourcePath`) has no default.

##### Safety criteria

Every default satisfies all six criteria:

1. **Non-destructive.** No default causes file writes, network access,
   or process execution.
2. **Non-silent.** The `LogLevel` default (`info`) produces enough
   output to be useful.
3. **Non-noisy.** The `LogLevel` default (`info`) does not produce
   debug-level spam.
4. **Interoperable.** The `OutputFormat` default (`text`) is readable
   in any terminal.
5. **Stable.** The defaults do not change without an ADR.
6. **Testable.** Every default has a test asserting its exact value.

##### The `Defaults` function

```go
func Defaults() Config
```

The function is pure: no I/O, no package-level state, deterministic. Its
return value is the lowest-precedence layer of the configuration chain.

##### Changing a default

Changing a default requires an ADR. The ADR must name the field, state
the old and new values, explain why the new default is better, name the
WBS item, and update this section.

#### 4.13.5 Configuration Precedence

WBS 8.1.2 freezes the order in which configuration values override one
another.

##### The precedence order

Highest priority first:

| # | Source | Example | Notes |
|---|--------|---------|-------|
| 1 | CLI flags | `--log-level=debug` | Highest priority |
| 2 | Environment variables | `FORGE_LOG_LEVEL=debug` | Prefixed with `FORGE_` |
| 3 | Config file | `log_level: debug` in `~/.forge/config.yaml` | YAML |
| 4 | Built-in defaults | `log_level: info` | Frozen in code |

A higher-numbered source does **not** override a lower-numbered one.

##### Rules

1. The order is linear.
2. A flag that is explicitly set wins. A flag whose value is the flag's
   default value is not "explicitly set".
3. Environment variables use the `FORGE_` prefix.
4. Environment variable names are derived from field names by the rule
   in § 4.13.3.
5. The config file is YAML with snake_case keys.
6. Defaults are defined in code and cannot be overridden by an empty
   config file.
7. An empty flag value is treated as "not set" unless the flag
   specifically accepts empty strings.

##### Testing

The precedence chain is tested at each pairwise boundary:

- Flag beats env.
- Env beats file.
- File beats default.
- Flag beats default (transitivity).

The acceptance test is `internal/config/precedence_test.go` (WBS 9.5.2).
It exercises nine scenarios covering every pairwise boundary plus the
empty-value and case-insensitivity rules.

#### 4.13.6 Config File Discovery

WBS 8.4.1 freezes the algorithm that finds the config file's path.

##### The algorithm

The loader tries the following paths in order, stopping at the first
existing file:

| # | Source | Path | Condition |
|---|--------|------|-----------|
| 1 | `--config <path>` flag | As given | Highest priority |
| 2 | `FORGE_CONFIG` env var | As given | Second priority |
| 3 | Project-local | `./.forge/config.yaml` | If cwd is inside a repo |
| 4 | User-level (XDG) | `$XDG_CONFIG_HOME/forge/config.yaml` | If `XDG_CONFIG_HOME` is set |
| 5 | User-level (fallback) | `$HOME/.config/forge/config.yaml` | If XDG not set |
| 6 | User-level (Windows) | `%APPDATA%\forge\config.yaml` | Windows only |
| 7 | None | (empty) | Falls back to defaults |

##### Rules

1. The first existing file wins.
2. `--config` and `FORGE_CONFIG` are explicit. If the file they name
   does not exist, the algorithm produces an error.
3. Discovery paths 3 through 6 are optional.
4. A directory is not a valid config file.
5. Symlinks are followed.
6. Discovery does not create files or directories.
7. Discovery does not read the file — only checks existence.
8. Discovery is pure with respect to its inputs.

##### The Go signature

```go
func Discover(explicit string, env func(string) string, cwd string) (string, error)
```

#### 4.13.7 Config File Loading

WBS 8.4.2 freezes the config file loader.

##### The pipeline

```text
path
  -> Read file (with 1 MiB size limit)
  -> Check encoding (BOM, UTF-8)
  -> Parse YAML (strict mode: unknown fields rejected)
  -> Validate values (enum check, case-insensitive)
  -> Return Config
```

##### The Go signature

```go
func Load(path string) (Config, error)
```

##### The strictness rules

1. Unknown YAML fields produce an error.
2. Type mismatches produce an error.
3. Invalid enum values produce an error.
4. Enum matching is case-insensitive.
5. Empty files and empty mappings produce a `Config` with zero values;
   the caller applies the defaults.
6. Null values are treated as "not set".
7. Duplicate keys produce an error.
8. Malformed YAML produces an error with a line number.
9. Trailing content after the first YAML document produces an error.
10. A BOM at the file's start produces an error.
11. Environment-variable expansion is not performed.

##### The classification

The loader classifies its errors by code. The codes and the sentinels
are defined by the config error taxonomy (WBS 8.5.1, WBS 10.5.2):

| Code | Sentinel | Trigger |
|------|----------|---------|
| `FORGE_CONFIG_NOT_FOUND` | `ErrConfigNotFound` | The file does not exist. |
| `FORGE_CONFIG_PERMISSION` | `ErrConfigPermission` | The file exists but cannot be read. |
| `FORGE_CONFIG_MALFORMED` | `ErrConfigMalformed` | YAML syntax error, duplicate key. |
| `FORGE_CONFIG_UNKNOWN_FIELD` | `ErrConfigUnknownField` | The YAML contains a key the schema does not declare. |
| `FORGE_CONFIG_INVALID_TYPE` | `ErrConfigInvalidType` | A field's YAML type does not match the field's Go type. |
| `FORGE_CONFIG_INVALID_VALUE` | `ErrConfigInvalid` | A field's value is outside its allowed set; trailing content after the first document. |
| `FORGE_CONFIG_TOO_LARGE` | `ErrConfigTooLarge` | The file exceeds 1 MiB. |
| `FORGE_CONFIG_INVALID_ENCODING` | `ErrConfigInvalidEncoding` | The file starts with a BOM. |

The parse-error classification is textual: yaml.v3 does not export a
structured error type for parse failures, so the loader distinguishes
the shapes by their message prefixes. The three recognized shapes
are documented on `classifyYAMLError` in
`internal/config/loader.go`.

##### The size limit

The file is limited to 1 MiB. A larger file produces an error.

##### The library

The loader uses `gopkg.in/yaml.v3`, pinned to an exact version in
`go.mod`. The dependency is authorised by
`docs/decisions/ADR-008-yaml-library.md`.

#### 4.13.8 Environment Variable Resolution

WBS 8.4.3 freezes the algorithm that applies `FORGE_*` environment
variables to a `Config` value.

##### The algorithm

For each configuration field in the schema:

1. Compute the field's environment variable name via `EnvVarNameFor`.
2. Read the variable.
3. Trim leading and trailing whitespace from the value.
4. If the trimmed value is empty, leave the field unchanged.
5. If the trimmed value is non-empty, validate it against the field's
   allowed values.
   - A valid value overrides the field (normalised to lowercase).
   - An invalid value produces an error wrapping `ErrConfigInvalid`.

##### The Go signature

```go
func OverrideFromEnv(cfg Config, env func(string) string) (Config, error)
```

##### The rules

1. Env vars use the naming convention from § 4.13.3.
2. Empty values are treated as "not set".
3. Invalid values produce an error.
4. Enum matching is case-insensitive.
5. Unknown `FORGE_*` variables are silently ignored.
6. Values are trimmed of leading and trailing whitespace.
7. `FORGE_CONFIG` is reserved for the config file path and is not
   treated as a config value.

##### The precedence chain

The full chain, from lowest to highest:

| Layer | Applied by |
|-------|------------|
| 4. Default | `Defaults()` |
| 3. Config file | `Load(path)` |
| 2. Environment variable | `OverrideFromEnv(cfg, env)` |
| 1. CLI flag | A future WBS item |

A caller applies the layers in ascending order. Each layer's non-empty
values override the layer below.

#### 4.13.9 Configuration Summary

| Contract | Source WBS | Section | Enforcement |
|----------|-----------|---------|-------------|
| Configuration scope | WBS 8.1.1 | § 4.13.1 | `scope_test.go` |
| Configuration schema | WBS 8.2.1 | § 4.13.2 | `schema_test.go`, `config_test.go` |
| Field naming | WBS 8.2.2 | § 4.13.3 | `naming_test.go` |
| Safe defaults | WBS 8.3.1 | § 4.13.4 | `defaults_test.go` |
| Precedence order | WBS 8.1.2 | § 4.13.5 | `precedence_test.go` |
| File discovery | WBS 8.4.1 | § 4.13.6 | `discover_test.go` |
| File loading | WBS 8.4.2 | § 4.13.7 | `loader_test.go` |
| Env resolution | WBS 8.4.3 | § 4.13.8 | `env_test.go` |
| Source attribution | WBS 9.2.2 | § 4.13.10 | `source_test.go`, `precedence_test.go` |

#### 4.13.10 Source Attribution

WBS 9.2.2 freezes how the loader records the source of each
configuration value.

##### The four sources

| Source | Meaning |
|--------|---------|
| `default` | The value came from the built-in default. |
| `file` | The value came from the resolved config file. |
| `env` | The value came from an environment variable. |
| `flag` | The value came from a CLI flag. |

These are the four layers of the precedence chain (§ 4.13.5), named in
the same vocabulary. A value's source is the last layer to set it.

##### How attribution is recorded

The loader records each field's source when it applies the field's
layer. The record is stored alongside the value; it is read via
`Config.SourceFor(fieldName)`.

- The field name is the Go name (`LogLevel`, `OutputFormat`), not the
  YAML key. See § 4.13.3.
- A field that no layer set reports `default`. The zero value of
  `Source` is `default`, so a `Config` with no recorded sources reports
  `default` for every field.
- An unknown field name reports `default` rather than an error. See the
  `SourceFor` docstring for the rationale.

##### Why attribution is recorded and not inferred

An earlier design considered inferring the source by comparing the
value to the default. The approach is wrong for two cases:

1. A file that explicitly sets a field to its default value
   (`log_level: info` when the default is `info`) has the source
   `file`, not `default`. Comparison cannot distinguish the two.
2. Two layers that set the same value (for example, a file and an env
   var that both set `log_level: debug`) produce the same effective
   value; the source is the higher-priority layer. Comparison cannot
   recover which layer produced it.

Recording the source at the moment of writing is the only correct
approach.

##### Where the recorded source appears

The source is rendered by `forge config` in both output modes:

- Text: as the `(source: <src>)` suffix on each value line. See § 4.6.1.
- JSON: as the `<field>_source` sibling of each value field. See
  § 4.6.2.

No other command renders source attribution in Phase 2.

##### The Go API

```go
func (c Config) SourceFor(field string) Source
```

The method is the package's read path for attribution. The write path
is the loader's; it is not exported. The asymmetry is deliberate: a
caller cannot construct a `Config` whose sources disagree with its
fields. See the `Config` type's docstring for the rationale.

### 4.14 `--format` Flag Semantics

Commands that render output accept the global `--format` flag (§ 4.12).
Each command that accepts it defines the value set it responds to.

#### The value sets

| Command | Value set | Default | Introduced by |
|---------|-----------|---------|---------------|
| `forge version` | `text`, `json` | `text` | WBS 6.4.2 |
| `forge config` | `text`, `json` | `text` | WBS 9.2.1, WBS 9.4.1 |
| `forge new` | `human`, `json` | `human` | Phase 5 |
| `forge init` | `human`, `json` | `human` | Phase 5 |
| `forge validate` | `human`, `json` | `human` | Phase 5 |
| `forge explain` | `human`, `json` | `human` | Phase 5 |
| `forge template list` | `human`, `json` | `human` | Phase 5 |
| `forge check` | `human`, `json`, `sarif` | `human` | Phase 7 |
| `forge diff` | `human`, `json` | `human` | Phase 9 |

#### Behaviour for unsupported values

An unsupported value produces a diagnostic on stderr and exit code
`ExitUsage` (2).

#### Cross-cutting requirement

Every command that accepts `--format` must:

- define its value set (see the table above);
- reject unsupported values with exit code `ExitUsage` (2) and a
  diagnostic on stderr that names the offending value;
- write output to stdout;
- emit JSON as a single line terminated by `\n`.

The flag itself is global (§ 4.12). A command that does not render
output may ignore it. A command that renders output reads it via
`cmd.Flags().GetString("format")`; a command that can be constructed in
isolation (without a parent) reads it via the `formatValue` helper in
`internal/cli/flags.go`, which falls back to `text` when the flag is
absent.

#### A note on `human`

An earlier draft of this specification listed `human` as the config
command's text value. The shipped value is `text`. The rename aligns
the config command with the version command's text format (WBS 6.4.1).
The `human` value is not accepted by any command in Phase 2.

---

## 5. Command Reference - Future Commands

### 5.1 `forge check` (Phase 7)

```text
forge check [flags]

--format human|json|sarif
--config <path>
--quiet
--verbose
```

### 5.2 `forge diff` (Phase 9)

```text
forge diff [flags]

--format human|json
--category <category>
--severity <severity>
--quiet
--verbose
```

### 5.3 `forge update` (Phase 14)

```text
forge update [flags]

--dry-run
--conflicts
--rollback
--format human|json
--yes
```

### 5.4 `forge add` / `forge remove` (Phase 12)

```text
forge add <component> [flags]
forge remove <component> [flags]

--dry-run
--yes
--format human|json
```

### 5.5 `forge template inspect|search|install|publish` (Phase 16)

```text
forge template inspect <id> [flags]
forge template search <query> [flags]
forge template install <id>[@version] [flags]
forge template publish [flags]
```

### 5.6 `forge doctor` (Phase 6)

```text
forge doctor [flags]

--format human|json
```

### 5.7 `forge blueprint validate` (Experimental)

```text
forge blueprint validate [<path>] [flags]

--format human|json
```

---

## 6. Dry-Run Behaviour

Every mutating command must support `--dry-run`.

Dry-run is **mandatory** for:

- `forge new`
- `forge init`
- `forge update`
- `forge add`
- `forge remove`

Dry-run is **optional** for:

- `forge validate`
- `forge check`
- `forge diff`
- `forge explain`
- `forge template list`
- `forge config`
- `forge version`

---

## 7. Exit Codes

### 7.1 The codes

| Code | Constant | Meaning |
|------|----------|---------|
| 0 | `ExitSuccess` | The command completed successfully. |
| 1 | `ExitFailure` | A general failure not covered by a more specific code. |
| 2 | `ExitUsage` | The command was invoked incorrectly. |
| 3 | `ExitConfig` | Configuration loading or validation failed. |
| 4 | `ExitFilesystem` | A filesystem operation failed. |
| 5 | `ExitValidation` | A validation failure from a check or diff operation. |
| 6 | `ExitSecurity` | A security boundary was violated. |
| 7 | `ExitConflict` | An update could not be applied without overwriting developer changes. |

### 7.2 Semantics

#### 0 - Success

#### 1 - General failure

#### 2 - Usage error

Includes: unknown command, unknown flag, missing required argument,
invalid flag value (including unsupported `--format` value), and
malformed invocations rejected by `validateArgs`.

#### 3 - Configuration failure

Includes: config file not found, config file unreadable, config file
invalid (unknown field, type mismatch, invalid enum value, malformed
YAML, duplicate key, BOM, trailing content), and config env var
invalid.

#### 4 - Filesystem failure

#### 5 - Validation failure

#### 6 - Security violation

#### 7 - Update conflict

### 7.3 Exit Code Stability

These codes are part of Forge's public contract. Changing them requires
an ADR.

### 7.4 The mapping

The exit code for an error is its category's exit code. The
categories and their codes are frozen in the closed taxonomy
(WBS 10.2.1); the mapping is a method on the category type, defined in
`internal/forgeerr/category.go`.

| Category | Exit code |
|----------|-----------|
| `CategoryCLI` | `ExitUsage` (2) |
| `CategoryConfig` | `ExitConfig` (3) |
| `CategoryFilesystem` | `ExitFilesystem` (4) |
| `CategoryValidation` | `ExitValidation` (5) |
| `CategoryTemplate` | `ExitFailure` (1) |
| `CategorySecurity` | `ExitSecurity` (6) |
| `CategoryInternal` | `ExitFailure` (1) |

An error that is not a `*forgeerr.Error` and does not wrap one is
wrapped as `FORGE_INTERNAL_UNEXPECTED` at the CLI boundary; the exit
code is `ExitFailure` (1). In practice, every CLI error path produces
a `*forgeerr.Error`, so the boundary's wrapping path fires only for a
genuine contract violation.

### 7.5 Scripting

### 7.6 Reserved integers

Integers 8 and above are reserved for future categories.

---

## 8. Machine-Readable Output

### 8.1 Schema Versioning

JSON output from `forge new`, `forge init`, `forge validate`,
`forge explain`, and `forge check` includes a `schemaVersion` field.

**Exception: `forge version --format json`.** The `forge version` JSON
output does not include a `schemaVersion` field.

**Exception: `forge config --format json`.** The `forge config` JSON
output is a fixed-schema object with five fields (see § 4.6.2). It does
not include a `schemaVersion` field. The schema is identified by its
field names and is published at
[`schemas/config.schema.json`](../schemas/config.schema.json).

### 8.2 Output Streams

- **stdout** - successful command output
- **stderr** - errors, warnings, diagnostics

The `forge version --format json` command writes errors to stderr, not
to stdout as JSON.

### 8.3 JSON Stability Contract

### 8.4 Format Values

| Value | Meaning | Commands |
|-------|---------|----------|
| `text` | Machine-friendly text | `forge version`, `forge config` |
| `human` | Default human-readable output | Creation and inspection commands (Phase 5) |
| `json` | Machine-readable JSON | All commands that support `--format` |
| `sarif` | SARIF | `forge check` (Phase 10+) |

### 8.5 JSON Output is a Single Line

Every command that emits JSON emits it as a single line terminated by
`\n`.

---

## 9. CLI UX Principles

### 9.1 Predictable Commands

### 9.2 Safe Defaults

### 9.3 Actionable Errors

### 9.4 No Destructive Operations Without Confirmation

### 9.5 Scriptability

### 9.6 Human-Readable Output

Default output is optimised for humans. In Phase 2, the symbols are
ASCII-safe (see § 4.10j). The vocabulary is:

| ASCII marker | Meaning | Usage |
|--------------|---------|-------|
| `[OK]` | PASS / success | Validation, checks |
| `[WARN]` | WARNING | Non-blocking issues |
| `[ERROR]` | ERROR | Blocking issues |
| `[SKIP]` | SKIP / EXEMPT | Deliberately not evaluated |
| `[!]` | ATTENTION | Notable but not a check result |

Unicode symbols (`✓`, `⚠`, `✗`, `○`) are deferred to Phase 6.

### 9.7 Machine-Readable Output

### 9.8 Output Vocabulary

See § 9.6 for the ASCII-safe markers.

### 9.9 Progressive Disclosure

### 9.10 No Hidden Magic

### 9.11 Colour Philosophy

- Colour is a visual aid, never a requirement.
- Meaning is always encoded in the symbol or text.
- Phase 2 emits no colour. See § 4.10j.
- When colour is introduced (Phase 6+), the rules in § 4.10j apply.

### 9.12 Accessibility

- Output is readable in monochrome terminals.
- Phase 2 output is ASCII-only. See § 4.10j.
- Long lines wrap at 80 columns where practical.
- No information is conveyed by position alone.

### 9.13 Performance

### 9.14 Cancellation

---

## 10. Interactive vs Non-Interactive

| Aspect | Interactive | Non-interactive |
|--------|-------------|-----------------|
| Prompts | Shown | Never shown |
| Missing input | Prompted | Error (exit 2) |
| Ambiguity | Offered choice | Error (exit 2) |
| Confirmation | Prompted | Requires `--yes` |
| Output | Plain, formatted | Plain, JSON if `--format json` |
| TTY required | No | No |

Non-interactive mode is triggered by:

- Explicit `--non-interactive` flag
- Presence of `--format json` (implies non-interactive)

`forge version` and `forge config` are always non-interactive.

---

## 11. Error Catalogue

Every error returned by Forge is a `*forgeerr.Error` with a stable
code. The codes are enumerated in
`internal/forgeerr/codes.go` and documented in
[`docs/reference/error-codes.md`](./reference/error-codes.md).

Categories:

| Category | Meaning |
|----------|---------|
| `CLI` | Invalid command, flag, or argument |
| `CONFIG` | Configuration file issue |
| `FILESYSTEM` | Filesystem failure |
| `VALIDATION` | Validation failure |
| `TEMPLATE` | Template issue |
| `SECURITY` | Security violation |
| `INTERNAL` | Internal error |

The taxonomy is closed. Adding a category requires an ADR (ADR-013).
Adding a code requires an ADR (ADR-014) and a regenerated golden file.

Example codes (from the Phase 2 registry):

- `FORGE_CLI_UNKNOWN_COMMAND`
- `FORGE_CLI_UNKNOWN_FLAG`
- `FORGE_CLI_INVALID_ARGUMENT`
- `FORGE_CLI_UNKNOWN_FORMAT`
- `FORGE_CONFIG_NOT_FOUND`
- `FORGE_CONFIG_PERMISSION`
- `FORGE_CONFIG_MALFORMED`
- `FORGE_CONFIG_UNKNOWN_FIELD`
- `FORGE_CONFIG_INVALID_TYPE`
- `FORGE_CONFIG_INVALID_VALUE`
- `FORGE_CONFIG_TOO_LARGE`
- `FORGE_CONFIG_INVALID_ENCODING`
- `FORGE_FILESYSTEM_NOT_FOUND`
- `FORGE_FILESYSTEM_PERMISSION`
- `FORGE_FILESYSTEM_ESCAPE`
- `FORGE_SECURITY_PATH_TRAVERSAL`
- `FORGE_SECURITY_SYMLINK_ESCAPE`
- `FORGE_INTERNAL_PANIC`
- `FORGE_INTERNAL_UNEXPECTED`

The registry's full contents are the authoritative list.

---

## 12. Open Questions

The following questions remain open:

- Should `forge validate` and `forge check` be merged, or remain
  separate commands?
- Should `forge explain` support subcommands in the MVP, or defer?
- Should `forge new` write the `forge.yaml` file before or after
  generating the project files?
- Should the default output for successful operations include a `Next
  steps` block, or should that be deferred to `--verbose`?
- Should `forge version` include template versions in its output?
- What is the exact behaviour when `forge new` is given a project name
  with a `/` in it?
- Should `forge init` detect and refuse to run if the repository has
  any uncommitted Git changes?
- Should `forge template list` distinguish between local and remote
  templates once a registry exists?
- Should the JSON format of `forge version` gain a `schemaVersion`
  field once a second JSON-emitting mode is added?
- Should `forge config` gain subcommands (`get`, `set`, `unset`,
  `list`) in Phase 8, or remain a read-only view?
- Should the config file support a `$ENV_VAR` interpolation syntax, or
  is the "no expansion" rule permanent?

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

---

## 14. Document History

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 0.1.0 | 2026-10-09 | @thapelomagqazana | Initial Phase 1 draft. |
| 0.2.0 | 2026-10-10 | @thapelomagqazana | Added § 4.7 (Root Command Identity) in response to WBS 5.1.1. |
| 0.3.0 | 2026-10-10 | @thapelomagqazana | Added § 4.8 (Version Output Contract) in response to WBS 5.1.2. Corrected the "Where each string appears" table in § 4.7. |
| 0.4.0 | 2026-10-10 | @thapelomagqazana | Added § 4.9 (Help Behaviour) in response to WBS 5.2.2. Added § 4.10 (Version Behaviour) in response to WBS 5.2.3. Added § 4.11 (Global Flags) in response to WBS 5.3.1. |
| 0.5.0 | 2026-10-10 | @thapelomagqazana | Extended § 4.6, § 4.8, and § 4.10 in response to WBS 6.4.1 and WBS 6.4.2. Added § 4.12 (`--format` Flag Semantics). Updated § 8 (Machine-Readable Output). |
| 0.6.0 | 2026-10-10 | @thapelomagqazana | Added § 4.9b through § 4.9m in response to WBS 7.1.1 through WBS 7.4.3. The new subsections cover the help invocation matrix, the root and per-command help content contracts, the Examples convention, the invalid-command contract, the error message format, the output stream boundary, the global flag interaction matrix, and the colour and terminal policy. Updated § 4.7 to reflect the ASCII-only constants. Updated § 9.6 and § 9.8 to use ASCII-safe markers. |
| 0.7.0 | 2026-10-11 | @thapelomagqazana | Added § 4.13 (Configuration) with nine subsections in response to WBS 8.1.1, WBS 8.1.2, WBS 8.2.1, WBS 8.2.2, WBS 8.3.1, WBS 8.4.1, WBS 8.4.2, and WBS 8.4.3. The new subsections cover the CLI configuration scope, the schema, the field-name mapping rules, the safe defaults, the precedence order, the file discovery algorithm, the YAML loading with strict validation, and the environment-variable resolution. Added § 4.6 (`forge config` command reference). Updated § 3.1 (command tree) and § 3.2 (tier assignments) to include `forge config`. Renumbered § 4.9 through § 4.12 to § 4.10 through § 4.14 to make room for the new `forge config` section. Updated § 6 (dry-run list), § 7.2 (exit-code semantics for the config category), § 8.1 (schema-versioning exceptions), § 10 (non-interactive commands), and § 11 (error catalogue) to reference the config subsystem. |
| 0.8.0 | 2026-10-12 | @thapelomagqazana | Added § 4.6.1 through § 4.6.5 in response to WBS 9.2.1, WBS 9.3.1, WBS 9.4.1, and WBS 9.4.2. The new subsections freeze the `forge config` text output format, the JSON output format, the `--path` mode, the interaction with the global flags, and the exit codes. Added § 4.13.10 (Source Attribution) in response to WBS 9.2.2. Updated § 4.12 (Global Flags) to include `--format` as the fourth global flag, superseding the earlier "`--format` is not global" note. Updated § 4.14 (`--format` Flag Semantics) to correct the config command's value set to `text`, `json` and to add a historical note about the superseded `human` value. Added a clause to § 4.10d about the `Flags:` vs. `Global Flags:` split in a command's help output. Added four config-specific rows to § 4.10i. Updated § 4.10c to include the config row in the root help's command list. Added § 4.13.9's source-attribution row. Updated § 4.10m (Summary of Contracts) to reference the new tests. |
| 0.9.0 | 2026-10-12 | @thapelomagqazana | Rewrote § 4.10g (Error Message Format) in response to WBS 10.4.1. The frozen format now includes a `Code:` line and is implemented by `forgeerr.Format`; the previous subsection described the pre-10.4.1 local format. Added the twelve byte-level rules (F1–F12), the four variations, and four worked examples. Updated the error-message-format row in § 4.10m to name `internal/forgeerr/format_test.go` as the enforcing test. The format is byte-frozen and requires an ADR (ADR-016) to change. |
| 0.10.0 | 2026-10-12 | @thapelomagqazana | Rewrote § 4.10f (Invalid Command Behaviour) in response to WBS 10.5.1. The CLI now wraps Cobra's parser and dispatcher errors as `*forgeerr.Error` values; the rendered output uses the frozen format, and the varying data (the flag name, the command name) is in the error's context, not the message. The table of cases in § 4.10f now lists the substrings the *rendered* output contains, per the frozen format. Updated § 4.10g's design note that referenced the removed `formatError` helper; the format is now applied by `forgeerr.Format` from `executeWithOptions`. Updated § 4.10i's `--config nonexistent` row: the missing explicit path is now a hard error (exit 3), not a warning (WBS 8.4.5). Rewrote § 7.4 (Exit Code Mapping) to describe the taxonomy-based mapping (`Category.ExitCode()`) instead of the string-based mapping. Rewrote § 11 (Error Catalogue) to list the codes the CLI actually produces today (the Phase 2 registry), replacing the aspirational pre-implementation list. Updated § 4.13.7's strictness rules with the code-to-sentinel classification table (WBS 10.5.2). Updated § 4.10m's rows to reference the current enforcing tests. |
