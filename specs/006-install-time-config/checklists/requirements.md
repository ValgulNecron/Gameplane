# Requirements Checklist: Install-Time Configuration (Storage Class & OIDC Role Mapping)

**Purpose**: Verification and sign-off on requirement quality and completeness for spec 006.

**Created**: 2026-08-22

**Feature**: [Install-Time Configuration Spec](/specs/006-install-time-config/spec.md)

**Note**: This checklist tracks the completeness and quality of the feature specification against the project's constitution and standard requirements-writing practices.

---

## Specification Quality & Completeness

- [x] **REQ001** — All user stories are labeled with priorities (P1, P2, P3); priorities are justified and reflect relative importance of the feature tracks.
  - **Finding**: Two independent feature tracks: OIDC at install time (P1 — silent security trap, blocks OIDC-only deployment) and storage class configuration (P2 — operational enhancement, known workaround exists). User Story 3 (OIDC post-install edit) was removed as it re-specified existing dashboard functionality that already ships.
  - **Status**: PASS

- [x] **REQ002** — Each user story has "Independent Test" section describing testability without dependency on other stories.
  - **Finding**: Both remaining stories are independently testable. Story 1 (OIDC at install time, P1) can be tested by installing with pre-configured mappings and logging in as a mapped admin without running bootstrap-admin. Story 2 (Storage class configuration, P2) can be tested by installing with a custom class and verifying PVC requests. Story 3 (OIDC post-install edit) was removed as it describes existing shipped functionality.
  - **Status**: PASS

- [x] **REQ003** — Acceptance scenarios use Given/When/Then format and cover happy path, error paths, and edge cases.
  - **Finding**: 4 scenarios for Story 1 (OIDC at install time): normal OIDC-mapped admin login, unmapped user gets default role, group membership change triggers re-eval, no mappings configured shows admin indicator, bootstrap-admin coexistence. 4 scenarios for Story 2 (storage class): class specified, existing PVCs immutable, no class = use cluster default, nonexistent class error. Edge case section now covers 9 scenarios including new over-broad mapping risk edge case (nonexistent class, removed class, claim structure drift, role upgrade, admin lockout, bootstrap-admin coexistence, API token edge, lazy re-evaluation, over-broad mapping).
  - **Status**: PASS

- [x] **REQ004** — Functional requirements are unambiguous, testable, and implementation-agnostic (no Go symbols, Helm key names, or file paths in the requirement text itself).
  - **Finding**: 17 functional requirements: storage class (FR-001 to FR-006), OIDC configuration (FR-007 to FR-015), cross-cutting (FR-016 to FR-017). Story 3's requirements (dashboard editing, immediate effect, admin removal warning) were removed as they describe existing functionality already shipped in the codebase. All FRs use operator-facing language and avoid implementation detail (e.g., "administrative configuration interface" instead of "/admin/config"; "Helm chart values" instead of specific key names). Literal routes and file paths removed.
  - **Status**: PASS

- [x] **REQ005** — Success criteria (SC-###) are measurable, technology-agnostic, and operator-visible.
  - **Finding**: 8 success criteria. Examples: "An operator can install Gameplane with a game-data storage class specified in Helm values and have all GameServers created afterward use that storage class for their PVCs (100% of new PVCs target the specified class)" (SC-001 — measurable: 100% compliance). "A GameServer with a nonexistent storage class shows a clear, specific error message in the dashboard status within 30 seconds" (SC-002 — measurable: 30-second SLA, visible in dashboard). "An operator can install Gameplane with OIDC enabled and role mappings pre-configured (via Helm values, no bootstrap-admin run), and a user whose OIDC group matches an admin mapping logs in and immediately receives admin role and dashboard access" (SC-003 — measurable: user gets admin role, can access admin features). All criteria are operator-observable.
  - **Status**: PASS

- [x] **REQ006** — Key Entities section describes domain concepts without implementation details.
  - **Finding**: 8 entities (Storage Class, Game-Data PVC, OIDC Provider, OIDC Token, Group/Claim Name, Role Mapping, Default Role, Helm-Owned Provider) described in plain English. Removed "API Configuration" entity and replaced with "Helm-Owned Provider" to clarify that Helm-configured auth is NOT stored in the API database. No Go struct names, database column names, or Kubernetes API field paths.
  - **Status**: PASS

- [x] **REQ007** — Assumptions are explicit and justified; [NEEDS CLARIFICATION] markers appear only where a genuine decision is required, not where a reasonable default exists.
  - **Finding**: 10 explicit assumptions covering storage class existence, OIDC provider claim exposure, Helm-owned provider immutability (verified via HelmProviderName convention), bootstrap-admin availability, role re-evaluation timing, independence of storage class configs, quota policy, PVC immutability, and over-broad mapping risk. **No [NEEDS CLARIFICATION] markers remain.** The persistence model is now established: Helm-configured auth follows the HelmProviderName convention and is NOT stored in the API database.
  - **Status**: PASS

- [x] **REQ008** — "Verification Required Before Implementation" section flags unverified preconditions and names fallback actions.
  - **Finding**: 3 verification claims: (1) Helm chart value structure (storage class key location) — UNVERIFIED; (2) Helm-configured provider flow and immutability — VERIFIED via HelmProviderName convention, including concern that reserved name must be protected; (3) OIDC group claim name configurability — UNVERIFIED. Claim 2 was updated to reflect verified convention and implementation concern.
  - **Status**: PASS

- [x] **REQ009** — Out of Scope section lists capabilities explicitly excluded (no ambiguity about what is and isn't in v1).
  - **Finding**: 8 out-of-scope items: custom storage provisioners, quota limits, dynamic migration, group nesting, role-based class selection, IPv6/dual-stack, multi-cluster sync, advanced OIDC features. Clear and justified.
  - **Status**: PASS

- [x] **REQ010** — Spec follows the house style of spec 002 (two independent tracks, clear priority justification, Given/When/Then, Key Entities, Key Assumptions, Verification Required Before Implementation, Out of Scope).
  - **Finding**: Structure mirrors spec 002's format: User Scenarios (Stories 1–3 with priorities justified), Edge Cases, Requirements (FR-###), Key Entities, Success Criteria (SC-###), Assumptions, Verification Required Before Implementation, Out of Scope. Consistent voice (operator-centric, no implementation prescriptions).
  - **Status**: PASS

---

## Requirement Completeness & Realism

- [x] **REQ011** — Functional requirements cover both feature tracks (storage class configuration and OIDC role mapping) with no gaps between user stories and requirements.
  - **Finding**: User Story 1 (OIDC at install time, P1) maps to FR-007 through FR-015 (mappings, re-evaluation, audit events, over-broad mapping warning). Story 2 (Storage class configuration, P2) maps to FR-001 through FR-006 (configuration, precedence, error messages, visibility). Cross-cutting FRs FR-016 and FR-017 cover documentation and admin configuration interface visibility. Story 3 (OIDC post-install editing) was removed as it described existing shipped functionality. All remaining stories and edge cases have corresponding FRs with no gaps or orphaned requirements.
  - **Status**: PASS

- [x] **REQ012** — Success criteria are achievable and measurable against the functional requirements.
  - **Finding**: SC-001 ("100% of new PVCs target the specified class") tests FR-002 and FR-003 (storage class application). SC-002 tests FR-005 (error messages on nonexistent class). SC-003 and SC-004 test FR-007 through FR-010 (OIDC mapping at install time, first login, role assignment). SC-005 tests FR-011 (role re-evaluation on next login). SC-006 tests FR-006 and FR-017 (visibility in admin interface). SC-007 tests FR-012 (no mappings configured, admin indicator). SC-008 tests backward compatibility (FR-001, FR-007). All SCs are independently verifiable and measurable. Note: Former SC-007 references Story 3 editing, which is removed; dashboard editing is existing functionality not re-specified here.
  - **Status**: PASS

- [x] **REQ013** — Requirements are feasible given the architecture described in CLAUDE.md.
  - **Finding**: Spec assumes operator reconciles GameServers (FR-004 precedence of explicit storage class over default), consistent with "operator is authoritative" principle. Storage class specification is a Helm value → operator template → GameServer → reconciler flow, matching existing patterns (e.g., api.storage.storageClassName for API SQLite). Helm-configured OIDC provider follows the established HelmProviderName convention (api/internal/auth/registry.go): sourced from Helm/CLI flags at startup, synthesized as a read-only provider, never persisted to the API database. OIDC role mappings and audit events are consistent with api/internal/db/migrations/ and api/internal/audit/ patterns. No architectural impossibility detected.
  - **Status**: PASS

- [x] **REQ014** — Assumptions do not contradict the constitution (`.specify/memory/constitution.md`).
  - **Finding**: Spec adheres to Principle I (E2E-tested delivery): requirements are observable end-to-end (operator installs, user logs in, role is correct). Principle IV (Spec-Driven Development): this spec drives implementation, not the reverse. No conflict with Principles II (Design-First), III (Language & Ecosystem), V (Delegate), or VI (CI Bears Heavy Lifting).
  - **Status**: PASS

---

## Edge Cases & Risk Coverage

- [x] **REQ015** — Edge cases cover error paths, boundary conditions, and the interaction between this feature and existing systems (bootstrap-admin, OIDC providers, Kubernetes PVC immutability).
  - **Finding**: 9 edge cases identified. Error paths: nonexistent storage class (Scenario 1), removed storage class (Scenario 2), OIDC claim not present in token (Scenario 3), misconfigured mappings leave no admin (Scenario 5), over-broad mapping grants unintended admin (added per security requirement FR-015). Boundary conditions: no mappings configured (Story 1 Scenario 4), explicit storage class override (Story 2), role upgrade without downgrade (Scenario 4). System interactions: bootstrap-admin coexistence (Story 1 Scenario 5, Scenario 6), PVC immutability on config change (Story 2 Scenario 2), API token edge case (Scenario 7). New FR-014 (audit events for role changes) and FR-015 (over-broad mapping warning) address security gaps.
  - **Status**: PASS

- [x] **REQ016** — [NEEDS CLARIFICATION] markers are minimal (≤3) and each one justifies why it genuinely needs human decision.
  - **Finding**: Zero [NEEDS CLARIFICATION] markers remain. The persistence model question was resolved by referencing the established HelmProviderName convention (api/internal/auth/registry.go): Helm-configured auth is synthesized at runtime, never seeded into the API database, and is immutable through the dashboard. This removed the need for a clarification marker.
  - **Status**: PASS (0 markers, all clarifications resolved)

---

## Feature Readiness Assessment

- [x] **REQ017** — Specification is complete and ready for planning phase (`/speckit-plan`).
  - **Finding**: All mandatory sections are present and filled after revision. User stories (2 remaining) are prioritized and independently testable. Functional requirements (17 total) are numbered, unambiguous, and implementation-agnostic. Success criteria (8 total) are measurable. Assumptions (10) are explicit and justified. Verification claims are named with one verified and two unverified. No blocking clarifications exist. Spec has been corrected to: remove Story 3 (re-specified existing functionality), remove login-page pre-auth disclosure, establish Helm provider architecture per HelmProviderName convention, remove implementation leakage, add security requirements for audit events and over-broad mapping risk. Spec is suitable for handoff to planning phase.
  - **Status**: PASS

- [ ] **REQ018** — Spec has been reviewed by stakeholder (user/maintainer) for priority alignment and correctness.
  - **Finding**: Not applicable in this review context. This checklist is generated during initial specification; stakeholder review is a separate gate before plan phase.
  - **Status**: DEFERRED (not part of this checklist's scope)

---

## Summary

**Overall Status**: ✅ **PASS (After Revisions)**

**Requirement Completeness**: 17 of 17 items passed (1 deferred as out of scope for this checklist).

**Revisions Made**: Spec was revised to address 8 critical defects:
1. **Persistence model corrected**: Helm-configured auth follows HelmProviderName convention (never seeded to DB, read-only, runtime synthesis)
2. **Story 3 removed**: Re-specified existing shipped functionality (dashboard editing of role mappings, immediate effect, lockout warnings)
3. **Pre-auth disclosure eliminated**: Removed login page indicators; moved to admin-only configuration interface
4. **[NEEDS CLARIFICATION] removed**: Persistence model established by convention; zero clarification markers remain
5. **Implementation leakage removed**: Removed literal routes (/admin/config), file paths, and symbols from all FRs, SCs, and entities
6. **Storage class UI contradiction fixed**: Made FR-006 and FR-017 consistent; added storage class visibility requirement
7. **Security requirements added**: FR-014 (audit events for role changes), FR-015 (over-broad mapping warning), edge case for mapping risk
8. **Checklist corrected**: Marked FRs by count, updated story-to-requirement mapping, corrected verification claims

**Quality Assessment**: 
- User stories (2 remaining, Story 3 removed) are clear, prioritized, and independently testable.
- Functional requirements (17 total, down from 19) are unambiguous, implementation-agnostic, with no leakage.
- Success criteria (8 total) are measurable and operator-observable.
- Assumptions (10 total) are explicit and justified; zero [NEEDS CLARIFICATION] markers remain.
- Edge cases (9 total) are comprehensive and include security-relevant scenarios (over-broad mapping risk).
- Spec follows established house style; architecture verified against existing conventions.
- No contradictions with constitution (Principle I E2E-tested, Principle IV Spec-Driven, no pre-auth disclosure per rule 3).

**Ready for Planning Phase**: Yes. The specification is complete, unambiguous, and suitable for the `/speckit-plan` phase.

**Next Steps**: 
1. Stakeholder review and feedback on priorities (P1 OIDC at install time vs P2 storage class).
2. Move to planning phase (`/speckit-plan`) to detail how and by whom each requirement will be implemented.
3. During planning, resolve the Verification Required Before Implementation claims (Helm chart structure, group claim configurability).
