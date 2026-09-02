# Research: Dedicated Server Modules for Top Steam Games

**Feature**: `014-top-steam-game-modules`  
**Date**: 2026-09-03  
**Status**: Completed  

---

## 1. Overview & Engine Matrix

This research defines the architectural foundation, container configurations, protocol specifications, persistence models, and security requirements for all 26 top-played Steam multiplayer games supporting user-hosted dedicated servers.

### 1.1 Complete Game Engine & Protocol Matrix

| # | Game Identifier | Display Name | Steam App ID | Engine & Platform | Default Ports & Protocols | Remote Console & Admin | Modding / Workshop |
|---|-----------------|--------------|--------------|-------------------|---------------------------|------------------------|--------------------|
| 1 | `cs2` | Counter-Strike 2 | 730 (Server) | Source 2 (Linux) | 27015 UDP (Game), 27015 UDP (Query), 27015 TCP (RCON) | Source RCON | Metamod / CS# (`game/csgo/addons`) |
| 2 | `palworld` | Palworld | 2394010 | Unreal Engine 5 (Linux) | 8211 UDP (Game), 27015 UDP (Query), 25575 TCP (RCON), 8212 TCP (REST) | REST API / Source RCON | UE4SS / Lua / Pak Mods |
| 3 | `fivem` | FiveM (GTA V MP) | N/A (CitizenFX) | fxserver / Alpine (Linux) | 30120 UDP/TCP (Game/Query), 40120 TCP (txAdmin Web) | txAdmin HTTP / RCON | CitizenFX Resources / MariaDB |
| 4 | `rust` | Rust | 258550 | Unity (Linux) | 28015 UDP (Game), 28016 TCP (WebSockets RCON), 28082 TCP (Rust+ App) | WebSocket RCON | Oxide / uMod / Harmony |
| 5 | `project-zomboid` | Project Zomboid | 380870 | Java / C++ (Linux) | 16261 UDP (Query/Direct), 16262-16272 UDP (Player Slots), 27015 UDP | Stdin CLI / In-game Admin | Steam Workshop collections |
| 6 | `team-fortress-2` | Team Fortress 2 | 232250 | Source Engine (Linux) | 27015 UDP (Game), 27015 UDP (Query), 27015 TCP (RCON) | Source RCON | SourceMod / MetaMod / Workshop |
| 7 | `dayz` | DayZ | 223350 | Enfusion / RV (Linux) | 2302 UDP (Game), 27016 UDP (Query), 2306 UDP (BattlEye RCON) | BattlEye RCON | Steam Workshop / DZConfig |
| 8 | `farming-simulator-25` | Farming Simulator 25 | N/A (GIANTS) | GIANTS Engine 10 (Wine/Linux) | 10823 UDP (Game), 8080 TCP (Web Admin Interface) | Web Admin Portal (HTTP) | GIANTS ModHub / ZIP mods |
| 9 | `euro-truck-simulator-2` | Euro Truck Simulator 2 | 1948400 | Prism3D (Linux) | 27015 UDP (Game), 27016 UDP (Query) | Server Logon Token / Console | `server_packages.sii` / Sync |
| 10 | `garrys-mod` | Garry's Mod | 4020 | Source Engine (Linux) | 27015 UDP (Game), 27015 UDP (Query), 27015 TCP (RCON) | Source RCON | Steam Workshop / FastDL / Lua |
| 11 | `mount-and-blade-2-bannerlord` | Mount & Blade II: Bannerlord | 1863440 | TaleWorlds (Wine/Linux) | 7210 UDP (Game), 7211 UDP (Query) | Stdin CLI / Dedicated XML | Bannerlord Module XMLs |
| 12 | `terraria` | Terraria | 105600 | .NET / XNA (Linux) | 7777 TCP (Game/Direct) | CLI Console / REST API | Vanilla / Config TShock |
| 13 | `7-days-to-die` | 7 Days to Die | 294420 | Unity (Linux) | 26900 UDP (Game), 26900-26902 UDP, 8081 TCP (Web), 8082 TCP (Telnet) | Telnet / Web Dashboard / A2S | Modlet XML / Unity Assets |
| 14 | `tmodloader` | tModLoader | 1281930 | .NET Core (Linux) | 7777 TCP (Game) | CLI Console | Steam Workshop / .tmod mods |
| 15 | `beammp` | BeamNG.drive (BeamMP) | N/A (BeamMP) | Torque3D Bridge (Linux) | 30814 UDP (Game), 30814 TCP (Auth/Bridge) | AuthKey Token / CLI | ZIP vehicle/map resources |
| 16 | `ark-survival-ascended` | ARK: Survival Ascended | 2430930 | Unreal Engine 5 (Linux) | 7777 UDP (Game), 27020 TCP (RCON) | Source RCON | CurseForge Automated Mod API |
| 17 | `left-4-dead-2` | Left 4 Dead 2 | 222860 | Source Engine (Linux) | 27015 UDP (Game), 27015 UDP (Query), 27015 TCP (RCON) | Source RCON | SourceMod / MetaMod / VPKs |
| 18 | `factorio` | Factorio | Standalone | Custom C++ (Linux) | 34197 UDP (Game), 27015 TCP (RCON) | RCON Protocol / Auth Token | Factorio Mod Portal API |
| 19 | `dont-starve-together` | Don't Starve Together | 343050 | Klei Engine (Linux) | 10999 UDP (Master), 10998 UDP (Caves), 27018-27019 UDP | Klei Token / Stdin Lua | Steam Workshop / `modoverrides.lua` |
| 20 | `valheim` | Valheim | 896660 | Unity (Linux) | 2456-2457 UDP (Game/Query) | In-game Admin / Steam Query | BepInEx / Harmony / Unity |
| 21 | `satisfactory` | Satisfactory | 1690800 | Unreal Engine 5 (Linux) | 7777 UDP (Game/Query), 7777 TCP (HTTPS TLS API) | HTTPS API (TLS v1.3) | SMM / Satisfactory Mod Loader |
| 22 | `the-isle` | The Isle | 412680 | Unreal Engine 4/5 (Linux) | 7777 UDP (Game), 7778 UDP (Query), 8888 TCP (RCON) | Source RCON | Steam Workshop / `Game.ini` |
| 23 | `ark-survival-evolved` | ARK: Survival Evolved | 376030 | Unreal Engine 4 (Linux) | 7777-7778 UDP (Game/Raw), 27015 UDP (Query), 27020 TCP (RCON) | Source RCON | Steam Workshop / Cluster sync |
| 24 | `arma-reforger` | Arma Reforger | 1874900 | Enfusion (Linux) | 2001 UDP (Game), 17777 UDP (Query) | Stdin CLI / Bohemia API | Bohemia Interactive Workshop |
| 25 | `hell-let-loose` | Hell Let Loose | 731790 | Unreal Engine 4 (Linux) | 7787 UDP (Game), 27165 UDP (Query), 22222 TCP (RCON) | UE4 RCON Protocol | Community Maps / Vietnam Preset |
| 26 | `squad` | Squad | 403240 | Unreal Engine 4 (Linux) | 7787 UDP (Game), 27165 UDP (Query), 21114 TCP (RCON) | Squad RCON Protocol | Steam Workshop / `Admins.cfg` |

---

## 2. Key Decisions & Rationales

### Decision 1: Upstream OCI Container Image Selection Strategy

- **Decision**: Prioritize official vendor images (e.g. Valve/CitizenFX/Factorio/BeamMP) or established, single-purpose community images (e.g., `cm2network`, `thijsvanloef`, `ich777`, `sprits`) pinned with immutable sha256 digests.
- **Rationale**: Game server images must be reproducible, security-audited, and statically verifiable via `modules/validate.py`. Pinned digests prevent upstream mutable tag breakage.
- **Alternatives Considered**:
  - *Building custom Gameplane base images for all 26 games*: Rejected because maintaining 26 bespoke upstream SteamCMD game downloaders and build pipelines introduces massive maintenance overhead compared to curated, community-standard images.

### Decision 2: Single-Pod Auxiliary Service Supervision (Clarification Q1)

- **Decision**: Package and supervise companion web management UIs (txAdmin, Farming Simulator Web Admin) and embedded database engines (SQLite, embedded MariaDB) directly within the single-pod container runtime.
- **Rationale**: Single-pod deployment aligns with Gameplane's core GameTemplate architecture, ensuring zero-configuration standalone deployments without needing multi-pod orchestration or external DB helm dependencies.
- **Alternatives Considered**:
  - *Requiring separate MariaDB/MySQL pods for FiveM*: Rejected due to complex multi-pod state management, secret wiring, and network routing in simple homelab/single-node clusters.

### Decision 3: Non-Crashing Diagnostic Idle for Unconfigured Tokens (Clarification Q3)

- **Decision**: Modules requiring third-party master server tokens (FiveM CFX key, BeamMP AuthKey, ETS2 Logon Token) must intercept missing token conditions at startup, output clear diagnostic instructions, and sleep/idle cleanly rather than entering a Kubernetes CrashLoopBackOff.
- **Rationale**: Prevents pod restart churn, log pollution, and rate-limiting penalties while giving human operators clear feedback directly in the Gameplane dashboard console.
- **Alternatives Considered**:
  - *Failing immediately (exit 1)*: Causes CrashLoopBackOff and makes reading the setup instructions difficult for novice operators.

### Decision 4: E2E Wire-Protocol Testing & Heavy Server Resource Exclusion (Principle I)

- **Decision**: Author real wire-protocol join probes for all 26 modules in `test/e2e/`. Classify servers into CI-runnable (lightweight, e.g., CS2, TF2, Terraria, Factorio, BeamMP, L4D2) and resource-heavy (e.g., ARK: Survival Ascended, DayZ, Squad, Arma Reforger requiring >8 GB RAM), annotating heavy servers with explicit CI bucket exclusions in `test/e2e/buckets.sh`.
- **Rationale**: Fulfills Constitution Principle I (mandatory protocol-level probe authorship without CI runner resource exhaustion).
- **Alternatives Considered**:
  - *Skipping E2E tests for heavy games*: Strictly prohibited by Constitution Principle I.

### Decision 5: Mandatory Module Specification Structure (Principle IV)

- **Decision**: Author a comprehensive, standardized `specs.md` inside every module directory (`modules/<name>/specs.md`) covering Purpose, Protocols, Ports, Storage Layout, Invariants, Security, and References.
- **Rationale**: Ensures complete compliance with Constitution Principle IV across all 26 top Steam dedicated server modules.

---

## 3. Storage & Security Invariants

1. **No Shadowing of Binaries**: `storage.mountPath` must strictly point to data/save/config subdirectories (`/data`, `/serverdata`, `/home/steam/gamesaves`) and never shadow entrypoint binaries or container root filesystems.
2. **UID/GID Alignment**: For containers running as non-root users (e.g. UID 1000 or 10000), `spec.security.runAsUser`, `spec.security.fsGroup`, and explicit `HOME` environment variables must be declared to ensure SteamCMD operations succeed.
3. **Save-on-Shutdown Execution**: Every template must define a `lifecycle.stop` action with the engine's native save command before terminating the pod.
