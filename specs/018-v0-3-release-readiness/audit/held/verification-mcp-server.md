# T045 aux chunk: independent verification (opus)

Held (OD-019, off-git). Verifier input: `audit/held/review-mcp-server.md` (opus reviewer), candidate H-mcp-server-01.

**Method.** I read the read-only guarantee as it is documented (`mcp-server/main.go:7-22`, `mcp-server/specs.md:77-91`, `mcp-server/README.md:10-25`, `docs/architecture.md:306-316`) and compared it with the code on `master` (`13a859ff`; identical on this branch): `runServe` (`main.go:106-121`), `kube.New`/`kube.NewFrom` (`internal/kube/client.go:125-147`), the `Client` type (`:116-120`) and the chart ClusterRole (`charts/gameplane/templates/mcp-server.yaml:22-36`). I grepped the module for mutating call sites (`.Create(`, `.Update(`, `.Patch(`, `.Delete(`, `.Apply(`, `UpdateStatus`, `DeleteCollection`) and found none. `SECURITY_AUDIT.md` lists this control as "reviewed with no gap found". That conclusion still holds for the code, and this item is only about how the control is described. I ran no test or lint suite. Severity follows research R3.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| H-mcp-server-01 | kept | S4 | Confirmed as a documentation-accuracy defect in a security control. `package main` obtains the `*rest.Config` itself (`ctrl.GetConfig()`, `main.go:107`) and imports controller-runtime, and `kube.NewFrom` is exported. So the package boundary only guarantees that code holding a `*kube.Client` can't reach a mutating verb *through that client*. It doesn't make mutation impossible for `package main`, which is what `main.go:13-15` ("no way to call Create/Update/Delete/Patch/Apply even by mistake") and `specs.md:91` claim. The current code has no mutating call site, and the RBAC layer, which the docs already call authoritative, holds. |

### H-mcp-server-01

**Location:** `mcp-server/main.go:8-17` (the package doc, layer 1), `mcp-server/specs.md:77-91` (the "Package boundary enforcement" bullet at `:91`) and `mcp-server/README.md:12-25`. The code that bounds the claim is `main.go:106-115` (`runServe` holds the `*rest.Config`) and `internal/kube/client.go:145-147` (the exported `NewFrom`).

**Control:** the MCP server's read-only guarantee. It has two layers: (1) tool handlers receive only a `*kube.Client`, whose exported methods are all List/Get-shaped; (2) a ClusterRole that grants only `get`/`list`/`watch`, plus `get` on `pods/log`.

**Repro / observation** (how to confirm the control holds):
1. RBAC, the authoritative layer: `kubectl auth can-i --list --as=system:serviceaccount:<release-ns>:gameplane-mcp-server` shows only `get`/`list`/`watch` on the seven `gameplane.local` resources and on core `pods` and `events`, and `get` on `pods/log`. Compare with `charts/gameplane/templates/mcp-server.yaml:22-36`.
2. Code: a grep of `mcp-server/` for `.Create(`, `.Update(`, `.Patch(`, `.Delete(`, `.Apply(`, `UpdateStatus` and `DeleteCollection` finds nothing. `kube.Client` exports only `ListCRD`, `GetCRD`, `ListPods`, `GetPod`, `ListEvents` and `PodLogs`, and `main_test.go`'s `TestClientHasNoMutatingMethods` pins that.
3. Scope of layer 1: read `main.go:106-115`. `runServe` itself calls `ctrl.GetConfig()`, so `package main` holds a full `*rest.Config`. Layer 1 therefore depends on the convention that handlers receive only the `*kube.Client` built from it. The unexported fields are not the only route to a clientset.
4. Optional hardening a maintainer could add: a test or lint check asserting that `registerTools` receives only a `*kube.Client`, and that `tools.go`/`fixadvice.go` don't import `k8s.io/client-go/kubernetes`, `k8s.io/client-go/dynamic` or `sigs.k8s.io/controller-runtime/pkg/client`.

**Expected:** The layer-1 description says what the package boundary actually enforces, for example "tool handlers receive only a `*kube.Client`, which exposes no mutating method". RBAC is described as the only layer that stops mutation by any code in the process. `docs/architecture.md:306-316` already phrases it that way ("its tool handlers only ever hold a `*kube.Client` ... The authoritative backstop is RBAC").

**Actual:** `main.go:13-15`, `specs.md:91` and `README.md:19-22` describe layer 1 as making mutation impossible for `package main` "even by mistake". That is broader than the boundary enforces. The guarantee holds today because of RBAC and the absence of mutating call sites, not because of the package boundary alone.
