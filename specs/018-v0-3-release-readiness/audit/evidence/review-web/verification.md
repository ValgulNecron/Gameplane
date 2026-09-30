# T045 web chunk: independent verification (opus)

Verifier input: `notes.md` in this directory (opus reviewer), candidates C-web-01 to C-web-27. Held candidates for this component are verified separately, off-git (OD-019). The older `notes-routes.md` in this directory was not an input to this pass.

Method: I read every cited location against master `13a859ff`. `web/`, `api/`, `operator/` and `agent/` are identical between this branch and master. For each candidate I followed the call to the code on the other side: the API handler (`api/internal/handlers/{resources,lifecycle,tunnelcreds,shares,config}.go`, `api/internal/scope/scope.go`, `api/internal/auth/ratelimit.go`, `api/internal/db/shares.go`, `api/internal/ws/dialer.go`), the agent (`agent/internal/files/files.go`) or the CRD (`operator/api/v1alpha1/gameserver_types.go` and the generated `operator/config/crd/gameplane.local_gameservers.yaml`). Where a test pins the behaviour, I read it too (`Settings.test.tsx`, `ws.test.ts`). I checked `web/specs.md`, `specs/done_016-user-theme-customization/`, `specs/done_017-share-link-expiry/` and `audit/findings.md` for decisions that allow the behaviour, or findings that already track it. None of the 27 is tracked in `findings.md`. F-030 (PR #421) covers only the module count in CLAUDE.md, not the "React 18" wording in C-web-27. `web/node_modules` isn't installed, so I didn't run `npx tsc --noEmit`. I ran no test or lint suite and changed no repo file other than this one.

Severity follows research R3. I changed one rating from what the reviewer suggested: C-web-03 goes from S2 to S3, because Save itself works and the lost change can be applied again. I also corrected one premise: I kept C-web-16, but its guard isn't simply stale. C-web-01 stays at S1 because R3 classes silent, unrecoverable data loss as S1. Its trigger needs a user to type an existing file's name, and triage may want to note that.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-web-01 | kept | S1 | Confirmed. `newFileMutation` POSTs an empty body to `/files/write` (`Files.tsx:165-170`). The API proxies that call straight to the agent (`api/internal/ws/dialer.go:56`), and the agent truncates the file with `os.Create` (`agent/internal/files/files.go:213`). The name check (`Files.tsx:511-514`) rejects only empty names, `/`, `.` and `..`. Nothing compares the name with `entries`. |
| C-web-02 | kept | S2 | Confirmed at every cited site. The endpoint helpers all take `ns` (`endpoints.ts:143-231`, `:725`), but these callers don't pass it. The Backups, Schedules and Restores clients take no namespace at all (`endpoints.ts:323-384`). With no `namespace` parameter, `scope.Resolve` falls back to `gameplane-games` (`scope.go:47-56`), and so does the generic list handler (`resources.go:110-135`). A clone is created in the source's namespace (`lifecycle.go:191-248`), but the dialog then navigates without `ns`. This only matters on installs that host servers outside `gameplane-games` (`GAMEPLANE_EXTRA_NAMESPACES` plus a RoleBinding, `charts/gameplane/templates/api.yaml:66-69`), and there the dashboard has no workaround. |
| C-web-03 | kept | S3 | Confirmed. `mergeDraftOntoLatest` replaces `spec` wholesale (`Settings.tsx:303`). The header Stop button stays available while Settings is open (`ServerDetail.tsx:200-207`), and it merge-patches only `spec.suspend` (`lifecycle.go:94-114`). The next Save PUTs the old `suspend: false` with a fresh `resourceVersion`, so the 409 path never fires. The existing test (`Settings.test.tsx:103`) checks only the annotations and `resourceVersion`. Downgraded from S2: Save works, and the reverted change can be made again. |
| C-web-04 | kept | S2 | Confirmed. The create body references `<name>-tunnel-auth` (`CreateServer.tsx:137-148`). Create validation (`resources.go:188-196`, check 5 at `:544-553`) calls `isServerOwnedSecret`. That returns false both when the Secret is missing and, on a create, when the Secret's owner reference carries a UID (`:576`), which Kubernetes always requires. The 403 is then shown as a role problem (`CreateServer.tsx:305-309`). Creating without the ref doesn't work either: the CEL rule at `gameserver_types.go:373` rejects `enabled: true` with no `credentialsSecretRef`. So the API refuses every wizard create with a tunnel enabled. |
| C-web-05 | kept | S2 | Confirmed. The toggle builds `{enabled, provider}` and drops any ref and frp config (`Networking.tsx:182-186`). "Save credential" patches only `tunnel.credentialsSecretRef` onto the live object (`tunnelcreds.go:149-163`), which fails when the live object has no `tunnel.provider` (the CRD requires it, `gameplane.local_gameservers.yaml:1552`). An unsaved, typed credential passes the validity check (`Networking.tsx:113-116`), and the PUT is then rejected by the CEL rule. Together with C-web-04, the dashboard has no way to turn on a tunnel. |
| C-web-06 | kept | S3 | Confirmed. The page polls every 2 s (`Share.tsx:147`). `ShareLimiter` is 20/min with a burst of 20 per IP, and it covers both GET and start (`ratelimit.go:141`, `shares.go:38-43`). The client maps 429 to the neutral response (`api.ts:259-266`), and the poller turns that into "invalid" (`Share.tsx:119-122`). That leaves about 108 s of polling after resolve plus start, against the page's own "a minute or two" (`Share.tsx:336-337`). |
| C-web-07 | kept | S3 | Confirmed. The reconnect timer isn't stored (`ws.ts:81`), and `connect()` doesn't check `closedByUser` (`:46-49`), so a timer that fires after `close()` opens a new socket. That socket's own `onclose` stops further retries, so each abandoned handle leaks at most one live connection. The existing test "close() suppresses reconnect" (`ws.test.ts:84`) closes an open socket, not one that's waiting in backoff. |
| C-web-08 | kept | S3 | Confirmed. The `onDirtyChange` effect (`Settings.tsx:112-114`) has no cleanup. `ServerDetail` keeps `settingsDirty` true after the tab unmounts (`:61`, `:68`), and the SSE invalidation key is `["servers"]` (`lib/sse.ts:64-67`), which doesn't match `["server", name, ns]`. No `useBlocker` or unsaved-changes prompt exists in `web/src`, although `web/specs.md:422` asks for one. |
| C-web-09 | kept | S3 | Confirmed. `ListShareLinks` returns revoked rows (`db/shares.go:199-207`). `shareResp` has no revoked field (`handlers/shares.go:60-66`), and `getLinkStatus` looks only at `expiresAt` (`ShareLinks.tsx:131-137`). The token itself is still refused on lookup. |
| C-web-10 | kept | S3 | Confirmed. `git grep` for `gameplane.local/pinned` or `gameplane.local/gpu` outside `CreateServer.tsx` and its test finds nothing. The operator copies `spec.nodeSelector` onto the pod template (`gameserver_controller.go:1498`). No web control edits `nodeSelector` after create (`Placement.tsx` covers tolerations and affinity only). |
| C-web-11 | kept | S3 | Confirmed. Nothing turns the URL parameter into the sessionStorage flag: only the shortcut (`AppLayout.tsx:131`) and the login link set it. The banner's button navigates to `/settings/theme` (`AppLayout.tsx:164`), and ThemeSettings' preview effect then calls `applyThemePreferences` (`ThemeSettings.tsx:293`). That call checks `isSafeModeActive()`, which is now false, and injects the overlay again. A workaround exists: open `/settings/theme?safe-mode=1` directly. |
| C-web-12 | kept | S3 | Confirmed. The six cited mutations have no `onError`, none of their `.error`/`.isError` values is read anywhere, and the `QueryClient` has no `MutationCache` handler (`main.tsx:17-21`). |
| C-web-13 | kept | S3 | Confirmed. A repo-wide `git grep -i defaultNamespace` finds the field only in the config validator (`config.go:438-455`), in web types and in fixtures. `Servers.create` sends no namespace (`endpoints.ts:111-112`). |
| C-web-14 | kept | S4 | Confirmed. The link sets a hash (`Modules.tsx:146`), but `initialSection` reads only `?section=` (`AdminSettings.tsx:80-87`). |
| C-web-15 | kept | S4 | Confirmed. `handleAddCluster` navigates to `/cluster` (`ClusterSelector.tsx:54-56`). The web `Clusters` client has only `list` (`endpoints.ts:292-295`), and `Cluster.tsx` has no registration UI. |
| C-web-16 | kept | S4 | Confirmed that the actions are missing, but the premise needs a correction. The comment is half right: the detail route and `ServerActionsMenu` do handle namespaces now. But this page's own row `act` mutation still calls `Servers.lifecycle(name, verb)` with no `ns` (`Servers.tsx:65-69`). So the guard currently stops the rows from targeting `gameplane-games`. The fix is to pass `gs.metadata.namespace` to `act`, then drop the guard. |
| C-web-17 | kept | S4 | Confirmed. The setters write the invalid values into the draft (`Lifecycle.tsx:70-86`, `:148-154`), and only Networking, Network capture and Placement report `onValidityChange` (`Settings.tsx:209-216`). The API error does show in the footer, so the impact is S4. |
| C-web-18 | kept | S4 | Confirmed. `rawTol`/`rawAff` are seeded once (`Placement.tsx:16-21`), and `reset` touches only the draft (`Settings.tsx:155-162`). |
| C-web-19 | kept | S4 | Confirmed. `InviteUserDialog` is always mounted (`Users.tsx:330`). Its field state is cleared only in `handleClose` (`InviteUserDialog.tsx:118-126`), which a successful create never calls (`Users.tsx:124-130`). |
| C-web-20 | kept | S4 | Confirmed. `IdpTab` is static text (`Users.tsx:742-748`), while Admin Settings → Authentication manages providers. |
| C-web-21 | kept | S4 | Confirmed. The API logs with `slog.NewJSONHandler` (`api/cmd/main.go:543`), whose time key is `time`. `AdminLogs.tsx:259-262` reads only `ts`. |
| C-web-22 | kept | S4 | Confirmed (`Players.tsx:348`, `:377`). |
| C-web-23 | kept | S4 | Confirmed. `reload` is async with no try/catch (`Settings.tsx:164-173`) and is passed as `onClick` (`:238`). The three `navigate`/`nav` arrow handlers return their promises (`Danger.tsx:60`, `CreateServer.tsx:418`, `:461`). `no-misused-promises` isn't enabled (`eslint.config.js:55-66`). This breaks CLAUDE.md rule 5, a project rule, so it isn't a style preference. |
| C-web-24 | kept | S4 | Confirmed with `grep -rn --exclude='*.test.*'` under `web/src` (excluding `src/test/`). None of the listed symbols has a non-test caller. The `/modules/sources` branch (`AuditLog.tsx:291`) can't be reached, because the API route really is `/modules/sources` (`module_sources.go:3-5`) and `m("/modules")` matches it first, so source changes get the label "module". The tunnel guard (`CreateServer.tsx:171`) is always true. `_onAct` is unused (`Servers.tsx:661`). spec.md:43 names dead code as review scope. |
| C-web-25 | kept | S4 | Confirmed. The quoted lines (`web/specs.md:139`, `:916`, `:101`, `:192`) contradict files that exist. The T139 note (`:618-629`) is dated 2026-09-06 and describes, in the present tense, defects that a grep no longer finds. |
| C-web-26 | kept | S4 | Confirmed with spot checks. `routes/Backups.tsx` has no StatCard or FilterPopover. `ScheduleForm.tsx` has no daily/weekly radios. `RetentionFields.tsx` holds `keepLast`/`keepDaily`/… counts. `Danger.tsx` has no Clone. The `lib/` exports don't include `getCurrentUser`, `isServerRunning` or custom error types. |
| C-web-27 | kept | S4 | Confirmed against `web/package.json:28-70`, `CLAUDE.md:78`, `:262`, `lib/api.ts:139` (Captures) and `ServerDetail.tsx:296`. The "React 18" wording isn't covered by F-030/PR #421. |

### C-web-01

**Location:** `web/src/routes/tabs/Files.tsx:165-170` (`newFileMutation`), `web/src/routes/tabs/Files.tsx:511-514` (the name check in `NamePromptDialog`); `agent/internal/files/files.go:202-224` (`write`, `os.Create` at `:213`).

**Repro / observation:**
1. On a test server, open the Files tab at `/`, where `server.properties` exists and has content.
2. Click **New file**, type `server.properties`, and click **Create**.
3. The browser sends `POST /servers/<name>/files/write?path=/server.properties` with an empty body (`endpoints.ts:673-683`). The API forwards it unchanged (`api/internal/ws/dialer.go:56`), and the agent opens the path with `os.Create`, which truncates it.
4. The editor opens the file, now 0 bytes. `kubectl exec` into the pod and `wc -c` the file to confirm it's 0 bytes.

**Expected:** "New file" refuses a name that's already in the current listing (`entries` is in scope in `FilesTab`), or asks before overwriting.

**Actual:** The existing file is silently replaced by an empty one. It can't be recovered without a backup.

### C-web-02

**Location:** `web/src/routes/tabs/Mods.tsx:114`, `:140`, `:153`, `:170`, `:1148`; `web/src/components/registry-browser.tsx:95-96`, `:115`; `web/src/components/server/ServerActionsCard.tsx:156`; `web/src/components/server/ServerStatusCard.tsx:36-37`; `web/src/routes/tabs/Logs.tsx:188`; `web/src/routes/tabs/Backups.tsx:16`, `:22-47`, `:59-61`, `:242-246`; `web/src/routes/tabs/Settings.tsx:165`, `:172`; `web/src/components/server/CloneServerDialog.tsx:71`. Client helpers without a namespace parameter: `web/src/lib/endpoints.ts:323-384` (Backups, Schedules, Restores).

**Repro / observation:**
1. On a test install, add `team-a` to `GAMEPLANE_EXTRA_NAMESPACES` on the API Deployment and grant the API a matching RoleBinding (`charts/gameplane/templates/api.yaml:66-69`). Create a GameServer `mc` in `team-a` owned by the admin. It appears under "Shared with you" on `/servers`, and its link opens `/servers/mc?ns=team-a` (`Servers.tsx:483`).
2. Mods tab: the list query passes `ns` (`Mods.tsx:94`), but Install, Upload, Update all, Remove and the version picker call `Servers.installMod(name, body)`, `uploadMod(name, file)`, `removeMod(name, mod)` and `modVersions(name, id, provider)` without it. The browser's network panel shows no `namespace=` query on those requests.
3. With no namespace, `scope.Resolve` returns `gameplane-games` (`api/internal/scope/scope.go:47-56`). The requests 404, or hit `gameplane-games/mc` if a server by that name exists there.
4. Backups tab: **Back up now** POSTs `/backups` with `serverRef.name: mc` and no namespace. The generic create and list handlers use `gameplane-games` (`resources.go:183-199`, `:110-135`). The list shows `gameplane-games` backups whose `serverRef.name` is `mc`, and **Restore** creates a Restore in `gameplane-games`.
5. Logs → **Download**, the Quick actions card, the Game status card, and Settings → conflict **Reload** all omit `ns` in the same way. After **Clone**, the dialog navigates to `/servers/<clone>` without `?ns=`, though the clone is created in `team-a` (`lifecycle.go:191-248`).

**Expected:** Every per-server call from the detail page carries the page's `ns`, as the list, Files, Players, Console, Events, Capture, Share links, Danger and Access calls already do.

**Actual:** For a server outside `gameplane-games`, these features 404. If a same-named server exists in `gameplane-games`, they act on that server instead: mods installed on it, backups taken from it, restores written over it. After a conflict, Settings → **Reload** can load that other server's spec into the draft.

### C-web-03

**Location:** `web/src/routes/tabs/Settings.tsx:122-130` (`save`), `web/src/routes/tabs/Settings.tsx:291-319` (`mergeDraftOntoLatest`, `out.spec = structuredClone(draft.spec)` at `:303`); `api/internal/handlers/lifecycle.go:94-114` (`patchSuspend`), `:77-89` (wipe).

**Repro / observation:**
1. Open a Running server's Settings → General and change the description. The form becomes dirty.
2. Click **Stop** in the page header. The API merge-patches `spec.suspend: true`, and the server stops.
3. Click **Save changes**. `save` GETs the latest object (`suspend: true`, new `resourceVersion`), sets `out.spec` to the draft's spec (`suspend: false`, captured before the Stop), and PUTs it.
4. The PUT succeeds, and `kubectl get gameserver <name> -o jsonpath='{.spec.suspend}'` prints `false`. The server starts again. The same thing happens to any spec field changed elsewhere while the form was dirty, for example a capture toggle from the Capture tab, or another admin's edit.

**Expected:** Save applies only the fields the user changed (`web/specs.md:420`: "with all changed fields"), or it detects the concurrent change (`web/specs.md:896`: "conflict detection on reload"). The in-code comment at `:124-126` says the merge keeps unmodelled fields from being clobbered.

**Actual:** The whole draft `spec` overwrites the latest one, so concurrent spec changes are reverted silently, and the 409 conflict path (`:142-144`) never fires. For a user without `captures:manage`, the save fails with 403 whenever `spec.capture` changed in the meantime (`resources.go:445-449`).

### C-web-04

**Location:** `web/src/routes/CreateServer.tsx:130-148` (the create body builds `credentialsSecretRef`), `:350-372` (the credential PUT runs after the create), `:305-309` (403 shown as "Not permitted"); `api/internal/handlers/resources.go:188-196`, `:544-553`, `:564-584` (`isServerOwnedSecret`); `operator/api/v1alpha1/gameserver_types.go:373` (CEL rule).

**Repro / observation:**
1. In **Create server** → Network, tick **Enable tunnel**, choose frp, and enter a token, a server address and one port mapping. Click **Create server**.
2. `POST /servers` carries `spec.networking.tunnel.credentialsSecretRef.name: "<name>-tunnel-auth"`. That Secret doesn't exist yet: the wizard creates it afterwards with `setTunnelCredentials`.
3. `validateAndProtectGameServer` check 5 calls `isServerOwnedSecret`. The Secret is missing, so it returns false, and the API answers 403 "tunnel credentials secret reference … is not permitted".
4. The wizard shows "Not permitted / Your role does not allow creating servers in this namespace."
5. The "use existing Secret (GitOps)" path fails the same way. On a create there is no GameServer UID, and any real owner reference carries a UID, so the check at `:576` skips it.
6. A body with the ref left out fails too: the CEL rule `!self.enabled || has(self.credentialsSecretRef)` rejects it with 422.

**Expected:** `web/specs.md:775`: "the mutation saves the GameServer first, then saves tunnel credentials to a Kubernetes Secret". A tunnel-enabled create succeeds and gets its credentials attached. One way is to create with the tunnel disabled, PUT `:tunnel-credentials`, then enable it.

**Actual:** Every wizard create with a tunnel enabled is refused, and the error blames the user's role. The comment at `:133-136` says the ref may name a Secret that doesn't exist yet, which contradicts the API.

### C-web-05

**Location:** `web/src/routes/tabs/settings/Networking.tsx:182-186` (the toggle), `:76-95` (`saveCredentialMutation` never updates the draft), `:111-116` (validity accepts a typed but unsaved credential); `api/internal/handlers/tunnelcreds.go:149-163`; `operator/config/crd/gameplane.local_gameservers.yaml:1552` (`required: [provider]`); `operator/api/v1alpha1/gameserver_types.go:373`.

**Repro / observation:**
1. On a server with no tunnel, open Settings → Networking and switch on **Enable tunnel**. The draft becomes `tunnel: {enabled: true, provider: "frp"}`. Fill in the frp address and one port mapping.
2. Enter a token and click **Save credential**. The API creates `<name>-tunnel-auth`, then merge-patches `spec.networking.tunnel.credentialsSecretRef` onto the live object. The live object has no `tunnel.provider`, so the apiserver rejects the patch ("provider: Required value"), and the section shows the error.
3. Instead, type the token and click **Save changes** without **Save credential**. Validity passes because `credentialValue` is non-empty. The PUT carries no `credentialsSecretRef`, so the CEL rule rejects it ("credentialsSecretRef is required when tunnel is enabled"). The typed token is never sent.
4. If the live object already had a disabled tunnel block with a provider (set with kubectl), step 2 succeeds. The draft still has no ref, though, so step 3's PUT is rejected. Switching the toggle off and on rebuilds `tunnel`, dropping any ref and frp config.

**Expected:** `web/specs.md:905`: the Networking section can turn a tunnel on, with provider config and credentials.

**Actual:** Every dashboard path ends in a validation error. Together with C-web-04, a tunnel can be set up only with kubectl.

### C-web-06

**Location:** `web/src/routes/Share.tsx:147` (2 s poll), `:117-123` (neutral response → `invalid`, polling stops); `web/src/lib/api.ts:255-266` (429 mapped to neutral); `api/internal/auth/ratelimit.go:141` (`ShareLimiter`, 20/min, burst 20, per client IP); `api/internal/handlers/shares.go:38-43`.

**Repro / observation:**
1. Create a can-start share link for a sleeping test server, where the pod needs more than about two minutes to become Running (a cold image pull, or a modded server).
2. Open `/share/<token>` signed out and click **Start server**. The resolve and the start spend 2 of the 20 tokens.
3. The page polls `GET /shares/<token>` every 2 s. That spends 0.5 tokens/s against a refill of 0.33 tokens/s, so the bucket runs out after about 108 s.
4. The next poll gets 429, `Shares.resolve` returns `{serverName: ""}`, and the page switches to "Link not available / This link may be invalid, expired, or revoked" and stops polling. The network panel shows the 429.

**Expected:** The page stays on "waking up" for the whole wake, as its own copy promises (`Share.tsx:336-337`: "usually takes a minute or two — this page updates on its own"). For example, it could poll at or below the limiter's refill rate, or back off on 429 instead of treating it as invalid.

**Actual:** A wake that takes longer than about 110 s, or several viewers behind one IP or one reverse proxy, ends on the invalid-link message while the link is valid and the server is still starting.

### C-web-07

**Location:** `web/src/lib/ws.ts:81` (`setTimeout(connect, delayMs)` is not stored or cleared), `:46-49` (`connect()` doesn't check `closedByUser`), `:106-110` (`close()`).

**Repro / observation:**
1. Create a new server. ServerDetail opens on the Logs tab (`ServerDetail.tsx:153-158`) while the pod-log stream is still refused, so `openWS` is waiting in backoff (up to 30 s).
2. Switch to Overview. `LogsTab` cleanup calls `sock.close()` (`Logs.tsx:99`). The current socket is already closed, and the scheduled timer is left in place.
3. When the timer fires, `connect()` opens a new WebSocket to `/ws/servers/<name>/logs/pod`. The network panel (WS filter) shows it opening after the tab has gone. Once the pod is up it keeps streaming into the unmounted tab's callbacks. The Console tab (`useConsoleTerminal.ts:113-148`) leaks a console session the same way.
4. Each tab switch during a backoff window adds one more connection. The retry loop on each leaked socket ends when that socket closes, because `closedByUser` is then true.

**Expected:** `close()` cancels any pending reconnect, and `connect()` does nothing after `close()`.

**Actual:** Orphaned log and console connections stay open until the server side closes them.

### C-web-08

**Location:** `web/src/routes/tabs/Settings.tsx:112-114` (reports `dirty` up, with no cleanup), `web/src/routes/ServerDetail.tsx:61`, `:68`, `:297-299`; `web/src/lib/sse.ts:64-67`.

**Repro / observation:**
1. Open Settings → General and change the description. `settingsDirty` becomes `true`, and polling of `["server", name, ns]` stops (`refetchInterval: false`).
2. Click the **Overview** tab. SettingsTab unmounts, the edit is discarded, and no prompt appears.
3. `settingsDirty` stays `true`, so the header phase chip, the uptime, the Start/Stop gating and the Overview cards stop updating. SSE watch events invalidate only `["servers"]`, which doesn't match. Polling comes back only after going back into Settings, clicking a lifecycle button, or reloading the page.

**Expected:** `web/specs.md:422`: "Navigation away without save prompts user". Polling resumes once the form is gone.

**Actual:** The edits are lost without a prompt, and the server page shows stale state.

### C-web-09

**Location:** `web/src/routes/tabs/settings/ShareLinks.tsx:128-137` (`getLinkStatus`), `:586-593`; `api/internal/db/shares.go:199-207` (`ListShareLinks`, "active and revoked alike"), `api/internal/handlers/shares.go:60-66`, `:218-227`.

**Repro / observation:**
1. Settings → Share links: create a link with **No expiry**, then **Revoke** it.
2. `RevokeShareLink` sets `revoked_at` and keeps the row (`db/shares.go:289-301`). The list reloads.
3. The row is still listed with status "Active", and **Revoke** is still offered. `GET /servers/<name>:shares` returns the row with no revoked field.

**Expected:** `web/specs.md:914`: the table shows "Status (Active/Expired/Revoked)".

**Actual:** "Revoked" never renders (the `statusColor` branch at `:125` is dead), so a revoked link looks active to its owner. The token is correctly refused on lookup. The fix needs the API to return `revokedAt` (or filter those rows out) and the UI to use it.

### C-web-10

**Location:** `web/src/routes/CreateServer.tsx:126-128`, `:772-774` (the "selectable after create" hint).

**Repro / observation:**
1. Run `git grep -n "gameplane.local/pinned\|gameplane.local/gpu"`. The only matches are `CreateServer.tsx` and its test. No chart value, doc or operator code labels nodes this way.
2. In Create server → Configure, choose **Pin to node** or **GPU-enabled**, and create.
3. The GameServer gets `spec.nodeSelector`, which the operator copies onto the pod template (`operator/internal/controller/gameserver_controller.go:1498`). `kubectl get pod` shows the pod stuck Pending with a "didn't match Pod's node affinity/selector" event.
4. Settings → Placement edits only tolerations and affinity, so the dashboard has no way to remove the selector or choose a node.

**Expected:** The option either produces a working placement, or the docs name the label an admin must apply. The "selectable after create" hint matches a real control.

**Actual:** On a default cluster, the server never starts.

### C-web-11

**Location:** `web/src/lib/useThemePreferences.ts:55-68` (`isSafeModeActive`), `:229-231`; `web/src/components/AppLayout.tsx:113`, `:126-139`, `:144-146`, `:163-165`, `:210-216`; `web/src/routes/ThemeSettings.tsx:291-294`.

**Repro / observation:**
1. Save custom CSS that visibly changes the layout, then load `/?safe-mode=1`. The banner shows, and `document.getElementById("gameplane-custom-css")` is `null`.
2. Click **Open Appearance Settings** on the banner. The app navigates to `/settings/theme`, and the query parameter is gone. `sessionStorage.getItem("gameplane-safe-mode")` is `null`, because only the shortcut and the login link set it.
3. ThemeSettings' preview effect calls `applyThemePreferences(draft)`. `isSafeModeActive()` is now false, so `#gameplane-custom-css` is injected again, while the banner still says "Safe Mode Active".

**Expected:** 016 FR-009 makes the URL parameter "the guaranteed path", and safe mode "suspends the custom CSS overlay for the session" (`specs/done_016-user-theme-customization/spec.md:128`, `contracts/theme-ui.md:132-142`).

**Actual:** Safe mode entered through the URL ends at the first in-app navigation, including the banner's own link to the page needed for the fix. Workaround: open `/settings/theme?safe-mode=1` directly.

### C-web-12

**Location:** `web/src/routes/Servers.tsx:65-69`; `web/src/routes/ServerDetail.tsx:80-83`; `web/src/components/server/ServerActionsCard.tsx:146-149`; `web/src/components/ui/RoleEditorModal.tsx:43-54`; `web/src/routes/AuditLog.tsx:35-46`; `web/src/components/CaptureWidget.tsx:159-165`, `:481-487`; `web/src/main.tsx:17-21` (no global mutation error handler).

**Repro / observation:**
1. As a viewer, click **Start** on a Servers row (or in the detail header, or in Quick actions). The API returns 403 (network panel), and the page shows nothing.
2. In Users & RBAC → Roles, create a role with a name that already exists. The API returns 409, the modal stays open with the button enabled again, and no message appears.
3. Export CSV from the audit log while the API is failing: nothing happens.
4. Delete a capture while the capture sidecar is unreachable: the dialog stays open with no message.

**Expected:** Failures are shown, as the neighbouring flows do (for example `ErrorBanner` in Files, Players, and the CaptureWidget enable/start/stop paths).

**Actual:** These failures are silent.

### C-web-13

**Location:** `web/src/routes/AdminSettings.tsx:258-263`; `api/internal/handlers/config.go:436-455`; `web/src/lib/endpoints.ts:111-112`; `api/internal/scope/scope.go:47-56`.

**Repro / observation:**
1. Run `git grep -n -i defaultNamespace -- ':!*_test.go' ':!*.test.ts*' ':!specs/' ':!*.pen'`. It's read only by the validator (`config.go:449-453`), web types, and fixtures. No handler or operator code consumes it.
2. Set Admin Settings → General → Default namespace to an allowed extra namespace and save.
3. Create a server in the wizard. `POST /servers` carries no namespace, so the API uses `gameplane-games`. `kubectl get gameserver -A` shows the new server in `gameplane-games`.

**Expected:** The setting does what its hint says ("Where new GameServers land by default."), or the field is removed or relabelled.

**Actual:** New servers always land in `gameplane-games`.

### C-web-14

**Location:** `web/src/routes/Modules.tsx:146`; `web/src/routes/AdminSettings.tsx:80-87`.

**Repro / observation:**
1. As an admin on `/modules`, click **Manage sources**. The URL becomes `/admin#modules`.
2. The page opens on the General section, because `initialSection` reads only `?section=`.

**Expected:** The Module sources section opens (`/admin?section=modules`, the form ThemeSettings' nav already uses).

**Actual:** The General section opens.

### C-web-15

**Location:** `web/src/components/ClusterSelector.tsx:54-56`, `:114-123`; `web/src/routes/Cluster.tsx` (actions are "Download kubeconfig" and "Add node" only); `web/src/lib/endpoints.ts:292-295`.

**Repro / observation:**
1. Open the top-bar cluster selector and choose **Add cluster**. It navigates to `/cluster`.
2. That page has no cluster registration UI, and the web client has no call to register a cluster. `docs/install.md` "Registering an additional cluster" documents only `kubectl apply` and a raw `POST /clusters`.

**Expected:** `web/specs.md:99` keeps an "Add cluster" action. Either a registration flow exists behind it, or the item is removed or relabelled (for example, "Cluster overview" with a link to the docs).

**Actual:** The item leads to a page that can't add a cluster.

### C-web-16

**Location (corrected):** `web/src/routes/Servers.tsx:560-563` (`isSharedNonDefault` and its comment), `:450-454`, `:525-529`, and the row mutation `:65-69`.

**Repro / observation:**
1. With a server in an extra namespace (see C-web-02 step 1), open `/servers`. Its row under "Shared with you" has an empty Actions cell: no Start, Stop or Restart, and no menu.
2. The guard's comment says "detail route and lifecycle calls are namespace-blind". The detail route is no longer namespace-blind: it accepts `?ns=` (`router/tree.tsx:57-59`), and `Servers.tsx:483` links with it. `ServerActionsMenu` also passes the namespace. The page's own `act` mutation, though, calls `Servers.lifecycle(args.name, args.verb)` with no `ns`, so taking the guard away on its own would send those clicks to `gameplane-games`.

**Expected:** `web/specs.md:204-206`: inline lifecycle buttons and an action menu on each row, with the calls carrying the row's namespace.

**Actual:** Those rows have no actions. The detail page header works as a workaround, because its `act` passes `ns`.

### C-web-17

**Location:** `web/src/routes/tabs/settings/Lifecycle.tsx:70-86`, `:148-170` (the comment at `:162-164`); `web/src/routes/tabs/Settings.tsx:209-216`, `:262-266`; `web/src/routes/tabs/settings/EnvVars.tsx:69-70`; `web/src/routes/tabs/settings/Resources.tsx:36`.

**Repro / observation:**
1. Settings → Lifecycle: turn on idle auto-sleep and set "Sleep after" to `3`. The inline error appears, and the draft holds `afterMinutes: 3`.
2. **Save changes** stays enabled. Clicking it sends the PUT, the CRD rejects it (Minimum 5, `gameserver_types.go:170-171`), and the footer shows the API error.
3. An out-of-range grace period, an invalid or duplicate env name, or an invalid storage size behave the same way.

**Expected:** Invalid sections disable Save, as Networking, Network capture and Placement do through `onValidityChange`, and as the Lifecycle comment says.

**Actual:** Save stays enabled, and the request fails at the API.

### C-web-18

**Location:** `web/src/routes/tabs/settings/Placement.tsx:16-21`; `web/src/routes/tabs/Settings.tsx:155-162`.

**Repro / observation:**
1. Settings → Placement: edit the affinity JSON, then click **Discard**. The draft goes back to the baseline, and Save is disabled.
2. The editor still shows the edited JSON.
3. Type one more character. `handleAffChange` parses the whole editor text into the draft, which brings the discarded edit back.

**Expected:** After Discard, the editors show the draft.

**Actual:** They show stale text, and the next keystroke brings back the discarded edit.

### C-web-19

**Location:** `web/src/routes/Users.tsx:124-130`, `:330-345`; `web/src/components/ui/admin/InviteUserDialog.tsx:71-76`, `:118-126`.

**Repro / observation:**
1. Users & RBAC → **Invite user**. Enter username `alice` and a password, then **Create user**. The user is created, and the dialog closes because `inviting` is set to `false`.
2. Click **Invite user** again. `alice` and the masked password are still filled in. `handleClose` isn't called on a programmatic close, and the component stays mounted.
3. Submitting again gives a 409.

**Expected:** A fresh form each time the dialog opens.

**Actual:** The previous entry, including the password, is still there.

### C-web-20

**Location:** `web/src/routes/Users.tsx:742-748`.

**Repro / observation:**
1. Open Users & RBAC → Identity providers. It reads: "OIDC identity providers configured in Helm values appear here. UI configuration is tracked for v1.1." No list follows.
2. Admin Settings → Authentication already adds, enables and deletes OIDC, Google and GitHub providers (`AdminSettings.tsx:287-424`).

**Expected:** The tab lists providers or points to Admin Settings → Authentication.

**Actual:** The copy contradicts the shipped feature, and nothing is listed.

### C-web-21

**Location:** `web/src/routes/AdminLogs.tsx:259-262`, `:273-276`; `api/cmd/main.go:542-544`.

**Repro / observation:**
1. Open Admin → System logs → API server.
2. Each line is slog JSON with `"time":…`. The parser reads only `ts`, so the timestamp column is empty, and `time=…` appears among the trailing structured fields. Operator lines (zap, `ts`) render correctly.

**Expected:** Timestamps render for both components.

**Actual:** API server lines have no timestamp.

### C-web-22

**Location:** `web/src/routes/tabs/Players.tsx:348`, `:377`.

**Repro / observation:**
1. Players → Ban a player → confirm.
2. While the request is pending, the button reads "Baning…".

**Expected:** "Banning…".

**Actual:** "Baning…".

### C-web-23

**Location:** `web/src/routes/tabs/Settings.tsx:164-173`, `:238`; `web/src/routes/tabs/settings/Danger.tsx:60`; `web/src/routes/CreateServer.tsx:418`, `:461`; `web/eslint.config.js:55-66`.

**Repro / observation:**
1. Get Settings into the conflict state (a 409 on Save), then make `GET /servers/<name>` fail (stop the API, or use a server outside `gameplane-games`, see C-web-02). Click **Reload**.
2. The async `reload` rejects with no handler: the console shows "Uncaught (in promise)", and no message appears.
3. `onDeleted={() => navigate(...)}` and `onPress={() => nav(...)}` also return promises that nothing handles. `no-floating-promises` doesn't flag a promise returned from a handler prop, and `no-misused-promises` isn't enabled.

**Expected:** CLAUDE.md rule 5: "Handle all promises: `await` or prefix with `void`", as `ServerDetail.tsx:245` does.

**Actual:** Four handlers leave their promises unhandled. The Reload one fails silently.

### C-web-24

**Location:** `web/src/components/RequireRole.tsx:17`; `web/src/lib/auth.ts:23`; `web/src/lib/theme-export.ts:226`; `web/src/lib/endpoints.ts:810-822`, `:349`, `:383-384`, `:395-396`, `:493`, `:751`; `web/src/routes/AuditLog.tsx:290-291`; `web/src/routes/CreateServer.tsx:171`; `web/src/routes/Servers.tsx:661`.

**Repro / observation:**
1. Run `grep -rn --exclude='*.test.*' "<symbol>" web/src | grep -v '^web/src/test/'` for `RequireRole` (the component; `RequirePermission` from the same file is used), `hasRole`, `themeExportToUpdate`, `getPreferences`, `Modules.get(`, `Schedules.get(`, `BackupDestinations.get(` and `Restores.remove(`. Each has only its definition. The app imports `Shares` from `@/lib/api` (`Share.tsx:9`, `ShareLinks.tsx:34`), never from `endpoints.ts`.
2. `AuditLog.tsx:290` `m("/modules")` matches `/modules/sources/...` (the real route, `api/internal/handlers/module_sources.go:3-5`) before the `:291` branch, so module-source changes are labelled "module".
3. `CreateServer.tsx:171`: `tunnelBase` always has `enabled` and `provider`, so `Object.keys(tunnelBase).length > 1` is always true.
4. `Servers.tsx:661`: `ServerCard` takes `onAct` as `_onAct` and never uses it.

**Expected:** Unused code is removed or wired up, and the audit label branch can be reached.

**Actual:** Dead code suggests features that don't exist (restore cancel, schedule edit), and module-source audit events are mislabelled.

### C-web-25

**Location:** `web/specs.md:139`, `:916`, `:101`, `:192`, `:618-629`.

**Repro / observation:**
1. `web/specs.md:139` says the Theme Settings UI, SafeModeBanner, shortcut, login safe-mode link, `deriveCustomThemeTokens` and the export/import utilities are "NOT implemented". `web/src/routes/ThemeSettings.tsx`, `components/ui/SafeModeBanner.tsx`, `lib/theme-derivation.ts` and `lib/theme-export.ts` exist. The shortcut is at `AppLayout.tsx:126-139`, and `Login.tsx` imports `SAFE_MODE_SESSION_KEY`.
2. `:916` says the feature 017 expiry UI is "not yet implemented". `ShareLinks.tsx` has the six choices and the no-expiry and long-lived warnings (`:245`, `:264`).
3. `:101` and `:192` describe `Dashboard.tsx` as a loading skeleton with a `// TODO(slice-2+)`. The file renders full content, and `grep -n TODO web/src/routes/Dashboard.tsx` finds nothing.
4. `:618-629` (T139, 2026-09-06) say `Backups.tsx` is "unparseable as written" and `Modules.tsx:141` uses `asChild`. `grep -rn "SelectItem\|emptyContent\|classNames=\|asChild" web/src --include='*.tsx'` finds nothing, and `Modules.tsx:146` uses `buttonVariants` on a `Link`.

**Expected:** specs.md describes what shipped, with historical notes marked as resolved.

**Actual:** It says the theme UI and the 017 UI don't exist and that two route files are broken.

### C-web-26

**Location:** `web/specs.md:374`, `:465-467`, `:491`, `:495`, `:510`, `:521`, `:551`, `:556`, `:562`, `:566`, `:585`, `:588`, `:598`, `:708-722`, `:910`, `:912`, `:995-1001`.

**Repro / observation:**
1. `grep -c "StatCard\|FilterPopover" web/src/routes/Backups.tsx` prints 0. The route has no row download, no schedule edit and no restore cancel.
2. `components/backups/ScheduleForm.tsx` has no daily/weekly radios, and `RetentionFields.tsx` holds `keepLast`/`keepHourly`/`keepDaily`/`keepWeekly`/`keepMonthly` counts, not a days/weeks/months duration.
3. `Modules.tsx` has source and category chips, with no Catalog/Installed/Upload switcher. `CreateServer.tsx` `validateStep` does no name-uniqueness query, shows no changelog, and does no CIDR validation.
4. `routes/tabs/settings/Danger.tsx` has no Clone, and `Placement.tsx` has no node-selector builder.
5. The `lib/` exports: `auth.ts` exports `useMe`, `hasRole`, `can`; `servers.ts` exports `countByState` and `phaseGroups`; `games.ts` holds category helpers; `config.ts` holds admin-config hooks; `errors.ts` holds `errorText` and `errorTextWithStatus`. None of the helpers the spec names exists.

**Expected:** specs.md lists only what exists.

**Actual:** It describes UI and helpers the code doesn't have.

### C-web-27

**Location:** `web/specs.md:5`, `:1061-1095`, `:895`, `:987`, `:60-73`, `:210`, `:334`, `:358`; `CLAUDE.md:78`, `:262`.

**Repro / observation:**
1. `web/specs.md:5` says "Vite 5.4 + React 18.3 + TypeScript 5.6". `web/package.json` has `react ^19.3.0`, `typescript ^6.0.3`, `vite ^8.3.0`, `vitest ^5.0.0`, `tailwindcss ^4.3.3`, `@xterm/xterm ^6.0.0`, `lucide-react ^1.47.0` and `eslint ^9.39.5` (lines 28-70).
2. `CLAUDE.md:78` and `:262` say "React 18". The F-030 fix branch (`docs/018-claude-md-gp-module`) doesn't touch this wording.
3. `web/specs.md:895` cites `api.ts:127-175` and `ServerDetail.tsx:278`. `Captures` is at `lib/api.ts:139`, and the capture tab renders at `ServerDetail.tsx:296`.
4. The T057 line references to `Login.tsx` (`:60-73`) no longer match the file.
5. `:210` says "nine sub-views" and then lists 11. `:358` says "11 sections", but `Settings.tsx:53-66` has 12.

**Expected:** Current versions, counts and line references.

**Actual:** Stale versions, counts and line references.
