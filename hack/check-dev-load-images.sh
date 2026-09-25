#!/usr/bin/env bash
# check-dev-load-images.sh: Verify `make dev-load` loads every image
# `make images` builds (spec 018, F-255).
#
# `make images` and `make dev-load` each enumerate their own image list in
# the Makefile. Nothing ties the two together, so it is easy for a new
# component's image to be added to one and forgotten in the other — exactly
# what happened before F-255 (dev-load loaded 4 of the 12 built images).
# This check statically diffs the two lists using `make -n` dry runs rather
# than actually building or loading anything, so it needs no docker/kind
# daemon and is cheap enough to run in CI or locally without touching a
# cluster.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$REPO_ROOT"

# Image names `make images` builds: every `docker build -t REG/<name>:TAG`
# line in a dry run of the `images` target.
built_names="$(make -n images REGISTRY=REG TAG=TAG 2>/dev/null \
    | grep -o 'docker build -t REG/[^:]*:TAG' \
    | sed -e 's#^docker build -t REG/##' -e 's#:TAG$##' \
    | sort -u)"

# Image names `dev-load` actually loads: the `for img in ...` list on
# dev-load's shell loop, which make expands before the shell ever runs
# (only `$$img` itself is deferred to the shell).
loaded_names="$(make -n dev-load REGISTRY=REG TAG=TAG KIND_CLUSTER=CLUSTER 2>/dev/null \
    | sed -n 's/^for img in \(.*\); do \\*$/\1/p' \
    | tr ' ' '\n' \
    | sort -u)"

if [[ -z "$built_names" ]]; then
    echo "✗ could not determine the images \`make images\` builds — is the images target still named/shaped the same?" >&2
    exit 1
fi

missing=""
for name in $built_names; do
    if ! printf '%s\n' "$loaded_names" | grep -qx "$name"; then
        missing="$missing $name"
    fi
done

if [[ -n "$missing" ]]; then
    echo "✗ make dev-load does not load every image make images builds" >&2
    echo "  missing:$missing" >&2
    echo "  built:  $(printf '%s' "$built_names" | tr '\n' ' ')" >&2
    echo "  loaded: $(printf '%s' "$loaded_names" | tr '\n' ' ')" >&2
    exit 1
fi

echo "✓ dev-load loads every image make images builds ($(printf '%s' "$built_names" | tr '\n' ' '))"
