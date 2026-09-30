# T045 web chunk: independent verification (opus)

Held under OD-019, off-git. This file covers the four candidates in `held/review-web.md`. Its wording is defensive: it names the control, where it lives, the correct behaviour, and how a maintainer confirms the control holds. It contains no misuse walkthroughs.

Method: I checked each candidate against master `13a859ff`. `web/`, `api/` and `operator/` are identical between this branch and master. I read `web/src/routes/AdminSettings.tsx` (`useSectionForm`, `AuthSection`, `RoleMappingOverridesCard`, `ModRegistriesSection`, the notification sink rows), `web/src/lib/config.ts`, `web/src/lib/verify.ts`, `web/src/components/modules/ModuleCard.tsx` (`VerifyBadge`), `web/src/lib/auth.ts` (`can`), every `can(me, …)` call site in `web/src`, `web/src/components/CaptureWidget.tsx`, `web/src/components/ClusterSelector.tsx` and `web/src/router/tree.tsx`. On the server side I read `api/internal/handlers/config.go` (`put`, `resetRoleMapping`), `operator/internal/controller/module_controller.go:88-108`, and `api/internal/auth/local.go` (the shape of the `permissions` map). I checked `audit/findings.md`, `web/specs.md` and the feature 016 and 017 spec folders for a decision that allows any of these; none does. I ran no test or lint suite and changed no repo file other than this one.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| HC-web-01 | kept | S3 | Confirmed. The row trash buttons call `deleteSecret` at once and fire-and-forget (`AdminSettings.tsx:307-315`, `:974-982`, `:1460-1468`), while the config row changes only in the local draft, which is kept until the section's Save (`useSectionForm`, `:197-224`). Nothing prompts about unsaved edits when leaving the page. Downgraded from S2: the dependent feature (that provider's sign-in, the keyed registry, or that sink) stops working, but an admin can recover by entering the secret again in the same form. |
| HC-web-02 | kept | S2 | Confirmed. `useSectionForm` seeds `draft` once from `useState(initial)` (`:198`) and never re-syncs it. `AuthSection` isn't keyed (`:130`), so it doesn't remount. `RoleMappingOverridesCard` renders from `f.draft` (`:414-420`), and a failed reset only logs to the console (`:1748-1757`). The auth `PUT` stores whatever `helmOverride` it receives (`config.go:102-156`). So the next save of the Authentication section writes the reset override back, including an admin-group mapping. That's an unintended change to authorization config, which is why the severity is S2. |
| HC-web-03 | kept | S3 | Confirmed. For an installed entry, `enforced` comes from the source's current `spec.verify` (`verify.ts:48-51`). The operator returns early for a module that is already Ready on the same version and digest (`module_controller.go:100-107`), so a policy added to the source later never verifies the installed bytes. The code's own contract (`verify.ts:22-24`: "the running bytes were signature-checked") is therefore not met, and the solid chip over-claims. |
| HC-web-04 | kept | S3 | Confirmed for (a) and (b). (c) is merged into C-web-15 (evidence, not held): it's the same "Add cluster" item with the same fix location, `ClusterSelector.tsx:114-123`. (a): `CaptureWidget` offers Enable, Start and Delete with no `captures:manage` check, although `web/specs.md:895` says the tab is gated on it. The API refuses these actions and the errors are shown, so this part alone would be S4. (b): `can()` consults `perms["*"]` unless a namespace is passed (`auth.ts:34-42`). `Mods.tsx:68`, `:434`, `Modpacks.tsx:40` and `ServerActionsCard.tsx:139` pass none, while `ServerActionsMenu.tsx:37-39` passes `ns ?? "gameplane-games"`. A user whose `servers:write` comes from a namespaced binding therefore sees mod, modpack and quick-action controls disabled for actions the API accepts. Raised from S4 to S3 because of (b): allowed actions are blocked in the UI, and the workaround is a cluster-wide binding. |

### HC-web-01

**Location:** `web/src/routes/AdminSettings.tsx:307-315` (identity provider removal), `:974-982` (mod-registry key removal), `:1460-1468` (notification sink removal); the add paths at `:502-536`, `:929-941`, `:1149-1176`; `useSectionForm` at `:197-224`.

**Control:** The lifecycle of the API-managed Secrets behind dashboard-managed identity providers (`gameplane-auth-<name>`), keyed mod registries and notification sinks. A Secret and the config row that references it should change together.

**Repro / observation** (by reading master `13a859ff`):
1. Each trash button does two things: `void <X>.deleteSecret(name).catch(() => undefined)`, which is sent at once, and `f.replace`/`f.update`, which changes only the section's local draft.
2. `useSectionForm.save` (`:215-222`) is the only path that PUTs the section config. Leaving the page, or a Save that fails validation for another field, leaves the stored config unchanged, with the row still in it.
3. The add forms work the other way round: they write the Secret before the row exists in the saved config, so a cancelled add leaves the Secret behind.

**Expected:** A Secret is deleted only as part of, or after, a successful save of the config change that stops referencing it. A cancelled add doesn't leave a Secret.

**Actual:** Removing a row and then leaving without saving keeps the row in the stored config while its Secret is already gone. The dependent feature stops working until the secret is entered again.

**How a maintainer confirms the control holds:** On a test install, remove a dashboard-managed identity provider row in Admin Settings → Authentication, then reload without saving. `GET /admin/config` should still list the provider, and `kubectl get secret gameplane-auth-<name> -n <control-plane ns>` should still find its Secret. Then save the removal and check that the Secret is gone. Repeat for a mod-registry key and a notification sink. For adds, cancel an add dialog and check that no new API-managed Secret exists.

### HC-web-02

**Location:** `web/src/routes/AdminSettings.tsx:197-199` (`useSectionForm` seeds its draft once), `:130` (`AuthSection` is not keyed), `:288`, `:413-420`, `:1748-1757` (`handleReset`); `web/src/lib/config.ts:170-176` (`useResetRoleMapping`); `api/internal/handlers/config.go:102-156` (`put`), `:159-250` (`resetRoleMapping`).

**Control:** The OIDC role-mapping overrides (`auth.helmOverride.roleMappings`), which decide which IdP groups get admin, operator or viewer. "Reset to Helm default" should remove an override for good.

**Repro / observation** (by reading master `13a859ff`):
1. `handleReset` calls `DELETE /admin/config/auth/role-mappings/{role}`, and on success invalidates `["config"]`. The API removes the override from the stored config.
2. `AdminSettingsPage` re-renders `AuthSection` with the fresh `initial`. `useSectionForm` ignores new `initial` values, though (`useState(initial)`), so `f.draft.helmOverride` still holds the removed mapping. `RoleMappingOverridesCard` receives `initial={f.draft}` and keeps showing it with its dashboard-override badge.
3. Any later "Save changes" in the Authentication card, or "Save role mappings", PUTs `f.draft`, and `config.go:put` stores it, putting the override back. `detectAuthAuditEvents` records it as a new override. The reset itself leaves only a `console.error` if it fails.

**Expected:** `web/specs.md:836`: "Reset to Helm default … allows reverting to Helm values". After a reset, the card and the draft show the Helm value, and no later save brings the override back.

**Actual:** The reset looks as if nothing happened, and the next save of the Authentication section re-applies the override the admin just reset, including an admin-group mapping. C-api-03 (review-api) covers a separate API-side problem with the same route.

**How a maintainer confirms the control holds:** On a test install, add an override for one role and save. Click Reset for that role. The provenance badge should switch to "From Helm values" without a page reload. Then change an unrelated field in the Authentication card and save. `GET /admin/config` should show no `auth.helmOverride.roleMappings.<role>`, and the audit log should record no new override for that role.

### HC-web-03

**Location:** `web/src/lib/verify.ts:48-51` (`verifyForEntry`, installed branch), `:22-24` (the contract comment); `web/src/components/modules/ModuleCard.tsx:249-265` (`VerifyBadge`); `operator/internal/controller/module_controller.go:100-107` (the early return for a converged module).

**Control:** The cosign signature-verification posture that the Modules catalog shows for installed modules. A solid "verified" chip should mean that the installed digest was signature-checked.

**Repro / observation** (by reading master `13a859ff`):
1. For an installed entry, `verifyForEntry` returns `enforced: mode !== "none"`, where `mode` comes from the `installedFrom` source's *current* `spec.verify`.
2. `VerifyBadge` renders the solid "verified" chip, with the tooltip "signature verified", whenever `enforced` is true.
3. The Module reconciler returns early when `AppliedVersion`, `AppliedTemplate`, `Phase == Ready` and the digest all match. A verify policy added to a source after install therefore doesn't make the operator pull or verify the installed bundle again. Nothing on the Module status records whether a verification actually ran.

**Expected:** The solid "verified" state appears only when the operator has actually verified the installed digest, for example as recorded on the Module status. Until then, a policy added later shows as the softer "policy" state.

**Actual:** A module installed while its source had no verify policy shows "verified" as soon as a policy is added to that source.

**How a maintainer confirms the control holds:** On a test install, install a module from a source with no `verify` block, then add a `verify` block to that ModuleSource. The installed card shouldn't show the solid "verified" chip until the operator has re-verified the installed digest. The Module status should record that verification (digest and outcome), and the badge should be derived from it.

### HC-web-04

**Location:** (a) `web/src/routes/ServerDetail.tsx:133-138` (the Capture tab is always visible), `web/src/components/CaptureWidget.tsx:180-203` and the start/delete controls below it. (b) `web/src/routes/tabs/Mods.tsx:68`, `:434`, `web/src/routes/tabs/Modpacks.tsx:40`, `web/src/components/server/ServerActionsCard.tsx:139`, with `web/src/lib/auth.ts:34-42`. For comparison, `web/src/routes/tabs/settings/NetworkCapture.tsx:53` and `web/src/components/server/ServerActionsMenu.tsx:37-39` pass a namespace. (c) is merged into C-web-15.

**Control:** The dashboard's permission gating (`web/specs.md:19`, and `:895`: "Capture … gated on the `captures:manage` permission"). The API is still the enforcer. The UI should show a control exactly when the API would accept the action, using the same permission and namespace.

**Repro / observation** (by reading master `13a859ff`):
1. (a) `visibleTabs` never filters out `capture`. `CaptureWidget` renders "Enable Capture", "Start Capture" and "Delete" with no `can(me, "captures:manage", ns)` check. The Settings → Network capture section does gate on it (`NetworkCapture.tsx:53`).
2. (b) `can(me, perm)` with no namespace checks only `perms["*"]`. The four call sites above pass no namespace. A user whose `servers:write` comes only from a binding in `gameplane-games` gets `false`, even for a server in `gameplane-games`, which the API authorizes against that namespace's binding (`docs/security.md:114-120`).

**Expected:** Each UI gate uses the permission and namespace the API checks for that action: `captures:manage` in the server's namespace for capture controls, and `servers:write` in the server's namespace for mods, modpacks and quick actions.

**Actual:** (a) shows capture controls to users the API will refuse; the refusal is displayed as an error. (b) disables mod, modpack and quick-action controls for namespace-scoped operators whose actions the API would accept.

**How a maintainer confirms the control holds:** On a test install, sign in as a viewer and open a server. The Capture tab should be hidden or read-only. Then give a user a primary role of viewer plus an `operator` binding in `gameplane-games` only. For a `gameplane-games` server, the Mods tab's "Install mod", the Modpacks install, and the Quick actions should be enabled, matching what the API accepts. For a server in a namespace where that user has no binding, they should stay disabled.
