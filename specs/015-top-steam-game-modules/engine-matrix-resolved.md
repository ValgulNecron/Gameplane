# Resolved Engine, Protocol & CI Classification Matrix

**Feature**: `015-top-steam-game-modules`  
**Status**: Authoritative Reference for Phase 3-7 Implementation  
**Dependencies**: Derived from `specs/015-top-steam-game-modules/contracts/engine-matrix-contract.md` (T009) and `specs/015-top-steam-game-modules/OPEN-DECISIONS.md` (T005, T012).

---

## 1. Per-Module Resolved Configuration Table

This table is the single authoritative reference for the 26 in-scope modules.
Protocol values are drawn from: `source | telnet | websocket | battleye | satisfactory | palworld | nuclearoption | rest | cli | none`.
No module is assigned `cli` pending maintainer ruling on its semantics (T005).

| Module Identifier | Status | Real `rcon.protocol` | `consoleMode` | Declared Ports (`spec.ports`) | `spec.storage.mountPath` | `spec.capabilities.lifecycle.stop` | CI Classification | CI Classification Justification |
|---|---|---|---|---|---|---|---|---|
| `cs2` | Existing | `source` | `rcon` | `game` (27015/UDP), `query` (27015/UDP), `rcon` (27015/TCP) | `/home/steam/cs2-dedicated` | `["quit"]` | `bot-heavy` | 60Gi disk required; exceeds GitHub runner disk capacity |
| `palworld` | Existing | `palworld` | `rcon` | `game` (8211/UDP), `query` (27015/UDP), `rest` (8212/TCP), `rcon` (25575/TCP) | `/palworld` (OD 7.1) | `["Save", "DoExit"]` | `bot-heavy` | 20Gi storage + multi-GB first boot download |
| `fivem` | New | `rest` | `rcon` | `game` (30120/UDP), `http` (30120/TCP), `txadmin` (40120/TCP) | `/server-data` | `["quit"]` | `bot-heavy` | >5Gi storage, embedded database + txAdmin supervision |
| `rust` | Existing | `websocket` | `rcon` | `game` (28015/UDP), `query` (28015/UDP), `rcon` (28016/TCP), `app` (28082/TCP) | `/steamcmd/rust` (OD 7.2) | `["server.save", "server.writecfg"]` | `bot-heavy` | 10Gi storage + 4Gi memory requests |
| `project-zomboid` | Existing | `source` (OD 2) | `rcon` | `game` (16261/UDP), `rcon` (27015/TCP) | `/home/steam/Zomboid` (OD 7.3) | `["save", "quit"]` | `bot-heavy` | 15Gi storage + 4Gi memory requests |
| `team-fortress-2` | New | `source` | `rcon` | `game` (27015/UDP), `query` (27015/UDP), `rcon` (27015/TCP) | `/home/steam/tf-dedicated` | `["quit"]` | `bot-heavy` | SteamCMD download (>15Gi storage) |
| `dayz` | Existing | `battleye` | `rcon` | `game` (2302/UDP), `query` (27016/UDP), `rcon` (2306/UDP) | `/data` (OD 7.4) | `[]` (OD T086) | `bot-heavy` | 40Gi storage + >10GB SteamCMD download |
| `farming-simulator-25` | New | `rest` | `none` | `game` (10823/UDP), `web` (8080/TCP) | `/data/My Games/FarmingSimulator2025` | `["save"]` | `bot-heavy` | Headless Wine/Proton layer + >20Gi storage |
| `euro-truck-simulator-2` | New | `none` | `pty` | `game` (27015/UDP), `query` (27016/UDP) | `/home/steam/.local/share/Euro Truck Simulator 2` | `["exit"]` | `bot-heavy` | SteamCMD download (>10Gi storage) |
| `garrys-mod` | Existing | `none` (T009) | `none` | `game` (27015/UDP), `query` (27015/UDP) | `/home/gmod/server/garrysmod/data` (OD 7.5) | `[]` (T009, no capabilities) | `bot-fast` | Pre-baked container image; fits in fast CI bucket |
| `mount-and-blade-2-bannerlord` | New | `none` | `pty` | `game` (7210/UDP), `query` (7211/UDP) | `/serverdata` | `[]` (Match-based) | `bot-heavy` | SteamCMD download (>15Gi storage) |
| `terraria` | Existing | `none` | `pty` | `game` (7777/TCP) | `/opt/terraria/config` (OD 7.6) | `["exit"]` | `bot-fast` | Lightweight .NET runtime, <1Gi storage |
| `7-days-to-die` | Existing | `none` (T009) | `none` | `game` (26900/UDP), `query` (26900/UDP), `telnet` (8081/TCP) | `/home/sdtdserver/.local/share/7DaysToDie` (OD 7.12) | `[]` (T009, no capabilities) | `bot-heavy` | 55Gi combined storage requirement |
| `tmodloader` | New | `none` | `pty` | `game` (7777/TCP) | `/root/.local/share/Terraria/tModLoader` | `["exit"]` | `bot-fast` | Lightweight Terraria-based mod runtime, 4Gi storage |
| `beammp` | New | `none` | `pty` | `game` (30814/UDP), `auth` (30814/TCP) | `/server/Root` | `[]` | `bot-fast` | Standalone C++ binary, 2Gi storage, no SteamCMD |
| `ark-survival-ascended` | Existing | `source` | `rcon` | `game` (7777/UDP), `query` (7777/UDP), `rcon` (27020/TCP) | `/home/gameserver` (OD 7.7) | `["SaveWorld"]` | `bot-heavy` | 30Gi persistent storage requirement |
| `left-4-dead-2` | New | `source` | `rcon` | `game` (27015/UDP), `query` (27015/UDP), `rcon` (27015/TCP) | `/home/steam/l4d2-dedicated` | `["quit"]` | `bot-heavy` | SteamCMD download (>12Gi storage) |
| `factorio` | Existing | `source` (OD 2) | `pty` | `game` (34197/UDP), `rcon` (27015/TCP) | `/factorio` (OD 7.8) | `["/server-save"]` | `bot-fast` | Fast boot and lightweight memory/storage |
| `the-isle` | New | `source` | `rcon` | `game` (7777/UDP), `query` (7778/UDP), `rcon` (8888/TCP) | `/serverdata/TheIsle/Saved` | `["save"]` | `bot-heavy` | SteamCMD download, UE4 (>20Gi storage) |
| `dont-starve-together` | Existing | `none` | `pty` | `game` (10999/UDP), `query` (27018/UDP) | `/data` (OD 7.9) | `["c_save()", "c_shutdown(true)"]` (OD 4) | `bot-heavy` | Multi-shard master/caves overhead |
| `valheim` | Existing | `none` | `pty` | `game` (2456/UDP), `query` (2457/UDP) | `/config` (OD 7.10) | `["save"]` | `bot-heavy` | >12GB SteamCMD download on first boot |
| `satisfactory` | Existing | `satisfactory` | `rcon` | `game` (7777/UDP), `api` (7777/TCP) | `/config` (OD 7.11) | `["SaveGame"]` | `bot-heavy` | 25Gi storage + multi-GB SteamCMD download |
| `ark-survival-evolved` | New | `source` | `rcon` | `game` (7777/UDP), `query` (27015/UDP), `rcon` (27020/TCP) | `/serverdata/ShooterGame/Saved` | `["SaveWorld"]` | `bot-heavy` | SteamCMD download, cluster travel (>30Gi storage) |
| `arma-reforger` | New | `none` | `pty` | `game` (2001/UDP), `query` (17777/UDP) | `/home/steam/.local/share/ArmaReforgerServer` | `["save"]` | `bot-heavy` | SteamCMD download, Enfusion engine (>25Gi storage) |
| `hell-let-loose` | New | `source` | `rcon` | `game` (7787/UDP), `query` (27165/UDP), `rcon` (22222/TCP) | `/serverdata/HLL/Saved` | `[]` (Match-based) | `bot-heavy` | SteamCMD download (>30Gi storage) |
| `squad` | New | `source` | `rcon` | `game` (7787/UDP), `query` (27165/UDP), `rcon` (21114/TCP) | `/serverdata/Squad/Saved` | `[]` (Match-based) | `bot-heavy` | SteamCMD download (>35Gi storage) |

---

## 2. 13 New Modules Classification Summary

- **`bot-fast` (2 modules)**:
  - `tmodloader`: Terraria modding framework, 4Gi storage, fast start, lightweight CPU/RAM.
  - `beammp`: BeamNG multiplayer server, lightweight standalone C++ binary, 2Gi storage, zero external downloads.
- **`bot-heavy` (11 modules)**:
  - `fivem`, `team-fortress-2`, `farming-simulator-25`, `euro-truck-simulator-2`, `mount-and-blade-2-bannerlord`, `left-4-dead-2`, `the-isle`, `ark-survival-evolved`, `arma-reforger`, `hell-let-loose`, `squad`.
  - All require multi-GB SteamCMD game installations or complex emulation layers (Wine/Proton) exceeding standard GitHub Actions runner disk (<14GB free) and memory budgets.

## 3. Distinction Between `fastGameSet` and CI Buckets

- `fastGameSet` (`test/e2e/gamebot_helpers_e2e_test.go:29`) controls the **local / default developer filter** when `GAMEPLANE_E2E_GAMES` is unset.
- `bucket_bot_fast` and `bucket_bot_heavy` in `test/e2e/buckets.sh` control **CI runner job segmentation**.
- For the 13 new modules:
  - `tmodloader` and `beammp` are added to `fastGameSet` in `gamebot_helpers_e2e_test.go` and bucketed in `bucket_bot_fast`.
  - The 11 heavy modules are added to `heavyGameSet` in `gamebot_helpers_e2e_test.go` and bucketed in `bucket_bot_heavy` with explicit CI-exclusion justification comments.
