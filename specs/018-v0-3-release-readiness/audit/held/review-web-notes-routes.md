# Review: web-routes

- **Date**: 2026-09-23
- **Reviewer tier**: sonnet (verified by opus)
- **Checked against**: web/specs.md (full read); design-export/json/ (directory listing / structure only, per task instructions — node JSON not opened)

## Scope reviewed

**Full, line-by-line read:**
- web/src/router/tree.tsx
- web/src/components/RequireRole.tsx (adjacent to router gating; read to verify `RequireRole`/`RequirePermission` semantics that `tree.tsx` depends on)
- web/src/routes/Login.tsx
- web/src/routes/Share.tsx
- web/src/routes/Cluster.tsx
- web/src/routes/Dashboard.tsx
- web/src/routes/AdminLogs.tsx
- web/src/routes/AuditLog.tsx
- web/src/routes/ServerDetail.tsx
- web/src/routes/tabs/Settings.tsx
- web/src/routes/tabs/Events.tsx
- web/src/routes/tabs/settings/Access.tsx
- web/src/routes/tabs/settings/Danger.tsx
- web/src/routes/tabs/settings/General.tsx
- web/src/routes/tabs/settings/Version.tsx
- web/src/routes/tabs/settings/Resources.tsx
- web/src/routes/tabs/settings/Field.tsx
- web/src/routes/tabs/settings/NetworkCapture.tsx

**Substantial partial read** (key functions/sections read in full, remainder skimmed or grep-scanned):
- web/src/routes/tabs/settings/ShareLinks.tsx (~300 of 635 lines — expiry logic, CreateDialog, CreatedDialog)
- web/src/routes/tabs/settings/EnvVars.tsx (~120 of 208 lines — validation logic)
- web/src/routes/tabs/settings/Placement.tsx (~90 of 158 lines — JSON-field validation)
- web/src/routes/tabs/Overview.tsx (~120 of 428 lines)
- web/src/routes/tabs/Console.tsx (~90 of 157 lines — message parsing)
- web/src/routes/tabs/Files.tsx (load effect, download-confirm helper; not the Monaco/editor bulk)
- web/src/routes/tabs/Modpacks.tsx (error-formatting helper only)
- web/src/routes/AdminSettings.tsx (~700 of 1906 lines: header/gating, `useSectionForm`, `AuthCfg` add-provider form incl. admin-mapping confirm dialog, `HelmOIDCProviderCard`, `RoleMappingOverridesCard`/admin-mapping-confirm logic in full)
- web/src/routes/CreateServer.tsx (tunnel-credential save/error-chain block, wizard nav buttons; not the per-step form bodies)
- web/src/routes/Users.tsx (~150 of 757 lines: imports, permission checks, dialog wiring)
- web/src/routes/Servers.tsx (~50 of 748 lines: lifecycle-action wiring only)
- web/src/routes/Modules.tsx (~60 of 328 lines)
- web/src/routes/tabs/Players.tsx (~55 of 390 lines)

**Grep/pattern-scanned only** (searched for `any`, floating promises, `eslint-disable`, TODO, smart-quote corruption, secret/permission literals; not read start-to-end — flagged here per the "be honest about anything skipped" instruction):
- web/src/routes/Backups.tsx, web/src/routes/ThemeSettings.tsx, web/src/routes/tabs/Mods.tsx, web/src/routes/tabs/Logs.tsx, web/src/routes/tabs/Backups.tsx, web/src/routes/tabs/ConsoleShell.tsx, web/src/routes/tabs/useConsoleTerminal.ts, web/src/routes/tabs/settings/Backups.tsx, web/src/routes/tabs/settings/Lifecycle.tsx, web/src/routes/tabs/settings/Networking.tsx (1033 lines — only the top ~50 lines and the `onValidityChange` wiring were actually read)

`npx tsc --noEmit` was not run (`node_modules` is not installed in this environment and installing was out of scope for a read-only review); relied on manual reading + `web/eslint.config.js` inspection (confirmed `@typescript-eslint/no-floating-promises: "error"` is enabled) instead.

## Method

For each file/section: compared behavior against `web/specs.md` (Routing & Pages, ServerDetail Tabs, ServerDetail Settings Sub-sections, Security considerations, Key Invariants sections), checked login-privacy on Login.tsx/Share.tsx, checked `RequirePermission`/`can()` gating against the permission each route/section's spec text names, checked promise handling (await/void) against every `const x = async (...) =>` definition in scope and every bare-identifier `onPress=`/`onClick=`/`onOpenChange=` handler, checked `catch` blocks for silent swallowing vs. user-visible error surfacing, and grepped for dead code / stale TODOs / smart-quote corruption (a defect class the codebase's own `web/specs.md` T139 note flags as previously real in this exact area — reverified as fixed, see Observations).

## Observations (no finding)

- `web/src/router/tree.tsx` permission gates match `web/specs.md`'s "Routing & Pages" section exactly: `/cluster` → `servers:write`, `/users` → `users:manage`, `/admin` → `config:manage`, `/admin/audit` → `audit:read`, `/admin/logs` → `*` (matches the explicit code comment referencing the API's admin wildcard), `/settings/theme` and `/backups` intentionally ungated (matches spec's "same as /backups" comment).
- `RequireRole`/`RequirePermission` (web/src/components/RequireRole.tsx) correctly distinguish a confirmed 401 (redirect to `/login`) from an indeterminate identity fetch (`IdentityUnavailable`, does not claim "access denied" when the reason is unknown) from a confirmed non-401 permission failure (`Forbidden`). This matches the intent described in `web/specs.md`'s permission-gating discussion.
- Login.tsx: no cluster name/version/hostname/server-count ever rendered; error copy is strictly "Invalid credentials" / "Network error"; SSO button labels come from the pre-auth `Auth.providers()` response's `label` field, never an issuer URL. Matches `web/specs.md`'s T057 findings and the Pre-auth privacy invariant.
- Share.tsx: the fields it reveals (server name, status, address, player count) are exactly the fields `web/specs.md`'s "Share" page description says a share link may reveal (an owner-opted-in surface, distinct from the general pre-auth privacy rule) — not a privacy violation. `enforceUnauthenticatedTheme()` is mounted the same way in both Login.tsx and Share.tsx per spec.
- Cluster.tsx's storage-class card correctly gates on `config:read` both on the query (`enabled: canReadConfig`) and the rendered card, per `web/specs.md`'s explicit "Cluster.tsx storage card... requires config:read on both the query... and the rendered card" precedent.
- `AdminSettings.tsx`'s `RoleMappingOverridesCard`/`HelmOIDCProviderCard` split (storage vs. auth) and provenance-badge derivation (override present / Helm-seeded / not configured) match the spec's OD text precisely, including the "empty list `[]` is a distinct, intentional state from absent" rule (`hasOverride` checks key-presence with `in`, not truthiness).
- CreateServer.tsx's tunnel-credential-save-after-server-create error path matches `web/specs.md` verbatim: on `Servers.setTunnelCredentials` failure it throws `new Error(..., { cause: err })` with the exact remediation framing ("server exists... set the credential from the server's Networking settings").
- T139's previously-recorded HeroUI v2-API defects (curly-quote corruption, `SelectItem`, `classNames`, `emptyContent`, `Button variant="outline" asChild`) in `Backups.tsx`/`Modules.tsx` are **no longer present** — reverified via targeted grep; `git log` shows subsequent fix commits (`dbc6b2d1`, `21bb1398`, `b6b363b6`) between the note's date and now. Confirming this stayed fixed, not re-reporting.
- `AccessSection` (tabs/settings/Access.tsx) matches its `web/specs.md` description exactly: the `setCollaborators` mutation runs unconditionally at render (no early return before the hook), returns early inside `mutationFn` when `gs` is absent, and the add/remove handlers' error/success state handling matches the documented behavior.
- `NetworkCaptureSection` (tabs/settings/NetworkCapture.tsx) correctly fail-closes on `captures:manage` (`disabled = meLoading || !canManage`), validates the retention window against the documented 604,800s CRD ceiling, and never states the ceiling as a legal requirement — all as specified.
- No unjustified `any`, no `eslint-disable`, no leftover `console.log`/TODO/FIXME found anywhere in the reviewed scope (grepped across all files, not just the deep-read ones).
- `JSON.parse` call sites in scope (Console.tsx x2, Mods.tsx, Modpacks.tsx, Placement.tsx x2) are all wrapped in `try/catch` with sensible fallback behavior.

## Candidate findings

### C-web-routes-01: Admin OIDC role-mapping confirmation can be skipped on a second Add while the dialog is open
- **Location**: web/src/routes/AdminSettings.tsx:1700-1730 (`handleAddGroup`), dialog wired at 1890-1902
- **Category**: security
- **Suggested severity**: S2
- **Observation / repro**: `handleAddGroup` reads:
  ```js
  if (role === "admin" && !confirmingRole) {
    setConfirmingRole(role);
    setPendingGroups([...getRoleGroups(role), trimmed]);
    return;
  }
  const current = getRoleGroups(role);
  ...
  onUpdate({ ...initial, helmOverride: newOverride }); // applies immediately, no confirmation
  ```
  The guard that routes an admin-group add through `ConfirmAdminMappingDialog` is `role === "admin" && !confirmingRole`. Once the first Add sets `confirmingRole = "admin"` (opening the dialog), that guard is false for any *subsequent* call to `handleAddGroup("admin", …)` made while the dialog is still open (`confirmingRole` is only reset to `null` by the dialog's own cancel path, or to `null` via `handleConfirmAdminMapping` after a real confirm) — such a call falls through to the direct-`onUpdate` branch and adds the group to the saved draft with **no confirmation dialog shown at all**. The admin input field (`adminInput`) is never cleared when the dialog opens (only `pendingGroups`/`setConfirmingRole` are touched), so the "Add group" control for the admin row remains populated and its `onKeyDown`/`onPress` handlers remain wired to `handleAddGroup` — a second Enter/Add invocation while the dialog is open is the concrete path into the unconfirmed branch, whether or not the modal itself would visually block a second pointer click. Contrast with the equivalent flow for the "Add identity provider" form (same file, `handleSubmitClick` at line 540): there, the *only* way to reach `submit()` after the dialog opens is via the dialog's own `onConfirm={() => { setConfirmingAdmin(false); void submit(); }}` callback — the underlying "Add provider" button, if reachable again, would hit the same class of gap (`adminList.length > 0 && !confirmingAdmin`), but at least the confirming path there doesn't also leave a live text-input pathway feeding the same handler.
- **Expected**: Per `web/specs.md` FR-015, "Mapping users to the admin role grants full cluster control... Requires explicit user confirmation before the override is saved" — every attempt to add an admin-role IdP group mapping must go through `ConfirmAdminMappingDialog`, unconditionally.
- **Actual**: The confirmation gate is keyed off transient dialog-open state (`confirmingRole`) rather than "has this specific pending group been confirmed yet", so a second add invocation reachable while the dialog is open bypasses the confirmation and calls `onUpdate` directly.

### C-web-routes-02: Capture tab not gated on `captures:manage`, contradicting web/specs.md
- **Location**: web/src/routes/ServerDetail.tsx:133-138 (`visibleTabs` filter), :296 (`<CaptureWidget .../>` render)
- **Category**: correctness / docs-drift
- **Suggested severity**: S3
- **Observation / repro**: `web/specs.md` ("ServerDetail Tabs", item 10) states: "**Capture** — a `CaptureWidget` component driving start/stop of packet captures and a table of past captures for this server, **gated on the `captures:manage` permission**." In `ServerDetail.tsx`, `visibleTabs` filters out the `console`/`mods`/`modpacks` tabs by template capability but has no case for `capture` (it falls through the `return true` default), so the Capture tab is shown to every authenticated user regardless of permission, and `<CaptureWidget name={name} ns={ns} gs={gs} />` is rendered unconditionally at line 296. `web/src/components/CaptureWidget.tsx` itself contains no `can(me, "captures:manage", …)` check either (`grep -n "captures:manage" components/CaptureWidget.tsx` returns nothing), unlike the sibling `tabs/settings/NetworkCapture.tsx`, which does correctly gate on the same permission (`can(me, "captures:manage", namespace)`, fail-closed while loading). The permission is real and namespaced (confirmed via `test/handlers.ts:647,1470`: `{ key: "captures:manage", ... namespaced: true }`).
- **Expected**: Per spec, a user without `captures:manage` should not see (or should see a disabled/read-only) Capture tab, mirroring the pattern `NetworkCaptureSection` uses.
- **Actual**: Any authenticated user with access to a server's detail page sees a fully interactive Capture tab. The backend capture routes are stated elsewhere in `web/specs.md` to be "all gated by the `captures:manage` permission", so this is not a full authorization bypass (the API still enforces it) — the concrete impact is a UX correctness gap: a viewer/operator without the permission gets a live-looking start/stop capture UI that will 403 on every action, and the promised UI-level gating from the spec simply isn't implemented for this tab.

### C-web-routes-03: `web/specs.md` says configurable share-link expiry is "not yet implemented"; it is fully implemented
- **Location**: web/src/routes/tabs/settings/ShareLinks.tsx:42-119 (options, date-math helpers), :167-268 (`CreateDialog` UI)
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**: `web/specs.md` (under "ServerDetail Settings Sub-sections", the "Configurable expiry (feature 017...)" paragraph) states verbatim: *"the UI not yet implemented, blocked on design-first per CLAUDE.md rule 1 until design.pen nodes `atqRh`/`xCJlu` are redesigned"*. The actual code in `ShareLinks.tsx` implements this target contract completely and matches the spec's own description of it almost word-for-word: the exact six `EXPIRY_OPTIONS` (15/30/60/90 days, "No expiry", "Custom"), `DEFAULT_EXPIRY_CHOICE = "30"`, the "This link works until you revoke it." copy for "No expiry" (line 245), the 365-day `LONG_LIVED_THRESHOLD_DAYS` warning "Long-lived link — it stays valid for over a year unless you revoke it." (lines 262-265), calendar-day (not wall-clock) expiry math converting to an absolute RFC3339 instant client-side (`presetDaysToExpiresAt`/`customDateToExpiresAt`), and sending `neverExpires: true` for "No expiry" instead of the deprecated `expiresIn`.
- **Expected**: `web/specs.md` should reflect that this UI has shipped (the same section that documents "Share links (T179)" as implemented, just above, should presumably have been updated when this landed).
- **Actual**: The spec's own "target contract... UI not yet implemented" framing is stale and will mislead anyone using it to scope remaining v0.3 release work (e.g., assuming this still needs a design.pen pass and implementation effort). This is exactly the kind of docs-vs-behavior drift this review chunk was asked to flag; recommend updating the spec section (or filing it as done) rather than a code change.

### C-web-routes-04: Three Settings sub-sections show inline validation errors but never disable Save (`onValidityChange` not wired)
- **Location**: web/src/routes/tabs/settings/Resources.tsx:36,95-97 (`sizeValid`/error text, no `onValidityChange` call); web/src/routes/tabs/settings/EnvVars.tsx:69-111 (`nameInvalid`/`dup` FieldErrors, component doesn't even destructure `onValidityChange`); web/src/routes/tabs/settings/Lifecycle.tsx:158-167,305-349 (`graceInvalid`/`idleInvalid`/wake-window `invalid`, same gap) — contrast with web/src/routes/tabs/Settings.tsx:212,216,219-224 and the sections they render (`NetworkingSection`, `NetworkCaptureSection`, `PlacementSection`), which all correctly call `onValidityChange?.(...)` and are wired with `onValidityChange={setSectionValid}` at the call site.
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**: In `ResourcesSection`, typing a non-quantity string (e.g. `"abc"`) into "Storage size" sets `sizeValid = false` and renders `<div className="pt-1 text-xs text-danger">Invalid quantity...</div>` (line 96), but `ResourcesSection`'s destructured props are only `{ draft, onChange, template }` — it never calls `onValidityChange`, and `tabs/Settings.tsx`'s render for `"resources"` (`{section === "resources" && <ResourcesSection draft={draft} onChange={onChangeDraft} template={template} />}`) doesn't even pass the prop. The footer's Save button is `isDisabled={!dirty || !sectionValid || save.isPending}`, and `sectionValid` is reset to `true` on every tab switch (`onSelectionChange={(k) => { setSection(k); setSectionValid(true); }}`) and is otherwise only ever set to `false` by the three sections that do wire it. So while viewing Resources/Environment/Lifecycle, `sectionValid` stays `true` regardless of the section's own displayed error state, and Save remains clickable. The same gap repeats for `EnvVarsSection`'s duplicate/invalid env-var-name check and `LifecycleSection`'s grace-period/idle/wake-window numeric checks.
- **Expected**: A section showing a field-level validation error should block Save the same way Networking/Network-capture/Placement do (per the `SettingsTab` footer's own `sectionValid` design, and the CRD-side validation these mirror — e.g. Resources' storage quantity has no such guard even though `isValidQuantity` exists specifically to check it).
- **Actual**: A user can leave an invalid value in place (red hint visible) and still click "Save changes"; the malformed value is sent to `Servers.update()`. The `save` mutation's generic `onError` will surface *some* error if the backend/CRD-admission rejects it, so this is not silent data corruption, but it's a real UX regression versus the pattern the same file establishes elsewhere, and for fields the backend doesn't strictly validate (e.g., a duplicate env-var name is not necessarily rejected by the API) it could persist a genuinely broken configuration silently.

### C-web-routes-05: `reload()` in the Settings tab conflict banner is an unhandled, uncaught async call
- **Location**: web/src/routes/tabs/Settings.tsx:164-173 (`reload` definition), :238 (`<button onClick={reload} ...>`)
- **Category**: error-handling / ts-strictness
- **Suggested severity**: S3
- **Observation / repro**: `reload` is `async () => { const fresh = await Servers.get(name); ... }` with no `try/catch`, and it is passed directly as `onClick={reload}` — neither `await`ed nor `void`-prefixed, unlike every other async trigger in this same file (`save.mutate(...)`, which routes through `useMutation`'s `onError`). If `Servers.get(name)` rejects (network blip, the server having been deleted in the interim that produced the 409 conflict in the first place, a transient 5xx), the rejection is never handled: no `error` state is set, the "Server changed since you opened this page... Reload" banner stays exactly as it was with no indication the reload attempt failed, and the browser logs an unhandled promise rejection. This is the one async handler in the file that isn't `void`-wrapped or wrapped in a `useMutation`.
- **Expected**: Per CLAUDE.md rule 5 / `web/specs.md` Key Invariant 3 ("No floating promises... either `await` or `void` prefix every Promise"), and consistent with this file's own `save` mutation's error handling.
- **Actual**: A failed reload fails silently with no user-facing feedback; the user has no way to know the "Reload" click didn't work (though they can still retry, or use "Save changes", which re-fetches independently — so a workaround exists, hence S3 not S2).

### C-web-routes-06: A few `navigate()`/`nav()` calls are floating promises (inconsistent with the rest of the codebase)
- **Location**: web/src/routes/tabs/settings/Danger.tsx:60 (`onDeleted={() => navigate({ to: "/servers" })}`); web/src/routes/CreateServer.tsx:418 and :461 (`onPress={() => nav({ to: "/servers" })}`)
- **Category**: ts-strictness
- **Suggested severity**: S4
- **Observation / repro**: TanStack Router's `navigate()`/the `nav` alias returns `Promise<void>`. Every other call site in the reviewed scope wraps it (`web/src/routes/Dashboard.tsx:117`, `web/src/routes/Login.tsx:86` via `await`, `web/src/routes/ThemeSettings.tsx:417`, `web/src/routes/ServerDetail.tsx:245` all use `void navigate(...)`/`void nav(...)`), but these three sites call it bare inside an arrow function passed to a `() => void`-typed prop (`onDeleted`, `onPress`), so the returned promise is discarded by the callee (HeroUI's `Button`/the dialog component), not by this code — `@typescript-eslint/no-floating-promises` cannot see across that boundary, which is presumably why CI lint didn't catch it.
- **Expected**: Per Key Invariant 3 (no floating promises) and the codebase's own established convention at every other call site.
- **Actual**: Functionally low-risk (router navigation essentially never rejects in normal operation), but it is a real, unaddressed violation of the stated invariant and an inconsistency with the rest of the file/codebase; worth a one-line fix (`void nav(...)`/`void navigate(...)`) for consistency and to close the gap CI can't detect here.

### C-web-routes-07: `web/specs.md` states TypeScript 5.6.2; `web/package.json` pins `^6.0.3`
- **Location**: web/package.json:68 (`"typescript": "^6.0.3"`) vs. web/specs.md lines 5 and 1077 ("TypeScript 5.6 (strict)", "typescript@5.6.2 — strict type checking")
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**: `web/specs.md`'s header ("Build: Vite 5.4 + React 18.3 + TypeScript 5.6 (strict)") and its Dependencies section both cite TypeScript 5.6.2, but `web/package.json` already declares `"typescript": "^6.0.3"` (alongside `@typescript-eslint/eslint-plugin@^8.70.0` / `parser@^8.68.0`). This is a different axis from CLAUDE.md's tracked "TypeScript 7" blocked-upgrade item (which is about the `@typescript-eslint/parser` peer range not yet accepting `^7`) — TS 6.x has apparently already shipped and been adopted here without `web/specs.md` being updated to match. Noted here because it surfaced while cross-checking `web/specs.md` against the build tooling for this review chunk; `package.json` itself is outside the strict file-scope list for this chunk, so treat as opportunistic/lower-confidence context rather than a core finding.
- **Expected/Actual**: Spec text and installed toolchain version disagree; low real-world impact (doesn't affect route behavior) but worth a spec refresh.

## Questions (not findings)

- The `web/src/routes/tabs/settings/Networking.tsx` re-seeding logic that `web/specs.md` calls out as intentionally complex ("local field state for `addressPool` and `address`... re-seeded from props by two complementary mechanisms") was only skimmed (top ~50 of 1033 lines plus the `onValidityChange` wiring); I did not trace it closely enough to confirm or rule out a bug in that specific mechanism. Flagging as a gap rather than guessing.
- `web/src/routes/AdminSettings.tsx`'s "Add identity provider" `handleSubmitClick` (line 540: `if (adminList.length > 0 && !confirmingAdmin) { setConfirmingAdmin(true); return; } void submit();`) has the same *shape* of guard as C-web-routes-01, but I could not build as concrete a repro for it as for the role-mapping-overrides case, because I don't have visibility into whether the "Add provider" button remains reachable/clickable while `ConfirmAdminMappingDialog` (a component outside this chunk's file scope) is open, and there's no parallel live text-input feeding the same handler the way there is in `RoleMappingOverridesCard`. Recording as a question rather than a duplicate finding — worth a look by whoever owns `components/ui/ConfirmAdminMappingDialog.tsx` and `components/ui/Modal` focus-trap behavior.
- I did not deeply review `web/src/routes/tabs/Mods.tsx` (1263 lines, the largest file in scope) beyond grep-level checks (no `any`, no floating promises via the bare-identifier-handler heuristic, one confirmed-safe `JSON.parse`/try-catch). Given its size and the amount of domain logic (idList vs. file-based mod rendering, registry browsing, per-loader capability checks), a dedicated follow-up pass on this file specifically seems warranted before release sign-off.
- Similarly under-reviewed relative to their size: `web/src/routes/AdminSettings.tsx` (~1200 of 1906 lines not read — sections not covered: Notifications sinks, ModRegistries, BackupDestSection, General/Telemetry beyond the `useSectionForm` pattern), `web/src/routes/CreateServer.tsx` (~1100 of 1312 lines not read — the five step bodies' field-level validation), `web/src/routes/Users.tsx` (~600 of 757 lines — RolesTab, ServiceAccountsTab, IdpTab, NamespaceGrants), `web/src/routes/ThemeSettings.tsx` (880 lines, only function signatures grepped), `web/src/routes/Backups.tsx` (564 lines, only grepped for the historical T139 defect signatures).
