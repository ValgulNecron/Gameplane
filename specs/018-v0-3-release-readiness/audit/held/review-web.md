# Held review candidates: web (OD-019)

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `web/specs.md`; `specs/done_016-user-theme-customization/`; `api/internal/handlers/` (config, auth provider, registry and notification secret routes); `operator/internal/controller/module_controller.go`

These candidates concern authentication config, secret handling, RBAC gating or supply-chain indicators, so they're held off-git until fixed (OD-019). Each one names the control, where it lives, what correct behaviour looks like, and how a maintainer can confirm it holds. None of them includes a reproduction beyond what's needed to confirm.

## Candidates

### HC-web-01: Admin Settings changes managed Secrets before the section is saved
- **Location**: `web/src/routes/AdminSettings.tsx:307-315` (identity provider row removal), `:974-982` (mod-registry key removal), `:1460-1468` (notification sink removal); also the add paths `:502-536`, `:929-941`, `:1149-1176`
- **Category**: correctness (secret handling)
- **Suggested severity**: S2
- **Control**: the lifecycle of the API-managed Secrets that back identity providers (`gameplane-auth-<name>`), keyed mod registries and notification sinks.
- **Observation**: the trash button on a row fires `AuthProviders.deleteSecret` / `ModRegistries.deleteSecret` / `Notifications.deleteSecret` at once, fire-and-forget, and only edits the local draft. The config row that references the Secret changes only when the admin clicks the section's Save. The add forms do the reverse: they write the Secret before the row is saved.
- **Expected**: Secret deletion is committed together with (or after) the config change that stops referencing it. Leaving the page without saving leaves both unchanged.
- **Actual**: removing a row and then leaving without saving keeps the provider, key or sink in the stored config while its Secret is already gone. SSO through that provider, the keyed registry, or delivery through that sink then stops working. Cancelled adds leave orphaned Secrets.
- **How to confirm it holds after a fix**: on a test install, remove a dashboard-managed provider row in Admin Settings → Authentication and reload without saving. `GET /admin/config` should still list the provider, and `kubectl get secret gameplane-auth-<name> -n <control-plane ns>` should still find the Secret. Repeat for a mod-registry key and a notification sink.

### HC-web-02: After "Reset to Helm default", the next Save puts the role-mapping override back
- **Location**: `web/src/routes/AdminSettings.tsx:197-199` (`useSectionForm` seeds its draft once), `:288`, `:413-420`, `:1748-1757` (`handleReset`); `web/src/lib/config.ts:170-176`
- **Category**: correctness (authorization config)
- **Suggested severity**: S2
- **Control**: the OIDC role-mapping overrides (`auth.helmOverride.roleMappings`), which decide which IdP groups get admin, operator or viewer.
- **Observation**: `handleReset` calls `DELETE /admin/config/auth/role-mappings/{role}` and invalidates `["config"]`. `RoleMappingOverridesCard` renders from `f.draft`, and `f.draft` is `useState(initial)`. It never re-syncs, and `AuthSection` isn't keyed, so it doesn't remount. The card keeps showing the old override and its "Overridden in dashboard" badge. The next click on "Save changes" or "Save role mappings" PUTs the stale `helmOverride` back. A failed reset only goes to `console.error`.
- **Expected** (`web/specs.md:836`): "Reset button: 'Reset to Helm default' … allows reverting to Helm values". After a reset, the card and draft show the Helm value, and a later Save doesn't restore the override.
- **Actual**: the reset looks like it did nothing, and any later Save of the auth section re-applies the override the admin just reset. That includes an admin-group mapping.
- **How to confirm it holds after a fix**: add an override for one role, save, then click Reset. The provenance badge should switch to "From Helm values" without a page reload. Then change something unrelated in the Authentication card, save, and check that `GET /admin/config` shows no `helmOverride.roleMappings.<role>`. Note: C-api-03 (review-api) is a separate API-side problem with the same reset route.

### HC-web-03: The Modules "verified" badge follows the source's current policy, not a verification that actually ran
- **Location**: `web/src/lib/verify.ts:48-51`; `web/src/components/modules/ModuleCard.tsx:249-265`; related operator behaviour at `operator/internal/controller/module_controller.go:100-107` (a converged module is not pulled or verified again)
- **Category**: correctness (supply-chain indicator)
- **Suggested severity**: S3
- **Control**: the cosign signature-verification posture that the Modules catalog shows for installed modules.
- **Observation**: for an installed entry, `verifyForEntry` sets `enforced: mode !== "none"` from the `installedFrom` source's *current* `spec.verify`. The card then shows a solid "verified" chip with the tooltip "signature verified". The operator returns early for a module that is already Ready on the same version and digest, so adding a verify policy to a source later doesn't verify the bundle that's already installed.
- **Expected**: the solid "verified" state appears only when the installed digest was actually signature-checked (for example, recorded on the Module status).
- **Actual**: a module installed while its source had no verify policy shows "verified" as soon as a policy is added to that source.
- **How to confirm it holds after a fix**: install a module from a source without `verify`, then add a `verify` block to that ModuleSource. The installed card should not show the solid "verified" chip until the operator has re-verified the installed digest, and the Module status should record that verification.

### HC-web-04: Frontend permission gating doesn't match the permission model
- **Location**: (a) `web/src/routes/ServerDetail.tsx:133-138` and `web/src/components/CaptureWidget.tsx:180-203`; (b) `web/src/routes/tabs/Mods.tsx:68`, `:434`, `web/src/routes/tabs/Modpacks.tsx:40`, `web/src/components/server/ServerActionsCard.tsx:139`, with `web/src/lib/auth.ts:34-42`; (c) `web/src/components/ClusterSelector.tsx:114-123`
- **Category**: correctness (RBAC gating in the UI; the API is still the enforcer)
- **Suggested severity**: S4
- **Control**: the dashboard's permission gating described in `web/specs.md:19` ("Enforce role-based access control (RBAC) on frontend routes") and `:895` ("Capture — … gated on the `captures:manage` permission").
- **Observation**:
  - (a) The Capture tab is always in `visibleTabs`. `CaptureWidget` offers "Enable Capture", "Start Capture" and "Delete" without checking `captures:manage`. The Settings → Network capture section does gate on it (`NetworkCapture.tsx:53`).
  - (b) These call sites use `can(me, "servers:write")` without a namespace. `can()` only consults `perms["*"]` unless a namespace is passed, so a user whose `servers:write` comes from a namespaced binding always sees these controls disabled, even for servers in that namespace.
  - (c) "Add cluster" is offered to every user. It navigates to `/cluster`, which is gated on `servers:write`.
- **Expected**: UI gates use the same permission and namespace the API checks.
- **Actual**: (a) shows capture controls to users the API will refuse. (b) hides mod, modpack and quick-action controls from namespace-scoped operators the API would allow. (c) sends viewers to an "Access denied" page.
- **How to confirm it holds after a fix**: as a viewer, open a server. The Capture tab should be hidden or read-only. As a user whose only binding is `operator` in `gameplane-games`, the Mods tab's "Install mod" and the Quick actions should be enabled for a `gameplane-games` server, matching what the API accepts.

## Moved from audit/evidence/review-web/notes.md (OD-019, 2026-09-24)

### Q-web-remote-cluster

- **Remote-cluster Console and Logs.** With a remote cluster selected, the detail page loads the remote GameServer (REST carries `?cluster=`). The Console and Logs WebSockets carry no `?cluster=` (documented at `web/specs.md:31`), so they attach to the **local** cluster's server of the same name. The API's `rejectRemoteCluster` only acts when a `cluster` parameter is present. Typing `stop` there would stop a different server. Should the UI hide or disable Console and Logs when the selected cluster isn't `local`, or send `?cluster=` so the API returns 404?
