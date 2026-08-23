# Implementation Plan: Network Capture Sidecar for Game Servers

**Branch**: `003-network-capture-sidecar` | **Date**: 2026-08-23 | **Spec**: [specs/003-network-capture-sidecar/spec.md](/specs/003-network-capture-sidecar/spec.md)

**Input**: Feature specification from `specs/003-network-capture-sidecar/spec.md`; Phase 0 research consolidated in `research.md`; Phase 1 data model, quickstart, and contracts in `data-model.md`, `quickstart.md`, and `contracts/`.

**Status of this document**: Planning artifact only. Nothing described below has been built,
run, or tested. No section may be read as evidence that any behavior works, any test
passes, or any principle is satisfied by anything other than a design commitment for
Phase 2. Where a prior draft of this plan asserted otherwise, it has been corrected below.

## Summary

**Primary Requirement**: Operators need to capture raw game-protocol traffic from real players connecting to instrumented servers for protocol reverse-engineering. Captures must be opt-in per GameServer, admin-only, automatically expire after a retention window, and produce downloadable PCAPNG files readable by standard third-party tools.

**Technical Approach** (from research.md, data-model.md, and contracts/):
- **Sidecar injection (human Decision 1)**: Kubernetes ephemeral containers (K8s 1.28+) add the capture sidecar to a running pod via the `pods/ephemeralcontainers` subresource, without restarting the game container — settled by explicit human decision, not chosen by research.md alone. Kubernetes provides **no API to remove an ephemeral container**; disabling capture stops routing new captures to it immediately, but the container itself lingers in `pod.status.ephemeralContainerStatuses` until the pod is next recreated. This asymmetry is already stated precisely in spec.md's US2 acceptance scenario 4 and in FR-001's disable clause as originally written; research.md confirms no amendment was applied there and none is needed (research.md:13), and corrects an earlier draft that had fabricated a "controlled pod restart" claim and a spec amendment that never existed (see Complexity Tracking below). This is separate from the real FR-001/SC-007 amendment for the pre-provisioned capture volume (research.md:126, Complexity Tracking below).
- **Capture engine**: `gopacket/afpacket` (AF_PACKET with MMap'd TPacket buffers) for live capture, `gopacket/pcapgo.NgWriter` for PCAPNG output. Filter compilation is intended to use `github.com/packetcap/go-pcap`'s `filter` package (`Compile`, confirmed to exist via the GitHub API) — this dependency has **no tagged release**, only a pseudo-version (`v0.0.0-20260731105150-c86974bbfbcd`), and is tracked as an open supply-chain risk, not a settled choice (see Complexity Tracking).
- **Addressing the sidecar (D-ADDRESSING)**: The API/operator dial the capture sidecar's HTTP control endpoint (`:9091`) through the existing `<gs>-agent` ClusterIP Service, not by pod IP. An ephemeral container cannot declare a named `containerPort`, so the constraint is only that the Service must target port 9091 **numerically** — a Service selects pods, not containers, so it can front an ephemeral container's port once that port is reachable on the pod's network namespace. Verified: `agentDNSNames` (`operator/internal/controller/agent_certs.go:207-224`) already issues the agent's mTLS cert with SANs covering `<gs>-agent`, `<gs>-agent.<ns>`, `<gs>-agent.<ns>.svc`, and `<gs>-agent.<ns>.svc.cluster.local`, and carries no IP SAN and no `<pod>.<ns>.svc.cluster.local` form — so dialing the pod IP directly would fail certificate verification, but dialing the existing `<gs>-agent` Service DNS name needs no new certificate and no `ServerName` override. The design is to add a second, numerically-targeted port (9091) to the existing `<gs>-agent` Service (`reconcileAgentService`, `operator/internal/controller/gameserver_controller.go:869-892`) rather than mint a new Service or dial by IP — this is operator work, tracked in the Project Structure file list below.
- **State tracking**: New namespaced **NetworkCapture CRD** owned by GameServer via controller reference; phase-based state machine (Pending → Running → Completed/Failed → Expired), detailed in data-model.md.
- **Storage**: A capture `emptyDir` volume is **pre-provisioned, unconditionally, on every game pod's StatefulSet template** — opted in or not (human Decision 2). This is required, not a convenience: the `pods/ephemeralcontainers` subresource cannot add a volume, and `pod.spec.volumes` is immutable on a running pod, so the volume must already exist before the ephemeral capture container can be injected restart-free. Consequences that follow directly: (a) every existing game server rolls once, on the release that ships this feature, whether or not it ever opts into capture; (b) FR-001 and SC-007's "byte-identical" claim no longer holds literally and is amended in spec.md to "no capture *component* attached" (the empty, unmounted volume is present either way). The emptyDir SHOULD carry a `sizeLimit` as a disk-full backstop behind FR-002's max-size enforcement. Small-file download via the existing agent proxy pattern and a large-file path via a sidecar HTTP endpoint were the research.md/data-model.md proposal; `contracts/rest-api.md` flags the two-path split as **UNVERIFIED** pending a read of `api/internal/ws/` and specifies a single API-facing route regardless of which internal path is used. The capture volume is mounted only on the sidecar, **never on the agent or game container** (data-model.md Entity 3) — a decision driven by `agentVolumeMounts`'s doc comment (`operator/internal/controller/gameserver_rcon.go:105-119`): the agent's file browser is deliberately rooted at exactly one path (`--data-root`) and has no notion of a second root, which is precisely why `spec.storage.extra` volumes are never mounted on the agent today. Adding the capture emptyDir as another agent VolumeMount would hit the same limitation — an unreachable mount `/files/download` cannot serve — so this plan respects that existing constraint rather than overriding it.
- **Retention**: `spec.ttlSecondsAfterFinished`-style field, default **24 hours** (86400 seconds) — a spec-derived constraint, not a human decision: spec.md FR-007 and its Out-of-Scope section fix 24 hours as the default and that is not open for re-discussion; a prior draft drifted to 7 days without authorization and has been reverted. A cluster maximum of 90 days per Helm values is an **UNRATIFIED research.md proposal** — no human has approved it, and it MUST NOT be implemented, cited, or built upon as settled until it is (see Complexity Tracking). Kubernetes does **not** auto-enforce this on custom CRDs, so an operator reconciler must explicitly check and delete expired captures (research.md specifies a 60-second reconciliation interval).
- **Concurrency**: One Running capture per GameServer, enforced via CRD phase serialization at the operator tier and, optionally, an API-tier fail-fast check.
- **Security/RBAC**: `api/internal/rbac/rbac.go` is a first-match-wins path-pattern rule table, not a `RequireRole()` call. A new `captures:manage` permission (added to `api/internal/rbac/catalog.go`, granted to `admin` only via a new append-only migration) MUST be matched by rules inserted **before** the existing `{segment: "servers", perm: "servers:write"}` catch-all at `rbac.go:184` — otherwise every capture route silently falls through to `servers:write`, which the built-in `operator` role already holds (`api/internal/db/migrations/003_roles.sql:48`), defeating FR-005/SC-005 without any test failure or log line. `contracts/rest-api.md` details the exact rule-table entries and makes an operator-role-403 test mandatory, not optional.
- **Audit**: The real `audit.Event` struct (`api/internal/audit/audit.go:727-736`) has no `payload`, `result`, or `operation` column — only `ID, TS, Actor, Method, Path, Target, Status, IP`. Human Decision 3 settles this: a new append-only migration (`api/internal/db/migrations/007_audit_reason.sql`, next after the six files currently in the directory) adds a nullable, structured `reason` column, plus a new exported synchronous write method on `Auditor` that capture handlers call directly (the generic `Middleware`'s query-string-derived `Target` extraction does not work for capture's path-parameter routes, and its post-response write ordering does not satisfy FR-006's "written before the response" requirement). `005_audit_chain.sql` hash-chains every row (`prev_hash`/`hash`, computed in `insertChained`), so this column change touches integrity-sensitive code shared by every audited operation, not just capture — see Complexity Tracking. `contracts/audit-events.md` §2.1 specifies that `Reason` participates in the hash computation for rows written after the migration, and that rows written before it must keep verifying against their original, pre-`reason` hash (the zero-value must not be silently folded into their hashed representation retroactively).
- **Error responses**: No new JSON error envelope — a codebase-derived constraint, not a human decision: verified by reading `api/internal/httperr/httperr.go`: `WriteCode`/`Write` emit **plain text** via `http.Error`, and `rbac.Middleware` writes a literal plain-text string on auth failure before any handler runs. Earlier drafts of `rest-api.md` and `quickstart.md` invented two different, mutually inconsistent JSON envelopes (`{"code","message"}` and `{"error","message"}`); both are wrong and have been removed. All capture endpoint errors go through the same plain-text path every other handler uses.

## Technical Context

**Language/Version**: Go 1.26.0 (verified in `go.work:1` and each module's `go.mod`, e.g. `operator/go.mod:3`), shared across all Go modules via `go.work`.

**Primary Dependencies** (module names verified where cited; version pins for the two new libraries are proposals, not confirmed against a written `go.mod`):
- `github.com/google/gopacket/afpacket` — AF_PACKET live packet capture with MMap'd buffers (proposed, not yet added to any `go.mod`)
- `github.com/google/gopacket/pcapgo` — PCAPNG file writer (`NgWriter`) (proposed, not yet added)
- `github.com/packetcap/go-pcap` (`filter` package, `Compile` function) — pure-Go BPF filter compiler; **no tagged release**, pseudo-version only (open risk, see Complexity Tracking)
- `github.com/go-chi/chi/v5 v5.3.1` — REST API router (verified: `api/go.mod:21`, `agent/go.mod:19` — the prior draft of this plan cited the nonexistent `github.com/chi-middleware/chi`, which has been corrected)
- `sigs.k8s.io/controller-runtime` — operator reconciliation framework (exact pinned version not re-verified in this pass; CLAUDE.md's stack reference states v0.19.0)
- Existing operator, API, agent, and audit infrastructure (`api/internal/rbac/`, `api/internal/audit/`, `api/internal/db/migrations/`)

**Storage**:
- **CRD state**: NetworkCapture CRD (etcd, namespaced, owned by GameServer) — types not yet written.
- **Capture files**: emptyDir volume at `/tmp/captures/capture-<capture-id>.pcapng` (ephemeral, lost on pod restart), pre-provisioned on every game pod's StatefulSet template regardless of opt-in (human Decision 2), ideally with a `sizeLimit`.
- **Audit trail**: existing `audit_events` SQLite/pgx table, plus a new append-only migration adding a structured `reason` column (human Decision 3) — see contracts/audit-events.md.

**Testing Tiers** (per Constitution Principle I — all PLANNED, none written):
- **Unit**: Go package tests for the new `capture-sidecar` module and the modified `operator`/`api` packages.
- **Integration/envtest**: Operator reconciliation of the NetworkCapture CRD against envtest (K8s 1.31 assets).
- **E2E**: Six tests, per `tasks.md` — five in the **operator bucket** and one in the **api-rbac bucket** (`test/e2e/buckets.sh`): `TestGameServer_NetworkCaptureStartStopDownload` (real captured traffic, including filter-matching vs. non-matching packets and asserting SC-001 third-party readability via `tshark`/`capinfos` and SC-008 filter correctness — not structural-only), `TestGameServer_NetworkCaptureEphemeralContainer` (structural: sidecar injection, ephemeralContainers, volumes, securityContext), `TestAPI_RBAC_OperatorCannotReachCaptures` (api-rbac bucket; operator-role 403 on all 8 capture routes), `TestNetworkCapture_RetentionExpiry` (TTL-driven CR + file deletion), `TestGameServer_NetworkCaptureRestartCleanup` (pod deletion mid-capture), and `TestGameServer_NetworkCaptureConcurrencyRejected` (409 on a second concurrent start). This supersedes an earlier draft's single-structural-test commitment; none of these have been written or run yet.

**Target Platform**: Kubernetes 1.28+ (ephemeral containers), Linux containers, `CAP_NET_RAW` on the capture container's binary only, granted via **file capabilities** (`setcap cap_net_raw+ep`), not via `securityContext.capabilities.add` (human Decision 4 — see D-SETCAP in Complexity Tracking: Kubernetes does not set ambient capabilities, so `add: ["NET_RAW"]` under a non-root `runAsUser` grants nothing — the effective set is cleared on `execve`). This requires `allowPrivilegeEscalation: true` on the capture container (file capabilities are ignored under `no_new_privs`, which every container gets by default unless this is set) — `runAsNonRoot` is preserved, `allowPrivilegeEscalation` is the trade given up.

**Project Type**: Multi-component Kubernetes operator system (new sidecar Go module + operator reconciler + REST API handlers + dashboard UI).

**Performance Goals** (SC-002 — "zero perceptible player impact"): no agreed automated measurement exists yet. `quickstart.md`'s SC-002 scenario is a manual/live-cluster comparison (capture ON vs OFF, packet-loss and RTT deltas) that the document itself says may need to be marked live-cluster-only because CI network jitter can mask the result. This is tracked as a validation gap in Complexity Tracking, not a passing benchmark.

**Constraints**:
- **Opt-in only, amended (human Decision 2)**: A non-opted-in GameServer MUST NOT have any capture *component* attached (no sidecar, no ephemeral container) — but it MAY, and per the pre-provisioned-volume decision below DOES, carry an empty, unmounted capture `emptyDir` volume. It is no longer literally byte-identical to servers today; FR-001 and SC-007 are amended to this weaker, true claim.
- **Capability isolation (human Decision 4)**: `CAP_NET_RAW` on the capture container only, granted via file capabilities on its binary (`setcap cap_net_raw+ep`), not via a `securityContext.capabilities.add` grant — Kubernetes does not set ambient capabilities, so `add: ["NET_RAW"]` under a non-root `runAsUser` would grant nothing (D-SETCAP). This requires `allowPrivilegeEscalation: true` on the capture container's `securityContext` (file capabilities are ignored under `no_new_privs`); `runAsNonRoot` stays true, `allowPrivilegeEscalation` is the accepted trade. The game container's securityContext is never touched.
- **Retention window**: Default **24 hours** (spec-derived, per spec.md FR-007 — not a human decision); a cluster maximum of 90 days is an **UNRATIFIED research.md proposal**, not yet approved by a human (see Complexity Tracking). MUST be enforced by an operator reconciliation loop (Kubernetes does not do this automatically for custom CRDs).
- **Concurrency**: One Running capture per GameServer; a second concurrent request is rejected (proposed HTTP 409).
- **Ephemeral-container asymmetry (human Decision 1)**: enable is live and restart-free; disable cannot remove the container until the next pod recreation. This is a platform limitation, not an implementation gap, and spec.md's US2 scenario 4 already stated it explicitly as written — no amendment was applied or needed here (research.md:13).
- **Pre-provisioned capture volume (human Decision 2)**: the capture `emptyDir` is added to the StatefulSet pod template unconditionally, for every game pod, because ephemeral containers cannot add a volume and `pod.spec.volumes` is immutable on a running pod. This causes a one-time rolling restart of every existing game server on upgrade to the release that ships this feature — an upgrade note operators MUST see beforehand — and is the reason FR-001/SC-007 no longer claim byte-identity.
- **PodSecurity admission**: the `restricted` PodSecurity profile forbids both added capabilities and `allowPrivilegeEscalation: true`; a games namespace enforcing `restricted` will reject the capture sidecar outright on either ground. The games namespace needs a documented exception, tracked as a `docs/security.md` obligation (see Complexity Tracking) — capture becomes unavailable in a namespace that cannot carry that exception.
- **RBAC ordering**: capture path rules MUST be inserted ahead of the existing `servers:write`/`servers:read` catch-alls in `rbac.go`'s rule table, or admin-only enforcement silently breaks (see contracts/rest-api.md for the exact ordering and the mandatory operator-403 regression test).

**Scale/Scope**:
- **CRDs**: One new namespaced CRD (NetworkCapture, v1alpha1, gameplane.local); GameServer spec extended with `spec.capture.{enabled, retentionSeconds}` and `status.capture.{ready, activeCapture, lastCaptureTime, sidecarRestarts}` (nested shape — canonical per data-model.md, not the flat `status.captureReady`/`status.captureMessage` an earlier draft used).
- **API surface**: 8 new REST endpoints per `contracts/rest-api.md` (`:capture-enable`, `:capture-disable`, `:capture-start`, `:capture-stop`, `:captures` list, `:capture` get, `:capture-file` download, `:capture` delete) — all under a fixed `:verb` suffix on `/servers/{name}`, a routing shape chosen specifically so `rbac.match`'s prefix/suffix matching can target them (see rest-api.md's "Routing Conventions" for why the originally-proposed nested `/servers/{name}/captures/{id}` paths do not work with the real rule-matching code).
- **Operator surface**: 1 new reconciler (`NetworkCaptureReconciler`) watching NetworkCapture and GameServer pod status; 1 garbage-collection reconciliation pass (proposed every 60s) for TTL expiry; the operator's own ClusterRole additionally needs `pods/ephemeralcontainers` verbs.
- **Sidecar binary**: new standalone Go module `capture-sidecar/`, not yet part of `go.work`.
- **Dashboard UI**: capture start/stop/download UI on the server detail page plus a Settings · Network capture page — the `design.pen` pass (Constitution Principle II) is **complete, pending save**: seven capture screens on Server Detail and the Settings page plus a reusable Capture Warning Banner component were designed via the `pencil` MCP server, re-exported to `design-export/json/` and `design-export/screenshots/` (8 objects, `design-export/MANIFEST.md`'s "Incremental export 2026-08-23" entry), and a tier+1 review passed them. The Pencil document has **not yet been saved by the human**, so none of this design is persisted — it remains at risk until that save happens, and React implementation still waits on it (Complexity Tracking).
- **Test coverage**: six E2E tests across the operator and api-rbac buckets, per `tasks.md` (PLANNED, not written).

## Constitution Check

*Every verdict below describes what the plan and its Phase 0/1 artifacts commit to, not
verified behavior. Nothing in this feature has been implemented, run, or tested — see
the Status note at the top of this document. Verdicts naming "PASS" describe a design
or process commitment judged now, on the documents that exist now; they are not claims
that code exists, compiles, or passes any test.*

**Principle I: E2E-Tested Delivery (NON-NEGOTIABLE)**
- **Status**: PLANNED (corrected from an earlier PASS)
- **Justification**: The earlier draft of this plan marked this principle PASS on the strength of a test named `TestGameServer_NetworkCaptureEphemeralContainer` — that test does not exist. No E2E test, no unit test, and no envtest case for this feature has been written, run, or observed to pass. Principle I is explicit that a feature is "incomplete... until it has a corresponding E2E test" — by that standard this feature cannot be marked PASS at the planning stage under any circumstance; the correct status at this stage is always PLANNED. `tasks.md` now specifies six E2E tests spanning the operator and api-rbac buckets, including one exercising real captured traffic against SC-001/SC-008, not merely the single structural test an earlier draft proposed — this is still a **design commitment for Phase 2**, not evidence of compliance. Verification of this principle is a Phase 2 deliverable, gated on the tasks in `tasks.md` and ultimately on a green CI run per Constitution Principle VI.

**Principle II: Design-First for User-Facing Change**
- **Status**: DESIGN COMPLETE, PENDING SAVE (no longer deferred, no longer merely in progress)
- **Justification**: The capture feature has a dashboard surface: seven capture screens on the Server Detail page (start/stop/download, capture history, filter input) plus a Settings · Network capture page, plus a reusable Capture Warning Banner component. That Pencil pass has been done **in this same change** — all eight objects were designed via the `pencil` MCP server, re-exported to `design-export/json/` and `design-export/screenshots/` (per Constitution II's ordering: design first, then React), and reviewed by a tier+1 review that passed them. As of this plan, the Pencil document has **still not been saved** by the human, so none of this design work is persisted — a lost or reverted in-memory document would lose it entirely, and it must not be treated as final until the save happens. React implementation still waits on that save. Backend/operator/sidecar work remains exempt from this principle (Backend-only per Constitution II). Tracked in Complexity Tracking below.

**Principle III: Language & Ecosystem Best Practice**
- **Status**: PLANNED (corrected from an earlier PASS)
- **Justification**: No Go or TypeScript code for this feature exists yet, so no code can currently be verified to wrap errors with `%w`, pass `golangci-lint`/ESLint, or avoid suppression directives. The commitment stands — new code MUST follow these rules, and any CRD type edit MUST be followed by `make generate && make manifests` in the same commit per Constitution Principle III / CLAUDE.md rule 7 — but this is a rule the Phase 2 implementation and its CI run must satisfy, not something the plan can mark PASS in advance of any code being written.

**Principle IV: Spec-Driven Development**
- **Status**: PASS for the lifecycle process itself; PLANNED for the per-module `specs.md` requirement
- **Justification**: The spec-kit lifecycle has genuinely been followed through this stage — spec.md, research.md, data-model.md, quickstart.md, and contracts/ all exist and were read in full for this revision. They are substantially more consistent than the prior draft on the RBAC mechanism, audit schema, ephemeral-container behavior, and status shape, and four architectural choices — ephemeral-container sidecar injection (human Decision 1), capture storage pre-provisioning (human Decision 2), the audit reason column (human Decision 3), and the `setcap`/file-capabilities grant mechanism (human Decision 4) — are settled by explicit human decision. Two other items that read like decisions are not: the plain-text error response format is derived from reading the existing `api/internal/httperr` code, not chosen by a human, and the 24-hour retention default is a constraint spec.md's FR-007 already fixes, not a fresh decision made here; the cluster-wide 90-day retention maximum remains an unratified research.md proposal (see Complexity Tracking). The migration-numbering inconsistency between `contracts/rest-api.md` and `contracts/audit-events.md` that a prior revision left open here is resolved below (007_audit_reason.sql, 008_captures_rbac.sql); the audit route shape between those two contracts was already reconciled in an earlier pass (see Phase 0 & 1 Artifacts, item 7). What remains PLANNED: Constitution Principle IV also requires every module folder to maintain a `specs.md`; the new `capture-sidecar/` module does not exist yet and therefore has no `specs.md` yet. This must be created in the same change that adds the module's code (Phase 2), not deferred past it — tracked in Complexity Tracking (retained from the prior draft, assessed as a solid entry).

**Principle V: Delegate to Workflows & Subagents**
- **Status**: N/A (planning phase, not implementation)
- **Justification**: This principle governs how the main agent loop delegates implementation work; it does not apply to the authoring of planning artifacts. When Phase 2 implementation begins, work MUST be fanned out via the `Workflow` tool with every `agent()` call setting `model` explicitly, and reviewed one tier above the tier it ran at before acceptance, per Constitution Principle V (2.1.0: this rule scopes to the main loop, not to subagents/workflows themselves).

**Principle VI: CI Bears the Heavy Lifting**
- **Status**: PASS (as a policy commitment for how this feature will be built and verified — not a claim that anything has run)
- **Justification**: This plan does not propose running `make test`, `make lint`, `go test`, `npm test`, or any envtest/kind/e2e suite locally at any point. All such verification is deferred to GitHub Actions CI, consistent with Constitution Principle VI and CLAUDE.md rule 8. A quick compilation check (`go build ./...`, `tsc --noEmit`) is permitted before pushing, once code exists; it is not a substitute for CI and is not being claimed as one here.

---

## Project Structure

### Documentation (this feature)

```text
specs/003-network-capture-sidecar/
├── plan.md              # This file (implementation plan)
├── spec.md              # Feature specification (US2 scenario 4 / FR-001 disable clause already correct as
│                        #   written, no amendment needed there; genuinely amended 2026-08-23 for FR-001/SC-007's
│                        #   "no capture component attached" wording, pre-provisioned-volume decision)
├── research.md          # Phase 0 research consolidation (9 probes; RBAC, audit, and go-pcap risk corrected 2026-08-23)
├── data-model.md         # Phase 1 data model (CRDs, entities, state machine, RBAC/audit validation rules, corrected 2026-08-23)
├── quickstart.md        # Phase 1 quickstart (validation scenarios per SC; SC-002 flagged as weak/manual)
├── contracts/
│   ├── rest-api.md       # REST endpoint contracts — REVISED: real RBAC mechanism, `:verb`-suffix routing
│   ├── capture-sidecar.md  # Sidecar HTTP endpoint contracts (:9091, mTLS, ephemeral-container constraints)
│   └── audit-events.md   # Audit event schema against the real `audit.Event` struct; a structured `reason` column and its hash-chain interaction are RATIFIED (human Decision 3)
└── tasks.md              # Phase 2 output (implementation task breakdown), generated by /speckit-tasks
```

### Source Code (repository root)

Every path below is either verified to exist today (marked EXISTING) or does not exist
yet and is proposed by this plan (marked NEW/PROPOSED). None of the NEW/PROPOSED files
have been written.

**New module**: `capture-sidecar` (Go binary) — PROPOSED, not yet added to `go.work`

```text
capture-sidecar/               # NEW: standalone Go module (not yet in go.work:1-14)
├── cmd/main.go               # Entry point; HTTP server on :9091 (per contracts/capture-sidecar.md)
├── internal/
│   ├── capture/
│   │   ├── afpacket.go       # AF_PACKET socket + MMap'd buffer setup
│   │   └── writer.go         # PCAPNG writing via pcapgo.NgWriter; manual snaplen truncation
│   ├── httpserver/
│   │   └── handlers.go       # POST /captures/{id}:start, :stop, GET /captures/{id}/status, /file
│   └── auth/
│       └── tls.go            # mTLS validation, reusing the agent's cert/CA pattern
├── go.mod, go.sum
├── specs.md                   # REQUIRED by Constitution Principle IV; does not exist until this module does
├── Dockerfile                 # distroless/static:nonroot base
└── (unit tests co-located per package, Go convention: *_test.go)
```

**Operator changes** — `operator/api/v1alpha1/` and `operator/internal/controller/` EXIST; the following are new files or modifications within them:

```text
operator/
├── api/v1alpha1/
│   ├── networkcapture_types.go   # NEW: NetworkCapture CRD type (Spec/Status/Phase per data-model.md)
│   ├── gameserver_types.go       # MODIFIED (EXISTING file): add CaptureConfiguration to GameServerSpec,
│   │                              #   CaptureStatus to GameServerStatus (nested status.capture.*, per
│   │                              #   data-model.md — NOT the flat status.captureReady/captureMessage
│   │                              #   an earlier draft used)
│   └── zz_generated.deepcopy.go  # REGENERATED by `make generate` (CLAUDE.md rule 7) — one regeneration
│                                  #   covering both the new NetworkCapture types and the GameServer additions
├── internal/controller/
│   ├── networkcapture_controller.go       # NEW: NetworkCaptureReconciler (phase transitions, concurrency, TTL)
│   ├── networkcapture_envtest_test.go     # NEW: envtest coverage (PLANNED, not written)
│   └── gameserver_controller.go           # MODIFIED (EXISTING file): add the pre-provisioned capture
│                                            #   emptyDir (with sizeLimit) to the pod template UNCONDITIONALLY,
│                                            #   for every game pod (human Decision 2) — today's pod template
│                                            #   declares only "data" and "agent-tls" plus extraVolumes(...);
│                                            #   inject the ephemeral capture container only when
│                                            #   spec.capture.enabled=true; detect pod restart/recreation to
│                                            #   drop a stale disabled-but-lingering ephemeral container;
│                                            #   ALSO (D-ADDRESSING): extend reconcileAgentService's
│                                            #   svc.Spec.Ports (gameserver_controller.go:869-892) with a
│                                            #   second, numerically-targeted port (9091 → 9091) on the
│                                            #   existing `<gs>-agent` Service so the API/operator can dial
│                                            #   the ephemeral capture sidecar by the Service's existing DNS
│                                            #   name/cert SANs (agent_certs.go:207-224) — no new Service,
│                                            #   no new cert, no IP SAN
├── cmd/main.go                # MODIFIED (EXISTING file): wire capture defaults (retention/duration/size) from Helm-sourced flags
├── config/
│   ├── crd/                   # REGENERATED by `make manifests`: new networkcaptures.gameplane.local base,
│   │                           #   plus the existing gameservers CRD picking up spec.capture/status.capture
│   └── rbac/                  # REGENERATED: new NetworkCapture editor/viewer roles; operator ClusterRole
│                               #   gains pods/ephemeralcontainers verbs
└── specs.md                   # EXISTING file; needs a capture section added in the same change
```

**API changes** — `api/internal/rbac/`, `api/internal/audit/`, `api/internal/db/migrations/` all EXIST today; changes below are additive within them:

```text
api/
├── cmd/main.go                        # MODIFIED (EXISTING file): mount the 8 new capture routes
├── internal/
│   ├── handlers/
│   │   └── capture.go                 # NEW: REST handlers for all 8 endpoints in contracts/rest-api.md
│   ├── kube/
│   │   └── capture.go                 # NEW: Kubernetes client helpers (create/patch/delete NetworkCapture CRs)
│   ├── rbac/
│   │   ├── catalog.go                 # MODIFIED (EXISTING file, entry shape per catalog.go:27): add the
│   │   │                              #   `captures:manage` permission (Namespaced: true)
│   │   └── rbac.go                    # MODIFIED (EXISTING file): insert 8 capture rules into the `rules`
│   │                                  #   slice BEFORE the servers:read rule (rbac.go:183) and the
│   │                                  #   servers:write catch-all (rbac.go:184) — ordering is
│   │                                  #   security-load-bearing, see contracts/rest-api.md
│   ├── db/migrations/
│   │   ├── 007_audit_reason.sql       # NEW, append-only (human Decision 3 — RATIFIED): adds a nullable,
│   │   │                              #   structured audit_events.reason column, next after the six files
│   │   │                              #   (001-006) that currently exist in this directory
│   │   └── 008_captures_rbac.sql      # NEW, append-only: grants captures:manage to admin only, numbered
│   │                                  #   after 007_audit_reason.sql (resolved: rest-api.md and
│   │                                  #   audit-events.md each independently proposed "007_" for their own
│   │                                  #   migration; 007 is assigned to the audit-reason column and
│   │                                  #   008 to the captures:manage grant, per tasks.md)
│   └── audit/
│       └── audit.go                   # MODIFIED (EXISTING file), human Decision 3 (RATIFIED, mechanism
│                                      #   not yet coded): Event gains `Reason string`; insertChained's
│                                      #   INSERT and Page/Stream's SELECT lists extended; `Reason`
│                                      #   participates in the hash-chain computation for rows written
│                                      #   after this migration, while rows written before it must keep
│                                      #   verifying against their original, pre-`reason` hash — this
│                                      #   touches integrity-sensitive code shared by every audited
│                                      #   operation in the system, not just capture (see Complexity
│                                      #   Tracking); a new exported synchronous write method is also
│                                      #   added so capture handlers can write audit rows before
│                                      #   responding (the generic Middleware's query-string Target
│                                      #   extraction does not work for capture's path-parameter routes)
└── specs.md                           # EXISTING file; needs a capture section added in the same change
```

**Agent changes** — UNVERIFIED scope, and now in tension with a settled decision elsewhere
in this artifact set: `contracts/rest-api.md` still flags the small-file-via-agent-proxy /
large-file-via-sidecar split from research.md as not re-verified against
`api/internal/ws/`'s actual routes, and specifies a single API-facing download route
regardless of the internal path chosen. But `data-model.md`'s Entity 3 has since settled
that the capture volume is mounted **only on the sidecar, never on the agent** — which
forecloses the agent-proxy path for capture downloads entirely, since the agent has no
mount to read the file from. `rest-api.md` has not been reconciled to that decision in
this pass; `files.go` is very likely untouched by this feature, not merely
"possibly modified":

```text
agent/
├── internal/files/
│   └── files.go              # LIKELY UNCHANGED (EXISTING file) — data-model.md's sidecar-only
│                              #   mount decision means the agent has no capture volume to serve
│                              #   from, which forecloses the agent-proxy download path this row
│                              #   previously left open; rest-api.md has not yet been reconciled
│                              #   to that decision, so this remains UNVERIFIED, not committed to
└── specs.md                  # EXISTING file; needs a capture section added only if files.go changes
```

**Helm chart**: `charts/gameplane/` EXISTS; changes below are additive

```text
charts/gameplane/
├── values.yaml                # MODIFIED (EXISTING file): capture.enabled, capture.defaultRetentionSeconds,
│                               #   capture.maxRetentionSeconds, capture.defaultMaxDurationSeconds,
│                               #   capture.defaultMaxSizeBytes
├── crds/                      # REGENERATED (synced from operator/config/crd by `make manifests`)
└── crd-manifests/             # REGENERATED (the .Files-readable copy consumed by the pre-upgrade hook)
```

**E2E tests**: `test/e2e/buckets.sh`, `test/e2e/gameserver_e2e_test.go`, and `test/e2e/api_rbac_matrix_e2e_test.go` EXIST; `test/e2e/networkcapture_retention_test.go` is new. Six tests total, per `tasks.md`:

```text
test/e2e/
├── buckets.sh                       # MODIFIED (EXISTING file): add all six test names below to their
│                                    #   respective buckets — operator bucket for the five
│                                    #   NetworkCapture*/GameServer_NetworkCapture* tests, api-rbac bucket
│                                    #   for TestAPI_RBAC_OperatorCannotReachCaptures
├── gameserver_e2e_test.go           # MODIFIED (EXISTING file): add TestGameServer_NetworkCaptureStartStopDownload
│                                    #   (real captured traffic; filter-matching vs. non-matching packets;
│                                    #   SC-001 via tshark/capinfos, SC-008 filter correctness),
│                                    #   TestGameServer_NetworkCaptureEphemeralContainer (structural only),
│                                    #   TestGameServer_NetworkCaptureRestartCleanup (pod deletion mid-capture),
│                                    #   and TestGameServer_NetworkCaptureConcurrencyRejected (409 on a second
│                                    #   concurrent start) — all PLANNED, not written
├── api_rbac_matrix_e2e_test.go      # MODIFIED (EXISTING file): add TestAPI_RBAC_OperatorCannotReachCaptures
│                                    #   (operator-role 403 on all 8 capture routes; admin unaffected),
│                                    #   reusing the file's existing admin/operator sessions (login budget)
└── networkcapture_retention_test.go # NEW: TestNetworkCapture_RetentionExpiry (short-TTL CR + backing file
                                     #   deletion via Eventually(), then a list call omitting it)
```

**Dashboard**: `web/src/routes/ServerDetail.tsx`, `web/src/lib/api.ts`, `web/src/types.ts` EXIST

```text
web/
├── src/
│   ├── routes/
│   │   └── ServerDetail.tsx   # MODIFIED (EXISTING file) — only after the Pencil document is saved
│   ├── components/
│   │   └── CaptureWidget.tsx  # NEW — only after the Pencil document is saved
│   ├── lib/
│   │   └── api.ts             # MODIFIED (EXISTING file): capture API client calls, matching the 8
│   │                          #   endpoints in contracts/rest-api.md
│   └── types.ts               # MODIFIED (EXISTING file): NetworkCapture type, nested
│                              #   status.capture.{ready,activeCapture,lastCaptureTime,sidecarRestarts}
├── design.pen                 # Edited via the pencil MCP server (Constitution II), never read/
│                              #   hand-edited directly — design work is COMPLETE in this same change:
│                              #   seven capture screens on Server Detail plus a Settings · Network
│                              #   capture page, plus a reusable Capture Warning Banner component,
│                              #   tier+1-reviewed and passed. Still NOT SAVED by the human as of this
│                              #   plan, so none of it is persisted; nothing from this pass may be
│                              #   claimed as final until that save happens.
└── design-export/
    └── json/, screenshots/    # DONE: all 8 touched objects re-exported (design-export/MANIFEST.md's
                                #   "Incremental export 2026-08-23" entry) — a plain-file snapshot of
                                #   the in-memory Pencil state, not a substitute for the human's save
```

**Structure Decision**: The capture feature spans six layers, mirroring the existing
optional-component pattern (sentinel, mcp-server):

1. **Capture sidecar** (`capture-sidecar/`, new module): live packet capture (AF_PACKET), PCAPNG writing, HTTP control endpoints. Decoupled from operator/API; communicates via the NetworkCapture CRD and mTLS HTTP.
2. **Operator**: pre-provisions the capture `emptyDir` unconditionally in the pod template for every game pod (human Decision 2); reconciles NetworkCapture CRD state; injects the sidecar via `pods/ephemeralcontainers`; enforces concurrency and TTL cleanup; cannot remove the ephemeral container on disable (platform limitation).
3. **API**: exposes the 8 REST endpoints; enforces `captures:manage` via the corrected rule-table ordering; validates filters and TTL; writes audit rows before responding.
4. **Agent**: likely unchanged — data-model.md's sidecar-only volume mount decision forecloses the agent-proxy download path a prior draft left open; see above.
5. **Helm**: feature-gating via `capture.enabled` and cluster-wide defaults.
6. **Dashboard**: capture UI on the server detail page plus a Settings · Network capture page; the Pencil design pass per Constitution II is **complete** in this same change (seven screens plus the Capture Warning Banner component, re-exported to `design-export/`, tier+1-reviewed), but the Pencil document is still unsaved by the human, so none of it is persisted or final yet.

Each layer is independently testable in principle (operator tests structural/envtest,
API tests contract-based, sidecar tests unit-focused) — none of that testing has
happened yet.

---

## Complexity Tracking

| Violation / Tracked Gap | Why Needed | Simpler Alternative Rejected Because |
|---|---|---|
| **Missing `specs.md` for the new `capture-sidecar` module** | Constitution Principle IV mandates every module folder maintain `specs.md` documenting responsibilities, protocols, inputs/outputs, invariants. The module does not exist yet, so neither does its `specs.md`. | Deferring `specs.md` past the module's initial commit is not optional under Principle IV; it must land in the same change as the module's code in Phase 2, not as a follow-up. |
| **Dashboard UI required a Pencil design pass first (Principle II) — design complete, save still pending, in this change** | The capture UI is user-facing: seven capture screens on Server Detail (start/stop/download, filter input, history list) plus a Settings · Network capture page, plus a reusable Capture Warning Banner component. Constitution II requires the `design.pen` edit and its `design-export/` re-export to happen before any React implementation. That Pencil pass is **done** in this same change via the `pencil` MCP server — all 8 objects re-exported to `design-export/json/` and `design-export/screenshots/` and tier+1-reviewed — but the human has not yet saved the document, so nothing from it is persisted or may be treated as final until that save happens. | Implementing the React UI first and reconciling with Pencil afterward is exactly the code-led-redesign pattern Principle II forbids and that has caused drift and reversion on this project before. React implementation is deliberately held until the save, rather than starting from the unsaved in-memory design, because an unsaved Pencil document is not durable — it exists only in the editor's in-memory state until the human saves — so building against it now risks building against state that may not exist tomorrow. |
| **Capture `emptyDir` pre-provisioned on every game pod, opted in or not (human Decision 2)** | Ephemeral containers cannot add a volume via `pods/ephemeralcontainers`, and `pod.spec.volumes` is immutable on a running pod — so the volume must already exist in the StatefulSet pod template before capture can be enabled restart-free. This is the only way to keep "enable capture" from requiring a pod recreation. | Adding the volume lazily, only when capture is first enabled, was ruled out (research.md) because it would itself be a pod-template change and therefore force a restart at the moment of opting in — the exact cost pre-provisioning exists to avoid. Accepting pre-provisioning instead costs: (a) a one-time rolling restart of every existing game server on upgrade to the release that adds it, regardless of whether that server ever uses capture — an upgrade note operators MUST see beforehand; and (b) FR-001 and SC-007's "byte-identical" claim can no longer be made as originally worded and is amended to "no capture *component* attached" (the pod may still carry the empty, unmounted volume). Both costs are accepted and written down here rather than glossed over. |
| **New append-only audit migration touches hash-chain integrity code (human Decision 3)** | Structured audit reasons (FR-006) require a new column on `audit_events`; the only place to add one, given the append-only migration convention, is a new migration file (`007_audit_reason.sql`, next after the six files currently in `api/internal/db/migrations/`). | `005_audit_chain.sql` hash-chains every row (`prev_hash`/`hash`, computed in `insertChained`), and that chain covers *every* audited operation in the system, not just capture — so this is not a capture-scoped migration in its blast radius even though capture is what motivates it. `Reason` participates in the hash computation for rows written after the migration; rows written before it have no `reason` value and were hashed without one, and verifying them retroactively must reproduce their original, pre-`reason` hash bit-for-bit — getting this wrong would break `Verify` for 6+ months of pre-existing rows the moment the migration ships. There is no simpler alternative that adds a structured, audited reason without touching this shared code path. |
| **Cluster-wide 90-day retention maximum is UNRATIFIED — open item, not a decision** | research.md proposes capping `spec.ttlSecondsAfterFinished`-style retention at 90 days cluster-wide via a Helm value, as a guardrail against an admin setting an unbounded or excessive per-GameServer retention. Recorded here so it is not silently treated as settled. | No human has approved this specific number, or the mechanism (Helm value vs. a cluster-scoped CRD field), or whether a maximum is needed at all — it is a Phase 0 proposal only. It MUST be raised as an explicit open question before Phase 2 implements any enforcement of a cluster maximum; only the 24-hour **default** is fixed today, by spec.md FR-007. |
| **E2E coverage now includes a real-traffic functional test, not structural-only (Principle I scope, revised)** | An earlier draft committed to a single structural-only E2E test. `tasks.md` has since expanded this to six tests: `TestGameServer_NetworkCaptureStartStopDownload` generates real filter-matching and non-matching traffic against a live GameServer, downloads the resulting file, and asserts SC-008 (zero non-matching packets) and SC-001 (third-party readability, via `tshark`/`capinfos` rather than re-reading with the same `gopacket/pcapgo` that wrote it) — plus `TestGameServer_NetworkCaptureEphemeralContainer` (structural-only), `TestAPI_RBAC_OperatorCannotReachCaptures`, `TestNetworkCapture_RetentionExpiry`, `TestGameServer_NetworkCaptureRestartCleanup`, and `TestGameServer_NetworkCaptureConcurrencyRejected`. | The structural-only scope reduction was accepted earlier because a functional test needs a real game image pull, a real join client, and materially longer per-test runtime. `tasks.md` accepts that cost for one test (gated on the setcap-survives-COPY CI proof, T039) while keeping the remaining five structural/behavioral to bound total runtime; none of the six have been written or run yet — see the corrected Principle I verdict above. |
| **SC-002 ("zero perceptible player impact") has no reliable automated validation path** | quickstart.md's SC-002 scenario is a manual capture-ON-vs-OFF comparison of packet loss and RTT against a real or synthetic game client; the document itself notes CI network jitter may force this to be live-cluster-only, with CI serving only a coarser sanity check (zero packet loss, 5% RTT tolerance) rather than the full comparison. | An automated, CI-reliable network-performance assertion for a live UDP/TCP game protocol under a kind cluster's variable network conditions was not designed in Phase 0/1; building one is a larger effort than this feature's scope justifies given the mitigation (manual live-cluster validation, documented rationale) quickstart.md already proposes. Tracked here so Phase 2 does not silently drop SC-002 verification rather than explicitly deferring the automated form of it. |
| **Elevated Linux capability (`CAP_NET_RAW`) on the capture container, granted via file capabilities (D-SETCAP, human Decision 4, final)** | AF_PACKET raw-socket capture is not possible without it. The grant mechanism is **file capabilities on the capture binary** (`setcap cap_net_raw+ep`), not `securityContext.capabilities.add` — Kubernetes does not set ambient capabilities, so `add: ["NET_RAW"]` under a non-root `runAsUser` grants nothing (the effective set is cleared on `execve`). Isolated to the capture container's own binary; the game container's security posture is left untouched. | An unprivileged capture mechanism (eBPF-based) was considered and deferred (research.md, Security Posture section) as newer and less proven across the range of game workloads this project targets. Running the capture container as root instead of using file capabilities was rejected as a strictly worse trade — it gives up `runAsNonRoot` entirely rather than the narrower `allowPrivilegeEscalation` concession below. This does not eliminate risk — a cluster enforcing `PodSecurity: restricted` on the games namespace will reject these pods (see the next two rows), which is recorded as a constraint, not resolved by this feature. |
| **File capabilities require `allowPrivilegeEscalation: true` (D-SETCAP consequence)** | File capabilities are ignored under `no_new_privs`, which is set whenever `allowPrivilegeEscalation` is unset or `false`. The capture container's `securityContext` MUST therefore set `allowPrivilegeEscalation: true` for `cap_net_raw+ep` on its binary to take effect at all. `runAsNonRoot: true` is preserved — only `allowPrivilegeEscalation` is given up. | Keeping `allowPrivilegeEscalation: false` and relying on `securityContext.capabilities.add: ["NET_RAW"]` instead was the initially assumed approach and does not work: Kubernetes does not grant ambient capabilities, so a non-root process's effective capability set is cleared on `execve` regardless of what the pod spec's `capabilities.add` list says. There is no combination of `runAsNonRoot: true` + `allowPrivilegeEscalation: false` that yields a working `CAP_NET_RAW` grant for a non-root process today. The `restricted` PodSecurity profile forbids `allowPrivilegeEscalation: true` outright, so the games namespace needs a documented exception — tracked as a `docs/security.md` obligation, not yet written. |
| **UNVERIFIED: whether file capabilities survive a multi-stage `COPY` into the capture-sidecar's distroless/scratch image (D-SETCAP build risk)** | File capabilities are stored as filesystem extended attributes (xattrs) on the binary's inode, set by `setcap` at build time. Whether `COPY --from=builder` in a multi-stage Dockerfile preserves those xattrs into the final distroless/scratch layer is not established for this project's build tooling. | This repo has already hit a COPY-time file-mode problem once (see `fix(images): set entrypoint mode at COPY time instead of chmod` in recent history) — xattr loss on COPY is a known class of failure here, not a hypothetical one. This plan does **not** assert that the capability survives the image build; that must be proven in CI (e.g., an image-build smoke step running `getcap` on the built image's binary) before this approach is trusted. If it does not survive, the fallback (rebuilding the layer with `setcap` run after the final `COPY`, or a non-scratch base that supports xattrs) is Phase 2 investigation, not yet designed. |
| **Dependency on an untagged Go module (`github.com/packetcap/go-pcap`)** | It is, per research.md, the only pure-Go BPF filter-expression compiler available, needed to validate filters at the API tier before a NetworkCapture CRD is created (FR-003) without depending on cgo/libpcap. | The module resolves only to a pseudo-version (`v0.0.0-20260731105150-c86974bbfbcd`) with no tagged release — no semver guarantee, no changelog, and upstream can rewrite its default-branch history out from under a pinned commit. Alternatives considered and rejected in research.md: `gopacket/pcap` (requires cgo + libpcap, breaks the distroless image goal) and shelling out to `tcpdump` (adds a binary dependency and subprocess-management surface for filter validation alone). The risk is accepted for now and must be re-evaluated (vendor the pinned commit, or re-check for a tagged release) before this dependency ships, per research.md's "Open Risks" §1. |
| **Ephemeral-container enable/disable asymmetry (human Decision 1)** | Kubernetes' ephemeral-container API provides no removal call. Enabling capture is live and restart-free (satisfies the original US2 acceptance-scenario-2 requirement); disabling capture cannot remove the container from a running pod — it can only stop routing new captures to it, with actual removal deferred to the pod's next recreation. | There is no simpler alternative that satisfies "add without restart" other than ephemeral containers (a regular `pod.spec.containers` sidecar requires a StatefulSet template change, which recreates the pod on *every* enable, not just disable — ruled out in research.md). spec.md's US2 acceptance scenario 4 and FR-001 already state the disable behavior precisely as originally written; no amendment was applied or needed (research.md:13 corrects an earlier draft that fabricated both a "controlled pod restart" claim and a spec amendment that never existed). |
| **New top-level Go module directory (`capture-sidecar/`)** | The capture engine (AF_PACKET, PCAPNG writing, mTLS HTTP control) needs its own binary and image, separate from the agent, so the agent's existing coverage gate, dependency set, and distroless base are not disturbed by capture-specific dependencies (`gopacket`, the untagged `go-pcap`). | Folding capture logic into the existing `agent/` module was considered implicitly by the "sidecar vs. embedded" framing in research.md's injection-mechanism section and rejected there on isolation grounds (the agent already has its own trust boundary and coverage gate — see CLAUDE.md's `agaction`/`netguard` package split for the established precedent of carving out a boundary rather than growing an existing module). Adding `capture-sidecar/` to `go.work`, giving it its own `.testcoverage.yml`, and gating it in `.golangci.yml` are all Phase 2 setup steps not yet done. |

---

## Phase 0 & 1 Artifacts

The following Phase 0 and Phase 1 artifacts exist. Each was re-read in full for this
revision; several were substantively corrected earlier in this review round (RBAC
mechanism, audit schema, CRD field names, the ephemeral-container removal limitation,
and the untagged `go-pcap` dependency risk) after an earlier draft fabricated APIs that
do not exist in this codebase. Nothing in any of them describes a test that has run or
a behavior that has been observed — all of it is planned.

1. **spec.md**: Feature specification with User Stories US1–US5, Functional Requirements
   FR-001–FR-012, Success Criteria SC-001–SC-008, Assumptions, and Out-of-Scope items.
   US2 acceptance scenario 4 and FR-001's disable clause already state precisely, as
   originally written, that capture-enable is restart-free via an ephemeral container
   and capture-disable cannot remove that container until the next pod recreation; no
   amendment was applied there or is needed (research.md:13). Separately, FR-001 and
   SC-007 genuinely were amended 2026-08-23 to their "no capture *component* attached"
   wording, in place of a literal "byte-identical" claim, because of the pre-provisioned
   capture volume (research.md:126, 231; see Complexity Tracking).

2. **research.md**: Consolidation of 9 research probes covering sidecar injection
   (ephemeral containers, with the removal limitation as an accepted trade-off), the
   capture engine (`gopacket/afpacket` + `pcapgo`), filter validation
   (`go-pcap/filter`, flagged untagged), NetworkCapture CRD design, storage (emptyDir),
   retention (a TTL-style field that the operator must actively enforce), concurrency
   (CRD phase-based), security posture (`CAP_NET_RAW`, admin-only via the real RBAC rule
   table, mandatory audit), E2E strategy (operator bucket, structural-only), and a
   numbered list of nine open risks.

3. **data-model.md**: NetworkCapture CRD (Spec/Status/Phase/lifecycle state machine,
   including the two-part lifecycle — ephemeral-container enable/disable vs. per-capture
   phase transitions), GameServer extensions using the real field names
   (`spec.templateRef.name`, `spec.networking.expose`, no `gameplane.io/gameserver`
   label) and the canonical nested `status.capture.{ready,activeCapture,
   lastCaptureTime,sidecarRestarts}` shape, the real `audit.Event` struct with no
   payload/result column, and the exact RBAC rule-table ordering requirement with a
   mandatory regression test named.

4. **quickstart.md**: Runnable (on CI or the live kubelab cluster only — never locally,
   per Constitution VI) validation scenarios mapped to SC-001 through SC-008, including
   the SC-002 performance-comparison scenario now explicitly flagged as weak/manual (see
   Complexity Tracking) and the US2.2 disable scenario now written to show the
   ephemeral container persisting in the running pod and only disappearing after an
   explicit pod recreation step.

5. **contracts/rest-api.md**: REST endpoint contracts for all 8 capture operations,
   rewritten around the real `rbac.match` mechanism — every path uses a fixed `:verb`
   suffix (never a variable path segment) so it can be matched by a `prefix`/`suffix`
   rule ahead of the `servers:write`/`servers:read` catch-alls, with the exact
   rule-table insertion shown and an operator-role-403 test made mandatory.

6. **contracts/capture-sidecar.md**: sidecar HTTP endpoint contracts (`:9091`,
   mTLS-authenticated, no bearer tokens), the ephemeral-container constraints that
   shape the design (no probes, cannot restart, no resource limits, cannot be removed),
   and the duration/size hard-limit enforcement mechanics.

7. **contracts/audit-events.md**: audit event schema reconciled against the real
   `audit.Event` struct (no payload/result/operation columns); documents the FR-006 gap
   this creates (Method+Path alone cannot distinguish enable from disable) against the
   real `POST /servers/{name}:capture-enable` / `:capture-disable` routes from
   `contracts/rest-api.md`, no longer the retired `PATCH /servers/{name}` shape used in
   an earlier draft; records the append-only `reason`-column migration and its
   hash-chain interaction (human Decision 3, RATIFIED) as this feature's canonical audit
   design, including how a pre-migration row must keep verifying against its original,
   pre-`reason` hash.

**Migration numbering (resolved)**: `contracts/rest-api.md` and `contracts/audit-events.md`
independently proposed the next migration file be numbered `007_` — `007_captures_rbac.sql`
and `007_audit_reason.sql` cannot both hold that number against the six files (`001`–`006`)
that exist today. This is resolved: `007_audit_reason.sql` (human Decision 3, the audit
`reason` column) is next after the six existing files, and `008_captures_rbac.sql` (the
`captures:manage` permission grant) follows it — see Project Structure above and
`tasks.md`, which already builds against this numbering. The earlier route-shape mismatch
between the two contracts (`PATCH /servers/{name}` vs. `:capture-enable`/`:capture-disable`)
was separately reconciled in `contracts/audit-events.md` and is no longer open.

**Next step**: `tasks.md` has been generated via `/speckit-tasks` and builds on the
migration numbering resolved above; proceed to Phase 2 implementation per its task
breakdown.
