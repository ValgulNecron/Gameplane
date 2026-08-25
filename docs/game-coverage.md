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
| `nuclear-option` | Nuclear Option | blocked-doc | QUERY | — | — | — | Undocumented proprietary UDP protocol; join handshake format unknown | documentation |

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
