# T045 guards chunk: independent verification (opus)

Held under OD-019, off-git. This file covers the four candidates in `held/review-netguard.md`. The wording is defensive: for each candidate it names the control, where it lives, the correct behaviour, and how a maintainer confirms the control holds. It contains no misuse walkthroughs.

**Method.** I tried to refute each candidate against master `13a859ff`. `netguard/`, `operator/`, `agent/`, `api/` and `docs/` are the same on this branch and on master. Only `SECURITY_AUDIT.md` differs.

I read `netguard/netguard.go`, `netguard/specs.md` and `netguard/netguard_test.go` in full. I read every netguard call site in context and checked who controls each destination:
- **Operator git/http ModuleSources** (`operator/internal/modsrc/{http,git}.go`): admin-only (`docs/security.md:314-315`).
- **API notification sinks** (`api/internal/notify`): mounted under `/admin/…` and gated on `config:manage` (`docs/security.md:354-362`).
- **Agent WebSocket RCON** (`agent/internal/rcon/websocket.go:302`): fixed at the `--rcon-host` default `127.0.0.1` (`agent/cmd/main.go:65`). No operator controller sets that flag.
- **Agent mod downloads** (`agent/internal/mods/mods.go`): user-supplied URLs, but only after the host passes the template's `allowedHosts`, and every redirect is checked again (`mods.go:187`, `:437-439`, `:813-830`, `:849-859`).
- **API Steam resolver** (`api/internal/steam/resolver.go:45-49`): a fixed host.

I also read `operator/internal/oci/client.go:32-39` and `operator/internal/modsrc/{fetcher,oci}.go`. I read `held/review-operator.md` and `held/review-agent.md` to look for duplicates, and `audit/findings.md`, where none of these candidates is tracked.

I ran two throwaway programs in the session scratchpad. Neither is a test or lint suite, and neither changed a repo file:
1. The first calls `netguard.IsAllowed` and `netguard.IsPublic` on each address the candidates cite.
2. The second opens a local TCP listener on this Linux 7.2 host, in its default network namespace, and dials it through each non-global address. Loopback reached it. `0.0.0.1` timed out on the default route. `::7f00:1`, `100::1` and `64:ff9b:1::7f00:1` each returned "network is unreachable".

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| H-netguard-01 | kept | S3 | Confirmed. `IsAllowed(fd00:ec2::254)` and `IsAllowed(100.100.100.200)` both return `true`. The first is the AWS IPv6 IMDS address, inside ULA. The second is the Alibaba Cloud ECS metadata address, inside CGNAT. `IsAllowed` allows both ranges on purpose, but its documented purpose is to keep fetches away from instance-metadata endpoints (`netguard.go:1-4`, `specs.md:8,76`, `docs/security.md:308-313`). `SECURITY_AUDIT.md` also says both policies refuse CGNAT: master `:141` claims it "confirmed all cloud metadata ranges … are blocked", and the branch `:155` says the same. Every `IsAllowed` destination is admin-configured or fixed to loopback, so this is defense in depth: S3, not S1. Corrections to the candidate: the agent's RCON address is fixed to loopback, not admin-configured, and the master line for the `SECURITY_AUDIT.md` claim is `:141`. |
| H-netguard-02 | kept | S4 | Confirmed as a mismatch between the contract and the code. `IsPublic` returns `true` for `0.0.0.1`, `::7f00:1`, `::a9fe:a9fe`, `64:ff9b:1::a9fe:a9fe` and `100::1`, contradicting "only globally routable unicast addresses" (`specs.md:42`, `netguard.go:17-18`, `docs/security.md:344`). No path to an internal host was shown in a real install. `IsPublic` sits behind the `allowedHosts` check and the redirect re-check. Three of the four ranges don't route anywhere local in a default netns (probe). The fourth, RFC 8215 `64:ff9b:1::/48`, matters only where a local-use NAT64 translator is on the pod's path. So the reviewer's S3 is lowered to S4. H-netguard-03 is merged into this item because it has the same root cause and fix. |
| H-netguard-03 | rejected | n/a | Merged into H-netguard-02. Both come from the same two gaps: `normalize` (`netguard.go:56-61`) unwraps only `::ffff:0:0/96`, and the NAT64 entries (`:65-68` for `IsAllowed`, `:85` for `IsPublic`) have only the well-known `64:ff9b::/96`. One change, with one test update, fixes both, so tracking them separately would duplicate the work. On its own it would be below S4: `specs.md:78` names exactly `64:ff9b::/96` and `2002::/16`, and the code blocks both; `::/96` doesn't route in a pod netns; and `IsAllowed` destinations are admin-configured. The `IsAllowed` test rows are carried in H-netguard-02's confirmation step. |
| H-netguard-04 | rejected | n/a | Duplicate of held **H-operator-03** (`held/review-operator.md:30-37`). That item has the same code location (`operator/internal/oci/client.go:37`, a plain `http.Transport` with no guarded dialer, reached through `modsrc/fetcher.go` → `modsrc/oci.go`) and the same two fix options, and it is broader because it also covers the cosign registry client (`verify.go:100-110`). The fact itself is confirmed: grepping `operator/internal` for `netguard` finds only `modsrc/http.go` and `modsrc/git.go`. The only thing this candidate adds is the `README.md:146` wording ("SSRF protection layer for outgoing mod downloads and OCI module fetches"). That should be fixed as part of H-operator-03's docs option. If the operator-chunk verifier rejects H-operator-03, this candidate needs another look. |

### H-netguard-01

**Location:** `netguard/netguard.go:95-110` (`IsAllowed`) and `netguard/netguard.go:134-145` (`metadataHosts`, `HostIsMetadata`). The promise is at `netguard/netguard.go:1-4`, `netguard/specs.md:8,76` and `docs/security.md:308-313`. The CGNAT claim is at `SECURITY_AUDIT.md:141` (master) and `SECURITY_AUDIT.md:155` (branch).

**Control:** `IsAllowed` is the permissive dial-time address policy. It is used for git/http ModuleSource fetches, notification-sink delivery (HTTP and SMTP), and the agent's loopback WebSocket RCON. It keeps RFC 1918, ULA, loopback and CGNAT reachable for self-hosted endpoints, and it is meant to refuse cloud instance-metadata endpoints. Today it does that only through the link-local range.

**Repro / observation** (by reading master `13a859ff`, or with a throwaway `main` that calls the exported function):
1. Read `netguard/netguard.go:99-108`. `IsAllowed` refuses unspecified, multicast, interface-local multicast, link-local unicast and link-local multicast addresses, plus `64:ff9b::/96` and `2002::/16`. Nothing else is refused. `TestIsAllowed` pins `fc00::1 → true` (`netguard_test.go:25`).
2. `fd00:ec2::254`, the AWS EC2 IPv6 instance-metadata address, is inside `fc00::/7`. `100.100.100.200`, the Alibaba Cloud ECS metadata address, is inside `100.64.0.0/10`. A throwaway program that prints `netguard.IsAllowed(net.ParseIP(addr))` shows `true` for both and `false` for `169.254.169.254`.
3. `HostIsMetadata` (`netguard.go:137-145`) recognises only `metadata.google.internal` and `metadata`, so it doesn't cover these addresses by name either.
4. `IsPublic` refuses both addresses (ULA and CGNAT). Only the permissive policy is affected.

**Expected:** `IsAllowed` refuses the known instance-metadata addresses by exact match, whatever range they fall in: at least `169.254.169.254` (already refused through link-local), `fd00:ec2::254` and `100.100.100.200`. RFC 1918, ULA, loopback and the rest of CGNAT stay allowed. `SECURITY_AUDIT.md` says that only `IsPublic` refuses CGNAT.

**Actual:** Both addresses pass `IsAllowed`, so on those clouds the policy doesn't enforce its stated metadata refusal for admin-configured ModuleSources and notification sinks. Whether a given pod can reach those endpoints depends on the cloud's IMDS settings and the cluster's CNI.

**How a maintainer confirms the control holds:** Add `{"fd00:ec2::254", false}` and `{"100.100.100.200", false}` to the `TestIsAllowed` table (`netguard_test.go:14-35`) and let CI run it. Today both rows fail. The fix holds when they pass and the existing `10.0.0.1`, `fc00::1` and `127.0.0.1` → `true` rows still pass. Then read `SECURITY_AUDIT.md` and check that it no longer says `IsAllowed` refuses CGNAT.

### H-netguard-02

**Location:** `netguard/netguard.go:117-132` (`IsPublic`), `:77-89` (`reservedBlocks`) and `:56-61` (`normalize`). Merged from H-netguard-03: `:65-68` (`blockedV6Prefixes`, used by `IsAllowed` at `:103-108`). The promise is at `netguard/specs.md:41-42`, `netguard/netguard.go:14-18` and `docs/security.md:344`.

**Control:** `IsPublic` is the strict dial-time policy. It applies to agent mod downloads (after the `allowedHosts` check) and to the API's Steam resolver, and it is documented to allow "only globally routable unicast addresses". It is built as a denylist: Go's `net.IP` predicates plus `reservedBlocks`. `IsAllowed` shares `normalize` and has its own, shorter NAT64/6to4 list.

**Repro / observation** (by reading master `13a859ff`, or with a throwaway `main` that calls the exported functions):
1. Read `netguard/netguard.go:121-131`. Go's `IsUnspecified` matches only `0.0.0.0`, and `reservedBlocks` has no `0.0.0.0/8` entry, so `IsPublic(0.0.0.1)` is `true`.
2. Read `netguard/netguard.go:56-61`. `normalize` relies on `To4()`, which unwraps only `::ffff:0:0/96`. So an IPv4-compatible `::/96` address (`::7f00:1`, `::a9fe:a9fe`) stays 16 bytes and misses every IPv4 check. `IsPublic` and `IsAllowed` both return `true`.
3. `reservedBlocks` (`:85`) and `blockedV6Prefixes` (`:66`) list only the well-known NAT64 prefix. The RFC 8215 local-use prefix `64:ff9b:1::/48` is in neither, so `IsPublic` and `IsAllowed` both return `true` for `64:ff9b:1::a9fe:a9fe`.
4. `100::/64` (RFC 6666, discard-only) isn't listed, so `IsPublic(100::1)` is `true`.
5. Reachability, which is why this is S4. Mod downloads must first pass `allowedHosts` (`mods.go:187`, `:437-439`), and each redirect is checked again (`:851-858`). A local-listener probe on a default Linux netns reached nothing through `0.0.0.1`, `::7f00:1`, `100::1` or `64:ff9b:1::7f00:1`. The local-use NAT64 prefix matters only where the network deploys such a translator.

**Expected:** `IsPublic` refuses every IANA special-purpose block marked not globally reachable: at least `0.0.0.0/8`, `::/96`, `64:ff9b:1::/48` and `100::/64`. For IPv6, an allowlist of `2000::/3` minus the special-use blocks is a more robust shape than a denylist. `IsAllowed` also gets `64:ff9b:1::/48` and an unwrap-and-recheck of `::/96`, so that `specs.md:41` ("NAT64/6to4 prefixes that can wrap link-local addresses") holds. The alternative is to narrow the "only globally routable" and "NAT64/6to4" wording to the ranges that are actually refused.

**Actual:** Each listed address passes `IsPublic`, and the `::/96` and `64:ff9b:1::/48` forms also pass `IsAllowed`. No path to an internal host was shown in a default install, so today this is a gap between the documented contract and the denylist.

**How a maintainer confirms the control holds:** Add `0.0.0.1`, `::7f00:1`, `::a9fe:a9fe`, `64:ff9b:1::a9fe:a9fe` and `100::1` to the `blocked` list in `TestIsPublic` (`netguard_test.go:54-66`). Add `{"64:ff9b:1::a9fe:a9fe", false}` and `{"::a9fe:a9fe", false}` to `TestIsAllowed` (`:14-35`). CI must pass with these rows. The existing public rows (`8.8.8.8`, `2606:4700:4700::1111`) must stay allowed by `IsPublic`, and `10.0.0.1`, `fc00::1` and `127.0.0.1` must stay allowed by `IsAllowed`.

## Moved from audit/evidence/review-netguard/verification.md (OD-019, 2026-09-24)

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-netguard-01 | kept | S4 | Confirmed. The spec, the package doc and `CLAUDE.md` describe exactly two consumers, split by policy: the operator uses `IsAllowed` and the agent uses `IsPublic`. The code has a third importer, the API, which uses both policies (`IsAllowed` for notification sinks, `IsPublic` for the Steam resolver). The agent also uses both (`IsAllowed` for WebSocket RCON). `docs/dependencies.md` says "three importers" but still leaves out the Steam resolver and the agent's `IsAllowed` call site. The code's choices are sound and documented at each call site, so only the documentation is wrong. |

### C-netguard-01

**Location:** `netguard/specs.md:8` (Purpose) and `:94-95` (References); `netguard/netguard.go:6-21` (package doc); `CLAUDE.md:56` (repository map) and `CLAUDE.md:250` (architecture table); `docs/dependencies.md:333-338`. `docs/architecture.md:272-275` also names only the operator and the agent.

**Repro / observation** (by reading master `13a859ff`):
1. `netguard/specs.md:8` says "Used by both the operator (reconciliation) and the agent (sidecar) via two deliberately different policies". `netguard/netguard.go:6-21` says "It is shared by the operator and the agent", with "IsAllowed (operator, permissive)" and "IsPublic (agent, strict)". `CLAUDE.md:250` says "`IsAllowed` for operator, `IsPublic` for agent". `CLAUDE.md:56` says "operator & agent".
2. Run `grep -rn 'netguard\.' --include='*.go' . | grep -v '^./netguard/' | grep -v _test.go`. It finds these call sites:
   - `operator/internal/modsrc/http.go:42,107,110` and `operator/internal/modsrc/git.go:39,55,142,145` use `IsAllowed` and `HostIsMetadata`. This matches the docs.
   - `agent/internal/mods/mods.go:843,850` uses `IsPublic`. This matches the docs.
   - `agent/internal/rcon/websocket.go:302` is `netguard.HTTPClient(dialTimeout, netguard.IsAllowed)`, so the agent uses the permissive policy too. The comment at `:294-301` gives the reason: the target is the in-pod game, whose `--rcon-host` defaults to `127.0.0.1` (`agent/cmd/main.go:65`).
   - `api/internal/notify/notify.go:86` and `api/internal/notify/deliver.go:172` use `IsAllowed` for admin-configured notification sinks.
   - `api/internal/steam/resolver.go:45` is `netguard.HTTPClient(opts.Timeout, netguard.IsPublic)`.
3. `docs/dependencies.md:333-338` ("netguard's two policies, three importers") adds `api/internal/notify`. It still leaves out `api/internal/steam` (`IsPublic`) and the agent's `IsAllowed` call site.
4. The References section of `netguard/specs.md` (`:94-95`) lists only `operator/internal/modsrc/http.go` and `agent/internal/mods/mods.go`.

**Expected:** The netguard spec (Purpose and References), the package doc, `CLAUDE.md:56,250` and `docs/dependencies.md` name every importer and the policy each one uses:
- operator git/http module sources: `IsAllowed`
- agent mod downloads: `IsPublic`
- agent WebSocket RCON: `IsAllowed`
- API notification sinks: `IsAllowed`
- API Steam resolver: `IsPublic`

**Actual:** The docs describe a strict split, operator → `IsAllowed` and agent → `IsPublic`. The code has two more call-site groups, the API is an importer, and the agent uses both policies. A related candidate about `agent/specs.md:153` (which says WebRcon uses `IsPublic`) was raised in the agent review. It is in a different file, so it is not a duplicate of this one.
