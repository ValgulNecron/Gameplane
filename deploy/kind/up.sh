#!/usr/bin/env bash
# Bootstrap a local kind cluster for Kestrel development.
#
# Usage: deploy/kind/up.sh [cluster-name]
#
# Idempotent — if the cluster already exists the script exits 0 without
# modifying it. Invoked by `make dev-up`.

set -euo pipefail

CLUSTER="${1:-kestrel-dev}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG="${HERE}/cluster.yaml"

# Local OCI registry settings — published on the host as
# localhost:${REG_HOST_PORT} for `oras push`, and reachable from cluster
# pods as ${REG_NAME}:${REG_INTERNAL_PORT} after the kind-network attach.
REG_NAME="kind-registry"
REG_HOST_PORT="${KESTREL_REG_HOST_PORT:-5001}"
REG_INTERNAL_PORT=5000

need() { command -v "$1" >/dev/null 2>&1 || { echo "missing: $1" >&2; exit 1; }; }
need kind
need kubectl
need helm
need docker

# Bring up a registry container if there isn't one. Same recipe as
# https://kind.sigs.k8s.io/docs/user/local-registry/ — keeps the data
# inside Docker so it survives kind cluster recreates.
if [ "$(docker inspect -f '{{.State.Running}}' "${REG_NAME}" 2>/dev/null || true)" != "true" ]; then
    echo "starting local registry ${REG_NAME} on localhost:${REG_HOST_PORT}"
    docker run -d --restart=always \
        -p "127.0.0.1:${REG_HOST_PORT}:${REG_INTERNAL_PORT}" \
        --name "${REG_NAME}" registry:2 >/dev/null
fi

if kind get clusters | grep -qx "${CLUSTER}"; then
    echo "cluster ${CLUSTER} already exists — skipping create"
else
    echo "creating kind cluster ${CLUSTER}"
    kind create cluster --name "${CLUSTER}" --config "${CONFIG}"
fi

# Attach the registry to kind's network so pods can pull from
# kind-registry:5000.
if ! docker network inspect kind | grep -q "\"${REG_NAME}\""; then
    docker network connect kind "${REG_NAME}" || true
fi

# Tell each node's containerd that <REG_NAME>:<port> is a known
# registry, mirroring the upstream local-registry recipe. This makes
# `kubectl run --image=kind-registry:5000/...` work out of the box.
for node in $(kind get nodes --name "${CLUSTER}"); do
    docker exec "${node}" mkdir -p "/etc/containerd/certs.d/${REG_NAME}:${REG_INTERNAL_PORT}"
    cat <<EOF | docker exec -i "${node}" tee "/etc/containerd/certs.d/${REG_NAME}:${REG_INTERNAL_PORT}/hosts.toml" >/dev/null
[host."http://${REG_NAME}:${REG_INTERNAL_PORT}"]
EOF
done

# Document the registry inside the cluster (matches the upstream KEP).
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${REG_HOST_PORT}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

kubectl cluster-info --context "kind-${CLUSTER}" >/dev/null

# ingress-nginx — the Kestrel dashboard is reached through the ingress
# mapped to host ports 8080/8443 by cluster.yaml.
if ! kubectl get ns ingress-nginx >/dev/null 2>&1; then
    echo "installing ingress-nginx"
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
    kubectl wait --namespace ingress-nginx \
        --for=condition=Ready pod \
        --selector=app.kubernetes.io/component=controller \
        --timeout=180s
fi

# Control-plane namespace. The games namespace (Values.gamesNamespace)
# is created and owned by the Helm chart's templates/namespaces.yaml —
# pre-creating it here makes `helm install` fail, because Helm refuses to
# adopt a namespace that lacks its ownership label/annotations.
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Namespace
metadata:
  name: kestrel-system
EOF

echo
echo "✓ cluster ${CLUSTER} ready"
echo "  kubectl config use-context kind-${CLUSTER}"
