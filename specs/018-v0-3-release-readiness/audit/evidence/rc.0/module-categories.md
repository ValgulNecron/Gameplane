# Module Categories (rc.0)

**Source:** Modules submodule commit `ff19983c4ac5879e25d9230e97d658dc88fc3f06`, recorded 2026-09-23.

**Definition:** A category (research R10) defines a module's capability cluster: console family (how the agent connects for console access) + join family (how a client connection is probed to detect readiness). Console family = `<consoleMode>/<rcon protocol>`. Join family identifies the wire protocol a real client uses to discover and join the server, sourced from gameproto (Minecraft/Terraria hand-rolled parsers), e2e bot probes (game-specific handshake implementations), or inferred from game port transports when no probe exists.

## Modules

| Module | consoleMode | RCON protocol | Console family | Join family | Game ports | Requests (cpu/mem) | Source files |
|--------|-------------|---------------|----------------|-------------|------------|-------------------|--------------|
| 7-days-to-die | none | none | none/none | e2e-probe:steam-a2s-udp | game (TCP:26900); game-udp (UDP:26900); game2 (UDP:26901); game3 (UDP:26902); telnet (TCP:8081) | cpu: 2, memory: 6Gi | test/e2e/internal/7-days-to-die/app.go |
| ark-survival-ascended | rcon | source | rcon/source | e2e-probe:ark-ascended-tcp | game (UDP:7777); peer (UDP:7778); rcon (TCP:27020) | cpu: 2, memory: 10Gi | test/e2e/internal/ark-survival-ascended/app.go |
| ark-survival-evolved | rcon | source | rcon/source | e2e-probe:steam-a2s-udp | game (UDP:7777); query (UDP:27015); rcon (TCP:27020) | cpu: 2, memory: 8Gi | test/e2e/internal/ark-survival-evolved/app.go |
| arma-reforger | pty | none | pty/none | e2e-probe:steam-a2s-udp | game (UDP:2001); query (UDP:17777) | cpu: 2, memory: 8Gi | test/e2e/internal/arma-reforger/app.go |
| beammp | pty | none | pty/none | e2e-probe:beammp-tcp | game (UDP:30814); auth (TCP:30814) | cpu: 1, memory: 2Gi | test/e2e/internal/beammp/app.go |
| cs2 | rcon | source | rcon/source | e2e-probe:steam-a2s-udp | game (UDP:27015); rcon (TCP:27015); gotv (UDP:27020) | cpu: 1, memory: 2Gi | test/e2e/internal/cs2/app.go |
| dayz | rcon | battleye | rcon/battleye | e2e-probe:steam-a2s-udp | game (UDP:2302); query (UDP:27015); rcon (UDP:2305) | cpu: 2, memory: 6Gi | test/e2e/internal/dayz/app.go |
| dont-starve-together | pty | none | pty/none | e2e-probe:steam-a2s-udp | game (UDP:10999); query (UDP:27018); caves (UDP:11000); steam1 (UDP:12346); steam2 (UDP:12347) | cpu: 1, memory: 1Gi | test/e2e/internal/dont-starve-together/app.go |
| enshrouded | none | none | none/none | e2e-probe:steam-a2s-udp | game (UDP:15636); query (UDP:15637) | cpu: 2, memory: 8Gi | test/e2e/internal/enshrouded/app.go |
| euro-truck-simulator-2 | pty | none | pty/none | e2e-probe:steam-a2s-udp | game (UDP:27015); query (UDP:27016) | cpu: 1, memory: 2Gi | test/e2e/internal/euro-truck-simulator-2/app.go |
| factorio | pty | source | pty/source | e2e-probe:factorio-tcp | game (UDP:34197); rcon (TCP:27015) | cpu: 500m, memory: 1Gi | test/e2e/internal/factorio/app.go |
| farming-simulator-25 | none | rest | none/rest | e2e-probe:http-rest | game (UDP:10823); web (TCP:8080) | cpu: 2, memory: 4Gi | test/e2e/internal/farming-simulator-25/app.go |
| fivem | rcon | rest | rcon/rest | e2e-probe:http-rest | game (UDP:30120); http (TCP:30120); txadmin (TCP:40120) | cpu: 1, memory: 4Gi | test/e2e/internal/fivem/app.go |
| garrys-mod | none | none | none/none | e2e-probe:steam-a2s-udp | game (UDP:27015); game-tcp (TCP:27015); client (UDP:27005) | cpu: 1, memory: 2Gi | test/e2e/internal/garrys-mod/app.go |
| hell-let-loose | rcon | source | rcon/source | e2e-probe:steam-a2s-udp | game (UDP:7787); query (UDP:27165); rcon (TCP:22222) | cpu: 2, memory: 8Gi | test/e2e/internal/hell-let-loose/app.go |
| left-4-dead-2 | rcon | source | rcon/source | e2e-probe:steam-a2s-udp | game (UDP:27015); query (UDP:27015); rcon (TCP:27015) | cpu: 1, memory: 2Gi | test/e2e/internal/left-4-dead-2/app.go |
| minecraft-java | rcon | source | rcon/source | gameproto:minecraft-java | game (TCP:25565); rcon (TCP:25575) | cpu: 500m, memory: 2Gi | gameproto/minecraft.go, test/e2e/internal/minecraft-java/app.go |
| mount-and-blade-2-bannerlord | pty | none | pty/none | e2e-probe:steam-a2s-udp | game (UDP:7210); query (UDP:7211) | cpu: 2, memory: 4Gi | test/e2e/internal/mount-and-blade-2-bannerlord/app.go |
| nuclear-option | rcon | nuclearoption | rcon/nuclearoption | udp (no probe) | game (UDP:7778); rcon (TCP:7779) | cpu: 1, memory: 2Gi | modules/nuclear-option/template.yaml |
| palworld | rcon | palworld | rcon/palworld | e2e-probe:steam-a2s-udp | game (UDP:8211); query (UDP:27015); rest-api (TCP:8212) | cpu: 1, memory: 4Gi | test/e2e/internal/palworld/app.go |
| project-zomboid | rcon | source | rcon/source | e2e-probe:steam-a2s-udp | game (UDP:16261); direct (UDP:16262); rcon (TCP:27015) | cpu: 1, memory: 4Gi | test/e2e/internal/project-zomboid/app.go |
| rust | rcon | websocket | rcon/websocket | e2e-probe:steam-a2s-udp | game (UDP:28015); rcon (TCP:28016) | cpu: 1, memory: 4Gi | test/e2e/internal/rust/app.go |
| satisfactory | rcon | satisfactory | rcon/satisfactory | e2e-probe:http-rest | game (UDP:7777); game-tcp (TCP:7777); messaging (TCP:8888) | cpu: 2, memory: 6Gi | test/e2e/internal/satisfactory/app.go |
| squad | rcon | source | rcon/source | e2e-probe:steam-a2s-udp | game (UDP:7787); query (UDP:27165); rcon (TCP:21114) | cpu: 2, memory: 8Gi | test/e2e/internal/squad/app.go |
| team-fortress-2 | rcon | source | rcon/source | e2e-probe:steam-a2s-udp | game (UDP:27015); rcon (TCP:27015); sourcetv (UDP:27020) | cpu: 1, memory: 2Gi | test/e2e/internal/team-fortress-2/app.go |
| terraria | pty | none | pty/none | gameproto:terraria | game (TCP:7777) | cpu: 200m, memory: 512Mi | gameproto/terraria.go, test/e2e/internal/terraria/app.go |
| the-isle | rcon | source | rcon/source | e2e-probe:steam-a2s-udp | game (UDP:7777); query (UDP:7778); rcon (TCP:8888) | cpu: 2, memory: 4Gi | test/e2e/internal/the-isle/app.go |
| tmodloader | pty | none | pty/none | e2e-probe:terraria-tcp | game (TCP:7777) | cpu: 500m, memory: 1Gi | test/e2e/internal/tmodloader/app.go |
| v-rising | rcon | source | rcon/source | e2e-probe:steam-a2s-udp | game (UDP:9876); query (UDP:9877); rcon (TCP:25575) | cpu: 1, memory: 4Gi | test/e2e/internal/v-rising/app.go |
| valheim | pty | none | pty/none | e2e-probe:http-rest | game (UDP:2456); game2 (UDP:2457); game3 (UDP:2458); status (TCP:80) | cpu: 500m, memory: 2Gi | test/e2e/internal/valheim/app.go |

**Total modules:** 30

## Categories

| Category (console family + join family) | Modules | Representative | Why |
|----------------------------------------|---------|-----------------|-----|
| none/none + e2e-probe:steam-a2s-udp | 7-days-to-die, enshrouded, garrys-mod | garrys-mod | 2Gi (lightest memory in category) |
| none/rest + e2e-probe:http-rest | farming-simulator-25 | farming-simulator-25 | Only module in category |
| pty/none + e2e-probe:beammp-tcp | beammp | beammp | Only module in category |
| pty/none + e2e-probe:http-rest | valheim | valheim | Only module in category |
| pty/none + e2e-probe:steam-a2s-udp | arma-reforger, dont-starve-together, euro-truck-simulator-2, mount-and-blade-2-bannerlord | dont-starve-together | 1Gi (lightest memory in category) |
| pty/none + e2e-probe:terraria-tcp | tmodloader | tmodloader | Only module in category |
| pty/none + gameproto:terraria | terraria | terraria | Only module in category |
| pty/source + e2e-probe:factorio-tcp | factorio | factorio | Only module in category |
| rcon/battleye + e2e-probe:steam-a2s-udp | dayz | dayz | Only module in category |
| rcon/nuclearoption + udp (no probe) | nuclear-option | nuclear-option | Only module in category |
| rcon/palworld + e2e-probe:steam-a2s-udp | palworld | palworld | Only module in category |
| rcon/rest + e2e-probe:http-rest | fivem | fivem | Only module in category |
| rcon/satisfactory + e2e-probe:http-rest | satisfactory | satisfactory | Only module in category |
| rcon/source + e2e-probe:ark-ascended-tcp | ark-survival-ascended | ark-survival-ascended | Only module in category (probe is a raw TCP dial to the RCON port, not an A2S query — see test/e2e/internal/ark-survival-ascended/app.go) |
| rcon/source + e2e-probe:steam-a2s-udp | ark-survival-evolved, cs2, hell-let-loose, left-4-dead-2, project-zomboid, squad, team-fortress-2, the-isle, v-rising | cs2 | 2Gi (lightest memory in category) |
| rcon/source + gameproto:minecraft-java | minecraft-java | minecraft-java | Designated representative (minecraft-java) |
| rcon/websocket + e2e-probe:steam-a2s-udp | rust | rust | Only module in category |

**Total categories:** 17
