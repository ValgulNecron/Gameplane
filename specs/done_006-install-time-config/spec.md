# Feature Specification: Install-Time Configuration (Storage Class & OIDC Role Mapping)

**Feature Branch**: `006-install-time-config`

**Created**: 2026-08-22

**Status**: Draft

**Input**: User description: "make the default storage class for gamedata configurable and not only use the default class. also fix oidc to have the policy mapping out of the box without ever needing an admin account."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Operator enables OIDC with role mappings at install time without running bootstrap-admin (Priority: P1)

An operator wants to deploy Gameplane with OIDC authentication only — no local password accounts, relying entirely on their organization's OIDC provider (e.g., Okta, Azure AD, Keycloak). They want to configure role mappings at install time (e.g., "users in the 'gameplane-admins' LDAP group get the admin role") so that the first user to log in receives the correct role without needing to run a manual bootstrap-admin command. Today, configuring role mappings requires accessing the dashboard's `/admin/config` page, which itself requires the admin role — creating a chicken-and-egg problem for OIDC-only installs.

**Why this priority**: This is a critical setup blocker for OIDC-only deployments. Without it, installing Gameplane with OIDC enabled and no bootstrap-admin results in every user receiving the viewer role permanently, with no in-dashboard route to fix it. It is a silent failure — the install completes successfully, OIDC works, but access control is broken. This blocks the entire "OIDC-only" operator story and must be resolved at install time.

**Independent Test**: Can be tested independently by installing Gameplane with OIDC enabled and role mappings pre-configured (e.g., via Helm values), without running bootstrap-admin, and then logging in as a user whose OIDC group/claim matches an admin mapping. That user must immediately receive the admin role and have access to all admin features without any additional configuration step. Can also test that a user whose group does not match any mapping receives the default role (viewer) as expected.

**Acceptance Scenarios**:

1. **Given** an OIDC provider (e.g., Okta) with users in groups "gameplane-admins", "gameplane-operators", and "gameplane-viewers", **When** an operator installs Gameplane with OIDC enabled and specifies role mappings (e.g., "users in 'gameplane-admins' get admin role", "users in 'gameplane-operators' get operator role", "everyone else gets viewer role"), **Then** a user in the "gameplane-admins" group logs in for the first time and immediately receives the admin role without any manual intervention or bootstrap-admin command.
2. **Given** a user logging in whose OIDC group/claim does not match any configured role mapping, **When** the user attempts to log in, **Then** the user receives the default role (viewer) and can view the dashboard but cannot make changes or access admin features.
3. **Given** a running Gameplane install with OIDC role mappings pre-configured at install time, **When** a user's OIDC group membership changes (e.g., promoted to admin in the OIDC provider), **Then** on the user's next login, their Gameplane role is re-evaluated and updated to match the new group membership.
4. **Given** an operator installing Gameplane with OIDC enabled but no role mappings configured (leaving role mappings unconfigured or empty), **When** a user logs in, **Then** the user receives the default role (viewer) and an admin can view a clear indicator in the admin configuration interface that role mappings are not configured, with guidance on how to configure them.
5. **Given** a bootstrap-admin command is run on an existing install with OIDC role mappings already configured, **When** the admin account is created, **Then** it coexists peacefully with the OIDC role mappings and does not interfere with users logging in via OIDC.

---

### User Story 2 - Operator configures a default storage class for game data at install time (Priority: P2)

An operator is setting up Gameplane on a cluster where the default StorageClass is not suitable for game data — e.g., the default is network-attached storage (NAS) but game servers need local NVMe for latency-sensitive workloads. Rather than editing every GameTemplate or GameServer after install to override the storage class, they specify a preferred storage class in the Helm chart at install time, and all game servers use that class by default.

**Why this priority**: This is a day-1 operational feature that eliminates repetitive manual edits post-install. It addresses a clear usability gap but has an existing workaround (edit GameTemplates after install). It is lower priority than the OIDC setup blocker, which is a silent failure mode.

**Independent Test**: Can be tested independently by installing Gameplane with a custom default storage class specified, creating a GameServer, and verifying that its game-data PVCs request that storage class (and only that class, not the cluster default). Can also be tested by changing the default at install time and confirming that new GameServers pick up the new default while existing ones' PVCs remain unchanged (because PVCs are immutable).

**Acceptance Scenarios**:

1. **Given** a cluster with two StorageClasses ("fast-local-nvme" and "standard-network"), where "standard-network" is the cluster default, **When** an operator installs Gameplane and specifies "fast-local-nvme" as the game-data storage class preference, **Then** a GameServer created from any template uses "fast-local-nvme" for its game-data PVCs, not the cluster default.
2. **Given** a GameServer already running with PVCs bound to "fast-local-nvme", **When** the operator updates the Helm chart to specify a different storage class ("gpu-attached-storage"), **Then** the existing server's PVCs remain on "fast-local-nvme" (PVCs are immutable and do not retarget) and only newly created GameServers use "gpu-attached-storage".
3. **Given** an operator installing Gameplane without explicitly specifying a game-data storage class, **When** a GameServer is created, **Then** the server's game-data PVCs use the cluster's default StorageClass (backward-compatible behavior, unchanged from today).
4. **Given** an operator installing Gameplane and specifying a storage class name that does not exist on the cluster (e.g., "nonexistent-class"), **When** a GameServer is created, **Then** the pod enters a Pending state and the dashboard clearly shows the reason: "StorageClass 'nonexistent-class' not found; verify the class exists on the cluster."

---


### Edge Cases

- **Storage class does not exist**: GameServers created with a nonexistent storage class should enter a Pending state with a clear error message. The error must be visible in the dashboard status, not buried in pod events.
- **Storage class is removed from cluster after install**: Existing GameServers' PVCs remain bound to the removed class (PVCs are immutable); new GameServers cannot be created if the configured class is gone. The dashboard must show this error.
- **OIDC group claim changes after install**: If the OIDC provider's group/claim structure changes (e.g., groups move from "groups" claim to "membership" claim), the mappings configured under the old structure stop working. The system must either automatically re-evaluate on next login (using the configured claim name) or show a clear error that the configured claim is not present in the token.
- **User's role is upgraded (viewer → operator → admin) via OIDC**: On each login, the user's role must be re-evaluated against current mappings and group membership. A user cannot be "downgraded" to a lower role without an explicit change to the OIDC mappings or group membership.
- **Misconfigured OIDC role mappings leave no admin**: If all admin-role mappings are deleted or misconfigured such that nobody can reach admin, the bootstrap-admin escape hatch exists. An admin who runs bootstrap-admin can then fix the mappings.
- **Install with OIDC role mappings, then later want to add bootstrap-admin admin**: Both mechanisms coexist; bootstrap-admin creates a local account while OIDC mappings drive OIDC users. A bootstrap-admin account and an OIDC-mapped admin can both exist.
- **API token holder accesses OIDC-only install**: API tokens (not OIDC) are created by admin users and can be used for automation. If OIDC role mappings leave nobody with admin, an API token created before that state was lost cannot be regained without bootstrap-admin. This is acceptable (token management is a separate feature).
- **Install-time OIDC role mapping is only applied on first login per user**: Existing users who logged in before the mapping was installed are not retroactively updated; they retain their old role until their next login (at which point the new mapping is applied). This is acceptable and simplifies implementation.
- **Over-broad OIDC group mapping grants unintended admin access**: If an operator configures a mapping such that a large LDAP organizational unit or directory group receives admin access (e.g., mapping the entire "employees" group to admin role), the system must warn of this risk or require confirmation. A mapping that unintentionally grants admin to hundreds of users is a security misconfiguration that should be flagged at configuration time, not discovered in production.

## Requirements *(mandatory)*

### Functional Requirements

**Storage Class Configuration:**

- **FR-001**: Operators MUST be able to specify a default storage class name for game-data volumes via the Helm chart values at install time.
- **FR-002**: When a GameServer is created (or a GameTemplate's storage settings are applied to a GameServer), the server's game-data PVCs (including the game binary volume, mission data, and any extra volumes declared in the template) MUST request the configured default storage class, not the cluster's default StorageClass.
- **FR-003**: The configured storage class setting MUST apply only to newly created PVCs; existing PVCs for running GameServers remain unchanged (PVCs are immutable after binding).
- **FR-004**: If a GameTemplate or GameServer explicitly sets a storage class name via its own settings (overriding the default), that explicit setting MUST take precedence over the install-time default.
- **FR-005**: When a GameServer's PVC creation fails because the specified storage class does not exist on the cluster, the GameServer status MUST display a clear, actionable error message (e.g., "StorageClass 'fast-nvme' not found on cluster") rather than remaining indefinitely in a Pending state.
- **FR-006**: Operators MUST be able to view the current default storage class setting via the administrative configuration interface or via querying Helm values.

**OIDC Role Mapping Configuration:**

- **FR-007**: Operators MUST be able to specify OIDC role mappings (group/claim → role) via the Helm chart values at install time, without requiring bootstrap-admin to be run.
- **FR-008**: The Helm values MUST include fields for: (a) the OIDC group claim name (e.g., "groups", "membership", "roles"), (b) a list of group-to-role mappings (e.g., "group 'gameplane-admins' → admin role"), (c) a default role for users who do not match any group mapping (defaulting to "viewer").
- **FR-009**: A user logging in via OIDC with pre-configured role mappings MUST have their role determined by the mapping rule at first login, with no additional manual admin step required.
- **FR-010**: When a user logs in, their OIDC token's group/claim value MUST be compared against the configured mappings, and their role MUST be assigned based on the first matching rule. If no rule matches, the default role MUST be assigned.
- **FR-011**: On each subsequent login, the user's role MUST be re-evaluated against the current mappings and group membership; if the user's group membership has changed in the OIDC provider, their Gameplane role MUST be updated to match.
- **FR-012**: If no role mappings are configured and an operator logs in via OIDC, the user MUST receive the default role (viewer). An administrator viewing the administrative configuration interface MUST see a clear indicator that OIDC role mappings are not configured, with guidance on how to configure them.
- **FR-013**: The bootstrap-admin break-glass mechanism MUST remain functional and documented, allowing an operator to create a local admin account even if OIDC role mappings are misconfigured. Bootstrap-admin and OIDC-mapped users MUST coexist peacefully (both are valid ways to be admin).
- **FR-014**: Role assignments and changes to a user's role arising from OIDC mappings MUST be recorded as auditable events in the API audit log, including the user's name, new role, and the mapping rule that triggered the assignment.
- **FR-015**: Operators configuring OIDC role mappings MUST be warned of the risk that overly broad mappings (e.g., mapping an LDAP organizational unit with hundreds of members to admin) will grant admin access to more users than intended. The specification MUST require validation or confirmation when a single mapping covers a non-trivial number of users.

**Cross-Cutting:**

- **FR-016**: Documentation MUST describe how to configure both features at install time, including worked examples for common OIDC providers (e.g., Okta, Azure AD) and typical storage class configurations.
- **FR-017**: Operators MUST be able to view the current default storage class and active OIDC role mappings via the administrative configuration interface for transparency, allowing admins to verify what is configured without re-reading Helm values or accessing database records directly.

### Key Entities

- **Storage Class (Kubernetes)**: A Kubernetes resource defining provisioning parameters for persistent volumes (e.g., storage type, access mode, provisioner). Only one can be marked as the cluster default; Gameplane allows overriding it per install.
- **Game-Data PVC (Persistent Volume Claim)**: A claim for persistent storage used by a GameServer for game binary, mission data, and other persistent state. When created, it references a StorageClass.
- **OIDC Provider**: An external identity provider (e.g., Okta, Azure AD, Keycloak) that issues OIDC tokens containing user information and group claims.
- **OIDC Token**: A JWT issued by the OIDC provider, containing claims (subject, name, email, group/roles, etc.). Gameplane uses the group/role claim to determine access control.
- **Group/Claim Name**: The name of the OIDC token claim that contains the user's group membership or role list (e.g., "groups", "roles", "membership"). Different OIDC providers use different claim names.
- **Role Mapping**: A rule mapping an OIDC group/claim value to a Gameplane role (admin, operator, viewer). Multiple mappings can be configured, forming a decision tree.
- **Default Role**: The Gameplane role assigned to a user if their OIDC group/claim does not match any configured mapping rule (typically "viewer").
- **Helm-Owned Provider**: An OIDC provider configuration sourced entirely from Helm values (via CLI flags to the API) at install time, read as a synthetic provider at runtime, and not stored in the API database. This provider's configuration is immutable through the dashboard and persists only in Helm values / cluster configuration.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An operator can install Gameplane with a game-data storage class specified in Helm values and have all GameServers created afterward use that storage class for their PVCs (100% of new PVCs target the specified class).
- **SC-002**: A GameServer with a nonexistent storage class shows a clear, specific error message in the dashboard status within 30 seconds of pod scheduling (not a generic "Pending" or a 10-minute timeout).
- **SC-003**: An operator can install Gameplane with OIDC enabled and role mappings pre-configured (via Helm values, no bootstrap-admin run), and a user whose OIDC group matches an admin mapping logs in and immediately receives admin role and dashboard access.
- **SC-004**: On a fresh Gameplane install with OIDC role mappings configured, the first user to log in (via OIDC, not bootstrap-admin) receives a role consistent with their group membership in the OIDC provider — no manual role assignment is required.
- **SC-005**: When a user's OIDC group membership changes in the provider (e.g., promoted to a group that maps to admin), their Gameplane role is updated on their next login (within 5 minutes if they are actively using the dashboard, or automatically on next login if they are not).
- **SC-006**: An operator can view the current default storage class and OIDC role mappings via the administrative configuration interface without reading Helm values or database records directly.
- **SC-007**: An operator can manage role mappings through the administrative configuration interface with changes taking effect for the next login attempt without restarting the API or re-running Helm.
- **SC-008**: Backward compatibility is preserved: Gameplane installs without an explicit storage class setting default to the cluster's default StorageClass; Gameplane installs with OIDC enabled but no role mappings configured default all OIDC users to the viewer role.

## Assumptions

- **Default storage class exists and is suitable for the operator's needs**: The Helm value accepts any string; operators are responsible for naming a class that exists on their cluster. If the class does not exist, PVCs fail to bind and the error is clearly reported.
- **OIDC provider exposes group/role claims**: The operator's OIDC provider is configured to include group or role claims in issued tokens. If the configured claim name does not appear in tokens, users will not match any mapping rule and receive the default role (this is acceptable; the operator must correct the claim name).
- **Helm-configured OIDC provider is a separate, synthetic provider not stored in the API database**: Following the existing `HelmProviderName` convention in the codebase (api/internal/auth/registry.go), OIDC configuration supplied via Helm values is synthesized as a reserved provider at runtime, read directly from Helm/environment configuration on each API startup, and never persisted to the API database. This provider is non-editable through the dashboard and not subject to divergence from Helm values. The Helm-configured provider coexists with database-stored providers but occupies a separate control path; users cannot edit, delete, or override the Helm provider through the UI.
- **Bootstrap-admin remains accessible as a break-glass mechanism**: If OIDC role mappings are misconfigured and nobody can reach admin, an operator can run bootstrap-admin to create a local admin account and fix the mappings. This is documented and expected.
- **Coexistence of OIDC and bootstrap-admin admin accounts is acceptable**: An operator can have both an OIDC-mapped admin and a bootstrap-admin admin account simultaneously; both are valid ways to be admin.
- **Role re-evaluation happens at login time, not in real-time**: If a user's OIDC group membership changes, their Gameplane role updates on their next login, not immediately. There is no real-time sync between OIDC provider and Gameplane; it is eventual-consistent per login.
- **The storage class configuration applies only to game-data volumes**: The API's SQLite PVC (if present, when api.db.driver=sqlite) continues to use the api.storage.storageClassName Helm value, independently of the game-data storage class setting. [Assumption: both settings coexist and are independent]
- **No quota or multi-tenancy on storage classes**: Any operator with GameServer-create permissions can request any storage class. There is no per-user or per-namespace storage class quota or allowlist in v1.
- **Existing GameServers and PVCs are not affected by install-time settings**: If the default storage class is changed after install (via Helm upgrade), existing PVCs remain bound to their original class. Only new PVCs use the new default. This is Kubernetes PVC immutability; Gameplane does not override it.

## Verification Required Before Implementation

**Claim 1: Helm chart value structure for storage class**

*Unverified:* The Helm chart values currently support an api.storage.storageClassName for the API SQLite PVC. Whether adding a parallel gameserver.storage.storageClassName or similar is the right place (vs. a more global default) requires reviewing the chart's current structure and design.

*Evidence Required:* Review of charts/gameplane/values.yaml and charts/gameplane/templates/operator.yaml to confirm where the storage-class default should be added (Operator pod config, Helm hook, or API config sync).

*Fallback if False:* Adjust the Helm value key name to match the chart's existing conventions.

**Claim 2: Helm-configured provider flow and immutability**

*Verified (via code review):* The `HelmProviderName` convention (api/internal/auth/registry.go) establishes that Helm-configured OIDC providers are synthesized at runtime from CLI flags and never persisted to the database. The provider is listed as read-only and cannot be edited or deleted through the dashboard.

*Implementation Concern:* Ensure the operator cannot accidentally create a database provider with the reserved name `helm` or otherwise create naming conflicts. Validate provider creation to reject the reserved name.

**Claim 3: OIDC group claim name configurability**

*Unverified:* The current OIDC setup assumes a fixed claim name (e.g., "groups"). Whether this is flexible (configurable per install) or hardcoded requires checking api/internal/auth/oidc.go.

*Evidence Required:* Review of the OIDC provider initialization to confirm whether the claim name is configurable and if so, where it should be set (Helm value, database config, environment variable).

*Fallback if False:* If the claim name is hardcoded, it must be made configurable as part of this feature.

---

## Out of Scope

- **Custom storage provisioners or StorageClass creation**: Gameplane does not create StorageClasses; operators must create them beforehand via cluster tools (kubectl, Helm, etc.).
- **Storage quota limits per GameServer or per namespace**: No per-server or per-namespace storage quota enforcement is in scope for v1.
- **Dynamic storage class migration**: Gameplane does not provide tooling to migrate running GameServers from one storage class to another; PVCs are immutable by design.
- **OIDC group nesting or LDAP hierarchy**: Group mappings are simple string matches; nested groups or hierarchical organization (e.g., "org/team/subteam") are not supported natively.
- **Role-based storage class selection**: Different roles do not have different storage class defaults; the default is cluster-wide for all GameServers.
- **IPv6 or dual-stack considerations for storage**: Storage classes are independent of network configuration.
- **Multi-cluster OIDC synchronization**: OIDC role mappings are cluster-local; no federation or sync to other clusters.
- **Advanced OIDC features**: Refresh token rotation, token introspection, PKCE (if applicable), and other advanced OAuth/OIDC flows are out of scope; Gameplane uses a standard library (coreos/go-oidc/v3) and does not customize it.
