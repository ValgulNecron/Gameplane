# Held review candidates: tunnel (OD-019, off-git)

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `tunnel/specs.md`; package doc `tunnel/main.go:1-18`; `docs/tunnels.md`; CRD field docs `operator/api/v1alpha1/gameserver_types.go:404-451`
- **Companion notes**: `audit/evidence/review-tunnel/notes.md` (it links here only as "held candidates: 2")

These candidates concern input validation and tailnet authorization. The descriptions are defensive: the control, where it lives, what correct behaviour is, and how a maintainer confirms it holds. Everything below comes from reading the code. Nothing was run against a cluster.

## Candidate findings

### H-tunnel-01: GameServer spec values go into the frpc TOML config without escaping (only the token is escaped)

- **Location**: `tunnel/main.go:304-309` (`serverAddr = "%s"` gets `cfg.FrpServerAddr` unescaped), `tunnel/main.go:325-332` (`name = "%s"` gets the proxy name from `BACKING_SERVICE_PORT` unescaped); `escapeTomlString` (`tunnel/main.go:424-432`) is applied only to the token at `:309`
- **Category**: correctness (security control: input validation)
- **Suggested severity**: S3
- **Control and its stated promise**:
  - The renderer escapes a TOML string value with `escapeTomlString` so the rendered file has exactly the keys the supervisor intends.
  - Where the values come from: `FRP_SERVER_ADDR` is `spec.networking.tunnel.frp.serverAddr` and each proxy name is `spec.networking.tunnel.frp.remotePorts[].name` (`operator/internal/controller/gameserver_tunnel.go:246,249,522-543`). Both are user-editable GameServer fields. The CRD bounds only their length (`gameserver_types.go:407-409` serverAddr MinLength 1 / MaxLength 253; `:427-429` name MinLength 1 / MaxLength 63), with no pattern. The field doc says serverAddr "is the hostname or IP of the frps server" (`gameserver_types.go:406`).
  - The API passes the GameServer spec through generically and checks only `credentialsSecretRef` for the tunnel block (`api/internal/handlers/resources.go:544-553`).
- **Observation**:
  1. A `serverAddr` or `remotePorts[].name` containing a double quote and a line break passes CRD validation (only lengths are checked).
  2. `renderFrpConfig` writes that value between literal `"` characters without escaping, so the value can end the string and start new lines of frpc configuration.
  3. So the frpc config the supervisor runs may hold keys and sections the operator never generated.
- **Correct behaviour**: Every value put into the frpc TOML is either escaped (`escapeTomlString`, or a real TOML encoder) or rejected before rendering: `serverAddr` must parse as a hostname or IP, and proxy names must be DNS labels, enforced by a CRD `Pattern` and/or by `loadConfig`. Then the rendered file holds exactly the generated keys, whatever the input.
- **How a maintainer confirms it holds**: Add a unit test that calls `renderFrpConfig` with a `FrpServerAddr` and a port-mapping name that each contain `"` and `\n`. Parse the written file with a TOML parser and assert that the only top-level keys are `serverAddr`, `serverPort`, `auth.method`, `auth.token` and `proxies`, and that each proxy has only `name`, `type`, `localIP`, `localPort`, `remotePort`. Alternatively, `kubectl apply --dry-run=server` a GameServer whose `serverAddr` holds a quote and should get a validation error once a pattern exists.

### H-tunnel-02: Tailscale ACL tags are documented as applied at device registration but never reach tailscaled

- **Location**: `tunnel/main.go:169` (`TAILSCALE_TAGS` read), `tunnel/main.go:291` (only hostname and auth key passed to the renderer), `tunnel/main.go:378-397` (config holds `version`, `authKey`, `hostname` only), `tunnel/main.go:478-482` (no tag flag)
- **Category**: docs-drift (security control: tailnet authorization)
- **Suggested severity**: S3. The component spec lists this as a known gap (`tunnel/specs.md:63,119`). The user-facing docs and the CRD field doc don't.
- **Control and its stated promise**:
  - `docs/tunnels.md:183-184`: "The tags are Tailscale ACL tags applied at device registration."
  - `operator/api/v1alpha1/gameserver_types.go:447`: "Tags are Tailscale ACL tags applied at device registration."
  - `docs/tunnels.md:394-396` rests the security posture on the tailnet ACL: "Tailscale tunnels expose servers only to devices already on your tailnet".
- **Observation**:
  1. The operator sets `TAILSCALE_TAGS` from `spec.networking.tunnel.tailscale.tags` (`gameserver_tunnel.go:267-282`).
  2. `loadConfig` stores it in `cfg.TailscaleTags` (`main.go:169`), and nothing reads that field afterwards (grep `TailscaleTags` in `tunnel/main.go`: only `:111` and `:169`).
  3. The tailscaled declarative config has no `AdvertiseTags` field, and the command line has no tag flag. So the device registers with whatever identity the auth key carries, not the tags the admin set. Any tailnet ACL rules written against those tags (`tag:gameplane` in the doc example) don't apply to the device.
- **Correct behaviour**: Either the tags from `spec.networking.tunnel.tailscale.tags` are passed to tailscaled at registration (for example an `AdvertiseTags` entry in the declarative config), or `docs/tunnels.md:183-184` and the CRD field doc say the tags are not applied yet, as `tunnel/specs.md:119` does.
- **How a maintainer confirms it holds**: Extend `TestRenderTailscaleConfig` to pass tags and assert that the decoded config carries them. On a live tailnet, create a tunnel with `tags: [tag:gameplane]` and check with `tailscale status --json` (the device's `Tags`) or in the admin console that the device shows `tag:gameplane`.

## Moved from audit/evidence/review-tunnel/notes.md (OD-019, 2026-09-24)

### Q-tunnel-empty-credential

- `readCredentials` accepts an empty or whitespace-only credential file (`main.go:270-276` returns `"", nil`), so frp renders `auth.token = ""`, tailscale omits `authKey`, and playit writes an empty secret file. `specs.md:79` says "Missing credentials are fatal errors". Is an empty value meant to count as missing?

### Q-tunnel-toml-escaping

- `escapeTomlString` (`main.go:424-432`) escapes `\\ " \n \r \t` but not the other control characters that TOML basic strings forbid (U+0000-U+001F, U+007F). A token containing one renders invalid TOML, frpc exits non-zero, and the supervisor retries forever. Is that worth hardening?

### OBS-tunnel-tailscale-tags

Sentence from the Observations bullet on `TAILSCALE_TAGS`:

The part of it that contradicts user-facing docs is held (see below).
