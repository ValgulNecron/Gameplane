# Draft inventory rows: AGT (rc.0)

Links are relative to audit/inventory.md.

| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-AGT-001 | Execute command via source RCON | agent/internal/console | agent/internal/rcon/rcon.go:128 | [p](procedures/agent.md#console-source) | TBD | untested | | [e](evidence/INV-AGT-001/) | | |
| INV-AGT-002 | Execute command via telnet RCON | agent/internal/console | agent/internal/rcon/telnet.go:1 | [p](procedures/agent.md#console-telnet) | TBD | untested | | [e](evidence/INV-AGT-002/) | | |
| INV-AGT-003 | Execute command via websocket RCON | agent/internal/console | agent/internal/rcon/websocket.go:1 | [p](procedures/agent.md#console-websocket) | TBD | untested | | [e](evidence/INV-AGT-003/) | | |
| INV-AGT-004 | Execute command via battleye RCON | agent/internal/console | agent/internal/rcon/battleye.go:1 | [p](procedures/agent.md#console-battleye) | TBD | untested | | [e](evidence/INV-AGT-004/) | | |
| INV-AGT-005 | Execute command via satisfactory RCON | agent/internal/console | agent/internal/rcon/satisfactory.go:1 | [p](procedures/agent.md#console-satisfactory) | TBD | untested | | [e](evidence/INV-AGT-005/) | | |
| INV-AGT-006 | Execute command via palworld RCON | agent/internal/console | agent/internal/rcon/palworld.go:1 | [p](procedures/agent.md#console-palworld) | TBD | untested | | [e](evidence/INV-AGT-006/) | | |
| INV-AGT-007 | Execute command via nuclearoption RCON | agent/internal/console | agent/internal/rcon/nuclearoption.go:1 | [p](procedures/agent.md#console-nuclearoption) | TBD | untested | | [e](evidence/INV-AGT-007/) | | |
| INV-AGT-008 | Execute command via REST RCON | agent/internal/console | agent/internal/rcon/rest.go:1 | [p](procedures/agent.md#console-rest) | TBD | untested | | [e](evidence/INV-AGT-008/) | | |
| INV-AGT-009 | Execute command via PTY console | agent/internal/console | api/internal/ws/attach.go:41 | [p](procedures/agent.md#console-pty) | TBD | untested | | [e](evidence/INV-AGT-009/) | | |
| INV-AGT-010 | List files and directories | agent/internal/files | agent/internal/files/files.go:37 | [p](procedures/agent.md#files-list) | TBD | untested | | [e](evidence/INV-AGT-010/) | | |
| INV-AGT-011 | Read file contents | agent/internal/files | agent/internal/files/files.go:38 | [p](procedures/agent.md#files-read) | TBD | untested | | [e](evidence/INV-AGT-011/) | | |
| INV-AGT-012 | Write or overwrite file | agent/internal/files | agent/internal/files/files.go:40 | [p](procedures/agent.md#files-write) | TBD | untested | | [e](evidence/INV-AGT-012/) | | |
| INV-AGT-013 | Upload file to server | agent/internal/files | agent/internal/files/files.go:41 | [p](procedures/agent.md#files-upload) | TBD | untested | | [e](evidence/INV-AGT-013/) | | |
| INV-AGT-014 | Download file from server | agent/internal/files | agent/internal/files/files.go:39 | [p](procedures/agent.md#files-download) | TBD | untested | | [e](evidence/INV-AGT-014/) | | |
| INV-AGT-015 | Create directory | agent/internal/files | agent/internal/files/files.go:42 | [p](procedures/agent.md#files-mkdir) | TBD | untested | | [e](evidence/INV-AGT-015/) | | |
| INV-AGT-016 | Delete file or directory | agent/internal/files | agent/internal/files/files.go:43 | [p](procedures/agent.md#files-delete) | TBD | untested | | [e](evidence/INV-AGT-016/) | | |
| INV-AGT-017 | Stream log file via WebSocket | agent/internal/logs | agent/internal/logs/logs.go:31 | [p](procedures/agent.md#logs-tail) | TBD | untested | | [e](evidence/INV-AGT-017/) | | |
| INV-AGT-018 | Download complete log file | agent/internal/logs | agent/internal/logs/logs.go:32 | [p](procedures/agent.md#logs-download) | TBD | untested | | [e](evidence/INV-AGT-018/) | | |
| INV-AGT-019 | List online players | agent/internal/players | agent/internal/players/players.go:78 | [p](procedures/agent.md#players-list) | TBD | untested | | [e](evidence/INV-AGT-019/) | | |
| INV-AGT-020 | List banned players | agent/internal/players | agent/internal/players/players.go:79 | [p](procedures/agent.md#players-banned) | TBD | untested | | [e](evidence/INV-AGT-020/) | | |
| INV-AGT-021 | Kick player from server | agent/internal/players | agent/internal/players/players.go:80 | [p](procedures/agent.md#players-kick) | TBD | untested | | [e](evidence/INV-AGT-021/) | | |
| INV-AGT-022 | Ban player from server | agent/internal/players | agent/internal/players/players.go:81 | [p](procedures/agent.md#players-ban) | TBD | untested | | [e](evidence/INV-AGT-022/) | | |
| INV-AGT-023 | Unban player from server | agent/internal/players | agent/internal/players/players.go:82 | [p](procedures/agent.md#players-unban) | TBD | untested | | [e](evidence/INV-AGT-023/) | | |
| INV-AGT-024 | List whitelisted players | agent/internal/players | agent/internal/players/players.go:83 | [p](procedures/agent.md#players-whitelist) | TBD | untested | | [e](evidence/INV-AGT-024/) | | |
| INV-AGT-025 | Add player to whitelist | agent/internal/players | agent/internal/players/players.go:84 | [p](procedures/agent.md#players-whitelist-add) | TBD | untested | | [e](evidence/INV-AGT-025/) | | |
| INV-AGT-026 | Remove player from whitelist | agent/internal/players | agent/internal/players/players.go:85 | [p](procedures/agent.md#players-whitelist-remove) | TBD | untested | | [e](evidence/INV-AGT-026/) | | |
| INV-AGT-027 | Pause game writes (quiesce) | agent/internal/quiesce | agent/internal/quiesce/quiesce.go:62 | [p](procedures/agent.md#quiesce-pause) | TBD | untested | | [e](evidence/INV-AGT-027/) | | |
| INV-AGT-028 | Resume game writes (unquiesce) | agent/internal/quiesce | agent/internal/quiesce/quiesce.go:74 | [p](procedures/agent.md#quiesce-resume) | TBD | untested | | [e](evidence/INV-AGT-028/) | | |
| INV-AGT-029 | Execute stop sequence | agent/internal/lifecycle | agent/internal/lifecycle/lifecycle.go:56 | [p](procedures/agent.md#lifecycle-stop) | TBD | untested | | [e](evidence/INV-AGT-029/) | | |
| INV-AGT-030 | Run module-declared action | agent/internal/actions | agent/internal/actions/actions.go:53 | [p](procedures/agent.md#actions-run) | TBD | untested | | [e](evidence/INV-AGT-030/) | | |
| INV-AGT-031 | Retrieve live status metrics | agent/internal/status | agent/internal/status/status.go:68 | [p](procedures/agent.md#status-metrics) | TBD | untested | | [e](evidence/INV-AGT-031/) | | |
| INV-AGT-032 | List installed mods | agent/internal/mods | agent/internal/mods/mods.go:72 | [p](procedures/agent.md#mods-list) | TBD | untested | | [e](evidence/INV-AGT-032/) | | |
| INV-AGT-033 | Install mod from URL | agent/internal/mods | agent/internal/mods/mods.go:73 | [p](procedures/agent.md#mods-install) | TBD | untested | | [e](evidence/INV-AGT-033/) | | |
| INV-AGT-034 | Upload mod file | agent/internal/mods | agent/internal/mods/mods.go:74 | [p](procedures/agent.md#mods-upload) | TBD | untested | | [e](evidence/INV-AGT-034/) | | |
| INV-AGT-035 | Remove installed mod | agent/internal/mods | agent/internal/mods/mods.go:75 | [p](procedures/agent.md#mods-remove) | TBD | untested | | [e](evidence/INV-AGT-035/) | | |
| INV-AGT-036 | Report metrics via Prometheus | agent/internal/heartbeat | agent/cmd/main.go:199 | [p](procedures/agent.md#heartbeat-metrics) | TBD | untested | | [e](evidence/INV-AGT-036/) | | |

Row count: 36
