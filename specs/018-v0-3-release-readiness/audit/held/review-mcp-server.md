# Held review candidates: mcp-server (OD-019, off-git)

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `mcp-server/specs.md` (Key invariants, layer 1), `mcp-server/README.md`, `mcp-server/main.go` package doc, `docs/security.md` `## mcp-server (optional)`, `docs/architecture.md:306-316`
- **Companion notes**: `audit/evidence/review-mcp-server/notes.md` (non-security candidates)

## Held candidates

### H-mcp-server-01: The documented "structural" read-only layer covers less than the docs claim: it constrains code holding only a *kube.Client, not package main as a whole

- **Control**: the read-only (privilege) guarantee, layer 1 ("structural enforcement by package boundary").
- **Location**: `mcp-server/main.go:8-17` (package doc), `mcp-server/main.go:106-115` (`runServe` holds the `*rest.Config`), `mcp-server/specs.md:78-91`, `mcp-server/README.md:12-25`
- **Category**: docs-drift
- **Suggested severity**: S4 (the current code performs no mutation, and the authoritative RBAC layer holds; this is about how accurately the control is described)
- **Spec statement**: `specs.md:91`: "**Package boundary enforcement**: code in `package main` cannot access unexported fields or call methods that don't exist on the exported `Client` API — therefore cannot call `Create`, `Update`, `Delete`, `Patch`, or `Apply` even if those methods existed on the underlying clientsets". `main.go:13-15`: "code here has no way to reach them, so it has no way to call Create/Update/Delete/Patch/Apply even by mistake". `docs/architecture.md:307-310` says the same.
- **Observation**: The package boundary does stop code in `package main` from reaching the clientsets *inside* a `kube.Client`. But `package main` itself obtains the `*rest.Config` (`main.go:107`), imports controller-runtime, and could build its own clientset. `kube.NewFrom` is also exported and accepts arbitrary `kubernetes.Interface`/`dynamic.Interface` values. So layer 1 depends on the convention that tool handlers are only given a `*kube.Client`. It doesn't make mutation impossible for `package main`, which is what the sentences above say. Current state, verified: there are no mutating call sites anywhere in the module, and the ClusterRole is get/list/watch plus get on `pods/log`.
- **Correct behaviour**: The layer-1 description matches what the package boundary enforces: "tool handlers receive only a `*kube.Client`, which exposes no mutating method". RBAC (layer 2) is stated as the only layer that prevents mutation by any code in the process. The docs already call RBAC "authoritative", so only the "cannot ... even by mistake" wording needs to change.
- **How a maintainer confirms it holds**: (1) `kubectl auth can-i --list --as=system:serviceaccount:<release-ns>:gameplane-mcp-server` shows only get/list/watch on the seven `gameplane.local` resources and core pods/events, and get on `pods/log`. (2) A grep of `mcp-server/` for `.Create(`, `.Update(`, `.Patch(`, `.Delete(`, `.Apply(`, `UpdateStatus` and `DeleteCollection` finds nothing. (3) Optionally, a test asserts that `runServe` passes nothing but the `*kube.Client` into `registerTools`.
- **Tracked-item note**: `SECURITY_AUDIT.md` lists the MCP server under "Controls reviewed with no gap found" ("backed by a Kubernetes client that implements only `Get` and `List`"). That conclusion still holds for the current code. The new aspect is only that the spec, README and architecture docs describe the package-boundary layer as broader than it is.

## Questions (not findings)

- **Pod specs are also readable.** The cluster-wide `get` on `pods` returns full Pod specs, including literal (non-`secretKeyRef`) env values of workloads in any namespace. The README's blast-radius section mentions "logs and object state for *any* pod". `docs/security.md:570-575` only discusses pod logs. Should docs/security.md name pod-spec env values explicitly alongside logs?
