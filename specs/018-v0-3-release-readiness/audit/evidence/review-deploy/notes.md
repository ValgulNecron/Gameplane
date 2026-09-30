# Review: deploy/

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: the scripts' own header and inline comments (`deploy/` has no `specs.md`); `CLAUDE.md` "Local Development" (`make dev-up` / `dev-load` / `dev-install` / `dev-down`); the `Makefile` targets that drive these scripts; `test/e2e/e2e_suite_test.go` and `test/e2e/upgrade_e2e_test.go` for how the e2e and upgrade scripts are consumed.

## Scope reviewed

Read in full: `deploy/kind/cluster.yaml`, `deploy/kind/up.sh`, `deploy/kind/down.sh`, `deploy/kind/e2e.sh`, `deploy/kind/upgrade.sh` (all 5 files under `deploy/`).

Cross-checked: `Makefile:8-32` (variables), `:187-203` (e2e targets), `:260-281` (images), `:336-397` (dev-* targets); `test/e2e/e2e_suite_test.go:20-60` and `test/e2e/env.go:54` (the `GAMEPLANE_E2E_REUSE_CLUSTER` / `GAMEPLANE_E2E_CLUSTER` env vars the scripts advertise); `test/e2e/upgrade_e2e_test.go:110-125`; `modules/minecraft-java/samples/gameserver.yaml:41` and `operator/config/samples/gameplane_v1alpha1_gameserver.yaml:27` (the 30565 NodePort mapping); `charts/gameplane/templates/_helpers.tpl` (which images the chart derives).

Nothing under `deploy/` was skipped. Nothing was run: the scripts create and destroy kind clusters and Docker containers.

## Method

Traced each script's control flow under `set -euo pipefail`, looking at which `kubectl`/`helm` calls pick their target cluster implicitly. Compared the images each script loads with the images the chart will reference under the same values. Checked each env var and Make target the scripts mention against where it is consumed.

## Observations (no finding)

- MetalLB is pinned (`v0.14.9`, up.sh:31 and e2e.sh:30), and pool ranges are carved from the live kind bridge subnet with a /16 sanity check (up.sh:64-77, e2e.sh:63-76). The webhook race is handled by a bounded retry (10×5 s). `kubectl rollout status` before `kubectl wait --selector` correctly avoids the "no matching resources" exit.
- e2e.sh and upgrade.sh always delete a same-named cluster before creating one (e2e.sh:148-151, upgrade.sh:48-51). `kind create cluster` then sets the current context, so their later `kubectl`/`helm` calls hit the new cluster.
- e2e.sh creates the fake-OIDC issuer before the Helm install and gives the reason (the API runs discovery once at startup). The order is right.
- `GAMEPLANE_E2E_REUSE_CLUSTER` (e2e.sh:324) and `make test-e2e-keep` (e2e.sh:146) both exist and are consumed (`test/e2e/e2e_suite_test.go:30`, `Makefile:196-198`).
- The cluster.yaml host port 30565 matches the `nodePort: 30565` in both sample GameServers.
- upgrade.sh loads only operator/api/agent/sentinel for the new tree. That matches `test/e2e/upgrade_e2e_test.go:110-120`, which leaves capture off and disables web and the default ModuleSource.
- Already tracked, not re-reported: the `FROM_VERSION` default `0.2.0-beta.5` (upgrade.sh:36; F-(d), T051/PR #422).

## Candidate findings

held candidates: 1 (see OD-019)

### C-deploy-01: `up.sh` on an existing cluster keeps modifying it, against whatever kube context is current, while its header says it exits without changes
- **Location**: `deploy/kind/up.sh:6-7` (claim); `:154-159` (existing-cluster branch doesn't select a context); `:178`, `:194-201`, `:203-204` (via `install_metallb`/`apply_metallb_pools`), `:210` (unqualified `kubectl apply`/`wait`); `:190` (the only `--context` use, a read-only check that runs after the first apply)
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. `make dev-up` once, so `kind-gameplane-dev` exists.
  2. `kubectl config use-context <some other cluster>`, e.g. a shared lab cluster.
  3. `make dev-up` again. up.sh prints "cluster gameplane-dev already exists — skipping create", then runs `kubectl apply` of the `local-registry-hosting` ConfigMap (line 178) against the *current* context. The `--context kind-gameplane-dev` check at line 190 passes because that context still exists in the kubeconfig. The script then installs ingress-nginx if missing (194-201), MetalLB and its `IPAddressPool`/`L2Advertisement` objects (203-204), and the `gameplane-system` namespace (210), all into the other cluster. `make dev-install` (Makefile:384) follows and Helm-installs Gameplane there too. If the kubeconfig in use lacks the kind context (e.g. `KUBECONFIG` exported to a lab file), line 190 aborts the run, but only after line 178 has already applied the ConfigMap to the lab cluster.
- **Expected**: up.sh:6-7: "Idempotent — if the cluster already exists the script exits 0 without modifying it." At minimum, every mutating call targets `kind-${CLUSTER}`.
- **Actual**: The script modifies whichever cluster is current, which may not be the kind cluster. Installing MetalLB into a cluster that already has a load-balancer implementation disrupts that cluster's LoadBalancer Services.

### C-deploy-02: `make dev-up` on kind never loads the sentinel, tunnel or capture images the chart references, so wake-on-connect and tunnels fail to pull
- **Location**: `Makefile:367-371` (`dev-load` loads only operator/api/web/agent); chart image derivation at `charts/gameplane/templates/_helpers.tpl:39-81`; defaults `Makefile:8-9` (`REGISTRY ?= ghcr.io/valgulnecron/gameplane`, `TAG ?= dev`). The Makefile is outside `deploy/`; it is raised here because it is the kind dev path that `deploy/kind/up.sh` is part of.
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. `make dev-up` (CLUSTER=kind) builds every image (`images` builds operator, api, web, agent, telemetry-receiver, sentinel, mcp-server, capture-sidecar, audit-syslog-bridge and tunnel-*), but `dev-load` loads only 4 of them.
  2. `dev-install` sets `image.registry=ghcr.io/valgulnecron/gameplane` and `image.tag=dev`, so the operator is started with `--sentinel-image=ghcr.io/valgulnecron/gameplane/sentinel:dev` and `--tunnel-*-image=…:dev`.
  3. Arm a GameServer for wake-on-connect (`spec.idle.wakeOnConnect`) and let it sleep, or give it a tunnel. The sentinel or tunnel pod goes to `ErrImagePull`, because the kind nodes don't have the image and GHCR publishes no `:dev` tag. The same applies to `capture-sidecar`, `mcp-server`, `audit-syslog-bridge` and `telemetry-receiver` when their chart toggles are turned on in the dev cluster.
- **Expected**: CLAUDE.md: "`make dev-up` — Start Kind cluster + local OCI registry (:5001) + deploy Helm chart", and "`make dev-load` — Rebuild and reload local images into Kind". Every image the chart can reference under the dev values is available in the cluster.
- **Actual**: Only the 4 core images are loaded. Features that start the other components fail in the dev cluster, and the e2e path (e2e.sh:165-172) loads sentinel and capture-sidecar as well, so e2e doesn't reproduce the gap.

### C-deploy-03: e2e.sh's header and progress message name the wrong images and too few of them
- **Location**: `deploy/kind/e2e.sh:11`, `:165`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**: Line 11 says "Loads pre-built gameplane/{operator,api,agent}:<tag> images." Line 165 prints "loading gameplane/{operator,api,agent,sentinel,capture-sidecar}:${TAG} images into kind". The loop at 166-172 loads `gameplane-test/{operator,api,agent,sentinel,capture-sidecar}` (and builds any that are missing). Lines 177-191 also load `gameplane-test/gameprobe` (when present) and `gameplane-test/fakeoidc` (always, building it when missing).
- **Expected**: The header and message name the `gameplane-test/` prefix and the full image set.
- **Actual**: Both name the wrong registry prefix, and the header lists 3 of the up-to-7 images.

### C-deploy-04: `up.sh` fails when the `kind-registry` container exists but is stopped
- **Location**: `deploy/kind/up.sh:147-152`
- **Category**: error-handling
- **Suggested severity**: S4
- **Observation / repro**:
  1. `docker stop kind-registry` (or a Docker restart where the container didn't come back).
  2. `make dev-up`. The guard `docker inspect -f '{{.State.Running}}'` returns `false`, so the script runs `docker run -d --restart=always ... --name kind-registry registry:2`. Docker rejects this with a name conflict (the stopped container still holds the name), and `set -e` aborts the bootstrap.
- **Expected**: A stopped registry is restarted (`docker start kind-registry`), or the error message says what to do.
- **Actual**: The run aborts on a raw Docker "Conflict. The container name "/kind-registry" is already in use" error.

## Questions (not findings)

- `cluster.yaml:11-12` pins the kind apiserver to host `127.0.0.1:6443`. On a dev box that already listens on 6443 (a local k3s or kubeadm cluster), `kind create cluster` fails on the port conflict. Is the fixed port needed? The e2e and upgrade clusters use kind's random port.
- up.sh and e2e.sh carry identical copies of `install_metallb` and `apply_metallb_pools` (up.sh:36-133, e2e.sh:35-132). Not a defect today, but any fix has to be made twice. Should they share a sourced helper?
