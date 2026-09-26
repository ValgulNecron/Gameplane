#!/usr/bin/env bash
# check-ci-report-coverage.sh: Verify that every job the `report` job in
# .github/workflows/ci.yaml depends on (its `needs:` list) is tallied by the
# report-rendering script's NEEDS_ORDER array and JOB_MATCHERS map.
#
# Purpose:
#   contracts/permissions-matrix.md (spec 008) requires that adding a job to
#   the report's `needs:` list also means adding it to NEEDS_ORDER and
#   JOB_MATCHERS: "Miss any one and the new job is silently absent from the
#   PR comment." There is no other mechanical check for this (F-238) — a job
#   can fail while the report's headline count and Failing table both say
#   everything passed.
#
# What this checks:
#   1. Extract the job ids listed in the `report:` job's `needs:` array.
#   2. Extract the string literals in the `const NEEDS_ORDER = [...]` array.
#   3. Extract the object keys in the `const JOB_MATCHERS = {...}` map.
#   4. Every id from (1) must appear in both (2) and (3).
#
# This is a static, line-based check of the YAML/JS source — it does not
# execute the workflow. It cannot see semantic bugs in a JOB_MATCHERS
# regex/predicate (only that a matcher exists for each id); it can only
# catch a job present in `needs:` but absent from the tally structures.
#
# Test override:
#   CI_REPORT_WORKFLOW: path to the workflow file to check
#   (default: .github/workflows/ci.yaml)
#
# Exit codes:
#   0 = every needs: job is present in both NEEDS_ORDER and JOB_MATCHERS
#   1 = at least one needs: job is missing from one or both, or the file
#       could not be parsed

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$REPO_ROOT"

workflow="${CI_REPORT_WORKFLOW:-.github/workflows/ci.yaml}"

if [[ ! -f "$workflow" ]] || [[ ! -r "$workflow" ]]; then
	echo "✗ Error: $workflow not found or unreadable" >&2
	exit 1
fi

# --- 1. job ids in the `report:` job's `needs:` list ------------------------
# The block looks like:
#   report:
#     name: ci report
#     needs:
#       [
#         changes,
#         build-images,
#         ...
#       ]
needs_block=$(awk '
  /^  report:$/ { in_report=1 }
  in_report && /needs:/ { in_needs=1; next }
  in_needs && /\]/ { exit }
  in_needs { print }
' "$workflow")

if [[ -z "$needs_block" ]]; then
	echo "✗ Error: could not find the report job'\''s needs: list in $workflow" >&2
	exit 1
fi

mapfile -t needs_ids < <(printf '%s\n' "$needs_block" | sed -E 's/[[:space:]]|,//g' | grep -vE '^$|^\[$|^\]$')

# --- 2. NEEDS_ORDER string literals -----------------------------------------
order_block=$(awk '
  /const NEEDS_ORDER = \[/ { in_order=1 }
  in_order { print }
  in_order && /\];/ { exit }
' "$workflow")

if [[ -z "$order_block" ]]; then
	echo "✗ Error: could not find NEEDS_ORDER in $workflow" >&2
	exit 1
fi

# --- 3. JOB_MATCHERS object -------------------------------------------------
matcher_block=$(awk '
  /const JOB_MATCHERS = \{/ { in_matchers=1 }
  in_matchers { print }
  in_matchers && /^ *\};$/ { exit }
' "$workflow")

if [[ -z "$matcher_block" ]]; then
	echo "✗ Error: could not find JOB_MATCHERS in $workflow" >&2
	exit 1
fi
matcher_keys=$(printf '%s\n' "$matcher_block" | grep -oE "^[[:space:]]*'?[A-Za-z0-9_-]+'?:" | sed -E "s/[[:space:]]|'|://g")

missing=0
for id in "${needs_ids[@]}"; do
	if ! grep -qF "'$id'" <<<"$order_block"; then
		echo "✗ missing from NEEDS_ORDER: $id" >&2
		missing=1
	fi
	if ! grep -qxF "$id" <<<"$matcher_keys"; then
		echo "✗ missing from JOB_MATCHERS: $id" >&2
		missing=1
	fi
done

if [[ "$missing" -ne 0 ]]; then
	echo "✗ FAIL: some jobs in the report job's needs: are not tallied by NEEDS_ORDER/JOB_MATCHERS" >&2
	echo "  see contracts/permissions-matrix.md: a new job needs the needs: list, NEEDS_ORDER, and JOB_MATCHERS" >&2
	exit 1
fi

echo "✓ every report needs: job (${#needs_ids[@]}) is present in NEEDS_ORDER and JOB_MATCHERS"
