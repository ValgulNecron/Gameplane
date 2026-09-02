# Tunnels

A GameServer can optionally route players through an external relay — the server
dials outbound to the relay and players connect to the relay's address. This exists
for installs with no public IP and no ability to port-forward (homelab k3s behind
CGNAT, ISP-controlled routers, etc.). It layers over the backing Service; it is
not a fifth `expose` mode.

## When to use a tunnel

Use a tunnel [optional] if:

- You have no public IP (e.g., CGNAT, shared ISP).
- Port-forwarding is unavailable (router is locked down or managed by your ISP).
- `expose: LoadBalancer` is not an option (you don't have an external load balancer
  service, e.g., you're not on AWS/GCP/Azure).

Do **not** use a tunnel if:

- You already have a public IP and working port-forwarding.
- You have a working LoadBalancer and it reaches your cluster.
- You're only deploying to private networks (no public play access needed).

If you have any of the above, those approaches are simpler and cheaper — tunnels
add an extra pod and outbound dependency.

## Choosing a provider

| Feature | frp | Tailscale | playit.gg |
|---|---|---|---|
| **Public/Private** | Public | Private (tailnet only) | Public |
| **UDP support** | Yes* | Yes | Yes |
| **Address assignment** | Static (you set it) | MagicDNS (static name) | Dynamic (assigned at runtime) |
| **VPS required** | Yes (you run frps) | No | No |
| **Cost** | Your VPS cost | Free (open source) | Free (limits: 8 hours/day or paid unlimited) |
| **Setup time** | 30–60 min (deploy frps) | 5 min (get auth key) | 2 min (sign up, get secret) |
| **Best for** | Production, custom domains | Private homemade networks | Casual play, quick setup |

\* For frp: the protocol (TCP or UDP) for each forwarded port is determined by the port's `protocol` field in the GameTemplate, not configured separately in the frp tunnel spec. Each port you forward to frps must be declared with its matching protocol in the template.

### frp

**Use case:** production servers, custom public domain, full control.

Self-hosted. You run an `frps` (frp server) on a VPS you control, and Gameplane
runs the `frpc` (frp client) in a tunnel pod that dials your `frps` and holds
open reverse-proxy tunnels. The public address is static and derived from your
config: `<serverAddr>:<remotePort>`.

### Tailscale

**Use case:** private networks, invite-only play, no public internet needed.

The tunnel pod joins your Tailscale network and becomes a device reachable by
all users on your tailnet via MagicDNS. Players access it only if they're also
on your tailnet. This is **not** public — Tailscale Funnel does not help here,
as Funnel only exposes TCP on ports 443/8443/10000 with TLS, and cannot carry
arbitrary game ports or UDP.

### playit.gg

**Use case:** free, public, zero setup. Casual play and quick testing.

Free public relay service. Gameplane writes a config file with your secret key,
the tunnel pod connects and holds a tunnel, and playit assigns a public address
at runtime. The address appears in `status.endpoints` once the tunnel pod
reports it back. Free tier allows 8 hours/day per secret; paid plans unlock
24/7. No VPS needed.

## Setup

### frp

#### 1. Deploy frps on a VPS

You control the VPS (this is not part of Gameplane). Deploy `frps` and pick a
free port for client connections (e.g., 7000). Refer to [frp's
documentation](https://github.com/fatedier/frp) for detailed setup.

Example systemd unit (adjust paths and token to suit):

```ini
[Unit]
Description=frps server
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/frps -c /etc/frp/frps.toml
Restart=on-failure
User=frp

[Install]
WantedBy=multi-user.target
```

Example `frps.toml`:

```toml
bindAddr = "0.0.0.0"
bindPort = 7000

# Require clients to authenticate with a token
auth.method = "token"
auth.additionalScopes = ["api"]
auth.token = "your-secure-token-here"
```

#### 2. Create a Secret with the token

```bash
kubectl -n gameplane-games create secret generic frp-creds \
  --from-literal=token=your-secure-token-here
```

#### 3. Create a GameServer with frp tunnel

```yaml
apiVersion: gameplane.local/v1alpha1
kind: GameServer
metadata:
  name: minecraft-tunneled
  namespace: gameplane-games
spec:
  template: minecraft-java
  networking:
    expose: ClusterIP
    tunnel:
      enabled: true
      provider: frp
      credentialsSecretRef:
        name: frp-creds
      frp:
        serverAddr: frp.example.com
        serverPort: 7000
        remotePorts:
          - name: game
            remotePort: 25565
```

Players connect to `frp.example.com:25565`.

### Tailscale

#### 1. Generate an auth key

Log into [login.tailscale.com](https://login.tailscale.com), go to **Settings →
Keys**, and create a **Reusable key**. Copy it.

#### 2. Create a Secret with the auth key

```bash
kubectl -n gameplane-games create secret generic tailscale-creds \
  --from-literal=authKey=tskey-client-xxxxx
```

#### 3. Create a GameServer with Tailscale tunnel

```yaml
apiVersion: gameplane.local/v1alpha1
kind: GameServer
metadata:
  name: minecraft-tailnet
  namespace: gameplane-games
spec:
  template: minecraft-java
  networking:
    expose: ClusterIP
    tunnel:
      enabled: true
      provider: tailscale
      credentialsSecretRef:
        name: tailscale-creds
      tailscale:
        hostname: my-minecraft-server
        tags:
          - tag:gameplane
          - tag:minecraft
```

The pod joins your tailnet and is reachable via MagicDNS under the hostname you
specify in `spec.networking.tunnel.tailscale.hostname` (or defaults to the
GameServer name if left empty). The tags are Tailscale ACL tags applied at device
registration. Port 25565 is available to devices on your tailnet.

> **Private only.** The server is not exposed to the public internet — only to
> devices on your Tailscale network. This is suitable for playing with friends
> or family who are all on your tailnet.

### playit.gg

#### 1. Sign up and get a secret key

Visit [playit.gg](https://playit.gg), sign up, and create an account. Your
**Secret Key** appears in **Settings**. Copy it.

#### 2. Create a Secret with the secret key

```bash
kubectl -n gameplane-games create secret generic playit-creds \
  --from-literal=secretKey=<your-secret-key>
```

#### 3. Create a GameServer with playit tunnel

```yaml
apiVersion: gameplane.local/v1alpha1
kind: GameServer
metadata:
  name: minecraft-playit
  namespace: gameplane-games
spec:
  template: minecraft-java
  networking:
    expose: ClusterIP
    tunnel:
      enabled: true
      provider: playit
      credentialsSecretRef:
        name: playit-creds
```

Wait for the pod to reach `Running` state. Once the tunnel pod connects and
reports the assigned address, it appears in `status.endpoints`. The address
changes if the pod restarts; playit does not guarantee address stability on free
tier.

> **Free tier limits.** By default, playit assigns 8 hours/day per secret. Upgrade
> to a paid plan for 24/7 access.

## Cost and resource usage

**A tunnel-enabled GameServer runs an extra always-on pod for its entire
lifecycle, including while the server is idle and asleep.** This is deliberate
— the tunnel must hold the public address so players can reconnect after waking
the server. (See **Interaction with idle auto-sleep** below for how sleep and
tunnels compose.)

Implications:

- You pay for two pods instead of one while the server is running.
- You pay for one pod (the tunnel) even when the game is asleep.
- The tunnel pod is small (~50 MiB memory, negligible CPU), but it is not free.

If you have many servers and want to minimize idle costs, consider:

- Disabling idle auto-sleep (`spec.idle.enabled: false`) and stopping servers
  manually.
- Routing multiple servers through a single tunnel relay (if your relay supports
  it; this requires multiple `remotePorts` entries in your frp config, or a
  custom relay setup).
- Using a provider with free tier or time-based billing (playit's free tier and
  Tailscale's free tier).

## Interaction with other networking fields

Tunnels layer over your backing Service. Other networking fields apply to the
Service, not to traffic arriving through the tunnel:

- **`hostname` / external-dns**: applies to the Service, not the tunnel address.
  If your tunnel provider assigns a public address, set your DNS CNAME to that
  address, not to the `hostname` field.
- **`sourceRanges` (ingress allowlist)**: applies to the Service port. Traffic
  through the tunnel originates from the relay's address, not the player's,
  so `sourceRanges` does not gate tunnel traffic. The relay (frp, Tailscale, or
  playit) is responsible for authentication and access control.
- **`portOverrides` (NodePort specifics)**: applies to NodePort-mode exposures
  only. Tunnels ignore this entirely.

> **Planned:** Gameplane will report configuration mismatches as a status
> condition (informational) if you set conflicting options — for example, both a
> public `expose: LoadBalancer` and a tunnel. This will not be an error, but will
> indicate redundant configuration.

## Interaction with idle auto-sleep and wake-on-connect

Idle auto-sleep and tunnels compose; a few things to know:

- **The tunnel pod stays awake.** Only the game pod sleeps. When the game enters
  idle sleep, the tunnel pod continues to hold the relay connection and the
  public address remains advertised.
- **Wake-on-connect still works.** If a player connects to the public address
  while the game is asleep, the relay wakes the game pod (assuming
  `spec.idle.wakeOnConnect` is enabled). The player's connection is handed off
  to the newly-started game pod.
- **Potential reconnect.** Depending on the relay provider and your expose mode,
  a player may need to reconnect once after the game wakes — the relay may drop
  the dormant connection and require a new dial to the newly-running pod.
  (frp is stateless about game connections, so reconnects are expected. Tailscale
  handles this more gracefully. playit is in between.)

If you want to avoid reconnects on wake, consider:

- Using Tailscale (handles connection state better).
- Keeping `spec.idle.enabled: false` and stopping servers manually.
- Disabling `spec.idle.wakeOnConnect` so players only join running servers.

## Troubleshooting

### No address appears in `status.endpoints`

1. **Check the tunnel pod is running:**
   ```bash
   kubectl -n gameplane-games get pods -l gameplane.local/tunnel=<server-name>
   ```
   Look for a pod in `Running` or `Ready 1/1`.

2. **Check tunnel pod logs:**
   ```bash
   kubectl -n gameplane-games logs -f <tunnel-pod-name>
   ```
   Look for connection errors, auth failures, or provider-specific errors.

3. **Verify the credential Secret:**
   ```bash
   kubectl -n gameplane-games get secret <secret-name> -o yaml
   ```
   Ensure the keys match the provider's requirements (`token` for frp, `authKey`
   for Tailscale, `secretKey` for playit).

4. **Check NetworkPolicy.**
   By default, `networkPolicies.enabled=true` applies a default-deny-egress
   policy to the games namespace. The tunnel pod must have egress to your relay.
   
   > **Planned:** Gameplane will automatically create a NetworkPolicy rule
   > granting the tunnel pod egress to the relay's address (for frp and playit;
   > Tailscale traffic is handled differently). For now, manually add a
   > NetworkPolicy if needed.
   
   If the tunnel pod cannot reach the relay:
   ```bash
   kubectl -n gameplane-games exec <tunnel-pod-name> -- \
     nc -zv <relay-address> <relay-port>
   ```
   If this fails, the NetworkPolicy may be overly restrictive. Check the
   `NetworkPolicy` resources in the namespace and ensure `podSelector` includes
   the tunnel pod.

### Address keeps changing (playit.gg)

On playit's free tier, the assigned address is not persistent — it may change if
the tunnel pod restarts or if your daily 8-hour window resets. Upgrade to a paid
plan for stable addresses, or accept that players must rejoin after address
changes.

### Players can connect but get `Connection refused`

1. Verify the game pod is `Running` and `Ready 1/1`:
   ```bash
   kubectl -n gameplane-games get pods <game-server-name>-0
   ```

2. Check the game is listening on the expected port. Most games bind to
   `0.0.0.0`, so `netstat` inside the pod will show it:
   ```bash
   kubectl -n gameplane-games exec <game-server-name>-0 -- netstat -tlnp
   ```

3. Verify the relay is correctly configured to forward the port. For frp, check
   your `frps.toml` and the `remotePorts` in the GameServer spec match.

### Tailscale: server not appearing in `tailscale status`

1. Check the pod logs:
   ```bash
   kubectl -n gameplane-games logs -f <tailscale-tunnel-pod>
   ```

2. Verify the auth key is valid and not expired. Log into
   [login.tailscale.com](https://login.tailscale.com) and check **Settings →
   Keys** — expired keys will be marked.

3. If the pod joined but the name is wrong, check the `hostname` in the GameServer spec:
   ```bash
   kubectl -n gameplane-games get gameserver <server-name> -o jsonpath='{.spec.networking.tunnel.tailscale.hostname}'
   ```
   If empty, the device defaults to the GameServer name.

## Security

- **Credentials are Secrets.** The relay authentication credentials (frp token,
  Tailscale auth key, playit secret key) are stored as Kubernetes Secrets in the
  games namespace and are never surfaced through the API to users. Only
  cluster-admin can read them directly.
- **Tunnel images are signed.** The tunnel relay clients (frp, Tailscale,
  playit) run as container images published and cosign-signed alongside other
  Gameplane images. Verify them the same way: with `cosign verify --key
  cosign.pub`.
- **Relay trust model.** When you route traffic through a relay, you are
  trusting the relay operator (whether it's your own frps VPS, Tailscale Inc.,
  or playit.gg) not to inspect or modify player connections. For production
  use, frp (self-hosted) gives you full control; Tailscale and playit are
  third-party services whose privacy policies should be reviewed before use.
- **Tailscale is invite-only.** Tailscale tunnels expose servers only to devices
  already on your tailnet, so they do not introduce new attack surface to the
  public internet.
- **playit and frp are public.** Both expose servers to the public internet; use
  the same security practices as you would for a public-facing LoadBalancer
  (e.g., disable admin consoles, run security updates, restrict in-game
  permissions).
