# T045 chart chunk: independent verification (opus)

Verifier input: `notes.md` in this directory (opus reviewer), candidates C-deploy-01 to -04. Held candidates for this component are verified separately, off-git (OD-019).

Method: I read all five files under `deploy/kind/` on master `13a859ff` (`git diff master HEAD -- deploy Makefile charts` is empty), along with the `Makefile` targets that drive them (`:8-35`, `:258-281`, `:336-398`) and the chart's image helpers (`charts/gameplane/templates/_helpers.tpl:11-81`). I asked GHCR anonymously whether `ghcr.io/valgulnecron/gameplane/sentinel:dev` exists; it returns 404. Nothing was run against kind or Docker, because these scripts create and destroy clusters and containers, and `kind` is not installed on this host. No test or lint suite was run, and no repo file other than this one was changed. I checked `audit/findings.md` (F-029 covers only the `FROM_VERSION` baseline) and the other chunks' notes for duplicates: C-deploy-02 is the same defect as C-hack-04.

Severity follows research R3.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-deploy-01 | kept | S3 | Confirmed. When the cluster already exists, `up.sh:154-159` never selects `kind-${CLUSTER}`. Every later `kubectl` call uses the current context: the ConfigMap apply at `:178`, the ingress-nginx check and install at `:194-201`, MetalLB and its pools at `:203-204` (the functions at `:36-133` use plain `kubectl`), and the namespace at `:210`. The only `--context` use (`:190`) is a read-only check that runs after the first apply. `make dev-install` (`Makefile:384`, empty `KUBECONFIG_ENV` for kind) then runs Helm against the same current context. The header (`:6-7`) says the script "exits 0 without modifying it". This is dev tooling, and a workaround exists (select the kind context first), so S3. |
| C-deploy-02 | rejected (duplicate) | (S3 if reinstated) | The defect is real, but it is the same finding as **C-hack-04** (`evidence/review-hack/notes.md:87-99`): the same `Makefile:367-371` `dev-load` recipe, the same 4-of-12 image count and the same `sentinel:dev` pull failure. The Makefile is in the hack review's scope (read in full there), and the deploy notes themselves say it is outside `deploy/`. I confirmed it independently: `dev-load` loads only operator, api, web and agent, `dev-install` sets `image.registry=ghcr.io/valgulnecron/gameplane` and `image.tag=dev`, `_helpers.tpl:43` derives `.../sentinel:dev`, and GHCR has no `:dev` tag for sentinel (manifest GET returns 404). If the hack-chunk verifier drops C-hack-04, reinstate this one at S3. |
| C-deploy-03 | kept | S4 | Confirmed. `e2e.sh:11` says it "Loads pre-built gameplane/{operator,api,agent}:<tag> images", and `:165` prints `loading gameplane/{operator,api,agent,sentinel,capture-sidecar}:${TAG}`. The loop at `:166-172` loads `gameplane-test/{operator,api,agent,sentinel,capture-sidecar}` and builds any that are missing, and `:177-191` also load `gameplane-test/gameprobe` (when present) and `gameplane-test/fakeoidc` (always). This is comment and message wording only. |
| C-deploy-04 | kept | S3 | Confirmed. The guard at `up.sh:147` treats "exists but not running" the same as "absent" and runs `docker run ... --name kind-registry` (`:149-151`). Docker refuses the name while the stopped container still holds it, and `set -e` (`:9`) aborts the bootstrap. The reachable trigger is a manual `docker stop kind-registry` (or stopping all containers). A daemon restart alone is not a trigger, because `--restart=always` brings the container back. The pattern is copied from the upstream kind recipe. The bootstrap fails, but `docker start kind-registry` recovers, so S3 (not S4, since this is a failed run, not wording). Dev tooling only. |

### C-deploy-01

**Location:** `deploy/kind/up.sh:6-7` (the claim) and `:154-159` (the existing-cluster branch, which selects no context). The mutating calls without `--context` are at `:178`, `:194-201`, `:203-204` (`install_metallb` / `apply_metallb_pools`, defined at `:36-133`) and `:210`. The only `--context` use is at `:190`. Follow-on: `Makefile:360-364` (`dev-up` kind path) and `:384-391` (`dev-install`, no `--kube-context`).

**Repro / observation** (by reading master; run live only with a throwaway second cluster as the "other" context):
1. `make dev-up` once, so `kind-gameplane-dev` exists.
2. `kubectl config use-context <another cluster>`.
3. `make dev-up` again. up.sh prints "cluster gameplane-dev already exists — skipping create". `kind create` does not run, so nothing switches the context.
4. `:178` applies the `local-registry-hosting` ConfigMap to the other cluster. `:190` (`kubectl cluster-info --context kind-gameplane-dev`) passes because that context still exists. `:194-201` installs ingress-nginx there if it's missing. `:203-204` installs MetalLB and creates `IPAddressPool`/`L2Advertisement` objects there. `:210` creates `gameplane-system` there. `make dev-install` then Helm-installs Gameplane into the same cluster.
5. Variant: if `KUBECONFIG` points at a file without the kind context, `:190` aborts, but only after `:178` has already written to the other cluster.

**Expected:** Every `kubectl` and `helm` call in the kind path targets `kind-${CLUSTER}`, through `--context`/`--kube-context` or a `kubectl config use-context` in the existing-cluster branch. The header describes what a re-run actually does.

**Actual:** A re-run changes whichever cluster is current. Installing MetalLB into a cluster that already has a load-balancer implementation can disrupt that cluster's LoadBalancer Services.

### C-deploy-03

**Location:** `deploy/kind/e2e.sh:11` and `:165`.

**Repro / observation:**
1. `sed -n 11p deploy/kind/e2e.sh`: "Loads pre-built gameplane/{operator,api,agent}:<tag> images."
2. `sed -n 165,172p deploy/kind/e2e.sh`: the message says `gameplane/{operator,api,agent,sentinel,capture-sidecar}`, while the loop loads `gameplane-test/${img}:${TAG}` and builds any image that is missing ("pre-built" is not required).
3. `sed -n 177,191p deploy/kind/e2e.sh`: `gameplane-test/gameprobe` is loaded when present and `gameplane-test/fakeoidc` always (built if missing).

**Expected:** The header and the message name the `gameplane-test/` prefix and the full set of images (5 always, plus gameprobe when present, plus fakeoidc), and the header says missing images are built.

**Actual:** Both name the wrong prefix, and the header lists 3 of up to 7 images.

### C-deploy-04

**Location:** `deploy/kind/up.sh:147-152`.

**Repro / observation** (live on a dev box; affects only the local registry container):
1. `make dev-up` once, so the `kind-registry` container exists.
2. `docker stop kind-registry`.
3. `make dev-up` again. `docker inspect -f '{{.State.Running}}' kind-registry` prints `false`, so up.sh runs `docker run -d --restart=always -p 127.0.0.1:5001:5000 --name kind-registry registry:2`.
4. Docker fails with `Conflict. The container name "/kind-registry" is already in use by container ...`, and `set -euo pipefail` ends the script before the cluster step.

**Expected:** A stopped registry is restarted (`docker start kind-registry`), and a new one is created only when none exists. Alternatively, the script prints what to do.

**Actual:** The bootstrap aborts on the raw Docker name-conflict error.
