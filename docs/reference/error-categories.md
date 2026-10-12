# Error Categories

Forge classifies every user-facing error into one of seven
categories. A category determines two things:

- The **Code prefix** that the error's Code must carry. The
  prefix is verified by the constructors.
- The **process exit code** returned when the error reaches the
  CLI boundary.

The taxonomy is **closed**. Adding a category requires an ADR.

## The categories

| # | Category | Prefix | Exit code | Scope |
|---|---|---|---|---|
| 1 | `CLI` | `FORGE_CLI_` | 2 (`ExitUsage`) | CLI usage, flags, commands |
| 2 | `CONFIG` | `FORGE_CONFIG_` | 3 (`ExitConfig`) | Configuration loading and validation |
| 3 | `FILESYSTEM` | `FORGE_FILESYSTEM_` | 4 (`ExitFilesystem`) | File I/O, paths, permissions |
| 4 | `VALIDATION` | `FORGE_VALIDATION_` | 5 (`ExitValidation`) | Schema, blueprint, check validation |
| 5 | `TEMPLATE` | `FORGE_TEMPLATE_` | 1 (`ExitFailure`) | Template resolution and rendering |
| 6 | `SECURITY` | `FORGE_SECURITY_` | 6 (`ExitSecurity`) | Path traversal, secrets, boundaries |
| 7 | `INTERNAL` | `FORGE_INTERNAL_` | 1 (`ExitFailure`) | Bugs, unexpected panics |

## Rules

- The taxonomy is closed. Adding a category requires an ADR.
- Every category has a fixed exit code (frozen in WBS 4.1.2).
- Every category has a fixed Code prefix. The category is
  derivable from the prefix and vice versa.
- Codes are stable across versions. A Code, once shipped, is
  never renamed (see WBS 10.3.1).
- There is no "OTHER" category. If an error does not fit
  another category, it is `INTERNAL`.
- `TEMPLATE` uses `ExitFailure` (1), not a dedicated code.
  Templates are a Phase 4 concern and may receive their own exit
  code later via ADR.

## Choosing a category

When constructing an error, pick the category that describes the
**scope of the failure**, not the mechanism by which it was
detected:

- A file that fails to open because a parent directory has
  restrictive permissions is `FILESYSTEM`, not `SECURITY`. The
  permission check is a filesystem concern; `SECURITY` is reserved
  for deliberate boundary violations (e.g. a path that escapes
  the project root).
- A malformed YAML file is `CONFIG`, not `FILESYSTEM`, even
  though it was read from disk. The failure is about the file's
  contents, not the I/O.
- A blueprint that fails its schema check is `VALIDATION`. A
  blueprint file that cannot be read is `FILESYSTEM`.

If you cannot decide between two categories, choose the one
whose exit code is more specific. `ExitFailure` (1) is the
fallback for `TEMPLATE` and `INTERNAL`; a more specific code is
almost always more useful to a script.

## Adding a category

Adding a category requires an ADR. The ADR must specify:

- The new category's name.
- Its Code prefix. The prefix must satisfy the Code regex
  (`^FORGE_[A-Z][A-Z0-9_]*$`) when combined with a valid tail.
- Its exit code. If the exit code is new, WBS 4.1.2 must be
  amended in the same ADR.
- The scope it covers and, where the scope overlaps an existing
  category, the boundary rule.

Once the ADR is accepted, the change is a single edit in
`internal/forgeerr/category.go` plus updates to the test suite.
