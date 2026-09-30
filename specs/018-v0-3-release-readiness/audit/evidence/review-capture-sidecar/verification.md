# T045 edge chunk: independent verification (opus)

- **Date**: 2026-09-24
- **Input**: `audit/evidence/review-capture-sidecar/notes.md`, 9 candidates. There is no `audit/held/review-capture-sidecar.md`, and the notes record "held candidates: 0".

## Method

I tried to refute each candidate against branch `018-v0-3-release-readiness` at `3de03ab0`. On that branch, `capture-sidecar/`, `operator/internal/`, `api/internal/` and `charts/` have no diff against `origin/master`, so the line numbers below also hold on master. I read all of `capture-sidecar/cmd/main.go`, `internal/httpserver/handlers.go`, `capture-sidecar/specs.md`, `go.mod` and `Dockerfile`. I read the relevant parts of `internal/auth/tls.go`, `internal/capture/{writer,filter}.go`, and the rest of the control plane:
- in the operator: the `captures` emptyDir and the ephemeral-container spec (`gameserver_controller.go:1418-1640`), the NetworkCapture start, running, fail and expiry paths (`networkcapture_controller.go:139-200`, `:340-480`, `:676-716`, `:774-816`), and `operator/internal/agent/sidecar_capture.go`;
- in the API: the create and delete handlers (`api/internal/handlers/capture.go:125-160`, `:245-265`, `:835-870`);
- in the chart and dashboard: `charts/gameplane/values.yaml:522-548` and the dashboard's filter handling (`web/src/components/CaptureWidget.tsx:523`, `:588`).

I compared these with the spec 003 folder (`specs/done_003-network-capture-sidecar/`: spec.md, data-model.md and contracts/capture-sidecar.md). I checked `audit/findings.md` (nothing tracked for capture-sidecar) and every other chunk's notes for duplicates. `go build ./...` in `capture-sidecar/` succeeds. I ran no tests or linters.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-capture-sidecar-01 | rejected | held (OD-019) | held (OD-019) |
| C-capture-sidecar-02 | kept | S3 | Confirmed by reading. Only one capture runs at a time, but finished files stay on the same 1 GiB disk-backed emptyDir until retention expiry (24 h by default). The sidecar checks each request's own `maxSizeBytes` against nothing (`handlers.go:365-368`). A capture deleted through the API leaves its file behind until the pod is recreated: the API removes only the CR (`capture.go:840-848`), and `expireCapture` is the only caller of the file delete (`networkcapture_controller.go:693`). The kubelet enforces `sizeLimit` on a disk-backed emptyDir by evicting the pod, not with ENOSPC. The chart comment at `values.yaml:541-544` itself warns about eviction. The spec folder assumes the opposite: data-model.md:914-920 says the volume "only needs to hold one in-flight file", and contract `:104-107` and `:613` expect ENOSPC at the limit. The workaround is to keep the retained captures under 1 GiB in total, so S3. |
| C-capture-sidecar-03 | kept | S4 | Confirmed. The operator sets `TLS_CERT_FILE`, `TLS_KEY_FILE` and `TLS_CA_FILE` (`gameserver_controller.go:1628-1630`). Nothing in `capture-sidecar/` reads the environment (`grep Getenv\|LookupEnv` finds nothing). The binary reads flags whose defaults happen to be the same paths (`main.go:37-39`). `specs.md:79` and `:94-96` document the env vars as required. It works only by coincidence. |
| C-capture-sidecar-04 | kept | S4 | Confirmed. The router registers `/captures/{id}/start`, `/captures/{id}/stop`, `/status`, `/file`, `DELETE /captures/{id}` and `GET /healthz` (`handlers.go:192-197`). `specs.md:21`, `:53`, `:75` and `:112-117` list colon paths and four routes. The `captureDelete` comment at `api/internal/handlers/capture.go:840-848` says the sidecar has no delete endpoint and cites this stale table, but the operator calls that endpoint (`sidecar_capture.go:227-268`). This is not a duplicate of C-api-15 or C-operator-18, which cover `api/specs.md` and `operator/specs.md`. |
| C-capture-sidecar-05 | kept | S4 | All four contradictions confirmed. (1) `:11` and `:253` call TTL expiry and the dashboard future work, but both exist. (2) `:73` says partial files are deleted, and the `:199` disclaimer does not cover that line. (3) Invariant 2 at `:183` ("no one more packet") contradicts `:148`, `:203` and `writer.go:262-269`. (4) `:280` lists test cases for a default filter the sidecar does not have. |
| C-capture-sidecar-06 | kept | S4 | Kept for the gopacket drift only. `go.mod:6` requires the fork `github.com/gopacket/gopacket v1.7.2`. `specs.md:5`, `:85` and `:127` name `github.com/google/gopacket v1.1.19`. Five "verified against" comments cite `@v1.6.1`: `specs.md:207`, `afpacket.go:79`, `:181` and `writer.go:55`, `:157`. The svcutil half of the candidate (`specs.md:58`, `:88`, the orphan `replace` at `go.mod:11-13`, `Dockerfile:4-7`) is fully covered by C-svcutil-01 in the svcutil chunk. It is not counted twice. |
| C-capture-sidecar-07 | kept | S4 | Confirmed and reachable. When a status poll fails once, the operator fails a Running capture and releases the per-server lock (`networkcapture_controller.go:473-475`, `:797-808`), without stopping the sidecar. The next capture then gets `409 capture '<new id>' already in progress` (`handlers.go:391-394`), and the operator stores that text as the new capture's failure message (`:381-389`). The message names the capture that never started, not the one that is running. FR-012 (`spec.md:133`) asks for "a clear error message". |
| C-capture-sidecar-08 | kept | S4 | Confirmed. `/healthz` skips only the HTTP middleware. The listener's `ClientAuth: tls.RequireAndVerifyClientCert` (`tls.go:37`) rejects the handshake before any route runs. Ephemeral containers cannot have probes, and nothing in `operator/`, `api/` or the chart calls the route. The comments at `main.go:59-60` and `handlers.go:201-202` say the route is unauthenticated, which is wrong about the sidecar's only authentication boundary. The real behaviour is stricter, so no control is weakened. |
| C-capture-sidecar-09 | rejected | n/a | This is a style and cleanup preference, not a defect. `IsLimitReached`, `LimitReasonUserRequested` and `(*Filter).Expression` do have no non-test callers. The comment at `handlers.go:65-67` is also partly inaccurate: `reasonMaxDuration` has the same value as `capture.LimitReasonDurationReached`, because the timer path needs the reason outside the Writer. But nothing behaves wrongly, nothing user-facing is misdocumented, and the only harm is a possible future drift between two equal string constants. |

Rejected: 2 ((see held item C-capture-sidecar-01, OD-019), C-capture-sidecar-09 as a style preference).

### C-capture-sidecar-02

**Location**: `capture-sidecar/internal/httpserver/handlers.go:365-368`; `operator/internal/controller/gameserver_controller.go:1432-1437` (the `captures` emptyDir, `SizeLimit` 1Gi, no `Medium`); `operator/internal/controller/networkcapture_controller.go:693` (the only file-delete caller); `api/internal/handlers/capture.go:840-848`; `charts/gameplane/values.yaml:531`, `:541-545`; `capture-sidecar/specs.md:24`, `:241`.

**Repro / observation**:
1. By reading on master: the `captures` volume is `EmptyDir{SizeLimit: 1Gi}` with no `Medium`, so it is disk-backed (`gameserver_controller.go:1432-1437`). The chart default and API ceiling for one capture is 900 MiB (`values.yaml:545`; the API rejects larger values at `capture.go:143-149`), and the API allows durations up to 3600 s (`capture.go:134`).
2. `HandleStart` checks only `maxSizeBytes > 0` (`handlers.go:365-368`). It never looks at the space that earlier capture files already use on the volume.
3. A finished file is removed only by `expireCapture` → `DeleteCaptureFile` (`networkcapture_controller.go:693`), after the retention window, which is 24 h by default (`values.yaml:531`). Deleting a capture through the API removes only the CR (`capture.go:840-848`), so that file stays until the pod is recreated.
4. Live: install with `capture.enabled: true` and opt a busy GameServer into capture. Run capture A with the default `maxSizeBytes` (943718400), a broad filter such as `tcp or udp` and a duration long enough to reach the size limit. When it ends with `max_size_reached`, run capture B the same way. Several smaller captures in one day add up the same way, and so does deleting captures through the dashboard and then capturing again.
5. Watch `kubectl get events -n <ns> --field-selector involvedObject.name=<gs>-0`. Once the volume passes 1 GiB, the kubelet's eviction manager evicts the pod with "Usage of EmptyDir volume "captures" exceeds the limit "1Gi"". The StatefulSet recreates the pod, so the game server restarts, capture B is failed with reason `PodRestarted` instead of `disk_full`, and every capture file on the old emptyDir is lost. On the live run, also check whether the kubelet skips the game's graceful stop for a local-storage eviction. If it does, game progress since the last autosave is lost too.

**Expected**: `specs.md:24` says the sidecar gracefully stops a capture when the volume fills. `specs.md:241` says the `sizeLimit` is a hard bound that the sidecar respects. The spec 003 edge case (`spec.md:107`) says a disk-full capture fails with a clear error "and the server remains playable". In total, the retained captures never push the pod past its emptyDir limit.

**Actual**: The ENOSPC path never fires for the `sizeLimit` case. When retained captures pass 1 GiB in total, the result is a pod eviction: the game server restarts, and all capture files are lost.

### C-capture-sidecar-03

**Location**: `capture-sidecar/cmd/main.go:36-41`; `operator/internal/controller/gameserver_controller.go:1627-1631`; `capture-sidecar/specs.md:79` and `:94-98`.

**Repro / observation**:
1. `grep -rn 'Getenv\|LookupEnv' capture-sidecar/ --include='*.go'` returns nothing.
2. `main.go:37-39` defines the flags `-tls-cert`, `-tls-key` and `-tls-client-ca`, with defaults `/etc/tls/tls.crt`, `/etc/tls/tls.key` and `/etc/tls/ca.crt`. The ephemeral container is started with no args.
3. `buildCaptureEphemeralContainer` sets `TLS_CERT_FILE`, `TLS_KEY_FILE` and `TLS_CA_FILE` to the same three paths (`gameserver_controller.go:1628-1630`). On a live pod, `kubectl get pod <gs>-0 -o jsonpath='{.spec.ephemeralContainers[?(@.name=="capture")].env}'` shows them.
4. Change one env value, for example to point at a different mount path. The sidecar still reads `/etc/tls/...`.

**Expected**: The sidecar reads the env vars that `specs.md` documents as required and that the operator passes. The other option is that the operator passes flags and `specs.md` documents flags.

**Actual**: The env vars have no effect. The sidecar works only because the flag defaults equal the env values.

### C-capture-sidecar-04

**Location**: `capture-sidecar/specs.md:21`, `:53`, `:75` and `:108-117`; `capture-sidecar/internal/httpserver/handlers.go:184-197`; `api/internal/handlers/capture.go:840-848`.

**Repro / observation**:
1. `handlers.go:192-197` registers `GET /healthz`, `POST /captures/{id}/start`, `POST /captures/{id}/stop`, `GET /captures/{id}/status`, `GET /captures/{id}/file` and `DELETE /captures/{id}`. The comment at `:184-186` says the `{id}:start` form is an invalid ServeMux pattern. The contract agrees (`specs/done_003-network-capture-sidecar/contracts/capture-sidecar.md:216`, `:234`, `:304`, `:499`).
2. `specs.md:21`, `:53`, `:75` and the table at `:112-117` list four operations, using `/captures/{id}:start` and `:stop`.
3. The `captureDelete` doc comment at `api/internal/handlers/capture.go:840-848` says "there is no sidecar endpoint to delete an individual capture file (capture-sidecar/specs.md's endpoint table lists only :start/:stop/status/file)". But the operator calls that endpoint in `DeleteCaptureFile` (`operator/internal/agent/sidecar_capture.go:227-268`).

**Expected**: `specs.md` lists the real paths and all six routes, and the API comment reflects that the delete endpoint exists.

**Actual**: `specs.md` gives colon paths and only four routes, and the API comment repeats that stale table.

### C-capture-sidecar-05

**Location**: `capture-sidecar/specs.md:11`, `:73`, `:183`, `:199`, `:247-257` and `:280`.

**Repro / observation**:
1. `:11` and `:253` say TTL expiry and a dashboard UI are future work, and that "no auto-deletion reconciliation runs yet". But `expireCapture` (`operator/internal/controller/networkcapture_controller.go:680-716`) deletes files and CRs, and `web/src/components/CaptureWidget.tsx` and `web/src/routes/tabs/settings/NetworkCapture.tsx` exist.
2. `:73` says the writer "stops gracefully, deleting partial files". `:211` and the code keep the file (`handlers.go:489-499`, `finish`). The disclaimer at `:199` names only "Packet Processing", "Shutdown" and "Security Considerations #5", not `:73`.
3. `:183` (invariant 2) says there is "no 'one more packet' tolerance". `:148`, `:203` and `writer.go:262-269` write the packet that crosses `maxSizeBytes`.
4. `:280` lists the test cases "empty filter fallback, default port-based filter". But `CompileFilter("")` returns an error (`filter.go:70-72`), and so does an empty request filter (`handlers.go:378-381`). The sidecar has no fallback.

**Expected**: `specs.md` describes one behaviour: the implemented one.

**Actual**: It contradicts itself and the code in these four places.

### C-capture-sidecar-06

**Location**: `capture-sidecar/specs.md:5`, `:85`, `:127` and `:207`; `capture-sidecar/go.mod:6`; `capture-sidecar/internal/capture/afpacket.go:79` and `:181`; `capture-sidecar/internal/capture/writer.go:55` and `:157`.

**Repro / observation**:
1. `go.mod:6` requires `github.com/gopacket/gopacket v1.7.2`, and `go.sum` lists only that version.
2. `specs.md:5` and `:85` name `github.com/google/gopacket v1.1.19`, which is a different module path and version. `:127` names `github.com/google/gopacket/pcapgo.NgWriter`.
3. `specs.md:207`, `afpacket.go:79` and `:181`, and `writer.go:55` and `:157` say the behaviour was "verified against" `github.com/gopacket/gopacket@v1.6.1`, not the v1.7.2 the module builds with.
4. The svcutil part of the original candidate is not repeated here. C-svcutil-01 already covers `specs.md:58` and `:88`, the orphan `replace` at `go.mod:11-13` and `Dockerfile:4-7`.

**Expected**: `specs.md` and the code comments name the gopacket module and version that `go.mod` actually uses.

**Actual**: They name an unrelated upstream module at v1.1.19, and verification notes against v1.6.1.

### C-capture-sidecar-07

**Location**: `capture-sidecar/internal/httpserver/handlers.go:391-394`. It is reached through `operator/internal/controller/networkcapture_controller.go:473-475` and `:797-808`, where the lock is released without a sidecar stop, and `:381-389`, where the 409 is stored as the failure message.

**Repro / observation**:
1. By reading on master: `HandleStart` returns `409 capture '<request id>' already in progress` whenever any capture is current, whatever its id (`:391-394`).
2. The operator can start a second capture while the first is still running on the sidecar. A single `GetCaptureStatus` error during Running goes to `r.fail(..., "sidecar unreachable: …")` (`:473-475`). `failWithReason` clears `status.capture.activeCapture` (`:797-808`) and never calls `StopCapture`, so `cap-aaa` keeps running on the sidecar until its own limit.
3. The user starts `cap-bbb`. The sidecar answers `409 capture 'cap-bbb' already in progress`. `IsTransientError` treats a 4xx as permanent (`sidecar_capture.go:52-60`), so the operator fails `cap-bbb` with `failed to start capture on sidecar: … capture 'cap-bbb' already in progress` (`:389`).
4. Unit-level check by reading: with a stub packet source, `POST /captures/cap-aaa/start` followed by `POST /captures/cap-bbb/start` on one `Server` returns a 409 whose body names `cap-bbb`.

**Expected**: For a conflict with a different id, the message names the capture that is actually running (`s.currentCapture.id`). FR-012 (`specs/done_003-network-capture-sidecar/spec.md:133`) requires "a clear error message stating that a capture is already in progress".

**Actual**: The message names the rejected capture, so the user sees a capture that never started described as "in progress".

### C-capture-sidecar-08

**Location**: `capture-sidecar/cmd/main.go:59-61`; `capture-sidecar/internal/httpserver/handlers.go:192` and `:201-207`; `capture-sidecar/internal/auth/tls.go:37`.

**Repro / observation**:
1. `main.go:59-60` says "/healthz is the only unauthenticated route", and `HandleHealthz`'s doc says "unauthenticated liveness check" (`handlers.go:201-202`). `handlers.go:192` registers the route without the middleware.
2. The same `http.Server` uses the `TLSConfig` from `auth.ServerTLS`, which sets `ClientAuth: tls.RequireAndVerifyClientCert` (`tls.go:37`). A client without a client certificate signed by the CA fails the handshake before any route runs.
3. The sidecar is an ephemeral container, and the Kubernetes API does not allow probes on ephemeral containers. `grep -rn healthz operator/internal api/internal charts/gameplane/templates` finds no caller on port 9091.

**Expected**: The comments describe the route as it behaves (mTLS-only, like every other route), or the unused route is removed.

**Actual**: The comments describe an unauthenticated liveness endpoint. The route in fact requires mTLS, and nothing calls it.
