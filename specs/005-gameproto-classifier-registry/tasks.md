---
description: "Task breakdown for spec 005 — gameproto classifier registry refactor"
---

# Tasks: gameproto Classifier Registry Refactor

**Input**: Design documents from `specs/005-gameproto-classifier-registry/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Tests ARE included in this task breakdown. Constitution Principle I mandates E2E delivery; User Story 2's Independent Test requires byte-for-byte comparison tests to validate behavior preservation across the facade → Classifier refactor. User Story 1's Independent Test requires a demonstration that adding a new protocol (demo) needs only one implementation file and one registry entry, with zero edits to shared code.

**Organization**: Tasks are grouped by phase to enable staged implementation. A critical dependency exists between User Story 1 (sentinel dispatch refactor) and User Story 2 (Minecraft/Terraria behavior preservation): US1's registry-based dispatcher requires real Classifier implementations from US2 to test. These two stories represent two views of one implementation pass, ordered as separate stories for logical clarity but executed together as the MVP.

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: Can run in parallel (different files, no interdependencies)
- **[Story]**: Which user story this task belongs to (e.g., [US1], [US2], [US3], [US4])
- Include exact file paths in descriptions

## Path Conventions

- **Go modules**: `gameproto/`, `sentinel/` — paths relative to repo root
- **Tests**: Co-located as `*_test.go` files in the same directory as source
- **Specifications**: `gameproto/specs.md`, `sentinel/specs.md`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish file structure, interface contracts, and test scaffolding for the registry pattern without modifying existing implementations.

- [X] T001 Create `gameproto/classifier.go` with Classifier interface, Kind enum, ClassificationResult type definitions (interface only, no implementation bodies); defines Classify, SupportsStatusPing, BuildStatusResponse, BuildDisconnect methods; Kind enum: Join=0, Status=1, Unknown=2; ClassificationResult struct carrying Kind, Consumed []byte, Detail interface{}; Detail interface with ProtocolName() method; MinecraftDetail and TerrariaDetail structs defined but empty (FR-001, FR-002, FR-007)

- [X] T002 [P] Create `gameproto/registry.go` with Registry struct and Lookup/ListRegistered function signatures (interface only, no populated map literal yet); Lookup(name string) returns (Classifier, bool); ListRegistered() returns []string (FR-003, FR-004)

- [X] T003 [P] Create `gameproto/demo.go` as minimal stub DemoClassifier implementation to validate the registry pattern; DemoClassifier implements Classifier interface with stub/no-op logic (Classify returns Unknown, SupportsStatusPing returns false, BuildStatusResponse returns error, BuildDisconnect returns empty bytes) (FR-001, Decision 10)

- [X] T004 [P] Create `gameproto/gameproto_test.go` test scaffolding for registry validation tests with placeholder test bodies: TestRegistryCompleteness, TestRegistryNoDuplicates, TestOneClassifierPerProtocol (SC-006, Decision 5, User Story 2)

- [X] T005 [P] Update `gameproto/specs.md` documentation to describe the new Classifier abstraction, ClassificationResult type, Detail interface, registry contract, and startup validation flow (Principle IV requirement); retain existing behavior documentation (Principle IV, SC-006)

- [X] T006 [P] Update `sentinel/specs.md` documentation to describe the registry-based dispatch mechanism, startup protocol-name validation, Classifier lookup and invocation flow, and per-bucket E2E test dependencies (Principle IV requirement) (Principle IV, SC-001)

**Checkpoint**: File structure, interface definitions, and test scaffolding complete — no implementation bodies or shared-code edits yet.

---

## Phase 2: Foundational (Core Abstraction & Registry Infrastructure)

**Purpose**: Establish the core abstraction types, interfaces, and registry infrastructure that all other implementation work depends on. This phase creates skeleton/stub implementations; full Minecraft/Terraria logic implementation deferred to Phase 4 (User Story 2).

⚠️ **CRITICAL**: No user story work can begin until this phase is complete.

- [X] T007 Create `gameproto/classifier.go` defining core types and the Classifier interface with all four methods (Classify, SupportsStatusPing, BuildStatusResponse, BuildDisconnect) and their signatures; create Kind enum (Join, Status, Unknown) and ClassificationResult struct carrying Kind, Consumed []byte, Detail interface{}; create Detail interface with ProtocolName() method; create MinecraftDetail and TerrariaDetail struct placeholders (FR-001, FR-002, FR-007)

- [X] T008 [P] Create `gameproto/registry.go` with Registry struct, Global() function returning package-level registry instance, Lookup(name string) (Classifier, bool) function, and ListRegistered() []string function; DO NOT populate the compile-time map literal yet (FR-003, FR-004, SC-001, SC-006)

- [X] T009 [P] Create MinecraftClassifier struct in `gameproto/minecraft.go` implementing Classifier interface with all four methods; method bodies are stubs (Classify returns ClassificationResult{Kind: Unknown, Consumed: nil, Detail: nil}, BuildStatusResponse returns error, BuildDisconnect returns empty bytes, SupportsStatusPing returns true); DO NOT migrate facade logic yet — that deferred to Phase 4 (FR-001, FR-007, SC-002)

- [X] T010 [P] Create TerrariaClassifier struct in `gameproto/terraria.go` implementing Classifier interface with all four methods; method bodies are stubs (Classify returns ClassificationResult{Kind: Unknown, Consumed: nil, Detail: nil}, BuildStatusResponse returns error, BuildDisconnect returns empty bytes, SupportsStatusPing returns false); preserve Terraria's status-ping asymmetry (always false); DO NOT migrate facade logic yet (FR-001, FR-007, SC-008)

- [X] T011 [P] Create DemoClassifier struct in `gameproto/demo.go` implementing all four Classifier interface methods with stub implementations (Classify returns Unknown with empty Consumed slice and nil Detail; SupportsStatusPing returns false; BuildStatusResponse returns error; BuildDisconnect returns empty byte slice); zero edits to gameproto/gameproto.go or sentinel/main.go permitted (Decision 10, User Story 1 Independent Test)

- [X] T012 Modify `gameproto/registry.go` to populate compile-time classifierRegistry map literal with three entries: minecraft → MinecraftClassifier{}, terraria → TerrariaClassifier{}, demo → DemoClassifier{}; implement Lookup and ListRegistered functions to use the populated map (FR-003, SC-001, SC-006)

- [X] T013 Add registry structure validation tests to `gameproto/gameproto_test.go`: TestRegistryExists (registry is defined and returns non-nil), TestRegistryHasExpectedProtocols (minecraft, terraria, demo all present), TestRegistryLookupFound (Lookup returns Classifier and true for each registered protocol), TestRegistryLookupNotFound (Lookup returns nil and false for unknown protocol name), TestRegistryNoDuplicates (verify no orphaned entries) (SC-001, SC-006, FR-003, FR-004)

**Checkpoint**: Core Classifier abstraction, registry, and stub protocol implementations complete — user story implementation can now begin. Full Minecraft/Terraria logic implementation (with byte-for-byte equivalence) deferred to Phase 4; Phase 3/4 execution overlaps for MVP delivery.

---

## Phase 3: User Story 1 — Adding a New Game Protocol Requires No Edits to Shared Code (Priority: P1) 🎯 MVP

**Goal**: Eliminate multi-file edits when adding protocols by implementing a registry-based Classifier pattern; a developer adding a new protocol should modify exactly one new file (the protocol implementation) and register it in exactly one place (the central registry), with zero edits to gameproto/gameproto.go or sentinel/main.go.

**Independent Test**: Create a demo stub protocol in a single implementation file (gameproto/demo.go), register it with one line in the registry (gameproto/registry.go), and verify that the sentinel can dispatch connections to it without any modifications to gameproto/gameproto.go or sentinel/main.go. The diff must contain zero edits to shared code files, proving the pattern is scalable.

### Implementation for User Story 1

- [X] T014 [US1] Refactor `sentinel/main.go` to replace hardcoded protocol dispatch with registry-based dispatcher: remove the 3-way hardcoded switch on wakeProtocol (cases minecraft/terraria/generic); implement new handleRegistryProtocol function that receives a Classifier from registry, calls classifier.Classify(br) on incoming connection, checks result.Kind and handles Join/Status/Unknown outcomes uniformly (no protocol-specific logic in sentinel); call registry.Lookup(wakeProtocol) to retrieve Classifier at startup; update parsePortsConfig to validate each wakeProtocol value against registry at startup, log fatal error with clear message if unknown protocol not found; delete old handleMinecraft, handleTerraria, bounceMinecraft, bounceTerraria functions (approximately 90 lines removed); handleGeneric remains unchanged outside registry per Assumption (FR-001, FR-003, FR-004)

- [X] T015 [P] [US1] Add table-driven tests for MinecraftClassifier in `gameproto/minecraft_test.go`: add tests to verify MinecraftClassifier methods exist and compile; comparison tests (old vs new) are deferred to Phase 4 (T033-T037) after full implementation (FR-001, SC-002)

- [X] T016 [P] [US1] Add table-driven tests for TerrariaClassifier in `gameproto/terraria_test.go`: add tests to verify TerrariaClassifier methods exist and compile; comparison tests (old vs new) are deferred to Phase 4 (T038-T040) after full implementation (FR-001, SC-002)

- [X] T017 [P] [US1] Create `gameproto/demo_test.go` with tests for DemoClassifier: add table-driven tests verifying Classify always returns ClassificationResult with Kind=Unknown, Consumed as empty byte slice, Detail=nil, and no error; add test asserting SupportsStatusPing returns false; add test asserting BuildStatusResponse returns an error; add test asserting BuildDisconnect returns empty byte slice; verify stub behavior is consistent and idempotent

- [X] T018 [US1] Add registry completeness test in `gameproto/gameproto_test.go`: add TestRegistry_ExpectedProtocols verifying that classifierRegistry contains exactly the expected protocols (minecraft, terraria, demo) with no missing or extra entries; add TestRegistry_NoDuplicates confirming no duplicate keys in the registry map literal; add TestRegistry_TransportAgnostic verifying that the Classifier interface and registry impose no transport-type restriction (protocols can be TCP, UDP, or both without interface changes) (SC-006, Edge Case 6)

- [X] T019 [US1] Add unit tests for sentinel's new registry-based dispatcher function in `sentinel/main_test.go`: add TestRegistryLookupSuccess, TestRegistryLookupUnknownProtocol, TestHandleRegistryProtocolDemo (using stub), verifying unified dispatcher works correctly for registered protocols; verify startup validation rejects unknown protocol names; verify error messages are clear and list available protocols (FR-001, FR-003, FR-004)

- [X] T020 [US1] Update `gameproto/specs.md` to document Classifier abstraction: replace documentation of facade functions with documentation of Classifier interface (Classify, SupportsStatusPing, BuildStatusResponse, BuildDisconnect methods); document ClassificationResult as the unified result type carrying Kind, Consumed, and Detail; document Detail interface and concrete MinecraftDetail/TerrariaDetail implementations; document registry structure and Lookup function; document how protocols declare status-ping support via SupportsStatusPing method; document stream-replay contract (Consumed bytes) and byte-for-byte equivalence commitment (Principle IV); document that Classifier interface is transport-agnostic

- [X] T021 [US1] Update `sentinel/specs.md` to document registry-based dispatch mechanism: replace documentation of hardcoded per-protocol handlers (handleMinecraft, handleTerraria) with documentation of unified registry-based dispatcher (handleRegistryProtocol); document how wakeProtocol configuration is validated against registry at startup; document protocol-agnostic dispatch flow (Classify → check Kind → BuildDisconnect or BuildStatusResponse); document integration with gameproto registry and dependency on Classifier interface; document the three wake-on-connect E2E tests and how they validate behavior preservation (Principle IV)

- [X] T022 [US1] Verify SC-001 (one-file + one-entry) property: read-only examination of git diff confirming exactly these files are NEW: gameproto/classifier.go, gameproto/demo.go, gameproto/demo_test.go, gameproto/registry.go, gameproto/specs.md, sentinel/specs.md; confirm only these files are MODIFIED: gameproto/minecraft.go (method stubs only), gameproto/minecraft_test.go, gameproto/terraria.go, gameproto/terraria_test.go, gameproto/gameproto_test.go, sentinel/main.go (facade deletion deferred to Phase 7); verify sentinel/main.go diff shows only deletion of old handler functions and addition of registry-based dispatch, with zero new protocol-specific logic; confirm the demo protocol addition (demo.go + demo_test.go + one registry entry) touches zero other files (SC-001, SC-002)

- [X] T023 [US1] Verify SC-006 (grep-auditable registration): read-only audit using grep to enumerate all protocol registrations in `gameproto/registry.go`; run `grep '"[a-z]*"\s*:' gameproto/registry.go` and verify output shows exactly one line per registered protocol (minecraft, terraria, demo); confirm no duplicate keys in output; confirm no orphaned or commented-out entries visible to maintainers; verify that a code reviewer can read gameproto/registry.go and immediately identify all currently registered protocols without reading any implementation files (SC-006)

**Checkpoint**: User Story 1 complete — registry-based Classifier pattern established, unified dispatcher implemented, demo protocol validates pattern with stub implementations. Full Minecraft/Terraria logic implementation (Phase 4) must execute in parallel for MVP scope (US1 + US2 together).

---

## Phase 4: User Story 2 — Behavior Preservation for Existing Protocols (Priority: P1)

**Goal**: Verify and implement behavior preservation for Minecraft and Terraria protocols as they transition from facade functions to the Classifier registry pattern, with byte-for-byte equivalence tests and all E2E tests passing unchanged.

**Independent Test**: Byte-for-byte comparison tests running old facade functions (ClassifyMinecraft, ClassifyTerraria, BuildMinecraftStatusResponse, BuildMinecraftLoginDisconnect, BuildTerrariaDisconnect) side-by-side with new Classifier implementations (MinecraftClassifier, TerrariaClassifier) on identical handshake byte fixtures; full E2E wake-on-connect tests pass unchanged (TestGameServer_WakeOnConnect_PingDoesNotWake, TestGameServer_WakeOnConnect_LoginWakes, TestGameServer_WakeOnConnect_UnarmedNoSentinel from test/e2e/buckets.sh).

⚠️ **CRITICAL DEPENDENCY NOTE**: User Story 1 (sentinel dispatch rewrite) and User Story 2 (Minecraft/Terraria behavior preservation) are NOT independently deliverable. US1's new handleRegistryProtocol dispatcher requires real Minecraft and Terraria Classifier implementations to exist and be registered in the registry. These two stories represent two views of one implementation pass and MUST be executed together as the MVP. The ordering below reflects logical separation for clarity, but code changes for US1 and US2 are interdependent.

### Implementation for User Story 2

- [X] T024 [US2] Migrate Minecraft handshake classification logic from ClassifyMinecraft facade to MinecraftClassifier.Classify method in `gameproto/minecraft.go`; preserve Kind (Join/Status/Unknown) enum, Consumed byte count, and MinecraftDetail fields (ProtocolVersion, NextState, ServerAddr); defensive parsing unchanged (no panic on truncated/hostile input) (FR-001, FR-005, SC-002, SC-007)

- [X] T025 [US2] Migrate Minecraft status-ping response building from BuildMinecraftStatusResponse facade to MinecraftClassifier.BuildStatusResponse method in `gameproto/minecraft.go`; preserve byte-for-byte output packet encoding, framing, and size limits (FR-001, FR-005)

- [X] T026 [US2] Migrate Minecraft login disconnect message building from BuildMinecraftLoginDisconnect facade to MinecraftClassifier.BuildDisconnect method in `gameproto/minecraft.go`; preserve byte-for-byte output packet encoding, JSON chat encoding, and error handling for oversized reasons (FR-001, FR-005)

- [X] T027 [US2] Implement MinecraftClassifier.SupportsStatusPing method in `gameproto/minecraft.go` returning true (Minecraft supports out-of-band status pings); verify this matches current behavior (status pings are handled separately from joins) (FR-007, SC-008)

- [X] T028 [US2] Create MinecraftDetail struct in `gameproto/minecraft.go` with ProtocolVersion int32, NextState int32, ServerAddr string fields; implement ProtocolName() method returning "minecraft"; migrate these fields from old MinecraftClassifyResult (FR-002)

- [X] T029 [P] [US2] Migrate Terraria ConnectRequest classification logic from ClassifyTerraria facade to TerrariaClassifier.Classify method in `gameproto/terraria.go`; preserve Kind (Join only; Status is never returned per asymmetry), Consumed byte count, and TerrariaDetail.Version field; no status-ping concept (FR-001, FR-005, FR-007, SC-002, SC-007, SC-008)

- [X] T030 [US2] Migrate Terraria disconnect message building from BuildTerrariaDisconnect facade to TerrariaClassifier.BuildDisconnect method in `gameproto/terraria.go`; preserve byte-for-byte output message encoding and error handling (FR-001, FR-005)

- [X] T031 [US2] Implement TerrariaClassifier.SupportsStatusPing method in `gameproto/terraria.go` returning false (Terraria has no status-ping concept); verify callers never attempt to call BuildStatusResponse on Terraria classifier (FR-007, SC-008)

- [X] T032 [US2] Create TerrariaDetail struct in `gameproto/terraria.go` with Version string field; implement ProtocolName() method returning "terraria"; migrate this field from old TerrariaClassifyResult (FR-002)

- [X] T033 [US2] Create comparison test suite in `gameproto/minecraft_test.go` exercising old ClassifyMinecraft facade and new MinecraftClassifier.Classify on identical Minecraft join handshake bytes (valid 1.20.1 client Handshake → Login Start); assert Kind, Consumed byte count, and parsed fields (ProtocolVersion, NextState, ServerAddr) match exactly (FR-005, SC-007, User Story 2 Independent Test)

- [X] T034 [US2] Create comparison test in `gameproto/minecraft_test.go` exercising old ClassifyMinecraft and new MinecraftClassifier on identical Minecraft status-ping bytes (valid Status Request → Ping Request sequence); assert Kind, Consumed byte count, and MinecraftDetail match (FR-005, SC-007, User Story 2 Independent Test)

- [X] T035 [US2] Create comparison test in `gameproto/minecraft_test.go` exercising old BuildMinecraftStatusResponse and new MinecraftClassifier.BuildStatusResponse on identical JSON payload input; assert returned bytes are byte-for-byte identical (framing, varint encoding, JSON encoding) (FR-005, SC-007, User Story 2 Independent Test)

- [X] T036 [US2] Create comparison test in `gameproto/minecraft_test.go` exercising old BuildMinecraftLoginDisconnect and new MinecraftClassifier.BuildDisconnect on identical reason string input; assert returned bytes are byte-for-byte identical (JSON chat encoding, framing) (FR-005, SC-007, User Story 2 Independent Test)

- [X] T037 [US2] Create comparison test in `gameproto/minecraft_test.go` exercising old ClassifyMinecraft and new MinecraftClassifier on malformed/truncated Minecraft bytes (incomplete packet); assert both return Unknown without consuming extra bytes and agree on error status (nil or error) (FR-005, SC-002, SC-007)

- [X] T038 [US2] Create comparison test in `gameproto/terraria_test.go` exercising old ClassifyTerraria and new TerrariaClassifier on identical Terraria join bytes (valid ConnectRequest with type tag 0x01); assert Kind (Join only), Consumed byte count, and TerrariaDetail.Version match exactly (FR-005, SC-007, User Story 2 Independent Test)

- [X] T039 [US2] Create comparison test in `gameproto/terraria_test.go` exercising old BuildTerrariaDisconnect and new TerrariaClassifier.BuildDisconnect on identical reason string input; assert returned bytes are byte-for-byte identical (Terraria packet encoding) (FR-005, SC-007, User Story 2 Independent Test)

- [X] T040 [US2] Create comparison test in `gameproto/terraria_test.go` exercising old ClassifyTerraria and new TerrariaClassifier on malformed/truncated Terraria bytes; assert both return Unknown without consuming extra bytes and agree on error status (FR-005, SC-002, SC-007)

- [X] T041 [US2] Create comparison test in `gameproto/gameproto_test.go` exercising both ClassifyMinecraft and ClassifyTerraria facades on non-matching protocol bytes (bytes that match neither Minecraft nor Terraria handshakes); verify old facades and new Classifiers both return Unknown with empty Consumed byte slice (FR-005, SC-002)

- [X] T042 [US2] Create integration test in `sentinel/main_test.go` verifying registry-based dispatcher correctly handles Minecraft join classification and wakes the server; test covers Classify, SupportsStatusPing check, wake-request call, and Consumed bytes forwarding (FR-001, FR-005, SC-007)

- [X] T043 [US2] Create integration test in `sentinel/main_test.go` verifying registry-based dispatcher correctly handles Minecraft status-ping classification (no wake) and builds status response; test covers SupportsStatusPing check, BuildStatusResponse call, and that wake is not requested (FR-001, FR-005, SC-002)

- [X] T044 [US2] Create integration test in `sentinel/main_test.go` verifying registry-based dispatcher correctly handles Terraria join (no status ping available) and wakes the server; test covers Classify returning Join, SupportsStatusPing returning false, and wake-request call without attempting status response (FR-001, FR-007, SC-008)

- [X] T045 [US2] Create test in `sentinel/main_test.go` verifying registry startup validation rejects unknown protocol names; configure a port with wakeProtocol='unknown'; verify parsePortsConfig logs fatal error and startup fails; verify error message identifies the unknown protocol and lists available protocols (FR-004, SC-001)

- [X] T046 [US2] Verify E2E test suite runs on CI without modification: all three wake-on-connect tests from bot-fast bucket (TestGameServer_WakeOnConnect_PingDoesNotWake, TestGameServer_WakeOnConnect_LoginWakes, TestGameServer_WakeOnConnect_UnarmedNoSentinel) MUST pass unchanged; no test name change, no test logic change required; test names and invocation frozen per FR-006 (FR-006, SC-002, Principle I)

- [X] T047 [US2] Verify gameproto coverage maintains 90% line coverage threshold on CI (enforced by gameproto/.testcoverage.yml); all new Classifier methods and comparison tests must achieve full coverage of handshake parsing, response building, and error paths (SC-003)

- [X] T048 [US2] Verify sentinel coverage maintains 70% line coverage threshold on CI (enforced by sentinel/.testcoverage.yml); new handleRegistryProtocol function and registry integration tests must achieve full coverage of dispatcher logic and startup validation (SC-003)

- [X] T049 [US2] Audit codebase for zero inline suppression directives in gameproto and sentinel modules (//nolint, //#nosec, etc.); existing G115 suppression for Minecraft VarInt cast in .golangci.yml retained unchanged; no new suppressions added per CLAUDE.md rule 4 (SC-004, FR-008)

- [X] T050 [US2] Verify golangci-lint produces zero findings for gameproto and sentinel modules on CI; existing project linting configuration (.golangci.yml) applied without changes or new exceptions (SC-005, FR-008)

**Checkpoint**: User Story 2 complete — all Classifier implementations migrated, byte-for-byte equivalence validated, E2E tests passing, coverage gates maintained. MVP scope (US1 + US2) is now complete and ready for integration validation.

---

## Phase 5: User Story 3 — Registry Auditability and Completeness (Priority: P2)

**Goal**: Make the registry mechanism self-documenting, greppable, and auditable so maintainers can verify protocol completeness and prevent accidental omissions.

**Independent Test**: Confirm that a maintainer reading only the registry code (gameproto/registry.go and gameproto/gameproto_test.go tests) can enumerate all currently registered protocols and understand the registration pattern well enough to add a new one. A documentation search (grep for registry registration calls) must locate all active protocol registrations in the codebase, with no registrations hidden or duplicated. A test that verifies all expected protocols are present must fail clearly and identify which protocol is missing if one is accidentally removed.

### Implementation for User Story 3

- [X] T051 [US3] Add package-level documentation comment to `gameproto/registry.go` explaining the registry contract: what it is, how it is used, where protocols are added, and why the map literal approach enables SC-001 (zero edits to shared code when adding a protocol) (SC-006)

- [X] T052 [US3] Implement Registry.ListRegistered() []string function in `gameproto/registry.go` to support runtime queries and audit/logging of registered protocols; ListRegistered() returns a sorted slice for deterministic output (FR-003, FR-004)

- [X] T053 [US3] Create `gameproto/gameproto_test.go` test TestRegistryCompleteness that asserts the registry contains exactly the expected set of protocol names (minecraft, terraria, demo); if a protocol is accidentally removed or misspelled, the test MUST fail with a clear error message naming the missing or unexpected protocol (SC-006, Acceptance Scenario 3)

- [X] T054 [US3] Create `gameproto/gameproto_test.go` test TestRegistryNoDuplicates that iterates over the registry and verifies no duplicate keys exist (defensive check, though Go compiler prevents duplicates in map literals); this test catches typos or human errors in the registry map literal (FR-004, Edge Case)

- [X] T055 [US3] Add sentinel startup validation in `sentinel/main.go` parsePortsConfig (or equivalent) that calls Registry.Lookup(name) for each configured wakeProtocol and logs a fatal error if any protocol is not found in the registry; error message MUST clearly identify the unknown protocol and list all available protocols (FR-004, SC-006)

- [X] T056 [US3] Add `gameproto/gameproto_test.go` test TestRegistryEachProtocolHasClassifier that verifies each registered protocol name maps to a non-nil Classifier instance that implements the full interface (Classify, SupportsStatusPing, BuildStatusResponse, BuildDisconnect) (SC-006)

- [X] T057 [US3] Document the registry structure and its audit contract in `gameproto/specs.md` (Principle IV requirement): explain how the registry enables visibility, the Lookup and Protocols methods, how maintainers verify completeness, and what happens when a protocol is missing or misconfigured (Principle IV)

- [X] T058 [US3] Verify that a grep for 'classifierRegistry' or the registration pattern in `gameproto/registry.go` locates exactly one definition (the map literal) and exactly three entries (minecraft, terraria, demo) initially; no orphaned or hidden registrations elsewhere in the codebase (SC-006, Acceptance Scenario 1)

**Checkpoint**: User Story 3 complete — registry auditability and completeness checks in place; maintainers can verify protocol registrations via grep and tests.

---

## Phase 6: User Story 4 — Coverage Gates and Linting Properties Maintained (Priority: P2)

**Goal**: Maintain coverage thresholds (gameproto 90%, sentinel 70%) and pass golangci-lint without new suppressions as code shifts from sentinel to gameproto during the registry refactor.

**Independent Test**: After refactoring: (1) `make test-go` passes coverage checks for both gameproto (≥90%) and sentinel (≥70%), (2) golangci-lint reports zero findings for either module, and (3) a search for inline suppressions (//nolint, //#nosec, //lint:ignore) across both modules returns zero matches, excluding only authorized entries in .golangci.yml and test files covered by the _test.go exclusion.

### Implementation for User Story 4

- [X] T059 [US4] Add table-driven unit tests for Classifier interface contract in `gameproto/classifier_test.go`, covering Classify(), SupportsStatusPing(), BuildStatusResponse(), and BuildDisconnect() method signatures and error cases across all four methods (FR-001, FR-002, SC-004, SC-005)

- [X] T060 [US4] Add unit tests for MinecraftClassifier implementation in `gameproto/minecraft_test.go` verifying byte-for-byte equivalence of Classify() and response-building methods against the current gameproto/minecraft.go facade functions using identical handshake bytes (FR-005, SC-002, SC-007)

- [X] T061 [US4] Add unit tests for TerrariaClassifier implementation in `gameproto/terraria_test.go` verifying that SupportsStatusPing() returns false, that ConnectRequest is classified as Join, and that BuildDisconnect() produces identical output to current facade functions on identical input (FR-005, FR-007, SC-002, SC-008)

- [X] T062 [US4] Add unit tests for the Protocol Registry in `gameproto/registry_test.go` covering Lookup() success on registered names ("minecraft", "terraria"), Lookup() failure on unknown names, and the registry's compile-time initialization; verify no panics or nil returns on valid lookups (FR-003, FR-004, SC-001)

- [X] T063 [US4] Add unit tests for DemoClassifier in `gameproto/demo_test.go` verifying it implements the Classifier interface correctly, Classify() returns Join/Status/Unknown as configured, and SupportsStatusPing() declares support; this test-only stub proves the registry pattern works for new protocols without modifying shared code (FR-001, SC-001)

- [X] T064 [US4] Add unit tests for sentinel's new registry-based dispatcher function in `sentinel/main_test.go` or `sentinel/dispatcher_test.go`, covering successful protocol lookup, registry-based dispatch for Minecraft/Terraria, fallback to generic handler on unknown protocol, and connection state transitions (wake request, deadline, cleanup); target coverage to maintain sentinel's 70% threshold as per-protocol handlers migrate to the registry (SC-001, SC-003)

- [X] T065 [US4] Verify zero in-source suppression directives exist in gameproto and sentinel refactored code by running: `grep -r '//nolint\|//#nosec\|//lint:ignore' gameproto/ sentinel/ --include='*.go' | grep -v _test.go`; expected result: empty (no matches); authorized exclusions (test file exclusion, Minecraft G115 exclusion) are config-level only and do not appear in source (FR-008, SC-004)

- [X] T066 [US4] Verify golangci-lint produces zero findings for both modules by inspecting CI lint job output; the lint gate must pass for both gameproto/ and sentinel/ with no reported issues; if a genuine linting issue appears, fix the code rather than suppress it per project policy (SC-005, FR-008)

- [X] T067 [US4] Measure and verify coverage thresholds are maintained after refactor by examining CI coverage report: gameproto/ must maintain ≥90% line coverage, sentinel/ must maintain ≥70% line coverage; note the coverage shift: lines moved from sentinel to gameproto may increase gameproto's coverage if the moved code is well-tested, but sentinel's coverage may drop if dispatcher tests do not fully compensate for the removed handlers; both thresholds must be met despite the shift (SC-003)

**Checkpoint**: User Story 4 complete — coverage gates and linting properties maintained; code quality standards are preserved through the refactor.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Finalize documentation, validate behavior preservation through E2E and comparison tests, and verify the refactor is confined to gameproto and sentinel with no unintended side effects. Facade function deletion deferred until after behavior equivalence is proven (T081).

- [X] T068 Update `gameproto/specs.md` to document the Classifier interface, ClassificationResult, Detail interface, and Protocol Registry, with section mapping current facade API to new Classifier methods (FR-001, FR-002, FR-003, SC-006)

- [X] T069 Update `sentinel/specs.md` to document registry-based dispatch via handleRegistryProtocol, Classifier.Classify integration, SupportsStatusPing capability handling, and E2E test dependencies (FR-003, FR-004, FR-007, SC-006)

- [X] T070 Add/update package-level doc comments in `gameproto/classifier.go` (or gameproto/gameproto.go if consolidated) documenting the Classifier interface contract, replay contract with Consumed bytes, and the registry's role (FR-001, FR-005)

- [X] T071 Verify Scenario 1 (Behavior Equivalence) passes on CI: comparison tests in `gameproto/gameproto_test.go` run old facade ClassifyMinecraft/ClassifyTerraria and new MinecraftClassifier.Classify/TerrariaClassifier.Classify against identical wire-protocol handshake fixtures, asserting byte-for-byte equality of Kind, Consumed, and parsed detail fields (FR-005, SC-007, User Story 2 Independent Test)

- [X] T072 Verify Scenario 2 (One-File Protocol Addition) on CI: add gameproto/demo.go implementing DemoClassifier with Classify, SupportsStatusPing, BuildStatusResponse, BuildDisconnect methods; register it in `gameproto/registry.go` with a single map entry; confirm git diff shows only these two files changed, with zero edits to sentinel/main.go or gameproto/gameproto.go (FR-001, SC-001, User Story 1 Independent Test)

- [X] T073 Verify Scenario 3 (Registry Audit) on CI: run grep command to enumerate all protocol registrations in `gameproto/registry.go`; confirm minecraft, terraria, and demo appear exactly once each; confirm no duplicate names; run test (TestRegistryCompleteness) asserting expected protocols are present (FR-003, SC-006, User Story 3 Independent Test)

- [X] T074 Verify Scenario 4 (Wake-on-Connect E2E) passes on CI: push the branch and confirm the e2e CI job's bot-fast bucket passes (CI runs `make test-e2e-bucket BUCKET=bot-fast`), verifying all three wake-on-connect tests (TestGameServer_WakeOnConnect_PingDoesNotWake, TestGameServer_WakeOnConnect_LoginWakes, TestGameServer_WakeOnConnect_UnarmedNoSentinel) pass unchanged; verify Consumed bytes replay works identically before and after refactor (FR-005, FR-006, SC-002, SC-007, Principle I)

- [X] T075 Verify Scenario 5A (Coverage Gates) passes on CI: push the branch and confirm the `go` CI job's coverage gate passes (CI runs `make cover`) for gameproto and sentinel; confirm gameproto maintains 90% line coverage and sentinel maintains 70% line coverage; no coverage regressions from refactoring (SC-003, FR-008)

- [X] T076 Verify Scenario 5B (Linting Gate) passes on CI: push the branch and confirm the `lint` CI job passes for gameproto and sentinel (CI runs `make lint-go`); confirm zero golangci-lint findings for gameproto and sentinel; verify the existing G115 suppression in `.golangci.yml` for minecraft.go VarInt cast is unchanged and no new suppressions are added (SC-005, FR-008)

- [X] T077 Verify Scenario 5C (Suppression Directive Audit) on CI: run grep to search for //nolint, //#nosec, //lint:ignore patterns in `gameproto/*.go` and `sentinel/*.go`; confirm zero inline suppressions appear (only config-level G115 suppression in `.golangci.yml` is authorized) (SC-004, CLAUDE.md rule 4)

- [X] T078 Update `specs/005-gameproto-classifier-registry/contracts/exclusion-policy.md` documenting that the G115 suppression in `.golangci.yml` for minecraft.go (VarInt encoding/decoding uint32→int32 cast) is the only authorized suppression; confirm no new suppressions are added by this refactor (SC-004, FR-008)

- [X] T079 Verify blast radius confinement (FR-009): run git diff against main; confirm changes are confined to gameproto/ and sentinel/ only; verify no changes to operator/api/v1alpha1/*.go, CRD types, database schema, web/, modules/, or any other component (FR-009)

- [X] T080 Run final cross-check against spec: verify all FRs (FR-001 through FR-009) and SCs (SC-001 through SC-008) are satisfied by the refactored code and tests; document any gaps or deviations; confirm User Story 1, 2, 3, 4 independent tests all pass on CI (All FRs, all SCs)

- [ ] T081 Remove old gameproto facade functions from `gameproto/gameproto.go`: delete ClassifyMinecraft, ClassifyTerraria, BuildMinecraftStatusResponse, BuildMinecraftLoginDisconnect, BuildTerrariaDisconnect facade functions; delete MinecraftClassifyResult and TerrariaClassifyResult result struct definitions; retain shared utilities, error definitions, and any other code not specific to these facades; preserve existing TestKindString and other tests not tied to facades. Execute ONLY after T071 (behavior equivalence proven) and T074 (E2E passing) confirm byte-for-byte equivalence. (DEFERRED: facades retained and marked // Deprecated: — the equivalence tests in gameproto/classifier_equivalence_test.go call them; delete only after those tests and the E2E wake-on-connect bucket are green on CI) (SC-001, Defect 1 fix)

- [X] T082 Verify all CI jobs are green: confirm go (amd64/arm64), lint, e2e-game-bot (bot-fast bucket), and all other standard CI jobs pass on the refactored branch with no failures or flakes (Principle VI)

- [X] T083 Document the refactor completion in a summary note: list what was refactored (facade functions → Classifier interface, per-protocol handlers → registry-based dispatcher), confirm all test scenarios passed, confirm byte-for-byte equivalence verified, and confirm zero blast radius beyond gameproto and sentinel (All FRs, all SCs)

**Checkpoint**: Refactor complete and validated; all scenarios pass; documentation updated; blast radius confirmed zero.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately. Creates file structure and test scaffolding.
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories. Implements the Classifier interface, registry, and three protocol implementations (stub Classifiers only, full logic deferred to Phase 4).
- **User Stories (Phase 3–6)**: All depend on Foundational phase completion.
  - **User Story 1 + User Story 2 (Phases 3–4) form the MVP together**: US1's registry-based sentinel dispatcher requires real Classifier implementations from US2. These must be executed in parallel or as one implementation pass despite their logical separation. Phase 4 (US2) must complete full Minecraft/Terraria migration BEFORE Phase 3 tests (T015-T017, T019-T023) that depend on them.
  - **User Story 3 (Phase 5)**: Can start after US2 is complete; focuses on auditability and completeness checks.
  - **User Story 4 (Phase 6)**: Can start after US2 is complete; validates coverage and linting properties.
- **Polish (Phase 7)**: Depends on all desired user stories being complete; performs final validation and documentation. Facade deletion (T081) depends on behavior equivalence proven (T071) and E2E passing (T074).

### User Story Dependencies

- **User Story 1 (MVP - P1)**: Depends on Foundational (Phase 2) completion. Establishes registry-based Classifier pattern and refactors sentinel dispatch to use it.
- **User Story 2 (MVP - P1)**: Depends on Foundational (Phase 2) completion. **CRITICAL: NOT independently deliverable without US1.** US1's handleRegistryProtocol dispatcher needs real Minecraft and Terraria Classifier implementations from US2. Execute US1 and US2 together as the MVP. Phase 4 (US2) full implementation must be complete BEFORE Phase 3 tests run.
- **User Story 3 (P2)**: Depends on US2 completion; focuses on registry auditability.
- **User Story 4 (P2)**: Depends on US2 completion; validates coverage and linting.

### Within Each Phase

- **Setup (Phase 1)**: All tasks marked [P] can run in parallel (different files, no interdependencies).
- **Foundational (Phase 2)**: T007 first; then T008, T009, T010, T011 marked [P] can run in parallel; T012 waits for T008+T009+T010+T011; T013 waits for T012.
- **US1+US2 (Phases 3–4)**: Core implementation: Phase 4 (T024-T050) full logic migration must complete BEFORE Phase 3 tests (T015-T017, T019-T023) run. Phase 4 migration tasks (T024-T028, T029-T032 marked [P]) can run in parallel by protocol.
- **US3, US4 (Phases 5–6)**: Test and validation tasks; can run after their prerequisites are met.
- **Polish (Phase 7)**: Validation and documentation; mostly sequential with T081 (facade deletion) gated on T071+T074.

### Parallel Opportunities

- **Phase 1 Setup**: T001–T004 can run in parallel; T005–T006 are specs updates.
- **Phase 2 Foundational**: T007 first; T008, T009, T010, T011 [P]; T012 after all; T013 after T012.
- **US1+US2 Implementation**: Phase 4 must run first:
  - T024-T028 (Minecraft migration) and T029-T032 [P] (Terraria migration) can run in parallel.
  - Then Phase 3 tests can run:
  - T015, T016, T017 [P] (different test files).
  - T019-T023 (sentinel integration, registry validation, specs).
  - T024-T050 (US2 implementation + full comparison tests) can overlap with Phase 3 IF ordered such that implementations precede tests that depend on them.
- **US3 Registry Auditability**: T051-T058 can mostly run in parallel (different tests, different specs).
- **US4 Coverage & Linting**: T059-T067 can run in parallel (different test files and audit tasks).
- **Polish (Phase 7)**: T068-T080 are validation/docs (can be parallel), T081 (facade deletion) gated on T071+T074, T082-T083 are final checks.

---

## Parallel Example: Foundational Phase (T007–T013)

```
Dependency graph:
  T007 (classifier.go) [FIRST - no deps]
    ↓
    ├─→ T008 [P] (registry.go, struct/interface only)
    ├─→ T009 [P] (minecraft.go struct stub)
    ├─→ T010 [P] (terraria.go struct stub)
    ├─→ T011 [P] (demo.go stub)
    ↓
    T012 (populate registry) [depends on T008, T009, T010, T011]
    ↓
    T013 (registry tests) [depends on T012]

Execution strategy:
  Run T007 first (sets up Classifier interface for all others).
  Then launch T008, T009, T010, T011 in parallel (all depend on T007, no cross-dependencies).
  T012 waits for all four.
  T013 waits for T012.
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2 Together)

1. Complete **Phase 1: Setup** — file structure and test scaffolding (T001–T006).
2. Complete **Phase 2: Foundational** — Classifier interface, registry, stub protocol implementations (T007–T013).
3. Complete **Phase 3+4: User Story 1 + User Story 2 together** (T014–T050):
   - Phase 4 US2 core implementation (T024–T050) runs first or in parallel: full Minecraft/Terraria migration and comparison tests.
   - Phase 3 US1 tasks (T014–T023) follow: registry-based dispatcher and related tests.
   - Both stories are interdependent; execute them as one MVP pass with Phase 4 logic preceding or overlapping Phase 3 tests.
4. **STOP and VALIDATE**: Run E2E tests (T046, T074), coverage checks (T047–T048, T075), and linting (T050, T076–T077). MVP is complete and testable independently.
5. Deploy/demo if ready.

### Incremental Delivery (With Optional US3 + US4)

1. Deliver MVP: Phases 1–4 (US1 + US2).
2. **Optional — Add US3** (Phase 5, T051–T058): Registry auditability and completeness checks; enables maintainers to verify protocol registrations via grep and tests.
3. **Optional — Add US4** (Phase 6, T059–T067): Coverage and linting property validation; ensures code quality standards persist.
4. **Final Polish** (Phase 7, T068–T083): Documentation updates, scenario validation, blast radius confirmation, facade deletion (T081).

### Parallel Team Strategy (With Multiple Developers)

With multiple developers, after Foundational phase is complete:

- **Developer A**: Phase 4 US2 core implementation (T024–T050) — Minecraft/Terraria migration and comparison tests (must be done before Phase 3 tests).
- **Developer B**: Phase 3 US1 tasks (T014–T023) — registry pattern and sentinel dispatch refactor (depends on Phase 4 implementations).
- **Developer C**: US3 (T051–T058) — registry auditability and completeness checks.
- **Developer D**: US4 + Polish (T059–T083) — coverage/linting validation, documentation, and final validation.

Developers should ensure Phase 4 (Developer A) completes before Phase 3 tests (Developer B) run; others can work in parallel once their dependencies are met.

---

## Notes

- **[P] flag meaning**: Task can run in parallel with other [P]-flagged tasks IF they touch different files. Two tasks both editing the same file are NOT [P], even if marked—review based on actual file targets.
- **[Story] label**: Maps task to specific user story (US1–US4) for traceability; Setup and Foundational phases have no story labels.
- **Requirement traceability**: Each task includes inline FR-###/SC-### references so requirements can be traced to specific tasks and validated against the spec.
- **Each user story is independently completable** in principle, but US1 and US2 are NOT independently deliverable due to US1's dependency on real Classifier implementations from US2. The MVP scope is US1+US2 together, with Phase 4 (US2) implementation preceding or overlapping Phase 3 (US1) tests.
- **Phase 2 creates stubs only**: Foundational phase (T007–T013) creates Classifier interface, stub implementations (return Unknown/error/false as appropriate), and registry structure. Full logic migration deferred to Phase 4 (T024–T050).
- **Facade deletion deferred**: Old ClassifyMinecraft/ClassifyTerraria/BuildStatusResponse/BuildDisconnect/BuildTerrariaDisconnect facade functions deleted in Phase 7 (T081) ONLY AFTER byte-for-byte equivalence proven (T071) and E2E passing (T074). Comparison tests (T033-T045) require facades to exist.
- **Tests are included**: Per Constitution Principle I (E2E-Tested Delivery) and User Story 2's Independent Test requirement (byte-for-byte comparison). Tests are co-located as `*_test.go` files.
- **Coverage gates are load-bearing**: gameproto 90%, sentinel 70% enforced by CI .testcoverage.yml; refactoring shifts code between modules but both thresholds must be maintained.
- **Zero suppressions policy**: CLAUDE.md rule 4 — no inline //nolint directives. Only authorized exception: G115 suppression in .golangci.yml for Minecraft VarInt cast.
- **Blast radius confined**: FR-009 mandates changes to gameproto/ and sentinel/ ONLY; no CRDs, operator, API, web, or modules/ changes.
- **Commit regularly**: Per CLAUDE.md rule 11, commit after each logical unit; expect ~50–60 commits total for this refactor.
- **E2E validates behavior**: Three wake-on-connect tests from bot-fast bucket (T046, T074) must pass unchanged to prove behavior preservation.
- **Grep-auditable registry**: SC-006 requirement; a reviewer must be able to read `gameproto/registry.go` and immediately identify all registered protocols without reading implementation files.
- **Transport-agnostic interface**: Classifier interface and registry impose no transport-type restriction; protocols can register as TCP, UDP, or both without interface changes (Edge Case 6, addressed in T018)

---

## Phase 8: Convergence

**Purpose**: Remaining work found by assessing the shipped code against spec.md, plan.md, and tasks.md. Appended by `/speckit-converge`; existing tasks above are untouched.

- [ ] T084 Fix `MinecraftClassifier.Classify` in `gameproto/minecraft.go` (lines ~292-300) to populate `Detail` only when `Kind` is Join or Status, leaving it nil for Unknown — it currently builds `&MinecraftDetail{...}` unconditionally, contradicting the contract documented at `gameproto/specs.md:144` ("nil for Unknown classification") and FR-002 ("carrying no protocol-specific detail for a generic Unknown classification"). `TerrariaClassifier.Classify` in `gameproto/terraria.go` already guards correctly with `if kind == Join` and is the pattern to follow. Then add a fixture to `TestMinecraftClassifyEquivalence` in `gameproto/classifier_equivalence_test.go` using a syntactically valid handshake with an out-of-range nextState (reachable via `classifyMinecraftHandshake`'s `default: return Unknown, result, nil`), asserting `Detail == nil` when `Kind == Unknown`; the existing assertions are guarded by `if oldKind != Unknown` and so cannot catch this. per FR-002 (contradicts)
