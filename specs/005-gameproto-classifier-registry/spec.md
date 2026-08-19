# Feature Specification: Protocol Classifier Registry Refactor

**Feature Branch**: `005-gameproto-classifier-registry`

**Created**: 2026-08-20

**Status**: Draft

**Input**: Refactor `gameproto/` Go module from per-protocol facade functions to a registry-based Classifier pattern, eliminating code duplication and shared-file edits when adding new game protocols.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Adding a new game protocol requires no edits to shared code (Priority: P1)

Today, when support for a new game's wire protocol is added to the system, four separate
locations must be modified: a new implementation file in `gameproto/`, new facade functions
and a result struct in `gameproto/gameproto.go`, a new `case` in the protocol dispatch switch
in `sentinel/main.go`, and a ~45-line near-duplicate handler function also in sentinel. This
scattered, multi-file edit pattern creates friction, makes pull reviews fragmented across
unrelated concerns, and forces the maintainer to coordinate changes across the shared
`gameproto.go` and `sentinel/main.go` files every time a protocol is added. At scale (16 games
or more), this becomes roughly 16 result structs, 40 facade wrappers, and 700 lines of
near-duplicate sentinel handlers — a maintenance burden that grows linearly with the protocol
count.

After this refactor, a developer adding protocol #N should need to modify exactly one new file
(the protocol implementation) and register it in exactly one place (a central registry), with
zero edits required to any existing shared-code file. The protocol is immediately available
for use without touching `gameproto/gameproto.go`, `sentinel/main.go`, or any other shared
surface.

**Why this priority**: Eliminating shared-code edits is the core value of the refactor. At
16+ protocols, the current linear-scaling maintenance cost becomes unsustainable and diverts
effort from feature development to mechanical boilerplate. This is foundational to making
protocol addition a scalable operation.

**Independent Test**: Can be tested independently by verifying that a new protocol implementation
can be added with a single new file and a single registry entry, and that the sentinel component
can dispatch to it without any changes to existing shared source code. A test or demo implementation
of a third protocol (e.g., a minimal stub) proves the pattern works by showing zero edits required
to `gameproto/gameproto.go` or `sentinel/main.go`.

**Acceptance Scenarios**:

1. **Given** the refactored `gameproto/` module with a Classifier registry in place, **When**
   a developer creates a new file implementing a complete protocol (e.g.,
   `gameproto/factorio.go`) and registers it with the protocol registry through the registry's
   standard registration mechanism, **Then** the sentinel can dispatch to and use that protocol
   without any changes to the sentinel's dispatch logic or handler code.
2. **Given** the same setup, **When** code review examines the diff for the new protocol, **Then**
   the diff is entirely contained in the new file and a single-line registry entry, with no
   changes to `gameproto/gameproto.go`, `sentinel/main.go`, or any other existing shared file.
3. **Given** a third new protocol added after the first, **When** it follows the same one-file,
   one-entry pattern, **Then** the two new protocols coexist with the original Minecraft and
   Terraria implementations without any conflicts or further edits to shared code.

---

### User Story 2 - Behavior for all currently supported protocols is preserved exactly (Priority: P1)

The refactored code must behave identically to the current code for Minecraft and Terraria:
same handshake parsing, same response building, same `Consumed` field correctness for stream
replay, same reconnect handling, same error paths. The wake-on-connect flow — which relies on
the `Consumed` bytes to forward an intact connection stream to the real upstream server — must
work identically before and after the refactor. All existing E2E tests must pass without
modification, and all existing game servers must continue to operate normally when the refactored
code is deployed.

This is a pure refactor: no logic changes, no new features, no behavior divergence for existing
code paths.

**Why this priority**: A refactor that changes observable behavior is a rewrite disguised as
a cleanup. Stakeholders, operators, and E2E tests depend on the current behavior; breaking that
trust, even temporarily, undermines the refactoring's safety. This must be verified as a
stand-alone priority, not bundled into other stories.

**Independent Test**: Can be tested independently by running the full E2E suite against the
refactored code and confirming that all tests that pass on `main` also pass on the refactored
branch without modification, including the wake-on-connect integration tests. A comparison
test (running the old implementation's handshake parser and response builder side-by-side with
the refactored version on identical inputs) can verify byte-for-byte equivalence of outputs
before the refactoring ships.

**Acceptance Scenarios**:

1. **Given** the old gameproto facades and the refactored Classifier-based code operating on
   the same Minecraft handshake bytes, **When** both are asked to classify the connection,
   **Then** they produce identical classification results (parsed fields and `Consumed` byte
   count).
2. **Given** the same setup for Terraria, **When** both implementations are asked to build a
   disconnect message, **Then** they produce byte-for-byte identical message payloads.
3. **Given** the full E2E suite running against a live Gameplane cluster with the refactored
   code, **When** the suite exercises wake-on-connect for both Minecraft and Terraria servers,
   **Then** all tests pass without modification and downstream servers receive the identical
   connection stream (replay works identically).
4. **Given** a production Gameplane cluster running the old code, **When** it is upgraded to
   the refactored code, **Then** all active GameServers continue to operate normally and no
   change in behavior is observable to end users.

---

### User Story 3 - The registry mechanism provides visibility and prevents accidental omission (Priority: P2)

A developer or maintainer must be able to inspect the current system and immediately understand
which protocols are registered, which are available for use, and what the mechanism for
registration is. The registry itself must be self-documenting and maintainable, so that a
missing protocol is detectable by review and the set of available protocols is not a mystery
buried in distributed code.

**Why this priority**: As the protocol count grows, the registry becomes a critical piece of
system documentation. If protocol registration is opaque, invisible, or scattered, maintainers
cannot quickly tell whether a new protocol is actually available or diagnose why a deployment
cannot use it. This is a quality-of-life and debugging feature that becomes more valuable as
the system scales.

**Independent Test**: Can be tested independently by confirming that a maintainer reading the
registry code (and no other code) can enumerate all currently registered protocols and
understand the registration pattern well enough to add a new one. A documentation search
(grep for registry registration calls) must locate all active protocol registrations in the
codebase, with no registrations hidden or duplicated.

**Acceptance Scenarios**:

1. **Given** a maintainer examining the registry structure and all registration callsites,
   **When** they search for protocol registrations in the codebase, **Then** they find a
   registration for every currently supported protocol (Minecraft and Terraria, plus any
   demonstration protocol added solely to validate the pattern), with no orphaned or duplicate
   entries.
2. **Given** the same registry structure, **When** a code reviewer inspects a PR that adds a
   new protocol, **Then** they can verify the registration is present and correctly formatted
   without needing to read the implementation code.
3. **Given** a hypothetical future where a protocol is accidentally removed from the registry,
   **When** a maintainer runs a audit check (e.g., a test that verifies all expected protocols
   are present), **Then** the check fails and clearly identifies which protocol is missing.

---

### User Story 4 - Coverage gates and linting properties are maintained (Priority: P2)

The refactored code must continue to pass the existing coverage gates (`gameproto/.testcoverage.yml`
at 90%, `sentinel/.testcoverage.yml` at 70%) and the golangci-lint gate applied to both modules.
No new suppression directives (`//nolint`, `//#nosec`, etc.) are introduced, and the
zero-suppression property of the codebase is preserved.

**Why this priority**: Coverage gates and linting are non-negotiable quality signals in this
project. A refactor that allows them to slip is a regression, not an improvement. This must be
verified as part of the completion criteria.

**Independent Test**: Can be tested independently by confirming that `make test-go` passes the
coverage check for `gameproto/` and `sentinel/`, that golangci-lint produces no findings in
either module, and that a search for suppression directives across both modules (excluding the
authorized `.golangci.yml` entries in `contracts/exclusion-policy.md`) returns zero matches.

**Acceptance Scenarios**:

1. **Given** the refactored code in `gameproto/` and `sentinel/`, **When** coverage is
   calculated, **Then** `gameproto/` maintains 90% line coverage and `sentinel/` maintains 70%
   line coverage.
2. **Given** the same code running through golangci-lint with the project's standard
   configuration, **When** the lint job completes, **Then** zero findings are reported for
   either module.
3. **Given** a search for suppression directives (`//nolint`, `//#nosec`, etc.) across the
   refactored code, **When** the search completes, **Then** zero inline suppressions are found
   in either module.

---

### Edge Cases

- **What happens when a protocol-specific handler receives a connection that does not match
  that protocol's handshake?** The handler must classify it correctly as `Unknown` and take
  the same action as the current code (e.g., forward to generic handler or close). The
  classification must not panic or return an error that silently loses the connection.
- **How does the refactored system handle Terraria's asymmetry: it has no out-of-band status
  ping, so any recognized ConnectRequest is a Join?** This must be a first-class case in the
  Classifier abstraction, not an error or a panic. A protocol that does not support status
  pings must be able to declare that cleanly, and the sentinel must handle it without
  attempting to generate a fake status response.
- **What happens when a protocol name is unregistered or unknown at runtime?** The sentinel must
  fall back to the generic handler (packets-in-window heuristic) or reject the configuration at
  startup with a clear error message, just as the current code does. The refactor must not
  introduce a new failure mode.
- **What if a protocol's handshake parsing fails mid-stream (incomplete packet, malformed bytes)?**
  The classification must return an Unknown result without consuming bytes it cannot parse, so
  the stream can be replayed to a generic handler or the real server. The current behavior must
  be preserved.
- **Can two protocols register themselves with the same name?** The registry must enforce unique
  names; a duplicate registration attempt must either fail at startup or log a fatal error.
  Silently overwriting an earlier registration is not acceptable.
- **How does the refactor handle protocols that support both TCP and UDP?** The current code
  treats them as separate concerns (Minecraft uses TCP, generic handler has UDP support). The
  refactored registry must not break this; a protocol can register itself for either or both
  transport types without restriction.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: A single, uniform abstraction (the Classifier) MUST encapsulate all
  protocol-specific logic for classifying an incoming connection and generating its responses,
  so that adding a protocol means providing one implementation of that abstraction rather than
  editing shared dispatch code.
- **FR-002**: A single, unified result type MUST be defined for all protocol classifications,
  carrying the bytes consumed while parsing the handshake (needed for stream replay, see FR-005)
  plus protocol-specific detail where relevant (for example: Minecraft's protocol version,
  next-state, and server address, or Terraria's version string), and carrying no
  protocol-specific detail for a generic Unknown classification.
- **FR-003**: A protocol registry MUST exist and MUST support registration of new classifiers
  by game name, so that sentinel and other consumers can look up the appropriate classifier
  for a given protocol without a hardcoded dispatch switch.
- **FR-004**: The registry lookup MUST handle unknown protocol names gracefully (falling back
  to a generic classifier or reporting the unknown name clearly) and MUST NOT panic, crash, or
  silently succeed when a protocol is not found.
- **FR-005**: The refactored code MUST preserve the exact behavior of the current Minecraft and
  Terraria implementations for all handshake parsing, response building, and stream-replay
  semantics. Byte-for-byte equivalence of outputs is the standard of correctness.
- **FR-006**: The refactored code MUST maintain all existing E2E test compatibility, so that
  no changes to test names, test logic, or test invocation are required. The E2E suite names
  listed in `test/e2e/buckets.sh` MUST remain frozen and unchanged.
- **FR-007**: A protocol that does not support out-of-band status pings (like Terraria) MUST
  have a way to declare this in the Classifier abstraction so that the sentinel does not
  attempt to generate a status response. This MUST be a first-class case, not an error or a
  panic.
- **FR-008**: The refactored code MUST NOT introduce any new in-source suppression directives
  (`//nolint`, `//#nosec`, `//lint:ignore`, etc.). The zero-suppression property of the
  codebase MUST be preserved throughout this work.
- **FR-009**: This refactoring MUST be confined to the `gameproto/` module and the sentinel
  component's protocol dispatch. No CRD types, no operator reconciliation behavior, no database
  schema, and no component outside `gameproto/` and `sentinel/` (other than the E2E test names
  already frozen by FR-006) may change as a result of this work.

### Key Entities

- **Classifier**: An abstraction that encapsulates the protocol-specific logic for identifying
  and handling an incoming wire-protocol handshake. A Classifier can classify a connection
  (is this a join attempt, a status ping, or unknown?), build protocol-specific responses
  (status messages, disconnect messages), and indicate whether it supports out-of-band status
  pings. Each game protocol (Minecraft, Terraria, future protocols) has one corresponding
  Classifier implementation.
- **ClassificationResult**: A unified result type carrying the parsed handshake classification
  (join, status, unknown), the bytes consumed while parsing (so the untouched remainder of the
  stream can be forwarded intact), and protocol-specific detail (e.g., server address, protocol
  version, player name). All protocol classifications return this type; detail that does not
  apply to a given protocol or outcome (e.g., status-response detail for a protocol with no
  status concept) is simply absent, and callers check for its presence before using it.
- **Protocol Registry**: A central lookup structure, keyed by game name (string), that returns
  the Classifier for that protocol. The registry is populated at startup via registration
  functions, and the sentinel uses it to dispatch incoming connections without a hardcoded
  switch statement.
- **Sentinel Dispatcher**: The simplified connection handler in `sentinel/main.go` that (1)
  looks up the protocol name in the registry, (2) invokes the Classifier to parse the
  connection, and (3) handles the result uniformly regardless of the protocol. The current
  per-protocol `handleMinecraft()`, `handleTerraria()` functions are consolidated into a
  single generic flow that works for all protocols via the registry.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Adding a new game protocol requires changing exactly one new file and exactly one
  registry entry, with zero modifications to any existing shared-code file (gameproto/gameproto.go,
  sentinel/main.go, or any other).
- **SC-002**: All E2E tests pass on the refactored code without modification to test names,
  logic, or invocation, confirming behavior is identical for currently supported protocols
  (Minecraft and Terraria).
- **SC-003**: Coverage thresholds are maintained: `gameproto/` at 90% line coverage and
  `sentinel/` at 70%, verified by the CI coverage gate.
- **SC-004**: Zero in-source suppression directives exist in the refactored code. A search for
  `//nolint`, `//#nosec`, `//lint:ignore`, and equivalent patterns across both modules returns
  zero matches (excluding authorized config-level `.golangci.yml` entries in
  `contracts/exclusion-policy.md`).
- **SC-005**: The golangci-lint gate passes for both `gameproto/` and `sentinel/` with zero
  reported findings.
- **SC-006**: A code reviewer can inspect the protocol registry and enumerate all registered
  protocols without reading protocol implementation files. A grep search for registry
  registration callsites locates all active registrations with no orphaned or duplicate entries.
- **SC-007**: Stream replay (the `Consumed` bytes) behaves identically before and after the
  refactor, verified by comparison tests and E2E wake-on-connect integration tests.
- **SC-008**: The refactored Classifier can express "this protocol does not support status
  pings" as a first-class case (e.g., Terraria), without errors, panics, or workarounds in
  calling code.

## Assumptions

- **Behavior-preserving refactor**: This is a pure refactor of existing code; no new features
  are added, no existing behavior is changed, and no new error cases are introduced for
  currently supported protocols.
- **CI is the sole verification venue**: No builds, tests, linting, or E2E runs are executed
  locally during this work. All verification happens on GitHub Actions CI, per project
  conventions (Principle VI in `constitution.md`). The first CI run on a pushed branch is the
  oracle for whether the refactoring is sound.
- **E2E test names are frozen**: The names and structure of E2E tests in `test/e2e/buckets.sh`
  and the test files themselves are unchanged by this work. The set of running tests, their
  parameters, and their expectations are identical before and after.
- **This work lands after PR #244 merges**: The refactoring is independent of other in-flight
  work and begins on a clean main branch. No coordinated merges with other features are
  required, and the git history of this branch does not interleave with unrelated PRs.
- **Sentinel's generic handler remains unchanged and outside the registry**: The
  `handleGeneric()` function (the packets-in-window heuristic used for protocols that have no
  handshake-based classifier) is not refactored, wrapped in a Classifier, or added to the
  registry as part of this work; only the protocol-specific dispatch and the Minecraft/Terraria
  handlers are consolidated into the registry-based flow. Folding the generic path into the
  registry is a candidate follow-up, not required here, because it is a fundamentally different
  kind of detection (statistical heuristic vs. handshake parsing) and forcing it into the same
  abstraction now would risk distorting the abstraction to fit an outlier.
- **The registry is initialized at startup, not dynamically**: Protocols are registered once,
  through explicit registration calls executed at program startup, not dynamically at runtime
  or based on external configuration. This keeps the registry stable and predictable.
- **Classifier results may omit detail that does not apply**: when a protocol has no concept of
  a given operation (for example, Terraria has no status-ping response), the result simply
  carries no detail for that operation rather than the Classifier being asked to fabricate one
  or return an error. Callers check what is present before acting on it.
- **No changes to game modules, CRDs, or operator**: This refactor is strictly within the
  `gameproto/` Go module and the sentinel component; it does not touch the `modules/` submodule,
  any CRD types, or the operator's reconciliation logic.

---

## Open Questions and Clarifications

No unresolved clarifications remain. Two questions came up while drafting this spec and were
each closed with a clear, low-risk default rather than left open, so they are recorded here as
decisions instead of markers:

1. **Should the generic (heuristic) fallback also move into the registry?** No, not in this
   refactor — see "Sentinel's generic handler remains unchanged and outside the registry" under
   Assumptions. It is a materially different detection mechanism, and folding it in can be a
   follow-up once the registry pattern has proven itself for handshake-based protocols.
2. **How should the unified result represent detail that does not apply to a given protocol or
   outcome (e.g., a status response for a protocol with no status concept)?** It is simply
   absent rather than an error — see "Classifier results may omit detail that does not apply"
   under Assumptions.
