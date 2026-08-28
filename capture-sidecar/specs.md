# capture-sidecar — Specification

**Status:** beta (v0.2.0-beta.8), Phase 2 Foundational (implementation complete)  
**Module / command:** `github.com/ValgulNecron/gameplane/capture-sidecar`  
**Dependencies (current):** stdlib + external libraries for packet capture and filtering (github.com/google/gopacket v1.1.19, github.com/packetcap/go-pcap, golang.org/x/net)

## Purpose

Optional packet capture sidecar injected into game pods to record network traffic for protocol reverse-engineering. The sidecar will use AF_PACKET live capture to collect packets matching a BPF filter expression, write them to a PCAPNG file on a pre-provisioned emptyDir volume, and expose a mTLS-authenticated HTTP control endpoint (`:9091`) for the operator and API to start/stop/monitor captures and download the resulting files.

> **Current state (Phase 2 Foundational):** Full implementation with AF_PACKET socket handling, PCAPNG writing, BPF filter compilation, mTLS authentication, and HTTP control endpoint. Real dependencies (gopacket, go-pcap) are pinned in go.mod. Unit tests for capture, filter, writer, and handlers are co-located with their implementations. Phase 2 Implementation (T067+) will add TTL-based expiry reconciliation and dashboard UI.

## Responsibilities (Design)

These are the design responsibilities for the completed sidecar (Phase 2+):

1. Bind an AF_PACKET socket to the pod's primary network interface and enter live capture mode.
2. Accept a compiled BPF filter expression and apply it at the packet-capture level (kernel-side filtering, not post-processing).
3. Write captured packets to a PCAPNG file on `/tmp/captures` using standard `gopacket/pcapgo.NgWriter` format, readable by `tcpdump`, Wireshark, and other third-party tools.
4. Enforce hard duration and size limits: stop capturing when `(now - startTime) >= maxDurationSeconds` or `bytesWritten >= maxSizeBytes`, whichever comes first.
5. Serve an mTLS-authenticated HTTP control endpoint on `0.0.0.0:9091` with four operations: start a capture (POST `:start`), stop a capture (POST `:stop`), poll status (GET `/status`), and download the completed file (GET `/file`).
6. Validate mTLS certificates against the cluster CA and reject unauthenticated requests.
7. Prevent multiple simultaneous captures on the same sidecar instance (409 Conflict if a duplicate capture ID arrives while one is running).
8. Monitor disk space and gracefully stop a capture if the emptyDir volume fills (`ENOSPC` handling).
9. Hold no persistent state; each sidecar instance is independent and does not retry captures or maintain history across restarts.

## Non-goals / boundaries

- Does **not** run the game server itself — that is the operator's and the game container's job.
- Does **not** authenticate operators or enforce access control — that is the API's job (RBAC applies to the API tier, not the sidecar).
- Does **not** modify game traffic or the game container — it captures passively via the shared pod network namespace.
- Does **not** filter packets post-capture — all filtering happens at the kernel level before the packet is copied to userspace.
- Does **not** persist or replicate captures across pod restarts — files live on emptyDir and are lost when the pod is deleted.
- Does **not** implement protocol parsing or protocol-specific logic — it is protocol-agnostic; filtering is done via generic BPF expressions.
- Does **not** serve captures through an HTTP proxy or gateway — the sidecar serves its own files directly over the mTLS endpoint.

## Current Implementation (Phase 2 Foundational)

### Directory & package layout

```
capture-sidecar/
├── cmd/main.go                    # Entry point; mTLS server setup on :9091; HTTP handler dispatch
├── internal/
│   ├── capture/
│   │   ├── afpacket.go            # AF_PACKET socket setup; live packet capture loop; MMap'd TPacket buffers
│   │   ├── afpacket_test.go       # Unit tests for AF_PACKET socket behavior
│   │   ├── filter.go              # BPF filter compilation and validation via go-pcap
│   │   ├── filter_test.go         # Unit tests for filter compilation and validation
│   │   ├── writer.go              # PCAPNG file writing via pcapgo.NgWriter; size/duration limit enforcement
│   │   └── writer_test.go         # Unit tests for PCAPNG writing and limit enforcement
│   ├── httpserver/
│   │   ├── handlers.go            # HTTP handlers for POST :start, POST :stop, GET /status, GET /file
│   │   └── handlers_test.go       # Unit tests for HTTP handlers and response shapes
│   └── auth/
│       ├── tls.go                 # mTLS certificate validation; TLS listener setup
│       └── tls_test.go            # Unit tests for mTLS validation
├── go.mod                         # Dependencies: gopacket/afpacket, gopacket/pcapgo, packetcap/go-pcap, svcutil
├── go.sum
├── Dockerfile                     # distroless/static:nonroot base; setcap cap_net_raw+ep on the built binary
├── .testcoverage.yml              # 70% coverage gate
└── specs.md                       # This file
```

Single Go module; packages organized by responsibility (capture, httpserver, auth). All packages have co-located test files exercising their behavior without root/CAP_NET_RAW/NIC dependencies.

### Phase 2 Foundational Packages

**`internal/capture/afpacket.go`**: Establishes AF_PACKET socket via gopacket/afpacket.TPacket with MMap'd kernel buffers. Compiles and applies BPF filters for kernel-side packet filtering. Reads packets via blocking poll (respecting context cancellation) and writes to PCAPNG writer. Manages socket lifecycle (Create, Start, Stop, Close) and enforces hard size/duration limits.

**`internal/capture/filter.go`**: Compiles BPF filter expressions via github.com/packetcap/go-pcap/filter into bytecode instructions. Validates filter syntax before capture starts (defense-in-depth, complementing API-tier validation).

**`internal/capture/writer.go`**: Writes captured packets to PCAPNG files via gopacket/pcapgo.NgWriter. Enforces hard size and duration limits (stops immediately when either is reached). Detects disk-full conditions (`ENOSPC`) and stops gracefully, deleting partial files. Produces valid PCAPNG files readable by `tcpdump`, Wireshark, and other third-party tools.

**`internal/httpserver/handlers.go`**: Implements four HTTP endpoints (POST `:start`, POST `:stop`, GET `/status`, GET `/file`) exposed on `:9091`. Validates requests, marshals capture state, and streams completed files to authenticated clients.

**`internal/auth/tls.go`**: Validates mTLS certificates against the cluster CA certificate. Sets up TLS listener with enforced client certificate authentication.

**`cmd/main.go`**: Entry point; parses environment variables for certificate paths (TLS_CERT_FILE, TLS_KEY_FILE, TLS_CA_FILE); constructs mTLS server; wires HTTP handlers and starts listening on `0.0.0.0:9091`.

### External Dependencies (Phase 2 Foundational)

The module declares the following external dependencies in `go.mod`:

- `github.com/google/gopacket v1.1.19` — AF_PACKET socket setup (afpacket package) and PCAPNG file writing (pcapgo package)
- `github.com/packetcap/go-pcap v0.0.0-20260731105150-c86974bbfbcd` — BPF filter compilation and validation
- `golang.org/x/net` — Networking utilities (bundled by gopacket)
- `github.com/ValgulNecron/gameplane/svcutil v0.0.0` — Shared utility helpers (graceful shutdown, env parsing) via local replace directive

## External Interface / Configuration (Phase 2 Design)

### Environment Variables

- **`TLS_CERT_FILE`** (required): Path to the sidecar's mTLS server certificate (e.g., `/etc/tls/tls.crt`). Comes from the `agent-tls` Secret volume.
- **`TLS_KEY_FILE`** (required): Path to the sidecar's mTLS server private key (e.g., `/etc/tls/tls.key`).
- **`TLS_CA_FILE`** (required): Path to the cluster CA certificate for validating client mTLS certificates (e.g., `/etc/tls/ca.crt`).

All three paths are mounted from the pre-existing `agent-tls` Secret that every game pod carries (no new Secret is created for capture).

### HTTP Server Configuration

**Listen Address**: `0.0.0.0:9091` (all pod network interfaces).

**Transport**: TLS with mTLS. All endpoints require both the sidecar's server certificate and the client's certificate (API server connecting with its own certificate). The sidecar validates the client certificate against the cluster CA.

**Port**: The existing `<gs>-agent` Kubernetes Service exposes this port numerically as target port 9091 (alongside the existing agent port). No separate Service or port declaration is needed.

### Endpoints (Phase 2)

Per `contracts/capture-sidecar.md`:

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/captures/{id}:start` | POST | Start a capture with filter, duration/size limits |
| `/captures/{id}:stop` | POST | Stop a running capture; finalize and close the file |
| `/captures/{id}/status` | GET | Poll capture status (running or completed) and stats |
| `/captures/{id}/file` | GET | Download the completed PCAPNG file |

**Error responses** will be plain text (e.g., `invalid filter: <syntax error>`, `capture 'cap-...' not found`), matching the codebase convention in `api/internal/httperr/httperr.go`.

## Capture File Format and Guarantees (Phase 2 Design)

### File Location

**Path**: `/tmp/captures/capture-<captureId>.pcapng`

**Format**: PCAPNG (Pcap Next Generation), will be produced by `github.com/google/gopacket/pcapgo.NgWriter`.

**Snaplen**: Full capture (65535 bytes per packet; no truncation).

**Timestamp Precision**: Nanosecond precision.

**Interfaces**: PCAPNG will include interface metadata (eth0, snaplen, filter expression recorded in the interface block).

### Filtering Guarantee (FR-011)

**At-Capture Filtering (Phase 2)**: The BPF filter will be applied by `gopacket/afpacket.TPacket` at the kernel level. The kernel will drop non-matching packets before they are copied to userspace. This will guarantee:
- 100% of packets in the file match the filter.
- 0% of non-matching packets are included.
- No post-processing or offline filtering.

**Filter Compilation (Phase 2)**: The sidecar will compile filter expressions locally using `github.com/packetcap/go-pcap/filter.Compile`. If compilation fails (defense-in-depth), the start request will return HTTP 400 with the syntax error.

### Limits and Enforcement (Phase 2)

**Duration**: Capture stops when `(now - startTime) >= maxDurationSeconds`. Hard limit; enforced by a timer.

**Size**: Capture stops when `bytesWritten >= maxSizeBytes` (checked before each packet write; the final file size may exceed the limit by one packet, typically < 64KB overrun). Hard limit.

**Either-Or Semantics**: Capture stops when either limit is reached first. The `stoppingReason` field in status indicates which: `max_duration_reached` or `max_size_reached`.

## Packet Capture Pipeline (Phase 2 Design)

### Startup Flow

1. Receive a start request with `{filter, maxDurationSeconds, maxSizeBytes}`.
2. Compile the BPF filter using `go-pcap/filter.Compile`; reject on syntax error.
3. Set up an AF_PACKET socket with MMap'd TPacket buffers and apply the compiled filter.
4. Create and open the PCAPNG file at `/tmp/captures/capture-<id>.pcapng`.
5. Start the capture loop: read packets from the kernel buffer, decode them via `gopacket`, write to the PCAPNG file.
6. Start a timer for the max-duration limit.
7. Return HTTP 200 with status `running`.

### Packet Processing

1. For each packet received:
   - Check if max-size limit is reached; if so, stop and finalize.
   - Check if max-duration timer expired; if so, stop and finalize.
   - Write the packet to the PCAPNG file.
2. On size or duration limit: finalize and close the PCAPNG file cleanly, marking the capture `completed`.
3. On explicit stop request: finalize and close immediately.

### Shutdown

- On a graceful stop (explicit `:stop` or limit reached): finalize the PCAPNG file (all headers, interface blocks, packet blocks written), close the file handle, and mark status `completed`.
- On disk full (`ENOSPC` during write): stop the capture, delete the partial file, and mark status `failed`.
- On sidecar crash: the partial file is left on disk; the operator detects the missing sidecar and marks the capture `failed`.

## Key Invariants (Design)

1. **Filter validation before capture**. The filter is compiled (via `go-pcap/filter.Compile`) before any AF_PACKET socket is opened. An invalid filter expression is rejected with HTTP 400 before any state is created. This is defense-in-depth; the API tier also validates filters before creating a NetworkCapture CRD.

2. **Hard duration and size limits**. Both are enforced strictly: the capture stops immediately when either limit is reached, even if a packet is mid-arrival. There is no grace period, no "one more packet" tolerance. The PCAPNG file is valid and complete even on a limit-triggered stop.

3. **File capabilities grant it, not `securityContext.capabilities.add` alone**. Raw-packet access (`CAP_NET_RAW`) is granted via a file capability on the sidecar's own binary (`setcap cap_net_raw+ep`), not via Kubernetes' `securityContext.capabilities.add`. Kubernetes does not set ambient capabilities; under a non-root `runAsUser`, `add: ["NET_RAW"]` by itself grants nothing (the effective set is cleared on `execve`). The container's `securityContext.capabilities` does list `Drop: ["ALL"], Add: ["NET_RAW"]`, but the `Add` exists only to keep NET_RAW in the process's *bounding* set — `Drop: ["ALL"]` alone would empty the bounding set too, and the kernel refuses to grant a file capability at `execve` that isn't in the bounding set (EPERM). The process's effective set is still empty at start; the actual grant comes from the setcap'd binary at exec. The file capability is ignored under `no_new_privs`, which is set when `allowPrivilegeEscalation: false`; therefore, `allowPrivilegeEscalation: true` is mandatory for this container, trading off that one flag to preserve `runAsNonRoot: true`.

4. **No capability on the game container**. The game container's `securityContext` is never modified. It retains its existing security posture and does not gain raw-packet capabilities. Elevated privileges are scoped to the capture container's own binary only.

5. **Cannot be removed once injected**. Ephemeral containers, once injected into a running pod, cannot be removed via the Kubernetes API — the `pods/ephemeralcontainers` subresource does not support deletion. When capture is disabled on a GameServer, the sidecar container persists in `pod.status.ephemeralContainerStatuses` until the pod is next recreated (e.g., via a new StatefulSet rollout). This asymmetry is accepted and documented as a constraint.

6. **One capture per sidecar instance**. Multiple simultaneous captures on the same sidecar are rejected with HTTP 409 (`capture '{id}' already in progress`). This is enforced in-memory; the authoritative per-GameServer concurrency lock lives in the operator's NetworkCaptureReconciler.

7. **Exact handshake byte preservation**. The sidecar does not modify or interpret captured packets beyond filtering. The PCAPNG file preserves the exact bytes received from the kernel.

8. **Stateless and ephemeral**. The sidecar holds no persistent state. Each sidecar instance is independent; captures are not replicated or synchronized across multiple instances. Partial files left by a crashed sidecar are cleaned up by the pod's deletion or an explicit operator action.

## Edge Cases (US5 — implemented, verified against source)

The "Phase 2 Design" language above (in "Packet Processing", "Shutdown", and "Security Considerations" #5) predates the actual implementation and is stale on a few points, called out below. This section describes verified behavior in `internal/capture/writer.go` and `internal/httpserver/handlers.go`.

### Max-size auto-stop and on-disk accounting

`Writer.WritePacket` tracks `bytesWritten` against real on-disk cost, not payload size: each packet's fixed Enhanced Packet Block overhead (28-byte header + 4-byte trailer = `epbFixedOverheadBytes`, 32 bytes) plus its snaplen-truncated payload plus 4-byte alignment padding. The check runs *after* the packet is written, so the packet that crosses `maxSizeBytes` **is** written to the file — the limit means "stop now that we've reached it", not "reject the packet that would exceed it". Every `WritePacket` call after the limit is reached returns an error wrapping `ErrLimitReached` without writing anything further. `LimitReason()` reports `max_size_reached`.

### ENOSPC / disk-full handling

`pcapgo.NgWriter` buffers writes through an internal 4096-byte `bufio.Writer` (verified against `github.com/gopacket/gopacket@v1.6.1` `pcapgo/ngwrite.go`: `NewNgWriterInterface` calls `bufio.NewWriter(w)`). This means a full disk usually does **not** surface from `WritePacket` — small packets accumulate in that buffer, and a write to the real underlying file only happens once the buffer fills or `Flush`/`Close` runs. `Writer.Close` calls the PCAPNG writer's `Flush()`; if that fails with an error wrapping `syscall.ENOSPC` (checked with `errors.Is`, so it matches whether the error arrives bare or wrapped the way real file-write errors are, e.g. inside `*fs.PathError`), `Close` sets `limitReached = true` and `limitReason = LimitReasonDiskFull` (`"disk_full"`) before returning the error. `WritePacket`'s own write path performs the identical ENOSPC check, covering the case where a packet's write happens to trigger the bufio buffer's internal flush.

**A disk-full capture is never reported as a clean "completed".** In `httpserver.Server.finish`, a non-nil error from `state.writer.Close()` always sets `status = statusFailed`, and if `errors.Is(err, syscall.ENOSPC)` it also sets `reason = reasonDiskFull` — regardless of what reason originally triggered the stop. So a duration- or size-triggered stop that *also* hits ENOSPC during its finalizing flush is still reported `failed` / `disk_full`, overriding the triggering reason.

**The partial file is kept, not deleted.** This corrects the "will... delete the partial file" language elsewhere in this document (see "Shutdown" and "Security Considerations" #5 above): the implemented behavior deliberately retains the file. `finish`'s doc comment in handlers.go states the file "already holds real packets and stays downloadable" after any failure, disk-full included, because a partial capture is still useful for the protocol-reverse-engineering use case this feature exists for. `HandleDownload` continues to serve the file once the capture reaches a terminal state. This is an intentional design decision, not an unimplemented gap — do not add file deletion on ENOSPC to "match" the older language.

### Max-duration auto-stop lifecycle

`HandleStart` arms `time.AfterFunc(time.Duration(maxDurationSeconds)*time.Second, ...)` immediately after publishing the capture as `s.currentCapture`, closing over that specific `*captureState` by pointer identity. On expiry it calls `s.finish("", state, reasonMaxDuration, nil)` — the same finalization path an explicit `POST /captures/{id}/stop` uses: cancel the capture's context (stopping the AF_PACKET read loop), close the writer (flushing and finalizing the PCAPNG file), and publish the terminal status under the capture's mutex. No caller-issued `:stop` is required; `GET /captures/{id}/status` reflects the stopped state and final counters as soon as the timer's `finish` call completes. Because the match is by pointer rather than by id, a stale timer belonging to an earlier capture can never terminate a later capture that happens to reuse the same id (see `TestStaleDurationTimerCannotKillASuccessor`). `Writer.WritePacket` also independently checks `elapsed >= maxDurationSeconds` on every packet it's asked to write (`LimitReasonDurationReached`), so an actively-receiving capture stops just as promptly on the packet path without waiting for the timer.

### Terminal states and stopping reasons

Every capture that reaches a terminal state reports `status` (`completed` or `failed`) and `stoppingReason` via `GET /captures/{id}/status` and the `:stop` response:

| `stoppingReason` | Meaning | `status` |
|---|---|---|
| `user_requested` | An explicit `POST /captures/{id}/stop` (or one with an empty/omitted `reason`) ended the capture. | `completed` |
| `max_duration_reached` | The `maxDurationSeconds` timer, or the writer's own per-packet duration check, fired. | `completed` |
| `max_size_reached` | `bytesWritten >= maxSizeBytes` was reached; the packet that crossed the limit is included in the file. | `completed` |
| `disk_full` | The PCAPNG writer's flush hit `syscall.ENOSPC`, discovered at `Close` (the common case) or occasionally at `WritePacket`. | `failed` |
| `error` | A non-ENOSPC I/O failure mid-capture (kernel-side read error, socket closed, etc.). | `failed` |

`completed` vs. `failed` is decided solely by whether `Writer.Close()` (or the packet-read loop) returned a non-nil error — not by which condition triggered the stop — so a duration- or size-triggered stop that also hits a disk-full flush at `Close` is still reported `failed` / `disk_full`, overriding the reason that started the stop. In every case `Close` runs, and its result is folded into `status`/`stoppingReason`, before the terminal state is published — so any observer that sees a non-`running` status is guaranteed a fully flushed, closed file: `completed` means the file is complete and valid up to the interrupted limit; `failed` / `disk_full` means the file is valid up to whatever was durably flushed before the disk filled, which is not necessarily every packet the capture logically accepted.

## Security Considerations (Phase 2 Design)

1. **mTLS-only communication**: All endpoints require mTLS. The sidecar validates the client certificate against the cluster CA. No bearer tokens or API keys are used. Only authenticated clients (the API server, the operator) can reach the sidecar's control endpoints.

2. **File capability isolation**: `CAP_NET_RAW` is granted to the sidecar's binary only, via file capabilities. The game container retains no elevated privileges. This is a structural boundary, not a runtime check.

3. **No privilege escalation in the game container**: The game container's `securityContext` is left unchanged. It cannot escape to gain raw-packet access or any other elevated privileges from the sidecar's presence.

4. **Minimal file access**: The sidecar writes only to `/tmp/captures` and reads only `/etc/tls` for certificates. No other container mounts the capture volume; the captured files are inaccessible within the pod except via the sidecar's own HTTP endpoint.

5. **Disk full handling**: The sidecar detects `ENOSPC` during a PCAPNG flush and stops gracefully, marking the capture `failed` with reason `disk_full` and keeping the partial file downloadable (see "Edge Cases" above). The emptyDir's `sizeLimit` enforces a hard disk bound; the sidecar respects it.

6. **No unbounded resource consumption**: Duration and size limits prevent captures from running forever or consuming unlimited disk. The sidecar's own implementation must use bounded buffers (the AF_PACKET MMap'd buffer is sized by the kernel and the configuration, not unlimited).

7. **Sensitive data in captures**: Captured packets may contain player IP addresses, network timing, game commands, and in-band credentials (session tokens, server passwords). Access control (mTLS + RBAC at the API tier) and time-limited retention (the operator's TTL reconciliation) are the primary mitigations, not redaction or sanitization.

## Planned (Phase 2 Implementation and beyond)

Phase 2 Implementation and future phases will add the following:

### TTL-based Expiry (Phase 2 Implementation, T067+)

The NetworkCaptureReconciler will implement automatic deletion of completed captures after `spec.ttlSecondsAfterFinished` seconds have elapsed. For Phase 2 Foundational, the API tier validates and clamps TTL at CRD creation time, but no auto-deletion reconciliation runs yet.

### Dashboard UI (Phase 2 Implementation, T067+)

Web dashboard support for starting, stopping, browsing, and downloading network captures. Planned for Phase 2 Implementation after the sidecar and operator reconciliation are fully wired.

### Testing & Coverage (Phase 2 Foundational, COMPLETE)

### Coverage Gate

- **Threshold**: 70% (from `.testcoverage.yml`).
- **Measured against**: package-level aggregate (not per-file).
- **What's covered**: AF_PACKET socket lifecycle, BPF filter compilation, PCAPNG writing with limit enforcement, mTLS validation, HTTP handler routing and response shapes, edge cases (disk full, concurrent captures, invalid filters).
- **Uncovered**: main() process wiring, signal handling, graceful shutdown (not unit-testable in isolation).

### Test Commands (Phase 2)

Run via the workspace root Makefile or isolated:

```sh
make test-go           # Runs all Go tests, including capture-sidecar
cd capture-sidecar && go test ./...    # Isolated run
```

### Key Test Cases (Phase 2)

- **AF_PACKET**: socket setup, filter application, packet reception, MMap'd buffer semantics.
- **Filter Compilation**: valid filter acceptance, invalid filter rejection with syntax error, empty filter fallback, default port-based filter.
- **PCAPNG Writer**: valid file creation and closure, size-limit auto-stop with clean finalization, duration-limit auto-stop, max-size enforcement (final file <= limit + one packet).
- **mTLS**: valid certificate acceptance, invalid/expired certificate rejection, CA validation.
- **HTTP Handlers**: start/stop/status/file happy paths, 400 on invalid filter, 409 on concurrent capture with same ID, 404 on nonexistent capture, file download with correct headers.
- **Disk Full**: `ENOSPC` detection on the PCAPNG flush, capture marked `failed` with reason `disk_full`, partial file kept downloadable (see "Edge Cases").
- **Concurrent Captures**: second start request on a running capture is rejected 409; only one running capture per sidecar at a time.

## References

- **`contracts/capture-sidecar.md`** — HTTP endpoint contracts (request/response shapes, status codes, error messages).
- **`specs/done_003-network-capture-sidecar/spec.md`** — Feature specification (User Stories US1–US5, Functional Requirements FR-001–FR-012, Success Criteria SC-001–SC-008).
- **`specs/done_003-network-capture-sidecar/data-model.md`** — NetworkCapture CRD design, status shape, lifecycle phases.
- **`operator/internal/controller/gameserver_controller.go`** — pod template injection and ephemeral-container mechanics.
- **`api/internal/handlers/capture.go`** — API-tier start/stop/download/enable/disable handlers that call the sidecar.
- **`gopacket` documentation** — AF_PACKET and PCAPNG format specs.
- **BPF filter syntax** — `man 7 pcap-filter` for the filter expression grammar.
- **PCAPNG format spec** — RFC 7468 (note: PCAPNG is not an RFC; the canonical spec is in the libpcap/tcpdump source).
