#!/usr/bin/env bash
# build.sh — build and push Gameplane module bundles to an OCI registry.
#
# Each module lives in modules/<name>/{template.yaml, module.yaml,
# README.md, icon.png}. This script wraps `oras push` with the
# Gameplane media types so the operator's pull path can find each
# layer by its filename annotation.
#
# Usage:
#   modules/build.sh push --registry ghcr.io/valgulnecron/gameplane-modules
#   modules/build.sh push --registry localhost:5001 --name minecraft-java --insecure
#   modules/build.sh push --registry $REG --plain-http   # all modules, plain-http
#
# Required: oras >= 1.2.0  (https://oras.land/docs/installation)
# For --sign: cosign >= 2.0  (https://docs.sigstore.dev/cosign/installation)

set -euo pipefail

ARTIFACT_TYPE="application/vnd.gameplane.module.v1+json"
MEDIA_METADATA="application/vnd.gameplane.module.metadata.v1+yaml"
MEDIA_TEMPLATE="application/vnd.gameplane.module.template.v1+yaml"
MEDIA_README="application/vnd.gameplane.module.readme.v1+md"
MEDIA_ICON="image/png"

usage() {
  cat <<USAGE
Usage: $0 push [flags]

Flags:
  --registry <ref>   Registry/repo prefix (e.g. ghcr.io/valgulnecron/gameplane-modules)   [required]
  --name <name>      Push only this module (defaults: every dir under modules/)
  --plain-http       Use plain HTTP (for local kind registries)
  --insecure         Skip TLS verification
  --tag-latest       Also tag :latest in addition to module.yaml's version
  --sign             cosign-sign each pushed bundle (keyed, offline; no Rekor)

Reads each modules/<name>/module.yaml for the version. The bundle is
pushed to <registry>/<name>:<version>.

With --sign, set COSIGN_PRIVATE_KEY to the PEM private key (and
COSIGN_PASSWORD if it is encrypted); each bundle is signed by digest with
\`cosign sign --tlog-upload=false\`, which the operator verifies offline via
ModuleSource.spec.verify.key. See docs/module-authoring.md.
USAGE
}

cmd="${1:-}"
shift || true
case "$cmd" in
  push) ;;
  ""|-h|--help) usage; exit 0 ;;
  *) echo "unknown command: $cmd" >&2; usage; exit 2 ;;
esac

REGISTRY=""
TARGET=""
PLAIN_HTTP=""
INSECURE=""
TAG_LATEST=0
SIGN=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --registry)   REGISTRY="$2";       shift 2 ;;
    --name)       TARGET="$2";         shift 2 ;;
    --plain-http) PLAIN_HTTP=1;        shift ;;
    --insecure)   INSECURE=1;          shift ;;
    --tag-latest) TAG_LATEST=1;        shift ;;
    --sign)       SIGN=1;              shift ;;
    -h|--help)    usage; exit 0 ;;
    *) echo "unknown flag: $1" >&2; usage; exit 2 ;;
  esac
done

[[ -n "$REGISTRY" ]] || { echo "--registry required" >&2; exit 2; }
command -v oras >/dev/null || { echo "oras not in PATH" >&2; exit 2; }
if (( SIGN )); then
  command -v cosign >/dev/null || { echo "cosign not in PATH (required for --sign)" >&2; exit 2; }
  [[ -n "${COSIGN_PRIVATE_KEY:-}" ]] || { echo "--sign requires COSIGN_PRIVATE_KEY (PEM private key) in the environment" >&2; exit 2; }
fi

# Resolve modules dir from the script's location, so callers can run
# this from anywhere.
MODULES_DIR="$(cd "$(dirname "$0")" && pwd)"

# yaml-grep: extract a top-level YAML scalar by key (poor-man's parser
# good enough for module.yaml — there are no nested mappings or quoted
# colons in the fields we read).
ymv() {
  local file="$1" key="$2"
  awk -v k="^$key:" 'BEGIN{IGNORECASE=0} $0 ~ k {sub(/^[^:]+:[ \t]*/, ""); gsub(/^["'"'"']|["'"'"']$/, ""); print; exit}' "$file"
}

push_one() {
  local name="$1"
  local dir="$MODULES_DIR/$name"
  local meta="$dir/module.yaml"
  local tmpl="$dir/template.yaml"

  [[ -f "$meta" ]] || { echo "skip $name: no module.yaml" >&2; return 0; }
  [[ -f "$tmpl" ]] || { echo "skip $name: no template.yaml" >&2; return 0; }

  local declared_name version
  declared_name="$(ymv "$meta" name)"
  version="$(ymv "$meta" version)"
  [[ -n "$version" ]]      || { echo "$name: module.yaml is missing version" >&2; return 1; }
  [[ "$declared_name" == "$name" ]] || {
    echo "$name: module.yaml#name=$declared_name does not match directory $name" >&2
    return 1
  }

  local ref="$REGISTRY/$name:$version"
  echo ">> pushing $ref"

  local args=( push )
  [[ -n "$PLAIN_HTTP" ]] && args+=( --plain-http )
  [[ -n "$INSECURE"  ]] && args+=( --insecure )
  args+=( --artifact-type "$ARTIFACT_TYPE" "$ref" )

  # Use a subshell with cd so oras records each layer as just the filename
  # (no leading directory) — that's what the puller expects in the title
  # annotation.
  local layer_args=( "module.yaml:$MEDIA_METADATA" "template.yaml:$MEDIA_TEMPLATE" )
  [[ -f "$dir/README.md" ]] && layer_args+=( "README.md:$MEDIA_README" )
  [[ -f "$dir/icon.png"  ]] && layer_args+=( "icon.png:$MEDIA_ICON"   )

  # Capture combined output so we can recover the pushed manifest digest
  # for signing; still echo it so the push stays visible in CI logs.
  local out
  out="$( ( cd "$dir" && oras "${args[@]}" "${layer_args[@]}" ) 2>&1 )"
  printf '%s\n' "$out"

  if (( SIGN )); then
    local digest
    digest="$(printf '%s\n' "$out" | grep -oE 'sha256:[0-9a-f]{64}' | head -n1)"
    [[ -n "$digest" ]] || { echo "$name: could not parse pushed digest from oras output" >&2; return 1; }
    echo ">> signing $REGISTRY/$name@$digest"
    # Keyed + offline: no transparency-log upload, matching the operator's
    # offline keyed verify path (IgnoreTlog/Offline). cosign reads the key
    # from $COSIGN_PRIVATE_KEY and the passphrase from $COSIGN_PASSWORD.
    local sargs=( sign --key env://COSIGN_PRIVATE_KEY --tlog-upload=false --yes )
    # --plain-http needs --allow-http-registry (force HTTP); --allow-insecure-
    # registry only skips TLS verification on an HTTPS connection, so against a
    # plain-HTTP registry it errors "server gave HTTP response to HTTPS client"
    # (except for localhost, which go-containerregistry auto-treats as HTTP).
    [[ -n "$PLAIN_HTTP" ]] && sargs+=( --allow-http-registry )
    [[ -n "$INSECURE"  ]] && sargs+=( --allow-insecure-registry )
    cosign "${sargs[@]}" "$REGISTRY/$name@$digest"
  fi

  if (( TAG_LATEST )); then
    local latest="$REGISTRY/$name:latest"
    echo ">> tagging $latest"
    local targs=( tag )
    [[ -n "$PLAIN_HTTP" ]] && targs+=( --plain-http )
    [[ -n "$INSECURE"  ]] && targs+=( --insecure )
    oras "${targs[@]}" "$ref" latest
  fi
}

if [[ -n "$TARGET" ]]; then
  push_one "$TARGET"
  exit 0
fi

# Push every directory that has a module.yaml.
for d in "$MODULES_DIR"/*/; do
  name="$(basename "$d")"
  [[ -f "$d/module.yaml" ]] || continue
  push_one "$name"
done
