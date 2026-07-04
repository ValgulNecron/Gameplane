#!/usr/bin/env bash
# Single source of truth for the CI e2e test buckets.
#
# CI runs the Go e2e suite as several independent jobs, each executing one
# bucket via `go test -run "$(buckets.sh regex <name>)"`. Keeping the lists
# here (instead of inline in ci.yaml) lets `buckets.sh verify` prove the
# buckets are pairwise disjoint and exhaustive, so a new test can never be
# silently dropped from CI again.
#
# Usage:
#   buckets.sh buckets        print the bucket names
#   buckets.sh list <name>    print a bucket's test names, one per line
#   buckets.sh regex <name>   print the anchored ^(A|B|...)$ `go test -run` regex
#   buckets.sh verify         fail unless every test in test/e2e/*_test.go is
#                             in exactly one bucket (or explicitly unbucketed)
set -euo pipefail

cd "$(dirname "$0")"

# The regexes are anchored ^(...)$ — prefix matching (e.g. TestGameServer
# vs TestGameServer_Cascading) would silently run tests in more than one
# bucket.

bucket_core() { cat <<'EOF'
TestHelmInstall_AllPodsReady
TestHelmInstall_AllCRDsPresent
TestHelmInstall_APIHealthz
TestHelmInstall_APILogsClean
TestHelmInstall_OperatorLogsClean
TestCRD_Validation_GameServerWithoutTemplate
TestCRD_Validation_BackupScheduleBadCron
TestCRD_Validation_BackupRequiresServerRef
TestCRD_Validation_GameTemplateRequiresImage
TestGameServer_OperatorMaterializesChildren
TestGameServer_SuspendScalesToZeroAndBack
TestAPI_BootstrapAndLogin
TestAPI_LoginPrivacy
TestAPI_AuditEmitsOnMutation
TestAPI_RBAC_ViewerCannotMutate
TestAPI_RBAC_ViewerCannotMutate_Matrix
TestAPI_RBAC_OperatorCanWriteServers_NotUsers
TestAPI_RBAC_AdminCanReachAll
TestAPI_OperatorCannotInviteUsers
TestAPI_LifecycleStartStop
TestAPI_LifecycleRestart
TestAPI_LifecycleClone
TestAPI_LifecycleNotFound
TestAPI_AgentFilesRoundTrip
TestAPI_AgentPlayers
TestAPI_AgentUnreachable
EOF
}

bucket_extended() { cat <<'EOF'
TestGameServer_PVCSurvivesPodDelete
TestGameServer_CascadingDelete
TestGameTemplate_DeletionWithLiveServer
TestModule_FinalizerBlocksWhileTemplateInUse
TestModule_FailsOnUnreachableRegistry
TestModule_VerifyRejectsUnsignedBundle
TestModule_VerifySignedBundleInstalls
TestModuleSource_RejectsSSRFTarget
TestModuleSourceAndModule
TestBackup_OperatorMaterializesJob
TestBackup_FailsOnMissingPVC
TestBackup_FailsOnBadCredentials
TestBackupSchedule_CreatesBackupCR
TestBackupSchedule_SuspendStopsScheduling
TestBackupSchedule_RetentionTrimsPast
TestBackupSchedule_ConcurrencyForbid
TestRestore_RoundTrip
TestRestore_RejectsMissingBackup
TestRestore_FailsOnMissingSnapshot
TestAPI_PasswordResetInvalidatesSession
TestAPI_LoginRateLimit
TestAPI_AuditPaginationAndFilter
TestAPI_LogoutInvalidatesSession
TestAPI_ConsolePTYRoundTrip
TestAPI_LogsTailWS
EOF
}

bucket_bot() { cat <<'EOF'
TestGameServer_MinecraftBotConnects
EOF
}

# Tests that exist in the suite but deliberately run in NO bucket. Every
# entry needs a reason; `verify` fails on any unlisted stray so additions
# here are a conscious, reviewed act.
#
# These five predate the bucket script: the old inline ci.yaml regexes
# silently omitted them, so they have never run in CI. Tracked to be
# bucketed once they have a green run on record.
unbucketed() { cat <<'EOF'
TestGameServer_HeartbeatReachesRunning
TestModuleSourceUpload
TestAPI_CustomRole_Lifecycle
TestAPI_BuiltinRole_Immutable
TestAPI_PerNamespaceBinding_GrantsScopedAccess
EOF
}

bucket_names() {
	printf '%s\n' core extended bot
}

list_bucket() {
	case "$1" in
	core) bucket_core ;;
	extended) bucket_extended ;;
	bot) bucket_bot ;;
	*)
		echo "unknown bucket: $1 (known: $(bucket_names | tr '\n' ' '))" >&2
		exit 2
		;;
	esac
}

regex_bucket() {
	printf '^(%s)$\n' "$(list_bucket "$1" | paste -sd'|' -)"
}

suite_tests() {
	# Every e2e test lives flat in this directory behind the same
	# `//go:build e2e` tag, so a file grep is an exact inventory and
	# needs no Go toolchain. TestMain is the harness, not a test.
	grep -hoE '^func Test[A-Za-z0-9_]+' ./*_test.go |
		sed 's/^func //' |
		grep -vx 'TestMain' |
		sort
}

verify() {
	local suite union dups missing stray fail=0
	suite=$(suite_tests)
	union=$( (bucket_names | while read -r b; do list_bucket "$b"; done; unbucketed) | sort)

	dups=$(printf '%s\n' "$union" | uniq -d)
	if [ -n "$dups" ]; then
		echo "FAIL: listed in more than one bucket:" >&2
		printf '%s\n' "$dups" >&2
		fail=1
	fi

	missing=$(comm -23 <(printf '%s\n' "$suite") <(printf '%s\n' "$union" | uniq))
	if [ -n "$missing" ]; then
		echo "FAIL: in the suite but in no bucket (add to a bucket in $0):" >&2
		printf '%s\n' "$missing" >&2
		fail=1
	fi

	stray=$(comm -13 <(printf '%s\n' "$suite") <(printf '%s\n' "$union" | uniq))
	if [ -n "$stray" ]; then
		echo "FAIL: bucketed but not found in the suite (deleted or renamed test?):" >&2
		printf '%s\n' "$stray" >&2
		fail=1
	fi

	if [ "$fail" -ne 0 ]; then
		exit 1
	fi
	echo "buckets OK: $(printf '%s\n' "$suite" | grep -c .) tests, all in exactly one bucket"
}

case "${1:-}" in
buckets) bucket_names ;;
list) list_bucket "${2:?usage: buckets.sh list <bucket>}" ;;
regex) regex_bucket "${2:?usage: buckets.sh regex <bucket>}" ;;
verify) verify ;;
*)
	sed -n '2,15p' "$0" | sed 's/^# \{0,1\}//'
	exit 2
	;;
esac
