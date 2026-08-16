# Specification Quality Checklist: Nuclear Option Module & Load-Balancer IP Pool Override

**Purpose**: Verify that the feature specification is complete, unambiguous, and ready for `/speckit-plan`
**Created**: 2026-08-16
**Feature**: [../spec.md](../spec.md)

## Content Quality

- [x] No implementation details in requirement text (no Kubernetes API field names, no file paths, no framework/library names)
- [x] Focused on user value and operator outcomes, not technical plumbing
- [x] Written for business/product stakeholders and operators, not engineers
- [x] All mandatory sections completed (User Scenarios, Requirements, Success Criteria, Assumptions)
- [x] User stories describe real operator journeys, not implementation tasks
- [x] Success criteria are measurable and observable without knowing internal architecture

## Requirement Completeness

- [x] No `[NEEDS CLARIFICATION]` markers remain; all ambiguities resolved via Assumptions section
- [x] Every functional requirement is a testable MUST statement
- [x] Every user story has clear acceptance scenarios (Given/When/Then format)
- [x] All edge cases identified and documented
- [x] Success criteria are technology-agnostic (e.g., "within 30 seconds" not "gRPC round-trip latency")
- [x] Success criteria are measurable (e.g., quantified time, percentage, or binary outcome)
- [x] Feature scope is clearly bounded (Out of Scope section lists 7 excluded items)
- [x] Dependencies and assumptions identified (9 assumptions; per-module `specs.md` obligation is FR-027)

## Feature Readiness

- [x] All 27 functional requirements have clear acceptance criteria
- [x] User scenarios (6 stories) cover both primary flows (Nuclear Option deployment, pool assignment) and error cases (misconfiguration, pool exhaustion)
- [x] Feature meets all 8 measurable Success Criteria
- [x] No test-implementation details leaked into spec (e.g., spec says "real join test" not "use the gameproto/nucleaboption join client")
- [x] Cross-cutting concerns addressed (dashboard parity, E2E coverage, documentation)
- [x] Prioritization clear: P1 (deployable, joinable, pool selection), P2 (moderation, fixed address), P3 (config validation, error clarity)

## Notes

**Fixes applied during review** (this checklist and the spec were revised together — see spec.md history):

1. **Implementation leakage removed**: the original draft named Kubernetes-specific mechanisms directly in requirement text and Key Entities — "CPU/memory requests" (the CRD field name), the Backup/Restore CRDs by name, "load-balancer Service", and "annotation/Service spec" as the pool-selection mechanism. These are now phrased at the resource/outcome level (e.g., "CPU allocation", "existing backup and restore capabilities", "public network endpoint") with the actual mechanism left as an implementation-phase decision.
2. **Compound requirement split**: the original FR-007 bundled kick, ban-add, and ban-remove into a single MUST connected by "or". This is now three independent, individually testable requirements (FR-008, FR-009, FR-010).
3. **Missing requirements added**: mission-rotation control (`set-next-mission`, `set-time-remaining`, tested by User Story 3's Acceptance Scenario 5) had no corresponding FR — added as FR-012. A requirement that pool-preference logic behaves identically whether a GameServer reaches the cluster via the dashboard, the REST API, *or* a direct `kubectl apply` was missing (the original FR-017 covered dashboard-vs-REST-API parity only, not direct-to-cluster) — broadened into FR-021, consistent with the project's "operator is authoritative" principle. An explicit requirement that the join test must be proven to fail against a dead address and succeed against a real listener (constitution Principle I) was implied but not stated — added as FR-005.
4. **Unverified upstream facts reframed as assumptions, not given facts**: the original spec stated the dedicated-server Steam app ID (3930080), the exact ports (UDP 7777/7778, TCP 7779), and the remote-command protocol's wire format/command names/result codes as settled facts inside FR text and Key Entities. All of these come from third-party hosting-provider documentation, never the publisher. They are now presented as explicit, flagged assumptions (see spec.md's Assumptions section) that implementation MUST verify against a real running server, with the E2E join test named as the mechanism that catches drift if any assumption is wrong. Whether any signal distinguishes "booted" from "actually accepting players" is now called out as unverified (Edge Cases + Assumptions) rather than assumed to exist.

**Residual risks (confirmed against a live server during implementation, not before):**

1. **Unverified upstream protocol/port/app-ID facts**: the dedicated-server app ID, the game/query/remote-command ports, and the remote-command JSON protocol's framing, command names, and result codes all trace back to third-party hosting-provider documentation rather than the publisher. Any of these could be wrong. Mitigation: the real-protocol E2E join test (FR-004, FR-005) is required to prove itself against a live server before being trusted, which is the mechanism that will surface a wrong assumption immediately rather than silently.
2. **Unknown readiness signal**: it is not established whether Nuclear Option exposes any way to tell "process started" apart from "accepting player connections." If no such signal exists, the "Accepting Players" status (User Story 1) will need to be inferred from the join/query probe itself rather than a simpler readiness check — a design constraint the planning phase must account for.
3. **Resource footprint vs. CI runners**: the assumed ~8–16 GB RAM / 30 GB storage footprint is itself an estimate pending confirmation; if the real server needs more, the module still ships (FR-026 requires the join test regardless) but moves from the CI-executed set to the heavy/deferred set, per the constitution's documented-exclusion escape hatch.
4. **Address-manager vendor variance**: the pool-override feature is scoped to work with "any CNCF-standard load-balancer address manager," but the actual mechanism each manager (MetalLB, Cilium, others) exposes for pool/address hints may differ enough that a single implementation doesn't cover all of them equally well. This is left as a planning-phase risk, not resolved here.

**Specification status**: spec.md now accurately separates settled requirements from unverified upstream facts. It is ready for `/speckit-plan`, with the explicit expectation that planning treats the protocol/port/app-ID details as things to confirm against a live server, not as given.
