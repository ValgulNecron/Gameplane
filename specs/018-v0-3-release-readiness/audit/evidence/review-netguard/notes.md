# Review: netguard

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `netguard/specs.md`; package doc in `netguard/netguard.go:1-27`; `CLAUDE.md` (repo map, architecture table); `docs/architecture.md:273-276`; `docs/security.md:300-368`; `docs/dependencies.md:328-339`; `README.md:146`

## Scope reviewed

In scope, read in full:
- `netguard/netguard.go` (219 lines)
- `netguard/netguard_test.go` (169 lines), read to see which behaviour is pinned
- `netguard/specs.md`
- `netguard/go.mod`, `netguard/.testcoverage.yml`

Cross-referenced, read only in part, to check what the spec and docs say about callers:
- Every `netguard.` call site outside the module, found by grep: `operator/internal/modsrc/{http,git}.go`, `agent/internal/mods/mods.go:236,841-850`, `agent/internal/rcon/websocket.go:285-310`, `api/internal/notify/{notify.go:86,deliver.go:114-174}`, `api/internal/steam/resolver.go:45,188`
- `operator/internal/oci/client.go:1-80`
- `test/e2e/modulesource_ssrf_e2e_test.go:16-60`
- Go stdlib `net/ip.go`, for the `IsPrivate`, `IsLoopback`, `IsUnspecified`, `IsLinkLocalUnicast` and `To4` semantics the policies rely on
- `SECURITY_AUDIT.md:145-165`

Not done: I ran no tests (Rule 8). The only command run was `go build ./...` in `netguard/`, which passed.

## Method

1. Compared each sentence of `specs.md` (Purpose, contracts, invariants, security considerations, testing, references) with the code.
2. Traced `IsAllowed` and `IsPublic` through the stdlib predicates by hand, for boundary and special-purpose addresses: the IANA IPv4/IPv6 special-purpose registries and the cloud metadata endpoints.
3. Grepped every importer and compared the result with the "who uses which policy" statements in the spec, the package doc, CLAUDE.md and docs/.
4. Checked error wrapping and the fail-closed paths in `dialControl` and `CheckHostAllowed`.

## Observations (no finding)

- `dialControl` (`netguard.go:151-163`) fails closed. An address it can't parse, such as a zone-scoped `fe80::1%eth0` where `net.ParseIP` returns nil, returns `ErrBlockedAddr`.
- `ErrBlockedAddr` from the Control hook still matches `errors.Is` through Go's `*net.OpError` and `*url.Error` unwrapping. `TestHTTPClientGuardsDial` pins this (`netguard_test.go:152-158`).
- `CheckHostAllowed` returns DNS lookup errors unwrapped, which matches the spec's "Returns DNS lookup errors as-is" (`specs.md:55`). Callers only test for `ErrBlockedAddr`.
- `HTTPClient` sets `Proxy: nil` and no `CheckRedirect`, as `specs.md:51,65` says. `TestHTTPClientNoProxy` pins the proxy setting.
- `normalize` handles IPv4-mapped `::ffff:a.b.c.d`, as `specs.md:66` says. Both policy tests pin it.
- The two policies really are split: `TestIsPublic` asserts `IsAllowed(10.0.0.1) && !IsPublic(10.0.0.1)`, as `specs.md:63,84` says.
- The coverage gate is 91 in both `.testcoverage.yml` and the `CLAUDE.md` table. Every CIDR literal in `blockedV6Prefixes` and `reservedBlocks` parses.
- Wording nit, not reported: `specs.md:79` has the heading "Metadata hostname whitelisting", but `HostIsMetadata` backs a denylist.
- `specs.md` states no Go version, so there is no drift against `go.mod` (`go 1.26.0`).
- `docs/security.md:308-309` limits its claim to "the operator's `git`/`http` source fetchers", which matches the code. (see held item OBS-netguard-wider-claim, OD-019)

held candidates: 4 (see OD-019)

## Candidate findings

### C-netguard-01: held (OD-019)

## Questions (not findings)

- `specs.md:88` and `.testcoverage.yml` put the remaining coverage gap in the "error branches of dialControl and CheckHostAllowed". Is that still accurate? This review didn't measure coverage (Rule 8).
- One more robustness question about this guard is recorded off-git with the held candidates (OD-019).
