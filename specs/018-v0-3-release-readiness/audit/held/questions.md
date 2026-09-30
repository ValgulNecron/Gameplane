# Held questions for the maintainer (OD-019, off-git)

Questions about security controls stay here, off-git, rather than in `OPEN-DECISIONS.md`.

## HQ-001: server ownership vs the operator role (2026-09-24)

While fixing `procedures/api.md`, an opus agent traced the ownership rules for the server `:transfer` and `:collaborators` actions and `DELETE`:
- `api/internal/rbac/rbac.go:109-141`: the owner/collaborator rule is a fallback that applies only when the caller lacks the namespace-level permission.
- The built-in operator role holds `servers:write` (`api/internal/db/migrations/003_roles.sql:48`).
- The `:transfer` and `:collaborators` handlers (`api/internal/handlers/ownership.go:79-230`) do no ownership check of their own. The share handlers do (`isServerOwner`).

Result: any account with the operator role can transfer, re-assign collaborators on, wipe or delete any server in its namespaces, whether or not it owns the server. The `rbac.go` package comment suggests this is intended.

Question: is "operators can manage any server in their namespaces" the intended control? If not, it's a security finding and gets an F-ID in `audit/held/findings.md`. Check first whether a held review candidate (`held/review-api.md`) already covers it.

**Answer (maintainer, 2026-09-24): not intended.** Record it as a held security finding and add it to the held fix plan.

**Recorded (2026-09-24):** no held entry already covered it. `held/review-api.md` has no candidate on these routes, and F-094 (H-api-05) is about share-link revocation. The behaviour was confirmed against the code at `c44cb179` and recorded as **F-256** (S2, `api/`) in `held/findings.md`. It is planned as held fix group **H31** in `held/fix-plan-held.md`, with decision D18 (who counts as an admin, and scope beyond the four owner-only operations).

## H31 split and admin scope (D18): maintainer answers, 2026-09-24
- Split H31. **H31a** (API-side owner-or-admin enforcement for transfer, collaborators, wipe-data and delete, with docs/specs and tests) goes ahead now. **H31b** (dashboard gates) waits for the maintainer's design.pen pass on the other machine (OD-025).
- D18: only full admins, meaning a role holding the `*` permission in that cluster and namespace, may act on a server they don't own. Scope for now: just those four operations.

## Held plan decisions D1–D17 (maintainer answers, 2026-09-24, session 4)
- Accepted every recommendation: D1 require both kubeconfig labels; D3 item 5 code, item 3 spec-only; D4 docs now; D5 and D6 wire netguard; D7 apply the tailscale tags; D8 document that S3 `insecure` means plain HTTP; D9 extend the denylist; D10 explicit deletes plus cleanup migration 012; D11 skip chart Secret generation for overridden names, and the new `helm template` check is approved; D13 separate metrics listener; D14 `spec.security` block; D17a keep the trusted-proxy default (with docs).
- D12: document the current reach of the apiserver egress rule, plus a release note. No default change.
- D15: a volume-snapshot restore copies the referenced Secrets/ConfigMaps, owned by the new server.
- D2: open. H10 waits with OD-026.
- D17(b): `type: security` labels are already visible on #427, #430 and #431; keep doing that.
- SECURITY_AUDIT.md: the reworded file was copied to this machine (uncommitted). Held branches don't edit it; H13 and H16 changes to it go into this working-tree copy only.

## Maintainer answers, 2026-09-27
- D2: settled by public PR #488 (F-262, merged): playit tunnel pods get unrestricted egress. H10 is unblocked; it covers the credential-refusal status and docs remainder of F-069/F-068 and lands after H29 and H11.
- H31b: the design.pen pass (OD-025) is done; H31b is unblocked.
- H22's e2e test TestAPI_ModuleUpload_ExtractionStaysWithinBudget goes in the multicluster bucket (login budget: api-mods is at its ceiling).
