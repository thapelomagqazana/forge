#!/usr/bin/env sh
# =============================================================================
# scripts/deps/detect-drift.sh — go.mod / go.sum drift detection
# =============================================================================
#
# Detect the five drift types defined in WBS 3.3.1 and report each
# finding with its category, cause, and fix procedure.
#
# # What is drift?
#
# Drift is any condition in which go.mod or go.sum does not
# accurately reflect the module's actual dependency requirements.
# Drift is insidious: it does not break the current build, but it
# breaks future builds on other machines, in CI, or after a
# dependency update.
#
# # The five drift types
#
#   D1  go.sum is missing entries required by go.mod.
#       Detected by: go mod verify.
#       Cause: a dependency was added to go.mod without regenerating
#       go.sum.
#       Fix: run `go mod tidy`, review the diff, commit.
#
#   D2  go.sum has extra entries not referenced by go.mod.
#       Detected by: `go mod tidy` produces a diff that removes
#       lines from go.sum.
#       Cause: a dependency was removed from go.mod without
#       regenerating go.sum.
#       Fix: run `go mod tidy`, review the diff, commit.
#
#   D3  go.mod is not tidy (unused require, missing indirect).
#       Detected by: `go mod tidy` produces a diff that changes
#       go.mod.
#       Cause: a source file was edited to add or remove an import,
#       but go.mod was not regenerated.
#       Fix: run `go mod tidy`, review the diff, commit.
#
#   D4  go.mod / go.sum inconsistent with the pinned toolchain.
#       Detected by: `go build ./...` fails with a version error.
#       Cause: a `require` directive declares a Go version newer
#       than the pinned toolchain, or a language feature is used
#       that the toolchain does not support.
#       Fix: either upgrade the toolchain or downgrade the
#       requirement, per the Go version matrix.
#
#   D5  Unpinned dependency version (+incompatible, pseudo-version).
#       Detected by: regex match against go.mod.
#       Cause: a dependency was added with `go get @master` or
#       `go get` without an explicit version.
#       Fix: run `go get <module>@<version>` with an explicit
#       semantic version.
#
# # Why a separate script
#
# The drift detection logic is invoked from CI, from the pre-push
# hook, and from `task check`. Each caller has different output
# requirements: CI wants machine-parseable output and a fast
# exit; a developer wants a detailed diff and fix procedure.
#
# Encoding the logic in a single script with a `--format` flag
# satisfies both callers. The alternative — duplicating the logic
# across callers — would drift (pun intended) over time.
#
# # Exit codes
#
#   0  No drift detected.
#   1  One or more drift types detected.
#   2  The script could not run (missing tooling, unexpected state).
#
# # Usage
#
#   sh scripts/deps/detect-drift.sh
#   sh scripts/deps/detect-drift.sh --format=json
#
#   # From the repository root:
#   task verify:deps:drift
#
# # POSIX compatibility
#
# The script is POSIX sh. It works on Linux, macOS, and Windows
# (via Git Bash or WSL) without requiring bash.
#
# The script deliberately avoids bash-specific features such as
# process substitution (`<(...)`), arrays, and `[[ ]]`. These
# features are unavailable in strict POSIX shells such as dash,
# which is the default `sh` on Debian and Ubuntu.
# =============================================================================

set -eu

# ─────────────────────────────────────────────────────────────────────
# Argument parsing
# ─────────────────────────────────────────────────────────────────────

format="text"
while [ $# -gt 0 ]; do
    case "$1" in
        --format=*)
            format="${1#--format=}"
            ;;
        --format)
            shift
            format="${1:-text}"
            ;;
        -h|--help)
            cat <<'USAGE'
Usage: detect-drift.sh [--format=text|json]

Detect go.mod / go.sum drift. Reports each finding with its
category, cause, and fix procedure.

Exit codes:
  0  No drift detected.
  1  One or more drift types detected.
  2  The script could not run.

Options:
  --format=text   Human-readable output (default).
  --format=json   Machine-readable JSON output.
USAGE
            exit 0
            ;;
        *)
            echo "unknown argument: $1" >&2
            exit 2
            ;;
    esac
    shift
done

case "$format" in
    text|json) ;;
    *)
        echo "unknown format: $format" >&2
        echo "expected 'text' or 'json'" >&2
        exit 2
        ;;
esac

# ─────────────────────────────────────────────────────────────────────
# Locate the repository root
# ─────────────────────────────────────────────────────────────────────

repo_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$repo_root"

# ─────────────────────────────────────────────────────────────────────
# Ensure required tooling is available
# ─────────────────────────────────────────────────────────────────────

if ! command -v go >/dev/null 2>&1; then
    echo "error: go toolchain is not available on PATH" >&2
    exit 2
fi

if [ ! -f go.mod ]; then
    echo "error: go.mod is missing at the repository root" >&2
    exit 2
fi

if [ ! -f go.sum ]; then
    echo "error: go.sum is missing at the repository root" >&2
    exit 2
fi

# ─────────────────────────────────────────────────────────────────────
# Drift collection
# ─────────────────────────────────────────────────────────────────────
#
# Each check appends findings to a temporary file, one per line, in
# the format:
#
#   <id>\t<description>
#
# At the end, the findings are rendered in the requested format.
#
findings_file=$(mktemp -t forge-drift-findings.XXXXXX)
# shellcheck disable=SC2064
trap "rm -f '$findings_file'" EXIT

record() {
    id="$1"
    shift
    printf '%s\t%s\n' "$id" "$*" >> "$findings_file"
}

# ─────────────────────────────────────────────────────────────────────
# D5 — unpinned dependency versions
# ─────────────────────────────────────────────────────────────────────
#
# Scans go.mod for version strings that are not exact semantic
# versions. Runs first because it is pure text analysis (no
# toolchain invocation) and because its findings affect the
# interpretation of the later checks.
#
# The pattern matches:
#   - +incompatible versions
#   - pseudo-versions (v0.0.0-YYYYMMDD-<hash>)
#   - branch names (@main, @master)
#   - commit hashes
#
# Exact semantic versions match `v<major>.<minor>.<patch>` with an
# optional prerelease suffix. They are not flagged.
#
# # Comment handling
#
# A require line may have a trailing comment, most commonly
# `// indirect`. The comment is added by the Go toolchain for
# transitive dependencies. It is not a version and must not be
# treated as one.
#
# The `sed 's|//.*$||'` filter strips the comment before the
# version is extracted. Without this filter, a line like:
#
#     github.com/spf13/pflag v1.0.5 // indirect
#
# would extract `indirect` as the version (the last field),
# producing a false positive.
#
while IFS= read -r line; do
    [ -z "$line" ] && continue

    # Strip trailing comments. In go.mod, a comment starts with //
    # and continues to the end of the line.
    stripped=$(printf '%s\n' "$line" | sed 's|//.*$||')

    # Extract the last field of the stripped line. This is the
    # version.
    version=$(printf '%s\n' "$stripped" | awk '{print $NF}')
    [ -z "$version" ] && continue

    case "$version" in
        *+incompatible*)
            record "D5" "unpinned dependency version (has +incompatible): $line"
            ;;
        v0.0.0-*-*)
            record "D5" "unpinned dependency version (pseudo-version): $line"
            ;;
        v[0-9]*.[0-9]*.[0-9]*) ;;
        *)
            record "D5" "unpinned dependency version (not semver): $line"
            ;;
    esac
done <<EOF
$(grep -E '^\s+[a-z0-9.-]+\.[a-z]{2,}/[^ ]+ v[0-9]' go.mod || true)
$(grep -E '^require\s+[a-z0-9.-]+\.[a-z]{2,}/[^ ]+ v[0-9]' go.mod || true)
EOF

# ─────────────────────────────────────────────────────────────────────
# D1 — go.sum missing entries required by go.mod
# ─────────────────────────────────────────────────────────────────────
#
# `go mod verify` checks that every module in the build list has a
# corresponding entry in go.sum and that the entry's checksum
# matches the module's actual content. A missing entry causes the
# command to fail.
#
# This is the most severe drift type: it prevents a fresh clone
# from building.
#
if ! go mod verify >/dev/null 2>&1; then
    record "D1" "go.sum is missing entries required by go.mod (go mod verify failed)"
fi

# ─────────────────────────────────────────────────────────────────────
# D2 and D3 — go.sum or go.mod not tidy
# ─────────────────────────────────────────────────────────────────────
#
# `go mod tidy` is idempotent on a clean module. If it produces a
# diff, the module is not tidy.
#
# The check is performed in a temporary copy of the module so that
# the repository is not modified. The copy contains go.mod, go.sum,
# and the source tree — enough for `go mod tidy` to determine the
# correct state.
#
# The diff is analysed to classify the drift:
#
#   - A change to go.sum alone is D2 (extra entries).
#   - A change to go.mod alone is D3 (not tidy).
#   - A change to both is D2 + D3.
#
# The distinction matters for the fix procedure: D2 is a
# consequence of a dependency removal; D3 is a consequence of a
# source-file edit.
#
tmpdir=$(mktemp -d -t forge-drift-tidy.XXXXXX)
trap "rm -rf '$tmpdir' '$findings_file'" EXIT

cp go.mod "$tmpdir/go.mod"
cp go.sum "$tmpdir/go.sum"
cp -R cmd "$tmpdir/cmd" 2>/dev/null || true
cp -R internal "$tmpdir/internal" 2>/dev/null || true

cp go.mod "$tmpdir/go.mod.orig"
cp go.sum "$tmpdir/go.sum.orig"

(
    cd "$tmpdir"
    go mod tidy >/dev/null 2>&1 || true
)

if ! diff -q "$tmpdir/go.mod.orig" "$tmpdir/go.mod" >/dev/null 2>&1; then
    record "D3" "go.mod is not tidy (go mod tidy produces a diff)"
fi

if ! diff -q "$tmpdir/go.sum.orig" "$tmpdir/go.sum" >/dev/null 2>&1; then
    record "D2" "go.sum has entries not referenced by go.mod (go mod tidy produces a diff)"
fi

# ─────────────────────────────────────────────────────────────────────
# D4 — inconsistent with pinned toolchain
# ─────────────────────────────────────────────────────────────────────
#
# A `go build ./...` that fails with a version-related error
# indicates the module requires a Go version newer than the
# pinned toolchain, or a language feature that the toolchain does
# not support.
#
# The check is performed in the temporary copy to avoid side
# effects (build artefacts) in the working tree.
#
if ! (
    cd "$tmpdir"
    go build ./... >/dev/null 2>&1
); then
    # Distinguish a version-related failure from other build
    # failures. A build failure caused by a compile error is not
    # drift.
    if (
        cd "$tmpdir"
        go build ./... 2>&1
    ) | grep -qE 'requires go >=|go\.mod requires'; then
        record "D4" "go.mod requires a newer Go version than the pinned toolchain"
    fi
fi

# ─────────────────────────────────────────────────────────────────────
# Render findings
# ─────────────────────────────────────────────────────────────────────

if [ ! -s "$findings_file" ]; then
    case "$format" in
        text)
            echo "OK: no drift detected"
            ;;
        json)
            printf '%s\n' '{"drift": false, "findings": []}'
            ;;
    esac
    exit 0
fi

case "$format" in
    text)
        echo "FAIL: dependency drift detected"
        echo ""
        while IFS='	' read -r id description; do
            printf '  [%s] %s\n' "$id" "$description"
        done < "$findings_file"
        echo ""
        echo "Fix procedures:"
        echo ""
        echo "  D1, D2, D3 — run 'go mod tidy' on a clean branch."
        echo "    Inspect the diff. Commit both go.mod and go.sum."
        echo ""
        echo "  D4 — align the pinned toolchain with go.mod."
        echo "    See docs/development.md 'Supported Go Versions'."
        echo ""
        echo "  D5 — re-pin the dependency with an explicit version:"
        echo "    go get <module>@<version>"
        echo ""
        echo "See docs/development.md 'Detecting Dependency Drift'"
        echo "for the full reference."
        ;;
    json)
        printf '{"drift": true, "findings": ['
        first=1
        while IFS='	' read -r id description; do
            [ "$first" -eq 1 ] || printf ','
            first=0
            # Escape the description for JSON. The descriptions
            # contain no special characters beyond the ones we
            # handle here.
            escaped=$(printf '%s' "$description" | sed 's/\\/\\\\/g; s/"/\\"/g')
            printf '{"id":"%s","description":"%s"}' "$id" "$escaped"
        done < "$findings_file"
        printf ']}\n'
        ;;
esac

exit 1