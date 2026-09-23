#!/usr/bin/env bash
# Purpose: List any remaining audit018- resources that should have been cleaned up
# Usage: cleanup-check.sh
# Exit codes: 0 = no leftover resources, 1 = leftover resources remain, 2 = jq missing
# Note: API-side leftovers (users, roles, shares) are checked separately via the API
#       (see contracts/test-resources.md § Cleanup)
set -euo pipefail

: "${KUBECONFIG:=$HOME/kubelab.yaml}"; export KUBECONFIG

# Check for jq
if ! command -v jq &>/dev/null; then
  echo "ERROR: jq is required but not installed" >&2
  exit 2
fi

leftovers_found=0

echo "Checking for remaining audit018- resources..."

# Get all the resource types from the spec
# gameservers, backups, restores, backupschedules, networkcaptures, modulesources, modules (all with .gameplane.local),
# and pvcs (no group)

# Construct fully qualified names
declare -a GAMEPLANE_KINDS=(
  "gameservers.gameplane.local"
  "backups.gameplane.local"
  "restores.gameplane.local"
  "backupschedules.gameplane.local"
  "networkcaptures.gameplane.local"
  "modulesources.gameplane.local"
  "modules.gameplane.local"
)

# Check Gameplane CRD objects
# Note: results are captured into a variable and iterated via a here-string
# (not piped into `while read`) so that leftovers_found, set inside the loop,
# is visible to the exit-code check below (a piped `while` runs in a subshell
# in bash and would silently discard the assignment).
for kind in "${GAMEPLANE_KINDS[@]}"; do
  lines=$(kubectl get "$kind" -A -o json 2>/dev/null | jq -r '.items[] | select(.metadata.name | startswith("audit018-")) | "\(.metadata.namespace)/\(.kind)/\(.metadata.name)"') || true
  if [[ -n "$lines" ]]; then
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      echo "  $line"
      leftovers_found=1
    done <<< "$lines"
  fi
done

# Check PVCs
pvc_lines=$(kubectl get pvc -A -o json 2>/dev/null | jq -r '.items[] | select(.metadata.name | startswith("audit018-")) | "\(.metadata.namespace)/PersistentVolumeClaim/\(.metadata.name)"') || true
if [[ -n "$pvc_lines" ]]; then
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    echo "  $line"
    leftovers_found=1
  done <<< "$pvc_lines"
fi

if [[ $leftovers_found -eq 1 ]]; then
  exit 1
fi

echo "No audit018- resources found."
exit 0
