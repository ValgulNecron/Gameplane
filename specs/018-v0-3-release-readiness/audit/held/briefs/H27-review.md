# H27 review: `fix/018-harden-cluster-removal`

- Worktree: `/tmp/claude-1000/-home-valgul-project-Gameplane/0cbfe030-6e64-40ca-9a18-d291dbf5b40a/scratchpad/wt-H27`
- Commit: `e5adbadf fix(api): harden cluster registration removal` on base `61f50265`
- Reviewer: opus (tier + 1 over the haiku implementer)
- Verdict: **needs-fix** (one small docs addition; the code, tests and E2E are correct)

## 1. Fidelity to the brief

- I applied `briefs/H27.patch` to a clean checkout of `61f50265`. The result matched HEAD exactly: `git diff HEAD` was empty. The same 9 files changed, nothing else. The worktree is clean.
- No existing test changed. `TestMountClusters_DeleteSuccess` (unlabelled Secret, 204) still holds, because `deleteManagedSecret` returns NotFound for a Secret it won't delete, the handler ignores NotFound, and the response is still 204.
- The commit is signed off and carries the Co-Authored-By and Claude-Session trailers. It is not pushed.
- Nothing under `specs/*/audit/held/` and no `SECURITY_AUDIT.md` is in the diff.

## 2. Findings and decisions

- **F-248 / D1 ("require both labels"):** closed. `POST /clusters` now sets `gameplane.local/managed-by=gameplane-api` next to the kubeconfig label. `DELETE /clusters/{name}` now removes the Secret only through `deleteManagedSecret(..., kube.ClusterKubeconfigLabel)`. That helper Gets the Secret and deletes it only when both labels match (`secrets_managed.go:83-92`), the same rule the other API-managed Secrets follow. Errors are no longer thrown away: anything other than not-found goes to `slog.Warn`, without the Secret's contents.
- **F-101:** closed. `removeDeletedCluster` handles `*unstructured.Unstructured`, `DeletedFinalStateUnknown` with a wrapped object, and a bare tombstone (it uses the key; Cluster is cluster-scoped, so the key is the name). It still never removes the default cluster. The handler also calls `h.reg.Remove(name)` directly after the CR delete, so client removal no longer depends on the watch.
- **Race check:** Cluster has no finalizer (`operator/internal/controller/cluster_controller.go`, `cluster_types.go`), so the CR is gone once Delete returns. `loadCluster` does a live `Get` first, so an UpdateFunc that runs after the delete fails and cannot re-register the client. The only remaining window is an in-flight load that passed its Get before the delete and calls `Set` after `Remove`. That window is tiny, and the watch's DeleteFunc removes the client again anyway. No action needed.
- **Legitimate callers:**
  - API-registered clusters (Path 2) still get their Secret cleaned up.
  - kubectl or GitOps clusters (Path 1) keep their Secret, as D1 intends.
  - Deleting the local cluster is still refused with a 400 before any of this runs.

## 3. Tests

Which unit tests would fail before the fix:

- `TestMountClusters_CreateMarksKubeconfigSecretAsAPIManaged`: fails before the fix (no managed-by label). It builds the kubeconfig inline, which is valid.
- `TestMountClusters_DeleteRemovesAPIManagedKubeconfigSecret`: a positive control. It passes before and after the fix, and it guards against the fix over-reaching. This is fine.
- `TestMountClusters_DeleteKeepsSecretsNotManagedByTheAPI`: all four subtests fail before the fix, because the old code deleted every Secret. It covers no labels, each label alone, and another feature's managed Secret. It also asserts the CR is gone.
- `TestMountClusters_DeleteDropsClusterClientImmediately`: fails before the fix, because the handler never called `reg.Remove`. It also checks that `local` survives.
- `TestRemoveDeletedCluster_*`: these are new. Before the fix they would not compile, and the old DeleteFunc ignored both tombstone forms. `KeepsDefaultCluster` is a positive control.
- The test names state the control. None describes the gap.

E2E (`checkClusterRemoval`, called at the end of `TestMultiCluster_ClusterDispatchAndScopedRBAC`):

- **Placement:** it runs last and removes cluster B's GameServer and GameTemplate first. The cleanups registered earlier then hit an unknown cluster (400) or a missing CR (404), and they ignore both. `APIClient.Do` closes response bodies itself, so there is no leak.
- **Status code:** it asserts **400** for `?cluster=<removed>`. The fix plan says 404, but 400 matches the existing unknown-cluster assertion in the same test and `rbac.go:67` ("an unknown cluster is a 400"). The code is right and the plan's wording is off. No change needed.
- **Path 1 check:** it creates a kubectl-style Cluster CR plus a Secret, once unlabelled and once with only the kubeconfig label. The CR passes CRD validation (`kubeconfigSecret.name` is set). It asserts that the CR is gone and the Secret is kept. It also asserts that the API-created `cluster-<id>-kubeconfig` Secret is removed. `envInstance.K8s` and `envInstance.Dyn` exist (`env.go:39-40`).
- **Login budget:** no new `APIClient` logins. It reuses `admin` and `operatorClient`, which the test already logs in. The multicluster bucket is unchanged.
- **buckets.sh:** no new top-level E2E function (`checkClusterRemoval` is a helper). `buckets.sh verify` reports 132 tests, all in exactly one bucket.

## 4. Docs, specs and CHANGELOG

- `api/specs.md`: accurate.
- `docs/security.md`: the new "Delete guard" bullet is accurate and neutral.
- `docs/install.md` Path 1: the added paragraph (Secret left in place, delete it with kubectl) is accurate.
- `CHANGELOG.md`: "- **api:** hardened cluster registration removal." sits under a new `### Security hardening` heading at the end of `## [Unreleased]`, just before `## [0.3.0-rc.1]`. The format is correct.

### Issue 1 (needs-fix, docs accuracy): clusters registered before this change

Secrets that `POST /clusters` created before this change carry only `gameplane.local/cluster-kubeconfig=true`, with no `managed-by`. After an upgrade, removing such a cluster leaves its kubeconfig Secret in the control-plane namespace. This is the intended D1 behaviour, and deleting it would contradict "require both labels". But `docs/install.md` Path 2 now says without qualification: "Removing the cluster deletes both the `Cluster` and that Secret." That is false for existing registrations. An admin could then leave a working remote-cluster credential behind without knowing it.

**Exact fix:** in `docs/install.md`, Path 2, replace

~~~
logged. Removing the cluster deletes both the `Cluster` and that Secret.
~~~

with

~~~
logged. Removing the cluster deletes both the `Cluster` and that Secret.
Clusters registered through the API before v0.3.0 have a Secret without
the `gameplane.local/managed-by=gameplane-api` label; removing one of them
leaves its Secret in place, so delete it with kubectl
(`kubectl -n gameplane-system delete secret cluster-<name>-kubeconfig`).
~~~

Commit this as a follow-up commit on the same branch (e.g. `docs(install): note Secret cleanup for earlier cluster registrations`), signed, with the usual trailers. Keep the wording neutral. Do not change code or tests.

### Note (not git-bound, for the orchestrator)

The PR body in `fix-plan-held.md` (H27) says the Secret is deleted "only when it carries the API's kubeconfig label". Under D1 it is **both** labels. When the PR is opened, use: "Deletes a kubeconfig Secret on cluster removal only when the API created it, and drops the cluster client as soon as its registration is removed."

## 5. Wording leak scan

I grepped every added line of the diff and the commit message for `F-NNN`, `Hxx`, `OD-`, `audit018`, "arbitrary", "attack", "exploit" and "vulnerab". There were no hits. The comments and doc text describe controls only. The context line "arbitrary control-plane Secret" in `docs/security.md` is pre-existing text and is not part of this diff. **wordingLeak: false.**

## 6. Compile checks (run by the reviewer)

- `api/`: `go build ./...` and `go vet ./...` passed.
- `test/e2e`: `go vet -tags e2e ./...` passed.
- `gofmt -l api/internal test/e2e`: no output.
- `helm template charts/gameplane`: passed.
- `test/e2e/buckets.sh verify`: OK (132 tests).
- No test or lint suite was run.
