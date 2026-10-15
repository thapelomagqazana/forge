#!/usr/bin/env bash
# verify-tier-timings.sh — report the median tier timings over the
# last N CI runs and flag any budget breach.
#
# This script is the mechanism that keeps the timing budgets honest.
# It is run quarterly by the Release Engineering owner, not by CI on
# every PR: the measurement is a review activity, not a gate.
#
# The script uses the GitHub CLI (`gh`) to fetch the last N successful
# runs of ci.yml, extracts each run's duration, and compares the
# median to the tier budget. A breach prints a report; it does not
# fail the script, because a breach is a finding to be triaged, not a
# build failure.
#
# Usage:
#   scripts/ci/verify-tier-timings.sh [--runs N] [--workflow NAME]
#
# Exit codes:
#   0 — the script ran and produced a report (regardless of breaches).
#   1 — the script could not run (gh not installed, no runs found).
set -euo pipefail

runs=20
workflow="ci.yml"

while [ $# -gt 0 ]; do
  case "$1" in
    --runs) runs="$2"; shift 2 ;;
    --workflow) workflow="$2"; shift 2 ;;
    *) echo "unknown flag: $1" >&2; exit 1 ;;
  esac
done

if ! command -v gh >/dev/null 2>&1; then
  echo "gh is not installed; install GitHub CLI to use this script" >&2
  exit 1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

# Fetch the last N successful runs. The jq filter extracts the run's
# started and completed timestamps and computes the duration in
# seconds. The output is one duration per line, sorted ascending.
durations="$(
  gh run list \
    --workflow "$workflow" \
    --status success \
    --limit "$runs" \
    --json createdAt,updatedAt \
    --jq '.[] | ((.updatedAt | fromdateiso8601) - (.createdAt | fromdateiso8601))' \
  | sort -n
)"

if [ -z "$durations" ]; then
  echo "no successful runs of $workflow found" >&2
  exit 1
fi

count=$(echo "$durations" | wc -l | tr -d ' ')
median_idx=$(( (count + 1) / 2 ))
median=$(echo "$durations" | sed -n "${median_idx}p")

echo "Workflow: $workflow"
echo "Runs:     $count"
echo "Median:   ${median}s"
echo

# The tier budgets, in seconds. These are duplicated from
# internal/ci/tiers.go by necessity: the script is not a Go program
# and cannot import the package. The duplication is policed by the
# review process: a change to a budget must update both this script
# and the Go model, and the ADR records the change.
declare -A budgets=(
  ["Tier 1"]=180
  ["Tier 2"]=900
  ["Tier 3"]=3600
  ["Tier 4"]=7200
)

echo "Tier budgets (from internal/ci/tiers.go):"
for tier in "Tier 1" "Tier 2" "Tier 3" "Tier 4"; do
  printf "  %-8s %6ds\n" "$tier" "${budgets[$tier]}"
done
echo

# The workflow file for each tier. A run of ci.yml covers Tiers 1-3;
# a run of security.yml or nightly.yml covers Tier 4.
case "$workflow" in
  ci.yml)
    if [ "$median" -le "${budgets["Tier 1"]}" ]; then
      echo "OK: median ${median}s is within the Tier 1 budget"
    elif [ "$median" -le "${budgets["Tier 2"]}" ]; then
      echo "FINDING: median ${median}s exceeds the Tier 1 budget of ${budgets["Tier 1"]}s"
    else
      echo "FINDING: median ${median}s exceeds the Tier 2 budget of ${budgets["Tier 2"]}s"
    fi
    ;;
  security.yml|nightly.yml)
    if [ "$median" -le "${budgets["Tier 4"]}" ]; then
      echo "OK: median ${median}s is within the Tier 4 budget"
    else
      echo "FINDING: median ${median}s exceeds the Tier 4 budget of ${budgets["Tier 4"]}s"
    fi
    ;;
esac