# Roadmap to v1

Gameplane is currently **`v0.2.0-beta.5`**. The CRDs, operator, API, agent, and
dashboard are feature-complete for the v1 scope and stabilized for external
testing, but the project is not yet recommended for unattended production. This
page tracks what stands between beta and a v1 GA release.

It is a living document: items move out of it as they ship, and anything
discovered along the way gets added. For what already works today, see
[`README.md`](../README.md) and [`architecture.md`](architecture.md).

---

## Blocking v1

These are the items a v1 GA should not ship without.

### Multi-cluster: dashboard cluster selector

The control plane is already multi-cluster: there is a `Cluster` CRD, an
operator health reconciler, a `/clusters` CRUD surface, a `?cluster=` dispatch
layer through the API, and the RBAC model carries a cluster dimension. See
[Registering an additional cluster](install.md#registering-an-additional-cluster).

What's missing is the last mile — the dashboard has no cluster picker, so every
page implicitly targets the local cluster. Remaining work:

- A cluster selector in the app shell, persisted across navigation, threading
  `?cluster=` into the existing API client.
- Per-cluster health/status surfacing (the reconciler already writes it).
- Empty/unreachable-cluster states.

Per the repo's design-first rule this starts in `design.pen`, not in React.

**Note on WebSocket streams:** Console, RCON, and log WebSocket streams remain
local-cluster-scoped in this release. The dashboard's API client threads `?cluster=`
through REST fetches, but WebSocket paths have not yet been updated. Cross-cluster
WebSocket support is a follow-up task.

### Multi-cluster: dual-cluster e2e coverage

Nothing in CI exercises two clusters at once, so the `?cluster=` dispatch and the
cluster-scoped RBAC paths are only unit- and single-cluster-tested. v1 wants a
kind-based e2e bucket that stands up a second cluster, registers it, and asserts
that a GameServer created against it lands there and is invisible to a viewer
scoped to the first.

### Audit log integrity (tamper evidence) — backend shipped, dashboard banner outstanding

The `audit_events` table is a plain, append-only-by-convention table. Anyone with
write access to the database can `UPDATE` or `DELETE` rows — including rows
outside the retention window. The optional stdout, webhook, and S3 sinks mirror
events but explicitly do not gate on delivery, so a dropped mirror is
indistinguishable from a suppressed event.

The backend half shipped per the design below (migration `005_audit_chain.sql`,
`Auditor.insertChained`/`Auditor.Verify`, `GET /admin/audit/verify`). Outstanding:
a dashboard integrity banner on the audit page that calls the verify endpoint
and surfaces a break to the admin — tracked as a separate follow-up
(design-first in `design.pen` per repo rule 1).

Design (as implemented):

- Migration `005_audit_chain.sql` added `prev_hash` + `hash` columns.
- On insert (inside a transaction, serialized per-process by a mutex — see the
  single-writer caveat on `insertChained`) compute
  `hash = SHA-256(prev_hash || canonical(row))`, and upsert `audit.head`
  (the newest row's id + hash) in the same transaction.
- `Verify()` walks the chain and reports the first break, exposed as an
  admin-only `GET /admin/audit/verify`. It also checks `audit.head` against
  the newest surviving row, so truncating the tail (`DELETE ... WHERE id > N`,
  which leaves every surviving link internally consistent) is detected too —
  previously a known blind spot.
- `Prune` writes a checkpoint (the newest about-to-be-deleted row's id + hash,
  in the `config` table under `audit.checkpoint`) before deleting, so
  verification resumes from the checkpoint rather than the genesis row.

**What this does and does not guarantee:** the chain is unkeyed, and the
checkpoint/head it verifies against live in the same DB-writable `config`
table. It reliably catches naive in-DB tampering (`UPDATE`/`DELETE`, including
tail truncation) and accidental corruption — it does not, and cannot, defeat a
sophisticated attacker with DB write access who also recomputes and rewrites
the checkpoint/head to match. The external sinks (stdout/webhook/S3) remain the
append-only record of last resort such an attacker cannot retroactively alter.
An HMAC-keyed chain (key held outside the database) would close that
remaining gap and is a candidate future hardening, not yet implemented.

Threat-model context lives in [`security.md`](security.md#audit-log-integrity).

### Module signing: activate for official bundles

The keyed-cosign signing mechanism is implemented and e2e-proven, and
`ModuleSource.spec.verify` can already require a valid signature. It is not
*active* for the official bundles: a maintainer still has to generate the key
pair, commit `cosign.pub`, and provision `COSIGN_PRIVATE_KEY` / `COSIGN_PASSWORD`
as CI secrets. See
[Signing official bundles](module-authoring.md#signing-official-bundles).

---

## Wanted for v1, not blocking

### Read-only MCP server

An [MCP](https://modelcontextprotocol.io) server that lets an AI assistant read
current cluster state and *propose* fixes — strictly read-only, no writes.

Shape: a new optional component alongside `audit-syslog-bridge/` and
`telemetry-receiver/` (its own `go.mod`, a distroless `Dockerfile`, a Helm
toggle), acquiring a client the same way the API does (`ctrl.GetConfig()` →
typed + dynamic clientsets). It would expose only `List` / `Get` / `Watch` over
the Gameplane CRDs plus Pods, Events, and pod logs. "Propose a fix" means
returning suggested YAML or `kubectl` invocations as text — the server never
applies anything, and no create/update/delete/patch tool exists.

Note that `api/internal/kube` is `internal` to the `api` module, so this either
copies a thin read-only client or that package gets promoted to a shared one.

### Module-authored categories for the official modules

`GameTemplate.spec.category` and the module-catalog `category` field exist, and
the dashboard builds its filter chips from the distinct values actually present.
The five official modules in the
[`gameplane-module`](https://github.com/ValgulNecron/gameplane-module) repo don't
declare a `category` yet, so they currently group via the frontend's heuristic
fallback. Declaring `category:` in each `module.yaml` + `template.yaml` (and
bumping the submodule pointer) makes the grouping author-owned end to end.

### Production-readiness hardening

- A documented backup/restore drill, and restore-path coverage against a real
  restic repository.
- Upgrade testing across at least one minor version, including the CRD-upgrade
  caveat (Helm never updates CRDs on `helm upgrade`; see
  [Helm CRD caveat](install.md#helm-crd-caveat)).
- Resource-limit guidance sized from real workloads rather than defaults.
- Hermetic module images: the shipped game modules pin floating upstream tags
  (`terraria-latest`, `tmodloader-latest`), meaning a server binary can change
  underneath a user on pod restart with no version bump or changelog. This is not
  hypothetical: `terraria-latest` moved from Terraria 1.4.4.9 to 1.4.5.6,
  changing the network protocol version and breaking the e2e Terraria bot. The
  fix is to pin explicit image tags for the default image, and keep a floating
  "latest" entry only in `spec.versions` where drift is the user's explicit,
  labelled choice. Changes live in the `gameplane-module` repo with a submodule
  pointer bump here.

---

## Explicitly out of scope for v1

- **Bot-testing every shipped game.** Only Minecraft and Terraria have practical
  headless protocol clients. Valheim and Palworld boot via multi-GB steamcmd
  downloads over proprietary UDP protocols, and Factorio's game traffic is
  UDP-only — none are bot-testable in CI.
- **A hosted/managed Gameplane.** The project targets self-hosted clusters, from
  a single-node k3s homelab to multi-node production.
