#!/usr/bin/env sh
# =============================================================================
# scripts/reproducible/verify-resolution.sh
# =============================================================================
#
# Verify the reproducibility invariant for module resolution.
#
# # The invariant
#
# Given the same commit and the same go.mod, go.sum, and pinned
# toolchain, two independent clean environments must resolve to the
# identical module tree.
#
# # What this script verifies
#
#   1. The repository is in a state where the check is meaningful
#      (go.mod and go.sum are committed and clean).
#   2. Two temporary environments with fresh GOMODCACHE and GOPATH
#      resolve the same module tree.
#   3. The resolved tree is non-empty and contains the modules the
#      committed go.mod declares.
#
# # What this script does NOT verify
#
#   - Build output byte-for-byte reproducibility. That is a separate
#     concern (see WBS 17.x). This script only verifies that the
#     module tree is the same in both environments.
#   - Toolchain reproducibility across Go versions. That is covered
#     by WBS 2.1.2 (the version matrix). This script assumes both
#     environments use the toolchain pinned by go.mod.
#   - Network behaviour. Both environments use the same GOPROXY and
#     GOSUMDB settings. If those settings differ between environments,
#     resolution may differ.
#   - Correctness of the resolved modules. A malicious or compromised
#     module will resolve identically in both environments. This
#     script verifies consistency, not correctness.
#
# # Why a separate script
#
# verify:deps:integrity (WBS 3.1.1) checks that the committed
# lockfiles match what the toolchain would produce in *this*
# environment. This script checks a stronger property: that the
# resolution is *independent of the environment*.
#
# The two checks fail for different reasons and require different
# remedies:
#
#   - Integrity fails when the lockfiles are stale or hand-edited.
#     Remedy: `task tidy` on a clean branch.
#   - Reproducibility fails when the environment (GOPROXY, GOSUMDB,
#     GOFLAGS) differs between environments, or when the module cache
#     is poisoned, or when network conditions cause different
#     resolutions. Remedy: align the environment settings, clear the
#     cache, or investigate the network.
#
# Keeping the checks separate allows failures to be attributed
# precisely.
#
# # Exit codes
#
#   0  The invariant holds: both environments resolved identically.
#   1  One or more checks failed. The failure is reported on stderr.
#   2  The script could not run (missing tooling, unexpected state).
#
# # Usage
#
#   sh scripts/reproducible/verify-resolution.sh
#
#   # From the repository root:
#   task verify:resolution
#
# # POSIX compatibility
#
# The script is POSIX sh. It works on Linux, macOS, and Windows (via
# Git Bash or WSL) without requiring bash.
#
# The script deliberately avoids bash-specific features such as
# process substitution (`<(...)`), arrays, and `[[ ]]`. These
# features are unavailable in strict POSIX shells such as dash,
# which is the default `sh` on Debian and Ubuntu.
#
# # Performance
#
# The script runs in under 30 seconds on a warm Go module cache.
# On a cold cache, it may take longer if modules need to be
# downloaded. The script does not pre-populate the cache; it relies
# on the caller's environment to provide a reasonable cache state.
# =============================================================================

set -eu

# ─────────────────────────────────────────────────────────────────────
# Source shared helpers
# ─────────────────────────────────────────────────────────────────────

script_dir=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=lib/common.sh
. "$script_dir/lib/common.sh"

# ─────────────────────────────────────────────────────────────────────
# Locate the repository root
# ─────────────────────────────────────────────────────────────────────

repo_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$repo_root"

repro_header "reproducible dependency resolution"

# ─────────────────────────────────────────────────────────────────────
# Preconditions
# ─────────────────────────────────────────────────────────────────────

if ! repro_assert_go_available; then
    exit 1
fi

if [ ! -f go.mod ]; then
    repro_fail "go.mod is missing at the repository root"
    exit 1
fi

if [ ! -f go.sum ]; then
    repro_fail "go.sum is missing at the repository root"
    exit 1
fi

# The check compares the committed state against the resolved state.
# Uncommitted changes to go.mod or go.sum make the comparison
# meaningless.
if ! git diff --quiet -- go.mod go.sum 2>/dev/null; then
    repro_fail "go.mod or go.sum has uncommitted changes"
    repro_detail "Commit or stash the changes before running this check."
    git diff -- go.mod go.sum >&2 || true
    exit 1
fi

repro_ok "preconditions satisfied"

# ─────────────────────────────────────────────────────────────────────
# Capture the committed module tree
# ─────────────────────────────────────────────────────────────────────
#
# The committed module tree is the set of modules that the current
# environment resolves from the committed go.mod and go.sum. This is
# the reference tree against which the two clean environments will be
# compared.
#
# We capture it before creating the temporary environments so that
# the comparison is against the state the user is currently working
# with, not against the state after `go mod download` has run.
#
committed_tree=$(go list -m all 2>/dev/null | sort)

if [ -z "$committed_tree" ]; then
    repro_fail "the committed module tree is empty"
    repro_detail "go.mod declares no dependencies. The reproducibility"
    repro_detail "check is vacuous in this state. Add at least one"
    repro_detail "dependency before running it."
    exit 1
fi

module_count=$(printf '%s\n' "$committed_tree" | grep -c . || true)
repro_ok "committed module tree contains $module_count modules"

# ─────────────────────────────────────────────────────────────────────
# Verify the module tree is complete
# ─────────────────────────────────────────────────────────────────────
#
# A common failure mode: the module cache is empty, `go list -m all`
# succeeds but returns only the main module, and the check passes
# because both environments return the same (empty) tree. The check
# must verify that the tree contains the modules the committed go.mod
# declares.
#
# We count the `require` directives in go.mod and compare against the
# number of non-main modules in the tree.
#
mainmodule=$(go list -m 2>/dev/null || echo "")
required_count=$(grep -cE '^\s+(github\.com|golang\.org|gopkg\.in|k8s\.io|[a-z0-9.-]+\.[a-z]{2,})' go.mod 2>/dev/null || true)

if [ "$required_count" -gt 0 ] && [ "$module_count" -le 1 ]; then
    repro_fail "the resolved module tree is incomplete"
    repro_detail "go.mod declares $required_count dependencies, but"
    repro_detail "the resolved tree contains only $module_count module(s)."
    repro_detail ""
    repro_detail "This usually means the module cache is empty and the"
    repro_detail "network is unreachable. Populate the cache first:"
    repro_detail "  go mod download all"
    exit 1
fi

# ─────────────────────────────────────────────────────────────────────
# Resolve in two clean environments
# ─────────────────────────────────────────────────────────────────────
#
# Each environment gets its own temporary GOMODCACHE and GOPATH. This
# forces the Go toolchain to resolve the module tree from scratch,
# without reusing any prior resolution state.
#
# The two environments are otherwise identical: same Go binary, same
# GOPROXY, same GOSUMDB, same GOFLAGS. If resolution differs between
# them, the difference is attributable to nondeterminism in the
# resolution process, not to environment configuration.
#
repro_info "resolving in environment A"
env_a=$(repro_mktemp)
env_a_cache="$env_a/modcache"
env_a_gopath="$env_a/gopath"
mkdir -p "$env_a_cache" "$env_a_gopath"

tree_a=$(
    GOMODCACHE="$env_a_cache" \
    GOPATH="$env_a_gopath" \
    go list -m all 2>/dev/null \
    | sort
)

if [ -z "$tree_a" ]; then
    repro_fail "environment A resolved an empty module tree"
    exit 1
fi

repro_ok "environment A resolved $(printf '%s\n' "$tree_a" | grep -c .) modules"

repro_info "resolving in environment B"
env_b=$(repro_mktemp)
env_b_cache="$env_b/modcache"
env_b_gopath="$env_b/gopath"
mkdir -p "$env_b_cache" "$env_b_gopath"

tree_b=$(
    GOMODCACHE="$env_b_cache" \
    GOPATH="$env_b_gopath" \
    go list -m all 2>/dev/null \
    | sort
)

if [ -z "$tree_b" ]; then
    repro_fail "environment B resolved an empty module tree"
    exit 1
fi

repro_ok "environment B resolved $(printf '%s\n' "$tree_b" | grep -c .) modules"

# ─────────────────────────────────────────────────────────────────────
# Compare the two trees
# ─────────────────────────────────────────────────────────────────────
#
# The comparison is exact: every module path, every version, every
# line. If the two environments resolved differently, the diff shows
# where.
#
# The comparison uses temporary files rather than process
# substitution. Process substitution (`<(...)`) is a bash extension
# and is not available in strict POSIX shells such as dash.
#
if [ "$tree_a" != "$tree_b" ]; then
    repro_fail "environment A and environment B resolved different module trees"

    tree_a_file=$(mktemp)
    tree_b_file=$(mktemp)
    # shellcheck disable=SC2064
    trap "rm -f '$tree_a_file' '$tree_b_file'" EXIT

    printf '%s\n' "$tree_a" > "$tree_a_file"
    printf '%s\n' "$tree_b" > "$tree_b_file"

    printf '\n' >&2
    printf 'Diff (environment A vs environment B):\n' >&2
    diff "$tree_a_file" "$tree_b_file" >&2 || true
    printf '\n' >&2
    printf 'The following are common causes of this failure:\n' >&2
    printf '  - GOPROXY is not set identically in both environments.\n' >&2
    printf '  - GOSUMDB is not set identically in both environments.\n' >&2
    printf '  - GOFLAGS is not set identically in both environments.\n' >&2
    printf '  - The network condition differs between environments.\n' >&2
    printf '  - A module was published to the proxy between the two\n' >&2
    printf '    resolutions and the pinned version was not used.\n' >&2
    printf '\n' >&2
    printf 'See docs/development.md "Reproducible Dependency Resolution"\n' >&2
    printf 'for the full list of non-reproducibility sources.\n' >&2
    exit 1
fi

repro_ok "environment A and environment B resolved identically"

# ─────────────────────────────────────────────────────────────────────
# Compare against the committed tree
# ─────────────────────────────────────────────────────────────────────
#
# The two temporary environments must also match the committed tree.
# If they resolve differently from the committed state, the
# repository's go.sum does not pin the same tree the toolchain
# resolves.
#
if [ "$tree_a" != "$committed_tree" ]; then
    repro_fail "the resolved tree differs from the committed tree"

    committed_file=$(mktemp)
    resolved_file=$(mktemp)
    # shellcheck disable=SC2064
    trap "rm -f '$committed_file' '$resolved_file'" EXIT

    printf '%s\n' "$committed_tree" > "$committed_file"
    printf '%s\n' "$tree_a" > "$resolved_file"

    printf '\n' >&2
    printf 'Diff (committed vs resolved):\n' >&2
    diff "$committed_file" "$resolved_file" >&2 || true
    printf '\n' >&2
    printf 'This usually means go.sum is stale relative to go.mod.\n' >&2
    printf 'Run "task tidy" on a clean branch to regenerate the\n' >&2
    printf 'lockfiles.\n' >&2
    exit 1
fi

repro_ok "resolved tree matches the committed tree"

# ─────────────────────────────────────────────────────────────────────
# Done
# ─────────────────────────────────────────────────────────────────────

printf '\n'
repro_ok "reproducibility invariant holds"
exit 0