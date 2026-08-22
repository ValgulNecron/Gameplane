#!/usr/bin/env bash
# Bootstrap a local kind cluster for Gameplane development.
#
# Usage: deploy/kind/up.sh [cluster-name]
#
# Idempotent — if the cluster already exists the script exits 0 without
# modifying it. Invoked by `make dev-up`.

set -euo pipefail

CLUSTER="${1:-gameplane-dev}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG="${HERE}/cluster.yaml"

# Local OCI registry settings — published on the host as
# localhost:${REG_HOST_PORT} for `oras push`, and reachable from cluster
# pods as ${REG_NAME}:${REG_INTERNAL_PORT} after the kind-network attach.
REG_NAME="kind-registry"
REG_HOST_PORT="${GAMEPLANE_REG_HOST_PORT:-5001}"
REG_INTERNAL_PORT=5000

need() { command -v "$1" >/dev/null 2>&1 || { echo "missing: $1" >&2; exit 1; }; }
need kind
need kubectl
need helm
need docker

# MetalLB manifest pin. Bumping it is a deliberate edit — never fetch a
# floating ref, or a fresh cluster silently changes its load-balancer
# implementation.
METALLB_VERSION="v0.14.9"

# kind ships no LoadBalancer implementation, so without MetalLB every
# LoadBalancer Service sits at <pending> forever and a GameServer's
# spec.networking.addressPool preference has nothing to act on.
install_metallb() {
    if kubectl get namespace metallb-system >/dev/null 2>&1; then
        echo "MetalLB already installed — skipping"
        return 0
    fi
    echo "installing MetalLB ${METALLB_VERSION}"
    kubectl apply -f \
        "https://raw.githubusercontent.com/metallb/metallb/${METALLB_VERSION}/config/manifests/metallb-native.yaml"
    kubectl wait --namespace metallb-system \
        --for=condition=Available deployment/controller --timeout=180s
    # `kubectl wait --selector` exits 1 with "no matching resources found" when
    # the selector matches nothing *at call time* — it does not poll for the
    # first match — which under `set -e` aborts the whole bootstrap. Wait for
    # the DaemonSet to materialise first; rollout status does poll.
    kubectl rollout status --namespace metallb-system \
        daemonset/speaker --timeout=180s
    kubectl wait --namespace metallb-system \
        --for=condition=Ready pod \
        --selector=component=speaker \
        --timeout=180s
}

# Two address pools to develop the pool override against. Their ranges MUST be
# carved from the kind docker bridge subnet: an address outside it is
# unroutable from the nodes, so MetalLB would hand out addresses nothing can
# reach and a "working" pool assignment would point at a dead endpoint. The
# prefix is read back from docker rather than hardcoded, because the bridge
# subnet is docker's choice (commonly 172.18.0.0/16) and not guaranteed.
apply_metallb_pools() {
    local subnet prefix range_east range_west attempt
    subnet="$(docker network inspect kind \
        -f '{{range .IPAM.Config}}{{.Subnet}} {{end}}' | tr ' ' '\n' | grep -m1 '\.' || true)"
    case "${subnet}" in
    */16) ;;
    *)
        echo "unexpected kind bridge subnet '${subnet}' (want an IPv4 /16) — cannot carve address pools" >&2
        return 1
        ;;
    esac
    prefix="$(echo "${subnet}" | cut -d. -f1,2)"
    range_east="${prefix}.255.100-${prefix}.255.110"
    range_west="${prefix}.255.200-${prefix}.255.210"

    echo "defining MetalLB pools pool-us-east (${range_east}) and pool-us-west (${range_west})"
    # IPAddressPool and L2Advertisement are gated by MetalLB's validating
    # webhook, which keeps refusing connections for a few seconds after the
    # controller Deployment reports Available. Retry instead of racing it.
    for attempt in 1 2 3 4 5 6 7 8 9 10; do
        if kubectl apply -f - <<EOF
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: pool-us-east
  namespace: metallb-system
spec:
  addresses:
    - ${range_east}
---
# pool-us-west never auto-assigns: only a Service that explicitly names it
# draws from it, so "the address came from pool-us-west" is proof the
# operator's pool translation ran, not an accident of allocation order.
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: pool-us-west
  namespace: metallb-system
spec:
  autoAssign: false
  addresses:
    - ${range_west}
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: pool-us-east
  namespace: metallb-system
spec:
  ipAddressPools:
    - pool-us-east
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: pool-us-west
  namespace: metallb-system
spec:
  ipAddressPools:
    - pool-us-west
EOF
        then
            return 0
        fi
        echo "  pool apply failed (attempt ${attempt}/10) — waiting for the MetalLB webhook"
        sleep 5
    done
    echo "MetalLB pools never applied — webhook did not come up" >&2
    return 1
}

# The chart defaults operator.addressManager to "none", which records a pool
# preference as unhonored instead of translating it. This cluster runs MetalLB
# (above), so the operator must be told so — but not from here: `make dev-up`
# runs `make dev-install` after this script, and that is a
# `helm upgrade --install` *without* --reuse-values, so any value set by a
# helm upgrade here is discarded moments later. The flavor is passed on that
# install instead, from the Makefile's ADDRESS_MANAGER (kind default:
# metallb).

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

# ingress-nginx — the Gameplane dashboard is reached through the ingress
# mapped to host ports 8080/8443 by cluster.yaml.
if ! kubectl get ns ingress-nginx >/dev/null 2>&1; then
    echo "installing ingress-nginx"
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
    kubectl wait --namespace ingress-nginx \
        --for=condition=Ready pod \
        --selector=app.kubernetes.io/component=controller \
        --timeout=180s
fi

install_metallb
apply_metallb_pools

# Control-plane namespace. The games namespace (Values.gamesNamespace)
# is created and owned by the Helm chart's templates/namespaces.yaml —
# pre-creating it here makes `helm install` fail, because Helm refuses to
# adopt a namespace that lacks its ownership label/annotations.
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Namespace
metadata:
  name: gameplane-system
EOF

echo
echo "✓ cluster ${CLUSTER} ready"
echo "  kubectl config use-context kind-${CLUSTER}"
echo "  MetalLB ${METALLB_VERSION} with pools pool-us-east (auto-assign) and pool-us-west (explicit only)"
