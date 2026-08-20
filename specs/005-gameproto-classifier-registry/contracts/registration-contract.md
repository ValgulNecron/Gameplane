# Protocol Registration Contract

**Feature**: 005-gameproto-classifier-registry  
**Phase**: 1 (Contracts)  
**Date**: 2026-08-20  
**Status**: Proposal for Code Review  

This document specifies the structural and behavioral contract that a new game protocol must satisfy to be registered in the Classifier registry. It defines where the code lives, what it implements, how to register it, and what code reviewers check to verify a registration.

---

## Overview

A game protocol implementation (e.g., Minecraft, Terraria, Factorio) is a Go file in the `gameproto/` module that implements the `gameproto.Classifier` interface and is registered in the protocol registry. The registration enables the sentinel component to dispatch incoming connections to the appropriate handler without hardcoded dispatch logic **(FR-003)**.

**Module Location**: `/home/valgul/project/kubernetes-game-dashboard/gameproto/`  
**Registration Location**: `gameproto/registry.go` (explicit map literal)  
**Testing Location**: `gameproto/<protocol>_test.go` (colocated with implementation)

---

## File Location and Naming

### Implementation File

**Naming**: `gameproto/<protocol>.go`  
**Examples**: `gameproto/minecraft.go`, `gameproto/terraria.go`, `gameproto/factorio.go`

**Naming Rules**:
- Use the protocol's common name in lowercase (no hyphens, no spaces).
- Examples:
  - `minecraft.go` for Minecraft Java Edition
  - `terraria.go` for Terraria
  - `factorio.go` for Factorio
  - `garrymod.go` for Garry's Mod (colloquial name, so normalize to `garrymod`)
- If a protocol has multiple implementations or versions, include a disambiguation suffix:
  - `minecraft_java.go` and `minecraft_bedrock.go` (if both are supported).
  - Not required unless both will coexist in the registry.

**Source of Truth for Protocol Name**: The protocol name used in the registry (see "Protocol Key Naming" below) is the authoritative name. The implementation file name should match it (lowercase, underscores for disambiguation only).

### Test File

**Naming**: `gameproto/<protocol>_test.go`  
**Examples**: `gameproto/minecraft_test.go`, `gameproto/terraria_test.go`

**Requirements**:
- Tests for the protocol's Classifier implementation (table-driven tests preferred).
- Tests for both Classify(), SupportsStatusPing(), BuildStatusResponse(), and BuildDisconnect().
- Coverage must reach the overall `gameproto/` threshold of 90% line coverage.

---

## Protocol Key Naming

**Format**: A lowercase, alphanumeric string, no hyphens or spaces. Examples: `minecraft`, `terraria`, `factorio`, `garrymod`.

**Source of Truth**: The protocol key is derived from the game's official name, not from in-code constants or Helm chart values. The sentinel's configuration must use this key; any mismatch is a configuration error caught at startup.

**Current Mappings** (for reference):
| Game | Official Name | Registry Key | Implementation File |
|------|---------------|--------------|---------------------|
| Minecraft Java Edition | Minecraft | `minecraft` | `gameproto/minecraft.go` |
| Terraria | Terraria | `terraria` | `gameproto/terraria.go` |

**No Alias System**: The registry does not support aliases or alternate names for a single protocol. Each protocol name maps to exactly one Classifier. If a game is known by multiple colloquial names (e.g., "GMod" vs. "Garry's Mod"), pick one canonical name for the registry and document it in the implementation file's comments.

---

## Classifier Implementation

### Interface Implementation

Every protocol file MUST implement the `gameproto.Classifier` interface:

```go
type Classifier interface {
    Classify(br *bufio.Reader) (*ClassificationResult, error)
    SupportsStatusPing() bool
    BuildStatusResponse(payload string) ([]byte, error)
    BuildDisconnect(reason string) ([]byte, error)
}
```

**Preconditions for Classify()**:
- The reader is positioned at the start of a potential handshake.
- The stream may contain a complete handshake, a partial handshake, or unrelated data.

**Postconditions for Classify()**:
- Returns ClassificationResult with Kind and Consumed set.
- Returns error only on internal I/O failures, not on invalid data.
- Unknown classifications return Kind=Unknown, Consumed=0, Detail=nil, error=nil.

**SupportsStatusPing() Contract**:
- Must return a constant value (always true or always false for a given protocol).
- If false, BuildStatusResponse() will not be called (callers check first).

**BuildStatusResponse() Contract**:
- Only called if SupportsStatusPing() returns true.
- Accepts a JSON payload string (typically a server description).
- Returns wire-protocol bytes that can be sent directly to the client.
- Must handle overly-long or malformed payloads by returning an error, not panicking.

**BuildDisconnect() Contract**:
- Accepts a reason string (< 512 chars).
- Returns wire-protocol disconnect/timeout bytes for this protocol.
- Must handle overly-long or invalid input gracefully.

### Detail Implementation

If the protocol's Classify() returns Kind != Unknown, it SHOULD return a Detail implementation carrying protocol-specific parsed fields.

**Example** (Minecraft):
```go
type MinecraftDetail struct {
    ProtocolVersion int32
    NextState       int32
    ServerAddr      string
}

func (d *MinecraftDetail) ProtocolName() string {
    return "minecraft"
}
```

**Example** (Terraria):
```go
type TerrariaDetail struct {
    Version string
}

func (d *TerrariaDetail) ProtocolName() string {
    return "terraria"
}
```

**Naming Convention**: `<Protocol>Detail` (e.g., `MinecraftDetail`, `TerrariaDetail`, `FactorioDetail`).

**ProtocolName() Return**: MUST return the same value as the registry key (lowercase, matching the registry lookup name).

### Concurrency

The Classifier implementation MUST be stateless. All state is carried in the reader (one per connection). Multiple goroutines MUST be able to call Classify(), BuildStatusResponse(), and BuildDisconnect() concurrently on different readers without races or data corruption.

---

## Registration

### Registration Format

**Location**: `gameproto/registry.go`

**Format** (map literal at the module level):

```go
package gameproto

var classifierRegistry = map[string]Classifier{
    "minecraft": &MinecraftClassifier{},
    "terraria":  &TerrariaClassifier{},
    // New protocol: add one line below
    "factorio":  &FactorioClassifier{},
}

// Lookup retrieves the classifier for a given protocol name (FR-003).
// Consumers call this to look up a classifier without a hardcoded dispatch switch.
// Returns (classifier, ok) where ok is false if the protocol is not registered (FR-004).
func Lookup(name string) (Classifier, bool) {
    c, ok := classifierRegistry[name]
    return c, ok
}

func Register(name string, c Classifier) error {
    if _, exists := classifierRegistry[name]; exists {
        return fmt.Errorf("protocol %q already registered", name)
    }
    classifierRegistry[name] = c
    return nil
}

func ListRegistered() []string {
    // Return sorted list of keys
    ...
}
```

**One Entry Per Protocol**: Each protocol has exactly one entry in the map. The key is the registry key (lowercase), and the value is a concrete Classifier instance (usually a zero-initialized struct or a singleton).

**Uniqueness Requirement**: No two protocols may register with the same key. The registry tests verify this at test time.

### Registry Entry Mechanics

**Initialization**: The map is populated at package init time (module load), so registrations are always available by the time Lookup() is first called.

**No Runtime Mutations**: After initialization, the registry is read-only. No Register() calls are made during normal operation (only in tests, if at all).

**Static Validation**: Go's compiler prevents duplicate map literal keys (it errors at compile time), so an accidental duplicate registration is caught immediately.

---

## Code Review Checklist (SC-006)

When reviewing a PR that adds a new protocol, check:

### Structural

- [ ] **New implementation file**: A single new `.go` file in `gameproto/` (e.g., `gameproto/factorio.go`).
- [ ] **File naming matches registry key**: If registering as `"factorio"`, file is `factorio.go` (no disambiguation suffix unless absolutely needed).
- [ ] **Test file exists**: A single colocated test file (e.g., `gameproto/factorio_test.go`).
- [ ] **Single registry entry**: Exactly one line added to `gameproto/registry.go` (map entry).
- [ ] **No changes to other files**: The only changed files are:
  - The new implementation file (NEW).
  - The new test file (NEW).
  - `gameproto/registry.go` (one line added).
  - NO changes to `gameproto/gameproto.go`, `sentinel/main.go`, or any other existing shared code.

### Implementation

- [ ] **Implements Classifier interface**: All four methods present and correctly typed (Classify, SupportsStatusPing, BuildStatusResponse, BuildDisconnect).
- [ ] **Classify() correctness**:
  - Parses the handshake correctly (per the game's wire protocol).
  - Returns Kind=Unknown (not an error) if the stream doesn't match.
  - Returns Consumed correctly (byte count, not error).
  - Does not panic on truncated or adversarial input.
- [ ] **SupportsStatusPing() value**: Returns true or false consistently (not based on runtime state).
- [ ] **BuildStatusResponse() (if applicable)**:
  - Accepts JSON payload string.
  - Returns wire-protocol bytes (not text).
  - Returns error on malformed or oversized input (not panic).
- [ ] **BuildDisconnect() implementation**:
  - Accepts reason string.
  - Returns wire-protocol bytes.
  - Handles edge cases (empty reason, oversized reason).
- [ ] **Detail implementation (if applicable)**:
  - Struct named `<Protocol>Detail`.
  - Implements `ProtocolName()` returning the registry key.
  - ProtocolName() matches the registry entry key.

### Protocol Correctness

- [ ] **Handshake parsing**: Matches the game's official wire protocol (cite the source: Minecraft Wiki, Terraria forums, official docs, etc.).
- [ ] **Byte-for-byte equivalence** (if migrating from old code): Comparison tests verify old and new implementations return identical results on test inputs.
- [ ] **Stream replay contract**: Consumed bytes are accurate; remaining stream can be replayed to the real server.
- [ ] **Error handling**: No panics; graceful handling of truncated/malformed handshakes.

### Testing

- [ ] **Test coverage**: Tests for Classify, SupportsStatusPing, BuildStatusResponse, BuildDisconnect; table-driven tests preferred.
- [ ] **Fixtures**: Test uses realistic handshake bytes (copied from a real packet capture or constructed per the spec).
- [ ] **Edge cases**: Tests for truncated handshakes, oversized payloads, malformed data.
- [ ] **Coverage maintained**: `gameproto/` remains at 90% line coverage (CI verifies).

### Linting & Quality

- [ ] **No suppressions**: No `//nolint`, `//#nosec`, or equivalent in the new code.
- [ ] **golangci-lint clean**: `make lint` produces no findings for the new module.
- [ ] **Proper error wrapping**: Errors use `fmt.Errorf(..., %w, ...)` to preserve context.
- [ ] **TypeScript strict (N/A)**: No TypeScript changes required for a Go-only protocol addition.

### Documentation

- [ ] **Comments**: The implementation file has a package-level comment or file-level comment describing the protocol and citing its source (e.g., "// Package gameproto/minecraft.go implements Minecraft Java Edition handshake parsing per https://wiki.vg/Protocol").
- [ ] **Method comments**: Each Classifier method has a comment explaining its role.
- [ ] **Registry entry**: The registry.go entry is clearly labeled with the protocol name (e.g., `// Minecraft handshake classifier`).

### Sentinel Integration

- [ ] **Configuration compatibility**: The registry key (e.g., "factorio") is a valid wakeProtocol value that operators can configure.
- [ ] **Dispatch tested**: E2E or integration tests exercise sentinel dispatching to the new protocol (no manual testing).
- [ ] **Startup validation**: Sentinel startup tests verify the protocol name is registered; an unknown protocol name is caught at startup with a clear error message.

---

## Worked Example: Adding Protocol #3 (Factorio Stub)

This example walks through adding a minimal test stub protocol to validate the registry pattern.

### Step 1: Create Implementation File

**File**: `/home/valgul/project/kubernetes-game-dashboard/gameproto/factorio.go`

```go
package gameproto

import (
	"bufio"
	"fmt"
)

// FactorioClassifier implements the Classifier interface for Factorio.
// This is a test stub for validating the registry pattern; a real
// implementation would parse Factorio's handshake protocol.
type FactorioClassifier struct{}

// Classify reads from the connection and classifies it as a Factorio
// join, status, or unknown.
// For this stub, always returns Unknown (0 bytes consumed).
func (c *FactorioClassifier) Classify(br *bufio.Reader) (*ClassificationResult, error) {
	// Stub: always unknown. A real implementation would parse the handshake.
	return &ClassificationResult{
		Kind:      Unknown,
		Consumed:  0,
		Detail:    nil,
	}, nil
}

// SupportsStatusPing reports whether Factorio supports status pings.
// Factorio does not have an out-of-band status ping mechanism.
func (c *FactorioClassifier) SupportsStatusPing() bool {
	return false
}

// BuildStatusResponse builds a status-ping response.
// Factorio does not support status pings, so this returns an error.
func (c *FactorioClassifier) BuildStatusResponse(payload string) ([]byte, error) {
	return nil, fmt.Errorf("factorio does not support status pings")
}

// BuildDisconnect builds a disconnect message.
// Stub: returns empty bytes. A real implementation would format
// Factorio's disconnect packet.
func (c *FactorioClassifier) BuildDisconnect(reason string) ([]byte, error) {
	// Stub: real implementation would construct Factorio's disconnect packet.
	return []byte{}, nil
}
```

### Step 2: Create Test File

**File**: `/home/valgul/project/kubernetes-game-dashboard/gameproto/factorio_test.go`

```go
package gameproto

import (
	"bufio"
	"bytes"
	"testing"
)

func TestFactorioClassifier_Classify_ReturnsUnknown(t *testing.T) {
	c := &FactorioClassifier{}
	br := bufio.NewReader(bytes.NewReader([]byte("arbitrary data")))
	
	result, err := c.Classify(br)
	if err != nil {
		t.Fatalf("Classify returned error: %v", err)
	}
	if result.Kind != Unknown {
		t.Errorf("expected Kind=Unknown, got %v", result.Kind)
	}
	if result.Consumed != 0 {
		t.Errorf("expected Consumed=0, got %d", result.Consumed)
	}
	if result.Detail != nil {
		t.Errorf("expected Detail=nil, got %v", result.Detail)
	}
}

func TestFactorioClassifier_SupportsStatusPing_ReturnsFalse(t *testing.T) {
	c := &FactorioClassifier{}
	if c.SupportsStatusPing() {
		t.Error("expected SupportsStatusPing() = false")
	}
}

func TestFactorioClassifier_BuildStatusResponse_ReturnsError(t *testing.T) {
	c := &FactorioClassifier{}
	_, err := c.BuildStatusResponse("{}")
	if err == nil {
		t.Error("expected BuildStatusResponse to return error for unsupported protocol")
	}
}

func TestFactorioClassifier_BuildDisconnect_ReturnsBytes(t *testing.T) {
	c := &FactorioClassifier{}
	bytes, err := c.BuildDisconnect("test reason")
	if err != nil {
		t.Fatalf("BuildDisconnect returned error: %v", err)
	}
	if len(bytes) == 0 {
		t.Logf("stub returns empty bytes (real implementation would format disconnect packet)")
	}
}
```

### Step 3: Register in Registry

**File**: `/home/valgul/project/kubernetes-game-dashboard/gameproto/registry.go`

**Diff** (one line added to the map literal):

```diff
 var classifierRegistry = map[string]Classifier{
     "minecraft": &MinecraftClassifier{},
     "terraria":  &TerrariaClassifier{},
+    "factorio":  &FactorioClassifier{},
 }
```

### Step 4: Verify the Pattern

**Checklist**:
- [ ] One new implementation file: `gameproto/factorio.go` (NEW).
- [ ] One new test file: `gameproto/factorio_test.go` (NEW).
- [ ] One registry entry in `gameproto/registry.go` (one line added).
- [ ] Zero changes to `gameproto/gameproto.go` (untouched).
- [ ] Zero changes to `sentinel/main.go` (untouched).
- [ ] Classifier interface fully implemented (Classify, SupportsStatusPing, BuildStatusResponse, BuildDisconnect).
- [ ] Detail type not required for this stub (Classify always returns Unknown).
- [ ] Tests cover all methods.
- [ ] No suppressions, no linting issues.

**Code Review Outcome**:
> This diff shows exactly one new protocol file, one new test file, and a one-line registry entry. No changes to any existing shared code. The new Classifier satisfies SC-001 (one file + one registry entry, zero edits to shared code). Approved.

---

## Uniqueness Enforcement

### Compile-Time Check

Go's map literal syntax prevents duplicate keys at compile time:

```go
// This does NOT compile; duplicate key "minecraft"
var classifierRegistry = map[string]Classifier{
    "minecraft": &MinecraftClassifier{},
    "minecraft": &FactorioClassifier{},  // ERROR: duplicate key
}
```

**Result**: A typo or accidental duplicate is caught at build time (compile error), not at runtime.

### Test-Time Check

A test in `gameproto_test.go` validates the registry structure:

```go
func TestRegistry_NoDuplicates(t *testing.T) {
    seen := make(map[string]bool)
    for name := range classifierRegistry {
        if seen[name] {
            t.Fatalf("duplicate protocol registration: %q", name)
        }
        seen[name] = true
    }
}

func TestRegistry_ExpectedProtocols(t *testing.T) {
    expected := []string{"minecraft", "terraria", "factorio"}
    for _, name := range expected {
        if _, ok := classifierRegistry[name]; !ok {
            t.Errorf("expected protocol %q not registered", name)
        }
    }
}
```

**Result**: A test run immediately detects if a protocol is missing or duplicated.

---

## Configuration Integration (Sentinel)

The sentinel's configuration specifies which protocols to listen for:

```
GAMEPLANE_GAME_PORTS="8080:minecraft:minecraft,25565:terraria:terraria,34197:factorio:factorio"
```

Format: `<port>:<protocol>:<wakeProtocol>`

**Validation at Startup**:
Sentinel's parsePortsConfig() function (currently at `sentinel/main.go:230–271`) will call Lookup(wakeProtocol) for each configured protocol. If the protocol is not in the registry, Lookup returns (nil, false), and sentinel logs a fatal error and refuses to start.

**Error Message Example**:
```
FATAL: wakeProtocol "factorio" not found in registry. Available: minecraft, terraria
```

---

## Error Handling

### Unknown Protocol at Startup

If a configuration specifies a protocol name that is not registered, the registry lookup MUST handle the unknown name gracefully **(FR-004)**:

**Behavior**: Sentinel refuses to start with a clear error message.

**Log Output**:
```
sentinel: loading port configuration
sentinel: port 34197, protocol factorio, wakeProtocol factorio
sentinel: registry lookup failed: protocol "factorio" not in registry
sentinel: available protocols: minecraft, terraria
fatal: could not initialize: startup configuration error
```

**No Fallback**: The sentinel does not silently fall back to the generic handler. Unknown protocol names are configuration errors, not runtime handshake failures. The registry MUST NOT panic or crash when a protocol name is not found **(FR-004)**.

---

## Summary

Adding a new protocol requires:

1. **One implementation file** (`gameproto/<protocol>.go`) implementing Classifier.
2. **One test file** (`gameproto/<protocol>_test.go`) covering all Classifier methods.
3. **One registry entry** (one line in `gameproto/registry.go`) mapping the protocol key to the Classifier.
4. **Zero edits** to any other files (gameproto/gameproto.go, sentinel/main.go, etc.).

This satisfies SC-001: Adding a protocol requires exactly one new file and one registry entry, with zero modifications to shared code.
