# Install-Time Configuration: Quickstart & Validation Guide

This guide shows how to validate the install-time configuration feature (feature 006) once implemented. It covers hands-on scenarios for both **storage class configuration** and **OIDC role mappings**, with exact commands and expected outputs.

**This is not an implementation guide.** Refer to [spec.md](./spec.md) for requirements (FR-###, SC-###) and [data-model.md](./data-model.md) for data-structure details.

---

## Prerequisites

### Environment Setup

1. **Kubernetes Cluster**
   - A kind cluster (for local testing) or kubelab cluster (for integration testing)
   - kubectl context configured and authenticated
   - Helm 3.13+ installed

2. **StorageClasses**
   - Create two test StorageClasses on your cluster:
     ```bash
     # Local NVMe storage (fast, low-latency)
     kubectl apply -f - <<EOF
     apiVersion: storage.k8s.io/v1
     kind: StorageClass
     metadata:
       name: fast-local-nvme
     provisioner: kubernetes.io/no-provisioner
     volumeBindingMode: WaitForFirstConsumer
     EOF

     # Standard network storage (default)
     kubectl apply -f - <<EOF
     apiVersion: storage.k8s.io/v1
     kind: StorageClass
     metadata:
       name: standard-network
     provisioner: local.csi/provisioner
     allowVolumeExpansion: true
     EOF
     ```

3. **OIDC Provider**
   - **For local testing**: Use a test OIDC issuer running in the cluster or localhost.
     - Example: Deploy Keycloak, mock an OIDC endpoint, or use an existing Okta/Azure AD test tenant.
     - The feature does NOT depend on external connectivity; a fake OIDC issuer (served locally) is sufficient for validation.
   - **For integration testing (CI)**: The e2e suite uses a fake OIDC issuer (see **Testing** section below). No external IdP is required.
   - Required OIDC scopes: `openid`, `email`, `profile`
   - Required claims in ID token: `sub`, `email`, `name`, and a group/role claim (e.g., `groups`)

4. **Helm Chart**
   - Ensure the Gameplane Helm chart (at `charts/gameplane/`) has been updated with:
     - `operator.gameDataStorage.storageClassName` value
     - `api.oidc.groupsClaim`, `api.oidc.defaultRole`, and `api.oidc.roleMappings.*` values
   - Chart version: 0.2.0-beta.5 or later (where 006-install-time-config is merged)

---

## Hybrid Role Mapping Model (Helm Seeds, DB Overlay)

**This is the binding design for OIDC role mappings** (resolves the FR-007 / SC-007 tension): Helm values *seed* the role mappings so an OIDC-only install works with no admin account and no bootstrap-admin run. Once an admin exists (via bootstrap-admin or a Helm-seeded admin mapping), they can override individual role lists through the **existing admin auth-config endpoint** — no new table, no new resource.

**Storage.** There is no `oidc_role_mappings` table and no migration. The overlay is one optional sibling field, `helmOverride`, added next to `providers` inside the *existing* `"auth"` row of the `config` table (the same row `api/internal/auth/registry.go:175-185` already reads as `{"providers":[...]}`):

```json
{
  "providers": [ /* unchanged */ ],
  "helmOverride": {
    "roleMappings": {
      "admin": ["gameplane-admins"],
      "operator": ["gameplane-ops-v2"],
      "viewer": []
    }
  }
}
```

`helmOverride` is **not** a provider entry — it never appears in `providers`, and the reserved-name guard on `"helm"` (`api/internal/handlers/auth_provider_secret.go:36`) is untouched: the synthetic `helm` provider's own issuer/clientID/scopes/seed mappings stay immutable through the dashboard exactly as before. Each of the three role keys (`admin`, `operator`, `viewer`) is independently optional; omitting a key (or setting it to `null`) leaves that role at its Helm-seeded value. Setting a key to `[]` is a valid override meaning "nobody maps to this role via the overlay."

**Merge algorithm — per-role list replacement**, resolved independently for each of the three roles: if `helmOverride.roleMappings` supplies a non-nil list for a role, that list *replaces* the Helm-seeded list for that role; otherwise the Helm-seeded list stands. This merge happens *before* role resolution, producing an effective `ProviderPolicy` that the **existing, unmodified** `computeRole` (`api/internal/auth/oidc.go:121`) is then called against exactly as today — its admin > operator > viewer tie-break is untouched.

**Consequence worth stating plainly**: because the merge is per-role, a user who matches an *overridden* viewer-role group and also matches a *Helm-seeded* admin-role group still resolves to **admin** — the most-privileged match wins regardless of which layer (DB overlay or Helm seed) each matching list came from. This is not a bug; it falls straight out of "merge first, then run the same computeRole."

**Why no restart is needed (SC-007)**: the DB-provider hash cache does **not** apply to the `helm` provider. `OIDCFor` (`api/internal/auth/registry.go:224-232`) short-circuits for `name == HelmProviderName` and returns `r.legacy` immediately — before `snapshot()`, before any row-hash, before the cache. `r.legacy` is a single `*OIDC` built once at process startup (`api/cmd/main.go:127`) and held for the process lifetime, so the hash-based cache invalidation governs DB providers only and never the helm path. Instead, the override must be (and is) read at **login time** on the helm path: `registry.go:152-153` already calls `legacy.AttachStore(store)`, so the helm provider has the store. During role resolution for the helm provider, `helmOverride` is read fresh from the `"auth"` config row and merged per-role over the Helm-seeded lists (`effectiveHelmPolicy`) before the existing, unmodified `computeRole` runs. Because that read happens on every login attempt — not from any cache — an admin's edit lands on the very next login with no API restart and no `helm upgrade`. No new caching layer is introduced.

**Scope limits (v1)**: `groupsClaim` and `defaultRole` are **Helm-only** — there is no override for either. Only the three role lists (`admin`/`operator`/`viewer`) are overridable. There is no `PUT /admin/config/role-mappings/default-role` or equivalent; nothing in this feature makes `defaultRole` editable.

**Upgrade semantics (M9)**: a later `helm upgrade` that changes `api.oidc.roleMappings.*` only changes the *seed* half of the merge. Any role carrying a `helmOverride` entry keeps that override — the upgrade never touches the `auth` config row. See Scenario 8b.

**Resetting an override**: `DELETE /admin/config/auth/role-mappings/{role}` (role ∈ `admin|operator|viewer`) — the one genuinely new route this feature adds — removes that role's key from `helmOverride.roleMappings`, restoring the Helm-seeded value on the next login. It sits under the same `/admin/config` prefix and the same `config:manage` RBAC rule (`api/internal/rbac/rbac.go:170`) that already gates every non-GET method under `/admin/config` — no new middleware. See Scenario 8c.

**Audit (M8)**: every write to `helmOverride.roleMappings` (via `PUT /admin/config/auth`) or reset (via the new `DELETE`) is admin-gated and recorded through the existing `WriteSync(ctx, method, path, target, reason, status)` call, with `target=<role>`. `audit_events` has columns `id, ts, actor, method, path, target, status, ip, reason` — no `action`/`metadata`/`message` column — so every scenario below uses one consistent `reason` format:

- On set: `oidc role mapping override set: role=<role> groups=<comma_joined_or_none>`
- On reset: `oidc role mapping override reset: role=<role>`

**Provenance (M7)**: `GET /admin/config` already returns every section keyed by name (`{"auth": {...}, "general": {...}, ...}` — see `api/internal/handlers/config.go:43-67`). There is no computed "effective" view and no `source` field — `config.go:43-67` returns each stored row verbatim, so the `auth` section is exactly `{"providers": [...], "helmOverride": {...}}` with nothing added. Provenance is **key presence**: a role key present in `auth.helmOverride.roleMappings` is overridden (dashboard-driven); a role key absent there falls back to the Helm seed, which the dashboard reads from the top-level `installTimeSettings.oidcHelmProvider.roleMappings` (the read-only Helm-seed snapshot — see the data model). The dashboard offers "Reset to Helm default" only where the key is present in `helmOverride.roleMappings`:

```json
{
  "auth": {
    "providers": [ /* unchanged */ ],
    "helmOverride": { "roleMappings": { "operator": ["gameplane-ops-v2"] } }
  },
  "installTimeSettings": {
    "gameDataStorageClass": "fast-local-nvme",
    "oidcHelmProvider": {
      "groupsClaim": "groups",
      "defaultRole": "viewer",
      "roleMappings": {
        "admin": ["gameplane-admins"],
        "operator": ["gameplane-operators"],
        "viewer": ["gameplane-viewers"]
      }
    }
  }
}
```

To determine provenance for a role, check whether that role's key is present in `auth.helmOverride.roleMappings`: present → overridden (use the value there as the active mapping), absent → Helm-seeded (use the value at `installTimeSettings.oidcHelmProvider.roleMappings.<role>` as the active mapping). There is no server-computed "active" or "effective" field anywhere in the response — the dashboard (and these scenarios) compute it client-side from these two sources.

**CLI flags (Helm-seed side, M6)**: following the verified pattern at `api/cmd/main.go:391-395` — every existing OIDC flag is a single `fs.StringVar` reading an env fallback, never a repeatable `flag.Var` — the three seed lists are comma-separated single strings, not repeatable flags:

- `--oidc-role-mapping-admin`, `--oidc-role-mapping-operator`, `--oidc-role-mapping-viewer` (comma-separated claim values)
- `--oidc-groups-claim`, `--oidc-default-role` (unchanged, Helm-only, not overridable — see Scope limits above)

Key points to keep straight while running the scenarios below:

- The synthetic read-only **`helm` provider** (`HelmProviderName` in `api/internal/auth/registry.go`) still exists and is **still immutable** through the dashboard — its issuer/clientID/scopes/seed mappings cannot be edited or deleted. Nothing in this feature makes that provider editable.
- The `helmOverride` overlay is consulted during role resolution, layered on top of (not written into) the `helm` provider's configuration.
- On a later `helm upgrade` that changes `api.oidc.roleMappings.*`, an existing override for that role **wins and is not clobbered** — see Scenario 8b. The dashboard offers an explicit **"Reset to Helm default"** action per role that deletes the override and falls back to the Helm-seeded value — see Scenario 8c.
- Every write or reset of an overlay entry is admin-gated (RBAC: `config:manage`) and written to the audit log using the reason format defined above.

Scenarios 3, 5, 7, and 8 below exercise this model end to end.

---

## Scenario 1: Install with Custom Game-Data Storage Class

**Objective**: Verify that when installed with a custom storage class, all GameServer PVCs use that class instead of the cluster default.

**Prerequisite**: The `fast-local-nvme` StorageClass exists on the cluster (see Prerequisites).

### Steps

1. **Create a test namespace**:
   ```bash
   kubectl create namespace gameplane-test-sc
   ```

2. **Install Gameplane with custom storage class**:
   ```bash
   helm install gameplane charts/gameplane/ \
     --namespace gameplane-test-sc \
     --set operator.gameDataStorage.storageClassName=fast-local-nvme \
     --set api.oidc.issuer="" \
     --set api.bootstrap.enabled=true
   ```

3. **Verify operator deployment**:
   ```bash
   kubectl rollout status deployment/gameplane-operator \
     -n gameplane-test-sc --timeout=60s
   ```

4. **Create a test GameServer**:
   ```bash
   # If using the dashboard:
   # - Log in with bootstrap-admin credentials
   # - Navigate to Servers > Create Server
   # - Choose a template (e.g., Minecraft Java)
   # - Click Create

   # Or via kubectl:
   kubectl apply -f - -n gameplane-test-sc <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: GameServer
   metadata:
     name: test-minecraft
   spec:
     templateRef:
       name: minecraft-java
     # No explicit storage class override; should inherit from install-time default
   EOF
   ```

5. **Verify PVC storage class**:
   ```bash
   kubectl get pvc -n gameplane-test-sc -o json | jq '.items[] | select(.metadata.name | startswith("test-minecraft")) | {name: .metadata.name, storageClassName: .spec.storageClassName}'
   ```

   **Expected output**:
   ```json
   {
     "name": "test-minecraft-data",
     "storageClassName": "fast-local-nvme"
   }
   ```

### Validation Checklist

- [ ] All PVCs for `test-minecraft-*` show `storageClassName: fast-local-nvme`
- [ ] PVCs do not show `storageClassName: standard-network` (cluster default)
- [ ] Operator logs contain no errors about storage class provisioning (check: `kubectl logs -n gameplane-test-sc deployment/gameplane-operator | grep -i storage`)

---

## Scenario 2: Nonexistent Storage Class → Actionable Error

**Objective**: Verify that when a GameServer requests a nonexistent storage class, the error is visible in the dashboard within 30 seconds (SC-002).

### Steps

1. **Install with a nonexistent storage class**:
   ```bash
   helm install gameplane charts/gameplane/ \
     --namespace gameplane-test-nonexist \
     --set operator.gameDataStorage.storageClassName=nonexistent-class \
     --set api.bootstrap.enabled=true
   ```

2. **Create a GameServer**:
   ```bash
   kubectl apply -f - -n gameplane-test-nonexist <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: GameServer
   metadata:
     name: test-failed-pvc
   spec:
     templateRef:
       name: minecraft-java
   EOF
   ```

3. **Check GameServer status within 30 seconds**:
   ```bash
   kubectl get gameserver test-failed-pvc -n gameplane-test-nonexist -o json | jq '.status | {phase: .phase, conditions: .conditions[]}'
   ```

   **Expected output** (within 30s):
   ```json
   {
     "phase": "Failed",
     "conditions": [
       {
         "type": "Ready",
         "status": "False",
         "reason": "PVCProvisioningFailed",
         "message": "StorageClass 'nonexistent-class' not found; verify the class exists on the cluster."
       }
     ]
   }
   ```

4. **Verify dashboard rendering**:
   - Log in to the dashboard (via bootstrap-admin or direct URL)
   - Navigate to Servers > `test-failed-pvc`
   - The **Status** section should show an error banner with the message: `"StorageClass 'nonexistent-class' not found; verify the class exists on the cluster."`
   - Elapsed time from GameServer creation to error visibility: < 30 seconds

### Validation Checklist

- [ ] GameServer phase is `Failed` (not indefinitely `Pending`)
- [ ] Ready condition reason is `PVCProvisioningFailed`
- [ ] Condition message includes the storage class name and actionable text
- [ ] Dashboard renders the error message with an AlertTriangle icon
- [ ] Error message appears within 30 seconds of GameServer creation

---

## Scenario 3: OIDC Role Mappings at Install Time (No Bootstrap-Admin)

**Objective**: Verify that a user can be assigned the admin role on first OIDC login via pre-configured role mappings, without requiring bootstrap-admin (SC-003, SC-004).

**Note (hybrid model, H1)**: This scenario exercises the **Helm-seed layer only** — no DB override exists yet, so the Helm-seeded mapping applies directly. Once an admin exists (as this scenario produces), Scenario 8 shows the admin overriding a mapping through the dashboard.

**Prerequisite**: An OIDC provider with a test user in a group named `gameplane-admins` (e.g., Keycloak, Okta, or a test fake-OIDC server).

### Steps

1. **Install with OIDC role mappings**:
   ```bash
   helm install gameplane charts/gameplane/ \
     --namespace gameplane-test-oidc \
     --set api.oidc.issuer=https://your-oidc-provider/auth/realms/test \
     --set api.oidc.clientID=gameplane-test \
     --set api.oidc.clientSecretRef.name=oidc-client-secret \
     --set api.oidc.clientSecretRef.key=secret \
     --set api.oidc.groupsClaim=groups \
     --set 'api.oidc.roleMappings.admin[0]=gameplane-admins' \
     --set 'api.oidc.roleMappings.operator[0]=gameplane-operators' \
     --set api.oidc.defaultRole=viewer \
     --set api.bootstrap.enabled=false
   ```

   **Note**: `api.bootstrap.enabled=false` prevents bootstrap-admin from being used; OIDC is the only auth method.

2. **Verify API startup**:
   ```bash
   kubectl rollout status deployment/gameplane-api \
     -n gameplane-test-oidc --timeout=60s
   kubectl logs -n gameplane-test-oidc deployment/gameplane-api | grep -i "oidc\|role mapping"
   ```

3. **Log in as a user in the `gameplane-admins` group**:
   - Open the dashboard: `kubectl port-forward -n gameplane-test-oidc svc/gameplane-api 8000:80`
   - Navigate to `http://localhost:8000/login`
   - Click "Sign in with single sign-on"
   - Enter credentials for a test user who is a member of the `gameplane-admins` group in your OIDC provider
   - You are redirected to the dashboard

4. **Verify admin role assignment**:
   - On the dashboard home, click **Settings** > **Admin Settings**
   - The page loads without a permission error (indicates admin role)
   - Verify your username in the top-right shows you are logged in

5. **Verify audit event (FR-014 — role assignment auditing)**:
   - Role assignments via OIDC must emit an audit event for compliance. Verify this by checking the API audit log:
     ```bash
     kubectl exec -n gameplane-test-oidc deployment/gameplane-api -- \
       sqlite3 /app/data/gameplane.db \
       "SELECT ts, actor, method, path, target, status, ip, reason FROM audit_events WHERE actor='<your-oidc-username>' ORDER BY ts DESC LIMIT 1;"
     ```

   **Expected output** (reason includes provider, matched claim value, previous role, new role):
   ```
   2026-08-25 10:15:30|<your-oidc-username>|POST|/auth/oidc/callback|<username>|200|127.0.0.1|oidc role assigned: provider=helm matched=gameplane-admins from=new_user to=admin
   ```

   **Alternatively**, view audit events via the dashboard:
   - Navigate to **Settings** > **Admin Settings** > **Audit Log** (or **Audit Events** tab)
   - Filter by your username
   - Confirm an audit event appears with a reason like "oidc role assigned: ..." within 10 seconds of login

### Validation Checklist

- [ ] OIDC login succeeds (no errors during callback)
- [ ] User is automatically assigned the admin role (no manual role change required)
- [ ] Admin Settings page loads without permission errors
- [ ] Audit log shows `oidc_role_assigned` reason for the login event
- [ ] No bootstrap-admin was run; user's first OIDC login grants admin role

---

## Scenario 4: Role Re-Evaluation on Group Change

**Objective**: Verify that when a user's OIDC group membership changes, their Gameplane role is updated on their next login (FR-011, SC-005).

**Prerequisite**: A user with the viewer role exists in the system (from a prior OIDC login) and is a member of a group mapping to viewer. The user's OIDC group membership in the IdP can be changed.

### Steps

1. **Verify initial role (viewer)**:
   - User logs in via OIDC with group membership in `gameplane-viewers` (mapped to viewer role)
   - Verify their role in the dashboard or database:
     ```bash
     kubectl exec -n gameplane-test-oidc deployment/gameplane-api -- \
       sqlite3 /app/data/gameplane.db \
       "SELECT username, role FROM users WHERE username='<oidc-username>';"
     ```
   - Expected: `<oidc-username>|viewer`

2. **Promote user in OIDC provider**:
   - In your OIDC provider, change the user's group membership: remove from `gameplane-viewers`, add to `gameplane-admins`
   - Record the timestamp of this change

3. **User logs in again**:
   - Log out of the Gameplane dashboard
   - Log in again as the same user
   - Record the timestamp of login

4. **Verify role update**:
   - Check the user's role in the database:
     ```bash
     kubectl exec -n gameplane-test-oidc deployment/gameplane-api -- \
       sqlite3 /app/data/gameplane.db \
       "SELECT username, role FROM users WHERE username='<oidc-username>';"
     ```
   - Expected: `<oidc-username>|admin`
   - Check the audit log for a role-update event:
     ```bash
     kubectl exec -n gameplane-test-oidc deployment/gameplane-api -- \
       sqlite3 /app/data/gameplane.db \
       "SELECT ts, actor, method, path, target, status, ip, reason FROM audit_events WHERE actor='<oidc-username>' ORDER BY ts DESC LIMIT 1;"
     ```
   - Expected reason: `oidc role assigned: provider=helm matched=gameplane-admins from=viewer to=admin`

5. **Verify new capabilities**:
   - The user can now access Admin Settings and other admin-only pages
   - The dashboard reflects the new admin role without a page refresh (or after refresh, depending on implementation)

### Validation Checklist

- [ ] Role changes from viewer to admin after group membership change
- [ ] Role change takes effect on next login (within 5 minutes if dashboard is open, or immediately on next login)
- [ ] Audit log shows `oidc_role_updated` (or equivalent) event
- [ ] Admin features become accessible after role update

---

## Scenario 5: Over-Broad OIDC Mapping Warning (FR-015)

**Objective**: Verify that when an admin configures a role mapping override that grants admin privileges, they receive a clear warning about the risk of unintended broad access, and must explicitly confirm before saving (FR-015).

**Prerequisite**: An admin user logged in via bootstrap-admin or existing OIDC role (so they can access AdminSettings), with the Helm-configured `helm` provider present per Scenario 3.

**Note (hybrid model, H4)**: This scenario writes to `helmOverride.roleMappings` on the existing `auth` config row, not to the immutable `helm` provider itself. The `helm` provider's own issuer/clientID/seed mappings remain read-only throughout — the editing surface is the separate role-mapping overlay described in the model overview above.

### Steps

1. **Navigate to Admin Settings**:
   - Click **Settings** > **Admin Settings**
   - Go to the **Auth** > **Role Mappings** section — this is the `helmOverride.roleMappings` editing surface, distinct from the read-only **Providers** list where the `helm` provider's own issuer/clientID/seed mappings are displayed and cannot be edited
   - The Helm-configured provider ("helm" or "Single sign-on") must already exist (per Scenario 3) for this section to be meaningful; its seeded per-role lists appear here as the current effective values until overridden (see Scenario 8d for provenance display)

2. **Attempt to map a group to admin role**:
   - In the role mappings editor, edit the `admin` row
   - Enter a group name (e.g., `developers`, `all-users`, or any broad group name)
   - Click **Save**
   - This sends `PUT /admin/config/auth` with `helmOverride.roleMappings.admin` set to the entered group(s) — it does not modify the `helm` provider's Helm-seeded configuration, and does not touch `groupsClaim` or `defaultRole` (both stay Helm-only, M5)

3. **Verify warning dialog appears (FR-015)**:
   - A warning dialog or modal should appear with text similar to:
     ```
     Warning: Admin Role Assignment
     
     You are about to grant the admin role to members of the group(s): [group name(s)]
     
     Admin users can manage all system settings, users, and configurations. If your group membership is large 
     or includes many unintended members, this may grant excessive access. Please verify that you intend to 
     give admin privileges to all members of this group.
     
     [Cancel]  [Confirm & Save]
     ```
   - The warning is **unconditional** (appears regardless of how many users are in the group — Gameplane cannot enumerate group membership)
   - The warning appears **only when the target role is admin** (not for operator or viewer roles)

4. **Confirm the mapping**:
   - Click **Confirm & Save** to proceed
   - The DB override should be saved successfully
   - Verify it appears in the role mappings list, marked with a DB-overlay provenance indicator (see Scenario 8d)

5. **Verify via API and audit log (H6)**:
   - Query `GET /admin/config` and verify the override appears in the `auth` section's `helmOverride`, and compare it against the Helm seed at `installTimeSettings.oidcHelmProvider.roleMappings` (see the model overview above for the exact shape) — provenance is key presence in `auth.helmOverride.roleMappings`; there is no server-computed `effective` view and no `source` field:
     ```bash
     curl -s -H "Cookie: gameplane_session=<token>" \
       http://localhost:8000/api/admin/config | jq '.auth.helmOverride.roleMappings, .installTimeSettings.oidcHelmProvider.roleMappings'
     ```
   - Verify the write was audit-logged (H6):
     ```bash
     kubectl exec -n gameplane-test-oidc deployment/gameplane-api -- \
       sqlite3 /app/data/gameplane.db \
       "SELECT ts, actor, method, path, target, status, reason FROM audit_events WHERE actor='<your-admin-username>' ORDER BY ts DESC LIMIT 1;"
     ```
   - Expected: a recent row with `method=PUT`, `path=/admin/config/auth`, `target=admin`, and `reason=oidc role mapping override set: role=admin groups=<the group(s) you entered>` (the one consistent reason format defined in the model overview above)

### Validation Checklist

- [ ] Warning dialog appears when attempting to map a group to admin role
- [ ] Warning text is clear and mentions the risk of unintended broad access
- [ ] Confirm button is present and labeled "Confirm & Save" or similar
- [ ] Cancel button allows the user to abort the mapping
- [ ] After confirmation, the DB override is saved and persists in the dashboard
- [ ] The `helm` provider's own seeded configuration is unchanged by this save (H4)
- [ ] Warning does NOT appear for operator or viewer role mappings
- [ ] Audit log shows the mapping override change, admin-gated (H6)

---

## Scenario 6: Backward Compatibility (No Storage Class, No Role Mappings)

**Objective**: Verify that Gameplane works as before when install-time configuration is not set (SC-008).

### Steps

1. **Install without specifying storage class or role mappings**:
   ```bash
   helm install gameplane charts/gameplane/ \
     --namespace gameplane-test-legacy \
     --set api.oidc.issuer="" \
     --set api.bootstrap.enabled=true
     # No storage class set; no role mappings set
   ```

2. **Create a GameServer**:
   ```bash
   kubectl apply -f - -n gameplane-test-legacy <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: GameServer
   metadata:
     name: test-default-storage
   spec:
     templateRef:
       name: minecraft-java
   EOF
   ```

3. **Verify PVC uses cluster default**:
   ```bash
   kubectl get pvc test-default-storage-data -n gameplane-test-legacy -o json | jq '.spec.storageClassName'
   ```
   - Expected: `null` (empty, meaning cluster default), or the actual name of the cluster's default StorageClass

4. **Log in via bootstrap-admin**:
   - Use the bootstrap-admin credentials provided during `helm install`
   - Verify you can access the dashboard and admin features

5. **Create a local user or use OIDC (if configured)**:
   - If OIDC is not configured, only local password auth works
   - If OIDC is configured (issuer is set), users logging in via OIDC receive the default role (viewer)
   - Verify in the database:
     ```bash
     kubectl exec -n gameplane-test-legacy deployment/gameplane-api -- \
       sqlite3 /app/data/gameplane.db \
       "SELECT username, role FROM users LIMIT 5;"
     ```

### Validation Checklist

- [ ] GameServer PVC is created (not blocked)
- [ ] PVC storage class is unset (nil) or cluster default
- [ ] Bootstrap-admin login works
- [ ] OIDC users (if configured) get viewer role by default
- [ ] No errors in operator/API logs related to missing config

---

## Scenario 7: Admin Configuration Interface (FR-006, FR-017, SC-006)

**Objective**: Verify that install-time settings are visible in the admin configuration interface. **Storage class remains fully read-only.** OIDC role mappings display both the Helm-seeded values and any DB overrides, with provenance — the mappings section is a display surface here; Scenario 8 exercises the editing surface itself.

### Steps

1. **Log in to the dashboard** as an admin (via bootstrap-admin or OIDC admin role)

2. **Navigate to Admin Settings**:
   - Click **Settings** in the top-right menu
   - Select **Admin Settings** (or **Configuration**)
   - Look for a new section: "Install-Time Settings" or "System Configuration"

3. **Verify storage class display (D2 — API reporting)**:
   - The section should display: `Game Data Storage Class: fast-local-nvme` (or `"(cluster default)"` if not set)
   - The field is read-only (no edit input, no save button)
   - **Behind the scenes**: The storage class value is passed to BOTH the operator and API via a single Helm key (`operator.gameDataStorage.storageClassName`):
     - Operator receives it via the CLI flag `--game-data-storage-class` (used for PVC provisioning)
     - API receives it via a separate report-only CLI flag on the `api serve` subcommand (following the same naming convention as the operator flag; see `contracts/api-cli.md` for the exact flag name) and reports it via `GET /admin/config` in the `installTimeSettings.gameDataStorageClass` field
   - **Example Helm install command**:
     ```bash
     helm install gameplane charts/gameplane/ \
       --set operator.gameDataStorage.storageClassName=fast-local-nvme \
       --set api.oidc.issuer="" \
       --set api.bootstrap.enabled=true
     ```
   - Verify via the API by querying `GET /admin/config`:
     ```bash
     curl -s -H "Cookie: gameplane_session=<token>" \
       http://localhost:8000/api/admin/config | jq '.installTimeSettings.gameDataStorageClass'
     ```

4. **Verify OIDC role mappings display (H4, H7)**:
   - The section should display the Helm-configured provider (labeled "helm" or "Single sign-on") as the seed layer:
     - `Group Claim: groups` (Helm-only; not overridable)
     - `Admin Groups (Helm seed): ["gameplane-admins"]`
     - `Operator Groups (Helm seed): ["gameplane-operators"]`
     - `Viewer Groups (Helm seed): ["gameplane-viewers"]` (or default role — also Helm-only; not overridable)
   - The `helm` provider's own fields (issuer, clientID, seed mappings) remain listed as read-only and cannot be edited or deleted — this has not changed
   - If any role carries an override (from Scenario 8, stored in `helmOverride.roleMappings` on the same `auth` config row), that role's row shows the override value with a distinct provenance label (e.g. "from dashboard") alongside the Helm seed value, and the row **does** carry edit/reset controls — this is the new hybrid editing surface (H7), driven by whether that role's key is present in `GET /admin/config`'s `auth.helmOverride.roleMappings` (present = overridden, absent = Helm-seeded — there is no `source` field), not an edit to the `helm` provider record itself

5. **Check for "not configured" message**:
   - If no install-time settings are configured:
     - "Game Data Storage Class: (cluster default)"
     - "OIDC Role Mappings: Not configured. Use Helm chart values to set role mappings at install time, or configure them here once an admin account exists."

### Validation Checklist

- [ ] Install-time settings are visible in the admin configuration interface
- [ ] Storage class setting is displayed as read-only (no edit controls) — unchanged by this feature's hybrid model
- [ ] Helm-configured provider's own fields (issuer, clientID, seed mappings) are listed and marked as immutable
- [ ] Any DB-overridden role mapping is visibly distinguished from the Helm-seeded value (provenance)
- [ ] No errors when viewing the configuration page
- [ ] Clear guidance is provided if settings are not configured

---

## Scenario 8: Hybrid Role Mapping — Overlay Wins, Helm Seeds (H1–H4, SC-007)

**Objective**: Verify the full hybrid lifecycle for OIDC role mappings: an overlay write (via `PUT /admin/config/auth`) takes effect on the next login with no restart and no `helm upgrade` (this is the scenario that proves SC-007), a subsequent `helm upgrade` does not clobber that override (H3), the reset route (`DELETE /admin/config/auth/role-mappings/{role}`) restores the Helm-declared value, and the dashboard shows provenance for each mapping.

**Prerequisite**: Gameplane installed per Scenario 3 (`api.oidc.roleMappings.operator[0]=gameplane-operators`, `api.oidc.defaultRole=viewer`), with an admin account available (via the Scenario 3 admin mapping or bootstrap-admin) and a test user `bob` who is a member of `gameplane-operators` in the OIDC provider (and nothing else).

### 8a. Overlay write takes effect on next login, no restart, no `helm upgrade` (proves SC-007)

1. **Confirm the pre-override role**:
   - Log in as `bob` via OIDC; confirm the assigned role is `operator` (matches the Helm-seeded `operator` mapping):
     ```bash
     kubectl exec -n gameplane-test-oidc deployment/gameplane-api -- \
       sqlite3 /app/data/gameplane.db \
       "SELECT username, role FROM users WHERE username='bob';"
     ```
     Expected: `bob|operator`
   - Log `bob` out.

2. **Admin sets an overlay for the operator role, WITHOUT touching Helm or restarting anything**:
   - Log in as the admin, go to **Settings** > **Admin Settings** > **Auth** > **Role Mappings**
   - Edit the `operator` row: remove `gameplane-operators`, add `gameplane-ops-v2`
   - Click **Save** — this sends `PUT /admin/config/auth` with `helmOverride.roleMappings.operator = ["gameplane-ops-v2"]` alongside the unchanged `providers` array; it writes only the existing `"auth"` row in the `config` table — **do not** run `helm upgrade` and **do not** restart the API pod
   - Confirm no rollout occurred:
     ```bash
     kubectl rollout status deployment/gameplane-api -n gameplane-test-oidc --timeout=5s
     # expect: already at the same revision, no new rollout triggered
     ```

3. **`bob` (still only in `gameplane-operators`) logs in again**:
   - Because the overlay now maps `operator` to `["gameplane-ops-v2"]` (replacing the Helm-seeded `["gameplane-operators"]` list per the merge algorithm), and `bob` is not a member of `gameplane-ops-v2`, `bob` no longer matches the `operator` role at all
   - Verify `bob`'s role fell through to the default role:
     ```bash
     kubectl exec -n gameplane-test-oidc deployment/gameplane-api -- \
       sqlite3 /app/data/gameplane.db \
       "SELECT username, role FROM users WHERE username='bob';"
     ```
     Expected: `bob|viewer`

4. **Add `bob` to `gameplane-ops-v2` in the OIDC provider and log in a third time**:
   - Expected: `bob|operator` again — now matching the overridden group, not the original Helm-seeded one

**This is the SC-007 proof point**: the `PUT /admin/config/auth` in step 2 took effect at `bob`'s next login (step 3/4) with no API restart and no `helm upgrade` in between — not because of the registry's row-hash cache (that cache governs DB providers only; the helm provider path in `OIDCFor` short-circuits past it entirely, `registry.go:224-232`), but because role resolution for the helm provider reads `helmOverride` fresh from the `"auth"` config row on every login attempt and merges it over the Helm-seeded lists before `computeRole` runs. `r.legacy` itself (built once at `main.go:127`) never changes — only the per-login override read does.

### 8b. `helm upgrade` does not clobber an existing override (H3)

1. **With the override from 8a still in place** (`operator` → `["gameplane-ops-v2"]` in `helmOverride.roleMappings`), run a `helm upgrade` that changes the *Helm-seeded* value for the same role:
   ```bash
   helm upgrade gameplane charts/gameplane/ \
     --namespace gameplane-test-oidc \
     --reuse-values \
     --set 'api.oidc.roleMappings.operator[0]=gameplane-operators-renamed'
   ```

2. **Verify the API pod picks up the new Helm value as the seed**:
   ```bash
   kubectl rollout status deployment/gameplane-api -n gameplane-test-oidc --timeout=60s
   kubectl get deployment gameplane-api -n gameplane-test-oidc \
     -o jsonpath='{.spec.template.spec.containers[0].args}' | grep -o 'oidc-role-mapping-operator=[^ ]*'
   ```
   Expected: the flag value now reflects `gameplane-operators-renamed`.

3. **Log in as `bob` (still a member of `gameplane-ops-v2` only, not `gameplane-operators-renamed`)**:
   - Expected: `bob` still receives `operator` — the overlay (`["gameplane-ops-v2"]`) is still in effect and wins over the freshly-upgraded Helm seed, because the `helm upgrade` never touched the `auth` config row
     ```bash
     kubectl exec -n gameplane-test-oidc deployment/gameplane-api -- \
       sqlite3 /app/data/gameplane.db \
       "SELECT username, role FROM users WHERE username='bob';"
     ```
     Expected: `bob|operator`
   - Confirm via the dashboard: **Admin Settings** > **Auth** > **Role Mappings** still shows `gameplane-ops-v2` as the *active* value for the `operator` role, with the new Helm seed (`gameplane-operators-renamed`) shown alongside as the seed value, not the active one (see 8d)

**This is the H3 proof point**: the `helm upgrade` in step 1 changed the Helm-seeded value the registry synthesizes at startup, but never wrote the `auth` DB row — the overlay was never touched and silently keeps winning, by design (M9).

### 8c. Reset to Helm default

1. **From the same Role Mappings view**, on the `operator` row (currently overridden to `["gameplane-ops-v2"]`), click **Reset to Helm default** — this calls the one new route this feature adds:
   ```bash
   curl -s -X DELETE -H "Cookie: gameplane_session=<admin-token>" \
     http://localhost:8000/api/admin/config/auth/role-mappings/operator
   ```
2. **Confirm the reset** (if a confirmation prompt appears in the dashboard)
3. **Verify the override is gone**: `helmOverride.roleMappings` no longer has an `operator` key — that key's absence is what marks the role Helm-seeded again (there is no `source` field to check):
   ```bash
   curl -s -H "Cookie: gameplane_session=<admin-token>" \
     http://localhost:8000/api/admin/config | jq '.auth.helmOverride.roleMappings, .installTimeSettings.oidcHelmProvider.roleMappings.operator'
   ```
   Expected: `helmOverride.roleMappings` has no `operator` key; `installTimeSettings.oidcHelmProvider.roleMappings.operator` is `["gameplane-operators-renamed"]` (the Helm seed from 8b) — with no `operator` key in `helmOverride.roleMappings`, that Helm-seed value is now the active mapping.
4. **`bob` (member of `gameplane-ops-v2` only) logs in again**:
   - Expected: `bob` no longer matches (only `gameplane-operators-renamed` is active), falls through to `defaultRole` (`viewer`)
     ```bash
     kubectl exec -n gameplane-test-oidc deployment/gameplane-api -- \
       sqlite3 /app/data/gameplane.db \
       "SELECT username, role FROM users WHERE username='bob';"
     ```
     Expected: `bob|viewer`
5. **Verify the reset was audited (H6)**:
   ```bash
   kubectl exec -n gameplane-test-oidc deployment/gameplane-api -- \
     sqlite3 /app/data/gameplane.db \
     "SELECT ts, actor, method, path, target, status, reason FROM audit_events WHERE actor='<admin-username>' ORDER BY ts DESC LIMIT 1;"
   ```
   Expected: a row with `method=DELETE`, `path=/admin/config/auth/role-mappings/operator`, `target=operator`, `status=204` (or `200`), and `reason=oidc role mapping override reset: role=operator` (the one consistent reason format from the model overview).

### 8d. Provenance display

1. **With one role overridden and others left at their Helm seed** (e.g. only `operator` overridden, `admin`/`viewer` still Helm-seeded), open **Admin Settings** > **Auth** > **Role Mappings**
2. **Verify each role row is labeled with its source**:
   - Overridden roles show something like `Active: <value> (from dashboard)` plus the underlying `Helm seed: <value>` for comparison
   - Non-overridden roles show `Active: <value> (Helm seed)` with no separate override value, and no reset control
3. **Verify via API**:
   ```bash
   curl -s -H "Cookie: gameplane_session=<token>" \
     http://localhost:8000/api/admin/config | jq '.auth.helmOverride.roleMappings, .installTimeSettings.oidcHelmProvider.roleMappings'
   ```
   Expected shape — there is no server-computed `effective` view and no `source` field anywhere in the response; provenance for each role comes from whether that role's key is present in `auth.helmOverride.roleMappings`, and the active value is computed client-side (override where present, Helm seed otherwise):
   ```json
   {
     "auth": {
       "helmOverride": { "roleMappings": { "operator": ["gameplane-ops-v2"] } }
     },
     "installTimeSettings": {
       "oidcHelmProvider": {
         "roleMappings": {
           "admin": ["gameplane-admins"],
           "operator": ["gameplane-operators-renamed"],
           "viewer": ["gameplane-viewers"]
         }
       }
     }
   }
   ```
   Here `operator` is overridden (its key is present in `auth.helmOverride.roleMappings`, so the active value is `["gameplane-ops-v2"]`, not the Helm seed shown above) while `admin` and `viewer` are Helm-seeded (their keys are absent from `helmOverride.roleMappings`, so the active value is read straight from `installTimeSettings.oidcHelmProvider.roleMappings`).

### Validation Checklist

- [ ] An overlay write saved through the dashboard (`PUT /admin/config/auth`) takes effect on the affected user's **next login**, with no API pod restart and no `helm upgrade` (SC-007)
- [ ] A `helm upgrade` that changes a Helm-seeded mapping does **not** overwrite an existing overlay entry for the same role (H3), because the upgrade never writes the `auth` DB row (M9)
- [ ] The overlay continues to win over the newly-upgraded Helm seed until explicitly reset
- [ ] `DELETE /admin/config/auth/role-mappings/{role}` removes that role's overlay entry and the Helm-seeded value becomes active again
- [ ] Every write (`PUT /admin/config/auth`) or reset (`DELETE .../role-mappings/{role}`) is admin-gated (`config:manage`) and appears in `audit_events` with `target=<role>` and reason `oidc role mapping override set: role=<role> groups=<comma_joined_or_none>` (set) or `oidc role mapping override reset: role=<role>` (reset) (H6)
- [ ] The dashboard visibly distinguishes overridden roles from Helm-seeded roles via key presence in `auth.helmOverride.roleMappings` (present = overridden, absent = Helm-seeded — there is no `source` field) (provenance)
- [ ] The `helm` provider's own record (issuer, clientID, seed mappings) is never mutated by any of the above — only `helmOverride` on the same `auth` row changes (H4)
- [ ] `groupsClaim` and `defaultRole` have no override surface anywhere in this scenario — they remain Helm-only (M5)

---

## Testing & Automation

### Unit & Envtest Tests

The feature includes unit tests for OIDC role mapping logic:

```bash
# Run API auth tests
go test ./api/internal/auth -run TestComputeRole -v

# Run operator storage class tests
go test ./operator/internal/controller -run TestGameServer -v
```

Expected test files:
- `api/internal/auth/oidc_rolemap_test.go` — tests claim parsing, role mapping, deny logic, and (hybrid model) the new `effectiveHelmPolicy(base *ProviderPolicy, ov *RoleMappings) *ProviderPolicy` helper: a non-nil override list replaces the Helm-seeded list for that role only, an un-overridden role falls back to the Helm seed unchanged, and — because the merge happens before the existing, unmodified `computeRole` runs — a user matching an overridden viewer group and a Helm-seeded admin group still resolves to admin (M4)
- `operator/internal/controller/gameserver_storage_envtest_test.go` — tests PVC storage class selection and error handling

### End-to-End Tests (CI)

The feature includes e2e tests in the CI pipeline:

**api-auth bucket** (login-pressure-constrained):
- `TestAPI_OIDCRoleMappingAtInstallTime` — verifies install-time role mapping loads correctly
- `TestAPI_OIDCRoleMappingFirstLogin` — verifies first user login gets correct role
- `TestAPI_OIDCRoleMappingReEvalOnLogin` — verifies role re-evaluation on group change
- `TestAPI_OIDCRoleMappingDBOverrideTakesEffectNextLogin` — automates Scenario 8a: a DB-saved role mapping override changes the computed role on the affected user's next login, with no process restart in between (the SC-007 automation)
- `TestAPI_OIDCRoleMappingSurvivesHelmReseed` — automates Scenario 8b: re-synthesizing the Helm-seeded provider policy (simulating a `helm upgrade` value change) does not clobber an existing DB override for the same role
- `TestAPI_OIDCRoleMappingResetToHelmDefault` — automates Scenario 8c: the reset action deletes the DB override and the Helm-seeded value becomes active again

**operator bucket** (zero login pressure):
- `TestGameServer_StorageClassFromHelmDefault` — verifies default storage class is applied
- `TestGameServer_StorageClassNotFound` — verifies error handling for nonexistent class
- `TestGameServer_StorageClassExplicitOverride` — verifies explicit override precedence

Run e2e tests locally (if you have a kind cluster running):

```bash
# Run a single bucket (operator tests)
make test-e2e-bucket BUCKET=operator

# Or run all e2e tests
make test-e2e
```

### Test OIDC Issuer (Fake IdP for Local Testing)

For local testing without an external OIDC provider, use a fake OIDC issuer:

1. **Start a fake OIDC server** (example using a test library):
   ```bash
   # In a separate terminal, run a test OIDC issuer on localhost:5555
   go run test/e2e/support/fake_oidc_server.go -listen :5555
   ```

2. **Configure Gameplane to use the fake issuer**:
   ```bash
   helm install gameplane charts/gameplane/ \
     --set api.oidc.issuer=http://localhost:5555 \
     --set api.oidc.clientID=gameplane-test \
     --set api.oidc.groupsClaim=groups \
     --set 'api.oidc.roleMappings.admin[0]=test-admins' \
     --set api.oidc.redirectURL=http://localhost:8000/auth/oidc/callback
   ```

3. **Test the login flow**:
   - Navigate to the dashboard login page
   - Click "Sign in with single sign-on"
   - The fake issuer will issue a test token with groups claim
   - Verify the user is assigned the correct role

---

## Traceability: Scenarios → Requirements → Tests

| Scenario | Requirement(s) | Success Criteria | E2E Test File | Touch Points |
|----------|---|---|---|---|
| **1: Custom Storage Class** | FR-001, FR-002 | SC-001: All new PVCs use specified class | `test/e2e/gameserver_e2e_test.go::TestGameServer_StorageClassFromHelmDefault` | operator, helm-values |
| **2: Nonexistent Class Error** | FR-005 | SC-002: Error visible in dashboard within 30s | `test/e2e/gameserver_e2e_test.go::TestGameServer_StorageClassNotFound` | operator, dashboard |
| **3: OIDC Role Mappings (First Login) + **FR-014 Audit** | FR-007, FR-009, **FR-014** | SC-003, SC-004: First user gets correct role without bootstrap-admin; **audit event emitted** | `test/e2e/api_auth_e2e_test.go::TestAPI_OIDCRoleMappingAtInstallTime`, `TestAPI_OIDCRoleMappingFirstLogin`, and **new audit event test** | api, audit, helm-values, web/specs.md |
| **4: Role Re-Evaluation** | FR-011 | SC-005: Role updated on next login when group membership changes | `test/e2e/api_auth_e2e_test.go::TestAPI_OIDCRoleMappingReEvalOnLogin` | api, web/specs.md |
| **5: Over-Broad Mapping Warning + **FR-015 Confirm** | **FR-015** | (supports SC-007 by exercising the DB-override editing surface, but does not itself prove it — see Scenario 8) | **new dashboard integration test** (or manual validation in `test/e2e/` via AdminSettings interaction) | dashboard, web/specs.md |
| **6: Backward Compatibility** | FR-003, FR-004, FR-012 | SC-008: Unset config defaults to cluster default and viewer role | `test/e2e/gameserver_e2e_test.go::TestGameServer_StorageClassExplicitOverride` (via null/empty case) and all api-auth tests with `roleMappings=nil` | operator, api, helm-values |
| **7: Admin Config Interface** | FR-006, FR-017, **D2 (API flag reporting)** | SC-006: Storage class viewable read-only; OIDC role mappings viewable with Helm-seed vs. DB-override provenance | (manual dashboard inspection; automated test in `test/e2e/api_config_e2e_test.go::TestAPI_GetConfig_InstallTimeSettings` if exists) | api, api-cli, api-http, helm-values, web/specs.md |
| **8: Hybrid Role Mapping (Overlay Wins, Helm Seeds) + **H1–H4, H6** | FR-007, FR-011, **H1, H2, H3, H4, H6** | **SC-007**: Admin writes `helmOverride.roleMappings` on the existing `auth` config row via `PUT /admin/config/auth`; the change takes effect on the affected user's next login with **no API restart and no `helm upgrade`** (Scenario 8a is the SC-007 automation, proved by the per-login read of `helmOverride` on the helm provider path — not the registry's DB-provider row-hash cache, which does not apply here since `OIDCFor` short-circuits past it for the helm provider) — a later `helm upgrade` changing the Helm-seeded value does **not** clobber the existing overlay entry (Scenario 8b, H3, M9); `DELETE /admin/config/auth/role-mappings/{role}` restores the seed (Scenario 8c) | `test/e2e/api_auth_e2e_test.go::TestAPI_OIDCRoleMappingDBOverrideTakesEffectNextLogin` (SC-007), `TestAPI_OIDCRoleMappingSurvivesHelmReseed` (H3), `TestAPI_OIDCRoleMappingResetToHelmDefault` | api, auth, audit, dashboard, helm-values, web/specs.md |

---

## Cleanup

After completing the scenarios, clean up test resources:

```bash
# Delete test namespaces
kubectl delete namespace gameplane-test-sc gameplane-test-nonexist gameplane-test-oidc gameplane-test-legacy

# If using kind, reset the cluster
kind delete cluster --name gameplane

# Or keep the cluster and re-run scenarios against the same deployment
```

---

## Troubleshooting

### OIDC login redirects to login page (infinite loop)

**Cause**: Redirect URL mismatch between Helm config and OIDC provider configuration.

**Fix**: Verify that `api.oidc.redirectURL` in Helm values matches the **Redirect URI** registered in your OIDC provider settings.

```bash
# Check what Gameplane is using
helm get values gameplane -n gameplane-test-oidc | grep redirectURL

# It should be: http://<your-api-domain>/auth/oidc/callback
```

### PVC remains Pending indefinitely

**Cause**: Storage class does not exist or is not provisioned correctly.

**Fix**:
1. Verify the storage class exists: `kubectl get storageclasses`
2. Check PVC events: `kubectl describe pvc <pvc-name> -n <namespace>`
3. Check provisioner logs if available

### Operator pod crashes on startup

**Cause**: Invalid Helm values passed to the operator (e.g., malformed OIDC role mapping JSON).

**Fix**:
1. Check operator logs: `kubectl logs deployment/gameplane-operator -n <namespace>`
2. Verify Helm values are well-formed: `helm lint charts/gameplane/`
3. Re-run `helm upgrade` with corrected values

---

## References

- **Specification**: [spec.md](./spec.md) — detailed requirements and acceptance criteria
- **Data Model**: [data-model.md](./data-model.md) — Helm values structure, CRD fields, database schema
- **Helm Chart**: [charts/gameplane/values.yaml](../../charts/gameplane/values.yaml) — current defaults and accepted keys
- **CLAUDE.md**: [CLAUDE.md](../../CLAUDE.md) — project conventions and code style
- **Documentation**: [docs/install.md](../../docs/install.md) — Helm installation guide (to be updated by this feature)
