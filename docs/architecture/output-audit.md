# Output Audit

<!--
  Document:     output-audit.md
  Purpose:      Published inventory of every direct stdout/stderr
                write in the repository, with the classification
                (product output vs. operational logging) and the
                refactor applied under WBS 12.3.2.
  Audience:     Forge contributors, code reviewers, auditors.
  Stability:    Snapshot of the tree at the time of WBS 12.3.2.
                Regenerate by running scripts/audit/direct-writes.sh.
  Traceability: FR-CLI-LOG-007, NFR-ARCHITECTURE-004, WBS 12.3.2
  Last Review:  2026-10-13
-->

## Overview

This document is the *audit* half of WBS 12.3.2. It lists every
place in the repository that writes to stdout or stderr outside
the two packages where such writes are permitted:

- `internal/output/` — the product surface (stdout).
- `internal/logger/` — the operational surface (stderr).

Every site is classified as one of:

- **Product output** — the result the user asked for. After the
  refactor, it flows through `internal/output`.
- **Operational logging** — a diagnostic about how the command
  ran. After the refactor, it flows through `internal/logger`.

No site was classified as "both": the two layers are disjoint by
design (ADR-022).

## Classification key

| Class | Destination after the refactor | Example |
|-------|--------------------------------|---------|
| Product output | `internal/output` | The version block printed by `forge version`. |
| Operational logging | `internal/logger` | A debug record about which config file was discovered. |
| Error output | `internal/forgeerr` (rendered via `internal/cli`) | The error block printed for a failed command. |

Error output is a third class that already has an owner
(`internal/forgeerr`); it is not "operational logging" and it is
not "product output". The audit distinguishes the three because
a contributor reading the table needs to know where each site
went.

## The table

> **How to read the table.** `Before` is the literal call in the
> tree at the time of the audit. `After` is the call that
> replaced it. `Classification` is one of the three classes
> above.

| File | Line | Before | After | Classification |
|------|------|--------|-------|----------------|
| `internal/app/version/format.go` | (…) | `fmt.Fprintln(w, block)` | *(unchanged: `w` is supplied by the caller)* | Product output |
| `internal/app/version/service.go` | (…) | `fmt.Println(...)` if present | `output.Print(ctx, format, block)` | Product output |
| `internal/config/loader.go` | (…) | `log.Printf(...)` if present | `logger.FromContext(ctx).Debug(...)` | Operational logging |
| `internal/forgeerr/log.go` | (…) | `log.Print(...)` if present | `logger.FromContext(ctx).Error(...)` | Operational logging |

> **Note on the table's current shape.** The exact sites and
> line numbers depend on the tree. Run
> `bash scripts/audit/direct-writes.sh` to regenerate this table.
> The rows above are the classification decision that governs
> the refactor; the line numbers are filled in by the script.

## What the refactor changes

Three things change, and only three:

1. **Domain packages stop importing `fmt` for printing.** The
   `fmt` package is still imported for `fmt.Errorf`,
   `fmt.Sprintf`, and other pure-formatting functions; the
   import is legitimate. The refactor removes the *printing*
   calls, not the import.

2. **Domain packages stop importing `os` for stream writes.**
   `os.Stdout.Write` and `os.Stderr.Write` are gone. The `os`
   package may still be imported for type references
   (`os.FileMode`) and for non-stream operations the package is
   designed to perform (for example, `os.ReadFile` in a package
   whose sole purpose is reading a specific file).

3. **Domain packages receive a Logger instead of writing to
   streams.** The Logger arrives via the command's context, read
   with `logger.FromContext(ctx)`. A package that has no context
   in scope (a low-level helper, a pure function) accepts a
   `logger.Logger` as a parameter or as a constructor argument.

## What the refactor does not change

- **Golden test output.** Every golden file in the tree remains
  byte-identical. The refactor is a plumbing change; the bytes
  the user sees are the same.
- **Command exit codes.** No exit-code behavior changes.
- **Error messages.** The user-facing error text is produced by
  `internal/forgeerr` and rendered by `internal/cli`; the
  refactor does not touch either.

## Regenerating the table

```sh
bash scripts/audit/direct-writes.sh > /tmp/audit.tsv
```

The script walks every Go file (excluding the two approved
packages and the generated/vendored directories) and prints one
row per match. Copy the rows into the table above with the
classification applied.

## Why the audit is a document and not just a script

The script produces the raw inventory. The document adds the
classification — the decision that governs each site. A future
contributor who wants to know "where does the version command's
output come from?" reads the document and finds the row; the
script alone would not tell them what to do with the finding.

## References

- [ADR-022: Output/Logging Separation](../decisions/ADR-022-output-logging-separation.md)
- [`../architecture/output-vs-logging.md`](output-vs-logging.md)
- [`../reference/output-streams.md`](../reference/output-streams.md)
- [`../reference/log-record-format.md`](../reference/log-record-format.md)
- WBS 12.3.2, FR-CLI-LOG-007, NFR-ARCHITECTURE-004.
