#!/usr/bin/env sh
# =============================================================================
# scripts/deps/verify-pin.sh — pin verification (WBS 2.5.1)
# =============================================================================
#
# Verify that every direct dependency in go.mod is pinned to an exact
# semantic version.
#
# A valid version is:
#
#   v<major>.<minor>.<patch>
#   v<major>.<minor>.<patch>-<prerelease>
#
# Rejected forms:
#
#   - Ranges (e.g., `>= v1.0.0`)
#   - Branches (e.g., `master`)
#   - `latest`
#   - Pseudo-versions (e.g., `v0.0.0-20240101120000-abcdef123456`)
#   - `+incompatible` versions
#
# This script is invoked by the `verify:deps:pin` task in Taskfile.yml.
# It is a separate script, rather than inline shell in the Taskfile,
# because Go's `go list -m -f` template syntax collides with Task's
# own template parser when inlined.
#
# Exit codes:
#
#   0  All direct dependencies are pinned correctly.
#   1  One or more dependencies are not pinned correctly.
#
# Usage:
#
#   sh scripts/deps/verify-pin.sh
# =============================================================================

set -eu

# Locate the repository root. The script may be invoked from any
# subdirectory. `git rev-parse` gives the canonical root, which is
# where go.mod lives.
repo_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$repo_root"

# Extract direct dependencies.
#
# `go list -m -json all` produces one JSON object per module. Each
# object has fields including `Path`, `Version`, and (optionally)
# `Indirect`. We parse the JSON with awk to avoid depending on `jq`.
#
# The parsing is deliberately simple:
#
#   - A line starting with `{` begins a new module record.
#   - `"Path": "..."` sets the current path.
#   - `"Version": "..."` sets the current version.
#   - `"Indirect": true` marks the current module as indirect.
#   - A line starting with `}` ends the record; emit if direct.
#
mainmodule=$(go list -m)

direct=$(
    go list -m -json all 2>/dev/null \
    | awk '
        /^\{/          { path=""; version=""; indirect=0 }
        /"Path":/      {
            gsub(/.*"Path": *"/, ""); gsub(/".*/, "");
            path=$0
        }
        /"Version":/   {
            gsub(/.*"Version": *"/, ""); gsub(/".*/, "");
            version=$0
        }
        /"Indirect": true/ { indirect=1 }
        /^\}/          {
            if (path != "" && !indirect) print path " " version
        }
    ' \
    | grep -v "^$mainmodule " \
    || true
)

if [ -z "$direct" ]; then
    echo "OK: no direct dependencies to verify"
    exit 0
fi

fail=0

# Use a here-string rather than a pipe so the loop runs in the current
# shell. A piped loop would run in a subshell, and the `fail` variable
# would not propagate.
while IFS=' ' read -r path version; do
    [ -z "$path" ] && continue

    # Reject pseudo-versions.
    if echo "$version" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+-[0-9]{14}-[a-f0-9]+$'; then
        echo "FAIL: $path uses a pseudo-version: $version" >&2
        fail=1
        continue
    fi

    # Reject +incompatible.
    if echo "$version" | grep -q '+incompatible'; then
        echo "FAIL: $path uses +incompatible: $version" >&2
        fail=1
        continue
    fi

    # Accept exact semver, optionally with a prerelease tag.
    if echo "$version" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$'; then
        echo "OK: $path is pinned to $version"
    else
        echo "FAIL: $path is not pinned to an exact version: $version" >&2
        fail=1
    fi
done <<EOF
$direct
EOF

exit $fail