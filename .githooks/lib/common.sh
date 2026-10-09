#!/usr/bin/env sh
# =============================================================================
# .githooks/lib/common.sh — shared helpers for Forge Git hooks
# =============================================================================
#
# This file is sourced by every hook in .githooks/. It provides:
#
#   - Consistent output formatting (headers, OK/FAIL markers).
#   - A command runner that reports success and failure uniformly.
#   - Detection of whether `task` is installed.
#   - Colour output that degrades gracefully on non-TTY streams.
#
# The file is POSIX sh, not bash. This makes it portable across Linux,
# macOS, and Windows (via Git Bash or WSL) without requiring bash.
#
# Do not execute this file directly. It is sourced.
# =============================================================================

# ─────────────────────────────────────────────────────────────────────────────
# Terminal detection
# ─────────────────────────────────────────────────────────────────────────────
#
# Colour is emitted only when stdout is a TTY. When output is piped or
# redirected, colours are suppressed. This matches the convention used
# by every modern CLI, including Git itself.
#
# The NO_COLOR environment variable (https://no-color.org) forces
# colour off when set, regardless of TTY status.
#
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
    HOOK_C_RESET=$(printf '\033[0m')
    HOOK_C_BOLD=$(printf '\033[1m')
    HOOK_C_RED=$(printf '\033[31m')
    HOOK_C_GREEN=$(printf '\033[32m')
    HOOK_C_YELLOW=$(printf '\033[33m')
    HOOK_C_BLUE=$(printf '\033[34m')
    HOOK_C_DIM=$(printf '\033[2m')
else
    HOOK_C_RESET=""
    HOOK_C_BOLD=""
    HOOK_C_RED=""
    HOOK_C_GREEN=""
    HOOK_C_YELLOW=""
    HOOK_C_BLUE=""
    HOOK_C_DIM=""
fi

# ─────────────────────────────────────────────────────────────────────────────
# Output helpers
# ─────────────────────────────────────────────────────────────────────────────

# hook_header prints a bold section header.
#
# Usage: hook_header "pre-commit"
hook_header() {
    printf '\n%s%sForge %s%s\n' \
        "$HOOK_C_BOLD" "$HOOK_C_BLUE" "$1" "$HOOK_C_RESET"
    printf '%s%s%s\n' \
        "$HOOK_C_DIM" "─────────────────────────────────────────" "$HOOK_C_RESET"
}

# hook_ok prints a success line.
#
# Usage: hook_ok "go.mod is tidy"
hook_ok() {
    printf '%s✓%s %s\n' "$HOOK_C_GREEN" "$HOOK_C_RESET" "$1"
}

# hook_fail prints a failure line.
#
# Usage: hook_fail "go.mod is not tidy"
hook_fail() {
    printf '%s✗%s %s\n' "$HOOK_C_RED" "$HOOK_C_RESET" "$1" >&2
}

# hook_warn prints a warning line.
#
# Usage: hook_warn "task is not installed; skipping"
hook_warn() {
    printf '%s!%s %s\n' "$HOOK_C_YELLOW" "$HOOK_C_RESET" "$1" >&2
}

# hook_info prints an informational line.
#
# Usage: hook_info "running task fmt:check"
hook_info() {
    printf '%s→%s %s\n' "$HOOK_C_BLUE" "$HOOK_C_RESET" "$1"
}

# ─────────────────────────────────────────────────────────────────────────────
# Task detection
# ─────────────────────────────────────────────────────────────────────────────

# hook_have_task returns 0 if `task` is available on PATH, 1 otherwise.
hook_have_task() {
    command -v task >/dev/null 2>&1
}

# hook_require_task fails the hook with a clear message when `task` is
# not installed. Hooks that require `task` should call this at the top.
#
# The message is intentionally actionable: it tells the contributor
# where to install the tool, not just that it is missing.
hook_require_task() {
    if hook_have_task; then
        return 0
    fi

    hook_fail "task is not installed"
    printf '\n' >&2
    printf 'Forge hooks depend on the task runner. Install it with:\n' >&2
    printf '  https://taskfile.dev/installation/\n' >&2
    printf '\n' >&2
    printf 'On macOS with Homebrew:\n' >&2
    printf '  brew install go-task\n' >&2
    printf '\n' >&2
    printf 'On Linux with a package manager:\n' >&2
    printf '  see https://taskfile.dev/installation/\n' >&2
    printf '\n' >&2
    printf 'To bypass this hook for one commit:\n' >&2
    printf '  git commit --no-verify\n' >&2
    printf '\n' >&2
    return 1
}

# ─────────────────────────────────────────────────────────────────────────────
# Task runner
# ─────────────────────────────────────────────────────────────────────────────

# hook_run_task runs a task target and reports its result.
#
# On success, prints an OK line. On failure, prints a FAIL line and
# returns the exit code from `task`. The caller is expected to
# accumulate the result and exit non-zero if any task failed.
#
# Usage:
#   if hook_run_task "fmt:check" "Go formatting"; then
#       :
#   else
#       HOOK_FAILED=1
#   fi
hook_run_task() {
    target="$1"
    label="${2:-$target}"

    hook_info "$label"
    if task "$target" >/dev/null 2>&1; then
        hook_ok "$label"
        return 0
    fi

    # Re-run without suppression so the contributor sees the failure
    # detail. The double-invocation is deliberate: the first run keeps
    # the success path quiet, and the second run only happens on
    # failure, where verbosity is desirable.
    hook_fail "$label"
    printf '\n' >&2
    task "$target" >&2 || true
    printf '\n' >&2
    return 1
}

# ─────────────────────────────────────────────────────────────────────────────
# Failure aggregation
# ─────────────────────────────────────────────────────────────────────────────

# hook_summarise prints a summary of failed checks and the bypass hint.
#
# Call this at the end of a hook if any check failed.
#
# Usage:
#   if [ "$HOOK_FAILED" -ne 0 ]; then
#       hook_summarise "commit"
#       exit 1
#   fi
hook_summarise() {
    action="${1:-push}"
    printf '\n' >&2
    printf '%s%sSome checks failed.%s\n' \
        "$HOOK_C_BOLD" "$HOOK_C_RED" "$HOOK_C_RESET" >&2
    printf '\n' >&2
    printf 'Fix the issues above, or bypass the hook with:\n' >&2

    case "$action" in
        commit)
            printf '  git commit --no-verify\n' >&2
            ;;
        push)
            printf '  git push --no-verify\n' >&2
            ;;
        *)
            printf '  git <command> --no-verify\n' >&2
            ;;
    esac
    printf '\n' >&2
    printf 'Bypassing is discouraged. Prefer fixing the issue.\n' >&2
    printf '\n' >&2
}