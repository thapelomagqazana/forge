#!/usr/bin/env sh
# =============================================================================
# scripts/install-hooks.sh — install Forge Git hooks into the local clone
# =============================================================================
#
# This script configures the local Git clone to use the hooks in
# .githooks/ instead of the default .git/hooks/ directory.
#
# Git does not version hooks by default: they live inside .git/, which
# is not committed. The workaround is to store the hooks in a tracked
# directory (.githooks/) and point Git at them via the core.hooksPath
# configuration.
#
# Usage:
#
#   ./scripts/install-hooks.sh
#
# Or, from anywhere in the repository:
#
#   task hooks:install
#
# The script is idempotent: running it twice is safe.
#
# To uninstall:
#
#   git config --unset core.hooksPath
# =============================================================================

set -eu

# ─────────────────────────────────────────────────────────────────────────────
# Setup
# ─────────────────────────────────────────────────────────────────────────────

# Find the repository root. The script may be invoked from any
# subdirectory, so we locate the root via git.
repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

hooks_dir=".githooks"

# ─────────────────────────────────────────────────────────────────────────────
# Preconditions
# ─────────────────────────────────────────────────────────────────────────────

if [ ! -d "$hooks_dir" ]; then
    echo "error: $hooks_dir directory not found" >&2
    echo "       run this script from the repository root" >&2
    exit 1
fi

# ─────────────────────────────────────────────────────────────────────────────
# Make hooks executable
# ─────────────────────────────────────────────────────────────────────────────
#
# Git refuses to run hooks that are not executable. The chmod is
# necessary because the executable bit is not always preserved when
# files are checked out (on Windows, in particular).
#
# The lib/ directory contains sourced files; they do not need the
# executable bit.
#
for hook in pre-commit pre-push commit-msg; do
    if [ -f "$hooks_dir/$hook" ]; then
        chmod +x "$hooks_dir/$hook"
    fi
done

# ─────────────────────────────────────────────────────────────────────────────
# Configure Git
# ─────────────────────────────────────────────────────────────────────────────

current=$(git config --get core.hooksPath 2>/dev/null || echo "")

if [ "$current" = "$hooks_dir" ]; then
    echo "hooks already installed: core.hooksPath = $hooks_dir"
    exit 0
fi

git config core.hooksPath "$hooks_dir"

echo "hooks installed: core.hooksPath = $hooks_dir"
echo ""
echo "The following hooks are now active:"
echo "  pre-commit  — fast checks before each commit"
echo "  pre-push    — slow checks before each push"
echo "  commit-msg  — commit message format validation"
echo ""
echo "To bypass a hook for a single operation:"
echo "  git commit --no-verify"
echo "  git push --no-verify"
echo ""
echo "To uninstall:"
echo "  git config --unset core.hooksPath"