#!/usr/bin/env bash
# Verifier for game protocol E2E coverage: validates consistency between
# modules/, test/e2e/buckets.sh, and docs/game-coverage.md.
#
# Usage:
#   joincoverage.sh [verify]     run all consistency checks (default)
#   joincoverage.sh diagnose <module>  (future; not required Phase 1)
#
# Exit codes:
#   0: all checks passed
#   1: hard check failed
#   2: warning only (staleness); checks passed but coverage is old
#
set -euo pipefail

# Hard failure; exit 1 immediately.
fail() {
	printf '%s\n' "ERROR: $*" >&2
	exit 1
}

# Warning (does not fail, but is printed).
warn() {
	printf '%s\n' "WARNING: $*"
}

# Determine repo root. This script lives at test/e2e/, so the root is TWO
# levels up, not one — a single ".." lands in test/ and every relative path
# then silently misses.
# Can be overridden with --root <dir> for testing against fixture trees.
REPO_ROOT="$(dirname "$0")/../.."
ROOT_EXPLICIT=0

# --root may appear before or after the subcommand: callers and the fixture
# harness use "verify --root <dir>", CI uses no flag at all.
SUBCOMMAND=""
while [ $# -gt 0 ]; do
	case "$1" in
	--root)
		if [ $# -lt 2 ]; then
			fail "--root requires an argument"
		fi
		REPO_ROOT="$2"
		ROOT_EXPLICIT=1
		shift 2
		;;
	*)
		SUBCOMMAND="$1"
		shift
		;;
	esac
done

# Change to repo root so all relative paths work correctly.
cd "$REPO_ROOT"

# When the root was DERIVED from $0, assert it looks like the Gameplane tree.
# Without this, a wrong derivation surfaces as a confusing "modules/ not found"
# instead of naming the real problem. Skipped for an explicit --root, since
# fixture trees are deliberately minimal (docs/ only, no test/e2e/).
if [ "$ROOT_EXPLICIT" -eq 0 ] && { [ ! -d test/e2e ] || [ ! -d docs ]; }; then
	fail "derived repo root does not look like the Gameplane tree (cwd: $(pwd)); expected test/e2e/ and docs/ to exist"
fi

# --- Artifact keyword list for Check 13 ---
# Heuristic guard against empty or hand-wavy Blocker cells. This is NOT a semantic
# check — it simply verifies that a blocked-doc Blocker text mentions at least one
# concrete artifact type. Extended when a genuinely new kind of blocking artifact
# appears in the docs. Current coverage verified against docs/game-coverage.md:
# - "undocumented" / "not documented" (7+ modules)
# - "field offset" / "field offsets" / "field map" (garrys-mod)
# - "wire format" (factorio, others)
# - "reverse-engineer" / "reverse-engineering" (ark, palworld)
# - "documentation" / "documented" (cs2)
# - "packet capture" / "specification" / "protocol spec" (generic)
artifact_keywords() {
	cat <<'EOF'
packet capture
field offset
field map
reverse-engineer
reverse engineering
vendor documentation
protocol spec
documentation
documented
specification
wire format
undocumented
not documented
EOF
}

# --- Initialization & Parsing ---

# Check if modules/ is initialized (at least one real directory).
check_modules_initialized() {
	# Deliberately avoids a `find ... | head` pipeline: under `set -o pipefail`
	# head closing the pipe early makes find exit non-zero, which has made this
	# check misreport a populated modules/ as empty. A glob test cannot do that.
	if [ ! -d modules ]; then
		fail "modules/ not found (cwd: $(pwd); run: git submodule update --init)"
	fi
	set -- modules/*/
	if [ ! -d "$1" ]; then
		fail "modules/ exists but contains no module directories (cwd: $(pwd); submodule not checked out: git submodule update --init)"
	fi
}

# Get list of module directories from modules/.
# Returns: sorted list of module names, one per line.
get_module_dirs() {
	find modules -maxdepth 1 -mindepth 1 -type d -not -name '.git' -not -name '.gitmodules' |
		sed 's|^modules/||' |
		sort
}

# Parse the coverage table from docs/game-coverage.md (first table only).
# Outputs: parsed table rows as pipe-separated values.
# Each line: module|game|status|depth|test|bucket|last_verified|blocker|blocker_class
parse_coverage_table() {
	local in_table=0
	local header_seen=0

	while IFS= read -r line; do
		# Skip until we see the first table separator (|---|...)
		if [ "$in_table" -eq 0 ] && [[ "$line" =~ ^[[:space:]]*\|[[:space:]]*-+[[:space:]]*\| ]]; then
			in_table=1
			continue
		fi

		# Once in table, process data rows (not separators).
		if [ "$in_table" -eq 1 ]; then
			# Separator row: skip (already seen header).
			if [[ "$line" =~ ^[[:space:]]*\|[[:space:]]*-+[[:space:]]*\| ]]; then
				continue
			fi

			# Empty line or end of first table: stop parsing.
			if [ -z "$(echo "$line" | tr -d ' \t')" ]; then
				break
			fi

			# Data row: parse and output (trimmed cells).
			if [[ "$line" =~ ^[[:space:]]*\| ]]; then
				# Remove leading/trailing whitespace and pipes.
				line=$(echo "$line" | sed 's/^[[:space:]]*|//' | sed 's/|[[:space:]]*$//')
				# Split on pipes, trim each cell.
				printf '%s\n' "$line" | awk -F'|' '{
					for (i=1; i<=NF; i++) {
						gsub(/^[[:space:]]+|[[:space:]]+$/, "", $i)
						printf "%s", $i
						if (i < NF) printf "|"
					}
					print ""
				}'
			fi
		fi
	done <docs/game-coverage.md
}

# Extract a specific column from a parsed row.
# Args: row_number (1-9), data_line (pipe-separated)
# Returns: cell value
get_column() {
	local col=$1
	local data=$2
	echo "$data" | awk -F'|' "{print \$$col}"
}

# --- Core Checks ---

# Check 1: Modules submodule must be initialized.
check_1_modules_initialized() {
	check_modules_initialized
}

# Check 2: Every module in modules/ must be listed in docs/game-coverage.md.
check_2_every_module_listed() {
	local modules=$(get_module_dirs)
	local coverage_modules=$(parse_coverage_table | awk -F'|' '{print $1}' | sed 's/^`//;s/`$//')

	while IFS= read -r mod; do
		if ! printf '%s\n' "$coverage_modules" | grep -qx "$mod"; then
			fail "module '$mod' found in modules/ but not listed in docs/game-coverage.md"
		fi
	done <<<"$modules"
}

# Check 3: No duplicate modules in coverage table.
check_3_no_duplicates() {
	local coverage=$(parse_coverage_table)
	local modules=$(echo "$coverage" | awk -F'|' '{print $1}' | sed 's/^`//;s/`$//')
	local dups=$(printf '%s\n' "$modules" | sort | uniq -d)

	if [ -n "$dups" ]; then
		while IFS= read -r dup; do
			# Find row numbers for this module.
			local rows=$(printf '%s\n' "$coverage" | awk -F'|' -v m="\`$dup\`" '$1 == m {print NR}')
			fail "module '$dup' listed twice in docs/game-coverage.md (rows: $rows)"
		done <<<"$dups"
	fi
}

# Check 4: No stray modules in coverage table (listed but not in modules/).
check_4_no_stray_modules() {
	local module_dirs=$(get_module_dirs)
	local coverage=$(parse_coverage_table)

	while IFS= read -r row; do
		local module=$(get_column 1 "$row" | sed 's/^`//;s/`$//')
		if [ -n "$module" ]; then
			if ! printf '%s\n' "$module_dirs" | grep -qx "$module"; then
				fail "docs/game-coverage.md lists module '$module' which does not exist in modules/ (deleted or renamed?)"
			fi
		fi
	done <<<"$coverage"
}

# Check 5: Status and Depth must be consistent.
check_5_status_depth_consistent() {
	local coverage=$(parse_coverage_table)

	while IFS= read -r row; do
		local module=$(get_column 1 "$row" | sed 's/^`//;s/`$//')
		local status=$(get_column 3 "$row")
		local depth=$(get_column 4 "$row")

		if [ -z "$module" ] || [ -z "$status" ]; then
			continue
		fi

		# covered-in-ci and covered-deferred MUST have JOINED depth.
		if [[ "$status" =~ ^covered- ]]; then
			if [ "$depth" != "JOINED" ]; then
				fail "module '$module' has Status '$status' but Depth '$depth' (only JOINED depth counts as coverage, per spec FR-006)"
			fi
		fi
	done <<<"$coverage"
}

# Check 6: Covered modules must have a test.
check_6_covered_has_test() {
	local coverage=$(parse_coverage_table)

	while IFS= read -r row; do
		local module=$(get_column 1 "$row" | sed 's/^`//;s/`$//')
		local status=$(get_column 3 "$row")
		local test=$(get_column 5 "$row")

		if [[ "$status" =~ ^covered- ]]; then
			if [ "$test" = "—" ] || [ -z "$test" ]; then
				fail "module '$module' has Status '$status' but Test is empty (no test listed)"
			fi
		fi
	done <<<"$coverage"
}

# Check 7: Test name must be valid (found in test/e2e/*_test.go).
check_7_test_names_valid() {
	local coverage=$(parse_coverage_table)

	while IFS= read -r row; do
		local module=$(get_column 1 "$row" | sed 's/^`//;s/`$//')
		local test=$(get_column 5 "$row")

		if [ -n "$test" ] && [ "$test" != "—" ]; then
			if ! grep -q "^func $test" test/e2e/*_test.go 2>/dev/null; then
				fail "module '$module' lists test '$test' which was not found in test/e2e/*_test.go (typo or renamed?)"
			fi
		fi
	done <<<"$coverage"
}

# Check 8: Covered modules must have a bucket.
check_8_covered_has_bucket() {
	local coverage=$(parse_coverage_table)

	while IFS= read -r row; do
		local module=$(get_column 1 "$row" | sed 's/^`//;s/`$//')
		local status=$(get_column 3 "$row")
		local bucket=$(get_column 6 "$row")

		if [[ "$status" =~ ^covered- ]]; then
			if [ "$bucket" = "—" ] || [ -z "$bucket" ]; then
				fail "module '$module' has Status '$status' but Bucket is empty (must be one of: operator, api-auth, api-roles, api-rbac, api-agent, api-mods, ratelimit, bot-fast, bot-heavy, multicluster, upgrade, or unbucketed() with comment)"
			fi
		fi
	done <<<"$coverage"
}

# Check 9: Bucket name must be valid (or unbucketed()).
check_9_bucket_names_valid() {
	local valid_buckets="operator api-auth api-roles api-rbac api-agent api-mods ratelimit bot-fast bot-heavy multicluster upgrade"
	local coverage=$(parse_coverage_table)

	while IFS= read -r row; do
		local module=$(get_column 1 "$row" | sed 's/^`//;s/`$//')
		local bucket=$(get_column 6 "$row")

		if [ -n "$bucket" ] && [ "$bucket" != "—" ]; then
			# Check if it's valid bucket or unbucketed().
			if ! printf '%s\n' "$valid_buckets" | grep -qx "$bucket"; then
				if [[ ! "$bucket" =~ ^unbucketed\(\) ]]; then
					fail "module '$module' has Bucket '$bucket' which is not recognized (valid: operator, api-auth, api-roles, api-rbac, api-agent, api-mods, ratelimit, bot-fast, bot-heavy, multicluster, upgrade, or unbucketed() with comment)"
				fi
			fi
		fi
	done <<<"$coverage"
}

# Check 10: Covered-in-CI cannot use bot-heavy bucket.
check_10_no_ci_in_bot_heavy() {
	local coverage=$(parse_coverage_table)

	while IFS= read -r row; do
		local module=$(get_column 1 "$row" | sed 's/^`//;s/`$//')
		local status=$(get_column 3 "$row")
		local bucket=$(get_column 6 "$row")

		if [ "$status" = "covered-in-ci" ] && [ "$bucket" = "bot-heavy" ]; then
			fail "module '$module' has Status 'covered-in-ci' but Bucket 'bot-heavy' (bot-heavy does not run in default CI, use bot-fast or another default-executed bucket)"
		fi
	done <<<"$coverage"
}

# Check 11: Covered modules must have Last Verified date.
check_11_covered_has_last_verified() {
	local coverage=$(parse_coverage_table)

	while IFS= read -r row; do
		local module=$(get_column 1 "$row" | sed 's/^`//;s/`$//')
		local status=$(get_column 3 "$row")
		local last_verified=$(get_column 7 "$row")

		if [[ "$status" =~ ^covered- ]]; then
			if [ "$last_verified" = "—" ] || [ -z "$last_verified" ]; then
				fail "module '$module' has Status '$status' but Last Verified is empty (must be YYYY-MM-DD)"
			fi
			# Validate date format (simple check).
			if ! [[ "$last_verified" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
				fail "module '$module' Last Verified '$last_verified' is not a valid YYYY-MM-DD date"
			fi
		fi
	done <<<"$coverage"
}

# Check 12: Blocked modules must have a blocker and blocker class.
check_12_blocked_has_blocker() {
	local coverage=$(parse_coverage_table)

	while IFS= read -r row; do
		local module=$(get_column 1 "$row" | sed 's/^`//;s/`$//')
		local status=$(get_column 3 "$row")
		local blocker=$(get_column 8 "$row")
		local blocker_class=$(get_column 9 "$row")

		if [[ "$status" =~ ^blocked- ]] || [ "$status" = "out-of-scope-by-design" ]; then
			if [ "$blocker" = "—" ] || [ -z "$blocker" ]; then
				fail "module '$module' has Status '$status' but Blocker is empty"
			fi
			# Check blocker class.
			if [ "$status" = "blocked-doc" ]; then
				if [ "$blocker_class" != "documentation" ]; then
					fail "module '$module' has Status 'blocked-doc' but Blocker Class '$blocker_class' (should be 'documentation')"
				fi
			fi
		fi
	done <<<"$coverage"
}

# Check 13: Blocked-doc must name a concrete unblocking artifact (keyword heuristic).
check_13_blocked_doc_artifact() {
	local keywords=$(artifact_keywords)
	local coverage=$(parse_coverage_table)

	while IFS= read -r row; do
		local module=$(get_column 1 "$row" | sed 's/^`//;s/`$//')
		local status=$(get_column 3 "$row")
		local blocker=$(get_column 8 "$row")

		if [ "$status" = "blocked-doc" ]; then
			local has_keyword=0
			while IFS= read -r keyword; do
				if printf '%s\n' "$blocker" | grep -iq "$keyword"; then
					has_keyword=1
					break
				fi
			done <<<"$keywords"

			if [ "$has_keyword" -eq 0 ]; then
				fail "module '$module' has Status 'blocked-doc' but Blocker does not name a concrete artifact (must mention: undocumented, not documented, field offset, wire format, reverse-engineer, packet capture, documentation, protocol spec, specification, or similar)"
			fi
		fi
	done <<<"$coverage"
}

# Check 14: Out-of-scope-by-design must have architectural blocker.
check_14_out_of_scope_architectural() {
	local coverage=$(parse_coverage_table)

	while IFS= read -r row; do
		local module=$(get_column 1 "$row" | sed 's/^`//;s/`$//')
		local status=$(get_column 3 "$row")
		local blocker_class=$(get_column 9 "$row")

		if [ "$status" = "out-of-scope-by-design" ]; then
			if [ "$blocker_class" != "architectural" ]; then
				fail "module '$module' has Status 'out-of-scope-by-design' but Blocker Class '$blocker_class' (should be 'architectural')"
			fi
		fi
	done <<<"$coverage"
}

# Check 15: Test and bucket cross-references must match (test must be in buckets.sh).
check_15_test_bucket_match() {
	local coverage=$(parse_coverage_table)

	while IFS= read -r row; do
		local module=$(get_column 1 "$row" | sed 's/^`//;s/`$//')
		local test=$(get_column 5 "$row")
		local bucket=$(get_column 6 "$row")

		if [ -n "$test" ] && [ "$test" != "—" ] && [ -n "$bucket" ] && [ "$bucket" != "—" ]; then
			if [[ ! "$bucket" =~ ^unbucketed\(\) ]]; then
				# Verify test is in this bucket's definition.
				if ! bash test/e2e/buckets.sh list "$bucket" 2>/dev/null | grep -qx "$test"; then
					fail "module '$module' lists test '$test' in Bucket '$bucket' but this test is not found in buckets.sh's '$bucket' definition"
				fi
			fi
		fi
	done <<<"$coverage"
}

# Check 16: Bot test names must agree with depth.
check_16_bot_test_depth_agreement() {
	local coverage=$(parse_coverage_table)

	while IFS= read -r row; do
		local module=$(get_column 1 "$row" | sed 's/^`//;s/`$//')
		local test=$(get_column 5 "$row")
		local depth=$(get_column 4 "$row")

		if [ -n "$test" ] && [ "$test" != "—" ]; then
			if [[ "$test" =~ _Joined$ ]]; then
				if [ "$depth" != "JOINED" ]; then
					fail "module '$module' lists test '$test' but Depth is '$depth' (test name ends in _Joined, must have JOINED depth)"
				fi
			elif [[ "$test" =~ _Query$ ]]; then
				if [ "$depth" != "QUERY" ]; then
					fail "module '$module' lists test '$test' but Depth is '$depth' (test name ends in _Query, must have QUERY depth)"
				fi
			fi
		fi
	done <<<"$coverage"
}

# --- Warning Checks ---

# W1: Last Verified dates must not be too old (covered-deferred only).
# Threshold: 90 days. Warning only (exit 0, not 1).
check_w1_staleness() {
	local coverage=$(parse_coverage_table)
	local now_epoch=$(date +%s)
	local threshold_days=90
	local threshold_sec=$((threshold_days * 86400))
	local has_warning=0

	while IFS= read -r row; do
		local module=$(get_column 1 "$row" | sed 's/^`//;s/`$//')
		local status=$(get_column 3 "$row")
		local last_verified=$(get_column 7 "$row")

		if [ "$status" = "covered-deferred" ] && [ -n "$last_verified" ] && [ "$last_verified" != "—" ]; then
			local verified_epoch=$(date -d "$last_verified" +%s 2>/dev/null || echo 0)
			if [ "$verified_epoch" -gt 0 ]; then
				local age_sec=$((now_epoch - verified_epoch))
				if [ "$age_sec" -gt "$threshold_sec" ]; then
					local age_days=$((age_sec / 86400))
					warn "module '$module' (Status 'covered-deferred') last verified $age_days days ago ($last_verified). Consider re-running this test on a real cluster to ensure the protocol is still correct."
					has_warning=1
				fi
			fi
		fi
	done <<<"$coverage"

	if [ "$has_warning" -eq 1 ]; then
		return 2  # Warning exit code.
	fi
	return 0
}

# --- Main Verification ---

verify() {
	local fail=0

	# Run all hard checks.
	check_1_modules_initialized || fail=1
	check_2_every_module_listed || fail=1
	check_3_no_duplicates || fail=1
	check_4_no_stray_modules || fail=1
	check_5_status_depth_consistent || fail=1
	check_6_covered_has_test || fail=1
	check_7_test_names_valid || fail=1
	check_8_covered_has_bucket || fail=1
	check_9_bucket_names_valid || fail=1
	check_10_no_ci_in_bot_heavy || fail=1
	check_11_covered_has_last_verified || fail=1
	check_12_blocked_has_blocker || fail=1
	check_13_blocked_doc_artifact || fail=1
	check_14_out_of_scope_architectural || fail=1
	check_15_test_bucket_match || fail=1
	check_16_bot_test_depth_agreement || fail=1

	if [ "$fail" -ne 0 ]; then
		exit 1
	fi

	# Check for staleness warnings.
	check_w1_staleness || {
		exit_code=$?
		if [ "$exit_code" -eq 2 ]; then
			exit 0  # Warnings don't fail.
		fi
	}

	exit 0
}

# --- Dispatch ---

case "${SUBCOMMAND:-verify}" in
verify)
	verify
	;;
diagnose)
	# Future subcommand (not required Phase 1).
	echo "diagnose not yet implemented" >&2
	exit 2
	;;
*)
	cat >&2 <<'EOF'
Usage:
  joincoverage.sh [verify]       Run all consistency checks (default)
  joincoverage.sh diagnose <mod> (future) Diagnose a single module

Exit codes:
  0: All checks passed
  1: Hard check failed
  2: Warning only (staleness)
EOF
	exit 2
	;;
esac
