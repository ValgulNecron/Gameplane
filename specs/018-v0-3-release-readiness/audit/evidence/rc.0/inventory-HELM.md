# Draft inventory rows: HELM (rc.0)

Links are relative to audit/inventory.md.

| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-HELM-001 | Enable OIDC authentication | api/ | charts/gameplane/values.yaml:240 | [p](procedures/helm.md#oidc-authentication) | TBD | untested | blocked candidate: needs external OIDC IdP; alternative: test with mock IdP stub | [e](INV-HELM-001/) | | |
| INV-HELM-002 | Configure audit webhook sink | api/ | charts/gameplane/values.yaml:160 | [p](procedures/helm.md#audit-webhook-sink) | TBD | untested | | [e](INV-HELM-002/) | | |
| INV-HELM-003 | Enable syslog bridge for audit events | audit-syslog-bridge/ | charts/gameplane/values.yaml:183 | [p](procedures/helm.md#syslog-bridge-audit) | TBD | untested | | [e](INV-HELM-003/) | | |
| INV-HELM-004 | Configure S3 audit sink | api/ | charts/gameplane/values.yaml:202 | [p](procedures/helm.md#s3-audit-sink) | TBD | untested | blocked candidate: needs S3-compatible endpoint; alternative: mock MinIO if available on kubelab | [e](INV-HELM-004/) | | |
| INV-HELM-005 | Enable audit event stdout logging | api/ | charts/gameplane/values.yaml:153 | [p](procedures/helm.md#audit-stdout-logging) | TBD | untested | | [e](INV-HELM-005/) | | |
| INV-HELM-006 | Configure telemetry collection endpoint | api/ | charts/gameplane/values.yaml:222 | [p](procedures/helm.md#telemetry-collection) | TBD | untested | blocked candidate: telemetry sent daily; requires deterministic triggers or extended wait | [e](INV-HELM-006/) | | |
| INV-HELM-007 | Enable bundled telemetry receiver | telemetry-receiver/ | charts/gameplane/values.yaml:235 | [p](procedures/helm.md#telemetry-receiver-bundled) | TBD | untested | | [e](INV-HELM-007/) | | |
| INV-HELM-008 | Enable cluster operations feature | api/ | charts/gameplane/values.yaml:392 | [p](procedures/helm.md#cluster-operations-feature) | TBD | untested | | [e](INV-HELM-008/) | | |
| INV-HELM-009 | Enable MCP (Model Context Protocol) server | mcp-server/ | charts/gameplane/values.yaml:400 | [p](procedures/helm.md#mcp-server-deployment) | TBD | untested | | [e](INV-HELM-009/) | | |
| INV-HELM-010 | Enable default-deny network policies | operator/ | charts/gameplane/values.yaml:316 | [p](procedures/helm.md#network-policies-enforcement) | TBD | untested | | [e](INV-HELM-010/) | | |
| INV-HELM-011 | Enforce restricted pod security policy | operator/ | charts/gameplane/values.yaml:440 | [p](procedures/helm.md#pod-security-enforcement) | TBD | untested | | [e](INV-HELM-011/) | | |
| INV-HELM-012 | Enable game pod egress to public internet | operator/ | charts/gameplane/values.yaml:343 | [p](procedures/helm.md#game-egress-policies) | TBD | untested | blocked candidate: requires GameServer pod startup for network testing | [e](INV-HELM-012/) | | |
| INV-HELM-013 | Enable per-GameServer ingress network policies | operator/ | charts/gameplane/values.yaml:371 | [p](procedures/helm.md#game-ingress-policies) | TBD | untested | blocked candidate: requires GameServer pod startup and external connectivity | [e](INV-HELM-013/) | | |
| INV-HELM-014 | Create Prometheus ServiceMonitor objects | charts/gameplane/ | charts/gameplane/values.yaml:418 | [p](procedures/helm.md#service-monitors) | TBD | untested | blocked candidate: needs Prometheus Operator CRDs (ServiceMonitor); alternative: verify on cluster with CRDs | [e](INV-HELM-014/) | | |
| INV-HELM-015 | Create Prometheus alert rules | charts/gameplane/ | charts/gameplane/values.yaml:428 | [p](procedures/helm.md#prometheus-rules) | TBD | untested | blocked candidate: needs Prometheus Operator CRDs (PrometheusRule); alternative: verify on cluster with CRDs | [e](INV-HELM-015/) | | |
| INV-HELM-016 | Create Grafana dashboard ConfigMap | charts/gameplane/ | charts/gameplane/values.yaml:435 | [p](procedures/helm.md#grafana-dashboards) | TBD | untested | blocked candidate: needs Grafana with sidecar loader; alternative: verify on Grafana-equipped cluster | [e](INV-HELM-016/) | | |
| INV-HELM-017 | Enable default module source | operator/ | charts/gameplane/values.yaml:446 | [p](procedures/helm.md#default-module-source) | TBD | untested | | [e](INV-HELM-017/) | | |
| INV-HELM-018 | Enable module bundle signature verification | operator/ | charts/gameplane/values.yaml:507 | [p](procedures/helm.md#module-signature-verification) | TBD | untested | blocked candidate: requires pulling and validating module bundles; alternative: verify ModuleSource spec includes verify settings | [e](INV-HELM-018/) | | |
| INV-HELM-019 | Enable dashboard module upload source | operator/ | charts/gameplane/values.yaml:518 | [p](procedures/helm.md#upload-module-source) | TBD | untested | | [e](INV-HELM-019/) | | |
| INV-HELM-020 | Enable packet capture sidecar for GameServers | agent/ | charts/gameplane/values.yaml:527 | [p](procedures/helm.md#packet-capture-sidecar) | TBD | untested | blocked candidate: requires GameServer creation and sidecar injection verification | [e](INV-HELM-020/) | | |
| INV-HELM-021 | Enable/disable web dashboard UI | web/ | charts/gameplane/values.yaml:289 | [p](procedures/helm.md#web-dashboard-ui) | TBD | untested | | [e](INV-HELM-021/) | | |
| INV-HELM-022 | Configure dashboard ingress | charts/gameplane/ | charts/gameplane/values.yaml:296 | [p](procedures/helm.md#ingress-configuration) | TBD | untested | | [e](INV-HELM-022/) | | |
| INV-HELM-023 | Use existing storage claim for API database | api/ | charts/gameplane/values.yaml:142 | [p](procedures/helm.md#existing-storage-claim) | TBD | untested | | [e](INV-HELM-023/) | | |
| INV-HELM-024 | Configure default storage class for GameServer data | operator/ | charts/gameplane/values.yaml:109 | [p](procedures/helm.md#game-storage-class) | TBD | untested | blocked candidate: requires GameServer creation and PVC provisioning | [e](INV-HELM-024/) | | |
| INV-HELM-025 | Enable automatic CRD upgrade hook | charts/gameplane/ | charts/gameplane/values.yaml:34 | [p](procedures/helm.md#crd-auto-apply-hook) | TBD | untested | | [e](INV-HELM-025/) | | |
| INV-HELM-026 | Override container image registry | charts/gameplane/ | charts/gameplane/values.yaml:17 | [p](procedures/helm.md#image-registry-override) | TBD | untested | blocked candidate: needs alternative registry accessible from cluster | [e](INV-HELM-026/) | | |
| INV-HELM-027 | Override container image tag | charts/gameplane/ | charts/gameplane/values.yaml:20 | [p](procedures/helm.md#image-tag-override) | TBD | untested | | [e](INV-HELM-027/) | | |
| INV-HELM-028 | Enable operator leader election (HA) | operator/ | charts/gameplane/values.yaml:53 | [p](procedures/helm.md#operator-leader-election) | TBD | untested | | [e](INV-HELM-028/) | | |

Row count: 28
