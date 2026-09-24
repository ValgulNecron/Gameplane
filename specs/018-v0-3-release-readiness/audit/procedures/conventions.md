# Procedures: shared conventions

## Conventions (shared by every procedures file)

- **Cluster**: `export KUBECONFIG=~/kubelab.yaml`. Release `gameplane` in `gameplane-system`; games namespace `gameplane-games`.
- **Reaching the API** (pending OD-016): the ingress host `gameplane.local` does not resolve from the audit devbox. Until that is settled, run `kubectl port-forward -n gameplane-system svc/gameplane-web 18080:80` and use `GP=http://127.0.0.1:18080`. The ingress sends `/` to `gameplane-web`, which serves the dashboard and proxies API paths to `gameplane-api`.
- **Sessions**: `POST $GP/auth/login` with `Content-Type: application/json` and body `{"username":"…","password":"…"}`. The response sets `gameplane_session` (HttpOnly) and `gameplane_csrf` cookies. Both are `Secure`, so curl drops them over plain HTTP. Capture them from the response headers (`curl -D`) into `~/gameplane-audit-018/session-<role>.txt` (off-git, mode 600). Send them back as `-H "Cookie: gameplane_session=…; gameplane_csrf=…"`. Every mutating request also sends `-H "X-Gameplane-CSRF: <gameplane_csrf value>"`.
- **Accounts**: one session per role per round: `audit018-admin`, `audit018-operator`, `audit018-viewer`, and `audit018-collab` where a collaborator is needed. Admin access is pending OD-015.
- **Login budget**: IP burst 10 (5/min), user burst 6 (3/min). Each procedure states its login cost. Procedures that spend the budget on purpose (brute force, throttling) are marked **deferred to T031** and run last.
- **Naming and isolation**: every created object is named `audit018-<purpose>[-n]`, 63 characters at most. Objects created with `kubectl` also get the label `gameplane.io/audit: "018"`. Pre-existing objects are never written to. That covers GameServers `mc-fabric`, `soak-bogus-pool`, `soak-no-preference`, `soak-pool-west` and `squad`, and ModuleSources `default` and `uploads`. A procedure that needs an existing object creates its own `audit018-` copy first.
- **Evidence**: saved under `audit/evidence/<INV-ID>/`, redacted. A share token is written as `<token>`, and no cookie, CSRF value, password or secret is ever written. Each file stays under 1 MB.
- **Automatable?**: say `yes` or `no`. For `yes`, propose a bucket from `test/e2e/buckets.sh`: operator, api-auth, api-roles, api-rbac, api-agent, api-mods, ratelimit, bot-fast, bot-heavy, multicluster or upgrade.
