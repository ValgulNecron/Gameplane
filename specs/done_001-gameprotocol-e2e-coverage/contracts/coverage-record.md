# Coverage Record Format: docs/game-coverage.md

The single tracked artifact recording join-protocol coverage status for every shipped game module must follow this format exactly so that machine parsers (`test/e2e/joincoverage.sh verify`) can validate it, human maintainers can read it in under 5 minutes (spec SC-004), and coverage status never drifts from the code.

## Page Structure

The document must follow this structure exactly:

1. **H1 title** (required, once at the top): e.g., `# Game Server Join Protocol Coverage`
2. **Introductory paragraph** (required, max 3 lines): Explain briefly what this page tracks and how to read the table.
3. **Machine-readable table** (required, exactly one, must be the first table on the page): The coverage table (see below).
4. **Explanatory sections** (optional, after the table):
   - `## Covered (in CI)` — describes what this status means
   - `## Covered (deferred)` — describes what this status means
   - `## Blocked` — describes common blockers (documentation, architectural)
   - `## Out of Scope by Design` — describes when a module is intentionally not tested

The parser consumes ONLY the first table on the page and ignores everything after it. Trailing explanatory sections are not machine-read; they are for human understanding only.

## Machine-Readable Table: Column Order and Headers

The table must have exactly these columns in this order. The parser keys off the header row; column order is strict.

| Column | Header Text (Exact) | Cell Grammar | Notes |
|---|---|---|---|
| 1 | `Module` | backtick-wrapped directory name under `modules/` (e.g., `` `minecraft-java` ``) | MUST match a real directory name; mismatches cause verify failure. |
| 2 | `Game` | human-readable display name (e.g., `Minecraft: Java Edition`) | For human clarity only; not machine-parsed. |
| 3 | `Status` | one of: `covered-in-ci`, `covered-deferred`, `blocked-doc`, `out-of-scope-by-design` | Exactly one of these tokens; no variations. Status is case-sensitive. |
| 4 | `Depth` | one of: `JOINED`, `PARTIAL`, `QUERY`, or `—` (em-dash U+2014) | JOINED/PARTIAL/QUERY are measured depths. `—` means unknown or not applicable (e.g., blocked modules with no client yet). |
| 5 | `Test` | Go function name or `—` | E.g., `TestGameServer_MinecraftJavaBot_Joined` for a real test, or `—` if no test exists. Function name MUST be found in `test/e2e/*_bot_e2e_test.go`. |
| 6 | `Bucket` | CI bucket name or `—` | One of: `bot-fast`, `bot-heavy`, or `—`. Reflects where the test runs (if at all). CI buckets are defined in `test/e2e/buckets.sh`. |
| 7 | `Last Verified` | YYYY-MM-DD or `—` | ISO 8601 date when a test for this module last passed on a real server (or in CI). Blank/`—` means never verified or never run. |
| 8 | `Blocker` | short reason or `—` | E.g., `Missing protocol packet capture`, `Requires Steam authentication`, `BattlEye anti-cheat blocks headless clients`. MUST be present if Status is `blocked-*`. |
| 9 | `Blocker Class` | `documentation` or `architectural` or `—` | Documentation = fixable with a capture or reverse-engineering session. Architectural = a permanent constraint (anti-cheat, platform relay, encrypted transport). Present ONLY for `blocked-*` status. |

## Parsing Rules

The verifier MUST be robust to these common Markdown variations:

1. **Trailing whitespace and column padding**: Markdown table cells may have leading/trailing spaces; parsers MUST trim them per cell.

2. **Em-dash variants**: The `—` character (U+2014, em-dash) is the canonical representation of "not applicable" or "unknown". Parsers MUST accept U+2014 and treat other dashes (U+002D hyphen-minus, U+2013 en-dash) as parse failures (typo).

3. **Backtick-wrapped module names**: Module names in the first column MUST be wrapped in backticks (`` ` ``) for Markdown code formatting. The parser extracts the text between backticks; mismatched or missing backticks cause a parse error.

4. **First table only**: If the page contains multiple tables, only the first one is machine-read. Subsequent tables (e.g., in explanatory sections) are ignored.

5. **Exact header match**: The header row cells MUST match the text above exactly, character for character (case-sensitive). Misspellings (e.g., `Last Verified Date` instead of `Last Verified`) cause a parse failure.

6. **Pipe-separated rows**: Each row is delimited by pipes (`|`); the parser splits on pipes and trims each cell.

## Cell Grammar Rules

### Module column

- **Format**: `` `<directory-name>` ``
- **Valid examples**: `` `minecraft-java` ``, `` `terraria` ``, `` `garrys-mod` ``
- **Required property**: The directory name must exist in `modules/` (the git submodule). A module name not found in the submodule causes a verify failure (spec FR-007 requirement).

### Status column

- **Valid tokens** (exact): `covered-in-ci`, `covered-deferred`, `blocked-doc`, `out-of-scope-by-design`
- **Constraints**:
  - `covered-*` status REQUIRES: Depth is JOINED, Test is present (not `—`), Last Verified is a date.
  - `blocked-doc` status REQUIRES: Blocker is present (not `—`), Blocker Class is `documentation`. Depth may be `—` (no test exists) or a measured depth like QUERY if a reachability test exists.
  - `out-of-scope-by-design` status REQUIRES: Blocker is present (not `—`), Blocker Class is `architectural`. Depth may be `—` (unknown) or QUERY (if a reachability test exists but join coverage is architecturally unreachable).

### Depth column

- **Valid tokens** (exact): `JOINED`, `PARTIAL`, `QUERY`, `—`
- **Semantics**:
  - `JOINED`: server accepted the client as a player
  - `PARTIAL`: real protocol spoken, but a credential gate CI cannot mint was encountered
  - `QUERY`: only query/status protocol reachable, no login possible
  - `—`: no depth measured yet (blocked module, no test yet)
- **Constraints**:
  - A `covered-*` status with `QUERY` depth is a hard failure (spec FR-006: QUERY depth does not count as coverage). Verify must reject this.
  - A `covered-*` status requires depth JOINED only. PARTIAL proves a handshake was parsed, not that a join completed.
  - A `blocked-doc` status usually has depth `—`, but can have depth QUERY if a reachability test exists but the module is not join-covered due to documentation gaps.
  - An `out-of-scope-by-design` status usually has depth `—`, but can have depth QUERY if a reachability test exists (test exists, module is architecturally unreachable for join).

### Test column

- **Format**: Go function name (e.g., `TestGameServer_MinecraftJavaBot_Joined`) or `—`
- **Constraints**:
  - A `covered-*` status REQUIRES a test name (not `—`).
  - A `blocked-doc` status can have `—` (no client exists yet) or a test name (client exists but blocked).
  - The test name MUST be found in `test/e2e/*_bot_e2e_test.go` via grep.

### Bucket column

- **Valid tokens**: `bot-fast`, `bot-heavy`, `—`
- **Semantics**:
  - `bot-fast`: test runs in CI on every PR (default bucket, ~4 games).
  - `bot-heavy`: test exists but never runs in CI by default (deferred, may run on demand via `GAMEPLANE_E2E_GAMES=all`).
  - `—`: no test, or test exists but not bucketed (for blocked-doc and out-of-scope modules with no join test).

### Last Verified column

- **Format**: YYYY-MM-DD or `—`
- **Semantics**: ISO 8601 date the test last passed on a real server (in CI for `bot-fast`, on the operator cluster for `bot-heavy`).
- **Staleness check**: If Last Verified is older than a stated threshold (e.g., 90 days), the verifier MUST warn (not fail, but warn loudly), so a long-unrun deferred test is visible.

### Blocker column

- **Format**: short prose description or `—`
- **Length**: aim for < 60 characters; never exceed 200.
- **Examples**:
  - `Missing protocol packet capture`
  - `Requires Steam authentication`
  - `BattlEye anti-cheat blocks headless clients`
  - `Routes through Steam Datagram Relay (platform-only)`
- **Requirement**: MUST be present (not `—`) if Status is `blocked-doc` or `out-of-scope-by-design`.

### Blocker Class column

- **Valid tokens**: `documentation`, `architectural`, `—`
- **Semantics**:
  - `documentation`: a temporary gap; fixable with a packet capture or reverse-engineering session.
  - `architectural`: a permanent constraint (anti-cheat validation, platform-only transport, encrypted proprietary protocol). Modules marked `architectural` are out-of-scope-by-design per FR-008.
  - `—`: not applicable (used for covered modules).
- **Requirement**: MUST be present (not `—`) if Status is `blocked-doc` or `out-of-scope-by-design`.

## Understanding Status and Depth vs Test and Bucket

The Status and Depth columns answer the question: **Is this module join-covered by CI testing?** A module is join-covered only when Status is `covered-in-ci` or `covered-deferred` AND Depth is JOINED.

The Test and Bucket columns answer a different question: **What reachability test exists today?** A module may have a test (Test column populated, Bucket assigned) that does not constitute join coverage. For example, a QUERY-depth test proves the server is reachable and responds to a basic query/status probe, but it does not prove a headless client can complete a login handshake. Similarly, a module with Status `blocked-doc` or `out-of-scope-by-design` and Depth QUERY has a real test (showing in Test column and bucketed in Bucket column) that measures protocol reachability, but that test does not satisfy the coverage definition.

The canonical rule is: Status + Depth carry the coverage judgment; Test + Bucket carry the implementation reality. An honest record shows both: a module can have a test without being coverage-complete.

## Worked Example

This is the initial content of `docs/game-coverage.md`, showing all 16 real modules in their current (honest) state as of 2026-08-16, based on spec Assumption 3 and the plan's Scope Reality section.

```markdown
# Game Server Join Protocol Coverage

Every Gameplane-shipped game module is listed here with its join-protocol coverage status: tested in CI, deferred from CI, or blocked by a specific constraint. Read the Status and Blocker columns to understand why each module is tested (or not) the way it is. A module blocked by an architectural constraint (anti-cheat, platform relay) is marked as such rather than left as an open item. All dates are ISO 8601 (YYYY-MM-DD).

| Module | Game | Status | Depth | Test | Bucket | Last Verified | Blocker | Blocker Class |
|---|---|---|---|---|---|---|---|---|
| `minecraft-java` | Minecraft: Java Edition | covered-in-ci | JOINED | TestGameServer_MinecraftJavaBot_Joined | bot-fast | 2026-08-16 | — | — |
| `terraria` | Terraria | covered-in-ci | JOINED | TestGameServer_TerrariaBot_Joined | bot-fast | 2026-08-16 | — | — |
| `valheim` | Valheim | out-of-scope-by-design | QUERY | TestGameServer_ValheimBot_Query | bot-heavy | — | Joins route through Steam Datagram Relay; no direct-UDP join path for a headless client | architectural |
| `dayz` | DayZ | out-of-scope-by-design | QUERY | TestGameServer_DayZBot_Query | bot-heavy | — | BattlEye anti-cheat gates the join; requires a real game binary | architectural |
| `garrys-mod` | Garry's Mod | blocked-doc | QUERY | TestGameServer_GarrysModBot_Query | bot-fast | — | C2S_CONNECT field offsets unknown; challenge exchange already decoded, sv_lan 1 removes the Steam gate | documentation |
| `factorio` | Factorio | blocked-doc | QUERY | TestGameServer_FactorioBot_Query | bot-heavy | — | Wire format undocumented; server and client must match exactly (version-locked) | documentation |
| `cs2` | Counter-Strike 2 | blocked-doc | QUERY | TestGameServer_CS2Bot_Query | bot-heavy | — | Source 2 connect packet reportedly carries embedded protobuf and Steam auth ticket; incomplete public documentation | documentation |
| `rust` | Rust | blocked-doc | QUERY | TestGameServer_RustBot_Query | bot-heavy | — | RakNet wire format partially documented but implementation incomplete; anti-cheat may block headless clients | documentation |
| `ark-survival-ascended` | ARK: Survival Ascended | blocked-doc | QUERY | TestGameServer_ArkBot_Query | bot-heavy | — | Unreal Engine 5 network stack partially documented; custom protocol variant under reverse-engineering | documentation |
| `palworld` | Palworld | blocked-doc | QUERY | TestGameServer_PalworldBot_Query | bot-heavy | — | Unreal Engine 5; stateless-connect handshake under reverse-engineering, Steam authentication integration incomplete | documentation |
| `satisfactory` | Satisfactory | blocked-doc | QUERY | TestGameServer_SatisfactoryBot_Query | bot-heavy | — | Undocumented proprietary HTTPS API (not standard UDP game protocol); TLS-over-UDP encryption from byte 0 | documentation |
| `7-days-to-die` | 7 Days to Die | blocked-doc | QUERY | TestGameServer_SevenDaysToDieBot_Query | bot-heavy | — | Undocumented proprietary UDP protocol; Steam A2S query reachable but join handshake unknown | documentation |
| `project-zomboid` | Project Zomboid | blocked-doc | QUERY | TestGameServer_ProjectZomboidBot_Query | bot-heavy | — | Undocumented custom UDP protocol; optional non-Steam private server mode possible but format unknown | documentation |
| `dont-starve-together` | Don't Starve Together | blocked-doc | QUERY | TestGameServer_DontStarveTogetherBot_Query | bot-heavy | — | Klei reliable-UDP frame format and handshake not documented; server credentials required at join | documentation |
| `enshrouded` | Enshrouded | blocked-doc | QUERY | TestGameServer_EnshroudedBot_Query | bot-heavy | — | Undocumented proprietary protocol; wire format and handshake feasibility unknown | documentation |
| `v-rising` | V Rising | blocked-doc | QUERY | TestGameServer_VRisingBot_Query | bot-heavy | — | Undocumented proprietary UDP protocol; transport layer unidentified | documentation |

## Covered (in CI)

These modules have a real protocol-join test that runs in the `bot-fast` bucket on every PR. The server is booted and the protocol client completes a login handshake, observing server-originated evidence of acceptance (e.g., Minecraft Login Success packet). Depth is JOINED.

- **minecraft-java**: Login state machine via Encryption Request rejection (offline-mode). Runs ~3 minutes.
- **terraria**: TCP message frames with per-player handshake and WorldData packet. Runs ~2 minutes.

## Covered (deferred)

Currently empty. No modules are committed to deferred join coverage at this time.

## Blocked

These modules lack a join-protocol client. The reason is documented as temporary (documentation gap, needs a packet capture or reverse-engineering). Each is a candidate for future protocol work once the missing documentation or wire format is available.

### Documentation Blockers (reversible)

- **garrys-mod**: C2S_CONNECT field offsets unknown. Challenge exchange is already decoded and sv_lan 1 removes the Steam authentication gate, but the version negotiation field order cannot be derived by blind iteration — a real client capture is needed to establish the exact packet layout.
- **factorio**: Wire format undocumented. Mutual random-ID exchange is known but no further byte layout is published. Critical constraint: Factorio is version-locked (client and server must match exactly); a capture must record both versions.
- **cs2**: Source 2 connect packet reportedly carries embedded protobuf and Steam auth ticket, making it more opaque than Source 1. Incomplete public documentation; packet capture would accelerate reverse-engineering.
- **rust**: RakNet wire format is open-source and partially documented, but implementation in the project is incomplete. Anti-cheat may block headless clients once protocol is solved.
- **ark-survival-ascended**: Unreal Engine 5 network stack is partially documented. Custom protocol variant under reverse-engineering; anti-cheat may apply.
- **palworld**: Unreal Engine 5 stateless-connect handshake under reverse-engineering. Steam authentication integration incomplete for a headless client.
- **satisfactory**: Undocumented proprietary HTTPS API (not standard UDP game protocol). Uses TLS-over-UDP with encryption from byte 0, making raw packet captures unreadable without the server's key material.
- **7-days-to-die**: Undocumented proprietary UDP protocol. Steam A2S query is reachable but the join handshake format is unknown. No public protocol analysis or modding documentation covers networking.
- **project-zomboid**: Undocumented custom UDP protocol. Optional non-Steam private server mode exists but the wire format is unknown. Packet capture and analysis required.
- **dont-starve-together**: Klei reliable-UDP frame format and handshake are not documented. Server credentials are required at join but the authentication flow is undocumented.
- **enshrouded**: Undocumented proprietary protocol. Wire format and handshake feasibility are completely unknown; no public reverse-engineering exists.
- **v-rising**: Undocumented proprietary UDP protocol. Transport layer is unidentified despite community modding ecosystem. Packet capture and reverse-engineering required.

## Out of Scope by Design

These modules encounter architectural constraints that prevent a headless automated client from ever completing a join, per spec FR-008. They are recorded here not as an open item forever, but as a deliberate decision that CI cannot test them further.

- **valheim**: Joins route exclusively through Steam Datagram Relay, a platform-specific UDP relay transport. There is no direct-UDP join path for a standalone headless client; Steam authentication is mandatory at the platform level. No headless client is possible.
- **dayz**: BattlEye anti-cheat validation is gated before the login handshake completes. Even a fully correct protocol implementation cannot join without a real game binary running the anti-cheat kernel module. No headless client is possible.

```

## Verification Guarantees

The verifier (`test/e2e/joincoverage.sh verify`) checks this format and guarantees:

1. **Every module in `modules/` is listed exactly once** — no duplicates, no missing entries.
2. **Every listed module exists in `modules/`** — typos in module names fail verification.
3. **Status, Depth, Blocker, and Blocker Class are consistent** — e.g., `covered-*` requires a test and a depth; `blocked-*` requires a blocker.
4. **Every test name listed is found in the source** (`test/e2e/*_bot_e2e_test.go`).
5. **Every bucket name is valid** (from `test/e2e/buckets.sh`).
6. **QUERY-depth modules are never marked `covered-in-ci`** (spec FR-006: QUERY does not count as join coverage in CI).
7. **Last Verified dates are not stale** (warning only, not a hard fail, if > 90 days old).

## Verifier Checks and Fixtures

The verifier's enforcement rules and their corresponding test fixtures are defined authoritatively in `contracts/verifier.md` (Verification Checks section and Shell Fixture Strategy section). This document defines only the on-disk format of `docs/game-coverage.md`: page structure, column order, cell grammars, and parsing rules.

