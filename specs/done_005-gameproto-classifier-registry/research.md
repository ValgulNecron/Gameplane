# Research: Protocol Classifier Registry Refactor

**Feature**: 005-gameproto-classifier-registry  
**Date**: 2026-08-20  
**Status**: Phase 0 Research Output  
**Scope**: Analysis of registry patterns, interface design, and verification strategy

---

## Executive Summary

This refactor replaces the current facade-function pattern (with per-protocol dispatch hardcoded in sentinel) with a registry-based Classifier abstraction. Adding a new protocol will require exactly one new implementation file + one registry entry, with zero edits to shared code (gameproto.go, sentinel/main.go). Byte-for-byte equivalence for Minecraft and Terraria is a hard constraint; all E2E tests must pass unchanged. Coverage gates (90% gameproto, 70% sentinel) and linting cleanness (zero suppressions) must be maintained.

---

## Decisions

### Decision 1: Registration Mechanism (Explicit Map Literal)

- **Decision**: Use an explicit, compile-time map literal in a new `gameproto/registry.go` file, populated with concrete Classifier instances for each protocol.

- **Rationale**:
  - Satisfies SC-001: Adding a new protocol requires editing only the new implementation file + one line in registry.go (map entry).
  - Satisfies SC-006: All registrations are grep-able in a single location (`grep ":" gameproto/registry.go`).
  - No hidden side effects or blank imports.
  - Precedent: The API uses explicit Mount* functions called from main.go (lines 237–281 in api/cmd/main.go), establishing this project's preference for explicit over implicit registration.
  - Registry.go is treated as a registry manifest (metadata), not business-logic shared code; the spec's intent is eliminating scattered multi-file edits to parsing and dispatch logic, not eliminating any shared-file touch.

- **Alternatives considered**:
  - Init-based self-registration (each protocol file has init()): Requires blank import in shared file to trigger initialization, violating SC-001.
  - Wire-up function called from sentinel/main.go: Requires editing sentinel/main.go, also violating SC-001.

---

### Decision 2: Classifier Interface Shape

- **Decision**: Define a Classifier interface with exactly four methods:
  - `Classify(br *bufio.Reader) (*ClassificationResult, error)` — parse handshake, classify as Join/Status/Unknown, return consumed bytes.
  - `SupportsStatusPing() bool` — declare whether the protocol supports out-of-band status pings (addresses FR-007).
  - `BuildStatusResponse(payload string) ([]byte, error)` — build a status-ping reply (if supported).
  - `BuildDisconnect(reason string) ([]byte, error)` — build a disconnect/timeout message.

- **Rationale**:
  - Encapsulates all protocol-specific logic in a single abstraction.
  - Classify returns a single unified result type (not a separate Kind return value) because FR-002 mandates one unified ClassificationResult; returning Kind separately duplicates a field that already exists on the result struct and creates two sources of truth that can disagree. Classify returns a non-nil result on every non-error path (including Unknown classification) because the caller needs `result.Consumed` to replay bytes to the generic handler losslessly.
  - SupportsStatusPing() is explicit, idiomatic Go (following the interface{} pattern for capabilities), and avoids type assertions in sentinel.
  - BuildStatusResponse and BuildDisconnect move response building into the protocol layer (currently Minecraft and Terraria facades do this), centralizing protocol knowledge.
  - Each protocol (Minecraft, Terraria, future) provides one Classifier implementation.

- **Alternatives considered**:
  - Optional StatusSupporter interface (type assertion): Viable but less discoverable; requires checking type assertion at every status-ping site.
  - Capability flags as separate fields in Classifier: More boilerplate; methods are cleaner.

---

### Decision 3: ClassificationResult Shape (Detail Interface)

- **Decision**: Define ClassificationResult as a struct carrying:
  - `Kind` — the classification (Join/Status/Unknown).
  - `Consumed []byte` — bytes read during handshake parsing (enables lossless stream replay per gameproto's replay contract).
  - `Detail Detail` — a small interface for protocol-specific metadata (nil for Unknown).
  
  Define a Detail interface with a single method `ProtocolName() string` for debugging. Each protocol implements a concrete Detail type (MinecraftDetail, TerrariaDetail) carrying its specific parsed fields (version, server address, etc.).

- **Rationale**:
  - Avoids `interface{}` entirely (per project strict no-unjustified-any rule).
  - Extensible: new protocols can implement Detail without editing ClassificationResult itself.
  - Type assertions are idiomatic Go; code that needs protocol-specific fields uses `if md, ok := result.Detail.(*MinecraftDetail) { ... }`.
  - Single, unified result type satisfies FR-002 ("one result type for all protocols").
  - Aligns with spec's Assumption: "Classifier results may omit detail that does not apply" — Unknown classification carries nil Detail.

- **Alternatives considered**:
  - Optional-pointer fields in ClassificationResult (MinecraftDetail *MinecraftDetail, TerrariaDetail *TerrariaDetail, ...): Bloats struct with many nil pointers; requires editing ClassificationResult for each new protocol.
  - Generics (ClassificationResult[D any]): Over-engineered for 2–3 protocols; complicates sentinel dispatch.

---

### Decision 4: "No Status Ping Support" as First-Class Case (Terraria)

- **Decision**: The `SupportsStatusPing()` method on Classifier is the primary mechanism. Terraria's classifier returns false; Minecraft's returns true. Sentinel checks this flag before attempting to build a status response.

- **Rationale**:
  - Addresses FR-007: "A protocol that does not support status pings must declare this cleanly."
  - Explicit and self-documenting: the capability is visible in the interface.
  - Scalable: if future protocols have other optional behaviors, additional methods can be added.
  - Avoids error returns or nil checks for missing behaviors; the flag cleanly separates "supported" from "unsupported."
  - Sentinel code is simple: `if classifier.SupportsStatusPing() && result.Kind == Status { ... }`

- **Alternatives considered**:
  - Protocol-specific interface (StatusSupporter): Requires type assertion and knowledge of that interface.
  - Optional-error return from BuildStatusResponse: Terraria would return an error for "not supported"; less clean than a capability flag.

---

### Decision 5: Duplicate-Name Registration Handling

- **Decision**: Use two mechanisms:
  1. Validated registry tests (TestRegistryNoDuplicates, TestRegistryCompleteness) in gameproto_test.go to catch duplicate or missing registrations at test time.
  2. Error-checked Lookup in sentinel at startup: if a protocol name from config cannot be found in the registry, log a fatal error and refuse to start.

- **Rationale**:
  - Idiomatic Go: tests catch structural errors; startup checks catch configuration mismatches.
  - Prevents silent failures: a typo in a protocol name is caught at CI (tests) before deployment.
  - Prevents duplicate silently overwriting earlier entries: tests verify exact set of expected protocols.
  - Panic at init time is not idiomatic; this avoids it.
  - The spec's Edge Case: "Can two protocols register themselves with the same name?" is preempted by the static map literal — no runtime self-registration, so duplicates would be caught at compile time if the map had duplicate literal keys (Go compiler error), or by tests if a typo was made in registry.go.

- **Alternatives considered**:
  - Panic in init() if duplicate detected: Incompatible with static map literal; not idiomatic.
  - Silently overwrite if duplicate: Violates spec edge case; unacceptable.

---

### Decision 6: Unknown Protocol-Name Handling

- **Decision**: At sentinel startup, when loading port configuration and attempting to look up each protocol name, if Lookup returns false (not found in registry), log a fatal error and refuse to start. The error message must clearly identify the unknown protocol and suggest checking the registry.

- **Rationale**:
  - Addresses FR-004: "The registry lookup MUST handle unknown protocol names gracefully and MUST NOT panic, crash, or silently succeed."
  - Early feedback: configuration errors are caught at startup, not at runtime when a connection arrives.
  - Per Assumption: "Sentinel's generic handler remains unchanged and outside the registry" — handleGeneric() is still available for wakeProtocol="generic" (which has no handshake-based classifier). Unknown protocol names don't fall back to generic; they are configuration errors.
  - No new failure mode introduced: current code already validates protocol names at startup (line 259 in sentinel/main.go).

- **Alternatives considered**:
  - Silent fallback to generic handler for unknown protocols: Confuses operators; violates the Assumption that generic stays outside the registry.
  - Return nil Classifier and handle nil in sentinel: Defers error to runtime (bad).

---

### Decision 7: Byte-for-Byte Equivalence Verification Strategy

- **Decision**: Use three parallel verification approaches:
  1. **Comparison tests** in gameproto_test.go: Run old Classify* and Build* facades side-by-side against identical handshake bytes (saved test fixtures), asserting that both return identical results (parsed fields, Consumed byte count, error/nil status).
  2. **E2E test revalidation** (no local execution): Run the full E2E suite against the refactored code on CI, confirming all tests that pass on main also pass on the refactored branch without modification.
  3. **Replay-contract spot-check**: In E2E, exercise wake-on-connect for both Minecraft and Terraria, verifying that downstream servers receive intact connection streams (proof that Consumed replay works identically).

- **Rationale**:
  - Addresses FR-005: "Byte-for-byte equivalence of outputs is the standard of correctness."
  - Addresses User Story 2 Independent Test: "A comparison test running the old and refactored versions on identical inputs."
  - CI is the sole verification venue (Assumption); no local builds/tests are run.
  - Three layers of evidence: unit-level byte comparison, integration (E2E unchanged), and functional (replay works).
  - The spec already names three E2E tests that will revalidate this: TestGameServer_WakeOnConnect_PingDoesNotWake, TestGameServer_WakeOnConnect_LoginWakes, TestGameServer_WakeOnConnect_UnarmedNoSentinel (in bot-fast bucket, executed by e2e-game-bot CI job).

- **Alternatives considered**:
  - Only E2E testing: Slower feedback if a regression exists; doesn't isolate which protocol is wrong.
  - Only unit comparison tests: Misses integration-level issues (e.g., bufio.Reader handling).

---

### Decision 8: Coverage Maintenance Strategy

- **Decision**: 
  - gameproto remains at 90% line coverage (enforced by .testcoverage.yml).
  - sentinel remains at 70% line coverage (enforced by .testcoverage.yml).
  - Code currently covered in sentinel/main.go (handleMinecraft, handleTerraria, ~90 lines) will shift coverage from sentinel to gameproto when those handlers are replaced with a generic registry-based dispatcher. The refactoring must ensure:
    - New Classifier interface methods in gameproto have test coverage (table-driven tests for Classify, SupportsStatusPing, BuildStatusResponse, BuildDisconnect).
    - Sentinel's new registry-lookup + dispatcher code has test coverage (e.g., TestRegistryLookup, TestDispatchUnknownProtocol).
    - Old per-protocol handlers (handleMinecraft, handleTerraria) can be deleted; new sentinel tests focus on the unified handleRegistryProtocol or equivalent.

- **Rationale**:
  - The refactoring moves ~90 lines of covered sentinel code (per-protocol handlers) into gameproto (Classifier implementations). This will increase gameproto's line count and coverage numerator; gameproto's denominator (total lines) increases due to new registry + interface definitions. The net effect on coverage ratio must be verified by CI.
  - Addresses SC-003: "Coverage thresholds are maintained."
  - Existing test structure (table-driven tests in minecraft_test.go, terraria_test.go) continues to validate the same behavior, just through the Classifier interface instead of facade functions.

- **Alternatives considered**:
  - Lower coverage gates during refactor: Violates SC-003; no exception to quality gates.
  - Keep old facade functions alongside Classifier: Violates goal of eliminating duplication.

---

### Decision 9: Transport (TCP/UDP) Neutrality of the Registry

- **Decision**: The registry is transport-agnostic. Classifiers are registered by protocol name only (e.g., "minecraft", "terraria"), not by transport. Sentinel's TCP dispatch uses the registry; UDP dispatch continues to use handleGeneric (no handshake-based classifier).

- **Rationale**:
  - Current code separates TCP and UDP (lines 391–398 in sentinel/main.go); UDP is orthogonal.
  - Assumption: "Sentinel's generic handler remains unchanged and outside the registry."
  - Future-proofing: a protocol could theoretically be registered for both TCP and UDP if both made sense, but that's a follow-up.
  - Simplifies this refactor: only consolidates TCP handshake-based classification.

- **Alternatives considered**:
  - Register each protocol twice (e.g., "minecraft-tcp", "minecraft-udp"): Overcomplicates the registry; TCP/UDP is orthogonal to protocol name.
  - Fold UDP heuristic into the registry: Out of scope; violates Assumption.

---

### Decision 10: Third Protocol Demo/Stub for Independent Test

- **Decision**: As part of this refactor, add a minimal third protocol (a test stub or real protocol) to user.story.1's independent test. This protocol must:
  - Implement the Classifier interface completely.
  - Be registered in gameproto/registry.go with one line.
  - Require zero edits to sentinel/main.go or gameproto/gameproto.go.
  - Work end-to-end: sentinel can dispatch to it and handle connections without errors.
  
  The stub can be a simple no-op classifier (e.g., always returns Unknown) or a minimal real protocol (e.g., a stub Factorio or Valheim detector). It is **test-only** — not deployed to production or exposed in the Helm chart.

- **Rationale**:
  - Addresses User Story 1 Independent Test: "A test or demo implementation of a third protocol proves the pattern works by showing zero edits required to gameproto/gameproto.go or sentinel/main.go."
  - Validates the registry pattern works for adding a protocol.
  - Catches any hidden assumptions in the interface (e.g., protocol-specific encoding that doesn't generalize).
  - Test-only scope keeps the refactor focused; adding a real game protocol is a follow-up.

- **Alternatives considered**:
  - No third protocol: Leaves the pattern unvalidated; User Story 1 acceptance scenario #3 cannot be tested.
  - Real game protocol (e.g., Factorio): Out of scope for this refactor; a separate effort.

---

## Behavior-Preservation Constraints (verified against sentinel/main.go)

The following six invariants enforce FR-005/SC-007 (byte-for-byte behavior preservation, zero silent failures) and must be checked by any code reviewer against the refactored diff:

1. **Protocol names remain strings, not enums**: The wakeProtocol dispatch must stay a string-based switch statement (sentinel/main.go:479-487). Registry lookup cannot change protocol names to enum values or symbolic keys without rewriting the dispatch logic that currently compares `port.WakeProtocol` against string literals like `"minecraft"`, `"terraria"`, `"generic"`.

2. **Startup validation remains strict and fatal**: Unknown protocol names must be rejected at startup via parsePortsConfig() (sentinel/main.go:259-260), with errors propagated fatally through loadConfig() to main (sentinel/main.go:88-90). Configuration errors are never deferred to runtime; they must fail fast before any listener binds.

3. **Handshake replay is verbatim**: Bytes returned by each Classifier's Classify() method in the ClassificationResult.Consumed field must be replayed exactly to the upstream connection (sentinel/main.go:632-637). No modification, truncation, or re-parsing is permitted; the replay contract depends on byte-for-byte fidelity.

4. **Pipelined bytes are preserved**: The same *bufio.Reader passed to Classify() (sentinel/main.go:500) must be reused by proxyBidirectional() (sentinel/main.go:639, 683-689) in its io.Copy call. Any buffering strategy that drops or re-reads bytes from the connection breaks the protocol stream.

5. **TCP/UDP split is structural**: TCP dispatch via handleTCPConnection uses Classifier.Classify() methods (sentinel/main.go:479-487, via handleMinecraft/handleTerraria). UDP dispatch via serveUDP uses only the packet-counting heuristic (sentinel/main.go:727-749) and must never call gameproto classifiers. This split is architectural, not a runtime choice.

6. **All initialization happens at startup**: The PORTS_CONFIG environment variable is parsed once in loadConfig()→parsePortsConfig() (sentinel/main.go:168-172) before any listener binds. By the time handleTCPConnection is called on an inbound connection, port.WakeProtocol is already resolved and the dispatch decision is deterministic. No lazy initialization, no runtime protocol registration.

---

## Resolved Unknowns

The spec marked two questions as resolved by Assumptions; they remain closed:

1. **Should the generic (heuristic) fallback also move into the registry?**
   - **Closed by Assumption**: No, not in this refactor. Sentinel's generic handler remains unchanged and outside the registry. It is a materially different detection mechanism (statistical heuristic vs. handshake parsing); folding it in now would risk distorting the abstraction. This is a candidate follow-up, not required here.

2. **How should the unified result represent detail that does not apply to a given protocol or outcome?**
   - **Closed by Assumption**: It is simply absent (nil) rather than an error. ClassificationResult.Detail is nil for Unknown classification or any outcome where the protocol has no relevant detail to return. Callers check for presence before acting on detail.

---

## Constraints from the Codebase

### Coverage Thresholds (Enforced by CI)

| Module | Threshold | Tool | File |
|--------|-----------|------|------|
| gameproto | 90% line | .testcoverage.yml | `gameproto/.testcoverage.yml:13` |
| sentinel | 70% line | .testcoverage.yml | `sentinel/.testcoverage.yml:15` |

### Go Version and Module Configuration

| Property | Value | Source |
|----------|-------|--------|
| Go version | 1.26.0 | gameproto/go.mod:3, sentinel/go.mod:3 |
| Workspace | 12 modules (includes gameproto, sentinel) | go.work:1-17 |
| Local replace | sentinel → ../gameproto | sentinel/go.mod:11-13 |

### Linting Configuration

| Setting | Value | Source |
|---------|-------|--------|
| Active linters | 14 enabled | .golangci.yml:7-23 (bodyclose, errcheck, gosec, govet, ineffassign, staticcheck, unused, misspell, revive, unparam, nilerr, noctx, errorlint, contextcheck) |
| Test file exemptions | errcheck, gosec, unparam | .golangci.yml:35-39 |
| Authorized suppression (gameproto) | G115 (uint32→int32 cast for Minecraft VarInt) in minecraft.go | .golangci.yml:47-52 |
| Zero inline suppressions | Entire codebase | Project rule (CLAUDE.md rule 4) |

### E2E Test Suite (Frozen by FR-006)

| Test Name | Bucket | CI Job | Platform |
|-----------|--------|--------|----------|
| TestGameServer_WakeOnConnect_PingDoesNotWake | bot-fast | e2e-game-bot | amd64 only |
| TestGameServer_WakeOnConnect_LoginWakes | bot-fast | e2e-game-bot | amd64 only |
| TestGameServer_WakeOnConnect_UnarmedNoSentinel | bot-fast | e2e-game-bot | amd64 only |
| TestGameServer_MinecraftJavaBot_Joined | bot-fast | e2e-game-bot | amd64 only |
| TestGameServer_TerrariaBot_Joined | bot-fast | e2e-game-bot | amd64 only |

**Source**: test/e2e/buckets.sh:128-135 (bot-fast bucket); lines 132-134 list the three wake-on-connect tests.

### Sentinel Protocol Configuration

| Item | Value | Source |
|------|-------|--------|
| Valid wakeProtocol values | minecraft, terraria, generic, none | sentinel/main.go:259-260 (validation); "none" disables listening |
| Validation location | parsePortsConfig() at startup | sentinel/main.go:230-271 |
| Config format | "port:protocol:wakeProtocol" triplets (CSV) | sentinel/main.go:243-246 |
| Hardcoded dispatch | 3-way switch on wakeProtocol (TCP only) | sentinel/main.go:479-487 |

### Current Facade Functions (To Be Refactored)

| Function | Module | Handles | Source |
|----------|--------|---------|--------|
| ClassifyMinecraft() | gameproto | Minecraft handshake → Kind + MinecraftClassifyResult | gameproto/gameproto.go:95-97 |
| ClassifyTerraria() | gameproto | Terraria ConnectRequest → Kind + TerrariaClassifyResult | gameproto/gameproto.go:114-116 |
| BuildMinecraftStatusResponse() | gameproto | JSON payload → Status packet bytes | gameproto/gameproto.go:121-123 |
| BuildMinecraftLoginDisconnect() | gameproto | Reason string → Login Disconnect packet bytes | gameproto/gameproto.go:127-129 |
| BuildTerrariaDisconnect() | gameproto | Reason string → Terraria Disconnect message bytes | gameproto/gameproto.go:133-135 |
| handleMinecraft() | sentinel | TCP Minecraft handshake dispatch + Status reply + Join wake | sentinel/main.go:499-530 |
| handleTerraria() | sentinel | TCP Terraria handshake dispatch + Join wake | sentinel/main.go:544-567 |
| handleGeneric() | sentinel | TCP generic (no handshake) → Join wake | sentinel/main.go:582-591 |
| bounceMinecraft() | sentinel | Send Minecraft Login Disconnect on deadline | sentinel/main.go:532-539 |
| bounceTerraria() | sentinel | Send Terraria Disconnect on deadline | sentinel/main.go:569-576 |

### Current ClassificationResult Types (To Be Unified)

| Type | Fields | Source |
|------|--------|--------|
| MinecraftClassifyResult | ProtocolVersion int32, NextState int32, ServerAddr string, Consumed []byte | gameproto/gameproto.go:71-86 |
| TerrariaClassifyResult | Version string, Consumed []byte | gameproto/gameproto.go:99-107 |

### Specification Contracts (To Be Created)

| File | Required by | Status |
|------|-------------|--------|
| contracts/exclusion-policy.md | SC-004 (Coverage gates and linting) | Does not exist; must document any authorized .golangci.yml suppression entries (currently only G115 for Minecraft VarInt) |

---

## Next Steps (Phase 1)

Once these research decisions are approved, Phase 1 will produce:

1. **data-model.md**: Detailed data structures (Classifier interface, ClassificationResult, Detail interface, registry map literal).
2. **quickstart.md**: Walkthrough of adding a new protocol (step-by-step).
3. **contracts/exclusion-policy.md**: Authorization for the existing G115 suppression; confirmation that no new suppressions are added.
4. **contracts/byte-equivalence-tests.md**: Specification of comparison tests (old facades vs. new Classifier, identical inputs).

---

## Notes for Implementation

- **No code changes in Phase 0**: This document is research and planning only.
- **Dossier verification**: All facts in the research dossier (gameproto module mapping, sentinel dispatch, coverage gates, test names) have been verified against source code.
- **Precedent cited**: API's Mount* pattern (api/cmd/main.go:237–281) establishes explicit registration as idiomatic in this codebase.
- **Independence**: This refactor is independent of PR #244 and later work; it begins on a clean main branch.

---

**End of Research**
