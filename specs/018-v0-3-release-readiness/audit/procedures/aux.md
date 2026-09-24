# Procedures: AUX

Shared conventions: [conventions.md](conventions.md).

### sentinel-minecraft-status-ping

**Preconditions:** sentinel deployed; a dormant GameServer with Minecraft; gameproto's Minecraft Classifier registered to handle join/status detection.

**Resources created:** `audit018-test-sentinel-mc` (GameServer copy with idle timeout enabled, sleeping); `audit018-sentinel-test` (temporary curl pod).

**Steps:**
1. Copy an existing Minecraft GameServer to `audit018-test-sentinel-mc` with `spec.idle.enabled=true` and initial idleAfterSeconds=5.
2. Wait for the GameServer to reach Idle phase (sentinel pod running).
3. From a test pod, send a Minecraft server-list ping (status) handshake to the sentinel's advertised port:
   ```bash
   kubectl run audit018-sentinel-test -n gameplane-games \
     --image=curlimages/curl:latest --rm -i --restart=Never -- \
     sh -c 'echo -ne "\x00\x00\x00\x00" | nc gameplane-games 25565' || true
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

**Resources created:** `audit018-test-sentinel-mc-join` (GameServer copy, idle); temporary netcat listener.

**Steps:**
1. Create `audit018-test-sentinel-mc-join` (idle, like above) and wait for Idle phase.
2. From a test pod, send a Minecraft join handshake to trigger a wake. Use a minimal Handshake packet (wakeProtocol=minecraft):
   ```bash
   kubectl run audit018-sentinel-join -n gameplane-games \
     --image=curlimages/curl:latest --rm -i --restart=Never -- \
     timeout 30 bash -c 'echo -ne "\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00" | nc gameplane-games 25565' || true
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

**Preconditions:** sentinel deployed; a dormant GameServer with Terraria or generic UDP wakeProtocol.

**Resources created:** `audit018-test-sentinel-terraria` (GameServer, idle).

**Steps:**
1. Create `audit018-test-sentinel-terraria` with idle enabled, wakeProtocol=generic (UDP) and wait for Idle phase.
2. From a test pod, send a single UDP packet to the sentinel's advertised UDP port. Sentinel's UDP heuristic requires N packets from one source within the window to trigger (default N=3, window=10s).
3. Send 3 UDP packets from the same source within 10 seconds:
   ```bash
   kubectl run audit018-sentinel-udp -n gameplane-games \
     --image=curlimages/curl:latest --rm -i --restart=Never -- \
     sh -c 'for i in 1 2 3; do echo "probe" | nc -u gameplane-games 19133; sleep 2; done' || true
   ```
4. Verify the wake annotation is patched: `kubectl get gs audit018-test-sentinel-terraria -o jsonpath='{.metadata.annotations.gameplane\.local/idle-wake-requested}'`.
5. Login cost: 0.

**Expected:** After 3 packets in 10s, sentinel triggers a wake and patches the annotation.

**Cleanup:** `kubectl delete gs audit018-test-sentinel-terraria -n gameplane-games`.

**Automatable?:** yes, api-agent bucket.

---

### capture-sidecar-start-stop

**Preconditions:** capture.enabled=true in values.yaml (line 527); a running GameServer with capture-sidecar attached.

**Resources created:** `audit018-capture-test` (GameServer with capture enabled).

**Steps:**
1. Enable capture in the Helm values: `helm upgrade gameplane charts/gameplane -n gameplane-system --set capture.enabled=true`.
2. Create `audit018-capture-test` GameServer and wait for it to reach Running.
3. Verify the capture-sidecar is injected: `kubectl get pod -n gameplane-games -l gameplane.io/game-server=audit018-capture-test -o jsonpath='{.items[0].spec.containers[*].name}' | grep capture`.
4. Start a capture via the API: `POST $GP/api/v1/captures/audit018-capture-test:start` with filter="" (match all) and maxDurationSeconds=10.
5. Verify the capture status: `GET $GP/api/v1/captures/audit018-capture-test:status` returns running.
6. Wait 5 seconds (packet capture in flight).
7. Stop the capture: `POST $GP/api/v1/captures/audit018-capture-test:stop`.
8. Verify the capture is complete: `GET $GP/api/v1/captures/audit018-capture-test:status` returns completed.
9. Download the PCAPNG file: `GET $GP/api/v1/captures/audit018-capture-test/file` returns a valid PCAPNG file (magic bytes: 0x0A 0x0D 0x0D 0x0A).
10. Login cost: 1 (one API session).

**Expected:** Capture starts, captures traffic, stops, and is downloadable as a valid PCAPNG file.

**Cleanup:** `kubectl delete gs audit018-capture-test -n gameplane-games`.

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

**Resources created:** none on kubelab (no external syslog collector); `audit018-syslog-listener` (local netcat listener as alternative).

**Steps:**
1. Note: kubelab has no external syslog collector deployed. The bridge is a blocked candidate for full end-to-end testing.
2. Alternative on kubelab: Deploy a netcat listener pod in `gameplane-system` that mimics a syslog server:
   ```bash
   kubectl run audit018-syslog-listener -n gameplane-system \
     --image=busybox:latest -i -d -- nc -l -p 514 -q 1
   ```
3. Reconfigure the bridge to point at the listener: `helm upgrade gameplane charts/gameplane -n gameplane-system --set api.audit.webhook.syslogBridge.enabled=true --set api.audit.webhook.syslogBridge.syslog.addr=audit018-syslog-listener:514`.
4. Create an audit event by logging in or making an API call.
5. Tail the listener logs: `kubectl logs -n gameplane-system audit018-syslog-listener` and verify RFC 5424 formatted syslog records arrive.
6. Login cost: 1 (one login triggers an audit event).

**Expected:** Syslog records arrive at the listener in RFC 5424 format.

**Cleanup:** `kubectl delete pod audit018-syslog-listener -n gameplane-system`.

**Automatable?:** yes, api-auth bucket (login triggers audit event).

---

### telemetry-receiver-opt-in

**Preconditions:** api.telemetry.receiver.enabled=true (values.yaml line 235); telemetry endpoint configured.

**Resources created:** none (receiver is already deployed).

**Steps:**
1. Verify the telemetry-receiver deployment exists: `kubectl get deploy gameplane-telemetry-receiver -n gameplane-system`.
2. Log in to the dashboard as admin.
3. Navigate to **Admin Settings → Telemetry**.
4. Toggle **Send anonymous usage metrics** to ON.
5. Wait for the API to send a report (or trigger manually if that option exists, typically once per day).
6. Tail the receiver logs: `kubectl logs -n gameplane-system deploy/gameplane-telemetry-receiver` and verify a JSON report with `version`, `servers`, and `templates` fields is logged.
7. Check Prometheus metrics on the receiver: `kubectl port-forward -n gameplane-system svc/gameplane-telemetry-receiver 8080:8080` and curl `localhost:8080/metrics`, verify `gameplane_telemetry_reports_total` counter incremented.
8. Login cost: 1 (one admin session).

**Expected:** Telemetry report is received, logged, and aggregated into Prometheus metrics.

**Cleanup:** none.

**Automatable?:** yes, api-auth bucket.

---

### mcp-server-read-only-check

**Preconditions:** mcpServer.enabled=true (values.yaml line 400); MCP client available (Claude Code, Claude Desktop, or standalone MCP client).

**Resources created:** none (server already deployed).

**Steps:**
1. Verify the mcp-server deployment exists: `kubectl get deploy gameplane-mcp-server -n gameplane-system`.
2. Connect an MCP client to the server via `kubectl exec`:
   ```bash
   kubectl exec -i deploy/gameplane-mcp-server -n gameplane-system -- /mcp-server serve
   ```
3. From the MCP client, invoke a read-only tool (e.g., `list_gameplane_resources`) to list GameServers in the cluster.
4. Verify the response includes cluster state (names, namespaces, statuses).
5. Attempt a mutating operation (if the client supports it) — the server must refuse it (e.g., a `create_gameplane_resource` call must not exist in the tool registry).
6. Verify RBAC: the mcp-server ServiceAccount has only `get`/`list`/`watch` on Gameplane CRDs and Pods: `kubectl get rolebinding gameplane-mcp-server -n gameplane-system -o yaml | grep -E "verbs|resources"`.
7. Login cost: 0.

**Expected:** MCP client reads cluster state successfully; server is read-only; no mutating tools exist.

**Cleanup:** none.

**Automatable?:** yes, api-agent bucket (read-only test, no mutations).

