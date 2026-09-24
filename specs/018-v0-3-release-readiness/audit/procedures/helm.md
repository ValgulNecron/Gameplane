# Procedures: HELM

Shared conventions: [conventions.md](conventions.md).

### oidc-authentication

**Preconditions:** External OIDC IdP available (e.g., Keycloak, Auth0, or similar; kubelab cluster must reach it). Credentials and IdP metadata configured but not yet enabled in Gameplane.

**Resources created:** none.

**Steps:**
1. Login as audit018-admin to the dashboard. (Login cost: 1)
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set api.oidc.enabled=true --set api.oidc.issuer=<issuer> --set api.oidc.clientID=<clientID> --set api.oidc.redirectURL=<redirectURL>` and wait for rollout.
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

**Resources created:** audit018-syslog-bridge (Deployment + Service if creating receiver).

**Steps:**
1. Deploy a test syslog receiver in gameplane-system namespace or use an existing one: `kubectl run audit018-syslog-receiver --image=nicolaka/netcat --command -- nc -l -u 0.0.0.0 514 &` (runs in background; for TCP, adjust command).
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.audit.webhook.syslogBridge.enabled=true' --set 'api.audit.webhook.syslogBridge.syslog.addr=audit018-syslog-receiver.gameplane-system:514' --set 'api.audit.webhook.syslogBridge.syslog.network=udp'` and wait for syslog-bridge pod to start.
3. Perform an audit event (e.g., login). (Login cost: 1)
4. Wait 2 seconds and check syslog receiver pod logs for RFC 5424 formatted audit event.
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.audit.webhook.syslogBridge.enabled=false'` and wait for syslog-bridge pod termination.
6. Kill the test receiver: `kubectl delete pod audit018-syslog-receiver -n gameplane-system`.

**Expected:** syslog-bridge Deployment exists when enabled, is removed when disabled. Syslog receiver pod sees RFC 5424 formatted events when bridge is running and a user action triggers audit logging.

**Cleanup:** Helm upgrade with syslogBridge.enabled=false; delete receiver pod.

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
1. Get current API pod name: `kubectl get pod -n gameplane-system -l app=gameplane-api -o jsonpath='{.items[0].metadata.name}'`.
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

**Resources created:** audit018-telemetry-receiver (Deployment + Service).

**Steps:**
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.telemetry.receiver.enabled=true'` and wait for receiver pod to start.
2. Verify receiver pod is running: `kubectl get pod -n gameplane-system -l app=gameplane-telemetry-receiver`.
3. Verify receiver Service is created: `kubectl get svc -n gameplane-system -l app=gameplane-telemetry-receiver`.
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

**Resources created:** audit018-mcp-server (Deployment).

**Steps:**
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'mcpServer.enabled=true'` and wait for MCP server pod to start.
2. Verify MCP server pod is running: `kubectl get pod -n gameplane-system -l app=gameplane-mcp-server`.
3. Verify the pod is ready and logs show JSON-RPC server initialized: `kubectl logs -n gameplane-system -l app=gameplane-mcp-server | grep -i 'serving\|ready\|json-rpc'`.
4. Test MCP server over stdin/stdout: `kubectl exec -it -n gameplane-system <mcp-pod> -- gameplane-mcp-server --serve` and send a JSON-RPC call (e.g., `{"jsonrpc":"2.0","method":"list_resources","id":1}`); observe response.
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'mcpServer.enabled=false'` and wait for pod termination.

**Expected:** MCP server Deployment exists and pod runs when enabled; Deployment is removed when disabled. MCP server is read-only and responds to JSON-RPC calls over stdin/stdout.

**Cleanup:** Helm upgrade with mcpServer.enabled=false.

**Automatable?** yes (bucket: api-mods; pod existence and JSON-RPC handshake test).

---

### network-policies-enforcement

**Preconditions:** None (policies are chart-defined, no external resources needed).

**Resources created:** audit018-network-policies (multiple NetworkPolicy objects in gamesNamespace if created for testing).

**Steps:**
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'networkPolicies.enabled=true'` and wait for Helm to apply.
2. Verify NetworkPolicy objects exist in gamesNamespace: `kubectl get networkpolicies -n gameplane-games | wc -l` (expect >0, at least the default-deny and allow-kubelet policies).
3. Verify label on gamesNamespace is set for network policies: `kubectl get namespace gameplane-games -o jsonpath='{.metadata.labels}' | grep -i network` (may or may not be present depending on Helm chart; verify by checking pod constraints if possible).
4. Create a test GameServer with a game pod and verify it has the network policy applied: check `kubectl get networkpolicies -n gameplane-games -o yaml` to see selectors.
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'networkPolicies.enabled=false'` and verify NetworkPolicy objects are removed (or namespace label is removed).

**Expected:** NetworkPolicy objects are created in gamesNamespace when enabled; are removed when disabled. Policies enforce ingress/egress rules as defined in the chart.

**Cleanup:** Helm upgrade with networkPolicies.enabled=false.

**Automatable?** yes (bucket: api-rbac or multicluster; NetworkPolicy resource observation).

---

### pod-security-enforcement

**Preconditions:** None (label applied to gamesNamespace).

**Resources created:** none (only namespace label is set).

**Steps:**
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'podSecurity.enforceRestricted=true'` and wait for Helm to apply.
2. Verify pod security label is set on gamesNamespace: `kubectl get namespace gameplane-games -o jsonpath='{.metadata.labels}' | grep 'pod-security'`.
3. Expected label: `pod-security.kubernetes.io/enforce=restricted`.
4. Attempt to create a pod that violates the restricted policy (e.g., runs as root or with privileged: true) in gamesNamespace and verify it is denied by the pod security policy: `kubectl run audit018-privileged-test --image=busybox --overrides='{"spec":{"containers":[{"name":"test","image":"busybox","securityContext":{"privileged":true}}]}}'` and expect failure.
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
1. Ensure networkPolicies.enabled=true (run network-policies-enforcement first if needed).
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'networkPolicies.gameEgress.enabled=true'` and wait for Helm to apply.
3. Verify NetworkPolicy for game egress exists: `kubectl get networkpolicies -n gameplane-games | grep -i egress` or inspect the policy YAML to confirm it allows only TCP 80/443 and blocks RFC1918 ranges.
4. Create a test GameServer in gameplane-games and start it, then attempt to download an asset from the public internet (should succeed) and from an internal service (should fail): simulate by running a curl command inside the game pod.
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'networkPolicies.gameEgress.enabled=false'`.
6. Verify game pod can now reach internal services.

**Expected:** Game pods are restricted to TCP 80/443 egress to non-RFC1918 ranges when enabled; are unrestricted when disabled.

**Cleanup:** Helm upgrade with gameEgress.enabled=false; delete test GameServer.

**Automatable?** no (blocked candidate: requires actual game pod startup and network testing; alternative: verify NetworkPolicy object definition matches expected egress rules).

---

### game-ingress-policies

**Preconditions:** None (networkPolicies.enabled should be true).

**Resources created:** none (test may create audit018-game-ingress-test GameServer).

**Steps:**
1. Ensure networkPolicies.enabled=true.
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'networkPolicies.gameIngress.enabled=true'` and wait for Helm to apply.
3. Verify NetworkPolicy for game ingress exists: `kubectl get networkpolicies -n gameplane-games | grep game-ingress`.
4. Create a test GameServer with a game pod that listens on port 25565 (Minecraft default), advertises it (Advertise: true in template), and verify the operator creates a per-server NetworkPolicy allowing ingress from 0.0.0.0/0 on that port.
5. Attempt to connect to the game server port from an external client (e.g., from the audit devbox); expect success if policy is enabled.
6. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'networkPolicies.gameIngress.enabled=false'`.
7. Verify the per-server NetworkPolicy is removed by the operator on the next reconciliation.

**Expected:** Per-GameServer ingress NetworkPolicy allows advertised ports from configured CIDRs when enabled; is removed when disabled.

**Cleanup:** Helm upgrade with gameIngress.enabled=false; delete test GameServer.

**Automatable?** no (blocked candidate: requires game pod startup and external connectivity test; alternative: verify NetworkPolicy object definition).

---

### service-monitors

**Preconditions:** Prometheus Operator CRDs (ServiceMonitor, PodMonitor) must be installed in the cluster. If not available, this is a blocked candidate.

**Resources created:** audit018-servicemonitor-operator, audit018-podmonitor-agents (if applicable).

**Steps:**
1. Verify Prometheus Operator is installed: `kubectl get crd servicemonitors.monitoring.coreos.com` (expect success; if not found, skip to blocked candidate note).
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'serviceMonitors.enabled=true'` and wait for Helm to apply.
3. Verify ServiceMonitor for operator is created: `kubectl get servicemonitor -n gameplane-system -l app=gameplane-operator`.
4. Verify ServiceMonitor for API is created: `kubectl get servicemonitor -n gameplane-system -l app=gameplane-api`.
5. Verify PodMonitor for agent sidecars is created (if included): `kubectl get podmonitor -n gameplane-system | grep agent`.
6. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'serviceMonitors.enabled=false'` and wait for Helm to remove ServiceMonitor/PodMonitor objects.

**Expected:** ServiceMonitor and PodMonitor objects are created in gameplane-system when enabled; are removed when disabled. Objects have labels matching Prometheus scrape selectors.

**Cleanup:** Helm upgrade with serviceMonitors.enabled=false.

**Automatable?** no (blocked candidate: needs Prometheus Operator CRDs; alternative: verify object creation in a cluster with CRDs present).

---

### prometheus-rules

**Preconditions:** Prometheus Operator CRDs (PrometheusRule) must be installed in the cluster.

**Resources created:** audit018-prometheusrule.

**Steps:**
1. Verify Prometheus Operator is installed: `kubectl get crd prometheusrules.monitoring.coreos.com` (expect success).
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'prometheusRules.enabled=true'` and wait for Helm to apply.
3. Verify PrometheusRule object is created: `kubectl get prometheusrule -n gameplane-system -l app=gameplane-operator`.
4. Inspect the rule YAML and verify it includes alert rules for operator metrics: `kubectl get prometheusrule -n gameplane-system -l app=gameplane-operator -o yaml | grep -i alert`.
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'prometheusRules.enabled=false'` and verify PrometheusRule is removed.

**Expected:** PrometheusRule object is created when enabled; is removed when disabled. Rule includes operator-specific alerts and Prometheus scrape configuration references.

**Cleanup:** Helm upgrade with prometheusRules.enabled=false.

**Automatable?** no (blocked candidate: needs Prometheus Operator CRDs; alternative: verify PrometheusRule resource definition on a Prometheus Operator-equipped cluster).

---

### grafana-dashboards

**Preconditions:** Grafana with sidecar dashboard loader (grafana-sidecar) must be installed in the cluster. This is typically part of kube-prometheus-stack.

**Resources created:** audit018-grafana-dashboard (ConfigMap).

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

**Preconditions:** None (module source is created in the operator namespace by default).

**Resources created:** audit018-default-modulesource (ModuleSource custom resource in operator namespace).

**Steps:**
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'defaultModuleSource.enabled=true'` and wait for Helm to apply.
2. Verify ModuleSource object is created: `kubectl get modulesource -n gameplane-system -l name=default` (or check for `default` ModuleSource).
3. Verify operator indexes the module source: `kubectl get modulesource default -n gameplane-system -o jsonpath='{.status.conditions[?(@.type=="Ready")].reason}'` (expect `Ready` or `Indexed`).
4. Access the dashboard Modules page and verify game templates are listed (e.g., Minecraft, Terraria, etc.).
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'defaultModuleSource.enabled=false'` and verify ModuleSource object is removed (or marked as disabled).
6. Refresh the dashboard Modules page; verify templates are no longer available.

**Expected:** Default ModuleSource object is created when enabled; is removed when disabled. Operator indexes the source and dashboard Modules page populates/empties accordingly.

**Cleanup:** Helm upgrade with defaultModuleSource.enabled=false.

**Automatable?** yes (bucket: api-mods; ModuleSource resource observation and dashboard API query).

---

### module-signature-verification

**Preconditions:** defaultModuleSource.type=oci (OCI module source). Module signature verification is only applicable to OCI sources, not git sources.

**Resources created:** none (verification is configured in ModuleSource spec).

**Steps:**
1. Ensure defaultModuleSource.type=oci (default value in values.yaml).
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'defaultModuleSource.oidc.verify.enabled=true'` (note: the value should be `oci.verify.enabled`, not `oidc.verify.enabled`; verify field name in values.yaml).
3. Wait for operator to reconcile ModuleSource.
4. Verify the ModuleSource spec includes cosign verification settings: `kubectl get modulesource default -n gameplane-system -o jsonpath='{.spec.verify}'`.
5. Attempt to pull a module and verify operator validates the cosign signature before indexing.
6. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'defaultModuleSource.oidc.verify.enabled=false'`.
7. Verify unsigned/tampered modules are now accepted (verification disabled).

**Expected:** ModuleSource includes cosign public key and verification flag when enabled; verification is skipped when disabled. Tampered modules are rejected when verification is on; accepted when off.

**Cleanup:** Helm upgrade with module verification disabled.

**Automatable?** no (blocked candidate: requires pulling and validating actual module bundles; alternative: verify ModuleSource spec includes verify settings).

---

### upload-module-source

**Preconditions:** None (upload source is created by default if enabled).

**Resources created:** audit018-upload-modulesource (ModuleSource custom resource).

**Steps:**
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'uploadModuleSource.enabled=true'` and wait for Helm to apply.
2. Verify upload ModuleSource object is created: `kubectl get modulesource uploads -n gameplane-system` (or check for object with name matching `uploadModuleSource.name` from values).
3. Access the dashboard Modules page and look for an "Upload" option or "Uploads" catalog section.
4. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'uploadModuleSource.enabled=false'` and verify ModuleSource object is removed.
5. Refresh dashboard Modules page; verify upload capability is no longer available.

**Expected:** Upload ModuleSource is created when enabled; is removed when disabled. Dashboard Modules page includes upload UI when source is present.

**Cleanup:** Helm upgrade with uploadModuleSource.enabled=false.

**Automatable?** yes (bucket: api-mods; ModuleSource resource observation).

---

### packet-capture-sidecar

**Preconditions:** None (capture is opt-in per GameServer; this test enables the cluster-wide feature).

**Resources created:** none (sidecars are injected into GameServers at creation time, not pre-created).

**Steps:**
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'capture.enabled=true'` and wait for Helm to apply.
2. Create a test GameServer with capture opt-in: `kubectl apply -f - <<'EOF'\napiVersion: gameplane.io/v1\nkind: GameServer\nmetadata:\n  name: audit018-capture-test\n  namespace: gameplane-games\nspec:\n  template: minecraft-java\n  capture:\n    enabled: true\nEOF`.
3. Wait for the GameServer pod to start and verify the capture sidecar is injected: `kubectl get pod -n gameplane-games -l gameserver=audit018-capture-test -o yaml | grep -A5 'capture-sidecar'`.
4. Verify capture is accessible via the API: POST to `$GP/api/v1/gameservers/audit018-capture-test/captures/start` with appropriate parameters and expect success.
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'capture.enabled=false'` and wait for Helm to apply.
6. Create another test GameServer (e.g., `audit018-no-capture-test`) with capture opt-in and verify the sidecar is NOT injected (operator ignores capture requests when cluster feature is disabled).

**Expected:** Capture sidecar is injected into GameServers when cluster capture is enabled and GameServer requests it; is not injected when cluster feature is disabled. Capture API endpoints are available when enabled.

**Cleanup:** Helm upgrade with capture.enabled=false; delete test GameServers.

**Automatable?** no (blocked candidate: requires GameServer creation and pod injection verification; alternative: verify operator code includes capture sidecar injection logic when feature is enabled).

---

### web-dashboard-ui

**Preconditions:** None (web is enabled by default).

**Resources created:** none (web pod is already running if enabled).

**Steps:**
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'web.enabled=false'` and wait for web Deployment to terminate.
2. Verify web pod is removed: `kubectl get pod -n gameplane-system -l app=gameplane-web` (expect no results).
3. Attempt to reach the dashboard at `$GP/` and expect failure (502 Bad Gateway or similar, depending on ingress configuration).
4. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'web.enabled=true'` and wait for web pod to start.
5. Verify web pod is running: `kubectl get pod -n gameplane-system -l app=gameplane-web` (expect running pod).
6. Attempt to reach the dashboard at `$GP/` and expect success (login page or main dashboard).

**Expected:** Web pod (nginx serving the SPA) is deployed when web.enabled=true; is removed when false. Dashboard is accessible when web pod is running; returns error when removed.

**Cleanup:** Helm upgrade with web.enabled=true.

**Automatable?** yes (bucket: web e2e or integration; pod/svc observation and HTTP endpoint test).

---

### ingress-configuration

**Preconditions:** Ingress controller (nginx-ingress, Traefik, etc.) must be installed in the cluster. If not, ingress objects will be created but are inactive.

**Resources created:** audit018-gameplane-ingress (Ingress object; reuses existing ingress if already present).

**Steps:**
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'ingress.enabled=true'` and wait for Helm to apply.
2. Verify Ingress object is created: `kubectl get ingress -n gameplane-system -l app=gameplane`.
3. Verify ingress routing rules: `kubectl get ingress -n gameplane-system -o yaml | grep -A5 'host: gameplane.local'` (or your configured host).
4. Attempt to reach the dashboard via the configured host (e.g., `https://gameplane.local/`); expect success if ingress controller is present and routes are working.
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'ingress.enabled=false'` and verify Ingress object is removed.
6. Attempt to reach the dashboard via the same host; expect failure (no route, or 404).

**Expected:** Ingress object is created when enabled; is removed when disabled. Routes are active and reachable when ingress is enabled (if ingress controller is present).

**Cleanup:** Helm upgrade with ingress.enabled=false.

**Automatable?** yes (bucket: api-auth or web e2e; Ingress object observation and HTTP request test).

---

### existing-storage-claim

**Preconditions:** A pre-existing PersistentVolumeClaim (PVC) named `audit018-api-storage` must exist in the gameplane-system namespace with at least 2Gi capacity and RWO access mode.

**Resources created:** none (uses pre-existing PVC).

**Steps:**
1. Create a test PVC: `kubectl apply -f - <<'EOF'\napiVersion: v1\nkind: PersistentVolumeClaim\nmetadata:\n  name: audit018-api-storage\n  namespace: gameplane-system\nspec:\n  accessModes:\n    - ReadWriteOnce\n  storageClassName: standard\n  resources:\n    requests:\n      storage: 2Gi\nEOF` and wait for it to bind (may be immediate if dynamic provisioning is available).
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.storage.existingClaim=audit018-api-storage'` and wait for API pod to restart.
3. Verify API pod is running and mounts the PVC: `kubectl get pod -n gameplane-system -l app=gameplane-api -o jsonpath='{.items[0].spec.volumes[?(@.name=="data")].persistentVolumeClaim.claimName}'` (expect `audit018-api-storage`).
4. Verify no new PVC is created: `kubectl get pvc -n gameplane-system | grep gameplane-api-data` (the default PVC name when existingClaim is not set; expect no results).
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'api.storage.existingClaim='` and wait for API pod to restart.
6. Verify API pod now mounts the auto-created PVC: `kubectl get pvc -n gameplane-system | grep gameplane-api-data` (expect a new PVC to be created by Helm).

**Expected:** When existingClaim is set, the API pod mounts that PVC and no new PVC is created. When empty (default), Helm creates a PVC for the API.

**Cleanup:** Helm upgrade with existingClaim empty; delete the test PVC `audit018-api-storage`.

**Automatable?** yes (bucket: operator or api-auth; PVC volume binding observation).

---

### game-storage-class

**Preconditions:** An alternative StorageClass other than the cluster default must be available (e.g., `fast-nvme`, `slow-hdd`, or a custom class). If only one StorageClass exists, this test is deferred.

**Resources created:** audit018-gameserver-storageclass-test (GameServer that uses the configured storage class).

**Steps:**
1. List available StorageClasses: `kubectl get storageclass` and choose one to test (e.g., `fast-nvme` if present, or create a test class).
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'operator.gameDataStorage.storageClassName=fast-nvme'` (or your chosen class) and wait for operator to reconcile.
3. Create a test GameServer: `kubectl apply -f - <<'EOF'\napiVersion: gameplane.io/v1\nkind: GameServer\nmetadata:\n  name: audit018-storage-class-test\n  namespace: gameplane-games\nspec:\n  template: minecraft-java\nEOF`.
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
3. Observe the pre-upgrade hook job running: `kubectl get job -n gameplane-system -l job-type=crd-apply` (or similar label; check Helm chart for exact label).
4. Verify the job completes successfully: `kubectl wait --for=condition=complete job/<job-name> -n gameplane-system --timeout=300s`.
5. Verify CRDs are updated to the new schema: `kubectl get crd gameserver.gameplane.io -o jsonpath='{.spec.versions[0].name}'` and compare with chart's CRD definition.
6. Revert: No specific revert needed; the hook is idempotent. A subsequent upgrade with crds.autoApply.enabled=false will skip the hook.

**Expected:** Pre-upgrade hook job runs `kubectl apply --server-side` on CRDs when enabled. CRDs are updated to the new schema. Job completes successfully and does not block the upgrade.

**Cleanup:** No cleanup required (hook is ephemeral).

**Automatable?** yes (bucket: upgrade; job existence and completion observation; requires an upgrade scenario).

---

### image-registry-override

**Preconditions:** An alternative container registry (e.g., a private mirror, gcr.io, or docker.io) must be accessible from the cluster and contain Gameplane images.

**Resources created:** none (only changes image references in deployments).

**Steps:**
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'image.registry=my-private-registry.example.com/gameplane'` and wait for pods to restart with the new image.
2. Verify operator pod is running with the new image: `kubectl get pod -n gameplane-system -l app=gameplane-operator -o jsonpath='{.items[0].spec.containers[0].image}'` (expect `my-private-registry.example.com/gameplane/operator:...`).
3. Verify API pod is running with the new image: `kubectl get pod -n gameplane-system -l app=gameplane-api -o jsonpath='{.items[0].spec.containers[0].image}'` (expect `my-private-registry.example.com/gameplane/api:...`).
4. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'image.registry=ghcr.io/valgulnecron/gameplane'` (original default) and wait for pods to restart.

**Expected:** Pods are pulled from the specified registry when image.registry is set. Pulling fails if the registry is not accessible (blocked candidate).

**Cleanup:** Helm upgrade with original registry.

**Automatable?** no (blocked candidate: requires access to alternative registry; alternative: verify Deployment image field is updated correctly in the generated manifests without actually pulling).

---

### image-tag-override

**Preconditions:** None (tag is a simple string parameter).

**Resources created:** none (only changes image references).

**Steps:**
1. Get the current image tag: `kubectl get deployment -n gameplane-system gameplane-operator -o jsonpath='{.spec.template.spec.containers[0].image}' | grep -o '[^:]*$'` (extract tag from image reference).
2. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'image.tag=latest'` and wait for pods to restart.
3. Verify operator pod is running with the new tag: `kubectl get pod -n gameplane-system -l app=gameplane-operator -o jsonpath='{.items[0].spec.containers[0].image}'` (expect tag `latest`).
4. Verify API pod is running with the new tag: `kubectl get pod -n gameplane-system -l app=gameplane-api -o jsonpath='{.items[0].spec.containers[0].image}'` (expect tag `latest`).
5. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'image.tag=v0.2.0-beta.8'` (or current version) and wait for pods to restart.

**Expected:** Pods are pulled with the specified tag. Image pulling may fail if the tag does not exist in the registry.

**Cleanup:** Helm upgrade with original tag.

**Automatable?** yes (bucket: api-auth or web e2e; Deployment image field observation; does not require actual pulling if image already exists locally).

---

### operator-leader-election

**Preconditions:** None (leader election is a configuration flag; meaningful only with multiple operator replicas, but can be toggled with single replica).

**Resources created:** none (only changes operator Deployment config).

**Steps:**
1. Run: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'operator.leaderElect=false'` and wait for operator pod to restart.
2. Verify operator pod environment: `kubectl get deployment -n gameplane-system gameplane-operator -o yaml | grep -i 'leader\|LEADER'` (expect no leader election flags or env vars when disabled).
3. Access operator logs and verify no lease-related messages: `kubectl logs -n gameplane-system -l app=gameplane-operator | grep -i lease` (expect minimal/no output when disabled).
4. Revert: `helm upgrade gameplane charts/gameplane -n gameplane-system --reuse-values --set 'operator.leaderElect=true'` and wait for operator to restart.
5. Verify operator logs now include lease-related messages: `kubectl logs -n gameplane-system -l app=gameplane-operator | grep -i lease` (expect output indicating leader election is active).

**Expected:** Leader election is disabled when flag is false; is enabled when true. Operator logs reflect the state.

**Cleanup:** Helm upgrade with leaderElect=true (default for HA).

**Automatable?** yes (bucket: operator; Deployment config observation and log grep).

