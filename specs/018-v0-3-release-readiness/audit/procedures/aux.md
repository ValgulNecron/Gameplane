# Procedures: AUX

Shared conventions: [conventions.md](conventions.md).

### sentinel-minecraft-status-ping

**Preconditions:** sentinel deployed; a dormant GameServer with Minecraft; gameproto's Minecraft Classifier registered to handle join/status detection.

**Resources created:** `audit018-test-sentinel-mc` (GameServer copy with idle timeout enabled, sleeping); `audit018-sentinel-test` (temporary busybox pod).

**Steps:**
1. Copy an existing Minecraft GameServer to `audit018-test-sentinel-mc` with `spec.idle.enabled=true`, `spec.idle.wakeOnConnect=true` (required for the operator to create a sentinel at all — see `wakeOnConnectEligible` in `operator/internal/controller/gameserver_sentinel.go`), and `spec.idle.afterMinutes=5` (the CRD's minimum; there is no `idleAfterSeconds` field — `operator/api/v1alpha1/gameserver_types.go`'s `IdleSpec` only has `enabled`, `afterMinutes` (`Minimum=5`), `wakeWindows`, `wakeOnConnect`).
2. Wait for the GameServer to reach Idle phase (sentinel pod running); with `afterMinutes=5` this takes about 5 minutes after the last player leaves, not seconds.
3. From a test pod, send a real Minecraft Handshake packet (protocol version 47, server address "localhost", port 25565, next state 1 = Status — `\x00\x00\x00\x00` is not a valid handshake; gameproto's `classifyMinecraftHandshake` rejects it as Unknown/error, so the sentinel closes without ever sending the expected status JSON) to the Service fronting `audit018-test-sentinel-mc` (the Service shares the GameServer's name — `gameplane-games` is the namespace, not a resolvable hostname):
   ```bash
   kubectl run audit018-sentinel-test -n gameplane-games \
     --image=busybox:1.36 --rm -i --restart=Never -- \
     sh -c 'printf "\x0f\x00\x2f\x09localhost\x63\xdd\x01" | nc -w 3 audit018-test-sentinel-mc 25565' || true
   ```
4. Expected: sentinel responds with Minecraft status JSON including "Asleep" in the description, without waking the server.
5. Verify GameServer remains in Idle phase (no wake annotation created): `kubectl get gs audit018-test-sentinel-mc -o json | grep idle-wake-requested`.
6. Login cost: 0 (no API calls).

**Expected:** Sentinel responds with protocol-correct status message (JSON) without triggering a wake.

**Cleanup:** `kubectl delete gs audit018-test-sentinel-mc -n gameplane-games` and delete test pod.

**Automatable?:** yes, api-agent bucket.

---

### sentinel-minecraft-join-wake

**Preconditions:** sentinel deployed; a dormant GameServer with Minecraft.

**Resources created:** `audit018-test-sentinel-mc-join` (GameServer copy, idle); `audit018-sentinel-join` (temporary busybox probe pod).

**Steps:**
1. Create `audit018-test-sentinel-mc-join` with `spec.idle.enabled=true`, `spec.idle.wakeOnConnect=true`, and `spec.idle.afterMinutes=5` (as in sentinel-minecraft-status-ping — there is no idleAfterSeconds field, and wakeOnConnect is required for a sentinel to be created) and wait for Idle phase (about 5 minutes after the last player leaves).
2. From a test pod, send a real Minecraft Handshake packet with next state 2 = Login (a genuine join) to the Service fronting `audit018-test-sentinel-mc-join`:
   ```bash
   kubectl run audit018-sentinel-join -n gameplane-games \
     --image=busybox:1.36 --rm -i --restart=Never -- \
     timeout 30 sh -c 'printf "\x0f\x00\x2f\x09localhost\x63\xdd\x02" | nc -w 30 audit018-test-sentinel-mc-join 25565' || true
   ```
3. Verify the GameServer's `idle-wake-requested` annotation was patched: `kubectl get gs audit018-test-sentinel-mc-join -o jsonpath='{.metadata.annotations.gameplane\.local/idle-wake-requested}'`.
4. Wait for operator to reconcile and bring up the real game pod.
5. Verify the sentinel's hold-and-poll behavior: a new game pod appears in `gameplane-games` namespace.
6. Login cost: 0.

**Expected:** Sentinel patches the wake annotation; operator reconciles and spins up the real game pod.

**Cleanup:** `kubectl delete gs audit018-test-sentinel-mc-join -n gameplane-games`.

**Automatable?:** yes, api-agent bucket (test the operator's wake flow).

---

### sentinel-terraria-generic-udp

**Preconditions:** sentinel deployed; the `tmodloader` and `ark-survival-evolved` Modules both `Ready` (per kubelab-baseline.md; the `terraria` Module itself is `Failed` there, so a fresh GameServer built from the `terraria` GameTemplate is unlikely to reach Running — use `tmodloader` instead, which sets the same `wakeProtocol: terraria` and is classified by the same `gameproto.TerrariaClassifier`).

**Resources created:** `audit018-test-sentinel-terraria` (GameServer from the `tmodloader` GameTemplate, idle, sleeping — exercises INV-AUX-003, the registered Terraria classifier); `audit018-test-sentinel-udp` (GameServer from the `ark-survival-evolved` GameTemplate, idle, sleeping — its `game` port sets no `wakeProtocol` in `modules/ark-survival-evolved/template.yaml`, so the GameTemplate CRD's schema default of `generic` applies; exercises INV-AUX-004, the generic UDP packet-counting heuristic).

**Steps:**
1. Create `audit018-test-sentinel-terraria`: copy an existing GameServer's YAML (e.g. `mc-fabric`), rename it, set `spec.templateRef.name=tmodloader`, `spec.idle.enabled=true`, `spec.idle.wakeOnConnect=true`, and `spec.idle.afterMinutes=5` (the CRD minimum), then apply. Wait for Running, then Idle phase (sentinel pod running; about 5 minutes after the last player leaves).
2. From a test pod, send a real Terraria `ConnectRequest` message (frame: 2-byte little-endian total length, type byte 0x01, then a 7-bit-length-prefixed version string — see `gameproto/terraria.go`'s `classifyTerrariaConnect`) to the Service fronting `audit018-test-sentinel-terraria` on TCP port 7777 (`modules/tmodloader/template.yaml`'s `game` port):
   ```bash
   kubectl run audit018-sentinel-terraria-join -n gameplane-games \
     --image=busybox:1.36 --rm -i --restart=Never -- \
     timeout 30 sh -c 'printf "\x0f\x00\x01\x0bTerraria194" | nc -w 30 audit018-test-sentinel-terraria 7777' || true
   ```
3. Verify the wake annotation is patched: `kubectl get gs audit018-test-sentinel-terraria -o jsonpath='{.metadata.annotations.gameplane\.local/idle-wake-requested}'`.
4. Create `audit018-test-sentinel-udp` the same way as step 1, with `spec.templateRef.name=ark-survival-evolved`, and wait for Idle phase.
5. Confirm the defaulted wake protocol: `kubectl get gametemplate ark-survival-evolved -o jsonpath='{.spec.ports[?(@.name=="game")].wakeProtocol}'` should print `generic`.
6. From a test pod, send 3 UDP packets from the same source within 10 seconds to the Service fronting `audit018-test-sentinel-udp` on port 7777 (sentinel's UDP heuristic requires N packets from one source within the window to trigger; default N=3, window=10s):
   ```bash
   kubectl run audit018-sentinel-udp -n gameplane-games \
     --image=busybox:1.36 --rm -i --restart=Never -- \
     sh -c 'for i in 1 2 3; do echo probe | nc -u -w 1 audit018-test-sentinel-udp 7777; sleep 2; done' || true
   ```
7. Verify the wake annotation is patched: `kubectl get gs audit018-test-sentinel-udp -o jsonpath='{.metadata.annotations.gameplane\.local/idle-wake-requested}'`.
8. Login cost: 0.

**Expected:** The Terraria `ConnectRequest` is classified as Join by gameproto's `TerrariaClassifier` and patches the wake annotation on `audit018-test-sentinel-terraria` (INV-AUX-003); after 3 UDP packets in 10s, the generic packet-counting heuristic patches the wake annotation on `audit018-test-sentinel-udp` (INV-AUX-004).

**Cleanup:** `kubectl delete gs audit018-test-sentinel-terraria audit018-test-sentinel-udp -n gameplane-games`.

**Automatable?:** yes, api-agent bucket (covers both the registered-protocol dispatch and the generic UDP heuristic).

---

### capture-sidecar-start-stop

**Preconditions:** `capture.enabled=true` (values.yaml line 527, set via `helm upgrade --reuse-values`); a running GameServer; an admin or operator session.

**Resources created:** `audit018-capture-test` (GameServer, opted into capture).

**Steps:**
1. Enable capture cluster-wide: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'capture.enabled=true'` and wait for the operator/API rollout (`--reuse-values` is required — a bare `--set` drops every other site-specific value in kubelab-baseline.md; mirrors helm.md's own capture-toggle procedure, which also reverts this at the end).
2. Create `audit018-capture-test` GameServer and wait for it to reach Running.
3. Opt the server into capture: `POST $GP/servers/audit018-capture-test:capture-enable` (session cookie + `X-Gameplane-CSRF` header). This patches `spec.capture.enabled`; without it no sidecar is injected and Step 5's capture-start 400s with "capture is not enabled on this server".
4. Verify the capture-sidecar is injected: `kubectl get pod -n gameplane-games -l gameplane.io/game-server=audit018-capture-test -o jsonpath='{.items[0].spec.containers[*].name}' | grep capture`.
5. Start a capture: `POST $GP/servers/audit018-capture-test:capture-start` with body `{"filter":"","maxDurationSeconds":10,"maxSizeBytes":10485760}` (`maxSizeBytes` is required — `api/internal/handlers/capture.go`'s `captureStartReq`/`captureStart` rejects `<1` with 400). Read `captureId` from the JSON response; every later call needs it.
6. Verify the capture status: `GET $GP/servers/audit018-capture-test:capture?id=<captureId>` returns `"phase":"Running"`.
7. Wait 5 seconds (packet capture in flight).
8. Stop the capture: `POST $GP/servers/audit018-capture-test:capture-stop` with body `{"captureId":"<captureId>"}`.
9. Verify the capture is complete: `GET $GP/servers/audit018-capture-test:capture?id=<captureId>` returns `"phase":"Completed"`.
10. Download the PCAPNG file: `GET $GP/servers/audit018-capture-test:capture-file?id=<captureId>` returns a valid PCAPNG file (magic bytes: 0x0A 0x0D 0x0D 0x0A).
11. Login cost: 1 (one API session).

**Expected:** Capture starts, captures traffic, stops, and is downloadable as a valid PCAPNG file.

**Cleanup:** `kubectl delete gs audit018-capture-test -n gameplane-games` (cascades the owned NetworkCapture CR — `api/internal/kube/capture.go`'s `CreateNetworkCapture` sets an OwnerReference); revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'capture.enabled=false'` and wait for rollout.

**Automatable?:** yes, api-agent bucket.

---

### tunnel-frp-blocked-candidate

**Preconditions:** tunnelImages.frp is configured (values.yaml line 85); external frp server not available on kubelab.

**Resources created:** none (blocked).

**Steps:**
1. Note: Testing frp relay requires an external frp server and a GameServer configured with tunnel.provider=frp.
2. This is a blocked candidate on kubelab: no frp server is deployed in the shared test environment.
3. Alternative on kubelab: test the operator's tunnel-pod injection and config rendering without a running relay (verify the Pod spec includes the tunnel init container and frpc config mount).

**Expected:** Tunnel pod is created with frpc config, but cannot connect (expected failure) since no relay server exists.

**Cleanup:** none.

**Automatable?:** no (external dependency). Alternative: test operator injection logic via envtest (api-agent bucket).

---

### tunnel-tailscale-blocked-candidate

**Preconditions:** tunnelImages.tailscale configured (line 86); external Tailscale account not available on kubelab.

**Resources created:** none (blocked).

**Steps:**
1. Note: Testing Tailscale relay requires a Tailscale account, auth key, and network access to tailscale.com.
2. This is a blocked candidate on kubelab: no Tailscale credentials or network path to Tailscale's servers.
3. Alternative: test the operator's tunnel pod injection and tailscaled config rendering (verify the auth key Secret is mounted and config.json is correctly rendered).

**Expected:** Tunnel pod starts but cannot authenticate to Tailscale (expected).

**Cleanup:** none.

**Automatable?:** no (external dependency). Alternative: test operator injection via envtest.

---

### tunnel-playit-blocked-candidate

**Preconditions:** tunnelImages.playit configured (line 87); external playit.gg account not available on kubelab.

**Resources created:** none (blocked).

**Steps:**
1. Note: Testing playit relay requires a playit.gg account and secret key.
2. This is a blocked candidate on kubelab: no playit.gg account or credentials.
3. Alternative: test the operator's tunnel pod injection and playitd config rendering.

**Expected:** Tunnel pod starts but cannot authenticate to playit.gg.

**Cleanup:** none.

**Automatable?:** no (external dependency). Alternative: test operator injection via envtest.

---

### audit-syslog-bridge-blocked-candidate

**Preconditions:** api.audit.webhook.syslogBridge.enabled=true (values.yaml line 183); api.audit.webhook.syslogBridge.syslog.addr is set to a real syslog collector.

**Resources created:** none on kubelab (no external syslog collector); `audit018-syslog-listener` (local netcat listener pod) and a same-named Service fronting it (a bare Pod has no DNS name, so the bridge — running in a different pod — cannot resolve it by hostname without one).

**Steps:**
1. Note: kubelab has no external syslog collector deployed. The bridge is a blocked candidate for full end-to-end testing.
2. Alternative on kubelab: deploy a netcat listener pod in `gameplane-system` that mimics a syslog server (busybox's `nc` has no `-q`; the listener just runs until the pod is deleted):
   ```bash
   kubectl run audit018-syslog-listener -n gameplane-system \
     --image=busybox:1.36 -i -d -- nc -l -p 514
   ```
3. Expose it so the bridge can reach it by hostname: `kubectl expose pod audit018-syslog-listener -n gameplane-system --port=514 --target-port=514`.
4. Reconfigure the bridge to point at the listener: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.audit.webhook.syslogBridge.enabled=true' --set 'api.audit.webhook.syslogBridge.syslog.addr=audit018-syslog-listener.gameplane-system:514'` and wait for the syslog-bridge pod to start (`--reuse-values` is required — a bare `--set` drops every other site-specific value in kubelab-baseline.md).
5. Create an audit event by logging in or making an API call.
6. Tail the listener logs: `kubectl logs -n gameplane-system audit018-syslog-listener` and verify RFC 5424 formatted syslog records arrive.
7. Login cost: 1 (one login triggers an audit event).

**Expected:** Syslog records arrive at the listener in RFC 5424 format.

**Cleanup:** `kubectl delete pod audit018-syslog-listener -n gameplane-system` and `kubectl delete service audit018-syslog-listener -n gameplane-system`; revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.audit.webhook.syslogBridge.enabled=false'` and wait for the syslog-bridge pod to terminate.

**Automatable?:** yes, api-auth bucket (login triggers audit event).

---

### telemetry-receiver-opt-in

**Preconditions:** an admin session; `api.telemetry.endpoint` left empty (values.yaml: setting it disables the bundled receiver's auto-wiring).

**Resources created:** none (Helm toggle only; see helm.md's telemetry-receiver toggle procedure).

**Steps:**
1. Enable the bundled receiver: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.telemetry.receiver.enabled=true'` and wait for the `gameplane-telemetry-receiver` deployment to start (`api.telemetry.receiver.enabled` is `false` by default — values.yaml line 235 — and is not among kubelab's site-specific overrides in kubelab-baseline.md, so it is not already running; `--reuse-values` is required or this drops every other site-specific value).
2. Verify the telemetry-receiver deployment exists: `kubectl get deploy gameplane-telemetry-receiver -n gameplane-system`.
3. Log in to the dashboard as admin.
4. Navigate to **Admin Settings → Telemetry**.
5. Toggle **Send anonymous usage metrics** to ON (the independent, dashboard-side gate — per values.yaml's comment above the `telemetry:` block, the Helm toggle in step 1 only controls whether the bundled receiver exists; both gates must be on for anything to be sent).
6. Wait for the API to send a report (or trigger manually if that option exists, typically once per day).
7. Tail the receiver logs: `kubectl logs -n gameplane-system deploy/gameplane-telemetry-receiver` and verify a JSON report with `version`, `servers`, and `templates` fields is logged.
8. Check Prometheus metrics on the receiver: `kubectl port-forward -n gameplane-system svc/gameplane-telemetry-receiver 8080:8080` and curl `localhost:8080/metrics`, verify `gameplane_telemetry_reports_total` counter incremented.
9. Login cost: 1 (one admin session).

**Expected:** Telemetry report is received, logged, and aggregated into Prometheus metrics.

**Cleanup:** toggle **Send anonymous usage metrics** back to OFF in the dashboard; revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.telemetry.receiver.enabled=false'` and wait for the receiver pod to terminate.

**Automatable?:** yes, api-auth bucket.

---

### mcp-server-read-only-check

**Preconditions:** an MCP client available (Claude Code, Claude Desktop, or standalone MCP client); `mcpServer.enabled` is `false` by default (values.yaml line 400) and is not among kubelab's site-specific overrides in kubelab-baseline.md, so it must be enabled first (step 1).

**Resources created:** none (Helm toggle only; see helm.md's mcp-server toggle procedure).

**Steps:**
1. Enable the server: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'mcpServer.enabled=true'` and wait for the `gameplane-mcp-server` deployment to start (`--reuse-values` is required or this drops every other site-specific value in kubelab-baseline.md).
2. Verify the mcp-server deployment exists: `kubectl get deploy gameplane-mcp-server -n gameplane-system`.
3. Connect an MCP client to the server via `kubectl exec`:
   ```bash
   kubectl exec -i deploy/gameplane-mcp-server -n gameplane-system -- /mcp-server serve
   ```
4. From the MCP client, invoke a read-only tool (e.g., `list_gameplane_resources`) to list GameServers in the cluster.
5. Verify the response includes cluster state (names, namespaces, statuses).
6. Attempt a mutating operation (if the client supports it) — the server must refuse it (e.g., a `create_gameplane_resource` call must not exist in the tool registry).
7. Verify RBAC: the mcp-server ServiceAccount has only `get`/`list`/`watch` on Gameplane CRDs and Pods, via a cluster-scoped `ClusterRole`/`ClusterRoleBinding` named `gameplane-mcp-server-read` — not a namespaced RoleBinding, and verbs/resources live on the ClusterRole, not on the binding (`charts/gameplane/templates/mcp-server.yaml`): `kubectl get clusterrole gameplane-mcp-server-read -o yaml | grep -E "verbs|resources"`.
8. Login cost: 0.

**Expected:** MCP client reads cluster state successfully; server is read-only; no mutating tools exist.

**Cleanup:** revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'mcpServer.enabled=false'` and wait for the mcp-server pod to terminate.

**Automatable?:** yes, api-agent bucket (read-only test, no mutations).

