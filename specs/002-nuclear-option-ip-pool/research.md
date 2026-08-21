# Phase 0 Research: Nuclear Option Module & Load-Balancer IP Pool Override

**Date**: 2026-08-21  
**Verification Status**: Claims 1–5 resolved; LB pool research verified

---

## Summary

This document resolves the five **Verification Required Before Implementation** claims in `specs/002-nuclear-option-ip-pool/spec.md`, records the verified facts about the Nuclear Option protocol and platform, and consolidates load-balancer address-pool technical decisions.

**Critical finding**: The Nuclear Option remote-command protocol (claim 3) is **completely unauthenticated** — no password, token, or handshake. This is a hard security constraint that drives the agent implementation to never expose the remote-command port externally.

**Blocking risk resolved**: Nuclear Option dedicated server (Steam app 3930080) is downloadable without base-game ownership via `+login anonymous`, ships a native Linux binary, and can be deployed following the existing Gameplane pattern (SteamCMD at container start, no binary redistribution). Module can proceed to implementation.

---

## Part 1: Nuclear Option Claim Resolution

### Claim 1: Dedicated Server Availability & Platform

**Verdict**: VERIFIED  
**Source**: `/tmp/claude-1000/.../scratchpad/nog.md` (DedicatedServerGuide.md, publisher-official from Shockfront-Studios GitHub org)

**Evidence Quoted**:
- Line 14: `steamcmd +force_install_dir /home/steam/NuclearOptionServer +login anonymous +app_update 3930080 validate +quit`
  - ✓ App ID is **3930080**
  - ✓ `+login anonymous` proves **no base-game ownership required**
- Line 24: `chmod +x NuclearOptionServer.x86_64` — **native Linux binary confirmed**
- Line 26: `./RunServer.sh` — startup script included

**Residual unknowns**: None; baseline deployment path is unblocked.

**Module architecture** (no binary redistribution): Gameplane follows the pattern established by existing Steam-based modules (Palworld, Valheim, factorio). The template specifies a container image (e.g., a SteamCMD base or community wrapper) that installs the binary at container boot via `steamcmd +app_update 3930080` with `+login anonymous`. The Gameplane operator/agent never bundles or pushes the binary itself. Verified in `/home/valgul/project/kubernetes-game-dashboard/modules/palworld/template.yaml` lines 5–8, which show the same pattern: `"The game server itself is installed/updated by steamcmd on boot (UPDATE_ON_BOOT), so the FIRST start downloads several GB — the startup probe budgets for that."` This architecture satisfies licensing requirements (Gameplane does not redistribute) and keeps modules small (only wrapper/config, not multi-gigabyte binaries).

---

### Claim 2: Network Ports

**Verdict**: VERIFIED  
**Source**: `/tmp/claude-1000/.../scratchpad/nog.md` (DedicatedServerGuide.md)

**Evidence Quoted**:
- Lines 16–20:
  ```
  - **UDP ports** 
    - (defaults, can be changed in `DedicatedServerConfig.json`)
    - if running locally or behind firewall then these UDP ports might need to be opened...
    - game port `7777`
    - query port `7778`
  ```
  - ✓ Game port: **UDP 7777**
  - ✓ Query port: **UDP 7778**
  
- Lines 125–135 (port override config):
  ```json
  "Port": {
      "IsOverride": true,
      "Value": 7777
  },
  "QueryPort": {
      "IsOverride": true,
      "Value": 7778
  }
  ```
  - ✓ Both overridable via the `IsOverride`/`Value` pair in JSON config

- Remote-command port: From `/tmp/claude-1000/.../scratchpad/nocmd.md` line 3:
  ```
  Remote commands can be enabled by adding `-ServerRemoteCommands [port]` when running from command line. 
  If the port is not given then it will default to `7779`.
  ```
  - ✓ Launch flag: `-ServerRemoteCommands [port]`
  - ✓ Default port: **TCP 7779** (not UDP; see protocol claim below)

**Residual unknowns**: None for port numbers. All three ports are documented and confirmed.

---

### Claim 3: Remote-Command Protocol Format

**Verdict**: VERIFIED with **CRITICAL SECURITY FINDING**  
**Source**: `/tmp/claude-1000/.../scratchpad/nocmd.md` (ServerCommands/Readme.md, publisher-official)

**Evidence Quoted**:

**Protocol transport** (lines 5–7):
- Line 7: `All commands and responses are sent over a custom TCP format.`
  - ✓ Transport: **TCP** (not UDP)
  - ✓ Port: **7779** (default, configurable at launch)

**Request format** (lines 9–26):
- Line 11: `It is serialized into **UTF8 JSON** and is preceded by a **4-byte length** prefix.`
- Lines 24–27: `1. **4 Bytes:** Length of the JSON data (Little-Endian). 2. **Length Bytes:** UTF8 JSON string of the `CommandMessage` struct.`
  - ✓ **4-byte little-endian length prefix** + UTF8 JSON request

**Response format** (lines 31–35):
- Lines 31–35:
  ```
  **TCP Format:**
  1.  **4 Bytes:** The `StatusCode` as an integer (e.g., `2000` for Success).
  2.  **4 Bytes:** Length of the Json data (0 if no body is present).
  3.  **Length Bytes:** UTF8 JSON data (only present if length > 0).
  ```
  - ✓ **Asymmetric** response framing: 4-byte status code (int) + 4-byte JSON length + JSON body

**Status codes** (lines 38–50):
- Documented codes: 2000 (Success), 4000–4005 (client errors), 5000–5002 (server errors)
- All named and mapped

**19 moderation commands** (lines 56–407, exact list):
1. `update-ready` (line 56)
2. `send-chat-message` (line 67)
3. `reload-config` (line 82)
4. `get-mission-time` (line 97)
5. `get-mission` (line 120)
6. `get-server-id` (line 152)
7. `get-player-list` (line 171)
8. `set-time-remaining` (line 202)
9. `set-next-mission` (line 215)
10. `kick-player` (line 230)
11. `unkick-player` (line 245)
12. `clear-kicked-players` (line 259)
13. `banlist-reload` (line 270)
14. `banlist-add` (line 283)
15. `banlist-remove` (line 297)
16. `banlist-clear` (line 310)
17. `get-mission-rotation` (line 321)
18. `set-mission-rotation` (line 364)
19. `clear-next-mission` (line 399)

### **CRITICAL SECURITY FINDING: Unauthenticated Protocol**

**Verdict**: CONFIRMED — the remote-command protocol has **NO authentication whatsoever**.

**Evidence**: A complete read of `/tmp/claude-1000/.../scratchpad/nocmd.md` (409 lines) contains:
- No mention of "password", "auth", "token", "credential", "handshake", "secret", or "key"
- No login command, authentication request/response, or capability negotiation
- The only mention of "Authenticator" (line 312) is unrelated — it refers to an internal ban-list cache: `Clears the ban list loaded in the Authenticator`

**Implication**: Any TCP client that can open a connection to port 7779 can:
- Kick any player
- Ban any player
- Reconfigure the server
- End the mission
- Load arbitrary missions

**Security constraint** (must be reflected in agent implementation):
- The remote-command port **MUST NOT** be exposed publicly (not set `advertise: true` in the GameTemplate's port list)
- The remote-command port **MUST** be treated as pod-local only (accessible only by the agent sidecar via 127.0.0.1:7779)
- The agent must enforce connection origin validation or run the port on loopback exclusively

**Residual unknowns**: None for protocol specification. Wire format, command names, and status codes are all confirmed.

---

### Claim 4: Readiness Signal

**Verdict**: VERIFIED  
**Source**: `/tmp/claude-1000/.../scratchpad/nog.md` (DedicatedServerGuide.md)

**Evidence Quoted**:
- Line 299: `If you see this line, it is likely the server is running ok: `[DedicatedServerManager] Waiting for Players before loading next map`.`

**Interpretation**: This log line appears when the server has finished initialization and is ready to accept player connections. It is a reliable readiness signal.

**Residual unknowns**: None; this is a clear, documented log marker.

---

### Claim 5: On-Disk Log Location & Format

**Verdict**: VERIFIED (with standard caveat)  
**Source**: `/tmp/claude-1000/.../scratchpad/nog.md` (DedicatedServerGuide.md)

**Evidence Quoted**:
- Line 297: `With the default run arguments the server will create log files in the `logs` directory with the timestamp the server was started. (`./logs/server-$(date +%Y-%m-%d-%H-%M-%S).log`)`.

**Location**: `./logs/server-<timestamp>.log` (relative to the server executable directory)

**Format**: Plain-text log files, rotated by timestamp. Standard format compatible with log aggregation and backup.

**Configuration location** (line 38–39): `DedicatedServerConfig.json` is auto-created on first run with defaults; later edits do not require restart (can be reloaded via the `reload-config` remote command).

**Ban lists** (line 59): `BanListPaths` can reference one or more files (default: `ban_list.txt`), relative to the server executable or absolute paths.

**Mission directory** (line 100, Linux example): `/home/steam/NuclearOption-Missions` — absolute path where custom missions are stored.

**Residual unknowns**: None; location and format are clearly documented and standard.

---

## Part 1b: Player Display-Name Resolution (Track A)

### Decision NO-1: Hydrate player display names in the API server via the Steam Web API

**Status**: DECIDED by the maintainer. Spec FR-007 and User Story 3 / Acceptance Scenario 1 stand as written and are **not** amended. This closes the previously-open gate recorded in `plan.md`; the corresponding entry there now points at Key Technical Decisions → Decision 9.

**Problem**

Spec FR-007 and US3/AC1 promise a player list showing Steam ID, display name, and faction. The dedicated server cannot supply the display name. The publisher's own protocol documentation for `get-player-list` states the server "returns only the steamId and faction fields (the displayName field has been removed since the server runs headlessly and does not cache names)", and directs integrators to "fetch steam name using Steam's Web API" (source: `ServerCommands/Readme.md`, Shockfront-Studios `Nuclear-Option-Server-Tools`, saved locally as `nocmd.md` — the same publisher-official document used for Claim 3). So the field the spec promises exists nowhere in the game's wire protocol; it must come from Steam or not at all.

**Decision**

- The lookup lives in the **API server** (`api/internal/…`), never in the agent sidecar.
- The agent's contract is unchanged: it returns exactly what the game returns — `steamId` and `faction`. Name hydration is a presentation concern layered on in the API, consistent with the project rule that the API is a UX layer.
- Outbound calls dial through the in-repo `netguard` SSRF dial-guard using the **strict `IsPublic` policy** (the one the agent uses for mod downloads), because Steam's Web API is a public internet endpoint — not the permissive `IsAllowed` policy the operator uses for self-hosted registries.
- Endpoint: `ISteamUser/GetPlayerSummaries/v2`, which accepts up to 100 `steamids` per call. The resolver **batches** a whole player list into one request rather than one request per player.
- The Steam Web API key is an **optional** credential: a Kubernetes Secret surfaced through a Helm value, never logged, never returned to the browser, never committed.
- **Graceful degradation is mandatory.** Key absent, Steam unreachable, request timed out, or a single id unresolved — the player list still renders, with the raw Steam ID in the name column. Name resolution never blocks, fails, or errors the player-list response, and carries a hard bounded timeout so SC-004's 5-second moderation-command budget is met by degrading rather than waiting.
- Results are **cached with a TTL** so repeated player-list calls do not hammer Steam.
- **Steam ID remains the identifier.** Kick, ban, and unban continue to key on Steam ID; the display name is display-only and must never become the identifier used for a moderation action.

**Rationale**

The decisive constraint is network policy, not taste. `charts/gameplane/templates/networkpolicies.yaml` installs a `default-deny-egress` NetworkPolicy in the games namespace (policy at line 24, `podSelector: {}` at line 28) that applies to **every** pod in that namespace and opens only DNS; game pods reach the internet solely through the opt-in `allow-game-public-egress` policy (line 149) that exists for SteamCMD/asset/mod downloads. The API server runs in the control-plane namespace and already has an egress path, so putting the resolver there yields one egress path, one Secret, and one shared cache — instead of a new hole and a distributed credential in every game pod.

**Wiring note**: `api/` is *already* a netguard importer — `api/go.mod` lines 5–9 declare the requirement plus the local `replace`, and `api/internal/notify/notify.go:17` and `api/internal/notify/deliver.go:18` import it today. No new `go.work`/`go.mod` wiring is required. `netguard/.testcoverage.yml` (total 91%) still gates any change made inside that package for this feature; the resolver's own tests land under the `api` gate (80%).

**Alternatives considered**

1. **Resolve in the agent sidecar, next to the game.** *Rejected.* Blocked by the games-namespace `default-deny-egress` policy above: it would require punching a new outbound hole into every game pod, and it would distribute the Steam Web API key to every game pod — multiplying both the egress surface and the credential's blast radius for a cosmetic field. It would also put a third-party HTTP call inside the component whose job is to speak the game's protocol.
2. **Amend the spec to drop `displayName` from the player list** (show Steam ID and faction only). *Rejected by the maintainer*, who chose to keep the promised UX. This was the cheapest option — no credential, no third-party dependency, no cache — but it would have made the moderation UI a wall of 17-digit numbers, which is precisely the usability problem FR-007 exists to prevent.
3. **Resolve names in the browser, calling Steam directly from the dashboard.** *Rejected.* It would expose the API key to every client (or require an unauthenticated proxy), makes name display dependent on each operator's browser being able to reach Steam, and provides no shared cache.

**Operational consequences (recorded honestly)**

- **New configuration surface.** An optional Steam Web API key becomes a new thing to provision, document, rotate, and support. Installs that skip it are a first-class supported configuration, not a broken one.
- **The feature degrades to raw Steam IDs** whenever the key is absent, Steam is down, or the call times out. Operators will see IDs instead of names and must not read that as a Gameplane fault; the dashboard copy and module `specs.md` should say so plainly.
- **A third-party runtime dependency on Steam's availability** is introduced for a cosmetic field. This is the real cost of choosing this design over rejected alternative 2 above. It is bounded by the mandatory timeout, the TTL cache, and the rule that name resolution can never fail the player-list response — but the dependency is now permanently in the request path for that view.
- **Rate limits and quotas.** Steam Web API keys are rate-limited per key; batching (100 ids/call) plus the TTL cache is what keeps a busy multi-server install inside that budget.

---

## Part 2: Load-Balancer Address-Pool Technical Decisions

This section documents the LB-pool feature's design decisions after upstream research.

### Load-Balancer Managers & Pool Selection Mechanisms

**Decision 1: Support multiple address managers via a typed, flavor-aware operator approach**

**Rationale**:
- Different LB managers use fundamentally different mechanisms to select pools:
  - **MetalLB**: pool selection via **annotation** `metallb.io/address-pool: <name>` on the Service (or legacy prefix `metallb.universe.tf`)
  - **Cilium LB-IPAM**: pool selection via **label** on the Service, matched by `CiliumLoadBalancerIPPool.spec.serviceSelector` — Cilium does not mandate a canonical label key, so Gameplane uses `gameplane.local/lb-pool` (a Gameplane convention) which the cluster administrator must configure their pool CRDs to match
- A single `serviceAnnotations` map cannot express both; the operator must translate a typed field into the right annotation or label based on cluster config
- The operator must know the cluster's LB flavor (MetalLB, Cilium, or unmanaged) to apply the correct mechanism

**Decision**: 
- Add typed fields `addressPool` and `address` to `GameServerNetworking` (in `operator/api/v1alpha1/gameserver_types.go`)
- Cluster-level configuration specifies the LB flavor (enum: `metallb` | `cilium` | `none`)
- The operator reconciler translates `addressPool` into the appropriate annotation (MetalLB) or label (Cilium) when creating the Service
- Keep the existing `serviceAnnotations` map as an escape hatch for edge cases and future managers

**Alternatives considered**:
1. **Annotations-only approach** — no typed field, operator-agnostic but poor UX (users hand-edit JSON, no validation, no error surfacing)
2. **Typed-only, bespoke per manager** — clean UX but couples the operator code to two (or more) vendors; hard to extend to a third LB manager without a code change
3. **Accept both annotations and typed fields** — typed field is the preferred path, but `serviceAnnotations` escapes hatch for operators who want to use other managers (e.g., custom in-house controller, or future CNCF standard if one emerges)

**Selected**: Typed field + operator translation + annotations escape hatch (option 3).

---

### Explicit IP Address Requests

**Decision 2: Support operator-requested explicit IP addresses via a separate field**

**Rationale**:
- Some operators need stable, published addresses (community rosters, DNS records, compliance requirements)
- Pool-name selection alone cannot guarantee the same address across restarts
- Both MetalLB and Cilium support explicit IP requests:
  - **MetalLB**: annotation `metallb.io/loadBalancerIPs: <ip[,ip]>` (or legacy `metallb.universe.tf`)
  - **Cilium**: annotation `lbipam.cilium.io/ips` (or older 1.13–1.14 `io.cilium/lb-ipam-ips`)
- Explicit IP can coexist with pool selection (operator specifies both a preferred pool and a preferred IP; if the IP is available and in that pool, use it; otherwise, fall back to the pool)

**Decision**:
- Add an optional `address` field to `GameServerNetworking` alongside `addressPool`
- If both are set, prefer the explicit address if it is available and within the specified pool
- If only pool is set, assign from that pool
- If only address is set, assign that specific address (regardless of pool, but report which pool it came from if available)
- If neither is set, assign from the default pool (backward compatible)

**Alternatives considered**:
1. **Pool name only** — simpler but doesn't support stable addresses for community listings
2. **Address only** — simpler but less useful for multi-pool clusters; operators want to segregate by tier/region (pool) and then pin a specific IP within that pool
3. **Pool XOR address** — mutually exclusive fields; harder to use if an operator wants both
4. **Selected: Pool AND address** (option 3) — allows operators to express sophisticated preferences without forcing a choice

---

### Cilium Label vs. MetalLB Annotation Asymmetry

**Decision 3: Operator must set a label for Cilium and an annotation for MetalLB from the same typed field**

**Rationale**:
- **MetalLB** selects pools by annotation on the Service (e.g., `metallb.io/address-pool`)
- **Cilium** selects pools by label on the Service, matched by `CiliumLoadBalancerIPPool.spec.serviceSelector` — but Cilium does NOT define a canonical label key
- A single typed field (e.g., `addressPool: "production-us-east"`) must be translated differently per manager:
  - MetalLB: set annotation `metallb.io/address-pool: production-us-east`
  - Cilium: set label **`gameplane.local/lb-pool: production-us-east`** — **this is a Gameplane-chosen convention**, not a Cilium standard
- **Critical requirement**: The cluster administrator MUST configure their `CiliumLoadBalancerIPPool.spec.serviceSelector` to include the label key `gameplane.local/lb-pool` in order for pool selection to work. If this selector is not configured, the label is applied but nothing matches it, and the Service receives an address from the default pool (or none) — exactly the kind of silent failure this project avoids.
- This asymmetry is **the main implementation risk** in Track B; the operator reconciler must:
  1. Read the flavor from cluster config
  2. Set the right annotation or label
  3. Avoid setting the wrong one (e.g., don't set the MetalLB annotation on a Cilium cluster)

**Decision**: Document this as the primary source of bugs and test it explicitly for each flavor. In status error messages for Cilium, acknowledge that "pool not found" is ambiguous: it could mean the pool CRD does not exist, OR the pool CRD exists but the `spec.serviceSelector` does not match our `gameplane.local/lb-pool` label.

---

### Failure Surfacing

**Decision 4: LB pool assignment failures must surface as distinct, actionable error messages**

**Rationale**:
- If a pool doesn't exist, the Service will stay in "Pending" forever (no IP assigned)
- If a pool is exhausted, same outcome
- If a requested explicit IP is already in use, same outcome
- Operators need to understand why their request failed so they can fix it
- Different LB managers surface failures differently:
  - **MetalLB**: emits Warning **Events** on the Service
  - **Cilium**: sets Service **Condition** entries with reasons
- The operator must watch these signals and translate them into a GameServer **status condition** with a clear message

**Decision**:
- Operator polls Service events/conditions at reconcile time
- If the Service has no assigned IP and a pool was requested, parse the LB manager's error message
- Set a GameServer condition (e.g., `type: PoolAssignmentFailed`, `reason: "PoolNotFound"`, `message: "Address pool 'production-us-east' not found in cluster"`)
- Dashboard displays this condition prominently in the server's networking details

**Success criteria**: Within 30 seconds of a misconfiguration, the dashboard shows a specific error (not generic "Pending").

---

### Backward Compatibility & Default Behavior

**Decision 5: Servers without a pool preference continue to work unchanged**

**Rationale**:
- Existing servers have no pool preference set
- If they receive an address from the default pool today, they must continue to do so after the feature ships
- No migration or data conversion required
- Feature is purely additive

**Decision**: If `addressPool` and `address` are both unset (the default), the operator takes no action and allows the LB manager to assign from its default pool (the status quo).

---

### Compatibility Across Managers

**Decision 6: Feature must work with any CNCF-standard LoadBalancer address manager**

**Rationale**: Per FR-022, Gameplane should not couple itself to one vendor. MetalLB and Cilium are the most common, but others exist (cloud-provider LBs, future CNCF standards).

**Decision**:
- Typed field + operator translation is the primary path for MetalLB and Cilium
- For unknown/unmanaged managers, the `serviceAnnotations` escape hatch allows operators to set arbitrary annotations
- If a cluster has no LB manager, or uses Ingress-only, the feature has no effect and the dashboard indicates this clearly

---

## Part 3: Consolidated Open Questions & Implementation Gates

The following items remain genuinely unresolved and will be addressed during implementation:

1. **Agent RCON implementation for Nuclear Option** (track-specific)
   - The existing agent's RCON allowlist (source, telnet, websocket, battleye, satisfactory, palworld, none) does NOT include the Nuclear Option protocol
   - A new protocol implementation is required in `agent/internal/rcon/`
   - Specific risk: The asymmetric response framing (4-byte status code + 4-byte length + body) is easy to implement wrong; test against a real running server before committing

2. **Module resource footprint** (track-specific)
   - Spec assumes 8–16 GB RAM, 2–4 CPU cores, 30 GB storage
   - Must verify on a real cluster; if resource requirements exceed CI runner capacity, E2E test is authored but excluded from default CI buckets

3. **Mission rotation validation** (track-specific)
   - When an operator sets a mission via remote command or config edit, the system must validate that the mission exists in the game's current mission list
   - This validation requires either parsing the game's mission database or calling a server-side query command
   - Exact mechanism TBD during implementation

4. **LB pool existence validation** (track B)
   - At reconcile time, the operator must detect if a requested pool exists in the cluster's LB manager
   - For MetalLB: query the `IPAddressPool` CRD list
   - For Cilium: query the `CiliumLoadBalancerIPPool` CRD list
   - For unknown managers: trust the operator knows the pool name; rely on the LB manager's own error events

5. **Address exhaustion handling** (track B)
   - If a pool is exhausted, the LB manager will not assign an address
   - Operator detects this via the Service's missing `status.loadBalancer.ingress[].ip`
   - Must surface the error within 30 seconds; exact backoff/retry strategy TBD

6. **Service port remapping for remote-command** (track-specific)
   - The remote-command port (7779) must NOT be advertised externally
   - Template must set `advertise: false` on the remote-command port
   - Agent reaches it pod-locally; no Service exposure
   - This is non-negotiable per the security finding (unauthenticated protocol)

7. **Multi-pool assignment semantics** (track B)
   - If operator specifies both pool and explicit IP, behavior is:
     - If IP is available AND in the specified pool: assign it
     - If IP is available but NOT in the specified pool: ??? (fail, or allow out-of-pool assignment?)
     - If IP is in use: fail with conflict error
   - Exact semantics TBD; recommend: fail if not in pool (operator made a conflicting request)

8. **Readiness probe for Nuclear Option** (track-specific)
   - Spec assumes readiness can be probed via the log line or a TCP port
   - Template must specify appropriate startup/readiness/liveness probes
   - SteamCMD first-boot may take 10+ minutes; startup probe must budget for this
   - Recommend: TCP probe to remote-command port (7779) as readiness signal (ensures server is listening and accepting RCON)

---

## Verification Checklist

Before implementation begins, confirm:

- [x] **Claim 1**: Nuclear Option app 3930080, `+login anonymous`, native Linux binary — **VERIFIED**
- [x] **Claim 2**: Ports UDP 7777, 7778, TCP 7779 configurable — **VERIFIED**
- [x] **Claim 3**: Remote-command protocol is length-prefixed JSON over TCP, 19 moderation commands, **completely unauthenticated** — **VERIFIED with CRITICAL SECURITY FINDING**
- [x] **Claim 4**: Log line `[DedicatedServerManager] Waiting for Players before loading next map` indicates readiness — **VERIFIED**
- [x] **Claim 5**: Config and log locations documented, standard format — **VERIFIED**
- [x] **LB pool architecture**: MetalLB (annotation-based), Cilium (label-based), typed operator translation — **VERIFIED upstream, decisions recorded**
- [x] **Licensing**: Gameplane does not redistribute binaries; SteamCMD downloads on container start following established pattern (Palworld, Valheim, factorio) — **VERIFIED**

---

## References

**Official sources**:
- GitHub org: `Shockfront-Studios` (github.com/Shockfront-Studios)
- Repository: `Nuclear-Option-Server-Tools` (public, verified HTTP 200)
- Documents used:
  - `DedicatedServerGuide.md` (saved locally as `nog.md`)
  - `ServerCommands/Readme.md` (saved locally as `nocmd.md`)

**Upstream LB manager documentation** (verified by prior agent research):
- MetalLB v0.14.9+: pool via annotation `metallb.io/address-pool: <name>`; explicit IP via `metallb.io/loadBalancerIPs`
- Cilium LB-IPAM: pool via label matched by `CiliumLoadBalancerIPPool.spec.serviceSelector`; explicit IP via annotation `lbipam.cilium.io/ips`
- Kubernetes core: `spec.loadBalancerClass` selects implementation; `spec.loadBalancerIP` (deprecated since v1.24) is not used

**Project codebase references**:
- Existing Steam-based module pattern: `/home/valgul/project/kubernetes-game-dashboard/modules/palworld/template.yaml` (SteamCMD on boot, no binary redistribution)

---

**Document approved for implementation planning.**
