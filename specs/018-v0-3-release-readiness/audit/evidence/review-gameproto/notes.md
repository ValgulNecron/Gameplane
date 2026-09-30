# Review: gameproto

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `gameproto/specs.md`; package doc `gameproto/gameproto.go:1-40`; interface docs `gameproto/classifier.go`; `docs/architecture.md:290-294`; `README.md:144,180`; `SECURITY_AUDIT.md:154`; `operator/api/v1alpha1/gametemplate_types.go:880-889` (the `WakeProtocol` enum)

## Scope reviewed

In scope, read in full:
- `gameproto/gameproto.go`, `classifier.go`, `registry.go`, `demo.go`, `minecraft.go`, `terraria.go`
- `gameproto/specs.md`, `go.mod`, `.testcoverage.yml`
- `gameproto/gameproto_test.go`, `registry_test.go`, `demo_test.go`

In scope, read in part (to see which behaviour is pinned):
- `gameproto/minecraft_test.go`: lines 360-470 and 660-792, plus the full list of `Test*` function names
- `gameproto/terraria_test.go`: lines 150-250 and 405-584, plus the function list
- `gameproto/classifier_golden_test.go`: lines 15-230 and 300-370, plus a grep for `Unknown`/`wantErr`

The skipped parts are roundtrip and golden cases for the happy path. I didn't read them line by line.

Cross-referenced, read only in part:
- `sentinel/main.go:250-275` (wakeProtocol validation) and `sentinel/main.go:490-600` (`handleRegistryProtocol`, `handleJoin`)
- The sentinel constants at `sentinel/main.go:62-70`
- `test/e2e/` directory listing

Not done: I ran no tests (Rule 8). The only command run was `go build ./...` in `gameproto/`, which passed.

## Method

1. Compared each contract, invariant, bounded-read defence and codec description in `specs.md` with `minecraft.go` and `terraria.go`, and built concrete byte inputs for each branch.
2. Checked the replay contract (exact-read accounting) and the nil/non-nil rules for `ClassificationResult` and `Detail` on every return path.
3. Looked for branches that can't be reached, and for docs that contradict the code: the package doc, the spec layout, testing and references sections, and README/architecture.
4. Checked how the sentinel consumes `Classify` errors versus `Unknown`, to judge the impact of each drift.

## Observations (no finding)

- **Replay contract holds.** Minecraft reads exactly the length VarInt plus `length` frame bytes (`minecraft.go:45-64`). Terraria reads exactly 3 header bytes plus `totalLength-3` payload bytes (`terraria.go:46-78`). Nothing more is read from `br`. Pipelined data is pinned by `TestClassifyMinecraftPipelinedPackets` and `TestTerrariaConsumedBytesWithPipelinedData`.
- **Result and Detail rules hold.** Every non-error path returns a non-nil result. `Detail` is nil for Unknown and non-nil for Join/Status (`minecraft.go:307-320`, `terraria.go:250-263`, `demo.go:24-30`), as `specs.md:150-156` says.
- **VarInt decoding** is bounded at 5 bytes and rejects bits above bit 31 (`minecraft.go:161-189`), as `specs.md:320-323` says. The final `return` at `minecraft.go:188` can't be reached, but the compiler requires it, so it isn't reported.
- **Terraria 7-bit decoder** (`terraria.go:189-202`) is bounded at 5 bytes. Unlike .NET's `BinaryReader.Read7BitEncodedInt`, it doesn't reject a 5th byte above `0x0F`; the extra bits are silently dropped. That is harmless here, because `readTerrariaString` then rejects negative lengths and lengths beyond the remaining payload (`terraria.go:158-165`). The spec (`specs.md:325-327`) doesn't claim the stricter check.
- **No allocation depends on untrusted input beyond fixed bounds.** Minecraft allocates at most 512 bytes per frame and at most 32 KiB per string attempt. Terraria allocates at most 65,532 bytes per payload. I found no reachable panic.
- **`escapeJSONString`** (`minecraft.go:263-290`) always emits valid JSON. Invalid UTF-8 becomes U+FFFD through `range`. `TestJSONEscaping` pins a round-trip through `encoding/json`.
- **The registry** is a bare map literal with `Lookup`/`ListRegistered`, exactly as `specs.md:187-224` describes.
- **The sentinel handles a `Classify` error and `Kind == Unknown` the same way**: it closes without waking or replying (`sentinel/main.go:513-517`, `:551-552`). So C-gameproto-03 has no runtime effect.
- **Error wrapping**: parse errors are wrapped with `%w`. A bare `io.EOF` or `io.ErrUnexpectedEOF` on the first read is passed through unwrapped on purpose (`minecraft.go:47-49`, `terraria.go:47-49`).
- **The status payload is a constant**: the sentinel always passes `minecraftAsleepStatusJSON` (`sentinel/main.go:66`), which is valid JSON. So C-gameproto-05 has no runtime effect either.
- **Dead only in production**: the `!ok` branch in `readMinecraftString` (`minecraft.go:224-227`) can't be reached from production, because the only caller passes a `*bytes.Reader`. It is deliberately exercised by `TestReadMinecraftStringNonIOReader`, so it isn't reported.
- **Unreachable string bound**: `readMinecraftString`'s 32,767-byte bound (`minecraft.go:219`) can never trigger inside a 512-byte frame. This is harmless, and `specs.md:295` states it only as the protocol limit.
- **Misplaced invariant**: `specs.md:333` ("Only JOINED depth satisfies a covered-* status") is test-coverage vocabulary sitting inside the classification invariants. It doesn't contradict the code.
- **Accurate docs**: `README.md:144` and `docs/architecture.md:290-294` describe gameproto correctly as a sentinel dependency. The coverage gate is 90 in both `.testcoverage.yml` and `CLAUDE.md`.

held candidates: 0 (see OD-019)

## Candidate findings

### C-gameproto-01: Terraria's maximum-frame-size check can never fire (`uint16 > 65535`), and the test named for it passes only because of truncation

- **Location**: `gameproto/terraria.go:64-66` (check), `gameproto/terraria.go:57` (`totalLength` is `uint16`), `gameproto/terraria.go:23`; test `gameproto/terraria_test.go:173-189`; spec `gameproto/specs.md:314`, `:344`
- **Category**: dead-code
- **Suggested severity**: S4
- **Observation / repro**:
  1. `totalLength := binary.LittleEndian.Uint16(header[:2])` is at most 65,535. `terrariaMaxPacketSize` is 65,535. So `if totalLength > terrariaMaxPacketSize` is always false, and its error return can't be reached.
  2. `TestClassifyTerrariaFrameTooLarge` sends length `0xFFFF` with a comment saying "Frame size = max int + 1 (way too large)". In fact `0xFFFF` is exactly the maximum the check allows. The test gets its expected error only because `io.ReadFull` of 65,532 payload bytes hits EOF. A complete 65,535-byte frame is accepted.
  3. Related dead code in the same file: the constant `terrariaContinueConnecting` (`terraria.go:16`) is referenced nowhere, in production or tests. `terrariaPasswordRequired` (`:17`) is used only by tests.
- **Expected**: `specs.md:314` says the parser should "validate length (>= 3, <= 65535)", and `:344` describes a Terraria "65535 bytes max" defence. That implies a check that can reject input, and a test that exercises it.
- **Actual**: The upper bound comes only from the field's type. The explicit check and its test are both no-ops. Either remove them (and describe the bound as inherent to the type), or lower the cap below 65,535 and test with a complete oversized frame.

### C-gameproto-02: `frameMinecraftPacket`'s oversize branch can't be reached, and its comment describes behaviour the code doesn't have

- **Location**: `gameproto/minecraft.go:248-259` (the branch is at `:251-255`)
- **Category**: dead-code
- **Suggested severity**: S4
- **Observation / repro**:
  1. Both production callers (`buildMinecraftStatusResponse`, `buildMinecraftLoginDisconnect`) frame an `inner` buffer made of a 1-byte packet ID plus a string. `writeMinecraftString` has already capped that string at 32,767 bytes (`minecraft.go:239-241`), so `len(data)` is at most about 32,771. `dlen > 0x7fffffff` can never be true.
  2. The comment at `:252-253` says "return empty to signal error". The code doesn't return empty. It clamps `dlen` to `0x7fffffff`, then writes all of `data`, which would produce a frame whose length prefix disagrees with its body.
- **Expected**: No unreachable branch, or at least a comment that matches what the branch does.
- **Actual**: The branch is unreachable and mis-documented. If a future caller ever reached it, it would silently emit a malformed frame instead of signalling an error.

### C-gameproto-03: The spec says a non-0x00 Minecraft packet ID, or a malformed Terraria ConnectRequest, "returns Unknown"; the code returns an error with a nil result

- **Location**: `gameproto/minecraft.go:74-76`, `gameproto/terraria.go:88-91`, `gameproto/minecraft.go:300-303`, `gameproto/terraria.go:242-245`; spec `gameproto/specs.md:295`, `:314`
- **Category**: docs-drift
- **Suggested severity**: S4. The sentinel closes on both outcomes (see Observations), so behaviour is the same.
- **Observation / repro**:
  1. Minecraft: send `[]byte{0x01, 0x02}`, a frame of length 1 with packet ID 2. `classifyMinecraftHandshake` returns `(Unknown, nil, "expected handshake packet id 0x00, got 0x02")`, and `MinecraftClassifier.Classify` turns that into `(nil, err)`. The golden case "bytes that don't match minecraft handshake" (`classifier_golden_test.go:83-87`) pins `expectErr: true`.
  2. Terraria: send a type-1 frame with an empty payload, or one whose version string length runs past the payload. `parseTerrariaConnectRequest` fails, and `Classify` returns `(nil, err)`.
- **Expected** (spec text):
  - `specs.md:295`: "The packet ID must be 0x00; any other value returns Unknown."
  - `specs.md:314`: "all other types or parse errors return Unknown."
  - Under the Classifier contract, "returns Unknown" means a non-nil result with `Consumed`.
- **Actual**: Both cases are errors with a nil result, and `Consumed` isn't available. Only a non-ConnectRequest Terraria message type, or a Minecraft `next_state` outside 1 and 2, yields a non-error `Unknown`. The spec should say "returns an error", or the code should return `Unknown` with `Consumed`.

### C-gameproto-04: The spec says Terraria strings are capped at 32 KB; there is no such cap, only "remaining payload"

- **Location**: `gameproto/terraria.go:152-173` (`readTerrariaString`); spec `gameproto/specs.md:348`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. Build a ConnectRequest whose version string is 40,000 bytes: frame length 40,000 + 3 (7-bit length prefix) + 3 (header), which is under 65,535.
  2. `TerrariaClassifier.Classify` returns `Join`, with `TerrariaDetail.Version` holding all 40,000 bytes. The only length checks are `length < 0` and `length > r.Len()` (`terraria.go:158-165`).
- **Expected** (`specs.md:348`): "String length bounds: … Terraria: 32KB max (derived from the 7-bit-encoded int max for a single message)."
- **Actual**: The bound is the remaining frame payload, up to 65,532 bytes. The memory use is still bounded, so this isn't a DoS issue, but the documented figure and its derivation are wrong.

### C-gameproto-05: The Classifier contract promises an error for a malformed status payload; the Minecraft implementation never validates the payload

- **Location**: `gameproto/minecraft.go:122-135` (`buildMinecraftStatusResponse`); contract `gameproto/classifier.go:41-45`, `gameproto/specs.md:99-103`
- **Category**: docs-drift
- **Suggested severity**: S4. The only production caller passes a constant, valid JSON string.
- **Observation / repro**:
  1. `(&MinecraftClassifier{}).BuildStatusResponse("{")` returns a well-framed packet and a nil error.
  2. The only check is the 32,767-byte length cap in `writeMinecraftString`. No golden or unit test has a malformed-payload `wantErr` case (`classifier_golden_test.go:170-226`).
- **Expected** (`classifier.go:43`, `specs.md:101`): "Error: if payload is malformed or oversized".
- **Actual**: The method errors only when the payload is oversized. Either the contract says "oversized" only, or the implementation checks the payload with `json.Valid`.

### C-gameproto-06: The package doc names the wrong consumer, and its usage example doesn't compile

- **Location**: `gameproto/gameproto.go:2-4`, `gameproto/gameproto.go:29-37`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `gameproto.go:2-4`: "Both are used by the operator's idle auto-sleep feature to classify an inbound connection". The only importer is `sentinel/main.go` (grep across the workspace). The operator doesn't import gameproto; `specs.md:9`, `README.md:144` and `docs/architecture.md:290` all correctly say sentinel.
  2. `gameproto.go:30`: `classifier := registry.Lookup("minecraft")`. The module has no `registry` package; `Lookup` is `gameproto.Lookup`. Also, `Lookup` returns `(Classifier, bool)`, so single-value assignment doesn't compile.
- **Expected**: The doc names the sentinel's wake-on-connect path, and the example is `classifier, ok := gameproto.Lookup("minecraft")` followed by an `ok` check.
- **Actual**: The doc names the wrong component, and the example code is invalid.

### C-gameproto-07: The spec's layout, testing, dependencies, references and Go-version metadata don't match the tree

- **Location**: `gameproto/specs.md:5`, `:31-47`, `:379`, `:406-414`, `:438`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:43` and `:379` describe `gameproto_test.go` as holding "Integration tests verifying Classifiers work end-to-end, including replay contract … Registry structure tests verifying all expected protocols are registered, no duplicates exist, and Lookup() works correctly." The file is 28 lines and contains only `TestKindString`. The replay tests are in `minecraft_test.go`/`terraria_test.go`, and the registry tests are in `registry_test.go`.
  2. The layout tree (`:31-47`) leaves out `classifier_golden_test.go` (559 lines) and `demo_test.go`.
  3. The Dependencies list (`:406-414`) leaves out `sort`, which `registry.go:3` imports.
  4. References (`:438`): "`test/e2e/tests/bot_*.go`". There is no `test/e2e/tests/` directory. The bot tests are `test/e2e/*_bot_e2e_test.go`, for example `minecraft_bot_e2e_test.go` and `terraria_bot_e2e_test.go`.
  5. `specs.md:5`: "stdlib only (Go 1.25+)", but `gameproto/go.mod:3` declares `go 1.26.0`. This is the same drift as C-gameaction-04.
- **Expected**: The spec's layout, testing, dependencies and references sections describe the files that exist.
- **Actual**: Five separate mismatches, listed above.

### C-gameproto-08: "Adding a New Protocol … That's it" leaves out the CRD enum, so a new classifier (and today's `demo`) can't be selected from a GameTemplate

- **Location**: `gameproto/specs.md:226-279` (specifically `:277`); `operator/api/v1alpha1/gametemplate_types.go:886`; `gameproto/registry.go:16`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:277`: "**That's it.** No changes to sentinel/main.go, gameproto.go, or any shared code."
  2. `GamePort.WakeProtocol` (`gametemplate_types.go:851`, `:889`) is declared with `// +kubebuilder:validation:Enum=minecraft;terraria;generic;none` (`gametemplate_types.go:886`). A template with `wakeProtocol: factorio` is rejected by the API server before the sentinel ever sees it.
  3. So `demo`, which is registered in the production registry (`registry.go:16`) and accepted by the sentinel's own validation (`sentinel/main.go:259-266`), can't be selected through the CRD. The worked example in `demo.go:10-12` claims it proves "zero edits to shared code", but that claim is untested end to end.
- **Expected**: The "Adding a New Protocol" steps include extending the `WakeProtocol` enum in `operator/api/v1alpha1/gametemplate_types.go` and running `make generate manifests`, which updates the CRD YAML in `operator/config/crd` and `charts/gameplane/crds` (CLAUDE.md rule 7).
- **Actual**: The spec says no other change is needed.

## Questions (not findings)

- **Minecraft handshake intent 3 (Transfer).** Minecraft 1.20.5+ (protocol 766+) defines handshake intent 3, "Transfer": a login redirected from another server with `accepts-transfers=true`. `minecraft.go:110-117`, as `specs.md:297` specifies, classifies it as `Unknown`, so the sentinel closes the connection without waking the server. The code matches the spec. Should a transfer count as a join?
- **Proxy-forwarded handshakes over 512 bytes.** BungeeCord/Waterfall legacy IP forwarding packs `host\0ip\0uuid\0properties-json` into the handshake server-address field. With skin textures included, that commonly runs past the 512-byte frame cap (`minecraft.go:17,53`), so such a connection errors and never wakes the server. Is fronting a proxied backend with the sentinel a supported topology? If it is, the cap and `specs.md:343` need revisiting.
