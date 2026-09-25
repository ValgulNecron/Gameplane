#!/usr/bin/env bash
# test-check-publish-edge-paths.sh: Fixture tests for hack/check-publish-edge-paths.sh (F-239).
#
# pass/ has an on.push.paths list that covers every COPY source directory of
# its fixture Dockerfile; fail/ is the same Dockerfile with one of those
# directories missing from on.push.paths. Also runs the checker against the
# real .github/workflows/publish-edge.yaml.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$REPO_ROOT"

FIX="hack/testdata/check-publish-edge-paths"
passed=0
failed=0

check() {
	local desc="$1" workflow="$2" root="$3" want_status="$4"
	local status=0
	PUBLISH_EDGE_WORKFLOW="$workflow" PUBLISH_EDGE_DOCKERFILE_ROOT="$root" \
		hack/check-publish-edge-paths.sh >/tmp/check-publish-edge-paths.out 2>&1 || status=$?
	if [[ "$status" -eq "$want_status" ]]; then
		echo "✓ $desc"
		passed=$((passed + 1))
	else
		echo "✗ $desc (expected exit $want_status, got $status)"
		cat /tmp/check-publish-edge-paths.out
		failed=$((failed + 1))
	fi
}

check "pass fixture accepted" "$FIX/pass/workflow.yaml" "$FIX/pass/root" 0
check "fixture missing a COPY source directory rejected" "$FIX/fail/workflow.yaml" "$FIX/fail/root" 1
check "real publish-edge.yaml is currently covered" ".github/workflows/publish-edge.yaml" "." 0

total=$((passed + failed))
if [[ $failed -eq 0 ]]; then
	echo "✓ $total fixtures behaved as expected"
	exit 0
else
	echo "✗ $failed fixture(s) misbehaved"
	exit 1
fi
