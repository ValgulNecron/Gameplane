#!/usr/bin/env bash
# Purpose: Capture cluster state (CRDs, PVCs, nodes, Helm) to JSON snapshots
# Usage: snapshot.sh <out-dir>
# Exit codes: 0 = success, 2 = missing jq or usage error
set -euo pipefail

: "${KUBECONFIG:=$HOME/kubelab.yaml}"; export KUBECONFIG

# Check for jq
if ! command -v jq &>/dev/null; then
  echo "ERROR: jq is required but not installed" >&2
  exit 2
fi

# Validate usage
if [[ $# -ne 1 ]]; then
  echo "Usage: snapshot.sh <out-dir>" >&2
  exit 2
fi

OUT_DIR="$1"
mkdir -p "$OUT_DIR"

# Get Gameplane release and namespaces
RELEASE="${RELEASE:-gameplane}"
API_NS="${API_NS:-gameplane-system}"

# Determine games namespace from Helm values
GAMES_NS="${GAMES_NS:-}"
if [[ -z "$GAMES_NS" ]]; then
  GAMES_NS=$(helm get values "$RELEASE" -n "$API_NS" -a -o json 2>/dev/null | jq -r '.gamesNamespace // empty' 2>/dev/null || true)
  GAMES_NS="${GAMES_NS:-gameplane-games}"
fi

echo "Capturing cluster state to $OUT_DIR"
echo "Release: $RELEASE, API namespace: $API_NS, Games namespace: $GAMES_NS"

# Array of CRD plurals (without group)
declare -a KINDS=("gameservers" "gametemplates" "backups" "backupschedules" "restores" "modules" "modulesources" "networkcaptures" "clusters")

# Helper function to capture CRD objects sorted by namespace then name
capture_crd() {
  local kind="$1"
  local fq_plural="${kind}.gameplane.local"
  local output_file="$OUT_DIR/crd-${kind}.json"

  # Fetch objects and convert to our format
  kubectl get "$fq_plural" -A -o json 2>/dev/null | jq -r '.items[] | {
    kind: .kind,
    name: .metadata.name,
    namespace: .metadata.namespace,
    uid: .metadata.uid,
    generation: .metadata.generation,
    phase: (.status.phase // null)
  }' | jq -s 'sort_by(.namespace, .name)' > "$output_file"

  echo "  $output_file"
}

# Capture each CRD kind
for kind in "${KINDS[@]}"; do
  capture_crd "$kind"
done

# Capture PVCs from both namespaces
pvcs_file="$OUT_DIR/pvcs.json"
(
  kubectl get pvc -n "$GAMES_NS" -o json 2>/dev/null || true
  kubectl get pvc -n "$API_NS" -o json 2>/dev/null || true
) | jq -r '.items[] // empty | {
  name: .metadata.name,
  namespace: .metadata.namespace,
  uid: .metadata.uid,
  capacity: (.status.capacity.storage // null),
  volumeName: (.spec.volumeName // null),
  phase: (.status.phase // null)
}' | jq -s 'sort_by(.namespace, .name)' > "$pvcs_file"
echo "  $pvcs_file"

# Capture nodes
nodes_file="$OUT_DIR/nodes.json"
kubectl get nodes -o json 2>/dev/null | jq -r '.items[] | {
  name: .metadata.name,
  uid: .metadata.uid,
  roles: ([.metadata.labels | to_entries[] | select(.key | startswith("node-role.kubernetes.io/")) | .key | sub("node-role.kubernetes.io/"; "")] | sort),
  schedulable: (.spec.unschedulable != true),
  kubeletVersion: .status.nodeInfo.kubeletVersion
}' | jq -s 'sort_by(.name)' > "$nodes_file"
echo "  $nodes_file"

# Capture helm list
helm_list_file="$OUT_DIR/helm-list.json"
helm list -A -o json > "$helm_list_file"
echo "  $helm_list_file"

# Capture helm values with secrets redacted
helm_values_file="$OUT_DIR/helm-values.json"
helm get values "$RELEASE" -n "$API_NS" -o json 2>/dev/null | jq 'walk(
  if type == "object" then
    with_entries(
      if (.key | test("(?i)secret|token|password|key|dsn")) then
        .value = "<redacted>"
      else
        .
      end
    )
  else
    .
  end
)' > "$helm_values_file"
echo "  $helm_values_file"

# Capture images from Deployments, StatefulSets, DaemonSets in API namespace
images_file="$OUT_DIR/images.json"
(
  kubectl get deployments,statefulsets,daemonsets -n "$API_NS" -o json 2>/dev/null | jq -r '.items[] | {
    kind: .kind,
    name: .metadata.name,
    namespace: .metadata.namespace,
    generation: .metadata.generation,
    uid: .metadata.uid,
    images: (([.spec.template.spec.containers[]?.image] + [.spec.template.spec.initContainers[]?.image]) | map(select(. != null)))
  }' | jq -s 'sort_by(.namespace, .name)'
) > "$images_file"
echo "  $images_file"

# Capture metadata
meta_file="$OUT_DIR/meta.json"
KUBE_CONTEXT=$(kubectl config current-context 2>/dev/null || echo "unknown")
jq -n \
  --arg capturedAt "$(date -u +%FT%TZ)" \
  --arg release "$RELEASE" \
  --arg apiNamespace "$API_NS" \
  --arg gamesNamespace "$GAMES_NS" \
  --arg kubeContext "$KUBE_CONTEXT" \
  '{capturedAt: $capturedAt, release: $release, apiNamespace: $apiNamespace, gamesNamespace: $gamesNamespace, kubeContext: $kubeContext}' > "$meta_file"
echo "  $meta_file"

echo "Done. Files written:"
ls -1 "$OUT_DIR"/*.json
