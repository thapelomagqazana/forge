# CLI UX Specification

- **Document type:** Specification
- **Status:** Draft
- **Version:** 0.6.0
- **Author:** @thapelomagqazana
- **Created:** 2026-10-09
- **Last Updated:** 2026-10-10
- **Supersedes:** 0.5.0
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

The prompts use ASCII characters only (see § 4.9l). The interactive
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
glyph. See § 4.9l for the ASCII-only policy.

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
   loop. The four keywords are the product's core loop; removing
   any of them is a product decision, not a copy edit.
5. `RootLongDesc` ends with a pointer to `<command> --help`. The
   pointer tells a user how to learn more about a specific
   subcommand.
6. None of the four contains emojis, ANSI colour codes, or tabs.
7. All four are ASCII-only. The em dash and right arrow used in an
   earlier draft were replaced with the ASCII equivalents `-` and
   `->` when the ASCII-only policy (WBS 7.4.2) was frozen. See
   § 4.9l "Colour and Terminal Policy" for the policy.

#### Changing a string

Changing any of the four constants requires:

1. Updating the constant in `internal/cli/root.go`.
2. Updating the corresponding assertion in
   `internal/cli/root_test.go`.
3. Updating this section of `docs/cli-ux-spec.md` to quote the new
   value.
4. Regenerating the golden files under
   `internal/cli/testdata/help/` that contain the rendered value.
5. Running `task check` and confirming the full suite passes.

The updates are in the same commit. A reviewer who sees a constant
change without the corresponding test and documentation changes
rejects the PR.

#### Why these strings are frozen

The root command's identity appears in every invocation of the CLI,
in every documentation page that describes the CLI, and in every
shell script that captures `forge --help` output. Changing the
strings is cheap in code and expensive in every artefact that
quotes them. Freezing the strings is the mechanism by which the
cost is bounded: a change requires a deliberate update to several
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
Every other value - including `"1"`, `"yes"`, `"TRUE"`, and the
empty string - renders as `false`. The rule is documented on
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

### 4.9 Help and Error Contracts

This subsection collects the frozen contracts that govern help
output, per-command help, invalid-command behaviour, error messages,
output streams, colour and ASCII policy, and global flag
interaction. Each contract has a subsection below, identified by a
letter suffix.

#### 4.9a Help Behaviour

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

##### Where the behaviour is implemented

Six of the eight invocations are handled by Cobra's defaults. The two
rejections are handled by `validateArgs` (validate.go), which runs in
`executeWithOptions` before Cobra parses the arguments. The rejection
cannot live in a Cobra hook: Cobra's `--help` interception
short-circuits the hook chain, so a `PersistentPreRunE` or `RunE`
never sees the malformed invocation.

##### Why the contract is frozen

Every CLI user encounters the help system. Every script that captures
help output for documentation or for error reporting depends on help
going to stdout and the exit code being 0. Freezing the contract is
the mechanism by which the cost of a change is bounded.

#### 4.9b Help Invocation Matrix

WBS 7.1.1 defines the complete inventory of every way to reach
help. The matrix is the contract; each row has a test in
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
8. Help output contains the frozen root identity strings from
   WBS 5.1.1.

##### Adding a new help path

Adding a new help path requires adding a row to the matrix, adding
a case to `internal/cli/help_matrix_test.go`, and (if the row is a
new equivalence class) adding a dedicated test. The three updates
are in the same commit.

#### 4.9c Root Help Output Contract

WBS 7.1.2 freezes the exact structure and content of the root
command's help output. The output is compared byte-for-byte against
`internal/cli/testdata/help/root.golden.txt`.

##### The frozen structure

```text
<RootLongDesc>

Usage:
  forge [command] [flags]
  forge [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  version     Print Forge version information

Flags:
      --config string   path to the configuration file (overrides discovery)
  -h, --help            help for forge
      --quiet           suppress all output except errors (log level ERROR)
      --verbose         enable verbose output (log level DEBUG)
  -v, --version         version for forge

Use "forge [command] --help" for more information about a command.
```

##### Contract properties

1. **No blank line at the start of the output.** The first line is
   the first line of `RootLongDesc`.

2. **One blank line between `RootLongDesc` and `Usage:`.**

3. **Commands are listed alphabetically by the `Use` field.**

4. **Command short descriptions are aligned with padding to the
   longest command name plus two spaces.**

5. **Flags are listed with their descriptions aligned similarly.**

6. **The trailing `Use ...` line is mandatory.**

7. **The output contains no ANSI escape sequences.**

8. **The output ends with a trailing newline.**

9. **The output is identical on all supported platforms.**

##### Where the behaviour is implemented

The root help output is rendered by Cobra from the root command's
`Use`, `Short`, and `Long` fields, plus the subcommand list (from
the registry) and the global flags (from `flags.go`). The golden
file `internal/cli/testdata/help/root.golden.txt` is the record.

#### 4.9d Per-Command Help Content Contract

WBS 7.1.3 freezes the structure and content of every subcommand's
help output. The contract is enforced structurally (by tests that
read each command's fields) and by golden files (one per command).

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

#### 4.9e Examples Convention

WBS 7.2.2 defines when a command must declare an Example and what
the Example's content must look like.

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

Examples are stored in the `Example` field of the Cobra command,
not in the `Long` field.

#### 4.9f Invalid Command Behaviour

WBS 7.3.1 freezes the behaviour of seven invalid invocations.

##### The contract

| Aspect | Frozen behaviour |
|--------|------------------|
| Exit code | `ExitUsage` (2) |
| Output stream | stderr (only) |
| stdout | Empty |
| Error format | `Error: unknown command "<input>" for "forge"` |
| Suggestion line (no match) | `Run 'forge --help' for usage.` |
| Suggestion line (match) | `Did you mean "<name>"? Run 'forge --help' for usage.` |
| Trailing newline | Yes (exactly one) |
| Colour | None |
| Timestamp | None |
| Stack trace | Never |

##### The cases

| # | Input | Exit | stderr contains |
|---|-------|------|-----------------|
| I1 | `forge foobar` | 2 | `unknown command "foobar"` |
| I2 | `forge verison` | 2 | `Did you mean` and `version` |
| I3 | `forge version extra` | 2 | `unknown command "extra"` |
| I4 | `forge --unknown-flag` | 2 | `unknown flag: --unknown-flag` |
| I5 | `forge version --unknown-flag` | 2 | `unknown flag: --unknown-flag` |
| I6 | `forge help unknown` | 2 | a diagnostic naming the unknown topic |
| I7 | `forge --help unknown` | 2 | a diagnostic about the help flag |

##### Suggestions

Cobra's suggestion mechanism fires when an unknown command's name
is within edit distance 2 of a known command. The value is
configured by `SuggestionsMinimumDistance` on the root command.

#### 4.9g Error Message Format

WBS 7.3.2 freezes the format of every error message the CLI
renders.

##### The frozen format

```text
Error: <message>
       <optional context line, indented 7 spaces>
       <optional context line, indented 7 spaces>

Suggestion:
  <actionable remediation>
```

##### Rules

1. The first line starts with `Error:`.
2. The message is a single line, 80 runes or fewer.
3. The message does not end with a period.
4. Context lines are indented 7 spaces.
5. A blank line separates the message block from the Suggestion
   block.
6. The `Suggestion:` label is on its own line; the body is
   indented 2 spaces.
7. The suggestion is imperative.
8. No stack traces unless `--verbose` is set (Phase 6+).
9. No timestamps.
10. No colour by default.

##### Examples

Minimal error:

```text
Error: unknown command "foobar" for "forge"

Suggestion:
  Run 'forge --help' for usage.
```

Typo error:

```text
Error: unknown command "verison" for "forge"

Suggestion:
  Did you mean "version"? Run 'forge --help' for usage.
```

##### Phase 2 scope

The format is the Phase 2 shape. Richer errors (with structured
fields, error codes, file/line information) arrive with WBS 10.0.

#### 4.9h Output Stream Boundary

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

Every author of a command decides which stream a line belongs on
by asking a single question:

> If the user pipes this command's output to another program,
> should that program receive this line?

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

#### 4.9i Global Flags Interaction Matrix

WBS 7.4.3 freezes how `--quiet`, `--verbose`, and `--config`
interact with help output, version output, and error output.

##### The matrix

| Invocation | Behaviour |
|------------|-----------|
| `forge --quiet --help` | Help still printed to stdout |
| `forge --verbose --help` | Help printed to stdout; verbose adds nothing |
| `forge --quiet version` | Version printed to stdout |
| `forge --quiet unknown-cmd` | Error still printed to stderr |
| `forge --verbose unknown-cmd` | Error printed to stderr; stack trace in Phase 6+ |
| `forge --quiet --verbose version` | `--quiet` wins; version still prints |
| `forge --config nonexistent.yml version` | Version prints (config is a warning in Phase 2) |
| `forge --config nonexistent.yml check` | Config is an error in future phases |

##### The rules

1. `--quiet` suppresses warnings and info logs; it never
   suppresses errors.
2. `--quiet` never suppresses stdout results.
3. `--verbose` never adds to stdout.
4. Errors are always printed to stderr.
5. `--quiet` wins over `--verbose` when both are set.
6. Help and version are always printed to stdout.
7. A config load failure in Phase 2 is a warning, not a fatal
   error.

##### Phase 2 scope

Phase 2 implements the log-level effects of `--quiet` and
`--verbose`. Phase 2 does not implement a stack-trace mode for
`--verbose`; the row is aspirational. Phase 2 does not implement
the config subsystem; the config row documents the intended future
behaviour.

#### 4.9j Colour and Terminal Policy

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

Every byte the CLI writes to stdout or stderr is in the ASCII
printable range (0x20 through 0x7e) or is one of two permitted
control bytes: newline (0x0a) and tab (0x09).

Bytes outside this set are forbidden.

##### Where the rule is enforced

`internal/cli/ascii_policy_test.go` iterates the registry's
commands, invokes each in a set of representative configurations,
and asserts the policy on the captured stdout and stderr.

##### Future-proofing: when colour is introduced

When colour is introduced (Phase 6+), the following rules apply:

1. Colour is disabled when `NO_COLOR` is set (per no-color.org).
2. Colour is disabled when stdout is not a TTY.
3. Colour is disabled when `--no-color` is passed.
4. Colour never conveys semantic information alone.

#### 4.9k Root Help Contains No Non-ASCII

The root help output contains no non-ASCII characters. The
`RootLongDesc` and `RootShortDesc` constants are ASCII-only; the
subcommand help output that inherits from them is ASCII-only. The
ASCII-only policy is documented in § 4.9j.

#### 4.9l Colour and Terminal Policy (see § 4.9j)

See § 4.9j for the policy. This subsection is a stub for
cross-references.

#### 4.9m Summary of Contracts

| Contract | Source WBS | Section | Enforcement |
|----------|-----------|---------|-------------|
| Help behaviour | WBS 5.2.2 | § 4.9a | `errors_test.go` |
| Help invocation matrix | WBS 7.1.1 | § 4.9b | `help_matrix_test.go` |
| Root help content | WBS 7.1.2 | § 4.9c | `help_golden_test.go` |
| Per-command help content | WBS 7.1.3 | § 4.9d | `help_content_test.go`, `help_golden_test.go` |
| Examples convention | WBS 7.2.2 | § 4.9e | `examples_test.go` |
| Invalid command | WBS 7.3.1 | § 4.9f | `errors_test.go` |
| Error message format | WBS 7.3.2 | § 4.9g | `format_error_test.go`, `errors_test.go` |
| Output stream boundary | WBS 7.4.1 | § 4.9h | `stream_boundary_test.go` |
| Global flag interaction | WBS 7.4.3 | § 4.9i | `flag_interaction_test.go` |
| Colour / ASCII policy | WBS 7.4.2 | § 4.9j | `ascii_policy_test.go` |

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
   text output.**
2. **`-v` is short for `--version`.**
3. **`-v` is not short for `--verbose`.**
4. **The `--version` flag always produces the text format.**
5. **Version output goes to stdout and exits 0.**
6. **Extra arguments to a version flag are rejected.**
7. **The version formats are frozen in § 4.8.**

### 4.11 Global Flags

Forge has exactly three global flags in Phase 2.

#### The inventory

| Flag | Short | Type | Persistent | Consumer | Semantics |
|------|-------|------|------------|----------|-----------|
| `--verbose` | - | bool | yes | WBS 12.0 | Set log level to DEBUG |
| `--quiet` | - | bool | yes | WBS 12.0 | Set log level to ERROR |
| `--config` | - | string | yes | WBS 8.0 | Path to config file |

#### Precedence

When `--verbose` and `--quiet` are both set, `--quiet` wins. The
effective log level is ERROR. No warning is emitted.

#### Persistence

All three flags are persistent. `forge --verbose config` and
`forge config --verbose` are equivalent.

#### The `--config` rejection

`forge --config` with no following value is a usage error.

#### The `--format` flag is not global

`--format` is a per-command flag. See § 4.12.

#### Adding a global flag

Adding a global flag requires an ADR.

### 4.12 `--format` Flag Semantics

Commands that produce structured output accept a per-command
`--format` flag.

#### The value sets

| Command | Value set | Default | Introduced by |
|---------|-----------|---------|---------------|
| `forge version` | `text`, `json` | `text` | WBS 6.4.2 |
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

Every command that accepts `--format` must define its value set,
reject unsupported values with exit code 2, write output to stdout,
and ensure that JSON output is a single line terminated by `\n`.

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
invalid flag value (including unsupported `--format` value),
malformed `forge.yaml` syntax, ambiguous foundation selection, and
malformed invocations rejected by `validateArgs`.

#### 3 - Configuration failure

#### 4 - Filesystem failure

#### 5 - Validation failure

#### 6 - Security violation

#### 7 - Update conflict

### 7.3 Exit Code Stability

These codes are part of Forge's public contract. Changing them
requires an ADR.

### 7.4 The mapping

| Error condition | Exit code |
|-----------------|-----------|
| No error | `ExitSuccess` |
| Error with `Category() == "config"` | `ExitConfig` |
| Error with `Category() == "filesystem"` | `ExitFilesystem` |
| Error with `Category() == "validation"` | `ExitValidation` |
| Error with `Category() == "security"` | `ExitSecurity` |
| Error with `Category() == "conflict"` | `ExitConflict` |
| Error with an unrecognised category | `ExitFailure` |
| Error without a category | `ExitUsage` |

### 7.5 Scripting

### 7.6 Reserved integers

Integers 8 and above are reserved for future categories.

---

## 8. Machine-Readable Output

### 8.1 Schema Versioning

JSON output from `forge new`, `forge init`, `forge validate`,
`forge explain`, and `forge check` includes a `schemaVersion`
field.

**Exception: `forge version --format json`.** The `forge version`
JSON output does not include a `schemaVersion` field.

### 8.2 Output Streams

- **stdout** - successful command output
- **stderr** - errors, warnings, diagnostics

The `forge version --format json` command writes errors to stderr,
not to stdout as JSON.

### 8.3 JSON Stability Contract

### 8.4 Format Values

| Value | Meaning | Commands |
|-------|---------|----------|
| `text` | Machine-friendly text | `forge version` |
| `human` | Default human-readable output | Creation and inspection commands |
| `json` | Machine-readable JSON | All commands that support `--format` |
| `sarif` | SARIF | `forge check` (Phase 10+) |

### 8.5 JSON Output is a Single Line

Every command that emits JSON emits it as a single line terminated
by `\n`.

---

## 9. CLI UX Principles

### 9.1 Predictable Commands

### 9.2 Safe Defaults

### 9.3 Actionable Errors

### 9.4 No Destructive Operations Without Confirmation

### 9.5 Scriptability

### 9.6 Human-Readable Output

Default output is optimised for humans. In Phase 2, the symbols are
ASCII-safe (see § 4.9j). The vocabulary is:

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
- Phase 2 emits no colour. See § 4.9j.
- When colour is introduced (Phase 6+), the rules in § 4.9j apply.

### 9.12 Accessibility

- Output is readable in monochrome terminals.
- Phase 2 output is ASCII-only. See § 4.9j.
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

`forge version` is always non-interactive.

---

## 11. Error Catalogue

Every error returned by Forge has a stable code.

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

---

## 12. Open Questions

The following questions remain open:

- Should `forge validate` and `forge check` be merged, or remain
  separate commands?
- Should `forge explain` support subcommands in the MVP, or defer?
- Should `forge new` write the `forge.yaml` file before or after
  generating the project files?
- Should the default output for successful operations include a
  `Next steps` block, or should that be deferred to `--verbose`?
- Should `forge version` include template versions in its output?
- What is the exact behaviour when `forge new` is given a project
  name with a `/` in it?
- Should `forge init` detect and refuse to run if the repository
  has any uncommitted Git changes?
- Should `forge template list` distinguish between local and
  remote templates once a registry exists?
- Should the JSON format of `forge version` gain a `schemaVersion`
  field once a second JSON-emitting mode is added?

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
| 0.6.0 | 2026-10-10 | @thapelomagqazana | Added § 4.9b through § 4.9m in response to WBS 7.1.1, WBS 7.1.2, WBS 7.1.3, WBS 7.2.1, WBS 7.2.2, WBS 7.3.1, WBS 7.3.2, WBS 7.4.1, WBS 7.4.2, and WBS 7.4.3. The new subsections cover: the help invocation matrix (§ 4.9b), the root help content contract (§ 4.9c), the per-command help content contract (§ 4.9d), the Examples convention (§ 4.9e), the invalid-command contract (§ 4.9f), the error message format (§ 4.9g), the output stream boundary (§ 4.9h), the global flag interaction matrix (§ 4.9i), the colour and terminal policy (§ 4.9j), and a summary of contracts (§ 4.9m). Updated § 4.7 to reflect the ASCII-only constants (`RootShortDesc` uses `-`; `RootLongDesc` uses `->`). Updated § 9.6 and § 9.8 to use ASCII-safe markers (`[OK]`, `[WARN]`, `[ERROR]`, `[SKIP]`, `[!]`). Updated § 3.1 and § 4.1's interactive wizard example to use ASCII-safe markers. |
