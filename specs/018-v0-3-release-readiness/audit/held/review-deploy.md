# Held review candidates: deploy/ (OD-019, off-git)

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Source review**: `audit/evidence/review-deploy/notes.md` (T042, chunk "chart")
- **Why held**: supply chain (an unpinned third-party manifest applied with cluster-admin rights). It is written defensively.

## Candidate findings

### C-deploy-H01: `up.sh` applies the ingress-nginx manifest from the floating `main` branch, against the script's own pinning rule
- **Location**: `deploy/kind/up.sh:196` (`kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml`); the rule it breaks is stated at `deploy/kind/up.sh:28-30` for MetalLB
- **Category**: correctness
- **Suggested severity**: S4 (dev-only cluster)
- **Control**: supply-chain pinning of third-party manifests applied to the dev cluster with the kind admin credentials.
- **Expected (correct behaviour)**: up.sh:28-30 states the rule for MetalLB: "Bumping it is a deliberate edit — never fetch a floating ref, or a fresh cluster silently changes its load-balancer implementation." The ingress controller manifest follows the same rule: a tagged release URL (e.g. `controller-vX.Y.Z`), ideally with a checksum check.
- **Actual**: Each fresh `make dev-up` applies whatever the upstream `main` branch holds at that moment. Upstream ingress-nginx has also announced end of maintenance, which makes the content of `main` less predictable over time.
- **How a maintainer confirms it holds**: `grep -n 'raw.githubusercontent.com' deploy/kind/*.sh`. Every URL resolves to an immutable tag or commit, with no `main`/`master` refs.

## Questions (not findings)

- None.
