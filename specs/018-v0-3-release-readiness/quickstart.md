# Quickstart: Running and Validating an Audit Round

This guide shows how to run one audit round and check that its output meets the spec. File formats are in [contracts/audit-records.md](contracts/audit-records.md). Deploy steps are in [contracts/rc-deploy.md](contracts/rc-deploy.md). Isolation rules are in [contracts/test-resources.md](contracts/test-resources.md).

## Prerequisites

- `git submodule update --init modules`, needed for the module inventory and category mapping.
- `gh auth status` shows a logged-in account.
- `KUBECONFIG=~/kubelab.yaml kubectl get nodes` lists 3 Ready nodes. If `kubelab-api` doesn't resolve, the live steps are blocked (OD-007). The code reviews and the inventory work can still go ahead.
- `helm`, `curl`, `jq` and `cosign` are installed. Chrome MCP is available for dashboard rows.
- Login budget: IP burst 10 (5/min), user burst 6 (3/min). Plan one login per role per round.

## Round 0: before anything is published

1. Commit `audit/release-criteria.md`. **Check:** every criterion names its `SC-`/`FR-` source.
2. Enumerate `audit/inventory.md` from code (research R2), with every row set to `untested`. **Check:** each area section exists and every row has a `Source` `file:line`.
3. Write `audit/coverage.md` with a row for every required component. **Check:** the row count matches the list in the contract.
4. Import the known bugs into `audit/findings.md` (research R11). **Check:** every listed source has an `F-` row with origin `imported:<source>`.
5. Run the component reviews (sonnet reviewers, opus verifiers) and record the findings. **Check:** every coverage row is `complete`.
6. Capture `audit/kubelab-baseline.md` and the evidence JSON. **Check:** it contains no secret values (`grep -iE 'secret|token|password' audit/kubelab-baseline.md` shows key names only).

## Round N (`rc.N`)

1. Request approval for the tag, then publish it (rc-deploy §1). **Check:** the GHCR images and chart exist, are signed, and the GitHub release is marked prerelease.
2. Take a DB snapshot, then run `helm upgrade` (rc-deploy §2). **Check:** `helm get values` equals the baseline apart from the listed overrides.
3. Run the procedures for every row that is `untested`. Security-violation and brute-force rows go last.
4. For each failure, open or update an `F-` row, then fix it on a `fix/018-F-xxx` branch with a regression test. The fix goes through PR, CI and human review.
5. Clean up. **Check:** there are zero `audit018-` resources, and the baseline diff is clean, apart from the release itself.
6. Log the round in `audit/rounds.md`.

Repeat until `findings.md` has no rows in `imported`, `open`, `fixing` or `fixed-unverified`.

## Upgrade round (once, subject to OD-005)

Start with kubelab on public `v0.2.0-beta.8` and seed an `audit018-` server with a data marker plus an `audit018-admin` user. Upgrade to the latest RC. **Expect:** the server comes back Running, the marker is intact, the admin can log in, and the audit chain `Verify` is ok. Then roll back (rc-deploy §3). **Expect:** the previous release is functional. Record the rows under `INV-UPG-*`.

## Final validation (go/no-go)

Run these checks over the records:

```sh
cd specs/018-v0-3-release-readiness/audit
grep -cE '\| (untested|fail) \|' inventory.md                               # expect 0
grep -cE '\| (imported|open|fixing|fixed-unverified) \|' findings.md        # expect 0
grep -cE '\| (not-started|in-review) \|' coverage.md                        # expect 0
grep -E '\| blocked \|' inventory.md                                         # each row must appear under "Not live-verified" in report.md
```

Also check:
- `report.md` states go or no-go against every `RC-nn` criterion, and a maintainer who didn't run the audit can follow it (SC-008).
- On a go: apply [contracts/status-wording.md](contracts/status-wording.md) in one commit, and re-run its re-check command. The `v0.3.0` tag needs maintainer approval.
