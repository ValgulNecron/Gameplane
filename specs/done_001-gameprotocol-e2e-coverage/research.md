# Research: Game Protocol E2E Coverage — Phase 0 Decisions

**Date**: 2026-08-16 | **Branch**: `001-gameprotocol-e2e-coverage` | **Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

This document records decisions already made during planning. Each decision is presented with rationale and rejected alternatives, backed by evidence from the codebase.

---

## Unknowns Resolved

| Planner's Question | Resolution Decision | Evidence |
|---|---|---|
| Where does the single coverage artifact live? | `docs/game-coverage.md`, replacing `PACKET_CAPTURE_NEEDED.md` at root | **Decision 3** below; `PACKET_CAPTURE_NEEDED.md` (root, ad-hoc, no gate) vs. `docs/roadmap.md` (mixes v1 planning with per-module facts) |
| What makes a join a join? | Only JOINED-depth probes count. QUERY and PARTIAL are real signal but not coverage. | **Decision 1** below; spec FR-006 explicitly rules out "a query/status probe alone" |
| How is a heavy test excluded without failing the bucket gate? | Exclude from `test/e2e/buckets.sh` with a comment; the verifier gate (CI job) permits such exclusions. | **Decision 4** below; plan §6.2 sketches the verifier logic; constitution I mandates "commit the test, exclude from buckets, comment why" |
| How is bit-rot detected? | The covered-deferred status record includes a last-verified date. The verifier or a CI nightly can check this against a threshold. | **Decision 5** below; edge-case requirement in spec lines 110–112 |

---

## Decision 1: Probe Depth Is the Coverage Currency

**Statement**: The existing informal `ExpectDepth` strings ("JOINED", "PARTIAL", "QUERY") in the harness become a typed value in a new package `test/e2e/internal/protocol/joindepth/`. Only probes reaching JOINED depth count toward join coverage. PARTIAL and QUERY are retained as real signal (liveness + server-reachability) but are explicitly excluded from SC-001's "100% covered" count.

**Rationale**:

The harness already declares depth informally. In `test/e2e/gamebot_helpers_e2e_test.go`, the `gameBotSpec` struct has `ExpectDepth: string` (line 133), and the E2E runner calls `RunGameProbe(t, GameProbe{ ExpectDepth: s.ExpectDepth, ... })` (line 269). In `test/e2e/gameprobe_job.go`, the Job CLI args include `-expect-depth` (line 90), and the probe binary validates this against its measured depth (line 48).

Spec FR-006 explicitly constrains the definition: "A game module MUST NOT be counted as join-tested based on a query/status probe alone". This boundary is currently enforced only by review discipline — a test with `ExpectDepth: "QUERY"` or `ExpectDepth: "PARTIAL"` can still be mistaken for coverage by a reader.

Making the depth a typed enum in a shared, testable package turns this from a human discipline into a machine-checkable contract. The type becomes the canonical definition; callers (harness, probe CLI, verifier) all import the same `joindepth` package and must use its typed values. A string "QUERY" is too weak; a type guards against typos and enables the verifier to fail loudly if someone tries to game the system by changing a test name.

The three values map directly to the measured state of a probe:
- **JOINED**: Server accepted the protocol-level join (handshake complete, player is in-world or authenticated).
- **PARTIAL**: Server's handshake succeeded partially but was rejected at a gate (Steam auth, credentials, bandwidth). This is real signal — the game is reachable and speaking — but not a join.
- **QUERY**: Only an out-of-band query (A2S, RCON, heartbeat) succeeded; no connection was established. Real liveness, but the weakest signal.

**Alternatives considered and rejected**:

1. **A free-form boolean "joined" flag per test** — rejected because it discards the intermediate PARTIAL signal, which has strategic value for understanding blockers. "Does the server boot?" (QUERY), "does it talk at all?" (PARTIAL), and "is it playable?" (JOINED) are three distinct questions. Collapsing to two loses information an operator needs.

2. **Deriving coverage from test-name conventions** — rejected because names drift and are ambiguous. A test named `TestGameServer_TerrariaBot_Joined` looks like join coverage even if it only does a query; a test named `TestGameServer_CS2Bot_Query` is unambiguous. But if names become the spec instead of depth contracts, they will eventually diverge (test renamed but depth unchanged, or vice versa). The enum in code is the only source of truth.

3. **Trusting the per-game probe binary's exit code alone** — rejected because it couples coverage verification to 16 different probe implementations (one per game). Each probe would need to encode its own depth-to-exit-code mapping, and the verifier would need to know which probe uses which convention. A centralized type eliminates this coordination burden.

**Evidence from codebase**:
- `test/e2e/gamebot_helpers_e2e_test.go:133` — `gameBotSpec.ExpectDepth: string`
- `test/e2e/gameprobe_job.go:48` — `GameProbe.ExpectDepth: string`
- Spec FR-006 (line 141–142 in spec.md): "A game module MUST NOT be counted as join-tested based on a query/status probe alone"
- Spec edge case (lines 100–103): "The join test MUST fail loudly rather than continue reporting success against a now-invalid assumption" — depth as a type enables automated assertion of this.

---

## Decision 2: The Negative Control Runs Automatically, In-Harness

**Statement**: FR-002 requires every probe to be proven to fail against a dead address AND succeed against a real listener. Today this is a manual, one-off ritual ("trust that the probe was tested"). Decision: the shared harness (`RunGameProbe` in `test/e2e/gameprobe_job.go`) runs each game's probe a second time against a guaranteed-closed address (127.0.0.1:1, which will never have a listener), expecting failure. If the probe does not fail on the dead address, the test fails.

**Rationale**:

The project learned a hard lesson before: in `project_probe_must_fail_and_pass.md` (memory entry from 2026-07-25), "verify every probe both ways: fails on dead address AND passes on real listener; UDP Dial proves nothing (no handshake)." A bare UDP `Dial` to a closed port does not fail — UDP is connectionless and has no handshake — so a test that only checks "did we dial without error?" is meaningless.

The only proof that a UDP probe actually works is:
1. It succeeds against a real listener (the live server).
2. It fails against an address with nothing listening (the negative control).

Without both, a probe that always returns "success" would pass the test and never catch a real protocol regression.

Currently this discipline is enforced by review and documentation (PACKET_CAPTURE_NEEDED.md mentions "prove the probe works"; the sourceproto/spec.md acknowledges "measured evidence" from real servers). But without an automated harness-level gate, the negative control can be skipped, and a weak probe can ship.

Making the negative control structural and automatic in the shared harness ensures every game's probe is proven on both sides without special code. A probe that cannot fail is a test failure, not an oversight. The harness will:
1. Run the probe against the real server (game Service in its actual namespace).
2. Run the same probe against 127.0.0.1:1 or another guaranteed-closed port.
3. Fail the test if either outcome is wrong (not JOINED on real server, or not failed on dead address).

A sub-second dial against a closed port adds negligible time (no server boot needed; the port is already known to be closed).

**Alternatives considered and rejected**:

1. **A one-off manual verification recorded in a comment** — rejected because comments rot. A comment saying "probe was tested both ways" becomes invisible in code review and is the first thing deleted when someone refactors. An automated gate catches violations the instant they are introduced.

2. **A separate, opt-in negative-control test** — rejected because it drifts out of sync. A separate test file (`<game>_bot_negative_e2e_test.go`) would be easy to forget, skip in CI, or delete as stale. Having a single test per game that internally performs both the positive and negative check keeps them synchronized.

3. **Relying on code review** — rejected because code review cannot catch a probe that is structurally correct but logically inverted (probe returns success on all inputs including dead addresses). An automated gate enforces the invariant; review alone cannot.

**Evidence from codebase**:
- `test/e2e/gameprobe_job.go:64–156` (`RunGameProbe`) — currently runs the Job once, synchronously. The decision extends this to run a second identical Job against a dead address in the same test.
- `test/e2e/gamebot_helpers_e2e_test.go:263–271` — the harness calls `RunGameProbe` once per game, with `ExpectDepth`. The decision adds a second call with `-expect-fail` mode (or similar) and a dead address.
- Memory: `project_probe_must_fail_and_pass.md` — "UDP Dial proves nothing (no handshake); a negative control against a closed UDP port must therefore assert on the probe's protocol verdict, not on the dial succeeding."

---

## Decision 3: One Tracked Artifact: `docs/game-coverage.md`, Superseding `PACKET_CAPTURE_NEEDED.md`

**Statement**: The single source of truth for join-coverage status is `docs/game-coverage.md` (new file). It lists all 16 shipped game modules with their status (covered-in-ci, covered-deferred, blocked-doc, or out-of-scope-by-design) and relevant detail. The root-level `PACKET_CAPTURE_NEEDED.md` is deleted — its content migrates into `docs/game-coverage.md` (blockers, diagnostics) and per-module `specs.md` files (protocol detail).

**Rationale**:

Today coverage tracking is scattered:
- `PACKET_CAPTURE_NEEDED.md` (root) lists 8 games that need packet captures, with detailed blocker notes. It is an ad-hoc markdown at the root of the repo with no CI gate and no owner. It has drifted before (per memory notes, it was last edited but not reviewed in prior context-reset sessions).
- `docs/roadmap.md` (line 240–244) has a throwaway bullet: "Bot-testing every shipped game" under "Explicitly out of scope for v1", with a brief mention that only Minecraft and Terraria have clients. This mixes v1 planning with per-module status and doesn't track individual blocker detail.
- `test/e2e/buckets.sh` (lines 143–157) has a `bucket_bot_heavy()` function listing 12 games that are excluded from CI, but it doesn't explain why — no comments.

Spec SC-004 requires: "A maintainer can determine full join-coverage status for every game module by reading a single artifact, in under 5 minutes, without searching git history, chat logs, or prior session memory." No single artifact today satisfies this.

Decision: `docs/game-coverage.md` becomes the canonical coverage registry. It lives in `docs/` because:
- `docs/` is mirrored selectively to the `gameplane-website` submodule, so the page can be published to the user-facing website if desired.
- It is a user-facing artifact, not infrastructure or code.
- It is in version control and thus reviewable in PRs.
- It is discovered by readers looking for documentation (not a root-level ad-hoc .md).
- It can have an owner (the release crew or maintainer) and a standard review gate.

The artifact records per-module status and, if blocked, the blocker and its class (documentation gap vs. architectural constraint). Per-game protocol detail stays where it already lives — `test/e2e/internal/<game>/spec.md`, co-located with that game's probe client — and the artifact indexes those files rather than duplicating them (see Decision 7). PACKET_CAPTURE_NEEDED.md is deleted; its guidance migrates into the matching per-game spec and the coverage doc.

**Alternatives considered and rejected**:

1. **Keeping `PACKET_CAPTURE_NEEDED.md` at the root** — rejected because root-level ad-hoc markdown has no CI gate and drifts silently. Operators don't discover it unless they already know it exists.

2. **A subsection inside `docs/roadmap.md`** — rejected because it mixes v1 planning (broad scope decisions like "we're not hiring cloud ops") with per-module status (specific facts about game X's blocker). These serve different readers: roadmap readers want strategic direction, coverage readers want a fact table. Collocating them makes both harder to use.

3. **A docs/superpowers/specs/ design doc** — rejected because `docs/superpowers/` is Gameplane-internal (developer guidelines for AI agents), not user-facing. Coverage status is something an operator or contributor needs to read and an external user might want to know.

4. **A machine-only YAML with no human page** — rejected because Spec SC-004 requires a human-readable, 5-minute-scan artifact. YAML is machine-friendly but not human-friendly; a structured markdown table is both.

**Evidence from codebase**:
- `PACKET_CAPTURE_NEEDED.md` at root (lines 1–184) — ad-hoc coverage of 8 games, detailed per-game blocker notes, no CI gate.
- `docs/roadmap.md:240–244` — terse line "Bot-testing every shipped game" under out-of-scope, no per-module detail.
- `test/e2e/buckets.sh:143–157` — heavy games list with only one comment (lines 138–142) explaining why they exist, not why each game is there.
- Spec SC-004 (line 180–182 in spec.md): "A maintainer can determine full join-coverage status ... by reading a single artifact, in under 5 minutes."

---

## Decision 4: The Artifact Is Verified by CI, Not Maintained by Discipline

**Statement**: A new CI gate `test/e2e/joincoverage.sh` cross-checks: (1) every directory under `modules/` is listed exactly once in `docs/game-coverage.md`; (2) every module claimed "covered-in-ci" has a JOINED-depth test in a bucket that CI runs by default (`bot-fast` only); (3) every "covered-deferred" module has a committed test and a commented exclusion in `buckets.sh`; (4) every module with status "blocked-doc" or "out-of-scope-by-design" names its blockerClass and records evidence. The verifier fails loudly if `modules/` is an uninitialized submodule (reports "0 modules, all covered" rather than silently passing). It runs in the existing "e2e bucket coverage" job in `.github/workflows/ci.yaml`.

**Rationale**:

Manual maintenance of a coverage artifact is where the rot happens. If a maintainer adds a new game to `modules/` and forgets to add it to `docs/game-coverage.md`, or moves a test between buckets without updating the doc, or deletes a game from modules but leaves a stale entry in the doc, these drift silently until someone eventually notices.

A machine-checkable gate catches these in CI the moment they happen. The verifier is a static consistency check (shell script reading files, no cluster required), so it is cheap to run and can fail fast without depending on the full E2E suite.

The verifier's logic:
1. **Module inventory**: Read every directory name under `modules/` and compare against entries in `docs/game-coverage.md`.
   - Fail if a module in `modules/` is not in the doc.
   - Fail if the doc lists a module not in `modules/`.
   - Fail if a module appears twice in the doc.
   - Fail if `modules/` is an uninitialized submodule (check for `.gitmodules` marker or a specific signal).

2. **Bucket coverage**: For each module claimed "covered-in-ci" in the doc, verify a JOINED-depth test exists in `bot-fast` (the default CI bucket that always runs).
   - Read `test/e2e/buckets.sh`, extract the bot-fast bucket list.
   - For each claimed covered-in-ci module, grep for its corresponding JOINED test name in the suite.
   - Fail if the test doesn't exist or is in a different bucket.

3. **Deferred coverage**: For each module claimed "covered-deferred", verify (a) a test exists in the suite, and (b) the test is listed in `buckets.sh`'s `unbucketed()` function or in a non-default bucket with a comment explaining why.
   - Grep for the test; fail if not found.
   - Check `buckets.sh` for an exclusion comment; warn if missing (could still be enforced).

4. **Blocker recording**: For each module with status "blocked-doc" or "out-of-scope-by-design", verify the blocker reason is recorded and its blockerClass is exactly one of the two known values (documentation, architectural). There is no third, catch-all class — an unclassifiable blocker is a signal the investigation is incomplete, not a new vocabulary entry.
   - Regex the markdown for the blocker field; fail if missing.

5. **Submodule check**: If `modules/` is a git submodule and is uninitialized (empty directory or contains only `.gitmodules`), fail with "submodule not initialized; run `git submodule update --init`".
   - This catches a common CI pitfall where the submodule was not cloned into the runner.

The verifier is deterministic and self-healing: if a module is added to `modules/`, the CI job fails, the developer adds it to the doc, and the job passes. No human remembers to maintain the list; the machine enforces it.

**Alternatives considered and rejected**:

1. **A Go test that reads the doc and `modules/`** — rejected because it would need the `e2e` build tag and thus a cluster to run. The verifier is a static check with no cluster required, so it can run in the lint tier (lower latency, cheaper) and fail faster.

2. **Generating the markdown from a YAML source** — rejected because it introduces a two-artifact problem: the YAML and the doc must be kept in sync, and whichever is hand-editable (probably the YAML) becomes the real source of truth, making the markdown decorative. Spec SC-004 requires a single artifact; generating from a source violates that.

3. **Manual review** — rejected because it doesn't scale. Every PR must manually verify the four checks above (covered-in-ci, covered-deferred, blocked-doc/out-of-scope-by-design, and submodule init); easy to miss, and the burden grows as the game count grows. An automated gate is more reliable.

**Evidence from codebase**:
- Spec SC-004 (line 180–182): "verifiable by reading a single artifact"
- Plan §4 (lines 195–198 in plan.md): describes the verifier in abstract; also see plan §6.4 (lines 152–153): "coverage verifier + artifact... CI wiring".
- Constitution I (lines 29–39 in constitution.md): "A heavy test that exists but never runs in CI is still runnable on demand... and stays exempt from the 'e2e bucket coverage' CI gate."
- Plan §7.1: existing `e2e bucket coverage` CI job already runs `buckets.sh verify` (line 8 in buckets.sh documents this).

---

## Decision 5: Four-Value Status Vocabulary

**Statement**: The coverage status has four values:
- **covered-in-ci**: Test in `bot-fast` bucket, executes on every default CI run, JOINED depth.
- **covered-deferred**: Test exists and is JOINED depth, but excluded from every CI bucket (runs on demand only).
- **blocked-doc**: Blocker is a documentation gap (missing packet capture, incomplete reverse-engineering) — temporary, an unblocking action exists.
- **out-of-scope-by-design**: Blocker is architectural (anti-cheat, platform-only relay) — permanent, cannot be resolved by code or docs.

These four are exhaustive and mutually exclusive. Every module is in exactly one state.

**Rationale**:

Spec FR-008 requires: "A module blocked by an architectural constraint... MUST be marked out-of-scope-by-design... rather than left in an indefinitely 'pending' state." This is distinct from a documentation gap, which has an unblocking path.

The two must be separate because they signal different things to a reader and to a release manager:
- **blocked-doc** (e.g., "Rust: need packet capture of join handshake"): a release manager sees a to-do item with a clear unblocking action. It is temporary; when the packet is captured, the status changes to covered-deferred or covered-in-ci.
- **out-of-scope-by-design** (e.g., "Valheim: joins through Steam Datagram Relay; a headless client cannot authenticate"): a release manager sees a permanent constraint. It is not a bug or an open item; it is an accepted limitation of the platform. It does not belong on a v1 "still-to-do" checklist.

Collapsing the two into a single "blocked" status would conflate "we don't have X yet" with "we never can have X," which leads to perpetual open items, roadmap drift, and release criteria confusion.

Spec edge case (lines 110–112): "What happens to a heavy-module test that is excluded from CI and never run for a long stretch — how is bit-rot detected before it's needed?" Decision: covered-deferred modules carry a `last-verified` date in the artifact. The verifier can be extended to warn (or fail) if a last-verified date is stale beyond a threshold (e.g., 6 months). This is a future enhancement but is enabled by the four-value model.

Why separate covered-deferred from covered-in-ci (not just "covered"): SC-001 requires "100% of shipped game modules have a committed, real protocol-join E2E test." This counts both in-CI and deferred as "covered" at the test-existence level. But a reader scanning the artifact wants to know which modules are regularly tested and which are on-demand only — that distinction is operationally important. Separating them makes that visible.

**Alternatives considered and rejected**:

1. **A single "blocked" status with a comment distinguishing documentation vs. architectural** — rejected because comments are not machine-read. The distinction between temporary and permanent is not queryable by automated tools or clearly visible at a glance. Separate status values are more declarative.

2. **A three-value system (covered-in-ci, covered-deferred, blocked)** — rejected because it loses the architectural vs. documentation distinction. Spec FR-008 explicitly requires distinguishing them, so the minimum is four distinct status values. A three-value system would have to use a separate blockerClass field or comments to separate blocked-doc from out-of-scope-by-design, which reintroduces the queryability problem.

3. **A boolean "blocked" plus a free-form text reason** — rejected because it is less queryable. The verifier needs to check "did a module marked as out-of-scope actually have an architectural blocker?" A structured type (four distinct statuses plus typed blockerClass) is easier to validate than free-form text.

**Evidence from codebase**:
- Spec FR-008 (line 146–150): "must be marked out-of-scope-by-design... rather than left in an indefinitely 'pending' state."
- Spec edge case (lines 110–112): bit-rot via stale deferred tests.
- PACKET_CAPTURE_NEEDED.md (lines 116–125): example architectural blocker (Valheim: SDR); example documentation blocker (Factorio: no public spec, but capture is the unblocking action).
- Plan §3 (lines 99–122 in plan.md): describes the three-way partition (Reachable, Blocked on documentation, Architecturally blocked); the decision formulates this as a four-value type.

---

## Decision 6: Existing QUERY-Depth Tests Are Kept but Renamed, Not Deleted

**Statement**: The 12 games currently tested only at QUERY depth (via A2S or RCON) are retained in the suite. They are valuable liveness signals — "does the server start and accept a basic query?" — but are not join coverage. Their test names are renamed to make the depth unambiguous (e.g., `TestGameServer_ArkBot_Query` instead of a name that could be misread as `TestGameServer_ArkBot_Joined`). The verifier refuses to count them toward SC-001 join-coverage.

**Rationale**:

Deleting the QUERY tests would lose real signal. A QUERY test that passes proves the server booted and is network-reachable; it catches bugs in the image, the service plumbing, or the operator reconciliation. These are valuable checks, just not join-coverage checks.

The risk is that a reader of the test suite (or a future contributor adding a test) might see a name like `TestGameServer_FactorioBot` and assume it tests a join, when in fact it only checks a query. This is a test-naming hygiene problem, not a test-quality problem.

Solution: rename the test to include the depth in the name (e.g., `TestGameServer_FactorioBot_Query`). The depth becomes part of the public contract of the test. Any reader scanning the suite can immediately see which tests are join-level and which are shallower. The verifier then refuses to count `*_Query` tests toward join coverage, even if someone later adds `ExpectDepth: "QUERY"` to the wrong test — the name and the depth must match, and the verifier enforces this.

**Alternatives considered and rejected**:

1. **Delete the QUERY tests** — rejected because it loses the liveness signal. An operator preparing a release wants to know "can this game boot?" as well as "can this game be joined?" The first is valuable even if the second is blocked.

2. **Leave names as-is and hope for careful reading** — rejected because SC-004 requires clarity at a glance ("under 5 minutes"). A reader must not have to wade through test code to determine whether a test is join-coverage or not. Spec SC-004 and Constitution IV both emphasize unambiguous communication.

3. **Use a comment to mark them** — rejected because comments are ignored by automated tools and invisible to many readers. The verifier needs to check depth; a name is more reliable than a comment.

**Evidence from codebase**:
- `test/e2e/buckets.sh:143–157` (`bucket_bot_heavy()`) lists 13 games with test names ending in `_Query` (e.g., `TestGameServer_ArkBot_Query`), all at line 145+ — these are exactly the QUERY-depth tests that need renaming.
- Some fast-path tests already follow the depth-in-name convention: `TestGameServer_MinecraftJavaBot_Joined` (line 129), `TestGameServer_TerrariaBot_Joined` (line 130). This naming pattern should be applied consistently.
- Spec SC-001 (line 173–174 in spec.md): "100% of shipped game modules have a committed, real protocol-join E2E test" — the emphasis on "real protocol join" signals that QUERY tests are distinct.

---

## Decision 7: Per-Game Wire-Protocol Records Exist; Feature Extends and Indexes Them

**Statement**: The per-game wire-protocol record already exists at `test/e2e/internal/<game>/spec.md` for all 16 shipped games (verified: 7-days-to-die, ark-survival-ascended, cs2, dayz, dont-starve-together, enshrouded, factorio, garrys-mod, minecraft-java, palworld, project-zomboid, rust, satisfactory, terraria, valheim, v-rising). These files are co-located with each game's probe client code — the correct home, since a protocol record belongs next to the code that speaks the protocol.

This feature does not create new spec locations; instead, it EXTENDS the 16 existing `test/e2e/internal/<game>/spec.md` files with a status header (status, depth, blocker, blockerClass, last-verified), and creates an index of them in `docs/game-coverage.md`.

Additionally, `gameproto/` and `sentinel/` are Go modules in the workspace that are missing their own `specs.md` files; they are added as part of this feature, as required by Constitution IV.

The main coverage doc (`docs/game-coverage.md`) stays scannable (5 minutes); protocol depth details remain in the test directory where they belong.

**Rationale**:

Constitution IV (lines 87–94 in constitution.md): "Every module folder... MUST maintain a `specs.md` describing in detail how that module actually works: its responsibilities and boundaries, the protocols or contracts it implements... `specs.md` MUST be updated in the same change that alters the behavior it documents."

The reason this principle exists is: "this repo has repeatedly lost work to agents re-deriving intent from scratch mid-session or drifting from what was actually agreed. A written spec is the artifact that survives context resets and hand-offs."

Game protocols are particularly vulnerable to this. Each game's wire protocol is often undocumented upstream (PACKET_CAPTURE_NEEDED.md, line 10, documents "Zero documentation" for 8 games), so the join protocol must be reverse-engineered from packet captures or empirical testing. Without a written spec per game, that reverse-engineering work is scattered across commit messages, PRs, meeting notes, and code comments. The next time someone picks up the work (weeks or months later), they re-derive it from scratch.

The 16 existing `test/e2e/internal/<game>/spec.md` files already document the protocol (handshake sequence, packet formats, known versions, authentication gates, dependencies, test coverage) and are kept in sync with the code because they sit next to the probe implementation. This feature extends those records with operational metadata: coverage status, join depth reached, blocker detail (if any), and last-verified date.

For blocked games (e.g., Valheim), the spec records the architectural constraint (Steam SDR) and why it is unsolvable for a headless test. This fulfills FR-005 and supports FR-008.

For deferred games (e.g., Rust), the spec records the current blocker (incomplete reverse-engineering of RakNet handshake), the unblocking action (run game client against live server, capture join handshake), and the last-verified date.

For in-CI games (Minecraft, Terraria), the spec documents the protocol version, the CI test reference, and the last-verified date.

**Alternatives considered and rejected**:

1. **Creating new `modules/<game>/specs.md` files** — rejected because it would duplicate an existing record (the protocol spec already exists at `test/e2e/internal/<game>/spec.md`) and split the protocol truth across two repositories (modules/ is a git submodule). The single source of truth must be next to the code that implements the protocol.

2. **A separate `docs/protocols/` directory tree** — rejected because it duplicates the existing game protocol records and would split the protocol truth across two directories. The canonical home for a protocol spec is next to the code that speaks the protocol, not in a docs tree.

3. **Only documenting protocol for in-CI games** — rejected because blockers (architectural, documentation gaps) are exactly what must be recorded per Constitution IV. If a game is blocked on missing protocol documentation, that documentation gap is part of the game's specification.

**Evidence from codebase**:
- Constitution IV (lines 87–94): mandatory `specs.md` per module, covering protocols and contracts.
- `test/e2e/internal/*/spec.md` (all 16 games): existing protocol specifications, co-located with probe implementations.
- Plan §2 (line 171 in plan.md): "gameproto/ ... specs.md # NEW; required by Constitution IV, currently missing" (modules/<game>/specs.md are not listed as new here, suggesting the protocol specs already exist).
- Plan §4, step 4 (lines 199–201): "Per-module specs.md fan-out... recording its actual wire protocol and its blocker detail."
- PACKET_CAPTURE_NEEDED.md throughout: each game section documents known protocol detail, unknowns, and the specific capture guidance — but this content already has a home at `test/e2e/internal/<game>/spec.md`.
- **Gap out of scope**: Constitution IV also requires `specs.md` per `modules/<game>/` directory (documenting the deployment template, not the protocol). This is a real but separate gap from the protocol specs that already exist.

---

## Decision 8: Scope Boundary — What Cannot Be Planned

**Statement**: Explicitly document that:
- Reverse-engineering 11 undocumented proprietary UDP protocols is not a planning deliverable. It is a sequence of capture-measure-decode steps, each per game, and each contributing to Step 5 (Close what is closable) incrementally.
- Valheim (Steam Datagram Relay) and DayZ (BattlEye anti-cheat) are permanent architectural blockers per FR-008 and must be marked `out-of-scope-by-design` in the coverage artifact.
- Garry's Mod is the highest-confidence next candidate after Minecraft and Terraria because the challenge exchange is already decoded, the protocol version issue has measured evidence (protocol 18 expected), and `sv_lan 1` removes the Steam auth gate.

**Rationale**:

The spec promises "every module ends in exactly one defensible, verified state" (plan §3, line 36). The plan does not promise 16 live joins. Clarifying scope avoids reviewers expecting a different outcome.

**Reachable now**: Minecraft Java (joined) and Terraria (joined) have published specs and working probes.

**Blocked on documentation** (temporary): Rust, CS2, Ark, Palworld, Satisfactory, Factorio, 7-Days-to-Die, V-Rising, Project-Zomboid, Don't-Starve-Together, Enshrouded. PACKET_CAPTURE_NEEDED.md (lines 12–138) details each game's specific blocker and the unblocking action (run a live capture).

**Architecturally blocked** (permanent): Valheim (lines 116–125 in PACKET_CAPTURE_NEEDED.md) routes joins through Steam Datagram Relay; a headless client without Steam credentials cannot reach the game port directly. DayZ (lines 128–136) gates joins with BattlEye anti-cheat, which requires a real game binary, GPU driver, and display stack — impossible in a headless test.

**Garry's Mod progression**: sourceproto/spec.md (lines 109–175) documents measured evidence:
- Round 1 (2026-07-24, PR #197): Challenge exchange works. Protocol version 17 was rejected as outdated; measured real-server response: `ffffffff39game#GameUI_ServerRejectOldVersion00`.
- Round 2 (2026-07-25): Protocol version updated to 18 (Source 2013 standard). Next CI run will test this.
- Packet layout for C2S_CONNECT is still unverified (server rejected before validating fields beyond the header), but the challenge exchange and handshake parsing are proven.

This progression shows Garry's Mod is a solved-step-at-a-time problem, not an architectural blocker. The next iteration will likely pass the version gate and reveal whether the packet layout needs adjustment. This is exactly the incremental work that Step 5 is designed for.

**Alternatives considered and rejected**:

1. **Claiming 16 live joins are in-scope for Phase 1** — rejected because the plan's Assumption 3 (spec line 199): "Full coverage of every module does not mean every module ends up executing a live join in CI." The spec explicitly permits out-of-scope-by-design. False scope is worse than honest scope.

2. **Conflating "documented" and "blocked"** — rejected because Spec FR-008 requires distinguishing them. A 6-month packet-capture project has an unblocking action; an architectural blocker does not. Conflating them obscures the release readiness question ("will this be solved before v1?").

3. **Deferring the Garry's Mod decision to Phase 2** — rejected because the evidence is in-tree and the next iteration is predictable. Documenting the progression in the spec-kit artifacts (sourceproto/spec.md and the plan) ensures continuity if the next implementer is a different agent or person.

**Evidence from codebase**:
- PACKET_CAPTURE_NEEDED.md: lines 1–10 (overview), 116–137 (Valheim and DayZ architectural blockers), 12–114 (documentation-gap games with capture guidance).
- sourceproto/spec.md: lines 109–175 (measured evidence from Garry's Mod CI runs 2026-07-24/25).
- sourceproto/source.go: lines 56–81 (protocol version constants and confidence levels).
- Plan §3 (lines 99–122 in plan.md): explicit three-way partition with evidence citations.

---

## Summary

These eight decisions establish the infrastructure and contracts for the four-step execution plan. Each decision is backed by evidence from files already in the codebase and requirements already in the spec. No decision overrides Constitution IV or the spec's mandatory requirements; all eight support them.

The next phase (Phase 1) translates these decisions into the concrete data model, typed interfaces, and CI gates that make them enforceable.
