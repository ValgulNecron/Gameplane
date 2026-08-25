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

## Scenario 5: Over-Broad OIDC Mapping Warning (FR-015, SC-007)

**Objective**: Verify that when an admin configures an OIDC role mapping that grants admin privileges, they receive a clear warning about the risk of unintended broad access, and must explicitly confirm before saving (FR-015).

**Prerequisite**: An admin user logged in via bootstrap-admin or existing OIDC role (so they can access AdminSettings). An OIDC provider is configured (or can be configured via the dashboard).

### Steps

1. **Navigate to Admin Settings**:
   - Click **Settings** > **Admin Settings**
   - Go to the **Auth** or **Providers** section
   - If a Helm-configured OIDC provider ("helm" or "Single sign-on") already exists, skip to step 3
   - If not, add a new OIDC provider (or edit an existing one for this test)

2. **Attempt to map a group to admin role**:
   - In the role mappings editor, find or create a mapping for the admin role
   - Enter a group name (e.g., `developers`, `all-users`, or any broad group name)
   - Click **Save** or **Update**

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
   - The mapping should be saved successfully
   - Verify it appears in the role mappings list

5. **Verify via API**:
   - Query `GET /admin/config` and verify the mapping appears in `auth.providers[]`:
     ```bash
     curl -s -H "Cookie: gameplane_session=<token>" \
       http://localhost:8000/api/admin/config | jq '.auth.providers[] | select(.name == "YOUR_PROVIDER_NAME") | .roleMappings'
     ```

### Validation Checklist

- [ ] Warning dialog appears when attempting to map a group to admin role
- [ ] Warning text is clear and mentions the risk of unintended broad access
- [ ] Confirm button is present and labeled "Confirm & Save" or similar
- [ ] Cancel button allows the user to abort the mapping
- [ ] After confirmation, the mapping is saved and persists in the dashboard
- [ ] Warning does NOT appear for operator or viewer role mappings
- [ ] Audit log shows the mapping change (if applicable)

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

**Objective**: Verify that install-time settings (storage class and OIDC role mappings) are visible and read-only in the admin configuration interface.

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

4. **Verify OIDC role mappings display**:
   - The section should display the Helm-configured OIDC provider (labeled "helm" or "Single sign-on"):
     - `Group Claim: groups`
     - `Admin Groups: ["gameplane-admins"]`
     - `Operator Groups: ["gameplane-operators"]`
     - `Viewer Groups: ["gameplane-viewers"]` (or default role)
   - The provider is listed as read-only and cannot be edited or deleted

5. **Check for "not configured" message**:
   - If no install-time settings are configured:
     - "Game Data Storage Class: (cluster default)"
     - "OIDC Role Mappings: Not configured. Use Helm chart values to set role mappings at install time."

### Validation Checklist

- [ ] Install-time settings are visible in the admin configuration interface
- [ ] Settings are displayed as read-only (no edit controls)
- [ ] Helm-configured OIDC provider is listed and marked as immutable
- [ ] No errors when viewing the configuration page
- [ ] Clear guidance is provided if settings are not configured

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
- `api/internal/auth/oidc_rolemap_test.go` — tests claim parsing, role mapping, deny logic
- `operator/internal/controller/gameserver_storage_envtest_test.go` — tests PVC storage class selection and error handling

### End-to-End Tests (CI)

The feature includes e2e tests in the CI pipeline:

**api-auth bucket** (login-pressure-constrained):
- `TestAPI_OIDCRoleMappingAtInstallTime` — verifies install-time role mapping loads correctly
- `TestAPI_OIDCRoleMappingFirstLogin` — verifies first user login gets correct role
- `TestAPI_OIDCRoleMappingReEvalOnLogin` — verifies role re-evaluation on group change

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
| **5: Over-Broad OIDC Mapping Warning + **FR-015 Confirm** | **FR-015** | **SC-007**: Warning appears when admin role mapping configured; explicit confirm required before save | **new dashboard integration test** (or manual validation in `test/e2e/` via AdminSettings interaction) | dashboard, web/specs.md |
| **6: Backward Compatibility** | FR-003, FR-004, FR-012 | SC-008: Unset config defaults to cluster default and viewer role | `test/e2e/gameserver_e2e_test.go::TestGameServer_StorageClassExplicitOverride` (via null/empty case) and all api-auth tests with `roleMappings=nil` | operator, api, helm-values |
| **7: Admin Config Interface** | FR-006, FR-017, **D2 (API flag reporting)** | SC-006: Settings viewable as read-only in admin UI; **storage class reported via GET /admin/config** | (manual dashboard inspection; automated test in `test/e2e/api_config_e2e_test.go::TestAPI_GetConfig_InstallTimeSettings` if exists) | api, api-cli, api-http, helm-values, web/specs.md |

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
