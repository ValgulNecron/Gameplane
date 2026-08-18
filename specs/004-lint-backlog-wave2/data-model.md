# Data Model: Lint Backlog Wave 2

**Related**: `plan.md` Phase 1 output. Defines the status vocabulary and gating model for the progressive golangci-lint enforcement across all go.work members.

**Canonical Vocabulary**: This file defines the gated/ungated module classifications and the three authorized exclusions in `.golangci.yml`, reconciled against `research.md` and `plan.md` for accuracy on measurement and finding counts. The state transitions are one-way: ungated → gated (never the reverse without explicit specification amendment).

## Entity Relationship Summary

```
LintGate (CI job: github.com/.../workflows/ci.yaml lint: job)
  ├─ has 13 GatedModule entries (one per go.work member)
  │  ├─ declares gated bool (CI verifies zero findings)
  │  ├─ declares build tags required (if any)
  │  ├─ references optional coverage threshold
  │  └─ records current and target gated status
  │
  ├─ has 3 AuthorizedExclusion rules (.golangci.yml)
  │  ├─ path pattern (regex)
  │  ├─ linters affected
  │  ├─ optional text matcher (nil for all findings by linter)
  │  └─ justification comment
  │
  └─ has 5 FrozenSurface items (interfaces that must not change)
     ├─ audit record schema (field names in audit events table)
     ├─ database migration order (append-only, 001_init.sql..006_share_links.sql)
     ├─ e2e test names (test/e2e/buckets.sh key by exact function name)
     ├─ game protocol byte layouts (gameproto/minecraft.go, terraria.go)
     └─ rate-limit thresholds and Prometheus metric names

Finding (output from golangci-lint or go vet)
  └─ record exists only in CI logs; no persisted form
  ├─ linter name (e.g., "errcheck", "gosec", "govet")
  ├─ file path (relative to module working directory)
  ├─ line number
  ├─ message text
  └─ module name

SuppressionDirective (in-source annotation)
  └─ Invariant: MUST be zero across the entire codebase
  ├─ forms forbidden: //nolint, //nolint:linter, //#nosec, //lint:ignore, ESLint equivalents
  └─ never appears in committed code

AuthorizedExclusion (scoped rule in .golangci.yml)
  ├─ path pattern (regex, relative to module root)
  ├─ linters array (affected linters)
  ├─ text matcher (optional, filters findings within the linter)
  └─ justification (quoted from config comment)

FrozenSurface (interface that must not change without major version bump)
  ├─ name (descriptor)
  ├─ path (location in tree)
  ├─ why frozen (reason for stability requirement)
  └─ what breaks (consequences of change)
```

---

## GatedModule

A go.work member subject to the lint gate. The primary artifact of this gate.

| Field | Type | Required | Constraints | Semantics |
|-------|------|----------|-------------|-----------|
| `module` | string | Yes | Must match a directory with go.mod under one of: `./`, `./test/`, or a direct subdirectory of root | Module identifier, e.g., `operator`, `api`, `test/e2e` |
| `goModPath` | string | Yes | Path to the go.mod file | Full path for reference in CI: e.g., `./operator`, `./api`, `./test/e2e` |
| `gated` | bool | Yes | Current gating status in the lint CI job | Is this module currently verified to have zero findings? |
| `targetGated` | bool | Yes | Desired status (FR-003: one-way, never reverts) | Should this module be gated by wave 2 completion? |
| `buildTags` | string | If gated and tags needed | Comma-separated, e.g., `envtest` for operator, `e2e` for test/e2e | Tags to pass to golangci-lint via `--build-tags=...` flag. Nil/empty if no tags required. |
| `packageCount` | int | Yes | Informational | Count of Go packages in this module (`go list ./... | wc -l`) |
| `hasCoverageGate` | bool | Yes | Informational | Does this module have a `.testcoverage.yml` threshold? |
| `coverageThreshold` | int | If hasCoverageGate | Percentage, 0–100 | Total coverage threshold, or nil if no gate |

### Full GatedModule Inventory

This table is the **canonical state** of all 13 go.work members and their lint gate status.

| Module | Path | Currently Gated | Target Gated | Build Tags | Packages | Coverage Gate | Notes |
|--------|------|-----------------|--------------|------------|----------|----------------|-------|
| `netguard` | `./netguard` | ✓ Yes | ✓ Yes | None | ~3 | 91% | Shared SSRF dial-guard; gated in wave 1 |
| `gameaction` | `./gameaction` | ✓ Yes | ✓ Yes | None | ~3 | 91% | Shared console-injection guard; gated in wave 1 |
| `gameproto` | `./gameproto` | ✓ Yes | ✓ Yes | None | ~2 | 90% | Shared wire-protocol parser; gated in wave 1 |
| `operator` | `./operator` | ✓ Yes | ✓ Yes | `envtest` | ~15 | 72% | Controller-runtime operator; gated wave 1; envtest build tag required |
| `api` | `./api` | ✗ No | ✓ Yes | `envtest` | 13 | 80% | REST + WebSocket gateway; entering the gate in wave 2; backlog count measured by CI, not asserted |
| `agent` | `./agent` | ✗ No | ✓ Yes | None | 17 | 90% | In-pod sidecar; entering the gate in wave 2; backlog count measured by CI, not asserted |
| `audit-syslog-bridge` | `./audit-syslog-bridge` | ✓ Yes | ✓ Yes | None | ~3 | 70% | HTTP-JSON → syslog relay; gated wave 1 |
| `telemetry-receiver` | `./telemetry-receiver` | ✓ Yes | ✓ Yes | None | ~3 | 70% | Anonymous-usage-telemetry collector; gated wave 1 |
| `sentinel` | `./sentinel` | ✓ Yes | ✓ Yes | None | ~5 | 70% | Wake-on-connect component; gated wave 1 |
| `mcp-server` | `./mcp-server` | ✓ Yes | ✓ Yes | None | ~5 | 70% | Read-only MCP server; gated wave 1 |
| `svcutil` | `./svcutil` | ✓ Yes | ✓ Yes | None | ~2 | 90% | Shared env/shutdown helpers; gated wave 1 |
| `tunnel` | `./tunnel` | ✓ Yes | ✓ Yes | None | ~4 | 70% | Network tunnel component; gated wave 1 |
| `test/e2e` | `./test/e2e` | ✗ No | ✓ Yes | `e2e` | 23 | None | E2E test suite; entering the gate in wave 2; backlog count measured by CI, not asserted |

**Key Observations**:
- **Currently gated**: 10 modules (all except api, agent, test/e2e)
- **Wave 2 targets**: api, agent, test/e2e
- **Build tag requirements**: operator (envtest), api (envtest), test/e2e (e2e) — must be passed via `args:` in CI
- **Coverage gates**: All 13 modules except `test/e2e` carry a coverage threshold; `test/e2e` is the only member with no `.testcoverage.yml`.

---

## Finding

A single linter or go vet diagnostic.

| Field | Type | Required | Semantics |
|-------|------|----------|-----------|
| `linter` | string | Yes | Name of the linter that produced this finding: `errcheck`, `gosec`, `govet`, `ineffassign`, `staticcheck`, `unused`, `misspell`, `revive`, `unparam`, `nilerr`, `noctx`, `errorlint`, `contextcheck`, or `gofmt` |
| `file` | string | Yes | File path, relative to the module working directory (e.g., `internal/handlers/users.go`) |
| `line` | int | Yes | Line number (1-indexed) where the finding occurs |
| `message` | string | Yes | The diagnostic text, e.g., `error not checked in range return` |
| `module` | string | Yes | The go.work member being linted (e.g., `api`, `agent`) |

**Invariants**:
- A Finding has no persisted form; it exists only in CI job logs and in the linter's output.
- Findings are ephemeral: they exist between the lint job start and the job's completion.
- CI must fail the entire job on the first non-zero finding (no `continue-on-error`).

---

## SuppressionDirective

An in-source annotation attempting to silence a linter.

| Form | Example | Forbidden | Reason |
|------|---------|-----------|--------|
| `//nolint` | `//nolint` | ✓ Always | Silences all linters; violates project policy |
| `//nolint:linter` | `//nolint:errcheck` | ✓ Always | Silences specific linter; violates project policy |
| `//#nosec` | `//#nosec` | ✓ Always | Silences gosec; violates project policy |
| `//lint:ignore` | `//lint:ignore SA1004` | ✓ Always | Silences by code; violates project policy |
| ESLint `eslint-disable` | `// eslint-disable-next-line` | ✓ Always | Web equivalent; forbidden in `web/src/**/*.tsx` |

**Invariant**: Zero suppressions exist in the codebase. The grep command `grep -r '//nolint\|//#nosec\|//lint:ignore' --include='*.go'` MUST return no matches across any go.work member.

**Note**: The distinction is critical:
- A **SuppressionDirective** (forbidden) is inline code that silences a linter.
- An **AuthorizedExclusion** (rare, scoped) is a rule in `.golangci.yml` that excludes a path/linter combination from the run entirely.

---

## AuthorizedExclusion

A scoped rule in `.golangci.yml` that excludes findings under specific conditions. Rare, maintainer-approved, and always justified.

| # | Path Pattern | Linters | Text Matcher | Justification | Why Authorized |
|---|--------------|---------|--------------|---|---|
| 1 | `_test\.go` | `[errcheck, gosec, unparam]` | None | "Tests often ignore errors deliberately in setup. Keep loud elsewhere." | Test-specific exception; setup patterns legitimately ignore errors (e.g., test fixture teardown) |
| 2 | `(^\\|/)internal/controller/` | `[revive]` | `"exported:"` | "Reconciler builder helpers have a lot of repeated patterns; revive can get noisy on them without catching real bugs." | Operator's reconciler boilerplate generates predictable exports; rule noise outweighs signal |
| 3 | `(gameproto/)?minecraft\.go$` | `[gosec]` | `"G115"` | "Minecraft VarInt encoding/decoding requires lossless reinterpretation between uint32 and int32 (two's complement). All 32 bits are preserved; length and range overflow are still checked explicitly in the code. G115 remains active everywhere else." | Intentional bit-level reinterpretation; gosec's G115 is a false positive; justification documents the control flow |

**Constraints**:
- Each exclusion has a narrow scope: specific path pattern + specific linter + optional text filter.
- The justification must cite the underlying reason (false positive, test fixture pattern, performance necessity) and reference the code that proves the control is correct.
- Text matchers (column 4) narrow the exclusion further (e.g., `text: "G115"` matches only the specific gosec code, not all gosec findings in that file).
- No exclusion disables a linter globally or broadens an enabled linter's rules.

---

## FrozenSurface

An interface whose structure must not change without a breaking version bump. Existing clients depend on these.

| Name | Path | Type | Why Frozen | What Breaks |
|------|------|------|-----------|-------------|
| **Audit Event Fields** | `api/internal/audit/audit.go` + `api/internal/db/migrations/005_audit_chain.sql` | Schema | Audit events are exported to external sinks (webhook, S3, syslog) and indexed by administrators. Field names are part of the external contract. | A field rename breaks: (1) external audit consumers parsing `audit_events` table rows; (2) SIEM/logging-system field mappings; (3) webhook JSON payloads; (4) audit trail continuity across version upgrades. Field reorder breaks column position assumptions in generated reports. |
| **Database Migrations** | `api/internal/db/migrations/` (001_init.sql, 002_config.sql, ..., 006_share_links.sql) | Versioning | Migrations are append-only and run at startup. A production database has already run 001–006. Deleting or reordering breaks old deployments. | Removing a migration breaks existing databases (schema diverges, newer installs miss the table/column). Reordering breaks the sequence (e.g., 005 assumes tables from 003). |
| **E2E Test Names** | `test/e2e/*_e2e_test.go` + `test/e2e/buckets.sh` | Registry | Test names are the stable identity for CI bucketing and coverage tracking (see `docs/game-coverage.md`). Renaming a test without updating the bucket entry silently drops it from CI. | Test renamed from `TestGameServer_MinecraftJavaBot_Joined` → `TestGameServer_MinecraftBot_JoinedLive` is no longer found in `bucket_bot_fast()` regex; CI silently skips it. Coverage verifier fails. |
| **Game Protocol Byte Layouts** | `gameproto/minecraft.go`, `gameproto/terraria.go`, `test/e2e/internal/*/proto/*` | Wire Format | These are wire-protocol parsers for real game servers. Packet structures and field positions are fixed by the game protocol spec (Minecraft, Terraria, etc.). Changing them breaks the handshake. | Flipping byte order in a VarInt reader, changing packet frame sizes, reordering field extraction — all cause join probes to fail or misinterpret packets. Sentinel and e2e probes stop working. |
| **Rate-Limit Thresholds** | `api/internal/auth/ratelimit.go` (lines 109–140): `LoginLimiter`, `LoginUserLimiter`, `OIDCCallbackLimiter`, `NotifyTestLimiter`, `MutationLimiter`, `ShareLimiter` | Configuration | Rate limits are operational contracts: ops teams configure deployments based on these thresholds, tests assert them. Changing them breaks assumptions about load capacity and attack resilience. | Increasing `LoginUserLimiter` burst from 6 to 20 breaks the per-user login budget assumption (documented in `test/e2e/buckets.sh` lines 25–30, which budgets 7 admin logins per API job). Test suite may exhaust the shared IP limiter and fail non-deterministically. |
| **Prometheus Metric Names and Labels** | `api/internal/audit/audit.go` (`gameplane_audit_webhook_events_total`), `api/internal/notify/notify.go` (`deliveries`), `api/internal/audit/s3.go` (`s3Events`) | Observability | Metric names are scraped by Prometheus and dashboards/alerts. Renaming a metric or changing its labels breaks existing dashboards and alert rules. | Renaming `gameplane_audit_webhook_events_total` → `gameplane_webhooks_sent_total` breaks Grafana dashboards and Prometheus queries that depend on the old name. Alerts referencing the old metric stop firing. |

**Invariants**:
- Changing a frozen surface requires (1) a major version bump, (2) a migration path for existing clients, and (3) release notes documenting the breaking change.
- Audit field additions (new columns) are non-breaking; deletions/renames are breaking.
- E2E test name changes must be accompanied by bucket registry updates.
- Protocol byte layouts are immutable; adding new protocol support requires a new file/module.

---

## Status State Transitions

A GatedModule transitions through states as lint cleanup progresses.

```
ungated ──────→ gated
  │
  └─ (terminal once gated; no reversion without spec amendment)
```

**Transition Rules**:
- **ungated → gated**: A module is declared `targetGated: true` and all findings are fixed. CI runs the module and reports zero findings for two consecutive runs (idempotence check).
- **gated → gated** (stable): CI continues to gate. If a new finding appears, the build fails immediately. A reviewer must decide: fix the code (preferred) or widen an exclusion (rare, with justification).
- **gated → ungated** (reversion): Not allowed without an explicit spec amendment and documented business reason.

**Evidence Required for → gated**:
- All findings in the module are fixed (not suppressed or excluded).
- CI runs the linters with the correct build tags (if any) and reports zero findings.
- The transition is recorded by changing `gated: false` → `gated: true` in this document and in the CI matrix.

---

## Build Tag Coverage

Modules requiring build tags must have those tags passed during linting, or tagged files are silently skipped.

| Module | Build Tag | Files Affected | CI Step (if: condition) |
|--------|-----------|-----------------|------------------------|
| `operator` | `envtest` | 7+ Go files with `//go:build envtest` | `if: matrix.module == 'operator'`, step `lint (operator - envtest build tags)` |
| `api` | `envtest` | 7 Go files with `//go:build envtest` | **Wave 2 target**: step `lint (api - envtest build tags)` |
| `test/e2e` | `e2e` | 51 Go files with `//go:build e2e` | **Wave 2 target**: step `lint (test/e2e - e2e build tags)` |

**CI Logic**: If `matrix.module == operator`, run golangci-lint with `--build-tags=envtest`. Similarly for api and test/e2e. Omitting the tag causes the linter to skip those files; findings in gated build-tagged files will be silently missed, and the job appears green when it should fail.

---

## Verification Recipe

To verify the integrity of this data model against the live tree:

```bash
# Verify zero suppressions exist
grep -r '//nolint\|//#nosec\|//lint:ignore' --include='*.go' || echo "✓ No suppressions"

# Verify .golangci.yml has exactly 3 exclusions
grep -c "^      - path:" .golangci.yml  # Should print 3

# Verify the three exclusion paths
grep "path:" .golangci.yml

# Verify all go.work members exist
go work edit -json | jq -r '.Use[].Path' | while read p; do
  [ -f "${p}/go.mod" ] || echo "MISSING: $p"
done

# Verify build-tagged files are present
find api -name "*.go" -exec grep -l "^//go:build envtest" {} \; | wc -l  # Should be 7
find test/e2e -name "*.go" -exec grep -l "^//go:build e2e" {} \; | wc -l  # Should be 51
```
