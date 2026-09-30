# T045 aux chunk: independent verification (opus)

Verifier input: `notes.md` in this directory (opus reviewer), candidates C-mcp-server-01 to -07. This component's held candidates are verified separately, off-git (OD-019).

**Method.** I tried to refute each candidate against the code on `master` (`13a859ff`). The component, chart and operator files on this branch are byte-identical to `master`. I read `mcp-server/main.go`, `tools.go`, `fixadvice.go`, `internal/kube/client.go`, `main_test.go`, `specs.md`, `README.md`, `go.mod` and `Dockerfile`, as well as `charts/gameplane/templates/mcp-server.yaml`, the operator's CRD markers (`operator/api/v1alpha1/*_types.go`), the game pod labels (`gameserver_controller.go:1373-1377`), the backup Job (`backup_controller.go:245-251`), `operator/go.mod` and `deploy/kind/cluster.yaml`. I ran `git grep` for every label key the server suggests. On kubelab (`KUBECONFIG=~/kubelab.yaml`) I ran only read-only commands: `kubectl get --show-labels`, `get`s with a label selector, and `kubectl logs --tail`. For C-03 I ran `labels.Parse` from the module's own apimachinery version in a scratch program. For C-07 I ran one throwaway container with `--network none` to test whether UID 65532 can read a bind-mounted file with mode 0600. `go build ./...` passes in `mcp-server/`. I ran no test or lint suite, and I changed no repo file other than this one. None of the kept items is tracked in `findings.md` or raised by another review. The charts review mentions the "7 CRDs" wording at `values.yaml:395` only as an unraised nit, and I've folded it into C-04. Severity follows research R3.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-mcp-server-01 | kept | S3 | Confirmed on kubelab data. `PodLogs` keeps the first 256 KiB of the stream (`io.ReadAll(io.LimitReader(stream, maxLogBytes))`, `client.go:262`). The API server returns a tail oldest-first, so the cap keeps the old end and drops the newest lines. For `soak-pool-west-0/game`, `--tail=5000` gives 5000 lines and 510,000 bytes. The first 262,144 bytes hold 2,570 lines and end at log time `127248.515`, while the newest line is at `145468.515`. The result has no truncation marker. Workaround: ask for a smaller `tailLines`. |
| C-mcp-server-02 | kept | S4 | Confirmed. `git grep` finds `gameplane.local/server=` and `gameplane.local/backup=` only in `mcp-server/tools.go:149` and `fixadvice.go:74`, so nothing sets either key. On kubelab, `kubectl get pods -A -l gameplane.local/server` returns "No resources found", while game pods carry `app.kubernetes.io/instance=<server>`. All 5 GameServers show labels `<none>`, and `-l gameplane.local/template=minecraft-java` on gameservers returns nothing, even though `mc-fabric` uses that template. The label exists only on pods (`gameserver_controller.go:1376`). Backup Jobs are created with `Name: b.Name` and no labels (`backup_controller.go:245-251`). |
| C-mcp-server-03 | kept | S4 | Confirmed. `labels.Parse("app in (")` returns `unable to parse requirement ...`, and `filterEventsByLabel` then returns the input list unchanged (`tools.go:219-222`). So `list_events` answers a malformed `labelSelector` with every event, reported as a success. The sibling tools pass the selector to the API server and return a tool error (`client.go:182-185`, `:213-216`). No test covers `filterEventsByLabel`. |
| C-mcp-server-04 | kept | S4 | Confirmed. `operator/api/v1alpha1/` defines 9 kinds. `Cluster` (`cluster_types.go:82`) and `NetworkCapture` (`networkcapture_types.go:112`) are in neither `CRDKinds` (`client.go:60-68`) nor the ClusterRole (`mcp-server.yaml:29`). Yet the README, the spec, the chart values comment and the `Instructions` string sent to every MCP client call the supported kinds "the 7 Gameplane CRDs". The defect is the wording, which presents 7 of 9 as the full set. Whether to add the two kinds is a separate RBAC-scope decision. |
| C-mcp-server-05 | kept | S4 | Confirmed. `specs.md:135-143` says "From `go.mod` (verified)" and lists go-sdk v0.8.0, k8s.io/* v0.35.0 and controller-runtime v0.23.3. `go.mod:6-10` has go-sdk **v1.8.0**, k8s.io/* **v0.37.0** and controller-runtime **v0.25.1**, which match `operator/go.mod:25-29`. The "pinned to match the operator" rationale still holds, but all five numbers are wrong, and go-sdk is a major version off. |
| C-mcp-server-06 | kept | S4 | Confirmed. `charts/gameplane/templates/mcp-server.yaml:19` refers to "Client in mcp-server/client.go". `ls mcp-server/` shows no such file. The type is at `mcp-server/internal/kube/client.go:116`, and the read-only argument in README.md:12-25 depends on it being in `internal/kube`. |
| C-mcp-server-07 | kept | S4 | Confirmed, with the two causes reordered by how widely they apply. (1) The image runs as `USER 65532:65532` (`Dockerfile:21`), and kubeconfigs are normally mode 0600 and owned by the host user. A test container running as 65532 reports a bind-mounted 0600 file as `NOT-readable` under this host's Docker, so the README command fails at `load kubeconfig` for any such kubeconfig. (2) With the repo's own kind cluster (`deploy/kind/cluster.yaml:11-12`, `127.0.0.1:6443`), 127.0.0.1 inside the container is the container itself. Without `--network host`, every tool call gets connection-refused, even with a readable kubeconfig. |

### C-mcp-server-01

**Location:** `mcp-server/internal/kube/client.go:262`, inside `PodLogs` (`:243-267`). `maxLogBytes` is defined at `:72`.

**Repro / observation:**
1. Pick a container whose last 5000 log lines exceed 256 KiB. On kubelab, `kubectl logs -n gameplane-games soak-pool-west-0 -c game --tail=5000 | wc -lc` prints `5000 510000`.
2. Reproduce what `PodLogs` returns: `kubectl logs ... --tail=5000 | head -c 262144 | wc -l` prints `2570`. The last complete line has log time `127248.515`, and `kubectl logs ... --tail=1` shows `145468.515`.
3. Through the MCP server (`kubectl exec -i deploy/gameplane-mcp-server -n <release-ns> -- /mcp-server serve`), call `get_pod_logs` with `namespace: gameplane-games`, `pod: soak-pool-west-0`, `container: game` and `tailLines: 5000`. The text returned ends at the same line as in step 2 and doesn't say it was truncated.
4. Code path: `client.go:251-256` requests `TailLines`, and the API server streams those lines oldest-first. `io.ReadAll(io.LimitReader(stream, maxLogBytes))` then keeps the first `maxLogBytes` bytes and throws away the rest.

**Expected:** The tool is described as "Fetch a bounded tail of a Pod's container logs" (`tools.go:88`), and `PodLogs` as fetching "(a bounded tail of) a pod's logs" (`client.go:240`). When the byte cap applies, the newest bytes are kept, or fewer lines are returned, and the result says it was truncated. The newest lines matter most here, because the CrashLoop advice in `propose_fix` sends users to this tool (`fixadvice.go:30-31`).

**Actual:** When the output is over 256 KiB, the result is the oldest part of the requested tail. The most recent output, which is the part closest to a crash, is dropped without any notice.

### C-mcp-server-02

**Location:** `mcp-server/tools.go:106` (the `list_gameplane_resources` example), `mcp-server/tools.go:149` (the `list_pods` example) and `mcp-server/fixadvice.go:74` (the backup advice).

**Repro / observation:**
1. `git grep -n "gameplane.local/server=\|gameplane.local/backup="` matches only `mcp-server/tools.go:149` and `mcp-server/fixadvice.go:74`. Nothing in `operator/` or `api/` sets either key. (`gameplane.local/server-name` is a different label, used on user-owned Secrets.)
2. The operator labels game pods with `app.kubernetes.io/name=gameplane-game`, `app.kubernetes.io/instance=<server>` and `gameplane.local/template=<template>`, on the StatefulSet pod template only (`operator/internal/controller/gameserver_controller.go:1373-1377`). Backup Jobs are created as `Name: b.Name` with no labels (`backup_controller.go:245-251`).
3. On kubelab (read-only), `kubectl get pods -A -l gameplane.local/server` prints "No resources found". `kubectl get gameservers.gameplane.local -A --show-labels` lists five GameServers, including `mc-fabric` with template `minecraft-java`, and all have labels `<none>`. `kubectl get gameservers.gameplane.local -A -l gameplane.local/template=minecraft-java` prints "No resources found".

**Expected:** The schema examples and suggested commands use selectors that match objects Gameplane creates. For example: `app.kubernetes.io/instance=<server>` for a server's pods, the `gameplane.local/template=<t>` example only on `list_pods`, and `kubectl get job <backup-name>` for a Backup's Job.

**Actual:** All three examples use label keys, or label-object pairings, that never occur. An assistant that follows the server's own guidance gets empty results and may conclude the resources don't exist.

### C-mcp-server-03

**Location:** `mcp-server/tools.go:218-222` (`filterEventsByLabel`), called from `tools.go:201-203`.

**Repro / observation:**
1. Read `tools.go:219-222`: `sel, err := labels.Parse(selector); if err != nil { return list }`. The error is dropped, and the full, unfiltered list is returned.
2. `labels.Parse("app in (")`, run against apimachinery v0.37.0 (the version in `go.mod`), returns `unable to parse requirement: found '', expected: ',', ')' or identifier`.
3. So calling `list_events` with `namespace: "gameplane-games"` and `labelSelector: "app in ("` returns every event in the namespace, as a success.
4. For comparison, `list_pods` passes the same selector to the API server (`client.go:213-216`), and the error comes back as a tool error.
5. `grep -rn filterEventsByLabel mcp-server/*_test.go` finds nothing, so no test pins either behaviour.

**Expected:** A malformed `labelSelector` gets a tool error, as it does in the sibling tools.

**Actual:** The parse error is dropped, and the caller gets unfiltered data that looks like a successful filtered result.

### C-mcp-server-04

**Location:** `mcp-server/README.md:5` and `:95`; `mcp-server/specs.md:8`, `:14` and `:66`; `mcp-server/main.go:3` and `:130-131` (the `Instructions` string sent to MCP clients); `charts/gameplane/values.yaml:395`.

**Repro / observation:**
1. `grep -n "+kubebuilder:resource" operator/api/v1alpha1/*_types.go` lists 9 kinds: Backup, BackupSchedule, Cluster, GameServer, GameTemplate, Module, ModuleSource, NetworkCapture and Restore.
2. `CRDKinds` (`client.go:60-68`) and the ClusterRole (`charts/gameplane/templates/mcp-server.yaml:29`) cover 7 of them. `Cluster` and `NetworkCapture` are missing.
3. `mcp-server/README.md:5` says the server reads "the 7 Gameplane CRDs". `main.go:130-131` tells every MCP client "list/get the 7 Gameplane CRDs". `values.yaml:395` uses the same phrase. `get_gameplane_resource` with `kind: "NetworkCapture"` returns `unknown Gameplane resource kind`.

**Expected:** The docs and the instructions sent to clients describe the real scope, for example "7 of the 9 Gameplane CRDs (not Cluster or NetworkCapture)".

**Actual:** "The 7 Gameplane CRDs" tells human readers and the AI client that these are all of Gameplane's CRDs.

### C-mcp-server-05

**Location:** `mcp-server/specs.md:135-143`, compared with `mcp-server/go.mod:5-11`.

**Repro / observation:**
1. `sed -n 135,143p mcp-server/specs.md` shows "From `go.mod` (verified):" followed by go-sdk `v0.8.0`, `k8s.io/api|apimachinery|client-go` `v0.35.0` and controller-runtime `v0.23.3`.
2. `sed -n 5,11p mcp-server/go.mod` shows go-sdk `v1.8.0`, `k8s.io/*` `v0.37.0` and controller-runtime `v0.25.1`. `operator/go.mod:25-29` has the same k8s and controller-runtime versions.

**Expected:** The table matches `go.mod`, or it drops the version column and the "(verified)" label.

**Actual:** All five versions are stale, and go-sdk is a major version off.

### C-mcp-server-06

**Location:** `charts/gameplane/templates/mcp-server.yaml:19`.

**Repro / observation:**
1. `sed -n 16,21p charts/gameplane/templates/mcp-server.yaml` shows "... (Client in mcp-server/client.go exposes no mutating method, ...)".
2. `ls mcp-server/client.go` fails. The type is defined at `mcp-server/internal/kube/client.go:116`.

**Expected:** The comment names `mcp-server/internal/kube/client.go`.

**Actual:** The comment names a file that doesn't exist.

### C-mcp-server-07

**Location:** `mcp-server/README.md:107-118`. Related: `mcp-server/Dockerfile:21` (`USER 65532:65532`) and `deploy/kind/cluster.yaml:11-12`.

**Repro / observation:**
1. Use a Linux host with Docker and a kubeconfig in the usual mode 0600, owned by your user (`ls -l ~/.kube/config`).
2. Run the README command as written: `KUBECONFIG=~/.kube/config docker run --rm -i -v ~/.kube/config:/kubeconfig:ro -e KUBECONFIG=/kubeconfig ghcr.io/valgulnecron/gameplane/mcp-server:edge serve`.
3. The process runs as UID 65532, which can't read the mounted file, so `serve` exits with `load kubeconfig: ...`. I confirmed the permission part with a local image, no network and a dummy mode-0600 file: `docker run --rm --network none -u 65532:65532 -v $PWD/dummy:/kubeconfig:ro --entrypoint sh <image> -c 'test -r /kubeconfig && echo readable || echo NOT-readable'` prints `NOT-readable`.
4. With a readable kubeconfig for the repo's dev cluster (`make dev-up`, server `https://127.0.0.1:6443` per `deploy/kind/cluster.yaml:11-12`), `serve` starts, because `kube.New` doesn't contact the server. Every tool call then fails with connection refused, because 127.0.0.1 inside the container is the container itself and the command has no `--network host`.
5. The leading `KUBECONFIG=~/.kube/config` assignment applies only to the `docker` CLI process, not to the container.

**Expected:** README.md:115-118 says `serve` "works ... locally against `KUBECONFIG`/`~/.kube/config`", and `main.go:33-34` mentions running "locally against a kubeconfig for a dev cluster". Either the standalone example works for that case, or the README says what it needs: `--user "$(id -u):$(id -g)"` or a readable copy of the kubeconfig, plus `--network host` for a loopback API server.

**Actual:** Copied as written, the example fails on a typical Linux host. Against the repo's own kind cluster, it also can't reach the API server.
