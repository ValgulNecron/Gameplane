#!/usr/bin/env bash
# test-check-ci-report-coverage.sh: Fixture tests for hack/check-ci-report-coverage.sh (F-238).
#
# Runs pass.yaml (needs: fully covered by NEEDS_ORDER + JOB_MATCHERS) and the
# fail-*.yaml fixtures (a needs: job missing from one or both structures)
# through the checker, and against the real .github/workflows/ci.yaml.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$REPO_ROOT"

FIX="hack/testdata/check-ci-report-coverage"
passed=0
failed=0

check() {
	local desc="$1" workflow="$2" want_status="$3"
	local status=0
	CI_REPORT_WORKFLOW="$workflow" hack/check-ci-report-coverage.sh >/tmp/check-ci-report-coverage.out 2>&1 || status=$?
	if [[ "$status" -eq "$want_status" ]]; then
		echo "✓ $desc"
		passed=$((passed + 1))
	else
		echo "✗ $desc (expected exit $want_status, got $status)"
		cat /tmp/check-ci-report-coverage.out
		failed=$((failed + 1))
	fi
}

check "pass fixture accepted" "$FIX/pass.yaml" 0
check "fixture missing from NEEDS_ORDER and JOB_MATCHERS rejected" "$FIX/fail-missing-both.yaml" 1
check "fixture missing only from JOB_MATCHERS rejected" "$FIX/fail-missing-matcher.yaml" 1
check "real ci.yaml is currently covered" ".github/workflows/ci.yaml" 0

total=$((passed + failed))
if [[ $failed -eq 0 ]]; then
	echo "✓ $total fixtures behaved as expected"
	exit 0
else
	echo "✗ $failed fixture(s) misbehaved"
	exit 1
fi
