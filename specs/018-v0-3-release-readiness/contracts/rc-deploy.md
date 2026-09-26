# Contract: Publishing a Release Candidate and Deploying It to kubelab

Covers FR-018 and FR-019. Background is in [../research.md](../research.md) sections R4 to R6.

## 1. Publish `v0.3.0-rc.N` (needs maintainer approval every time)

Preconditions:
- Every fix for this round is merged to `master`, and CI is green on the commit to tag.
- `CHANGELOG.md` has a `## [0.3.0-rc.N]` section. Without it, `release.yaml:226-236` falls back to auto-generated notes.
- The approval request is logged in `OPEN-DECISIONS.md` as `RC-TAG-N: pending`, with the commit SHA and the changelog summary.

After the maintainer approves (and only then):
```sh
git tag -s v0.3.0-rc.N <sha> -m "v0.3.0-rc.N"
git push origin v0.3.0-rc.N
```

Expected results, recorded in `rounds.md`:
- `release.yaml` run is green.
- Images `ghcr.io/valgulnecron/gameplane/<component>:v0.3.0-rc.N` and `:0.3.0-rc.N` exist and are cosign-signed. Check with `cosign verify --key cosign.pub …`.
- Chart `oci://ghcr.io/valgulnecron/charts/gameplane` version `0.3.0-rc.N` exists and is signed.
- The GitHub release is marked **prerelease**.
- The `0.3` image tag was **not** created or moved. If it was, that's a finding.
- Module bundles were republished with the same signing. Note: the chart's default git module source is pinned to `ref: v0.2.0-beta.6` (`charts/gameplane/values.yaml:473`). A matching `gameplane-module` tag for v0.3.0 has to exist before the final release. That's seeded as a finding.

## 2. Deploy to kubelab

Preconditions: `audit/kubelab-baseline.md` has been captured, and the DB snapshot has been taken and stored off-git (research R6).

```sh
export KUBECONFIG=~/kubelab.yaml
helm upgrade <release> oci://ghcr.io/valgulnecron/charts/gameplane \
  --version 0.3.0-rc.N -n <ns> --reuse-values \
  <only the overrides listed in rounds.md for this round>
```

- The first move from the private side-loaded tag to public GHCR images needs overrides for exactly the image registry and tag keys that differ from the chart defaults. They are listed by key name in `rounds.md`.
- These must be left untouched unless the change is itself under test: module source, ingress host, storage, OIDC, and every other site-specific value.
- After the deploy, `helm get values` minus the listed overrides must equal the baseline. Any other difference is a finding.

## 2a. beta.8 baseline for the upgrade round (OD-005)

1. Take a real DB snapshot, off-git.
2. If kubelab's migration level is ahead of beta.8, uninstall kubelab's Gameplane release without deleting the CRDs or game PVCs, and reinstall `v0.2.0-beta.8` with a fresh API database. Otherwise run `helm upgrade` to beta.8 directly.
3. Seed the `audit018-` state, upgrade to the RC, and verify.
4. Restore the real database snapshot and re-verify the baseline.

The exact commands and the reinstall's effect on pre-existing GameServers (they must stay running) are recorded in `rounds.md`. Any loss is an S1 finding.

## 3. Roll back (tested once, in the upgrade round)

```sh
helm rollback <release> <previous-revision> -n <ns>
# only if the upgrade applied new DB migrations:
#   scale api to 0, restore the SQLite snapshot into the PVC, scale api back
```
Expected: the previous release serves logins, lists the `audit018-` server, and the data marker is intact. The exact steps that worked are written into `docs/install.md` as the documented rollback procedure (US4 AS-3).

## 4. After the audit

The maintainer decides whether kubelab stays on public `v0.3.0` or goes back to the baseline values. The decision is recorded in `rounds.md`.
