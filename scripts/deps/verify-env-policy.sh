#!/usr/bin/env sh
# =============================================================================
# scripts/deps/verify-env-policy.sh — module resolution policy enforcement
# =============================================================================
#
# Verify that the running environment conforms to the module
# resolution policy documented in docs/dependency-policy.md,
# section "Module resolution policy".
#
# # What this script verifies
#
#   1. GOFLAGS is set and includes -mod=readonly.
#   2. GOTOOLCHAIN is set to "auto" or "local".
#   3. GOSUMDB is set to its default value (or is unset, which
#      defaults to sum.golang.org).
#   4. GONOSUMDB is empty.
#   5. GONOSUMCHECK is not set.
#   6. GO111MODULE is not set.
#   7. GOINSECURE is not set.
#   8. GOFLAGS does not include -mod=mod.
#   9. GOPROXY is either unset, or set to a value that includes
#      ",direct" as a fallback.
#  10. GOMODCACHE is either unset, or set to an absolute path.
#
# # What this script does NOT verify
#
#   - The value of GOPROXY beyond the ",direct" fallback. Any
#     proxy that supports the Go module protocol is acceptable.
#   - The contents of the module cache. That is the domain of
#     verify:resolution.
#   - Whether a vendor/ directory exists. That is checked by the
#     .gitignore rules and by the repository structure check.
#
# # Why a separate script
#
# The policy decision is documented in docs/dependency-policy.md.
# The script verifies that the running environment conforms to the
# decision. If a contributor's environment diverges from the
# policy — for example, because they have GONOSUMCHECK set from an
# old shell profile — the script identifies the divergence before
# it causes a reproducibility failure.
#
# # Exit codes
#
#   0  The environment conforms to the policy.
#   1  One or more violations were found.
#
# # Usage
#
#   sh scripts/deps/verify-env-policy.sh
#
#   # From the repository root:
#   task verify:deps:env-policy
#
# # POSIX compatibility
#
# The script is POSIX sh. It works on Linux, macOS, and Windows (via
# Git Bash or WSL) without requiring bash.
# =============================================================================

set -eu

# ─────────────────────────────────────────────────────────────────────
# Locate the repository root
# ─────────────────────────────────────────────────────────────────────

repo_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$repo_root"

# ─────────────────────────────────────────────────────────────────────
# Helpers
# ─────────────────────────────────────────────────────────────────────

# go_env returns the effective value of an environment variable as
# seen by the Go toolchain. It respects both shell exports and
# `go env -w` overrides.
go_env() {
    go env "$1" 2>/dev/null || echo ""
}

# fail records a policy violation.
fail_count=0

fail() {
    printf 'FAIL: %s\n' "$1" >&2
    if [ -n "${2:-}" ]; then
        printf '      %s\n' "$2" >&2
    fi
    if [ -n "${3:-}" ]; then
        printf '      %s\n' "$3" >&2
    fi
    fail_count=$((fail_count + 1))
}

ok() {
    printf 'OK: %s\n' "$1"
}

# ─────────────────────────────────────────────────────────────────────
# Check 1 — GOFLAGS
# ─────────────────────────────────────────────────────────────────────
#
# GOFLAGS must include -mod=readonly. This is the single most
# important environment setting for reproducibility. Without it,
# `go build` may silently mutate go.mod.
#
goflags=$(go_env GOFLAGS)

if [ -z "$goflags" ]; then
    fail "GOFLAGS is not set" \
         "The reproducibility invariant requires -mod=readonly." \
         "See docs/development.md for setup instructions."
elif ! echo "$goflags" | grep -q -- "-mod=readonly"; then
    fail "GOFLAGS does not include -mod=readonly" \
         "actual GOFLAGS: $goflags" \
         "See docs/development.md for setup instructions."
elif echo "$goflags" | grep -q -- "-mod=mod"; then
    fail "GOFLAGS includes -mod=mod (prohibited)" \
         "actual GOFLAGS: $goflags" \
         "-mod=mod allows silent mutation of go.mod."
else
    ok "GOFLAGS=$goflags"
fi

# ─────────────────────────────────────────────────────────────────────
# Check 2 — GOTOOLCHAIN
# ─────────────────────────────────────────────────────────────────────

gotoolchain=$(go_env GOTOOLCHAIN)

if [ "$gotoolchain" != "auto" ] && [ "$gotoolchain" != "local" ]; then
    if [ -z "$gotoolchain" ]; then
        fail "GOTOOLCHAIN is not set" \
             "Expected 'auto' or 'local'." \
             "See docs/development.md for setup instructions."
    else
        fail "GOTOOLCHAIN has an unexpected value" \
             "actual GOTOOLCHAIN: $gotoolchain" \
             "Expected 'auto' or 'local'."
    fi
else
    ok "GOTOOLCHAIN=$gotoolchain"
fi

# ─────────────────────────────────────────────────────────────────────
# Check 3 — GOSUMDB
# ─────────────────────────────────────────────────────────────────────
#
# GOSUMDB must be either unset (which defaults to sum.golang.org) or
# explicitly set to sum.golang.org. Any other value disables or
# redirects checksum verification.
#
gosumdb=$(go_env GOSUMDB)

if [ "$gosumdb" != "" ] && [ "$gosumdb" != "sum.golang.org" ]; then
    fail "GOSUMDB has a non-default value" \
         "actual GOSUMDB: $gosumdb" \
         "Only 'sum.golang.org' or empty is permitted."
else
    ok "GOSUMDB=${gosumdb:-sum.golang.org (default)}"
fi

# ─────────────────────────────────────────────────────────────────────
# Check 4 — GONOSUMDB
# ─────────────────────────────────────────────────────────────────────

gonosumdb=$(go_env GONOSUMDB)

if [ -n "$gonosumdb" ]; then
    fail "GONOSUMDB is not empty" \
         "actual GONOSUMDB: $gonosumdb" \
         "Forge has no private modules. Leave GONOSUMDB empty."
else
    ok "GONOSUMDB is empty"
fi

# ─────────────────────────────────────────────────────────────────────
# Check 5 — GONOSUMCHECK (prohibited)
# ─────────────────────────────────────────────────────────────────────
#
# GONOSUMCHECK is deprecated but still honoured by some Go versions.
# It disables checksum verification globally. It must never be set.
#
gonosumcheck=$(go_env GONOSUMCHECK)

if [ -n "$gonosumcheck" ]; then
    fail "GONOSUMCHECK is set (prohibited)" \
         "actual GONOSUMCHECK: $gonosumcheck" \
         "GONOSUMCHECK is deprecated and disables checksum verification."
else
    ok "GONOSUMCHECK is not set"
fi

# ─────────────────────────────────────────────────────────────────────
# Check 6 — GO111MODULE (prohibited)
# ─────────────────────────────────────────────────────────────────────
#
# GO111MODULE is deprecated. Go 1.16 and later always use modules.
# Setting it produces confusing diagnostics without changing
# behaviour. It must never be set.
#
go111module=$(go_env GO111MODULE)

if [ -n "$go111module" ]; then
    fail "GO111MODULE is set (deprecated)" \
         "actual GO111MODULE: $go111module" \
         "Unset it. Go 1.16 and later always use modules."
else
    ok "GO111MODULE is not set"
fi

# ─────────────────────────────────────────────────────────────────────
# Check 7 — GOINSECURE (prohibited)
# ─────────────────────────────────────────────────────────────────────
#
# GOINSECURE bypasses TLS for module downloads. It must never be
# set, even in restricted-network environments. If a private proxy
# requires this, the proxy is misconfigured.
#
goinsecure=$(go_env GOINSECURE)

if [ -n "$goinsecure" ]; then
    fail "GOINSECURE is set (prohibited)" \
         "actual GOINSECURE: $goinsecure" \
         "GOINSECURE bypasses TLS for module downloads."
else
    ok "GOINSECURE is not set"
fi

# ─────────────────────────────────────────────────────────────────────
# Check 8 — GOPROXY
# ─────────────────────────────────────────────────────────────────────
#
# GOPROXY may be the default, a private mirror, or a comma-separated
# list ending in ",direct". The only requirement is that if it is
# set to a custom value, the value must include ",direct" as a
# fallback. Without it, the toolchain cannot fetch a module if the
# proxy fails to serve it.
#
goproxy=$(go_env GOPROXY)

if [ -z "$goproxy" ]; then
    ok "GOPROXY= (default: https://proxy.golang.org,direct)"
elif [ "$goproxy" = "off" ]; then
    # GOPROXY=off is a valid value for offline mode, but it should
    # only be used in specific scenarios.
    fail "GOPROXY=off is not permitted in interactive environments" \
         "GOPROXY=off disables all module downloads." \
         "It is intended for offline CI, not for developers."
elif echo "$goproxy" | grep -q ",direct"; then
    ok "GOPROXY=$goproxy"
else
    fail "GOPROXY does not include ',direct' as a fallback" \
         "actual GOPROXY: $goproxy" \
         "Append ',direct' to allow direct git access as a fallback."
fi

# ─────────────────────────────────────────────────────────────────────
# Check 9 — GOMODCACHE
# ─────────────────────────────────────────────────────────────────────
#
# GOMODCACHE may be the default or an absolute path. A relative path
# would be interpreted relative to the current directory, which is
# rarely what the contributor intends.
#
gomodcache=$(go_env GOMODCACHE)

if [ -z "$gomodcache" ]; then
    ok "GOMODCACHE= (default: \$GOPATH/pkg/mod)"
else
    case "$gomodcache" in
        /*)
            ok "GOMODCACHE=$gomodcache"
            ;;
        *:*)
            # Windows absolute paths contain a drive letter followed
            # by a colon.
            ok "GOMODCACHE=$gomodcache"
            ;;
        *)
            fail "GOMODCACHE is not an absolute path" \
                 "actual GOMODCACHE: $gomodcache" \
                 "Use an absolute path. A relative path is interpreted" \
                 "relative to the current directory."
            ;;
    esac
fi

# ─────────────────────────────────────────────────────────────────────
# Check 10 — vendor/ directory
# ─────────────────────────────────────────────────────────────────────
#
# The module resolution policy forbids vendoring. A vendor/
# directory should not exist. If it does, it indicates either an
# accidental `go mod vendor` invocation or a deliberate deviation
# from the policy.
#
if [ -d vendor ]; then
    fail "vendor/ directory exists (prohibited)" \
         "The module resolution policy forbids vendoring." \
         "Remove it with: rm -rf vendor"
else
    ok "no vendor/ directory"
fi

# ─────────────────────────────────────────────────────────────────────
# Summary
# ─────────────────────────────────────────────────────────────────────

if [ "$fail_count" -gt 0 ]; then
    printf '\n' >&2
    printf 'FAIL: %d policy violation(s) found\n' "$fail_count" >&2
    printf '\n' >&2
    printf 'See docs/dependency-policy.md "Module resolution policy" for the\n' >&2
    printf 'full policy. See docs/env.example for the environment template.\n' >&2
    exit 1
fi

echo ""
echo "OK: environment conforms to the module resolution policy"
exit 0