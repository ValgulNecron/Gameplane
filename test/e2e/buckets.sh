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
#
# The buckets are cut by LOGIN PRESSURE, not by feature area. The API's
# login rate limiter is per-IP (5/min, burst 10) plus per-username
# (3/min, burst 6), and every test in a job shares one client IP through
# the kubectl port-forward — so admin logins, not CPU, bound an API
# bucket's wall clock. `operator` does zero logins and runs wide; each
# api bucket stays within its own cluster's login budget (~7 admin
# logins ≈ the per-user burst, so retries absorb the overflow instead
# of exhausting it). `ratelimit` deliberately drains the shared limiter,
# so it runs as a separate, LAST `go test` invocation (Go runs
# non-parallel tests first — inside the main bucket it would starve
# everyone else's logins).

bucket_operator() { cat <<'EOF'
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
TestGameServer_HeartbeatReachesRunning
TestGameServer_PVCSurvivesPodDelete
TestGameServer_VersionSwitch
TestGameServer_CascadingDelete
TestGameServer_IngressNetworkPolicyShapeAndCascade
TestGameTemplate_DeletionWithLiveServer
TestModule_FinalizerBlocksWhileTemplateInUse
TestModule_FailsOnUnreachableRegistry
TestModule_VerifyRejectsUnsignedBundle
TestModule_VerifySignedBundleInstalls
TestModuleSource_RejectsSSRFTarget
TestModuleSourceAndModule
TestModuleSourceUpload
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
EOF
}

bucket_api_auth() { cat <<'EOF'
TestAPI_BootstrapAndLogin
TestAPI_LoginPrivacy
TestAPI_DynamicAuthProviders
TestAPI_AuditEmitsOnMutation
TestAPI_AuditPaginationAndFilter
TestAPI_LogoutInvalidatesSession
TestAPI_PasswordResetInvalidatesSession
EOF
}

bucket_api_roles() { cat <<'EOF'
TestAPI_CustomRole_Lifecycle
TestAPI_BuiltinRole_Immutable
TestAPI_PerNamespaceBinding_GrantsScopedAccess
TestAPI_OwnerCollaboratorAccess
EOF
}

bucket_api_rbac() { cat <<'EOF'
TestAPI_RBAC_ViewerCannotMutate
TestAPI_RBAC_ViewerCannotMutate_Matrix
TestAPI_RBAC_OperatorCanWriteServers_NotUsers
TestAPI_RBAC_AdminCanReachAll
TestAPI_OperatorCannotInviteUsers
TestAPI_LifecycleClone
TestAPI_LifecycleNotFound
EOF
}

bucket_api_agent() { cat <<'EOF'
TestAPI_AgentFilesRoundTrip
TestAPI_AgentPlayers
TestAPI_AgentUnreachable
TestAPI_ConsolePTYRoundTrip
TestAPI_LogsTailWS
TestAPI_LifecycleStartStop
TestAPI_LifecycleRestart
EOF
}

bucket_api_mods() { cat <<'EOF'
TestAPI_ModManifestInstallUpgrade
TestAPI_ModUpload
EOF
}

bucket_ratelimit() { cat <<'EOF'
TestAPI_LoginRateLimit
EOF
}

bucket_bot() { cat <<'EOF'
TestGameServer_MinecraftBotConnects
TestGameServer_TerrariaBotConnects
EOF
}

# Needs a SECOND kind cluster (dual-cluster ?cluster= dispatch + scoped
# RBAC), which none of the other buckets' single-cluster jobs provide — see
# multicluster_e2e_test.go's package doc. Its own dedicated CI job
# (e2e-multicluster) brings up both clusters before running it.
bucket_multicluster() { cat <<'EOF'
TestMultiCluster_ClusterDispatchAndScopedRBAC
EOF
}

# Tests that exist in the suite but deliberately run in NO bucket. Every
# entry needs a reason; `verify` fails on any unlisted stray so additions
# here are a conscious, reviewed act. Currently empty.
unbucketed() { :; }

bucket_names() {
	printf '%s\n' operator api-auth api-roles api-rbac api-agent api-mods ratelimit bot multicluster
}

list_bucket() {
	case "$1" in
	operator) bucket_operator ;;
	api-auth) bucket_api_auth ;;
	api-roles) bucket_api_roles ;;
	api-rbac) bucket_api_rbac ;;
	api-agent) bucket_api_agent ;;
	api-mods) bucket_api_mods ;;
	ratelimit) bucket_ratelimit ;;
	bot) bucket_bot ;;
	multicluster) bucket_multicluster ;;
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
