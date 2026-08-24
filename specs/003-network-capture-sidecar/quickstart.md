# Quickstart & Validation Guide: Network Capture Sidecar for Game Servers

**Feature**: `003-network-capture-sidecar`  
**Phase**: Phase 1 Validation (post-implementation)  
**Scope**: Runnable scenarios proving SC-001 through SC-008 have been met.

---

## Where This Runs

Per Constitution Principle VI and CLAUDE.md rule 8, **all validation runs on GitHub Actions CI or against the operator-provided live cluster (`~/kubelab.yaml`, context `default`)**. Local machine execution is not permitted. Commands below are documented for CI to execute and for maintainers to understand; they are reference only for developers.

---

## Prerequisites

### For All Scenarios

1. **Kubernetes Cluster**: Either:
   - GitHub Actions CI environment (kind cluster, provided by CI)
   - Remote kubelab cluster (kubeconfig at `~/kubelab.yaml`, context `default`)
2. **Gameplane installed**: Operator, API, and agent running in the cluster
3. **NetworkCapture CRD**: Present in the cluster (via `make manifests` during implementation)
4. **kubectl** access: Able to create/read/list GameServer, NetworkCapture, Pod, Service resources
5. **Game module available**: A lightweight module (Minecraft Java, Terraria) for testing; a test GameServer already bootstrapped from a prior scenario is acceptable

### For Live Cluster Validation Only

- API port-forward access (for REST capture endpoints)
- WebSocket connectivity to game pod sidecars: either the agent proxy, or the capture container's
  own endpoint reached by a **numerically-targeted port 9091 on the existing `<gs>-agent`
  ClusterIP Service** (an ephemeral container cannot declare a named `containerPort`, so the
  Service must select the port by number). Dial the Service's DNS name — `<gs>-agent`,
  `<gs>-agent.<ns>`, `<gs>-agent.<ns>.svc`, or `<gs>-agent.<ns>.svc.cluster.local` — never the pod
  IP directly: the agent's mTLS certificate SANs cover only those DNS names (see the doc comment
  on `agentDNSNames` in `operator/internal/controller/agent_certs.go`) and carry no IP SAN, so a
  pod-IP dial fails certificate verification. Adding the 9091 port to that Service is operator
  work this feature requires; it is not assumed to already exist.
- Standard third-party tools: `tshark` or `tcpdump` to verify captured pcap/pcapng files are readable

---

## Success Criteria Reference Map

| Success Criterion | Observable Proof | Validation Scenario |
|---|---|---|
| **SC-001** | Captured file is valid PCAPNG, readable by Wireshark/tshark | US1.3 (capture output file verification) |
| **SC-002** | Zero perceptible player-experience impact (packet loss & latency) | SC-002 Scenario: Performance comparison with capture ON vs OFF |
| **SC-003** | 100% of capture operations (start/stop/download/delete) are recorded in the audit log with operator identity, server name, capture ID, timestamp, and result | US3.2 (audit logging) |
| **SC-004** | Expired capture 404s or returns "expired" error | US4 (retention expiry) |
| **SC-005** | Non-admin POST `/servers/{name}:capture-start` returns 403 | US3 (admin-only enforcement) |
| **SC-006** | Concurrent capture requests on the same server are serialized or rejected; the second request receives a clear error | US5.2 (concurrent capture rejection) |
| **SC-007** | Non-opted-in GameServer has no capture container (it MAY carry an empty, pre-provisioned capture-storage volume — see Decision below); `kubectl diff` shows only the sidecar container addition, never a change to the game container | US2 + Negative Check (enable/disable lifecycle) |
| **SC-008** | All packets in output file match filter; zero packets not matching filter included | US1.3 (filter compliance) |

---

## Scenario: US1.1 — Initiate a Capture with Filter (SC-001)

**Objective**: Start a manual capture on an opted-in GameServer, verify the sidecar receives the request and begins capturing.

### Setup

```bash
# Prerequisites: A GameServer with spec.capture.enabled=true is already running
# Example via kubectl:
cat <<'EOF' | kubectl apply -f -
apiVersion: gameplane.local/v1alpha1
kind: GameServer
metadata:
  name: capture-test-1
  namespace: default
spec:
  templateRef:
    name: minecraft-java  # or terraria; lightweight games preferable
  networking:
    expose: LoadBalancer
  capture:
    enabled: true
EOF

# Wait for pod to be Running
kubectl get pod -l app.kubernetes.io/instance=capture-test-1 -w
# Expected: Pod enters Running state within ~5 minutes (depends on module image size)
```

### Initiate Capture via API

```bash
# Get API endpoint (from port-forward or cluster service)
# For CI: API is in-cluster at http://gameplane-api.gameplane-system.svc.cluster.local:8000
# For live cluster: use port-forward
kubectl port-forward -n gameplane-system svc/gameplane-api 8000:8000 &

# Initiate capture with filter expression
curl -X POST http://localhost:8000/servers/capture-test-1:capture-start \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "filter": "udp port 25565",
    "maxDurationSeconds": 60,
    "maxSizeBytes": 104857600
  }'

# Expected response (HTTP 202 Accepted — Pending, not Completed; the operator
# reconciler transitions it to Running):
# {
#   "captureId": "cap-<uuid>",
#   "phase": "Pending",
#   "serverName": "capture-test-1",
#   "createdAt": "2026-08-23T12:34:56Z",
#   "startedAt": null
# }
```

### Validation

```bash
# Verify the NetworkCapture CRD was created
kubectl get networkcapture
# Expected: One entry with name matching the captureId, phase=Pending or Running

# Check the NetworkCapture status
kubectl get networkcapture cap-<uuid> -o jsonpath='{.status.phase}'
# Expected: "Pending" (briefly) or "Running" (capture active)

# Verify the capture sidecar is running in the pod
kubectl describe pod <pod-name> | grep -A 10 "Containers:"
# Expected: Both "minecraft" (or game) and "capture" containers listed
# Sidecar status: Running
```

**PASS Criteria (SC-001, step 1)**:
- NetworkCapture CRD created with correct captureId
- Capture sidecar container is Running (visible in `kubectl describe pod`)
- Status transitions from Pending → Running within 5 seconds

---

## Scenario: US1.2 — Send Traffic Matching Filter

**Objective**: Generate network traffic matching the filter expression and verify the sidecar captures packets.

### Setup (using real game traffic)

```bash
# Option A: Real game client joins the server
# Get the server's public address
PUBLIC_ADDR=$(kubectl get svc capture-test-1 -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
PUBLIC_PORT=$(kubectl get svc capture-test-1 -o jsonpath='{.spec.ports[0].port}')
echo "Server accessible at $PUBLIC_ADDR:$PUBLIC_PORT"

# Launch a real game client (Minecraft Java Edition, Terraria, etc.) and join
# Expected: Game connects successfully and exchanges packets

# Allow the client to exchange packets with the server for ~10 seconds
sleep 10
```

### Setup (using synthetic traffic probe, if real client unavailable)

```bash
# For headless CI, use a minimal join probe (mcproto or similar)
# This is a placeholder; real implementation uses the test/e2e/gameprobe helpers
# Example concept (not runnable as-is without gameprobe package):

# Send a valid Minecraft join handshake to the server
(
  # Construct a Minecraft Handshake packet (simplified)
  # In practice: use existing gameprobe/minecraft module
  printf "<handshake-bytes>" | nc -u "$PUBLIC_ADDR" "$PUBLIC_PORT"
) &

sleep 3
```

### Validation (check capture is collecting packets)

```bash
# Query the capture status. Real route: GET /servers/{name}:captures (the fixed ":captures"
# suffix is required for the RBAC rule table to match this route ahead of the servers:read
# catch-all — see contracts/rest-api.md "Routing Conventions"; the nested
# GET /servers/{name}/captures shape does not exist).
curl -X GET http://localhost:8000/servers/capture-test-1:captures \
  -H "Authorization: Bearer <admin-token>"

# Expected response:
# {
#   "captures": [
#     {
#       "captureId": "cap-<uuid>",
#       "phase": "Running",
#       "bytesWritten": 1024,
#       "packetsWritten": 5,
#       "startedAt": "2026-08-23T12:34:56Z"
#     }
#   ],
#   "total": 1,
#   "limit": 100,
#   "offset": 0
# }

# Verify bytesWritten > 0 and packetsWritten > 0
# This confirms the sidecar is capturing traffic
```

**PASS Criteria (US1.2)**:
- NetworkCapture status shows bytesWritten > 0
- NetworkCapture status shows packetsWritten > 0
- No errors in NetworkCapture.status.message

---

## Scenario: US1.3 — Stop Capture and Download File (SC-001, SC-008)

**Objective**: Stop the capture and retrieve the pcap file for analysis.

### Stop Capture

```bash
# Stop the capture via API
curl -X POST http://localhost:8000/servers/capture-test-1:capture-stop \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{"captureId": "cap-<uuid>"}'

# Expected response (HTTP 200 OK):
# {
#   "phase": "Completed",
#   "bytesWritten": 1024,
#   "packetsWritten": 5,
#   "completionTime": "2026-08-23T12:34:56Z"
# }
```

### Download Captured File

```bash
# Download the pcap file via API endpoint. Real route: GET /servers/{name}:capture-file?id={id}
# (the capture ID moves to a query parameter so the path suffix stays fixed and RBAC-matchable —
# GET /servers/{name}/captures/{id}/file does not exist; see contracts/rest-api.md). Whether the
# handler proxies through the agent or streams from the sidecar is an internal implementation
# choice, not a second route the caller sees.
curl -X GET "http://localhost:8000/servers/capture-test-1:capture-file?id=cap-<uuid>" \
  -H "Authorization: Bearer <admin-token>" \
  -o capture-test-1.pcapng

# Verify file exists and has content
ls -lh capture-test-1.pcapng
# Expected: File size > 100 bytes (at least a pcapng header + a few packets)
```

### Verify File Integrity (SC-001)

```bash
# Use standard third-party tools to verify the pcap/pcapng file
# Option 1: tshark (Wireshark's command-line tool)
tshark -r capture-test-1.pcapng -c 5
# Expected output: Packet listing showing captured traffic
# Example:
#   1   0.000000    192.0.2.10 → 192.0.2.100  MINECRAFT  Length: 512
#   2   0.001234    192.0.2.100 → 192.0.2.10  MINECRAFT  Length: 128
#   ...

# Option 2: tcpdump (read & validate file structure)
tcpdump -r capture-test-1.pcapng -c 5
# Expected: Packet summary lines (same as tshark, but simpler format)

# Option 3: pcapfix (if file corruption is suspected)
pcapfix capture-test-1.pcapng -o capture-test-1-fixed.pcapng
# Expected: No repairs needed (exit code 0); file is valid
```

### Verify Filter Compliance (SC-008)

```bash
# Analyze packets with tshark to verify ALL match the filter expression
# Recall: filter was "udp port 25565" (Minecraft game port)

tshark -r capture-test-1.pcapng -Y "!udp.port == 25565" -c 1
# Expected: No output (zero non-matching packets)
# If there IS output, the capture includes packets that should have been filtered out.

# Count total packets and matching packets
TOTAL=$(tshark -r capture-test-1.pcapng | wc -l)
MATCHING=$(tshark -r capture-test-1.pcapng -Y "udp.port == 25565" | wc -l)
echo "Total packets: $TOTAL, Matching filter: $MATCHING"
# Expected: Total == Matching (all packets match filter)
```

**PASS Criteria (SC-001, SC-008)**:
- File is readable by tshark without errors
- File contains captured packets (> 0 packets)
- All packets in the file satisfy the filter criteria (100% match)
- Zero packets that do NOT match the filter are present (0% non-matching)

---

## Scenario: SC-002 — Zero Perceptible Player-Experience Impact

**Objective**: Measure that capture introduces no measurable packet loss or latency degradation to game traffic.

**⚠️ DETAILED PROCEDURE**: The complete SC-002 benchmark procedure — including exact setup, measurement tools, pass/fail criteria, confounder controls, and troubleshooting — is documented in `sc-002-benchmark.md`. This scenario provides a high-level overview; refer to that document for the full, executable procedure.

**Status**: **NOT YET MEASURED** — This benchmark has not been executed on a live cluster. The detailed procedure in `sc-002-benchmark.md` is ready for execution.

**Context**: This scenario validates SC-002 ("zero perceptible player-experience impact") by comparing network metrics with capture enabled vs. disabled. Real game clients (Minecraft join bot, Terraria client, etc.) are used to establish baseline and test performance.

### Setup: Two Baseline GameServers (Capture ON/OFF)

```bash
# Server A: Capture ENABLED
cat <<'EOF' | kubectl apply -f -
apiVersion: gameplane.local/v1alpha1
kind: GameServer
metadata:
  name: perf-test-capture-on
  namespace: default
spec:
  templateRef:
    name: minecraft-java
  networking:
    expose: LoadBalancer
  capture:
    enabled: true
EOF

# Server B: Capture DISABLED (baseline)
cat <<'EOF' | kubectl apply -f -
apiVersion: gameplane.local/v1alpha1
kind: GameServer
metadata:
  name: perf-test-capture-off
  namespace: default
spec:
  templateRef:
    name: minecraft-java
  networking:
    expose: LoadBalancer
EOF

# Wait for both pods to be Running
kubectl get pod -l app.kubernetes.io/instance=perf-test-capture-on -w
kubectl get pod -l app.kubernetes.io/instance=perf-test-capture-off -w
```

### Run Game Traffic and Measure Latency/Loss

For detailed SC-002 benchmark procedure, see `sc-002-benchmark.md`. Below is a summary of the measurement approach.

**Important**: The test/e2e suite includes in-cluster game-probes (`test/e2e/internal/<game>/app.go`), but these are single-connection join probes only — they do NOT generate sustained traffic or emit latency metrics. For this benchmark, use:
- **Real game client** (recommended for this feature) — e.g., a Minecraft join bot, Terraria client
- **iperf3** or **ping** — standard tools for network measurement
- See `sc-002-benchmark.md` "Tools Available" section for full details and tradeoffs

```bash
# Start an active capture on server A (to ensure the sidecar is exercising packet I/O)
CAPTURE_ID=$(curl -s -X POST http://localhost:8000/servers/perf-test-capture-on:capture-start \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "filter": "udp",
    "maxDurationSeconds": 120,
    "maxSizeBytes": 104857600
  }' | jq -r '.captureId')

echo "Capture ID on Server A: $CAPTURE_ID"

# Run your chosen traffic generator (real client, iperf3, or similar)
# This measures RTT, packet loss, and throughput for comparison.

# Expected metrics to capture:
# - Packets sent from client
# - Packets received at server (dropped = sent - received)
# - Round-trip time (RTT) samples: min, max, mean, stddev
# - Throughput (bytes/sec)

TIMEOUT=60
echo "Connecting to perf-test-capture-off (no capture)..."
# Example: use iperf3 (replace with your tool of choice)
iperf3 -c perf-test-capture-off -p 25565 -u -b 1M -t "$TIMEOUT" -J > /tmp/perf-baseline.json

echo "Connecting to perf-test-capture-on (with capture)..."
iperf3 -c perf-test-capture-on -p 25565 -u -b 1M -t "$TIMEOUT" -J > /tmp/perf-capture.json
```

### Analyze Results and Verify Pass Condition

```bash
# Parse the JSON output files and extract metrics
# Both files should have structure like:
# {
#   "packets_sent": 1024,
#   "packets_received": 1024,
#   "rtt_min_ms": 2.5,
#   "rtt_max_ms": 15.3,
#   "rtt_mean_ms": 5.2,
#   "rtt_stddev_ms": 1.8,
#   "throughput_kbps": 512
# }

# Extract key metrics for comparison
BASELINE_LOSS=$(jq '.packets_sent - .packets_received' /tmp/perf-baseline.json)
CAPTURE_LOSS=$(jq '.packets_sent - .packets_received' /tmp/perf-capture.json)

BASELINE_RTT=$(jq '.rtt_mean_ms' /tmp/perf-baseline.json)
CAPTURE_RTT=$(jq '.rtt_mean_ms' /tmp/perf-capture.json)

echo "Baseline packet loss: $BASELINE_LOSS packets"
echo "Capture packet loss:  $CAPTURE_LOSS packets"
echo "Baseline RTT:         $BASELINE_RTT ms"
echo "Capture RTT:          $CAPTURE_RTT ms"

# Calculate relative deltas
LOSS_DELTA=$(( (CAPTURE_LOSS - BASELINE_LOSS) * 100 / (BASELINE_LOSS + 1) ))
RTT_DELTA=$(echo "scale=2; ($CAPTURE_RTT - $BASELINE_RTT) * 100 / $BASELINE_RTT" | bc)

echo "Packet loss delta:    $LOSS_DELTA %"
echo "RTT delta:            $RTT_DELTA %"

# Stop the capture
curl -X POST http://localhost:8000/servers/perf-test-capture-on:capture-stop \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d "{\"captureId\": \"$CAPTURE_ID\"}"
```

### Measurable Pass Condition (SC-002)

**PASS if ALL of the following are true**:
1. **Packet loss is identical or better with capture ON**: `CAPTURE_LOSS <= BASELINE_LOSS + 1` (tolerance: 1 lost packet due to randomness)
2. **Latency increase is imperceptible**: `RTT_DELTA <= 5%` (mean RTT with capture is within 5% of baseline)
3. **No error responses from either server**: Both bots report successful join/gameplay (not timeouts or disconnects)
4. **Capture file contains expected packets**: The pcapng file written during this run is valid and contains game traffic (verified with tshark)

**FAIL if any of the following occur**:
- Packet loss increases by > 1 packet due to capture
- Mean RTT increases by > 5% due to capture
- Either server becomes unresponsive or times out during the test
- Sidecar crashes or CPU usage spikes abnormally (check `kubectl top pod`)

### Notes on CI vs. Live Cluster Validation

- **CI**: This scenario does not run in CI. The kind cluster's synthetic network is too tightly controlled for meaningful performance assertions; a CI version would only verify that no crashes occur (a structural test, not a performance test).
- **Live Cluster Only**: The SC-002 benchmark is **manual and live-cluster-only** — run against the kubelab cluster or equivalent with real network conditions. The detailed procedure in `sc-002-benchmark.md` provides exact steps, tools, confounder controls, and pass/fail thresholds for a maintainer to execute.
- **Current Status**: As of this document's creation, this benchmark has **NOT been executed**. No measurement has been taken, and no pass/fail result has been recorded. See `sc-002-benchmark.md` for the ready-to-execute procedure.

**PASS Criteria (SC-002)**:
- Packet loss: capture ≤ baseline + 1 packet
- Latency: capture RTT ≤ baseline RTT + 5%
- Both servers remain responsive throughout the test
- Captured pcapng file is valid and contains game traffic

---

## Scenario: US2.1 — Enable Capture on a Running GameServer (SC-007)

**Objective**: Verify capture can be added to a running server without restarting the game container.

**Note on the pre-provisioned volume (Decision, not open for re-litigation)**: the capture
`emptyDir` volume is added to the StatefulSet pod template **unconditionally**, for every game
pod, whether or not it opts into capture — the `pods/ephemeralcontainers` subresource cannot add
a volume, and `pod.spec.volumes` is immutable on a running pod, so the volume must already exist
before the ephemeral capture container can be injected. This means:
- Upgrading to the release that adds this volume causes a **one-time rolling restart of every
  existing game server**, opted in or not. This must be called out to operators in the upgrade
  notes before they upgrade.
- A non-opted-in GameServer is **not** byte-identical to a pre-feature GameServer: it carries an
  empty, unused capture-storage volume. What it does **not** carry is the capture container. Every
  "identical pod spec" assertion below is scoped to "no capture container", not "no capture
  volume".

### Setup: Create GameServer WITHOUT Capture

```bash
cat <<'EOF' | kubectl apply -f -
apiVersion: gameplane.local/v1alpha1
kind: GameServer
metadata:
  name: capture-test-2
  namespace: default
spec:
  templateRef:
    name: minecraft-java
  networking:
    expose: LoadBalancer
  # capture field intentionally absent or set to enabled: false
EOF

# Wait for pod to be Running
kubectl get pod -l app.kubernetes.io/instance=capture-test-2 -w

# Record the pod name and snapshot the pod spec
POD_NAME=$(kubectl get pod -l app.kubernetes.io/instance=capture-test-2 -o jsonpath='{.items[0].metadata.name}')
kubectl get pod "$POD_NAME" -o yaml > pod-before-enable.yaml
```

### Enable Capture via REST API

Per `contracts/rest-api.md`, enable/disable is a dedicated verb-form route, not the generic
`PATCH /servers/{name}` route — that route is already matched by the `servers:write` RBAC
catch-all the `operator` role holds, so it cannot carry the stricter `captures:manage`
permission without also re-gating ordinary server edits. The route below is the real one:

```bash
curl -X POST http://localhost:8000/servers/capture-test-2:capture-enable \
  -H "Authorization: Bearer <admin-token>"

# Expected response (HTTP 200 OK):
# {
#   "name": "capture-test-2",
#   "status": {
#     "capture": {
#       "ready": false,
#       "activeCapture": null,
#       "lastCaptureTime": null,
#       "sidecarRestarts": 0
#     }
#   }
# }
```

This single call is the mechanism: per the ephemeral-container decision, it patches the
GameServer's `pods/ephemeralcontainers` subresource directly to inject the capture sidecar into
the already-running pod. No game-container restart occurs, and there is no separate `kubectl
patch spec.capture.enabled` step required to trigger injection.

### Verify Sidecar Added WITHOUT Game Container Restart (SC-007)

```bash
# Check pod status (should still be Running)
kubectl get pod "$POD_NAME" -o jsonpath='{.status.phase}'
# Expected: "Running"

# Verify game container is still running (no restart)
kubectl get pod "$POD_NAME" -o jsonpath='{.status.containerStatuses[0].restartCount}'
# Expected: "0" (no restart since pod creation)

# Verify ephemeralContainers now includes capture sidecar
kubectl get pod "$POD_NAME" -o jsonpath='{.spec.ephemeralContainers[*].name}'
# Expected: "capture" (or similar sidecar name)

# Take a snapshot after enable
kubectl get pod "$POD_NAME" -o yaml > pod-after-enable.yaml

# Diff the snapshots to verify only the sidecar container was added
diff -u pod-before-enable.yaml pod-after-enable.yaml
# Expected: Only spec.ephemeralContainers gains the capture container entry;
# spec.containers is unchanged. The pre-provisioned capture emptyDir volume is
# present in BOTH snapshots (it is added unconditionally to every pod template,
# opted in or not — see the Decision note above), so it is NOT expected to
# appear in this diff.
```

**PASS Criteria (SC-007, US2.1)**:
- Game pod remains Running throughout
- Game container has restartCount = 0 (not restarted)
- Capture ephemeralContainer is added to the pod
- `kubectl diff` between the before/after snapshots shows only the capture container being
  added to `spec.ephemeralContainers` — no change to `spec.containers` and no change to
  `spec.volumes` (the capture volume was already present before enable)

---

## Scenario: US2.2 — Disable Capture (Requires Pod Restart)

**Objective**: Verify capture can be disabled. Document that disabling sets the capability to disabled, but the ephemeral container persists in the running pod until the pod is recreated on the next reconciliation.

**Important**: Kubernetes provides no API to remove an ephemeral container from a running pod. Disabling capture stops any active capture and clears the capability **immediately** — no new capture can be started, and the `capture-disable` response's `ready` field drops to `false` in that same response — but the ephemeralContainer itself remains visible in pod status until the pod is recreated. This is expected behavior per the platform constraint.

### Disable Capture via REST API

```bash
# Disable capture on the running pod
curl -X POST http://localhost:8000/servers/capture-test-2:capture-disable \
  -H "Authorization: Bearer <admin-token>"

# Expected response (HTTP 200 OK):
# {
#   "name": "capture-test-2",
#   "status": {
#     "capture": {
#       "ready": false,
#       "activeCapture": null,
#       "lastCaptureTime": "2026-08-23T14:31:00Z",
#       "sidecarRestarts": 0
#     }
#   }
# }

# The pod REMAINS RUNNING at this point
# The ephemeral container is still present in pod status but the capability is off:
# no new capture can be started
```

### Verify Disable Takes Effect (SC-007)

```bash
# Attempt a capture start on the disabled server
curl -X POST http://localhost:8000/servers/capture-test-2:capture-start \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{"filter": "udp", "maxDurationSeconds": 60, "maxSizeBytes": 104857600}'

# Expected response: HTTP 400, plain-text body (httperr.Write / http.Error —
# there is no JSON error envelope anywhere in this API; see the RBAC and Error
# Handling section of contracts/rest-api.md):
#   capture is not enabled on this server

# Note: The ephemeralContainer is STILL VISIBLE in the running pod
POD_NAME=$(kubectl get pod -l app.kubernetes.io/instance=capture-test-2 -o jsonpath='{.items[0].metadata.name}')
kubectl get pod "$POD_NAME" -o jsonpath='{.spec.ephemeralContainers[*].name}'
# Expected: "capture" (sidecar is still there until pod is recreated)
```

### Pod Restart Lifecycle (Reconciliation Cleanup)

```bash
# When the controller next reconciles this GameServer, it will recreate the pod
# (This happens automatically; you can also force it by deleting the pod)

# Option 1: Let the operator reconcile naturally (may take up to 15 seconds)
# Option 2: Trigger immediate reconciliation by deleting the pod
kubectl delete pod "$POD_NAME"

# Watch for pod restart
kubectl get pod -l app.kubernetes.io/instance=capture-test-2 -w
# Expected: Old pod terminates; new pod is created and reaches Running

# Get the new pod name
NEW_POD_NAME=$(kubectl get pod -l app.kubernetes.io/instance=capture-test-2 -o jsonpath='{.items[0].metadata.name}')

# NOW verify ephemeralContainers is empty in the NEW pod
kubectl get pod "$NEW_POD_NAME" -o jsonpath='{.spec.ephemeralContainers}'
# Expected: null or empty array (sidecar is gone in the fresh pod)

# Verify game container is operational (one restart expected during pod recreation)
kubectl get pod "$NEW_POD_NAME" -o jsonpath='{.status.containerStatuses[0].restartCount}'
# Expected: "0" (new pod has zero restarts)
```

**PASS Criteria (US2.2, SC-007)**:
- `spec.capture.enabled: false` is set on the GameServer
- Capture start requests return 400 (capability disabled), plain-text body, on running pod
- Ephemeral container is visible in running pod status (expected platform constraint)
- Ephemeral container is REMOVED from new pod after pod recreation
- Game remains playable in new pod
- The pre-provisioned capture emptyDir volume is still present in `spec.volumes` of the new
  pod (it is never removed — every game pod carries it, opted in or not); the only thing
  absent from the final pod spec is the capture container

---

## Scenario: US3.1 — Enforce Admin-Only Access (SC-005)

**Objective**: Verify non-admin users cannot start captures; admin users can.

### Setup: Two Users (Admin and Non-Admin)

Assuming the cluster has auth configured (e.g., OIDC or local users):
- Admin token: `<admin-token>` (user with admin role)
- Non-admin token: `<viewer-token>` (user with viewer or operator role)

### Test Non-Admin Rejection

```bash
# Attempt capture start as a non-admin user
curl -X POST http://localhost:8000/servers/capture-test-1:capture-start \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <viewer-token>" \
  -d '{
    "filter": "udp port 25565",
    "maxDurationSeconds": 60,
    "maxSizeBytes": 104857600
  }'

# Expected response: HTTP 403, plain-text body (rbac.Middleware, not a JSON
# envelope — see api/internal/rbac/rbac.go):
#   forbidden
```

### Test Admin Success

```bash
# Same request with admin token
curl -X POST http://localhost:8000/servers/capture-test-1:capture-start \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "filter": "udp port 25565",
    "maxDurationSeconds": 60,
    "maxSizeBytes": 104857600
  }'

# Expected response (HTTP 202 Accepted):
# {
#   "captureId": "cap-<uuid>",
#   "phase": "Pending"
# }
```

**PASS Criteria (SC-005)**:
- Non-admin user receives HTTP 403 with plain-text body `forbidden`
- Admin user receives HTTP 202
- Both outcomes are recorded as audit events (SC-003)

---

## Scenario: US3.2 — Audit Logging (FR-006)

**Objective**: Verify all capture operations are logged with full audit trail.

### Initiate & Complete a Capture (as admin)

```bash
# Start a capture
CAPTURE_ID=$(curl -s -X POST http://localhost:8000/servers/capture-test-1:capture-start \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "filter": "tcp port 22",
    "maxDurationSeconds": 10,
    "maxSizeBytes": 10485760
  }' | jq -r '.captureId')

echo "Capture ID: $CAPTURE_ID"

# Stop the capture
curl -X POST http://localhost:8000/servers/capture-test-1:capture-stop \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d "{\"captureId\": \"$CAPTURE_ID\"}"

# Download the file. Real route: GET /servers/{name}:capture-file?id={id} (see the download
# scenario above and contracts/rest-api.md — the nested /captures/{id}/file shape does not exist).
curl -X GET "http://localhost:8000/servers/capture-test-1:capture-file?id=$CAPTURE_ID" \
  -H "Authorization: Bearer <admin-token>" \
  -o /tmp/capture.pcapng
```

### Query Audit Log

```bash
# Fetch audit events (API endpoint: GET /admin/audit). The handler
# (api/internal/handlers/audit.go) reads ONLY the 'limit' and 'before' query params — there is no
# 'target' filter, so a "?target=$CAPTURE_ID" query string is silently ignored and the whole first
# page is returned regardless. Filter client-side instead. The response body is also a BARE JSON
# array (audit.WriteJSON encodes []audit.Event directly, api/internal/audit/audit.go:936-939) —
# there is no {"events": [...]} envelope, so 'jq .[]' is correct here and 'jq .events[]' is not.
curl -X GET "http://localhost:8000/admin/audit?limit=100" \
  -H "Authorization: Bearer <admin-token>" \
  | jq --arg id "$CAPTURE_ID" '.[] | select((.target // "") | contains($id))'

# Adding a server-side target filter to GET /admin/audit (a real ?target= query param honored by
# the handler) is new work this feature would need to request separately; it does not exist today.

# Expected audit entries (one per operation), after client-side filtering. Fields are exactly
# audit.Event's columns (api/internal/audit/audit.go:727-736: ID, TS, Actor, Method, Path, Target,
# Status, IP) — there is no "details"/payload column, so filter/captureId are not recoverable from
# the audit row itself, only from Target and Path. The raw (unfiltered) response is a bare array:
# [
#   {
#     "id": "...",
#     "ts": "2026-08-23T12:34:56Z",
#     "actor": "user@example.com",
#     "method": "POST",
#     "path": "/servers/capture-test-1:capture-start",
#     "target": "capture-test-1:cap-<uuid>",
#     "status": 202,
#     "ip": "..."
#   },
#   {
#     "id": "...",
#     "ts": "2026-08-23T12:34:57Z",
#     "actor": "user@example.com",
#     "method": "POST",
#     "path": "/servers/capture-test-1:capture-stop",
#     "target": "capture-test-1:cap-<uuid>",
#     "status": 200,
#     "ip": "..."
#   },
#   {
#     "id": "...",
#     "ts": "2026-08-23T12:35:00Z",
#     "actor": "user@example.com",
#     "method": "GET",
#     "path": "/servers/capture-test-1:capture-file",
#     "target": "capture-test-1:cap-<uuid>",
#     "status": 200,
#     "ip": "..."
#   }
# ]
```

**PASS Criteria (SC-003, FR-006)**:
- Audit log contains one entry per operation: start, stop, and download
- Each entry includes exactly the `audit.Event` fields: ID, timestamp, actor (user identity),
  method, path, target (`{server}` or `{server}:{captureId}`), status, IP — there is no
  separate details/payload field to inspect
- All three operations appear in correct chronological order

---

## Scenario: US4.1 — Retention Expiry and Auto-Deletion (SC-004)

**Objective**: Verify captures auto-expire after the retention window and become inaccessible.

**Retention default (FR-007, not open for re-discovery)**: the cluster default retention window
is **24 hours (86400 seconds)**, not 7 days. This scenario uses a short, explicit
`ttlSecondsAfterFinished` override for test speed rather than waiting out the real default.

### Setup: Create a Capture with Short TTL (For Testing)

```bash
# Create a capture with a very short TTL (e.g., 60 seconds, for testing)
# Note: The exact TTL setting depends on implementation; this assumes a configurable field
# or a test fixture with shortened defaults

cat <<'EOF' | kubectl apply -f -
apiVersion: gameplane.local/v1alpha1
kind: NetworkCapture
metadata:
  name: cap-test-short-ttl
  namespace: default
spec:
  serverRef:
    name: capture-test-1
  filter: "udp port 25565"
  maxDurationSeconds: 30
  maxSizeBytes: 104857600
  ttlSecondsAfterFinished: 60  # 1 minute TTL for testing
status:
  phase: "Completed"
  completionTime: "2026-08-23T12:34:56Z"
  bytesWritten: 1024
EOF

# Record the current time
CAPTURE_TIME=$(date -u +%s)
echo "Capture created at epoch: $CAPTURE_TIME"
```

### Wait for Expiry (CI Only - Use Shortened TTL)

In CI, this can be sped up by:
1. Setting a test cluster config with very short default TTL (e.g., 10 seconds)
2. Waiting for the operator's reconciliation loop to detect expiry
3. Verifying the capture is deleted

```bash
# Wait for TTL to expire (in CI, use short TTL; for this example, 60 seconds)
echo "Waiting 65 seconds for capture to expire..."
sleep 65

# Check if NetworkCapture CRD still exists
kubectl get networkcapture cap-test-short-ttl 2>&1
# Expected (after expiry): error: networkcaptures.gameplane.local "cap-test-short-ttl" not found

# Alternative: check operator logs for cleanup event
kubectl logs -n gameplane-system -l app=gameplane-operator | grep -i "expired\|cleanup\|cap-test-short-ttl"
# Expected: Log line indicating the capture was garbage collected
```

### Verify Download Fails After Expiry (SC-004)

```bash
# Attempt to download an expired capture
# (Before expiry, this succeeds; after expiry, it should fail)

curl -X GET "http://localhost:8000/servers/capture-test-1:capture-file?id=cap-test-short-ttl" \
  -H "Authorization: Bearer <admin-token>"

# Expected response: HTTP 404, plain-text body (httperr.Write — no JSON envelope):
#   capture 'cap-test-short-ttl' not found or has expired
```

**PASS Criteria (SC-004)**:
- Capture expires after TTL window (verified by CRD absence)
- Download attempt returns 404 after expiry
- Response body clearly indicates expiry/not-found (plain text, not JSON)
- Operator logs show cleanup event

---

## Scenario: US5.1 — Handle Server Restart During Capture (Edge Case)

**Objective**: Verify the system recovers gracefully if a pod restarts while a capture is running.

### Setup: Start Capture and Trigger Pod Restart

```bash
# Initiate a capture with longer duration (to give time for restart)
CAPTURE_ID=$(curl -s -X POST http://localhost:8000/servers/capture-test-1:capture-start \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "filter": "udp port 25565",
    "maxDurationSeconds": 300,
    "maxSizeBytes": 104857600
  }' | jq -r '.captureId')

echo "Capture ID: $CAPTURE_ID"

# Wait a few seconds for capture to be in Running state
sleep 3

# Verify capture is running
kubectl get networkcapture "cap-<uuid>" -o jsonpath='{.status.phase}'
# Expected: "Running"

# Trigger a pod restart (simulates game update, node drain, etc.)
kubectl delete pod -l app.kubernetes.io/instance=capture-test-1
# Watch for pod restart
kubectl get pod -l app.kubernetes.io/instance=capture-test-1 -w
# Expected: Old pod terminates; new pod starts
```

### Verify Clean Recovery (FR-010)

```bash
# Check that the NetworkCapture CRD reflects the restart
# Implementation option 1: NetworkCapture is marked Failed with reason "pod restarted"
# Implementation option 2: NetworkCapture status is updated to show termination

kubectl get networkcapture "cap-<uuid>" -o jsonpath='{.status.phase}'
# Expected: "Failed" (if marked as failed) or "Completed" (if gracefully terminated)

kubectl get networkcapture "cap-<uuid>" -o jsonpath='{.status.message}'
# Expected: Message like "pod restarted" or "capture terminated due to pod restart"

# Verify partial capture file is cleaned up (not left orphaned on node)
# (Verification would require kubectl exec into a node, implementation detail)

# Verify new pod is healthy and playable
kubectl get pod -l app.kubernetes.io/instance=capture-test-1 -o jsonpath='{.status.phase}'
# Expected: "Running"
```

**PASS Criteria (US5.1, FR-010)**:
- Pod restarts without data loss
- NetworkCapture status is updated (not left in Running indefinitely)
- Game pod reaches Running state after restart
- No orphaned capture files on the node
- New captures can be started on the restarted pod

---

## Scenario: US5.2 — Prevent Concurrent Captures (FR-012)

**Objective**: Verify only one capture can run at a time per GameServer.

### Attempt Two Concurrent Captures

```bash
# Start first capture
CAPTURE_1=$(curl -s -X POST http://localhost:8000/servers/capture-test-1:capture-start \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "filter": "udp port 25565",
    "maxDurationSeconds": 120,
    "maxSizeBytes": 104857600
  }' | jq -r '.captureId')

echo "Capture 1: $CAPTURE_1"

# Immediately attempt a second capture (before first is stopped)
curl -s -X POST http://localhost:8000/servers/capture-test-1:capture-start \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "filter": "tcp port 22",
    "maxDurationSeconds": 120,
    "maxSizeBytes": 104857600
  }' \
  -w "\nHTTP Status: %{http_code}\n"

# Expected response: HTTP 409, plain-text body (httperr.Write — no JSON envelope):
#   capture already in progress on this server
# HTTP Status: 409
```

**PASS Criteria (SC-006, FR-012)**:
- First capture succeeds (HTTP 202)
- Second, concurrent capture request is rejected (HTTP 409), not left in an undefined state
- Both requests are recorded in the audit log (SC-003)

---

## Scenario: US5.3 — Invalid Filter Expression Rejection (FR-003)

**Objective**: Verify malformed BPF filter expressions are rejected before capture starts.

### Attempt Invalid Filters

```bash
# Test 1: Completely invalid syntax
curl -X POST http://localhost:8000/servers/capture-test-1:capture-start \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "filter": "this is not a valid pcap filter expression at all!!!",
    "maxDurationSeconds": 60,
    "maxSizeBytes": 104857600
  }' \
  -w "\nHTTP Status: %{http_code}\n"

# Expected: HTTP 400 Bad Request, plain-text body (httperr.Write — no JSON envelope):
#   invalid filter: <syntax error detail>

# Test 2: Valid syntax but incomplete (tcp without condition)
curl -X POST http://localhost:8000/servers/capture-test-1:capture-start \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "filter": "tcp and",
    "maxDurationSeconds": 60,
    "maxSizeBytes": 104857600
  }' \
  -w "\nHTTP Status: %{http_code}\n"

# Expected: HTTP 400 Bad Request with specific error message
```

**PASS Criteria (FR-003)**:
- Invalid filters are rejected before NetworkCapture CRD is created
- HTTP 400 returned (not 202 / Pending)
- Error message describes the syntax problem
- No orphaned NetworkCapture CRDs are created

---

## Scenario: Negative Check — Non-Opted-In Server Has No Capture Container (SC-007)

**Objective**: Verify that a GameServer without capture enabled carries no capture container.
Per the Decision on pre-provisioning (see the note above US2.1), this is **not** a
byte-identical-to-pre-feature check: the capture `emptyDir` volume is added unconditionally to
every game pod's template, opted in or not, so a non-opted-in pod still differs from a
pre-feature pod by that one empty volume. What must be absent is the capture **container** — in
`spec.ephemeralContainers` for the ephemeral-container mechanism this feature uses.

### Create Two Identical Servers, One Without Capture

```bash
# Server A: No capture (intentionally omitted)
cat <<'EOF' | kubectl apply -f -
apiVersion: gameplane.local/v1alpha1
kind: GameServer
metadata:
  name: no-capture-a
  namespace: default
spec:
  templateRef:
    name: minecraft-java
  networking:
    expose: LoadBalancer
EOF

# Server B: Capture disabled explicitly
cat <<'EOF' | kubectl apply -f -
apiVersion: gameplane.local/v1alpha1
kind: GameServer
metadata:
  name: no-capture-b
  namespace: default
spec:
  templateRef:
    name: minecraft-java
  networking:
    expose: LoadBalancer
  capture:
    enabled: false
EOF

# Wait for both pods to be Running
kubectl get pod -l app.kubernetes.io/instance=no-capture-a -w
kubectl get pod -l app.kubernetes.io/instance=no-capture-b -w
```

### Compare Pod Specs

```bash
# Extract and compare pod specs (should be identical)
POD_A=$(kubectl get pod -l app.kubernetes.io/instance=no-capture-a -o jsonpath='{.items[0].metadata.name}')
POD_B=$(kubectl get pod -l app.kubernetes.io/instance=no-capture-b -o jsonpath='{.items[0].metadata.name}')

kubectl get pod "$POD_A" -o yaml > /tmp/no-capture-a.yaml
kubectl get pod "$POD_B" -o yaml > /tmp/no-capture-b.yaml

# Diff (should be identical between the two non-opted-in servers except for pod name, UID, etc.
# — omitting the field vs. explicitly setting enabled:false must not produce different pods)
diff -u /tmp/no-capture-a.yaml /tmp/no-capture-b.yaml | grep -E "^\+" | grep -v "^\+\+\+" | head -20
# Expected: Only differences are metadata fields (name, uid, resourceVersion)
# NO differences in spec.containers, spec.ephemeralContainers, spec.volumes, etc.
# (both pods carry the same pre-provisioned capture emptyDir volume, so it produces no diff here)

# Explicit check: verify NO ephemeralContainers in either pod (the container is what must be
# absent — NOT the pre-provisioned volume, which is present on every game pod unconditionally)
kubectl get pod "$POD_A" -o jsonpath='{.spec.ephemeralContainers}'
# Expected: null or empty array

kubectl get pod "$POD_B" -o jsonpath='{.spec.ephemeralContainers}'
# Expected: null or empty array
```

**PASS Criteria (SC-007)**:
- Pod specs for non-opted-in servers are identical to each other (except metadata like UID, name)
- No capture ephemeralContainers in either pod
- Both pods DO carry the pre-provisioned capture emptyDir volume in `spec.volumes` — this is
  expected and correct (see the Decision note above US2.1), not a regression

---

## What PASS Looks Like: Summary Checklist

| User Story / SC | Observable Proof | Validation Scenario |
|---|---|---|
| **US1 / SC-001** | Captured file is valid PCAPNG, readable by Wireshark/tshark | US1.3: Download file and verify with tshark |
| **SC-002** | Capture ON vs OFF shows no perceptible packet loss or latency increase | SC-002 Scenario: performance comparison |
| **SC-003** | 100% of capture operations (start/stop/download/delete) recorded in the audit log with operator identity, server name, capture ID, timestamp, and result | US3.2: Query audit API and verify entries |
| **US1 / SC-008** | 100% of packets in file match filter; 0% non-matching | US1.3: tshark filter compliance test |
| **US2 / SC-007** | Opt-out pod has no capture container (it may carry the pre-provisioned, empty capture volume — see Decision); opt-in adds only the sidecar container | US2.1 + Negative Check: kubectl diff shows container-only addition |
| **US3 / SC-005** | Non-admin POST 403 (plain text); admin POST 202 | US3.1: Admin-only access test |
| **SC-006** | Concurrent capture requests on the same server are serialized or rejected with a clear error, not left undefined | US5.2: Attempt two captures, verify second is rejected |
| **US4 / SC-004** | Expired capture 404s after retention window | US4.1: Wait for TTL, attempt download |
| **US5.1 / FR-010** | Pod restart does not crash system; pod recovers | US5.1: Delete pod mid-capture, verify recovery |
| **US5.2 / FR-012** | Concurrent capture request rejected (HTTP 409) | US5.2: Attempt two captures, verify second fails |
| **US5.3 / FR-003** | Invalid filter rejected HTTP 400 before CRD created | US5.3: POST invalid filter, verify 400 response |

---

## Troubleshooting

### Symptom: Capture sidecar never enters Running state

**Likely Causes**:
1. **Image not found**: Capture sidecar image not pushed to registry or invalid image URI
   - **Check**: `kubectl describe pod <pod-name> | grep -A 5 "Events:"`
   - **Fix**: Build and push capture sidecar image; verify image URI in operator code

2. **Raw-socket access not effective**: per the accepted design (see `research.md`/`data-model.md`
   D-SETCAP), the capture container does **not** rely on `capabilities.add: ["NET_RAW"]` alone under
   a non-root `runAsUser` — Kubernetes does not set ambient capabilities, so that combination by
   itself grants nothing (the effective set is cleared on `execve`). Raw-socket access instead comes
   from a FILE capability (`setcap cap_net_raw+ep`) baked onto the capture binary at image-build
   time, which only takes effect if the container also sets `allowPrivilegeEscalation: true` — file
   capabilities are ignored under `no_new_privs`, which is what `allowPrivilegeEscalation: false`
   enforces. `runAsNonRoot` is preserved; `allowPrivilegeEscalation` is the deliberate trade-off
   given up for it. The container's `securityContext.capabilities` also lists `add: ["NET_RAW"]`
   alongside `drop: ["ALL"]` — not because `add` grants anything, but because `drop: ["ALL"]` on its
   own would empty the process's *bounding* set too, and the kernel refuses to grant the setcap'd
   file capability at `execve` if NET_RAW isn't in the bounding set. A "restricted" PodSecurity
   profile forbids `allowPrivilegeEscalation: true`, so the games namespace needs a documented
   admission exception for this container (see `docs/security.md`).
   - **Check**: `kubectl describe pod <pod-name> | grep -i "securitycontext\|capability\|allowPrivilegeEscalation"`
   - **Fix**: Exempt the games namespace from the restricted admission's
     `allowPrivilegeEscalation` check for this container, and confirm `cap_net_raw+ep` is actually
     set on the binary inside the built image — **this is an unverified build risk**: file
     capabilities are extended attributes (xattrs) and are not guaranteed to survive a multi-stage
     `COPY` into a distroless/scratch final stage (this repo has already hit COPY-time file-mode
     loss once, in the game-image entrypoint work). Whether the capability survives the image build
     must be proven in CI before this approach can be trusted; nothing here asserts that it does.

3. **Insufficient resources**: Node does not have memory/CPU for sidecar
   - **Check**: `kubectl top nodes` and `kubectl describe node`
   - **Fix**: Free resources or use a larger cluster

### Symptom: Capture file is empty or corrupted

**Likely Causes**:
1. **Filter too restrictive**: No packets matched the filter
   - **Check**: Verify filter syntax with tshark: `tshark -d udp -f "udp port 25565" < /dev/null` (should parse without error)
   - **Fix**: Adjust filter to match actual traffic (e.g., broaden to `udp` without port restriction)

2. **Traffic did not flow during capture**: Server was idle or no clients connected
   - **Check**: Verify game client was actually connected and exchanging packets
   - **Fix**: Ensure real traffic is flowing during capture window (e.g., real player join, not just pod creation)

3. **Snaplen too small**: Packets are truncated, file appears corrupted to parser
   - **Check**: `tcpdump -r <file>` and look for truncation warnings
   - **Fix**: Increase snaplen (default should be 65535 for full packets)

### Symptom: Filter validation passes but sidecar silently drops packets

**Likely Causes**:
1. **API and sidecar filter mismatch**: API validates with go-pcap/filter but sidecar uses different filter engine
   - **Check**: Verify both use same filter library (research.md specifies go-pcap/filter for API validation)
   - **Fix**: Ensure sidecar also uses go-pcap/filter or equivalent BPF compiler

2. **Filter expression differences between pcap-filter(7) and implementation**:
   - **Check**: Test filter with multiple packet analyzers: tshark, tcpdump, Wireshark
   - **Fix**: Test filter compliance in unit tests before implementing sidecar

### Symptom: Download endpoint returns 413 Payload Too Large

**Likely Causes**:
1. **Capture file exceeds API proxy size limit (64 MiB)**:
   - **Check**: NetworkCapture status shows bytesWritten > 64MB
   - **Fix**: Large captures should use sidecar endpoint (`:9091`) instead of agent proxy; ensure API correctly routes large files

### Symptom: Non-admin user can start captures (SC-005 failure)

**Likely Causes**:
1. **RBAC middleware not applied to capture endpoints**:
   - **Check**: Inspect API handler registration (api/internal/handlers/capture.go or similar)
   - **Fix**: Wrap capture endpoints with `rbac.Middleware` with admin-only check

2. **Token validation bypassed**:
   - **Check**: Verify `Authorization` header is being parsed and validated
   - **Fix**: Ensure auth middleware is registered before capture handlers

---

## Running These Scenarios on CI vs. Live Cluster

### On GitHub Actions CI

Nothing in this feature has been built, run, or tested yet. Once implemented, the intent is for
these scenarios to run in an e2e job on a kind cluster (per Constitution Principle VI and CLAUDE.md
rule 8), with capture syscalls (AF_PACKET) and game-probe traffic fixtures available there — but
that automation does not exist today, and no duration estimate can be claimed until it does.

### On Live kubelab Cluster

For maintainer validation:
```bash
# Port-forward API
kubectl --kubeconfig ~/kubelab.yaml port-forward -n gameplane-system svc/gameplane-api 8000:8000 &

# Export auth token (if OIDC is available)
export AUTH_TOKEN="<bearer-token>"

# Run each scenario manually, following the commands above
curl -X POST http://localhost:8000/servers/capture-test-1:capture-start \
  -H "Authorization: Bearer $AUTH_TOKEN" ...
```

---

## References

- **Specification**: `specs/003-network-capture-sidecar/spec.md` (User Stories US1–US5, Success Criteria SC-001–SC-008, Requirements FR-001–FR-012)
- **Research Document**: `specs/003-network-capture-sidecar/research.md` (Architecture decisions, technology choices, open risks)
- **CRD Types**: `operator/api/v1alpha1/networkcapture_types.go` (NetworkCapture spec and status fields)
- **API Handlers**: `api/internal/handlers/capture.go` (REST endpoints for capture lifecycle)
- **E2E Test Bucket**: `test/e2e/buckets.sh` (bucket placement; capture tests may go in operator or bot-fast bucket)
- **CLAUDE.md**: `CLAUDE.md` (project rules: CI-only testing, Principle VI, no local execution)

---

**Document Revision**: Phase 1 Quickstart  
**Last Updated**: 2026-08-23  
**Status**: Ready for implementation validation
