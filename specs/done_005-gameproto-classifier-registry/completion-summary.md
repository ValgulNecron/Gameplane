# Completion Summary: Protocol Classifier Registry Refactor

> **Superseded in part by PR #248.** The deprecated facades and equivalence test suite have been deleted. See "SUPERSEDED" sections below for corrections. The specification documents (gameproto/specs.md and sentinel/specs.md) remain accurate.

**PR**: #245 | **Merge Commit**: 51932528 | **Date**: 2026-08-20

---

## What Changed

### Before

The `gameproto` module exposed per-protocol facade functions in `gameproto/gameproto.go`:

- `ClassifyMinecraft(br *bufio.Reader) (Kind, *MinecraftClassifyResult, error)`
- `ClassifyTerraria(br *bufio.Reader) (Kind, *TerrariaClassifyResult, error)`
- `BuildMinecraftStatusResponse(jsonPayload string) ([]byte, error)`
- `BuildMinecraftLoginDisconnect(reason string) ([]byte, error)`
- `BuildTerrariaDisconnect(reason string) ([]byte, error)`

Result types were protocol-specific: `MinecraftClassifyResult` and `TerrariaClassifyResult`.

The `sentinel/main.go` dispatcher contained a hardcoded 3-way switch over protocol names and near-duplicate handler functions (`handleMinecraft`, `handleTerraria`, `handleGeneric`, `bounceMinecraft`, `bounceTerraria` — ~90 lines of protocol-specific logic spread across multiple functions). Adding a new protocol required edits to: (1) new implementation file, (2) facade functions in `gameproto.go`, (3) dispatch switch in `sentinel/main.go`, and (4) new handler functions in sentinel.

### After

The refactored structure uses a **Classifier registry pattern**:

**New files:**
- `gameproto/classifier.go` — defines the `Classifier` interface (4 methods: `Classify`, `SupportsStatusPing`, `BuildStatusResponse`, `BuildDisconnect`), the unified `ClassificationResult` struct, and the `Detail` interface for protocol-specific metadata.
- `gameproto/registry.go` — contains the compile-time registry map literal `classifierRegistry` with explicit entries for `"minecraft"`, `"terraria"`, and `"demo"` (test stub); exports `Lookup(name string) (Classifier, bool)` and `ListRegistered() []string`.
- `gameproto/demo.go` — a minimal reference implementation of the Classifier interface (test-only, always classifies as Unknown).
- New test files: `gameproto/classifier_golden_test.go` (SUPERSEDED: `classifier_equivalence_test.go` was deleted in PR #248), `gameproto/demo_test.go`, `gameproto/registry_test.go`, and extended `sentinel/main_test.go`.

**Modified files:**
- `gameproto/minecraft.go` — now defines `MinecraftClassifier` struct implementing the Classifier interface; wraps the existing unexported `classifyMinecraftHandshake` function inside `Classify()`.
- `gameproto/terraria.go` — now defines `TerrariaClassifier` struct implementing the Classifier interface; wraps the existing unexported `classifyTerrariaConnectRequest` function inside `Classify()`.
- `sentinel/main.go` — removes hardcoded handler functions (`handleMinecraft`, `handleTerraria`, `bounceMinecraft`, `bounceTerraria`); replaces them with a single unified `handleRegistryProtocol(ctx context.Context, conn net.Conn, w wakeRequester, upstreamAddr string, protocol string, deadline time.Duration)` function that uses `gameproto.Lookup()` to fetch the Classifier and dispatch through it. The TCP dispatch path now calls either `handleRegistryProtocol` (for registered protocols) or `handleGeneric` (for UDP fallback, intentionally outside the registry).
- `gameproto/gameproto.go` — (SUPERSEDED by PR #248) facade functions were initially retained (marked Deprecated) for equivalence testing, but have since been deleted as they are dead code in production. See "T081: Delete the Deprecated Facades (COMPLETED)" below.

**Real symbols (verified from merged code):**
- Interface: `gameproto.Classifier` (methods: `Classify`, `SupportsStatusPing`, `BuildStatusResponse`, `BuildDisconnect`)
- Unified result: `gameproto.ClassificationResult` (fields: `Kind`, `Consumed`, `Detail`)
- Registry: `gameproto.classifierRegistry` (package-level map literal; exported via `Lookup` and `ListRegistered`)
- Classifier implementations: `MinecraftClassifier`, `TerrariaClassifier`, `DemoClassifier` (all satisfy the Classifier interface)
- Sentinel dispatcher: `handleRegistryProtocol` in `sentinel/main.go`

---

## How to Add a New Protocol

### Concrete Steps

1. **Create a new implementation file** in `gameproto/<protocol>.go` (e.g., `gameproto/factorio.go`). Copy `gameproto/demo.go` as a template; its four methods show the signature contract.

2. **Implement the Classifier interface**:
   ```go
   type YourProtocolClassifier struct{}

   func (y *YourProtocolClassifier) Classify(br *bufio.Reader) (*ClassificationResult, error) {
       // Parse handshake; return Kind (Join/Status/Unknown), Consumed bytes, and optional Detail.
   }

   func (y *YourProtocolClassifier) SupportsStatusPing() bool {
       // Return true if protocol has out-of-band status pings (Minecraft), false if not (Terraria).
   }

   func (y *YourProtocolClassifier) BuildStatusResponse(payload string) ([]byte, error) {
       // Build a status-ping reply. Must return ErrStatusPingUnsupported if SupportsStatusPing() is false.
   }

   func (y *YourProtocolClassifier) BuildDisconnect(reason string) ([]byte, error) {
       // Build a disconnect/rejection message.
   }
   ```

3. **Register in gameproto/registry.go** (one line):
   ```go
   var classifierRegistry = map[string]Classifier{
       "minecraft": &MinecraftClassifier{},
       "terraria":  &TerrariaClassifier{},
       "yourprotocol": &YourProtocolClassifier{},  // ← Add here
   }
   ```

4. **That's it.** Sentinel automatically picks up the new protocol via `gameproto.Lookup()`. No changes to `sentinel/main.go` or `gameproto/gameproto.go` needed.

### Worked Template

See `gameproto/demo.go` (143 lines) for a reference implementation proving the pattern. The `DemoClassifier` shows:
- A struct with no fields (all logic is in the methods).
- Stubs for all four Classifier methods (intentionally minimal; real implementations add handshake parsing).
- Error handling (returns `ErrStatusPingUnsupported` when appropriate).
- The Consumed field pattern (returns nil when no bytes are read; actual implementations populate this for stream replay).

---

## What Did Not Change

- **Parsing logic**: All handshake parsing code (unexported functions like `classifyMinecraftHandshake`, `classifyTerrariaConnectRequest`, and their frame-building helpers) remains unchanged. Classifiers wrap these functions; the logic inside them is identical before and after.
- **Kind enum ordering**: The `Kind` enum (Join=0, Status=1, Unknown=2) is unchanged; ordinal values are preserved.
- **UDP fallback handler**: `sentinel/main.go`'s `handleGeneric` (TCP payload classification, treats any completed TCP connect as Join) is deliberately left outside the registry as a non-protocol, since UDP has no standard wire format and the handler is a Sentinel-specific fallback. This is an explicit architectural choice, not an oversight.
- **E2E test names**: All three wake-on-connect tests remain frozen (per FR-006):
  - `TestGameServer_WakeOnConnect_LoginWakes`
  - `TestGameServer_WakeOnConnect_PingDoesNotWake`
  - `TestGameServer_WakeOnConnect_UnarmedNoSentinel`
- **Suppression directives**: No new `//nolint` or `//#nosec` directives were added. The existing G115 suppression in `.golangci.yml` (for the VarInt read cast in minecraft.go) is retained.
- **Deprecated facades**: (SUPERSEDED by PR #248) The old facade functions were initially retained in `gameproto/gameproto.go` with `// Deprecated:` annotations for equivalence testing, but have been deleted as task T081 (see "Follow-Ups" section below).

---

## Verification

### CI Evidence (PR #245, all 62 checks pass)

**Go unit tests:**
- `go (gameproto / amd64 + arm64)` — unit tests + 90% line coverage gate PASS
- `go (sentinel / amd64 + arm64)` — unit tests + 70% line coverage gate PASS (measured 82.6%)

**Lint:**
- `lint (gameproto)` — golangci-lint with 14 active linters: PASS (zero findings)
- `lint (sentinel)` — golangci-lint with 14 active linters: PASS (zero findings)

**E2E game bot (kind cluster):**
- `TestGameServer_WakeOnConnect_LoginWakes` — verifies a genuine join wakes the server: PASS
- `TestGameServer_WakeOnConnect_PingDoesNotWake` — verifies a server-list ping does not wake: PASS
- `TestGameServer_WakeOnConnect_UnarmedNoSentinel` — verifies unarmed sleeping servers (no sentinel pod) remain asleep: PASS

**E2E operator (amd64 + arm64)** — full operator integration suite: PASS (all tests unchanged from main branch)

### Equivalence Testing (SUPERSEDED by PR #248)

**Historical**: The refactored code initially passed a suite of equivalence tests comparing old facade functions against new Classifier implementations on identical handshake bytes.

**Current state**: PR #248 deleted the old-vs-new comparison suite and replaced it with a golden-vector suite (`gameproto/classifier_golden_test.go`, 559 lines) that directly verifies Classifier behavior without the deprecated facades. The golden suite validates the same behavioral contract (Kind, Consumed, Detail consistency) more directly and maintainably. All E2E tests (wake-on-connect) continue to pass, confirming behavior preservation.

---

## Follow-Ups

### T081: Delete the Deprecated Facades (COMPLETED by PR #248)

**Status**: COMPLETED ✓

The gate for deletion (equivalence suite passing + wake-on-connect e2e green) was satisfied after PR #245 merged. PR #248 completed task T081 by executing:

- ✓ Deleted functions: `ClassifyMinecraft`, `ClassifyTerraria`, `BuildMinecraftStatusResponse`, `BuildMinecraftLoginDisconnect`, `BuildTerrariaDisconnect`
- ✓ Deleted result types: `MinecraftClassifyResult`, `TerrariaClassifyResult`
- ✓ Deleted equivalence test file: `gameproto/classifier_equivalence_test.go` (replaced by golden-vector suite)
- ✓ Committed as "fix(gameproto): correct golden expectation and six lint findings" (commit 1122239)

The behavior-preservation contract is now expressed via the golden-vector test suite (`gameproto/classifier_golden_test.go`), which is more maintainable and clearly specifies the behavioral contract without old-vs-new comparisons.

### Optional Follow-Ups (Deferred)

1. **Fold `handleGeneric` into the registry** — Currently, UDP fallback (`handleGeneric`) is outside the registry because UDP has no standard handshake. A future enhancement could create a `GenericClassifier` (always returns Unknown, no-op response building) and register it under `"generic"`, then unify the dispatch logic. This would simplify `sentinel/main.go` further but offers marginal value today since UDP is Sentinel-specific. Deferred per spec Assumption (Assumption 6).

2. **Protocol version negotiation** — Some game servers support multiple protocol versions (e.g., Minecraft 1.20 and 1.21). The current Classify contract detects the version and returns it in Detail, but response building doesn't adapt. A follow-up could add version-aware response building. Out-of-scope for this refactor.

3. **Performance profiling** — The registry lookup is O(1) map access (negligible overhead), but a real deployment may warrant measuring per-connection classification time and GC pressure during high-volume bursts. Deferred until deployment feedback.

---

## Summary

The refactor successfully replaces a fragmented, multi-file editing pattern with a composable registry abstraction. Adding a new protocol now requires one implementation file and one registry entry with zero edits to shared code. All existing behavior is preserved (verified by E2E and equivalence tests). The codebase is ready for scaling to 16+ protocols without linear growth in maintenance burden.
