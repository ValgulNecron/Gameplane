# sentinel — Specification

**Status:** beta (v0.2.0-beta.8)  
**Module / command:** `github.com/ValgulNecron/gameplane/sentinel`  
**Dependencies:** `controller-runtime` libs, Kubernetes client-go, gameproto

## Purpose

Wake-on-connect daemon that runs as a replacement pod for dormant game servers. The sentinel listens on the server's advertised ports, classifies inbound connection attempts using gameproto to distinguish genuine player joins from server-list status pings, and decides whether to wake the server or answer without waking. For join attempts, the sentinel holds the connection open, patches a Kubernetes annotation to signal the operator, waits for the real game pod to come up, replays the handshake bytes to it, and proxies the remainder of the connection bidirectionally.

## Responsibilities

1. Bind listeners on configured TCP and UDP ports (replacing the sleeping game pod).
2. Classify TCP connections using a registry-based Classifier (Minecraft, Terraria, or custom protocols registered in gameproto), or a generic protocol-agnostic heuristic (UDP) for protocols without a parser.
3. For Status/ping classifications (if the protocol supports them), send a protocol-native "server is asleep" reply **without** waking the server.
4. For Join classifications, request a wake via a Kubernetes annotation patch, then hold the connection open.
5. Poll for the real game pod to come up (by repeatedly attempting to dial the game-direct Service), replay the consumed handshake bytes to it, and proxy the remaining traffic bidirectionally.
6. Bound concurrent TCP handlers with a semaphore to prevent resource exhaustion under port scans.
7. Track UDP sources by IP and apply a "N packets from one source within a window" heuristic to detect real players vs. scanners.
8. On deadline expiry (server taking too long to wake), send a protocol-native bounce message (if one exists) and close.

## Non-goals / boundaries

- Does **not** run the game server itself — the operator/controller does that.
- Does **not** authenticate players or check credentials — join/status classification is protocol-only.
- Does **not** persist wake requests — patches the GameServer annotation; the operator's reconciler is authoritative.
- Does **not** validate game-specific rules or enforce RBAC — those are handled by the game pod and/or API when it's up.
- Does **not** implement protocol parsing itself — uses the gameproto registry of Classifier implementations for protocol-specific dispatch. Protocols without a Classifier fall back to the generic heuristic.
- Does **not** filter or modify traffic — handshake replay and bidirectional proxying preserve the exact byte stream.

## Directory & package layout

```
sentinel/
├── main.go          # Entry point, config parsing, listeners, handlers, upstream polling, proxying
├── main_test.go     # Config parsing, ports parsing, UDP heuristic, waker, TCP/UDP handlers, proxying, bidirectional copy
├── Dockerfile       # Image build (sets Version at compile time)
├── go.mod           # Dependencies: k8s.io/client-go, k8s.io/apimachinery, github.com/ValgulNecron/gameplane/gameproto
├── go.sum           # Locked versions
└── .testcoverage.yml # 70% coverage gate
```

Single executable module; no subdirectories or packages.

## External Interface / Configuration

### Environment Variables

- **`GAMESERVER_NAME`** (required): Name of the GameServer CRD this sentinel is responsible for (e.g., `my-server`).
- **`GAMESERVER_NAMESPACE`** (required): Namespace of the GameServer (e.g., `games`).
- **`PORTS_CONFIG`** (required): Comma-separated list of ports to listen on, format: `port:protocol:wakeProtocol,...`
  - Example: `25565:TCP:minecraft,19133:UDP:generic,8080:TCP:none`
  - `protocol`: `TCP` or `UDP`.
  - `wakeProtocol`: Any protocol registered in the gameproto registry (e.g., `minecraft`, `terraria`), or the special values `generic` or `none`.
    - Registered protocols (e.g., `minecraft`, `terraria`): Use the corresponding gameproto Classifier to parse handshakes and determine Join vs. Status classifications.
    - `generic`: No protocol-specific parsing; treat every accepted TCP connection as a join; for UDP, use the packet-counting heuristic.
    - `none`: Operator still advertises the port (container spec, Service), but the sentinel doesn't listen on it (port scan sink).
    - **Startup validation:** At startup, all configured `wakeProtocol` values (except `generic` and `none`) are validated against the gameproto registry. Unknown protocol names cause a fatal startup error listing available registered protocols.

- **`WAKE_DEADLINE`** (optional, default `25s`): How long to hold a join connection waiting for the game pod to come up. Must be less than the game client's own timeout (Minecraft ~30s, Terraria ~30s) or the client gives up first.
- **`UDP_PACKET_THRESHOLD`** (optional, default `3`): Number of packets from one source within the window before triggering a wake.
- **`UDP_PACKET_WINDOW`** (optional, default `10s`): Time window for counting UDP packets.
- **`UDP_MAX_SOURCES`** (optional, default `4096`): Maximum number of distinct UDP source IPs to track. Older sources are evicted when this limit is reached.
- **`MAX_CONNECTIONS`** (optional, default `256`): Maximum concurrent TCP connection handlers. Excess connections queue in the OS accept backlog.
- **`WAKE_PATCH_INTERVAL`** (optional, default `2s`): Minimum time between GameServer annotation patches. Bursts of joining clients within this window are coalesced into one apiserver call.

### Kubernetes RBAC

The operator grants this pod's ServiceAccount exactly two permissions on the target GameServer:
- `get gameservers` — read the current object state.
- `patch gameservers` — write to the `gameplane.local/idle-wake-requested` annotation (via JSON merge patch; other annotations survive).

No create, delete, update, or status subresource permissions. The sentinel is read-only except for the one annotation it patches.

### Service Discovery

Once woken, the real game pod is reachable at a stable DNS name:
```
<GAMESERVER_NAME>-game-direct.<GAMESERVER_NAMESPACE>.svc.cluster.local:<port>
```

This is a Kubernetes Service created by the operator pointing to the game pod's port (the same port the sentinel was listening on). The sentinel dials this address with exponential backoff / polling (default interval: 250ms) until it either succeeds or the deadline expires.

## Protocol Dispatch: Registry-Based Architecture

### Unified Registry Dispatcher (`handleRegistryProtocol`)

TCP connections using a registered protocol (Minecraft, Terraria, or custom) are dispatched through a single unified handler:

1. Accept a TCP connection.
2. Look up the configured `wakeProtocol` in the gameproto registry via `gameproto.Lookup(name)`.
3. Call the Classifier's `Classify(br)` method with a `*bufio.Reader` wrapping the connection.
4. Check the result's `Kind` field:
   - **Status:** If `Classifier.SupportsStatusPing()` is true, build a status response (e.g., `{"version":{"name":"Asleep","protocol":0},"players":{"max":0,"online":0,"description":{"text":"Asleep — joining wakes it"}}`) via `Classifier.BuildStatusResponse()`, send it, and close without waking. If `SupportsStatusPing()` is false, fall through to close without responding.
   - **Join:** Build a bounce function that calls `Classifier.BuildDisconnect(reason)` to send a protocol-native disconnect message on timeout (if the protocol supports it). Call `RequestWake` to patch the annotation, then proceed to hold-and-poll (see hold-and-poll path below).
   - **Unknown:** Close without waking or replying.

This unified dispatcher is protocol-agnostic and makes adding a new game protocol a matter of implementing a Classifier and registering it in the gameproto registry, with zero edits to sentinel's dispatch logic.

### Startup Validation

At startup, `parsePortsConfig` validates each configured `wakeProtocol` against the gameproto registry (via `gameproto.Lookup`). Unknown protocol names cause a fatal startup error with a clear message listing available protocols. The sentinel refuses to start if any port's `wakeProtocol` is not registered, preventing misconfiguration from being discovered at connection time.

### Generic Protocol (TCP and UDP)

The `handleGeneric` handler remains **outside the registry** because it performs statistical detection (packets-in-window heuristic for UDP, or unconditional accept for TCP) rather than handshake parsing. Generic is not a Classifier; it is a fallback heuristic for protocols without a wire-protocol parser.

1. Accept a TCP connection.
2. Treat the mere acceptance as a join (no handshake to parse).
3. Call `RequestWake` to patch the annotation and proceed to hold-and-poll.
4. On deadline expiry: close without sending a bounce message (no generic protocol-native disconnect exists).

### Generic (UDP)

1. Bind a UDP listener.
2. For each packet received, extract the remote IP address and timestamp.
3. Feed it to the UDP heuristic (`shouldWake`), which counts packets from that IP within a sliding window.
4. If the heuristic triggers (N packets within the window from one source), call `RequestWake`.
5. Drop the packet itself (no upstream listening yet; forwarding would fail). The UDP client is responsible for re-sending if the server doesn't respond.
6. Suppress re-triggering from the same source for the duration of the window (cooldown), to avoid hammering the apiserver on every incoming packet from a scanner or rapid reconnect.

### Hold-and-Poll Path (Registered Protocols with Join Classification, Generic TCP)

1. After `RequestWake` succeeds (or fails but logged; always proceed to poll), start polling for the upstream.
2. Repeatedly dial `game-direct-address:port` with a fixed interval (250ms by default) and bounded deadline (e.g., 25s).
3. On successful dial:
   - Write the `Consumed` handshake bytes (from the Classifier's `Classify` method) to the upstream, if any.
   - Create two goroutines copying data bidirectionally: upstream→client and client→upstream.
   - Use `io.Copy` on each goroutine but read the client side through the same `*bufio.Reader` the handshake was parsed from (to preserve pipelined bytes).
   - When one side hits EOF, half-close the write side of the other (if supported; `net.TCPConn` does) so any remaining buffered data can drain.
   - Wait for both copies to finish before closing both connections.
4. On deadline expiry before the upstream connects:
   - If the protocol supports a protocol-native bounce message (via `Classifier.BuildDisconnect`), send it (e.g., "Server is waking up, try again in a moment.").
   - Close the connection.

## Per-Expose-Mode Behaviour

The sentinel works with any Kubernetes expose mode because it binds local ports and the operator's Service/NetworkPolicy configuration routes traffic to it:

- **ClusterIP:** Sentinel listens on port N inside the pod; the Service's ClusterIP:N routes traffic to that port inside the pod. Works as above.
- **NodePort:** Sentinel listens on port N inside the pod; the Service exposes it as NodePort M on all nodes. The OS port-forwards external traffic to the sentinel's pod port N. Works as above.
- **LoadBalancer:** Sentinel listens on port N; the Service's LoadBalancer routes traffic to NodePort M and thence to the pod. Works as above.
- **Hostport:** Sentinel listens on port N inside the pod; the kubelet binds the host's port N to the pod's port N. Works as above (the pod sees normal port binding; the kubelet handles the host-level binding).

In all cases, the sentinel binds `0.0.0.0:port` inside the pod (or `[::]` for IPv6). The container runtime and kubelet orchestrate external routing.

## UDP Heuristic Semantics

The `udpHeuristic` type tracks packet timestamps per source IP to detect genuine players (multiple packets) from scanners (single packets). This is necessary because UDP has no connection handshake; the sentinel cannot determine intent from a single packet.

### Threshold and Window

- If a source sends `threshold` (default 3) packets within `window` (default 10s), it's classified as a genuine player and `RequestWake` is triggered once.
- If `threshold` packets arrive over 15s (outside the 10s window), they don't accumulate; the window slides and old packets age out.

### Cooldown

Once a source triggers a wake, a cooldown of `window` duration is applied. Additional packets from that source within the cooldown are counted but don't trigger another `RequestWake` (to suppress re-triggering on rapid reconnects or scanner retries). The cooldown expires after the window period passes.

### Source Tracking Bounds

- The per-source map is bounded at `maxSources` (default 4096) distinct IPs. When the limit is reached, the least-recently-active source is evicted to make room.
- A background `sweepLoop` runs periodically and evicts sources whose last packet is older than 2x the window (well past the point where they could accumulate to the threshold).

Together, these mechanisms prevent an internet-facing UDP port from being weaponized as an infinite-growth amplifier or memory bomb.

### PARTIAL-Depth Semantics

The UDP heuristic is **PARTIAL depth** in e2e testing because it's a heuristic, not a definitive proof of a real join:

- A single packet doesn't wake (correct, defensive).
- N packets from one source wakes (good signal, but not proof — could be a game bot in the test harness, or a real game client retrying).
- No actual gameplay or join confirmation is verified; the test only confirms that the heuristic fired and `RequestWake` was called.

To reach **JOINED depth**, the test would need to establish an actual game connection to the woken server and verify that gameplay works end-to-end. Most e2e tests for UDP are **PARTIAL** (heuristic fires, waker called, annotation patched) rather than **JOINED** (actual player login confirmed in-game).

## Connection Lifecycle

### TCP Join Attempt Flow

```
1. Player connects
   ↓
2. Sentinel accepts connection
   ↓
3. Classify (gameproto or protocol check)
   ├─ Status: answer and close, no wake
   ├─ Unknown: close, no wake
   └─ Join:
       ↓
4. RequestWake (patch annotation, coalesce bursts)
   ↓
5. Poll for upstream (game-direct Service)
   │  [repeated dial with interval, bounded by deadline]
   ├─ Success:
   │   ↓
   │ 6a. Write Consumed bytes to upstream
   │   ↓
   │ 6b. Bidirectional copy (with half-close on EOF)
   │   ↓
   │ 6c. Close both connections
   └─ Deadline:
       ↓
   6d. Send bounce message (if protocol supports it)
       ↓
   6e. Close connection
```

### UDP Packet Flow

```
1. Packet received
   ↓
2. Extract source IP, timestamp
   ↓
3. Feed to heuristic.shouldWake(ip, now)
   │  [tracks timestamps per source, counts within window]
   ├─ Not enough packets: return false
   ├─ Already woken recently (cooldown): return false
   └─ Threshold reached:
       ↓
4. RequestWake (coalesced if already in flight)
   ↓
5. Return true (but packet is dropped; UDP is best-effort)
```

## Key Invariants

1. **Handshake replay is exact.** The gameproto `Consumed` field holds the exact bytes read; the sentinel writes them to upstream verbatim. If gameproto misreported the consumed bytes, the game pod would receive a corrupted stream.

2. **Pipelined bytes are preserved.** The `*bufio.Reader` passed to gameproto is re-used by `proxyBidirectional` to read the client side. Any bytes buffered past the handshake stay in the reader and are forwarded correctly.

3. **Bidirectional proxy waits for both directions.** The sentinel doesn't close the downstream connection until both the upstream→downstream and downstream→upstream copies have reached EOF. If it closed early, data in flight would be dropped.

4. **Half-close semantics.** When one side hits EOF, the sentinel half-closes the write side of the other (if supported). This allows the other side to drain any remaining data before the connection fully closes. Generic protocol (no protocol-native disconnect) doesn't half-close (no-op).

5. **No data duplication or loss.** The complete original stream is: `Consumed + remaining(bufio.Reader) + new-data-from-client`. Both classifiers and proxying preserve this boundary.

6. **Annotation patch coalesces bursts.** Bursts of players reconnecting at once don't hammer the apiserver; RequestWake is a no-op if a patch is already in flight or within `wakePatchInterval` of the last one.

7. **UDP sources are evicted fairly.** The least-recently-active source is evicted (LRU), not a random or oldest source, so brief bursts of scanners don't unfairly starve legitimate players trying to rejoin.

## Dependencies

**Stdlib:**
- `bufio`, `bytes`, `context`, `encoding/json`, `fmt`, `io`, `log`, `net`, `os`, `os/signal`, `strconv`, `strings`, `sync`, `syscall`, `time`.

**Kubernetes:**
- `k8s.io/apimachinery/pkg/apis/meta/v1` — metav1 types (PatchOptions, GetOptions).
- `k8s.io/apimachinery/pkg/runtime/schema` — schema.GroupVersionResource.
- `k8s.io/apimachinery/pkg/types` — types.MergePatchType.
- `k8s.io/client-go/dynamic` — dynamic client for get/patch.
- `k8s.io/client-go/rest` — in-cluster configuration.

**Gameplane:**
- `github.com/ValgulNecron/gameplane/gameproto` — Classifier interface (Classify, SupportsStatusPing, BuildStatusResponse, BuildDisconnect), registry functions (Lookup, ListRegistered) for protocol dispatch.

## Security Considerations

1. **Resource exhaustion (TCP):** The semaphore (`MaxConnections`) bounds handler goroutines, so a port scan can't spawn unbounded goroutines. Excess connections queue in the OS accept backlog, which is kernel-managed and bounded.

2. **Resource exhaustion (UDP):** The `UDPMaxSources` limit and `sweepLoop` prevent the per-source map from growing without bound under a scanner. Sources are evicted and forgotten after 2x the window.

3. **Coalesced wake requests:** The `WakePatchInterval` prevents a burst of reconnecting players or a scanner from hammering the apiserver.

4. **Protocol-native defences:** The sentinel uses gameproto's classifiers (which defend against hostile input: bounded reads, no unbounded allocations) and response builders (which escape JSON, validate sizes).

5. **No authentication bypass:** The sentinel doesn't grant access; it only wakes the server. Authentication, RBAC, and game logic are the real server's job once it's up.

6. **Annotation patch isolation:** The JSON merge patch only touches the `gameplane.local/idle-wake-requested` annotation, so other annotations on the GameServer survive. The RBAC role is read + patch only, not update or delete.

7. **In-cluster only:** The sentinel uses in-cluster config to build the Kubernetes client. It cannot be weaponized from outside the cluster to patch arbitrary resources.

## Testing & Coverage

**Test structure:**

- **Config parsing:** `TestParsePortsConfig`, `TestLoadConfigDefaults`, `TestLoadConfigOverrides`, `TestLoadConfigErrors` verify environment variable parsing and validation.
- **Ports parsing:** `TestWantsListener` confirms `none` protocol disables listening.
- **UDP heuristic:** `TestUDPHeuristicThresholdAndCooldown`, `TestUDPHeuristicWindowExpiry`, `TestUDPHeuristicIndependentSources`, `TestUDPHeuristicEviction`, `TestUDPHeuristicSweep` verify packet counting, source tracking, eviction, and sweep.
- **Upstream polling:** `TestWaitForUpstreamSucceedsImmediately`, `TestWaitForUpstreamPollsUntilAvailable`, `TestWaitForUpstreamDeadlineExpires`, `TestWaitForUpstreamRespectsParentContext` verify polling loop, deadline enforcement, context cancellation.
- **Bidirectional proxy:** `TestProxyBidirectionalWaitsForBothDirections`, `TestProxyBidirectionalStopsOnContextCancel` verify copy goroutines, half-close semantics, both-directions-finish guarantee.
- **Waker (annotation patching):** `TestWakerRequestWakeSetsAnnotation`, `TestWakerRequestWakePreservesOtherAnnotations`, `TestWakerRequestWakeCoalescesBursts` verify merge patch isolation, burst coalescing.
- **Registry dispatch:** `TestHandleRegistryProtocolLookupSuccess`, `TestHandleRegistryProtocolStatusPing`, `TestHandleRegistryProtocolJoinAndWake`, `TestHandleRegistryProtocolUnknown` verify registry lookup, Classifier.Classify invocation, protocol-agnostic join/status/unknown handling.
- **Startup validation:** `TestParsePortsConfigUnknownProtocol`, `TestParsePortsConfigValidRegisteredProtocols` verify that unknown protocol names cause fatal startup errors with clear error messages listing available protocols.
- **Generic handlers:** `TestHandleGenericWakesOnConnect`, `TestHandleGenericClosesSilentlyOnDeadline` verify generic TCP (always join), generic UDP bounce (no message).
- **TCP listener:** `TestRunShutsDownOnContextCancel`, `TestRunReturnsErrorOnListenFailure`, `TestServeTCPBoundsConcurrentHandlers` verify clean shutdown, error reporting, semaphore enforcement.

**Test doubles:**

- **`countingWaker`:** Mock `wakeRequester` that counts how many times `RequestWake` was called; used to verify wake conditions without a real Kubernetes cluster.
- **`newFakeGameServerWaker`:** Constructs a waker backed by a `dynamicfake.FakeDynamicClient` (no real API server), allowing annotation patches to be verified.
- **`tcpPipe`:** Constructs a connected pair of loopback TCP connections (real net.Conn semantics, including CloseWrite for half-close).
- **`encodeMinecraftHandshake`, `encodeTerrariaConnectRequest`:** Hand-built wire encoders for testing classifiers end-to-end without depending on external clients.

**Coverage gate:** 70% (enforced by `.testcoverage.yml`). The gap includes error paths in listener setup (already covered at a higher level by error-reporting tests) and some configuration edge cases.

## Non-goals (What Sentinel Does NOT Do)

- **Does not run the game server.** The operator starts the real pod; sentinel is a placeholder.
- **Does not authenticate players.** Join classification is protocol-based, not credential-based.
- **Does not enforce game-specific rules.** RBAC, player caps, banned players, etc. are the game server's responsibility.
- **Does not modify game traffic.** Handshake replay and proxying are bit-for-bit preserving.
- **Does not persist state across restarts.** Sentinel is stateless; the operator and game pod carry state.
- **Does not implement protocol parsing.** Uses the gameproto Classifier registry for protocol-specific dispatch. Games without a registered Classifier fall back to the generic heuristic.
- **Does not manage DNS or load balancing.** The operator's Service and Kubernetes DNS handle routing.

## References

- **`operator/internal/controller/gameserver_sentinel.go`** — operator controller that manages sentinel deployment, RBAC, configuration via environment variables.
- **`test/e2e/tests/bot_*.go`** — e2e tests that launch real game clients (minecraft-launcher, tshock, etc.) to verify join handshake replay and server wake with registry-based dispatch.
- **`gameproto/classifier.go`** — Classifier interface, ClassificationResult type, and protocol registry; defines the contract that all game protocol parsers implement.
- **`gameproto/registry.go`** — gameproto registry: central lookup structure (Lookup, ListRegistered) mapping protocol names to Classifier implementations.
- **`gameproto/specs.md`** — detailed handshake codec, replay contract, and Classifier interface specification.
- **`CLAUDE.md`** ("Wake-on-connect") — project context and design rationale.
