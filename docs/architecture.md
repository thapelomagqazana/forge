# Technical Architecture

- **Document type:** Model
- **Status:** Draft
- **Version:** 0.5.0
- **Author:** @thapelomagqazana
- **Created:** 2026-10-09
- **Last Updated:** 2026-10-10
- **Supersedes:** 0.4.0
- **Superseded by:** —

---

## 1. Purpose

This document defines the internal technical architecture of Forge.
It turns the Phase 1 specifications into a coherent module structure,
dependency direction, interface contract, data flow, error model,
logging model, and execution boundary that Phase 2 can implement
against.

This document exists to answer:

- What are the top-level modules of Forge?
- Which direction do dependencies flow?
- What interfaces connect modules?
- What is the end-to-end data flow?
- How is the process boundary made testable?
- How are the handler and application logic separated?
- How is the command tree assembled?
- How are global flags managed?
- How are malformed invocations rejected before the command tree runs?
- How are errors structured?
- How is logging structured?
- What does the architecture look like as a diagram?

This is not a specification of behaviour. Behaviour is defined in the
other specification documents. This document defines **structure**.

---

## 2. Scope

**In scope:**

- Top-level modules
- Dependency direction rules
- Interface contracts between modules
- End-to-end data flow
- The two-boundary execution model
- The handler / service boundary
- Command registration
- Global flags
- Pre-parse argument validation
- Error architecture
- Logging architecture
- Architecture diagram

**Out of scope:**

- Behaviour of individual commands (see
  [`docs/cli-ux-spec.md`](./cli-ux-spec.md))
- Schema definitions (see their respective `*-spec.md` documents)
- Security policies (see
  [`docs/security-model.md`](./security-model.md))
- Update algorithm (see
  [`docs/update-model.md`](./update-model.md))
- Implementation details (belongs in code)
- Package-level layout (belongs in Phase 2)

---

## 3. Architectural Principles

The Forge architecture rests on seven principles.

### 3.1 Clean Architecture

Dependencies point inward. The domain layer does not depend on
infrastructure. Infrastructure depends on the domain.

### 3.2 Single Responsibility

Each module has one responsibility. Modules do not overlap.

### 3.3 Explicit Interfaces

Modules communicate through interfaces, not concrete types. This
enables testing and future extension.

### 3.4 Pure Domain Logic

The domain layer contains pure logic. It does not perform I/O. This
makes it fully testable without filesystem or network access.

### 3.5 Side Effects at the Edges

I/O, filesystem writes, and process execution happen at the edges of
the system (infrastructure layer), not in the domain.

### 3.6 Fail-Fast

Errors are detected as early as possible. Malformed input is
rejected before it can cause downstream damage.

### 3.7 Observable

Every significant operation emits structured logs and produces
structured results that can be consumed by the CLI or by automation.

---

## 4. Module Overview

Forge is organised into the following top-level modules.

### 4.1 Module List

| Module | Layer | Responsibility |
|--------|-------|----------------|
| **CLI** | Application | Parse commands, present output, map results to exit codes |
| **Application** | Application | Orchestrate use cases; coordinate domain and infrastructure |
| **Domain** | Domain | Express the core concepts of Forge |
| **Configuration** | Domain | Load, validate, and represent configuration |
| **Blueprint** | Domain | Represent and validate Blueprints |
| **Template** | Domain | Represent and resolve templates |
| **Component** | Domain | Represent and resolve components |
| **Policy** | Domain | Represent and evaluate policies |
| **Renderer** | Application | Transform template content into rendered content |
| **Validator** | Application | Evaluate rules against repository state |
| **Update Engine** | Application | Compute and apply safe updates |
| **Version Service** | Application | Report build metadata for the running binary |
| **Filesystem** | Infrastructure | Provide safe, sandboxed filesystem access |
| **Process** | Infrastructure | Provide process execution where needed |
| **Registry** | Infrastructure | Fetch and verify remote artifacts (future) |
| **Logging** | Infrastructure | Provide structured logging |
| **Output** | Infrastructure | Format human and machine-readable output |

### 4.2 Module Descriptions

#### 4.2.1 CLI

**Responsibility:** Parse arguments, dispatch to Application, present
results, map to exit codes.

**Does not:** Contain business logic, read files, render templates.

**Depends on:** Application, Output, Logging.

The CLI module is composed of several files within
`internal/cli/`:

- `root.go` — the root command constructor.
- `registry.go` — the central command registry (§ 11.13).
- `flags.go` — the global flag registration and readers (§ 11.14).
- `validate.go` — the pre-parse argument validator (§ 11.15).
- `execute.go` — the process boundary and the transformation to
  `Dependencies` (§ 11).
- `deps.go` — the `Dependencies` struct (§ 11.2).
- `version.go` — the version handler (a subcommand, not part of the
  CLI's own architecture).
- `config.go` — the hidden placeholder for `forge config`.
- `exitcodes.go` — the exit code constants and the error-to-code
  mapping.
- `doc.go` — the package documentation.

#### 4.2.2 Application

**Responsibility:** Implement use cases (create project, validate,
check, diff, update, explain).

**Does not:** Contain domain logic (delegates to Domain), read files
(delegates to Infrastructure).

**Depends on:** Domain, Renderer, Validator, Update Engine,
Filesystem, Logging.

##### 4.2.2a Version Service — A Concrete Example

The `Version Service` submodule (`internal/app/version/`) is the
first implemented application service. It is deliberately simple:
it reads the build metadata (version, commit, build date) from
`internal/version` and formats it for a human reader.

It exists to demonstrate the handler / service pattern that every
future command follows. Its structure is documented in § 11.12.

**Responsibility:** Produce an `Info` value describing the running
binary, and render it to an `io.Writer`.

**Does not:** Parse flags, construct Cobra commands, read files, or
write to process-global streams.

**Depends on:** `internal/version` (for the build metadata). It does
not depend on `internal/cli` or `github.com/spf13/cobra`.

#### 4.2.3 Domain

**Responsibility:** Express Forge's core concepts and rules. Contains
pure logic.

**Does not:** Perform I/O, depend on any infrastructure.

**Depends on:** Nothing (pure).

**Submodules:**

- `Configuration`
- `Blueprint`
- `Template`
- `Component`
- `Policy`

#### 4.2.4 Configuration

**Responsibility:** Load, validate, and represent Forge configuration
(`forge.yaml`).

**Depends on:** Nothing (pure parsing after YAML decode).

#### 4.2.5 Blueprint

**Responsibility:** Represent Blueprints, apply defaults, validate
against schema and semantic rules.

**Depends on:** Nothing (pure).

#### 4.2.6 Template

**Responsibility:** Represent templates, resolve template references,
determine compatibility.

**Depends on:** Blueprint (read-only, for compatibility checks).

#### 4.2.7 Component

**Responsibility:** Represent components, resolve dependencies,
detect conflicts, compose contributions.

**Depends on:** Blueprint, Template (read-only).

#### 4.2.8 Policy

**Responsibility:** Represent policies, evaluate against repository
state, produce findings.

**Depends on:** Blueprint (read-only).

#### 4.2.9 Renderer

**Responsibility:** Transform template files into rendered content.

**Does not:** Write to disk (that is Filesystem).

**Depends on:** Domain, Filesystem (read-only for template sources).

#### 4.2.10 Validator

**Responsibility:** Evaluate validation rules against repository
state. Produces findings.

**Depends on:** Domain, Filesystem (read-only).

#### 4.2.11 Update Engine

**Responsibility:** Compute update plans, perform three-way merge,
detect conflicts, apply updates atomically.

**Depends on:** Domain, Renderer, Filesystem.

#### 4.2.12 Filesystem

**Responsibility:** Provide safe, sandboxed filesystem access.
Enforce boundary rules, path resolution, atomic writes.

**Depends on:** Security rules (from Domain).

#### 4.2.13 Process

**Responsibility:** Execute external processes where explicitly
required (e.g., Git operations).

**Depends on:** Nothing (wraps OS APIs).

#### 4.2.14 Registry

**Responsibility:** Fetch, verify, and cache remote artifacts.

**Introduced in:** Phase 16.

**Depends on:** Filesystem, Security rules.

#### 4.2.15 Logging

**Responsibility:** Emit structured log messages at defined levels.

**Depends on:** Nothing.

#### 4.2.16 Output

**Responsibility:** Format results for human and machine consumers.

**Depends on:** Domain (for result types).

---

## 5. Dependency Direction

### 5.1 The Rule

> **Dependencies point inward.**

```text
┌─────────────────────────────────────────────────────────┐
│                                                         │
│  ┌───────────────────────────────────────────────┐      │
│  │  Application Layer                            │      │
│  │  ┌───────────────────────────────────────┐    │      │
│  │  │  Domain Layer                         │    │      │
│  │  │                                       │    │      │
│  │  │  Configuration  Blueprint  Template   │    │      │
│  │  │  Component      Policy                │    │      │
│  │  │                                       │    │      │
│  │  └───────────────────────────────────────┘    │      │
│  │                                               │      │
│  │  Application Services                         │      │
│  │  Renderer  Validator  Update Engine  Version  │      │
│  │                                               │      │
│  └───────────────────────────────────────────────┘      │
│                                                         │
│  ┌───────────────────────────────────────────────┐      │
│  │  Infrastructure Layer                         │      │
│  │  Filesystem  Process  Registry  Logging       │      │
│  │  Output                                       │      │
│  └───────────────────────────────────────────────┘      │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 5.2 Allowed Dependencies

| From | To | Allowed |
|------|----|---------|
| CLI | Application | Yes |
| CLI | Infrastructure (Output, Logging) | Yes |
| Application | Domain | Yes |
| Application | Infrastructure | Yes |
| Domain | Domain (same layer) | Yes, within submodules |
| Infrastructure | Domain | Yes (for shared types) |
| Any | CLI | **No** |
| Domain | Application | **No** |
| Domain | Infrastructure (I/O) | **No** |
| Infrastructure | CLI | **No** |

### 5.3 Forbidden Dependencies

The following are explicitly forbidden:

- **CLI directly reading files.** The CLI must call Application
  services, which call Infrastructure.
- **CLI directly parsing YAML.** The CLI must not depend on the YAML
  parser.
- **Domain reading files.** Domain logic is pure; it receives data,
  never fetches it.
- **Domain calling external processes.** Domain logic is pure.
- **Infrastructure depending on CLI.** This would invert the
  dependency direction.
- **Any package outside `internal/cli` importing Cobra.** The CLI
  framework is a CLI-layer concern. See § 11.12.

### 5.4 Rationale

The dependency rule ensures:

- **Testability:** Domain logic can be tested without filesystem or
  network.
- **Replaceability:** The filesystem, YAML parser, or CLI framework
  can be changed without touching domain logic.
- **Safety:** I/O is centralised in the Infrastructure layer, where
  security policies are enforced.
- **Clarity:** Every module's role is unambiguous.

### 5.5 Enforcement

The dependency direction is enforced by:

- Code review
- Package structure (`internal/` subdirectories)
- Automated tests that check import graphs (Phase 2)
- Linting rules (Phase 2)

---

## 6. Interface Contracts

Interfaces are the contract between modules. They are defined in the
Domain or Application layer, and implemented by Infrastructure.

### 6.1 Filesystem

```go
type Filesystem interface {
    // Read returns the content of a file within the boundary.
    Read(path string) ([]byte, error)

    // Write writes content to a file within the boundary.
    Write(path string, content []byte, mode fs.FileMode) error

    // Exists reports whether a path exists within the boundary.
    Exists(path string) (bool, error)

    // MkdirAll creates a directory and any parents within the boundary.
    MkdirAll(path string, mode fs.FileMode) error

    // Remove deletes a file within the boundary.
    Remove(path string) error

    // List returns the entries in a directory within the boundary.
    List(path string) ([]fs.DirEntry, error)

    // Stat returns file metadata within the boundary.
    Stat(path string) (fs.FileInfo, error)

    // Boundary returns the target directory.
    Boundary() string
}
```

**Responsibilities:**

- Enforce boundary rules (no path escapes)
- Resolve symlinks safely
- Provide atomic writes where practical
- Return structured errors

**Does not:**

- Interpret file contents
- Apply business logic

### 6.2 ConfigurationLoader

```go
type ConfigurationLoader interface {
    // Load reads and parses forge.yaml from the given path.
    Load(path string) (*Configuration, error)
}
```

**Responsibilities:**

- Read `forge.yaml`
- Parse YAML
- Validate schema
- Apply defaults
- Return structured errors

### 6.3 BlueprintLoader

```go
type BlueprintLoader interface {
    // Load reads and parses a Blueprint.
    Load(id string) (*Blueprint, error)

    // LoadFromFile reads and parses a Blueprint from a file.
    LoadFromFile(path string) (*Blueprint, error)
}
```

### 6.4 TemplateResolver

```go
type TemplateResolver interface {
    // Resolve returns the template matching the Blueprint.
    Resolve(bp *Blueprint) (*Template, error)

    // ResolveByID returns a template by identifier.
    ResolveByID(id string) (*Template, error)
}
```

### 6.5 ComponentResolver

```go
type ComponentResolver interface {
    // Resolve returns components that satisfy the Blueprint.
    Resolve(bp *Blueprint) ([]*Component, error)

    // ResolveDependencies returns the full dependency closure.
    ResolveDependencies(c *Component) ([]*Component, error)
}
```

### 6.6 Renderer

```go
type Renderer interface {
    // Render produces the content of a file from a template.
    Render(
        file *TemplateFile,
        ctx *RenderContext,
    ) ([]byte, error)

    // Plan produces the full render plan for a template.
    Plan(
        tpl *Template,
        ctx *RenderContext,
    ) (*RenderPlan, error)
}
```

### 6.7 Validator

```go
type Validator interface {
    // Validate evaluates all rules against the repository state.
    Validate(
        ctx context.Context,
        repo *Repository,
        rules []Rule,
    ) (*ValidationResult, error)
}
```

### 6.8 UpdateEngine

```go
type UpdateEngine interface {
    // Plan computes an update plan without modifying the repository.
    Plan(
        ctx context.Context,
        repo *Repository,
        target *Foundation,
    ) (*UpdatePlan, error)

    // Apply executes an update plan.
    Apply(
        ctx context.Context,
        repo *Repository,
        plan *UpdatePlan,
    ) (*UpdateResult, error)

    // Rollback reverses the most recent update.
    Rollback(
        ctx context.Context,
        repo *Repository,
    ) error
}
```

### 6.9 StateStore

```go
type StateStore interface {
    // Load reads the Forge state.
    Load(root string) (*State, error)

    // Save writes the Forge state.
    Save(root string, s *State) error

    // Backup creates a backup of the current state and files.
    Backup(root string) (*Backup, error)
}
```

### 6.10 Logger

```go
type Logger interface {
    // Log emits a structured log message at the given level.
    Log(level Level, msg string, fields ...Field)

    // With returns a logger with additional fields.
    With(fields ...Field) Logger
}
```

### 6.11 Output

```go
type Output interface {
    // WriteHuman writes a human-readable result.
    WriteHuman(w io.Writer, r Result) error

    // WriteJSON writes a machine-readable result.
    WriteJSON(w io.Writer, r Result) error
}
```

### 6.12 Version Service (informal)

The version service does not define an interface. It is a pure
function with a value type:

```go
// In internal/app/version:
type Info struct {
    Version   string
    Commit    string
    BuildDate string
    Dirty     string
}

func Get() Info
func Format(w io.Writer, info Info) error
func Raw() string
```

**Responsibilities:**

- Produce an `Info` value describing the running binary.
- Render the value to an `io.Writer` (via `Format`).
- Produce the value as a string (via `Raw`).

**Does not:**

- Parse flags.
- Construct Cobra commands.
- Read files.
- Write to process-global streams.
- Import `internal/cli`.

The service is documented in § 11.12 as the reference implementation
of the handler / service pattern.

---

## 7. Data Flow

### 7.1 End-to-End Flow

Every Forge command follows the same high-level flow:

```text
┌──────────────────┐
│  CLI input       │  User runs `forge <command>`
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Pre-parse       │  validateArgs rejects malformed invocations
│  validation      │  (§ 11.15)
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Configuration   │  Load forge.yaml (if present)
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Blueprint       │  Load or construct Blueprint
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Component       │  Resolve components from Blueprint
│  resolution      │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Template        │  Resolve template from Blueprint
│  resolution      │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Validation      │  Validate all inputs
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Rendering       │  Render template files
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Filesystem      │  Write files (if mutating)
└──────────────────┘
```

The pre-parse validation stage is described in § 11.15. It is a
prerequisite for every other stage: an invocation that fails the
validation never reaches the command tree.

### 7.2 Flow Variations by Command

| Command | Skips |
|---------|-------|
| `forge validate` | Rendering, Filesystem (write) |
| `forge check` | Rendering, Filesystem (write) |
| `forge diff` | Filesystem (write) |
| `forge explain` | Rendering, Filesystem (write) |
| `forge version` | All of the above; writes fixed metadata to stdout |
| `forge new` | Configuration (creates new) |
| `forge init` | Rendering (only writes forge.yaml) |
| `forge update` | Configuration (reads existing) |

### 7.3 Data Flow Invariants

Every flow must satisfy:

- **Pure domain:** Domain logic receives data; it never fetches it.
- **Boundary enforcement:** Filesystem access is always through the
  Filesystem interface, which enforces boundaries.
- **Determinism:** Given the same inputs, the flow produces the same
  outputs.
- **Observability:** Every step logs its progress at debug level.

### 7.4 Specific Command Flows

#### 7.4.1 `forge new`

```text
CLI → parse args
  → Application.NewProject(args)
    → Configuration.BuildFromArgs(args)
    → Blueprint.FromConfiguration(config)
    → Blueprint.Validate()
    → ComponentResolver.Resolve(blueprint)
    → ComponentResolver.ResolveDependencies(components)
    → TemplateResolver.Resolve(blueprint)
    → Validator.Validate(blueprint, template, components)
    → Renderer.Plan(template, context)
    → Filesystem.MkdirAll(target)
    → For each file in plan:
        → Renderer.Render(file)
        → Filesystem.Write(path, content)
    → StateStore.Save(state)
  → Output.WriteHuman(result)
  → Exit code 0
```

#### 7.4.2 `forge init`

```text
CLI → parse args
  → Application.InitProject(args)
    → Filesystem.List(".") → detect technologies
    → Repository.Detect() → technology profile
    → Blueprint.Recommend(profile) → candidate blueprints
    → (interactive) prompt for confirmation
    → Blueprint.FromSelection(selection)
    → Blueprint.Validate()
    → Configuration.BuildFromBlueprint(blueprint)
    → Filesystem.Write("forge.yaml", content)
  → Output.WriteHuman(result)
  → Exit code 0
```

#### 7.4.3 `forge update`

```text
CLI → parse args
  → Application.UpdateProject(args)
    → Configuration.Load("forge.yaml")
    → Blueprint.Load(config.blueprint)
    → StateStore.Load(".forge/state.yaml")
    → UpdateEngine.Plan(repo, target)
      → For each file:
        → compute BASE, CURRENT, TARGET
        → classify by change-tracking
        → if BOTH_MODIFIED, three-way merge
        → collect plan or conflict
    → (if dry-run) Output.WriteHuman(plan)
    → (if interactive) prompt for conflict resolution
    → StateStore.Backup(root)
    → For each change:
      → Renderer.Render or apply merge
      → Filesystem.Write
    → Validator.Validate(repo)
    → (if validation fails) UpdateEngine.Rollback()
    → StateStore.Save(state)
  → Output.WriteHuman(result)
  → Exit code 0 (or 5 for conflicts)
```

#### 7.4.4 `forge version`

```text
CLI → newVersionCmd(deps).RunE
  → version.Get()      (pure; reads internal/version)
  → version.Format(deps.Stdout, info)
  → Exit code 0 (or 1 on write failure)
```

The version command is the reference implementation of the handler /
service pattern (§ 11.12). It has no configuration, no blueprint, no
component resolution, no validation, no rendering, and no filesystem
write. Its flow is two lines: a pure call, and a formatted write to
the injected stdout.

---

## 8. Error Architecture

### 8.1 Principles

Errors in Forge are:

- **Structured:** Every error has a code, message, cause, and context
- **Actionable:** Every error includes a suggestion
- **Traceable:** Every error includes enough context to reproduce
- **Safe:** Errors do not leak secrets

### 8.2 Error Type

Every Forge error conforms to:

```go
type ForgeError struct {
    Code        ErrorCode  // Stable identifier (e.g., "FORGE_CONFIG_INVALID")
    Message     string     // Human-readable description
    Cause       error      // Underlying error (may be nil)
    Context     ErrorContext // Additional structured context
    Suggestion  string     // What the user can do
    ExitCode    int        // Process exit code
}
```

### 8.3 Error Codes

Error codes follow the pattern:

```text
FORGE_<CATEGORY>_<SPECIFIC>
```

Categories:

| Category | Meaning |
|----------|---------|
| `USAGE` | Invalid command or flags |
| `INPUT` | Invalid user input |
| `CONFIG` | Configuration file issue |
| `BLUEPRINT` | Blueprint issue |
| `TEMPLATE` | Template issue |
| `COMPONENT` | Component issue |
| `VALIDATION` | Validation failure |
| `FILESYSTEM` | Filesystem failure |
| `SECURITY` | Security violation |
| `UPDATE` | Update failure or conflict |
| `NETWORK` | Network failure |
| `INTERNAL` | Internal error |

### 8.4 Error Construction

Every error is constructed with:

- The specific code
- A message in the developer's language
- The cause (for wrapping)
- Context (structured data)
- A suggestion (what to do)

### 8.5 Error Wrapping

Forge uses Go's error wrapping:

```go
if err := fs.Write(path, content); err != nil {
    return NewForgeError(
        CodeFilesystemWriteFailed,
        "Failed to write file",
        err, // wrapped cause
        Context{"path": path},
        "Check that the path is writable.",
    )
}
```

The underlying error is preserved for debugging (`--verbose`).

### 8.6 Error Presentation

Errors are presented differently depending on context:

**Human output:**

```text
✗ Failed to write file

File: src/main.py
Reason: permission denied

Suggestion:
  Check that you have write permission for this directory.
```

**JSON output:**

```json
{
  "status": "error",
  "error": {
    "code": "FORGE_FILESYSTEM_WRITE_FAILED",
    "message": "Failed to write file",
    "context": {
      "path": "src/main.py"
    },
    "cause": "permission denied",
    "suggestion": "Check that you have write permission for this directory."
  }
}
```

### 8.7 Exit Code Mapping

| Error category | Exit code |
|----------------|-----------|
| Success | 0 |
| Validation failure | 1 |
| Usage / input error | 2 |
| Filesystem failure | 3 |
| Security violation | 4 |
| Update conflict | 5 |
| Internal error | 1 (default) |

The full exit code contract is defined in
[`docs/cli-ux-spec.md`](./cli-ux-spec.md) § 7.

### 8.8 Secret Redaction

Errors never include secret values. If an error's context contains a
value matching a secret pattern (e.g., API key format), the value is
replaced with `<redacted>`.

---

## 9. Logging Architecture

### 9.1 Log Levels

Forge supports four log levels:

| Level | Purpose |
|-------|---------|
| `ERROR` | Failures requiring attention |
| `WARN` | Potential issues |
| `INFO` | High-level progress |
| `DEBUG` | Detailed diagnostic information |

### 9.2 Default Level

The default level is `INFO`.

- `--quiet` suppresses `INFO` and `WARN`
- `--verbose` enables `DEBUG`
- `--debug` enables `DEBUG` with additional internal details

### 9.3 Log Destinations

| Destination | Content |
|-------------|---------|
| **stdout** | Command output (human or JSON) |
| **stderr** | Logs, warnings, errors |
| **file** | Optional (`--log-file`) |

By default, logs go to stderr so that stdout remains clean for
piping.

### 9.4 Log Format

**Human format (default):**

```text
[INFO]  Loading forge.yaml
[INFO]  Validating blueprint
[DEBUG] Resolving template python-fastapi@1.0.0
[DEBUG] Rendered 12 files
[INFO]  Project created successfully
```

**JSON format (`--log-format json`):**

```json
{"level":"info","msg":"Loading forge.yaml","time":"2026-10-09T12:00:00Z"}
{"level":"info","msg":"Validating blueprint","time":"2026-10-09T12:00:01Z"}
{"level":"debug","msg":"Resolving template","template":"python-fastapi@1.0.0","time":"2026-10-09T12:00:02Z"}
```

### 9.5 Structured Fields

Every log message includes structured fields:

| Field | Purpose |
|-------|---------|
| `command` | The command being executed |
| `path` | File path (if relevant) |
| `template` | Template ID (if relevant) |
| `component` | Component ID (if relevant) |
| `duration` | Time taken (for completed operations) |

### 9.6 Redaction

Log messages never include:

- Secret values
- API keys
- Passwords
- Private keys
- Environment variable values

If a value matches a secret pattern, it is redacted:

```text
[INFO] Connecting to database
[DEBUG] Connection string: postgres://user:<redacted>@host/db
```

### 9.7 Determinism

Log output is deterministic where practical. Timestamps are the only
exception (they reflect actual time).

### 9.8 Log Levels by Verbosity

| Flag | Level |
|------|-------|
| (default) | `INFO` |
| `--quiet` | `ERROR` |
| `--verbose` | `DEBUG` |
| `--debug` | `DEBUG` (with internal details) |

The `--verbose` and `--quiet` flags are two of the three global flags
(§ 11.14). When both are set, `--quiet` wins.

### 9.9 Logging in CI

When running in CI (`CI=true` environment variable):

- Default log format is `JSON`
- Default level is `INFO`
- No colour is emitted
- Progress indicators are disabled

---

## 10. Architecture Diagram

```text
┌───────────────────────────────────────────────────────────────────┐
│                          USER / CI                                │
└───────────────────────────────┬───────────────────────────────────┘
                                │
                                ▼
┌───────────────────────────────────────────────────────────────────┐
│                          CLI LAYER                                │
│                                                                   │
│  ┌───────────────┐   Pre-parse validation (§ 11.15)               │
│  │ validateArgs  │   Rejects malformed invocations                │
│  └───────┬───────┘                                                │
│          │                                                        │
│          ▼                                                        │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐       │
│  │ new       │  │ init      │  │ validate  │  │ update    │  ...  │
│  └─────┬─────┘  └─────┬─────┘  └─────┬─────┘  └─────┬─────┘       │
│        │              │              │              │             │
│        └──────────────┴──────────────┴──────────────┘             │
│                              │                                    │
└──────────────────────────────┼────────────────────────────────────┘
                               │
                               ▼
┌───────────────────────────────────────────────────────────────────┐
│                       APPLICATION LAYER                           │
│                                                                   │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐   │
│  │ Renderer   │  │ Validator  │  │ Update     │  │ Explain    │   │
│  │            │  │            │  │ Engine     │  │ Service    │   │
│  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘   │
│        │               │               │               │          │
│  ┌─────┴──────┐                                                   │
│  │ Version    │                                                   │
│  │ Service    │                                                   │
│  └────────────┘                                                   │
│                                                                   │
└────────┼───────────────┼───────────────┼───────────────┼──────────┘
         │               │               │               │
         └───────────────┴───────────────┴───────────────┘
                         │
                         ▼
┌───────────────────────────────────────────────────────────────────┐
│                          DOMAIN LAYER                             │
│                                                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐           │
│  │Blueprint │  │Template  │  │Component │  │Policy    │           │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘           │
│                                                                   │
│  ┌──────────────┐                                                 │
│  │Configuration │                                                 │
│  └──────────────┘                                                 │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘
                         ▲
                         │ (implements interfaces)
                         │
┌───────────────────────────────────────────────────────────────────┐
│                     INFRASTRUCTURE LAYER                          │
│                                                                   │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐   │
│  │ Filesystem │  │ Process    │  │ Registry   │  │ Logging    │   │
│  └────────────┘  └────────────┘  └────────────┘  └────────────┘   │
│                                                                   │
│  ┌────────────┐                                                   │
│  │ Output     │                                                   │
│  └────────────┘                                                   │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘
```

### 10.1 Reading the Diagram

- **Arrows point downward** — dependencies flow from CLI → Application
  → Domain. Infrastructure is below Domain and provides
  implementations.
- **Domain is at the center** — it has no dependencies on other
  layers.
- **Infrastructure implements Domain interfaces** — the arrow from
  Infrastructure to Domain is an implementation arrow, not a
  dependency arrow.
- **CLI is at the top** — it depends on Application, not the other
  way around.
- **The Version Service is an Application-layer service** — it is
  listed alongside Renderer, Validator, and Update Engine because it
  has the same architectural role: it orchestrates a use case on
  behalf of a CLI handler.
- **Pre-parse validation is the first stage** — the CLI's pipeline
  begins with `validateArgs`, which rejects malformed invocations
  before the command tree runs. The stage is documented in § 11.15.

### 10.2 Alternative View: By Concern

```text
┌───────────────────────────────────────────────────────────────────┐
│                                                                   │
│  USER-FACING                                                      │
│  ├── CLI commands                                                 │
│  ├── Human output                                                 │
│  └── Machine output (JSON, SARIF)                                 │
│                                                                   │
├───────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ORCHESTRATION                                                    │
│  ├── Application services (New, Init, Validate, Check, Diff,      │
│  │   Update, Explain, Version)                                    │
│  ├── Renderer                                                     │
│  ├── Validator                                                    │
│  └── Update Engine                                                │
│                                                                   │
├───────────────────────────────────────────────────────────────────┤
│                                                                   │
│  CORE LOGIC                                                       │
│  ├── Blueprint (parse, validate, default)                         │
│  ├── Template (parse, resolve, plan)                              │
│  ├── Component (resolve, compose, conflict)                       │
│  ├── Policy (parse, evaluate)                                     │
│  └── Configuration (load, merge)                                  │
│                                                                   │
├───────────────────────────────────────────────────────────────────┤
│                                                                   │
│  FOUNDATIONS                                                      │
│  ├── Filesystem (boundary, atomic writes)                         │
│  ├── Process (Git, subprocess)                                    │
│  ├── Registry (fetch, verify)                                     │
│  ├── Logging                                                      │
│  ├── Output (formatting)                                          │
│  └── Version (build metadata)                                     │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘
```

---

## 11. The Two-Boundary Execution Model

### 11.1 Overview

The Forge CLI is structured around **two boundaries** and **two
structs**, each serving a distinct purpose:

| Layer | Type | Visibility | Purpose |
|-------|------|------------|---------|
| Public entry point | `Execute() int` | Exported | Contract with `cmd/forge/main.go` |
| Injectable implementation | `executeWithOptions(opts options) int` | Unexported | Contract with tests |
| Process boundary | `options` | Unexported | Raw `os.*` values |
| Command boundary | `Dependencies` | Unexported | Resolved collaborators |

The two functions live in
[`internal/cli/execute.go`](../internal/cli/execute.go). The two
structs live in `execute.go` and
[`internal/cli/deps.go`](../internal/cli/deps.go) respectively.

The public entry point is a one-line wrapper:

```go
func Execute() int {
    return executeWithOptions(defaultOptions())
}
```

Every behaviour a user can observe is implemented in
`executeWithOptions` or below it. `Execute` exists only to establish
the contract with the process entry point.

### 11.2 The Two Structs

Forge's injectable environment is described by two structs, one per
boundary.

#### 11.2.1 `options` — The Process Boundary

```go
type options struct {
    args     []string
    stdin    io.Reader
    stdout   io.Writer
    stderr   io.Writer
    env      func(string) string
    rootPath string
}
```

Each field captures one class of side effect the CLI may perform.

| Field | Purpose | Default source |
|-------|---------|----------------|
| `args` | Command-line arguments, excluding the program name. | `os.Args[1:]` |
| `stdin` | Reader for interactive input. | `os.Stdin` |
| `stdout` | Writer for successful output. | `os.Stdout` |
| `stderr` | Writer for diagnostics and errors. | `os.Stderr` |
| `env` | Environment variable lookup. | `os.Getenv` |
| `rootPath` | Directory for relative path resolution. | `os.Getwd` |

The struct is deliberately small: six fields, each a primitive or an
interface. No nested structs, no pointers to structs, no maps.

`options` is constructed by:

- `defaultOptions()` in production, which reads the process's actual
  `os.*` values.
- A test, which constructs a synthetic environment.

#### 11.2.2 `Dependencies` — The Command Boundary

```go
type Dependencies struct {
    Config *config.Config
    Logger Logger
    FS     filesystem.FS
    Stdout io.Writer
    Stderr io.Writer
    Env    func(string) string
}
```

Each field is a resolved collaborator that a command may use.

| Field | Purpose | Phase 2 placeholder |
|-------|---------|---------------------|
| `Config` | Resolved configuration for this invocation. | `&config.Config{}` |
| `Logger` | Diagnostic logger. | `newLogger(stderr)` (slog-backed). |
| `FS` | Filesystem abstraction. | `filesystem.NewOSFS(rootPath)`. |
| `Stdout` | Destination for successful output. | `opts.stdout`. |
| `Stderr` | Destination for diagnostics. | `opts.stderr`. |
| `Env` | Environment variable lookup. | `opts.env`. |

`Dependencies` is constructed by `buildDependencies(opts)` — **exactly
once per invocation**, from `executeWithOptions`.

The field list above is the **Phase 2 shape**. Future phases extend
it as new subsystems land (blueprint, template, renderer, policy,
registry). Adding a field is a two-line change: one line in the
struct definition, one line in `buildDependencies`. No command
constructor signature changes. This is the property that makes
`Dependencies` a stable boundary across phases.

### 11.3 The Transformation

`executeWithOptions` is the single transformation point between the
two boundaries:

```text
process ──► options ──► buildDependencies ──► Dependencies ──► commands
```

The transformation is one-directional and pure:

- **One-directional:** There is no path from `Dependencies` back to
  `options`, and no path from a command back to the process.
- **Pure:** Given the same `options`, `buildDependencies` always
  produces the same `Dependencies`. It reads only its argument,
  allocates a new value, and returns it. It does not touch the
  filesystem, the environment, or any global state.

The transformation is performed in exactly one place:

```go
func executeWithOptions(opts options) int {
    if err := validateArgs(opts.args); err != nil {
        fmt.Fprintln(opts.stderr, formatError(err))
        return exitCodeFromError(err)
    }
    deps := buildDependencies(opts)   // the single transformation
    root := newRootCmd(deps)          // Dependencies, not options
    root.SetArgs(opts.args)
    root.SetIn(opts.stdin)
    root.SetOut(opts.stdout)
    root.SetErr(opts.stderr)
    // ...
}
```

The `validateArgs` call is the pre-parse validation stage (§ 11.15).
It runs before `buildDependencies` because it inspects the raw
argument list, not the resolved collaborators.

### 11.4 Why Two Boundaries

Without the two-boundary split, testing the CLI would require spawning
a subprocess for every test case. Subprocess tests are slow
(milliseconds per case instead of microseconds), brittle (they depend
on the test binary being built), and inconvenient (they cannot
inspect internal state).

With the split, the CLI has **two testability seams**:

1. **The `options` seam.** A test constructs an `options` value with
   synthetic inputs, calls `executeWithOptions`, and asserts on the
   captured output. This tests the entire CLI end-to-end without a
   subprocess.

2. **The `Dependencies` seam.** A test constructs a `Dependencies`
   value directly and passes it to a subcommand constructor. This
   tests an individual command in isolation, without going through
   `options` at all.

The two seams serve different purposes:

| Seam | Tests | Cost | When to use |
|------|-------|------|-------------|
| `options` | The whole CLI, from argument parsing to output. | Higher (constructs the full command tree). | Integration tests. |
| `Dependencies` | A single command's behaviour. | Lower (bypasses the tree). | Unit tests. |

Both seams are supported by the same struct definitions. Neither
requires touching `os.*`.

### 11.5 The Single `os.*` Reader and the Single Construction Site

Two invariants make the two-boundary model auditable:

1. **`defaultOptions` is the only function that reads `os.Args`,
   `os.Stdin`, `os.Stdout`, `os.Stderr`, `os.Getenv`, or `os.Getwd`.**
   Every other function receives these values through the `options`
   struct, and every command receives them through the `Dependencies`
   struct.

2. **`buildDependencies` is called exactly once in production code,
   from `executeWithOptions`.** No other function constructs a
   `Dependencies` value. A future refactor that constructs
   `Dependencies` in `newRootCmd`, or in a subcommand constructor, or
   in a test helper outside the package, would violate this
   invariant.

Both invariants are verified by:

- **Structural tests** in `internal/cli/structure_test.go` and
  `internal/cli/execute_test.go` (white-box package `cli`).
- **Taskfile targets** `verify:two-boundary` and
  `verify:handler-boundary`, which grep the source tree.
- **Code review.**

### 11.6 What Is Not Tested In Process

A few behaviours are only observable at the process boundary:

- `main.go` correctly forwards `Execute()`'s return value to
  `os.Exit`.
- The compiled binary starts, runs, and exits with the expected
  code.
- Signal handling (not yet implemented).

These are covered by integration tests in
`cmd/forge/binary_integration_test.go`, behind the `integration`
build tag. They are the exception: most of the CLI is testable
in-process.

### 11.7 The Error Path

When the command tree returns a non-nil error, `executeWithOptions`:

1. Formats the error via `formatError`.
2. Writes the formatted string to `opts.stderr`, followed by a
   newline.
3. Returns the exit code for the error's category, via
   `exitCodeFromError`.

When the command tree returns a nil error, the function returns
`ExitSuccess` without writing anything.

The pre-parse validation stage uses the same error path. A malformed
invocation produces an error from `validateArgs`, which is formatted
by the same `formatError`, written to the same stderr, and mapped to
an exit code by the same `exitCodeFromError`. The result is
indistinguishable from an error produced by a command: same stream,
same message shape, same exit code.

The two functions called here are documented in
[`docs/development.md`](./development.md) (exit codes) and in
[`docs/cli-ux-spec.md`](./cli-ux-spec.md) § 7 (exit code contract).

### 11.8 The Output Path

Successful output is written by the command tree itself, to
`opts.stdout` (which is `deps.Stdout`). `executeWithOptions` does not
write to `opts.stdout`; it only wires it to the command tree.

This preserves the standard Unix convention: successful output goes
to stdout, diagnostics and errors go to stderr.

### 11.9 Related Files

| File | Purpose |
|------|---------|
| `internal/cli/execute.go` | `Execute`, `executeWithOptions`, `options`, `defaultOptions`, `formatError`. |
| `internal/cli/deps.go` | `Dependencies`, `Logger`, `slogLogger`, `noopLogger`, `newLogger`, `buildDependencies`. |
| `internal/cli/deps_test.go` | White-box tests for `Dependencies` and its construction. |
| `internal/cli/execute_test.go` | White-box tests for the process boundary and the transformation. |
| `internal/cli/exitcodes.go` | Exit code constants and the error-to-code mapping. |
| `internal/cli/root.go` | The root command constructor. Accepts a `Dependencies` value and registers subcommands. |
| `internal/cli/registry.go` | The central command registry (§ 11.13). |
| `internal/cli/flags.go` | The global flag registration and readers (§ 11.14). |
| `internal/cli/validate.go` | The pre-parse argument validator (§ 11.15). |
| `internal/cli/version.go` | The `forge version` handler. Reference implementation of the handler / service pattern (§ 11.12). |
| `internal/cli/version_test.go` | White-box tests for the version handler, including the structural tests that enforce § 11.12. |
| `internal/cli/structure_test.go` | Black-box structural tests for the package's shape. |
| `internal/cli/registry_test.go` | Tests for the command registry. |
| `internal/cli/contract_test.go` | Structural tests for the command constructor contract. |
| `internal/cli/validate_test.go` | Unit tests for the pre-parse argument validator. |
| `internal/cli/flags_test.go` | Unit tests for the global flag readers and the log-level resolver. |
| `internal/app/version/service.go` | The version application service. Pure. |
| `internal/app/version/format.go` | The version formatter. Takes an `io.Writer`. |
| `internal/app/version/buildinfo.go` | Reads build metadata from `internal/version`. |
| `internal/app/version/service_test.go` | Unit tests for the version service, without Cobra. |
| `internal/version/version.go` | The build metadata variables and their accessor. Leaf package; imported by `internal/cli` and `internal/app/version`. |
| `internal/version/version_test.go` | Unit tests for the metadata accessor. |
| `cmd/forge/main.go` | The process entry point. Calls `Execute()` and forwards its return to `os.Exit`. |
| `cmd/forge/binary_integration_test.go` | Process-boundary tests. |

### 11.10 Rules for Extending the Model

1. **`Execute` remains a one-line wrapper.** Its body must not grow.
   Every new behaviour belongs in `executeWithOptions` or below.

2. **`executeWithOptions` reads only from `opts`.** It must not read
   `os.Args`, `os.Stdin`, `os.Stdout`, `os.Stderr`, or `os.Getenv`.
   This invariant is enforced by a structural test
   (`TestExecuteWithOptions_NoProcessStreamsReferenced`) in
   `execute_test.go`.

3. **`buildDependencies` is the only function that constructs a
   `Dependencies` value.** A struct literal of the form
   `Dependencies{...}` must not appear outside `deps.go` in
   production code. This invariant is enforced by a structural test
   (`TestExecuteWithOptions_DoesNotConstructDependenciesByHand`) in
   `execute_test.go`.

4. **`newRootCmd` accepts `Dependencies`, not `options`.** A wrapper
   with the old signature must not be added: it would create a second
   construction path and violate rule 3.

5. **`formatError` and `exitCodeFromError` remain separate.** The
   first renders an error; the second classifies it. Merging them
   would couple error presentation to error classification, which are
   distinct concerns.

6. **Adding a field to `options` is a breaking change to the test
   suite.** Every test that constructs an `options` value must be
   updated. The change is small, but it must be deliberate.

7. **Adding a field to `Dependencies` is the intended extension
   mechanism.** The field is added to the struct, constructed in
   `buildDependencies`, and consumed by commands that want it. No
   command constructor signature changes. This is the property that
   makes `Dependencies` a stable boundary across phases.

### 11.11 Relationship to § 5 and § 16

The two-boundary model is the CLI's implementation of two
architectural principles:

- **§ 5.1 — Dependencies point inward.** The `Dependencies` struct is
  the CLI's inward-facing surface. Commands depend on resolved
  collaborators, not on raw process inputs. The transformation from
  `options` to `Dependencies` is the CLI's single point of contact
  with the outside world.

- **§ 16.4 — No global state.** Commands do not read `os.Args`,
  `os.Getenv`, or any other global. Every effect flows through
  `Dependencies`. The `options` struct exists only so that tests can
  inject the process environment; production code reads it once, in
  `defaultOptions`.

The two-boundary model does not introduce a new architectural
principle. It refines the existing principles into a concrete,
testable, auditable structure.

### 11.12 Handler / Service Boundary

WBS 4.3.1 establishes a boundary between **Cobra handlers** (the
`RunE` closures in `internal/cli/`) and **application services** (the
packages under `internal/app/`). The boundary is the operational form
of the "no business logic in handlers" rule from WBS 4.3.

#### Allowed in a handler

A Cobra handler is permitted to do only the following:

- Parse and validate flags.
- Collect arguments.
- Construct an application service.
- Call the application service.
- Format the result — and only by delegating to the service's
  formatter, not by formatting inline.
- Return an error.

#### Not allowed in a handler

A Cobra handler must not do any of the following:

- Perform filesystem I/O.
- Perform network I/O.
- Contain business rules.
- Contain business logic.
- Format complex output (that belongs to a formatter in the service
  package).
- Call `os.Exit`.
- Write to `os.Stdout`, `os.Stderr`, or `os.Stdin`.
- Call `fmt.Println`, `fmt.Printf`, `fmt.Fprintln`, or any other
  function that writes to a process-global stream.

#### The pattern

For every command, there must be:

1. **A handler.** A thin `RunE` closure in
   `internal/cli/<name>.go`. Target: under 20 lines of handler body.
   The handler parses flags, constructs the service, calls it, and
   returns the error.

2. **An application service.** A package under
   `internal/app/<name>/`. The service contains the logic, in one
   or more pure functions. It receives its inputs as parameters and
   its output writer as a parameter; it never reads a process global.

3. **A result type.** A plain struct that the service returns and the
   formatter renders. The result type is exported from the service
   package. It has no methods and no dependencies.

#### Reference implementation

The `forge version` command is the reference implementation. Its
structure:

| File | Purpose |
|------|---------|
| `internal/cli/version.go` | The thin handler. Three lines of body. |
| `internal/app/version/service.go` | The pure service. `Get() Info`, `Raw() string`. |
| `internal/app/version/format.go` | The formatter. `Format(io.Writer, Info) error`. |
| `internal/app/version/service_test.go` | Unit tests that do not construct a Cobra command. |
| `internal/version/version.go` | The build metadata variables. Leaf package. |
| `internal/version/version_test.go` | Unit tests for the accessor. |

The handler body is:

```go
RunE: func(cmd *cobra.Command, args []string) error {
    return version.Format(deps.Stdout, version.Get())
},
```

Every future command follows this shape. The handler receives a
`Dependencies` value, reads the collaborators it needs, and delegates
the work. The service knows nothing about Cobra, flags, or
`Dependencies`.

#### The build-metadata leaf package

The version service needs four values — `Version`, `Commit`,
`BuildDate`, `Dirty` — that are injected at link time. Before WBS
4.3.1, the first three lived in `internal/cli`. That created an
import cycle:

```text
internal/cli  ──────►  internal/app/version
     ▲                        │
     │                        │
     └────────────────────────┘
              (cycle)
```

`internal/cli` imports `internal/app/version` for the version handler.
`internal/app/version` would need to import `internal/cli` for the
build variables. Go rejects the cycle.

WBS 4.3.1 breaks the cycle by relocating the variables to
`internal/version`, a leaf package that imports nothing from Forge.
Both `internal/cli` and `internal/app/version` import it. The graph
becomes a directed acyclic graph:

```text
internal/cli        internal/app/version
         \            /
          v          v
          internal/version
```

The relocation is not optional. "Treat the values as internal" is not
a construct that Go supports; either the import exists or it does
not, and if it exists, the cycle exists.

#### Enforcement

The boundary is enforced by:

- **Structural tests** in `internal/cli/version_test.go` that assert
  the handler body contains exactly one service call and no
  forbidden tokens (`os.Stdout`, `os.Exit`, `fmt.Fprintln`, ...).
  The tests strip comments before searching, so that the handler's
  docstring may name the forbidden tokens while explaining that the
  handler does not use them.
- **Code review**, using the checklist in
  [`docs/development.md`](./development.md).
- **The example itself.** `version.go` is the shortest handler in the
  codebase, and it is the template every subsequent command copies.

#### Why this boundary

Without it, every command would accumulate its own I/O, formatting,
and business rules. The CLI package would grow into a monolith. The
application layer would be empty. Testing a command would require
constructing a Cobra command and inspecting a buffer, rather than
calling a pure function.

With the boundary, the application logic is testable in isolation. A
test calls `version.Get()` and asserts on the returned `Info`; no
Cobra command is constructed, no buffer is inspected, no flags are
parsed. This is the property that keeps the test suite fast as the
command count grows.

#### Relationship to § 4.2.1 and § 4.2.2

This boundary is the concrete shape of two module-level rules:

- **§ 4.2.1 (CLI).** "Does not: contain business logic, read files,
  render templates." The handler / service split is how that rule is
  enforced in code.

- **§ 4.2.2 (Application).** "Implements use cases (create project,
  validate, check, diff, update, explain)." The service package is
  the concrete shape of an application use case.

### 11.13 Command Registration

WBS 4.4.1 establishes a central command registry: the single,
deterministic list of subcommand constructors that `newRootCmd`
iterates to build the command tree.

#### The registry

`internal/cli/registry.go` defines two things:

```go
type commandConstructor func(Dependencies) *cobra.Command

var registry = []commandConstructor{
    newConfigCmd,
    newVersionCmd,
    // WBS 5.x adds more
}
```

`newRootCmd` consumes the registry:

```go
for _, ctor := range registry {
    root.AddCommand(ctor(deps))
}
```

#### The rules

1. **One file owns the registry.** `registry.go` is the only file
   that lists subcommands. Adding a command means appending to the
   slice in that file.

2. **Order is explicit.** The registry is a slice, not a map. The
   order of the slice determines the order in which Cobra receives
   the commands, which determines the order in help output for
   visible commands.

3. **Every constructor takes `Dependencies`.** No exceptions. A
   constructor that accepted `options` would be free to read
   process globals, violating the two-boundary model (§ 11.2).

4. **Every constructor returns `*cobra.Command`.** No exceptions.
   The registry does not abstract over CLI frameworks.

5. **Command files are named `<command>.go`.** The file contains the
   constructor and its helpers. It does not contain business logic
   (that lives in the application service; see § 11.12).

6. **No `init()`-based registration.** A command's file must not
   append to the registry from an `init()` function. Registration is
   a static, reviewable property of `registry.go`.

#### Adding a command

The four-step procedure is documented in the package docstring of
`registry.go`. In summary:

1. Create `internal/cli/<name>.go`.
2. Append `new<Name>Cmd` to the registry slice in the correct
   position.
3. Add the file to `expectedSourceFiles` in
   `internal/cli/structure_test.go`.
4. Add tests.

#### Enforcement

The registry is enforced by:

- **Structural tests** in `internal/cli/registry_test.go` that
  assert the registry's contents are non-empty, have no duplicates,
  and produce commands with the expected shape.
- **Structural tests** in `internal/cli/contract_test.go` that
  assert every registry entry satisfies the command constructor
  contract (WBS 4.4.2).
- **Structural tests** in `internal/cli/root_test.go` that assert
  the registry order matches the help output order.
- **Code review.**

### 11.14 Global Flags

WBS 5.3.1 establishes the CLI's global flag inventory. The inventory
is deliberately small: three flags in Phase 2, each with a documented
consumer in a later WBS item.

#### The inventory

| Flag | Short | Type | Persistent | Consumer | Semantics |
|------|-------|------|------------|----------|-----------|
| `--verbose` | — | bool | yes | WBS 12.0 (logging) | Set log level to DEBUG |
| `--quiet` | — | bool | yes | WBS 12.0 (logging) | Set log level to ERROR |
| `--config` | — | string | yes | WBS 8.0 (config) | Path to config file |

No other global flags exist. Adding a fourth flag requires an ADR.

#### Where the flags live

The flag names, the registration function, and the helpers that read
the parsed values live in `internal/cli/flags.go`:

- `FlagVerbose`, `FlagQuiet`, `FlagConfig` — the flag-name
  constants.
- `registerGlobalFlags(cmd *cobra.Command)` — the registration
  function, called from `newRootCmd`.
- `verboseRequested(cmd)`, `quietRequested(cmd)`,
  `configPath(cmd)` — the readers.
- `resolveLogLevel(cmd) string` — the resolver that applies the
  precedence rule.

#### Persistence

All three flags are **persistent** flags. Cobra's persistent flags
are inherited by every subcommand, so both `forge --verbose config`
and `forge config --verbose` are equivalent. The persistence is the
reason the flags appear in the root command's help output under
"Global Flags" rather than "Flags".

The persistence is the reason the flags can appear **either before or
after** the subcommand name. Cobra's parser handles both positions;
the CLI's behaviour is identical for both.

#### Precedence

When `--verbose` and `--quiet` are both set, `--quiet` wins. The
effective log level is ERROR.

The rule is implemented in `resolveLogLevel`, which reads the two
flags from the command and returns the effective log level as a
string. The rule is documented in
[`docs/cli-ux-spec.md`](./cli-ux-spec.md) § 4.11 and is pinned by
`TestResolveLogLevel` in `flags_test.go`.

The precedence is not accompanied by a runtime warning. The rationale
is that a warning about conflicting flags is itself output, and the
user who asked for quiet asked for less output. The decision is
documented in the specification; the code does not emit a warning.

#### Why the flags are not in `Dependencies`

The global flags are read from the command (`cmd.Flags()`), not from
`Dependencies`. This is deliberate: the flags are a property of the
invocation, and Cobra's parser is the source of truth for their
values. The `Dependencies` struct holds resolved collaborators, not
parsed flags. A future `config` subsystem that consumes `--config`
will read the value from the command and use it to construct the
resolved configuration, which is what `Dependencies.Config` will
hold.

#### Adding a global flag

Adding a global flag requires an ADR. The ADR must name the consumer
WBS item, define the flag's semantics, and define its precedence
relative to the existing flags. The rule is documented in
[`docs/cli-ux-spec.md`](./cli-ux-spec.md) § 4.11.

The rule exists to prevent flag creep. A CLI with fifteen global
flags has no global flags, because users cannot remember which one
does what.

#### Enforcement

The inventory is enforced by:

- **Structural tests** in `internal/cli/root_test.go` that assert the
  root command has exactly three persistent flags.
- **Unit tests** in `internal/cli/flags_test.go` that exercise the
  readers and the resolver.
- **Taskfile target** `verify:global-flags`, which counts the flag
  registrations in `flags.go`.
- **Code review.**

### 11.15 Pre-Parse Argument Validation

WBS 5.2.2 and WBS 5.2.3 establish a pre-parse argument validator: a
stage in the CLI's pipeline that runs **before** Cobra parses the
arguments, and rejects a small set of malformed invocations that
Cobra would otherwise accept silently.

#### The four rejected shapes

The validator rejects four malformed invocations:

1. `forge --help <cmd>` — the `--help` flag takes no argument.
2. `forge --version <arg>` — the `--version` flag takes no argument.
3. `forge --config` with no value — `--config` requires a value.
4. `forge help <unknown>` — an unknown help topic.

Each rejection is documented in
[`docs/cli-ux-spec.md`](./cli-ux-spec.md) § 4.9 (help), § 4.10
(version), or § 4.11 (config).

#### Where the validator lives

The validator lives in `internal/cli/validate.go`. The file
defines:

- `validateArgs(args []string) error` — the entry point. Dispatches
  to the three sub-validators.
- `validateHelpFlagWithArgs(args)` — rejects shapes 1.
- `validateVersionFlagWithArgs(args)` — rejects shape 2.
- `validateConfigFlagWithArgs(args)` — rejects shape 3.
- `validateHelpTopic(args)` — rejects shape 4.
- `isKnownCommandName(name)` — the predicate used by shape 4. Reads
  the registry directly.
- `commandName(use string) string` — extracts a command's name from
  its `Use` field.
- `validationDependencies()` — the `Dependencies` value used to
  construct commands during validation.

#### Why the validator is at the pre-parse stage

Cobra's `--help` and `--version` flags are **interception flags**:
when either is set, Cobra short-circuits the hook chain
(`PersistentPreRunE`, `PreRunE`, `RunE`) and runs its help or version
path directly. A malformed `--help <cmd>` or `--version <arg>`
invocation is therefore invisible from inside the command tree.

The pre-parse stage is the only point at which the malformed
invocations are observable. `validateArgs` runs in
`executeWithOptions` before `root.Execute()` is called, which is
before Cobra parses the arguments.

#### How the rejection flows

The validator returns a plain `error`. `executeWithOptions` formats
it with `formatError`, writes it to `opts.stderr`, and maps it to an
exit code with `exitCodeFromError`. The result is indistinguishable
from an error produced by a command: same stream, same message shape,
same exit code. The `exitCodeFromError` mapping classifies the plain
error as `ExitUsage` because it has no category.

#### The validator's data sources

The validator reads two inputs:

1. **The raw argument list.** The `args` parameter is the raw
   `[]string` from `options.args`. The validator scans it with simple
   left-to-right loops. It does not parse flags, does not resolve
   commands, and does not construct the command tree.

2. **The registry.** The `registry` package-level slice is the
   source of truth for the set of Forge-authored commands. The
   validator reads the slice directly rather than the constructed
   command tree, because the tree is not available at this stage
   (`newRootCmd` runs after `validateArgs`).

The validator also accepts Cobra's auto-generated `help` and
`completion` command names as valid help topics. They are not in the
registry; the validator handles them by name.

#### Enforcement

The validator is enforced by:

- **Unit tests** in `internal/cli/validate_test.go` that exercise
  the top-level dispatcher and each sub-validator in isolation.
- **Contract tests** in `internal/cli/root_test.go` that assert the
  observable behaviour of the four rejected invocations: non-zero
  exit, empty stdout, non-empty stderr.
- **Taskfile targets** `verify:help-contract` and
  `verify:version-contract`, which run the contract tests and grep
  for accidental regressions.
- **Code review.**

#### Why the validator is not a Cobra hook

A Cobra hook (`PersistentPreRunE` or `RunE`) cannot observe the
malformed invocations, because Cobra's interception flags short-
circuit the hook chain before it runs. The pre-parse stage is the
only layer that sees the raw arguments before Cobra's parser
processes them. This is the same reasoning that § 11.5 applies to
the `options` struct: the earlier a stage sits in the pipeline, the
more it can see.

---

## 12. Cross-Cutting Concerns

The following concerns cut across modules and must be handled
consistently.

### 12.1 Error Handling

Every module:

- Returns errors using the `ForgeError` type
- Never panics on user input
- Wraps errors with context
- Includes suggestions where possible

See § 8 for the full error architecture.

### 12.2 Logging

Every module:

- Receives a `Logger` via constructor injection
- Logs at appropriate levels
- Includes structured fields
- Never logs secrets

See § 9 for the full logging architecture.

### 12.3 Context Propagation

Every long-running operation:

- Accepts a `context.Context` as the first argument
- Checks for cancellation at safe points
- Propagates deadlines

### 12.4 Determinism

Every module that produces output:

- Uses deterministic ordering
- Avoids timestamps in output (except metadata)
- Avoids environment-dependent behaviour
- Uses stable algorithms

### 12.5 Security

Every module that accesses the filesystem:

- Goes through the `Filesystem` interface
- Respects the boundary rules
- Never writes outside the target directory
- Never logs secrets

See [`docs/security-model.md`](./security-model.md) for full
policies.

### 12.6 Testing

Every module has:

- **Unit tests:** Pure functions, no I/O
- **Integration tests:** With the Filesystem interface mocked
- **End-to-end tests:** Full command execution

---

## 13. Package Layout (Illustrative)

The following is the expected package layout. It is illustrative, not
binding; the exact structure will be finalised in Phase 2.

```text
forge/
├── cmd/
│   └── forge/
│       └── main.go
├── internal/
│   ├── cli/          # CLI commands
│   ├── app/          # Application services
│   │   ├── new/
│   │   ├── init/
│   │   ├── validate/
│   │   ├── check/
│   │   ├── diff/
│   │   ├── update/
│   │   ├── explain/
│   │   └── version/  # Version service (WBS 4.3.1)
│   ├── domain/       # Domain logic (pure)
│   │   ├── blueprint/
│   │   ├── template/
│   │   ├── component/
│   │   ├── policy/
│   │   └── config/
│   ├── renderer/     # Rendering engine
│   ├── validator/    # Validation engine
│   ├── update/       # Update engine
│   ├── state/        # State store
│   ├── version/      # Build metadata (WBS 4.3.1) — leaf package
│   └── infra/        # Infrastructure
│       ├── fs/       # Filesystem
│       ├── process/  # Process execution
│       ├── registry/ # Registry (future)
│       ├── logging/  # Logging
│       └── output/   # Output formatting
├── templates/        # Bundled templates
├── examples/         # Example blueprints and templates
└── docs/             # Documentation
```

### 13.1 Package Boundaries

| Package | Contains |
|---------|----------|
| `internal/cli` | Command definitions, argument parsing, output wiring |
| `internal/app/<command>` | Use case orchestration for each command |
| `internal/domain/<concept>` | Pure domain logic and types |
| `internal/renderer` | Rendering of template content |
| `internal/validator` | Validation rules and engine |
| `internal/update` | Update planning, merging, application |
| `internal/state` | Reading and writing `.forge/state.yaml` |
| `internal/version` | Build metadata (version, commit, build date) |
| `internal/infra/fs` | Filesystem interface and OS implementation |
| `internal/infra/logging` | Logger interface and implementation |
| `internal/infra/output` | Human and JSON output formatters |
| `templates/` | Bundled templates |

Within `internal/cli`, the files are:

| File | Purpose |
|------|---------|
| `doc.go` | Package documentation and public contract |
| `execute.go` | `Execute`, `executeWithOptions`, `options`, `defaultOptions`, `formatError` |
| `deps.go` | `Dependencies`, `Logger`, `slogLogger`, `noopLogger`, `newLogger`, `buildDependencies` |
| `exitcodes.go` | Exit code constants and the error-to-code mapping |
| `root.go` | The root command constructor |
| `registry.go` | The central command registry |
| `flags.go` | The global flag registration and readers |
| `validate.go` | The pre-parse argument validator |
| `version.go` | The `forge version` handler |
| `config.go` | The hidden `forge config` placeholder |

### 13.2 Import Rules

- `internal/domain/*` may not import any other `internal/*` package
  (except other `internal/domain/*`).
- `internal/app/*` may import `internal/domain/*` and
  `internal/infra/*`.
- `internal/cli` may import `internal/app/*` and `internal/infra/*`.
- `internal/infra/*` may import `internal/domain/*` (for shared
  types) but not `internal/app/*` or `internal/cli`.
- `internal/version` may not import any other `internal/*` package.
  It is a leaf.
- No package outside `internal/cli` may import
  `github.com/spf13/cobra`. The CLI framework is a CLI-layer
  concern.

---

## 14. Testability

The architecture is designed for testability.

### 14.1 Unit Tests

Domain logic is tested in isolation:

- No filesystem access
- No process execution
- No network access
- Fast (milliseconds)

### 14.2 Integration Tests

Application services are tested with a mock Filesystem:

- In-memory filesystem for speed
- Deterministic behaviour
- No OS interaction

### 14.3 End-to-End Tests

The CLI is tested as a subprocess:

- Real filesystem
- Real process execution
- Full command flow

### 14.4 Golden Tests

Some outputs are compared against golden files:

- CLI output
- JSON output
- Rendered repositories

Golden files are committed and version-controlled.

### 14.5 Fuzz Tests

Fuzz tests are used for:

- Path parsing
- YAML parsing
- Template rendering
- Blueprint validation

---

## 15. Extensibility

The architecture supports future extension without breaking existing
behaviour.

### 15.1 Adding a New Command

1. Define the handler in `internal/cli/<name>.go`. It must be thin
   (under 20 lines of body) and follow the pattern in § 11.12.
2. Define the use case in `internal/app/<name>/`. The service must
   be pure (no `os.*`, no Cobra import).
3. Append the handler's constructor to the registry slice in
   `internal/cli/registry.go`. See § 11.13.
4. Add the handler file to `expectedSourceFiles` in
   `internal/cli/structure_test.go`.
5. Add the handler file to `HANDLER_FILES` in `Taskfile.yml`.
6. Add tests: handler tests in `internal/cli/<name>_test.go`,
   service tests in `internal/app/<name>/`.

### 15.2 Adding a New Domain Concept

1. Define the concept in `internal/domain/<concept>`
2. Define its interfaces
3. Wire it into the application layer
4. Implement infrastructure as needed

### 15.3 Adding a New Infrastructure Provider

1. Define the interface (in domain or application)
2. Implement the provider in `internal/infra/<provider>`
3. Wire it into the CLI

### 15.4 Adding a Global Flag

1. Write an ADR that names the consumer WBS item and the flag's
   semantics.
2. Add the flag-name constant to `internal/cli/flags.go`.
3. Register the flag in `registerGlobalFlags`.
4. Add the reader and (if needed) the resolver.
5. Update § 4.11 of `docs/cli-ux-spec.md`.
6. Add tests.

### 15.5 Adding a Pre-Parse Rejection

1. Write the rejection's contract in `docs/cli-ux-spec.md`. Name the
   malformed shape and the correct alternative.
2. Add a sub-validator to `internal/cli/validate.go`.
3. Add the sub-validator to the `validateArgs` dispatcher.
4. Add unit tests in `internal/cli/validate_test.go`.
5. Add a contract test in `internal/cli/root_test.go`.

### 15.6 Plugin System

A plugin system is not supported in Phase 1. The architecture reserves
space for future plugins:

- Extension points at the Domain layer
- Plugin loading in the CLI layer
- Sandboxing in the Infrastructure layer

Plugins will be introduced in a future phase (see
[`docs/product-discovery.md`](./product-discovery.md) § 10).

---

## 16. Anti-Patterns to Avoid

The following are explicitly discouraged.

### 16.1 Business Logic in the CLI

The CLI layer is for command parsing and output formatting only. Any
business logic belongs in the Application or Domain layer. See § 11.12
for the operational rule.

### 16.2 Direct Filesystem Access from Domain

Domain code never calls `os.ReadFile`, `os.WriteFile`, or similar. All
filesystem access goes through the `Filesystem` interface.

### 16.3 God Objects

No single object orchestrates everything. Responsibilities are split
across modules.

### 16.4 Global State

No global mutable variables. Dependencies are injected via
constructors.

### 16.5 Implicit Dependencies

Every dependency is explicit in a constructor. No hidden globals, no
service locators.

### 16.6 Circular Dependencies

Packages do not import each other cyclically. The dependency
direction is strictly enforced.

### 16.7 Silent Failures

Every error is reported. No swallowed errors, no ignored returns.

### 16.8 Unstructured Errors

Every error uses the `ForgeError` type. No `errors.New("something
failed")`.

### 16.9 Unstructured Logs

Every log message has a level and structured fields. No `fmt.Println`
for diagnostics.

### 16.10 Environment-Dependent Behaviour

Forge behaves the same regardless of environment. Environment
variables do not change behaviour except through explicit, documented
configurations.

### 16.11 Cobra Outside the CLI Layer

No package outside `internal/cli` may import
`github.com/spf13/cobra`. The CLI framework is a CLI-layer concern; an
application service that imports Cobra has collapsed the boundary
between the handler and the service.

### 16.12 Build Metadata Outside the Leaf Package

The build metadata variables (`Version`, `Commit`, `BuildDate`,
`Dirty`) live in `internal/version` and only there. Defining them
elsewhere would recreate the import cycle that WBS 4.3.1 broke. See
§ 11.12.

### 16.13 `init()`-Based Command Registration

Commands are registered in the `registry` slice in `registry.go`. A
command file must not append to the registry from an `init()`
function. See § 11.13.

### 16.14 Speculative Global Flags

A global flag must have a documented consumer in a later WBS item. A
flag without a consumer is not added. See § 11.14.

### 16.15 Pre-Parse Rejection in a Cobra Hook

Malformed invocations that involve interception flags (`--help`,
`--version`) cannot be rejected in a Cobra hook, because Cobra
short-circuits the hook chain. The rejection belongs at the
pre-parse stage. See § 11.15.

---

## 17. Relationship to Other Specifications

| Specification | Relationship |
|---------------|--------------|
| [Blueprint](./blueprint-spec.md) | Domain model consumed by the Blueprint package |
| [forge.yaml](./forge-yaml-spec.md) | Configuration consumed by the Configuration package |
| [Template](./template-spec.md) | Domain model consumed by the Template package |
| [Component](./component-spec.md) | Domain model consumed by the Component package |
| [Validation](./validation-spec.md) | Implemented by the Validator package |
| [Security Model](./security-model.md) | Enforced by the Filesystem package |
| [Update Model](./update-model.md) | Implemented by the Update Engine package |
| [CLI UX Spec](./cli-ux-spec.md) | Implemented by the CLI layer |
| [Product Discovery](./product-discovery.md) | Defines the product this architecture implements |

---

## 18. Open Questions

The following questions remain open and should be resolved before
implementation:

- Should the CLI layer be built with **Cobra** (the de facto standard)
  or with a lighter framework?
- Should the Application layer use a **use case** pattern (one service
  per command) or a **service** pattern (one service per domain
  concept)?
- Should the Domain layer use **value objects** or plain structs?
- Should the Filesystem interface be **narrow** (few methods) or
  **wide** (all operations)?
- Should the Update Engine use a library for **three-way merge** or
  implement one?
- Should logging use a library (e.g., `slog`, `zap`) or a custom
  implementation?
- Should the CLI support **shell completion** out of the box?
- Should the architecture support **parallelism** for large
  repositories, and if so, where?
- Should the State store use **YAML**, **JSON**, or a binary format?
- Should the architecture include a **cache** layer for template
  resolution?

These questions will be addressed in Phase 2 as implementation
begins.

---

## 19. Document History

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 0.1.0 | 2026-10-09 | @thapelomagqazana | Initial Phase 1 draft. |
| 0.2.0 | 2026-10-09 | @thapelomagqazana | Added module list, dependency direction, interface contracts, and data flow. |
| 0.3.0 | 2026-10-09 | @thapelomagqazana | Refined § 11 to describe the two-boundary execution model introduced by WBS 4.2.2. Added the `Dependencies` struct, the `options`/`Dependencies` split, the transformation, the two testability seams, the two auditable invariants, and the seven rules for extending the model. Renamed § 11 from "The Two-Layer Execution Model" to "The Two-Boundary Execution Model". |
| 0.4.0 | 2026-10-10 | @thapelomagqazana | Added § 11.12 (Handler / Service Boundary) in response to WBS 4.3.1. Added the reference implementation (`forge version`), the allowed / forbidden table, the enforcement rules, and the rationale for the `internal/version` leaf package. Added § 4.2.2a (Version Service) as a concrete example of the Application module. Updated § 5.3, § 7.2, § 7.4, § 10, § 11.9, § 13, § 16, and the module list to reflect the new service. Added rules for adding a command in § 15.1. |
| 0.5.0 | 2026-10-10 | @thapelomagqazana | Added § 11.13 (Command Registration) in response to WBS 4.4.1. Documents the central registry, its rules, the four-step procedure for adding a command, and the enforcement. Added § 11.14 (Global Flags) in response to WBS 5.3.1. Documents the three-flag inventory, the persistence, the precedence rule, the rationale for reading flags from the command rather than `Dependencies`, and the ADR requirement for a fourth flag. Added § 11.15 (Pre-Parse Argument Validation) in response to WBS 5.2.2 and WBS 5.2.3. Documents the four rejected shapes, the file's structure, the pipeline placement, the data sources, and the reasoning for the pre-parse stage. Extended § 4.2.1 to list the CLI's files; extended § 7.1 and § 10 to show the pre-parse validation stage; extended § 11.9's related-files table; added § 15.4 and § 15.5 (adding a global flag, adding a pre-parse rejection); extended § 16 with three new anti-patterns (init-based registration, speculative flags, rejection in a Cobra hook). |
