# Contract: Action Pin Registry

**Feature**: 008-hardened-github-actions | **Resolved**: 2026-08-29

Authoritative SHA pins for every external GitHub Action referenced in this repository.
Resolved with `git ls-remote --tags --refs https://github.com/<owner>/<repo>`, taking the
highest `sort -V` tag matching the currently-referenced major — **sorting on the tag field,
not the raw `ls-remote` line** (see the warning immediately below; this is not optional).

**Reproduce this table**:

```sh
for a in actions/checkout@v7 actions/cache@v6 ... ; do
  repo=${a%@*}; tag=${a#*@}
  git ls-remote --tags --refs "https://github.com/$repo" \
    | awk -F'refs/tags/' '{print $2, $1}' \
    | grep -E "^${tag}(\.[0-9]+)*( |$)" | sort -V | tail -1
done
```

**Why the `awk` step is required — do not drop it.** `git ls-remote` prints
`<SHA>\t<ref>`, SHA first. Piping that straight into `sort -V` sorts on the *SHA*, which is
effectively random hex, not on the version tag that comes after it — `sort -V` never sees a
version number to compare. The `awk -F'refs/tags/' '{print $2, $1}'` step reorders each line
to `<tag> <SHA>` first, so `sort -V` finally sorts what it's supposed to.

**Incident, so this isn't just a warning nobody reads.** The original (broken) recipe was the
plain `grep ... | sort -V | tail -1` form, no `awk`. Run against `helm/kind-action` it returned
`fa81e57adff234b2908110485695db0f181f3c67` / **v1.7.0** (released 2023) instead of the true
latest, **v1.14.0** (`ef37e7f390d99f746eb8b610417061a60e82a6cc`) — because the SHA `fa81e57…`
happens to sort higher than `ef37e7f…` under `sort -V`, and the broken recipe was comparing
SHAs, not tags. `kind-action@v1.7.0` defaults to kind v0.19.0, whose node image is
`kindest/node:v1.27.1` — **Kubernetes 1.27**, one minor below this project's documented
1.28+ target (CLAUDE.md) and built against `client-go` v0.35.0. Result: `e2e web live` failed
on **four consecutive CI runs** with `[vite] http proxy error` / `Error: read ECONNRESET` /
`seed template … failed: 502`. `master` never saw it, because `master` still floats `@v1`
(which resolves live to v1.14.0) — so the same job passed on `master` the same day running
*newer* application code (`ad3e30a2`, 2026-08-30), while the branch failed running *older*
code (identical to the 2026-08-28 fork point) pinned to a three-year-stale action. That
inversion is the tell: it rules out an application regression and points squarely at the pin. Fixed in
commit `ced90c58` (all five `ci.yaml` call sites re-pinned to `ef37e7f390d99f746eb8b610417061a60e82a6cc # v1.14.0`).

**Measured scope — don't overstate this.** The broken (SHA-sorting) recipe was re-run against
every pinned action in the table below that has a bare major tag. It diverges from the correct
answer for exactly **two**:

- `helm/kind-action` — v1.7.0 vs v1.14.0 (severe: 7 minor versions, ~3 years stale; see incident above).
- `actions/checkout` — v7.0.0 vs v7.0.1 (trivial: one patch. The current `v7.0.0` pin is
  fine as-is; upgrading it is Dependabot's job, not something to change in this fix).

The other six bare-major pins (`actions/cache`, `actions/setup-go`, `actions/setup-node`,
`docker/bake-action`, `docker/login-action`, `docker/metadata-action`) happened to return the
*correct* SHA under the broken recipe too — but that is a coincidence of how their SHAs
happen to sort as hex strings, not evidence the method was sound. A method that is right by
luck is still broken; hence the fix above, applied uniformly rather than only where it was
caught diverging.

---

## The 18 pins

| # | Action | Current ref | Pin SHA | Tag | Tier |
|---|---|---|---|---|---|
| 1 | `actions/cache` | `@v6` | `55cc8345863c7cc4c66a329aec7e433d2d1c52a9` | `v6.1.0` | first-party |
| 2 | `actions/checkout` | `@v7` | `9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0` | `v7.0.0` | first-party |
| 3 | `actions/download-artifact` | `@v8` | `70fc10c6e5e1ce46ad2ea6f2b72d43f7d47b13c3` | `v8.0.0` | first-party |
| 4 | `actions/setup-go` | `@v7` | `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` | `v7.0.0` | first-party |
| 5 | `actions/setup-node` | `@v7` | `820762786026740c76f36085b0efc47a31fe5020` | `v7.0.0` | first-party |
| 6 | `actions/upload-artifact` | `@v7` | `bbbca2ddaa5d8feaa63e36b76fdaad77386f024f` | `v7.0.0` | first-party |
| 7 | `azure/setup-helm` | `@v5` | `dda3372f752e03dde6b3237bc9431cdc2f7a02a2` | `v5.0.0` | verified |
| 8 | `docker/bake-action` | `@v7` | `d3418bd7d0e9324001bca92fa8ba175ea7e6dc9b` | `v7.3.0` | verified |
| 9 | `docker/build-push-action` | `@v7` | `f9f3042f7e2789586610d6e8b85c8f03e5195baf` | `v7.2.0` | verified |
| 10 | `docker/login-action` | `@v4` | `dbcb813823bdd20940b903addbd779551569679f` | `v4.6.0` | verified |
| 11 | `docker/metadata-action` | `@v6` | `dc802804100637a589fabce1cb79ff13a1411302` | `v6.2.0` | verified |
| 12 | `docker/setup-buildx-action` | `@v4` | `d7f5e7f509e45cec5c76c4d5afdd7de93d0b3df5` | `v4.1.0` | verified |
| 13 | `docker/setup-qemu-action` | `@v4` | `ce360397dd3f832beb865e1373c09c0e9f86d70a` | `v4.0.0` | verified |
| 14 | `dorny/paths-filter` | `@v4` | `fbd0ab8f3e69293af611ebaee6363fc25e6d187d` | `v4.0.1` | community |
| 15 | `golangci/golangci-lint-action` | `@v9` | `e7fa5ac41e1cf5b7d48e45e42232ce7ada589601` | `v9.1.0` | verified |
| 16 | `helm/kind-action` | `@v1` | `ef37e7f390d99f746eb8b610417061a60e82a6cc` | `v1.14.0` | verified |
| 17 | `oras-project/setup-oras` | `@v2` | `38de303aac69abb66f3e6255b7198bff35f323e3` | `v2.0.0` | verified |
| 18 | `sigstore/cosign-installer` | `@v4.1.2` | `6f9f17788090df1f26f669e9d70d6ae9567deba6` | `v4.1.2` | verified |
| 19 | `anthropics/claude-code-action` | *(new)* | `a874e9ecd7bb36efdad65429c6b35815f5a08f10` | `v1.0.210` | verified |

Entry 19 is not in the tree yet; it arrives with `ai-review.yaml` (T033–T036). Resolved
2026-08-30 by the same method as the rest of the table.

**Note:** `v1.0.210` is an **annotated tag** in the claude-code-action repository — the only
one among the pinned actions. Annotated tags have two entries in `git ls-remote --tags` output:
the tag object itself (line 1: `50b26a71effe456d50842a733597491c5636cb6f  refs/tags/v1.0.210`)
and the peeled commit (line 2: `a874e9ecd7bb36efdad65429c6b35815f5a08f10  refs/tags/v1.0.210^{}`).
GitHub Actions resolves `uses: owner/repo@<sha>` to a commit, so the pin must use the peeled
commit SHA from the `^{}` line, not the tag object. The resolution method documented here
(`git ls-remote --tags --refs`) strips the `^{}` entries, which is why this was caught after
deployment. The corrected method: read `git ls-remote --tags` without `--refs`, and prefer the
`refs/tags/X^{}` line when present.

`dorny/paths-filter` is the sole `community`-tier action and drives the `changes` job that
gates every other job's execution. It is the highest-value pin in the table.

---

## Required form

```yaml
- uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
```

Rules:

1. Exactly 40 lowercase hex characters after `@`.
2. Exactly one space, then `#`, then one space, then the `vX.Y.Z` tag. Dependabot parses
   this comment to propose upgrades; a deviation makes the pin unmaintainable.
3. No other text on the line after the tag.
4. The same `owner/repo` pins the same SHA at every call site in the repository.

Local composite actions are exempt and keep their path form — they are versioned by the
checkout itself:

```yaml
- uses: ./.github/actions/go-cache
```

---

## Call-site inventory

Where each pin must be applied. Every file below is modified by this feature.

| File | Actions referenced |
|---|---|
| `.github/workflows/ci.yaml` | checkout, paths-filter, setup-buildx, build-push, golangci-lint, setup-node, cache, upload-artifact, setup-helm, kind-action |
| `.github/workflows/images.yaml` | checkout, setup-qemu, setup-buildx, login, metadata, build-push, cosign-installer |
| `.github/workflows/publish-edge.yaml` | checkout, setup-qemu, setup-buildx, login, metadata, build-push, cosign-installer, upload-artifact |
| `.github/workflows/release.yaml` | checkout, setup-qemu, setup-buildx, login, metadata, build-push, cosign-installer, setup-helm, setup-oras |
| `.github/workflows/republish-modules.yaml` | checkout, setup-oras, cosign-installer, login |
| `.github/actions/build-e2e-images/action.yml` | setup-buildx, bake-action, upload-artifact |
| `.github/actions/e2e-images/action.yml` | download-artifact |
| `.github/actions/go-cache/action.yml` | setup-go, cache |
| `.github/actions/dump-cluster-state/action.yml` | *(none — pure `run:` steps)* |
| `.github/workflows/ai-review.yaml` *(new)* | checkout, upload-artifact, download-artifact, claude-code-action |

---

## Verification

The workflow-lint gate (actionlint + zizmor) enforces the required form:

- **actionlint** validates schema and type errors, malformed `uses:` keys, expression
  injection into `run:` bodies, and deprecated syntax. It does not enforce SHA pinning.
- **zizmor** detects unpinned `uses:` refs — validating that every action reference is a
  40-character lowercase hex SHA, not a tag or branch. This is the control that makes
  SC-001 mechanically verifiable.

The `# vX.Y.Z` comment convention is upheld by code review only; it is not enforced by
tooling. This comment syntax enables Dependabot to parse and propose version upgrades.

**SC-001 is satisfied when zizmor reports no unpinned actions across all workflow and
action files.**
