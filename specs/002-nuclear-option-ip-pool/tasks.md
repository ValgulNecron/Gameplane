---

description: "Task list for feature 002 — Nuclear Option Module & Load-Balancer IP Pool Override"
---

# Tasks: Nuclear Option Module & Load-Balancer IP Pool Override

**Input**: Design documents from `/specs/002-nuclear-option-ip-pool/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/address-pool-api.md, contracts/module-contract.md, contracts/nuclear-option-remote-command.md, quickstart.md

**Tests**: Test tasks ARE included — the project constitution (Principle I, E2E-Tested Delivery) and the coverage gates (operator 72%, api 80%, agent 90%, gameaction 91%, web 92/76/82/92) make them mandatory, not optional.

**Organization**: Tasks are grouped by user story so each story can be implemented, tested, and shipped as independently as its declared dependencies allow.

## ⚠️ Read this before starting

This feature is **two independent tracks that share no code**.

- **Track B — load-balancer address-pool override** (US2, US4, US6). **No external unknowns.** Everything it needs is decided: two typed fields on `GameServerNetworking`, MetalLB annotation translation, Cilium label + annotation translation, status conditions, dashboard surfacing. **Track B ships FIRST and is the MVP.**
- **Track A — Nuclear Option game module** (US1, US3, US5). **GATED.** Track A must not be considered shippable until BOTH of the following are resolved:
  1. **The display-name spec conflict** (T022). Spec FR-007 and US3 Acceptance Scenario 1 promise a player list containing a display name, but the dedicated server returns only `steamId` and `faction` — upstream removed `displayName` because the server runs headless. The decision (Steam Web API hydration routed through this repo's `netguard` SSRF dial-guard **vs.** amending the spec) is open and is **not** pre-decided by this task list.
  2. **The per-command JSON response body shapes** (T019), which are undocumented and must be confirmed against a real running server started with `-ServerRemoteCommands 7779`.

Additional standing constraints for every task below:

- **No local execution.** Builds, tests, lint, codegen and cluster runs happen on GitHub Actions (or the sanctioned remote build host), never on the maintainer's machine. Codegen output is still committed alongside the source change that triggered it; CI is the gate that catches drift.
- **Two-repo change.** `modules/` is a git submodule pointing at the separate `gameplane-module` repo. Module files are committed **there**, then the submodule pointer is bumped **here** (T108). Nothing under `modules/` is visible to this repo's CI until that pointer moves — which is why the coverage-table row (T107) must land in the pointer-bump commit.
- **Never set the deprecated `Service.spec.loadBalancerIP`** (deprecated since K8s 1.24). Pool/address selection is expressed only through annotations and the Gameplane label.
- **Wake-on-connect is out of scope for v1**; the Nuclear Option module declares no wake-on-connect protocol.
- **The remote-command port (TCP 7779) has no authentication.** It must never be advertised in the Service, never exposed outside the pod, and is reached only by the agent sidecar over pod-local loopback.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an incomplete task)
- **[Story]**: `US1`–`US6`. Setup, Foundational and Polish tasks carry **no** story label.
- Every task names an exact file path. Paths that do not exist yet are marked `(NEW)`.

## Path Conventions

- Operator (Go): `operator/api/v1alpha1/`, `operator/internal/controller/`, `operator/cmd/`
- Agent (Go): `agent/internal/rcon/`, `agent/cmd/`
- Dashboard (TS/React): `web/src/routes/`, `web/src/lib/`, `web/src/types.ts`
- E2E: `test/e2e/`
- Game module (submodule `gameplane-module`): `modules/nuclear-option/`
- Design source: `design.pen` (Pencil MCP only) with exports in `design-export/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Make both tracks workable in the checkout before any code is written. Track B has no load-balancer provider today — `metallb` appears nowhere in `deploy/`, `charts/` or `test/`, and kind ships no LoadBalancer implementation — so installing one is Setup work, not an assumption.

- [ ] T001 Initialize the `modules/` git submodule (`git submodule update --init`) and confirm the pinned `gameplane-module` commit resolves, per /home/valgul/project/kubernetes-game-dashboard/.gitmodules
- [ ] T002 Create the module directory skeleton /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/ (NEW) inside the `gameplane-module` submodule checkout (requires T001 — the directory does not exist until the submodule is populated)
- [ ] T003 [P] Install MetalLB (controller + speaker, pinned manifest) and define the two test address pools `pool-us-east` and `pool-us-west` as `IPAddressPool` + `L2Advertisement` CRs, with ranges carved from the kind docker bridge subnet (e.g. `172.18.255.100-172.18.255.110` and `172.18.255.200-172.18.255.210`) and never from TEST-NET-1 `192.0.2.0/24`, which is unroutable from kind nodes, in the CI e2e bootstrap /home/valgul/project/kubernetes-game-dashboard/deploy/kind/e2e.sh
- [ ] T004 [P] Install MetalLB and define the same two pools in the developer bootstrap /home/valgul/project/kubernetes-game-dashboard/deploy/kind/up.sh
- [ ] T005 [P] Register the three Track B design nodes (`f1Vga`, `J5pjJ3`, `EZFW0`) as in-scope for this feature in /home/valgul/project/kubernetes-game-dashboard/design-export/MANIFEST.md

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented. Track B foundational work (T006–T015) unblocks US2/US4/US6; Track A gates and foundations (T016–T036) unblock US1/US3/US5.

**⚠️ CRITICAL**: No user story work can begin until the relevant half of this phase is complete. Track A's gates T018, T019 and T022 block **all** Track A stories.

### Track B foundation — CRD, flavor selection, Service translation

- [ ] T006 Add optional `addressPool` (MaxLength 63, DNS-1123 subdomain pattern) and `address` (MaxLength 45, **no CEL regex** — this repo has had CRDs rejected on CEL cost) to the `GameServerNetworking` struct (~lines 226–278), add the reconciler-owned `pool` field to `GameServerEndpoint` (~lines 494–511), and declare the `AddressAssignment` condition type in /home/valgul/project/kubernetes-game-dashboard/operator/api/v1alpha1/gameserver_types.go
- [ ] T007 Regenerate the CRD artifacts (`make generate && make manifests`, run on CI or the sanctioned remote build host — never locally) and commit /home/valgul/project/kubernetes-game-dashboard/operator/api/v1alpha1/zz_generated.deepcopy.go, /home/valgul/project/kubernetes-game-dashboard/operator/config/crd/*.yaml, /home/valgul/project/kubernetes-game-dashboard/charts/gameplane/crds/*.yaml and /home/valgul/project/kubernetes-game-dashboard/charts/gameplane/crd-manifests/*.yaml (the copy the pre-upgrade apply hook ships — omitting it means existing clusters never get the new fields) in the SAME commit as T006, per project rule 7
- [ ] T008 Add the explicit cluster address-manager flavor setting (`metallb` | `cilium` | `none`, defaulting to `none`) as an operator flag/env in /home/valgul/project/kubernetes-game-dashboard/operator/cmd/main.go
- [ ] T009 [P] Expose the address-manager flavor as a Helm value in /home/valgul/project/kubernetes-game-dashboard/charts/gameplane/values.yaml
- [ ] T010 Wire the address-manager flavor into the operator Deployment's container args/env in /home/valgul/project/kubernetes-game-dashboard/charts/gameplane/templates/operator.yaml (the operator Deployment and its ClusterRole live here; there is no `templates/deployment.yaml` in this chart)
- [ ] T011 Set the flavor to `metallb` on the chart install so the kind clusters actually translate pool requests instead of silently no-opping, in /home/valgul/project/kubernetes-game-dashboard/deploy/kind/e2e.sh and /home/valgul/project/kubernetes-game-dashboard/deploy/kind/up.sh
- [ ] T012 Extend the managed-key mechanism (currently `gameplane.local/managed-service-annotations`, annotations only, ~lines 474–540) to also track managed **labels**, so the Cilium `gameplane.local/lb-pool` label is removed cleanly when the pool preference is unset, in /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_controller.go
- [ ] T013 Implement the MetalLB branch of pool/address translation in `reconcileService()` (~lines 433–490) — annotations `metallb.io/address-pool` and `metallb.io/loadBalancerIPs`, registered through the managed-key mechanism, never writing the deprecated `spec.loadBalancerIP` — in /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_controller.go
- [ ] T014 Implement the Cilium branch of pool/address translation in `reconcileService()` — label `gameplane.local/lb-pool` (a Gameplane convention the cluster admin must mirror in `CiliumLoadBalancerIPPool.spec.serviceSelector`; Cilium does not recognise it natively) plus annotation `lbipam.cilium.io/ips`, both registered through the managed-key mechanism — in /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_controller.go
- [ ] T015 Implement the remaining translation branches in `reconcileService()` — flavor `none`: mutate nothing but record that a pool/address was requested with no address manager configured (feeding the `NoAddressManagerConfigured` reason in T083, so the request never silently falls back to the default pool); non-LoadBalancer expose modes: ignore and flag — in /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_controller.go

### Track A gates — unverified claims and the open decisions

- [ ] T016 [P] Verify Claim 1 (dedicated-server availability): Steam app 3930080 downloads via SteamCMD with `+login anonymous` without owning base game 2168680, and ships an executable native Linux `NuclearOptionServer.x86_64`; record the result (or the licensing blocker) in /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/specs.md (NEW)
- [ ] T017 Verify Claim 2 (network ports): confirm the running server listens on UDP 7777 (game, advertised), UDP 7778 (query) and TCP 7779 (remote-command, opt-in via the `-ServerRemoteCommands` launch flag, never advertised) and append the observed ports to /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/specs.md
- [ ] T018 [P] **GATE** — Verify Claim 3a (remote-command wire framing) against a real server: request = 4-byte little-endian length counting ONLY the UTF-8 JSON body bytes + that JSON; response is ASYMMETRIC = 4-byte status code + 4-byte body length (0 when absent) + body; status codes 2000 Success, 4000–4005 client errors, 5000–5002 server errors — recording the confirmed framing in /home/valgul/project/kubernetes-game-dashboard/specs/002-nuclear-option-ip-pool/contracts/nuclear-option-remote-command.md
- [ ] T019 **GATE** — Verify Claim 3b: capture the **per-command JSON response body shape** for every command this feature implements (`get-player-list`, `kick-player`, `banlist-add`, `banlist-remove`, `send-chat-message`, `set-next-mission`) and append them to /home/valgul/project/kubernetes-game-dashboard/specs/002-nuclear-option-ip-pool/contracts/nuclear-option-remote-command.md
- [ ] T020 Verify Claim 4 (readiness signal): confirm the line `[DedicatedServerManager] Waiting for Players before loading next map` appears in `./logs/server-<timestamp>.log` when the server starts accepting players, and append it to /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/specs.md
- [ ] T021 Verify Claim 5 (on-disk log location and format): confirm the pod-accessible log path and format and append it to /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/specs.md
- [ ] T022 **GATE — OPEN DECISION, DO NOT SILENTLY PICK ONE.** The dedicated server's player list returns only `steamId` and `faction`; upstream removed `displayName` because the server runs headless, yet FR-007 and US3 Acceptance Scenario 1 promise a display name. Write up both options — **(A)** hydrate names via a Steam Web API lookup routed through this repo's `netguard` SSRF dial-guard (new outbound dependency, needs its own security review), or **(B)** amend the spec to drop `displayName` from the player list — with costs and consequences, surface the choice to the maintainer, and record the outcome in /home/valgul/project/kubernetes-game-dashboard/specs/002-nuclear-option-ip-pool/spec.md
- [ ] T023 **DECISION** — The players capability is parsed by regex over console text (`PlayerList.EntryRegex` in /home/valgul/project/kubernetes-game-dashboard/agent/internal/caps/caps.go, ~lines 96–123), which is a poor fit for this protocol's JSON response body; the Players tab, `status.agent.playersOnline` and idle auto-sleep all flow through it. Write up the options (regex over the rendered JSON, a protocol-aware parser, or declaring no players capability for v1) with consequences and record the outcome in /home/valgul/project/kubernetes-game-dashboard/specs/002-nuclear-option-ip-pool/plan.md
- [ ] T024 **DECISION** — The operator mints and mounts a per-GameServer RCON password Secret (`reconcileRCONSecret`, /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_rcon.go ~line 73) for every template declaring an RCON protocol, but the Nuclear Option remote-command port has no authentication; decide whether `nuclearoption` is exempted or a dead Secret per server is accepted, recording the outcome in /home/valgul/project/kubernetes-game-dashboard/specs/002-nuclear-option-ip-pool/plan.md

### Track A foundation — module bundle, agent protocol, protocol registration

- [ ] T025 Author the module manifest (name, version, maintainer, game metadata, no wake-on-connect protocol) in /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/module.yaml (NEW)
- [ ] T026 Author the base server template — resources (2–4 CPU, 8–16 GB RAM, 30 GB storage) and ports (UDP 7777 game advertised, UDP 7778 query, TCP 7779 remote-command pod-local only, never advertised) — in /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/template.yaml (NEW)
- [ ] T027 [P] Write the operator-facing module overview in /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/README.md (NEW)
- [ ] T028 Write the FR-027 module spec — setup, configuration options, remote-console command syntax, resource usage, known limitations, ports and log locations, incorporating the verified findings recorded by T016, T017, T020 and T021 — in /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/specs.md
- [ ] T029 [P] Add the 256×256 module icon, recording its source and licence terms (the game's art is commercial; use only assets cleared for redistribution) alongside it, at /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/icon.png (NEW)
- [ ] T030 Implement the `nuclearoption` console protocol transport — connect over pod-local loopback, 4-byte LE length-prefixed JSON request, asymmetric status+length+body response decode, status-code mapping, bounded body length, errors wrapped with `%w` — in /home/valgul/project/kubernetes-game-dashboard/agent/internal/rcon/nuclearoption.go (NEW)
- [ ] T031 Register the protocol identifier exactly `"nuclearoption"` (one lowercase word) in the protocol dispatch switch (~lines 126–145, alongside `case strings.EqualFold(rconProtocol, "palworld")`) and in the `--rcon-protocol` flag help text (~lines 68–73) in /home/valgul/project/kubernetes-game-dashboard/agent/cmd/main.go — note `agent/internal/rcon/rcon.go` holds only the Source-protocol client and has no allowlist
- [ ] T032 Add `nuclearoption` to the RCON protocol enum marker `+kubebuilder:validation:Enum=source;telnet;websocket;battleye;satisfactory;palworld;none` (~line 992) in /home/valgul/project/kubernetes-game-dashboard/operator/api/v1alpha1/gametemplate_types.go — without this the apiserver rejects the module's `template.yaml` outright
- [ ] T033 Regenerate and commit the CRD artifacts for T032 — /home/valgul/project/kubernetes-game-dashboard/operator/api/v1alpha1/zz_generated.deepcopy.go, /home/valgul/project/kubernetes-game-dashboard/operator/config/crd/*.yaml, /home/valgul/project/kubernetes-game-dashboard/charts/gameplane/crds/*.yaml and /home/valgul/project/kubernetes-game-dashboard/charts/gameplane/crd-manifests/*.yaml — in the SAME commit as T032, per project rule 7
- [ ] T034 Add `"nuclearoption"` to the `RCON_PROTOCOLS` allowlist (~line 129) in the `gameplane-module` repo's validate.py, reached here at /home/valgul/project/kubernetes-game-dashboard/modules/validate.py
- [ ] T035 Add `nuclearoption` to the enumerated `rcon.protocol` values (~line 701) in /home/valgul/project/kubernetes-game-dashboard/docs/module-authoring.md
- [ ] T036 Configure the readiness probe to match the startup log line verified in T020, in /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/template.yaml

**Checkpoint**: Track B foundation (T006–T015) ready → US2 can start. Track A gates (T016–T024) resolved and foundation (T025–T036) landed → US1 can start; US3 and US5 additionally need the US1 image (see their phase headers).

---

## Phase 3: User Story 2 - Operator pins a game server's public address to a chosen load-balancer address pool (Priority: P1) 🎯 MVP

**Goal**: An operator can name an address pool when creating or editing a LoadBalancer-exposed game server, and the server reliably receives an address from that pool, shown in the dashboard.

**Independent Test**: On a kind cluster with the MetalLB install from T003/T004 and the flavor set to `metallb` (T011), create a LoadBalancer game server with `addressPool: pool-us-west`, and confirm the assigned address falls inside the `pool-us-west` range and is displayed with its pool name on the Server Detail overview within 30 seconds — with no Track A code present.

**Depends on**: T003/T004 (MetalLB), T006–T015 (Track B foundation). No Track A dependency of any kind.

### Design pass (Constitution Principle II — blocks all React work in this phase)

- [ ] T037 [US2] Via the Pencil MCP server only (never hand-edit), add pool-preference and explicit-address fields to `f1Vga` (Create Server — Step 4 Network), a pool/address display + edit section **and its failed-assignment error state** to `J5pjJ3` (Server Detail — Settings · Networking), and the assigned address + pool name to `EZFW0` (Server Detail — Overview) in /home/valgul/project/kubernetes-game-dashboard/design.pen — then **explicitly ask the user to press Ctrl/Cmd-S**, because Pencil does not auto-save
- [ ] T038 [US2] In one Pencil MCP session after the save (never two concurrent agents against the same in-memory document), re-export the three touched nodes as JSON to /home/valgul/project/kubernetes-game-dashboard/design-export/json/f1Vga.json, /home/valgul/project/kubernetes-game-dashboard/design-export/json/J5pjJ3.json and /home/valgul/project/kubernetes-game-dashboard/design-export/json/EZFW0.json and as screenshots to /home/valgul/project/kubernetes-game-dashboard/design-export/screenshots/f1Vga.png, /home/valgul/project/kubernetes-game-dashboard/design-export/screenshots/J5pjJ3.png and /home/valgul/project/kubernetes-game-dashboard/design-export/screenshots/EZFW0.png, in the same change as T037

### Operator status surface

- [ ] T039 [US2] Extend `endpointsFromService()` (~lines 616–644) to populate `GameServerEndpoint.pool` from the requested pool (and from the address manager's own metadata only where it actually exposes one — MetalLB does not, so the requested pool is the honest source) alongside the assigned address, in /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_status.go
- [ ] T040 [US2] Add the `AddressAssignment` condition to `computeConditions()` (~lines 211–300) with the success path `Assigned` (`Address 172.18.255.203 assigned from pool 'pool-us-west'`) and the transient reasons `AssignmentPending` / `ServiceNotReady`, in /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_status.go

### Dashboard

- [ ] T041 [P] [US2] Mirror the CRD change with `addressPool?: string` and `address?: string` on the `GameServerNetworking` interface (~lines 394–401) **and `pool?: string` on the `GameServerEndpoint` interface (~lines 437–445)** — without the second, T045 does not compile under `strict` — in /home/valgul/project/kubernetes-game-dashboard/web/src/types.ts
- [ ] T042 [P] [US2] Widen the file-local `networking` object type in `ServerCreate` (~line 76 — this file carries its own inline type, independent of `web/src/types.ts`) and include `networking.addressPool` and `networking.address` in the create payload when set (omitted when empty, for backward compatibility) in /home/valgul/project/kubernetes-game-dashboard/web/src/lib/endpoints.ts
- [ ] T043 [US2] Add the optional pool-preference and explicit-address inputs to the Network step (~lines 787–812), including the "ignored unless exposure mode is LoadBalancer" affordance, in /home/valgul/project/kubernetes-game-dashboard/web/src/routes/CreateServer.tsx
- [ ] T044 [US2] Add the pool/address display and edit section (current assignment, pool preference field, explicit address field, exposure-mode warning) and render the `AddressAssignment` condition message — including a router link to the conflicting server when the reason is `AddressInUse` — **and extend the `setNet()` `cleaned` allowlist (~lines 45–64) to carry `addressPool` and `address`, or every unrelated settings save silently wipes them**, in /home/valgul/project/kubernetes-game-dashboard/web/src/routes/tabs/settings/Networking.tsx
- [ ] T045 [US2] Render the assigned address with its pool name in the endpoint list (~lines 83–205), falling back to the bare address when no pool is set, in /home/valgul/project/kubernetes-game-dashboard/web/src/routes/tabs/Overview.tsx

### Tests

- [ ] T046 [P] [US2] Cover the Network-step pool/address fields and the emitted payload in /home/valgul/project/kubernetes-game-dashboard/web/src/routes/CreateServer.test.tsx
- [ ] T047 [P] [US2] Cover the Networking settings display/edit round-trip, the exposure-mode warning, and a save of an unrelated field preserving `addressPool`/`address`, in /home/valgul/project/kubernetes-game-dashboard/web/src/routes/tabs/settings/Networking.test.tsx
- [ ] T048 [P] [US2] Cover endpoint rendering with and without a pool name in /home/valgul/project/kubernetes-game-dashboard/web/src/routes/tabs/Overview.test.tsx
- [ ] T049 [US2] Add envtest cases for CRD validation of both fields, MetalLB annotation translation, Cilium label + annotation translation, the `none` flavor no-op, managed-key cleanup when the pool is unset, changing `addressPool` on an existing server mutating the Service in place (US2 Acceptance Scenario 2), and unchanged behaviour for servers with no preference, in /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_envtest_test.go
- [ ] T050 [US2] Add unit tests for endpoint pool extraction and the `Assigned` / `AssignmentPending` / `ServiceNotReady` conditions in /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_status_test.go

**Checkpoint**: US2 is unit- and envtest-complete and independently shippable; its end-to-end proof is T096/T097 in Phase 9, which ship with it. **This is the MVP — stop here, validate, and ship Track B before starting Track A.**

---

## Phase 4: User Story 1 - Operator deploys a playable Nuclear Option server from the catalog (Priority: P1)

**Goal**: An operator finds Nuclear Option in the module catalog, fills in the configuration form, deploys, and the server becomes joinable within five minutes.

**Independent Test**: With the module bundle pushed to the registry, create a Nuclear Option GameServer from the dashboard catalog with default settings, and join it from a real game client on UDP 7777 within five minutes (SC-001) — no remote-console commands needed.

**Depends on**: T016 (availability gate), T025–T036 (module bundle + protocol registration).

- [ ] T051 [US1] Define the `configSchema` fields — `SERVER_NAME` (string), `MAX_PLAYERS` (int), `SERVER_PASSWORD` (password), `MISSION_ROTATION` (enum `sequence`|`random`), `MISSION_LIST` (comma-separated), each marked required/optional per FR-003 — in /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/template.yaml
- [ ] T052 [US1] Render `DedicatedServerConfig.json` through `configFiles` with port overrides (7777/7778/7779, `IsOverride: true`) and the configSchema values, using the structure confirmed in T018/T019, in /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/template.yaml
- [ ] T053 [US1] Declare the persisted on-disk paths (server config, ban list, mission files and logs) on the PVC mount so the stock backup framework captures them (FR-013) — if the SteamCMD install writes any of them outside the mount, move them — in /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/template.yaml
- [ ] T054 [US1] **DECISION** — This repo has no module image-build convention: every module pins a third-party image by digest (e.g. `modules/palworld/template.yaml`) and `modules/build.sh` is an `oras push` bundle wrapper with no `docker build`. Choose **(A)** pin an existing published Nuclear Option dedicated-server image by digest, or **(B)** build and publish our own image from a new CI workflow, and record the choice with its consequences in /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/specs.md
- [ ] T055 [US1] If T054 chose (B), implement the container entrypoint — SteamCMD `+login anonymous +app_update 3930080`, launch `NuclearOptionServer.x86_64` with the remote-command flag bound pod-local, stream server logs to stdout — in /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/Dockerfile (NEW)
- [ ] T056 [US1] If T054 chose (B), add the image build-and-publish workflow (buildx, push to GHCR, emit the digest; never a local `docker build`) in /home/valgul/project/kubernetes-game-dashboard/.github/workflows/module-image-nuclear-option.yaml (NEW)
- [ ] T057 [US1] Point `spec.image` at the resolved digest (never a floating tag) in /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/template.yaml
- [ ] T058 [US1] Author the real protocol-level join test `TestGameServer_NuclearOptionBot_Joined` — hand-rolled UDP 7777 join client, `t.Parallel()`, per-test unique resource names, and proof the probe **fails against a dead address AND passes against a real listener** — in /home/valgul/project/kubernetes-game-dashboard/test/e2e/nuclearoption_bot_e2e_test.go (NEW)
- [ ] T059 [US1] Bucket `TestGameServer_NuclearOptionBot_Joined` under `bot-heavy` — bucketed for the coverage audit, never executed by default CI because of its 8–16 GB footprint, still runnable on demand via `make test-e2e-bucket BUCKET=bot-heavy` — in the SAME commit as T058, because `buckets.sh verify` fails both on an unbucketed suite test and on a bucketed test that no longer exists, in /home/valgul/project/kubernetes-game-dashboard/test/e2e/buckets.sh
- [ ] T060 [US1] Assert catalog-to-joinable in under five minutes (module visible, config form renders every schema field, GameServer reaches Ready, join succeeds) — bucketing any new top-level `func Test…` this adds, per T059 — in /home/valgul/project/kubernetes-game-dashboard/test/e2e/nuclearoption_bot_e2e_test.go

**Checkpoint**: A default Nuclear Option server deploys and is playable.

---

## Phase 5: User Story 4 - Operator requests one specific fixed address so a published DNS record stays stable (Priority: P2)

**Goal**: An operator can pin one exact address to a server, and a request for an address already in use is rejected with a message naming the conflicting server rather than silently reassigning.

**Independent Test**: Create server A with an explicit address from `pool-us-east`, confirm the Service receives exactly that address; then create server B requesting the same address and confirm B reports `AddressAssignment=False / AddressInUse` naming server A within 30 seconds, while A keeps its address.

**Depends on**: T006–T015 and the `AddressAssignment` condition from T040 (Phase 3) — US4 is **not** independent of US2 and cannot land before it. The dashboard surface it needs is T044.

- [ ] T061 [US4] Detect conflicts for an explicitly requested `spec.networking.address` by listing LoadBalancer Services cluster-wide (the operator already holds cluster-scoped `services` `get;list;watch`, so this needs no RBAC change) before applying the annotation, keeping re-requests by the same server idempotent, in /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_controller.go
- [ ] T062 [US4] Emit the `AddressInUse` reason on the `AddressAssignment` condition with a message naming the conflicting server in /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_status.go
- [ ] T063 [US4] Add an envtest case where two GameServers request the same address and only the first is granted it in /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_envtest_test.go
- [ ] T064 [US4] Add an envtest case proving an address released when its GameServer is deleted becomes assignable to another server (US4 Acceptance Scenario 3) in /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_envtest_test.go
- [ ] T065 [P] [US4] Cover the explicit-address field and the rendered `AddressInUse` message with its link to the conflicting server in /home/valgul/project/kubernetes-game-dashboard/web/src/routes/tabs/settings/Networking_more.test.tsx

**Checkpoint**: Fixed-address requests work and conflicts are explicit.

---

## Phase 6: User Story 3 - Operator administers a running Nuclear Option match remotely (Priority: P2)

**Goal**: An operator can list players, kick, ban/unban, broadcast chat and set the next mission from the existing Remote Console, each command returning within five seconds (SC-004).

**Independent Test**: With a running Nuclear Option server and one connected player, run each moderation command from the Remote Console and confirm the effect on the server (player disconnects, message appears in game chat, mission rotates) within five seconds — no new dashboard screens involved.

**Depends on**: T018/T019 (framing and per-command body shapes), T022 (display-name decision), T023 (players capability decision), T030/T031 (transport + registration), **and the whole of Phase 4 (US1)** — the independent test needs a running server, which needs the image from T054–T057. US3 is not independent of US1.

**Serialization warning**: T066, T068, T070, T071, T073 and T074 all edit the same file (`nuclearoption.go`) and T067, T069, T072, T075, T076, T077 and T078 all edit the same test file. They are **serial**, not parallel.

- [ ] T066 [US3] Implement `get-player-list` (request `{"name":"get-player-list","arguments":[]}`, decode the status-2000 body into steamId/faction entries) in /home/valgul/project/kubernetes-game-dashboard/agent/internal/rcon/nuclearoption.go
- [ ] T067 [US3] Test `get-player-list` across populated list, empty list, 4003 JsonError and 5000 InternalServerError in /home/valgul/project/kubernetes-game-dashboard/agent/internal/rcon/nuclearoption_test.go (NEW)
- [ ] T068 [US3] Implement `kick-player` (status 2000 success, 4005 BadArguments when the Steam ID is unknown) per FR-008 in /home/valgul/project/kubernetes-game-dashboard/agent/internal/rcon/nuclearoption.go
- [ ] T069 [US3] Test `kick-player` with a valid ID, an unknown ID and an oversized ID in /home/valgul/project/kubernetes-game-dashboard/agent/internal/rcon/nuclearoption_test.go
- [ ] T070 [US3] Implement `banlist-add` (status 2000 success, 5002 ConfigError when the ban-list file is missing) per FR-009 in /home/valgul/project/kubernetes-game-dashboard/agent/internal/rcon/nuclearoption.go
- [ ] T071 [US3] Implement `banlist-remove` (status 2000 success, 4005 BadArguments when the ID is not banned) per FR-010 in /home/valgul/project/kubernetes-game-dashboard/agent/internal/rcon/nuclearoption.go
- [ ] T072 [US3] Test `banlist-add` and `banlist-remove` across success, unknown ID and missing-config paths in /home/valgul/project/kubernetes-game-dashboard/agent/internal/rcon/nuclearoption_test.go
- [ ] T073 [US3] Implement `send-chat-message` (Rich Text tags preserved, 4005 BadArguments when too long) per FR-011 in /home/valgul/project/kubernetes-game-dashboard/agent/internal/rcon/nuclearoption.go
- [ ] T074 [US3] Implement `set-next-mission` (group/name/maxtime arguments, 4005 BadArguments when the mission is not in rotation) and, for FR-012's second half, either wire the command that adjusts the **currently running** mission's remaining time or record in /home/valgul/project/kubernetes-game-dashboard/specs/002-nuclear-option-ip-pool/contracts/nuclear-option-remote-command.md that the protocol exposes none, in /home/valgul/project/kubernetes-game-dashboard/agent/internal/rcon/nuclearoption.go
- [ ] T075 [US3] Test `send-chat-message` across success, empty message and over-long message in /home/valgul/project/kubernetes-game-dashboard/agent/internal/rcon/nuclearoption_test.go
- [ ] T076 [US3] Test `set-next-mission` across a valid mission, an unknown mission and a read timeout in /home/valgul/project/kubernetes-game-dashboard/agent/internal/rcon/nuclearoption_test.go
- [ ] T077 [US3] Add the high-visibility table-driven framing test proving the response is **not** a mirror of the request — request `len(body)+body`, response `status+len(body)+body`, little-endian throughout, length counting only UTF-8 body bytes, oversized and truncated lengths rejected, and a `net.Pipe` case delivering the status word, the length word and the body in separate one-byte segments — in /home/valgul/project/kubernetes-game-dashboard/agent/internal/rcon/nuclearoption_test.go
- [ ] T078 [US3] Add the explicit error-path cases — unknown status code, a frame shorter than the 8-byte status+length header, `bodyLen == 0` paired with a non-2000 status, and a JSON decode failure — so the agent module stays at its 90% gate, in /home/valgul/project/kubernetes-game-dashboard/agent/internal/rcon/nuclearoption_test.go
- [ ] T079 [US3] Declare the moderation capability actions (player-list per the T023 decision, kick-player with confirm+danger, banlist-add, banlist-remove, send-chat-message, set-next-mission) so they route through the existing Remote Console with no new dashboard screens, in /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/template.yaml
- [ ] T080 [US3] Add the SC-004 five-second command-result assertion for the moderation commands — bucketing any new top-level `func Test…` this adds, per T059 — in /home/valgul/project/kubernetes-game-dashboard/test/e2e/nuclearoption_bot_e2e_test.go
- [ ] T081 [US3] Implement whichever branch of the T022 decision the maintainer chose — either Steam Web API display-name hydration dialled through `netguard`'s strict `IsPublic` policy in /home/valgul/project/kubernetes-game-dashboard/agent/internal/rcon/nuclearoption.go, or the FR-007 / US3 Acceptance Scenario 1 amendment in /home/valgul/project/kubernetes-game-dashboard/specs/002-nuclear-option-ip-pool/spec.md

**Checkpoint**: Remote administration works end to end.

---

## Phase 7: User Story 6 - Operator gets a clear status message when address pool assignment fails (Priority: P3)

**Goal**: Every pool-assignment failure mode surfaces a specific, actionable message in the dashboard within 30 seconds instead of an indefinite "Pending" (SC-003).

**Independent Test**: Create a LoadBalancer server naming a pool that does not exist and confirm the dashboard shows `Pool 'xyz' not found in cluster` within 30 seconds; repeat with an exhausted pool, with a non-LoadBalancer exposure mode, and with the address-manager flavor set to `none`.

**Depends on**: T040 (condition type) and T044 (the Networking panel that renders it) — both in Phase 3. US6 is **not** independent of US2.

- [ ] T082 [US6] Grant the operator read access to Service events — add `+kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch` beside the existing `create;patch` marker (~line 172) in /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_controller.go, commit the regenerated /home/valgul/project/kubernetes-game-dashboard/operator/config/rbac/role.yaml, and hand-edit the matching ClusterRole rule (~lines 64–66, the chart RBAC is not generated from the markers) in /home/valgul/project/kubernetes-game-dashboard/charts/gameplane/templates/operator.yaml
- [ ] T083 [US6] Derive `PoolNotFound`, `PoolExhausted` and `ManagerFailure` from MetalLB's `Warning`-type Service events and Cilium's Service status conditions, add `NoAddressManagerConfigured` for a pool/address requested while the flavor is `none` (SC-002 forbids the silent default-pool fallback), and fall back to `ServiceNotReady` when 30 seconds have elapsed since the `AddressAssignment` condition's `lastTransitionTime` — naming the requeue interval that guarantees the re-evaluation — in /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_status.go
- [ ] T084 [US6] Emit the informational `IgnoredForExposureMode` reason when a pool or address is set while `expose != LoadBalancer` in /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_controller.go
- [ ] T085 [P] [US6] Add unit tests for all five failure reasons (`PoolNotFound`, `PoolExhausted`, `ManagerFailure`, `NoAddressManagerConfigured`, `IgnoredForExposureMode`) and their message text in /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_status_test.go
- [ ] T086 [P] [US6] Cover the failing `AddressAssignment` condition messages rendered by the Networking settings panel in /home/valgul/project/kubernetes-game-dashboard/web/src/routes/tabs/settings/Networking.test.tsx

**Checkpoint**: Pool failures are diagnosable from the dashboard alone.

---

## Phase 8: User Story 5 - Operator edits Nuclear Option server settings and invalid configuration surfaces as a clear error (Priority: P3)

**Goal**: Invalid Nuclear Option configuration is rejected at save time with a message naming the offending field, never as a boot crash-loop (SC-007).

**Independent Test**: Save a Nuclear Option server with `MAX_PLAYERS: 100` and confirm a condition appears within 10 seconds reading like `MAX_PLAYERS must be between 4 and 64`, with no pod ever entering CrashLoopBackOff.

**Depends on**: T051 (configSchema) and the whole of Phase 4 (US1) — the e2e assertion needs a running server. **`ConfigField` has no constraint keys today** (`Name, DisplayName, Description, Type, Default, Enum, Required, Target, AutoFromMemoryLimit` only), and `materializeConfig` checks only int/bool parseability, enum membership and required-ness — so FR-023 needs the mechanism built first, in T087–T092.

- [ ] T087 [US5] Add the numeric and length constraint keys (`min`, `max`, `minLength`, `maxLength`) to the `ConfigField` struct (~lines 1051–1102) in /home/valgul/project/kubernetes-game-dashboard/operator/api/v1alpha1/gametemplate_types.go
- [ ] T088 [US5] Regenerate and commit the CRD artifacts for T087 — /home/valgul/project/kubernetes-game-dashboard/operator/api/v1alpha1/zz_generated.deepcopy.go, /home/valgul/project/kubernetes-game-dashboard/operator/config/crd/*.yaml, /home/valgul/project/kubernetes-game-dashboard/charts/gameplane/crds/*.yaml and /home/valgul/project/kubernetes-game-dashboard/charts/gameplane/crd-manifests/*.yaml — in the SAME commit as T087, per project rule 7
- [ ] T089 [US5] Enforce the new constraints in `materializeConfig()` (~lines 86–175), rejecting with a message that names the field and the constraint, in /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_config.go
- [ ] T090 [US5] Add validation unit tests asserting each rejection carries a message naming the field and the constraint, and that templates declaring no constraints behave exactly as before, in /home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_config_test.go
- [ ] T091 [US5] Mirror the constraint keys in the wizard's client-side field validation so the operator sees the error before saving, in /home/valgul/project/kubernetes-game-dashboard/web/src/routes/CreateServer.tsx
- [ ] T092 [P] [US5] Cover the wizard-side constraint rejections in /home/valgul/project/kubernetes-game-dashboard/web/src/routes/CreateServer.test.tsx
- [ ] T093 [US5] Declare the field constraints (`SERVER_NAME` 1–64 chars, `MAX_PLAYERS` 4–64, `MISSION_ROTATION` enum, `MISSION_LIST` entries checked against the available missions) using the new keys in the configSchema in /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/template.yaml
- [ ] T094 [US5] Assert that an invalid configuration surfaces on `status.conditions` within 10 seconds without a crash-loop — bucketing any new top-level `func Test…` this adds, per T059 — in /home/valgul/project/kubernetes-game-dashboard/test/e2e/nuclearoption_bot_e2e_test.go
- [ ] T095 [US5] Reconcile the two US5 acceptance scenarios that no mechanism covers — Scenario 1's "Configuration Changed — Restart Required" status and Scenario 4's "Invalid JSON in [field]" (there is no `json` config-field type) — by either tasking them explicitly or amending them, recording the outcome in /home/valgul/project/kubernetes-game-dashboard/specs/002-nuclear-option-ip-pool/spec.md

**Checkpoint**: All six user stories are functional.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Cross-story e2e coverage, documentation, real-cluster validation, and the gates that must be green before either track ships.

- [ ] T096 Author the Track B e2e suite — default pool for a server with no preference (SC-006), assignment from a named pool (SC-002), nonexistent-pool error within 30s (SC-003), exhausted pool, address in use, wrong exposure mode, address visible in status and dashboard within 30s (SC-008), and a GameServer created through the REST API carrying `networking.addressPool` receiving the same assignment and the same audit trail (FR-021, FR-025) — with `t.Parallel()` and unique per-test names in /home/valgul/project/kubernetes-game-dashboard/test/e2e/pool_assignment_e2e_test.go (NEW)
- [ ] T097 Register every top-level `func Test…` from T096 in the default-executed `operator` bucket, in the SAME commit as T096 (`buckets.sh verify` fails both on unbucketed suite tests and on bucketed tests it cannot find), in /home/valgul/project/kubernetes-game-dashboard/test/e2e/buckets.sh
- [ ] T098 [P] Document address-pool configuration for MetalLB and Cilium (including the `gameplane.local/lb-pool` serviceSelector convention the cluster admin must mirror), setting a pool or fixed address, and troubleshooting each failure reason, in /home/valgul/project/kubernetes-game-dashboard/docs/networking.md (NEW)
- [ ] T099 [P] Document the operator's address-manager flavor Helm value in /home/valgul/project/kubernetes-game-dashboard/docs/install.md
- [ ] T100 Link the new networking guide (created by T098) from /home/valgul/project/kubernetes-game-dashboard/README.md
- [ ] T101 Correct the Cilium translation description (~line 193), which names a bare `pool=<value>` label instead of the `gameplane.local/lb-pool` label the contracts and T014 specify, in /home/valgul/project/kubernetes-game-dashboard/specs/002-nuclear-option-ip-pool/plan.md
- [ ] T102 Reconcile the two Track B requirements the implementation diverges from — FR-022's "no bespoke integration per address manager" (the design ships two hand-written per-manager translations behind an explicit flavor selector) and SC-006's latency-parity claim (no baseline is measured anywhere) — by amending them or tasking the generic mechanism, in /home/valgul/project/kubernetes-game-dashboard/specs/002-nuclear-option-ip-pool/spec.md
- [ ] T103 Run `TestGameServer_NuclearOptionBot_Joined` against a real cluster and record the outcome, updating the join wire format if the handshake differs, in /home/valgul/project/kubernetes-game-dashboard/test/e2e/nuclearoption_bot_e2e_test.go
- [ ] T104 Walk the full moderation command suite against a real server per quickstart.md and record any signature drift in /home/valgul/project/kubernetes-game-dashboard/specs/002-nuclear-option-ip-pool/contracts/nuclear-option-remote-command.md
- [ ] T105 Walk the valid/invalid configuration matrix against a real server per quickstart.md, confirm no crash-loops or hung states, and record the results in /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/specs.md
- [ ] T106 Prove the FR-013 backup/restore round-trip preserves config, ban list and mission state using the stock Gameplane backup framework, recording the verified paths in /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/specs.md
- [ ] T107 Add the Nuclear Option row with status `covered-deferred`, depth `JOINED`, test `TestGameServer_NuclearOptionBot_Joined`, bucket `bot-heavy` and a `lastVerified` date to /home/valgul/project/kubernetes-game-dashboard/docs/game-coverage.md — in the SAME commit as T108, because `joincoverage.sh` fails a module directory with no row (check 2) and a row with no module directory (check 4), and `modules/` only changes here when the pointer moves
- [ ] T108 Bump the `modules/` submodule pointer to the `gameplane-module` commit containing the Nuclear Option bundle, in the `modules` gitlink of this repo recorded against /home/valgul/project/kubernetes-game-dashboard/.gitmodules
- [ ] T109 Satisfy every remaining check of the coverage verifier (`test/e2e/joincoverage.sh verify`) for the Nuclear Option row in /home/valgul/project/kubernetes-game-dashboard/docs/game-coverage.md
- [ ] T110 Final module documentation sync — setup, commands and limitations complete and consistent with the verified findings — in /home/valgul/project/kubernetes-game-dashboard/modules/nuclear-option/specs.md
- [ ] T111 Add the release entry for both tracks in /home/valgul/project/kubernetes-game-dashboard/CHANGELOG.md
- [ ] T112 Exit criterion, not implementation work: confirm on CI that every coverage gate stayed green with no new suppressions — /home/valgul/project/kubernetes-game-dashboard/operator/.testcoverage.yml (72%), /home/valgul/project/kubernetes-game-dashboard/api/.testcoverage.yml (80%), /home/valgul/project/kubernetes-game-dashboard/agent/.testcoverage.yml (90%), /home/valgul/project/kubernetes-game-dashboard/gameaction/.testcoverage.yml (91%) and /home/valgul/project/kubernetes-game-dashboard/web/vitest.config.ts (92/76/82/92)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies — start immediately. T002 requires T001.
- **Foundational (Phase 2)**: depends on Setup. Splits cleanly in two:
  - T006–T015 (Track B) block Phase 3 (US2), Phase 5 (US4) and Phase 7 (US6).
  - T016–T036 (Track A) block Phase 4 (US1), Phase 6 (US3) and Phase 8 (US5). T018, T019 and T022 are hard gates: no Track A story is shippable while any is open, and T016 returning false kills the whole track.
- **Phase 3 (US2, MVP)**: depends on T003/T004 and T006–T015. The only genuinely story-independent phase.
- **Phase 4 (US1)**: depends on T016 and T025–T036.
- **Phase 5 (US4)**: depends on T006–T015, T040 and T044 — i.e. on US2 having landed.
- **Phase 6 (US3)**: depends on T018, T019, T022, T023, T030, T031 **and all of Phase 4** (its independent test needs a running server built by T054–T057).
- **Phase 7 (US6)**: depends on T040 and T044 (US2), and on T082 for the RBAC T083 needs.
- **Phase 8 (US5)**: depends on T051 and all of Phase 4, and on the constraint mechanism T087–T090 which does not exist yet.
- **Phase 9 (Polish)**: T096–T102 and T112 depend on Track B stories; T103–T111 depend on Track A stories. Track B's polish subset can be completed and shipped before Track A starts.

### Task-Level Blocking

- T007 must be in the **same commit** as T006; T033 as T032; T088 as T087 (project rule 7).
- T007 must land **before** T012, T013, T014, T015, T039, T040 and T049 — without the regenerated CRD YAML the apiserver strips the new fields and the envtest fails looking like a reconciler bug.
- T013, T014 and T015 depend on T008 (flavor) and T012 (managed labels); T011 depends on T009/T010.
- T003 and T004 block the US2 independent test, T049's LoadBalancer cases and all of T096.
- T037 **blocks** T038, T043, T044 and T045 — no React work before the design pass is saved and re-exported.
- T041 blocks T043, T044 and T045. T042 does **not** depend on T041: `endpoints.ts` carries its own inline networking type.
- T043–T045 block T046–T048.
- T040 blocks T062, T083 and T086; T044 blocks T065 and T086.
- T036 depends on T020; T052 depends on T018/T019 and T051; T057 depends on T054 (and T055/T056 when branch B is chosen).
- T017, T020, T021 append to the file T016 creates; T028 depends on all four.
- T019 appends to the file T018 creates.
- T051, T052, T053, T057, T079 and T093 all edit `modules/nuclear-option/template.yaml`, which T026 creates — they are serial on that file.
- T066, T068, T070, T071, T073, T074 and T081 are serial on `nuclearoption.go`; T067, T069, T072, T075, T076, T077 and T078 are serial on `nuclearoption_test.go`.
- T080 depends on T030 and on T066–T074; T079 depends on T026 and T066–T074, not on the transport alone.
- T058 depends on T051, T052, T057 (there is nothing to join before the image exists); T059 same commit as T058; T060, T080 and T094 append to T058's file and must bucket any new test function they introduce.
- T082 blocks T083.
- T096 and T097 same commit; T107 and T108 same commit; T109 depends on T059, T107 and T108.
- T108 depends on every `modules/nuclear-option/` file being committed in the `gameplane-module` repo first.
- T112 is last: it observes the state produced by everything else.

### Within Each User Story

- Operator types → codegen → reconciler → status → dashboard → tests.
- Design pass → React → React tests (never the other way round).
- Agent protocol transport → per-command wrappers → per-command tests → framing regression test.

---

## Parallel Opportunities

Concrete examples that touch disjoint files with no incomplete dependency:

**Setup wave** — launch together: T003 (e2e kind MetalLB + pools), T004 (dev kind MetalLB + pools), T005 (design manifest). T002 waits for T001.

**Foundational wave 1** — launch together: T006 (CRD types), T009 (Helm value), T016 (availability claim), T018 (protocol framing gate). T017, T020 and T021 append to the same `specs.md` T016 creates and are therefore serial behind it; T019 appends to the same contract file as T018.

**Foundational wave 2** — launch together: T027 (module README), T029 (icon), T030 (agent protocol transport), T034 (validate.py), T035 (module-authoring docs) — five different files. T028 waits for T016–T021.

**US2 wave 1** — launch together: T037 (design pass, no code dependency), T039 and T040 are serial on `gameserver_status.go`, T041 (web types) and T042 (endpoint helper) are independent files.

**US2 wave 2** — after T037 is saved and T041 has landed: T043, T044 and T045 are three different route files and run in parallel. T038 is a single Pencil session and must not be split across agents.

**US2 wave 3** — launch together: T046, T047, T048 (three separate test files).

**US3** — **no parallel wave exists.** Every command wrapper writes `agent/internal/rcon/nuclearoption.go` and every command test writes `agent/internal/rcon/nuclearoption_test.go`; run T066–T081 serially.

**Polish wave** — launch together: T098 (docs/networking.md) and T099 (docs/install.md). T100 waits for T098.

**Cross-track**: the tracks share three files — `test/e2e/buckets.sh` (T059 vs T097), `charts/gameplane/crds`/`crd-manifests` (T007/T033/T088) and `docs/` — so those tasks must not run concurrently across tracks. Everything else is disjoint.

---

## Implementation Strategy

### MVP First — User Story 2 only

1. Complete Phase 1 (Setup), including the MetalLB install.
2. Complete the Track B half of Phase 2 (T006–T015). Track A's gates can proceed in the background but block nothing here.
3. Complete Phase 3 (US2), design pass first.
4. **STOP and VALIDATE**: run the US2 independent test on a kind cluster with the MetalLB pools and the `metallb` flavor.
5. Complete the Track B polish subset (T096–T102, T112) and ship. Track B is a complete, useful feature with zero Track A code in the tree.

### Incremental Delivery

1. Setup + Track B foundation → foundation ready.
2. **US2 (P1, Track B)** → validate → ship. **MVP.**
3. **US4 (P2, Track B)** → fixed-address pinning → ship.
4. **US6 (P3, Track B)** → failure diagnostics → ship. Track B is now complete.
5. Resolve Track A gates T016–T024. If T016 fails, document the blocker and stop — Track A is dead and Track B is unaffected.
6. **US1 (P1, Track A)** → a playable server → ship.
7. **US3 (P2, Track A)** → remote administration → ship.
8. **US5 (P3, Track A)** → configuration validation → ship.
9. Complete Track A polish (T103–T111) and the coverage sweep (T112).

Each step adds value without breaking the previous one; the two tracks can be released independently and in either order once Track B is out.

### Parallel Team Strategy

- Developer A owns Track B end to end: Phase 1 MetalLB, Phase 2 (T006–T015), Phase 3, Phase 5, Phase 7, and polish T096–T102.
- Developer B owns Track A gates first (T016–T024) — the highest-uncertainty work in the feature — then Phase 2 (T025–T036), Phase 4, Phase 6, Phase 8, and polish T103–T111.
- The two developers must coordinate on `test/e2e/buckets.sh` and the regenerated CRD artifacts; only T112 needs both tracks finished.

---

## Notes

- `[P]` means different files and no incomplete dependency.
- Story labels appear on user-story phases only; Setup, Foundational and Polish tasks carry none.
- Commit per logical unit, signed (`git commit -s`), conventional prefixes; codegen output ships in the commit that triggered it.
- One branch per unit of work, deleted once merged.
- Fix lint findings; never suppress them.
