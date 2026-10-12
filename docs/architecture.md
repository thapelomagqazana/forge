# Technical Architecture

- **Document type:** Model
- **Status:** Draft
- **Version:** 0.8.0
- **Author:** @thapelomagqazana
- **Created:** 2026-10-09
- **Last Updated:** 2026-10-12
- **Supersedes:** 0.7.0
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
- How is the ForgeError type structured, categorised, and wrapped?
- How is the user-facing error format frozen and tested?
- How is the programmer-facing log format produced?
- How are secrets redacted from errors?
- How do verbose and non-verbose modes differ?
- How is the error boundary integrated with the process exit code?
- How is logging structured?
- What does the architecture look like as a diagram?
- How does this document stay accurate as the code changes?

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
- Document accuracy policy

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

**Where interfaces live.** An interface is defined in the package
that owns the concept it describes, not necessarily in the domain
layer. The `Filesystem` interface is defined in
`internal/filesystem` alongside its OS implementation, because the
interface is a package-level contract for that package, not a
domain concept. The `Logger` interface is defined in
`internal/app/logging` for the same reason.

This is a deliberate deviation from the strict ports-and-adapters
convention, which places all interfaces in the domain layer. The
deviation is chosen because Phase 2 has a small number of leaf
packages, and each leaf's interface is consumed only by that leaf
and its callers. If a future phase introduces a second
implementation of the same interface for a different reason (for
example, an in-memory filesystem for testing that is not in the
same package as the OS filesystem), the interface moves to the
domain layer at that point. Until then, it lives with its
implementation.

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

| Module | Layer | Responsibility | Introduced by |
|--------|-------|----------------|---------------|
| **CLI** | Application | Parse commands, present output, map results to exit codes | WBS 4.1.1 |
| **Application** | Application | Orchestrate use cases; coordinate domain and infrastructure | WBS 4.2.1 |
| **Domain** | Domain | Express the core concepts of Forge | WBS 4.2.1 |
| **Configuration** | Domain | Load, validate, and represent configuration | WBS 4.2.1 |
| **Blueprint** | Domain | Represent and validate Blueprints | WBS 4.2.1 |
| **Template** | Domain | Represent and resolve templates | WBS 4.2.1 |
| **Component** | Domain | Represent and resolve components | WBS 4.2.1 |
| **Policy** | Domain | Represent and evaluate policies | WBS 4.2.1 |
| **ForgeErr** | Domain | Canonical error type, taxonomy, registry, wrapping, user format, log format, redaction, verbose modes, boundary integration | WBS 10.1.1, 10.1.2, 10.2.1, 10.3.1, 10.3.2, 10.4.1, 10.4.2, 10.4.3, 10.4.4, 10.5.1, 10.5.2 |
| **Renderer** | Application | Transform template content into rendered content | WBS 4.2.1 |
| **Validator** | Application | Evaluate rules against repository state | WBS 4.2.1 |
| **Update Engine** | Application | Compute and apply safe updates | WBS 4.2.1 |
| **Version Service** | Application | Report build metadata for the running binary | WBS 4.3.1 |
| **Filesystem** | Infrastructure | Provide safe, sandboxed filesystem access | WBS 4.2.1 |
| **Process** | Infrastructure | Provide process execution where needed | WBS 4.2.1 |
| **Registry** | Infrastructure | Fetch and verify remote artifacts (future) | Phase 16 |
| **Logging** | Infrastructure | Provide structured logging | WBS 4.2.1 |
| **Output** | Infrastructure | Format human and machine-readable output | WBS 4.2.1 |
| **Version Model** | Leaf | Hold the linker-injected build metadata | WBS 6.1.1 |

### 4.2 Module Descriptions

#### 4.2.1 CLI

**Responsibility:** Parse arguments, dispatch to Application, present
results, map to exit codes.

**Does not:** Contain business logic, read files, render templates.

**Depends on:** Application, Output, Logging.

**Introduced by:** WBS 4.1.1 (single-symbol public surface),
WBS 4.2.2 (two-boundary execution model), WBS 4.4.1 (command
registry), WBS 5.2.2 / WBS 5.2.3 (pre-parse validation),
WBS 5.3.1 (global flags), WBS 10.5.1 (error boundary integration).

The CLI module is composed of the files listed in § 13.1.

#### 4.2.2 Application

**Responsibility:** Implement use cases (create project, validate,
check, diff, update, explain).

**Does not:** Contain domain logic (delegates to Domain), read files
(delegates to Infrastructure).

**Depends on:** Domain, Renderer, Validator, Update Engine,
Filesystem, Logging.

**Introduced by:** WBS 4.2.1 (module structure), WBS 4.3.1
(handler / service boundary).

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

**Introduced by:** WBS 4.3.1.

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
- `ForgeErr`

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

**Interface placement:** The `Filesystem` interface is defined in
`internal/filesystem` alongside its OS implementation. See § 3.3
for the rationale.

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

#### 4.2.17 Version Model

**Responsibility:** Hold the four linker-injected build metadata
values (`Version`, `Commit`, `BuildDate`, `Dirty`) and expose them
through a pure accessor.

**Depends on:** Nothing. It is a **leaf package** in the module
graph, so that `internal/cli` and `internal/app/version` can both
import it without creating a cycle. See § 11.12.

**Introduced by:** WBS 6.1.1. Relocated from `internal/cli` by
WBS 4.3.1 to break the import cycle.

#### 4.2.18 ForgeErr

**Responsibility:** Define the canonical error type for Forge, the
closed category taxonomy, the frozen code registry, the wrapping
contract, the user-facing format, the programmer-facing log format,
the redaction policy, the verbose/non-verbose modes, and the
boundary integration with the process exit code.

**Does not:** Perform I/O. Read configuration. Know about the CLI,
the application services, or the infrastructure. Import anything
outside the Go standard library.

**Depends on:** Nothing. It is a **domain-layer leaf** package. It
sits inside the Domain layer because every other module produces
errors of its type; it is a leaf because nothing it does requires
another Forge package.

**Introduced by:**

- WBS 10.1.1 — the `Error` type, its unexported fields, its
  accessors, and the pure invariant helpers (`validateMessage`,
  `validateRemediation`, `validateCategory`).
- WBS 10.1.2 — the frozen constructor API: `New`, `Wrap`,
  `WithCause`, `WithContext`, `WithRemediation`.
- WBS 10.2.1 — the closed category taxonomy: seven categories,
  each with a Code prefix and a process exit code.
- WBS 10.3.1 — the frozen code registry: 19 Phase 2 codes, each
  with a category (derived from its prefix), an exit code (derived
  from its category), an introduced-phase marker, and a deprecation
  flag.
- WBS 10.3.2 — the wrapping contract: `Unwrap`, `Is`, and `As`
  semantics, and the interaction with `errors.Is`, `errors.As`,
  `fmt.Errorf("%w", ...)`, and `errors.Join`.
- WBS 10.4.1 — the user-facing format: `Format`, the byte-level
  rules F1–F12, and the golden-file tests in
  `testdata/format/`.
- WBS 10.4.2 — the programmer-facing log format: `LogValue` and the
  `slog.LogValuer` interface.
- WBS 10.4.3 — the redaction policy: `redact`, `redactPath`, the
  frozen secret-keys list, and `AllSecretKeys`.
- WBS 10.4.4 — the verbose mode: `Options.Verbose`, `Options.Stack`,
  and the verbose extensions (cause block, stack block, internal
  context keys, full home paths).
- WBS 10.5.1 — the boundary integration: the CLI's `Execute` and
  `executeWithOptions` render through `forgeerr.Format` and derive
  the exit code from `Category.ExitCode()`; `formatError` and
  `exitCodeFromError` are deleted.
- WBS 10.5.2 — the migration of the config package's errors to
  `*forgeerr.Error` values, and the deletion of the parallel
  `configError` type.

The package's files are listed in § 13.2. Its public surface is
listed in § 6.13. Its architectural rules are in § 8.

---

## 5. Dependency Direction

### 5.1 The Rule

> **Dependencies point inward.**

```text
┌───────────────────────────────────────────────────────────┐
│                                                           │
│  ┌─────────────────────────────────────────────────┐      │
│  │  Application Layer                              │      │
│  │  ┌───────────────────────────────────────────┐  │      │
│  │  │  Domain Layer                             │  │      │
│  │  │                                           │  │      │
│  │  │  Configuration  Blueprint  Template       │  │      │
│  │  │  Component      Policy      ForgeErr      │  │      │
│  │  │                                           │  │      │
│  │  └───────────────────────────────────────────┘  │      │
│  │                                                 │      │
│  │  Application Services                           │      │
│  │  Renderer  Validator  Update Engine  Version    │      │
│  │                                                 │      │
│  └─────────────────────────────────────────────────┘      │
│                                                           │
│  ┌─────────────────────────────────────────────────┐      │
│  │  Infrastructure Layer                           │      │
│  │  Filesystem  Process  Registry  Logging         │      │
│  │  Output                                         │      │
│  └─────────────────────────────────────────────────┘      │
│                                                           │
└───────────────────────────────────────────────────────────┘
```

### 5.2 Allowed Dependencies

| From | To | Allowed |
|------|----|---------|
| CLI | Application | Yes |
| CLI | Infrastructure (Output, Logging) | Yes |
| CLI | Version Model | Yes (leaf) |
| CLI | ForgeErr | Yes |
| Application | Domain | Yes |
| Application | Infrastructure | Yes |
| Application | Version Model | Yes (leaf) |
| Application | ForgeErr | Yes |
| Domain | Domain (same layer) | Yes, within submodules |
| Infrastructure | Domain | Yes (for shared types) |
| Infrastructure | ForgeErr | Yes |
| Any Forge package | ForgeErr | Yes |
| Any | CLI | **No** |
| Domain | Application | **No** |
| Domain | Infrastructure (I/O) | **No** |
| Domain | Version Model | **No** |
| ForgeErr | Any Forge package | **No** (leaf) |
| Infrastructure | CLI | **No** |
| Infrastructure | Version Model | **No** |
| Version Model | Any Forge package | **No** (leaf) |

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
- **Any Forge package importing the Version Model.** The Version
  Model is a leaf. Nothing in Forge may import it except the two
  consumers named in § 5.2. See § 11.12.
- **Constructing a `*forgeerr.Error` outside the `forgeerr`
  package.** The type's fields are unexported, and the only
  public ways to create a value are `forgeerr.New`,
  `forgeerr.Wrap`, and the `WithX` methods. A caller that
  constructs the struct literally — even if it could — would
  bypass the invariant checks and produce a malformed error.
  Every error in Forge must flow through the constructor API.

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

- **Code review**, using the checklist in
  [`docs/development.md`](./development.md).
- **Package structure** (`internal/` subdirectories).
- **Taskfile target** `verify:cobra:single-import`, which fails if
  Cobra is imported outside `internal/cli`.
- **Go structural tests** in `internal/cli/structure_test.go`
  (`TestCobraImportedOnlyInCliPackage`) that assert the same rule
  from the test side.
- **Future:** a Taskfile target `verify:imports` (Phase 2) that
  greps each package's imports against an allowlist. Until that
  target exists, import-direction enforcement is by review and by
  the two named checks above.

---

## 6. Interface Contracts

Interfaces are the contract between modules. They are defined in the
package that owns the concept they describe (see § 3.3).

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

**Defined in:** `internal/filesystem/filesystem.go`.

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

**Defined in:** `internal/app/logging` (to be created; Phase 2
placeholder is `internal/cli/deps.go`).

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
- Import `github.com/spf13/cobra`.
- Import `internal/version` directly. It reads the metadata through
  a small accessor (`buildInfo` in `internal/app/version/buildinfo.go`)
  that isolates the import to one file. See § 11.12.

The service is documented in § 11.12 as the reference implementation
of the handler / service pattern.

### 6.13 ForgeErr

The `ForgeErr` package does not define an interface. It defines a
concrete type and a fixed set of functions and methods that
operate on it.

#### The type

```go
// internal/forgeerr/forgeerr.go
type Error struct {
    code        string
    message     string
    category    Category
    cause       error
    context     map[string]string
    remediation string
}
```

All fields are unexported. The type is immutable after
construction and safe for concurrent use. `*Error` implements
`error`, `Unwrap`, `Is`, and `As`.

#### The constructors

```go
// internal/forgeerr/constructors.go
func New(category Category, code, message string) *Error
func Wrap(category Category, code, message string, cause error) *Error

// Fluent builders; each returns a copy of the receiver.
func (e *Error) WithCause(cause error) *Error
func (e *Error) WithContext(key, value string) *Error
func (e *Error) WithRemediation(remediation string) *Error
```

#### The accessors

```go
func (e *Error) Code() string
func (e *Error) Message() string
func (e *Error) Category() Category
func (e *Error) Context() map[string]string
func (e *Error) Remediation() string
```

#### The taxonomy

```go
// internal/forgeerr/category.go
type Category string

const (
    CategoryCLI        Category = "CLI"
    CategoryConfig     Category = "CONFIG"
    CategoryFilesystem Category = "FILESYSTEM"
    CategoryValidation Category = "VALIDATION"
    CategoryTemplate   Category = "TEMPLATE"
    CategorySecurity   Category = "SECURITY"
    CategoryInternal   Category = "INTERNAL"
)

func AllCategories() []Category
func (c Category) Prefix() string
func (c Category) ExitCode() int
func (c Category) String() string
```

#### The code registry

```go
// internal/forgeerr/codes.go
type Code string

const (
    CodeCLIUnknownCommand Code = "FORGE_CLI_UNKNOWN_COMMAND"
    // ... 18 more constants
)

func AllCodes() []Code
func Lookup(c Code) (introduced string, deprecated, ok bool)
func IsRegistered(c Code) bool
func (c Code) Category() (Category, bool)
```

#### The wrapping contract

```go
func (e *Error) Unwrap() error
func (e *Error) Is(target error) bool
func (e *Error) As(target any) bool
```

The semantics of these three methods are frozen in WBS 10.3.2 and
documented in § 8.5.

#### The user-facing format

```go
// internal/forgeerr/format.go
type Options struct {
    NoColor bool
    Redact  bool
    Verbose bool
    Stack   []string
}

func Format(err error, opts Options) string
```

`Format` renders a `*Error` (or an error that wraps one) into the
frozen user-facing format. Its rules (F1–F12) are documented in
§ 8.7. The verbose mode is documented in § 8.13. The redaction
policy is documented in § 8.12.

#### The log format

```go
// internal/forgeerr/log.go
func (e *Error) LogValue() slog.Value
```

`LogValue` implements `slog.LogValuer`, making
`slog.Any("error", err)` produce a structured entry. Its shape is
documented in § 8.11.

#### The redaction policy

```go
// internal/forgeerr/redact.go
func redact(key, value string) string
func redactPath(path string) string
func redactPathFor(path string, verbose bool) string
func AllSecretKeys() []string
```

`redact` and `redactPath` implement the frozen policy; `AllSecretKeys`
returns a copy of the secret-keys list. The policy is documented in
§ 8.12.

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

Every stage in the pipeline produces errors through the
`forgeerr` package. A stage that detects a failure returns a
`*forgeerr.Error` constructed with `forgeerr.New` (for errors the
stage itself produced) or `forgeerr.Wrap` (for errors that wrap a
cause). The error flows up the pipeline to the CLI's boundary,
where it is:

1. Recognized (or wrapped, if it is a Cobra error) by
   `wrapCobraError` in `executeWithOptions`.
2. Rendered by `forgeerr.Format`.
3. Written to the injected stderr.
4. Mapped to a process exit code by `exitCodeOf`, which reads
   `Category.ExitCode()`.

No stage constructs a `*forgeerr.Error` literally, and no stage
returns a bare `errors.New`. See § 8 for the error architecture,
§ 11.7 for the boundary's error path, and § 11.16 for the CLI's
error contract.

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
  → version.Get()      (pure; reads internal/version via buildInfo)
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

- **Structured.** Every error is a `*forgeerr.Error` with a
  stable code, a category, a message, an optional cause, optional
  structured context, and an optional remediation.
- **Categorised.** Every error belongs to exactly one of seven
  categories (CLI, CONFIG, FILESYSTEM, VALIDATION, TEMPLATE,
  SECURITY, INTERNAL).
- **Registered.** Every code is a member of a closed registry.
  The registry is the single source of truth for the set of codes
  Forge emits.
- **Stable.** Codes and categories are frozen once shipped.
  Adding a code requires an ADR (per ADR-014). Adding a category
  requires an ADR (per ADR-013).
- **Actionable.** The error carries an optional remediation: a
  suggested action that the user can take.
- **Traceable.** The error carries an optional cause, reachable
  through `errors.Is` and `errors.As`.
- **Safe.** The error's context never contains secrets. The
  secret pattern list is defined in
  [`docs/security-model.md`](./security-model.md) § "Secret
  patterns" and applied by the redaction layer (§ 8.12).

### 8.2 The Type

The canonical error type lives in
`internal/forgeerr/forgeerr.go`. Its fields are unexported; its
constructors and accessors are documented in § 6.13. The type is
immutable, safe for concurrent use, and compatible with the
standard library's chain protocols.

The type's design is deliberately narrow. It knows nothing about
the CLI, the application services, or the infrastructure. It
depends only on the Go standard library.

### 8.3 The Category Taxonomy

The closed set of categories is defined in
`internal/forgeerr/category.go`. Each category carries:

- **A Code prefix**, exposed as `Category.Prefix()`. A code's
  category is derivable from its prefix; the constructor verifies
  the derivation at construction time.
- **A process exit code**, exposed as `Category.ExitCode()`. The
  values are frozen in WBS 4.1.2.

| Category | Prefix | Exit code | Scope |
|---|---|---|---|
| `CLI` | `FORGE_CLI_` | 2 | CLI usage, flags, commands |
| `CONFIG` | `FORGE_CONFIG_` | 3 | Configuration loading and validation |
| `FILESYSTEM` | `FORGE_FILESYSTEM_` | 4 | File I/O, paths, permissions |
| `VALIDATION` | `FORGE_VALIDATION_` | 5 | Schema, blueprint, check validation |
| `TEMPLATE` | `FORGE_TEMPLATE_` | 1 | Template resolution and rendering |
| `SECURITY` | `FORGE_SECURITY_` | 6 | Path traversal, secrets, boundaries |
| `INTERNAL` | `FORGE_INTERNAL_` | 1 | Bugs, unexpected panics |

The taxonomy is **closed**: adding a category requires an ADR
(ADR-013). `VALIDATION` and `TEMPLATE` are declared as categories
but have no registered codes in Phase 2; the pending list is
tracked in `internal/forgeerr/codes_test.go` as
`pendingCategories`, and the reference doc duplicates it as
"Pending categories" in
`docs/reference/error-categories.md`.

### 8.4 The Code Registry

The closed set of codes is defined in
`internal/forgeerr/codes.go`. Each code is a member of exactly one
category, derivable from its prefix. The registry is the single
source of truth for the set of codes Forge emits; the constructors
reject any code that is not registered.

The registry's contents are frozen in WBS 10.3.1. Adding a code
requires:

1. A PR that touches `internal/forgeerr/codes.go`.
2. A regenerated `internal/forgeerr/testdata/codes/codes.golden.txt`.
3. An updated `docs/reference/error-codes.md`.
4. An ADR (per ADR-014).

Codes are never renamed, never reused for different meanings, and
never removed. A code that has been superseded is marked
deprecated and remains in the registry. As of Phase 2, no code is
deprecated.

### 8.5 The Wrapping Contract

The semantics of `Unwrap`, `Is`, and `As` on `*Error` are frozen
in WBS 10.3.2 (ADR-015).

| Method | Semantics |
|---|---|
| `Unwrap() error` | Returns the immediate cause, or nil if none. Does not traverse the chain. |
| `Is(target error) bool` | Returns true if `target` is a `*Error` with the same `Code`. Does not compare messages, categories, or causes. Does not traverse the target's chain. |
| `As(target any) bool` | Assigns the receiver to a `**Error` target and returns true; returns false for any other target type. Does not traverse the chain. |

The standard library's `errors.Is` and `errors.As` are the
traversal engines: they walk the chain by calling `Unwrap`
repeatedly and invoking the receiver's `Is` or `As` method at each
level. The methods themselves are per-link operations.

Rules:

1. **`Is` matches by Code, not by pointer identity and not by
   message.** Two `*Error` values with the same Code match; two
   values with different Codes do not.
2. **`Is` does not match by message.** Messages may change;
   codes are stable.
3. **`As` assigns the entire `*Error`.** Callers can inspect the
   `Category`, `Context`, and `Remediation` of the error.
4. **No sentinels.** Forge does not export sentinel values.
   Callers use `errors.As` to extract the `*Error` and compare
   its `Code`.
5. **`errors.Join` is supported.** A `*Error` participates in a
   joined error as an ordinary operand.
6. **`fmt.Errorf("%w", ...)` preserves the code.** The wrapper is
   traversed by `errors.Is` and `errors.As` via the wrapper's own
   `Unwrap` method.
7. **Double-wrapping is impossible.** `forgeerr.Wrap` returns
   the existing `*Error` unchanged if the cause is already a
   `*Error`.

### 8.6 Error Construction

Every error is constructed through the frozen constructor API
(§ 6.13). The constructors validate the category, the code shape,
the code's registry membership, the code's prefix, the message,
and (where applicable) the remediation. A violation is a
programming error and panics.

Two constructors exist:

- `New(category, code, message)` creates an error without a
  cause. Use it for errors the program itself produced.
- `Wrap(category, code, message, cause)` creates an error with a
  cause, or returns the cause unchanged if it is already a
  `*Error`. Use it when wrapping a library error, an `os` error,
  or any error whose original type or value a caller might want
  to inspect via `errors.Is` or `errors.As`.

Three fluent builders extend a `*Error`:

- `WithCause(cause)` attaches a cause.
- `WithContext(key, value)` adds a structured key-value pair.
- `WithRemediation(remediation)` attaches a suggested action.

Each builder returns a copy; the receiver is never mutated.

### 8.7 The User-Facing Format

The user-facing format is frozen in WBS 10.4.1 and implemented by
`forgeerr.Format` in `internal/forgeerr/format.go`. The format is
byte-exact and enforced by the golden files in
`internal/forgeerr/testdata/format/`.

The non-verbose format is:

```text
Error: <message>
Code:  <code>
       <context-key-1>: <value-1>
       <context-key-2>: <value-2>

Suggestion:
  <remediation>
```

The byte-level rules:

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

The `*Error.Error()` method is **programmer-facing**: it renders
`<code>: <message>` and nothing else. It is intended for logs and
debugging. It is not the user-facing format.

**Example (non-verbose):**

```text
Error: invalid YAML
Code:  FORGE_CONFIG_MALFORMED
       file: config.yaml
       line: 12

Suggestion:
  Fix the YAML syntax at line 12.
```

### 8.8 Exit Code Mapping

| Category | Exit code | Constant |
|---|---|---|
| (success) | 0 | `ExitSuccess` |
| `TEMPLATE` | 1 | `ExitFailure` |
| `CLI` | 2 | `ExitUsage` |
| `CONFIG` | 3 | `ExitConfig` |
| `FILESYSTEM` | 4 | `ExitFilesystem` |
| `VALIDATION` | 5 | `ExitValidation` |
| `SECURITY` | 6 | `ExitSecurity` |
| `INTERNAL` | 1 | `ExitFailure` |

The constants are defined in `internal/cli/exitcodes.go`. The
mapping is pinned by `TestAC4_ExitCodesMatchCLIConstants` in
`internal/forgeerr/category_test.go`. If WBS 4.1.2 changes, the
test fails and both the constants and the `switch` in
`Category.ExitCode` are updated in the same commit.

### 8.9 Error Presentation

The user-facing presentation of an error is a concern of
`forgeerr.Format` (§ 8.7), not of the type. The `*Error.Error()`
method is programmer-facing.

The CLI's boundary calls `forgeerr.Format` and writes the result
to the injected stderr. The boundary is documented in § 11.7. The
categorisation and exit-code derivation are documented in § 8.8.

### 8.10 Secret Redaction

Redaction is applied by `forgeerr` before any value reaches a user
or a log consumer. The policy is frozen in WBS 10.4.3 and
implemented in `internal/forgeerr/redact.go`. See § 8.12 for the
details.

### 8.11 ForgeError Logging

The `*Error` type implements `slog.LogValuer` (WBS 10.4.2). The
implementation is in `internal/forgeerr/log.go`. The interface is
what makes `slog.Any("error", err)` produce a structured entry
rather than a single formatted string.

The value returned by `LogValue` is a `slog` group named
`"error"`. The group contains:

| Attribute | Source | Presence |
|---|---|---|
| `error.code` | `e.Code()` | Always. |
| `error.category` | `e.Category()` | Always. |
| `error.cause` | `e.Unwrap().Error()` | Only when a cause is attached. |
| `error.<key>` | one per `e.Context()` entry | When the context is non-empty. |

The context keys are sorted alphabetically, so that the output is
deterministic across runs. The context values are redacted
(§ 8.12) before they are appended.

**Text rendering:**

```text
level=error msg=failed error.code=FORGE_CONFIG_MALFORMED error.category=CONFIG error.cause="yaml: line 12" error.file=config.yaml error.line=12
```

**JSON rendering:**

```json
{"level":"error","msg":"failed","error":{"code":"FORGE_CONFIG_MALFORMED","category":"CONFIG","cause":"yaml: line 12","file":"config.yaml","line":"12"}}
```

The log format does not include the error's message (it is the
primary log message), the remediation, or the user-facing
wrapper. The code, category, and cause are the log consumer's
concerns; the message and remediation are the user's.

### 8.12 Redaction

The redaction policy is frozen in WBS 10.4.3 and implemented in
`internal/forgeerr/redact.go`. Both `Format` (§ 8.7) and `LogValue`
(§ 8.11) apply it, so the two outputs cannot diverge on what is
redacted.

The rules:

| Input | Output |
|---|---|
| A context value whose key is `token`, `password`, `secret`, `key`, `credential`, or `auth` | `<redacted>` |
| A context value that is a path beginning with the user's home directory | The same path with the home prefix replaced by `~` |
| A context value matching neither rule | Unchanged |

The key match is case-insensitive and word-boundary-based: the key
is split on `_`, `-`, and `.`, and each segment is compared to the
frozen list. `api_token` and `API_TOKEN` match; `monkey`,
`tokens`, and `author` do not.

The redaction layer does **not** redact the message, the code, the
category, or the remediation. The constructor authors them; it is
responsible for not placing secrets in them. The redaction layer
also does **not** redact the cause; the cause is a programmer
concern, rendered only by the log format as a field.

The frozen list is exposed via `AllSecretKeys`, which returns a
defensive copy. Adding a key requires an ADR.

### 8.13 Verbose and Non-Verbose Modes

The verbose mode is frozen in WBS 10.4.4 and selected by
`Options.Verbose`. The non-verbose mode is the frozen WBS 10.4.1
format; a caller that passes the zero-value `Options` sees exactly
the WBS 10.4.1 output.

The verbose mode adds three elements the non-verbose mode omits:

| Element | Non-verbose | Verbose |
|---|---|---|
| Error message | Shown | Shown |
| Error code | Shown | Shown |
| Context values (secret) | `<redacted>` | `<redacted>` |
| Context values (home path) | `~/...` | Full path |
| Context values (other) | Shown | Shown |
| `internal_` context keys | Filtered out | Shown |
| Cause chain | Not shown | Shown as a `Cause:` block |
| Stack trace | Not shown | Shown as a `Stack:` block |

The `Cause:` and `Stack:` blocks are inserted between the context
lines and the `Suggestion:` block. The `Stack:` block is omitted
when no stack is provided.

Secret redaction is unchanged in both modes. The rule "Secrets
are never shown" is absolute.

### 8.14 Boundary Integration

The CLI's error boundary is frozen in WBS 10.5.1. The boundary
performs three jobs:

1. **Panic recovery.** `Execute` recovers panics, converts them to
   `FORGE_INTERNAL_PANIC` errors via `recoveredPanicError`, and
   renders them through the same pipeline as any other error.

2. **Non-`*forgeerr.Error` wrapping.** `executeWithOptions` calls
   `wrapCobraError` to convert Cobra's parser and dispatcher
   errors to `CategoryCLI` errors. It then wraps any error that is
   still not a `*forgeerr.Error` via `wrappedUnexpectedError`.

3. **Exit-code derivation.** `exitCodeOf` extracts the
   `*forgeerr.Error` from the chain and reads
   `Category.ExitCode()`.

The boundary's design and its relationship to the two-boundary
model are documented in § 11.7 and § 11.16.

### 8.15 Reference Implementation

The `forgeerr` package itself is the reference implementation of
the error architecture. Its tests — `forgeerr_test.go`,
`constructors_test.go`, `category_test.go`, `codes_test.go`,
`wrap_test.go`, `format_test.go`, `log_test.go`, `redact_test.go`
— are the specification of its behaviour. See § 11.16 for the
CLI's use of the package and § 13.2 for the package's file list.

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

When the logged value is a `*forgeerr.Error`, the JSON format
includes the structured error group documented in § 8.11.

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

The secret pattern list is defined once, in
`docs/security-model.md` § "Secret patterns", and is the same list
used by the error layer (§ 8.12). Do not maintain a second list.

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
┌─────────────────────────────────────────────────────────────────┐
│                          USER / CI                              │
└──────────────────────────────┬──────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                          CLI LAYER                              │
│                                                                 │
│  ┌──────────────┐   Pre-parse validation (§ 11.15)              │
│  │ validateArgs │   Rejects malformed invocations               │
│  └──────┬───────┘                                               │
│         │                                                       │
│         ▼                                                       │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐            │
│  │ new     │  │ init    │  │ validate│  │ update  │  ...       │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘            │
│       │            │            │            │                  │
│       └────────────┴────────────┴────────────┘                  │
│                              │                                  │
└──────────────────────────────┼──────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                       APPLICATION LAYER                         │
│                                                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │ Renderer │  │ Validator│  │ Update   │  │ Explain  │        │
│  │          │  │          │  │ Engine   │  │ Service  │        │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘        │
│       │             │             │             │               │
│  ┌────┴─────────────┴─────────────┴─────────────┴─────┐         │
│  │ Version Service                                     │         │
│  └─────────────────────────────────────────────────────┘         │
│                                                                 │
└──────────┬──────────────┬──────────────┬──────────────┬─────────┘
           │              │              │              │
           └──────────────┴──────────────┴──────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                          DOMAIN LAYER                           │
│                                                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │Blueprint │  │Template  │  │Component │  │Policy    │        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐                             │
│  │Configuration │  │ ForgeErr     │                             │
│  └──────────────┘  └──────────────┘                             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                         ▲
                         │ (implements interfaces)
                         │
┌─────────────────────────────────────────────────────────────────┐
│                     INFRASTRUCTURE LAYER                        │
│                                                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │Filesystem│  │ Process  │  │ Registry │  │ Logging  │        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
│                                                                 │
│  ┌──────────┐                                                   │
│  │ Output   │                                                   │
│  └──────────┘                                                   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

                    ┌─────────────────────┐
                    │  LEAF: Version      │
                    │  (build metadata)   │
                    │  imported by CLI    │
                    │  and Application    │
                    └─────────────────────┘
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
- **The Version Model is a leaf package** — it is drawn outside the
  three layers because it belongs to none of them. It is imported by
  `internal/cli` and `internal/app/version` and by nothing else. See
  § 11.12.
- **ForgeErr is a Domain-layer leaf** — it sits inside the Domain
  layer because every other module produces errors of its type. It
  is a leaf: nothing it does requires another Forge package, and
  every Forge package may import it. See § 4.2.18.
- **Pre-parse validation is the first stage** — the CLI's pipeline
  begins with `validateArgs`, which rejects malformed invocations
  before the command tree runs. The stage is documented in § 11.15.

### 10.2 Alternative View: By Concern

```text
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│  USER-FACING                                                    │
│  ├── CLI commands                                               │
│  ├── Human output                                               │
│  └── Machine output (JSON, SARIF)                               │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ORCHESTRATION                                                  │
│  ├── Application services (New, Init, Validate, Check, Diff,    │
│  │   Update, Explain, Version)                                  │
│  ├── Renderer                                                   │
│  ├── Validator                                                  │
│  └── Update Engine                                              │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  CORE LOGIC                                                     │
│  ├── Blueprint (parse, validate, default)                       │
│  ├── Template (parse, resolve, plan)                            │
│  ├── Component (resolve, compose, conflict)                     │
│  ├── Policy (parse, evaluate)                                   │
│  ├── Configuration (load, merge)                                │
│  └── ForgeErr (type, taxonomy, registry, wrapping, format,      │
│               log, redaction, verbose modes)                    │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  FOUNDATIONS                                                    │
│  ├── Filesystem (boundary, atomic writes)                       │
│  ├── Process (Git, subprocess)                                  │
│  ├── Registry (fetch, verify)                                   │
│  ├── Logging                                                    │
│  ├── Output (formatting)                                        │
│  └── Version (build metadata)                                   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 11. The Two-Boundary Execution Model

### 11.1 Overview

The Forge CLI is structured around **two boundaries** and **two
structs**, each serving a distinct purpose. This model is established
by WBS 4.2.2.

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

The public entry point is a one-line wrapper (the named return is
a consequence of the panic-recovery path, § 11.7):

```go
func Execute() (code int) {
    defer func() {
        if r := recover(); r != nil {
            // recover, convert, render, write
            code = forgeerr.CategoryInternal.ExitCode()
        }
    }()
    return executeWithOptions(defaultOptions())
}
```

Every behaviour a user can observe is implemented in
`executeWithOptions` or below it.

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
    config   *config.Config
    verbose  bool
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
| `config` | Test-injection seam for the config value. | `nil` (production) |
| `verbose` | Verbose mode for the error rendering (WBS 10.4.4). | `false` |

`options` is constructed by:

- `defaultOptions()` in production, which reads the process's actual
  `os.*` values.
- A test, which constructs a synthetic environment.

#### 11.2.2 `Dependencies` — The Command Boundary

```go
type Dependencies struct {
    Config config.Config
    Logger Logger
    FS     filesystem.Filesystem
    Stdout io.Writer
    Stderr io.Writer
    Env    func(string) string
}
```

Each field is a resolved collaborator that a command may use.

| Field | Purpose | Phase 2 placeholder |
|-------|---------|---------------------|
| `Config` | Resolved configuration for this invocation. | `config.Config{}` (zero value) |
| `Logger` | Diagnostic logger. | `newLogger(stderr)` (slog-backed). |
| `FS` | Filesystem abstraction. | `filesystem.NewOSFS(rootPath)`. |
| `Stdout` | Destination for successful output. | `opts.stdout`. |
| `Stderr` | Destination for diagnostics. | `opts.stderr`. |
| `Env` | Environment variable lookup. | `opts.env`. |

**Why `Config` is a value, not a pointer.** Five of the six fields
are values or interfaces. Making `Config` a pointer would introduce
a `nil` state that is indistinguishable from "config not loaded",
and would make `Dependencies` non-comparable in tests that want to
assert equality. A `config.Config` value has a valid zero value
(the empty configuration), which is what Phase 2 uses when no
`forge.yaml` is present.

If a future phase needs to distinguish "no config file" from "empty
config file", it adds a field (`ConfigSource string` or similar)
rather than changing the pointer-ness of `Config`.

`Dependencies` is constructed by `buildDependencies(opts)` —
**exactly once per invocation**, from `executeWithOptions`.

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
process ──▶ options ──▶ buildDependencies ──▶ Dependencies ──▶ commands
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
        // render and map the validation error
    }
    deps := buildDependencies(opts)   // the single transformation
    root := newRootCmd(deps)          // Dependencies, not options
    root.SetArgs(opts.args)
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
   struct. The panic-recovery path in `Execute` is a documented
   exception: it writes to `os.Stderr` directly, because the injected
   writers are not trustworthy during a panic. See § 11.7.

2. **`buildDependencies` is called exactly once in production code,
   from `executeWithOptions`.** No other function constructs a
   `Dependencies` value.

Both invariants are enforced by:

- **Taskfile target** `verify:two-boundary`, which greps the source
  tree for violations (see `Taskfile.yml`).
- **Code review**, using the checklist in
  [`docs/development.md`](./development.md).
- **Future:** Go structural tests in
  `internal/cli/execute_test.go` that assert the same invariants
  from the test side. The tests are not yet written; this document
  records them as required before Phase 2 exit.

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

`Execute` is the process's error boundary. Every error reaching it
is a `*forgeerr.Error` or wraps one; every panic below it is
recovered and converted to a `*forgeerr.Error`. The boundary
performs three jobs:

1. **Panic recovery.** A panic is recovered, converted to a
   `FORGE_INTERNAL_PANIC` error via `recoveredPanicError`, rendered
   with `forgeerr.Format`, written to `os.Stderr`, and mapped to
   the INTERNAL category's exit code (1).

2. **Cobra-error wrapping and boundary wrapping.**
   `executeWithOptions` first calls `wrapCobraError` to convert
   Cobra's parser and dispatcher errors (unknown flags, unknown
   commands) to `CategoryCLI` errors. It then wraps any error that
   is still not a `*forgeerr.Error` via `wrappedUnexpectedError`.

3. **Exit-code derivation.** `exitCodeOf` extracts the
   `*forgeerr.Error` from the chain and returns
   `Category.ExitCode()`.

The `Execute` shape:

```go
func Execute() (code int) {
    defer func() {
        if r := recover(); r != nil {
            e := recoveredPanicError(r)
            fmt.Fprint(os.Stderr, forgeerr.Format(e, forgeerr.Options{
                Verbose: hasVerboseArg(os.Args),
            }))
            code = e.Category().ExitCode()
        }
    }()
    return executeWithOptions(defaultOptions())
}
```

The `executeWithOptions` error path:

```go
err := root.Execute()
if err == nil {
    return ExitSuccess
}

// Convert Cobra's parser and dispatcher errors.
err = wrapCobraError(err)

// Wrap any remaining non-*forgeerr.Error.
var ferr *forgeerr.Error
if !errors.As(err, &ferr) {
    err = wrappedUnexpectedError(err)
}

verbose := verboseRequested(root)
fmt.Fprint(opts.stderr, forgeerr.Format(err, forgeerr.Options{
    Verbose: verbose,
}))
return exitCodeOf(err)
```

The pre-parse validation stage uses the same rendering and
exit-code derivation. A malformed invocation produces a
`*forgeerr.Error` from `validateArgs`, which is rendered by the
same `forgeerr.Format`, written to the same stderr, and mapped to
its category's exit code.

The two helper functions (`formatError`, `exitCodeFromError`) that
the earlier version of this document described have been **deleted**
in WBS 10.5.1. Their replacements are `forgeerr.Format` and
`Category.ExitCode()`; the boundary's local helpers
(`recoveredPanicError`, `wrappedUnexpectedError`, `exitCodeOf`)
encapsulate the boundary's specific behaviour.

### 11.8 The Output Path

Successful output is written by the command tree itself, to
`opts.stdout` (which is `deps.Stdout`). `executeWithOptions` does not
write to `opts.stdout`; it only wires it to the command tree.

This preserves the standard Unix convention: successful output goes
to stdout, diagnostics and errors go to stderr.

### 11.9 Related Files

| File | Purpose |
|------|---------|
| `internal/cli/execute.go` | `Execute`, `executeWithOptions`, `options`, `defaultOptions`, `recoveredPanicError`, `wrappedUnexpectedError`, `wrapCobraError`, `parseUnknownCommandMessage`, `hasVerboseArg`, `exitCodeOf`. |
| `internal/cli/deps.go` | `Dependencies`, `Logger`, `slogLogger`, `noopLogger`, `newLogger`, `buildDependencies`. |
| `internal/cli/deps_test.go` | White-box tests for `Dependencies` and its construction. |
| `internal/cli/execute_test.go` | White-box tests for the process boundary, the transformation, the panic-recovery conversion, the non-`*Error` wrapping, and the exit-code derivation. |
| `internal/cli/exitcodes.go` | Exit code constants. |
| `internal/cli/root.go` | The root command constructor. Accepts a `Dependencies` value and registers subcommands. |
| `internal/cli/registry.go` | The central command registry (§ 11.13). |
| `internal/cli/flags.go` | The global flag registration and readers (§ 11.14). |
| `internal/cli/validate.go` | The pre-parse argument validator (§ 11.15). |
| `internal/cli/errors.go` | The Cobra-error constructor (`unknownCommandError`), the suggestion distance constant, and the shared `suggestionsMinimumDistance` value. |
| `internal/cli/version.go` | The `forge version` handler. Reference implementation of the handler / service pattern (§ 11.12). |
| `internal/cli/version_test.go` | White-box tests for the version handler, including the structural tests that enforce § 11.12 and the service-symbol allowlist. |
| `internal/cli/structure_test.go` | Black-box structural tests for the package's shape. |
| `internal/cli/registry_test.go` | Tests for the command registry. |
| `internal/cli/contract_test.go` | Structural tests for the command constructor contract. |
| `internal/cli/validate_test.go` | Unit tests for the pre-parse argument validator. |
| `internal/cli/flags_test.go` | Unit tests for the global flag readers and the log-level resolver. |
| `internal/cli/errors_test.go` | The invalid-command matrix (I1–I7), the typo suggestion test, and the suggestion-distance test. |
| `internal/cli/config_interaction_test.go` | The `--config` interaction tests. |
| `internal/cli/flag_interaction_test.go` | The `--quiet` / `--verbose` / `--config` interaction matrix. |
| `internal/cli/testhelper_test.go` | The shared `runCLI` helpers, the `testRun` struct, the golden-file flag, and the test-only registry hooks. |
| `internal/app/version/service.go` | The version application service. Pure. |
| `internal/app/version/format.go` | The version formatter. Takes an `io.Writer`. |
| `internal/app/version/buildinfo.go` | Reads build metadata from `internal/version`. |
| `internal/app/version/service_test.go` | Unit tests for the version service, without Cobra. |
| `internal/app/version/format_test.go` | Tests for the frozen format. |
| `internal/app/version/buildinfo_test.go` | Tests for the accessor. |
| `internal/version/version.go` | The build metadata variables and their accessor. Leaf package; imported by `internal/cli` and `internal/app/version`. |
| `internal/version/version_test.go` | Unit tests for the metadata accessor. |
| `internal/version/injection_test.go` | Tests for the linker injection contract (WBS 6.1.2). |
| `cmd/forge/main.go` | The process entry point. Calls `Execute()` and forwards its return to `os.Exit`. |
| `cmd/forge/binary_integration_test.go` | Process-boundary tests. |

### 11.10 Rules for Extending the Model

1. **`Execute` remains a wrapper.** Its body contains only the
   panic recovery and the delegation to `executeWithOptions`.
   Every other behaviour belongs in `executeWithOptions` or below.

2. **`executeWithOptions` reads only from `opts`.** It must not read
   `os.Args`, `os.Stdin`, `os.Stdout`, `os.Stderr`, or `os.Getenv`.
   This invariant is enforced by the `verify:two-boundary` Taskfile
   target and by
   `TestExecuteWithOptions_NoProcessStreamsReferenced` in
   `execute_test.go`.

3. **`buildDependencies` is the only function that constructs a
   `Dependencies` value.** A struct literal of the form
   `Dependencies{...}` must not appear outside `deps.go` in
   production code. This invariant is enforced by the
   `verify:two-boundary` Taskfile target and by
   `TestExecuteWithOptions_DoesNotConstructDependenciesByHand` in
   `execute_test.go`.

4. **`newRootCmd` accepts `Dependencies`, not `options`.** A wrapper
   with the old signature must not be added: it would create a second
   construction path and violate rule 3.

5. **The boundary's error path uses `forgeerr.Format` and
   `Category.ExitCode()`.** The legacy helpers `formatError` and
   `exitCodeFromError` are deleted. The boundary's local helpers
   (`recoveredPanicError`, `wrappedUnexpectedError`, `exitCodeOf`)
   encapsulate the boundary's specific behaviour; they are not
   general-purpose renderers.

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
  `defaultOptions`, and the panic path's direct write to `os.Stderr`
  is a documented exception.

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

#### What the service must not import

Three independent constraints, each enforced separately:

| Constraint | Enforcement |
|------------|-------------|
| The service must not import `github.com/spf13/cobra`. | `verify:cobra:single-import` Taskfile target; `TestCobraImportedOnlyInCliPackage` in `structure_test.go`. |
| The service must not import `internal/cli`. | Code review; a future `verify:imports` target. |
| The service must not receive a `Dependencies` value. | The constructor signature of `newVersionCmd` takes `Dependencies` at the handler level, not at the service level. The service's exported functions take plain parameters. |

The distinction matters: "the service knows nothing about Cobra" is
a vibe; the three constraints above are checkable properties.

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

#### Line-count enforcement

The handler body line limit (20 lines) is enforced by the
`verify:handler-boundary:line-count` Taskfile target. The target
extracts the `RunE` closure by scanning for the `RunE:` line and
the matching closing brace. The extraction relies on
gofmt-canonical formatting: `RunE:` on its own line, and the closing
`},` at the same indentation as the `RunE:` line. A handler that is
formatted differently (for example, with the closure on one line,
or with the closing brace indented differently) will be reported as
"could not locate RunE closure" rather than "over the limit". This
is intentional: the check prefers a false negative to a false
positive, and the extraction is documented in the Taskfile's
comment for that target.

#### The build-metadata leaf package

The version service needs four values — `Version`, `Commit`,
`BuildDate`, `Dirty` — that are injected at link time. Before WBS
4.3.1, the first three lived in `internal/cli`. That created an
import cycle:

```text
internal/cli  ──────▶  internal/app/version
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

The leaf package's public surface is four exported `var`s and one
exported function:

```go
// internal/version/version.go
var Version string
var Commit string
var BuildDate string
var Dirty string

func Get() (version, commit, buildDate, dirty string)
```

The four variables are the only `-X` write targets in the module.
Their names are frozen. The `Get` accessor returns the four values
as a tuple. See WBS 6.1.1 and WBS 6.1.2 for the model and the
linker injection contract.

#### Import isolation in the service

The version service does not import `internal/version` directly.
It reads the metadata through a small accessor,
`internal/app/version/buildinfo.go`, whose only job is to isolate
the import to one file:

```go
// internal/app/version/buildinfo.go
func buildInfo() (version, commit, buildDate, dirty string) {
    return forgeversion.Get()
}
```

This has two benefits:

1. **The import appears in one place.** A reader of `service.go`
   sees a pure `Get` that reads no Forge packages except the local
   call to `buildInfo`.
2. **The seam is refactorable.** If a future change moves the
   metadata source (for example, to `runtime/debug.ReadBuildInfo`),
   only `buildinfo.go` changes. `Get` and its callers are unchanged.

`TestNonFunctional_BuildInfoIsolatesTheImport` in
`internal/app/version/buildinfo_test.go` pins the property: the
import line must be in `buildinfo.go` and must not be in
`service.go`.

#### Enforcement

The boundary is enforced by:

- **Structural tests** in `internal/cli/version_test.go` that assert
  the handler body contains exactly one service call and no
  forbidden tokens (`os.Stdout`, `os.Exit`, `fmt.Fprintln`, ...).
  The tests strip comments before searching, so that the handler's
  docstring may name the forbidden tokens while explaining that the
  handler does not use them.
- **The `verify:handler-boundary` Taskfile target**, which runs
  the line-count check, the direct-I/O check, the `os.Exit` check,
  the filesystem-I/O check, and the leaf-package check.
- **The `verify:cobra:single-import` Taskfile target**, which
  enforces the "no Cobra outside `internal/cli`" rule.
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

The complete procedure is documented in the package docstring of
`registry.go` and repeated here so that a contributor reading the
architecture document does not have to find the file:

1. Create `internal/cli/<name>.go`. The file must define exactly
   one constructor with the signature
   `func new<Name>Cmd(deps Dependencies) *cobra.Command`.
2. Append `new<Name>Cmd` to the registry slice in the correct
   position (alphabetical within the user-facing group; see the
   ordering convention in `registry.go`).
3. Add the file to `expectedSourceFiles` in
   `internal/cli/structure_test.go`.
4. Add the file to `HANDLER_FILES` in `Taskfile.yml` so that
   `verify:handler-boundary` checks it.
5. Add tests: handler tests in `internal/cli/<name>_test.go`,
   service tests in `internal/app/<name>/`.
6. If the command is user-facing, add its contract to
   `docs/cli-ux-spec.md`.

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
- **Code review**.

### 11.14 Global Flags

WBS 5.3.1 establishes the CLI's global flag inventory. The inventory
is deliberately small: three flags in Phase 2, each with a documented
consumer in a later WBS item.

#### The inventory

| Flag | Short | Type | Persistent | Consumer | Semantics |
|------|-------|------|------------|----------|-----------|
| `--verbose` | — | bool | yes | WBS 12.0 (logging), WBS 10.4.4 (error rendering) | Set log level to DEBUG; enable verbose error rendering |
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
and `forge config --verbose` are equivalent.

#### Precedence

When `--verbose` and `--quiet` are both set, `--quiet` wins for the
log level. The effective log level is ERROR.

The rule is implemented in `resolveLogLevel`. The precedence is not
accompanied by a runtime warning: a warning about conflicting flags
is itself output, and the user who asked for quiet asked for less
output.

The `--verbose` flag's *error rendering* consumer is distinct from
its *log level* consumer. `--quiet` suppresses diagnostic logs but
does not suppress the error block (§ 10.4.4 of the CLI UX spec);
the error block is always rendered.

#### Why the flags are not in `Dependencies`

The global flags are read from the command (`cmd.Flags()`), not from
`Dependencies`. The flags are a property of the invocation, and
Cobra's parser is the source of truth for their values. The
`Dependencies` struct holds resolved collaborators, not parsed
flags.

#### Adding a global flag

Adding a global flag requires an ADR. The ADR must name the consumer
WBS item, define the flag's semantics, and define its precedence
relative to the existing flags.

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

- `validateArgs(args []string) error` — the entry point.
- `validateHelpFlagWithArgs(args)` — rejects shape 1.
- `validateVersionFlagWithArgs(args)` — rejects shape 2.
- `validateConfigFlagWithArgs(args)` — rejects shape 3.
- `validateHelpTopic(args)` — rejects shape 4.
- `isKnownCommandName(name)` — the predicate used by shape 4.
- `commandName(use string) string` — extracts a command's name.
- `validationDependencies()` — the `Dependencies` value used to
  construct commands during validation.

The validator returns `*forgeerr.Error` values with category
`CategoryCLI` and code `CodeCLIInvalidArg` (for the flag-shape
rejections) or `CodeCLIUnknownCommand` (for the unknown help topic).
The boundary renders them like any other error.

#### Why the validator is at the pre-parse stage

Cobra's `--help` and `--version` flags are **interception flags**:
when either is set, Cobra short-circuits the hook chain and runs its
help or version path directly. The pre-parse stage is the only
point at which the malformed invocations are observable.

#### Enforcement

The validator is enforced by:

- **Unit tests** in `internal/cli/validate_test.go`.
- **Contract tests** in `internal/cli/root_test.go`.
- **Taskfile targets** `verify:help-contract` and
  `verify:version-contract`.
- **Code review.**

### 11.16 ForgeError in the CLI

The CLI produces `*forgeerr.Error` values through the constructors
in `internal/forgeerr/constructors.go`. The constructors validate
the category, the code, and the message; a violation is a
programming error and panics.

#### The error path

```go
func executeWithOptions(opts options) int {
    if err := validateArgs(opts.args); err != nil {
        fmt.Fprint(opts.stderr, forgeerr.Format(err, forgeerr.Options{
            Verbose: opts.verbose,
        }))
        return exitCodeOf(err)
    }
    deps := buildDependencies(opts)
    root := newRootCmd(deps)
    root.SetArgs(opts.args)
    // ...
    err := root.Execute()
    if err == nil {
        return ExitSuccess
    }

    // Convert Cobra's parser and dispatcher errors.
    err = wrapCobraError(err)

    // Wrap any remaining non-*forgeerr.Error.
    var ferr *forgeerr.Error
    if !errors.As(err, &ferr) {
        err = wrappedUnexpectedError(err)
    }

    verbose := verboseRequested(root)
    fmt.Fprint(opts.stderr, forgeerr.Format(err, forgeerr.Options{
        Verbose: verbose,
    }))
    return exitCodeOf(err)
}
```

The rendering is `forgeerr.Format`; the exit-code derivation is
`exitCodeOf`, which reads `Category.ExitCode()`. The
`formatError` and `exitCodeFromError` helpers that earlier versions
of this document described have been **deleted** (WBS 10.5.1).

#### The boundary's helpers

Three helpers encapsulate the boundary's specific behaviour:

| Helper | Purpose |
|---|---|
| `recoveredPanicError(any) *forgeerr.Error` | Converts a recovered panic value to a `FORGE_INTERNAL_PANIC` error with the value in the context. |
| `wrappedUnexpectedError(error) *forgeerr.Error` | Wraps a non-`*forgeerr.Error` in a `FORGE_INTERNAL_UNEXPECTED` error with the original message in the context. |
| `wrapCobraError(error) error` | Converts a Cobra parser or dispatcher error to a `CategoryCLI` error with the appropriate code. |

The helpers exist so that the boundary's behaviour can be tested
directly, without invoking `Execute` (which reads `os.Args` and
writes to `os.Stderr`).

#### The CLI's error contract

- Every error returned by a command is a `*forgeerr.Error` or
  wraps one. A command that returns a bare `errors.New` violates
  the contract; the boundary wraps it as
  `FORGE_INTERNAL_UNEXPECTED`.
- The pre-parse validator constructs `*forgeerr.Error` values.
- Cobra's parser and dispatcher errors are converted by
  `wrapCobraError`.
- The CLI never constructs a `*forgeerr.Error` literal. It uses
  `forgeerr.New` or `forgeerr.Wrap`.

#### Why no sentinels in the CLI

The CLI does not export sentinel values and does not match on
`*forgeerr.Error` pointer identity. Callers that want to match on
a specific code use `errors.As`:

```go
var target *forgeerr.Error
if errors.As(err, &target) && target.Code() == string(forgeerr.CodeConfigMalformed) {
    // handle FORGE_CONFIG_MALFORMED
}
```

This is the pattern documented in ADR-015 and enforced by the
tests in `internal/forgeerr/wrap_test.go`.

---

## 12. Cross-Cutting Concerns

The following concerns cut across modules and must be handled
consistently.

### 12.1 Error Handling

Every module:

- Returns errors using the `*forgeerr.Error` type.
- Never panics on user input. Panics are reserved for
  programming errors detected at construction time (see § 8.6).
- Wraps errors with `forgeerr.Wrap` and a cause when the
  original error is meaningful to a caller.
- Includes structured context where the context is useful to the
  user (a file path, a line number, a flag name).
- Includes a remediation where a specific action would resolve
  the error.

See § 8 for the full error architecture and § 11.16 for the
CLI's error contract.

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
│   ├── forgeerr/     # Canonical error type (WBS 10.1.1–10.5.2)
│   ├── renderer/     # Rendering engine
│   ├── validator/    # Validation engine
│   ├── update/       # Update engine
│   ├── state/        # State store
│   ├── version/      # Build metadata (WBS 4.3.1) — leaf package
│   ├── filesystem/   # Filesystem interface and OS implementation
│   ├── config/       # Configuration type
│   └── infra/        # Infrastructure (future)
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
| `internal/cli` | Command definitions, argument parsing, output wiring, the CLI's error boundary |
| `internal/app/<command>` | Use case orchestration for each command |
| `internal/domain/<concept>` | Pure domain logic and types |
| `internal/forgeerr` | Canonical error type, taxonomy, registry, wrapping contract, user format, log format, redaction, verbose modes |
| `internal/renderer` | Rendering of template content |
| `internal/validator` | Validation rules and engine |
| `internal/update` | Update planning, merging, application |
| `internal/state` | Reading and writing `.forge/state.yaml` |
| `internal/version` | Build metadata (version, commit, build date) — leaf |
| `internal/filesystem` | Filesystem interface and OS implementation |
| `internal/config` | Configuration type and its error taxonomy |
| `internal/infra/process` | Process execution (future) |
| `internal/infra/logging` | Logger interface and implementation |
| `internal/infra/output` | Human and JSON output formatters |
| `templates/` | Bundled templates |

### 13.2 Files in `internal/cli` and `internal/forgeerr`

The following tables list the files in the two packages that have
a fixed, enumerated file set.

#### `internal/cli`

| File | Purpose |
|------|---------|
| `doc.go` | Package documentation and public contract |
| `execute.go` | `Execute`, `executeWithOptions`, `options`, `defaultOptions`, `recoveredPanicError`, `wrappedUnexpectedError`, `wrapCobraError`, `parseUnknownCommandMessage`, `hasVerboseArg`, `exitCodeOf` |
| `deps.go` | `Dependencies`, `Logger`, `slogLogger`, `noopLogger`, `newLogger`, `buildDependencies` |
| `exitcodes.go` | Exit code constants |
| `root.go` | The root command constructor |
| `registry.go` | The central command registry |
| `flags.go` | The global flag registration and readers |
| `validate.go` | The pre-parse argument validator |
| `errors.go` | `unknownCommandError`, `suggestionsMinimumDistance` |
| `version.go` | The `forge version` handler |
| `config.go` | The hidden `forge config` placeholder |
| `*_test.go` | Tests colocated with their subjects. The complete list is in § 11.9. |

#### `internal/forgeerr`

| File | Purpose |
|------|---------|
| `forgeerr.go` | The `Error` type, its fields, its accessors, its `Error()` string, its chain methods (`Unwrap`, `Is`, `As`), and the pure invariant helpers (`validateMessage`, `validateRemediation`, `validateCategory`). |
| `constructors.go` | The frozen constructor API: `New`, `Wrap`, `WithCause`, `WithContext`, `WithRemediation`. |
| `category.go` | The closed category taxonomy: `Category`, seven constants, `AllCategories`, `Prefix`, `ExitCode`, `String`. |
| `codes.go` | The frozen code registry: `Code`, 19 constants, `AllCodes`, `Lookup`, `IsRegistered`, `Code.Category`. |
| `format.go` | The user-facing format: `Format`, `Options`, and the byte-level rules F1–F12. |
| `log.go` | The programmer-facing log format: `LogValue` (implements `slog.LogValuer`). |
| `redact.go` | The redaction policy: `redact`, `redactPath`, `redactPathFor`, `AllSecretKeys`. |
| `forgeerr_test.go` | Tests for the type and its invariant helpers. |
| `constructors_test.go` | Tests for the constructor API (AC1–AC9, WBS 10.1.2). |
| `category_test.go` | Tests for the category taxonomy (AC1–AC6, WBS 10.2.1). |
| `codes_test.go` | Tests for the code registry (AC1–AC8, WBS 10.3.1). |
| `codes_flags_test.go` | Registration of the `-update` flag used by the golden-file tests. |
| `wrap_test.go` | Tests for the wrapping contract (W1–W8, AC1–AC7, WBS 10.3.2). |
| `format_test.go` | Tests for the user-facing format (AC1–AC10, WBS 10.4.1). |
| `log_test.go` | Tests for the log format (AC1–AC8, WBS 10.4.2). |
| `redact_test.go` | Tests for the redaction policy (AC1–AC8, WBS 10.4.3). |
| `testdata/codes/codes.golden.txt` | The canonical rendering of the registry. |
| `testdata/format/` | The golden files for the user-facing format. |
| `testdata/log/` | The golden files for the log format. |

The `internal/forgeerr` package does not import any Forge package.
It depends only on the Go standard library. See § 4.2.18.

### 13.3 Import Rules

- `internal/domain/*` may not import any other `internal/*` package
  (except other `internal/domain/*`).
- `internal/app/*` may import `internal/domain/*` and
  `internal/infra/*`.
- `internal/cli` may import `internal/app/*`, `internal/infra/*`,
  and `internal/forgeerr`.
- `internal/infra/*` may import `internal/domain/*` (for shared
  types) but not `internal/app/*` or `internal/cli`.
- `internal/version` may not import any other `internal/*` package.
  It is a leaf.
- `internal/forgeerr` may not import any other `internal/*`
  package. It is a leaf.
- `internal/filesystem` may import `internal/domain/*` (for shared
  types) but not `internal/app/*` or `internal/cli`.
- `internal/config` may import `internal/forgeerr` (for its error
  taxonomy).
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
- The error code registry (`internal/forgeerr/testdata/codes/codes.golden.txt`, WBS 10.3.1)
- The user-facing error format (`internal/forgeerr/testdata/format/`, WBS 10.4.1)
- The log format (`internal/forgeerr/testdata/log/`, WBS 10.4.2)

Golden files are committed and version-controlled.

### 14.5 Fuzz Tests

Fuzz tests are used for:

- Path parsing
- YAML parsing
- Template rendering
- Blueprint validation
- Error invariant helpers (`FuzzValidateMessage`,
  `FuzzValidateRemediation` in `forgeerr_test.go`)
- Constructor inputs (`FuzzCodeFormat`, `FuzzMessage`,
  `FuzzContextKey` in `constructors_test.go`)
- Taxonomy membership (`FuzzCategoryValid` in `category_test.go`)
- Registry membership (`FuzzRegistryMembership` in `codes_test.go`)
- User-facing format (`FuzzFormat` in `format_test.go`)
- Log format (`FuzzLogValue` in `log_test.go`)
- Redaction (`FuzzRedact`, `FuzzRedactPath` in `redact_test.go`)

---

## 15. Extensibility

The architecture supports future extension without breaking existing
behaviour.

### 15.1 Adding a New Command

See § 11.13 for the complete procedure.

### 15.2 Adding a New Domain Concept

1. Define the concept in `internal/domain/<concept>`
2. Define its interfaces
3. Wire it into the application layer
4. Implement infrastructure as needed

### 15.3 Adding a New Infrastructure Provider

1. Define the interface (in the package that owns the concept; see
   § 3.3).
2. Implement the provider in `internal/<provider>` or
   `internal/infra/<provider>`.
3. Wire it into the CLI.

### 15.4 Adding a Global Flag

1. Write an ADR that names the consumer WBS item and the flag's
   semantics.
2. Add the flag-name constant to `internal/cli/flags.go`.
3. Register the flag in `registerGlobalFlags`.
4. Add the reader and (if needed) the resolver.
5. Update § 4.11 of `docs/cli-ux-spec.md`.
6. Add tests.
7. Add the flag to the `verify:global-flags` Taskfile target's
   expected count.

### 15.5 Adding a Pre-Parse Rejection

1. Write the rejection's contract in `docs/cli-ux-spec.md`.
2. Add a sub-validator to `internal/cli/validate.go`.
3. Add the sub-validator to the `validateArgs` dispatcher.
4. Add unit tests in `internal/cli/validate_test.go`.
5. Add a contract test in `internal/cli/root_test.go`.
6. If the rejection involves an interception flag, extend
   `TestValidateHelpTopic_CobraGeneratedNames` or write an
   equivalent test to pin the coupling.

### 15.6 Adding a Field to `Dependencies`

1. Add the field to the `Dependencies` struct in
   `internal/cli/deps.go`.
2. Construct the field in `buildDependencies`.
3. Add a test to `internal/cli/deps_test.go`.
4. If the field is a new collaborator (not a primitive), add its
   interface to the appropriate package and its implementation to
   the appropriate infrastructure package.
5. Update the § 11.2.2 table.

No command constructor signature changes. This is the property that
makes `Dependencies` a stable boundary across phases.

### 15.7 Adding a New Error Code

1. Write an ADR (per ADR-014) that names the new code, its
   category, its scope, and the phase in which it is introduced.
2. Add the constant to `internal/forgeerr/codes.go`.
3. Add the row to `codeSpecs` in the same file.
4. Regenerate the golden file:
   `go test -run TestAC4 -update ./internal/forgeerr/`.
5. Update `docs/reference/error-codes.md`.
6. If the code belongs to a category currently listed in
   `pendingCategories`, remove the category from that list.
7. Run the full test suite.

### 15.8 Adding a New Error Category

1. Write an ADR (per ADR-013) that names the new category, its
   code prefix, its exit code, and its scope.
2. Add the constant to `internal/forgeerr/category.go`.
3. Add the category to `allCategories` in the same file.
4. Add the prefix to `Category.Prefix` and the exit code to
   `Category.ExitCode`.
5. Add the exit-code constant to `internal/cli/exitcodes.go`.
6. Add the category to `pendingCategories` in
   `internal/forgeerr/codes_test.go` unless the same commit also
   adds at least one code for it.
7. Update `docs/reference/error-categories.md`.
8. Run the full test suite.

### 15.9 Adding a New Verbose-Mode Element

1. Write an ADR (per ADR-019) that names the new element, its
   non-verbose treatment, and its verbose treatment.
2. Add the element to `Format`'s verbose path in
   `internal/forgeerr/format.go`.
3. Add the element to the mode matrix in § 8.13.
4. Regenerate the verbose golden file:
   `go test -run TestGolden_VerboseFormat -update ./internal/forgeerr/`.
5. Add tests for the element in `format_test.go`.
6. Run the full test suite.

### 15.10 Adding a New Secret-Key Pattern

1. Write an ADR that names the new key and the reason it is a
   secret.
2. Add the key to `secretKeys` in
   `internal/forgeerr/redact.go`.
3. Update `docs/reference/error-codes.md` if the pattern is
   reflected there.
4. Add a test row in `redact_test.go`.
5. Run the full test suite.

### 15.11 Plugin System

A plugin system is not supported in Phase 1. The architecture
reserves space for future plugins. Plugins will be introduced in a
future phase.

---

## 16. Anti-Patterns to Avoid

The following are explicitly discouraged.

### 16.1 Cardinal Sins

#### 16.1.1 Business Logic in the CLI

The CLI layer is for command parsing and output formatting only.
Any business logic belongs in the Application or Domain layer. See
§ 11.12.

#### 16.1.2 Direct Filesystem Access from Domain

Domain code never calls `os.ReadFile`, `os.WriteFile`, or similar.
All filesystem access goes through the `Filesystem` interface.

#### 16.1.3 Global State

No global mutable variables. Dependencies are injected via
constructors.

#### 16.1.4 Circular Dependencies

Packages do not import each other cyclically.

#### 16.1.5 Cobra Outside the CLI Layer

No package outside `internal/cli` may import
`github.com/spf13/cobra`.

### 16.2 Additional Rules

#### 16.2.1 God Objects

No single object orchestrates everything.

#### 16.2.2 Implicit Dependencies

Every dependency is explicit in a constructor.

#### 16.2.3 Silent Failures

Every error is reported.

#### 16.2.4 Unstructured Errors

Every error uses the `*forgeerr.Error` type, constructed through
`forgeerr.New` or `forgeerr.Wrap`. No
`errors.New("something failed")`, and no `fmt.Errorf` without a
`%w` wrapping a `*forgeerr.Error`. The registry in
`internal/forgeerr/codes.go` is the single source of truth for
the set of codes an error may carry. See § 8.

#### 16.2.5 Unstructured Logs

Every log message has a level and structured fields.

#### 16.2.6 Environment-Dependent Behaviour

Forge behaves the same regardless of environment, except through
explicit, documented configurations.

#### 16.2.7 Build Metadata Outside the Leaf Package

The build metadata variables live in `internal/version` and only
there.

#### 16.2.8 `init()`-Based Command Registration

Commands are registered in the `registry` slice in `registry.go`.

#### 16.2.9 Speculative Global Flags

A global flag must have a documented consumer in a later WBS item.

#### 16.2.10 Pre-Parse Rejection in a Cobra Hook

Malformed invocations that involve interception flags cannot be
rejected in a Cobra hook. The rejection belongs at the pre-parse
stage.

#### 16.2.11 Error Literals Outside `forgeerr`

A `*forgeerr.Error` is constructed only through `forgeerr.New`,
`forgeerr.Wrap`, or the `WithX` methods. The struct literal
`&forgeerr.Error{...}` must not appear in any package.

#### 16.2.12 Sentinels for Error Codes

Forge does not export sentinel values for error codes. Callers
match on the code through `errors.As` and `Code()`. Exporting a
sentinel would create a second way to identify the same error,
and the two would drift.

#### 16.2.13 Classifying a Parse Error as a Value Error

A YAML syntax error, a duplicate key, an unknown field, and a
type mismatch are distinct classifications. The loader must
classify each by the parser's message shape (see
`classifyYAMLError` in `internal/config/loader.go`) and produce
the corresponding code: `CodeConfigMalformed` for syntax errors
and duplicate keys, `CodeConfigUnknownField` for unknown fields,
`CodeConfigInvalidType` for type mismatches, and
`CodeConfigInvalidValue` for value errors. Collapsing the four
into a single "invalid value" code hides the user's remediation
and mislabels the failure. A future change to the loader must
preserve the distinction or amend the taxonomy via an ADR.

#### 16.2.14 Direct Writes to Process-Global Streams

No function in the CLI or the application layer may write to
`os.Stdout`, `os.Stderr`, or `os.Stdin` directly, except for the
panic-recovery path in `Execute` (which is a documented
exception, § 11.7). All other output flows through the injected
`options` or `Dependencies` writers.

#### 16.2.15 Legacy Renderers in the Boundary

The boundary's error path uses `forgeerr.Format` and
`Category.ExitCode()`. The legacy helpers `formatError` and
`exitCodeFromError` are deleted. A future contributor must not
reintroduce a local renderer or a parallel exit-code mapping;
the format and the mapping each have exactly one home.

---

## 17. Keeping This Document Accurate

This document describes the code. When the code changes, the
document changes in the same commit.

### 17.1 The Rule

> A pull request that changes a structural property described by
> this document must update the corresponding section of this
> document in the same commit.

A PR that renames `validateArgs`, or moves a file between packages,
or adds a `Dependencies` field, or changes the exit-code mapping, or
adds a code to the registry, is incomplete without a corresponding
document update.

### 17.2 How to Reference Sections

Use section headings, not section numbers, in commit messages, PR
descriptions, and code comments.

### 17.3 How the Document Is Verified

This document is reviewed by human reviewers, not by a linter. The
three mechanical claims it makes — that certain tests exist, that
certain Taskfile targets exist, and that certain structural rules
are enforced — are verified by running the named tests and targets.

If you find a claim in this document that does not match the code,
fix the code or fix the document, in the same commit, and note which
you did.

### 17.4 What Is Not Verified

The document does not claim to be complete. It describes the
structure that Phase 2 has established.

### 17.5 Document History

Every substantive change to this document is recorded in § 20.

---

## 18. Relationship to Other Specifications

| Specification | Relationship |
|---------------|--------------|
| [Blueprint](./blueprint-spec.md) | Domain model consumed by the Blueprint package |
| [forge.yaml](./forge-yaml-spec.md) | Configuration consumed by the Configuration package |
| [Template](./template-spec.md) | Domain model consumed by the Template package |
| [Component](./component-spec.md) | Domain model consumed by the Component package |
| [Validation](./validation-spec.md) | Implemented by the Validator package |
| [Security Model](./security-model.md) | Enforced by the Filesystem package; defines the secret pattern list |
| [Update Model](./update-model.md) | Implemented by the Update Engine package |
| [CLI UX Spec](./cli-ux-spec.md) | Implemented by the CLI layer; pins the flag inventory, the error format, the verbose mode, and the pre-parse rejection contract |
| [Error Categories](./reference/error-categories.md) | Implemented by `internal/forgeerr/category.go` |
| [Error Codes](./reference/error-codes.md) | Implemented by `internal/forgeerr/codes.go` |
| [Development Guide](./development.md) | Contributor-facing companion |
| [Dependency Policy](./dependency-policy.md) | Governs what this architecture may depend on |
| [Product Discovery](./product-discovery.md) | Defines the product this architecture implements |

---

## 19. Open Questions

The following questions remain open:

- Should the Application layer use a use-case pattern or a
  service pattern?
- Should the Domain layer use value objects or plain structs?
- Should the Filesystem interface be narrow or wide?
- Should the Update Engine use a library for three-way merge or
  implement one?
- Should logging use a library or a custom implementation?
- Should the CLI support shell completion out of the box?
- Should the architecture support parallelism for large
  repositories?
- Should the State store use YAML, JSON, or a binary format?
- Should the architecture include a cache layer for template
  resolution?

These questions will be addressed in Phase 2 as implementation
begins.

---

## 20. Document History

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 0.1.0 | 2026-10-09 | @thapelomagqazana | Initial Phase 1 draft. |
| 0.2.0 | 2026-10-09 | @thapelomagqazana | Added module list, dependency direction, interface contracts, and data flow. |
| 0.3.0 | 2026-10-09 | @thapelomagqazana | Refined § 11 to describe the two-boundary execution model. |
| 0.4.0 | 2026-10-10 | @thapelomagqazana | Added § 11.12 (Handler / Service Boundary). |
| 0.5.0 | 2026-10-10 | @thapelomagqazana | Added § 11.13, § 11.14, § 11.15. |
| 0.6.0 | 2026-10-10 | @thapelomagqazana | Structural-review response. |
| 0.7.0 | 2026-10-12 | @thapelomagqazana | Error-architecture update for WBS 10.1.1, 10.1.2, 10.2.1, 10.3.1, and 10.3.2. |
| 0.8.0 | 2026-10-12 | @thapelomagqazana | Error-architecture update for WBS 10.4.1, 10.4.2, 10.4.3, 10.4.4, 10.5.1, and 10.5.2. Added § 8.7 (user-facing format), § 8.11 (log format), § 8.12 (redaction), § 8.13 (verbose modes), § 8.14 (boundary integration). Rewrote § 8.7, § 8.9, § 11.7, § 11.16 to reflect `forgeerr.Format`, `wrapCobraError`, `wrappedUnexpectedError`, `recoveredPanicError`, and `exitCodeOf`; the legacy `formatError` and `exitCodeFromError` helpers are deleted. Updated § 6.13 with the formatter, log, redaction, and verbose surfaces. Updated § 13.2 with the new `internal/forgeerr` files and the updated `internal/cli` file list. Added § 16.2.13 (parse-error classification), § 16.2.14 (direct writes to process streams), and § 16.2.15 (legacy renderers in the boundary). Updated § 11.9, § 11.10, § 11.14, § 14.4, § 14.5, and § 15 (added § 15.9 and § 15.10). |
