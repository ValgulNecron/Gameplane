# Contract: GameTemplate CRD & Lifecycle Mechanics

**Feature**: `014-top-steam-game-modules`  
**Contract Version**: `1.0.0`  
**Status**: Normative  

---

## 1. Port Declaration Contract

Every port defined in `spec.ports` MUST satisfy:

1. **Named Convention**: Standardized names (`game`, `query`, `rcon`, `web`, `beacon`, `txadmin`, `telnet`).
2. **Protocol Validation**: Explicit `UDP` or `TCP` protocol identifier.
3. **Collision-Free Defaults**: Default container port allocations must match the game's upstream standard to simplify client direct connections.

---

## 2. Remote Administration & RCON Contract

Supported remote administration protocols in `spec.rcon.protocol`:

| Protocol Key | Handshake / Transport | Supported Games | Example Port |
|---|---|---|---|
| `source` | Valve Source RCON (TCP) | CS2, TF2, L4D2, GMod, Squad, The Isle, ARK, HLL | `27015` |
| `battleye` | BattlEye RCON (UDP) | DayZ | `2306` |
| `websocket` | Rust WebSocket RCON (TCP/WS) | Rust | `28016` |
| `rest` | REST API over HTTP/JSON | Palworld, Satisfactory | `8212` / `7777` |
| `telnet` | Plaintext Telnet Socket (TCP) | 7 Days to Die | `8082` |
| `cli` | Stdin/Stdout PTY Console | Terraria, Factorio, BeamMP, Arma Reforger | N/A |

---

## 3. Lifecycle Stop Action Contract

For all persistent games, `spec.lifecycle.stop` MUST be declared as follows:

```yaml
lifecycle:
  stop:
    action: rcon # or "cli" or "http"
    command: "<engine save command>"
    timeoutSeconds: 30
```

Examples:
- Source / Unreal Engine: `saveworld` or `broadcast Server shutting down...; save`
- Rust: `server.save; server.writecfg`
- Factorio: `/server-save`
- Terraria: `save; exit`
- 7 Days to Die: `saveworld; shutdown`
- Don't Starve Together: `c_save(); c_shutdown(true)`
