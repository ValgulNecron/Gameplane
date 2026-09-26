#!/usr/bin/env bash
# Purpose: Compare two cluster snapshots and report changes
# Usage: snapshot-diff.sh <before-dir> <after-dir>
# Exit codes: 0 = no mismatches, 1 = mismatches found, 2 = usage error or missing files
set -euo pipefail

: "${KUBECONFIG:=$HOME/kubelab.yaml}"; export KUBECONFIG

# Check for jq
if ! command -v jq &>/dev/null; then
  echo "ERROR: jq is required but not installed" >&2
  exit 2
fi

# Validate usage
if [[ $# -ne 2 ]]; then
  echo "Usage: snapshot-diff.sh <before-dir> <after-dir>" >&2
  exit 2
fi

BEFORE_DIR="$1"
AFTER_DIR="$2"

# Check that directories exist
if [[ ! -d "$BEFORE_DIR" ]] || [[ ! -d "$AFTER_DIR" ]]; then
  echo "ERROR: One or both directories do not exist" >&2
  exit 2
fi

mismatch_found=0

# Files to compare (excluding images.json, helm-list.json, helm-values.json)
declare -a CRD_KINDS=("gameservers" "gametemplates" "backups" "backupschedules" "restores" "modules" "modulesources" "networkcaptures" "clusters")

compare_crd() {
  local kind="$1"
  local filename="crd-${kind}.json"
  local before_file="$BEFORE_DIR/$filename"
  local after_file="$AFTER_DIR/$filename"

  if [[ ! -f "$before_file" ]]; then
    echo "WARNING: $filename not found in before-dir" >&2
    return
  fi

  if [[ ! -f "$after_file" ]]; then
    echo "ERROR: $filename not found in after-dir" >&2
    mismatch_found=1
    return
  fi

  # Create keyed versions and compare, excluding audit018- objects
  local before_keyed=$(jq -r '.[] | select(.name | startswith("audit018-") | not) | "\(.kind)/\(.namespace)/\(.name) uid=\(.uid) gen=\(.generation)"' "$before_file" | sort)
  local after_keyed=$(jq -r '.[] | select(.name | startswith("audit018-") | not) | "\(.kind)/\(.namespace)/\(.name) uid=\(.uid) gen=\(.generation)"' "$after_file" | sort)

  # Check for MISSING
  while IFS= read -r line; do
    if [[ -z "$line" ]]; then continue; fi
    local key=$(echo "$line" | cut -d' ' -f1)
    if ! grep -q "^$key " <<< "$after_keyed"; then
      echo "MISSING: $line"
      mismatch_found=1
    fi
  done <<< "$before_keyed"

  # Check for UID/generation mismatches
  while IFS= read -r before_line; do
    if [[ -z "$before_line" ]]; then continue; fi
    local key=$(echo "$before_line" | cut -d' ' -f1)
    local after_line=$(grep "^$key " <<< "$after_keyed" || true)

    if [[ -n "$after_line" ]]; then
      local before_uid=$(echo "$before_line" | grep -oP 'uid=\K[^ ]+')
      local after_uid=$(echo "$after_line" | grep -oP 'uid=\K[^ ]+')

      if [[ "$before_uid" != "$after_uid" ]]; then
        echo "UID MISMATCH: $key ($before_uid → $after_uid)"
        mismatch_found=1
      fi

      local before_gen=$(echo "$before_line" | grep -oP 'gen=\K[^ ]+')
      local after_gen=$(echo "$after_line" | grep -oP 'gen=\K[^ ]+')

      if [[ "$before_gen" != "$after_gen" ]]; then
        echo "GENERATION MISMATCH: $key ($before_gen → $after_gen)"
        mismatch_found=1
      fi
    fi
  done <<< "$before_keyed"

  # Check for NEW (warning)
  while IFS= read -r line; do
    if [[ -z "$line" ]]; then continue; fi
    local key=$(echo "$line" | cut -d' ' -f1)
    if ! grep -q "^$key " <<< "$before_keyed"; then
      echo "NEW (warning): $line"
      mismatch_found=1
    fi
  done <<< "$after_keyed"
}

# Compare CRD files
for kind in "${CRD_KINDS[@]}"; do
  compare_crd "$kind"
done

# Compare PVCs
if [[ -f "$BEFORE_DIR/pvcs.json" ]] && [[ -f "$AFTER_DIR/pvcs.json" ]]; then
  pvcs_before=$(jq -r '.[] | select(.name | startswith("audit018-") | not) | "\(.namespace)/\(.name) uid=\(.uid)"' "$BEFORE_DIR/pvcs.json" | sort)
  pvcs_after=$(jq -r '.[] | select(.name | startswith("audit018-") | not) | "\(.namespace)/\(.name) uid=\(.uid)"' "$AFTER_DIR/pvcs.json" | sort)

  # Check for MISSING
  while IFS= read -r line; do
    if [[ -z "$line" ]]; then continue; fi
    key=$(echo "$line" | cut -d' ' -f1)
    if ! grep -q "^$key " <<< "$pvcs_after"; then
      echo "MISSING PVC: $line"
      mismatch_found=1
    fi
  done <<< "$pvcs_before"

  # Check for UID mismatches
  while IFS= read -r before_line; do
    if [[ -z "$before_line" ]]; then continue; fi
    key=$(echo "$before_line" | cut -d' ' -f1)
    after_line=$(grep "^$key " <<< "$pvcs_after" || true)

    if [[ -n "$after_line" ]]; then
      before_uid=$(echo "$before_line" | grep -oP 'uid=\K[^ ]+')
      after_uid=$(echo "$after_line" | grep -oP 'uid=\K[^ ]+')

      if [[ "$before_uid" != "$after_uid" ]]; then
        echo "PVC UID MISMATCH: $key ($before_uid → $after_uid)"
        mismatch_found=1
      fi
    fi
  done <<< "$pvcs_before"

  # Check for NEW
  while IFS= read -r line; do
    if [[ -z "$line" ]]; then continue; fi
    key=$(echo "$line" | cut -d' ' -f1)
    if ! grep -q "^$key " <<< "$pvcs_before"; then
      echo "NEW PVC (warning): $line"
      mismatch_found=1
    fi
  done <<< "$pvcs_after"
fi

# Compare nodes
if [[ -f "$BEFORE_DIR/nodes.json" ]] && [[ -f "$AFTER_DIR/nodes.json" ]]; then
  nodes_before=$(jq -r '.[] | "\(.name) uid=\(.uid)"' "$BEFORE_DIR/nodes.json" | sort)
  nodes_after=$(jq -r '.[] | "\(.name) uid=\(.uid)"' "$AFTER_DIR/nodes.json" | sort)

  # Check for MISSING nodes
  while IFS= read -r line; do
    if [[ -z "$line" ]]; then continue; fi
    name=$(echo "$line" | cut -d' ' -f1)
    if ! grep -q "^$name " <<< "$nodes_after"; then
      echo "MISSING NODE: $line"
      mismatch_found=1
    fi
  done <<< "$nodes_before"

  # Check for UID mismatches
  while IFS= read -r before_line; do
    if [[ -z "$before_line" ]]; then continue; fi
    name=$(echo "$before_line" | cut -d' ' -f1)
    after_line=$(grep "^$name " <<< "$nodes_after" || true)

    if [[ -n "$after_line" ]]; then
      before_uid=$(echo "$before_line" | grep -oP 'uid=\K[^ ]+')
      after_uid=$(echo "$after_line" | grep -oP 'uid=\K[^ ]+')

      if [[ "$before_uid" != "$after_uid" ]]; then
        echo "NODE UID MISMATCH: $name ($before_uid → $after_uid)"
        mismatch_found=1
      fi
    fi
  done <<< "$nodes_before"

  # Check for NEW nodes
  while IFS= read -r line; do
    if [[ -z "$line" ]]; then continue; fi
    name=$(echo "$line" | cut -d' ' -f1)
    if ! grep -q "^$name " <<< "$nodes_before"; then
      echo "NEW NODE (warning): $line"
      mismatch_found=1
    fi
  done <<< "$nodes_after"
fi

# Skip images.json
echo "NOTE: images.json skipped (Gameplane Deployments/StatefulSets/DaemonSets change on purpose during RC upgrades)"

# Skip helm-list.json and helm-values.json
echo "NOTE: helm-list.json and helm-values.json skipped (Gameplane release changes on purpose)"

if [[ $mismatch_found -eq 1 ]]; then
  exit 1
fi

exit 0
