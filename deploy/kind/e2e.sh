#!/usr/bin/env bash
# Ephemeral kind cluster for end-to-end tests.
#
# Usage:
#   deploy/kind/e2e.sh up    [cluster-name] [tag]
#   deploy/kind/e2e.sh down  [cluster-name]
#
# Diffs from dev-up (deploy/kind/up.sh):
#   - Single-node cluster (faster boot, sufficient for E2E coverage).
#   - Skips ingress-nginx (the dashboard isn't exercised here).
#   - Loads pre-built gameplane/{operator,api,agent}:<tag> images.
#   - Helm install with --wait so pods are Ready before tests start.
#
# Image tag defaults to "e2e". Override via the second argument or
# the GAMEPLANE_E2E_TAG env var.

set -euo pipefail

ACTION="${1:-up}"
CLUSTER="${2:-gameplane-e2e}"
TAG="${3:-${GAMEPLANE_E2E_TAG:-e2e}}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "${HERE}/../.." && pwd)"
CHART_DIR="${REPO}/charts/gameplane"

need() { command -v "$1" >/dev/null 2>&1 || { echo "missing: $1" >&2; exit 1; }; }

case "${ACTION}" in
up)
    need kind
    need kubectl
    need helm
    need docker

    # Start from a clean cluster every time. The self-hosted CI runner is
    # persistent, so a cancelled run can leave a zombie cluster behind: kind
    # still lists it, but its kubeconfig context is gone (kubectl then falls
    # back to localhost:8080 and everything fails) and a leftover Helm release
    # would break the reinstall. Delete any same-named cluster before creating.
    # Local fast-iteration reuses the cluster via `make test-e2e-keep`, which
    # does not call this `up` path.
    if kind get clusters | grep -qx "${CLUSTER}"; then
        echo "removing pre-existing cluster ${CLUSTER} for a clean slate"
        kind delete cluster --name "${CLUSTER}" || true
    fi
    echo "creating kind cluster ${CLUSTER} (single-node)"
    kind create cluster --name "${CLUSTER}" --wait 90s --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
EOF

    kubectl cluster-info --context "kind-${CLUSTER}" >/dev/null

    echo "loading gameplane/{operator,api,agent}:${TAG} images into kind"
    for img in operator api agent; do
        if ! docker image inspect "gameplane-test/${img}:${TAG}" >/dev/null 2>&1; then
            echo "  missing local image gameplane-test/${img}:${TAG} — building"
            docker build -t "gameplane-test/${img}:${TAG}" -f "${REPO}/${img}/Dockerfile" "${REPO}"
        fi
        kind load docker-image "gameplane-test/${img}:${TAG}" --name "${CLUSTER}"
    done

    echo "helm upgrade --install gameplane"
    # Bump the API container's memory limit above the chart default of
    # 256Mi. The bootstrap-admin subcommand and every login endpoint
    # invocation runs argon2id, which uses ~64Mi of working memory per
    # hash; the default limit OOM-kills the container under e2e's
    # frequent BootstrapAdmin/APIClient calls. The suite runs with
    # t.Parallel(), so several logins (64Mi each) can hash at once —
    # 1Gi keeps ~4 concurrent hashes plus baseline comfortably clear.
    # Disable the default upstream ModuleSource: it points at a public OCI
    # registry the e2e cluster can't reach, so `--wait` would block on it
    # (kstatus reports the never-indexed source as InProgress) until timeout.
    # The module e2e tests provision their own in-cluster registry + source.
    helm upgrade --install gameplane "${CHART_DIR}" \
        --namespace gameplane-system --create-namespace \
        --set "image.registry=gameplane-test" \
        --set "image.tag=${TAG}" \
        --set "ingress.enabled=false" \
        --set "operator.agentImage=gameplane-test/agent:${TAG}" \
        --set "api.resources.limits.memory=1Gi" \
        --set "operator.leaderElect=false" \
        --set "defaultModuleSource.enabled=false" \
        --wait --timeout 5m

    echo
    echo "✓ cluster ${CLUSTER} ready (image tag ${TAG})"
    echo "  GAMEPLANE_E2E_REUSE_CLUSTER=1 reuses this cluster across go test runs"
    ;;

down)
    if ! kind get clusters | grep -qx "${CLUSTER}"; then
        echo "cluster ${CLUSTER} not found — nothing to do"
        exit 0
    fi
    kind delete cluster --name "${CLUSTER}"
    ;;

*)
    echo "usage: $0 up|down [cluster-name] [tag]" >&2
    exit 2
    ;;
esac
