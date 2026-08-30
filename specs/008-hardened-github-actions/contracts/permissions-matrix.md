# Contract: Job Permission & Timeout Matrix

**Feature**: 008-hardened-github-actions | **Date**: 2026-08-29

The target state for every job in the repository. Permissions misuse is enforced by the workflow-lint gate (zizmor); this matrix's specific per-job scopes are code-review-only. Timeout configuration is code-review-only.

> ⚠️ **Timeouts are ratified.** See OPEN-DECISIONS.md D-A. Jobs exceeding FR-004's ≤30 default: five E2E jobs at 60 (e2e-go, e2e-multicluster, e2e-upgrade, e2e-web-live, e2e-game-bot), plus publish-edge.images at 35. The E2E budget is the maintainer's explicit call, justified because "an image build has no business taking longer than the entire E2E suite it feeds." Release.images at 30 is at the ceiling, not above it. Values marked **[EXTENSION]** are applied by extension of the ruling and may be vetoed by the maintainer. Removed [UNRATIFIED] markers where the ruling settled the value.
>
> The `packages: write` top-level blocks have been corrected — FR-001 holds strictly.
> See OPEN-DECISIONS.md D-E (already implemented in commit 01af5953).

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
| `e2e-go` | `contents: read` | 60 | All matrix legs run with the 60-minute E2E budget. |
| `e2e-multicluster` | `contents: read` | 60 | |
| `e2e-upgrade` | `contents: read` | 60 | |
| `e2e-web-live` | `contents: read` | 60 | |
| `e2e-game-bot` | `contents: read` | 60 | E2E suite timeout per D-A. |
| `report` | `contents: read`, `statuses: read`, `pull-requests: write` | 5 | Already correct. Fork-degradation already handled. |
| `workflow-lint` *(new)* | `contents: read` | 5 | The workflow-lint gate runs actionlint and zizmor over `.github/workflows/`. Gated on the new `github` path-filter output. |


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
| `common-base` | `contents: read`, `packages: write` | 10 **[EXTENSION]** | 10 minutes per image; matrix legs run in parallel. |
| `game-images` | `contents: read`, `packages: write` | 10 | 10 minutes per image; matrix legs run in parallel. |

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
| `images` | `contents: read`, `packages: write`, `id-token: write` | 35 | Multi-arch buildx + cosign. Observed max 31m across 142 samples (12 runs). Exceeds FR-004 ≤30 default; measured data backs the increase. Add `id-token: write` only if keyless signing is in use; this repo signs keyed/offline with `COSIGN_PRIVATE_KEY`, so **omit it** unless verified otherwise during implementation. |

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
| `images` | `contents: read`, `packages: write` | 30 | Multi-arch buildx + cosign. Observed max 26m across 29 samples (5 runs). At FR-004 ≤30 ceiling; measured data backs the value. |
| `chart` | `contents: read`, `packages: write` | 15 **[EXTENSION]** | Release budget. |
| `github-release` | `contents: write` | 15 **[EXTENSION]** | **Only** job needing `contents: write` — it creates the GitHub Release. Release budget. |
| `modules` | `contents: read`, `packages: write` | 15 **[EXTENSION]** | Release budget. |

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
| `modules` | `contents: read`, `packages: write` | 15 **[EXTENSION]** | Module push budget. |

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
