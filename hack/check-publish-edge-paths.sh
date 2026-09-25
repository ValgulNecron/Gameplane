#!/usr/bin/env bash
# check-publish-edge-paths.sh: Verify that every directory a publish-edge.yaml
# matrix image's Dockerfile actually COPYs from is listed in the workflow's
# `on.push.paths` trigger (F-239).
#
# Purpose:
#   publish-edge.yaml rebuilds and pushes :edge images "when something that
#   lands in an image changes" (its own header comment). The only mechanical
#   link between "lands in an image" and "triggers a rebuild" is the `paths:`
#   list under `on.push`. If a Dockerfile gains a new COPY source directory
#   (or a directory is renamed) and `paths:` isn't updated to match, a push
#   that only touches that directory silently publishes nothing to :edge.
#
# What this checks:
#   1. Read the `component:`/`dockerfile:` pairs out of the `images` job's
#      matrix `include:` list.
#   2. For each pair, resolve the Dockerfile path (matrix.dockerfile, or
#      `<component>/Dockerfile` when omitted) and extract every top-level
#      directory named in a `COPY <src> ...` instruction (ignoring
#      multi-stage `COPY --from=...` lines, which don't read from the build
#      context).
#   3. Every such top-level directory must appear, verbatim, as `<dir>/**` in
#      the workflow's `on.push.paths` list (or the bare `<dir>` form used by
#      `go.work`/`go.work.sum`).
#
# This is a static, line-based check — it does not build any image. It
# cannot see a COPY source added inside a build stage that Docker BuildKit
# would skip, only what's textually present.
#
# Test overrides:
#   PUBLISH_EDGE_WORKFLOW: path to the workflow file to check
#     (default: .github/workflows/publish-edge.yaml)
#   PUBLISH_EDGE_DOCKERFILE_ROOT: directory Dockerfile paths are resolved
#     against (default: repo root)
#
# Exit codes:
#   0 = every COPY source directory is covered by on.push.paths
#   1 = at least one is missing, or the files could not be parsed

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$REPO_ROOT"

workflow="${PUBLISH_EDGE_WORKFLOW:-.github/workflows/publish-edge.yaml}"
dockerfile_root="${PUBLISH_EDGE_DOCKERFILE_ROOT:-.}"

if [[ ! -f "$workflow" ]] || [[ ! -r "$workflow" ]]; then
	echo "✗ Error: $workflow not found or unreadable" >&2
	exit 1
fi

# --- matrix component/dockerfile pairs --------------------------------------
# The matrix looks like:
#   matrix:
#     include:
#       - component: operator
#       - component: tunnel-frp
#         dockerfile: tunnel/Dockerfile.frp
matrix_block=$(awk '
  /^ *include: *$/ { in_matrix=1; next }
  in_matrix && /^ *steps: *$/ { exit }
  in_matrix { print }
' "$workflow")

if [[ -z "$matrix_block" ]]; then
	echo "✗ Error: could not find the images job'\''s matrix include: list in $workflow" >&2
	exit 1
fi

component=""
declare -a dockerfiles=()
while IFS= read -r line; do
	if [[ "$line" =~ component:\ *([A-Za-z0-9_-]+) ]]; then
		if [[ -n "$component" && -z "${pending_dockerfile:-}" ]]; then
			dockerfiles+=("$component/Dockerfile")
		fi
		component="${BASH_REMATCH[1]}"
		pending_dockerfile=""
	elif [[ "$line" =~ dockerfile:\ *([A-Za-z0-9_./-]+) ]]; then
		pending_dockerfile="${BASH_REMATCH[1]}"
		dockerfiles+=("$pending_dockerfile")
	fi
done <<<"$matrix_block"
if [[ -n "$component" && -z "${pending_dockerfile:-}" ]]; then
	dockerfiles+=("$component/Dockerfile")
fi

if [[ "${#dockerfiles[@]}" -eq 0 ]]; then
	echo "✗ Error: no matrix components found in $workflow" >&2
	exit 1
fi

# --- on.push.paths list ------------------------------------------------------
paths_block=$(awk '
  /^ *push: *$/ { in_push=1; next }
  in_push && /^ *paths: *$/ { in_paths=1; next }
  in_push && /^ *workflow_dispatch/ { exit }
  in_paths { print }
' "$workflow")

if [[ -z "$paths_block" ]]; then
	echo "✗ Error: could not find on.push.paths in $workflow" >&2
	exit 1
fi

# --- collect COPY source top-level directories ------------------------------
declare -A seen_dirs=()
missing=0
for df in "${dockerfiles[@]}"; do
	full="$dockerfile_root/$df"
	if [[ ! -f "$full" ]]; then
		echo "✗ Error: $full (referenced by the images matrix) does not exist" >&2
		missing=1
		continue
	fi
	while IFS= read -r src; do
		[[ -z "$src" ]] && continue
		dir="${src%%/*}"
		[[ -z "$dir" ]] && continue
		seen_dirs["$dir"]=1
	done < <(grep -E '^COPY ' "$full" | grep -v -- '--from=' | awk '{print $2}')
done

for dir in "${!seen_dirs[@]}"; do
	if ! grep -qE "\"${dir}/\*\*\"|'${dir}/\*\*'" <<<"$paths_block"; then
		echo "✗ missing from on.push.paths: ${dir}/** (a COPY source of one or more matrix Dockerfiles)" >&2
		missing=1
	fi
done

if [[ "$missing" -ne 0 ]]; then
	echo "✗ FAIL: publish-edge.yaml's on.push.paths does not cover every Dockerfile COPY input" >&2
	exit 1
fi

echo "✓ every COPY source directory across ${#dockerfiles[@]} matrix Dockerfiles is covered by on.push.paths"
