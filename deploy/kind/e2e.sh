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
#   - Loads pre-built kestrel/{operator,api,agent}:<tag> images.
#   - Helm install with --wait so pods are Ready before tests start.
#
# Image tag defaults to "e2e". Override via the second argument or
# the KESTREL_E2E_TAG env var.

set -euo pipefail

ACTION="${1:-up}"
CLUSTER="${2:-kestrel-e2e}"
TAG="${3:-${KESTREL_E2E_TAG:-e2e}}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "${HERE}/../.." && pwd)"
CHART_DIR="${REPO}/charts/kestrel"

need() { command -v "$1" >/dev/null 2>&1 || { echo "missing: $1" >&2; exit 1; }; }

case "${ACTION}" in
up)
    need kind
    need kubectl
    need helm
    need docker

    if kind get clusters | grep -qx "${CLUSTER}"; then
        echo "cluster ${CLUSTER} already exists — reusing"
    else
        echo "creating kind cluster ${CLUSTER} (single-node)"
        kind create cluster --name "${CLUSTER}" --wait 90s --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
EOF
    fi

    kubectl cluster-info --context "kind-${CLUSTER}" >/dev/null

    echo "loading kestrel/{operator,api,agent}:${TAG} images into kind"
    for img in operator api agent; do
        if ! docker image inspect "kestrel-test/${img}:${TAG}" >/dev/null 2>&1; then
            echo "  missing local image kestrel-test/${img}:${TAG} — building"
            docker build -t "kestrel-test/${img}:${TAG}" -f "${REPO}/${img}/Dockerfile" "${REPO}"
        fi
        kind load docker-image "kestrel-test/${img}:${TAG}" --name "${CLUSTER}"
    done

    echo "helm upgrade --install kestrel"
    # Bump the API container's memory limit above the chart default of
    # 256Mi. The bootstrap-admin subcommand and every login endpoint
    # invocation runs argon2id, which uses ~64Mi of working memory per
    # hash; the default limit OOM-kills the container under e2e's
    # frequent BootstrapAdmin/APIClient calls. 512Mi is comfortable.
    helm upgrade --install kestrel "${CHART_DIR}" \
        --namespace kestrel-system --create-namespace \
        --set "image.registry=kestrel-test" \
        --set "image.tag=${TAG}" \
        --set "ingress.enabled=false" \
        --set "operator.agentImage=kestrel-test/agent:${TAG}" \
        --set "api.resources.limits.memory=512Mi" \
        --wait --timeout 5m

    echo
    echo "✓ cluster ${CLUSTER} ready (image tag ${TAG})"
    echo "  KESTREL_E2E_REUSE_CLUSTER=1 reuses this cluster across go test runs"
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
