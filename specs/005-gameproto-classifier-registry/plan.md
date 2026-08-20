# Implementation Plan: Protocol Classifier Registry Refactor

**Branch**: `005-gameproto-classifier-registry` | **Date**: 2026-08-20 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/005-gameproto-classifier-registry/spec.md`

## Summary

Refactor the `gameproto/` Go module from per-protocol facade functions to a registry-based Classifier abstraction. Currently, adding a new game protocol requires scattered edits across four locations: a new implementation file, facade functions and result struct in `gameproto/gameproto.go`, a dispatch switch in `sentinel/main.go`, and near-duplicate handler functions. After this refactor, adding a protocol requires exactly one new implementation file and one registry entry with zero edits to shared code. The technical approach (from research.md) uses an explicit compile-time map literal in `gameproto/registry.go` containing Classifier instances, a unified ClassificationResult type with a Detail interface for protocol-specific metadata, and a four-method Classifier interface (Classify, SupportsStatusPing, BuildStatusResponse, BuildDisconnect) to encapsulate all protocol-specific logic. Byte-for-byte equivalence for Minecraft and Terraria handshake parsing, response building, and stream-replay semantics is a hard constraint; all existing E2E tests must pass unchanged.

## Technical Context

**Language/Version**: Go 1.26.0 (verified from `gameproto/go.mod:3` and `sentinel/go.mod:3`; discrepancy noted: constitution.md states Go 1.25, but actual codebase uses 1.26.0)

**Primary Dependencies**: 
- stdlib only (bufio, encoding/binary, fmt, io, errors, sync)
- sentinel imports gameproto as `github.com/ValgulNecron/gameplane/gameproto` via `go.work` workspace

**Storage**: N/A (pure parsing and classification, no persistence)

**Testing**: 
- `go test` across gameproto/ and sentinel/ modules
- Coverage gates: gameproto/.testcoverage.yml at 90% line coverage, sentinel/.testcoverage.yml at 70% line coverage (enforced by CI via go-test-coverage/v2)
- E2E tier: `test/e2e/buckets.sh` bot-fast bucket includes three wake-on-connect tests (TestGameServer_WakeOnConnect_PingDoesNotWake, TestGameServer_WakeOnConnect_LoginWakes, TestGameServer_WakeOnConnect_UnarmedNoSentinel) that validate behavior preservation

**Target Platform**: Linux, amd64 (standard Kubernetes/sentinel deployment)

**Project Type**: Go library module (gameproto/) + sidecar-style component (sentinel/) using that library

**Performance Goals**: Per-connection handshake parsing on the sentinel's hot path; must parse incoming TCP handshakes with zero allocation waste and preserve the exact byte-stream semantics required for lossless replay to upstream servers (Consumed field correctness is load-bearing)

**Constraints**:
- Behavior-preserving refactor: identical handshake parsing, response building, error handling, and Consumed field semantics before and after
- Coverage thresholds must be maintained: gameproto 90%, sentinel 70%
- Zero in-source suppression directives allowed (no `//nolint`, `//#nosec` in new code); existing authorized G115 suppression in minecraft.go is retained
- E2E test names in test/e2e/buckets.sh are frozen (FR-006); test suite must pass without modification
- Blast-radius limit (FR-009): changes confined to gameproto/ and sentinel/ components only; no CRD types, operator reconciliation, database schema, or other components affected

**Scale/Scope**: 2 protocols currently supported (Minecraft, Terraria); registry pattern must scale to ~16 game modules; a third protocol stub is added during this refactor as a proof that the pattern works (test-only, not production)

## Constitution Check

*GATE: Principles I, III, IV, V, VI verified below; Principle II explicitly exempt.*

| Principle | Status | Justification |
|-----------|--------|---------------|
| **I. E2E-Tested Delivery** | PASS | Existing E2E wake-on-connect tests (test/e2e/buckets.sh, bot-fast bucket, three tests) gate this refactor: all must pass unchanged on the refactored code, proving behavior preservation. Comparison tests in gameproto_test.go (old facades vs. new Classifier on identical handshake bytes) provide unit-level verification. No new E2E tests are added; existing frozen suite is revalidated. |
| **II. Design-First** | EXEMPT | No dashboard or website UI surface in this refactor. gameproto/ is pure protocol parsing (Go library), sentinel/ is a daemon component. No Pencil design pass required. |
| **III. Language & Ecosystem** | PASS | Go 1.26.0 throughout. Code wraps errors with `%w` per CLAUDE.md rule 6. No `//nolint` or `//#nosec` directives introduced; existing G115 suppression for Minecraft VarInt cast (minecraft.go) is retained. golangci-lint gate applied to both modules (14 active linters per .golangci.yml). Zero in-source suppressions maintained per Principle. |
| **IV. Spec-Driven Development** | PASS with Note | This plan flows from spec.md (feature spec) and research.md (Phase 0 decisions); Phase 1 will produce data-model.md, quickstart.md, and contracts/exclusion-policy.md. **Note**: Per Principle IV, both `gameproto/specs.md` and `sentinel/specs.md` must be updated in the same change documenting the behavior this refactor preserves. gameproto/specs.md must document the Classifier abstraction, ClassificationResult, Detail interface, and registry contract. sentinel/specs.md must document the registry-based dispatch mechanism and its integration with the per-bucket E2E tests. These documentation updates are mandatory and in-scope. |
| **V. Delegate to Workflows** | PASS | Work is decomposed into independent units (Classifier interface design, concrete implementations for Minecraft and Terraria, registry map literal, sentinel dispatcher rewrite, test coverage for each) and delegated to subagents per tier. Phase 1 design is reviewed at tier+1 before implementation begins. Execution runs through a Workflow (non-blocking), not ad hoc blocking calls. |
| **VI. CI Bears Heavy Lifting** | PASS | No local builds, tests, linting, or E2E runs. All verification happens on GitHub Actions CI via `make build-go`, `make test-go`, `make lint-go`, and `make test-e2e-bucket BUCKET=bot-fast` on pushed branch. First CI run is oracle for soundness. |

## Project Structure

### Documentation (this feature)

```text
specs/005-gameproto-classifier-registry/
├── spec.md                    # Feature specification (user stories, requirements, success criteria)
├── research.md                # Phase 0 research output (decisions 1–10, constraints, verification strategy)
├── plan.md                    # This file (Phase 0 summary → Phase 1 inputs)
├── data-model.md              # Phase 1: Detailed Classifier interface, ClassificationResult, Detail, registry structure
├── quickstart.md              # Phase 1: Step-by-step walkthrough of adding a new protocol (the proof pattern)
├── contracts/
│   └── exclusion-policy.md    # Phase 1: Authorization of existing G115 suppression; confirmation no new suppressions added
└── tasks.md                   # Phase 2: Breakdown of implementation work (not created by this plan)
```

### Source Code (repository root)

**Before refactor** (current state):

```text
gameproto/
├── go.mod                                    # Go 1.26.0
├── go.sum
├── gameproto.go                              # Facade functions: ClassifyMinecraft, ClassifyTerraria, BuildMinecraftStatusResponse, BuildMinecraftLoginDisconnect, BuildTerrariaDisconnect
├── minecraft.go                              # Minecraft handshake parser, BuildMinecraftStatusResponse helper, BuildMinecraftLoginDisconnect helper
├── minecraft_test.go                         # Table-driven tests for Minecraft classification
├── terraria.go                               # Terraria ConnectRequest parser, BuildTerrariaDisconnect helper
├── terraria_test.go                          # Table-driven tests for Terraria classification
├── .testcoverage.yml                         # Gate: 90% line coverage
└── [no registry.go, no unified Classifier interface]

sentinel/
├── go.mod                                    # Go 1.26.0, replace github.com/ValgulNecron/gameplane/gameproto => ../gameproto
├── go.sum
├── main.go                                   # Entry point with hardcoded 3-way switch (minecraft, terraria, generic) and per-protocol handlers (handleMinecraft, handleTerraria, handleGeneric, bounceMinecraft, bounceTerraria, ~90 lines per-protocol logic)
├── .testcoverage.yml                         # Gate: 70% line coverage
└── [handler functions scattered across main.go]

test/e2e/
└── buckets.sh                                # Three wake-on-connect tests in bot-fast bucket (names frozen by FR-006)
```

**After refactor** (target state):

```text
gameproto/
├── go.mod                                    # Go 1.26.0 (unchanged)
├── go.sum
├── gameproto.go                              # Removed: facade functions (ClassifyMinecraft, ClassifyTerraria, etc.). Retained: Helper types, utilities, error definitions.
├── classifier.go                             # NEW: Classifier interface (Classify, SupportsStatusPing, BuildStatusResponse, BuildDisconnect), Kind enum (Join, Status, Unknown), ClassificationResult struct, Detail interface
├── registry.go                               # NEW: Protocol registry map literal with Minecraft and Terraria instances; Lookup(name string) function
├── minecraft.go                              # Modified: MinecraftClassifier struct implementing Classifier; old parsing logic moved into Classify method; BuildStatusResponse and BuildDisconnect become Classifier methods
├── minecraft_test.go                         # Modified: Tests refactored to use Classifier interface; old facade tests converted to Classifier tests; comparison tests added (old vs. new on identical bytes)
├── terraria.go                               # Modified: TerrariaClassifier struct implementing Classifier; old parsing logic moved into Classify method; BuildDisconnect becomes Classifier method
├── terraria_test.go                          # Modified: Tests refactored for Classifier interface; comparison tests added
├── demo.go                                   # NEW: DemoClassifier (test-only, minimal stub protocol proving pattern works)
├── gameproto_test.go                         # MODIFIED: Currently tests the facade-function API (ClassifyMinecraft, ClassifyTerraria, etc.). Will be refactored to test Classifier interface; comparison tests added (old vs. new on identical bytes). Existing TestKindString retained.
├── specs.md                                  # MODIFIED: Currently documents the facade functions and per-protocol behavior. Will be updated to document Classifier abstraction, ClassificationResult, Detail interface, registry contract, and integration with sentinel. Required by Principle IV.
├── .testcoverage.yml                         # Gate: 90% line coverage (unchanged threshold, new code covered via Classifier interface tests)
└── .golangci.yml                             # No changes; existing G115 suppression retained; no new suppressions

sentinel/
├── go.mod                                    # Go 1.26.0 (unchanged)
├── go.sum
├── main.go                                   # Modified: parsePortsConfig validates protocol name against registry at startup (fails fast if unknown). New handleRegistryProtocol function (generic dispatcher using Classifier from registry). Old handleMinecraft, handleTerraria, bounceMinecraft, bounceTerraria functions DELETED (~90 lines removed, replaced with ~40 lines of generic dispatch + registry lookup). TCP dispatch switch simplified from 3-way hardcoded switch to registry.Lookup + handleRegistryProtocol. UDP dispatch (handleGeneric) unchanged, outside registry per Assumption.
├── specs.md                                  # MODIFIED: Currently documents the hardcoded per-protocol handler functions (handleMinecraft, handleTerraria, handleGeneric). Will be updated to document registry-based dispatch mechanism, startup validation, integration with gameproto registry, and per-bucket E2E test dependencies. Required by Principle IV.
├── .testcoverage.yml                         # Gate: 70% line coverage (unchanged threshold; new tests for registry dispatch, old per-protocol handler tests deleted)
└── main_test.go                              # NEW or MODIFIED: Tests for registry-based dispatcher (e.g., TestRegistryLookup, TestDispatchUnknownProtocol, TestHandleRegistryProtocol)

test/e2e/
└── buckets.sh                                # Frozen (FR-006): no changes to test names or bucket assignments. Three wake-on-connect tests remain in bot-fast bucket.
```

**Structure Decision**: This refactor is confined to two Go modules (gameproto and sentinel) and the documentation tree (specs/005-gameproto-classifier-registry/). The change is localized: shared-code edits are consolidated into a single registry manifest (gameproto/registry.go) and a single startup validation (sentinel/main.go parsePortsConfig), eliminating the multi-file edit burden. The Classifier interface provides extensibility: future protocols implement one interface rather than editing shared code. Per Principle IV, both gameproto/specs.md and sentinel/specs.md must be added as behavior documentation, ensuring the abstraction and its contract are captured for future maintainers. No external components (operator, API, web, CRDs) are touched; this is a pure internal refactor with external verification through E2E tests.

## Complexity Tracking

No violations — table intentionally empty. All six applicable constitution principles (I, III, IV, V, VI) are satisfied. Principle II is exempt (no UI surface). One discrepancy is flagged for transparency: constitution.md states Go 1.25, but the actual codebase (gameproto/go.mod, sentinel/go.mod) uses Go 1.26.0; this plan uses the actual version from the source files as the authoritative value.
