# Review: web

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `web/specs.md` (whole file); `CLAUDE.md` rules 3 and 5; `specs/done_016-user-theme-customization/spec.md` (FR-009) and `contracts/theme-ui.md` §4; `docs/architecture.md` "Multi-cluster (federation)"; `docs/install.md` "Registering an additional cluster". Design checks used `design-export/json/` and `design-export/screenshots/` file names only (`atqRh`, `xCJlu`, `EZFW0`, `O08uaD`, `b4eaUf`, `m5kOm4`, `s12HO`, `IdaU7`, `Xn5ns` all exist). No `.pen` file was opened.

held candidates: 4 (see OD-019)

## Scope reviewed

Read in full:
- `web/specs.md`, `web/index.html` (theme boot script), `web/package.json`, `web/eslint.config.js`, `web/tsconfig.json`, `web/nginx.conf.template` (lines 1-120)
- `web/src/main.tsx`, `web/src/router/tree.tsx`
- `web/src/lib/`: `api.ts`, `endpoints.ts`, `ws.ts`, `sse.ts`, `cluster.ts`, `auth.ts`, `config.ts`, `servers.ts`, `capabilities.ts`, `validation.ts`, `errors.ts`, `games.ts`, `verify.ts`, `events.ts`, `destinations.ts`, `ariaRouter.ts`, `useGameCodes.ts`, `media.ts`, `annotations.ts`, `utils.ts`, `useThemePreferences.ts`, `enforceUnauthenticatedTheme.ts`, `useDelayedLoading.ts`
- `web/src/routes/`: `Login.tsx`, `Share.tsx`, `Dashboard.tsx`, `Servers.tsx`, `ServerDetail.tsx`, `CreateServer.tsx`, `Modules.tsx`, `Cluster.tsx`, `Users.tsx`, `AdminSettings.tsx`, `ThemeSettings.tsx`, `Backups.tsx`, `AuditLog.tsx`, `AdminLogs.tsx`
- `web/src/routes/tabs/`: `Overview.tsx`, `Events.tsx`, `Console.tsx`, `ConsoleShell.tsx`, `useConsoleTerminal.ts`, `Logs.tsx`, `Files.tsx`, `Mods.tsx`, `Modpacks.tsx`, `Players.tsx`, `Backups.tsx`, `Settings.tsx`
- `web/src/routes/tabs/settings/`: `General.tsx`, `Version.tsx`, `Resources.tsx`, `Networking.tsx`, `EnvVars.tsx`, `Lifecycle.tsx`, `Backups.tsx`, `NetworkCapture.tsx`, `Placement.tsx`, `Access.tsx`, `ShareLinks.tsx`, `Danger.tsx`
- `web/src/components/`: `AppLayout.tsx`, `RequireRole.tsx`, `ClusterSelector.tsx`, `CaptureWidget.tsx`, `registry-browser.tsx`
- `web/src/components/server/`: `ServerActionsMenu.tsx`, `ServerActionsCard.tsx`, `ServerStatusCard.tsx`, `ServerSleepCard.tsx`, `CloneServerDialog.tsx`, `TransferServerDialog.tsx`, `WipeServerDialog.tsx`, `DeleteServerDialog.tsx`, `PortOverridesEditor.tsx`
- `web/src/components/backups/`: `BackupDetailDrawer.tsx`, `RestoreDialog.tsx`, `ScheduleForm.tsx`, `BackupFilters.tsx`, `BackupRow.tsx`, `RetentionFields.tsx`
- `web/src/components/modules/`: `ModuleCard.tsx`, `InstallDialog.tsx`, `UploadModuleDialog.tsx`, `ModuleSourcesPanel.tsx`
- `web/src/components/ui/`: `Sidebar.tsx`, `TopBar.tsx`, `NotificationsPanel.tsx`, `GlobalSearch.tsx`, `Breadcrumbs.tsx`, `ConfirmDialog.tsx`, `DropdownMenu.tsx`, `FilterPopover.tsx`, `PhaseChip.tsx`, `ResourceInput.tsx`, `SettingsNav.tsx`, `RoleEditorModal.tsx`, `SafeModeBanner.tsx`, `admin/EditUserDialog.tsx`, `admin/InviteUserDialog.tsx`

Read in part or at grep level only:
- `components/modules/SourceDialog.tsx` (submit and error paths only) and `components/modules/BuildModuleDialog.tsx` (lines 100-360)
- `lib/quantity.ts` (function heads), `lib/theme-derivation.ts`, `lib/theme-export.ts`, `lib/theme-sanitize.ts` (exports and call sites only)
- `web/src/types.ts` (grep for the types I needed, not read top to bottom)

Not read:
- `lib/gameIcon.ts`
- `routes/tabs/settings/Field.tsx`, `routes/tabs/settings/types.ts`
- `components/PageHeader.tsx`, `components/server/EventList.tsx`, `components/backups/ErrorBanner.tsx`
- The purely presentational `components/ui/` atoms (`AppShell`, `AppearanceToggle`, `AppLoadingSkeleton`, `AuditIntegrityBanner`, `CaptureWarningBanner`, `ConfirmAdminMappingDialog`, `ErrorBanner`, `ErrorCard`, `GameIcon`, `LoadingCard`, `Meter`, `PageHeader`, `ProvenanceBadge`, `RemovableGroupChip`, `SlackIcon`, `Sparkline`, `StatCard`, `admin/ResetPasswordDialog`)
- `styles/globals.css`, `src/test/**`, `*.test.*` (targeted greps only), `web/e2e/`

To verify claims about what the web client sends and gets back, I also read the server side of the calls: `api/internal/handlers/{shares,resources,tunnelcreds,lifecycle,audit,clusters,config}.go` (relevant functions), `api/internal/auth/ratelimit.go`, `api/internal/scope/{scope,cluster}.go`, `api/internal/db/shares.go` (`ListShareLinks`, `RevokeShareLink`), `api/cmd/main.go` (routes), `api/internal/ws/dialer.go:120-140`, `operator/api/v1alpha1/gameserver_types.go` (tunnel types, phases), the generated GameServer CRD, `operator/internal/controller/{module_controller,gameserver_idle,gameserver_status,backup_controller}.go` (the relevant parts), and `agent/internal/files/files.go` (grep).

## Method

I compared every file above against `web/specs.md` and the docs it cites, checked error handling and TypeScript strictness (CLAUDE.md rule 5: no unjustified `any`, every promise handled), looked for dead code, and checked pre-auth privacy (CLAUDE.md rule 3). For each web→API call I followed the path, namespace and cluster parameters through to the API handler that serves it, to see which server the call actually reaches. I didn't run `npx tsc --noEmit`, because `web/node_modules` isn't installed and I didn't install dependencies. No test or lint suite was run. `design.pen` was not opened.

## Observations (no finding)

- **Pre-auth privacy holds.** `Login.tsx` renders static product copy and the SSO labels from `/auth/providers` only. Its errors are "Invalid credentials" or "Network error" (`:88`). `Share.tsx` shows only the server name, address and players from the public view, and maps 404 and 429 to "Link not available". `AppLayout.tsx` shows the skeleton and then leaves via `location.assign("/login")` on a 401. The queries the shell starts before the redirect all return 401, so no cluster name, version, count or user data renders. The `index.html` boot guard skips cached prefs on `/login` and `/share/*`, and `enforceUnauthenticatedTheme()` pins the pink theme with no overlay.
- **TypeScript strictness.** No `any`, `@ts-ignore`, `@ts-expect-error` or `eslint-disable` appears in non-test sources (grep). `no-floating-promises` is on. `no-misused-promises` is not, which is how C-web-23 gets past lint.
- **CSRF.** `api.ts` sends `X-Gameplane-CSRF` on every method except GET, HEAD and OPTIONS. That covers more than the spec's "POST/PUT/PATCH", because DELETE is included. Raw-fetch helpers (`filesFetch`, `uploadBundle`, `Cluster.kubeconfig`, `ModuleBuilder.downloadArchive`) add `csrfHeaders()` themselves.
- **Reconnect backoff.** `ws.ts` backoff is `500 ms × 2^min(attempt, 6)`, capped at 30 s, and the attempt counter resets on open, as `web/specs.md:1015-1016` says.
- **Cluster threading.** WebSocket and SSE paths don't carry `?cluster=`. `web/specs.md:31` and `:1024` document that limitation.
- **Namespace threading.** The Players, Files, Console, Events, Capture, Share links, Danger zone, Access and the id-list mods editor tabs pass `ns` correctly. The tabs that don't are listed in C-web-02.
- **Capture retention.** The client-side ceiling for capture retention (604,800 s) matches the CRD maximum (`NetworkCapture.tsx:22`).
- **Share-link create body.** Share-link creation sends only `expiresAt` or `neverExpires: true`, never `expiresIn`, as the feature 017 contract requires (`ShareLinks.tsx:177-186`).
- **Idle-sleep reason strings.** The strings `ServerSleepCard.tsx` matches (`"asleep (no players)"`, `"this game reports no player count"`) match `operator/internal/controller/gameserver_idle.go:160,188,206`.

## Candidate findings

### C-web-01: "New file" with the name of an existing file empties that file without warning
- **Location**: `web/src/routes/tabs/Files.tsx:165-170` (`newFileMutation` → `Files.write(name, path, "", ns)`); `web/src/routes/tabs/Files.tsx:511-514` (the name check)
- **Category**: correctness
- **Suggested severity**: S1 (data loss; triage may decide a user-typed existing name makes this S2)
- **Observation / repro**:
  1. Open a server's Files tab at `/`, where `server.properties` exists.
  2. Click **New file**, type `server.properties`, and click **Create**.
  3. `Files.write` POSTs an empty body to `/files/write`. The agent opens the target with `os.Create`, which truncates it (`agent/internal/files/files.go:213`).
  4. The editor then opens the now-empty file.
- **Expected**: "New file" / "Create" refuses a name that's already in the listing (or asks before overwriting). The dialog already has `entries` in scope.
- **Actual**: the existing file's contents are silently replaced with an empty file. This is a web-side trigger for the agent's overwrite-in-place behaviour, separate from C-agent-01 (failed transfers).

### C-web-02: Server-detail tabs drop `?ns=` on many calls, so for a non-default-namespace server they read and write the same-named server in `gameplane-games`
- **Location**:
  - `web/src/routes/tabs/Mods.tsx:114` (install), `:140` (upload), `:153` (update all), `:170` (remove), `:1148` (version list)
  - `web/src/components/registry-browser.tsx:96`, `:115` (providers and search, no `ns` prop at all; this also affects the Modpacks browser)
  - `web/src/components/server/ServerActionsCard.tsx:156` (`runAction`)
  - `web/src/components/server/ServerStatusCard.tsx:37` (`Servers.status`)
  - `web/src/routes/tabs/Logs.tsx:188` (`Logs.downloadURL(name)`)
  - `web/src/routes/tabs/Backups.tsx:16` (`ns: _ns` discarded; `:42-47` create, `:59` filter, `:242-246` restore target)
  - `web/src/routes/tabs/Settings.tsx:165` (Reload), `:172` (cache key without `ns`)
  - `web/src/components/server/CloneServerDialog.tsx:71` (navigates to the clone without `ns`)
- **Category**: correctness
- **Suggested severity**: S2
- **Observation / repro**:
  1. With `GAMEPLANE_EXTRA_NAMESPACES=team-a`, own a server `mc` in `team-a`. It appears under "Shared with you" on `/servers`, and its link opens `/servers/mc?ns=team-a` (`Servers.tsx:483`).
  2. Mods tab: the list comes from `team-a/mc` (`Mods.tsx:94` passes `ns`). **Install mod**, **Upload**, **Update** and **Remove** call `/servers/mc/mods/...` with no `namespace`. `scope.Resolve` then targets `gameplane-games` (`api/internal/scope/scope.go:47-56`).
  3. Backups tab: **Back up now** creates a Backup in `gameplane-games` with `serverRef.name: mc`. The list shows `gameplane-games` backups whose `serverRef.name` is `mc`. **Restore** targets `gameplane-games/mc`.
  4. Logs → **Download** fetches `gameplane-games/mc`'s log. Quick actions and the Game status card do the same.
- **Expected**: every per-server call from the detail page carries the page's `ns`, as the other tabs already do.
- **Actual**: if no `gameplane-games/mc` exists, the calls 404 and these features are broken for that server. If one does exist (same-named servers in two namespaces), an admin installs mods on, backs up, restores over, or downloads logs from the wrong server.

### C-web-03: Settings "Save changes" writes the whole draft `spec` over the latest object, silently undoing concurrent changes such as Stop or Wipe
- **Location**: `web/src/routes/tabs/Settings.tsx:122-130` (`save`), `:291-319` (`mergeDraftOntoLatest`; `out.spec = structuredClone(draft.spec)` at `:303`)
- **Category**: correctness
- **Suggested severity**: S2
- **Observation / repro**:
  1. Open Settings → General and edit the description. The form is now dirty, and ServerDetail stops polling (`ServerDetail.tsx:68`).
  2. Click **Stop** in the page header. The API merge-patches `spec.suspend=true` (`api/internal/handlers/lifecycle.go:94-105`). **Wipe world** does the same (`:82`).
  3. Click **Save changes**. `save` GETs the latest object (`suspend: true`, fresh `resourceVersion`), copies `draft.spec` (`suspend: false`, taken when the page opened) over it, and PUTs it.
  4. The PUT succeeds and the server is un-suspended. Any other spec field changed since the page loaded is reverted too: capture enable/disable from the Capture tab, the mod id list, modpack pins, tunnel credential refs.
- **Expected** (`web/specs.md:420`): "Save button triggers PATCH /servers/{name} with all changed fields". `web/specs.md:896` says "changes are draft-until-save; conflict detection on reload". The in-code comment at `Settings.tsx:124-126` says the merge keeps "fields the UI doesn't model … from being clobbered".
- **Actual**: `spec` is replaced wholesale. The `resourceVersion` comes from the GET a moment earlier, so the 409 "Server changed since you opened this page" path (`:142-144`) is effectively unreachable. For a user without `captures:manage`, the save instead fails with 403 "modifying capture settings requires captures:manage permission" whenever `spec.capture` changed meanwhile (`resources.go:446-449`).

### C-web-04: The Create Server wizard can't create a server with a tunnel; the API refuses it, and the wizard reports it as a role problem
- **Location**: `web/src/routes/CreateServer.tsx:137-148` (the create body references `<name>-tunnel-auth` or a user-named Secret), `:350-371` (the credential PUT happens after the create), `:305-309` (403 shown as "Not permitted — Your role does not allow creating servers in this namespace."). Server side: `api/internal/handlers/resources.go:188-195` (create validation) and `:544-553` (tunnel `credentialsSecretRef` must name a Secret owned by that GameServer).
- **Category**: correctness
- **Suggested severity**: S2
- **Observation / repro**:
  1. In the wizard's Network step, tick **Enable tunnel**, choose frp, and enter a token, server address and one port mapping. Then **Create server**.
  2. The POST body has `spec.networking.tunnel.credentialsSecretRef.name = "<name>-tunnel-auth"`. That Secret doesn't exist yet: the wizard plans to create it afterwards with `setTunnelCredentials`.
  3. The API's create validation looks the Secret up, doesn't find a GameServer-owned Secret, and returns 403.
  4. The wizard shows "Not permitted / Your role does not allow creating servers in this namespace."
  5. The "Or use existing Secret (GitOps)" path fails the same way unless that Secret already has an owner reference naming the not-yet-created GameServer.
- **Expected** (`web/specs.md:775`): "the mutation saves the GameServer first, then saves tunnel credentials to a Kubernetes Secret". The create should succeed and the credentials be attached afterwards (for example, create without the ref, then `PUT :tunnel-credentials`, which sets the ref itself).
- **Actual**: every create with a tunnel enabled fails, and the error message blames the user's role. The code comment at `:133-136` ("the ref can point to a Secret that doesn't yet exist at create time") contradicts the API.

### C-web-05: Settings → Networking can't enable a tunnel on an existing server
- **Location**: `web/src/routes/tabs/settings/Networking.tsx:182-186` (the toggle builds `{ enabled: true, provider }` with no `credentialsSecretRef`), `:76-95` ("Save credential" never updates the draft), `:113-116` (a typed-but-unsaved credential passes validity); the whole-spec save in C-web-03. Server side: `api/internal/handlers/tunnelcreds.go:149-163` (merge-patches only `spec.networking.tunnel.credentialsSecretRef`); the CRD requires `tunnel.provider` (`operator/config/crd/gameplane.local_gameservers.yaml`, tunnel `required: [provider]`) and has the CEL rule "credentialsSecretRef is required when tunnel is enabled" (`operator/api/v1alpha1/gameserver_types.go:373`).
- **Category**: correctness
- **Suggested severity**: S2 (no in-dashboard workaround; kubectl only)
- **Observation / repro**:
  1. On a server without a tunnel, open Settings → Networking and toggle **Enable tunnel**. The draft becomes `tunnel: {enabled: true, provider: "frp"}`.
  2. Enter a token and click **Save credential**. The API creates the Secret, then merge-patches `tunnel.credentialsSecretRef` onto a live object that has no `tunnel.provider`, which the CRD rejects ("provider: Required").
  3. Alternatively, type the token but don't click Save credential. The validity check passes (`credentialValue.trim()` is non-empty). **Save changes** PUTs the draft without a `credentialsSecretRef`, and the CEL rule rejects it. The typed token is never sent.
  4. If the server already had a disabled tunnel block with a provider, step 2 succeeds, but the draft still has no ref, so step 3's PUT is rejected the same way. Toggling the switch rebuilds `tunnel` and drops any existing ref and frp config.
- **Expected** (`web/specs.md:905`): the Networking section can enable a tunnel with provider config and credentials.
- **Actual**: every path ends in a validation error.

### C-web-06: The public share page trips its own rate limit while a server wakes, then shows "Link not available"
- **Location**: `web/src/routes/Share.tsx:147` (polls every 2000 ms while in the starting state), `:119-122` (a neutral response flips the page to `invalid` and stops polling); `web/src/lib/api.ts:259-266` (429 mapped to the neutral response); `api/internal/auth/ratelimit.go:141` (`ShareLimiter` is 20 per minute, burst 20, per client IP)
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. Open `/share/<token>` for a sleeping server with a can-start link, and click **Start server**. That's 2 tokens spent (resolve plus start).
  2. The page polls `GET /shares/<token>` every 2 s: 0.5 tokens/s spent against a refill of 0.333 tokens/s.
  3. After about 110 s the bucket is empty. The next poll gets 429, `Shares.resolve` returns `{serverName: ""}`, and the page switches to "Link not available" and stops polling.
- **Expected**: `Share.tsx:336-337` itself says waking "usually takes a minute or two — this page updates on its own". The page should keep working for the whole wake, for example by polling at or below the limiter's refill rate, or by backing off on 429.
- **Actual**: any wake longer than about 2 minutes, or several viewers behind one IP, ends on the "invalid, expired, or revoked" message, even though the link is valid and the server is still starting.

### C-web-07: `openWS().close()` doesn't cancel a pending reconnect, so unmounted Console and Logs tabs leak live WebSockets
- **Location**: `web/src/lib/ws.ts:81` (`setTimeout(connect, delayMs)` is never stored or cleared), `:46-49` (`connect()` doesn't check `closedByUser`), `:106-110` (`close()`)
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. Open a new server. ServerDetail lands on the Logs tab (`ServerDetail.tsx:153-158`) while the pod-log stream is still refused, so `openWS` is in its backoff (up to 30 s).
  2. Switch to Overview. `LogsTab` cleanup calls `sock.close()`. The current socket is already closed, and the scheduled timer is left in place.
  3. When the timer fires, `connect()` opens a new WebSocket to `/ws/servers/<name>/logs/pod?from=start`. Once the pod is up it streams indefinitely into callbacks for an unmounted component. The Console tab (`useConsoleTerminal.ts:113-148`) leaks an open console session the same way.
  4. Each round of switching tabs during the backoff adds another connection.
- **Expected**: `close()` cancels any pending reconnect, and `connect()` does nothing after `close()`.
- **Actual**: orphaned connections to the API and agent stay open until the server closes them.

### C-web-08: Leaving the Settings tab with unsaved edits discards them without a prompt and leaves the server page frozen
- **Location**: `web/src/routes/tabs/Settings.tsx:112-114` (reports `dirty` up, with no cleanup that resets it); `web/src/routes/ServerDetail.tsx:61`, `:68` (`refetchInterval: settingsDirty ? false : 5_000`), `:297-299` (SettingsTab unmounts when the tab changes)
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. In Settings → General, change the description. `settingsDirty` becomes `true` and polling of `["server", name, ns]` stops.
  2. Click the **Overview** tab. SettingsTab unmounts, the edit is gone, and no prompt appears.
  3. `settingsDirty` stays `true`, so the header phase chip, uptime, the Start/Stop gating and the Overview resource cards (all from `gs`) stop updating. The SSE invalidation targets `["servers"]`, which doesn't match `["server", …]`. Things only resume after re-entering Settings or reloading.
- **Expected** (`web/specs.md:422`): "Navigation away without save prompts user (e.g., 'You have unsaved changes')". Polling should resume once the form is gone.
- **Actual**: the edits are lost silently and the page shows stale server state.

### C-web-09: Revoked share links still show as "Active"
- **Location**: `web/src/routes/tabs/settings/ShareLinks.tsx:131-137` (status is computed from `expiresAt` only), `:590-593`; server side: `api/internal/db/shares.go:205-208` (the list doesn't filter out revoked rows) and `api/internal/handlers/shares.go:60-66` (`shareResp` has no revoked field)
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. Settings → Share links → create a link with **No expiry**, then **Revoke** it.
  2. `RevokeShareLink` sets `revoked_at` and never deletes the row. The list reloads.
  3. The row is still listed, and `getLinkStatus(null)` returns "Active", with the Revoke button still offered.
- **Expected** (`web/specs.md:914`): "table shows … Status (Active/Expired/Revoked)".
- **Actual**: "Revoked" can never render (`statusColor`'s `"Revoked"` branch is dead). A revoked link looks active to the owner. The token itself is correctly rejected. The fix needs the API to return `revokedAt` (or filter those rows) and the UI to use it.

### C-web-10: The wizard's "Pin to node" and "GPU-enabled" placements add node labels that nothing sets, so the pod stays Pending
- **Location**: `web/src/routes/CreateServer.tsx:127-128` (`nodeSelector` `gameplane.local/pinned: "true"` / `gameplane.local/gpu: "true"`), `:773` (hint "Pins to a specific node (selectable after create).")
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. In Create Server → Configure, pick **Pin to node** (or **GPU-enabled**) and create.
  2. The GameServer gets `spec.nodeSelector` with that label, and the operator passes it through to the pod.
  3. No code, chart value or doc in the repo applies or mentions `gameplane.local/pinned` or `gameplane.local/gpu` (grep across Go, YAML, MD and TS finds only this file and its tests). The pod stays unschedulable.
  4. Nothing lets the user "select" a node after create: Settings → Placement edits only tolerations and affinity (`Placement.tsx`).
- **Expected**: either document the node labels an admin must apply, or have the choice produce a working placement. The "selectable after create" hint should match an actual control.
- **Actual**: a default cluster gets a server that never starts, and the only visible cause is the Pending pod.

### C-web-11: Safe mode entered with `?safe-mode=1` ends after the first in-app navigation, including the banner's own "Open Appearance Settings"
- **Location**: `web/src/lib/useThemePreferences.ts:55-68` (`isSafeModeActive` reads the URL or the sessionStorage flag, and the URL path never sets the flag), `:229-231`; `web/src/components/AppLayout.tsx:113`, `:144-146`, `:163-165`; `web/src/routes/ThemeSettings.tsx:291-294` (live preview calls `applyThemePreferences(draft)`)
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. Save custom CSS that breaks the layout, then load `/?safe-mode=1`. The banner shows and the overlay is suspended.
  2. Click **Open Appearance Settings**. That navigates to `/settings/theme`, and the query parameter is gone.
  3. ThemeSettings' preview effect calls `applyThemePreferences(draft)`. `isSafeModeActive()` is now false (no URL parameter, no sessionStorage flag), so `#gameplane-custom-css` is injected again. The broken CSS comes back on the page needed to fix it, while the banner still says "Safe Mode Active".
- **Expected** (016 FR-009, and `contracts/theme-ui.md` §4): the URL parameter is "the guaranteed path", and safe mode "suspends the custom CSS overlay for the session".
- **Actual**: only the keyboard shortcut and the login-page link (which set the sessionStorage flag) survive navigation.

### C-web-12: Several mutations drop their errors, so failed actions look like they did nothing
- **Location**:
  - `web/src/routes/Servers.tsx:65-69` (row Start/Stop/Restart/Wake)
  - `web/src/routes/ServerDetail.tsx:80-83` (header lifecycle buttons)
  - `web/src/components/server/ServerActionsCard.tsx:146-149` (Quick actions lifecycle)
  - `web/src/components/ui/RoleEditorModal.tsx:43-54` (create/update role)
  - `web/src/routes/AuditLog.tsx:35-46` (Export CSV)
  - `web/src/components/CaptureWidget.tsx:159-165`, `:481-487` (delete capture: `deleteMut.error` is never rendered)
- **Category**: error-handling
- **Suggested severity**: S3
- **Observation / repro**:
  1. As a viewer, click **Start** on a Servers row. The API returns 403, and nothing appears: no `onError`, no rendered error, and no global mutation error handler (`main.tsx:17-21`).
  2. In Users & RBAC → Roles, create a role with a name that already exists. The modal stays open with the button re-enabled and shows no message.
  3. Export CSV with a filter the API rejects: nothing happens.
- **Expected**: failures are shown, as the neighbouring flows do (for example `ErrorBanner` in Files, Players and CaptureWidget start/stop).
- **Actual**: failures are silent.

### C-web-13: The Admin Settings "Default namespace" setting has no effect
- **Location**: `web/src/routes/AdminSettings.tsx:258-263` (hint: "Where new GameServers land by default."); `api/internal/handlers/config.go:436-455` (validated and stored only)
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. Set General → Default namespace to `team-a` (listed in `GAMEPLANE_EXTRA_NAMESPACES`) and save.
  2. Create a server in the wizard. `Servers.create` sends no namespace (`endpoints.ts:111-112`), so the API uses `scope.DefaultNamespace` (`gameplane-games`).
  3. A repo-wide grep finds `defaultNamespace` read only by the validator. No handler, operator code or web code consumes it.
- **Expected**: the setting does what its hint says, or the field is removed or relabelled.
- **Actual**: new servers always land in `gameplane-games`.

### C-web-14: Modules "Manage sources" opens Admin Settings on General, not Module sources
- **Location**: `web/src/routes/Modules.tsx:146` (`<Link to="/admin" hash="modules">`); `web/src/routes/AdminSettings.tsx:80-87` (`initialSection` reads `?section=`, not the hash)
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. On `/modules`, click **Manage sources**. The URL becomes `/admin#modules`.
  2. The page opens on the General section.
- **Expected**: the Module sources section, as `docs/install.md` ("Modules → Manage sources (admin)") implies. `SettingsNav`'s own deep links use `?section=` (`ThemeSettings.tsx:417`).
- **Actual**: the General section opens.

### C-web-15: "Add cluster" in the cluster selector leads to a page that can't add a cluster
- **Location**: `web/src/components/ClusterSelector.tsx:54-56`, `:114-123`; `web/src/routes/Cluster.tsx:78-116` (the only actions are "Download kubeconfig" and "Add node")
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. Open the top-bar cluster selector and choose **Add cluster**. It navigates to `/cluster`.
  2. That page has no cluster registration UI. `docs/install.md` "Registering an additional cluster" documents only `kubectl apply` and a raw `POST /clusters`.
- **Expected**: `web/specs.md:99` says the selector keeps the "'Add cluster' action". Either a registration flow exists behind it, or the item is removed or relabelled.
- **Actual**: it's a dead end ("Add node" is a different operation).

### C-web-16: The Servers list hides every action for non-default-namespace servers, based on a stale comment
- **Location**: `web/src/routes/Servers.tsx:560-563` (`isSharedNonDefault`), `:450-454`, `:525-529`
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. A server in `team-a` appears under "Shared with you" with an empty Actions cell: no Start, Stop or Restart, and no menu.
  2. The comment says "detail route and lifecycle calls are namespace-blind". But `Servers.lifecycle` accepts `ns` (`endpoints.ts:117-118`), the detail route accepts `?ns=` (`tree.tsx:57-59`), and `ServerActionsMenu` already passes `gs.metadata.namespace`.
- **Expected** (`web/specs.md:204-206`): inline lifecycle buttons and an action menu per row.
- **Actual**: the actions are missing for those rows. The workaround is the detail page (subject to C-web-02).

### C-web-17: Settings sections show validation errors but still let Save send the invalid values
- **Location**: `web/src/routes/tabs/settings/Lifecycle.tsx:157-170` (the comment at `:162-164` says the check keeps a bad value from "sail[ing] past Save"); `web/src/routes/tabs/Settings.tsx:209-216` (Lifecycle, EnvVars and Resources get no `onValidityChange`); `web/src/routes/tabs/settings/EnvVars.tsx:69-70`; `web/src/routes/tabs/settings/Resources.tsx:36`
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. Settings → Lifecycle: enable idle auto-sleep and set "Sleep after" to `3`. The inline error "Must be an integer between 5 and 1440" shows.
  2. **Save changes** stays enabled. The PUT is rejected by CRD validation and the footer shows the server error.
  3. The same happens with an out-of-range grace period, a malformed wake window, an invalid or duplicate env name, or an invalid storage size.
- **Expected**: invalid sections disable Save, as Networking, Network capture and Placement already do through `onValidityChange`, and as the Lifecycle comment claims.
- **Actual**: Save stays enabled and the request fails at the API.

### C-web-18: Placement's JSON editors keep discarded text after "Discard"
- **Location**: `web/src/routes/tabs/settings/Placement.tsx:16-21` (`rawTol`/`rawAff` seeded once); `web/src/routes/tabs/Settings.tsx:155-162` (`reset` replaces the draft only)
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. Settings → Placement: edit the affinity JSON, then click **Discard**. The draft goes back to the baseline and Save is disabled.
  2. The editor still shows the edited JSON.
  3. Typing one more character re-applies the whole discarded text.
- **Expected**: the editors show the draft after a Discard.
- **Actual**: they show stale text, and the next keystroke brings back the discarded edit.

### C-web-19: The Invite user dialog reopens with the previous user's details, including the password
- **Location**: `web/src/routes/Users.tsx:124-130` (a successful create only sets `inviting=false`); `web/src/components/ui/admin/InviteUserDialog.tsx:71-76` (field state), `:118-126` (only `handleClose` clears it, and it isn't called on programmatic close)
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. Users & RBAC → **Invite user**. Enter username `alice` and a password, then **Create user**. It succeeds and the dialog closes.
  2. Click **Invite user** again. `alice`, her display name, email and the masked initial password are pre-filled.
  3. Submitting again gives a conflict.
- **Expected**: a fresh form each time the dialog opens.
- **Actual**: the previous entry is still there.

### C-web-20: The Users → "Identity providers" tab says IdP configuration isn't available yet
- **Location**: `web/src/routes/Users.tsx:742-748`
- **Category**: docs-drift (UI copy)
- **Suggested severity**: S4
- **Observation / repro**:
  1. Users & RBAC → Identity providers shows: "OIDC identity providers configured in Helm values appear here. UI configuration is tracked for v1.1."
  2. Nothing is listed there. Meanwhile, Admin Settings → Authentication already adds, enables and deletes OIDC, Google and GitHub providers (`AdminSettings.tsx:287-424`).
- **Expected**: the tab lists providers or points to Admin Settings → Authentication.
- **Actual**: it contradicts the shipped feature.

### C-web-21: System logs never show a timestamp for API server lines
- **Location**: `web/src/routes/AdminLogs.tsx:258-262` (reads `parsed.ts`); `api/cmd/main.go:543` (the API logs with `slog.NewJSONHandler`, whose time key is `time`)
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. Open Admin → System logs → API server.
  2. Each JSON line has `"time":…`, not `"ts"`, so the timestamp column is empty and `time=…` is dumped among the trailing structured fields. Operator (zap) lines do use `ts`.
- **Expected**: timestamps render for both components.
- **Actual**: API server lines have no timestamp column.

### C-web-22: The ban confirm button reads "Baning…"
- **Location**: `web/src/routes/tabs/Players.tsx:377` (`` `${verb}ing…` `` with `verb = "Ban"`)
- **Category**: correctness (UI copy)
- **Suggested severity**: S4
- **Observation / repro**:
  1. Players → Ban a player → confirm.
  2. The button shows "Baning…" while the request is pending.
- **Expected**: "Banning…".
- **Actual**: "Baning…".

### C-web-23: Promise-returning handlers are passed without `void` or error handling (CLAUDE.md rule 5)
- **Location**: `web/src/routes/tabs/Settings.tsx:238` (`onClick={reload}`; `reload` has no try/catch); `web/src/routes/tabs/settings/Danger.tsx:60` (`onDeleted={() => navigate(...)}`); `web/src/routes/CreateServer.tsx:418`, `:461` (`onPress={() => nav(...)}`)
- **Category**: ts-strictness
- **Suggested severity**: S4
- **Observation / repro**:
  1. In the conflict state, click **Reload** while the GET fails. For a non-default-namespace server it always fails, because `Servers.get(name)` has no `ns` (C-web-02).
  2. The rejection is unhandled and no message appears.
  3. `no-floating-promises` doesn't catch returned promises in handler props, and `no-misused-promises` isn't enabled (`eslint.config.js:61-66`).
- **Expected**: CLAUDE.md rule 5: "Handle all promises: `await` or prefix with `void`". Elsewhere the code does this (for example `ServerDetail.tsx:245`).
- **Actual**: these four handlers return unhandled promises.

### C-web-24: Dead or unreachable code
- **Location**:
  - `web/src/components/RequireRole.tsx:17` (`RequireRole`, used only in tests)
  - `web/src/lib/auth.ts:23` (`hasRole`, unused)
  - `web/src/lib/theme-export.ts:226` (`themeExportToUpdate`, tests only)
  - `web/src/lib/endpoints.ts:810-822` (the path-builder `Shares` object; the app imports `Shares` from `api.ts`), `:383-384` (`Restores.remove`), `:349` (`Schedules.get`), `:493` (`Users.getPreferences`), `:395-396` (`BackupDestinations.get`), `:751` (`Modules.get`), all without callers
  - `web/src/routes/AuditLog.tsx:290-291` (the `/modules/sources` branch can't match, because `m("/modules")` on the line before catches it, so source changes are labelled "module")
  - `web/src/routes/CreateServer.tsx:171` (the "meaningful config" guard is always true, since `tunnelBase` always has `enabled` and `provider`)
  - `web/src/routes/Servers.tsx:661` (`onAct: _onAct` passed to `ServerCard` and never used)
- **Category**: dead-code
- **Suggested severity**: S4
- **Observation / repro**:
  1. `grep -rn` each symbol under `web/src` excluding `*.test.*` and `src/test/`.
  2. Each has no non-test caller, or the branch can't be reached.
- **Expected**: removed or wired up.
- **Actual**: unused code that implies features exist (restore cancel, schedule edit) when they don't; see C-web-26.

### C-web-25: `web/specs.md` says shipped features are "not implemented", and still describes superseded slice-era states
- **Location**:
  - `web/specs.md:139`: "The Theme Settings modal, SafeModeBanner, keyboard shortcut, login-page safe-mode link, custom-colors derivation utility (`deriveCustomThemeTokens`), and the export/import utilities are … NOT implemented". The code has all of them: `routes/ThemeSettings.tsx` (route `/settings/theme`, `tree.tsx:117-121`), `components/ui/SafeModeBanner.tsx`, `AppLayout.tsx:126-139` (Ctrl+Shift+Alt+T), `Login.tsx:232-243`, `lib/theme-derivation.ts:185`, `lib/theme-export.ts`. The line "nothing consumes [ThemeExport] yet" is also false (`ThemeSettings.tsx:301`).
  - `web/specs.md:916`: "Configurable expiry (feature 017, target contract — UI not yet implemented …)". `ShareLinks.tsx:44-51`, `:244-266` implement the six choices, the "No expiry" and long-lived warnings, and the "Never" column.
  - `web/specs.md:101` and `:192`: "Dashboard.tsx … narrowed scope to loading skeleton and empty frame only"; "a `// TODO(slice-2+)` comment marks where it would be added". `Dashboard.tsx` renders the full stats, fleet, resources, activity and backups content, and has no such TODO.
  - `web/specs.md:623-629` (T139): says `Backups.tsx` is "unparseable" (curly quotes, `SelectItem`, `classNames`, `emptyContent`) and `Modules.tsx:141` uses `asChild`. The current files have none of these (grep for `SelectItem|emptyContent|classNames=|asChild` in `src` finds nothing), and `Modules.tsx:146` uses `buttonVariants` on a `Link`.
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. Quote each spec line above next to the cited code.
- **Expected**: specs.md describes the shipped state.
- **Actual**: readers are told the theme UI and the 017 UI don't exist, and that two route files are broken.

### C-web-26: `web/specs.md` describes UI and helpers the code doesn't have
- **Location**:
  - `web/specs.md:551` "Shared header: StatCard summary", `:556` "Row actions: download, restore …, delete", `:562` "edit (ScheduleForm)", `:566` "cancel restore", `:465-467` (StatCard/FilterPopover "in `web/src/routes/Backups.tsx` line 23/25"). `routes/Backups.tsx` has no StatCard, FilterPopover, download, schedule edit or restore cancel.
  - `:585` "Schedule picker: daily/weekly/cron radio buttons", `:588`/`:598` "RetentionFields … numeric input + unit dropdown: days/weeks/months". `ScheduleForm.tsx` has a type select and a cron input, and `RetentionFields.tsx` holds restic keep-N buckets.
  - `:521` "Tab/section switcher (Catalog / Installed / Upload)". `Modules.tsx` has source and category chips only.
  - `:491` "name uniqueness checked via API query", `:495` "Displays version notes/changelog", `:510` "address valid CIDR if provided". `CreateServer.tsx` `validateStep` does none of these.
  - `:912` "Danger zone — Clone, transfer owner, wipe data …, delete server". `Danger.tsx` has no Clone.
  - `:374` "pod-node-selector builder" and `:910` "Node selector labels". `Placement.tsx` has tolerations and affinity only.
  - `:708-722` and `:995-1001`: `auth.ts` "OIDC provider detection, logout" / "`getCurrentUser()`"; `servers.ts` "`isServerRunning(phase)`"; `games.ts` "icon URLs, console protocol detection (RCON/Satisfactory/Battleye)"; `config.ts` "API base URL, feature flags"; `errors.ts` "Custom error types". None of these exist. `games.ts` holds category helpers, `config.ts` holds admin-config types and hooks, and `errors.ts` holds two string helpers.
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. Quote each spec line above next to the cited file.
- **Expected**: specs.md lists only what exists.
- **Actual**: specs.md describes features the code doesn't have.

### C-web-27: Stale versions and line references in `web/specs.md` (plus a new aspect in CLAUDE.md)
- **Location**:
  - `web/specs.md:5` "Vite 5.4 + React 18.3 + TypeScript 5.6", and the Dependencies list `:1061-1095` (react 18.3.1, router 1.75.0, query 5.59.0, tailwindcss 3.4.13, lucide 0.445.0, xterm 5.5.0, typescript 5.6.2, vite 5.4.8, vitest 2.1.1, eslint 9.11.1, …). `web/package.json` has react ^19.3.0, @tanstack/react-router ^1.170.38, @tanstack/react-query ^5.103.1, tailwindcss ^4.3.3, lucide-react ^1.47.0, @xterm/xterm ^6.0.0, typescript ^6.0.3, vite ^8.3.0, vitest ^5.0.0, eslint ^9.39.5.
  - `CLAUDE.md:78` and `:262` say "React 18". This is a new aspect, separate from the tracked repo-map module count.
  - `web/specs.md:895` and `:987` cite "`web/src/lib/api.ts:127-175`" (Captures is at `:139-187`) and "`ServerDetail.tsx:278`" (the capture tab is at `:296`).
  - `web/specs.md:60-73` (T057) cite `Login.tsx` "lines 26–233", "error copy at line 94", "Marketing panel (lines 196–231)", "comment (lines 23–25)", "Only `Input`, `Label`, `InputGroup` … (lines 17–18)", "line 262 in SSOButtons". Today the error copy is at `:88`, the comment at `:27-29`, the marketing panel at `:257-287`, and SSOButtons at `:295-313`. The imports are `Input, Label, Button, Alert, Spinner` (`:14-20`); there's no `InputGroup`.
  - `web/specs.md:210` "Tabbed interface … routing to nine sub-views: Overview, Events, Console, Logs, Files, Mods, Modpacks, Players, Backups, Capture, Settings" lists 11. `:334` "All nine tabs". `:358` "(11 sections below)", but `Settings.tsx:53-66` has 12 including Share links.
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. Compare each quoted value with `web/package.json` or the current file line.
- **Expected**: current versions and line references.
- **Actual**: stale versions and line references.

## Questions (not findings)

- Q-web-remote-cluster: held (OD-019)
- **Share page start flow.** The page always shows **Start server** for a sleeping server, because the public response has no `canStart`. A view-only link then goes Start → "Starting…" → (first poll sees Suspended) → "asleep-viewonly". With a can-start link, a first poll 2 s after a successful start that still sees `Suspended` also shows "ask the server owner to start it". Is that intended?
- **Overview "External Address".** When a tunnel endpoint is first in `status.endpoints`, "External Address" shows the tunnel host, the same as the Tunnel row, and the LoadBalancer address moves to "Cluster Address" (`Overview.tsx:91-93`, `:228-248`). Does that match design node `EZFW0`? Only the file name was checked.
- **Module source delete.** `ModuleSourcesPanel` deletes a ModuleSource on one click with no confirmation (`ModuleSourcesPanel.tsx:116`, `:224-232`), while backup destinations require typing the name. Is that intended?
- **`repoRef.key`.** The operator ignores `repoRef.key` and always reads the Secret keys `repo` and `password` (`operator/internal/controller/backup_controller.go:654-658`). The per-server Backups tab sends `key: "url"` (`tabs/Backups.tsx:46`) while the other paths send `"repo"`, and ScheduleForm exposes a "Repo secret · key" input that has no effect. Should the field be removed, or should the operator honour it?
- **`useMe` retries.** The comment in `lib/auth.ts:7-9` says "Total attempts … Three". With `retry: (failureCount) => failureCount < 3`, TanStack Query v5 retries three times, which is four attempts. I couldn't confirm this against the installed library because `node_modules` isn't present.
- **Wizard YAML preview.** The Preview pane (`CreateServer.tsx:1269-1281`) shows `resources: {cpu, memory, storage}`, but the submitted spec has `resources.requests/limits` and `storage.size`. Is the preview meant to be illustrative only?
