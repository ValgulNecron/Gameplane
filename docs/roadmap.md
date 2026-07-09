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

### Multi-cluster: dual-cluster e2e coverage

Nothing in CI exercises two clusters at once, so the `?cluster=` dispatch and the
cluster-scoped RBAC paths are only unit- and single-cluster-tested. v1 wants a
kind-based e2e bucket that stands up a second cluster, registers it, and asserts
that a GameServer created against it lands there and is invisible to a viewer
scoped to the first.

### Audit log integrity (tamper evidence)

The `audit_events` table is a plain, append-only-by-convention table. Anyone with
write access to the database can `UPDATE` or `DELETE` rows — including rows
outside the retention window — and nothing detects it. The optional stdout,
webhook, and S3 sinks mirror events but explicitly do not gate on delivery, so a
dropped mirror is indistinguishable from a suppressed event.

Planned design:

- Migration `005_audit_chain.sql` adding `prev_hash` + `hash` columns.
- On insert (inside a transaction, to serialize under concurrency) compute
  `hash = SHA-256(prev_hash || canonical(row))`.
- A `Verify()` that walks the chain and reports the first break, exposed as an
  admin-only `GET /admin/audit/verify` and surfaced as an integrity banner on the
  dashboard's audit page.
- Retention interacts badly with a naive chain: `Prune` deletes rows. Store a
  periodic **checkpoint** hash (in the `config` table) so verification resumes
  from the checkpoint rather than the genesis row.

Threat-model context lives in [`security.md`](security.md).

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

---

## Explicitly out of scope for v1

- **Bot-testing every shipped game.** Only Minecraft and Terraria have practical
  headless protocol clients. Valheim and Palworld boot via multi-GB steamcmd
  downloads over proprietary UDP protocols, and Factorio's game traffic is
  UDP-only — none are bot-testable in CI.
- **A hosted/managed Gameplane.** The project targets self-hosted clusters, from
  a single-node k3s homelab to multi-node production.
