# Roadmap to v1

Gameplane is currently **`v0.2.0-beta.7`**. The CRDs, operator, API, agent, and
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

### Audit log integrity (tamper evidence) (PR #103, dashboard banner PR #108)

Complete end-to-end. Backend: migration `005_audit_chain.sql`,
`Auditor.insertChained`/`Auditor.Verify`, and `GET /admin/audit/verify`. The
dashboard integrity banner — surfacing a verification break to the admin on the
audit page — shipped in PR #108.

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

### Upgrade testing (`e2e upgrade` bucket)

`helm upgrade` was the most dangerous operation a user performs and the only
one CI never exercised. It now runs on every PR, on both amd64 and arm64:
`deploy/kind/upgrade.sh` installs the **previous release's published chart and
published GHCR images**, `TestUpgrade_FromPreviousRelease` seeds a running
GameServer with data plus an admin user, upgrades to the working-tree chart,
and asserts the CRD schema was updated, the server and its volume bytes
survived, and the pre-upgrade admin can still log in (proving migrations ran
against a populated SQLite volume).

The CRD assertion is self-calibrating: it computes which schema properties the
working tree declares that the installed release lacks, then requires the
pre-upgrade hook to have added exactly those — so it cannot rot into a vacuous
pass as the schema evolves.

This closes the CRD-upgrade caveat as a *tested* path rather than a documented
one. Note the caveat's own docs were stale and are corrected: the chart's
`crds.autoApply` pre-upgrade hook has been applying CRDs automatically for a
while, but [`install.md`](install.md#helm-crd-caveat) still told users to run
`kubectl apply` by hand.

Not yet covered: upgrades skipping several releases at once, and Postgres.

### Idle auto-sleep (backend PR #180, dashboard PR #182)

Opt-in per server (`spec.idle`): the operator scales a GameServer to zero once
it has reported no online players for `afterMinutes`, and brings it back on a
`wakeWindows` cron tick or an explicit `POST /servers/{name}:wake`. On a
single-node homelab the overnight reservation of several idle servers is the
dominant cost; this releases it without touching the data volume.

**Design (as implemented):**

- Sleeping drives the existing `softStop` path, so the module-declared stop
  sequence runs and the world saves before the pod goes away.
- The sleep marker is an operator-owned annotation, **not** `spec.suspend`.
  That field is the user's own power switch (the `:start`/`:stop` verbs patch
  it), so overloading it would make an automatic sleep indistinguishable from a
  deliberate stop — and a wake window would then resurrect a server its owner
  had turned off. Wake uses a request/ack token pair like the restart primitive.
- Phase reuses `Suspended`; the `IdleAsleep` condition reason, `status.idle`,
  and an `-o wide` print column distinguish it from a manual stop.
- A stale heartbeat or an absent player count both freeze the idle clock rather
  than reading as empty — unknown is not zero. Each refusal records a reason on
  `status.idle`, so a server that can never sleep explains itself.
- `gameplane_gameservers_idle{state=asleep|awake}` reports the saving.
- The dashboard (PR #182) renders a slept server as **Asleep** rather than
  `Suspended`, and replaces its Start button with **Wake**. That substitution
  is a bug fix, not decoration: an asleep server has `spec.suspend` still
  `false`, so the Start button the phase used to offer patched a field to the
  value it already held — a silent no-op that never woke anything. Only
  `:wake` clears the operator's marker.
- The server's Overview reports `status.idle.reason` verbatim, so a server
  that will never sleep says why instead of looking broken.

**Follow-up:** `status.idle.nextWakeTime`. The dashboard lists the configured
wake windows as raw cron strings, because the operator computes the next tick
only for its own requeue and never persists it — and re-deriving it in the
browser would duplicate scheduling semantics the operator owns (rule 10).
Persisting it on status is the right fix, and is a CRD change rather than a
dashboard one.

**Known gap:** no wake-on-connect. Waking needs the dashboard, the API, or a
wake window; a player cannot start a sleeping server by trying to join. Closing
it means a per-protocol listener holding the Service port while the pod is down,
across all four expose modes — a substantially larger piece of work, tracked
below rather than in this one.

### Read-only MCP server (PR #105)

An [MCP](https://modelcontextprotocol.io) server letting an AI assistant read
current cluster state and *propose* fixes — strictly read-only, no writes.

A new optional component (distroless Docker image, Helm toggle) exposing only
`List` / `Get` / `Watch` over the Gameplane CRDs plus Pods, Events, and pod logs.
"Propose a fix" returns suggested YAML or `kubectl` invocations as text — no
create/update/delete/patch tool exists. See [`mcp-server/README.md`](../mcp-server/README.md).

### Module signing: active for official bundles

The keyed-cosign signing mechanism is implemented and e2e-proven, and
`ModuleSource.spec.verify` can require a valid signature. It is now **active**
for the official bundles: `cosign.pub` is committed at the repo root (and baked
into the chart for module verification), and the `release` and
`republish-modules` workflows sign every published image and module bundle by
digest with the provisioned `COSIGN_PRIVATE_KEY`. See
[Signing official bundles](module-authoring.md#signing-official-bundles).

---

## Blocking v1

Nothing hard-blocks v1 anymore. What stands between beta and a v1 GA is the
production-readiness hardening below — tracked items, not code gaps.

---

## Wanted for v1, not blocking

### Production-readiness hardening

- A documented backup/restore **drill** — a runbook an operator can follow.
  The *coverage* half of this item is already done and was stale here:
  `TestRestore_RoundTrip` (`test/e2e/restore_e2e_test.go`) writes a marker into
  a server's PVC, backs it up, wipes it, restores, and asserts the bytes came
  back — against the real in-cluster restic-server that `ensureResticRepo`
  provisions. What is missing is the human-facing runbook, not the test.
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

### Wake-on-connect for idle auto-sleep

Idle auto-sleep ships without it, so a player who finds a server asleep cannot
start it by trying to join. The fix is a listener that holds the Service port
while the pod is down, recognizes a genuine connection attempt, flips the wake
request, and either holds or replays the handshake until the game is ready —
the `lazytainer` / "sleeping server starter" pattern.

It is a real component, not a tweak: it needs enough per-game protocol
awareness not to corrupt handshakes, and parity across all four `expose` modes
(ClusterIP, NodePort, LoadBalancer, Hostport). Until it exists, cron wake
windows are the answer for predictable play times.

### Postgres driver: make the store fully driver-portable

SQLite is the only production-tested driver. Postgres support (via build tag
`-tags postgres`) is work-in-progress: the SQL written in migrations lacks
portable placeholder rebinding (`?` → `$n`), and timestamp defaults are SQLite-specific.
Making it production-ready requires: portable SQL migration syntax, adding Postgres
to the CI coverage matrix, and e2e testing against a real Postgres instance.

---

## Explicitly out of scope for v1

- **Bot-testing every shipped game.** Only Minecraft and Terraria have practical
  headless protocol clients. Valheim and Palworld boot via multi-GB steamcmd
  downloads over proprietary UDP protocols, and Factorio's game traffic is
  UDP-only — none are bot-testable in CI.
- **A hosted/managed Gameplane.** The project targets self-hosted clusters, from
  a single-node k3s homelab to multi-node production.
