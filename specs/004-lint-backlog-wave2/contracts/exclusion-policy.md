# Exclusion Policy: Suppressions and Authorized Exclusions

This contract distinguishes between forbidden in-source suppressions and rare, scoped, maintainer-approved exclusions in `.golangci.yml`. It defines what is never acceptable, what is permitted, and the exact procedure for authorizing a new exclusion.

---

## Zero-Suppression Rule (Normative)

**Imperative**: The codebase MUST NOT contain any in-source suppressions or ignore directives.

### Forbidden Directive Forms (Go)

The following patterns are **always forbidden** in Go source code:

| Pattern | Example | Forbidden Because |
|---------|---------|-------------------|
| `//nolint` | `result := foo() //nolint` | Silences all linters; violates "fix, don't silence" policy. No scope; no justification. |
| `//nolint:linter` | `result := foo() //nolint:errcheck` | Silences a specific linter; violates policy. Enables drawer-filing a finding without fixing it. |
| `//nolint:<id>` | `result := foo() //nolint:gosec:G602` | Silences a specific finding code; violates policy. Enables targeted suppression. |
| `//#nosec` | `exec.Command(cmd) //#nosec` | Silences gosec (security linter); violates policy. Particularly dangerous because it hides security findings. |
| `//lint:ignore` | `var unused int //lint:ignore U1000` | Ignores specific linter code; violates policy. |

### Forbidden Directive Forms (Web / TypeScript / ESLint)

Equivalents in web code are equally forbidden:

| Pattern | Example | Forbidden Because |
|---------|---------|-------------------|
| `// eslint-disable-next-line` | `const x: any = 5; // eslint-disable-next-line @typescript-eslint/no-explicit-any` | Silences ESLint; violates policy. |
| `/* eslint-disable */` | `/* eslint-disable */` followed by code | Disables ESLint for a block; violates policy. |
| `// ts-ignore` | `const x: any = bar(); // @ts-ignore` | Silences TypeScript; violates policy. |

### Verification Recipe (Go)

To verify zero suppressions exist in the Go tree:

```bash
# Find any in-source suppressions
grep -r '//nolint\|//#nosec\|//lint:ignore' --include='*.go' | tee /tmp/suppressions.txt

# Exit 0 if none found, exit 1 if any found
[ ! -s /tmp/suppressions.txt ] && echo "✓ No suppressions" || (echo "✗ Found suppressions"; exit 1)

# Alternative: single grep command for CI
if grep -r '//nolint\|//#nosec\|//lint:ignore' --include='*.go'; then
  echo "FAIL: Suppressions found"
  exit 1
fi
echo "✓ Zero suppressions in codebase"
```

### Verification Recipe (Web)

To verify zero ESLint suppressions in web code:

```bash
# Find any ESLint disable directives
grep -r 'eslint-disable\|@ts-ignore' web/src --include='*.tsx' --include='*.ts' | tee /tmp/web_suppressions.txt

[ ! -s /tmp/web_suppressions.txt ] && echo "✓ No web suppressions" || (echo "✗ Found web suppressions"; exit 1)
```

---

## Distinction: Suppression vs. Exclusion

This distinction is **critical**. They sound similar but are fundamentally different.

| Aspect | **In-Source Suppression** | **Config-Level Authorized Exclusion** |
|--------|--------------------------|--------------------------------------|
| **Location** | Inline in source code (`//nolint`) | In configuration file (`.golangci.yml`) |
| **Scope** | Single line or statement | Path pattern + linter + optional text filter |
| **Governance** | None; can be added by any contributor | Rare, maintainer-approved, justified in code comment |
| **Justification** | Silent; no explanation in code | Documented inline in `.golangci.yml` with reasoning |
| **Example** | `return nil //nolint:nilerr` | `path: _test\.go` → `linters: [errcheck]` |
| **Policy** | **Forbidden. Always.** | **Permitted. Rarely. Scoped.** |
| **Use Case** | Never justified; violates "fix, don't silence" | Compensate for false positives in narrowly-scoped code (tests, platform-specific files) |

**Analogies**:
- A suppression is like adding `// TODO: fix this later` next to a bug.
- An exclusion is like a building code waiver: documented, approved, and applies only to a specific building.

---

## Current Authorization Inventory

These are the eight authorized exclusions currently in `.golangci.yml`. All other findings must be fixed, not excluded.

### Exclusion #1: Test Files

```yaml
      - path: _test\.go
        linters: [errcheck, gosec, unparam]
```

| Aspect | Value |
|--------|-------|
| **Path Pattern** | `_test\.go` (all files ending in `_test.go`) |
| **Linters Affected** | `errcheck`, `gosec`, `unparam` |
| **Text Matcher** | None (all findings by these linters in matched files are excluded) |
| **Justification** (from config line 37–38) | "Tests often ignore errors deliberately in setup. Keep loud elsewhere." |
| **Rationale** | Test files have different contracts than production code. Setup/teardown often deliberately ignores errors (e.g., closing a test server that is already closed). Security checks (gosec) in test code are less critical because tests are not deployed. Unused parameters are common when a test helper accepts arguments it doesn't need in all code paths. |
| **Why Authorized** | Narrow scope (only `_test.go` files). Linters affected (errcheck, gosec, unparam) are the most likely to produce false positives in tests. The exclusion does not disable a linter globally. |
| **Abuse Risk** | Low, because the pattern is file-specific. A contributor cannot hide production errors by putting them in a test file (they would be in a file ending in `_test.go`, which is easily audited). |

### Exclusion #2: Operator Reconciler Builders

```yaml
      - path: (^|/)internal/controller/
        linters: [revive]
        text: "exported:"
```

| Aspect | Value |
|--------|-------|
| **Path Pattern** | `(^|/)internal/controller/` (any path containing `internal/controller/`) |
| **Linters Affected** | `revive` |
| **Text Matcher** | `"exported:"` (only revive's "exported" rule; other revive findings are not excluded) |
| **Justification** (from config line 40–41) | "Reconciler builder helpers have a lot of repeated patterns; revive can get noisy on them without catching real bugs." |
| **Rationale** | Operator reconcilers use a builder pattern with repeated public helper methods (`WithField`, `WithLabel`, `WithAnnotation`, etc.). The revive linter's `exported` rule checks that exported symbols have comments. In a builder with 50 repetitive methods, this generates noise without catching bugs (the pattern is intentional; all methods are intentional exports). |
| **Why Authorized** | Narrow scope (only `internal/controller/`). Text matcher narrows further (only `exported:` findings). The exclusion does not disable revive globally; other revive checks (e.g., `blank-imports`, `unused-parameter`) still run in controller code. |
| **Abuse Risk** | Low, because the scope is semantic (controller builders are a known pattern). A contributor could not use this exclusion to hide, e.g., a missing godoc in other packages. |

### Exclusion #3: Minecraft Protocol Two's Complement Reinterpretation

```yaml
      - path: (gameproto/)?minecraft\.go$
        linters: [gosec]
        text: "G115"
```

| Aspect | Value |
|--------|-------|
| **Path Pattern** | `(gameproto/)?minecraft\.go$` (the file `gameproto/minecraft.go` or any path ending in `minecraft.go`) |
| **Linters Affected** | `gosec` |
| **Text Matcher** | `"G115"` (gosec's "G115: possible unintended type assertion" / uint32↔int32 reinterpretation) |
| **Justification** (from config line 47–49) | "Minecraft VarInt encoding/decoding requires lossless reinterpretation between uint32 and int32 (two's complement). All 32 bits are preserved; length and range overflow are still checked explicitly in the code. G115 remains active everywhere else." |
| **Rationale** | Minecraft's wire protocol uses variable-length integers (VarInts) that are signed in the spec but unsigned in Go's `encoding/binary`. The parser intentionally reinterprets bytes between `uint32` and `int32` to decode the spec-correct value. All bits are preserved (no truncation), and overflow checks are explicit. gosec's G115 rule flags this as a security risk, but in this context, it is safe and necessary. The exclusion is scoped to only this file, so gosec's G115 rule still runs everywhere else in the codebase. |
| **Why Authorized** | Narrow scope (only `minecraft.go`). Text matcher narrows to the specific gosec code (G115); other gosec findings in the file are still reported. The justification documents the control flow (bit preservation, overflow checks) that makes the reinterpretation safe. The exclusion is explicitly documented in code, not hidden. |
| **Abuse Risk** | Very low, because the scope is file-specific and semantically tied to a known protocol requirement. A contributor could not use this to hide unsafe type assertions elsewhere in the codebase. |

### Exclusion #4: Mod Extraction File Permissions

```yaml
      - path: (^|/)internal/mods/mods\.go$
        linters: [gosec]
        text: "G302"
```

| Aspect | Value |
|--------|-------|
| **Path Pattern** | `(^|/)internal/mods/mods\.go$` (any path ending in `internal/mods/mods.go`) |
| **Linters Affected** | `gosec` |
| **Text Matcher** | `"G302"` (gosec's "Expect file permissions to be 0600 or less") |
| **Justification** (from config) | "Mod extraction writes files to a volume shared with the game container, which runs as a different uid than the agent (65532). At 0o600 the game server cannot read files the agent just installed. FSGroup does not help: it changes group ownership, not the mode bits. Forcing a shared uid breaks modules that pin their own." |
| **Rationale** | The agent extracts mod archives into a PVC shared with the game server container. The two containers run as different UIDs, so a 0600 mode (owner read/write only) would leave the extracted files unreadable by the game server process. Group-writable/readable permissions are required for the handoff to work, and Kubernetes `fsGroup` only affects group *ownership*, not the mode bits gosec is checking. |
| **Why Authorized** | Narrow scope (only `internal/mods/mods.go`). Text matcher narrows further (only `G302` findings). G302 remains active everywhere else in the codebase. |
| **Abuse Risk** | Low, because the scope is file-specific and tied to a documented cross-container ownership constraint. |

### Exclusion #5: Agent-Proxy Upstream URL Construction

```yaml
      - path: (^|/)internal/ws/dialer\.go$
        linters: [gosec]
        text: "G704"
```

| Aspect | Value |
|--------|-------|
| **Path Pattern** | `(^|/)internal/ws/dialer\.go$` (any path ending in `internal/ws/dialer.go`) |
| **Linters Affected** | `gosec` |
| **Text Matcher** | `"G704"` (gosec's URL-construction-from-request-data check) |
| **Justification** (from config) | "The ws and http proxy handlers take a namespace and pod name from the request path and build an upstream URL from them. Both are validated as DNS-1123 labels first and the URL is assembled with url.URL, but gosec's taint analysis does not model a custom validator as a sanitiser." |
| **Rationale** | The agent-proxy handlers extract `namespace` and `pod` from the incoming request path, validate each as a DNS-1123 label before use, and then build the upstream URL with `url.URL` (not string concatenation). gosec's taint analysis flags any request-derived value reaching a URL construction, regardless of intervening validation, because it does not recognize the project's validator function as a sanitizing boundary. |
| **Why Authorized** | Narrow scope (only `internal/ws/dialer.go`). Text matcher narrows further (only `G704` findings). The validation step is real and enforced before the flagged construction. |
| **Abuse Risk** | Low, because the scope is file-specific and the validator is exercised by tests; a contributor could not use this exclusion to skip validating other request-derived values elsewhere. |

### Exclusion #6: CSRF Cookie Not HttpOnly

```yaml
      - path: (^|/)internal/auth/sessions\.go$
        linters: [gosec]
        text: "G124"
```

| Aspect | Value |
|--------|-------|
| **Path Pattern** | `(^|/)internal/auth/sessions\.go$` (any path ending in `internal/auth/sessions.go`) |
| **Linters Affected** | `gosec` |
| **Text Matcher** | `"G124"` (gosec's cookie-without-HttpOnly check) |
| **Justification** (from config) | "The CSRF cookie is deliberately not HttpOnly so the SPA can read it and echo it back as the X-Gameplane-CSRF header — the double-submit pattern documented in docs/security.md. Making it HttpOnly would break the protection gosec thinks it is asking for." |
| **Rationale** | `setCSRFCookie` intentionally sets a non-`HttpOnly` cookie because the double-submit CSRF pattern requires the SPA's JavaScript to read the cookie value and echo it back in the `X-Gameplane-CSRF` request header; the server then compares the two. Setting `HttpOnly` would prevent the SPA from reading the value at all, disabling the CSRF protection gosec's rule assumes it is enforcing. |
| **Why Authorized** | Narrow scope (only `internal/auth/sessions.go`). Text matcher narrows further (only `G124` findings). The design is documented in `docs/security.md`, so the exclusion is not a hidden decision. |
| **Abuse Risk** | Low, because the scope is a single cookie-setting function tied to a documented, deliberate protocol choice. |

### Exclusion #7: Test Helper Kubectl Invocation

```yaml
      - path: (^|/)env\.go$
        linters: [gosec]
        text: "G204"
```

| Aspect | Value |
|--------|-------|
| **Path Pattern** | `(^|/)env\.go$` (any path ending in `env.go`, scoped to `test/e2e`) |
| **Linters Affected** | `gosec` |
| **Text Matcher** | `"G204"` (gosec's subprocess-with-variable-arguments check) |
| **Justification** (from config) | "The Kubectl/KubectlWithStdin/port-forward helpers exist to run kubectl with caller-supplied arguments; the args come from the suite's own test code and the helper rejects shell metacharacters ahead of the -- separator, which gosec cannot see." |
| **Rationale** | The e2e suite's `Kubectl`, `KubectlWithStdin`, and port-forward helpers exec `kubectl` with arguments supplied by test code within the same repository (never external input), and the helper itself rejects shell metacharacters before the `--` separator. gosec flags any `exec.Command` call built from a variable argument slice, regardless of that in-process validation, because it cannot trace the rejection logic as a sanitizing boundary. |
| **Why Authorized** | Narrow scope (only `env.go` in `test/e2e`). Text matcher narrows further (only `G204` findings). Callers are exclusively the suite's own tests, never untrusted input, and the helper validates its own arguments. |
| **Abuse Risk** | Low, because the scope is a test-only helper invoked exclusively by trusted, in-repo test code, and it is not reachable from production binaries. |

### Exclusion #8: Satisfactory HTTPS Probe with Self-Signed Cert

```yaml
      - path: (^|/)internal/satisfactory/app\.go$
        linters: [gosec]
        text: "G402"
```

| Aspect | Value |
|--------|-------|
| **Path Pattern** | `(^|/)internal/satisfactory/app\.go$` (any path ending in `internal/satisfactory/app.go`, scoped to `test/e2e`) |
| **Linters Affected** | `gosec` |
| **Text Matcher** | `"G402"` (gosec's TLS `InsecureSkipVerify`/weak-config check) |
| **Justification** (from config) | "The probe dials the game server's HTTPS API over a pod-local address; the server generates a self-signed cert at first boot so there is nothing to pin, and the connection never leaves the pod network." |
| **Rationale** | The Satisfactory e2e probe connects to the game server's HTTPS management API using a pod-local address inside the test cluster. The server generates a self-signed certificate on first boot with no stable fingerprint to pin ahead of time, and the connection is confined to the pod network (never traverses an untrusted path), so there is no practical MITM surface for the test to defend against. |
| **Why Authorized** | Narrow scope (only `internal/satisfactory/app.go` in `test/e2e`). Text matcher narrows further (only `G402` findings). The probe never runs against production traffic and never leaves the pod network. |
| **Abuse Risk** | Low, because the scope is a single e2e probe file exercising a test-only, pod-local connection, not any production TLS client. |

---

## Authorization Procedure for New Exclusions

If a finding is raised and deemed a false positive, the **only** acceptable path is a new authorized exclusion. This procedure ensures exclusions are rare and justified.

### Prerequisites (What Must Be True)

Before proposing a new exclusion:

1. **Genuine False Positive**: The finding is incorrect, not a real bug. Example: a security linter flags `unsafe.Pointer` arithmetic, but the code uses it correctly under a specific constraint documented in an adjacent comment.

2. **Narrowest Possible Scope**: The exclusion applies to:
   - A single file or narrow path pattern (e.g., `internal/legacy/`), not a broad category (e.g., `.*\.go`).
   - A single linter or linter rule (e.g., `gosec` with `text: "G115"`), not multiple linters.
   - An optional text matcher to filter within the linter (e.g., "exported:" for revive).

3. **Documented in Code**: The `.golangci.yml` config includes a comment immediately above or inline with the exclusion rule, explaining:
   - Why the finding is a false positive (cite the underlying control flow or constraint).
   - What ensures the code is safe (e.g., "overflow checks are explicit", "pattern is intentional").
   - Why the scope is minimal (e.g., "limited to Minecraft handshake parsing").

4. **Maintainer Sign-Off**: The exclusion is reviewed and approved by a project maintainer (commit is signed, PR is approved).

### Unacceptable Responses (What Is Never OK)

| Response | Why It's Not OK | Example |
|----------|-----------------|---------|
| Broaden an existing exclusion rule | Widens the scope of an already-rare exclusion. | Changing `path: _test\.go` to `path: .*\.go` so non-test files can also ignore errors. |
| Disable a linter globally | Removes all checks; hides bugs everywhere. | Removing `- errcheck` from the `enable:` list in `.golangci.yml`. |
| Delete a linter from the enabled list | Same as disabling globally; breaks the gate for all modules. | Commenting out `- gosec` so security checks are skipped entirely. |
| Add multiple linters to one exclusion | Couples unrelated rules; makes auditing harder. | An exclusion that excludes `[errcheck, gosec, revive]` from a single path (unclear why all three need exemption). |
| Justify by "it's a common pattern" | Patterns are not justifications; correctness is. Example: "many callers ignore this error" is not a reason to exclude an errcheck finding. | Proposing to exclude all errcheck findings in `api/handlers/` because "setup functions often ignore errors". The fix is to inspect each error and decide locally. |

### Approval Workflow

1. **Propose** (PR/issue): Describe the finding, why it's a false positive, and propose the narrowest exclusion path.
2. **Code Review**: Reviewer inspects the code, verifies the false positive claim, and checks that the scope is minimal.
3. **Justification Check**: Ensure the `.golangci.yml` comment is clear and cites the control flow.
4. **Approval**: Maintainer approves and merges with sign-off.

### Example: Hypothetical New Exclusion

Suppose a gosec finding "G402: TLS MinVersion not hardcoded" appears in `api/internal/tls/builder.go`, and the code is:

```go
// The MinVersion is loaded from KUBECONFIG at startup and validated.
// It cannot be hardcoded because it depends on cluster policy.
cfg.TLSConfig.MinVersion = loadMinVersionFromEnv()
```

The developer proposes:

```yaml
      - path: api/internal/tls/builder\.go
        linters: [gosec]
        text: "G402"
        # G402 (TLS MinVersion) assumes hardcoded values. This file loads MinVersion
        # from KUBECONFIG at startup and validates it via validateTLSVersion().
        # Hardcoding would break multi-cluster deployments.
```

This exclusion is acceptable because:
- **Narrow scope**: Single file (`api/internal/tls/builder.go`), single linter (`gosec`), single rule (`G402`).
- **Justified**: Comment explains why hardcoding is wrong and cites the runtime validation.
- **False positive confirmed**: The code is safe; G402's assumption (hardcode only) doesn't apply.

---

## Detection and Audit

### Detection in CI

The lint job should include a verification step (optional but recommended for strictness):

```bash
# Fail if any suppressions exist
if grep -r '//nolint\|//#nosec\|//lint:ignore' --include='*.go'; then
  echo "FAIL: Suppressions found in source code (use .golangci.yml exclusions only)"
  exit 1
fi
echo "✓ Zero suppressions in Go code"
```

### Audit of Exclusions

To audit the current exclusions:

```bash
# Count exclusions in .golangci.yml
grep -c "^      - path:" .golangci.yml  # Should be 8 (current state)

# List exclusions
echo "=== Current Authorized Exclusions ==="
grep -B 1 "^      - path:" .golangci.yml | grep -A 3 "^      - path:"

# Verify each exclusion's justification comment
echo "=== Justification Comments ==="
grep -B 5 "^      - path:" .golangci.yml | grep "^      #"
```

### Manual Review Checklist

When reviewing a PR that proposes a new exclusion:

- [ ] Is the finding a genuine false positive? (Not just annoying or inconvenient)
- [ ] Is the scope minimal? (Single file or narrowly-defined path pattern)
- [ ] Is the linter scope minimal? (Single linter or single linter + text filter)
- [ ] Is the justification clear and cites code/control-flow evidence?
- [ ] Does the rule avoid broadening an existing exclusion?
- [ ] Does the rule avoid disabling a linter globally?
- [ ] Is the commit signed (git -s)?

---

## Special Cases

### Building Without Certain Build Tags

**Question**: If a module requires build tags (e.g., `//go:build envtest`), and lint runs without those tags, are the tagged files exempt from the zero-suppression rule?

**Answer**: No. If tagged files are not linted, they cannot contain suppressions because they're not analyzed. However, this is a configuration bug (Rule R-002 in `lint-gate.md`), not a license to add suppressions. The fix is to ensure the lint job passes the correct build tags.

### Vendor and Third-Party Code

**Question**: If vendor/ is included in the module tree, should vendored third-party code follow the zero-suppression rule?

**Answer**: golangci-lint skips `vendor/` by default (see `.golangci.yml` run config), so suppressions in vendored code are never analyzed. This is not a concern in practice. If a vendor package is pulled in with suppressions, they're ignored by the gate.

### Generated Code

**Question**: If code is generated (e.g., by protoc or go:generate), can the generated code contain suppressions?

**Answer**: Generated code should not contain suppressions if avoidable. If a generator always outputs suppressions, the generator itself is broken and should be fixed upstream. If the generated code has a linter false positive, the fix is an exclusion rule in `.golangci.yml`, not a suppression in the generated code.

---

## Enforcement Summary

| Entity | Must Have | Must NOT Have | Audit Method |
|--------|-----------|----------------|--------------|
| **Go source files** | Correct code (no bugs) | Suppressions (`//nolint`, `//#nosec`, etc.) | `grep -r '//nolint' --include='*.go'` → must be empty |
| **Web source files** | Correct code (no bugs) | ESLint suppressions (`eslint-disable`) | `grep -r 'eslint-disable' web/src` → must be empty |
| **`.golangci.yml`** | Justified exclusions (8 in current state) | Disabled linters, global silencing | `grep "^      - path:" .golangci.yml` ≤ N (where N is authorized count) |
| **New PRs proposing exclusions** | Clear justification, narrow scope, maintainer approval | Ad-hoc suppressions, broad exclusions, no comment | Code review + PR approval from maintainer |

---

## Rationale

The zero-suppression rule + rare authorized exclusions model ensures:

1. **Transparency**: Every exception to the lint rules is visible in a single config file, not scattered through source.
2. **Accountability**: Exclusions require justification and maintainer approval, preventing bit-filing of findings.
3. **Auditability**: A reviewer can quickly scan `.golangci.yml` and understand all exceptions; source code remains clean.
4. **Scalability**: As the codebase grows, a config-based model is easier to track than 1000s of inline suppressions.
5. **Compliance**: Aligns with CLAUDE.md rule 4 ("Fix, don't silence"), enforcing that linter findings are addressed at the root, not hidden.
