# CLI UX Specification

- **Document type:** Specification
- **Status:** Draft
- **Version:** 0.5.0
- **Author:** @thapelomagqazana
- **Created:** 2026-10-09
- **Last Updated:** 2026-10-10
- **Supersedes:** 0.4.0
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
- Global flag semantics and precedence

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

Displays version information. The command supports two output
formats: `text` (the default) and `json`. The text format is frozen
by WBS 6.4.1 and documented in § 4.8.1. The JSON format is frozen by
WBS 6.4.2 and documented in § 4.8.2.

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

`--format text` and `--format=text` are equivalent. `--format json`
and `--format=json` are equivalent. Any other value is rejected with
exit code `ExitUsage` (2) and a diagnostic on stderr that names the
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

### 4.7 Root Command Identity

The root command's identity strings are frozen. They are defined as
package-level constants in
[`internal/cli/root.go`](../internal/cli/root.go) and consumed by
tests, documentation, and help output.

#### The four constants

| Constant | Value |
|----------|-------|
| `RootName` | `forge` |
| `RootUsage` | `forge [command]` |
| `RootShortDesc` | `Forge — Engineering Foundations as Code` |
| `RootLongDesc` | see below |

`RootLongDesc` is a raw string literal:

```text
Forge is a cross-platform CLI for defining, generating,
validating, and evolving software project foundations as code.

The core loop:

    CREATE  →  forge new
    VERIFY  →  forge check
    EXPLAIN →  forge explain
    EVOLVE  →  forge update

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
   limit is measured in runes, not bytes, so the em dash (—) counts
   as one character.
3. `RootLongDesc` is a raw string literal. Its line breaks and
   indentation are preserved verbatim in the help output.
4. `RootLongDesc` contains the CREATE / VERIFY / EXPLAIN / EVOLVE
   loop. The four keywords are the product's core loop; removing
   any of them is a product decision, not a copy edit.
5. `RootLongDesc` ends with a pointer to `<command> --help`. The
   pointer tells a user how to learn more about a specific
   subcommand.
6. None of the four contains emojis, ANSI colour codes, or tabs.
   Colour and emphasis belong to the terminal, not to the CLI's
   identity strings.

#### Changing a string

Changing any of the four constants requires:

1. Updating the constant in `internal/cli/root.go`.
2. Updating the corresponding assertion in
   `internal/cli/root_test.go`.
3. Updating this section of `docs/cli-ux-spec.md` to quote the new
   value.
4. Running `task check` and confirming the full suite passes.

The three updates are in the same commit. A reviewer who sees a
constant change without the corresponding test and documentation
changes rejects the PR.

#### Why these strings are frozen

The root command's identity appears in every invocation of the CLI,
in every documentation page that describes the CLI, and in every
shell script that captures `forge --help` output. Changing the
strings is cheap in code and expensive in every artefact that
quotes them. Freezing the strings is the mechanism by which the
cost is bounded: a change requires a deliberate update to three
files, which a reviewer sees.

### 4.8 Version Output Contract

The `--version` flag and the `version` subcommand produce output
derived from a single formatter in `internal/app/version`. The
`version` subcommand supports two output formats: `text` (the
default) and `json`. The `--version` flag always produces the text
format, because a flag cannot take a format argument.

#### 4.8.1 Text Format (frozen by WBS 6.4.1)

The text format is the default. It is produced by
`Format(w io.Writer, info Info) error` in
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
- The value column is fixed at position 15 (1-indexed) on every
  detail line.
- The output ends with a trailing newline.
- The output contains no ANSI escape sequences, regardless of
  whether stdout is a TTY.

**Field values:**

| Field | Source | Example |
|-------|--------|---------|
| `<version>` | `internal/version.Version` (injected via `-X` at build time) | `0.1.0` |
| `<commit>` | `internal/version.Commit` (injected via `-X`) | `a1b2c3d` |
| `<build-date>` | `internal/version.BuildDate` (injected via `-X`) | `2026-10-09T08:00:00Z` |
| `<dirty>` | `internal/version.Dirty` (injected via `-X`) | `false` |
| `<go-version>` | `runtime.Version()` | `go1.23.4` |
| `<os>/<arch>` | `runtime.GOOS + "/" + runtime.GOARCH` | `linux/amd64` |

**The `dirty` rule:** Only the literal `"true"` renders as `true`.
Every other value — including `"1"`, `"yes"`, `"TRUE"`, and the
empty string — renders as `false`. The rule is documented on
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

**Why the format is frozen:** Scripts that parse the output depend
on its shape. The format is a contract with those scripts, in the
same way that the exit codes are a contract with the shells that
branch on them. Freezing the format bounds the cost of a change:
a change requires updating the formatter, the tests, and this
document in one commit, which a reviewer sees.

#### 4.8.2 JSON Format (frozen by WBS 6.4.2)

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
- All four keys are always present, even when their values are
  empty.
- Field order is `version`, `commit`, `build_date`, `dirty`.
- The output contains no ANSI escape sequences.

**Example output:** The line below is the exact output for a
populated build:

```json
{"version":"0.1.0","commit":"a1b2c3d","build_date":"2026-10-09T08:00:00Z","dirty":false}
```

**Uninstrumented example:** The line below is the exact output for
a build without `-X` flags:

```json
{"version":"","commit":"","build_date":"","dirty":false}
```

**Schema versioning:** The JSON schema is versioned by its field
names. A breaking change to any field name requires an ADR. Adding
a new field is additive and requires a new WBS item but not an
ADR.

**Why the schema is frozen:** Machine consumers parse the output.
The schema is a contract with those consumers, in the same way the
text format is a contract with human-facing scripts. Freezing the
schema bounds the cost of a change: a change requires updating the
encoder, the tests, the JSON Schema document, and this section in
one commit, which a reviewer sees.

#### 4.8.3 Where the format is implemented

The text format is implemented in
[`internal/app/version/format.go`](../internal/app/version/format.go),
in the function `Format`. The function writes to an `io.Writer`. The
function `Raw` in
[`internal/app/version/service.go`](../internal/app/version/service.go)
returns the same string as a `string`, for callers that need the
value rather than a stream.

The JSON format is implemented in
[`internal/app/version/format_json.go`](../internal/app/version/format_json.go),
in the function `WriteJSON`. The function is symmetric with the
text formatter: it takes an `io.Writer` and the same `Info` value,
and it writes the JSON object followed by a newline.

The dispatcher is `FormatAs(w io.Writer, info Info, format string)`,
also in `format_json.go`. It routes to `WriteText` for
`FormatText` and to `WriteJSON` for `FormatJSON`, and returns an
error wrapping `ErrUnknownFormat` for any other value.

The three CLI invocations consume the formatters:

- `forge version` (the subcommand, default format) calls
  `FormatAs(deps.Stdout, version.Get(), FormatText)`, which routes
  to the text formatter.
- `forge version --format json` calls
  `FormatAs(deps.Stdout, version.Get(), FormatJSON)`, which routes
  to the JSON formatter.
- `forge --version` (the flag) reads `version.Raw()` in `newRootCmd`
  and assigns the result to `root.Version`. Cobra renders the flag's
  output using a version template that prints the value verbatim:

  ```go
  root.SetVersionTemplate("{{.Version}}")
  ```

  Cobra's default template prepends `"forge version "` to the value
  and appends its own trailing newline. Forge overrides the default
  so that the flag's output is exactly the value produced by
  `version.Raw`, with no prefix and no extra newline.

Because the `--version` flag and the default `forge version` both
derive from the same text formatter, they cannot diverge without a
change to the formatter. The JSON formatter is used only by
`forge version --format json`.

#### 4.8.4 Changing a format

Changing either format requires:

1. Updating the corresponding formatter in
   `internal/app/version/` (`format.go` for text, `format_json.go`
   for JSON).
2. Updating the golden files under
   `internal/app/version/testdata/format/`.
3. Updating the byte-for-byte test for the format
   (`TestFormat_FullOutput` for text, `TestWriteJSON_FullOutput`
   for JSON).
4. Updating the corresponding subsection of § 4.8 above.
5. For a JSON change that renames or removes a field, opening an
   ADR. Adding a field is additive and does not require an ADR.
6. Running `task check` and confirming the full suite passes.

The updates are in the same commit. A reviewer who sees a change
to a formatter without the corresponding documentation update
rejects the PR.

#### 4.8.5 Why the format is frozen

Scripts that parse `forge --version`, `forge version`, or
`forge version --format json` depend on the format. The format is
a contract with those scripts, in the same way that the exit codes
are a contract with the shells that branch on them. Freezing the
format is the mechanism by which the cost of a change is bounded:
a change requires the updates listed in § 4.8.4, and a reviewer
sees all of them in one diff.

### 4.9 Help Behaviour

Forge's help system has eight entry points. The table below is the
contract: every release must handle every row consistently.

#### The contract

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

#### Rules

1. **All help goes to stdout, never stderr.** The help text is a
   successful result, not a diagnostic.

2. **All help exits 0.** The user's request was fulfilled.

3. **Errors about unknown help topics go to stderr and exit 2.**
   `forge help unknown` is a usage error: the user asked for help on
   a topic that does not exist.

4. **`--help` and `-h` are equivalent.** Both produce the same
   output and the same exit code.

5. **`forge help <cmd>` and `forge <cmd> --help` are equivalent.**
   The two forms produce the same output and the same exit code.

6. **`forge --help <cmd>` is rejected.** `--help` is a flag that
   takes no argument. Passing a positional argument after it is a
   usage error.

7. **The help output contains the frozen identity strings.** The
   output includes `RootName`, the composed usage line built from
   `RootName` and `RootUsage`, and the body of the help text
   (`RootLongDesc`). `RootShortDesc` is not rendered by
   `forge --help` for the root command; see § 4.7.

8. **The help output lists every visible registered command.** Every
   visible constructor in `internal/cli/registry.go` produces a
   command that appears in the "Available Commands:" section. Hidden
   commands do not appear; they are documented separately.

#### Where the behaviour is implemented

Six of the eight invocations are handled by Cobra's defaults. The two
rejections are handled by `validateArgs` (validate.go), which runs in
`executeWithOptions` before Cobra parses the arguments. The rejection
cannot live in a Cobra hook: Cobra's `--help` interception
short-circuits the hook chain, so a `PersistentPreRunE` or `RunE`
never sees the malformed invocation.

#### Why the contract is frozenEvery CLI user encounters the help system. Every script that captures

help output for documentation or for error reporting depends on help
going to stdout and the exit code being 0. Freezing the contract is
the mechanism by which the cost of a change is bounded.

### 4.10 Version Behaviour

The `--version` flag and the `version` subcommand have six
invocations. The table below is the contract.

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

1. **`forge --version` and `forge version` produce byte-identical
   text output.** The two invocations are interchangeable for
   scripts and for users. The text format is frozen in § 4.8.1.

2. **`-v` is short for `--version`.** The two forms produce the same
   output, the same exit code, and the same stderr.

3. **`-v` is not short for `--verbose`.** `--verbose` has no short
   form in Phase 2. The alias is reserved for `--version`, in keeping
   with the convention used by `git`, `go`, `cargo`, and many other
   CLIs.

4. **The `--version` flag always produces the text format.** A flag
   cannot take a format argument. A consumer that needs the JSON
   format calls `forge version --format json`.

5. **Version output goes to stdout and exits 0.**

6. **Extra arguments to a version flag are rejected.** `forge
   --version extra` and `forge -v extra` are usage errors: the
   version flag takes no argument. The CLI rejects the invocation
   with a diagnostic on stderr and exit code 2.

7. **The version formats are frozen in § 4.8.** This section does
   not restate the formats; it references the section that defines
   them.

#### Where the behaviour is implemented

The text block is produced by `internal/app/version`, in the
`Format` function (format.go). The JSON block is produced by
`internal/app/version`, in the `WriteJSON` function
(format_json.go). The dispatcher is `FormatAs`, which routes to the
correct formatter based on the format name.

The `--version` flag's wiring is implemented in `newRootCmd`
(root.go), which sets `root.Version` to the string returned by
`version.Raw()` and overrides Cobra's default version template.

The extra-argument rejection is implemented in `validateArgs`
(validate.go). The check cannot live in a Cobra hook for the same
reason as the help-flag rejection.

#### Why the contract is frozen

The version output is the most commonly parsed CLI output. Scripts
extract the version number to decide whether to upgrade; CI systems
compare versions to decide whether to rebuild; users pipe the output
to `grep` to answer "am I on the right build?". Freezing the contract
is what makes those consumers safe.

### 4.11 Global Flags

Forge has exactly three global flags in Phase 2. Global flags are
**persistent**: they are inherited by every subcommand and may appear
before or after the subcommand name.

#### The inventory

| Flag | Short | Type | Persistent | Consumer | Semantics |
|------|-------|------|------------|----------|-----------|
| `--verbose` | — | bool | yes | WBS 12.0 (logging) | Set log level to DEBUG |
| `--quiet` | — | bool | yes | WBS 12.0 (logging) | Set log level to ERROR (errors only) |
| `--config` | — | string | yes | WBS 8.0 (config) | Path to config file; overrides discovery |

No other global flags exist. The inventory is frozen; adding a fourth
flag requires an ADR (see "Adding a global flag" below).

#### Justification

Each flag has a documented consumer in a later WBS item:

- **`--verbose`** is required by WBS 12.5 (verbose mode). It raises
  the log level to DEBUG, causing the logger to emit diagnostic
  messages that are suppressed by default.

- **`--quiet`** is required by WBS 12.6 (quiet mode). It lowers the
  log level to ERROR, suppressing INFO and WARN messages while
  preserving ERROR messages and the command's own output.

- **`--config`** is required by WBS 8.4 (configuration loader). It
  provides a path to a configuration file, overriding the loader's
  discovery mechanism.

No flag is speculative. A flag that has no consumer in a later WBS
item is not in the inventory.

#### Precedence

When `--verbose` and `--quiet` are both set, **`--quiet` wins**.
The effective log level is ERROR.

The rationale is that `--quiet` is the stricter contract: the user
who asked for quiet asked for a smaller output surface, and honoring
a smaller surface when a larger one is also requested is the correct
default.

When both flags are set, the CLI does **not** emit a warning about
the conflict. The rationale is that a warning about conflicting
flags is itself output, and the user who asked for quiet asked for
less output. Emitting a warning would violate the quiet contract.

#### Persistence

All three flags are **persistent**. Persistent flags are inherited
by every subcommand. This means:

- `forge --verbose config` and `forge config --verbose` are
  equivalent.
- `forge --config path version` and `forge version --config path`
  are equivalent.

Cobra's parser accepts global flags either before or after the
subcommand name.

#### The `--config` rejection

`--config` takes a value. An invocation like `forge --config` with
no following value is a usage error: the flag is present but its
value is not. The rejection is implemented in `validateArgs`
(validate.go), which runs before Cobra parses the arguments. The
`--config=path` form is well-formed and is not rejected; the form
carries its own value.

#### The `--format` flag is not global

`--format` is a per-command flag. It appears on `forge version`
(this WBS), and it will appear on every command that produces
structured output. It is not in the global flag inventory, because
its value type and its allowed values differ between commands.

A global `--format` flag would force every command to accept the
same value set. The `forge check` command will accept `sarif`
(Phase 10+); `forge version` does not. A per-command flag lets each
command define its own value set without cross-command coupling.

#### Adding a global flag

Adding a global flag requires an ADR. The ADR must:

1. Name the consumer WBS item that requires the flag.
2. Define the flag's type, default value, and semantics.
3. Define the flag's precedence relative to the existing flags (if
   it conflicts with any).
4. Update this section of `docs/cli-ux-spec.md` to list the new
   flag.

The inventory is frozen for Phase 2. The rule exists to prevent flag
creep: a CLI with fifteen global flags has no global flags, because
users cannot remember which one does what.

#### Where the flags are implemented

The flag names, registration, and helpers are defined in
`internal/cli/flags.go`. The registration is called from
`newRootCmd` (root.go). The pre-parse rejection of `forge --config`
with no value is implemented in `validateConfigFlagWithArgs`
(validate.go).

### 4.12 `--format` Flag Semantics

Commands that produce structured output accept a per-command
`--format` flag. The flag's value set is defined by the command, not
by the CLI.

#### The value sets

| Command | Value set | Default | Introduced by |
|---------|-----------|---------|---------------|
| `forge version` | `text`, `json` | `text` | WBS 6.4.2 |
| `forge new` | `human`, `json` | `human` | Phase 5 (future) |
| `forge init` | `human`, `json` | `human` | Phase 5 (future) |
| `forge validate` | `human`, `json` | `human` | Phase 5 (future) |
| `forge explain` | `human`, `json` | `human` | Phase 5 (future) |
| `forge template list` | `human`, `json` | `human` | Phase 5 (future) |
| `forge check` | `human`, `json`, `sarif` | `human` | Phase 7 (future) |
| `forge diff` | `human`, `json` | `human` | Phase 9 (future) |

The value sets differ. `forge version` uses `text` because its
output is a two-word, machine-friendly format; the other commands
use `human` because their output is prose. `forge check` will
accept `sarif` in Phase 10+; no other command will.

#### Behaviour for unsupported values

An unsupported value for a command's `--format` flag produces a
diagnostic on stderr and exit code `ExitUsage` (2). The diagnostic
names the flag and the offending value.

The diagnostic format is:

```text
Error: --format: <command-specific message>
```

For `forge version`, the message is `version: unknown format:
"<value>"` (from the service's `ErrUnknownFormat` sentinel).

#### Why per-command, not global

Three reasons:

1. **Different value sets.** `forge version` needs `text`; `forge
   check` needs `sarif`. A global flag cannot express both.

2. **Different defaults.** `forge version` defaults to `text`;
   `forge new` defaults to `human`. A global flag has one default.

3. **Different failure modes.** `forge version --format yaml` fails
   immediately (the format name is unknown). `forge new --format
   json --dry-run` fails later (the combination is disallowed). The
   two failures happen at different layers, and the per-command
   flag lets each command decide where to enforce its own
   constraints.

#### Cross-cutting requirement

Every command that accepts `--format` must:

- Define its value set in the command's section of this document.
- Reject unsupported values with exit code 2.
- Write output to stdout, regardless of format.
- Ensure that JSON output is a single line terminated by `\n`.

The last requirement is shared across commands. The first JSON
formatter (`forge version`) is the reference implementation; a
future command's formatter follows the same shape.

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
- `forge version` (already non-mutating)

---

## 7. Exit Codes

Forge uses stable exit codes so that scripts and CI systems can
interpret results without parsing output. The codes are part of
Forge's public contract: a code, once shipped, is never renumbered.

### 7.1 The codes

| Code | Constant          | Meaning                                                             |
|------|-------------------|---------------------------------------------------------------------|
| 0    | `ExitSuccess`     | The command completed successfully.                                 |
| 1    | `ExitFailure`     | A general failure not covered by a more specific code.              |
| 2    | `ExitUsage`       | The command was invoked incorrectly: unknown command, unknown flag, malformed argument. |
| 3    | `ExitConfig`      | Configuration loading or validation failed.                         |
| 4    | `ExitFilesystem`  | A filesystem operation failed.                                      |
| 5    | `ExitValidation`  | A validation failure from a check or diff operation.                |
| 6    | `ExitSecurity`    | A security boundary was violated.                                   |
| 7    | `ExitConflict`    | An update could not be applied without overwriting developer changes. |

The constants are defined in
[`internal/cli/exitcodes.go`](../internal/cli/exitcodes.go). They are
exported from that package.

### 7.2 Semantics

#### 0 — Success

The command completed successfully. No findings of severity `ERROR`.

#### 1 — General failure

The command ran but failed for a reason that does not fall into a
more specific category. This is the default for unclassified errors.

Typical causes:

- An internal error that has not been categorised.
- A future error category that is not yet recognised by the exit
  code mapping.

#### 2 — Usage error

Forge could not interpret the command:

- Unknown command or subcommand.
- Unknown flag.
- Missing required argument.
- Invalid flag value, including an unsupported `--format` value.
- Malformed `forge.yaml` (a syntax error, not a semantic error).
- Ambiguous foundation selection in non-interactive mode.
- Malformed invocations rejected by `validateArgs` (for example,
  `--help <cmd>`, `--version <arg>`, `--config` with no value, or
  `help <unknown>`).

This is the default for any error that does not carry a category.
Cobra's own errors (unknown command, unknown flag) are always
classified as `ExitUsage`.

#### 3 — Configuration failure

A configuration file failed to load, parse, or validate:

- `forge.yaml` missing when required.
- `forge.yaml` semantically invalid (a field is the wrong type, a
  required field is missing).
- Environment variable that overrides configuration is invalid.

See WBS 8.x for the configuration subsystem specification.

#### 4 — Filesystem failure

Forge was unable to read or write a required file:

- Permission denied.
- Disk full.
- Target directory already exists.
- Read-only file system.

See WBS 13.x for the filesystem subsystem specification.

#### 5 — Validation failure

The command ran a validation and one or more required conditions
were not satisfied:

- `forge check` found ERROR-level findings.
- `forge diff` detected actionable drift.
- `forge validate` found ERROR-level findings.

See WBS 7.x and WBS 9.x for the validation and drift subsystem
specifications.

#### 6 — Security violation

Forge refused to perform an operation because of a security rule:

- A template attempted path traversal.
- A template attempted to escape the target directory.
- A template requested hooks without opt-in.
- A symlink escape was attempted.
- A secret leak was detected.

See WBS 13.x for the security subsystem specification.

#### 7 — Update conflict

An update could not be applied without overwriting developer changes:

- Both Forge and the developer modified the same region.
- A file was deleted by the developer and modified by Forge.
- A merge could not be performed automatically.

See WBS 14.x for the update subsystem specification.

### 7.3 Exit Code Stability

These codes are part of Forge's public contract. Changing them requires
an ADR.

### 7.4 The mapping

Errors are mapped to exit codes by the function
`exitCodeFromError`, defined in `internal/cli/exitcodes.go`. It is
the only function in the package that derives an exit code from an
error.

The mapping is:

| Error condition                                    | Exit code         |
|----------------------------------------------------|-------------------|
| No error                                           | `ExitSuccess`     |
| Error with `Category() == "config"`                | `ExitConfig`      |
| Error with `Category() == "filesystem"`            | `ExitFilesystem`  |
| Error with `Category() == "validation"`            | `ExitValidation`  |
| Error with `Category() == "security"`              | `ExitSecurity`    |
| Error with `Category() == "conflict"`              | `ExitConflict`    |
| Error with an unrecognised category                | `ExitFailure`     |
| Error without a category (Cobra's usage errors)    | `ExitUsage`       |

An error "carries a category" if it implements the
`CategorizedError` interface defined in
`internal/cli/exitcodes.go`. The interface has one method,
`Category() string`. The values shown in the table above are the
recognised values. Any other value falls through to `ExitFailure`.

### 7.5 Scripting

Scripts that invoke Forge should check the exit code to determine
the outcome. For example:

```sh
if forge check; then
    echo "Foundation intact"
else
    case "$?" in
        5) echo "Validation failed" ;;
        6) echo "Security boundary violated" ;;
        7) echo "Update conflict" ;;
        *) echo "Unexpected failure" ;;
    esac
fi
```

### 7.6 Reserved integers

Integers 8 and above are reserved for future categories. They are
not assigned to any constant. A future WBS that introduces a new
failure category allocates the next available integer.

The reservation is deliberate: it prevents a future category from
being assigned a value that collides with an existing code.

---

## 8. Machine-Readable Output

Every command that produces structured output supports a
per-command `--format json` flag. JSON output is governed by schema
versions to enable safe automation.

### 8.1 Schema Versioning

JSON output from the `forge new`, `forge init`, `forge validate`,
`forge explain`, and `forge check` commands includes a
`schemaVersion` field:

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

**Exception: `forge version --format json`.** The `forge version`
JSON output does not include a `schemaVersion` field. Its schema is
trivially small (four fields, all strings or boolean) and stable by
construction; the four field names are the schema. A future change
to any field name requires an ADR (per § 4.8.2). The absence of a
`schemaVersion` field is deliberate: a field that is always `"1"`
adds no information, and a consumer that needs to detect a version
change reads the field names.

If the schema grows beyond four fields, or if a second JSON-emitting
version of the command is added (for example, one that emits more
fields under a different flag), a `schemaVersion` field is added at
that point. Until then, the schema is identified by its field names.

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

The `forge version --format json` command is an exception to this
rule. If the command fails (for example, because the format value
is unknown, or because the writer failed), the diagnostic goes to
stderr as plain text, not to stdout as JSON. The reason is that the
failure happens before or during the JSON emission, and a partial
JSON document on stdout is worse than no JSON document. The
`forge version` JSON schema has no `status` or `error` field;
consumers detect failure by the exit code.

### 8.3 JSON Stability Contract

The following fields are contractually stable and will not be renamed
or removed within a schema version:

- `schemaVersion` (where present; see § 8.1)
- `forgeVersion` (where present)
- `status` (where present)
- `error.code` (where present)
- `error.message` (where present)
- For `forge version --format json`: `version`, `commit`,
  `build_date`, `dirty` (all four; see § 4.8.2)

All other fields are stable within a schema version but may evolve in
a new schema version.

### 8.4 Format Values

Supported `--format` values:

| Value | Meaning | Commands |
|-------|---------|----------|
| `text` | Machine-friendly text | `forge version` |
| `human` | Default human-readable output | `forge new`, `forge init`, `forge validate`, `forge explain`, `forge template list`, `forge check`, `forge diff` |
| `json` | Machine-readable JSON | All commands that support `--format` |
| `sarif` | SARIF | `forge check` (Phase 10+) |

Unsupported format values exit with code 2. The value set is
per-command; see § 4.12 for the full table.

### 8.5 JSON Output is a Single Line

Every command that emits JSON emits it as a single line terminated
by `\n`. The output contains no pretty-printing by default. A future
`--format json-pretty` would be an additive change and a separate
value in the per-command value set.

The single-line rule has two reasons:

1. **Line-oriented consumers.** A script that reads one JSON object
   per line (`while read -r line; do ...; done`) expects each
   record to occupy exactly one line. Multi-line JSON breaks such
   scripts.

2. **Log interleaving.** When JSON output is written to a log that
   interleaves output from multiple processes, a single-line record
   is easier to filter and parse than a multi-line one.

The rule is enforced by the `forge version` JSON formatter (the
reference implementation) and must be enforced by every future JSON
formatter.

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

- Schema-versioned (where a schema version is meaningful; see § 8.1)
- Stable across releases within a schema version
- Free of terminal formatting codes
- Deterministic ordering where practical
- A single line terminated by `\n` (see § 8.5)

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

`forge version` is always non-interactive. It has no prompts, no
confirmations, and no ambiguity; the command produces output and
exits.

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
- `FORGE_USAGE_UNKNOWN_FLAG`
- `FORGE_USAGE_UNKNOWN_FORMAT`
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
(§ 8) where the JSON schema has an error field. The
`forge version --format json` schema does not have an error field;
see § 8.2.

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
- Should the JSON format of `forge version` gain a `schemaVersion`
  field once a second JSON-emitting mode is added (for example,
  a `--format json-full` that includes additional fields)?

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

---

## 14. Document History

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 0.1.0 | 2026-10-09 | @thapelomagqazana | Initial Phase 1 draft. Command hierarchy, per-command reference, exit codes, JSON output, UX principles, error catalogue, and open questions. |
| 0.2.0 | 2026-10-10 | @thapelomagqazana | Added § 4.7 (Root Command Identity) in response to WBS 5.1.1. Documents the four frozen identity constants, their rules, and the process for changing them. |
| 0.3.0 | 2026-10-10 | @thapelomagqazana | Added § 4.8 (Version Output Contract) in response to WBS 5.1.2. Documents the frozen format shared by `forge --version` and `forge version`, the field sources, the single formatter, and the process for changing the format. Corrected the "Where each string appears" table in § 4.7: `RootShortDesc` is not rendered by `forge --help`; `RootLongDesc` is the body of the help text, not the text below the short description. |
| 0.4.0 | 2026-10-10 | @thapelomagqazana | Added § 4.9 (Help Behaviour) in response to WBS 5.2.2. Documents the eight help invocations, the stdout/stderr contract, and the two rejections. Added § 4.10 (Version Behaviour) in response to WBS 5.2.3. Documents the five version invocations, the `-v` alias, and the extra-argument rejection. Added § 4.11 (Global Flags) in response to WBS 5.3.1. Documents the three-flag inventory, the precedence rule, the persistence of the flags, and the process for adding a new flag. Extended § 7.2's usage-error list to cite the pre-parse rejections and updated § 2's in-scope list. |
| 0.5.0 | 2026-10-10 | @thapelomagqazana | Extended § 4.6, § 4.8, and § 4.10 in response to WBS 6.4.1 and WBS 6.4.2. § 4.8 now has three subsections: § 4.8.1 freezes the text format byte-for-byte; § 4.8.2 freezes the JSON schema field-for-field; § 4.8.3 names the implementation files; § 4.8.4 lists the change process; § 4.8.5 justifies the freeze. § 4.10 lists six version invocations (up from five), adds `forge version --format json` to the contract table, and adds a rule that `--version` always produces the text format. § 4.12 (new) defines the `--format` flag as a per-command flag with a per-command value set, distinct from the global flag inventory; it explains why `--format` is not global, and lists the value sets for every command that has one or will have one. § 8.1 documents the deliberate exception that `forge version --format json` does not carry a `schemaVersion` field and explains why. § 8.2 documents that `forge version --format json` writes errors to stderr rather than embedding them in JSON. § 8.4 adds `text` to the format-value table and adds a per-command column. § 8.5 documents the single-line JSON rule with its rationale. § 9.7 and § 10.10 updated to reference § 8.5 and to note that `forge version` is always non-interactive. § 11 adds `FORGE_USAGE_UNKNOWN_FLAG` and `FORGE_USAGE_UNKNOWN_FORMAT` to the example error codes. § 12 adds an open question about a `schemaVersion` field for a future full version JSON. |
