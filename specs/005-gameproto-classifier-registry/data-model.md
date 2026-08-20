# Phase 1: Entity Data Model

**Feature**: 005-gameproto-classifier-registry  
**Phase**: 1 (Data Model)  
**Date**: 2026-08-20  
**Status**: Design (No Implementation)

---

## Entity Overview

This Phase 1 model defines four core entities that implement the registry-based Classifier pattern:

1. **Classifier** — interface for protocol-specific handshake parsing and response building
2. **ClassificationResult** — unified result type carrying classification outcome and protocol-specific detail
3. **Detail** — interface for protocol-specific metadata
4. **Protocol Registry** — central lookup structure mapping game names to Classifiers

No implementation bodies appear in this document. This is a specification of signatures, semantics, and contracts only.

---

## Entity 1: Classifier Interface

### Purpose & Responsibility

The **Classifier** encapsulates all protocol-specific logic for classifying an incoming connection and generating protocol-native responses. Each game protocol (Minecraft, Terraria, future) has one Classifier implementation.

**What it owns**:
- Handshake parsing (reading from bufio.Reader, interpreting wire format)
- Classification decision (is this Join / Status / Unknown?)
- Bytes-consumed tracking (for stream replay contract)
- Response building (Status messages, Disconnect messages)
- Capability declaration (supports status pings? yes/no)

**What it does NOT own**:
- Network I/O beyond reading the handshake (caller owns connection)
- Connection state management (caller orchestrates wake, polling, deadline)
- Registry mechanics (registry owns storage and lookup)

### Interface Signature

```go
type Classifier interface {
	// Classify reads a handshake from br and returns a *ClassificationResult or error.
	// On success (err == nil), result is always non-nil; Kind field indicates Join, Status, or Unknown.
	// On error (err != nil), result may be nil or carry partial Consumed bytes for stream replay.
	//
	// Invariant: Classify returns a non-nil *ClassificationResult on every non-error path,
	// including Kind == Unknown, because the caller needs result.Consumed to replay the already-read bytes.
	// A nil result is only valid together with a non-nil error.
	//
	// The caller must create br as bufio.NewReader(conn) and continue using it
	// for the rest of the connection. Any pipelined data remains in br's buffer.
	Classify(br *bufio.Reader) (*ClassificationResult, error)

	// SupportsStatusPing declares whether this protocol supports out-of-band
	// status pings (e.g., Minecraft's ping/pong). Terraria returns false here.
	//
	// If false, the caller must not attempt to call BuildStatusResponse for
	// classifications with kind == Status; doing so is a caller contract violation.
	SupportsStatusPing() bool

	// BuildStatusResponse builds a status-ping reply. Only valid if
	// SupportsStatusPing() returns true and kind == Status.
	//
	// Input: payload string (JSON for Minecraft, version string for other
	// protocols, format is protocol-specific).
	// Output: framed packet bytes ready to write to the connection.
	// Error: if payload is malformed or oversized, or if this protocol does
	// not support status pings (should not happen if SupportsStatusPing()
	// was checked first).
	BuildStatusResponse(payload string) ([]byte, error)

	// BuildDisconnect builds a disconnect/timeout message. Valid for Join and
	// Status kinds; sent to the client when the server is waking.
	//
	// Input: reason string (human-readable explanation).
	// Output: framed packet bytes ready to write to the connection.
	// Error: if reason is oversized or contains characters this protocol
	// cannot encode.
	BuildDisconnect(reason string) ([]byte, error)
}
```

### Validation Rules

| Validation Rule | Traced to | Semantics |
|---|---|---|
| `Classify()` must return (*ClassificationResult, nil) where result.Kind is Join/Status/Unknown, or (nil, error) on I/O failure | FR-001, FR-002 | On success, result is always non-nil; result.Kind determines the outcome. If Kind != Unknown, result.Detail MUST be non-nil with protocol-specific data. If Kind == Unknown, result.Detail MUST be nil but result.Consumed holds partial bytes for replay. |
| `Classify()` must consume 0 or more bytes, never panic on hostile input | FR-001, SC-002 | Defensive parsing: max packet sizes, no unbounded allocations from length prefixes, bounded reads. Any oversized length prefix is rejected before allocation. |
| `Classify()` Consumed field must be exact byte count read, enabling stream replay: original == Consumed + remaining in bufio.Reader | FR-005, SC-007 | Exact byte accounting is the replay contract. Len(Consumed) + Len(remaining) must equal Len(original). This is verified by comparison tests. |
| `SupportsStatusPing()` must return consistently (same answer on every call) | FR-007, SC-008 | Capabilities do not change at runtime. Terraria returns false; Minecraft returns true. |
| If `SupportsStatusPing()` returns false, protocol must never return kind == Status | FR-007, SC-008 | Terraria has no status concept: any recognized message is Join; anything else is Unknown. |
| `BuildStatusResponse()` must fail (return error) if this protocol does not support status pings | FR-007 | Double-check: if SupportsStatusPing() == false, BuildStatusResponse() MUST return an error. |
| `BuildDisconnect()` must return framed packet bytes or error, never nil bytes | FR-002, SC-002 | Both response builders must either succeed fully or fail with a clear error; no partial/nil returns. |

### Relationships

- **Registry** — The Classifier is registered in the Protocol Registry by game name ("minecraft", "terraria", etc.). Registry.Lookup(name) returns a Classifier.
- **ClassificationResult** — Classifier.Classify() returns a ClassificationResult on Join/Status (non-Unknown). The result carries the Classifier's parsed detail via ClassificationResult.Detail.
- **Detail** — The Classifier's ClassificationResult.Detail field holds a concrete Detail implementation (MinecraftDetail, TerrariaDetail) with protocol-specific fields. Callers type-assert to access protocol-specific data.

---

## Entity 2: ClassificationResult

### Purpose & Responsibility

The **ClassificationResult** is the unified result type returned by all Classifiers. It carries the classification outcome, the bytes consumed during parsing (enabling lossless stream replay), and optionally a protocol-specific detail object.

**What it owns**:
- Classification outcome (Kind: Join / Status / Unknown)
- Bytes consumed during handshake parsing
- Protocol-specific detail (via Detail interface)

**What it does NOT own**:
- Connection state or metadata beyond the handshake
- Interpretation of detail (caller is responsible for type-asserting and checking nil)

### Struct Signature

```go
type ClassificationResult struct {
	// Kind is the classification outcome: Join, Status, or Unknown.
	// Always present.
	Kind Kind

	// Consumed holds the actual bytes read from the input during handshake
	// parsing. Always present (may be empty for Unknown or on error).
	// Enables lossless stream replay: original_stream == Consumed + remaining_in_bufio.Reader.
	Consumed []byte

	// Detail is a protocol-specific detail object carrying parsed handshake
	// fields (server address, protocol version, player name, etc.).
	// Conditionally present: nil for Unknown classification; non-nil for Join/Status.
	// Type: *MinecraftDetail, *TerrariaDetail, or other protocol implementations.
	// Caller must type-assert: if md, ok := result.Detail.(*MinecraftDetail) { ... }.
	Detail Detail
}
```

### Field Presence & Conditions

| Field | Always Present? | Presence Condition | Type |
|---|---|---|---|
| Kind | Yes | Always set by Classifier.Classify() | Kind (Join / Status / Unknown) |
| Consumed | Yes | Always set; may be empty slice if no bytes were read | []byte |
| Detail | Conditionally | Nil for Unknown; non-nil for Join or Status | Detail interface (nil is valid) |

**Detail Presence Semantics**:
- If `Kind == Unknown`: Detail MUST be nil.
- If `Kind == Join` or `Kind == Status`: Detail SHOULD be non-nil and carry protocol-specific parsed fields.
- Callers check `if result.Detail != nil` before type-asserting to access protocol-specific fields.

### Validation Rules

| Validation Rule | Traced to | Semantics |
|---|---|---|
| If Kind == Unknown, Detail MUST be nil | FR-002, SC-008 | Unknown classifications carry no protocol-specific detail. |
| If Kind == Join, Detail MUST be non-nil and implement Detail interface | FR-002 | Join requires parsed handshake fields (e.g., server address, protocol version). |
| If Kind == Status, Detail MUST be non-nil (for protocols that support Status) | FR-002, FR-007 | Status requires enough parsed detail to identify the protocol (e.g., Minecraft version). |
| If classifier.SupportsStatusPing() == false, Kind must never be Status | FR-007, SC-008 | Terraria example: no status concept, so Kind is always Join or Unknown. |
| Consumed must be exact byte count read; Len(Consumed) + Len(remaining_in_bufio) == Len(original) | FR-005, SC-007 | Exact byte accounting enables lossless stream replay. Verified by comparison tests. |
| Consumed may be empty (0 bytes) if no bytes were read before classification failed (EOF on empty stream) | FR-002 | Defensive parsing on truncated input. |

### Relationships

- **Classifier** — Returned by Classifier.Classify(). Classifier.Detail field is set by the protocol's implementation.
- **Detail** — ClassificationResult.Detail is either nil or holds a concrete implementation (MinecraftDetail, TerrariaDetail, etc.).
- **Sentinel Dispatcher** — Consumed field is used to replay bytes to the upstream server after waking.

---

## Entity 3: Detail Interface

### Purpose & Responsibility

The **Detail** interface represents protocol-specific metadata parsed from a handshake. Each protocol (Minecraft, Terraria, future) implements a concrete Detail type carrying its specific fields.

**What it owns**:
- Protocol identification (ProtocolName() string)
- Protocol-specific parsed fields (server address, version, player name, etc.)

**What it does NOT own**:
- Connection state or I/O
- Validation of detail fields (that is a Classifier responsibility)

### Interface Signature

```go
type Detail interface {
	// ProtocolName returns the name of the protocol ("minecraft", "terraria", etc.)
	// for debugging and logging.
	ProtocolName() string
}
```

### Concrete Implementations

**MinecraftDetail** (struct):
```go
type MinecraftDetail struct {
	// ProtocolVersion is the version field from the handshake packet.
	// Value varies by Minecraft client version.
	// Always present.
	ProtocolVersion int32

	// NextState is the requested next protocol state after handshake.
	// 1 = Status (server-list ping), 2 = Login (player join attempt).
	// Always present.
	NextState int32

	// ServerAddr is the server address (host:port) the client sent in the handshake.
	// Always present (may be empty or malformed if parsed from truncated input).
	ServerAddr string
}

func (d *MinecraftDetail) ProtocolName() string { return "minecraft" }
```

**TerrariaDetail** (struct):
```go
type TerrariaDetail struct {
	// Version is the protocol version string from the ConnectRequest.
	// Always present.
	Version string
}

func (d *TerrariaDetail) ProtocolName() string { return "terraria" }
```

### Validation Rules

| Validation Rule | Traced to | Semantics |
|---|---|---|
| Every Detail implementation MUST define ProtocolName() returning a non-empty string | FR-001 | ProtocolName() is the only contract that all Detail types must implement. |
| Protocol-specific fields may contain defensive defaults or empty values if parsing was incomplete | SC-002 | If handshake bytes are truncated, detail fields reflect what was parsed; it is the caller's responsibility to decide whether to use them. |

### Relationships

- **ClassificationResult** — Held in ClassificationResult.Detail (nil for Unknown, non-nil for Join/Status).
- **Classifier** — The Classifier implementation creates and populates the concrete Detail type during Classify().

---

## Entity 4: Protocol Registry

### Purpose & Responsibility

The **Protocol Registry** is a central, initialization-time lookup structure mapping game protocol names (strings) to Classifier implementations. The registry is static after initialization and provides no write operations at runtime.

**What it owns**:
- Storage of name-to-Classifier mappings
- Lookup operations (find a Classifier by name)
- Validation of registry completeness (all expected protocols present, no duplicates)

**What it does NOT own**:
- Classifier implementations (those are defined in their protocol files)
- Connection handling (that is the Sentinel Dispatcher's job)
- Runtime protocol registration (registration is compile-time via map literal)

### API Signature

```go
// Registry holds a static map of protocol names to Classifiers.
// Initialized at startup with explicit registration; read-only thereafter.
type Registry struct {
	classifiers map[string]Classifier
}

// Register adds a Classifier to the registry under the given name.
// If called during initialization (main.go), it populates classifiers.
// Panics if name is empty or already registered (double registration).
//
// Note: The current implementation uses an explicit map literal in registry.go.
// Register() is shown for API documentation; the actual registry is populated
// by declaring a global map[string]Classifier in registry.go and passing it to
// NewRegistry().
func (r *Registry) Register(name string, classifier Classifier) {
	if name == "" {
		panic("register: empty protocol name")
	}
	if _, exists := r.classifiers[name]; exists {
		panic(fmt.Sprintf("register: protocol %q already registered", name))
	}
	r.classifiers[name] = classifier
}

// Lookup retrieves a Classifier by protocol name.
// Returns (classifier, true) if found, (nil, false) if not found.
// Never panics on unknown names; caller must handle lookup miss gracefully.
func (r *Registry) Lookup(name string) (Classifier, bool) {
	classifier, ok := r.classifiers[name]
	return classifier, ok
}

// Protocols returns a slice of all registered protocol names (for debugging/audit).
// Useful for logging or verifying registry completeness at startup.
func (r *Registry) Protocols() []string {
	names := make([]string, 0, len(r.classifiers))
	for name := range r.classifiers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
```

### Registry Initialization

**Explicit Map Literal** (in `gameproto/registry.go`):

```go
// defaultRegistry is the static, compile-time registry mapping protocol names to Classifiers.
// Populated with explicit entries for each supported protocol.
var defaultRegistry = &Registry{
	classifiers: map[string]Classifier{
		"minecraft": &MinecraftClassifier{},
		"terraria":  &TerrariaClassifier{},
		// Future protocols registered here: "factorio", "valheim", etc.
		// Each is a single map line; no edits to gameproto/gameproto.go or sentinel/main.go required.
	},
}

// Global returns the default (singleton) registry.
func Global() *Registry {
	return defaultRegistry
}
```

### Validation Rules

| Validation Rule | Traced to | Semantics |
|---|---|---|
| Registry MUST be initialized with all expected protocols (Minecraft, Terraria, plus any demo/test protocols) | SC-001, SC-006 | Empty registry or missing protocols are detected by tests at CI time, not at runtime. |
| No duplicate names allowed in the registry | FR-004, SC-001 | Explicit map literal prevents duplicates (Go compiler error on duplicate keys), or tests catch typos. |
| Lookup MUST handle unknown names gracefully (return false, not panic) | FR-004 | Unknown protocol names are configuration errors caught at sentinel startup (log fatal if not found), not runtime crashes. |
| Registry MUST be queryable for completeness audits (Protocols() method) | SC-006 | Maintainers can verify all expected protocols are present via grep or test assertions. |

### Relationships

- **Classifier** — Registry stores Classifier implementations. Each registered protocol has exactly one Classifier in the registry.
- **Sentinel Dispatcher** — Calls Registry.Lookup(wakeProtocol) at startup and during each connection dispatch. Unknown protocols cause a fatal startup error.

---

## State/Outcome Transitions

### Classification Outcomes

The Classifier.Classify() method returns one of three outcomes, each with distinct semantics for Consumed bytes, Detail presence, and caller obligations.

**Core Invariant**: Classify returns a non-nil *ClassificationResult on every non-error path, including Kind == Unknown, because the caller needs result.Consumed to replay the already-read bytes. A nil result is only valid together with a non-nil error.

#### Outcome 1: Join (Kind = 0)

**Meaning**: A player is attempting to connect to the server.

**Consumed bytes**: Full handshake bytes read (Len(Consumed) > 0 in healthy input).

**Detail presence**: Always non-nil; holds protocol-specific parsed fields (e.g., MinecraftDetail with ProtocolVersion, NextState, ServerAddr).

**Caller obligations**:
1. Replay Consumed bytes to the upstream server (via connection write or proxy).
2. Forward any pipelined bytes remaining in the bufio.Reader to the upstream server.
3. Wake the GameServer (if dormant) and establish the upstream connection.
4. After the deadline (if server is still waking), send a protocol-native Disconnect message (via Classifier.BuildDisconnect()) and close.

**Example flow** (Minecraft):
```
Classify() → Kind=Join, Detail=MinecraftDetail{ProtocolVersion=765, NextState=2, ServerAddr="example.com:25565"}, Consumed=[handshake bytes]
→ Write Consumed to upstream
→ io.Copy remaining bufio.Reader to upstream
→ Wake GameServer
→ After deadline: BuildDisconnect("Server is starting") → Write to client → close
```

---

#### Outcome 2: Status (Kind = 1)

**Meaning**: The client is pinging the server for status information (e.g., server-list entry). Answer without waking.

**Consumed bytes**: Full status-ping request bytes read.

**Detail presence**: Non-nil for protocols that support Status (Minecraft only, in current codebase). Terraria never returns Status (see Asymmetry below).

**Caller obligations**:
1. Check if Classifier.SupportsStatusPing() is true. If false, treat as Unknown (close without responding).
2. If supported, build a status response via Classifier.BuildStatusResponse(payload) and write to the client.
3. Close the connection (do NOT wake the server).

**Example flow** (Minecraft):
```
Classify() → Kind=Status, Detail=MinecraftDetail{NextState=1, ...}, Consumed=[ping request bytes]
→ SupportsStatusPing() → true
→ BuildStatusResponse(minecraftAsleepStatusJSON) → response bytes
→ Write response to client
→ close (no wake, no upstream connection)
```

**Terraria asymmetry** (see below): Terraria has no Status outcome. Any recognized connection is Join.

---

#### Outcome 3: Unknown (Kind = 2)

**Meaning**: The bytes did not parse as a handshake for this protocol. Do not wake; let the connection fail naturally or fall through to a generic handler.

**Consumed bytes**: May be 0 (if parsing failed immediately) or partial (some bytes were examined before determining this is not the protocol).

**Detail presence**: Always nil (no protocol-specific detail for unrecognized bytes).

**Caller obligations**:
1. Do NOT wake the GameServer.
2. Do NOT attempt to build a response (Detail is nil, and sending protocol-native bytes for an unknown protocol is meaningless).
3. Either close the connection or fall back to a generic handler (e.g., packets-in-window heuristic in sentinel/main.go).

**Example flow**:
```
Classify() → Kind=Unknown, Detail=nil, Consumed=[0 or partial bytes]
→ close (or forward to handleGeneric)
```

---

### Terraria Asymmetry (First-Class Case)

**Terraria has no out-of-band status ping.** This is a wire-protocol property: Terraria's only handshake is a player ConnectRequest; there is no separate status-ping message.

**Consequence**:
- `TerrariaClassifier.SupportsStatusPing()` returns `false`.
- `TerrariaClassifier.Classify()` returns either:
  - `Kind=Join` if the bytes parse as a valid ConnectRequest.
  - `Kind=Unknown` if the bytes do not parse as a ConnectRequest.
- `TerrariaClassifier.Classify()` **never** returns `Kind=Status`.
- If caller attempts to call `TerrariaClassifier.BuildStatusResponse()`, it must return an error (or panic, but errors are safer).

**In Sentinel dispatcher**:
```go
// Pseudocode (not part of data model, just illustration)
result := classifier.Classify(br)

if result.Kind == Status {
	// Only reachable if SupportsStatusPing() == true
	response := classifier.BuildStatusResponse(payload)
	conn.Write(response)
} else if result.Kind == Join {
	// Wake the server
	handleRegistryProtocol(...)
} else {
	// Unknown
	close(conn)
}
```

For Terraria, the `if result.Kind == Status` branch is **unreachable** because Classify() never returns Status. This is not an error or special case in the dispatcher; it is a natural consequence of the protocol's design.

---

## Registry Lifecycle

### Initialization (Startup)

1. **Registry construction** (`gameproto/registry.go`):
   - A package-level registry map is declared with static entries for all supported protocols.
   - No runtime registration calls; all protocols are known at compile time.
   - Example:
     ```go
     var defaultRegistry = &Registry{
         classifiers: map[string]Classifier{
             "minecraft": &MinecraftClassifier{},
             "terraria":  &TerrariaClassifier{},
         },
     }
     ```

2. **Validation** (in `gameproto_test.go`):
   - Tests assert that:
     - All expected protocols are registered (TestRegistryCompleteness).
     - No duplicate names exist (TestRegistryNoDuplicates, verified by Go compiler on duplicate literal keys).
     - Exactly one Classifier per protocol (TestOneClassifierPerProtocol).
   - Tests run at CI time; failures block merge.

3. **Sentinel startup** (`sentinel/main.go`):
   - On startup, sentinel loads port configuration (wakeProtocol values).
   - For each configured wakeProtocol, sentinel calls Registry.Lookup(name).
   - If Lookup returns false (not found), log a fatal error and refuse to start.
   - Error message must clearly identify the unknown protocol and suggest checking the registry.
   - Example: `"fatal: wakeProtocol 'typoname' not registered; available: minecraft, terraria"`

---

### Read Operations (Runtime)

1. **Protocol dispatch** (in `sentinel/main.go`):
   - On each inbound TCP connection, sentinel reads the wakeProtocol from config and calls Registry.Lookup(name).
   - If found, sentinel passes the Classifier to the dispatcher.
   - If not found, this is a config error (should not happen if startup validation passed).

2. **Classifier instantiation**:
   - Each protocol has one Classifier instance (singleton).
   - Classifiers are stateless (no per-connection state).
   - Multiple goroutines can call Classifier.Classify() concurrently (must be goroutine-safe).

---

### Duplicate Registration Handling

1. **Static map literal** (compile time):
   - Go compiler rejects duplicate literal keys in a map literal with a compile error.
   - Example: `map[string]Classifier{"minecraft": ..., "minecraft": ...}` is a Go syntax error.

2. **Typo detection** (test time):
   - If a protocol name is misspelled in the registry map literal (e.g., "minecfraft"), the test for completeness catches it.
   - Test: `TestRegistryCompleteness` asserts the registry has exactly the set of expected protocol names.
   - Failure: `"expected protocols [minecraft, terraria, demo] but registry has [minecfraft, terraria, demo]"`.

3. **Sentinel validation** (startup time):
   - If config references a protocol not in the registry (e.g., via manual helm value edit), sentinel startup fails fatally.
   - Error logged with protocol name and available registrations.

---

### Lookup Miss Handling (Unknown Protocol Name)

**Trigger**: Sentinel config specifies a wakeProtocol value (e.g., "unknown_protocol") that is not in the registry.

**Detection**:
1. **Startup** (primary): Sentinel calls Registry.Lookup("unknown_protocol") and receives (nil, false).
2. Sentinel logs a fatal error and refuses to start, preventing silent misconfiguration.

**Error message format**:
```
fatal: wakeProtocol 'unknown_protocol' not registered; available protocols: minecraft, terraria
```

**Consequence**:
- Configuration error is caught immediately (within seconds of startup), not deferred to the first connection.
- Operator can fix config and retry without any connection state lost.
- No fallback to generic handler (generic is still available, but it must be explicitly configured as `wakeProtocol="generic"`).

---

### Demo/Test Protocol Registration (Validation of Pattern)

As part of Phase 1 (per Decision 10 in research.md), a third protocol (demo stub) is registered to validate the registry pattern.

1. **Registration**: Added to the registry map literal in `gameproto/registry.go`.
   ```go
   classifiers: map[string]Classifier{
       "minecraft": &MinecraftClassifier{},
       "terraria":  &TerrariaClassifier{},
       "demo": &DemoClassifier{}, // Demo/test-only protocol
   }
   ```

2. **Implementation**: Demo stub implements Classifier fully (Classify, SupportsStatusPing, BuildStatusResponse, BuildDisconnect) but with minimal logic (e.g., always returns Unknown, or always returns Join).

3. **E2E validation**: A sentinel E2E test configures a listening port with `wakeProtocol="demo"` and verifies:
   - Sentinel starts without error.
   - Registry.Lookup("demo") succeeds.
   - Classifier dispatch works (connection is handled by DemoClassifier).

4. **Cleanup**: Test stub remains in the codebase permanently as a worked example of the pattern; it is never deployed to production (not exposed in Helm chart values).

---

## Mapping from Today's Types

This table shows how the current facade functions and result types map to the new registry-based Classifier model. Every current capability is preserved; nothing is dropped.

| Current (Today) | Category | New (Registry Model) | Mapping Semantics |
|---|---|---|---|
| `ClassifyMinecraft(br *bufio.Reader) (Kind, *MinecraftClassifyResult, error)` | Facade function | `MinecraftClassifier.Classify(br *bufio.Reader) (*ClassificationResult, error)` | MinecraftClassifier implements the Classifier interface. Classify() returns Kind (as result.Kind) and MinecraftClassifyResult fields are moved into MinecraftDetail within the result. |
| `ClassifyTerraria(br *bufio.Reader) (Kind, *TerrariaClassifyResult, error)` | Facade function | `TerrariaClassifier.Classify(br *bufio.Reader) (*ClassificationResult, error)` | TerrariaClassifier implements the Classifier interface. TerrariaClassifyResult fields are moved into TerrariaDetail within the result. |
| `BuildMinecraftStatusResponse(jsonPayload string) ([]byte, error)` | Facade function | `MinecraftClassifier.BuildStatusResponse(payload string) ([]byte, error)` | Moved into Classifier interface method. Now a method on MinecraftClassifier, not a package-level facade. |
| `BuildMinecraftLoginDisconnect(reason string) ([]byte, error)` | Facade function | `MinecraftClassifier.BuildDisconnect(reason string) ([]byte, error)` | Moved into Classifier interface method. Unified name (BuildDisconnect for both protocols). |
| `BuildTerrariaDisconnect(reason string) ([]byte, error)` | Facade function | `TerrariaClassifier.BuildDisconnect(reason string) ([]byte, error)` | Moved into Classifier interface method. Same unified name. |
| `MinecraftClassifyResult { ProtocolVersion, NextState, ServerAddr, Consumed }` | Result struct | `ClassificationResult { Kind, Consumed, Detail(*MinecraftDetail) }` where `MinecraftDetail { ProtocolVersion, NextState, ServerAddr }` | Protocol-specific fields moved from result struct into Detail. Consumed field preserved exactly. Kind replaces the out-of-band return value. |
| `TerrariaClassifyResult { Version, Consumed }` | Result struct | `ClassificationResult { Kind, Consumed, Detail(*TerrariaDetail) }` where `TerrariaDetail { Version }` | Protocol-specific fields moved into Detail. Consumed field preserved exactly. Kind is determined by Classify(). |
| `handleMinecraft(ctx, conn, w, upstreamAddr, deadline)` | Handler function | Unified dispatcher: `registry.Lookup("minecraft").Classify(br)` → result → switch on result.Kind → handleRegistryProtocol or BuildStatusResponse | Hardcoded switch removed. Registry lookup and Classifier dispatch replace the per-protocol handler. Sentinel has one unified handler for all protocols via the registry. |
| `handleTerraria(ctx, conn, w, upstreamAddr, deadline)` | Handler function | Same unified dispatcher (one per protocol via registry) | Same: hardcoded switch removed. |
| `bounceMinecraft(conn)` | Helper function | `MinecraftClassifier.BuildDisconnect(wakingUpMessage)` | Disconnect building moved into Classifier method. Called from the unified handleRegistryProtocol, not a separate function. |
| `bounceTerraria(conn)` | Helper function | `TerrariaClassifier.BuildDisconnect(wakingUpMessage)` | Same: disconnect building is now a Classifier method. |
| `Kind { Join=0, Status=1, Unknown=2 }` | Enum | `Kind { Join=0, Status=1, Unknown=2 }` | Unchanged. Preserved exactly. |
| `handleGeneric(ctx, conn, w, upstreamAddr, deadline)` | Handler function (no classifier) | Outside registry (unchanged) | Generic handler remains unchanged. It is not wrapped in a Classifier; it stays as a separate code path. (Assumption: generic handler outside registry). |

### Key Invariants Preserved

1. **Consumed bytes**: Every protocol's Consumed field is preserved exactly, enabling lossless stream replay.
2. **Kind classification**: The three outcomes (Join / Status / Unknown) are unchanged.
3. **Response building**: Status responses and Disconnect messages continue to be built identically (byte-for-byte verified by comparison tests).
4. **Terraria asymmetry**: Terraria never returns Status; any recognized message is Join.
5. **Error handling**: Parsing errors (EOF, malformed bytes, etc.) are handled defensively and returned as errors or Unknown, never panics.
6. **Goroutine safety**: Classifiers are stateless and can be called concurrently.

---

## Row Count Summary

**Total mapping rows**: 13 (Classify facades: 2, BuildResponse facades: 3, Result structs: 2, Handler functions: 2, Helper functions: 2, Enum: 1, Generic handler: 1)

---

## Entity List

1. **Classifier** — interface for protocol-specific handshake parsing and response building
2. **ClassificationResult** — unified result type carrying Kind, Consumed, and protocol-specific Detail
3. **Detail** — interface for protocol-specific metadata (MinecraftDetail, TerrariaDetail)
4. **Protocol Registry** — central static lookup structure mapping protocol names to Classifiers

**Total entities: 4** (1 core interface + 1 core result type + 1 detail interface + 1 registry structure)

---

**End of Phase 1 Data Model**
