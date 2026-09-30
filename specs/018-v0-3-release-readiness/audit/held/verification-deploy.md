# T045 chart chunk: independent verification (opus)

Held under OD-019, off-git. This file covers the one candidate in `held/review-deploy.md`. It is worded defensively: it names the control, where it lives, the correct behaviour, and how a maintainer confirms the control holds.

Method: I read `deploy/kind/up.sh` and `deploy/kind/e2e.sh` on master `13a859ff` (the branch is identical for `deploy/`). I listed every remote URL either script fetches (`grep -n 'https://' deploy/kind/*.sh`) and sent one anonymous HEAD request for the cited manifest URL (HTTP 200: it resolves today). Nothing was applied to any cluster, no test or lint suite was run, and no repo file was changed; this file is off-git. I checked `audit/findings.md` and the other held files for duplicates and found none.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-deploy-H01 | kept | S4 | Confirmed. `up.sh:196` applies `https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml` from the floating `main` branch, while the same script's rule for MetalLB (`:28-31`) forbids floating refs and pins `v0.14.9`, which `:43` and `e2e.sh:42` use. e2e.sh does not install ingress-nginx, so CI is not affected. The manifest comes over HTTPS from the upstream Kubernetes organisation, and only the local dev cluster (`make dev-up`, kind path) applies it. There is no user-install or release path through it, and nothing fails today. So this is a consistency and hardening gap in dev tooling, and S4. |

### C-deploy-H01

**Location:** `deploy/kind/up.sh:196`. The pinning rule it breaks is stated at `deploy/kind/up.sh:28-31`. The only other remote manifest in `deploy/` is MetalLB's, which is pinned (`up.sh:43`, `e2e.sh:42`).

**Control:** supply-chain pinning of third-party manifests applied to the dev cluster with the kind admin credentials, as `up.sh:28-30` states it: "Bumping it is a deliberate edit — never fetch a floating ref".

**Repro / observation:**
1. `grep -n 'raw.githubusercontent.com' deploy/kind/*.sh`. MetalLB URLs interpolate `${METALLB_VERSION}` (a tag). The ingress-nginx URL uses `/main/`.
2. `sed -n 192,201p deploy/kind/up.sh`: the manifest is applied whenever the `ingress-nginx` namespace is missing, which is every fresh `make dev-up`.

**Expected (correct behaviour):** The ingress-nginx manifest URL names an immutable release tag (for example `controller-vX.Y.Z`) or a commit SHA, held in a variable next to `METALLB_VERSION` and bumped deliberately. A checksum check of the fetched file is optional.

**Actual:** Each fresh dev cluster applies whatever upstream `main` holds at that moment.

**How a maintainer confirms it holds:** `grep -nE 'raw.githubusercontent.com/.+/(main|master)/' deploy/kind/*.sh` returns nothing, and every remote manifest URL in `deploy/` resolves to a tag or commit.
