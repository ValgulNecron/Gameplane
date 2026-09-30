# T045 libs chunk: independent verification (opus)

Held candidates for `tunnel/` (OD-019, off-git). The companion public verification is `audit/evidence/review-tunnel/verification.md`, which does not mention these items.

## Method

I tried to refute both candidates in `audit/held/review-tunnel.md` against master `13a859ff`. None of the cited files differ between this branch and master. I read `tunnel/main.go` in full (config loading, `renderFrpConfig`, `escapeTomlString`, `renderTailscaleConfig`, `buildCommand`) and `tunnel/specs.md` (Known Gaps). I read the operator side of the env contract: `operator/internal/controller/gameserver_tunnel.go:240-282` (env construction), `:362-492` (the tunnel egress NetworkPolicy) and `:522-541` (`buildFrpRemotePortsConfig`). I also read the CRD fields and markers in `operator/api/v1alpha1/gameserver_types.go:404-452`, the API's GameServer write validation in `api/internal/handlers/resources.go:544-554`, and `docs/tunnels.md:176-186` and `:390-396`. I checked `audit/held/` for overlaps: `review-operator.md` H-operator-05 and H-operator-06 concern the playit egress rule and credential-Secret docs, not these controls. Everything below comes from reading the code. I ran no test or lint suite and nothing against a cluster or a tailnet.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| H-tunnel-01 | kept | S3 | Confirmed. `renderFrpConfig` puts `FrpServerAddr` and each proxy name between literal quotes with no escaping. Only the token goes through `escapeTomlString`. Both values come from user-editable GameServer fields whose CRD markers bound only their length, with no pattern. The API checks only `credentialsSecretRef` in the tunnel block. So the rendered frpc file can hold keys the operator never generated. Whoever can do this can already edit that GameServer's tunnel settings, and the tunnel pod's egress is limited to DNS, the frps port and the advertised ports. So this is a missing input-validation layer, not an outright boundary break: S3. |
| H-tunnel-02 | kept | S3 | Confirmed. `TAILSCALE_TAGS` is read into `cfg.TailscaleTags` (`main.go:169`) and never used after that. The rendered config has no tags, and the command line has no tag flag. `tunnel/specs.md:63,119` records this as a known gap. The user-facing guide (`docs/tunnels.md:183-184`) and the CRD field doc (`gameserver_types.go:447`) still say the tags are "applied at device registration", and the guide's security notes rely on tailnet ACLs. The fix can be code or docs. The device's real tailnet identity comes from the auth key, not from the tags. |

### H-tunnel-01

**Location:** `tunnel/main.go:304-309` (`serverAddr = "%s"` gets `cfg.FrpServerAddr` unescaped); `tunnel/main.go:325-332` (`name = "%s"` gets the proxy name from `BACKING_SERVICE_PORT` unescaped); `tunnel/main.go:424-432` (`escapeTomlString`, applied only to the token at `:309`). Value sources: `operator/internal/controller/gameserver_tunnel.go:246,249,522-541`; CRD markers `operator/api/v1alpha1/gameserver_types.go:406-409` (`serverAddr`: MinLength 1, MaxLength 253) and `:426-429` (`remotePorts[].name`: MinLength 1, MaxLength 63).

**Control:** The frpc config renderer must produce a file that holds exactly the keys the supervisor intends, whatever the GameServer spec contains. The intended layers are escaping in the renderer (`escapeTomlString`, or a real TOML encoder) and format validation of the inputs (a hostname or IP for `serverAddr`, a DNS label for proxy names) in the CRD schema and/or `loadConfig`.

**Repro / observation (defensive; confirms whether the control holds, no payload):**
1. Read `tunnel/main.go:304-309`. Of the three string values in the header, only `auth.token` goes through `escapeTomlString`. `serverAddr` is inserted as is.
2. Read `tunnel/main.go:313-332`. Each `BACKING_SERVICE_PORT` entry is split on `,` and `:`, and the name part is inserted into `name = "%s"` as is.
3. Read `operator/api/v1alpha1/gameserver_types.go:404-432`. `ServerAddr` and `RemotePortMapping.Name` carry only length markers, with no `Pattern`. Compare `TailscaleTunnelSpec.Hostname` at `:440-445`, which does have an RFC 1123 pattern.
4. Read `api/internal/handlers/resources.go:544-554`. Of the tunnel block, only `credentialsSecretRef` is validated. `loadConfig` (`tunnel/main.go:141-162`) checks only that the values are non-empty.
5. So a `serverAddr` or `remotePorts[].name` containing a double quote and a line break passes every layer, and the rendered file can gain keys the operator never generated.

**Expected:** Every value written into the frpc TOML is either escaped or rejected before rendering. `serverAddr` parses as a hostname or IP, and proxy names are DNS labels, enforced by a CRD `Pattern` and/or `loadConfig`. The rendered file then holds exactly the generated keys.

**How a maintainer confirms it holds:** Add a unit test that calls `renderFrpConfig` with a `FrpServerAddr` and a port-mapping name that each contain `"` and `\n`, parses the written file with a TOML parser, and asserts that the only top-level keys are `serverAddr`, `serverPort`, `auth` and `proxies`, and that each proxy has only `name`, `type`, `localIP`, `localPort` and `remotePort`. Once a CRD pattern exists, also check that `kubectl apply --dry-run=server` of a GameServer whose `serverAddr` holds a quote gets a validation error.

**Actual:** Only the token is escaped. The server address and proxy names reach the TOML file unescaped and unvalidated beyond their length.

### H-tunnel-02

**Location:** `tunnel/main.go:169` (`TAILSCALE_TAGS` read), `tunnel/main.go:291` (only the hostname and auth key are passed to the renderer), `tunnel/main.go:378-397` (the config holds `version`, `authKey` and `hostname` only), `tunnel/main.go:478-482` (no tag flag); `docs/tunnels.md:183-184`; `operator/api/v1alpha1/gameserver_types.go:447`.

**Control:** Tailnet authorization of the tunnel device. The docs and the CRD tell admins that `spec.networking.tunnel.tailscale.tags` become the device's ACL tags at registration, so tailnet ACL rules written against those tags (the guide uses `tag:gameplane`) govern who can reach the device and what it can reach. `docs/tunnels.md:394-396` rests the Tailscale security posture on the tailnet.

**Repro / observation (defensive; confirms whether the control holds):**
1. `operator/internal/controller/gameserver_tunnel.go:260-282` joins `spec.networking.tunnel.tailscale.tags` into `TAILSCALE_TAGS`.
2. `tunnel/main.go:169` stores it in `cfg.TailscaleTags`. `grep -n TailscaleTags tunnel/main.go` finds only the struct field (`:111`) and that assignment (`:169`).
3. `renderTailscaleConfig` (`:378-397`) writes only `version`, `authKey` and `hostname`, and `buildCommand` (`:478-482`) passes no tag argument. So tailscaled registers with whatever identity the auth key carries.
4. `tunnel/specs.md:63` and `:119` list this as a known gap. `docs/tunnels.md:183-184` ("The tags are Tailscale ACL tags applied at device registration.") and the CRD field doc at `gameserver_types.go:447` do not.

**Expected:** Either the configured tags reach tailscaled at registration (in the declarative config or by an equivalent mechanism) and the device shows them, or `docs/tunnels.md:183-184` and the CRD field doc say the tags are not applied yet, as `tunnel/specs.md:119` does, and tell admins to put the tags on the auth key instead.

**How a maintainer confirms it holds:** Extend `TestRenderTailscaleConfig` to pass tags and assert that the decoded config carries them. On a test tailnet, create a tunnel with `tags: [tag:gameplane]` and check in the admin console, or with `tailscale status --json` from another node, that the device shows `tag:gameplane`.

**Actual:** The tags are accepted by the CRD and passed to the pod, then dropped. The device carries only the auth key's identity, while the guide and the CRD doc say it carries the configured tags.
