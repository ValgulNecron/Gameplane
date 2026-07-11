# Roadmap to v1

Gameplane is currently **`v0.2.0-beta.5`**. The CRDs, operator, API, agent, and
dashboard are feature-complete for the v1 scope and stabilized for external
testing, but the project is not yet recommended for unattended production. This
page tracks what stands between beta and a v1 GA release.

It is a living document: items move out of it as they ship, and anything
discovered along the way gets added. For what already works today, see
[`README.md`](../README.md) and [`architecture.md`](architecture.md).

---

## Shipped toward v1

These items have shipped and are reflected in current `main`.

### Multi-cluster: dashboard cluster selector (PR #107)

A Topbar cluster selector with per-cluster health, threading `?cluster=` through
the API client and raw-fetch escape hatches, with `queryClient.clear()` on cluster
switch. WebSocket streams remain local-cluster-scoped — a documented follow-up.

### Multi-cluster: dual-cluster e2e coverage (PR #104)

A real `kind ×2` e2e test (multicluster bucket) asserting `?cluster=` dispatch
lands a GameServer on cluster B and that a viewer scoped to cluster A cannot see it.

### Audit log integrity (tamper evidence) (PR #103, dashboard banner in feat/audit-integrity-banner)

Backend half shipped: migration `005_audit_chain.sql`, `Auditor.insertChained`/`Auditor.Verify`,
and `GET /admin/audit/verify`. Dashboard integrity banner — surfacing a verification break
to the admin on the audit page — lands in feat/audit-integrity-banner. Once that merges,
audit integrity is complete end-to-end.

**Design (as implemented):**

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

### Read-only MCP server (PR #105)

An [MCP](https://modelcontextprotocol.io) server letting an AI assistant read
current cluster state and *propose* fixes — strictly read-only, no writes.

A new optional component (distroless Docker image, Helm toggle) exposing only
`List` / `Get` / `Watch` over the Gameplane CRDs plus Pods, Events, and pod logs.
"Propose a fix" returns suggested YAML or `kubectl` invocations as text — no
create/update/delete/patch tool exists. See [`mcp-server/README.md`](../mcp-server/README.md).

---

## Blocking v1

The sole outstanding v1 blocker:

### Module signing: activate for official bundles

The keyed-cosign signing mechanism is implemented and e2e-proven, and
`ModuleSource.spec.verify` can already require a valid signature. It is not
*active* for the official bundles: a maintainer still has to generate the key
pair, commit `cosign.pub`, and provision `COSIGN_PRIVATE_KEY` / `COSIGN_PASSWORD`
as CI secrets. See
[Signing official bundles](module-authoring.md#signing-official-bundles).

---

## Wanted for v1, not blocking

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
