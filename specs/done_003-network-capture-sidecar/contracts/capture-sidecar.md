# Capture Sidecar Control Protocol Contract

**Feature**: 003-network-capture-sidecar  
**Phase**: 1 (Contracts)  
**Date**: 2026-08-23  
**Status**: Proposal for Code Review  

This document specifies the contract between the Kubernetes operator and the capture sidecar container: how the operator tells the sidecar to start/stop captures, how the sidecar authenticates and reports status, where captures are written, and the exact guarantees the sidecar makes.

---

## Overview

The capture sidecar is an optional ephemeral container added to game pods when `spec.capture.enabled = true`. It runs as a non-root user and obtains raw-socket access from a **file capability** (`cap_net_raw+ep`) set on its own binary — not by running as root, and not by `securityContext.capabilities.add` alone. See "Raw-socket access (`CAP_NET_RAW`)" below for why, and for the trade-offs that choice forces. The sidecar communicates with:
- **The operator**: Via the NetworkCapture CRD's spec and status fields (Kubernetes-native state machine).
- **The API server**: For trigger commands and status polling (mTLS-authenticated HTTP).
- **Pre-provisioned emptyDir volume**: For writing captured `.pcapng` files. The volume is declared
  in the StatefulSet pod template for *every* game pod (see "Pod Injection" below for why), and is
  mounted **only** into the capture container — never into the game container (FR-008) and never
  into the agent.
- **The API server, for download**: the capture container serves its own completed files over the
  pod network. It is *not* proxied through the agent's file browser — that browser is rooted at
  exactly one path (`--data-root`) and rejects any resolved path outside it, so a second mount
  would be unreachable to it (`operator/internal/controller/gameserver_rcon.go:111-119`). Serving
  captures through the agent would require teaching the agent to serve multiple roots, which this
  feature does not do.

---

## Sidecar Lifecycle and Container Specification

### Pod Injection

**Two distinct steps, and they happen at different times.** Conflating them is what made an earlier
draft of this contract specify a mutation Kubernetes rejects.

**Step 1 — volumes are pre-provisioned, ahead of time, for every game pod.** The
`pods/ephemeralcontainers` subresource accepts changes to `ephemeralContainers` **only**, and
`pod.spec.volumes` is immutable on a running pod. An ephemeral container may therefore mount only
volumes that *already exist* in the pod. Consequently the capture `emptyDir` is declared in the
StatefulSet pod template unconditionally — on every game pod, opted in or not — alongside the
existing `data` and `agent-tls` volumes
(`operator/internal/controller/gameserver_controller.go:1290-1308`).

This is the accepted trade recorded in `research.md`: pre-provisioning is what keeps *enabling*
capture restart-free. Its two costs are real and must not be glossed:

- Upgrading to the release that adds the volume performs a **one-time rolling restart of every
  existing game server**.
- A non-opted-in pod therefore carries an (empty) capture volume. FR-001 and SC-007 have been
  amended accordingly: what a non-opted-in pod does *not* carry is the capture **container**.

**Step 2 — the container is injected on enable, with no game-container restart.** When a GameServer
is reconciled with `spec.capture.enabled = true`, the operator injects the capture container via
`POST /pods/{name}/ephemeralcontainers`. The patch body contains `ephemeralContainers` and nothing
else.

**Injected patch** (this is the entire body — no `volumes`, no `ports`):
```yaml
spec:
  ephemeralContainers:
  - name: capture
    image: gcr.io/distroless/static:nonroot@sha256:...
    imagePullPolicy: IfNotPresent
    securityContext:
      runAsNonRoot: true
      runAsUser: 65532  # nonroot user
      # Raw-socket access comes from a FILE capability on the binary
      # (setcap cap_net_raw+ep) — Kubernetes sets no ambient capabilities, so
      # `add: ["NET_RAW"]` alone under a non-root runAsUser grants nothing:
      # the effective set is cleared on execve. `add: ["NET_RAW"]` IS listed
      # below anyway, but only to counter `drop: ["ALL"]` also emptying the
      # bounding set — without NET_RAW in the bounding set, the kernel
      # refuses to grant the setcap'd binary's file capability at execve
      # (EPERM). The process's own effective set is still empty at start.
      capabilities:
        drop: ["ALL"]
        add: ["NET_RAW"]
      # REQUIRED, and deliberate: file capabilities are ignored under no_new_privs,
      # which is what allowPrivilegeEscalation: false sets. See "Raw-socket access".
      allowPrivilegeEscalation: true
      readOnlyRootFilesystem: true
    # NOTE: no `ports:` — the EphemeralContainerCommon schema forbids a named
    # containerPort here, and the API server rejects a patch that sets one. The
    # listener is still reachable; see "Addressing" below.
    volumeMounts:
    - name: capture-data            # pre-provisioned in the pod template (Step 1)
      mountPath: /tmp/captures
      readOnly: false
    - name: agent-tls               # the pod's existing TLS volume — nothing new is added
      mountPath: /etc/tls
      readOnly: true
    env:
    - name: TLS_CERT_FILE
      value: /etc/tls/tls.crt
    - name: TLS_KEY_FILE
      value: /etc/tls/tls.key
    - name: TLS_CA_FILE
      value: /etc/tls/ca.crt
```

**Volumes referenced** (both already present in the pod before injection):

- `capture-data` (`emptyDir`, `sizeLimit` default 5Gi, configurable via Helm) — mounted **only**
  on the capture container. Not on the game container (FR-008) and not on the agent. The
  `sizeLimit` is the hard disk guard backing FR-002's max-size and the spec's disk-full edge case:
  the writer receives `ENOSPC` when it is hit. The name `capture-data` must match the pod-template
  declaration byte-for-byte (`specs/done_003-network-capture-sidecar/data-model.md:422`): a
  `volumeMounts[].name` that does not resolve to a declared `pod.spec.volumes[].name` makes the
  injection patch invalid.
- `agent-tls` — the per-GameServer Secret the pod already mounts, containing `tls.crt`, `tls.key`
  and `ca.crt`. The Secret is named `<gs>-agent-tls`
  (`operator/internal/controller/agent_certs.go:38`).

**mTLS certificates**: the capture container reuses the agent's existing per-GameServer identity
rather than provisioning a second one, so no new credential distribution is required. There is **no**
`ca-certificates` ConfigMap and no `<gs>-agent-cert` Secret — earlier drafts of this contract named
both, and neither exists in the tree.

**Addressing**: the capture endpoint is reached at
`<gs>-agent.<namespace>.svc.cluster.local:9091` over mTLS — the **existing** per-GameServer
`<gs>-agent` ClusterIP Service, with a second port added.

The constraint an ephemeral container imposes is narrower than earlier drafts claimed. A Service
selects **pods**, not containers, so a Service can perfectly well front a listener that happens to
live in an ephemeral container: the container shares the pod's network namespace (the same sharing
that lets it see the game's traffic), so the listener is simply a port on the pod. What an ephemeral
container *cannot* do is declare a named `containerPort` — `EphemeralContainerCommon` forbids
`ports:` entirely. The only consequence is that the Service must target port 9091 **numerically**
(`targetPort: 9091`), never by port name.

Dialing by this DNS name is what makes mTLS work without any new plumbing. `agentDNSNames`
(`operator/internal/controller/agent_certs.go:207-224`) already SANs the agent server certificate
for `<gs>-agent`, `<gs>-agent.<ns>`, `<gs>-agent.<ns>.svc` and `<gs>-agent.<ns>.svc.cluster.local`,
so the existing certificate already covers the capture endpoint. Two things follow, and both matter:

- **Do not dial the pod IP.** The certificate has **no IP SAN at all** — `agent_certs.go` never sets
  `IPAddresses` (verified: the field appears nowhere in the file) — so an IP dial fails certificate
  verification. Nor is there a `<pod-name>.<namespace>.svc.cluster.local` SAN; the pod-DNS form the
  cert does carry is `<gs>-0.<gs>.<ns>.svc.cluster.local`.
- **No new credential work is needed**: no second certificate, no IP SAN, no `ServerName` override
  at the TLS client.

**Operator work this implies**: adding port 9091 to the existing `<gs>-agent` Service (numeric
`targetPort`) is a change to the operator's Service reconciliation, and is part of this feature's
implementation scope. There is no `<gs>-capture.<namespace>.svc.cluster.local` Service — an earlier
draft invented one, and none is needed.

### Raw-socket access (`CAP_NET_RAW`)

Packet capture needs `CAP_NET_RAW` for `AF_PACKET` sockets. The capture container gets it from a
**file capability on its own binary** (`setcap cap_net_raw+ep /capture`), while still running as
non-root (UID 65532).

*Why not `securityContext.capabilities.add: ["NET_RAW"]` alone?* Because Kubernetes does not set
**ambient** capabilities. With a non-root `runAsUser`, the effective capability set is cleared on
`execve`, so `add: ["NET_RAW"]` by itself grants the process nothing. The listed capability would
look correct in the pod spec and the socket would still fail with `EPERM`.

The pod spec *does* list `add: ["NET_RAW"]` (alongside `drop: ["ALL"]`) — but not because `add`
grants the capability. `drop: ["ALL"]` on its own empties the process's *bounding* capability set as
well as its effective set, and the kernel refuses to grant a file capability at `execve` that is not
in the bounding set (the socket open would fail with `EPERM`, exactly as in the naive-`add`-only
case above, but for a different reason). Re-adding `NET_RAW` keeps it in the bounding set so the
setcap'd binary's file capability can still be granted; the process's effective set is still empty at
container start. The actual grant remains the file capability on the binary, not this `add`.

Three consequences, all of which are accepted costs rather than solved problems:

1. **`allowPrivilegeEscalation: true` is mandatory for this container.** File capabilities are
   ignored under `no_new_privs`, which is exactly what `allowPrivilegeEscalation: false` sets. The
   trade is explicit: `runAsNonRoot: true` is preserved, `allowPrivilegeEscalation: false` is given
   up. Everything else stays locked down (`drop: ["ALL"]` plus the bounding-set-only `add: ["NET_RAW"]`, `readOnlyRootFilesystem: true`).
2. **The `restricted` PodSecurity profile forbids `allowPrivilegeEscalation: true`.** A game
   namespace running under `restricted` cannot admit this pod. That namespace therefore needs a
   documented PodSecurity exception, and recording it is an obligation on `docs/security.md` — not
   something this contract can wave through.
3. **UNVERIFIED BUILD RISK — file capabilities are extended attributes, and xattrs do not reliably
   survive a multi-stage `COPY` into a distroless/scratch image.** This repo has already been bitten
   by `COPY`-time file-mode loss (commit `641a783`, "fix(images): set entrypoint mode at COPY time
   instead of chmod"). Whether `cap_net_raw+ep` survives the image build here is **not known**: it
   has not been built, run, or tested. It MUST be proven in CI before this approach is trusted. Do
   not read anything in this document as a claim that it works.

### Ephemeral Container Constraints

Ephemeral containers are a Kubernetes feature with specific limitations that shape the capture design:

1. **No Probes**: LivenessProbe and readinessProbe are not supported on ephemeral containers. The control plane cannot directly monitor sidecar health.
2. **Cannot Be Restarted**: if the capture process crashes, Kubernetes does NOT restart it. The crash is therefore TERMINAL for that capture: the capture is marked failed, there is no retry, and any completed file still on the volume becomes unreachable until a new ephemeral container is injected or the pod is recreated. The game container is unaffected.
3. **No Resource Requests/Limits**: CPU and memory limits cannot be specified on ephemeral containers. The sidecar runs with the pod's default quality-of-service but no hard resource bounds.
4. **Cannot Be Removed**: Once injected, an ephemeral container cannot be removed from a running pod via the Kubernetes API. It persists in `pod.spec.ephemeralContainers`, with its runtime state in `pod.status.ephemeralContainerStatuses`, until the pod is deleted.
5. **Network Namespace Sharing**: Ephemeral containers always share the pod's network namespace (no network isolation), which is necessary for packet capture but requires careful design to avoid interference.

The control plane accepts these constraints in exchange for zero downtime on pod join (US2 scenario 2: capture enabled without game container restart).

---

## Sidecar HTTP Server

**Error body shape (applies to every error table below)**: errors are **plain text**, exactly as in
`contracts/rest-api.md` — a bare message, `Content-Type: text/plain; charset=utf-8`, written with
Go's `http.Error`. There is no `{"code", "message"}` JSON envelope anywhere in this system. The
sidecar is a separate binary and does not import `api/internal/httperr`, but it deliberately matches
that package's wire shape rather than inventing a second one; it is not exempt.

### Port and Transport

**Listen Address**: `0.0.0.0:9091` (TCP, all interfaces within pod network).

**TLS**: Required. All communication is over HTTPS with mTLS (client cert + server cert). The API server connects with its own client certificate, and the sidecar validates it against the cluster CA.

**Routing note**: the sidecar routes with the standard library's `net/http.ServeMux`, whose
wildcard segments must be exactly `{name}`. The chi-style `:verb` suffix used by the REST API tier
(`/servers/{name}:capture-start`) is an **invalid** ServeMux pattern and makes `mux.Handle` panic at
startup, so every sidecar control path uses a whole `/start` / `/stop` segment instead.

**Certificates**:
- Server cert: `/etc/tls/tls.crt`
- Server key: `/etc/tls/tls.key`
- CA cert: `/etc/tls/ca.crt`

All three come from the single pre-existing `agent-tls` volume mounted at `/etc/tls` (Secret
`<gs>-agent-tls`, `operator/internal/controller/agent_certs.go:38`). There is no separate `tls-ca`
or `tls-cert` volume.

### Endpoint: Start Capture

**Functional Requirement**: FR-001, FR-002, FR-003, FR-004.

#### Request

```
POST /captures/{id}/start
Host: <gs>-agent.<namespace>.svc.cluster.local:9091   # existing agent Service, numeric port
Content-Type: application/json
Authorization: Bearer <not used; mTLS is auth mechanism>

{
  "filter": "tcp port 8080",
  "maxDurationSeconds": 300,
  "maxSizeBytes": 5368709120
}
```

**Path Parameters**:
- `{id}` (string): Capture ID (e.g., `cap-8f7d3c1a`). Must match the NetworkCapture CRD ID.

**Request Body Fields**:
- `filter` (string): Validated BPF filter expression. **Required on this hop, and non-empty.** FR-003 makes the filter optional at the *API* boundary, where an omitted filter means "restrict the capture to the game server's own advertised ports" — but only the control plane knows those ports, so it must materialise that default before calling the sidecar. The sidecar cannot reconstruct it and never treats "no filter" as "capture everything", which would record the agent's and API's mTLS traffic, RCON, and every plaintext protocol on the pod into a downloadable file, voiding Guarantee 1. An empty or absent `filter` is rejected with 400. The sidecar also re-compiles the expression (defense-in-depth) and rejects a malformed one with 400.
- `maxDurationSeconds` (integer): Max runtime. Capture stops when this duration elapses. Must be in `1..604800`; the sidecar rejects anything outside that range with 400, independently of the API tier's tighter cap, so an oversized value can never overflow into a negative timer that fires immediately.
- `maxSizeBytes` (integer): Max file size. Capture stops when file reaches this size.

**Preconditions**:
- Sidecar must be running and listening on port 9091.
- No other capture with the same ID may be active (sidecar rejects duplicate IDs with HTTP 409).
- Pod's emptyDir must have available space (check during capture; stop if disk fills).

**Socket opening is synchronous**: the sidecar opens the AF_PACKET socket *before* it creates the capture file and *before* it answers. A socket that cannot be opened (no `CAP_NET_RAW`, missing interface) is reported as a 500 with the underlying error, per Guarantee 6 — it never yields a 200 followed by a valid-but-empty PCAPNG, which a user cannot tell apart from "the filter matched nothing". A failed start leaves no capture registered and no orphan file on the volume.

**Sidecar Availability**: Because ephemeral containers have no probes (livenessProbe/readinessProbe are not supported), the control plane cannot directly poll sidecar health. The API learns the sidecar is alive by receiving a 200 response to the start request; if the sidecar does not respond (crash, hang, not injected), the API marks the capture as Failed and reports the error to the user.

#### Response

**Success (200 OK)**:
```json
{
  "status": "running",
  "captureId": "cap-8f7d3c1a",
  "startedAt": "2026-08-23T14:30:05Z",
  "bytesWritten": 0,
  "packetsWritten": 0
}
```

**Response Fields**:
- `status` (string): `running` on success.
- `captureId` (string): Echoed from request.
- `startedAt` (RFC3339 timestamp): When the capture started (sidecar time, not client time).
- `bytesWritten` (integer): Bytes written so far (0 at start).
- `packetsWritten` (integer): Packets written so far (0 at start).

#### Error Responses

| Status | Condition | Response Body (plain text) |
|--------|-----------|---|
| **400** | Filter is invalid (defense-in-depth) | `invalid filter: <syntax error>` |
| **400** | maxDurationSeconds or maxSizeBytes out of range | `maxDurationSeconds out of range` / `maxSizeBytes out of range` |
| **400** | Filter is absent or empty | `filter is required: the control plane must supply the default port filter` |
| **400** | Capture id is empty or not `[A-Za-z0-9_-]{1,64}` | `capture id required` |
| **409** | Capture with same ID already running | `capture 'cap-...' already in progress` |
| **500** | Packet source could not be opened (e.g. no `CAP_NET_RAW`) | `failed to start capture: <error>` |
| **500** | Disk is full or other write error | `failed to open capture file: <error>` |

---

### Endpoint: Stop Capture

**Functional Requirement**: FR-001.

#### Request

```
POST /captures/{id}/stop
Host: <gs>-agent.<namespace>.svc.cluster.local:9091   # existing agent Service, numeric port
Content-Type: application/json

{
  "reason": "user_requested"
}
```

**Path Parameters**:
- `{id}` (string): Capture ID.

**Request Body Fields**:
- `reason` (string, optional): Human-readable reason for stopping. Values: `user_requested`, `max_duration_reached`, `max_size_reached`, `pod_restarting`, `error`. Used for logging and status reporting.

**Preconditions**:
- The capture must be known to this sidecar. A capture that has *already* finished — stopped by the
  duration timer, by the size limit, or by an earlier stop — is not an error: the sidecar replays its
  stored terminal result with 200, so the final counters and `stoppingReason` are never lost to a
  client that raced the timer.

#### Response

**Success (200 OK)**:
```json
{
  "status": "completed",
  "captureId": "cap-8f7d3c1a",
  "startedAt": "2026-08-23T14:30:05Z",
  "completedAt": "2026-08-23T14:31:00Z",
  "stoppingReason": "user_requested",
  "bytesWritten": 524288,
  "packetsWritten": 1024,
  "file": "/tmp/captures/capture-cap-8f7d3c1a.pcapng"
}
```

**Response Fields**:
- `status` (string): `completed`, or `failed` when the capture ended on an error (see *Status*).
- `message` (string, omitted when empty): the underlying cause when `status` is `failed`.
- `completedAt` (RFC3339 timestamp): When the capture finished.
- `stoppingReason` (string): Reason captured (echoed from request).
- `bytesWritten` (integer): Final byte count.
- `packetsWritten` (integer): Final packet count.
- `file` (string): Full path to the `.pcapng` file on the emptyDir volume.

#### Error Responses

| Status | Condition | Response Body (plain text) |
|--------|-----------|---|
| **400** | Capture id is malformed | `capture id required` |
| **404** | Capture ID unknown to this sidecar | `capture 'cap-...' not found` |

---

### Endpoint: Get Capture Status

**Functional Requirement**: FR-001 (status polling), FR-002 (limit enforcement).

#### Request

```
GET /captures/{id}/status
Host: <gs>-agent.<namespace>.svc.cluster.local:9091   # existing agent Service, numeric port
```

**Path Parameters**:
- `{id}` (string): Capture ID.

#### Response

**Success (200 OK)** (Capture Running):
```json
{
  "captureId": "cap-8f7d3c1a",
  "status": "running",
  "startedAt": "2026-08-23T14:30:05Z",
  "completedAt": null,
  "stoppingReason": null,
  "bytesWritten": 12288,
  "packetsWritten": 32,
  "estimatedTimeRemainingSeconds": 295,
  "estimatedBytesRemaining": 5356420832
}
```

**Success (200 OK)** (Capture Completed):
```json
{
  "captureId": "cap-8f7d3c1a",
  "status": "completed",
  "startedAt": "2026-08-23T14:30:05Z",
  "completedAt": "2026-08-23T14:31:00Z",
  "stoppingReason": "user_requested",
  "bytesWritten": 524288,
  "packetsWritten": 1024,
  "file": "/tmp/captures/capture-cap-8f7d3c1a.pcapng"
}
```

**Retention of finished captures**: a capture's terminal state stays queryable after it finishes —
the sidecar keeps the last 16 completed captures in memory. This is what makes the "Capture
Completed" response above reachable at all: the operator only *starts* a duration-bounded capture and
then polls, so if a finished capture were discarded, every normal completion would surface to the
user as `Failed` with "sidecar unreachable: status 404".

**Response Fields**:
- `status` (string): `running`, `completed`, or `failed`. `failed` means the capture ended on an
  error — the packet source died mid-run, or the file could not be finalized (a full disk usually
  surfaces at the final flush, not at packet-write time). The file is still finalized and
  downloadable, but it is truncated, and the operator's reconciler maps `failed` to
  `CapturePhaseFailed` rather than reporting a clean completion.
- `message` (string, omitted when empty): the underlying cause when `status` is `failed`.
- `bytesWritten` (integer): Bytes written so far.
- `packetsWritten` (integer): Packets written so far.
- `estimatedTimeRemainingSeconds` (integer, running only): Estimated seconds until max duration. Calculated as `maxDurationSeconds - (now - startedAt)`. Does not account for max-size limit.
- `estimatedBytesRemaining` (integer, running only): Estimated bytes until max-size limit. Calculated as `maxSizeBytes - bytesWritten`.

#### Error Responses

| Status | Condition | Response Body (plain text) |
|--------|-----------|---|
| **400** | Capture id is malformed | `capture id required` |
| **404** | Capture ID not found | `capture 'cap-...' not found` |

---

### Endpoint: Download Capture File

**Functional Requirement**: FR-004 (download), FR-011 (filtered packets only).

#### Request

```
GET /captures/{id}/file
Host: <gs>-agent.<namespace>.svc.cluster.local:9091   # existing agent Service, numeric port
Accept: application/octet-stream
Range: bytes=0-65535  (optional, for resumable downloads)
```

**Path Parameters**:
- `{id}` (string): Capture ID.

**Query Parameters**: None.

**Preconditions**:
- Capture must be in `completed` status. Running captures return 409 Conflict.
- File must exist on the emptyDir volume.

#### Response

**Success (200 OK)**:
```
HTTP/1.1 200 OK
Content-Type: application/vnd.tcpdump.pcap
Content-Disposition: attachment; filename="capture-cap-8f7d3c1a.pcapng"
Content-Length: 524288

[Binary PCAPNG file data]
```

**Response Headers**:
- `Content-Type`: `application/vnd.tcpdump.pcap` (standard MIME type for pcap/pcapng).
- `Content-Disposition`: `attachment; filename="capture-<captureId>.pcapng"` (forces download with suggested filename).
- `Content-Length`: Exact file size in bytes.
- `Accept-Ranges`: `bytes`. Range requests **are** honoured: the sidecar serves the file through
  `http.ServeContent`, which streams without buffering and returns `206 Partial Content` with a
  `Content-Range` header, or `416` for an unsatisfiable range.

**Response Body**: Raw binary PCAPNG file (produced by `github.com/google/gopacket/pcapgo.NgWriter`).

#### Error Responses

| Status | Condition | Response Body (plain text) |
|--------|-----------|---|
| **400** | Capture id is malformed | `capture id required` |
| **404** | No such capture, and no file for it | `capture 'cap-...' not found` |
| **409** | Capture is still running | `capture is still running` |
| **410** | Capture is known to this sidecar but its file is gone (expired/deleted) | `capture file has been deleted` |
| **416** | `Range` header cannot be satisfied | (from `http.ServeContent`) |
| **500** | The file exists but could not be opened or stat'd (EACCES, EIO, …) | `failed to open capture file: <error>` |

410 is deliberately narrow: it means "this sidecar ran this capture and the file is no longer there".
Any other open failure is a real server-side fault and is reported as 500 with the underlying error,
rather than being dressed up as a deletion that sends an operator down the wrong diagnostic path.

---

### Endpoint: Delete Capture File

**Functional Requirement**: FR-007 (retention/cleanup).

#### Request

```
DELETE /captures/{id}
Host: <gs>-agent.<namespace>.svc.cluster.local:9091   # existing agent Service, numeric port
```

**Path Parameters**:
- `{id}` (string): Capture ID.

**Preconditions**:
- Capture must not be running (sidecar rejects running captures with HTTP 409).
- Capture may already be deleted (idempotent — returns 204 even if the file is already gone).

#### Response

**Success (204 No Content)**:
```
HTTP/1.1 204 No Content
```

The capture file has been deleted, or was already absent. The request succeeds in either case (idempotent). No response body. This allows the retention reconciler to safely retry deletion if a request times out or fails transiently.

#### Error Responses

| Status | Condition | Response Body (plain text) |
|--------|-----------|---|
| **400** | Capture id is malformed | `capture id required` |
| **409** | Capture is still running | `capture is still running` |
| **500** | Deletion failed for another reason (e.g., EACCES, EIO) | `failed to delete capture file: <error>` |

---

## Capture File Format and Guarantees

### File Location

**Path**: `/tmp/captures/capture-<captureId>.pcapng`

**Ownership**: Written by the sidecar (UID 65532, nonroot). No other container mounts the
`capture-data` volume — not the game container (FR-008), not the agent — so no other container can
read these files at all. Captures leave the pod only over the sidecar's own mTLS HTTP endpoint.

**Retention**: Lives in emptyDir. Auto-deleted when the pod is restarted or deleted. Manual deletion by the operator or retention reconciler.

### File Format

**Format**: PCAPNG (Pcap Next Generation format), produced by `github.com/google/gopacket/pcapgo.NgWriter`.

**Snaplen**: Full packet capture (65535 bytes per packet; no truncation). Game protocols may have variable-length payloads; full capture ensures nothing is lost.

**Timestamp Precision**: Nanosecond precision (supported by PCAPNG; PCAP is limited to microsecond).

**Interfaces**: The PCAPNG file includes interface metadata (eth0, with snaplen and filter expression documented in the interface block).

### Packet Filtering Guarantee (FR-011)

**Filter Application**: BPF filter is applied **at packet capture time** by `gopacket/afpacket.TPacket` via the kernel's BPF bytecode. The kernel drops packets not matching the filter **before they are copied to userspace**. This is the most efficient filtering mechanism and guarantees correctness:
- 100% of packets in the file match the filter.
- 0% of non-matching packets are included.
- No post-processing or offline filtering.

**Filter Compilation**: The sidecar receives the filter as a string. It compiles it locally using `github.com/packetcap/go-pcap/filter` (same library as API tier, for consistency). **Note**: `github.com/packetcap/go-pcap` has no tagged release (pseudo-version only, e.g. `v0.0.0-20260731...`); the module version must be pinned deliberately in go.mod with rationale in the PR. If compilation fails (defense-in-depth), the start request fails with HTTP 400.

---

## Capture Limits and Enforcement (FR-002)

### Duration Limit

**Mechanism**: The sidecar sets a timer at start time. When `(now - startTime) >= maxDurationSeconds`, the sidecar stops the capture and transitions to `completed` status.

**Enforcement**: Hard limit. The capture stops immediately, even if a packet is mid-arrival.

**Signaling**: The operator can also send a `POST /captures/{id}/stop` request to stop early; duration limit enforcement is independent.

### Size Limit

**Mechanism**: The sidecar monitors the file size during packet write. When `bytesWritten >= maxSizeBytes`, the sidecar closes the file and stops capturing.

**Enforcement**: Hard limit. The final file size may slightly exceed the limit (by one packet) due to the atomic write of each packet, but the limit is enforced before opening the file for the next write. A packet that would exceed the limit is not written.

**Accuracy**: Checked before every packet write. The file size is guaranteed to be ≤ `maxSizeBytes + sizeof(largestPacket)` (typically < 64KB over-run for game traffic).

### What Stops First

**Either-Or Semantics**: The capture stops when **either** limit is reached, whichever comes first. The `stoppingReason` field in status indicates which limit was hit: `max_duration_reached` or `max_size_reached`.

---

## Pod Restart Handling (FR-010)

### Ephemeral Container Lifecycle

**Pod Restart Trigger**: When a GameServer's pod template is updated (game code deploy, operator rollout) or the pod is manually restarted, a rolling update occurs. The old pod is deleted, and a new pod is created.

**Ephemeral Container Loss**: Ephemeral containers are **not replicated** across pod restarts. When the old pod is deleted, the sidecar is lost.

**Active Capture During Restart**: If a capture is running when the pod restarts:
1. The capture process is terminated (no graceful shutdown opportunity for the sidecar).
2. The `.pcapng` file may be left in a partially-written state.
3. The operator detects the pod restart (watch pod events) and marks the NetworkCapture CRD as Failed with reason `pod_restarted`.

### Cleanup on Pod Restart

**Partial File Handling**: Partial captures are left on the old emptyDir (lost when the pod is deleted). A clean marker (e.g., writing a `.incomplete` suffix or metadata flag) is not used for v1 (too complex). Files are identified by their inode; pod deletion clears the emptyDir.

**CRD State**: The operator's NetworkCaptureReconciler watches for pod events. If a pod is recreated, the reconciler marks orphaned captures (those with `status.phase = Running` on a deleted/recreated pod) as Failed with reason `pod_restarted`.

**New Sidecar on Restarted Pod**: The new pod has a fresh ephemeral container (if `spec.capture.enabled` is still true). Previous captures are inaccessible but remain queryable in the NetworkCapture CRD with status = Failed or Completed.

---

## Error Handling and Resilience

### Disk Full

**Detection**: During packet write, the sidecar receives an OS error (ENOSPC) when the emptyDir sizeLimit is hit.

**Response**: The sidecar immediately stops the capture, closes the file, and marks the NetworkCapture status as Failed with reason `disk_full` and error message `"disk full: captured <N> packets before hitting size limit"`.

**Recovery**: The operator cannot restart a failed capture; the user must initiate a new capture.

### Kernel Buffer Overflow (High Packet Rate)

**Detection**: AF_PACKET's MMap'd buffer may overflow under extreme packet rates (e.g., 100k packets/sec). The sidecar detects when the kernel's drop counter increases (via TPacket statistics).

**Response**: The sidecar logs a warning and continues capturing (doesn't stop). Dropped packets are noted in the PCAPNG interface statistics block. The capture is still valid (not corrupt) but incomplete.

**Mitigation**: Operators must use filters to reduce traffic on busy servers. The spec and research acknowledge this (FR-003 notes that unfiltered captures on busy servers will fill quickly).

### Sidecar Crash

**Scenario**: The sidecar process crashes unexpectedly (OOM, panic, SIGSEGV).

**Behavior**: Ephemeral containers **cannot be restarted** by Kubernetes. Once the process exits, the ephemeral container is permanently gone (it remains in pod status but is not automatically respawned, unlike regular containers with restartPolicy). The capture is immediately lost; the partial file is left on disk (later cleaned by pod restart or manual deletion).

**No Retry Mechanism**: There is no automatic recovery or retry. The capture does not resume on a subsequent request; the sidecar must be re-injected into the pod (via a new ephemeral container injection, if the pod is still running and capture is still enabled). The simpler path is pod restart (which re-injects a fresh sidecar) or manual user initiation of a new capture.

**Audit Trail**: The operator detects the missing Running capture (pod still exists, but sidecar process is gone) and marks it Failed with reason `sidecar_crashed`. The user must initiate a new capture if needed.

---

## Authentication and Authorization

### mTLS

**Client Authentication**: The API server authenticates to the sidecar using mTLS. The API holds its own client certificate and validates the sidecar's server certificate.

**Certificate Details**:
- Sidecar certificate: **the existing agent server certificate**, reused as-is — the `tls.crt` /
  `tls.key` already in the `<gs>-agent-tls` Secret, signed by the agent CA
  (`operator/internal/controller/agent_certs.go:38`). No capture-specific certificate is issued.
- SANs: the DNS names produced by `agentDNSNames`
  (`operator/internal/controller/agent_certs.go:207-224`). The name the API dials for captures,
  `<gs>-agent.<ns>.svc.cluster.local`, is already among them, along with `<gs>-agent`,
  `<gs>-agent.<ns>` and `<gs>-agent.<ns>.svc`.
- The certificate is **not** issued to `<pod-name>.<namespace>.svc.cluster.local`; no such SAN
  exists. The pod-DNS SAN it does carry is `<gs>-0.<gs>.<ns>.svc.cluster.local`.
- There is **no IP SAN** — `IPAddresses` is never set in `agent_certs.go` — so the API must dial the
  Service DNS name, never the pod IP.
- API client certificate: The API's own certificate (shared with other components).
- Certificate validation: Both sides validate the peer certificate against the agent CA.

**No API Key or Bearer Token**: The sidecar does not use API keys or Bearer tokens (mTLS is the auth mechanism).

### Authorization

**Implicit**: The sidecar is part of the game pod and has no user identity. Authorization happens at the API tier (rest-api.md FR-005), not at the sidecar. The sidecar trusts all requests over mTLS (from the API server only, due to certificate validation).

---

## Operational Guarantees

### Capture Data Consistency

**Guarantee 1** (Filter Correctness): If a filter is provided, the captured file contains only packets matching that filter. No non-matching packets are included (FR-011).

**Guarantee 2** (File Integrity): The `.pcapng` file is always in a valid format (proper headers, interface blocks, packet blocks). Tools like `tcpdump -r` and Wireshark can open and read the file without corruption.

**Guarantee 3** (Limit Enforcement): The capture stops when the duration limit or size limit is reached. Both are hard limits (not soft). The capture does not continue past the limit.

**Guarantee 4** (No Game Container Modification): The sidecar never modifies the game container's runtime, filesystem, or network stack. The game container is unaware of the sidecar (FR-008). The `CAP_NET_RAW` required for packet capture lives on the capture binary as a file capability and is scoped to the capture container only; the game container's securityContext is unchanged and retains no elevated capabilities. The `allowPrivilegeEscalation: true` this forces is likewise scoped to the capture container alone (see "Raw-socket access"), and it is the reason the namespace needs a PodSecurity `restricted` exception.

**Guarantee 5** (Isolation): The sidecar has no access to the game container's filesystem or
processes. Its only writable surface is the capture emptyDir, which no other container mounts.

### Availability and Best Effort

**Guarantee 6** (Graceful Degradation): If the sidecar cannot start a capture (e.g., filter is invalid, disk is full), it returns a clear error. The game server is not affected; the pod remains Running and playable.

**Guarantee 7** (Resource Bounds): **No cgroup limit applies to the sidecar.** Ephemeral containers
cannot carry resource requests or limits (Constraint 3 above), and a container's limits are per
container — the game container's limits do not bound the sidecar. The only bounds that actually
exist are:

- the sidecar's own internal `maxDurationSeconds` and `maxSizeBytes` enforcement, which bounds how
  long and how large a single capture runs; and
- the capture emptyDir's `sizeLimit`, which bounds disk.

Memory and CPU are therefore bounded only by the sidecar's own implementation discipline (bounded
ring buffers, no unbounded accumulation) — not by the kernel. Under sustained high packet rates this
is a real risk surface, and it is not mitigated by the platform.

---

## Related Contracts

- **rest-api.md**: HTTP REST interface for operators to trigger and monitor captures.
- **audit-events.md**: Audit logging schema for all capture operations.

---

**Document compiled**: 2026-08-23  
**Status**: Ready for Phase 1 (Contracts Review)  
**Next step**: Implementation of sidecar binary and operator reconciliation in Phase 1.
