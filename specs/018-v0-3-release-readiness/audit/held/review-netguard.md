# Held review candidates: netguard (OD-019, off-git)

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `netguard/specs.md`; package doc `netguard/netguard.go:1-27`; `docs/security.md:300-368`; `README.md:146`; `SECURITY_AUDIT.md:155`
- **Companion notes**: `audit/evidence/review-netguard/notes.md` (it links here only as "held candidates: 4")

These candidates concern the SSRF dial guard, a security control. They stay here until the fix merges. The descriptions are defensive: the control, where it lives, what correct behaviour is, and how a maintainer confirms it holds. Everything below comes from reading the code and the stdlib predicate sources. Nothing was run against a cluster.

## Candidate findings

### H-netguard-01: The operator policy (`IsAllowed`) refuses metadata endpoints only by link-local range; two documented cloud metadata endpoints are outside that range

- **Location**: `netguard/netguard.go:95-110` (`IsAllowed`), `netguard/netguard.go:137-140` (`metadataHosts`)
- **Category**: correctness (security control)
- **Suggested severity**: S3. It is defense in depth: every `IsAllowed` caller takes an admin-configured destination (ModuleSource, notification sink, GameServer RCON address).
- **Control and its stated promise**:
  - `specs.md:8`: the guard exists "so they cannot be aimed at cloud instance-metadata endpoints"
  - `netguard.go:1-4`: "so they cannot be aimed at the cloud instance-metadata endpoint"
  - `specs.md:76`: both policies block "ranges that can be exploited to reach … cloud metadata endpoints"
  - `SECURITY_AUDIT.md:155` (a new aspect of an already-tracked item): "`IsAllowed` (operator) and `IsPublic` (agent) … refuse … the Kubernetes pod/node CGNAT ranges". `IsAllowed` has no CGNAT refusal.
- **Observation**:
  1. `IsAllowed` refuses only unspecified, multicast, link-local, `64:ff9b::/96` and `2002::/16` (`netguard.go:99-108`). It allows RFC1918, ULA (`fc00::/7`) and RFC 6598 CGNAT (`100.64.0.0/10`), all on purpose.
  2. The AWS EC2 IPv6 instance-metadata endpoint `fd00:ec2::254` is inside `fc00::/7`, so `IsAllowed(net.ParseIP("fd00:ec2::254"))` returns `true`.
  3. The Alibaba Cloud ECS metadata endpoint `100.100.100.200` is inside `100.64.0.0/10`, so `IsAllowed(net.ParseIP("100.100.100.200"))` returns `true`.
  4. `HostIsMetadata` knows only the GCP names (`metadata.google.internal`, `metadata`). It is an extra check by name and doesn't close the address gap.
- **Correct behaviour**: `IsAllowed` refuses the known instance-metadata addresses by exact match, whatever range they fall in (at least `169.254.169.254`, `fd00:ec2::254` and `100.100.100.200`). RFC1918, ULA and loopback stay allowed for self-hosted registries. `SECURITY_AUDIT.md:155` then states which policy refuses CGNAT.
- **How a maintainer confirms it holds**: Add `{"fd00:ec2::254", false}` and `{"100.100.100.200", false}` to the `TestIsAllowed` table (`netguard_test.go:14-35`) and let CI run it. Today both rows would fail. The fix holds when they pass and the existing `10.0.0.1`/`fc00::1`/`127.0.0.1` → `true` rows still pass.

### H-netguard-02: The agent policy (`IsPublic`) promises "only globally routable unicast addresses" but accepts several non-global ranges

- **Location**: `netguard/netguard.go:117-132` (`IsPublic`), `netguard/netguard.go:77-89` (`reservedBlocks`)
- **Category**: correctness (security control)
- **Suggested severity**: S3. Whether these ranges can be reached depends on the node's network: a local-use NAT64 gateway, or an IPv4-compatible tunnel device that is up. Reachability was not shown.
- **Control and its stated promise**:
  - `specs.md:42`: "Allows only globally routable unicast addresses"
  - `netguard.go:17-18`: "only globally routable unicast addresses are allowed"
  - `docs/security.md:344`: "only globally routable addresses are allowed"
- **Observation**: `IsPublic` is a denylist over Go's predicates plus `reservedBlocks`. The following addresses match none of those checks, so `IsPublic` returns `true` for each:
  1. `0.0.0.1`, from `0.0.0.0/8` ("this network"). `IsUnspecified` matches only `0.0.0.0` exactly (stdlib `ip.Equal(IPv4zero)`).
  2. IPv4-compatible IPv6 addresses (deprecated `::/96`), for example `::7f00:1` or `::a9fe:a9fe`. `normalize` relies on `To4()`, which unwraps only `::ffff:0:0/96`, so these stay 16-byte and miss the IPv4 checks.
  3. `64:ff9b:1::/48`, the RFC 8215 local-use IPv4/IPv6 translation prefix, for example `64:ff9b:1::a9fe:a9fe`. Only the well-known `64:ff9b::/96` is listed.
  4. `100::/64`, the RFC 6666 discard-only prefix, for example `100::1`.

  The IANA special-purpose registries mark every one of these "not globally reachable" (or reserved).
- **Correct behaviour**: `IsPublic` returns `false` for all of the above. One robust shape for IPv6 is an allowlist of `2000::/3` minus the special-use blocks, plus explicit `0.0.0.0/8`, `::/96` and `64:ff9b:1::/48` entries.
- **How a maintainer confirms it holds**: Add `0.0.0.1`, `::7f00:1`, `::a9fe:a9fe`, `64:ff9b:1::a9fe:a9fe` and `100::1` to the `blocked` list in `TestIsPublic` (`netguard_test.go:54-66`). CI must pass with them, and the existing public cases `8.8.8.8` and `2606:4700:4700::1111` must stay allowed.

### H-netguard-03: The operator policy (`IsAllowed`) refuses only the well-known NAT64 prefix; other forms that embed a link-local IPv4 pass

- **Location**: `netguard/netguard.go:65-68` (`blockedV6Prefixes`), `netguard/netguard.go:103-108`
- **Category**: correctness (security control)
- **Suggested severity**: S4. It depends on network topology, as in H-netguard-02, and the callers are admin-configured.
- **Control and its stated promise**:
  - `specs.md:41`: `IsAllowed` "disallows … NAT64/6to4 prefixes that can wrap link-local addresses"
  - `specs.md:78`: NAT64 and 6to4 prefixes "can wrap IPv4 addresses, including link-local (169.254.0.0/16). Both policies block them defensively."
  - `netguard.go:14-15`
- **Observation**:
  1. `IsAllowed(net.ParseIP("64:ff9b:1::a9fe:a9fe"))` returns `true`. This is the RFC 8215 local-use NAT64 prefix wrapping `169.254.169.254`.
  2. `IsAllowed(net.ParseIP("::a9fe:a9fe"))` returns `true`. This is the IPv4-compatible form of `169.254.169.254`, which `normalize` doesn't unwrap (see H-netguard-02 item 2).
- **Correct behaviour**: `IsAllowed` refuses the local-use NAT64 prefix `64:ff9b:1::/48`, or at least any NAT64 address whose embedded IPv4 is link-local. It also refuses (or unwraps and re-checks) IPv4-compatible `::/96` forms. Operator-chosen network-specific NAT64 prefixes (RFC 6052 NSP) can't be listed generically, so the spec sentence should narrow its promise to the prefixes that are actually covered.
- **How a maintainer confirms it holds**: Add `{"64:ff9b:1::a9fe:a9fe", false}` and `{"::a9fe:a9fe", false}` to `TestIsAllowed` and let CI run it.

### H-netguard-04: README says netguard also guards "OCI module fetches"; the operator's OCI client dials without the guard

- **Location**: `operator/internal/oci/client.go:37` (`&http.Client{Transport: retry.NewTransport(&http.Transport{})}`, with no guarded `DialContext`); claim at `README.md:146`; broader wording at `netguard/specs.md:8` and `netguard/netguard.go:1-4` ("module-source fetches")
- **Category**: correctness / docs-drift (security control coverage). This is cross-component, and the operator reviewer may report the same location, so dedupe against the operator chunk.
- **Suggested severity**: S3. ModuleSource mutation is admin-only (`docs/security.md:314-315`), so this is defense in depth.
- **Observation**:
  1. `README.md:146`: "`netguard/` | Go | SSRF protection layer for outgoing mod downloads and OCI module fetches."
  2. The OCI ModuleSource path (`operator/internal/modsrc/oci.go` through `operator/internal/oci/client.go`) builds its HTTP client from a plain `http.Transport`. Grepping `operator/internal` for `netguard` finds only `modsrc/http.go` and `modsrc/git.go`.
  3. `docs/security.md:308-309` limits its claim correctly to "the operator's `git`/`http` source fetchers". The e2e guard test `test/e2e/modulesource_ssrf_e2e_test.go:39-58` covers only the `http` and `git` source types.
- **Correct behaviour**: One of two things. Either the OCI client dials through `netguard.Dialer(…, netguard.IsAllowed)` (or `netguard.HTTPClient`), keeping `retry.NewTransport` as the outer wrapper, so OCI sources get the same dial-time refusal as git and http. Or, if leaving OCI unguarded is intended, `README.md:146` and the netguard spec's Purpose say "git/http module-source fetches".
- **How a maintainer confirms it holds**: Read `operator/internal/oci/client.go` and check that the transport's `DialContext` comes from `netguard`. Then add an `oci`-type row to `TestModuleSource_RejectsSSRFTarget` (bucket `operator`) that points at a link-local registry host and expects the fetch-failure condition the http and git rows already assert.

## Questions (not findings)

- `parseCIDRs` (`netguard/netguard.go:44-52`) silently skips any CIDR literal that fails to parse. Every current literal parses, so nothing is dropped today. But `192.0.0.0/24` (`netguard.go:79`) is the one `reservedBlocks` entry with no `TestIsPublic` row, so a future typo in it would silently weaken `IsPublic` without failing any test. Should a malformed literal panic at init, the way `regexp.MustCompile` does, and should that entry get a test row?

## Moved from audit/evidence/review-netguard/notes.md (OD-019, 2026-09-24)

### C-netguard-01: The spec, package doc and CLAUDE.md say there are two consumers (operator uses IsAllowed, agent uses IsPublic); in the code the API is a third importer and the agent also uses IsAllowed

- **Location**: `netguard/specs.md:8`, `netguard/specs.md:94-95`, `netguard/netguard.go:6-21`, `CLAUDE.md:56`, `CLAUDE.md:250`, `docs/architecture.md:273-276`, `docs/dependencies.md:333-339`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:8`: "Used by both the operator (reconciliation) and the agent (sidecar) via two deliberately different policies". The package doc (`netguard.go:6-9,17`) says: "It is shared by the operator and the agent … IsAllowed (operator, permissive) … IsPublic (agent, strict)". `CLAUDE.md:250`: "Dial-time SSRF prevention (`IsAllowed` for operator, `IsPublic` for agent)". `CLAUDE.md:56`: "SSRF dial-guard (Go) — operator & agent".
  2. `grep -rn 'netguard\.' --include='*.go'`, outside the module and excluding tests, finds more importers:
     - `api/internal/notify/notify.go:86` and `deliver.go:172` use `netguard.IsAllowed`
     - `api/internal/steam/resolver.go:45` uses `netguard.HTTPClient(opts.Timeout, netguard.IsPublic)`
     - `agent/internal/rcon/websocket.go:302` uses `netguard.HTTPClient(dialTimeout, netguard.IsAllowed)`, so the agent uses the permissive policy too
  3. `docs/dependencies.md:333-339` corrects this only in part. It says there are "three importers" (operator, agent, `api/internal/notify`) and leaves out `api/internal/steam` (IsPublic) and the agent's IsAllowed use.
  4. The spec's References section (`specs.md:94-95`) lists only `operator/internal/modsrc/http.go` and `agent/internal/mods/mods.go`.
- **Expected**: The spec, the package doc and CLAUDE.md name all importers, and the policy each one uses: operator modsrc git/http → IsAllowed; agent mods → IsPublic; agent websocket RCON → IsAllowed; API notify → IsAllowed; API Steam resolver → IsPublic.
- **Actual**: The docs describe a strict operator=IsAllowed / agent=IsPublic split. The code has two more call sites, and the agent uses both policies.

### OBS-netguard-wider-claim

Sentence from the Observations bullet on `docs/security.md:308-309`:

Other docs make a wider claim; see the held file.
