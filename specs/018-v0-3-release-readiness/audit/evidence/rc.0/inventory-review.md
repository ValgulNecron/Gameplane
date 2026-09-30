# T024 — Inventory completeness review (tier-up, sonnet)

Reviews `audit/inventory.md` against the code it claims to describe:
`web/src/router/tree.tsx` + `web/src/routes/`, `api/cmd/main.go` (+ handler `Mount*`
functions), `operator/api/v1alpha1/` + `operator/internal/controller/`,
`agent/internal/` + `agent/cmd/main.go`, `charts/gameplane/values.yaml`, and
`audit/evidence/rc.0/module-categories.md`.

Method: every `Source` cell was machine-parsed and checked against the file it
names (existence + line count), every `[p](procedures/x.md#slug)` anchor was
checked against that file's headings, and the flagged rows plus the explicitly
named code paths were read by hand to confirm or refute the row's claim.

Row counts before this review: WEB 140, API 146, CRD 35, AGT 36, AUX 18,
HELM 28, MOD 68, UPG 5, NODE 3, SEC 16 (total 495).
Row counts after: **WEB 145, API 146, CRD 37, AGT 36, AUX 18, HELM 28, MOD 68,
UPG 5, NODE 3, SEC 16 (total 502)**.

---

## Missed capabilities added (7)

| ID | Capability | Source | Procedure added |
|----|------------|--------|------------------|
| INV-CRD-036 | Create backup using VolumeSnapshot strategy (`spec.strategy=volume-snapshot`) | `operator/internal/controller/backup_volumesnapshot.go:26-68` | `procedures/crd.md#backup-volumesnapshot-strategy` |
| INV-CRD-037 | Restore from a VolumeSnapshot-strategy backup | `operator/internal/controller/restore_volumesnapshot.go:24-103` | `procedures/crd.md#restore-volumesnapshot-strategy` |
| INV-WEB-141 | Select game template in create-server wizard | `web/src/routes/CreateServer.tsx:541-645` | `procedures/web.md#create-server-wizard-pick-template` |
| INV-WEB-142 | Select template version in create-server wizard | `web/src/routes/CreateServer.tsx:646-684` | `procedures/web.md#create-server-wizard-pick-version` |
| INV-WEB-143 | Configure server name, resources, and labels in create-server wizard | `web/src/routes/CreateServer.tsx:685-823` | `procedures/web.md#create-server-wizard-configure` |
| INV-WEB-144 | Configure networking (exposure, ports, tunnel) in create-server wizard | `web/src/routes/CreateServer.tsx:824-1086` | `procedures/web.md#create-server-wizard-network` |
| INV-WEB-145 | Review and submit new server from create-server wizard | `web/src/routes/CreateServer.tsx:348-500` | `procedures/web.md#create-server-wizard-review-create` |

**Why these were missed:**
- `operator/api/v1alpha1/backup_types.go` declares `Backup.spec.strategy` with
  two values (`restic-snapshot` default, `volume-snapshot`), each with its own
  reconciler file (`backup_volumesnapshot.go`, 145 lines;
  `restore_volumesnapshot.go`, 128 lines). The original CRD-015..020 rows only
  ever exercise the restic path (`backup_controller.go`,
  `restore_controller.go`); the volume-snapshot reconcile functions had no row
  at all. CI backing: `TestBackup_VolumeSnapshotSucceeds`,
  `TestRestore_VolumeSnapshotProvisionsNewServer`
  (`operator/internal/controller/backup_volumesnapshot_envtest_test.go`).
- `web/src/router/tree.tsx` registers `/servers/new` → `CreateServerWizard`
  from `web/src/routes/CreateServer.tsx` (1312 lines: a 4-5 step wizard —
  `PickTemplate`, `PickVersion`, `Configure`, `Network`, review/create). The
  existing WEB rows only cover the *button* that navigates there
  (`INV-WEB-010`, `dashboard-create-server-button`, which already says
  "Cleanup: Cancel the wizard without creating a server" — i.e. the wizard's
  own screen was known to exist but never got rows). CI backing for the final
  submit step: `TestAPI_LifecycleClone` (same as `INV-WEB-010`); the four
  UI-only steps carry CI test `none`, matching the convention used elsewhere
  in WEB for filter/browse/step-only capabilities.

All seven new rows: `Outcome = untested`, appended at the end of their area's
table, same column order as existing rows.

---

## Invented rows withdrawn (1)

| ID | Capability | Old Source | Reason |
|----|------------|-----------|--------|
| INV-API-067 | "Get user" | `api/internal/handlers/users.go:35` | **No `GET /users/{id}` route exists.** `MountUsers` in `users.go` registers exactly 12 routes on `/users`; line 35 is `r.Delete("/{id}", h.del)`. `userHandler` has no `get` method (only `list`, `create`, `me`, `getPreferences`, `putPreferences`, `resetPreferences`, `del`, `update`, `resetPassword`, `listBindings`, `addBinding`, `deleteBinding`, plus the internal `fetchByID` used by `update`'s response). Row left in place per contract, `Outcome` set to `n/a`, `Reason` set to `withdrawn at T024: no GET /users/{id} route exists in MountUsers; line 35 is` `` `r.Delete("/{id}", h.del)` ``, and no `get` method exists on userHandler`. |

Fetching a single user by id is not a capability of this API — list (`INV-API-061`),
`/me` (`INV-API-063`), and update (`INV-API-068`, which returns the row via
`fetchByID` after a `PATCH`) already cover the code that exists.

---

## Wrong Source lines fixed in place (51)

All confirmed by reading the named file at both the old and the corrected line
number. Root cause, where visible in git blame context: unrelated edits (added
doc comments, an `agentCABundle` parameter, etc.) shifted line numbers in the
named file after the row was written, and the row was never re-synced.

**AUX / UPG — missing subdirectory prefix (path pointed at repo root, not the module dir):**
- INV-AUX-005: `internal/capture/afpacket.go` → `capture-sidecar/internal/capture/afpacket.go`
- INV-AUX-012: `main.go:1-30` → `audit-syslog-bridge/main.go:1-30`
- INV-AUX-016: `main.go` → `mcp-server/main.go`
- INV-AUX-018: `internal/kube/client.go` → `mcp-server/internal/kube/client.go`
- INV-UPG-002: `api/internal/handlers/auth.go:1` (file does not exist) → `api/internal/auth/local.go:105-173` (the actual `HandleLogin`, same function `INV-SEC-001` already cites)
- INV-UPG-005: `api/internal/db/migrations/:1` (a directory, not a file) → `api/internal/db/db.go:70-117` (`Store.Migrate` / `runMigration`)

**API — off-by-N line drift (40 rows):**
| ID | Old | New |
|----|-----|-----|
| INV-API-007 | `main.go:278` | `main.go:278-279` (multi-line `.With(...).Get(...)` chain) |
| INV-API-009 | `main.go:284` | `main.go:284-285` |
| INV-API-034 | `auth_provider_secret.go:25` | `:26` |
| INV-API-035 | `auth_provider_secret.go:26` | `:27` |
| INV-API-036 | `notifications.go:31` | `:33-34` |
| INV-API-037 | `notifications.go:32` | `:50` |
| INV-API-038 | `notifications.go:33` | `:51` |
| INV-API-039 | `registry_secret.go:22` | `:24` |
| INV-API-040 | `registry_secret.go:23` | `:25` |
| INV-API-041..048 | `capture.go:57..64` | `:67..74` (`+10`, an `agentCABundle` block was added ahead of the route block) |
| INV-API-058 | `ownership.go:57` | `:58` |
| INV-API-059 | `ownership.go:58` | `:59` |
| INV-API-060 | `ownership.go:59` | `:60` |
| INV-API-082 | `cluster_actions.go:39` | `:40` |
| INV-API-083 | `cluster_actions.go:40` | `:41` |
| INV-API-084 | `clusters.go:27-29` | `:27-28` (tightened to the `list` route only) |
| INV-API-085 | `clusters.go:28` (was `list`) | `:29` (`create`) |
| INV-API-086 | `clusters.go:29` (was `create`) | `:30` (`delete`) |
| INV-API-104..108 | `registry.go:44..48` | `:43..47` (`-1`) |
| INV-API-109 | `mod_updates.go:32` | `:40` |
| INV-API-110 | `mod_ids.go:63` | `:64` |
| INV-API-111 | `mod_ids.go:64` | `:65` |
| INV-API-112 | `tunnelcreds.go:32` (was `put`) | `:33` (`get`) |
| INV-API-113 | `tunnelcreds.go:31` (struct literal) | `:32` (`put`) |
| INV-API-114 | `tunnelcreds.go:33` (was `get`) | `:34` (`delete`) |
| INV-API-115 | `destinations.go:44-47` | `:42-45` |
| INV-API-116 | `destinations.go:45` | `:43` |
| INV-API-117 | `destinations.go:46` | `:44` |
| INV-API-118 | `destinations.go:47` | `:45` |
| INV-API-120 | `pod_events.go:26` (func decl) | `:27` (the route) |

Two of these (`INV-API-085`/`086` and `INV-API-112`/`113`) were not just off by
one — the old line numbers pointed at the *wrong route* entirely (e.g.
`INV-API-085` "Register remote cluster" pointed at `r.Get("/", h.list)`, and
`INV-API-113` "Set tunnel credentials" pointed at the struct literal, one line
above the actual `r.Put(...)`).

**AGT — 10 rows:**
| ID | Old | New |
|----|-----|-----|
| INV-AGT-001 | `rcon.go:128` (a closing `}`) | `rcon.go:122-188` (the actual `Exec` method) |
| INV-AGT-009 | `attach.go:41` (func decl) | `attach.go:41-46` (adds the actual `r.Get(...)` route line) |
| INV-AGT-019..026 | `players.go:78..85` | `players.go:97..104` |

`players.go:78-85` was the constructor body that sets `h.listCmd` from
`actions.List` — unrelated to any of the 8 routes it was cited for. The real
`Mount` block (`r.Get("/players", ...)` through
`r.Post("/players/whitelist/remove", ...)`) is at lines 97-104.

---

## Broken procedure anchors

**None found.** Every `[p](procedures/<area>.md#<slug>)` in the (now 502-row)
inventory resolves to a heading with that exact slug in that file — checked
by slugifying every `##`/`###` heading in all 10 procedures files and
diffing against every anchor reference (script-verified, not sampled).

Two **non-blocking** conventions notes, left as-is (out of this task's fix
list, called out below under Problems):
1. `procedures/agent.md`, `crd.md`, `modules.md`, `nodes.md`, `upgrade.md` use
   `##` for every procedure heading; `api.md`, `aux.md`, `helm.md`,
   `security.md`, `web.md` use `###`, matching
   `contracts/audit-records.md`'s "One anchor-addressable `### <slug>`" rule.
   Anchors resolve identically either way (GitHub slug generation ignores
   heading level), so no link is actually broken, but the five `##`-only
   files are non-compliant with the contract's literal heading level.
2. The two new CRD procedures and five new WEB procedures added by this
   review use `###`, per the contract and per this task's own instructions —
   in `crd.md` that is one level deeper than that file's other 33 procedures
   (also `##`), so the file is now internally mixed until someone normalizes
   the other 33.

---

## Problems found but not fixed (out of this task's scope)

1. **`## UPG` and `## NODE` sections are structurally empty; their rows live inside `## SEC`.**
   `inventory.md` lines 525-536 show `## UPG` and `## NODE` each followed only
   by a duplicated header/separator pair and no data rows. All 5 `INV-UPG-*`
   rows and all 3 `INV-NODE-*` rows are physically located inside the `## SEC`
   table (interleaved between `INV-SEC-006`/`INV-SEC-007` and between
   `INV-SEC-011`/`INV-SEC-012` respectively), so a tool or reviewer that reads
   "rows under the `## UPG` heading" literally finds zero. This predates T024
   (visible in the version this review started from) and is a real defect,
   but it is a table-restructuring job — cutting 8 rows out of the `## SEC`
   table, removing 2 duplicate empty header/separator pairs, and reinserting
   the rows under their own headings — which is broader than the "fix wrong
   Source lines / broken anchors, add missed rows at the end, withdraw
   invented rows" scope this task was given, and touching that much table
   structure without sign-off risks losing/misordering rows. Left in place;
   flagged here for a maintainer or a dedicated follow-up task. The **counts
   below use each row's `INV-<AREA>-NNN` prefix** (i.e. logically, not
   physically, grouped), since that is what every other tool in this repo
   (including the script used to produce these counts) keys off.
2. Heading-level inconsistency across procedures files (`##` vs `###`) — see
   above. Not fixed because normalizing 5 whole files' heading levels (~90
   headings) is a bulk mechanical edit outside this row-level review's remit,
   and it changes nothing functionally (no anchor is actually broken).
3. `INV-SEC-012`'s `verifyHead` and `INV-SEC-014`'s `audit redaction` Source
   annotations are not file paths (they don't resolve as files) but they also
   aren't wrong — they're a descriptive second clause after the real file
   path, consistent with how `INV-AUX-003`/`004` and others append plain-text
   context. Left unchanged.
4. `INV-UPG-001`, `003`, `004` still cite generic `:1` line anchors
   (`api/cmd/main.go:1`, etc.) as a stand-in for "the whole binary changed
   version" rather than a specific function. These aren't wrong (line 1 of
   every one of those files exists), just very coarse; left alone since the
   capability being tested (behavior across an upgrade) genuinely isn't
   localized to one function.

---

## Areas checked with no findings

- `api/internal/handlers/resources.go`'s shared `mountOne` (lines 53-58) is
  legitimately reused by `INV-API-012..026` (servers/templates/backups/
  schedules/restores all route through the same generic CRUD handler,
  confirmed against `api/internal/kube/client.go`'s `GVRs` map) — not a copy-paste bug.
- `api/internal/handlers/modules.go` (17 routes), `registry.go` (5, after the
  fix above), `capture.go` (8, after the fix above), `clusters.go` (3, after
  the fix above), `cluster.go` (3), `roles.go` (5), `users.go` (12, minus the
  withdrawn "get user"), `destinations.go` (4), `tunnelcreds.go` (3),
  `pod_events.go` (1), `mod_ids.go` (2), `mod_updates.go` (1) — each Mount
  function's route count now matches its inventory row count exactly.
- `agent/internal/files/files.go` (list/read/write/upload/download/mkdir/delete,
  lines 37-43) — all 7 `INV-AGT-010..016` lines were already correct.
- `charts/gameplane/values.yaml` top-level keys (`image`, `crds`, `operator`,
  `api`, `web`, `ingress`, `networkPolicies`, `clusterOps`, `mcpServer`,
  `serviceMonitors`, `prometheusRules`, `grafanaDashboards`, `podSecurity`,
  `defaultModuleSource`, `uploadModuleSource`, `capture`) all have at least
  one HELM row; `updates.channel` (a single string field, line 415) does not,
  but it's a thin passthrough with no distinct reconcile behavior to test
  beyond what `INV-WEB-105` (View update availability) already covers from
  the UI side — judged not worth a new row.
- `module-categories.md`'s 17 representative categories × 4 capability rows =
  68 = the full `MOD` section; line numbers 48-64 in that file map 1:1 to the
  `INV-MOD-*` `Source` column (confirmed by reading the category table).
- `operator/api/v1alpha1/gametemplate_types.go` +
  `gametemplate_controller.go`: the controller only maintains
  `status.inUseCount`; the single existing `INV-CRD-014` row covers it fully.
