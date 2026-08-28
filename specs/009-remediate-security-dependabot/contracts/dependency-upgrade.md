# Dependabot PR Merge Contract

## Per-PR Gate (Required Before Merge)

Every Dependabot PR MUST satisfy:

1. **All checks in our own `ci` workflow green** (`.github/workflows/ci.yaml`): `lint`, `go`, `web`, `web-e2e-mock`, `helm`, `chart-template`, `go-e2e-unit`, `e2e-buckets`, `e2e-go` (all buckets and architectures), `e2e-multicluster`, `e2e-upgrade`, `e2e-web-live`, `e2e-game-bot`, and `report`.

2. **GitHub-owned workflows are NOT merge gates**: CodeQL, Advanced Security, Copilot, Dependabot, and Dependabot Security Update workflows can fail without blocking the merge. These are known to have infrastructure flakes and are not owned by this project.

3. **No changes are required to address GitHub-owned workflow failures** — they are not indicative of actual defects in the code.

---

## Merge Mechanics

Each Dependabot PR is merged **individually**, not consolidated into an integration branch. The merge command is:

```bash
gh pr merge <PR_NUMBER> -R ValgulNecron/Gameplane --admin --merge
```

**Flag requirements**:

- `--admin`: **Required**. The master branch has a ruleset with an "update" rule that blocks a plain `gh pr merge` call. Only a `--admin` merge (using your user's authorization for green PRs) bypasses this restriction.
- `--merge`: **Required**. Use a merge commit, not squash. Merge commits preserve the PR's commit history and are reversible with `git revert` if needed; squash loses that history.

**Authorization**: You have authorized `--admin` merges for all green PRs in this repository, as per the project's merge policy.

---

## Rebase Protocol

When a merge to master occurs, it can invalidate a sibling Dependabot PR's `go.sum` entries if that PR is based on an older master state.

**Rebase workflow**:

1. After a Dependabot PR merges to master, check the next queued PR's CI run.
2. If a `go.mod`/`go.sum` conflict is detected (CI reports a tidying or sum-check error), comment on that PR:
   ```
   @dependabot rebase
   ```
3. Dependabot will automatically rebase the PR onto the new master state, re-run CI, and update the PR's head branch.
4. Once rebased CI is green, proceed to merge.

**Expected behavior**: Workspace-wide bumps (e.g., `x/mod`, `x/net`, `minio-go`) that touch 7–8 modules will cause multiple sibling PRs to require rebase. This is normal and expected; the rebase workflow is the standard recovery path.

---

## Ordering Rule (Ascending Blast Radius)

Dependabot PRs are merged in order of **ascending blast radius** (number of Go modules or projects touched), with **Go and npm running in parallel** since they touch disjoint file trees.

**Rationale**: Merging the smallest, most-isolated PRs first reduces the risk of compounding failures when a larger PR introduces an issue. Parallel tracks for Go and npm avoid sequential delays on unrelated updates.

### Merge Sequence (ordered by blast radius, with parallel tracks noted)

**Phase 1 — Smallest blast radius (1 module or 1 pkg)**

These can be merged in any order; each touches only one package or a single small module.

```
npm: #283 (SECURITY: brace-expansion + js-yaml)
npm: #280 (@types/react-dom)
npm: #278 (vitest)
npm: #277 (@vitejs/plugin-react)
npm: #275 (@types/node)
npm: #270 (@tanstack/react-router)
npm: #266 (@playwright/test)
npm: #264 (@testing-library/jest-dom)
npm: #262 (@typescript-eslint/parser)

Go:  #276 (gopacket, 2 modules: capture-sidecar, test/e2e)
```

**Phase 2 — Medium blast radius (3 modules)**

```
Go:  #279 (sigstore/cosign/v2, 3 modules: capture-sidecar, operator, test/e2e)
```

**Phase 3 — Larger blast radius (5+ modules)**

These touch multiple core modules and should be validated thoroughly after Phase 1–2.

```
Go:  #267 (chi/v5, 5 modules)
Go:  #281 (modernc.org/sqlite, 5 modules: api, capture-sidecar, mpc-server, operator, test/e2e)

Go:  #271 (k8s.io/api, 7 modules)
Go:  #274 (golang.org/x/mod, 7 modules)
Go:  #269 (golang.org/x/net, 7 modules)

Go:  #273 (minio/minio-go/v7, 8 modules)
Go:  #265 (google/go-containerregistry, 8 modules)
```

**Phase 4 — Blocked (awaiting diagnosis or deferred)**

These PRs have failing checks or require a separate gated phase and are NOT merged in this pass:

```
Go:  #263 (sigstore/sigstore 1.10.9, 3 modules) — 1 FAILING CHECK, cause not yet diagnosed
Go:  #272 (TypeScript 7.0.2, MAJOR, 4 FAILING CHECKS) — DEFERRED to separate gated phase
npm: #268 (@eslint/js 10.0.1, MAJOR, 1 FAILING CHECK) — DEFERRED to separate gated phase
```

---

## Acceptance Criterion Per PR

A Dependabot PR is considered successfully merged when:

1. The target version from the version inventory in the PR title/body is present on the master branch after the merge (e.g., `sqlite 1.57.0` appears in all affected `go.mod` files).
2. GitHub marks the PR as closed by Dependabot (usually within seconds of the merge commit being pushed).
3. The next CI run on master (triggered by the merge commit) shows all `ci` workflow checks green.

**Verification command**:
```bash
# Confirm the bumped version is in go.mod or package.json on master
git checkout master && git pull
grep "sigstore/cosign" go.mod  # example for PR #279
# or
grep '"vitest"' web/package.json  # example for PR #278
```

---

## Exception List

### Exception #263: sigstore/sigstore 1.10.9 (Go, 3 modules)

**Status**: 1 failing check; cause not yet diagnosed.  
**Action**: Deferred from this merge wave. Requires investigation of the failing check (likely a transitive dependency conflict with `sigstore/cosign` or `k8s.io/api`). Investigation may happen in parallel; if the cause is identified as a non-blocker, #263 can be merged as a follow-up once the diagnosis is documented.  
**PR #279 (cosign 2.6.5)**: Merges in Phase 3. #263 is separate and can be merged afterward if the diagnosis clears it, or skipped if it remains blocked.

### Exception #272: TypeScript 7.0.2 (npm, MAJOR, 4 failing checks)

**Status**: Major version bump with 4 failing checks; likely requires type annotations or config updates in the frontend code.  
**Action**: **DEFERRED to a separate gated phase**. This PR will be merged only after all non-major Dependabot PRs (#262–#281 except #268/#272) are on master and passing. A dedicated TypeScript 7 migration task will then be created with explicit scope and review criteria.  
**Rationale**: TypeScript major version bumps often require manual type annotation changes. This PR is intentionally isolated to avoid compounding failure modes when other packages have unrelated issues.

### Exception #268: @eslint/js 10.0.1 (npm, MAJOR, 1 failing check)

**Status**: Major version bump (drops eslintrc, drops deprecated methods, requires Node ^20.19 || ^22.13 || >=24, updates eslint:recommended, minimatch v10); 1 failing check.  
**Action**: **DEFERRED to a separate gated phase** alongside #272. ESLint 10 is a semver-major bump that may require config changes in `web/eslint.config.js` and possibly rule adjustments. Like TypeScript 7, it is merged only after all other PRs stabilize.  
**Rationale**: ESLint major bumps often introduce new rule categories or renamed rules. Isolating it prevents it from masking issues in other dependency updates.

### Exception: #283 Security (SECURITY brace-expansion + js-yaml)

**Status**: NOT originally named in the specification (spec says 20 PRs #262–#281).  
**Note**: This is a real security fix (CVE-2026-13149 / GHSA-mh99-v99m-4gvg in brace-expansion + DoS in js-yaml). #283 is green and must be merged before or alongside the first wave. It is included in Phase 1 (smallest blast radius, 1 package, `web/` only).  
**Correction**: The feature's 20-PR estimate in the spec should be 21 PRs; #283 is within scope.

---

## Summary Table

| Phase | PRs | Blast Radius | Status | Notes |
|-------|-----|--------------|--------|-------|
| 1 | #283, #280, #278, #277, #275, #270, #266, #264, #262, #276 | 1–2 modules | All green | Merge in any order |
| 2 | #279 | 3 modules | All green | Cosign; proceed after Phase 1 |
| 3 | #267, #281, #271, #274, #269, #273, #265 | 5–8 modules | All green | Core libraries; validate Phase 1–2 first |
| Blocked | #263 | 3 modules | 1 failing | Diagnosis pending; can follow-up merge if cleared |
| Gated | #272, #268 | — | 4 + 1 failing | Deferred to separate TypeScript 7 & ESLint 10 phase |

---

## Key Points for Execution

- **No integration branch**: Each PR is merged individually with `--admin --merge`, not consolidated first.
- **Rebase on conflict**: Use `@dependabot rebase` to update `go.sum` if a sibling PR's CI fails after a merge.
- **Parallel tracks**: npm PRs and Go PRs can be merged concurrently (they touch disjoint files); order within each track by blast radius.
- **All-green gate**: Do not merge on partial-green or ignored flakes; every `ci` workflow check must pass.
- **Exceptions are isolated**: #263, #272, #268 are tracked separately and do not block the main wave.
