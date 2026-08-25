# Baseline Finding Inventory (T009)

**Measured**: 2026-08-17 · **Run**: [32063472380](https://github.com/ValgulNecron/Gameplane/actions/runs/32063472380) · **Commit**: `0cf6814` · **PR**: #237

This is the output of the deliberately-red CI run described in [plan.md](plan.md) Phase 2.
It is the only way to obtain these numbers: until `api`, `agent`, and `test/e2e` entered
the lint matrix, CI did not analyse them at all, and nothing runs locally on this project
(Constitution VI). Numbers here are read from CI job logs, not estimated.

**Total: 321 findings** across the three modules.

## Completeness

| Module | Findings | Verdict |
|---|---|---|
| `api` | 98 | COMPLETE — verified no typecheck/compile abort in the log |
| `agent` | 41 | COMPLETE |
| `test/e2e` | 182 | COMPLETE |

The first measurement attempt (run `32062092752`) produced a **truncated** api list of 1
finding, because `lint (api)` aborted at a compile error inherited from the salvaged
commit. That number is void; the 98 above comes from the re-run after the repair.

## By module and linter

### api — 98

| Linter | Count |
|---|---|
| noctx | 50 |
| revive | 16 |
| gosec | 10 |
| errcheck | 9 |
| errorlint | 6 |
| contextcheck | 5 |
| gofmt | 1 |

| Package | Count |
|---|---|
| `api/internal/auth` | 34 |
| `api/internal/db` | 24 |
| `api/internal/handlers` | 18 |
| `api/internal/audit` | 8 |
| `api/internal/ws` | 3 |
| `api/internal/rbac` | 3 |
| `api/internal/registry` | 1 |

Within `api/internal/handlers`, `shares.go` carries 12 of the 18; the rest are single
findings in `users.go`, `tunnelcreds.go`, `tunnelcreds_test.go`, `systemlogs.go`,
`cluster.go`, and `audit.go`.

Note this distribution differs sharply from the task partition in [tasks.md](tasks.md),
which was written before measurement and split `api/internal/handlers` into 15 tasks by
theme. The findings are concentrated in `auth` and `db` instead. The partition should be
re-weighted against this table rather than followed as written.

### agent — 41

| Linter | Count |
|---|---|
| contextcheck | 20 |
| gosec | 17 |
| revive | 3 |
| gofmt | 1 |

| Package | Count |
|---|---|
| `agent/internal/rcon` | 20 |
| `agent/internal/usage` | 11 |
| `agent/internal/files` | 3 |
| `agent/internal/heartbeat` | 3 |
| `agent/internal/mods` | 2 |
| `agent/internal/auth` | 1 |
| `agent/internal/logs` | 1 |

### test/e2e — 182

| Linter | Count |
|---|---|
| revive | 49 |
| noctx | 40 |
| errcheck | 21 |
| bodyclose | 20 |
| gofmt | 14 |
| errorlint | 10 |
| staticcheck | 8 |
| contextcheck | 7 |
| gosec | 7 |
| nilerr | 3 |
| unused | 3 |

| Area | Count |
|---|---|
| `test/e2e` (root package) | 57 |
| `test/e2e/internal/protocol/*` | 39 |
| `test/e2e/internal/minecraft-java/*` | 15 |
| `test/e2e/internal/terraria/*` | 14 |
| `test/e2e/internal/factorio/*` | 7 |
| remaining per-game directories | 1–5 each |

## Findings needing care

**Frozen-surface collisions.** No finding lands in `api/internal/db/migrations/`, and none
touches `test/e2e/buckets.sh` or the game-protocol byte layouts. But two frozen areas do
carry findings, and both must be refactored around rather than renamed (FR-006):

- `api/internal/audit/audit.go` — 8 findings (5 contextcheck, 3 revive `exported`), on the
  `Auditor` and `Event` types and the `New` function. Audit field names and the chained-hash
  logic are consumed externally.
- `api/internal/auth/ratelimit.go` — 1 revive `exported` finding on `TokenBucket.Allow`.
  Thresholds here bound the e2e login budget.

**Rename-pressure linters.** 1 gosec G101 (`api/internal/registry/keys.go:24`) and 57 revive
`exported`/`var-naming` findings across the three modules push toward renaming identifiers.
In `test/e2e` this is the sharpest risk: `buckets.sh` maps tests to CI buckets **by exact
name**, so a revive-driven rename of an e2e test function silently breaks the
"e2e bucket coverage" job without breaking the lint job.

## Collateral discovered during measurement

The salvaged commits from PR #216 did **not** apply cleanly in the semantic sense, despite a
textually conflict-free cherry-pick. They required two repairs, both because PR #216's own
later fix commits were not carried across:

1. `api/internal/handlers/lifecycle_envtest_test.go` — a hoisted variable left a `resp :=`
   redeclaration that failed to compile, which also truncated the api lint measurement.
2. `agent/internal/rcon/websocket.go` — `resp.Body.Close()` on a successful upgrade, where
   `resp.Body` is nil, panicking `TestWebSocketHappyPath`.

A third fix (`ECONNRESET` as a WebRcon auth signal) was needed for
`TestWebSocketAuthFailureCooldown`; unlike the first two that one is new handling, not
regression repair — the function involved is byte-identical to its pre-wave-2 form.

The lesson for the salvage decision in [research.md](research.md) Decision 1: file-level
overlap with master is necessary but not sufficient evidence that a commit is reusable in
isolation. A commit that depends on later commits in its own chain will apply cleanly and
still break.
