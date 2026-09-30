# Procedures: HELM

Shared conventions: [conventions.md](conventions.md). Every live `helm upgrade gameplane charts/gameplane …` below follows its **Chart source for live Helm toggles (FR-018)** rule: once an RC is deployed, `charts/gameplane` means the pulled RC chart, never this checkout's `0.2.0-beta.8` chart.

### oidc-authentication

**Preconditions:** External OIDC IdP available (e.g., Keycloak, Auth0, or similar; kubelab cluster must reach it). Credentials and IdP metadata configured but not yet enabled in Gameplane.

**Resources created:** none.

**Steps:**
1. Login as audit018-admin to the dashboard. (Login cost: 1)
2. Run: `CURRENT_TAG=$(kubectl get deployment gameplane-api -n gameplane-system -o jsonpath='{.spec.template.spec.containers[0].image}' | grep -o '[^:]*$') && helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set api.oidc.enabled=true --set api.oidc.issuer=<issuer> --set api.oidc.clientID=<clientID> --set api.oidc.redirectURL=<redirectURL> --set image.tag=$CURRENT_TAG` and wait for rollout.
3. Visit `$GP/login` in a new incognito browser tab and observe OIDC provider link on login page.
4. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set api.oidc.enabled=false` and wait for API pod restart.

**Expected:** OIDC login button appears on /login when enabled; disappears when disabled. API pod restarts on both upgrade and downgrade.

**Cleanup:** Helm upgrade with api.oidc.enabled=false; verify local login still works.

**Automatable?** no (needs external IdP; alternative: stub test with local mock IdP if available).

---

### audit-webhook-sink

**Preconditions:** HTTP endpoint listening on a known URL accessible from the API pod (e.g., an in-cluster webhook receiver service or external SaaS endpoint).

**Resources created:** none.

**Steps:**
1. Create a simple webhook receiver in the cluster (e.g., a busybox nc listener or HTTP service) on an internal ClusterIP, or use a test endpoint URL if available.
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.audit.webhook.url=http://webhook-receiver.gameplane-system:8080/events'` and wait for API rollout.
3. Perform an auditable action (e.g., login as audit018-viewer). (Login cost: 1)
4. Wait 2 seconds and check the webhook receiver's logs or HTTP request log for an audit event POST.
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.audit.webhook.url='` (empty string).

**Expected:** API sends audit events as POST JSON to the webhook URL when configured; requests stop after revert.

**Cleanup:** Helm upgrade with webhook.url empty; tear down webhook receiver.

**Automatable?** yes (bucket: api-auth; mock webhook receiver can be deployed inline).

---

### syslog-bridge-audit

**Preconditions:** Syslog collector (rsyslog, syslog-ng, or test endpoint) listening on a known host:port, preferably in-cluster or on audit network. Either tcp or udp; tcp is more reliable.

**Resources created:** audit018-syslog-receiver (Pod + Service, the test collector). Also toggles the chart's fixed-name `gameplane-audit-syslog-bridge` Deployment + Service (not audit018-named; it is the release's own bridge component, not a new test object).

**Steps:**
1. Deploy a test syslog receiver in gameplane-system namespace or use an existing one: `kubectl run audit018-syslog-receiver -n gameplane-system --image=nicolaka/netcat --command -- nc -l -u 0.0.0.0 514 &` (runs in background; for TCP, adjust command), then create a matching Service so the DNS name used in the next step resolves — a bare `kubectl run` pod gets no DNS record of its own: `kubectl expose pod audit018-syslog-receiver -n gameplane-system --port=514 --protocol=UDP --name=audit018-syslog-receiver`.
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.audit.webhook.syslogBridge.enabled=true' --set 'api.audit.webhook.syslogBridge.syslog.addr=audit018-syslog-receiver.gameplane-system:514' --set 'api.audit.webhook.syslogBridge.syslog.network=udp'` and wait for syslog-bridge pod to start.
3. Perform an audit event (e.g., login). (Login cost: 1)
4. Wait 2 seconds and check syslog receiver pod logs for RFC 5424 formatted audit event.
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.audit.webhook.syslogBridge.enabled=false'` and wait for syslog-bridge pod termination.
6. Kill the test receiver: `kubectl delete pod audit018-syslog-receiver -n gameplane-system && kubectl delete svc audit018-syslog-receiver -n gameplane-system`.

**Expected:** syslog-bridge Deployment exists when enabled, is removed when disabled. Syslog receiver pod sees RFC 5424 formatted events when bridge is running and a user action triggers audit logging.

**Cleanup:** Helm upgrade with syslogBridge.enabled=false; delete receiver pod and Service.

**Automatable?** yes (bucket: api-auth; mock syslog endpoint can be containerized).

---

### s3-audit-sink

**Preconditions:** S3-compatible endpoint (MinIO, S3, Backblaze, etc.) accessible from API pod with valid credentials and an existing bucket.

**Resources created:** none (S3 bucket must pre-exist; credentials stored in Secret via Helm values).

**Steps:**
1. Create or identify an S3 bucket (e.g., `audit018-gameplane`) and obtain access key and secret key.
2. Create a Secret in gameplane-system: `kubectl create secret generic audit018-s3-creds -n gameplane-system --from-literal=access-key=<key> --from-literal=secret-key=<secret>`.
3. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.audit.s3.endpoint=minio.gameplane-system:9000' --set 'api.audit.s3.bucket=audit018-gameplane' --set 'api.audit.s3.credentialsSecretRef.name=audit018-s3-creds'` and wait for API rollout.
4. Perform an audit event (login). (Login cost: 1)
5. Wait 5 seconds and verify an NDJSON batch file appears in the S3 bucket.
6. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.audit.s3.endpoint=' --set 'api.audit.s3.bucket=' --set 'api.audit.s3.credentialsSecretRef.name='` and wait for API rollout.

**Expected:** S3 bucket receives audit event batches in NDJSON format when S3 endpoint and bucket are configured; uploads stop after revert.

**Cleanup:** Helm upgrade with S3 settings empty; delete Secret; clear S3 bucket.

**Automatable?** no (blocked candidate: needs S3-compatible storage; alternative: mock S3 via MinIO if available on kubelab).

---

### audit-stdout-logging

**Preconditions:** None (audit stdout is always logged; this test toggles mirroring to stdout).

**Resources created:** none.

**Steps:**
1. Get current API pod name: `kubectl get pod -n gameplane-system -l app.kubernetes.io/name=gameplane-api -o jsonpath='{.items[0].metadata.name}'`.
2. Start tailing API logs: `kubectl logs -n gameplane-system <pod> -f &` (background).
3. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.audit.stdout=true'` and wait for API rollout.
4. Perform an audit event (login). (Login cost: 1)
5. Observe a JSON audit line in the API pod's stdout logs within 2 seconds.
6. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.audit.stdout=false'` and wait for rollout.
7. Perform another audit event. (Login cost: 1)
8. Verify no audit JSON lines appear in stdout logs after revert.

**Expected:** Audit events appear as JSON lines in API stdout when audit.stdout=true; absent when false.

**Cleanup:** Helm upgrade with audit.stdout=false; kill log tail.

**Automatable?** yes (bucket: api-auth; pure log observation).

---

### telemetry-collection

**Preconditions:** External telemetry endpoint URL available and accessible from API pod, or telemetry receiver enabled in-cluster (see telemetry-receiver-bundled below).

**Resources created:** none.

**Steps:**
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.telemetry.endpoint=http://telemetry-receiver.gameplane-system:8080/ingest'` (if using bundled receiver, ensure it is also enabled; see separate procedure).
2. Access the dashboard and verify API is healthy.
3. Wait 5 minutes (telemetry is batched daily, so an immediate observation is unlikely; use mock collector with forced send for faster verification, or accept deferred check).
4. Query the external endpoint logs or the bundled receiver's /metrics endpoint to confirm an ingest event was logged.
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.telemetry.endpoint='`.

**Expected:** API sends telemetry batch to the configured endpoint; no requests after revert.

**Cleanup:** Helm upgrade with telemetry.endpoint empty.

**Automatable?** no (blocked candidate: telemetry is sent daily by default and requires external endpoint; alternative: enable bundled receiver and mock ingest endpoint with deterministic triggers in code).

---

### telemetry-receiver-bundled

**Preconditions:** None (receiver is self-contained if api.telemetry.endpoint is not set, auto-wiring applies).

**Resources created:** none (toggles the chart's fixed-name `gameplane-telemetry-receiver` Deployment + Service, not an audit018-named object).

**Steps:**
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.telemetry.receiver.enabled=true'` and wait for receiver pod to start.
2. Verify receiver pod is running: `kubectl get pod -n gameplane-system -l app.kubernetes.io/name=gameplane-telemetry-receiver`.
3. Verify receiver Service is created: `kubectl get svc -n gameplane-system -l app.kubernetes.io/name=gameplane-telemetry-receiver`.
4. Access receiver /metrics endpoint: `kubectl port-forward -n gameplane-system svc/gameplane-telemetry-receiver 8080:8080 &` and curl `http://localhost:8080/metrics` to see Prometheus metrics endpoint.
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.telemetry.receiver.enabled=false'` and wait for pod termination.

**Expected:** Receiver Deployment and Service exist when enabled; are removed when disabled. /metrics endpoint returns Prometheus metrics when running.

**Cleanup:** Helm upgrade with receiver.enabled=false; kill port-forward.

**Automatable?** yes (bucket: api-mods; pure pod/svc resource observation).

---

### cluster-operations-feature

**Preconditions:** None (feature is purely API/RBAC; no external dependencies).

**Resources created:** none (RBAC binding already created by Helm; clusterOps affects what dashboard API endpoints are available).

**Steps:**
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'clusterOps.enabled=true'` and wait for RBAC resources to update.
2. Login to the dashboard as audit018-admin. (Login cost: 1)
3. Navigate to Admin → Cluster (or Cluster Settings page, if applicable).
4. Observe that "Add Node" / "Download Kubeconfig" buttons or similar cluster operations are visible.
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'clusterOps.enabled=false'` and wait for RBAC update.
6. Refresh dashboard and verify cluster operations buttons are no longer visible.

**Expected:** Cluster operations buttons/UI elements appear when enabled; disappear when disabled. RBAC ClusterRole/ClusterRoleBinding for kube-system access are created/removed.

**Cleanup:** Helm upgrade with clusterOps.enabled=false.

**Automatable?** yes (bucket: api-rbac; dashboard UI observation; requires admin login).

---

### mcp-server-deployment

**Preconditions:** None (MCP server is self-contained).

**Resources created:** none (toggles the chart's fixed-name `gameplane-mcp-server` Deployment, not an audit018-named object).

**Steps:**
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'mcpServer.enabled=true'` and wait for MCP server pod to start.
2. Verify MCP server pod is running: `kubectl get pod -n gameplane-system -l app.kubernetes.io/name=gameplane-mcp-server`.
3. Verify the pod is ready and logs show it waiting for a session: `kubectl logs -n gameplane-system -l app.kubernetes.io/name=gameplane-mcp-server | grep -i 'idle: waiting for'` (the idle-mode log line is `mcp-server idle: waiting for \`kubectl exec -i ... -- /mcp-server serve\` sessions` — it never logs "serving", "ready", or "json-rpc").
4. Test MCP server over stdin/stdout: `kubectl exec -i deploy/gameplane-mcp-server -n gameplane-system -- /mcp-server serve` and send a JSON-RPC call (e.g., `{"jsonrpc":"2.0","method":"list_resources","id":1}`); observe response. (Use `-i`, not `-it` — a pty would corrupt the JSON-RPC stdio framing; the binary is `/mcp-server` with subcommand `serve`, not a `--serve` flag — this is the chart's own documented invocation, see templates/mcp-server.yaml.)
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'mcpServer.enabled=false'` and wait for pod termination.

**Expected:** MCP server Deployment exists and pod runs when enabled; Deployment is removed when disabled. MCP server is read-only and responds to JSON-RPC calls over stdin/stdout.

**Cleanup:** Helm upgrade with mcpServer.enabled=false.

**Automatable?** yes (bucket: api-mods; pod existence and JSON-RPC handshake test).

---

### network-policies-enforcement

**Preconditions:** None (policies are chart-defined, no external resources needed).

**Resources created:** audit018-netpol-test (GameServer in gamesNamespace, created and deleted by this procedure, used only to produce a game pod to check selectors against). The NetworkPolicy objects themselves are the chart's own fixed-name objects — with default values this is `default-deny-ingress`, `default-deny-egress`, `allow-agent-to-apiserver`, `allow-api-to-agent`, `allow-kubelet-probes`, `allow-game-public-egress` (gameEgress.enabled defaults true), `allow-backup-restore-egress` (backupEgress defaults enabled), `allow-sentinel-ingress`, `allow-sentinel-to-apiserver`, `allow-sentinel-to-game`, and `allow-sentinel-to-game-egress`, plus `allow-prometheus-to-agent` when serviceMonitors.enabled is set with a non-empty scrapeNamespaceSelector — not audit018-named or audit018-labeled; they are Helm-managed, not created by this procedure with `kubectl`. The operator additionally creates per-GameServer ingress (and tunnel) NetworkPolicies outside this chart template.

**Steps:**
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'networkPolicies.enabled=true'` and wait for Helm to apply. Note: `networkPolicies.enabled` defaults to `true` in `charts/gameplane/values.yaml:327-328`, and kubelab's baseline (`audit/evidence/baseline/helm-values.json`) does not override it, so this is a no-op against kubelab — it does not newly enable anything.
2. Verify NetworkPolicy objects exist in gamesNamespace: `kubectl get networkpolicies -n gameplane-games | wc -l` (expect >0 — with the chart's default values this is 11 fixed-name policies, see Resources created above, plus any per-GameServer policies the operator has created).
3. Verify the default-deny-ingress policy renders as the chart defines it (`templates/networkpolicies.yaml`): `kubectl get networkpolicy default-deny-ingress -n gameplane-games -o jsonpath='{.spec.podSelector} {.spec.ingress}'` (expect `{}` for podSelector, matching every pod in the namespace, and an empty list for ingress — the default-deny baseline the template actually renders when networkPolicies.enabled=true).
4. Create a test GameServer (`audit018-netpol-test`, labeled `gameplane.io/audit: "018"`, using the `minecraft-java` template) and wait for its pod to start. Do **not** try to find "the" NetworkPolicy applied to it by filtering `kubectl get networkpolicies -n gameplane-games -l app.kubernetes.io/name=gameplane-game` or any other label selector — the chart labels these NetworkPolicy *objects* only via the shared `gameplane.labels` helper (`app.kubernetes.io/instance`, `app.kubernetes.io/version`, `app.kubernetes.io/managed-by`; `charts/gameplane/templates/_helpers.tpl:88-92`), which never includes `app.kubernetes.io/name`, so that filter returns zero objects even though the policies exist and apply. Instead list every policy unfiltered and read `spec.podSelector` from each: `kubectl get networkpolicies -n gameplane-games -o yaml`. Confirm the test pod's own labels (`kubectl get pod -n gameplane-games -l app.kubernetes.io/instance=audit018-netpol-test -o jsonpath='{.items[0].metadata.labels}'`, expect `app.kubernetes.io/name=gameplane-game` among them, set by the StatefulSet pod-template builder at `operator/internal/controller/gameserver_controller.go:1371-1375` (name at line 1372; `ss.Spec.Template.Labels = labels` at line 1379) — not line 573, which is the game Service's selector, not a pod label) match `default-deny-ingress`'s empty podSelector (matches every pod) and `allow-kubelet-probes`'/`allow-api-to-agent`'s `matchLabels: {app.kubernetes.io/name: gameplane-game}` (`charts/gameplane/templates/networkpolicies.yaml:85` for allow-api-to-agent, `:145` for allow-kubelet-probes).
5. Delete the test GameServer: `kubectl delete gameserver audit018-netpol-test -n gameplane-games --wait=true`.
6. **Do not run this step against kubelab as written.** `networkPolicies.enabled` defaults to `true` (`charts/gameplane/values.yaml:327-328`) and kubelab's baseline does not override it, so `--set 'networkPolicies.enabled=false'` is not a "revert" — it is a baseline write that deletes every pre-existing chart NetworkPolicy (all 11 fixed-name policies listed under Resources created, including `default-deny-egress`, which `conventions.md`/`rounds.md` setup relies on staying present for the `audit018-restic` NetworkPolicy to make sense) from any pre-existing GameServers, not just this procedure's test object. Setting it `false` also flips the operator to `--game-ingress-policy=false` (`charts/gameplane/templates/operator.yaml:314-323`), which deletes the per-GameServer ingress NetworkPolicies of every pre-existing GameServer in the cluster. Disabling and re-enabling it, even temporarily, is a baseline change requiring OD-level approval before it is attempted; absent that approval, skip this step (there is nothing to revert, since step 1 was a no-op) and leave/confirm `networkPolicies.enabled=true`.

**Expected:** NetworkPolicy objects exist in gamesNamespace under the chart's default values (`networkPolicies.enabled=true`, unchanged by this procedure). Policies enforce ingress/egress rules as defined in the chart.

**Cleanup:** Leave (or restore) `networkPolicies.enabled=true` — do **not** set it to `false`; see step 6. Delete `audit018-netpol-test` if step 5 did not already remove it.

**Automatable?** yes (bucket: api-rbac or multicluster; NetworkPolicy resource observation).

---

### pod-security-enforcement

**Preconditions:** None (label applied to gamesNamespace).

**Resources created:** none (only namespace label is set).

**Steps:**
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'podSecurity.enforceRestricted=true'` and wait for Helm to apply.
2. Verify pod security label is set on gamesNamespace: `kubectl get namespace gameplane-games -o jsonpath='{.metadata.labels}' | grep 'pod-security'`.
3. Expected label: `pod-security.kubernetes.io/enforce=restricted`.
4. Attempt to create a pod that violates the restricted policy (e.g., runs as root or with privileged: true) in gamesNamespace and verify it is denied by the pod security policy: `kubectl run audit018-privileged-test -n gameplane-games --image=busybox --overrides='{"spec":{"containers":[{"name":"test","image":"busybox","securityContext":{"privileged":true}}]}}'` and expect failure.
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'podSecurity.enforceRestricted=false'` and verify label is removed.
6. Attempt to create the privileged pod again and verify it is now allowed (i.e., no enforcement).

**Expected:** Label pod-security.kubernetes.io/enforce=restricted is set on gamesNamespace when enabled; is removed when disabled. Restricted pods are rejected by K8s when label is present; are allowed when absent.

**Cleanup:** Helm upgrade with podSecurity.enforceRestricted=false; delete test pod if it was created.

**Automatable?** yes (bucket: api-rbac or operator; namespace label observation and pod security admission test).

---

### game-egress-policies

**Preconditions:** None (networkPolicies.enabled should be true for egress policies to be meaningful; see network-policies-enforcement above).

**Resources created:** none (policies are defined in chart; test may create audit018-game-egress-test GameServer).

**Steps:**
0. Record the current gameEgress.enabled setting: `helm get values gameplane -n gameplane-system -a -o json | jq -r '.networkPolicies.gameEgress.enabled' > /tmp/audit018-gameegressenabled-before.txt`. `-a` returns the computed values, chart defaults included, so this is the effective value even when the release never overrode it (`true` on kubelab). Do not add jq's `// "true"` fallback: `//` also replaces an explicit `false`.
1. Ensure networkPolicies.enabled=true (run network-policies-enforcement first if needed).
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'networkPolicies.gameEgress.enabled=true'` and wait for Helm to apply.
3. Verify NetworkPolicy for game egress exists: `kubectl get networkpolicies -n gameplane-games | grep -i egress` or inspect the policy YAML to confirm it allows only TCP 80/443 and blocks RFC1918 ranges.
4. Create a test GameServer in gameplane-games and start it, then attempt to download an asset from the public internet (should succeed) and from an internal service (should fail): simulate by running a curl command inside the game pod.
5. Revert: restore the pre-test value recorded in step 0: `EGRESS_VAL=$(cat /tmp/audit018-gameegressenabled-before.txt) && helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set "networkPolicies.gameEgress.enabled=$EGRESS_VAL"`.
6. Verify the egress policy matches the restored setting. With kubelab's recorded `true`, `kubectl get networkpolicy allow-game-public-egress -n gameplane-games` still succeeds and the game pod still cannot reach internal services. Only a recorded `false` removes `allow-game-public-egress`, and that does not unrestrict the pod: `default-deny-egress` still applies.

**Expected:** Game pods are restricted to TCP 80/443 egress to non-RFC1918 ranges when enabled. When disabled, `allow-game-public-egress` is removed while `default-deny-egress` stays, so public downloads are blocked too (the `networkPolicies.gameEgress` comment in `charts/gameplane/values.yaml`); disabling never unrestricts game pods.

**Cleanup:** Restore `networkPolicies.gameEgress.enabled` to the value recorded in step 0 — do **not** set it to `false`; see step 5. Delete test GameServer.

**Automatable?** no (blocked candidate: requires actual game pod startup and network testing; alternative: verify NetworkPolicy object definition matches expected egress rules).

---

### game-ingress-policies

**Preconditions:** None (networkPolicies.enabled should be true).

**Resources created:** none (test may create audit018-game-ingress-test GameServer).

**Steps:**
0. Record the current gameIngress.enabled setting: `helm get values gameplane -n gameplane-system -a -o json | jq -r '.networkPolicies.gameIngress.enabled' > /tmp/audit018-gameingressenabled-before.txt`. `-a` includes chart defaults, so this is the effective value (`true` on kubelab). Do not add jq's `// "true"` fallback: `//` also replaces an explicit `false`.
1. Ensure networkPolicies.enabled=true.
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'networkPolicies.gameIngress.enabled=true'` and wait for Helm to apply.
3. Verify NetworkPolicy for game ingress exists: `kubectl get networkpolicies -n gameplane-games | grep game-ingress`.
4. Create a test GameServer with a game pod that listens on port 25565 (Minecraft default), advertises it (Advertise: true in template), and verify the operator creates a per-server NetworkPolicy allowing ingress from 0.0.0.0/0 on that port.
5. Attempt to connect to the game server port from an external client (e.g., from the audit devbox); expect success if policy is enabled.
6. Revert: restore the pre-test value recorded in step 0: `GAMEINGRESS_VAL=$(cat /tmp/audit018-gameingressenabled-before.txt) && helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set "networkPolicies.gameIngress.enabled=$GAMEINGRESS_VAL"`.
7. Verify the per-server NetworkPolicy reflects the restored setting: removed if the pre-test value was `false`, still present if it was `true` (as recorded in step 0).

**Expected:** Per-GameServer ingress NetworkPolicy allows advertised ports from configured CIDRs when enabled; is removed when disabled.

**Cleanup:** Restore `networkPolicies.gameIngress.enabled` to the value recorded in step 0 — do **not** set it to `false`; see step 6. Delete test GameServer.

**Automatable?** no (blocked candidate: requires game pod startup and external connectivity test; alternative: verify NetworkPolicy object definition).

---

### service-monitors

**Preconditions:** Prometheus Operator CRDs (ServiceMonitor, PodMonitor) must be installed in the cluster. If not available, this is a blocked candidate.

**Resources created:** none (toggles the chart's fixed-name `gameplane-operator`/`gameplane-api` ServiceMonitors in gameplane-system and the `gameplane-agent` PodMonitor in gameplane-games; none are audit018-named).

**Steps:**
1. Verify Prometheus Operator is installed: `kubectl get crd servicemonitors.monitoring.coreos.com` (expect success; if not found, skip to blocked candidate note).
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'serviceMonitors.enabled=true'` and wait for Helm to apply.
3. Verify ServiceMonitor for operator is created: `kubectl get servicemonitor -n gameplane-system gameplane-operator` (fixed object name; the ServiceMonitor itself carries no `app` label — only its selector targets one).
4. Verify ServiceMonitor for API is created: `kubectl get servicemonitor -n gameplane-system gameplane-api`.
5. Verify PodMonitor for agent sidecars is created: `kubectl get podmonitor -n gameplane-games gameplane-agent` (it lives in the games namespace, not gameplane-system).
6. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'serviceMonitors.enabled=false'` and wait for Helm to remove ServiceMonitor/PodMonitor objects.

**Expected:** ServiceMonitor and PodMonitor objects are created in gameplane-system when enabled; are removed when disabled. Objects have labels matching Prometheus scrape selectors.

**Cleanup:** Helm upgrade with serviceMonitors.enabled=false.

**Automatable?** no (blocked candidate: needs Prometheus Operator CRDs; alternative: verify object creation in a cluster with CRDs present).

---

### prometheus-rules

**Preconditions:** Prometheus Operator CRDs (PrometheusRule) must be installed in the cluster.

**Resources created:** none (toggles the chart's fixed-name `gameplane-operator` PrometheusRule, not an audit018-named object).

**Steps:**
1. Verify Prometheus Operator is installed: `kubectl get crd prometheusrules.monitoring.coreos.com` (expect success).
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'prometheusRules.enabled=true'` and wait for Helm to apply.
3. Verify PrometheusRule object is created: `kubectl get prometheusrule -n gameplane-system gameplane-operator` (fixed object name; no `app` label exists on it).
4. Inspect the rule YAML and verify it includes alert rules for operator metrics: `kubectl get prometheusrule -n gameplane-system gameplane-operator -o yaml | grep -i alert`.
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'prometheusRules.enabled=false'` and verify PrometheusRule is removed.

**Expected:** PrometheusRule object is created when enabled; is removed when disabled. Rule includes operator-specific alerts and Prometheus scrape configuration references.

**Cleanup:** Helm upgrade with prometheusRules.enabled=false.

**Automatable?** no (blocked candidate: needs Prometheus Operator CRDs; alternative: verify PrometheusRule resource definition on a Prometheus Operator-equipped cluster).

---

### grafana-dashboards

**Preconditions:** Grafana with sidecar dashboard loader (grafana-sidecar) must be installed in the cluster. This is typically part of kube-prometheus-stack.

**Resources created:** none (toggles the chart's fixed-name `gameplane-operator-dashboard` ConfigMap, not an audit018-named object).

**Steps:**
1. Verify Grafana is installed and sidecar is configured: `kubectl get deployment -n monitoring -l app=grafana` (adjust namespace as needed; common is `monitoring` or `prometheus`).
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'grafanaDashboards.enabled=true'` and wait for Helm to apply.
3. Verify dashboard ConfigMap is created: `kubectl get configmap -n gameplane-system -l grafana_dashboard=1`.
4. Access Grafana UI and navigate to Dashboards → browse; verify a new dashboard for Gameplane appears in the list.
5. Click the dashboard and verify it displays operator metrics (e.g., controller reconciliation rates, error counts).
6. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'grafanaDashboards.enabled=false'` and verify ConfigMap is removed.
7. Refresh Grafana UI; verify dashboard is no longer available.

**Expected:** Grafana dashboard ConfigMap is created when enabled; is removed when disabled. Dashboard appears in Grafana UI when sidecar detects the ConfigMap; disappears when ConfigMap is removed.

**Cleanup:** Helm upgrade with grafanaDashboards.enabled=false.

**Automatable?** no (blocked candidate: needs Grafana and sidecar; alternative: verify ConfigMap creation and schema on a Grafana-equipped cluster).

---

### default-module-source

**OD-021 item 5 (resolved):** `defaultModuleSource.enabled` gates whether the chart's Helm-managed `default` ModuleSource exists at all — toggling it to `false` against the live release **deletes** the pre-existing `default` ModuleSource (and back to `true` recreates it as a new object with a new UID), and `default` is one of the pre-existing objects the audit must never write to (conventions.md). The live toggle is therefore **blocked**. This procedure is a `helm template` rendering check only: it never runs `helm upgrade` against kubelab.

**Preconditions:** A local checkout of `charts/gameplane` (no cluster access required for this check).

**Resources created:** none (dry-run template rendering only; nothing is applied to the cluster).

**Steps:**
1. Render with the source enabled (the chart default), scoped to just the ModuleSource template — a full `helm template` render of this chart also emits `templates/mtls.yaml`'s generated CA/client-cert Secrets (`genCA`/`genSignedCert`, since the `lookup` call returns empty in an offline template render), and those Secrets hold generated private keys that must never land in evidence (conventions.md): `helm template gameplane charts/gameplane -n gameplane-system --set 'defaultModuleSource.enabled=true' --show-only templates/modulesource.yaml > /tmp/audit018-tpl-default-on.yaml` and confirm it contains a `ModuleSource` document named `default`: `grep -A2 '^kind: ModuleSource' /tmp/audit018-tpl-default-on.yaml | grep 'name: default'`.
2. Render with the source disabled, same scoped template: `helm template gameplane charts/gameplane -n gameplane-system --set 'defaultModuleSource.enabled=false' --show-only templates/modulesource.yaml > /tmp/audit018-tpl-default-off.yaml` and confirm no `ModuleSource` named `default` is rendered: `grep -c '^kind: ModuleSource' /tmp/audit018-tpl-default-off.yaml` (expect `0`, or if other ModuleSources are enabled by other values in the same render, confirm none of them is named `default`).
3. Save both rendered files as evidence: `mkdir -p ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-017 && cp /tmp/audit018-tpl-default-on.yaml /tmp/audit018-tpl-default-off.yaml ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-017/` (see conventions.md; scoped to `templates/modulesource.yaml` only, so this contains no secrets or cluster state, only chart output for that one template).

**Expected:** The rendered manifest includes a `default` ModuleSource when `defaultModuleSource.enabled=true` and omits it when `false`. This is not verified live against kubelab's actual `default` ModuleSource.

**Cleanup:** none (no cluster resources touched); remove the two `/tmp/audit018-tpl-default-*.yaml` scratch files once copied to evidence.

**Automatable?** yes (bucket: api-mods; pure `helm template` rendering check, no cluster required — the live toggle itself is blocked per OD-021 item 5 and is not automated).

---

### module-signature-verification

**OD-021 items 6 and 21 (resolved):** kubelab's `default` ModuleSource is `type: git` (not `oci`), and `ModuleSourceSpec.verify` is only valid when `spec.type == "oci"` (`operator/api/v1alpha1/modulesource_types.go:31`, a CEL rule). The Helm value `defaultModuleSource.oci.verify.enabled` does exist (`charts/gameplane/values.yaml:553-554`, default `false`), but it is inert on kubelab: `templates/modulesource.yaml` only renders `verify:` inside the `{{- else if eq $type "oci" }}` branch (lines 20-36), and kubelab's baseline sets `defaultModuleSource.type: git` (`audit/evidence/baseline/helm-values.json`), so that branch never executes and the value has no effect on a git source. This procedure never touches Helm or the `default`/`uploads` ModuleSources at all. Instead it stands up an in-cluster registry (`audit018-registry`) and a `kubectl`-created OCI ModuleSource (`audit018-verify-source`) with `spec.verify.key` set, then pushes one signed and one unsigned test bundle to it: the unsigned one must be rejected. This mirrors the operator's own e2e coverage in `test/e2e/module_verify_e2e_test.go` and `test/e2e/module_verify_signed_e2e_test.go`, scaled down to `kubectl`/`oras`/`cosign` one-shot Jobs run by hand instead of Go test helpers.

**Preconditions:** None beyond cluster access. No Helm change and no interaction with the `default` or `uploads` ModuleSources.

**Resources created:** `audit018-registry` (Deployment + Service, in-cluster plain-HTTP OCI registry, modeled on `test/e2e/fixtures/oci-registry.yaml`); `audit018-module-bundle-unsigned` and `audit018-module-bundle-signed` (ConfigMaps holding the test module's `module.yaml`/`template.yaml` for the `0.1.0` and `0.2.0` tags respectively — each manifest's `version:` field matches the OCI tag it is pushed under, modeled on `test/e2e/fixtures/oras-push-job.yaml`); `audit018-cosign-signer` (ServiceAccount + Role + RoleBinding, scoped to Secret CRUD in `gameplane-system`, modeled on `test/e2e/fixtures/cosign-sign-job.yaml:28-63`); `audit018-oras-push-unsigned` and `audit018-oras-push-signed` (one-shot oras push Jobs, pushing tags `0.1.0` and `0.2.0` of module `audit018-test-game` — the operator's OCI client only keeps semver tags, `semver.IsValid("v"+t)` in `operator/internal/oci/client.go:79-83`, so tags must be valid semver, not the literal strings `unsigned`/`signed`); `audit018-cosign-keypair` (Secret holding a cosign keypair generated in-cluster, modeled on `test/e2e/fixtures/cosign-sign-job.yaml`); `audit018-cosign-sign` (one-shot Job, running as the `audit018-cosign-signer` ServiceAccount, that signs only the `0.2.0` tag); `audit018-verify-source` (ModuleSource, `type: oci`, `spec.verify.key.name: audit018-cosign-keypair`); `audit018-verify-unsigned` (Module CR installing version `0.1.0`, unsigned) and `audit018-verify-signed` (Module CR installing version `0.2.0`, signed).

**Steps:**
1. Deploy the in-cluster registry:
```sh
kubectl apply -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: audit018-registry
  namespace: gameplane-system
  labels:
    gameplane.io/audit: "018"
spec:
  replicas: 1
  selector:
    matchLabels: { app: audit018-registry }
  template:
    metadata:
      labels: { app: audit018-registry, gameplane.io/audit: "018" }
    spec:
      containers:
        - name: registry
          image: registry:2.8.3
          ports: [{ name: http, containerPort: 5000 }]
          env: [{ name: REGISTRY_HTTP_ADDR, value: ":5000" }]
---
apiVersion: v1
kind: Service
metadata:
  name: audit018-registry
  namespace: gameplane-system
  labels:
    gameplane.io/audit: "018"
spec:
  selector: { app: audit018-registry }
  ports: [{ name: http, port: 5000, targetPort: http }]
EOF
kubectl wait --for=condition=available deployment/audit018-registry -n gameplane-system --timeout=90s
```
2. Create two test bundle ConfigMaps (module name `audit018-test-game`, same two-file layout `modules/build.sh` produces) and push each under its own semver tag with `oras`, unsigned. Tags must be valid semver — the operator's OCI client keeps only semver tags and drops everything else (`ListTags`, `operator/internal/oci/client.go:79-83`, `semver.IsValid("v"+t)`); the literal tags `unsigned`/`signed` are not semver and are silently filtered out, which leaves `ListTags` empty, `indexModule` failing with `"no semver tags found"` (`operator/internal/modsrc/oci.go:76-77`), and — because a single-module source treats any one module error as total failure (`operator/internal/modsrc/oci.go:58-59`, "all N module(s) failed to index") — `status.modules` on `audit018-verify-source` never populates at all, so step 4's wait times out before either Module in steps 5-6 can even resolve a version. Use `0.1.0` for the unsigned tag and `0.2.0` for the signed tag. Each bundle's own `module.yaml`/`template.yaml` `version:` field must match the OCI tag it is pushed under — `modules/build.sh` derives its push tag from that same field (`push_one()`, `modules/build.sh`), so a bundle whose manifest version disagreed with its tag could never be produced by the real build pipeline and would be an invalid test fixture:
```sh
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: audit018-module-bundle-unsigned
  namespace: gameplane-system
  labels: { gameplane.io/audit: "018" }
data:
  module.yaml: |
    apiVersion: gameplane.local/module/v1
    name: audit018-test-game
    displayName: Audit018 Test Game
    version: "0.1.0"
    game: audit018-test-game
    summary: Audit-only test module
    license: MIT
    gameplaneMinVersion: 0.1.0
  template.yaml: |
    apiVersion: gameplane.local/v1alpha1
    kind: GameTemplate
    metadata:
      name: audit018-test-game
      labels: { gameplane.local/module: audit018-test-game, gameplane.io/audit: "018" }
    spec:
      displayName: Audit018 Test Game
      game: audit018-test-game
      version: "0.1.0"
      image: busybox:1.37.0
      command: ["sh", "-c", "sleep 100000"]
      ports:
        - { name: noop, containerPort: 12345, advertise: true, protocol: TCP }
EOF
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: audit018-module-bundle-signed
  namespace: gameplane-system
  labels: { gameplane.io/audit: "018" }
data:
  module.yaml: |
    apiVersion: gameplane.local/module/v1
    name: audit018-test-game
    displayName: Audit018 Test Game
    version: "0.2.0"
    game: audit018-test-game
    summary: Audit-only test module
    license: MIT
    gameplaneMinVersion: 0.1.0
  template.yaml: |
    apiVersion: gameplane.local/v1alpha1
    kind: GameTemplate
    metadata:
      name: audit018-test-game
      labels: { gameplane.local/module: audit018-test-game, gameplane.io/audit: "018" }
    spec:
      displayName: Audit018 Test Game
      game: audit018-test-game
      version: "0.2.0"
      image: busybox:1.37.0
      command: ["sh", "-c", "sleep 100000"]
      ports:
        - { name: noop, containerPort: 12345, advertise: true, protocol: TCP }
EOF
for pair in "audit018-oras-push-unsigned:0.1.0:audit018-module-bundle-unsigned" "audit018-oras-push-signed:0.2.0:audit018-module-bundle-signed"; do
  jobname="$(cut -d: -f1 <<<"$pair")"
  tag="$(cut -d: -f2 <<<"$pair")"
  bundle="$(cut -d: -f3 <<<"$pair")"
  kubectl apply -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: $jobname
  namespace: gameplane-system
  labels: { gameplane.io/audit: "018" }
spec:
  backoffLimit: 4
  template:
    metadata: { labels: { gameplane.io/audit: "018" } }
    spec:
      restartPolicy: OnFailure
      containers:
        - name: push
          image: ghcr.io/oras-project/oras:v1.3.3
          command: ["oras"]
          args:
            - push
            - --plain-http
            - --artifact-type=application/vnd.gameplane.module.v1+json
            - audit018-registry.gameplane-system.svc:5000/audit018-test-game:$tag
            - module.yaml:application/vnd.gameplane.module.metadata.v1+yaml
            - template.yaml:application/vnd.gameplane.module.template.v1+yaml
          workingDir: /workspace
          volumeMounts: [{ name: bundle, mountPath: /workspace }]
      volumes:
        - { name: bundle, configMap: { name: $bundle } }
EOF
  kubectl wait --for=condition=complete job/$jobname -n gameplane-system --timeout=90s
done
```
(the unsigned bundle is pushed as tag `0.1.0`, the one to be signed as tag `0.2.0` — both valid semver, so the operator's `ListTags` keeps them.)
3. Sign only the `0.2.0` tag in-cluster (keyed, offline, no Rekor — matches `modules/build.sh --sign` minus the Rekor upload). This Job needs `get/list/create/update/patch/delete` on Secrets in `gameplane-system` for `generate-key-pair k8s://…` — the default ServiceAccount has no such permissions, so first create a scoped `audit018-cosign-signer` ServiceAccount/Role/RoleBinding (modeled on `test/e2e/fixtures/cosign-sign-job.yaml:28-63`) and reference it from the Job:
```sh
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: ServiceAccount
metadata:
  name: audit018-cosign-signer
  namespace: gameplane-system
  labels: { gameplane.io/audit: "018" }
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: audit018-cosign-signer
  namespace: gameplane-system
  labels: { gameplane.io/audit: "018" }
rules:
  - apiGroups: [""]
    resources: [secrets]
    verbs: [get, list, create, update, patch, delete]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: audit018-cosign-signer
  namespace: gameplane-system
  labels: { gameplane.io/audit: "018" }
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: audit018-cosign-signer
subjects:
  - kind: ServiceAccount
    name: audit018-cosign-signer
    namespace: gameplane-system
EOF
```
```sh
kubectl apply -f - <<'EOF'
apiVersion: batch/v1
kind: Job
metadata:
  name: audit018-cosign-sign
  namespace: gameplane-system
  labels: { gameplane.io/audit: "018" }
spec:
  backoffLimit: 0
  template:
    metadata: { labels: { gameplane.io/audit: "018" } }
    spec:
      restartPolicy: Never
      serviceAccountName: audit018-cosign-signer
      initContainers:
        - name: keygen
          image: ghcr.io/sigstore/cosign/cosign:v3.1.2
          args: ["generate-key-pair", "k8s://gameplane-system/audit018-cosign-keypair"]
          env: [{ name: COSIGN_PASSWORD, value: "audit018-pass" }, { name: COSIGN_YES, value: "true" }, { name: HOME, value: /tmp }]
          volumeMounts: [{ name: tmp, mountPath: /tmp }]
      containers:
        - name: sign
          image: ghcr.io/sigstore/cosign/cosign:v3.1.2
          args:
            - sign
            - --key=k8s://gameplane-system/audit018-cosign-keypair
            - --new-bundle-format=false
            - --use-signing-config=false
            - --tlog-upload=false
            - --allow-http-registry
            - --allow-insecure-registry
            - --yes
            - audit018-registry.gameplane-system.svc:5000/audit018-test-game:0.2.0
          env: [{ name: COSIGN_PASSWORD, value: "audit018-pass" }, { name: COSIGN_YES, value: "true" }, { name: HOME, value: /tmp }]
          volumeMounts: [{ name: tmp, mountPath: /tmp }]
      volumes: [{ name: tmp, emptyDir: {} }]
EOF
kubectl wait --for=condition=complete job/audit018-cosign-sign -n gameplane-system --timeout=120s
```
4. Create the verify-enabled OCI ModuleSource, keyed to the Secret cosign just wrote:
```sh
kubectl apply -f - <<'EOF'
apiVersion: gameplane.local/v1alpha1
kind: ModuleSource
metadata:
  name: audit018-verify-source
  labels: { gameplane.io/audit: "018" }
spec:
  type: oci
  oci:
    url: audit018-registry.gameplane-system.svc:5000
    insecure: true
    modules: [{ name: audit018-test-game }]
  verify:
    key: { name: audit018-cosign-keypair }
  refreshInterval: 10m
EOF
kubectl wait --for=jsonpath='{.status.modules}' modulesource/audit018-verify-source --timeout=90s
```
5. Install the **unsigned** tag (`0.1.0`) and confirm it is rejected:
```sh
mkdir -p ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-018
kubectl apply -f - <<'EOF'
apiVersion: gameplane.local/v1alpha1
kind: Module
metadata:
  name: audit018-verify-unsigned
  labels: { gameplane.io/audit: "018" }
spec:
  source: { name: audit018-verify-source }
  name: audit018-test-game
  version: "0.1.0"
EOF
kubectl wait --for=jsonpath='{.status.phase}'=Failed module/audit018-verify-unsigned --timeout=120s
kubectl get module audit018-verify-unsigned -o jsonpath='{.status.lastError}' | tee ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-018/unsigned-lasterror.txt
# expect lastError to contain "cosign verify" (operator wraps it: "cosign verify <ref>@<digest>: …")
kubectl get gametemplate audit018-verify-unsigned 2>&1 | tee ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-018/unsigned-no-gametemplate.txt | grep -i "not found"  # no GameTemplate must be materialized
```
6. Install the **signed** tag (`0.2.0`) and confirm it is accepted:
```sh
kubectl apply -f - <<'EOF'
apiVersion: gameplane.local/v1alpha1
kind: Module
metadata:
  name: audit018-verify-signed
  labels: { gameplane.io/audit: "018" }
spec:
  source: { name: audit018-verify-source }
  name: audit018-test-game
  version: "0.2.0"
EOF
kubectl wait --for=jsonpath='{.status.phase}'=Ready module/audit018-verify-signed --timeout=120s
kubectl get module audit018-verify-signed -o jsonpath='{.status.phase}' | tee ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-018/signed-phase.txt
kubectl get gametemplate audit018-verify-signed | tee ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-018/signed-gametemplate.txt  # must exist
```

**Expected:** The unsigned bundle drives `Module.status.phase=Failed` with a `cosign verify` error in `status.lastError`, and materializes no GameTemplate. The signed bundle reaches `Ready` and materializes its GameTemplate. `kubelab`'s real `default` (git) ModuleSource is untouched throughout.

**Cleanup:** `kubectl delete module audit018-verify-unsigned audit018-verify-signed; kubectl delete modulesource audit018-verify-source; kubectl delete job audit018-oras-push-unsigned audit018-oras-push-signed audit018-cosign-sign -n gameplane-system; kubectl delete secret audit018-cosign-keypair -n gameplane-system; kubectl delete configmap audit018-module-bundle-unsigned audit018-module-bundle-signed -n gameplane-system; kubectl delete deployment,service audit018-registry -n gameplane-system; kubectl delete serviceaccount,role,rolebinding audit018-cosign-signer -n gameplane-system` (the ServiceAccount/Role/RoleBinding created in step 3).

**Automatable?** yes (bucket: api-mods; this is exactly what `test/e2e/module_verify_e2e_test.go` and `test/e2e/module_verify_signed_e2e_test.go` already cover in CI — this manual run is a spot-check, not new coverage).

---

### upload-module-source

**OD-021 item 5 (resolved):** same as `default-module-source` above — `uploadModuleSource.enabled` gates the pre-existing `uploads` ModuleSource, and toggling it against the live release deletes/recreates that pre-existing object, which the audit must never write to. The live toggle is **blocked**; this is a `helm template` rendering check only.

**Preconditions:** A local checkout of `charts/gameplane` (no cluster access required for this check).

**Resources created:** none (dry-run template rendering only; nothing is applied to the cluster).

**Steps:**
1. Render with the source enabled, scoped to just the ModuleSource template (a full chart render also emits `templates/mtls.yaml`'s generated CA/client-cert Secrets, which must never land in evidence — see `default-module-source` above): `helm template gameplane charts/gameplane -n gameplane-system --set 'uploadModuleSource.enabled=true' --show-only templates/modulesource.yaml > /tmp/audit018-tpl-upload-on.yaml` and confirm it contains a `ModuleSource` document named `uploads`: `grep -A2 '^kind: ModuleSource' /tmp/audit018-tpl-upload-on.yaml | grep 'name: uploads'`.
2. Render with the source disabled, same scoped template: `helm template gameplane charts/gameplane -n gameplane-system --set 'uploadModuleSource.enabled=false' --show-only templates/modulesource.yaml > /tmp/audit018-tpl-upload-off.yaml` and confirm no `ModuleSource` named `uploads` is rendered.
3. Save both rendered files as evidence: `mkdir -p ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-019 && cp /tmp/audit018-tpl-upload-on.yaml /tmp/audit018-tpl-upload-off.yaml ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-019/` (conventions.md; scoped to `templates/modulesource.yaml` only, so this is chart output with no secrets or cluster state).

**Expected:** The rendered manifest includes an `uploads` ModuleSource when `uploadModuleSource.enabled=true` and omits it when `false`. This is not verified live against kubelab's actual `uploads` ModuleSource.

**Cleanup:** none (no cluster resources touched); remove the two `/tmp/audit018-tpl-upload-*.yaml` scratch files once copied to evidence.

**Automatable?** yes (bucket: api-mods; pure `helm template` rendering check, no cluster required — the live toggle itself is blocked per OD-021 item 5 and is not automated).

---

### packet-capture-sidecar

**OD-021 item 17 (resolved):** `capture.enabled=true` is approved as a per-round Helm override (same family as INV-CRD-031..035 in `crd.md`, which need the same flag). Before step 1, snapshot cluster state and record the override in `rounds.md` under the current round's "Helm overrides vs baseline" field; after step 5's revert, snapshot again and run `snapshot-diff` against the pre-override snapshot to confirm nothing besides the capture toggle itself changed.

**Preconditions:** None (capture is opt-in per GameServer; this test enables the cluster-wide feature).

**Resources created:** none (sidecars are injected into GameServers at creation time, not pre-created).

**Steps:**
0. Snapshot before the override and record it in `rounds.md`:
```sh
mkdir -p ~/gameplane-audit-018/snapshot-capture-before
bash ~/Gameplane/specs/018-v0-3-release-readiness/audit/tools/snapshot.sh ~/gameplane-audit-018/snapshot-capture-before
```
   Add a line to the current round's section in `rounds.md`: `**Helm overrides vs baseline**: capture.enabled=true (per-round override, OD-021 item 17, packet-capture-sidecar procedure; snapshot at ~/gameplane-audit-018/snapshot-capture-before)`.
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'capture.enabled=true'` and wait for Helm to apply.
2. Create a test GameServer with capture opt-in (using real line breaks below — the single-line form above would send a literal backslash-n string to the shell, not a heredoc):
```sh
kubectl apply -f - <<'EOF'
apiVersion: gameplane.local/v1alpha1
kind: GameServer
metadata:
  name: audit018-capture-test
  namespace: gameplane-games
  labels:
    gameplane.io/audit: "018"
spec:
  templateRef:
    name: minecraft-java
  capture:
    enabled: true
EOF
```
3. Wait for the GameServer pod to start and verify the capture sidecar is injected, saving the check as evidence: `mkdir -p ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-020 && kubectl get pod -n gameplane-games -l app.kubernetes.io/instance=audit018-capture-test -o yaml | grep -A5 'capture-sidecar' | tee ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-020/sidecar-injected.txt`.
4. Login as audit018-admin (Login cost: 1) per conventions.md, then verify capture is accessible via the API — the real route is `POST /servers/{name}:capture-start` (not `/api/v1/gameservers/.../captures/start`, which does not exist): `curl -X POST -H "Content-Type: application/json" -d '{"maxDurationSeconds":30,"maxSizeBytes":1048576}' -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: <csrf>" "$GP/servers/audit018-capture-test:capture-start?namespace=gameplane-games" | jq '.captureId' | tee ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-020/capture-start-response.txt` and expect a `captureId` in the response.
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'capture.enabled=false'` and wait for Helm to apply.
6. Create another test GameServer (e.g., `audit018-no-capture-test`) with capture opt-in and verify the sidecar is NOT injected (operator ignores capture requests when cluster feature is disabled), saving the check as evidence: `kubectl get pod -n gameplane-games -l app.kubernetes.io/instance=audit018-no-capture-test -o yaml | grep -A5 'capture-sidecar' | tee ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-020/sidecar-not-injected.txt` (expect no match/empty output).
7. Delete both test GameServers, then snapshot after the revert and diff against the before-snapshot:
```sh
kubectl delete gameserver audit018-capture-test audit018-no-capture-test -n gameplane-games --wait=true
mkdir -p ~/gameplane-audit-018/snapshot-capture-after
bash ~/Gameplane/specs/018-v0-3-release-readiness/audit/tools/snapshot.sh ~/gameplane-audit-018/snapshot-capture-after

mkdir -p ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-020
bash ~/Gameplane/specs/018-v0-3-release-readiness/audit/tools/snapshot-diff.sh \
  ~/gameplane-audit-018/snapshot-capture-before \
  ~/gameplane-audit-018/snapshot-capture-after \
  | tee ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-020/snapshot-diff.txt
```
   Record the diff result (and, once cleaned up, that the override is fully reverted) in the round's `rounds.md` entry from step 0.

**Expected:** Capture sidecar is injected into GameServers when cluster capture is enabled and GameServer requests it; is not injected when cluster feature is disabled. Capture API endpoints are available when enabled. After revert and cleanup, `snapshot-diff` shows no mismatches versus the pre-override snapshot (both test GameServers are `audit018-` and excluded from the comparison; everything else must be identical).

**Cleanup:** Helm upgrade with capture.enabled=false; delete test GameServers (done in step 7); the snapshot directories under `~/gameplane-audit-018/` are off-git and can be removed once the diff is recorded.

**Automatable?** no (blocked candidate: requires GameServer creation and pod injection verification; alternative: verify operator code includes capture sidecar injection logic when feature is enabled).

---

### web-dashboard-ui

**OD-021 item 7 (resolved):** `templates/web.yaml` gates the whole file (Deployment *and* Service `gameplane-web`) behind a single top-level `{{- if .Values.web.enabled }}` (`charts/gameplane/templates/web.yaml:1`), so `web.enabled=false` removes `svc/gameplane-web` entirely — the exact port-forward target (`kubectl port-forward -n gameplane-system svc/gameplane-web 18080:80`, conventions.md) every other procedure in every `.md` file depends on for `$GP`. This procedure therefore **runs last in the round, after `existing-storage-claim`** (see that procedure's ordering note — the two "runs at the end" claims are reconciled as: `existing-storage-claim` second-to-last and alone, `web-dashboard-ui` last), once every procedure that needs `$GP` has finished. It uses a temporary port-forward straight to `svc/gameplane-api` (which stays up throughout) instead of `svc/gameplane-web` to prove the API itself is unaffected while the dashboard is down.

**Preconditions:** None (web is enabled by default). All other procedures in this round must already be complete — this is the last procedure run, immediately after `existing-storage-claim`.

**Resources created:** none (web pod is already running if enabled).

**Steps:**
1. Before disabling web, kill the standing `$GP` port-forward to `svc/gameplane-web` from conventions.md (it will fail once the Service is removed anyway) and start a temporary one to `svc/gameplane-api` on a different local port, so the API's own health can still be checked while web is down. `gameplane-api`'s Service exposes port `80` (`{ name: http, port: 80, targetPort: http }`, `charts/gameplane/templates/api.yaml:435`), not `8080`: `kubectl port-forward -n gameplane-system svc/gameplane-api 18081:80 &`.
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'web.enabled=false'` and wait for web Deployment to terminate.
3. Verify web pod is removed: `kubectl get pod -n gameplane-system -l app.kubernetes.io/name=gameplane-web` (expect no results). Verify the Service is also gone (whole template is gated, not just the Deployment): `kubectl get svc gameplane-web -n gameplane-system` and save both checks as evidence:
```sh
mkdir -p ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-021
kubectl get pod -n gameplane-system -l app.kubernetes.io/name=gameplane-web 2>&1 | tee ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-021/web-pod-removed.txt
kubectl get svc gameplane-web -n gameplane-system 2>&1 | tee ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-021/web-svc-removed.txt  # expect NotFound
```
4. Attempt to reach the dashboard through the (now-nonexistent) `svc/gameplane-web` path — `curl http://127.0.0.1:18080/` on the old port-forward — and expect connection refused/failure, since there is nothing left to forward to. Save the result: `curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:18080/ 2>&1 | tee ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-021/dashboard-unreachable.txt`.
5. Confirm the API itself is still healthy via the temporary port-forward from step 1, using the unauthenticated `GET /healthz` route (`api/cmd/main.go:256`) — a cleaner health check than a GET on the POST-only `/auth/login`, which only proves the router is up, not that the handler is healthy: `curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:18081/healthz | tee ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-021/api-healthz.txt` and expect `200`, proving `gameplane-api` is unaffected by `web.enabled=false`.
6. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'web.enabled=true'` and wait for web pod to start.
7. Verify web pod is running: `kubectl get pod -n gameplane-system -l app.kubernetes.io/name=gameplane-web` (expect running pod) and save it: `kubectl get pod -n gameplane-system -l app.kubernetes.io/name=gameplane-web | tee ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-021/web-pod-restored.txt`.
8. Kill the temporary port-forward to `svc/gameplane-api` from step 1, restart the standard `$GP` port-forward to `svc/gameplane-web` per conventions.md, and confirm the dashboard is reachable again: `curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:18080/ | tee ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-021/dashboard-restored.txt` and expect success (login page or main dashboard HTML).

**Expected:** Web Deployment and Service are both deployed when web.enabled=true; both are removed when false. The dashboard (fronted by `svc/gameplane-web`) is unreachable while removed; `gameplane-api` stays healthy throughout via its own Service. `$GP` is restored to normal (pointing at `svc/gameplane-web` again) once this procedure — the last one in the round — completes.

**Cleanup:** Helm upgrade with web.enabled=true; kill the temporary port-forward to `svc/gameplane-api`; restart the standard `$GP` port-forward to `svc/gameplane-web`.

**Automatable?** yes (bucket: api-auth or api-mods; pod/svc observation and HTTP endpoint test — an automated run would need to run last in its bucket for the same port-forward-target reason).

---

### ingress-configuration

**Preconditions:** Ingress controller (nginx-ingress, Traefik, etc.) must be installed in the cluster. If not, ingress objects will be created but are inactive.

**Resources created:** none (toggles the chart's fixed-name `gameplane` Ingress object, not an audit018-named object).

**Steps:**
0. Record the current ingress.enabled setting: `helm get values gameplane -n gameplane-system -a -o json | jq -r '.ingress.enabled' > /tmp/audit018-ingress-enabled-before.txt`. `-a` includes chart defaults, so this is the effective value (`true` on kubelab). Do not add jq's `// "true"` fallback: `//` also replaces an explicit `false`.
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'ingress.enabled=true'` and wait for Helm to apply.
2. Verify Ingress object is created: `kubectl get ingress -n gameplane-system gameplane` (fixed object name; the Ingress carries no `app` label at all).
3. Verify ingress routing rules: `kubectl get ingress -n gameplane-system -o yaml | grep -A5 'host: gameplane.local'` (or your configured host).
4. Attempt to reach the dashboard via the configured host (e.g., `https://gameplane.local/`); expect success if ingress controller is present and routes are working.
5. Revert: restore the pre-test value recorded in step 0: `INGRESS_VAL=$(cat /tmp/audit018-ingress-enabled-before.txt) && helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set "ingress.enabled=$INGRESS_VAL"` and verify Ingress state.
6. Attempt to reach the dashboard via the same host; expect the result to match the value restored in step 5. It is still reachable for kubelab's recorded `true`. Only a recorded `false` removes the `gameplane` Ingress and gives no route or 404.

**Expected:** Ingress object is created when enabled; is removed when disabled. Routes are active and reachable when ingress is enabled (if ingress controller is present).

**Cleanup:** Restore `ingress.enabled` to the value recorded in step 0 — do **not** set it to `false`; see step 5.

**Automatable?** yes (bucket: api-auth or web e2e; Ingress object observation and HTTP request test).

---

### existing-storage-claim

**OD-021 item 8 (resolved):** Setting `api.storage.existingClaim` swaps the live API database for an empty PVC for as long as the toggle is on — any write the API makes during that window goes to the empty volume, not the real database. This procedure therefore **runs second-to-last in the round, alone**, immediately before `web-dashboard-ui` (the last procedure — see that procedure's ordering note; the two "runs at the end" claims are reconciled this way), after every other procedure that needs a working, unchanged API database has finished (in particular after every login-costing procedure, since `audit018-admin`/`audit018-operator`/etc. sessions and any state they create must already be settled).

A byte-for-byte SHA-256 of the whole `gameplane.db` file is **not** used: the check also spans windows where the API legitimately runs against the real database — after the step-0 scale-up, before step 2's swap, and after step 5's revert, before step 7's scale-down — and any normal write in those windows (new sessions, audit rows, the SQLite WAL checkpoint) would fail a byte-for-byte compare even though `existingClaim` behaved correctly. Instead this procedure dumps the **identity/config tables** that a correctly-behaved swap must never change — `users`, `roles`, `role_permissions`, `user_role_bindings`, `oidc_links`, `api_tokens`, `config`, `share_links`, `user_preferences` (`api/internal/db/migrations/001_init.sql`, `002_config.sql`, `003_roles.sql`, `004_cluster_rbac.sql`, `006_share_links.sql`, `010_share_links_expiry_nullable.sql`, `011_user_theme_preferences.sql`) — before the swap and after the revert, and diffs those dumps. `sessions` and `audit_events` are excluded from the diff: they are expected to gain rows from ordinary API activity in the surrounding windows (new login sessions, audit log entries from the very procedures that ran earlier in the round), and a difference there is not evidence of a problem.

The `kubectl scale deployment gameplane-api` calls in steps 0 and 7 are direct writes to the Helm-managed `gameplane-api` Deployment, a pre-existing object. This is covered by the OD-021 item 8 approval of this procedure (scaling the API down is intrinsic to taking a consistent snapshot of its live SQLite file) and needs no separate `rounds.md` override entry; the scale is reverted to `replicas: 1` within the same step each time.

**Preconditions:** A pre-existing PersistentVolumeClaim (PVC) named `audit018-api-storage` must exist in the gameplane-system namespace with at least 2Gi capacity and RWO access mode. This procedure runs second-to-last in the round and alone (no other procedure runs concurrently against the API); `web-dashboard-ui` runs immediately after it, last.

**Resources created:** `audit018-db-tool` Pod (ephemeral; created and deleted twice — once for the before-snapshot, once for the after-snapshot — same pattern as `upgrade.md`'s `restore-real-db`, since the API container has no `sqlite3` binary or shell to exec into).

**Steps:**
0. Snapshot the real database before touching `existingClaim`. Scale the API down first so the file isn't being written while copied:
```sh
kubectl scale deployment gameplane-api --replicas=0 -n gameplane-system
kubectl wait --for=delete pod -l app.kubernetes.io/name=gameplane-api -n gameplane-system --timeout=60s || true

kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: audit018-db-tool
  namespace: gameplane-system
  labels:
    gameplane.io/audit: "018"
spec:
  restartPolicy: Never
  containers:
    - name: tool
      image: alpine:3.20
      command: ["sleep", "300"]
      volumeMounts:
        - { name: data, mountPath: /data }
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: gameplane-api-data
EOF
kubectl wait --for=condition=Ready pod/audit018-db-tool -n gameplane-system --timeout=60s
mkdir -p ~/gameplane-audit-018/db-snapshots
kubectl exec -n gameplane-system audit018-db-tool -- sh -c \
  "apk add --no-cache sqlite >/dev/null && sqlite3 /data/gameplane.db '.dump users roles role_permissions user_role_bindings oidc_links api_tokens config share_links user_preferences'" \
  > ~/gameplane-audit-018/db-snapshots/existing-claim-before.sql
kubectl delete pod audit018-db-tool -n gameplane-system --wait=true

kubectl scale deployment gameplane-api --replicas=1 -n gameplane-system
kubectl rollout status deployment/gameplane-api -n gameplane-system --timeout=300s
```
1. Create a test PVC (omits `storageClassName` to use the cluster's default — kubelab-baseline.md and evidence/baseline/pvcs.json record no StorageClass named `standard`, and this is a k3s cluster, whose built-in default class is `local-path`):
```sh
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: audit018-api-storage
  namespace: gameplane-system
  labels:
    gameplane.io/audit: "018"
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 2Gi
EOF
```
and wait for it to bind (may be immediate if dynamic provisioning is available).
2. Protect the live database PVC first — required, or the chart's `fail()` guard aborts this upgrade to stop Helm from later pruning and erasing the real SQLite database. Check the annotation before writing it: the chart's own `templates/api.yaml:187` sets `helm.sh/resource-policy: keep` on `gameplane-api-data` whenever it renders that PVC (sqlite driver, `existingClaim` empty), which is kubelab's normal state, so this is expected to already be a no-op. Confirm first, and only annotate if it is actually missing: `kubectl get pvc gameplane-api-data -n gameplane-system -o jsonpath='{.metadata.annotations.helm\.sh/resource-policy}'` (expect `keep`; if it prints nothing, run `kubectl annotate pvc gameplane-api-data -n gameplane-system helm.sh/resource-policy=keep --overwrite` — this write to the pre-existing PVC is covered by the OD-021 item 8 approval of this procedure, the same as the `kubectl scale` calls in steps 0 and 7, and needs no separate `rounds.md` override entry). Then run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.storage.existingClaim=audit018-api-storage'` and wait for API pod to restart.
3. Verify API pod is running and mounts the PVC, saving the check as evidence: `mkdir -p ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-023 && kubectl get pod -n gameplane-system -l app.kubernetes.io/name=gameplane-api -o jsonpath='{.items[0].spec.volumes[?(@.name=="data")].persistentVolumeClaim.claimName}' | tee ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-023/pod-mounts-audit018-pvc.txt` (expect `audit018-api-storage`).
4. Verify the original `gameplane-api-data` PVC still exists, kept (not deleted) by the annotation in step 2 — it should no longer be the API pod's mounted volume, but the object itself must remain: `kubectl get pvc -n gameplane-system gameplane-api-data audit018-api-storage | tee ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-023/pvcs-both-present.txt` (expect both present).
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.storage.existingClaim='` and wait for API pod to restart.
6. Verify the API pod mounts `gameplane-api-data` again — the same PVC from step 2, re-adopted by Helm, not newly created: `kubectl get pod -n gameplane-system -l app.kubernetes.io/name=gameplane-api -o jsonpath='{.items[0].spec.volumes[?(@.name=="data")].persistentVolumeClaim.claimName}' | tee ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-023/pod-mounts-gameplane-api-data-restored.txt` (expect `gameplane-api-data`).
7. Snapshot the real database again, immediately after the revert, and diff the identity/config table dumps against the before-snapshot from step 0 (same scale-down / ephemeral-pod pattern; `sessions` and `audit_events` are intentionally excluded — see the OD-021 item 8 note above):
```sh
kubectl scale deployment gameplane-api --replicas=0 -n gameplane-system
kubectl wait --for=delete pod -l app.kubernetes.io/name=gameplane-api -n gameplane-system --timeout=60s || true

kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: audit018-db-tool
  namespace: gameplane-system
  labels:
    gameplane.io/audit: "018"
spec:
  restartPolicy: Never
  containers:
    - name: tool
      image: alpine:3.20
      command: ["sleep", "300"]
      volumeMounts:
        - { name: data, mountPath: /data }
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: gameplane-api-data
EOF
kubectl wait --for=condition=Ready pod/audit018-db-tool -n gameplane-system --timeout=60s
kubectl exec -n gameplane-system audit018-db-tool -- sh -c \
  "apk add --no-cache sqlite >/dev/null && sqlite3 /data/gameplane.db '.dump users roles role_permissions user_role_bindings oidc_links api_tokens config share_links user_preferences'" \
  > ~/gameplane-audit-018/db-snapshots/existing-claim-after.sql
kubectl delete pod audit018-db-tool -n gameplane-system --wait=true

kubectl scale deployment gameplane-api --replicas=1 -n gameplane-system
kubectl rollout status deployment/gameplane-api -n gameplane-system --timeout=300s

mkdir -p ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-023
diff ~/gameplane-audit-018/db-snapshots/existing-claim-before.sql \
     ~/gameplane-audit-018/db-snapshots/existing-claim-after.sql \
  > ~/gameplane-audit-018/db-snapshots/db-diff-raw.txt
# WARNING: sqlite3 `.dump` emits positional `INSERT INTO t VALUES(...)` rows, never
# `col = 'val'` assignments, so a `col = '...'` sed pattern can never match a dump
# line and must not be used here (it would silently redact nothing). The real
# secret-bearing columns in these tables are `users.pw_hash` (not
# `password_hash`; `001_init.sql:6`), `api_tokens.token` (not `token_hash`;
# `001_init.sql:43-44`), and `share_links.token_hash` (`006_share_links.sql:22`,
# `010_share_links_expiry_nullable.sql:21`) — the only one of the three actually
# named `token_hash`. Redact by column POSITION in each table's own
# `INSERT INTO <table> VALUES(...)` rows (position depends on each table's
# column order in its CREATE TABLE / current schema), e.g. with a per-table
# awk/sed pass, or open the raw diff and hand-redact any changed
# `users`/`api_tokens`/`share_links` row. db-diff-raw.txt itself stays under
# ~/gameplane-audit-018/ (off-git, per conventions.md) and must never be copied
# into the evidence tree unredacted. Redact an off-git copy first:
cp ~/gameplane-audit-018/db-snapshots/db-diff-raw.txt \
   ~/gameplane-audit-018/db-snapshots/db-diff-redacted.txt
chmod 600 ~/gameplane-audit-018/db-snapshots/db-diff-raw.txt \
   ~/gameplane-audit-018/db-snapshots/db-diff-redacted.txt
# Now open ~/gameplane-audit-018/db-snapshots/db-diff-redacted.txt and
# hand-redact (by column position, per the warning above) every changed
# `users`/`api_tokens`/`share_links` row. Do not skip this if the diff is
# non-empty. Only the reviewed, redacted copy enters the evidence tree:
cp ~/gameplane-audit-018/db-snapshots/db-diff-redacted.txt \
   ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-023/db-diff.txt
cat ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-HELM-023/db-diff.txt
```
   (the identity/config table dumps, not the raw `.db` file, are saved as evidence — `users.pw_hash`, `api_tokens.token`, and `share_links.token_hash` are secret-bearing columns, so any changed row touching one of those three tables must be hand-redacted, per conventions.md's no-secrets rule, before `db-diff.txt` is committed; see the warning above — no single find/replace pattern can safely do this against real `.dump` output.) An empty diff (ignoring any hand-redaction) means the dumped tables are identical and the real database's identity/config data was untouched throughout the test.

**Expected:** When existingClaim is set, the API pod mounts that PVC, and the original `gameplane-api-data` PVC (kept via step 2's annotation) is not deleted. When reverted to empty, Helm re-adopts and mounts that same original PVC again — no new PVC is created, and its data survives the whole test. The before/after dumps of `users`, `roles`, `role_permissions`, `user_role_bindings`, `oidc_links`, `api_tokens`, `config`, `share_links` and `user_preferences` are identical, proving no write reached those tables in the real database while `existingClaim` pointed at `audit018-api-storage`. (`sessions`/`audit_events` may differ; that is expected and acceptable.)

**Cleanup:** Helm upgrade with existingClaim empty; delete the test PVC `audit018-api-storage`; delete `~/gameplane-audit-018/db-snapshots/existing-claim-{before,after}.sql`, `~/gameplane-audit-018/db-snapshots/db-diff-raw.txt` and `~/gameplane-audit-018/db-snapshots/db-diff-redacted.txt` (off-git) once the redacted diff is recorded. Leave the `helm.sh/resource-policy: keep` annotation from step 2 in place — removing it re-exposes the live database PVC to accidental pruning on a future helm upgrade/uninstall.

**Automatable?** yes (bucket: operator or api-auth; PVC volume binding observation; must run alone, last in its bucket, for the same DB-swap reason as the manual run).

---

### game-storage-class

**Preconditions:** An alternative StorageClass other than the cluster default must be available (e.g., `fast-nvme`, `slow-hdd`, or a custom class). If only one StorageClass exists, this test is deferred.

**Resources created:** audit018-gameserver-storageclass-test (GameServer that uses the configured storage class).

**Steps:**
1. List available StorageClasses: `kubectl get storageclass` and choose one to test (e.g., `fast-nvme` if present, or create a test class).
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'operator.gameDataStorage.storageClassName=fast-nvme'` (or your chosen class) and wait for operator to reconcile.
3. Create a test GameServer (using real line breaks below — the single-line form above would send a literal backslash-n string to the shell, not a heredoc):
```sh
kubectl apply -f - <<'EOF'
apiVersion: gameplane.local/v1alpha1
kind: GameServer
metadata:
  name: audit018-storage-class-test
  namespace: gameplane-games
  labels:
    gameplane.io/audit: "018"
spec:
  templateRef:
    name: minecraft-java
EOF
```
4. Wait for the GameServer to provision its data volume and verify the PVC uses the configured StorageClass: `kubectl get pvc -n gameplane-games | grep audit018-storage-class-test` and check the CLASS column (expect `fast-nvme`).
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'operator.gameDataStorage.storageClassName='` (empty = use cluster default) and wait for operator to reconcile.
6. Create another test GameServer (e.g., `audit018-storage-default-test`) and verify its PVC uses the default StorageClass (or no specific class if cluster default is unset).

**Expected:** GameServer PVCs use the configured StorageClass when set; use the cluster default when empty.

**Cleanup:** Helm upgrade with storageClassName empty; delete test GameServers and their PVCs.

**Automatable?** no (blocked candidate: requires GameServer creation and PVC provisioning; alternative: verify operator code includes storageClassName field in GameServer PVC creation logic).

---

### crd-auto-apply-hook

**Preconditions:** An upgrade in progress or scheduled (this test is best run during a helm upgrade after the initial install).

**Resources created:** none (hook job is ephemeral; runs once per upgrade and is not retained).

**Steps:**
1. Ensure you have a baseline Gameplane install with an older CRD version.
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'crds.autoApply.enabled=true'` and watch the pre-upgrade hook job.
3. Observe the pre-upgrade hook job: `kubectl get job -n gameplane-system gameplane-crd-apply` (fixed name `<release>-crd-apply`; no `job-type` label exists). Note: `helm.sh/hook-delete-policy: before-hook-creation,hook-succeeded` deletes the Job immediately on success, so by the time step 2's `helm upgrade` returns, it is very likely already gone — start `kubectl get job -n gameplane-system gameplane-crd-apply --watch` in a second terminal just before running step 2 if you need to catch it live.
4. Verify the job completes successfully — run this in the second terminal, started just before step 2, since the Job is hook-deleted right after success: `kubectl wait --for=condition=complete job/gameplane-crd-apply -n gameplane-system --timeout=300s`.
5. Verify CRDs are updated to the new schema: `kubectl get crd gameservers.gameplane.local -o jsonpath='{.spec.versions[0].name}'` and compare with chart's CRD definition.
6. Revert (required — this cluster's baseline is `crds.autoApply.enabled=false`, per kubelab-baseline.md, and step 2 persisted `true` via `--reuse-values`): `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'crds.autoApply.enabled=false'`.

**Expected:** Pre-upgrade hook job runs `kubectl apply --server-side` on CRDs when enabled. CRDs are updated to the new schema. Job completes successfully and does not block the upgrade.

**Cleanup:** Helm upgrade with `crds.autoApply.enabled=false` (this cluster's baseline; see step 6); the hook Job itself is ephemeral and self-deletes.

**Automatable?** yes (bucket: upgrade; job existence and completion observation; requires an upgrade scenario).

---

### image-registry-override

**Preconditions:** An alternative container registry (e.g., a private mirror, gcr.io, or docker.io) must be accessible from the cluster and contain Gameplane images.

**Resources created:** none (only changes image references in deployments).

**Steps:**
0. Record the current image registry before the test: `helm get values gameplane -n gameplane-system -a -o json | jq -r '.image.registry' > /tmp/audit018-imageregistry-before.txt`. `-a` includes the chart default, so this is the effective registry: `gameplane-test` on the pre-audit baseline, `ghcr.io/valgulnecron/gameplane` once an RC is deployed.
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'image.registry=my-private-registry.example.com/gameplane'` and wait for pods to restart with the new image.
2. Verify operator pod is running with the new image: `kubectl get pod -n gameplane-system -l app.kubernetes.io/name=gameplane-operator -o jsonpath='{.items[0].spec.containers[0].image}'` (expect `my-private-registry.example.com/gameplane/operator:...`).
3. Verify API pod is running with the new image: `kubectl get pod -n gameplane-system -l app.kubernetes.io/name=gameplane-api -o jsonpath='{.items[0].spec.containers[0].image}'` (expect `my-private-registry.example.com/gameplane/api:...`).
4. Revert to the registry recorded in step 0, not a hardcoded value: `REG=$(cat /tmp/audit018-imageregistry-before.txt) && helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set "image.registry=$REG"` and wait for pods to restart. Restoring the pre-audit `gameplane-test` during an RC round would put kubelab back on the private side-loaded images.

**Expected:** Pods are pulled from the specified registry when image.registry is set. Pulling fails if the registry is not accessible (blocked candidate).

**Cleanup:** Restore `image.registry` to the value recorded in step 0 (step 4). Do **not** hardcode `gameplane-test` (kubelab-baseline.md's pre-audit value): once an RC is deployed, the recorded value is the RC's registry.

**Automatable?** no (blocked candidate: requires access to alternative registry; alternative: verify Deployment image field is updated correctly in the generated manifests without actually pulling).

---

### image-tag-override

**Preconditions:** None (tag is a simple string parameter).

**Resources created:** none (only changes image references).

**Steps:**
0. Record the current image tag before the test: `kubectl get deployment -n gameplane-system gameplane-operator -o jsonpath='{.spec.template.spec.containers[0].image}' | grep -o '[^:]*$' > /tmp/audit018-imagetag-before.txt`.
1. Get the current image tag: `kubectl get deployment -n gameplane-system gameplane-operator -o jsonpath='{.spec.template.spec.containers[0].image}' | grep -o '[^:]*$'` (extract tag from image reference).
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'image.tag=latest'` and wait for pods to restart.
3. Verify operator pod is running with the new tag: `kubectl get pod -n gameplane-system -l app.kubernetes.io/name=gameplane-operator -o jsonpath='{.items[0].spec.containers[0].image}'` (expect tag `latest`).
4. Verify API pod is running with the new tag: `kubectl get pod -n gameplane-system -l app.kubernetes.io/name=gameplane-api -o jsonpath='{.items[0].spec.containers[0].image}'` (expect tag `latest`).
5. Revert to the pre-test image tag recorded in step 0: `TAG=$(cat /tmp/audit018-imagetag-before.txt) && helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set "image.tag=$TAG"` and wait for pods to restart.

**Expected:** Pods are pulled with the specified tag. Image pulling may fail if the tag does not exist in the registry.

**Cleanup:** Restore `image.tag` to the value recorded in step 0 (step 5). Do **not** hardcode `016` (kubelab-baseline.md's pre-audit private tag): once an RC is deployed, the recorded value is the RC's tag.

**Automatable?** yes (bucket: api-auth or web e2e; Deployment image field observation; does not require actual pulling if image already exists locally).

---

### operator-leader-election

**Preconditions:** None (leader election is a configuration flag; meaningful only with multiple operator replicas, but can be toggled with single replica).

**Resources created:** none (only changes operator Deployment config).

**Steps:**
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'operator.leaderElect=false'` and wait for operator pod to restart.
2. Verify operator pod environment: `kubectl get deployment -n gameplane-system gameplane-operator -o yaml | grep -i 'leader\|LEADER'` (expect no leader election flags or env vars when disabled).
3. Access operator logs and verify no lease-related messages: `kubectl logs -n gameplane-system -l app.kubernetes.io/name=gameplane-operator | grep -i lease` (expect minimal/no output when disabled).
4. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'operator.leaderElect=true'` and wait for operator to restart.
5. Verify operator logs now include lease-related messages: `kubectl logs -n gameplane-system -l app.kubernetes.io/name=gameplane-operator | grep -i lease` (expect output indicating leader election is active).

**Expected:** Leader election is disabled when flag is false; is enabled when true. Operator logs reflect the state.

**Cleanup:** Helm upgrade with leaderElect=true (default for HA).

**Automatable?** yes (bucket: operator; Deployment config observation and log grep).

