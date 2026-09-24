# Review: capture-sidecar

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `capture-sidecar/specs.md`; `specs/done_003-network-capture-sidecar/contracts/capture-sidecar.md` (filter, 409, endpoint sections, via grep); the `NetworkCaptureSpec.Filter` doc in `operator/api/v1alpha1/networkcapture_types.go:38-44`; `charts/gameplane/values.yaml:522-548`; operator wiring in `operator/internal/controller/gameserver_controller.go`, `operator/internal/controller/networkcapture_controller.go` and `operator/internal/agent/sidecar_capture.go`

held candidates: 0 (see OD-019)

## Scope reviewed

Read in full:
- `capture-sidecar/cmd/main.go`
- `capture-sidecar/internal/auth/tls.go`
- `capture-sidecar/internal/capture/afpacket.go`, `filter.go`, `writer.go`
- `capture-sidecar/internal/httpserver/handlers.go`
- `capture-sidecar/specs.md`, `Dockerfile`, `go.mod`, `.testcoverage.yml`
- `operator/internal/agent/sidecar_capture.go` (the operator's client for the sidecar)

Read in part:
- `capture-sidecar/internal/httpserver/handlers_test.go`: the test list and lines 534-554
- `capture-sidecar/internal/{auth,capture}/*_test.go`: test function names only
- `operator/internal/controller/gameserver_controller.go`: lines 960-990 (the agent Service's 9091 port), 1420-1437 (the `captures` emptyDir), 1550-1757 (ephemeral container spec, `reconcileCapture`)
- `operator/internal/controller/networkcapture_controller.go`: lines 25-80, 340-420, 665-714
- `operator/internal/controller/agent_certs.go:189` (per-pod cert EKU)
- `api/internal/handlers/capture.go`: lines 143-147, 245-262, 836-848, 1329-1371 (grep and excerpts)
- `web/src/components/CaptureWidget.tsx`: the filter handling (grep)
- `test/e2e/gameserver_e2e_test.go`, `test/e2e/networkcapture_retention_test.go`: filter usage (grep)
- `docs/security.md:159-166`
- gopacket `pcapgo/ngwrite.go` in the module cache (v1.7.1; go.mod pins v1.7.2), for the timestamp resolution

Not reviewed: the bodies of the capture-sidecar tests; `api/internal/handlers/capture.go` beyond the excerpts above (the api chunk covers it).

Compile check: `go build ./...` in `capture-sidecar/` succeeds.

## Method

I compared each statement in `specs.md` (responsibilities, endpoints, env vars, file format, limits, invariants, edge cases, security considerations) with the code, then followed the control plane. First the API request, then the NetworkCapture CR, then the operator's `StartCapture`/`StopCapture`/`DeleteCaptureFile`, then the sidecar handler. That showed which inputs actually reach the sidecar and which sidecar behaviours the control plane relies on. I traced the error paths in `writer.go` and `handlers.go` (ENOSPC wrapping, finish ordering, idempotency). No tests or linters were run.

## Observations (no finding)

- The mTLS listener uses `RequireAndVerifyClientCert` with TLS 1.2 minimum (`tls.go:34-39`). Per-pod certs signed by the operator carry only `ExtKeyUsageServerAuth` (`agent_certs.go:189`), so they cannot pass client verification. This matches `specs.md:233`.
- Capture IDs are checked against `^[A-Za-z0-9_-]{1,64}$` before any path is built (`handlers.go:78`, `:274-276`), and containment is checked again in `captureFilePath` (`:290-299`).
- The sidecar refuses an empty filter and filters that compile to no packet test, including unsupported protocol keywords (`handlers.go:378-388`; `filter.go:69-134`). This matches contract line 250 ("never treats 'no filter' as 'capture everything'").
- Only the capture ephemeral container mounts the `captures` emptyDir (`gameserver_controller.go:1424-1427`, `:1613-1620`), as `specs.md:239` says.
- The ephemeral container's security context matches `specs.md` invariant 3: Drop ALL, Add NET_RAW, `allowPrivilegeEscalation: true`, non-root, read-only root filesystem (`gameserver_controller.go:1576-1612`). The Dockerfile applies `setcap cap_net_raw+ep`.
- PCAPNG timestamp resolution is fixed at nanoseconds by pcapgo (`ngwrite.go:41`, `:274`), matching `specs.md:131`. Default snaplen is 65535 (`writer.go:40`), matching `:129`.
- ENOSPC is wrapped with `%w` all the way through (`writer.go:89`, `:249`, `:300`; `afpacket.go:257-261`), so the `errors.Is(…, syscall.ENOSPC)` checks at `handlers.go:496` and `:560` work.
- `Writer.WritePacket` and `Writer.Close` share `w.mu`, so `finish` closing the writer while the read loop is mid-write is safe.
- Duration-timer pointer identity (`handlers.go:442-444`, `:516-529`) prevents a stale timer from ending a successor capture.
- All e2e capture tests pass an explicit `filter` (`gameserver_e2e_test.go:415`, `:898`, `:988`, `:1113`, `:1177`, `:1291`; `networkcapture_retention_test.go:86`). (see held item C-capture-sidecar-01, OD-019)

## Candidate findings

### C-capture-sidecar-01: held (OD-019)

### C-capture-sidecar-02: Retained capture files add up in the 1 GiB emptyDir, and the sidecar only enforces a per-capture limit
- **Location**: `capture-sidecar/internal/httpserver/handlers.go:365-368`; `operator/internal/controller/gameserver_controller.go:1432-1437`; `capture-sidecar/specs.md:24`, `:241`
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. The `captures` emptyDir is disk-backed with `SizeLimit: 1Gi` (`gameserver_controller.go:1432-1437`). The chart's per-capture default is 900 MiB, "safely under the 1 GiB emptyDir limit" to "prevent pod eviction" (`values.yaml:540-545`).
  2. The sidecar checks only `maxSizeBytes > 0` per request (`handlers.go:365-368`). It never looks at space already used by earlier files on the volume.
  3. Completed files stay on the volume until retention expiry, 24 h by default (`values.yaml:531`), because `expireCapture` is the only caller of `DeleteCaptureFile` (`networkcapture_controller.go:693`). Deleting a capture through the API removes only the CR; the file stays (`api/internal/handlers/capture.go:840-848`).
  4. Run two captures within 24 h, each with `maxSizeBytes` near 600 MiB and a long enough duration or broad enough filter to reach the size limit. Together they pass 1 GiB on the volume.
  5. For a disk-backed emptyDir, going over `sizeLimit` does not return ENOSPC to the writer. The kubelet eviction manager evicts the whole pod ("Usage of EmptyDir volume … exceeds the limit"), which restarts the game server and loses the capture files.
- **Expected**: `specs.md:24` promises to "gracefully stop a capture if the emptyDir volume fills (`ENOSPC` handling)". `specs.md:241` says "The emptyDir's `sizeLimit` enforces a hard disk bound; the sidecar respects it."
- **Actual**: The ENOSPC path never fires for the `sizeLimit` case. Captures that add up past 1 GiB end in a pod eviction, not a `disk_full` capture.

### C-capture-sidecar-03: The TLS env vars the operator sets are ignored; the sidecar reads flags
- **Location**: `capture-sidecar/cmd/main.go:36-41`; `operator/internal/controller/gameserver_controller.go:1627-1631`; `capture-sidecar/specs.md:79`, `:94-98`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:79`: "parses environment variables for certificate paths (TLS_CERT_FILE, TLS_KEY_FILE, TLS_CA_FILE)". `:94-96` marks all three "(required)".
  2. The operator sets exactly those env vars on the ephemeral container (`gameserver_controller.go:1627-1631`).
  3. `main.go` never reads the environment. It uses the flags `-tls-cert`, `-tls-key` and `-tls-client-ca`, whose defaults are `/etc/tls/tls.crt`, `/etc/tls/tls.key` and `/etc/tls/ca.crt` (`main.go:37-39`).
- **Expected**: The sidecar reads the env vars it documents and receives, or `specs.md` and the operator document and pass flags.
- **Actual**: It works only because the env values equal the flag defaults. Changing the mount path or the env values in the operator would have no effect on the sidecar.

### C-capture-sidecar-04: The endpoint paths and endpoint count in specs.md do not match the router, and an API comment repeats the stale table
- **Location**: `capture-sidecar/specs.md:21`, `:53`, `:75`, `:108-117`; `capture-sidecar/internal/httpserver/handlers.go:192-197`; `api/internal/handlers/capture.go:840-845`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:21`: "four operations: start a capture (POST `:start`), stop a capture (POST `:stop`) …". `:114-115` lists `/captures/{id}:start` and `/captures/{id}:stop`.
  2. The router registers `POST /captures/{id}/start` and `POST /captures/{id}/stop`, and also `DELETE /captures/{id}` and `GET /healthz` (`handlers.go:192-197`). The comment at `:184-186` says the `{id}:start` form is invalid for `net/http.ServeMux`. The contract uses `/start` (`contracts/capture-sidecar.md:216`, `:234`) and documents `DELETE /captures/{id}` (`:492-525`).
  3. `api/internal/handlers/capture.go:841-843` says: "there is no sidecar endpoint to delete an individual capture file (capture-sidecar/specs.md's endpoint table lists only :start/:stop/status/file)". The sidecar does have that endpoint, and the operator calls it (`sidecar_capture.go:231-268`).
- **Expected** / **Actual**: `specs.md` should list the real paths and all five capture routes, plus `/healthz`. It lists colon paths and four routes.

### C-capture-sidecar-05: specs.md "Planned" and design statements the code and other sections contradict
- **Location**: `capture-sidecar/specs.md:11`, `:73`, `:183`, `:199`, `:247-257`, `:280`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:11` and `:253`: "Phase 2 Implementation (T067+) will add TTL-based expiry reconciliation and dashboard UI" and "no auto-deletion reconciliation runs yet". Retention expiry exists (`networkcapture_controller.go:680-714` calls `DeleteCaptureFile`), and so does the dashboard UI (`web/src/components/CaptureWidget.tsx`, `web/src/routes/tabs/settings/NetworkCapture.tsx`).
  2. `specs.md:73` (writer package summary): "Detects disk-full conditions (`ENOSPC`) and stops gracefully, deleting partial files." `:211` and the code (`handlers.go:489-491`) keep the partial file. The stale-language disclaimer at `:199` names only "Packet Processing", "Shutdown" and "Security Considerations #5", not `:73`. #5 (`:241`) has in fact already been corrected.
  3. `specs.md:183` (invariant 2): "There is no grace period, no 'one more packet' tolerance." `:148` and `:203` say the packet that crosses `maxSizeBytes` *is* written, and `writer.go:262-269` confirms it.
  4. `specs.md:280` lists key test cases "empty filter fallback, default port-based filter". The sidecar has no fallback or default filter; `CompileFilter("")` is an error (`filter.go:70-72`), and so is an empty request filter (`handlers.go:378-381`).
- **Expected** / **Actual**: `specs.md` should describe one behaviour consistently. It contradicts itself and the code in these four places.

### C-capture-sidecar-06: Dependency drift in specs.md; orphan svcutil replace and Docker copy
- **Location**: `capture-sidecar/specs.md:5`, `:58`, `:85`, `:88`; `capture-sidecar/go.mod:6`, `:11-13`; `capture-sidecar/Dockerfile:4-7`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:5` and `:85` name `github.com/google/gopacket v1.1.19`. `go.mod:6` requires the fork `github.com/gopacket/gopacket v1.7.2`. Code comments cite `@v1.6.1` (`afpacket.go:79`, `writer.go:55`, `:157`), as does `specs.md:207`.
  2. `specs.md:58` and `:88` list `svcutil` as a dependency. `go.mod` has `replace …/svcutil => ../svcutil` (`:13`) but no `require` for svcutil, and no Go file imports it. `main.go:84-86` explicitly says `svcutil.RunHTTP` must not be used. The Dockerfile still copies `svcutil/` "so `go mod download` and the build resolve it" (`Dockerfile:4-7`).
- **Expected** / **Actual**: `specs.md`, `go.mod` and the Dockerfile should agree on the real dependency set. They don't.

### C-capture-sidecar-07: The 409 conflict message names the rejected capture instead of the running one
- **Location**: `capture-sidecar/internal/httpserver/handlers.go:391-394`
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. `cap-aaa` is running on the sidecar.
  2. `POST /captures/cap-bbb/start` arrives, for example from an operator retry after it lost track of `cap-aaa`.
  3. The response is `409 capture 'cap-bbb' already in progress`. `cap-bbb` never started.
  4. The operator treats the 409 as permanent and fails `cap-bbb` with that text (`networkcapture_controller.go:381-389`), so the user sees a message claiming the capture that just failed is "in progress".
- **Expected**: `specs.md:191` and contract `:291` give the message as `capture '{id}' already in progress`. For a different-id conflict, the useful id is the running capture's (`s.currentCapture.id`).
- **Actual**: The message uses the request's id.

### C-capture-sidecar-08: `/healthz` is described as unauthenticated, but the TLS handshake requires a verified client cert and nothing calls it
- **Location**: `capture-sidecar/cmd/main.go:59-61`; `capture-sidecar/internal/httpserver/handlers.go:192`, `:201-207`
- **Category**: dead-code
- **Suggested severity**: S4
- **Observation / repro**:
  1. `main.go:59-60`: "/healthz is the only unauthenticated route". `handlers.go:192` registers it without the middleware.
  2. The server's `TLSConfig` has `ClientAuth: tls.RequireAndVerifyClientCert` (`tls.go:37`). A client without a CA-signed client cert fails the TLS handshake before any route is reached, `/healthz` included.
  3. The container is an ephemeral container, and Kubernetes does not allow probes on ephemeral containers. No code in `operator/` or `api/` requests `/healthz`.
- **Expected** / **Actual**: The comment and route describe an unauthenticated liveness endpoint that no one can reach without mTLS and no one calls. Either it is dead, or the comment misstates it.

### C-capture-sidecar-09: Unused exports in the capture package; a handlers.go comment contradicts the constants below it
- **Location**: `capture-sidecar/internal/capture/writer.go:32-33`, `:65`, `:329-334`; `capture-sidecar/internal/capture/filter.go:136-139`; `capture-sidecar/internal/httpserver/handlers.go:65-73`
- **Category**: dead-code
- **Suggested severity**: S4
- **Observation / repro**:
  1. `PacketWriter.IsLimitReached` / `(*Writer).IsLimitReached`, `LimitReasonUserRequested` and `(*Filter).Expression` have no non-test callers. Grepping outside `_test.go` finds only their declarations.
  2. `handlers.go:65-67`: "The size- and duration-limit reasons are produced by capture.Writer itself and read back through LimitReason, so they are deliberately not duplicated here." The next lines define `reasonMaxDuration = "max_duration_reached"` and `reasonDiskFull = "disk_full"` (`:70-71`), which duplicate `capture.LimitReasonDurationReached` and `capture.LimitReasonDiskFull`. `reasonUserRequested` duplicates the unused `capture.LimitReasonUserRequested`.
- **Expected** / **Actual**: The unused API should be removed or used, and the comment should match the constants. Neither is true today.

## Questions (not findings)

- `HandleStop` stores the caller's `reason` string verbatim as `stoppingReason` (`handlers.go:584-595`). `specs.md:221-227` lists a closed set of values. The operator always sends `user_requested` (`sidecar_capture.go:150`). Should unknown reasons be rejected or normalised?
- On SIGTERM, `FinalizeCurrentCapture` records the stop as `user_requested` (`handlers.go:766`), and the in-flight capture's own goroutine exits through the `ctx.Err() != nil` branch without calling `finish` (`:484-485`). The state is in memory only and the process is exiting, so this is probably unobservable. Is it worth a distinct reason?
- `AFPacketSource.Close` unmaps the ring (`afpacket.go:183`) with no guard against a concurrent `ReadPacketData` on another goroutine (`:143-146`). Production calls them in sequence (`handlers.go:471-481`), but `TestCapturer_Stop_UnblocksReadPackets` exercises concurrent Stop with a mock only. Is the sequential-only contract documented anywhere?
- `specs.md:279` and `:282` list test cases for "AF_PACKET: socket setup, filter application, packet reception, MMap'd buffer semantics" and "invalid/expired certificate rejection". The test names suggest only mock-source Capturer tests and no expired-cert test (`afpacket_test.go`, `tls_test.go`), but I did not read the test bodies.
- The API binary's built-in default `GAMEPLANE_CAPTURE_DEFAULT_MAX_SIZE` is 5368709120 (5 GiB; `api/cmd/main.go:511`), above the 1 GiB emptyDir. The chart overrides it to 900 MiB (`charts/gameplane/templates/api.yaml:376-377`). Only non-chart deployments are affected.
- `main.go:10` and `specs.md:110` refer to `contracts/capture-sidecar.md` with no spec folder. The file is at `specs/done_003-network-capture-sidecar/contracts/capture-sidecar.md`.
