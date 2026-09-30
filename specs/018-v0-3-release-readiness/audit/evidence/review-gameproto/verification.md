# T045 guards chunk: independent verification (opus)

**Method.** I tried to refute each candidate in `notes.md` (written by another opus reviewer) against master `13a859ff`. `gameproto/`, `sentinel/` and `operator/api/` are the same on this branch and on master.

What I read:
- `gameproto/{gameproto,classifier,registry,demo,minecraft,terraria}.go` and `gameproto/specs.md`, in full.
- The tests each candidate cites: `terraria_test.go:170-189`, `classifier_golden_test.go:80-90` and `gameproto_test.go`.
- The only importer, `sentinel/main.go`: the constants at `:55-66`, port-config validation at `:245-275`, and `handleRegistryProtocol` at `:503-554`.
- The `WakeProtocol` field and its enum in `operator/api/v1alpha1/gametemplate_types.go:878-889`, and the generated `charts/gameplane/crds/gameplane.local_gametemplates.yaml:1398-1412`.
- The feature 005 spec (`specs/done_005-gameproto-classifier-registry/spec.md`, US1 and FR-009), to see whether it allows the "no other change" claim.
- `audit/findings.md`, which tracks none of these candidates. F-035 is about `docs/dependencies.md`, not `gameproto/specs.md`.

To confirm the runtime behaviour, I built a throwaway program in the session scratchpad, outside the repo. It imports the real module through a `replace` directive. This is not a test or lint suite, and no repo file changed. The gameproto review has no held candidates, and there is no `audit/held/review-gameproto.md`.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-gameproto-01 | rejected | n/a | The comparison `uint16 > 65535` at `terraria.go:64` can never be true, but nothing documented is broken. The spec promises `<= 65535` (`specs.md:314`) and "65535 bytes max" (`:344`), and the field's type enforces both. A full 65,535-byte frame is accepted, which the spec allows. The doc comment on `TestClassifyTerrariaFrameTooLarge` (`terraria_test.go:172`) accurately says it tests "truncated data due to claimed large size". Only its inline comment ("max int + 1") is wrong. The constant `terrariaContinueConnecting` is unused, but it sits in an enum-style `const` group, and removing it is tidying up. This is dead code with no wrong behaviour and no false contract, so it's a style issue, not a defect. |
| C-gameproto-02 | rejected | n/a | The branch can't be reached. Both callers pass at most about 32,771 bytes (`writeMinecraftString` caps the string at 32,767), so `len(data) > 0x7fffffff` can't happen, and the stale comment inside it has no effect. The harm described ("if a future caller ever reached it") is hypothetical. |
| C-gameproto-03 | kept | S4 | Confirmed. The scratch run gave `Classify([0x01,0x02])` → `(nil, "expected handshake packet id 0x00, got 0x02")`, and a Terraria type-1 frame with an empty payload → `(nil, "parse terraria ConnectRequest: empty ConnectRequest payload")`. `specs.md:295` and `:314` say both "return Unknown". The golden test at `classifier_golden_test.go:83-87` pins `expectErr: true`. The sentinel closes the connection on both outcomes (`sentinel/main.go:515-517`, `:551-552`), so the drift has no runtime effect today; it is documentation only. |
| C-gameproto-04 | kept | S4 | Confirmed. The scratch run built a 40,006-byte ConnectRequest whose version string is 40,000 bytes long. `Classify` returned `Join`, with `TerrariaDetail.Version` 40,000 bytes long. `readTerrariaString` (`terraria.go:152-173`) bounds a string only by the remaining payload (at most 65,532 bytes). `specs.md:348` states a 32 KB cap, with a derivation that is also wrong. Memory use stays bounded by the frame limit, so the only defect is the documented figure. |
| C-gameproto-05 | kept | S4 | Confirmed. `(&MinecraftClassifier{}).BuildStatusResponse("{")` returns a 4-byte framed packet and a nil error. The interface contract (`classifier.go:43-45`, `specs.md:101-103`) says it errors when the payload "is malformed or oversized". Only the size is checked (`writeMinecraftString`, `minecraft.go:238-241`). The only production caller passes a constant, valid JSON string (`sentinel/main.go:66,523`), so the error the contract promises has no effect today. |
| C-gameproto-06 | kept | S4 | Confirmed. The usage example in the package doc (`gameproto.go:29-31`) calls `registry.Lookup("minecraft")` and assigns one value. There is no `registry` package; the function is `gameproto.Lookup`, and it returns `(Classifier, bool)`, so the example doesn't compile. The consumer sentence (`:2-4`, "used by the operator's idle auto-sleep feature") is loose but defensible: auto-sleep is an operator feature, and its wake path is the sentinel, the only importer. So the example is the defect I'm keeping; the maintainer may want to reword the consumer sentence at the same time. |
| C-gameproto-07 | kept | S4 | All five mismatches confirmed. (1) `specs.md:43,379` describe `gameproto_test.go` as integration, replay and registry tests; the file has 28 lines and one test, `TestKindString`. (2) The layout tree (`:31-47`) leaves out `classifier_golden_test.go` and `demo_test.go`. (3) The dependency list (`:406-414`) leaves out `sort` (`registry.go:3`). (4) `:438` cites `test/e2e/tests/bot_*.go`, but no such directory exists; the tests are `test/e2e/minecraft_bot_e2e_test.go`, `terraria_bot_e2e_test.go` and `wake_on_connect_e2e_test.go`. (5) `:5` says "Go 1.25+", but `go.mod:3` says `go 1.26.0`. This is the same stale version as C-gameaction-04, but in a different file. |
| C-gameproto-08 | kept | S4 | Confirmed. `specs.md:277` says "That's it. No changes to sentinel/main.go, gameproto.go, or any shared code." But `GamePort.WakeProtocol` is enum-restricted to `minecraft;terraria;generic;none` (`gametemplate_types.go:886`, generated into the chart CRD at `gameplane.local_gametemplates.yaml:1407-1411`). So a newly registered classifier, or today's `demo`, can't be selected from a GameTemplate without a CRD change plus `make generate manifests`. Feature 005 (FR-009) only confined *that refactor* to `gameproto/` and `sentinel/`. It doesn't say a new protocol becomes usable without a CRD edit, so no spec allows the claim. |

### C-gameproto-03

**Location:** `gameproto/minecraft.go:74-76` and `:300-303`; `gameproto/terraria.go:88-91` and `:242-245`. The claims are at `gameproto/specs.md:295` and `:314`.

**Repro / observation:**
1. Read `gameproto/minecraft.go:69-76`. A packet ID other than `0x00` returns `Unknown, nil, fmt.Errorf("expected handshake packet id 0x00, got 0x%02x", …)`. `MinecraftClassifier.Classify` (`:299-303`) turns any error into `(nil, err)`.
2. Read `gameproto/terraria.go:87-91`. A ConnectRequest that fails `parseTerrariaConnectRequest` (for example, an empty payload, or a version length past the payload) returns an error. `TerrariaClassifier.Classify` (`:241-245`) turns that into `(nil, err)`.
3. In a scratch program that imports the module, `mc.Classify(bufio.NewReader(bytes.NewReader([]byte{0x01, 0x02})))` returns `<nil>, expected handshake packet id 0x00, got 0x02`. `tc.Classify(bufio.NewReader(bytes.NewReader([]byte{0x03, 0x00, 0x01})))` returns `<nil>, parse terraria ConnectRequest: empty ConnectRequest payload`. For comparison, a type-5 Terraria frame `[0x03,0x00,0x05]` does return `Kind=Unknown` with 3 consumed bytes.
4. `classifier_golden_test.go:83-87` pins `expectErr: true` for a non-handshake Minecraft frame.

**Expected** (spec text): `specs.md:295` says "The packet ID must be 0x00; any other value returns Unknown." `specs.md:314` says "all other types or parse errors return Unknown." Under the Classifier contract (`specs.md:152-155`), "returns Unknown" means a non-nil result that carries `Consumed`.

**Actual:** Both cases return an error with a nil result, and no `Consumed`. A non-error `Unknown` comes only from a non-ConnectRequest Terraria type, or from a Minecraft `next_state` other than 1 or 2. The spec should say these cases return an error. Changing the code instead would contradict the golden test.

### C-gameproto-04

**Location:** `gameproto/terraria.go:152-173` (`readTerrariaString`); the claim is at `gameproto/specs.md:348`.

**Repro / observation:**
1. Read `gameproto/terraria.go:158-165`. The only length checks are `length < 0` and `length > r.Len()`.
2. In a scratch program, build a Terraria frame of type 1 whose payload is a 7-bit length prefix (3 bytes) followed by 40,000 `v` bytes. The total frame length is 40,006, which is below 65,535. Pass it to `TerrariaClassifier.Classify`.
3. The result is `Kind=Join`, and `result.Detail.(*gameproto.TerrariaDetail).Version` is 40,000 bytes long.

**Expected** (`specs.md:348`): "Terraria: 32KB max (derived from the 7-bit-encoded int max for a single message)."

**Actual:** There is no 32 KB cap. A Terraria string is bounded only by the remaining frame payload, at most 65,532 bytes. The derivation in the spec is also wrong: a 7-bit-encoded int can represent values up to 2^31-1. The spec line should state the real bound, "the remaining frame payload".

### C-gameproto-05

**Location:** `gameproto/minecraft.go:122-135` (`buildMinecraftStatusResponse`) and `:328-332`. The contract is at `gameproto/classifier.go:38-46` and `gameproto/specs.md:96-104`.

**Repro / observation:**
1. Read `gameproto/classifier.go:43-45`: "Error: if payload is malformed or oversized, or if this protocol does not support status pings". `specs.md:101-103` says the same.
2. Read `gameproto/minecraft.go:122-135`. The payload goes straight into `writeMinecraftString`, which checks only the length (`:238-241`). Nothing checks the JSON.
3. In a scratch program, `(&gameproto.MinecraftClassifier{}).BuildStatusResponse("{")` returns 4 bytes and a nil error.
4. `sentinel/main.go:66,523` is the only production caller, and it always passes the constant `minecraftAsleepStatusJSON`, which is valid JSON.

**Expected:** The implementation rejects a malformed payload, for example with `json.Valid`, or the contract says it only rejects an oversized payload.

**Actual:** A malformed payload is framed and returned without an error. There is no effect today, because the only caller passes a constant.

### C-gameproto-06

**Location:** `gameproto/gameproto.go:27-38` (the example; the error is at `:30`). A secondary wording point is at `:2-4`.

**Repro / observation:**
1. Read `gameproto/gameproto.go:30`: `classifier := registry.Lookup("minecraft")`.
2. `grep -rn 'package registry' gameproto/` finds nothing. `Lookup` is declared in `gameproto/registry.go:22` as `func Lookup(name string) (Classifier, bool)`, in package `gameproto`.
3. So the example refers to a package that doesn't exist and assigns one value from a function that returns two. It wouldn't compile if copied.
4. Secondary: `:2-4` says the parsers "are used by the operator's idle auto-sleep feature". `grep -rln '"github.com/ValgulNecron/gameplane/gameproto"' --include='*.go' .` finds only `sentinel/main.go` outside the module. `specs.md:9` and `docs/architecture.md:290-292` name the sentinel.

**Expected:** The example reads `classifier, ok := gameproto.Lookup("minecraft")`, followed by an `ok` check, and ideally the package doc names the sentinel's wake-on-connect path as the consumer.

**Actual:** The example is invalid Go, and the consumer sentence is vaguer than the spec.

### C-gameproto-07

**Location:** `gameproto/specs.md:5`, `:31-47` (layout; the `gameproto_test.go` line is `:43`), `:379`, `:406-414` and `:438`.

**Repro / observation:**
1. `wc -l gameproto/gameproto_test.go` gives 28, and `grep -n '^func Test' gameproto/gameproto_test.go` finds only `TestKindString`. `specs.md:43` ("Public API integration tests (+ registry structure tests)") and `:379` ("Integration tests verifying Classifiers work end-to-end, including replay contract … Registry structure tests …") both describe content that is actually in `minecraft_test.go`, `terraria_test.go`, `classifier_golden_test.go` and `registry_test.go`.
2. The layout tree at `:31-47` doesn't list `classifier_golden_test.go` (559 lines) or `demo_test.go` (143 lines), although both are in `gameproto/`.
3. `gameproto/registry.go:3` is `import "sort"`. The stdlib list at `:406-414` has no `sort`.
4. `ls test/e2e/tests` fails: there is no such directory. The files `test/e2e/minecraft_bot_e2e_test.go`, `test/e2e/terraria_bot_e2e_test.go` and `test/e2e/wake_on_connect_e2e_test.go` exist. `:438` cites `test/e2e/tests/bot_*.go`.
5. `:5` says "stdlib only (Go 1.25+)", but `gameproto/go.mod:3` says `go 1.26.0`.

**Expected:** The layout, testing, dependencies and references sections describe the files that exist, and the header states Go 1.26.

**Actual:** There are five mismatches, listed above.

### C-gameproto-08

**Location:** `gameproto/specs.md:226-279` (the claim is at `:277`, and its echo at `:279`); `operator/api/v1alpha1/gametemplate_types.go:886`; `gameproto/demo.go:10-12`.

**Repro / observation:**
1. Read `gameproto/specs.md:277`: "**That's it.** No changes to sentinel/main.go, gameproto.go, or any shared code."
2. Read `operator/api/v1alpha1/gametemplate_types.go:878-889`. `WakeProtocol` has `// +kubebuilder:validation:Enum=minecraft;terraria;generic;none`. The generated CRD shipped by the chart has the same four-value `enum` (`charts/gameplane/crds/gameplane.local_gametemplates.yaml:1407-1411`).
3. The operator passes the template's `WakeProtocol` straight to the sentinel (`operator/internal/controller/gameserver_sentinel.go:436-448`). The sentinel accepts any registered name (`sentinel/main.go:259-266`). But the API server rejects a GameTemplate with `wakeProtocol: demo`, or with the name of any newly registered classifier, before it is stored.
4. On a cluster: `kubectl apply` a GameTemplate with one port whose `wakeProtocol` is `demo`. It is rejected with an `Unsupported value: "demo"` error listing the four allowed values.

**Expected:** "Adding a New Protocol" includes extending the `WakeProtocol` enum in `operator/api/v1alpha1/gametemplate_types.go` and running `make generate && make manifests` to regenerate `operator/config/crd` and `charts/gameplane/crds` (CLAUDE.md rule 7). Or the section says plainly that the listed steps only register the classifier in the sentinel, and that the CRD still gates which protocols a template can select.

**Actual:** The spec says no other change is needed. A maintainer who follows it gets a classifier that no GameTemplate can select.
