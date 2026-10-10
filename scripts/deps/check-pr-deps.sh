#!/usr/bin/env sh
# =============================================================================
# scripts/deps/check-pr-deps.sh — pull request dependency check
# =============================================================================
#
# Detect policy violations in a pull request that modifies go.mod.
#
# # What this script detects
#
#   V1  (best-effort) New direct dependency added without a policy
#       entry or ADR.
#   V2  Version pin changed from exact to range.
#   V3  +incompatible version marker.
#   V4  Pseudo-version in a direct dependency.
#
# V5 (license incompatibility) is deferred to Phase 13 and is not
# checked by this script.
#
# # How it works
#
# The script inspects go.mod and, when a base revision is provided,
# the git diff against that revision.
#
#   - V2, V3, V4 are pure regex checks against the current go.mod.
#     They do not need a base revision.
#   - V1 requires a base revision. The script compares the current
#     go.mod against the base to determine whether a new direct
#     dependency was added. If a new dependency is added, the script
#     emits a warning that prompts the reviewer to verify the ADR
#     and the registry entry.
#
# # Scope of V2, V3, and V4
#
# V2, V3, and V4 apply only to **direct** dependencies (the require
# lines that do not carry the "// indirect" comment). Indirect
# dependencies are selected by the Go toolchain to satisfy the direct
# dependencies' version constraints; they are recorded by `go mod
# tidy` and are not subject to hand-pinning. Applying the checks to
# indirect lines would produce false positives on every transitive
# dependency whose version happens not to be the latest patch — and
# would contradict `verify:deps:integrity`, which requires that
# `go mod tidy` produces no diff.
#
# The scoping was introduced after
# `github.com/inconshreveable/mousetrap v1.1.0 // indirect` — a
# transitive dependency of Cobra — was flagged by the V2 check as a
# version-pin violation. The flag was a false positive: the line
# carries an `// indirect` comment, and the version is not the last
# field on the line. The check now filters indirect lines and
# extracts the version by pattern, not by position.
#
# See docs/dependency-policy.md, section "V2 — Version pin", for the
# policy's wording.
#
# # Why V1 is a warning, not an error
#
# The script can detect that a dependency was added. It cannot
# detect whether the addition is justified. Justification is a
# review decision, not a mechanical check. The warning is a prompt
# for the reviewer, not a build failure.
#
# # Exit codes
#
#   0  No violations detected. (Warnings do not cause a non-zero
#      exit.)
#   1  One or more V2, V3, or V4 violations detected.
#   2  The script could not run (missing tooling, unexpected state).
#
# # Usage
#
#   # Check the current go.mod only (V2, V3, V4).
#   sh scripts/deps/check-pr-deps.sh
#
#   # Check against a base revision (adds V1 detection).
#   sh scripts/deps/check-pr-deps.sh --base=origin/main
#
#   # From the repository root:
#   task verify:deps:pr-check
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

base=""
while [ $# -gt 0 ]; do
    case "$1" in
        --base=*)
            base="${1#--base=}"
            ;;
        --base)
            shift
            base="${1:-}"
            ;;
        -h|--help)
            cat <<'USAGE'
Usage: check-pr-deps.sh [--base=<revision>]

Detect policy violations in a pull request that modifies go.mod.

Options:
  --base=<revision>   Compare against the given git revision to
                      detect new direct dependencies (V1).
                      Example: --base=origin/main

Exit codes:
  0  No violations detected.
  1  V2, V3, or V4 violation detected.
  2  The script could not run.
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

# ─────────────────────────────────────────────────────────────────────
# Locate the repository root
# ─────────────────────────────────────────────────────────────────────

repo_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$repo_root"

if [ ! -f go.mod ]; then
    echo "error: go.mod is missing at the repository root" >&2
    exit 2
fi

# ─────────────────────────────────────────────────────────────────────
# Violation collection
# ─────────────────────────────────────────────────────────────────────

errors=0
warnings=0

error() {
    printf 'FAIL: %s\n' "$1" >&2
    errors=$((errors + 1))
}

warn() {
    printf 'WARN: %s\n' "$1" >&2
    warnings=$((warnings + 1))
}

ok() {
    printf 'OK: %s\n' "$1"
}

# ─────────────────────────────────────────────────────────────────────
# V2, V3, V4 — regex checks against the current go.mod
# ─────────────────────────────────────────────────────────────────────
#
# The checks apply to **direct** dependencies only. Indirect
# dependencies (those carrying the "// indirect" comment) are
# selected by the Go toolchain and are out of scope for V2, V3, and
# V4. The policy is documented in docs/dependency-policy.md, section
# "V2 — Version pin".
#
# # How the require lines are extracted
#
# The extraction has three steps:
#
#   1. Find every line in go.mod that resembles a require entry.
#      Both the block form and the single-line form are handled:
#
#          require (
#              github.com/spf13/cobra v1.8.1
#              github.com/spf13/pflag v1.0.5 // indirect
#          )
#
#          require github.com/spf13/cobra v1.8.1
#
#      The pattern matches lines that contain a module path (a
#      dotted path with a slash) followed by a version token
#      starting with "v" and a digit.
#
#   2. Exclude lines that carry the "// indirect" comment. Those
#      lines are the toolchain's choices, not Forge's.
#
#   3. From each remaining line, extract the version. The version
#      is the first whitespace-separated token that starts with
#      "v" and a digit. This is more robust than using the last
#      field, which for an indirect line would be the comment
#      itself. It is also correct for direct lines that carry a
#      trailing comment for other reasons (for example, a "//
#      pinned by ADR-006" annotation).
#
# The extraction is deliberately conservative: it does not attempt
# to parse go.mod. Parsing is unnecessary because the patterns are
# unambiguous, and a hand-rolled parser would be larger than the
# checks it supports.

require_lines=$(
    grep -E '^[[:space:]]*([a-z0-9.-]+\.[a-z]{2,}/[^[:space:]]+[[:space:]]+v[0-9])' \
        go.mod \
    | grep -v '// indirect' \
    || true
)

# The single-line require form does not start with whitespace, so
# the pattern above does not match it. Add it separately.
#
# A single-line require looks like:
#
#     require github.com/spf13/cobra v1.8.1
#
# The pattern below matches the whole line, excluding the leading
# "require " keyword so that the extraction below (which scans for
# the first version-like token) sees only the module path and the
# version.
single_require_lines=$(
    grep -E '^require[[:space:]]+[a-z0-9.-]+\.[a-z]{2,}/[^[:space:]]+[[:space:]]+v[0-9]' \
        go.mod \
    | sed -E 's/^require[[:space:]]+//' \
    || true
)

require_lines=$(printf '%s\n%s\n' "$require_lines" "$single_require_lines")

v2_count=0
v3_count=0
v4_count=0

while IFS= read -r line; do
    [ -z "$line" ] && continue

    # Extract the version: the first whitespace-separated token
    # that starts with "v" and a digit. This is robust against
    # trailing comments and against the version not being the last
    # field.
    version=""
    for token in $line; do
        case "$token" in
            v[0-9]*)
                version="$token"
                break
                ;;
        esac
    done

    if [ -z "$version" ]; then
        # The line does not carry a version-like token. This should
        # not happen given the extraction above, but the check is
        # defensive: a line without a version is not a violation
        # this script knows how to classify, so it is skipped.
        continue
    fi

    # V3 — +incompatible marker.
    case "$version" in
        *+incompatible*)
            error "V3: +incompatible marker: $line"
            v3_count=$((v3_count + 1))
            continue
            ;;
    esac

    # V4 — pseudo-version.
    # A pseudo-version has the form v0.0.0-YYYYMMDDHHMMSS-<hash>.
    case "$version" in
        v0.0.0-[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]-*)
            error "V4: pseudo-version: $line"
            v4_count=$((v4_count + 1))
            continue
            ;;
    esac

    # V2 — version pin changed from exact to range.
    # Accept only the exact semver form, optionally with a
    # prerelease suffix.
    case "$version" in
        v[0-9]*.[0-9]*.[0-9]*)
            # Check that it does not contain a range character.
            case "$version" in
                *">"*|*"<"*|*"^"*|*"~"*|*"*"*)
                    error "V2: version range: $line"
                    v2_count=$((v2_count + 1))
                    ;;
            esac
            ;;
        *)
            # Not an exact semver. This is a violation.
            error "V2: not an exact semantic version: $line"
            v2_count=$((v2_count + 1))
            ;;
    esac
done <<EOF
$require_lines
EOF

if [ "$v2_count" -eq 0 ]; then
    ok "no version ranges in direct dependencies"
fi
if [ "$v3_count" -eq 0 ]; then
    ok "no +incompatible markers in direct dependencies"
fi
if [ "$v4_count" -eq 0 ]; then
    ok "no pseudo-versions in direct dependencies"
fi

# ─────────────────────────────────────────────────────────────────────
# V1 — new direct dependency (best-effort)
# ─────────────────────────────────────────────────────────────────────
#
# V1 detection requires a base revision. When a base is provided,
# the script:
#
#   1. Extracts the direct dependencies from the base's go.mod.
#   2. Extracts the direct dependencies from the current go.mod.
#   3. Computes the difference (new dependencies).
#   4. For each new dependency, emits a warning that prompts the
#      reviewer to verify the ADR and the registry entry.
#
# The check is skipped if no base is provided. The PR template's
# checklist covers V1 for the case where the script cannot.
#
# The extraction excludes indirect dependencies (the lines
# carrying the "// indirect" comment), for the same reason V2, V3,
# and V4 do: indirect dependencies are the toolchain's choices,
# not Forge's, and a new indirect dependency is not a new direct
# dependency.
#
# # POSIX compatibility
#
# The comparison uses temporary files rather than process
# substitution. Process substitution (`<(...)`) is a bash extension
# and is not available in strict POSIX shells such as dash.
#
if [ -n "$base" ]; then
    if ! git rev-parse --verify "$base" >/dev/null 2>&1; then
        echo "error: base revision not found: $base" >&2
        exit 2
    fi

    # Extract direct dependencies from the base's go.mod.
    base_deps=$(
        git show "$base:go.mod" 2>/dev/null \
        | awk '
            /^require \(/ { in_block=1; next }
            /^\)/ { in_block=0; next }
            in_block && !/\/\/ indirect/ { print $1 }
            /^require [^()]/ { print $2 }
        ' \
        | grep -E '^[a-z0-9.-]+\.[a-z]{2,}/' \
        | sort \
        || true
    )

    current_deps=$(
        awk '
            /^require \(/ { in_block=1; next }
            /^\)/ { in_block=0; next }
            in_block && !/\/\/ indirect/ { print $1 }
            /^require [^()]/ { print $2 }
        ' go.mod \
        | grep -E '^[a-z0-9.-]+\.[a-z]{2,}/' \
        | sort \
        || true
    )

    # Write the two lists to temporary files and use `comm` to
    # compute the difference. Process substitution would be simpler
    # but is not POSIX. The temporary files are removed by the EXIT
    # trap below.
    base_deps_file=$(mktemp)
    current_deps_file=$(mktemp)
    # shellcheck disable=SC2064
    trap "rm -f '$base_deps_file' '$current_deps_file'" EXIT

    printf '%s\n' "$base_deps" > "$base_deps_file"
    printf '%s\n' "$current_deps" > "$current_deps_file"

    new_deps=$(comm -13 "$base_deps_file" "$current_deps_file" || true)

    if [ -n "$new_deps" ]; then
        while IFS= read -r dep; do
            [ -z "$dep" ] && continue
            warn "V1: new direct dependency added: $dep"
            warn "    Verify that an ADR exists and that the dependency is listed"
            warn "    in Section 7 of docs/dependency-policy.md."
        done <<EOF
$new_deps
EOF
    else
        ok "no new direct dependencies"
    fi
else
    printf 'SKIP: V1 check requires a base revision (use --base=<revision>)\n'
fi

# ─────────────────────────────────────────────────────────────────────
# Summary
# ─────────────────────────────────────────────────────────────────────

if [ "$errors" -gt 0 ]; then
    printf '\n' >&2
    printf 'FAIL: %d error(s), %d warning(s) found\n' "$errors" "$warnings" >&2
    printf '\n' >&2
    printf 'See docs/dependency-policy.md "Dependency change detection" for the\n' >&2
    printf 'policy.\n' >&2
    exit 1
fi

if [ "$warnings" -gt 0 ]; then
    printf '\n'
    printf 'OK: no errors, but %d warning(s) require review\n' "$warnings"
    exit 0
fi

echo ""
echo "OK: no dependency policy violations detected"
exit 0