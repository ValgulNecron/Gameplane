# T045 libs chunk: independent verification (opus)

Method: I tried to refute each of the 9 candidates in `notes.md` (reviewer: opus) against master `13a859ff`. None of the cited files differ between this branch and master. I read `tunnel/main.go` and `tunnel/specs.md` in full, the three Dockerfiles, `tunnel/go.mod`, and the supervision tests in `tunnel/main_test.go:690-967`. On the operator side I read `operator/internal/controller/gameserver_tunnel.go` in full, `tunnel_rbac.go:20-35`, the playit status handling in `gameserver_status.go:197-230` and `:1078-1103`, and the CRD field docs in `operator/api/v1alpha1/gameserver_types.go:400-452`. I also read `docs/tunnels.md` (overview, walkthroughs, troubleshooting and security notes) and grepped `docs/architecture.md`. I ran a scratch copy of `exponentialBackoff.next()` with `GOWORK=off go run` in the session scratchpad (not a test suite). I checked `audit/findings.md` (F-018 and F-031 touch tunnels, but neither overlaps) and the other review notes. That check found `C-operator-09` in `audit/evidence/review-operator/notes.md`, which already covers both C-tunnel-03 and C-tunnel-04. I ran no test or lint suite and nothing against a cluster or a tailnet. The tunnel review's two held candidates are verified separately, off-git (OD-019).

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-tunnel-01 | kept | S3 | Confirmed. The scratch copy of `next()` returns 1s…4m16s, then 5m0s up to retry 63, then `0s` from retry 64 on (cumulative sleep before retry 64: 4h38m31s). The counter is created once per `run` and never reset. A realistic trigger is a wrong frp token or an unreachable frps left for about 4h40m: frpc exits non-zero on a failed first login, so each attempt counts as a transient exit. After that, the supervisor respawns with no delay for the life of the pod. Restarting the pod clears it. |
| C-tunnel-02 | rejected | n/a | This is speculative. The code does skip both the log line and the backoff on a nil error (`main.go:226-248`). But the notes name no real case in which `frpc`, `tailscaled` or `playitd` exits 0 on its own. Their examples ("clean remote disconnect", "nothing to do") are hypothetical, and frpc reconnects internally after a disconnect. With no reachable trigger, this is a spec-text mismatch and not a demonstrated defect. |
| C-tunnel-03 | rejected | n/a | Duplicate. The defect is real (`buildFrpRemotePortsConfig` sends `name:remotePort` only, at `gameserver_tunnel.go:522-541`, and `renderFrpConfig` uses that port as `localPort`, at `main.go:325-332`). It is the first half of `C-operator-09` (`audit/evidence/review-operator/notes.md:148-157`), which cites the same two locations. Track it there. |
| C-tunnel-04 | rejected | n/a | Duplicate. The defect is real (`type = "tcp"` is hard-coded at `main.go:328`, and no protocol is passed in `BACKING_SERVICE_PORT`). It is the second half of `C-operator-09` ("A UDP port is always proxied as TCP"). Track it there. |
| C-tunnel-05 | kept | S2 | Confirmed by reading the code; not confirmed live. For `tailscale`, `BACKING_SERVICE_DNS`/`BACKING_SERVICE_PORTS` are only checked for presence. The rendered tailscaled config holds only `version`, `authKey` and `hostname`, and the pod's only process is `tailscaled --tun=userspace-networking`. Nothing forwards tailnet traffic to `<gs>.<ns>.svc`, and the tunnel is a separate Deployment, not a sidecar in the game pod. Meanwhile the operator advertises `<hostname>:<containerPort>` as a tailnet endpoint. No Gameplane setting makes this provider reach the game, so S2. Switching provider changes the exposure model (public instead of tailnet-only), so it is not a like-for-like workaround. |
| C-tunnel-06 | kept | S3 | Confirmed. The tunnel binary is stdlib-only and has no code that reads playitd's address or writes GameServer status (`main.go:282-285` is a TODO). The operator consumes `status.tunnelEndpoints` for playit (`gameserver_status.go:197-230`) and creates RBAC for the tunnel to patch it (`tunnel_rbac.go:24-29`), but nothing ever writes it. `TunnelReady` stays at "waiting for playit endpoint assignment". Workaround: read the address from the playit.gg dashboard. |
| C-tunnel-07 | rejected | n/a | Unreachable with the shipped images. All three Dockerfiles install the relay at exactly the path `buildCommand` execs (`/usr/local/bin/frpc`, `tailscaled`, `playitd`). Only a hand-built image passed through `--tunnel-*-image` could hit it. Even then, the `start relay: fork/exec … no such file or directory` error is logged on every retry. The only test on this path (`main_test.go:803-812`) uses the missing binary on purpose to drive the transient branch. |
| C-tunnel-08 | kept | S4 | Confirmed. `docs/tunnels.md:304` selects `-l gameplane.local/tunnel=<server-name>`, a label nothing sets. The operator labels tunnel pods `app.kubernetes.io/name=gameplane-tunnel` and `app.kubernetes.io/instance=<gs>` (`gameserver_tunnel.go:21-22,170-181`). `docs/tunnels.md:325-328` calls the egress NetworkPolicy "Planned", but `reconcileTunnelNetworkPolicy` already creates `<gs>-tunnel-egress` (`gameserver_tunnel.go:362-492`, called from `gameserver_controller.go:386`). |
| C-tunnel-09 | kept | S4 | Confirmed. `docs/architecture.md` has no tunnel section. All three Dockerfiles say tunnel depends on gameaction "via a local replace" and copy `gameaction/`, but `tunnel/go.mod` has no `require` or `replace` and `main.go` doesn't import it. `specs.md:89` says files are removed when the relay exits, but the rendered file is removed only when `run` returns, and `/tmp/tailscale.state` is never removed. |

### C-tunnel-01

**Location:** `tunnel/main.go:571-575` (`base := 1 << uint(b.retries-1)` with only an upper cap); `tunnel/main.go:215` (one `exponentialBackoff` per `run`, never reset) and `:240` (`backoff.next()` after every transient exit).

**Repro / observation:**
1. Copy `exponentialBackoff.next()` (`main.go:558-576`) into a scratch `main` package, call it 70 times, and print each delay: retries 1-9 give 1s…4m16s, retries 10-63 give 5m0s, and retries 64-70 give `0s`. Cumulative sleep before retry 64 is 4h38m31s. The mechanism: on 64-bit `int`, `1 << 63` is `math.MinInt64`, which is not `> 300`, so the cap is skipped, and `time.Duration(MinInt64) * time.Second` wraps to 0. From retry 65 on, `1 << 64` is 0.
2. `run` creates the backoff once (`main.go:215`) and only ever increments it. A relay that later runs healthily for days still inherits the count.
3. Live (optional): create an frp-tunnelled GameServer whose `serverAddr` points at an address where no frps listens, or with a wrong token. frpc's default `loginFailExit` ends it with a non-zero status after a failed first login. `kubectl logs -f deploy/<gs>-tunnel` shows "restarting relay in 5m0s" repeating. After about 4h40m it changes to "restarting relay in 0s" in a tight loop.
4. `TestExponentialBackoffCap` (`main_test.go:726-743`) checks only retry 11, so it doesn't reach the overflow.

**Expected:** `tunnel/specs.md:107` and `:175`: delays follow `min(2^(N-1), 300)` seconds, so no delay is ever below 1s.

**Actual:** From the 64th lifetime restart on, every delay is `0s` until the pod is recreated. The supervisor respawns the relay in a busy loop against the relay server and floods the pod log.

### C-tunnel-05

**Location:** `tunnel/main.go:291` (the renderer gets only the hostname and the key), `tunnel/main.go:378-397` (the config holds `version`, `authKey` and `hostname` only), `tunnel/main.go:478-482` (tailscaled flags), `tunnel/main.go:171-174` (`BACKING_SERVICE_PORTS` presence check only); operator side `operator/internal/controller/gameserver_tunnel.go:98-121` (endpoint advertisement) and `:161-352` (a separate single-container Deployment).

**Repro / observation:**
1. In `tunnel/main.go`, `cfg.BackingServiceDNS` is read at `:125`, checked at `:137` and used only by the frp renderer at `:332`. `cfg.BackingServicePorts` is read and checked at `:171-172` and `:182-183`, and never used after that. For `TUNNEL_TYPE=tailscale`, neither value reaches tailscaled.
2. `renderTailscaleConfig` writes `{"version":"alpha0","authKey":…,"hostname":…}` with no serve or forwarding configuration and no advertised routes. `buildCommand` runs `tailscaled --tun=userspace-networking --state=… --config=…` and nothing else.
3. The operator runs the tunnel as its own Deployment `<gs>-tunnel` with one container, `tunnel`, not as a sidecar in the game pod. So the tunnel pod's loopback has no game listener. In userspace-networking mode, tailscaled delivers inbound tailnet connections to listeners in its own network namespace, or to Serve handlers configured in its config. The supervisor configures neither.
4. `planTunnel` still publishes `<hostname>:<containerPort>` (`Private: true`, provider `tailscale`) into `status.endpoints` for every advertised port.
5. Live confirmation (needs a tailnet): create a GameServer with `provider: tailscale`, wait for the device to join, then from another tailnet device run `nc -vz <hostname> <containerPort>` (or join with the game client). The connection is refused or times out while the game Service answers in-cluster.

**Expected:** `docs/tunnels.md:54-55`: the tunnel pod "becomes a device reachable by all users on your tailnet". `docs/tunnels.md:184`: "Port 25565 is available to devices on your tailnet." `tunnel/specs.md:28`: "the relay process handles all inbound connections and port forwarding". Connections to the advertised endpoint reach the game Service.

**Actual:** The device registers under the hostname, but nothing bridges tailnet traffic to `<gs>.<ns>.svc`, so the advertised endpoint has no game behind it.

### C-tunnel-06

**Location:** `tunnel/main.go:282-285` (TODO: "The exact mechanism (interface, callback, status update) is TBD."); `operator/internal/controller/gameserver_tunnel.go:124-131` (`// TODO(tunnel): playit endpoint arrives via the gameservers/status subresource`); `operator/internal/controller/tunnel_rbac.go:24-29` (the unused grant); consumer `operator/internal/controller/gameserver_status.go:197-230`.

**Repro / observation:**
1. `tunnel/go.mod` has no requirements (stdlib only), and `tunnel/main.go` contains no Kubernetes client, no read of playitd's output or socket, and no status write.
2. `planTunnel` computes no endpoints for playit (`gameserver_tunnel.go:124-131`). The only playit path into `status.endpoints` is `gameserver_status.go:197-230`, which validates and merges `gs.Status.TunnelEndpoints` that someone else wrote.
3. The operator still creates a ServiceAccount, Role and RoleBinding so the tunnel pod can "patch the GameServer's status subresource with the playit-assigned endpoint" (`tunnel_rbac.go:24-29`). No process in the pod uses them.
4. `gameserver_status.go:1096-1103` sets `TunnelReady` to "tunnel deployment ready; waiting for playit endpoint assignment", and nothing ever advances it.

**Expected:** `tunnel/specs.md:21` (responsibility 9): "(Playit only) Validate and patch the GameServer's status subresource with assigned relay addresses once discovered." `docs/tunnels.md:66-67` and `:223-224`: the address "appears in `status.endpoints` once the tunnel pod reports it back".

**Actual:** No playit address ever reaches `status.endpoints`. A user following the docs waits indefinitely and must read the address from the playit.gg dashboard.

### C-tunnel-08

**Location:** `docs/tunnels.md:304` and `docs/tunnels.md:325-328`.

**Repro / observation:**
1. `docs/tunnels.md:304`: `kubectl -n gameplane-games get pods -l gameplane.local/tunnel=<server-name>`. `grep -rn 'gameplane.local/tunnel'` over `*.go`, `*.yaml` and `*.md` finds only this doc line. The operator labels tunnel pods `app.kubernetes.io/name=gameplane-tunnel` and `app.kubernetes.io/instance=<gs>` (`operator/internal/controller/gameserver_tunnel.go:21-22`, `:170-181`), so the documented selector returns no pods.
2. `docs/tunnels.md:325-328`: "> **Planned:** Gameplane will automatically create a NetworkPolicy rule granting the tunnel pod egress … For now, manually add a NetworkPolicy if needed." The operator already creates `<gs>-tunnel-egress` in `reconcileTunnelNetworkPolicy` (`gameserver_tunnel.go:362-492`), called on every reconcile from `gameserver_controller.go:386`. `tunnel/specs.md:157-163` documents it.

**Expected:** The troubleshooting steps use `-l app.kubernetes.io/name=gameplane-tunnel,app.kubernetes.io/instance=<server-name>` and describe the `<gs>-tunnel-egress` policy the operator creates.

**Actual:** The first troubleshooting command finds nothing, and the NetworkPolicy step tells users to hand-write a policy that already exists.

### C-tunnel-09

**Location:** `tunnel/specs.md:245`, `tunnel/specs.md:89`; `tunnel/Dockerfile.frp:4-8`, `tunnel/Dockerfile.playit:4-8`, `tunnel/Dockerfile.tailscale:4-8`.

**Repro / observation:**
1. `tunnel/specs.md:245` cites `docs/architecture.md` § "tunnel". `grep -n -i tunnel docs/architecture.md` returns nothing.
2. Each Dockerfile says "gameaction is an in-repo module tunnel depends on via a local replace (../gameaction)" and runs `COPY gameaction/ ./gameaction/`. `tunnel/go.mod` contains only the `module` line and `go 1.26.0`, and `tunnel/main.go` imports only the standard library. `tunnel/specs.md:5` and `:198-202` say correctly that the module is stdlib-only.
3. `tunnel/specs.md:89`: "Files are cleaned up automatically when the relay process exits or the pod is terminated." The rendered file is removed only by the deferred `os.Remove` when `run` returns (`main.go:208-212`). It is kept across relay exits on purpose, so a restart reuses it. `/tmp/tailscale.state` (`main.go:480`) is never removed.

**Expected:** The spec's cross-reference and cleanup statement, and the Dockerfile comments and build steps, match the module.

**Actual:** A dead section reference, a stale gameaction dependency copied into all three image builds, and a cleanup claim the code doesn't implement.
