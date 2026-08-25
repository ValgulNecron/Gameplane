# Feature Specification: Game Protocol E2E Coverage

**Feature Branch**: `001-gameprotocol-e2e-coverage`

**Created**: 2026-08-11

**Status**: Draft

**Amendments**: FR-001, SC-001, and Assumption 3 amended on 2026-08-16 to separate the unconditional status-recording obligation from the protocol-dependent join-test obligation; FR-010 added to require `blocked-doc` items to name their unblocking artifact.

**Input**: User description: "Full E2E testing for gameprotocol, and finish the project for a v1 release"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Every shipped game module proves a real join (Priority: P1)

A maintainer preparing a release needs confidence that a player can actually connect to
each game type Gameplane offers, not just that a GameServer's pod is Running and a port
is open. For every module under `modules/`, the E2E suite performs a real join using
that game's own wire protocol against a booted GameServer, and reports pass/fail per
game.

**Why this priority**: This is the core gap the constitution's E2E principle exists to
close. Without a protocol-level join, "the server is up" and "the server is playable"
are indistinguishable, and Gameplane has already shipped username/version-drift bugs
that a port-open check would never have caught.

**Independent Test**: Can be tested independently per game — run that game's E2E join
test against a freshly booted GameServer of that module and confirm it reports success;
run it again against a GameServer that is deliberately not accepting connections and
confirm it reports failure (never a false positive).

**Acceptance Scenarios**:

1. **Given** a booted GameServer for a module with an implemented join client, **When**
   the module's E2E test runs, **Then** it completes a real protocol handshake and
   reports the join as successful.
2. **Given** an address with nothing listening, **When** the same join test runs against
   it, **Then** it reports failure rather than a false success.
3. **Given** a module's join test passes today, **When** the module's server or client
   version drifts in a way that breaks the real protocol, **Then** the test fails
   instead of continuing to report success on a stale assumption.

---

### User Story 2 - Heavy games get a written, on-demand test instead of no test (Priority: P2)

A maintainer wants coverage for game modules whose server resource footprint (multi-GB
images, sustained CPU/RAM) makes them impractical to boot inside a GitHub Actions
runner, without those modules being silently excluded from having a test at all.

**Why this priority**: This is the constitution's explicit tradeoff (deferred
execution, never deferred authorship). Skipping test authorship entirely for heavy
games would leave real join defects undiscoverable until a user hits them.

**Independent Test**: Can be tested independently by confirming the test file for a
heavy module exists, compiles, and runs to yield a definitive verdict when executed
against a real cluster (remote operator-provided host or a manually triggered CI job):
either a successful join or a recorded protocol-level failure attributable to that
module's known blocker, while being provably excluded from every default CI bucket.

**Acceptance Scenarios**:

1. **Given** a game module too heavy to run in the default CI runner, **When** the
   E2E suite is inventoried, **Then** a join test for that module exists and is
   committed, but is absent from every bucket `test/e2e/buckets.sh` runs by default.
2. **Given** the same heavy-module test, **When** a maintainer runs it manually against
   a real cluster, **Then** it exercises the same real protocol join as an in-CI test
   and reports pass/fail.
3. **Given** the bucket exclusion for a heavy module, **When** a maintainer reads
   `test/e2e/buckets.sh`, **Then** the exclusion carries a comment explaining why the
   module is excluded.

---

### User Story 3 - Blocked and undocumented protocols are tracked, not silently missing (Priority: P3)

A maintainer needs a single place that shows, for every game module, whether its join
protocol is covered, deferred-from-CI, or blocked — and if blocked, why (missing
packet capture, anti-cheat gate, platform-relay routing, encrypted transport) — so
"why isn't this game tested" is answered by reading a doc instead of asking around.

**Why this priority**: Lower priority than actually closing coverage gaps, but
necessary so gaps that can't be closed before v1 are a visible, deliberate decision
rather than an invisible one.

**Independent Test**: Can be tested independently by cross-checking the tracked list
against the actual module set under `modules/` and the actual bucket contents of
`test/e2e/buckets.sh`, and confirming every module appears exactly once with a status.

**Acceptance Scenarios**:

1. **Given** the full set of shipped game modules, **When** the tracked coverage list
   is reviewed, **Then** every module has exactly one status: covered-in-ci,
   covered-deferred, blocked-doc, or out-of-scope-by-design.
2. **Given** a module blocked on missing protocol documentation, **When** the blocker
   is architectural (anti-cheat, platform-only transport) rather than a documentation
   gap, **Then** it is marked out-of-scope-by-design instead of perpetually "pending",
   so it does not silently linger as an open item forever.

---

### Edge Cases

- What happens when a game's server or client version drifts upstream and the
  previously-working join handshake breaks? The join test must fail with a non-zero
  exit code and emit a VERDICT line naming the failure reason (per contracts/probe-cli.md)
  rather than continue reporting success against a now-invalid assumption.
- How does coverage tracking handle a game whose join fundamentally requires a
  platform credential (e.g. Steam authentication) that an automated test environment
  cannot supply? It is recorded as out-of-scope-by-design with blocker class
  architectural, not left unaccounted for.
- What happens to a heavy-module test that is excluded from CI and never run for a
  long stretch — how is bit-rot detected before it's needed? Coverage tracking MUST
  record when each deferred-from-CI test was last actually run, so a stale test is
  visible rather than assumed current.
- What happens when a module is removed or renamed? Its entry in the tracked coverage
  list MUST be removed or updated in the same change, so the list never references a
  module that no longer exists.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: For every game module shipped under `modules/`, the E2E suite MUST
  record a verified join-coverage status in the single tracked artifact (covered-in-ci,
  covered-deferred, blocked-doc, or out-of-scope-by-design), so no module sits in an
  unknown or unrecorded state. Additionally, for every game module whose join protocol
  is documented well enough to build a client, the E2E suite MUST contain a test that
  performs a real, protocol-correct join against a booted GameServer of that module —
  a bare TCP/UDP dial or a mocked handshake does not satisfy this requirement. (see
  FR-006 for the prohibitive corollary; keep the two in sync)
- **FR-002**: Every protocol-join E2E test MUST be verified to fail against a
  non-listening address and to succeed against a real listener before it is trusted as
  a correctness signal for that module.
- **FR-003**: For any game module whose server resource requirements make it
  impractical to run inside the default CI runner, its join test MUST still be
  authored and committed to `test/e2e/`; it MAY be excluded only from CI bucket
  execution, and that exclusion MUST be commented with the reason in
  `test/e2e/buckets.sh`.
- **FR-004**: A join test excluded from CI execution MUST remain runnable on demand —
  locally, against an operator-provided remote host/cluster, or via a manually
  triggered job — without code changes.
- **FR-005**: For any game module whose join protocol lacks usable public
  documentation, the specific blocker MUST be recorded in one tracked location
  (missing packet capture, incomplete reverse-engineering, anti-cheat gate, encrypted
  transport, platform-relay-only routing, or similar).
- **FR-006**: A game module MUST NOT be counted as join-tested based on a query/status
  probe alone (e.g. Steam A2S or similar out-of-band query protocols) — only a
  completed protocol-level join satisfies join coverage. (the prohibitive statement of
  FR-001; keep the two in sync)
- **FR-007**: The join-coverage status of every shipped game module (covered-in-ci,
  covered-deferred, blocked-doc, or out-of-scope-by-design) MUST be readable from a
  single tracked artifact, kept current as modules are added, removed, or change status.
- **FR-008**: A module blocked by an architectural constraint that prevents a headless
  automated client from ever completing a join (e.g. anti-cheat validation requiring a
  real game binary/GPU, or a platform-only relay transport) MUST be marked
  out-of-scope-by-design with the specific constraint recorded, rather than left in an
  indefinitely "pending" state.
- **FR-009**: A join test MUST fail when the real protocol handshake it depends on
  breaks (e.g. due to server/client version drift), rather than continuing to report
  success based on a cached or assumed-valid interaction.
- **FR-010**: A module in `blocked-doc` status MUST name the specific next artifact
  required to unblock it (for example: a packet capture of a real client's join, a
  reverse-engineered field map, or vendor documentation), so the blocker is an
  actionable work item rather than a standing excuse. `blocked-doc` is by definition
  temporary; a blocker proven permanent MUST be reclassified `out-of-scope-by-design`
  per FR-008 rather than left naming an artifact that will never arrive.

### Key Entities

- **Game Module**: A directory under `modules/` (e.g. `minecraft-java`, `terraria`,
  `factorio`) representing one supported game; has a join-coverage status.
- **Protocol Join Test**: An E2E test that performs a real client-side handshake in
  that game's own wire protocol against a booted GameServer, distinct from a query
  probe or a bare socket dial.
- **CI Bucket**: A named grouping in `test/e2e/buckets.sh` that GitHub Actions executes;
  a join test's presence or deliberate absence here determines covered-in-ci vs.
  covered-deferred.
- **Coverage/Blocker Record**: The tracked entry, per module, recording its status and
  (if blocked) the specific reason and whether that reason is a temporary
  documentation gap or a permanent architectural constraint.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of shipped game modules have a recorded, verified join-coverage
  status in the tracked artifact, AND 100% of modules whose join protocol is documented
  have a committed real protocol-join E2E test, whether or not that test executes in CI.
- **SC-002**: 100% of game modules excluded from CI E2E execution have a documented,
  reviewable reason recorded alongside the exclusion.
- **SC-003**: Zero game modules remain in an undocumented or unknown join-coverage
  status at v1 GA — every module is covered-in-ci, covered-deferred, blocked-doc,
  or out-of-scope-by-design with a recorded reason.
- **SC-004**: A maintainer can determine full join-coverage status for every game
  module by reading a single artifact, in under 5 minutes, without searching git
  history, chat logs, or prior session memory.
- **SC-005**: A join test that starts failing due to an upstream protocol/version
  change is caught in the same CI run for tests that execute in CI. For deferred tests,
  drift is caught on the next on-demand run, and a deferred test that has not been run
  within the staleness threshold is surfaced as a warning by the coverage verifier
  (referencing the Last Verified field) so stale tests remain visible rather than silent.

## Assumptions

- This spec scopes "finish the project for a v1 release" down to the game-protocol
  E2E coverage slice of v1 readiness, per the constitution's E2E-Tested Delivery
  principle. Broader v1-readiness items outside game-protocol E2E testing are tracked
  separately in `docs/roadmap.md` and are out of scope here.
- "Game module" means a directory under `modules/` as currently structured (the
  `gameplane-module` submodule); modules added or removed after this spec is written
  are covered by FR-007's requirement to keep the tracked artifact current, not by
  enumerating them here.
- This effort is expected to end with architecturally-blocked games marked
  out-of-scope-by-design, documentation-blocked games marked `blocked-doc` each naming
  its unblocking artifact per FR-010, and all other games tested or deferred per their
  coverage status. Full coverage of every module means every module has a verified
  recorded status, not that every module ends up executing a live join in CI.
- The single tracked coverage artifact (FR-007) is a documentation deliverable; its
  exact location and format are a planning-phase decision, not specified here.
- CI resource constraints (which modules count as "too heavy for the default runner")
  are bounded by the GitHub Actions runner's ~14 GB disk. The operative convention is
  the existing fast/heavy split encoded in `test/e2e/buckets.sh` (bucket `bot-fast` runs
  in default CI; bucket `bot-heavy` never does), not a new threshold introduced by this
  spec.
