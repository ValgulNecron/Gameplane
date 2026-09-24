#!/usr/bin/env bash
# test-check-doc-versions.sh: Fixture tests for the doc-version checker (spec 018 OD-012).
#
# Runs pass/ and fail/ test fixtures against hack/check-doc-versions.sh,
# verifying that the checker correctly identifies stale versions and accepts
# allowlisted ones. Each pass/* file must exit 0; each fail/* file must exit
# non-zero with output matching the corresponding .expected file.
#
# See: specs/018-v0-3-release-readiness/OPEN-DECISIONS.md (OD-012)

set -euo pipefail

# Navigate to repo root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$REPO_ROOT"

# Test fixture directory
FIX="hack/testdata/check-doc-versions"

# Export overrides for the doc-version checker
export DOC_VERSIONS_CHART="$FIX/Chart.yaml"

passed=0
failed=0

# Test pass/* fixtures
for f in "$FIX"/pass/*.md; do
	[[ ! -f "$f" ]] && continue

	basename_f="$(basename "$f")"

	if out=$(DOC_VERSIONS_FILES="$FIX/pass/$basename_f" hack/check-doc-versions.sh 2>&1); then
		echo "✓ pass: $basename_f"
		passed=$((passed+1))
	else
		echo "✗ expected pass: $basename_f"
		echo "$out"
		failed=$((failed+1))
	fi
done

# Test fail/* fixtures
for f in "$FIX"/fail/*.md; do
	[[ ! -f "$f" ]] && continue

	expected_file="${f%.md}.expected"
	basename_f="$(basename "$f")"

	out=""
	# Use || true to allow the checker to exit non-zero without failing the test script
	out=$(DOC_VERSIONS_FILES="$FIX/fail/$basename_f" hack/check-doc-versions.sh 2>&1 || true)

	# Extract stale version strings from the output
	got=$(printf '%s\n' "$out" | sed -n "s|^✗ $FIX/fail/$basename_f:\([0-9]*\): \([^ ]*\) (current appVersion is .*)$|\1: \2|p")

	expected=$(cat "$expected_file")

	if [[ "$got" == "$expected" ]]; then
		echo "✓ fail: $basename_f"
		passed=$((passed+1))
	else
		echo "✗ $basename_f output mismatch"
		echo "Expected:"
		echo "$expected"
		echo "Got:"
		echo "$got"
		failed=$((failed+1))
	fi
done

# Report results
total=$((passed + failed))
if [[ $failed -eq 0 ]]; then
	echo "✓ $total fixtures behaved as expected"
	exit 0
else
	echo "✗ $failed fixture(s) misbehaved"
	exit 1
fi
