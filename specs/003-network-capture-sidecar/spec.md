# Feature Specification: Network Capture Sidecar for Game Servers

**Feature Branch**: `003-network-capture-sidecar`

**Created**: 2026-08-17

**Status**: Draft

**Input**: User description: "server side network capture with opt in side car for specific game server"

## Motivation

Feature #001 (Game Protocol E2E Coverage) established that 12 of 16 shipped game modules are blocked on protocol documentation. Of those 12, five modules have recorded their specific blocker as requiring a packet capture of a real client joining: **garrys-mod**, **factorio**, **cs2**, **project-zomboid**, and **v-rising**. See `docs/game-coverage.md` for the full blocker list and `docs/game-coverage.md` "Documentation Blockers" section for capture-specific blockers. This feature is the operator tool that produces those protocol-discovery artifacts.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Initiate a packet capture of a real player join and retrieve the artifact (Priority: P1)

An operator needs to capture raw game-protocol traffic from a real player connecting to an instrumented server so they can record the wire format for protocol reverse-engineering. The operator starts a manual capture with an optional filter (e.g. restrict to one client address or port), lets the player join, stops the capture, and downloads the resulting packet file for analysis.

**Why this priority**: This is the core use case — the reason the feature exists. Protocol discovery depends on this workflow. Without a working capture-and-download path, the feature is inert.

**Independent Test**: Can be tested independently by booting a GameServer with capture enabled, initiating a capture with a filter expression, sending traffic matching that filter, stopping the capture, downloading the resulting file, and verifying the file is readable and contains packets matching the filter criteria.

**Acceptance Scenarios**:

1. **Given** a GameServer that has capture opt-in enabled, **When** an admin initiates a packet capture with a filter expression, **Then** the capture starts and reports a capture ID.
2. **Given** an active capture, **When** network traffic matching the filter expression flows through the pod, **Then** packets matching the filter are recorded.
3. **Given** a running capture, **When** the operator stops it (or the max duration elapses), **Then** the capture terminates and a packet file is produced.
4. **Given** a completed capture within the retention window, **When** the admin downloads the packet file via the API, **Then** the file is returned in a standard packet format (pcap or pcapng) readable by standard third-party packet-analysis tooling.

---

### User Story 2 - Enable and disable capture capability per GameServer (Priority: P2)

An operator should be able to opt a specific GameServer into packet-capture capability, and later opt it out, without modifying or redeploying the game server. Enabling capture adds the sidecar; disabling it removes it. The game container itself is never modified.

**Why this priority**: Capture must be opt-in — it is not the default. A server that has not opted in must be byte-identical to today. This story covers the lifecycle mechanics.

**Independent Test**: Can be tested independently by creating a GameServer without capture, verifying the sidecar is absent, enabling capture via the API, verifying the sidecar is added and the game pod remains Running, disabling capture, and verifying the sidecar is removed.

**Acceptance Scenarios**:

1. **Given** a freshly created GameServer, **When** the server is inspected, **Then** capture is not present or is disabled.
2. **Given** a GameServer with capture disabled, **When** an admin enables capture, **Then** the capture sidecar is added to the pod without restarting the game container.
3. **Given** a GameServer with capture enabled, **When** the sidecar is restarted or the pod is rebuilt, **Then** the capture setting persists across the update.
4. **Given** a GameServer with capture enabled, **When** an admin disables capture, **Then** the sidecar is removed without affecting the running game container.

---

### User Story 3 - Enforce admin-only access and audit all capture operations (Priority: P2)

All capture operations (start, stop, download, delete) are restricted to users with the admin role. Every operation is recorded in the audit log with the user identity, operation type, timestamp, and result.

**Why this priority**: Captures contain real player traffic (addresses, chat, and for some games in-band credentials). Access control and audit trails are security requirements, not optional.

**Independent Test**: Can be tested independently by attempting to start a capture as a non-admin user and verifying the request is rejected; attempting the same as an admin and verifying it succeeds; then reading the audit log and confirming both attempts are recorded.

**Acceptance Scenarios**:

1. **Given** a user with the viewer or operator role, **When** they attempt to start a packet capture, **Then** the request is rejected with a 403 Forbidden response.
2. **Given** an admin user, **When** they start, stop, download, or delete a capture, **Then** the operation succeeds.
3. **Given** any capture operation, **When** the admin audit log is inspected, **Then** an entry records the user, action, target (server name / capture ID), timestamp, and result.

---

### User Story 4 - Captures auto-expire and become inaccessible after the retention window (Priority: P2)

Captures are not permanent. Each capture is assigned an auto-expiration time based on a configured retention window (e.g., 24 hours). Once a capture expires, it is deleted from storage and subsequent download attempts are rejected.

**Why this priority**: Captures contain sensitive player data. Time-limiting their retention reduces the window of exposure and clarifies to operators that captures are ephemeral, not an archive.

**Independent Test**: Can be tested independently by creating a capture, verifying it is downloadable immediately, advancing the system clock past the retention window (or using a short window in tests), and verifying the capture is no longer downloadable and returns a 404 or "expired" response.

**Acceptance Scenarios**:

1. **Given** a completed capture, **When** the admin attempts to download it within the retention window, **Then** the download succeeds and the file is returned.
2. **Given** a capture that has expired, **When** the admin attempts to download it, **Then** the request is rejected with a 404 or "capture expired" response.
3. **Given** an expired capture, **When** the admin queries the capture list, **Then** the expired capture is no longer listed.
4. **Given** a running capture, **When** the retention window expires while the capture is still running, **Then** the capture is terminated and cleaned up.

---

### User Story 5 - Handle edge cases: server restart mid-capture, multiple captures, size/duration limits (Priority: P3)

The system must handle scenarios where the capture process is interrupted or constrained. These edge cases should fail gracefully without corrupting the server or leaving orphaned resources.

**Why this priority**: Robustness. These scenarios are less common than the happy path but must not crash the server or leave the operator in an unrecoverable state.

**Independent Test**: Can be tested independently by triggering each edge case scenario (restart, concurrent requests, limit scenarios) and verifying the system behaves as expected (capture is halted, error is reported, server remains playable).

**Acceptance Scenarios**:

1. **Given** a running capture, **When** the GameServer pod is restarted, **Then** the capture is terminated, the partial capture file is cleaned up (or marked as incomplete if preserved for debugging), and the server returns to a playable state.
2. **Given** a server with an active capture, **When** an admin requests a second capture on the same server, **Then** the request is rejected (or queued, depending on design) with a clear error message.
3. **Given** a capture approaching its configured max duration, **When** the duration limit is reached, **Then** the capture stops automatically and the collected packets are saved.
4. **Given** a capture approaching its configured max size, **When** the size limit is reached, **Then** the capture stops automatically without data loss or corruption.
5. **Given** a capture on a pod that is evicted or deleted, **When** the pod terminates, **Then** the capture is cleaned up and no orphaned resources remain on the node.

---

### Edge Cases

- What happens when a capture hits its maximum size limit before its maximum duration? The capture stops automatically, the collected packets are saved, and the file is marked as complete. The operator is notified that the size limit was the stopping condition.
- What happens when a capture is running but the pod's disk fills up? The capture fails with a clear error message (disk full), the partial file is cleaned up, and the server remains playable.
- What happens on a server that never opted into capture? Capture operations are not available for that server; requests return a 404 or "capture not enabled" error.
- What happens if two admins request captures on the same server simultaneously? The system must serialize or reject the concurrent request. Whichever approach is chosen, both users must be notified: one succeeds, the other gets a clear error (e.g., "capture already in progress").
- What happens when a capture filter expression is malformed or invalid? The capture request is rejected before starting, with a clear error message describing the syntax error.
- How does the capture behave on a heavily loaded server with high packet volume? The capture filter is the primary mechanism for keeping the volume manageable. Unfiltered captures on busy servers will hit size limits quickly, which is expected behavior (the operator must refine their filter). The sidecar must not cause the game server to lag or degrade even under high capture load.
- What happens to audit events if the audit system is temporarily unavailable? Capture operations should not fail if audit logging is down, but a warning or notification should be raised so the operator knows that the audit trail is incomplete.
- What happens if the capture sidecar crashes while a capture is running? The sidecar is restarted by Kubernetes (like any pod restart). The partial capture file is cleaned up. The operator is notified (via status or a failed audit event). The game server is unaffected.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The capture system MUST be opt-in per GameServer. A GameServer that has not opted in MUST NOT have any capture component attached and MUST be byte-identical to servers as they exist today. Enabling or disabling capture for a GameServer MUST cause the capture sidecar to be added or removed from the pod without modifying the game container itself.
- **FR-002**: An admin user initiating a capture MUST provide a maximum duration (in seconds or minutes) and a maximum size (in MB or GB). Both limits are hard: the capture stops when either limit is reached, whichever comes first.
- **FR-003**: Capture filter expressions MUST be a first-class input to the capture start request. The filter is OPTIONAL; when omitted, a default filter is applied that restricts the capture to the game server's own advertised ports. Custom filters MUST restrict captured traffic by at least packet criteria (source/destination IP, source/destination port, protocol). Invalid filter expressions MUST be rejected before the capture starts.
- **FR-004**: A completed capture MUST be downloadable as a file in a standard packet format (pcap or pcapng) readable by standard third-party packet-analysis tooling. The file MUST contain only the packets matching the filter expression provided at capture start.
- **FR-005**: Capture operations (start, stop, download, delete) MUST be restricted to users with the admin role (per `docs/security.md` role definitions). Non-admin users MUST receive a 403 Forbidden response on any capture operation.
- **FR-006**: Every capture operation MUST generate an audit event recording: user identity, operation type (start/stop/download/delete), target GameServer name, capture ID, timestamp, and result (success or failure reason). Audit events MUST be written before the operation response is returned to the user.
- **FR-007**: Captures MUST be automatically deleted after a configured retention window (default: 24 hours). Per-server retention settings override the cluster default; a cluster-configured maximum retention window caps any per-server setting to prevent servers from extending retention beyond cluster policy. Completed captures outside the retention window MUST not be listed or downloadable. Expired captures MUST be cleaned from storage.
- **FR-008**: The capture sidecar MUST never modify the game container, its runtime, its filesystem, or its network stack. The game container MUST remain unaware of the sidecar's presence.
- **FR-009**: A GameServer MUST remain playable during an active capture. The capture MUST not cause perceptible lag, packet loss, or disconnections for game players.
- **FR-010**: The capture system MUST fail gracefully when the pod is restarted, evicted, or deleted. Orphaned capture processes and partial files MUST be cleaned up. The game server MUST return to a playable state.
- **FR-011**: If a capture filter expression is provided, packets not matching the filter MUST not be included in the captured file. Filtering MUST happen in the capture process, not as a post-processing step on the full capture.
- **FR-012**: Concurrent capture requests on the same GameServer MUST be serialized or rejected. If rejected, the second request MUST receive a clear error message stating that a capture is already in progress.

### Key Entities

- **NetworkCapture**: Represents a single capture session, uniquely identified and associated with a GameServer. A capture session records when it began, when it ended (if complete), the maximum duration and size limits that apply, and the filter expression configured at start time. The capture tracks its lifecycle (running, completed, failed, or expired), the volume of data collected, and its expiration deadline. The entity records the identity of the admin who initiated the capture.

- **CaptureConfiguration**: The per-server capture opt-in flag together with its retention setting (or cluster default, if not overridden). A GameServer either has capture capability enabled or does not; when enabled, captures are subject to a retention window after which they auto-expire.

- **CaptureFile**: The output artifact — a pcap/pcapng file containing the captured packets. A file is associated with a capture session and is downloadable only while the capture's retention window has not expired.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A capture initiated with a valid filter expression collects packets and produces a file that is valid and readable by standard third-party packet-analysis tooling without errors.
- **SC-002**: A capture initiated and stopped on a running GameServer results in zero perceptible player-experience impact: no lag spikes, no packet loss, no disconnections reported by game clients.
- **SC-003**: 100% of capture operations (start/stop/download/delete) are recorded in the audit log with complete information: operator identity, server name, capture ID, timestamp, and result.
- **SC-004**: Captures automatically expire and become inaccessible after the configured retention window; download attempts after expiration return a 404 or "expired" response.
- **SC-005**: A non-admin user attempting any capture operation receives a 403 Forbidden response; an admin performing the same operation succeeds.
- **SC-006**: Concurrent capture requests on the same server are serialized or rejected; the second request receives a clear error instead of an undefined result.
- **SC-007**: A GameServer with capture disabled or not opted in does not have a capture sidecar and is byte-identical to a server today; a `kubectl diff` between a non-capturing and capturing server shows only the sidecar addition.
- **SC-008**: Packet capture files match the filter expression: 100% of packets in the output file satisfy the filter criteria; 0% of packets that do not match the filter are included.

---

## Assumptions

- **Assumption 1**: Packet captures contain real player data including IP addresses, network timing, game commands, and for some games in-band credentials (authentication tokens, server passwords, or session identifiers embedded in the protocol). This data is sensitive and must not be exposed to unprivileged users. This is the rationale for admin-only access and time-limited retention.
- **Assumption 2**: Binary game protocols are undocumented and cannot be reliably redacted post-capture (because redacting without knowing the protocol structure risks corrupting the binary format and making the capture useless for reverse-engineering). Therefore, captures are not sanitised. Role-based access control and retention windows are the primary security mechanisms, not redaction.
- **Assumption 3**: The capture sidecar runs with elevated capabilities (e.g., CAP_NET_RAW for packet capture) only in its own container. The game container retains its current security posture and never gains elevated capabilities or raw-packet access.
- **Assumption 4**: The maximum duration and maximum size limits are hard limits. A capture always stops when either limit is reached, even if a player is mid-join. This is necessary to prevent captures from consuming unbounded disk/memory or running forever.
- **Assumption 5**: Capture configuration is stored in the GameServer spec as an opt-in boolean (`spec.capture.enabled`) and cluster-wide defaults (retention window, size limits, duration limits) are configurable via the Helm chart or admin settings.
- **Assumption 6**: The capture mechanism operates at the network layer and does not require privilege escalation within the game container. The sidecar's capabilities are its own.

---

## Out of Scope

- **Automatic capture on join failure or any event**: This feature provides only manual, operator-initiated captures. Automatic capture (e.g., "capture every failed join attempt") is future work and explicitly out of scope for v1. This is a design choice to keep manual captures predictable and audit-friendly.
- **Real-time streaming or live packet inspection**: Captures are downloaded after completion. Real-time packet streaming to an external tool is not in scope.
- **Cross-cluster or multi-server capture coordination**: Each capture is tied to one GameServer in one cluster. Recording traffic from multiple servers or replaying/forwarding captures across clusters is out of scope.
- **Automatic redaction or sanitization of sensitive fields**: As noted in Assumptions, binary protocols cannot be reliably redacted. The feature does not attempt to sanitise captures.
- **Operator or system-level traffic capture**: Only game-pod network traffic is captured. Control-plane traffic, operator communication, or inter-pod mesh traffic is not in scope.
- **Packet replay or injection from a captured file**: This feature is read-only (capture and download). Replaying packets back into the server is out of scope.

---

## Out of Scope (Architectural Constraints Already Decided)

The following design decisions were made before this spec and are not open for re-discussion in this feature:

1. **Mechanism: OPT-IN SIDECAR ONLY**. Capture is delivered as an opt-in sidecar container added to the game pod. The game container itself is never modified and never gains raw-packet capabilities. A server that has not opted in is byte-identical to today's servers. This architecture ensures that capture capability is never a security regression for users who do not need it.

2. **Trigger + OUTPUT: MANUAL CAPTURE ONLY**. Captures are initiated manually by an admin via the API and produce a downloadable file. No always-on collection, no automatic capture on failure events, no background telemetry. The operator is in full control of when captures start and stop.

3. **PRIVACY POSTURE: ADMIN-ONLY, AUTO-DELETED, AUDITED**. Starting and downloading a capture is restricted to the admin role. Captures are automatically deleted after a short retention window (default 24 hours, configurable). Every start, stop, download, and delete is audited with full user identity, timestamp, and operation details.

4. **FILTERING IS FIRST-CLASS**. A capture filter expression is optional; when omitted, a sensible default is applied that restricts the capture to the game server's own advertised ports. On a busy server, an unfiltered capture hits the size cap in seconds and yields nothing but noise. Requiring every operator to hand-write a filter expression would be hostile; a default that is both safe and useful prevents that friction. The filter is what makes a capture actionable. Filtering is not a post-processing step; it is enforced at packet-capture time by the sidecar.

---

## Rationale for Design Constraints

The four architectural decisions above exist because:

- **Sidecar (not game modification)**: The game is a third-party binary. Adding capabilities to it, even elevated read-only ones, is a security and support risk. Containing the capability in a sidecar ensures the game environment is unchanged.
- **Manual trigger (not automatic)**: Automatic capture on "failure" is hard to define reliably (what is failure?) and generates false positives. Manual triggers are predictable, auditable, and under operator control.
- **Admin-only and time-limited (not role-graduated or permanent)**: Captures contain real player data. If accessed by lower-privileged roles, that data might leak; if kept permanently, the exposure window is unbounded. Admin-only + auto-expiration is the simplest model that closes both gaps.
- **Filtering first-class (not optional)**: Without it, captures are useless on busy servers. With it, they are surgical. Including filtering in the design from day one is more effective than retrofitting it later.

