# Contract: Action Pin Registry

**Feature**: 008-hardened-github-actions | **Resolved**: 2026-08-29

Authoritative SHA pins for every external GitHub Action referenced in this repository.
Resolved with `git ls-remote --tags --refs https://github.com/<owner>/<repo>`, taking the
highest `sort -V` tag matching the currently-referenced major.

**Reproduce this table**:

```sh
for a in actions/checkout@v7 actions/cache@v6 ... ; do
  repo=${a%@*}; tag=${a#*@}
  git ls-remote --tags --refs "https://github.com/$repo" \
    | grep -E "refs/tags/${tag}(\.[0-9]+)*$" | sort -V | tail -1
done
```

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
| 16 | `helm/kind-action` | `@v1` | `fa81e57adff234b2908110485695db0f181f3c67` | `v1.7.0` | verified |
| 17 | `oras-project/setup-oras` | `@v2` | `38de303aac69abb66f3e6255b7198bff35f323e3` | `v2.0.0` | verified |
| 18 | `sigstore/cosign-installer` | `@v4.1.2` | `6f9f17788090df1f26f669e9d70d6ae9567deba6` | `v4.1.2` | verified |
| 19 | `anthropics/claude-code-action` | *(new)* | `50b26a71effe456d50842a733597491c5636cb6f` | `v1.0.210` | verified |

Entry 19 is not in the tree yet; it arrives with `ai-review.yaml` (T033–T036). Resolved
2026-08-30 by the same method as the rest of the table.

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

Enforced by `.github/workflows-verify.sh` rule **R1**. The check, in essence:

```sh
# every `uses:` naming owner/repo must carry a 40-hex ref and a version comment
grep -rnE '^\s*(-\s+)?uses:\s+[^./]' .github/workflows .github/actions \
  | grep -vE '@[0-9a-f]{40} # v[0-9]+\.[0-9]+\.[0-9]+\s*$'
# any output = failure
```

The verifier's real implementation parses YAML with `python3` rather than grepping, so a
`uses:` inside a comment or a heredoc cannot produce a false positive or a false negative.

**SC-001 is satisfied when this check returns no output across all 10 files.**
