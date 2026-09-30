# Review: tunnel

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `tunnel/specs.md`; package doc `tunnel/main.go:1-18`; `docs/tunnels.md`; the operator side of the env contract `operator/internal/controller/gameserver_tunnel.go` (named authoritative in `main.go:6-7`); CRD field docs `operator/api/v1alpha1/gameserver_types.go:404-462`

## Scope reviewed

Read in full: `tunnel/main.go`, `tunnel/specs.md`, `tunnel/Dockerfile.frp`, `tunnel/Dockerfile.playit`, `tunnel/Dockerfile.tailscale`, `tunnel/go.mod`, `tunnel/.testcoverage.yml`.

`tunnel/main_test.go`: read lines 690-968 (backoff, `isUnrecoverable`, `run`, `runCommand` tests) plus the list of test functions. Lines 1-689 (config, render and command-building tests) were not read line by line.

Supporting code read to check the operator/tunnel contract and the docs: `operator/internal/controller/gameserver_tunnel.go` (full), `operator/internal/controller/tunnel_rbac.go` (full), `operator/internal/controller/gameserver_controller.go:555-575,1105-1135` (backing Service ports) and `:326,381-386` (reconciler wiring), `operator/api/v1alpha1/gameserver_types.go:404-462`, `api/internal/handlers/resources.go:544-553`, `modules/valheim/template.yaml:43-57`, `docs/tunnels.md` (full), and a grep of `docs/architecture.md` for "tunnel".

## Method

- Went through each responsibility, invariant and failure-handling rule in `tunnel/specs.md` and traced it through `main.go`.
- Traced every env var from the operator (`gameserver_tunnel.go:242-309`) into `loadConfig` and on to its use (or lack of use) in rendering and `buildCommand`.
- Checked what the `exponentialBackoff` arithmetic does over a pod's lifetime by running a copy of `next()` in a scratch program (`GOWORK=off go run`; not a test suite): retries 1-63 give 1s…300s, retry 64 onward gives `0s`. Cumulative sleep before retry 64 is 4h38m31s.
- Compared each `docs/tunnels.md` claim with the code.
- Did not run tests or linters (Rule 8). Nothing was run against a cluster.

## Observations (no finding)

- Credentials are read only from the closed key set (`main.go:67-71`) with a `filepath.Rel` containment check (`:259-268`), and are never put on argv: `buildCommand` passes only constant paths (`:465-495`). This matches `specs.md:99,105,206-208`.
- Rendered files are written `0o600` (`:335,392,416`), matching `specs.md:89`.
- SIGTERM handling matches `specs.md:186-194`: `cmd.Cancel` sends SIGTERM (`:510-515`), `WaitDelay` is 10s (`:516`), and `run` returns `nil` on cancellation (`:227-230,244-245`).
- `loadConfig` enforces the per-provider required variables and the `FRP_SERVER_PORT` range exactly as `specs.md:67` says.
- `TAILSCALE_TAGS` not applied is a known gap in `specs.md:63,119`. (see held item OBS-tunnel-tailscale-tags, OD-019)
- The operator's playit RBAC (`tunnel_rbac.go:98-111`) is scoped by `resourceNames` to one GameServer, as `specs.md:150-153` says.
- Held candidates: 2 (see OD-019).

## Candidate findings

### C-tunnel-01: The restart backoff never resets and overflows to 0s after the 64th lifetime restart, so the supervisor then respawns in a tight loop

- **Location**: `tunnel/main.go:571-575` (`base := 1 << uint(b.retries-1)`), `tunnel/main.go:215` (one `exponentialBackoff` per `run`, never reset)
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. `run` creates one `exponentialBackoff` (`main.go:215`) and calls `backoff.next()` after every transient exit (`:240`). `retries` only goes up. Nothing resets it after a relay has run healthily for a while.
  2. On 64-bit `int`, at `retries == 64` the shift `1 << 63` gives `math.MinInt64`. That is not `> 300`, so the cap is skipped, and `time.Duration(MinInt64) * time.Second` wraps to `0`. At `retries >= 65`, `1 << 64` is `0`. Every later delay is `0s` (checked with a scratch copy of `next()`; see Method).
  3. A realistic trigger: the frps host is unreachable for about 4h40m (the cumulative sleep up to retry 63, plus relay run time). frpc exits non-zero on a failed first login and gets retried, and at retry 64 the supervisor starts respawning the relay with no delay, indefinitely. The same happens after 64 unrelated transient exits spread over the pod's life.
- **Expected**: `specs.md:107`: "retry delays follow 2^n seconds (1s, 2s, 4s, ..., 256s) capped at 300 seconds (5 minutes)". `specs.md:175`: "retry N: min(2^(N-1), 300) seconds". The delay never falls below 1s.
- **Actual**: From the 64th restart on, the delay is `0s` for the rest of the pod's life. The result is a busy respawn loop against the relay server, with log flooding. Restarting the pod is the only way out.

### C-tunnel-02: A relay that exits with status 0 is restarted immediately, with no backoff and no log line

- **Location**: `tunnel/main.go:226-248`
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. `runCommand` returns `nil` when the child exits 0 (`main.go:522-528`).
  2. In `run`, a `nil` error with a live context skips both `if` blocks (`:227-248`), so neither the "relay process exited" log nor the backoff sleep runs. The `for` loop respawns straight away.
  3. A relay that exits 0 right after starting (for example after a clean remote disconnect or a config it treats as "nothing to do") therefore spins with no delay and no trace in the logs.
- **Expected**: `specs.md:183`: "**Transient (retry):** All other exit codes and errors. Pod computes backoff delay, sleeps, and spawns the relay binary again." Exit code 0 is one of "all other exit codes".
- **Actual**: Exit 0 means an immediate respawn with no delay and nothing logged.

### C-tunnel-03: frp forwards to `localPort = remotePort`, not to the game's port, so any `remotePort` different from the container port can't reach the game

- **Location**: `tunnel/main.go:325-332` (`localPort = %s` and `remotePort = %s` both get the same `port`); operator side `operator/internal/controller/gameserver_tunnel.go:249,522-543` (`BACKING_SERVICE_PORT` is built as `name:remotePort`)
- **Category**: correctness
- **Suggested severity**: S3 (workaround: set every `remotePort` equal to the template's container port)
- **Observation / repro**:
  1. The operator builds `BACKING_SERVICE_PORT` from `spec.networking.tunnel.frp.remotePorts` as `"<name>:<remotePort>"` (`gameserver_tunnel.go:522-531`). The template's `containerPort` is not passed.
  2. `renderFrpConfig` renders each entry as `localIP = "<gs>.<ns>.svc"`, `localPort = <remotePort>`, `remotePort = <remotePort>` (`main.go:325-332`).
  3. The backing Service listens on the template `containerPort`, or the `portOverrides` servicePort (`gameserver_controller.go:1114-1127`), not on the frps remote port.
  4. Example: `minecraft-java` (container port 25565) with `remotePorts: [{name: game, remotePort: 30565}]`, which is the kind of mapping someone uses to share one frps between two servers. frpc dials `<gs>.<ns>.svc:30565`, which no Service port matches, so every player connection fails. The tunnel egress NetworkPolicy also admits only the advertised container ports plus the frps port (`gameserver_tunnel.go:425-480`), so 30565 is blocked there too.
- **Expected**: `docs/tunnels.md:47-48`: the public address is "`<serverAddr>:<remotePort>`", and the tunnel forwards it to the game. The CRD says `remotePorts` "maps advertised template ports to ports on the frps host" (`gameserver_types.go:418`), so the local side is the template port.
- **Actual**: Forwarding works only when `remotePort` equals the Service port. The `docs/tunnels.md:136-141` example works only because it uses 25565 on both sides.

### C-tunnel-04: frp proxies are always `type = "tcp"`, so UDP game ports (Valheim, Bedrock, SteamCMD games) can't be tunnelled

- **Location**: `tunnel/main.go:326-332` (hard-coded `type = "tcp"`)
- **Category**: correctness
- **Suggested severity**: S3 (UDP games have no frp path; the other providers are the only alternative)
- **Observation / repro**:
  1. `BACKING_SERVICE_PORT` carries only `name:port` (`gameserver_tunnel.go:530`). The protocol is not passed, and the renderer writes `type = "tcp"` for every proxy.
  2. `modules/valheim/template.yaml:43-55` advertises `game`/`game2`/`game3` on 2456-2458 **UDP**. With `provider: frp` and matching `remotePorts`, frpc registers TCP proxies. UDP datagrams to `<serverAddr>:2456` are never forwarded, while `planTunnel` still advertises those endpoints with `Protocol: UDP` (`gameserver_tunnel.go:87-93`).
- **Expected**: `docs/tunnels.md:32` lists frp "**UDP support** | Yes*". `docs/tunnels.md:39` says: "For frp: the protocol (TCP or UDP) for each forwarded port is determined by the port's `protocol` field in the GameTemplate, not configured separately in the frp tunnel spec."
- **Actual**: The protocol is always TCP, whatever the template's `protocol` says.

### C-tunnel-05: The Tailscale provider joins the tailnet but forwards nothing to the game Service

- **Location**: `tunnel/main.go:291` (renderer gets only hostname and key), `tunnel/main.go:378-397` (config holds `version`/`authKey`/`hostname` only), `tunnel/main.go:478-482` (tailscaled flags); `BACKING_SERVICE_DNS`/`BACKING_SERVICE_PORTS` are only presence-checked for tailscale (`:137-139,171-174`)
- **Category**: correctness
- **Suggested severity**: S2 (the Tailscale provider's only purpose, reaching the game, doesn't work; no in-product workaround)
- **Observation / repro**:
  1. For `TUNNEL_TYPE=tailscale`, `cfg.BackingServiceDNS` and `cfg.BackingServicePorts` are validated as non-empty and never read again (grep in `main.go`: `BackingServiceDNS` is used only at `:125,137,332`, `:332` being the frp renderer; `BackingServicePorts` only at `:171-172,182-183`).
  2. The tailscaled config holds no serve/forwarding configuration, no advertised routes and no proxy. The only process in the pod is `tailscaled --tun=userspace-networking` (`:478-482`). With userspace networking, tailscaled delivers inbound tailnet connections to the pod's own loopback, where nothing listens on the game port. The supervisor never configures anything that would route them on to `<gs>.<ns>.svc`.
  3. The operator still advertises `hostname:containerPort` as a tailnet endpoint (`gameserver_tunnel.go:108-121`).
- **Expected**: `docs/tunnels.md:54-55`: "The tunnel pod joins your Tailscale network and becomes a device reachable by all users on your tailnet". `docs/tunnels.md:184`: "Port 25565 is available to devices on your tailnet." `specs.md:9`: the supervisor "renders provider-specific config files … so connection attempts can trigger wake-on-connect".
- **Actual**: The device registers under the hostname, but a player connecting to `<hostname>:25565` reaches no listener, because nothing bridges tailnet traffic to the game Service. Verification tier: confirm live with a tailnet by joining and connecting to the advertised endpoint.

### C-tunnel-06: The playit address is never reported to `status.endpoints`, although the spec lists it as a responsibility and the docs tell users to wait for it

- **Location**: `tunnel/main.go:282-285` (TODO: "The exact mechanism (interface, callback, status update) is TBD."); operator side `gameserver_tunnel.go:124-131` (`// TODO(tunnel): playit endpoint arrives via the gameservers/status subresource`)
- **Category**: correctness / docs-drift
- **Suggested severity**: S3 (the playit.gg dashboard shows the address)
- **Observation / repro**:
  1. The tunnel binary is stdlib-only, with no Kubernetes client, and has no code that reads playitd's assigned address or patches the GameServer status.
  2. `planTunnel` computes no endpoints for playit (`gameserver_tunnel.go:124-131`), so `status.endpoints` never gets a playit entry.
  3. The operator still creates a ServiceAccount, Role and RoleBinding for the tunnel pod solely so it can "patch the GameServer's status subresource with the playit-assigned endpoint" (`tunnel_rbac.go:24-29`). Nothing uses that grant.
- **Expected**: `specs.md:21`: "9. (Playit only) Validate and patch the GameServer's status subresource with assigned relay addresses once discovered." `docs/tunnels.md:66-67`: "The address appears in `status.endpoints` once the tunnel pod reports it back." `docs/tunnels.md:223-224`: "Once the tunnel pod connects and reports the assigned address, it appears in `status.endpoints`."
- **Actual**: No playit address ever appears in `status.endpoints`. Users following the docs wait indefinitely and must read the address from the playit.gg dashboard instead.

### C-tunnel-07: A missing relay binary is retried forever instead of treated as unrecoverable

- **Location**: `tunnel/main.go:535-549` (`isUnrecoverable` string-matches `exit status 126/127` and `permission denied`), `tunnel/main.go:518-520` (Start error)
- **Category**: correctness
- **Suggested severity**: S4 (only a misbuilt image triggers it)
- **Observation / repro**:
  1. The supervisor execs the relay directly, not through a shell, so a missing binary never produces exit status 127. `cmd.Start()` fails with `start relay: fork/exec /usr/local/bin/frpc: no such file or directory`.
  2. That string matches none of the patterns in `isUnrecoverable`, so it is treated as transient and retried with backoff indefinitely (and, per C-tunnel-01, eventually with no delay). The test file says so: `main_test.go:803-812` notes the "no such file or directory" Start failure "is treated as a transient failure".
- **Expected**: `specs.md:109`: "Exit code 126 (permission denied) and 127 (command not found) indicate misconfiguration, missing binaries, or permission issues." `specs.md:19`: on unrecoverable failure, "exit immediately". The spec's intent is that a missing binary is fatal.
- **Actual**: A missing binary is retried forever. The 127 check can only fire if the relay binary itself exits 127.

### C-tunnel-08: The `docs/tunnels.md` troubleshooting steps use a label that doesn't exist and describe the egress NetworkPolicy as future work

- **Location**: `docs/tunnels.md:302-305`, `docs/tunnels.md:321-328`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `docs/tunnels.md:304`: `kubectl -n gameplane-games get pods -l gameplane.local/tunnel=<server-name>`. The tunnel pods are labelled `app.kubernetes.io/name=gameplane-tunnel` and `app.kubernetes.io/instance=<gs>` (`gameserver_tunnel.go:21-22,178-181`), and nothing sets `gameplane.local/tunnel` (repo-wide grep finds only this doc line). The command returns no pods.
  2. `docs/tunnels.md:325-328`: "> **Planned:** Gameplane will automatically create a NetworkPolicy rule granting the tunnel pod egress to the relay's address … For now, manually add a NetworkPolicy if needed." The operator already creates `<gs>-tunnel-egress` (`reconcileTunnelNetworkPolicy`, `gameserver_tunnel.go:369-492`, called from `gameserver_controller.go:386`), and `tunnel/specs.md:157-163` documents it.
- **Expected**: The troubleshooting steps use the real labels and describe the NetworkPolicy the operator creates.
- **Actual**: As quoted above.

### C-tunnel-09: tunnel/specs.md and the Dockerfiles describe a dependency, a doc section and cleanup behaviour that don't exist

- **Location**: `tunnel/specs.md:245`, `tunnel/specs.md:89`, `tunnel/Dockerfile.frp:4-8`, `tunnel/Dockerfile.playit:4-8`, `tunnel/Dockerfile.tailscale:4-8`
- **Category**: docs-drift / dead-code
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:245`: "**Architecture:** `docs/architecture.md` § "tunnel"". `grep -n -i tunnel docs/architecture.md` returns nothing.
  2. All three Dockerfiles say: "gameaction is an in-repo module tunnel depends on via a local replace (../gameaction); copy it first so `go mod download` and the build resolve it" and run `COPY gameaction/ ./gameaction/`. `tunnel/go.mod` is `module …/tunnel` + `go 1.26.0` only: no `require`, no `replace`, and `main.go` imports nothing from gameaction. `specs.md:5,198-202` correctly says "stdlib only". The COPY is dead weight in the build context, and the comment is wrong.
  3. `specs.md:89`: "Files are cleaned up automatically when the relay process exits or the pod is terminated." The rendered file is removed only when `run` returns (`main.go:208-212`). It is deliberately kept across relay exits so the restart can reuse it. `/tmp/tailscale.state` (`:480`) is never removed.
- **Expected**: The spec and Dockerfile comments match the module.
- **Actual**: As listed above.

## Questions (not findings)

- `tunnel/Dockerfile.playit:40-44` downloads the release asset `playit-linux-<arch>` and installs it as `/usr/local/bin/playitd`. `main.go:484-490` says `playit-cli` ("historically installed as `playit`") is a separate service manager with no foreground mode, and that `playitd` must be the binary. Is the `playit-linux-<arch>` asset of playit-agent v1.0.10 the `playitd` daemon, or the CLI? That needs an upstream check.
- Tailscale node state lives in `/tmp/tailscale.state` in the container's writable layer (`main.go:480`). The Deployment mounts no volume for it (`gameserver_tunnel.go:312-342`). After any pod recreation (rollout, node drain, operator image bump), tailscaled registers as a new device. With the reusable, non-ephemeral key the docs recommend (`docs/tunnels.md:147-148`), the old device record remains, and Tailscale may give the new one `<hostname>-1`, leaving the advertised `hostname` endpoint on a stale device. Needs live confirmation.
- `docs/tunnels.md:154` shows `tskey-client-xxxxx`, the OAuth client-secret format, as the auth key. As far as I know, tailscaled accepts an OAuth secret as an auth key only with advertised tags, which aren't applied (known gap; held H-tunnel-02). Would registration with the documented key format fail?
- Q-tunnel-empty-credential: held (OD-019)
- Q-tunnel-toml-escaping: held (OD-019)
