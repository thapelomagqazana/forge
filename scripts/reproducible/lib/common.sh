#!/usr/bin/env sh
# =============================================================================
# scripts/reproducible/lib/common.sh — shared helpers
# =============================================================================
#
# This file is sourced by the reproducibility scripts in
# scripts/reproducible/. It provides:
#
#   - Consistent output formatting (headers, OK/FAIL markers).
#   - Temporary directory management with exception-safe cleanup.
#   - Detection of whether the environment satisfies the "clean"
#     preconditions.
#   - Colour output that degrades gracefully on non-TTY streams.
#
# The file is POSIX sh, not bash. This makes it portable across Linux,
# macOS, and Windows (via Git Bash or WSL) without requiring bash.
#
# Do not execute this file directly. It is sourced.
# =============================================================================

# ─────────────────────────────────────────────────────────────────────
# Terminal detection
# ─────────────────────────────────────────────────────────────────────
#
# Colour is emitted only when stdout is a TTY. When output is piped or
# redirected, colours are suppressed. This matches the convention used
# by every modern CLI, including Git itself.
#
# The NO_COLOR environment variable (https://no-color.org) forces
# colour off when set, regardless of TTY status.
#
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
    REPRO_C_RESET=$(printf '\033[0m')
    REPRO_C_BOLD=$(printf '\033[1m')
    REPRO_C_RED=$(printf '\033[31m')
    REPRO_C_GREEN=$(printf '\033[32m')
    REPRO_C_YELLOW=$(printf '\033[33m')
    REPRO_C_BLUE=$(printf '\033[34m')
    REPRO_C_DIM=$(printf '\033[2m')
else
    REPRO_C_RESET=""
    REPRO_C_BOLD=""
    REPRO_C_RED=""
    REPRO_C_GREEN=""
    REPRO_C_YELLOW=""
    REPRO_C_BLUE=""
    REPRO_C_DIM=""
fi

# ─────────────────────────────────────────────────────────────────────
# Output helpers
# ─────────────────────────────────────────────────────────────────────

# repro_header prints a bold section header.
repro_header() {
    printf '\n%s%sForge %s%s\n' \
        "$REPRO_C_BOLD" "$REPRO_C_BLUE" "$1" "$REPRO_C_RESET"
    printf '%s%s%s\n' \
        "$REPRO_C_DIM" "─────────────────────────────────────────" "$REPRO_C_RESET"
}

# repro_ok prints a success line.
repro_ok() {
    printf '%s✓%s %s\n' "$REPRO_C_GREEN" "$REPRO_C_RESET" "$1"
}

# repro_fail prints a failure line to stderr.
repro_fail() {
    printf '%s✗%s %s\n' "$REPRO_C_RED" "$REPRO_C_RESET" "$1" >&2
}

# repro_warn prints a warning line to stderr.
repro_warn() {
    printf '%s!%s %s\n' "$REPRO_C_YELLOW" "$REPRO_C_RESET" "$1" >&2
}

# repro_info prints an informational line.
repro_info() {
    printf '%s→%s %s\n' "$REPRO_C_BLUE" "$REPRO_C_RESET" "$1"
}

# repro_detail prints an indented detail line.
repro_detail() {
    printf '  %s\n' "$1"
}

# ─────────────────────────────────────────────────────────────────────
# Temporary directory management
# ─────────────────────────────────────────────────────────────────────

# repro_mktemp creates a temporary directory and registers it for
# cleanup when the script exits.
#
# The cleanup is registered via a trap on EXIT. Multiple calls to
# repro_mktemp accumulate: all directories are removed when the
# script exits, regardless of success or failure.
#
# Usage:
#   tmpdir=$(repro_mktemp)
repro_mktemp() {
    dir=$(mktemp -d)
    if [ -z "${REPRO_TMPDIRS:-}" ]; then
        REPRO_TMPDIRS="$dir"
        # Install the trap the first time.
        trap 'repro_cleanup' EXIT
    else
        REPRO_TMPDIRS="$REPRO_TMPDIRS $dir"
    fi
    echo "$dir"
}

# repro_cleanup removes every temporary directory created by
# repro_mktemp. It is installed as an EXIT trap and is idempotent.
repro_cleanup() {
    if [ -z "${REPRO_TMPDIRS:-}" ]; then
        return 0
    fi
    for d in $REPRO_TMPDIRS; do
        rm -rf "$d" 2>/dev/null || true
    done
    REPRO_TMPDIRS=""
}

# ─────────────────────────────────────────────────────────────────────
# Clean-environment detection
# ─────────────────────────────────────────────────────────────────────

# repro_has_clean_cache returns 0 if the environment appears to have a
# clean Go module cache, 1 otherwise.
#
# "Clean" means GOMODCACHE points to a directory that does not yet
# exist, or points to an empty directory. A non-empty cache may
# contain modules from previous resolutions, which would defeat the
# purpose of the check.
#
# The function does not modify the environment. It only inspects it.
repro_has_clean_cache() {
    cache=$(go env GOMODCACHE 2>/dev/null || echo "")
    if [ -z "$cache" ]; then
        return 1
    fi
    if [ ! -d "$cache" ]; then
        # Directory does not exist; Go will create it. This is clean.
        return 0
    fi
    if [ -z "$(ls -A "$cache" 2>/dev/null)" ]; then
        # Directory exists and is empty. This is clean.
        return 0
    fi
    # Directory exists and is non-empty.
    return 1
}

# repro_assert_go_available fails the script if the Go toolchain is
# not available on PATH.
repro_assert_go_available() {
    if ! command -v go >/dev/null 2>&1; then
        repro_fail "go toolchain is not available on PATH"
        printf '\n' >&2
        printf 'Install Go from https://go.dev/dl/ or via your package manager.\n' >&2
        return 1
    fi
    return 0
}