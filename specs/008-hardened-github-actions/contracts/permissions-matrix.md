# Contract: Job Permission & Timeout Matrix

**Feature**: 008-hardened-github-actions | **Date**: 2026-08-29

The target state for every job in the repository. Enforced by
`.github/workflows-verify.sh` rules **R2**, **R3**, **R4**, **R8**.

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
| `workflows-verify` *(new)* | `contents: read` | 5 | Gated on the new `github` path-filter output. |

**`e2e-go` note**: `timeout-minutes: ${{ matrix.job_timeout }}` is an expression, so the
verifier cannot read a literal. R4 resolves the matrix's `job_timeout` values and checks
each against the ceiling, or the rule would be trivially bypassable by any future job that
switches to an expression. Doing so surfaced a leg the plan had assumed away: the
`operator` bucket is 35, not ≤ 30. It is legitimate — 8-way parallel behind a 20m test
timeout, plus cluster-boot headroom — so it is recorded in R4's exception table rather than
lowered, which would have reddened a real bucket.

**`report` wiring**: adding `workflows-verify` requires three edits in the reporter — the
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
| `common-base` | `contents: read`, `packages: write` | 30 | Currently unbounded. |
| `game-images` | `contents: read`, `packages: write` | 60 | Currently unbounded. **Exception** — SteamCMD downloads dominate. |

Also: this workflow triggers on `push`, `pull_request`, and `workflow_dispatch` but has no
`concurrency` block. R5 requires one. Recommended:

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
| `images` | `contents: read`, `packages: write`, `id-token: write` | 45 | Currently unbounded. **Exception** — multi-arch buildx + cosign across 8 images. Add `id-token: write` only if keyless signing is in use; this repo signs keyed/offline with `COSIGN_PRIVATE_KEY`, so **omit it** unless verified otherwise during implementation. |

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
| `images` | `contents: read`, `packages: write` | 45 | Currently unbounded. **Exception** — as publish-edge. |
| `chart` | `contents: read`, `packages: write` | 20 | Currently unbounded. |
| `github-release` | `contents: write` | 15 | **Only** job needing `contents: write` — it creates the GitHub Release. Currently unbounded. |
| `modules` | `contents: read`, `packages: write` | 30 | Currently unbounded. |

This is the second-biggest privilege reduction in the feature: `contents: write` is
currently granted to all four release jobs, including the three that only read the tree and
push to a registry. Only `github-release` writes to the repository.

No `concurrency` block — R5 exempts it (`on: push: tags` only, not `push`-to-branch or
`pull_request`), and cancelling an in-flight release is undesirable regardless.

---

## `republish-modules.yaml`

```yaml
permissions:
  contents: read
  packages: write
```

| Job | `permissions` | `timeout-minutes` | Notes |
|---|---|---|---|
| `modules` | `contents: read`, `packages: write` | 30 | Currently unbounded. |

`workflow_dispatch` only — R5 exempt.

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

## Secret confinement (R8)

| Secret | Permitted in | Forbidden in |
|---|---|---|
| `COSIGN_PRIVATE_KEY`, `COSIGN_PASSWORD` | `images.yaml`, `publish-edge.yaml`, `release.yaml`, `republish-modules.yaml` | `ci.yaml`, `ai-review.yaml` |
| Registry credentials (`docker/login-action`) | same four | `ci.yaml`, `ai-review.yaml` |
| `ANTHROPIC_API_KEY` | `ai-review.yaml` job `review` **only** | everywhere else, including `ai-review.yaml` job `collect` |

R8 fails on any reference to a confined secret outside its permitted file, and on any
reference to `ANTHROPIC_API_KEY` inside the `collect` job.

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
