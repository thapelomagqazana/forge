<!--
  Forge version output
  ====================
  Public contract. Versioned with the Forge release. Additive changes
  only.

  Normative references:
    - Versioning scheme:    docs/reference/versioning.md
    - Release channels:     docs/reference/release-channels.md
    - Reproducible builds:  docs/guides/reproducible-builds.md
    - Building:             docs/guides/building.md
    - Schemas:              schemas/version.schema.json
                            schemas/version-full.schema.json
                            schemas/version-metadata.schema.json
    - ADR:                  docs/decisions/ADR-046-version-metadata-and-provenance.md

  Editing rules:
    1. Every field name and JSON type is frozen. A change is a MAJOR
       version bump.
    2. Every example MUST be a value the binary can actually produce.
       An example that the code cannot emit is a defect in this file.
    3. If this document and schemas/version.schema.json disagree, the
       schema wins and this document is corrected in the same commit.
    4. The default human format and the JSON format are contracts.
       The verbose format is not a contract; it may grow new lines,
       but existing lines are not reformatted.
-->

# Forge version output

`forge version` reports the version of the running binary. It has
four output modes:

- the **default** human format, five lines
- **`--format json`**, a single-line JSON object
- **`--verbose`**, an extended human format with provenance
- **`--format json --verbose`**, a single-line JSON object with
  provenance

The five linker-injected fields are the same in every mode. The
provenance fields are present only in the two verbose modes.

## Fields

The five linker-injected fields:

| Field | Type | Source | Example |
|---|---|---|---|
| `version` | string | `git describe --tags --always --dirty` or `"dev"` | `v1.4.2` |
| `commit` | string | `git rev-parse --short HEAD` or `"none"` | `abc1234` |
| `buildDate` | string | `SOURCE_DATE_EPOCH`, RFC 3339 UTC | `2026-06-01T12:00:00Z` |
| `dirty` | boolean | `git status --porcelain` at build time | `false` |
| `buildType` | string | `release`, `ci`, or `local` | `release` |

The field names, their JSON types, and their order are frozen. A
change is a MAJOR version bump and requires a new ADR.

The provenance fields, present only in the verbose modes:

| Field | Type | Source | Example |
|---|---|---|---|
| `goVersion` | string | `runtime/debug.BuildInfo.GoVersion` | `go1.23.4` |
| `module` | string | `runtime/debug.BuildInfo.Main.Path` | `github.com/thapelomagqazana/forge` |
| `moduleVersion` | string (optional) | `runtime/debug.BuildInfo.Main.Version` | `v1.4.2` |
| `vcsRevision` | string (optional) | `vcs.revision` build setting | `0123456789abcdef0123456789abcdef01234567` |
| `vcsTime` | string (optional) | `vcs.time` build setting | `2026-05-15T09:30:00Z` |
| `vcsModified` | boolean | `vcs.modified` build setting | `false` |
| `dependencies` | array (optional) | `runtime/debug.BuildInfo.Deps` | see below |
| `settings` | array (optional) | `runtime/debug.BuildInfo.Settings` | see below |

The optional provenance fields are absent when they carry no
information. A consumer that expects `vcsRevision` in a binary built
without `-buildvcs=true` will not find it; the absence is the signal.

## Default format

The default format is five lines:

```text
forge version v1.4.2 (commit abc1234, built 2026-06-01T12:00:00Z)
  go version: go1.23.4
  platform:   linux/amd64
  build type: release
```

The header line carries the version, commit, and build date. The
three lines below carry the Go toolchain version, the
`GOOS/GOARCH` pair the binary was built for, and the build type.

A local build shows `dev`:

```text
forge version dev (commit none, built 2026-06-01T12:00:00Z)
  go version: go1.23.4
  platform:   linux/amd64
  build type: local
```

The format is stable enough to grep. It is not a machine contract; a
user who needs to parse the output uses `--format json`.

## JSON format

`forge version --format json` emits a single-line JSON object
followed by a newline:

```json
{"version":"v1.4.2","commit":"abc1234","buildDate":"2026-06-01T12:00:00Z","dirty":false,"buildType":"release"}
```

Field order matches the table above. The output is validated against
[`schemas/version.schema.json`](../../schemas/version.schema.json) by
`task version:verify`.

HTML escaping is disabled: a version string containing `<`, `>`, or
`&` (unusual but permitted) survives verbatim.

## Verbose format

`forge version --verbose` prints up to four sections separated by
blank lines. The format is for humans; a user who needs to parse it
uses `--format json --verbose`.

```text
version:    v1.4.2
commit:     abc1234
buildDate:  2026-06-01T12:00:00Z
dirty:      false
buildType:  release

go:         go1.23.4
module:     github.com/thapelomagqazana/forge
vcs:        0123456789abcdef0123456789abcdef01234567
vcsTime:    2026-05-15T09:30:00Z
vcsDirty:   false

dependencies:
  github.com/spf13/cobra v1.8.1
  github.com/spf13/pflag v1.0.5
  gopkg.in/yaml.v3 v3.0.1

settings:
  -buildmode=exe
  -compiler=gc
  -trimpath=true
  CGO_ENABLED=0
  GOARCH=amd64
  GOOS=linux
  vcs.modified=false
  vcs.revision=0123456789abcdef0123456789abcdef01234567
  vcs.time=2026-05-15T09:30:00Z
```

Every top-level line is `<label>:<spaces> <value>`. The label column
is padded to a fixed width so the values align; the padding is
spaces, not tabs, so the alignment survives a captured log.

The dependencies and settings sections are omitted when their lists
are empty. A binary built from a single-module repository with no
external dependencies produces a two-section output.

Empty top-level values render as `-`. The dash is the conventional
Unix marker for "not present"; it is unambiguous and does not collide
with any value a user would supply.

The `moduleVersion` line is suppressed when its value is `(devel)`,
which is the sentinel the Go toolchain uses for a binary built from a
working tree. The line is present only when the value is meaningful.

The `vcsTime` line is suppressed when the time is zero. A binary
built without `-buildvcs=true` has no time; printing `vcsTime: -`
would suggest the setting was present but empty.

## JSON verbose format

`forge version --format json --verbose` emits a single-line JSON
object followed by a newline:

```json
{"version":"v1.4.2","commit":"abc1234","buildDate":"2026-06-01T12:00:00Z","dirty":false,"buildType":"release","provenance":{"goVersion":"go1.23.4","module":"github.com/thapelomagqazana/forge","vcsRevision":"0123456789abcdef0123456789abcdef01234567","vcsTime":"2026-05-15T09:30:00Z","vcsModified":false,"dependencies":[{"path":"github.com/spf13/cobra","version":"v1.8.1","sum":"h1:...","indirect":false}],"settings":[{"key":"-trimpath","value":"true"}]}}
```

The output is a superset of the simple JSON shape: the five linker
fields appear at the top level, and a `provenance` object carries the
rest. A consumer that reads only the five fields reads the same
values from either shape.

The output is validated against
[`schemas/version-full.schema.json`](../../schemas/version-full.schema.json).

The `provenance` object omits its optional fields when they carry no
information: a binary built without `-buildvcs=true` has no
`vcsRevision` and no `vcsTime`, and a binary with no dependencies has
no `dependencies` array.

## Value derivation

### version

The value is the linker-injected `Version` string. It is whatever
`git describe --tags --always --dirty` produced, or `dev` if the
build type is `local` and there is no tag.

| Build | Value |
|---|---|
| Tagged, clean commit | `v1.4.2` |
| Three commits after a tag | `v1.4.2-3-gabc1234` |
| Tagged, dirty tree (local) | `v1.4.2-dirty` |
| Untagged, dirty tree (local) | `v1.4.2-3-gabc1234-dirty` |
| Local with no git | `dev` |
| Uninstrumented | empty string |

The value is preserved verbatim. It is not normalized, not parsed,
and not validated. A consumer that needs the SemVer components parses
the string itself.

### commit

The short SHA of the commit the binary was built from. `none` outside
a git working tree. Empty for an uninstrumented build.

### buildDate

An RFC 3339 UTC timestamp. Derived from `SOURCE_DATE_EPOCH` if set;
otherwise from `git log -1 --format=%ct`. The value ends in `Z`. A
numeric offset (`+02:00`) is a defect; the release pipeline rejects
it. Empty for an uninstrumented build.

### dirty

`true` if the working tree had uncommitted changes when the binary
was built. The release pipeline refuses to run on a dirty tree, so a
released binary always reports `false`.

### buildType

`release`, `ci`, or `local`:

| Value | Meaning |
|---|---|
| `release` | Set by the release pipeline. The tree was clean and the commit was tagged. |
| `ci` | Set by a non-release CI job. The tree may be untagged; the artifact is not a release. |
| `local` | Set by any build that carries no explicit value. This includes a contributor's machine and an uninstrumented build. |

An unrecognized value is normalized to `local` rather than causing an
error. A binary that carries no build-type information reports
`local`.

### provenance

The provenance is read from
`runtime/debug.ReadBuildInfo`, which the Go runtime populates at
startup from the module and VCS information embedded in the binary.
The same information is what `go version -m ./forge` prints.

The reader is tolerant: a binary built with `GO111MODULE=off` has no
build info, and the provenance fields are all empty or zero.

## Compatibility

The five linker field names, their JSON types, and their order are
frozen. A change is a MAJOR version bump and requires a new ADR.

The provenance shape is frozen as of WBS 17.4.1. Adding a new
provenance field is a MINOR change; removing or renaming one is a
MAJOR change.

The human format is stable enough to grep but is not a contract. A
user who needs a stable format uses `--format json`.

The verbose format is not a contract. Fields may be added. Existing
lines are not reformatted. A future field that appears in the
verbose format does not appear in the simple human or JSON formats
unless this document says so.

The `--verbose` flag and the `--format` flag are independent. Any
combination is valid. A future change that binds them (for example,
making `--verbose` imply `--format json`) would be a MAJOR change.

## How the output is verified

Three layers:

1. **Unit tests** in `internal/version/format_json_test.go` and
   `internal/version/format_verbose_test.go` assert the exact output
   for fixed inputs. A change to the format fails a test in
   milliseconds.

2. **Schema tests** in `internal/version/schema_test.go` validate the
   output of `WriteJSON` against
   [`schemas/version.schema.json`](../../schemas/version.schema.json)
   and the output of `WriteFullJSON` against
   [`schemas/version-full.schema.json`](../../schemas/version-full.schema.json).
   The schema and the Go writer are two independent implementations
   of the same contract; the tests keep them in agreement.

3. **Golden files** in `internal/cli/testdata/` capture the help
   output for each command, including the version command's Examples
   block. `TestHelpGolden_Subcommands/version` fails when the help
   text changes without a corresponding golden-file update.

The `task version:verify` target runs all three.

## Related documents

| Document | Role |
|---|---|
| [`versioning.md`](./versioning.md) | Version string semantics and the release channels |
| [`release-channels.md`](./release-channels.md) | Channel definitions and tag-to-release routing |
| [`building.md`](../guides/building.md) | Build entry points and preconditions |
| [`reproducible-builds.md`](../guides/reproducible-builds.md) | Determinism and the role of `SOURCE_DATE_EPOCH` |
| [`ADR-046`](../decisions/ADR-046-version-metadata-and-provenance.md) | Why five fields, why `ReadBuildInfo` |
| [`schemas/version.schema.json`](../../schemas/version.schema.json) | The simple JSON shape |
| [`schemas/version-full.schema.json`](../../schemas/version-full.schema.json) | The full JSON shape |
| [`schemas/version-metadata.schema.json`](../../schemas/version-metadata.schema.json) | The policy document's shape (distinct) |
