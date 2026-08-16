#!/usr/bin/env bash
# Integration test for the joincoverage verifier.
#
# This script runs the verifier against all 16 fixtures and asserts:
# 1. Each fixture exits with code 1 (hard failure)
# 2. The error message matches the expected pattern for that check
# 3. The verifier exits 0 against the real repository root
#
# Exit codes:
#   0: All tests passed
#   1: One or more tests failed
#
# Usage:
#   test/e2e/joincoverage_test.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
FIXTURES_DIR="$SCRIPT_DIR/testdata/joincoverage"
VERIFIER="$SCRIPT_DIR/joincoverage.sh"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Helper to run a test
run_test() {
    local fixture="$1"
    local check_num="$2"
    local expected_pattern="$3"

    local fixture_path="$FIXTURES_DIR/$fixture"

    if [ ! -d "$fixture_path" ]; then
        echo -e "${RED}✗${NC} Fixture not found: $fixture"
        ((TESTS_FAILED++))
        return 1
    fi

    # Run the verifier against the fixture
    # Note: assumes the verifier supports --root <dir> override
    local output
    local exit_code=0
    output=$("$VERIFIER" verify --root "$fixture_path" 2>&1) || exit_code=$?

    # Check exit code is 1
    if [ "$exit_code" -ne 1 ]; then
        echo -e "${RED}✗${NC} $fixture (Check $check_num): Expected exit code 1, got $exit_code"
        echo "  Output: $output"
        ((TESTS_FAILED++))
        return 1
    fi

    # Check error message contains expected pattern
    if ! echo "$output" | grep -q "$expected_pattern"; then
        echo -e "${RED}✗${NC} $fixture (Check $check_num): Expected pattern not found"
        echo "  Expected pattern: $expected_pattern"
        echo "  Got output: $output"
        ((TESTS_FAILED++))
        return 1
    fi

    echo -e "${GREEN}✓${NC} $fixture (Check $check_num)"
    ((TESTS_PASSED++))
    return 0
}

# Main test suite
echo "Running joincoverage verifier integration tests..."
echo ""

# Check 1: Modules submodule must be initialized
# Matches on the remediation hint rather than the exact prose: check 1 reports
# two distinct conditions (modules/ absent vs present-but-empty) and both end
# with this instruction, so refining the wording will not re-break this test.
run_test "case-uninitialized-submodule" "1" \
    "git submodule update --init"

# Check 2: Every module in modules/ must be listed
run_test "case-missing-module" "2" \
    "found in modules/ but not listed"

# Check 3: No duplicate modules
run_test "case-duplicate-module" "3" \
    "listed twice in docs/game-coverage.md"

# Check 4: No stray modules
run_test "case-stray-module" "4" \
    "which does not exist in modules/"

# Check 5: Status and depth consistency
run_test "case-covered-with-query-depth" "5" \
    "has Status .covered-in-ci. but Depth .QUERY."

# Check 6: Covered modules must have a test
run_test "case-covered-without-test" "6" \
    "but Test is empty"

# Check 7: Test name must be valid
run_test "case-invalid-test-name" "7" \
    "which was not found in test/e2e/\*_test.go"

# Check 8: Covered modules must have a bucket
run_test "case-covered-without-bucket" "8" \
    "but Bucket is empty"

# Check 9: Bucket name must be valid
run_test "case-bad-bucket-name" "9" \
    "which is not recognized"

# Check 10: Covered-in-CI cannot use bot-heavy
run_test "case-covered-in-ci-in-bot-heavy" "10" \
    "bot-heavy does not run in default CI"

# Check 11: Covered modules must have last verified date
run_test "case-deferred-without-lastverified" "11" \
    "but Last Verified is empty"

# Check 12: Blocked modules must have a blocker
run_test "case-blocked-without-blocker" "12" \
    "but Blocker is empty"

# Check 13: Blocked-doc must name a concrete artifact
run_test "case-blocked-doc-without-artifact" "13" \
    "does not name a concrete artifact"

# Check 14: Out-of-scope must have architectural blocker
run_test "case-architectural-not-out-of-scope" "14" \
    "should be .architectural."

# Check 15: Test and bucket cross-references must match
run_test "case-test-bucket-mismatch" "15" \
    "but this test is not found in buckets.sh"

# Check 16: Bot test names must agree with depth
run_test "case-bot-test-depth-mismatch" "16" \
    "test name ends in _Joined, must have JOINED depth"

# Bonus: Verify the verifier passes against the real repo root
echo ""
echo "Verifying verifier passes against real repository root..."
if "$VERIFIER" verify 2>&1 >/dev/null; then
    echo -e "${GREEN}✓${NC} Verifier passes on real repo root"
    ((TESTS_PASSED++))
else
    echo -e "${RED}✗${NC} Verifier failed on real repo root (should pass)"
    ((TESTS_FAILED++))
fi

# Summary
echo ""
echo "====================================="
echo "Tests passed: $TESTS_PASSED"
echo "Tests failed: $TESTS_FAILED"
echo "====================================="

if [ "$TESTS_FAILED" -ne 0 ]; then
    exit 1
fi

exit 0
