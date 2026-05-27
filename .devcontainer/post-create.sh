#!/usr/bin/env bash
# Devcontainer post-create bootstrap for Kestrel.
#
# Installs the extra tooling the Makefile expects but the devcontainer
# features don't provide (kind, oras, golangci-lint), pre-fetches Go
# modules + envtest assets, and installs the web npm deps so the first
# `make test` / `make web-dev` run is fast.
#
# Idempotent: every step is `command -v` guarded or version-checked.

set -euo pipefail

log() { printf "\n\033[1;34m[devcontainer]\033[0m %s\n" "$*"; }

ARCH="$(dpkg --print-architecture)"   # amd64 | arm64
GOBIN_DIR="$(go env GOPATH)/bin"
mkdir -p "$GOBIN_DIR"
case ":$PATH:" in *":$GOBIN_DIR:"*) ;; *) export PATH="$GOBIN_DIR:$PATH" ;; esac

# ---------- kind ----------
KIND_VERSION="v0.24.0"
if ! command -v kind >/dev/null 2>&1; then
	log "installing kind ${KIND_VERSION}"
	curl -fsSL -o /tmp/kind \
		"https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-linux-${ARCH}"
	sudo install -m 0755 /tmp/kind /usr/local/bin/kind
	rm -f /tmp/kind
fi

# ---------- oras (≥ 1.2.0; required by modules/build.sh) ----------
ORAS_VERSION="1.2.0"
if ! command -v oras >/dev/null 2>&1; then
	log "installing oras ${ORAS_VERSION}"
	curl -fsSL -o /tmp/oras.tar.gz \
		"https://github.com/oras-project/oras/releases/download/v${ORAS_VERSION}/oras_${ORAS_VERSION}_linux_${ARCH}.tar.gz"
	sudo tar -C /usr/local/bin -xzf /tmp/oras.tar.gz oras
	sudo chmod 0755 /usr/local/bin/oras
	rm -f /tmp/oras.tar.gz
fi

# ---------- golangci-lint ----------
# The Go devcontainer feature pre-installs golangci-lint at /go/bin, but
# pulls v2 by default — and the project's .golangci.yml is v1 format
# (`disable-all`, `enable:` list). We force v1.
#
# Additionally, the *release binary* of v1.64.8 is built with Go 1.24,
# which makes it refuse to lint this repo's Go 1.25.0 module ("Go
# language version used to build golangci-lint is lower than the
# targeted Go version"). Building from source with the in-container
# Go toolchain sidesteps that — same trick the host uses.
GOLANGCI_VERSION="v1.64.8"
GOLANGCI_BIN="/go/bin/golangci-lint"
if ! "$GOLANGCI_BIN" --version 2>/dev/null | grep -qE 'version v?1\.6[4-9]\.' \
		|| "$GOLANGCI_BIN" --version 2>/dev/null | grep -q 'built with go1\.2[0-4]\.'; then
	log "installing golangci-lint ${GOLANGCI_VERSION} (built from source with Go $(go env GOVERSION))"
	GOBIN=/go/bin go install \
		"github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCI_VERSION}"
fi

# ---------- envtest assets (K8s 1.31; pulled lazily by the Makefile but
# we warm the cache here so the first `make test-integration` doesn't
# need network) ----------
if [ ! -x "$GOBIN_DIR/setup-envtest" ]; then
	log "installing setup-envtest"
	GOBIN="$GOBIN_DIR" go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
fi
log "fetching envtest binaries for K8s 1.31.0"
"$GOBIN_DIR/setup-envtest" use 1.31.0 >/dev/null

# ---------- Go module cache warmup ----------
for m in operator api agent test/e2e; do
	log "go mod download ($m)"
	( cd "$m" && go mod download )
done

# ---------- Web deps ----------
log "npm ci (web)"
( cd web && npm ci )

# ---------- Friendly summary ----------
log "tool versions"
{
	go version
	node --version
	npm --version
	docker version --format 'docker {{.Server.Version}}' 2>/dev/null || true
	kubectl version --client=true --output=yaml 2>/dev/null | grep gitVersion || true
	helm version --short
	kind version
	oras version | head -1
	golangci-lint --version | head -1
} || true

cat <<'EOF'

[devcontainer] ready.

Quick commands:
  make lint test         # Go + web unit tests, lint
  make test-integration  # envtest tier (operator + api)
  make dev-up            # kind cluster + helm install (~5 min)
  make web-dev           # Vite dev server (forwarded on port 5173)
  make test-e2e          # full kind-based e2e (~10–20 min)

Notes:
  * Docker runs *inside* the devcontainer (docker-in-docker). The kind
    cluster, local registry, and built images all live in this
    container's docker daemon — they're gone when the container is
    rebuilt.
  * Ports 5173 (web dev), 8080/8443 (ingress), and 5001 (OCI registry)
    are forwarded to your host.
EOF
