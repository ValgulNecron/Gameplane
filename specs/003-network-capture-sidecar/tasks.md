---

description: "Task list for feature 003-network-capture-sidecar"
---

# Tasks: Network Capture Sidecar

**Input**: Design documents from `/specs/003-network-capture-sidecar/`

**Prerequisites**: plan.md, spec.md, data-model.md, contracts/rest-api.md, contracts/capture-sidecar.md, contracts/audit-events.md, quickstart.md, `.specify/memory/constitution.md`, `CLAUDE.md`

**Tests**: Required. Constitution Principle I makes E2E delivery non-negotiable for this feature; unit, envtest, and e2e tasks are included throughout.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no unfinished dependency)
- **[US1]..[US5]**: User story this task belongs to (Setup/Foundational/Polish carry no story tag)
- Every task names an exact file path

---

## Phase 1: Setup

**Purpose**: Scaffold the new `capture-sidecar` Go module and wire it into the repo's build/CI/release machinery, before any capture logic exists.

- [X] T001 Create the capture-sidecar module skeleton: capture-sidecar/go.mod (module github.com/ValgulNecron/gameplane/capture-sidecar, go 1.26), capture-sidecar/cmd/main.go (entry-point stub for the :9091 HTTP server), and package skeletons capture-sidecar/internal/capture/afpacket.go, capture-sidecar/internal/capture/writer.go, capture-sidecar/internal/httpserver/handlers.go, capture-sidecar/internal/auth/tls.go. Add `capture-sidecar` to the `use` block of go.work.
- [X] T002 [P] Write capture-sidecar/specs.md (Constitution Principle IV) documenting the module's responsibilities (AF_PACKET live capture, PCAPNG writing, mTLS-authenticated :9091 control surface), inputs/outputs (contracts/capture-sidecar.md's endpoints), and invariants (filter validated before capture starts, hard duration/size limits, file capabilities not securityContext.capabilities.add, cannot be removed once injected).
- [X] T003 [P] Add capture-sidecar/Dockerfile: distroless/static:nonroot multi-stage build applying `setcap cap_net_raw+ep` to the built binary and preserving it across the final-stage COPY — the blocking-vs-alongside sequencing of the survive-COPY CI proof (T039) is open decision 3, see T009 (not T014, which is the GameServer CRD edit); and capture-sidecar/.testcoverage.yml (initial threshold 70), matching the sentinel/.testcoverage.yml shape.
- [X] T004 [P] Add `capture-sidecar` to the `IMAGES :=` list in Makefile (around Makefile:240) so the existing `image-%` pattern rule and `images` target cover it, matching the sentinel/mcp-server precedent; also add it to `GO_MODULES :=` at Makefile:35 so `build-go`, `test-go`, `lint-go`, and `cover` pick it up, and to whatever `make tidy` iterates if that does not already derive from `GO_MODULES` — verify before assuming duplication.
- [X] T005 [P] Wire capture-sidecar into .github/workflows/ci.yaml: add it to the go-build module matrix, the lint/coverage matrix, and add a `capture-sidecar/**` path filter alongside the existing `sentinel/**`/`mcp-server/**` entries.
- [X] T006 [P] Wire capture-sidecar into image publishing: add a `component: capture-sidecar` matrix entry to .github/workflows/publish-edge.yaml and .github/workflows/release.yaml, mirroring their existing sentinel entries. Verify whether .github/workflows/images.yaml separately enumerates components before deciding whether it also needs an entry.

**Checkpoint**: capture-sidecar exists as a buildable, CI-visible, empty module. No capture logic yet.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: CRD/type plumbing, RBAC/audit groundwork, StatefulSet/Service wiring, Helm/operator flags, and the maintainer decision spikes that gate later work.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

### Decision spikes (do first — six open maintainer decisions)

- [X] T007 [P] DECISION SPIKE — Open Decision 1 (download path): read api/internal/ws/ in full and resolve whether `GET /servers/{name}:capture-file` streams directly from the sidecar's :9091 `/file` endpoint (mTLS, through the existing `<gs>-agent` Service) or is proxied through a new agent endpoint. Record the resolution in specs/003-network-capture-sidecar/research.md, replacing rest-api.md's UNVERIFIED note. Phrase the outcome so T044 (download handler) holds for either branch if genuinely undecidable.
- [X] T008 [P] DECISION SPIKE — Open Decision 2 (PodSecurity "restricted" exception ownership): determine who owns writing the docs/security.md note that the capture sidecar's ephemeral container needs `allowPrivilegeEscalation: true` (setcap requires it) despite a "restricted" namespace baseline. Record the answer in specs/003-network-capture-sidecar/research.md's Open Risks section.
- [X] T009 [P] DECISION SPIKE — Open Decision 3 (setcap-survives-distroless-COPY CI proof): determine whether the "do file capabilities survive a multi-stage COPY into distroless" CI proof BLOCKS capture-sidecar/Dockerfile authorship or runs alongside it. Record the decision in specs/003-network-capture-sidecar/research.md next to the D-SETCAP entry.
- [X] T010 [P] DECISION SPIKE — Open Decision 4 (GET auditing scope): determine whether handler-side GET auditing (list, get-status, download) is in-scope as new work, given `audit.shouldLog` (api/internal/audit/audit.go) returns false for all GET requests but FR-006 requires the download to be audited and rest-api.md's own table claims `:captures` (list) is audited too. Record the decision as a "GET auditing scope" note appended to specs/003-network-capture-sidecar/contracts/audit-events.md.
- [X] T011 [P] DECISION SPIKE — Open Decision 5 (cluster-max retention value): confirm the exact cluster-max-retention value/Helm-key name (research.md proposes 90 days/7776000s — this is an assistant-derived, unratified proposal, not a maintainer decision). Migration numbering is already settled (`007_audit_reason.sql`, `008_captures_rbac.sql` per plan.md) and is not gated by this spike. Record the ratified max value in specs/003-network-capture-sidecar/data-model.md.
- [X] T012 [P] DECISION SPIKE — Open Decision 6 (captures:manage grantability): FR-005 says `captures:manage` is admin-only, but adding it as an ordinary entry to api/internal/rbac/catalog.go makes it grantable to any custom role by anyone holding `roles:manage` — no non-grantability mechanism exists today. Record the resolution in specs/003-network-capture-sidecar/data-model.md: either amend FR-005 to accept custom-role grantability, or specify the exact non-grantability mechanism to build.

### CRD types and codegen

- [X] T013 Create operator/api/v1alpha1/networkcapture_types.go: the NetworkCapture CRD Go type (namespaced, group `gameplane.local`, shortname `cap`, owned by GameServer) — Spec: filter, maxDurationSeconds, maxSizeBytes, ttlSecondsAfterFinished (kubebuilder `minimum=0`, per data-model.md Entity 1, honoring T011's ratified max), gameServerRef; Status: phase (Pending/Running/Completed/Failed/Expired), startTime, completionTime, fileSizeBytes, error — with kubebuilder markers matching an existing type file (e.g. operator/api/v1alpha1/backup_types.go).
- [X] T014 Edit operator/api/v1alpha1/gameserver_types.go: add `CaptureConfiguration` to GameServerSpec (`spec.capture.{enabled, retentionSeconds}`) and a nested `CaptureStatus` to GameServerStatus (`status.capture.{ready, activeCapture, lastCaptureTime, sidecarRestarts}`) — the canonical NESTED shape from data-model.md, not a flat `status.captureReady`/`captureMessage` shape.
- [X] T015 MANDATORY, same change as T013/T014 (CLAUDE.md rule 7): run `make generate && make manifests` (in CI/an environment that can invoke it — never locally) and commit the regenerated operator/api/v1alpha1/zz_generated.deepcopy.go, the new operator/config/crd/gameplane.local_networkcaptures.yaml plus the regenerated gameservers CRD YAML, operator/config/rbac/role.yaml, and their synced copies charts/gameplane/crds/gameplane.local_networkcaptures.yaml + charts/gameplane/crd-manifests/gameplane.local_networkcaptures.yaml.

### StatefulSet/Service/operator wiring

- [X] T016 Add a `+kubebuilder:rbac:groups="",resources=pods/ephemeralcontainers,verbs=get;list;watch;patch;update` marker near the existing pod-related RBAC markers in operator/internal/controller/gameserver_controller.go, so `make manifests` grants the operator ClusterRole this verb (must land before re-running T015's regeneration, or before the next regeneration if T015 already ran).
- [X] T017 In operator/internal/controller/gameserver_controller.go, add the pre-provisioned capture `emptyDir` volume (with a `sizeLimit` backstop) UNCONDITIONALLY to every game pod's StatefulSet template, alongside the existing `data` and `agent-tls` volumes — every game pod gets this volume regardless of `spec.capture.enabled` (human Decision 2).
- [X] T018 In operator/internal/controller/gameserver_controller.go's `reconcileAgentService` (around gameserver_controller.go:869-895), extend `svc.Spec.Ports` with a second, numerically-targeted port (port 9091, targetPort 9091 — not a named containerPort, since an ephemeral container cannot declare one) on the existing `<gs>-agent` Service, so the sidecar's :9091 control endpoint is reachable through the Service's existing DNS name and mTLS cert SANs (D-ADDRESSING) without a new Service, cert, or IP SAN.
- [X] T019 [P] Edit operator/cmd/main.go: add CLI flags for cluster-wide capture defaults (retention default/max seconds, default max capture duration, default max capture size bytes) sourced from Helm values, wired to a config struct the reconciler and API will read. Plumbing only.
- [X] T020 [P] Add capture values to charts/gameplane/values.yaml (`capture.enabled`, `capture.defaultRetentionSeconds` = 86400, `capture.maxRetentionSeconds` per T011, `capture.defaultMaxDurationSeconds`, `capture.defaultMaxSizeBytes`) and wire the matching operator Deployment flags in charts/gameplane/templates/operator.yaml.
- [X] T021 [P] Update operator/specs.md (Constitution Principle IV) with what Foundational actually established: the pre-provisioned-emptyDir decision (human Decision 2) and the new `pods/ephemeralcontainers` RBAC verb. The reconciler itself doesn't exist yet at this point — its responsibilities and the injection design are documented once built, in T048.

### RBAC and audit foundations

- [X] T022 Add the `captures:manage` permission to the Catalog in api/internal/rbac/catalog.go (`Namespaced: true`), implementing whatever admin-only non-grantability mechanism T012 settled on.
- [X] T023 SECURITY-LOAD-BEARING: insert the capture rule-table entries into the `rules` slice in api/internal/rbac/rbac.go — `POST :capture-enable`, `POST :capture-disable`, `POST :capture-start`, `POST :capture-stop`, `GET :captures`, `GET :capture-file`, `GET :capture`, `DELETE :capture`, all `segment:"servers"`, `perm:"captures:manage"` — BEFORE the existing `{method:"GET", segment:"servers", perm:"servers:read"}` rule (rbac.go:183) and the `{segment:"servers", perm:"servers:write"}` catch-all (rbac.go:184). Add a doc comment matching the file's style noting these MUST precede the catch-all. If T007 resolves the download path onto a `/ws`-routed proxy, also insert `GET :capture-file`'s rule ahead of the `/ws` catch-all at rbac.go:219.
- [X] T024 Add a unit test in api/internal/rbac/rbac_test.go asserting: each of the 8 capture routes resolves via `match()` to `captures:manage` (not `servers:read`/`servers:write`), and the capture rules' slice index is structurally less than the `servers:write` catch-all's index — so a future reordering fails this test, not just production traffic.
- [X] T025 Add the captures-RBAC migration file api/internal/db/migrations/008_captures_rbac.sql (per plan.md's fixed numbering — 007 is audit_reason, 008 is captures_rbac; append-only, next after 001-006) granting `captures:manage` to the admin role only — never operator or viewer (verify against 003_roles.sql's grant pattern, which is NOT itself edited).
- [X] T026 Add the audit-reason migration file api/internal/db/migrations/007_audit_reason.sql (per plan.md's fixed numbering — see T025) adding a nullable `reason TEXT` column to `audit_events`. Schema only.
- [X] T027 In api/internal/audit/audit.go: add `Reason string \`json:"reason,omitempty"\`` to the `Event` struct (audit.go:727-736); extend `insertChained`'s Event literal, INSERT statement, and both Page's and Stream's SELECT column lists (audit.go:745, 797) to carry the column through.
- [X] T028 Fix `computeHash`'s field coverage in api/internal/audit/audit.go for the new `Reason` field: an empty/NULL Reason MUST serialize bit-for-bit identically to the field not existing, so pre-migration rows keep verifying under `Verify` (audit.go:454); a non-empty Reason MUST change the hash. Add api/internal/audit/audit_test.go cases: (a) a pre-migration-shaped Event's hash is unchanged after Reason is added with its zero value; (b) two rows differing only in Reason hash differently; (c) `Verify` passes over a chain mixing pre- and post-migration rows.
- [X] T029 Add a new exported, synchronous audit-write method on `Auditor` (e.g. `WriteSync`) in api/internal/audit/audit.go, callable directly from handler code before the HTTP response is written (today `insertChained` is unexported, only reachable via `Middleware`). It must accept `Reason` and the composite `Target` encoding from contracts/audit-events.md section 3. On write failure it MUST `slog.Warn` and return without erroring the caller — a capture operation must never fail because the audit sink is down. Add a unit test in api/internal/audit/audit_test.go asserting a broken/closed DB handle does not panic or surface an error to a caller that ignores the failure path, and that a warning is logged.
- [X] T030 [P] Update api/specs.md (Constitution Principle IV) noting what Foundational actually established: `captures:manage`, the two new migrations and their resolved numbers (007_audit_reason.sql, 008_captures_rbac.sql), and the audit `Reason` column's hash-chain interaction. The REST endpoint surface itself is documented per-story as each story's routes land (T047 for US1, etc.) — don't enumerate it here.

**Checkpoint**: Foundation ready — CRD types exist and are generated, RBAC/audit plumbing and migrations are in place, StatefulSet/Service carry the capture volume/port, and all six decisions are either resolved or explicitly recorded as pending. User story implementation can now begin.

---

## Phase 3: User Story 1 - Start, record, stop, and download a filtered capture (Priority: P1) 🎯 MVP

**Goal**: An operator can start a filtered packet capture on a GameServer, have it record traffic to a valid PCAPNG file, stop it (explicitly or via a hard duration/size limit), and download the completed file.

**Independent Test**: Start a capture with a BPF filter against a running GameServer, generate matching traffic, stop the capture (and separately, let a tiny size/duration limit trigger an automatic stop), then download the file and confirm it opens as valid, non-empty PCAPNG containing only filter-matching packets.

### Tests for User Story 1

- [ ] T031 [P] [US1] Write capture-sidecar/internal/capture/writer_test.go covering: a normal stop produces a valid file openable via `gopacket/pcapgo`; a size-limit-triggered stop truncates cleanly to a valid file; a duration-limit-triggered stop likewise. Write before T033 and expect these to fail first.
- [ ] T032 [P] [US1] Write capture-sidecar/internal/capture/filter_test.go covering: a valid filter is accepted and applied; an invalid filter expression is rejected before capture start; an empty filter falls back to the port-based default; a filter matching zero traffic produces an empty-but-valid pcapng file; a filter matching some traffic (synthetic packet source is acceptable) passes only matching packets through. Write before T035.

### Implementation for User Story 1

- [ ] T033 [US1] Implement capture-sidecar/internal/capture/afpacket.go: AF_PACKET socket + MMap'd TPacket buffer setup for live capture, reading CAP_NET_RAW from the process's file-capability grant (no `securityContext.capabilities.add` dependency). Expose a Start/Stop lifecycle and a packet channel/callback for the writer. (depends on T031)
- [ ] T034 [US1] Implement capture-sidecar/internal/capture/writer.go: PCAPNG writing via `gopacket/pcapgo.NgWriter`, manual snaplen truncation, and enforcement of FR-002's two hard limits (max duration, max size) — the writer stops accepting packets and cleanly finalizes the file the instant either limit is hit, producing a valid, complete file even on a limit-triggered stop. Make T031 pass.
- [ ] T035 [US1] Implement capture-sidecar/internal/capture/filter.go: BPF filter compilation/validation via `github.com/packetcap/go-pcap`'s `filter.Compile`, invoked BEFORE any capture starts (FR-003, FR-011) — reject invalid expressions before the AF_PACKET loop exists; apply the compiled filter at the capture-process level (never as post-processing); default (no filter supplied) restricts to the GameServer's own advertised ports. Make T032 pass. (depends on T033)
- [ ] T036 [P] [US1] Implement capture-sidecar/internal/auth/tls.go: mTLS validation for the :9091 control endpoint, reusing the agent's existing cert/CA verification pattern (client cert required, no bearer tokens, per contracts/capture-sidecar.md).
- [ ] T037 [US1] Write capture-sidecar/internal/httpserver/handlers_test.go covering: start with an invalid filter errors before any capture begins; start/stop/status happy path; file download of a completed capture returns valid content. Write before T038.
- [ ] T038 [US1] Implement capture-sidecar/internal/httpserver/handlers.go and capture-sidecar/cmd/main.go: the :9091 HTTP surface per contracts/capture-sidecar.md — `POST /captures/{id}:start` (validates the filter via T035 before touching AF_PACKET), `POST /captures/{id}:stop`, `GET /captures/{id}/status`, `GET /captures/{id}/file` (serves the completed PCAPNG directly from the pre-provisioned emptyDir mount; whether this is reached directly or proxied from the API stays T007's call, not decided here). Wire mTLS from T036. Make T037 pass. (depends on T034, T035, T036)
- [ ] T039 [US1] Add the capture-sidecar/Dockerfile's setcap-survival CI proof (per T009's ruling on blocking-vs-alongside): a CI step/job that builds the capture-sidecar image and runs `getcap` (or equivalent) against the copied binary inside it, proving `cap_net_raw` survived the multi-stage COPY.
- [ ] T040 [US1] Add Kubernetes-client helpers in api/internal/kube/capture.go: create a NetworkCapture CR (start), patch it to request a stop (stop), read its status (get), and list NetworkCaptures owned by a GameServer (list) — used by the REST handlers. No CRD mutation beyond what start/stop/status/list needs.
- [ ] T041 [US1] Create operator/internal/controller/networkcapture_controller.go: `NetworkCaptureReconciler` driving Pending→Running→Completed/Failed — injects the ephemeral capture container via `pods/ephemeralcontainers` when a NetworkCapture is created for an opted-in GameServer, calls the sidecar's `:start` over mTLS through the `<gs>-agent` Service, watches the sidecar's status endpoint for completion or hard-limit auto-stop, marks Completed with `fileSizeBytes` on success or Failed with an error otherwise, and enforces one Running capture per GameServer (a second start while one is Running is rejected). US1 happy path only — TTL/GC reconciliation is Phase 6 (US4). (depends on T017, T018, T038)
- [ ] T042 [US1] Add operator/internal/controller/networkcapture_envtest_test.go covering the Pending→Running→Completed transition on envtest: start, hard-limit-triggered auto-completion, and the one-Running-capture-per-GameServer rejection (phase does not change, no second ephemeral-container injection attempted). (depends on T041)
- [ ] T043 [US1] Add api/internal/handlers/capture.go implementing `POST /servers/{name}:capture-start` (validates filter/duration/size, writes the NetworkCapture CR via T040, 409 if one is already Running) and `POST /servers/{name}:capture-stop`, guarded by the `captures:manage` RBAC rule (T023). Each handler calls the synchronous audit-write method (T029) before the response is returned, per FR-006. Mount both routes in api/cmd/main.go. (depends on T040, T023, T029)
- [ ] T044 [US1] Add the `GET /servers/{name}:capture-file` download handler to api/internal/handlers/capture.go per T007's resolved download path — either dial the sidecar's `:9091 GET /captures/{id}/file` over mTLS through the `<gs>-agent` Service and stream the body through unmodified, or proxy through the agent route T007 identified. Set `Content-Type`/`Content-Disposition` for `.pcapng`. Write the audit event (T029) synchronously before streaming. Reject downloads for captures not `Completed` or past retention with 404 (SC-004). Mount the route in api/cmd/main.go. (depends on T007, T038, T043)
- [ ] T045 [US1] Add `GET /servers/{name}:captures` (list) and `GET /servers/{name}:capture?id=` (get single) handlers to api/internal/handlers/capture.go, mounted in api/cmd/main.go, gated by the `captures:manage` RBAC rule (T023): list returns all NetworkCaptures owned by the GameServer per contracts/rest-api.md's response shape; get returns one by id, 404 if not found. Neither route had an implementing task before this one, despite T023 already declaring their RBAC rules. (depends on T040, T023)
- [ ] T046 [US1] Add api/internal/handlers/capture_envtest_test.go covering the start/stop/download happy path with an admin token, the 409-on-concurrent-start case, and the mandatory operator-role-403 regression test (an OPERATOR-role token, not merely a viewer, hitting `:capture-start`/`:capture-stop`/`:capture-file` must get 403), proving T023's RBAC ordering holds end-to-end through the real router. (depends on T043, T044)
- [ ] T047 [P] [US1] Update api/specs.md with a capture section documenting the three US1 endpoints, the `captures:manage` RBAC gate, and the synchronous pre-response audit write.
- [ ] T048 [P] [US1] Update operator/specs.md with the NetworkCaptureReconciler's responsibilities, the injection design, and the one-Running-capture-per-GameServer invariant.
- [ ] T049 [US1] Add `TestGameServer_NetworkCaptureStartStopDownload` to test/e2e/gameserver_e2e_test.go, and add its name to the operator bucket in test/e2e/buckets.sh in this same commit (per the stray bucket-vs-suite check in buckets.sh's `verify()`, listing the name before the test function exists turns the "e2e bucket coverage" job red): an opted-in GameServer, start a capture with a filter, generate BOTH filter-matching traffic and deliberately non-matching traffic against the server's advertised port, stop the capture (and, in a second sub-case, let a tiny max-size/max-duration limit trigger the stop automatically), download the file and assert (SC-008) it contains zero non-matching packets. Assert SC-001 (third-party readability) by shelling out to `tshark`/`capinfos` against the downloaded file, not by re-reading it with the same `gopacket/pcapgo` that wrote it. Gated on T039 passing, since a silently-missing `CAP_NET_RAW` would make this test either falsely pass (empty capture) or hang. (depends on T043, T044, T041, T039)

**Checkpoint**: User Story 1 is fully functional and independently testable — a capture can be started, recorded, stopped, and downloaded end to end.

---

## Phase 4: User Story 2 - Enable/disable the capture capability per GameServer (Priority: P2)

**Goal**: An admin can turn the capture capability on or off for a specific GameServer without restarting the game process, and the setting persists across pod rebuilds.

**Independent Test**: Enable capture on a running GameServer and confirm an ephemeral capture container appears without disturbing the game container; disable it and confirm `status.capture.ready` flips false immediately; delete/recreate the pod with capture still enabled and confirm the ephemeral container reappears.

### Tests for User Story 2

- [ ] T050 [P] [US2] Add unit tests to api/internal/handlers/capture_test.go for the enable/disable handlers: successful enable (200, correct body), enable on a terminating server (409), enable when the cluster-wide capture feature is disabled (501), successful disable (200, `ready=false`, `activeCapture=null`), disable when already disabled (409), not-found (404) for both. Use an OPERATOR-role token for at least one rejection case to prove the RBAC ordering doesn't regress.

### Implementation for User Story 2

- [ ] T051 [US2] In operator/internal/controller/gameserver_controller.go, extend the ephemeral-container injection path: when `spec.capture.enabled` transitions to true, patch `pods/ephemeralcontainers` to add the capture sidecar (mounting the T017 emptyDir, no new volume) without touching `pod.spec.volumes` or restarting the game container. When it transitions to false, stop any active capture (patch owned NetworkCapture(s) with `phase != Completed` to a terminal state) and set `status.capture.ready=false`/`activeCapture=null` immediately, WITHOUT attempting to remove the ephemeral container (no Kubernetes API for that) — a reconcile of a disabled GameServer with a lingering ephemeral container entry must not error or loop.
- [ ] T052 [US2] Extend operator/internal/controller/networkcapture_envtest_test.go with: (a) enabling capture injects an ephemeral container with the game container's spec/restart-count unchanged; (b) disabling sets `status.capture.ready=false`/`activeCapture=null` immediately while the envtest pod's `ephemeralContainerStatuses` entry remains present (asserting the platform limitation, not fighting it); (c) deleting and recreating the Pod with `spec.capture.enabled` still true re-injects the ephemeral container into the new pod. (depends on T051)
- [ ] T053 [US2] Add `POST /servers/{name}:capture-enable` and `POST /servers/{name}:capture-disable` handlers to api/internal/handlers/capture.go per contracts/rest-api.md (enable preconditions: server exists and not terminating → 409; capture disabled cluster-wide → 501; disable preconditions: server exists, capture currently enabled else 409). Both patch `spec.capture.enabled` and call T029's synchronous audit write before responding. Mount both routes in api/cmd/main.go, gated by the `captures:manage` RBAC rule (T023). Make T050 pass. (depends on T014, T029, T023)
- [ ] T054 [US2] Set the capture ephemeral-container's `securityContext` in operator/internal/controller/gameserver_controller.go's injection code to `runAsNonRoot: true`, `allowPrivilegeEscalation: true` (file capabilities are ignored under `no_new_privs`), per T008's ruling on the PodSecurity "restricted" exception (Open Decision 2) — the Dockerfile's setcap stage itself is T003's, already written.
- [ ] T055 [P] [US2] Update operator/specs.md and api/specs.md with the GameServer `spec.capture`/`status.capture` extensions, the injection/no-removal-on-disable semantics, and the two new REST endpoints.
- [ ] T056 [US2] Add `TestGameServer_NetworkCaptureEphemeralContainer` to test/e2e/gameserver_e2e_test.go, and add its name to the operator bucket in test/e2e/buckets.sh in this same commit (same stray-check hazard as T049 — see buckets.sh's `verify()`): create a GameServer without capture, verify no ephemeral container; enable and verify an ephemeral container appears with the game container's containers list/restart count unchanged; disable and verify `status.capture.ready` flips false while the ephemeral container entry is still observably present; delete/recreate the pod with capture still enabled and verify the ephemeral container reappears. Structural only, no real packet traffic. (depends on T051, T053)

**Checkpoint**: User Stories 1 and 2 both work independently — capture can be enabled/disabled per server on top of the working start/stop/download flow.

---

## Phase 5: User Story 3 - Admin-only access and full auditing (Priority: P2)

**Goal**: Only admins can manage captures, every capture operation is auditable, and the RBAC ordering hazard (operator role silently gaining access via the `servers:write` catch-all) cannot regress unnoticed.

**Independent Test**: An operator-role token gets 403 on all 8 capture routes while retaining its normal `servers:write` access to non-capture routes; an admin token succeeds; each of enable/disable/start/stop/download/delete produces exactly one correctly-shaped audit row.

### Tests for User Story 3

- [ ] T057 [P] [US3] Add api/internal/handlers/capture_rbac_envtest_test.go: mint an OPERATOR-role token and assert each of the 8 capture routes returns 403 while a control request to a non-capture `/servers` route with the same token still succeeds (proving the 403 is capture-specific); mint an admin token and assert each of the 8 routes is RBAC-reachable (downstream 404/409 from missing fixtures is acceptable); assert a successful enable/start/stop/disable/delete each produce exactly one new `audit_events` row with the correct Method/Path/Target/Status/Reason per contracts/audit-events.md section 3.
- [ ] T058 [US3] Add a failure-path case to api/internal/handlers/capture_rbac_envtest_test.go: point the Auditor at a broken/closed DB handle (or the failure seam T029 exposes) and assert a capture operation still returns its normal HTTP status while a warning-level log line is emitted — exercising the real T029 write path, not a mock. (depends on T057)

### Implementation for User Story 3

- [ ] T059 [US3] Extend api/internal/rbac/catalog_test.go's cross-check (catalog vs rbac.go vs migrations) to cover the `captures:manage` key added in T022/T025, if that generic test iterates all catalog keys already — verify it does before assuming a new case is needed.
- [ ] T060 [US3] Wire the `DELETE /servers/{name}:capture` route and its handler into api/internal/handlers/capture.go and api/cmd/main.go (the last of T023's 8 routes — the other seven are built by T043/T044/T045/T053): deletes a completed/failed NetworkCapture CR and its backing file, guarded by `captures:manage`, auditing via T029 before responding.
- [ ] T061 [US3] DECISION-GATED (per T010's ruling): implement or explicitly defer handler-side audit writes for `GET :captures` (list) and `GET :capture` (get single status) in api/internal/handlers/capture.go — both bypass `Middleware` since `shouldLog` returns false for GET, and both handlers already exist from T045. If T010 says list is in scope, call T029's write method from the list handler only, matching rest-api.md's audit-event claim; get-single-status stays unaudited per audit-events.md 3.1 unless T010 extends further. If out of scope, document the decision in api/specs.md instead of writing code. (depends on T045)
- [ ] T062 [US3] Add `TestAPI_RBAC_OperatorCannotReachCaptures` to test/e2e/api_rbac_matrix_e2e_test.go (extending the file's existing `TestAPI_RBAC_OperatorCanWriteServers_NotUsers`-style pattern), and add its name to the api-rbac bucket in test/e2e/buckets.sh in this same commit (same stray-check hazard as T049): assert an operator-role token gets 403 on all 8 capture routes while retaining normal `servers:write` access, and an admin token is not RBAC-blocked on the same routes. Reuse the file's already-established admin/operator sessions rather than logging in fresh (bucket login budget). (depends on T057)
- [ ] T063 [P] [US3] Update api/specs.md with a capture-auditing section: the audited operations, the `captures:manage` grantability resolution (T012), the rule-table ordering invariant, the synchronous audit-write method's "never fails the operation" contract, and the migration numbers used (007_audit_reason.sql, 008_captures_rbac.sql).

**Checkpoint**: Admin-only access and full auditing hold across every capture route, with a regression test protecting the RBAC ordering fix.

---

## Phase 6: User Story 4 - Captures auto-expire after the retention window (Priority: P2)

**Goal**: Completed captures (and their files) are automatically deleted once their retention window elapses, and expired captures become inaccessible before that cleanup even runs.

**Independent Test**: Create a completed NetworkCapture with a short TTL; confirm it transitions to Expired and is deleted within a bounded window, its backing file is gone, and a list call omits it; confirm a still-Running capture whose TTL elapses is terminated and cleaned up rather than skipped.

### Tests for User Story 4

- [ ] T064 [US4] Add table-driven unit tests in operator/internal/controller/networkcapture_controller_test.go for the expiry arithmetic: `expiresAt = completionTime + ttlSecondsAfterFinished`, `isExpired(now)`, and `clampRetention(requested, clusterMax, clusterDefault)` — covering nil TTL falling back to the cluster default, an under-max TTL passing through unchanged, an over-max TTL clamping to max, zero/negative TTL rejected, and the exact `completionTime+ttl==now` boundary.

### Implementation for User Story 4

- [ ] T065 [US4] Verify that `TTLSecondsAfterFinished` on NetworkCaptureSpec (operator/api/v1alpha1/networkcapture_types.go, from T013) and `RetentionSeconds` on the nested CaptureConfiguration (operator/api/v1alpha1/gameserver_types.go, from T014) carry the T011-ratified cluster-max as their kubebuilder validation ceiling and a 24h default (per data-model.md's FR-007 constraint — do not let the default drift to 7 days). These fields and their codegen were already added in T013/T014/T015; this task confirms the ratified max landed correctly, it does not re-declare the fields or re-run `make generate`/`make manifests`.
- [ ] T066 [US4] Implement the expiry-arithmetic helpers exercised by T064 in operator/internal/controller/networkcapture_controller.go, making T064 pass.
- [ ] T067 [US4] Implement the operator's retention reconciliation pass in `NetworkCaptureReconciler` (operator/internal/controller/networkcapture_controller.go): on each reconcile and via a periodic `RequeueAfter` (research.md's 60s interval), scan Completed/Failed NetworkCaptures and transition any past `completionTime + ttlSecondsAfterFinished` to Expired, delete the backing file from the capture emptyDir, then delete the CR. Handle the edge case of a Running capture whose retention window elapses with no `completionTime` set — stop it and clean up its partial file rather than skipping it. (depends on T066)
- [ ] T068 [US4] Extend operator/internal/controller/networkcapture_envtest_test.go: a Completed capture with a short TTL transitions to Expired and is deleted within a bounded `Eventually()` window (short-circuit the requeue constant for the test rather than waiting the full interval); a Running capture whose TTL elapses is terminated; a per-server `RetentionSeconds` override above the cluster max is clamped when its NetworkCapture is created. (depends on T067)
- [ ] T069 [US4] Wire cluster-max clamping into api/internal/handlers/capture.go's start handler: validate the requested `ttlSecondsAfterFinished` against the Helm-sourced cluster max (T019/T020) and the GameServer's `RetentionSeconds` override, returning 400 with the documented `requested retention Xs exceeds cluster maximum Ys` body when exceeded, defaulting to 86400 when omitted. Add unit tests in api/internal/handlers/capture_test.go: omitted TTL defaults to 24h, under-max passes, over-max 400s, per-server override honored.
- [ ] T070 [US4] Make List/Get/Download in api/internal/handlers/capture.go honor expiry: List omits any Expired-phase (or already-deleted) NetworkCapture; Get and Download return 404 with `capture '{id}' not found or has expired` for an Expired or nonexistent capture. Add unit tests: list omits Expired, download of Expired 404s, download of a still-Completed capture succeeds. (depends on T045, T067)
- [ ] T071 [US4] Add `TestNetworkCapture_RetentionExpiry` to test/e2e/networkcapture_retention_test.go, and add its name to the operator bucket in test/e2e/buckets.sh in this same commit (same stray-check hazard as T049): create a GameServer with capture enabled, create a completed NetworkCapture with a short TTL, assert via `Eventually()` the CR is deleted and its backing file is gone, and a subsequent list call omits it. (depends on T067, T069, T070)
- [ ] T072 [P] [US4] Update operator/specs.md with the retention/expiry behavior actually implemented: TTL fields, the requeue interval, the cluster-max clamp, and Running-capture-terminated-at-expiry handling.

**Checkpoint**: Captures expire and clean up automatically, and expired captures are inaccessible even before the GC pass runs.

---

## Phase 7: User Story 5 - Edge cases: restart mid-capture, concurrency, size/duration limits (Priority: P3)

**Goal**: Captures behave correctly under adverse conditions — disk full, pod restart, concurrent start requests, and a crashed sidecar — without corrupting data, leaking state, or destabilizing the game server.

**Independent Test**: Delete the game pod mid-capture and confirm the NetworkCapture fails cleanly with no orphaned state and the server returns to Running; issue two concurrent start requests and confirm exactly one succeeds with a clean 409 on the other; exhaust disk space mid-capture and confirm a clean stop with the partial file removed, not a crash.

### Tests for User Story 5

- [ ] T073 [P] [US5] Add capture-sidecar/internal/capture/writer_test.go cases for max-size auto-stop (clean finalize, no corruption) and simulated `ENOSPC` (clean "disk full" stop, partial file removed, no panic).
- [ ] T074 [P] [US5] Add capture-sidecar/internal/httpserver/handlers_test.go cases for max-duration auto-stop: a capture started with a short `maxDurationSeconds` transitions itself to stopped without an explicit `:stop`, `GET /captures/{id}/status` reflects the stopped state and final counts, and packets collected so far are preserved intact.
- [ ] T075 [P] [US5] Add api/internal/handlers/capture_test.go cases: a start request with a syntactically invalid BPF filter is rejected 400 with the syntax-error detail and creates no NetworkCapture CR; a start request against a GameServer whose `status.capture.activeCapture` is already set is rejected 409 immediately, without a Kubernetes create call, exercising the validation order (filter compiles → concurrency check → TTL).
- [ ] T076 [P] [US5] Add operator/internal/controller/networkcapture_envtest_test.go cases: (a) a second Pending NetworkCapture for a GameServer that already has one Running is failed with reason `capture_already_in_progress`, not allowed to run alongside it; (b) a Running NetworkCapture whose Pod is deleted-and-recreated (UID mismatch or not found) transitions to Failed with a `PodRestarted`-style condition and clears `status.capture.activeCapture`; (c) a Running NetworkCapture whose ephemeral capture container status shows `Terminated` (non-zero exit) while the Pod itself is still Running transitions to Failed with a `SidecarCrashed` condition and is never retried.

### Implementation for User Story 5

- [ ] T077 [US5] Implement max-size auto-stop and `ENOSPC` handling in capture-sidecar/internal/capture/writer.go: track cumulative bytes against `maxSizeBytes` and finalize on limit; wrap the file-write path to detect `syscall.ENOSPC` specifically, stop the capture, delete the partial file, and return a distinguishable "disk full during capture" error. Make T073 pass. (depends on T034)
- [ ] T078 [US5] Implement the max-duration timer and auto-stop lifecycle in capture-sidecar/internal/httpserver/handlers.go: on a successful `:start`, arm a timer for `maxDurationSeconds`; on expiry, stop the AF_PACKET loop, finalize via the same path an explicit `:stop` uses, and update in-memory status so `GET /status` reflects the stopped state without a caller-issued `:stop`. Make T074 pass. (depends on T038)
- [ ] T079 [US5] Implement the filter-compile check and API-tier concurrency fast-path in api/internal/handlers/capture.go's start handler: compile the filter (T035) before constructing the CR, 400 on failure; check the GameServer's `status.capture.activeCapture` (and/or owned NetworkCaptures) and 409 immediately if one exists, before any CR create. Comment that this is a fast-path convenience only — the authoritative lock is T080, since two concurrent POSTs can both pass this check before either CR exists. Make T075 pass. (depends on T043)
- [ ] T080 [US5] Implement the authoritative concurrency lock and failure-detection behaviors in operator/internal/controller/networkcapture_controller.go: on the Pending→Running transition, list NetworkCaptures owned by the same GameServer and fail any but the earliest-created Pending/Running one with reason `capture_already_in_progress`; record the Pod UID observed at capture start so a later reconcile can detect recreation; on each Running reconcile, check the Pod's existence/UID and the capture ephemeral container's status, transitioning to Failed (`PodRestarted` or `SidecarCrashed`) as appropriate; on any terminal transition, clear the parent GameServer's `status.capture.activeCapture`. Make T076 pass. (depends on T041)
- [ ] T081 [P] [US5] Update operator/specs.md with an edge-cases section: the concurrency lock's true location (reconciler, not the API handler) and why, pod-restart/eviction detection and cleanup, and the sidecar-crash terminal handling with no retry.
- [ ] T082 [P] [US5] Update capture-sidecar/specs.md with an edge-cases section: the writer's `ENOSPC`/max-size auto-stop semantics and the HTTP server's max-duration auto-stop lifecycle.
- [ ] T083 [US5] Add `TestGameServer_NetworkCaptureRestartCleanup` to test/e2e/gameserver_e2e_test.go, and add its name to the operator bucket in test/e2e/buckets.sh in this same commit (same stray-check hazard as T049): with a capture Running, delete the game pod; assert the NetworkCapture transitions to Failed, no orphaned ephemeral-container state or capture-data survives once the replacement pod is Running, and the GameServer returns to a playable state with `status.capture.activeCapture` cleared. (depends on T080)
- [ ] T084 [US5] Add `TestGameServer_NetworkCaptureConcurrencyRejected` to test/e2e/gameserver_e2e_test.go, and add its name to the operator bucket in test/e2e/buckets.sh in this same commit (same stray-check hazard as T049): as an admin, start a capture, then immediately issue a second `:capture-start` against the same server; assert the second returns 409 with the exact message from contracts/rest-api.md, and the first capture proceeds to Completed unaffected. (depends on T079, T080)

**Checkpoint**: All five user stories are independently functional; the feature handles its documented edge cases without data corruption or leaked state.

---

## Phase 8: Polish & Cross-Cutting

**Purpose**: Dashboard UI, the SC-002 benchmark, documentation, the upgrade note, and image signing — spanning all stories.

- [ ] T085 [P] Add NetworkCapture and CaptureConfiguration/CaptureStatus TypeScript types to web/src/types.ts, mirroring data-model.md's CRD fields and the nested `status.capture.{ready,activeCapture,lastCaptureTime,sidecarRestarts}`.
- [ ] T086 Add the 8 capture REST client calls to web/src/lib/api.ts per contracts/rest-api.md, following the file's existing `api()`/`withNS`/CSRF conventions; route the download call through a single indirection point matching T007's resolved path. (depends on T085, T007)
- [ ] T087 Build web/src/components/CaptureWidget.tsx per the "Gameplane/Capture Warning Banner" and the shared capture-list/start-modal pieces in design-export/json/f0s9zG.json and the Bbnga/dBILX/xvlB6/m5kOm4/O08uaD/b4eaUf screens: warning banner, status badge, elapsed/remaining progress bars, BPF filter input with the invalid-filter error state. Read the design-export JSON/screenshots for exact copy and layout first (never read design.pen directly). (depends on T085)
- [ ] T088 Add a Capture tab to web/src/routes/ServerDetail.tsx's tab bar, positioned between `backups` and `settings` (per design-export/MANIFEST.md), rendering CaptureWidget-backed content for the enabled/empty/running/list states. (depends on T087)
- [ ] T089 Extend web/src/routes/ServerDetail.test.tsx: the Capture tab renders between Backups and Settings, the disabled-state copy matches the Bbnga screen, and the start-capture modal shows the invalid-BPF-filter error state (red border, disabled Start button, exact error copy). (depends on T088)
- [ ] T090 Create web/src/routes/tabs/settings/NetworkCapture.tsx (Enable Capture switch, admin-access warning text, Retention Window input + unit select + cluster-max note, Discard/Save footer per design-export/json/RodrS.json) and register it in web/src/routes/tabs/Settings.tsx's `SECTIONS` array between `backups` and `placement`. (depends on T086)
- [ ] T091 Add web/src/routes/tabs/settings/NetworkCapture.test.tsx: the sub-nav entry renders in the correct position, Save/Discard wiring, and a non-admin (operator-role) session renders the section read-only/disabled per FR-005/SC-005. (depends on T090)
- [ ] T092 [P] Confirm (do not assume) that web/src/router/tree.tsx routes `ServerDetailPage` as a single route with tab/section state held client-side, the same as the existing Backups/Settings tabs — meaning no new tree.tsx entries are needed for the Capture tab or the Network-capture settings section. Record the confirmation in the PR description; if false, file a follow-up task instead of editing blind.
- [ ] T093 Update web/specs.md (Constitution Principle IV) documenting the dashboard's capture surface: the CaptureWidget component, the Capture server-detail tab and its settings sub-nav entry, the TypeScript types mirroring the NetworkCapture/CaptureConfiguration/CaptureStatus CRD fields, and the download-path indirection point from T007. (depends on T087, T090)
- [ ] T094 [P] Run the SC-002 (zero perceptible player impact) benchmark as a documented manual procedure against the maintainer's live cluster (not CI — kind runners exclude heavy game images and lack stable-enough network conditions): paired capture-ON/capture-OFF GameServers, real join-bot traffic, packet-loss/latency comparison per quickstart.md's "SC-002" scenario. Record the pass/fail threshold and the actual result back into specs/003-network-capture-sidecar/quickstart.md.
- [ ] T095 [P] Update docs/architecture.md: add a "capture-sidecar (optional)" entry to the component list/table (mirroring sentinel/mcp-server), describing the ephemeral-container injection model, the pre-provisioned emptyDir, and the `:9091` via `<gs>-agent` Service addressing (D-ADDRESSING).
- [ ] T096 [P] Update README.md's component list with one line on capture-sidecar: opt-in per GameServer (`spec.capture.enabled`).
- [ ] T097 Update docs/security.md per T008's ruling: document the PodSecurity "restricted" exception the capture ephemeral container needs (`allowPrivilegeEscalation: true` for setcap-granted `CAP_NET_RAW`), scoped only to that container, never the game container. (depends on T008)
- [ ] T098 [P] Update docs/install.md with the new Helm values from charts/gameplane/values.yaml's `capture.*` block: defaults and the admin-only capture-management model.
- [ ] T099 [P] Update CLAUDE.md's repo map: add `capture-sidecar/` to the top-level tree listing alongside `sentinel/`, `mcp-server/`, with a one-line description.
- [ ] T100 Add an UPGRADE NOTE to CHANGELOG.md's unreleased section: this feature unconditionally adds a pre-provisioned capture emptyDir to every game pod's template (T017, human Decision 2), changing every existing GameServer's pod spec and triggering a one-time rolling restart of all running game servers on upgrade — an explicit operator-facing warning to read before upgrading.

**Checkpoint**: Dashboard surfaces the feature, documentation and upgrade guidance are complete, and the SC-002 measurement has a recorded result.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately.
- **Foundational (Phase 2)**: Depends on Setup (needs the capture-sidecar module and go.work entry from T001 for T002/T003). BLOCKS all user stories.
- **User Story 1 (Phase 3, P1/MVP)**: Depends on Foundational completion only.
- **User Story 2 (Phase 4, P2)**: Depends on Foundational; touches `gameserver_controller.go`/`capture.go`/`api/cmd/main.go` also touched by US1 — sequence edits to those files after US1 lands, or hold both in the same branch carefully. Functionally independent of US1's start/stop/download logic.
- **User Story 3 (Phase 5, P2)**: Depends on Foundational's RBAC/audit groundwork (T022-T029); its 403/audit regression tests exercise routes built in US1/US2, so run this phase after Phase 3 (and ideally Phase 4) for a meaningful test target, though its own new code (DELETE route, GET-audit decision, RBAC regression test) is otherwise independent.
- **User Story 4 (Phase 6, P2)**: Depends on Foundational's CRD types (T013) and the NetworkCaptureReconciler skeleton (T041 from US1) to extend with TTL/GC logic.
- **User Story 5 (Phase 7, P3)**: Depends on the writer (T034), HTTP server (T038), start handler (T043), and reconciler (T041) from US1 to extend with edge-case handling.
- **Polish (Phase 8)**: Depends on the REST surface (US1/US2) existing for the dashboard tasks; T094/T095/T096/T098/T099/T100 have no code dependency and can start anytime.

### Within Each User Story

- Tests are written before their corresponding implementation task (see each task's file and dependency).
- CRD/type changes precede the controller/handler code that reads them.
- Sidecar core (afpacket/writer/filter) precedes the HTTP server that wires them together.
- Handlers depend on the RBAC rule (T023) and audit method (T029) existing.
- Envtest coverage follows its controller/handler implementation; e2e tests follow the full vertical slice they exercise.

### Parallel Opportunities

- All `[P]` Setup tasks (T002-T006) can run together once T001 lands.
- The six decision spikes (T007-T012) are fully parallel with each other and with T001.
- T019, T020, T021 (operator flags, Helm values, specs.md) run in parallel once their file-level dependencies land.
- Within US1: T031/T032 (tests) in parallel; T036 (mTLS) in parallel with T033-T035 (capture pipeline).
- Within US5: T073-T076 (all test-writing tasks) are fully parallel with each other.
- Across stories: once Foundational is done, US1 and the non-file-colliding parts of US2/US4 can be staffed in parallel, with US2/US3/US4/US5's file-colliding edits (gameserver_controller.go, capture.go, networkcapture_controller.go) sequenced against whichever story reaches that file first.

---

## Parallel Example: User Story 1

```bash
# Tests, launched together:
Task: "writer_test.go — normal/size-limit/duration-limit stop produce valid PCAPNG"
Task: "filter_test.go — valid/invalid/default/empty-match/some-match filter behavior"

# Independent implementation pieces, launched together once tests exist:
Task: "capture-sidecar/internal/auth/tls.go — mTLS validation for :9091"
Task: "operator/cmd/main.go — capture-defaults CLI flags"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories; resolve or explicitly park the six decisions).
3. Complete Phase 3: User Story 1.
4. **STOP and VALIDATE**: start/record/stop/download works end to end on a real cluster.
5. Ship the MVP behind `capture.enabled: false` by default.

### Incremental Delivery

1. Setup + Foundational → foundation ready.
2. + User Story 1 → validate independently → MVP.
3. + User Story 2 → validate independently (enable/disable persists across rebuild).
4. + User Story 3 → validate independently (403 regression test + audit rows).
5. + User Story 4 → validate independently (expiry/GC).
6. + User Story 5 → validate independently (edge cases).
7. + Polish → dashboard, docs, benchmark, release wiring.

---

## OPEN DECISIONS

The maintainer has not yet resolved these six items. Do not silently pick an answer — the tasks below are the only ones authorized to record a resolution, and everything they gate must wait.

1. **Download path** (proxy through a new agent endpoint vs. stream direct from the sidecar's `:9091 /file`) — gated by **T007**. Blocks: T044 (US1 download handler), T023's `/ws`-catch-all insertion if applicable, T086 (dashboard API client indirection).
2. **PodSecurity "restricted" exception ownership** (who documents/owns the `allowPrivilegeEscalation: true` exception) — gated by **T008**. Blocks: T054 (securityContext edit), T097 (docs/security.md).
3. **Setcap-survives-distroless-COPY CI proof: blocking or alongside** — gated by **T009**. Blocks: T003/T039's sequencing (whether the Dockerfile capability step must be proven green before merge or can land with a parallel proof), and by extension T049's e2e gate.
4. **GET-operation auditing scope** (is auditing `list`/`get-status` in scope, given `shouldLog` excludes all GETs but rest-api.md's own table claims `:captures` is audited) — gated by **T010**. Blocks: T061 (US3 GET-audit implementation or explicit deferral).
5. **Cluster-max retention value** (research.md's unratified 90-day/7776000s proposal needs maintainer ratification; migration numbering for `007_audit_reason.sql`/`008_captures_rbac.sql` is settled per plan.md and no longer gated here) — gated by **T011**. Blocks: T013/T065 (TTL field's validation ceiling), T020 (Helm `maxRetentionSeconds`).
6. **`captures:manage` grantability** (amend FR-005 to allow custom-role grants, or build a non-grantability mechanism) — gated by **T012**. Blocks: T022 (catalog entry), T025 (grant migration), T063 (audit specs documenting the resolution).
