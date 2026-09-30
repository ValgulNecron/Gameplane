# Review H08: client IP derivation behind trusted proxies

- Worktree: `/tmp/claude-1000/-home-valgul-project-Gameplane/0cbfe030-6e64-40ca-9a18-d291dbf5b40a/scratchpad/wt-H08`
- Branch: `fix/018-harden-client-ip-trust`, commit `7acd9c7c` on `origin/master` `61f50265`
- Reviewer tier: opus (implementer: haiku)
- Verdict: **needs-fix** (one blocking issue, a bug that predates this branch but defeats the documented mitigation this branch depends on)

## 1. Diff vs brief

The committed diff matches brief sections 2a, 2b and 2c exactly: 7 files, both new files byte-for-byte, all 8 edits. Nothing under `specs/*/audit/held/` is committed and `SECURITY_AUDIT.md` is not touched. The commit is signed off. The `Co-Authored-By` line names Haiku 4.5, not the Opus line in the brief. That follows CLAUDE.md ("current-running-model"), so it is not an issue.

## 2. Compile checks (run by the reviewer)

- `gofmt -l ./cmd ./internal/auth`: no output
- `go build ./...` (api): OK
- `go vet ./...` (api, all packages, compiles the new test file): OK
- `helm template gp charts/gameplane --namespace gameplane-system`: OK

No test or lint suite was run.

## 3. Against F-091 and D17a

- Expected behaviour ("the TCP peer is checked against the trusted set before any XFF entry is used"): met. `derivedClientIP` parses and unmaps the peer and returns it before reading any header when the peer is untrusted.
- Check (a), untrusted RemoteAddr plus XFF gives the RemoteAddr host: case "untrusted peer is the client and its forwarded header is ignored" plus `TestClientIPFromTrustedProxies_UntrustedPeerKeepsOneLimiterBucket`.
- Check (b), trusted proxy plus private-range client gives that client: case "trusted peer forwarding a private-range client records that client".
- Check (c), two such clients get different buckets: `TestClientIPFromTrustedProxies_ClientsBehindOneProxyGetSeparateLimiterBuckets`.
- D17a (keep the broad default, with docs): the default is unchanged, and docs/security.md gains a "Clients on a private network" section that tells operators to narrow the list. **But the knob the docs point to does not work; see 5.1.**

### Would the tests fail before the fix?

I checked this against the chi v5.3.2 source (`middleware/client_ip.go`). The new tests call a new symbol, so they cannot compile on master. The behaviour they pin differs from chi's `ClientIPFromXFF`:
- (a): chi walks the header without looking at the peer and records `198.51.100.1`. Both limiter requests pass, so the untrusted-peer limiter test would fail.
- (b)/(c): with every hop trusted, chi sets no IP, so the limiter falls back to RemoteAddr `10.42.0.5`, which is shared. The second client gets 429 and the separate-buckets test would fail.

I traced all 18 table cases by hand against the implementation, including the empty-entry split, the IPv4-mapped peer and entry, the IPv6 ULA peer, the bare IPv6 recorded through chi's `ClientIPFromRemoteAddr` (SplitHostPort fails, bare-host fallback parses), the bare peer, the nil list and the unparseable peer. All pass. `newTokenBucket(0, 1)` starts each key at 1 token and never refills, so the limiter assertions are deterministic. No helper names clash with other `package auth` test files.

### Code correctness

- The RemoteAddr swap is sound. `withClient` is a shallow copy, chi's `ClientIPFromRemoteAddr` makes its own `WithContext` copy, and the inner closure restores `RemoteAddr` on that copy only. The ResponseWriter passes through unchanged, so hijack (WebSocket) and flush (SSE) still work.
- Consumers are unchanged: `auth/ratelimit.go:85` and `audit/audit.go:633,707` read `middleware.GetClientIP` and fall back to RemoteAddr when it is empty. The only time nothing is recorded is when the peer does not parse, which matches the old fallback.
- Legitimate callers:
  - Public clients through ingress, web nginx (`proxy_add_x_forwarded_for`, `web/nginx.conf.template:127`) and then the API are still recorded by their own address.
  - In-cluster callers with no header (agents, port-forward in `TestAPI_LoginRateLimit`) key on the peer as before.
  - LAN clients behind the default now get separate buckets instead of one shared pod IP.
- The leftmost-when-all-trusted choice (held note in the brief) is accepted. Picking the rightmost entry would reproduce the collapse the finding reports. The remaining trade-off only applies to private-range clients under the broad default, and D17a accepted it with docs. The per-username limiter is still a hard bound.

## 4. E2E, buckets, login budget

No E2E was added, as the plan says (kind cannot simulate distinct private-range peers). `buckets.sh` is untouched and the login budget is unchanged. `TestAPI_LoginRateLimit` stays the guard, because its loopback port-forward peer is trusted and sends no header, so it keys the same as before.

## 5. Issues

### 5.1 BLOCKING: `--trusted-proxies` flag value is never applied (`api/cmd/main.go:534`)

`bindFlags` registers `fs.StringVar(&trustedProxiesStr, "trusted-proxies", ...)` and then, still inside `bindFlags` (so before `fs.Parse` at `main.go:70`), runs `c.trustedProxies = strings.Split(trustedProxiesStr, ",")`. The split captures the env/default value. The flag value that `Parse` later writes into the local `trustedProxiesStr` is thrown away.

The chart passes the setting only as a flag (`charts/gameplane/templates/api.yaml:326`, `--trusted-proxies={{ .Values.api.trustedProxies }}`). So `api.trustedProxies` has no effect on a Helm install, and the API always runs with the built-in broad default.

This bug predates the branch, but the branch's own docs and comments depend on the flag working:
- the docs/security.md "Clients on a private network" section says to narrow the list
- the docs/security.md "When the API is directly exposed" section says to set it to `""`
- the values.yaml comment gives the same advice

D17a ("keep the default, with docs") is only safe if that advice works. As shipped, an operator who follows the docs gets no change.

Exact fix (same branch, new commit):

1. `api/cmd/main.go`, the config struct (around line 459): add a field `trustedProxiesRaw string` next to `trustedProxies []string`.
2. `api/cmd/main.go` bindFlags (lines 526-534): bind the flag to the field and drop the early split:
   ```go
   	fs.StringVar(&c.trustedProxiesRaw, "trusted-proxies",
   		envOr("GAMEPLANE_TRUSTED_PROXIES",
   			"127.0.0.0/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,169.254.0.0/16,::1/128,fc00::/7,fe80::/10"),
   		"comma-separated list of CIDR blocks for trusted reverse proxies; X-Forwarded-For is read only when the TCP peer is in one of these ranges")
   ```
   Delete the `trustedProxiesStr` local, the trailing "Parse and validate ... after flags are parsed" comment and `c.trustedProxies = strings.Split(trustedProxiesStr, ",")`.
3. `api/cmd/main.go` main, directly after `// Validate and trim trusted proxies CIDR list.` (line 112), before the loop:
   ```go
   	cfg.trustedProxies = strings.Split(cfg.trustedProxiesRaw, ",")
   ```
   An empty value then splits to `[""]`, which the loop skips, so the list is empty and the peer is always the client (docs rule 5).
4. Add a unit test in `api/cmd` (for example `main_trustedproxies_test.go`, `package main`) named as a control statement, such as `TestTrustedProxiesFlagOverridesDefault`. It builds a `flag.NewFlagSet`, calls `cfg.bindFlags(fs)`, runs `fs.Parse([]string{"--trusted-proxies=10.42.0.0/16"})`, applies the split, and asserts `cfg.trustedProxies == []string{"10.42.0.0/16"}`. Add a second case with `--trusted-proxies=` that asserts a single empty entry. Use `t.Setenv("GAMEPLANE_TRUSTED_PROXIES", ...)` only if needed. It must not call `main()`. Before checking the exact wiring, see whether other `api/cmd` tests already build a config through `bindFlags`.
5. Compile checks again: `gofmt -l ./cmd`, `go build ./...`, `go vet ./...` (api), `helm template`.

Commit wording (neutral): `fix(api): apply the --trusted-proxies flag value`, with the body "The trusted proxy list was read before flags were parsed, so the flag, and with it api.trustedProxies in the chart, had no effect." No CHANGELOG change is needed, because the existing hardening line covers it. Also add the fix to the PR body's Changes list.

### 5.2 Non-blocking notes (no change required for pass)

- Stale comments that still name `ClientIPFromXFF` are at `api/internal/auth/ratelimit.go:79,82` and `api/internal/audit/audit.go:630,690,704`. The brief left them alone on purpose to avoid overlapping the public work in `ratelimit.go`. That is fine as a follow-up.
- A release note is still owed at merge time (fix plan H08 "Depends on"). It should cover two changes: installs whose proxy sits outside `api.trustedProxies` now record the proxy address, and an empty list now uses the peer instead of the rightmost header entry. After 5.1, add a third point: `api.trustedProxies` now takes effect, so installs that set a custom value in their Helm values will see it applied for the first time on upgrade. This is the maintainer's hand-off, not the branch's.

## 6. Wording check (all git-bound lines)

I scanned the added diff lines and the commit message for `F-NNN`, `Hxx`, spoof, attack, exploit, bypass, forge, evade, rotate, abuse and vulnerab: no hits. Test names state the control they check. docs/security.md drops the old "spoofing" wording. Its private-network caveat ("the recorded IP depends on the forwarded chain those addresses present") is the operator trade-off D17a asked for, with no repro, and is acceptable. The CHANGELOG line reads `- **api:** hardened client IP derivation behind trusted proxies.`, under a new `### Security hardening` heading at the end of `## [Unreleased]`, just before `## [0.3.0-rc.1]`. Its format is correct. The brief's merge-order note covers the heading clash with #427, #430 and #431.

No wording leaks.
