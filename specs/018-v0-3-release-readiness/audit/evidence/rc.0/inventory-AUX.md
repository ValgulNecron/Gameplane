# Draft inventory rows: AUX (rc.0)

Links are relative to audit/inventory.md.

| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-AUX-001 | Detect join vs status on Minecraft (wakeProtocol=minecraft) | sentinel/ | sentinel/main.go:41-58 | [p](procedures/aux.md#sentinel-minecraft-status-ping) | TBD | untested | | [e](INV-AUX-001/) | | |
| INV-AUX-002 | Detect Minecraft join handshake and patch wake annotation | sentinel/ | sentinel/main.go:41-58 | [p](procedures/aux.md#sentinel-minecraft-join-wake) | TBD | untested | | [e](INV-AUX-002/) | | |
| INV-AUX-003 | Detect Terraria join attempt via gameproto classifier | sentinel/ | sentinel/main.go:41-58, gameproto/terraria.go | [p](procedures/aux.md#sentinel-terraria-generic-udp) | TBD | untested | | [e](INV-AUX-003/) | | |
| INV-AUX-004 | UDP packet-counting heuristic for generic wake detection | sentinel/ | sentinel/main.go (UDP heuristic) | [p](procedures/aux.md#sentinel-terraria-generic-udp) | TBD | untested | | [e](INV-AUX-004/) | | |
| INV-AUX-005 | Start network packet capture with BPF filter | capture-sidecar/ | capture-sidecar/cmd/main.go, internal/capture/afpacket.go | [p](procedures/aux.md#capture-sidecar-start-stop) | TBD | untested | | [e](INV-AUX-005/) | | |
| INV-AUX-006 | Stop capture and finalize PCAPNG file | capture-sidecar/ | capture-sidecar/internal/capture/writer.go | [p](procedures/aux.md#capture-sidecar-start-stop) | TBD | untested | | [e](INV-AUX-006/) | | |
| INV-AUX-007 | Download completed capture file via mTLS HTTP endpoint | capture-sidecar/ | capture-sidecar/internal/httpserver/handlers.go | [p](procedures/aux.md#capture-sidecar-start-stop) | TBD | untested | | [e](INV-AUX-007/) | | |
| INV-AUX-008 | Enforce capture size and duration limits | capture-sidecar/ | capture-sidecar/internal/capture/writer.go | [p](procedures/aux.md#capture-sidecar-start-stop) | TBD | untested | | [e](INV-AUX-008/) | | |
| INV-AUX-009 | Supervise frp (forward proxy) relay client | tunnel/ | tunnel/main.go:1-18, charts/gameplane/values.yaml:85 | [p](procedures/aux.md#tunnel-frp-blocked-candidate) | TBD | blocked | blocked candidate: needs external frp server; alternative: test operator tunnel injection via envtest (api-agent bucket) | [e](INV-AUX-009/) | | |
| INV-AUX-010 | Supervise Tailscale relay client | tunnel/ | tunnel/main.go:1-18, charts/gameplane/values.yaml:86 | [p](procedures/aux.md#tunnel-tailscale-blocked-candidate) | TBD | blocked | blocked candidate: needs Tailscale account and auth key; alternative: test operator injection | [e](INV-AUX-010/) | | |
| INV-AUX-011 | Supervise playit relay client | tunnel/ | tunnel/main.go:1-18, charts/gameplane/values.yaml:87 | [p](procedures/aux.md#tunnel-playit-blocked-candidate) | TBD | blocked | blocked candidate: needs playit.gg account and secret key; alternative: test operator injection | [e](INV-AUX-011/) | | |
| INV-AUX-012 | Accept HTTP audit events and relay to syslog collector | audit-syslog-bridge/ | audit-syslog-bridge/README.md, main.go:1-30, charts/gameplane/values.yaml:183 | [p](procedures/aux.md#audit-syslog-bridge-blocked-candidate) | TBD | blocked | blocked candidate: needs external RFC 5424 syslog receiver; alternative: test with local netcat listener (api-auth bucket) | [e](INV-AUX-012/) | | |
| INV-AUX-013 | Format and send RFC 5424 syslog records | audit-syslog-bridge/ | audit-syslog-bridge/main.go | [p](procedures/aux.md#audit-syslog-bridge-blocked-candidate) | TBD | blocked | blocked candidate: needs external syslog collector | [e](INV-AUX-013/) | | |
| INV-AUX-014 | Accept and aggregate anonymous usage telemetry reports | telemetry-receiver/ | telemetry-receiver/main.go, charts/gameplane/values.yaml:235 | [p](procedures/aux.md#telemetry-receiver-opt-in) | TBD | untested | | [e](INV-AUX-014/) | | |
| INV-AUX-015 | Expose aggregated telemetry metrics on /metrics endpoint | telemetry-receiver/ | telemetry-receiver/main.go | [p](procedures/aux.md#telemetry-receiver-opt-in) | TBD | untested | | [e](INV-AUX-015/) | | |
| INV-AUX-016 | Provide read-only Model Context Protocol (MCP) interface | mcp-server/ | mcp-server/README.md, main.go, charts/gameplane/values.yaml:400 | [p](procedures/aux.md#mcp-server-read-only-check) | TBD | untested | | [e](INV-AUX-016/) | | |
| INV-AUX-017 | List Gameplane CRDs (GameServers, GameTemplates, etc.) via MCP | mcp-server/ | mcp-server/tools.go | [p](procedures/aux.md#mcp-server-read-only-check) | TBD | untested | | [e](INV-AUX-017/) | | |
| INV-AUX-018 | Structurally enforce read-only access (no mutating methods) | mcp-server/ | mcp-server/main.go, internal/kube/client.go | [p](procedures/aux.md#mcp-server-read-only-check) | TBD | untested | | [e](INV-AUX-018/) | | |

Row count: 18
