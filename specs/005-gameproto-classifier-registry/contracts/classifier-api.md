# Classifier API Contract

**Feature**: 005-gameproto-classifier-registry  
**Phase**: 1 (Contracts)  
**Date**: 2026-08-20  
**Status**: Proposal for Code Review  

This document specifies the exported Go API surface of the refactored `gameproto/` module after the registry-based Classifier pattern is implemented. Every item is mapped relative to the current API to show what is NEW, UNCHANGED, or REMOVED.

---

## Overview

The refactored `gameproto/` module replaces per-protocol facade functions with a Classifier abstraction, enabling protocol-specific logic to be encapsulated in a single type per protocol. A central registry maps protocol names to Classifiers, allowing the sentinel and other consumers to dispatch to the appropriate handler without hardcoded switches.

**Module Location**: `/home/valgul/project/kubernetes-game-dashboard/gameproto/`  
**Primary Consumer**: `/home/valgul/project/kubernetes-game-dashboard/sentinel/main.go`  
**Scope**: This is an internal workspace module with one known consumer (sentinel). Removals of current public API items are not breaking changes to external parties.

---

## Type Definitions

### Classification Kind (Enumeration)

```go
type Kind int

const (
    Join Kind = iota     // Connection is a join attempt (player login)
    Status               // Connection is a status ping (server list query)
    Unknown              // Connection does not match any known handshake pattern
)
```

**Status**: NEW  
**Semantics**: The Kind enumeration classifies an incoming connection based on handshake parsing. Used by the Classifier to report what type of connection was detected.

**Preconditions**: None (zero value is Unknown).  
**Postconditions**: One of the three values is always returned; no other values exist.  
**Error/Zero-Value Behavior**: Zero value is Kind(0) = Unknown.  
**Concurrency**: Safe to use from multiple goroutines (value type).

---

### Detail Interface

```go
type Detail interface {
    // ProtocolName returns the protocol identifier for debugging.
    // Examples: "minecraft", "terraria", "factorio".
    ProtocolName() string
}
```

**Status**: NEW  
**Semantics**: An interface for protocol-specific metadata carried in a ClassificationResult. Each protocol implements a concrete Detail type (MinecraftDetail, TerrariaDetail, etc.) carrying parsed fields from the handshake (version, server address, player name, etc.). The interface contains only a single method to enforce consistent protocol identification.

**Preconditions**: Implemented by each Classifier's Detail type.  
**Postconditions**: ProtocolName() returns a consistent, lowercased protocol identifier.  
**Error/Zero-Value Behavior**: nil Detail (for Unknown classification) is valid; callers must check for nil before asserting or calling methods.  
**Concurrency**: Safe if the underlying Detail implementation is immutable.

---

### ClassificationResult Struct

```go
type ClassificationResult struct {
    // Kind is the classification of the connection (Join, Status, or Unknown).
    Kind Kind
    
    // Consumed holds the bytes read and parsed from the stream
    // during handshake classification. The sentinel uses this to replay
    // the consumed handshake bytes to the real upstream server, enabling
    // lossless stream forwarding. Exact byte accounting: Len(Consumed) + 
    // Len(remaining in bufio.Reader) == Len(original stream).
    Consumed []byte
    
    // Detail is protocol-specific metadata (e.g., MinecraftDetail, TerrariaDetail).
    // Nil for Unknown classification or any outcome where the protocol
    // has no relevant detail to return. Callers must check for nil before
    // using type assertions.
    Detail Detail
}
```

**Status**: NEW  
**Semantics**: The unified result type for all protocol classifications. Carries the classification outcome, the byte count for stream replay, and optional protocol-specific detail.

**Preconditions**:
- Consumed is a byte slice (may be empty if no bytes were parsed).
- For Unknown classification, Detail is nil.
- For Join/Status, Detail may be nil if the protocol has no relevant detail to return, or non-nil if detail is available.

**Postconditions**:
- Returned by every Classifier.Classify() call that does not error.
- On a non-error path (error==nil), *ClassificationResult is always non-nil. 
  This is load-bearing: even when Kind == Unknown, the caller needs result.Consumed 
  to replay already-read bytes to a generic handler or upstream server.
- On an error path (error!=nil), result may be nil.
- Consumed contains exactly the bytes read during handshake parsing, enabling stream replay.

**Error/Zero-Value Behavior**: The zero value ClassificationResult{} has Kind=Unknown, Consumed=[]byte(nil), Detail=nil, which is a valid result (no handshake matched).

**Concurrency**: Safe to pass between goroutines (value type).

---

### Classifier Interface

```go
type Classifier interface {
    // Classify reads handshake bytes from the reader and classifies
    // the connection as Join, Status, or Unknown.
    //
    // The reader must be positioned at the start of the handshake.
    // Classify reads only the bytes it needs to make a classification
    // decision; remaining bytes are left in the reader for the caller
    // to replay or forward.
    //
    // Preconditions:
    // - br is a valid *bufio.Reader positioned at the start of the stream.
    // - The stream contains at least the beginning of a potential handshake
    //   (or may be empty/truncated).
    //
    // Postconditions:
    // - Returns (*ClassificationResult, error) where the result contains
    //   the classification outcome (Kind, Consumed, Detail).
    // - On a non-error path: result is always non-nil, with Kind set and
    //   Consumed containing the bytes read for handshake parsing (may be empty).
    // - The first len(Consumed) bytes have been read from br; remaining bytes
    //   are available for the next read (replay to upstream).
    // - On error: result may be nil; the stream position is undefined and the
    //   caller must not attempt to replay.
    //
    // Error Behavior:
    // - If the stream is incomplete or malformed (cannot parse a complete
    //   handshake), returns result with result.Kind=Unknown and result.Consumed=[]byte{},
    //   error=nil. The caller can choose to wait for more bytes or forward the
    //   truncated stream to a generic handler.
    // - If an internal error occurs (e.g., I/O failure on the reader),
    //   returns error!=nil. On error, result may be nil.
    // - Never panics or crashes on adversarial input; errors are returned
    //   as values.
    //
    // Concurrency: Safe to call from multiple goroutines concurrently
    // on different connections (each connection has its own reader).
    // The Classifier itself is stateless.
    Classify(br *bufio.Reader) (*ClassificationResult, error)
    
    // SupportsStatusPing reports whether this protocol supports
    // out-of-band status pings (e.g., Minecraft's Server List Ping).
    //
    // Preconditions: None.
    //
    // Postconditions: Returns true if BuildStatusResponse() is applicable
    // for this protocol, false otherwise.
    //
    // Error/Zero-Value Behavior: Zero value is false (does not support).
    //
    // Concurrency: Safe to call from multiple goroutines concurrently.
    // Returns a constant value per protocol.
    SupportsStatusPing() bool
    
    // BuildStatusResponse constructs a status-ping response (if the
    // protocol supports it) from a JSON payload.
    //
    // Preconditions:
    // - SupportsStatusPing() returns true for this protocol.
    // - payload is a valid JSON string (typically a server description
    //   or player count).
    // - Call this only if the classification was Kind=Status.
    //
    // Postconditions:
    // - Returns the wire-protocol bytes to send back to the client.
    // - Byte order, framing, and encoding match the protocol's spec.
    //
    // Error Behavior:
    // - If the protocol does not support status pings (SupportsStatusPing()
    //   is false), returns error!=nil. The caller should not call this
    //   method if SupportsStatusPing() is false.
    // - If payload is malformed or too large, returns error!=nil with a
    //   descriptive message.
    // - Never panics.
    //
    // Concurrency: Safe to call from multiple goroutines concurrently.
    BuildStatusResponse(payload string) ([]byte, error)
    
    // BuildDisconnect constructs a disconnect/timeout message to send
    // to a client that initiated a join but was rejected (e.g., due to
    // server timeout or explicit rejection).
    //
    // Preconditions:
    // - reason is a short string (< 512 chars) describing why the
    //   disconnect is happening (e.g., "server is starting").
    //
    // Postconditions:
    // - Returns the wire-protocol bytes to send to the client, formatted
    //   according to the protocol's disconnect message spec.
    //
    // Error Behavior:
    // - If reason is too long or malformed, returns error!=nil.
    // - Never panics.
    //
    // Concurrency: Safe to call from multiple goroutines concurrently.
    BuildDisconnect(reason string) ([]byte, error)
}
```

**Status**: NEW  
**Semantics**: The core abstraction encapsulating all protocol-specific logic. Each game protocol (Minecraft, Terraria, future) provides one implementation. The Classifier's methods are called by the sentinel dispatcher to parse handshakes, determine response types, and build protocol-conformant replies.

**Method-Level Contracts**: See inline comments above for each method's preconditions, postconditions, and concurrency guarantees.

**Concurrency Note**: The Classifier itself is stateless and safe for concurrent use. Each connection has its own reader; the Classifier's methods coordinate via that reader only, not via shared mutable state.

---

## Registry Functions

### Register Function

```go
func Register(name string, classifier Classifier) error
```

**Status**: NEW  
**Semantics**: Registers a Classifier in the central registry, keyed by protocol name.

**Preconditions**:
- name is a non-empty, lowercased string (e.g., "minecraft", "terraria").
- classifier is a non-nil Classifier implementation.
- name must be unique; registering a duplicate name is an error (see below).

**Postconditions**:
- The Classifier is stored in the registry and available via Lookup().
- Returns nil on success.
- Returns an error if name is already registered (duplicate prevention).

**Error/Zero-Value Behavior**:
- Returns error!=nil if name is already registered (the error message clearly identifies the duplicate).
- Returns error!=nil if name is empty or contains invalid characters.
- Does not panic.

**Concurrency Note**: Registration MUST complete at startup (during init or main()), before any connection handling begins. Register() is not safe for concurrent use alongside Lookup() during normal operation. The project assumes a single-threaded startup phase followed by concurrent Lookup() calls.

**Design Note**: The actual implementation uses an explicit map literal in `gameproto/registry.go`, not a Register() function called at runtime. This preserves compile-time safety and simplifies review (all registrations are visible in one place). However, the Register() function is available as an optional convenience for tests or future extensibility.

---

### Lookup Function

```go
func Lookup(name string) (Classifier, bool)
```

**Status**: NEW  
**Semantics**: Retrieves a Classifier from the registry by protocol name.

**Preconditions**:
- name is a non-empty string (should be lowercased to match registered names).

**Postconditions**:
- Returns (classifier, true) if the name is found in the registry.
- Returns (nil, false) if the name is not found.

**Error/Zero-Value Behavior**:
- Returns (nil, false) for unknown names; does not return an error or panic.
- Returns (nil, false) for empty string input.

**Concurrency**: Lookup() is safe for concurrent use from multiple goroutines after startup completes. The registry is read-only during normal operation.

---

### ListRegistered Function

```go
func ListRegistered() []string
```

**Status**: NEW  
**Semantics**: Returns a list of all registered protocol names, for inspection and debugging.

**Preconditions**: None.

**Postconditions**:
- Returns a slice of protocol names (e.g., ["minecraft", "terraria"]).
- The slice is a snapshot of registered names at the time of call.
- Names are sorted alphabetically for stable output.

**Error/Zero-Value Behavior**:
- Returns an empty slice if no protocols are registered (unlikely in normal operation).
- Never panics.

**Concurrency**: Safe to call from multiple goroutines at any time.

---

## Current Facade Functions (REMOVED)

The following functions in `gameproto/gameproto.go` are REMOVED in this refactor. Callers (primarily sentinel) are updated to use the Classifier interface instead.

| Function | Current Signature | Removal Reason | Replacement |
|----------|-------------------|-----------------|-------------|
| `ClassifyMinecraft` | `func ClassifyMinecraft(br *bufio.Reader) (Kind, *MinecraftClassifyResult, error)` | Replaced by Minecraft's Classifier.Classify() | `result, err := Lookup("minecraft").Classify(br); kind := result.Kind` |
| `ClassifyTerraria` | `func ClassifyTerraria(br *bufio.Reader) (Kind, *TerrariaClassifyResult, error)` | Replaced by Terraria's Classifier.Classify() | `result, err := Lookup("terraria").Classify(br); kind := result.Kind` |
| `BuildMinecraftStatusResponse` | `func BuildMinecraftStatusResponse(payload string) ([]byte, error)` | Moved into Minecraft Classifier.BuildStatusResponse() | `Lookup("minecraft").BuildStatusResponse(payload)` |
| `BuildMinecraftLoginDisconnect` | `func BuildMinecraftLoginDisconnect(reason string) ([]byte, error)` | Moved into Minecraft Classifier.BuildDisconnect() | `Lookup("minecraft").BuildDisconnect(reason)` |
| `BuildTerrariaDisconnect` | `func BuildTerrariaDisconnect(reason string) ([]byte, error)` | Moved into Terraria Classifier.BuildDisconnect() | `Lookup("terraria").BuildDisconnect(reason)` |

**Breaking Change Statement**: `gameproto` is an internal workspace module with exactly one known consumer (sentinel). These removals are not external breaking changes — sentinel is updated in the same refactor. No third-party code depends on these facade functions.

---

## Current Result Types (REPLACED)

The following result types in `gameproto/gameproto.go` are REPLACED by the unified ClassificationResult and protocol-specific Detail implementations.

| Type | Current Structure | Replacement | Migration Path |
|------|-------------------|-------------|-----------------|
| `MinecraftClassifyResult` | `struct { ProtocolVersion int32; NextState int32; ServerAddr string; Consumed []byte }` | `ClassificationResult{ Kind, Consumed, Detail: *MinecraftDetail }` | Type-assert `result.Detail.(*MinecraftDetail)` to access protocol-specific fields |
| `TerrariaClassifyResult` | `struct { Version string; Consumed []byte }` | `ClassificationResult{ Kind, Consumed, Detail: *TerrariaDetail }` | Type-assert `result.Detail.(*TerrariaDetail)` to access protocol-specific fields |

**Design Rationale**: A single ClassificationResult type (with a detail interface) scales better than maintaining a separate result struct per protocol. Callers use type assertion to extract protocol-specific fields only when needed.

---

## New Type Definitions (Protocol-Specific Detail)

### MinecraftDetail Struct

```go
type MinecraftDetail struct {
    ProtocolVersion int32  // Minecraft protocol version from the handshake
    NextState       int32  // Minecraft next state (1=Status, 2=Login)
    ServerAddr      string // Server address from the handshake (host:port)
}

func (d *MinecraftDetail) ProtocolName() string {
    return "minecraft"
}
```

**Status**: NEW  
**Semantics**: Protocol-specific metadata parsed from a Minecraft handshake. Implements the Detail interface.

**Preconditions**: Populated by MinecraftClassifier.Classify() if Kind != Unknown.

**Postconditions**: Fields are immutable after creation.

**Concurrency**: Safe to share between goroutines (value semantics).

---

### TerrariaDetail Struct

```go
type TerrariaDetail struct {
    Version string  // Terraria version string from the connect request
}

func (d *TerrariaDetail) ProtocolName() string {
    return "terraria"
}
```

**Status**: NEW  
**Semantics**: Protocol-specific metadata parsed from a Terraria connect request. Implements the Detail interface.

**Preconditions**: Populated by TerrariaClassifier.Classify() if Kind != Unknown.

**Postconditions**: Fields are immutable after creation.

**Concurrency**: Safe to share between goroutines (value semantics).

---

## New Constants

```go
const (
    // Default protocol name for generic (heuristic) classification.
    // The generic classifier has no handshake-based Classifier
    // implementation and is handled separately by sentinel.
    GenericProtocol = "generic"
)
```

**Status**: NEW  
**Semantics**: A named constant identifying the generic protocol (which has no Classifier in the registry, as per the project assumption that generic remains outside the registry).

---

## Existing Exports (UNCHANGED)

The following items remain unchanged and continue to be exported. Sentinel and other consumers may continue to use them as-is.

| Item | Type | Source | Purpose | Status |
|------|------|--------|---------|--------|
| `Consumed` (field/method on old result types) | int / method | `gameproto/gameproto.go` | Byte count for stream replay | Unchanged in ClassificationResult.Consumed |
| Protocol wire-protocol parsing helpers (e.g., `ReadVarInt`, `WriteVarInt` if exported) | func | `gameproto/*.go` | Internal handshake parsing utilities | Unchanged; may be private or public depending on current usage |

---

## Summary Table

| Item | Current Status | New Status | Reason |
|------|----------------|-----------|--------|
| Classifier interface | N/A | NEW | Core abstraction for protocol classification |
| Kind enumeration | N/A | NEW | Classification outcome type |
| Detail interface | N/A | NEW | Protocol-specific metadata carrier |
| ClassificationResult struct | N/A (current: separate per-protocol results) | NEW | Unified result type |
| MinecraftDetail struct | N/A | NEW | Minecraft-specific metadata |
| TerrariaDetail struct | N/A | NEW | Terraria-specific metadata |
| Lookup(name string) func | N/A | NEW | Registry lookup function |
| Register(name string, classifier) func | N/A | NEW | Registry registration function (optional; implementation uses map literal) |
| ListRegistered() func | N/A | NEW | Registry inspection function |
| ClassifyMinecraft func | EXPORTED | REMOVED | Replaced by Classifier.Classify() |
| ClassifyTerraria func | EXPORTED | REMOVED | Replaced by Classifier.Classify() |
| BuildMinecraftStatusResponse func | EXPORTED | REMOVED | Replaced by Classifier.BuildStatusResponse() |
| BuildMinecraftLoginDisconnect func | EXPORTED | REMOVED | Replaced by Classifier.BuildDisconnect() |
| BuildTerrariaDisconnect func | EXPORTED | REMOVED | Replaced by Classifier.BuildDisconnect() |
| MinecraftClassifyResult struct | EXPORTED | REMOVED | Replaced by ClassificationResult + MinecraftDetail |
| TerrariaClassifyResult struct | EXPORTED | REMOVED | Replaced by ClassificationResult + TerrariaDetail |

---

## Design Rationale

1. **Single Unified Result**: ClassificationResult replaces protocol-specific result types, reducing boilerplate and improving consistency.

2. **Detail Interface**: Protocol-specific fields are carried via a Detail interface, avoiding a bloated result struct with many nil pointers (one per protocol).

3. **Capability Flag**: SupportsStatusPing() is an explicit boolean flag, making protocol capabilities self-documenting and avoiding type assertions.

4. **Stateless Classifier**: The Classifier is stateless; concurrency is via independent readers (one per connection).

5. **Registry Lookup**: A simple Lookup(name string) function allows sentinel to dispatch without hardcoded switches, enabling new protocols to be added in isolation.

6. **No Panics**: All methods handle errors gracefully; no panics on adversarial input.

---

## Notes for Implementation

- **Minecraft and Terraria implementations** must be migrated to implement the Classifier interface and register themselves in the registry.
- **Sentinel's hardcoded dispatch** (the per-protocol switch in main.go:479–487) is replaced with a single registry-based dispatch loop.
- **Third-protocol validation** (User Story 1): A test stub or minimal implementation is added to validate the registry pattern works for a new protocol.
- **Byte-for-byte equivalence tests** (User Story 2) compare old facade results with new Classifier results on identical inputs to prove behavior is preserved.
