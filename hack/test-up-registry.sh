#!/usr/bin/env bash
# test-up-registry.sh: Unit tests for deploy/kind/up.sh's registry bootstrap
# (F-234) and kubectl context pinning (F-232).
#
# Sources up.sh with GAMEPLANE_UP_SH_SOURCE_ONLY=1 so its function
# definitions (ensure_registry, the kubectl() wrapper) load without running
# the rest of the bootstrap or requiring kind/kubectl/helm/docker to be
# installed. `docker` itself is stubbed per test case so each of
# ensure_registry's three states (absent / stopped / running) can be
# exercised without touching a real Docker daemon. Each test runs in its own
# subshell so stubs and sourced state never leak between cases; each prints
# its own PASS/FAIL line, and the parent counts them.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
UP_SH="$REPO_ROOT/deploy/kind/up.sh"

check() {
    local desc="$1" got="$2" want="$3"
    if [[ "$got" == "$want" ]]; then
        echo "PASS: $desc"
    else
        echo "FAIL: $desc (want '$want', got '$got')"
    fi
}

# --- F-232: kubectl() must pin --context kind-<cluster> ---
test_kubectl_context() (
    set -euo pipefail
    # shellcheck disable=SC1090
    GAMEPLANE_UP_SH_SOURCE_ONLY=1 source "$UP_SH" some-other-cluster

    command() {
        if [[ "$1" == "kubectl" ]]; then
            shift
            echo "kubectl-called-with: $*"
            return 0
        fi
        builtin command "$@"
    }

    out="$(kubectl get pods 2>&1)"
    check "kubectl() pins --context kind-<cluster> (F-232)" \
        "$out" "kubectl-called-with: --context kind-some-other-cluster get pods"
)

# --- F-234: ensure_registry must handle absent / stopped / running ---
test_ensure_registry_absent() (
    set -euo pipefail
    # shellcheck disable=SC1090
    GAMEPLANE_UP_SH_SOURCE_ONLY=1 source "$UP_SH" gameplane-dev

    ran="none"
    docker() {
        case "$1" in
        inspect) return 1 ;; # no such container
        run)
            ran="run"
            return 0
            ;;
        start)
            ran="start"
            return 0
            ;;
        *) return 0 ;;
        esac
    }

    ensure_registry >/dev/null
    check "ensure_registry creates a new container when absent (F-234)" "$ran" "run"
)

test_ensure_registry_stopped() (
    set -euo pipefail
    # shellcheck disable=SC1090
    GAMEPLANE_UP_SH_SOURCE_ONLY=1 source "$UP_SH" gameplane-dev

    ran="none"
    docker() {
        case "$1" in
        inspect)
            echo "false" # container exists, State.Running=false
            return 0
            ;;
        run)
            ran="run"
            return 0
            ;;
        start)
            ran="start"
            return 0
            ;;
        *) return 0 ;;
        esac
    }

    ensure_registry >/dev/null
    check "ensure_registry restarts (docker start), not docker run, when stopped (F-234)" "$ran" "start"
)

test_ensure_registry_running() (
    set -euo pipefail
    # shellcheck disable=SC1090
    GAMEPLANE_UP_SH_SOURCE_ONLY=1 source "$UP_SH" gameplane-dev

    ran="none"
    docker() {
        case "$1" in
        inspect)
            echo "true"
            return 0
            ;;
        run)
            ran="run"
            return 0
            ;;
        start)
            ran="start"
            return 0
            ;;
        *) return 0 ;;
        esac
    }

    ensure_registry >/dev/null
    check "ensure_registry does nothing when already running (F-234)" "$ran" "none"
)

out="$(
    test_kubectl_context
    test_ensure_registry_absent
    test_ensure_registry_stopped
    test_ensure_registry_running
)"

echo "$out"

if echo "$out" | grep -q '^FAIL:'; then
    exit 1
fi
