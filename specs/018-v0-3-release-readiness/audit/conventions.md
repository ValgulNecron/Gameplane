# Conventions (shared by every procedures file)

## Cluster

Export the kubelab kubeconfig:
```sh
export KUBECONFIG=~/kubelab.yaml
```

The Gameplane release is in namespace `gameplane-system`, named `gameplane`. Game servers are in namespace `gameplane-games`.

## Reaching the API

**Pending OD-016:** The ingress host `gameplane.local` does not resolve from the audit devbox. Until that is settled, run:
```sh
kubectl port-forward -n gameplane-system svc/gameplane-web 18080:80
```

Use `GP=http://127.0.0.1:18080` for all API requests. The ingress sends `/` to `gameplane-web`, which serves the dashboard and proxies API paths to `gameplane-api`.

## Sessions

Create a session with a POST to `$GP/auth/login`:
```sh
curl -s -D headers.txt -H "Content-Type: application/json" \
  -d '{"username":"audit018-admin","password":"<password>"}' \
  $GP/auth/login
```

The response sets `gameplane_session` and `gameplane_csrf` cookies (both `HttpOnly` and `Secure`). Capture them:
```sh
grep -i "^set-cookie:" headers.txt | grep gameplane_session > ~/gameplane-audit-018/session-<role>.txt
grep -i "^set-cookie:" headers.txt | grep gameplane_csrf >> ~/gameplane-audit-018/session-<role>.txt
chmod 600 ~/gameplane-audit-018/session-<role>.txt
```

Over plain HTTP, `curl` drops `Secure` cookies. Instead, extract and send them back:
```sh
SESS=$(grep gameplane_session ~/gameplane-audit-018/session-<role>.txt | cut -d'=' -f2 | cut -d';' -f1)
CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-<role>.txt | cut -d'=' -f2 | cut -d';' -f1)
curl -H "Cookie: gameplane_session=$SESS; gameplane_csrf=$CSRF" \
  -H "X-Gameplane-CSRF: $CSRF" \
  $GP/api/path
```

Every mutating request also includes `-H "X-Gameplane-CSRF: <gameplane_csrf value>"`.

## Accounts

One session per role per round:
- `audit018-admin`: full access
- `audit018-operator`: operator access
- `audit018-viewer`: read-only access
- `audit018-collab`: collaborator access (when needed)

Admin access is pending OD-015.

## Login budget

Rate limits per cluster:
- IP burst 10 (5/min)
- User burst 6 (3/min)

Each procedure states its login cost. Procedures that spend the budget on purpose (brute force, throttling tests) are marked **deferred to T031** and run last.

## Naming and isolation

Every created object is named `audit018-<purpose>[-n]`, within Kubernetes' 63-character label limit.

Objects created with `kubectl` also carry the label:
```sh
kubectl apply -f <file> -l gameplane.io/audit=018 -n gameplane-games
```

Pre-existing objects are **never** written to. This covers:
- GameServers: `mc-fabric`, `soak-bogus-pool`, `soak-no-preference`, `soak-pool-west`, `squad`
- ModuleSources: `default`, `uploads`

A procedure that needs an existing object creates its own `audit018-` copy first.

## Evidence

Save all evidence under `audit/evidence/<INV-ID>/`, redacted:
- Share tokens as `<token>` (not the actual token)
- No cookies, CSRF values, passwords or secrets
- Each file stays under 1 MB

## Automatable?

Each procedure states `yes` or `no`. For `yes`, propose a bucket from `test/e2e/buckets.sh`:
- `operator`, `api-auth`, `api-roles`, `api-rbac`, `api-agent`, `api-mods`, `ratelimit`, `bot-fast`, `bot-heavy`, `multicluster`, `upgrade`
