# Phase 1 Data Model: Hardened GitHub Actions

**Feature**: 008-hardened-github-actions | **Date**: 2026-08-29

There is no database and no runtime object graph in this feature. The "entities" are
declarative configuration structures evaluated by GitHub's workflow engine, plus the
artifacts they produce. Each is defined below with its fields, validation rules (all
enforced in CI unless noted), and — where it has
one — its lifecycle.

---

## E1. Workflow Security Policy

The cross-cutting policy every file under `.github/workflows/` and `.github/actions/` must
satisfy. Not a file itself; the security invariant all workflows must preserve.

| Field | Type | Rule |
|---|---|---|
| `workflow.permissions` | map | REQUIRED at top level. Floor is `contents: read`. Elevated scopes forbidden at top level except `packages: write` in the four publish workflows. |
| `job.permissions` | map | REQUIRED on every job, no exceptions. Must be a subset of what the job's steps actually use. |
| `job.timeout-minutes` | integer | REQUIRED. `1 ≤ n ≤ 30`, unless the job is on the exception list, which additionally requires an inline `#` comment stating why. |
| `step.uses` (external) | string | MUST match `^[\w.-]+/[\w.-]+(/[\w./-]+)?@[0-9a-f]{40}$`, followed by a `# vX.Y.Z` comment. |
| `step.uses` (local) | string | MUST start with `./.github/` — local composite actions are exempt from pinning (they are versioned by the repo checkout itself). |
| `workflow.concurrency` | map | REQUIRED when `on:` includes `push` or `pull_request`. Must set `group` and `cancel-in-progress`. Exception: `release.yaml` is tag-only (`push: tags:`) so concurrency is not required (a tag push is a one-shot publish; cancelling it in flight would abort a release mid-way). |
| `run:` script body | string | MUST NOT contain `${{ github.event.*.{title,body,ref,label,name} }}`, `${{ github.head_ref }}`, or any interpolation of a user-writable field. Such values pass through `env:`. |
| `on:` triggers | list | MUST NOT include `pull_request_target`. |
| secret references | string | `COSIGN_PRIVATE_KEY`, `GHCR_*`, registry credentials permitted only in `images.yaml`, `publish-edge.yaml`, `release.yaml`, `republish-modules.yaml`. |

**Timeout exception list** (the only values > 30 permitted, per ruling D-A):

| Workflow | Job | Max | Justification |
|---|---|---|---|
| `ci.yaml` | `e2e-go` (all matrix legs) | 60 | Boots kind cluster, installs chart, runs bucket suite. |
| `ci.yaml` | `e2e-multicluster` | 60 | Same, across two clusters. |
| `ci.yaml` | `e2e-upgrade` | 60 | Same, plus chart upgrade across versions. |
| `ci.yaml` | `e2e-web-live` | 60 | Same, plus live dashboard drive. |
| `ci.yaml` | `e2e-game-bot` | 60 | Pulls multi-GB game images, boots real servers, runs protocol joins. |
| `publish-edge.yaml` | `images` | 35 | Observed max 31m across 142 job samples (2026-08-30 measurement). |

Every other job is ≤ 30 and needs no exception: `images.yaml`'s `game-images` and
`common-base` are 10, `release.images` is 30, and the release chart/github-release/modules and republish-modules jobs are 15 [EXTENSION].
D-A raises the E2E set and publish-edge.images above FR-004's ≤ 30 default by the maintainer's explicit call;
the justifications are recorded in OPEN-DECISIONS.md D-A.

**Validation**: Expression-injection patterns in `run:` bodies are enforced by actionlint in the `workflow-lint` job. Action `uses:` pinning, permissions misuse, and `pull_request_target` exclusion are enforced by zizmor (also in the workflow-lint gate). The following rows are upheld by code review and are not automatically enforced: `workflow.permissions` and `job.permissions` (presence and scope requirements), `job.timeout-minutes` (presence and value; actionlint enforces only malformed keys), `workflow.concurrency` (presence and fields), `secret references` (confinement to approved workflows), `step.uses (local)` (exemption from pinning), and the `# vX.Y.Z` comment convention for external action refs.

---

## E2. Action Pin Registry

The mapping from each external action to its immutable commit. Authoritative copy lives in
[contracts/action-pins.md](./contracts/action-pins.md); this defines its shape.

| Field | Type | Rule |
|---|---|---|
| `owner/repo` | string | Unique key. 18 entries at baseline. |
| `sha` | string | Exactly 40 lowercase hex chars. Must be a real commit reachable from the named tag. |
| `tag` | string | `vX.Y.Z` form, recorded as the trailing comment. |
| `usage_count` | integer | How many `uses:` sites reference it — informational, used to size the diff. |
| `trust_tier` | enum | `first-party` (`actions/*`), `verified` (`docker/*`, `azure/*`, `sigstore/*`, `golangci/*`, `helm/*`, `oras-project/*`), `community` (`dorny/*`). |

**Invariants**:
- Every occurrence of a given `owner/repo` across all 9 files pins the **same** SHA. Version
  skew between two call sites of the same action is a defect.
- A pin's tag comment must match the tag the SHA was resolved from — a stale comment misleads
  Dependabot and reviewers alike.

**Lifecycle**: `resolved` → `committed` → `superseded by Dependabot PR` → `re-resolved`.
Dependabot's `github-actions` ecosystem owns the transition after landing; the pin table in
`contracts/` is a point-in-time snapshot, not a file to maintain by hand forever.

---

## E3. Job Permission & Timeout Matrix

One row per job across all 7 workflows — 26 at baseline.
Authoritative copy in [contracts/permissions-matrix.md](./contracts/permissions-matrix.md).

> ⚠️ **Superseded — AI review infrastructure moved**: The AI review infrastructure
> originally designed as `ai-review.yaml` + `ai-review-respond.yaml` has been retired and
> replaced by the CodeRabbit GitHub App integration (see OPEN-DECISIONS.md D-L and the
> superseded banner on `contracts/ai-review-contract.md`). The matrix entry below is
> archived; the contract document (linked above) carries the full rationale for the change.

| Field | Type | Rule |
|---|---|---|
| `workflow` | string | Filename under `.github/workflows/`. |
| `job_id` | string | YAML key under `jobs:`. Unique within a workflow. |
| `permissions` | map | Explicit. Minimum viable set for that job's steps. |
| `timeout_minutes` | integer | Explicit. Per E1's rule. |
| `justification` | string | REQUIRED when `permissions` exceeds `contents: read` or `timeout_minutes > 30`. Rendered as an inline YAML comment. (Note: `workflow-lint` is 10 minutes — not a justification; it is a linter gate and does not require inline commentary.) |

**State transition** — the only one in this feature, and it is imposed by GitHub, not by us:

```
same-repo PR / push  →  declared permissions granted as written
fork PR              →  ALL write scopes silently downgraded to read
```

Consequence, and the rule that follows from it: any step that writes (commit status, PR
comment) MUST detect failure and degrade to `$GITHUB_STEP_SUMMARY` rather than failing the
job. The existing `report` job already implements this; new write-capable steps must copy
the pattern.

---

## E4. Dependabot Ecosystem Matrix

The full contents of `.github/dependabot.yml`. Authoritative copy in
[contracts/dependabot-matrix.md](./contracts/dependabot-matrix.md).

| Field | Type | Rule |
|---|---|---|
| `package-ecosystem` | enum | `gomod` \| `npm` \| `docker` \| `github-actions`. |
| `directory` | string | MUST contain the ecosystem's manifest: `go.mod` for gomod, `package.json` for npm, `Dockerfile` for docker, `.github/workflows/` for github-actions. |
| `schedule` | map | `interval: weekly`, `day`, `time` (UTC). |
| `commit-message.prefix` | string | `chore(deps)`. |
| `commit-message.include` | string | `scope`. |
| `open-pull-requests-limit` | integer | gomod 5, npm 10, docker 5, github-actions 5. |
| `groups` | map | ≥ 1 group per entry. Each group has `patterns` and/or `update-types`. |

**Entry count invariant** — the load-bearing rule:

```
count(gomod entries)  == count(module lines in go.work)                  == 14
count(docker entries) == count(find . -name Dockerfile, excl. website/)  == 12
count(npm entries)    == 1
count(github-actions entries) == 1
                                                            total entries = 28
```

**COVERAGE GAP**: Adding a 15th Go module or a 13th Dockerfile without a matching Dependabot entry is no longer automatically caught in CI. This check was previously enforced but has been removed. Maintainers must manually verify Dependabot entries remain in sync with `go.work` and actual Dockerfiles to keep SC-003 true over time.

---

## E5. Cluster Diagnostics Bundle

What `dump-cluster-state` emits when an e2e job fails.

| Field | Type | Rule |
|---|---|---|
| `context` | string | Kind cluster context name. Input. |
| `namespaces` | string | Comma-separated. Input. |
| `pod_descriptions` | text | `kubectl describe pods`. **Redacted before emit.** |
| `controller_logs` | text | Operator + API pod logs. **Redacted.** |
| `container_logs` | text | Game server + agent + ephemeral capture container logs. **Redacted.** |
| `events` | text | `kubectl get events --sort-by=.lastTimestamp`. **Redacted.** |
| `helm_history` | text | Optional, gated by input. **Redacted.** |
| Secret objects | — | **NEVER collected.** No `kubectl get secret`, no `describe secret`, in any form. |

**Redaction contract** — applied to every text field above, at the point of emit, before it
reaches the job log:

| Pattern (case-insensitive) | Replacement |
|---|---|
| `(password\|passwd\|token\|secret\|api[-_]?key\|bearer\|authorization)\s*[:=]\s*.*` | key preserved, value → `***REDACTED***` (to end of line) |
| `eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+` | `***REDACTED-JWT***` |
| `-----BEGIN [A-Z ]*PRIVATE KEY-----` … `-----END [A-Z ]*PRIVATE KEY-----` | `***REDACTED-KEY***` |

**Lifecycle**: `job fails` → `if: failure()` fires → collect → **redact** → write to
stdout → captured as the job log.

Redaction sits between collect and write. `dump-cluster-state`'s `action.yml` has no
`upload-artifact` step and no `$GITHUB_STEP_SUMMARY` write (`grep -cE
'upload-artifact|GITHUB_STEP_SUMMARY' .github/actions/dump-cluster-state/action.yml` → `0`)
— the redacted stream's only sink today is the job log, which is readable by anyone who can
see the run, and on a public repo that is the entire internet. **Forward-looking rule**: if
either sink is ever added, it must be fed from the already-redacted stream, never from the
raw collected text — do not re-derive redaction per sink.

**Validation**: quickstart.md scenario 5 seeds a known sentinel value into a pod's
environment, fails the job deliberately, and asserts the sentinel does not appear in either
sink. **COVERAGE GAP**: CI does not automatically verify that all `dump-cluster-state` call sites apply the redaction filter before emit; this was previously checked but is no longer enforced. The redaction filter implementation itself is present and functional.

**Resolved issue #306 — quoted/JSON-embedded values.** Previously, the delimiter pattern
matched only when quotes did not sit between the key and the operator, so `token: abc` and
`token=abc` were caught but `"token":"abc"` was not. This left JSON-formatted credentials
in container logs unredacted on public repos — a real gap, not theoretical, since the action
collects container logs. The delimiter pattern was widened from
`\([[:space:]]*[=:][[:space:]]*\)` to `\(["']*[[:space:]]*[=:][[:space:]]*["']*\)` — POSIX BRE, as
`sed` is invoked here with no `-E`/`-r` — to tolerate optional quotes on either side of the
key/delimiter/value sequence. All six inlined `redact()` copies in `dump-cluster-state`'s
`action.yml` were updated together and remain byte-identical; the first `-e` argument moved from a
single-quoted to a double-quoted shell string so the literal `'` inside the bracket expression does
not need close-escape-reopen. Verified against the issue #306 fixtures: A1–A7 secret values are
redacted (including `{"password":"hunter2"}` and `{"level":"info","authorization":"Bearer abc"}`),
and B1–B7 non-secrets survive unchanged — critically `GAMEPLANE_CONTROL_CANARY`, the control that
distinguishes working redaction from a dump that collected nothing.

Redaction here remains **pattern-based and therefore best-effort** against credentials in an
unrecognised shape. Known residuals, documented rather than claimed fixed: XML/SGML-style
attributes (`<password="value">`), bare base64 blobs with no adjacent key name, and values split
across lines. Separately and pre-existing (not introduced by #306, confirmed by diffing against the
old pattern): because the rule consumes the rest of the line after the key, `kubectl describe`
`Environment:` descriptor lines such as
`DATABASE_PASSWORD: <set to the key 'password' in secret 'x'>  Optional: false` lose their
secret-*reference* metadata even though no secret value was present.

---

## E6. AI Review Exchange

The artifact handed from the untrusted `collect` job to the privileged `review` job. Full
contract in [contracts/ai-review-contract.md](./contracts/ai-review-contract.md).

| Field | Type | Rule |
|---|---|---|
| `pr_number` | integer | Validated `^[0-9]+$` in the privileged job before use. |
| `head_sha` | string | Validated `^[0-9a-f]{40}$` before use. |
| `base_ref` | string | Validated against `^[\w./-]+$`. |
| `title` | string | Sanitised: backticks and `${` stripped, truncated to 200 chars. |
| `body` | string | Sanitised, truncated to 4000 chars. |
| `diff` | text | Truncated to 200 KB. Passed as labelled untrusted data, never interpolated into the instruction section of the prompt. |
| `changed_files` | list | Paths only. Used to decide which spec artifacts to load. |

**Trust rule**: every field above originates in the untrusted job and is attacker-controlled.
The privileged job re-validates each one on receipt — it does not trust that `collect`
sanitised anything, because `collect` ran alongside the attacker's code. This mirrors the
`gameaction/` boundary discipline already in the codebase: both sides validate independently,
and neither skips because "the other side already checked."

**Sticky comment identity**: the `review` job upserts a single comment per PR, located by a
hidden marker `<!-- gameplane-ai-review -->` as the first line of the body. Update in place
on every run; never post a second comment on the same PR.

**Lifecycle**:

```
PR opened/synchronized
  → collect (untrusted code, no secrets, contents:read)  → artifact
  → workflow_run completed
  → review (no code checkout, has secrets, pull-requests:write)
      → re-validate → prompt → upsert sticky comment
      → on any failure: continue-on-error, write to step summary, never block the PR
```
