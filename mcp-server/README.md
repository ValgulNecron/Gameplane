# mcp-server

A strictly **read-only** [Model Context Protocol](https://modelcontextprotocol.io/)
(MCP) server for Gameplane clusters. It lets an AI assistant read cluster
state — the 7 Gameplane CRDs, Pods, Events, and pod logs — and get a
suggested fix as plain text (YAML and/or `kubectl` commands) for a human
operator to review and run.

It **never** creates, updates, patches, deletes, or applies anything. That
is a hard invariant, enforced two ways:

1. **Structurally**: `Client` (`client.go`) exposes only List/Get-shaped
   methods (`ListCRD`, `GetCRD`, `ListPods`, `GetPod`, `ListEvents`,
   `PodLogs`). There is no mutating method for a tool to call, even by
   mistake.
2. **By registration**: every tool this server installs (`tools.go`) carries
   the MCP spec's `readOnlyHint: true` annotation, and `main_test.go` asserts
   the registered tool set never contains a mutating verb.

The corresponding Kubernetes RBAC (the chart's `mcpServer.enabled` toggle)
grants only `get`/`list`/`watch` — there is no `create`/`update`/`patch`/
`delete` in its ClusterRole either.

## Tools

| Tool | Reads |
|---|---|
| `list_gameplane_resources` | One Gameplane CRD kind (GameServer, GameTemplate, Backup, BackupSchedule, Restore, Module, ModuleSource), optionally scoped to a namespace and/or label selector |
| `get_gameplane_resource` | A single Gameplane CRD object by kind/namespace/name |
| `list_pods` | Core Pods in a namespace (or all namespaces), optionally by label selector |
| `get_pod` | A single Pod's spec and status |
| `list_events` | Core Events in a namespace (or all namespaces), optionally by field/label selector |
| `get_pod_logs` | A bounded tail (default 200 lines, capped at 5000) of a container's logs |
| `propose_fix` | Given a resource reference + a free-text symptom, returns suggested YAML/kubectl text, grounded in a best-effort read of the resource's current status. Never applies anything itself. |

## Transport: stdio only, no network port

This server speaks MCP (JSON-RPC 2.0) over stdio — it never opens an HTTP
or TCP port. It has two subcommands:

- `idle` (default, no args) — block until terminated. This is what the
  bundled Helm Deployment runs as its long-lived container process, purely
  so there is always a live container to `kubectl exec` into.
- `serve` — run one MCP stdio session against the real cluster. Point an
  MCP host's launcher (e.g. Claude Desktop / Claude Code's MCP config, or
  any other MCP-compatible client) at:

  ```sh
  kubectl exec -i deploy/gameplane-mcp-server -n gameplane-system -- /mcp-server serve
  ```

  Each `kubectl exec -i` spawns an independent, isolated `serve` process
  sharing the pod's ServiceAccount credentials and network access —
  concurrent sessions from different users/hosts don't interfere with each
  other or with the idle placeholder process.

## Run via the Helm chart

Set `mcpServer.enabled=true`. The chart deploys a Deployment (no Service —
there is no network port to expose), a dedicated ServiceAccount, and a
cluster-scoped, read-only ClusterRole/ClusterRoleBinding (`get`/`list`/
`watch` only, on the 7 Gameplane CRDs plus core Pods/Events, and `get` on
the `pods/log` subresource).

```yaml
mcpServer:
  enabled: true
  replicas: 1
  resources:
    requests: { cpu: 50m, memory: 32Mi }
    limits:   { cpu: 200m, memory: 128Mi }
```

## Run standalone (against a kubeconfig)

```sh
KUBECONFIG=~/.kube/config docker run --rm -i \
  -v ~/.kube/config:/kubeconfig:ro -e KUBECONFIG=/kubeconfig \
  ghcr.io/valgulnecron/gameplane/mcp-server:edge serve
```

`serve` builds its Kubernetes client via `ctrl.GetConfig()`, so it works
in-cluster (via the pod's ServiceAccount) or locally against `KUBECONFIG`/
`~/.kube/config`, same as any other controller-runtime-based Gameplane
component.
