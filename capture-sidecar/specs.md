# capture-sidecar — Specification

**Status:** beta (v0.2.0-beta.8), Phase 1 (stub packages)  
**Module / command:** `github.com/ValgulNecron/gameplane/capture-sidecar`  
**Dependencies (current):** stdlib only (crypto/tls, encoding/binary, fmt, io, log, net, os, os/signal, sync, syscall, time)

## Purpose

Optional packet capture sidecar injected into game pods to record network traffic for protocol reverse-engineering. The sidecar will use AF_PACKET live capture to collect packets matching a BPF filter expression, write them to a PCAPNG file on a pre-provisioned emptyDir volume, and expose a mTLS-authenticated HTTP control endpoint (`:9091`) for the operator and API to start/stop/monitor captures and download the resulting files.

> **Current state (Phase 1):** Stub package layout with skeleton implementations in `internal/capture/afpacket.go`, `internal/capture/writer.go`, `internal/httpserver/handlers.go`, and `internal/auth/tls.go`. Packet capture, filter compilation, and testing are planned for Phase 2.

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

## Current Implementation (Phase 1)

### Directory & package layout

```
capture-sidecar/
├── cmd/main.go                    # Entry point stub (not yet implemented)
├── internal/
│   ├── capture/
│   │   ├── afpacket.go            # AF_PACKET socket stubs (Phase 2)
│   │   └── writer.go              # PCAPNG file writer stubs (Phase 2)
│   ├── httpserver/
│   │   └── handlers.go            # HTTP handler stubs (Phase 2)
│   └── auth/
│       └── tls.go                 # mTLS certificate validation stubs (Phase 2)
├── go.mod                         # Stdlib only (no external dependencies yet)
├── Dockerfile                     # distroless/static:nonroot base (setcap prepared but not implemented)
├── .testcoverage.yml              # 70% coverage gate (placeholder)
└── specs.md                       # This file
```

Single Go module; packages organized by responsibility (capture, httpserver, auth) for Phase 2 organization.

### Phase 1 Package Stubs

**`internal/capture/afpacket.go`**: Empty struct and function stubs. Will eventually set up AF_PACKET sockets with MMap'd TPacket buffers and apply BPF filters.

**`internal/capture/writer.go`**: Empty struct and function stubs. Will eventually write PCAPNG files and enforce size/duration limits.

**`internal/httpserver/handlers.go`**: Empty HTTP handler stubs. Will eventually implement POST `:start`, POST `:stop`, GET `/status`, GET `/file`.

**`internal/auth/tls.go`**: Empty TLS validation stubs. Will eventually validate mTLS certificates against the cluster CA.

**`cmd/main.go`**: Placeholder entry point. Phase 2 will wire up the packages, parse environment variables, and run the mTLS server on `:9091`.

### No External Dependencies (Phase 1)

The module declares no external dependencies in `go.mod`. All stubs use only stdlib. Packet capture libraries (`gopacket`, `go-pcap`) will be added in Phase 2.

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

3. **File capabilities, not securityContext.capabilities.add**. Raw-packet access (`CAP_NET_RAW`) is granted via a file capability on the sidecar's own binary (`setcap cap_net_raw+ep`), not via Kubernetes' `securityContext.capabilities.add`. Kubernetes does not set ambient capabilities; under a non-root `runAsUser`, `add: ["NET_RAW"]` grants nothing (the effective set is cleared on `execve`). The file capability is ignored under `no_new_privs`, which is set when `allowPrivilegeEscalation: false`; therefore, `allowPrivilegeEscalation: true` is mandatory for this container, trading off that one flag to preserve `runAsNonRoot: true`.

4. **No capability on the game container**. The game container's `securityContext` is never modified. It retains its existing security posture and does not gain raw-packet capabilities. Elevated privileges are scoped to the capture container's own binary only.

5. **Cannot be removed once injected**. Ephemeral containers, once injected into a running pod, cannot be removed via the Kubernetes API — the `pods/ephemeralcontainers` subresource does not support deletion. When capture is disabled on a GameServer, the sidecar container persists in `pod.status.ephemeralContainerStatuses` until the pod is next recreated (e.g., via a new StatefulSet rollout). This asymmetry is accepted and documented as a constraint.

6. **One capture per sidecar instance**. Multiple simultaneous captures on the same sidecar are rejected with HTTP 409 (`capture '{id}' already in progress`). This is enforced in-memory; the authoritative per-GameServer concurrency lock lives in the operator's NetworkCaptureReconciler.

7. **Exact handshake byte preservation**. The sidecar does not modify or interpret captured packets beyond filtering. The PCAPNG file preserves the exact bytes received from the kernel.

8. **Stateless and ephemeral**. The sidecar holds no persistent state. Each sidecar instance is independent; captures are not replicated or synchronized across multiple instances. Partial files left by a crashed sidecar are cleaned up by the pod's deletion or an explicit operator action.

## Security Considerations (Phase 2 Design)

1. **mTLS-only communication**: All endpoints require mTLS. The sidecar validates the client certificate against the cluster CA. No bearer tokens or API keys are used. Only authenticated clients (the API server, the operator) can reach the sidecar's control endpoints.

2. **File capability isolation**: `CAP_NET_RAW` is granted to the sidecar's binary only, via file capabilities. The game container retains no elevated privileges. This is a structural boundary, not a runtime check.

3. **No privilege escalation in the game container**: The game container's `securityContext` is left unchanged. It cannot escape to gain raw-packet access or any other elevated privileges from the sidecar's presence.

4. **Minimal file access**: The sidecar writes only to `/tmp/captures` and reads only `/etc/tls` for certificates. No other container mounts the capture volume; the captured files are inaccessible within the pod except via the sidecar's own HTTP endpoint.

5. **Disk full handling**: The sidecar will detect `ENOSPC` during file write and stop gracefully, deleting the partial file. The emptyDir's `sizeLimit` enforces a hard disk bound; the sidecar respects it.

6. **No unbounded resource consumption**: Duration and size limits prevent captures from running forever or consuming unlimited disk. The sidecar's own implementation must use bounded buffers (the AF_PACKET MMap'd buffer is sized by the kernel and the configuration, not unlimited).

7. **Sensitive data in captures**: Captured packets may contain player IP addresses, network timing, game commands, and in-band credentials (session tokens, server passwords). Access control (mTLS + RBAC at the API tier) and time-limited retention (the operator's TTL reconciliation) are the primary mitigations, not redaction or sanitization.

## Planned (Phase 2)

Phase 2 will complete the implementation with the following:

### Dependencies (Phase 2)

**Stdlib** (current):
- `crypto/tls` — mTLS certificate validation and listener setup.
- `encoding/binary` — PCAPNG binary format handling.
- `fmt`, `io`, `log`, `net`, `os`, `os/signal`, `sync`, `syscall`, `time` — standard utility.

**Packet Capture & Encoding** (Phase 2):
- `github.com/google/gopacket/afpacket` — AF_PACKET live capture with MMap'd TPacket buffers.
- `github.com/google/gopacket/pcapgo` — PCAPNG file writer (`NgWriter`).

**Filter Compilation** (Phase 2):
- `github.com/packetcap/go-pcap/filter` — BPF filter compilation and validation. **NOTE**: This module has no tagged release (pseudo-version only). The pin must be deliberate and documented in `go.mod`.

### Directory & Package Layout (Phase 2)

```
capture-sidecar/
├── cmd/main.go                    # Entry point; mTLS server setup on :9091; HTTP handler dispatch
├── internal/
│   ├── capture/
│   │   ├── afpacket.go            # AF_PACKET socket setup; live packet capture loop; MMap'd TPacket buffers
│   │   ├── filter.go              # BPF filter compilation and validation via go-pcap
│   │   ├── writer.go              # PCAPNG file writing via pcapgo.NgWriter; size/duration limit enforcement
│   │   ├── afpacket_test.go       # Unit tests for AF_PACKET socket behavior
│   │   ├── filter_test.go         # Unit tests for filter compilation and validation
│   │   └── writer_test.go         # Unit tests for PCAPNG writing and limit enforcement
│   ├── httpserver/
│   │   ├── handlers.go            # HTTP handlers for POST :start, POST :stop, GET /status, GET /file
│   │   └── handlers_test.go       # Unit tests for HTTP handlers and response shapes
│   └── auth/
│       ├── tls.go                 # mTLS certificate validation; TLS listener setup
│       └── tls_test.go            # Unit tests for mTLS validation
├── go.mod                         # Dependencies: gopacket/afpacket, gopacket/pcapgo, packetcap/go-pcap
├── go.sum
├── Dockerfile                     # distroless/static:nonroot base; setcap cap_net_raw+ep on the built binary
├── .testcoverage.yml              # 70% coverage gate
└── specs.md                       # This file
```

### Testing & Coverage (Phase 2)

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
- **Disk Full**: `ENOSPC` detection, partial file deletion, capture marked failed.
- **Concurrent Captures**: second start request on a running capture is rejected 409; only one running capture per sidecar at a time.

## References

- **`contracts/capture-sidecar.md`** — HTTP endpoint contracts (request/response shapes, status codes, error messages).
- **`specs/003-network-capture-sidecar/spec.md`** — Feature specification (User Stories US1–US5, Functional Requirements FR-001–FR-012, Success Criteria SC-001–SC-008).
- **`specs/003-network-capture-sidecar/data-model.md`** — NetworkCapture CRD design, status shape, lifecycle phases.
- **`operator/internal/controller/gameserver_controller.go`** — pod template injection and ephemeral-container mechanics.
- **`api/internal/handlers/capture.go`** — API-tier start/stop/download/enable/disable handlers that call the sidecar.
- **`gopacket` documentation** — AF_PACKET and PCAPNG format specs.
- **BPF filter syntax** — `man 7 pcap-filter` for the filter expression grammar.
- **PCAPNG format spec** — RFC 7468 (note: PCAPNG is not an RFC; the canonical spec is in the libpcap/tcpdump source).
