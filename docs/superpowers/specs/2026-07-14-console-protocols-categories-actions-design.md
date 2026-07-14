# Console protocols, multi-category modules, and richer quick actions

**Date:** 2026-07-14
**Status:** approved, not yet implemented

## Problem

Three requests, which turn out to be one dependency chain:

1. Support more remote-console ("RCON") protocols.
2. Let a module belong to several categories, not one.
3. Add more quick actions to every module — expand the modules that have some, add them to the modules that have none.

The chain: quick actions are declared under `capabilities.actions` and run over the
template's RCON connection, so they require `spec.rcon.protocol != none`. Of the 16
official modules, only 7 have RCON at all. The other 9 *structurally cannot* carry a
quick action today. "More actions everywhere" is therefore gated on "more protocols",
and the two must be designed together.

### Current state

RCON is already pluggable. `agent/internal/rcon/` implements two protocols behind a
single-method interface:

```go
type Rcon interface { Exec(cmd string) (string, error) }
```

`source` (Valve packet framing) and `telnet` (line-based) both satisfy it, along with a
`Disabled` no-op. Every consumer — console, players, quiesce, lifecycle, status, actions,
heartbeat — depends only on that interface. The template selects a protocol via
`spec.rcon.protocol` (enum `source;telnet;none`); the operator resolves it
(`rconProtocol()` in `operator/internal/controller/helpers.go`) and passes it to the agent
as `GAMEPLANE_RCON_PROTOCOL`, which the agent switches on in `agent/cmd/main.go`.

Adding a protocol is consequently: one new file in `agent/internal/rcon/`, one enum value,
one `switch` case. No downstream consumer changes.

Module inventory as of this spec:

| | Modules |
|---|---|
| RCON `source` | minecraft-java, factorio, palworld, project-zomboid, cs2, v-rising, ark-survival-ascended |
| RCON `telnet` | **none** — the client exists but no module references it |
| No console protocol | 7-days-to-die, dayz, dont-starve-together, enshrouded, garrys-mod, rust, satisfactory, terraria, valheim |
| Has quick actions | minecraft-java (6), ark (2), project-zomboid (2), palworld (1), cs2 (1), v-rising (1) |
| Has RCON, zero actions | factorio |

Two of the nine "no protocol" modules look like misconfiguration rather than a missing
protocol: **garrys-mod** is an srcds server and has spoken Source RCON all along, and
**7-days-to-die** could use the telnet client that already exists (its template notes it is
"blocked by password management").

Categories are a single `string` on `GameTemplateSpec`, `ModuleEntry`, and the bundle
`Metadata`. **No module declares one** — all 16 fall back to a regex heuristic
(`gameCategory()` in `web/src/lib/games.ts`) that guesses Survival/Sandbox/Shooter from
the game slug. The field is effectively unused, which makes changing it cheap.

## Design

### 1. Console protocols

Three new clients in `agent/internal/rcon/`, each implementing `Exec`. The CRD enum
`spec.rcon.protocol` becomes `source;telnet;websocket;battleye;satisfactory;none`, and
`agent/cmd/main.go` gains three `switch` cases.

| Client | Wire protocol | Unlocks |
|---|---|---|
| `websocket.go` | Facepunch WebRcon: JSON `{Identifier, Message, Name}` over `ws://host:port/<password>`. Replies correlate by `Identifier`; unsolicited console output arrives on id 0 and must be filtered out of command replies. | rust |
| `battleye.go` | BattlEye RCon: UDP, `BE` magic + CRC32 + type byte. Login / command / server-message packet types, sequenced multi-page replies, and a mandatory keepalive goroutine — the server drops the session after roughly 45s of silence. | dayz |
| `satisfactory.go` | Satisfactory's official HTTPS API: a password login yields a bearer token, then `POST /api/v1` with `{"function": …, "data": …}` against a self-signed certificate on localhost. | satisfactory |

`coder/websocket` is already a direct dependency of the agent module (it serves the console
WebSocket), so the Rust client adds no new Go dependency.

**Naming.** The third client is `satisfactory`, not `http`. It is not a generic HTTP
console — it is a function-call API with a bespoke login handshake, and no other game
shares its shape. Naming it `http` would promise a genericity we do not have.

**Correction (2026-07-14, after protocol research).** This spec originally assumed
Satisfactory's API exposed no arbitrary-console-command endpoint, and proposed mapping
`Exec` onto it by treating a command line as `FunctionName {json-args}`. That was wrong.
The API **does** expose `RunCommand`, which takes a free-text console command and returns
its output:

```
POST /api/v1   Authorization: Bearer <token>
{"function": "RunCommand", "data": {"command": "<cmd>"}}
  → {"data": {"CommandResult": "<output>"}}
```

So `Exec(cmd)` maps onto `RunCommand` directly, with exactly the same semantics as the
other four protocols, and the `FunctionName {json-args}` hack is dropped. The named
functions (`SaveGame`, `Shutdown`, `QueryServerState`, …) remain available and are the
right choice where one exists, but the console path is now honest.

**Known limitation:** Satisfactory's API cannot enumerate players. `QueryServerState`
returns `NumConnectedPlayers` (a count) and there is no documented endpoint for names or
IDs. So satisfactory gets a console and quick actions, but **not** the
`capabilities.players` list — its template must leave that unset rather than ship a
players tab that cannot populate.

The CRD doc comment on `RCONSpec.Protocol` currently states there is no generic HTTP-console
implementation; it is rewritten to describe the five protocols.

**RCON dials the game container over pod-local loopback**, so `netguard` is not in the path
for any of these clients — its dial guard governs ModuleSource fetches and mod downloads,
not console connections.

**Free fixes** (no new code, one small PR):

- **garrys-mod** → `protocol: source`, port 27015.
- **7-days-to-die** → `protocol: telnet`, port 8081.

Both depend on the game image exposing a password knob (`passwordEnv` or `passwordFile`).
Each is verified against the real image before its module ships; if an image turns out not
to expose one, that module stays `none` and the finding is recorded in its template comment.

### 2. Multi-category modules

`category: string` becomes `categories: []string`.

| Layer | Change |
|---|---|
| `operator/api/v1alpha1/gametemplate_types.go` | `Category string` → `Categories []string`, `MaxItems=8`, each `MaxLength=32` |
| `operator/api/v1alpha1/modulesource_types.go` | same, on `ModuleEntry` |
| `operator/internal/modsrc/bundle.go` | `Metadata.Categories []string`; the YAML parser **also accepts a legacy scalar** `category: Sandbox` and normalizes it to `["Sandbox"]`, so third-party bundles do not break |
| `operator/internal/modsrc/{oci,dir,upload}.go` | assign the list through to `ModuleEntry` |
| `api/internal/handlers/modules.go` | `CatalogEntry.Categories []string`; the multi-source merge changes from "first non-empty wins" to a case-insensitively deduped **union** |
| `web/src/types.ts` | `categories?: string[]` on `GameTemplate` and `CatalogEntry` |
| `web/src/lib/games.ts` | `resolveCategories(explicit, game): string[]`, falling back to `[gameCategory(game)]` when a module declares nothing |
| `web/src/routes/Modules.tsx`, `CreateServer.tsx` | chips built from the flattened union; a module matches a chip if **any** of its categories match |

Values stay **free-form** in the CRD. The current design comment promises that a module can
coin a new category simply by naming it, with no frontend change; a kubebuilder enum would
break that promise and force a CRD change per genre. Instead `docs/module-authoring.md`
publishes a canonical vocabulary that the official modules adhere to:

**Survival · Sandbox · Shooter · Simulation · Building · Adventure · Horror · Co-op · PvP · Modded · Creative**

Filter chips dedupe case-insensitively so `Survival` and `survival` do not both appear.

Assignments for the 16 official modules:

| Module | Categories |
|---|---|
| minecraft-java | Sandbox, Survival, Building, Modded, Creative |
| valheim | Survival, Co-op, Building |
| terraria | Sandbox, Survival, Adventure, Modded |
| factorio | Simulation, Building, Sandbox, Modded, Co-op |
| palworld | Survival, Sandbox, Co-op |
| 7-days-to-die | Survival, Horror, Co-op, Shooter |
| rust | Survival, Shooter, PvP |
| dayz | Survival, Shooter, PvP, Horror |
| dont-starve-together | Survival, Co-op |
| garrys-mod | Sandbox, Modded, Creative |
| satisfactory | Simulation, Building, Sandbox, Co-op |
| enshrouded | Survival, Co-op, Building, Adventure |
| cs2 | Shooter, PvP |
| project-zomboid | Survival, Horror, Co-op |
| v-rising | Survival, PvP, Co-op |
| ark-survival-ascended | Survival, PvP, Co-op, Modded |

This is a CRD type edit: `make generate && make manifests`, with the regenerated
`zz_generated.deepcopy.go`, `operator/config/crd/*.yaml`, `operator/config/rbac/*.yaml` and
`charts/gameplane/crds/*.yaml` committed alongside the source change (CLAUDE.md rule 7).

A module card that previously showed at most one chip may now show up to five, so the
Modules page needs a `design.pen` pass for chip overflow (show two, `+3`) before the React
changes.

### 3. Action schema and the stdin transport

#### Schema

Three fields added to `ServerActionSpec`:

```go
// Commands runs several console commands in order (mutually exclusive
// with Command). Mirrors capabilities.quiesce / capabilities.lifecycle.stop,
// which already express sequences as a plain []string.
// +kubebuilder:validation:MaxItems=16
// +optional
Commands []string `json:"commands,omitempty"`

// Transport selects how the commands reach the game. Empty resolves to
// rcon when rcon.protocol != none, else stdin.
// +kubebuilder:validation:Enum=rcon;stdin
// +optional
Transport string `json:"transport,omitempty"`

// Group labels a section on the server detail page (e.g. "World").
// +kubebuilder:validation:MaxLength=24
// +optional
Group string `json:"group,omitempty"`
```

`Command` is retained for the ~13 existing actions. A CEL `XValidation` rule enforces that
exactly one of `command` / `commands` is set. The rule sits on `ServerActionSpec`, which is
inside `capabilities.actions` — an array already bounded by `MaxItems=32`, with `Commands`
bounded by `MaxItems=16` and command strings bounded in length, keeping the apiserver's CEL
cost budget satisfied.

The transport default is **"prefer the protocol that returns output"**, not "follow
`consoleMode`". This matters for factorio, which is `consoleMode: pty` but has a working
Source RCON — its actions should run over RCON and return their output, not be fired blindly
at stdin.

#### Where each transport executes

**RCON actions** continue to run in the **agent** (`POST /actions/run`,
`agent/internal/actions/`), extended to iterate `Commands` in order and concatenate output.
A command error aborts the remaining steps and returns the failure.

**Stdin actions** must run in the **API**. The agent is a sidecar and cannot write to the
game container's stdin — only the kubelet holds that pipe, and the agent deliberately has no
Kubernetes credentials. The API already performs SPDY pod-attach for the Console tab
(`api/internal/ws/attach.go`) and already holds `pods/attach` RBAC, so this is wiring rather
than new capability:

- a non-WebSocket helper, `WriteStdinLines(ctx, ns, pod, container, lines []string)`, in
  `api/internal/kube/`, modelled on the existing attach path and on the operator's
  `gameserver_stop_attach.go`, which already writes a single stop line to stdin;
- `POST /servers/{name}/actions/run` resolves the transport and branches: forward to the
  agent for `rcon`, attach-and-write for `stdin`.

Stdin is fire-and-forget: no output returns synchronously (it surfaces in the Console
stream), so the response's `raw` is empty and the dashboard reports *sent* rather than
rendering output.

#### Shared validation: the `gameaction/` module

Param sanitization currently lives in `agent/internal/actions/` — `resolveParams`,
`validateParam`, and `hasControl`, the last of which rejects CR/LF and control bytes and is
the only thing standing between a parameter value and arbitrary console injection. It is an
*internal* package of the agent Go module, so the API cannot import it. The API is now an
execution path for actions, so it needs exactly that logic.

Extract param resolution, validation, and `text/template` rendering into a new top-level Go
module, **`gameaction/`**, imported by both `agent` and `api`. This follows the precedent of
`netguard/`, which was carved out of the agent for the same reason — security-critical logic
shared by two modules. Cost: a `go.work` entry, a Makefile target, a `.testcoverage.yml`
gate, and a CI matrix entry.

The rejected alternative is reimplementing injection defense in the API. Duplicating the one
piece of code we least want two versions of is not acceptable, and the divergence would be
silent.

Both transports validate. The agent keeps validating even though the API validates first:
the agent's endpoint is reachable with a token, so it is a trust boundary in its own right.

#### UI

A game with 4–8 grouped buttons is a materially different card from today's flat row of 1–6.
`web/src/components/server/ServerActionsCard.tsx` needs grouped sections, overflow behavior,
and a distinct "sent" state for stdin actions. Per CLAUDE.md rule 1 this starts in
`design.pen` via the Pencil MCP server, not in React.

### 4. Module content

In the `gameplane-module` repo (the `modules/` submodule). Every module gets categories;
every module whose console supports them gets a rich action set drawn from a **canonical id
vocabulary**, so the same id means the same thing across games:

`broadcast` · `save-world` · `set-time` · `set-weather` · `announce-restart` ·
`reload-config` · `set-difficulty` · `toggle-pvp`

plus genuinely game-specific ones (factorio's `set-evolution`, ark's `destroy-wild-dinos`).
Grouped as **World / Server / Players**. Roughly 4–8 per game, capped by what each console
genuinely supports.

Actions deliberately do **not** duplicate kick / ban / unban / whitelist. Those already exist
as the `capabilities.players` feature rendered on the Players page; re-adding them as buttons
would be two UIs for one operation. (The existing minecraft-java `whitelist-add` /
`whitelist-remove` actions are reviewed against this rule during phase 4.)

`announce-restart` is the motivating case for `commands:` — warn, wait, save, stop.

After all three protocols land, 12 modules have RCON of some flavour, and 3 more
(terraria, dont-starve-together, valheim) have a `consoleMode: pty` stdin console.
**enshrouded** has no remote console of any kind and gets no actions.

One caveat to verify in phase 4: **valheim** declares `consoleMode: pty` chiefly so the
Console tab can stream the server's stdout. Whether the Valheim dedicated server actually
*reads commands* from stdin is unconfirmed — terraria and dont-starve-together demonstrably
do. If valheim's stdin is write-only in practice, it gets no actions and the finding is
recorded in its template comment, leaving 14 of 16 modules with actions.

## Sequencing

Four phases, each its own PR or small stack. Phases 1 and 2 are independent of each other;
phase 4 depends on 2 and 3.

1. **Categories** — CRD + codegen, API union-merge, web multi-chip filter, Pencil pass for
   chip overflow, all 16 `module.yaml` files. Self-contained.
2. **Protocols** — one PR each: `websocket`/rust, then `battleye`/dayz, then `satisfactory`.
   Plus the two free fixes (garrys-mod, 7-days-to-die) as one small PR.
3. **Action schema** — extract `gameaction/`, add `commands` / `transport` / `group`, build
   the API stdin path, Pencil pass, rework `ServerActionsCard`.
4. **Module action content** — the rich per-game action sets, then bump the submodule pointer
   in this repo.

## Testing

Per CLAUDE.md rule 8, nothing runs locally; every phase pushes to a branch and CI is the
source of truth.

- **Protocol clients** — unit tests against fake servers, matching the existing
  `agent/internal/rcon/rcon_test.go` and `telnet_test.go` pattern: `httptest` +
  `coder/websocket` for WebRcon, a fake UDP listener for BattlEye, `httptest` TLS for
  Satisfactory. Cover auth failure, multi-packet/multi-page replies, timeout, and — for
  BattlEye — keepalive and server-message acking.
- **`gameaction/`** — carries the coverage gate for param validation, including the
  control-character and enum-membership cases the agent tests assert today.
- **Operator** — envtest for `categories`, the new protocol enum values, and the
  `command` xor `commands` CEL rule.
- **API** — handler test for the stdin transport against a fake attacher interface, plus the
  RBAC check (running an action requires `servers:write`).
- **Web** — vitest for multi-category filtering (a module in two categories appears under
  both chips) and for grouped action rendering.
- **E2E** — any new e2e test must be registered in `test/e2e/buckets.sh`, or the bucket
  coverage job fails.

Beyond CI, the claims unit tests cannot settle are verified against live servers on the
kubelab cluster before the module bundles ship:

- that garrys-mod's and 7-days-to-die's images actually expose a password knob;
- that each new action's command string is real rather than plausible.

## Decisions and rejected alternatives

- **Replace `category` rather than adding `categories` alongside it.** Purely additive would
  break nothing, but leaves two CRD fields meaning the same thing forever, with every reader
  unioning them. Zero modules declare a category today, so replacement is cheap. Legacy
  scalar `category:` is still accepted by the bundle parser for third-party bundles.
- **Free-form categories, not a CRD enum.** An enum guarantees a tidy chip row but hard-blocks
  third-party authors from coining a category and needs a CRD change per genre, breaking the
  no-frontend-change promise the current design comment makes.
- **The regex heuristic in `web/src/lib/games.ts` stays.** Once every official module declares
  categories the heuristic only serves third-party modules that declare none — but for those,
  a guess beats dumping everything into "Other".
- **`satisfactory`, not `http`, as a protocol name.** See §1.
- **Stdin actions execute in the API, not the agent.** The agent cannot reach another
  container's stdin, and giving it Kubernetes credentials to do so would widen its blast
  radius for no gain.
- **Shared `gameaction/` module, not duplicated validation.** See §3.
