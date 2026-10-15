#!/usr/bin/env bash
#
# verify-release-lint.sh — lint the verify-release scripts.
#
# The script runs:
#
#   - shellcheck on scripts/verify-release.sh
#   - shfmt on scripts/verify-release.sh
#   - PSScriptAnalyzer on scripts/verify-release.ps1 (if pwsh is on
#     PATH)
#
# Each check is skipped when its tool is not on PATH. The script's
# purpose is a fast local lint; CI runs the same checks with the
# tools installed.
#
# Exit codes:
#   0  every available check passed
#   1  a check failed
#   2  usage error

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

status=0

if command -v shellcheck >/dev/null 2>&1; then
	echo "shellcheck scripts/verify-release.sh"
	if ! shellcheck scripts/verify-release.sh; then
		status=1
	fi
else
	echo "SKIP: shellcheck is not on PATH"
fi

if command -v shfmt >/dev/null 2>&1; then
	echo "shfmt -d scripts/verify-release.sh"
	if ! shfmt -d scripts/verify-release.sh; then
		status=1
	fi
else
	echo "SKIP: shfmt is not on PATH"
fi

if command -v pwsh >/dev/null 2>&1; then
	echo "PSScriptAnalyzer scripts/verify-release.ps1"
	if ! pwsh -NoProfile -Command '
		$ErrorActionPreference = "Stop"
		if (-not (Get-Module -ListAvailable -Name PSScriptAnalyzer)) {
			Write-Host "SKIP: PSScriptAnalyzer is not installed"
			exit 0
		}
		Import-Module PSScriptAnalyzer
		$results = Invoke-ScriptAnalyzer -Path scripts/verify-release.ps1
		if ($results) {
			$results | Format-Table -AutoSize
			exit 1
		}
	'; then
		status=1
	fi
else
	echo "SKIP: pwsh is not on PATH"
fi

exit "$status"