# Review: mcp-server

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `mcp-server/specs.md`, `mcp-server/README.md`, `docs/security.md` (`## mcp-server (optional)`), `docs/architecture.md` (mcp-server bullet), `docs/install.md:165-166`, `charts/gameplane/templates/mcp-server.yaml`

## Scope reviewed

Read in full:
- `mcp-server/main.go`
- `mcp-server/tools.go`
- `mcp-server/fixadvice.go`
- `mcp-server/internal/kube/client.go`
- `mcp-server/internal/kube/client_test.go`
- `mcp-server/main_test.go`
- `mcp-server/specs.md`
- `mcp-server/README.md`
- `mcp-server/Dockerfile`
- `mcp-server/.testcoverage.yml`
- `mcp-server/.gitignore`

Read in part:
- `mcp-server/go.mod`: the direct `require` block only. The indirect list and `go.sum` were not read.

Read for cross-reference (only the relevant sections):
- `charts/gameplane/templates/mcp-server.yaml` (full)
- `operator/api/v1alpha1/*_types.go`: the `+kubebuilder:resource` markers of all 9 kinds
- `operator/internal/controller/gameserver_controller.go:1360-1395`: pod template labels
- `operator/internal/controller/backup_controller.go:240-257`: backup Job name and labels
- `operator/internal/controller/restore_controller.go:139`: restore Job name
- `operator/go.mod`: k8s and controller-runtime versions
- `deploy/kind/cluster.yaml:11`: dev cluster `apiServerAddress`
- `docs/security.md` lines 561-582, `docs/architecture.md` lines 306-316
- `SECURITY_AUDIT.md`, the "Controls reviewed with no gap found" MCP row (tracked)
- Upstream `github.com/modelcontextprotocol/go-sdk@v1.8.0` `mcp/server.go` `Server.Run` and `internal/jsonrpc2/conn.go` `Connection.wait`, to check how a client disconnect is handled

Compile check: `go build ./...` in `mcp-server/` succeeds (this downloaded go-sdk v1.8.0 into the module cache). No tests or linters were run.

## Method

- Checked every tool handler against the tool table in specs.md and README, and against the input schema descriptions the tools advertise to MCP clients (those descriptions are the contract an AI client acts on).
- Checked every concrete Kubernetes name the server emits or suggests (label keys, Job names, container names) against what the operator creates.
- Checked the read-only claims. `grep` for `.Create(`/`.Update(`/`.Delete(`/`.Patch(`/`.Apply(`/`UpdateStatus`/`DeleteCollection` across the module finds nothing. The chart ClusterRole grants only `get/list/watch` plus `get` on `pods/log`.
- Compared specs.md's factual claims (dependency versions, CRD count, file paths) with the tree.

## Observations (no finding)

- The server has no mutating call sites. `kube.Client` exports only `ListCRD`, `GetCRD`, `ListPods`, `GetPod`, `ListEvents` and `PodLogs`, plus the `Scheme` field. The ClusterRole (`mcp-server.yaml:22-37`) matches specs.md lines 99-110 exactly.
- The Deployment has no Service and no ports, runs as non-root with a read-only root filesystem and all capabilities dropped. `idle` performs no cluster access. This matches the README and specs.
- `runServe` treats a client EOF correctly: go-sdk v1.8.0 `Connection.wait` filters `io.EOF` from the read error, so a normal disconnect returns `nil` and the process exits 0. Context cancellation is filtered by `errors.Is(err, context.Canceled)`.
- `slog` writes to stderr by default, so log lines don't corrupt the stdio JSON-RPC stream on stdout.
- Tool errors are returned as the handler's `error`, and go-sdk turns them into `IsError` results. `TestToolsListAndGetHappyPath` exercises this for an unknown kind. Errors from `internal/kube` are wrapped with `%w`, and `errUnknownKind` can be matched with `errors.Is`.
- `CRDKinds` scopes (4 namespaced, 3 cluster-scoped) match the `+kubebuilder:resource:scope` markers of the seven kinds it lists.
- `Client.Scheme` / `NewScheme()` are built on every `serve` but never read by production code; only tests read them. The doc comment (`client.go:81-88`) says this is deliberate, so it is not reported as dead code.
- `knownKindsList()` iterates a map, so the order of kinds in the unknown-kind error message changes between calls. This is cosmetic.
- The specs.md coverage line "Measured against: package-level aggregate (not per-file)" (`specs.md:159`) is loosely worded: `.testcoverage.yml` gates `total: 70` with `package: 0`, which is a module-total gate. Too minor to report separately.

## Candidate findings

### C-mcp-server-01: get_pod_logs keeps the oldest part of the requested tail when the output exceeds 256 KiB, dropping the newest lines without saying so

- **Location**: `mcp-server/internal/kube/client.go:243-266` (specifically `:262`)
- **Category**: correctness
- **Suggested severity**: S3 (workaround: ask for a smaller `tailLines`)
- **Observation / repro**:
  1. Call `get_pod_logs` with `tailLines: 5000` (the documented cap) on a container whose log lines average about 100 bytes, for example a Minecraft server log.
  2. The API server returns the last 5000 lines in chronological order, about 500 KB.
  3. `io.ReadAll(io.LimitReader(stream, maxLogBytes))` with `maxLogBytes = 256 << 10` keeps the **first** 262,144 bytes of that stream, which are the oldest ~2,600 lines of the tail. The newest ~2,400 lines are discarded, and those include the lines closest to a crash, which is the case the `propose_fix` CrashLoop advice sends users to (`fixadvice.go:30-31`).
  4. The returned text has no marker saying it was truncated, so neither the AI client nor the operator can tell that the end of the log is missing.
- **Expected**: The tool description reads "Fetch a bounded tail of a Pod's container logs" (`tools.go:88`). specs.md:62 says "Log text (capped 256 KiB)". A byte cap on a tail request should keep the most recent bytes, or the tool should return fewer lines and say it truncated.
- **Actual**: When the cap applies, the result is the head of the tail, and the most recent log output is dropped without any indication.

### C-mcp-server-02: Tool schema examples and propose_fix advice use label keys the operator never sets

- **Location**: `mcp-server/tools.go:106`, `mcp-server/tools.go:149`, `mcp-server/fixadvice.go:74`
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. `list_pods` advertises `labelSelector` "e.g. 'gameplane.local/server=my-server'" (`tools.go:149`). The label key `gameplane.local/server` doesn't appear anywhere in `operator/` or `api/`. Game pods carry `app.kubernetes.io/name=gameplane-game`, `app.kubernetes.io/instance=<server>` and `gameplane.local/template=<template>` (`operator/internal/controller/gameserver_controller.go:1373-1377`). An AI client that follows the example gets an empty `PodList`.
  2. `list_gameplane_resources` advertises "e.g. 'gameplane.local/template=minecraft-java'" (`tools.go:106`). The operator only sets that label on the StatefulSet pod template (`gameserver_controller.go:1376`), not on GameServer (or any other) CR objects, which is what this tool lists. The example selector returns an empty list.
  3. The `propose_fix` backup advice suggests `kubectl get jobs -n <namespace> -l gameplane.local/backup=<name>` (`fixadvice.go:74`). Backup Jobs are created with `Name: b.Name` and no labels (`operator/internal/controller/backup_controller.go:245-251`), and `gameplane.local/backup` isn't set anywhere. The suggested command always prints "No resources found".
- **Expected**: The examples and suggested commands use selectors that match objects Gameplane creates, for example `app.kubernetes.io/instance=<server>` for pods, or `kubectl get job <backup-name>` for a Backup's Job.
- **Actual**: All three reference label keys that don't exist, so an assistant following the server's own guidance gets empty results.

### C-mcp-server-03: list_events silently ignores an invalid labelSelector and returns every event

- **Location**: `mcp-server/tools.go:201-203`, `mcp-server/tools.go:218-222`
- **Category**: error-handling
- **Suggested severity**: S4
- **Observation / repro**:
  1. Call `list_events` with `namespace: "gameplane-games"` and `labelSelector: "app in ("` (malformed).
  2. `filterEventsByLabel` calls `labels.Parse(selector)`. On error it `return list`, the full unfiltered list, and drops the parse error.
  3. The tool returns every event in the namespace as if the filter had matched all of them.
  4. By contrast, `list_pods` and `list_gameplane_resources` pass the selector to the API server and return a tool error for the same input.
- **Expected**: An invalid selector gives a tool error, as it does in the sibling tools. At minimum, the result shouldn't look like a successful filtered list.
- **Actual**: The error is dropped and the caller gets unfiltered data. Related: a *valid* selector matches only the Event objects' own labels (`tools.go:212-217` says so). Kubernetes event recorders don't set labels, so in practice any non-empty valid selector returns nothing. The advertised tool description ("optionally filtered by a field selector ... or label selector", `tools.go:82-83`) doesn't tell the client this.

### C-mcp-server-04: "the 7 Gameplane CRDs" is stale: the operator defines 9, and Cluster and NetworkCapture can't be read

- **Location**: `mcp-server/README.md:5`, `mcp-server/README.md:95`, `mcp-server/specs.md:8`, `:14`, `:66`, `mcp-server/main.go:3`, `:130` (the `Instructions` string sent to MCP clients), `mcp-server/internal/kube/client.go:44`, `:82`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. README.md:5 says the server can read "the 7 Gameplane CRDs, Pods, Events, and pod logs". The server's `Instructions` tell the client "list/get the 7 Gameplane CRDs".
  2. `operator/api/v1alpha1/` defines 9 kinds. `Cluster` (`cluster_types.go:82`, cluster-scoped) and `NetworkCapture` (`networkcapture_types.go:112`, namespaced) are not in `CRDKinds` (`client.go:60-68`) and not in the ClusterRole (`mcp-server.yaml:28-30`). CLAUDE.md also describes the operator as reconciling 9 CRDs.
  3. `get_gameplane_resource` with `kind: "NetworkCapture"` returns `unknown Gameplane resource kind`.
- **Expected**: The docs and the client-facing instructions describe the actual scope ("7 of the 9 Gameplane CRDs"), or the two missing kinds are added. Whether they should be added is a product decision; see Questions.
- **Actual**: The definite "the 7" tells both human readers and the AI client that the listed kinds are all of Gameplane's CRDs.

### C-mcp-server-05: specs.md dependency table, marked "verified", lists versions that don't match go.mod

- **Location**: `mcp-server/specs.md:135-145`, `mcp-server/go.mod:5-10`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:135` says "From `go.mod` (verified):" and lists `go-sdk v0.8.0`, `k8s.io/api v0.35.0`, `k8s.io/apimachinery v0.35.0`, `k8s.io/client-go v0.35.0`, `controller-runtime v0.23.3`.
  2. `go.mod` requires `go-sdk v1.8.0`, `k8s.io/* v0.37.0`, and `controller-runtime v0.25.1`. These match `operator/go.mod`, so the "pinned to match the operator" rationale at `specs.md:145` still holds; only the numbers are stale. The go-sdk entry is a major version off (v0 vs v1).
- **Expected**: The table matches `go.mod`, or it drops the version column and the "(verified)" label.
- **Actual**: All five versions are stale.

### C-mcp-server-06: Chart comment points to a non-existent mcp-server/client.go

- **Location**: `charts/gameplane/templates/mcp-server.yaml:19`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. The comment above the ClusterRole reads: "(Client in mcp-server/client.go exposes no mutating method, ...)".
  2. There is no `mcp-server/client.go`. The type is in `mcp-server/internal/kube/client.go`, and the package-boundary argument in the README and specs depends on it being in `internal/kube`.
- **Expected**: The comment names `mcp-server/internal/kube/client.go`.
- **Actual**: The path is wrong. (The file is in the chart, but the comment documents mcp-server, so it's reported here.)

### C-mcp-server-07: The README's standalone run example doesn't work against the repo's own dev cluster

- **Location**: `mcp-server/README.md:107-118`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. Bring up the dev cluster with `make dev-up`. Kind writes a kubeconfig whose server is `https://127.0.0.1:<port>` (`deploy/kind/cluster.yaml:11` sets `apiServerAddress: "127.0.0.1"`).
  2. Run the README command exactly: `KUBECONFIG=~/.kube/config docker run --rm -i -v ~/.kube/config:/kubeconfig:ro -e KUBECONFIG=/kubeconfig ghcr.io/valgulnecron/gameplane/mcp-server:edge serve`.
  3. Inside the container, 127.0.0.1 is the container's own loopback, and the command doesn't pass `--network host`. `serve` starts because `kube.New` doesn't contact the server, but every tool call fails with a connection-refused error from the Kubernetes client.
  4. Separately, the image runs as UID 65532 (`Dockerfile`, `USER 65532:65532`). client-go writes kubeconfigs with mode 0600 (kind uses it), so with rootful Docker the bind-mounted file isn't readable by that UID, and `serve` exits with `load kubeconfig: ...`. The leading `KUBECONFIG=~/.kube/config` assignment only affects the `docker` CLI process, not the container.
- **Expected**: The README (line 115-118) says `serve` "works in-cluster ... or locally against `KUBECONFIG`/`~/.kube/config`", and `main.go:33-34` mentions "locally against a kubeconfig for a dev cluster". The standalone example works for the documented local case, or it mentions `--network host` and `--user`/file-mode requirements.
- **Actual**: The example as written fails against the kind cluster that `make dev-up` creates.

## Questions (not findings)

- **Missing kinds.** Should `NetworkCapture` and `Cluster` be readable through `list_gameplane_resources`/`get_gameplane_resource`? `NetworkCapture` status is the kind of thing `propose_fix` would help with. Adding them means widening the ClusterRole, which is a security-review decision, so it's left as a question.
- **Heuristic keyword matching.** `matchFixRules` uses plain substring matching (`fixadvice.go:144-156`), so the `oom` keyword matches "room", "zoom" or "bloom", and a symptom like "players can't join the room" gets OOMKilled advice. The spec calls these heuristics, so this is a question of whether word-boundary matching is wanted, not a defect.
- **Missing container name.** `get_pod_logs` without `container` fails on every Gameplane game pod, because they always have at least the game and `agent` containers. The schema says the field is "Required if the pod has more than one container", so this is documented. Would a default (the game container) be preferable?

held candidates: 1 (see OD-019)
