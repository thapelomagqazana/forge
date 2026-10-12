# Error Codes

Forge emits errors identified by a stable, machine-readable code.
A code has the form `FORGE_<CATEGORY>_<SPECIFIC>`, where
`<CATEGORY>` matches one of the seven members of the
[error category taxonomy](error-categories.md) and `<SPECIFIC>`
disambiguates the code within its category.

## Stability policy

A code, once shipped in a release, is **frozen forever**. The
following rules are enforced by the test suite and by code review:

- **Codes are never renamed.** If the meaning of a code must
  change, a new code is added and the old code is marked
  deprecated.
- **Codes are never reused for different meanings.** A code that
  is deprecated is never repurposed for a new error.
- **Deprecated codes remain in the registry.** They are marked
  with `Deprecated: yes` in the table below and remain valid for
  construction so that historical logs and scripts continue to
  resolve.
- **Adding a code requires a PR that touches
  `internal/forgeerr/codes.go`.** The registry is a closed set;
  the constructors reject any code that is not listed there.
- **Every code has a documentation entry.** This file is kept in
  sync with the registry by a test
  (`TestAC5_DocsReferenceSyncWithRegistry`). A registry change
  without a corresponding documentation change fails CI.

## The registry

<!-- The table below is generated from codes.go. Do not edit it by
     hand. To regenerate, run:
       go test -run TestAC5_DocsReferenceSyncWithRegistry -update ./internal/forgeerr/
     and copy the emitted table into this file. -->

| Code | Category | Exit | Introduced | Deprecated |
|---|---|---|---|---|
| `FORGE_CLI_INVALID_ARGUMENT` | CLI | 2 | Phase 2 | no |
| `FORGE_CLI_UNKNOWN_COMMAND` | CLI | 2 | Phase 2 | no |
| `FORGE_CLI_UNKNOWN_FLAG` | CLI | 2 | Phase 2 | no |
| `FORGE_CLI_UNKNOWN_FORMAT` | CLI | 2 | Phase 2 | no |
| `FORGE_CONFIG_INVALID_ENCODING` | CONFIG | 3 | Phase 2 | no |
| `FORGE_CONFIG_INVALID_TYPE` | CONFIG | 3 | Phase 2 | no |
| `FORGE_CONFIG_INVALID_VALUE` | CONFIG | 3 | Phase 2 | no |
| `FORGE_CONFIG_MALFORMED` | CONFIG | 3 | Phase 2 | no |
| `FORGE_CONFIG_NOT_FOUND` | CONFIG | 3 | Phase 2 | no |
| `FORGE_CONFIG_PERMISSION` | CONFIG | 3 | Phase 2 | no |
| `FORGE_CONFIG_TOO_LARGE` | CONFIG | 3 | Phase 2 | no |
| `FORGE_CONFIG_UNKNOWN_FIELD` | CONFIG | 3 | Phase 2 | no |
| `FORGE_FILESYSTEM_ESCAPE` | FILESYSTEM | 4 | Phase 2 | no |
| `FORGE_FILESYSTEM_NOT_FOUND` | FILESYSTEM | 4 | Phase 2 | no |
| `FORGE_FILESYSTEM_PERMISSION` | FILESYSTEM | 4 | Phase 2 | no |
| `FORGE_INTERNAL_PANIC` | INTERNAL | 1 | Phase 2 | no |
| `FORGE_INTERNAL_UNEXPECTED` | INTERNAL | 1 | Phase 2 | no |
| `FORGE_SECURITY_PATH_TRAVERSAL` | SECURITY | 6 | Phase 2 | no |
| `FORGE_SECURITY_SYMLINK_ESCAPE` | SECURITY | 6 | Phase 2 | no |

## Choosing a code

When you are adding a new error path to Forge, the checklist is:

1. Identify the category. Use the rule in
   [error-categories.md](error-categories.md#choosing-a-category).
2. If a code in the registry already describes the failure,
   reuse it. Do not invent a new code for an existing meaning.
3. If no code fits, add a new one. The `<SPECIFIC>` portion
   should be a noun phrase in `UPPER_SNAKE_CASE` that names the
   failure, not the function that detected it.
4. Update `codes.go`, regenerate the golden file, and update the
   table above. The sync test will fail otherwise.

## Deprecation

A code is deprecated when its meaning has drifted and a new code
now covers the case it used to cover. Deprecation is additive:

- The deprecated code remains in the registry with
  `Deprecated: yes`.
- The deprecated code remains valid for construction; the
  constructors do not reject it.
- A future release may emit a warning when a deprecated code is
  constructed (tracked in WBS 10.5.x).
- A deprecated code is never removed and never reused.

As of Phase 2, no code is deprecated.
