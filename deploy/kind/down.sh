#!/usr/bin/env bash
# Tear down the Kestrel dev kind cluster.

set -euo pipefail

CLUSTER="${1:-kestrel-dev}"

if ! kind get clusters | grep -qx "${CLUSTER}"; then
    echo "cluster ${CLUSTER} not found — nothing to do"
    exit 0
fi

kind delete cluster --name "${CLUSTER}"
