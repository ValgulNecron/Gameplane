# Contract: Job Permission & Timeout Matrix

**Feature**: 008-hardened-github-actions | **Date**: 2026-08-29

The target state for every job in the repository. Permissions misuse is enforced by the workflow-lint gate (zizmor); this matrix's specific per-job scopes are code-review-only. Timeout configuration is code-review-only.

> ⚠️ **Not all of this is ratified.** The `permissions` column follows FR-001/FR-002. The
> `timeout-minutes` column does not: FR-004 gives a 30-minute default and names game-bot as
> its one example, so every other number below is an agent proposal. Values marked
> **[UNRATIFIED]** are not automatically enforced. See OPEN-DECISIONS.md D-A.
>
> The `packages: write` top-level blocks below also contradict FR-001, which permits only
> `contents: read` or `{}` at the top level. Code review is expected to enforce this.
> See OPEN-DECISIONS.md D-E.

---

## The headline finding

`ci.yaml` declares `statuses: write` at the top level. All **26** of its jobs inherit it.
Exactly **one** step uses it — `web` → "report coverage (commit status)" at `ci.yaml:497`.

That inherited write token is live inside the six Kind e2e jobs, which build and execute
PR-authored test code against a real cluster. A malicious PR needs no exploit to reach it:
`$GITHUB_TOKEN` is in the environment of every step in the job.

**Change**: `ci.yaml` top level drops to `contents: read`. `statuses: write` moves to the
`web` job. `report` keeps its already-correct `statuses: read` + `pull-requests: write`.

---

## `ci.yaml` — top level

```yaml
permissions:
  contents: read
```

| Job | `permissions` | `timeout-minutes` | Notes |
|---|---|---|---|
| `changes` | `contents: read` | 5 | Already declares its own; verify it is not broader than needed. |
| `build-images` | `contents: read` | 15 | |
| `build-images-arm64` | `contents: read` | 20 | |
| `capture-sidecar-setcap-proof` | `contents: read` | 10 | Local buildx only, no registry push. |
| `lint` | `contents: read` | 15 | |
| `go` | `contents: read` | 20 | Publishes **no** status — do not grant `statuses: write`. |
| `web` | `contents: read`, `statuses: write` | 15 | **Only** job needing `statuses: write` (`ci.yaml:497`). |
| `web-e2e-mock` | `contents: read` | 20 | |
| `helm` | `contents: read` | 10 | |
| `chart-template` | `contents: read` | 10 | |
| `go-e2e-unit` | `contents: read` | 10 | |
| `e2e-buckets` | `contents: read` | 5 | |
| `e2e-go` | `contents: read` | matrix (`job_timeout`) | Legs are 25–35. The `operator` leg's 35 is a documented exception — 8-way parallel behind a 20m test timeout. |
| `e2e-multicluster` | `contents: read` | 30 | |
| `e2e-upgrade` | `contents: read` | 30 | |
| `e2e-web-live` | `contents: read` | 25 | |
| `e2e-game-bot` | `contents: read` | 50 | **Exception** — multi-GB game images + real protocol joins. Inline comment required. |
| `report` | `contents: read`, `statuses: read`, `pull-requests: write` | 5 | Already correct. Fork-degradation already handled. |
| `workflow-lint` *(new)* | `contents: read` | 5 | The workflow-lint gate runs actionlint and zizmor over `.github/workflows/`. Gated on the new `github` path-filter output. |

**`e2e-go` note**: `timeout-minutes: ${{ matrix.job_timeout }}` is an expression. Timeout
values are resolved from the matrix and each is checked against reasonable ceilings. The
`operator` bucket is 35, not ≤ 30 — legitimate due to 8-way parallel behind a 20m test
timeout, plus cluster-boot headroom. This is recorded as an exception rather than lowered.

**`report` wiring**: adding `workflow-lint` requires three edits in the reporter — the
`needs:` list, the `NEEDS_ORDER` array, and the `JOB_MATCHERS` map. Miss any one and the new
job is silently absent from the PR comment.

---

## `images.yaml`

```yaml
permissions:
  contents: read
  packages: write     # both jobs push to ghcr.io
```

| Job | `permissions` | `timeout-minutes` | Notes |
|---|---|---|---|
| `common-base` | `contents: read`, `packages: write` | 30 **[UNRATIFIED]** | Currently unbounded. |
| `game-images` | `contents: read`, `packages: write` | 60 **[UNRATIFIED]** | Currently unbounded. Nobody timed this; 60 is a guess at SteamCMD download cost. |

Also: this workflow triggers on `push`, `pull_request`, and `workflow_dispatch` but has no
`concurrency` block. A concurrency block should be added. Recommended:

```yaml
concurrency:
  group: images-${{ github.ref }}
  cancel-in-progress: ${{ github.event_name == 'pull_request' }}
```

Cancel superseded PR runs; never cancel a master publish mid-flight.

---

## `publish-edge.yaml`

```yaml
permissions:
  contents: read
  packages: write
```

| Job | `permissions` | `timeout-minutes` | Notes |
|---|---|---|---|
| `images` | `contents: read`, `packages: write`, `id-token: write` | 45 **[UNRATIFIED]** | Currently unbounded; 45 is a guess at multi-arch buildx + cosign across 8 images. Add `id-token: write` only if keyless signing is in use; this repo signs keyed/offline with `COSIGN_PRIVATE_KEY`, so **omit it** unless verified otherwise during implementation. |

Concurrency block already present and correct (`group: publish-edge`).

---

## `release.yaml`

```yaml
permissions:
  contents: read      # was: contents: write
  packages: write
```

| Job | `permissions` | `timeout-minutes` | Notes |
|---|---|---|---|
| `images` | `contents: read`, `packages: write` | 45 **[UNRATIFIED]** | Currently unbounded; guess, as publish-edge. |
| `chart` | `contents: read`, `packages: write` | 20 **[UNRATIFIED]** | Currently unbounded. |
| `github-release` | `contents: write` | 15 **[UNRATIFIED]** | **Only** job needing `contents: write` — it creates the GitHub Release. Currently unbounded. |
| `modules` | `contents: read`, `packages: write` | 30 **[UNRATIFIED]** | Currently unbounded. |

This is the second-biggest privilege reduction in the feature: `contents: write` is
currently granted to all four release jobs, including the three that only read the tree and
push to a registry. Only `github-release` writes to the repository.

No `concurrency` block needed (`on: push: tags` only, not `push`-to-branch or
`pull_request`); cancelling an in-flight release is undesirable regardless.

---

## `republish-modules.yaml`

```yaml
permissions:
  contents: read
  packages: write
```

| Job | `permissions` | `timeout-minutes` | Notes |
|---|---|---|---|
| `modules` | `contents: read`, `packages: write` | 30 **[UNRATIFIED]** | Currently unbounded. |

`workflow_dispatch` only — no concurrency block needed.

---

## `ai-review.yaml` *(new)*

```yaml
permissions: {}       # top-level: nothing by default
```

| Job | `permissions` | `timeout-minutes` | Notes |
|---|---|---|---|
| `collect` | `contents: read` | 10 | Untrusted: checks out PR head. **No secrets.** Produces the artifact only. |
| `review` | `contents: read`, `pull-requests: write`, `actions: read` | 15 | Privileged: holds `ANTHROPIC_API_KEY`. **Never checks out PR code.** `actions: read` is needed to download the artifact from the triggering run. |

The empty top-level default is deliberate here and safe — unlike the other workflows, both
jobs declare exactly what they need, and neither fetches a private submodule.

---

## Secret confinement

| Secret | Permitted in | Forbidden in | Enforced by |
|---|---|---|---|
| `COSIGN_PRIVATE_KEY`, `COSIGN_PASSWORD` | `images.yaml`, `publish-edge.yaml`, `release.yaml`, `republish-modules.yaml` | `ci.yaml`, `ai-review.yaml` | Code review — not automatically enforced. |
| Registry credentials (`docker/login-action`) | same four | `ci.yaml`, `ai-review.yaml` | Code review — not automatically enforced. |
| `ANTHROPIC_API_KEY` | `ai-review.yaml` job `review` **only** | everywhere else, including `ai-review.yaml` job `collect` | Code review — not automatically enforced. |

These constraints follow from the trust model (ai-review-contract.md) and are validated during code review.

---

## Fork-PR degradation

GitHub silently downgrades every write scope to read on a fork PR, whatever the YAML says.
Two jobs are affected:

| Job | Write attempt | Required behavior |
|---|---|---|
| `ci.yaml` → `web` | `POST /statuses/{sha}` | Already `continue-on-error: true` with `|| true`. Keep. |
| `ci.yaml` → `report` | upsert PR comment | Already detects and skips quietly. Keep. |
| `ai-review.yaml` → `review` | upsert PR comment | Must fall back to `$GITHUB_STEP_SUMMARY`. **New code — must implement.** |

A fork PR must never show a red job for a permission it was never going to be granted.
